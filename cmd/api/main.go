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
	"service-platform/internal/api/v1/routes"
	"service-platform/internal/database"
	"service-platform/internal/installer"
	"service-platform/internal/middleware"
	"service-platform/internal/pkg/fun"
	"service-platform/internal/pkg/logger"
	"service-platform/internal/scheduler"
	"sync/atomic"
	"syscall"
	"time"

	"service-platform/internal/config"
	"service-platform/internal/whatsapp"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
	"github.com/go-redis/redis/v8"
	"github.com/sirupsen/logrus"
	"github.com/swaggo/swag/example/basic/docs"
	"gopkg.in/natefinch/lumberjack.v2"
	"gorm.io/gorm"
)

// @title           API Server for Dashboard's Service
// @version         1.0
// @description     This is a server API documentation for Service Platform Dashboard.
// @termsOfService  http://swagger.io/terms/

// @contact.name    RM Developer
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
	GlobalDB    *gorm.DB
	RedisClient atomic.Value // will store *redis.Client
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
func setupRedis(cfg config.YamlConfig) {
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
func connectRedis(cfg config.YamlConfig, redisHost string) (*redis.Client, error) {
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
func monitorRedis(cfg config.YamlConfig, redisHost string, maxAttempts int, delay time.Duration) {
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
	cfg := config.GetConfig()
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
func HandleCLIArgs(yamlCfg *config.YamlConfig) bool {
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
func createFolderNeeds(cfg *config.YamlConfig) {
	folderDir, err := fun.FindValidDirectory([]string{
		"web/file",
		"../web/file",
		"../../web/file",
		"../../../web/file",
	})
	if err != nil {
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
	yamlCfg *config.YamlConfig,
	systemMonitor *fun.SystemResourceMonitor,
) {
	appLogDir := config.GetConfig().App.LogDir
	// Resolve absolute path for log directory
	if resolvedDir, err := fun.GetLogDir(appLogDir); err == nil {
		appLogDir = resolvedDir
	} else {
		log.Printf("⚠️ Failed to resolve log dir, using default: %v", err)
	}

	if err := os.MkdirAll(appLogDir, os.ModePerm); err != nil {
		log.Fatal(err)
	}
	logPath := filepath.Join(appLogDir, config.GetConfig().App.AppLogFilename)
	logWriter := &lumberjack.Logger{
		Filename:   logPath,
		MaxSize:    config.GetConfig().App.AppLogMaxSize,
		MaxBackups: config.GetConfig().App.AppLogMaxBackups,
		MaxAge:     config.GetConfig().App.AppLogMaxAge,
		Compress:   config.GetConfig().App.AppLogCompress,
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

	routes.HtmlRoutes(GlobalDB, r, getRedisClient(), systemMonitor)

	listenAddr := fmt.Sprintf(":%d", webHostPort)
	printHostInfo(yamlCfg, listenAddr)

	// Graceful shutdown setup
	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, syscall.SIGINT, syscall.SIGTERM)

	srv := &http.Server{
		Addr:    listenAddr,
		Handler: r,
	}

	serverErr := make(chan error, 1)
	go func() {
		logrus.Printf("🌐 Starting server on %s ...", listenAddr)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			serverErr <- fmt.Errorf("server listen error: %w", err)
		}
	}()

	select {
	case sig := <-sigs:
		logrus.Infof("🔔 Caught signal %s: shutting down server...", sig)
	case err := <-serverErr:
		logrus.Errorf("❌ Server error: %v", err)
	}

	whatsapp.Close()        // Close WhatsApp gRPC connection
	scheduler.CloseClient() // Close Scheduler gRPC connection

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
func printHostInfo(yamlCfg *config.YamlConfig, listenAddr string) {
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

// main is the entry point of the application, initializing resources, loading config, and starting services.
func main() {
	// Initialize system resource monitor
	systemMonitor := fun.NewSystemResourceMonitor()
	printSystemInfo()
	// Start system resource monitoring
	systemMonitor.StartResourceMonitoring()

	// Dynamic update yaml config
	if err := config.LoadConfig(); err != nil {
		log.Fatalf("Error loading .yaml conf :%v", err)
	}

	go config.WatchConfig()
	yamlCfg := config.GetConfig()

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

	// Scheduler gRPC Client - connects to scheduler service
	scheduler.InitClient()

	// NOTE: Scheduler is now running as a separate gRPC service (cmd/grpc/main.go)
	// ADD: email client & listener if exists and needed !
	startWebServer(&yamlCfg, systemMonitor)
}
