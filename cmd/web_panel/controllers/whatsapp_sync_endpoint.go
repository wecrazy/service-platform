package controllers

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

// SyncWhatsappContactsEndpoint triggers contact sync for a user
func SyncWhatsappContactsEndpoint(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		userIDStr := c.Param("userid")
		if userIDStr == "" {
			c.JSON(400, gin.H{"error": "User ID is required"})
			return
		}

		userID, err := strconv.ParseUint(userIDStr, 10, 32)
		if err != nil {
			c.JSON(400, gin.H{"error": "Invalid user ID"})
			return
		}

		// Refresh contacts for the user
		err = RefreshWhatsappContactsForUser(uint(userID), db)
		if err != nil {
			c.JSON(500, gin.H{
				"error":   "Failed to sync contacts",
				"message": err.Error(),
			})
			return
		}

		c.JSON(http.StatusOK, gin.H{
			"success": true,
			"message": "Contacts synced successfully",
			"user_id": userID,
		})
	}
}
