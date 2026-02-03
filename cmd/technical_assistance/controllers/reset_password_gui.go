package controllers

import (
	"context"
	"net/http"
	"os"

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
			"APP_NAME":         os.Getenv("APP_NAME"),
			"APP_LOGO":         os.Getenv("APP_LOGO"),
			"APP_VERSION":      os.Getenv("APP_VERSION"),
			"APP_VERSION_NO":   os.Getenv("APP_VERSION_NO"),
			"APP_VERSION_CODE": os.Getenv("APP_VERSION_CODE"),
			"APP_VERSION_NAME": os.Getenv("APP_VERSION_NAME"),
			"EMAIL":            email,
			"TOKEN":            tokenData,
		}
		// If the token matches, render the verification page
		c.HTML(http.StatusOK, "reset-password.html", parameters)
	}
}
