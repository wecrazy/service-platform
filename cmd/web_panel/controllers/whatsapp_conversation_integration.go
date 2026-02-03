package controllers

import (
	"context"
	"fmt"
	whatsappmodel "service-platform/cmd/web_panel/model/whatsapp_model"
	"time"

	"github.com/sirupsen/logrus"
	"go.mau.fi/whatsmeow/types/events"
	"gorm.io/gorm"
)

// SaveMessageToConversation saves incoming/outgoing messages to the new conversation database models
func SaveMessageToConversation(event *events.Message, userID uint, db *gorm.DB) {
	if event == nil {
		return
	}

	if db == nil {
		logrus.Error("Database connection is nil for saving conversation message")
		return
	}

	// Extract message details
	fromJID := normalizeJID(event.Info.Sender.String())
	toJID := normalizeJID(event.Info.MessageSource.Chat.String())
	messageID := event.Info.ID
	timestamp := event.Info.Timestamp

	// Determine if it's outgoing or incoming
	isOutgoing := event.Info.MessageSource.IsFromMe

	// Extract message content and type
	messageContent := extractMessageText(event)
	messageType := determineMessageType(event)

	if messageType != "unknown" {
		if messageContent == "[Non-text message]" {
			messageContent = ""
		}
	}

	// REMOVE the debug soon !
	fmt.Println("========================================")
	fmt.Println("Message Details:")
	fmt.Printf("From JID: %s\n", fromJID)
	fmt.Printf("To JID: %s\n", toJID)
	fmt.Printf("Message ID: %s\n", messageID)
	fmt.Printf("Timestamp: %s\n", timestamp)
	fmt.Println("Message Content:", messageContent)
	fmt.Printf("Message Type: %s\n", messageType)
	fmt.Println("========================================")

	// Extract media information if applicable
	var mediaURL, mediaFileName, mediaMimeType, thumbnailURL string
	var mediaSize int64

	// Initialize media handler
	mediaHandler := NewMediaHandler("./web/assets") // Adjust base path as needed

	switch messageType {
	case "image":
		if event.Message.ImageMessage != nil {
			// Generate proper filename for image using MIME type
			mediaMimeType = getSafeString(event.Message.ImageMessage.Mimetype)
			mediaSize = int64(event.Message.ImageMessage.GetFileLength())

			// Caption is the text message that comes with the image
			if caption := getSafeString(event.Message.ImageMessage.Caption); caption != "" {
				messageContent = caption // Use caption as message content
			}

			// Download and save image
			if imageData, err := WhatsappClient.Download(context.Background(), event.Message.ImageMessage); err == nil {
				// Use new method that generates filename from MIME type
				if mediaInfo, err := mediaHandler.SaveMediaFileWithMimeType(userID, messageID, "image", imageData, mediaMimeType, timestamp); err == nil {
					mediaURL = mediaHandler.GetMediaFileURL(mediaInfo.RelativePath)
					mediaFileName = mediaInfo.FileName
					logrus.Infof("Saved image: %s", mediaURL)
				} else {
					logrus.WithError(err).Error("Failed to save image file")
				}
			} else {
				logrus.WithError(err).Error("Failed to download image")
			}
		}
	case "video":
		if event.Message.VideoMessage != nil {
			// Generate proper filename for video, caption is separate
			mediaFileName = fmt.Sprintf("video_%s.mp4", messageID)
			mediaMimeType = getSafeString(event.Message.VideoMessage.Mimetype)
			mediaSize = int64(event.Message.VideoMessage.GetFileLength())

			// Caption is the text message that comes with the video
			if caption := getSafeString(event.Message.VideoMessage.Caption); caption != "" {
				messageContent = caption // Use caption as message content
			}

			// Download and save video
			if videoData, err := WhatsappClient.Download(context.Background(), event.Message.VideoMessage); err == nil {
				if mediaInfo, err := mediaHandler.SaveMediaFile(userID, messageID, "video", videoData, mediaFileName, mediaMimeType, timestamp); err == nil {
					mediaURL = mediaHandler.GetMediaFileURL(mediaInfo.RelativePath)
					logrus.Infof("Saved video: %s", mediaURL)
				} else {
					logrus.WithError(err).Error("Failed to save video file")
				}
			} else {
				logrus.WithError(err).Error("Failed to download video")
			}
		}
	case "document":
		if event.Message.DocumentMessage != nil {
			mediaFileName = getSafeString(event.Message.DocumentMessage.FileName)
			if mediaFileName == "" {
				mediaFileName = fmt.Sprintf("document_%s.bin", messageID)
			}
			mediaMimeType = getSafeString(event.Message.DocumentMessage.Mimetype)
			mediaSize = int64(event.Message.DocumentMessage.GetFileLength())

			// Download and save document
			if docData, err := WhatsappClient.Download(context.Background(), event.Message.DocumentMessage); err == nil {
				if mediaInfo, err := mediaHandler.SaveMediaFile(userID, messageID, "document", docData, mediaFileName, mediaMimeType, timestamp); err == nil {
					mediaURL = mediaHandler.GetMediaFileURL(mediaInfo.RelativePath)
					logrus.Infof("Saved document: %s", mediaURL)
				} else {
					logrus.WithError(err).Error("Failed to save document file")
				}
			} else {
				logrus.WithError(err).Error("Failed to download document")
			}
		}
	case "audio":
		if event.Message.AudioMessage != nil {
			mediaFileName = fmt.Sprintf("audio_%s.ogg", messageID)
			mediaMimeType = getSafeString(event.Message.AudioMessage.Mimetype)
			mediaSize = int64(event.Message.AudioMessage.GetFileLength())

			// Download and save audio
			if audioData, err := WhatsappClient.Download(context.Background(), event.Message.AudioMessage); err == nil {
				if mediaInfo, err := mediaHandler.SaveMediaFile(userID, messageID, "audio", audioData, mediaFileName, mediaMimeType, timestamp); err == nil {
					mediaURL = mediaHandler.GetMediaFileURL(mediaInfo.RelativePath)
					logrus.Infof("Saved audio: %s", mediaURL)
				} else {
					logrus.WithError(err).Error("Failed to save audio file")
				}
			} else {
				logrus.WithError(err).Error("Failed to download audio")
			}
		}
	case "sticker":
		if event.Message.StickerMessage != nil {
			mediaFileName = fmt.Sprintf("sticker_%s.webp", messageID)
			mediaMimeType = getSafeString(event.Message.StickerMessage.Mimetype)
			mediaSize = int64(event.Message.StickerMessage.GetFileLength())

			// Download and save sticker
			if stickerData, err := WhatsappClient.Download(context.Background(), event.Message.StickerMessage); err == nil {
				if mediaInfo, err := mediaHandler.SaveMediaFile(userID, messageID, "sticker", stickerData, mediaFileName, mediaMimeType, timestamp); err == nil {
					mediaURL = mediaHandler.GetMediaFileURL(mediaInfo.RelativePath)
					logrus.Infof("Saved sticker: %s", mediaURL)
				} else {
					logrus.WithError(err).Error("Failed to save sticker file")
				}
			} else {
				logrus.WithError(err).Error("Failed to download sticker")
			}
		}
	}

	// Determine conversation JID (for groups vs private chats)
	var conversationJID, contactName, contactPhone string
	var isGroup bool

	if isGroupJID(toJID) {
		// Group chat
		isGroup = true
		conversationJID = toJID
		contactName = "" // Will be filled from group info
	} else {
		// Private chat
		isGroup = false
		if isOutgoing {
			conversationJID = toJID
		} else {
			conversationJID = fromJID
		}
		// Extract phone number
		contactPhone = extractPhoneFromJID(conversationJID)
		contactName = contactPhone // Default, can be updated from contacts
	}

	// Find or create conversation
	var conversation whatsappmodel.WAConversation
	err := db.Where("user_id = ? AND contact_jid = ?", userID, conversationJID).
		First(&conversation).Error

	if err == gorm.ErrRecordNotFound {
		// Create new conversation
		conversation = whatsappmodel.WAConversation{
			UserID:          userID,
			ContactJID:      conversationJID,
			ContactName:     contactName,
			ContactPhone:    contactPhone,
			IsGroup:         isGroup,
			LastMessage:     getMessagePreview(messageContent, messageType),
			LastMessageTime: &timestamp,
			UnreadCount:     0, // Will be incremented for incoming messages
		}

		if err := db.Create(&conversation).Error; err != nil {
			logrus.WithError(err).Error("Failed to create new conversation")
			return
		}
	} else if err != nil {
		logrus.WithError(err).Error("Failed to fetch conversation")
		return
	}

	// Update conversation with latest message
	updateData := map[string]interface{}{
		"last_message":      getMessagePreview(messageContent, messageType),
		"last_message_time": timestamp,
		"updated_at":        time.Now(),
	}

	// Increment unread count for incoming messages
	if !isOutgoing {
		updateData["unread_count"] = gorm.Expr("unread_count + ?", 1)
	}

	if err := db.Model(&conversation).Updates(updateData).Error; err != nil {
		logrus.WithError(err).Error("Failed to update conversation")
	}

	// Extract quoted message ID if this is a reply
	var quotedMessageID *uint
	var quotedContent string
	if event.Message.ExtendedTextMessage != nil &&
		event.Message.ExtendedTextMessage.ContextInfo != nil &&
		event.Message.ExtendedTextMessage.ContextInfo.StanzaID != nil {

		quotedStanzaID := *event.Message.ExtendedTextMessage.ContextInfo.StanzaID
		var quotedMsg whatsappmodel.WAChatMessage
		if err := db.Where("message_id = ? AND conversation_id = ?", quotedStanzaID, conversation.ID).
			First(&quotedMsg).Error; err == nil {
			quotedMessageID = &quotedMsg.ID
			quotedContent = quotedMsg.GetPreviewText(50)
		}
	}

	// Create the message record
	chatMessage := whatsappmodel.WAChatMessage{
		ConversationID:  conversation.ID,
		UserID:          userID,
		MessageID:       messageID,
		FromJID:         fromJID,
		ToJID:           toJID,
		MessageType:     messageType,
		MessageContent:  messageContent,
		MediaURL:        mediaURL,
		MediaFileName:   mediaFileName,
		MediaMimeType:   mediaMimeType,
		MediaSize:       mediaSize,
		ThumbnailURL:    thumbnailURL,
		IsOutgoing:      isOutgoing,
		MessageStatus:   "received", // Will be updated by delivery receipts
		QuotedMessageID: quotedMessageID,
		QuotedContent:   quotedContent,
		Timestamp:       timestamp,
	}

	if err := db.Create(&chatMessage).Error; err != nil {
		logrus.WithError(err).Error("Failed to save chat message")
		return
	}

	// Update contact info if it's a private chat
	if !isGroup {
		var contact whatsappmodel.WAContactInfo
		err := db.Where("user_id = ? AND contact_jid = ?", userID, conversationJID).
			First(&contact).Error

		switch err {
		case gorm.ErrRecordNotFound:
			// Create new contact
			contact = whatsappmodel.WAContactInfo{
				UserID:      userID,
				ContactJID:  conversationJID,
				ContactName: contactName,
				PhoneNumber: contactPhone,
				IsOnline:    true, // Assume online since they just sent a message
			}
			db.Create(&contact)
		case nil:
			// Update last seen for incoming messages
			if !isOutgoing {
				now := time.Now()
				db.Model(&contact).Updates(map[string]interface{}{
					"is_online":  true,
					"last_seen":  &now,
					"updated_at": now,
				})
			}
		}
	}

	// logrus.WithFields(logrus.Fields{
	// 	"user_id":         userID,
	// 	"conversation_id": conversation.ID,
	// 	"message_id":      messageID,
	// 	"message_type":    messageType,
	// 	"is_outgoing":     isOutgoing,
	// }).Info("Message saved to conversation database")
}

// Helper functions
func determineMessageType(event *events.Message) string {
	if event.Message.Conversation != nil {
		return "text"
	}
	if event.Message.ExtendedTextMessage != nil {
		return "text"
	}
	if event.Message.ImageMessage != nil {
		return "image"
	}
	if event.Message.VideoMessage != nil {
		return "video"
	}
	if event.Message.AudioMessage != nil {
		return "audio"
	}
	if event.Message.DocumentMessage != nil {
		return "document"
	}
	if event.Message.StickerMessage != nil {
		return "sticker"
	}
	if event.Message.LocationMessage != nil {
		return "location"
	}
	if event.Message.ContactMessage != nil {
		return "contact"
	}
	return "unknown"
}

func getMessagePreview(content, messageType string) string {
	switch messageType {
	case "text":
		if len(content) > 50 {
			return content[:50] + "..."
		}
		return content
	case "image":
		return "📷 Image"
	case "video":
		return "🎥 Video"
	case "audio":
		return "🎵 Audio"
	case "document":
		return "📄 Document"
	case "sticker":
		return "🏷️ Sticker"
	case "location":
		return "📍 Location"
	case "contact":
		return "👤 Contact"
	default:
		return "📝 Message"
	}
}

// UpdateMessageStatus updates message delivery status (for read receipts, delivery receipts)
func UpdateMessageStatus(messageID string, status string, userID uint, db *gorm.DB) {
	if db == nil {
		return
	}

	err := db.Model(&whatsappmodel.WAChatMessage{}).
		Where("message_id = ? AND user_id = ?", messageID, userID).
		Update("message_status", status).Error

	if err != nil {
		logrus.WithError(err).WithFields(logrus.Fields{
			"message_id": messageID,
			"status":     status,
			"user_id":    userID,
		}).Error("Failed to update message status")
	}
}
