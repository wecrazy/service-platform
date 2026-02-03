package controllers

import (
	"fmt"
	"net/http"
	whatsappmodel "service-platform/cmd/web_panel/model/whatsapp_model"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/sirupsen/logrus"
	"gorm.io/gorm"
)

// SeedCompleteWhatsappData creates comprehensive WhatsApp conversation data for testing
func SeedCompleteWhatsappData(db *gorm.DB) gin.HandlerFunc {
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

		uid := uint(userID)

		// Clear existing data first
		db.Where("user_id = ?", uid).Delete(&whatsappmodel.WAChatMessage{})
		db.Where("user_id = ?", uid).Delete(&whatsappmodel.WAConversation{})
		db.Where("user_id = ?", uid).Delete(&whatsappmodel.WAContactInfo{})

		// Create contacts
		contacts := []whatsappmodel.WAContactInfo{
			{
				UserID:      uid,
				ContactJID:  "6281234567890@c.us",
				ContactName: "John Doe",
				PhoneNumber: "+62 812-3456-7890",
				StatusText:  "Hey there! I am using WhatsApp.",
				IsOnline:    true,
				PushName:    "John",
			},
			{
				UserID:      uid,
				ContactJID:  "6289876543210@c.us",
				ContactName: "Jane Smith",
				PhoneNumber: "+62 898-7654-3210",
				StatusText:  "Busy working...",
				IsOnline:    false,
				LastSeen:    func() *time.Time { t := time.Now().Add(-1 * time.Hour); return &t }(),
				PushName:    "Jane",
			},
			{
				UserID:      uid,
				ContactJID:  "120363023398765432@g.us",
				ContactName: "Work Team",
				PhoneNumber: "",
				StatusText:  "Group chat for work discussions",
				IsOnline:    false,
				PushName:    "Work Team",
			},
		}

		// Insert contacts
		for i := range contacts {
			if err := db.Create(&contacts[i]).Error; err != nil {
				logrus.WithError(err).Error("Failed to create contact")
			}
		}

		// Create conversations
		now := time.Now()
		conversations := []whatsappmodel.WAConversation{
			{
				UserID:          uid,
				ContactJID:      "6281234567890@c.us",
				ContactName:     "John Doe",
				ContactPhone:    "+62 812-3456-7890",
				IsGroup:         false,
				LastMessage:     "Hello! How are you doing?",
				LastMessageTime: func() *time.Time { t := now.Add(-5 * time.Minute); return &t }(),
				UnreadCount:     2,
			},
			{
				UserID:          uid,
				ContactJID:      "6289876543210@c.us",
				ContactName:     "Jane Smith",
				ContactPhone:    "+62 898-7654-3210",
				IsGroup:         false,
				LastMessage:     "See you tomorrow!",
				LastMessageTime: func() *time.Time { t := now.Add(-1 * time.Hour); return &t }(),
				UnreadCount:     0,
			},
			{
				UserID:          uid,
				ContactJID:      "120363023398765432@g.us",
				ContactName:     "",
				IsGroup:         true,
				GroupSubject:    "Work Team",
				LastMessage:     "Meeting at 3 PM",
				LastMessageTime: func() *time.Time { t := now.Add(-2 * time.Hour); return &t }(),
				UnreadCount:     5,
			},
		}

		// Insert conversations
		for i := range conversations {
			if err := db.Create(&conversations[i]).Error; err != nil {
				logrus.WithError(err).Error("Failed to create conversation")
			}
		}

		// Fetch the conversations to get their IDs
		var johnConv, janeConv, groupConv whatsappmodel.WAConversation
		db.Where("user_id = ? AND contact_jid = ?", uid, "6281234567890@c.us").First(&johnConv)
		db.Where("user_id = ? AND contact_jid = ?", uid, "6289876543210@c.us").First(&janeConv)
		db.Where("user_id = ? AND contact_jid = ?", uid, "120363023398765432@g.us").First(&groupConv)

		// Create messages for John Doe conversation
		johnMessages := []whatsappmodel.WAChatMessage{
			{
				ConversationID: johnConv.ID,
				UserID:         uid,
				MessageID:      "msg_john_1",
				FromJID:        "6281234567890@c.us",
				ToJID:          fmt.Sprintf("%d@c.us", uid),
				MessageType:    "text",
				MessageContent: "Hello! How are you doing?",
				IsOutgoing:     false,
				MessageStatus:  "read",
				Timestamp:      now.Add(-2 * time.Hour),
			},
			{
				ConversationID: johnConv.ID,
				UserID:         uid,
				MessageID:      "msg_john_2",
				FromJID:        fmt.Sprintf("%d@c.us", uid),
				ToJID:          "6281234567890@c.us",
				MessageType:    "text",
				MessageContent: "I'm doing great, thanks! How about you?",
				IsOutgoing:     true,
				MessageStatus:  "read",
				Timestamp:      now.Add(-1 * time.Hour),
			},
			{
				ConversationID: johnConv.ID,
				UserID:         uid,
				MessageID:      "msg_john_3",
				FromJID:        "6281234567890@c.us",
				ToJID:          fmt.Sprintf("%d@c.us", uid),
				MessageType:    "text",
				MessageContent: "Excellent! Let's meet up soon.",
				IsOutgoing:     false,
				MessageStatus:  "delivered",
				Timestamp:      now.Add(-30 * time.Minute),
			},
			{
				ConversationID: johnConv.ID,
				UserID:         uid,
				MessageID:      "msg_john_4",
				FromJID:        fmt.Sprintf("%d@c.us", uid),
				ToJID:          "6281234567890@c.us",
				MessageType:    "text",
				MessageContent: "Sure! What time works for you?",
				IsOutgoing:     true,
				MessageStatus:  "sent",
				Timestamp:      now.Add(-15 * time.Minute),
			},
		}

		// Insert John's messages
		for i := range johnMessages {
			if err := db.Create(&johnMessages[i]).Error; err != nil {
				logrus.WithError(err).Error("Failed to create John's message")
			}
		}

		// Create messages for Jane Smith conversation
		janeMessages := []whatsappmodel.WAChatMessage{
			{
				ConversationID: janeConv.ID,
				UserID:         uid,
				MessageID:      "msg_jane_1",
				FromJID:        fmt.Sprintf("%d@c.us", uid),
				ToJID:          "6289876543210@c.us",
				MessageType:    "text",
				MessageContent: "Are we still on for tomorrow?",
				IsOutgoing:     true,
				MessageStatus:  "read",
				Timestamp:      now.Add(-3 * time.Hour),
			},
			{
				ConversationID: janeConv.ID,
				UserID:         uid,
				MessageID:      "msg_jane_2",
				FromJID:        "6289876543210@c.us",
				ToJID:          fmt.Sprintf("%d@c.us", uid),
				MessageType:    "text",
				MessageContent: "See you tomorrow!",
				IsOutgoing:     false,
				MessageStatus:  "read",
				Timestamp:      now.Add(-1 * time.Hour),
			},
		}

		// Insert Jane's messages
		for i := range janeMessages {
			if err := db.Create(&janeMessages[i]).Error; err != nil {
				logrus.WithError(err).Error("Failed to create Jane's message")
			}
		}

		// Create messages for Work Team group
		groupMessages := []whatsappmodel.WAChatMessage{
			{
				ConversationID: groupConv.ID,
				UserID:         uid,
				MessageID:      "msg_group_1",
				FromJID:        "6281111222333@c.us",
				ToJID:          "120363023398765432@g.us",
				MessageType:    "text",
				MessageContent: "Good morning team!",
				IsOutgoing:     false,
				MessageStatus:  "read",
				Timestamp:      now.Add(-4 * time.Hour),
			},
			{
				ConversationID: groupConv.ID,
				UserID:         uid,
				MessageID:      "msg_group_2",
				FromJID:        fmt.Sprintf("%d@c.us", uid),
				ToJID:          "120363023398765432@g.us",
				MessageType:    "text",
				MessageContent: "Good morning! Ready for today's meeting?",
				IsOutgoing:     true,
				MessageStatus:  "read",
				Timestamp:      now.Add(-3 * time.Hour),
			},
			{
				ConversationID: groupConv.ID,
				UserID:         uid,
				MessageID:      "msg_group_3",
				FromJID:        "6282333444555@c.us",
				ToJID:          "120363023398765432@g.us",
				MessageType:    "text",
				MessageContent: "Meeting at 3 PM",
				IsOutgoing:     false,
				MessageStatus:  "delivered",
				Timestamp:      now.Add(-2 * time.Hour),
			},
		}

		// Insert group messages
		for i := range groupMessages {
			if err := db.Create(&groupMessages[i]).Error; err != nil {
				logrus.WithError(err).Error("Failed to create group message")
			}
		}

		c.JSON(http.StatusOK, gin.H{
			"message": "Complete WhatsApp data seeded successfully",
			"data": gin.H{
				"contacts":       len(contacts),
				"conversations":  len(conversations),
				"john_messages":  len(johnMessages),
				"jane_messages":  len(janeMessages),
				"group_messages": len(groupMessages),
				"total_messages": len(johnMessages) + len(janeMessages) + len(groupMessages),
			},
		})
	}
}

// GetWhatsappConversationDataAPI returns conversation data as JSON for testing
func GetWhatsappConversationDataAPI(db *gorm.DB) gin.HandlerFunc {
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

		// Get conversations
		var conversations []whatsappmodel.WAConversation
		err = db.Where("user_id = ? AND deleted_at IS NULL", userID).
			Order("CASE WHEN last_message_time IS NULL THEN 1 ELSE 0 END, last_message_time DESC, updated_at DESC").
			Find(&conversations).Error

		if err != nil {
			c.JSON(500, gin.H{"error": "Failed to fetch conversations: " + err.Error()})
			return
		}

		// Get messages for each conversation
		conversationData := make([]gin.H, 0)
		for _, conv := range conversations {
			var messages []whatsappmodel.WAChatMessage
			db.Where("conversation_id = ? AND deleted_at IS NULL", conv.ID).
				Order("timestamp ASC").
				Find(&messages)

			messageData := make([]gin.H, 0)
			for _, msg := range messages {
				messageData = append(messageData, gin.H{
					"id":             msg.ID,
					"message_id":     msg.MessageID,
					"from":           msg.FromJID,
					"to":             msg.ToJID,
					"content":        msg.MessageContent,
					"type":           msg.MessageType,
					"is_outgoing":    msg.IsOutgoing,
					"status":         msg.MessageStatus,
					"timestamp":      msg.Timestamp,
					"formatted_time": msg.Timestamp.Format("15:04"),
				})
			}

			conversationData = append(conversationData, gin.H{
				"conversation_id":   conv.ID,
				"contact_jid":       conv.ContactJID,
				"contact_name":      conv.GetDisplayName(),
				"contact_phone":     conv.ContactPhone,
				"is_group":          conv.IsGroup,
				"last_message":      conv.LastMessage,
				"last_message_time": conv.LastMessageTime,
				"unread_count":      conv.UnreadCount,
				"is_pinned":         conv.IsPinned,
				"messages":          messageData,
				"message_count":     len(messageData),
			})
		}

		c.JSON(http.StatusOK, gin.H{
			"user_id":       userID,
			"conversations": conversationData,
			"total_convs":   len(conversations),
		})
	}
}
