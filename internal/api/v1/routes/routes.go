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
	staticPath := config.ServicePlatform.Get().App.StaticDir
	publishedDir := config.ServicePlatform.Get().App.PublishedDir

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
	router.Use(analytics.Analytics(config.ServicePlatform.Get().API.AnalyticsDevAPIKey))

	// Rate limiting middleware
	router.Use(middleware.RateLimitMiddleware())

	// Health check endpoint for monitoring
	router.GET(globalURL+"health", controllers.GetHealthCheck(db, systemMonitor))

	// Prometheus metrics endpoint
	router.GET("/api-metrics", gin.WrapH(promhttp.Handler()))

	RegisterPprofRoutes(router, globalURL)

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

		RegisterAppConfigRoutes(api, db)
		RegisterWhatsAppRoutes(api, db, redisDB)
		RegisterWhatsAppUserManagementRoutes(api, db)
		RegisterSchedulerRoutes(api)
		RegisterTelegramRoutes(api, db)
		RegisterTwilioWhatsAppRoutes(api, db)
	}

	// Twilio WhatsApp Webhook - Incoming messages (NOT authenticated)
	// This receives incoming WhatsApp messages from Twilio Sandbox
	// Endpoint: POST /twilio_reply
	twilioGroup := router.Group("", middleware.TwilioRateLimitMiddleware())
	twilioGroup.POST("/twilio_reply", controllers.HandleTwilioWhatsAppWebhook(db))
}
