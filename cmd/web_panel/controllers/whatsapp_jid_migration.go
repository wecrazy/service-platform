package controllers

import (
	"net/http"
	whatsappmodel "service-platform/cmd/web_panel/model/whatsapp_model"

	"github.com/gin-gonic/gin"
	"github.com/sirupsen/logrus"
	"gorm.io/gorm"
)

// MigrateJIDFormats updates existing conversation JIDs to use normalized format
func MigrateJIDFormats(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		logrus.Info("Starting JID format migration...")

		var updatedConversations, updatedContacts, updatedMessages int

		// Update WAConversation contact_jid from @c.us to @s.whatsapp.net
		result := db.Model(&whatsappmodel.WAConversation{}).
			Where("contact_jid LIKE '%@c.us'").
			Update("contact_jid", gorm.Expr("REPLACE(contact_jid, '@c.us', '@s.whatsapp.net')"))

		if result.Error != nil {
			logrus.WithError(result.Error).Error("Failed to update conversation JIDs")
			c.JSON(500, gin.H{"error": "Failed to update conversation JIDs"})
			return
		}
		updatedConversations = int(result.RowsAffected)

		// Update WAContactInfo contact_jid from @c.us to @s.whatsapp.net
		result = db.Model(&whatsappmodel.WAContactInfo{}).
			Where("contact_jid LIKE '%@c.us'").
			Update("contact_jid", gorm.Expr("REPLACE(contact_jid, '@c.us', '@s.whatsapp.net')"))

		if result.Error != nil {
			logrus.WithError(result.Error).Error("Failed to update contact JIDs")
			c.JSON(500, gin.H{"error": "Failed to update contact JIDs"})
			return
		}
		updatedContacts = int(result.RowsAffected)

		// Update WAChatMessage from_jid and to_jid from @c.us to @s.whatsapp.net
		result = db.Model(&whatsappmodel.WAChatMessage{}).
			Where("from_jid LIKE '%@c.us'").
			Update("from_jid", gorm.Expr("REPLACE(from_jid, '@c.us', '@s.whatsapp.net')"))

		if result.Error != nil {
			logrus.WithError(result.Error).Error("Failed to update message from_jid")
		} else {
			updatedMessages += int(result.RowsAffected)
		}

		result = db.Model(&whatsappmodel.WAChatMessage{}).
			Where("to_jid LIKE '%@c.us'").
			Update("to_jid", gorm.Expr("REPLACE(to_jid, '@c.us', '@s.whatsapp.net')"))

		if result.Error != nil {
			logrus.WithError(result.Error).Error("Failed to update message to_jid")
		} else {
			updatedMessages += int(result.RowsAffected)
		}

		logrus.Infof("JID migration complete: %d conversations, %d contacts, %d messages updated",
			updatedConversations, updatedContacts, updatedMessages)

		c.JSON(http.StatusOK, gin.H{
			"success":               true,
			"message":               "JID format migration completed",
			"updated_conversations": updatedConversations,
			"updated_contacts":      updatedContacts,
			"updated_messages":      updatedMessages,
		})
	}
}
