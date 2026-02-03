package controllers

import (
	"context"
	"fmt"
	"net/http"
	"service-platform/cmd/web_panel/fun"
	"service-platform/cmd/web_panel/model"
	whatsappmodel "service-platform/cmd/web_panel/model/whatsapp_model"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/go-redis/redis/v8"
	"github.com/sirupsen/logrus"
	"gorm.io/gorm"
)

// GetUserWhatsappStatus returns detailed status information for a specific user's WhatsApp client
func GetUserWhatsappStatus(db *gorm.DB, redisDB *redis.Client) gin.HandlerFunc {
	return func(c *gin.Context) {
		userIDStr := c.Param("userid")
		if userIDStr == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "User ID is required"})
			return
		}

		userID, err := strconv.ParseUint(userIDStr, 10, 32)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid user ID"})
			return
		}

		manager := GetUserClientManager()

		// Check if client exists and get detailed status
		if client, exists := manager.GetClient(uint(userID)); exists {
			status, err := client.SendCommand(CommandGetStatus, nil)
			if err != nil {
				logrus.WithError(err).Error("Failed to get client status")
				c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get status"})
				return
			}

			if clientStatus, ok := status.(ClientStatus); ok {
				c.JSON(http.StatusOK, clientStatus)
				return
			}
		}

		// Client doesn't exist, return default status
		c.JSON(http.StatusOK, ClientStatus{
			UserID:          uint(userID),
			State:           StateDisconnected,
			IsConnected:     false,
			IsAuthenticated: false,
			HasQRCode:       false,
			Message:         "Client not initialized",
			Timestamp:       time.Now(),
		})
	}
}

// IsUserWhatsappLoggedIn checks if a specific user's WhatsApp client is connected
func IsUserWhatsappLoggedIn(db *gorm.DB, redisDB *redis.Client) gin.HandlerFunc {
	return func(c *gin.Context) {
		userIDStr := c.Param("userid")
		if userIDStr == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "User ID is required"})
			return
		}

		userID, err := strconv.ParseUint(userIDStr, 10, 32)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid user ID"})
			return
		}

		manager := GetUserClientManager()

		// Check if client exists and is connected
		if client, exists := manager.GetClient(uint(userID)); exists {
			status, err := client.SendCommand(CommandGetStatus, nil)
			if err != nil {
				logrus.WithError(err).Error("Failed to get client status")
				c.JSON(http.StatusOK, gin.H{"logged_in": false})
				return
			}

			if clientStatus, ok := status.(ClientStatus); ok {
				// User is considered "logged in" only if authenticated (QR scanned)
				c.JSON(http.StatusOK, gin.H{"logged_in": clientStatus.IsAuthenticated})
				return
			}
		}

		// Fallback to Redis check for backward compatibility
		ctx := context.Background()
		key := fmt.Sprintf("whatsapp_logged_in:%s", userIDStr)
		val, err := redisDB.Get(ctx, key).Result()
		if err == redis.Nil || val != "true" {
			c.JSON(http.StatusOK, gin.H{"logged_in": false})
			return
		}
		if err != nil {
			logrus.WithError(err).Error("Redis error")
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Internal server error"})
			return
		}
		c.JSON(http.StatusOK, gin.H{"logged_in": true})
	}
}

// ConnectUserWhatsapp initializes and connects a WhatsApp client for a specific user
func ConnectUserWhatsapp(db *gorm.DB, redisDB *redis.Client) gin.HandlerFunc {
	return func(c *gin.Context) {
		userIDStr := c.Param("userid")
		if userIDStr == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "User ID is required"})
			return
		}

		userID, err := strconv.ParseUint(userIDStr, 10, 32)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid user ID"})
			return
		}

		manager := GetUserClientManager()

		// Initialize manager if not already done
		if manager.db == nil {
			manager.Initialize(redisDB, db)
		}

		// Get or create client for user
		client, err := manager.GetOrCreateClient(uint(userID))
		if err != nil {
			logrus.WithError(err).Error("Failed to create WhatsApp client")
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create WhatsApp client"})
			return
		}

		// Send connect command
		result, err := client.SendCommand(CommandConnect, nil)
		if err != nil {
			logrus.WithError(err).Error("Failed to connect WhatsApp client")
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to connect WhatsApp client"})
			return
		}

		if connectErr, ok := result.(error); ok && connectErr != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": connectErr.Error()})
			return
		}

		c.JSON(http.StatusOK, gin.H{"message": "WhatsApp client connected successfully"})
	}
}

// DisconnectUserWhatsapp disconnects a WhatsApp client for a specific user
func DisconnectUserWhatsapp(db *gorm.DB, redisDB *redis.Client) gin.HandlerFunc {
	return func(c *gin.Context) {
		userIDStr := c.Param("userid")
		if userIDStr == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "User ID is required"})
			return
		}

		userID, err := strconv.ParseUint(userIDStr, 10, 32)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid user ID"})
			return
		}

		manager := GetUserClientManager()

		if client, exists := manager.GetClient(uint(userID)); exists {
			_, err := client.SendCommand(CommandDisconnect, nil)
			if err != nil {
				logrus.WithError(err).Error("Failed to disconnect WhatsApp client")
				c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to disconnect WhatsApp client"})
				return
			}

			// Wait a moment for disconnect to complete, then remove the client
			time.Sleep(1 * time.Second)
			manager.RemoveClient(uint(userID))
		}

		c.JSON(http.StatusOK, gin.H{"message": "WhatsApp client disconnected successfully"})
	}
}

// GetUserWhatsappQR generates and returns QR code for a specific user's WhatsApp client
func GetUserWhatsappQR(db *gorm.DB, redisDB *redis.Client) gin.HandlerFunc {
	return func(c *gin.Context) {
		userIDStr := c.Param("userid")
		if userIDStr == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "User ID is required"})
			return
		}

		userID, err := strconv.ParseUint(userIDStr, 10, 32)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid user ID"})
			return
		}

		manager := GetUserClientManager()

		// Initialize manager if not already done
		if manager.db == nil {
			manager.Initialize(redisDB, db)
		}

		// Get or create client for user
		client, err := manager.GetOrCreateClient(uint(userID))
		if err != nil {
			logrus.WithError(err).Error("Failed to create WhatsApp client")
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create WhatsApp client"})
			return
		}

		// Get QR code
		result, err := client.SendCommand(CommandGetQR, nil)
		if err != nil {
			logrus.WithError(err).Error("Failed to get QR code")
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get QR code"})
			return
		}

		qrCode := ""
		if qr, ok := result.(string); ok {
			qrCode = qr
		}

		c.JSON(http.StatusOK, gin.H{
			"qr_code":   qrCode,
			"user_id":   userID,
			"timestamp": time.Now(),
		})
	}
}

// RefreshUserWhatsappQR refreshes and returns a new QR code for a specific user's WhatsApp client
func RefreshUserWhatsappQR(db *gorm.DB, redisDB *redis.Client) gin.HandlerFunc {
	return func(c *gin.Context) {
		userIDStr := c.Param("userid")
		if userIDStr == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "User ID is required"})
			return
		}

		userID, err := strconv.ParseUint(userIDStr, 10, 32)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid user ID"})
			return
		}

		manager := GetUserClientManager()

		// Initialize manager if not already done
		if manager.db == nil {
			manager.Initialize(redisDB, db)
		}

		// Get or create client for user
		client, err := manager.GetOrCreateClient(uint(userID))
		if err != nil {
			logrus.WithError(err).Error("Failed to create WhatsApp client")
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create WhatsApp client"})
			return
		}

		// Refresh QR code
		result, err := client.SendCommand(CommandRefreshQR, nil)
		if err != nil {
			logrus.WithError(err).Error("Failed to refresh QR code")
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to refresh QR code"})
			return
		}

		if refreshErr, ok := result.(error); ok && refreshErr != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": refreshErr.Error()})
			return
		}

		// Wait a moment for the new QR to be generated
		time.Sleep(2 * time.Second)

		// Get the new QR code
		qrResult, err := client.SendCommand(CommandGetQR, nil)
		if err != nil {
			logrus.WithError(err).Error("Failed to get new QR code")
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get new QR code"})
			return
		}

		qrCode := ""
		if qr, ok := qrResult.(string); ok {
			qrCode = qr
		}

		c.JSON(http.StatusOK, gin.H{
			"qr_code":   qrCode,
			"user_id":   userID,
			"refreshed": true,
			"timestamp": time.Now(),
		})
	}
}

// SendUserWhatsappMessage sends a WhatsApp message using a specific user's client
func SendUserWhatsappMessage(db *gorm.DB, redisDB *redis.Client) gin.HandlerFunc {
	return func(c *gin.Context) {
		userIDStr := c.Param("userid")
		if userIDStr == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "User ID is required"})
			return
		}

		userID, err := strconv.ParseUint(userIDStr, 10, 32)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid user ID"})
			return
		}

		// Parse request body
		var req struct {
			Recipient string `json:"recipient" binding:"required"`
			Message   string `json:"message" binding:"required"`
			IsGroup   bool   `json:"is_group"`
		}

		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}

		manager := GetUserClientManager()

		// Get client for user
		client, exists := manager.GetClient(uint(userID))
		if !exists {
			c.JSON(http.StatusNotFound, gin.H{"error": "WhatsApp client not found for user"})
			return
		}

		// Prepare payload
		payload := SendTextPayload{
			Recipient: req.Recipient,
			Message:   req.Message,
			IsGroup:   req.IsGroup,
		}

		// Send message
		result, err := client.SendCommand(CommandSendText, payload)
		if err != nil {
			logrus.WithError(err).Error("Failed to send WhatsApp message")
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to send message"})
			return
		}

		if sendErr, ok := result.(error); ok && sendErr != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": sendErr.Error()})
			return
		}

		c.JSON(http.StatusOK, gin.H{
			"message":   "Message sent successfully",
			"user_id":   userID,
			"recipient": req.Recipient,
		})
	}
}

// ListActiveWhatsappClients returns a list of all active WhatsApp clients
func ListActiveWhatsappClients() gin.HandlerFunc {
	return func(c *gin.Context) {
		manager := GetUserClientManager()
		activeClients := manager.ListActiveClients()

		c.JSON(http.StatusOK, gin.H{
			"active_clients": activeClients,
			"count":          len(activeClients),
		})
	}
}

// WhatsApp Display Components for AJAX

// GetWhatsappMainInterface returns the complete WhatsApp interface layout
func GetWhatsappMainInterface(db *gorm.DB, redisDB *redis.Client) gin.HandlerFunc {
	return func(c *gin.Context) {
		userIDStr := c.Param("userid")
		if userIDStr == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "User ID is required"})
			return
		}

		userID, err := strconv.ParseUint(userIDStr, 10, 32)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid user ID"})
			return
		}

		// Get user status
		manager := GetUserClientManager()
		var status ClientStatus

		if client, exists := manager.GetClient(uint(userID)); exists {
			client.mutex.RLock()
			status = ClientStatus{
				UserID:          uint(userID),
				State:           client.State,
				IsConnected:     client.IsConnected,
				IsAuthenticated: client.IsAuthenticated,
				PhoneNumber:     client.PhoneNumber,
				// LastSeen:        client.LastUsed,
			}
			client.mutex.RUnlock()
		} else {
			status = ClientStatus{
				UserID:          uint(userID),
				State:           StateDisconnected,
				IsConnected:     false,
				IsAuthenticated: false,
			}
		}

		// Prepare template data
		data := gin.H{
			"userid":           userID,
			"is_authenticated": status.IsAuthenticated,
			"is_connected":     status.IsConnected,
			"phone_number":     status.PhoneNumber,
			"state":            status.State,
		}

		c.HTML(http.StatusOK, "whatsapp-main-interface.html", data)
	}
}

// GetWhatsappSidebarLeft returns the left sidebar component for WhatsApp chat interface
func GetWhatsappSidebarLeft(db *gorm.DB, redisDB *redis.Client) gin.HandlerFunc {
	return func(c *gin.Context) {
		userIDStr := c.Param("userid")
		if userIDStr == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "User ID is required"})
			return
		}

		userID, err := strconv.ParseUint(userIDStr, 10, 32)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid user ID"})
			return
		} // Get user status
		manager := GetUserClientManager()
		var status ClientStatus

		if client, exists := manager.GetClient(uint(userID)); exists {
			client.mutex.RLock()
			status = ClientStatus{
				UserID:          uint(userID),
				State:           client.State,
				IsConnected:     client.IsConnected,
				IsAuthenticated: client.IsAuthenticated,
				PhoneNumber:     client.PhoneNumber,
				LastSeen:        client.LastUsed,
				Message:         string(client.State),
				Timestamp:       time.Now(),
			}
			client.mutex.RUnlock()
		} else {
			// Default disconnected status
			status = ClientStatus{
				UserID:          uint(userID),
				State:           StateDisconnected,
				IsConnected:     false,
				IsAuthenticated: false,
				Message:         "Client not initialized",
				Timestamp:       time.Now(),
			}
		}

		// Get user information from database
		var user model.Admin
		if err := db.Where("id = ?", userID).First(&user).Error; err != nil {
			logrus.WithError(err).Errorf("Failed to get user data for ID %d", userID)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get user data"})
			return
		}

		// Generate profile image path
		imageMaps := map[string]interface{}{
			"t":  fun.GenerateRandomString(3),
			"id": user.ID,
		}
		pathString, err := fun.GetAESEcryptedURLfromJSON(imageMaps)
		if err != nil {
			logrus.WithError(err).Error("Could not encrypt image path")
			pathString = "default"
		}
		profileImage := "/profile/default.jpg?f=" + pathString

		// Prepare template data
		data := gin.H{
			"userid":           userID,
			"fullname":         user.Fullname,
			"username":         user.Username,
			"phone_number":     status.PhoneNumber,
			"profile_image":    profileImage,
			"is_connected":     status.IsConnected,
			"is_authenticated": status.IsAuthenticated,
			"state":            status.State,
			"last_seen":        status.LastSeen.Format("15:04:05"),
		}

		c.HTML(http.StatusOK, "whatsapp-sidebar-left.html", data)
	}
}

// GetWhatsappChatArea returns the main chat area component
func GetWhatsappChatArea(db *gorm.DB, redisDB *redis.Client) gin.HandlerFunc {
	return func(c *gin.Context) {
		userIDStr := c.Param("userid")
		if userIDStr == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "User ID is required"})
			return
		}

		userID, err := strconv.ParseUint(userIDStr, 10, 32)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid user ID"})
			return
		}

		contactID := c.Query("contact_id") // Optional: specific contact to show chat with

		// Get user status
		manager := GetUserClientManager()
		var userStatus ClientStatus
		if client, exists := manager.GetClient(uint(userID)); exists {
			client.mutex.RLock()
			userStatus = ClientStatus{
				UserID:          uint(userID),
				State:           client.State,
				IsConnected:     client.IsConnected,
				IsAuthenticated: client.IsAuthenticated,
				// PhoneNumber:     client.PhoneNumber,
				// LastSeen:        client.LastUsed,
				Timestamp: time.Now(),
			}
			client.mutex.RUnlock()
		} else {
			userStatus = ClientStatus{
				UserID:          uint(userID),
				State:           StateDisconnected,
				IsConnected:     false,
				IsAuthenticated: false,
				Timestamp:       time.Now(),
			}
		}

		// Prepare template data
		data := gin.H{
			"userid":           userID,
			"is_authenticated": userStatus.IsAuthenticated,
			"is_connected":     userStatus.IsConnected,
			"contact_id":       contactID,
			"messages":         []gin.H{}, // Will be loaded via AJAX in real implementation
		}

		// Add contact information if provided
		if contactID != "" && userStatus.IsAuthenticated {
			// Normalize the contact JID
			normalizedContactID := NormalizeJID(contactID)

			var contactName, contactPhone string
			var isGroup bool

			// Find conversation in database
			var conversation whatsappmodel.WAConversation
			err := db.Where("user_id = ? AND contact_jid = ? AND deleted_at IS NULL", userID, normalizedContactID).
				First(&conversation).Error

			if err == nil {
				// Use data from database
				contactName = conversation.GetDisplayName()
				contactPhone = conversation.ContactPhone
				isGroup = conversation.IsGroup
			} else {
				// Fallback to extracting from JID
				if strings.Contains(normalizedContactID, "@g.us") {
					isGroup = true
					contactName = "Group Chat"
				} else {
					isGroup = false
					phoneOnly := extractPhoneFromJID(normalizedContactID)
					contactPhone = formatPhoneDisplay(phoneOnly)
					contactName = "Contact " + phoneOnly
				}
			}

			data["contact"] = gin.H{
				"id":        normalizedContactID,
				"name":      contactName,
				"phone":     contactPhone,
				"avatar":    "/assets/img/avatars/default-avatar.jpeg",
				"is_online": false,
				"is_group":  isGroup,
				"last_seen": "Recently",
			}

			// Fetch real messages from database instead of dummy data
			var dbMessages []whatsappmodel.WAChatMessage
			if err == nil { // Conversation exists
				err = db.Where("conversation_id = ? AND user_id = ? AND deleted_at IS NULL", conversation.ID, userID).
					Order("timestamp ASC").
					Find(&dbMessages).Error

				if err != nil {
					logrus.WithError(err).Error("Failed to fetch messages")
					dbMessages = []whatsappmodel.WAChatMessage{} // Empty if error
				}

				// Mark messages as read when opening conversation
				go func() {
					db.Model(&whatsappmodel.WAChatMessage{}).
						Where("conversation_id = ? AND user_id = ? AND is_outgoing = ? AND message_status != ?",
							conversation.ID, userID, false, "read").
						Update("message_status", "read")

					// Update conversation unread count
					db.Model(&conversation).Update("unread_count", 0)
				}()
			}

			// Convert database messages to display format
			var messages []gin.H
			for _, msg := range dbMessages {
				// Generate sender initials
				senderInitials := "ME"
				if !msg.IsOutgoing {
					if contactName != "" {
						words := strings.Fields(contactName)
						if len(words) >= 2 {
							senderInitials = string(words[0][0]) + string(words[1][0])
						} else if len(words) == 1 && len(words[0]) >= 2 {
							senderInitials = string(words[0][0]) + string(words[0][1])
						}
						senderInitials = strings.ToUpper(senderInitials)
					} else {
						senderInitials = "UN"
					}
				}

				// Format timestamp for display
				timestampStr := msg.Timestamp.Format("15:04")

				messages = append(messages, gin.H{
					"id":              fmt.Sprintf("msg_%d", msg.ID),
					"message_id":      msg.MessageID,
					"message":         msg.MessageContent,
					"timestamp":       timestampStr,
					"is_outgoing":     msg.IsOutgoing,
					"is_read":         msg.MessageStatus == "read",
					"is_delivered":    msg.MessageStatus == "delivered" || msg.MessageStatus == "read",
					"message_type":    msg.MessageType,
					"sender_initials": senderInitials,
					"media_url":       msg.MediaURL,
					"media_filename":  msg.MediaFileName,
					"media_mime_type": msg.MediaMimeType,
				})
			}

			data["messages"] = messages
		}

		c.HTML(http.StatusOK, "whatsapp-chat-area.html", data)
	}
}

// GetWhatsappContactList returns the contact list component
func GetWhatsappContactList(db *gorm.DB, redisDB *redis.Client) gin.HandlerFunc {
	return func(c *gin.Context) {
		userIDStr := c.Param("userid")
		if userIDStr == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "User ID is required"})
			return
		}

		userID, err := strconv.ParseUint(userIDStr, 10, 32)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid user ID"})
			return
		}

		// Get user status first
		manager := GetUserClientManager()
		var userStatus ClientStatus

		if client, exists := manager.GetClient(uint(userID)); exists {
			client.mutex.RLock()
			userStatus = ClientStatus{
				IsAuthenticated: client.IsAuthenticated,
				// UserID:          uint(userID),
				// State:           client.State,
				// IsConnected:     client.IsConnected,
				// PhoneNumber:     client.PhoneNumber,
				// LastSeen:        client.LastUsed,
			}
			client.mutex.RUnlock()
		}

		// Get contacts/conversations from database
		var contacts []gin.H

		if userStatus.IsAuthenticated {
			// Fetch real conversations from database
			var allConversations []whatsappmodel.WAConversation
			err := db.Where("user_id = ? AND deleted_at IS NULL", userID).
				Order("is_pinned DESC, CASE WHEN last_message_time IS NULL THEN 1 ELSE 0 END, last_message_time DESC, updated_at DESC").
				Preload("Messages", func(db *gorm.DB) *gorm.DB {
					return db.Where("deleted_at IS NULL").Order("timestamp DESC").Limit(1)
				}).
				Find(&allConversations).Error

			if err != nil {
				logrus.WithError(err).Error("Failed to fetch conversations")
				allConversations = []whatsappmodel.WAConversation{} // Empty slice if error
			}

			// Deduplicate conversations by normalized JID
			conversationMap := make(map[string]whatsappmodel.WAConversation)
			for _, conv := range allConversations {
				normalizedJID := NormalizeJID(conv.ContactJID)
				// Keep the most recent conversation for each normalized JID
				if existing, exists := conversationMap[normalizedJID]; !exists ||
					(conv.LastMessageTime != nil && (existing.LastMessageTime == nil || conv.LastMessageTime.After(*existing.LastMessageTime))) {
					// Update the JID to the normalized version
					conv.ContactJID = normalizedJID
					conversationMap[normalizedJID] = conv
				}
			}

			// Convert map back to slice
			conversations := make([]whatsappmodel.WAConversation, 0, len(conversationMap))
			for _, conv := range conversationMap {
				conversations = append(conversations, conv)
			}

			// Re-sort conversations by priority
			sort.Slice(conversations, func(i, j int) bool {
				// Pinned conversations first
				if conversations[i].IsPinned != conversations[j].IsPinned {
					return conversations[i].IsPinned
				}
				// Then by last message time (newest first)
				if conversations[i].LastMessageTime == nil && conversations[j].LastMessageTime == nil {
					return conversations[i].UpdatedAt.After(conversations[j].UpdatedAt)
				}
				if conversations[i].LastMessageTime == nil {
					return false
				}
				if conversations[j].LastMessageTime == nil {
					return true
				}
				return conversations[i].LastMessageTime.After(*conversations[j].LastMessageTime)
			})

			// Convert database conversations to display format
			for _, conv := range conversations {
				// Calculate time ago
				var timeAgo string
				if conv.LastMessageTime != nil {
					diff := time.Since(*conv.LastMessageTime)
					if diff < time.Minute {
						timeAgo = "now"
					} else if diff < time.Hour {
						timeAgo = fmt.Sprintf("%dm", int(diff.Minutes()))
					} else if diff < 24*time.Hour {
						timeAgo = fmt.Sprintf("%dh", int(diff.Hours()))
					} else {
						timeAgo = conv.LastMessageTime.Format("01/02")
					}
				} else {
					timeAgo = ""
				}

				// Generate initials
				initials := ""
				displayName := conv.GetDisplayName()
				if displayName != "" {
					words := strings.Fields(displayName)
					if len(words) >= 2 {
						initials = string(words[0][0]) + string(words[1][0])
					} else if len(words) == 1 {
						if len(words[0]) >= 2 {
							initials = string(words[0][0]) + string(words[0][1])
						} else {
							initials = string(words[0][0])
						}
					}
					initials = strings.ToUpper(initials)
				}

				contacts = append(contacts, gin.H{
					"id":           conv.ContactJID,
					"name":         displayName,
					"phone":        conv.ContactPhone,
					"initials":     initials,
					"last_message": conv.LastMessage,
					"last_seen":    conv.LastMessageTime,
					"time_ago":     timeAgo,
					"unread_count": conv.UnreadCount,
					"avatar":       conv.Avatar,
					"is_group":     conv.IsGroup,
					"is_online":    false, // Will be updated from WhatsApp client status
					"is_active":    false,
					"is_pinned":    conv.IsPinned,
					"is_muted":     conv.IsMuted,
					"is_archived":  conv.IsArchived,
				})
			}

			// If no conversations in database, show empty state
			if len(contacts) == 0 {
				logrus.WithField("user_id", userID).Info("No conversations found in database")
			}
		}

		// Generate HTML directly instead of using template
		var html strings.Builder

		if !userStatus.IsAuthenticated {
			html.WriteString(`
				<li class="chat-contact-list-item">
					<div class="d-flex justify-content-center align-items-center p-3">
						<div class="text-center">
							<i class="bx bx-wifi-off bx-lg text-warning mb-2"></i>
							<p class="text-muted mb-0">WhatsApp not connected</p>
							<small class="text-muted">Please connect your WhatsApp first</small>
						</div>
					</div>
				</li>
			`)
		} else if len(contacts) == 0 {
			html.WriteString(`
				<li class="chat-contact-list-item">
					<div class="d-flex justify-content-center align-items-center p-3">
						<div class="text-center">
							<i class="bx bx-message-dots bx-lg text-muted mb-2"></i>
							<p class="text-muted mb-0">No conversations yet</p>
							<small class="text-muted">Start by sending a message</small>
						</div>
					</div>
				</li>
			`)
		} else {
			// Generate HTML for each contact
			for _, contact := range contacts {
				contactID := contact["id"].(string)
				displayName := contact["name"].(string)
				if displayName == "" {
					if contact["is_group"].(bool) {
						displayName = "Group Chat"
					} else {
						// Extract phone number from JID
						if strings.Contains(contactID, "@") {
							phone := strings.Split(contactID, "@")[0]
							// Remove any :xx suffix (like :21)
							if strings.Contains(phone, ":") {
								phone = strings.Split(phone, ":")[0]
							}
							displayName = phone
						} else {
							displayName = "Unknown Contact"
						}
					}
				}

				lastMessage := contact["last_message"].(string)
				if len(lastMessage) > 50 {
					lastMessage = lastMessage[:47] + "..."
				}
				if lastMessage == "" {
					lastMessage = "No messages yet"
				}

				timeAgo := contact["time_ago"].(string)
				unreadCount := contact["unread_count"].(int)
				isGroup := contact["is_group"].(bool)

				html.WriteString(fmt.Sprintf(`
					<li class="chat-contact-list-item" data-contact-id="%s">
						<div class="d-flex">
							<div class="flex-shrink-0 avatar %s">
								%s
							</div>
							<div class="chat-contact-info flex-grow-1 ms-3">
								<div class="d-flex justify-content-between align-items-center">
									<h6 class="chat-contact-name mb-0">%s</h6>
									<small class="text-muted chat-time">%s</small>
								</div>
								<div class="d-flex justify-content-between align-items-center">
									<small class="chat-contact-status text-muted">%s</small>
									%s
								</div>
							</div>
						</div>
					</li>
				`,
					contactID,
					func() string {
						if isGroup {
							return "avatar-group"
						}
						return ""
					}(),
					func() string {
						if isGroup {
							return `<span class="avatar-initial rounded-circle bg-primary">G</span>`
						}
						return `<img src="/assets/img/avatars/default-avatar.jpeg" alt="Avatar" class="rounded-circle" />`
					}(),
					displayName,
					timeAgo,
					lastMessage,
					func() string {
						if unreadCount > 0 {
							return fmt.Sprintf(`<span class="badge bg-danger rounded-pill badge-notifications">%d</span>`, unreadCount)
						}
						return ""
					}(),
				))
			}
		}

		// Return the HTML content directly
		c.Data(http.StatusOK, "text/html; charset=utf-8", []byte(html.String()))
	}
}

// GetWhatsappConversationHistory returns the conversation history for a specific contact
func GetWhatsappConversationHistory(db *gorm.DB, redisDB *redis.Client) gin.HandlerFunc {
	return func(c *gin.Context) {
		userIDStr := c.Param("userid")
		contactID := c.Query("contact_id")

		if userIDStr == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "User ID is required"})
			return
		}
		if contactID == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Contact ID is required"})
			return
		}

		userID, err := strconv.ParseUint(userIDStr, 10, 32)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid user ID"})
			return
		}

		// Get conversation history - in real implementation, fetch from WhatsApp client
		// For demo purposes, provide sample conversation
		var messages []gin.H

		// Check if user is authenticated and get conversation messages from database
		manager := GetUserClientManager()
		if client, exists := manager.GetClient(uint(userID)); exists && client.IsAuthenticated {
			// Normalize the contact JID to handle device suffixes
			normalizedContactID := NormalizeJID(contactID)

			// Find conversation in database using normalized JID
			var conversation whatsappmodel.WAConversation
			err := db.Where("user_id = ? AND contact_jid = ? AND deleted_at IS NULL", userID, normalizedContactID).
				First(&conversation).Error

			if err != nil && err != gorm.ErrRecordNotFound {
				logrus.WithError(err).Error("Failed to fetch conversation")
				c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch conversation"})
				return
			}

			// Fetch messages from database
			var dbMessages []whatsappmodel.WAChatMessage
			if err == nil { // Conversation exists
				err = db.Where("conversation_id = ? AND user_id = ? AND deleted_at IS NULL", conversation.ID, userID).
					Order("timestamp ASC").
					// Preload("QuotedMessage"). // Removed to avoid circular dependency
					Find(&dbMessages).Error

				if err != nil {
					logrus.WithError(err).Error("Failed to fetch messages")
					dbMessages = []whatsappmodel.WAChatMessage{} // Empty if error
				}

				// Mark messages as read when viewing conversation history
				go func() {
					db.Model(&whatsappmodel.WAChatMessage{}).
						Where("conversation_id = ? AND user_id = ? AND is_outgoing = ? AND message_status != ?",
							conversation.ID, userID, false, "read").
						Update("message_status", "read")

					// Update conversation unread count
					db.Model(&conversation).Update("unread_count", 0)
				}()
			}

			// Convert database messages to display format
			for _, msg := range dbMessages {
				// Generate sender initials
				senderInitials := "ME"
				senderAvatar := ""

				if !msg.IsOutgoing {
					// For incoming messages, get initials from contact name
					if conversation.ContactName != "" {
						words := strings.Fields(conversation.ContactName)
						if len(words) >= 2 {
							senderInitials = string(words[0][0]) + string(words[1][0])
						} else if len(words) == 1 && len(words[0]) >= 2 {
							senderInitials = string(words[0][0]) + string(words[0][1])
						}
						senderInitials = strings.ToUpper(senderInitials)
					} else {
						senderInitials = "UN" // Unknown
					}
					senderAvatar = conversation.Avatar
				}

				// Handle quoted messages
				var quotedContent string
				if msg.HasQuotedMessage() {
					// We could fetch quoted message separately if needed
					quotedContent = msg.QuotedContent // Use the stored quoted content instead
				}

				messages = append(messages, gin.H{
					"id":              fmt.Sprintf("msg_%d", msg.ID),
					"message_id":      msg.MessageID,
					"from":            msg.FromJID,
					"to":              msg.ToJID,
					"message":         msg.MessageContent,
					"timestamp":       msg.Timestamp,
					"is_outgoing":     msg.IsOutgoing,
					"message_type":    msg.MessageType,
					"read_status":     msg.MessageStatus,
					"sender_initials": senderInitials,
					"sender_avatar":   senderAvatar,
					"media_url":       msg.MediaURL,
					"media_filename":  msg.MediaFileName,
					"media_mime_type": msg.MediaMimeType,
					"thumbnail_url":   msg.ThumbnailURL,
					"is_starred":      msg.IsStarred,
					"is_forwarded":    msg.IsForwarded,
					"forwarded_from":  msg.ForwardedFrom,
					"quoted_content":  quotedContent,
					"location":        msg.Location,
					"location_name":   msg.LocationName,
					"contact_vcard":   msg.ContactVCard,
					"mentions":        msg.GetMentions(),
					"reactions":       msg.GetReactions(),
					"formatted_time":  msg.GetFormattedTimestamp(),
					"formatted_date":  msg.GetFormattedDate(),
				})
			}

			// If no messages found, log it
			if len(messages) == 0 {
				logrus.WithFields(logrus.Fields{
					"user_id":    userID,
					"contact_id": contactID,
				}).Info("No messages found for conversation")
			}
		}

		data := gin.H{
			"userid":     userID,
			"contact_id": contactID,
			"messages":   messages,
		}

		c.JSON(http.StatusOK, data)
	}
}

// Search Functions

// SearchWhatsappContacts searches for contacts based on query
func SearchWhatsappContacts(db *gorm.DB, redisDB *redis.Client) gin.HandlerFunc {
	return func(c *gin.Context) {
		userIDStr := c.Param("userid")
		if userIDStr == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "User ID is required"})
			return
		}

		var req struct {
			Query string `json:"query" binding:"required"`
			Limit int    `json:"limit"`
		}

		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}

		if req.Limit == 0 {
			req.Limit = 20
		}

		userID, err := strconv.ParseUint(userIDStr, 10, 32)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid user ID"})
			return
		}

		// Search contacts in database
		contacts := []gin.H{}
		if len(req.Query) > 0 {
			var dbContacts []whatsappmodel.WAContactInfo
			searchQuery := "%" + strings.ToLower(req.Query) + "%"

			err := db.Where("user_id = ? AND deleted_at IS NULL", userID).
				Where("LOWER(contact_name) LIKE ? OR LOWER(phone_number) LIKE ? OR LOWER(push_name) LIKE ?",
					searchQuery, searchQuery, searchQuery).
				Limit(req.Limit).
				Find(&dbContacts).Error

			if err != nil {
				logrus.WithError(err).Error("Failed to search contacts")
			} else {
				for _, contact := range dbContacts {
					contacts = append(contacts, gin.H{
						"id":        contact.ContactJID,
						"name":      contact.GetDisplayName(),
						"phone":     contact.PhoneNumber,
						"avatar":    contact.Avatar,
						"status":    contact.StatusText,
						"is_online": contact.IsOnline,
						"last_seen": contact.GetFormattedLastSeen(),
					})
				}
			}
		}

		c.JSON(http.StatusOK, gin.H{
			"contacts": contacts,
			"query":    req.Query,
			"count":    len(contacts),
		})
	}
}

// SearchWhatsappMessages searches for messages based on query
func SearchWhatsappMessages(db *gorm.DB, redisDB *redis.Client) gin.HandlerFunc {
	return func(c *gin.Context) {
		userIDStr := c.Param("userid")
		if userIDStr == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "User ID is required"})
			return
		}

		var req struct {
			Query     string `json:"query" binding:"required"`
			ContactID string `json:"contact_id"`
			Limit     int    `json:"limit"`
		}

		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}

		if req.Limit == 0 {
			req.Limit = 50
		}

		userID, err := strconv.ParseUint(userIDStr, 10, 32)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid user ID"})
			return
		}

		// Search messages in database
		messages := []gin.H{}
		if len(req.Query) > 0 {
			var dbMessages []whatsappmodel.WAChatMessage
			searchQuery := "%" + strings.ToLower(req.Query) + "%"

			query := db.Where("user_id = ? AND deleted_at IS NULL", userID).
				Where("LOWER(message_content) LIKE ?", searchQuery).
				Preload("Conversation")

			// If contact_id is specified, filter by conversation
			if req.ContactID != "" {
				query = query.Joins("JOIN wa_conversations ON wa_chat_messages.conversation_id = wa_conversations.id").
					Where("wa_conversations.contact_jid = ?", req.ContactID)
			}

			err := query.Order("timestamp DESC").
				Limit(req.Limit).
				Find(&dbMessages).Error

			if err != nil {
				logrus.WithError(err).Error("Failed to search messages")
			} else {
				for _, message := range dbMessages {
					messages = append(messages, gin.H{
						"id":             fmt.Sprintf("msg_%d", message.ID),
						"contact_id":     message.Conversation.ContactJID,
						"contact_name":   message.Conversation.GetDisplayName(),
						"message":        message.MessageContent,
						"timestamp":      message.Timestamp,
						"is_outgoing":    message.IsOutgoing,
						"message_type":   message.MessageType,
						"preview":        message.GetPreviewText(100),
						"formatted_time": message.GetFormattedTimestamp(),
						"formatted_date": message.GetFormattedDate(),
					})
				}
			}
		}

		c.JSON(http.StatusOK, gin.H{
			"messages": messages,
			"query":    req.Query,
			"count":    len(messages),
		})
	}
}

// SearchWhatsappConversations searches for conversations based on query
func SearchWhatsappConversations(db *gorm.DB, redisDB *redis.Client) gin.HandlerFunc {
	return func(c *gin.Context) {
		userIDStr := c.Param("userid")
		if userIDStr == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "User ID is required"})
			return
		}

		var req struct {
			Query string `json:"query" binding:"required"`
			Limit int    `json:"limit"`
		}

		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}

		if req.Limit == 0 {
			req.Limit = 10
		}

		userID, err := strconv.ParseUint(userIDStr, 10, 32)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid user ID"})
			return
		}

		// Search conversations in database
		conversations := []gin.H{}
		if len(req.Query) > 0 {
			var dbConversations []whatsappmodel.WAConversation
			searchQuery := "%" + strings.ToLower(req.Query) + "%"

			err := db.Where("user_id = ? AND deleted_at IS NULL", userID).
				Where("LOWER(contact_name) LIKE ? OR LOWER(group_subject) LIKE ? OR LOWER(last_message) LIKE ?",
					searchQuery, searchQuery, searchQuery).
				Order("CASE WHEN last_message_time IS NULL THEN 1 ELSE 0 END, last_message_time DESC").
				Limit(req.Limit).
				Find(&dbConversations).Error

			if err != nil {
				logrus.WithError(err).Error("Failed to search conversations")
			} else {
				for _, conv := range dbConversations {
					// Calculate time ago
					var timeAgo string
					if conv.LastMessageTime != nil {
						diff := time.Since(*conv.LastMessageTime)
						if diff < time.Minute {
							timeAgo = "now"
						} else if diff < time.Hour {
							timeAgo = fmt.Sprintf("%dm", int(diff.Minutes()))
						} else if diff < 24*time.Hour {
							timeAgo = fmt.Sprintf("%dh", int(diff.Hours()))
						} else {
							timeAgo = conv.LastMessageTime.Format("01/02")
						}
					}

					conversations = append(conversations, gin.H{
						"contact_id":   conv.ContactJID,
						"contact_name": conv.GetDisplayName(),
						"last_message": conv.LastMessage,
						"last_seen":    conv.LastMessageTime,
						"time_ago":     timeAgo,
						"unread_count": conv.UnreadCount,
						"is_group":     conv.IsGroup,
						"is_pinned":    conv.IsPinned,
						"is_muted":     conv.IsMuted,
						"avatar":       conv.Avatar,
					})
				}
			}
		}

		c.JSON(http.StatusOK, gin.H{
			"conversations": conversations,
			"query":         req.Query,
			"count":         len(conversations),
		})
	}
}
