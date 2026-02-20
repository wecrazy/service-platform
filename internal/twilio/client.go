package twilio

import (
	"fmt"
	"log"
	"regexp"

	"service-platform/internal/config"

	"github.com/sirupsen/logrus"
	"github.com/twilio/twilio-go"
	openapi "github.com/twilio/twilio-go/rest/api/v2010"
)

// Client handles WhatsApp messaging via Twilio
type messageAPI interface {
	CreateMessage(*openapi.CreateMessageParams) (*openapi.ApiV2010Message, error)
}

// Client is a wrapper around the Twilio REST client for sending WhatsApp messages. It provides methods to send both text and media messages, and it handles configuration and error logging. The client is designed to be initialized with the necessary Twilio credentials and can be used throughout the application to interact with the Twilio API for WhatsApp messaging.
type Client struct {
	restClient   *twilio.RestClient
	api          messageAPI
	twilioNumber string
	accountSid   string
	authToken    string
}

// NewClient initializes a new Twilio WhatsApp client using config
func NewClient() (*Client, error) {
	cfg := config.ServicePlatform.Get()
	twilioConfig := cfg.Twilio

	if twilioConfig.AccountSID == "" || twilioConfig.AuthToken == "" || twilioConfig.WhatsAppNumber == "" {
		return nil, fmt.Errorf("missing required Twilio WhatsApp configuration: account_sid, auth_token, whatsapp_number")
	}

	restClient := twilio.NewRestClientWithParams(twilio.ClientParams{
		Username: twilioConfig.AccountSID,
		Password: twilioConfig.AuthToken,
	})

	tc := &Client{
		restClient:   restClient,
		api:          restClient.Api,
		twilioNumber: twilioConfig.WhatsAppNumber,
		accountSid:   twilioConfig.AccountSID,
		authToken:    twilioConfig.AuthToken,
	}

	logrus.Infof("✅ Twilio WhatsApp client initialized with account: %s", twilioConfig.AccountSID)
	return tc, nil
}

// SendMessage sends a WhatsApp text message via Twilio
// Prerequisites:
// - Phone number format: "+countrycodephonenumber" (E.164 format, e.g., "+6285173207755")
// - Recipient must be in your WhatsApp Sandbox approved list (or have opted in for production)
// - This method supports the 24-hour customer service window after receiving a message from the user
// https://www.twilio.com/docs/whatsapp/api
func (c *Client) SendMessage(to string, message string) (string, error) {
	// Validate phone number format
	if !isValidPhoneNumber(to) {
		err := fmt.Errorf("invalid phone number format: %s (use E.164 format like +6285173207755)", to)
		logrus.Errorf(err.Error())
		return "", err
	}

	params := &openapi.CreateMessageParams{}
	params.SetFrom(c.twilioNumber)
	params.SetTo(fmt.Sprintf("whatsapp:%s", to))
	params.SetBody(message)

	resp, err := c.api.CreateMessage(params)
	if err != nil {
		logrus.Errorf("❌ Failed to send WhatsApp message to %s: %v", to, err)
		return "", err
	}

	logrus.Infof("✅ WhatsApp message sent successfully to %s. SID: %s | Status: %s", to, *resp.Sid, *resp.Status)
	return *resp.Sid, nil
}

// SendMediaMessage sends a media message (image, video, document, audio) via Twilio
// Supported media types: JPG, JPEG, PNG, audio files, PDF (max 16MB)
// Prerequisites: Same as SendMessage (E.164 format, sandbox approval required)
// https://www.twilio.com/docs/whatsapp/api
func (c *Client) SendMediaMessage(to string, mediaUrl string, caption string) (string, error) {
	// Validate phone number format
	if !isValidPhoneNumber(to) {
		err := fmt.Errorf("invalid phone number format: %s (use E.164 format like +6285173207755)", to)
		logrus.Errorf(err.Error())
		return "", err
	}

	params := &openapi.CreateMessageParams{}
	params.SetFrom(c.twilioNumber)
	params.SetTo(fmt.Sprintf("whatsapp:%s", to))
	params.SetMediaUrl([]string{mediaUrl})

	if caption != "" {
		params.SetBody(caption)
	}

	resp, err := c.api.CreateMessage(params)
	if err != nil {
		logrus.Errorf("❌ Failed to send WhatsApp media message to %s: %v", to, err)
		return "", err
	}

	logrus.Infof("✅ WhatsApp media message sent successfully to %s. SID: %s | Media: %s | Status: %s",
		to, *resp.Sid, mediaUrl, *resp.Status)
	return *resp.Sid, nil
}

// Close closes the Twilio client connection
func (c *Client) Close() {
	logrus.Info("🔌 Twilio WhatsApp client closed")
}

// Example usage function
func ExampleSendWhatsAppMessage() {
	client, err := NewClient()
	if err != nil {
		log.Fatalf("Failed to initialize Twilio client: %v", err)
	}
	defer client.Close()

	// Send a simple text message
	sid, err := client.SendMessage("+1234567890", "Hello from Twilio WhatsApp!")
	if err != nil {
		log.Fatalf("Failed to send message: %v", err)
	}

	logrus.Infof("Message sent with SID: %s", sid)

	// Send a message with media
	sid, err = client.SendMediaMessage("+1234567890", "https://example.com/image.jpg", "Check this out!")
	if err != nil {
		log.Fatalf("Failed to send media message: %v", err)
	}

	logrus.Infof("Media message sent with SID: %s", sid)
}

// isValidPhoneNumber validates E.164 format phone numbers
// Valid format: +[country code][number] (e.g., +6285173207755)
// Uses a simple regex pattern to validate the format
func isValidPhoneNumber(phone string) bool {
	// E.164 format: + followed by 1-15 digits
	// Pattern: ^\\+[1-9]\\d{1,14}$
	pattern := "^\\+[1-9]\\d{1,14}$"
	match, err := regexp.MatchString(pattern, phone)
	if err != nil {
		logrus.Errorf("Error validating phone number: %v", err)
		return false
	}
	return match
}
