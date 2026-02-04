package controllers

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"service-platform/cmd/web_panel/fun"
	"service-platform/cmd/web_panel/internal/gormdb"
	"service-platform/cmd/web_panel/model"
	contracttechnicianmodel "service-platform/cmd/web_panel/model/contract_technician_model"
	odooms "service-platform/cmd/web_panel/model/odoo_ms"
	"service-platform/internal/config"
	"sort"
	"strings"
	"sync"
	"time"

	"codeberg.org/go-pdf/fpdf"
	"github.com/TigorLazuardi/tanggal"
	"github.com/sirupsen/logrus"
	"gorm.io/datatypes"
	"gorm.io/gorm"
)

var (
	TechODOOMSDataForContract                          = make(map[string]TechnicianODOOData) // Key: Technician Name
	contractTechnicianODOOMutex                        sync.Mutex
	checkAvailableForSendContractToTechnicianODOOMutex sync.Mutex

	getDataFSTechnicianInODOOForKontrakTeknisiMutex sync.Mutex
)

// TechnicianAggregateDataForContract holds aggregated data for a technician
type TechnicianAggregateDataForContract struct {
	TechnicianID               int
	TechnicianName             string
	SPL                        string
	SAC                        string
	WONumbers                  []string
	TicketSubjects             []string
	WONumbersAlreadyVisit      []string
	TicketSubjectsAlreadyVisit []string
	FirstUploaded              *time.Time
	LatestVisit                *time.Time
	Email                      string
	NoHP                       string
	Name                       string
	UserCreatedOn              *time.Time
	JobGroupID                 int
	NIK                        string
	Address                    string
	Area                       string
	TTL                        string
	EmployeeCode               string
}

type pdfField struct {
	Label string
	Value string
}

type TextRun struct {
	Text  string
	Style string // "", "B", "I", "BI"
}

type ListItem struct {
	Parts    []TextRun
	Children [][]TextRun
}

// Deprecated: Use GetDataTechnicianForContractInODOO instead
func ContractTechnicianODOO() error {
	mustJoinedDays := config.WebPanel.Get().ContractTechnicianODOO.MustJoinedAfter
	if mustJoinedDays <= 0 {
		return errors.New("MUST_JOINED_AFTER config must be greater than 0")
	}

	taskDoing := fmt.Sprintf("Check which technicians joined %d days ago, if already doing JO & login then send the contract sign to them", mustJoinedDays)

	if !contractTechnicianODOOMutex.TryLock() {
		return fmt.Errorf("another process is still running for task: %s", taskDoing)
	}
	defer contractTechnicianODOOMutex.Unlock()

	forProject := "ODOO MS"

	loc, _ := time.LoadLocation(config.WebPanel.Get().Default.Timezone)
	now := time.Now().In(loc)

	dateJoined := now.AddDate(0, 0, -mustJoinedDays)
	dateJoinedStart := time.Date(dateJoined.Year(), dateJoined.Month(), dateJoined.Day(), 0, 0, 0, 0, loc)
	dateJoinedEnd := time.Date(dateJoined.Year(), dateJoined.Month(), dateJoined.Day(), 23, 59, 59, 0, loc)

	nowEnd := time.Date(now.Year(), now.Month(), now.Day(), 23, 59, 59, 0, loc)

	ODOOModel := "fs.technician"
	domain := []interface{}{
		[]interface{}{"active", "=", true},
		[]interface{}{"create_date", ">=", dateJoinedStart.Format("2006-01-02 15:04:05")},
		[]interface{}{"create_date", "<=", dateJoinedEnd.Format("2006-01-02 15:04:05")},
	}
	fields := []string{
		"id",
		"name",
		"email",
		"x_no_telp",
		"x_technician_name",
		"technician_code",
		"x_spl_leader",
		"login_ids",
		"download_ids",
		"create_date",
		"job_group_id",
		"work_location",
		"nik",
		"address",
		"area",
		"birth_status",
		"x_employee_code",
	}
	order := "name asc"

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
		return fmt.Errorf("failed to marshal ODOO MS request payload: %v", err)
	}
	ODOOresponse, err := GetODOOMSData(string(payloadBytes))
	if err != nil {
		return fmt.Errorf("failed fetching data from ODOO MS API: %v", err)
	}
	ODOOResponseArray, ok := ODOOresponse.([]interface{})
	if !ok {
		return errors.New("failed to assert ODOO MS response as []interface{}")
	}

	ODOOResponseBytes, err := json.Marshal(ODOOResponseArray)
	if err != nil {
		return fmt.Errorf("failed to marshal ODOO response: %v", err)
	}

	var employeeData []ODOOMSTechnicianItem
	if err := json.Unmarshal(ODOOResponseBytes, &employeeData); err != nil {
		return fmt.Errorf("failed to unmarshal ODOO response body: %v", err)
	}

	if len(employeeData) == 0 {
		return fmt.Errorf("no technician joined in (%d) days before or %v", mustJoinedDays, dateJoined.Format("02 Jan 2006"))
	}

	// Collect all login and download IDs for batch processing
	var allLoginIDs []float64
	var allDownloadIDs []float64
	technicianLoginMap := make(map[string]float64)    // technician name -> latest login ID
	technicianDownloadMap := make(map[string]float64) // technician name -> latest download ID

	technicianNotHasValidPhone := make(map[string]TechnicianODOOData)
	technicianResign := make(map[string]TechnicianODOOData)

	// Process each technician to get their latest login/download IDs
	for _, emp := range employeeData {
		technicianName := emp.NameFS.String
		if technicianName == "" {
			continue
		}

		// Find latest login ID
		if len(emp.LoginIDs) > 0 {
			sort.Slice(emp.LoginIDs, func(i, j int) bool {
				if emp.LoginIDs[i].Valid && emp.LoginIDs[j].Valid {
					return emp.LoginIDs[i].Float > emp.LoginIDs[j].Float
				}
				return emp.LoginIDs[i].Valid
			})
			if emp.LoginIDs[0].Valid {
				lastLoginID := emp.LoginIDs[0].Float
				technicianLoginMap[technicianName] = lastLoginID
				allLoginIDs = append(allLoginIDs, lastLoginID)
			}
		}

		// Find latest download ID
		if len(emp.DownloadIDs) > 0 {
			sort.Slice(emp.DownloadIDs, func(i, j int) bool {
				if emp.DownloadIDs[i].Valid && emp.DownloadIDs[j].Valid {
					return emp.DownloadIDs[i].Float > emp.DownloadIDs[j].Float
				}
				return emp.DownloadIDs[i].Valid
			})
			if emp.DownloadIDs[0].Valid {
				lastDownloadID := emp.DownloadIDs[0].Float
				technicianDownloadMap[technicianName] = lastDownloadID
				allDownloadIDs = append(allDownloadIDs, lastDownloadID)
			}
		}

		phoneNumberUsed := emp.NoTelp.String

		var userCreatedOn *time.Time
		if emp.CreatedOn.Valid {
			createdTime, err := time.Parse("2006-01-02 15:04:05", emp.CreatedOn.String)
			if err != nil {
				logrus.Errorf("Failed to parse created date for technician: %v", err)
			} else {
				createdTime = createdTime.Add(7 * time.Hour)
				userCreatedOn = &createdTime
			}
		}

		// If not registered in whatsapp then you will send the list to HRD
		sanitizedPhone, err := fun.SanitizePhoneNumber(phoneNumberUsed)
		if err != nil {
			logrus.Errorf("Failed to sanitize phone number %s of technician %s: %v", phoneNumberUsed, emp.TechnicianName.String, err)
			technicianNotHasValidPhone[technicianName] = TechnicianODOOData{
				SPL:            emp.SPL.String,
				SAC:            emp.Head.String,
				LastLogin:      nil,
				LastDownloadJO: nil,
				Email:          emp.Email.String,
				NoHP:           phoneNumberUsed,
				Name:           emp.TechnicianName.String,
				UserCreatedOn:  userCreatedOn,
				EmployeeCode:   emp.EmployeeCode.String,
			}
		} else {
			phoneNumberUsed = "62" + sanitizedPhone
		}

		if strings.Contains(technicianName, "*") {
			// Resigned technician
			technicianResign[technicianName] = TechnicianODOOData{
				SPL:            emp.SPL.String,
				SAC:            emp.Head.String,
				LastLogin:      nil,
				LastDownloadJO: nil,
				Email:          emp.Email.String,
				NoHP:           phoneNumberUsed,
				Name:           emp.TechnicianName.String,
				UserCreatedOn:  userCreatedOn,
				EmployeeCode:   emp.EmployeeCode.String,
			}
			// Skip adding resigned technicians to the main map
			continue
		}

		jobGroupID, _, err := parseJSONIDDataCombined(emp.JobGroupId)
		if err != nil {
			logrus.Errorf("Failed to parse job group ID for technician %s: %v", technicianName, err)
		}

		// Initialize technician data with basic info
		TechODOOMSDataForContract[technicianName] = TechnicianODOOData{
			SPL:            emp.SPL.String,
			SAC:            emp.Head.String,
			LastLogin:      nil,
			LastDownloadJO: nil,
			Email:          emp.Email.String,
			NoHP:           phoneNumberUsed,
			Name:           emp.TechnicianName.String,
			UserCreatedOn:  userCreatedOn,
			JobGroupID:     jobGroupID,
			NIK:            emp.NIK.String,
			Address:        emp.Alamat.String,
			Area:           emp.Area.String,
			TTL:            emp.TempatTanggalLahir.String,
			EmployeeCode:   emp.EmployeeCode.String,
		}
	}

	// Batch get all login and download times
	loginTimes, downloadTimes, err := getBatchLoginAndDownloadTimes(allLoginIDs, allDownloadIDs)
	if err != nil {
		logrus.Errorf("Failed to get batch login/download times: %v", err)
		// Continue without login/download times
	}

	// Update technician data with login/download times
	for technicianName, data := range TechODOOMSDataForContract {
		// Update login time
		if loginID, exists := technicianLoginMap[technicianName]; exists {
			if loginTime, found := loginTimes[loginID]; found {
				data.LastLogin = loginTime
			}
		}

		// Update download time
		if downloadID, exists := technicianDownloadMap[technicianName]; exists {
			if downloadTime, found := downloadTimes[downloadID]; found {
				data.LastDownloadJO = downloadTime
			}
		}

		// Update the map with new data
		TechODOOMSDataForContract[technicianName] = data

		// Update the map of Resign Technician & Not Valid Phone Number
		if len(technicianResign) > 0 {
			technicianResign[technicianName] = data
		}
		if len(technicianNotHasValidPhone) > 0 {
			technicianNotHasValidPhone[technicianName] = data
		}
	}

	// Send message to HRD if there are technicians without valid phone numbers
	if len(technicianNotHasValidPhone) > 0 {
		var sbID strings.Builder
		var sbEN strings.Builder
		Number := 1

		sbID.WriteString(fmt.Sprintf("Berikut adalah daftar teknisi yang bergabung pada %d hari lalu (%s) namun tidak memiliki nomor HP yang valid atau belum terdaftar di WhatsApp:\n\n", mustJoinedDays, dateJoined.Format("02 Jan 2006")))
		for _, tech := range technicianNotHasValidPhone {
			sbID.WriteString(fmt.Sprintf("%d. %s\n", Number, tech.Name))
			sbID.WriteString(fmt.Sprintf("    SPL: %s\n", tech.SPL))
			sbID.WriteString(fmt.Sprintf("    SAC: %s\n", tech.SAC))
			sbID.WriteString(fmt.Sprintf("    Email: %s\n", tech.Email))
			sbID.WriteString(fmt.Sprintf("    No HP: %s\n", tech.NoHP))
			if tech.UserCreatedOn != nil {
				sbID.WriteString(fmt.Sprintf("    Bergabung pada: %s\n", tech.UserCreatedOn.Format("02 Jan 2006")))
			}
			sbID.WriteString("\n")
			Number++
		}

		Number = 1
		sbEN.WriteString(fmt.Sprintf("The following is a list of technicians who joined %d days ago (%s) but do not have a valid phone number or are not registered on WhatsApp:\n\n", mustJoinedDays, dateJoined.Format("02 Jan 2006")))
		for _, tech := range technicianNotHasValidPhone {
			sbEN.WriteString(fmt.Sprintf("%d. %s\n", Number, tech.Name))
			sbEN.WriteString(fmt.Sprintf("    SPL: %s\n", tech.SPL))
			sbEN.WriteString(fmt.Sprintf("    SAC: %s\n", tech.SAC))
			sbEN.WriteString(fmt.Sprintf("    Email: %s\n", tech.Email))
			sbEN.WriteString(fmt.Sprintf("    Phone No: %s\n", tech.NoHP))
			if tech.UserCreatedOn != nil {
				sbEN.WriteString(fmt.Sprintf("    Joined on: %s\n", tech.UserCreatedOn.Format("02 Jan 2006")))
			}
			sbEN.WriteString("\n")
			Number++
		}

		jidStrHRD := fmt.Sprintf("%s@%s", config.WebPanel.Get().Default.PTHRD[0].PhoneNumber, "s.whatsapp.net")
		originalSenderJID := NormalizeSenderJID(jidStrHRD)
		SendLangMessage(originalSenderJID, sbID.String(), sbEN.String(), "id")
	}

	// Send message to HRD if there are resigned technicians
	if len(technicianResign) > 0 {
		var sbID strings.Builder
		var sbEN strings.Builder
		Number := 1

		sbID.WriteString(fmt.Sprintf("Berikut adalah daftar teknisi yang mengundurkan diri pada %d hari lalu (%s):\n\n", mustJoinedDays, dateJoined.Format("02 Jan 2006")))
		for _, tech := range technicianResign {
			sbID.WriteString(fmt.Sprintf("%d. %s\n", Number, tech.Name))
			sbID.WriteString(fmt.Sprintf("    SPL: %s\n", tech.SPL))
			sbID.WriteString(fmt.Sprintf("    SAC: %s\n", tech.SAC))
			sbID.WriteString(fmt.Sprintf("    Email: %s\n", tech.Email))
			sbID.WriteString(fmt.Sprintf("    No HP: %s\n", tech.NoHP))
			if tech.UserCreatedOn != nil {
				sbID.WriteString(fmt.Sprintf("    Bergabung pada: %s\n", tech.UserCreatedOn.Format("02 Jan 2006")))
			}
			sbID.WriteString("\n")
			Number++
		}

		Number = 1
		sbEN.WriteString(fmt.Sprintf("The following is a list of technicians who resigned %d days ago (%s):\n\n", mustJoinedDays, dateJoined.Format("02 Jan 2006")))
		for _, tech := range technicianResign {
			sbEN.WriteString(fmt.Sprintf("%d. %s\n", Number, tech.Name))
			sbEN.WriteString(fmt.Sprintf("    SPL: %s\n", tech.SPL))
			sbEN.WriteString(fmt.Sprintf("    SAC: %s\n", tech.SAC))
			sbEN.WriteString(fmt.Sprintf("    Email: %s\n", tech.Email))
			sbEN.WriteString(fmt.Sprintf("    Phone No: %s\n", tech.NoHP))
			if tech.UserCreatedOn != nil {
				sbEN.WriteString(fmt.Sprintf("    Joined on: %s\n", tech.UserCreatedOn.Format("02 Jan 2006")))
			}
			sbEN.WriteString("\n")
			Number++
		}

		jidStrHRD := fmt.Sprintf("%s@%s", config.WebPanel.Get().Default.PTHRD[0].PhoneNumber, "s.whatsapp.net")
		originalSenderJID := NormalizeSenderJID(jidStrHRD)
		SendLangMessage(originalSenderJID, sbID.String(), sbEN.String(), "id")
	}

	// Get JO list from ODOO for technicians created on the %d days ago
	allTechniciansCreatedFewDaysAgo := []string{}
	for technicianName := range TechODOOMSDataForContract {
		allTechniciansCreatedFewDaysAgo = append(allTechniciansCreatedFewDaysAgo, technicianName)
	}

	ODOOModel = "project.task"
	domain = []interface{}{
		[]interface{}{"planned_date_begin", ">=", dateJoinedStart.Format("2006-01-02 15:04:05")},
		[]interface{}{"planned_date_begin", "<=", nowEnd.Format("2006-01-02 15:04:05")},
		[]interface{}{"technician_id", "=", allTechniciansCreatedFewDaysAgo},
		// []interface{}{"timesheet_timer_last_stop", "=", false},
	}
	fieldsID := []string{
		"id",
	}

	fields = []string{
		"planned_date_begin",
		"technician_id",
		"helpdesk_ticket_id",
		"x_no_task",
		"timesheet_timer_last_stop",
	}

	order = "id asc"
	odooParams = map[string]interface{}{
		"model":  ODOOModel,
		"domain": domain,
		"fields": fieldsID,
		"order":  order,
	}

	payload = map[string]interface{}{
		"jsonrpc": config.WebPanel.Get().ApiODOO.JSONRPC,
		"params":  odooParams,
	}

	payloadBytes, err = json.Marshal(payload)
	if err != nil {
		return err
	}

	ODOOresponse, err = GetODOOMSData(string(payloadBytes))
	if err != nil {
		errMsg := fmt.Sprintf("failed fetching data from ODOO MS API: %v", err)
		return errors.New(errMsg)
	}

	ODOOResponseArray, ok = ODOOresponse.([]interface{})
	if !ok {
		errMsg := "failed to asset results as []interface{}"
		return errors.New(errMsg)
	}

	ids := extractUniqueIDs(ODOOResponseArray)

	if len(ids) == 0 {
		return errors.New("empty data in ODOO MS")
	}

	const batchSize = 1000
	chunks := chunkIdsSlice(ids, batchSize)
	var allRecords []interface{}

	// Use workers to process chunks concurrently for better performance
	type chunkResult struct {
		records []interface{}
		err     error
		index   int // Add index to maintain order
	}

	resultChan := make(chan chunkResult, len(chunks))
	semaphore := make(chan struct{}, 2) // Reduced from 3 to 2 to lower memory pressure

	// Process chunks with timeout protection
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Minute)
	defer cancel()

	for i, chunk := range chunks {
		go func(chunkIndex int, chunkData []uint64) {
			defer func() {
				if r := recover(); r != nil {
					logrus.Errorf("Panic in chunk processing %d: %v", chunkIndex, r)
					resultChan <- chunkResult{nil, fmt.Errorf("panic in chunk %d: %v", chunkIndex, r), chunkIndex}
				}
			}()

			select {
			case semaphore <- struct{}{}: // Acquire semaphore with timeout
			case <-ctx.Done():
				resultChan <- chunkResult{nil, fmt.Errorf("chunk %d timeout", chunkIndex), chunkIndex}
				return
			}
			defer func() { <-semaphore }() // Release semaphore

			// logrus.Debugf("Processing (%s) chunk %d of %d (IDs %v to %v)", taskDoing, chunkIndex+1, len(chunks), chunkData[0], chunkData[len(chunkData)-1])

			chunkDomain := []interface{}{
				[]interface{}{"id", "=", chunkData},
				[]interface{}{"active", "=", true},
			}

			odooParams := map[string]interface{}{
				"model":  ODOOModel,
				"domain": chunkDomain,
				"fields": fields,
				"order":  order,
			}

			payload := map[string]interface{}{
				"jsonrpc": config.WebPanel.Get().ApiODOO.JSONRPC,
				"params":  odooParams,
			}

			payloadBytes, err := json.Marshal(payload)
			if err != nil {
				resultChan <- chunkResult{nil, fmt.Errorf("failed to marshal payload for chunk %d: %v", chunkIndex+1, err), chunkIndex}
				return
			}

			ODOOresponse, err := GetODOOMSData(string(payloadBytes))
			if err != nil {
				resultChan <- chunkResult{nil, fmt.Errorf("failed fetching data from ODOO MS API for chunk %d: %v", chunkIndex+1, err), chunkIndex}
				return
			}

			ODOOResponseArray, ok := ODOOresponse.([]interface{})
			if !ok {
				resultChan <- chunkResult{nil, fmt.Errorf("type assertion failed for chunk %d", chunkIndex+1), chunkIndex}
				return
			}

			resultChan <- chunkResult{ODOOResponseArray, nil, chunkIndex}
		}(i, chunk)
	}

	// Collect results from all goroutines with timeout protection
	results := make([]chunkResult, len(chunks))
	for i := 0; i < len(chunks); i++ {
		select {
		case result := <-resultChan:
			if result.index < len(results) {
				results[result.index] = result
			}
		case <-ctx.Done():
			logrus.Errorf("Timeout waiting for chunk results")
			return errors.New("timeout waiting for chunk results")
		}
	}

	// Process results in order and handle errors gracefully
	for i, result := range results {
		if result.err != nil {
			logrus.Errorf("Error processing chunk %d: %v", i, result.err)
			continue // Continue with other chunks instead of failing completely
		}
		if result.records != nil {
			// logrus.Debugf("Appending %d records from chunk %d", len(result.records), i)
			allRecords = append(allRecords, result.records...)
		}
	}

	// logrus.Debugf("Finished processing all chunks, total records collected: %d", len(allRecords))
	if len(allRecords) == 0 {
		return fmt.Errorf("no JO found for technicians joined %d days ago or %v", mustJoinedDays, dateJoined.Format("02 Jan 2006"))
	}

	ODOOResponseBytes, err = json.Marshal(allRecords)
	if err != nil {
		return fmt.Errorf("failed to marshal combined response: %v", err)
	}

	// Pre-allocate slice with estimated capacity to reduce memory allocations
	var listOfData []OdooTaskDataRequestItem
	estimatedCapacity := len(allRecords) * 10 // Reduced from 50 to prevent over-allocation
	if estimatedCapacity > 50000 {            // Cap maximum pre-allocation
		estimatedCapacity = 50000
	}
	if estimatedCapacity > 0 {
		listOfData = make([]OdooTaskDataRequestItem, 0, estimatedCapacity)
	}

	if err := json.Unmarshal(ODOOResponseBytes, &listOfData); err != nil {
		errMsg := fmt.Sprintf("failed to unmarshal response body: %v", err)
		return errors.New(errMsg)
	}

	// Log memory usage for monitoring
	var memStats runtime.MemStats
	runtime.ReadMemStats(&memStats)
	// logrus.Infof("Memory usage before DB operations - Allocated: %d MB, System: %d MB",
	// 	memStats.Alloc/1024/1024, memStats.Sys/1024/1024)

	// Force garbage collection to free up memory before database operations
	runtime.GC()

	// Clear old data for today before inserting new batch
	if err := clearOldTechnicianDataContract(forProject); err != nil {
		logrus.Errorf("Failed to clear old technician data: %v", err)
		// Continue anyway - we might be updating existing data
	}

	// Group data by technician and create aggregated records
	groupedData := groupDataByTechnicianForContract(forProject, listOfData)

	// Use a single transaction for all database operations to improve performance
	tx := dbWeb.Begin()
	if tx.Error != nil {
		return fmt.Errorf("failed to begin transaction: %v", tx.Error)
	}

	// Process grouped data in batches
	const dbBatchSize = 1000
	var batch []contracttechnicianmodel.ContractTechnicianODOO
	batchCount := 0

	for _, record := range groupedData {
		// Check if technician already exists for this project
		var existing contracttechnicianmodel.ContractTechnicianODOO
		err := tx.Model(&contracttechnicianmodel.ContractTechnicianODOO{}).
			Where("technician = ? AND for_project = ?", record.Technician, record.ForProject).
			First(&existing).Error
		if err == nil {
			continue // Skip duplicate
		}

		batch = append(batch, record)
		batchCount++

		// Insert batch when it reaches the batch size or at the end
		if len(batch) >= dbBatchSize || batchCount == len(groupedData) {
			if err := tx.Model(&contracttechnicianmodel.ContractTechnicianODOO{}).Create(batch).Error; err != nil {
				if rollbackErr := tx.Rollback().Error; rollbackErr != nil {
					logrus.Errorf("Failed to rollback transaction: %v", rollbackErr)
				}
				return fmt.Errorf("failed to insert batch of (%s) data to DB: %v", taskDoing, err)
			}

			// Log progress
			// logrus.Infof("Progress: processed %d/%d technician records", batchCount, len(groupedData))

			// Reset batch
			batch = make([]contracttechnicianmodel.ContractTechnicianODOO, 0, dbBatchSize)
		}
	}

	// Commit the transaction
	if err := tx.Commit().Error; err != nil {
		if rollbackErr := tx.Rollback().Error; rollbackErr != nil {
			logrus.Errorf("Failed to rollback transaction after commit failure: %v", rollbackErr)
		}
		return fmt.Errorf("failed to commit transaction: %v", err)
	}

	// Update technician last visit times from the batch data
	if err := updateTechnicianLastVisitFromBatchContract(forProject, listOfData); err != nil {
		logrus.Errorf("Failed to update technician last visit times: %v", err)
		// Don't return error as the main data insertion was successful
	}

	// Update technician first uploaded times from the batch data
	if err := updateTechnicianFirstUploadFromBatchContract(forProject, listOfData); err != nil {
		logrus.Errorf("Failed to update technician first upload times: %v", err)
		// Don't return error as the main data insertion was successful
	}

	return nil
}

// groupDataByTechnicianForContract groups the ODOO data by technician and creates aggregated records
func groupDataByTechnicianForContract(forProject string, listOfData []OdooTaskDataRequestItem) []contracttechnicianmodel.ContractTechnicianODOO {
	mustJoinedDays := config.WebPanel.Get().ContractTechnicianODOO.MustJoinedAfter
	if mustJoinedDays <= 0 {
		logrus.Errorf("MUST_JOINED_AFTER config must be greater than 0")
		return nil
	}

	loc, _ := time.LoadLocation(config.WebPanel.Get().Default.Timezone)
	now := time.Now().In(loc)
	dateJoined := now.AddDate(0, 0, -mustJoinedDays)

	// Map to group data by technician
	technicianMap := make(map[string]*TechnicianAggregateDataForContract)
	// Map to collect technicians who never uploaded or visited
	technicianNotVisits := make(map[string]*TechnicianAggregateDataForContract)

	// Process each data item
	for _, data := range listOfData {
		technicianID, technicianName := parseJSONIDDataCombinedSafe(data.TechnicianId)
		if technicianName == "" {
			continue // skip records without technician
		}

		skippedTechnicians := config.WebPanel.Get().ContractTechnicianODOO.SkippedTechnician
		if len(skippedTechnicians) > 0 {
			for _, skippedName := range skippedTechnicians {
				if strings.Contains(strings.ToLower(technicianName), skippedName) {
					continue // Skip if technician name contains skipped keywords
				}
			}
		}

		_, ticketSubjectUncleaned := parseJSONIDDataCombinedSafe(data.HelpdeskTicketId)
		ticketSubject := CleanSPKNumber(ticketSubjectUncleaned)

		// Get or create technician aggregate data
		if technicianMap[technicianName] == nil {
			// Get SPL and SAC from TechODOOMSDataForContract if available
			var spl, sac, techEmail, techNoHP, techName, employeeCode string
			var userCreatedOn *time.Time
			var jobGroupID int
			var nik, alamat, area, ttl string
			if techData, exists := TechODOOMSDataForContract[technicianName]; exists {
				spl = techData.SPL
				sac = techData.SAC
				techEmail = techData.Email
				techNoHP = techData.NoHP
				techName = techData.Name
				userCreatedOn = techData.UserCreatedOn
				jobGroupID = techData.JobGroupID
				nik = techData.NIK
				alamat = techData.Address
				area = techData.Area
				ttl = techData.TTL
				employeeCode = techData.EmployeeCode
			}

			technicianMap[technicianName] = &TechnicianAggregateDataForContract{
				TechnicianID:   technicianID,
				TechnicianName: technicianName,
				SPL:            spl,
				SAC:            sac,
				WONumbers:      []string{},
				TicketSubjects: []string{},
				FirstUploaded:  nil,
				LatestVisit:    nil,
				Email:          techEmail,
				NoHP:           techNoHP,
				Name:           techName,
				UserCreatedOn:  userCreatedOn,
				JobGroupID:     jobGroupID,
				NIK:            nik,
				Address:        alamat,
				Area:           area,
				TTL:            ttl,
				EmployeeCode:   employeeCode,
			}
		}

		// Add WO number to array (just the string value)
		if data.WoNumber != "" {
			technicianMap[technicianName].WONumbers = append(technicianMap[technicianName].WONumbers, data.WoNumber)
		}

		// Add ticket subject to array (just the string value)
		if ticketSubject != "" {
			technicianMap[technicianName].TicketSubjects = append(technicianMap[technicianName].TicketSubjects, ticketSubject)
		}

		// Add WO Number to array that already visit (just the string value)
		if data.WoNumber != "" && data.TimesheetLastStop.Valid {
			technicianMap[technicianName].WONumbersAlreadyVisit = append(technicianMap[technicianName].WONumbersAlreadyVisit, data.WoNumber)
		}

		// Add Ticket Subject to array that already visit (just the string value)
		if ticketSubject != "" && data.TimesheetLastStop.Valid {
			technicianMap[technicianName].TicketSubjectsAlreadyVisit = append(technicianMap[technicianName].TicketSubjectsAlreadyVisit, ticketSubject)
		}

		// Update latest visit time
		if data.TimesheetLastStop.Valid {
			if technicianMap[technicianName].LatestVisit == nil || data.TimesheetLastStop.Time.After(*technicianMap[technicianName].LatestVisit) {
				technicianMap[technicianName].LatestVisit = &data.TimesheetLastStop.Time
			}
		}
	}

	// Convert map to slice of database records
	var result []contracttechnicianmodel.ContractTechnicianODOO
	for _, aggData := range technicianMap {
		// Convert arrays to JSON
		woNumbersJSON, _ := json.Marshal(aggData.WONumbers)
		ticketSubjectsJSON, _ := json.Marshal(aggData.TicketSubjects)
		woNumbersJSONAlreadyVisit, _ := json.Marshal(aggData.WONumbersAlreadyVisit)
		ticketSubjectsJSONAlreadyVisit, _ := json.Marshal(aggData.TicketSubjectsAlreadyVisit)

		// Get login and download times from TechODOOMSData
		var lastLogin, lastDownload *time.Time
		if techData, exists := TechODOOMSDataForContract[aggData.TechnicianName]; exists {
			lastLogin = techData.LastLogin
			lastDownload = techData.LastDownloadJO
		}

		// Collect technicians who never uploaded or visited
		if aggData.FirstUploaded == nil && aggData.LatestVisit == nil {
			technicianNotVisits[aggData.TechnicianName] = aggData
		}

		record := contracttechnicianmodel.ContractTechnicianODOO{
			TechnicianID:              aggData.TechnicianID,
			Technician:                aggData.TechnicianName,
			Name:                      aggData.Name,
			ForProject:                forProject,
			Email:                     aggData.Email,
			Phone:                     aggData.NoHP,
			SPL:                       aggData.SPL,
			SAC:                       aggData.SAC,
			LastLogin:                 lastLogin,
			LastDownloadJO:            lastDownload,
			UserCreatedOn:             aggData.UserCreatedOn,
			FirstUploadJO:             aggData.FirstUploaded,
			LastVisit:                 aggData.LatestVisit,
			WONumber:                  woNumbersJSON,
			TicketSubject:             ticketSubjectsJSON,
			WONumberAlreadyVisit:      woNumbersJSONAlreadyVisit,
			TicketSubjectAlreadyVisit: ticketSubjectsJSONAlreadyVisit,
			JobGroupID:                aggData.JobGroupID,
			NIK:                       aggData.NIK,
			Alamat:                    aggData.Address,
			Area:                      aggData.Area,
			TempatTanggalLahir:        aggData.TTL,
			EmployeeCode:              aggData.EmployeeCode,
		}

		result = append(result, record)
	}

	// Notify HRD about technicians who never uploaded or visited
	go func() {
		if len(technicianNotVisits) > 0 {
			var sbID strings.Builder
			var sbEN strings.Builder
			Number := 1

			sbID.WriteString(fmt.Sprintf("Berikut %d daftar teknisi yang bergabung pada %d hari lalu (%s) namun belum mengupload JO atau belum melakukan kunjungan:\n\n", len(technicianNotVisits), mustJoinedDays, dateJoined.Format("02 Jan 2006")))
			for _, tech := range technicianNotVisits {
				sbID.WriteString(fmt.Sprintf("%d. %s\n", Number, tech.Name))
				sbID.WriteString(fmt.Sprintf("    SPL: %s\n", tech.SPL))
				sbID.WriteString(fmt.Sprintf("    SAC: %s\n", tech.SAC))
				sbID.WriteString(fmt.Sprintf("    Email: %s\n", tech.Email))
				sbID.WriteString(fmt.Sprintf("    No HP: %s\n", tech.NoHP))
				if tech.UserCreatedOn != nil {
					sbID.WriteString(fmt.Sprintf("    Bergabung pada: %s\n", tech.UserCreatedOn.Format("02 Jan 2006")))
				}
				sbID.WriteString("\n")
				Number++
			}

			Number = 1
			sbEN.WriteString(fmt.Sprintf("The following is a list of technicians who joined %d days ago (%s) but have not uploaded any JO or made any visits:\n\n", mustJoinedDays, dateJoined.Format("02 Jan 2006")))
			for _, tech := range technicianNotVisits {
				sbEN.WriteString(fmt.Sprintf("%d. %s\n", Number, tech.Name))
				sbEN.WriteString(fmt.Sprintf("    SPL: %s\n", tech.SPL))
				sbEN.WriteString(fmt.Sprintf("    SAC: %s\n", tech.SAC))
				sbEN.WriteString(fmt.Sprintf("    Email: %s\n", tech.Email))
				sbEN.WriteString(fmt.Sprintf("    Phone No: %s\n", tech.NoHP))
				if tech.UserCreatedOn != nil {
					sbEN.WriteString(fmt.Sprintf("    Joined on: %s\n", tech.UserCreatedOn.Format("02 Jan 2006")))
				}
				sbEN.WriteString("\n")
				Number++
			}

			jidStrHRD := fmt.Sprintf("%s@%s", config.WebPanel.Get().Default.PTHRD[0].PhoneNumber, "s.whatsapp.net")
			originalSenderJID := NormalizeSenderJID(jidStrHRD)
			SendLangMessage(originalSenderJID, sbID.String(), sbEN.String(), "id")
		}
	}()

	return result
}

func updateTechnicianLastVisitFromBatchContract(forProject string, listOfData []OdooTaskDataRequestItem) error {
	// Create a map to track the latest visit time for each technician
	technicianLastVisit := make(map[string]*time.Time)

	// Process the batch data to find the latest visit time for each technician
	for _, data := range listOfData {
		_, technicianName := parseJSONIDDataCombinedSafe(data.TechnicianId)
		if technicianName == "" {
			continue
		}

		// Check if this record has a valid timesheet_timer_last_stop
		if data.TimesheetLastStop.Valid {
			visitTime := &data.TimesheetLastStop.Time

			// Check if this is the latest visit time for this technician
			if existingTime, exists := technicianLastVisit[technicianName]; !exists || visitTime.After(*existingTime) {
				technicianLastVisit[technicianName] = visitTime
			}
		}
	}

	// Update the database with the latest visit times for each technician
	for technician, latestVisit := range technicianLastVisit {
		if err := dbWeb.Model(&contracttechnicianmodel.ContractTechnicianODOO{}).
			Where("technician = ?", technician).
			Where("for_project = ?", forProject).
			Update("last_visit", latestVisit).Error; err != nil {
			logrus.Errorf("Failed to update last visit for technician %s: %v", technician, err)
		} else {
			// logrus.Debugf("Updated last visit for technician %s: %v", technician, latestVisit)
		}
	}

	return nil
}

func updateTechnicianFirstUploadFromBatchContract(forProject string, listOfData []OdooTaskDataRequestItem) error {
	// Create a map to track the earliest upload time for each technician
	technicianFirstUpload := make(map[string]*time.Time)

	// Process the batch data to find the earliest upload time for each technician
	for _, data := range listOfData {
		_, technicianName := parseJSONIDDataCombinedSafe(data.TechnicianId)
		if technicianName == "" {
			continue
		}

		// Check if this record has a valid timesheet_timer_last_stop
		if data.TimesheetLastStop.Valid {
			uploadTime := &data.TimesheetLastStop.Time

			// Check if this is the earliest upload time for this technician
			if existingTime, exists := technicianFirstUpload[technicianName]; !exists || uploadTime.Before(*existingTime) {
				technicianFirstUpload[technicianName] = uploadTime
			}
		}
	}

	// Update the database with the earliest upload times for each technician
	for technician, firstUpload := range technicianFirstUpload {
		if err := dbWeb.Model(&contracttechnicianmodel.ContractTechnicianODOO{}).
			Where("technician = ?", technician).
			Where("for_project = ?", forProject).
			Update("first_upload_jo", firstUpload).Error; err != nil {
			logrus.Errorf("Failed to update first upload for technician %s: %v", technician, err)
		} else {
			// logrus.Debugf("Updated first upload for technician %s: %v", technician, firstUpload)
		}
	}

	return nil
}

func clearOldTechnicianDataContract(forProject string) error {
	loc, _ := time.LoadLocation(config.WebPanel.Get().Default.Timezone)
	now := time.Now().In(loc)

	startOfDay := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, loc)
	endOfDay := time.Date(now.Year(), now.Month(), now.Day(), 23, 59, 59, 0, loc)

	// Delete records that were created today
	result := dbWeb.
		Where("created_at >= ? AND created_at <= ?", startOfDay, endOfDay).
		Where("for_project = ?", forProject).
		Where("contract_file_path IS NULL OR contract_file_path = ''").   // Add this condition to avoid deleting records with contract files, coz HRD soon will re-send the contract files if technician's number phone is changed
		Where("is_contract_sent = ? OR is_contract_sent IS NULL", false). // Add this condition to avoid deleting records that have been sent the contract, coz HRD soon will re-send the contract files if technician's number phone is changed
		Delete(&contracttechnicianmodel.ContractTechnicianODOO{})

	if result.Error != nil {
		return fmt.Errorf("failed to clear old technician data: %v", result.Error)
	}

	// logrus.Infof("Cleared %d old technician records for today", result.RowsAffected)
	return nil
}

func CheckAvailableForContractTechnicianODOO() error {
	taskDoing := "Check Available for Contract Technician ODOO"
	if !checkAvailableForSendContractToTechnicianODOOMutex.TryLock() {
		return fmt.Errorf("%s is already running, skipping this execution", taskDoing)
	}
	defer checkAvailableForSendContractToTechnicianODOOMutex.Unlock()

	if err := ContractTechnicianODOO(); err != nil {
		return fmt.Errorf("error in ContractTechnicianODOO: %v", err)
	}

	loc, _ := time.LoadLocation(config.WebPanel.Get().Default.Timezone)
	now := time.Now().In(loc)

	hour := now.Hour()
	// Greeting logic (ensuring correct 24-hour format)
	var greetingID string
	var greetingEN string
	if hour >= 0 && hour < 4 {
		greetingID = "Selamat Dini Hari" // 00:00 - 03:59
		greetingEN = "Good Early Morning"
	} else if hour >= 4 && hour < 12 {
		greetingID = "Selamat Pagi" // 04:00 - 11:59
		greetingEN = "Good Morning"
	} else if hour >= 12 && hour < 15 {
		greetingID = "Selamat Siang" // 12:00 - 14:59
		greetingEN = "Good Afternoon"
	} else if hour >= 15 && hour < 17 {
		greetingID = "Selamat Sore" // 15:00 - 16:59
		greetingEN = "Good Late Afternoon"
	} else if hour >= 17 && hour < 19 {
		greetingID = "Selamat Petang" // 17:00 - 18:59
		greetingEN = "Good Evening"
	} else {
		greetingID = "Selamat Malam" // 19:00 - 23:59
		greetingEN = "Good Night"
	}

	mustJoinedDays := config.WebPanel.Get().ContractTechnicianODOO.MustJoinedAfter
	if mustJoinedDays <= 0 {
		return fmt.Errorf("MUST_JOINED_AFTER config must be greater than 0")
	}
	dateJoined := now.AddDate(0, 0, -mustJoinedDays)
	dateJoinedStart := time.Date(dateJoined.Year(), dateJoined.Month(), dateJoined.Day(), 0, 0, 0, 0, loc)

	// Changed to include today's data as well
	todayEnd := time.Date(now.Year(), now.Month(), now.Day(), 23, 59, 59, 0, loc)

	dbWeb := gormdb.Databases.Web
	forProject := "ODOO MS"

	var technicians []contracttechnicianmodel.ContractTechnicianODOO
	if err := dbWeb.
		Where("for_project = ?", forProject).
		Where("user_created_on >= ?", dateJoinedStart).
		Where("user_created_on <= ?", todayEnd).
		Find(&technicians).Error; err != nil {
		return fmt.Errorf("failed to fetch technicians from DB: %v", err)
	}

	if len(technicians) == 0 {
		logrus.Infof("No technicians found who joined %d days ago (%s)", mustJoinedDays, dateJoined.Format("02 Jan 2006"))
		return nil
	}

	// Send contract report to each technician who has valid phone number
	for _, tech := range technicians {
		if tech.Phone == "" {
			continue // Skip if no phone number
		}
		if tech.LastLogin == nil || tech.LastDownloadJO == nil {
			continue // Skip if never logged in or never downloaded JO
		}

		if tech.IsContractSent {
			continue // Skip if contract already sent
		}

		skippedTechnicians := config.WebPanel.Get().ContractTechnicianODOO.SkippedTechnician

		if len(skippedTechnicians) > 0 {
			for _, skippedName := range skippedTechnicians {
				if strings.Contains(strings.ToLower(tech.Technician), skippedName) {
					continue // Skip if technician name contains skipped keywords
				}
			}
		}

		_, err := fun.SanitizePhoneNumber(tech.Phone)
		if err != nil {
			continue // Skip if phone number is invalid
		}

		// Check if login or download happened before user was created (invalid data)
		loginBeforeCreation := tech.LastLogin.Before(*tech.UserCreatedOn)
		downloadBeforeCreation := tech.LastDownloadJO.Before(*tech.UserCreatedOn)
		if loginBeforeCreation || downloadBeforeCreation {
			continue // Skip if login/download times are before user creation
		}

		if tech.WONumberAlreadyVisit == nil || tech.TicketSubjectAlreadyVisit == nil {
			continue // Skip if no JO has been marked as visited
		}
		if len(tech.WONumberAlreadyVisit) == 0 || len(tech.TicketSubjectAlreadyVisit) == 0 {
			continue // Skip if no JO has been marked as visited
		}

		var woNumbers []string
		if err := json.Unmarshal(tech.WONumberAlreadyVisit, &woNumbers); err != nil {
			logrus.Errorf("Failed to unmarshal WO numbers for technician %s: %v", tech.Technician, err)
			continue
		}

		// var woNumbersAll []string
		// if err := json.Unmarshal(tech.WONumber, &woNumbersAll); err != nil {
		// 	logrus.Errorf("Failed to unmarshal all WO numbers for technician %s: %v", tech.Technician, err)
		// 	continue
		// }

		if len(woNumbers) == 0 {
			continue // Skip if no JO has been marked as visited
		}

		var phoneNumberUsed string
		var jidStr string
		if config.WebPanel.Get().ContractTechnicianODOO.ActiveDebug {
			phoneNumberUsed = config.WebPanel.Get().ContractTechnicianODOO.PhoneNumberTest
			jidStr = fmt.Sprintf("%s@%s", phoneNumberUsed, "s.whatsapp.net")
		} else {
			phoneNumberUsed = tech.Phone
			jidStr = fmt.Sprintf("62%s@%s", phoneNumberUsed, "s.whatsapp.net")
		}

		var namaTeknisi string
		if tech.Name != "" {
			namaTeknisi = tech.Name
		} else {
			namaTeknisi = tech.Technician
		}
		if namaTeknisi == "" {
			continue // Skip if no name available
		}

		monthRoman, err := fun.MonthToRoman(int(now.Month()))
		if err != nil {
			logrus.Errorf("Failed to convert month to Roman numeral: %v", err)
			continue
		}

		noSurat, err := IncrementNomorSuratContract(dbWeb, "LAST_NOMOR_SURAT_CONTRACT_GENERATED")
		if err != nil {
			logrus.Errorf("Failed to increment nomor surat: %v", err)
			continue
		}
		var noSuratStr string
		if noSurat < 1000 {
			noSuratStr = fmt.Sprintf("%03d", noSurat)
		} else {
			noSuratStr = fmt.Sprintf("%d", noSurat)
		}

		ODOOMSSAC := config.WebPanel.Get().ODOOMSSAC
		SACData, ok := ODOOMSSAC[tech.SAC]
		if !ok {
			logrus.Errorf("SAC %s not found in configuration for technician %s", tech.SAC, tech.Technician)
			continue
		}

		tglSuratKontrak, err := tanggal.Papar(now, "Jakarta", tanggal.WIB)
		if err != nil {
			logrus.Errorf("Failed to format contract date: %v", err)
			continue
		}
		tglSuratKontrakDiterbitkan := tglSuratKontrak.Format(" ", []tanggal.Format{
			tanggal.Hari,      // 27
			tanggal.NamaBulan, // Maret
			tanggal.Tahun,     // 2025
		})

		if tech.UserCreatedOn == nil {
			logrus.Errorf("Skipping technician %s due to nil UserCreatedOn", tech.Technician)
			continue
		}

		var perjanjianBerlakuStart, perjanjianBerlakuEnd string
		tglPerjanjianBerlaku1, err := tanggal.Papar(*tech.UserCreatedOn, "Jakarta", tanggal.WIB)
		if err != nil {
			logrus.Errorf("Failed to format agreement start date for technician %s: %v", tech.Technician, err)
			continue
		}
		perjanjianBerlakuStart = tglPerjanjianBerlaku1.Format(" ", []tanggal.Format{
			tanggal.Hari,      // 27
			tanggal.NamaBulan, // Maret
			tanggal.Tahun,     // 2025
		})

		tglPerjanjianBerlaku2, err := tanggal.Papar(tech.UserCreatedOn.AddDate(1, 0, -1), "Jakarta", tanggal.WIB) // 1 year minus 1 day
		if err != nil {
			logrus.Errorf("Failed to format agreement end date for technician %s: %v", tech.Technician, err)
			continue
		}
		perjanjianBerlakuEnd = tglPerjanjianBerlaku2.Format(" ", []tanggal.Format{
			tanggal.Hari,      // 26
			tanggal.NamaBulan, // Maret
			tanggal.Tahun,     // 2026
		})

		var jobGroupData odooms.ODOOMSJobGroups
		if err := dbWeb.Where("id = ?", tech.JobGroupID).First(&jobGroupData).Error; err != nil {
			logrus.Errorf("Failed to fetch job group data for technician %s: %v", tech.Technician, err)
			continue
		}

		upahPokokStr := fun.FormatRupiah(jobGroupData.BasicSalary * 2) // * 2 coz its got salary 2 times in a month
		pekerjaanPertamaJOStr := fmt.Sprintf("%d", jobGroupData.TaskMax)
		insentifStr := fun.FormatRupiah(jobGroupData.InsentivePerTask)

		var nonWorkPM, nonWorkNONPM string = "Rp. 7.000", "Rp. 7.000"
		var dataFSParam []odooms.ODOOMSFSParams
		if err := dbWeb.Model(&dataFSParam).Where("id != 0").Find(&dataFSParam).Error; err != nil {
			logrus.Errorf("Failed to fetch FS Params data for technician %s: %v", tech.Technician, err)
		}
		for _, fsParam := range dataFSParam {
			lowerFsParam := strings.ToLower(fsParam.ParamKey)
			if strings.Contains(lowerFsParam, "not_worked_price_pm") && fsParam.ParamValue != "" {
				nonWorkPM, err = fun.ReturnRupiahFormat(fsParam.ParamValue)
				if err != nil {
					logrus.Errorf("Failed to format nonWorkPM rupiah for technician %s: %v", tech.Technician, err)
					nonWorkPM = "Rp. 7.000" // default value on error
				}
			} else if strings.Contains(lowerFsParam, "not_worked_price_npm") && fsParam.ParamValue != "" {
				nonWorkNONPM, err = fun.ReturnRupiahFormat(fsParam.ParamValue)
				if err != nil {
					logrus.Errorf("Failed to format nonWorkNONPM rupiah for technician %s: %v", tech.Technician, err)
					nonWorkNONPM = "Rp. 7.000" // default value on error
				}
			}
		}

		placeHolders := map[string]string{
			"$nomor_surat":                       noSuratStr,
			"$bulan_romawi":                      monthRoman,
			"$tahun_contract":                    now.Format("2006"),
			"$nama_teknisi":                      fun.CapitalizeWord(namaTeknisi),
			"$tanggal_surat_kontrak_diterbitkan": tglSuratKontrakDiterbitkan,
			"$sac_nama":                          SACData.FullName,
			"$sac_ttd":                           SACData.TTDPath,

			// Pihak Kedua Page 1
			"$nik_teknisi":    tech.NIK,
			"$alamat_teknisi": tech.Alamat,
			"$area_teknisi":   tech.Area,
			"$ttl_teknisi":    tech.TempatTanggalLahir,
			"$email_teknisi":  tech.Email,

			// Pasal 2 perjanjian date range
			"$perjanjian_berlaku_start": perjanjianBerlakuStart,
			"$perjanjian_berlaku_end":   perjanjianBerlakuEnd,

			// Pasal 5 pembayaran upah
			"$upah_pokok":                 upahPokokStr,
			"$pekerjaan_pertama_total_jo": pekerjaanPertamaJOStr,
			"$insentif":                   insentifStr,
			"$penalty_non_worked_pm":      nonWorkPM,
			"$penalty_non_worked_non_pm":  nonWorkNONPM,
			"$penalty_overdue_pm":         "Rp. 5.000",
			"$penalty_overdue_non_pm":     "Rp. 2.000",
		}

		selectedMainDir, err := fun.FindValidDirectory([]string{
			"web/file/contract_technician",
			"../web/file/contract_technician",
			"../../web/file/contract_technician",
		})
		if err != nil {
			logrus.Errorf("Failed to find valid directory for contract template: %v", err)
			continue
		}

		fileDir := filepath.Join(selectedMainDir, time.Now().Format("2006-01-02"))
		if err := os.MkdirAll(fileDir, 0755); err != nil {
			logrus.Errorf("Failed to create PDF directory %s: %v", fileDir, err)
			continue
		}

		pdfFileName := fmt.Sprintf("Surat Kontrak_%s_%s.pdf", strings.ReplaceAll(namaTeknisi, " ", "_"), now.Format("02Jan2006"))
		pdfFilePath := filepath.Join(fileDir, pdfFileName)
		if err := CreatePDFForContractTechnician(placeHolders, pdfFilePath); err != nil {
			logrus.Errorf("Failed to create PDF contract for technician %s: %v", tech.Technician, err)
			continue
		}

		if err := dbWeb.Model(&contracttechnicianmodel.ContractTechnicianODOO{}).
			Where("for_project = ? AND technician = ?", forProject, tech.Technician).
			Update("contract_file_path", pdfFilePath).
			Error; err != nil {
			logrus.Errorf("Failed to update contract file path for technician %s: %v", tech.Technician, err)
			continue
		}

		if phoneNumberUsed == "" {
			logrus.Errorf("Skipping sending contract to technician %s due to empty phone number", tech.Technician)
			continue
		}

		var sbID strings.Builder
		var sbEN strings.Builder

		sbID.WriteString(fmt.Sprintf("Halo, %s Pak %s.\n\n", greetingID, namaTeknisi))
		sbID.WriteString("Berikut kami lampirkan Surat Kontrak Kerja Teknisi untuk dapat dibaca dan dipahami bersama.\n\n")
		sbID.WriteString("Mohon untuk menyimpan baik-baik surat kontrak tersebut sebagai arsip pribadi.\n\n")
		sbID.WriteString("Terima kasih atas perhatian dan kerjasama Bapak.\n\n")
		sbID.WriteString("Hormat kami,\n")
		sbID.WriteString(fmt.Sprintf("*HRD %s*", config.WebPanel.Get().Default.PT))

		sbEN.WriteString(fmt.Sprintf("Hello, %s Mr. %s.\n\n", greetingEN, namaTeknisi))
		sbEN.WriteString("Attached is the Technician Work Contract Letter for your review and understanding.\n\n")
		sbEN.WriteString("Please keep the contract letter safe as a personal archive.\n\n")
		sbEN.WriteString("Thank you for your attention and cooperation.\n\n")
		sbEN.WriteString("Sincerely,\n")
		sbEN.WriteString(fmt.Sprintf("*HRD %s*", config.WebPanel.Get().Default.PT))

		// Register to chatbot if not exists
		if !config.WebPanel.Get().ContractTechnicianODOO.ActiveDebug {
			go func() {
				allowedTypes := model.AllWAMessageTypes
				jsonBytes, err := json.Marshal(allowedTypes)
				if err != nil {
					logrus.Errorf("Failed to marshal allowedTypes to JSON: %v", err)
				}

				var userChatBot model.WAPhoneUser
				res := dbWeb.
					Where("phone_number = ?", "62"+phoneNumberUsed).
					First(&userChatBot)
				if res.Error != nil {
					if errors.Is(res.Error, gorm.ErrRecordNotFound) {
						newUser := model.WAPhoneUser{
							FullName:      namaTeknisi,
							Email:         tech.Email,
							PhoneNumber:   "62" + phoneNumberUsed,
							IsRegistered:  true,
							AllowedChats:  model.DirectChat,
							AllowedTypes:  datatypes.JSON(jsonBytes),
							AllowedToCall: false,
							Description:   "ODOO MS Technician",
							IsBanned:      false,
							UserType:      model.ODOOMSTechnician,
							MaxDailyQuota: 250,
							UserOf:        model.UserOfCSNA,
						}
						if err := dbWeb.Create(&newUser).Error; err != nil {
							logrus.Errorf("Failed to create WAPhoneUser for technician %s: %v", tech.Technician, err)
						}
						logrus.Infof("✅ Successfully registered new ODOO MS Technician user %s to chatbot with phone number: %s", namaTeknisi, "62"+phoneNumberUsed)
					} else {
						logrus.Errorf("Failed to query WAPhoneUser for technician %s: %v", tech.Technician, res.Error)
					}
				}
			}()
		}

		sendLangDocumentMessageForContractTechnician(forProject, tech.Technician, jidStr, sbID.String(), sbEN.String(), "id", pdfFilePath)
		// ADD: send to technician's email too if needed !
	}

	return nil
}

func CreatePDFForContractTechnician(placeholders map[string]string, outputPath string) error {
	imgAssetsDir, err := fun.FindValidDirectory([]string{
		"web/assets/self/img",
		"../web/assets/self/img",
		"../../web/assets/self/img",
	})
	if err != nil {
		return fmt.Errorf("failed to find image assets directory: %v", err)
	}
	imgCSNA := filepath.Join(imgAssetsDir, "csna_small_jpg.jpg")
	imgTTDSAC := filepath.Join(imgAssetsDir, placeholders["$sac_ttd"])

	fontMainDir, err := fun.FindValidDirectory([]string{
		"web/assets/font",
		"../web/assets/font",
		"../../web/assets/font",
	})
	if err != nil {
		return fmt.Errorf("failed to find font directory: %v", err)
	}

	pdf := fpdf.New("P", "mm", "A4", fontMainDir)
	pdf.SetTitle("Surat Kontrak Kerja", true)
	pdf.SetAuthor(fmt.Sprintf("HRD %s", config.WebPanel.Get().Default.PT), true)
	pdf.SetCreator(fmt.Sprintf("Service Report %s", config.WebPanel.Get().Default.PT), true)
	pdf.SetKeywords("kontrak, surat kontrak, teknisi", true)
	pdf.SetSubject("Surat Kontrak Kerja - Atas bergabungnya karyawan ke perusahaan", true)
	pdf.SetCreationDate(time.Now())
	pdf.SetLang("id-ID")

	// Add fonts
	pdf.AddFont("Arial", "", "arial.json")                       // Regular
	pdf.AddFont("Arial", "B", "arialbd.json")                    // Bold
	pdf.AddFont("Arial", "BI", "arialbi.json")                   // Bold Italic
	pdf.AddFont("Arial", "I", "ariali.json")                     // Italic
	pdf.AddFont("Arial", "Blk", "ariblk.json")                   // Black
	pdf.AddFont("CenturyGothic", "", "CenturyGothic.json")       // Regular
	pdf.AddFont("CenturyGothic", "B", "CenturyGothic-Bold.json") // Bold
	pdf.AddFont("Calibri", "", "calibri.json")
	pdf.AddFont("Calibri", "B", "calibrib.json")

	// Set header function for all pages
	pdf.SetHeaderFuncMode(func() {
		// Draw logo at top left
		pdf.ImageOptions(imgCSNA, 13, 5, 50, 0, false, fpdf.ImageOptions{ImageType: "JPG"}, 0, "")
		// // Draw a horizontal line under header
		// pdf.SetDrawColor(220, 220, 220)
		// pdf.SetLineWidth(0.5)
		// pdf.Line(15, 30, 195, 30)
		// pdf.SetY(35) // Move Y below header for content
	}, true)

	// Set footer function for all pages
	pdf.SetFooterFunc(func() {
		footerY := 283.0 // Bottom margin for A4

		// Footer text
		pdf.SetFont("CenturyGothic", "B", 11)
		pdf.SetTextColor(100, 100, 100)

		// Move to right corner with specific positioning
		companyName := config.WebPanel.Get().Default.PT
		textWidth := pdf.GetStringWidth(companyName)
		pageWidth, _ := pdf.GetPageSize()
		rightMargin := 4.0 // Adjust this value to control distance from right edge

		pdf.SetXY(pageWidth-rightMargin-textWidth, footerY-5)
		pdf.CellFormat(textWidth, 5, companyName, "", 0, "L", false, 0, "")

		// Other info details with MultiCell right aligned
		pdf.SetFont("CenturyGothic", "", 7)
		pdf.SetTextColor(120, 120, 120)

		footerText := "Jl. Puri Utama Blok H1 No.19-22, Kel. Petir Kec. Cipondoh\n" +
			"Kota Tangerang, Banten - Indonesia 15147\n" +
			"Tel.: (021)55717377"

		// Right margin position
		marginRight := 4.0

		lines := strings.Split(footerText, "\n")
		y := footerY - 0.5

		for _, line := range lines {
			textWidth := pdf.GetStringWidth(line)
			x := pageWidth - marginRight - textWidth // shift so it's right-aligned
			pdf.SetXY(x, y)
			pdf.CellFormat(textWidth, 3, line, "", 0, "L", false, 0, "")
			y += 3
		}

		pdf.SetTextColor(0, 0, 0)

	})

	// ########################### Page 1 #################################
	pdf.AddPage()
	// ====================== Title with underline ========================
	currentY := 45.0
	pdf.SetY(currentY)
	pdf.SetFont("Arial", "B", 10) // Use same font to measure text width
	titleText := "PERJANJIAN KERJA WAKTU TERTENTU"
	titleWidth := pdf.GetStringWidth(titleText)
	// Calculate center position
	pageWidth, _ := pdf.GetPageSize()
	lineStartX := (pageWidth - titleWidth) / 2
	lineEndX := lineStartX + titleWidth

	// Write the title text first
	currentY = 30.0
	pdf.SetXY(0, currentY)
	pdf.CellFormat(210, 8, titleText, "", 1, "C", false, 0, "")

	// Draw underline
	currentY += 6 // Move down 6mm from current Y position
	pdf.SetLineWidth(0.5)
	pdf.SetDrawColor(0, 0, 0) // black
	pdf.Line(lineStartX, currentY, lineEndX, currentY)

	// Nomor Surat
	currentY -= 1
	pdf.SetFont("Arial", "B", 10)
	pdf.SetXY(0, currentY)
	textToWrite := fmt.Sprintf("Nomor : %s/Teknisi/HRD-CSNA/%s/%s",
		placeholders["$nomor_surat"],
		placeholders["$bulan_romawi"],
		placeholders["$tahun_contract"],
	)
	pdf.CellFormat(210, 8, textToWrite, "", 1, "C", false, 0, "")
	// ====================================================================

	// Body
	pdf.SetFont("Arial", "", 9)
	pdf.SetY(currentY + 15)
	pdf.SetX(20)
	pdf.CellFormat(0, 7, "Yang bertanda tangan dibawah ini :", "", 1, "L", false, 0, "")

	currentY = pdf.GetY()
	currentY += 1
	pdf.SetY(currentY)
	// define fields
	pihakPertamaFields := []pdfField{
		{"Nama Perusahaan", config.WebPanel.Get().Default.PT},
		{"Alamat", config.WebPanel.Get().Default.PTAddress},
		{"Kota", config.WebPanel.Get().Default.PTCity},
	}

	// loop
	for i, f := range pihakPertamaFields {
		pdf.SetX(20)

		if i == 0 {
			// First line has "I." before the label
			pdf.CellFormat(5, 4, "I.", "", 0, "L", false, 0, "")
		} else {
			// Other lines: keep blank space instead of "I."
			pdf.CellFormat(5, 4, "", "", 0, "L", false, 0, "")
		}

		// label
		pdf.CellFormat(46, 4, f.Label, "", 0, "L", false, 0, "")
		// colon
		pdf.CellFormat(5, 4, ":", "", 0, "L", false, 0, "")
		// value (allow wrapping)
		pdf.MultiCell(120, 4, f.Value, "", "L", false)
	}

	currentY = pdf.GetY()
	currentY += 4
	pdf.SetXY(20, currentY)
	pdf.SetFont("Arial", "", 9)
	pdf.CellFormat(pdf.GetStringWidth("Dalam hal ini bertindak sebagai "), 5, "Dalam hal ini bertindak sebagai ", "", 0, "L", false, 0, "")
	pdf.SetFont("Arial", "B", 9)
	pdf.CellFormat(pdf.GetStringWidth("PIHAK PERTAMA"), 5, "PIHAK PERTAMA", "", 0, "L", false, 0, "")

	pdf.Ln(7)
	pdf.SetFont("Arial", "", 9)
	pihakKeduaFields := []pdfField{
		{"Nama", placeholders["$nama_teknisi"]},
		{"NIK", placeholders["$nik_teknisi"]},
		{"Alamat", placeholders["$alamat_teknisi"]},
		{"Area", placeholders["$area_teknisi"]},
		{"Tempat Tanggal Lahir", placeholders["$ttl_teknisi"]},
		{"Email Address", placeholders["$email_teknisi"]},
	}

	// loop
	for i, f := range pihakKeduaFields {
		pdf.SetX(20)

		if i == 0 {
			// First line has "II." before the label
			pdf.CellFormat(5, 4, "II.", "", 0, "L", false, 0, "")
		} else {
			// Other lines: keep blank space instead of "II."
			pdf.CellFormat(5, 4, "", "", 0, "L", false, 0, "")
		}

		// label
		pdf.CellFormat(46, 4, f.Label, "", 0, "L", false, 0, "")
		// colon
		pdf.CellFormat(5, 4, ":", "", 0, "L", false, 0, "")
		// value (allow wrapping)
		pdf.MultiCell(120, 4, f.Value, "", "L", false)
	}

	currentY = pdf.GetY()
	currentY += 4
	pdf.SetXY(20, currentY)
	pdf.SetFont("Arial", "", 9)
	pdf.CellFormat(pdf.GetStringWidth("Dalam hal ini bertindak sebagai "), 5, "Dalam hal ini bertindak sebagai ", "", 0, "L", false, 0, "")
	pdf.SetFont("Arial", "B", 9)
	pdf.CellFormat(pdf.GetStringWidth("PIHAK KEDUA"), 5, "PIHAK KEDUA", "", 0, "L", false, 0, "")

	// Example: "Dengan ini sepakat bahwa PIHAK KEDUA bekerja di tempat PIHAK PERTAMA dalam bidang pekerjaan “Manage Service EDC” dengan jabatan “Teknisi”"
	// Styles: regular, bold, italic, bold italic
	currentY = pdf.GetY() + 7
	pdf.SetLeftMargin(20)   // ensure wrap always starts at 20
	pdf.SetXY(20, currentY) // starting point

	pdf.SetFont("Arial", "", 9)
	pdf.Write(4, "Dengan ini sepakat bahwa ")

	pdf.SetFont("Arial", "B", 9)
	pdf.Write(4, "PIHAK KEDUA ")

	pdf.SetFont("Arial", "", 9)
	pdf.Write(4, "bekerja di tempat ")

	pdf.SetFont("Arial", "B", 9)
	pdf.Write(4, "PIHAK PERTAMA ")

	pdf.SetFont("Arial", "", 9)
	pdf.Write(4, "dalam bidang pekerjaan ")

	pdf.SetFont("Arial", "BI", 9)
	pdf.Write(4, `"Manage Service EDC" `)

	pdf.SetFont("Arial", "", 9)
	pdf.Write(4, "dengan jabatan ")

	pdf.SetFont("Arial", "BI", 9)
	pdf.Write(4, `"Teknisi"`)

	currentY = pdf.GetY() + 7
	pdf.SetXY(20, currentY)
	pdf.SetFont("Arial", "", 9)
	pdf.Write(4, "Maka dari itu, ")

	pdf.SetFont("Arial", "B", 9)
	pdf.Write(4, "PIHAK PERTAMA ")

	pdf.SetFont("Arial", "", 9)
	pdf.Write(4, "dan ")

	pdf.SetFont("Arial", "B", 9)
	pdf.Write(4, "PIHAK KEDUA ")

	pdf.SetFont("Arial", "", 9)
	pdf.Write(4, "sepakat mengadakan Perjanjian Kerja Waktu Tertentu sesuai dengan ketentuan-ketentuan sebagai berikut : ")

	// Pasal 1
	currentY = pdf.GetY() + 7
	pdf.SetFont("Arial", "B", 9)
	pdf.SetXY(0, currentY)
	pdf.MultiCell(210, 4, "PASAL 1\nRUANG LINGKUP PEKERJAAN", "", "C", false)

	pdf.Ln(5)
	pasal1Items := []ListItem{
		{
			Parts: []TextRun{
				{"Bahwa PIHAK KEDUA", ""},
				{" akan bekerja sebagai ", ""},
				{"Teknisi", "BI"},
				{", dengan tanggung jawab sebagai berikut.:", ""},
			},
			Children: [][]TextRun{
				{
					{"Penyelesaian pekerjaan sesuai dengan SLA yang telah ditentukan.", ""},
				},
			},
		},
		{
			Parts: []TextRun{
				{"Pekerjaan yang dimaksud pada ayat (1), adalah :", ""},
			},
			Children: [][]TextRun{
				{{"Instalasi / pemasangan mesin dan perlengkapannya", ""}},
				{{"Memberikan pelatihan kepada customer tentang penggunaan mesin", ""}},
				{{"Penarikan mesin dan perlengkapannya.", ""}},
				{{"Preventive Maintenance", "I"}}, // italic
				{{"Corrective Maintenance", "I"}}, // italic
				{{"Pengiriman material (Thermal, Adaptor, Sticker, dll)", ""}},
				{{"Melakukan stock opname (Asset) sesuai penugasan, Wajib dilaporkan kepada team Asset/Leader.", ""}},
			},
		},
	}

	for i, item := range pasal1Items {
		// parent number (1., 2., …)
		pdf.SetX(20)
		pdf.SetFont("Arial", "", 9)
		pdf.CellFormat(5, 4, fmt.Sprintf("%d.", i+1), "", 0, "L", false, 0, "")

		// main text with mixed styles
		pdf.SetX(25)
		writeFpdfRuns(pdf, item.Parts, 4)
		pdf.Ln(5)

		// children (a., b., …)
		for j, child := range item.Children {
			pdf.SetX(25)
			pdf.SetFont("Arial", "", 9)
			pdf.CellFormat(5, 4, fmt.Sprintf("%c.", rune('a'+j)), "", 0, "L", false, 0, "")

			pdf.SetX(30)
			writeFpdfRuns(pdf, child, 4)
			pdf.Ln(5)
		}

		pdf.Ln(2)
	}

	// Pasal 2
	currentY = pdf.GetY() + 5
	pdf.SetFont("Arial", "B", 9)
	pdf.SetXY(0, currentY)
	pdf.MultiCell(210, 4, "PASAL 2\nJANGKA WAKTU DAN\nPENGAKHIRAN PERJANJIAN", "", "C", false)

	pdf.Ln(5)
	pasal2Items := []ListItem{
		{
			Parts: []TextRun{
				{"Perjanjian ini berlaku dari tanggal ", ""},
				{placeholders["$perjanjian_berlaku_start"], "B"},
				{" sampai dengan tanggal ", ""},
				{placeholders["$perjanjian_berlaku_end"], "B"},
			},
		},
		{
			Parts: []TextRun{
				{"Dengan berakhirnya Perjanjian Kerjasama ini, maka hubungan kerja antara PIHAK PERTAMA dengan PIHAK KEDUA berakhir secara otomatis", ""},
			},
		},
		{
			Parts: []TextRun{
				{"Dengan berakhirnya hubungan kerja antara PIHAK PERTAMA dengan PIHAK KEDUA, maka tidak ada kewajiban PIHAK PERTAMA untuk memberikan pesangon atau/dan ganti rugi berupa apapun kepada PIHAK KEDUA.", ""},
			},
		},
		{
			Parts: []TextRun{
				{"Selama masa berlakunya Perjanjian Kerja Waktu Tertentu ini, setelah dilakukan evaluasi, PIHAK PERTAMA dapat melakukan Pemutusan Hubungan Kerja sewaktu-waktu, apabila PIHAK KEDUA tidak mencapai Performa/Target yang diberikan oleh PIHAK PERTAMA.", ""},
			},
		},
	}

	for i, item := range pasal2Items {
		// Set Y for each item
		curY := pdf.GetY()
		numberX := 20.0
		startX := 25.0
		usableWidth := 170.0
		lineHeight := 5.0

		// Print number using Text (not CellFormat) for precise placement
		pdf.SetFont("Arial", "", 9)
		pdf.Text(numberX, curY+lineHeight, fmt.Sprintf("%d.", i+1))

		// Start text at startX
		curX := startX
		curY = pdf.GetY()

		// Loop styled parts
		for _, r := range item.Parts {
			pdf.SetFont("Arial", r.Style, 9)
			chunks := pdf.SplitLines([]byte(r.Text), usableWidth-(curX-startX))
			for j, chunk := range chunks {
				pdf.Text(curX, curY+lineHeight, string(chunk))
				if j < len(chunks)-1 {
					curY += lineHeight
					curX = startX
				} else {
					curX += pdf.GetStringWidth(string(chunk))
				}
			}
		}
		// Move Y for next item
		pdf.SetY(curY + lineHeight)
	}

	// ############################ Page 2 #################################
	pdf.AddPage()
	// Pasal 3
	currentY = 35.0
	pdf.SetFont("Arial", "B", 9)
	pdf.SetXY(0, currentY)
	pdf.MultiCell(210, 4, "PASAL 3\nHAK DAN KEWAJIBAN", "", "C", false)

	pdf.Ln(3)
	pasal3Items := []ListItem{
		{
			Parts: []TextRun{
				{"Kewajiban Pihak Pertama", ""},
			},
			Children: [][]TextRun{
				{{"Pihak Pertama WAJIB membayarkan upah kepada Pihak Kedua yang dilaksanakan setiap bulannya.", ""}},
				{{"Pihak Pertama WAJIB memberikan Tunjangan Hari Raya kepada Pihak Kedua yang mempunyai masa kerja 1(satu) bulan secara terus menerus atau lebih.", ""}},
			},
		},
		{
			Parts: []TextRun{
				{"Hak Pihak Pertama", ""},
			},
			Children: [][]TextRun{
				{{"Pihak Pertama BERHAK mendapatkan setiap hasil pekerjaan yang diberikan kepada  Pihak Kedua.", ""}},
				{{"Pihak Pertama BERHAK memberhentikan Pihak Kedua dengan alasan tertentu, seperti tidak mentaati aturan perusahaan, merugikan perusahaan serta melanggar norma yang berlaku.", ""}},
			},
		},
		{
			Parts: []TextRun{
				{"Kewajiban Pihak Kedua", ""},
			},
			Children: [][]TextRun{
				{{"Pihak Kedua WAJIB mematuhi aturan dan standard yang telah ditentukan oleh Pihak Pertama.", ""}},
				{{"Pihak Kedua WAJIB menyimpan informasi yang sifatnya rahasia dan tidak membuka rahasia perusahaan kepada pihak lainnya.", ""}},
				{{"Pihak Kedua WAJIB menjaga Asset yang diberikan oleh Pihak Pertama dan WAJIB mengembalikan Asset kepada Pihak Pertama saat masa kerja berakhir/pengakhiran masa kerja.", ""}},
			},
		},
		{
			Parts: []TextRun{
				{"Hak Pihak Kedua", ""},
			},
			Children: [][]TextRun{
				{{"Pihak Kedua BERHAK menyampaikan pendapatnya secara terbuka, sesuai dengan norma dan aturan yang berlaku.", ""}},
				{{"Pihak Kedua BERHAK menerima Tunjangan Hari Raya yang telah bekerja selama 1 (satu) bulan atau lebih, tetapi kurang dari 1 (satu) tahun, THR diberikan secara proporsional.", ""}},
			},
		},
	}

	for i, item := range pasal3Items {
		// parent number (1., 2., …)
		pdf.SetX(20)
		pdf.SetFont("Arial", "", 9)
		pdf.CellFormat(5, 4, fmt.Sprintf("%d.", i+1), "", 0, "L", false, 0, "")

		// main text with mixed styles
		pdf.SetX(25)
		writeFpdfRuns(pdf, item.Parts, 4)
		pdf.Ln(3)

		// children (a., b., …)
		for j, child := range item.Children {
			pdf.SetX(25)
			pdf.SetFont("Arial", "", 9)
			pdf.CellFormat(5, 4, fmt.Sprintf("%c.", rune('a'+j)), "", 0, "L", false, 0, "")

			pdf.SetX(30)
			// Concatenate all TextRun.Text for the child, applying styles if needed
			var childText string
			for _, r := range child {
				childText += r.Text + " "
			}
			// Use MultiCell for wrapping and indentation
			pdf.MultiCell(170, 4, childText, "", "L", false)
		}
	}

	// Pasal 4
	currentY = pdf.GetY() + 5
	pdf.SetFont("Arial", "B", 9)
	pdf.SetXY(0, currentY)
	pdf.MultiCell(210, 4, "PASAL 4\nWAKTU KERJA TEKNISI", "", "C", false)

	pdf.Ln(3)
	pdf.SetX(20)
	pdf.SetFont("Arial", "", 9)
	pdf.Write(4, "Waktu kerja ")
	pdf.SetFont("Arial", "B", 9)
	pdf.Write(4, "PIHAK KEDUA ")
	pdf.SetFont("Arial", "", 9)
	pdf.Write(4, "disesuaikan dengan kondisi di lapangan dengan ketentuan Hari Kerja dan Jam Kerja berdasarkan tugas yang diberikan dengan memperhatikan SLA pekerjaan tersebut, termasuk pada hari libur.")

	// Pasal 5
	currentY = pdf.GetY() + 7
	pdf.SetFont("Arial", "B", 9)
	pdf.SetXY(0, currentY)
	pdf.MultiCell(210, 4, "PASAL 5\nPEMBAYARAN UPAH", "", "C", false)

	pdf.Ln(3)
	pdf.SetFont("Arial", "", 9)
	pdf.SetX(20)
	pdf.MultiCell(170, 5, "PIHAK PERTAMA akan memberikan upah kepada PIHAK KEDUA dengan ketentuan sebagai berikut :", "", "L", false)

	type Pasal5Item struct {
		Letter string
		Label  string
		Value  []TextRun
	}

	pasal5Items := []Pasal5Item{
		{
			"a", "Upah Pokok",
			[]TextRun{
				{"Rp. ", ""},
				{placeholders["$upah_pokok"], "B"},
				{" /bulan untuk pekerjaan ", ""},
				{placeholders["$pekerjaan_pertama_total_jo"], "B"},
				{" JO pertama dengan status Done.", ""},
			},
		},
		{
			"b", "Insentif",
			[]TextRun{
				{fmt.Sprintf("Rp. %s /JO diberikan untuk kelebihan JO yang dikerjakan dengan status Done.", placeholders["$insentif"]), ""},
			},
		},
		// {
		// 	"c", "Pinalti",
		// 	[]TextRun{
		// 		{"Rp. 5.000 /JO per hari (maksimum Rp. 15.000) akan di potong untuk JO Non-PM yang tidak dikerjakan.", ""},
		// 	},
		// },
		{
			"c", "Pinalti",
			[]TextRun{
				{fmt.Sprintf("%s /JO akan di potong untuk JO Non-PM yang tidak dikerjakan.", placeholders["$penalty_non_worked_non_pm"]), ""},
			},
		},
		{
			"d", "Pinalti",
			[]TextRun{
				{fmt.Sprintf("%s /JO akan di potong untuk JO PM yang tidak dikerjakan.", placeholders["$penalty_non_worked_pm"]), ""},
			},
		},
		{
			"e", "Pinalti",
			[]TextRun{
				{fmt.Sprintf("%s /JO akan di potong untuk JO PM (Preventive Maintenance) yang over SLA per periode bulan PM.", placeholders["$penalty_overdue_pm"]), ""},
			},
		},
	}

	for _, item := range pasal5Items {
		pdf.SetX(25)
		pdf.SetFont("Arial", "", 9)
		pdf.CellFormat(5, 4, fmt.Sprintf("%s.", item.Letter), "", 0, "L", false, 0, "")
		pdf.CellFormat(35, 4, item.Label, "", 0, "L", false, 0, "")
		pdf.CellFormat(5, 4, ":", "", 0, "L", false, 0, "")
		// Use writeFpdfRuns for styled value
		writeFpdfRuns(pdf, item.Value, 4)
		pdf.Ln(4)
	}

	// Use a simple bullet (•) instead of black diamond
	pdf.SetX(25)
	y := pdf.GetY() + 2             // Center vertically with text
	pdf.SetDrawColor(0, 0, 0)       // Black outline
	pdf.SetFillColor(255, 255, 255) // White fill
	pdf.Circle(23, y, 1.5, "D")     // (x, y, radius, "D" for draw/outline only)

	pdf.SetFont("Arial", "BI", 9)
	pdf.MultiCell(165, 4, "Upah sebagaimana tersebut pada pasal ini a.b.c adalah Take Home Pay (sudah dipotong PPH 21)", "", "L", false)

	// Pasal 6
	currentY = pdf.GetY() + 5
	pdf.SetFont("Arial", "B", 9)
	pdf.SetXY(0, currentY)
	pdf.MultiCell(210, 4, "PASAL 6\nSANKSI - SANKSI", "", "C", false)

	pdf.Ln(3)
	pasal6Items := []ListItem{
		{
			Parts: []TextRun{
				{"PIHAK PERTAMA akan memberikan Sanksi berupa teguran Lisan,SP-1, SP-2 sampai dengan SP-3 (Pemutusan Hubungan Kerja)  kepada PIHAK KEDUA apabila PIHAK KEDUA :", ""},
			},
			Children: [][]TextRun{
				{{"Lalai dalam melakukan tugas yang diberikan.", ""}},
				{{"Tidak mematuhi aturan yang diberikan oleh Perusahaan.", ""}},
			},
		},
		{
			Parts: []TextRun{
				{"PIHAK PERTAMA dapat melakukan Pemutusan Hubungan Kerja (PHK) apabila PIHAK KEDUA melakukan kesalahan berat serta merugikan perusahaan ataupun tersangkut masalah hukum pidana baik didalam maupun diluar perusahaan sesuai Undang-Undang Nomor 13 Tahun 2003, tentang Ketenagakerjaan.", ""},
			},
		},
		{
			Parts: []TextRun{
				{"PIHAK KEDUA Wajib membayarkan ganti rugi (sebesar harga Asset yang diberikan) kepada PIHAK PERTAMA atau dilakukan pemotongan gaji pada PIHAK KEDUA jika melanggar Pasal 3 Ayat 3 Poin C.", ""},
			},
		},
		{
			Parts: []TextRun{
				{"PIHAK KEDUA Tidak Diperbolehkan bekerja di 2 (dua) Perusahaan/Vendor lain, apabila melanggar akan dikenakan Pinalty dengan WAJIB membayarkan kepada PIHAK PERTAMA 3 (tiga) kali lipat dari Jumlah Gaji yang diterima PIHAK KEDUA dan Pemutusan Hubungan Kerja (PHK).", ""},
			},
		},
	}

	for i, item := range pasal6Items {
		numberX := 20.0
		textX := 25.0
		usableWidth := 165.0 // 170 - 5 for number width
		lineHeight := 4.0

		// Prepare the full parent text with styles
		var parentText string
		for _, r := range item.Parts {
			parentText += r.Text + " "
		}

		// Split into lines for wrapping
		lines := pdf.SplitLines([]byte(parentText), usableWidth)
		for j, line := range lines {
			if j == 0 {
				pdf.SetXY(numberX, pdf.GetY())
				pdf.SetFont("Arial", "", 9)
				pdf.CellFormat(5, lineHeight, fmt.Sprintf("%d.", i+1), "", 0, "L", false, 0, "")
				pdf.SetXY(textX, pdf.GetY())
				pdf.MultiCell(usableWidth, lineHeight, string(line), "", "L", false)
			} else {
				pdf.SetXY(textX, pdf.GetY())
				pdf.MultiCell(usableWidth, lineHeight, string(line), "", "L", false)
			}
		}

		// children (a., b., …)
		for j, child := range item.Children {
			pdf.SetX(25)
			pdf.SetFont("Arial", "", 9)
			pdf.CellFormat(5, 4, fmt.Sprintf("%c.", rune('a'+j)), "", 0, "L", false, 0, "")

			pdf.SetX(30)
			var childText string
			for _, r := range child {
				childText += r.Text + " "
			}
			pdf.MultiCell(170, 4, childText, "", "L", false)
		}
		// pdf.Ln(2)
	}

	// ############################ Page 3 #################################
	pdf.AddPage()
	// Pasal 7
	currentY = 35.0
	pdf.SetFont("Arial", "B", 9)
	pdf.SetXY(0, currentY)
	pdf.MultiCell(210, 4, "PASAL 7\nPERSELISIHAN", "", "C", false)

	pdf.Ln(3)
	pasal7Items := []ListItem{
		{
			Parts: []TextRun{
				{"Apabila terjadi perselisihan para pihak sepakat diselesaikan secara musyawarah dan mufakat.", ""},
			},
		},
		{
			Parts: []TextRun{
				{"Apabila penyelesaian perselisihan secara musyawarah mufakat mengalami jalan buntu, maka kedua belah pihak sepakat untuk diselesaikan sesuai dengan hukum yang berlaku di NKRI.", ""},
			},
		},
	}

	for i, item := range pasal7Items {
		numberX := 20.0
		textX := 25.0
		usableWidth := 165.0 // 170 - 5 for number width
		lineHeight := 4.0

		// Prepare the full parent text with styles
		var parentText string
		for _, r := range item.Parts {
			parentText += r.Text + " "
		}

		// Split into lines for wrapping
		lines := pdf.SplitLines([]byte(parentText), usableWidth)
		for j, line := range lines {
			if j == 0 {
				pdf.SetXY(numberX, pdf.GetY())
				pdf.SetFont("Arial", "", 9)
				pdf.CellFormat(5, lineHeight, fmt.Sprintf("%d.", i+1), "", 0, "L", false, 0, "")
				pdf.SetXY(textX, pdf.GetY())
				pdf.MultiCell(usableWidth, lineHeight, string(line), "", "L", false)
			} else {
				pdf.SetXY(textX, pdf.GetY())
				pdf.MultiCell(usableWidth, lineHeight, string(line), "", "L", false)
			}
		}

		// children (a., b., …)
		for j, child := range item.Children {
			pdf.SetX(25)
			pdf.SetFont("Arial", "", 9)
			pdf.CellFormat(5, 4, fmt.Sprintf("%c.", rune('a'+j)), "", 0, "L", false, 0, "")

			pdf.SetX(30)
			var childText string
			for _, r := range child {
				childText += r.Text + " "
			}
			pdf.MultiCell(170, 4, childText, "", "L", false)
		}
		// pdf.Ln(2)
	}

	// Pasal 8
	currentY = pdf.GetY() + 5
	pdf.SetFont("Arial", "B", 9)
	pdf.SetXY(0, currentY)
	pdf.MultiCell(210, 4, "PASAL 8\nPENUTUP", "", "C", false)

	pdf.Ln(3)
	pasal8Text := "Demikian Perjanjian Kerja Waktu Tertentu ini dibuat rangkap 2 (dua) bermeterai cukup dan setelah para pihak membaca, mengerti serta menandatanganinya dalam keadaan sadar, sehat jasmani dan rohani tanpa ada paksaan dari siapapun dan dari pihak manapun, kemudian masing-masing pihak memegang 1 (satu) bundel asli."
	pdf.SetFont("Arial", "", 9)
	pdf.MultiCell(170, 4, pasal8Text, "", "J", false)

	// =================================== Signatures ===================================
	currentY += 35
	leftX := 35.0
	rightX := 155.0 // adjust for your page width

	pdf.SetFont("Arial", "", 9)
	pdf.SetXY(20, currentY)
	pdf.CellFormat(80, 5, fmt.Sprintf("Tangerang, %v", placeholders["$tanggal_surat_kontrak_diterbitkan"]), "", 0, "L", false, 0, "")

	currentY += 8
	pdf.SetFont("Calibri", "B", 11)
	pdf.SetXY(leftX, currentY)
	pdf.CellFormat(80, 5, "PIHAK PERTAMA", "", 0, "L", false, 0, "")
	pdf.SetXY(rightX, currentY)
	pdf.CellFormat(80, 5, "PIHAK KEDUA", "", 0, "L", false, 0, "")

	currentY += 35 // space for signatures

	// --- Left signature ---
	ttdSacWidth := 55.0
	leftXForTTD := leftX
	currentYTTDSAC := currentY - 33
	switch placeholders["$sac_ttd"] {
	case "ttd_angga.png":
		leftXForTTD = leftX - 10.0
		currentYTTDSAC = currentY - 25
	case "ttd_osvaldo.png":
		ttdSacWidth = 33.0
		leftXForTTD = leftX - 3.0
		currentYTTDSAC = currentY - 30
	case "ttd_tomi.png":
		leftXForTTD = leftX - 8.0
	case "ttd_burhan.png":
		ttdSacWidth = 13.0
		leftXForTTD = leftX + 8.0
		currentYTTDSAC = currentY - 27
	case "ttd_tetty.png":
		ttdSacWidth = 28.0
		leftXForTTD = leftX + 0.0
		currentYTTDSAC = currentY - 30
	}
	pdf.ImageOptions(imgTTDSAC, leftXForTTD, currentYTTDSAC, ttdSacWidth, 0, false, fpdf.ImageOptions{ImageType: "PNG"}, 0, "")

	labelDiterbitkan := "PIHAK PERTAMA"
	labelWidth := pdf.GetStringWidth(labelDiterbitkan)
	nameWidth := pdf.GetStringWidth(placeholders["$sac_nama"])
	// padding := 4.0

	// Compute X so the name is centered under the label
	centerX := leftX + (labelWidth / 2) - (nameWidth / 2)

	pdf.SetXY(centerX, currentY)
	pdf.CellFormat(nameWidth, 5, placeholders["$sac_nama"], "", 0, "L", false, 0, "")

	// Underline
	pdf.SetDrawColor(0, 0, 0)
	pdf.SetLineWidth(0.5)
	// pdf.Line(centerX, currentY+5, centerX+nameWidth+padding, currentY+5)
	pdf.Line(centerX+2, currentY+5, centerX+nameWidth, currentY+5)

	roleWidth := pdf.GetStringWidth("Service Area Coordinator")
	roleX := leftX + (labelWidth / 2) - (roleWidth / 2)
	pdf.SetXY(roleX, currentY+5)
	pdf.CellFormat(roleWidth, 5, "Service Area Coordinator", "", 0, "L", false, 0, "")

	// --- Right signature ---
	labelMengetahui := "PIHAK KEDUA"
	labelWidthR := pdf.GetStringWidth(labelMengetahui)
	mgrWidth := pdf.GetStringWidth(placeholders["$nama_teknisi"])

	// Compute X so the SAC name is centered under the label
	centerXR := rightX + (labelWidthR / 2) - (mgrWidth / 2)

	pdf.SetXY(centerXR, currentY)
	pdf.CellFormat(mgrWidth, 5, placeholders["$nama_teknisi"], "", 0, "L", false, 0, "")

	// Underline
	pdf.SetDrawColor(0, 0, 0)
	pdf.SetLineWidth(0.5)
	// pdf.Line(centerXR, currentY+5, centerXR+mgrWidth+padding, currentY+5)
	pdf.Line(centerXR+2, currentY+5, centerXR+mgrWidth, currentY+5)

	roleR := "Teknisi"
	roleRWidth := pdf.GetStringWidth(roleR)
	roleRX := rightX + (labelWidthR / 2) - (roleRWidth / 2)

	pdf.SetXY(roleRX, currentY+5)
	pdf.CellFormat(roleRWidth, 5, roleR, "", 0, "L", false, 0, "")
	// ==================================================================================

	return pdf.OutputFileAndClose(outputPath)
}

func IncrementNomorSuratContract(db *gorm.DB, id string) (int, error) {
	var nomorSurat contracttechnicianmodel.NomorSuratContract

	// Find or create the row for given ID
	if err := db.FirstOrCreate(&nomorSurat, contracttechnicianmodel.NomorSuratContract{
		ID: id,
	}).Error; err != nil {
		return 0, err
	}

	// Increment
	nomorSurat.LastNomorSurat++

	// Save update
	if err := db.Save(&nomorSurat).Error; err != nil {
		return 0, err
	}

	return nomorSurat.LastNomorSurat, nil
}

func writeFpdfRuns(pdf *fpdf.Fpdf, runs []TextRun, lineHeight float64) {
	for _, r := range runs {
		pdf.SetFont("Arial", r.Style, 9)
		pdf.Write(lineHeight, r.Text+" ")
	}
}

// New Contract Technicians Logic
func GetDataTechnicianForContractInODOO() error {
	taskDoing := "Get data from fs.technician for Kontrak Teknisi"

	if !getDataFSTechnicianInODOOForKontrakTeknisiMutex.TryLock() {
		return fmt.Errorf("%s is still running, please wait until it's finished", taskDoing)
	}
	defer getDataFSTechnicianInODOOForKontrakTeknisiMutex.Unlock()

	loc, _ := time.LoadLocation(config.WebPanel.Get().Default.Timezone)
	now := time.Now().In(loc)
	startTime := now
	nowEnd := time.Date(now.Year(), now.Month(), now.Day(), 23, 59, 59, 0, loc)

	/*
		Manage Service - EDC
	*/
	forProject := "MS_EDC"
	ODOOModel := "fs.technician"
	domain := []interface{}{
		[]interface{}{"active", "=", true},
	}
	fields := []string{
		"id",
		"name",
		"email",
		"x_no_telp",
		"x_technician_name",
		"technician_code",
		"x_spl_leader",
		"login_ids",
		"download_ids",
		"create_date",
		"job_group_id",
		"x_employee_code",
		"work_location",
		"nik",
		"address",
		"area",
		"birth_status",
		"marriage_status",
		"payment_bank",
		"payment_bank_id",
		"payment_bank_name",
	}
	order := "name asc"

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
		return fmt.Errorf("failed to marshal ODOO MS request payload: %v", err)
	}
	ODOOresponse, err := GetODOOMSData(string(payloadBytes))
	if err != nil {
		return fmt.Errorf("failed fetching data from ODOO MS API: %v", err)
	}
	ODOOResponseArray, ok := ODOOresponse.([]interface{})
	if !ok {
		return errors.New("failed to assert ODOO MS response as []interface{}")
	}

	ODOOResponseBytes, err := json.Marshal(ODOOResponseArray)
	if err != nil {
		return fmt.Errorf("failed to marshal ODOO response: %v", err)
	}

	var employeeData []ODOOMSTechnicianItem
	if err := json.Unmarshal(ODOOResponseBytes, &employeeData); err != nil {
		return fmt.Errorf("failed to unmarshal ODOO response body: %v", err)
	}

	if len(employeeData) == 0 {
		logrus.Infof("%s: no data found, skipping processing", taskDoing)
		return nil
	}

	// Collect all login and download IDs for batch processing
	var allLoginIDs []float64
	var allDownloadIDs []float64
	technicianLoginMap := make(map[string]float64)    // technician name -> latest login ID
	technicianDownloadMap := make(map[string]float64) // technician name -> latest download ID

	technicianNotHasValidPhone := make(map[string]TechnicianODOOData)
	technicianResign := make(map[string]TechnicianODOOData)

	// Process each technician to get their latest login/download IDs
	for _, emp := range employeeData {
		technicianName := emp.NameFS.String
		if technicianName == "" {
			continue
		}

		technicianSkipped := config.WebPanel.Get().ContractTechnicianODOO.SkippedTechnician
		if len(technicianSkipped) > 0 {
			for _, skippedName := range technicianSkipped {
				if strings.Contains(strings.ToLower(technicianName), skippedName) {
					logrus.Infof("%s: skipping technician %s as it's in skipped list", taskDoing, technicianName)
					continue
				}
			}
		}

		// Find latest login ID
		if len(emp.LoginIDs) > 0 {
			sort.Slice(emp.LoginIDs, func(i, j int) bool {
				if emp.LoginIDs[i].Valid && emp.LoginIDs[j].Valid {
					return emp.LoginIDs[i].Float > emp.LoginIDs[j].Float
				}
				return emp.LoginIDs[i].Valid
			})
			if emp.LoginIDs[0].Valid {
				lastLoginID := emp.LoginIDs[0].Float
				technicianLoginMap[technicianName] = lastLoginID
				allLoginIDs = append(allLoginIDs, lastLoginID)
			}
		}

		// Find latest download ID
		if len(emp.DownloadIDs) > 0 {
			sort.Slice(emp.DownloadIDs, func(i, j int) bool {
				if emp.DownloadIDs[i].Valid && emp.DownloadIDs[j].Valid {
					return emp.DownloadIDs[i].Float > emp.DownloadIDs[j].Float
				}
				return emp.DownloadIDs[i].Valid
			})
			if emp.DownloadIDs[0].Valid {
				lastDownloadID := emp.DownloadIDs[0].Float
				technicianDownloadMap[technicianName] = lastDownloadID
				allDownloadIDs = append(allDownloadIDs, lastDownloadID)
			}
		}

		phoneNumberUsed := emp.NoTelp.String

		var userCreatedOn *time.Time
		if emp.CreatedOn.Valid {
			createdTime, err := time.Parse("2006-01-02 15:04:05", emp.CreatedOn.String)
			if err != nil {
				logrus.Errorf("Failed to parse created date for technician: %v", err)
			} else {
				createdTime = createdTime.Add(7 * time.Hour)
				userCreatedOn = &createdTime
			}
		}

		// If not registered in whatsapp then you will send the list to HRD
		var noHPTeknisi string
		sanitizedPhone, err := fun.SanitizePhoneNumber(phoneNumberUsed)
		if err != nil {
			logrus.Errorf("Failed to sanitize phone number %s of technician %s: %v", phoneNumberUsed, emp.TechnicianName.String, err)
			technicianNotHasValidPhone[technicianName] = TechnicianODOOData{
				SPL:            emp.SPL.String,
				SAC:            emp.Head.String,
				LastLogin:      nil,
				LastDownloadJO: nil,
				Email:          emp.Email.String,
				NoHP:           phoneNumberUsed,
				Name:           emp.TechnicianName.String,
				UserCreatedOn:  userCreatedOn,
				EmployeeCode:   emp.EmployeeCode.String,
			}
		} else {
			noHPTeknisi = "62" + sanitizedPhone
		}

		jobGroupID, _, err := parseJSONIDDataCombined(emp.JobGroupId)
		if err != nil {
			logrus.Errorf("Failed to parse job group ID for technician %s: %v", technicianName, err)
		}

		// Initialize technician data with basic info
		TechODOOMSDataForContract[technicianName] = TechnicianODOOData{
			SPL:            emp.SPL.String,
			SAC:            emp.Head.String,
			LastLogin:      nil,
			LastDownloadJO: nil,
			Email:          emp.Email.String,
			NoHP:           noHPTeknisi,
			Name:           emp.TechnicianName.String,
			UserCreatedOn:  userCreatedOn,
			JobGroupID:     jobGroupID,
			NIK:            emp.NIK.String,
			Address:        emp.Alamat.String,
			Area:           emp.Area.String,
			TTL:            emp.TempatTanggalLahir.String,
			EmployeeCode:   emp.EmployeeCode.String,
		}

		if strings.Contains(technicianName, "*") {
			// Resigned technician
			technicianResign[technicianName] = TechnicianODOOData{
				SPL:            emp.SPL.String,
				SAC:            emp.Head.String,
				LastLogin:      nil,
				LastDownloadJO: nil,
				Email:          emp.Email.String,
				NoHP:           noHPTeknisi,
				Name:           emp.TechnicianName.String,
				UserCreatedOn:  userCreatedOn,
				EmployeeCode:   emp.EmployeeCode.String,
			}
		}
	}

	// Batch get all login and download times
	loginTimes, downloadTimes, err := getBatchLoginAndDownloadTimes(allLoginIDs, allDownloadIDs)
	if err != nil {
		logrus.Errorf("Failed to get batch login/download times: %v", err)
		// Continue without login/download times
	}

	// Update technician data with login/download times
	for technicianName, data := range TechODOOMSDataForContract {
		// Update login time
		if loginID, exists := technicianLoginMap[technicianName]; exists {
			if loginTime, found := loginTimes[loginID]; found {
				data.LastLogin = loginTime
			}
		}

		// Update download time
		if downloadID, exists := technicianDownloadMap[technicianName]; exists {
			if downloadTime, found := downloadTimes[downloadID]; found {
				data.LastDownloadJO = downloadTime
			}
		}

		// Update the map with new data
		TechODOOMSDataForContract[technicianName] = data

		// Update the map of Resign Technician - only if already marked as resigned
		if _, exists := technicianResign[technicianName]; exists {
			technicianResign[technicianName] = data
		}
		// Update the map of Not Valid Phone Number - only if already marked as invalid phone
		if _, exists := technicianNotHasValidPhone[technicianName]; exists {
			technicianNotHasValidPhone[technicianName] = data
		}
	}

	// Send message to HRD if there are technicians without valid phone numbers
	if len(technicianNotHasValidPhone) > 0 {
		var sbID strings.Builder
		var sbEN strings.Builder
		Number := 1

		sbID.WriteString(fmt.Sprintf("Berikut %d daftar teknisi yang tidak memiliki nomor HP yang valid atau belum terdaftar di WhatsApp:\n\n", len(technicianNotHasValidPhone)))
		for _, tech := range technicianNotHasValidPhone {
			sbID.WriteString(fmt.Sprintf("%d. %s\n", Number, tech.Name))
			sbID.WriteString(fmt.Sprintf("    SPL: %s\n", tech.SPL))
			sbID.WriteString(fmt.Sprintf("    SAC: %s\n", tech.SAC))
			sbID.WriteString(fmt.Sprintf("    Email: %s\n", tech.Email))
			sbID.WriteString(fmt.Sprintf("    No HP: %s\n", tech.NoHP))
			if tech.UserCreatedOn != nil {
				sbID.WriteString(fmt.Sprintf("    Bergabung pada: %s\n", tech.UserCreatedOn.Format("02 Jan 2006")))
			}
			sbID.WriteString("\n")
			Number++
		}

		Number = 1
		sbEN.WriteString(fmt.Sprintf("The following is a list of %d technicians who do not have a valid phone number or are not registered on WhatsApp:\n\n", len(technicianNotHasValidPhone)))
		for _, tech := range technicianNotHasValidPhone {
			sbEN.WriteString(fmt.Sprintf("%d. %s\n", Number, tech.Name))
			sbEN.WriteString(fmt.Sprintf("    SPL: %s\n", tech.SPL))
			sbEN.WriteString(fmt.Sprintf("    SAC: %s\n", tech.SAC))
			sbEN.WriteString(fmt.Sprintf("    Email: %s\n", tech.Email))
			sbEN.WriteString(fmt.Sprintf("    Phone No: %s\n", tech.NoHP))
			if tech.UserCreatedOn != nil {
				sbEN.WriteString(fmt.Sprintf("    Joined on: %s\n", tech.UserCreatedOn.Format("02 Jan 2006")))
			}
			sbEN.WriteString("\n")
			Number++
		}

		jidStrHRD := fmt.Sprintf("%s@%s", config.WebPanel.Get().Default.PTHRD[0].PhoneNumber, "s.whatsapp.net")
		originalSenderJID := NormalizeSenderJID(jidStrHRD)
		SendLangMessage(originalSenderJID, sbID.String(), sbEN.String(), "id")
	}

	if len(technicianResign) > 0 {
		var sbID strings.Builder
		var sbEN strings.Builder
		Number := 1

		sbID.WriteString(fmt.Sprintf("Berikut %d daftar teknisi yang telah resign:\n\n", len(technicianResign)))
		for _, tech := range technicianResign {
			sbID.WriteString(fmt.Sprintf("%d. %s\n", Number, tech.Name))
			sbID.WriteString(fmt.Sprintf("    SPL: %s\n", tech.SPL))
			sbID.WriteString(fmt.Sprintf("    SAC: %s\n", tech.SAC))
			sbID.WriteString(fmt.Sprintf("    Email: %s\n", tech.Email))
			sbID.WriteString(fmt.Sprintf("    No HP: %s\n", tech.NoHP))
			if tech.UserCreatedOn != nil {
				sbID.WriteString(fmt.Sprintf("    Bergabung pada: %s\n", tech.UserCreatedOn.Format("02 Jan 2006")))
			}
			sbID.WriteString("\n")
			Number++
		}

		Number = 1
		sbEN.WriteString(fmt.Sprintf("The following is a list of %d technicians who resigned:\n\n", len(technicianResign)))
		for _, tech := range technicianResign {
			sbEN.WriteString(fmt.Sprintf("%d. %s\n", Number, tech.Name))
			sbEN.WriteString(fmt.Sprintf("    SPL: %s\n", tech.SPL))
			sbEN.WriteString(fmt.Sprintf("    SAC: %s\n", tech.SAC))
			sbEN.WriteString(fmt.Sprintf("    Email: %s\n", tech.Email))
			sbEN.WriteString(fmt.Sprintf("    Phone No: %s\n", tech.NoHP))
			if tech.UserCreatedOn != nil {
				sbEN.WriteString(fmt.Sprintf("    Joined on: %s\n", tech.UserCreatedOn.Format("02 Jan 2006")))
			}
			sbEN.WriteString("\n")
			Number++
		}

		jidStrHRD := fmt.Sprintf("%s@%s", config.WebPanel.Get().Default.PTHRD[0].PhoneNumber, "s.whatsapp.net")
		originalSenderJID := NormalizeSenderJID(jidStrHRD)
		SendLangMessage(originalSenderJID, sbID.String(), sbEN.String(), "id")
	}

	// Trace JO technicians by must joinedDays
	mustJoinedDays := config.WebPanel.Get().ContractTechnicianODOO.MustJoinedAfter
	if mustJoinedDays <= 0 {
		return errors.New("MUST_JOINED_AFTER config must be greater than 0")
	}
	dateJoined := now.AddDate(0, 0, -mustJoinedDays)
	dateJoinedStart := time.Date(dateJoined.Year(), dateJoined.Month(), dateJoined.Day(), 0, 0, 0, 0, loc)

	// Get JO list from ODOO for technicians with planned is %d days ago
	allTechniciansDataPlanned := []string{}
	for technicianName := range TechODOOMSDataForContract {
		allTechniciansDataPlanned = append(allTechniciansDataPlanned, technicianName)
	}

	ODOOModel = "project.task"
	domain = []interface{}{
		[]interface{}{"planned_date_begin", ">=", dateJoinedStart.Format("2006-01-02 15:04:05")},
		[]interface{}{"planned_date_begin", "<=", nowEnd.Format("2006-01-02 15:04:05")},
		[]interface{}{"technician_id", "=", allTechniciansDataPlanned},
	}
	fieldsID := []string{
		"id",
	}

	fields = []string{
		"planned_date_begin",
		"technician_id",
		"helpdesk_ticket_id",
		"x_no_task",
		"timesheet_timer_last_stop",
	}

	order = "id asc"
	odooParams = map[string]interface{}{
		"model":  ODOOModel,
		"domain": domain,
		"fields": fieldsID,
		"order":  order,
	}

	payload = map[string]interface{}{
		"jsonrpc": config.WebPanel.Get().ApiODOO.JSONRPC,
		"params":  odooParams,
	}

	payloadBytes, err = json.Marshal(payload)
	if err != nil {
		return err
	}

	ODOOresponse, err = GetODOOMSData(string(payloadBytes))
	if err != nil {
		errMsg := fmt.Sprintf("failed fetching data from ODOO MS API: %v", err)
		return errors.New(errMsg)
	}

	ODOOResponseArray, ok = ODOOresponse.([]interface{})
	if !ok {
		errMsg := "failed to asset results as []interface{}"
		return errors.New(errMsg)
	}

	ids := extractUniqueIDs(ODOOResponseArray)

	if len(ids) == 0 {
		return errors.New("empty data in ODOO MS_EDC")
	}

	const batchSize = 1000
	chunks := chunkIdsSlice(ids, batchSize)
	var allRecords []interface{}

	// Use workers to process chunks concurrently for better performance
	type chunkResult struct {
		records []interface{}
		err     error
		index   int // Add index to maintain order
	}

	resultChan := make(chan chunkResult, len(chunks))
	semaphore := make(chan struct{}, 3)

	// Process chunks with timeout protection
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Minute)
	defer cancel()

	for i, chunk := range chunks {
		go func(chunkIndex int, chunkData []uint64) {
			defer func() {
				if r := recover(); r != nil {
					logrus.Errorf("Panic in chunk processing %d: %v", chunkIndex, r)
					resultChan <- chunkResult{nil, fmt.Errorf("panic in chunk %d: %v", chunkIndex, r), chunkIndex}
				}
			}()

			select {
			case semaphore <- struct{}{}: // Acquire semaphore with timeout
			case <-ctx.Done():
				resultChan <- chunkResult{nil, fmt.Errorf("chunk %d timeout", chunkIndex), chunkIndex}
				return
			}
			defer func() { <-semaphore }() // Release semaphore

			// logrus.Debugf("Processing (%s) chunk %d of %d (IDs %v to %v)", taskDoing, chunkIndex+1, len(chunks), chunkData[0], chunkData[len(chunkData)-1])

			chunkDomain := []interface{}{
				[]interface{}{"id", "=", chunkData},
				[]interface{}{"active", "=", true},
			}

			odooParams := map[string]interface{}{
				"model":  ODOOModel,
				"domain": chunkDomain,
				"fields": fields,
				"order":  order,
			}

			payload := map[string]interface{}{
				"jsonrpc": config.WebPanel.Get().ApiODOO.JSONRPC,
				"params":  odooParams,
			}

			payloadBytes, err := json.Marshal(payload)
			if err != nil {
				resultChan <- chunkResult{nil, fmt.Errorf("failed to marshal payload for chunk %d: %v", chunkIndex+1, err), chunkIndex}
				return
			}

			ODOOresponse, err := GetODOOMSData(string(payloadBytes))
			if err != nil {
				resultChan <- chunkResult{nil, fmt.Errorf("failed fetching data from ODOO MS API for chunk %d: %v", chunkIndex+1, err), chunkIndex}
				return
			}

			ODOOResponseArray, ok := ODOOresponse.([]interface{})
			if !ok {
				resultChan <- chunkResult{nil, fmt.Errorf("type assertion failed for chunk %d", chunkIndex+1), chunkIndex}
				return
			}

			resultChan <- chunkResult{ODOOResponseArray, nil, chunkIndex}
		}(i, chunk)
	}

	// Collect results from all goroutines with timeout protection
	results := make([]chunkResult, len(chunks))
	for i := 0; i < len(chunks); i++ {
		select {
		case result := <-resultChan:
			if result.index < len(results) {
				results[result.index] = result
			}
		case <-ctx.Done():
			logrus.Errorf("Timeout waiting for chunk results")
			return errors.New("timeout waiting for chunk results")
		}
	}

	// Process results in order and handle errors gracefully
	for i, result := range results {
		if result.err != nil {
			logrus.Errorf("Error processing chunk %d: %v", i, result.err)
			continue // Continue with other chunks instead of failing completely
		}
		if result.records != nil {
			// logrus.Debugf("Appending %d records from chunk %d", len(result.records), i)
			allRecords = append(allRecords, result.records...)
		}
	}

	if len(allRecords) == 0 {
		logrus.Infof("%s: no active job orders found for technicians joined after %v, skipping further processing", taskDoing, dateJoinedStart.Format("2006-01-02"))
		return nil
	}

	ODOOResponseBytes, err = json.Marshal(allRecords)
	if err != nil {
		return fmt.Errorf("failed to marshal combined response: %v", err)
	}

	// Pre-allocate slice with estimated capacity to reduce memory allocations
	var listOfData []OdooTaskDataRequestItem
	estimatedCapacity := len(allRecords) * 10 // Reduced from 50 to prevent over-allocation
	if estimatedCapacity > 50000 {            // Cap maximum pre-allocation
		estimatedCapacity = 50000
	}
	if estimatedCapacity > 0 {
		listOfData = make([]OdooTaskDataRequestItem, 0, estimatedCapacity)
	}

	if err := json.Unmarshal(ODOOResponseBytes, &listOfData); err != nil {
		errMsg := fmt.Sprintf("failed to unmarshal response body: %v", err)
		return errors.New(errMsg)
	}

	// Log memory usage for monitoring
	var memStats runtime.MemStats
	runtime.ReadMemStats(&memStats)
	// logrus.Infof("Memory usage before DB operations - Allocated: %d MB, System: %d MB",
	// 	memStats.Alloc/1024/1024, memStats.Sys/1024/1024)

	// Force garbage collection to free up memory before database operations
	runtime.GC()

	if err := clearDataOldTechnicianForNewListDataOfKontrakTeknisi(forProject); err != nil {
		logrus.Errorf("failed to clear old data technician %v", err)
	}

	// Remove any duplicate records (keep the one with is_contract_sent = true if exists)
	if err := removeDuplicateTechnicianRecords(forProject); err != nil {
		logrus.Errorf("failed to remove duplicate technician records: %v", err)
	}

	// Group data by technician and create aggregated records
	groupedData := groupDataByTechnicianForContract(forProject, listOfData)

	// Use a single transaction for all database operations to improve performance
	tx := dbWeb.Begin()
	if tx.Error != nil {
		return fmt.Errorf("failed to begin transaction: %v", tx.Error)
	}

	// Process grouped data in batches
	const dbBatchSize = 1000
	var batch []contracttechnicianmodel.ContractTechnicianODOO
	batchCount := 0

	for _, record := range groupedData {
		// Create the preview contract file before append to batch
		technicianName := record.Technician
		if strings.Contains(technicianName, "*") {
			technicianName = strings.ReplaceAll(technicianName, "*", "(Resigned)")
		}

		selectedMainDir, err := fun.FindValidDirectory([]string{
			"web/file/contract_technician",
			"../web/file/contract_technician",
			"../../web/file/contract_technician",
		})
		if err != nil {
			logrus.Errorf("Failed to find valid directory for contract technician files: %v", err)
		}
		pdfFileDir := filepath.Join(selectedMainDir, time.Now().Format("2006-01-02"))
		if err := os.MkdirAll(pdfFileDir, 0755); err != nil {
			logrus.Errorf("Failed to create directory %s: %v", pdfFileDir, err)
		}
		pdfFileName := fmt.Sprintf("[Preview]Surat_Kontrak_%s.pdf", strings.ReplaceAll(technicianName, " ", "_"))
		pdfFilePath := filepath.Join(pdfFileDir, pdfFileName)
		err = GeneratePDFContractTechnician(&record, pdfFilePath)
		if err != nil {
			logrus.Errorf("Failed to generate PDF contract for technician %s: %v", record.Technician, err)
		}

		batch = append(batch, record)
		batchCount++

		// Insert batch when it reaches the batch size or at the end
		if len(batch) >= dbBatchSize || batchCount == len(groupedData) {
			// Process each record in the batch
			for _, rec := range batch {
				// Use a more robust query that locks the row
				var existingRecord contracttechnicianmodel.ContractTechnicianODOO
				err := tx.Where("technician = ? AND for_project = ?",
					rec.Technician, rec.ForProject).
					Order("id ASC"). // Get the first record if there are duplicates
					First(&existingRecord).Error

				if err == nil {
					// Record exists - check if contract was already sent
					if existingRecord.IsContractSent {
						// Contract already sent, update only non-critical fields
						// DO NOT update is_contract_sent, contract_file_path, or contract_send_at
						updateData := map[string]interface{}{
							"wo_number":                    rec.WONumber,
							"ticket_subject":               rec.TicketSubject,
							"wo_number_already_visit":      rec.WONumberAlreadyVisit,
							"ticket_subject_already_visit": rec.TicketSubjectAlreadyVisit,
							"last_login":                   rec.LastLogin,
							"last_download_jo":             rec.LastDownloadJO,
							"first_upload_jo":              rec.FirstUploadJO,
							"last_visit":                   rec.LastVisit,
							"email":                        rec.Email,
							"phone":                        rec.Phone,
							"spl":                          rec.SPL,
							"sac":                          rec.SAC,
						}

						if err := tx.Model(&existingRecord).Updates(updateData).Error; err != nil {
							logrus.Errorf("Failed to update existing sent record for technician %s: %v", rec.Technician, err)
						}
					} else {
						// Contract not sent yet, update all fields BUT preserve ID and contract status
						rec.ID = existingRecord.ID
						rec.IsContractSent = existingRecord.IsContractSent
						rec.ContractFilePath = existingRecord.ContractFilePath
						rec.ContractSendAt = existingRecord.ContractSendAt

						if err := tx.Model(&existingRecord).Updates(&rec).Error; err != nil {
							logrus.Errorf("Failed to update existing unsent record for technician %s: %v", rec.Technician, err)
						}
					}

					// Clean up any duplicate records for this technician (keep only the one we just updated)
					tx.Where("technician = ? AND for_project = ? AND id != ?",
						rec.Technician, rec.ForProject, existingRecord.ID).
						Unscoped().
						Delete(&contracttechnicianmodel.ContractTechnicianODOO{})

				} else if errors.Is(err, gorm.ErrRecordNotFound) {
					// No record exists, check once more outside transaction to avoid race condition
					var checkRecord contracttechnicianmodel.ContractTechnicianODOO
					checkErr := dbWeb.Where("technician = ? AND for_project = ?",
						rec.Technician, rec.ForProject).
						First(&checkRecord).Error

					if checkErr == nil {
						// Record exists outside transaction, skip insertion
						logrus.Warnf("Skipping duplicate insertion for technician %s (found outside tx)", rec.Technician)
						continue
					}

					// No record exists, insert new one (ID will be auto-generated)
					rec.ID = 0 // Ensure ID is 0 for auto-increment
					if err := tx.Create(&rec).Error; err != nil {
						// Check if it's a duplicate key error, if so just log and continue
						if strings.Contains(err.Error(), "Duplicate entry") || strings.Contains(err.Error(), "duplicate key") {
							logrus.Warnf("Duplicate entry detected for technician %s, skipping", rec.Technician)
							continue
						}
						if rollbackErr := tx.Rollback().Error; rollbackErr != nil {
							logrus.Errorf("Failed to rollback transaction: %v", rollbackErr)
						}
						return fmt.Errorf("failed to insert record for technician %s: %v", rec.Technician, err)
					}
				} else {
					// Other database error
					logrus.Errorf("Error checking existing record for technician %s: %v", rec.Technician, err)
				}
			} // Log progress
			// logrus.Infof("Progress: processed %d/%d technician records", batchCount, len(groupedData))

			// Reset batch
			batch = make([]contracttechnicianmodel.ContractTechnicianODOO, 0, dbBatchSize)
		}
	}

	// Commit the transaction
	if err := tx.Commit().Error; err != nil {
		if rollbackErr := tx.Rollback().Error; rollbackErr != nil {
			logrus.Errorf("Failed to rollback transaction after commit failure: %v", rollbackErr)
		}
		return fmt.Errorf("failed to commit transaction: %v", err)
	}

	// Update technician last visit times from the batch data
	if err := updateTechnicianLastVisitFromBatchContract(forProject, listOfData); err != nil {
		logrus.Errorf("Failed to update technician last visit times: %v", err)
		// Don't return error as the main data insertion was successful
	}

	// Update technician first uploaded times from the batch data
	if err := updateTechnicianFirstUploadFromBatchContract(forProject, listOfData); err != nil {
		logrus.Errorf("Failed to update technician first upload times: %v", err)
		// Don't return error as the main data insertion was successful
	}

	/*
		Manage Service - ATM
	*/
	// TODO: implement the logic for contract technician MS ATM if needed !!

	totalDuration := time.Since(startTime)
	logrus.Infof("%s completed in %v", taskDoing, totalDuration)

	return nil
}

func clearDataOldTechnicianForNewListDataOfKontrakTeknisi(forProject string) error {
	// Delete only unsent records for the project to prevent duplicate key errors on refresh
	// Keep records where contract has already been sent (is_contract_sent = true)
	result := dbWeb.
		Unscoped().
		Where("for_project = ?", forProject).
		Where("is_contract_sent = ? OR is_contract_sent IS NULL", false).
		Delete(&contracttechnicianmodel.ContractTechnicianODOO{})

	if result.Error != nil {
		return fmt.Errorf("failed to clear old technician data: %v", result.Error)
	}

	logrus.Infof("Cleared %d unsent technician records for kontrak teknisi (project: %s), keeping sent contracts", result.RowsAffected, forProject)
	return nil
}

func removeDuplicateTechnicianRecords(forProject string) error {
	// Find all technicians that have duplicate records
	var duplicates []struct {
		Technician string
		Count      int64
	}

	err := dbWeb.Model(&contracttechnicianmodel.ContractTechnicianODOO{}).
		Select("technician, COUNT(*) as count").
		Where("for_project = ?", forProject).
		Group("technician").
		Having("COUNT(*) > 1").
		Scan(&duplicates).Error

	if err != nil {
		return fmt.Errorf("failed to find duplicate technicians: %v", err)
	}

	if len(duplicates) == 0 {
		logrus.Infof("No duplicate technician records found for project: %s", forProject)
		return nil
	}

	logrus.Infof("Found %d technicians with duplicate records, cleaning up...", len(duplicates))

	// For each technician with duplicates, keep only one record (prefer the sent one)
	for _, dup := range duplicates {
		var records []contracttechnicianmodel.ContractTechnicianODOO
		err := dbWeb.Where("technician = ? AND for_project = ?", dup.Technician, forProject).
			Order("is_contract_sent DESC, id ASC"). // Prioritize sent records, then oldest ID
			Find(&records).Error

		if err != nil {
			logrus.Errorf("Failed to fetch duplicate records for technician %s: %v", dup.Technician, err)
			continue
		}

		if len(records) <= 1 {
			continue
		}

		// Keep the first record (highest priority), delete the rest
		keepRecord := records[0]
		var idsToDelete []uint
		for i := 1; i < len(records); i++ {
			idsToDelete = append(idsToDelete, records[i].ID)
		}

		if len(idsToDelete) > 0 {
			err := dbWeb.Unscoped().Delete(&contracttechnicianmodel.ContractTechnicianODOO{}, idsToDelete).Error
			if err != nil {
				logrus.Errorf("Failed to delete duplicate records for technician %s: %v", dup.Technician, err)
			} else {
				logrus.Infof("Removed %d duplicate records for technician %s (kept ID: %d, sent: %v)",
					len(idsToDelete), dup.Technician, keepRecord.ID, keepRecord.IsContractSent)
			}
		}
	}

	return nil
}
