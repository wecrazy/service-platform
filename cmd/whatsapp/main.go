// Package main is the entry point for the WhatsApp (whatsmeow) gRPC service.
//
// This is the most feature-rich microservice in the platform. It uses whatsmeow
// to maintain a real WhatsApp Web session and exposes gRPC RPCs defined in
// proto/whatsapp.proto including: SendMessage, CreateStatus, HasContacts,
// GetMessages, Connect (QR / phone pairing), Disconnect, IsConnected, Logout,
// RefreshQR, GetMe, GetGroupInfo, IsOnWhatsApp, GetJoinedGroups, SyncGroups,
// and GetProfilePicture.
//
// Incoming messages (text, media, reactions, replies, edits) are processed in
// background goroutines with support for keyword search, language prompts,
// auto-reply, document validation, user quota checking, and group interaction
// control. Calls are auto-rejected. Startup/shutdown notifications are sent to
// the configured superuser phone number.
//
// Configuration (service-platform.<env>.yaml) sections used:
//
//	Whatsnyan.*          WhatsApp-specific settings (gRPC port, pairing, files, etc.)
//	Default.SuperUserPhone  Phone number for admin notifications
//	Database.*, Redis.*, Metrics.*
//
// Usage:
//
//	go run cmd/whatsapp/main.go
//	make build-wa && ./bin/wa
package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/go-redis/redis/v8"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/sirupsen/logrus"
	"github.com/yeqown/go-qrcode/v2"
	"github.com/yeqown/go-qrcode/writer/standard"
	"go.mau.fi/whatsmeow"
	waE2E "go.mau.fi/whatsmeow/proto/waE2E"
	"go.mau.fi/whatsmeow/store/sqlstore"
	"go.mau.fi/whatsmeow/types"
	"go.mau.fi/whatsmeow/types/events"
	"google.golang.org/grpc"
	"google.golang.org/grpc/reflection"
	"google.golang.org/protobuf/proto"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"service-platform/internal/api/v1/controllers"
	"service-platform/internal/config"
	"service-platform/internal/core/model"
	whatsnyanmodel "service-platform/internal/core/model/whatsnyan_model"
	"service-platform/internal/database"
	"service-platform/pkg/fun"
	"service-platform/pkg/logger"
	pb "service-platform/proto"

	_ "github.com/lib/pq" // PostgreSQL driver
)

// server implements the WhatsAppServiceServer interface.
type server struct {
	pb.UnimplementedWhatsAppServiceServer                      // Embed for forward compatibility
	client                                *whatsmeow.Client    // WhatsApp client instance
	db                                    *gorm.DB             // Database connection
	rdb                                   *redis.Client        // Redis client
	pairingAttempts                       map[string]time.Time // Track pairing attempts by phone number
	lastBatteryLevel                      int32                // Track last battery level from WhatsApp events
}

// buildWAMessage converts a protobuf MessageContent into a whatsmeow message.
// It handles all supported content types: text, location, live location, poll, contact, and media.
func buildWAMessage(client *whatsmeow.Client, content *pb.MessageContent) (*waE2E.Message, error) {
	switch c := content.ContentType.(type) {
	case *pb.MessageContent_Text:
		return &waE2E.Message{Conversation: proto.String(c.Text)}, nil
	case *pb.MessageContent_Location:
		loc := c.Location
		return &waE2E.Message{
			LocationMessage: &waE2E.LocationMessage{
				DegreesLatitude:  proto.Float64(loc.Latitude),
				DegreesLongitude: proto.Float64(loc.Longitude),
				Name:             proto.String(loc.Name),
				Address:          proto.String(loc.Address),
			},
		}, nil
	case *pb.MessageContent_LiveLocation:
		loc := c.LiveLocation
		return &waE2E.Message{
			LiveLocationMessage: &waE2E.LiveLocationMessage{
				DegreesLatitude:                   proto.Float64(loc.Latitude),
				DegreesLongitude:                  proto.Float64(loc.Longitude),
				AccuracyInMeters:                  proto.Uint32(loc.AccuracyInMeters),
				SpeedInMps:                        proto.Float32(float32(loc.SpeedInMps)),
				DegreesClockwiseFromMagneticNorth: proto.Uint32(loc.DegreesClockwiseFromMagneticNorth),
				Caption:                           proto.String(loc.Caption),
				SequenceNumber:                    proto.Int64(loc.SequenceNumber),
				TimeOffset:                        proto.Uint32(loc.TimeOffset),
			},
		}, nil
	case *pb.MessageContent_Poll:
		poll := c.Poll
		options := make([]*waE2E.PollCreationMessage_Option, len(poll.Options))
		for i, opt := range poll.Options {
			options[i] = &waE2E.PollCreationMessage_Option{OptionName: proto.String(opt)}
		}
		return &waE2E.Message{
			PollCreationMessage: &waE2E.PollCreationMessage{
				Name:                   proto.String(poll.Name),
				Options:                options,
				SelectableOptionsCount: proto.Uint32(poll.SelectableOptionsCount),
			},
		}, nil
	case *pb.MessageContent_Contact:
		contact := c.Contact
		return &waE2E.Message{
			ContactMessage: &waE2E.ContactMessage{
				DisplayName: proto.String(contact.DisplayName),
				Vcard:       proto.String(contact.Vcard),
			},
		}, nil
	case *pb.MessageContent_Media:
		return buildWAMediaMessage(client, c.Media)
	default:
		return nil, fmt.Errorf("unsupported message type")
	}
}

// buildWAMediaMessage uploads and constructs a whatsmeow media message.
func buildWAMediaMessage(client *whatsmeow.Client, media *pb.MediaContent) (*waE2E.Message, error) {
	if len(media.Data) == 0 {
		return nil, fmt.Errorf("media data is empty")
	}
	var appMedia whatsmeow.MediaType
	switch media.MediaType {
	case "image":
		appMedia = whatsmeow.MediaImage
	case "video":
		appMedia = whatsmeow.MediaVideo
	case "audio":
		appMedia = whatsmeow.MediaAudio
	default:
		appMedia = whatsmeow.MediaDocument
	}
	uploaded, err := client.Upload(context.Background(), media.Data, appMedia)
	if err != nil {
		return nil, fmt.Errorf("failed to upload media: %v", err)
	}
	return buildUploadedMediaMessage(media, uploaded), nil
}

// buildUploadedMediaMessage creates the correct message type for an already-uploaded media item.
func buildUploadedMediaMessage(media *pb.MediaContent, uploaded whatsmeow.UploadResponse) *waE2E.Message {
	fileLen := proto.Uint64(uint64(len(media.Data)))
	switch media.MediaType {
	case "image":
		return &waE2E.Message{
			ImageMessage: &waE2E.ImageMessage{
				URL: proto.String(uploaded.URL), DirectPath: proto.String(uploaded.DirectPath),
				MediaKey: uploaded.MediaKey, Mimetype: proto.String(media.Mimetype),
				FileEncSHA256: uploaded.FileEncSHA256, FileSHA256: uploaded.FileSHA256,
				FileLength: fileLen, Caption: proto.String(media.Caption), ViewOnce: proto.Bool(media.ViewOnce),
			},
		}
	case "video":
		return &waE2E.Message{
			VideoMessage: &waE2E.VideoMessage{
				URL: proto.String(uploaded.URL), DirectPath: proto.String(uploaded.DirectPath),
				MediaKey: uploaded.MediaKey, Mimetype: proto.String(media.Mimetype),
				FileEncSHA256: uploaded.FileEncSHA256, FileSHA256: uploaded.FileSHA256,
				FileLength: fileLen, Caption: proto.String(media.Caption), ViewOnce: proto.Bool(media.ViewOnce),
			},
		}
	case "audio":
		return &waE2E.Message{
			AudioMessage: &waE2E.AudioMessage{
				URL: proto.String(uploaded.URL), DirectPath: proto.String(uploaded.DirectPath),
				MediaKey: uploaded.MediaKey, Mimetype: proto.String(media.Mimetype),
				FileEncSHA256: uploaded.FileEncSHA256, FileSHA256: uploaded.FileSHA256,
				FileLength: fileLen, ViewOnce: proto.Bool(media.ViewOnce),
			},
		}
	default:
		return &waE2E.Message{
			DocumentMessage: &waE2E.DocumentMessage{
				URL: proto.String(uploaded.URL), DirectPath: proto.String(uploaded.DirectPath),
				MediaKey: uploaded.MediaKey, Mimetype: proto.String(media.Mimetype),
				FileEncSHA256: uploaded.FileEncSHA256, FileSHA256: uploaded.FileSHA256,
				FileLength: fileLen, FileName: proto.String(media.Filename), Caption: proto.String(media.Caption),
			},
		}
	}
}

// SendMessage sends a WhatsApp message to a specified recipient.
// It supports both text and media messages.
func (s *server) SendMessage(ctx context.Context, req *pb.SendMessageRequest) (*pb.SendMessageResponse, error) {
	if s.client == nil || !s.client.IsConnected() {
		return &pb.SendMessageResponse{Success: false, Message: "WhatsApp client not connected"}, nil
	}

	jid, err := types.ParseJID(req.To)
	if err != nil {
		return &pb.SendMessageResponse{Success: false, Message: "Invalid recipient JID"}, nil
	}

	msg, err := buildWAMessage(s.client, req.Content)
	if err != nil {
		return &pb.SendMessageResponse{Success: false, Message: err.Error()}, nil
	}

	resp, err := s.client.SendMessage(ctx, jid, msg)
	if err != nil {
		return &pb.SendMessageResponse{Success: false, Message: err.Error()}, nil
	}

	return &pb.SendMessageResponse{
		Success:   true,
		Message:   "Message sent successfully",
		Id:        resp.ID,
		Timestamp: fmt.Sprintf("%d", resp.Timestamp.Unix()),
	}, nil
}

// buildTextStatusMessage constructs an ExtendedTextMessage suitable for a WhatsApp status post.
func buildTextStatusMessage(text, bgColorHex string, fontIdx int32) *waE2E.Message {
	var bgColor *uint32
	if bgColorHex != "" {
		hexColor := strings.TrimPrefix(bgColorHex, "#")
		if val, err := strconv.ParseUint(hexColor, 16, 32); err == nil {
			if len(hexColor) == 6 {
				val |= 0xFF000000
			}
			v := uint32(val)
			bgColor = &v
		}
	}
	if bgColor == nil {
		v := uint32(0xFF541560)
		bgColor = &v
	}
	textColor := uint32(0xFFFFFFFF)
	font := waE2E.ExtendedTextMessage_FontType(fontIdx)
	if font < 0 || font > 5 {
		font = waE2E.ExtendedTextMessage_FontType(0)
	}
	return &waE2E.Message{
		ExtendedTextMessage: &waE2E.ExtendedTextMessage{
			Text:           proto.String(text),
			BackgroundArgb: bgColor,
			TextArgb:       &textColor,
			Font:           &font,
			PreviewType:    waE2E.ExtendedTextMessage_NONE.Enum(),
		},
	}
}

// buildMediaStatusMessage uploads the given media and constructs the appropriate status message.
func buildMediaStatusMessage(client *whatsmeow.Client, media *pb.MediaContent) (*waE2E.Message, error) {
	var appMedia whatsmeow.MediaType
	switch media.MediaType {
	case "image":
		appMedia = whatsmeow.MediaImage
	case "video":
		appMedia = whatsmeow.MediaVideo
	case "audio":
		appMedia = whatsmeow.MediaAudio
	default:
		return nil, fmt.Errorf("unsupported media type for status")
	}
	uploaded, err := client.Upload(context.Background(), media.Data, appMedia)
	if err != nil {
		return nil, fmt.Errorf("failed to upload media: %v", err)
	}
	fileLen := proto.Uint64(uint64(len(media.Data)))
	switch media.MediaType {
	case "image":
		return &waE2E.Message{
			ImageMessage: &waE2E.ImageMessage{
				Caption: proto.String(media.Caption), Mimetype: proto.String(media.Mimetype),
				URL: proto.String(uploaded.URL), DirectPath: proto.String(uploaded.DirectPath),
				MediaKey: uploaded.MediaKey, FileEncSHA256: uploaded.FileEncSHA256,
				FileSHA256: uploaded.FileSHA256, FileLength: fileLen,
			},
		}, nil
	case "video":
		return &waE2E.Message{
			VideoMessage: &waE2E.VideoMessage{
				Caption: proto.String(media.Caption), Mimetype: proto.String(media.Mimetype),
				URL: proto.String(uploaded.URL), DirectPath: proto.String(uploaded.DirectPath),
				MediaKey: uploaded.MediaKey, FileEncSHA256: uploaded.FileEncSHA256,
				FileSHA256: uploaded.FileSHA256, FileLength: fileLen,
			},
		}, nil
	default: // audio
		return &waE2E.Message{
			AudioMessage: &waE2E.AudioMessage{
				Mimetype: proto.String(media.Mimetype),
				URL:      proto.String(uploaded.URL), DirectPath: proto.String(uploaded.DirectPath),
				MediaKey: uploaded.MediaKey, FileEncSHA256: uploaded.FileEncSHA256,
				FileSHA256: uploaded.FileSHA256, FileLength: fileLen, PTT: proto.Bool(true),
			},
		}, nil
	}
}

// CreateStatus posts a status update (story).
func (s *server) CreateStatus(ctx context.Context, req *pb.CreateStatusRequest) (*pb.CreateStatusResponse, error) {
	if s.client == nil || !s.client.IsConnected() {
		return &pb.CreateStatusResponse{Success: false, Message: "WhatsApp client not connected"}, nil
	}

	targetJID := types.StatusBroadcastJID

	var msg *waE2E.Message
	var statusType string

	switch content := req.Content.ContentType.(type) {
	case *pb.MessageContent_Text:
		msg = buildTextStatusMessage(content.Text, req.BackgroundColor, req.Font)
		statusType = "Text status"
	case *pb.MessageContent_Media:
		var err error
		msg, err = buildMediaStatusMessage(s.client, content.Media)
		if err != nil {
			return &pb.CreateStatusResponse{Success: false, Message: err.Error()}, nil
		}
		statusType = "Media status"
	default:
		return &pb.CreateStatusResponse{Success: false, Message: "Unsupported content type for status"}, nil
	}

	resp, err := s.client.SendMessage(ctx, targetJID, msg)
	if err != nil {
		return &pb.CreateStatusResponse{Success: false, Message: "Failed to send status: " + err.Error()}, nil
	}

	return &pb.CreateStatusResponse{
		Success:  true,
		Message:  fmt.Sprintf("%s created successfully", statusType),
		StatusId: resp.ID,
	}, nil
}

// HasContacts checks if the user has any contacts.
func (s *server) HasContacts(ctx context.Context, _ *pb.HasContactsRequest) (*pb.HasContactsResponse, error) {
	if s.client == nil || s.client.Store == nil {
		return &pb.HasContactsResponse{Success: false, Message: "WhatsApp client not initialized"}, nil
	}

	contacts, err := s.client.Store.Contacts.GetAllContacts(ctx)
	if err != nil {
		return &pb.HasContactsResponse{Success: false, Message: "Failed to get contacts: " + err.Error()}, nil
	}

	count := len(contacts)
	return &pb.HasContactsResponse{
		Success:      true,
		Message:      fmt.Sprintf("Found %d contacts", count),
		HasContacts:  count > 0,
		ContactCount: int32(count),
	}, nil
}

// GetMessages retrieves messages from the WhatsApp history.
// Currently, this is a placeholder and returns an empty list.
func (*server) GetMessages(_ context.Context, _ *pb.GetMessagesRequest) (*pb.GetMessagesResponse, error) {
	// For now, return empty - would need to implement message history retrieval
	return &pb.GetMessagesResponse{Messages: []*pb.Message{}}, nil
}

// initializeClient sets up the WhatsApp client, including database connection and logger.
// It ensures that the client is only initialized once.
func (s *server) initializeClient() error {
	if s.client != nil {
		return nil
	}

	config.ServicePlatform.MustInit("service-platform") // Load config with name "service-platform.%s.yaml"
	cfg := config.ServicePlatform.Get()

	// Build PostgreSQL DSN
	dsn := fmt.Sprintf("host=%s port=%d user=%s password=%s dbname=%s sslmode=%s",
		cfg.Database.Host,
		cfg.Database.Port,
		cfg.Database.Username,
		cfg.Database.Password,
		cfg.Database.Name,
		cfg.Database.SSLMode,
	)

	// Initialize App DB
	db, err := database.InitAndCheckDB(
		cfg.Database.Type,
		cfg.Database.Username,
		cfg.Database.Password,
		cfg.Database.Host,
		cfg.Database.Port,
		cfg.Database.Name,
		cfg.Database.SSLMode,
	)
	if err != nil {
		return fmt.Errorf("failed to init db: %v", err)
	}
	s.db = db

	// Initialize redis client
	redisHost := cfg.Redis.Host
	if redisHost == "" {
		redisHost = "localhost"
	}

	if err := fun.EnsureRedisRunning(redisHost, cfg.Redis.Port); err != nil {
		return fmt.Errorf("failed to ensure Redis is running: %v", err)
	}

	s.rdb = redis.NewClient(&redis.Options{
		Addr:     fmt.Sprintf("%s:%d", redisHost, cfg.Redis.Port),
		Password: cfg.Redis.Password,
		DB:       cfg.Redis.Db,
	})
	fmt.Println("💾 Redis initialized @", fmt.Sprintf("%s:%d", redisHost, cfg.Redis.Port))

	// Create a logger that writes to stdout
	dbLevel := logger.ParseWhatsmeowLogLevel(config.ServicePlatform.Get().Whatsnyan.DBLogLevel)
	clientLevel := logger.ParseWhatsmeowLogLevel(config.ServicePlatform.Get().Whatsnyan.ClientLogLevel)
	logDir, err := fun.FindValidDirectory([]string{
		"log",
		"../log",
		"../../log",
		"../../../log",
	})
	if err != nil {
		return fmt.Errorf("failed to find log directory: %v", err)
	}
	dbLogFile := filepath.Join(logDir, config.ServicePlatform.Get().Whatsnyan.DBLog)
	clientLogFile := filepath.Join(logDir, config.ServicePlatform.Get().Whatsnyan.ClientLog)

	dbLogger := logger.NewWhatsmeowLogger("Database", dbLogFile, dbLevel)
	clientLogger := logger.NewWhatsmeowLogger("Client", clientLogFile, clientLevel)

	container, err := sqlstore.New(context.Background(), "postgres", dsn, dbLogger)
	if err != nil {
		return err
	}

	deviceStore, err := container.GetFirstDevice(context.Background())
	if err != nil {
		return err
	}

	s.client = whatsmeow.NewClient(deviceStore, clientLogger)
	s.client.AddEventHandler(s.eventHandler)

	return nil
}

// connectPhonePairing handles the phone-pairing flow given an open qrChan.
// It returns immediately after generating the pairing code.
func (s *server) connectPhonePairing(qrChan <-chan whatsmeow.QRChannelItem, pairingPhone string) (*pb.ConnectResponse, error) {
	logrus.Info("Waiting for connection to establish before phone pairing...")
	firstEvt := <-qrChan
	if firstEvt.Event != "code" {
		if firstEvt.Event == "success" {
			return &pb.ConnectResponse{Success: true, Message: fmt.Sprintf("Connected successfully as %s", firstEvt.Code)}, nil
		}
		return &pb.ConnectResponse{Success: false, Message: fmt.Sprintf("Unexpected event: %s", firstEvt.Event)}, nil
	}

	const cooldownPeriod = 5 * time.Minute
	if lastAttempt, exists := s.pairingAttempts[pairingPhone]; exists {
		if remaining := cooldownPeriod - time.Since(lastAttempt); remaining > 0 {
			return &pb.ConnectResponse{
				Success: false,
				Message: fmt.Sprintf("Please wait %v before attempting to pair again. WhatsApp limits pairing attempts.", remaining.Round(time.Second)),
			}, nil
		}
	}

	logrus.Infof("Requesting phone pairing for: %s", pairingPhone)
	pairingCode, err := s.client.PairPhone(context.Background(), pairingPhone, true, whatsmeow.PairClientChrome, "Chrome (Linux)")
	if err != nil {
		errorMsg := fmt.Sprintf("Phone pairing failed: %v", err)
		if strings.Contains(err.Error(), "rate-overlimit") || strings.Contains(err.Error(), "429") {
			errorMsg = "Rate limit exceeded. Please wait 5-10 minutes before trying to pair again. WhatsApp limits pairing attempts to prevent abuse."
			logrus.Warn("Rate limit hit during phone pairing, suggesting cooldown period")
		}
		return &pb.ConnectResponse{Success: false, Message: errorMsg}, nil
	}

	logrus.Infof("✅ Pairing code generated: %s", pairingCode)
	logrus.Info("Enter this code in WhatsApp: Settings > Linked Devices > Link a Device > Link with Phone Number")
	s.pairingAttempts[pairingPhone] = time.Now()

	return &pb.ConnectResponse{
		Success:     true,
		Message:     "Pairing code generated. Enter this code in WhatsApp on your phone.",
		PairingCode: pairingCode,
	}, nil
}

// generateAndSaveQRCode saves a QR code image to disk and returns the response.
func generateAndSaveQRCode(qrCode string) (*pb.ConnectResponse, error) {
	now := time.Now()
	dateStr := now.Format(config.DateYYYYMMDD)
	dirQR, err := fun.FindValidDirectory([]string{
		"web/file/wa_qr", "../web/file/wa_qr", "../../web/file/wa_qr", "../../../web/file/wa_qr",
	})
	if err != nil {
		return &pb.ConnectResponse{Success: false, Message: fmt.Sprintf("Failed to find QR directory: %v", err)}, nil
	}
	dirPath := filepath.Join(dirQR, dateStr)
	if err := os.MkdirAll(dirPath, 0755); err != nil {
		return &pb.ConnectResponse{Success: false, Message: fmt.Sprintf("Failed to create directory: %v", err)}, nil
	}
	filePath := filepath.Join(dirPath, fmt.Sprintf("qr_%d.png", now.UnixNano()))
	qrc, err := qrcode.New(qrCode)
	if err != nil {
		return &pb.ConnectResponse{Success: false, Message: fmt.Sprintf("Failed to create QR object: %v", err)}, nil
	}
	webDir, err := fun.FindValidDirectory([]string{"web", "../web", "../../web", "../../../web"})
	if err != nil {
		return &pb.ConnectResponse{Success: false, Message: fmt.Sprintf("Failed to find web directory: %v", err)}, nil
	}
	logoPath := config.ServicePlatform.Get().App.LogoJPG
	if logoPath == "" {
		return &pb.ConnectResponse{Success: false, Message: "Logo path is not configured"}, nil
	}
	logoFullPath := filepath.Join(webDir, logoPath)
	f, err := os.Open(logoFullPath)
	if err != nil {
		return &pb.ConnectResponse{Success: false, Message: fmt.Sprintf("Failed to open logo file: %v", err)}, nil
	}
	defer f.Close()
	options, err := fun.QRWithLogo(logoFullPath)
	if err != nil {
		return &pb.ConnectResponse{Success: false, Message: fmt.Sprintf("Failed to get QR options with logo: %v", err)}, nil
	}
	w, err := standard.New(filePath, options...)
	if err != nil {
		return &pb.ConnectResponse{Success: false, Message: fmt.Sprintf("Failed to create QR writer: %v", err)}, nil
	}
	if err = qrc.Save(w); err != nil {
		return &pb.ConnectResponse{Success: false, Message: fmt.Sprintf("Failed to save QR image: %v", err)}, nil
	}
	logrus.Infof("QR code saved to: %s", filePath)
	return &pb.ConnectResponse{Success: true, Message: "QR code generated", QrCode: qrCode}, nil
}

// waitForQRChannel processes events from the QR channel and returns a connect response.
func (s *server) waitForQRChannel(qrChan <-chan whatsmeow.QRChannelItem) (*pb.ConnectResponse, error) {
	for evt := range qrChan {
		switch evt.Event {
		case "success":
			return &pb.ConnectResponse{Success: true, Message: fmt.Sprintf("Whatsapp logged in successfully as %s", s.client.Store.ID.User)}, nil
		case "timeout":
			return &pb.ConnectResponse{Success: false, Message: "QR code scan timed out"}, nil
		case "code":
			return generateAndSaveQRCode(evt.Code)
		case "err-unexpected-state":
			return &pb.ConnectResponse{Success: false, Message: "Unexpected state during QR generation"}, nil
		case "err-scanned-without-multidevice":
			return &pb.ConnectResponse{Success: false, Message: "Scanned without multi-device support"}, nil
		case "error":
			return &pb.ConnectResponse{Success: false, Message: "Error generating QR code"}, nil
		}
	}
	return &pb.ConnectResponse{Success: false, Message: "QR channel closed unexpectedly"}, nil
}

// Connect handles the connection process to WhatsApp.
// It initializes the client if needed, checks for existing sessions,
// and generates a QR code if a new login is required.
func (s *server) Connect(_ context.Context, req *pb.ConnectRequest) (*pb.ConnectResponse, error) {
	if err := s.initializeClient(); err != nil {
		return &pb.ConnectResponse{Success: false, Message: err.Error()}, nil
	}

	if s.client.IsConnected() {
		if s.client.IsLoggedIn() {
			return &pb.ConnectResponse{Success: true, Message: fmt.Sprintf("Already connected as %v", s.client.Store.ID)}, nil
		}
		logrus.Info("Client connected but not logged in. Disconnecting to restart login flow.")
		s.client.Disconnect()
	}

	if !req.ForceQr && s.client.Store.ID != nil {
		if err := s.client.Connect(); err != nil {
			logrus.Warnf("Failed to connect with existing session: %v", err)
		} else {
			return &pb.ConnectResponse{Success: true, Message: "Connected with existing session"}, nil
		}
	}

	qrChan, err := s.client.GetQRChannel(context.Background())
	if err != nil {
		return &pb.ConnectResponse{Success: false, Message: fmt.Sprintf("Failed to get QR channel: %v", err)}, nil
	}
	if err = s.client.Connect(); err != nil {
		return &pb.ConnectResponse{Success: false, Message: fmt.Sprintf("Failed to connect: %v", err)}, nil
	}

	enablePhonePairing := config.ServicePlatform.Get().Whatsnyan.EnablePhonePairing
	pairingPhone := req.PhoneNumber
	if pairingPhone == "" && enablePhonePairing {
		pairingPhone = config.ServicePlatform.Get().Whatsnyan.PairingPhoneNumber
	}
	if enablePhonePairing && pairingPhone != "" {
		return s.connectPhonePairing(qrChan, pairingPhone)
	}

	return s.waitForQRChannel(qrChan)
}

// Disconnect closes the connection to WhatsApp.
func (s *server) Disconnect(_ context.Context, _ *pb.DisconnectRequest) (*pb.DisconnectResponse, error) {
	if s.client != nil && s.client.IsConnected() {
		s.client.Disconnect()
		return &pb.DisconnectResponse{Success: true, Message: "Disconnected successfully"}, nil
	}
	return &pb.DisconnectResponse{Success: false, Message: "Not connected"}, nil
}

// IsConnected checks if the WhatsApp client is currently connected.
func (s *server) IsConnected(_ context.Context, _ *pb.IsConnectedRequest) (*pb.IsConnectedResponse, error) {
	if s.client == nil {
		return &pb.IsConnectedResponse{Connected: false, Message: "WhatsApp client not initialized"}, nil
	}

	connected := s.client.IsConnected()
	loggedIn := s.client.IsLoggedIn()
	message := "Connected and logged in"
	if !connected {
		message = "Not connected"
	} else if !loggedIn {
		message = "Connected but not logged in"
	}

	return &pb.IsConnectedResponse{
		Connected: connected && loggedIn, // Only consider connected if both connected AND logged in
		Message:   message,
	}, nil
}

// Logout logs out the current user from WhatsApp and deletes the session.
func (s *server) Logout(ctx context.Context, _ *pb.WALogoutRequest) (*pb.WALogoutResponse, error) {
	if s.client != nil {
		err := s.client.Logout(ctx)
		if err != nil {
			return &pb.WALogoutResponse{Success: false, Message: fmt.Sprintf("Logout failed: %v", err)}, nil
		}
		return &pb.WALogoutResponse{Success: true, Message: "Logged out successfully"}, nil
	}
	return &pb.WALogoutResponse{Success: false, Message: "Client not initialized"}, nil
}

// findRecentQRFile looks in dirPath for a QR PNG file modified within maxAge.
// Returns the path of the most recently modified such file, or "".
func findRecentQRFile(dirPath string, maxAge time.Duration) string {
	files, err := os.ReadDir(dirPath)
	if err != nil {
		return ""
	}
	var latestFile string
	var latestTime time.Time
	now := time.Now()
	for _, file := range files {
		if !strings.HasSuffix(file.Name(), ".png") || !strings.HasPrefix(file.Name(), "qr_") {
			continue
		}
		fp := filepath.Join(dirPath, file.Name())
		fi, err := os.Stat(fp)
		if err != nil {
			continue
		}
		if now.Sub(fi.ModTime()) <= maxAge && fi.ModTime().After(latestTime) {
			latestTime = fi.ModTime()
			latestFile = fp
		}
	}
	return latestFile
}

// RefreshQR forces a logout and generates a new QR code for connection.
func (s *server) RefreshQR(ctx context.Context, req *pb.RefreshQRRequest) (*pb.ConnectResponse, error) {
	if s.client == nil {
		return &pb.ConnectResponse{Success: false, Message: "Client not initialized"}, nil
	}

	if !req.GetForceNew() {
		dirQR, err := fun.FindValidDirectory([]string{
			"web/file/wa_qr", "../web/file/wa_qr", "../../web/file/wa_qr", "../../../web/file/wa_qr",
		})
		if err == nil {
			dirPath := filepath.Join(dirQR, time.Now().Format(config.DateYYYYMMDD))
			if _, err := os.Stat(dirPath); err == nil {
				if latest := findRecentQRFile(dirPath, 5*time.Minute); latest != "" {
					logrus.Infof("Using existing QR PNG from: %s", latest)
					return &pb.ConnectResponse{Success: true, Message: "QR code refreshed (using existing)", QrCode: latest}, nil
				}
			}
		}
	}

	logrus.Info("Generating new QR code (force_new or no recent QR found)")
	if s.client.IsLoggedIn() || s.client.Store.ID != nil {
		logrus.Info("RefreshQR requested. Logging out existing session.")
		if err := s.client.Logout(ctx); err != nil {
			logrus.Errorf("Failed to logout during refresh: %v", err)
		}
	}
	if s.client.IsConnected() {
		s.client.Disconnect()
	}
	return s.Connect(ctx, &pb.ConnectRequest{})
}

// GetMe retrieves the current user's information.
func (s *server) GetMe(ctx context.Context, _ *pb.GetMeRequest) (*pb.GetMeResponse, error) {
	if s.client == nil || !s.client.IsConnected() {
		return &pb.GetMeResponse{Success: false, Message: "WhatsApp client not connected"}, nil
	}
	if s.client.Store.ID == nil {
		return &pb.GetMeResponse{Success: false, Message: "Not logged in"}, nil
	}

	// Get user JID and phone
	userJID := s.client.Store.ID.User
	phoneNumber := s.client.Store.ID.User // The User field is the phone number

	// Get push name (display name)
	pushName := ""
	if s.client.Store.PushName != "" {
		pushName = s.client.Store.PushName
	}

	// Get device info
	device := "Unknown"
	platform := "Unknown"
	if s.client.Store.ID != nil {
		if s.client.Store.ID.Device > 0 {
			device = fmt.Sprintf("Device %d", s.client.Store.ID.Device)
		}
		// Platform is in the Server field
		if s.client.Store.ID.Server != "" {
			platform = s.client.Store.ID.Server
		}
	}

	// Try to get profile picture
	profilePicURL := ""
	if s.client.Store.ID != nil {
		pic, err := s.client.GetProfilePictureInfo(ctx, types.NewJID(userJID, types.DefaultUserServer), nil)
		if err == nil && pic != nil {
			profilePicURL = pic.URL
		}
	}

	// Get battery percentage
	// Note: Battery info comes from WhatsApp server updates
	// Battery is tracked by listening to BatteryEvent in event handlers
	// For now, return 0 if not available (implement battery tracking in event handler)
	batteryLevel := int32(0)
	if s.lastBatteryLevel > 0 {
		batteryLevel = s.lastBatteryLevel
	}

	return &pb.GetMeResponse{
		Success:       true,
		Message:       "User info retrieved successfully",
		Jid:           userJID,
		PhoneNumber:   phoneNumber,
		Name:          pushName,
		ProfilePicUrl: profilePicURL,
		Device:        device,
		Platform:      platform,
		Battery:       batteryLevel,
	}, nil
}

// GetGroupInfo retrieves information about a specific WhatsApp group.
func (s *server) GetGroupInfo(ctx context.Context, req *pb.GetGroupInfoRequest) (*pb.GetGroupInfoResponse, error) {
	if s.client == nil || !s.client.IsConnected() {
		return &pb.GetGroupInfoResponse{Success: false, Message: "WhatsApp client not connected"}, nil
	}

	jid, err := types.ParseJID(req.GroupJid)
	if err != nil {
		return &pb.GetGroupInfoResponse{Success: false, Message: "Invalid JID"}, nil
	}

	info, err := s.client.GetGroupInfo(ctx, jid)
	if err != nil {
		return &pb.GetGroupInfoResponse{Success: false, Message: err.Error()}, nil
	}

	participants := make([]*pb.GroupParticipant, len(info.Participants))
	for i, p := range info.Participants {
		phoneNumber := p.JID.User
		if p.JID.Server == "lid" {
			var lidMap whatsnyanmodel.WhatsmeowLIDMap
			if err := s.db.Where("lid = ?", p.JID.User).First(&lidMap).Error; err == nil {
				phoneNumber = lidMap.PN
			}
		}

		// Try to get contact info from store
		var displayName string
		if contact, err := s.client.Store.Contacts.GetContact(ctx, p.JID); err == nil && contact.Found {
			displayName = contact.PushName
			if displayName == "" {
				displayName = contact.FullName
			}
		}

		// Try to get profile picture
		profilePicURL := ""
		if pic, err := s.client.GetProfilePictureInfo(ctx, p.JID, nil); err == nil && pic != nil {
			profilePicURL = pic.URL
		}

		participants[i] = &pb.GroupParticipant{
			Jid:               p.JID.String(),
			IsAdmin:           p.IsAdmin,
			IsSuperAdmin:      p.IsSuperAdmin,
			Lid:               p.LID.String(),
			DisplayName:       displayName,
			PhoneNumber:       phoneNumber,
			ProfilePictureUrl: profilePicURL,
		}
	}

	// Get group profile picture
	groupPhotoURL := ""
	if pic, err := s.client.GetProfilePictureInfo(ctx, jid, nil); err == nil && pic != nil {
		groupPhotoURL = pic.URL
	}

	return &pb.GetGroupInfoResponse{
		Success:           true,
		Message:           "Group info retrieved successfully",
		Name:              info.Name,
		Participants:      participants,
		Jid:               info.JID.String(),
		OwnerJid:          info.OwnerJID.String(),
		Topic:             info.Topic,
		TopicSetAt:        info.TopicSetAt.Unix(),
		TopicSetBy:        info.TopicSetBy.String(),
		LinkedParentJid:   info.LinkedParentJID.String(),
		IsDefaultSubGroup: info.IsDefaultSubGroup,
		IsParent:          info.IsParent,
		Description:       info.Topic,
		PhotoUrl:          groupPhotoURL,
		Settings: &pb.GroupSettings{
			Locked:                info.IsLocked,
			AnnouncementOnly:      info.IsAnnounce,
			NoFrequentlyForwarded: false, // Not sure if available
			Ephemeral:             info.IsEphemeral,
			EphemeralDuration:     int32(info.DisappearingTimer),
		},
	}, nil
}

// IsOnWhatsApp checks if a given phone number is registered on WhatsApp.
func (s *server) IsOnWhatsApp(ctx context.Context, req *pb.IsOnWhatsAppRequest) (*pb.IsOnWhatsAppResponse, error) {
	if s.client == nil || !s.client.IsConnected() {
		return &pb.IsOnWhatsAppResponse{Success: false, Message: "WhatsApp client not connected"}, nil
	}

	results, err := s.client.IsOnWhatsApp(ctx, req.PhoneNumbers)
	if err != nil {
		return &pb.IsOnWhatsAppResponse{Success: false, Message: err.Error()}, nil
	}

	pbResults := make([]*pb.OnWhatsAppResult, len(results))
	for i, res := range results {
		pbResults[i] = &pb.OnWhatsAppResult{
			Query:        res.Query,
			Jid:          res.JID.String(),
			IsRegistered: res.IsIn,
		}
	}

	return &pb.IsOnWhatsAppResponse{
		Success: true,
		Results: pbResults,
	}, nil
}

// GetJoinedGroups retrieves the list of groups the user is currently in.
func (s *server) GetJoinedGroups(ctx context.Context, _ *pb.GetJoinedGroupsRequest) (*pb.GetJoinedGroupsResponse, error) {
	if s.client == nil || !s.client.IsConnected() {
		return &pb.GetJoinedGroupsResponse{Success: false, Message: "WhatsApp client not connected"}, nil
	}

	groups, err := s.client.GetJoinedGroups(ctx)
	if err != nil {
		return &pb.GetJoinedGroupsResponse{Success: false, Message: err.Error()}, nil
	}

	pbGroups := make([]*pb.GroupInfo, len(groups))
	for i, g := range groups {
		participants := make([]*pb.GroupParticipant, len(g.Participants))
		for j, p := range g.Participants {
			// Try to get contact info from store
			var displayName string
			if contact, err := s.client.Store.Contacts.GetContact(context.Background(), p.JID); err == nil && contact.Found {
				displayName = contact.PushName
				if displayName == "" {
					displayName = contact.FullName
				}
			}

			phoneNumber := p.JID.User
			if p.JID.Server == "lid" {
				var lidMap whatsnyanmodel.WhatsmeowLIDMap
				if err := s.db.Where("lid = ?", p.JID.User).First(&lidMap).Error; err == nil {
					phoneNumber = lidMap.PN
				}
			}

			// Try to get profile picture
			profilePicURL := ""
			if pic, err := s.client.GetProfilePictureInfo(ctx, p.JID, nil); err == nil && pic != nil {
				profilePicURL = pic.URL
			}

			participants[j] = &pb.GroupParticipant{
				Jid:               p.JID.User,
				IsAdmin:           p.IsAdmin,
				IsSuperAdmin:      p.IsSuperAdmin,
				Lid:               p.LID.String(),
				DisplayName:       displayName,
				PhoneNumber:       phoneNumber,
				ProfilePictureUrl: profilePicURL,
			}
		}

		pbGroups[i] = &pb.GroupInfo{
			Jid:               g.JID.String(),
			OwnerJid:          g.OwnerJID.String(),
			Name:              g.Name,
			Topic:             g.Topic,
			TopicSetAt:        g.TopicSetAt.Unix(),
			TopicSetBy:        g.TopicSetBy.String(),
			LinkedParentJid:   g.LinkedParentJID.String(),
			IsDefaultSubGroup: g.IsDefaultSubGroup,
			IsParent:          g.IsParent,
			Participants:      participants,
		}
	}

	return &pb.GetJoinedGroupsResponse{
		Success: true,
		Groups:  pbGroups,
	}, nil
}

// buildGroupParticipant resolves display name, phone number, and profile pic for a group participant.
func (s *server) buildGroupParticipant(groupJID string, p types.GroupParticipant) whatsnyanmodel.WhatsAppGroupParticipant {
	var displayName string
	if contact, err := s.client.Store.Contacts.GetContact(context.Background(), p.JID); err == nil && contact.Found {
		displayName = contact.PushName
		if displayName == "" {
			displayName = contact.FullName
		}
	}
	phoneNumber := p.JID.User
	if p.JID.Server == "lid" {
		var lidMap whatsnyanmodel.WhatsmeowLIDMap
		if s.db.Where("lid = ?", p.JID.User).First(&lidMap).Error == nil {
			phoneNumber = lidMap.PN
		}
	}
	profilePicURL := ""
	if pic, err := s.client.GetProfilePictureInfo(context.Background(), p.JID, nil); err == nil && pic != nil {
		profilePicURL = pic.URL
	}
	return whatsnyanmodel.WhatsAppGroupParticipant{
		GroupJID:          groupJID,
		UserJID:           p.JID.String(),
		LID:               p.LID.String(),
		DisplayName:       displayName,
		IsAdmin:           p.IsAdmin,
		IsSuperAdmin:      p.IsSuperAdmin,
		PhoneNumber:       phoneNumber,
		ProfilePictureURL: profilePicURL,
	}
}

// syncGroupParticipants replaces all participants for a group inside a transaction.
func (s *server) syncGroupParticipants(tx *gorm.DB, group *types.GroupInfo) error {
	if err := tx.Where("group_jid = ?", group.JID.String()).Delete(&whatsnyanmodel.WhatsAppGroupParticipant{}).Error; err != nil {
		return err
	}
	if len(group.Participants) == 0 {
		return nil
	}
	participants := make([]whatsnyanmodel.WhatsAppGroupParticipant, len(group.Participants))
	for i, p := range group.Participants {
		participants[i] = s.buildGroupParticipant(group.JID.String(), p)
	}
	return tx.Create(&participants).Error
}

// SyncGroups fetches joined groups and saves them to the database.
func (s *server) SyncGroups() {
	if s.client == nil || !s.client.IsConnected() {
		return
	}

	groups, err := s.client.GetJoinedGroups(context.Background())
	if err != nil {
		logrus.Errorf("SyncGroups: %v", err)
		return
	}

	for _, group := range groups {
		groupModel := whatsnyanmodel.WhatsAppGroup{
			JID:               group.JID.String(),
			Name:              group.Name,
			OwnerJID:          group.OwnerJID.String(),
			Topic:             group.Topic,
			TopicSetAt:        group.TopicSetAt,
			TopicSetBy:        group.TopicSetBy.String(),
			LinkedParentJID:   group.LinkedParentJID.String(),
			IsDefaultSubGroup: group.IsDefaultSubGroup,
			IsParent:          group.IsParent,
		}

		if err := s.db.Clauses(clause.OnConflict{
			Columns:   []clause.Column{{Name: "jid"}},
			UpdateAll: true,
		}).Create(&groupModel).Error; err != nil {
			logrus.Errorf("Failed to upsert group %s: %v", group.Name, err)
			continue
		}

		if err := s.db.Transaction(func(tx *gorm.DB) error {
			return s.syncGroupParticipants(tx, group)
		}); err != nil {
			logrus.Errorf("Failed to update participants for group %s: %v", group.Name, err)
		}
	}
	logrus.Infof("Successfully synced %d WhatsApp groups to database", len(groups))
}

// GetProfilePicture retrieves a user's profile picture.
func (s *server) GetProfilePicture(ctx context.Context, req *pb.GetProfilePictureRequest) (*pb.GetProfilePictureResponse, error) {
	if s.client == nil || !s.client.IsConnected() {
		return &pb.GetProfilePictureResponse{
			Success: false,
			Message: "WhatsApp client not connected",
		}, nil
	}

	// Parse JID
	parsedJID, err := types.ParseJID(req.Jid)
	if err != nil {
		return &pb.GetProfilePictureResponse{
			Success: false,
			Message: fmt.Sprintf("Invalid JID format: %v", err),
		}, nil
	}

	// Get profile picture info
	pic, err := s.client.GetProfilePictureInfo(ctx, parsedJID, nil)
	if err != nil || pic == nil || pic.URL == "" {
		return &pb.GetProfilePictureResponse{
			Success: false,
			Message: "Profile picture not found or not available",
		}, nil
	}

	// Download the image
	resp, err := http.Get(pic.URL)
	if err != nil {
		return &pb.GetProfilePictureResponse{
			Success: false,
			Message: fmt.Sprintf("Failed to download profile picture: %v", err),
		}, nil
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return &pb.GetProfilePictureResponse{
			Success: false,
			Message: fmt.Sprintf("Failed to fetch profile picture from WhatsApp (status: %d)", resp.StatusCode),
		}, nil
	}

	// Read the image data
	imageData, err := io.ReadAll(resp.Body)
	if err != nil {
		return &pb.GetProfilePictureResponse{
			Success: false,
			Message: fmt.Sprintf("Failed to read image data: %v", err),
		}, nil
	}

	// Get content type
	contentType := resp.Header.Get("Content-Type")
	if contentType == "" {
		contentType = "image/jpeg" // Default fallback
	}

	return &pb.GetProfilePictureResponse{
		Success:     true,
		Message:     "Profile picture retrieved successfully",
		ImageData:   imageData,
		ContentType: contentType,
	}, nil
}

// handleCallOfferEvent processes an incoming call offer: resolves the caller's phone number,
// optionally rejects the call and sends a denial message if verification is required.
func (s *server) handleCallOfferEvent(v *events.CallOffer) {
	callerJID := v.CallCreator
	phoneNumber := callerJID.User

	if callerJID.Server == "lid" {
		var lidMap whatsnyanmodel.WhatsmeowLIDMap
		if s.db.Where("lid = ?", callerJID.User).First(&lidMap).Error == nil {
			phoneNumber = lidMap.PN
		}
	} else {
		sanitizedPhone, err := fun.SanitizeIndonesiaPhoneNumber(phoneNumber)
		if err == nil {
			phoneNumber = config.ServicePlatform.Get().Default.DialingCodeDefault + sanitizedPhone
		} else {
			logrus.Errorf("Failed to sanitize phone number %s: %v", phoneNumber, err)
		}
	}

	if !config.ServicePlatform.Get().Whatsnyan.NeedVerifyAccount {
		return
	}

	callReject := true
	var waUser model.WAUsers
	if s.db.Where("phone_number = ?", phoneNumber).First(&waUser).Error == nil {
		callReject = !waUser.AllowedToCall
	}
	logrus.Infof("Incoming call from %s", phoneNumber)
	if !callReject {
		return
	}

	if err := s.client.RejectCall(context.Background(), v.CallCreator, v.CallID); err != nil {
		logrus.Errorf("Failed to reject call: %v", err)
	}
	support := config.ServicePlatform.Get().Whatsnyan.WATechnicalSupport
	langMessages := map[string]string{
		fun.LangID: fmt.Sprintf("📞❌ Maaf, nomor *%s* tidak diizinkan untuk melakukan panggilan ke WhatsApp ini.\n\nApabila ada kendala, Anda bisa menghubungi layanan bantuan teknis kami di nomor berikut: +%s. Terima kasih. 🙏", phoneNumber, support),
		fun.LangEN: fmt.Sprintf("📞❌ Sorry, the number *%s* is not allowed to make calls to this WhatsApp.\n\nIf you have any issues, you can contact our technical support service at the following number: +%s. Thank you. 🙏", phoneNumber, support),
		fun.LangES: fmt.Sprintf("📞❌ Lo sentimos, el número *%s* no tiene permitido llamar a este WhatsApp.\n\nSi tienes algún inconveniente, puedes contactar a nuestro servicio de soporte técnico al siguiente número: +%s. Gracias. 🙏", phoneNumber, support),
		fun.LangFR: fmt.Sprintf("📞❌ Désolé, le numéro *%s* n'est pas autorisé à appeler ce WhatsApp.\n\nEn cas de problème, vous pouvez contacter notre support technique au numéro suivant : +%s. Merci. 🙏", phoneNumber, support),
		fun.LangDE: fmt.Sprintf("📞❌ Entschuldigung, die Nummer *%s* darf diesen WhatsApp-Account nicht anrufen.\n\nBei Problemen können Sie unseren technischen Support unter folgender Nummer kontaktieren: +%s. Vielen Dank. 🙏", phoneNumber, support),
		fun.LangPT: fmt.Sprintf("📞❌ Desculpe, o número *%s* não está autorizado a fazer chamadas para este WhatsApp.\n\nSe tiver algum problema, você pode contatar nosso suporte técnico pelo seguinte número: +%s. Obrigado. 🙏", phoneNumber, support),
		fun.LangAR: fmt.Sprintf("📞❌ عذرًا، الرقم *%s* غير مسموح له بإجراء مكالمات إلى هذا الواتساب.\n\nإذا واجهت أي مشكلة، يمكنك التواصل مع الدعم الفني عبر الرقم التالي: +%s. شكرًا لك. 🙏", phoneNumber, support),
		fun.LangJP: fmt.Sprintf("📞❌ 申し訳ありませんが、番号 *%s* はこのWhatsAppへの通話が許可されていません。\n\n問題がある場合は、次の番号から技術サポートにお問い合わせください: +%s。ありがとうございます。🙏", phoneNumber, support),
		fun.LangCN: fmt.Sprintf("📞❌ 抱歉，号码 *%s* 不允许拨打此 WhatsApp。\n\n如果您有任何问题，可以联系技术支持：+%s。谢谢。🙏", phoneNumber, support),
		fun.LangRU: fmt.Sprintf("📞❌ Извините, номер *%s* не может совершать звонки на этот WhatsApp.\n\nЕсли у вас возникли проблемы, вы можете связаться с нашей технической поддержкой по номеру: +%s. Спасибо. 🙏", phoneNumber, support),
	}
	lang := controllers.NewLanguageMsgTranslation(fun.DefaultLang)
	lang.Texts = langMessages
	jidStr := fmt.Sprintf("%s@%s", phoneNumber, types.DefaultUserServer)
	controllers.SendLangWhatsAppTextMsg(jidStr, "", nil, lang, lang.LanguageCode, s.client, s.rdb, s.db)
}

// extractReplyMediaText downloads a media attachment and returns a human-readable text representation.
func (s *server) extractReplyMediaText(v *events.Message, uploadDir, waReplyPublicURL string) string {
	switch {
	case v.Message.Conversation != nil:
		return *v.Message.Conversation
	case v.Message.ExtendedTextMessage != nil && v.Message.ExtendedTextMessage.Text != nil:
		return *v.Message.ExtendedTextMessage.Text
	case v.Message.ImageMessage != nil:
		return s.downloadReplyMedia(v.Message.ImageMessage, uploadDir, waReplyPublicURL, "img", "📷 ")
	case v.Message.VideoMessage != nil:
		return s.downloadReplyMedia(v.Message.VideoMessage, uploadDir, waReplyPublicURL, "vid", "🎥 ")
	case v.Message.AudioMessage != nil:
		return s.downloadReplyMediaSimple(v.Message.AudioMessage, uploadDir, waReplyPublicURL, "aud", "🎧 Audio message: ")
	case v.Message.DocumentMessage != nil:
		return s.downloadReplyMedia(v.Message.DocumentMessage, uploadDir, waReplyPublicURL, "doc", "📄 ")
	case v.Message.StickerMessage != nil:
		return s.downloadReplyMediaSimple(v.Message.StickerMessage, uploadDir, waReplyPublicURL, "stk", "🖼️ Sticker: ")
	default:
		return "(non-text or unknown reply)"
	}
}

// mediaDownloadable is the interface satisfied by all downloadable message types.
type mediaDownloadable interface {
	GetMimetype() string
	GetCaption() string
}

// downloadReplyMedia downloads a media file, saves it, and returns a prefixed text with caption.
func (s *server) downloadReplyMedia(msg whatsmeow.DownloadableMessage, uploadDir, baseURL, prefix, emoji string) string {
	data, err := s.client.Download(context.Background(), msg)
	if err != nil {
		logrus.Errorf("Failed to download media (%s): %v", prefix, err)
		return ""
	}
	var mimeType, caption string
	if dm, ok := msg.(mediaDownloadable); ok {
		mimeType = dm.GetMimetype()
		caption = dm.GetCaption()
	}
	ext := fun.GetFileExtension(mimeType)
	filename := fmt.Sprintf("%s_%d%s", prefix, time.Now().UnixNano(), ext)
	savePath := filepath.Join(uploadDir, filename)
	os.WriteFile(savePath, data, 0644) //nolint:errcheck
	publicURL := fmt.Sprintf("%s/%s", baseURL, filename)
	return fmt.Sprintf("%s%s %s", emoji, caption, publicURL)
}

// downloadReplyMediaSimple downloads a media file without a caption field.
func (s *server) downloadReplyMediaSimple(msg whatsmeow.DownloadableMessage, uploadDir, baseURL, prefix, label string) string {
	data, err := s.client.Download(context.Background(), msg)
	if err != nil {
		logrus.Errorf("Failed to download media (%s): %v", prefix, err)
		return ""
	}
	var mimeType string
	if dm, ok := msg.(interface{ GetMimetype() string }); ok {
		mimeType = dm.GetMimetype()
	}
	ext := fun.GetFileExtension(mimeType)
	filename := fmt.Sprintf("%s_%d%s", prefix, time.Now().UnixNano(), ext)
	savePath := filepath.Join(uploadDir, filename)
	os.WriteFile(savePath, data, 0644) //nolint:errcheck
	publicURL := fmt.Sprintf("%s/%s", baseURL, filename)
	return fmt.Sprintf("%s%s", label, publicURL)
}

// safeTimestampPtr returns a pointer to ts if non-zero, otherwise a pointer to now.
func safeTimestampPtr(ts time.Time) *time.Time {
	if !ts.IsZero() {
		return &ts
	}
	now := time.Now()
	return &now
}

// handleMessageReply processes a reply-to-message event and updates the DB.
func (s *server) handleMessageReply(v *events.Message, uploadDir string) {
	var ctxInfo *waE2E.ContextInfo
	switch {
	case v.Message.ExtendedTextMessage != nil:
		ctxInfo = v.Message.ExtendedTextMessage.GetContextInfo()
	case v.Message.ImageMessage != nil:
		ctxInfo = v.Message.ImageMessage.GetContextInfo()
	case v.Message.VideoMessage != nil:
		ctxInfo = v.Message.VideoMessage.GetContextInfo()
	case v.Message.DocumentMessage != nil:
		ctxInfo = v.Message.DocumentMessage.GetContextInfo()
	case v.Message.AudioMessage != nil:
		ctxInfo = v.Message.AudioMessage.GetContextInfo()
	case v.Message.StickerMessage != nil:
		ctxInfo = v.Message.StickerMessage.GetContextInfo()
	}
	if ctxInfo == nil || ctxInfo.QuotedMessage == nil || ctxInfo.StanzaID == nil || *ctxInfo.StanzaID == "" {
		return
	}
	waReplyPublicURL := config.ServicePlatform.Get().Whatsnyan.WAReplyPublicURL + "/" + time.Now().Format(config.DateYYYYMMDD)
	replyText := s.extractReplyMediaText(v, uploadDir, waReplyPublicURL)
	repliedAt := safeTimestampPtr(v.Info.Timestamp)
	if err := s.db.Model(&whatsnyanmodel.WhatsAppMsg{}).
		Where("whatsapp_chat_id = ?", *ctxInfo.StanzaID).
		Updates(map[string]interface{}{
			"whatsapp_replied_by": v.Info.Sender.String(),
			"whatsapp_replied_at": repliedAt,
			"whatsapp_reply_text": replyText,
		}).Error; err != nil {
		logrus.Printf("Failed to update reply info: %v", err)
	}
}

// handleMessageEdit processes message edit protocol messages and updates the quoted message in DB.
func (s *server) handleMessageEdit(v *events.Message) {
	pm := v.Message.GetProtocolMessage()
	if pm == nil || pm.GetType() != waE2E.ProtocolMessage_MESSAGE_EDIT {
		return
	}
	edited := pm.GetEditedMessage()
	var replyText, repliedToMsgID string
	if etm := edited.GetExtendedTextMessage(); etm != nil {
		replyText = etm.GetText()
		if ctx := etm.GetContextInfo(); ctx != nil && ctx.GetStanzaID() != "" {
			repliedToMsgID = ctx.GetStanzaID()
		} else {
			logrus.Println("❌ No ContextInfo or stanzaID in edited reply")
		}
	}
	if replyText == "" && edited.GetConversation() != "" {
		replyText = edited.GetConversation()
		logrus.Println("📝 Edited plain text (not a reply)")
	}
	if repliedToMsgID == "" || replyText == "" {
		return
	}
	repliedAt := safeTimestampPtr(v.Info.Timestamp)
	if err := s.db.Model(&whatsnyanmodel.WhatsAppMsg{}).
		Where("whatsapp_chat_id = ?", repliedToMsgID).
		Updates(map[string]interface{}{
			"whatsapp_reply_text": replyText,
			"whatsapp_replied_at": repliedAt,
			"whatsapp_replied_by": v.Info.Sender.String(),
		}).Error; err != nil {
		logrus.Errorf("Failed to update quoted message (reply edit): %v", err)
	}
}

// handleMessageEvent processes incoming WhatsApp messages: dispatches processing goroutine,
// then handles reply tracking, reaction tracking, and message edits.
func (s *server) handleMessageEvent(v *events.Message, uploadDir string) {
	go s.processIncomingWhatsappMessage(v)

	s.handleMessageReply(v, uploadDir)

	reactedAt := safeTimestampPtr(v.Info.Timestamp)
	if r := v.Message.GetReactionMessage(); r != nil {
		if err := s.db.Model(&whatsnyanmodel.WhatsAppMsg{}).
			Where("whatsapp_chat_id = ?", r.GetKey().GetID()).
			Updates(map[string]interface{}{
				"whatsapp_reaction_emoji": r.GetText(),
				"whatsapp_reacted_by":     v.Info.Sender.String(),
				"whatsapp_reacted_at":     reactedAt,
			}).Error; err != nil {
			logrus.Errorf("error while try to update reaction for wa msg: %v", err)
		}
	}

	s.handleMessageEdit(v)
}

// resolveUploadDir resolves the upload directory for the current day, creating it if needed.
func resolveUploadDir() (string, error) {
	cwd, err := os.Getwd()
	if err != nil {
		return "", fmt.Errorf("failed to get working directory: %v", err)
	}
	baseDir := filepath.Join(cwd, "web", "file")
	if _, err := os.Stat(baseDir); os.IsNotExist(err) {
		baseDir, err = fun.FindValidDirectory([]string{
			"web/file", "../web/file", "../../web/file", "../../../web/file",
		})
		if err != nil {
			return "", fmt.Errorf("failed to find base directory: %v", err)
		}
	}
	uploadDir := filepath.Join(baseDir, "wa_reply", time.Now().Format(config.DateYYYYMMDD))
	if err := os.MkdirAll(uploadDir, 0755); err != nil {
		return "", fmt.Errorf("failed to create upload directory: %v", err)
	}
	return uploadDir, nil
}

// eventHandler handles incoming events from the WhatsApp client.
func (s *server) eventHandler(evt interface{}) {
	uploadDir, err := resolveUploadDir()
	if err != nil {
		logrus.Error(err)
		return
	}

	switch v := evt.(type) {
	case *events.Connected:
		logrus.Infof("✅ WhatsApp %v connected", s.client.Store.ID.User)
		s.SyncGroups()
	case *events.Disconnected:
		jidStr := "unknown"
		if s.client.Store.ID != nil {
			jidStr = s.client.Store.ID.User
		}
		logrus.Infof("❌ WhatsApp %s disconnected", jidStr)
	case *events.LoggedOut:
		logrus.Warnf("Received LoggedOut 🏃🏻 event for %s", s.client.Store.ID.User)
	case *events.Receipt:
		for _, msgID := range v.MessageIDs {
			if string(v.Type) != "" {
				if err := s.db.Model(&whatsnyanmodel.WhatsAppMsg{}).
					Where("whatsapp_chat_id = ?", msgID).
					Update("whatsapp_msg_status", string(v.Type)).Error; err != nil {
					logrus.Errorf("Failed to update message status for %s: %v", msgID, err)
				}
			}
		}
	case *events.CallOffer:
		s.handleCallOfferEvent(v)
	case *events.Message:
		s.handleMessageEvent(v, uploadDir)
	}
}

// unwrapMessage unwraps nested message containers (Ephemeral, ViewOnce, etc.).
func unwrapMessage(msg *waE2E.Message) *waE2E.Message {
	if msg.GetEphemeralMessage() != nil {
		msg = msg.GetEphemeralMessage().GetMessage()
	}
	if msg.GetViewOnceMessage() != nil {
		msg = msg.GetViewOnceMessage().GetMessage()
	}
	if msg.GetViewOnceMessageV2() != nil {
		msg = msg.GetViewOnceMessageV2().GetMessage()
	}
	if msg.GetDocumentWithCaptionMessage() != nil {
		msg = msg.GetDocumentWithCaptionMessage().GetMessage()
	}
	if msg.GetGroupMentionedMessage() != nil {
		msg = msg.GetGroupMentionedMessage().GetMessage()
	}
	return msg
}

// extractMsgBodyAndType returns the text body and type string from a message.
func extractMsgBodyAndType(msg *waE2E.Message) (string, string) {
	switch {
	case msg.GetConversation() != "":
		return msg.GetConversation(), "text"
	case msg.GetExtendedTextMessage() != nil:
		return msg.GetExtendedTextMessage().GetText(), "text"
	case msg.GetImageMessage() != nil:
		return msg.GetImageMessage().GetCaption(), "image"
	case msg.GetVideoMessage() != nil:
		return msg.GetVideoMessage().GetCaption(), "video"
	case msg.GetAudioMessage() != nil:
		return "", "audio"
	case msg.GetDocumentMessage() != nil:
		return msg.GetDocumentMessage().GetCaption(), "document"
	case msg.GetStickerMessage() != nil:
		return "", "sticker"
	default:
		return "", "unknown"
	}
}

// resolveSenderPhoneNumber resolves a WhatsApp sender's phone number via the LID map.
func (s *server) resolveSenderPhoneNumber(v *events.Message) string {
	raw := v.Info.Sender.User
	var lidMap whatsnyanmodel.WhatsmeowLIDMap
	err := s.db.Where("pn = ?", raw).First(&lidMap).Error
	if err == nil {
		return lidMap.PN
	}
	if errors.Is(err, gorm.ErrRecordNotFound) {
		err2 := s.db.Where("lid = ?", raw).First(&lidMap).Error
		if err2 == nil {
			return lidMap.PN
		}
		if errors.Is(err2, gorm.ErrRecordNotFound) {
			logrus.Errorf("Failed to find any phone number using %s : %v", raw, err2)
		} else {
			logrus.Errorf("got error while trying to search phone number : %v", err2)
		}
	} else {
		logrus.Errorf("Failed to find phone number based on sender JID: %v using %s", err, raw)
	}
	if v.Info.Sender.Server != "lid" {
		logrus.Warnf("Phone number not found in LID map, using raw User ID: %s", raw)
		return raw
	}
	return ""
}

// resolveSenderName resolves the display name of a sender.
func (s *server) resolveSenderName(v *events.Message, senderPhoneNumber string) string {
	name := v.Info.PushName
	if v.Info.VerifiedName != nil && v.Info.VerifiedName.Details != nil {
		name = v.Info.VerifiedName.Details.GetVerifiedName()
	}
	if name != "" {
		return name
	}
	var waUser model.WAUsers
	if s.db.Where("phone_number = ?", senderPhoneNumber).First(&waUser).Error == nil {
		return waUser.FullName
	}
	var waContacts whatsnyanmodel.WhatsmeowContacts
	if s.db.Where("their_jid LIKE ?", senderPhoneNumber).First(&waContacts).Error == nil {
		if waContacts.BusinessName != nil && *waContacts.BusinessName != "" {
			return *waContacts.BusinessName
		}
		if waContacts.PushName != nil && *waContacts.PushName != "" {
			return *waContacts.PushName
		}
		if waContacts.FullName != nil && *waContacts.FullName != "" {
			return *waContacts.FullName
		}
	}
	return "N/A"
}

// showLangPromptIfNeeded sends the language selection prompt if the user hasn't seen it recently.
// Returns true if the function should return early (prompt shown or config empty).
func (s *server) showLangPromptIfNeeded(originalSenderJID string, v *events.Message) bool {
	langPromptKey := fmt.Sprintf("lang_prompted_%s", originalSenderJID)
	exists, err := s.rdb.Exists(context.Background(), langPromptKey).Result()
	if err != nil {
		logrus.Errorf("Failed to check language prompt key: %v", err)
	}
	if exists != 0 {
		return false
	}
	langPrompt := config.ServicePlatform.Get().Whatsnyan.LanguagePrompt
	if len(langPrompt) == 0 {
		return true
	}
	langMsg := map[string]string{fun.DefaultLang: langPrompt}
	lang := controllers.NewLanguageMsgTranslation(fun.DefaultLang)
	lang.Texts = langMsg
	controllers.SendLangWhatsAppTextMsg(originalSenderJID, "", v, lang, lang.LanguageCode, s.client, s.rdb, s.db)
	expiry := time.Duration(config.ServicePlatform.Get().Whatsnyan.LanguagePromptShownExpiry) * time.Second
	if err := s.rdb.Set(context.Background(), langPromptKey, "true", expiry).Err(); err != nil {
		logrus.Errorf("Failed to set language prompt key: %v", err)
	}
	return true
}

// buildGroupErrorLang resolves the lang error message for a group context.
// Returns false if processing should stop (group not allowed).
func (s *server) buildGroupErrorLang(v *events.Message, lang *controllers.LanguageTranslation, langErr map[string]string, allowedWAG []string) bool { //nolint:gocritic
	groupInfo, groupErr := s.client.GetGroupInfo(context.Background(), v.Info.Chat)
	if groupErr != nil {
		logrus.Errorf("Failed to get group info for %s: %v", v.Info.Chat.String(), groupErr)
		lang.Texts = map[string]string{
			fun.LangID: fmt.Sprintf("🗣 Maaf terjadi kesalahan saat mengambil info grup %s: %v", v.Info.Chat.String(), groupErr),
			fun.LangEN: fmt.Sprintf("🗣 Sorry, an error occurred while fetching group info %s: %v", v.Info.Chat.String(), groupErr),
			fun.LangES: fmt.Sprintf("🗣 Lo sentimos, ocurrió un error al obtener la información del grupo %s: %v", v.Info.Chat.String(), groupErr),
			fun.LangFR: fmt.Sprintf("🗣 Désolé, une erreur s'est produite lors de la récupération des informations du groupe %s : %v", v.Info.Chat.String(), groupErr),
			fun.LangDE: fmt.Sprintf("🗣 Entschuldigung, beim Abrufen der Gruppeninformationen %s ist ein Fehler aufgetreten: %v", v.Info.Chat.String(), groupErr),
			fun.LangPT: fmt.Sprintf("🗣 Desculpe, ocorreu um erro ao obter as informações do grupo %s: %v", v.Info.Chat.String(), groupErr),
			fun.LangRU: fmt.Sprintf("🗣 Извините, произошла ошибка при получении информации о группе %s: %v", v.Info.Chat.String(), groupErr),
			fun.LangJP: fmt.Sprintf("🗣 申し訳ありません、グループ情報 %s を取得中にエラーが発生しました: %v", v.Info.Chat.String(), groupErr),
			fun.LangCN: fmt.Sprintf("🗣 抱歉，获取群组信息 %s 时发生错误：%v", v.Info.Chat.String(), groupErr),
			fun.LangAR: fmt.Sprintf("🗣 عذرًا، حدث خطأ أثناء جلب معلومات المجموعة %s: %v", v.Info.Chat.String(), groupErr),
		}
		return true
	}
	if groupInfo == nil {
		logrus.Errorf("Group info is nil for %s", v.Info.Chat.String())
		for lc, text := range langErr {
			langErr[lc] = fmt.Sprintf("🗣 %s", text)
		}
		lang.Texts = langErr
		return true
	}
	groupName := strings.TrimSpace(groupInfo.Name)
	if !controllers.ContainsJID(allowedWAG, v.Info.Chat) {
		logrus.Infof("Group %s (%s) is not allowed to interact", groupName, v.Info.Chat.String())
		return false
	}
	prefix := "🗣 "
	if groupName != "" {
		prefix = fmt.Sprintf("[*%s*] 🗣 ", groupName)
	}
	for lc, text := range langErr {
		langErr[lc] = prefix + text
	}
	lang.Texts = langErr
	return true
}

// validateUserAndGetShouldProcess checks user authorization. Returns (user, shouldProcess).
func (s *server) validateUserAndGetShouldProcess(
	v *events.Message,
	senderPhoneNumber, originalSenderJID, stanzaID, msgType string,
	allowedWAG []string,
) (*model.WAUsers, bool) {
	if !config.ServicePlatform.Get().Whatsnyan.NeedVerifyAccount {
		var waUser model.WAUsers
		if s.db.Where("phone_number = ?", senderPhoneNumber).First(&waUser).Error == nil {
			return &waUser, true
		}
		return nil, true
	}

	userSanitizeResult, langErr := controllers.ValidateUserToUseBotWhatsapp(
		senderPhoneNumber, originalSenderJID, v.Info.IsGroup, msgType, s.client, s.rdb, s.db,
	)
	if userSanitizeResult != nil {
		return userSanitizeResult, true
	}

	logrus.Warnf("User %s not allowed to use bot. Sending error message if applicable.", senderPhoneNumber)
	if langErr == nil {
		return nil, false
	}

	lang := controllers.NewLanguageMsgTranslation(fun.DefaultLang)
	lang.Texts = langErr

	if v.Info.IsGroup && len(allowedWAG) > 0 {
		if !s.buildGroupErrorLang(v, &lang, langErr, allowedWAG) {
			return nil, false
		}
	}

	if v.Info.IsGroup {
		controllers.SendLangWhatsAppTextMsg(senderPhoneNumber+"@"+types.DefaultUserServer, "", nil, lang, lang.LanguageCode, s.client, s.rdb, s.db)
	} else {
		controllers.SendLangWhatsAppTextMsg(originalSenderJID, stanzaID, v, lang, lang.LanguageCode, s.client, s.rdb, s.db)
	}
	return nil, false
}

// handleDocumentValidation validates and filters the incoming document message.
// Returns false if processing should stop.
func (s *server) handleDocumentValidation(v *events.Message, originalSenderJID, userLang string, user *model.WAUsers) bool {
	doc := v.Message.DocumentMessage
	if doc == nil || doc.FileName == nil || doc.FileLength == nil {
		return true
	}
	fileRules := map[string]controllers.FilePermissionRule{
		"document": {
			MaxFileSizeBytes:  int64(config.ServicePlatform.Get().Whatsnyan.Files.Document.MaxSize) * 1024 * 1024,
			AllowedExtensions: config.ServicePlatform.Get().Whatsnyan.Files.Document.AllowedExtensions,
			AllowedMimeTypes:  config.ServicePlatform.Get().Whatsnyan.Files.Document.AllowedMimeTypes,
		},
	}
	mimeType := ""
	if doc.Mimetype != nil {
		mimeType = *doc.Mimetype
	}
	valid, errMsg := controllers.ValidateFileProperties(*doc.FileName, int64(*doc.FileLength), mimeType, fileRules["document"], userLang)
	if !valid {
		langMsg := controllers.NewLanguageMsgTranslation(userLang)
		langMsg.Texts[userLang] = errMsg
		controllers.SendLangWhatsAppTextMsg(originalSenderJID, "", nil, langMsg, userLang, s.client, s.rdb, s.db)
		return false
	}
	allowed, msg := controllers.SanitizeAndFilterDocument(v, user, userLang)
	if !allowed {
		controllers.SendLangWhatsAppTextMsg(originalSenderJID, "", nil, msg, userLang, s.client, s.rdb, s.db)
		return false
	}
	return true
}

// handleAuthorizedMessage processes a verified user's message (quota check, text/file dispatch).
// Returns true if further processing (keyword search) should happen.
func (s *server) handleAuthorizedMessage(v *events.Message, originalSenderJID, stanzaID, msgType, lowerMsgText, userLang string, user *model.WAUsers) bool {
	shouldProcessQuota, err := controllers.CheckAndNotifyQuotaLimit(
		user.ID, user.UseBot, originalSenderJID, user.MaxDailyQuota, s.client, s.rdb, s.db,
	)
	if err != nil {
		logrus.Errorf("Failed to check quota limit for %s: %v", originalSenderJID, err)
		return false
	}
	if !shouldProcessQuota {
		logrus.Warnf("Quota exceeded for %s", originalSenderJID)
		return false
	}

	if msgType != "text" {
		fileResult := controllers.CheckFilePermission(context.Background(), v, msgType, user, userLang, s.rdb)
		if !fileResult.Allowed {
			controllers.SendLangWhatsAppTextMsg(originalSenderJID, "", nil, fileResult.Message, userLang, s.client, s.rdb, s.db)
			return false
		}
		if msgType == "document" {
			if !s.handleDocumentValidation(v, originalSenderJID, userLang, user) {
				return false
			}
		}
		return false
	}

	result := controllers.CheckPromptPermission(context.Background(), v, lowerMsgText, user, userLang, s.rdb, s.db)
	if !result.Allowed {
		controllers.SendLangWhatsAppTextMsg(originalSenderJID, stanzaID, v, result.Message, userLang, s.client, s.rdb, s.db)
		return false
	}
	switch {
	case lowerMsgText == "ping":
		controllers.HandlePing(context.Background(), v, stanzaID, originalSenderJID, userLang, s.client, s.rdb, s.db)
		return false
	case strings.Contains(lowerMsgText, "get pprof"):
		controllers.HandlePprof(context.Background(), v, stanzaID, originalSenderJID, userLang, s.client, s.rdb, s.db)
		return false
	case strings.Contains(lowerMsgText, "get metrics"):
		controllers.HandleMetrics(context.Background(), v, stanzaID, originalSenderJID, userLang, s.client, s.rdb, s.db)
		return false
	}
	return true
}

// handleGroupHello handles group greeting messages (halo/hello/hola) for allowed WAGs.
// Returns true if the message was handled and processing should stop.
func (s *server) handleGroupHello(v *events.Message, senderPhoneNumber, senderName, stanzaID, lowerMsgText string, allowedWAG []string) bool {
	if len(allowedWAG) == 0 || !v.Info.IsGroup || !controllers.ContainsJID(allowedWAG, v.Info.Chat) {
		return false
	}
	jidStr := fmt.Sprintf("%s@%s", senderPhoneNumber, types.DefaultUserServer)
	switch strings.TrimSpace(lowerMsgText) {
	case "halo", "hello", "hola":
		controllers.HelloFromBot(v, senderPhoneNumber, senderName, stanzaID, jidStr, s.client, s.rdb, s.db)
		return true
	}
	return false
}

// processIncomingWhatsappMessage handles a single incoming WhatsApp message event.
// It skips messages older than the configured tolerance window, then dispatches
// text, media, reaction, reply, and edit events to the appropriate handlers.
func (s *server) processIncomingWhatsappMessage(v *events.Message) {
	toleranceHours := config.ServicePlatform.Get().Whatsnyan.MessageProcessedToleranceHours
	if toleranceHours <= 0 {
		toleranceHours = 24
	}
	if v.Info.Timestamp.Before(time.Now().Add(-time.Duration(toleranceHours) * time.Hour)) {
		logrus.Debugf("Skipping old message sent at %v (older than %d hours)", v.Info.Timestamp, toleranceHours)
		return
	}

	msg := unwrapMessage(v.Message)
	msgBody, msgType := extractMsgBodyAndType(msg)

	incomingMsg := whatsnyanmodel.WhatsAppIncomingMsg{
		WhatsappChatID:      v.Info.ID,
		WhatsappSenderJID:   v.Info.Sender.String(),
		WhatsappSenderName:  v.Info.PushName,
		WhatsappChatJID:     v.Info.Chat.String(),
		WhatsappMessageBody: msgBody,
		WhatsappMessageType: msgType,
		WhatsappIsGroup:     v.Info.IsGroup,
		WhatsappReceivedAt:  v.Info.Timestamp,
	}
	if err := s.db.Create(&incomingMsg).Error; err != nil {
		logrus.Errorf("Failed to save incoming message: %v", err)
	}

	if v.Info.MessageSource.IsFromMe {
		return
	}

	senderPhoneNumber := s.resolveSenderPhoneNumber(v)
	if senderPhoneNumber == "" {
		logrus.Warnf("Sender phone number not found for JID: %s. Skipping processing.", v.Info.Sender.String())
		return
	}

	if s.client == nil {
		logrus.Error("WhatsApp client is not initialized")
		return
	}

	originalSenderJID := controllers.NormalizeSenderJID(v.Info.Sender.String())
	stanzaID := v.Info.ID
	lowerMsgText := strings.ToLower(strings.TrimSpace(msgBody))
	senderName := s.resolveSenderName(v, senderPhoneNumber)
	_ = senderName // used in group hello handler below

	userLang, err := controllers.GetUserLang(originalSenderJID, s.rdb)
	if err != nil {
		logrus.Errorf("Failed to get user language: %v", err)
		userLang = fun.DefaultLang
	}
	controllers.HandleLanguageChange(originalSenderJID, lowerMsgText, s.client, s.rdb, s.db)

	if userLang == "" {
		if s.showLangPromptIfNeeded(originalSenderJID, v) {
			return
		}
	}

	allowedWAG := config.ServicePlatform.Get().Whatsnyan.WAGAllowedToInteract
	if s.handleGroupHello(v, senderPhoneNumber, senderName, stanzaID, lowerMsgText, allowedWAG) {
		return
	}

	userSanitizeResult, shouldProcess := s.validateUserAndGetShouldProcess(v, senderPhoneNumber, originalSenderJID, stanzaID, msgType, allowedWAG)

	if shouldProcess && userSanitizeResult != nil {
		if !s.handleAuthorizedMessage(v, originalSenderJID, stanzaID, msgType, lowerMsgText, userLang, userSanitizeResult) {
			return
		}
	}

	if shouldProcess {
		controllers.HandleKeywordSearch(context.Background(), v, stanzaID, lowerMsgText, originalSenderJID, userLang, userSanitizeResult, s.client, s.rdb, s.db)
	}
}

// sendPairingCodeNotification sends the pairing code to the superuser via WhatsApp.
func sendPairingCodeNotification(srv *server, pairingCode string) {
	suPhoneNumber := config.ServicePlatform.Get().Default.SuperUserPhone
	if suPhoneNumber == "" {
		return
	}
	func() {
		defer func() {
			if r := recover(); r != nil {
				logrus.Warnf("Failed to send pairing code notification: %v", r)
			}
		}()
		jidStr := fmt.Sprintf("%s@%s", suPhoneNumber, types.DefaultUserServer)
		langMsg := controllers.NewLanguageMsgTranslation(fun.DefaultLang)
		langMsg.Texts = map[string]string{
			fun.LangID: fmt.Sprintf("🔐 Kode Pairing WhatsApp: %s\n\nMasukkan kode ini di WhatsApp: Pengaturan > Perangkat Tertaut > Tautkan Perangkat > Tautkan dengan Nomor Telepon", pairingCode),
			fun.LangEN: fmt.Sprintf("🔐 WhatsApp Pairing Code: %s\n\nEnter this code in WhatsApp: Settings > Linked Devices > Link a Device > Link with Phone Number", pairingCode),
			fun.LangES: fmt.Sprintf("🔐 Código de Emparejamiento de WhatsApp: %s\n\nIngresa este código en WhatsApp: Configuración > Dispositivos Vinculados > Vincular Dispositivo > Vincular con Número de Teléfono", pairingCode),
			fun.LangFR: fmt.Sprintf("🔐 Code de Jumelage WhatsApp: %s\n\nEntrez ce code dans WhatsApp: Paramètres > Appareils Liés > Lier un Appareil > Lier avec Numéro de Téléphone", pairingCode),
			fun.LangDE: fmt.Sprintf("🔐 WhatsApp-Kopplungscode: %s\n\nGib diesen Code in WhatsApp ein: Einstellungen > Verknüpfte Geräte > Gerät Verknüpfen > Mit Telefonnummer Verknüpfen", pairingCode),
			fun.LangPT: fmt.Sprintf("🔐 Código de Emparelhamento WhatsApp: %s\n\nDigite este código no WhatsApp: Configurações > Dispositivos Vinculados > Vincular Dispositivo > Vincular com Número de Telefone", pairingCode),
			fun.LangAR: fmt.Sprintf("🔐 رمز الاقتران بواتساب: %s\n\nأدخل هذا الرمز في واتساب: الإعدادات > الأجهزة المرتبطة > ربط جهاز > الربط برقم الهاتف", pairingCode),
			fun.LangJP: fmt.Sprintf("🔐 WhatsAppペアリングコード: %s\n\nこのコードをWhatsAppに入力してください: 設定 > リンク済みデバイス > デバイスをリンク > 電話番号でリンク", pairingCode),
			fun.LangCN: fmt.Sprintf("🔐 WhatsApp配对代码: %s\n\n在WhatsApp中输入此代码: 设置 > 已关联设备 > 关联设备 > 使用电话号码关联", pairingCode),
			fun.LangRU: fmt.Sprintf("🔐 Код сопряжения WhatsApp: %s\n\nВведите этот код в WhatsApp: Настройки > Связанные устройства > Привязать устройство > Привязать по номеру телефона", pairingCode),
		}
		if srv.client != nil && srv.client.IsConnected() && srv.rdb != nil && srv.db != nil {
			controllers.SendLangWhatsAppTextMsg(jidStr, "", nil, langMsg, langMsg.LanguageCode, srv.client, srv.rdb, srv.db)
		}
	}()
}

// autoConnectOrPair attempts to auto-connect using an existing session or phone pairing.
func autoConnectOrPair(srv *server, cfg config.TypeServicePlatform) {
	if srv.client.Store.ID != nil {
		if err := srv.client.Connect(); err != nil {
			logrus.Errorf("Failed to auto-connect to WhatsApp: %v", err)
		} else {
			logrus.Infof("✅ Auto-connected to WhatsApp as %s", srv.client.Store.ID.User)
		}
		return
	}
	if cfg.Whatsnyan.EnablePhonePairing && cfg.Whatsnyan.PairingPhoneNumber != "" {
		logrus.Infof("📱 Phone pairing enabled. Initiating pairing for: %s", cfg.Whatsnyan.PairingPhoneNumber)
		go func() {
			time.Sleep(2 * time.Second)
			resp, err := srv.Connect(context.Background(), &pb.ConnectRequest{
				PhoneNumber: cfg.Whatsnyan.PairingPhoneNumber,
			})
			if err != nil {
				logrus.Errorf("Auto phone pairing failed: %v", err)
				return
			}
			if !resp.Success {
				logrus.Errorf("Phone pairing failed: %s", resp.Message)
				return
			}
			logrus.Infof("✅ %s", resp.Message)
			if resp.PairingCode != "" {
				logrus.Infof("🔐 PAIRING CODE: %s", resp.PairingCode)
				logrus.Info("📲 Enter this code in WhatsApp: Settings > Linked Devices > Link a Device > Link with Phone Number")
				sendPairingCodeNotification(srv, resp.PairingCode)
			}
		}()
		return
	}
	logrus.Info("📱 No existing session found. Use Connect RPC with QR code or enable phone pairing in config.")
}

// startMetricsServer starts the Prometheus metrics HTTP server.
func startMetricsServer() {
	go func() {
		http.Handle("/whatsapp-metrics", promhttp.Handler())
		metricsPort := config.ServicePlatform.Get().Metrics.WhatsAppPort
		logrus.Printf("📊 Metrics server listening on :%d", metricsPort)
		logrus.Fatal(http.ListenAndServe(fmt.Sprintf(":%d", metricsPort), nil))
	}()
}

// sendStartupNotification notifies the superuser when the WhatsApp gRPC server is ready.
func sendStartupNotification(srv *server) {
	go func() {
		defer func() {
			if r := recover(); r != nil {
				logrus.Warnf("Startup notification failed: %v", r)
			}
		}()
		for i := 0; i < 30; i++ {
			if srv.client != nil && srv.client.IsConnected() {
				break
			}
			time.Sleep(1 * time.Second)
		}
		suPhoneNumber := config.ServicePlatform.Get().Default.SuperUserPhone
		if suPhoneNumber == "" || srv.client == nil || !srv.client.IsConnected() || srv.rdb == nil || srv.db == nil {
			if suPhoneNumber != "" {
				logrus.Warn("Could not send startup notification: WhatsApp client not connected")
			}
			return
		}
		jidStr := fmt.Sprintf("%s@%s", suPhoneNumber, types.DefaultUserServer)
		langMsg := controllers.NewLanguageMsgTranslation(fun.DefaultLang)
		langMsg.Texts = map[string]string{
			fun.LangID: "✅ WhatsApp gRPC server berhasil dijalankan dan siap menerima pesan.",
			fun.LangEN: "✅ WhatsApp gRPC server has been successfully started and is ready to receive messages.",
			fun.LangES: "✅ El servidor gRPC de WhatsApp se ha iniciado correctamente y está listo para recibir mensajes.",
			fun.LangFR: "✅ Le serveur gRPC WhatsApp a démarré avec succès et est prêt à recevoir des messages.",
			fun.LangDE: "✅ Der WhatsApp gRPC-Server wurde erfolgreich gestartet und ist bereit, Nachrichten zu empfangen.",
			fun.LangPT: "✅ O servidor gRPC do WhatsApp foi iniciado com sucesso e está pronto para receber mensagens.",
			fun.LangAR: "✅ تم تشغيل خادم WhatsApp gRPC بنجاح وهو جاهز لاستقبال الرسائل.",
			fun.LangJP: "✅ WhatsApp gRPCサーバーが正常に起動し、メッセージの受信準備が整いました。",
			fun.LangCN: "✅ WhatsApp gRPC 服务器已成功启动，准备接收消息。",
			fun.LangRU: "✅ Сервер WhatsApp gRPC успешно запущен и готов принимать сообщения.",
		}
		controllers.SendLangWhatsAppTextMsg(jidStr, "", nil, langMsg, langMsg.LanguageCode, srv.client, srv.rdb, srv.db)
	}()
}

// handleGracefulShutdown waits for SIGTERM/interrupt, notifies the superuser, and stops the server.
func handleGracefulShutdown(s *grpc.Server, srv *server) {
	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt, syscall.SIGTERM)
	go func() {
		<-c
		logrus.Info("🔴 Shutting down WhatsApp gRPC server...")
		suPhoneNumber := config.ServicePlatform.Get().Default.SuperUserPhone
		if suPhoneNumber != "" && srv.client != nil && srv.client.IsConnected() && srv.rdb != nil && srv.db != nil {
			func() {
				defer func() {
					if r := recover(); r != nil {
						logrus.Warnf("Failed to send shutdown notification (client may have disconnected): %v", r)
					}
				}()
				jidStr := fmt.Sprintf("%s@%s", suPhoneNumber, types.DefaultUserServer)
				langMsg := controllers.NewLanguageMsgTranslation(fun.DefaultLang)
				langMsg.Texts = map[string]string{
					fun.LangID: "🔴 WhatsApp gRPC server sedang dimatikan...",
					fun.LangEN: "🔴 WhatsApp gRPC server is shutting down...",
					fun.LangES: "🔴 El servidor gRPC de WhatsApp se está apagando...",
					fun.LangFR: "🔴 Le serveur gRPC WhatsApp est en train de s'arrêter...",
					fun.LangDE: "🔴 Der WhatsApp gRPC-Server wird heruntergefahren...",
					fun.LangPT: "🔴 O servidor gRPC do WhatsApp está sendo desligado...",
					fun.LangAR: "🔴 يتم إيقاف تشغيل خادم WhatsApp gRPC...",
					fun.LangJP: "🔴 WhatsApp gRPCサーバーをシャットダウンしています...",
					fun.LangCN: "🔴 WhatsApp gRPC 服务器正在关闭...",
					fun.LangRU: "🔴 Сервер WhatsApp gRPC выключается...",
				}
				controllers.SendLangWhatsAppTextMsg(jidStr, "", nil, langMsg, langMsg.LanguageCode, srv.client, srv.rdb, srv.db)
				time.Sleep(1 * time.Second)
			}()
		}
		s.GracefulStop()
	}()
}

// main is the entry point for the WhatsApp microservice.
func main() {
	config.ServicePlatform.MustInit("service-platform") // Load config with name "service-platform.%s.yaml"
	go config.ServicePlatform.Watch()

	cfg := config.ServicePlatform.Get()

	logger.InitLogrus()

	port := fmt.Sprintf("%d", cfg.Whatsnyan.GRPCPort)
	lis, err := net.Listen("tcp", ":"+port)
	if err != nil {
		logrus.Fatalf("Failed to listen: %v", err)
	}

	s := grpc.NewServer()
	srv := &server{
		pairingAttempts: make(map[string]time.Time),
	}

	if err := srv.initializeClient(); err != nil {
		log.Fatal(err)
	}
	autoConnectOrPair(srv, cfg)

	pb.RegisterWhatsAppServiceServer(s, srv)

	reflection.Register(s)

	startMetricsServer()

	fmt.Printf("📞 Whatsapp gRPC server listening on port %s\n", port)
	sendStartupNotification(srv)

	handleGracefulShutdown(s, srv)

	if err := s.Serve(lis); err != nil {
		logrus.Fatalf("Failed to serve: %v", err)
	}
}
