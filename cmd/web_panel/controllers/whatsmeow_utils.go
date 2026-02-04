package controllers

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"mime"
	"net/http"
	"os"
	"path/filepath"
	"service-platform/cmd/web_panel/fun"
	"service-platform/cmd/web_panel/internal/gormdb"
	"service-platform/cmd/web_panel/model"
	tamodel "service-platform/cmd/web_panel/model/ta_model"
	"service-platform/internal/config"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/TigorLazuardi/tanggal"
	"github.com/briandowns/openweathermap"
	"github.com/gin-gonic/gin"
	"github.com/sirupsen/logrus"
	"go.mau.fi/whatsmeow"
	"go.mau.fi/whatsmeow/proto/waE2E"
	"go.mau.fi/whatsmeow/types"
	"go.mau.fi/whatsmeow/types/events"
	"google.golang.org/protobuf/proto"
	"gorm.io/gorm"
)

// contains checks whether a given string `item` exists in the provided slice of strings `slice`.
// Returns true if the item is found, false otherwise.
func contains(slice []string, item string) bool {
	for _, s := range slice {
		if s == item {
			return true
		}
	}
	return false
}

// containsJID checks whether a given JID exists in a list of group JID strings.
// It iterates over the provided groupList, converts each string to a types.JID
// with the server set to "g.us", and compares it to the target jid.
// Returns true if the jid is found in the list, otherwise returns false.
//
// Parameters:
//   - groupList: A slice of strings representing group JIDs (without the server part).
//   - jid: The types.JID to search for in the groupList.
//
// Returns:
//   - bool: true if the jid is present in the groupList, false otherwise.
func containsJID(groupList []string, jid types.JID) bool {
	for _, group := range groupList {
		groupJID := types.NewJID(group, "g.us") // Convert string to types.JID
		if groupJID == jid {
			return true
		}
	}
	return false
}

// getFileExtension returns the file extension associated with the provided MIME type.
// If the MIME type is recognized, it returns the first corresponding extension (e.g., ".mp4").
// If the MIME type is unknown or an error occurs, it returns a safe default extension ".bin".
func getFileExtension(mimeType string) string {
	exts, err := mime.ExtensionsByType(mimeType)
	if err == nil && len(exts) > 0 {
		return exts[0] // e.g., ".mp4"
	}
	// fallback to safe default if unknown
	return ".bin"
}

// getSafeString safely dereferences a string pointer and returns its value.
// If the pointer is nil or points to an empty string, it returns an empty string instead.
func getSafeString(s *string) string {
	if s != nil && *s != "" {
		return *s
	}
	return ""
}

// CheckValidWhatsappPhoneNumber sanitizes and validates a phone number to ensure it is registered on WhatsApp.
//
// Steps performed:
// - Checks if the phone number is provided and meets minimum length.
// - Sanitizes the number by removing invalid characters (via helper).
// - Constructs the WhatsApp JID and checks if it exists using WhatsAppClient.
//
// Parameters:
// - phoneNumber: Raw user input phone number (e.g., "08123456789").
//
// Returns:
// - A sanitized version of the phone number without country code prefix (e.g., "8123456789").
// - An error if the number is empty, too short, invalid, or not registered on WhatsApp.
//
// This function ensures that outbound messages are not sent to non-WhatsApp users.
func CheckValidWhatsappPhoneNumber(phoneNumber string) (string, error) {
	if phoneNumber == "" {
		return "", errors.New("no phone number provided")
	}

	// Check minimum length
	if len(phoneNumber) < config.WebPanel.Get().Default.MinLengthPhoneNumber {
		return "", fmt.Errorf("%s is too short, min. %d digits", phoneNumber, config.WebPanel.Get().Default.MinLengthPhoneNumber)
	}

	// Sanitize the number (e.g., remove +, spaces, etc.)
	sanitizedPhone, err := fun.SanitizePhoneNumber(phoneNumber)
	if err != nil {
		return "", fmt.Errorf("failed to sanitize phone number: %w", err)
	}

	// Append @s.whatsapp.net for WhatsApp JID
	jid := sanitizedPhone + "@s.whatsapp.net"

	// Check if the number is registered on WhatsApp
	results, err := WhatsappClient.IsOnWhatsApp(context.Background(), []string{jid})
	if err != nil {
		return "", fmt.Errorf("failed to check WhatsApp status: %w", err)
	}

	// No results returned
	if len(results) == 0 {
		return "", errors.New("no WhatsApp result returned")
	}

	// Check contact status
	if !results[0].IsIn {
		return "", fmt.Errorf("%s is not registered on WhatsApp", phoneNumber)
	}

	return sanitizedPhone, nil
}

// GetWhatsappGroup retrieves the list of WhatsApp groups that the client has joined using the WhatsappClient.
// It serializes the group data into a formatted JSON structure and writes it to a file specified in the configuration
// (config.WebPanel.Get().Whatsmeow.WaGroupSource). If an error occurs during group retrieval or file writing, it logs the error
// to the standard output. This function is useful for exporting or backing up group metadata for further processing or inspection.
//
// Details:
//   - Fetches all joined WhatsApp groups via WhatsappClient.GetJoinedGroups().
//   - Serializes the group data into indented JSON for readability.
//   - Writes the JSON data to a configured file path.
//   - Logs errors encountered during group retrieval or file writing.
//
// Note: Logging of detailed group and participant information is available but commented out for optional use.
func GetWhatsappGroup() {
	data, err := WhatsappClient.GetJoinedGroups(context.Background())
	if err != nil {
		logrus.Errorf("GetWhatsappGroup: %v\n", err)
		return
	}

	// // DEBUG
	// for _, group := range data {
	// 	logrus.Infof("\n📌 Group: %s (ID: %s)\n", group.Name, group.JID.String())
	// 	logrus.Infof("   🔹 Creator: %s \n", group.OwnerJID.User)

	// 	logrus.Info("   👥 Participants:")
	// 	for _, participant := range group.Participants {
	// 		role := "member"
	// 		if participant.IsAdmin {
	// 			role = "admin"
	// 		}
	// 		if participant.IsSuperAdmin {
	// 			role = "superadmin"
	// 		}
	// 		logrus.Infof("      - %s (%s)\n", participant.JID.String(), role)
	// 	}
	// }

	jsonData, _ := json.MarshalIndent(data, "", "  ")

	err = os.WriteFile(config.WebPanel.Get().Whatsmeow.WaGroupSource, jsonData, 0644)
	if err != nil {
		logrus.Errorf("Failed to write group data to file: %v\n", err)
		return
	}
}

// sendTextMessageViaBot sends a plain text message to a specified WhatsApp JID using the provided WhatsApp client.
// It parses the given JID string, removes any device part, and sends the message as a simple conversation message.
// If parsing the JID or sending the message fails, it prints an error to the standard output.
//
// Parameters:
//   - jid: The recipient's JID as a string.
//   - message: The text message to send.
func sendTextMessageViaBot(jid string, message string) {
	parsedJID, err := types.ParseJID(jid)
	if err != nil {
		logrus.Errorf("Failed to parse JID: %v\n", err)
		return
	}

	// Remove device part if present
	userJID := types.JID{
		User:   parsedJID.User,
		Server: parsedJID.Server,
	}

	_, err = WhatsappClient.SendMessage(context.Background(), userJID, &waE2E.Message{
		Conversation: proto.String(message),
	})

	if err != nil {
		logrus.Errorf("Failed to send message to %s: %v\n", userJID.String(), err)
	}
}

// whatsmeowPing sends a "Pong!" response to a WhatsApp message, quoting the original message.
// It also asynchronously logs the sent message to the database.
//
// Parameters:
//   - v: Pointer to the incoming WhatsApp message event.
//   - stanzaID: The stanza ID of the original message to quote.
//   - originalSenderJID: The JID of the original sender.
//
// The function sends a reply using the global WhatsappClient, quoting the original message.
// If sending fails, it logs the error. After sending, it inserts the sent message details
// into the database asynchronously, logging any database errors encountered.
func whatsmeowPing(v *events.Message, stanzaID, originalSenderJID string) {
	taskDoing := "Ping WA Bot"
	textToSend := "Pong! 🏓"

	quotedMsg := &waE2E.ContextInfo{
		StanzaID:      &stanzaID,
		Participant:   &originalSenderJID,
		QuotedMessage: v.Message,
	}
	resp, err := WhatsappClient.SendMessage(context.Background(), v.Info.Chat, &waE2E.Message{
		ExtendedTextMessage: &waE2E.ExtendedTextMessage{
			Text:        proto.String(textToSend),
			ContextInfo: quotedMsg,
		},
	})
	if err != nil {
		logrus.Errorf("%s: %v\n", taskDoing, err)
	}

	// Insert whatsapp msg to DB
	go func() {
		msg := model.WAMessage{
			ID:          resp.ID,
			ChatJID:     v.Info.Chat.String(),
			SenderJID:   resp.Sender.User + "@" + resp.Sender.Server,
			MessageBody: textToSend,
			MessageType: "text",
			IsGroup:     v.Info.Chat.Server == "g.us",
			Status:      "sent",
			SentAt:      resp.Timestamp,
		}
		if err := dbWeb.Create(&msg).Error; err != nil {
			logrus.Errorf("error while trying to insert msg of %v: %v", taskDoing, err)
		}
	}()
}

// PingWhatsapp returns a Gin handler function that pings a WhatsApp user by sending a "Ping!" message
// to the user's WhatsApp number, identified by their userId in the database. It validates the WhatsApp client
// connection, retrieves the user's phone number, sanitizes it, and sends the message. The sent message is also
// logged asynchronously to the database. Responds with appropriate HTTP status codes and messages based on the outcome.
//
// Returns:
//   - gin.HandlerFunc: The Gin handler for the ping endpoint.
func PingWhatsapp() gin.HandlerFunc {
	return func(c *gin.Context) {
		taskDoing := "Ping Whatsapp"

		if WhatsappClient == nil {
			c.JSON(http.StatusInternalServerError, gin.H{"message": "WhatsApp client not initialized, try to refresh QR code"})
			return
		}

		if !WhatsappClient.IsConnected() {
			c.JSON(http.StatusInternalServerError, gin.H{"message": "WhatsApp client not connected"})
			return
		}

		userId := c.PostForm("userId")
		if userId == "" {
			c.JSON(http.StatusBadRequest, gin.H{"message": "userId is required"})
			return
		}

		var userData model.Admin
		result := dbWeb.Where("id = ?", userId).First(&userData)
		if result.Error != nil {
			if result.Error == gorm.ErrRecordNotFound {
				c.JSON(http.StatusNotFound, gin.H{"message": "user not found"})
				return
			}
			c.JSON(http.StatusInternalServerError, gin.H{"message": "failed to query user data", "detail": result.Error.Error()})
			return
		}

		validPhoneNumber, err := fun.SanitizePhoneNumber(userData.Phone)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"message": "failed to sanitize phone number", "detail": err.Error()})
			return
		}

		msgToSend := "Ping!"
		resp, err := WhatsappClient.SendMessage(context.Background(), types.NewJID("62"+validPhoneNumber, "s.whatsapp.net"), &waE2E.Message{
			Conversation: &msgToSend,
		})
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"message": "failed to send ping message", "detail": err.Error()})
			return
		}

		// Insert whatsapp msg to DB
		go func() {
			msg := model.WAMessage{
				ID:          resp.ID,
				ChatJID:     types.NewJID("62"+validPhoneNumber, "s.whatsapp.net").String(),
				SenderJID:   resp.Sender.User + "@" + resp.Sender.Server,
				MessageBody: msgToSend,
				MessageType: "text",
				IsGroup:     types.NewJID("62"+validPhoneNumber, "s.whatsapp.net").Server == "g.us",
				Status:      "sent",
				SentAt:      resp.Timestamp,
			}
			if err := dbWeb.Create(&msg).Error; err != nil {
				logrus.Errorf("error while trying to insert msg of %v: %v", taskDoing, err)
			}
		}()

		c.JSON(http.StatusOK, gin.H{"message": "Ping message sent successfully"})
	}
}

// SendTextWhatsapp handles sending a plain text message via WhatsApp to either a user or a group.
// It is designed as a Gin handler function for an HTTP POST request.
//
// This handler performs the following steps:
//   - Validates the incoming form parameters: userId, recipient, message.
//   - Verifies WhatsApp client connectivity.
//   - Checks if the user exists in the database.
//   - Validates and formats the recipient JID (individual or group).
//   - Ensures the message length does not exceed configured character limits.
//   - Optionally appends a footer to the message.
//   - Sends the text message using the global WhatsApp client.
//   - Asynchronously logs the sent message into the database.
//
// Request Form Parameters:
//   - userId: The ID of the admin user sending the message.
//   - recipient: The phone number or group JID to send the message to.
//   - message: The text message content to be sent.
//   - isGroup: Flag indicating if the message is sent to a group (expects "true" or "false").
//   - useFooter: Flag indicating if a standard footer should be appended to the message.
//
// Response Codes:
//   - 200 OK: Message successfully sent.
//   - 400 Bad Request: Missing or invalid input.
//   - 404 Not Found: User not found.
//   - 500 Internal Server Error: WhatsApp not connected or internal failures.
//
// This function relies on global configuration and a connected instance of WhatsAppClient.
// It uses GORM for database operations and logs errors using logrus.
func SendTextWhatsapp() gin.HandlerFunc {
	return func(c *gin.Context) {
		taskDoing := "Sent Text Whatsapp"
		// Check WhatsApp connection
		if WhatsappClient == nil || !WhatsappClient.IsConnected() {
			c.JSON(http.StatusInternalServerError, gin.H{"message": "WhatsApp client not connected"})
			return
		}

		userID := c.PostForm("userId")
		isGroup := c.PostForm("isGroup")
		recipient := c.PostForm("recipient")
		message := c.PostForm("message")
		useFooter := c.PostForm("useFooter")

		// Validate required fields
		if userID == "" || recipient == "" || message == "" {
			c.JSON(http.StatusBadRequest, gin.H{"message": "Missing required fields"})
			return
		}

		var userData model.Admin
		result := dbWeb.Where("id = ?", userID).First(&userData)
		if result.Error != nil {
			if result.Error == gorm.ErrRecordNotFound {
				c.JSON(http.StatusNotFound, gin.H{"message": "user not found"})
				return
			}
			c.JSON(http.StatusInternalServerError, gin.H{"message": "failed to query user data", "detail": result.Error.Error()})
			return
		}

		var jid types.JID
		if isGroup == "true" {
			j, err := ValidateGroupJID(recipient)
			if err != nil {
				c.JSON(http.StatusBadRequest, gin.H{"message": err.Error()})
				return
			}
			jid = j
		} else {
			validWAPhoneNumber, err := CheckValidWhatsappPhoneNumber(recipient)
			if err != nil {
				c.JSON(http.StatusBadRequest, gin.H{"message": err.Error()})
				return
			}
			jid = types.NewJID("62"+validWAPhoneNumber, "s.whatsapp.net")
		}

		if message == "" {
			c.JSON(http.StatusBadRequest, gin.H{"message": "empty message to send"})
			return
		}

		if len(message) > config.WebPanel.Get().Default.MaxMessageCharacters {
			c.JSON(http.StatusBadRequest, gin.H{"message": fmt.Sprintf("max chars of message cannot more than %d characters", config.WebPanel.Get().Default.MaxMessageCharacters)})
			return
		}

		var sb strings.Builder
		sb.WriteString(message)
		if useFooter == "true" {
			sb.WriteString(fmt.Sprintf("\n\n~Regards, *%v*", config.WebPanel.Get().Default.PT))
		}
		textToSend := sb.String()

		resp, err := WhatsappClient.SendMessage(context.Background(), jid, &waE2E.Message{
			Conversation: &textToSend,
		})

		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"message": "failed to send text message", "detail": err.Error()})
			return
		}

		// Insert whatsapp msg to DB
		go func() {
			msg := model.WAMessage{
				ID:          resp.ID,
				ChatJID:     jid.String(),
				SenderJID:   resp.Sender.User + "@" + resp.Sender.Server,
				MessageBody: textToSend,
				MessageType: "text",
				IsGroup:     jid.Server == "g.us",
				Status:      "sent",
				SentAt:      resp.Timestamp,
			}
			if err := dbWeb.Create(&msg).Error; err != nil {
				logrus.Errorf("error while trying to insert msg of %v: %v", taskDoing, err)
			}
		}()

		c.JSON(http.StatusOK, gin.H{"message": fmt.Sprintf("Message successfully sent @%v", resp.Timestamp.Format("15:04:05, 02 Jan 2006"))})
	}
}

// SendImageWhatsapp handles the upload and sending of image messages via WhatsApp through an HTTP POST request.
//
// This handler performs the following:
// - Validates required form data (userId, recipient, image file).
// - Ensures WhatsApp client is connected.
// - Validates and formats the recipient JID (individual or group).
// - Validates and stores the uploaded image file temporarily.
// - Verifies the MIME type is either JPEG or PNG.
// - Uploads the image to WhatsApp servers via WhatsAppClient.
// - Sends the image with an optional caption and view-once flag.
// - Logs the sent message asynchronously into the database.
//
// Request Form Parameters:
// - userId: The ID of the admin user sending the image.
// - recipient: The target phone number or group JID.
// - message: Optional caption for the image.
// - isGroup: Whether the recipient is a group ("true" or "false").
// - useFooter: Whether to append a default footer to the caption.
// - viewOnce: If "true", enables WhatsApp's view-once feature.
//
// File Upload:
// - image: Multipart form file. Only JPEG and PNG are accepted.
//
// Response Codes:
// - 200 OK: Image successfully sent.
// - 400 Bad Request: Missing or invalid data, wrong image type, or file too large.
// - 404 Not Found: User not found.
// - 500 Internal Server Error: Upload/send failures or DB issues.
//
// Image files are processed in-memory and deleted after use. Max image size is configurable.
func SendImageWhatsapp() gin.HandlerFunc {
	return func(c *gin.Context) {
		taskDoing := "Send Image Whatsapp"

		// Check WhatsApp connection
		if WhatsappClient == nil || !WhatsappClient.IsConnected() {
			c.JSON(http.StatusInternalServerError, gin.H{"message": "WhatsApp client not connected"})
			return
		}

		userID := c.PostForm("userId")
		isGroup := c.PostForm("isGroup")
		recipient := c.PostForm("recipient")
		message := c.PostForm("message")
		useFooter := c.PostForm("useFooter")
		viewOnce := c.PostForm("viewOnce")

		// Validate required fields
		if userID == "" || recipient == "" {
			c.JSON(http.StatusBadRequest, gin.H{"message": "Missing required fields"})
			return
		}

		var userData model.Admin
		result := dbWeb.Where("id = ?", userID).First(&userData)
		if result.Error != nil {
			if result.Error == gorm.ErrRecordNotFound {
				c.JSON(http.StatusNotFound, gin.H{"message": "user not found"})
				return
			}
			c.JSON(http.StatusInternalServerError, gin.H{"message": "failed to query user data", "detail": result.Error.Error()})
			return
		}

		fileHeader, err := c.FormFile("image")
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{
				"message": "Image file is required",
			})
			return
		}

		// ✅ Check file size
		if fileHeader.Size > config.WebPanel.Get().Default.MaxImageSize*1024*1024 {
			c.JSON(http.StatusBadRequest, gin.H{
				"message": "File too large",
				"detail":  fmt.Sprintf("Maximum allowed size is %d MB", config.WebPanel.Get().Default.MaxImageSize),
			})
			return
		}

		// ✅ Open the uploaded file
		file, err := fileHeader.Open()
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"message": "Failed to read image", "detail": err.Error()})
			return
		}
		defer file.Close()

		// ✅ Verify the content is a valid image (not just .jpg extension)
		buffer := make([]byte, 512)
		if _, err := file.Read(buffer); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"message": "Failed to read image content", "detail": err.Error()})
			return
		}

		contentType := http.DetectContentType(buffer)
		if contentType != "image/jpeg" && contentType != "image/png" {
			c.JSON(http.StatusBadRequest, gin.H{
				"message": "Invalid image content",
				"detail":  "Only JPG and PNG images are allowed",
			})
			return
		}

		// ✅ Reset reader so we can save the file later
		if _, err := file.Seek(0, io.SeekStart); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"message": "Failed to reset file pointer", "detail": err.Error()})
			return
		}

		// ✅ Save file to temporary path
		filename := fmt.Sprintf("%d-%s", time.Now().UnixNano(), filepath.Base(fileHeader.Filename))
		tempPath := filepath.Join(os.TempDir(), filename)
		outFile, err := os.Create(tempPath)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"message": "Failed to create temp file", "detail": err.Error()})
			return
		}
		defer func() {
			outFile.Close()
			os.Remove(tempPath)
		}()

		if _, err := io.Copy(outFile, file); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"message": "Failed to save uploaded file", "detail": err.Error()})
			return
		}

		// Upload image
		imgData, err := os.ReadFile(tempPath)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"message": "Failed to read file", "detail": err.Error()})
			return
		}

		uploaded, err := WhatsappClient.Upload(context.Background(), imgData, whatsmeow.MediaImage)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"message": "Failed to upload image", "detail": err.Error()})
			return
		}

		if uploaded.URL == "" || uploaded.DirectPath == "" {
			c.JSON(http.StatusInternalServerError, gin.H{"message": "Failed to upload image", "detail": "upload did not return a valid URL or DirectPath"})
			return
		}

		var sb strings.Builder
		if message != "" {
			if len(message) > config.WebPanel.Get().Default.MaxMessageCharacters {
				c.JSON(http.StatusBadRequest, gin.H{"message": fmt.Sprintf("max chars of message cannot more than %d characters", config.WebPanel.Get().Default.MaxMessageCharacters)})
				return
			}

			sb.WriteString(message)
		}
		if useFooter == "true" {
			sb.WriteString(fmt.Sprintf("\n\n~Regards, *%s*", config.WebPanel.Get().Default.PT))
		}
		textToSend := sb.String()

		// Send image
		var jid types.JID
		if isGroup == "true" {
			j, err := ValidateGroupJID(recipient)
			if err != nil {
				c.JSON(http.StatusBadRequest, gin.H{"message": err.Error()})
				return
			}
			jid = j
		} else {
			validWAPhoneNumber, err := CheckValidWhatsappPhoneNumber(recipient)
			if err != nil {
				c.JSON(http.StatusBadRequest, gin.H{"message": err.Error()})
				return
			}
			jid = types.NewJID("62"+validWAPhoneNumber, "s.whatsapp.net")
		}

		var onlyViewOnce bool = true
		if viewOnce != "true" {
			onlyViewOnce = false
		}

		resp, err := WhatsappClient.SendMessage(context.Background(), jid, &waE2E.Message{
			ImageMessage: &waE2E.ImageMessage{
				URL:               proto.String(uploaded.URL),
				DirectPath:        proto.String(uploaded.DirectPath),
				MediaKey:          uploaded.MediaKey,
				Mimetype:          proto.String(contentType), // proto.String("image/jpeg"), // Change MIME type as needed (jpeg, png, etc.)
				Caption:           &textToSend,
				FileSHA256:        uploaded.FileSHA256,
				FileEncSHA256:     uploaded.FileEncSHA256,
				FileLength:        proto.Uint64(uint64(len(imgData))),
				MediaKeyTimestamp: proto.Int64(time.Now().Unix()),
				ViewOnce:          &onlyViewOnce,
			},
		})
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"message": "Failed to send image", "detail": err.Error()})
			return
		}

		// Insert whatsapp msg to DB
		go func() {
			msg := model.WAMessage{
				ID:          resp.ID,
				ChatJID:     jid.String(),
				SenderJID:   resp.Sender.User + "@" + resp.Sender.Server,
				MessageBody: textToSend,
				MessageType: "image",
				IsGroup:     jid.Server == "g.us",
				Status:      "sent",
				SentAt:      resp.Timestamp,
			}
			if err := dbWeb.Create(&msg).Error; err != nil {
				logrus.Errorf("error while trying to insert msg of %v: %v", taskDoing, err)
			}
		}()

		c.JSON(http.StatusOK, gin.H{"message": fmt.Sprintf("Image successfully sent @%v", resp.Timestamp.Format("15:04:05, 02 Jan 2006"))})
	}
}

// SendDocumentWhatsapp handles uploading and sending a document via WhatsApp through an HTTP POST request.
//
// This Gin handler performs the following tasks:
// - Validates required form fields (userId, recipient, document file).
// - Checks if the WhatsApp client is connected.
// - Validates the user from the database (userId).
// - Validates and processes the uploaded file:
// - Checks MIME type and file extension (only allows PDF, DOC, DOCX, TXT).
// - Reads the file contents and uploads to WhatsApp's media servers.
// - Prepares the caption (supports optional footer and character limit).
// - Determines if the recipient is a group or individual, and validates the JID.
// - Sends the document via WhatsApp with metadata (filename, mimetype, size, etc).
// - Asynchronously logs the message into the database (model.WAMessage).
//
// Accepted Form Fields:
// - userId: ID of the admin user sending the message.
// - recipient: Phone number or group JID.
// - message: Optional caption for the document.
// - isGroup: "true" if the recipient is a group.
// - useFooter: "true" to append a company-branded footer.
// - document: Multipart form file. Accepted types: PDF, DOC, DOCX, TXT.
//
// Validation Details:
// - Allowed MIME types: application/pdf, application/msword, application/vnd.openxmlformats-officedocument.wordprocessingml.document, text/plain, application/zip.
// - Allowed file extensions: .pdf, .doc, .docx, .txt.
// - Caption length must not exceed the configured character limit.
//
// Returns (JSON):
// - 200 OK: If the document was successfully sent.
// - 400 Bad Request: If form data is missing, file is invalid, or recipient is incorrect.
// - 404 Not Found: If the user does not exist in the database.
// - 500 Internal Server Error: If any processing step fails (file I/O, upload, DB, or WhatsApp errors).
func SendDocumentWhatsapp() gin.HandlerFunc {
	return func(c *gin.Context) {
		taskDoing := "Send Document Whatsapp"

		// Check WhatsApp connection
		if WhatsappClient == nil || !WhatsappClient.IsConnected() {
			c.JSON(http.StatusInternalServerError, gin.H{"message": "WhatsApp client not connected"})
			return
		}

		userID := c.PostForm("userId")
		isGroup := c.PostForm("isGroup")
		recipient := c.PostForm("recipient")
		message := c.PostForm("message")
		useFooter := c.PostForm("useFooter")

		// Validate required fields
		if userID == "" || recipient == "" {
			c.JSON(http.StatusBadRequest, gin.H{"message": "Missing required fields"})
			return
		}

		// Validate user
		var userData model.Admin
		result := dbWeb.Where("id = ?", userID).First(&userData)
		if result.Error != nil {
			if result.Error == gorm.ErrRecordNotFound {
				c.JSON(http.StatusNotFound, gin.H{"message": "user not found"})
				return
			}
			c.JSON(http.StatusInternalServerError, gin.H{"message": "failed to query user data", "detail": result.Error.Error()})
			return
		}

		// Get file
		fileHeader, err := c.FormFile("document")
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"message": "Document file is required", "detail": err.Error()})
			return
		}

		// Open and detect MIME type
		file, err := fileHeader.Open()
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"message": "Failed to open document", "detail": err.Error()})
			return
		}
		defer file.Close()

		buf := make([]byte, 512)
		if _, err := file.Read(buf); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"message": "Failed to read document", "detail": err.Error()})
			return
		}
		contentType := http.DetectContentType(buf)

		// Reset pointer
		if _, err := file.Seek(0, io.SeekStart); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"message": "Failed to reset file pointer", "detail": err.Error()})
			return
		}

		// Validate type
		allowedMimeTypes := config.WebPanel.Get().Whatsmeow.DocumentAllowedMimeTypes

		// Extract file extension as fallback validation
		ext := strings.ToLower(filepath.Ext(fileHeader.Filename))
		allowedExtensions := config.WebPanel.Get().Whatsmeow.DocumentAllowedExtensions

		if !contains(allowedMimeTypes, contentType) && !contains(allowedExtensions, ext) {
			c.JSON(http.StatusBadRequest, gin.H{
				"message": "Invalid document type",
				"detail":  fmt.Sprintf("Allowed types are: %v or extensions: %v", allowedMimeTypes, allowedExtensions),
			})
			return
		}

		// Read all bytes
		fileBytes, err := io.ReadAll(file)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"message": "Failed to read document"})
			return
		}

		// Upload to WhatsApp
		uploaded, err := WhatsappClient.Upload(context.Background(), fileBytes, whatsmeow.MediaDocument)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"message": "Upload failed", "detail": err.Error()})
			return
		}

		var sb strings.Builder
		if message != "" {
			if len(message) > config.WebPanel.Get().Default.MaxMessageCharacters {
				c.JSON(http.StatusBadRequest, gin.H{"message": fmt.Sprintf("max chars of message cannot more than %d characters", config.WebPanel.Get().Default.MaxMessageCharacters)})
				return
			}

			sb.WriteString(message)
		}
		if useFooter == "true" {
			sb.WriteString(fmt.Sprintf("\n\n~Regards, *%s*", config.WebPanel.Get().Default.PT))
		}
		textToSend := sb.String()

		// Send the document
		var jid types.JID
		if isGroup == "true" {
			j, err := ValidateGroupJID(recipient)
			if err != nil {
				c.JSON(http.StatusBadRequest, gin.H{"message": err.Error()})
				return
			}
			jid = j
		} else {
			validWAPhoneNumber, err := CheckValidWhatsappPhoneNumber(recipient)
			if err != nil {
				c.JSON(http.StatusBadRequest, gin.H{"message": err.Error()})
				return
			}
			jid = types.NewJID("62"+validWAPhoneNumber, "s.whatsapp.net")
		}
		resp, err := WhatsappClient.SendMessage(context.Background(), jid, &waE2E.Message{
			DocumentMessage: &waE2E.DocumentMessage{
				URL:               proto.String(uploaded.URL),
				DirectPath:        proto.String(uploaded.DirectPath),
				MediaKey:          uploaded.MediaKey,
				Mimetype:          proto.String(contentType),
				Title:             proto.String(fileHeader.Filename),
				FileName:          proto.String(fileHeader.Filename),
				FileSHA256:        uploaded.FileSHA256,
				FileEncSHA256:     uploaded.FileEncSHA256,
				FileLength:        proto.Uint64(uint64(len(fileBytes))),
				MediaKeyTimestamp: proto.Int64(time.Now().Unix()),
				Caption:           &textToSend,
			},
		})
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"message": "Failed to send document", "detail": err.Error()})
			return
		}

		// Insert whatsapp msg to DB
		go func() {
			msg := model.WAMessage{
				ID:          resp.ID,
				ChatJID:     jid.String(),
				SenderJID:   resp.Sender.User + "@" + resp.Sender.Server,
				MessageBody: textToSend,
				MessageType: "document",
				IsGroup:     jid.Server == "g.us",
				Status:      "sent",
				SentAt:      resp.Timestamp,
			}
			if err := dbWeb.Create(&msg).Error; err != nil {
				logrus.Errorf("error while trying to insert msg of %v: %v", taskDoing, err)
			}
		}()

		c.JSON(http.StatusOK, gin.H{"message": fmt.Sprintf("Document successfully sent @%v", resp.Timestamp.Format("15:04:05, 02 Jan 2006"))})
	}
}

// GetWhatsappGroups returns a Gin handler function that retrieves the list of WhatsApp groups
// the client has joined. For each group, it gathers group details including group JID, name,
// owner information (JID, name, avatar), creation time, and a list of participants with their
// JID, name, status, and avatar. The handler responds with a JSON array of group information
// or an error message if the groups cannot be fetched.
func GetWhatsappGroups() gin.HandlerFunc {
	return func(c *gin.Context) {
		groups, err := WhatsappClient.GetJoinedGroups(context.Background())
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"message": "Failed to fetch groups", "detail": err.Error()})
			return
		}

		var output []map[string]interface{}
		for _, group := range groups {
			var participants []map[string]interface{}

			for _, p := range group.Participants {
				jid := p.JID

				userInfos, _ := WhatsappClient.GetUserInfo(context.Background(), []types.JID{jid})
				userInfo := userInfos[jid]

				avatarInfo, _ := WhatsappClient.GetProfilePictureInfo(context.Background(), jid, &whatsmeow.GetProfilePictureParams{
					Preview: false,
				})

				var avatarURL string
				if avatarInfo != nil {
					avatarURL = avatarInfo.URL
				}

				participants = append(participants, map[string]interface{}{
					"JID":    jid.String(),
					"Name":   userInfo.VerifiedName,
					"Status": userInfo.Status,
					"Avatar": avatarURL,
				})
			}

			ownerJID := group.OwnerJID
			ownerInfos, _ := WhatsappClient.GetUserInfo(context.Background(), []types.JID{ownerJID})
			ownerInfo := ownerInfos[ownerJID]

			ownerAvatarInfo, _ := WhatsappClient.GetProfilePictureInfo(context.Background(), ownerJID, &whatsmeow.GetProfilePictureParams{
				Preview: false,
			})

			var ownerAvatarURL string
			if ownerAvatarInfo != nil {
				ownerAvatarURL = ownerAvatarInfo.URL
			}

			output = append(output, map[string]interface{}{
				"JID":          group.JID.String(),
				"Name":         group.Name,
				"OwnerJID":     ownerJID.User,
				"OwnerName":    ownerInfo.VerifiedName,
				"OwnerAvatar":  ownerAvatarURL,
				"GroupCreated": group.GroupCreated.Format("2006-01-02 15:04:05"),
				"Participants": participants,
			})
		}

		c.JSON(http.StatusOK, output)
	}
}

// SendLocationWhatsapp returns a Gin handler function that sends a location or live location message via WhatsApp.
//
// The handler expects the following POST form parameters:
//   - userId:      ID of the user sending the message (required)
//   - isGroup:     "true" if sending to a group, otherwise "false"
//   - recipient:   WhatsApp JID or phone number of the recipient (required)
//   - long:        Longitude of the location (required)
//   - lat:         Latitude of the location (required)
//   - locName:     Name or caption of the location
//   - locAddress:  Address or detail of the location (optional, limited by config)
//   - isLive:      "true" to send a live location, otherwise sends a static location
//
// The handler performs the following steps:
//  1. Validates WhatsApp client connection.
//  2. Validates required fields and user existence.
//  3. Validates and parses latitude and longitude.
//  4. Constructs and sends a WhatsApp location or live location message.
//  5. Stores the sent message in the database asynchronously.
//  6. Returns a JSON response indicating success or failure.
//
// Responses:
//   - 200 OK:      Location sent successfully.
//   - 400 BadRequest: Missing or invalid parameters.
//   - 404 NotFound:  User not found.
//   - 500 InternalServerError: WhatsApp client not connected or other errors.
func SendLocationWhatsapp() gin.HandlerFunc {
	return func(c *gin.Context) {
		taskDoing := "Send Location Whatsapp"

		// Check WhatsApp connection
		if WhatsappClient == nil || !WhatsappClient.IsConnected() {
			c.JSON(http.StatusInternalServerError, gin.H{"message": "WhatsApp client not connected"})
			return
		}

		userID := c.PostForm("userId")
		isGroup := c.PostForm("isGroup")
		recipient := c.PostForm("recipient")
		long := c.PostForm("long")
		lat := c.PostForm("lat")
		locName := c.PostForm("locName")
		locAddress := c.PostForm("locAddress")
		isLive := c.PostForm("isLive")

		// Validate required fields
		if userID == "" || recipient == "" || long == "" || lat == "" {
			c.JSON(http.StatusBadRequest, gin.H{"message": "Missing required fields"})
			return
		}

		if locAddress != "" {
			if len(locAddress) > config.WebPanel.Get().Default.MaxMessageCharacters {
				c.JSON(http.StatusBadRequest, gin.H{"message": fmt.Sprintf("max chars of detail address cannot more than %d characters", config.WebPanel.Get().Default.MaxMessageCharacters)})
				return
			}
		}

		// Validate user
		var userData model.Admin
		if result := dbWeb.Where("id = ?", userID).First(&userData); result.Error != nil {
			if errors.Is(result.Error, gorm.ErrRecordNotFound) {
				c.JSON(http.StatusNotFound, gin.H{"message": "User not found"})
			} else {
				c.JSON(http.StatusInternalServerError, gin.H{"message": "Failed to query user data", "detail": result.Error.Error()})
			}
			return
		}

		accuracy := uint32(10)     // accuracy in meters
		speed := float32(0)        // speed in meters per second
		heading := uint32(0)       // degrees clockwise from magnetic north
		sequenceNumber := int64(1) // message sequence (start from 1)
		timeOffset := uint32(6000) // time offset in seconds or milliseconds since live location started

		// Prepare message
		latFloat, errLat := strconv.ParseFloat(lat, 64)
		longFloat, errLong := strconv.ParseFloat(long, 64)
		if errLat != nil || errLong != nil {
			c.JSON(http.StatusBadRequest, gin.H{"message": "Invalid latitude or longitude"})
			return
		}

		var msgToUse *waE2E.Message
		if isLive == "true" { // Live location
			msgToUse = &waE2E.Message{
				LiveLocationMessage: &waE2E.LiveLocationMessage{
					DegreesLatitude:                   &latFloat,
					DegreesLongitude:                  &longFloat,
					Caption:                           proto.String(locName),
					AccuracyInMeters:                  &accuracy,
					SpeedInMps:                        &speed,
					TimeOffset:                        &timeOffset,
					DegreesClockwiseFromMagneticNorth: &heading,
					SequenceNumber:                    &sequenceNumber,
				},
			}
		} else {
			msgToUse = &waE2E.Message{
				LocationMessage: &waE2E.LocationMessage{
					DegreesLatitude:  &latFloat,
					DegreesLongitude: &longFloat,
					Name:             proto.String(locName),
					Address:          proto.String(locAddress),
				},
			}
		}

		var jid types.JID
		if isGroup == "true" {
			j, err := ValidateGroupJID(recipient)
			if err != nil {
				c.JSON(http.StatusBadRequest, gin.H{"message": err.Error()})
				return
			}
			jid = j
		} else {
			validWAPhoneNumber, err := CheckValidWhatsappPhoneNumber(recipient)
			if err != nil {
				c.JSON(http.StatusBadRequest, gin.H{"message": err.Error()})
				return
			}
			jid = types.NewJID("62"+validWAPhoneNumber, "s.whatsapp.net")
		}
		resp, err := WhatsappClient.SendMessage(context.Background(), jid, msgToUse)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"message": err.Error()})
			return
		}

		// Insert whatsapp msg to DB
		go func() {
			msg := model.WAMessage{
				ID:          resp.ID,
				ChatJID:     jid.String(),
				SenderJID:   resp.Sender.User + "@" + resp.Sender.Server,
				MessageBody: locName + " - " + locAddress,
				MessageType: "location",
				IsGroup:     jid.Server == "g.us",
				Status:      "sent",
				SentAt:      resp.Timestamp,
			}
			if err := dbWeb.Create(&msg).Error; err != nil {
				logrus.Errorf("error while trying to insert msg of %v: %v", taskDoing, err)
			}
		}()

		c.JSON(http.StatusOK, gin.H{"message": fmt.Sprintf("Location sent successfully @%v", resp.Timestamp.Format("15:04:05, 02 Jan 2006"))})
	}
}

// SendPollingWhatsapp returns a Gin handler function that processes a request to send a WhatsApp poll message.
//
// The handler performs the following steps:
//   - Checks if the WhatsApp client is connected.
//   - Parses and validates required POST form fields: userId, recipient, question, options, isGroup, and onlyOneSelection.
//   - Validates the user by querying the database.
//   - Parses the poll options from a JSON string and ensures at least two options are provided.
//   - Formats the recipient JID, handling both group and individual recipients.
//   - Builds and sends the poll message using the WhatsApp client.
//   - Asynchronously logs the sent message to the database.
//   - Returns appropriate JSON responses for success or error cases.
//
// Expected POST form fields:
//   - userId: string (required) - ID of the admin user sending the poll.
//   - recipient: string (required) - WhatsApp ID or phone number of the recipient.
//   - question: string (required) - The poll question.
//   - options: JSON array string (required) - List of poll options (minimum 2).
//   - isGroup: string ("true"/"false") - Indicates if the recipient is a group.
//   - onlyOneSelection: string ("true"/"false") - If true, only one option can be selected.
//
// Responses:
//   - 200 OK: Poll sent successfully.
//   - 400 Bad Request: Missing or invalid input.
//   - 404 Not Found: User not found.
//   - 500 Internal Server Error: WhatsApp client not connected or other server error.
func SendPollingWhatsapp() gin.HandlerFunc {
	return func(c *gin.Context) {
		taskDoing := "Send Polling Whatsapp"

		// Check WhatsApp connection
		if WhatsappClient == nil || !WhatsappClient.IsConnected() {
			c.JSON(http.StatusInternalServerError, gin.H{"message": "WhatsApp client not connected"})
			return
		}

		userID := c.PostForm("userId")
		isGroup := c.PostForm("isGroup")
		recipient := c.PostForm("recipient")
		question := c.PostForm("question")
		onlyOneSelection := c.PostForm("onlyOneSelection")
		optionsStr := c.PostForm("options")

		// Validate required fields
		if userID == "" || recipient == "" || question == "" || optionsStr == "" {
			c.JSON(http.StatusBadRequest, gin.H{"message": "Missing required fields"})
			return
		}

		// Validate user
		var userData model.Admin
		if result := dbWeb.Where("id = ?", userID).First(&userData); result.Error != nil {
			if errors.Is(result.Error, gorm.ErrRecordNotFound) {
				c.JSON(http.StatusNotFound, gin.H{"message": "User not found"})
			} else {
				c.JSON(http.StatusInternalServerError, gin.H{"message": "Failed to query user data", "detail": result.Error.Error()})
			}
			return
		}

		maxSelections := 0
		if onlyOneSelection == "true" {
			maxSelections = 1
		}

		// Get and parse options JSON string
		var options []string
		if err := json.Unmarshal([]byte(optionsStr), &options); err != nil || len(options) < 2 {
			c.JSON(http.StatusBadRequest, gin.H{
				"message": "Invalid or insufficient poll options (minimum 2 required)",
				"detail":  err.Error(),
			})
			return
		}

		if len(options) == 0 {
			c.JSON(http.StatusBadRequest, gin.H{"message": "no option for polling found"})
			return
		}

		// Format JID
		var jid types.JID
		if isGroup == "true" {
			j, err := ValidateGroupJID(recipient)
			if err != nil {
				c.JSON(http.StatusBadRequest, gin.H{"message": err.Error()})
				return
			}
			jid = j
		} else {
			validWAPhoneNumber, err := CheckValidWhatsappPhoneNumber(recipient)
			if err != nil {
				c.JSON(http.StatusBadRequest, gin.H{"message": err.Error()})
				return
			}
			jid = types.NewJID("62"+validWAPhoneNumber, "s.whatsapp.net")
		}

		// Build the poll message
		pollMessage := WhatsappClient.BuildPollCreation(question, options, maxSelections)
		resp, err := WhatsappClient.SendMessage(context.Background(), jid, pollMessage)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"message": err.Error()})
			return
		}

		// Insert whatsapp msg to DB
		go func() {
			msg := model.WAMessage{
				ID:        resp.ID,
				ChatJID:   jid.String(),
				SenderJID: resp.Sender.User + "@" + resp.Sender.Server,
				MessageBody: fmt.Sprintf(
					"Poll: %s | Options: %s | Max Selections: %d",
					question,
					strings.Join(options, ", "),
					maxSelections,
				),
				MessageType: "polling",
				IsGroup:     jid.Server == "g.us",
				Status:      "sent",
				SentAt:      resp.Timestamp,
			}
			if err := dbWeb.Create(&msg).Error; err != nil {
				logrus.Errorf("error while trying to insert msg of %v: %v", taskDoing, err)
			}
		}()

		c.JSON(http.StatusOK, gin.H{"message": fmt.Sprintf("Polling sent successfully @%v", resp.Timestamp.Format("15:04:05, 02 Jan 2006"))})
	}
}

// GetTbLogMsgReceived returns a Gin handler function that serves log data for DataTables.
// It reads a CSV-formatted log file specified in the configuration, supports searching,
// sorting, and pagination as requested by DataTables AJAX requests.
//
// The handler expects the following query parameters:
//   - draw: Draw counter for DataTables (int)
//   - start: Paging first record indicator (int)
//   - length: Number of records to return (int)
//   - search[value]: Search term (string)
//   - order[0][column]: Column index to sort (int)
//   - order[0][dir]: Sort direction ("asc" or "desc")
//
// The response is a JSON object containing:
//   - draw: Echoed draw counter
//   - recordsTotal: Total number of records
//   - recordsFiltered: Number of records after filtering
//   - data: 2D array of string rows for the current page
//
// Returns HTTP 400 for invalid requests, HTTP 500 for file errors.
func GetTbLogMsgReceived() gin.HandlerFunc {
	return func(c *gin.Context) {
		// Request structures
		type DataTableOrder struct {
			Column int    `form:"order[0][column]"`
			Dir    string `form:"order[0][dir]"`
		}

		type DataTableRequest struct {
			Draw   int            `form:"draw"`
			Start  int            `form:"start"`
			Length int            `form:"length"`
			Search string         `form:"search[value]"`
			Order  DataTableOrder `form:""`
		}

		type DataTableResponse struct {
			Draw            int        `json:"draw"`
			RecordsTotal    int        `json:"recordsTotal"`
			RecordsFiltered int        `json:"recordsFiltered"`
			Data            [][]string `json:"data"`
		}

		// Bind request
		var req DataTableRequest
		if err := c.ShouldBind(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request", "details": err.Error()})
			return
		}

		// Open log file
		logFile := config.WebPanel.Get().Whatsmeow.MsgReceivedLogFile
		file, err := os.Open(logFile)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to open log file"})
			return
		}
		defer file.Close()

		// Read and parse lines
		var allRows [][]string
		var currentRow []string

		scanner := bufio.NewScanner(file)
		for scanner.Scan() {
			line := scanner.Text()
			fields := strings.SplitN(line, ",", 2)
			if len(fields) > 1 && isLogLevel(fields[0]) {
				// save previous row
				if len(currentRow) > 0 {
					allRows = append(allRows, currentRow)
				}
				cols := strings.Split(line, ",")
				for len(cols) < 10 {
					cols = append(cols, "")
				}
				currentRow = cols
			} else {
				// continuation line
				if len(currentRow) > 0 {
					currentRow[len(currentRow)-1] += " " + strings.TrimSpace(line)
				}
			}
		}

		// add last row
		if len(currentRow) > 0 {
			allRows = append(allRows, currentRow)
		}

		if err := scanner.Err(); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to scan log file"})
			return
		}

		// Filter by search
		var matching [][]string
		if req.Search != "" {
			searchLower := strings.ToLower(req.Search)
			for _, row := range allRows {
				for _, col := range row {
					if strings.Contains(strings.ToLower(col), searchLower) {
						matching = append(matching, row)
						break
					}
				}
			}
		} else {
			matching = allRows
		}

		// Sort if ordering is requested
		if req.Order.Column >= 0 && req.Order.Column < 10 {
			colIndex := req.Order.Column
			dir := strings.ToLower(req.Order.Dir)

			sort.SliceStable(matching, func(i, j int) bool {
				a, b := matching[i][colIndex], matching[j][colIndex]
				if dir == "asc" {
					return a < b
				}
				return a > b
			})
		}

		// Paginate
		start := req.Start
		end := start + req.Length
		if start > len(matching) {
			start = len(matching)
		}
		if end > len(matching) {
			end = len(matching)
		}
		paged := matching[start:end]

		// Response
		resp := DataTableResponse{
			Draw:            req.Draw,
			RecordsTotal:    len(allRows),
			RecordsFiltered: len(matching),
			Data:            paged,
		}

		c.JSON(http.StatusOK, resp)
	}
}

func haloFromBot(v *events.Message, stanzaID, originalSenderJID string) {
	taskDoing := "Halo From Bot"
	var sb strings.Builder

	sender := v.Info.Sender.String()
	number := strings.Split(sender, ":")[0] // Remove device-specific suffix
	number = strings.ReplaceAll(number, "@s.whatsapp.net", "")

	sb.WriteString("Halo!")

	senderName := v.Info.PushName
	if senderName == "" {
		senderName = number
	} else {
		senderName = fmt.Sprintf("*%v*", senderName)
	}

	sb.WriteString(fmt.Sprintf(" %v\n", senderName))

	loc, _ := time.LoadLocation(config.WebPanel.Get().Default.Timezone)
	now := time.Now().In(loc)

	tgl, err := tanggal.Papar(now, "Jakarta", tanggal.WIB)
	if err != nil {
		logrus.Errorf("failed to parse indonesiaan time zone: %v", err)
		return
	}

	formattedDate := tgl.Format(" ", []tanggal.Format{
		tanggal.NamaHariDenganKoma, // e.g., "Jumat,"
		tanggal.Hari,               // e.g., "07"
		tanggal.NamaBulan,          // e.g., "Maret"
		tanggal.Tahun,              // e.g., "2025"
	})

	// Format time in 12-hour format with AM/PM
	formattedTime := now.Format("03:04 PM")

	message := fmt.Sprintf("Sekarang pukul %s dan hari ini adalah %s. \n", formattedTime, formattedDate)
	sb.WriteString(message)

	weatherApiKey := config.WebPanel.Get().Whatsmeow.OpenWeatherMapAPIKey
	w, err := openweathermap.NewCurrent("C", "EN", weatherApiKey)
	if err != nil {
		log.Print(err)
		return
	}

	botLocation := "Jakarta"
	w.CurrentByName(botLocation)

	sb.WriteString("\n")
	sb.WriteString(fmt.Sprintf("🌆 *%s*\n", botLocation))
	sb.WriteString(fmt.Sprintf("🌡️ Suhu: %.2f°C\n", w.Main.Temp))
	sb.WriteString(fmt.Sprintf("💧 Kelembaban: %d%%\n", w.Main.Humidity))
	sb.WriteString(fmt.Sprintf("💨 Kecepatan Angin: %.2f m/s\n", w.Wind.Speed))
	sb.WriteString("\nTetap semangat dan jaga kesehatan! 💪")
	sb.WriteString("\n✨ _Semoga harimu menyenangkan!_ 😊")

	textToSend := sb.String()
	quotedMsg := &waE2E.ContextInfo{
		StanzaID:      &stanzaID,
		Participant:   &originalSenderJID,
		QuotedMessage: v.Message,
	}

	resp, err := WhatsappClient.SendMessage(context.Background(), v.Info.Chat, &waE2E.Message{
		ExtendedTextMessage: &waE2E.ExtendedTextMessage{
			Text:        &textToSend,
			ContextInfo: quotedMsg,
		},
	})
	if err != nil {
		logrus.Errorf("%s: %v\n", taskDoing, err)
	}
	// Insert whatsapp msg to DB
	go func() {
		msg := model.WAMessage{
			ID:          resp.ID,
			ChatJID:     v.Info.Chat.String(),
			SenderJID:   resp.Sender.User + "@" + resp.Sender.Server,
			MessageBody: textToSend,
			MessageType: "text",
			IsGroup:     v.Info.Chat.Server == "g.us",
			Status:      "sent",
			SentAt:      resp.Timestamp,
		}
		if err := dbWeb.Create(&msg).Error; err != nil {
			logrus.Errorf("error while trying to insert msg of %v: %v", taskDoing, err)
		}
	}()
}

func sendWhatsAppMessageWithStanza(v *events.Message, stanzaID, originalSenderJID, message string) {
	// sender := v.Info.Sender.String()
	// number := strings.Split(sender, ":")[0] // Remove device-specific suffix
	// number = strings.ReplaceAll(number, "@s.whatsapp.net", "")

	quotedMsg := &waE2E.ContextInfo{
		StanzaID:      &stanzaID,
		Participant:   &originalSenderJID,
		QuotedMessage: v.Message,
	}

	_, err := WhatsappClient.SendMessage(context.Background(), v.Info.Chat, &waE2E.Message{
		ExtendedTextMessage: &waE2E.ExtendedTextMessage{
			Text:        &message,
			ContextInfo: quotedMsg,
		},
	})

	if err != nil {
		logrus.Error(err)
		return
	}
}

func SendExcelFileWithStanza(v *events.Message, stanzaID, originalSenderJID, filePath, caption string, mentions []string, userLang string) {
	// Read file contents
	fileData, err := os.ReadFile(filePath)
	if err != nil {
		logrus.Errorf("Failed to read file %s: %v", filePath, err)
		return
	}

	// Get file info
	fileInfo, err := os.Stat(filePath)
	if err != nil {
		logrus.Errorf("Failed to stat file %s: %v", filePath, err)
		return
	}

	// Upload to WhatsApp servers
	uploaded, err := WhatsappClient.Upload(context.Background(), fileData, whatsmeow.MediaDocument)
	if err != nil {
		logrus.Errorf("Failed to upload file %s: %v", filePath, err)
		return
	}

	if uploaded.URL == "" || uploaded.DirectPath == "" {
		logrus.Error("Upload failed: missing URL or DirectPath")
		return
	}

	fileName := fileInfo.Name()
	var taggedMessage string
	var mentionJIDs []string

	if len(mentions) > 0 {
		for _, num := range mentions {
			jid := num + "@s.whatsapp.net"
			mentionJIDs = append(mentionJIDs, jid)
		}
		// Create @mention tags for caption
		var mentionTags []string
		for _, num := range mentions {
			mentionTags = append(mentionTags, "@"+num)
		}
		taggedMessage = fmt.Sprintf("%s\n\nCC: %s", caption, strings.Join(mentionTags, " "))
	} else {
		taggedMessage = caption
	}

	// Send document
	quotedMsg := &waE2E.ContextInfo{
		StanzaID:      &stanzaID,
		Participant:   &originalSenderJID,
		QuotedMessage: v.Message,
		MentionedJID:  mentionJIDs,
	}

	_, err = WhatsappClient.SendMessage(context.Background(), v.Info.Chat, &waE2E.Message{
		DocumentMessage: &waE2E.DocumentMessage{
			Caption:           proto.String(taggedMessage),
			FileName:          proto.String(fileName),
			Mimetype:          proto.String("application/vnd.openxmlformats-officedocument.spreadsheetml.sheet"),
			URL:               proto.String(uploaded.URL),
			DirectPath:        proto.String(uploaded.DirectPath),
			FileSHA256:        uploaded.FileSHA256,
			FileEncSHA256:     uploaded.FileEncSHA256,
			FileLength:        &uploaded.FileLength,
			MediaKey:          uploaded.MediaKey,
			MediaKeyTimestamp: proto.Int64(time.Now().Unix()),
			ContextInfo:       quotedMsg,
		},
	})
	if err != nil {
		logrus.Errorf("Failed to send file to JID %s: %v", originalSenderJID, err)
		return
	}
}

func SendImgFileWithStanza(v *events.Message, stanzaID, originalSenderJID, imgPath, imgCaption string, mentions []string, userLang string) {
	// Read file contents
	fileData, err := os.ReadFile(imgPath)
	if err != nil {
		logrus.Errorf("Failed to read image file %s: %v", imgPath, err)
		return
	}

	// Get file info
	fileInfo, err := os.Stat(imgPath)
	if err != nil {
		logrus.Errorf("Failed to stat image file %s: %v", imgPath, err)
		return
	}

	// Check max file size for WhatsApp image upload (~16 MB)
	const maxWhatsAppImgSize = 16 * 1024 * 1024 // 16 MB in bytes
	if fileInfo.Size() > maxWhatsAppImgSize {
		logrus.Errorf("Image file %s is too large: %d bytes (max allowed is %d bytes)", imgPath, fileInfo.Size(), maxWhatsAppImgSize)
		return
	}

	// Upload to WhatsApp servers
	uploaded, err := WhatsappClient.Upload(context.Background(), fileData, whatsmeow.MediaImage)
	if err != nil {
		logrus.Errorf("Failed to upload image file %s: %v", imgPath, err)
		return
	}
	if uploaded.URL == "" || uploaded.DirectPath == "" {
		logrus.Error("Upload failed: missing URL or DirectPath for image")
		return
	}

	// // Generate thumbnail (very small JPEG)
	// imgThumb, err := generateThumbnail(fileData)
	// if err != nil {
	// 	logrus.Errorf("Failed to generate thumbnail: %v", err)
	// 	return
	// }

	// Build @mention tags
	var taggedMessage string
	var mentionJIDs []string
	if len(mentions) > 0 {
		for _, num := range mentions {
			jid := num + "@s.whatsapp.net"
			mentionJIDs = append(mentionJIDs, jid)
		}
		var mentionTags []string
		for _, num := range mentions {
			mentionTags = append(mentionTags, "@"+num)
		}
		taggedMessage = fmt.Sprintf("%s\n\nCC: %s", imgCaption, strings.Join(mentionTags, " "))
	} else {
		taggedMessage = imgCaption
	}

	// Prepare quoted message
	quotedMsg := waE2E.ContextInfo{
		StanzaID:      &stanzaID,
		Participant:   &originalSenderJID,
		QuotedMessage: v.Message,
		MentionedJID:  mentionJIDs,
	}

	// Send image message
	_, err = WhatsappClient.SendMessage(context.Background(), v.Info.Chat, &waE2E.Message{
		ImageMessage: &waE2E.ImageMessage{
			Caption:           proto.String(taggedMessage),
			Mimetype:          proto.String("image/jpeg"), // adjust if sending png
			URL:               proto.String(uploaded.URL),
			DirectPath:        proto.String(uploaded.DirectPath),
			FileSHA256:        uploaded.FileSHA256,
			FileEncSHA256:     uploaded.FileEncSHA256,
			FileLength:        &uploaded.FileLength,
			MediaKey:          uploaded.MediaKey,
			MediaKeyTimestamp: proto.Int64(time.Now().Unix()),
			ContextInfo:       &quotedMsg,
		},
	})
	if err != nil {
		logrus.Errorf("Failed to send image file to JID %s: %v", originalSenderJID, err)
		return
	}
}

// func generateThumbnail(data []byte) ([]byte, error) {
// 	img, _, err := image.Decode(bytes.NewReader(data))
// 	if err != nil {
// 		return nil, err
// 	}

// 	// Resize to ~100px width (keep aspect)
// 	thumb := resize.Resize(100, 0, img, resize.Lanczos3)

// 	var buf bytes.Buffer
// 	if err := jpeg.Encode(&buf, thumb, &jpeg.Options{Quality: 60}); err != nil {
// 		return nil, err
// 	}
// 	return buf.Bytes(), nil
// }

func informUserRequestReceived(eventToDo string) (id string, en string) {
	id = fmt.Sprintf("Permintaan Anda terkait *%s* telah kami terima dan akan diproses.\nMohon bersabar, karena mungkin akan butuh waktu untuk memproses datanya 😊", eventToDo)
	en = fmt.Sprintf("Your request regarding to *%s* has been received and will be processed.\nPlease be patient, as it may take some time to process the data 😊", eventToDo)
	return
}

func isLogLevel(s string) bool {
	s = strings.ToLower(strings.TrimSpace(s))
	switch s {
	case "panic", "fatal", "error", "warn", "info", "debug", "trace":
		return true
	default:
		return false
	}
}

func SendLangDocumentViaBotWhatsapp(jid, idMsg, enMsg, lang, filePath string) {
	userLang, err := GetUserLang(jid)
	if err != nil {
		userLang = lang
	}
	if userLang != "" {
		lang = userLang
	}

	var msg string
	switch lang {
	case "id":
		msg = idMsg
	default:
		msg = enMsg
	}
	SendDocumentViaBotWhatsapp(jid, msg, filePath)
}

func SendDocumentViaBotWhatsapp(jid, msg, filePath string) {
	parsedJID, err := types.ParseJID(jid)
	if err != nil {
		logrus.Errorf("Failed to parse JID: %v\n", err)
		return
	}

	userJID := types.JID{
		User:   parsedJID.User,
		Server: parsedJID.Server,
	}

	fileData, err := os.ReadFile(filePath)
	if err != nil {
		logrus.Errorf("Failed to read file %s: %v", filePath, err)
		return
	}

	fileInfo, err := os.Stat(filePath)
	if err != nil {
		logrus.Errorf("Failed to stat file %s: %v", filePath, err)
		return
	}

	if fileInfo.Size() == 0 {
		logrus.Errorf("File %s is empty", filePath)
		return
	}

	// Detect MIME type
	mimeType := mime.TypeByExtension(strings.ToLower(filepath.Ext(filePath)))
	if mimeType == "" {
		mimeType = "application/octet-stream" // default fallback
	}

	fileName := filepath.Base(filePath)

	uploaded, err := WhatsappClient.Upload(context.Background(), fileData, whatsmeow.MediaDocument)
	if err != nil {
		logrus.Errorf("Failed to upload file %s: %v", filePath, err)
		return
	}

	if uploaded.URL == "" || uploaded.DirectPath == "" {
		logrus.Error("Upload failed: missing URL or DirectPath")
		return
	}

	resp, err := WhatsappClient.SendMessage(context.Background(), userJID, &waE2E.Message{
		DocumentMessage: &waE2E.DocumentMessage{
			Caption:           proto.String(msg),
			FileName:          proto.String(fileName),
			Mimetype:          proto.String(mimeType),
			URL:               proto.String(uploaded.URL),
			DirectPath:        proto.String(uploaded.DirectPath),
			FileSHA256:        uploaded.FileSHA256,
			FileEncSHA256:     uploaded.FileEncSHA256,
			FileLength:        &uploaded.FileLength,
			MediaKey:          uploaded.MediaKey,
			MediaKeyTimestamp: proto.Int64(time.Now().Unix()),
		},
	})
	if err != nil {
		logrus.Errorf("Failed to send file to JID %s: %v", jid, err)
		return
	}

	logrus.Infof("Document %s sent @%v to %s", fileName, resp.Timestamp.Format("15:04:05, 02 Jan 2006"), jid)

	// Insert whatsapp msg to DB
	go func() {
		dbWeb := gormdb.Databases.Web
		msg := model.WAMessage{
			ID:          resp.ID,
			ChatJID:     userJID.String(),
			SenderJID:   resp.Sender.User + "@" + resp.Sender.Server,
			MessageBody: msg,
			MessageType: "document",
			IsGroup:     userJID.Server == "g.us",
			Status:      "sent",
			SentAt:      resp.Timestamp,
		}
		if err := dbWeb.Create(&msg).Error; err != nil {
			logrus.Errorf("error while trying to insert msg of SendDocumentViaBotWhatsapp: %v", err)
		}
	}()
}

func SendWhatsappMessageWithMentionsbyTA(jid types.JID, mentions []string, message string) {
	dbWebTA := gormdb.Databases.WebTA

	mentionJIDsStr := strings.Join(mentions, ", ")

	// No mentions: send simple message
	if len(mentions) == 0 {
		taggedMessage := fmt.Sprintf("%s\n\n~ Regards, Technical Assistance Team *%v*", message, config.WebPanel.Get().Default.PT)
		resp, err := WhatsappClient.SendMessage(context.Background(), jid, &waE2E.Message{
			Conversation: &taggedMessage,
		})
		if err != nil {
			logrus.Errorf("Failed to send message to JID %s: %v", jid, err)
			return
		}

		// Insert whatsapp msg to DB TA
		go func() {
			sentAt := resp.Timestamp
			if sentAt.IsZero() {
				sentAt = time.Now()
			}

			msg := tamodel.WAMessage{
				ID:          resp.ID,
				ChatJID:     jid.String(),
				SenderJID:   resp.Sender.User + "@" + resp.Sender.Server,
				Mentions:    mentionJIDsStr,
				MessageBody: taggedMessage,
				MessageType: "text",
				StanzaID:    "",
				IsGroup:     jid.Server == "g.us",
				Status:      "sent",
				SentAt:      sentAt,
			}

			if err := dbWebTA.Create(&msg).Error; err != nil {
				logrus.Errorf("error while trying to insert msg of SendWhatsappMessageWithMentionsbyTA: %v", err)
			}
		}()
	}

	// If mentions exist, build tagged message
	var mentionedJIDs []string
	var mentionTags []string
	for _, num := range mentions {
		jid := num + "@s.whatsapp.net"
		mentionedJIDs = append(mentionedJIDs, jid)
		mentionTags = append(mentionTags, "@"+num)
	}
	taggedMessage := fmt.Sprintf("%s\n\nCc: %s\n\n~ Regards, Technical Assistance Team *%v*", message, strings.Join(mentionTags, " "), config.WebPanel.Get().Default.PT)

	resp, err := WhatsappClient.SendMessage(context.Background(), jid, &waE2E.Message{
		ExtendedTextMessage: &waE2E.ExtendedTextMessage{
			Text: &taggedMessage,
			ContextInfo: &waE2E.ContextInfo{
				MentionedJID: mentionedJIDs,
			},
		},
	})

	if err != nil {
		logrus.Errorf("Failed to send message with mentions to JID %s: %v", jid, err)
		return
	}

	// Insert whatsapp msg to DB TA
	go func() {
		sentAt := resp.Timestamp
		if sentAt.IsZero() {
			sentAt = time.Now()
		}

		msg := tamodel.WAMessage{
			ID:          resp.ID,
			ChatJID:     jid.String(),
			SenderJID:   resp.Sender.User + "@" + resp.Sender.Server,
			Mentions:    mentionJIDsStr,
			MessageBody: taggedMessage,
			MessageType: "text",
			StanzaID:    "",
			IsGroup:     jid.Server == "g.us",
			Status:      "sent",
			SentAt:      sentAt,
		}

		if err := dbWebTA.Create(&msg).Error; err != nil {
			logrus.Errorf("error while trying to insert msg of SendWhatsappMessageWithMentionsbyTA: %v", err)
		}
	}()
}

// ConvertPhoneNumbersToJIDs converts an array of phone numbers to WhatsApp JIDs
// Parameters:
//   - phoneNumbers: slice of phone numbers (e.g., ["6281234567890", "6289876543210"])
//
// Returns:
//   - slice of normalized JID strings ready for sending WhatsApp messages
//   - returns nil if phoneNumbers is empty
func ConvertPhoneNumbersToJIDs(phoneNumbers []string) []string {
	if len(phoneNumbers) == 0 {
		return nil
	}

	jids := make([]string, 0, len(phoneNumbers))
	for _, phone := range phoneNumbers {
		jid := types.NewJID(phone, "s.whatsapp.net")
		jids = append(jids, NormalizeSenderJID(jid.String()))
	}
	return jids
}
