package controllers

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"service-platform/cmd/web_panel/fun"
	"service-platform/cmd/web_panel/internal/gormdb"
	dspmodel "service-platform/cmd/web_panel/model/dsp_model"
	"service-platform/internal/config"
	"strings"
	"sync"
	"time"

	"github.com/TigorLazuardi/tanggal"
	"github.com/gin-gonic/gin"
	"github.com/sirupsen/logrus"
	"github.com/xuri/excelize/v2"
)

var (
	getTicketDSPODOOMSMutex sync.Mutex
)

func GetTicketDSPODOOMS() (string, error) {
	taskDoing := "Get Ticket DSP"
	if !getTicketDSPODOOMSMutex.TryLock() {
		return "", fmt.Errorf("%s task is still running. Please wait until it is finished", taskDoing)
	}
	defer getTicketDSPODOOMSMutex.Unlock()

	ODOOModel := "helpdesk.ticket"
	domain := []interface{}{
		[]interface{}{"company_id", "=", config.WebPanel.Get().DSP.CompanyIDInODOOMS},
	}
	fieldID := []string{"id"}
	fields := []string{
		"priority",
		"name",
		"stage_id",
		"x_master_mid",
		"x_master_tid",
		"x_wo_number",
		"x_wo_number_last",
		"x_reason",
		"x_reasoncode",
		"x_link",
		"x_merchant",
		"x_merchant_pic",
		"x_merchant_pic_phone",
		"x_studio_kota",
		"x_merchant_zipcode",
		"x_studio_alamat",
		"x_status_merchant",
		"x_status_edc",
		"x_condition_edc",
		"x_wo_remark",
		"x_partner_latitude",
		"x_partner_longitude",
		"ticket_type_id",
		"technician_id",
		"x_merchant_sn_edc",
		"x_merchant_tipe_edc",
		"x_received_datetime_spk",
		"x_sla_deadline",
		"complete_datetime_wo",
	}
	order := "id ASC"
	odooParams := map[string]interface{}{
		"model":  ODOOModel,
		"domain": domain,
		"fields": fieldID,
		"order":  order,
	}

	payload := map[string]interface{}{
		"jsonrpc": config.WebPanel.Get().ApiODOO.JSONRPC,
		"params":  odooParams,
	}
	payloadBytes, err := json.Marshal(payload)
	if err != nil {
		return "", fmt.Errorf("error marshalling payload to JSON: %v", err)
	}

	ODOOResponse, err := GetODOOMSData(string(payloadBytes))
	if err != nil {
		return "", fmt.Errorf("error getting data from ODOOMS: %v", err)
	}

	ODOOResponseArray, ok := ODOOResponse.([]interface{})
	if !ok {
		return "", errors.New("invalid response format from ODOOMS")
	}

	ids := extractUniqueIDs(ODOOResponseArray)
	if len(ids) == 0 {
		return "", errors.New("no ticket IDs found in ODOOMS response")
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
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Minute)
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

			logrus.Debugf("Processing (%s) chunk %d of %d (IDs %v to %v)", taskDoing, chunkIndex+1, len(chunks), chunkData[0], chunkData[len(chunkData)-1])

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
			return "", errors.New("timeout waiting for chunk processing")
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
		return "", errors.New("no records fetched from ODOO MS")
	}

	ODOOResponseBytes, err := json.Marshal(allRecords)
	if err != nil {
		return "", fmt.Errorf("failed to marshal combined response: %v", err)
	}

	// Pre-allocate slice with estimated capacity to reduce memory allocations
	var listOfData []OdooTicketDataRequestItem
	estimatedCapacity := len(allRecords) * 10 // Reduced from 50 to prevent over-allocation
	if estimatedCapacity > 50000 {            // Cap maximum pre-allocation
		estimatedCapacity = 50000
	}
	if estimatedCapacity > 0 {
		listOfData = make([]OdooTicketDataRequestItem, 0, estimatedCapacity)
	}

	if err := json.Unmarshal(ODOOResponseBytes, &listOfData); err != nil {
		return "", fmt.Errorf("failed to unmarshal ODOO response: %v", err)
	}

	// Log memory usage for monitoring
	var memStats runtime.MemStats
	runtime.ReadMemStats(&memStats)
	// Force garbage collection to free up memory before database operations
	runtime.GC()

	// Clear existing records from the database
	dbWeb := gormdb.Databases.Web
	result := dbWeb.Unscoped().Where("1=1").Delete(&dspmodel.TicketDSP{})
	if result.Error != nil {
		return "", fmt.Errorf("failed to clear existing DSP tickets: %v", result.Error)
	}
	logrus.Infof("Cleared %d existing DSP tickets from the database", result.RowsAffected)

	const dbBatchSize = 1000
	for i := 0; i < len(listOfData); i += dbBatchSize {
		end := i + dbBatchSize
		if end > len(listOfData) {
			end = len(listOfData)
		}

		var batch []dspmodel.TicketDSP
		for _, data := range listOfData[i:end] {
			_, technicianName, err := parseJSONIDDataCombined(data.TechnicianId)
			if err != nil {
				logrus.Warnf("Failed to parse technician_id for ticket ID %d: %v", data.ID, err)
			}

			_, edcType, err := parseJSONIDDataCombined(data.EdcType)
			if err != nil {
				logrus.Error(err)
			}

			_, snEdc, err := parseJSONIDDataCombined(data.SnEdc)
			if err != nil {
				logrus.Error(err)
			}

			_, stage, err := parseJSONIDDataCombined(data.StageId)
			if err != nil {
				logrus.Error(err)
			}

			_, ticketType, err := parseJSONIDDataCombined(data.TicketTypeId)
			if err != nil {
				logrus.Error(err)
			}

			// _, worksheetTemplate, err := parseJSONIDDataCombined(data.WorksheetTemplateId)
			// if err != nil {
			// 	logrus.Error(err)
			// }

			// _, project, err := parseJSONIDDataCombined(data.ProjectId)
			// if err != nil {
			// 	logrus.Error(err)
			// }

			// techGroup, err := techGroup(technicianName)
			// if err != nil {
			// 	logrus.Error(err)
			// }

			ticketSLAStatus, firstTaskDatetime, firstTaskReason, firstTaskMessage := setSLAStatus(data.TaskCount.Int, data.SlaDeadline, data.CompleteDatetimeWo, data.WoRemarkTiket, data.TaskType)

			firstReasonCode := ""
			ticketReasonCodes := parseReasonCode(data.ReasonCode.String)
			if len(ticketReasonCodes) > 0 {
				firstReasonCode = ticketReasonCodes[len(ticketReasonCodes)-1]
			}

			// slaExpired := SLAExpired(data.SlaDeadline)

			var slaDeadline, receivedDatetimeSpk, completeDatetimeWo, firstTaskCompleteDatetime *time.Time
			if data.SlaDeadline.Valid {
				slaDeadline = &data.SlaDeadline.Time
			}
			if data.ReceivedDatetimeSpk.Valid {
				receivedDatetimeSpk = &data.ReceivedDatetimeSpk.Time
			}
			if data.CompleteDatetimeWo.Valid {
				completeDatetimeWo = &data.CompleteDatetimeWo.Time
			}

			if !firstTaskDatetime.IsZero() {
				firstTaskCompleteDatetime = &firstTaskDatetime
			}

			var merchantLatitude, merchantLongitude *float64
			if data.Latitude.Valid {
				merchantLatitude = &data.Latitude.Float
			}
			if data.Longitude.Valid {
				merchantLongitude = &data.Longitude.Float
			}

			batch = append(batch, dspmodel.TicketDSP{
				ID:                        data.ID,
				Priority:                  data.Priority.String,
				Subject:                   data.TicketSubject.String,
				Stage:                     stage,
				Mid:                       data.Mid.String,
				Tid:                       data.Tid.String,
				WoNumberFirst:             data.WOFirst.String,
				WoNumberLast:              data.WoNumberLast.String,
				ReasonCode:                data.ReasonCode.String,
				Reason:                    data.Reason.String,
				LinkWod:                   data.LinkWO.String,
				MerchantName:              data.MerchantName.String,
				MerchantPic:               data.PicMerchant.String,
				PicPhoneNumber:            data.PicPhone.String,
				MerchantCity:              data.MerchantCity.String,
				MerchantZipCode:           data.MerchantZipCode.String,
				MerchantAddress:           data.MerchantAddress.String,
				MerchantCondition:         data.StatusMerchant.String,
				EdcStatus:                 data.StatusEDC.String,
				EdcCondition:              data.KondisiEDC.String,
				WoRemark:                  data.WoRemarkTiket.String,
				Latitude:                  merchantLatitude,
				Longitude:                 merchantLongitude,
				TicketType:                ticketType,
				Technician:                technicianName,
				SnEdc:                     snEdc,
				EdcType:                   edcType,
				ReceivedDatetimeSpk:       receivedDatetimeSpk,
				SlaDeadline:               slaDeadline,
				CompleteDatetimeWO:        completeDatetimeWo,
				FirstTaskCompleteDatetime: firstTaskCompleteDatetime,
				FirstTaskReason:           firstTaskReason,
				FirstTaskReasonCode:       firstReasonCode,
				FirstTaskMessage:          firstTaskMessage,
				SLAStatus:                 ticketSLAStatus,
			})
		}

		if err := dbWeb.Model(&dspmodel.TicketDSP{}).Create(&batch).Error; err != nil {
			return "", fmt.Errorf("failed to insert DSP tickets batch starting at index %d: %v", i, err)
		}
	}

	logrus.Infof("Inserted %d DSP tickets into the database", len(listOfData))
	// Log final memory usage
	runtime.ReadMemStats(&memStats)
	logrus.Infof("Final Memory Usage: Alloc = %v MiB, TotalAlloc = %v MiB, Sys = %v MiB, NumGC = %v",
		memStats.Alloc/1024/1024, memStats.TotalAlloc/1024/1024, memStats.Sys/1024/1024, memStats.NumGC)

	return fmt.Sprintf("[%s] task completed successfully @%v", taskDoing, time.Now().Format("15:04:05, 02 Jan 2006")), nil

}

func RefreshTicketDSP() gin.HandlerFunc {
	return func(c *gin.Context) {
		result, err := GetTicketDSPODOOMS()
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}

		c.JSON(http.StatusOK, gin.H{"message": result})
	}
}

func GetLastUpdateTicketDSP() gin.HandlerFunc {
	return func(c *gin.Context) {
		dbWeb := gormdb.Databases.Web

		// Get the latest update timestamp from the database
		var lastUpdatedData time.Time
		if err := dbWeb.Model(&dspmodel.TicketDSP{}).
			Select("updated_at").
			Order("updated_at DESC").
			Limit(1).
			Scan(&lastUpdatedData).
			Error; err != nil {
			// If there's an error during the database query
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Error retrieving the last update timestamp: " + err.Error()})
			return
		}

		// If the lastUpdatedData is still zero, return not found error
		if lastUpdatedData.IsZero() {
			c.JSON(http.StatusNotFound, gin.H{"message": "No last updated timestamp found."})
			return
		}

		// Return the last updated timestamp in the required format
		tgl, err := tanggal.Papar(lastUpdatedData, "Jakarta", tanggal.WIB)
		if err != nil {
			// If there's an error during formatting
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Error formatting the last update timestamp: " + err.Error()})
			return
		}
		lastUpdated := tgl.Format(" ", []tanggal.Format{
			tanggal.NamaHari,
			tanggal.Hari,
			tanggal.NamaBulan,
			tanggal.Tahun,
			tanggal.PukulDenganDetik,
			tanggal.ZonaWaktu,
		})
		c.JSON(http.StatusOK, gin.H{
			"lastUpdated": lastUpdated,
		})
	}
}

func TableTicketDSP() gin.HandlerFunc {
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

		t := reflect.TypeOf(dspmodel.TicketDSP{})

		// Initialize the map
		columnMap := make(map[int]string)

		// Loop through the fields of the struct
		colNum := 0
		for i := 0; i < t.NumField(); i++ {
			field := t.Field(i)
			// Get the JSON key
			jsonKey := field.Tag.Get("json")
			if jsonKey == "" || jsonKey == "-" || jsonKey == "link_wod" || jsonKey == "location" {
				continue
			}
			columnMap[colNum] = jsonKey
			colNum++
		}

		// Get the column name based on SortColumn value
		sortColumnName := columnMap[request.SortColumn]
		var orderString string
		if request.Search == "" {
			orderString = "id DESC"
		} else {
			orderString = fmt.Sprintf("%s %s", sortColumnName, request.SortDir)
		}

		// Initial query for filtering
		dbWeb := gormdb.Databases.Web
		filteredQuery := dbWeb.Model(&dspmodel.TicketDSP{})

		// // Apply filters
		if request.Search != "" {
			// var querySearch []string
			// var querySearchParams []interface{}

			// fmt.Println("++++++++++++++++++++++++++++++")
			// fmt.Print("Search: ", request.Search)
			// fmt.Println("++++++++++++++++++++++++++++++")

			for i := 0; i < t.NumField(); i++ {
				dataField := ""
				field := t.Field(i)
				// Get the data type
				dataType := field.Type.String()
				// Get the JSON key
				jsonKey := field.Tag.Get("json")
				// Get the GORM tag
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
				if jsonKey == "" || jsonKey == "-" || jsonKey == "link_wod" || jsonKey == "location" {
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

				// Get the variable name
				varName := field.Name
				fmt.Printf("Variable Name: %s, Data Type: %s, JSON Key: %s, GORM Column Key: %s\n", varName, dataType, jsonKey, columnKey)

				filteredQuery = filteredQuery.Or("`"+dataField+"` LIKE ?", "%"+request.Search+"%")
			}

		} else {
			for i := 0; i < t.NumField(); i++ {
				field := t.Field(i)
				// formKey := field.Tag.Get("form") // fix this SOON !!
				formKey := field.Tag.Get("json")

				if formKey == "" || formKey == "-" || formKey == "link_wod" {
					continue
				}

				formValue := c.PostForm(formKey)
				// fmt.Print("Form Key: ", formKey)
				// fmt.Print("Form Value: ", formValue)

				// if formValue != "" {
				// 	filteredQuery = filteredQuery.Where("`"+formKey+"` LIKE ?", "%"+formValue+"%")
				// }
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
		dbWeb.Model(&dspmodel.TicketDSP{}).Count(&totalRecords)

		// Count the number of filtered records
		var filteredRecords int64
		filteredQuery.Count(&filteredRecords)

		// Apply sorting and pagination to the filtered query
		query := filteredQuery.Order(orderString)
		var Teknisis []dspmodel.TicketDSP
		query = query.
			Offset(request.Start).
			Limit(request.Length).
			Find(&Teknisis)

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
		for _, person := range Teknisis {
			newData := make(map[string]interface{})

			v := reflect.ValueOf(person)

			for i := 0; i < t.NumField(); i++ {
				field := t.Field(i)
				fieldValue := v.Field(i)

				// varName := field.Name

				// Get the JSON key
				theKey := field.Tag.Get("json")
				if theKey == "" {
					theKey = field.Tag.Get("form")
					if theKey == "" {
						continue
					}
				}

				switch theKey {
				case "birthdate":
					if fieldValue.Type() == reflect.TypeOf(time.Time{}) {
						newData[theKey] = fieldValue.Interface().(time.Time).Format(fun.T_YYYYMMDD)
					} else {
						newData[theKey] = fieldValue.Interface()
					}
				case "date":
					if fieldValue.Type() == reflect.TypeOf(time.Time{}) {
						newData[theKey] = fieldValue.Interface().(time.Time).
							Add(7 * time.Hour).
							Format(fun.T_YYYYMMDD_HHmmss)
					} else {
						newData[theKey] = fieldValue.Interface()
					}
				case "link_wod":
					link := fieldValue.Interface().(string)
					if link != "" {
						newData[theKey] = fmt.Sprintf(`
							<button class="btn btn-sm btn-warning" onclick="window.open('%s', '_blank')">
								<i class='bx bxs-file-find me-2 text-white'></i> Open WOD
							</button>
						`, link)
					} else {
						newData[theKey] = "<p class='text-danger'>No link available</p>"
					}
				case "location":
					lat, latOK := newData["x_partner_latitude"].(float64)
					lng, lngOK := newData["x_partner_longitude"].(float64)

					if latOK && lngOK && (lat != 0 || lng != 0) {
						newData[theKey] = fmt.Sprintf(`<a href="https://maps.google.com/?q=%f,%f" target="_blank" class="btn btn-sm btn-outline-warning">
						<i class="bx bx-map me-1"></i> Open Location</a>`, lat, lng)
					} else {
						newData[theKey] = `<p class="text-danger">Location not available</p>`
					}
				case "stage":
					switch fieldValue.Interface() {
					case "New":
						newData[theKey] = `<span class="badge bg-info">New</span>` // Cyan
					case "Pending":
						newData[theKey] = `<span class="badge bg-warning">Pending</span>` // Yellow
					case "Waiting For Verification":
						newData[theKey] = `<span class="badge bg-primary">Waiting For Verification</span>` // Blue
					case "Done":
						newData[theKey] = `<span class="badge bg-label-success">Done</span>` // Green
					case "Solved":
						newData[theKey] = `<span class="badge bg-success">Solved</span>` // Green
					case "Cancel":
						newData[theKey] = `<span class="badge bg-danger">Cancel</span>` // Red
					case "Closed":
						newData[theKey] = `<span class="badge bg-secondary">Closed</span>` // Gray
					case "Solved Pending":
						newData[theKey] = `<span class="badge bg-label-warning">Solved Pending</span>` // Light yellow (custom class or label-style)
					case "Cancel New":
						newData[theKey] = `<span class="badge bg-label-danger">Cancel New</span>` // Light red (custom class or label-style)
					default:
						newData[theKey] = `<span class="badge bg-label-light">Unknown</span>` // Fallback for unknown status
					}
				case "received_datetime_spk", "sla_deadline", "complete_datetime_wo":
					if fieldValue.IsNil() {
						newData[theKey] = "N/A"
					} else {
						newData[theKey] = fieldValue.Interface().(*time.Time).Format("2006-01-02 15:04:05")
					}
				case "priority":
					var ratingHTML string
					switch fieldValue.Interface() {
					case "0":
						ratingHTML = `<i class="bx bx-star"></i><i class="bx bx-star"></i><i class="bx bx-star"></i>`
					case "1":
						ratingHTML = `<i class="bx bxs-star text-dark"></i><i class="bx bx-star"></i><i class="bx bx-star"></i>`
					case "2":
						ratingHTML = `<i class="bx bxs-star text-dark"></i><i class="bx bxs-star text-dark"></i><i class="bx bx-star"></i>`
					case "3":
						ratingHTML = `<i class="bx bxs-star text-dark"></i><i class="bx bxs-star text-dark"></i><i class="bx bxs-star text-dark"></i>`
					default:
						ratingHTML = `<i class="bx bx-star"></i><i class="bx bx-star"></i><i class="bx bx-star"></i>`
					}
					newData[theKey] = ratingHTML
				default:
					if fieldValue.Type() == reflect.TypeOf(time.Time{}) {
						newData[theKey] = fieldValue.Interface().(time.Time).Format(fun.T_YYYYMMDD_HHmmss)
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

func GetReportALLTicketDSP() gin.HandlerFunc {
	return func(c *gin.Context) {
		var request struct {
			UserID int `form:"user_id"`
		}

		// Bind form data to request struct
		if err := c.Bind(&request); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}

		fileDirPaths := []string{
			"../../web/file/excel_report",
			"../web/file/excel_report",
			"web/file/excel_report",
		}

		var existingPath string
		for _, path := range fileDirPaths {
			info, err := os.Stat(path)
			if err == nil && info.IsDir() {
				existingPath = path
				break
			}
		}

		if existingPath == "" {
			c.JSON(http.StatusInternalServerError, gin.H{
				"error": "No file report directory path valid found!",
			})
		}

		excelFilename := fmt.Sprintf("(DSP)all_data_report_%d_%s.xlsx", request.UserID, time.Now().Format("02Jan2006_15_04"))
		excelFilePath := filepath.Join(existingPath, excelFilename)

		dbWeb := gormdb.Databases.Web
		var countData int64
		if err := dbWeb.Model(&dspmodel.TicketDSP{}).Count(&countData).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Error counting DSP tickets: " + err.Error()})
			return
		}

		if countData == 0 {
			c.JSON(http.StatusNotFound, gin.H{"error": "No DSP ticket data available to generate report."})
			return
		}

		f := excelize.NewFile()
		sheetMaster := "MASTER"
		sheetPivot := "PIVOT"
		f.NewSheet(sheetMaster)
		f.NewSheet(sheetPivot)

		/* Styles */
		style, err := f.NewStyle(&excelize.Style{
			Alignment: &excelize.Alignment{
				Horizontal: "center",
				Vertical:   "center",
			},
			// Border: []excelize.Border{
			// 	{Type: "left", Color: "000000", Style: 1},
			// 	{Type: "right", Color: "000000", Style: 1},
			// 	{Type: "top", Color: "000000", Style: 1},
			// 	{Type: "bottom", Color: "000000", Style: 1},
			// },
		})
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Error creating Excel style: " + err.Error()})
			return
		}

		styleTitleMaster, err := f.NewStyle(styleExcelTitle("#000000", false, "#FFFFFF"))
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Error creating Excel title style: " + err.Error()})
			return
		}

		titles := []struct {
			Title    string
			ColWidth float64
		}{
			{"Ticket Number", 40},
			{"Priority", 15},
			{"Stage", 30},
			{"MID", 20},
			{"TID", 20},
			{"WO Number First", 20},
			{"WO Number Last", 20},
			{"Reason Code", 30},
			{"Reason", 50},
			{"Link WOD", 50},
			{"Merchant Name", 30},
			{"PIC Merchant", 25},
			{"PIC Phone Number", 20},
			{"Merchant City", 20},
			{"Merchant Zip Code", 15},
			{"Merchant Address", 50},
			{"Merchant Condition", 20},
			{"EDC Status", 15},
			{"EDC Condition", 20},
			{"WO Remark", 50},
			{"Latitude", 15},
			{"Longitude", 15},
			{"Ticket Type", 20},
			{"Technician", 25},
			{"SN EDC", 25},
			{"EDC Type", 20},
			{"Received Datetime SPK", 20},
			{"SLA Deadline", 20},
			{"Complete Datetime WO", 20},
			{"First Task Complete Datetime", 20},
			{"First Task Reason", 50},
			{"First Task Reason Code", 30},
			{"First Task Message", 50},
			{"SLA Status", 15},
		}

		var columns []ExcelColumn
		for i, t := range titles {
			columns = append(columns, ExcelColumn{
				ColIndex: fun.GetColName(i),
				ColTitle: t.Title,
				ColSize:  t.ColWidth,
			})
		}

		for _, col := range columns {
			cell := fmt.Sprintf("%s1", col.ColIndex)
			f.SetColWidth(sheetMaster, col.ColIndex, col.ColIndex, col.ColSize)
			f.SetCellValue(sheetMaster, cell, col.ColTitle)
			f.SetCellStyle(sheetMaster, cell, cell, styleTitleMaster)
		}
		lastColMaster := fun.GetColName(len(columns) - 1)
		filterRangeMaster := fmt.Sprintf("A1:%s1", lastColMaster)
		f.AutoFilter(sheetMaster, filterRangeMaster, []excelize.AutoFilterOptions{})

		// Log memory usage for excel generation
		var memStats runtime.MemStats
		runtime.ReadMemStats(&memStats)
		// Force garbage collection to free up memory before data processing
		runtime.GC()

		rowIndex := 2
		const batchSize = 2000
		totalBatches := (int(countData) + batchSize - 1) / batchSize
		type batchResult struct {
			Offset int
			Data   []dspmodel.TicketDSP
			Err    error
		}
		results := make([]batchResult, totalBatches)
		var wg sync.WaitGroup
		for batchNum := 0; batchNum < totalBatches; batchNum++ {
			offset := batchNum * batchSize
			wg.Add(1)
			go func(batchNum, offset int) {
				defer wg.Done()
				var batchData []dspmodel.TicketDSP
				err := dbWeb.Model(&dspmodel.TicketDSP{}).
					Order("id ASC").
					Offset(offset).
					Limit(batchSize).
					Find(&batchData).Error
				results[batchNum] = batchResult{Offset: offset, Data: batchData, Err: err}
			}(batchNum, offset)
		}
		wg.Wait()

		for batchNum, res := range results {
			if res.Err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("Error fetching DSP tickets (batch %d): %v", batchNum+1, res.Err)})
				return
			}
			batchData := res.Data
			if len(batchData) == 0 {
				continue
			}
			processed := res.Offset + len(batchData)
			if processed > int(countData) {
				processed = int(countData)
			}
			logrus.Infof("Batch %d/%d: Processing %d records (offset %d-%d), Memory Usage: Alloc = %v MiB, TotalAlloc = %v MiB, Sys = %v MiB, NumGC = %v",
				batchNum+1, totalBatches, len(batchData), res.Offset+1, processed,
				memStats.Alloc/1024/1024, memStats.TotalAlloc/1024/1024, memStats.Sys/1024/1024, memStats.NumGC)

			for _, record := range batchData {
				for _, column := range columns {
					cell := fmt.Sprintf("%s%d", column.ColIndex, rowIndex)
					var value interface{} = ""
					var needToSetValue bool = true
					switch column.ColTitle {
					case "Ticket Number":
						value = record.Subject
					case "Priority":
						val := record.Priority
						if val != "" {
							switch val {
							case "0":
								value = "☆ ☆ ☆"
							case "1":
								value = "★ ☆ ☆"
							case "2":
								value = "★ ★ ☆"
							case "3":
								value = "★ ★ ★"
							default:
								value = "☆ ☆ ☆"
							}
						}
					case "Stage":
						value = record.Stage
					case "MID":
						value = record.Mid
					case "TID":
						value = record.Tid
					case "WO Number First":
						value = record.WoNumberFirst
					case "WO Number Last":
						value = record.WoNumberLast
					case "Reason Code":
						value = record.ReasonCode
					case "Reason":
						value = record.Reason
					case "Link WOD":
						if record.LinkWod != "" {
							needToSetValue = false
							wo := record.LinkWod
							f.SetCellHyperLink(sheetMaster, cell, wo, "External")
							value = wo
							f.SetCellValue(sheetMaster, cell, value)
						} else {
							value = "No link available"
						}
					case "Merchant Name":
						value = record.MerchantName
					case "PIC Merchant":
						value = record.MerchantPic
					case "PIC Phone Number":
						value = record.PicPhoneNumber
					case "Merchant City":
						value = record.MerchantCity
					case "Merchant Zip Code":
						value = record.MerchantZipCode
					case "Merchant Address":
						value = record.MerchantAddress
					case "Merchant Condition":
						value = record.MerchantCondition
					case "EDC Status":
						value = record.EdcStatus
					case "EDC Condition":
						value = record.EdcCondition
					case "WO Remark":
						value = record.WoRemark
					case "Latitude":
						if record.Latitude != nil {
							value = *record.Latitude
						} else {
							value = "N/A"
						}
					case "Longitude":
						if record.Longitude != nil {
							value = *record.Longitude
						} else {
							value = "N/A"
						}
					case "Ticket Type":
						value = record.TicketType
					case "Technician":
						value = record.Technician
					case "SN EDC":
						value = record.SnEdc
					case "EDC Type":
						value = record.EdcType
					case "Received Datetime SPK":
						if record.ReceivedDatetimeSpk != nil {
							value = record.ReceivedDatetimeSpk.Format("2006-01-02 15:04:05")
						}
					case "SLA Deadline":
						if record.SlaDeadline != nil {
							value = record.SlaDeadline.Format("2006-01-02 15:04:05")
						}
					case "Complete Datetime WO":
						if record.CompleteDatetimeWO != nil {
							value = record.CompleteDatetimeWO.Format("2006-01-02 15:04:05")
						}
					case "First Task Complete Datetime":
						if record.FirstTaskCompleteDatetime != nil {
							value = record.FirstTaskCompleteDatetime.Format("2006-01-02 15:04:05")
						}
					case "First Task Reason":
						value = record.FirstTaskReason
					case "First Task Reason Code":
						value = record.FirstTaskReasonCode
					case "First Task Message":
						value = record.FirstTaskMessage
					case "SLA Status":
						value = record.SLAStatus
					default:
						value = ""
					}
					if needToSetValue {
						if value != "" {
							f.SetCellValue(sheetMaster, cell, value)
							f.SetCellStyle(sheetMaster, cell, cell, style)
						}
					}
				}
				rowIndex++
			}
			if batchNum > 0 && batchNum%(10) == 0 {
				runtime.GC()
			}
		}

		// Pivot
		pivotMasterRange := fmt.Sprintf("%s!$A$1:$%s$%d", sheetMaster, lastColMaster, rowIndex-1)
		pivotRange := fmt.Sprintf("%s!A7:J1000", sheetPivot)
		err = f.AddPivotTable(&excelize.PivotTableOptions{
			Name:            sheetPivot,
			DataRange:       pivotMasterRange,
			PivotTableRange: pivotRange,
			Rows: []excelize.PivotTableField{
				{Data: "TID"},
			},
			Columns: []excelize.PivotTableField{},
			Data: []excelize.PivotTableField{
				{Data: "Complete Datetime WO", Subtotal: "count"},
			},
			Filter: []excelize.PivotTableField{
				{Data: "Priority"},
				{Data: "MID"},
				{Data: "SN EDC"},
				{Data: "Stage"},
			},
			RowGrandTotals:      true,
			ColGrandTotals:      true,
			ShowDrill:           true,
			ShowRowHeaders:      true,
			ShowColHeaders:      true,
			ShowLastColumn:      true,
			PivotTableStyleName: "PivotStyleMedium3",
		})
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Error creating Excel pivot table: " + err.Error()})
			return
		}
		f.SetColWidth(sheetPivot, "A", "A", 40)
		f.SetCellValue(sheetPivot, "A1", "Count of TID visited")
		stylePivotTitle, err := f.NewStyle(&excelize.Style{
			Font: &excelize.Font{
				Bold:      true,
				Underline: "single",
				Size:      13,
			},
		})
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Error creating Excel pivot title style: " + err.Error()})
			return
		}
		f.SetCellStyle(sheetPivot, "A1", "A1", stylePivotTitle)

		f.DeleteSheet("Sheet1")
		if err := f.SaveAs(excelFilePath); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Error creating Excel file: " + err.Error()})
			return
		}

		// Open the file after saving with a 10-minute timeout
		openFileCh := make(chan *os.File)
		openErrCh := make(chan error)
		go func() {
			file, err := os.Open(excelFilePath)
			if err != nil {
				openErrCh <- err
				return
			}
			openFileCh <- file
		}()

		var file *os.File
		select {
		case f := <-openFileCh:
			file = f
			defer file.Close()
		case err := <-openErrCh:
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Error opening Excel file: " + err.Error()})
			return
		case <-time.After(10 * time.Minute):
			c.JSON(http.StatusGatewayTimeout, gin.H{"error": "Timeout opening Excel file."})
			return
		}

		// Set headers for file download
		NewExcelFilename := fmt.Sprintf("Report_All_DSP_Ticket_%s.xlsx", time.Now().Format("02Jan2006_15_04"))
		c.Header("Content-Type", "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet")
		c.Header("Content-Disposition", fmt.Sprintf("attachment; filename=%s", NewExcelFilename))
		c.Status(http.StatusOK)

		// Stream the file content
		if _, err := io.Copy(c.Writer, file); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		}
	}
}

func GetReportTicketDSPFiltered() gin.HandlerFunc {
	return func(c *gin.Context) {
		dbWeb := gormdb.Databases.Web

		var request struct {
			UserID     int    `form:"user_id"`
			Draw       int    `form:"draw"`
			Search     string `form:"search[value]"`
			SortColumn int    `form:"order[0][column]"`
			SortDir    string `form:"order[0][dir]"`

			No       string `form:"no" json:"no"`
			FullName string `form:"full_name" json:"full_name" gorm:"column:full_name"`
		}

		if err := c.Bind(&request); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}

		t := reflect.TypeOf(dspmodel.TicketDSP{})
		columnMap := make(map[int]string)
		colNum := 0
		for i := 0; i < t.NumField(); i++ {
			field := t.Field(i)
			jsonKey := field.Tag.Get("json")
			if jsonKey == "" || jsonKey == "-" || jsonKey == "link_wod" || jsonKey == "location" {
				continue
			}
			columnMap[colNum] = jsonKey
			colNum++
		}

		sortColumnName := columnMap[request.SortColumn]
		var orderString string
		if request.Search == "" {
			orderString = "complete_datetime_wo DESC"
		} else {
			orderString = fmt.Sprintf("%s %s", sortColumnName, request.SortDir)
		}

		filteredQuery := dbWeb.Model(&dspmodel.TicketDSP{})
		if request.Search != "" {
			for i := 0; i < t.NumField(); i++ {
				dataField := ""
				field := t.Field(i)
				dataType := field.Type.String()
				jsonKey := field.Tag.Get("json")
				gormTag := field.Tag.Get("gorm")
				columnKey := ""
				tags := strings.Split(gormTag, ";")
				for _, tag := range tags {
					if strings.HasPrefix(tag, "column:") {
						columnKey = strings.TrimPrefix(tag, "column:")
						break
					}
				}
				if jsonKey == "" || jsonKey == "-" || jsonKey == "link_wod" || jsonKey == "location" {
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
				if formKey == "" || formKey == "-" || formKey == "link_wod" {
					continue
				}
				formValue := c.PostForm(formKey)
				if formValue != "" {
					isHandled := false
					if strings.Contains(formValue, " to ") {
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
						if date, err := time.Parse("02/01/2006", formValue); err == nil {
							filteredQuery = filteredQuery.Where(
								"DATE(`"+formKey+"`) = ?",
								date.Format("2006-01-02"),
							)
							isHandled = true
						}
					}
					if !isHandled {
						filteredQuery = filteredQuery.Where("`"+formKey+"` LIKE ?", "%"+formValue+"%")
					}
				}
			}
		}

		var filteredRecords int64
		filteredQuery.Count(&filteredRecords)
		if filteredRecords == 0 {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "No data found from your request, try use another filter"})
			return
		}

		// Parallel batch fetch for filtered data (fix: use new query for each batch)
		var dbData []dspmodel.TicketDSP
		const fetchBatchSize = 2000
		fetchTotalBatches := int((filteredRecords + int64(fetchBatchSize) - 1) / int64(fetchBatchSize))
		type fetchBatchResult struct {
			Offset int
			Data   []dspmodel.TicketDSP
			Err    error
		}
		fetchResults := make([]fetchBatchResult, fetchTotalBatches)
		var fetchWg sync.WaitGroup
		for batchNum := 0; batchNum < fetchTotalBatches; batchNum++ {
			offset := batchNum * fetchBatchSize
			fetchWg.Add(1)
			go func(batchNum, offset int) {
				defer fetchWg.Done()
				var batch []dspmodel.TicketDSP
				// Always re-apply the same filters for each batch
				q := dbWeb.Model(&dspmodel.TicketDSP{})
				// Re-apply filters from request
				if request.Search != "" {
					for i := 0; i < t.NumField(); i++ {
						dataField := ""
						field := t.Field(i)
						dataType := field.Type.String()
						jsonKey := field.Tag.Get("json")
						gormTag := field.Tag.Get("gorm")
						columnKey := ""
						tags := strings.Split(gormTag, ";")
						for _, tag := range tags {
							if strings.HasPrefix(tag, "column:") {
								columnKey = strings.TrimPrefix(tag, "column:")
								break
							}
						}
						if jsonKey == "" || jsonKey == "-" || jsonKey == "link_wod" || jsonKey == "location" {
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
						q = q.Or("`"+dataField+"` LIKE ?", "%"+request.Search+"%")
					}
				} else {
					for i := 0; i < t.NumField(); i++ {
						field := t.Field(i)
						formKey := field.Tag.Get("json")
						if formKey == "" || formKey == "-" || formKey == "link_wod" {
							continue
						}
						formValue := c.PostForm(formKey)
						if formValue != "" {
							isHandled := false
							if strings.Contains(formValue, " to ") {
								dates := strings.Split(formValue, " to ")
								if len(dates) == 2 {
									from, err1 := time.Parse("02/01/2006", strings.TrimSpace(dates[0]))
									to, err2 := time.Parse("02/01/2006", strings.TrimSpace(dates[1]))
									if err1 == nil && err2 == nil {
										q = q.Where(
											"DATE(`"+formKey+"`) BETWEEN ? AND ?",
											from.Format("2006-01-02"),
											to.Format("2006-01-02"),
										)
										isHandled = true
									}
								}
							} else {
								if date, err := time.Parse("02/01/2006", formValue); err == nil {
									q = q.Where(
										"DATE(`"+formKey+"`) = ?",
										date.Format("2006-01-02"),
									)
									isHandled = true
								}
							}
							if !isHandled {
								q = q.Where("`"+formKey+"` LIKE ?", "%"+formValue+"%")
							}
						}
					}
				}
				err := q.Order(orderString).
					Offset(offset).
					Limit(fetchBatchSize).
					Find(&batch).Error
				fetchResults[batchNum] = fetchBatchResult{Offset: offset, Data: batch, Err: err}
			}(batchNum, offset)
		}
		fetchWg.Wait()
		for _, res := range fetchResults {
			if res.Err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": res.Err.Error()})
				return
			}
			dbData = append(dbData, res.Data...)
		}

		fileDirPaths := []string{
			"../../web/file/excel_report",
			"../web/file/excel_report",
			"web/file/excel_report",
		}
		var existingPath string
		for _, path := range fileDirPaths {
			info, err := os.Stat(path)
			if err == nil && info.IsDir() {
				existingPath = path
				break
			}
		}
		if existingPath == "" {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "No file report directory path valid found!"})
			return
		}

		excelFilename := fmt.Sprintf("(DSP)filtered_data_report_%d_%s.xlsx", request.UserID, time.Now().Format("02Jan2006_15_04"))
		excelFilePath := filepath.Join(existingPath, excelFilename)
		if len(dbData) == 0 {
			c.JSON(http.StatusNotFound, gin.H{"error": "No DSP ticket data available to generate report."})
			return
		}

		f := excelize.NewFile()
		sheetMaster := "MASTER (Filtered)"
		sheetPivot := "PIVOT"
		f.NewSheet(sheetMaster)
		f.NewSheet(sheetPivot)

		style, err := f.NewStyle(&excelize.Style{
			Alignment: &excelize.Alignment{
				Horizontal: "center",
				Vertical:   "center",
			},
		})
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Error creating Excel style: " + err.Error()})
			return
		}

		styleTitleMaster, err := f.NewStyle(styleExcelTitle("#000000", false, "#FFFFFF"))
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Error creating Excel title style: " + err.Error()})
			return
		}

		titles := []struct {
			Title    string
			ColWidth float64
		}{
			{"Ticket Number", 40},
			{"Priority", 15},
			{"Stage", 30},
			{"MID", 20},
			{"TID", 20},
			{"WO Number First", 20},
			{"WO Number Last", 20},
			{"Reason Code", 30},
			{"Reason", 50},
			{"Link WOD", 50},
			{"Merchant Name", 30},
			{"PIC Merchant", 25},
			{"PIC Phone Number", 20},
			{"Merchant City", 20},
			{"Merchant Zip Code", 15},
			{"Merchant Address", 50},
			{"Merchant Condition", 20},
			{"EDC Status", 15},
			{"EDC Condition", 20},
			{"WO Remark", 50},
			{"Latitude", 15},
			{"Longitude", 15},
			{"Ticket Type", 20},
			{"Technician", 25},
			{"SN EDC", 25},
			{"EDC Type", 20},
			{"Received Datetime SPK", 20},
			{"SLA Deadline", 20},
			{"Complete Datetime WO", 20},
			{"First Task Complete Datetime", 20},
			{"First Task Reason", 50},
			{"First Task Reason Code", 30},
			{"First Task Message", 50},
			{"SLA Status", 15},
		}
		var columns []ExcelColumn
		for i, t := range titles {
			columns = append(columns, ExcelColumn{
				ColIndex: fun.GetColName(i),
				ColTitle: t.Title,
				ColSize:  t.ColWidth,
			})
		}
		for _, col := range columns {
			cell := fmt.Sprintf("%s1", col.ColIndex)
			f.SetColWidth(sheetMaster, col.ColIndex, col.ColIndex, col.ColSize)
			f.SetCellValue(sheetMaster, cell, col.ColTitle)
			f.SetCellStyle(sheetMaster, cell, cell, styleTitleMaster)
		}
		lastColMaster := fun.GetColName(len(columns) - 1)
		filterRangeMaster := fmt.Sprintf("A1:%s1", lastColMaster)
		f.AutoFilter(sheetMaster, filterRangeMaster, []excelize.AutoFilterOptions{})

		// Write all data in a single loop
		rowIndex := 2
		for _, record := range dbData {
			for _, column := range columns {
				cell := fmt.Sprintf("%s%d", column.ColIndex, rowIndex)
				var value interface{} = ""
				var needToSetValue bool = true
				switch column.ColTitle {
				case "Ticket Number":
					value = record.Subject
				case "Priority":
					val := record.Priority
					if val != "" {
						switch val {
						case "0":
							value = "☆ ☆ ☆"
						case "1":
							value = "★ ☆ ☆"
						case "2":
							value = "★ ★ ☆"
						case "3":
							value = "★ ★ ★"
						default:
							value = "☆ ☆ ☆"
						}
					}
				case "Stage":
					value = record.Stage
				case "MID":
					value = record.Mid
				case "TID":
					value = record.Tid
				case "WO Number First":
					value = record.WoNumberFirst
				case "WO Number Last":
					value = record.WoNumberLast
				case "Reason Code":
					value = record.ReasonCode
				case "Reason":
					value = record.Reason
				case "Link WOD":
					if record.LinkWod != "" {
						needToSetValue = false
						wo := record.LinkWod
						f.SetCellHyperLink(sheetMaster, cell, wo, "External")
						value = wo
						f.SetCellValue(sheetMaster, cell, value)
					} else {
						value = "No link available"
					}
				case "Merchant Name":
					value = record.MerchantName
				case "PIC Merchant":
					value = record.MerchantPic
				case "PIC Phone Number":
					value = record.PicPhoneNumber
				case "Merchant City":
					value = record.MerchantCity
				case "Merchant Zip Code":
					value = record.MerchantZipCode
				case "Merchant Address":
					value = record.MerchantAddress
				case "Merchant Condition":
					value = record.MerchantCondition
				case "EDC Status":
					value = record.EdcStatus
				case "EDC Condition":
					value = record.EdcCondition
				case "WO Remark":
					value = record.WoRemark
				case "Latitude":
					if record.Latitude != nil {
						value = *record.Latitude
					} else {
						value = "N/A"
					}
				case "Longitude":
					if record.Longitude != nil {
						value = *record.Longitude
					} else {
						value = "N/A"
					}
				case "Ticket Type":
					value = record.TicketType
				case "Technician":
					value = record.Technician
				case "SN EDC":
					value = record.SnEdc
				case "EDC Type":
					value = record.EdcType
				case "Received Datetime SPK":
					if record.ReceivedDatetimeSpk != nil {
						value = record.ReceivedDatetimeSpk.Format("2006-01-02 15:04:05")
					}
				case "SLA Deadline":
					if record.SlaDeadline != nil {
						value = record.SlaDeadline.Format("2006-01-02 15:04:05")
					}
				case "Complete Datetime WO":
					if record.CompleteDatetimeWO != nil {
						value = record.CompleteDatetimeWO.Format("2006-01-02 15:04:05")
					}
				case "First Task Complete Datetime":
					if record.FirstTaskCompleteDatetime != nil {
						value = record.FirstTaskCompleteDatetime.Format("2006-01-02 15:04:05")
					}
				case "First Task Reason":
					value = record.FirstTaskReason
				case "First Task Reason Code":
					value = record.FirstTaskReasonCode
				case "First Task Message":
					value = record.FirstTaskMessage
				case "SLA Status":
					value = record.SLAStatus
				default:
					value = ""
				}
				if needToSetValue {
					if value != "" {
						f.SetCellValue(sheetMaster, cell, value)
						f.SetCellStyle(sheetMaster, cell, cell, style)
					}
				}
			}
			rowIndex++
		}

		pivotMasterRange := fmt.Sprintf("%s!$A$1:$%s$%d", sheetMaster, lastColMaster, rowIndex-1)
		pivotRange := fmt.Sprintf("%s!A7:J1000", sheetPivot)
		err = f.AddPivotTable(&excelize.PivotTableOptions{
			Name:            sheetPivot,
			DataRange:       pivotMasterRange,
			PivotTableRange: pivotRange,
			Rows: []excelize.PivotTableField{
				{Data: "TID"},
			},
			Columns: []excelize.PivotTableField{},
			Data: []excelize.PivotTableField{
				{Data: "Complete Datetime WO", Subtotal: "count"},
			},
			Filter: []excelize.PivotTableField{
				{Data: "Priority"},
				{Data: "MID"},
				{Data: "SN EDC"},
				{Data: "Stage"},
			},
			RowGrandTotals:      true,
			ColGrandTotals:      true,
			ShowDrill:           true,
			ShowRowHeaders:      true,
			ShowColHeaders:      true,
			ShowLastColumn:      true,
			PivotTableStyleName: "PivotStyleMedium3",
		})
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Error creating Excel pivot table: " + err.Error()})
			return
		}
		f.SetColWidth(sheetPivot, "A", "A", 40)
		f.SetCellValue(sheetPivot, "A1", "Count of TID visited")
		stylePivotTitle, err := f.NewStyle(&excelize.Style{
			Font: &excelize.Font{
				Bold:      true,
				Underline: "single",
				Size:      13,
			},
		})
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Error creating Excel pivot title style: " + err.Error()})
			return
		}
		f.SetCellStyle(sheetPivot, "A1", "A1", stylePivotTitle)

		f.DeleteSheet("Sheet1")
		if err := f.SaveAs(excelFilePath); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Error creating Excel file: " + err.Error()})
			return
		}

		openFileCh := make(chan *os.File)
		openErrCh := make(chan error)
		go func() {
			file, err := os.Open(excelFilePath)
			if err != nil {
				openErrCh <- err
				return
			}
			openFileCh <- file
		}()

		var file *os.File
		select {
		case f := <-openFileCh:
			file = f
			defer file.Close()
		case err := <-openErrCh:
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Error opening Excel file: " + err.Error()})
			return
		case <-time.After(10 * time.Minute):
			c.JSON(http.StatusGatewayTimeout, gin.H{"error": "Timeout opening Excel file."})
			return
		}

		NewExcelFilename := fmt.Sprintf("(Filtered)Report_DSP_Ticket_%s.xlsx", time.Now().Format("02Jan2006_15_04"))
		c.Header("Content-Type", "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet")
		c.Header("Content-Disposition", fmt.Sprintf("attachment; filename=%s", NewExcelFilename))
		c.Status(http.StatusOK)

		if _, err := io.Copy(c.Writer, file); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		}
	}
}
