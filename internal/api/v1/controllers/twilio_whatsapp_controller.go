package controllers

import (
	"net/http"
	"service-platform/internal/api/v1/dto"
	"service-platform/internal/twilio"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/sirupsen/logrus"
	"gorm.io/gorm"
)

// HandleTwilioWhatsAppWebhook handles incoming WhatsApp messages from Twilio Sandbox
// @Summary     Handle incoming Twilio WhatsApp messages
// @Description Webhook endpoint that receives incoming WhatsApp messages from Twilio Sandbox
// @Tags        Twilio WhatsApp
// @Accept      x-www-form-urlencoded
// @Produce     xml
// @Param       From        formData string true  "Sender phone number (e.g., whatsapp:+1234567890)"
// @Param       To          formData string true  "Recipient phone number (e.g., whatsapp:+14155238886)"
// @Param       Body        formData string true  "Message body"
// @Param       MessageSid  formData string true  "Twilio Message SID"
// @Success     200         {string} string      "TwiML XML response"
// @Failure     400         {string} string      "Invalid request"
// @Router      /twilio_reply [post]
func HandleTwilioWhatsAppWebhook(_ *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		// Parse incoming webhook from Twilio
		var req dto.HandleTwilioWhatsAppWebhookRequest

		if err := c.ShouldBind(&req); err != nil {
			logrus.Errorf("❌ Failed to parse Twilio webhook: %v", err)
			c.String(http.StatusBadRequest, "Invalid request")
			return
		}

		// Log incoming message with details
		logrus.Infof("📨 Twilio WhatsApp incoming message")
		logrus.Infof("   From: %s", req.From)
		logrus.Infof("   To: %s", req.To)
		logrus.Infof("   MessageSid: %s", req.MessageSid)
		logrus.Infof("   Body: %s", req.Body)
		logrus.Infof("   Received at: %s", time.Now().Format(time.RFC3339))

		// ADD: Store message in database if needed
		// Example:
		// incomingMsg := &model.TwilioWhatsAppIncomingMessage{
		//     MessageSid: req.MessageSid,
		//     From:       req.From,
		//     To:         req.To,
		//     Body:       req.Body,
		//     ReceivedAt: time.Now(),
		// }
		// if err := db.Create(incomingMsg).Error; err != nil {
		//     logrus.Errorf("❌ Failed to store incoming message: %v", err)
		// }

		// Generate auto-reply based on message content
		responseMessage := "Thank you for your message! We'll respond shortly."

		// Process business logic (auto-reply, routing, intelligent response)
		if len(req.Body) > 0 {
			// Example: Custom responses based on keywords
			lowerBody := strings.ToLower(req.Body)
			switch {
			case strings.Contains(lowerBody, "hello") || strings.Contains(lowerBody, "halo"):
				responseMessage = "Hello! 👋 How can we help you today?"
			case strings.Contains(lowerBody, "help"):
				responseMessage = "I'm here to help! Please let me know what you need assistance with."
			case strings.Contains(lowerBody, "thank"):
				responseMessage = "You're welcome! Happy to help! 😊"
			default:
				responseMessage = "Thank you for reaching out! We'll get back to you soon."
			}
		}

		// TODO: enable it again if you need to send the response message via Twilio API instead of TwiML (currently just sending empty TwiML response)
		_ = responseMessage // To avoid unused variable warning if auto-reply logic is not implemented yet
		// // Send auto-reply via Twilio API (not TwiML - this actually sends a message)
		// client, err := twilio.NewClient()
		// if err != nil {
		// 	logrus.Errorf("❌ Failed to create Twilio client for reply: %v", err)
		// } else {
		// 	defer client.Close()

		// 	// Send message back to the sender
		// 	// Extract phone number from "whatsapp:+6285173207755" format
		// 	replyToPhone := strings.ReplaceAll(req.From, "whatsapp:", "")
		// 	topicName := "Twilio WhatsApp"

		// 	messageSID, err := client.SendMessage(replyToPhone, responseMessage)
		// 	if err != nil {
		// 		logrus.Errorf("❌ Failed to send auto-reply to %s: %v", replyToPhone, err)
		// 	} else {
		// 		logrus.Infof("✅ Auto-reply sent successfully to %s. SID: %s", replyToPhone, messageSID)
		// 		logrus.Infof("   Topic: %s", topicName)
		// 		logrus.Infof("   Message: %s", responseMessage)
		// 	}
		// }

		// Return TwiML 200 OK (just acknowledgment, not the actual reply)
		twimlResponse := `<?xml version="1.0" encoding="UTF-8"?>
<Response>
</Response>`

		logrus.Infof("✅ Webhook acknowledgment sent")

		c.Header("Content-Type", "application/xml")
		c.String(http.StatusOK, twimlResponse)
	}
}

// SendTwilioWhatsAppMessage sends a WhatsApp message via Twilio (authenticated)
// @Summary     Send WhatsApp message via Twilio
// @Description Send a text message to a WhatsApp number using Twilio API
// @Tags        Twilio WhatsApp
// @Accept      json
// @Produce     json
// @Security    ApiKeyAuth
// @Param       body body     dto.SendTwilioWhatsAppMessageRequest true "Message request body"
// @Success     200  {object} dto.SendTwilioWhatsAppMessageResponse
// @Failure     400  {object} dto.TwilioWhatsAppErrorResponse
// @Failure     500  {object} dto.TwilioWhatsAppErrorResponse
// @Router      /api/v1/{access}/tab-twilio-whatsapp/send_message [post]
func SendTwilioWhatsAppMessage(_ *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		var req dto.SendTwilioWhatsAppMessageRequest

		if err := c.ShouldBindJSON(&req); err != nil {
			logrus.Errorf("❌ Invalid request: %v", err)
			c.JSON(http.StatusBadRequest, dto.TwilioWhatsAppErrorResponse{
				Error:     err.Error(),
				Timestamp: time.Now().Format(time.RFC3339),
			})
			return
		}

		// Create Twilio client
		client, err := twilio.NewClient()
		if err != nil {
			logrus.Errorf("❌ Failed to create Twilio client: %v", err)
			c.JSON(http.StatusInternalServerError, dto.TwilioWhatsAppErrorResponse{
				Error:     "Failed to initialize Twilio",
				Timestamp: time.Now().Format(time.RFC3339),
			})
			return
		}
		defer client.Close()

		// Send message
		messageSID, err := client.SendMessage(req.To, req.Message)
		if err != nil {
			logrus.Errorf("❌ Failed to send WhatsApp message to %s: %v", req.To, err)
			c.JSON(http.StatusInternalServerError, dto.TwilioWhatsAppErrorResponse{
				Error:     err.Error(),
				To:        req.To,
				Timestamp: time.Now().Format(time.RFC3339),
			})
			return
		}

		logrus.Infof("✅ WhatsApp message sent successfully to %s. SID: %s", req.To, messageSID)
		c.JSON(http.StatusOK, dto.SendTwilioWhatsAppMessageResponse{
			MessageSid: messageSID,
			To:         req.To,
			Status:     "queued",
			Timestamp:  time.Now(),
		})
	}
}

// SendTwilioWhatsAppMediaMessage sends a WhatsApp media message via Twilio (authenticated)
// @Summary     Send WhatsApp media message via Twilio
// @Description Send media (image, document, etc.) to a WhatsApp number using Twilio API
// @Tags        Twilio WhatsApp
// @Accept      json
// @Produce     json
// @Security    ApiKeyAuth
// @Param       body body     dto.SendTwilioWhatsAppMediaMessageRequest true "Media message request body"
// @Success     200  {object} dto.SendTwilioWhatsAppMediaMessageResponse
// @Failure     400  {object} dto.TwilioWhatsAppErrorResponse
// @Failure     500  {object} dto.TwilioWhatsAppErrorResponse
// @Router      /api/v1/{access}/tab-twilio-whatsapp/send_media_message [post]
func SendTwilioWhatsAppMediaMessage(_ *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		var req dto.SendTwilioWhatsAppMediaMessageRequest

		if err := c.ShouldBindJSON(&req); err != nil {
			logrus.Errorf("❌ Invalid request: %v", err)
			c.JSON(http.StatusBadRequest, dto.TwilioWhatsAppErrorResponse{
				Error:     err.Error(),
				Timestamp: time.Now().Format(time.RFC3339),
			})
			return
		}

		// Create Twilio client
		client, err := twilio.NewClient()
		if err != nil {
			logrus.Errorf("❌ Failed to create Twilio client: %v", err)
			c.JSON(http.StatusInternalServerError, dto.TwilioWhatsAppErrorResponse{
				Error:     "Failed to initialize Twilio",
				Timestamp: time.Now().Format(time.RFC3339),
			})
			return
		}
		defer client.Close()

		// Send media message
		messageSID, err := client.SendMediaMessage(req.To, req.MediaURL, req.Caption)
		if err != nil {
			logrus.Errorf("❌ Failed to send WhatsApp media message to %s: %v", req.To, err)
			c.JSON(http.StatusInternalServerError, dto.TwilioWhatsAppErrorResponse{
				Error:     err.Error(),
				To:        req.To,
				Timestamp: time.Now().Format(time.RFC3339),
			})
			return
		}

		logrus.Infof("✅ WhatsApp media message sent successfully to %s. SID: %s", req.To, messageSID)
		c.JSON(http.StatusOK, dto.SendTwilioWhatsAppMediaMessageResponse{
			MessageSid: messageSID,
			To:         req.To,
			MediaURL:   req.MediaURL,
			Status:     "queued",
			Timestamp:  time.Now(),
		})
	}
}

// GetTwilioWhatsAppMessages retrieves sent WhatsApp messages (authenticated)
// @Summary     Get sent WhatsApp messages
// @Description Retrieve history of sent WhatsApp messages via Twilio
// @Tags        Twilio WhatsApp
// @Accept      json
// @Produce     json
// @Security    ApiKeyAuth
// @Success     200 {object} dto.TwilioWhatsAppMessageListResponse
// @Router      /api/v1/{access}/tab-twilio-whatsapp/messages [get]
func GetTwilioWhatsAppMessages(_ *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		// TODO: Implement message history retrieval from database
		logrus.Infof("📊 Retrieving sent WhatsApp messages")
		c.JSON(http.StatusOK, dto.TwilioWhatsAppMessageListResponse{
			Messages: []dto.TwilioWhatsAppMessageHistory{},
			Count:    0,
		})
	}
}

// GetTwilioWhatsAppIncomingMessages retrieves incoming WhatsApp messages (authenticated)
// @Summary     Get incoming WhatsApp messages
// @Description Retrieve history of incoming WhatsApp messages received via Twilio
// @Tags        Twilio WhatsApp
// @Accept      json
// @Produce     json
// @Security    ApiKeyAuth
// @Success     200 {object} dto.TwilioWhatsAppMessageListResponse
// @Router      /api/v1/{access}/tab-twilio-whatsapp/incoming [get]
func GetTwilioWhatsAppIncomingMessages(_ *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		// TODO: Implement incoming message history retrieval from database
		logrus.Infof("📊 Retrieving incoming WhatsApp messages")
		c.JSON(http.StatusOK, dto.TwilioWhatsAppMessageListResponse{
			Messages: []dto.TwilioWhatsAppMessageHistory{},
			Count:    0,
		})
	}
}
