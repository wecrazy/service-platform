package controllers

import (
	"fmt"
	whatsappmodel "service-platform/cmd/web_panel/model/whatsapp_model"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/sirupsen/logrus"
	"gorm.io/gorm"
)

// SeedWhatsappSampleData creates sample WhatsApp conversation data for testing
func SeedWhatsappSampleData(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		userIDStr := c.Param("userid")
		if userIDStr == "" {
			c.JSON(400, gin.H{"error": "User ID is required"})
			return
		}

		userID := uint(1) // Default to user ID 1 for testing
		if userIDStr != "1" {
			c.JSON(400, gin.H{"error": "Only user ID 1 is supported for seeding"})
			return
		}

		// Create sample contacts
		contacts := []whatsappmodel.WAContactInfo{
			{
				UserID:      userID,
				ContactJID:  "6281234567890@c.us",
				ContactName: "John Doe",
				PhoneNumber: "+62 812-3456-7890",
				StatusText:  "Hey there! I am using WhatsApp.",
				IsOnline:    true,
				PushName:    "John",
			},
			{
				UserID:      userID,
				ContactJID:  "6289876543210@c.us",
				ContactName: "Jane Smith",
				PhoneNumber: "+62 898-7654-3210",
				StatusText:  "Busy working...",
				IsOnline:    false,
				LastSeen:    func() *time.Time { t := time.Now().Add(-1 * time.Hour); return &t }(),
				PushName:    "Jane",
			},
			{
				UserID:      userID,
				ContactJID:  "6285555666777@c.us",
				ContactName: "Bob Johnson",
				PhoneNumber: "+62 855-5566-6777",
				StatusText:  "Available",
				IsOnline:    false,
				LastSeen:    func() *time.Time { t := time.Now().Add(-2 * time.Hour); return &t }(),
				PushName:    "Bob",
			},
		}

		// Create contacts
		for _, contact := range contacts {
			result := db.Where("user_id = ? AND contact_jid = ?", contact.UserID, contact.ContactJID).
				FirstOrCreate(&contact)
			if result.Error != nil {
				logrus.WithError(result.Error).Error("Failed to create contact")
			}
		}

		// Create sample conversations
		conversations := []whatsappmodel.WAConversation{
			{
				UserID:          userID,
				ContactJID:      "6281234567890@c.us",
				ContactName:     "John Doe",
				ContactPhone:    "+62 812-3456-7890",
				IsGroup:         false,
				LastMessage:     "Hello! How are you doing?",
				LastMessageTime: func() *time.Time { t := time.Now().Add(-5 * time.Minute); return &t }(),
				UnreadCount:     2,
			},
			{
				UserID:          userID,
				ContactJID:      "6289876543210@c.us",
				ContactName:     "Jane Smith",
				ContactPhone:    "+62 898-7654-3210",
				IsGroup:         false,
				LastMessage:     "See you tomorrow!",
				LastMessageTime: func() *time.Time { t := time.Now().Add(-1 * time.Hour); return &t }(),
				UnreadCount:     0,
			},
			{
				UserID:          userID,
				ContactJID:      "120363023398765432@g.us",
				ContactName:     "",
				IsGroup:         true,
				GroupSubject:    "Work Team",
				LastMessage:     "Meeting at 3 PM",
				LastMessageTime: func() *time.Time { t := time.Now().Add(-2 * time.Hour); return &t }(),
				UnreadCount:     5,
			},
		}

		// Create conversations and get their IDs
		for i, conv := range conversations {
			result := db.Where("user_id = ? AND contact_jid = ?", conv.UserID, conv.ContactJID).
				FirstOrCreate(&conversations[i])
			if result.Error != nil {
				logrus.WithError(result.Error).Error("Failed to create conversation")
			}
		}

		// Create sample messages for John Doe conversation
		johnConv := conversations[0]
		db.Where("user_id = ? AND contact_jid = ?", johnConv.UserID, johnConv.ContactJID).First(&johnConv)

		messages := []whatsappmodel.WAChatMessage{
			{
				ConversationID: johnConv.ID,
				UserID:         userID,
				MessageID:      "msg_john_1",
				FromJID:        "6281234567890@c.us",
				ToJID:          fmt.Sprintf("%d@c.us", userID),
				MessageType:    "text",
				MessageContent: "Hello! How are you doing?",
				IsOutgoing:     false,
				MessageStatus:  "read",
				Timestamp:      time.Now().Add(-2 * time.Hour),
			},
			{
				ConversationID: johnConv.ID,
				UserID:         userID,
				MessageID:      "msg_john_2",
				FromJID:        fmt.Sprintf("%d@c.us", userID),
				ToJID:          "6281234567890@c.us",
				MessageType:    "text",
				MessageContent: "I'm doing great, thanks! How about you?",
				IsOutgoing:     true,
				MessageStatus:  "read",
				Timestamp:      time.Now().Add(-1 * time.Hour),
			},
			{
				ConversationID: johnConv.ID,
				UserID:         userID,
				MessageID:      "msg_john_3",
				FromJID:        "6281234567890@c.us",
				ToJID:          fmt.Sprintf("%d@c.us", userID),
				MessageType:    "text",
				MessageContent: "Excellent! Let's meet up soon.",
				IsOutgoing:     false,
				MessageStatus:  "delivered",
				Timestamp:      time.Now().Add(-30 * time.Minute),
			},
			{
				ConversationID: johnConv.ID,
				UserID:         userID,
				MessageID:      "msg_john_4",
				FromJID:        fmt.Sprintf("%d@c.us", userID),
				ToJID:          "6281234567890@c.us",
				MessageType:    "text",
				MessageContent: "Sure! What time works for you?",
				IsOutgoing:     true,
				MessageStatus:  "sent",
				Timestamp:      time.Now().Add(-15 * time.Minute),
			},
		}

		// Create messages
		for _, msg := range messages {
			result := db.Where("message_id = ?", msg.MessageID).FirstOrCreate(&msg)
			if result.Error != nil {
				logrus.WithError(result.Error).Error("Failed to create message")
			}
		}

		// Create some messages for Jane conversation too
		janeConv := conversations[1]
		db.Where("user_id = ? AND contact_jid = ?", janeConv.UserID, janeConv.ContactJID).First(&janeConv)

		janeMessages := []whatsappmodel.WAChatMessage{
			{
				ConversationID: janeConv.ID,
				UserID:         userID,
				MessageID:      "msg_jane_1",
				FromJID:        fmt.Sprintf("%d@c.us", userID),
				ToJID:          "6289876543210@c.us",
				MessageType:    "text",
				MessageContent: "Are we still on for tomorrow?",
				IsOutgoing:     true,
				MessageStatus:  "read",
				Timestamp:      time.Now().Add(-3 * time.Hour),
			},
			{
				ConversationID: janeConv.ID,
				UserID:         userID,
				MessageID:      "msg_jane_2",
				FromJID:        "6289876543210@c.us",
				ToJID:          fmt.Sprintf("%d@c.us", userID),
				MessageType:    "text",
				MessageContent: "See you tomorrow!",
				IsOutgoing:     false,
				MessageStatus:  "read",
				Timestamp:      time.Now().Add(-1 * time.Hour),
			},
		}

		for _, msg := range janeMessages {
			result := db.Where("message_id = ?", msg.MessageID).FirstOrCreate(&msg)
			if result.Error != nil {
				logrus.WithError(result.Error).Error("Failed to create Jane message")
			}
		}

		c.JSON(200, gin.H{
			"message": "Sample WhatsApp data seeded successfully",
			"data": gin.H{
				"contacts":      len(contacts),
				"conversations": len(conversations),
				"messages":      len(messages) + len(janeMessages),
			},
		})
	}
}
