package fun

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
)

func ClearCookiesAndRedirect(c *gin.Context, cookies []*http.Cookie) {
	tokenString, err := c.Cookie("token")
	if err == nil {
		tokenString = strings.ReplaceAll(tokenString, " ", "+")
		decrypted, err := GetAESDecrypted(tokenString)
		if err != nil {
			fmt.Println("Error during decryption", err)
			ClearCookiesOnly(c, cookies)
			return
		}
		var claims map[string]interface{}
		err = json.Unmarshal(decrypted, &claims)
		if err != nil {
			fmt.Printf("Error converting JSON to map: %v", err)
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
	c.Redirect(http.StatusFound, GLOBAL_URL+"login")
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
