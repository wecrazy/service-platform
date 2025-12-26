package controllers

import (
	"context"
	"encoding/base64"
	"fmt"
	"net/http"
	"service-platform/internal/api/v1/dto"
	"service-platform/internal/config"
	whatsnyanmodel "service-platform/internal/core/model/whatsnyan_model"
	"service-platform/internal/pkg/fun"
	"service-platform/internal/whatsapp"
	"time"

	pb "service-platform/proto"

	"github.com/gin-gonic/gin"
	"go.mau.fi/whatsmeow/types"
	"google.golang.org/grpc/status"
	"gorm.io/gorm"
)

// SendWhatsAppMessage godoc
// @Summary      Send WhatsApp Message
// @Description  Sends a text or media message via WhatsApp
// @Tags         WhatsApp
// @Accept       json
// @Produce      json
// @Param        request body dto.SendWhatsAppMessageRequest true "Message Request"
// @Success      200  {object}   map[string]string
// @Failure      503  {object}   map[string]string "Service Unavailable"
// @Router       /web/tab-whatsapp/send_message [post]
func SendWhatsAppMessage(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		if whatsapp.Client == nil {
			c.JSON(http.StatusServiceUnavailable, gin.H{"error": "WhatsApp service not available"})
			return
		}

		var req dto.SendWhatsAppMessageRequest

		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}

		// Default type to text if not specified
		if req.Type == "" {
			req.Type = "text"
		}

		var recipientJID types.JID
		if req.IsGroup {
			j, err := ValidateGroupJID(req.Recipient)
			if err != nil {
				c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid group JID: " + err.Error()})
				return
			}
			recipientJID = j
		} else {
			// Sanitize the phone number
			sanitizedPhone, err := fun.SanitizeIndonesiaPhoneNumber(req.Recipient)
			if err != nil {
				c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid phone number: " + err.Error()})
				return
			}
			jid := config.GetConfig().Default.DialingCodeDefault + sanitizedPhone + "@" + types.DefaultUserServer

			// Check if registered on WhatsApp
			resp, err := whatsapp.Client.IsOnWhatsApp(c.Request.Context(), &pb.IsOnWhatsAppRequest{
				PhoneNumbers: []string{jid},
			})
			if err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to check WhatsApp status: " + err.Error()})
				return
			}
			if !resp.Success {
				c.JSON(http.StatusBadRequest, gin.H{"error": "Failed to check WhatsApp status: " + resp.Message})
				return
			}
			if len(resp.Results) == 0 || !resp.Results[0].IsRegistered {
				c.JSON(http.StatusBadRequest, gin.H{"error": "Phone number is not registered on WhatsApp"})
				return
			}

			recipientJID = types.NewJID(config.GetConfig().Default.DialingCodeDefault+sanitizedPhone, types.DefaultUserServer)
		}

		var content *pb.MessageContent

		switch req.Type {
		case "text":
			if req.Message == "" {
				c.JSON(http.StatusBadRequest, gin.H{"error": "Message content cannot be empty for text type"})
				return
			}
			if len(req.Message) > config.GetConfig().Whatsnyan.MaxMessageLength {
				c.JSON(http.StatusBadRequest, gin.H{"error": fmt.Sprintf("Message content exceeds maximum length of %d characters", config.GetConfig().Whatsnyan.MaxMessageLength)})
				return
			}
			content = &pb.MessageContent{
				ContentType: &pb.MessageContent_Text{Text: req.Message},
			}
		case "image", "video", "audio", "document":
			if req.MediaData == "" {
				c.JSON(http.StatusBadRequest, gin.H{"error": "Media data is required"})
				return
			}
			data, err := base64.StdEncoding.DecodeString(req.MediaData)
			if err != nil {
				c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid base64 media data"})
				return
			}
			content = &pb.MessageContent{
				ContentType: &pb.MessageContent_Media{
					Media: &pb.MediaContent{
						MediaType: req.Type,
						Data:      data,
						Filename:  req.Filename,
						Caption:   req.Caption,
						Mimetype:  req.MimeType,
						ViewOnce:  req.ViewOnce,
					},
				},
			}
		case "location":
			content = &pb.MessageContent{
				ContentType: &pb.MessageContent_Location{
					Location: &pb.LocationContent{
						Latitude:  req.Latitude,
						Longitude: req.Longitude,
						Name:      req.LocName,
						Address:   req.Address,
					},
				},
			}
		case "live_location":
			content = &pb.MessageContent{
				ContentType: &pb.MessageContent_LiveLocation{
					LiveLocation: &pb.LiveLocationContent{
						Latitude:                          req.Latitude,
						Longitude:                         req.Longitude,
						AccuracyInMeters:                  req.Accuracy,
						SpeedInMps:                        uint32(req.Speed),
						DegreesClockwiseFromMagneticNorth: req.Degrees,
						Caption:                           req.Caption,
						SequenceNumber:                    req.Sequence,
						TimeOffset:                        req.TimeOffset,
					},
				},
			}
		case "poll":
			content = &pb.MessageContent{
				ContentType: &pb.MessageContent_Poll{
					Poll: &pb.PollContent{
						Name:                   req.PollName,
						Options:                req.PollOptions,
						SelectableOptionsCount: req.SelectableCount,
					},
				},
			}
		case "contact":
			content = &pb.MessageContent{
				ContentType: &pb.MessageContent_Contact{
					Contact: &pb.ContactContent{
						DisplayName: req.ContactName,
						Vcard:       req.Vcard,
					},
				},
			}
		default:
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid message type"})
			return
		}

		resp, err := whatsapp.Client.SendMessage(c.Request.Context(), &pb.SendMessageRequest{
			To:      recipientJID.String(),
			Content: content,
		})

		if err != nil {
			if grpcErr, ok := status.FromError(err); ok {
				c.JSON(http.StatusInternalServerError, gin.H{"error": grpcErr.Message()})
				return
			}
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}

		// Insert log send msg to DB
		go func() {
			// Get sender JID
			senderJID := ""
			if whatsapp.Client != nil {
				meResp, err := whatsapp.Client.GetMe(context.Background(), &pb.GetMeRequest{})
				if err == nil && meResp.Success {
					senderJID = meResp.Jid
				}
			}

			// Parse timestamp
			var sentAt *time.Time
			if resp.Timestamp != "" {
				// Try to parse as unix timestamp
				var ts int64
				if _, err := fmt.Sscanf(resp.Timestamp, "%d", &ts); err == nil {
					t := time.Unix(ts, 0)
					sentAt = &t
				}
			}

			if sentAt == nil {
				now := time.Now()
				sentAt = &now
			}

			// Save msg to DB
			msgBody := req.Message
			if req.Type != "text" {
				msgBody = fmt.Sprintf("[%s] %s", req.Type, req.Caption)
			}

			msg := whatsnyanmodel.WhatsAppMsg{
				WhatsappChatID:        resp.Id,
				WhatsappChatJID:       recipientJID.String(),
				WhatsappSenderJID:     senderJID,
				WhatsappMessageBody:   msgBody,
				WhatsappMessageType:   req.Type,
				WhatsappSentAt:        sentAt,
				WhatsappIsGroup:       req.IsGroup,
				WhatsappMsgStatus:     "sent",
				WhatsappMessageSentTo: recipientJID.String(),
			}
			_ = SaveWhatsAppMessage(db, &msg)
		}()

		c.JSON(http.StatusOK, gin.H{
			"success":    resp.Success,
			"message":    resp.Message,
			"message_id": resp.Id,
		})
	}
}

// ConnectWhatsApp connects the WhatsApp client
// ConnectWhatsApp godoc
// @Summary      Connect WhatsApp
// @Description  Initiates connection to WhatsApp
// @Tags         WhatsApp
// @Accept       json
// @Produce      json
// @Success      200  {object}   map[string]string
// @Router       /web/tab-whatsapp/connect [post]
func ConnectWhatsApp(c *gin.Context) {
	if whatsapp.Client == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "WhatsApp service not available"})
		return
	}

	// No request body needed for connection as it uses the first available device or creates a new one
	resp, err := whatsapp.Client.Connect(c.Request.Context(), &pb.ConnectRequest{
		PhoneNumber: "", // Not used in single-device mode
	})

	if err != nil {
		if grpcErr, ok := status.FromError(err); ok {
			c.JSON(http.StatusInternalServerError, gin.H{"error": grpcErr.Message()})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": resp.Success,
		"message": resp.Message,
		"qr_code": resp.QrCode,
	})
}

// DisconnectWhatsApp disconnects the WhatsApp client
// DisconnectWhatsApp godoc
// @Summary      Disconnect WhatsApp
// @Description  Disconnects from WhatsApp
// @Tags         WhatsApp
// @Accept       json
// @Produce      json
// @Param        request body dto.DisconnectWhatsAppRequest true "Disconnect Request"
// @Success      200  {object}   map[string]string
// @Router       /web/tab-whatsapp/disconnect [post]
func DisconnectWhatsApp(c *gin.Context) {
	if whatsapp.Client == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "WhatsApp service not available"})
		return
	}

	var req dto.DisconnectWhatsAppRequest

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	resp, err := whatsapp.Client.Disconnect(c.Request.Context(), &pb.DisconnectRequest{
		PhoneNumber: req.PhoneNumber,
	})

	if err != nil {
		if grpcErr, ok := status.FromError(err); ok {
			c.JSON(http.StatusInternalServerError, gin.H{"error": grpcErr.Message()})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": resp.Success,
		"message": resp.Message,
	})
}

// LogoutWhatsApp godoc
// @Summary      Logout WhatsApp
// @Description  Logs out from WhatsApp session
// @Tags         WhatsApp
// @Accept       json
// @Produce      json
// @Success      200  {object}   map[string]string
// @Router       /web/tab-whatsapp/logout [post]
func LogoutWhatsApp(c *gin.Context) {
	if whatsapp.Client == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "WhatsApp service not available"})
		return
	}

	resp, err := whatsapp.Client.Logout(c.Request.Context(), &pb.WALogoutRequest{})

	if err != nil {
		if grpcErr, ok := status.FromError(err); ok {
			c.JSON(http.StatusInternalServerError, gin.H{"error": grpcErr.Message()})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": resp.Success,
		"message": resp.Message,
	})
}

// RefreshWhatsAppQR godoc
// @Summary      Refresh WhatsApp QR
// @Description  Refreshes the WhatsApp QR code
// @Tags         WhatsApp
// @Accept       json
// @Produce      json
// @Success      200  {object}   map[string]string
// @Router       /web/tab-whatsapp/refresh_qr [post]
func RefreshWhatsAppQR(c *gin.Context) {
	if whatsapp.Client == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "WhatsApp service not available"})
		return
	}

	resp, err := whatsapp.Client.RefreshQR(c.Request.Context(), &pb.RefreshQRRequest{})

	if err != nil {
		if grpcErr, ok := status.FromError(err); ok {
			c.JSON(http.StatusInternalServerError, gin.H{"error": grpcErr.Message()})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": resp.Success,
		"message": resp.Message,
		"qr_code": resp.QrCode,
	})
}

// CreateStatus godoc
// @Summary      Create WhatsApp Status
// @Description  Creates a new WhatsApp status
// @Tags         WhatsApp
// @Accept       json
// @Produce      json
// @Param        request body dto.CreateWhatsAppStatusRequest true "Status Request"
// @Success      200  {object}   map[string]string
// @Router       /web/tab-whatsapp/create_status [post]
func CreateStatus(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		if whatsapp.Client == nil {
			c.JSON(http.StatusServiceUnavailable, gin.H{"error": "WhatsApp service not available"})
			return
		}

		// Check if user has contacts
		contactsResp, err := whatsapp.Client.HasContacts(c.Request.Context(), &pb.HasContactsRequest{})
		if err != nil {
			st, ok := status.FromError(err)
			if ok {
				c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to check contacts: " + st.Message()})
			} else {
				c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to check contacts: " + err.Error()})
			}
			return
		}

		if !contactsResp.HasContacts {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Cannot create status: No contacts found. You need at least one contact to post a status."})
			return
		}

		var req dto.CreateWhatsAppStatusRequest

		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}

		var content *pb.MessageContent

		switch req.Type {
		case "text":
			content = &pb.MessageContent{
				ContentType: &pb.MessageContent_Text{
					Text: req.Text,
				},
			}
		case "image", "video", "audio":
			data, err := base64.StdEncoding.DecodeString(req.MediaData)
			if err != nil {
				c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid base64 media data"})
				return
			}
			content = &pb.MessageContent{
				ContentType: &pb.MessageContent_Media{
					Media: &pb.MediaContent{
						MediaType: req.Type,
						Data:      data,
						Filename:  req.Filename,
						Caption:   req.Caption,
						Mimetype:  req.MimeType,
					},
				},
			}
		default:
			c.JSON(http.StatusBadRequest, gin.H{"error": "Unsupported status type"})
			return
		}

		resp, err := whatsapp.Client.CreateStatus(c.Request.Context(), &pb.CreateStatusRequest{
			Content:         content,
			BackgroundColor: req.BackgroundColor,
			Font:            req.Font,
		})

		if err != nil {
			st, ok := status.FromError(err)
			if ok {
				c.JSON(http.StatusInternalServerError, gin.H{"error": st.Message()})
			} else {
				c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			}
			return
		}

		c.JSON(http.StatusOK, gin.H{
			"success":   resp.Success,
			"message":   resp.Message,
			"status_id": resp.StatusId,
		})
	}
}
