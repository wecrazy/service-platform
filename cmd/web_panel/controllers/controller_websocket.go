package controllers

import (
	"encoding/json"
	"fmt"
	"service-platform/cmd/web_panel/fun"
	"service-platform/cmd/web_panel/model"
	"service-platform/cmd/web_panel/ws"
	"strings"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

func WebSocketVerify(db *gorm.DB) gin.HandlerFunc {
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
		var admin model.Admin
		if err := db.Where("id = ? AND session = ?", claims["id"], claims["session"].(string)).First(&admin).Error; err != nil {
			fun.ClearCookiesAndRedirect(c, cookies)
			return
		}

		ws.HandleWebSocket(c.Writer, c.Request, admin.Email+fun.GenerateRandomString(10), db)
	}
}
