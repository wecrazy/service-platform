package controllers

import (
	"encoding/json"
	"fmt"
	"html/template"
	"net/http"
	"service-platform/cmd/web_panel/config"
	"service-platform/cmd/web_panel/fun"
	"service-platform/cmd/web_panel/model"
	"service-platform/cmd/web_panel/webguibuilder"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/go-redis/redis/v8"
	"github.com/sirupsen/logrus"
	"gorm.io/gorm"
)

func ComponentPage(db *gorm.DB, redisDB *redis.Client) gin.HandlerFunc {
	return func(c *gin.Context) {
		cookies := c.Request.Cookies()

		// Parse JWT token from cookie
		tokenString, err := c.Cookie("token")
		if err != nil {
			fun.ClearCookiesAndRedirect(c, cookies)
			return
		}
		tokenString = strings.ReplaceAll(tokenString, " ", "+")

		decrypted, err := fun.GetAESDecrypted(tokenString)
		if err != nil {
			logrus.Error("Error during decryption:", err)
			fun.ClearCookiesAndRedirect(c, cookies)
			return
		}
		var claims map[string]interface{}
		err = json.Unmarshal(decrypted, &claims)
		if err != nil {
			logrus.Error("Error converting JSON to map:", err)
			fun.ClearCookiesAndRedirect(c, cookies)
			return
		}
		componentID := c.Param("component")
		componentID = strings.ReplaceAll(componentID, "/", "")
		componentID = strings.ReplaceAll(componentID, "..", "")
		componentPrv, ok := claims[componentID]
		if !ok {
			fun.ClearCookiesAndRedirect(c, cookies)
			return
		}
		componentPrvStr, ok := componentPrv.(string)
		if !ok || componentPrvStr == "" {
			fun.ClearCookiesAndRedirect(c, cookies)
			return
		}
		if string(componentPrvStr[1:2]) != "1" {
			fun.ClearCookiesAndRedirect(c, cookies)
			return
		}
		var admin model.Admin
		db.Where("id = ?", uint(claims["id"].(float64))).Find(&admin)

		imageMaps := map[string]interface{}{
			"t":  fun.GenerateRandomString(3),
			"id": admin.ID,
		}
		pathString, err := fun.GetAESEcryptedURLfromJSON(imageMaps)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Could not encripting image " + err.Error()})
			return
		}
		profile_image := "/profile/default.jpg?f=" + pathString

		var adminStatusData model.AdminStatus
		if err := db.First(&adminStatusData, admin.Status).Error; err != nil {
			logrus.Errorf("failed to parse data status for admin: %v", err)
		}
		var sb strings.Builder
		sb.WriteString(fmt.Sprintf("<div class='%s'>", adminStatusData.ClassName))
		sb.WriteString(adminStatusData.Title)
		sb.WriteString("</div>")
		adminStatusHTML := sb.String()

		// Check if admin role has specific app configuration
		var appConfig model.AppConfig
		var appName, appLogo, appVersion, appVersionNo, appVersionCode, appVersionName string

		if err := db.Where("role_id = ? AND is_active = ?", admin.Role, true).First(&appConfig).Error; err == nil {
			// Use role-specific app configuration
			appName = appConfig.AppName
			appLogo = appConfig.AppLogo
			appVersion = appConfig.AppVersion
			appVersionNo = appConfig.VersionNo
			appVersionCode = appConfig.VersionCode
			appVersionName = appConfig.VersionName
			// logrus.Infof("Using role-specific app config for role %d: %s", admin.Role, appConfig.AppName)
		} else {
			// Fallback to default config
			appName = config.GetConfig().App.Name
			appLogo = config.GetConfig().App.Logo
			appVersion = config.GetConfig().App.Version
			appVersionNo = strconv.Itoa(config.GetConfig().App.VersionNo)
			appVersionCode = config.GetConfig().App.VersionCode
			appVersionName = config.GetConfig().App.VersionName
			// logrus.Infof("Using default app config for role %d (no specific config found)", admin.Role)
		}

		replacements := map[string]any{
			"APP_NAME":         appName,
			"APP_LOGO":         appLogo,
			"APP_VERSION":      appVersion,
			"APP_VERSION_NO":   appVersionNo,
			"APP_VERSION_CODE": appVersionCode,
			"APP_VERSION_NAME": appVersionName,
			"fullname":         admin.Fullname,
			"username":         admin.Username,
			"userid":           admin.ID,
			"phone":            admin.Phone,
			"email":            admin.Email,
			"role_name":        claims["role_name"].(string),
			// "status_name":      claims["status_name"].(string),
			"status_name":    template.HTML(adminStatusHTML),
			"last_login":     claims["last_login"].(string),
			"created_at_str": claims["created_at_str"].(string),
			"profile_image":  profile_image,
			"ip":             admin.IP,
			"GLOBAL_URL":     fun.GLOBAL_URL,
			/* Whatsmeow */
			"REFRESH_WHATSAPP_QRCODE":                      fun.GLOBAL_URL + "web/" + fun.GetRedis("web:"+admin.Session, redisDB) + "/tab-whatsapp/refresh-qrcode",
			"QR_CODE":                                      "log-data/qrcode.txt",
			"PING_BOT":                                     fun.GLOBAL_URL + "web/" + fun.GetRedis("web:"+admin.Session, redisDB) + "/tab-whatsapp/ping",
			"SEND_TEXT_BOT":                                fun.GLOBAL_URL + "web/" + fun.GetRedis("web:"+admin.Session, redisDB) + "/tab-whatsapp/send_text",
			"SEND_IMAGE_BOT":                               fun.GLOBAL_URL + "web/" + fun.GetRedis("web:"+admin.Session, redisDB) + "/tab-whatsapp/send_image",
			"SEND_DOCUMENT_BOT":                            fun.GLOBAL_URL + "web/" + fun.GetRedis("web:"+admin.Session, redisDB) + "/tab-whatsapp/send_document",
			"SEND_LOCATION_BOT":                            fun.GLOBAL_URL + "web/" + fun.GetRedis("web:"+admin.Session, redisDB) + "/tab-whatsapp/send_location",
			"SEND_POLLING_BOT":                             fun.GLOBAL_URL + "web/" + fun.GetRedis("web:"+admin.Session, redisDB) + "/tab-whatsapp/send_polling",
			"WAG_JSON":                                     fun.GLOBAL_URL + "web/" + fun.GetRedis("web:"+admin.Session, redisDB) + "/tab-whatsapp/groups",
			"TABLE_WHATSAPP_BOT_LANGUAGE":                  webguibuilder.TABLE_WHATSAPP_BOT_LANGUAGE(admin.Session, redisDB, db),
			"TABLE_WHATSAPP_BOT_MESSAGE_REPLY":             webguibuilder.TABLE_WHATSAPP_BOT_MESSAGE_REPLY(admin.Session, redisDB, db),
			"TABLE_WHATSAPP_USER_MANAGEMENT":               webguibuilder.TABLE_WHATSAPP_USER_MANAGEMENT(admin.Session, redisDB, db),
			"END_SESSION_WHATSAPP":                         fun.GLOBAL_URL + "web/" + fun.GetRedis("web:"+admin.Session, redisDB) + "/tab-whatsapp/end-session",
			"TABLE_WHATSAPP_BOT_LOG_MSG_RECEIVED":          webguibuilder.TABLE_WHATSAPP_BOT_LOG_MSG_RECEIVED(admin.Session, redisDB, db),
			"ENDPOINT_TABLE_WHATSAPP_BOT_LOG_MSG_RECEIVED": fun.GLOBAL_URL + "web/" + fun.GetRedis("web:"+admin.Session, redisDB) + "/tab-whatsapp/wa_log_msg_received",
			"RESET_QUOTA_PROMPT":                           fun.GLOBAL_URL + "web/" + fun.GetRedis("web:"+admin.Session, redisDB) + "/tab-whatsapp-user-management/reset_quota_prompt",
			"UNBAN_USER_ENDPOINT":                          fun.GLOBAL_URL + "web/" + fun.GetRedis("web:"+admin.Session, redisDB) + "/tab-whatsapp-user-management/unban_user",

			/* Ticket */
			"REFRESH_TABLE_DATA_TICKET": fun.GLOBAL_URL + "web/" + fun.GetRedis("web:"+admin.Session, redisDB) + "/tab-hommy-pay-cc-ticket/refresh",
			"TABLE_DATA_TICKET":         webguibuilder.TABLE_DATA_TICKET(admin.Session, redisDB, db),
			"LAST_UPDATE_TICKET_DATA":   fun.GLOBAL_URL + "web/" + fun.GetRedis("web:"+admin.Session, redisDB) + "/tab-hommy-pay-cc-ticket/last_update",
			"REPORT_ALL":                fun.GLOBAL_URL + "web/" + fun.GetRedis("web:"+admin.Session, redisDB) + "/tab-hommy-pay-cc-ticket/report_all",
			"REPORT_DATA_FILTERED":      fun.GLOBAL_URL + "web/" + fun.GetRedis("web:"+admin.Session, redisDB) + "/tab-hommy-pay-cc-ticket/report_data_filtered",
			/* Merchant */
			"REFRESH_TABLE_MERCHANT":  fun.GLOBAL_URL + "web/" + fun.GetRedis("web:"+admin.Session, redisDB) + "/tab-hommy-pay-cc-merchant/refresh",
			"TABLE_MERCHANT":          webguibuilder.TABLE_MERCHANT(admin.Session, redisDB),
			"TABLE_MERCHANT_FASTLINK": webguibuilder.TABLE_MERCHANT_FASTLINK(admin.Session, redisDB),

			"REFRESH_TABLE_MERCHANT_KRESEKBAG": fun.GLOBAL_URL + "web/" + fun.GetRedis("web:"+admin.Session, redisDB) + "/tab-hommy-pay-cc-merchant/refresh_kresekbag",
			"TABLE_MERCHANT_KRESEKBAG":         webguibuilder.TABLE_MERCHANT_KRESEKBAG(admin.Session, redisDB),
			"PHOTOS_MERCHANT_KRESEKBAG":        fun.GLOBAL_URL + "web/" + fun.GetRedis("web:"+admin.Session, redisDB) + "/tab-hommy-pay-cc-merchant/photos/merchantKresekBag",

			/* App Config */
			"TABLE_APP_CONFIGURATION": webguibuilder.TABLE_APP_CONFIGURATION(admin.Session, redisDB),

			/* ODOO Manage Service */
			"UPLOADED_TIMEOUT_SECONDS":        config.GetConfig().UploadedExcelForODOOMS.Timeout,
			"UPLOADED_MAX_FILE_SIZE_MB":       config.GetConfig().UploadedExcelForODOOMS.MaxFileSize,
			"UPDATE_TICKET_ODOO_MS_URL":       fun.GLOBAL_URL + "web/" + fun.GetRedis("web:"+admin.Session, redisDB) + "/tab-odoo-ms-upload-excel/upload",
			"UPLOAD_NEW_TICKET_ODOO_MS_URL":   fun.GLOBAL_URL + "web/" + fun.GetRedis("web:"+admin.Session, redisDB) + "/tab-odoo-ms-upload-excel/upload_new_ticket",
			"UPLOAD_HISTORY_URL":              fun.GLOBAL_URL + "web/" + fun.GetRedis("web:"+admin.Session, redisDB) + "/tab-odoo-ms-upload-excel/history/table",
			"UPLOAD_BA_LOST_URL":              fun.GLOBAL_URL + "web/" + fun.GetRedis("web:"+admin.Session, redisDB) + "/tab-odoo-ms-upload-excel/upload_ba_lost",
			"UPLOAD_TECHNICIAN_PAYROLL_URL":   fun.GLOBAL_URL + "web/" + fun.GetRedis("web:"+admin.Session, redisDB) + "/tab-odoo-ms-upload-excel/upload_technician_payroll",
			"UPDATE_PROJECT_TASK_ODOO_MS_URL": fun.GLOBAL_URL + "web/" + fun.GetRedis("web:"+admin.Session, redisDB) + "/tab-odoo-ms-upload-excel/upload_updated_task",
			"VALIDATE_ODOO_CREDENTIALS_URL":   fun.GLOBAL_URL + "web/" + fun.GetRedis("web:"+admin.Session, redisDB) + "/tab-odoo-ms-upload-excel/validate-credentials",

			/* Whatsapp Conversation */
			"IsUserWhatsappLoggedInEndpoint":  fun.GLOBAL_URL + "web/" + fun.GetRedis("web:"+admin.Session, redisDB) + "/tab-whatsapp-conversation/is_user_logged_in/" + strconv.Itoa(int(admin.ID)),
			"CheckUserWAStatusEndpoint":       fun.GLOBAL_URL + "web/" + fun.GetRedis("web:"+admin.Session, redisDB) + "/tab-whatsapp-conversation/" + strconv.Itoa(int(admin.ID)) + "/status",
			"GetDetailedUserWAStatusEndpoint": fun.GLOBAL_URL + "web/" + fun.GetRedis("web:"+admin.Session, redisDB) + "/tab-whatsapp-conversation/" + strconv.Itoa(int(admin.ID)) + "/status/detailed",
			"ConnectUserWAEndpoint":           fun.GLOBAL_URL + "web/" + fun.GetRedis("web:"+admin.Session, redisDB) + "/tab-whatsapp-conversation/" + strconv.Itoa(int(admin.ID)) + "/connect",
			"DisconnectUserWAEndpoint":        fun.GLOBAL_URL + "web/" + fun.GetRedis("web:"+admin.Session, redisDB) + "/tab-whatsapp-conversation/" + strconv.Itoa(int(admin.ID)) + "/disconnect",
			"GetUserWAQREndpoint":             fun.GLOBAL_URL + "web/" + fun.GetRedis("web:"+admin.Session, redisDB) + "/tab-whatsapp-conversation/" + strconv.Itoa(int(admin.ID)) + "/qr",
			"RefreshUserWAQREndpoint":         fun.GLOBAL_URL + "web/" + fun.GetRedis("web:"+admin.Session, redisDB) + "/tab-whatsapp-conversation/" + strconv.Itoa(int(admin.ID)) + "/qr/refresh",
			"CommonWAEndpoint":                fun.GLOBAL_URL + "web/" + fun.GetRedis("web:"+admin.Session, redisDB) + "/tab-whatsapp-conversation/" + strconv.Itoa(int(admin.ID)) + "/",

			/* CSNA Human Resource */
			// SP
			"TABLE_DATA_SP_TECHNICIAN": webguibuilder.TABLE_DATA_SP_TECHNICIAN(admin.Session, redisDB),
			"TABLE_DATA_SP_SPL":        webguibuilder.TABLE_DATA_SP_SPL(admin.Session, redisDB),
			"TABLE_DATA_SP_SAC":        webguibuilder.TABLE_DATA_SP_SAC(admin.Session, redisDB), "DELETE_ALL_SP_TECHNICIAN": fun.GLOBAL_URL + "web/" + fun.GetRedis("web:"+admin.Session, redisDB) + "/tab-hr-sp/delete_all_sp_technician",
			"DELETE_ALL_SP_SPL":              fun.GLOBAL_URL + "web/" + fun.GetRedis("web:"+admin.Session, redisDB) + "/tab-hr-sp/delete_all_sp_spl",
			"DELETE_ALL_SP_SAC":              fun.GLOBAL_URL + "web/" + fun.GetRedis("web:"+admin.Session, redisDB) + "/tab-hr-sp/delete_all_sp_sac",
			"SP_TECHNICIAN_COUNT":            webguibuilder.GET_SP_TECHNICIAN_COUNT(),
			"SP_SPL_COUNT":                   webguibuilder.GET_SP_SPL_COUNT(),
			"SP_SAC_COUNT":                   webguibuilder.GET_SP_SAC_COUNT(),
			"GET_SAC_GROUPS_URL":             fun.GLOBAL_URL + "web/" + fun.GetRedis("web:"+admin.Session, redisDB) + "/tab-hr-sp/get_sac_groups",
			"DOWNLOAD_SP_REPLY_TEMPLATE_URL": fun.GLOBAL_URL + "web/" + fun.GetRedis("web:"+admin.Session, redisDB) + "/tab-hr-sp/download_sp_reply_template",
			"UPLOAD_SP_REPLY_SIMULATION_URL": fun.GLOBAL_URL + "web/" + fun.GetRedis("web:"+admin.Session, redisDB) + "/tab-hr-sp/upload_sp_reply_simulation",
			// Kontrak
			"TABLE_DATA_KONTRAK_TEKNISI":                    webguibuilder.TABLE_DATA_KONTRAK_TEKNISI(admin.Session, redisDB),
			"REFRESH_TABLE_DATA_CONTRACT_TECHNICIAN":        fun.GLOBAL_URL + "web/" + fun.GetRedis("web:"+admin.Session, redisDB) + "/tab-hr-kontrak-teknisi/refresh",
			"REGENERATE_PDF_CONTRACT_TECHNICIAN":            fun.GLOBAL_URL + "web/" + fun.GetRedis("web:"+admin.Session, redisDB) + "/tab-hr-kontrak-teknisi/regenerate_pdf_contract",
			"SEND_INDIVIDUAL_CONTRACT_TECHNICIAN":           fun.GLOBAL_URL + "web/" + fun.GetRedis("web:"+admin.Session, redisDB) + "/tab-hr-kontrak-teknisi/send_individual_contract",
			"GET_CONTRACT_TECHNICIAN_WHATSAPP_CONVERSATION": fun.GLOBAL_URL + "web/" + fun.GetRedis("web:"+admin.Session, redisDB) + "/tab-hr-kontrak-teknisi/get_contract_technician_whatsapp_conversation",
			"SEND_ALL_CONTRACT_TECHNICIAN":                  fun.GLOBAL_URL + "web/" + fun.GetRedis("web:"+admin.Session, redisDB) + "/tab-hr-kontrak-teknisi/send_all_contract",
			// Slip Gaji
			"TABLE_SLIP_GAJI_TEKNISI_EDC":       webguibuilder.TABLE_SLIP_GAJI_TEKNISI_EDC(admin.Session, redisDB),
			"TABLE_SLIP_GAJI_TEKNISI_ATM":       webguibuilder.TABLE_SLIP_GAJI_TEKNISI_ATM(admin.Session, redisDB),
			"SEND_INDIVIDUAL_PAYSLIP":           fun.GLOBAL_URL + "web/" + fun.GetRedis("web:"+admin.Session, redisDB) + "/tab-hr-slip-gaji-teknisi/send_individual_payslip",
			"SEND_ALL_PAYSLIP":                  fun.GLOBAL_URL + "web/" + fun.GetRedis("web:"+admin.Session, redisDB) + "/tab-hr-slip-gaji-teknisi/send_all_payslip",
			"GET_PAYSLIP_WHATSAPP_CONVERSATION": fun.GLOBAL_URL + "web/" + fun.GetRedis("web:"+admin.Session, redisDB) + "/tab-hr-slip-gaji-teknisi/get_payslip_whatsapp_conversation",
			"REGENERATE_PAYSLIP_TECHNICIAN_EDC": fun.GLOBAL_URL + "web/" + fun.GetRedis("web:"+admin.Session, redisDB) + "/tab-hr-slip-gaji-teknisi/regenerate_payslip_edc",
			"REGENERATE_PAYSLIP_TECHNICIAN_ATM": fun.GLOBAL_URL + "web/" + fun.GetRedis("web:"+admin.Session, redisDB) + "/tab-hr-slip-gaji-teknisi/regenerate_payslip_atm",
			// SP Stock Opname
			"TABLE_DATA_SP_SO": webguibuilder.TABLE_DATA_SP_SO(admin.Session, redisDB),

			/* MTI */
			"TABLE_DATA_PM_MTI":               webguibuilder.TABLE_DATA_PM_MTI(admin.Session, redisDB),
			"TABLE_DATA_NON_PM_MTI":           webguibuilder.TABLE_DATA_NON_PM_MTI(admin.Session, redisDB),
			"REFRESH_TABLE_DATA_MTI":          fun.GLOBAL_URL + "tab-mti/refresh-task",
			"REPORT_ALL_PM_MTI":               fun.GLOBAL_URL + "web/" + fun.GetRedis("web:"+admin.Session, redisDB) + "/tab-mti-monitoring-pm/report_all_pm_mti",
			"REPORT_DATA_FILTERED_PM_MTI":     fun.GLOBAL_URL + "web/" + fun.GetRedis("web:"+admin.Session, redisDB) + "/tab-mti-monitoring-pm/report_data_filtered_pm_mti",
			"REPORT_ALL_NON_PM_MTI":           fun.GLOBAL_URL + "web/" + fun.GetRedis("web:"+admin.Session, redisDB) + "/tab-mti-monitoring-non-pm/report_all_non_pm_mti",
			"REPORT_DATA_FILTERED_NON_PM_MTI": fun.GLOBAL_URL + "web/" + fun.GetRedis("web:"+admin.Session, redisDB) + "/tab-mti-monitoring-non-pm/report_data_filtered_non_pm_mti",

			/* DKI */
			"REFRESH_TABLE_TICKET_DKI":        fun.GLOBAL_URL + "web/" + fun.GetRedis("web:"+admin.Session, redisDB) + "/tab-dki-ticket/refresh",
			"TABLE_TICKET_DKI":                webguibuilder.TABLE_TICKET_DKI(admin.Session, redisDB),
			"REPORT_ALL_TICKET_DKI":           fun.GLOBAL_URL + "web/" + fun.GetRedis("web:"+admin.Session, redisDB) + "/tab-dki-ticket/report_all_ticket",
			"REPORT_DATA_FILTERED_TICKET_DKI": fun.GLOBAL_URL + "web/" + fun.GetRedis("web:"+admin.Session, redisDB) + "/tab-dki-ticket/report_all_ticket_filtered",

			/* DSP */
			"REFRESH_TABLE_TICKET_DSP":        fun.GLOBAL_URL + "web/" + fun.GetRedis("web:"+admin.Session, redisDB) + "/tab-dsp-ticket/refresh",
			"TABLE_TICKET_DSP":                webguibuilder.TABLE_TICKET_DSP(admin.Session, redisDB),
			"REPORT_ALL_TICKET_DSP":           fun.GLOBAL_URL + "web/" + fun.GetRedis("web:"+admin.Session, redisDB) + "/tab-dsp-ticket/report_all_ticket",
			"REPORT_DATA_FILTERED_TICKET_DSP": fun.GLOBAL_URL + "web/" + fun.GetRedis("web:"+admin.Session, redisDB) + "/tab-dsp-ticket/report_all_ticket_filtered",

			/* BNI */
			"TABLE_DATA_PM_BNI":               webguibuilder.TABLE_DATA_PM_BNI(admin.Session, redisDB),
			"TABLE_DATA_NON_PM_BNI":           webguibuilder.TABLE_DATA_NON_PM_BNI(admin.Session, redisDB),
			"REFRESH_TABLE_DATA_BNI":          fun.GLOBAL_URL + "tab-bni/refresh-task",
			"REPORT_ALL_PM_BNI":               fun.GLOBAL_URL + "web/" + fun.GetRedis("web:"+admin.Session, redisDB) + "/tab-bni-monitoring-pm/report_all_pm_bni",
			"REPORT_DATA_FILTERED_PM_BNI":     fun.GLOBAL_URL + "web/" + fun.GetRedis("web:"+admin.Session, redisDB) + "/tab-bni-monitoring-pm/report_data_filtered_pm_bni",
			"REPORT_ALL_NON_PM_BNI":           fun.GLOBAL_URL + "web/" + fun.GetRedis("web:"+admin.Session, redisDB) + "/tab-bni-monitoring-non-pm/report_all_non_pm_bni",
			"REPORT_DATA_FILTERED_NON_PM_BNI": fun.GLOBAL_URL + "web/" + fun.GetRedis("web:"+admin.Session, redisDB) + "/tab-bni-monitoring-non-pm/report_data_filtered_non_pm_bni",
		}
		c.HTML(http.StatusOK, componentID+".html", replacements)
	}
}
