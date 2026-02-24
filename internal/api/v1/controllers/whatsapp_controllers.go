package controllers

import (
	"context"
	"encoding/base64"
	"fmt"
	"net/http"
	"os"
	"service-platform/internal/api/v1/dto"
	"service-platform/internal/config"
	whatsnyanmodel "service-platform/internal/core/model/whatsnyan_model"
	"service-platform/internal/whatsapp"
	"service-platform/pkg/fun"
	"strings"
	"time"

	pb "service-platform/proto"

	"github.com/gin-gonic/gin"
	"github.com/go-redis/redis/v8"
	"github.com/sirupsen/logrus"
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
// @Failure      503  {object}   dto.APIErrorResponse "Service Unavailable"
// @Router       /api/v1/{access}/tab-whatsapp/send_message [post]
func SendWhatsAppMessage(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		if whatsapp.Client == nil {
			fun.HandleAPIErrorSimple(c, http.StatusServiceUnavailable, "WhatsApp service not available")
			return
		}

		var req dto.SendWhatsAppMessageRequest

		if err := c.ShouldBindJSON(&req); err != nil {
			fun.HandleAPIErrorSimple(c, http.StatusBadRequest, err.Error())
			return
		}

		// Default type to text if not specified
		if req.Type == "" {
			req.Type = "text"
		}

		recipientJID, ok := resolveWARecipientJID(c, &req)
		if !ok {
			return
		}

		content, ok := buildWAMessageContent(c, &req)
		if !ok {
			return
		}

		resp, err := whatsapp.Client.SendMessage(c.Request.Context(), &pb.SendMessageRequest{
			To:      recipientJID.String(),
			Content: content,
		})

		if err != nil {
			if grpcErr, ok := status.FromError(err); ok {
				fun.HandleAPIErrorSimple(c, http.StatusInternalServerError, grpcErr.Message())
				return
			}
			fun.HandleAPIErrorSimple(c, http.StatusInternalServerError, err.Error())
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

// resolveWARecipientJID returns the JID for the recipient described in req.
// Returns (jid, true) on success; writes an error response and returns (zero, false) on failure.
func resolveWARecipientJID(c *gin.Context, req *dto.SendWhatsAppMessageRequest) (types.JID, bool) {
	if req.IsGroup {
		j, err := ValidateGroupJID(req.Recipient)
		if err != nil {
			fun.HandleAPIErrorSimple(c, http.StatusBadRequest, "Invalid group JID: "+err.Error())
			return types.JID{}, false
		}
		return j, true
	}

	sanitizedPhone, err := fun.SanitizeIndonesiaPhoneNumber(req.Recipient)
	if err != nil {
		fun.HandleAPIErrorSimple(c, http.StatusBadRequest, "Invalid phone number: "+err.Error())
		return types.JID{}, false
	}
	jid := config.ServicePlatform.Get().Default.DialingCodeDefault + sanitizedPhone + "@" + types.DefaultUserServer

	resp, err := whatsapp.Client.IsOnWhatsApp(c.Request.Context(), &pb.IsOnWhatsAppRequest{
		PhoneNumbers: []string{jid},
	})
	if err != nil {
		fun.HandleAPIErrorSimple(c, http.StatusInternalServerError, "Failed to check WhatsApp status: "+err.Error())
		return types.JID{}, false
	}
	if !resp.Success {
		fun.HandleAPIErrorSimple(c, http.StatusBadRequest, "Failed to check WhatsApp status: "+resp.Message)
		return types.JID{}, false
	}
	if len(resp.Results) == 0 || !resp.Results[0].IsRegistered {
		fun.HandleAPIErrorSimple(c, http.StatusBadRequest, "Phone number is not registered on WhatsApp")
		return types.JID{}, false
	}
	return types.NewJID(config.ServicePlatform.Get().Default.DialingCodeDefault+sanitizedPhone, types.DefaultUserServer), true
}

// buildWAMessageContent constructs the pb.MessageContent for req.
// Returns (content, true) on success; writes an error response and returns (nil, false) on failure.
func buildWAMessageContent(c *gin.Context, req *dto.SendWhatsAppMessageRequest) (*pb.MessageContent, bool) {
	switch req.Type {
	case "text":
		if req.Message == "" {
			fun.HandleAPIErrorSimple(c, http.StatusBadRequest, "Message content cannot be empty for text type")
			return nil, false
		}
		if len(req.Message) > config.ServicePlatform.Get().Whatsnyan.MaxMessageLength {
			fun.HandleAPIErrorSimple(c, http.StatusBadRequest, fmt.Sprintf("Message content exceeds maximum length of %d characters", config.ServicePlatform.Get().Whatsnyan.MaxMessageLength))
			return nil, false
		}
		return &pb.MessageContent{
			ContentType: &pb.MessageContent_Text{Text: req.Message},
		}, true
	case "image", "video", "audio", "document":
		if req.MediaData == "" {
			fun.HandleAPIErrorSimple(c, http.StatusBadRequest, "Media data is required")
			return nil, false
		}
		data, err := base64.StdEncoding.DecodeString(req.MediaData)
		if err != nil {
			fun.HandleAPIErrorSimple(c, http.StatusBadRequest, "Invalid base64 media data")
			return nil, false
		}
		return &pb.MessageContent{
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
		}, true
	case "location":
		return &pb.MessageContent{
			ContentType: &pb.MessageContent_Location{
				Location: &pb.LocationContent{
					Latitude:  req.Latitude,
					Longitude: req.Longitude,
					Name:      req.LocName,
					Address:   req.Address,
				},
			},
		}, true
	case "live_location":
		return &pb.MessageContent{
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
		}, true
	case "poll":
		return &pb.MessageContent{
			ContentType: &pb.MessageContent_Poll{
				Poll: &pb.PollContent{
					Name:                   req.PollName,
					Options:                req.PollOptions,
					SelectableOptionsCount: req.SelectableCount,
				},
			},
		}, true
	case "contact":
		return &pb.MessageContent{
			ContentType: &pb.MessageContent_Contact{
				Contact: &pb.ContactContent{
					DisplayName: req.ContactName,
					Vcard:       req.Vcard,
				},
			},
		}, true
	default:
		fun.HandleAPIErrorSimple(c, http.StatusBadRequest, "Invalid message type")
		return nil, false
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
// @Failure      503  {object}   dto.APIErrorResponse "Service Unavailable"
// @Failure      500  {object}   dto.APIErrorResponse "Internal Server Error"
// @Router       /api/v1/{access}/tab-whatsapp/connect [post]
func ConnectWhatsApp(redisDB *redis.Client) gin.HandlerFunc {
	return func(c *gin.Context) {
		if whatsapp.Client == nil {
			fun.HandleAPIErrorSimple(c, http.StatusServiceUnavailable, "WhatsApp service not available")
			return
		}

		// Accept optional phone number and force_qr flag
		var req struct {
			PhoneNumber string `json:"phone_number"`
			ForceQR     bool   `json:"force_qr"`
		}
		// Don't fail if no body provided, just use empty values
		c.ShouldBindJSON(&req)

		resp, err := whatsapp.Client.Connect(c.Request.Context(), &pb.ConnectRequest{
			PhoneNumber: req.PhoneNumber,
			ForceQr:     req.ForceQR,
		})

		if err != nil {
			if grpcErr, ok := status.FromError(err); ok {
				fun.HandleAPIErrorSimple(c, http.StatusInternalServerError, grpcErr.Message())
				return
			}
			fun.HandleAPIErrorSimple(c, http.StatusInternalServerError, err.Error())
			return
		}

		qrCode := resp.QrCode

		// If response contains a file path (PNG file), generate secure token and proxy URL
		if resp.Success && strings.HasSuffix(resp.QrCode, ".png") && strings.Contains(resp.QrCode, "qr_") {
			// Generate secure token for QR access
			token := fmt.Sprintf("qr_%d_%s", time.Now().Unix(), fun.GenerateRandomString(16))

			// Store token in Redis with 5-minute expiration
			ctx := context.Background()
			key := fmt.Sprintf("qr_token:%s", token)
			err := redisDB.Set(ctx, key, resp.QrCode, 5*time.Minute).Err()
			if err != nil {
				logrus.Errorf("Failed to store QR token in Redis: %v", err)
				// Fall back to returning the raw QR data if token storage fails
			} else {
				// Generate proxy URL
				randomAccess := c.Param("access")
				qrCode = fmt.Sprintf("/api/v1/%s/tab-whatsapp/qr/%s", randomAccess, token)
			}
		}

		c.JSON(http.StatusOK, gin.H{
			"success":      resp.Success,
			"message":      resp.Message,
			"qr_code":      qrCode,
			"pairing_code": resp.PairingCode,
		})
	}
}

// DisconnectWhatsApp disconnects the WhatsApp client
// DisconnectWhatsApp godoc
// @Summary      Disconnect WhatsApp
// @Description  Disconnects from WhatsApp
// @Tags         WhatsApp
// @Accept       json
// @Produce      json
// @Success      200  {object}   map[string]string
// @Failure      503  {object}   dto.APIErrorResponse "Service Unavailable"
// @Failure      500  {object}   dto.APIErrorResponse "Internal Server Error"
// @Router       /api/v1/{access}/tab-whatsapp/disconnect [post]
func DisconnectWhatsApp(c *gin.Context) {
	if whatsapp.Client == nil {
		fun.HandleAPIErrorSimple(c, http.StatusServiceUnavailable, "WhatsApp service not available")
		return
	}

	resp, err := whatsapp.Client.Disconnect(c.Request.Context(), &pb.DisconnectRequest{})

	if err != nil {
		if grpcErr, ok := status.FromError(err); ok {
			fun.HandleAPIErrorSimple(c, http.StatusInternalServerError, grpcErr.Message())
			return
		}
		fun.HandleAPIErrorSimple(c, http.StatusInternalServerError, err.Error())
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
// @Failure      503  {object}   dto.APIErrorResponse "Service Unavailable"
// @Failure      500  {object}   dto.APIErrorResponse "Internal Server Error"
// @Router       /api/v1/{access}/tab-whatsapp/logout [post]
func LogoutWhatsApp(c *gin.Context) {
	if whatsapp.Client == nil {
		fun.HandleAPIErrorSimple(c, http.StatusServiceUnavailable, "WhatsApp service not available")
		return
	}

	resp, err := whatsapp.Client.Logout(c.Request.Context(), &pb.WALogoutRequest{})

	if err != nil {
		if grpcErr, ok := status.FromError(err); ok {
			fun.HandleAPIErrorSimple(c, http.StatusInternalServerError, grpcErr.Message())
			return
		}
		fun.HandleAPIErrorSimple(c, http.StatusInternalServerError, err.Error())
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
// @Failure      503  {object}   dto.APIErrorResponse "Service Unavailable"
// @Failure      500  {object}   dto.APIErrorResponse "Internal Server Error"
// @Router       /api/v1/{access}/tab-whatsapp/refresh_qr [post]
func RefreshWhatsAppQR(redisDB *redis.Client) gin.HandlerFunc {
	return func(c *gin.Context) {
		if whatsapp.Client == nil {
			fun.HandleAPIErrorSimple(c, http.StatusServiceUnavailable, "WhatsApp service not available")
			return
		}

		// Parse request body for force_new parameter
		var reqBody struct {
			ForceNew bool `json:"force_new"`
		}
		if err := c.ShouldBindJSON(&reqBody); err != nil {
			// If JSON parsing fails, default to force_new=false
			reqBody.ForceNew = false
		}

		resp, err := whatsapp.Client.RefreshQR(c.Request.Context(), &pb.RefreshQRRequest{
			ForceNew: reqBody.ForceNew,
		})

		if err != nil {
			if grpcErr, ok := status.FromError(err); ok {
				fun.HandleAPIErrorSimple(c, http.StatusInternalServerError, grpcErr.Message())
				return
			}
			fun.HandleAPIErrorSimple(c, http.StatusInternalServerError, err.Error())
			return
		}

		qrCode := resp.QrCode

		// If response contains a file path (PNG file), generate secure token and proxy URL
		if resp.Success && strings.HasSuffix(resp.QrCode, ".png") && strings.Contains(resp.QrCode, "qr_") {
			// Generate secure token for QR access
			token := fmt.Sprintf("qr_%d_%s", time.Now().Unix(), fun.GenerateRandomString(16))

			// Store token in Redis with 5-minute expiration
			ctx := context.Background()
			key := fmt.Sprintf("qr_token:%s", token)
			err := redisDB.Set(ctx, key, resp.QrCode, 5*time.Minute).Err()
			if err != nil {
				logrus.Errorf("Failed to store QR token in Redis: %v", err)
				// Fall back to returning the raw QR data if token storage fails
			} else {
				// Generate proxy URL
				randomAccess := c.Param("access")
				qrCode = fmt.Sprintf("/api/v1/%s/tab-whatsapp/qr/%s", randomAccess, token)
			}
		}

		c.JSON(http.StatusOK, gin.H{
			"success": resp.Success,
			"message": resp.Message,
			"qr_code": qrCode,
		})
	}
}

// CreateStatus godoc
// @Summary      Create WhatsApp Status
// @Description  Creates a new WhatsApp status
// @Tags         WhatsApp
// @Accept       json
// @Produce      json
// @Param        request body dto.CreateWhatsAppStatusRequest true "Status Request"
// @Success      200  {object}   map[string]string
// @Failure      503  {object}   dto.APIErrorResponse "Service Unavailable"
// @Failure      400  {object}   dto.APIErrorResponse "Bad Request"
// @Failure      500  {object}   dto.APIErrorResponse "Internal Server Error"
// @Router       /api/v1/{access}/tab-whatsapp/create_status [post]
func CreateStatus(_ *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		if whatsapp.Client == nil {
			fun.HandleAPIErrorSimple(c, http.StatusServiceUnavailable, "WhatsApp service not available")
			return
		}

		// Check if user has contacts
		contactsResp, err := whatsapp.Client.HasContacts(c.Request.Context(), &pb.HasContactsRequest{})
		if err != nil {
			st, ok := status.FromError(err)
			if ok {
				fun.HandleAPIErrorSimple(c, http.StatusInternalServerError, "Failed to check contacts: "+st.Message())
			} else {
				fun.HandleAPIErrorSimple(c, http.StatusInternalServerError, "Failed to check contacts: "+err.Error())
			}
			return
		}

		if !contactsResp.HasContacts {
			fun.HandleAPIErrorSimple(c, http.StatusBadRequest, "Cannot create status: No contacts found. You need at least one contact to post a status.")
			return
		}

		var req dto.CreateWhatsAppStatusRequest

		if err := c.ShouldBindJSON(&req); err != nil {
			fun.HandleAPIErrorSimple(c, http.StatusBadRequest, err.Error())
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
				fun.HandleAPIErrorSimple(c, http.StatusBadRequest, "Invalid base64 media data")
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
			fun.HandleAPIErrorSimple(c, http.StatusBadRequest, "Unsupported status type")
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
				fun.HandleAPIErrorSimple(c, http.StatusInternalServerError, st.Message())
			} else {
				fun.HandleAPIErrorSimple(c, http.StatusInternalServerError, err.Error())
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

// ServeQRImage serves QR code images with secure token verification
func ServeQRImage(redisDB *redis.Client) gin.HandlerFunc {
	return func(c *gin.Context) {
		// Get token from URL parameter
		token := c.Param("token")
		if token == "" {
			fun.HandleAPIErrorSimple(c, http.StatusBadRequest, "Token required")
			return
		}

		// Verify token in Redis
		ctx := context.Background()
		key := fmt.Sprintf("qr_token:%s", token)
		qrPath, err := redisDB.Get(ctx, key).Result()
		if err != nil {
			if err == redis.Nil {
				fun.HandleAPIErrorSimple(c, http.StatusUnauthorized, "Invalid or expired token")
				return
			}
			fun.HandleAPIErrorSimple(c, http.StatusInternalServerError, "Token verification failed")
			return
		}

		// Check if file exists
		if _, err := os.Stat(qrPath); os.IsNotExist(err) {
			fun.HandleAPIErrorSimple(c, http.StatusNotFound, "QR image not found")
			return
		}

		// Serve the image file
		c.File(qrPath)
	}
}
