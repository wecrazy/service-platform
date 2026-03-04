// Package main is the entry point for the Service Platform API/Dashboard server.
//
// It initialises the full HTTP stack including Gin middleware (CORS, rate-limiting,
// caching, sanitisation, security headers, request logging), Swagger documentation,
// static file serving, and HTML template routes. On the backend side it connects to
// PostgreSQL (with health monitoring and automatic reconnection), Redis (with retry
// and reconnect logic), and establishes gRPC clients for the WhatsApp, Telegram, and
// Scheduler microservices.
//
// On graceful shutdown (SIGINT / SIGTERM) the server sends a multilingual WhatsApp
// notification to the configured superuser phone number.
//
// Configuration is loaded from service-platform.<env>.yaml via the config package.
//
// Usage:
//
//	# Run directly
//	go run cmd/api/main.go or make run-api
//
//	# Build and run
//	make build-api && ./bin/api
//
//	# Install as a system service
//	./bin/api --install
package main

import (
	"context"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"runtime"
	"service-platform/docs"
	"service-platform/internal/api/v1/controllers"
	"service-platform/internal/api/v1/routes"
	"service-platform/internal/database"
	"service-platform/internal/installer"
	"service-platform/internal/middleware"
	"service-platform/internal/scheduler"
	"service-platform/pkg/fun"
	"service-platform/pkg/logger"
	"sync/atomic"
	"syscall"
	"time"

	"service-platform/internal/config"
	"service-platform/internal/telegram"
	"service-platform/internal/whatsapp"
	pb "service-platform/proto"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
	"github.com/go-redis/redis/v8"
	"github.com/sirupsen/logrus"
	"go.mau.fi/whatsmeow/types"
	"gopkg.in/natefinch/lumberjack.v2"
	"gorm.io/gorm"
)

// @title           API Server for Dashboard's Service
// @version         1.0
// @description     This is a server API documentation for Service Platform Dashboard.
// @termsOfService  http://swagger.io/terms/

// @contact.name    Wegil
// @contact.url     https://github.com/wecrazy
// @contact.email   wegirandol@smartwebindonesia.com

// @license.name    Apache 2.0
// @license.url     http://www.apache.org/licenses/LICENSE-2.0.html

// @host            localhost:6221
// @BasePath        /

// @securityDefinitions.apikey ApiKeyAuth
// @in header
// @name Authorization

var (
	GlobalDB    *gorm.DB     // GlobalDB is a package-level variable that holds the main database connection instance, which is initialized in the main function and used across the application for database operations.
	RedisClient atomic.Value // RedisClient is a package-level variable that holds the Redis client instance, which is initialized in the main function and used across the application for Redis operations.
)

// printSystemInfo prints detailed system information including Go version, OS details, CPU cores, memory statistics, and active network interfaces.
func printSystemInfo() {
	fmt.Println("🛠 Starting system info...")
	fmt.Printf("📦 Go version: %s\n", runtime.Version())
	fmt.Printf("🖥 OS: %s %s\n", runtime.GOOS, runtime.GOARCH)
	fmt.Printf("💻 CPU cores: %d\n", runtime.NumCPU())

	// Memory stats
	var m runtime.MemStats
	runtime.ReadMemStats(&m)
	fmt.Printf("🧠 Alloc = %.2f MB | Sys = %.2f MB\n", float64(m.Alloc)/1024/1024, float64(m.Sys)/1024/1024)

	// Network interfaces
	interfaces, err := net.Interfaces()
	if err != nil {
		fmt.Printf("🌐 Failed to get network interfaces: %v\n", err)
	} else {
		for _, iface := range interfaces {
			if iface.Flags&net.FlagUp != 0 && iface.Flags&net.FlagLoopback == 0 {
				addrs, _ := iface.Addrs()
				for _, addr := range addrs {
					fmt.Printf("🌐 Interface: %s → %v\n", iface.Name, addr.String())
				}
			}
		}
	}

	fmt.Println("✅ Done collecting system info!")
}

// retryConnect retries connectFn up to maxAttempts times, waiting delay between each attempt.
// maxAttempts: the maximum number of connection attempts.
// delay: the duration to wait between attempts.
// connectFn: the function to execute, which should return (T, error).
// Returns the result of a successful connection or an error if all attempts fail.
func retryConnect[T any](maxAttempts int, delay time.Duration, connectFn func() (T, error)) (T, error) {
	var zero T
	var lastErr error

	for attempt := 1; attempt <= maxAttempts; attempt++ {
		conn, err := connectFn()
		if err == nil {
			return conn, nil
		}
		lastErr = err
		fmt.Printf("Connect attempt %d/%d failed: %v\n", attempt, maxAttempts, err)
		time.Sleep(delay)
	}
	return zero, fmt.Errorf("all attempts failed: %w", lastErr)
}

// setupRedis initializes and connects to Redis using the provided configuration, with retry logic and health monitoring.
// cfg: the YAML configuration containing Redis settings.
func setupRedis(cfg config.TypeServicePlatform) {
	redisHost := cfg.Redis.Host
	if redisHost == "" {
		logrus.Fatal("Redis host is not configured.")
	}

	// Ensure Redis is running
	if err := fun.EnsureRedisRunning(redisHost, cfg.Redis.Port); err != nil {
		logrus.Fatalf("Failed to ensure Redis is running: %v", err)
		os.Exit(0)
	}

	maxAttempts := cfg.Redis.MaxRetry
	delay := time.Duration(cfg.Redis.RetryDelay) * time.Second

	client, err := retryConnect(maxAttempts, delay, func() (*redis.Client, error) {
		return connectRedis(cfg, redisHost)
	})
	if err != nil {
		logrus.Fatalf("Failed to connect to Redis after retries: %v", err)
	}
	RedisClient.Store(client)

	// health monitor
	go monitorRedis(cfg, redisHost, maxAttempts, delay)
}

// connectRedis creates and returns a new Redis client with the given configuration.
// cfg: the YAML configuration containing Redis settings.
// redisHost: the hostname of the Redis server.
// Returns a connected Redis client or an error if connection fails.
func connectRedis(cfg config.TypeServicePlatform, redisHost string) (*redis.Client, error) {
	client := redis.NewClient(&redis.Options{
		Addr:            fmt.Sprintf("%s:%d", redisHost, cfg.Redis.Port),
		Password:        cfg.Redis.Password,
		DB:              cfg.Redis.Db,
		MaxRetries:      cfg.Redis.MaxRetry,
		MinRetryBackoff: time.Duration(cfg.Redis.RetryDelay) * time.Second,
		PoolSize:        cfg.Redis.PoolSize,
	})
	_, err := client.Ping(context.Background()).Result()
	if err != nil {
		return nil, err
	}
	logrus.Infof("✅ Connected to Redis at %s:%d", redisHost, cfg.Redis.Port)
	return client, nil
}

// getRedisClient retrieves the current Redis client from the atomic value.
// Returns the Redis client or nil if not set.
func getRedisClient() *redis.Client {
	v := RedisClient.Load()
	if v == nil {
		return nil
	}
	return v.(*redis.Client)
}

// pingRedis pings the Redis client to check connectivity.
// client: the Redis client to ping.
// Returns an error if the ping fails, nil otherwise.
func pingRedis(client *redis.Client) error {
	_, err := client.Ping(context.Background()).Result()
	return err
}

// monitorRedis runs a goroutine that periodically checks Redis connectivity and reconnects if necessary.
// cfg: the YAML configuration containing Redis settings.
// redisHost: the hostname of the Redis server.
// maxAttempts: the maximum number of reconnection attempts.
// delay: the duration to wait between reconnection attempts.
func monitorRedis(cfg config.TypeServicePlatform, redisHost string, maxAttempts int, delay time.Duration) {
	ticker := time.NewTicker(10 * time.Second)
	defer ticker.Stop()

	for range ticker.C {
		client := getRedisClient()
		if client == nil || pingRedis(client) != nil {
			logrus.Warn("Redis disconnected. Reconnecting...")
			newClient, err := retryConnect(maxAttempts, delay, func() (*redis.Client, error) {
				return connectRedis(cfg, redisHost)
			})
			if err == nil {
				RedisClient.Store(newClient)
				logrus.Info("Reconnected to Redis")
			} else {
				logrus.WithError(err).Error("Redis reconnection attempts failed.")
			}
		}
	}
}

// mustInitDB initializes and checks the database connection, panicking on failure.
// dbType: the type of database (e.g., "postgres").
// user: the database username.
// pass: the database password.
// host: the database host.
// port: the database port.
// name: the database name.
// sslMode: the SSL mode for the connection.
// label: a label for logging purposes.
// Returns the initialized GORM database instance.
func mustInitDB(dbType, user, pass, host string, port int, name, sslMode, label string) *gorm.DB {
	db, err := database.InitAndCheckDB(dbType, user, pass, host, port, name, sslMode)
	if err != nil {
		logrus.Fatalf("Failed to init %s: %v", label, err)
	}
	logrus.Infof("✅ Connected to %s as %s at %s:%d", name, label, host, port)
	return db
}

// MakeDBReconnectFunc creates a reconnection function for the database.
// dbType: the type of database.
// user: the database username.
// pass: the database password.
// host: the database host.
// port: the database port.
// name: the database name.
// sslMode: the SSL mode.
// Returns a function that attempts to reconnect to the database.
func MakeDBReconnectFunc(
	dbType,
	user,
	pass,
	host string,
	port int,
	name, sslMode string) func() (*gorm.DB, error) {
	return func() (*gorm.DB, error) {
		return database.InitAndCheckDB(dbType, user, pass, host, port, name, sslMode)
	}
}

// StartGenericDBHealthMonitor starts a goroutine to monitor database health and reconnect if necessary.
// getDB: function to get the current database instance.
// setDB: function to set a new database instance.
// label: label for the database (e.g., "main DB").
// reconnect: function to attempt reconnection.
func StartGenericDBHealthMonitor(
	getDB func() *gorm.DB,
	setDB func(*gorm.DB),
	label string,
	reconnect func() (*gorm.DB, error),
) {
	go func() {
		ticker := time.NewTicker(10 * time.Second)
		defer ticker.Stop()

		for range ticker.C {
			db := getDB()
			if db == nil {
				fmt.Printf("⚠️ %s is nil. Attempting reconnection...\n", label)
				reconnectWithRetries(setDB, reconnect, label)
				continue
			}

			sqlDB, err := db.DB()
			if err != nil || sqlDB == nil {
				fmt.Printf("⚠️ %s disconnected (DB() error). Attempting reconnection...\n", label)
				reconnectWithRetries(setDB, reconnect, label)
				continue
			}

			if err := sqlDB.Ping(); err != nil {
				fmt.Printf("⚠️ %s disconnected (Ping error). Attempting reconnection...\n", label)
				reconnectWithRetries(setDB, reconnect, label)
			}
		}
	}()
}

// reconnectWithRetries attempts to reconnect to the database with retries.
// setDB: function to set the new database instance.
// reconnect: function to attempt reconnection.
// label: label for the database.
func reconnectWithRetries(
	setDB func(*gorm.DB),
	reconnect func() (*gorm.DB, error),
	label string,
) {
	cfg := config.ServicePlatform.Get()
	maxRetry := cfg.Database.MaxRetryConnect
	delay := time.Duration(cfg.Database.RetryDelay) * time.Second

	for attempt := 1; attempt <= maxRetry; attempt++ {
		newDB, err := reconnect()
		if err == nil {
			setDB(newDB)
			fmt.Printf("✅ Reconnected to %s.\n", label)
			return
		}
		fmt.Printf("Reconnect attempt %d to %s failed: %v\n", attempt, label, err)
		time.Sleep(delay)
	}
	fmt.Printf("❌ All reconnection attempts to %s failed.\n", label)
}

// HandleCLIArgs processes command-line arguments for installation.
// yamlCfg: the YAML configuration.
// Returns true if the application should exit after handling the argument.
func HandleCLIArgs(yamlCfg *config.TypeServicePlatform) bool {
	if len(os.Args) > 1 {
		arg := os.Args[1]
		switch arg {
		case "--install":
			fmt.Println("🔧 Running install process...")
			installer.EnsureAdminPrivileges()
			installer.Install(yamlCfg)
			return true
		default:
			fmt.Printf("⚠️ Unknown argument: %s\n", arg)
			return false
		}
	}
	return false
}

// createFolderNeeds creates necessary folders based on configuration.
// cfg: the YAML configuration containing folder needs.
func createFolderNeeds(cfg *config.TypeServicePlatform) {
	folderDir, err := fun.FindValidDirectory([]string{
		"web/file",
		"../web/file",
		"../../web/file",
		"../../../web/file",
	})
	if err != nil {
		fmt.Printf("❌ Failed to find valid directory for file: %v\n", err)
		logrus.Fatalf("❌ Failed to find valid directory for file: %v", err)
	}

	needs := cfg.FolderFileNeeds

	for _, folderName := range needs {
		folderPath := filepath.Join(folderDir, folderName)
		if _, err := os.Stat(folderPath); os.IsNotExist(err) {
			if err := os.MkdirAll(folderPath, 0755); err != nil {
				logrus.Errorf("❌ Failed to create folder %s: %v", folderPath, err)
			} else {
				logrus.Infof("📁 Successfully created folder: %s", folderName)
			}
		} else if err != nil {
			logrus.Errorf("❌ Error checking folder %s: %v", folderPath, err)
		}
	}
}

// startWebServer starts the Gin web server with routes, middleware, and logging.
// yamlCfg: the YAML configuration.
// systemMonitor: the system resource monitor.
func startWebServer(
	yamlCfg *config.TypeServicePlatform,
	systemMonitor *fun.SystemResourceMonitor,
) {
	appLogDir := config.ServicePlatform.Get().App.LogDir
	// Resolve absolute path for log directory
	if resolvedDir, err := fun.GetLogDir(appLogDir); err == nil {
		appLogDir = resolvedDir
	} else {
		log.Printf("⚠️ Failed to resolve log dir, using default: %v", err)
	}

	if err := os.MkdirAll(appLogDir, os.ModePerm); err != nil {
		log.Fatal(err)
	}
	logPath := filepath.Join(appLogDir, config.ServicePlatform.Get().App.AppLogFilename)
	logWriter := &lumberjack.Logger{
		Filename:   logPath,
		MaxSize:    config.ServicePlatform.Get().App.AppLogMaxSize,
		MaxBackups: config.ServicePlatform.Get().App.AppLogMaxBackups,
		MaxAge:     config.ServicePlatform.Get().App.AppLogMaxAge,
		Compress:   config.ServicePlatform.Get().App.AppLogCompress,
	}

	r := gin.Default()
	r.Use(middleware.LoggerMiddleware(logWriter))
	r.Use(middleware.CacheControlMiddleware())
	r.Use(middleware.SanitizeMiddleware())
	r.Use(middleware.SanitizeCsvMiddleware())
	r.Use(middleware.SecurityControlMiddleware())
	r.Use(cors.Default())

	webHostPort := yamlCfg.App.Port
	gin.SetMode(yamlCfg.App.GinMode)

	routes.StaticFile(r)

	routes.HTMLRoutes(GlobalDB, r, getRedisClient(), systemMonitor)

	listenAddr := fmt.Sprintf(":%d", webHostPort)
	printHostInfo(yamlCfg, listenAddr)

	// Graceful shutdown setup
	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, syscall.SIGINT, syscall.SIGTERM)

	srv := &http.Server{
		Addr:    listenAddr,
		Handler: r,
	}

	keyFile, err := fun.FindValidDirectory([]string{
		"cert/demo.wecrazy.my.id.key",
		"../cert/demo.wecrazy.my.id.key",
		"../../cert/demo.wecrazy.my.id.key",
		"../../../cert/demo.wecrazy.my.id.key",
	})
	if err != nil {
		logrus.Fatalf("❌ Failed to find SSL key file: %v", err)
	}

	certFile, err := fun.FindValidDirectory([]string{
		"cert/demo.wecrazy.my.id.crt",
		"../cert/demo.wecrazy.my.id.crt",
		"../../cert/demo.wecrazy.my.id.crt",
		"../../../cert/demo.wecrazy.my.id.crt",
	})
	if err != nil {
		logrus.Fatalf("❌ Failed to find SSL cert file: %v", err)
	}

	serverErr := make(chan error, 1)
	go func() {
		logrus.Printf("🌐 Starting server on %s ...", listenAddr)
		// if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		// 	log.Fatal(err)
		// 	serverErr <- fmt.Errorf("server listen error: %w", err)
		// }
		if err := srv.ListenAndServeTLS(certFile, keyFile); err != nil && err != http.ErrServerClosed {
			serverErr <- fmt.Errorf("server listen error: %w", err)
		}
	}()

	select {
	case sig := <-sigs:
		logrus.Infof("🔔 Caught signal %s: shutting down server...", sig)
	case err := <-serverErr:
		logrus.Errorf("❌ Server error: %v", err)
	}

	// Flush any remaining Loki logs before shutdown
	if lokiHook := logger.GetLokiHook(); lokiHook != nil {
		logrus.Info("📤 Flushing remaining logs to Loki...")
		_ = lokiHook.Flush()
	}

	// Send shutdown notification to super user BEFORE closing connections
	suPhoneNumber := config.ServicePlatform.Get().Default.SuperUserPhone
	jidStr := fmt.Sprintf("%s@%s", suPhoneNumber, types.DefaultUserServer)

	// Send shutdown message and wait for completion before closing connections
	sendShutdownNotification(jidStr)

	scheduler.CloseClient() // Close Scheduler gRPC connection
	whatsapp.Close()        // Close WhatsApp gRPC connection

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := srv.Shutdown(ctx); err != nil {
		logrus.Errorf("❌ Server forced to shutdown: %v", err)
	} else {
		logrus.Info("✅ Server exited gracefully.")
	}
}

// printHostInfo prints the host information for the web server.
// yamlCfg: the YAML configuration.
// listenAddr: the address the server is listening on.
func printHostInfo(yamlCfg *config.TypeServicePlatform, listenAddr string) {
	url := func() string {
		if listenAddr == ":80" || listenAddr == ":443" {
			return "localhost" + listenAddr
		}
		host := yamlCfg.App.Host
		if host == "" {
			host = "localhost"
		}
		return host + listenAddr
	}()
	fmt.Printf("🌐 Web Hosted at http://%s/\n", url)
}

// sendShutdownNotification sends a shutdown notification to the super user via WhatsApp gRPC service.
// This function blocks until the message is sent or timeout occurs.
func sendShutdownNotification(jidStr string) {
	if whatsapp.Client == nil {
		logrus.Warn("⚠️ WhatsApp gRPC client is not available. Shutdown notification could not be sent.")
		return
	}

	// Create multilingual shutdown message
	langMsg := controllers.NewLanguageMsgTranslation(fun.DefaultLang)
	langMsg.Texts = map[string]string{
		fun.LangID: "📴 API Server telah berhasil dimatikan.",
		fun.LangEN: "📴 API Server has been shut down successfully.",
		fun.LangES: "📴 El servidor API se ha apagado con éxito.",
		fun.LangFR: "📴 Le serveur API a été arrêté avec succès.",
		fun.LangDE: "📴 Der API-Server wurde erfolgreich heruntergefahren.",
		fun.LangPT: "📴 O servidor API foi desligado com sucesso.",
		fun.LangAR: "📴 تم إيقاف خادم API بنجاح.",
		fun.LangJP: "📴 APIサーバーは正常にシャットダウンされました。",
		fun.LangCN: "📴 API服务器已成功关闭。",
		fun.LangRU: "📴 Сервер API успешно завершил работу.",
	}

	// Send the message with timeout context
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// Send via gRPC client
	done := make(chan error, 1)
	go func() {
		defer func() {
			if r := recover(); r != nil {
				logrus.Errorf("❌ Panic while sending shutdown notification: %v", r)
				done <- fmt.Errorf("panic: %v", r)
			}
		}()

		// Call gRPC method to send message (send English version)
		shutdownMsg := langMsg.Texts[langMsg.LanguageCode]
		resp, err := whatsapp.Client.SendMessage(ctx, &pb.SendMessageRequest{
			To: jidStr,
			Content: &pb.MessageContent{
				ContentType: &pb.MessageContent_Text{
					Text: shutdownMsg,
				},
			},
		})

		if err != nil {
			done <- fmt.Errorf("gRPC error: %w", err)
			return
		}

		if !resp.Success {
			done <- fmt.Errorf("message send failed: %s", resp.Message)
			return
		}

		done <- nil
	}()

	select {
	case err := <-done:
		if err != nil {
			logrus.Warnf("⚠️ Failed to send shutdown notification: %v", err)
		} else {
			logrus.Info("✅ Shutdown notification sent successfully to super user.")
		}
	case <-ctx.Done():
		logrus.Warn("⚠️ Shutdown notification send timeout. Server will proceed with shutdown.")
	}
}

// main is the entry point of the application, initializing resources, loading config, and starting services.
func main() {
	// Initialize system resource monitor
	systemMonitor := fun.NewSystemResourceMonitor()
	printSystemInfo()
	// Start system resource monitoring
	systemMonitor.StartResourceMonitoring()

	config.ServicePlatform.MustInit("service-platform") // Load config with name "service-platform.%s.yaml"
	go config.ServicePlatform.Watch()

	yamlCfg := config.ServicePlatform.Get()

	// Update Swagger Info dynamically
	docs.SwaggerInfo.Host = fmt.Sprintf("%s:%d", yamlCfg.App.Host, yamlCfg.App.Port)
	if yamlCfg.App.Host == "" {
		docs.SwaggerInfo.Host = fmt.Sprintf("localhost:%d", yamlCfg.App.Port)
	}
	docs.SwaggerInfo.Title = yamlCfg.App.Name
	docs.SwaggerInfo.Description = "API Documentation for " + yamlCfg.App.Name

	// Init log
	logger.InitLogrus()

	exePath, err := os.Executable()
	if err != nil {
		logrus.Fatalf("Error getting executable path: %v", err)
	}

	exeDir := filepath.Dir(exePath)
	logrus.Infof("📁 Executable directory: %s", exeDir)

	// CLI
	if HandleCLIArgs(&yamlCfg) {
		return
	}

	// Increase resource limitations (handled by OS-specific implementations)
	fun.IncreaseFileDescriptorLimit()

	// Redis
	setupRedis(yamlCfg)

	// Initialize rate limiter
	middleware.InitRateLimiter(getRedisClient())

	GlobalDB = mustInitDB(
		yamlCfg.Database.Type,
		yamlCfg.Database.Username,
		yamlCfg.Database.Password,
		yamlCfg.Database.Host,
		yamlCfg.Database.Port,
		yamlCfg.Database.Name,
		yamlCfg.Database.SSLMode,
		"main DB",
	)

	database.AutoMigrateDB(GlobalDB)

	// Start monitors
	StartGenericDBHealthMonitor(
		func() *gorm.DB { return GlobalDB },
		func(db *gorm.DB) { GlobalDB = db },
		"main DB",
		MakeDBReconnectFunc(
			yamlCfg.Database.Type,
			yamlCfg.Database.Username,
			yamlCfg.Database.Password,
			yamlCfg.Database.Host,
			yamlCfg.Database.Port,
			yamlCfg.Database.Name,
			yamlCfg.Database.SSLMode,
		),
	)

	database.DBList = &database.DBUsed{
		Main: GlobalDB,
	}

	createFolderNeeds(&yamlCfg)

	// WhatsApp Client by Whatsmeow https://pkg.go.dev/go.mau.fi/whatsmeow
	whatsapp.InitClient()

	// Telegram Client by github.com/go-telegram-bot-api/telegram-bot-api/v5 registered telegram account via BotFather and get the token, then set it in .yaml config
	telegram.InitClient()

	// Scheduler gRPC Client - connects to the Scheduler microservice (cmd/scheduler/main.go) which runs the task scheduler for periodic jobs like sending reminders, notifications, etc.
	// This decouples scheduling logic from the main API server and allows for better scalability and separation of concerns.
	scheduler.InitClient()

	// ADD: more microservice clients (e.g., email, SMS, payment gateway) can be initialized here as needed in the future.

	startWebServer(&yamlCfg, systemMonitor)
}
