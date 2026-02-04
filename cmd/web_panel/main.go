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
	"sync/atomic"
	"syscall"
	"time"

	"service-platform/cmd/web_panel/controllers"
	"service-platform/cmd/web_panel/database"
	"service-platform/cmd/web_panel/email"
	"service-platform/cmd/web_panel/fun"
	"service-platform/cmd/web_panel/grpc/telegram"
	"service-platform/cmd/web_panel/installer"
	"service-platform/cmd/web_panel/internal/gormdb"
	"service-platform/cmd/web_panel/logger"
	"service-platform/cmd/web_panel/middleware"
	"service-platform/cmd/web_panel/pkg/infrastructure"
	"service-platform/cmd/web_panel/routes"
	"service-platform/cmd/web_panel/scheduler"
	"service-platform/internal/config"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
	"github.com/go-co-op/gocron"
	"github.com/go-redis/redis/v8"
	"github.com/sirupsen/logrus"
	"go.mau.fi/whatsmeow"
	"gopkg.in/natefinch/lumberjack.v2"
	"gorm.io/gorm"
)

var (
	GlobalDB         *gorm.DB
	GlobalDBFastlink *gorm.DB
	GlobalDBTA       *gorm.DB
	GlobalDBWebTA    *gorm.DB
	redisClient      atomic.Value // will store *redis.Client
)

func main() {
	// Initialize system resource monitor
	systemMonitor := fun.NewSystemResourceMonitor()
	printSystemInfo()

	// Start system resource monitoring
	systemMonitor.StartResourceMonitoring()

	// Dynamic update yaml config
	config.WebPanel.MustInit("web_panel")
	go config.WebPanel.Watch()

	yamlCfg := config.WebPanel.Get()

	// Init log
	logger.InitLogrus()

	// Open WSL for Redis & MySQL in Windows
	if fun.IsWindows() {
		err := fun.OpenWSL()
		if err != nil {
			fmt.Println("❌ Error opening WSL: " + err.Error())
		}

		err = fun.StartMySQL()
		if err != nil {
			fmt.Println("❌ Error starting MySQL: " + err.Error())
		}
	}

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

	// Increase resource limitations for LINUX
	increaseFileDescriptorLimit()

	// Redis
	setupRedis(yamlCfg)

	//PREPARE DB
	GlobalDB = mustInitDB(
		yamlCfg.Database.Username,
		yamlCfg.Database.Password,
		yamlCfg.Database.Host,
		yamlCfg.Database.Port,
		yamlCfg.Database.Name,
		"main DB",
	)
	// Migrate
	go database.AutoMigrateWeb(GlobalDB)

	GlobalDBFastlink = tryInitDB(
		yamlCfg.Database.UsernameFastlink,
		yamlCfg.Database.PasswordFastlink,
		yamlCfg.Database.HostFastlink,
		yamlCfg.Database.PortFastlink,
		yamlCfg.Database.NameFastlink,
		"fastlink DB",
	)

	GlobalDBTA = tryInitDB(
		yamlCfg.Database.UsernameTA,
		yamlCfg.Database.PasswordTA,
		yamlCfg.Database.HostTA,
		yamlCfg.Database.PortTA,
		yamlCfg.Database.NameTA,
		"ta DB",
	)

	GlobalDBWebTA = tryInitDB(
		yamlCfg.Database.UsernameWebTA,
		yamlCfg.Database.PasswordWebTA,
		yamlCfg.Database.HostWebTA,
		yamlCfg.Database.PortWebTA,
		yamlCfg.Database.NameWebTA,
		"web ta DB",
	)

	// Start monitors
	StartGenericDBHealthMonitor(
		func() *gorm.DB { return GlobalDB },
		func(db *gorm.DB) { GlobalDB = db },
		"main DB",
		MakeDBReconnectFunc(
			yamlCfg.Database.Username,
			yamlCfg.Database.Password,
			yamlCfg.Database.Host,
			yamlCfg.Database.Port,
			yamlCfg.Database.Name,
		),
	)

	StartGenericDBHealthMonitor(
		func() *gorm.DB { return GlobalDBFastlink },
		func(db *gorm.DB) { GlobalDBFastlink = db },
		"fastlink DB",
		MakeDBReconnectFunc(
			yamlCfg.Database.UsernameFastlink,
			yamlCfg.Database.PasswordFastlink,
			yamlCfg.Database.HostFastlink,
			yamlCfg.Database.PortFastlink,
			yamlCfg.Database.NameFastlink,
		),
	)

	StartGenericDBHealthMonitor(
		func() *gorm.DB { return GlobalDBTA },
		func(db *gorm.DB) { GlobalDBTA = db },
		"TA DB",
		MakeDBReconnectFunc(
			yamlCfg.Database.UsernameTA,
			yamlCfg.Database.PasswordTA,
			yamlCfg.Database.HostTA,
			yamlCfg.Database.PortTA,
			yamlCfg.Database.NameTA,
		),
	)

	StartGenericDBHealthMonitor(
		func() *gorm.DB { return GlobalDBWebTA },
		func(db *gorm.DB) { GlobalDBWebTA = db },
		"Web TA DB",
		MakeDBReconnectFunc(
			yamlCfg.Database.UsernameWebTA,
			yamlCfg.Database.PasswordWebTA,
			yamlCfg.Database.HostWebTA,
			yamlCfg.Database.PortWebTA,
			yamlCfg.Database.NameWebTA,
		),
	)

	gormdb.Databases = &gormdb.DBUsed{
		Web:      GlobalDB,
		FastLink: GlobalDBFastlink,
		TA:       GlobalDBTA,
		WebTA:    GlobalDBWebTA,
	}

	// Initialize Telegram gRPC connection
	initTelegramGRPC(&yamlCfg)

	// Scheduler
	sched := scheduler.StartSchedulers(GlobalDB, &yamlCfg)

	// 🔄 Sync Insert New Ticket Data to ODOO from Inserted ticket record
	go controllers.UpdateDatatoODOOFromInsertedTicketHommyPayCC()

	// Whatsapp
	waClient := initWhatsapp(getRedisClient(), GlobalDB)
	_ = waClient // keep in scope if needed

	// ODOO MS Excel Uploaded Processing
	controllers.ProcessUploadedExcelofODOOMSMustUpdatedTicket(GlobalDB)
	controllers.ProcessUploadedExcelofODOOMSNewTicketCreated(GlobalDB)
	controllers.ProcessUploadExcelofCSNABALost(GlobalDB)
	controllers.ProcessUploadExcelofTechnicianPayroll(GlobalDB)
	controllers.ProcessUploadedExcelofODOOMSMustUpdatedTask(GlobalDB)
	controllers.StartProcessingStateCleanup(GlobalDB)

	// Email Listener
	ctx, cancel := initEmailListener(GlobalDB)
	defer cancel()

	// Make folder needed
	folderFileMainDir, err := fun.FindValidDirectory([]string{
		"web/file",
		"../web/file",
		"../../web/file",
	})
	if err != nil {
		logrus.Fatalf("❌ Failed to find valid directory for file: %v", err)
	}
	folderNeeds := yamlCfg.FolderFileNeeds

	// Create required folders if they don't exist
	for _, folderName := range folderNeeds {
		folderPath := filepath.Join(folderFileMainDir, folderName)
		if _, err := os.Stat(folderPath); os.IsNotExist(err) {
			if err := os.MkdirAll(folderPath, os.ModePerm); err != nil {
				logrus.Errorf("❌ Failed to create folder %s: %v", folderPath, err)
			} else {
				logrus.Infof("📁 Successfully created folder: %s", folderName)
			}
		} else if err != nil {
			logrus.Errorf("❌ Error checking folder %s: %v", folderPath, err)
		}
	}

	// Start web server
	startWebServer(&yamlCfg, sched, ctx, cancel, systemMonitor)
}

func initWhatsapp(RedisDB *redis.Client, db *gorm.DB) *whatsmeow.Client {
	logrus.Info("📲 Initializing WhatsApp client...")
	waClient, err := controllers.StartWhatsappClient(RedisDB, db)
	if err != nil {
		logrus.Fatalf("❌ Failed to init WhatsApp client: %v", err)
	}
	waClient.Connect()
	jidStr := config.WebPanel.Get().Whatsmeow.WaSuperUser + "@s.whatsapp.net"
	idText := fmt.Sprintf("[%v] 📞 Whatsapp siap digunakan", time.Now().Format("2006-01-02 15:04:05"))
	enText := fmt.Sprintf("[%v] 📞 Whatsapp is ready to use", time.Now().Format("2006-01-02 15:04:05"))
	controllers.SendLangMessage(jidStr, idText, enText, "id")
	logrus.Info("✅ WhatsApp client connected")
	return waClient
}

func initEmailListener(db *gorm.DB) (context.Context, context.CancelFunc) {
	ctx, cancel := context.WithCancel(context.Background())
	email.IsYahooMailListenerRunning = false
	go email.StartYahooMailListener(ctx, db)
	return ctx, cancel
}

func startWebServer(
	yamlCfg *config.TypeWebPanel,
	sched *gocron.Scheduler,
	ctx context.Context,
	cancel context.CancelFunc,
	systemMonitor *fun.SystemResourceMonitor,
) {
	// HANDLE WEB ENDPOINT
	appLogDir := yamlCfg.App.LogDir
	if err := os.MkdirAll(appLogDir, os.ModePerm); err != nil {
		log.Fatal(err)
	}
	logPath := filepath.Join(appLogDir, yamlCfg.App.AppLogFilename)
	logWriter := &lumberjack.Logger{
		Filename:   logPath,
		MaxSize:    15, // MB
		MaxBackups: 30,
		MaxAge:     7,    // days
		Compress:   true, // compress rotated files
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

	routes.HtmlRoutes(r, getRedisClient(), systemMonitor)

	listenAddr := fmt.Sprintf(":%s", webHostPort)
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
	case <-ctx.Done():
		logrus.Println("🔻 Context cancelled, shutting down server...")
	case <-sigs:
		logrus.Println("🔻 Received shutdown signal (SIGINT/SIGTERM), shutting down server...")
		logrus.Println("📝 This appears to be a manual shutdown or system signal")
		cancel() // Cancel any other operations
	case err := <-serverErr:
		logrus.Fatalf("❌ Server error: %v", err)
	}

	// Perform graceful shutdown with timeout
	shutdownCtx, cancelShutdown := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancelShutdown()

	if err := srv.Shutdown(shutdownCtx); err != nil {
		logrus.Errorf("⚠ Graceful shutdown failed: %v", err)
	} else {
		logrus.Println("✅ Server stopped gracefully.")

		jidStr := config.WebPanel.Get().Whatsmeow.WaSuperUser + "@s.whatsapp.net"
		idText := fmt.Sprintf("[%v] 🔻 Server dimatikan", time.Now().Format("2006-01-02 15:04:05"))
		enText := fmt.Sprintf("[%v] 🔻 Server stopped", time.Now().Format("2006-01-02 15:04:05"))
		controllers.SendLangMessage(jidStr, idText, enText, "id")
	}

	// Close Telegram gRPC connection
	closeTelegramGRPC()

	sched.Stop()
	os.Exit(0)
}

func printHostInfo(yamlCfg *config.TypeWebPanel, listenAddr string) {
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

func getRedisClient() *redis.Client {
	v := redisClient.Load()
	if v == nil {
		return nil
	}
	return v.(*redis.Client)
}

func pingRedis(client *redis.Client) error {
	_, err := client.Ping(context.Background()).Result()
	return err
}

func HandleCLIArgs(yamlCfg *config.TypeWebPanel) bool {
	if len(os.Args) > 1 {
		arg := os.Args[1]
		switch arg {
		case "--install":
			fmt.Println("🔧 Running install process...")

			installer.EnsureAdminPrivileges()

			switch runtime.GOOS {
			case "windows":
				installer.WindowsService(yamlCfg)
			case "linux":
				installer.LinuxService(yamlCfg)
			case "darwin":
				fmt.Println("🍎 macOS detected — but we are sorry, we don't have macOS installer yet")
			default:
				fmt.Printf("⚠️ Unsupported OS: %s\n", runtime.GOOS)
			}

			return true
		default:
			fmt.Printf("⚠️ Unknown argument: %s\n", arg)
			return false
		}
	}
	return false
}

func increaseFileDescriptorLimit() {
	var rLimit syscall.Rlimit
	fmt.Println("🔍 Checking current file descriptor limit...")
	if err := syscall.Getrlimit(syscall.RLIMIT_NOFILE, &rLimit); err != nil {
		panic(fmt.Errorf("❌ Failed to get rlimit: %w", err))
	}
	fmt.Printf("📊 Current limit: Soft = %d, Hard = %d\n", rLimit.Cur, rLimit.Max)

	rLimit.Cur = rLimit.Max

	fmt.Printf("🔧 Increasing soft limit to match hard limit (%d)...\n", rLimit.Max)
	if err := syscall.Setrlimit(syscall.RLIMIT_NOFILE, &rLimit); err != nil {
		panic(fmt.Errorf("❌ Failed to set rlimit: %w", err))
	}
	fmt.Println("✅ File descriptor limit successfully increased.")
}

func setupRedis(cfg config.TypeWebPanel) {
	redisHost := os.Getenv("REDIS_HOST")
	if redisHost == "" {
		redisHost = cfg.Redis.Host
	}
	maxAttempts := cfg.Redis.MaxRetry
	delay := time.Duration(cfg.Email.RetryDelay) * time.Second

	client, err := infrastructure.RetryConnect(maxAttempts, delay, func() (*redis.Client, error) {
		return connectRedis(cfg, redisHost)
	})
	if err != nil {
		logrus.Fatalf("Failed to connect to Redis after retries: %v", err)
	}
	redisClient.Store(client)

	// health monitor
	go monitorRedis(cfg, redisHost, maxAttempts, delay)
}

func connectRedis(cfg config.TypeWebPanel, redisHost string) (*redis.Client, error) {
	client := redis.NewClient(&redis.Options{
		Addr:     fmt.Sprintf("%s:%d", redisHost, cfg.Redis.Port),
		Password: cfg.Redis.Password,
		DB:       cfg.Redis.Db,
	})
	_, err := client.Ping(context.Background()).Result()
	if err != nil {
		return nil, err
	}
	logrus.Info("✅ Connected to Redis")
	return client, nil
}

func monitorRedis(cfg config.TypeWebPanel, redisHost string, maxAttempts int, delay time.Duration) {
	ticker := time.NewTicker(10 * time.Second)
	defer ticker.Stop()

	for range ticker.C {
		client := getRedisClient()
		if client == nil || pingRedis(client) != nil {
			logrus.Warn("Redis disconnected. Reconnecting...")
			newClient, err := infrastructure.RetryConnect(maxAttempts, delay, func() (*redis.Client, error) {
				return connectRedis(cfg, redisHost)
			})
			if err == nil {
				redisClient.Store(newClient)
				logrus.Info("Reconnected to Redis")
			} else {
				logrus.WithError(err).Error("Redis reconnection attempts failed.")
			}
		}
	}
}

func mustInitDB(user, pass, host, port, name, label string) *gorm.DB {
	db, err := database.InitAndCheckDB(user, pass, host, port, name)
	if err != nil {
		logrus.Fatalf("Failed to init %s: %v", label, err)
	}
	logrus.Infof("✅ Connected to %s", label)
	return db
}

func tryInitDB(user, pass, host, port, name, label string) *gorm.DB {
	db, err := database.InitAndCheckDB(user, pass, host, port, name)
	if err != nil {
		logrus.Warnf("Failed to init %s: %v", label, err)
		return nil
	}
	logrus.Infof("✅ Connected to %s", label)
	return db
}

func GlobalDBReconnectFunc(cfg config.TypeWebPanel) func() (*gorm.DB, error) {
	return func() (*gorm.DB, error) {
		return database.InitAndCheckDB(
			cfg.Database.Username,
			cfg.Database.Password,
			cfg.Database.Host,
			cfg.Database.Port,
			cfg.Database.Name,
		)
	}
}

func DBFastlinkReconnectFunc(cfg config.TypeWebPanel) func() (*gorm.DB, error) {
	return func() (*gorm.DB, error) {
		return database.InitAndCheckDB(
			cfg.Database.UsernameFastlink,
			cfg.Database.PasswordFastlink,
			cfg.Database.HostFastlink,
			cfg.Database.PortFastlink,
			cfg.Database.NameFastlink,
		)
	}
}

func DBTAReconnectFunc(cfg config.TypeWebPanel) func() (*gorm.DB, error) {
	return func() (*gorm.DB, error) {
		return database.InitAndCheckDB(
			cfg.Database.UsernameTA,
			cfg.Database.PasswordTA,
			cfg.Database.HostTA,
			cfg.Database.PortTA,
			cfg.Database.NameTA,
		)
	}
}

func DBWebTAReconnectFunc(cfg config.TypeWebPanel) func() (*gorm.DB, error) {
	return func() (*gorm.DB, error) {
		return database.InitAndCheckDB(
			cfg.Database.UsernameWebTA,
			cfg.Database.PasswordWebTA,
			cfg.Database.HostWebTA,
			cfg.Database.PortWebTA,
			cfg.Database.NameWebTA,
		)
	}
}

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

func reconnectWithRetries(
	setDB func(*gorm.DB),
	reconnect func() (*gorm.DB, error),
	label string,
) {
	cfg := config.WebPanel.Get()
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

func MakeDBReconnectFunc(user, pass, host, port, name string) func() (*gorm.DB, error) {
	return func() (*gorm.DB, error) {
		return database.InitAndCheckDB(user, pass, host, port, name)
	}
}

func initTelegramGRPC(cfg *config.TypeWebPanel) {
	logrus.Info("📡 Initializing Telegram gRPC connection...")
	if err := telegram.InitConnection(cfg.TelegramService.GRPCHost, cfg.TelegramService.GRPCPort); err != nil {
		logrus.Warnf("⚠️ Telegram gRPC connection failed at startup: %v (will retry on first send)", err)
	} else {
		logrus.Info("✅ Telegram gRPC connection established")
	}
}

func closeTelegramGRPC() {
	logrus.Info("📡 Closing Telegram gRPC connection...")
	if err := telegram.CloseConnection(); err != nil {
		logrus.Warnf("⚠️ Error closing Telegram gRPC connection: %v", err)
	}
}

func printSystemInfo() {
	fmt.Println("🛠 Starting system info...")

	// Go version
	fmt.Printf("📦 Go version: %s\n", runtime.Version())

	// OS and Arch
	fmt.Printf("🖥 OS: %s %s\n", runtime.GOOS, runtime.GOARCH)

	// CPU cores
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
