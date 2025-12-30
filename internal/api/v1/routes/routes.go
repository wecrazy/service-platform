package routes

import (
	"fmt"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"service-platform/internal/api/v1/controllers"
	"service-platform/internal/config"
	"service-platform/internal/middleware"
	"service-platform/internal/pkg/fun"
	"strings"

	"github.com/dchest/captcha"
	"github.com/gin-gonic/gin"
	"github.com/go-redis/redis/v8"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/sirupsen/logrus"
	analytics "github.com/tom-draper/api-analytics/analytics/go/gin"
	"gorm.io/gorm"

	swaggerFiles "github.com/swaggo/files"
	ginSwagger "github.com/swaggo/gin-swagger"
)

func StaticFile(router *gin.Engine) {
	staticPath := config.GetConfig().App.StaticDir
	publishedDir := config.GetConfig().App.PublishedDir

	// Resolve static path to absolute
	staticPath, err := filepath.Abs(staticPath)
	if err != nil {
		fmt.Println("Error getting absolute path:", err)
		return
	}

	// Load global HTML templates
	router.LoadHTMLGlob(filepath.Join(staticPath, "**", "*.html"))

	// Swagger Route
	router.GET("/swagger", func(c *gin.Context) {
		c.Redirect(http.StatusMovedPermanently, "/swagger/index.html")
	})
	router.GET("/swagger/*any", func(c *gin.Context) {
		if c.Param("any") == "" || c.Param("any") == "/" {
			c.Redirect(http.StatusMovedPermanently, "/swagger/index.html")
		} else {
			ginSwagger.WrapHandler(swaggerFiles.Handler)(c)
		}
	})

	// Serve static directories
	if publishedDir != "" {
		var directories []string

		// Support multiple directories
		if strings.Contains(publishedDir, "|") {
			directories = strings.Split(publishedDir, "|")
		} else {
			directories = append(directories, publishedDir)
		}

		for _, dir := range directories {
			// Skip entries with '#' (optional: handle as comment/ignore marker)
			if strings.Contains(dir, "#") {
				continue
			}

			// Clean relative path
			cleanDir := filepath.Clean(dir)

			// Combine with static root
			staticDirPath := filepath.Join(staticPath, cleanDir)

			// Check if it exists
			if _, err := os.Stat(staticDirPath); os.IsNotExist(err) {
				fmt.Println("Directory does not exist:", staticDirPath)
				continue
			}

			// Serve static files under constructed URL
			urlPath := path.Join(config.GLOBAL_URL, cleanDir)
			router.Static(urlPath, staticDirPath)

			fmt.Println("📂 Published static dir:", staticDirPath, "at", urlPath)
		}
	}

	webUploadDir, err := fun.FindValidDirectory([]string{
		"web/uploads",
		"../web/uploads",
		"../../web/uploads",
		"../../../web/uploads",
	})

	if err != nil {
		logrus.Errorf("Error finding valid upload directory: %v", err)
		return
	}

	logDir, err := fun.FindValidDirectory([]string{
		"log",
		"../log",
		"../../log",
		"../../../log",
	})

	if err != nil {
		logrus.Errorf("Error finding valid log directory: %v", err)
		return
	}

	waReplyDir, err := fun.FindValidDirectory([]string{
		"web/file/wa_reply",
		"../web/file/wa_reply",
		"../../web/file/wa_reply",
		"../../../web/file/wa_reply",
	})

	if err != nil {
		logrus.Errorf("Error finding valid WhatsApp reply directory: %v", err)
		return
	}

	router.Static("./uploads", webUploadDir)
	router.Static("./log", logDir)
	router.Static("./wa_reply", waReplyDir)
	// router.Static("/media", "./web/assets/whatsapp_media") // Serve WhatsApp media files
}

func HtmlRoutes(
	db *gorm.DB,
	router *gin.Engine,
	redisDB *redis.Client,
	systemMonitor *fun.SystemResourceMonitor) {

	globalURL := config.GLOBAL_URL
	if globalURL == "" {
		logrus.Fatal("no global URL set in config")
	}

	apiURL := config.API_URL
	if apiURL == "" {
		logrus.Fatal("no API URL set in config")
	}

	// To view the dashboard API analytics go to: https://www.apianalytics.dev/dashboard and enter your API key
	router.Use(analytics.Analytics(config.GetConfig().API.AnalyticsDevAPIKey))

	// Rate limiting middleware
	router.Use(middleware.RateLimitMiddleware())

	// Health check endpoint for monitoring
	router.GET(globalURL+"health", controllers.GetHealthCheck(db, systemMonitor))

	// Prometheus metrics endpoint
	router.GET("/api-metrics", gin.WrapH(promhttp.Handler()))

	// Note: net/http/pprof uses DefaultServeMux, so we mount it using gin's Handle method
	// if you want in charts view mode try using go tool pprof -http=:2222 http://localhost:2221/debug/pprof/profile
	pprofGroup := router.Group(globalURL + "debug/pprof")
	{
		pprofGroup.GET("/", gin.WrapF(http.HandlerFunc(controllers.PprofIndex)))
		pprofGroup.GET("/heap", gin.WrapF(http.HandlerFunc(controllers.PprofHeap)))
		pprofGroup.GET("/profile", gin.WrapF(http.HandlerFunc(controllers.PprofProfile)))
		pprofGroup.GET("/block", gin.WrapF(http.HandlerFunc(controllers.PprofBlock)))
		pprofGroup.GET("/goroutine", gin.WrapF(http.HandlerFunc(controllers.PprofGoroutine)))
		pprofGroup.GET("/threadcreate", gin.WrapF(http.HandlerFunc(controllers.PprofThreadcreate)))
		pprofGroup.GET("/cmdline", gin.WrapF(http.HandlerFunc(controllers.PprofCmdline)))
		pprofGroup.GET("/symbol", gin.WrapF(http.HandlerFunc(controllers.PprofSymbol)))
		pprofGroup.POST("/symbol", gin.WrapF(http.HandlerFunc(controllers.PprofSymbol)))
		pprofGroup.GET("/trace", gin.WrapF(http.HandlerFunc(controllers.PprofTrace)))
		pprofGroup.GET("/allocs", gin.WrapF(http.HandlerFunc(controllers.PprofAllocs)))
		pprofGroup.GET("/mutex", gin.WrapF(http.HandlerFunc(controllers.PprofMutex)))
	}

	router.GET(globalURL+"hello", controllers.GetHello)

	// Server-side captcha endpoints are present but unused when client generates CAPTCHA locally.
	router.GET(globalURL+"captcha/:id.png", gin.WrapH(captcha.Server(240, 80)))
	router.GET(globalURL+"captcha/new", controllers.GetNewCaptcha)

	router.GET(globalURL+"ws", controllers.WebSocketVerify(db))
	router.GET(globalURL, func(c *gin.Context) { c.Redirect(http.StatusPermanentRedirect, globalURL+"login") }) // Permanent redirect / landing page

	router.GET(globalURL+"login", controllers.GetWebLogin(db))            // Web login GUI
	router.POST(globalURL+"login", controllers.PostWebLogin(db, redisDB)) // Send login credentials
	// Note: client-side verification is used; server-side verify route still available if needed.
	router.GET(globalURL+"forgot-password", controllers.GetWebForgotPassword(db))
	router.POST(globalURL+"forgot-password", controllers.PostForgotPassword(db, redisDB))
	router.GET(globalURL+"reset-password/:email/:token_data", controllers.GetWebResetPassword(db, redisDB))
	router.POST(globalURL+"reset-password/:email/:token_data", controllers.PostResetPassword(db, redisDB))
	router.GET(globalURL+"page", controllers.MainPage(db, redisDB)) // Dashboard with multi tabs shown in index.html
	router.GET(globalURL+"logout", controllers.GetWebLogout(db))    // Logout and clear cookies via button click
	router.GET(globalURL+"profile/default.jpg", controllers.GetUserProfile(db))

	router.GET(globalURL+"check_wa", controllers.CheckWAPhoneNumberIsRegistered()) // Check phone number if registered in WhatsApp

	// Authenticated API v1 routes
	api := router.Group(fmt.Sprintf("%s%s:access", globalURL, apiURL), middleware.AuthMiddleware(db, redisDB))
	{
		// GUI Page
		api.GET("/components/:component", controllers.ComponentPage(db, redisDB))

		// Handle dynamic folder structure
		api.GET("/uploads/:year/:month/:day/:filename", controllers.GetUploadedFile)

		/* Dashboard */
		tabDashboard := api.Group("/tab-dashboard")
		{
			tabDashboard.GET("")
		}

		/* Tab App Config */
		tabAppConfig := api.Group("/tab-app-config")
		{
			tabAppConfig.POST("/table", controllers.TableAppConfig(db))
		}

		/*
			Tab Whatsapp
		*/
		tabWhatsapp := api.Group("/tab-whatsapp")
		{
			tabWhatsapp.POST("/connect", controllers.ConnectWhatsApp)
			tabWhatsapp.POST("/disconnect", controllers.DisconnectWhatsApp)
			tabWhatsapp.POST("/logout", controllers.LogoutWhatsApp)
			tabWhatsapp.POST("/refresh_qr", controllers.RefreshWhatsAppQR)
			tabWhatsapp.POST("/send_message", controllers.SendWhatsAppMessage(db))
			tabWhatsapp.POST("/create_status", controllers.CreateStatus(db))
		}

		/*
			Tab Scheduler - Manage scheduled jobs via gRPC
		*/
		tabScheduler := api.Group("/tab-scheduler")
		{
			tabScheduler.GET("/jobs", controllers.ListScheduledJobs())               // List all jobs
			tabScheduler.GET("/jobs/:name", controllers.GetJobStatus())              // Get specific job status
			tabScheduler.POST("/jobs", controllers.RegisterScheduledJob())           // Register new job
			tabScheduler.POST("/jobs/trigger", controllers.TriggerScheduledJob())    // Trigger job manually
			tabScheduler.DELETE("/jobs/:name", controllers.UnregisterScheduledJob()) // Unregister job
			tabScheduler.POST("/reload", controllers.ReloadScheduler())              // Reload scheduler config
		}
	}
}
