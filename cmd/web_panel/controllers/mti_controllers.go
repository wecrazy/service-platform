package controllers

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"net/http"
	"reflect"
	"runtime"
	"service-platform/cmd/web_panel/fun"
	"service-platform/cmd/web_panel/internal/gormdb"
	"service-platform/cmd/web_panel/model"
	mtimodel "service-platform/cmd/web_panel/model/mti_model"
	"service-platform/internal/config"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/sirupsen/logrus"
	"github.com/xuri/excelize/v2"
	"gorm.io/gorm"
)

// Cache for province lookup to avoid repeated database queries
var (
	getDataTaskODOOMSMTIMutex sync.Mutex
	regionCacheMTI            map[string]string
	regionCacheMutex          sync.RWMutex
	regionCacheInitialized    bool
)

// Initialize region cache once
func initRegionCacheMTI() {
	regionCacheMutex.Lock()
	defer regionCacheMutex.Unlock()

	if regionCacheInitialized {
		return
	}

	logrus.Info("Initializing MTI region cache...")
	var allRegions []model.IndonesiaRegion
	dbWeb := gormdb.Databases.Web
	dbWeb.Select("province, district, subdistrict, area, post_code").Find(&allRegions)

	regionCacheMTI = make(map[string]string, 1000)
	for _, region := range allRegions {
		province := region.Province
		// Index by district
		if region.District != "" {
			regionCacheMTI[strings.ToLower(region.District)] = province
		}
		// Index by subdistrict
		if region.Subdistrict != "" {
			regionCacheMTI[strings.ToLower(region.Subdistrict)] = province
		}
		// Index by area
		if region.Area != "" {
			regionCacheMTI[strings.ToLower(region.Area)] = province
		}
		// Index by post code
		if region.PostCode != "" {
			regionCacheMTI[strings.ToLower(region.PostCode)] = province
		}
	}
	regionCacheInitialized = true
	logrus.Infof("MTI region cache initialized with %d entries", len(regionCacheMTI))
}

// Get province from city using cache
func getProvinceFromCityMTI(city string) string {
	if !regionCacheInitialized {
		initRegionCacheMTI()
	}

	regionCacheMutex.RLock()
	defer regionCacheMutex.RUnlock()

	if city == "" {
		return "N/A"
	}

	cityLower := strings.ToLower(city)
	if province, exists := regionCacheMTI[cityLower]; exists {
		return province
	}

	// Try partial matching for common abbreviations
	completedCity := config.WebPanel.Get().Indonesia.CompletedCity

	if fullCity, exists := completedCity[cityLower]; exists {
		if province, exists := regionCacheMTI[strings.ToLower(fullCity)]; exists {
			return province
		}
	}

	return "N/A"
}

// Get SAC from technician name using TechODOOMSData
func getSACFromTechnicianMTI(technicianName string) string {
	if technicianName == "" {
		return "N/A"
	}

	if techData, exists := TechODOOMSData[technicianName]; exists {
		if techData.SAC != "" {
			return techData.SAC
		}
	}

	return "N/A"
}

func RefreshTaskODOOMSMTI() gin.HandlerFunc {
	return func(c *gin.Context) {
		err := GetTaskODOOMSMTI()
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, gin.H{
			"message": fmt.Sprintf("Data successfully refreshed at %s", time.Now().Format(fun.T_YYYYMMDD_HHmmss)),
		})
	}
}

func GetTaskODOOMSMTI() error {
	taskDoing := "Get MTI Task Data from ODOO MS"
	startTime := time.Now()

	if !getDataTaskODOOMSMTIMutex.TryLock() {
		return fmt.Errorf("%s is still running, please wait until the previous process is finished", taskDoing)
	}
	defer getDataTaskODOOMSMTIMutex.Unlock()

	logrus.Infof("Starting %s at %v", taskDoing, startTime)

	ODOOModel := "project.task"
	domain := []interface{}{
		[]interface{}{"company_id", "=", config.WebPanel.Get().MTI.CompanyIDInODOOMS},
	}
	if config.WebPanel.Get().MTI.ActiveDebug {
		startParam := config.WebPanel.Get().MTI.StartParam
		endParam := config.WebPanel.Get().MTI.EndParam
		if startParam != "" && endParam != "" {
			domain = append(domain, []interface{}{"x_received_datetime_spk", ">=", startParam})
			domain = append(domain, []interface{}{"x_received_datetime_spk", "<=", endParam})
		}
	} else {
		// Get current year from January 1st until date now
		loc, _ := time.LoadLocation(config.WebPanel.Get().Default.Timezone)
		now := time.Now().In(loc)
		startOfYear := time.Date(now.Year(), 1, 1, 0, 0, 0, 0, loc)

		startParam := startOfYear.Format("2006-01-02 15:04:05")
		endParam := now.Format("2006-01-02 15:04:05")

		domain = append(domain, []interface{}{"x_received_datetime_spk", ">=", startParam})
		domain = append(domain, []interface{}{"x_received_datetime_spk", "<=", endParam})
	}

	fieldID := []string{"id"}
	fields := []string{
		"id",
		"x_no_task",
		"x_merchant",
		"x_pic_merchant",
		"x_pic_phone",
		"partner_street",
		"x_title_cimb",
		"x_task_type",
		"x_cimb_master_mid",
		"x_cimb_master_tid",
		"x_source",
		"x_message_call",
		"x_status_merchant",
		"x_wo_remark",
		"x_longitude",
		"x_latitude",
		"x_link_photo",
		"x_ticket_type2",
		"worksheet_template_id",
		"company_id",
		"stage_id",
		"helpdesk_ticket_id",
		"x_studio_edc",
		"x_product",
		"technician_id",
		"x_reason_code_id",
		"write_uid",
		"x_sla_deadline",
		"create_date",
		"x_received_datetime_spk",
		"planned_date_begin",
		"timesheet_timer_last_stop",
		"date_last_stage_update",
		"x_studio_kota",
		"partner_zip",
	}
	order := "id desc"
	ODOOParams := map[string]interface{}{
		"model":  ODOOModel,
		"domain": domain,
		"fields": fieldID,
		"order":  order,
	}
	payload := map[string]interface{}{
		"jsonrpc": config.WebPanel.Get().ApiODOO.JSONRPC,
		"params":  ODOOParams,
	}
	payloadBytes, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("failed to marshal payload: %v", err)
	}

	ODOOResponse, err := GetODOOMSData(string(payloadBytes))
	if err != nil {
		return fmt.Errorf("failed to get data from ODOO MS: %v", err)
	}

	ODOOResponseArray, ok := ODOOResponse.([]interface{})
	if !ok {
		return errors.New("invalid response format from ODOOMS")
	}

	ids := extractUniqueIDs(ODOOResponseArray)
	if len(ids) == 0 {
		return errors.New("no MTI task IDs found in ODOO MS response")
	}

	logrus.Infof("Got MTI Task %d IDs", len(ids))

	const batchSize = 1000
	chunks := chunkIdsSlice(ids, batchSize)

	logrus.Debugf("Starting concurrent processing of %d chunks with batch size %d", len(chunks), batchSize)

	// Stream processing: process chunks as they complete and insert immediately
	type chunkResult struct {
		records []OdooTaskDataRequestItem
		err     error
		index   int
	}

	resultChan := make(chan chunkResult, len(chunks))
	var wg sync.WaitGroup
	semaphore := make(chan struct{}, 5) // Limit concurrent API calls

	// Process chunks with timeout protection
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Minute)
	defer cancel()

	logrus.Debugf("Processing %d chunks concurrently with %d workers", len(chunks), cap(semaphore))

	for i, chunk := range chunks {
		wg.Add(1)
		go func(chunkIndex int, chunkIDs []uint64) {
			defer wg.Done()
			defer func() {
				if r := recover(); r != nil {
					logrus.Errorf("Panic in chunk processing %d: %v", chunkIndex, r)
					resultChan <- chunkResult{nil, fmt.Errorf("panic in chunk %d: %v", chunkIndex, r), chunkIndex}
				}
			}()

			select {
			case semaphore <- struct{}{}:
			case <-ctx.Done():
				resultChan <- chunkResult{nil, fmt.Errorf("chunk %d timeout", chunkIndex), chunkIndex}
				return
			}
			defer func() { <-semaphore }()

			startTime := time.Now()
			logrus.Debugf("Processing (%s) chunk %d of %d", taskDoing, chunkIndex+1, len(chunks))

			chunkDomain := []interface{}{
				[]interface{}{"id", "=", chunkIDs},
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

			// Convert directly to struct without intermediate JSON
			var records []OdooTaskDataRequestItem
			for _, item := range ODOOResponseArray {
				var record OdooTaskDataRequestItem
				itemBytes, _ := json.Marshal(item)
				if err := json.Unmarshal(itemBytes, &record); err != nil {
					logrus.Warnf("Failed to unmarshal record in chunk %d: %v", chunkIndex, err)
					continue
				}
				records = append(records, record)
			}

			processingTime := time.Since(startTime)
			logrus.Debugf("Chunk %d processed successfully in %v, retrieved %d records", chunkIndex+1, processingTime, len(records))
			resultChan <- chunkResult{records, nil, chunkIndex}
		}(i, chunk)
	}

	// Close result channel when all goroutines are done
	go func() {
		wg.Wait()
		close(resultChan)
	}()

	// Clear existing records before processing
	dbWeb := gormdb.Databases.Web
	result := dbWeb.Unscoped().Where("1=1").Delete(&mtimodel.MTIOdooMSData{})
	if result.Error != nil {
		return fmt.Errorf("failed to clear existing MTI ODOO MS data: %v", result.Error)
	}
	logrus.Infof("Cleared %d existing MTI ODOO MS records", result.RowsAffected)

	// Stream processing: process and insert chunks as they complete
	const dbBatchSize = 500
	var totalRecords int64
	var insertWg sync.WaitGroup
	insertSemaphore := make(chan struct{}, 3)    // Allow 3 concurrent database operations
	errorChan := make(chan error, len(chunks)*2) // Buffer for errors

	// Process results as they come in (streaming)
	for result := range resultChan {
		if result.err != nil {
			logrus.Errorf("Error processing chunk %d: %v", result.index, result.err)
			continue
		}

		if len(result.records) == 0 {
			continue
		}

		// Process this chunk immediately
		insertWg.Add(1)
		go func(chunkData []OdooTaskDataRequestItem, chunkIndex int) {
			defer insertWg.Done()

			select {
			case insertSemaphore <- struct{}{}:
			case <-ctx.Done():
				errorChan <- fmt.Errorf("chunk %d db insert timeout", chunkIndex)
				return
			}
			defer func() { <-insertSemaphore }()

			// Process in sub-batches for database insertion
			for i := 0; i < len(chunkData); i += dbBatchSize {
				end := i + dbBatchSize
				if end > len(chunkData) {
					end = len(chunkData)
				}

				var batch []mtimodel.MTIOdooMSData
				for _, data := range chunkData[i:end] {
					_, technicianName, err := parseJSONIDDataCombined(data.TechnicianId)
					if err != nil {
						logrus.Warnf("Failed to parse technician_id for ticket ID %d: %v", data.ID, err)
					}

					_, stage, err := parseJSONIDDataCombined(data.StageId)
					if err != nil {
						logrus.Error(err)
					}

					_, ticketType, err := parseJSONIDDataCombined(data.TicketTypeId)
					if err != nil {
						logrus.Warnf("Failed to parse ticket_type_id for ticket ID %d: %v", data.ID, err)
					}

					_, worksheetTemplate, err := parseJSONIDDataCombined(data.WorksheetTemplateId)
					if err != nil {
						logrus.Warnf("Failed to parse worksheet_template_id for ticket ID %d: %v", data.ID, err)
					}

					_, helpdeskTicket, err := parseJSONIDDataCombined(data.HelpdeskTicketId)
					if err != nil {
						logrus.Warnf("Failed to parse helpdesk_ticket_id for ticket ID %d: %v", data.ID, err)
					}
					cleanedTicketSubject := CleanSPKNumber(helpdeskTicket)

					_, snEdc, err := parseJSONIDDataCombined(data.SnEdc)
					if err != nil {
						logrus.Warnf("Failed to parse sn_edc for ticket ID %d: %v", data.ID, err)
					}

					_, edcType, err := parseJSONIDDataCombined(data.EdcType)
					if err != nil {
						logrus.Warnf("Failed to parse edc_type for ticket ID %d: %v", data.ID, err)
					}

					_, reasonCode, err := parseJSONIDDataCombined(data.ReasonCodeId)
					if err != nil {
						logrus.Warnf("Failed to parse reason_code_id for ticket ID %d: %v", data.ID, err)
					}

					// Handle time fields
					var slaDeadline, createDate, receivedDatetimeSpk, planDate, timesheetLastStop, dateLastStageUpdate *time.Time
					if data.SlaDeadline.Valid {
						slaDeadline = &data.SlaDeadline.Time
					}
					if data.CreateDate.Valid {
						createDate = &data.CreateDate.Time
					}
					if data.ReceivedDatetimeSpk.Valid {
						receivedDatetimeSpk = &data.ReceivedDatetimeSpk.Time
					}
					if data.PlanDate.Valid {
						planDate = &data.PlanDate.Time
					}
					if data.TimesheetLastStop.Valid {
						timesheetLastStop = &data.TimesheetLastStop.Time
					}
					if data.DateLastStageUpdate.Valid {
						dateLastStageUpdate = &data.DateLastStageUpdate.Time
					}

					batch = append(batch, mtimodel.MTIOdooMSData{
						ID:                  uint(data.ID),
						WONumber:            data.WoNumber,
						Technician:          technicianName,
						Stage:               stage,
						TaskType:            data.TaskType.String,
						MerchantName:        data.MerchantName.String,
						PicMerchant:         data.PicMerchant.String,
						PicPhone:            data.PicPhone.String,
						MerchantAddress:     data.MerchantAddress.String,
						Description:         data.Description.String,
						Mid:                 data.Mid.String,
						Tid:                 data.Tid.String,
						Source:              data.Source.String,
						MessageCC:           data.MessageCC.String,
						StatusMerchant:      data.StatusMerchant.String,
						WoRemarkTiket:       data.WoRemarkTiket.String,
						Longitude:           data.Longitude.String,
						Latitude:            data.Latitude.String,
						LinkPhoto:           data.LinkPhoto.String,
						TicketType:          ticketType,
						WorksheetTemplate:   worksheetTemplate,
						TicketSubject:       cleanedTicketSubject,
						SNEDC:               snEdc,
						EDCType:             edcType,
						ReasonCode:          reasonCode,
						SlaDeadline:         slaDeadline,
						CreateDate:          createDate,
						ReceivedDatetimeSpk: receivedDatetimeSpk,
						PlanDate:            planDate,
						TimesheetLastStop:   timesheetLastStop,
						DateLastStageUpdate: dateLastStageUpdate,
						MerchantCity:        data.MerchantCity.String,
						MerchantZip:         data.MerchantZip.String,
					})
				}

				if len(batch) > 0 {
					if err := dbWeb.Model(&mtimodel.MTIOdooMSData{}).Create(&batch).Error; err != nil {
						errorChan <- fmt.Errorf("failed to insert MTI ODOO MS data batch for chunk %d: %v", chunkIndex, err)
						return
					}
					totalRecords += int64(len(batch))
					logrus.Debugf("Inserted %d records from chunk %d sub-batch", len(batch), chunkIndex)
				}
			}
		}(result.records, result.index)
	}

	// Wait for all database operations to complete
	go func() {
		insertWg.Wait()
		close(errorChan)
	}()

	// Check for any database errors
	for err := range errorChan {
		if err != nil {
			return err
		}
	}

	logrus.Debug("All database batches completed successfully")

	// Log memory usage for monitoring
	var memStats runtime.MemStats
	runtime.ReadMemStats(&memStats)

	logrus.Infof("Inserted %d new MTI ODOO MS records successfully, memory usage: Alloc=%dMB Sys=%dMB NumGC=%d", totalRecords, memStats.Alloc/1024/1024, memStats.Sys/1024/1024, memStats.NumGC)

	totalDuration := time.Since(startTime)
	logrus.Infof("%s completed successfully in %v (avg: %.2f records/sec)", taskDoing, totalDuration, float64(totalRecords)/totalDuration.Seconds())
	// Log final memory usage
	runtime.ReadMemStats(&memStats)
	logrus.Infof("Final Memory Usage: Alloc = %v MiB, TotalAlloc = %v MiB, Sys = %v MiB, NumGC = %v",
		memStats.Alloc/1024/1024, memStats.TotalAlloc/1024/1024, memStats.Sys/1024/1024, memStats.NumGC)

	return nil
}

func TablePMMTI() gin.HandlerFunc {
	return func(c *gin.Context) {
		var request struct {
			Draw       int    `form:"draw"`
			Start      int    `form:"start"`
			Length     int    `form:"length"`
			Search     string `form:"search[value]"`
			SortColumn int    `form:"order[0][column]"`
			SortDir    string `form:"order[0][dir]"`

			No       string `form:"no" json:"no"`
			FullName string `form:"full_name" json:"full_name" gorm:"column:full_name"`
		}

		// Bind form data to request struct
		if err := c.Bind(&request); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}

		dbWeb := gormdb.Databases.Web
		t := reflect.TypeOf(mtimodel.MTIOdooMSData{})

		// Initialize the map
		columnMap := make(map[int]string)

		// Loop through the fields of the struct
		excludedKeys := map[string]bool{
			"":          true,
			"-":         true,
			"location":  true,
			"task_type": true,
		}

		colNum := 0
		for i := 0; i < t.NumField(); i++ {
			field := t.Field(i)
			// Get the JSON key
			jsonKey := field.Tag.Get("json")
			// Skip excluded keys
			if excludedKeys[jsonKey] {
				continue
			}
			columnMap[colNum] = jsonKey
			colNum++
		}

		// Get the column name based on SortColumn value
		sortColumnName := columnMap[request.SortColumn]
		orderString := fmt.Sprintf("%s %s", sortColumnName, request.SortDir)

		// Initial query for filtering
		filteredQuery := dbWeb.Model(&mtimodel.MTIOdooMSData{})

		// Apply filters
		if request.Search != "" {
			for i := 0; i < t.NumField(); i++ {
				dataField := ""
				field := t.Field(i)
				dataType := field.Type.String()
				jsonKey := field.Tag.Get("json")
				gormTag := field.Tag.Get("gorm")

				// Initialize a variable to hold the column key
				columnKey := ""

				// Manually parse the gorm tag to find the column value
				tags := strings.Split(gormTag, ";")
				for _, tag := range tags {
					if strings.HasPrefix(tag, "column:") {
						columnKey = strings.TrimPrefix(tag, "column:")
						break
					}
				}

				if jsonKey == "" || jsonKey == "-" || excludedKeys[jsonKey] {
					if columnKey == "" || columnKey == "-" {
						continue
					} else {
						dataField = columnKey
					}
				} else {
					dataField = jsonKey
				}
				if jsonKey == "" {
					continue
				}
				if dataType != "string" {
					continue
				}

				filteredQuery = filteredQuery.Or("`"+dataField+"` LIKE ?", "%"+request.Search+"%")
			}

		} else {
			for i := 0; i < t.NumField(); i++ {
				field := t.Field(i)
				formKey := field.Tag.Get("json")

				if formKey == "" || formKey == "-" || formKey == "location" {
					continue
				}

				formValue := c.PostForm(formKey)
				if formValue != "" {
					isHandled := false

					if strings.Contains(formValue, " to ") {
						// Attempt to parse date range
						dates := strings.Split(formValue, " to ")
						if len(dates) == 2 {
							from, err1 := time.Parse("02/01/2006", strings.TrimSpace(dates[0]))
							to, err2 := time.Parse("02/01/2006", strings.TrimSpace(dates[1]))
							if err1 == nil && err2 == nil {
								filteredQuery = filteredQuery.Where(
									"DATE(`"+formKey+"`) BETWEEN ? AND ?",
									from.Format("2006-01-02"),
									to.Format("2006-01-02"),
								)
								isHandled = true
							}
						}
					} else {
						// Attempt to parse single date
						if date, err := time.Parse("02/01/2006", formValue); err == nil {
							filteredQuery = filteredQuery.Where(
								"DATE(`"+formKey+"`) = ?",
								date.Format("2006-01-02"),
							)
							isHandled = true
						}
					}

					if !isHandled {
						// Fallback to LIKE if no valid date
						filteredQuery = filteredQuery.Where("`"+formKey+"` LIKE ?", "%"+formValue+"%")
					}
				}
			}
		}

		// Count the total number of records
		var totalRecords int64
		dbWeb.Model(&mtimodel.MTIOdooMSData{}).Where(&mtimodel.MTIOdooMSData{TaskType: "Preventive Maintenance"}).Count(&totalRecords)

		// Count the number of filtered records
		var filteredRecords int64
		filteredQuery.Where(&mtimodel.MTIOdooMSData{TaskType: "Preventive Maintenance"}).Count(&filteredRecords)

		// Apply sorting and pagination to the filtered query
		query := filteredQuery.Order(orderString)
		var Dbdata []mtimodel.MTIOdooMSData
		query = query.Offset(request.Start).Limit(request.Length).Find(&Dbdata)

		if query.Error != nil {
			c.JSON(http.StatusInternalServerError, gin.H{
				"draw":            request.Draw,
				"recordsTotal":    totalRecords,
				"recordsFiltered": 0,
				"data":            []gin.H{},
				"error":           query.Error.Error(),
			})
			return
		}

		var data []gin.H
		for _, dataInDB := range Dbdata {
			newData := make(map[string]interface{})
			v := reflect.ValueOf(dataInDB)

			idTask := dataInDB.ID

			for i := 0; i < t.NumField(); i++ {
				field := t.Field(i)
				fieldValue := v.Field(i)

				// Get the JSON key
				theKey := field.Tag.Get("json")
				if theKey == "" {
					theKey = field.Tag.Get("form")
					if theKey == "" {
						continue
					}
				}

				// Handle data rendered in col
				switch theKey {
				case "birthdate", "date":
					if fieldValue.Type() == reflect.TypeOf(time.Time{}) {
						t := fieldValue.Interface().(time.Time)
						switch theKey {
						case "birthdate":
							newData[theKey] = t.Format(fun.T_YYYYMMDD)
						case "date":
							newData[theKey] = t.Add(7 * time.Hour).Format(fun.T_YYYYMMDD_HHmmss)
						}
					} else {
						newData[theKey] = fieldValue.Interface()
					}

				case "stage":
					if fieldValue.Type() == reflect.TypeOf("") {
						stage := fieldValue.Interface().(string)
						var stageLabel string
						switch stage {
						case "New":
							stageLabel = `<span class="badge bg-secondary">New</span>`
						case "Open Pending":
							stageLabel = `<span class="badge bg-info">Open Pending</span>`
						case "Verified":
							stageLabel = `<span class="badge bg-warning">Verified</span>`
						case "Done":
							stageLabel = `<span class="badge bg-success">Done</span>`
						case "Cancel":
							stageLabel = `<span class="badge bg-danger">Cancelled</span>`
						default:
							stageLabel = fmt.Sprintf(`<span class="badge bg-light text-dark">%s</span>`, stage)
						}
						newData[theKey] = stageLabel
					} else {
						newData[theKey] = fieldValue.Interface()
					}

				case "description":
					if fieldValue.Type() == reflect.TypeOf("") {
						desc := fieldValue.Interface().(string)
						if len(desc) > 50 {
							truncated := desc[:50] + "..."
							fullDesc := strings.ReplaceAll(desc, `"`, `&quot;`) // Escape quotes for HTML
							newData[theKey] = fmt.Sprintf(`<span data-bs-toggle="popover" data-bs-placement="top" data-bs-html="true" data-bs-content="%s <button type='button' class='btn-close ms-2' aria-label='Close' onclick='$(this).closest(&quot;.popover&quot;).popover(&quot;hide&quot;)'></button>">%s</span>`, fullDesc, truncated)
						} else {
							newData[theKey] = desc
						}
					} else {
						newData[theKey] = fieldValue.Interface()
					}

				case "location":
					longitude := ""
					latitude := ""
					if lonField, ok := t.FieldByName("Longitude"); ok {
						lonValue := v.FieldByName(lonField.Name)
						if lonValue.IsValid() && lonValue.Type() == reflect.TypeOf("") {
							longitude = lonValue.Interface().(string)
						}
					}
					if latField, ok := t.FieldByName("Latitude"); ok {
						latValue := v.FieldByName(latField.Name)
						if latValue.IsValid() && latValue.Type() == reflect.TypeOf("") {
							latitude = latValue.Interface().(string)
						}
					}
					if longitude != "" && latitude != "" && longitude != "0" && longitude != "0.0" && latitude != "0" && latitude != "0.0" {
						mapLink := fmt.Sprintf("https://www.google.com/maps/search/?api=1&query=%s,%s", latitude, longitude)
						newData["location"] = fmt.Sprintf(`<button onclick="openMapWindow('%s')" class="btn btn-md btn-label-primary"><i class="fal fa-map-marker-alt me-2"></i>View Map</button>`, mapLink)
					} else {
						newData["location"] = "<span class='text-muted text-danger'>N/A</span>"
					}

				case "sla_deadline", "create_date", "received_datetime_spk", "plan_date", "timesheet_last_stop", "date_last_stage_update":
					if fieldValue.Type() == reflect.TypeOf(&time.Time{}) {
						if !fieldValue.IsNil() {
							t := fieldValue.Elem().Interface().(time.Time)
							newData[theKey] = t.Format("02 January 2006 15:04:05")
						} else {
							newData[theKey] = nil
						}
					} else {
						newData[theKey] = fieldValue.Interface()
					}

				case "link_photo":
					newData[theKey] =
						fmt.Sprintf(
							`
							<div class="card-cek">
								<button class="btn btn-md btn-label-info" onclick="openPopupODOOMSPhotos('%d', 'task-mti')">
									<i class='fal fa-images me-2'></i> View Photos
								</button>
							</div>
							`, idTask,
						)

				default:
					if fieldValue.Type() == reflect.TypeOf(time.Time{}) {
						t := fieldValue.Interface().(time.Time)
						newData[theKey] = t.Format(fun.T_YYYYMMDD_HHmmss)
					} else {
						newData[theKey] = fieldValue.Interface()
					}
				}

			}

			data = append(data, gin.H(newData))
		}

		// Respond with the formatted data for DataTables
		c.JSON(http.StatusOK, gin.H{
			"draw":            request.Draw,
			"recordsTotal":    totalRecords,
			"recordsFiltered": filteredRecords,
			"data":            data,
		})
	}
}

func TableNonPMMTI() gin.HandlerFunc {
	return func(c *gin.Context) {
		var request struct {
			Draw       int    `form:"draw"`
			Start      int    `form:"start"`
			Length     int    `form:"length"`
			Search     string `form:"search[value]"`
			SortColumn int    `form:"order[0][column]"`
			SortDir    string `form:"order[0][dir]"`

			No       string `form:"no" json:"no"`
			FullName string `form:"full_name" json:"full_name" gorm:"column:full_name"`
		}

		// Bind form data to request struct
		if err := c.Bind(&request); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}

		dbWeb := gormdb.Databases.Web
		t := reflect.TypeOf(mtimodel.MTIOdooMSData{})

		// Initialize the map
		columnMap := make(map[int]string)

		// Loop through the fields of the struct
		excludedKeys := map[string]bool{
			"":         true,
			"-":        true,
			"location": true,
			// "task_type": true,
		}

		colNum := 0
		for i := 0; i < t.NumField(); i++ {
			field := t.Field(i)
			// Get the JSON key
			jsonKey := field.Tag.Get("json")
			// Skip excluded keys
			if excludedKeys[jsonKey] {
				continue
			}
			columnMap[colNum] = jsonKey
			colNum++
		}

		// Get the column name based on SortColumn value
		sortColumnName := columnMap[request.SortColumn]
		orderString := fmt.Sprintf("%s %s", sortColumnName, request.SortDir)

		// Initial query for filtering
		filteredQuery := dbWeb.Model(&mtimodel.MTIOdooMSData{})

		// Apply filters
		if request.Search != "" {
			for i := 0; i < t.NumField(); i++ {
				dataField := ""
				field := t.Field(i)
				dataType := field.Type.String()
				jsonKey := field.Tag.Get("json")
				gormTag := field.Tag.Get("gorm")

				// Initialize a variable to hold the column key
				columnKey := ""

				// Manually parse the gorm tag to find the column value
				tags := strings.Split(gormTag, ";")
				for _, tag := range tags {
					if strings.HasPrefix(tag, "column:") {
						columnKey = strings.TrimPrefix(tag, "column:")
						break
					}
				}

				if jsonKey == "" || jsonKey == "-" || excludedKeys[jsonKey] {
					if columnKey == "" || columnKey == "-" {
						continue
					} else {
						dataField = columnKey
					}
				} else {
					dataField = jsonKey
				}
				if jsonKey == "" {
					continue
				}
				if dataType != "string" {
					continue
				}

				filteredQuery = filteredQuery.Or("`"+dataField+"` LIKE ?", "%"+request.Search+"%")
			}

		} else {
			for i := 0; i < t.NumField(); i++ {
				field := t.Field(i)
				formKey := field.Tag.Get("json")

				if formKey == "" || formKey == "-" || formKey == "location" {
					continue
				}

				formValue := c.PostForm(formKey)
				if formValue != "" {
					isHandled := false

					if strings.Contains(formValue, " to ") {
						// Attempt to parse date range
						dates := strings.Split(formValue, " to ")
						if len(dates) == 2 {
							from, err1 := time.Parse("02/01/2006", strings.TrimSpace(dates[0]))
							to, err2 := time.Parse("02/01/2006", strings.TrimSpace(dates[1]))
							if err1 == nil && err2 == nil {
								filteredQuery = filteredQuery.Where(
									"DATE(`"+formKey+"`) BETWEEN ? AND ?",
									from.Format("2006-01-02"),
									to.Format("2006-01-02"),
								)
								isHandled = true
							}
						}
					} else {
						// Attempt to parse single date
						if date, err := time.Parse("02/01/2006", formValue); err == nil {
							filteredQuery = filteredQuery.Where(
								"DATE(`"+formKey+"`) = ?",
								date.Format("2006-01-02"),
							)
							isHandled = true
						}
					}

					if !isHandled {
						// Fallback to LIKE if no valid date
						filteredQuery = filteredQuery.Where("`"+formKey+"` LIKE ?", "%"+formValue+"%")
					}
				}
			}
		}

		// Count the total number of records
		var totalRecords int64
		dbWeb.Model(&mtimodel.MTIOdooMSData{}).Where("task_type != ?", "Preventive Maintenance").Count(&totalRecords)

		// Count the number of filtered records
		var filteredRecords int64
		filteredQuery.Where("task_type != ?", "Preventive Maintenance").Count(&filteredRecords)

		// Apply sorting and pagination to the filtered query
		query := filteredQuery.Order(orderString)
		var Dbdata []mtimodel.MTIOdooMSData
		query = query.Offset(request.Start).Limit(request.Length).Find(&Dbdata)

		if query.Error != nil {
			c.JSON(http.StatusInternalServerError, gin.H{
				"draw":            request.Draw,
				"recordsTotal":    totalRecords,
				"recordsFiltered": 0,
				"data":            []gin.H{},
				"error":           query.Error.Error(),
			})
			return
		}

		var data []gin.H
		for _, dataInDB := range Dbdata {
			newData := make(map[string]interface{})
			v := reflect.ValueOf(dataInDB)

			idTask := dataInDB.ID

			for i := 0; i < t.NumField(); i++ {
				field := t.Field(i)
				fieldValue := v.Field(i)

				// Get the JSON key
				theKey := field.Tag.Get("json")
				if theKey == "" {
					theKey = field.Tag.Get("form")
					if theKey == "" {
						continue
					}
				}

				// Handle data rendered in col
				switch theKey {
				case "birthdate", "date":
					if fieldValue.Type() == reflect.TypeOf(time.Time{}) {
						t := fieldValue.Interface().(time.Time)
						switch theKey {
						case "birthdate":
							newData[theKey] = t.Format(fun.T_YYYYMMDD)
						case "date":
							newData[theKey] = t.Add(7 * time.Hour).Format(fun.T_YYYYMMDD_HHmmss)
						}
					} else {
						newData[theKey] = fieldValue.Interface()
					}

				case "task_type":
					if fieldValue.Type() == reflect.TypeOf("") {
						taskType := fieldValue.Interface().(string)
						var typeLabel string
						switch taskType {
						case "Preventive Maintenance":
							typeLabel = `<span class="badge bg-primary">Preventive Maintenance</span>`
						case "Corrective Maintenance":
							typeLabel = `<span class="badge bg-warning text-dark">Corrective Maintenance</span>`
						case "Installation":
							typeLabel = `<span class="badge bg-success">Installation</span>`
						case "Withdrawal":
							typeLabel = `<span class="badge bg-danger">Withdrawal</span>`
						case "Re-Init":
							typeLabel = `<span class="badge bg-info text-dark">Re-Init</span>`
						case "Replacement":
							typeLabel = `<span class="badge bg-secondary text-dark">Replacement</span>`
						case "Roll Out":
							typeLabel = `<span class="badge bg-dark">Roll Out</span>`
						case "Deploy":
							typeLabel = `<span class="badge bg-label-dark">Deploy</span>`
						default:
							typeLabel = fmt.Sprintf(`<span class="badge bg-light text-dark">%s</span>`, taskType)
						}
						newData[theKey] = typeLabel
					} else {
						newData[theKey] = fieldValue.Interface()
					}

				case "stage":
					if fieldValue.Type() == reflect.TypeOf("") {
						stage := fieldValue.Interface().(string)
						var stageLabel string
						switch stage {
						case "New":
							stageLabel = `<span class="badge bg-secondary">New</span>`
						case "Open Pending":
							stageLabel = `<span class="badge bg-info">Open Pending</span>`
						case "Verified":
							stageLabel = `<span class="badge bg-warning">Verified</span>`
						case "Done":
							stageLabel = `<span class="badge bg-success">Done</span>`
						case "Cancel":
							stageLabel = `<span class="badge bg-danger">Cancelled</span>`
						default:
							stageLabel = fmt.Sprintf(`<span class="badge bg-light text-dark">%s</span>`, stage)
						}
						newData[theKey] = stageLabel
					} else {
						newData[theKey] = fieldValue.Interface()
					}

				case "description":
					if fieldValue.Type() == reflect.TypeOf("") {
						desc := fieldValue.Interface().(string)
						if len(desc) > 50 {
							truncated := desc[:50] + "..."
							fullDesc := strings.ReplaceAll(desc, `"`, `&quot;`) // Escape quotes for HTML
							newData[theKey] = fmt.Sprintf(`<span data-bs-toggle="popover" data-bs-placement="top" data-bs-html="true" data-bs-content="%s <button type='button' class='btn-close ms-2' aria-label='Close' onclick='$(this).closest(&quot;.popover&quot;).popover(&quot;hide&quot;)'></button>">%s</span>`, fullDesc, truncated)
						} else {
							newData[theKey] = desc
						}
					} else {
						newData[theKey] = fieldValue.Interface()
					}

				case "location":
					longitude := ""
					latitude := ""
					if lonField, ok := t.FieldByName("Longitude"); ok {
						lonValue := v.FieldByName(lonField.Name)
						if lonValue.IsValid() && lonValue.Type() == reflect.TypeOf("") {
							longitude = lonValue.Interface().(string)
						}
					}
					if latField, ok := t.FieldByName("Latitude"); ok {
						latValue := v.FieldByName(latField.Name)
						if latValue.IsValid() && latValue.Type() == reflect.TypeOf("") {
							latitude = latValue.Interface().(string)
						}
					}
					if longitude != "" && latitude != "" && longitude != "0" && longitude != "0.0" && latitude != "0" && latitude != "0.0" {
						mapLink := fmt.Sprintf("https://www.google.com/maps/search/?api=1&query=%s,%s", latitude, longitude)
						newData["location"] = fmt.Sprintf(`<button onclick="openMapWindow('%s')" class="btn btn-md btn-label-primary"><i class="fal fa-map-marker-alt me-2"></i>View Map</button>`, mapLink)
					} else {
						newData["location"] = "<span class='text-muted text-danger'>N/A</span>"
					}

				case "sla_deadline", "create_date", "received_datetime_spk", "plan_date", "timesheet_last_stop", "date_last_stage_update":
					if fieldValue.Type() == reflect.TypeOf(&time.Time{}) {
						if !fieldValue.IsNil() {
							t := fieldValue.Elem().Interface().(time.Time)
							newData[theKey] = t.Format("02 January 2006 15:04:05")
						} else {
							newData[theKey] = nil
						}
					} else {
						newData[theKey] = fieldValue.Interface()
					}

				case "link_photo":
					newData[theKey] =
						fmt.Sprintf(
							`
							<div class="card-cek">
								<button class="btn btn-md btn-label-info" onclick="openPopupODOOMSPhotos('%d', 'task-mti')">
									<i class='fal fa-images me-2'></i> View Photos
								</button>
							</div>
							`, idTask,
						)

				default:
					if fieldValue.Type() == reflect.TypeOf(time.Time{}) {
						t := fieldValue.Interface().(time.Time)
						newData[theKey] = t.Format(fun.T_YYYYMMDD_HHmmss)
					} else {
						newData[theKey] = fieldValue.Interface()
					}
				}

			}

			data = append(data, gin.H(newData))
		}

		// Respond with the formatted data for DataTables
		c.JSON(http.StatusOK, gin.H{
			"draw":            request.Draw,
			"recordsTotal":    totalRecords,
			"recordsFiltered": filteredRecords,
			"data":            data,
		})
	}
}

func PivotPMMTI() gin.HandlerFunc {
	return func(c *gin.Context) {
		var request struct {
			Draw       int    `form:"draw"`
			Start      int    `form:"start"`
			Length     int    `form:"length"`
			Search     string `form:"search[value]"`
			SortColumn int    `form:"order[0][column]"`
			SortDir    string `form:"order[0][dir]"`

			// Add all the same form fields as TablePMMTI
			Stage               string `form:"stage"`
			WONumber            string `form:"wo_number"`
			Technician          string `form:"technician"`
			Mid                 string `form:"mid"`
			Tid                 string `form:"tid"`
			MerchantName        string `form:"merchant_name"`
			PicMerchant         string `form:"pic_merchant"`
			PicPhone            string `form:"pic_phone"`
			MerchantAddress     string `form:"merchant_address"`
			Description         string `form:"description"`
			Source              string `form:"source"`
			MessageCC           string `form:"message_cc"`
			StatusMerchant      string `form:"status_merchant"`
			WoRemarkTiket       string `form:"wo_remark_tiket"`
			Longitude           string `form:"longitude"`
			Latitude            string `form:"latitude"`
			LinkPhoto           string `form:"link_photo"`
			TicketType          string `form:"ticket_type"`
			WorksheetTemplate   string `form:"worksheet_template"`
			TicketSubject       string `form:"ticket_subject"`
			SNEDC               string `form:"sn_edc"`
			EDCType             string `form:"edc_type"`
			ReasonCode          string `form:"reason_code"`
			SlaDeadline         string `form:"sla_deadline"`
			CreateDate          string `form:"create_date"`
			ReceivedDatetimeSpk string `form:"received_datetime_spk"`
			PlanDate            string `form:"plan_date"`
			TimesheetLastStop   string `form:"timesheet_last_stop"`
			DateLastStageUpdate string `form:"date_last_stage_update"`
		}

		// Bind form data to request struct
		if err := c.Bind(&request); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}

		dbWeb := gormdb.Databases.Web

		type PivotResult struct {
			NamaVendorBank             string  `json:"nama_vendor_bank"`
			TerkunjungiCount           int64   `json:"terkunjungi_count"`
			TerkunjungiPercentage      float64 `json:"terkunjungi_percentage"`
			GagalTerkunjungiCount      int64   `json:"gagal_terkunjungi_count"`
			GagalTerkunjungiPercentage float64 `json:"gagal_terkunjungi_percentage"`
			BelumKunjunganCount        int64   `json:"belum_kunjungan_count"`
			BelumKunjunganPercentage   float64 `json:"belum_kunjungan_percentage"`
			TotalCount                 int64   `json:"total_count"`
			RunRate                    int64   `json:"run_rate"`
			RunRatePercentage          float64 `json:"run_rate_percentage"`
			TargetPerHari              int64   `json:"target_per_hari"`
		}

		// Build WHERE clause with filters (same logic as TablePMMTI)
		whereConditions := []string{"task_type = 'Preventive Maintenance'", "deleted_at IS NULL"}
		args := []interface{}{}

		// Apply filters based on form data
		if request.Stage != "" {
			whereConditions = append(whereConditions, "stage LIKE ?")
			args = append(args, "%"+request.Stage+"%")
		}
		if request.WONumber != "" {
			whereConditions = append(whereConditions, "wo_number LIKE ?")
			args = append(args, "%"+request.WONumber+"%")
		}
		if request.Technician != "" {
			whereConditions = append(whereConditions, "technician LIKE ?")
			args = append(args, "%"+request.Technician+"%")
		}
		if request.Mid != "" {
			whereConditions = append(whereConditions, "mid LIKE ?")
			args = append(args, "%"+request.Mid+"%")
		}
		if request.Tid != "" {
			whereConditions = append(whereConditions, "tid LIKE ?")
			args = append(args, "%"+request.Tid+"%")
		}
		if request.MerchantName != "" {
			whereConditions = append(whereConditions, "merchant_name LIKE ?")
			args = append(args, "%"+request.MerchantName+"%")
		}
		if request.PicMerchant != "" {
			whereConditions = append(whereConditions, "pic_merchant LIKE ?")
			args = append(args, "%"+request.PicMerchant+"%")
		}
		if request.PicPhone != "" {
			whereConditions = append(whereConditions, "pic_phone LIKE ?")
			args = append(args, "%"+request.PicPhone+"%")
		}
		if request.MerchantAddress != "" {
			whereConditions = append(whereConditions, "merchant_address LIKE ?")
			args = append(args, "%"+request.MerchantAddress+"%")
		}
		if request.Description != "" {
			whereConditions = append(whereConditions, "description LIKE ?")
			args = append(args, "%"+request.Description+"%")
		}
		if request.Source != "" {
			whereConditions = append(whereConditions, "source LIKE ?")
			args = append(args, "%"+request.Source+"%")
		}
		if request.MessageCC != "" {
			whereConditions = append(whereConditions, "message_cc LIKE ?")
			args = append(args, "%"+request.MessageCC+"%")
		}
		if request.StatusMerchant != "" {
			whereConditions = append(whereConditions, "status_merchant LIKE ?")
			args = append(args, "%"+request.StatusMerchant+"%")
		}
		if request.WoRemarkTiket != "" {
			whereConditions = append(whereConditions, "wo_remark_tiket LIKE ?")
			args = append(args, "%"+request.WoRemarkTiket+"%")
		}
		if request.TicketType != "" {
			whereConditions = append(whereConditions, "ticket_type LIKE ?")
			args = append(args, "%"+request.TicketType+"%")
		}
		if request.WorksheetTemplate != "" {
			whereConditions = append(whereConditions, "worksheet_template LIKE ?")
			args = append(args, "%"+request.WorksheetTemplate+"%")
		}
		if request.TicketSubject != "" {
			whereConditions = append(whereConditions, "ticket_subject LIKE ?")
			args = append(args, "%"+request.TicketSubject+"%")
		}
		if request.SNEDC != "" {
			whereConditions = append(whereConditions, "sn_edc LIKE ?")
			args = append(args, "%"+request.SNEDC+"%")
		}
		if request.EDCType != "" {
			whereConditions = append(whereConditions, "edc_type LIKE ?")
			args = append(args, "%"+request.EDCType+"%")
		}
		if request.ReasonCode != "" {
			whereConditions = append(whereConditions, "reason_code LIKE ?")
			args = append(args, "%"+request.ReasonCode+"%")
		}

		// Build the complete WHERE clause
		whereClause := strings.Join(whereConditions, " AND ")

		// Get aggregated data grouped by source (vendor/bank) with filters applied
		var results []PivotResult

		// Raw SQL query to aggregate the data with dynamic WHERE clause
		query := `
			SELECT 
				COALESCE(NULLIF(source, ''), 'Unknown') as nama_vendor_bank,
				COUNT(DISTINCT CASE WHEN timesheet_last_stop IS NOT NULL AND reason_code LIKE 'A00%' THEN tid END) as terkunjungi_count,
				COUNT(DISTINCT CASE WHEN reason_code IS NOT NULL AND reason_code NOT LIKE 'A00%' THEN tid END) as gagal_terkunjungi_count,
				COUNT(DISTINCT CASE WHEN timesheet_last_stop IS NULL THEN tid END) as belum_kunjungan_count,
				COUNT(DISTINCT tid) as total_count
			FROM ` + config.WebPanel.Get().MTI.TBDataODOOMS + `
			WHERE ` + whereClause + `
			GROUP BY source
			ORDER BY source
		`

		rows, err := dbWeb.Raw(query, args...).Rows()
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		defer rows.Close()

		const targetPerHari = 6000 // Default target, you can make this configurable

		for rows.Next() {
			var result PivotResult
			var source string
			var terkunjungi, gagal, belum, total int64

			err := rows.Scan(&source, &terkunjungi, &gagal, &belum, &total)
			if err != nil {
				continue
			}

			result.NamaVendorBank = source
			result.TerkunjungiCount = terkunjungi
			result.GagalTerkunjungiCount = gagal
			result.BelumKunjunganCount = belum
			result.TotalCount = total
			result.TargetPerHari = targetPerHari

			// Calculate percentages
			if total > 0 {
				result.TerkunjungiPercentage = float64(terkunjungi) / float64(total) * 100
				result.GagalTerkunjungiPercentage = float64(gagal) / float64(total) * 100
				result.BelumKunjunganPercentage = float64(belum) / float64(total) * 100
			}

			// Calculate run rate (completed tasks)
			result.RunRate = terkunjungi + gagal // Tasks that have been processed
			if targetPerHari > 0 {
				result.RunRatePercentage = float64(result.RunRate) / float64(targetPerHari) * 100
			}

			results = append(results, result)
		}

		// Calculate totals for filtering if search is provided
		filteredRecords := int64(len(results))
		totalRecords := filteredRecords

		// Apply search filter if provided
		if request.Search != "" {
			var filteredResults []PivotResult
			searchLower := strings.ToLower(request.Search)
			for _, result := range results {
				if strings.Contains(strings.ToLower(result.NamaVendorBank), searchLower) {
					filteredResults = append(filteredResults, result)
				}
			}
			results = filteredResults
			filteredRecords = int64(len(results))
		}

		// Apply sorting
		if request.SortColumn >= 0 && request.SortColumn < 11 { // We have 11 columns
			// Sort implementation can be added here if needed
		}

		// Apply pagination if length is not -1 (get all)
		if request.Length > 0 && request.Start >= 0 {
			end := request.Start + request.Length
			if end > len(results) {
				end = len(results)
			}
			if request.Start < len(results) {
				results = results[request.Start:end]
			} else {
				results = []PivotResult{}
			}
		}

		// Round percentages to 2 decimal places
		for i := range results {
			results[i].TerkunjungiPercentage = math.Round(results[i].TerkunjungiPercentage*100) / 100
			results[i].GagalTerkunjungiPercentage = math.Round(results[i].GagalTerkunjungiPercentage*100) / 100
			results[i].BelumKunjunganPercentage = math.Round(results[i].BelumKunjunganPercentage*100) / 100
			results[i].RunRatePercentage = math.Round(results[i].RunRatePercentage*100) / 100
		}

		// Respond with the formatted data
		c.JSON(http.StatusOK, gin.H{
			"draw":            request.Draw,
			"recordsTotal":    totalRecords,
			"recordsFiltered": filteredRecords,
			"data":            results,
		})
	}
}

func PivotNonPMMTI() gin.HandlerFunc {
	return func(c *gin.Context) {
		var request struct {
			Draw       int    `form:"draw"`
			Start      int    `form:"start"`
			Length     int    `form:"length"`
			Search     string `form:"search[value]"`
			SortColumn int    `form:"order[0][column]"`
			SortDir    string `form:"order[0][dir]"`

			// Add all the same form fields as TableNonPMMTI
			TaskType            string `form:"task_type"`
			Stage               string `form:"stage"`
			WONumber            string `form:"wo_number"`
			Technician          string `form:"technician"`
			Mid                 string `form:"mid"`
			Tid                 string `form:"tid"`
			MerchantName        string `form:"merchant_name"`
			PicMerchant         string `form:"pic_merchant"`
			PicPhone            string `form:"pic_phone"`
			MerchantAddress     string `form:"merchant_address"`
			Description         string `form:"description"`
			Source              string `form:"source"`
			MessageCC           string `form:"message_cc"`
			StatusMerchant      string `form:"status_merchant"`
			WoRemarkTiket       string `form:"wo_remark_tiket"`
			Longitude           string `form:"longitude"`
			Latitude            string `form:"latitude"`
			LinkPhoto           string `form:"link_photo"`
			TicketType          string `form:"ticket_type"`
			WorksheetTemplate   string `form:"worksheet_template"`
			TicketSubject       string `form:"ticket_subject"`
			SNEDC               string `form:"sn_edc"`
			EDCType             string `form:"edc_type"`
			ReasonCode          string `form:"reason_code"`
			SlaDeadline         string `form:"sla_deadline"`
			CreateDate          string `form:"create_date"`
			ReceivedDatetimeSpk string `form:"received_datetime_spk"`
			PlanDate            string `form:"plan_date"`
			TimesheetLastStop   string `form:"timesheet_last_stop"`
			DateLastStageUpdate string `form:"date_last_stage_update"`
			MerchantCity        string `form:"merchant_city"`
		}

		// Bind form data to request struct
		if err := c.Bind(&request); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}

		dbWeb := gormdb.Databases.Web

		type PivotResult struct {
			Provinsi                           string `json:"provinsi"`
			Kota                               string `json:"kota"`
			SPLeader                           string `json:"sp_leader"`
			Teknisi                            string `json:"teknisi"`
			Activity                           string `json:"activity"`
			Priority1MissSLA                   int64  `json:"priority_1_miss_sla"`
			Priority1MustVisitToday            int64  `json:"priority_1_must_visit_today"`
			Priority2MustVisitTomorrow         int64  `json:"priority_2_must_visit_tomorrow"`
			Priority3MustVisitMoreThanTomorrow int64  `json:"priority_3_must_visit_more_than_tomorrow"`
			PendingMerchantOrBucketBusiness    int64  `json:"pending_merchant_or_bucket_business"`
		}

		// Build WHERE clause with filters (same logic as TableNonPMMTI)
		whereConditions := []string{"task_type != 'Preventive Maintenance'", "deleted_at IS NULL"}
		args := []interface{}{}

		// Apply filters based on form data
		if request.Stage != "" {
			whereConditions = append(whereConditions, "stage LIKE ?")
			args = append(args, "%"+request.Stage+"%")
		}
		if request.WONumber != "" {
			whereConditions = append(whereConditions, "wo_number LIKE ?")
			args = append(args, "%"+request.WONumber+"%")
		}
		if request.Technician != "" {
			whereConditions = append(whereConditions, "technician LIKE ?")
			args = append(args, "%"+request.Technician+"%")
		}
		if request.Mid != "" {
			whereConditions = append(whereConditions, "mid LIKE ?")
			args = append(args, "%"+request.Mid+"%")
		}
		if request.Tid != "" {
			whereConditions = append(whereConditions, "tid LIKE ?")
			args = append(args, "%"+request.Tid+"%")
		}
		if request.MerchantName != "" {
			whereConditions = append(whereConditions, "merchant_name LIKE ?")
			args = append(args, "%"+request.MerchantName+"%")
		}
		if request.PicMerchant != "" {
			whereConditions = append(whereConditions, "pic_merchant LIKE ?")
			args = append(args, "%"+request.PicMerchant+"%")
		}
		if request.PicPhone != "" {
			whereConditions = append(whereConditions, "pic_phone LIKE ?")
			args = append(args, "%"+request.PicPhone+"%")
		}
		if request.MerchantAddress != "" {
			whereConditions = append(whereConditions, "merchant_address LIKE ?")
			args = append(args, "%"+request.MerchantAddress+"%")
		}
		if request.Description != "" {
			whereConditions = append(whereConditions, "description LIKE ?")
			args = append(args, "%"+request.Description+"%")
		}
		if request.Source != "" {
			whereConditions = append(whereConditions, "source LIKE ?")
			args = append(args, "%"+request.Source+"%")
		}
		if request.MessageCC != "" {
			whereConditions = append(whereConditions, "message_cc LIKE ?")
			args = append(args, "%"+request.MessageCC+"%")
		}
		if request.StatusMerchant != "" {
			whereConditions = append(whereConditions, "status_merchant LIKE ?")
			args = append(args, "%"+request.StatusMerchant+"%")
		}
		if request.WoRemarkTiket != "" {
			whereConditions = append(whereConditions, "wo_remark_tiket LIKE ?")
			args = append(args, "%"+request.WoRemarkTiket+"%")
		}
		if request.TicketType != "" {
			whereConditions = append(whereConditions, "ticket_type LIKE ?")
			args = append(args, "%"+request.TicketType+"%")
		}
		if request.WorksheetTemplate != "" {
			whereConditions = append(whereConditions, "worksheet_template LIKE ?")
			args = append(args, "%"+request.WorksheetTemplate+"%")
		}
		if request.TicketSubject != "" {
			whereConditions = append(whereConditions, "ticket_subject LIKE ?")
			args = append(args, "%"+request.TicketSubject+"%")
		}
		if request.SNEDC != "" {
			whereConditions = append(whereConditions, "sn_edc LIKE ?")
			args = append(args, "%"+request.SNEDC+"%")
		}
		if request.EDCType != "" {
			whereConditions = append(whereConditions, "edc_type LIKE ?")
			args = append(args, "%"+request.EDCType+"%")
		}
		if request.ReasonCode != "" {
			whereConditions = append(whereConditions, "reason_code LIKE ?")
			args = append(args, "%"+request.ReasonCode+"%")
		}
		if request.MerchantCity != "" {
			whereConditions = append(whereConditions, "merchant_city LIKE ?")
			args = append(args, "%"+request.MerchantCity+"%")
		}

		// Build the complete WHERE clause
		whereClause := strings.Join(whereConditions, " AND ")

		// Get aggregated data grouped by Provinsi, Kota, SPLeader, Teknisi, Activity with filters applied
		var results []PivotResult

		// Initialize region cache if not already done
		if !regionCacheInitialized {
			initRegionCacheMTI()
		}

		// Get raw data from database with basic filters - avoiding complex aggregation in SQL
		var rawData []mtimodel.MTIOdooMSData
		query := dbWeb.Where(whereClause, args...)

		if err := query.Find(&rawData).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}

		logrus.Debugf("PivotNonPMMTI: Found %d raw records to process", len(rawData))

		// Process data in Go to build pivot results with caching
		pivotMap := make(map[string]*PivotResult)

		// Debug counters
		var missedSLACount, totalProcessed, nullTimesheetCount, nullSLACount, validSLACount int

		for _, data := range rawData {
			totalProcessed++

			// Use cached functions to get province and SAC
			province := getProvinceFromCityMTI(data.MerchantCity)
			if province == "" {
				province = getProvinceFromCityMTI(data.MerchantZip)
			}

			city := data.MerchantCity
			if city == "" {
				city = "Unknown"
			}

			technician := data.Technician
			if technician == "" {
				technician = "Unknown"
			}

			sac := getSACFromTechnicianMTI(technician)

			activity := data.TaskType
			if activity == "" {
				activity = "Unknown"
			}

			// Create composite key for grouping
			key := fmt.Sprintf("%s|%s|%s|%s|%s", province, city, sac, technician, activity)

			if pivotMap[key] == nil {
				pivotMap[key] = &PivotResult{
					Provinsi: province,
					Kota:     city,
					SPLeader: sac,
					Teknisi:  technician,
					Activity: activity,
				}
			}

			// Count priorities based on SLA deadlines and task status
			loc, _ := time.LoadLocation(config.WebPanel.Get().Default.Timezone)
			now := time.Now().In(loc)

			// Only count if timesheet_last_stop is null (not yet completed)
			if data.TimesheetLastStop == nil {
				nullTimesheetCount++

				if data.SlaDeadline != nil && !data.SlaDeadline.IsZero() {
					validSLACount++
					// Use configured timezone for consistent comparison
					slaDateInTimezone := data.SlaDeadline.In(loc)
					slaDate := time.Date(slaDateInTimezone.Year(), slaDateInTimezone.Month(), slaDateInTimezone.Day(), 0, 0, 0, 0, loc)
					todayInTimezone := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, loc)
					tomorrowInTimezone := todayInTimezone.AddDate(0, 0, 1)

					if slaDate.Before(todayInTimezone) {
						// Overdue - Miss SLA
						pivotMap[key].Priority1MissSLA++
						missedSLACount++
					} else if slaDate.Equal(todayInTimezone) {
						// Due today
						pivotMap[key].Priority1MustVisitToday++
					} else if slaDate.Equal(tomorrowInTimezone) {
						// Due tomorrow
						pivotMap[key].Priority2MustVisitTomorrow++
					} else if slaDate.After(tomorrowInTimezone) {
						// Due after tomorrow
						pivotMap[key].Priority3MustVisitMoreThanTomorrow++
					}
				} else {
					nullSLACount++
					// No SLA deadline or null SLA - count as pending
					pivotMap[key].PendingMerchantOrBucketBusiness++
				}
			}
		}

		logrus.Debugf("PivotNonPMMTI: Processed %d records, nullTimesheet=%d, validSLA=%d, nullSLA=%d, missedSLA=%d",
			totalProcessed, nullTimesheetCount, validSLACount, nullSLACount, missedSLACount)

		// Convert map to slice
		for _, result := range pivotMap {
			results = append(results, *result)
		}

		// Calculate totals for filtering if search is provided
		filteredRecords := int64(len(results))
		totalRecords := filteredRecords

		// Apply search filter if provided
		if request.Search != "" {
			var filteredResults []PivotResult
			searchLower := strings.ToLower(request.Search)
			for _, result := range results {
				if strings.Contains(strings.ToLower(result.Provinsi), searchLower) ||
					strings.Contains(strings.ToLower(result.Kota), searchLower) ||
					strings.Contains(strings.ToLower(result.SPLeader), searchLower) ||
					strings.Contains(strings.ToLower(result.Teknisi), searchLower) ||
					strings.Contains(strings.ToLower(result.Activity), searchLower) {
					filteredResults = append(filteredResults, result)
				}
			}
			results = filteredResults
			filteredRecords = int64(len(results))
		}

		// Apply sorting
		if request.SortColumn >= 0 && request.SortColumn < 10 { // We have 10 columns now
			// Sort implementation can be added here if needed
		}

		// Apply pagination if length is not -1 (get all)
		if request.Length > 0 && request.Start >= 0 {
			end := request.Start + request.Length
			if end > len(results) {
				end = len(results)
			}
			if request.Start < len(results) {
				results = results[request.Start:end]
			} else {
				results = []PivotResult{}
			}
		}

		// Respond with the formatted data
		c.JSON(http.StatusOK, gin.H{
			"draw":            request.Draw,
			"recordsTotal":    totalRecords,
			"recordsFiltered": filteredRecords,
			"data":            results,
		})
	}
}

func LastUpdateDataTaskODOOMSMTI() gin.HandlerFunc {
	return func(c *gin.Context) {
		var result struct {
			LastUpdate string `json:"last_update"`
		}

		dbWeb := gormdb.Databases.Web
		var lastUpdate time.Time

		err := dbWeb.Model(&mtimodel.MTIOdooMSData{}).
			Select("MAX(updated_at)").
			Scan(&lastUpdate).Error
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}

		if lastUpdate.IsZero() {
			result.LastUpdate = "N/A"
		} else {
			result.LastUpdate = fmt.Sprintf("Last updated @%s", lastUpdate.Format("Monday, 02 Jan 2006 15:04:05 MST"))
		}

		c.JSON(http.StatusOK, result)
	}
}

// ReportAllPMMTI generates and downloads an Excel report containing all PM MTI data with optimizations
func ReportAllPMMTI() gin.HandlerFunc {
	return func(c *gin.Context) {
		dbWeb := gormdb.Databases.Web
		startTime := time.Now()

		logrus.Info("Starting generation of All PM MTI Excel report with optimizations")

		// First, check the count to see if we have data
		var totalCount int64
		if err := dbWeb.Model(&mtimodel.MTIOdooMSData{}).Where("task_type = ?", "Preventive Maintenance").Count(&totalCount).Error; err != nil {
			logrus.Errorf("Error counting PM MTI data: %v", err)
			c.JSON(http.StatusInternalServerError, gin.H{
				"success": false,
				"error":   "Failed to count PM MTI data",
			})
			return
		}

		if totalCount == 0 {
			logrus.Warn("No PM MTI data found")
			c.JSON(http.StatusNotFound, gin.H{
				"success": false,
				"error":   "No PM MTI data available for report generation",
			})
			return
		}

		logrus.Infof("Found %d PM MTI records, proceeding with parallel processing", totalCount)

		// Use context with timeout for the entire operation
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
		defer cancel()

		// Process data and pivot generation in parallel using goroutines
		type result struct {
			allData   []mtimodel.MTIOdooMSData
			pivotData []map[string]interface{}
			err       error
		}

		resultChan := make(chan result, 2)

		// Goroutine 1: Fetch all data with batching
		go func() {
			defer func() {
				if r := recover(); r != nil {
					logrus.Errorf("Panic in data fetching goroutine: %v", r)
					resultChan <- result{err: fmt.Errorf("panic in data fetching: %v", r)}
				}
			}()

			allData, err := fetchPMMTIDataBatched(ctx, dbWeb, totalCount)
			resultChan <- result{allData: allData, err: err}
		}()

		// Goroutine 2: Generate pivot data
		go func() {
			defer func() {
				if r := recover(); r != nil {
					logrus.Errorf("Panic in pivot generation goroutine: %v", r)
					resultChan <- result{err: fmt.Errorf("panic in pivot generation: %v", r)}
				}
			}()

			pivotData, err := generatePivotDataPMMTI(dbWeb, "")
			resultChan <- result{pivotData: pivotData, err: err}
		}()

		// Collect results from both goroutines
		var allData []mtimodel.MTIOdooMSData
		var pivotData []map[string]interface{}
		var dataFetchErr, pivotErr error

		for i := 0; i < 2; i++ {
			select {
			case res := <-resultChan:
				if res.err != nil {
					if len(res.allData) > 0 {
						dataFetchErr = res.err
					} else {
						pivotErr = res.err
					}
				} else {
					if len(res.allData) > 0 {
						allData = res.allData
					} else {
						pivotData = res.pivotData
					}
				}
			case <-ctx.Done():
				logrus.Error("Timeout while fetching PM MTI data and generating pivot")
				c.JSON(http.StatusRequestTimeout, gin.H{
					"success": false,
					"error":   "Request timeout while processing data",
				})
				return
			}
		}

		// Check for errors from parallel processing
		if dataFetchErr != nil {
			logrus.Errorf("Error fetching PM MTI data: %v", dataFetchErr)
			c.JSON(http.StatusInternalServerError, gin.H{
				"success": false,
				"error":   "Failed to fetch PM MTI data",
			})
			return
		}

		if pivotErr != nil {
			logrus.Errorf("Error generating pivot data: %v", pivotErr)
			c.JSON(http.StatusInternalServerError, gin.H{
				"success": false,
				"error":   "Failed to generate pivot data",
			})
			return
		}

		fetchDuration := time.Since(startTime)
		logrus.Infof("Data fetching and pivot generation completed in %v", fetchDuration)

		// Create Excel file with optimized creation
		excelStartTime := time.Now()
		excelFile, err := createExcelReportPMMTIOptimized(allData, pivotData, "All PM MTI Data")
		if err != nil {
			logrus.Errorf("Error creating Excel file: %v", err)
			c.JSON(http.StatusInternalServerError, gin.H{
				"success": false,
				"error":   "Failed to create Excel file",
			})
			return
		}
		defer excelFile.Close()

		excelDuration := time.Since(excelStartTime)
		logrus.Infof("Excel file creation completed in %v", excelDuration)

		// Generate filename
		loc, _ := time.LoadLocation(config.WebPanel.Get().Default.Timezone)
		now := time.Now().In(loc)
		filename := fmt.Sprintf("PM_MTI_All_Data_%s.xlsx", now.Format("02Jan2006_150405"))

		// Set headers for file download
		c.Header("Content-Type", "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet")
		c.Header("Content-Disposition", fmt.Sprintf("attachment; filename=\"%s\"", filename))
		c.Header("Content-Transfer-Encoding", "binary")

		// Save and serve the file
		if err := excelFile.Write(c.Writer); err != nil {
			logrus.Errorf("Error writing Excel file to response: %v", err)
			c.JSON(http.StatusInternalServerError, gin.H{
				"success": false,
				"error":   "Failed to download file",
			})
			return
		}

		logrus.Infof("Successfully generated and served PM MTI Excel report: %s", filename)
	}
}

// ReportDataFilteredPMMTI generates and downloads an Excel report containing filtered PM MTI data
func ReportDataFilteredPMMTI() gin.HandlerFunc {
	return func(c *gin.Context) {
		timeStart := time.Now()
		dbWeb := gormdb.Databases.Web

		logrus.Info("Starting generation of Filtered PM MTI Excel report with optimizations")

		// Parse form data for batched processing
		if err := c.Request.ParseForm(); err != nil {
			logrus.Errorf("Error parsing form data: %v", err)
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid form data"})
			return
		}

		// Build filtered query and get count first
		filteredQuery := dbWeb.Model(&mtimodel.MTIOdooMSData{}).Where("task_type = ?", "Preventive Maintenance")
		filteredQuery = applyDynamicFilters(filteredQuery, c)

		var countDataFiltered int64
		if err := filteredQuery.Count(&countDataFiltered).Error; err != nil {
			logrus.Errorf("Error counting filtered PM MTI data: %v", err)
			c.JSON(http.StatusInternalServerError, gin.H{
				"success": false,
				"error":   "Failed to count filtered PM MTI data",
			})
			return
		}

		logrus.Infof("Found %d filtered PM MTI records to process", countDataFiltered)

		if countDataFiltered == 0 {
			logrus.Warn("No data found for the given filters")
			c.JSON(http.StatusOK, gin.H{
				"success": false,
				"error":   "No data found for the given filters",
			})
			return
		}

		// Use context with timeout for large datasets
		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Minute)
		defer cancel()

		// Parallel processing for data fetching and pivot generation
		var wg sync.WaitGroup
		var filteredData []mtimodel.MTIOdooMSData
		var pivotData []map[string]interface{}
		var dataErr error

		// Goroutine 1: Fetch filtered data using optimized approach
		wg.Add(1)
		go func() {
			defer wg.Done()

			if countDataFiltered > 10000 {
				// Use batched approach for large datasets
				filteredData, dataErr = fetchFilteredPMMTIDataBatched(ctx, dbWeb, c.Request.Form, c, countDataFiltered)
			} else {
				// Direct query for smaller datasets using same logic as batched approach
				filteredData, dataErr = fetchFilteredPMMTIDataBatched(ctx, dbWeb, c.Request.Form, c, countDataFiltered)
			}

			if dataErr != nil {
				logrus.Errorf("Error fetching filtered data: %v", dataErr)
			} else {
				logrus.Infof("Successfully fetched %d filtered records", len(filteredData))
			}
		}()

		// Wait for data fetching to complete before pivot generation
		wg.Wait()

		if dataErr != nil {
			c.JSON(http.StatusInternalServerError, gin.H{
				"success": false,
				"error":   "Failed to fetch filtered PM MTI data",
			})
			return
		}

		if len(filteredData) == 0 {
			c.JSON(http.StatusOK, gin.H{
				"success": false,
				"error":   "No data found for the given filters",
			})
			return
		}

		wg.Wait()

		if dataErr != nil {
			c.JSON(http.StatusInternalServerError, gin.H{
				"success": false,
				"error":   "Failed to fetch filtered PM MTI data",
			})
			return
		}

		if len(filteredData) == 0 {
			c.JSON(http.StatusOK, gin.H{
				"success": false,
				"error":   "No data found for the given filters",
			})
			return
		}

		// Generate pivot data from the same filtered data to ensure consistency
		pivotData = generatePivotDataFromFilteredData(filteredData)
		logrus.Infof("Generated pivot data with %d entries from %d filtered records", len(pivotData), len(filteredData))

		// Create Excel file using optimized function
		excelFile, err := createExcelReportPMMTIOptimized(filteredData, pivotData, "Filtered PM MTI Data")
		if err != nil {
			logrus.Errorf("Error creating Excel file: %v", err)
			c.JSON(http.StatusInternalServerError, gin.H{
				"success": false,
				"error":   "Failed to create Excel file",
			})
			return
		}
		defer excelFile.Close()

		// Generate filename
		loc, _ := time.LoadLocation(config.WebPanel.Get().Default.Timezone)
		now := time.Now().In(loc)
		filename := fmt.Sprintf("PM_MTI_Filtered_Data_%s.xlsx", now.Format("02Jan2006_150405"))

		// Set headers for file download
		c.Header("Content-Type", "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet")
		c.Header("Content-Disposition", fmt.Sprintf("attachment; filename=\"%s\"", filename))
		c.Header("Content-Transfer-Encoding", "binary")

		// Save and serve the file
		if err := excelFile.Write(c.Writer); err != nil {
			logrus.Errorf("Error writing Excel file to response: %v", err)
			c.JSON(http.StatusInternalServerError, gin.H{
				"success": false,
				"error":   "Failed to download file",
			})
			return
		}

		duration := time.Since(timeStart)
		logrus.Infof("Successfully generated and served filtered PM MTI Excel report: %s in %v", filename, duration)
	}
}

// generatePivotDataPMMTI generates pivot table data for PM MTI reports
func generatePivotDataPMMTI(dbWeb *gorm.DB, searchValue string) ([]map[string]interface{}, error) {
	const targetPerHari = 12 // Default target, make configurable if needed

	whereClause := "task_type = 'Preventive Maintenance'"
	var args []interface{}

	// Apply search filter if provided
	if searchValue != "" {
		searchConditions := []string{
			"wo_number LIKE ?",
			"merchant_name LIKE ?",
			"pic_merchant LIKE ?",
			"pic_phone LIKE ?",
			"merchant_address LIKE ?",
			"description LIKE ?",
			"mid LIKE ?",
			"tid LIKE ?",
			"source LIKE ?",
			"message_cc LIKE ?",
			"status_merchant LIKE ?",
			"wo_remark_tiket LIKE ?",
			"technician LIKE ?",
			"reason_code LIKE ?",
			"stage LIKE ?",
			"merchant_city LIKE ?",
		}

		searchValue = "%" + searchValue + "%"
		searchCondition := strings.Join(searchConditions, " OR ")
		whereClause += " AND (" + searchCondition + ")"

		for range searchConditions {
			args = append(args, searchValue)
		}
	}

	tableMTI := config.WebPanel.Get().MTI.TBDataODOOMS

	// Raw SQL query to aggregate the data
	query := fmt.Sprintf(`
		SELECT 
			source as nama_vendor_bank,
			SUM(CASE WHEN stage = 'Terkunjungi' THEN 1 ELSE 0 END) as terkunjungi_count,
			ROUND(SUM(CASE WHEN stage = 'Terkunjungi' THEN 1 ELSE 0 END) * 100.0 / COUNT(*), 2) as terkunjungi_percentage,
			SUM(CASE WHEN stage = 'Gagal Terkunjungi' THEN 1 ELSE 0 END) as gagal_terkunjungi_count,
			ROUND(SUM(CASE WHEN stage = 'Gagal Terkunjungi' THEN 1 ELSE 0 END) * 100.0 / COUNT(*), 2) as gagal_terkunjungi_percentage,
			SUM(CASE WHEN stage NOT IN ('Terkunjungi', 'Gagal Terkunjungi') THEN 1 ELSE 0 END) as belum_kunjungan_count,
			ROUND(SUM(CASE WHEN stage NOT IN ('Terkunjungi', 'Gagal Terkunjungi') THEN 1 ELSE 0 END) * 100.0 / COUNT(*), 2) as belum_kunjungan_percentage,
			COUNT(*) as total_count,
			SUM(CASE WHEN stage = 'Terkunjungi' THEN 1 ELSE 0 END) as run_rate,
			ROUND(SUM(CASE WHEN stage = 'Terkunjungi' THEN 1 ELSE 0 END) * 100.0 / %d, 2) as run_rate_percentage,
			%d as target_per_hari
		FROM %s 
		WHERE %s
		GROUP BY source
		ORDER BY source`,
		targetPerHari, targetPerHari, tableMTI, whereClause)

	var results []map[string]interface{}
	rows, err := dbWeb.Raw(query, args...).Rows()
	if err != nil {
		return nil, fmt.Errorf("failed to execute pivot query: %v", err)
	}
	defer rows.Close()

	for rows.Next() {
		var (
			namaVendorBank             string
			terkunjungiCount           int
			terkunjungiPercentage      float64
			gagalTerkunjungiCount      int
			gagalTerkunjungiPercentage float64
			belumKunjunganCount        int
			belumKunjunganPercentage   float64
			totalCount                 int
			runRate                    int
			runRatePercentage          float64
			targetPerHariValue         int
		)

		if err := rows.Scan(&namaVendorBank, &terkunjungiCount, &terkunjungiPercentage,
			&gagalTerkunjungiCount, &gagalTerkunjungiPercentage, &belumKunjunganCount,
			&belumKunjunganPercentage, &totalCount, &runRate, &runRatePercentage, &targetPerHariValue); err != nil {
			return nil, fmt.Errorf("failed to scan pivot row: %v", err)
		}

		result := map[string]interface{}{
			"nama_vendor_bank":             namaVendorBank,
			"terkunjungi_count":            terkunjungiCount,
			"terkunjungi_percentage":       terkunjungiPercentage,
			"gagal_terkunjungi_count":      gagalTerkunjungiCount,
			"gagal_terkunjungi_percentage": gagalTerkunjungiPercentage,
			"belum_kunjungan_count":        belumKunjunganCount,
			"belum_kunjungan_percentage":   belumKunjunganPercentage,
			"total_count":                  totalCount,
			"run_rate":                     runRate,
			"run_rate_percentage":          runRatePercentage,
			"target_per_hari":              targetPerHariValue,
		}
		results = append(results, result)
	}

	return results, nil
}

// generatePivotDataFromFilteredData generates pivot data from already filtered data slice
// This ensures consistency between master data and pivot data in filtered reports
func generatePivotDataFromFilteredData(filteredData []mtimodel.MTIOdooMSData) []map[string]interface{} {
	const targetPerHari = 12 // Default target, make configurable if needed

	// Process data in Go to build pivot results
	pivotMap := make(map[string]map[string]interface{})

	for _, record := range filteredData {
		source := record.Source
		if source == "" {
			source = "Unknown"
		}

		if _, exists := pivotMap[source]; !exists {
			pivotMap[source] = map[string]interface{}{
				"nama_vendor_bank":             source,
				"terkunjungi_count":            0,
				"terkunjungi_percentage":       0.0,
				"gagal_terkunjungi_count":      0,
				"gagal_terkunjungi_percentage": 0.0,
				"belum_kunjungan_count":        0,
				"belum_kunjungan_percentage":   0.0,
				"total_count":                  0,
				"run_rate":                     0,
				"run_rate_percentage":          0.0,
				"target_per_hari":              targetPerHari,
			}
		}

		// Count records based on stage
		pivotMap[source]["total_count"] = pivotMap[source]["total_count"].(int) + 1

		switch record.Stage {
		case "Terkunjungi":
			pivotMap[source]["terkunjungi_count"] = pivotMap[source]["terkunjungi_count"].(int) + 1
			pivotMap[source]["run_rate"] = pivotMap[source]["run_rate"].(int) + 1
		case "Gagal Terkunjungi":
			pivotMap[source]["gagal_terkunjungi_count"] = pivotMap[source]["gagal_terkunjungi_count"].(int) + 1
		default:
			pivotMap[source]["belum_kunjungan_count"] = pivotMap[source]["belum_kunjungan_count"].(int) + 1
		}
	}

	// Calculate percentages and convert to results slice
	var results []map[string]interface{}
	for _, data := range pivotMap {
		totalCount := data["total_count"].(int)
		if totalCount > 0 {
			terkunjungi := data["terkunjungi_count"].(int)
			gagal := data["gagal_terkunjungi_count"].(int)
			belum := data["belum_kunjungan_count"].(int)
			runRate := data["run_rate"].(int)

			data["terkunjungi_percentage"] = math.Round((float64(terkunjungi)/float64(totalCount)*100)*100) / 100
			data["gagal_terkunjungi_percentage"] = math.Round((float64(gagal)/float64(totalCount)*100)*100) / 100
			data["belum_kunjungi_percentage"] = math.Round((float64(belum)/float64(totalCount)*100)*100) / 100
			data["run_rate_percentage"] = math.Round((float64(runRate)/float64(totalCount)*100)*100) / 100
		}

		results = append(results, data)
	}

	logrus.Infof("Generated pivot data from filtered data: %d vendors, %d total records", len(results), len(filteredData))
	return results
}

// generatePivotDataFromFilteredDataNonPM generates pivot data from already filtered Non-PM data slice
func generatePivotDataFromFilteredDataNonPM(filteredData []mtimodel.MTIOdooMSData) []map[string]interface{} {
	const targetPerHari = 12 // Default target, make configurable if needed

	// Process data in Go to build pivot results
	pivotMap := make(map[string]map[string]interface{})

	for _, record := range filteredData {
		source := record.Source
		if source == "" {
			source = "Unknown"
		}

		if _, exists := pivotMap[source]; !exists {
			pivotMap[source] = map[string]interface{}{
				"nama_vendor_bank":             source,
				"terkunjungi_count":            0,
				"terkunjungi_percentage":       0.0,
				"gagal_terkunjungi_count":      0,
				"gagal_terkunjungi_percentage": 0.0,
				"belum_kunjungan_count":        0,
				"belum_kunjungan_percentage":   0.0,
				"total_count":                  0,
				"run_rate":                     0,
				"run_rate_percentage":          0.0,
				"target_per_hari":              targetPerHari,
			}
		}

		// Count records based on stage
		pivotMap[source]["total_count"] = pivotMap[source]["total_count"].(int) + 1

		switch record.Stage {
		case "Terkunjungi":
			pivotMap[source]["terkunjungi_count"] = pivotMap[source]["terkunjungi_count"].(int) + 1
			pivotMap[source]["run_rate"] = pivotMap[source]["run_rate"].(int) + 1
		case "Gagal Terkunjungi":
			pivotMap[source]["gagal_terkunjungi_count"] = pivotMap[source]["gagal_terkunjungi_count"].(int) + 1
		default:
			pivotMap[source]["belum_kunjungan_count"] = pivotMap[source]["belum_kunjungan_count"].(int) + 1
		}
	}

	// Calculate percentages and convert to results slice
	var results []map[string]interface{}
	for _, data := range pivotMap {
		totalCount := data["total_count"].(int)
		if totalCount > 0 {
			terkunjungi := data["terkunjungi_count"].(int)
			gagal := data["gagal_terkunjungi_count"].(int)
			belum := data["belum_kunjungan_count"].(int)
			runRate := data["run_rate"].(int)

			data["terkunjungi_percentage"] = math.Round((float64(terkunjungi)/float64(totalCount)*100)*100) / 100
			data["gagal_terkunjungi_percentage"] = math.Round((float64(gagal)/float64(totalCount)*100)*100) / 100
			data["belum_kunjungan_percentage"] = math.Round((float64(belum)/float64(totalCount)*100)*100) / 100
			data["run_rate_percentage"] = math.Round((float64(runRate)/float64(totalCount)*100)*100) / 100
		}

		results = append(results, data)
	}

	logrus.Infof("Generated Non-PM pivot data from filtered data: %d vendors, %d total records", len(results), len(filteredData))
	return results
}

// fetchPMMTIDataBatched fetches PM MTI data in batches for better performance
func fetchPMMTIDataBatched(ctx context.Context, dbWeb *gorm.DB, totalCount int64) ([]mtimodel.MTIOdooMSData, error) {
	const batchSize = 5000 // Process 5000 records at a time
	var allData []mtimodel.MTIOdooMSData

	// Pre-allocate slice with known capacity for better memory management
	allData = make([]mtimodel.MTIOdooMSData, 0, totalCount)

	// Calculate number of batches
	numBatches := int(math.Ceil(float64(totalCount) / float64(batchSize)))
	logrus.Infof("Processing %d records in %d batches of %d", totalCount, numBatches, batchSize)

	// Process batches with parallel processing
	type batchResult struct {
		data  []mtimodel.MTIOdooMSData
		err   error
		batch int
	}

	resultChan := make(chan batchResult, numBatches)
	semaphore := make(chan struct{}, 3) // Limit to 3 concurrent database queries

	var wg sync.WaitGroup

	for i := 0; i < numBatches; i++ {
		wg.Add(1)
		go func(batchNum int) {
			defer wg.Done()

			// Acquire semaphore
			semaphore <- struct{}{}
			defer func() { <-semaphore }()

			offset := batchNum * batchSize
			var batchData []mtimodel.MTIOdooMSData

			query := dbWeb.WithContext(ctx).Where("task_type = ?", "Preventive Maintenance").
				Offset(offset).Limit(batchSize).Order("id ASC")

			if err := query.Find(&batchData).Error; err != nil {
				logrus.Errorf("Error fetching batch %d: %v", batchNum, err)
				resultChan <- batchResult{err: err, batch: batchNum}
				return
			}

			logrus.Debugf("Batch %d completed: %d records", batchNum, len(batchData))
			resultChan <- batchResult{data: batchData, batch: batchNum}
		}(i)
	}

	// Close result channel when all goroutines complete
	go func() {
		wg.Wait()
		close(resultChan)
	}()

	// Collect results and maintain order
	batchResults := make(map[int][]mtimodel.MTIOdooMSData)

	for result := range resultChan {
		if result.err != nil {
			return nil, fmt.Errorf("failed to fetch batch %d: %v", result.batch, result.err)
		}
		batchResults[result.batch] = result.data
	}

	// Combine results in order
	for i := 0; i < numBatches; i++ {
		if batchData, exists := batchResults[i]; exists {
			allData = append(allData, batchData...)
		}
	}

	logrus.Infof("Successfully fetched %d records using batched processing", len(allData))
	return allData, nil
}

// createExcelReportPMMTIOptimized creates an Excel file with optimized performance
func createExcelReportPMMTIOptimized(masterData []mtimodel.MTIOdooMSData, pivotData []map[string]interface{}, reportTitle string) (*excelize.File, error) {
	_ = reportTitle // Currently unused, can be used for metadata if needed

	// Create a new Excel file
	f := excelize.NewFile()

	// Create Master Data sheet
	masterSheetName := "Master Data"
	f.SetSheetName("Sheet1", masterSheetName)

	// Define master data headers
	masterHeaders := []string{
		"Task Type", "Stage", "WO Number", "Technician", "MID", "TID",
		"Merchant Name", "Merchant City", "Merchant Zip", "PIC Merchant", "PIC Phone",
		"Merchant Address", "Description", "Source", "Message CC", "Status Merchant",
		"WO Remark Tiket", "Longitude", "Latitude", "Link Photo", "Ticket Type",
		"Worksheet Template", "Ticket Subject", "SN EDC", "EDC Type", "Reason Code",
		"SLA Deadline", "Create Date", "Received DateTime SPK", "Plan Date",
		"Timesheet Last Stop", "Date Last Stage Update",
	}

	// Set master data headers using batch cell updates
	headerCells := make([][]interface{}, 1)
	headerCells[0] = make([]interface{}, len(masterHeaders))
	for i, header := range masterHeaders {
		headerCells[0][i] = header
	}

	if err := f.SetSheetRow(masterSheetName, "A1", &headerCells[0]); err != nil {
		return nil, fmt.Errorf("failed to set header row: %v", err)
	}

	// Process master data in batches for better performance
	const rowBatchSize = 1000
	numBatches := int(math.Ceil(float64(len(masterData)) / float64(rowBatchSize)))

	logrus.Infof("Writing %d master data rows in %d batches", len(masterData), numBatches)

	for batchNum := 0; batchNum < numBatches; batchNum++ {
		startIdx := batchNum * rowBatchSize
		endIdx := startIdx + rowBatchSize
		if endIdx > len(masterData) {
			endIdx = len(masterData)
		}

		// Prepare batch data
		batchRows := make([][]interface{}, endIdx-startIdx)

		for i, dataIdx := 0, startIdx; dataIdx < endIdx; i, dataIdx = i+1, dataIdx+1 {
			data := masterData[dataIdx]
			batchRows[i] = []interface{}{
				data.TaskType, data.Stage, data.WONumber, data.Technician, data.Mid, data.Tid,
				data.MerchantName, data.MerchantCity, data.MerchantZip, data.PicMerchant, data.PicPhone,
				data.MerchantAddress, data.Description, data.Source, data.MessageCC, data.StatusMerchant,
				data.WoRemarkTiket, data.Longitude, data.Latitude, data.LinkPhoto, data.TicketType,
				data.WorksheetTemplate, data.TicketSubject, data.SNEDC, data.EDCType, data.ReasonCode,
				data.SlaDeadline, data.CreateDate, data.ReceivedDatetimeSpk, data.PlanDate,
				data.TimesheetLastStop, data.DateLastStageUpdate,
			}
		}

		// Write batch to Excel
		for i, row := range batchRows {
			rowNum := startIdx + i + 2
			if err := f.SetSheetRow(masterSheetName, fmt.Sprintf("A%d", rowNum), &row); err != nil {
				return nil, fmt.Errorf("failed to set row %d: %v", rowNum, err)
			}
		}

		logrus.Debugf("Completed batch %d/%d for master data", batchNum+1, numBatches)
	}

	// Create Pivot Data sheet with optimized writing
	pivotSheetName := "Pivot Summary"
	f.NewSheet(pivotSheetName)

	// Define pivot headers
	pivotHeaders := []string{
		"NAMA VENDOR (BANK)", "TERKUNJUNGI Count", "TERKUNJUNGI Percentage (%)",
		"GAGAL TERKUNJUNGI Count", "GAGAL TERKUNJUNGI Percentage (%)",
		"BELUM KUNJUNGAN Count", "BELUM KUNJUNGAN Percentage (%)",
		"Total Count of TID", "Run Rate", "RUN RATE (%)", "TARGET PER HARI",
	}

	// Set pivot headers
	pivotHeaderCells := make([]interface{}, len(pivotHeaders))
	for i, header := range pivotHeaders {
		pivotHeaderCells[i] = header
	}

	if err := f.SetSheetRow(pivotSheetName, "A1", &pivotHeaderCells); err != nil {
		return nil, fmt.Errorf("failed to set pivot header row: %v", err)
	}

	// Add pivot data rows in batches
	logrus.Infof("Writing %d pivot data rows", len(pivotData))

	for rowIdx, data := range pivotData {
		row := []interface{}{
			data["nama_vendor_bank"],
			data["terkunjungi_count"],
			data["terkunjungi_percentage"],
			data["gagal_terkunjungi_count"],
			data["gagal_terkunjungi_percentage"],
			data["belum_kunjungan_count"],
			data["belum_kunjungan_percentage"],
			data["total_count"],
			data["run_rate"],
			data["run_rate_percentage"],
			data["target_per_hari"],
		}

		if err := f.SetSheetRow(pivotSheetName, fmt.Sprintf("A%d", rowIdx+2), &row); err != nil {
			return nil, fmt.Errorf("failed to set pivot row %d: %v", rowIdx+2, err)
		}
	}

	// Calculate and add totals row for pivot (same as before but optimized)
	if len(pivotData) > 0 {
		totalRow := len(pivotData) + 2

		var totalTerkunjungi, totalGagal, totalBelum, totalCount, totalRunRate, totalTarget int
		for _, data := range pivotData {
			if val, ok := data["terkunjungi_count"].(int); ok {
				totalTerkunjungi += val
			}
			if val, ok := data["gagal_terkunjungi_count"].(int); ok {
				totalGagal += val
			}
			if val, ok := data["belum_kunjungan_count"].(int); ok {
				totalBelum += val
			}
			if val, ok := data["total_count"].(int); ok {
				totalCount += val
			}
			if val, ok := data["run_rate"].(int); ok {
				totalRunRate += val
			}
			if val, ok := data["target_per_hari"].(int); ok {
				totalTarget += val
			}
		}

		var avgTerkunjungiPercentage, avgGagalPercentage, avgBelumPercentage, avgRunRatePercentage float64
		if totalCount > 0 {
			avgTerkunjungiPercentage = float64(totalTerkunjungi) / float64(totalCount) * 100
			avgGagalPercentage = float64(totalGagal) / float64(totalCount) * 100
			avgBelumPercentage = float64(totalBelum) / float64(totalCount) * 100
			avgRunRatePercentage = float64(totalRunRate) / float64(totalCount) * 100
		}

		totalsRow := []interface{}{
			"Grand Total",
			totalTerkunjungi,
			math.Round(avgTerkunjungiPercentage*100) / 100,
			totalGagal,
			math.Round(avgGagalPercentage*100) / 100,
			totalBelum,
			math.Round(avgBelumPercentage*100) / 100,
			totalCount,
			totalRunRate,
			math.Round(avgRunRatePercentage*100) / 100,
			totalTarget,
		}

		if err := f.SetSheetRow(pivotSheetName, fmt.Sprintf("A%d", totalRow), &totalsRow); err != nil {
			return nil, fmt.Errorf("failed to set totals row: %v", err)
		}
	}

	// Apply styling in batch operations for better performance
	headerStyle, _ := f.NewStyle(&excelize.Style{
		Font: &excelize.Font{Bold: true},
		Fill: excelize.Fill{Type: "pattern", Color: []string{"E0E0E0"}, Pattern: 1},
	})

	// Apply header style to both sheets
	masterHeaderRange := fmt.Sprintf("A1:%s1", string(rune('A'+len(masterHeaders)-1)))
	f.SetCellStyle(masterSheetName, "A1", masterHeaderRange, headerStyle)

	pivotHeaderRange := fmt.Sprintf("A1:%s1", string(rune('A'+len(pivotHeaders)-1)))
	f.SetCellStyle(pivotSheetName, "A1", pivotHeaderRange, headerStyle)

	logrus.Info("Excel file creation completed with optimizations")
	return f, nil
}

// fetchFilteredPMMTIDataBatched fetches filtered PM MTI data in batches
func fetchFilteredPMMTIDataBatched(ctx context.Context, dbWeb *gorm.DB, formData map[string][]string, c *gin.Context, totalCount int64) ([]mtimodel.MTIOdooMSData, error) {
	_ = c // Unused parameter, kept for potential future use
	const batchSize = 5000
	var allData []mtimodel.MTIOdooMSData

	// Pre-allocate slice
	allData = make([]mtimodel.MTIOdooMSData, 0, totalCount)

	numBatches := int(math.Ceil(float64(totalCount) / float64(batchSize)))
	logrus.Infof("Processing %d filtered PM records in %d batches", totalCount, numBatches)

	type batchResult struct {
		data  []mtimodel.MTIOdooMSData
		err   error
		batch int
	}

	resultChan := make(chan batchResult, numBatches)
	semaphore := make(chan struct{}, 3)

	var wg sync.WaitGroup

	for i := 0; i < numBatches; i++ {
		wg.Add(1)
		go func(batchNum int) {
			defer wg.Done()

			semaphore <- struct{}{}
			defer func() { <-semaphore }()

			offset := batchNum * batchSize
			var batchData []mtimodel.MTIOdooMSData

			// Build filtered query for this batch
			query := dbWeb.WithContext(ctx).Where("task_type = ?", "Preventive Maintenance")

			// Apply all filters (both universal search and specific columns)
			query = applyFormDataFilters(query, formData)

			query = query.Offset(offset).Limit(batchSize).Order("id ASC")

			if err := query.Find(&batchData).Error; err != nil {
				logrus.Errorf("Error fetching filtered PM batch %d: %v", batchNum, err)
				resultChan <- batchResult{err: err, batch: batchNum}
				return
			}

			resultChan <- batchResult{data: batchData, batch: batchNum}
		}(i)
	}

	go func() {
		wg.Wait()
		close(resultChan)
	}()

	// Collect results
	batchResults := make(map[int][]mtimodel.MTIOdooMSData)

	for result := range resultChan {
		if result.err != nil {
			return nil, fmt.Errorf("failed to fetch filtered PM batch %d: %v", result.batch, result.err)
		}
		batchResults[result.batch] = result.data
	}

	// Combine results in order
	for i := 0; i < numBatches; i++ {
		if batchData, exists := batchResults[i]; exists {
			allData = append(allData, batchData...)
		}
	}

	logrus.Infof("Successfully fetched %d filtered PM records using batched processing", len(allData))
	return allData, nil
}

// applyFormDataFilters applies filtering logic from form data to a GORM query
// This function handles BOTH universal search AND specific column filters
func applyFormDataFilters(query *gorm.DB, formData map[string][]string) *gorm.DB {
	logrus.Debugf("Applying filters from form data: %+v", formData)

	// Map of form field names to database column names
	fieldMap := map[string]string{
		"wo_number":          "wo_number",
		"merchant_name":      "merchant_name",
		"pic_merchant":       "pic_merchant",
		"pic_phone":          "pic_phone",
		"merchant_address":   "merchant_address",
		"description":        "description",
		"mid":                "mid",
		"tid":                "tid",
		"source":             "source",
		"message_cc":         "message_cc",
		"status_merchant":    "status_merchant",
		"wo_remark_tiket":    "wo_remark_tiket",
		"technician":         "technician",
		"reason_code":        "reason_code",
		"stage":              "stage",
		"merchant_city":      "merchant_city",
		"task_type":          "task_type",
		"ticket_type":        "ticket_type",
		"worksheet_template": "worksheet_template",
		"ticket_subject":     "ticket_subject",
		"sn_edc":             "sn_edc",
		"edc_type":           "edc_type",
		"merchant_zip":       "merchant_zip",
	}

	// Handle universal search first
	if searchValues, exists := formData["search[value]"]; exists && len(searchValues) > 0 && searchValues[0] != "" {
		searchValue := searchValues[0]
		logrus.Infof("Applying universal search filter: '%s'", searchValue)
		searchConditions := []string{}
		var searchArgs []interface{}
		searchPattern := "%" + searchValue + "%"

		// Apply search across all mapped columns
		for _, dbColumn := range fieldMap {
			searchConditions = append(searchConditions, dbColumn+" LIKE ?")
			searchArgs = append(searchArgs, searchPattern)
		}

		if len(searchConditions) > 0 {
			queryCondition := strings.Join(searchConditions, " OR ")
			query = query.Where(queryCondition, searchArgs...)
		}
	}

	// Also handle alternative search field
	if searchValues, exists := formData["search_value"]; exists && len(searchValues) > 0 && searchValues[0] != "" {
		searchValue := searchValues[0]
		// Check if universal search was already applied
		if _, universalExists := formData["search[value]"]; !universalExists {
			logrus.Infof("Applying alternative search filter: '%s'", searchValue)
			searchConditions := []string{}
			var searchArgs []interface{}
			searchPattern := "%" + searchValue + "%"

			// Apply search across all mapped columns
			for _, dbColumn := range fieldMap {
				searchConditions = append(searchConditions, dbColumn+" LIKE ?")
				searchArgs = append(searchArgs, searchPattern)
			}

			if len(searchConditions) > 0 {
				queryCondition := strings.Join(searchConditions, " OR ")
				query = query.Where(queryCondition, searchArgs...)
			}
		}
	}

	// Handle date range filters
	if dateFromValues, exists := formData["date_from"]; exists && len(dateFromValues) > 0 && dateFromValues[0] != "" {
		if dateToValues, exists := formData["date_to"]; exists && len(dateToValues) > 0 && dateToValues[0] != "" {
			dateFrom := dateFromValues[0]
			dateTo := dateToValues[0]
			logrus.Infof("Applying date range filter: %s to %s", dateFrom, dateTo)
			query = query.Where("DATE(create_date) BETWEEN ? AND ?", dateFrom, dateTo)
		}
	}

	// Handle specific column filters
	for formField, dbColumn := range fieldMap {
		if values, exists := formData[formField]; exists && len(values) > 0 && values[0] != "" {
			value := values[0]
			// Skip universal search fields as they were already handled above
			if formField == "search[value]" || formField == "search_value" {
				continue
			}
			logrus.Infof("Applying specific column filter: %s = '%s'", formField, value)
			// Handle different filter types
			if strings.Contains(strings.ToLower(formField), "date") {
				// Date exact match
				query = query.Where(dbColumn+" = ?", value)
			} else {
				// Text filtering with LIKE
				query = query.Where(dbColumn+" LIKE ?", "%"+value+"%")
			}
		}
	}

	return query
}

// fetchNonPMMTIDataBatched fetches Non-PM MTI data in batches for better performance
func fetchNonPMMTIDataBatched(ctx context.Context, dbWeb *gorm.DB, totalCount int64) ([]mtimodel.MTIOdooMSData, error) {
	const batchSize = 5000
	var allData []mtimodel.MTIOdooMSData

	allData = make([]mtimodel.MTIOdooMSData, 0, totalCount)

	numBatches := int(math.Ceil(float64(totalCount) / float64(batchSize)))
	logrus.Infof("Processing %d Non-PM records in %d batches", totalCount, numBatches)

	type batchResult struct {
		data  []mtimodel.MTIOdooMSData
		err   error
		batch int
	}

	resultChan := make(chan batchResult, numBatches)
	semaphore := make(chan struct{}, 3)

	var wg sync.WaitGroup

	for i := 0; i < numBatches; i++ {
		wg.Add(1)
		go func(batchNum int) {
			defer wg.Done()

			semaphore <- struct{}{}
			defer func() { <-semaphore }()

			offset := batchNum * batchSize
			var batchData []mtimodel.MTIOdooMSData

			query := dbWeb.WithContext(ctx).Where("task_type != ?", "Preventive Maintenance").
				Offset(offset).Limit(batchSize).Order("id ASC")

			if err := query.Find(&batchData).Error; err != nil {
				logrus.Errorf("Error fetching Non-PM batch %d: %v", batchNum, err)
				resultChan <- batchResult{err: err, batch: batchNum}
				return
			}

			logrus.Debugf("Non-PM batch %d completed: %d records", batchNum, len(batchData))
			resultChan <- batchResult{data: batchData, batch: batchNum}
		}(i)
	}

	go func() {
		wg.Wait()
		close(resultChan)
	}()

	batchResults := make(map[int][]mtimodel.MTIOdooMSData)

	for result := range resultChan {
		if result.err != nil {
			return nil, fmt.Errorf("failed to fetch Non-PM batch %d: %v", result.batch, result.err)
		}
		batchResults[result.batch] = result.data
	}

	for i := 0; i < numBatches; i++ {
		if batchData, exists := batchResults[i]; exists {
			allData = append(allData, batchData...)
		}
	}

	logrus.Infof("Successfully fetched %d Non-PM records using batched processing", len(allData))
	return allData, nil
}

// fetchFilteredNonPMMTIDataBatched fetches filtered Non-PM MTI data in batches
func fetchFilteredNonPMMTIDataBatched(ctx context.Context, dbWeb *gorm.DB, formData map[string][]string, c *gin.Context, totalCount int64) ([]mtimodel.MTIOdooMSData, error) {
	_ = c // Unused parameter, kept for potential future use
	const batchSize = 5000
	var allData []mtimodel.MTIOdooMSData

	// Pre-allocate slice
	allData = make([]mtimodel.MTIOdooMSData, 0, totalCount)

	numBatches := int(math.Ceil(float64(totalCount) / float64(batchSize)))
	logrus.Infof("Processing %d filtered Non-PM records in %d batches", totalCount, numBatches)

	type batchResult struct {
		data  []mtimodel.MTIOdooMSData
		err   error
		batch int
	}

	resultChan := make(chan batchResult, numBatches)
	semaphore := make(chan struct{}, 3)

	var wg sync.WaitGroup

	for i := 0; i < numBatches; i++ {
		wg.Add(1)
		go func(batchNum int) {
			defer wg.Done()

			semaphore <- struct{}{}
			defer func() { <-semaphore }()

			offset := batchNum * batchSize
			var batchData []mtimodel.MTIOdooMSData

			// Build filtered query for this batch
			query := dbWeb.WithContext(ctx).Where("task_type != ?", "Preventive Maintenance")

			// Apply all filters (both universal search and specific columns)
			query = applyFormDataFilters(query, formData)

			query = query.Offset(offset).Limit(batchSize).Order("id ASC")

			if err := query.Find(&batchData).Error; err != nil {
				logrus.Errorf("Error fetching filtered Non-PM batch %d: %v", batchNum, err)
				resultChan <- batchResult{err: err, batch: batchNum}
				return
			}

			resultChan <- batchResult{data: batchData, batch: batchNum}
		}(i)
	}

	go func() {
		wg.Wait()
		close(resultChan)
	}()

	// Collect results
	batchResults := make(map[int][]mtimodel.MTIOdooMSData)

	for result := range resultChan {
		if result.err != nil {
			return nil, fmt.Errorf("failed to fetch filtered Non-PM batch %d: %v", result.batch, result.err)
		}
		batchResults[result.batch] = result.data
	}

	// Combine results in order
	for i := 0; i < numBatches; i++ {
		if batchData, exists := batchResults[i]; exists {
			allData = append(allData, batchData...)
		}
	}

	logrus.Infof("Successfully fetched %d filtered Non-PM records using batched processing", len(allData))
	return allData, nil
}

// createExcelReportNonPMMTIOptimized creates an Excel file with optimized performance for Non-PM MTI
func createExcelReportNonPMMTIOptimized(masterData []mtimodel.MTIOdooMSData, pivotData []map[string]interface{}, reportTitle string) (*excelize.File, error) {
	_ = reportTitle // Currently unused, can be used for metadata if needed

	f := excelize.NewFile()

	masterSheetName := "Master Data"
	f.SetSheetName("Sheet1", masterSheetName)

	masterHeaders := []string{
		"Task Type", "Stage", "WO Number", "Technician", "MID", "TID",
		"Merchant Name", "Merchant City", "Merchant Zip", "PIC Merchant", "PIC Phone",
		"Merchant Address", "Description", "Source", "Message CC", "Status Merchant",
		"WO Remark Tiket", "Longitude", "Latitude", "Link Photo", "Ticket Type",
		"Worksheet Template", "Ticket Subject", "SN EDC", "EDC Type", "Reason Code",
		"SLA Deadline", "Create Date", "Received DateTime SPK", "Plan Date",
		"Timesheet Last Stop", "Date Last Stage Update",
	}

	headerCells := make([][]interface{}, 1)
	headerCells[0] = make([]interface{}, len(masterHeaders))
	for i, header := range masterHeaders {
		headerCells[0][i] = header
	}

	if err := f.SetSheetRow(masterSheetName, "A1", &headerCells[0]); err != nil {
		return nil, fmt.Errorf("failed to set header row: %v", err)
	}

	const rowBatchSize = 1000
	numBatches := int(math.Ceil(float64(len(masterData)) / float64(rowBatchSize)))

	logrus.Infof("Writing %d Non-PM master data rows in %d batches", len(masterData), numBatches)

	for batchNum := 0; batchNum < numBatches; batchNum++ {
		startIdx := batchNum * rowBatchSize
		endIdx := startIdx + rowBatchSize
		if endIdx > len(masterData) {
			endIdx = len(masterData)
		}

		batchRows := make([][]interface{}, endIdx-startIdx)

		for i, dataIdx := 0, startIdx; dataIdx < endIdx; i, dataIdx = i+1, dataIdx+1 {
			data := masterData[dataIdx]
			batchRows[i] = []interface{}{
				data.TaskType, data.Stage, data.WONumber, data.Technician, data.Mid, data.Tid,
				data.MerchantName, data.MerchantCity, data.MerchantZip, data.PicMerchant, data.PicPhone,
				data.MerchantAddress, data.Description, data.Source, data.MessageCC, data.StatusMerchant,
				data.WoRemarkTiket, data.Longitude, data.Latitude, data.LinkPhoto, data.TicketType,
				data.WorksheetTemplate, data.TicketSubject, data.SNEDC, data.EDCType, data.ReasonCode,
				data.SlaDeadline, data.CreateDate, data.ReceivedDatetimeSpk, data.PlanDate,
				data.TimesheetLastStop, data.DateLastStageUpdate,
			}
		}

		for i, row := range batchRows {
			rowNum := startIdx + i + 2
			if err := f.SetSheetRow(masterSheetName, fmt.Sprintf("A%d", rowNum), &row); err != nil {
				return nil, fmt.Errorf("failed to set row %d: %v", rowNum, err)
			}
		}

		logrus.Debugf("Completed batch %d/%d for Non-PM master data", batchNum+1, numBatches)
	}

	pivotSheetName := "Pivot Summary"
	f.NewSheet(pivotSheetName)

	pivotHeaders := []string{
		"NAMA VENDOR (BANK)", "TERKUNJUNGI Count", "TERKUNJUNGI Percentage (%)",
		"GAGAL TERKUNJUNGI Count", "GAGAL TERKUNJUNGI Percentage (%)",
		"BELUM KUNJUNGAN Count", "BELUM KUNJUNGAN Percentage (%)",
		"Total Count of TID", "Run Rate", "RUN RATE (%)", "TARGET PER HARI",
	}

	pivotHeaderCells := make([]interface{}, len(pivotHeaders))
	for i, header := range pivotHeaders {
		pivotHeaderCells[i] = header
	}

	if err := f.SetSheetRow(pivotSheetName, "A1", &pivotHeaderCells); err != nil {
		return nil, fmt.Errorf("failed to set pivot header row: %v", err)
	}

	logrus.Infof("Writing %d Non-PM pivot data rows", len(pivotData))

	for rowIdx, data := range pivotData {
		row := []interface{}{
			data["nama_vendor_bank"],
			data["terkunjungi_count"],
			data["terkunjungi_percentage"],
			data["gagal_terkunjungi_count"],
			data["gagal_terkunjungi_percentage"],
			data["belum_kunjungan_count"],
			data["belum_kunjungan_percentage"],
			data["total_count"],
			data["run_rate"],
			data["run_rate_percentage"],
			data["target_per_hari"],
		}

		if err := f.SetSheetRow(pivotSheetName, fmt.Sprintf("A%d", rowIdx+2), &row); err != nil {
			return nil, fmt.Errorf("failed to set pivot row %d: %v", rowIdx+2, err)
		}
	}

	// Add totals row (similar to PM version)
	if len(pivotData) > 0 {
		totalRow := len(pivotData) + 2

		var totalTerkunjungi, totalGagal, totalBelum, totalCount, totalRunRate, totalTarget int
		for _, data := range pivotData {
			if val, ok := data["terkunjungi_count"].(int); ok {
				totalTerkunjungi += val
			}
			if val, ok := data["gagal_terkunjungi_count"].(int); ok {
				totalGagal += val
			}
			if val, ok := data["belum_kunjungan_count"].(int); ok {
				totalBelum += val
			}
			if val, ok := data["total_count"].(int); ok {
				totalCount += val
			}
			if val, ok := data["run_rate"].(int); ok {
				totalRunRate += val
			}
			if val, ok := data["target_per_hari"].(int); ok {
				totalTarget += val
			}
		}

		var avgTerkunjungiPercentage, avgGagalPercentage, avgBelumPercentage, avgRunRatePercentage float64
		if totalCount > 0 {
			avgTerkunjungiPercentage = float64(totalTerkunjungi) / float64(totalCount) * 100
			avgGagalPercentage = float64(totalGagal) / float64(totalCount) * 100
			avgBelumPercentage = float64(totalBelum) / float64(totalCount) * 100
			avgRunRatePercentage = float64(totalRunRate) / float64(totalCount) * 100
		}

		totalsRow := []interface{}{
			"Grand Total",
			totalTerkunjungi,
			math.Round(avgTerkunjungiPercentage*100) / 100,
			totalGagal,
			math.Round(avgGagalPercentage*100) / 100,
			totalBelum,
			math.Round(avgBelumPercentage*100) / 100,
			totalCount,
			totalRunRate,
			math.Round(avgRunRatePercentage*100) / 100,
			totalTarget,
		}

		if err := f.SetSheetRow(pivotSheetName, fmt.Sprintf("A%d", totalRow), &totalsRow); err != nil {
			return nil, fmt.Errorf("failed to set totals row: %v", err)
		}
	}

	// Apply styling
	headerStyle, _ := f.NewStyle(&excelize.Style{
		Font: &excelize.Font{Bold: true},
		Fill: excelize.Fill{Type: "pattern", Color: []string{"E0E0E0"}, Pattern: 1},
	})

	masterHeaderRange := fmt.Sprintf("A1:%s1", string(rune('A'+len(masterHeaders)-1)))
	f.SetCellStyle(masterSheetName, "A1", masterHeaderRange, headerStyle)

	pivotHeaderRange := fmt.Sprintf("A1:%s1", string(rune('A'+len(pivotHeaders)-1)))
	f.SetCellStyle(pivotSheetName, "A1", pivotHeaderRange, headerStyle)

	logrus.Info("Non-PM Excel file creation completed with optimizations")
	return f, nil
}

// applyDynamicFilters applies filters based on form data to a GORM query
func applyDynamicFilters(query *gorm.DB, c *gin.Context) *gorm.DB {
	// Convert gin.Context form data to map[string][]string format
	formData := make(map[string][]string)

	// Parse form data
	if err := c.Request.ParseForm(); err == nil {
		for key, values := range c.Request.Form {
			formData[key] = values
		}
	}

	// Also handle POST form data
	if c.Request.Method == "POST" {
		for key := range c.Request.PostForm {
			if value := c.PostForm(key); value != "" {
				formData[key] = []string{value}
			}
		}
	}

	// Use the unified filtering logic
	return applyFormDataFilters(query, formData)
}

// ReportAllNonPMMTI generates and downloads an Excel report containing all Non-PM MTI data
func ReportAllNonPMMTI() gin.HandlerFunc {
	return func(c *gin.Context) {
		timeStart := time.Now()
		dbWeb := gormdb.Databases.Web

		logrus.Info("Starting generation of All Non-PM MTI Excel report with optimizations")

		// Get count first to determine processing strategy
		var totalCount int64
		if err := dbWeb.Model(&mtimodel.MTIOdooMSData{}).Where("task_type != ?", "Preventive Maintenance").Count(&totalCount).Error; err != nil {
			logrus.Errorf("Error counting Non-PM MTI records: %v", err)
			c.JSON(http.StatusInternalServerError, gin.H{
				"success": false,
				"error":   "Failed to count Non-PM MTI records",
			})
			return
		}

		logrus.Infof("Found %d Non-PM MTI records to process", totalCount)

		if totalCount == 0 {
			logrus.Info("No Non-PM MTI data found")
			c.JSON(http.StatusOK, gin.H{
				"success": false,
				"error":   "No Non-PM MTI data found",
			})
			return
		}

		// Use context with timeout for large datasets
		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Minute)
		defer cancel()

		// Parallel processing for data fetching and pivot generation
		var wg sync.WaitGroup
		var allData []mtimodel.MTIOdooMSData
		var pivotData []map[string]interface{}
		var dataErr, pivotErr error

		// Goroutine 1: Fetch all Non-PM data using optimized approach
		wg.Add(1)
		go func() {
			defer wg.Done()

			if totalCount > 10000 {
				// Use batched approach for large datasets
				allData, dataErr = fetchNonPMMTIDataBatched(ctx, dbWeb, totalCount)
			} else {
				// Direct query for smaller datasets
				dataErr = dbWeb.WithContext(ctx).Where("task_type != ?", "Preventive Maintenance").Find(&allData).Error
			}

			if dataErr != nil {
				logrus.Errorf("Error fetching Non-PM data: %v", dataErr)
			} else {
				logrus.Infof("Successfully fetched %d Non-PM records", len(allData))
			}
		}()

		// Goroutine 2: Generate pivot data in parallel
		wg.Add(1)
		go func() {
			defer wg.Done()
			pivotData, pivotErr = generatePivotDataNonPMMTI(dbWeb, "")
			if pivotErr == nil {
				logrus.Infof("Generated Non-PM pivot data with %d entries", len(pivotData))
			}
		}()

		wg.Wait()

		if dataErr != nil {
			c.JSON(http.StatusInternalServerError, gin.H{
				"success": false,
				"error":   "Failed to fetch Non-PM MTI data",
			})
			return
		}

		if pivotErr != nil {
			logrus.Errorf("Error generating Non-PM pivot data: %v", pivotErr)
			c.JSON(http.StatusInternalServerError, gin.H{
				"success": false,
				"error":   "Failed to generate pivot data",
			})
			return
		}

		// Create Excel file using optimized function
		excelFile, err := createExcelReportNonPMMTIOptimized(allData, pivotData, "All Non-PM MTI Data")
		if err != nil {
			logrus.Errorf("Error creating Excel file: %v", err)
			c.JSON(http.StatusInternalServerError, gin.H{
				"success": false,
				"error":   "Failed to create Excel file",
			})
			return
		}
		defer excelFile.Close()

		// Generate filename
		loc, _ := time.LoadLocation(config.WebPanel.Get().Default.Timezone)
		now := time.Now().In(loc)
		filename := fmt.Sprintf("Non_PM_MTI_All_Data_%s.xlsx", now.Format("02Jan2006_150405"))

		// Set headers for file download
		c.Header("Content-Type", "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet")
		c.Header("Content-Disposition", fmt.Sprintf("attachment; filename=\"%s\"", filename))
		c.Header("Content-Transfer-Encoding", "binary")

		// Save and serve the file
		if err := excelFile.Write(c.Writer); err != nil {
			logrus.Errorf("Error writing Excel file to response: %v", err)
			c.JSON(http.StatusInternalServerError, gin.H{
				"success": false,
				"error":   "Failed to download file",
			})
			return
		}

		duration := time.Since(timeStart)
		logrus.Infof("Successfully generated and served Non-PM MTI Excel report: %s in %v", filename, duration)
	}
}

// ReportDataFilteredNonPMMTI generates and downloads an Excel report containing filtered Non-PM MTI data
func ReportDataFilteredNonPMMTI() gin.HandlerFunc {
	return func(c *gin.Context) {
		timeStart := time.Now()
		dbWeb := gormdb.Databases.Web

		logrus.Info("Starting generation of Filtered Non-PM MTI Excel report with optimizations")

		// Parse form data for batched processing
		if err := c.Request.ParseForm(); err != nil {
			logrus.Errorf("Error parsing form data: %v", err)
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid form data"})
			return
		}

		// Build filtered query and get count first
		filteredQuery := dbWeb.Model(&mtimodel.MTIOdooMSData{}).Where("task_type != ?", "Preventive Maintenance")
		filteredQuery = applyDynamicFilters(filteredQuery, c)

		var countDataFiltered int64
		if err := filteredQuery.Count(&countDataFiltered).Error; err != nil {
			logrus.Errorf("Error counting filtered Non-PM MTI data: %v", err)
			c.JSON(http.StatusInternalServerError, gin.H{
				"success": false,
				"error":   "Failed to count filtered Non-PM MTI data",
			})
			return
		}

		logrus.Infof("Found %d filtered Non-PM MTI records to process", countDataFiltered)

		if countDataFiltered == 0 {
			logrus.Warn("No data found for the given filters")
			c.JSON(http.StatusOK, gin.H{
				"success": false,
				"error":   "No data found for the given filters",
			})
			return
		}

		// Use context with timeout for large datasets
		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Minute)
		defer cancel()

		// Parallel processing for data fetching and pivot generation
		var wg sync.WaitGroup
		var filteredData []mtimodel.MTIOdooMSData
		var pivotData []map[string]interface{}
		var dataErr error

		// Goroutine 1: Fetch filtered data using optimized approach
		wg.Add(1)
		go func() {
			defer wg.Done()

			if countDataFiltered > 10000 {
				// Use batched approach for large datasets
				filteredData, dataErr = fetchFilteredNonPMMTIDataBatched(ctx, dbWeb, c.Request.Form, c, countDataFiltered)
			} else {
				// Direct query for smaller datasets using same logic as batched approach
				filteredData, dataErr = fetchFilteredNonPMMTIDataBatched(ctx, dbWeb, c.Request.Form, c, countDataFiltered)
			}

			if dataErr != nil {
				logrus.Errorf("Error fetching filtered Non-PM data: %v", dataErr)
			} else {
				logrus.Infof("Successfully fetched %d filtered Non-PM records", len(filteredData))
			}
		}()

		// Wait for data fetching to complete before pivot generation
		wg.Wait()

		if dataErr != nil {
			c.JSON(http.StatusInternalServerError, gin.H{
				"success": false,
				"error":   "Failed to fetch filtered Non-PM MTI data",
			})
			return
		}

		if len(filteredData) == 0 {
			c.JSON(http.StatusOK, gin.H{
				"success": false,
				"error":   "No data found for the given filters",
			})
			return
		}

		wg.Wait()

		if dataErr != nil {
			c.JSON(http.StatusInternalServerError, gin.H{
				"success": false,
				"error":   "Failed to fetch filtered Non-PM MTI data",
			})
			return
		}

		if len(filteredData) == 0 {
			c.JSON(http.StatusOK, gin.H{
				"success": false,
				"error":   "No data found for the given filters",
			})
			return
		}

		// Generate pivot data from the same filtered data to ensure consistency
		pivotData = generatePivotDataFromFilteredDataNonPM(filteredData)
		logrus.Infof("Generated Non-PM pivot data with %d entries from %d filtered records", len(pivotData), len(filteredData))

		// Create Excel file using optimized function
		excelFile, err := createExcelReportNonPMMTIOptimized(filteredData, pivotData, "Filtered Non-PM MTI Data")
		if err != nil {
			logrus.Errorf("Error creating Excel file: %v", err)
			c.JSON(http.StatusInternalServerError, gin.H{
				"success": false,
				"error":   "Failed to create Excel file",
			})
			return
		}
		defer excelFile.Close()

		// Generate filename
		loc, _ := time.LoadLocation(config.WebPanel.Get().Default.Timezone)
		now := time.Now().In(loc)
		filename := fmt.Sprintf("Non_PM_MTI_Filtered_Data_%s.xlsx", now.Format("02Jan2006_150405"))

		// Set headers for file download
		c.Header("Content-Type", "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet")
		c.Header("Content-Disposition", fmt.Sprintf("attachment; filename=\"%s\"", filename))
		c.Header("Content-Transfer-Encoding", "binary")

		// Save and serve the file
		if err := excelFile.Write(c.Writer); err != nil {
			logrus.Errorf("Error writing Excel file to response: %v", err)
			c.JSON(http.StatusInternalServerError, gin.H{
				"success": false,
				"error":   "Failed to download file",
			})
			return
		}

		duration := time.Since(timeStart)
		logrus.Infof("Successfully generated and served filtered Non-PM MTI Excel report: %s in %v", filename, duration)
	}
}

// generatePivotDataNonPMMTI generates pivot table data for Non-PM MTI reports
func generatePivotDataNonPMMTI(dbWeb *gorm.DB, searchValue string) ([]map[string]interface{}, error) {
	whereClause := "task_type != 'Preventive Maintenance'"
	var args []interface{}

	// Apply search filter if provided
	if searchValue != "" {
		searchConditions := []string{
			"wo_number LIKE ?",
			"merchant_name LIKE ?",
			"pic_merchant LIKE ?",
			"pic_phone LIKE ?",
			"merchant_address LIKE ?",
			"description LIKE ?",
			"mid LIKE ?",
			"tid LIKE ?",
			"source LIKE ?",
			"message_cc LIKE ?",
			"status_merchant LIKE ?",
			"wo_remark_tiket LIKE ?",
			"technician LIKE ?",
			"reason_code LIKE ?",
			"stage LIKE ?",
			"merchant_city LIKE ?",
		}

		searchValue = "%" + searchValue + "%"
		searchCondition := strings.Join(searchConditions, " OR ")
		whereClause += " AND (" + searchCondition + ")"

		for range searchConditions {
			args = append(args, searchValue)
		}
	}

	// Initialize region cache if not already done
	if !regionCacheInitialized {
		initRegionCacheMTI()
	}

	// Get raw data from database with basic filters
	var rawData []mtimodel.MTIOdooMSData
	if err := dbWeb.Where(whereClause, args...).Find(&rawData).Error; err != nil {
		return nil, fmt.Errorf("failed to fetch Non-PM data: %v", err)
	}

	// Process data in Go to build pivot results with caching
	pivotMap := make(map[string]map[string]interface{})

	for _, record := range rawData {
		// Get province from city using cache
		province := getProvinceFromCityMTI(record.MerchantCity)
		city := record.MerchantCity
		if city == "" {
			city = "Unknown"
		}

		// Get SAC from technician name - can be used later if needed
		// sac := getSACFromTechnicianMTI(record.Technician)
		spLeader := "N/A" // You may need to implement getSPLeaderFromTechnicianMTI if needed
		technician := record.Technician
		if technician == "" {
			technician = "Unknown"
		}
		activity := record.TaskType
		if activity == "" {
			activity = "Unknown"
		}

		// Create composite key
		key := fmt.Sprintf("%s|%s|%s|%s|%s", province, city, spLeader, technician, activity)

		if _, exists := pivotMap[key]; !exists {
			pivotMap[key] = map[string]interface{}{
				"provinsi":                       province,
				"kota":                           city,
				"sp_leader":                      spLeader,
				"teknisi":                        technician,
				"activity":                       activity,
				"priority_1_miss_sla":            0,
				"priority_1_must_visit_today":    0,
				"priority_2_must_visit_tomorrow": 0,
				"priority_3_must_visit_more_than_tomorrow": 0,
				"pending_merchant_or_bucket_business":      0,
			}
		}

		// Count records based on priority/status
		// This is a simplified version - you may need to implement proper priority logic
		switch record.Stage {
		case "Priority 1 - Miss SLA":
			pivotMap[key]["priority_1_miss_sla"] = pivotMap[key]["priority_1_miss_sla"].(int) + 1
		case "Priority 1 - Must Visit Today":
			pivotMap[key]["priority_1_must_visit_today"] = pivotMap[key]["priority_1_must_visit_today"].(int) + 1
		case "Priority 2 - Must Visit Tomorrow":
			pivotMap[key]["priority_2_must_visit_tomorrow"] = pivotMap[key]["priority_2_must_visit_tomorrow"].(int) + 1
		case "Priority 3 - Must Visit More Than Tomorrow":
			pivotMap[key]["priority_3_must_visit_more_than_tomorrow"] = pivotMap[key]["priority_3_must_visit_more_than_tomorrow"].(int) + 1
		default:
			pivotMap[key]["pending_merchant_or_bucket_business"] = pivotMap[key]["pending_merchant_or_bucket_business"].(int) + 1
		}
	}

	// Convert map to slice
	var results []map[string]interface{}
	for _, data := range pivotMap {
		results = append(results, data)
	}

	return results, nil
}

// generatePivotDataNonPMMTIFromContext generates pivot table data for Non-PM MTI reports using gin context
// func generatePivotDataNonPMMTIFromContext(dbWeb *gorm.DB, c *gin.Context) ([]map[string]interface{}, error) {
// 	// Initialize region cache if not already done
// 	if !regionCacheInitialized {
// 		initRegionCacheMTI()
// 	}

// 	// Get filtered data first
// 	var filteredData []mtimodel.MTIOdooMSData
// 	query := dbWeb.Model(&mtimodel.MTIOdooMSData{}).Where("task_type != 'Preventive Maintenance'")
// 	query = applyDynamicFilters(query, c)

// 	if err := query.Find(&filteredData).Error; err != nil {
// 		return nil, fmt.Errorf("failed to fetch filtered Non-PM data: %v", err)
// 	}

// 	// Process data in Go to build pivot results with caching
// 	pivotMap := make(map[string]map[string]interface{})

// 	for _, record := range filteredData {
// 		// Get province from city using cache
// 		province := getProvinceFromCityMTI(record.MerchantCity)
// 		city := record.MerchantCity
// 		if city == "" {
// 			city = "Unknown"
// 		}

// 		// Get SAC from technician name - can be used later if needed
// 		// sac := getSACFromTechnicianMTI(record.Technician)
// 		spLeader := "N/A" // You may need to implement getSPLeaderFromTechnicianMTI if needed
// 		technician := record.Technician
// 		if technician == "" {
// 			technician = "Unknown"
// 		}
// 		activity := record.TaskType
// 		if activity == "" {
// 			activity = "Unknown"
// 		}

// 		// Create composite key
// 		key := fmt.Sprintf("%s|%s|%s|%s|%s", province, city, spLeader, technician, activity)

// 		if _, exists := pivotMap[key]; !exists {
// 			pivotMap[key] = map[string]interface{}{
// 				"provinsi":                       province,
// 				"kota":                           city,
// 				"sp_leader":                      spLeader,
// 				"teknisi":                        technician,
// 				"activity":                       activity,
// 				"priority_1_miss_sla":            0,
// 				"priority_1_must_visit_today":    0,
// 				"priority_2_must_visit_tomorrow": 0,
// 				"priority_3_must_visit_more_than_tomorrow": 0,
// 				"pending_merchant_or_bucket_business":      0,
// 			}
// 		}

// 		// Count records based on priority/status
// 		// This is a simplified version - you may need to implement proper priority logic
// 		switch record.Stage {
// 		case "Priority 1 - Miss SLA":
// 			pivotMap[key]["priority_1_miss_sla"] = pivotMap[key]["priority_1_miss_sla"].(int) + 1
// 		case "Priority 1 - Must Visit Today":
// 			pivotMap[key]["priority_1_must_visit_today"] = pivotMap[key]["priority_1_must_visit_today"].(int) + 1
// 		case "Priority 2 - Must Visit Tomorrow":
// 			pivotMap[key]["priority_2_must_visit_tomorrow"] = pivotMap[key]["priority_2_must_visit_tomorrow"].(int) + 1
// 		case "Priority 3 - Must Visit More Than Tomorrow":
// 			pivotMap[key]["priority_3_must_visit_more_than_tomorrow"] = pivotMap[key]["priority_3_must_visit_more_than_tomorrow"].(int) + 1
// 		default:
// 			pivotMap[key]["pending_merchant_or_bucket_business"] = pivotMap[key]["pending_merchant_or_bucket_business"].(int) + 1
// 		}
// 	}

// 	// Convert map to slice
// 	var results []map[string]interface{}
// 	for _, data := range pivotMap {
// 		results = append(results, data)
// 	}

// 	return results, nil
// }

// createExcelReportNonPMMTI creates an Excel file with master data and pivot sheets for Non-PM
// func createExcelReportNonPMMTI(masterData []mtimodel.MTIOdooMSData, pivotData []map[string]interface{}, reportTitle string) (*excelize.File, error) {
// 	// Create a new Excel file
// 	f := excelize.NewFile()

// 	// Create Master Data sheet
// 	masterSheetName := "Master Data"
// 	f.SetSheetName("Sheet1", masterSheetName)

// 	// Define master data headers (same as PM)
// 	masterHeaders := []string{
// 		"Task Type", "Stage", "WO Number", "Technician", "MID", "TID",
// 		"Merchant Name", "Merchant City", "Merchant Zip", "PIC Merchant", "PIC Phone",
// 		"Merchant Address", "Description", "Source", "Message CC", "Status Merchant",
// 		"WO Remark Tiket", "Longitude", "Latitude", "Link Photo", "Ticket Type",
// 		"Worksheet Template", "Ticket Subject", "SN EDC", "EDC Type", "Reason Code",
// 		"SLA Deadline", "Create Date", "Received DateTime SPK", "Plan Date",
// 		"Timesheet Last Stop", "Date Last Stage Update",
// 	}

// 	// Set master data headers
// 	for i, header := range masterHeaders {
// 		cellName, _ := excelize.CoordinatesToCellName(i+1, 1)
// 		f.SetCellValue(masterSheetName, cellName, header)
// 	}

// 	// Add master data rows
// 	for rowIdx, data := range masterData {
// 		row := rowIdx + 2 // Start from row 2 (after headers)

// 		values := []interface{}{
// 			data.TaskType, data.Stage, data.WONumber, data.Technician, data.Mid, data.Tid,
// 			data.MerchantName, data.MerchantCity, data.MerchantZip, data.PicMerchant, data.PicPhone,
// 			data.MerchantAddress, data.Description, data.Source, data.MessageCC, data.StatusMerchant,
// 			data.WoRemarkTiket, data.Longitude, data.Latitude, data.LinkPhoto, data.TicketType,
// 			data.WorksheetTemplate, data.TicketSubject, data.SNEDC, data.EDCType, data.ReasonCode,
// 			data.SlaDeadline, data.CreateDate, data.ReceivedDatetimeSpk, data.PlanDate,
// 			data.TimesheetLastStop, data.DateLastStageUpdate,
// 		}

// 		for colIdx, value := range values {
// 			cellName, _ := excelize.CoordinatesToCellName(colIdx+1, row)
// 			f.SetCellValue(masterSheetName, cellName, value)
// 		}
// 	}

// 	// Create Pivot Data sheet
// 	pivotSheetName := "Pivot Summary"
// 	f.NewSheet(pivotSheetName)

// 	// Define pivot headers for Non-PM
// 	pivotHeaders := []string{
// 		"Provinsi", "Kota", "Nama SP Leader", "Nama Teknisi", "Activity",
// 		"Priority 1 Miss SLA", "Priority 1 Must Visit Today", "Priority 2 Must Visit Tomorrow",
// 		"Priority 3 Must Visit More Than Tomorrow", "Pending Merchant / Bucket Business MTI",
// 	}

// 	// Set pivot headers
// 	for i, header := range pivotHeaders {
// 		cellName, _ := excelize.CoordinatesToCellName(i+1, 1)
// 		f.SetCellValue(pivotSheetName, cellName, header)
// 	}

// 	// Add pivot data rows
// 	for rowIdx, data := range pivotData {
// 		row := rowIdx + 2 // Start from row 2 (after headers)

// 		values := []interface{}{
// 			data["provinsi"],
// 			data["kota"],
// 			data["sp_leader"],
// 			data["teknisi"],
// 			data["activity"],
// 			data["priority_1_miss_sla"],
// 			data["priority_1_must_visit_today"],
// 			data["priority_2_must_visit_tomorrow"],
// 			data["priority_3_must_visit_more_than_tomorrow"],
// 			data["pending_merchant_or_bucket_business"],
// 		}

// 		for colIdx, value := range values {
// 			cellName, _ := excelize.CoordinatesToCellName(colIdx+1, row)
// 			f.SetCellValue(pivotSheetName, cellName, value)
// 		}
// 	}

// 	// Apply basic styling
// 	headerStyle, _ := f.NewStyle(&excelize.Style{
// 		Font: &excelize.Font{Bold: true},
// 		Fill: excelize.Fill{Type: "pattern", Color: []string{"E0E0E0"}, Pattern: 1},
// 	})

// 	// Apply header style to both sheets
// 	for i := range masterHeaders {
// 		cellName, _ := excelize.CoordinatesToCellName(i+1, 1)
// 		f.SetCellStyle(masterSheetName, cellName, cellName, headerStyle)
// 	}

// 	for i := range pivotHeaders {
// 		cellName, _ := excelize.CoordinatesToCellName(i+1, 1)
// 		f.SetCellStyle(pivotSheetName, cellName, cellName, headerStyle)
// 	}

// 	return f, nil
// }
