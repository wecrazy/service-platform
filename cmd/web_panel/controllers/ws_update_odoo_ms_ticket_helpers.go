package controllers

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"runtime/debug"
	"service-platform/cmd/web_panel/fun"
	"service-platform/cmd/web_panel/internal/gormdb"
	"service-platform/cmd/web_panel/model"
	odooms "service-platform/cmd/web_panel/model/odoo_ms"
	"service-platform/internal/config"
	"strings"
	"sync/atomic"
	"time"

	"github.com/TigorLazuardi/tanggal"
	"github.com/sirupsen/logrus"
	"github.com/xuri/excelize/v2"
	"gorm.io/gorm"
)

// Helper functions to reduce code duplication in ws_update_odoo_ms_ticket.go

// buildFieldMap creates a lookup map from ticket field descriptions to field names
func buildFieldMap(db *gorm.DB) (map[string]string, error) {
	var ticketFields []odooms.ODOOMSTicketField
	if err := db.Find(&ticketFields).Error; err != nil {
		logrus.Errorf("Failed to fetch ODOO MS ticket fields: %v", err)
		return nil, fmt.Errorf("failed to fetch ticket field definitions: %v", err)
	}

	fieldMap := make(map[string]string) // lowercase description -> field name
	for _, field := range ticketFields {
		fieldMap[strings.ToLower(field.Description)] = field.Name
	}

	return fieldMap, nil
}

// ODOOMSFieldInfo holds both the field name and its type
type ODOOMSFieldInfo struct {
	Name string
	Type string
}

// buildFieldMapProjTask creates a lookup map from task field descriptions to field names and types
func buildFieldMapProjTask(db *gorm.DB) (map[string]ODOOMSFieldInfo, error) {
	var dbData []odooms.ODOOMSTaskField
	if err := db.Find(&dbData).Error; err != nil {
		logrus.Errorf("Failed to fetch ODOO MS task fields: %v", err)
		return nil, fmt.Errorf("failed to fetch task field definitions: %v", err)
	}

	fieldMap := make(map[string]ODOOMSFieldInfo) // lowercase description -> field info (name + type)
	for _, field := range dbData {
		fieldMap[strings.ToLower(field.Description)] = ODOOMSFieldInfo{
			Name: field.Name,
			Type: field.Type,
		}
	}

	return fieldMap, nil
}

// buildFieldMapHelpdeskTicket creates a lookup map from helpdesk ticket field descriptions to field names and types
func buildFieldMapHelpdeskTicket(db *gorm.DB) (map[string]ODOOMSFieldInfo, error) {
	var dbData []odooms.ODOOMSTicketField
	if err := db.Find(&dbData).Error; err != nil {
		logrus.Errorf("Failed to fetch ODOO MS ticket fields: %v", err)
		return nil, fmt.Errorf("failed to fetch ticket field definitions: %v", err)
	}

	fieldMap := make(map[string]ODOOMSFieldInfo) // lowercase description -> field info (name + type)
	for _, field := range dbData {
		fieldMap[strings.ToLower(field.Description)] = ODOOMSFieldInfo{
			Name: field.Name,
			Type: field.Type,
		}
	}

	return fieldMap, nil
}

// validateRequiredColumns checks if the required columns are present in order
func validateRequiredColumns(headerRow []string, requiredFields []string, fieldMap map[string]string) (bool, string) {
	if len(headerRow) < len(requiredFields) {
		return false, fmt.Sprintf("Header has %d columns but %d required fields expected", len(headerRow), len(requiredFields))
	}

	for i, expectedFieldLower := range requiredFields {
		if i >= len(headerRow) {
			return false, fmt.Sprintf("Missing required column at position %d: '%s'", i+1, expectedFieldLower)
		}

		columnHeader := strings.TrimSpace(headerRow[i])
		lowerHeader := strings.ToLower(columnHeader)

		// Direct match
		if lowerHeader == expectedFieldLower {
			continue
		}

		// Check if it maps to the expected field
		if fieldName, exists := fieldMap[lowerHeader]; exists {
			if fieldName == expectedFieldLower || strings.Contains(strings.ToLower(fieldName), expectedFieldLower) {
				continue
			}
		}

		return false, fmt.Sprintf("Column %d must be '%s' but found '%s'", i+1, expectedFieldLower, columnHeader)
	}

	return true, ""
}

// validateRequiredColumnsWithType checks if the required columns are present in order (for ODOOMSFieldInfo)
func validateRequiredColumnsWithType(headerRow []string, requiredFields []string, fieldMap map[string]ODOOMSFieldInfo) (bool, string) {
	if len(headerRow) < len(requiredFields) {
		return false, fmt.Sprintf("Header has %d columns but %d required fields expected", len(headerRow), len(requiredFields))
	}

	for i, expectedFieldLower := range requiredFields {
		if i >= len(headerRow) {
			return false, fmt.Sprintf("Missing required column at position %d: '%s'", i+1, expectedFieldLower)
		}

		columnHeader := strings.TrimSpace(headerRow[i])
		lowerHeader := strings.ToLower(columnHeader)

		// Direct match
		if lowerHeader == expectedFieldLower {
			continue
		}

		// Check if it maps to the expected field
		if fieldInfo, exists := fieldMap[lowerHeader]; exists {
			if fieldInfo.Name == expectedFieldLower || strings.Contains(strings.ToLower(fieldInfo.Name), expectedFieldLower) {
				continue
			}
		}

		return false, fmt.Sprintf("Column %d must be '%s' but found '%s'", i+1, expectedFieldLower, columnHeader)
	}

	return true, ""
}

// validateFirstColumn checks if the first column matches the expected field (DEPRECATED - use validateRequiredColumns)
// func validateFirstColumn(headerRow []string, expectedFieldLower string, fieldMap map[string]string) bool {
// 	if len(headerRow) == 0 {
// 		return false
// 	}

// 	firstColumnHeader := strings.TrimSpace(headerRow[0])
// 	lowerFirstHeader := strings.ToLower(firstColumnHeader)

// 	// Direct match
// 	if lowerFirstHeader == expectedFieldLower {
// 		return true
// 	}

// 	// Check if it maps to the expected field
// 	if fieldName, exists := fieldMap[lowerFirstHeader]; exists {
// 		if fieldName == expectedFieldLower || strings.Contains(strings.ToLower(fieldName), expectedFieldLower) {
// 			return true
// 		}
// 	}

// 	return false
// }

// mapHeaderColumns maps Excel header columns to ODOO field names
func mapHeaderColumns(headerRow []string, fieldMap map[string]string) map[int]string {
	columnMapping := make(map[int]string) // column index -> odoo field name

	for colIndex, header := range headerRow {
		header = strings.TrimSpace(header)
		if header == "" {
			continue
		}

		lowerHeader := strings.ToLower(header)
		if fieldName, exists := fieldMap[lowerHeader]; exists {
			columnMapping[colIndex] = fieldName
			logrus.Infof("Mapped column %d '%s' to field '%s'", colIndex, header, fieldName)
		} else if lowerHeader == "id" || lowerHeader == "subject" {
			// Handle direct ID or Subject columns
			columnMapping[colIndex] = lowerHeader
			logrus.Infof("Mapped column %d '%s' to field '%s'", colIndex, header, lowerHeader)
		} else {
			logrus.Warnf("Column %d '%s' does not match any ODOO ticket field", colIndex, header)
		}
	}

	return columnMapping
}

// mapHeaderColumnsWithType maps Excel header columns to ODOO field names (for ODOOMSFieldInfo)
func mapHeaderColumnsWithType(headerRow []string, fieldMap map[string]ODOOMSFieldInfo) map[int]string {
	columnMapping := make(map[int]string) // column index -> odoo field name

	for colIndex, header := range headerRow {
		header = strings.TrimSpace(header)
		if header == "" {
			continue
		}

		lowerHeader := strings.ToLower(header)
		if fieldInfo, exists := fieldMap[lowerHeader]; exists {
			columnMapping[colIndex] = fieldInfo.Name
			logrus.Infof("Mapped column %d '%s' to field '%s' (type: %s)", colIndex, header, fieldInfo.Name, fieldInfo.Type)
		} else if lowerHeader == "id" || lowerHeader == "subject" {
			// Handle direct ID or Subject columns
			columnMapping[colIndex] = lowerHeader
			logrus.Infof("Mapped column %d '%s' to field '%s'", colIndex, header, lowerHeader)
		} else {
			logrus.Warnf("Column %d '%s' does not match any ODOO task field", colIndex, header)
		}
	}

	return columnMapping
}

// sendNotificationMessage sends WhatsApp notification to the user
// operationType: "update", "create", "archive", "delete", etc. - use switch-case for custom messages
// isMultiSheet: set to true if processing multiple sheets (will format message differently)
func sendNotificationMessage(
	uploadedExcel odooms.UploadedExcelToODOOMS,
	totalSuccess, totalFail int64,
	logAct map[int]string,
	db *gorm.DB,
	operationType string,
	isMultiSheet bool,
) {
	var userRequest model.Admin
	if err := db.Where("email = ?", uploadedExcel.Email).First(&userRequest).Error; err != nil {
		logrus.Errorf("Failed to fetch user request for email %s: %v", uploadedExcel.Email, err)
		return
	}

	phoneNumber := userRequest.Phone
	sanitizedPhoneNumber, err := fun.SanitizePhoneNumber(phoneNumber)
	if err != nil {
		logrus.Errorf("Failed to sanitize phone number %s: %v", phoneNumber, err)
		return
	}

	var jidString string
	if config.WebPanel.Get().UploadedExcelForODOOMS.ActiveDebug {
		jidString = config.WebPanel.Get().Whatsmeow.WaSuperUser + "@s.whatsapp.net"
	} else {
		jidString = "62" + sanitizedPhoneNumber + "@s.whatsapp.net"
	}

	var sbID, sbEN strings.Builder
	// Pretty-print the logAct JSON for better readability
	var prettyLog string
	if len(logAct) > 0 {
		// For multi-sheet, logAct contains sheet summaries (key 0 = overall summary)
		// For single-sheet, logAct contains row-by-row processing details
		if isMultiSheet {
			prettyLines := make([]string, 0, len(logAct))

			// Add overall summary first (key 0)
			if overallMsg, exists := logAct[0]; exists {
				prettyLines = append(prettyLines, fmt.Sprintf("📊 %s\n", overallMsg))
			}

			// Add per-sheet details in order (keys 1, 2, 3, ...)
			for i := 1; i < len(logAct); i++ {
				if msg, exists := logAct[i]; exists {
					prettyLines = append(prettyLines, fmt.Sprintf("  Sheet %d: %s", i, msg))
				}
			}

			prettyLog = strings.Join(prettyLines, "\n")
		} else {
			// Limit row details to first 20 rows to avoid message overflow
			maxRows := 20
			prettyLines := make([]string, 0, len(logAct))
			count := 0
			for row, msg := range logAct {
				if count >= maxRows {
					prettyLines = append(prettyLines, fmt.Sprintf("... and %d more rows", len(logAct)-maxRows))
					break
				}
				prettyLines = append(prettyLines, fmt.Sprintf("Row %d: %s", row, msg))
				count++
			}
			prettyLog = strings.Join(prettyLines, "\n")
		}
	} else {
		prettyLog = "No details available."
	}

	// Build messages based on operation type using switch-case
	sbID.WriteString(fmt.Sprintf("👋 Halo %s (%s),\n\n", userRequest.Fullname, userRequest.Email))
	sbEN.WriteString(fmt.Sprintf("👋 Hello %s (%s),\n\n", userRequest.Fullname, userRequest.Email))

	// Add multi-sheet indicator
	sheetIndicatorID := ""
	sheetIndicatorEN := ""
	if isMultiSheet {
		sheetIndicatorID = " (Multi-Sheet)"
		sheetIndicatorEN = " (Multi-Sheet)"
	}

	switch strings.ToLower(operationType) {
	case "update":
		sbID.WriteString(fmt.Sprintf("📄 Permintaan update tiket Anda untuk file *%s*%s telah diproses.\n\n", uploadedExcel.OriginalFilename, sheetIndicatorID))
		sbEN.WriteString(fmt.Sprintf("📄 Your ticket update request for file *%s*%s has been processed.\n\n", uploadedExcel.OriginalFilename, sheetIndicatorEN))
	case "create":
		sbID.WriteString(fmt.Sprintf("📄 Permintaan pembuatan tiket Anda untuk file *%s*%s telah diproses.\n\n", uploadedExcel.OriginalFilename, sheetIndicatorID))
		sbEN.WriteString(fmt.Sprintf("📄 Your ticket creation request for file *%s*%s has been processed.\n\n", uploadedExcel.OriginalFilename, sheetIndicatorEN))
	case "archive":
		sbID.WriteString(fmt.Sprintf("📄 Permintaan pengarsipan tiket Anda untuk file *%s*%s telah diproses.\n\n", uploadedExcel.OriginalFilename, sheetIndicatorID))
		sbEN.WriteString(fmt.Sprintf("📄 Your ticket archive request for file *%s*%s has been processed.\n\n", uploadedExcel.OriginalFilename, sheetIndicatorEN))
	case "delete":
		sbID.WriteString(fmt.Sprintf("📄 Permintaan penghapusan tiket Anda untuk file *%s*%s telah diproses.\n\n", uploadedExcel.OriginalFilename, sheetIndicatorID))
		sbEN.WriteString(fmt.Sprintf("📄 Your ticket deletion request for file *%s*%s has been processed.\n\n", uploadedExcel.OriginalFilename, sheetIndicatorEN))
	case "csna_ba_lost":
		sbID.WriteString(fmt.Sprintf("📄 Permintaan upload data CSNA BA Lost Anda untuk file *%s*%s telah diproses.\n\n", uploadedExcel.OriginalFilename, sheetIndicatorID))
		sbEN.WriteString(fmt.Sprintf("📄 Your CSNA BA Lost data upload request for file *%s*%s has been processed.\n\n", uploadedExcel.OriginalFilename, sheetIndicatorEN))
	case "technician_payroll":
		sbID.WriteString(fmt.Sprintf("💵 Permintaan slip gaji teknisi Anda untuk file *%s*%s telah diproses.\n\n", uploadedExcel.OriginalFilename, sheetIndicatorID))
		sbEN.WriteString(fmt.Sprintf("💵 Your technician payslip request for file *%s*%s has been processed.\n\n", uploadedExcel.OriginalFilename, sheetIndicatorEN))
	case "update_task":
		sbID.WriteString(fmt.Sprintf("📄 Permintaan update task Anda untuk file *%s*%s telah diproses.\n\n", uploadedExcel.OriginalFilename, sheetIndicatorID))
		sbEN.WriteString(fmt.Sprintf("📄 Your task update request for file *%s*%s has been processed.\n\n", uploadedExcel.OriginalFilename, sheetIndicatorEN))
	default:
		// Generic message for unknown operation types
		sbID.WriteString(fmt.Sprintf("📄 Permintaan Anda untuk file *%s*%s telah diproses.\n\n", uploadedExcel.OriginalFilename, sheetIndicatorID))
		sbEN.WriteString(fmt.Sprintf("📄 Your request for file *%s*%s has been processed.\n\n", uploadedExcel.OriginalFilename, sheetIndicatorEN))
	}

	sbID.WriteString("📈 *Hasil Pemrosesan:*\n")
	sbID.WriteString(fmt.Sprintf("✅ Berhasil: *%d*\n", totalSuccess))
	sbID.WriteString(fmt.Sprintf("❌ Gagal: *%d*\n\n", totalFail))

	sbEN.WriteString("📈 *Processing Results:*\n")
	sbEN.WriteString(fmt.Sprintf("✅ Success: *%d*\n", totalSuccess))
	sbEN.WriteString(fmt.Sprintf("❌ Failed: *%d*\n\n", totalFail))

	if isMultiSheet {
		sbID.WriteString(fmt.Sprintf("%s\n", prettyLog))
		sbEN.WriteString(fmt.Sprintf("%s\n", prettyLog))
	} else {
		sbID.WriteString(fmt.Sprintf("📝 *Detail:*\n%s\n", prettyLog))
		sbEN.WriteString(fmt.Sprintf("📝 *Details:*\n%s\n", prettyLog))
	}

	SendLangMessage(jidString, sbID.String(), sbEN.String(), "id")
}

// ========== UNIVERSAL FUNCTIONS TO ELIMINATE ALL DUPLICATION ==========

// OperationConfig defines configuration for different ODOO operations
type OperationConfig struct {
	TemplateID             int
	TriggerChannel         chan struct{}
	BroadcastProgressFunc  func(odooms.UploadedExcelToODOOMS, string, int, string)
	OperationType          string   // "update", "create", "archive", "delete"
	RequiredFieldInColumns []string // Required fields in order: ["subject", "company", etc] - used for single sheet
	MinValidColumns        int      // Minimum valid columns required
	MultiSheet             bool     // true if operation processes multiple sheets
	Sheets                 []SheetProcessingConfig
}

// SheetProcessingConfig defines configuration for processing a specific sheet in multi-sheet operations
type SheetProcessingConfig struct {
	SheetName              string // Name of the sheet (can use placeholders like {MONTH_EN}, {MONTH_ID})
	AlternateSheetName     string // Fallback sheet name if primary not found
	Handler                func([][]string, *atomic.Int64, *atomic.Int64, odooms.UploadedExcelToODOOMS, []*http.Cookie, *gorm.DB) string
	RequiredFieldInColumns []string // Required fields for this specific sheet
	MinValidColumns        int      // Minimum valid columns for this sheet
	HeaderRowIndex         int      // Which row contains headers (0-based index, default: 0)
	SkipIfNotFound         bool     // true to skip if sheet not found instead of failing
}

// sendODOORequest is UNIVERSAL function that replaces both updateDataExceltoTicketODOOMS and createDataExceltoNewTicketODOOMS
func sendODOORequest(cookieODOO []*http.Cookie, req string, operationType string) (string, error) {
	odooConfig := config.WebPanel.Get().ApiODOO

	// Select URL based on operation type
	var url string
	switch strings.ToLower(operationType) {
	case "update":
		url = odooConfig.UrlUpdateData
	case "create":
		url = odooConfig.UrlCreateData
	default:
		return "", fmt.Errorf("unsupported operation type: %s", operationType)
	}

	maxRetries := odooConfig.MaxRetry
	if maxRetries <= 0 {
		logrus.Warnf("Invalid max retries '%d', using default value: %d", maxRetries, 5)
		maxRetries = 5
	}
	retryDelay := odooConfig.RetryDelay
	if retryDelay <= 0 {
		logrus.Warnf("Invalid retry delay '%d', using default value: %d seconds", retryDelay, 10)
		retryDelay = 10
	}

	var response *http.Response
	var err error
	var bodyBytes []byte

	client := getHTTPClient()

	for attempts := 1; attempts <= maxRetries; attempts++ {
		request, err := http.NewRequest("POST", url, strings.NewReader(req))
		if err != nil {
			logrus.Errorf("Failed to create request: %v", err)
			return "", err
		}

		request.Header.Set("Content-Type", "application/json")
		request.Header.Set("Connection", "keep-alive")

		for _, cookie := range cookieODOO {
			request.AddCookie(cookie)
		}

		response, err = client.Do(request)
		if err != nil {
			logrus.Errorf("POST request failed (Attempt %d/%d): %v", attempts, maxRetries, err)
			if attempts < maxRetries {
				time.Sleep(time.Duration(retryDelay) * time.Second)
				continue
			}
			return "", err
		}

		isResponseOK := false

		body, err := io.ReadAll(response.Body)
		if err != nil {
			logrus.Errorf("reading response body: %v", err)
		}

		var tryJSONResponse map[string]interface{}
		err = json.Unmarshal(body, &tryJSONResponse)
		if err != nil {
			logrus.Errorf("unmarshalling response body: %v", err)
		}

		needToRetry := false
		if errorResponse, ok := tryJSONResponse["error"].(map[string]interface{}); ok {
			// Extract error message from various possible locations
			var errMsg string
			if msg, ok := errorResponse["message"].(string); ok {
				errMsg = msg
			}
			if data, ok := errorResponse["data"].(map[string]interface{}); ok {
				if args, ok := data["arguments"].([]interface{}); ok && len(args) > 0 {
					if arg, ok := args[0].(string); ok {
						errMsg = arg
					}
				}
			}
			lowerErrMsg := strings.ToLower(errMsg)
			markAsError := []string{
				"odoo session expired",
				"transaction is aborted",
			}
			for _, errorToSearch := range markAsError {
				if strings.Contains(lowerErrMsg, errorToSearch) {
					needToRetry = true
					break
				}
			}
		}

		if needToRetry {
			if attempts < maxRetries {
				response.Body.Close()
				time.Sleep(time.Duration(retryDelay) * time.Second)
				continue
			}
		}

		// Try check the status code if all retry attempts failed
		if response.StatusCode == http.StatusOK {
			isResponseOK = true
			bodyBytes = body // Store the body for later use
		}

		if isResponseOK {
			break
		} else {
			response.Body.Close()
			return "", fmt.Errorf("request failed with status code: %v", response.StatusCode)
		}
	}

	defer response.Body.Close()

	if response.Body == nil {
		return "", errors.New("empty response body")
	}

	var jsonResponse map[string]interface{}
	err = json.Unmarshal(bodyBytes, &jsonResponse)
	if err != nil {
		return "", fmt.Errorf("failed to parse JSON response: %v", err)
	}

	// Check for Odoo session expiration
	if errorResponse, ok := jsonResponse["error"].(map[string]interface{}); ok {
		if errorMessage, ok := errorResponse["message"].(string); ok && errorMessage == "Odoo Session Expired" {
			return "", fmt.Errorf("ODOO Session Expired: %v", errorMessage)
		}
		return "", fmt.Errorf("ODOO Error: %v", errorResponse)
	}

	// Handle success response
	result, exists := jsonResponse["result"]
	if !exists || result == nil {
		return "", fmt.Errorf("missing 'result' in response: %v", jsonResponse)
	}

	resultMap, ok := result.(map[string]interface{})
	if !ok {
		return "", fmt.Errorf("invalid 'result' format: %v", result)
	}

	// Check for expected response fields
	status, statusOk := resultMap["status"].(float64)
	success, successOk := resultMap["success"].(bool)
	message, messageOk := resultMap["message"].(string)

	if !statusOk || !messageOk {
		return "", fmt.Errorf("unexpected response format: %v", resultMap)
	}

	// Validate response values
	if int(status) == 200 && successOk && success && message == "Success" {
		infoMsg := fmt.Sprintf("data %s in ODOO", operationType)
		logrus.Debug(infoMsg)
		return infoMsg, nil
	} else {
		errorMsg := fmt.Sprintf("ODOO %s failed: status=%v, success=%v, message=%v", operationType, status, success, message)
		logrus.Error(errorMsg)
		return "", errors.New(errorMsg)
	}
}

// processExcelFileUniversal is UNIVERSAL function that replaces both processExcelFileforUpdateTicketinODOOMS and processExcelFileforCreateNewTicketinODOOMS
func processExcelFileUniversal(
	excelPath string,
	uploadedExcel odooms.UploadedExcelToODOOMS,
	loginCookie []*http.Cookie,
	db *gorm.DB,
	config OperationConfig,
) (int, int, string) {
	var totalFail atomic.Int64
	var totalSuccess atomic.Int64

	// Open the Excel file
	file, err := excelize.OpenFile(excelPath)
	if err != nil {
		return 0, 0, fmt.Sprintf("Failed to open file: %s", err.Error())
	}
	defer file.Close()

	// Multi-sheet processing
	if config.MultiSheet && len(config.Sheets) > 0 {
		return processExcelFileMultiSheet(file, uploadedExcel, loginCookie, db, config)
	}

	// Single-sheet processing (original logic)
	var bulanID string
	// Get the previous month by subtracting 1 month from now
	prevMonth := time.Now().AddDate(0, -1, 0)
	bulanEN := strings.ToUpper(prevMonth.Format("January"))
	tgl, err := tanggal.Papar(prevMonth, "Jakarta", tanggal.WIB)
	if err != nil {
		logrus.Errorf("Failed to get previous month date in Jakarta timezone: %v", err)
		return 0, 0, fmt.Sprintf("Failed to get previous month date: %s", err.Error())
	}
	bulanID = tgl.Format("", []tanggal.Format{
		tanggal.NamaBulan,
	})
	bulanID = strings.ToUpper(bulanID)

	var sheetName string
	switch uploadedExcel.Template {
	// CSNA BA Lost
	case 5:
		sheetName = "BA LOST " + bulanEN
		rows, err := file.GetRows(sheetName)
		if err != nil || len(rows) == 0 {
			sheetName = "BA LOST " + bulanID
		}
	default:
		sheetName = file.GetSheetName(0)
	}

	if sheetName == "" {
		return 0, 0, "No sheets found in the file"
	}

	rows, err := file.GetRows(sheetName)
	if err != nil {
		return 0, 0, fmt.Sprintf("Failed to read rows: %s", err.Error())
	}

	db.Model(&uploadedExcel).Update("total_row", len(rows)-1)
	config.BroadcastProgressFunc(uploadedExcel, "Processing", 0, "Data is being processed...")

	// Select appropriate handler
	var handler func([][]string, *atomic.Int64, *atomic.Int64, odooms.UploadedExcelToODOOMS, []*http.Cookie, *gorm.DB) string

	switch uploadedExcel.Template {
	// Create New Ticket
	case 3:
		handler = processCustomTemplateCreateNewTicketInODOOMS
	// Update Existing Ticket
	case 4:
		handler = processCustomTemplateUpdateTicketInODOOMS
	// CSNA BA LOST
	case 5:
		handler = processCSNABALost
	// Update Existing Task
	case 7:
		handler = processCustomTemplateUpdateTaskInODOOMS
	default:
		return 0, 0, fmt.Sprintf("Unsupported template ID: %d", uploadedExcel.Template)
	}

	// Process using the selected handler
	report := handler(rows, &totalSuccess, &totalFail, uploadedExcel, loginCookie, db)
	return int(totalSuccess.Load()), int(totalFail.Load()), report
}

// processExcelFileMultiSheet handles Excel files with multiple sheets
func processExcelFileMultiSheet(
	file *excelize.File,
	uploadedExcel odooms.UploadedExcelToODOOMS,
	loginCookie []*http.Cookie,
	db *gorm.DB,
	config OperationConfig,
) (int, int, string) {
	var totalFail atomic.Int64
	var totalSuccess atomic.Int64
	var allReports []string

	// Debug: Log the uploadedExcel ID to ensure it's set
	logrus.Infof("Processing multi-sheet Excel - ID: %d, Filename: %s", uploadedExcel.ID, uploadedExcel.OriginalFilename)
	if uploadedExcel.ID == 0 {
		logrus.Warnf("uploadedExcel.ID is 0! Cannot update database record")
	}

	// Track per-sheet statistics for detailed reporting
	type SheetStats struct {
		Name    string
		Success int64
		Fail    int64
	}
	sheetStats := make([]SheetStats, 0, len(config.Sheets))

	// Get month placeholders for dynamic sheet names
	prevMonth := time.Now().AddDate(0, -1, 0)
	bulanEN := strings.ToUpper(prevMonth.Format("January"))
	tgl, err := tanggal.Papar(prevMonth, "Jakarta", tanggal.WIB)
	if err != nil {
		logrus.Errorf("Failed to get previous month date in Jakarta timezone: %v", err)
		return 0, 0, fmt.Sprintf("Failed to get previous month date: %s", err.Error())
	}
	bulanID := strings.ToUpper(tgl.Format("", []tanggal.Format{tanggal.NamaBulan}))

	// Calculate total rows across all sheets first
	totalRowsAllSheets := 0
	for _, sheetConfig := range config.Sheets {
		sheetName := resolveSheetName(sheetConfig.SheetName, bulanEN, bulanID)
		sheetName = strings.TrimSpace(sheetName)
		rows, err := file.GetRows(sheetName)
		if err != nil {
			if sheetConfig.AlternateSheetName != "" {
				altSheetName := resolveSheetName(sheetConfig.AlternateSheetName, bulanEN, bulanID)
				rows, err = file.GetRows(altSheetName)
			}
			if err != nil {
				if sheetConfig.SkipIfNotFound {
					logrus.Warnf("Sheet '%s' not found, skipping as configured", sheetName)
					continue
				}
				return 0, 0, fmt.Sprintf("Failed to read sheet '%s': %s", sheetName, err.Error())
			}
		}
		if len(rows) > 1 { // Exclude header row
			totalRowsAllSheets += len(rows) - 1
		}
	}

	db.Model(&uploadedExcel).Update("total_row", totalRowsAllSheets)

	totalSheets := len(config.Sheets)
	config.BroadcastProgressFunc(uploadedExcel, "Processing", 0, fmt.Sprintf("Processing multi-sheet Excel (%d sheets)...", totalSheets))

	// Process each sheet
	for sheetIndex, sheetConfig := range config.Sheets {
		sheetName := resolveSheetName(sheetConfig.SheetName, bulanEN, bulanID)
		logrus.Infof("Processing sheet %d/%d: %s", sheetIndex+1, totalSheets, sheetName)

		rows, err := file.GetRows(sheetName)
		if err != nil {
			if sheetConfig.AlternateSheetName != "" {
				altSheetName := resolveSheetName(sheetConfig.AlternateSheetName, bulanEN, bulanID)
				rows, err = file.GetRows(altSheetName)
				if err == nil {
					sheetName = altSheetName
				}
			}
			if err != nil {
				if sheetConfig.SkipIfNotFound {
					logrus.Warnf("Sheet '%s' not found, skipping", sheetName)
					continue
				}
				errMsg := fmt.Sprintf("Sheet '%s' not found or unreadable: %s", sheetName, err.Error())
				allReports = append(allReports, errMsg)
				totalFail.Add(1)
				continue
			}
		}

		// Adjust rows based on header row index
		if sheetConfig.HeaderRowIndex > 0 && len(rows) > sheetConfig.HeaderRowIndex {
			rows = rows[sheetConfig.HeaderRowIndex:]
		}

		// Create temporary counters for this sheet
		var sheetSuccess atomic.Int64
		var sheetFail atomic.Int64

		// Broadcast start of this sheet processing
		sheetStartProgress := int(((sheetIndex) * 100) / totalSheets)
		config.BroadcastProgressFunc(uploadedExcel, "Processing", sheetStartProgress,
			fmt.Sprintf("Processing sheet %d/%d: %s", sheetIndex+1, totalSheets, sheetName))

		// Process the sheet using its handler
		// NOTE: Handler will broadcast its own "Completed" status, which we'll override later
		report := sheetConfig.Handler(rows, &sheetSuccess, &sheetFail, uploadedExcel, loginCookie, db)

		// Accumulate totals
		totalSuccess.Add(sheetSuccess.Load())
		totalFail.Add(sheetFail.Load())

		// Store sheet statistics for notification
		sheetStats = append(sheetStats, SheetStats{
			Name:    sheetName,
			Success: sheetSuccess.Load(),
			Fail:    sheetFail.Load(),
		})

		allReports = append(allReports, fmt.Sprintf("Sheet '%s': %s", sheetName, report))

		// Update progress after completing each sheet - based on sheet count
		completedSheets := sheetIndex + 1
		progress := int((completedSheets * 100) / totalSheets)
		if progress > 100 {
			progress = 100
		}
		// Don't send 100% progress until all sheets are done
		if progress >= 100 && completedSheets < totalSheets {
			progress = 99
		}

		progressMsg := fmt.Sprintf("Sheet %d/%d complete (%s): %d success, %d fail | Total: %d success, %d fail",
			completedSheets, totalSheets, sheetName, sheetSuccess.Load(), sheetFail.Load(),
			totalSuccess.Load(), totalFail.Load())
		config.BroadcastProgressFunc(uploadedExcel, "Processing", progress, progressMsg)
	}

	// Prepare final report combining all sheets
	finalReport := strings.Join(allReports, "\n\n")

	// Build combined log for notification with detailed per-sheet statistics
	combinedLogAct := make(map[int]string)

	// Overall summary at key 0
	combinedLogAct[0] = fmt.Sprintf("Processed %d sheets | Total Success: %d | Total Failed: %d",
		len(config.Sheets), totalSuccess.Load(), totalFail.Load())

	// Add detailed per-sheet results (starting from key 1)
	for i, stats := range sheetStats {
		combinedLogAct[i+1] = fmt.Sprintf("'%s' → ✅ %d success, ❌ %d failed",
			stats.Name, stats.Success, stats.Fail)
	}

	// Save log to file if it's too large, otherwise store directly
	logToStore := saveLogToFile(finalReport, uploadedExcel)

	// Update database to Completed status FIRST
	updateResult := db.Model(&odooms.UploadedExcelToODOOMS{}).
		Where("id = ?", uploadedExcel.ID).
		Updates(map[string]interface{}{
			"total_success": totalSuccess.Load(),
			"total_fail":    totalFail.Load(),
			"status":        "Completed",
			"complete_time": time.Now(),
			"log":           logToStore,
		})

	if updateResult.Error != nil {
		logrus.Errorf("Failed to update uploadedExcel record ID %d: %v", uploadedExcel.ID, updateResult.Error)
	} else if updateResult.RowsAffected == 0 {
		logrus.Warnf("No rows updated for uploadedExcel ID %d - record may not exist", uploadedExcel.ID)
	}

	// Send WhatsApp notification after database update
	sendNotificationMessage(uploadedExcel, totalSuccess.Load(), totalFail.Load(), combinedLogAct, db, config.OperationType, true)

	// Trigger operation-specific post-processing actions
	switch strings.ToLower(config.OperationType) {
	case "technician_payroll":
		TryGeneratePDFPayslipTechnicianEDC()
		TryGeneratePDFPayslipTechnicianATM()
	}

	// NOTE: Return values are still used by worker for final broadcast
	return int(totalSuccess.Load()), int(totalFail.Load()), finalReport
}

// resolveSheetName replaces placeholders in sheet names with actual values
func resolveSheetName(sheetName, monthEN, monthID string) string {
	sheetName = strings.ReplaceAll(sheetName, "{MONTH_EN}", monthEN)
	sheetName = strings.ReplaceAll(sheetName, "{MONTH_ID}", monthID)
	return sheetName
}

// processUploadedExcelWorker is UNIVERSAL function that replaces both ProcessUploadedExcelofODOOMSMustUpdatedTicket and ProcessUploadedExcelofODOOMSNewTicketCreated
func processUploadedExcelWorker(db *gorm.DB, opConfig OperationConfig) {
	// Start multiple worker goroutines for concurrent processing
	maxWorkers := runtime.NumCPU() // Use all available CPU cores

	for i := 0; i < maxWorkers; i++ {
		go func(workerID int) {
			defer func() {
				if r := recover(); r != nil {
					// Get stack trace
					stackTrace := debug.Stack()
					logrus.Errorf("🚨 Worker %d PANIC recovered!\nPanic: %v\n\nStack Trace:\n%s", workerID, r, string(stackTrace))

					// Try to save panic info to a file for debugging
					logDir, err := fun.FindValidDirectory([]string{
						"log",
						"../log",
						"../../log",
						"../../../log",
					})
					if err != nil {
						logDir = "." // Fallback to current directory
					}
					panicFile := filepath.Join(logDir, fmt.Sprintf("panic_worker_%d_%s.log", workerID, time.Now().Format("20060102_150405")))
					if err := os.WriteFile(panicFile, []byte(fmt.Sprintf("Worker %d Panic: %v\n\nStack Trace:\n%s", workerID, r, string(stackTrace))), 0644); err == nil {
						logrus.Infof("Panic details saved to %s", panicFile)
					}
				}
			}()

			logrus.Infof("💻 Worker %d started for template %d (%s)", workerID, opConfig.TemplateID, opConfig.OperationType)

			for {
				// Wait for trigger
				<-opConfig.TriggerChannel
				logrus.Infof("Worker %d triggered! Looking for pending files...", workerID)

				// Process pending files
				for {
					var uploadedExcel odooms.UploadedExcelToODOOMS

					// Use transaction to avoid race conditions
					err := db.Transaction(func(tx *gorm.DB) error {
						if err := tx.Set("gorm:query_option", "FOR UPDATE SKIP LOCKED").
							Where("status = ?", "Pending").
							Where("template = ?", opConfig.TemplateID).
							First(&uploadedExcel).Error; err != nil {
							if errors.Is(err, gorm.ErrRecordNotFound) {
								// No pending file found, just chill (do nothing)
								return err
							}
							// Return error if it's not a not-found error
							return err
						}

						// Mark as processing
						return tx.Model(&uploadedExcel).Update("status", "Processing").Error
					})

					if err != nil {
						if errors.Is(err, gorm.ErrRecordNotFound) {
							logrus.Infof("Worker %d: No more pending files", workerID)
						} else {
							logrus.Errorf("Worker %d database error: %v", workerID, err)
						}
						break // Exit the inner loop
					}

					logrus.Infof("Worker %d processing: %s (ID: %d)", workerID, uploadedExcel.OriginalFilename, uploadedExcel.ID)

					// Send initial processing status
					opConfig.BroadcastProgressFunc(uploadedExcel, "Processing", 0, "Starting processing...")

					// Decrypt password
					decPwd, err := fun.GetAESDecrypted(uploadedExcel.Password)
					if err != nil {
						logrus.Errorf("Worker %d decryption failed: %v", workerID, err)
						db.Model(&uploadedExcel).Updates(map[string]interface{}{
							"status": "Failed",
							"log":    fmt.Sprintf("Decryption failed: %v", err),
						})
						opConfig.BroadcastProgressFunc(uploadedExcel, "Failed", 100, fmt.Sprintf("Decryption failed: %v", err))
						continue
					}

					// Get session cookies for Odoo login
					var emailUploadedBy, decryptedPwd string
					if config.WebPanel.Get().UploadedExcelForODOOMS.ActiveDebug {
						emailUploadedBy = config.WebPanel.Get().UploadedExcelForODOOMS.EmailParam
						decryptedPwd = config.WebPanel.Get().UploadedExcelForODOOMS.PwdParam
					} else {
						emailUploadedBy = uploadedExcel.Email
						decryptedPwd = string(decPwd)
					}

					loginCookie, err := GetODOOMSCookies(emailUploadedBy, decryptedPwd)
					if err != nil {
						logrus.Errorf("Worker %d Odoo login failed for email %v: %v", workerID, uploadedExcel.Email, err)
						db.Model(&uploadedExcel).Updates(map[string]interface{}{
							"status": "Failed",
							"log":    fmt.Sprintf("ODOO login failed: %v", err),
						})
						opConfig.BroadcastProgressFunc(uploadedExcel, "Failed", 100, fmt.Sprintf("ODOO login failed: %v", err))
						continue
					}

					// Process the Excel file
					selectedMainDir, err := fun.FindValidDirectory([]string{
						"web/file/uploaded_excel_to_odoo_ms",
						"../web/file/uploaded_excel_to_odoo_ms",
						"../../web/file/uploaded_excel_to_odoo_ms",
					})
					if err != nil {
						logrus.Errorf("Worker %d: No valid directory found: %v", workerID, err)
						db.Model(&uploadedExcel).Updates(map[string]interface{}{
							"status": "Failed",
							"log":    fmt.Sprintf("Directory not found: %v", err),
						})
						opConfig.BroadcastProgressFunc(uploadedExcel, "Failed", 100, fmt.Sprintf("Directory not found: %v", err))
						continue
					}

					excelPath := fmt.Sprintf("%s/%s", selectedMainDir, uploadedExcel.Filename)
					totalSuccess, totalFail, report := processExcelFileUniversal(excelPath, uploadedExcel, loginCookie, db, opConfig)

					if report != "" && (totalSuccess == 0 && totalFail == 0) {
						// This indicates an error occurred before processing
						logrus.Errorf("Worker %d Excel %v processing error: %v", workerID, excelPath, report)
						db.Model(&uploadedExcel).Updates(map[string]interface{}{
							"log":    report,
							"status": "Failed",
						})
						opConfig.BroadcastProgressFunc(uploadedExcel, "Failed", 100, report)
						continue
					}

					// For multi-sheet operations, database update and notifications are already done
					// For single-sheet operations, update database and send final broadcast here
					if opConfig.MultiSheet {
						// Multi-sheet: only send final broadcast (DB already updated in processExcelFileMultiSheet)
						opConfig.BroadcastProgressFunc(uploadedExcel, "Completed", 100, report)
						logrus.Infof("Worker %d completed multi-sheet processing: %s", workerID, uploadedExcel.OriginalFilename)
					} else {
						// Single-sheet: update database and send final broadcast
						db.Model(&uploadedExcel).Updates(map[string]interface{}{
							"total_success": totalSuccess,
							"total_fail":    totalFail,
							"status":        "Completed",
							"complete_time": time.Now(),
							"log":           report,
						})
						opConfig.BroadcastProgressFunc(uploadedExcel, "Completed", 100, report)
						logrus.Infof("Worker %d completed single-sheet processing: %s", workerID, uploadedExcel.OriginalFilename)
					}
				}

				logrus.Infof("Worker %d waiting for next trigger...", workerID)
			}
		}(i)
	}
}

// validateCompanyExists checks if a company exists in the database by name or ID
func validateCompanyExists(db *gorm.DB, companyValue string) (bool, *odooms.ODOOMSCompany, error) {
	if companyValue == "" {
		return false, nil, fmt.Errorf("company value is empty")
	}

	var company odooms.ODOOMSCompany
	// Check by name (case-insensitive) or ID
	err := db.Where("LOWER(name) = LOWER(?) OR id = ?", companyValue, companyValue).First(&company).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return false, nil, fmt.Errorf("company '%s' not found", companyValue)
		}
		return false, nil, fmt.Errorf("database error: %v", err)
	}

	return true, &company, nil
}

// validateTicketTypeExists checks if a ticket type exists in the database by name or ID
func validateTicketTypeExists(db *gorm.DB, ticketTypeValue string) (bool, *odooms.ODOOMSTicketType, error) {
	if ticketTypeValue == "" {
		return false, nil, fmt.Errorf("ticket type value is empty")
	}

	var ticketType odooms.ODOOMSTicketType
	// Check by type (case-insensitive) or ID
	err := db.Where("LOWER(type) = LOWER(?) OR id = ?", ticketTypeValue, ticketTypeValue).First(&ticketType).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return false, nil, fmt.Errorf("ticket type '%s' not found", ticketTypeValue)
		}
		return false, nil, fmt.Errorf("database error: %v", err)
	}

	return true, &ticketType, nil
}

func validateMIDTIDExists(midtid string) (bool, *uint, error) {
	if midtid == "" {
		return false, nil, fmt.Errorf("MIDTID value is empty")
	}

	ODOOModel := "res.partner"
	fields := []string{"id"}
	domain := []interface{}{
		[]interface{}{"active", "=", true},
		[]interface{}{"name", "=", midtid},
	}
	order := "id asc"

	odooParams := map[string]interface{}{
		"model":  ODOOModel,
		"domain": domain,
		"fields": fields,
		"order":  order,
	}

	payload := map[string]interface{}{
		"jsonrpc": config.WebPanel.Get().ApiODOO.JSONRPC,
		"params":  odooParams,
	}

	payloadBytes, err := json.Marshal(payload)
	if err != nil {
		return false, nil, fmt.Errorf("failed to marshal payload: %v", err)
	}

	ODOOresponse, err := GetODOOMSData(string(payloadBytes))
	if err != nil {
		return false, nil, fmt.Errorf("failed fetching data from ODOO MS API: %v", err)
	}

	ODOOResponseArray, ok := ODOOresponse.([]interface{})
	if !ok {
		return false, nil, fmt.Errorf("failed to assert results as []interface{}")
	}

	if len(ODOOResponseArray) == 0 {
		return false, nil, fmt.Errorf("MIDTID '%s' not found in ODOO", midtid)
	}

	ODOOResponseBytes, err := json.Marshal(ODOOResponseArray)
	if err != nil {
		return false, nil, fmt.Errorf("failed to marshal response: %v", err)
	}

	var resPartner []OdooResPartnerItem
	err = json.Unmarshal(ODOOResponseBytes, &resPartner)
	if err != nil {
		return false, nil, fmt.Errorf("failed to unmarshal ODOO response: %v", err)
	}

	if len(resPartner) == 0 {
		return false, nil, fmt.Errorf("MIDTID '%s' not found in ODOO", midtid)
	}

	return true, &resPartner[0].ID, nil
}

// parseExcelBoolean converts various Excel boolean representations to Go bool
// Supports: true/false, yes/no, y/n, 1/0, t/f, on/off, checked/unchecked (case-insensitive)
// Also supports checkbox symbols: ☑, ✓, ✔, ☐, ✗, ✘
func parseExcelBoolean(value string) (bool, error) {
	if value == "" {
		return false, fmt.Errorf("empty value")
	}

	value = strings.TrimSpace(value)
	lowerValue := strings.ToLower(value)

	// Handle common true values
	trueValues := []string{
		"true", "yes", "y", "1", "t", "on", "checked", "check",
		"✓", "✔", "☑", "x", // x often used for checkboxes
	}
	for _, tv := range trueValues {
		if lowerValue == tv {
			return true, nil
		}
	}

	// Handle common false values
	falseValues := []string{
		"false", "no", "n", "0", "f", "off", "unchecked", "uncheck",
		"☐", "✗", "✘", "-",
	}
	for _, fv := range falseValues {
		if lowerValue == fv {
			return false, nil
		}
	}

	// If not recognized, return error
	return false, fmt.Errorf("unrecognized boolean value: %s", value)
}

func BackupTableBALostPrevMonth() error {
	dbWeb := gormdb.Databases.Web
	table := config.WebPanel.Get().Database.TbBALost

	now := time.Now()
	prevMonth := now.AddDate(0, -1, 0)
	yearStr := prevMonth.Format("2006")
	monthStr := prevMonth.Format("Jan")
	backupTable := strings.ToLower(fmt.Sprintf("%s_%s%s", table, monthStr, yearStr))

	// Check if source table exists
	var sourceExists bool
	sourceCheckQuery := fmt.Sprintf("SELECT COUNT(*) > 0 FROM information_schema.tables WHERE table_schema = DATABASE() AND table_name = '%s'", table)
	err := dbWeb.Raw(sourceCheckQuery).Scan(&sourceExists).Error
	if err != nil {
		return fmt.Errorf("failed to check if source table exists: %v", err)
	}

	if !sourceExists {
		logrus.Warnf("Source table %s does not exist, skipping backup", table)
		return nil
	}

	// Check if source table has data
	var count int64
	err = dbWeb.Table(table).Count(&count).Error
	if err != nil {
		return fmt.Errorf("failed to count records in source table: %v", err)
	}

	if count == 0 {
		logrus.Warnf("Source table %s has no data, skipping backup", table)
		return nil
	}

	// Check if backup table already exists
	var exists bool
	checkQuery := fmt.Sprintf("SELECT COUNT(*) > 0 FROM information_schema.tables WHERE table_schema = DATABASE() AND table_name = '%s'", backupTable)
	err = dbWeb.Raw(checkQuery).Scan(&exists).Error
	if err != nil {
		return fmt.Errorf("failed to check if table exists: %v", err)
	}

	if exists {
		logrus.Warnf("Backup table %s already exists, skipping creation", backupTable)
		return nil
	}

	// Create backup table by copying structure and data
	createQuery := fmt.Sprintf("CREATE TABLE %s AS SELECT * FROM %s", backupTable, table)
	err = dbWeb.Exec(createQuery).Error
	if err != nil {
		return fmt.Errorf("failed to create backup table: %v", err)
	}

	// ADD: delete the data from source table if needed, coz if its exists it will cause duplicate when count for cost revenue

	logrus.Infof("Backup table %s created successfully with %d records", backupTable, count)
	return nil
}

// buildFieldMapFromStruct creates a lookup map from Excel header names to struct JSON field names
// This is a UNIVERSAL function that works with any struct by using reflection
// Usage: buildFieldMapFromStruct(odooms.CSNABALost{})
// Usage: buildFieldMapFromStruct(odooms.MSTechnicianPayroll{})
func buildFieldMapFromStruct(structInstance interface{}) map[string]string {
	fieldMap := make(map[string]string)

	v := reflect.ValueOf(structInstance)
	t := reflect.TypeOf(structInstance)

	// Handle pointer types
	if v.Kind() == reflect.Ptr {
		v = v.Elem()
		t = t.Elem()
	}

	// Ensure it's a struct
	if v.Kind() != reflect.Struct {
		logrus.Warnf("buildFieldMapFromStruct: expected struct, got %v", v.Kind())
		return fieldMap
	}

	for i := 0; i < v.NumField(); i++ {
		field := t.Field(i)
		jsonTag := field.Tag.Get("json")
		if jsonTag != "" && jsonTag != "-" {
			// Remove options from json tag (e.g., "name,omitempty" -> "name")
			jsonTag = strings.Split(jsonTag, ",")[0]

			// Replace underscores with spaces for matching
			displayName := strings.ReplaceAll(jsonTag, "_", " ")
			fieldMap[strings.ToLower(displayName)] = jsonTag
		}
	}

	return fieldMap
}

// saveLogToFile saves large log content to a file and returns the file path
// If the log is small enough, returns the log content directly
func saveLogToFile(logContent string, uploadedExcel odooms.UploadedExcelToODOOMS) string {
	const maxLogSize = 60000 // ~60KB limit for TEXT column (keeping some buffer)

	// If log is small enough, return it directly
	if len(logContent) <= maxLogSize {
		return logContent
	}

	// Create log file directory
	logDir, err := fun.FindValidDirectory([]string{
		"web/file/uploaded_excel_to_odoo_ms",
		"../web/file/uploaded_excel_to_odoo_ms",
		"../../web/file/uploaded_excel_to_odoo_ms",
	})
	if err != nil {
		// Try to create the directory
		possibleDirs := []string{
			"web/file/uploaded_excel_to_odoo_ms",
			"../web/file/uploaded_excel_to_odoo_ms",
			"../../web/file/uploaded_excel_to_odoo_ms",
		}
		created := false
		for _, dir := range possibleDirs {
			if err := os.MkdirAll(dir, 0755); err == nil {
				logDir = dir
				created = true
				break
			}
		}
		if !created {
			logrus.Errorf("Failed to create log directory: %v", err)
			// Return truncated log as fallback
			return logContent[:maxLogSize] + "\n... (log truncated, could not create file)"
		}
	}

	// Generate unique log filename
	timestamp := time.Now().Format("20060102_150405")
	logFilename := fmt.Sprintf("log_%d_%s.txt", uploadedExcel.ID, timestamp)
	logFilePath := fmt.Sprintf("%s/%s", logDir, logFilename)

	// Write log to file
	err = os.WriteFile(logFilePath, []byte(logContent), 0644)
	if err != nil {
		logrus.Errorf("Failed to write log to file %s: %v", logFilePath, err)
		// Return truncated log as fallback
		return logContent[:maxLogSize] + "\n... (log truncated, file write failed)"
	}

	logrus.Infof("Large log saved to file: %s (%d bytes)", logFilePath, len(logContent))

	// Return file path reference
	return fmt.Sprintf("LOG_FILE: %s", logFilename)
}

func getIDValueOfMany2OneField(model, fieldName string, fieldValue any) (uint, error) {
	if model == "" || fieldName == "" || fieldValue == "" {
		return 0, fmt.Errorf("model, fieldName, or fieldValue is empty")
	}

	dbWeb := gormdb.Databases.Web

	field := []string{"id"}
	domain := []interface{}{
		[]interface{}{"active", "=", true},
		[]interface{}{fieldName, "=", fieldValue},
	}

	switch strings.ToLower(model) {
	case "helpdesk.team":
		var OdooCompany odooms.ODOOMSCompany
		err := dbWeb.Where("LOWER(name) = LOWER(?)", fieldValue).First(&OdooCompany).Error
		if err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				logrus.Warnf("Company '%v' not found in local DB for helpdesk.team", fieldValue)
			}
			logrus.Errorf("Database error while looking up company '%v': %v", fieldValue, err)
		} else {
			domain = append(domain, []interface{}{"company_id", "=", OdooCompany.ID})
		}
	}

	order := "id desc"

	odooParams := map[string]interface{}{
		"model":  model,
		"domain": domain,
		"fields": field,
		"order":  order,
	}

	payload := map[string]interface{}{
		"jsonrpc": config.WebPanel.Get().ApiODOO.JSONRPC,
		"params":  odooParams,
	}

	payloadBytes, err := json.Marshal(payload)
	if err != nil {
		return 0, fmt.Errorf("failed to marshal payload: %v", err)
	}

	ODOOresponse, err := GetODOOMSData(string(payloadBytes))
	if err != nil {
		return 0, fmt.Errorf("failed fetching data from ODOO MS API: %v", err)
	}

	ODOOResponseArray, ok := ODOOresponse.([]interface{})
	if !ok {
		return 0, fmt.Errorf("failed to assert results as []interface{}")
	}

	if len(ODOOResponseArray) == 0 {
		// Try again with active != true
		domain = domain[1:2]

		odooParams["domain"] = domain

		payload["params"] = odooParams
		payloadBytes, err = json.Marshal(payload)
		if err != nil {
			return 0, fmt.Errorf("failed to marshal payload for retry: %v", err)
		}

		ODOOresponse, err = GetODOOMSData(string(payloadBytes))
		if err != nil {
			return 0, fmt.Errorf("failed fetching data from ODOO MS API on retry: %v", err)
		}

		ODOOResponseArray, ok = ODOOresponse.([]interface{})
		if !ok {
			return 0, fmt.Errorf("failed to assert results as []interface{} on retry")
		}

		if len(ODOOResponseArray) == 0 {
			return 0, fmt.Errorf("value '%v' for field '%s' not found in ODOO model '%s' even with active=true", fieldValue, fieldName, model)
		}
	}

	ODOOResponseBytes, err := json.Marshal(ODOOResponseArray)
	if err != nil {
		return 0, fmt.Errorf("failed to marshal response: %v", err)
	}

	var idUint uint = 0
	var errorGot error = fmt.Errorf("unknown error or maybe the model is not supported")

	switch strings.ToLower(model) {
	case "res.partner":
		var resPartner []OdooResPartnerItem
		err = json.Unmarshal(ODOOResponseBytes, &resPartner)
		if err != nil {
			return 0, fmt.Errorf("failed to unmarshal ODOO response: %v", err)
		}

		if len(resPartner) == 0 {
			return 0, fmt.Errorf("value '%v' for field '%s' not found in ODOO", fieldValue, fieldName)
		}

		idUint = resPartner[0].ID
		errorGot = nil
	case "helpdesk.team":
		var helpdeskTeams []OdooHelpdeskTeamItem
		err = json.Unmarshal(ODOOResponseBytes, &helpdeskTeams)
		if err != nil {
			return 0, fmt.Errorf("failed to unmarshal ODOO response: %v", err)
		}

		if len(helpdeskTeams) == 0 {
			return 0, fmt.Errorf("value '%v' for field '%s' not found in ODOO", fieldValue, fieldName)
		}

		idUint = helpdeskTeams[0].ID
		errorGot = nil
	case "res.users":
		var resUsers []OdooResUsersItem
		err = json.Unmarshal(ODOOResponseBytes, &resUsers)
		if err != nil {
			return 0, fmt.Errorf("failed to unmarshal ODOO response: %v", err)
		}

		if len(resUsers) == 0 {
			return 0, fmt.Errorf("value '%v' for field '%s' not found in ODOO", fieldValue, fieldName)
		}

		idUint = resUsers[0].ID
		errorGot = nil
	case "project.project":
		var projectProjects []OdooProjectProjectItem
		err = json.Unmarshal(ODOOResponseBytes, &projectProjects)
		if err != nil {
			return 0, fmt.Errorf("failed to unmarshal ODOO response: %v", err)
		}

		if len(projectProjects) == 0 {
			return 0, fmt.Errorf("value '%v' for field '%s' not found in ODOO", fieldValue, fieldName)
		}

		idUint = projectProjects[0].ID
		errorGot = nil
	case "fs.technician":
		var fsTechnicians []ODOOMSTechnicianItem
		err = json.Unmarshal(ODOOResponseBytes, &fsTechnicians)
		if err != nil {
			return 0, fmt.Errorf("failed to unmarshal ODOO response: %v", err)
		}

		if len(fsTechnicians) == 0 {
			return 0, fmt.Errorf("value '%v' for field '%s' not found in ODOO", fieldValue, fieldName)
		}

		idUint = fsTechnicians[0].ID
		errorGot = nil
	}

	return idUint, errorGot
}
