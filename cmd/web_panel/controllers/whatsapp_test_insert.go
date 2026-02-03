package controllers

import (
	"net/http"
	whatsappmodel "service-platform/cmd/web_panel/model/whatsapp_model"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

// TestInsertSingleContact tests inserting a single contact to debug database issues
func TestInsertSingleContact(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		// Try to insert a single contact
		contact := whatsappmodel.WAContactInfo{
			UserID:      1,
			ContactJID:  "6281111111111@c.us",
			ContactName: "Test Contact",
			PhoneNumber: "+62 811-1111-1111",
			StatusText:  "Test status",
			IsOnline:    false,
		}

		// Insert contact
		result := db.Create(&contact)
		if result.Error != nil {
			c.JSON(http.StatusInternalServerError, gin.H{
				"error":         result.Error.Error(),
				"affected_rows": result.RowsAffected,
			})
			return
		}

		// Try to insert a conversation
		conversation := whatsappmodel.WAConversation{
			UserID:       1,
			ContactJID:   "6281111111111@c.us",
			ContactName:  "Test Contact",
			ContactPhone: "+62 811-1111-1111",
			IsGroup:      false,
			LastMessage:  "Test message",
		}

		result2 := db.Create(&conversation)
		if result2.Error != nil {
			c.JSON(http.StatusInternalServerError, gin.H{
				"contact_success":            true,
				"conversation_error":         result2.Error.Error(),
				"conversation_affected_rows": result2.RowsAffected,
			})
			return
		}

		c.JSON(http.StatusOK, gin.H{
			"success":           true,
			"contact_id":        contact.ID,
			"conversation_id":   conversation.ID,
			"contact_rows":      result.RowsAffected,
			"conversation_rows": result2.RowsAffected,
		})
	}
}
