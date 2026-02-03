package controllers

import (
	"net/http"
	whatsappmodel "service-platform/cmd/web_panel/model/whatsapp_model"
	"strconv"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

// GetWhatsappContactListAPI returns the contact list data as JSON
func GetWhatsappContactListAPI(db *gorm.DB) gin.HandlerFunc {
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

		// Get conversations for contact list
		var conversations []whatsappmodel.WAConversation
		err = db.Where("user_id = ? AND deleted_at IS NULL", userID).
			Order("CASE WHEN last_message_time IS NULL THEN 1 ELSE 0 END, last_message_time DESC, updated_at DESC").
			Find(&conversations).Error

		if err != nil {
			c.JSON(500, gin.H{"error": "Failed to fetch conversations: " + err.Error()})
			return
		}

		// Format for contact list
		contactList := make([]gin.H, 0)
		for _, conv := range conversations {
			contactList = append(contactList, gin.H{
				"conversation_id":   conv.ID,
				"contact_jid":       conv.ContactJID,
				"contact_name":      conv.GetDisplayName(),
				"contact_phone":     conv.ContactPhone,
				"is_group":          conv.IsGroup,
				"group_subject":     conv.GroupSubject,
				"last_message":      conv.LastMessage,
				"last_message_time": conv.LastMessageTime,
				"unread_count":      conv.UnreadCount,
				"is_pinned":         conv.IsPinned,
				"is_archived":       conv.IsArchived,
				"avatar_url":        "/path/to/default/avatar.png", // Placeholder
			})
		}

		c.JSON(http.StatusOK, gin.H{
			"success":     true,
			"user_id":     userID,
			"contacts":    contactList,
			"total_count": len(contactList),
		})
	}
}

// GetWhatsappConversationMessagesAPI returns messages for a specific conversation
func GetWhatsappConversationMessagesAPI(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		userIDStr := c.Param("userid")
		conversationIDStr := c.Param("conversationid")

		if userIDStr == "" {
			c.JSON(400, gin.H{"error": "User ID is required"})
			return
		}

		if conversationIDStr == "" {
			c.JSON(400, gin.H{"error": "Conversation ID is required"})
			return
		}

		userID, err := strconv.ParseUint(userIDStr, 10, 32)
		if err != nil {
			c.JSON(400, gin.H{"error": "Invalid user ID"})
			return
		}

		conversationID, err := strconv.ParseUint(conversationIDStr, 10, 32)
		if err != nil {
			c.JSON(400, gin.H{"error": "Invalid conversation ID"})
			return
		}

		// Verify conversation belongs to user
		var conv whatsappmodel.WAConversation
		err = db.Where("id = ? AND user_id = ?", conversationID, userID).First(&conv).Error
		if err != nil {
			c.JSON(404, gin.H{"error": "Conversation not found"})
			return
		}

		// Get messages for this conversation
		var messages []whatsappmodel.WAChatMessage
		err = db.Where("conversation_id = ? AND deleted_at IS NULL", conversationID).
			Order("timestamp ASC").
			Find(&messages).Error

		if err != nil {
			c.JSON(500, gin.H{"error": "Failed to fetch messages: " + err.Error()})
			return
		}

		// Format messages
		messageList := make([]gin.H, 0)
		for _, msg := range messages {
			messageList = append(messageList, gin.H{
				"id":             msg.ID,
				"message_id":     msg.MessageID,
				"from_jid":       msg.FromJID,
				"to_jid":         msg.ToJID,
				"content":        msg.MessageContent,
				"message_type":   msg.MessageType,
				"is_outgoing":    msg.IsOutgoing,
				"status":         msg.MessageStatus,
				"timestamp":      msg.Timestamp,
				"formatted_time": msg.Timestamp.Format("15:04"),
				"formatted_date": msg.Timestamp.Format("02/01/2006"),
			})
		}

		c.JSON(http.StatusOK, gin.H{
			"success": true,
			"conversation": gin.H{
				"id":            conv.ID,
				"contact_jid":   conv.ContactJID,
				"contact_name":  conv.GetDisplayName(),
				"contact_phone": conv.ContactPhone,
				"is_group":      conv.IsGroup,
				"group_subject": conv.GroupSubject,
			},
			"messages":      messageList,
			"message_count": len(messageList),
		})
	}
}
