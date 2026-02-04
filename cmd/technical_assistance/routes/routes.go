package routes

import (
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"service-platform/cmd/technical_assistance/controllers"
	"service-platform/cmd/technical_assistance/fun"
	"service-platform/cmd/technical_assistance/middleware"
	"service-platform/cmd/technical_assistance/model"
	"service-platform/cmd/technical_assistance/model/cc_model"
	"service-platform/cmd/technical_assistance/model/op_model"
	"service-platform/internal/config"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/go-redis/redis/v8"
	"gorm.io/gorm"
)

func StaticFile(router *gin.Engine) {
	staticPath := config.TechnicalAssistance.Get().APP_STATIC_DIR
	publishedDir := config.TechnicalAssistance.Get().APP_PUBLISHED_DIR

	// Ensure the static path is absolute
	staticPath, err := filepath.Abs(staticPath)
	if err != nil {
		fmt.Println("Error getting absolute path:", err)
		return
	}

	// Load HTML assets
	router.LoadHTMLGlob(filepath.Join(staticPath, "**/*.html")) // GLOBAL HTML ASSETS

	// SERVE STATIC FILE
	if publishedDir != "" {
		var directories []string

		if strings.Contains(publishedDir, "|") {
			directories = strings.Split(publishedDir, "|")
		} else {
			directories = append(directories, publishedDir)
		}

		// Print the resulting slice of directories
		for _, dir := range directories {
			if strings.Contains(dir, "#") {
				break
			}

			// Remove any potential traversal sequences
			cleanDir := filepath.Clean(dir)
			cleanDir = strings.TrimPrefix(cleanDir, "..")
			cleanDir = strings.TrimPrefix(cleanDir, "/")
			cleanDir = strings.TrimPrefix(cleanDir, "\\")

			staticDirPath := filepath.Join(staticPath, cleanDir)
			if _, err := os.Stat(staticDirPath); os.IsNotExist(err) {
				fmt.Println("Directory does not exist:", staticDirPath)
				continue
			}

			router.Static(fun.GLOBAL_URL+cleanDir, staticDirPath)
			fmt.Println("WARN PUBLISHED DIR --> " + staticDirPath)
		}
	}
	router.Static("./filestore", "filestore")
	// router.Static("/wa_reply", "/home/user/server/odoo_wa/public/file/wa_reply") // dev
	router.Static("/wa_reply", "/home/administrator/odoo_wa/public/file/wa_reply") // prod

	//	router.Static("/uploads", "./uploads")

}
func HtmlRoutes(router *gin.Engine, db *gorm.DB, db_call_center *gorm.DB, db_pengerjaan *gorm.DB, redisDB *redis.Client) {
	router.GET(fun.GLOBAL_URL+"api/ping", func(c *gin.Context) {
		i := c.Query("i")
		if i != "" {
			c.JSON(http.StatusOK, gin.H{"message": "pong", "i": i})
		} else {
			c.JSON(http.StatusOK, gin.H{"message": "pong"})
		}
	})
	router.POST("/submit_cc_file", controllers.SubmitCcFile(db))
	router.POST("/submit_cc_image", controllers.SubmitCcImageFile(db_call_center))
	router.GET(fun.GLOBAL_URL+"ws", controllers.WebSocketVerify(db))
	// WS for Realtime Data
	// router.GET(fun.GLOBAL_URL+"ws-realtime", controllers.WebSocketRealtime())

	// WS for lock TA Checked Data
	router.GET(fun.GLOBAL_URL+"ws-lock", controllers.WebSocketLockData(redisDB, db_pengerjaan))

	// TA Report
	router.GET("/ta_report", controllers.GetTAReport(db_pengerjaan, db))
	router.GET("/send_report", controllers.SendReportHandler(db_pengerjaan, db))
	router.GET("/ta_monthly_report", controllers.GetTAMonthlyReport(db_pengerjaan))
	router.GET("/compared_report", controllers.GetTAComparedReport(db_pengerjaan))
	router.GET("/tech_error_report", controllers.GetTechErrorReport(db_pengerjaan, db))
	router.POST("/check_existing_status_id_in_table_ta", controllers.GetIDTaskStatusInTableTA(db_pengerjaan))

	// TA Feedback
	router.POST(fun.GLOBAL_URL+"ta_feedback", controllers.TAFeedback(redisDB, db_pengerjaan, db))

	// Photos
	// To read an "id" from the query string, use c.Query("id") in the controller and read the table data using query table
	router.GET(fun.GLOBAL_URL+"photos/:id", controllers.ShowPhotoByID(redisDB, db_pengerjaan))
	router.GET(fun.GLOBAL_URL+"ta_additional_data/:id", controllers.ShowAdditionalDataByID(redisDB, db_pengerjaan))

	// router.GET(fun.GLOBAL_URL+"", controllers.GetWebLandingPage(db)) // LANDING PAGE
	router.GET(fun.GLOBAL_URL, func(c *gin.Context) { c.Redirect(http.StatusPermanentRedirect, fun.GLOBAL_URL+"login") })

	router.GET(fun.GLOBAL_URL+"login", controllers.GetWebLogin(db)) // WEB LOGIN
	// SEND LOGIN CREDENTIALS
	router.POST(fun.GLOBAL_URL+"login", controllers.PostWebLogin(db, redisDB))

	router.GET(fun.GLOBAL_URL+"captcha", controllers.GetCaptchaImage())

	router.GET(fun.GLOBAL_URL+"forgot-password", controllers.GetWebForgotPassword(db))
	router.POST(fun.GLOBAL_URL+"forgot-password", controllers.PostForgotPassword(db, redisDB))
	router.GET(fun.GLOBAL_URL+"reset-password/:email/:token_data", controllers.GetWebResetPassword(db, redisDB))
	router.POST(fun.GLOBAL_URL+"reset-password/:email/:token_data", controllers.PostResetPassword(db, redisDB))

	//MAIN PAGE
	router.GET(fun.GLOBAL_URL+"page", controllers.MainPage(db, redisDB))

	// LOGOUT BY BUTTON
	router.GET(fun.GLOBAL_URL+"logout", controllers.GetWebLogout(db))
	router.Any("/here/*path", controllers.PostHere())
	// router.GET(fun.GLOBAL_URL+"register", controllers.getRegister(db))

	router.GET(fun.GLOBAL_URL+"profile/default.jpg", controllers.GetUserProfile(db))
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
		tabUploadedFile := web.Group("/tab-uploaded-file")
		{
			tabUploadedFile.POST("/table", controllers.TableUploadedFile(db))
			tabUploadedFile.GET("/table.csv", controllers.ExportTable[model.UploadedFiles](db, "File di unggah"))
		}
		tabCcMerchantCallLog := web.Group("/tab-cc-merchant-call-log")
		{
			tabCcMerchantCallLog.POST("/table", controllers.TableMerchantH_1CallLog(db_call_center))
			tabCcMerchantCallLog.GET("/table.csv", controllers.ExportTable[cc_model.JOMerchantHmin1CallLog](db_call_center, "Call Log"))
		}
		tabKonfirmasiDataPending := web.Group("/tab-konfirmasi-data-pending")
		{
			tabKonfirmasiDataPending.POST("/table", controllers.TablePengerjaanTeknisiPending(db_pengerjaan, db))
			tabKonfirmasiDataPending.GET("/table.csv", controllers.ExportTable[op_model.Pending](db_pengerjaan, "Konfirmasi Data Pengerjaan Teknisi Pending"))
		}
		tabKonfirmasiDataError := web.Group("/tab-konfirmasi-data-error")
		{
			tabKonfirmasiDataError.POST("/table", controllers.TablePengerjaanTeknisiError(db_pengerjaan, db))
			tabKonfirmasiDataError.GET("/table.csv", controllers.ExportTable[op_model.Error](db_pengerjaan, "Konfirmasi Data Pengerjaan Teknisi Error"))
		}
		tabKonfirmasiDataSubmission := web.Group("/tab-konfirmasi-data-submission")
		{
			tabKonfirmasiDataSubmission.POST("/table", controllers.TablePengerjaanTeknisiSubmission(db_pengerjaan, db))
			tabKonfirmasiDataSubmission.GET("/table.csv", controllers.ExportTable[op_model.TempSubmission](db_pengerjaan, "Konfirmasi Data Pengerjaan Teknisi Submission"))
		}
		tabLogAct := web.Group("/tab-log-act")
		{
			tabLogAct.POST("/table", controllers.TabelPengerjaanLogActivity(db_pengerjaan))
			tabLogAct.GET("/table.csv", controllers.ExportTable[op_model.LogAct](db_pengerjaan, "Activity Technical Assistance"))
			tabLogAct.POST("/table2", controllers.TabelDataFotoError(db_pengerjaan))
			tabLogAct.GET("/table2.csv", controllers.ExportTable[op_model.Error](db_pengerjaan, "Data Foto Error"))
			tabLogAct.POST("/table3", controllers.TabelDataPending(db_pengerjaan))
			tabLogAct.GET("/table3.csv", controllers.ExportTable[op_model.Pending](db_pengerjaan, "Data Pending"))
		}
		tabTeknisi := web.Group("/tab-teknisi")
		{
			tabTeknisi.POST("/teknisi/serial_number/unlock", controllers.PostUnlockSerialNumber(db))

			tabTeknisi.GET("/teknisi/name", controllers.ListTeknisiName(db))
			tabTeknisi.GET("/teknisi/serial_number", controllers.ListSerialNumber(db))
			tabTeknisi.GET("/teknisi/app", controllers.ListNamaAplikasi(db))
			tabTeknisi.GET("/teknisi/app/ver", controllers.ListVersiAplikasi(db))
			tabTeknisi.GET("/teknisi/app/tid_mid", controllers.GetTidMid(db))

			tabTeknisi.GET("/teknisi/app/info", controllers.GetSnInfo(db))

			tabTeknisi.POST("/kunjungan/table", controllers.PostKunjunganTeknisiList(db))
			tabTeknisi.GET("/kunjungan/table.csv", controllers.ExportTable[model.TeknisiKunjungan](db, "Kunjungan Teknisi"))

			tabTeknisi.POST("/teknisi/table", controllers.PostTeknisiList(db))
			tabTeknisi.GET("/teknisi/table.csv", controllers.ExportTable[model.Teknisi](db, "Teknisi"))
			// tabTeknisi.PUT("/teknisi/table", controllers.PutTeknisiList(db))
			tabTeknisi.GET("/teknisi/table/batch/template", controllers.GetBatchTemplate[model.Teknisi](db))
			tabTeknisi.POST("/teknisi/table/create", controllers.PostNewTeknisi(db))
			tabTeknisi.POST("/teknisi/table/batch/create", controllers.PostBatchUpload[model.Teknisi](db))
			tabTeknisi.PUT("/teknisi/table", controllers.PutTeknisiList(db))
			tabTeknisi.PATCH("/teknisi/table", controllers.UpdatePatchTeknisi(db))
			tabTeknisi.DELETE("/teknisi/table/:id", controllers.DeleteTeknisi(db))
			// tabTeknisi.DELETE("/teknisi/batch", controllers.DeleteBatchTeknisi(db))
			tabTeknisi.GET("/teknisi/maps", controllers.GetTeknisiMaps(db))
		}
		tabTrackTeknisi := web.Group("/tab-track-teknisi")
		{
			tabTrackTeknisi.GET("/teknisi/table", controllers.GetTeknisiList(db))
			tabTrackTeknisi.POST("/teknisi/table", controllers.PostTeknisiList(db))
			tabTrackTeknisi.GET("/teknisi/table/batch/template", controllers.GetBatchTemplate[model.Teknisi](db))
			tabTrackTeknisi.POST("/teknisi/table/create", controllers.PostNewTeknisi(db))
			tabTrackTeknisi.POST("/teknisi/table/batch/create", controllers.PostBatchUpload[model.Teknisi](db))
			tabTrackTeknisi.PUT("/teknisi/table", controllers.PutTeknisiList(db))
			tabTrackTeknisi.PATCH("/teknisi/table", controllers.UpdatePatchTeknisi(db))
			tabTrackTeknisi.DELETE("/teknisi/table/:id", controllers.DeleteTeknisi(db))
			// tabTrackTeknisi.DELETE("/teknisi/batch", controllers.DeleteBatchTeknisi(db))
			tabTrackTeknisi.GET("/teknisi/maps", controllers.GetTeknisiMaps(db))
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
		{ // /web/tab-system-log
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

}
