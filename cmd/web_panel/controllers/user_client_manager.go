package controllers

import (
	"context"
	"errors"
	"fmt"
	"service-platform/cmd/web_panel/logger"
	"service-platform/cmd/web_panel/model"
	"service-platform/internal/config"
	"sync"
	"time"

	"github.com/go-redis/redis/v8"
	"github.com/sirupsen/logrus"
	"go.mau.fi/whatsmeow"
	"go.mau.fi/whatsmeow/proto/waE2E"
	"go.mau.fi/whatsmeow/store/sqlstore"
	"go.mau.fi/whatsmeow/types"
	"go.mau.fi/whatsmeow/types/events"
	"gorm.io/gorm"
)

// ConnectionState represents the different states of WhatsApp connection
type ConnectionState string

const (
	StateDisconnected   ConnectionState = "disconnected"   // Not connected to servers
	StateConnecting     ConnectionState = "connecting"     // Attempting to connect
	StateConnected      ConnectionState = "connected"      // Connected to servers, waiting for QR scan
	StateQRGenerated    ConnectionState = "qr_generated"   // QR code is available for scanning
	StateAuthenticating ConnectionState = "authenticating" // QR scanned, waiting for confirmation
	StateAuthenticated  ConnectionState = "authenticated"  // Fully logged in and ready
	StateReconnecting   ConnectionState = "reconnecting"   // Attempting to reconnect
	StateError          ConnectionState = "error"          // Error state
)

// UserClientWhatsapp represents a WhatsApp client for a specific user
type UserClientWhatsapp struct {
	UserID          uint
	PhoneNumber     string
	Client          *whatsmeow.Client
	State           ConnectionState // Current connection state
	IsConnected     bool            // Connected to WhatsApp servers
	IsAuthenticated bool            // Authenticated with phone (QR scanned)
	LastUsed        time.Time
	QRCode          string
	QRChannel       chan string        // Channel for QR code updates
	StatusChan      chan ClientStatus  // Channel for connection status updates
	CommandChan     chan ClientCommand // Channel for client commands
	StopChan        chan bool          // Channel to stop the client
	db              *gorm.DB           // Database connection for conversation storage
	mutex           sync.RWMutex
}

// ClientStatus represents the connection status of a client
type ClientStatus struct {
	UserID          uint            `json:"user_id"`
	State           ConnectionState `json:"state"`
	IsConnected     bool            `json:"is_connected"`     // Connected to servers
	IsAuthenticated bool            `json:"is_authenticated"` // Authenticated with phone
	HasQRCode       bool            `json:"has_qr_code"`      // QR code is available
	Message         string          `json:"message"`
	PhoneNumber     string          `json:"phone_number,omitempty"` // Phone number if authenticated
	LastSeen        time.Time       `json:"last_seen"`
	Timestamp       time.Time       `json:"timestamp"`
}

// ClientCommand represents commands that can be sent to a client
type ClientCommand struct {
	Type    CommandType
	Payload interface{}
	Result  chan interface{}
}

type CommandType string

const (
	CommandConnect    CommandType = "connect"
	CommandDisconnect CommandType = "disconnect"
	CommandGetQR      CommandType = "get_qr"
	CommandRefreshQR  CommandType = "refresh_qr"
	CommandSendText   CommandType = "send_text"
	CommandGetStatus  CommandType = "get_status"
)

// SendTextPayload represents the payload for sending text messages
type SendTextPayload struct {
	Recipient string
	Message   string
	IsGroup   bool
}

// UserClientManager manages multiple WhatsApp clients for different users
type UserClientManager struct {
	clients   map[uint]*UserClientWhatsapp
	mutex     sync.RWMutex
	redisDB   *redis.Client
	db        *gorm.DB
	stopChan  chan bool
	clientTTL time.Duration // Time to keep inactive clients alive
}

var (
	clientManager *UserClientManager
	managerOnce   sync.Once
)

// GetUserClientManager returns the singleton instance of UserClientManager
func GetUserClientManager() *UserClientManager {
	managerOnce.Do(func() {
		clientManager = &UserClientManager{
			clients:   make(map[uint]*UserClientWhatsapp),
			stopChan:  make(chan bool),
			clientTTL: 30 * time.Minute, // Keep clients alive for 30 minutes
		}
	})
	return clientManager
}

// Initialize sets up the UserClientManager with database connections
func (ucm *UserClientManager) Initialize(redisDB *redis.Client, db *gorm.DB) {
	ucm.redisDB = redisDB
	ucm.db = db

	// Start cleanup goroutine
	go ucm.cleanupInactiveClients()
}

// GetOrCreateClient gets an existing client or creates a new one for the user
func (ucm *UserClientManager) GetOrCreateClient(userID uint) (*UserClientWhatsapp, error) {
	ucm.mutex.Lock()
	defer ucm.mutex.Unlock()

	// Check if client already exists
	if client, exists := ucm.clients[userID]; exists {
		client.mutex.Lock()
		client.LastUsed = time.Now()
		client.mutex.Unlock()
		return client, nil
	}

	// Get user info from database
	var user model.WAPhoneUser
	if err := ucm.db.Where("id = ?", userID).First(&user).Error; err != nil {
		return nil, fmt.Errorf("user not found: %v", err)
	}

	// Create new client
	client, err := ucm.createNewClient(userID, user.PhoneNumber)
	if err != nil {
		return nil, fmt.Errorf("failed to create client: %v", err)
	}

	ucm.clients[userID] = client

	// Start client goroutine
	go client.run()

	return client, nil
}

// createNewClient creates a new WhatsApp client for a user
func (ucm *UserClientManager) createNewClient(userID uint, phoneNumber string) (*UserClientWhatsapp, error) {
	clientLogPath := fmt.Sprintf("log/whatsapp_client_%d.log", userID)
	dbLogPath := fmt.Sprintf("log/whatsapp_db_%d.log", userID)

	dbLevelStr := config.WebPanel.Get().Whatsmeow.WhatsmeowDBLogLevel
	clientLevelStr := config.WebPanel.Get().Whatsmeow.WhatsmeowClientLogLevel

	dbLevel := logger.ParseWhatsmeowLogLevel(dbLevelStr)
	clientLevel := logger.ParseWhatsmeowLogLevel(clientLevelStr)

	dbLog := logger.NewWhatsmeowLogger(fmt.Sprintf("Database-User-%d", userID), dbLogPath, dbLevel)
	clientLog := logger.NewWhatsmeowLogger(fmt.Sprintf("Client-User-%d", userID), clientLogPath, clientLevel)

	// Create container with user-specific database
	containerPath := fmt.Sprintf("whatsmeow/whatsapp_user_%d.db", userID)
	container, err := sqlstore.New(
		context.Background(),
		config.WebPanel.Get().Whatsmeow.SqlDriver,
		fmt.Sprintf("file:%s?_foreign_keys=on", containerPath),
		dbLog,
	)
	if err != nil {
		return nil, err
	}

	deviceStore, err := container.GetFirstDevice(context.Background())
	if err != nil {
		return nil, err
	}

	whatsClient := whatsmeow.NewClient(deviceStore, clientLog)

	userClient := &UserClientWhatsapp{
		UserID:          userID,
		PhoneNumber:     phoneNumber,
		Client:          whatsClient,
		State:           StateDisconnected,
		IsConnected:     false,
		IsAuthenticated: false,
		LastUsed:        time.Now(),
		QRChannel:       make(chan string, 10),
		StatusChan:      make(chan ClientStatus, 10),
		CommandChan:     make(chan ClientCommand, 50),
		StopChan:        make(chan bool, 1),
		db:              ucm.db, // Pass database connection
	}

	// Set up event handler
	whatsClient.AddEventHandler(userClient.handleEvent)

	return userClient, nil
}

// run starts the client goroutine for handling commands and events
func (uc *UserClientWhatsapp) run() {
	logrus.Infof("Starting WhatsApp client for user %d", uc.UserID)

	for {
		select {
		case <-uc.StopChan:
			logrus.Infof("Stopping WhatsApp client for user %d", uc.UserID)
			uc.disconnect()
			return

		case cmd := <-uc.CommandChan:
			uc.handleCommand(cmd)

		case <-time.After(1 * time.Minute):
			// Periodic health check
			uc.mutex.Lock()
			uc.LastUsed = time.Now()
			uc.mutex.Unlock()
		}
	}
}

// handleCommand processes commands sent to the client
func (uc *UserClientWhatsapp) handleCommand(cmd ClientCommand) {
	defer func() {
		if cmd.Result != nil {
			close(cmd.Result)
		}
	}()

	switch cmd.Type {
	case CommandConnect:
		result := uc.connect()
		if cmd.Result != nil {
			cmd.Result <- result
		}

	case CommandDisconnect:
		uc.disconnect()
		if cmd.Result != nil {
			cmd.Result <- true
		}

	case CommandGetQR:
		if cmd.Result != nil {
			cmd.Result <- uc.QRCode
		}

	case CommandRefreshQR:
		result := uc.refreshQR()
		if cmd.Result != nil {
			cmd.Result <- result
		}

	case CommandSendText:
		payload, ok := cmd.Payload.(SendTextPayload)
		if !ok {
			if cmd.Result != nil {
				cmd.Result <- errors.New("invalid payload for send text command")
			}
			return
		}
		result := uc.sendText(payload)
		if cmd.Result != nil {
			cmd.Result <- result
		}

	case CommandGetStatus:
		uc.mutex.RLock()
		phoneNumber := ""
		if uc.IsAuthenticated && uc.Client.Store.ID != nil {
			phoneNumber = uc.Client.Store.ID.User
		}

		status := ClientStatus{
			UserID:          uc.UserID,
			State:           uc.State,
			IsConnected:     uc.IsConnected,
			IsAuthenticated: uc.IsAuthenticated,
			HasQRCode:       uc.QRCode != "",
			Message:         string(uc.State),
			PhoneNumber:     phoneNumber,
			LastSeen:        uc.LastUsed,
			Timestamp:       time.Now(),
		}
		uc.mutex.RUnlock()

		if cmd.Result != nil {
			cmd.Result <- status
		}
	}
}

// connect initiates connection for the WhatsApp client
func (uc *UserClientWhatsapp) connect() error {
	uc.mutex.Lock()
	uc.State = StateConnecting
	uc.mutex.Unlock()

	if uc.Client.IsConnected() {
		uc.mutex.Lock()
		uc.IsConnected = true
		// Check if we're already authenticated (logged in before)
		if uc.Client.Store.ID != nil {
			uc.IsAuthenticated = true
			uc.State = StateAuthenticated
		} else {
			uc.State = StateConnected
		}
		uc.mutex.Unlock()
		return nil
	}

	err := uc.Client.Connect()
	if err != nil {
		logrus.Errorf("Failed to connect WhatsApp client for user %d: %v", uc.UserID, err)
		uc.mutex.Lock()
		uc.State = StateError
		uc.mutex.Unlock()
		return err
	}

	uc.mutex.Lock()
	uc.IsConnected = true
	uc.State = StateConnected
	// Don't mark as authenticated until PairSuccess event
	uc.IsAuthenticated = false
	uc.mutex.Unlock()

	// Send status update
	uc.StatusChan <- ClientStatus{
		UserID:          uc.UserID,
		State:           StateConnected,
		IsConnected:     true,
		IsAuthenticated: false,
		Message:         "Connected to WhatsApp servers",
		Timestamp:       time.Now(),
	}

	return nil
}

// disconnect disconnects the WhatsApp client
func (uc *UserClientWhatsapp) disconnect() {
	uc.mutex.Lock()
	defer uc.mutex.Unlock()

	logrus.Infof("Disconnecting WhatsApp client for user %d", uc.UserID)

	// Disconnect from WhatsApp servers
	if uc.Client != nil && uc.Client.IsConnected() {
		logrus.Infof("Disconnecting from WhatsApp servers for user %d", uc.UserID)
		uc.Client.Disconnect()
	}

	// Clear the device session from database if it exists
	if uc.Client != nil && uc.Client.Store != nil {
		logrus.Infof("Clearing device session for user %d", uc.UserID)
		device := uc.Client.Store.ID
		if device != nil {
			// Delete the device session to fully logout
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()

			err := uc.Client.Store.Delete(ctx)
			if err != nil {
				logrus.WithError(err).Errorf("Failed to delete device session for user %d", uc.UserID)
			} else {
				logrus.Infof("Successfully deleted device session for user %d", uc.UserID)
			}
		}
	}

	// Reset client state
	uc.State = StateDisconnected
	uc.IsConnected = false
	uc.IsAuthenticated = false
	uc.PhoneNumber = "" // Clear phone number
	uc.QRCode = ""      // Clear any existing QR code
	uc.LastUsed = time.Now()

	logrus.Infof("WhatsApp client state reset for user %d", uc.UserID)

	// Send status update
	uc.StatusChan <- ClientStatus{
		UserID:          uc.UserID,
		State:           StateDisconnected,
		IsConnected:     false,
		IsAuthenticated: false,
		PhoneNumber:     "",
		Message:         "Disconnected and logged out",
		Timestamp:       time.Now(),
	}
}

// sendText sends a text message using the client
func (uc *UserClientWhatsapp) sendText(payload SendTextPayload) error {
	if !uc.Client.IsConnected() {
		return errors.New("client not connected to WhatsApp servers")
	}

	// Check if authenticated (QR scanned)
	uc.mutex.RLock()
	isAuthenticated := uc.IsAuthenticated
	uc.mutex.RUnlock()

	if !isAuthenticated {
		return errors.New("client not authenticated, please scan QR code")
	}

	var jid types.JID
	if payload.IsGroup {
		// Handle group JID validation
		parsedJID, err := types.ParseJID(payload.Recipient)
		if err != nil {
			return fmt.Errorf("invalid group JID: %v", err)
		}
		jid = parsedJID
	} else {
		// Handle individual phone number
		phoneNumber := payload.Recipient
		if len(phoneNumber) > 2 && phoneNumber[:2] == "62" {
			phoneNumber = phoneNumber[2:] // Remove country code if present
		}
		jid = types.NewJID("62"+phoneNumber, "s.whatsapp.net")
	}

	// Send the message
	_, err := uc.Client.SendMessage(context.Background(), jid, &waE2E.Message{
		Conversation: &payload.Message,
	})

	if err != nil {
		logrus.Errorf("Failed to send message from user %d to %s: %v", uc.UserID, payload.Recipient, err)
		return fmt.Errorf("failed to send message: %v", err)
	}

	logrus.Infof("Message sent successfully from user %d to %s", uc.UserID, payload.Recipient)
	return nil
}

// refreshQR disconnects and reconnects the client to generate a new QR code
func (uc *UserClientWhatsapp) refreshQR() error {
	// If already connected, disconnect first
	if uc.Client.IsConnected() {
		uc.Client.Disconnect()
	}

	// Clear current QR code and reset authentication status
	uc.mutex.Lock()
	uc.QRCode = ""
	uc.State = StateConnecting
	uc.IsConnected = false
	uc.IsAuthenticated = false
	uc.mutex.Unlock()

	// Reconnect to generate new QR
	err := uc.Client.Connect()
	if err != nil {
		logrus.Errorf("Failed to refresh QR for user %d: %v", uc.UserID, err)
		uc.mutex.Lock()
		uc.State = StateError
		uc.mutex.Unlock()
		return fmt.Errorf("failed to refresh QR: %v", err)
	}

	logrus.Infof("QR refresh initiated for user %d", uc.UserID)
	return nil
}

// handleEvent handles WhatsApp events for this specific client
func (uc *UserClientWhatsapp) handleEvent(evt interface{}) {
	switch v := evt.(type) {
	case *events.Message:
		// logrus.Infof("User %d received message from %s: %s", uc.UserID, v.Info.Sender, v.Message.GetConversation())

		// Save message to conversation database
		if uc.db != nil {
			go SaveMessageToConversation(v, uc.UserID, uc.db)
		} else {
			logrus.Error("Database connection is nil, cannot save message to conversation")
		}

	case *events.Receipt:
		switch v.Type {
		case types.ReceiptTypeRead, types.ReceiptTypeReadSelf:
			// case events.ReceiptTypeRead, events.ReceiptTypeReadSelf:
			// logrus.Infof("User %d: Message read by %s", uc.UserID, v.SourceString())

			// Update message status in database
			if uc.db != nil && len(v.MessageIDs) > 0 {
				for _, msgID := range v.MessageIDs {
					go UpdateMessageStatus(msgID, "read", uc.UserID, uc.db)
				}
			}

		case types.ReceiptTypeDelivered:
			// case events.ReceiptTypeDelivered:
			// logrus.Infof("User %d: Message delivered to %s", uc.UserID, v.SourceString())

			// Update message status in database
			if uc.db != nil && len(v.MessageIDs) > 0 {
				for _, msgID := range v.MessageIDs {
					go UpdateMessageStatus(msgID, "delivered", uc.UserID, uc.db)
				}
			}
		}

	case *events.Connected:
		logrus.Infof("User %d WhatsApp client connected to servers", uc.UserID)
		uc.mutex.Lock()
		uc.IsConnected = true
		uc.State = StateConnected
		// Don't set authenticated here - wait for PairSuccess
		uc.mutex.Unlock()

		// Send status update
		uc.StatusChan <- ClientStatus{
			UserID:          uc.UserID,
			State:           StateConnected,
			IsConnected:     true,
			IsAuthenticated: uc.IsAuthenticated, // Keep current auth status
			Message:         "Connected to WhatsApp servers",
			Timestamp:       time.Now(),
		}

	case *events.Disconnected:
		logrus.Infof("User %d WhatsApp client disconnected", uc.UserID)
		uc.mutex.Lock()
		uc.State = StateDisconnected
		uc.IsConnected = false
		uc.IsAuthenticated = false
		uc.mutex.Unlock()

		// Send status update
		uc.StatusChan <- ClientStatus{
			UserID:          uc.UserID,
			State:           StateDisconnected,
			IsConnected:     false,
			IsAuthenticated: false,
			Message:         "Client disconnected",
			Timestamp:       time.Now(),
		}

	case *events.QR:
		logrus.Infof("User %d QR code generated", uc.UserID)
		uc.mutex.Lock()
		uc.QRCode = v.Codes[0]
		uc.State = StateQRGenerated
		uc.mutex.Unlock()

		// Send QR code update
		select {
		case uc.QRChannel <- v.Codes[0]:
		default:
			// Channel is full, skip
		}

	case *events.PairSuccess:
		logrus.Infof("User %d successfully paired with phone", uc.UserID)
		uc.mutex.Lock()
		uc.IsConnected = true
		uc.IsAuthenticated = true // NOW the user is truly authenticated
		uc.State = StateAuthenticated
		uc.QRCode = "" // Clear QR code after successful pairing
		uc.mutex.Unlock()

		// Send status update
		uc.StatusChan <- ClientStatus{
			UserID:          uc.UserID,
			State:           StateAuthenticated,
			IsConnected:     true,
			IsAuthenticated: true,
			Message:         "Successfully paired with phone",
			Timestamp:       time.Now(),
		}

		// Sync WhatsApp contacts and groups to database after successful authentication
		go func() {
			logrus.Infof("Starting automatic contact sync for user %d after authentication", uc.UserID)

			// Wait a bit for the client to fully stabilize
			time.Sleep(2 * time.Second)

			// Sync contacts
			err := SyncWhatsappContactsToDatabase(uc.Client, uc.UserID, uc.db)
			if err != nil {
				logrus.WithError(err).Errorf("Failed to sync contacts for user %d", uc.UserID)
			} else {
				logrus.Infof("Successfully synced contacts for user %d", uc.UserID)
			}

			// Sync groups
			err = SyncWhatsappGroupsToDatabase(uc.Client, uc.UserID, uc.db)
			if err != nil {
				logrus.WithError(err).Errorf("Failed to sync groups for user %d", uc.UserID)
			} else {
				logrus.Infof("Successfully synced groups for user %d", uc.UserID)
			}
		}()

		// default:
		// 	logrus.Debugf("User %d received unhandled event: %T", uc.UserID, v)
	}
}

// SendCommand sends a command to the client and optionally waits for a result
func (uc *UserClientWhatsapp) SendCommand(cmdType CommandType, payload interface{}) (interface{}, error) {
	resultChan := make(chan interface{}, 1)

	cmd := ClientCommand{
		Type:    cmdType,
		Payload: payload,
		Result:  resultChan,
	}

	select {
	case uc.CommandChan <- cmd:
		// Wait for result with timeout
		select {
		case result := <-resultChan:
			return result, nil
		case <-time.After(30 * time.Second):
			return nil, fmt.Errorf("command timed out after %d seconds", 30)
		}
	case <-time.After(5 * time.Second):
		return nil, errors.New("failed to send command to client, channel full or client stopped")
	}
}

// cleanupInactiveClients removes clients that haven't been used recently
func (ucm *UserClientManager) cleanupInactiveClients() {
	ticker := time.NewTicker(5 * time.Minute)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			ucm.mutex.Lock()
			now := time.Now()

			for userID, client := range ucm.clients {
				client.mutex.RLock()
				if now.Sub(client.LastUsed) > ucm.clientTTL {
					client.mutex.RUnlock()

					// Stop the client
					select {
					case client.StopChan <- true:
					default:
					}

					delete(ucm.clients, userID)
					logrus.Infof("Cleaned up inactive client for user %d", userID)
				} else {
					client.mutex.RUnlock()
				}
			}

			ucm.mutex.Unlock()

		case <-ucm.stopChan:
			return
		}
	}
}

// Stop stops the UserClientManager and all managed clients
func (ucm *UserClientManager) Stop() {
	ucm.mutex.Lock()
	defer ucm.mutex.Unlock()

	// Stop all clients
	for _, client := range ucm.clients {
		select {
		case client.StopChan <- true:
		default:
		}
	}

	// Stop cleanup goroutine
	select {
	case ucm.stopChan <- true:
	default:
	}

	ucm.clients = make(map[uint]*UserClientWhatsapp)
}

// GetClient returns a client for a specific user
func (ucm *UserClientManager) GetClient(userID uint) (*UserClientWhatsapp, bool) {
	ucm.mutex.RLock()
	defer ucm.mutex.RUnlock()

	client, exists := ucm.clients[userID]
	return client, exists
}

// ListActiveClients returns a list of all active client user IDs
func (ucm *UserClientManager) ListActiveClients() []uint {
	ucm.mutex.RLock()
	defer ucm.mutex.RUnlock()

	userIDs := make([]uint, 0, len(ucm.clients))
	for userID := range ucm.clients {
		userIDs = append(userIDs, userID)
	}

	return userIDs
}

// RemoveClient removes a client from the manager completely
func (ucm *UserClientManager) RemoveClient(userID uint) {
	ucm.mutex.Lock()
	defer ucm.mutex.Unlock()

	if client, exists := ucm.clients[userID]; exists {
		logrus.Infof("Removing WhatsApp client for user %d from manager", userID)

		// Stop the client goroutine
		select {
		case client.StopChan <- true:
		default:
		}

		// Remove from the map
		delete(ucm.clients, userID)

		logrus.Infof("WhatsApp client for user %d removed from manager", userID)
	}
}
