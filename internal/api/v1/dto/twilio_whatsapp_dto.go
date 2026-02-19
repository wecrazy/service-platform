package dto

import "time"

// Twilio WhatsApp Request DTOs

// SendTwilioWhatsAppMessageRequest represents a request to send a WhatsApp message
type SendTwilioWhatsAppMessageRequest struct {
	To      string `json:"to" binding:"required" example:"+6285173207755" description:"Recipient phone number in E.164 format"`
	Message string `json:"message" binding:"required" example:"Hello from Twilio WhatsApp!" description:"Message body"`
}

// SendTwilioWhatsAppMediaMessageRequest represents a request to send a WhatsApp media message
type SendTwilioWhatsAppMediaMessageRequest struct {
	To       string `json:"to" binding:"required" example:"+6285173207755" description:"Recipient phone number in E.164 format"`
	MediaUrl string `json:"media_url" binding:"required" example:"https://example.com/image.jpg" description:"URL to media file"`
	Caption  string `json:"caption" example:"Check this out!" description:"Optional caption for the media"`
}

// HandleTwilioWhatsAppWebhookRequest represents incoming webhook from Twilio
type HandleTwilioWhatsAppWebhookRequest struct {
	From       string `form:"From" binding:"required" description:"Sender phone number (whatsapp:+xxx)"`
	To         string `form:"To" binding:"required" description:"Recipient phone number (whatsapp:+xxx)"`
	Body       string `form:"Body" binding:"required" description:"Message body"`
	MessageSid string `form:"MessageSid" binding:"required" description:"Twilio Message SID"`
}

// Twilio WhatsApp Response DTOs

// SendTwilioWhatsAppMessageResponse represents the response after sending a message
type SendTwilioWhatsAppMessageResponse struct {
	MessageSid string    `json:"message_sid" example:"SMxxxxxxxxxxxxxxxxxxxxxxxxxx" description:"Twilio Message SID"`
	To         string    `json:"to" example:"+6285173207755" description:"Recipient phone number"`
	Status     string    `json:"status" example:"queued" description:"Current message status (queued, sent, delivered, failed)"`
	Timestamp  time.Time `json:"timestamp" description:"When the message was sent"`
}

// SendTwilioWhatsAppMediaMessageResponse represents the response after sending a media message
type SendTwilioWhatsAppMediaMessageResponse struct {
	MessageSid string    `json:"message_sid" example:"SMxxxxxxxxxxxxxxxxxxxxxxxxxx" description:"Twilio Message SID"`
	To         string    `json:"to" example:"+6285173207755" description:"Recipient phone number"`
	MediaUrl   string    `json:"media_url" description:"URL of the media that was sent"`
	Status     string    `json:"status" example:"queued" description:"Current message status (queued, sent, delivered, failed)"`
	Timestamp  time.Time `json:"timestamp" description:"When the message was sent"`
}

// TwilioWhatsAppMessageHistory represents a sent or incoming message
type TwilioWhatsAppMessageHistory struct {
	ID         string    `json:"id" description:"Message ID in our system"`
	MessageSid string    `json:"message_sid" description:"Twilio Message SID"`
	From       string    `json:"from" description:"Sender phone number"`
	To         string    `json:"to" description:"Recipient phone number"`
	Body       string    `json:"body" description:"Message body"`
	Status     string    `json:"status" description:"Message status"`
	Direction  string    `json:"direction" example:"outbound" description:"Direction: outbound or inbound"`
	CreatedAt  time.Time `json:"created_at" description:"When the message was created"`
}

// TwilioWhatsAppMessageListResponse represents a list of messages
type TwilioWhatsAppMessageListResponse struct {
	Messages []TwilioWhatsAppMessageHistory `json:"messages" description:"List of messages"`
	Count    int                            `json:"count" example:"10" description:"Total number of messages"`
}

// TwilioWhatsAppErrorResponse represents an error response
type TwilioWhatsAppErrorResponse struct {
	Error     string `json:"error" description:"Error message"`
	To        string `json:"to,omitempty" description:"Recipient phone number (if applicable)"`
	Timestamp string `json:"timestamp" description:"When the error occurred"`
}
