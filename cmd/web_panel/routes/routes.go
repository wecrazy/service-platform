package routes

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"service-platform/cmd/web_panel/controllers"
	"service-platform/cmd/web_panel/fun"
	"service-platform/cmd/web_panel/internal/gormdb"
	"service-platform/cmd/web_panel/middleware"
	"service-platform/cmd/web_panel/model"
	"service-platform/internal/config"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/go-redis/redis/v8"
	"github.com/sirupsen/logrus"
	analytics "github.com/tom-draper/api-analytics/analytics/go/gin"
)

func StaticFile(router *gin.Engine) {
	staticPath := config.WebPanel.Get().App.StaticDir
	publishedDir := config.WebPanel.Get().App.PublishedDir

	// Resolve static path to absolute
	staticPath, err := filepath.Abs(staticPath)
	if err != nil {
		fmt.Println("Error getting absolute path:", err)
		return
	}

	// Load global HTML templates
	router.LoadHTMLGlob(filepath.Join(staticPath, "**", "*.html"))

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
			urlPath := path.Join(fun.GLOBAL_URL, cleanDir)
			router.Static(urlPath, staticDirPath)

			fmt.Println("📂 Published static dir:", staticDirPath, "at", urlPath)
		}
	}

	router.Static("./uploads", "uploads")
	router.Static("./log-data", "log")
	router.Static("./wa", "whatsmeow")
	router.Static("./wa_reply", "web/file/wa_reply")
	router.Static("/media", "./web/assets/whatsapp_media") // Serve WhatsApp media files

	// Surat Peringatan (SP) Technician, SPL & SAC
	router.Static("/sp_sounding", "./web/file/sounding_sp_technician") // Serve SP Technician sounding files
	router.Static("/sp_sounding_spl", "./web/file/sounding_sp_spl")    // Serve SP SPL sounding files
	router.Static("/sp_sounding_sac", "./web/file/sounding_sp_sac")    // Serve SP SAC sounding files
}

func HtmlRoutes(router *gin.Engine, redisDB *redis.Client, systemMonitor *fun.SystemResourceMonitor) {
	db := gormdb.Databases.Web
	dbFastlink := gormdb.Databases.FastLink

	// To view the dashboard API analytics go to: https://www.apianalytics.dev/dashboard and enter your API key
	router.Use(analytics.Analytics(config.WebPanel.Get().Default.APIKeyApiAnalyticsDev)) // Add middleware

	// Health check endpoint for monitoring
	router.GET("/health", func(c *gin.Context) {
		health := systemMonitor.GetHealthStatus(db)

		// Check database connections
		dbStatus := "healthy"
		if db == nil {
			dbStatus = "disconnected"
		} else {
			sqlDB, err := db.DB()
			if err != nil || sqlDB.Ping() != nil {
				dbStatus = "unhealthy"
			}
		}
		health["database"] = dbStatus

		if dbStatus != "healthy" {
			health["status"] = "degraded"
			c.JSON(http.StatusServiceUnavailable, health)
			return
		}

		// Check if status is critical (from system monitor)
		if status, ok := health["status"].(string); ok && status == "critical" {
			c.JSON(http.StatusServiceUnavailable, health)
			return
		}

		c.JSON(http.StatusOK, health)
	})

	// Note: net/http/pprof uses DefaultServeMux, so we mount it using gin's Handle method
	// if you want in charts view mode try using go tool pprof -http=:2222 http://localhost:2221/debug/pprof/profile
	pprofGroup := router.Group(fun.GLOBAL_URL + "debug/pprof")
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

	router.GET("/hello", func(ctx *gin.Context) {
		data := map[string]string{
			"message": "Hello, World!",
		}
		ctx.JSON(http.StatusOK, data)
	})

	router.GET(fun.GLOBAL_URL+"ping", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"message": "pong!"})
	})

	router.GET(fun.GLOBAL_URL+"api/ping", func(c *gin.Context) {
		i := c.Query("i")
		if i != "" {
			c.JSON(http.StatusOK, gin.H{"message": "pong", "i": i})
		} else {
			c.JSON(http.StatusOK, gin.H{"message": "pong"})
		}
	})
	router.GET(fun.GLOBAL_URL+"ws", controllers.WebSocketVerify(db))

	// router.GET(fun.GLOBAL_URL+"", controllers.GetWebLandingPage(db)) // LANDING PAGE
	router.GET(fun.GLOBAL_URL, func(c *gin.Context) { c.Redirect(http.StatusPermanentRedirect, fun.GLOBAL_URL+"login") })

	router.GET(fun.GLOBAL_URL+"login", controllers.GetWebLogin(db))            // WEB LOGIN
	router.POST(fun.GLOBAL_URL+"login", controllers.PostWebLogin(db, redisDB)) // SEND LOGIN CREDENTIALS
	router.GET(fun.GLOBAL_URL+"captcha", controllers.GetCaptchaImage())

	router.GET(fun.GLOBAL_URL+"forgot-password", controllers.GetWebForgotPassword(db))
	router.POST(fun.GLOBAL_URL+"forgot-password", controllers.PostForgotPassword(db, redisDB))
	router.GET(fun.GLOBAL_URL+"reset-password/:email/:token_data", controllers.GetWebResetPassword(db, redisDB))
	router.POST(fun.GLOBAL_URL+"reset-password/:email/:token_data", controllers.PostResetPassword(db, redisDB))

	//MAIN PAGE
	router.GET(fun.GLOBAL_URL+"page", controllers.MainPage(db, redisDB))

	// LOGOUT BY BUTTON
	router.GET(fun.GLOBAL_URL+"logout", controllers.GetWebLogout(db))
	// router.GET(fun.GLOBAL_URL+"register", controllers.getRegister(db))

	router.GET(fun.GLOBAL_URL+"profile/default.jpg", controllers.GetUserProfile(db))

	// Photos of Data in Merchant Fastlink
	router.GET(fun.GLOBAL_URL+"photos/merchant_fastlink/:id", controllers.ShowPhotoByIDForMerchantFastlink(redisDB, dbFastlink))

	// Check WhatsApp number registration
	router.GET(fun.GLOBAL_URL+"check_wa", controllers.CheckWAPhoneNumberIsRegistered())

	// Debug and migration endpoints for WhatsApp (development and testing purposes only)
	// router.GET("/debug/tables", controllers.CheckDatabaseTables(db))
	// router.POST("/debug/migrate", controllers.ForceMigrateWhatsappTables(db))
	// router.POST("/debug/migrate-jids", controllers.MigrateJIDFormats(db))
	// router.POST("/debug/seed/:userid", controllers.SeedCompleteWhatsappData(db))
	// router.GET("/debug/conversations/:userid", controllers.GetWhatsappConversationDataAPI(db))
	// router.GET("/debug/contacts/:userid", controllers.GetWhatsappContactListAPI(db))
	// router.GET("/debug/messages/:userid/:conversationid", controllers.GetWhatsappConversationMessagesAPI(db))
	// router.POST("/debug/sync-contacts/:userid", controllers.SyncWhatsappContactsEndpoint(db))
	// router.GET("/debug/contact-list/:userid", controllers.GetWhatsappContactListHTML(db))
	// router.POST("/debug/test-insert", controllers.TestInsertSingleContact(db))

	// Endpoint Web routes group
	web := router.Group(fun.GLOBAL_URL+"web/:access", middleware.AuthMiddleware(db, redisDB))
	{

		//GUI PAGE COMPONENT
		web.GET("/components/:component", controllers.ComponentPage(db, redisDB))

		// Handle dynamic folder structure
		web.GET("/uploads/:year/:month/:day/:filename", func(c *gin.Context) {
			// Extract parameters from the route
			year := c.Param("year")
			month := c.Param("month")
			day := c.Param("day")
			filename := c.Param("filename")

			// Construct the file path
			filePath := filepath.Join("./uploads", year, month, day, filename)

			// Clean the file path to prevent directory traversal
			safePath := filepath.Clean(filePath)

			// Ensure the safePath is within the uploads directory
			if !filepath.HasPrefix(safePath, filepath.Clean("./uploads")) {
				c.JSON(http.StatusForbidden, gin.H{"error": "invalid file path"})
				return
			}

			// Serve the file
			c.File(safePath)
		})

		/* Dashboard */
		tabDashboard := web.Group("/tab-wp-dashboard")
		{
			tabDashboard.GET("")
		}

		/* Tab App Config */
		tabAppConfig := web.Group("/tab-app-config")
		{
			tabAppConfig.POST("/table", controllers.TableAppConfig())
		}

		/* Hommy Pay */
		tabTicket := web.Group("/tab-hommy-pay-cc-ticket")
		{
			tabTicket.GET("/refresh", controllers.RefreshTicketHommyPay(db))
			tabTicket.GET("/last_update", controllers.LastUpdateTicketHommyPayCC(db))
			tabTicket.POST("/table", controllers.TableTicketHommyPayCC(db))
			tabTicket.PUT("/table", controllers.PutDataTicketHommyPayCC(db))
			tabTicket.DELETE("/table/:id", controllers.DeleteDataTicketHommyPayCC(db))
			tabTicket.GET("/table.csv", controllers.ExportTable[model.TicketHommyPayCC](db, "File di unggah"))
			tabTicket.POST("/table/create", controllers.PostNewTicketHommyPayCC(db))
			tabTicket.GET("/table/batch/template", controllers.GetBatchTemplateDataTicket[model.TicketHommyPayCC](db))
			tabTicket.POST("/table/batch/create", controllers.PostBatchUploadDataTicket[model.TicketHommyPayCC](db))
		}

		tabMerchant := web.Group("/tab-hommy-pay-cc-merchant")
		{
			tabMerchant.GET("/refresh", controllers.RefreshMerchantHommyPay(db))
			tabMerchant.GET("/last_update", controllers.LastUpdateMerchantHommyPay(db))
			tabMerchant.POST("/table", controllers.TableMerchantHommyPay(db))
			tabMerchant.GET("/table.csv", controllers.ExportTable[model.MerchantHommyPayCC](db, "File di unggah"))

			tabMerchant.POST("/table_fastlink", controllers.TableMerchantFastlink(dbFastlink))
			tabMerchant.GET("/table_fastlink.csv", controllers.ExportTable[model.MerchantFastlink](dbFastlink, "File di unggah"))
			tabMerchant.GET("/last_update_fastlink", controllers.LastUpdateMerchantHommyPay(dbFastlink))

			tabMerchant.POST("/table_kresekbag", controllers.TableMerchantKresekBag(db))
			tabMerchant.GET("/table_kresekbag.csv", controllers.ExportTable[model.MerchantKresekBag](db, "File di unggah"))
			tabMerchant.GET("/refresh_kresekbag", controllers.RefreshMerchantKresekBag(db))
			tabMerchant.GET("/last_update_kresekbag", controllers.LastUpdateMerchantKresekBag(db))
			tabMerchant.GET("/photos/merchantKresekBag/:id", controllers.ShowPhotoByIDForMerchantKresekBag(redisDB, db))
		}

		/*
			Tab Whatsapp
		*/
		tabWhatsapp := web.Group("/tab-whatsapp")
		{
			tabWhatsapp.GET("/refresh-qrcode", controllers.RefreshWhatsappQrcode())
			tabWhatsapp.GET("/end-session", controllers.EndSessionWhatsapp())
			tabWhatsapp.POST("/ping", controllers.PingWhatsapp())
			tabWhatsapp.POST("/send_text", controllers.SendTextWhatsapp())
			tabWhatsapp.POST("/send_image", controllers.SendImageWhatsapp())
			tabWhatsapp.POST("/send_document", controllers.SendDocumentWhatsapp())
			tabWhatsapp.POST("/send_location", controllers.SendLocationWhatsapp())
			tabWhatsapp.POST("/send_polling", controllers.SendPollingWhatsapp())
			tabWhatsapp.GET("/groups", controllers.GetWhatsappGroups())
			tabWhatsapp.POST("/wa_log_msg_received", controllers.GetTbLogMsgReceived())
			// Language
			tabWhatsapp.POST("/table_language", controllers.TableWhatsappBotLanguage())
			tabWhatsapp.PUT("/table_language", controllers.PutDataWhatsappBotLanguage())
			tabWhatsapp.DELETE("/table_language/:id", controllers.DeleteDataWhatsappBotLanguage())
			tabWhatsapp.GET("/table_language.csv", controllers.ExportTable[model.Language](db, "File di unggah"))
			tabWhatsapp.POST("/table_language/create", controllers.PostNewWhatsappBotLanguage())
			tabWhatsapp.GET("/last_update_table_language", controllers.LastUpdateTableWhatsappBotLanguage())
			// Message Reply
			tabWhatsapp.POST("/table_message_reply", controllers.TableWhatsappBotMessageReply())
			tabWhatsapp.PUT("/table_message_reply", controllers.PutDataWhatsappBotMessageReply())
			tabWhatsapp.DELETE("/table_message_reply/:id", controllers.DeleteDataWhatsappBotMessageReply())
			tabWhatsapp.GET("/table_message_reply.csv", controllers.ExportTable[model.WAMessageReply](db, "File di unggah"))
			tabWhatsapp.POST("/table_message_reply/create", controllers.PostNewWhatsappBotMessageReply())
			tabWhatsapp.GET("/table_message_reply/batch/template", controllers.GetBatchTemplateWhatsappBotMessageReply[model.WAMessageReply]())
			tabWhatsapp.POST("/table_message_reply/batch/create", controllers.PostBatchUploadDataWhatsappBotMessageReply[model.WAMessageReply]())
			tabWhatsapp.GET("/last_update_table_message_reply", controllers.LastUpdateTableWhatsappBotMessageReply())
		}

		/*
			Tab Whatsapp User Management
		*/
		tabWaUserManagement := web.Group("/tab-whatsapp-user-management")
		{
			tabWaUserManagement.POST("/table", controllers.TableWhatsappUserManagement())
			tabWaUserManagement.PUT("/table", controllers.PutUpdatedWhatsappUserManagement())
			tabWaUserManagement.DELETE("/table/:id", controllers.DeleteDataFromTableWhatsappUserManagement())
			tabWaUserManagement.POST("/table/create", controllers.CreateNewDataTableWhatsappUserManagement())
			tabWaUserManagement.GET("/table/batch/template", controllers.GetBatchTemplateWhatsappUserManagement[model.WAPhoneUser]())
			tabWaUserManagement.POST("/table/batch/create", controllers.PostBatchUploadDataWhatsappUserManagement[model.WAPhoneUser]())
			tabWaUserManagement.POST("/reset_quota_prompt", controllers.ResetQuotaWhatsappPrompt())
			tabWaUserManagement.POST("/unban_user", controllers.UnbanUser())
		}

		/*
			Tab Whatsapp Conversation
		*/
		tabWhatsappConversation := web.Group("/tab-whatsapp-conversation")
		{
			// Check if user's WhatsApp is logged in (legacy - returns only logged_in boolean)
			tabWhatsappConversation.GET("/:userid/status", controllers.IsUserWhatsappLoggedIn(db, redisDB))
			// Get detailed status information for user's WhatsApp client
			tabWhatsappConversation.GET("/:userid/status/detailed", controllers.GetUserWhatsappStatus(db, redisDB))
			// Connect user's WhatsApp client
			tabWhatsappConversation.POST("/:userid/connect", controllers.ConnectUserWhatsapp(db, redisDB))
			// Disconnect user's WhatsApp client
			tabWhatsappConversation.POST("/:userid/disconnect", controllers.DisconnectUserWhatsapp(db, redisDB))
			// Get QR code for user's WhatsApp client
			tabWhatsappConversation.GET("/:userid/qr", controllers.GetUserWhatsappQR(db, redisDB))
			// Refresh QR code for user's WhatsApp client
			tabWhatsappConversation.POST("/:userid/qr/refresh", controllers.RefreshUserWhatsappQR(db, redisDB))
			// Send message using user's WhatsApp client
			tabWhatsappConversation.POST("/:userid/send", controllers.SendUserWhatsappMessage(db, redisDB))
			// WhatsApp Interface Components
			tabWhatsappConversation.GET("/:userid/sidebar-left", controllers.GetWhatsappSidebarLeft(db, redisDB))
			tabWhatsappConversation.GET("/:userid/chat-area", controllers.GetWhatsappChatArea(db, redisDB))
			tabWhatsappConversation.GET("/:userid/contact-list", controllers.GetWhatsappContactList(db, redisDB))
			tabWhatsappConversation.GET("/:userid/conversation-history", controllers.GetWhatsappConversationHistory(db, redisDB))
			// Search Functions
			tabWhatsappConversation.POST("/:userid/search/contacts", controllers.SearchWhatsappContacts(db, redisDB))
			tabWhatsappConversation.POST("/:userid/search/messages", controllers.SearchWhatsappMessages(db, redisDB))
			tabWhatsappConversation.POST("/:userid/search/conversations", controllers.SearchWhatsappConversations(db, redisDB))
			// List all active clients (admin endpoint)
			tabWhatsappConversation.GET("/active", controllers.ListActiveWhatsappClients())

		}

		/*
			Tab Email
		*/
		tabEmail := web.Group("/tab-email")
		{
			tabEmail.GET("/gaktauwkwk", func(c *gin.Context) {
				// Example: just return a 204 No Content with no body :O
				// FIX: this for its contents
				c.Status(http.StatusNoContent)
			})
		}

		/*
			Tab ODOO Manage Service
		*/
		tabODOOMSUploadExcel := web.Group("/tab-odoo-ms-upload-excel")
		{
			tabODOOMSUploadExcel.POST("/validate-credentials", controllers.ValidateODOOCredentials())
			tabODOOMSUploadExcel.POST("/upload", controllers.UploadMustUpdatedTicket(db, redisDB))
			tabODOOMSUploadExcel.POST("/upload_new_ticket", controllers.UploadODOOMSNewTicket(redisDB))
			tabODOOMSUploadExcel.GET("/history", controllers.GetUploadHistoryByEmail(db))
			tabODOOMSUploadExcel.POST("/history/table", controllers.GetUploadHistoryTable(db))
			tabODOOMSUploadExcel.GET("/details/:id", controllers.GetUploadDetails(db))
			tabODOOMSUploadExcel.POST("/upload_ba_lost", controllers.UploadCSNABALost(redisDB))
			tabODOOMSUploadExcel.POST("/upload_technician_payroll", controllers.UploadTechnicianPayrollIntoPayslip(redisDB))
			tabODOOMSUploadExcel.POST("/upload_updated_task", controllers.UploadMustUpdatedTask(db, redisDB))
		} /*
			Tab Human Resource (HR)
		*/
		tabHRSP := web.Group("/tab-hr-sp")
		{
			tabHRSP.POST("/table_technician", controllers.TableSuratPeringatanTechnicianForHR())
			tabHRSP.DELETE("/table_technician/:id", controllers.DeleteSuratPeringatanTechnician())
			tabHRSP.POST("/table_spl", controllers.TableSuratPeringatanSPLForHR())
			tabHRSP.DELETE("/table_spl/:id", controllers.DeleteSuratPeringatanSPL())
			tabHRSP.POST("/table_sac", controllers.TableSuratPeringatanSACForHR())
			tabHRSP.DELETE("/table_sac/:id", controllers.DeleteSuratPeringatanSAC())
			// Delete all SP records
			tabHRSP.POST("/delete_all_sp_technician", controllers.DeleteAllSPTechnician())
			tabHRSP.POST("/delete_all_sp_spl", controllers.DeleteAllSPSPL())
			tabHRSP.POST("/delete_all_sp_sac", controllers.DeleteAllSPSAC())
			// SP Reply Simulation
			tabHRSP.GET("/get_sac_groups", controllers.GetSPSACGroups())
			tabHRSP.GET("/download_sp_reply_template", controllers.DownloadSPReplySimulationTemplate())
			tabHRSP.POST("/upload_sp_reply_simulation", controllers.UploadSPReplySimulation(db))
		}
		tabHRContract := web.Group("/tab-hr-kontrak-teknisi")
		{
			tabHRContract.POST("/table", controllers.TableContractTechnicianForHR())
			tabHRContract.PATCH("/table", controllers.UpdateTableContractTechnicianForHR())
			tabHRContract.DELETE("/table/:id", controllers.DeleteTableContractTechnicianForHR())
			tabHRContract.GET("/last_update", controllers.LastUpdateContractTechnician())
			tabHRContract.GET("/refresh", controllers.RefreshDataContractTechnician())
			tabHRContract.POST("/regenerate_pdf_contract/:id", controllers.RegeneratePDFContractTechnician())
			tabHRContract.POST("/send_individual_contract", controllers.SendIndividualContractTechnician())
			tabHRContract.POST("/get_contract_technician_whatsapp_conversation", controllers.GetContractTechnicianWhatsAppConversation())
			tabHRContract.POST("/send_all_contract", controllers.SendAllContractTechnician())
		}
		tabHRPayslip := web.Group("/tab-hr-slip-gaji-teknisi")
		{
			tabHRPayslip.POST("/table_edc", controllers.TablePayslipMSEDC())
			tabHRPayslip.PATCH("/table_edc", controllers.UpdateTablePayslipMS("edc"))
			tabHRPayslip.DELETE("/table_edc/:id", controllers.DeleteTablePayslipMS("edc"))
			tabHRPayslip.GET("/last_update", controllers.LastUpdatePayslipTechnicianEDC())
			tabHRPayslip.POST("/table_atm", controllers.TablePayslipMSATM())
			tabHRPayslip.PATCH("/table_atm", controllers.UpdateTablePayslipMS("atm"))
			tabHRPayslip.DELETE("/table_atm/:id", controllers.DeleteTablePayslipMS("atm"))
			tabHRPayslip.GET("/last_update_atm", controllers.LastUpdatePayslipTechnicianATM())
			tabHRPayslip.POST("/regenerate_pdf_payslip/:id", controllers.RegeneratePDFPayslipMS())
			// Individual regenerate endpoints for EDC and ATM
			tabHRPayslip.POST("/regenerate_payslip_edc/:id", controllers.RegeneratePayslipTechnicianEDC())
			tabHRPayslip.POST("/regenerate_payslip_atm/:id", controllers.RegeneratePayslipTechnicianATM())
			// Simplified send endpoints using JSON body
			tabHRPayslip.POST("/send_individual_payslip", controllers.SendIndividualPayslipTechnician())
			tabHRPayslip.POST("/send_all_payslip", controllers.SendAllPayslipTechnician())
			// WhatsApp conversation endpoint
			tabHRPayslip.POST("/get_payslip_whatsapp_conversation", controllers.GetPayslipWhatsAppConversation())
		}
		tabHRSPSO := web.Group("/tab-hr-sp-so")
		{
			tabHRSPSO.POST("/table", controllers.TableSuratPeringatanStockOpnameForHR())
			tabHRSPSO.DELETE("/table/:id", controllers.DeleteSuratPeringatanSO())
		}

		/*
			Tab MTI (Mitra Transaksi Indonesia)
		*/
		tabMTI := router.Group("/tab-mti")
		{
			tabMTI.GET("/refresh-task", controllers.RefreshTaskODOOMSMTI())
			tabMTI.GET("/last_update", controllers.LastUpdateDataTaskODOOMSMTI())
		}

		tabMTIPM := web.Group("tab-mti-monitoring-pm")
		{
			tabMTIPM.POST("/table_pm", controllers.TablePMMTI())
			tabMTIPM.POST("/pivot_pm", controllers.PivotPMMTI())
			tabMTIPM.POST("/report_all_pm_mti", controllers.ReportAllPMMTI())
			tabMTIPM.POST("/report_data_filtered_pm_mti", controllers.ReportDataFilteredPMMTI())
		}

		tabMTINonPM := web.Group("tab-mti-monitoring-non-pm")
		{
			tabMTINonPM.POST("/table_non_pm", controllers.TableNonPMMTI())
			tabMTINonPM.POST("/pivot_non_pm", controllers.PivotNonPMMTI())
			tabMTINonPM.POST("/report_all_non_pm_mti", controllers.ReportAllNonPMMTI())
			tabMTINonPM.POST("/report_data_filtered_non_pm_mti", controllers.ReportDataFilteredNonPMMTI())
		}

		/*
			Tab DKI (DKI Jakarta)
		*/
		tabDKITicket := web.Group("/tab-dki-ticket")
		{
			tabDKITicket.GET("/refresh", controllers.RefreshTicketDKI())
			tabDKITicket.GET("/last_update", controllers.GetLastUpdateTicketDKI())
			tabDKITicket.POST("/table", controllers.TableTicketDKI())
			tabDKITicket.POST("/report_all_ticket", controllers.GetReportALLTicketDKI())
			tabDKITicket.POST("/report_all_ticket_filtered", controllers.GetReportTicketDKIFiltered())

		}

		/*
			Tab DSP (PT. Digital Solusi Pratama)
		*/
		tabDSPTicket := web.Group("/tab-dsp-ticket")
		{
			tabDSPTicket.GET("/refresh", controllers.RefreshTicketDSP())
			tabDSPTicket.GET("/last_update", controllers.GetLastUpdateTicketDSP())
			tabDSPTicket.POST("/table", controllers.TableTicketDSP())
			tabDSPTicket.POST("/report_all_ticket", controllers.GetReportALLTicketDSP())
			tabDSPTicket.POST("/report_all_ticket_filtered", controllers.GetReportTicketDSPFiltered())
		}

		/*
			Tab BNI
		*/
		tabBNI := router.Group("/tab-bni")
		{
			tabBNI.GET("/refresh-task", controllers.RefreshTicketBNI())
			tabBNI.GET("/last_update", controllers.GetLastUpdateTicketBNI())
		}

		tabBNIPM := web.Group("tab-bni-monitoring-pm")
		{
			tabBNIPM.POST("/table_pm", controllers.TablePMBNI())
			tabBNIPM.POST("/pivot_pm", controllers.PivotPMBNI())
			tabBNIPM.POST("/report_all_pm_bni", controllers.ReportAllPMBNI())
			tabBNIPM.POST("/report_data_filtered_pm_bni", controllers.ReportDataFilteredPMBNI())
		}

		tabBNINonPM := web.Group("tab-bni-monitoring-non-pm")
		{
			tabBNINonPM.POST("/table_non_pm", controllers.TableNonPMBNI())
			tabBNINonPM.POST("/pivot_non_pm", controllers.PivotNonPMBNI())
			tabBNINonPM.POST("/report_all_non_pm_bni", controllers.ReportAllNonPMBNI())
			tabBNINonPM.POST("/report_data_filtered_non_pm_bni", controllers.ReportDataFilteredNonPMBNI())
		}

		tabRoles := web.Group("/tab-roles")
		{
			// /web/tab-roles/admin/status
			tabRoles.GET("/roles/gui", controllers.GetRolesGui(db))

			tabRoles.GET("/roles/modal", controllers.ModalTabRoles(db))

			tabRoles.POST("/roles/create", controllers.PostRole(db))
			tabRoles.PATCH("/roles", controllers.PatchRole(db))
			tabRoles.DELETE("/roles", controllers.DeleteRoles(db))

			tabRoles.GET("/roles/list", controllers.GetRolesList(db))

			tabRoles.GET("/admins/table", controllers.GetAdminTable(db))
			tabRoles.POST("/admins/create", controllers.PostNewAdminUser(db))
			tabRoles.PATCH("/admins", controllers.PatchAdminData(db))
			tabRoles.DELETE("/admins/:id", controllers.DeleteUserAdmin(db))
		}

		tabSystemLog := web.Group("/tab-system-log")
		{
			tabSystemLog.GET("/system/log/file", controllers.GetSystemLogFiles(db))
			tabSystemLog.GET("/table", controllers.GetSystemLog(db))
			tabSystemLog.GET("/table.csv", controllers.GetSystemLogFileDump(db))
		}

		tabActivityLog := web.Group("/tab-activity-log")
		{ // /web/tab-activity-log/activity/log
			tabActivityLog.GET("/table", controllers.GetActivityLog(db))
			tabActivityLog.GET("/table.csv", controllers.DumpActivityLog(db))
		}
		tabUserProfile := web.Group("/tab-user-profile")
		{
			tabUserProfile.GET("/activity/table", controllers.TableUserActivities(db))
			tabUserProfile.PATCH("/profile-image", controllers.UpdateAdminProfileImage(db))
		}
	}

	router.GET(fun.GLOBAL_URL+"ws-odoo-ms-upload-excel", controllers.WebSocketODOOMSUpdatedTicket(db))

	router.GET((fun.GLOBAL_URL + "get-report-so"), controllers.GetReportOfListSO())

	// ODOOMS
	router.Any(fun.GLOBAL_URL+"here/*path", controllers.PostHere()) // Kukuh Filestore
	tabODOOMS := router.Group(fun.GLOBAL_URL + "odooms")
	{
		tabODOOMS.GET("/task-photos/:id", controllers.GetPhotosOfTaskODOOMS())
	}

	// Mini PC Sound API routes
	miniPCMetland := router.Group(fun.GLOBAL_URL + "mini-pc-metland")
	{
		// Technician
		miniPCMetland.GET(fun.GLOBAL_URL+"sound-statuses", controllers.GetSoundStatusOfSPTechnician())
		miniPCMetland.POST(fun.GLOBAL_URL+"sound-played", controllers.UpdateSoundStatusOfSPTechnician())
		// SPL
		miniPCMetland.GET(fun.GLOBAL_URL+"sound-statuses-spl", controllers.GetSoundStatusOfSPSPL())
		miniPCMetland.POST(fun.GLOBAL_URL+"sound-played-spl", controllers.UpdateSoundStatusOfSPSPL())
		// SAC
		miniPCMetland.GET(fun.GLOBAL_URL+"sound-statuses-sac", controllers.GetSoundStatusOfSPSAC())
		miniPCMetland.POST(fun.GLOBAL_URL+"sound-played-sac", controllers.UpdateSoundStatusOfSPSAC())
	}

	routerTA := router.Group(fun.GLOBAL_URL + "ta")
	{
		routerTA.POST(fun.GLOBAL_URL+"feedback_by_ta", controllers.SendTAFeedbackAboutJONeedsEvidenceToGroupTechnician())
		routerTA.POST(fun.GLOBAL_URL+"followed_up_ta", controllers.SendTAFollowedUpResultToGroupTechnician())
	}

	// Monitoring (Report)
	monitoringODOOMS := router.Group("/odooms-monitoring")
	{
		monitoringODOOMS.GET(fun.GLOBAL_URL+"ticket-performance", controllers.GetTicketODOOMSPerformanceAchivementsChart())
		monitoringODOOMS.POST(fun.GLOBAL_URL+"data-ticket-performance", controllers.GetDataTicketPerformance())
		monitoringODOOMS.GET(fun.GLOBAL_URL+"data-date-range-ticket-performance", controllers.GetDataDateRangeTicketPerformance())
		monitoringODOOMS.GET(fun.GLOBAL_URL+"data-sac-ticket-performance", controllers.GetDataSACTicketPerformance())
		monitoringODOOMS.POST(fun.GLOBAL_URL+"data-spl-ticket-performance", controllers.GetDataSPLTicketPerformance())
		monitoringODOOMS.POST(fun.GLOBAL_URL+"data-technician-ticket-performance", controllers.GetDataTechnicianTicketPerformance())
		monitoringODOOMS.GET(fun.GLOBAL_URL+"data-company-ticket-performance", controllers.GetDataCompanyTicketPerformance())
		monitoringODOOMS.GET(fun.GLOBAL_URL+"data-sla-status-ticket-performance", controllers.GetDataSLAStatusTicketPerformance())
		monitoringODOOMS.GET(fun.GLOBAL_URL+"data-task-type-ticket-performance", controllers.GetDataTaskTypeTicketPerformance())
		monitoringODOOMS.GET(fun.GLOBAL_URL+"download-master-ticket-performance", controllers.DownloadReportMasterTicketPerformance())
		monitoringODOOMS.POST(fun.GLOBAL_URL+"download-filtered-ticket-performance", controllers.DownloadReportFilteredTicketPerformance())

		monitoringODOOMS.GET(fun.GLOBAL_URL+"login-visit-technician", controllers.GetLoginVisitTechnicianChart())
		monitoringODOOMS.GET(fun.GLOBAL_URL+"data-sac-login-visit-technician", controllers.GetDataSACLoginVisitTechnician())
		monitoringODOOMS.GET(fun.GLOBAL_URL+"data-spl-login-visit-technician", controllers.GetDataSPLLoginVisitTechnician())
		monitoringODOOMS.GET(fun.GLOBAL_URL+"data-technician-login-visit-technician", controllers.GetDataTechnicianLoginVisitTechnician())
		monitoringODOOMS.POST(fun.GLOBAL_URL+"data-login-visit-technician", controllers.GetDataLoginVisitTechnician())
		monitoringODOOMS.GET(fun.GLOBAL_URL+"download-master-login-visit-technician", controllers.DownloadReportMasterLoginVisitTechnician())
		monitoringODOOMS.POST(fun.GLOBAL_URL+"highcharts-export", controllers.HighchartsExportProxy())
		monitoringODOOMS.OPTIONS(fun.GLOBAL_URL+"highcharts-export", controllers.HighchartsExportProxy())

		// Cost & Revenue
		monitoringODOOMS.GET(fun.GLOBAL_URL+"list-price-task-type", controllers.GetListPriceTaskType())
		monitoringODOOMS.GET(fun.GLOBAL_URL+"list-price-sales-payment", controllers.GetListPriceSalesPayment())

		// Old Cost Revenue (Daily prediction)
		// monitoringODOOMS.GET(fun.GLOBAL_URL+"cost-revenue", controllers.GetCostRevenueODOOMSChart())
		monitoringODOOMS.POST(fun.GLOBAL_URL+"data-ticket-performance-revenue-cost", controllers.GetDataTicketPerformanceCostRevenue())

		// New Cost vs Revenue Yearly Chart
		monitoringODOOMS.GET(fun.GLOBAL_URL+"cost-revenue", controllers.GetNewCostRevenueODOOMSChart())
		monitoringODOOMS.GET(fun.GLOBAL_URL+"data-new-cost-revenue-yearly", controllers.GetDataNewCostRevenueYearly())
		monitoringODOOMS.GET(fun.GLOBAL_URL+"available-years-cost-revenue", controllers.GetAvailableYears())
		monitoringODOOMS.GET(fun.GLOBAL_URL+"available-months-cost-revenue", controllers.GetAvailableMonths())
		monitoringODOOMS.GET(fun.GLOBAL_URL+"available-companies-cost-revenue", controllers.GetAvailableCompanies())
		monitoringODOOMS.POST(fun.GLOBAL_URL+"drill-down-cost-revenue", controllers.GetDrillDownData())
	}

	// ODOO Manage Service - Project.Task
	odoomsProjectTask := router.Group(fun.GLOBAL_URL + "odooms-project-task")
	{
		odoomsProjectTask.GET("/detailWO", controllers.GetODOOMSProjectTaskDetail())
	}

	// Export group for downloadable reports
	exportGroup := router.Group(fun.GLOBAL_URL + "export")
	{
		exportGroup.GET("/sp-stock-opname-replied-excel", controllers.DownloadSPStockOpnameRepliedExcel)
	}

	// Proxy Report AI Error
	proxyReportAIErr := router.Group(fun.GLOBAL_URL + "report-ai-error")
	{
		proxyReportAIErr.GET("/*filepath", func(c *gin.Context) {
			filepathParam := c.Param("filepath") // e.g. "/2025-09-03/Report.xlsx"
			filename := path.Base(filepathParam)

			selectedMainDir, err := fun.FindValidDirectory([]string{
				"web/file/ai_error/",
				"../web/file/ai_error/",
				"../../web/file/ai_error/",
			})
			var filePath string
			var localFileExists bool = false
			if err == nil {
				filePath = filepath.Join(selectedMainDir, filepathParam)
				if _, err := os.Stat(filePath); err == nil {
					localFileExists = true
				}
			}

			c.Header("Content-Type", "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet")
			c.Header("Content-Disposition", "inline; filename="+filename)
			c.Header("X-Frame-Options", "ALLOWALL")
			c.Header("Access-Control-Allow-Origin", "*")
			c.Header("Accept-Ranges", "bytes") // Required for Excel file handling

			if localFileExists {
				// Use http.ServeFile to support Range requests
				http.ServeFile(c.Writer, c.Request, filePath)
				return
			}

			// If not found locally, fetch from remote server
			remoteURL := config.WebPanel.Get().App.WebPublicURL + c.Request.URL.Path
			resp, err := http.Get(remoteURL)
			if err != nil {
				logrus.Errorf("Error fetching remote Excel file: %v", err)
				c.String(http.StatusNotFound, "Excel file not found locally or remotely")
				return
			}
			defer resp.Body.Close()

			if resp.StatusCode != http.StatusOK {
				logrus.Errorf("Remote Excel file returned status: %v", resp.Status)
				c.String(http.StatusNotFound, "Excel file not found locally or remotely")
				return
			}

			c.Stream(func(w io.Writer) bool {
				_, err := io.Copy(w, resp.Body)
				return err == nil
			})
		})
	}

	proxyReportSLAODOOMS := router.Group(fun.GLOBAL_URL + "report-sla")
	{
		proxyReportSLAODOOMS.GET("/*filepath", func(c *gin.Context) {
			filepathParam := c.Param("filepath") // e.g. "/2025-09-03/Report.xlsx"
			filename := path.Base(filepathParam)

			selectedMainDir, err := fun.FindValidDirectory([]string{
				"web/file/sla_report/",
				"../web/file/sla_report/",
				"../../web/file/sla_report/",
			})
			var filePath string
			var localFileExists bool = false
			if err == nil {
				filePath = filepath.Join(selectedMainDir, filepathParam)
				if _, err := os.Stat(filePath); err == nil {
					localFileExists = true
				}
			}

			c.Header("Content-Type", "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet")
			c.Header("Content-Disposition", "inline; filename="+filename)
			c.Header("X-Frame-Options", "ALLOWALL")
			c.Header("Access-Control-Allow-Origin", "*")
			c.Header("Accept-Ranges", "bytes") // Required for Excel file handling

			if localFileExists {
				// Use http.ServeFile to support Range requests
				http.ServeFile(c.Writer, c.Request, filePath)
				return
			}

			// If not found locally, fetch from remote server
			remoteURL := config.WebPanel.Get().App.WebPublicURL + c.Request.URL.Path
			resp, err := http.Get(remoteURL)
			if err != nil {
				logrus.Errorf("Error fetching remote Excel file: %v", err)
				c.String(http.StatusNotFound, "Excel file not found locally or remotely")
				return
			}
			defer resp.Body.Close()

			if resp.StatusCode != http.StatusOK {
				logrus.Errorf("Remote Excel file returned status: %v", resp.Status)
				c.String(http.StatusNotFound, "Excel file not found locally or remotely")
				return
			}

			c.Stream(func(w io.Writer) bool {
				_, err := io.Copy(w, resp.Body)
				return err == nil
			})
		})
	}

	//  Proxy Report Monitoring (Ticket Achievements & Login Visit Technician)
	proxyReportMonitoring := router.Group(fun.GLOBAL_URL + "report-monitoring")
	{
		proxyReportMonitoring.GET("/*filepath", func(c *gin.Context) {
			filepathParam := c.Param("filepath") // e.g. "/2025-09-03/Report.xlsx"
			filename := path.Base(filepathParam)

			selectedMainDir, err := fun.FindValidDirectory([]string{
				"web/file/monitoring_ticket/",
				"../web/file/monitoring_ticket/",
				"../../web/file/monitoring_ticket/",
			})
			var filePath string
			var localFileExists bool = false
			if err == nil {
				filePath = filepath.Join(selectedMainDir, filepathParam)
				if _, err := os.Stat(filePath); err == nil {
					localFileExists = true
				}
			}

			c.Header("Content-Type", "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet")
			c.Header("Content-Disposition", "inline; filename="+filename)
			c.Header("X-Frame-Options", "ALLOWALL")
			c.Header("Access-Control-Allow-Origin", "*")
			c.Header("Accept-Ranges", "bytes") // Required for Excel file handling

			if localFileExists {
				// Use http.ServeFile to support Range requests
				http.ServeFile(c.Writer, c.Request, filePath)
				return
			}

			// If not found locally, fetch from remote server
			remoteURL := config.WebPanel.Get().App.WebPublicURL + c.Request.URL.Path
			resp, err := http.Get(remoteURL)
			if err != nil {
				logrus.Errorf("Error fetching remote Excel file: %v", err)
				c.String(http.StatusNotFound, "Excel file not found locally or remotely")
				return
			}
			defer resp.Body.Close()

			if resp.StatusCode != http.StatusOK {
				logrus.Errorf("Remote Excel file returned status: %v", resp.Status)
				c.String(http.StatusNotFound, "Excel file not found locally or remotely")
				return
			}

			c.Stream(func(w io.Writer) bool {
				_, err := io.Copy(w, resp.Body)
				return err == nil
			})
		})
	}

	// Surat Peringatan
	proxyPDFSPTechnician := router.Group(fun.GLOBAL_URL + "proxy-pdf-sp-technician")
	{
		proxyPDFSPTechnician.GET("/*filepath", func(c *gin.Context) {
			filepathParam := c.Param("filepath") // e.g. "/2025-09-03/SP_1_1.1 Jakpus.pdf"
			filename := path.Base(filepathParam)

			spTechnicianMainDir, err := fun.FindValidDirectory([]string{
				"web/file/sp_technician/",
				"../web/file/sp_technician/",
				"../../web/file/sp_technician/",
			})
			var filePath string
			var localFileExists bool = false
			if err == nil {
				filePath = filepath.Join(spTechnicianMainDir, filepathParam)
				if _, err := os.Stat(filePath); err == nil {
					localFileExists = true
				}
			}

			c.Header("Content-Type", "application/pdf")
			c.Header("Content-Disposition", "inline; filename="+filename)
			c.Header("X-Frame-Options", "ALLOWALL")
			c.Header("Access-Control-Allow-Origin", "*")
			c.Header("Accept-Ranges", "bytes") // Required for PDF.js

			if localFileExists {
				// Use http.ServeFile to support Range requests
				http.ServeFile(c.Writer, c.Request, filePath)
				return
			}

			// If not found locally, fetch from remote server
			remoteURL := config.WebPanel.Get().App.WebPublicURL + c.Request.URL.Path
			resp, err := http.Get(remoteURL)
			if err != nil {
				logrus.Errorf("Error fetching remote PDF: %v", err)
				c.String(http.StatusNotFound, "PDF not found locally or remotely")
				return
			}
			defer resp.Body.Close()
			if resp.StatusCode != http.StatusOK {
				logrus.Errorf("Remote PDF returned status: %v", resp.Status)
				c.String(http.StatusNotFound, "PDF not found locally or remotely")
				return
			}
			c.Stream(func(w io.Writer) bool {
				_, err := io.Copy(w, resp.Body)
				return err == nil
			})
		})
	}

	proxyPDFSPSPL := router.Group(fun.GLOBAL_URL + "proxy-pdf-sp-spl")
	{
		proxyPDFSPSPL.GET("/*filepath", func(c *gin.Context) {
			filepathParam := c.Param("filepath") // e.g. "/2025-09-05/SP_1_2.3 SPL Bekasi Omar Elakham_2025-09-05.pdf"
			filename := path.Base(filepathParam)

			spTechnicianMainDir, err := fun.FindValidDirectory([]string{
				"web/file/sp_spl/",
				"../web/file/sp_spl/",
				"../../web/file/sp_spl/",
			})
			var filePath string
			var localFileExists bool = false
			if err == nil {
				filePath = filepath.Join(spTechnicianMainDir, filepathParam)
				if _, err := os.Stat(filePath); err == nil {
					localFileExists = true
				}
			}

			c.Header("Content-Type", "application/pdf")
			c.Header("Content-Disposition", "inline; filename="+filename)
			c.Header("X-Frame-Options", "ALLOWALL")
			c.Header("Access-Control-Allow-Origin", "*")
			c.Header("Accept-Ranges", "bytes") // Required for PDF.js

			if localFileExists {
				// Use http.ServeFile to support Range requests
				http.ServeFile(c.Writer, c.Request, filePath)
				return
			}

			// If not found locally, fetch from remote server
			remoteURL := config.WebPanel.Get().App.WebPublicURL + c.Request.URL.Path
			resp, err := http.Get(remoteURL)
			if err != nil {
				logrus.Errorf("Error fetching remote PDF: %v", err)
				c.String(http.StatusNotFound, "PDF not found locally or remotely")
				return
			}
			defer resp.Body.Close()
			if resp.StatusCode != http.StatusOK {
				logrus.Errorf("Remote PDF returned status: %v", resp.Status)
				c.String(http.StatusNotFound, "PDF not found locally or remotely")
				return
			}
			c.Stream(func(w io.Writer) bool {
				_, err := io.Copy(w, resp.Body)
				return err == nil
			})
		})
	}

	proxyPDFSPSAC := router.Group(fun.GLOBAL_URL + "proxy-pdf-sp-sac")
	{
		proxyPDFSPSAC.GET("/*filepath", func(c *gin.Context) {
			filepathParam := c.Param("filepath") // e.g. "/2025-09-05/SP_1_ SAC asdasdad.pdf"
			filename := path.Base(filepathParam)

			spTechnicianMainDir, err := fun.FindValidDirectory([]string{
				"web/file/sp_sac/",
				"../web/file/sp_sac/",
				"../../web/file/sp_sac/",
			})
			var filePath string
			var localFileExists bool = false
			if err == nil {
				filePath = filepath.Join(spTechnicianMainDir, filepathParam)
				if _, err := os.Stat(filePath); err == nil {
					localFileExists = true
				}
			}

			c.Header("Content-Type", "application/pdf")
			c.Header("Content-Disposition", "inline; filename="+filename)
			c.Header("X-Frame-Options", "ALLOWALL")
			c.Header("Access-Control-Allow-Origin", "*")
			c.Header("Accept-Ranges", "bytes") // Required for PDF.js

			if localFileExists {
				// Use http.ServeFile to support Range requests
				http.ServeFile(c.Writer, c.Request, filePath)
				return
			}

			// If not found locally, fetch from remote server
			remoteURL := config.WebPanel.Get().App.WebPublicURL + c.Request.URL.Path
			resp, err := http.Get(remoteURL)
			if err != nil {
				logrus.Errorf("Error fetching remote PDF: %v", err)
				c.String(http.StatusNotFound, "PDF not found locally or remotely")
				return
			}
			defer resp.Body.Close()
			if resp.StatusCode != http.StatusOK {
				logrus.Errorf("Remote PDF returned status: %v", resp.Status)
				c.String(http.StatusNotFound, "PDF not found locally or remotely")
				return
			}
			c.Stream(func(w io.Writer) bool {
				_, err := io.Copy(w, resp.Body)
				return err == nil
			})
		})
	}

	proxyPDFSlipGajiTeknisi := router.Group(fun.GLOBAL_URL + "proxy-pdf-slip-gaji-teknisi")
	{
		proxyPDFSlipGajiTeknisi.GET("/*filepath", func(c *gin.Context) {
			filepathParam := c.Param("filepath") // e.g. web/file/payslip_technician/2025-11-07/[EDC]SlipGaji_2.5 Tangsel Herman Indra_07Nov2025.pdf
			filename := path.Base(filepathParam)

			spTechnicianMainDir, err := fun.FindValidDirectory([]string{
				"web/file/payslip_technician/",
				"../web/file/payslip_technician/",
				"../../web/file/payslip_technician/",
			})
			var filePath string
			var localFileExists bool = false
			if err == nil {
				filePath = filepath.Join(spTechnicianMainDir, filepathParam)
				if _, err := os.Stat(filePath); err == nil {
					localFileExists = true
				}
			}

			c.Header("Content-Type", "application/pdf")
			c.Header("Content-Disposition", "inline; filename="+filename)
			c.Header("X-Frame-Options", "ALLOWALL")
			c.Header("Access-Control-Allow-Origin", "*")
			c.Header("Accept-Ranges", "bytes") // Required for PDF.js

			if localFileExists {
				// Use http.ServeFile to support Range requests
				http.ServeFile(c.Writer, c.Request, filePath)
				return
			}

			// If not found locally, fetch from remote server
			remoteURL := config.WebPanel.Get().App.WebPublicURL + c.Request.URL.Path
			resp, err := http.Get(remoteURL)
			if err != nil {
				logrus.Errorf("Error fetching remote PDF: %v", err)
				c.String(http.StatusNotFound, "PDF not found locally or remotely")
				return
			}
			defer resp.Body.Close()
			if resp.StatusCode != http.StatusOK {
				logrus.Errorf("Remote PDF returned status: %v", resp.Status)
				c.String(http.StatusNotFound, "PDF not found locally or remotely")
				return
			}
			c.Stream(func(w io.Writer) bool {
				_, err := io.Copy(w, resp.Body)
				return err == nil
			})
		})
	}

	proxyMP3SPTechnician := router.Group(fun.GLOBAL_URL + "proxy-mp3-sp-technician")
	{
		proxyMP3SPTechnician.GET("/*filepath", func(c *gin.Context) {
			filepathParam := c.Param("filepath") // e.g. "/2025-09-03/sound1.mp3"
			filename := path.Base(filepathParam)

			spTechnicianMainDir, err := fun.FindValidDirectory([]string{
				"web/file/sounding_sp_technician/",
				"../web/file/sounding_sp_technician/",
				"../../web/file/sounding_sp_technician/",
			})
			var filePath string
			var localFileExists bool = false
			if err == nil {
				filePath = filepath.Join(spTechnicianMainDir, filepathParam)
				if _, err := os.Stat(filePath); err == nil {
					localFileExists = true
				}
			}

			c.Header("Content-Type", "audio/mpeg")
			c.Header("Content-Disposition", "inline; filename="+filename)
			c.Header("X-Frame-Options", "ALLOWALL")
			c.Header("Access-Control-Allow-Origin", "*")

			if localFileExists {
				// Use http.ServeFile to support Range requests
				http.ServeFile(c.Writer, c.Request, filePath)
				return
			}

			// If not found locally, fetch from remote server
			remoteURL := config.WebPanel.Get().App.WebPublicURL + c.Request.URL.Path
			resp, err := http.Get(remoteURL)
			if err != nil {
				logrus.Errorf("Error fetching remote MP3: %v", err)
				c.String(http.StatusNotFound, "MP3 not found locally or remotely")
				return
			}
			defer resp.Body.Close()
			if resp.StatusCode != http.StatusOK {
				logrus.Errorf("Remote MP3 returned status: %v", resp.Status)
				c.String(http.StatusNotFound, "MP3 not found locally or remotely")
				return
			}
			c.Stream(func(w io.Writer) bool {
				_, err := io.Copy(w, resp.Body)
				return err == nil
			})
		})
	}

	proxyMP3SPSPL := router.Group(fun.GLOBAL_URL + "proxy-mp3-sp-spl")
	{
		proxyMP3SPSPL.GET("/*filepath", func(c *gin.Context) {
			filepathParam := c.Param("filepath") // e.g. "/2025-09-03/sound1.mp3"
			filename := path.Base(filepathParam)

			spTechnicianMainDir, err := fun.FindValidDirectory([]string{
				"web/file/sounding_sp_spl/",
				"../web/file/sounding_sp_spl/",
				"../../web/file/sounding_sp_spl/",
			})
			var filePath string
			var localFileExists bool = false
			if err == nil {
				filePath = filepath.Join(spTechnicianMainDir, filepathParam)
				if _, err := os.Stat(filePath); err == nil {
					localFileExists = true
				}
			}

			c.Header("Content-Type", "audio/mpeg")
			c.Header("Content-Disposition", "inline; filename="+filename)
			c.Header("X-Frame-Options", "ALLOWALL")
			c.Header("Access-Control-Allow-Origin", "*")

			if localFileExists {
				// Use http.ServeFile to support Range requests
				http.ServeFile(c.Writer, c.Request, filePath)
				return
			}

			// If not found locally, fetch from remote server
			remoteURL := config.WebPanel.Get().App.WebPublicURL + c.Request.URL.Path
			resp, err := http.Get(remoteURL)
			if err != nil {
				logrus.Errorf("Error fetching remote MP3: %v", err)
				c.String(http.StatusNotFound, "MP3 not found locally or remotely")
				return
			}
			defer resp.Body.Close()
			if resp.StatusCode != http.StatusOK {
				logrus.Errorf("Remote MP3 returned status: %v", resp.Status)
				c.String(http.StatusNotFound, "MP3 not found locally or remotely")
				return
			}
			c.Stream(func(w io.Writer) bool {
				_, err := io.Copy(w, resp.Body)
				return err == nil
			})
		})
	}

	proxyMP3SPSAC := router.Group(fun.GLOBAL_URL + "proxy-mp3-sp-sac")
	{
		proxyMP3SPSAC.GET("/*filepath", func(c *gin.Context) {
			filepathParam := c.Param("filepath") // e.g. "/2025-09-03/sound1.mp3"
			filename := path.Base(filepathParam)

			spTechnicianMainDir, err := fun.FindValidDirectory([]string{
				"web/file/sounding_sp_sac/",
				"../web/file/sounding_sp_sac/",
				"../../web/file/sounding_sp_sac/",
			})
			var filePath string
			var localFileExists bool = false
			if err == nil {
				filePath = filepath.Join(spTechnicianMainDir, filepathParam)
				if _, err := os.Stat(filePath); err == nil {
					localFileExists = true
				}
			}

			c.Header("Content-Type", "audio/mpeg")
			c.Header("Content-Disposition", "inline; filename="+filename)
			c.Header("X-Frame-Options", "ALLOWALL")
			c.Header("Access-Control-Allow-Origin", "*")

			if localFileExists {
				// Use http.ServeFile to support Range requests
				http.ServeFile(c.Writer, c.Request, filePath)
				return
			}

			// If not found locally, fetch from remote server
			remoteURL := config.WebPanel.Get().App.WebPublicURL + c.Request.URL.Path
			resp, err := http.Get(remoteURL)
			if err != nil {
				logrus.Errorf("Error fetching remote MP3: %v", err)
				c.String(http.StatusNotFound, "MP3 not found locally or remotely")
				return
			}
			defer resp.Body.Close()
			if resp.StatusCode != http.StatusOK {
				logrus.Errorf("Remote MP3 returned status: %v", resp.Status)
				c.String(http.StatusNotFound, "MP3 not found locally or remotely")
				return
			}
			c.Stream(func(w io.Writer) bool {
				_, err := io.Copy(w, resp.Body)
				return err == nil
			})
		})
	}

	// Contract Technician File Proxy
	proxyPDFContractTechnician := router.Group(fun.GLOBAL_URL + "proxy-pdf-contract-technician")
	{
		proxyPDFContractTechnician.GET("/*filepath", func(c *gin.Context) {
			filepathParam := c.Param("filepath") // e.g. "/Surat Kontrak_Bryan_Adam_27Oct2025.pdf"
			filename := path.Base(filepathParam)

			mainDir, err := fun.FindValidDirectory([]string{
				"web/file/contract_technician/",
				"../web/file/contract_technician/",
				"../../web/file/contract_technician/",
			})
			var filePath string
			var localFileExists bool = false
			if err == nil {
				filePath = filepath.Join(mainDir, filepathParam)
				if _, err := os.Stat(filePath); err == nil {
					localFileExists = true
				}
			}

			c.Header("Content-Type", "application/pdf")
			c.Header("Content-Disposition", "inline; filename="+filename)
			c.Header("X-Frame-Options", "ALLOWALL")
			c.Header("Access-Control-Allow-Origin", "*")
			c.Header("Accept-Ranges", "bytes") // Required for PDF.js

			if localFileExists {
				// Use http.ServeFile to support Range requests
				http.ServeFile(c.Writer, c.Request, filePath)
				return
			}

			// If not found locally, fetch from remote server
			remoteURL := config.WebPanel.Get().App.WebPublicURL + c.Request.URL.Path
			resp, err := http.Get(remoteURL)
			if err != nil {
				logrus.Errorf("Error fetching remote PDF: %v", err)
				c.String(http.StatusNotFound, "PDF not found locally or remotely")
				return
			}
			defer resp.Body.Close()
			if resp.StatusCode != http.StatusOK {
				logrus.Errorf("Remote PDF returned status: %v", resp.Status)
				c.String(http.StatusNotFound, "PDF not found locally or remotely")
				return
			}
			c.Stream(func(w io.Writer) bool {
				_, err := io.Copy(w, resp.Body)
				return err == nil
			})
		})
	}

	// Proxy SP Stock Opname
	proxyPDFSPStockOpname := router.Group(fun.GLOBAL_URL + "proxy-pdf-sp-so")
	{
		proxyPDFSPStockOpname.GET("/*filepath", func(c *gin.Context) {
			filepathParam := c.Param("filepath") // e.g. "/2025-09-03/SP_1_1.1 Jakpus.pdf"
			filename := path.Base(filepathParam)

			spTechnicianMainDir, err := fun.FindValidDirectory([]string{
				"web/file/sp_so/",
				"../web/file/sp_so/",
				"../../web/file/sp_so/",
			})
			var filePath string
			var localFileExists bool = false
			if err == nil {
				filePath = filepath.Join(spTechnicianMainDir, filepathParam)
				if _, err := os.Stat(filePath); err == nil {
					localFileExists = true
				}
			}

			c.Header("Content-Type", "application/pdf")
			c.Header("Content-Disposition", "inline; filename="+filename)
			c.Header("X-Frame-Options", "ALLOWALL")
			c.Header("Access-Control-Allow-Origin", "*")
			c.Header("Accept-Ranges", "bytes") // Required for PDF.js

			if localFileExists {
				// Use http.ServeFile to support Range requests
				http.ServeFile(c.Writer, c.Request, filePath)
				return
			}

			// If not found locally, fetch from remote server
			remoteURL := config.WebPanel.Get().App.WebPublicURL + c.Request.URL.Path
			resp, err := http.Get(remoteURL)
			if err != nil {
				logrus.Errorf("Error fetching remote PDF: %v", err)
				c.String(http.StatusNotFound, "PDF not found locally or remotely")
				return
			}
			defer resp.Body.Close()
			if resp.StatusCode != http.StatusOK {
				logrus.Errorf("Remote PDF returned status: %v", resp.Status)
				c.String(http.StatusNotFound, "PDF not found locally or remotely")
				return
			}
			c.Stream(func(w io.Writer) bool {
				_, err := io.Copy(w, resp.Body)
				return err == nil
			})
		})
	}

	proxyMP3SPStockOpname := router.Group(fun.GLOBAL_URL + "proxy-mp3-sp-so")
	{
		proxyMP3SPStockOpname.GET("/*filepath", func(c *gin.Context) {
			filepathParam := c.Param("filepath") // e.g. "/2025-09-03/sound1.mp3"
			filename := path.Base(filepathParam)

			spTechnicianMainDir, err := fun.FindValidDirectory([]string{
				"web/file/sounding_sp_so/",
				"../web/file/sounding_sp_so/",
				"../../web/file/sounding_sp_so/",
			})
			var filePath string
			var localFileExists bool = false
			if err == nil {
				filePath = filepath.Join(spTechnicianMainDir, filepathParam)
				if _, err := os.Stat(filePath); err == nil {
					localFileExists = true
				}
			}

			c.Header("Content-Type", "audio/mpeg")
			c.Header("Content-Disposition", "inline; filename="+filename)
			c.Header("X-Frame-Options", "ALLOWALL")
			c.Header("Access-Control-Allow-Origin", "*")

			if localFileExists {
				// Use http.ServeFile to support Range requests
				http.ServeFile(c.Writer, c.Request, filePath)
				return
			}

			// If not found locally, fetch from remote server
			remoteURL := config.WebPanel.Get().App.WebPublicURL + c.Request.URL.Path
			resp, err := http.Get(remoteURL)
			if err != nil {
				logrus.Errorf("Error fetching remote MP3: %v", err)
				c.String(http.StatusNotFound, "MP3 not found locally or remotely")
				return
			}
			defer resp.Body.Close()
			if resp.StatusCode != http.StatusOK {
				logrus.Errorf("Remote MP3 returned status: %v", resp.Status)
				c.String(http.StatusNotFound, "MP3 not found locally or remotely")
				return
			}
			c.Stream(func(w io.Writer) bool {
				_, err := io.Copy(w, resp.Body)
				return err == nil
			})
		})
	}
}
