package controllers

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"service-platform/cmd/web_panel/config"
	"service-platform/cmd/web_panel/fun"
	"service-platform/cmd/web_panel/logger"
	"service-platform/cmd/web_panel/model"
	whatsappmodel "service-platform/cmd/web_panel/model/whatsapp_model"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/go-redis/redis/v8"
	"github.com/lithammer/fuzzysearch/fuzzy"
	_ "github.com/mattn/go-sqlite3"
	"github.com/mdp/qrterminal"
	"github.com/sirupsen/logrus"
	"go.mau.fi/whatsmeow"
	"go.mau.fi/whatsmeow/proto/waE2E"
	"go.mau.fi/whatsmeow/store/sqlstore"
	"go.mau.fi/whatsmeow/types"
	"go.mau.fi/whatsmeow/types/events"
	"gorm.io/gorm"
)

var (
	WhatsappClient    *whatsmeow.Client
	lastQRGeneratedAt time.Time
	qrCodeMutex       sync.Mutex
	currentQRCode     string

	clientConnecting bool
	clientConnMutex  sync.Mutex

	rdb   *redis.Client
	dbWeb *gorm.DB

	contx = context.Background()

	checkTechnicianExistsInODOOMSMutex sync.Mutex
)

type OllamaRequest struct {
	Model  string `json:"model"`
	Prompt string `json:"prompt"`
	Stream bool   `json:"stream"`
}

type OllamaResponse struct {
	Response string `json:"response"`
}

// Sanitize user's prompt for BOT
type SanitizationResult struct {
	User *model.WAPhoneUser
}

// ProcessMessage handles incoming WhatsApp messages and processes them according to the following logic:
//   - Ignores messages sent by the bot itself.
//   - Normalizes the sender JID to ensure it has the correct format.
//   - Extracts the message text, handling both conversation and extended text messages.
//   - Ignores empty or non-text messages.
//   - Handles group messages if the group is in the allowed list (currently placeholder).
//   - Responds to specific commands such as "ping" and "/form-request".
//   - Processes form requests if the message starts with "[REQUEST".
//   - Allows users to set their language preference with "en" or "id", stores it in Redis, and confirms the change.
//   - Prompts the user to select a language if not already set.
//   - Fetches language-specific bot replies from the database and matches user messages to keywords.
//   - Suggests the closest matching keyword if no exact match is found.
//   - Provides default replies or escalates to customer service if the message is not understood.
//   - Sends the appropriate reply back to the user via WhatsApp.
//
// The function is designed to be extensible for additional message handling and integration with external systems.
func ProcessMessage(v *events.Message) {
	if v.Info.MessageSource.IsFromMe {
		return
	}

	redisExpiry := time.Duration(config.GetConfig().Whatsmeow.RedisExpiry) * time.Hour

	originalSenderJID := NormalizeSenderJID(v.Info.Sender.String())
	stanzaID := v.Info.ID
	messageText := extractMessageText(v)
	messageTextLower := strings.ToLower(strings.TrimSpace(messageText))

	// Extract phone number from the JID properly
	senderPhoneNumber := v.Info.Sender.User

	// Debug logging to see what we're getting
	logrus.Debugf("Sender info: Full=%s, User=%s, Server=%s",
		v.Info.Sender.String(),
		v.Info.Sender.User,
		v.Info.Sender.Server)

	// Try to find user by JID first if senderPhoneNumber looks like an internal ID
	if len(senderPhoneNumber) > 12 && !strings.HasPrefix(senderPhoneNumber, "62") && !strings.HasPrefix(senderPhoneNumber, "0") {
		logrus.Debugf("Looks like internal WhatsApp ID, trying to find user by JID: %s", originalSenderJID)

		// For linked devices (@lid), try different approaches
		if v.Info.Sender.Server == "lid" {
			logrus.Debugf("This is a linked device (@lid), trying alternative methods")

			// Method 1: Try to find in whatsmeow_lid_map table in whatsmeow.db
			whatsmeowDB, err := WhatsmeowDBSQliteConnect()
			if err != nil {
				logrus.Errorf("Failed to connect to Whatsmeow SQLite DB: %v", err)
			}

			if whatsmeowDB != nil {
				phoneNumberFromWhatsmeow, err := GetPhoneNumberFromLID(whatsmeowDB, originalSenderJID)
				if err == nil && phoneNumberFromWhatsmeow != "" {
					senderPhoneNumber = phoneNumberFromWhatsmeow
				} else {
					logrus.Warnf("Could not find phone number in whatsmeow_lid_map for JID %s: %v", originalSenderJID, err)
				}

				logrus.Debugf("Found sender phone number from @lid: %s", phoneNumberFromWhatsmeow)
			}

			// Method 2: Try to find in message history by chat_jid
			var message model.WAMessage
			err = dbWeb.Where("chat_jid LIKE ?", "%"+senderPhoneNumber+"%").First(&message).Error
			if err == nil && message.ChatJID != "" {
				// Extract phone from chat_jid if it contains a phone number
				extractedPhone := extractPhoneFromJID(message.ChatJID)
				if strings.HasPrefix(extractedPhone, "62") || strings.HasPrefix(extractedPhone, "0") {
					logrus.Debugf("Found phone from message history: %s", extractedPhone)
					senderPhoneNumber = extractedPhone
				}
			}

		} else {
			// Try to find in WAConversation table first
			var conversation whatsappmodel.WAConversation
			err := dbWeb.Where("contact_jid = ? OR contact_jid LIKE ?", originalSenderJID, "%"+senderPhoneNumber+"%").First(&conversation).Error
			if err == nil && conversation.ContactPhone != "" {
				logrus.Debugf("Found conversation with phone: %s", conversation.ContactPhone)
				senderPhoneNumber = conversation.ContactPhone
			} else {
				// Try WAContactInfo table
				var contact whatsappmodel.WAContactInfo
				err = dbWeb.Where("contact_jid = ? OR contact_jid LIKE ?", originalSenderJID, "%"+senderPhoneNumber+"%").First(&contact).Error
				if err == nil && contact.PhoneNumber != "" {
					logrus.Debugf("Found contact with phone: %s", contact.PhoneNumber)
					senderPhoneNumber = contact.PhoneNumber
				} else {
					logrus.Warnf("Could not find user by JID in any table: %v", err)
				}
			}
		}
	}

	// Additional debug: try to get contact info to find the real phone number
	if WhatsappClient != nil {
		contact, err := WhatsappClient.Store.Contacts.GetContact(context.Background(), v.Info.Sender)
		if err == nil {
			logrus.Debugf("Contact info: %+v", contact)
			// If contact has a valid phone number, use it
			if contact.PushName != "" {
				logrus.Debugf("Contact PushName: %s", contact.PushName)
			}
		}

		// For linked devices, try to resolve the actual user JID
		if v.Info.Sender.Server == "lid" {
			logrus.Debugf("Attempting to resolve linked device to actual phone number")

			// Try a different approach: check if we can get chat info
			chatJID := v.Info.Chat
			logrus.Debugf("Chat JID: %s", chatJID.String())

			// For direct messages, the chat JID should be the actual phone number
			if !v.Info.IsGroup {
				actualPhone := extractPhoneFromJID(chatJID.String())
				if actualPhone != senderPhoneNumber && (strings.HasPrefix(actualPhone, "62") || strings.HasPrefix(actualPhone, "0")) {
					logrus.Debugf("Using chat JID phone instead: %s", actualPhone)
					senderPhoneNumber = actualPhone
				}
			} else {
				// This is a group message from a linked device
				logrus.Debugf("Group message from linked device, trying to resolve participant")

				// Try to find in group participant records
				var participant whatsappmodel.WAGroupParticipant
				err := dbWeb.Where("participant_jid LIKE ? OR participant_jid = ?", "%"+senderPhoneNumber+"%", originalSenderJID).First(&participant).Error
				if err == nil {
					logrus.Debugf("Found group participant: %s", participant.ParticipantJID)
					// Try to extract phone from participant JID
					participantPhone := extractPhoneFromJID(participant.ParticipantJID)
					if participantPhone != senderPhoneNumber && (strings.HasPrefix(participantPhone, "62") || strings.HasPrefix(participantPhone, "0")) {
						logrus.Debugf("Using participant phone: %s", participantPhone)
						senderPhoneNumber = participantPhone
					}
				} else {
					logrus.Warnf("Could not find group participant: %v", err)
				}
			}
		}
	}

	// Final fallback: if we still have an internal ID and it's from a linked device,
	// #######################################################
	logrus.Debugf("(1) SenderPhoneNumber: %s", senderPhoneNumber)
	// #######################################################
	// try to use it as-is but mark it specially
	if len(senderPhoneNumber) > 12 && !strings.HasPrefix(senderPhoneNumber, "62") && !strings.HasPrefix(senderPhoneNumber, "0") && v.Info.Sender.Server == "lid" {
		logrus.Warnf("Could not resolve linked device ID to phone number, using original sender JID for lookup: %s", originalSenderJID)
		// Use the original JID for validation instead of trying to validate as phone number
		senderPhoneNumber = originalSenderJID
	}

	// #######################################################
	logrus.Debugf("(2) SenderPhoneNumber: %s", senderPhoneNumber)
	logrus.Debugf("OriginalJID: %s", originalSenderJID)
	// #######################################################

	if messageTextLower == "" {
		return
	}

	// Get user language (early)
	userLang, err := GetUserLang(originalSenderJID)
	if err != nil {
		logrus.Errorf("Failed to get user lang: %v", err)
		userLang = "en"
	}

	// Set language if "en" or "id"
	if messageTextLower == "en" || messageTextLower == "id" {
		handleLanguageChange(originalSenderJID, messageTextLower)
		return
	}

	// ADD: if you need to send the interactive button response handling
	// // Handle interactive button responses first
	// if v.Message.ButtonsResponseMessage != nil && v.Message.ButtonsResponseMessage.SelectedButtonID != nil {
	// 	buttonID := *v.Message.ButtonsResponseMessage.SelectedButtonID
	// 	HandleInteractiveButtonResponse(buttonID, originalSenderJID)
	// 	return
	// }

	// // Handle list response messages
	// if v.Message.ListResponseMessage != nil && v.Message.ListResponseMessage.SingleSelectReply != nil {
	// 	selectedRowID := v.Message.ListResponseMessage.SingleSelectReply.SelectedRowID
	// 	if selectedRowID != nil {
	// 		HandleInteractiveButtonResponse(*selectedRowID, originalSenderJID)
	// 		return
	// 	}
	// }

	// Detect message type: for example, "text", "image", etc.
	// You could parse v.Message to decide real type; for now we assume text:
	msgType := "text"
	if v.Message.ImageMessage != nil {
		msgType = "image"
	}
	if v.Message.VideoMessage != nil {
		msgType = "video"
	}
	if v.Message.DocumentMessage != nil {
		msgType = "document"
	}
	if v.Message.AudioMessage != nil {
		msgType = "audio"
	}
	// if v.Message.StickerMessage != nil {
	// 	msgType = "sticker"
	// }

	// Validate user
	sanitizeRes, err := ValidateWhatsappBOTPhoneUser(senderPhoneNumber, originalSenderJID, v.Info.IsGroup, msgType)
	if err != nil {
		logrus.Warnf("Blocked by validation: %v", err)

		// Check if number not registered (ID & EN message from config)
		if err.Error() == config.GetConfig().Whatsmeow.WaErrorMessage.ID.PhoneNumberNotRegistered ||
			err.Error() == config.GetConfig().Whatsmeow.WaErrorMessage.EN.PhoneNumberNotRegistered {

			notRegKey := "not_registered_" + originalSenderJID
			exists, err := rdb.Exists(contx, notRegKey).Result()
			if err != nil {
				logrus.Error(err)
			}

			if exists == 0 {
				idMsg := fmt.Sprintf(
					"Nomor Anda _(%s)_ belum terdaftar.\n"+
						"Silakan hubungi Admin untuk melakukan pendaftaran agar dapat menggunakan layanan ini.\n\n"+
						"Terima kasih atas pengertiannya. 😊\n"+
						"📞 Technical Support *@+%s*",
					senderPhoneNumber,
					config.GetConfig().Whatsmeow.WaTechnicalSupport,
				)
				enMsg := fmt.Sprintf(
					"Your number _(%s)_ is not registered yet.\n"+
						"Please contact the Admin to register your number so you can use this service.\n\n"+
						"Thank you for your understanding. 😊\n"+
						"📞 Technical Support *@+%s*",
					senderPhoneNumber,
					config.GetConfig().Whatsmeow.WaTechnicalSupport,
				)

				SendLangMessage(originalSenderJID, idMsg, enMsg, userLang)
				rdb.Set(contx, notRegKey, "true", redisExpiry)
			}
		} else {
			// Check if message came from group
			if v.Info.IsGroup {
				waGroup := config.GetConfig().Whatsmeow.WaGroupAllowedToUsePrompt

				if len(waGroup) > 0 {
					senderPhoneNumberJID := senderPhoneNumber + "@s.whatsapp.net"
					if strings.Contains(senderPhoneNumber, "@") {
						senderPhoneNumberJID = senderPhoneNumber
					}

					// Use different name to avoid shadowing original err
					groupInfo, groupErr := WhatsappClient.GetGroupInfo(context.Background(), v.Info.Chat)
					if groupErr != nil {
						logrus.Errorf("failed to get group info: %v", groupErr)
						// fallback: just send original err
						// sendTextMessageViaBot(originalSenderJID, fmt.Sprintf("🗣 %s", err.Error()))
						sendTextMessageViaBot(senderPhoneNumberJID, fmt.Sprintf("🗣 %s", err.Error()))
					} else if groupInfo != nil {
						groupName := strings.TrimSpace(groupInfo.Name)
						isGroupIsAllowed := containsJID(waGroup, v.Info.Chat)
						if !isGroupIsAllowed {
							// Group not allowed to use BOT
							logrus.Warnf("Group %s (%s) is not allowed to use BOT", groupName, v.Info.Chat.String())
							return
						}

						if groupName != "" {
							errMsg := fmt.Sprintf("*[%s]* 🗣 %s", groupName, err.Error())
							// sendTextMessageViaBot(originalSenderJID, errMsg)
							sendTextMessageViaBot(senderPhoneNumberJID, errMsg)
						} else {
							// group has no name
							// sendTextMessageViaBot(originalSenderJID, fmt.Sprintf("🗣 %s", err.Error()))
							sendTextMessageViaBot(senderPhoneNumberJID, fmt.Sprintf("🗣 %s", err.Error()))
						}
					} else {
						// unexpected: groupInfo nil
						logrus.Warn("groupInfo is nil")
						sendTextMessageViaBot(originalSenderJID, fmt.Sprintf("🗣 %s", err.Error()))
					}
				}
			} else {
				// private chat
				sendTextMessageViaBot(originalSenderJID, err.Error())
			}
		}
		return
	}

	shouldProcess, err := CheckAndNotifyQuota(
		sanitizeRes.User.ID,
		userLang,
		originalSenderJID,
		sanitizeRes.User.MaxDailyQuota,
	)
	if err != nil {
		logrus.Warnf("Quota check failed: %v", err)
		return
	}
	if !shouldProcess {
		// Over quota: stop processing
		return
	}

	// Handle file/document messages (non-text)
	if msgType != "text" {
		fileResult := CheckFilePermission(v, msgType, sanitizeRes.User, userLang)
		if !fileResult.Allowed {
			sendTextMessageViaBot(originalSenderJID, fileResult.Message)
			return
		}

		// Optional: Additional file validation if you have access to file properties
		// You can extract file info from WhatsApp message and validate
		// Example for document:
		if msgType == "document" && v.Message.DocumentMessage != nil {
			doc := v.Message.DocumentMessage
			if doc.FileName != nil && doc.FileLength != nil {
				// Get file permission rule for validation
				fileRules := map[string]FilePermissionRule{
					"document": {
						MaxFileSizeBytes:  config.GetConfig().Whatsmeow.MaxUploadedDocumentSize * 1024 * 1024, // max size in MB converted to bytes
						AllowedExtensions: config.GetConfig().Whatsmeow.DocumentAllowedExtensions,             // e.g. []string{".pdf", ".doc", ".docx", ".txt", ".zip"}
						AllowedMimeTypes:  config.GetConfig().Whatsmeow.DocumentAllowedMimeTypes,              // e.g. []string{"application/pdf", "application/msword", "application/vnd.openxmlformats-officedocument.wordprocessingml.document", "text/plain", "application/zip"
					},
				}

				rule := fileRules["document"]
				mimeType := ""
				if doc.Mimetype != nil {
					mimeType = *doc.Mimetype
				}

				valid, errMsg := ValidateFileProperties(*doc.FileName, int64(*doc.FileLength), mimeType, rule, userLang)
				if !valid {
					sendTextMessageViaBot(originalSenderJID, errMsg)
					return
				}
			}
		}

		// Process file message - you can add your file handling logic here
		// handleFileMessage(v, msgType, originalSenderJID, userLang, sanitizeRes.User)
		return
	}

	result := CheckPromptPermission(v, messageText, sanitizeRes.User, userLang)
	if result.Allowed {
		switch messageTextLower {
		case "ping":
			whatsmeowPing(v, stanzaID, originalSenderJID)
		case "/form-request":
			CCFormRequestTemplate(v, stanzaID, originalSenderJID)
		case "/cs":
			handleCSCommand(originalSenderJID, userLang)
		case "/logout-cs":
			handleLogoutCSCommand(originalSenderJID, userLang)
		case "report mr oliver":
			CheckMrOliverReportAvailability(v, userLang)
		case "generate report ta":
			ReportTA(v, userLang)
		case "generate report compared":
			// ReportCompared(v, userLang)
			ReportComparedGenerated(v, userLang)
		case "generate report tech error":
			ReportTechError(v, userLang)
		case "generate report ai error":
			ReportAIError(v, userLang)
		case "show status vm odoo dashboard":
			ShowStatusVMODOODashboard(v, userLang)
		case "restart mysql vm odoo dashboard":
			RestartMySQLVMODOODashboard(v, userLang)
		case "report so":
			GetReportOfStockOpname(v, userLang)
		case "/all-cmd":
			AllCMDWhatsapp(v, userLang)
		case "active ai":
			ActiveAIRafy(v, userLang)
		case "deactivate ai":
			DeactivateAIRafy(v, userLang)
		}
	} else {
		if result.Message != "" {
			sendTextMessageViaBot(originalSenderJID, result.Message)
		}
	}

	// Group handling placeholder
	waGroup := config.GetConfig().Whatsmeow.WaGroupAllowedToUsePrompt
	if v.Info.IsGroup && containsJID(waGroup, v.Info.Chat) {
		switch messageTextLower {
		case "ping pong ping!!":
			whatsmeowPing(v, stanzaID, originalSenderJID)
			return
		case "halo bot!":
			haloFromBot(v, stanzaID, originalSenderJID)
			return
			// Add more prompt ...
		}
	}

	// Forward if user is in CS session
	isCSActive, _ := IsCSSessionActive(originalSenderJID)
	if isCSActive {
		forwardMessageToCS(v, originalSenderJID, messageText)
		return
	}

	// Prompt language if not set
	if userLang == "" {
		langPromptKey := "lang_prompted_" + originalSenderJID
		exists, err := rdb.Exists(contx, langPromptKey).Result()
		if err != nil {
			logrus.Error(err)
		}

		if exists == 0 {
			langPrompt := config.GetConfig().Whatsmeow.InitLanguagePrompt
			sendTextMessageViaBot(originalSenderJID, langPrompt)
			// rdb.Set(contx, langPromptKey, "true", redisExpiry)
			rdb.Set(contx, langPromptKey, "true", 24*2*time.Hour) // 2 days
		}
		return
	}

	handleKeywordReply(originalSenderJID, messageText, messageTextLower, userLang, sanitizeRes)

}

// StartWhatsappClient initializes and starts a WhatsApp client using the provided Redis and GORM database clients.
// It sets up logging for both the database and client, creates a SQL store for WhatsApp device data,
// retrieves the first available device, and creates a new WhatsApp client instance.
// The function also registers an event handler for WhatsApp events.
// Returns the initialized WhatsApp client or an error if setup fails.
//
// Parameters:
//   - redisDB: a pointer to a Redis client instance.
//   - db: a pointer to a GORM database instance.
//
// Returns:
//   - *whatsmeow.Client: the initialized WhatsApp client.
//   - error: an error if initialization fails.
func StartWhatsappClient(redisDB *redis.Client, db *gorm.DB) (*whatsmeow.Client, error) {
	rdb = redisDB
	dbWeb = db

	clientLogPath := config.GetConfig().Whatsmeow.WhatsmeowClientLog
	dbLogPath := config.GetConfig().Whatsmeow.WhatsmeowDBLog
	if clientLogPath == "" {
		return nil, errors.New("whatsmeow client log path is not defined")
	}
	if dbLogPath == "" {
		return nil, errors.New("whatsmeow db log path is not defined")
	}

	dbLevelStr := config.GetConfig().Whatsmeow.WhatsmeowDBLogLevel
	clientLevelStr := config.GetConfig().Whatsmeow.WhatsmeowClientLogLevel

	dbLevel := logger.ParseWhatsmeowLogLevel(dbLevelStr)
	clientLevel := logger.ParseWhatsmeowLogLevel(clientLevelStr)

	dbLog := logger.NewWhatsmeowLogger("Database", dbLogPath, dbLevel)
	clientLog := logger.NewWhatsmeowLogger("Client", clientLogPath, clientLevel)

	// Original logger
	// dbLog := waLog.Stdout("Database", "ERROR", true)   // DEBUG | ERROR | WARN | INFO
	// clientLog := waLog.Stdout("Client", "ERROR", true) // DEBUG | ERROR | WARN | INFO

	container, err := sqlstore.New(
		context.Background(),
		config.GetConfig().Whatsmeow.SqlDriver,
		fmt.Sprintf("file:%s?_foreign_keys=on", config.GetConfig().Whatsmeow.SqlSource),
		dbLog,
	)
	if err != nil {
		return nil, err
	}

	deviceStore, err := container.GetFirstDevice(context.Background())
	if err != nil {
		return nil, err
	}

	client := whatsmeow.NewClient(deviceStore, clientLog)
	WhatsappClient = client

	client.AddEventHandler(HandleWhatsappEvent)

	return client, nil
}

// saveQRCodeToFile saves the provided QR code string to a file specified in the configuration.
// The QR code is written as plain text. If an error occurs during the file write operation,
// an error message is printed to the console. Optionally, the function can be extended to
// save the QR code as an image file by using the commented-out code.
func saveQRCodeToFile(code string) {
	filePath := config.GetConfig().Whatsmeow.QrCode
	err := os.WriteFile(filePath, []byte(code), 0644)
	if err != nil {
		fmt.Printf("Failed to save QR code: %v\n", err)
	}
	// Uncomment the following lines if you want to save the QR code as an image file
	// file, err := os.Create(filePath)
	// if err != nil {
	// 	fmt.Printf("Failed to create QR code file: %v\n", err)
	// 	return
	// }
	// defer file.Close()
	// qrterminal.GenerateHalfBlock(code, qrterminal.L, file)
	// fmt.Println("QR Code saved to:", filePath)
}

// This goroutine handles connection and QR code updates
// startConnectAndListenQRCode initializes the WhatsApp client connection and listens for QR code events.
// It manages the QR code lifecycle, including generating, displaying, saving, and clearing the QR code
// based on events received from the WhatsApp client. The function ensures thread-safe access to the
// current QR code and its generation timestamp. It also handles connection errors and updates the
// client connection state accordingly.
func startConnectAndListenQRCode() {
	if WhatsappClient == nil {
		return
	}

	setClientConnecting(true)
	defer setClientConnecting(false)

	qrChan, _ := WhatsappClient.GetQRChannel(context.Background())
	err := WhatsappClient.Connect()
	if err != nil {
		fmt.Printf("Failed to connect: %v\n", err)
		return
	}

	// Listen for QR code events asynchronously
	for evt := range qrChan {
		switch evt.Event {
		case "code":
			qrCodeMutex.Lock()
			currentQRCode = evt.Code
			lastQRGeneratedAt = time.Now()
			qrCodeMutex.Unlock()

			fmt.Println("New QR Code generated:")
			qrterminal.GenerateHalfBlock(evt.Code, qrterminal.L, os.Stdout)
			saveQRCodeToFile(evt.Code)

		case "success":
			botNumber := WhatsappClient.Store.ID.User
			fmt.Printf("✅ WhatsApp logged in successfully as +%s\n", botNumber)
			// Clear QR code after success
			qrCodeMutex.Lock()
			currentQRCode = ""
			lastQRGeneratedAt = time.Time{}
			qrCodeMutex.Unlock()
		case "timeout", "error", "done":
			fmt.Printf("QR code event: %s\n", evt.Event)
			// Optionally clear QR code on timeout/error
			qrCodeMutex.Lock()
			currentQRCode = ""
			lastQRGeneratedAt = time.Time{}
			qrCodeMutex.Unlock()
		}
	}
}

// RefreshWhatsappQrcode returns a Gin handler function that manages the WhatsApp QR code refresh process.
// It checks the current connection status of the WhatsApp client and the validity of the cached QR code.
// - If the client is not initialized, it attempts to start it.
// - If already connected and logged in, it responds that no QR code is needed.
// - If a valid QR code is cached, it returns the cached QR code.
// - If not connected and not connecting, it starts the connection and QR code generation in the background.
// - If a connection is in progress but the QR code is not yet available or expired, it instructs the client to wait and refresh.
// The handler responds with appropriate JSON messages for each scenario.
func RefreshWhatsappQrcode() gin.HandlerFunc {
	return func(c *gin.Context) {
		qrCodeMutex.Lock()
		valid := time.Since(lastQRGeneratedAt) < time.Duration(config.GetConfig().Whatsmeow.QrExpired)*time.Second
		qrCode := currentQRCode
		qrCodeMutex.Unlock()

		if WhatsappClient == nil {
			_, err := StartWhatsappClient(rdb, dbWeb)
			if err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"message": "failed to start WhatsApp client", "detail": err.Error()})
				return
			}
		}

		// If connected and logged in, no QR needed
		if WhatsappClient.IsConnected() && WhatsappClient.IsLoggedIn() {
			c.JSON(http.StatusOK, gin.H{"message": "already connected to WhatsApp"})
			return
		}

		// If QR code is still valid, just return cached one
		if valid && qrCode != "" {
			c.JSON(http.StatusOK, gin.H{"qrcode": qrCode, "message": "QR code still valid. Please scan it."})
			return
		}

		// If not connected or not connecting, start connection & QR generation in background
		if !WhatsappClient.IsConnected() && !isClientConnecting() {
			go startConnectAndListenQRCode()
			c.JSON(http.StatusOK, gin.H{"message": "starting connection, generating new QR code. Please wait a moment and refresh."})
			return
		}

		// If connection ongoing but QR not yet generated or expired, respond accordingly
		c.JSON(http.StatusOK, gin.H{"message": "connection in progress, please wait and refresh to get new QR code."})
	}
}

// EndSessionWhatsapp returns a Gin handler function that terminates the current WhatsApp session.
// The handler performs the following actions:
//  1. Checks if the WhatsApp client is initialized and connected.
//  2. Disconnects the WhatsApp client.
//  3. Clears QR code tracking variables and removes any stored QR code file.
//  4. Deletes the WhatsApp session from the store.
//  5. Optionally deletes the session SQLite file if it exists and is not in-memory.
//  6. Resets the WhatsApp client reference to nil.
//
// Responds with a JSON message indicating success or failure at each step.
func EndSessionWhatsapp() gin.HandlerFunc {
	return func(c *gin.Context) {
		if WhatsappClient == nil {
			c.JSON(http.StatusInternalServerError, gin.H{"message": "WhatsApp client not initialized"})
			return
		}

		if !WhatsappClient.IsConnected() {
			c.JSON(http.StatusInternalServerError, gin.H{"message": "WhatsApp client not connected"})
			return
		}

		ctx := context.Background()

		// 1. Disconnect and Logout
		WhatsappClient.Disconnect()

		// if err := WhatsappClient.Logout(ctx); err != nil {
		// 	c.JSON(http.StatusInternalServerError, gin.H{
		// 		"message": "Failed to logout WhatsApp client",
		// 		"detail":  err.Error(),
		// 	})
		// 	return
		// }

		// 4. Clear QR code tracking vars
		qrCodeMutex.Lock()
		currentQRCode = ""
		lastQRGeneratedAt = time.Time{}
		qrCodeMutex.Unlock()
		saveQRCodeToFile("")

		// 2. Delete session from store
		if err := WhatsappClient.Store.Delete(ctx); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{
				"message": "Failed to delete WhatsApp session from store",
				"detail":  err.Error(),
			})
			return
		}

		// 3. Optional: Delete the session SQLite file (if used)
		sessionPath := config.GetConfig().Whatsmeow.SqlSource
		if sessionPath != ":memory:" {
			if err := os.Remove(sessionPath); err != nil && !os.IsNotExist(err) {
				logrus.Errorf("Failed to remove WhatsApp session file: %v", err)
				c.JSON(http.StatusInternalServerError, gin.H{
					"message": "Failed to remove WhatsApp session file",
					"detail":  err.Error(),
				})
				return
			}
		}

		// 5. Reset the client reference
		WhatsappClient = nil

		c.JSON(http.StatusOK, gin.H{"message": "WhatsApp session ended successfully @" + time.Now().Format("15:04:05, 02 Jan 2006") + ". Please refresh the QR code."})
	}
}

// ValidateGroupJID validates and normalizes a WhatsApp group JID (ID), ensuring it is in the correct format,
// and checks if the bot is a participant of the group. It removes common suffixes, normalizes the input,
// and fetches group information using the WhatsappClient. If the group exists and the bot is a member,
// it returns the normalized JID; otherwise, it returns an error describing the issue.
//
// Parameters:
//   - groupJID: The group JID string to validate and normalize.
//
// Returns:
//   - types.JID: The normalized group JID if valid and the bot is a participant.
//   - error: An error if the JID is invalid, the group does not exist, or the bot is not a member.
func ValidateGroupJID(groupJID string) (types.JID, error) {
	// Normalize: remove spaces, lowercase
	groupJID = strings.TrimSpace(strings.ToLower(groupJID))

	// Remove any "@g.us" or "g.us" suffix if present
	groupJID = strings.TrimSuffix(groupJID, "@g.us")
	groupJID = strings.TrimSuffix(groupJID, "g.us")
	groupJID = strings.TrimSuffix(groupJID, "@") // Remove trailing @ if any

	if groupJID == "" {
		return types.JID{}, errors.New("group JID cannot be empty")
	}

	jid := types.NewJID(groupJID, "g.us")

	// Check if group exists and bot is a participant
	groupInfo, err := WhatsappClient.GetGroupInfo(context.Background(), jid)
	if err != nil {
		return types.JID{}, fmt.Errorf("failed to fetch group info: %v", err)
	}

	// Ensure the bot is part of the group
	me := WhatsappClient.Store.ID.User
	isMember := false
	for _, p := range groupInfo.Participants {
		if p.JID.User == me {
			isMember = true
			break
		}
	}

	if !isMember {
		botNumber := me
		if strings.HasPrefix(botNumber, "62") {
			botNumber = "+" + botNumber
		}
		return types.JID{}, fmt.Errorf("your bot number (%s) is not a member of group %s", botNumber, groupInfo.Name)
	}

	return jid, nil
}

// suggestClosestKeyword takes an input string and a slice of all possible keywords,
// then returns the closest matching keyword based on fuzzy string matching.
// The function normalizes the input by trimming spaces and converting to lowercase,
// then uses fuzzy ranking to find the best match. If no match is found, it returns an empty string.
//
// Parameters:
//   - input: The input string to match against the keywords.
//   - allKeywords: A slice of strings containing all possible keywords.
//
// Returns:
//   - The closest matching keyword as a string, or an empty string if no match is found.
func suggestClosestKeyword(input string, allKeywords []string) string {
	inputLower := strings.ToLower(strings.TrimSpace(input))
	matches := fuzzy.RankFindNormalizedFold(inputLower, allKeywords)

	if len(matches) == 0 {
		return ""
	}

	// Return the best ranked match (highest similarity)
	bestMatch := matches[0]
	return bestMatch.Target
}

// CCFormRequestTemplate sends a form request template message to a WhatsApp user based on their language preference.
// It first checks the user's language setting. If not set, it prompts the user to select a language.
// Then, it sends an instructional message followed by a sample request form in the appropriate language.
// The function also supports quoting the original message for context.
// Parameters:
//   - v: Pointer to the incoming WhatsApp message event.
//   - stanzaID: The stanza ID of the message to be quoted.
//   - originalSenderJID: The JID of the original sender to whom the template will be sent.
func CCFormRequestTemplate(v *events.Message, stanzaID, originalSenderJID string) {
	taskDoing := "Send Form Request Template"
	quotedMsg := &waE2E.ContextInfo{
		StanzaID:      &stanzaID,
		Participant:   &originalSenderJID,
		QuotedMessage: v.Message,
	}

	userLang, err := GetUserLang(originalSenderJID)
	if err != nil {
		logrus.Errorf("failed to get user lang: %v", err)
		return
	}

	if userLang == "" {
		langPrompt := config.GetConfig().Whatsmeow.InitLanguagePrompt
		sendTextMessageViaBot(originalSenderJID, langPrompt)
		return
	}

	var textToSend string
	switch userLang {
	case "id":
		textToSend = "Berikut contoh form request yang dapat digunakan.\nSilahkan salin dan lengkapi datanya sesuai kebutuhan Anda. 🙂"
	case "en":
		textToSend = "Here is a sample request form you can use.\nPlease copy and complete the data as needed. 🙂"
	}

	_, err = WhatsappClient.SendMessage(context.Background(), v.Info.Chat, &waE2E.Message{
		ExtendedTextMessage: &waE2E.ExtendedTextMessage{
			Text:        &textToSend,
			ContextInfo: quotedMsg,
		},
	})
	if err != nil {
		logrus.Errorf("%s: %v\n", taskDoing, err)
		return
	}

	var sb strings.Builder
	switch userLang {
	case "id":
		sb.WriteString("[REQUEST]\n")
		sb.WriteString("Nama: Nama Anda\n")
		sb.WriteString("Alamat: JL. KARAN SITRAINI 33. CIREBON\n")
		sb.WriteString("SN EDC: 1171017021\n")
		sb.WriteString("Kendala: .......\n")
	case "en":
		sb.WriteString("[REQUEST]\n")
		sb.WriteString("Name: Your Name\n")
		sb.WriteString("Address: Street ...\n")
		sb.WriteString("EDC SN: SN-12345\n")
		sb.WriteString("Issue: EDC failed to process transaction\n")
	}
	textToSend = sb.String()

	_, err = WhatsappClient.SendMessage(context.Background(), v.Info.Chat, &waE2E.Message{
		ExtendedTextMessage: &waE2E.ExtendedTextMessage{
			Text: &textToSend,
			// ContextInfo: quotedMsg,
		},
	})
	if err != nil {
		logrus.Errorf("%s: %v\n", taskDoing, err)
		return
	}

	// // Insert whatsapp msg to DB
	// go func() {
	// 	msg := model.WAMessage{
	// 		ID:          resp.ID,
	// 		ChatJID:     v.Info.Chat.String(),
	// 		SenderJID:   resp.Sender.User + "@" + resp.Sender.Server,
	// 		MessageBody: textToSend,
	// 		MessageType: "text",
	// 		IsGroup:     v.Info.Chat.Server == "g.us",
	// 		Status:      "sent",
	// 		SentAt:      resp.Timestamp,
	// 	}
	// 	if err := dbWeb.Create(&msg).Error; err != nil {
	// 		logrus.Errorf("error while trying to insert msg of %v: %v", taskDoing, err)
	// 	}
	// }()
}

// ReplyRequestTemplate processes a WhatsApp message containing a request template,
// validates the input, checks for required fields, and creates a new ticket in the database
// if the request is valid and not a duplicate. It also sends appropriate feedback messages
// to the user based on their language preference and the validity of the request data.
//
// Parameters:
//   - v: Pointer to the incoming WhatsApp message event.
//   - stanzaID: The stanza ID of the message to be quoted in replies.
//   - originalSenderJID: The JID (Jabber ID) of the original sender.
//   - lines: The lines of the request message, typically split by newline.
//
// Behavior:
//   - Determines the user's language and prompts for language selection if not set.
//   - Validates the request format and required fields based on the user's language.
//   - Sends error messages if the request is invalid or missing required data.
//   - Creates a new ticket in the database if the request is valid and not a duplicate.
//   - Sends a confirmation message to the user upon successful ticket creation.
//   - Triggers an asynchronous process to insert the ticket data into Odoo.
//
// Note: This function assumes the existence of several global variables and helper functions,
// such as WhatsappClient, dbWeb, config, fun.GenerateRandomString, and TriggerInsertDatatoODOO.
func ReplyRequestTemplate(v *events.Message, stanzaID, originalSenderJID string, lines []string) {
	userLang, err := GetUserLang(originalSenderJID)
	if err != nil {
		logrus.Errorf("failed to get user lang: %v", err)
		return
	}

	if userLang == "" {
		langPrompt := config.GetConfig().Whatsmeow.InitLanguagePrompt
		sendTextMessageViaBot(originalSenderJID, langPrompt)
		return
	}

	if len(lines) < 5 {
		quotedMsg := &waE2E.ContextInfo{
			StanzaID:      &stanzaID,
			Participant:   &originalSenderJID,
			QuotedMessage: v.Message,
		}

		var textToSend string
		switch userLang {
		case "id":
			textToSend = "Data request invalid! Mohon lengkapi data request sesuai template yang tersedia. Untuk contoh template, Anda bisa ketik *_/form-request_*"
		case "en":
			textToSend = "Invalid request data! Please complete your request according to the available template. For a sample template, you can type *_/form-request_*"
		}

		_, err := WhatsappClient.SendMessage(context.Background(), v.Info.Chat, &waE2E.Message{
			ExtendedTextMessage: &waE2E.ExtendedTextMessage{
				Text:        &textToSend,
				ContextInfo: quotedMsg,
			},
		})

		if err != nil {
			logrus.Errorln("Failed format data request:", err)
		}

		return
	}

	// Parsing data request
	headerParts := strings.Fields(lines[0])
	requestType := strings.Join(headerParts[1:], " ")
	requestType = strings.Replace(requestType, "]", "", -1)
	dataMap := make(map[string]string)

	// Make map data for request template
	for _, line := range lines[1:] {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		// Handle key-value separation (using ":" or tab "\t")
		parts := strings.SplitN(line, ":", 2)

		if len(parts) == 2 {
			key := strings.TrimSpace(parts[0])
			value := strings.TrimSpace(parts[1])
			value = strings.ReplaceAll(value, "*", "")
			dataMap[key] = value // Store in map
		} else {
			logrus.Warnf("Skipping invalid line: %v on request: %v", line, requestType)
		}
	}

	// Check if all fields are empty
	allEmpty := true

	var keys []string
	switch userLang {
	case "id":
		keys = []string{
			// "RequestType",
			"Nama",
			"Alamat",
			"SN EDC",
			"Kendala",
		}
	case "en":
		keys = []string{
			// "RequestType",
			"Name",
			"Address",
			"EDC SN",
			"Issue",
		}
	}

	for _, key := range keys {
		if dataMap[key] != "" { // If any value is NOT empty, set allEmpty to false
			allEmpty = false
			break
		}
	}

	if allEmpty {
		quotedMsg := &waE2E.ContextInfo{
			StanzaID:      &stanzaID,
			Participant:   &originalSenderJID,
			QuotedMessage: v.Message,
		}

		var textToSend string
		switch userLang {
		case "id":
			textToSend = "Data yang Anda request kosong! Tolong dilengkapi!"
		case "en":
			textToSend = "Your request data is empty! Please complete it!"
		}

		_, err := WhatsappClient.SendMessage(context.Background(), v.Info.Chat, &waE2E.Message{
			ExtendedTextMessage: &waE2E.ExtendedTextMessage{
				Text:        &textToSend,
				ContextInfo: quotedMsg,
			},
		})

		if err != nil {
			logrus.Errorln("Failed format data request coz empty:", err)
		}
		return
	}

	// Get sender phone number from v.Info.Sender
	senderJID := v.Info.Sender
	senderPhoneNumber := "+" + senderJID.User // This is the phone number part (without @s.whatsapp.net)

	// ADD check existing ticket if was requested
	// Ticket Subject e.g. HPY/dd/mm/yyyy/random // FIX ticket subject format
	ticketSubject := fmt.Sprintf("HPY/%s/%s", time.Now().Format("02/01/2006"), fun.GenerateRandomString(30))

	var descFromUser string
	var snEDC string
	var namaPIC string
	var address string
	switch userLang {
	case "id":
		descFromUser = dataMap["Kendala"]
		snEDC = dataMap["SN EDC"]
		namaPIC = dataMap["Nama"]
		address = dataMap["Alamat"]
	case "en":
		snEDC = dataMap["EDC SN"]
		descFromUser = dataMap["Issue"]
		namaPIC = dataMap["Name"]
		address = dataMap["Address"]
	}

	var sbDesc strings.Builder
	sbDesc.WriteString(fmt.Sprintf("Nama PIC: %s<br>", namaPIC))
	sbDesc.WriteString(fmt.Sprintf("No. HP PIC: %s<br>", senderPhoneNumber))
	// sbDesc.WriteString(fmt.Sprintf("Tipe Request: %s\n", requestType))
	sbDesc.WriteString(fmt.Sprintf("Alamat: %s<br>", address))
	sbDesc.WriteString(fmt.Sprintf("SN EDC: %s<br>", snEDC))
	sbDesc.WriteString(fmt.Sprintf("Deskripsi Kendala: %s<br>", descFromUser))
	finalDescription := sbDesc.String()

	newTicket := model.TicketHommyPayCC{
		TicketNumber:  ticketSubject,
		Description:   finalDescription,
		CustomerPhone: senderPhoneNumber,
		StatusInOdoo:  "Draft",
		Priority:      "0",      // ADD priority of the case
		StanzaId:      stanzaID, // mark ticket got from WA
		Sn:            snEDC,
	}
	if errCheck := dbWeb.First(&model.TicketHommyPayCC{}, "ticket_number = ?", newTicket.TicketNumber).Error; errCheck != nil {
		if errCheck == gorm.ErrRecordNotFound {
			if err := dbWeb.Create(&newTicket).Error; err != nil {
				logrus.Errorf("Failed to create new ticket: %v", err)
				return
			} else {
				// logrus.Infof("✅ New ticket created: %s", newTicket.TicketNumber)
				// newTicket.ID is now populated with the auto-incremented primary key
			}
		} else {
			logrus.Errorf("Failed to check if ticket exists: %v", errCheck)
			return
		}
	} else {
		logrus.Warnf("Ticket Number %s already exists, skipping creation", newTicket.TicketNumber)
		return
	}

	var sb strings.Builder
	switch userLang {
	case "id":
		sb.WriteString(fmt.Sprintf("Mohon bersabar Bapak/Ibu *%v*, request Anda terkait EDC dengan Serial Number: _%v_ telah kami terima.\nSelanjutnya akan diproses dan nantinya akan diinfokan kembali ☺", strings.TrimSpace(dataMap["Nama"]), strings.TrimSpace(dataMap["SN EDC"])))
	case "en":
		sb.WriteString(fmt.Sprintf("Please wait Mr/Mrs *%v*, your request regarding the EDC with Serial Number: _%v_ has been received.\nIt will be processed and you will be informed once it is completed ☺", strings.TrimSpace(dataMap["Name"]), strings.TrimSpace(dataMap["EDC SN"])))
	}

	sb.WriteString(fmt.Sprintf("\n\n~Regards,\n *%s*", config.GetConfig().Default.PT))
	textToSend := sb.String()

	WhatsappClient.SendMessage(context.Background(), v.Info.Chat, &waE2E.Message{
		ExtendedTextMessage: &waE2E.ExtendedTextMessage{
			Text: &textToSend,
			// ContextInfo: quotedMsg,
		},
	})

	// Create new ticket in ODOO
	TriggerInsertDatatoODOO <- InsertedDataTriggerItem{
		Database: dbWeb,
		IDinDB:   newTicket.ID,
	}
}

// forwardMessageToCS forwards a received message to the Customer Service (CS) team.
// It sends an acknowledgment message back to the original sender indicating that their
// message has been forwarded and they should wait for a response.
//
// Parameters:
//
//	v                - The incoming WhatsApp message event.
//	originalSenderJID - The JID (Jabber ID) of the original message sender.
//	msg              - The message content to be forwarded.
//
// Note: The actual forwarding logic (e.g., sending to an admin group or storing in a database)
// should be implemented where indicated in the function.
func forwardMessageToCS(v *events.Message, originalSenderJID string, msg string) {
	// // FIX Replace this with real forwarding logic: send to admin group, store in DB, etc.
	// // Forward the message to a specific CS phone number (replace with your CS number)
	// csMessage := fmt.Sprintf("📨 *Message from:* %s\n\n%s", originalSenderJID, msg)
	// sendTextMessageViaBot(originalSenderJID, csMessage)

	_ = v
	_ = msg

	ackMsg := "📬 Message sent to Customer Service. Please wait for a response."
	sendTextMessageViaBot(originalSenderJID, ackMsg)
}

//	func NormalizeSenderJID(jid string) string {
//		if !strings.HasSuffix(jid, "@s.whatsapp.net") {
//			return strings.Split(jid, ":")[0] + "@s.whatsapp.net"
//		}
//		return jid
//	}
func NormalizeSenderJID(jid string) string {
	// First parse the JID properly
	parsed, err := types.ParseJID(jid)
	if err != nil {
		// Fallback for invalid JIDs
		if strings.Contains(jid, "@") {
			return strings.Split(jid, "@")[0] + "@s.whatsapp.net"
		}
		return jid + "@s.whatsapp.net"
	}

	// For groups, return the group JID as-is
	if parsed.Server == types.GroupServer {
		return parsed.String()
	}

	// For users, ensure standard format
	return parsed.User + "@s.whatsapp.net"
}

func extractMessageText(v *events.Message) string {
	if v == nil || v.Message == nil {
		return "[Invalid message: nil]"
	}
	if v.Message.Conversation != nil {
		return *v.Message.Conversation
	} else if v.Message.ExtendedTextMessage != nil && v.Message.ExtendedTextMessage.Text != nil {
		return *v.Message.ExtendedTextMessage.Text
	}
	return "[Non-text message]"
}

func handleLanguageChange(jid, langCode string) {
	if err := SetUserLang(jid, langCode); err != nil {
		logrus.Errorf("Failed to set language for %s: %v", jid, err)
		sendTextMessageViaBot(jid, fmt.Sprintf("Failed to set language: %v. Please try again.", err.Error()))
		return
	}

	var languageData model.Language
	codeToQuery := map[string]string{"en": "us", "id": "id"}[langCode]
	if err := dbWeb.Where("code = ?", codeToQuery).First(&languageData).Error; err != nil {
		logrus.Errorf("Failed to load language data for code %s: %v", codeToQuery, err)
		sendTextMessageViaBot(jid, "Error loading language data. Contact support.")
		return
	}

	var confirm string
	if langCode == "id" {
		confirm = "Bahasa telah diatur ke *" + strings.ToUpper(languageData.Name) + "* 🇲🇨."
	} else {
		confirm = "Language set to *" + strings.ToUpper(languageData.Name) + "* 🇺🇸."
	}
	logrus.Infof("Language for %s set to %s", jid, langCode)
	sendTextMessageViaBot(jid, confirm)
}

func handleCSCommand(jid, lang string) {
	active, err := IsCSSessionActive(jid)
	if err != nil {
		SendLangMessage(jid, "⚠️ Terjadi kesalahan. Silakan coba lagi.", "⚠️ Something went wrong. Please try again.", lang)
		return
	}
	if active {
		SendLangMessage(jid,
			"👩‍💼 Kamu sudah terhubung dengan Customer Service kami. Silakan ketik pertanyaan atau keluhanmu.",
			"👩‍💼 You're already connected to our Customer Service. Please type your question or concern.",
			lang)
	} else {
		if err := StartCSSession(jid); err != nil {
			SendLangMessage(jid,
				"⚠️ Gagal memulai sesi Customer Service. Silakan coba lagi.",
				"⚠️ Failed to start CS session. Please try again.",
				lang)
			return
		}
		// FIX its session time expiry soon !!!!!!
		expiry := time.Now().Add(1 * time.Hour).Format("15:04:05")
		SendLangMessage(jid,
			fmt.Sprintf("✅ Kamu telah terhubung dengan Customer Service kami. Sesi ini berlaku hingga pukul %s.", expiry),
			fmt.Sprintf("✅ You've been connected to our Customer Service. This session is valid until %s.", expiry),
			lang)
	}
}

func handleLogoutCSCommand(jid, lang string) {
	active, err := IsCSSessionActive(jid)
	if err != nil || !active {
		SendLangMessage(jid,
			"ℹ️ Kamu tidak sedang berada dalam sesi Customer Service.",
			"ℹ️ You're not currently in a CS session.",
			lang)
		return
	}
	if err := EndCSSession(jid); err != nil {
		SendLangMessage(jid,
			"⚠️ Gagal mengakhiri sesi Customer Service. Silakan coba lagi.",
			"⚠️ Failed to end CS session. Please try again.",
			lang)
		return
	}
	SendLangMessage(jid,
		"👋 Kamu berhasil keluar dari sesi Customer Service.",
		"👋 You've successfully ended the Customer Service session.",
		lang)
}

// // handleFileMessage processes file/document messages after permission validation
// func handleFileMessage(v *events.Message, msgType string, originalSenderJID string, userLang string, user *model.WAPhoneUser) {
// 	// Log the file message for audit purposes
// 	logrus.Infof("File message received from %s, type: %s, user: %s", originalSenderJID, msgType, user.PhoneNumber)

// 	// Send acknowledgment message to user
// 	var ackMsg string
// 	switch msgType {
// 	case "image":
// 		if userLang == "id" {
// 			ackMsg = "📸 Gambar berhasil diterima! Terimakasih telah mengirimkan gambar."
// 		} else {
// 			ackMsg = "📸 Image received successfully! Thank you for sending the image."
// 		}
// 	case "video":
// 		if userLang == "id" {
// 			ackMsg = "🎥 Video berhasil diterima! Terima kasih telah mengirimkan video."
// 		} else {
// 			ackMsg = "🎥 Video received successfully! Thank you for sending the video."
// 		}
// 	case "document":
// 		if userLang == "id" {
// 			ackMsg = "📄 Dokumen berhasil diterima! Terima kasih telah mengirimkan dokumen."
// 		} else {
// 			ackMsg = "📄 Document received successfully! Thank you for sending the document."
// 		}
// 	case "audio":
// 		if userLang == "id" {
// 			ackMsg = "🎵 Audio berhasil diterima! Terima kasih telah mengirimkan pesan suara."
// 		} else {
// 			ackMsg = "🎵 Audio received successfully! Thank you for sending the voice message."
// 		}
// 	default:
// 		if userLang == "id" {
// 			ackMsg = "📎 File berhasil diterima! Terima kasih."
// 		} else {
// 			ackMsg = "📎 File received successfully! Thank you."
// 		}
// 	}

// 	// Send acknowledgment
// 	sendLangMessageWithStanza(v, v.Info.ID, originalSenderJID, ackMsg, ackMsg, userLang)

// 	// Here you can add additional file processing logic:
// 	// - Save file information to database
// 	// - Download and process the file
// 	// - Forward to appropriate department
// 	// - etc.

// 	// Example: Save file information to database (placeholder)
// 	// saveFileMessageToDatabase(v, msgType, user)
// }

// // saveFileMessageToDatabase saves file message information to database
// ADD this function to handle file message saving if needed
// func saveFileMessageToDatabase(v *events.Message, msgType string, user *model.WAPhoneUser) {
// 	// This is a placeholder function - implement according to your database schema
// 	logrus.Infof("Saving file message to database: type=%s, user_id=%d", msgType, user.ID)

// 	// Example implementation:
// 	// - Extract file metadata from WhatsApp message
// 	// - Save to appropriate table in your database
// 	// - Log the transaction
// }

func handleKeywordReply(jid, messageText, messageTextLower, userLang string, sanitizeRes *SanitizationResult) {
	if sanitizeRes != nil {
		var waBotReplyData []model.WAMessageReply
		if err := dbWeb.
			Where("language_id = (SELECT id FROM languages WHERE code = ?)", userLang).
			Where("for_user_type = ?", sanitizeRes.User.UserType).
			Where("user_of = ?", sanitizeRes.User.UserOf).
			Find(&waBotReplyData).
			Error; err != nil {
			id := "Terjadi kesalahan saat memuat respons. Mohon dicoba lagi nanti"
			en := "Got error while trying load responses. Please try again later"
			SendLangMessage(jid, id, en, userLang)
			return
		}

		var matchedReply *model.WAMessageReply
		for _, reply := range waBotReplyData {
			for _, keyword := range strings.Split(reply.Keywords, config.GetConfig().Whatsmeow.KeywordSeparator) {
				if strings.Contains(strings.ToLower(messageText), strings.ToLower(strings.TrimSpace(keyword))) {
					matchedReply = &reply
					break
				}
			}
			if matchedReply != nil {
				break
			}
		}

		if matchedReply != nil {
			replyText := matchedReply.ReplyText
			if matchedReply.Language != "" {
				replyText = fmt.Sprintf("(%s) %s", strings.ToUpper(matchedReply.Language), replyText)
			}
			sendTextMessageViaBot(jid, replyText)
			return
		}

		// Suggest similar keyword
		var allKeywords []string
		for _, reply := range waBotReplyData {
			for _, keyword := range strings.Split(reply.Keywords, config.GetConfig().Whatsmeow.KeywordSeparator) {
				if trimmed := strings.TrimSpace(keyword); trimmed != "" {
					allKeywords = append(allKeywords, trimmed)
				}
			}
		}

		suggestion := suggestClosestKeyword(messageTextLower, allKeywords)
		if suggestion != "" {
			SendLangMessage(jid,
				fmt.Sprintf("Maaf permintaan Anda tidak dapat diproses, mungkin maksud Anda: *%s*?", suggestion),
				fmt.Sprintf("Sorry, I didn't understand that. Did you mean: *%s*?", suggestion),
				userLang)
			return
		}

		// redisExpiry := time.Duration(config.GetConfig().Whatsmeow.RedisExpiry) * time.Hour
		welcomeKey := "welcome_" + jid
		exists, err := rdb.Exists(contx, welcomeKey).Result()
		if err != nil {
			logrus.Error(err)
		}
		if exists == 0 {
			indo := config.GetConfig().Default.WelcomeID
			eng := config.GetConfig().Default.WelcomeEN

			switch sanitizeRes.User.UserOf {
			case model.UserOfHommyPay:
				indo = config.GetConfig().HommyPayCCData.WelcomeID
				eng = config.GetConfig().HommyPayCCData.WelcomeEN
			}

			SendLangMessage(jid, indo, eng, userLang)
			// rdb.Set(contx, welcomeKey, "true", redisExpiry)
			rdb.Set(contx, welcomeKey, "true", 24*3*time.Hour) // 3 days expiry for welcome message

			// ADD send reply with OLLAMA if needed
			// SendLangMessage(jid,
			// 	"Maaf, saya tidak mengerti. Silakan coba lagi atau pilih bahasa dengan input 'en' atau 'id'.",
			// 	"Sorry, I didn't understand that. Please try again or choose a language with input 'en' or 'id'.",
			// 	userLang)
		}
	} else {
		// ADD condition to check if not validated user !!
	}
}

func SendLangMessage(jid, idText, enText, lang string) {
	userLang, err := GetUserLang(jid)
	if err != nil {
		lang = "en"
	}
	if userLang != "" {
		lang = userLang
	}

	var msg string
	switch lang {
	case "id":
		msg = idText
	case "en":
		msg = enText
	default:
		msg = enText
	}
	sendTextMessageViaBot(jid, msg)
}

func sendLangMessageWithStanza(v *events.Message, stanzaID, originalSenderJID, idText, enText, lang string) {
	userLang, err := GetUserLang(originalSenderJID)
	if err != nil {
		lang = "en"
	}
	if userLang != "" {
		lang = userLang
	}

	var msg string
	switch lang {
	case "id":
		msg = idText
	case "en":
		msg = enText
	default:
		msg = enText
	}

	sendWhatsAppMessageWithStanza(v, stanzaID, originalSenderJID, msg)
}

// askOllama sends a prompt to the Ollama API and returns the generated response as a string.
// It retrieves the Ollama URL and model from the application configuration. If the URL is not configured,
// it returns an error. The function sends a POST request with the prompt to the Ollama endpoint,
// parses the JSON response, and returns the generated text. Any errors during the HTTP request or
// JSON unmarshalling are returned.
// func askOllama(prompt string) (string, error) {
// 	url := config.GetConfig().Whatsmeow.OllamaURL
// 	if url == "" {
// 		return "", errors.New("ollama URL is not configured")
// 	}

// 	body, _ := json.Marshal(OllamaRequest{
// 		Model:  config.GetConfig().Whatsmeow.OllamaModel,
// 		Prompt: prompt,
// 		Stream: false,
// 	})

// 	resp, err := http.Post(url, "application/json", bytes.NewBuffer(body))
// 	if err != nil {
// 		return "", err
// 	}
// 	defer resp.Body.Close()

// 	data, _ := io.ReadAll(resp.Body)

// 	// Ollama streams tokens, but we can assume it's finished if we just parse latest response
// 	var result OllamaResponse
// 	if err := json.Unmarshal(data, &result); err != nil {
// 		return "", err
// 	}

// 	return result.Response, nil
// }

// ValidateWhatsappBOTPhoneUser checks PhoneUser against allowed rules
func ValidateWhatsappBOTPhoneUser(phoneNumber, senderJID string, isGroup bool, msgType string) (*SanitizationResult, error) {
	// Get user language (early)
	var userLang string
	var errNotRegistered, errBanned, errInvalidChat, errMessageTypeDenied error
	userLang, err := GetUserLang(senderJID)
	if err != nil {
		logrus.Errorf("Failed to get user lang: %v", err)
		userLang = "en"
	}

	switch strings.ToLower(userLang) {
	case "id":
		errNotRegistered = errors.New(config.GetConfig().Whatsmeow.WaErrorMessage.ID.PhoneNumberNotRegistered)
		errBanned = errors.New(config.GetConfig().Whatsmeow.WaErrorMessage.ID.PhoneNumberIsBanned)
		errInvalidChat = errors.New(config.GetConfig().Whatsmeow.WaErrorMessage.ID.InvalidChat)
		errMessageTypeDenied = errors.New(config.GetConfig().Whatsmeow.WaErrorMessage.ID.MessageTypeDenied)
	default:
		errNotRegistered = errors.New(config.GetConfig().Whatsmeow.WaErrorMessage.EN.PhoneNumberNotRegistered)
		errBanned = errors.New(config.GetConfig().Whatsmeow.WaErrorMessage.EN.PhoneNumberIsBanned)
		errInvalidChat = errors.New(config.GetConfig().Whatsmeow.WaErrorMessage.EN.InvalidChat)
		errMessageTypeDenied = errors.New(config.GetConfig().Whatsmeow.WaErrorMessage.EN.MessageTypeDenied)
	}

	var user model.WAPhoneUser
	db := dbWeb

	// Special handling for linked device JIDs
	if strings.Contains(phoneNumber, "@") && strings.Contains(phoneNumber, "lid") {
		logrus.Infof("DEBUG - Validating linked device JID: %s", phoneNumber)
		// For linked devices, try to find user by JID in a different way
		// Since we can't validate the phone number, we'll need to find by JID or skip validation

		// Try to find any user that might have used this JID before
		// For now, let's skip the strict phone validation for linked devices
		logrus.Warnf("Linked device detected, skipping phone validation and allowing message")

		return &SanitizationResult{
			User: nil, // No specific user found
		}, nil
	}

	// Find user by phone number (normal flow)
	validPhoneNumber, err := CheckValidWhatsappPhoneNumber(phoneNumber)
	if err != nil {
		return nil, err
	}

	err = db.Where("phone_number LIKE ?", "%"+validPhoneNumber+"%").First(&user).Error
	if err != nil {
		// Not found in DB
		return nil, errNotRegistered
	}

	if !user.IsRegistered {
		return nil, errNotRegistered
	}

	if user.IsBanned {
		return nil, errBanned
	}

	// Check allowed chat context
	switch user.AllowedChats {
	case model.DirectChat:
		if isGroup {
			return nil, errInvalidChat
		}
	case model.GroupChat:
		if !isGroup {
			return nil, errInvalidChat
		}
	case model.BothChat:
		// allowed everywhere
	default:
		return nil, errInvalidChat
	}

	// Check message type
	var allowedTypes []model.WAMessageType
	if len(user.AllowedTypes) > 0 {
		if err := json.Unmarshal(user.AllowedTypes, &allowedTypes); err != nil {
			logrus.Warnf("Failed to parse AllowedTypes JSON for user %s: %v", user.PhoneNumber, err)
			return nil, errMessageTypeDenied // or return nil, err to block if corrupt data
		}
	}

	allowed := false
	for _, t := range allowedTypes {
		if strings.EqualFold(string(t), msgType) {
			allowed = true
			break
		}
	}
	if !allowed {
		return nil, errMessageTypeDenied
	}

	return &SanitizationResult{User: &user}, nil
}

// CheckAndNotifyQuota checks quota for user, sends warning once, and tells caller if processing should stop.
// Returns: shouldProcess (true if under quota), error if something failed.
func CheckAndNotifyQuota(
	userID uint,
	userLang string,
	userJID string,
	maxQuota int,
) (bool, error) {

	// Daily quota key: e.g. wa_quota:123:2025-07-11
	today := time.Now().Format("2006-01-02")
	quotaKey := fmt.Sprintf("wa_quota:%d:%s", userID, today)

	// Increment counter
	quotaCount, err := rdb.Incr(contx, quotaKey).Result()
	if err != nil {
		logrus.Errorf("Failed to increment quota counter for user %d: %v", userID, err)
		// Fail-safe: block processing
		return false, err
	}

	// On first increment, set expiry so it resets daily
	if quotaCount == 1 {
		rdb.Expire(contx, quotaKey, time.Duration(config.GetConfig().Whatsmeow.RedisExpiry)*time.Hour)
	}

	// Over quota?
	if int(quotaCount) > maxQuota {
		// Check if we've already warned today
		warnKey := fmt.Sprintf("quota_warned:%d:%s", userID, today)
		isWarned, err := rdb.Exists(contx, warnKey).Result()
		if err != nil {
			logrus.Errorf("Failed to check warnKey: %v", err)
			// fallback: warn anyway
			isWarned = 0
		}

		if isWarned == 0 {
			// Get TTL to tell user when quota resets
			ttl, err := rdb.TTL(contx, quotaKey).Result()
			if err != nil || ttl <= 0 {
				ttl = time.Duration(config.GetConfig().Whatsmeow.RedisExpiry) * time.Hour
			}

			waitMsg := fun.FormatTTL(ttl)

			idMsg := fmt.Sprintf(
				"Anda telah mencapai batas kuota harian (%d) pesan.\nMohon tunggu *%s* lagi hingga dapat mengirim pesan lagi besok.",
				maxQuota, waitMsg,
			)
			enMsg := fmt.Sprintf(
				"You have reached your daily quota of %d messages.\nPlease wait *%s* until you can send messages again tomorrow.",
				maxQuota, waitMsg,
			)

			// Replace with your actual send function
			SendLangMessage(userJID, idMsg, enMsg, userLang)

			// Mark as warned today (with same TTL as quota key)
			rdb.Set(contx, warnKey, "1", ttl)
		}

		// Over quota: do not process further
		return false, nil
	}

	// Under quota: proceed
	return true, nil
}

// GetQuotaResetTime returns the time when user's quota resets today.
func GetQuotaResetTime(userID uint) (*time.Time, error) {
	today := time.Now().Format("2006-01-02")
	quotaKey := fmt.Sprintf("wa_quota:%d:%s", userID, today)

	ttl, err := rdb.TTL(contx, quotaKey).Result()
	if err != nil {
		return nil, err
	}
	if ttl <= 0 {
		// No expiry set, or already expired
		return nil, nil
	}

	resetTime := time.Now().Add(ttl)
	return &resetTime, nil
}

// ResetQuotaExceeded resets the quota counter and warning flag for a user for today.
// Effectively lets the user send messages again immediately.
func ResetQuotaExceeded(userID uint) error {
	today := time.Now().Format("2006-01-02")

	// Keys to check
	quotaKey := fmt.Sprintf("wa_quota:%d:%s", userID, today)
	warnKey := fmt.Sprintf("quota_warned:%d:%s", userID, today)

	// Check if quotaKey exists
	quotaExists, err := rdb.Exists(contx, quotaKey).Result()
	if err != nil {
		logrus.Errorf("Failed to check if quota key exists for user %d: %v", userID, err)
		return err
	}

	// Check if warnKey exists
	warnExists, err := rdb.Exists(contx, warnKey).Result()
	if err != nil {
		logrus.Errorf("Failed to check if warn key exists for user %d: %v", userID, err)
		return err
	}

	// Log the existence status
	logrus.Infof("Quota key exists: %v, Warn key exists: %v for user %d", quotaExists > 0, warnExists > 0, userID)

	if quotaExists == 0 && warnExists == 0 {
		logrus.Warnf("No quota keys found to reset for user %d", userID)
		// You could decide to return here if you only want to reset if keys exist
	}

	// Delete keys (whether or not they existed)
	_, err = rdb.Del(contx, quotaKey, warnKey).Result()
	if err != nil {
		logrus.Errorf("Failed to reset quota for user %d: %v", userID, err)
		return err
	}

	logrus.Infof("Quota reset for user %d", userID)
	return nil
}

func UnbanAndUnlockUser(userID uint) error {
	// Update user: set is_banned = false
	if err := dbWeb.Model(&model.WAPhoneUser{}).Where("id = ?", userID).Update("is_banned", false).Error; err != nil {
		return fmt.Errorf("failed to unban user %d: %w", userID, err)
	}

	// Remove bad word strike counter from Redis if exists
	key := fmt.Sprintf("badword:user:%d", userID)
	ctx := context.Background()
	if err := rdb.Del(ctx, key).Err(); err != nil {
		// Not fatal, just log
		logrus.Warnf("Failed to delete Redis badword counter for user %d: %v", userID, err)
	}

	logrus.Infof("✅ Successfully unbanned user ID %d and removed badword strike counter", userID)
	return nil
}
