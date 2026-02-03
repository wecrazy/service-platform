package controllers

import (
	"context"
	"net/http"
	"service-platform/cmd/web_panel/config"

	"github.com/gin-gonic/gin"
	"github.com/go-redis/redis/v8"
	"gorm.io/gorm"
)

func GetWebResetPassword(db *gorm.DB, redisDB *redis.Client) gin.HandlerFunc {
	return func(c *gin.Context) {
		// Extract email and token_data from URL parameters
		email := c.Param("email")
		tokenData := c.Param("token_data")

		// Create Redis key
		redisKey := "reset_pwd:" + email

		// Fetch the token from Redis
		val, err := redisDB.Get(context.Background(), redisKey).Result()
		if err == redis.Nil {
			// Key does not exist
			c.HTML(http.StatusNotFound, "misc-error-page.html", gin.H{})
			return
		} else if err != nil {
			// Some other Redis error
			c.HTML(http.StatusInternalServerError, "misc-error-page.html", gin.H{})
			return
		}

		// Check if the token matches
		if val != tokenData {
			c.HTML(http.StatusNotFound, "misc-error-page.html", gin.H{})
			return
		}

		parameters := gin.H{
			"APP_NAME":         config.GetConfig().App.Name,
			"APP_LOGO":         config.GetConfig().App.Logo,
			"APP_VERSION":      config.GetConfig().App.Version,
			"APP_VERSION_NO":   config.GetConfig().App.VersionNo,
			"APP_VERSION_CODE": config.GetConfig().App.VersionCode,
			"APP_VERSION_NAME": config.GetConfig().App.VersionName,
			"EMAIL":            email,
			"TOKEN":            tokenData,
		}
		// If the token matches, render the verification page
		c.HTML(http.StatusOK, "reset-password.html", parameters)
	}
}
