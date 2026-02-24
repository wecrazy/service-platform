// Package ws provides WebSocket connection management and message handling.
package ws

import (
	"fmt"
	"net/http"
	"service-platform/internal/config"
	"service-platform/internal/core/model"
	"strings"
	"sync"
	"time"

	"github.com/gorilla/websocket"
	"github.com/sirupsen/logrus"
	"gorm.io/gorm"
)

var (
	// Upgrader is the websocket upgrader used to upgrade HTTP connections to WebSocket.
	Upgrader = websocket.Upgrader{
		CheckOrigin: func(_ *http.Request) bool {
			return true
		},
	}

	// Clients holds all active WebSocket connections, indexed by client ID.
	Clients = make(map[string]*websocket.Conn)
	// Mutex protects concurrent access to the Clients map.
	Mutex = sync.Mutex{}
)

// HandleWebSocket upgrades an HTTP connection to WebSocket and registers the client connection.
func HandleWebSocket(w http.ResponseWriter, r *http.Request, clientID string, db *gorm.DB) {
	conn, err := Upgrader.Upgrade(w, r, nil)
	if err != nil {
		logrus.Println(err)
		return
	}

	Mutex.Lock()
	Clients[clientID] = conn
	Mutex.Unlock()

	go HandleMessages(clientID, conn, db)
}

// HandleMessages reads messages from a WebSocket connection and dispatches them to recipients.
func HandleMessages(clientID string, conn *websocket.Conn, db *gorm.DB) {
	defer func() {
		Mutex.Lock()
		delete(Clients, clientID)
		Mutex.Unlock()

		conn.Close()

		// Start the async check for reconnection
		go checkForReconnection(clientID, db)
	}()

	for {
		messageType, p, err := conn.ReadMessage()
		if err != nil {
			logrus.Println(err)
			return
		}

		logrus.Printf("Received message from %s", p)

		// Handle the message (you can implement your own message format)
		HandleMessage(messageType, p)
	}
}

// HandleMessage parses and routes a single WebSocket message to the intended recipient.
func HandleMessage(messageType int, message []byte) {
	// Example: Assume messages have the format "recipientID:message"
	parts := strings.SplitN(string(message), ":", 2)
	if len(parts) != 2 {
		logrus.Println("Invalid message format:", string(message))
		return
	}

	recipientID := parts[0]
	actualMessage := parts[1]

	// Broadcast the message to the intended recipient
	SendMessageToRecipient(messageType, actualMessage, recipientID)
}

// SendMessageToRecipient sends a WebSocket message to a specific recipient by client ID.
// messageType 1 is text, 2 is binary.
func SendMessageToRecipient(messageType int, message, recipientID string) {
	Mutex.Lock()
	defer Mutex.Unlock()

	if clientConn, ok := Clients[recipientID]; ok {
		if clientConn != nil {
			err := clientConn.WriteMessage(messageType, []byte(message))
			if err != nil {
				logrus.Println(err)
			}
		}
	}
}

// CloseWebsocketConnection closes and removes a client's WebSocket connection by client ID.
func CloseWebsocketConnection(clientID string) {
	Mutex.Lock()
	if clientConn, ok := Clients[clientID]; ok {
		if clientConn != nil {
			clientConn.Close()
		}
		delete(Clients, clientID)
	}
	Mutex.Unlock()
}

func checkForReconnection(clientID string, db *gorm.DB) {
	disconectionTime := config.ServicePlatform.Get().App.MaxDisconnectionTime
	if disconectionTime <= 0 {
		disconectionTime = 10 // default to 10 seconds if not set properly
	}

	timer := time.After(time.Duration(disconectionTime) * time.Second)

	<-timer
	Mutex.Lock()
	_, connected := Clients[clientID]
	Mutex.Unlock()

	if !connected {
		fmt.Printf("🔌 Client %s did not reconnect within %d seconds\n", clientID, disconectionTime)
		updates := map[string]any{
			"Session":        "",
			"SessionExpired": 0,
		}

		if err := db.Model(&model.Users{}).Where("email = ?", clientID).Updates(updates).Error; err != nil {
			return
		}
	}
}

// IsClientConnected reports whether a client with the given ID is currently connected.
func IsClientConnected(clientID string) bool {
	Mutex.Lock()
	defer Mutex.Unlock()

	_, connected := Clients[clientID]
	return connected
}
