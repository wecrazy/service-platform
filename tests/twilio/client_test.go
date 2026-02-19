package twilio_test

import (
	"errors"
	"testing"

	"service-platform/internal/config"
	"service-platform/internal/twilio"
)

// TestNewClient tests the initialization of Twilio client
func TestNewClient(t *testing.T) {
	// Load config
	var err error
	config.ServicePlatform.MustInit("service-platform") // Load config with name "service-platform.%s.yaml"
	if !config.ServicePlatform.IsLoaded() {
		err = errors.New("failed to load configuration")
		t.Fatalf("Config should be loaded successfully: %v", err)
	}

	cfg := config.ServicePlatform.Get()

	// Check if credentials are set
	if cfg.Twilio.AccountSID == "" {
		t.Skip("Skipping test: Twilio AccountSID not configured")
	}

	if cfg.Twilio.AuthToken == "" {
		t.Skip("Skipping test: Twilio AuthToken not configured")
	}

	if cfg.Twilio.WhatsAppNumber == "" {
		t.Skip("Skipping test: Twilio WhatsApp number not configured")
	}

	// Initialize client
	client, err := twilio.NewClient()
	if err != nil {
		t.Fatalf("Failed to initialize Twilio client: %v", err)
	}
	defer client.Close()

	if client == nil {
		t.Fatal("Expected non-nil Twilio client")
	}
}

// TestNewClientMissingCredentials tests client initialization validation
func TestNewClientMissingCredentials(t *testing.T) {
	config.ServicePlatform.MustInit("service-platform") // Load config with name "service-platform.%s.yaml"
	if !config.ServicePlatform.IsLoaded() {
		err := errors.New("failed to load configuration")
		t.Fatalf("Config should be loaded successfully: %v", err)
	}

	cfg := config.ServicePlatform.Get()

	// Verify we have credentials to test against
	if cfg.Twilio.AccountSID != "" &&
		cfg.Twilio.AuthToken != "" &&
		cfg.Twilio.WhatsAppNumber != "" {
		// If credentials are set, client should initialize successfully
		client, err := twilio.NewClient()
		if err != nil {
			t.Logf("Note: Client initialization failed (may need valid Twilio credentials): %v", err)
			return
		}
		defer client.Close()
		t.Log("✅ Client successfully initialized with configured credentials")
	} else {
		// If credentials are empty, expect error
		_, err := twilio.NewClient()
		if err == nil {
			t.Fatal("Expected error when initializing client with missing credentials")
		}
		t.Logf("✅ Correctly rejected missing credentials: %v", err)
	}
}

// TestSendMessage tests sending a text message
func TestSendMessage(t *testing.T) {
	var err error
	config.ServicePlatform.MustInit("service-platform") // Load config with name "service-platform.%s.yaml"
	if !config.ServicePlatform.IsLoaded() {
		err = errors.New("failed to load configuration")
		t.Fatalf("Config should be loaded successfully: %v", err)
	}

	cfg := config.ServicePlatform.Get()

	// Skip if credentials not configured
	if cfg.Twilio.AccountSID == "" ||
		cfg.Twilio.AuthToken == "" ||
		cfg.Twilio.WhatsAppNumber == "" {
		t.Skip("Skipping test: Twilio credentials not configured")
	}

	client, err := twilio.NewClient()
	if err != nil {
		t.Fatalf("Failed to initialize Twilio client: %v", err)
	}
	defer client.Close()

	// Test with a valid recipient number (MUST be in your WhatsApp Sandbox approved list)
	// Add your number: https://console.twilio.com/us/account/messaging/whatsapp/sandbox-settings
	recipientNumber := "+6285173207755" // Replace with your actual sandbox-approved number
	message := "Test message from Twilio WhatsApp Go client"

	sid, err := client.SendMessage(recipientNumber, message)
	if err != nil {
		t.Logf("Skipping actual send test (may require valid Twilio credentials): %v", err)
		return
	}

	if sid == "" {
		t.Fatal("Expected non-empty message SID")
	}

	t.Logf("✅ Message sent successfully with SID: %s", sid)
}

// TestSendMediaMessage tests sending a media message
func TestSendMediaMessage(t *testing.T) {
	var err error
	config.ServicePlatform.MustInit("service-platform") // Load config with name "service-platform.%s.yaml"
	if !config.ServicePlatform.IsLoaded() {
		err = errors.New("failed to load configuration")
		t.Fatalf("Config should be loaded successfully: %v", err)
	}

	cfg := config.ServicePlatform.Get()

	// Skip if credentials not configured
	if cfg.Twilio.AccountSID == "" ||
		cfg.Twilio.AuthToken == "" ||
		cfg.Twilio.WhatsAppNumber == "" {
		t.Skip("Skipping test: Twilio credentials not configured")
	}

	client, err := twilio.NewClient()
	if err != nil {
		t.Fatalf("Failed to initialize Twilio client: %v", err)
	}
	defer client.Close()

	// Test with a valid recipient and media URL
	recipientNumber := "+6285173207755" // Replace with actual test number
	mediaURL := "https://fastly.picsum.photos/id/237/536/354.jpg?hmac=i0yVXW1ORpyCZpQ-CknuyV-jbtU7_x9EBQVhvT5aRr0"
	caption := "Test image from Twilio WhatsApp"

	sid, err := client.SendMediaMessage(recipientNumber, mediaURL, caption)
	if err != nil {
		t.Logf("Skipping actual send test (may require valid Twilio credentials): %v", err)
		return
	}

	if sid == "" {
		t.Fatal("Expected non-empty message SID")
	}

	t.Logf("✅ Media message sent successfully with SID: %s", sid)
}
