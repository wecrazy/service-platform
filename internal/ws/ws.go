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
	Upgrader = websocket.Upgrader{
		CheckOrigin: func(r *http.Request) bool {
			return true
		},
	}

	Clients = make(map[string]*websocket.Conn)
	Mutex   = sync.Mutex{}
)

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

// SendMessageToRecipient(1, "the message", "email") //1 is text message, 2 is binary
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
	disconectionTime := config.GetConfig().App.MaxDisconnectionTime
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

func IsClientConnected(clientID string) bool {
	Mutex.Lock()
	defer Mutex.Unlock()

	_, connected := Clients[clientID]
	return connected
}
