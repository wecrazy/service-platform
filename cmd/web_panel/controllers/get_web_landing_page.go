package controllers

import (
	"net/http"
	"service-platform/cmd/web_panel/config"
	"service-platform/cmd/web_panel/fun"
	"service-platform/cmd/web_panel/model"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

func GetWebLandingPage(db *gorm.DB) gin.HandlerFunc {
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
			"APP_NAME":         config.GetConfig().App.Name,
			"APP_LOGO":         config.GetConfig().App.Logo,
			"APP_VERSION":      config.GetConfig().App.Version,
			"APP_VERSION_NO":   config.GetConfig().App.VersionNo,
			"APP_VERSION_CODE": config.GetConfig().App.VersionCode,
			"APP_VERSION_NAME": config.GetConfig().App.VersionName,
			"LOGIN":            "LOGIN",
		}

		if credentialsCookie != nil {
			var admin model.Admin
			if err := db.Where("session = ?", credentialsCookie.Value).First(&admin).Error; err != nil {
				fun.ClearCookiesAndRedirect(c, cookies)
			}

		}
		c.HTML(http.StatusOK, "landing-page.html", parameters)
	}
}
