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
	"service-platform/internal/pkg/fun"
	"service-platform/internal/pkg/logger"
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

	var msg *waE2E.Message
	switch content := req.Content.ContentType.(type) {
	case *pb.MessageContent_Text:
		msg = &waE2E.Message{
			Conversation: proto.String(content.Text),
		}
	case *pb.MessageContent_Location:
		loc := content.Location
		msg = &waE2E.Message{
			LocationMessage: &waE2E.LocationMessage{
				DegreesLatitude:  proto.Float64(loc.Latitude),
				DegreesLongitude: proto.Float64(loc.Longitude),
				Name:             proto.String(loc.Name),
				Address:          proto.String(loc.Address),
			},
		}
	case *pb.MessageContent_LiveLocation:
		loc := content.LiveLocation
		msg = &waE2E.Message{
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
		}
	case *pb.MessageContent_Poll:
		poll := content.Poll
		options := make([]*waE2E.PollCreationMessage_Option, len(poll.Options))
		for i, opt := range poll.Options {
			options[i] = &waE2E.PollCreationMessage_Option{
				OptionName: proto.String(opt),
			}
		}
		msg = &waE2E.Message{
			PollCreationMessage: &waE2E.PollCreationMessage{
				Name:                   proto.String(poll.Name),
				Options:                options,
				SelectableOptionsCount: proto.Uint32(poll.SelectableOptionsCount),
			},
		}
	case *pb.MessageContent_Contact:
		contact := content.Contact
		msg = &waE2E.Message{
			ContactMessage: &waE2E.ContactMessage{
				DisplayName: proto.String(contact.DisplayName),
				Vcard:       proto.String(contact.Vcard),
			},
		}
	case *pb.MessageContent_Media:
		media := content.Media
		if len(media.Data) == 0 {
			return &pb.SendMessageResponse{Success: false, Message: "Media data is empty"}, nil
		}

		var appMedia whatsmeow.MediaType
		switch media.MediaType {
		case "image":
			appMedia = whatsmeow.MediaImage
		case "video":
			appMedia = whatsmeow.MediaVideo
		case "audio":
			appMedia = whatsmeow.MediaAudio
		case "document":
			appMedia = whatsmeow.MediaDocument
		default:
			appMedia = whatsmeow.MediaDocument
		}

		// Upload the media
		uploaded, err := s.client.Upload(context.Background(), media.Data, appMedia)
		if err != nil {
			return &pb.SendMessageResponse{Success: false, Message: fmt.Sprintf("Failed to upload media: %v", err)}, nil
		}

		switch media.MediaType {
		case "image":
			msg = &waE2E.Message{
				ImageMessage: &waE2E.ImageMessage{
					URL:           proto.String(uploaded.URL),
					DirectPath:    proto.String(uploaded.DirectPath),
					MediaKey:      uploaded.MediaKey,
					Mimetype:      proto.String(media.Mimetype),
					FileEncSHA256: uploaded.FileEncSHA256,
					FileSHA256:    uploaded.FileSHA256,
					FileLength:    proto.Uint64(uint64(len(media.Data))),
					Caption:       proto.String(media.Caption),
					ViewOnce:      proto.Bool(media.ViewOnce),
				},
			}
		case "video":
			msg = &waE2E.Message{
				VideoMessage: &waE2E.VideoMessage{
					URL:           proto.String(uploaded.URL),
					DirectPath:    proto.String(uploaded.DirectPath),
					MediaKey:      uploaded.MediaKey,
					Mimetype:      proto.String(media.Mimetype),
					FileEncSHA256: uploaded.FileEncSHA256,
					FileSHA256:    uploaded.FileSHA256,
					FileLength:    proto.Uint64(uint64(len(media.Data))),
					Caption:       proto.String(media.Caption),
					ViewOnce:      proto.Bool(media.ViewOnce),
				},
			}
		case "audio":
			msg = &waE2E.Message{
				AudioMessage: &waE2E.AudioMessage{
					URL:           proto.String(uploaded.URL),
					DirectPath:    proto.String(uploaded.DirectPath),
					MediaKey:      uploaded.MediaKey,
					Mimetype:      proto.String(media.Mimetype),
					FileEncSHA256: uploaded.FileEncSHA256,
					FileSHA256:    uploaded.FileSHA256,
					FileLength:    proto.Uint64(uint64(len(media.Data))),
					ViewOnce:      proto.Bool(media.ViewOnce),
				},
			}
		default:
			// Create document message
			msg = &waE2E.Message{
				DocumentMessage: &waE2E.DocumentMessage{
					URL:           proto.String(uploaded.URL),
					DirectPath:    proto.String(uploaded.DirectPath),
					MediaKey:      uploaded.MediaKey,
					Mimetype:      proto.String(media.Mimetype),
					FileEncSHA256: uploaded.FileEncSHA256,
					FileSHA256:    uploaded.FileSHA256,
					FileLength:    proto.Uint64(uint64(len(media.Data))),
					FileName:      proto.String(media.Filename),
					Caption:       proto.String(media.Caption),
				},
			}
		}
	default:
		return &pb.SendMessageResponse{Success: false, Message: "Unsupported message type"}, nil
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

// CreateStatus posts a status update (story).
func (s *server) CreateStatus(ctx context.Context, req *pb.CreateStatusRequest) (*pb.CreateStatusResponse, error) {
	if s.client == nil || !s.client.IsConnected() {
		return &pb.CreateStatusResponse{Success: false, Message: "WhatsApp client not connected"}, nil
	}

	// Status JID is always status@broadcast
	targetJID := types.StatusBroadcastJID

	var msg *waE2E.Message

	switch content := req.Content.ContentType.(type) {
	case *pb.MessageContent_Text:
		// Text Status
		var bgColor *uint32
		if req.BackgroundColor != "" {
			// Remove # if present
			hexColor := strings.TrimPrefix(req.BackgroundColor, "#")
			if val, err := strconv.ParseUint(hexColor, 16, 32); err == nil {
				// WhatsApp expects ARGB, so if 6 chars (RRGGBB), prepend FF
				if len(hexColor) == 6 {
					val |= 0xFF000000
				}
				v := uint32(val)
				bgColor = &v
			}
		}

		// Default background color if not set (e.g. Purple 0xFF541560)
		if bgColor == nil {
			v := uint32(0xFF541560)
			bgColor = &v
		}

		// Text color (White)
		textColor := uint32(0xFFFFFFFF)

		font := waE2E.ExtendedTextMessage_FontType(req.Font)
		// Ensure valid font range (0-5), default to Sans Serif (0)
		if font < 0 || font > 5 {
			font = waE2E.ExtendedTextMessage_FontType(0)
		}

		msg = &waE2E.Message{
			ExtendedTextMessage: &waE2E.ExtendedTextMessage{
				Text:           proto.String(content.Text),
				BackgroundArgb: bgColor,
				TextArgb:       &textColor,
				Font:           &font,
				PreviewType:    waE2E.ExtendedTextMessage_NONE.Enum(),
			},
		}
	case *pb.MessageContent_Media:
		// Media Status
		media := content.Media

		var appMedia whatsmeow.MediaType
		switch media.MediaType {
		case "image":
			appMedia = whatsmeow.MediaImage
		case "video":
			appMedia = whatsmeow.MediaVideo
		case "audio":
			appMedia = whatsmeow.MediaAudio
		default:
			return &pb.CreateStatusResponse{Success: false, Message: "Unsupported media type for status"}, nil
		}

		// Upload media
		uploaded, err := s.client.Upload(context.Background(), media.Data, appMedia)
		if err != nil {
			return &pb.CreateStatusResponse{Success: false, Message: "Failed to upload media: " + err.Error()}, nil
		}

		switch media.MediaType {
		case "image":
			msg = &waE2E.Message{
				ImageMessage: &waE2E.ImageMessage{
					Caption:       proto.String(media.Caption),
					Mimetype:      proto.String(media.Mimetype),
					URL:           proto.String(uploaded.URL),
					DirectPath:    proto.String(uploaded.DirectPath),
					MediaKey:      uploaded.MediaKey,
					FileEncSHA256: uploaded.FileEncSHA256,
					FileSHA256:    uploaded.FileSHA256,
					FileLength:    proto.Uint64(uint64(len(media.Data))),
				},
			}
		case "video":
			msg = &waE2E.Message{
				VideoMessage: &waE2E.VideoMessage{
					Caption:       proto.String(media.Caption),
					Mimetype:      proto.String(media.Mimetype),
					URL:           proto.String(uploaded.URL),
					DirectPath:    proto.String(uploaded.DirectPath),
					MediaKey:      uploaded.MediaKey,
					FileEncSHA256: uploaded.FileEncSHA256,
					FileSHA256:    uploaded.FileSHA256,
					FileLength:    proto.Uint64(uint64(len(media.Data))),
				},
			}
		case "audio":
			msg = &waE2E.Message{
				AudioMessage: &waE2E.AudioMessage{
					Mimetype:      proto.String(media.Mimetype),
					URL:           proto.String(uploaded.URL),
					DirectPath:    proto.String(uploaded.DirectPath),
					MediaKey:      uploaded.MediaKey,
					FileEncSHA256: uploaded.FileEncSHA256,
					FileSHA256:    uploaded.FileSHA256,
					FileLength:    proto.Uint64(uint64(len(media.Data))),
					PTT:           proto.Bool(true), // Status audio is usually voice note style
				},
			}
		default:
			return &pb.CreateStatusResponse{Success: false, Message: "Unsupported media type for status"}, nil
		}
	default:
		return &pb.CreateStatusResponse{Success: false, Message: "Unsupported content type for status"}, nil
	}

	resp, err := s.client.SendMessage(ctx, targetJID, msg)
	if err != nil {
		return &pb.CreateStatusResponse{Success: false, Message: "Failed to send status: " + err.Error()}, nil
	}

	statusType := "Status"
	switch req.Content.ContentType.(type) {
	case *pb.MessageContent_Text:
		statusType = "Text status"
	case *pb.MessageContent_Media:
		statusType = "Media status"
	}

	return &pb.CreateStatusResponse{
		Success:  true,
		Message:  fmt.Sprintf("%s created successfully", statusType),
		StatusId: resp.ID,
	}, nil
}

// HasContacts checks if the user has any contacts.
func (s *server) HasContacts(ctx context.Context, req *pb.HasContactsRequest) (*pb.HasContactsResponse, error) {
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
func (s *server) GetMessages(ctx context.Context, req *pb.GetMessagesRequest) (*pb.GetMessagesResponse, error) {
	// For now, return empty - would need to implement message history retrieval
	return &pb.GetMessagesResponse{Messages: []*pb.Message{}}, nil
}

// initializeClient sets up the WhatsApp client, including database connection and logger.
// It ensures that the client is only initialized once.
func (s *server) initializeClient() error {
	if s.client != nil {
		return nil
	}

	// Load config
	if err := config.LoadConfig(); err != nil {
		return fmt.Errorf("failed to load config: %v", err)
	}
	cfg := config.GetConfig()

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
	dbLevel := logger.ParseWhatsmeowLogLevel(config.GetConfig().Whatsnyan.DBLogLevel)
	clientLevel := logger.ParseWhatsmeowLogLevel(config.GetConfig().Whatsnyan.ClientLogLevel)
	logDir, err := fun.FindValidDirectory([]string{
		"log",
		"../log",
		"../../log",
		"../../../log",
	})
	if err != nil {
		return fmt.Errorf("failed to find log directory: %v", err)
	}
	dbLogFile := filepath.Join(logDir, config.GetConfig().Whatsnyan.DBLog)
	clientLogFile := filepath.Join(logDir, config.GetConfig().Whatsnyan.ClientLog)

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

// Connect handles the connection process to WhatsApp.
// It initializes the client if needed, checks for existing sessions,
// and generates a QR code if a new login is required.
func (s *server) Connect(ctx context.Context, req *pb.ConnectRequest) (*pb.ConnectResponse, error) {
	// Initialize WhatsApp client if not already done
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

	// Check if we should force QR code generation (skip existing session check)
	if !req.ForceQr && s.client.Store.ID != nil {
		// Try to connect with existing session
		err := s.client.Connect()
		if err != nil {
			logrus.Warnf("Failed to connect with existing session: %v", err)
			// Continue to QR generation
		} else {
			return &pb.ConnectResponse{Success: true, Message: "Connected with existing session"}, nil
		}
	}

	// No valid session, need QR code or phone pairing
	qrChan, err := s.client.GetQRChannel(context.Background())
	if err != nil {
		return &pb.ConnectResponse{Success: false, Message: fmt.Sprintf("Failed to get QR channel: %v", err)}, nil
	}

	err = s.client.Connect()
	if err != nil {
		return &pb.ConnectResponse{Success: false, Message: fmt.Sprintf("Failed to connect: %v", err)}, nil
	}

	// Check if phone pairing is requested (config must enable it)
	// If enable_phone_pairing is false, always use QR code
	enablePhonePairing := config.GetConfig().Whatsnyan.EnablePhonePairing
	usePhonePairing := enablePhonePairing && (req.PhoneNumber != "" || config.GetConfig().Whatsnyan.PairingPhoneNumber != "")
	pairingPhone := req.PhoneNumber
	if pairingPhone == "" && enablePhonePairing {
		pairingPhone = config.GetConfig().Whatsnyan.PairingPhoneNumber
	}

	if usePhonePairing && pairingPhone != "" {
		// Wait for first QR event to ensure connection is established
		logrus.Info("Waiting for connection to establish before phone pairing...")
		firstEvt := <-qrChan
		if firstEvt.Event != "code" {
			// If first event is not a code, handle other cases
			if firstEvt.Event == "success" {
				return &pb.ConnectResponse{
					Success: true,
					Message: fmt.Sprintf("Connected successfully as %s", firstEvt.Code),
				}, nil
			}
			return &pb.ConnectResponse{
				Success: false,
				Message: fmt.Sprintf("Unexpected event: %s", firstEvt.Event),
			}, nil
		}

		// Check for rate limiting - don't allow pairing attempts too frequently
		const cooldownPeriod = 5 * time.Minute // 5 minutes cooldown between pairing attempts
		if lastAttempt, exists := s.pairingAttempts[pairingPhone]; exists {
			timeSinceLastAttempt := time.Since(lastAttempt)
			if timeSinceLastAttempt < cooldownPeriod {
				remainingTime := cooldownPeriod - timeSinceLastAttempt
				return &pb.ConnectResponse{
					Success: false,
					Message: fmt.Sprintf("Please wait %v before attempting to pair again. WhatsApp limits pairing attempts.", remainingTime.Round(time.Second)),
				}, nil
			}
		}

		// Connection established, now request pairing code
		logrus.Infof("Requesting phone pairing for: %s", pairingPhone)
		pairingCode, err := s.client.PairPhone(context.Background(), pairingPhone, true, whatsmeow.PairClientChrome, "Chrome (Linux)")
		if err != nil {
			errorMsg := fmt.Sprintf("Phone pairing failed: %v", err)
			// Check for rate limit errors and provide more helpful message
			if strings.Contains(err.Error(), "rate-overlimit") || strings.Contains(err.Error(), "429") {
				errorMsg = "Rate limit exceeded. Please wait 5-10 minutes before trying to pair again. WhatsApp limits pairing attempts to prevent abuse."
				logrus.Warn("Rate limit hit during phone pairing, suggesting cooldown period")
			}
			return &pb.ConnectResponse{
				Success: false,
				Message: errorMsg,
			}, nil
		}

		logrus.Infof("✅ Pairing code generated: %s", pairingCode)
		logrus.Info("Enter this code in WhatsApp: Settings > Linked Devices > Link a Device > Link with Phone Number")

		// Record successful pairing attempt
		s.pairingAttempts[pairingPhone] = time.Now()

		// Return pairing code immediately so frontend can display it
		// The pairing success/failure will be monitored asynchronously
		return &pb.ConnectResponse{
			Success:     true,
			Message:     "Pairing code generated. Enter this code in WhatsApp on your phone.",
			PairingCode: pairingCode,
		}, nil

		// Note: The original code below would wait for pairing completion,
		// but we return immediately so the frontend can display the code
		/*
			// Continue listening for pairing success
			for evt := range qrChan {
				switch evt.Event {
				case "success":
					logrus.Infof("✅ Phone pairing successful! Connected as: %s", evt.Code)
					return &pb.ConnectResponse{
						Success:     true,
						Message:     fmt.Sprintf("Phone pairing successful. Connected as %s", evt.Code),
						PairingCode: pairingCode,
					}, nil
				case "timeout":
					return &pb.ConnectResponse{
						Success:     false,
						Message:     "Pairing code expired. Please try again.",
						PairingCode: pairingCode,
					}, nil
				case "code":
					// Ignore additional QR codes during phone pairing
					continue
				default:
					logrus.Warnf("Unexpected event during phone pairing: %s", evt.Event)
				}
			}
			return &pb.ConnectResponse{
				Success:     false,
				Message:     "Phone pairing channel closed unexpectedly",
				PairingCode: pairingCode,
			}, nil
		*/
	}

	// Wait for QR code or successful login (original flow)
	for evt := range qrChan {
		switch evt.Event {
		case "success":
			numberConnected := s.client.Store.ID.User
			return &pb.ConnectResponse{Success: true, Message: fmt.Sprintf("Whatsapp logged in successfully as %s", numberConnected)}, nil
		case "timeout":
			return &pb.ConnectResponse{Success: false, Message: "QR code scan timed out"}, nil
		case "code":
			// Generate QR code image
			qrCode := evt.Code

			// Create directory: web/file/wa_qr/YYYY-MM-DD/
			now := time.Now()
			dateStr := now.Format(config.DATE_YYYY_MM_DD)

			dirQR, err := fun.FindValidDirectory([]string{
				"web/file/wa_qr",
				"../web/file/wa_qr",
				"../../web/file/wa_qr",
				"../../../web/file/wa_qr",
			})
			if err != nil {
				return &pb.ConnectResponse{Success: false, Message: fmt.Sprintf("Failed to find QR directory: %v", err)}, nil
			}

			dirPath := filepath.Join(dirQR, dateStr)

			if err := os.MkdirAll(dirPath, 0755); err != nil {
				return &pb.ConnectResponse{Success: false, Message: fmt.Sprintf("Failed to create directory: %v", err)}, nil
			}

			// Filename: qr_<timestamp>.png
			filename := fmt.Sprintf("qr_%d.png", now.UnixNano())
			filePath := filepath.Join(dirPath, filename)

			// Generate and save
			qrc, err := qrcode.New(qrCode)
			if err != nil {
				return &pb.ConnectResponse{Success: false, Message: fmt.Sprintf("Failed to create QR object: %v", err)}, nil
			}

			webDir, err := fun.FindValidDirectory([]string{
				"web",
				"../web",
				"../../web",
				"../../../web",
			})
			if err != nil {
				return &pb.ConnectResponse{Success: false, Message: fmt.Sprintf("Failed to find web directory: %v", err)}, nil
			}
			logoPath := config.GetConfig().App.LogoJPG
			if logoPath == "" {
				return &pb.ConnectResponse{Success: false, Message: "Logo path is not configured"}, nil
			}
			logoFullPath := filepath.Join(webDir, logoPath)
			file, err := os.Open(logoFullPath)
			if err != nil {
				return &pb.ConnectResponse{Success: false, Message: fmt.Sprintf("Failed to open logo file: %v", err)}, nil
			}
			defer file.Close()

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

			return &pb.ConnectResponse{
				Success: true,
				Message: "QR code generated",
				QrCode:  qrCode,
			}, nil
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

// Disconnect closes the connection to WhatsApp.
func (s *server) Disconnect(ctx context.Context, req *pb.DisconnectRequest) (*pb.DisconnectResponse, error) {
	if s.client != nil && s.client.IsConnected() {
		s.client.Disconnect()
		return &pb.DisconnectResponse{Success: true, Message: "Disconnected successfully"}, nil
	}
	return &pb.DisconnectResponse{Success: false, Message: "Not connected"}, nil
}

// IsConnected checks if the WhatsApp client is currently connected.
func (s *server) IsConnected(ctx context.Context, req *pb.IsConnectedRequest) (*pb.IsConnectedResponse, error) {
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
func (s *server) Logout(ctx context.Context, req *pb.WALogoutRequest) (*pb.WALogoutResponse, error) {
	if s.client != nil {
		err := s.client.Logout(ctx)
		if err != nil {
			return &pb.WALogoutResponse{Success: false, Message: fmt.Sprintf("Logout failed: %v", err)}, nil
		}
		return &pb.WALogoutResponse{Success: true, Message: "Logged out successfully"}, nil
	}
	return &pb.WALogoutResponse{Success: false, Message: "Client not initialized"}, nil
}

// RefreshQR forces a logout and generates a new QR code for connection.
func (s *server) RefreshQR(ctx context.Context, req *pb.RefreshQRRequest) (*pb.ConnectResponse, error) {
	if s.client == nil {
		return &pb.ConnectResponse{Success: false, Message: "Client not initialized"}, nil
	}

	// If force_new is true, skip checking for existing QR files and generate a new one
	if !req.GetForceNew() {
		// Check for existing recent QR PNG files first
		dirQR, err := fun.FindValidDirectory([]string{
			"web/file/wa_qr",
			"../web/file/wa_qr",
			"../../web/file/wa_qr",
			"../../../web/file/wa_qr",
		})
		if err == nil {
			now := time.Now()
			dateStr := now.Format(config.DATE_YYYY_MM_DD)
			dirPath := filepath.Join(dirQR, dateStr)

			// Check if directory exists
			if _, err := os.Stat(dirPath); err == nil {
				// Look for QR PNG files from the last 5 minutes
				files, err := os.ReadDir(dirPath)
				if err == nil {
					const maxAge = 5 * time.Minute
					var latestQRFile string
					var latestTime time.Time

					for _, file := range files {
						if strings.HasSuffix(file.Name(), ".png") && strings.HasPrefix(file.Name(), "qr_") {
							filePath := filepath.Join(dirPath, file.Name())
							fileInfo, err := os.Stat(filePath)
							if err != nil {
								continue
							}

							if now.Sub(fileInfo.ModTime()) <= maxAge {
								if fileInfo.ModTime().After(latestTime) {
									latestTime = fileInfo.ModTime()
									latestQRFile = filePath
								}
							}
						}
					}

					// If we found a recent QR PNG file, return its path for proxy serving
					if latestQRFile != "" {
						logrus.Infof("Using existing QR PNG from: %s", latestQRFile)
						return &pb.ConnectResponse{
							Success: true,
							Message: "QR code refreshed (using existing)",
							QrCode:  latestQRFile, // Return file path for proxy serving
						}, nil
					}
				}
			}
		}
	}

	// No recent QR found or force_new requested, generate a new one
	logrus.Info("Generating new QR code (force_new or no recent QR found)")

	// Force logout if logged in or if session exists to ensure new QR generation
	if s.client.IsLoggedIn() || s.client.Store.ID != nil {
		logrus.Info("RefreshQR requested. Logging out existing session.")
		err := s.client.Logout(ctx)
		if err != nil {
			logrus.Errorf("Failed to logout during refresh: %v", err)
			// Continue anyway to try to reset
		}
	}

	// Disconnect if connected (to reset state)
	if s.client.IsConnected() {
		s.client.Disconnect()
	}

	// Call Connect to generate a new QR
	return s.Connect(ctx, &pb.ConnectRequest{})
}

// GetMe retrieves the current user's information.
func (s *server) GetMe(ctx context.Context, req *pb.GetMeRequest) (*pb.GetMeResponse, error) {
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

	return &pb.GetMeResponse{
		Success:       true,
		Message:       "User info retrieved successfully",
		Jid:           userJID,
		PhoneNumber:   phoneNumber,
		Name:          pushName,
		ProfilePicUrl: profilePicURL,
		Device:        device,
		Platform:      platform,
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
func (s *server) GetJoinedGroups(ctx context.Context, req *pb.GetJoinedGroupsRequest) (*pb.GetJoinedGroupsResponse, error) {
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
		// Map to WhatsAppGroup model
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

		// Upsert Group
		if err := s.db.Clauses(clause.OnConflict{
			Columns:   []clause.Column{{Name: "jid"}},
			UpdateAll: true,
		}).Create(&groupModel).Error; err != nil {
			logrus.Errorf("Failed to upsert group %s: %v", group.Name, err)
			continue
		}

		// Update Participants
		err := s.db.Transaction(func(tx *gorm.DB) error {
			if err := tx.Where("group_jid = ?", group.JID.String()).Delete(&whatsnyanmodel.WhatsAppGroupParticipant{}).Error; err != nil {
				return err
			}

			if len(group.Participants) > 0 {
				participants := make([]whatsnyanmodel.WhatsAppGroupParticipant, len(group.Participants))
				for i, p := range group.Participants {
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
					if pic, err := s.client.GetProfilePictureInfo(context.Background(), p.JID, nil); err == nil && pic != nil {
						profilePicURL = pic.URL
					}

					participants[i] = whatsnyanmodel.WhatsAppGroupParticipant{
						GroupJID:          group.JID.String(),
						UserJID:           p.JID.String(),
						LID:               p.LID.String(),
						DisplayName:       displayName,
						IsAdmin:           p.IsAdmin,
						IsSuperAdmin:      p.IsSuperAdmin,
						PhoneNumber:       phoneNumber,
						ProfilePictureURL: profilePicURL,
					}
				}
				if err := tx.Create(&participants).Error; err != nil {
					return err
				}
			}
			return nil
		})

		if err != nil {
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

// eventHandler handles incoming events from the WhatsApp client.
// It processes various event types including:
// - Connection state changes (Connected, Disconnected, LoggedOut)
// - Message receipts (updates message status in DB)
// - Call offers (rejects unauthorized calls based on DB config)
// - Incoming messages (saves to DB, handles auto-replies, media downloads)
// - Message reactions and edits
func (s *server) eventHandler(evt interface{}) {
	cwd, err := os.Getwd()
	if err != nil {
		logrus.Errorf("Failed to get working directory: %v", err)
	}

	baseDir := filepath.Join(cwd, "web", "file")
	if _, err := os.Stat(baseDir); os.IsNotExist(err) {
		// logrus.Errorf("Directory does not exist: %s, try searching it dynamically", baseDir)
		baseDir, err = fun.FindValidDirectory([]string{
			"web/file",
			"../web/file",
			"../../web/file",
			"../../../web/file",
		})
		if err != nil {
			logrus.Errorf("Failed to find base directory: %v", err)
			return
		}
	}

	uploadDir := filepath.Join(baseDir, "wa_reply", time.Now().Format(config.DATE_YYYY_MM_DD))
	if err := os.MkdirAll(uploadDir, 0755); err != nil {
		logrus.Errorf("Failed to create upload directory: %v", err)
		return
	}

	switch v := evt.(type) {
	case *events.Connected:
		jid := s.client.Store.ID.User
		logrus.Infof("✅ WhatsApp %v connected", jid)
		s.SyncGroups()
	case *events.Disconnected:
		var jidStr string
		if s.client.Store.ID != nil {
			jidStr = s.client.Store.ID.User
		} else {
			jidStr = "unknown"
		}

		logrus.Infof("❌ WhatsApp %s disconnected", jidStr)
	case *events.LoggedOut:
		jid := s.client.Store.ID.User
		logrus.Warnf("Received LoggedOut 🏃🏻 event for %s", jid)
		// whatsmeow automatically deletes the device from the store on LoggedOut
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
		callerJID := v.CallCreator
		phoneNumber := callerJID.User

		if callerJID.Server == "lid" {
			var lidMap whatsnyanmodel.WhatsmeowLIDMap
			if err := s.db.Where("lid = ?", callerJID.User).First(&lidMap).Error; err == nil {
				phoneNumber = lidMap.PN
			}
		} else {
			sanitizedPhone, err := fun.SanitizeIndonesiaPhoneNumber(phoneNumber)
			if err == nil {
				phoneNumber = config.GetConfig().Default.DialingCodeDefault + sanitizedPhone
			} else {
				logrus.Errorf("Failed to sanitize phone number %s: %v", phoneNumber, err)
			}
		}

		if config.GetConfig().Whatsnyan.NeedVerifyAccount {
			callReject := true
			var waUser model.WAUsers
			if err := s.db.Where("phone_number = ?", phoneNumber).First(&waUser).Error; err == nil {
				callReject = !waUser.AllowedToCall
			}

			logrus.Infof("Incoming call from %s", phoneNumber)

			if callReject {
				if err := s.client.RejectCall(context.Background(), v.CallCreator, v.CallID); err != nil {
					logrus.Errorf("Failed to reject call: %v", err)
				}

				langMessages := make(map[string]string)
				langMessages[fun.LangID] = fmt.Sprintf("📞❌ Maaf, nomor *%s* tidak diizinkan untuk melakukan panggilan ke WhatsApp ini.\n\nApabila ada kendala, Anda bisa menghubungi layanan bantuan teknis kami di nomor berikut: +%s. Terima kasih. 🙏",
					phoneNumber, config.GetConfig().Whatsnyan.WATechnicalSupport)

				langMessages[fun.LangEN] = fmt.Sprintf("📞❌ Sorry, the number *%s* is not allowed to make calls to this WhatsApp.\n\nIf you have any issues, you can contact our technical support service at the following number: +%s. Thank you. 🙏",
					phoneNumber, config.GetConfig().Whatsnyan.WATechnicalSupport)

				langMessages[fun.LangES] = fmt.Sprintf("📞❌ Lo sentimos, el número *%s* no tiene permitido llamar a este WhatsApp.\n\nSi tienes algún inconveniente, puedes contactar a nuestro servicio de soporte técnico al siguiente número: +%s. Gracias. 🙏",
					phoneNumber, config.GetConfig().Whatsnyan.WATechnicalSupport)

				langMessages[fun.LangFR] = fmt.Sprintf("📞❌ Désolé, le numéro *%s* n'est pas autorisé à appeler ce WhatsApp.\n\nEn cas de problème, vous pouvez contacter notre support technique au numéro suivant : +%s. Merci. 🙏",
					phoneNumber, config.GetConfig().Whatsnyan.WATechnicalSupport)

				langMessages[fun.LangDE] = fmt.Sprintf("📞❌ Entschuldigung, die Nummer *%s* darf diesen WhatsApp-Account nicht anrufen.\n\nBei Problemen können Sie unseren technischen Support unter folgender Nummer kontaktieren: +%s. Vielen Dank. 🙏",
					phoneNumber, config.GetConfig().Whatsnyan.WATechnicalSupport)

				langMessages[fun.LangPT] = fmt.Sprintf("📞❌ Desculpe, o número *%s* não está autorizado a fazer chamadas para este WhatsApp.\n\nSe tiver algum problema, você pode contatar nosso suporte técnico pelo seguinte número: +%s. Obrigado. 🙏",
					phoneNumber, config.GetConfig().Whatsnyan.WATechnicalSupport)

				langMessages[fun.LangAR] = fmt.Sprintf("📞❌ عذرًا، الرقم *%s* غير مسموح له بإجراء مكالمات إلى هذا الواتساب.\n\nإذا واجهت أي مشكلة، يمكنك التواصل مع الدعم الفني عبر الرقم التالي: +%s. شكرًا لك. 🙏",
					phoneNumber, config.GetConfig().Whatsnyan.WATechnicalSupport)

				langMessages[fun.LangJP] = fmt.Sprintf("📞❌ 申し訳ありませんが、番号 *%s* はこのWhatsAppへの通話が許可されていません。\n\n問題がある場合は、次の番号から技術サポートにお問い合わせください: +%s。ありがとうございます。🙏",
					phoneNumber, config.GetConfig().Whatsnyan.WATechnicalSupport)

				langMessages[fun.LangCN] = fmt.Sprintf("📞❌ 抱歉，号码 *%s* 不允许拨打此 WhatsApp。\n\n如果您有任何问题，可以联系技术支持：+%s。谢谢。🙏",
					phoneNumber, config.GetConfig().Whatsnyan.WATechnicalSupport)

				langMessages[fun.LangRU] = fmt.Sprintf("📞❌ Извините, номер *%s* не может совершать звонки на этот WhatsApp.\n\nЕсли у вас возникли проблемы, вы можете связаться с нашей технической поддержкой по номеру: +%s. Спасибо. 🙏",
					phoneNumber, config.GetConfig().Whatsnyan.WATechnicalSupport)

				lang := controllers.NewLanguageMsgTranslation(fun.DefaultLang)
				lang.Texts = langMessages

				jidStr := fmt.Sprintf("%s@%s", phoneNumber, types.DefaultUserServer)
				controllers.SendLangWhatsAppTextMsg(
					jidStr,
					"",
					nil,
					lang,
					lang.LanguageCode,
					s.client, s.rdb, s.db)
			}
		}
	case *events.Message:
		// Process incoming message asynchronously
		go s.processIncomingWhatsappMessage(v)

		// 🔧 Extract context info from any type
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

		// 💬 Handle replies
		if ctxInfo != nil && ctxInfo.QuotedMessage != nil && ctxInfo.StanzaID != nil && *ctxInfo.StanzaID != "" {
			var replyText string
			waReplyPublicURL := config.GetConfig().Whatsnyan.WAReplyPublicURL + "/" + time.Now().Format(config.DATE_YYYY_MM_DD)

			switch {

			case v.Message.Conversation != nil:
				replyText = *v.Message.Conversation

			case v.Message.ExtendedTextMessage != nil && v.Message.ExtendedTextMessage.Text != nil:
				replyText = *v.Message.ExtendedTextMessage.Text

			case v.Message.ImageMessage != nil:
				msg := v.Message.ImageMessage
				data, err := s.client.Download(context.Background(), msg)
				if err != nil {
					logrus.Error("Failed to download image:", err)
					break
				}
				mimeType := fun.GetSafeString(msg.Mimetype)
				ext := fun.GetFileExtension(mimeType)
				filename := fmt.Sprintf("img_%d%s", time.Now().UnixNano(), ext)
				savePath := filepath.Join(uploadDir, filename)
				os.WriteFile(savePath, data, 0644)
				publicURL := fmt.Sprintf("%s/%s", waReplyPublicURL, filename)
				caption := fun.GetSafeString(msg.Caption)
				replyText = fmt.Sprintf("📷 %s %s", caption, publicURL)

			case v.Message.VideoMessage != nil:
				msg := v.Message.VideoMessage
				data, err := s.client.Download(context.Background(), msg)
				if err != nil {
					logrus.Info("Failed to download video:", err)
					break
				}
				mimeType := fun.GetSafeString(msg.Mimetype)
				ext := fun.GetFileExtension(mimeType)
				filename := fmt.Sprintf("vid_%d%s", time.Now().UnixNano(), ext)
				savePath := filepath.Join(uploadDir, filename)
				os.WriteFile(savePath, data, 0644)
				publicURL := fmt.Sprintf("%s/%s", waReplyPublicURL, filename)
				caption := fun.GetSafeString(msg.Caption)
				replyText = fmt.Sprintf("🎥 %s %s", caption, publicURL)

			case v.Message.AudioMessage != nil:
				msg := v.Message.AudioMessage
				data, err := s.client.Download(context.Background(), msg)
				if err != nil {
					logrus.Error("Failed to download audio:", err)
					break
				}
				mimeType := fun.GetSafeString(msg.Mimetype)
				ext := fun.GetFileExtension(mimeType)
				filename := fmt.Sprintf("aud_%d%s", time.Now().UnixNano(), ext)
				savePath := filepath.Join(uploadDir, filename)
				os.WriteFile(savePath, data, 0644)
				publicURL := fmt.Sprintf("%s/%s", waReplyPublicURL, filename)
				replyText = fmt.Sprintf("🎧 Audio message: %s", publicURL)

			case v.Message.DocumentMessage != nil:
				msg := v.Message.DocumentMessage
				data, err := s.client.Download(context.Background(), msg)
				if err != nil {
					logrus.Error("Failed to download document:", err)
					break
				}
				mimeType := fun.GetSafeString(msg.Mimetype)
				ext := fun.GetFileExtension(mimeType)
				filename := fmt.Sprintf("doc_%d%s", time.Now().UnixNano(), ext)
				savePath := filepath.Join(uploadDir, filename)
				os.WriteFile(savePath, data, 0644)
				publicURL := fmt.Sprintf("%s/%s", waReplyPublicURL, filename)
				caption := fun.GetSafeString(msg.Caption)
				replyText = fmt.Sprintf("📄 %s %s", caption, publicURL)

			case v.Message.StickerMessage != nil:
				msg := v.Message.StickerMessage
				data, err := s.client.Download(context.Background(), msg)
				if err != nil {
					logrus.Error("Failed to download sticker:", err)
					break
				}
				mimeType := fun.GetSafeString(msg.Mimetype)
				ext := fun.GetFileExtension(mimeType)
				filename := fmt.Sprintf("stk_%d%s", time.Now().UnixNano(), ext)
				savePath := filepath.Join(uploadDir, filename)
				os.WriteFile(savePath, data, 0644)
				publicURL := fmt.Sprintf("%s/%s", waReplyPublicURL, filename)
				replyText = fmt.Sprintf("🖼️ Sticker: %s", publicURL)

			default:
				replyText = "(non-text or unknown reply)"
			}

			var repliedAt *time.Time
			if !v.Info.Timestamp.IsZero() {
				t := v.Info.Timestamp
				repliedAt = &t
			} else {
				now := time.Now()
				repliedAt = &now
			}

			err := s.db.Model(&whatsnyanmodel.WhatsAppMsg{}).
				Where("whatsapp_chat_id = ?", *ctxInfo.StanzaID).
				Updates(map[string]interface{}{
					"whatsapp_replied_by": v.Info.Sender.String(),
					"whatsapp_replied_at": repliedAt,
					"whatsapp_reply_text": replyText,
				}).Error

			if err != nil {
				logrus.Printf("Failed to update reply info: %v", err)
			}
		}

		// 🤖 Handle reactions
		var reactedAt *time.Time
		if !v.Info.Timestamp.IsZero() {
			t := v.Info.Timestamp
			reactedAt = &t
		} else {
			now := time.Now()
			reactedAt = &now
		}
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

		// ✏️ Edited Message
		if pm := v.Message.GetProtocolMessage(); pm != nil {
			if pm.GetType() == waE2E.ProtocolMessage_MESSAGE_EDIT {
				var replyText string
				var repliedToMsgID string
				edited := pm.GetEditedMessage()

				// Case: edited reply (ExtendedTextMessage with context)
				if etm := edited.GetExtendedTextMessage(); etm != nil {
					replyText = etm.GetText()
					// fmt.Println("📝 Edited reply text:", replyText)

					// ✅ This is the message ID you originally replied to
					if ctx := etm.GetContextInfo(); ctx != nil && ctx.GetStanzaID() != "" {
						repliedToMsgID = ctx.GetStanzaID()
					} else {
						logrus.Println("❌ No ContextInfo or stanzaID in edited reply")
					}
				}

				// Case: plain edited message (no reply, just a text change)
				if replyText == "" && edited.GetConversation() != "" {
					replyText = edited.GetConversation()
					// Not a reply, just text coz whatsapp desktop didnt show the replied msg destination only edited reply
					logrus.Println("📝 Edited plain text (not a reply)")
				}

				// 💾 Update quoted message with edited reply
				var repliedAt *time.Time
				if !v.Info.Timestamp.IsZero() {
					t := v.Info.Timestamp
					repliedAt = &t
				} else {
					now := time.Now()
					repliedAt = &now
				}
				if repliedToMsgID != "" && replyText != "" {
					err := s.db.Model(&whatsnyanmodel.WhatsAppMsg{}).
						Where("whatsapp_chat_id = ?", repliedToMsgID).
						Updates(map[string]interface{}{
							"whatsapp_reply_text": replyText,
							"whatsapp_replied_at": repliedAt,
							"whatsapp_replied_by": v.Info.Sender.String(),
						}).Error

					if err != nil {
						logrus.Errorf("Failed to update quoted message (reply edit): %v", err)
					}
				}
			}
		}
	}
}

func (s *server) processIncomingWhatsappMessage(v *events.Message) {
	// Only process messages sent within the tolerance period to avoid processing old replayed messages
	toleranceHours := config.GetConfig().Whatsnyan.MessageProcessedToleranceHours
	if toleranceHours <= 0 {
		toleranceHours = 24 // default to 24 hours
	}
	cutoff := time.Now().Add(-time.Duration(toleranceHours) * time.Hour)
	if v.Info.Timestamp.Before(cutoff) {
		logrus.Debugf("Skipping old message sent at %v (older than %d hours)", v.Info.Timestamp, toleranceHours)
		return
	}

	// Unwrap the message if it's wrapped (e.g. Ephemeral, ViewOnce, GroupMentioned)
	msg := v.Message
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

	// Extract message content for storage
	var msgBody string
	var msgType string

	switch {
	case msg.GetConversation() != "":
		msgBody = msg.GetConversation()
		msgType = "text"
	case msg.GetExtendedTextMessage() != nil:
		msgBody = msg.GetExtendedTextMessage().GetText()
		msgType = "text"
	case msg.GetImageMessage() != nil:
		msgBody = msg.GetImageMessage().GetCaption()
		msgType = "image"
	case msg.GetVideoMessage() != nil:
		msgBody = msg.GetVideoMessage().GetCaption()
		msgType = "video"
	case msg.GetAudioMessage() != nil:
		msgType = "audio"
	case msg.GetDocumentMessage() != nil:
		msgBody = msg.GetDocumentMessage().GetCaption()
		msgType = "document"
	case msg.GetStickerMessage() != nil:
		msgType = "sticker"
	default:
		msgType = "unknown"
	}

	// Save incoming message
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
		// Ignore messages sent by ourselves
		return
	}

	originalSenderJID := controllers.NormalizeSenderJID(v.Info.Sender.String())
	stanzaID := v.Info.ID
	lowerMsgText := strings.ToLower(strings.TrimSpace(msgBody))

	var senderPhoneNumber, senderName string
	senderPhoneNumberUnsanitized := v.Info.Sender.User
	var lidMap whatsnyanmodel.WhatsmeowLIDMap
	err := s.db.Where("pn = ?", senderPhoneNumberUnsanitized).First(&lidMap).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			err2 := s.db.Where("lid = ?", senderPhoneNumberUnsanitized).First(&lidMap).Error
			if err2 != nil {
				if errors.Is(err2, gorm.ErrRecordNotFound) {
					logrus.Errorf("Failed to find any phone number using %s : %v", senderPhoneNumberUnsanitized, err2)
				} else {
					logrus.Errorf("got error while trying to search phone number : %v", err2)
				}
			} else {
				senderPhoneNumber = lidMap.PN
			}
		} else {
			logrus.Errorf("Failed to find phone number based on sender JID: %v using %s", err, senderPhoneNumberUnsanitized)
		}
	} else {
		senderPhoneNumber = lidMap.PN
	}

	// Fallback: If senderPhoneNumber is still empty, and the sender is not a LID, use the User part directly
	if senderPhoneNumber == "" && v.Info.Sender.Server != "lid" {
		logrus.Warnf("Phone number not found in LID map, using raw User ID: %s", senderPhoneNumberUnsanitized)
		senderPhoneNumber = senderPhoneNumberUnsanitized
	}

	if senderPhoneNumber == "" {
		// Skip processing if sender phone number is not found
		logrus.Warnf("Sender phone number not found for JID: %s. Skipping processing.", v.Info.Sender.String())
		return
	}

	if v.Info.PushName != "" {
		senderName = v.Info.PushName
	}
	if v.Info.VerifiedName != nil && v.Info.VerifiedName.Details != nil {
		senderName = v.Info.VerifiedName.Details.GetVerifiedName()
	}

	if senderName == "" {
		// Try finding sender name from other sources
		var waUser model.WAUsers
		if err := s.db.Where("phone_number = ?", senderPhoneNumber).First(&waUser).Error; err == nil {
			senderName = waUser.FullName
		} else {
			var waContacts whatsnyanmodel.WhatsmeowContacts
			if err := s.db.Where("their_jid LIKE ?", senderPhoneNumber).First(&waContacts).Error; err == nil {
				if *waContacts.FullName != "" {
					senderName = *waContacts.FullName
				}
				if *waContacts.PushName != "" {
					senderName = *waContacts.PushName
				}
				if *waContacts.BusinessName != "" {
					senderName = *waContacts.BusinessName
				}
			}
		}
	}

	if senderName == "" {
		senderName = "N/A"
	}

	if s.client == nil {
		logrus.Error("WhatsApp client is not initialized")
		return
	}

	// Get user language (earlier)
	userLang, err := controllers.GetUserLang(originalSenderJID, s.rdb)
	if err != nil {
		logrus.Errorf("Failed to get user language: %v", err)
		userLang = fun.DefaultLang
	}

	controllers.HandleLanguageChange(originalSenderJID, lowerMsgText, s.client, s.rdb, s.db)

	// Show prompt language if not shown recently
	if userLang == "" {
		langPromptKey := fmt.Sprintf("lang_prompted_%s", originalSenderJID)
		exists, err := s.rdb.Exists(context.Background(), langPromptKey).Result()
		if err != nil {
			logrus.Errorf("Failed to check language prompt key: %v", err)
		}

		if exists == 0 {
			langPrompt := config.GetConfig().Whatsnyan.LanguagePrompt
			if len(langPrompt) == 0 {
				return
			} else {
				langMsg := make(map[string]string)
				langMsg[fun.DefaultLang] = langPrompt
				lang := controllers.NewLanguageMsgTranslation(fun.DefaultLang)
				lang.Texts = langMsg
				controllers.SendLangWhatsAppTextMsg(
					originalSenderJID,
					"",
					v,
					lang,
					lang.LanguageCode,
					s.client, s.rdb, s.db)

				if err := s.rdb.Set(context.Background(), langPromptKey, "true", time.Duration(config.GetConfig().Whatsnyan.LanguagePromptShownExpiry)*time.Second).Err(); err != nil {
					logrus.Errorf("Failed to set language prompt key: %v", err)
					return
				}
			}
		}
	}

	// Processing messages if sender user is verified
	allowedWAG := config.GetConfig().Whatsnyan.WAGAllowedToInteract

	/* WhatsApp Group Chat */
	if len(allowedWAG) > 0 {
		if v.Info.IsGroup && controllers.ContainsJID(allowedWAG, v.Info.Chat) {
			jidStr := fmt.Sprintf("%s@%s", senderPhoneNumber, types.DefaultUserServer)

			switch strings.TrimSpace(lowerMsgText) {
			case "halo", "hello", "hola":
				controllers.HelloFromBot(v, senderPhoneNumber, senderName, stanzaID, jidStr, s.client, s.rdb, s.db)
				return
			}
		}
	}

	var userSanitizeResult *model.WAUsers
	var shouldProcess bool = true

	if config.GetConfig().Whatsnyan.NeedVerifyAccount {
		/* WhatSapp Direct Chat */

		// logrus.Debugf("Verifying user %s (JID: %s)...", senderPhoneNumber, originalSenderJID)

		// Try to verify the user first if registered or not
		var langErr map[string]string
		userSanitizeResult, langErr = controllers.ValidateUserToUseBotWhatsapp(
			senderPhoneNumber,
			originalSenderJID,
			v.Info.IsGroup,
			msgType,
			s.client,
			s.rdb,
			s.db,
		)
		if userSanitizeResult == nil {
			logrus.Warnf("User %s not allowed to use bot. Sending error message if applicable.", senderPhoneNumber)
			if langErr != nil {
				lang := controllers.NewLanguageMsgTranslation(fun.DefaultLang)
				lang.Texts = langErr

				// Check if message came from group
				if v.Info.IsGroup {
					if len(allowedWAG) > 0 {
						groupInfo, groupErr := s.client.GetGroupInfo(context.Background(), v.Info.Chat)
						if groupErr != nil {
							logrus.Errorf("Failed to get group info for %s: %v", v.Info.Chat.String(), groupErr)
							langErrors := make(map[string]string)

							langErrors[fun.LangID] = fmt.Sprintf("🗣 Maaf terjadi kesalahan saat mengambil info grup %s: %v", v.Info.Chat.String(), groupErr)
							langErrors[fun.LangEN] = fmt.Sprintf("🗣 Sorry, an error occurred while fetching group info %s: %v", v.Info.Chat.String(), groupErr)
							langErrors[fun.LangES] = fmt.Sprintf("🗣 Lo sentimos, ocurrió un error al obtener la información del grupo %s: %v", v.Info.Chat.String(), groupErr)
							langErrors[fun.LangFR] = fmt.Sprintf("🗣 Désolé, une erreur s'est produite lors de la récupération des informations du groupe %s : %v", v.Info.Chat.String(), groupErr)
							langErrors[fun.LangDE] = fmt.Sprintf("🗣 Entschuldigung, beim Abrufen der Gruppeninformationen %s ist ein Fehler aufgetreten: %v", v.Info.Chat.String(), groupErr)
							langErrors[fun.LangPT] = fmt.Sprintf("🗣 Desculpe, ocorreu um erro ao obter as informações do grupo %s: %v", v.Info.Chat.String(), groupErr)
							langErrors[fun.LangRU] = fmt.Sprintf("🗣 Извините, произошла ошибка при получении информации о группе %s: %v", v.Info.Chat.String(), groupErr)
							langErrors[fun.LangJP] = fmt.Sprintf("🗣 申し訳ありません、グループ情報 %s を取得中にエラーが発生しました: %v", v.Info.Chat.String(), groupErr)
							langErrors[fun.LangCN] = fmt.Sprintf("🗣 抱歉，获取群组信息 %s 时发生错误：%v", v.Info.Chat.String(), groupErr)
							langErrors[fun.LangAR] = fmt.Sprintf("🗣 عذرًا، حدث خطأ أثناء جلب معلومات المجموعة %s: %v", v.Info.Chat.String(), groupErr)
							lang.Texts = langErrors
						} else if groupInfo != nil {
							groupName := strings.TrimSpace(groupInfo.Name)
							isGroupAllowedToInteract := controllers.ContainsJID(allowedWAG, v.Info.Chat)
							if !isGroupAllowedToInteract {
								logrus.Infof("Group %s (%s) is not allowed to interact", groupName, v.Info.Chat.String())
								return
							}

							if groupName != "" {
								for langCode, text := range langErr {
									langErr[langCode] = fmt.Sprintf("[*%s*] 🗣 %s", groupName, text)
								}
								lang.Texts = langErr
							} else {
								for langCode, text := range langErr {
									langErr[langCode] = fmt.Sprintf("🗣 %s", text)
								}
								lang.Texts = langErr
							}
						} else {
							logrus.Errorf("Group info is nil for %s", v.Info.Chat.String())
							for langCode, text := range langErr {
								langErr[langCode] = fmt.Sprintf("🗣 %s", text)
							}
							lang.Texts = langErr
						}
					}
				}

				if v.Info.IsGroup {
					// Keep send to private chat if from group, prevent spam
					controllers.SendLangWhatsAppTextMsg(
						senderPhoneNumber+"@"+types.DefaultUserServer,
						"",
						nil,
						lang,
						lang.LanguageCode,
						s.client, s.rdb, s.db)
				} else {
					controllers.SendLangWhatsAppTextMsg(
						originalSenderJID,
						stanzaID,
						v,
						lang,
						lang.LanguageCode,
						s.client, s.rdb, s.db)
				}
			}
			shouldProcess = false
		}
	} else {
		// Verification disabled: Try to find user or create dummy
		var waUser model.WAUsers
		if err := s.db.Where("phone_number = ?", senderPhoneNumber).First(&waUser).Error; err == nil {
			userSanitizeResult = &waUser
		}
	}

	if shouldProcess && userSanitizeResult != nil {
		// logrus.Debugf("Processing message from %s. Msg: %s", senderPhoneNumber, lowerMsgText)
		shouldProcessQuota, err := controllers.CheckAndNotifyQuotaLimit(
			userSanitizeResult.ID,
			userSanitizeResult.UseBot,
			originalSenderJID,
			userSanitizeResult.MaxDailyQuota,
			s.client,
			s.rdb,
			s.db,
		)

		if err != nil {
			logrus.Errorf("Failed to check quota limit for %s: %v", originalSenderJID, err)
			return
		}

		if !shouldProcessQuota {
			logrus.Warnf("Quota exceeded for %s", originalSenderJID)
			// Quota exceeded, do not process further
			return
		}

		// Handle file/document messages (non-text)
		if msgType != "text" {
			fileResult := controllers.CheckFilePermission(context.Background(), v, msgType, userSanitizeResult, userLang, s.rdb)
			if !fileResult.Allowed {
				controllers.SendLangWhatsAppTextMsg(originalSenderJID, "", nil, fileResult.Message, userLang, s.client, s.rdb, s.db)
				return
			}

			// Optional: Additional file validation if you have access to file properties
			// You can extract file info from WhatsApp message and validate
			// Example for document:
			if msgType == "document" && v.Message.DocumentMessage != nil {
				doc := v.Message.DocumentMessage
				if doc.FileName != nil && doc.FileLength != nil {
					// Get file permission rule for validation
					fileRules := map[string]controllers.FilePermissionRule{
						"document": {
							MaxFileSizeBytes:  int64(config.GetConfig().Whatsnyan.Files.Document.MaxSize) * 1024 * 1024, // max size in MB converted to bytes
							AllowedExtensions: config.GetConfig().Whatsnyan.Files.Document.AllowedExtensions,            // e.g. []string{".pdf", ".doc", ".docx", ".txt", ".zip"}
							AllowedMimeTypes:  config.GetConfig().Whatsnyan.Files.Document.AllowedMimeTypes,             // e.g. []string{"application/pdf", "application/msword", "application/vnd.openxmlformats-officedocument.wordprocessingml.document", "text/plain", "application/zip"
						},
					}

					rule := fileRules["document"]
					mimeType := ""
					if doc.Mimetype != nil {
						mimeType = *doc.Mimetype
					}

					valid, errMsg := controllers.ValidateFileProperties(*doc.FileName, int64(*doc.FileLength), mimeType, rule, userLang)
					if !valid {
						langMsg := controllers.NewLanguageMsgTranslation(userLang)
						langMsg.Texts[userLang] = errMsg
						controllers.SendLangWhatsAppTextMsg(originalSenderJID, "", nil, langMsg, userLang, s.client, s.rdb, s.db)
						return
					}
				}
			}

			// Process file message
			if msgType == "document" {
				allowed, msg := controllers.SanitizeAndFilterDocument(v, userSanitizeResult, userLang)
				if !allowed {
					controllers.SendLangWhatsAppTextMsg(originalSenderJID, "", nil, msg, userLang, s.client, s.rdb, s.db)
					return
				}
			}
			return
		}

		// Handle text messages
		// logrus.Debugf("CheckPromptPermission result for %s: Allowed=%v", lowerMsgText, result.Allowed)
		result := controllers.CheckPromptPermission(context.Background(), v, lowerMsgText, userSanitizeResult, userLang, s.rdb, s.db)
		if result.Allowed {
			switch {
			case lowerMsgText == "ping":
				controllers.HandlePing(context.Background(), v, stanzaID, originalSenderJID, userLang, s.client, s.rdb, s.db)
				return
			case strings.Contains(lowerMsgText, "get pprof"):
				controllers.HandlePprof(context.Background(), v, stanzaID, originalSenderJID, userLang, s.client, s.rdb, s.db)
				return
			case strings.Contains(lowerMsgText, "get metrics"):
				controllers.HandleMetrics(context.Background(), v, stanzaID, originalSenderJID, userLang, s.client, s.rdb, s.db)
				return
			}
		} else {
			controllers.SendLangWhatsAppTextMsg(originalSenderJID, stanzaID, v, result.Message, userLang, s.client, s.rdb, s.db)
			return
		}
	}

	if shouldProcess {
		controllers.HandleKeywordSearch(context.Background(), v, stanzaID, lowerMsgText, originalSenderJID, userLang, userSanitizeResult, s.client, s.rdb, s.db)
	}

}

// main is the entry point for the WhatsApp microservice.
func main() {
	if err := config.LoadConfig(); err != nil {
		log.Fatalf("Error loading .yaml conf :%v", err)
	}

	go config.WatchConfig()
	cfg := config.GetConfig()

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

	// Initialize client and try to auto-connect
	if err := srv.initializeClient(); err != nil {
		log.Fatal(err)
		logrus.Errorf("Failed to initialize WhatsApp client: %v", err)
	} else if srv.client.Store.ID != nil {
		// If session exists, try to connect
		if err := srv.client.Connect(); err != nil {
			logrus.Errorf("Failed to auto-connect to WhatsApp: %v", err)
		} else {
			logrus.Infof("✅ Auto-connected to WhatsApp as %s", srv.client.Store.ID.User)
		}
	} else if cfg.Whatsnyan.EnablePhonePairing && cfg.Whatsnyan.PairingPhoneNumber != "" {
		// No session exists and phone pairing is enabled - initiate pairing
		logrus.Infof("📱 Phone pairing enabled. Initiating pairing for: %s", cfg.Whatsnyan.PairingPhoneNumber)
		go func() {
			// Wait a moment for server to be ready
			time.Sleep(2 * time.Second)
			resp, err := srv.Connect(context.Background(), &pb.ConnectRequest{
				PhoneNumber: cfg.Whatsnyan.PairingPhoneNumber,
			})
			if err != nil {
				logrus.Errorf("Auto phone pairing failed: %v", err)
			} else if resp.Success {
				logrus.Infof("✅ %s", resp.Message)
				if resp.PairingCode != "" {
					logrus.Infof("🔐 PAIRING CODE: %s", resp.PairingCode)
					logrus.Info("📲 Enter this code in WhatsApp: Settings > Linked Devices > Link a Device > Link with Phone Number")

					// Notify superuser with pairing code
					if suPhoneNumber := config.GetConfig().Default.SuperUserPhone; suPhoneNumber != "" {
						// Wrap in panic recovery
						func() {
							defer func() {
								if r := recover(); r != nil {
									logrus.Warnf("Failed to send pairing code notification: %v", r)
								}
							}()

							jidStr := fmt.Sprintf("%s@%s", suPhoneNumber, types.DefaultUserServer)
							langMsg := controllers.NewLanguageMsgTranslation(fun.DefaultLang)
							langMsg.Texts = map[string]string{
								fun.LangID: fmt.Sprintf("🔐 Kode Pairing WhatsApp: %s\n\nMasukkan kode ini di WhatsApp: Pengaturan > Perangkat Tertaut > Tautkan Perangkat > Tautkan dengan Nomor Telepon", resp.PairingCode),
								fun.LangEN: fmt.Sprintf("🔐 WhatsApp Pairing Code: %s\n\nEnter this code in WhatsApp: Settings > Linked Devices > Link a Device > Link with Phone Number", resp.PairingCode),
								fun.LangES: fmt.Sprintf("🔐 Código de Emparejamiento de WhatsApp: %s\n\nIngresa este código en WhatsApp: Configuración > Dispositivos Vinculados > Vincular Dispositivo > Vincular con Número de Teléfono", resp.PairingCode),
								fun.LangFR: fmt.Sprintf("🔐 Code de Jumelage WhatsApp: %s\n\nEntrez ce code dans WhatsApp: Paramètres > Appareils Liés > Lier un Appareil > Lier avec Numéro de Téléphone", resp.PairingCode),
								fun.LangDE: fmt.Sprintf("🔐 WhatsApp-Kopplungscode: %s\n\nGib diesen Code in WhatsApp ein: Einstellungen > Verknüpfte Geräte > Gerät Verknüpfen > Mit Telefonnummer Verknüpfen", resp.PairingCode),
								fun.LangPT: fmt.Sprintf("🔐 Código de Emparelhamento WhatsApp: %s\n\nDigite este código no WhatsApp: Configurações > Dispositivos Vinculados > Vincular Dispositivo > Vincular com Número de Telefone", resp.PairingCode),
								fun.LangAR: fmt.Sprintf("🔐 رمز الاقتران بواتساب: %s\n\nأدخل هذا الرمز في واتساب: الإعدادات > الأجهزة المرتبطة > ربط جهاز > الربط برقم الهاتف", resp.PairingCode),
								fun.LangJP: fmt.Sprintf("🔐 WhatsAppペアリングコード: %s\n\nこのコードをWhatsAppに入力してください: 設定 > リンク済みデバイス > デバイスをリンク > 電話番号でリンク", resp.PairingCode),
								fun.LangCN: fmt.Sprintf("🔐 WhatsApp配对代码: %s\n\n在WhatsApp中输入此代码: 设置 > 已关联设备 > 关联设备 > 使用电话号码关联", resp.PairingCode),
								fun.LangRU: fmt.Sprintf("🔐 Код сопряжения WhatsApp: %s\n\nВведите этот код в WhatsApp: Настройки > Связанные устройства > Привязать устройство > Привязать по номеру телефона", resp.PairingCode),
							}
							if srv.client != nil && srv.client.IsConnected() && srv.rdb != nil && srv.db != nil {
								controllers.SendLangWhatsAppTextMsg(jidStr, "", nil, langMsg, langMsg.LanguageCode, srv.client, srv.rdb, srv.db)
							}
						}()
					}
				}
			} else {
				logrus.Errorf("Phone pairing failed: %s", resp.Message)
			}
		}()
	} else {
		logrus.Info("📱 No existing session found. Use Connect RPC with QR code or enable phone pairing in config.")
	}

	pb.RegisterWhatsAppServiceServer(s, srv)

	reflection.Register(s)

	// Start metrics server
	go func() {
		http.Handle("/whatsapp-metrics", promhttp.Handler())
		metricsPort := config.GetConfig().Metrics.WhatsAppPort
		logrus.Printf("📊 Metrics server listening on :%d", metricsPort)
		logrus.Fatal(http.ListenAndServe(fmt.Sprintf(":%d", metricsPort), nil))
	}()

	fmt.Printf("📞 Whatsapp gRPC server listening on port %s\n", port)
	// Notify superuser that server is up (only if connected)
	go func() {
		// Wrap entire goroutine in panic recovery
		defer func() {
			if r := recover(); r != nil {
				logrus.Warnf("Startup notification failed: %v", r)
			}
		}()

		// Wait for connection to be established
		for i := 0; i < 30; i++ { // Wait up to 30 seconds
			if srv.client != nil && srv.client.IsConnected() {
				break
			}
			time.Sleep(1 * time.Second)
		}

		suPhoneNumber := config.GetConfig().Default.SuperUserPhone
		if suPhoneNumber != "" && srv.client != nil && srv.client.IsConnected() && srv.rdb != nil && srv.db != nil {
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
		} else if suPhoneNumber != "" {
			logrus.Warn("Could not send startup notification: WhatsApp client not connected")
		}
	}()

	// Handle graceful shutdown
	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt, syscall.SIGTERM)

	go func() {
		<-c
		logrus.Info("🔴 Shutting down WhatsApp gRPC server...")

		// Inform superuser about shutdown (with error recovery)
		suPhoneNumber := config.GetConfig().Default.SuperUserPhone
		if suPhoneNumber != "" && srv.client != nil && srv.client.IsConnected() && srv.rdb != nil && srv.db != nil {
			// Wrap in defer to recover from any panic during shutdown
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
				// Give it a moment to send before killing the server
				time.Sleep(1 * time.Second)
			}()
		}

		s.GracefulStop()
	}()

	if err := s.Serve(lis); err != nil {
		logrus.Fatalf("Failed to serve: %v", err)
	}
}
