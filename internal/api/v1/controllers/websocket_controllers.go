package controllers

import (
	"encoding/json"
	"fmt"
	"service-platform/internal/core/model"
	"service-platform/internal/ws"
	"service-platform/pkg/fun"
	"strings"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

// WebSocketVerify godoc
// @Summary      WebSocket Connection
// @Description  Upgrades connection to WebSocket for real-time communication
// @Tags         WebSocket
// @Param        token query string true "Auth Token"
// @Success      101  {string}   string "Switching Protocols"
// @Failure      401  {string}   string "Unauthorized"
// @Router       /ws [get]
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
		var user model.Users
		if err := db.Where("id = ? AND session = ?", claims["id"], claims["session"].(string)).First(&user).Error; err != nil {
			fun.ClearCookiesAndRedirect(c, cookies)
			return
		}

		ws.HandleWebSocket(c.Writer, c.Request, user.Email+fun.GenerateRandomString(10), db)
	}
}
