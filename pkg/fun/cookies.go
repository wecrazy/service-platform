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

// ClearCookiesAndRedirect clears the specified cookies and redirects the user to the login page
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
	c.Redirect(http.StatusFound, config.GlobalURL+"login")
	c.Abort()
}

// ClearCookiesOnly clears the specified cookies without redirecting the user
func ClearCookiesOnly(c *gin.Context, cookies []*http.Cookie) {
	for _, cookie := range cookies {
		cookie.Expires = time.Now().AddDate(0, 0, -1)
		http.SetCookie(c.Writer, cookie)
	}
	c.Abort()
}

// ValidateCookie checks if the specified cookie exists and matches the expected value
func ValidateCookie(c *gin.Context, cookieName string, expectedValue interface{}) bool {
	cookie, err := c.Cookie(cookieName)
	if err != nil || cookie != expectedValue {
		return false
	}
	return true
}

// RemoveEmailSession clears the session and last login time for the user with the specified email in the database
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
