package controllers

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"service-platform/cmd/technical_assistance/database"
	"service-platform/cmd/technical_assistance/fun"
	"service-platform/cmd/technical_assistance/model"
	"service-platform/cmd/technical_assistance/model/op_model"
	"service-platform/cmd/technical_assistance/ws"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/go-redis/redis/v8"
	"github.com/gorilla/websocket"
	"gorm.io/gorm"
)

func WebSocketVerify(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		cookies := c.Request.Cookies()
		// Parse JWT token from cookie
		tokenString, err := c.Cookie("token")
		if err != nil {
			fun.ClearCookiesAndRedirect(c, cookies)
			return
		}
		tokenString = strings.ReplaceAll(tokenString, " ", "+")

		decrypted, err := fun.GetAESDecrypted(tokenString)
		if err != nil {
			fmt.Println("Error during decryption", err)
			fun.ClearCookiesAndRedirect(c, cookies)
			return
		}
		var claims map[string]interface{}
		err = json.Unmarshal(decrypted, &claims)
		if err != nil {
			fmt.Printf("Error converting JSON to map: %v", err)
			fun.ClearCookiesAndRedirect(c, cookies)
			return
		}
		var admin model.Admin
		if err := db.Where("id = ? AND session = ?", claims["id"], claims["session"].(string)).First(&admin).Error; err != nil {
			fun.ClearCookiesAndRedirect(c, cookies)
			return
		}

		ws.HandleWebSocket(c.Writer, c.Request, admin.Email+fun.GenerateRandomString(10), db)
	}
}

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool {
		return true
	},
}

func WebSocketRealtime() gin.HandlerFunc {
	return func(c *gin.Context) {
		db, err := database.InitAndCheckDB(
			os.Getenv("MYSQL_USER_DB_KONFIRMASI_PENGERJAAN"),
			os.Getenv("MYSQL_PASS_DB_KONFIRMASI_PENGERJAAN"),
			os.Getenv("MYSQL_HOST_DB_KONFIRMASI_PENGERJAAN"),
			os.Getenv("MYSQL_PORT_DB_KONFIRMASI_PENGERJAAN"),
			os.Getenv("MYSQL_NAME_DB_KONFIRMASI_PENGERJAAN"),
		)
		if err != nil {
			log.Fatalf("WS Realtime Database setup failed: %v", err)
		}

		table := c.Query("data")
		if table == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Missing 'table' query parameter"})
			return
		}

		conn, err := upgrader.Upgrade(c.Writer, c.Request, nil)
		if err != nil {
			log.Println("WebSocket Realtime upgrade error:", err)
			return
		}
		defer conn.Close()

		var lastID int
		for {
			time.Sleep(2 * time.Second) // Check database every 2 seconds

			var latestID int
			query := fmt.Sprintf("SELECT MAX(id_task) FROM %s", table) // Use dynamic table name
			err := db.Raw(query).Scan(&latestID).Error
			if err != nil {
				// log.Println("WS Realtime Database error:", err)
				continue
			}

			if latestID > lastID {
				lastID = latestID

				// Struct to hold the latest data
				var latestData interface{}

				// Switch case to map tables to models
				switch table {
				case "error":
					var data op_model.Error
					if err := db.Where("id_task = ?", latestID).First(&data).Error; err == nil {
						latestData = data
					}
				case "pending":
					var data op_model.Pending
					if err := db.Where("id_task = ?", latestID).First(&data).Error; err == nil {
						latestData = data
					}
				case "temp_submission":
					var data op_model.TempSubmission
					if err := db.Where("id_task = ?", latestID).First(&data).Error; err == nil {
						latestData = data
					}
				default:
					log.Println("WS Realtime: Unknown table")
					continue
				}

				// Convert struct to JSON
				jsonData, err := json.Marshal(latestData)
				if err != nil {
					log.Println("WS Realtime JSON encoding error:", err)
					continue
				}

				// Send WebSocket message
				err = conn.WriteMessage(websocket.TextMessage, jsonData)
				if err != nil {
					log.Println("WebSocket Realtime write error:", err)
					return
				}
			}
		}
	}
}

var upgraderWSLock = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool {
		return true
	},
}

var clientsLockRow = make(map[*websocket.Conn]bool) // Track connected clients
var clientsLockRowMutex = &sync.Mutex{}             // Mutex for thread safety

func WebSocketLockData(redisDB *redis.Client, db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		conn, err := upgraderWSLock.Upgrade(c.Writer, c.Request, nil)
		if err != nil {
			log.Println("WebSocket LockData upgrade error:", err)
			return
		}
		defer conn.Close()

		// Add client to map
		clientsLockRowMutex.Lock()
		clientsLockRow[conn] = true
		clientsLockRowMutex.Unlock()

		defer func() {
			// Remove client from map when disconnected
			clientsLockRowMutex.Lock()
			delete(clientsLockRow, conn)
			clientsLockRowMutex.Unlock()
		}()

		for {
			var msg struct {
				Action         string `json:"action"` // "lock" or "unlock"
				DatatableClass string `json:"dt_class"`
				RowID          string `json:"row_id"`
				UserID         string `json:"user_id"`
				UserName       string `json:"user_name"`
			}

			// Read JSON message from client
			err := conn.ReadJSON(&msg)
			if err != nil {
				log.Println("WebSocket ReadJSON error:", err)
				break
			}

			lockKey := fmt.Sprintf("row_lock:%v", msg.RowID)

			if msg.Action == "lock" {
				// Store UserID and Timestamp in Redis
				success, err := redisDB.SetNX(c, lockKey, msg.UserID, 60*time.Second).Result()
				if err != nil {
					log.Println("Redis error:", err)
					continue
				}

				intID, err := strconv.Atoi(msg.RowID)
				if err != nil {
					log.Println("Error converting RowID to int:", err)
					continue
				}

				switch strings.ToLower(msg.DatatableClass) {
				case "dt_teknisi_pengerjaan_pending":
					// Always update date_on_check to current time, regardless of its previous value
					if err := db.Model(&op_model.Pending{}).Where("id_task = ?", intID).Update("date_on_check", time.Now()).Error; err != nil {
						log.Println("Error updating date_on_check:", err)
						continue
					}
				case "dt_teknisi_pengerjaan_error":
					// Always update date_on_check to current time, regardless of its previous value
					if err := db.Model(&op_model.Error{}).Where("id_task = ?", intID).Update("date_on_check", time.Now()).Error; err != nil {
						log.Println("Error updating date_on_check:", err)
						continue
					}
				case "dt_teknisi_pengerjaan_submission":
					// Always update date_on_check to current time, regardless of its previous value
					if err := db.Model(&op_model.TempSubmission{}).Where("id_task = ?", intID).Update("date_on_check", time.Now()).Error; err != nil {
						log.Println("Error updating date_on_check:", err)
						continue
					}
				}

				if !success {
					// Notify user row is already locked
					conn.WriteJSON(map[string]string{"error": "Row is already locked"})
					continue
				}
			} else if msg.Action == "unlock" {
				val, _ := redisDB.Get(c, lockKey).Result()
				if val == msg.UserID {
					redisDB.Del(c, lockKey)
				}
			}

			// Broadcast lock/unlock event to all clients
			broadcastLockState(msg)
		}
	}
}

// **Broadcast function**
func broadcastLockState(msg struct {
	Action         string `json:"action"`
	DatatableClass string `json:"dt_class"`
	RowID          string `json:"row_id"`
	UserID         string `json:"user_id"`
	UserName       string `json:"user_name"`
}) {
	messageJSON, _ := json.Marshal(msg)

	clientsLockRowMutex.Lock()
	defer clientsLockRowMutex.Unlock()

	for client := range clientsLockRow {
		err := client.WriteMessage(websocket.TextMessage, messageJSON)
		if err != nil {
			log.Println("WebSocket WriteMessage error:", err)
			client.Close()
			delete(clientsLockRow, client)
			continue // Continue broadcasting to other clients
		}
	}
}
