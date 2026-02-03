package controllers

import (
	"net/http"
	"service-platform/cmd/web_panel/model"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

func GetAdminStatusCount(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		loginCookie, err := c.Request.Cookie("credentials")
		if err != nil || loginCookie == nil || loginCookie.Value == "" {
			// If cookie doesn't exist or is empty, redirect to login
			expiredCookie := http.Cookie{
				Name:    "credentials",
				Expires: time.Now().AddDate(0, 0, -1),
			}
			http.SetCookie(c.Writer, &expiredCookie)
			c.Redirect(http.StatusFound, "/login")
			return
		}

		var adminLogin model.Admin
		if err := db.Where("session = ?", loginCookie.Value).First(&adminLogin).Error; err != nil || adminLogin.ID == 0 {
			loginCookie.Expires = time.Now().AddDate(0, 0, -1)
			http.SetCookie(c.Writer, loginCookie)
			c.Redirect(302, "/login")
			return
		}

		const (
			ActiveStatus   = 1
			InactiveStatus = 2
			PendingStatus  = 0
		)

		var totalCount, activeCount, inactiveCount, pendingCount int64
		if err := db.Model(&model.Admin{}).Where("status = ?", ActiveStatus).Count(&activeCount).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{
				"error": "Error counting active users",
			})
			return
		}
		// Count the number of records in the 'admins' table
		if err := db.Model(&model.Admin{}).Where("status = ?", InactiveStatus).Count(&inactiveCount).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{
				"error": "Error counting inactive users",
			})
			return
		}
		// Count the number of records in the 'admins' table
		if err := db.Model(&model.Admin{}).Where("status = ?", PendingStatus).Count(&pendingCount).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{
				"error": "Error counting pending users",
			})
			return
		}
		if err := db.Model(&model.Admin{}).Count(&totalCount).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{
				"error": "Error counting records",
			})
			return
		}
		// Respond with the formatted data
		c.JSON(http.StatusOK, gin.H{
			"data": map[string]int64{
				"total":    totalCount,
				"active":   activeCount,
				"inactive": inactiveCount,
				"pending":  pendingCount,
			},
		})

	}
}
