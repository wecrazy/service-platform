package controllers

import (
	"net/http"
	"service-platform/cmd/technical_assistance/fun"
	"service-platform/cmd/technical_assistance/model"
	"service-platform/internal/config"

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
			"APP_NAME":         config.TechnicalAssistance.Get().APP_NAME,
			"APP_LOGO":         config.TechnicalAssistance.Get().APP_LOGO,
			"APP_VERSION":      config.TechnicalAssistance.Get().APP_VERSION_NO,
			"APP_VERSION_NO":   config.TechnicalAssistance.Get().APP_VERSION_NO,
			"APP_VERSION_CODE": config.TechnicalAssistance.Get().APP_VERSION_CODE,
			"APP_VERSION_NAME": config.TechnicalAssistance.Get().APP_VERSION_NAME,
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
