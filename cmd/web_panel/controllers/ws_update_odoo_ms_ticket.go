package controllers

import (
	"crypto/tls"
	"encoding/json"
	"fmt"
	"net/http"
	"reflect"
	"service-platform/cmd/web_panel/fun"
	odooms "service-platform/cmd/web_panel/model/odoo_ms"
	"service-platform/internal/config"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
	"github.com/sirupsen/logrus"
	"gorm.io/gorm"
)

// Global HTTP client with connection pooling for better performance
var (
	httpClient *http.Client
	clientOnce sync.Once
)

func getHTTPClient() *http.Client {
	clientOnce.Do(func() {
		transport := &http.Transport{
			MaxIdleConns:        100,
			MaxIdleConnsPerHost: 10,
			IdleConnTimeout:     90 * time.Second,
			TLSClientConfig:     &tls.Config{InsecureSkipVerify: true},
		}

		httpClient = &http.Client{
			Timeout:   30 * time.Second,
			Transport: transport,
		}
	})
	return httpClient
}

var (
	// Global trigger channel
	TriggerProcessUploadedExcelforUpdateTicketODOOMS    = make(chan struct{}, 100) // Buffered channel
	TriggerProcessUploadExcelforCreateNewTicketODOOMS   = make(chan struct{}, 100) // Buffered channel
	TriggerProcessUploadExcelforCreateDataCSNABALost    = make(chan struct{}, 100) // Buffered channel
	TriggerProcessUploadExcelforCreateTechnicianPayslip = make(chan struct{}, 100) // Buffered channel
	TriggerProcessUploadedExcelforUpdateTaskODOOMS      = make(chan struct{}, 100) // Buffered channel

	upgrader      = websocket.Upgrader{CheckOrigin: func(r *http.Request) bool { return true }}
	connections   = make(map[*websocket.Conn]bool)
	connectionsMu sync.RWMutex

	// Mutex map for per-connection write safety
	connMutexes   = make(map[*websocket.Conn]*sync.Mutex)
	connMutexesMu sync.Mutex

	// Track processing files to avoid duplicates and provide state recovery
	processingFiles   = make(map[string]*odooms.UploadedExcelToODOOMS)
	processingFilesMu sync.RWMutex
)

// Message structure for progress updates
type UploadedExcelODOOMSProgress struct {
	OriginalFilename string `json:"ori_filename"`
	Filename         string `json:"filename"`
	Status           string `json:"status"`
	Progress         int    `json:"progress"`
	Logs             string `json:"logs"`
}

// Send current processing state to a specific connection (for reconnection)
func sendCurrentProcessingState(conn *websocket.Conn, db *gorm.DB) {
	processingFilesMu.RLock()
	defer processingFilesMu.RUnlock()

	// Send current processing files
	for _, uploadedExcel := range processingFiles {
		// Get latest progress from database
		var currentRecord odooms.UploadedExcelToODOOMS
		if err := db.First(&currentRecord, uploadedExcel.ID).Error; err == nil {
			message := UploadedExcelODOOMSProgress{
				OriginalFilename: currentRecord.OriginalFilename,
				Filename:         currentRecord.Filename,
				Logs:             currentRecord.Logs,
				Status:           currentRecord.Status,
				Progress: func() int {
					if currentRecord.TotalRow == 0 {
						return 0
					}
					processed := currentRecord.TotalSuccess + currentRecord.TotalFail
					return int((processed * 100) / currentRecord.TotalRow)
				}(),
			}

			jsonMessage, _ := json.Marshal(message)
			safeWriteMessage(conn, websocket.TextMessage, jsonMessage)
		}
	}
}

// Cleanup completed files from processing state periodically
func StartProcessingStateCleanup(db *gorm.DB) {
	go func() {
		ticker := time.NewTicker(5 * time.Minute) // Check every 5 minutes
		defer ticker.Stop()

		for range ticker.C {
			processingFilesMu.Lock()
			for filename, uploadedExcel := range processingFiles {
				var currentRecord odooms.UploadedExcelToODOOMS
				if err := db.First(&currentRecord, uploadedExcel.ID).Error; err == nil {
					// Remove completed files from processing state
					if currentRecord.Status == "Completed" || currentRecord.Status == "Failed" || currentRecord.Status == "Done" {
						delete(processingFiles, filename)
						logrus.Infof("Cleaned up completed file from processing state: %s", filename)
					}
				}
			}
			processingFilesMu.Unlock()
		}
	}()
}

// safeWriteMessage safely writes to a websocket connection with mutex protection
func safeWriteMessage(conn *websocket.Conn, messageType int, data []byte) error {
	connMutexesMu.Lock()
	mu, exists := connMutexes[conn]
	if !exists {
		mu = &sync.Mutex{}
		connMutexes[conn] = mu
	}
	connMutexesMu.Unlock()

	mu.Lock()
	defer mu.Unlock()

	return conn.WriteMessage(messageType, data)
}

// cleanupConnectionMutex removes the mutex for a connection when it's closed
func cleanupConnectionMutex(conn *websocket.Conn) {
	connMutexesMu.Lock()
	delete(connMutexes, conn)
	connMutexesMu.Unlock()
}

func WebSocketODOOMSUpdatedTicket(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		conn, err := upgrader.Upgrade(c.Writer, c.Request, nil)
		if err != nil {
			logrus.Error("WebSocket upgrade error:", err)
			return
		}

		// Store connection
		connectionsMu.Lock()
		connections[conn] = true
		connectionsMu.Unlock()
		logrus.Info("WebSocket connected for updates to ODOO Manage Service")

		// Send current processing state to new connection
		go sendCurrentProcessingState(conn, db)

		// Keep the connection alive until closed by the client
		defer func() {
			connectionsMu.Lock()
			delete(connections, conn)
			connectionsMu.Unlock()
			cleanupConnectionMutex(conn)
			conn.Close()
		}()

		// WebSocket listener with ping/pong for connection health
		conn.SetReadDeadline(time.Now().Add(60 * time.Second))
		conn.SetPongHandler(func(string) error {
			conn.SetReadDeadline(time.Now().Add(60 * time.Second))
			return nil
		})

		// Send ping periodically
		ticker := time.NewTicker(30 * time.Second)
		defer ticker.Stop()

		go func() {
			for range ticker.C {
				if err := safeWriteMessage(conn, websocket.PingMessage, nil); err != nil {
					return
				}
			}
		}()

		for {
			_, _, err := conn.ReadMessage()
			if err != nil {
				logrus.Error("WebSocket read error:", err)
				break
			}
		}
	}
}

/*
	Update Helpdesk.Ticket
*/
// ProcessUploadedExcelofODOOMSMustUpdatedTicket starts workers for UPDATE operations (Template 4)
func ProcessUploadedExcelofODOOMSMustUpdatedTicket(db *gorm.DB) {
	processUploadedExcelWorker(db, OperationConfig{
		TemplateID:            4,
		TriggerChannel:        TriggerProcessUploadedExcelforUpdateTicketODOOMS,
		BroadcastProgressFunc: BroadcastUploadedTicketForTicketUpdateinODOOMSProgress,
		OperationType:         "update",
		RequiredFieldInColumns: []string{
			"id", // First column must be ID for updates
		},
		MinValidColumns: 2,
	})
}

func processCustomTemplateUpdateTicketInODOOMS(rows [][]string, totalSuccess, totalFail *atomic.Int64, uploadedExcel odooms.UploadedExcelToODOOMS, loginCookie []*http.Cookie, db *gorm.DB) string {
	var wg sync.WaitGroup
	sem := make(chan struct{}, config.WebPanel.Get().Default.ConcurrencyLimit)
	logAct := make(map[int]string)
	odooModel := "helpdesk.ticket"
	totalRows := len(rows) - 1

	// Update total row count in database
	db.Model(&uploadedExcel).Update("total_row", totalRows)

	// Check if excel has enough data
	if len(rows) < 2 {
		db.Model(&uploadedExcel).Updates(map[string]interface{}{
			"status": "Failed",
			"log":    "Excel has no data to process!",
		})
		db.Delete(&uploadedExcel)
		return fmt.Sprintf("Excel: %v has no data to process!", uploadedExcel.OriginalFilename)
	}

	// Check if header row has enough columns
	if len(rows[0]) < 3 {
		db.Model(&uploadedExcel).Updates(map[string]interface{}{
			"status": "Failed",
			"log":    "Excel must have at least 3 columns in header!",
		})
		db.Delete(&uploadedExcel)
		return fmt.Sprintf("Excel: %v must have at least 3 columns in header!", uploadedExcel.OriginalFilename)
	}

	// Get ODOO MS ticket fields for validation - USE HELPER
	fieldMap, err := buildFieldMapHelpdeskTicket(db)
	if err != nil {
		return err.Error()
	}

	// Validate required columns in order - USE NEW HELPER
	headerRow := rows[0]
	requiredFields := []string{"id"} // For update, only ID is required in first column
	// isValid, validationError := validateRequiredColumns(headerRow, requiredFields, fieldMap)
	isValid, validationError := validateRequiredColumnsWithType(headerRow, requiredFields, fieldMap)
	if !isValid {
		db.Model(&uploadedExcel).Updates(map[string]interface{}{
			"status": "Failed",
			"log":    fmt.Sprintf("Column validation failed: %s", validationError),
		})
		db.Delete(&uploadedExcel)
		return fmt.Sprintf("Excel: %v - %s", uploadedExcel.OriginalFilename, validationError)
	}

	// columnMapping := mapHeaderColumns(headerRow, fieldMap)
	columnMapping := mapHeaderColumnsWithType(headerRow, fieldMap)

	// Check if we have enough valid columns (ID column + at least 1 other field)
	if len(columnMapping) < 2 {
		db.Model(&uploadedExcel).Updates(map[string]interface{}{
			"status": "Failed",
			"log":    "Excel must have at least 2 valid columns in header!",
		})
		db.Delete(&uploadedExcel)
		return fmt.Sprintf("Excel: %v must have at least 2 valid columns that match ODOO ticket fields (ID + at least 1 other field)!", uploadedExcel.OriginalFilename)
	}

	// Process data rows
	batchSize := 100
	dataRows := rows[1:]

	for batchStart := 0; batchStart < len(dataRows); batchStart += batchSize {
		batchEnd := batchStart + batchSize
		if batchEnd > len(dataRows) {
			batchEnd = len(dataRows)
		}

		for i, row := range dataRows[batchStart:batchEnd] {
			rowIndex := batchStart + i
			// Skip empty rows
			if len(row) == 0 {
				continue
			}

			// Check if row has any data
			hasData := false
			for _, cell := range row {
				if strings.TrimSpace(cell) != "" {
					hasData = true
					break
				}
			}

			if !hasData {
				logMessage := fmt.Sprintf("Row %d: Empty row, skipping", rowIndex+1)
				logrus.Debug(logMessage)
				continue
			}

			sem <- struct{}{}
			wg.Add(1)

			go func(rowIndex int, row []string) {
				defer wg.Done()
				defer func() { <-sem }()

				// Check if we have ID in first column
				if len(row) == 0 || strings.TrimSpace(row[0]) == "" {
					logMessage := fmt.Sprintf("Row %d: Missing ID in first column", rowIndex+1)
					logrus.Debug(logMessage)
					logAct[rowIndex+1] = logMessage
					totalFail.Add(1)
					return
				}

				ticketID := strings.TrimSpace(row[0])

				// Build ODOO parameters based on column mapping
				odooParams := map[string]interface{}{
					"model": odooModel,
					"id":    ticketID, // Always include the ID for update operations
				}

				// Map Excel columns to ODOO fields (skip first column as it's the ID)
				for colIndex, fieldName := range columnMapping {
					if colIndex == 0 {
						// Skip first column as it's already handled as ID
						continue
					}
					if colIndex < len(row) {
						cellValue := strings.TrimSpace(row[colIndex])
						if cellValue != "" {
							// Check if this field is a date/datetime/many2one field and parse it
							lowerHeader := ""
							for headerIndex, headerCol := range headerRow {
								if headerIndex == colIndex {
									lowerHeader = strings.ToLower(strings.TrimSpace(headerCol))
									break
								}
							}

							if fieldInfo, exists := fieldMap[lowerHeader]; exists {
								fieldType := strings.ToLower(fieldInfo.Type)

								switch fieldType {
								case "date", "datetime":
									// Parse date/datetime fields
									parsedDate, err := fun.ParseFlexibleDate(cellValue)
									if err != nil {
										logMessage := fmt.Sprintf("Row %d: Failed to parse date value '%s' for field '%s': %v", rowIndex+1, cellValue, fieldName, err)
										logrus.Warn(logMessage)
										// Use original value if parsing fails
										odooParams[fieldName] = cellValue
									} else {
										// Format according to field type
										if fieldType == "date" {
											odooParams[fieldName] = parsedDate.Format("2006-01-02")
										} else { // datetime
											odooParams[fieldName] = parsedDate.Format("2006-01-02 15:04:05")
										}
										logrus.Debugf("Row %d: Parsed %s field '%s': '%s' -> '%v'", rowIndex+1, fieldType, fieldName, cellValue, odooParams[fieldName])
									}

								case "many2one":
									// Many2one fields expect a single integer ID
									if id, err := strconv.Atoi(cellValue); err == nil {
										odooParams[fieldName] = id
										logrus.Debugf("Row %d: Converted many2one field '%s': '%s' -> %d", rowIndex+1, fieldName, cellValue, id)
									} else {
										logMessage := fmt.Sprintf("Row %d: many2one field '%s' must be an integer, got '%s'", rowIndex+1, fieldName, cellValue)
										logrus.Warn(logMessage)
										odooParams[fieldName] = cellValue // Keep as string, let ODOO handle the error
									}

								default:
									// Other field types, use value as-is
									odooParams[fieldName] = cellValue
								}
							} else {
								// Field not in fieldMap, use value as-is
								odooParams[fieldName] = cellValue
							}
						}
					}
				}

				// Check if we have at least the required fields (model + id + at least one update field)
				if len(odooParams) < 3 { // model + id + at least one field to update
					logMessage := fmt.Sprintf("Row %d: No valid data to update (ID: %s)", rowIndex+1, ticketID)
					logrus.Debug(logMessage)
					logAct[rowIndex+1] = logMessage
					totalFail.Add(1)
					return
				}

				// Remove the 'name' field if it exists, as it's not needed for updates
				delete(odooParams, "name")

				payload := map[string]interface{}{
					"jsonrpc": config.WebPanel.Get().ApiODOO.JSONRPC,
					"params":  odooParams,
				}

				payloadBytes, err := json.Marshal(payload)
				if err != nil {
					logMessage := fmt.Sprintf("Row %d: Failed to marshal JSON payload: %v", rowIndex+1, err)
					logrus.Error(logMessage)
					logAct[rowIndex+1] = logMessage
					totalFail.Add(1)
					return
				}

				response, err := updateDataExceltoTicketODOOMS(loginCookie, string(payloadBytes))
				if err != nil {
					if err.Error() == "ODOO Session Expired: Odoo Session Expired" {
						logAct[rowIndex+1] = fmt.Sprintf("Row %d (ID: %s) failed: session expired using email %s, please check if the email/password submitted was incorrect", rowIndex+1, ticketID, uploadedExcel.Email)
					} else {
						logAct[rowIndex+1] = fmt.Sprintf("Row %d (ID: %s) got error: %v", rowIndex+1, ticketID, err)
					}
					totalFail.Add(1)
				} else {
					logAct[rowIndex+1] = fmt.Sprintf("Row %d (ID: %s) success: %v", rowIndex+1, ticketID, response)
					totalSuccess.Add(1)
				}

				db.Model(&uploadedExcel).Updates(map[string]interface{}{
					"total_success": totalSuccess.Load(),
					"total_fail":    totalFail.Load(),
				})

				// Calculate progress based on completed rows vs total rows
				processed := totalSuccess.Load() + totalFail.Load()
				progress := int((processed * 100) / int64(totalRows))
				if progress > 100 {
					progress = 100
				}

				// Send progress update with current row log
				BroadcastUploadedTicketForTicketUpdateinODOOMSProgress(uploadedExcel, "Processing", progress, logAct[rowIndex+1])
			}(rowIndex, row)
		}
		wg.Wait()
	}
	jsonLog, _ := json.Marshal(logAct)

	BroadcastUploadedTicketForTicketUpdateinODOOMSProgress(uploadedExcel, "Completed", 100, string(jsonLog))

	// Use helper function for notification
	sendNotificationMessage(uploadedExcel, totalSuccess.Load(), totalFail.Load(), logAct, db, "update", false)

	return string(jsonLog)
}

// updateDataExceltoTicketODOOMS sends update request to ODOO MS API
func updateDataExceltoTicketODOOMS(cookieODOO []*http.Cookie, req string) (string, error) {
	return sendODOORequest(cookieODOO, req, "update")
}

// updateDataExceltoTaskODOOMS sends update request to ODOO MS API
func updateDataExceltoTaskODOOMS(cookieODOO []*http.Cookie, req string) (string, error) {
	return sendODOORequest(cookieODOO, req, "update")
}

/*
Create New Helpdesk.Ticket
*/
// ProcessUploadedExcelofODOOMSNewTicketCreated starts workers for CREATE operations (Template 3)
func ProcessUploadedExcelofODOOMSNewTicketCreated(db *gorm.DB) {
	processUploadedExcelWorker(db, OperationConfig{
		TemplateID:            3,
		TriggerChannel:        TriggerProcessUploadExcelforCreateNewTicketODOOMS,
		BroadcastProgressFunc: BroadcastUploadedExcelForCreateNewTicketinODOOMSProgress,
		OperationType:         "create",
		RequiredFieldInColumns: []string{
			"subject",     // First column must be Subject
			"company",     // Second column must be Partner/Company
			"ticket type", // Third column must be Ticket Type
		},
		MinValidColumns: 3,
	})
}

// Broadcast updates to all connected clients
func BroadcastUploadedTicketForTicketUpdateinODOOMSProgress(uploadedExcel odooms.UploadedExcelToODOOMS, status string, progress int, logs string) {
	connectionsMu.RLock()
	defer connectionsMu.RUnlock()

	message := UploadedExcelODOOMSProgress{
		OriginalFilename: uploadedExcel.OriginalFilename,
		Filename:         uploadedExcel.Filename,
		Logs:             logs,
		Status:           status,
		Progress:         progress,
	}

	jsonMessage, _ := json.Marshal(message)

	// Store processing state using switch
	switch status {
	case "Processing", "Pending":
		processingFilesMu.Lock()
		processingFiles[uploadedExcel.Filename] = &uploadedExcel
		processingFilesMu.Unlock()
	case "Completed", "Done", "Failed":
		processingFilesMu.Lock()
		delete(processingFiles, uploadedExcel.Filename)
		processingFilesMu.Unlock()
	}

	for conn := range connections {
		err := safeWriteMessage(conn, websocket.TextMessage, jsonMessage)
		if err != nil {
			logrus.Error("WebSocket write error:", err)
			conn.Close()
			delete(connections, conn)
			cleanupConnectionMutex(conn)
		}
	}
}

// Broadcast updates to all connected clients
func BroadcastUploadedExcelForCreateNewTicketinODOOMSProgress(uploadedExcel odooms.UploadedExcelToODOOMS, status string, progress int, logs string) {
	connectionsMu.RLock()
	defer connectionsMu.RUnlock()

	message := UploadedExcelODOOMSProgress{
		OriginalFilename: uploadedExcel.OriginalFilename,
		Filename:         uploadedExcel.Filename,
		Logs:             logs,
		Status:           status,
		Progress:         progress,
	}

	jsonMessage, _ := json.Marshal(message)

	// Store processing state using switch
	switch status {
	case "Processing", "Pending":
		processingFilesMu.Lock()
		processingFiles[uploadedExcel.Filename] = &uploadedExcel
		processingFilesMu.Unlock()
	case "Completed", "Done", "Failed":
		processingFilesMu.Lock()
		delete(processingFiles, uploadedExcel.Filename)
		processingFilesMu.Unlock()
	}

	for conn := range connections {
		err := safeWriteMessage(conn, websocket.TextMessage, jsonMessage)
		if err != nil {
			logrus.Error("WebSocket write error:", err)
			conn.Close()
			delete(connections, conn)
			cleanupConnectionMutex(conn)
		}
	}
}

func processCustomTemplateCreateNewTicketInODOOMS(rows [][]string, totalSuccess, totalFail *atomic.Int64, uploadedExcel odooms.UploadedExcelToODOOMS, loginCookie []*http.Cookie, db *gorm.DB) string {
	var wg sync.WaitGroup
	sem := make(chan struct{}, config.WebPanel.Get().Default.ConcurrencyLimit)
	logAct := make(map[int]string)
	odooModel := "helpdesk.ticket"
	totalRows := len(rows) - 1

	// Update total row count in database
	db.Model(&uploadedExcel).Update("total_row", totalRows)

	// Check if excel has enough data
	if len(rows) < 2 {
		db.Model(&uploadedExcel).Updates(map[string]interface{}{
			"status": "Failed",
			"log":    "Excel has no data to process!",
		})
		db.Delete(&uploadedExcel)
		return fmt.Sprintf("Excel: %v has no data to process!", uploadedExcel.OriginalFilename)
	}

	// Check if header row has enough columns
	if len(rows[0]) < 3 {
		db.Model(&uploadedExcel).Updates(map[string]interface{}{
			"status": "Failed",
			"log":    "Excel must have at least 3 columns in header!",
		})
		db.Delete(&uploadedExcel)
		return fmt.Sprintf("Excel: %v must have at least 3 columns in header!", uploadedExcel.OriginalFilename)
	}

	// Get ODOO MS ticket fields for validation - USE HELPER
	// fieldMap, err := buildFieldMap(db)
	fieldMap, err := buildFieldMapHelpdeskTicket(db)
	if err != nil {
		return err.Error()
	}

	// Validate required columns in order - USE NEW HELPER
	headerRow := rows[0]
	requiredFields := []string{"subject", "company", "ticket type"} // Required fields in order
	isValid, validationError := validateRequiredColumnsWithType(headerRow, requiredFields, fieldMap)
	if !isValid {
		db.Model(&uploadedExcel).Updates(map[string]interface{}{
			"status": "Failed",
			"log":    fmt.Sprintf("Column validation failed: %s", validationError),
		})
		db.Delete(&uploadedExcel)
		return fmt.Sprintf("Excel: %v - %s", uploadedExcel.OriginalFilename, validationError)
	}

	columnMapping := mapHeaderColumnsWithType(headerRow, fieldMap)

	// Check if we have enough valid columns (required fields + optional fields)
	if len(columnMapping) < len(requiredFields) {
		db.Model(&uploadedExcel).Updates(map[string]interface{}{
			"status": "Failed",
			"log":    fmt.Sprintf("Excel must have at least %d valid columns in header!", len(requiredFields)),
		})
		db.Delete(&uploadedExcel)
		return fmt.Sprintf("Excel: %v must have at least %d valid columns that match ODOO ticket fields!", uploadedExcel.OriginalFilename, len(requiredFields))
	}

	// Process data rows
	batchSize := 100
	dataRows := rows[1:]

	for batchStart := 0; batchStart < len(dataRows); batchStart += batchSize {
		batchEnd := batchStart + batchSize
		if batchEnd > len(dataRows) {
			batchEnd = len(dataRows)
		}

		for i, row := range dataRows[batchStart:batchEnd] {
			rowIndex := batchStart + i
			// Skip empty rows
			if len(row) == 0 {
				continue
			}

			// Check if row has any data
			hasData := false
			for _, cell := range row {
				if strings.TrimSpace(cell) != "" {
					hasData = true
					break
				}
			}

			if !hasData {
				logMessage := fmt.Sprintf("Row %d: Empty row, skipping", rowIndex+1)
				logrus.Debug(logMessage)
				continue
			}

			sem <- struct{}{}
			wg.Add(1)

			go func(rowIndex int, row []string) {
				defer wg.Done()
				defer func() { <-sem }()

				// Check if we have Subject in first column
				if len(row) == 0 || strings.TrimSpace(row[0]) == "" {
					logMessage := fmt.Sprintf("Row %d: Missing Subject in first column", rowIndex+1)
					logrus.Debug(logMessage)
					logAct[rowIndex+1] = logMessage
					totalFail.Add(1)
					return
				}

				ticketSubject := strings.TrimSpace(row[0])

				// Store validated IDs for use in ODOO params
				var validatedCompanyID *uint
				var validatedTicketTypeID *uint
				var validatedMIDTID *uint

				// ✅ VALIDATE: Check if Company exists (Column B - index 1)
				if len(row) > 1 {
					companyValue := strings.TrimSpace(row[1])
					if companyValue != "" {
						exists, company, err := validateCompanyExists(db, companyValue)
						if !exists || err != nil {
							logMessage := fmt.Sprintf("Row %d (Subject: %s): %v", rowIndex+1, ticketSubject, err)
							logrus.Warn(logMessage)
							logAct[rowIndex+1] = logMessage
							totalFail.Add(1)
							return
						}
						validatedCompanyID = &company.ID
						logrus.Debugf("Row %d: Company '%s' validated (ID: %d)", rowIndex+1, companyValue, company.ID)
					}
				}

				// ✅ VALIDATE: Check if Ticket Type exists (Column C - index 2)
				if len(row) > 2 {
					ticketTypeValue := strings.TrimSpace(row[2])
					if ticketTypeValue != "" {
						exists, ticketType, err := validateTicketTypeExists(db, ticketTypeValue)
						if !exists || err != nil {
							logMessage := fmt.Sprintf("Row %d (Subject: %s): %v", rowIndex+1, ticketSubject, err)
							logrus.Warn(logMessage)
							logAct[rowIndex+1] = logMessage
							totalFail.Add(1)
							return
						}
						validatedTicketTypeID = &ticketType.ID
						logrus.Debugf("Row %d: Ticket Type '%s' validated (ID: %d)", rowIndex+1, ticketTypeValue, ticketType.ID)
					}
				}

				// ✅ VALIDATE: Check if MIDTID exists (Column D - index 3)
				if len(row) > 3 {
					midtidValue := strings.TrimSpace(row[3])
					if midtidValue != "" {
						exists, midtid, err := validateMIDTIDExists(midtidValue)
						if !exists || err != nil {
							logMessage := fmt.Sprintf("Row %d (Subject: %s): %v", rowIndex+1, ticketSubject, err)
							logrus.Warn(logMessage)
							logAct[rowIndex+1] = logMessage
							totalFail.Add(1)
							return
						}
						validatedMIDTID = midtid
						logrus.Debugf("Row %d: MIDTID '%s' validated (ID: %d)", rowIndex+1, midtidValue, midtid)
					}
				}

				// Build ODOO parameters based on column mapping
				odooParams := map[string]interface{}{
					"model": odooModel,
				}

				// Map Excel columns to ODOO fields (including first column which is Subject)
				for colIndex, fieldName := range columnMapping {
					if colIndex < len(row) {
						cellValue := strings.TrimSpace(row[colIndex])
						if cellValue != "" {
							// Check if this field is a date/datetime/many2one field and parse it
							lowerHeader := ""
							for headerIndex, headerCol := range headerRow {
								if headerIndex == colIndex {
									lowerHeader = strings.ToLower(strings.TrimSpace(headerCol))
									break
								}
							}

							needToCheck := false
							// Use validated IDs instead of text values for specific fields
							switch fieldName {
							case "company_id":
								if validatedCompanyID != nil && colIndex == 1 {
									odooParams[fieldName] = *validatedCompanyID
									logrus.Debugf("Row %d: Using validated company_id = %d (column %d)", rowIndex+1, *validatedCompanyID, colIndex)
								} else {
									odooParams[fieldName] = cellValue
									logrus.Debugf("Row %d: Using cellValue for company_id = %s (column %d)", rowIndex+1, cellValue, colIndex)
								}
							case "ticket_type_id":
								if validatedTicketTypeID != nil && colIndex == 2 {
									odooParams[fieldName] = *validatedTicketTypeID
									logrus.Debugf("Row %d: Using validated ticket_type_id = %d (column %d)", rowIndex+1, *validatedTicketTypeID, colIndex)
								} else {
									odooParams[fieldName] = cellValue
									logrus.Debugf("Row %d: Using cellValue for ticket_type_id = %s (column %d)", rowIndex+1, cellValue, colIndex)
								}
							case "partner_id":
								if validatedMIDTID != nil && colIndex == 3 {
									odooParams[fieldName] = *validatedMIDTID
									logrus.Debugf("Row %d: Using validated partner_id = %d (column %d)", rowIndex+1, *validatedMIDTID, colIndex)
								} else {
									odooParams[fieldName] = cellValue
									logrus.Debugf("Row %d: Using cellValue for partner_id = %s (column %d)", rowIndex+1, cellValue, colIndex)
								}
							default:
								needToCheck = true
							}

							// Check other fields that not being processed
							if needToCheck {
								if fieldInfo, exists := fieldMap[lowerHeader]; exists {
									fieldType := strings.ToLower(fieldInfo.Type)

									switch fieldType {
									case "date", "datetime":
										// Parse date/datetime fields
										parsedDate, err := fun.ParseFlexibleDate(cellValue)
										if err != nil {
											logMessage := fmt.Sprintf("Row %d: Failed to parse date value '%s' for field '%s': %v", rowIndex+1, cellValue, fieldName, err)
											logrus.Warn(logMessage)
											// Use original value if parsing fails
											odooParams[fieldName] = cellValue
										} else {
											// Format according to field type
											if fieldType == "date" {
												odooParams[fieldName] = parsedDate.Format("2006-01-02")
											} else { // datetime
												// odooParams[fieldName] = parsedDate.Format("2006-01-02 15:04:05")
												// -7 Hours
												odooParams[fieldName] = parsedDate.Add(-7 * time.Hour).Format("2006-01-02 15:04:05")
											}
											logrus.Debugf("Row %d: Parsed %s field '%s': '%s' -> '%v'", rowIndex+1, fieldType, fieldName, cellValue, odooParams[fieldName])
										}

									case "many2one":
										// Many2one fields expect a single integer ID
										if id, err := strconv.Atoi(cellValue); err == nil {
											odooParams[fieldName] = id
											logrus.Debugf("Row %d: Converted many2one field '%s': '%s' -> %d", rowIndex+1, fieldName, cellValue, id)
										} else {
											logMessage := fmt.Sprintf("Row %d: many2one field '%s' must be an integer, got '%s'. Trying to search its id in odoo", rowIndex+1, fieldName, cellValue)
											logrus.Warn(logMessage)

											var needToGetItsID = false
											var odooModel, odooFieldName string
											var odooFieldVal any

											switch strings.ToLower(fieldName) {
											case "team_id":
												needToGetItsID = true
												odooModel = "helpdesk.team"
												odooFieldName = "name"
												odooFieldVal = cellValue
											case "user_id":
												needToGetItsID = true
												odooModel = "res.users"
												odooFieldName = "name"
												odooFieldVal = cellValue
											case "project_id":
												needToGetItsID = true
												odooModel = "project.project"
												odooFieldName = "name"
												odooFieldVal = cellValue
											case "technician_id":
												needToGetItsID = true
												odooModel = "fs.technician"
												odooFieldName = "name"
												odooFieldVal = cellValue
											default:
												odooParams[fieldName] = cellValue // Keep as string, let ODOO handle the error
											}

											if needToGetItsID {
												uid, err := getIDValueOfMany2OneField(odooModel, odooFieldName, odooFieldVal)
												if err != nil {
													logMessage := fmt.Sprintf("Row %d: Failed to get ID for many2one field '%s' with value '%v': %v", rowIndex+1, fieldName, odooFieldVal, err)
													logrus.Warn(logMessage)
													odooParams[fieldName] = cellValue // Keep as string, let ODOO handle the error
												} else {
													odooParams[fieldName] = uid
													logrus.Debugf("Row %d: Retrieved ID %d for many2one field '%s' with value '%v'", rowIndex+1, uid, fieldName, odooFieldVal)
												}
											}
										}

									case "selection":
										switch strings.ToLower(fieldName) {
										case "priority":
											// Map priority text to ODOO expected values
											priorityMap := map[string]string{
												"all":           "0",
												"low priority":  "1",
												"high priority": "2",
												"urgent":        "3",
											}
											lowerVal := strings.ToLower(cellValue)
											if mappedVal, found := priorityMap[lowerVal]; found {
												odooParams[fieldName] = mappedVal
												logrus.Debugf("Row %d: Mapped selection field '%s': '%s' -> '%s'", rowIndex+1, fieldName, cellValue, mappedVal)
											} else {
												logMessage := fmt.Sprintf("Row %d: Invalid value '%s' for selection field '%s'", rowIndex+1, cellValue, fieldName)
												logrus.Warn(logMessage)
												odooParams[fieldName] = cellValue // Keep as-is, let ODOO handle the error
											}
										default:
											odooParams[fieldName] = cellValue
										}

									default:
										// Other field types, use value as-is
										odooParams[fieldName] = cellValue
									}
								} else {
									// Field not in fieldMap, use value as-is
									odooParams[fieldName] = cellValue
								}
							}
						}
					}
				}

				// Check if we have at least the required fields (model + at least 2 fields for creation)
				if len(odooParams) < 3 { // model + subject + at least one other field
					logMessage := fmt.Sprintf("Row %d: No valid data to create ticket (Subject: %s)", rowIndex+1, ticketSubject)
					logrus.Debug(logMessage)
					logAct[rowIndex+1] = logMessage
					totalFail.Add(1)
					return
				}

				payload := map[string]interface{}{
					"jsonrpc": config.WebPanel.Get().ApiODOO.JSONRPC,
					"params":  odooParams,
				}

				// Debug log the payload in JSON format
				if payloadJSON, err := json.MarshalIndent(payload, "", "  "); err == nil {
					logrus.Debugf("Row %d: Payload JSON: %s", rowIndex+1, string(payloadJSON))
				}

				payloadBytes, err := json.Marshal(payload)
				if err != nil {
					logMessage := fmt.Sprintf("Row %d: Failed to marshal JSON payload: %v", rowIndex+1, err)
					logrus.Error(logMessage)
					logAct[rowIndex+1] = logMessage
					totalFail.Add(1)
					return
				}

				response, err := createDataExceltoNewTicketODOOMS(loginCookie, string(payloadBytes))
				if err != nil {
					if err.Error() == "ODOO Session Expired: Odoo Session Expired" {
						logAct[rowIndex+1] = fmt.Sprintf("Row %d (Subject: %s) failed: session expired using email %s, please check if the email/password submitted was incorrect", rowIndex+1, ticketSubject, uploadedExcel.Email)
					} else {
						logAct[rowIndex+1] = fmt.Sprintf("Row %d (Subject: %s) failed: %v", rowIndex+1, ticketSubject, err)
					}
					totalFail.Add(1)
				} else {
					logAct[rowIndex+1] = fmt.Sprintf("Row %d (Subject: %s) success: %v", rowIndex+1, ticketSubject, response)
					totalSuccess.Add(1)
				}

				db.Model(&uploadedExcel).Updates(map[string]interface{}{
					"total_success": totalSuccess.Load(),
					"total_fail":    totalFail.Load(),
				})

				// Calculate progress based on completed rows vs total rows
				processed := totalSuccess.Load() + totalFail.Load()
				progress := int((processed * 100) / int64(totalRows))
				if progress > 100 {
					progress = 100
				}

				// Send progress update with current row log
				BroadcastUploadedExcelForCreateNewTicketinODOOMSProgress(uploadedExcel, "Processing", progress, logAct[rowIndex+1])
			}(rowIndex, row)
		}
		wg.Wait()
	}
	jsonLog, _ := json.Marshal(logAct)

	BroadcastUploadedExcelForCreateNewTicketinODOOMSProgress(uploadedExcel, "Completed", 100, string(jsonLog))

	// Use helper function for notification
	sendNotificationMessage(uploadedExcel, totalSuccess.Load(), totalFail.Load(), logAct, db, "create", false)

	return string(jsonLog)
}

// createDataExceltoNewTicketODOOMS sends creation request to ODOO MS API
func createDataExceltoNewTicketODOOMS(cookieODOO []*http.Cookie, req string) (string, error) {
	return sendODOORequest(cookieODOO, req, "create")
}

/*
Create New CSNA BA Lost Previous Month Data
*/

func ProcessUploadExcelofCSNABALost(db *gorm.DB) {
	processUploadedExcelWorker(db, OperationConfig{
		TemplateID:            5,
		TriggerChannel:        TriggerProcessUploadExcelforCreateDataCSNABALost,
		BroadcastProgressFunc: BroadcastUploadedExcelForCreateCSNABALostProgress,
		OperationType:         "csna_ba_lost",
		RequiredFieldInColumns: []string{
			"no",
			"count",
			"serialnumber",
		},
		MinValidColumns: 1,
	})
}

// Broadcast updates to all connected clients
func BroadcastUploadedExcelForCreateCSNABALostProgress(uploadedExcel odooms.UploadedExcelToODOOMS, status string, progress int, logs string) {
	connectionsMu.RLock()
	defer connectionsMu.RUnlock()

	message := UploadedExcelODOOMSProgress{
		OriginalFilename: uploadedExcel.OriginalFilename,
		Filename:         uploadedExcel.Filename,
		Logs:             logs,
		Status:           status,
		Progress:         progress,
	}

	jsonMessage, _ := json.Marshal(message)

	// Store processing state using switch
	switch status {
	case "Processing", "Pending":
		processingFilesMu.Lock()
		processingFiles[uploadedExcel.Filename] = &uploadedExcel
		processingFilesMu.Unlock()
	case "Completed", "Done", "Failed":
		processingFilesMu.Lock()
		delete(processingFiles, uploadedExcel.Filename)
		processingFilesMu.Unlock()
	}

	for conn := range connections {
		err := safeWriteMessage(conn, websocket.TextMessage, jsonMessage)
		if err != nil {
			logrus.Error("WebSocket write error:", err)
			conn.Close()
			delete(connections, conn)
			cleanupConnectionMutex(conn)
		}
	}
}

func processCSNABALost(rows [][]string, totalSuccess, totalFail *atomic.Int64, uploadedExcel odooms.UploadedExcelToODOOMS, loginCookie []*http.Cookie, db *gorm.DB) string {
	logAct := make(map[int]string)
	totalRows := len(rows) - 1

	// Update total row count in database
	db.Model(&uploadedExcel).Update("total_row", totalRows)

	// Check if excel has enough data
	if len(rows) < 2 {
		db.Model(&uploadedExcel).Updates(map[string]interface{}{
			"status": "Failed",
			"log":    "Excel has no data to process!",
		})
		db.Delete(&uploadedExcel)
		return fmt.Sprintf("Excel: %v has no data to process!", uploadedExcel.OriginalFilename)
	}

	// Check if header row has enough columns
	if len(rows[0]) < 3 {
		db.Model(&uploadedExcel).Updates(map[string]interface{}{
			"status": "Failed",
			"log":    "Excel must have at least 3 column in header!",
		})
		db.Delete(&uploadedExcel)
		return fmt.Sprintf("Excel: %v must have at least 3 column in header!", uploadedExcel.OriginalFilename)
	}

	// Build field map for CSNABALost
	fieldMap := buildFieldMapFromStruct(odooms.CSNABALost{})

	// Map header columns
	headerRow := rows[0]
	columnMapping := make(map[int]string) // column index -> json field name

	for colIndex, header := range headerRow {
		header = strings.TrimSpace(header)

		// Special handling for empty headers at specific column positions
		// Column AY (index 50) should be mapped to "approved_status" - but only if it exists
		if colIndex == 50 && colIndex < len(headerRow) { // AY column (A=0, B=1, ..., Y=24, Z=25, AA=26, ..., AY=50)
			header = "approved status"
		}

		if header == "" {
			continue
		}

		lowerHeader := strings.ToLower(header)

		if strings.Contains(lowerHeader, "ada di ba lost") {
			lowerHeader = "ada di ba lost prev"
		}

		if strings.Contains(lowerHeader, "pheripheral") || strings.Contains(lowerHeader, "peripheral") || strings.Contains(lowerHeader, "periperal") || strings.Contains(lowerHeader, "pheriperal") {
			lowerHeader = "peripheral"
		}

		if strings.Contains(lowerHeader, "maas") {
			lowerHeader = "maas"
		}

		if strings.Contains(lowerHeader, "location") && colIndex == 39 { // Column AN (index 39)
			lowerHeader = "location2"
		}

		if fieldName, exists := fieldMap[lowerHeader]; exists {
			columnMapping[colIndex] = fieldName
			logrus.Infof("Mapped column %d '%s' to field '%s'", colIndex, header, fieldName)
		} else {
			logrus.Warnf("Column %d '%s' does not match any CSNABALost field", colIndex, header)
		}
	}

	// Check if we have at least one valid column
	if len(columnMapping) < 3 {
		db.Model(&uploadedExcel).Updates(map[string]interface{}{
			"status": "Failed",
			"log":    "Excel must have at least 3 valid column that matches CSNABALost fields!",
		})
		db.Delete(&uploadedExcel)
		return fmt.Sprintf("Excel: %v must have at least 3 valid column that matches CSNABALost fields!", uploadedExcel.OriginalFilename)
	}

	// Check if there are existing rows in CSNABALost table and delete them unscoped
	var existingCount int64
	if err := db.Model(&odooms.CSNABALost{}).Count(&existingCount).Error; err != nil {
		logMessage := fmt.Sprintf("Failed to count existing CSNABALost records: %v", err)
		logrus.Error(logMessage)
		db.Model(&uploadedExcel).Updates(map[string]interface{}{
			"status": "Failed",
			"log":    logMessage,
		})
		return logMessage
	}

	if existingCount > 0 {
		logrus.Infof("Found %d existing CSNABALost records, deleting them unscoped", existingCount)
		if err := db.Unscoped().Where("1=1").Delete(&odooms.CSNABALost{}).Error; err != nil {
			logMessage := fmt.Sprintf("Failed to delete existing CSNABALost records: %v", err)
			logrus.Error(logMessage)
			db.Model(&uploadedExcel).Updates(map[string]interface{}{
				"status": "Failed",
				"log":    logMessage,
			})
			return logMessage
		}
		logrus.Infof("Successfully deleted %d existing CSNABALost records", existingCount)
	}

	// Collect valid records for batch insert
	var recordsToInsert []odooms.CSNABALost

	// Process data rows
	for rowIndex, row := range rows[1:] {
		// Skip empty rows
		if len(row) == 0 {
			continue
		}

		// Check if row has any data
		hasData := false
		for _, cell := range row {
			if strings.TrimSpace(cell) != "" {
				hasData = true
				break
			}
		}

		if !hasData {
			logMessage := fmt.Sprintf("Row %d: Empty row, skipping", rowIndex+1)
			logrus.Debug(logMessage)
			continue
		}

		// Create new CSNABALost record
		record := odooms.CSNABALost{}

		// Map Excel columns to record fields
		validData := false
		for colIndex, fieldName := range columnMapping {
			if colIndex < len(row) {
				cellValue := strings.TrimSpace(row[colIndex])
				if cellValue != "" {
					validData = true
					// Set field value using reflection
					v := reflect.ValueOf(&record).Elem()
					field := v.FieldByNameFunc(func(name string) bool {
						f, _ := v.Type().FieldByName(name)
						return f.Tag.Get("json") == fieldName
					})

					if field.IsValid() && field.CanSet() {
						switch field.Kind() {
						case reflect.String:
							// Skip setting "n/a" and Excel error values (case insensitive)
							lowerValue := strings.ToLower(cellValue)
							if lowerValue != "n/a" && lowerValue != "na" && lowerValue != "#n/a" &&
								lowerValue != "-" && lowerValue != "" &&
								!strings.HasPrefix(lowerValue, "#") { // Skip Excel error values like #DIV/0!, #REF!, etc.
								field.SetString(cellValue)
							}
						case reflect.Int, reflect.Int32, reflect.Int64:
							if intVal, err := strconv.Atoi(cellValue); err == nil {
								field.SetInt(int64(intVal))
							}
						case reflect.Float32, reflect.Float64:
							if floatVal, err := fun.ParseRupiah(cellValue); err == nil {
								field.SetFloat(floatVal)
							}
						case reflect.Bool:
							if boolVal, err := strconv.ParseBool(cellValue); err == nil {
								field.SetBool(boolVal)
							}
						case reflect.Ptr:
							// Handle pointer types like *time.Time
							if field.Type().Elem().Kind() == reflect.Struct && field.Type().Elem().String() == "time.Time" {
								if parsedTime, err := fun.ParseFlexibleDate(cellValue); err == nil {
									timePtr := reflect.New(field.Type().Elem())
									timePtr.Elem().Set(reflect.ValueOf(parsedTime))
									field.Set(timePtr)
								}
							}
						}
					}
				}
			}
		}

		if validData {
			recordsToInsert = append(recordsToInsert, record)
			logMessage := fmt.Sprintf("Row %d: Valid record prepared for insertion", rowIndex+1)
			// logrus.Debug(logMessage)
			logAct[rowIndex+1] = logMessage
			totalSuccess.Add(1)
		} else {
			logMessage := fmt.Sprintf("Row %d: No valid data to insert", rowIndex+1)
			logrus.Debug(logMessage)
			logAct[rowIndex+1] = logMessage
			totalFail.Add(1)
		}

		// Update progress periodically
		if (rowIndex+1)%10 == 0 || rowIndex+1 == totalRows {
			processed := totalSuccess.Load() + totalFail.Load()
			progress := int((processed * 100) / int64(totalRows))
			if progress > 100 {
				progress = 100
			}
			BroadcastUploadedExcelForCreateCSNABALostProgress(uploadedExcel, "Processing", progress, fmt.Sprintf("Processed %d/%d rows", processed, totalRows))
		}
	}

	// Batch insert records
	if len(recordsToInsert) > 0 {
		logrus.Infof("Batch inserting %d CSNABALost records", len(recordsToInsert))
		if err := db.CreateInBatches(&recordsToInsert, 100).Error; err != nil {
			logMessage := fmt.Sprintf("Failed to batch insert CSNABALost records: %v", err)
			logrus.Error(logMessage)
			db.Model(&uploadedExcel).Updates(map[string]interface{}{
				"status": "Failed",
				"log":    logMessage,
			})
			return logMessage
		}
		logrus.Infof("Successfully inserted %d CSNABALost records", len(recordsToInsert))
	}

	jsonLog, _ := json.Marshal(logAct)

	BroadcastUploadedExcelForCreateCSNABALostProgress(uploadedExcel, "Completed", 100, string(jsonLog))

	// Use helper function for notification
	sendNotificationMessage(uploadedExcel, totalSuccess.Load(), totalFail.Load(), logAct, db, "csna_ba_lost", false)

	return string(jsonLog)
}

/*
Create technician's payslip based on payroll template
*/
func ProcessUploadExcelofTechnicianPayroll(db *gorm.DB) {
	processUploadedExcelWorker(db, OperationConfig{
		TemplateID:            6,
		TriggerChannel:        TriggerProcessUploadExcelforCreateTechnicianPayslip,
		BroadcastProgressFunc: BroadcastUploadedExcelForCreateTechnicianPayslipProgress,
		OperationType:         "technician_payroll",
		MultiSheet:            true, // Enable multi-sheet processing
		Sheets: []SheetProcessingConfig{
			/*
				MS - EDC
			*/
			{
				SheetName: "Tickets Regular EDC",
				Handler:   processTechnicianPayrollIntoPayslipTicketsRegularEDC,
				RequiredFieldInColumns: []string{
					"ticket",
					"ticket type",
					"company",
				},
				MinValidColumns: 3,
				HeaderRowIndex:  0, // Row 1 (0-based index = 0)
				SkipIfNotFound:  false,
			},
			{
				SheetName: "Tickets BP",
				Handler:   processTechnicianPayrollIntoPayslipTicketsBP,
				RequiredFieldInColumns: []string{
					"ticket",
					"ticket type",
					"company",
				},
				MinValidColumns: 3,
				HeaderRowIndex:  0, // Row 1 (0-based index = 0)
				SkipIfNotFound:  false,
			},
			{
				SheetName: "Tickets Unworked EDC",
				Handler:   processTechnicianPayrollIntoPayslipTicketsUnworkedEDC,
				RequiredFieldInColumns: []string{
					"ticket",
					"ticket type",
					"company",
				},
				MinValidColumns: 3,
				HeaderRowIndex:  0, // Row 1 (0-based index = 0)
				SkipIfNotFound:  false,
			},
			/*
				MS - ATM
			*/
			{
				SheetName: "Ticket Reguler ATM",
				Handler:   processTechnicianPayrollIntoPayslipTicketsRegularATM,
				RequiredFieldInColumns: []string{
					"ticket",
					"ticket type",
					"company",
				},
				MinValidColumns: 3,
				HeaderRowIndex:  0, // Row 1 (0-based index = 0)
				SkipIfNotFound:  false,
			},
			{
				SheetName: "Ticket Unworked ATM",
				Handler:   processTechnicianPayrollIntoPayslipTicketsUnworkedATM,
				RequiredFieldInColumns: []string{
					"ticket",
					"ticket type",
					"company",
				},
				MinValidColumns: 3,
				HeaderRowIndex:  0, // Row 1 (0-based index = 0)
				SkipIfNotFound:  false,
			},
			{
				SheetName: "Dedicated ATM",
				Handler:   processTechnicianPayrollDedicatedATM,
				RequiredFieldInColumns: []string{
					"no",
					"contract no",
					"name",
					"email",
					"group",
					"basic",
					"jo target",
				},
				MinValidColumns: 7,
				HeaderRowIndex:  6, // Row 7 (0-based index = 6)
				SkipIfNotFound:  false,
			},
			{
				SheetName: "Technician Payrolls",
				Handler:   processTechnicianPayrollIntoPayslip,
				RequiredFieldInColumns: []string{
					"no",
					"contract no",
					"name",
					"email",
					"group",
					"basic",
					"jo target",
				},
				MinValidColumns: 7,
				HeaderRowIndex:  6, // Row 7 (0-based index = 6)
				SkipIfNotFound:  false,
			},
		},
	})
}

// Broadcast updates to all connected clients
func BroadcastUploadedExcelForCreateTechnicianPayslipProgress(uploadedExcel odooms.UploadedExcelToODOOMS, status string, progress int, logs string) {
	connectionsMu.RLock()
	defer connectionsMu.RUnlock()

	message := UploadedExcelODOOMSProgress{
		OriginalFilename: uploadedExcel.OriginalFilename,
		Filename:         uploadedExcel.Filename,
		Logs:             logs,
		Status:           status,
		Progress:         progress,
	}

	jsonMessage, _ := json.Marshal(message)

	// Store processing state using switch
	switch status {
	case "Processing", "Pending":
		processingFilesMu.Lock()
		processingFiles[uploadedExcel.Filename] = &uploadedExcel
		processingFilesMu.Unlock()
	case "Completed", "Done", "Failed":
		processingFilesMu.Lock()
		delete(processingFiles, uploadedExcel.Filename)
		processingFilesMu.Unlock()
	}

	for conn := range connections {
		err := safeWriteMessage(conn, websocket.TextMessage, jsonMessage)
		if err != nil {
			logrus.Error("WebSocket write error:", err)
			conn.Close()
			delete(connections, conn)
			cleanupConnectionMutex(conn)
		}
	}
}

func processTechnicianPayrollIntoPayslip(rows [][]string, totalSuccess, totalFail *atomic.Int64, uploadedExcel odooms.UploadedExcelToODOOMS, loginCookie []*http.Cookie, db *gorm.DB) string {
	logAct := make(map[int]string)
	totalRows := len(rows) - 1

	// Update total row count in database
	db.Model(&uploadedExcel).Update("total_row", totalRows)

	// Check if excel has enough data
	if len(rows) < 2 {
		db.Model(&uploadedExcel).Updates(map[string]interface{}{
			"status": "Failed",
			"log":    "Excel has no data to process!",
		})
		db.Delete(&uploadedExcel)
		return fmt.Sprintf("Excel: %v has no data to process!", uploadedExcel.OriginalFilename)
	}

	// Check if header row has enough columns
	if len(rows[0]) < 7 {
		db.Model(&uploadedExcel).Updates(map[string]interface{}{
			"status": "Failed",
			"log":    "Excel must have at least 7 column in header!",
		})
		db.Delete(&uploadedExcel)
		return fmt.Sprintf("Excel: %v must have at least 7 column in header!", uploadedExcel.OriginalFilename)
	}

	// Build field map
	fieldMap := buildFieldMapFromStruct(odooms.MSTechnicianPayroll{})

	// Map header columns
	headerRow := rows[6]                  // Assuming header is in row 7 (index 6)
	columnMapping := make(map[int]string) // column index -> json field name
	var containsColName bool = false
	var indexHeaderRow int = 7
	for _, header := range headerRow {
		if strings.ToLower(strings.TrimSpace(header)) == "name" {
			containsColName = true

			break
		}
	}
	if !containsColName {
		headerRow = rows[0] // Fallback to first row if "name" column not found in row 7
		indexHeaderRow = 1
	}

	for colIndex, header := range headerRow {
		header = strings.TrimSpace(header)
		if header == "" {
			continue
		}

		lowerHeader := strings.ToLower(header)

		switch colIndex {
		case 7: // PM Meet
			lowerHeader = "pm meet"
		case 8: // PM Over
			lowerHeader = "pm over"
		case 9: // PM Unworked
			lowerHeader = "pm unworked"
		case 10: // Non PM Meet
			lowerHeader = "non pm meet"
		case 11: // Non PM Over
			lowerHeader = "non pm over"
		case 12: // Non PM Unworked
			lowerHeader = "non pm unworked"
		case 13: // Incentive/Ticket
			lowerHeader = "incentive per ticket"
		case 21: // Potongan Overdue PM
			lowerHeader = "potongan overdue pm"
		case 22: // Potongan Overdue NPM
			lowerHeader = "potongan overdue non pm"
		case 23: // Potongan Overdue Unworked PM
			lowerHeader = "potongan overdue unworked pm"
		case 24: // Potongan Overdue Unworked NPM
			lowerHeader = "potongan overdue unworked non pm"
		case 25: // Total Potongan Overdue
			lowerHeader = "total potongan overdue"
		case 26: // Total Potongan Unworked
			lowerHeader = "total potongan unworked"
		case 27: // Total Potongan Total
			lowerHeader = "total potongan total"
		case 30: // Bank Penerima Gaji
			lowerHeader = "bank penerima gaji"
		case 31: // Nomor Rekening Bank Penerima Gaji
			lowerHeader = "nomor rekening bank penerima gaji"
		case 32: // Nama Rekening Bank Penerima Gaji
			lowerHeader = "nama rekening bank penerima gaji"
		case 35: // JO BP (All)
			lowerHeader = "jo bp all"
		}

		if fieldName, exists := fieldMap[lowerHeader]; exists {
			columnMapping[colIndex] = fieldName
			logrus.Infof("Mapped column %d '%s' to field '%s'", colIndex, header, fieldName)
		} else {
			logrus.Warnf("Column %d '%s' does not match any model struct field", colIndex, header)
		}
	}

	// Check if we have at least one valid column
	if len(columnMapping) < 3 {
		db.Model(&uploadedExcel).Updates(map[string]interface{}{
			"status": "Failed",
			"log":    "Excel must have at least 3 valid column that matches model struct fields!",
		})
		db.Delete(&uploadedExcel)
		return fmt.Sprintf("Excel: %v must have at least 3 valid column that matches model struct fields!", uploadedExcel.OriginalFilename)
	}

	// Check if there are existing rows in model struct table and delete them
	var existingCount int64
	if err := db.Model(&odooms.MSTechnicianPayroll{}).Count(&existingCount).Error; err != nil {
		logMessage := fmt.Sprintf("Failed to count existing model struct records: %v", err)
		logrus.Error(logMessage)
		db.Model(&uploadedExcel).Updates(map[string]interface{}{
			"status": "Failed",
			"log":    logMessage,
		})
		return logMessage
	}

	if existingCount > 0 {
		logrus.Infof("Found %d existing MSTechnicianPayroll records, deleting them", existingCount)
		if err := db.Where("1=1").Delete(&odooms.MSTechnicianPayroll{}).Error; err != nil {
			logMessage := fmt.Sprintf("Failed to delete existing MSTechnicianPayroll records: %v", err)
			logrus.Error(logMessage)
			db.Model(&uploadedExcel).Updates(map[string]interface{}{
				"status": "Failed",
				"log":    logMessage,
			})
			return logMessage
		}
		logrus.Infof("Successfully deleted %d existing MSTechnicianPayroll records", existingCount)
	}

	// Collect valid records for batch insert
	var recordsToInsert []odooms.MSTechnicianPayroll

	// Process data rows
	for rowIndex, row := range rows[indexHeaderRow:] {
		// Skip empty rows
		if len(row) == 0 {
			continue
		}

		// Skip if column C (index 2) is empty
		if len(row) > 2 && strings.TrimSpace(row[2]) == "" {
			logMessage := fmt.Sprintf("Row %d: Column C is empty, skipping", rowIndex+1)
			logrus.Debug(logMessage)
			continue
		}

		// Check if row has any data
		hasData := false
		for _, cell := range row {
			if strings.TrimSpace(cell) != "" {
				hasData = true
				break
			}
		}

		if !hasData {
			logMessage := fmt.Sprintf("Row %d: Empty row, skipping", rowIndex+1)
			logrus.Debug(logMessage)
			continue
		}

		// Create new MSTechnicianPayroll record
		record := odooms.MSTechnicianPayroll{}

		// Map Excel columns to record fields
		validData := false
		for colIndex, fieldName := range columnMapping {
			if colIndex < len(row) {
				cellValue := strings.TrimSpace(row[colIndex])
				if cellValue != "" {
					validData = true
					// Set field value using reflection
					v := reflect.ValueOf(&record).Elem()
					field := v.FieldByNameFunc(func(name string) bool {
						f, _ := v.Type().FieldByName(name)
						return f.Tag.Get("json") == fieldName
					})

					if field.IsValid() && field.CanSet() {
						switch field.Kind() {
						case reflect.String:
							// Skip setting "n/a" and Excel error values (case insensitive)
							lowerValue := strings.ToLower(cellValue)
							if lowerValue != "n/a" && lowerValue != "na" && lowerValue != "#n/a" &&
								lowerValue != "-" && lowerValue != "" &&
								!strings.HasPrefix(lowerValue, "#") { // Skip Excel error values like #DIV/0!, #REF!, etc.
								field.SetString(cellValue)
							}
						case reflect.Int, reflect.Int32, reflect.Int64:
							if intVal, err := strconv.Atoi(cellValue); err == nil {
								field.SetInt(int64(intVal))
							}
						case reflect.Float32, reflect.Float64:
							if floatVal, err := fun.ParseRupiah(cellValue); err == nil {
								field.SetFloat(floatVal)
							}
						case reflect.Bool:
							// Use Excel boolean parser for better compatibility (YES/NO, Y/N, TRUE/FALSE, etc.)
							if boolVal, err := parseExcelBoolean(cellValue); err == nil {
								field.SetBool(boolVal)
							} else {
								logrus.Warnf("Row %d, Column %d (%s): Failed to parse boolean value '%s': %v",
									rowIndex+1, colIndex, fieldName, cellValue, err)
							}
						case reflect.Ptr:
							// Handle pointer types like *time.Time
							if field.Type().Elem().Kind() == reflect.Struct && field.Type().Elem().String() == "time.Time" {
								if parsedTime, err := fun.ParseFlexibleDate(cellValue); err == nil {
									timePtr := reflect.New(field.Type().Elem())
									timePtr.Elem().Set(reflect.ValueOf(parsedTime))
									field.Set(timePtr)
								}
							}
						}
					}
				}
			}
		}

		// Set the uploaded_by field
		record.UploadedBy = uploadedExcel.Email

		var odooTech odooms.ODOOMSTechnicianData
		if err := db.Where("technician = ?", record.Name).First(&odooTech).Error; err == nil {
			if odooTech.NoHP != "" {
				record.NoHP = odooTech.NoHP
			}
		}

		if validData {
			recordsToInsert = append(recordsToInsert, record)
			logMessage := fmt.Sprintf("Row %d: Valid record prepared for insertion", rowIndex+1)
			// logrus.Debug(logMessage)
			logAct[rowIndex+1] = logMessage
			totalSuccess.Add(1)
		} else {
			logMessage := fmt.Sprintf("Row %d: No valid data to insert", rowIndex+1)
			logrus.Debug(logMessage)
			logAct[rowIndex+1] = logMessage
			totalFail.Add(1)
		}

		// Update progress periodically
		if (rowIndex+1)%10 == 0 || rowIndex+1 == totalRows {
			processed := totalSuccess.Load() + totalFail.Load()
			progress := int((processed * 100) / int64(totalRows))
			if progress > 100 {
				progress = 100
			}
			BroadcastUploadedExcelForCreateTechnicianPayslipProgress(uploadedExcel, "Processing", progress, fmt.Sprintf("Processed %d/%d rows", processed, totalRows))
		}
	}

	// Batch insert records
	if len(recordsToInsert) > 0 {
		logrus.Infof("Batch inserting %d records", len(recordsToInsert))
		if err := db.CreateInBatches(&recordsToInsert, 100).Error; err != nil {
			logMessage := fmt.Sprintf("Failed to batch insert records: %v", err)
			logrus.Error(logMessage)
			db.Model(&uploadedExcel).Updates(map[string]interface{}{
				"status": "Failed",
				"log":    logMessage,
			})
			return logMessage
		}
		logrus.Infof("Successfully inserted %d records", len(recordsToInsert))
	}

	jsonLog, _ := json.Marshal(logAct)

	return string(jsonLog)
}

func processTechnicianPayrollIntoPayslipTicketsRegularEDC(rows [][]string, totalSuccess, totalFail *atomic.Int64, uploadedExcel odooms.UploadedExcelToODOOMS, loginCookie []*http.Cookie, db *gorm.DB) string {
	logAct := make(map[int]string)
	totalRows := len(rows) - 1

	// Update total row count in database
	db.Model(&uploadedExcel).Update("total_row", totalRows)

	// Check if excel has enough data
	if len(rows) < 2 {
		db.Model(&uploadedExcel).Updates(map[string]interface{}{
			"status": "Failed",
			"log":    "Excel has no data to process!",
		})
		db.Delete(&uploadedExcel)
		return fmt.Sprintf("Excel: %v has no data to process!", uploadedExcel.OriginalFilename)
	}

	// Check if header row has enough columns
	if len(rows[0]) < 3 {
		db.Model(&uploadedExcel).Updates(map[string]interface{}{
			"status": "Failed",
			"log":    "Excel must have at least 3 column in header!",
		})
		db.Delete(&uploadedExcel)
		return fmt.Sprintf("Excel: %v must have at least 3 column in header!", uploadedExcel.OriginalFilename)
	}

	// Build field map
	fieldMap := buildFieldMapFromStruct(odooms.MSTechnicianPayrollTicketsRegularEDC{})

	// Map header columns
	headerRow := rows[0]                  // Assuming header is in row 1 (index 0)
	columnMapping := make(map[int]string) // column index -> json field name

	for colIndex, header := range headerRow {
		header = strings.TrimSpace(header)
		if header == "" {
			continue
		}

		lowerHeader := strings.ToLower(header)

		switch lowerHeader {
		case "re-assigned":
			lowerHeader = "re assigned"
		case "sla status (assume)":
			lowerHeader = "sla status assume"
		case "why ?":
			lowerHeader = "why"
		}

		if fieldName, exists := fieldMap[lowerHeader]; exists {
			columnMapping[colIndex] = fieldName
			logrus.Infof("Mapped column %d '%s' to field '%s'", colIndex, header, fieldName)
		} else {
			logrus.Warnf("Column %d '%s' does not match any model struct field", colIndex, header)
		}
	}

	// Check if we have at least one valid column
	if len(columnMapping) < 3 {
		db.Model(&uploadedExcel).Updates(map[string]interface{}{
			"status": "Failed",
			"log":    "Excel must have at least 3 valid column that matches model struct fields!",
		})
		db.Delete(&uploadedExcel)
		return fmt.Sprintf("Excel: %v must have at least 3 valid column that matches model struct fields!", uploadedExcel.OriginalFilename)
	}

	// Check if there are existing rows in model struct table and delete them
	var existingCount int64
	if err := db.Model(&odooms.MSTechnicianPayrollTicketsRegularEDC{}).Count(&existingCount).Error; err != nil {
		logMessage := fmt.Sprintf("Failed to count existing model struct records: %v", err)
		logrus.Error(logMessage)
		db.Model(&uploadedExcel).Updates(map[string]interface{}{
			"status": "Failed",
			"log":    logMessage,
		})
		return logMessage
	}

	if existingCount > 0 {
		logrus.Infof("Found %d existing MSTechnicianPayrollTicketsRegularEDC records, deleting them", existingCount)
		if err := db.Where("1=1").Delete(&odooms.MSTechnicianPayrollTicketsRegularEDC{}).Error; err != nil {
			logMessage := fmt.Sprintf("Failed to delete existing MSTechnicianPayrollTicketsRegularEDC records: %v", err)
			logrus.Error(logMessage)
			db.Model(&uploadedExcel).Updates(map[string]interface{}{
				"status": "Failed",
				"log":    logMessage,
			})
			return logMessage
		}
		logrus.Infof("Successfully deleted %d existing MSTechnicianPayrollTicketsRegularEDC records", existingCount)
	}

	// Collect valid records for batch insert
	var recordsToInsert []odooms.MSTechnicianPayrollTicketsRegularEDC

	// Process data rows
	for rowIndex, row := range rows[1:] {
		// Skip empty rows
		if len(row) == 0 {
			continue
		}

		// Check if row has any data
		hasData := false
		for _, cell := range row {
			if strings.TrimSpace(cell) != "" {
				hasData = true
				break
			}
		}

		if !hasData {
			logMessage := fmt.Sprintf("Row %d: Empty row, skipping", rowIndex+1)
			logrus.Debug(logMessage)
			continue
		}

		// Create new MSTechnicianPayrollTicketsRegularEDC record
		record := odooms.MSTechnicianPayrollTicketsRegularEDC{}

		// Map Excel columns to record fields
		validData := false
		for colIndex, fieldName := range columnMapping {
			if colIndex < len(row) {
				cellValue := strings.TrimSpace(row[colIndex])
				if cellValue != "" {
					validData = true
					// Set field value using reflection
					v := reflect.ValueOf(&record).Elem()
					field := v.FieldByNameFunc(func(name string) bool {
						f, _ := v.Type().FieldByName(name)
						return f.Tag.Get("json") == fieldName
					})

					if field.IsValid() && field.CanSet() {
						switch field.Kind() {
						case reflect.String:
							// Skip setting "n/a" and Excel error values (case insensitive)
							lowerValue := strings.ToLower(cellValue)
							if lowerValue != "n/a" && lowerValue != "na" && lowerValue != "#n/a" &&
								lowerValue != "-" && lowerValue != "" &&
								!strings.HasPrefix(lowerValue, "#") { // Skip Excel error values like #DIV/0!, #REF!, etc.
								field.SetString(cellValue)
							}
						case reflect.Int, reflect.Int32, reflect.Int64:
							if intVal, err := strconv.Atoi(cellValue); err == nil {
								field.SetInt(int64(intVal))
							}
						case reflect.Float32, reflect.Float64:
							if floatVal, err := fun.ParseRupiah(cellValue); err == nil {
								field.SetFloat(floatVal)
							}
						case reflect.Bool:
							// Use Excel boolean parser for better compatibility (YES/NO, Y/N, TRUE/FALSE, etc.)
							if boolVal, err := parseExcelBoolean(cellValue); err == nil {
								field.SetBool(boolVal)
							} else {
								logrus.Warnf("Row %d, Column %d (%s): Failed to parse boolean value '%s': %v",
									rowIndex+1, colIndex, fieldName, cellValue, err)
							}
						case reflect.Ptr:
							// Handle pointer types like *time.Time
							if field.Type().Elem().Kind() == reflect.Struct && field.Type().Elem().String() == "time.Time" {
								if parsedTime, err := fun.ParseFlexibleDate(cellValue); err == nil {
									timePtr := reflect.New(field.Type().Elem())
									timePtr.Elem().Set(reflect.ValueOf(parsedTime))
									field.Set(timePtr)
								}
							}
						}
					}
				}
			}
		}

		// Set the uploaded_by field
		record.UploadedBy = uploadedExcel.Email

		if validData {
			recordsToInsert = append(recordsToInsert, record)
			logMessage := fmt.Sprintf("Row %d: Valid record prepared for insertion", rowIndex+1)
			// logrus.Debug(logMessage)
			logAct[rowIndex+1] = logMessage
			totalSuccess.Add(1)
		} else {
			logMessage := fmt.Sprintf("Row %d: No valid data to insert", rowIndex+1)
			logrus.Debug(logMessage)
			logAct[rowIndex+1] = logMessage
			totalFail.Add(1)
		}

		// Update progress periodically
		if (rowIndex+1)%10 == 0 || rowIndex+1 == totalRows {
			processed := totalSuccess.Load() + totalFail.Load()
			progress := int((processed * 100) / int64(totalRows))
			if progress > 100 {
				progress = 100
			}
			BroadcastUploadedExcelForCreateTechnicianPayslipProgress(uploadedExcel, "Processing", progress, fmt.Sprintf("Processed %d/%d rows", processed, totalRows))
		}
	}

	// Batch insert records
	if len(recordsToInsert) > 0 {
		logrus.Infof("Batch inserting %d records", len(recordsToInsert))
		if err := db.CreateInBatches(&recordsToInsert, 100).Error; err != nil {
			logMessage := fmt.Sprintf("Failed to batch insert records: %v", err)
			logrus.Error(logMessage)
			db.Model(&uploadedExcel).Updates(map[string]interface{}{
				"status": "Failed",
				"log":    logMessage,
			})
			return logMessage
		}
		logrus.Infof("Successfully inserted %d records", len(recordsToInsert))
	}

	jsonLog, _ := json.Marshal(logAct)

	return string(jsonLog)
}

func processTechnicianPayrollIntoPayslipTicketsBP(rows [][]string, totalSuccess, totalFail *atomic.Int64, uploadedExcel odooms.UploadedExcelToODOOMS, loginCookie []*http.Cookie, db *gorm.DB) string {
	logAct := make(map[int]string)
	totalRows := len(rows) - 1

	// Update total row count in database
	db.Model(&uploadedExcel).Update("total_row", totalRows)

	// Check if excel has enough data
	if len(rows) < 2 {
		db.Model(&uploadedExcel).Updates(map[string]interface{}{
			"status": "Failed",
			"log":    "Excel has no data to process!",
		})
		db.Delete(&uploadedExcel)
		return fmt.Sprintf("Excel: %v has no data to process!", uploadedExcel.OriginalFilename)
	}

	// Check if header row has enough columns
	if len(rows[0]) < 3 {
		db.Model(&uploadedExcel).Updates(map[string]interface{}{
			"status": "Failed",
			"log":    "Excel must have at least 3 column in header!",
		})
		db.Delete(&uploadedExcel)
		return fmt.Sprintf("Excel: %v must have at least 3 column in header!", uploadedExcel.OriginalFilename)
	}

	// Build field map
	fieldMap := buildFieldMapFromStruct(odooms.MSTechnicianPayrollTicketsBP{})

	// Map header columns
	headerRow := rows[0]                  // Assuming header is in row 1 (index 0)
	columnMapping := make(map[int]string) // column index -> json field name

	for colIndex, header := range headerRow {
		header = strings.TrimSpace(header)
		if header == "" {
			continue
		}

		lowerHeader := strings.ToLower(header)

		switch lowerHeader {
		case "re-assigned":
			lowerHeader = "re assigned"
		case "sla status (assume)":
			lowerHeader = "sla status assume"
		case "why ?":
			lowerHeader = "why"
		}

		if fieldName, exists := fieldMap[lowerHeader]; exists {
			columnMapping[colIndex] = fieldName
			logrus.Infof("Mapped column %d '%s' to field '%s'", colIndex, header, fieldName)
		} else {
			logrus.Warnf("Column %d '%s' does not match any model struct field", colIndex, header)
		}
	}

	// Check if we have at least one valid column
	if len(columnMapping) < 3 {
		db.Model(&uploadedExcel).Updates(map[string]interface{}{
			"status": "Failed",
			"log":    "Excel must have at least 3 valid column that matches model struct fields!",
		})
		db.Delete(&uploadedExcel)
		return fmt.Sprintf("Excel: %v must have at least 3 valid column that matches model struct fields!", uploadedExcel.OriginalFilename)
	}

	// Check if there are existing rows in model struct table and delete them
	var existingCount int64
	if err := db.Model(&odooms.MSTechnicianPayrollTicketsBP{}).Count(&existingCount).Error; err != nil {
		logMessage := fmt.Sprintf("Failed to count existing model struct records: %v", err)
		logrus.Error(logMessage)
		db.Model(&uploadedExcel).Updates(map[string]interface{}{
			"status": "Failed",
			"log":    logMessage,
		})
		return logMessage
	}

	if existingCount > 0 {
		logrus.Infof("Found %d existing MSTechnicianPayrollTicketsBP records, deleting them", existingCount)
		if err := db.Where("1=1").Delete(&odooms.MSTechnicianPayrollTicketsBP{}).Error; err != nil {
			logMessage := fmt.Sprintf("Failed to delete existing MSTechnicianPayrollTicketsBP records: %v", err)
			logrus.Error(logMessage)
			db.Model(&uploadedExcel).Updates(map[string]interface{}{
				"status": "Failed",
				"log":    logMessage,
			})
			return logMessage
		}
		logrus.Infof("Successfully deleted %d existing MSTechnicianPayrollTicketsBP records", existingCount)
	}

	// Collect valid records for batch insert
	var recordsToInsert []odooms.MSTechnicianPayrollTicketsBP

	// Process data rows
	for rowIndex, row := range rows[1:] {
		// Skip empty rows
		if len(row) == 0 {
			continue
		}

		// Check if row has any data
		hasData := false
		for _, cell := range row {
			if strings.TrimSpace(cell) != "" {
				hasData = true
				break
			}
		}

		if !hasData {
			logMessage := fmt.Sprintf("Row %d: Empty row, skipping", rowIndex+1)
			logrus.Debug(logMessage)
			continue
		}

		// Create new MSTechnicianPayrollTicketsBP record
		record := odooms.MSTechnicianPayrollTicketsBP{}

		// Map Excel columns to record fields
		validData := false
		for colIndex, fieldName := range columnMapping {
			if colIndex < len(row) {
				cellValue := strings.TrimSpace(row[colIndex])
				if cellValue != "" {
					validData = true
					// Set field value using reflection
					v := reflect.ValueOf(&record).Elem()
					field := v.FieldByNameFunc(func(name string) bool {
						f, _ := v.Type().FieldByName(name)
						return f.Tag.Get("json") == fieldName
					})

					if field.IsValid() && field.CanSet() {
						switch field.Kind() {
						case reflect.String:
							// Skip setting "n/a" and Excel error values (case insensitive)
							lowerValue := strings.ToLower(cellValue)
							if lowerValue != "n/a" && lowerValue != "na" && lowerValue != "#n/a" &&
								lowerValue != "-" && lowerValue != "" &&
								!strings.HasPrefix(lowerValue, "#") { // Skip Excel error values like #DIV/0!, #REF!, etc.
								field.SetString(cellValue)
							}
						case reflect.Int, reflect.Int32, reflect.Int64:
							if intVal, err := strconv.Atoi(cellValue); err == nil {
								field.SetInt(int64(intVal))
							}
						case reflect.Float32, reflect.Float64:
							if floatVal, err := fun.ParseRupiah(cellValue); err == nil {
								field.SetFloat(floatVal)
							}
						case reflect.Bool:
							// Use Excel boolean parser for better compatibility (YES/NO, Y/N, TRUE/FALSE, etc.)
							if boolVal, err := parseExcelBoolean(cellValue); err == nil {
								field.SetBool(boolVal)
							} else {
								logrus.Warnf("Row %d, Column %d (%s): Failed to parse boolean value '%s': %v",
									rowIndex+1, colIndex, fieldName, cellValue, err)
							}
						case reflect.Ptr:
							// Handle pointer types like *time.Time
							if field.Type().Elem().Kind() == reflect.Struct && field.Type().Elem().String() == "time.Time" {
								if parsedTime, err := fun.ParseFlexibleDate(cellValue); err == nil {
									timePtr := reflect.New(field.Type().Elem())
									timePtr.Elem().Set(reflect.ValueOf(parsedTime))
									field.Set(timePtr)
								}
							}
						}
					}
				}
			}
		}

		// Set the uploaded_by field
		record.UploadedBy = uploadedExcel.Email

		if validData {
			recordsToInsert = append(recordsToInsert, record)
			logMessage := fmt.Sprintf("Row %d: Valid record prepared for insertion", rowIndex+1)
			// logrus.Debug(logMessage)
			logAct[rowIndex+1] = logMessage
			totalSuccess.Add(1)
		} else {
			logMessage := fmt.Sprintf("Row %d: No valid data to insert", rowIndex+1)
			logrus.Debug(logMessage)
			logAct[rowIndex+1] = logMessage
			totalFail.Add(1)
		}

		// Update progress periodically
		if (rowIndex+1)%10 == 0 || rowIndex+1 == totalRows {
			processed := totalSuccess.Load() + totalFail.Load()
			progress := int((processed * 100) / int64(totalRows))
			if progress > 100 {
				progress = 100
			}
			BroadcastUploadedExcelForCreateTechnicianPayslipProgress(uploadedExcel, "Processing", progress, fmt.Sprintf("Processed %d/%d rows", processed, totalRows))
		}
	}

	// Batch insert records
	if len(recordsToInsert) > 0 {
		logrus.Infof("Batch inserting %d records", len(recordsToInsert))
		if err := db.CreateInBatches(&recordsToInsert, 100).Error; err != nil {
			logMessage := fmt.Sprintf("Failed to batch insert records: %v", err)
			logrus.Error(logMessage)
			db.Model(&uploadedExcel).Updates(map[string]interface{}{
				"status": "Failed",
				"log":    logMessage,
			})
			return logMessage
		}
		logrus.Infof("Successfully inserted %d records", len(recordsToInsert))
	}

	jsonLog, _ := json.Marshal(logAct)

	return string(jsonLog)
}

func processTechnicianPayrollIntoPayslipTicketsUnworkedEDC(rows [][]string, totalSuccess, totalFail *atomic.Int64, uploadedExcel odooms.UploadedExcelToODOOMS, loginCookie []*http.Cookie, db *gorm.DB) string {
	logAct := make(map[int]string)
	totalRows := len(rows) - 1

	// Update total row count in database
	db.Model(&uploadedExcel).Update("total_row", totalRows)

	// Check if excel has enough data
	if len(rows) < 2 {
		db.Model(&uploadedExcel).Updates(map[string]interface{}{
			"status": "Failed",
			"log":    "Excel has no data to process!",
		})
		db.Delete(&uploadedExcel)
		return fmt.Sprintf("Excel: %v has no data to process!", uploadedExcel.OriginalFilename)
	}

	// Check if header row has enough columns
	if len(rows[0]) < 3 {
		db.Model(&uploadedExcel).Updates(map[string]interface{}{
			"status": "Failed",
			"log":    "Excel must have at least 3 column in header!",
		})
		db.Delete(&uploadedExcel)
		return fmt.Sprintf("Excel: %v must have at least 3 column in header!", uploadedExcel.OriginalFilename)
	}

	// Build field map
	fieldMap := buildFieldMapFromStruct(odooms.MSTechnicianPayrollTicketsUnworkedEDC{})

	// Map header columns
	headerRow := rows[0]                  // Assuming header is in row 1 (index 0)
	columnMapping := make(map[int]string) // column index -> json field name

	for colIndex, header := range headerRow {
		header = strings.TrimSpace(header)
		if header == "" {
			continue
		}

		lowerHeader := strings.ToLower(header)

		if fieldName, exists := fieldMap[lowerHeader]; exists {
			columnMapping[colIndex] = fieldName
			logrus.Infof("Mapped column %d '%s' to field '%s'", colIndex, header, fieldName)
		} else {
			logrus.Warnf("Column %d '%s' does not match any model struct field", colIndex, header)
		}
	}

	// Check if we have at least one valid column
	if len(columnMapping) < 3 {
		db.Model(&uploadedExcel).Updates(map[string]interface{}{
			"status": "Failed",
			"log":    "Excel must have at least 3 valid column that matches model struct fields!",
		})
		db.Delete(&uploadedExcel)
		return fmt.Sprintf("Excel: %v must have at least 3 valid column that matches model struct fields!", uploadedExcel.OriginalFilename)
	}

	// Check if there are existing rows in model struct table and delete them
	var existingCount int64
	if err := db.Model(&odooms.MSTechnicianPayrollTicketsUnworkedEDC{}).Count(&existingCount).Error; err != nil {
		logMessage := fmt.Sprintf("Failed to count existing model struct records: %v", err)
		logrus.Error(logMessage)
		db.Model(&uploadedExcel).Updates(map[string]interface{}{
			"status": "Failed",
			"log":    logMessage,
		})
		return logMessage
	}

	if existingCount > 0 {
		logrus.Infof("Found %d existing MSTechnicianPayrollTicketsUnworkedEDC records, deleting them", existingCount)
		if err := db.Where("1=1").Delete(&odooms.MSTechnicianPayrollTicketsUnworkedEDC{}).Error; err != nil {
			logMessage := fmt.Sprintf("Failed to delete existing MSTechnicianPayrollTicketsUnworkedEDC records: %v", err)
			logrus.Error(logMessage)
			db.Model(&uploadedExcel).Updates(map[string]interface{}{
				"status": "Failed",
				"log":    logMessage,
			})
			return logMessage
		}
		logrus.Infof("Successfully deleted %d existing MSTechnicianPayrollTicketsUnworkedEDC records", existingCount)
	}

	// Collect valid records for batch insert
	var recordsToInsert []odooms.MSTechnicianPayrollTicketsUnworkedEDC

	// Process data rows
	for rowIndex, row := range rows[1:] {
		// Skip empty rows
		if len(row) == 0 {
			continue
		}

		// Check if row has any data
		hasData := false
		for _, cell := range row {
			if strings.TrimSpace(cell) != "" {
				hasData = true
				break
			}
		}

		if !hasData {
			logMessage := fmt.Sprintf("Row %d: Empty row, skipping", rowIndex+1)
			logrus.Debug(logMessage)
			continue
		}

		// Create new MSTechnicianPayrollTicketsUnworkedEDC record
		record := odooms.MSTechnicianPayrollTicketsUnworkedEDC{}

		// Map Excel columns to record fields
		validData := false
		for colIndex, fieldName := range columnMapping {
			if colIndex < len(row) {
				cellValue := strings.TrimSpace(row[colIndex])
				if cellValue != "" {
					validData = true
					// Set field value using reflection
					v := reflect.ValueOf(&record).Elem()
					field := v.FieldByNameFunc(func(name string) bool {
						f, _ := v.Type().FieldByName(name)
						return f.Tag.Get("json") == fieldName
					})

					if field.IsValid() && field.CanSet() {
						switch field.Kind() {
						case reflect.String:
							// Skip setting "n/a" and Excel error values (case insensitive)
							lowerValue := strings.ToLower(cellValue)
							if lowerValue != "n/a" && lowerValue != "na" && lowerValue != "#n/a" &&
								lowerValue != "-" && lowerValue != "" &&
								!strings.HasPrefix(lowerValue, "#") { // Skip Excel error values like #DIV/0!, #REF!, etc.
								field.SetString(cellValue)
							}
						case reflect.Int, reflect.Int32, reflect.Int64:
							if intVal, err := strconv.Atoi(cellValue); err == nil {
								field.SetInt(int64(intVal))
							}
						case reflect.Float32, reflect.Float64:
							if floatVal, err := fun.ParseRupiah(cellValue); err == nil {
								field.SetFloat(floatVal)
							}
						case reflect.Bool:
							// Use Excel boolean parser for better compatibility (YES/NO, Y/N, TRUE/FALSE, etc.)
							if boolVal, err := parseExcelBoolean(cellValue); err == nil {
								field.SetBool(boolVal)
							} else {
								logrus.Warnf("Row %d, Column %d (%s): Failed to parse boolean value '%s': %v",
									rowIndex+1, colIndex, fieldName, cellValue, err)
							}
						case reflect.Ptr:
							// Handle pointer types like *time.Time
							if field.Type().Elem().Kind() == reflect.Struct && field.Type().Elem().String() == "time.Time" {
								if parsedTime, err := fun.ParseFlexibleDate(cellValue); err == nil {
									timePtr := reflect.New(field.Type().Elem())
									timePtr.Elem().Set(reflect.ValueOf(parsedTime))
									field.Set(timePtr)
								}
							}
						}
					}
				}
			}
		}

		// Set the uploaded_by field
		record.UploadedBy = uploadedExcel.Email

		if validData {
			recordsToInsert = append(recordsToInsert, record)
			logMessage := fmt.Sprintf("Row %d: Valid record prepared for insertion", rowIndex+1)
			// logrus.Debug(logMessage)
			logAct[rowIndex+1] = logMessage
			totalSuccess.Add(1)
		} else {
			logMessage := fmt.Sprintf("Row %d: No valid data to insert", rowIndex+1)
			logrus.Debug(logMessage)
			logAct[rowIndex+1] = logMessage
			totalFail.Add(1)
		}

		// Update progress periodically
		if (rowIndex+1)%10 == 0 || rowIndex+1 == totalRows {
			processed := totalSuccess.Load() + totalFail.Load()
			progress := int((processed * 100) / int64(totalRows))
			if progress > 100 {
				progress = 100
			}
			BroadcastUploadedExcelForCreateTechnicianPayslipProgress(uploadedExcel, "Processing", progress, fmt.Sprintf("Processed %d/%d rows", processed, totalRows))
		}
	}

	// Batch insert records
	if len(recordsToInsert) > 0 {
		logrus.Infof("Batch inserting %d records", len(recordsToInsert))
		if err := db.CreateInBatches(&recordsToInsert, 100).Error; err != nil {
			logMessage := fmt.Sprintf("Failed to batch insert records: %v", err)
			logrus.Error(logMessage)
			db.Model(&uploadedExcel).Updates(map[string]interface{}{
				"status": "Failed",
				"log":    logMessage,
			})
			return logMessage
		}
		logrus.Infof("Successfully inserted %d records", len(recordsToInsert))
	}

	jsonLog, _ := json.Marshal(logAct)

	return string(jsonLog)
}

func processTechnicianPayrollIntoPayslipTicketsRegularATM(rows [][]string, totalSuccess, totalFail *atomic.Int64, uploadedExcel odooms.UploadedExcelToODOOMS, loginCookie []*http.Cookie, db *gorm.DB) string {
	logAct := make(map[int]string)
	totalRows := len(rows) - 1

	// Update total row count in database
	db.Model(&uploadedExcel).Update("total_row", totalRows)

	// Check if excel has enough data
	if len(rows) < 2 {
		db.Model(&uploadedExcel).Updates(map[string]interface{}{
			"status": "Failed",
			"log":    "Excel has no data to process!",
		})
		db.Delete(&uploadedExcel)
		return fmt.Sprintf("Excel: %v has no data to process!", uploadedExcel.OriginalFilename)
	}

	// Check if header row has enough columns
	if len(rows[0]) < 3 {
		db.Model(&uploadedExcel).Updates(map[string]interface{}{
			"status": "Failed",
			"log":    "Excel must have at least 3 column in header!",
		})
		db.Delete(&uploadedExcel)
		return fmt.Sprintf("Excel: %v must have at least 3 column in header!", uploadedExcel.OriginalFilename)
	}

	// Build field map
	fieldMap := buildFieldMapFromStruct(odooms.MSTechnicianPayrollTicketsRegularATM{})

	// Map header columns
	headerRow := rows[0]                  // Assuming header is in row 1 (index 0)
	columnMapping := make(map[int]string) // column index -> json field name

	for colIndex, header := range headerRow {
		header = strings.TrimSpace(header)
		if header == "" {
			continue
		}

		lowerHeader := strings.ToLower(header)

		switch lowerHeader {
		case "re-assigned":
			lowerHeader = "re assigned"
		case "sla status (assume)":
			lowerHeader = "sla status assume"
		case "why ?":
			lowerHeader = "why"
		}

		if fieldName, exists := fieldMap[lowerHeader]; exists {
			columnMapping[colIndex] = fieldName
			logrus.Infof("Mapped column %d '%s' to field '%s'", colIndex, header, fieldName)
		} else {
			logrus.Warnf("Column %d '%s' does not match any model struct field", colIndex, header)
		}
	}

	// Check if we have at least one valid column
	if len(columnMapping) < 3 {
		db.Model(&uploadedExcel).Updates(map[string]interface{}{
			"status": "Failed",
			"log":    "Excel must have at least 3 valid column that matches model struct fields!",
		})
		db.Delete(&uploadedExcel)
		return fmt.Sprintf("Excel: %v must have at least 3 valid column that matches model struct fields!", uploadedExcel.OriginalFilename)
	}

	// Check if there are existing rows in model struct table and delete them
	var existingCount int64
	if err := db.Model(&odooms.MSTechnicianPayrollTicketsRegularATM{}).Count(&existingCount).Error; err != nil {
		logMessage := fmt.Sprintf("Failed to count existing model struct records: %v", err)
		logrus.Error(logMessage)
		db.Model(&uploadedExcel).Updates(map[string]interface{}{
			"status": "Failed",
			"log":    logMessage,
		})
		return logMessage
	}

	if existingCount > 0 {
		logrus.Infof("Found %d existing MSTechnicianPayrollTicketsRegularATM records, deleting them", existingCount)
		if err := db.Where("1=1").Delete(&odooms.MSTechnicianPayrollTicketsRegularATM{}).Error; err != nil {
			logMessage := fmt.Sprintf("Failed to delete existing MSTechnicianPayrollTicketsRegularATM records: %v", err)
			logrus.Error(logMessage)
			db.Model(&uploadedExcel).Updates(map[string]interface{}{
				"status": "Failed",
				"log":    logMessage,
			})
			return logMessage
		}
		logrus.Infof("Successfully deleted %d existing MSTechnicianPayrollTicketsRegularATM records", existingCount)
	}

	// Collect valid records for batch insert
	var recordsToInsert []odooms.MSTechnicianPayrollTicketsRegularATM

	// Process data rows
	for rowIndex, row := range rows[1:] {
		// Skip empty rows
		if len(row) == 0 {
			continue
		}

		// Check if row has any data
		hasData := false
		for _, cell := range row {
			if strings.TrimSpace(cell) != "" {
				hasData = true
				break
			}
		}

		if !hasData {
			logMessage := fmt.Sprintf("Row %d: Empty row, skipping", rowIndex+1)
			logrus.Debug(logMessage)
			continue
		}

		// Create new MSTechnicianPayrollTicketsRegularATM record
		record := odooms.MSTechnicianPayrollTicketsRegularATM{}

		// Map Excel columns to record fields
		validData := false
		for colIndex, fieldName := range columnMapping {
			if colIndex < len(row) {
				cellValue := strings.TrimSpace(row[colIndex])
				if cellValue != "" {
					validData = true
					// Set field value using reflection
					v := reflect.ValueOf(&record).Elem()
					field := v.FieldByNameFunc(func(name string) bool {
						f, _ := v.Type().FieldByName(name)
						return f.Tag.Get("json") == fieldName
					})

					if field.IsValid() && field.CanSet() {
						switch field.Kind() {
						case reflect.String:
							// Skip setting "n/a" and Excel error values (case insensitive)
							lowerValue := strings.ToLower(cellValue)
							if lowerValue != "n/a" && lowerValue != "na" && lowerValue != "#n/a" &&
								lowerValue != "-" && lowerValue != "" &&
								!strings.HasPrefix(lowerValue, "#") { // Skip Excel error values like #DIV/0!, #REF!, etc.
								field.SetString(cellValue)
							}
						case reflect.Int, reflect.Int32, reflect.Int64:
							if intVal, err := strconv.Atoi(cellValue); err == nil {
								field.SetInt(int64(intVal))
							}
						case reflect.Float32, reflect.Float64:
							if floatVal, err := fun.ParseRupiah(cellValue); err == nil {
								field.SetFloat(floatVal)
							}
						case reflect.Bool:
							// Use Excel boolean parser for better compatibility (YES/NO, Y/N, TRUE/FALSE, etc.)
							if boolVal, err := parseExcelBoolean(cellValue); err == nil {
								field.SetBool(boolVal)
							} else {
								logrus.Warnf("Row %d, Column %d (%s): Failed to parse boolean value '%s': %v",
									rowIndex+1, colIndex, fieldName, cellValue, err)
							}
						case reflect.Ptr:
							// Handle pointer types like *time.Time
							if field.Type().Elem().Kind() == reflect.Struct && field.Type().Elem().String() == "time.Time" {
								if parsedTime, err := fun.ParseFlexibleDate(cellValue); err == nil {
									timePtr := reflect.New(field.Type().Elem())
									timePtr.Elem().Set(reflect.ValueOf(parsedTime))
									field.Set(timePtr)
								}
							}
						}
					}
				}
			}
		}

		// Set the uploaded_by field
		record.UploadedBy = uploadedExcel.Email

		if validData {
			recordsToInsert = append(recordsToInsert, record)
			logMessage := fmt.Sprintf("Row %d: Valid record prepared for insertion", rowIndex+1)
			// logrus.Debug(logMessage)
			logAct[rowIndex+1] = logMessage
			totalSuccess.Add(1)
		} else {
			logMessage := fmt.Sprintf("Row %d: No valid data to insert", rowIndex+1)
			logrus.Debug(logMessage)
			logAct[rowIndex+1] = logMessage
			totalFail.Add(1)
		}

		// Update progress periodically
		if (rowIndex+1)%10 == 0 || rowIndex+1 == totalRows {
			processed := totalSuccess.Load() + totalFail.Load()
			progress := int((processed * 100) / int64(totalRows))
			if progress > 100 {
				progress = 100
			}
			BroadcastUploadedExcelForCreateTechnicianPayslipProgress(uploadedExcel, "Processing", progress, fmt.Sprintf("Processed %d/%d rows", processed, totalRows))
		}
	}

	// Batch insert records
	if len(recordsToInsert) > 0 {
		logrus.Infof("Batch inserting %d records", len(recordsToInsert))
		if err := db.CreateInBatches(&recordsToInsert, 100).Error; err != nil {
			logMessage := fmt.Sprintf("Failed to batch insert records: %v", err)
			logrus.Error(logMessage)
			db.Model(&uploadedExcel).Updates(map[string]interface{}{
				"status": "Failed",
				"log":    logMessage,
			})
			return logMessage
		}
		logrus.Infof("Successfully inserted %d records", len(recordsToInsert))
	}

	jsonLog, _ := json.Marshal(logAct)

	return string(jsonLog)
}

func processTechnicianPayrollIntoPayslipTicketsUnworkedATM(rows [][]string, totalSuccess, totalFail *atomic.Int64, uploadedExcel odooms.UploadedExcelToODOOMS, loginCookie []*http.Cookie, db *gorm.DB) string {
	logAct := make(map[int]string)
	totalRows := len(rows) - 1

	// Update total row count in database
	db.Model(&uploadedExcel).Update("total_row", totalRows)

	// Check if excel has enough data
	if len(rows) < 2 {
		db.Model(&uploadedExcel).Updates(map[string]interface{}{
			"status": "Failed",
			"log":    "Excel has no data to process!",
		})
		db.Delete(&uploadedExcel)
		return fmt.Sprintf("Excel: %v has no data to process!", uploadedExcel.OriginalFilename)
	}

	// Check if header row has enough columns
	if len(rows[0]) < 3 {
		db.Model(&uploadedExcel).Updates(map[string]interface{}{
			"status": "Failed",
			"log":    "Excel must have at least 3 column in header!",
		})
		db.Delete(&uploadedExcel)
		return fmt.Sprintf("Excel: %v must have at least 3 column in header!", uploadedExcel.OriginalFilename)
	}

	// Build field map
	fieldMap := buildFieldMapFromStruct(odooms.MSTechnicianPayrollTicketsUnworkedATM{})

	// Map header columns
	headerRow := rows[0]                  // Assuming header is in row 1 (index 0)
	columnMapping := make(map[int]string) // column index -> json field name

	for colIndex, header := range headerRow {
		header = strings.TrimSpace(header)
		if header == "" {
			continue
		}

		lowerHeader := strings.ToLower(header)

		if fieldName, exists := fieldMap[lowerHeader]; exists {
			columnMapping[colIndex] = fieldName
			logrus.Infof("Mapped column %d '%s' to field '%s'", colIndex, header, fieldName)
		} else {
			logrus.Warnf("Column %d '%s' does not match any model struct field", colIndex, header)
		}
	}

	// Check if we have at least one valid column
	if len(columnMapping) < 3 {
		db.Model(&uploadedExcel).Updates(map[string]interface{}{
			"status": "Failed",
			"log":    "Excel must have at least 3 valid column that matches model struct fields!",
		})
		db.Delete(&uploadedExcel)
		return fmt.Sprintf("Excel: %v must have at least 3 valid column that matches model struct fields!", uploadedExcel.OriginalFilename)
	}

	// Check if there are existing rows in model struct table and delete them
	var existingCount int64
	if err := db.Model(&odooms.MSTechnicianPayrollTicketsUnworkedATM{}).Count(&existingCount).Error; err != nil {
		logMessage := fmt.Sprintf("Failed to count existing model struct records: %v", err)
		logrus.Error(logMessage)
		db.Model(&uploadedExcel).Updates(map[string]interface{}{
			"status": "Failed",
			"log":    logMessage,
		})
		return logMessage
	}

	if existingCount > 0 {
		logrus.Infof("Found %d existing MSTechnicianPayrollTicketsUnworkedATM records, deleting them", existingCount)
		if err := db.Where("1=1").Delete(&odooms.MSTechnicianPayrollTicketsUnworkedATM{}).Error; err != nil {
			logMessage := fmt.Sprintf("Failed to delete existing MSTechnicianPayrollTicketsUnworkedATM records: %v", err)
			logrus.Error(logMessage)
			db.Model(&uploadedExcel).Updates(map[string]interface{}{
				"status": "Failed",
				"log":    logMessage,
			})
			return logMessage
		}
		logrus.Infof("Successfully deleted %d existing MSTechnicianPayrollTicketsUnworkedATM records", existingCount)
	}

	// Collect valid records for batch insert
	var recordsToInsert []odooms.MSTechnicianPayrollTicketsUnworkedATM

	// Process data rows
	for rowIndex, row := range rows[1:] {
		// Skip empty rows
		if len(row) == 0 {
			continue
		}

		// Check if row has any data
		hasData := false
		for _, cell := range row {
			if strings.TrimSpace(cell) != "" {
				hasData = true
				break
			}
		}

		if !hasData {
			logMessage := fmt.Sprintf("Row %d: Empty row, skipping", rowIndex+1)
			logrus.Debug(logMessage)
			continue
		}

		// Create new MSTechnicianPayrollTicketsUnworkedATM record
		record := odooms.MSTechnicianPayrollTicketsUnworkedATM{}

		// Map Excel columns to record fields
		validData := false
		for colIndex, fieldName := range columnMapping {
			if colIndex < len(row) {
				cellValue := strings.TrimSpace(row[colIndex])
				if cellValue != "" {
					validData = true
					// Set field value using reflection
					v := reflect.ValueOf(&record).Elem()
					field := v.FieldByNameFunc(func(name string) bool {
						f, _ := v.Type().FieldByName(name)
						return f.Tag.Get("json") == fieldName
					})

					if field.IsValid() && field.CanSet() {
						switch field.Kind() {
						case reflect.String:
							// Skip setting "n/a" and Excel error values (case insensitive)
							lowerValue := strings.ToLower(cellValue)
							if lowerValue != "n/a" && lowerValue != "na" && lowerValue != "#n/a" &&
								lowerValue != "-" && lowerValue != "" &&
								!strings.HasPrefix(lowerValue, "#") { // Skip Excel error values like #DIV/0!, #REF!, etc.
								field.SetString(cellValue)
							}
						case reflect.Int, reflect.Int32, reflect.Int64:
							if intVal, err := strconv.Atoi(cellValue); err == nil {
								field.SetInt(int64(intVal))
							}
						case reflect.Float32, reflect.Float64:
							if floatVal, err := fun.ParseRupiah(cellValue); err == nil {
								field.SetFloat(floatVal)
							}
						case reflect.Bool:
							// Use Excel boolean parser for better compatibility (YES/NO, Y/N, TRUE/FALSE, etc.)
							if boolVal, err := parseExcelBoolean(cellValue); err == nil {
								field.SetBool(boolVal)
							} else {
								logrus.Warnf("Row %d, Column %d (%s): Failed to parse boolean value '%s': %v",
									rowIndex+1, colIndex, fieldName, cellValue, err)
							}
						case reflect.Ptr:
							// Handle pointer types like *time.Time
							if field.Type().Elem().Kind() == reflect.Struct && field.Type().Elem().String() == "time.Time" {
								if parsedTime, err := fun.ParseFlexibleDate(cellValue); err == nil {
									timePtr := reflect.New(field.Type().Elem())
									timePtr.Elem().Set(reflect.ValueOf(parsedTime))
									field.Set(timePtr)
								}
							}
						}
					}
				}
			}
		}

		// Set the uploaded_by field
		record.UploadedBy = uploadedExcel.Email

		if validData {
			recordsToInsert = append(recordsToInsert, record)
			logMessage := fmt.Sprintf("Row %d: Valid record prepared for insertion", rowIndex+1)
			// logrus.Debug(logMessage)
			logAct[rowIndex+1] = logMessage
			totalSuccess.Add(1)
		} else {
			logMessage := fmt.Sprintf("Row %d: No valid data to insert", rowIndex+1)
			logrus.Debug(logMessage)
			logAct[rowIndex+1] = logMessage
			totalFail.Add(1)
		}

		// Update progress periodically
		if (rowIndex+1)%10 == 0 || rowIndex+1 == totalRows {
			processed := totalSuccess.Load() + totalFail.Load()
			progress := int((processed * 100) / int64(totalRows))
			if progress > 100 {
				progress = 100
			}
			BroadcastUploadedExcelForCreateTechnicianPayslipProgress(uploadedExcel, "Processing", progress, fmt.Sprintf("Processed %d/%d rows", processed, totalRows))
		}
	}

	// Batch insert records
	if len(recordsToInsert) > 0 {
		logrus.Infof("Batch inserting %d records", len(recordsToInsert))
		if err := db.CreateInBatches(&recordsToInsert, 100).Error; err != nil {
			logMessage := fmt.Sprintf("Failed to batch insert records: %v", err)
			logrus.Error(logMessage)
			db.Model(&uploadedExcel).Updates(map[string]interface{}{
				"status": "Failed",
				"log":    logMessage,
			})
			return logMessage
		}
		logrus.Infof("Successfully inserted %d records", len(recordsToInsert))
	}

	jsonLog, _ := json.Marshal(logAct)

	return string(jsonLog)
}

func processTechnicianPayrollDedicatedATM(rows [][]string, totalSuccess, totalFail *atomic.Int64, uploadedExcel odooms.UploadedExcelToODOOMS, loginCookie []*http.Cookie, db *gorm.DB) string {
	logAct := make(map[int]string)
	totalRows := len(rows) - 1

	// Update total row count in database
	db.Model(&uploadedExcel).Update("total_row", totalRows)

	// Check if excel has enough data
	if len(rows) < 2 {
		db.Model(&uploadedExcel).Updates(map[string]interface{}{
			"status": "Failed",
			"log":    "Excel has no data to process!",
		})
		db.Delete(&uploadedExcel)
		return fmt.Sprintf("Excel: %v has no data to process!", uploadedExcel.OriginalFilename)
	}

	// Build field map
	fieldMap := buildFieldMapFromStruct(odooms.MSTechnicianPayrollDedicatedATM{})

	// Map header columns - safely determine which row contains the header
	var headerRow []string
	var indexHeaderRow int = 1
	var containsColName bool = false

	// Check if row 7 (index 6) exists and contains "name" column
	if len(rows) > 6 {
		for _, header := range rows[6] {
			if strings.ToLower(strings.TrimSpace(header)) == "name" {
				containsColName = true
				headerRow = rows[6]
				indexHeaderRow = 7
				break
			}
		}
	}

	// Fallback to first row if header not found in row 7
	if !containsColName {
		if len(rows[0]) < 7 {
			db.Model(&uploadedExcel).Updates(map[string]interface{}{
				"status": "Failed",
				"log":    "Excel header must have at least 7 columns!",
			})
			db.Delete(&uploadedExcel)
			return fmt.Sprintf("Excel: %v header must have at least 7 columns!", uploadedExcel.OriginalFilename)
		}
		headerRow = rows[0]
		indexHeaderRow = 1
	}

	columnMapping := make(map[int]string) // column index -> json field name

	for colIndex, header := range headerRow {
		header = strings.TrimSpace(header)
		if header == "" {
			continue
		}

		lowerHeader := strings.ToLower(header)

		switch colIndex {
		case 7: // PM Meet
			lowerHeader = "pm meet"
		case 8: // PM Over
			lowerHeader = "pm over"
		case 9: // PM Unworked
			lowerHeader = "pm unworked"
		case 10: // Non PM Meet
			lowerHeader = "non pm meet"
		case 11: // Non PM Over
			lowerHeader = "non pm over"
		case 12: // Non PM Unworked
			lowerHeader = "non pm unworked"
		case 13: // Incentive/Ticket
			lowerHeader = "incentive per ticket"
		case 21: // Potongan Overdue PM
			lowerHeader = "potongan overdue pm"
		case 22: // Potongan Overdue NPM
			lowerHeader = "potongan overdue non pm"
		case 23: // Potongan Overdue Unworked PM
			lowerHeader = "potongan overdue unworked pm"
		case 24: // Potongan Overdue Unworked NPM
			lowerHeader = "potongan overdue unworked non pm"
		case 25: // Total Potongan Overdue
			lowerHeader = "total potongan overdue"
		case 26: // Total Potongan Unworked
			lowerHeader = "total potongan unworked"
		case 27: // Total Potongan Total
			lowerHeader = "total potongan total"
		case 35: // JO BP (All)
			lowerHeader = "jo bp all"
		}

		if fieldName, exists := fieldMap[lowerHeader]; exists {
			columnMapping[colIndex] = fieldName
			logrus.Infof("Mapped column %d '%s' to field '%s'", colIndex, header, fieldName)
		} else {
			logrus.Warnf("Column %d '%s' does not match any model struct field", colIndex, header)
		}
	}

	// Check if we have at least one valid column
	if len(columnMapping) < 3 {
		db.Model(&uploadedExcel).Updates(map[string]interface{}{
			"status": "Failed",
			"log":    "Excel must have at least 3 valid column that matches model struct fields!",
		})
		db.Delete(&uploadedExcel)
		return fmt.Sprintf("Excel: %v must have at least 3 valid column that matches model struct fields!", uploadedExcel.OriginalFilename)
	}

	// Check if there are existing rows in model struct table and delete them
	var existingCount int64
	if err := db.Model(&odooms.MSTechnicianPayrollDedicatedATM{}).Count(&existingCount).Error; err != nil {
		logMessage := fmt.Sprintf("Failed to count existing model struct records: %v", err)
		logrus.Error(logMessage)
		db.Model(&uploadedExcel).Updates(map[string]interface{}{
			"status": "Failed",
			"log":    logMessage,
		})
		return logMessage
	}

	if existingCount > 0 {
		logrus.Infof("Found %d existing MSTechnicianPayrollDedicatedATM records, deleting them", existingCount)
		if err := db.Where("1=1").Delete(&odooms.MSTechnicianPayrollDedicatedATM{}).Error; err != nil {
			logMessage := fmt.Sprintf("Failed to delete existing MSTechnicianPayrollDedicatedATM records: %v", err)
			logrus.Error(logMessage)
			db.Model(&uploadedExcel).Updates(map[string]interface{}{
				"status": "Failed",
				"log":    logMessage,
			})
			return logMessage
		}
		logrus.Infof("Successfully deleted %d existing MSTechnicianPayrollDedicatedATM records", existingCount)
	}

	// Collect valid records for batch insert
	var recordsToInsert []odooms.MSTechnicianPayrollDedicatedATM

	// Process data rows
	for rowIndex, row := range rows[indexHeaderRow:] {
		// Skip empty rows
		if len(row) == 0 {
			continue
		}

		// Skip if column C (index 2) is empty
		if len(row) > 2 && strings.TrimSpace(row[2]) == "" {
			logMessage := fmt.Sprintf("Row %d: Column C is empty, skipping", rowIndex+1)
			logrus.Debug(logMessage)
			continue
		}

		// Check if row has any data
		hasData := false
		for _, cell := range row {
			if strings.TrimSpace(cell) != "" {
				hasData = true
				break
			}
		}

		if !hasData {
			logMessage := fmt.Sprintf("Row %d: Empty row, skipping", rowIndex+1)
			logrus.Debug(logMessage)
			continue
		}

		// Create new MSTechnicianPayrollDedicatedATM record
		record := odooms.MSTechnicianPayrollDedicatedATM{}

		// Map Excel columns to record fields
		validData := false
		for colIndex, fieldName := range columnMapping {
			if colIndex < len(row) {
				cellValue := strings.TrimSpace(row[colIndex])
				if cellValue != "" {
					validData = true
					// Set field value using reflection
					v := reflect.ValueOf(&record).Elem()
					field := v.FieldByNameFunc(func(name string) bool {
						f, _ := v.Type().FieldByName(name)
						return f.Tag.Get("json") == fieldName
					})

					if field.IsValid() && field.CanSet() {
						switch field.Kind() {
						case reflect.String:
							// Skip setting "n/a" and Excel error values (case insensitive)
							lowerValue := strings.ToLower(cellValue)
							if lowerValue != "n/a" && lowerValue != "na" && lowerValue != "#n/a" &&
								lowerValue != "-" && lowerValue != "" &&
								!strings.HasPrefix(lowerValue, "#") { // Skip Excel error values like #DIV/0!, #REF!, etc.
								field.SetString(cellValue)
							}
						case reflect.Int, reflect.Int32, reflect.Int64:
							if intVal, err := strconv.Atoi(cellValue); err == nil {
								field.SetInt(int64(intVal))
							}
						case reflect.Float32, reflect.Float64:
							if floatVal, err := fun.ParseRupiah(cellValue); err == nil {
								field.SetFloat(floatVal)
							}
						case reflect.Bool:
							// Use Excel boolean parser for better compatibility (YES/NO, Y/N, TRUE/FALSE, etc.)
							if boolVal, err := parseExcelBoolean(cellValue); err == nil {
								field.SetBool(boolVal)
							} else {
								logrus.Warnf("Row %d, Column %d (%s): Failed to parse boolean value '%s': %v",
									rowIndex+1, colIndex, fieldName, cellValue, err)
							}
						case reflect.Ptr:
							// Handle pointer types like *time.Time
							if field.Type().Elem().Kind() == reflect.Struct && field.Type().Elem().String() == "time.Time" {
								if parsedTime, err := fun.ParseFlexibleDate(cellValue); err == nil {
									timePtr := reflect.New(field.Type().Elem())
									timePtr.Elem().Set(reflect.ValueOf(parsedTime))
									field.Set(timePtr)
								}
							}
						}
					}
				}
			}
		}

		// Set the uploaded_by field
		record.UploadedBy = uploadedExcel.Email

		var odooTech odooms.ODOOMSTechnicianData
		if err := db.Where("technician = ?", record.Name).First(&odooTech).Error; err == nil {
			if odooTech.NoHP != "" {
				record.NoHP = odooTech.NoHP
			}
		}

		if validData {
			recordsToInsert = append(recordsToInsert, record)
			logMessage := fmt.Sprintf("Row %d: Valid record prepared for insertion", rowIndex+1)
			// logrus.Debug(logMessage)
			logAct[rowIndex+1] = logMessage
			totalSuccess.Add(1)
		} else {
			logMessage := fmt.Sprintf("Row %d: No valid data to insert", rowIndex+1)
			logrus.Debug(logMessage)
			logAct[rowIndex+1] = logMessage
			totalFail.Add(1)
		}

		// Update progress periodically
		if (rowIndex+1)%10 == 0 || rowIndex+1 == totalRows {
			processed := totalSuccess.Load() + totalFail.Load()
			progress := int((processed * 100) / int64(totalRows))
			if progress > 100 {
				progress = 100
			}
			BroadcastUploadedExcelForCreateTechnicianPayslipProgress(uploadedExcel, "Processing", progress, fmt.Sprintf("Processed %d/%d rows", processed, totalRows))
		}
	}

	// Batch insert records
	if len(recordsToInsert) > 0 {
		logrus.Infof("Batch inserting %d records", len(recordsToInsert))
		if err := db.CreateInBatches(&recordsToInsert, 100).Error; err != nil {
			logMessage := fmt.Sprintf("Failed to batch insert records: %v", err)
			logrus.Error(logMessage)
			db.Model(&uploadedExcel).Updates(map[string]interface{}{
				"status": "Failed",
				"log":    logMessage,
			})
			return logMessage
		}
		logrus.Infof("Successfully inserted %d records", len(recordsToInsert))
	}

	jsonLog, _ := json.Marshal(logAct)

	return string(jsonLog)
}

/*
	Update Project.Task
*/

// ProcessUploadedExcelofODOOMSMustUpdatedTask processes the uploaded excel for updating tasks in ODOOMS
func ProcessUploadedExcelofODOOMSMustUpdatedTask(db *gorm.DB) {
	processUploadedExcelWorker(db, OperationConfig{
		TemplateID:            7,
		TriggerChannel:        TriggerProcessUploadedExcelforUpdateTaskODOOMS,
		BroadcastProgressFunc: BroadcastUploadedTaskIDForTaskUpdateinODOOMSProgress,
		OperationType:         "update_task",
		RequiredFieldInColumns: []string{
			"id", // First column must be ID for updates
		},
		MinValidColumns: 2,
	})
}

// BroadcastUploadedTaskIDForTaskUpdateinODOOMSProgress broadcasts the progress of updating tasks in ODOOMS
func BroadcastUploadedTaskIDForTaskUpdateinODOOMSProgress(uploadedExcel odooms.UploadedExcelToODOOMS, status string, progress int, logs string) {
	connectionsMu.RLock()
	defer connectionsMu.RUnlock()

	message := UploadedExcelODOOMSProgress{
		OriginalFilename: uploadedExcel.OriginalFilename,
		Filename:         uploadedExcel.Filename,
		Logs:             logs,
		Status:           status,
		Progress:         progress,
	}

	jsonMessage, _ := json.Marshal(message)

	// Store processing state using switch
	switch status {
	case "Processing", "Pending":
		processingFilesMu.Lock()
		processingFiles[uploadedExcel.Filename] = &uploadedExcel
		processingFilesMu.Unlock()
	case "Completed", "Done", "Failed":
		processingFilesMu.Lock()
		delete(processingFiles, uploadedExcel.Filename)
		processingFilesMu.Unlock()
	}

	for conn := range connections {
		err := safeWriteMessage(conn, websocket.TextMessage, jsonMessage)
		if err != nil {
			logrus.Error("WebSocket write error:", err)
			conn.Close()
			delete(connections, conn)
			cleanupConnectionMutex(conn)
		}
	}
}

// processCustomTemplateUpdateTaskInODOOMS processes the custom template for updating tasks in ODOOMS
func processCustomTemplateUpdateTaskInODOOMS(rows [][]string, totalSuccess, totalFail *atomic.Int64, uploadedExcel odooms.UploadedExcelToODOOMS, loginCookie []*http.Cookie, db *gorm.DB) string {
	var wg sync.WaitGroup
	sem := make(chan struct{}, config.WebPanel.Get().Default.ConcurrencyLimit)
	logAct := make(map[int]string)
	odooModel := "project.task"
	totalRows := len(rows) - 1

	// Update total row count in database
	db.Model(&uploadedExcel).Update("total_row", totalRows)

	// Check if excel has enough data
	if len(rows) < 2 {
		db.Model(&uploadedExcel).Updates(map[string]interface{}{
			"status": "Failed",
			"log":    "Excel has no data to process!",
		})
		db.Delete(&uploadedExcel)
		return fmt.Sprintf("Excel: %v has no data to process!", uploadedExcel.OriginalFilename)
	}

	// Check if header row has enough columns
	if len(rows[0]) < 3 {
		db.Model(&uploadedExcel).Updates(map[string]interface{}{
			"status": "Failed",
			"log":    "Excel must have at least 3 columns in header!",
		})
		db.Delete(&uploadedExcel)
		return fmt.Sprintf("Excel: %v must have at least 3 columns in header!", uploadedExcel.OriginalFilename)
	}

	// Get ODOO MS task fields for validation - USE HELPER
	fieldMap, err := buildFieldMapProjTask(db)
	if err != nil {
		return err.Error()
	}

	// Validate required columns in order - USE NEW HELPER FOR TASK FIELDS
	headerRow := rows[0]
	requiredFields := []string{"id"} // For update, only ID is required in first column
	isValid, validationError := validateRequiredColumnsWithType(headerRow, requiredFields, fieldMap)
	if !isValid {
		db.Model(&uploadedExcel).Updates(map[string]interface{}{
			"status": "Failed",
			"log":    fmt.Sprintf("Column validation failed: %s", validationError),
		})
		db.Delete(&uploadedExcel)
		return fmt.Sprintf("Excel: %v - %s", uploadedExcel.OriginalFilename, validationError)
	}

	columnMapping := mapHeaderColumnsWithType(headerRow, fieldMap)

	// Check if we have enough valid columns (ID column + at least 1 other field)
	if len(columnMapping) < 2 {
		db.Model(&uploadedExcel).Updates(map[string]interface{}{
			"status": "Failed",
			"log":    "Excel must have at least 2 valid columns in header!",
		})
		db.Delete(&uploadedExcel)
		return fmt.Sprintf("Excel: %v must have at least 2 valid columns that match ODOO task fields (ID + at least 1 other field)!", uploadedExcel.OriginalFilename)
	}

	// Process data rows
	batchSize := 100
	dataRows := rows[1:]

	for batchStart := 0; batchStart < len(dataRows); batchStart += batchSize {
		batchEnd := batchStart + batchSize
		if batchEnd > len(dataRows) {
			batchEnd = len(dataRows)
		}

		for i, row := range dataRows[batchStart:batchEnd] {
			rowIndex := batchStart + i
			// Skip empty rows
			if len(row) == 0 {
				continue
			}

			// Check if row has any data
			hasData := false
			for _, cell := range row {
				if strings.TrimSpace(cell) != "" {
					hasData = true
					break
				}
			}

			if !hasData {
				logMessage := fmt.Sprintf("Row %d: Empty row, skipping", rowIndex+1)
				logrus.Debug(logMessage)
				continue
			}

			sem <- struct{}{}
			wg.Add(1)

			go func(rowIndex int, row []string) {
				defer wg.Done()
				defer func() { <-sem }()

				// Check if we have ID in first column
				if len(row) == 0 || strings.TrimSpace(row[0]) == "" {
					logMessage := fmt.Sprintf("Row %d: Missing ID in first column", rowIndex+1)
					logrus.Debug(logMessage)
					logAct[rowIndex+1] = logMessage
					totalFail.Add(1)
					return
				}

				taskID := strings.TrimSpace(row[0])

				// Build ODOO parameters based on column mapping
				odooParams := map[string]interface{}{
					"model": odooModel,
					"id":    taskID, // Always include the ID for update operations
				}

				// Map Excel columns to ODOO fields (skip first column as it's the ID)
				for colIndex, fieldName := range columnMapping {
					if colIndex == 0 {
						// Skip first column as it's already handled as ID
						continue
					}
					if colIndex < len(row) {
						cellValue := strings.TrimSpace(row[colIndex])
						if cellValue != "" {
							// Check if this field is a date/datetime/many2one field and parse it
							lowerHeader := ""
							for headerIndex, headerCol := range headerRow {
								if headerIndex == colIndex {
									lowerHeader = strings.ToLower(strings.TrimSpace(headerCol))
									break
								}
							}

							if fieldInfo, exists := fieldMap[lowerHeader]; exists {
								fieldType := strings.ToLower(fieldInfo.Type)

								switch fieldType {
								case "date", "datetime":
									// Parse date/datetime fields
									parsedDate, err := fun.ParseFlexibleDate(cellValue)
									if err != nil {
										logMessage := fmt.Sprintf("Row %d: Failed to parse date value '%s' for field '%s': %v", rowIndex+1, cellValue, fieldName, err)
										logrus.Warn(logMessage)
										// Use original value if parsing fails
										odooParams[fieldName] = cellValue
									} else {
										// Format according to field type
										if fieldType == "date" {
											odooParams[fieldName] = parsedDate.Format("2006-01-02")
										} else { // datetime
											// odooParams[fieldName] = parsedDate.Format("2006-01-02 15:04:05")
											// -7 Hours
											odooParams[fieldName] = parsedDate.Add(-7 * time.Hour).Format("2006-01-02 15:04:05")
										}
										logrus.Debugf("Row %d: Parsed %s field '%s': '%s' -> '%v'", rowIndex+1, fieldType, fieldName, cellValue, odooParams[fieldName])
									}

								case "many2one":
									// Many2one fields expect a single integer ID
									if id, err := strconv.Atoi(cellValue); err == nil {
										odooParams[fieldName] = id
										logrus.Debugf("Row %d: Converted many2one field '%s': '%s' -> %d", rowIndex+1, fieldName, cellValue, id)
									} else {
										logMessage := fmt.Sprintf("Row %d: many2one field '%s' must be an integer, got '%s'", rowIndex+1, fieldName, cellValue)
										logrus.Warn(logMessage)
										odooParams[fieldName] = cellValue // Keep as string, let ODOO handle the error
									}

								default:
									// Other field types, use value as-is
									odooParams[fieldName] = cellValue
								}
							} else {
								// Field not in fieldMap, use value as-is
								odooParams[fieldName] = cellValue
							}
						}
					}
				}

				// Check if we have at least the required fields (model + id + at least one update field)
				if len(odooParams) < 3 { // model + id + at least one field to update
					logMessage := fmt.Sprintf("Row %d: No valid data to update (ID: %s)", rowIndex+1, taskID)
					logrus.Debug(logMessage)
					logAct[rowIndex+1] = logMessage
					totalFail.Add(1)
					return
				}

				// Remove the 'name' field if it exists, as it's not needed for updates
				delete(odooParams, "name")

				payload := map[string]interface{}{
					"jsonrpc": config.WebPanel.Get().ApiODOO.JSONRPC,
					"params":  odooParams,
				}

				payloadBytes, err := json.Marshal(payload)
				if err != nil {
					logMessage := fmt.Sprintf("Row %d: Failed to marshal JSON payload: %v", rowIndex+1, err)
					logrus.Error(logMessage)
					logAct[rowIndex+1] = logMessage
					totalFail.Add(1)
					return
				}

				// // REMOVE: if you dont need to make it wait
				// // Simulate network delay for testing concurrency
				// time.Sleep(60 * time.Second)

				response, err := updateDataExceltoTaskODOOMS(loginCookie, string(payloadBytes))
				if err != nil {
					if err.Error() == "ODOO Session Expired: Odoo Session Expired" {
						logAct[rowIndex+1] = fmt.Sprintf("Row %d (ID: %s) failed: session expired using email %s, please check if the email/password submitted was incorrect", rowIndex+1, taskID, uploadedExcel.Email)
					} else {
						logAct[rowIndex+1] = fmt.Sprintf("Row %d (ID: %s) got error: %v", rowIndex+1, taskID, err)
					}
					totalFail.Add(1)
				} else {
					logAct[rowIndex+1] = fmt.Sprintf("Row %d (ID: %s) success: %v", rowIndex+1, taskID, response)
					totalSuccess.Add(1)
				}

				db.Model(&uploadedExcel).Updates(map[string]interface{}{
					"total_success": totalSuccess.Load(),
					"total_fail":    totalFail.Load(),
				})

				// Calculate progress based on completed rows vs total rows
				processed := totalSuccess.Load() + totalFail.Load()
				progress := int((processed * 100) / int64(totalRows))
				if progress > 100 {
					progress = 100
				}

				// Send progress update with current row log
				BroadcastUploadedTaskIDForTaskUpdateinODOOMSProgress(uploadedExcel, "Processing", progress, logAct[rowIndex+1])
			}(rowIndex, row)
		}

		wg.Wait()
	}

	jsonLog, _ := json.Marshal(logAct)

	BroadcastUploadedTaskIDForTaskUpdateinODOOMSProgress(uploadedExcel, "Completed", 100, string(jsonLog))

	// Use helper function for notification
	sendNotificationMessage(uploadedExcel, totalSuccess.Load(), totalFail.Load(), logAct, db, "update_task", false)

	return string(jsonLog)
}
