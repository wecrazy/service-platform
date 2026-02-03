package controllers

import (
	"net/http"
	"service-platform/cmd/web_panel/config"
	"time"

	"github.com/dchest/captcha"
	"github.com/gin-gonic/gin"
)

func GetCaptchaImage() gin.HandlerFunc {
	return func(c *gin.Context) {

		expiration := time.Now().Add(time.Duration(10) * time.Minute)
		// Generate a new CAPTCHA ID
		captchaID := captcha.NewLen(6)
		captchaID_Cookie := &http.Cookie{
			Name:     "halo",
			Value:    captchaID,
			Expires:  expiration,
			Path:     "/",
			Domain:   config.GetConfig().App.CookieLoginDomain,
			SameSite: http.SameSiteStrictMode,
			Secure:   config.GetConfig().App.CookieLoginSecure,
			HttpOnly: true,
		}
		http.SetCookie(c.Writer, captchaID_Cookie)
		width := 240
		height := 80
		// // Generate a new CAPTCHA ID
		// captchaID := captcha.NewLen(6)
		// Set the response content type
		c.Header("Content-Type", "image/png")
		// Write the image to the response body
		captcha.WriteImage(c.Writer, captchaID, width, height)
		// Set a cookie to store the CAPTCHA ID
		// c.SetCookie("captcha_id", captchaID, 0, "/", "", false, true)
	}
}
