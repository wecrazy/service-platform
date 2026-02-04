package controllers

import (
	"net/http"
	"service-platform/cmd/web_panel/fun"
	"service-platform/cmd/web_panel/model"
	"service-platform/internal/config"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

func GetWebForgotPassword(db *gorm.DB) gin.HandlerFunc {
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
		if credentialsCookie != nil {
			var admin model.Admin
			if err := db.Where("session = ?", credentialsCookie.Value).First(&admin).Error; err != nil {
				c.HTML(http.StatusOK, "forgot-password.html", parameters)
				return
			}
			c.Redirect(302, fun.GLOBAL_URL+"page")
		} else {
			c.HTML(http.StatusOK, "forgot-password.html", parameters)
		}
	}
}
