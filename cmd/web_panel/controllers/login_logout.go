package controllers

import (
	"encoding/json"
	"fmt"
	"net/http"
	"service-platform/cmd/web_panel/fun"
	"service-platform/cmd/web_panel/model"
	"strings"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

func GetWebLogout(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		cookies := c.Request.Cookies()

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

		id := uint(claims["id"].(float64))
		updates := map[string]interface{}{
			"Session":        "",
			"SessionExpired": 0,
		}

		if err := db.Model(&model.Admin{}).Where("id = ?", id).Updates(updates).Error; err != nil {
			fun.ClearCookiesAndRedirect(c, cookies)
			return
		}

		// if id != 0 {
		// 	ws.CloseWebsocketConnection(claims["email"].(string))
		// }

		// Redirect to the login page
		c.Redirect(http.StatusFound, fun.GLOBAL_URL+"login")

		// Save the newLog to the database
		db.Create(&model.LogActivity{
			AdminID:   uint(claims["id"].(float64)),
			FullName:  claims["fullname"].(string),
			Action:    "LOGOUT",
			Status:    "Success",
			Log:       "Logout by button",
			IP:        c.ClientIP(),
			UserAgent: c.Request.UserAgent(),
			ReqMethod: c.Request.Method,
			ReqUri:    c.Request.RequestURI,
		})
	}
}
