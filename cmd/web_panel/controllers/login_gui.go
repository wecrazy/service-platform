package controllers

import (
	"net/http"
	"service-platform/cmd/web_panel/fun"
	"service-platform/cmd/web_panel/model"
	"service-platform/internal/config"
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
			"APP_NAME":         config.WebPanel.Get().App.Name,
			"APP_LOGO":         config.WebPanel.Get().App.Logo,
			"APP_VERSION":      config.WebPanel.Get().App.Version,
			"APP_VERSION_NO":   config.WebPanel.Get().App.VersionNo,
			"APP_VERSION_CODE": config.WebPanel.Get().App.VersionCode,
			"APP_VERSION_NAME": config.WebPanel.Get().App.VersionName,
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
