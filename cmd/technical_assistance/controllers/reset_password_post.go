package controllers

import (
	"context"
	"net/http"
	"service-platform/cmd/technical_assistance/fun"
	"service-platform/cmd/technical_assistance/model"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/go-redis/redis/v8"
	"gorm.io/gorm"
)

func PostResetPassword(db *gorm.DB, redisDB *redis.Client) gin.HandlerFunc {
	return func(c *gin.Context) {
		// Extract email, token_data, password, and confirm-password from the form data
		email := c.PostForm("email")
		tokenData := c.PostForm("token_data")
		password := c.PostForm("password")
		confirmPwd := c.PostForm("confirm-password")

		// Validate passwords
		if password != confirmPwd {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Passwords do not match"})
			return
		}
		// Validate the password
		if err := fun.ValidatePassword(password); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		var check_admin_password_changelogs []model.AdminPasswordChangeLog
		db.Where("email = ?", email).Order("created_at desc").Find(&check_admin_password_changelogs)
		for _, data := range check_admin_password_changelogs {
			if fun.IsPasswordMatched(password, data.Password) {
				c.JSON(http.StatusBadRequest, gin.H{"error": "Password MUST not Similar as 4 Passwords Before"})
				return
			}
		}

		// Create Redis key
		redisKey := "reset_pwd:" + email

		// Fetch the token from Redis
		val, err := redisDB.Get(context.Background(), redisKey).Result()
		if err == redis.Nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Link expired or invalid : " + err.Error()})
			return
		} else if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Error accessing Redis"})
			return
		}

		// Check if the token matches
		if val != tokenData {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid reset link"})
			return
		}
		// Update the password in the database
		var admin model.Admin
		if err := db.Where("email = ?", email).First(&admin).Error; err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Email not found : " + err.Error()})
			return
		}

		admin.Password = fun.GenerateSaltedPassword(password)
		admin.LastLogin = time.Now()
		if err := db.Save(&admin).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Error updating password " + err.Error()})
			return
		}

		var admin_password_changelog model.AdminPasswordChangeLog
		admin_password_changelog.Email = admin.Email
		admin_password_changelog.Password = admin.Password
		if err := db.Create(&admin_password_changelog).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Error updating password changelog" + err.Error()})
			return
		}
		var admin_password_changelogs []model.AdminPasswordChangeLog

		// Fetch the password change logs sorted by CreatedAt in ascending order
		if err := db.Where("email = ?", admin.Email).
			Order("created_at asc").
			Find(&admin_password_changelogs).Error; err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Email not found"})
			return
		}

		// If there are more than 4 records, delete the oldest ones
		if len(admin_password_changelogs) > 4 {
			for i := 0; i < len(admin_password_changelogs)-4; i++ {
				db.Delete(&admin_password_changelogs[i])
			}
		}
		// Remove the token from Redis
		if err := redisDB.Del(context.Background(), redisKey).Err(); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Error removing Redis key"})
			return
		}

		// Respond with success message
		c.JSON(http.StatusOK, gin.H{"msg": "Password reset successful"})
	}
}
