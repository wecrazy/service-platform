package fun

import (
	"encoding/json"
	"net/http"
	"service-platform/internal/config"
	"service-platform/internal/core/model"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/sirupsen/logrus"
	"gorm.io/gorm"
)

func ClearCookiesAndRedirect(c *gin.Context, cookies []*http.Cookie) {
	tokenString, err := c.Cookie("token")
	if err == nil {
		tokenString = strings.ReplaceAll(tokenString, " ", "+")
		decrypted, err := GetAESDecrypted(tokenString)
		if err != nil {
			logrus.Errorf("error during decrypt: %v", err)
			ClearCookiesOnly(c, cookies)
			return
		}
		var claims map[string]interface{}
		err = json.Unmarshal(decrypted, &claims)
		if err != nil {
			logrus.Errorf("error converting JSON to map: %v", err)
			ClearCookiesOnly(c, cookies)
			return
		}
		// emailToken := claims["email"].(string)
		// if emailToken != "" {
		// 	ws.CloseWebsocketConnection(emailToken)
		// }
	}
	for _, cookie := range cookies {
		cookie.Expires = time.Now().AddDate(0, 0, -1)
		http.SetCookie(c.Writer, cookie)
	}
	c.Redirect(http.StatusFound, config.GLOBAL_URL+"login")
	c.Abort()
}
func ClearCookiesOnly(c *gin.Context, cookies []*http.Cookie) {
	for _, cookie := range cookies {
		cookie.Expires = time.Now().AddDate(0, 0, -1)
		http.SetCookie(c.Writer, cookie)
	}
	c.Abort()
}

func ValidateCookie(c *gin.Context, cookieName string, expectedValue interface{}) bool {
	cookie, err := c.Cookie(cookieName)
	if err != nil || cookie != expectedValue {
		return false
	}
	return true
}

func RemoveEmailSession(db *gorm.DB, email string) {
	user := map[string]any{
		"session":    "",
		"last_login": nil,
	}

	// Perform the update
	err := db.Model(&model.Users{}).Where(model.Users{Email: email}).Updates(user).Error
	if err != nil {
		logrus.Errorf("got error: %v", err)
	}
}
