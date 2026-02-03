package controllers

import (
	"net/http"
	"os"
	"service-platform/cmd/technical_assistance/fun"
	"service-platform/cmd/technical_assistance/model"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

func GetWebLogin(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		// Retrieve cookies from the request
		cookies := c.Request.Cookies()

		// Check if the "credentials" cookie exists
		var credentialsCookie *http.Cookie
		for _, cookie := range cookies {
			if cookie.Name == "credentials" {
				credentialsCookie = cookie
				break
			}
		}

		parameters := gin.H{
			"APP_NAME":         os.Getenv("APP_NAME"),
			"APP_LOGO":         os.Getenv("APP_LOGO"),
			"APP_VERSION":      os.Getenv("APP_VERSION"),
			"APP_VERSION_NO":   os.Getenv("APP_VERSION_NO"),
			"APP_VERSION_CODE": os.Getenv("APP_VERSION_CODE"),
			"APP_VERSION_NAME": os.Getenv("APP_VERSION_NAME"),
		}

		// Check if the credentials cookie is not nil before accessing its value
		if credentialsCookie != nil && credentialsCookie.Value != "" {
			var admin model.Admin
			if err := db.Where("session = ?", credentialsCookie.Value).First(&admin).Error; err != nil {
				for _, cookie := range cookies {
					cookie.Expires = time.Now().AddDate(0, 0, -1)
					http.SetCookie(c.Writer, cookie)
				}
				c.HTML(http.StatusOK, "login.html", parameters)
				return
			}
			c.Redirect(http.StatusFound, fun.GLOBAL_URL+"page")
		} else {
			for _, cookie := range cookies {
				cookie.Expires = time.Now().AddDate(0, 0, -1)
				http.SetCookie(c.Writer, cookie)
			}
			c.HTML(http.StatusOK, "login.html", parameters)
		}
	}
}
