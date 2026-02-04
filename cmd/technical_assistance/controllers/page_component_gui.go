package controllers

import (
	"encoding/json"
	"fmt"
	"net/http"
	"service-platform/cmd/technical_assistance/fun"
	"service-platform/cmd/technical_assistance/model"
	"service-platform/cmd/technical_assistance/webguibuilder"
	"service-platform/internal/config"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/go-redis/redis/v8"
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
			fmt.Println("Error during decryption", err)
			fun.ClearCookiesAndRedirect(c, cookies)
			return
		}
		var claims map[string]interface{}
		err = json.Unmarshal(decrypted, &claims)
		if err != nil {
			fmt.Printf("Error converting JSON to map: %v", err)
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

		replacements := map[string]any{
			"APP_NAME":         config.TechnicalAssistance.Get().APP_NAME,
			"APP_LOGO":         config.TechnicalAssistance.Get().APP_LOGO,
			"APP_VERSION":      config.TechnicalAssistance.Get().APP_VERSION_NO,
			"APP_VERSION_NO":   config.TechnicalAssistance.Get().APP_VERSION_NO,
			"APP_VERSION_CODE": config.TechnicalAssistance.Get().APP_VERSION_CODE,
			"APP_VERSION_NAME": config.TechnicalAssistance.Get().APP_VERSION_NAME,
			"fullname":         admin.Fullname,
			"username":         admin.Username,
			"userid":           admin.ID,
			"phone":            admin.Phone,
			"email":            admin.Email,
			"role_name":        claims["role_name"].(string),
			"status_name":      claims["status_name"].(string),
			"last_login":       claims["last_login"].(string),
			"created_at_str":   claims["created_at_str"].(string),
			"profile_image":    profile_image,
			"ip":               admin.IP,
			"GLOBAL_URL":       fun.GLOBAL_URL,
			// "TABLE_MERCHANT_JO_HMIN1_CALL_LOG": webguibuilder.TABLE_MERCHANT_JO_HMIN1_CALL_LOG(admin.Session, redisDB),
			// "TABLE_UPLOADED_FILE":              webguibuilder.TABLE_UPLOADED_FILE(admin.Session, redisDB),
			"TABLE_KONFIRMASI_DATA_PENGERJAAN_PENDING":    webguibuilder.TABLE_KONFIRMASI_DATA_PENGERJAAN_PENDING(admin.Session, redisDB),
			"TABLE_KONFIRMASI_DATA_PENGERJAAN_ERROR":      webguibuilder.TABLE_KONFIRMASI_DATA_PENGERJAAN_ERROR(admin.Session, redisDB),
			"TABLE_KONFIRMASI_DATA_PENGERJAAN_SUBMISSION": webguibuilder.TABLE_KONFIRMASI_DATA_PENGERJAAN_SUBMISSION(admin.Session, redisDB),
			"TABLE_VIEW_DATA_ERROR":                       webguibuilder.TABLE_VIEW_DATA_ERROR(admin.Session, redisDB),
			"TABLE_VIEW_DATA_PENDING":                     webguibuilder.TABLE_VIEW_DATA_PENDING(admin.Session, redisDB),
			"TABLE_LOG_ACT":                               webguibuilder.TABLE_LOG_ACT(admin.Session, redisDB),
			"get_nama_teknisi":                            fun.GLOBAL_URL + "web/" + fun.GetRedis("web:"+admin.Session, redisDB) + "/tab-teknisi/teknisi/name",
			"get_serial_number":                           fun.GLOBAL_URL + "web/" + fun.GetRedis("web:"+admin.Session, redisDB) + "/tab-teknisi/teknisi/serial_number",
			"get_nama_aplikasi":                           fun.GLOBAL_URL + "web/" + fun.GetRedis("web:"+admin.Session, redisDB) + "/tab-teknisi/teknisi/app",
			"get_versi_aplikasi":                          fun.GLOBAL_URL + "web/" + fun.GetRedis("web:"+admin.Session, redisDB) + "/tab-teknisi/teknisi/app/ver",
			"post_unlock_key":                             fun.GLOBAL_URL + "web/" + fun.GetRedis("web:"+admin.Session, redisDB) + "/tab-teknisi/teknisi/serial_number/unlock",
		}
		c.HTML(200, componentID+".html", replacements)
	}
}
