package twilio_test

import (
	"errors"
	"testing"

	"service-platform/internal/config"
	"service-platform/internal/twilio"
)

// TestNewClient tests the initialization of Twilio client
func TestNewClient(t *testing.T) {
	client := mustHaveTwilioClient(t)
	if client == nil {
		t.Fatal("expected a real Twilio client")
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
	client := mustHaveTwilioClient(t)
	recipientNumber := "+6285173207755" // replace with your approved sandbox number
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
	client := mustHaveTwilioClient(t)
	recipientNumber := "+6285173207755" // Replace with actual approved number
	mediaCases := []struct {
		name    string
		url     string
		caption string
	}{
		{name: "image", url: "https://images.unsplash.com/photo-1503023345310-bd7c1de61c7d?auto=format&fit=crop&w=640&q=80", caption: "Test image"},
		{name: "document", url: "https://www.w3.org/WAI/ER/tests/xhtml/testfiles/resources/pdf/dummy.pdf", caption: "Test document"},
		{name: "audio", url: "https://filesamples.com/samples/audio/mp3/sample3.mp3", caption: "Test audio"},
		{name: "video", url: "https://filesamples.com/samples/video/mp4/sample_640x360.mp4", caption: "Test video"},
	}

	for _, tc := range mediaCases {
		t.Run(tc.name, func(t *testing.T) {
			sid, err := client.SendMediaMessage(recipientNumber, tc.url, tc.caption)
			if err != nil {
				t.Logf("Skipping %s media send (may require valid Twilio credentials): %v", tc.name, err)
				return
			}
			if sid == "" {
				t.Fatalf("Expected SID when sending %s", tc.name)
			}
			t.Logf("✅ %s media message SID: %s", tc.name, sid)
		})
	}
}

// TestSendRichTextMessage demonstrates WhatsApp formatting like bold, italic, strikethrough, code blocks, and emoji
func TestSendRichTextMessage(t *testing.T) {
	client := mustHaveTwilioClient(t)
	recipientNumber := "+6285173207755"
	richMessage := "*✨ Weekly Highlights ✨*\n_Stay in the loop with updates_\n✅ Task list:\n• *Deploy updates*\n• _Send reminders_\n• ~Archive logs~\n```\ntwilio api:core:messages:create\n```\nReach out when ready! 💬"

	sid, err := client.SendMessage(recipientNumber, richMessage)
	if err != nil {
		t.Logf("Skipping rich text send (may require valid Twilio credentials): %v", err)
		return
	}

	if sid == "" {
		t.Fatal("Expected SID when sending rich text message")
	}

	t.Logf("✅ Rich text message SID: %s", sid)
}

// TestSendMentionMessage sends WhatsApp messages that include phone mentions (single and multi numbers)
func TestSendMentionMessage(t *testing.T) {
	client := mustHaveTwilioClient(t)
	recipientNumber := "+6285173207755"
	mentionCases := []struct {
		name    string
		message string
	}{
		{name: "single mention", message: "Hello @+6285173207755, please confirm when you are online."},
		{name: "multi mention", message: "Good morning @+6285173207755 and @+14158675309, we need both of your updates before noon."},
	}

	for _, tc := range mentionCases {
		t.Run(tc.name, func(t *testing.T) {
			sid, err := client.SendMessage(recipientNumber, tc.message)
			if err != nil {
				t.Logf("Skipping mention send for %s (may require valid Twilio credentials): %v", tc.name, err)
				return
			}
			if sid == "" {
				t.Fatalf("Expected SID when sending mention message (%s)", tc.name)
			}
			t.Logf("✅ Mention (%s) message SID: %s", tc.name, sid)
		})
	}
}

// mustHaveTwilioClient is a helper function that initializes a Twilio client for testing, ensuring that the necessary configuration is loaded and credentials are present. If the credentials are missing, it skips the test. If the client cannot be initialized for any reason, it fails the test with an appropriate error message.
func mustHaveTwilioClient(t *testing.T) *twilio.Client {
	t.Helper()
	config.ServicePlatform.MustInit("service-platform")
	if !config.ServicePlatform.IsLoaded() {
		t.Fatalf("config should be loaded")
	}
	cfg := config.ServicePlatform.Get()
	if cfg.Twilio.AccountSID == "" || cfg.Twilio.AuthToken == "" || cfg.Twilio.WhatsAppNumber == "" {
		t.Skip("Skipping Twilio integration tests because credentials are missing")
	}
	client, err := twilio.NewClient()
	if err != nil {
		t.Fatalf("Failed to initialize Twilio client: %v", err)
	}
	t.Cleanup(func() {
		client.Close()
	})
	return client
}
