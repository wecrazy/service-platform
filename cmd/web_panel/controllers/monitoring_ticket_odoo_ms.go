package controllers

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"service-platform/cmd/web_panel/config"
	"service-platform/cmd/web_panel/fun"
	"service-platform/cmd/web_panel/internal/gormdb"
	reportmodel "service-platform/cmd/web_panel/model/report_model"
	"strings"
	"sync"
	"time"

	"github.com/sirupsen/logrus"
	"github.com/xuri/excelize/v2"
)

var (
	getMonitoringTicketODOOMSMutex sync.Mutex
)

type MonitoringTicketChartResumeSeries struct {
	ColorLine     string  // Hex color code (6 digits)
	ColorMarker   string  // Hex color code (6 digits)
	MarkerSymbol  string  // Marker symbol for the chart e.g. "circle", "square", "diamond", "triangle", "cross"
	MarkerSize    int     // Size of the marker
	LineThickness float64 // Thickness of the line
}

func MonitoringTicketODOOMS() (string, error) {
	taskDoing := "Show Ticket ODOO MS with Resume Line Charts"
	if !getMonitoringTicketODOOMSMutex.TryLock() {
		return "", fmt.Errorf("%s still running, please wait until it's finished", taskDoing)
	}
	defer getMonitoringTicketODOOMSMutex.Unlock()

	loc, _ := time.LoadLocation(config.GetConfig().Default.Timezone)
	now := time.Now().In(loc)
	numDays := time.Date(now.Year(), now.Month()+1, 0, 0, 0, 0, 0, loc).Day()

	startOfMonth := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, loc)
	endOfMonth := time.Date(now.Year(), now.Month()+1, 0, 23, 59, 59, 999999999, loc)
	startOfMonth = startOfMonth.Add(-7 * time.Hour)
	endOfMonth = endOfMonth.Add(-7 * time.Hour)

	startDateParam := startOfMonth.Format("2006-01-02 15:04:05")
	endDateParam := endOfMonth.Format("2006-01-02 15:04:05")

	if config.GetConfig().Report.MonitoringTicketODOOMS.ActiveDebug {
		if config.GetConfig().Report.MonitoringTicketODOOMS.StartParam != "" {
			startDateParam = config.GetConfig().Report.MonitoringTicketODOOMS.StartParam
		}
		if config.GetConfig().Report.MonitoringTicketODOOMS.EndParam != "" {
			endDateParam = config.GetConfig().Report.MonitoringTicketODOOMS.EndParam
		}
	}

	ODOOModel := "helpdesk.ticket"
	excludedCompany := config.GetConfig().ApiODOO.CompanyExcluded
	domain := []interface{}{
		[]interface{}{"active", "=", true},
		[]interface{}{"x_received_datetime_spk", ">=", startDateParam},
		[]interface{}{"x_received_datetime_spk", "<=", endDateParam},
		[]interface{}{"company_id", "!=", excludedCompany},
	}

	fieldsID := []string{
		"id",
	}

	fields := []string{
		"id",
		"technician_id",
		"name",
		"stage_id",
		"company_id",
		"x_task_type",
		"x_received_datetime_spk",
		"x_sla_deadline",
		"complete_datetime_wo",
		"x_master_mid",
		"x_master_tid",
		"x_merchant",
		"x_merchant_pic",
		"x_merchant_pic_phone",
		"x_studio_alamat",
		"x_partner_latitude",
		"x_partner_longitude",
		"fsm_task_count",
		"x_wo_remark",
		"x_reasoncode",
		"x_link",
		"x_wo_number",
		"x_wo_number_last",
		"x_status_edc",
		"x_status_merchant",
		"x_merchant_tipe_edc",
		"x_merchant_sn_edc",
		"x_source",
		"description",
		"x_paid",
	}
	order := "id asc"

	odooParams := map[string]interface{}{
		"model":  ODOOModel,
		"domain": domain,
		"fields": fieldsID,
		"order":  order,
	}

	payload := map[string]interface{}{
		"jsonrpc": config.GetConfig().ApiODOO.JSONRPC,
		"params":  odooParams,
	}

	payloadBytes, err := json.Marshal(payload)
	if err != nil {
		return "", fmt.Errorf("failed to marshal payload: %w", err)
	}

	ODOOresponse, err := GetODOOMSData(string(payloadBytes))
	if err != nil {
		return "", fmt.Errorf("failed fetching data from ODOO MS API: %w", err)
	}

	ODOOResponseArray, ok := ODOOresponse.([]interface{})
	if !ok {
		errMsg := "failed to asset results as []interface{}"
		return "", errors.New(errMsg)
	}

	ids := extractUniqueIDs(ODOOResponseArray)
	if len(ids) == 0 {
		return "", errors.New("no IDs found from ODOO response")
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
	ctx, cancel := context.WithTimeout(context.Background(), 25*time.Minute)
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
				"jsonrpc": config.GetConfig().ApiODOO.JSONRPC,
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

	dbWeb := gormdb.Databases.Web

	// Clear existing data first before inserting new data
	result := dbWeb.Unscoped().Where("1 = 1").Delete(&reportmodel.MonitoringTicketODOOMS{})
	if result.Error != nil {
		return "", fmt.Errorf("failed to clear existing MonitoringTicketODOOMS records: %v", result.Error)
	}
	logrus.Infof("Deleted %d existing MonitoringTicketODOOMS records", result.RowsAffected)
	logrus.Infof("Trying to insert %d MonitoringTicketODOOMS records", len(listOfData))

	const dbBatchSize = 1000
	dataCount := 0
	for i := 0; i < len(listOfData); i += dbBatchSize {
		end := i + dbBatchSize
		if end > len(listOfData) {
			end = len(listOfData)
		}

		var batch []reportmodel.MonitoringTicketODOOMS
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

			_, companyName, err := parseJSONIDDataCombined(data.CompanyId)
			if err != nil {
				logrus.Error(err)
			}

			_, stage, err := parseJSONIDDataCombined(data.StageId)
			if err != nil {
				logrus.Error(err)
			}

			// _, ticketType, err := parseJSONIDDataCombined(data.TicketTypeId)
			// if err != nil {
			// 	logrus.Error(err)
			// }

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

			slaExpired := SLAExpired(data.SlaDeadline)

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

			isExcludedDataforSLAReport := excludeDataForSLAReport(data, ticketSLAStatus)
			if isExcludedDataforSLAReport {
				continue // Skip this record and do not include in the SLA report
			}

			paid := false
			if data.Paid.Valid {
				paid = data.Paid.Bool
			}

			batch = append(batch, reportmodel.MonitoringTicketODOOMS{
				ID:                      data.ID,
				Technician:              technicianName,
				TicketNumber:            data.TicketSubject.String,
				Stage:                   stage,
				Company:                 companyName,
				TaskType:                data.TaskType.String,
				ReceivedSPKAt:           receivedDatetimeSpk,
				SLADeadline:             slaDeadline,
				CompleteWO:              completeDatetimeWo,
				SLAStatus:               ticketSLAStatus,
				SLAExpired:              slaExpired,
				MID:                     data.Mid.String,
				TID:                     data.Tid.String,
				Merchant:                data.MerchantName.String,
				MerchantPIC:             data.PicMerchant.String,
				MerchantPhone:           data.PicPhone.String,
				MerchantAddress:         data.MerchantAddress.String,
				MerchantLatitude:        merchantLatitude,
				MerchantLongitude:       merchantLongitude,
				TaskCount:               data.TaskCount.Int,
				WORemark:                data.WoRemarkTiket.String,
				ReasonCode:              data.ReasonCode.String,
				FirstJOCompleteDatetime: firstTaskCompleteDatetime,
				FirstJOReason:           firstTaskReason,
				FirstJOMessage:          firstTaskMessage,
				FirstJOReasonCode:       firstReasonCode,
				LinkWO:                  data.LinkWO.String,
				WOFirst:                 data.WOFirst.String,
				WOLast:                  data.WoNumberLast.String,
				StatusEDC:               data.StatusEDC.String,
				KondisiMerchant:         data.StatusMerchant.String,
				EDCType:                 edcType,
				EDCSerial:               snEdc,
				Source:                  data.Source.String,
				Description:             data.Description.String,
				IsPaid:                  paid,
			})

			dataCount++
		}

		if err := dbWeb.Model(&reportmodel.MonitoringTicketODOOMS{}).Create(&batch).Error; err != nil {
			return "", fmt.Errorf("failed to insert MonitoringTicketODOOMS batch starting at index %d: %v", i, err)
		}
	}

	logrus.Infof("Inserted %d MonitoringTicketODOOMS records successfully", len(listOfData))
	logrus.Infof("Total MonitoringTicketODOOMS records processed: %d", dataCount)
	// Log final memory usage
	runtime.ReadMemStats(&memStats)
	logrus.Infof("Final Memory Usage: Alloc = %v MiB, TotalAlloc = %v MiB, Sys = %v MiB, NumGC = %v",
		memStats.Alloc/1024/1024, memStats.TotalAlloc/1024/1024, memStats.Sys/1024/1024, memStats.NumGC)

	// Create report monitoring ticket ODOO MS
	const fetchBatchSize = 1000
	var totalCountPM int64
	var totalCountNonPM int64
	var totalCount int64
	if err := dbWeb.Model(&reportmodel.MonitoringTicketODOOMS{}).
		Where("task_type = ?", "Preventive Maintenance").
		Count(&totalCountPM).Error; err != nil {
		return "", fmt.Errorf("failed to count MonitoringTicketODOOMS PM records: %v", err)
	}
	if err := dbWeb.Model(&reportmodel.MonitoringTicketODOOMS{}).
		Where("task_type != ?", "Preventive Maintenance").
		Count(&totalCountNonPM).Error; err != nil {
		return "", fmt.Errorf("failed to count MonitoringTicketODOOMS Non-PM records: %v", err)
	}
	if err := dbWeb.Model(&reportmodel.MonitoringTicketODOOMS{}).
		Count(&totalCount).Error; err != nil {
		return "", fmt.Errorf("failed to count MonitoringTicketODOOMS total records: %v", err)
	}

	if totalCount == 0 {
		return "", errors.New("no MonitoringTicketODOOMS data available for report generation")
	}

	reportName := fmt.Sprintf("Monitoring_Ticket_%v.xlsx", now.Format("02Jan2006"))
	selectedMainDir, err := fun.FindValidDirectory([]string{
		"web/file/monitoring_ticket",
		"../web/file/monitoring_ticket",
		"../../web/file/monitoring_ticket",
	})
	if err != nil {
		return "", fmt.Errorf("failed to find or create report directory: %v", err)
	}
	fileReportDir := filepath.Join(selectedMainDir, now.Format("2006-01-02"))
	if err := os.MkdirAll(fileReportDir, 0755); err != nil {
		return "", fmt.Errorf("failed to create report directory: %v", err)
	}

	excelFilePath := filepath.Join(fileReportDir, reportName)

	f := excelize.NewFile()

	sheetEmployee := "EMPLOYEES"
	sheetPM := "PM"
	sheetNonPM := "NON-PM"
	sheetMaster := "MASTER"
	sheetPvtPM := "PM PIVOT"
	sheetPvtNonPM := "NON-PM PIVOT"
	sheetResume := "RESUME"
	// sheetChart := "CHART"

	f.NewSheet(sheetEmployee)
	f.NewSheet(sheetPM)
	f.NewSheet(sheetNonPM)
	f.NewSheet(sheetMaster)
	f.NewSheet(sheetPvtPM)
	f.NewSheet(sheetPvtNonPM)
	// f.NewSheet(sheetChart)
	indexSheetResume, err := f.NewSheet(sheetResume)
	if err != nil {
		return "", fmt.Errorf("failed to create resume sheet: %v", err)
	}

	/* Styles */
	style, err := f.NewStyle(&excelize.Style{
		Alignment: &excelize.Alignment{
			Horizontal: "center",
			Vertical:   "center",
		},
	})
	if err != nil {
		return "", fmt.Errorf("failed to create general style: %v", err)
	}

	styleTitleEmployee, err := f.NewStyle(&excelize.Style{
		Alignment: &excelize.Alignment{
			Horizontal: "center",
			Vertical:   "center",
		},
		Fill: excelize.Fill{
			Type:    "pattern",
			Color:   []string{"#fbff00"},
			Pattern: 1,
		},
		Font: &excelize.Font{
			Bold:  true,
			Color: "#000000",
		},
	})
	if err != nil {
		return "", fmt.Errorf("failed to create style for title employee: %v", err)
	}

	styleResumeCaption, err := f.NewStyle(&excelize.Style{
		Font: &excelize.Font{
			Bold:      true,
			Underline: "single",
			Size:      13,
		},
	})
	if err != nil {
		return "", fmt.Errorf("failed to create style for resume title: %v", err)
	}

	styleResumeNo, err := f.NewStyle(&excelize.Style{
		Font: &excelize.Font{
			Bold:  true,
			Color: "#FFFFFF",
		},
		Fill: excelize.Fill{
			Type:    "pattern",
			Color:   []string{"#9BBB59"}, // Green background
			Pattern: 1,
		},
		Alignment: &excelize.Alignment{
			Horizontal: "center",
			Vertical:   "center",
		},
	})
	if err != nil {
		return "", fmt.Errorf("failed to create style for resume no: %v", err)
	}

	styleResumeItem, err := f.NewStyle(&excelize.Style{
		Font: &excelize.Font{
			Bold:  true,
			Color: "#000000",
		},
		Fill: excelize.Fill{
			Type:    "pattern",
			Color:   []string{"#FFFF00"}, // Yellow background
			Pattern: 1,
		},
		Alignment: &excelize.Alignment{
			Horizontal: "center",
			Vertical:   "center",
		},
	})
	if err != nil {
		return "", fmt.Errorf("failed to create style for resume item: %v", err)
	}

	styleResumeDay, err := f.NewStyle(&excelize.Style{
		Font: &excelize.Font{
			Bold:  true,
			Color: "#FFFFFF",
		},
		Fill: excelize.Fill{
			Type:    "pattern",
			Color:   []string{"#808080"}, // Gray background
			Pattern: 1,
		},
		Alignment: &excelize.Alignment{
			Horizontal: "center",
			Vertical:   "center",
		},
	})
	if err != nil {
		return "", fmt.Errorf("failed to create style for resume day: %v", err)
	}

	styleResumeGrandTotal, err := f.NewStyle(&excelize.Style{
		Font: &excelize.Font{
			Bold:  true,
			Color: "#FFFFFF",
		},
		Fill: excelize.Fill{
			Type:    "pattern",
			Color:   []string{"#4F81BD"}, // Blue background
			Pattern: 1,
		},
		Alignment: &excelize.Alignment{
			Horizontal: "center",
			Vertical:   "center",
		},
	})
	if err != nil {
		return "", fmt.Errorf("failed to create style for resume grand total: %v", err)
	}

	// styleItemStage, err := f.NewStyle(&excelize.Style{
	// 	Fill: excelize.Fill{
	// 		Type:    "pattern",
	// 		Color:   []string{"#D4F4AA"}, // Light lime green background
	// 		Pattern: 1,
	// 	},
	// 	Alignment: &excelize.Alignment{
	// 		Horizontal: "left",
	// 		Vertical:   "center",
	// 	},
	// })
	// if err != nil {
	// 	return "", fmt.Errorf("failed to create style for item stage: %v", err)
	// }

	styleItemTotalTicketTitle, err := f.NewStyle(&excelize.Style{
		Fill: excelize.Fill{
			Type:    "pattern",
			Color:   []string{"#2D2DC5"}, // Light purple background
			Pattern: 1,
		},
		Alignment: &excelize.Alignment{
			Horizontal: "left",
			Vertical:   "center",
		},
		Font: &excelize.Font{
			Bold:  true,
			Color: "#FFFFFF",
		},
	})
	if err != nil {
		return "", fmt.Errorf("failed to create style for item total ticket: %v", err)
	}

	// styleItemTotalTicketCount, err := f.NewStyle(&excelize.Style{
	// 	Fill: excelize.Fill{
	// 		Type:    "pattern",
	// 		Color:   []string{"#2D2DC5"}, // Light purple background
	// 		Pattern: 1,
	// 	},
	// 	Alignment: &excelize.Alignment{
	// 		Horizontal: "center",
	// 		Vertical:   "center",
	// 	},
	// 	Font: &excelize.Font{
	// 		Bold:  true,
	// 		Color: "#FFFFFF",
	// 	},
	// })
	// if err != nil {
	// 	return "", fmt.Errorf("failed to create style for item total ticket: %v", err)
	// }

	// styleItemStageAccumulatedTitle, err := f.NewStyle(&excelize.Style{
	// 	Fill: excelize.Fill{
	// 		Type:    "pattern",
	// 		Color:   []string{"#e3e327"}, // Light orange background
	// 		Pattern: 1,
	// 	},
	// 	Alignment: &excelize.Alignment{
	// 		Horizontal: "left",
	// 		Vertical:   "center",
	// 	},
	// 	Font: &excelize.Font{
	// 		Bold:  true,
	// 		Color: "#000000",
	// 	},
	// })
	// if err != nil {
	// 	return "", fmt.Errorf("failed to create style for item stage accumulated: %v", err)
	// }

	styleItemTarget, err := f.NewStyle(&excelize.Style{
		Fill: excelize.Fill{
			Type:    "pattern",
			Color:   []string{"#e00b0b"}, // Solid red background
			Pattern: 1,
		},
		Alignment: &excelize.Alignment{
			Horizontal: "center",
			Vertical:   "center",
		},
		Font: &excelize.Font{
			Bold:  true,
			Color: "#FFFFFF",
		},
	})
	if err != nil {
		return "", fmt.Errorf("failed to create style for item target: %v", err)
	}

	// styleItemTotalAchievement, err := f.NewStyle(&excelize.Style{
	// 	Fill: excelize.Fill{
	// 		Type:    "pattern",
	// 		Color:   []string{"#000000"}, // Solid green background
	// 		Pattern: 1,
	// 	},
	// 	Alignment: &excelize.Alignment{
	// 		Horizontal: "center",
	// 		Vertical:   "center",
	// 	},
	// 	Font: &excelize.Font{
	// 		Bold:  true,
	// 		Color: "#FFFFFF",
	// 	},
	// })
	// if err != nil {
	// 	return "", fmt.Errorf("failed to create style for item total achievement: %v", err)
	// }

	// styleSimplePivotTitle, err := f.NewStyle(&excelize.Style{
	// 	Alignment: &excelize.Alignment{
	// 		Horizontal: "left",
	// 		Vertical:   "center",
	// 	},
	// 	Font: &excelize.Font{
	// 		// Bold: true,
	// 	},
	// })
	// if err != nil {
	// 	return "", fmt.Errorf("failed to create style for simple pivot title: %v", err)
	// }

	// styleSimplePivotValue, err := f.NewStyle(&excelize.Style{
	// 	Alignment: &excelize.Alignment{
	// 		Horizontal: "right",
	// 		Vertical:   "center",
	// 	},
	// 	Font: &excelize.Font{
	// 		Bold: true,
	// 	},
	// })
	// if err != nil {
	// 	return "", fmt.Errorf("failed to create style for simple pivot value: %v", err)
	// }

	styleCountGrandTotalTicketStage, err := f.NewStyle(&excelize.Style{
		Font: &excelize.Font{
			Bold: true,
		},
		Alignment: &excelize.Alignment{
			Horizontal: "center",
			Vertical:   "center",
		},
	})
	if err != nil {
		return "", fmt.Errorf("failed to create style for count grand total ticket stage: %v", err)
	}

	/* * * * * * * * * * * * * * * * * * * * * * * * * * * * * * * * * * * * * * * * * * * * * * * * * * * * * * * * */

	sheetTitleMaster, err := f.NewStyle(&excelize.Style{
		Alignment: &excelize.Alignment{
			Horizontal: "center",
			Vertical:   "center",
		},
		Fill: excelize.Fill{
			Type:    "pattern",
			Color:   []string{"#FF0000"},
			Pattern: 1,
		},
		Font: &excelize.Font{
			Bold:  true,
			Color: "#ffffff",
		},
	})
	if err != nil {
		return "", fmt.Errorf("failed to create style for title master: %v", err)
	}

	/* Employees */
	titleEmployee := []struct {
		Title string
		Size  float64
	}{
		{"Technician", 25},
		{"SPL", 20},
		{"SAC", 20},
	}
	var columnsEmployee []ExcelColumn
	for i, t := range titleEmployee {
		columnsEmployee = append(columnsEmployee, ExcelColumn{
			ColIndex: fun.GetColName(i),
			ColTitle: t.Title,
			ColSize:  t.Size,
		})
	}
	for _, column := range columnsEmployee {
		cell := fmt.Sprintf("%s1", column.ColIndex)
		f.SetCellValue(sheetEmployee, cell, column.ColTitle)
		f.SetColWidth(sheetEmployee, column.ColIndex, column.ColIndex, column.ColSize)
		f.SetCellStyle(sheetEmployee, cell, cell, styleTitleEmployee)
	}

	lastColEmployee := fun.GetColName(len(columnsEmployee) - 1)
	filterRangeEmployee := fmt.Sprintf("A1:%s1", lastColEmployee)
	f.AutoFilter(sheetEmployee, filterRangeEmployee, []excelize.AutoFilterOptions{})

	ODOOModel = "fs.technician"
	excludedTechnicians := []string{
		"Tes Dev Mfjr",
		"Call Center",
		"Teknisi Pameran",
		"Asset Edi Purwanto",
		"Admin AOB",
		"Teknisi BCA",
	}
	domain = []interface{}{
		[]interface{}{"active", "=", true},
		[]interface{}{"name", "!=", excludedTechnicians},
	}
	fields = []string{"id", "name", "technician_code", "x_spl_leader"}
	order = "name asc"

	odooParams = map[string]interface{}{
		"model":  ODOOModel,
		"domain": domain,
		"fields": fields,
		"order":  order,
	}
	payload = map[string]interface{}{
		"jsonrpc": config.GetConfig().ApiODOO.JSONRPC,
		"params":  odooParams,
	}
	payloadBytes, err = json.Marshal(payload)
	if err != nil {
		return "", fmt.Errorf("failed to marshal payload for technician data: %v", err)
	}
	ODOOresponse, err = GetODOOMSData(string(payloadBytes))
	if err != nil {
		return "", fmt.Errorf("failed fetching technician data from ODOO MS API: %v", err)
	}
	ODOOResponseArray, ok = ODOOresponse.([]interface{})
	if !ok {
		return "", errors.New("failed to assert technician results as []interface{}")
	}

	ODOOResponseBytes, err = json.Marshal(ODOOResponseArray)
	if err != nil {
		return "", fmt.Errorf("failed to marshal technician response: %v", err)
	}

	var employeeData []ODOOMSTechnicianItem
	if err := json.Unmarshal(ODOOResponseBytes, &employeeData); err != nil {
		return "", fmt.Errorf("failed to unmarshal technician data: %v", err)
	}

	if len(employeeData) == 0 {
		return "", errors.New("no technician data available from ODOO MS")
	}

	employeeRowIndex := 2
	for _, record := range employeeData {
		for _, column := range columnsEmployee {
			cell := fmt.Sprintf("%s%d", column.ColIndex, employeeRowIndex)
			var value string = "N/A"
			switch column.ColTitle {
			case "Technician":
				if record.NameFS.String != "" {
					value = record.NameFS.String
				}
				f.SetCellValue(sheetEmployee, cell, value)
			case "SPL":
				if record.SPL.String != "" {
					value = record.SPL.String
				}
				f.SetCellValue(sheetEmployee, cell, value)
			case "SAC":
				if record.Head.String != "" {
					value = record.Head.String
				}
				f.SetCellValue(sheetEmployee, cell, value)
			}
		}
		employeeRowIndex++ // increment once per record (row), not per column
	}

	/* PM */
	titlesPM := []struct {
		Title string
		Size  float64
	}{
		{"SAC", 20},
		{"SPL", 25},
		{"Technician", 35},
		{"SPK Number", 55},
		{"Stage", 45},
		{"Company", 25},
		{"Task Type", 25},
		{"Received SPK at", 30},
		{"SLA Deadline", 30},
		{"Complete WO", 30},
		{"SLA Status", 25},
		{"SLA Expired", 25},
		{"MID", 25},
		{"TID", 25},
		{"Merchant", 30},
		{"Merchant PIC", 25},
		{"Merchant Phone", 20},
		{"Merchant Address", 40},
		{"Merchant Latitude", 20},
		{"Merchant Longitude", 20},
		{"Task Count", 15},
		{"WO Remark", 30},
		{"Reason Code", 25},
		{"First JO Complete Datetime", 25},
		{"First JO Reason", 25},
		{"First JO Message", 30},
		{"First JO Reason Code", 25},
		{"Link WO", 30},
		{"WO First", 20},
		{"WO Last", 20},
		{"Status EDC", 20},
		{"Kondisi Merchant", 20},
		{"EDC Type", 20},
		{"EDC Serial", 20},
		{"Source", 15},
		{"Description", 40},
		{"Day Complete", 15},
		{"Day SPK", 15},
	}
	var columnsPM []ExcelColumn
	for i, t := range titlesPM {
		columnsPM = append(columnsPM, ExcelColumn{
			ColIndex: fun.GetColName(i),
			ColTitle: t.Title,
			ColSize:  t.Size,
		})
	}
	for _, column := range columnsPM {
		cell := fmt.Sprintf("%s1", column.ColIndex)
		f.SetCellValue(sheetPM, cell, column.ColTitle)
		f.SetColWidth(sheetPM, column.ColIndex, column.ColIndex, column.ColSize)
		f.SetCellStyle(sheetPM, cell, cell, style)
	}

	lastColPM := fun.GetColName(len(columnsPM) - 1)
	filterRangePM := fmt.Sprintf("A1:%s1", lastColPM)
	f.AutoFilter(sheetPM, filterRangePM, []excelize.AutoFilterOptions{})

	// Get Technician Col Index
	var technicianColIndex string
	for _, col := range columnsPM {
		if col.ColTitle == "Technician" {
			technicianColIndex = col.ColIndex
			break
		}
	}

	var receivedDateSPKColIndex string
	for _, col := range columnsPM {
		if col.ColTitle == "Received SPK at" {
			receivedDateSPKColIndex = col.ColIndex
			break
		}
	}

	var completeWOColIndexPM string
	for _, col := range columnsPM {
		if col.ColTitle == "Complete WO" {
			completeWOColIndexPM = col.ColIndex
			break
		}
	}

	pmRowIndex := 2
	if totalCountPM > 0 {
		for offset := 0; offset < int(totalCountPM); offset += fetchBatchSize {
			var batchData []reportmodel.MonitoringTicketODOOMS

			if err := dbWeb.
				Where("task_type = ?", "Preventive Maintenance").
				Offset(offset).
				Limit(fetchBatchSize).
				Order("received_spk_at asc").
				Find(&batchData).Error; err != nil {
				return "", fmt.Errorf("failed to fetch MonitoringTicketODOOMS PM batch at offset %d: %v", offset, err)
			}

			// Log progress with ID list
			processed := offset + len(batchData)
			if processed > int(totalCountPM) {
				processed = int(totalCountPM)
			}

			batchNumber := (offset / fetchBatchSize) + 1
			totalBatches := (int(totalCountPM) + fetchBatchSize - 1) / fetchBatchSize

			logrus.Infof("Batch %d/%d: Processing %d records (offset %d-%d), Memory Usage: Alloc = %v MiB, TotalAlloc = %v MiB, Sys = %v MiB, NumGC = %v",
				batchNumber, totalBatches, len(batchData), offset+1, processed,
				memStats.Alloc/1024/1024, memStats.TotalAlloc/1024/1024, memStats.Sys/1024/1024, memStats.NumGC)

			// Process each record in the batch
			for _, record := range batchData {
				for _, column := range columnsPM {
					cell := fmt.Sprintf("%s%d", column.ColIndex, pmRowIndex)
					var value interface{} = "N/A"

					var needToSetValue bool = true

					switch column.ColTitle {
					case "SAC":
						needToSetValue = false
						formula := fmt.Sprintf(`IFERROR(VLOOKUP(%s%d, %v!A:C, 3, FALSE), "N/A")`, technicianColIndex, pmRowIndex, sheetEmployee)
						f.SetCellFormula(sheetPM, cell, formula)
					case "SPL":
						needToSetValue = false
						formula := fmt.Sprintf(`IFERROR(VLOOKUP(%s%d, %v!A:C, 2, FALSE), "N/A")`, technicianColIndex, pmRowIndex, sheetEmployee)
						f.SetCellFormula(sheetPM, cell, formula)
					case "Technician":
						if record.Technician != "" {
							value = record.Technician
						}
					case "SPK Number":
						if record.TicketNumber != "" {
							value = record.TicketNumber
						}
					case "Stage":
						if record.Stage != "" {
							value = record.Stage
						}
					case "Company":
						if record.Company != "" {
							value = record.Company
						}
					case "Task Type":
						if record.TaskType != "" {
							value = record.TaskType
						}
					case "Received SPK at":
						if record.ReceivedSPKAt != nil && !record.ReceivedSPKAt.IsZero() {
							value = record.ReceivedSPKAt.Format("2006-01-02 15:04:05")
						}
					case "SLA Deadline":
						if record.SLADeadline != nil && !record.SLADeadline.IsZero() {
							value = record.SLADeadline.Format("2006-01-02 15:04:05")
						}
					case "Complete WO":
						if record.CompleteWO != nil && !record.CompleteWO.IsZero() {
							value = record.CompleteWO.Format("2006-01-02 15:04:05")
						}
					case "SLA Status":
						if record.SLAStatus != "" {
							value = record.SLAStatus
						}
					case "SLA Expired":
						if record.SLAExpired != "" {
							value = record.SLAExpired
						}
					case "MID":
						if record.MID != "" {
							value = record.MID
						}
					case "TID":
						if record.TID != "" {
							value = record.TID
						}
					case "Merchant":
						if record.Merchant != "" {
							value = record.Merchant
						}
					case "Merchant PIC":
						if record.MerchantPIC != "" {
							value = record.MerchantPIC
						}
					case "Merchant Phone":
						if record.MerchantPhone != "" {
							value = record.MerchantPhone
						}
					case "Merchant Address":
						if record.MerchantAddress != "" {
							value = record.MerchantAddress
						}
					case "Merchant Latitude":
						if record.MerchantLatitude != nil {
							value = *record.MerchantLatitude
						}
					case "Merchant Longitude":
						if record.MerchantLongitude != nil {
							value = *record.MerchantLongitude
						}
					case "Task Count":
						if record.TaskCount != 0 {
							value = record.TaskCount
						}
					case "WO Remark":
						if record.WORemark != "" {
							value = record.WORemark
						}
					case "Reason Code":
						if record.ReasonCode != "" {
							value = record.ReasonCode
						}
					case "First JO Complete Datetime":
						if record.FirstJOCompleteDatetime != nil && !record.FirstJOCompleteDatetime.IsZero() {
							value = record.FirstJOCompleteDatetime.Format("2006-01-02 15:04:05")
						}
					case "First JO Reason":
						if record.FirstJOReason != "" {
							value = record.FirstJOReason
						}
					case "First JO Message":
						if record.FirstJOMessage != "" {
							value = record.FirstJOMessage
						}
					case "First JO Reason Code":
						if record.FirstJOReasonCode != "" {
							value = record.FirstJOReasonCode
						}
					case "Link WO":
						if record.LinkWO != "" {
							wo := record.LinkWO
							f.SetCellHyperLink(sheetPM, cell, wo, "External")
							value = "Click to see WOD"
						}
					case "WO First":
						if record.WOFirst != "" {
							value = record.WOFirst
						}
					case "WO Last":
						if record.WOLast != "" {
							value = record.WOLast
						}
					case "Status EDC":
						if record.StatusEDC != "" {
							value = record.StatusEDC
						}
					case "Kondisi Merchant":
						if record.KondisiMerchant != "" {
							value = record.KondisiMerchant
						}
					case "EDC Type":
						if record.EDCType != "" {
							value = record.EDCType
						}
					case "EDC Serial":
						if record.EDCSerial != "" {
							value = record.EDCSerial
						}
					case "Source":
						if record.Source != "" {
							value = record.Source
						}
					case "Description":
						if record.Description != "" {
							value = record.Description
						}
					case "Day Complete":
						needToSetValue = false
						if record.CompleteWO != nil && !record.CompleteWO.IsZero() {
							formula := fmt.Sprintf(`=IFERROR(DAY(%s%d), "N/A")`, completeWOColIndexPM, pmRowIndex)
							f.SetCellFormula(sheetPM, cell, formula)
						} else {
							f.SetCellValue(sheetPM, cell, "N/A")
						}
					case "Day SPK":
						needToSetValue = false
						if record.ReceivedSPKAt != nil && !record.ReceivedSPKAt.IsZero() {
							formula := fmt.Sprintf(`=IFERROR(DAY(%s%d), "N/A")`, receivedDateSPKColIndex, pmRowIndex)
							f.SetCellFormula(sheetPM, cell, formula)
						} else {
							f.SetCellValue(sheetPM, cell, "N/A")
						}
					}

					if needToSetValue {
						f.SetCellValue(sheetPM, cell, value)
						f.SetCellStyle(sheetPM, cell, cell, style)
					}

				}

				pmRowIndex++
			}

			// Optional: Force garbage collection every 10 batches to manage memory
			if offset > 0 && offset%(fetchBatchSize*10) == 0 {
				runtime.GC()
			}
		}
	}

	/* Non-PM */
	titleNonPM := []struct {
		Title string
		Size  float64
	}{
		{"SAC", 20},
		{"SPL", 25},
		{"Technician", 35},
		{"SPK Number", 55},
		{"Stage", 45},
		{"Company", 25},
		{"Task Type", 25},
		{"Received SPK at", 30},
		{"SLA Deadline", 30},
		{"Complete WO", 30},
		{"SLA Status", 25},
		{"SLA Expired", 25},
		{"MID", 25},
		{"TID", 25},
		{"Merchant", 30},
		{"Merchant PIC", 25},
		{"Merchant Phone", 20},
		{"Merchant Address", 40},
		{"Merchant Latitude", 20},
		{"Merchant Longitude", 20},
		{"Task Count", 15},
		{"WO Remark", 30},
		{"Reason Code", 25},
		{"First JO Complete Datetime", 25},
		{"First JO Reason", 25},
		{"First JO Message", 30},
		{"First JO Reason Code", 25},
		{"Link WO", 30},
		{"WO First", 20},
		{"WO Last", 20},
		{"Status EDC", 20},
		{"Kondisi Merchant", 20},
		{"EDC Type", 20},
		{"EDC Serial", 20},
		{"Source", 15},
		{"Description", 40},
		{"Day Complete", 15},
		{"Day SPK", 15},
	}
	var columnsNonPM []ExcelColumn
	for i, t := range titleNonPM {
		columnsNonPM = append(columnsNonPM, ExcelColumn{
			ColIndex: fun.GetColName(i),
			ColTitle: t.Title,
			ColSize:  t.Size,
		})
	}
	for _, column := range columnsNonPM {
		cell := fmt.Sprintf("%s1", column.ColIndex)
		f.SetCellValue(sheetNonPM, cell, column.ColTitle)
		f.SetColWidth(sheetNonPM, column.ColIndex, column.ColIndex, column.ColSize)
		f.SetCellStyle(sheetNonPM, cell, cell, style)
	}

	lastColNonPM := fun.GetColName(len(columnsNonPM) - 1)
	filterRangeNonPM := fmt.Sprintf("A1:%s1", lastColNonPM)
	f.AutoFilter(sheetNonPM, filterRangeNonPM, []excelize.AutoFilterOptions{})

	var completeWOColIndexNonPM string
	for _, col := range columnsNonPM {
		if col.ColTitle == "Complete WO" {
			completeWOColIndexNonPM = col.ColIndex
			break
		}
	}

	nonPMRowIndex := 2
	if totalCountNonPM > 0 {
		for offset := 0; offset < int(totalCountNonPM); offset += fetchBatchSize {
			var batchData []reportmodel.MonitoringTicketODOOMS

			if err := dbWeb.
				Where("task_type != ?", "Preventive Maintenance").
				Offset(offset).
				Limit(fetchBatchSize).
				Order("received_spk_at asc").
				Find(&batchData).Error; err != nil {
				return "", fmt.Errorf("failed to fetch MonitoringTicketODOOMS Non-PM batch at offset %d: %v", offset, err)
			}

			// Log progress with ID list
			processed := offset + len(batchData)
			if processed > int(totalCountNonPM) {
				processed = int(totalCountNonPM)
			}

			batchNumber := (offset / fetchBatchSize) + 1
			totalBatches := (int(totalCountNonPM) + fetchBatchSize - 1) / fetchBatchSize

			logrus.Infof("Batch %d/%d: Processing %d records (offset %d-%d), Memory Usage: Alloc = %v MiB, TotalAlloc = %v MiB, Sys = %v MiB, NumGC = %v",
				batchNumber, totalBatches, len(batchData), offset+1, processed,
				memStats.Alloc/1024/1024, memStats.TotalAlloc/1024/1024, memStats.Sys/1024/1024, memStats.NumGC)

			// Process each record in the batch
			for _, record := range batchData {
				for _, column := range columnsNonPM {
					cell := fmt.Sprintf("%s%d", column.ColIndex, nonPMRowIndex)
					var value interface{} = "N/A"

					var needToSetValue bool = true

					switch column.ColTitle {
					case "SAC":
						needToSetValue = false
						formula := fmt.Sprintf(`IFERROR(VLOOKUP(%s%d, %v!A:C, 3, FALSE), "N/A")`, technicianColIndex, nonPMRowIndex, sheetEmployee)
						f.SetCellFormula(sheetNonPM, cell, formula)
					case "SPL":
						needToSetValue = false
						formula := fmt.Sprintf(`IFERROR(VLOOKUP(%s%d, %v!A:C, 2, FALSE), "N/A")`, technicianColIndex, nonPMRowIndex, sheetEmployee)
						f.SetCellFormula(sheetNonPM, cell, formula)
					case "Technician":
						if record.Technician != "" {
							value = record.Technician
						}
					case "SPK Number":
						if record.TicketNumber != "" {
							value = record.TicketNumber
						}
					case "Stage":
						if record.Stage != "" {
							value = record.Stage
						}
					case "Company":
						if record.Company != "" {
							value = record.Company
						}
					case "Task Type":
						if record.TaskType != "" {
							value = record.TaskType
						}
					case "Received SPK at":
						if record.ReceivedSPKAt != nil && !record.ReceivedSPKAt.IsZero() {
							value = record.ReceivedSPKAt.Format("2006-01-02 15:04:05")
						}
					case "SLA Deadline":
						if record.SLADeadline != nil && !record.SLADeadline.IsZero() {
							value = record.SLADeadline.Format("2006-01-02 15:04:05")
						}
					case "Complete WO":
						if record.CompleteWO != nil && !record.CompleteWO.IsZero() {
							value = record.CompleteWO.Format("2006-01-02 15:04:05")
						}
					case "SLA Status":
						if record.SLAStatus != "" {
							value = record.SLAStatus
						}
					case "SLA Expired":
						if record.SLAExpired != "" {
							value = record.SLAExpired
						}
					case "MID":
						if record.MID != "" {
							value = record.MID
						}
					case "TID":
						if record.TID != "" {
							value = record.TID
						}
					case "Merchant":
						if record.Merchant != "" {
							value = record.Merchant
						}
					case "Merchant PIC":
						if record.MerchantPIC != "" {
							value = record.MerchantPIC
						}
					case "Merchant Phone":
						if record.MerchantPhone != "" {
							value = record.MerchantPhone
						}
					case "Merchant Address":
						if record.MerchantAddress != "" {
							value = record.MerchantAddress
						}
					case "Merchant Latitude":
						if record.MerchantLatitude != nil {
							value = *record.MerchantLatitude
						}
					case "Merchant Longitude":
						if record.MerchantLongitude != nil {
							value = *record.MerchantLongitude
						}
					case "Task Count":
						if record.TaskCount != 0 {
							value = record.TaskCount
						}
					case "WO Remark":
						if record.WORemark != "" {
							value = record.WORemark
						}
					case "Reason Code":
						if record.ReasonCode != "" {
							value = record.ReasonCode
						}
					case "First JO Complete Datetime":
						if record.FirstJOCompleteDatetime != nil && !record.FirstJOCompleteDatetime.IsZero() {
							value = record.FirstJOCompleteDatetime.Format("2006-01-02 15:04:05")
						}
					case "First JO Reason":
						if record.FirstJOReason != "" {
							value = record.FirstJOReason
						}
					case "First JO Message":
						if record.FirstJOMessage != "" {
							value = record.FirstJOMessage
						}
					case "First JO Reason Code":
						if record.FirstJOReasonCode != "" {
							value = record.FirstJOReasonCode
						}
					case "Link WO":
						if record.LinkWO != "" {
							wo := record.LinkWO
							f.SetCellHyperLink(sheetNonPM, cell, wo, "External")
							value = "Click to see WOD"
						}
					case "WO First":
						if record.WOFirst != "" {
							value = record.WOFirst
						}
					case "WO Last":
						if record.WOLast != "" {
							value = record.WOLast
						}
					case "Status EDC":
						if record.StatusEDC != "" {
							value = record.StatusEDC
						}
					case "Kondisi Merchant":
						if record.KondisiMerchant != "" {
							value = record.KondisiMerchant
						}
					case "EDC Type":
						if record.EDCType != "" {
							value = record.EDCType
						}
					case "EDC Serial":
						if record.EDCSerial != "" {
							value = record.EDCSerial
						}
					case "Source":
						if record.Source != "" {
							value = record.Source
						}
					case "Description":
						if record.Description != "" {
							value = record.Description
						}
					case "Day Complete":
						needToSetValue = false
						if record.CompleteWO != nil && !record.CompleteWO.IsZero() {
							formula := fmt.Sprintf(`=IFERROR(DAY(%s%d), "N/A")`, completeWOColIndexNonPM, nonPMRowIndex)
							f.SetCellFormula(sheetNonPM, cell, formula)
						} else {
							f.SetCellValue(sheetNonPM, cell, "N/A")
						}
					case "Day SPK":
						needToSetValue = false
						if record.ReceivedSPKAt != nil && !record.ReceivedSPKAt.IsZero() {
							formula := fmt.Sprintf(`=IFERROR(DAY(%s%d), "N/A")`, receivedDateSPKColIndex, nonPMRowIndex)
							f.SetCellFormula(sheetNonPM, cell, formula)
						} else {
							f.SetCellValue(sheetNonPM, cell, "N/A")
						}
					}

					if needToSetValue {
						f.SetCellValue(sheetNonPM, cell, value)
						f.SetCellStyle(sheetNonPM, cell, cell, style)
					}

				}

				nonPMRowIndex++
			}

			// Optional: Force garbage collection every 10 batches to manage memory
			if offset > 0 && offset%(fetchBatchSize*10) == 0 {
				runtime.GC()
			}
		}
	}

	/* Master */
	titleMaster := []struct {
		Title string
		Size  float64
	}{
		{"SAC", 20},
		{"SPL", 25},
		{"Technician", 35},
		{"SPK Number", 55},
		{"Stage", 45},
		{"Company", 25},
		{"Task Type", 25},
		{"Received SPK at", 30},
		{"SLA Deadline", 30},
		{"Complete WO", 30},
		{"SLA Status", 25},
		{"SLA Expired", 25},
		{"MID", 25},
		{"TID", 25},
		{"Merchant", 30},
		{"Merchant PIC", 25},
		{"Merchant Phone", 20},
		{"Merchant Address", 40},
		{"Merchant Latitude", 20},
		{"Merchant Longitude", 20},
		{"Task Count", 15},
		{"WO Remark", 30},
		{"Reason Code", 25},
		{"First JO Complete Datetime", 25},
		{"First JO Reason", 25},
		{"First JO Message", 30},
		{"First JO Reason Code", 25},
		{"Link WO", 30},
		{"WO First", 20},
		{"WO Last", 20},
		{"Status EDC", 20},
		{"Kondisi Merchant", 20},
		{"EDC Type", 20},
		{"EDC Serial", 20},
		{"Source", 15},
		{"Description", 40},
		{"Day Complete", 15},
		{"Day SPK", 15},
		{"Item", 25},
	}
	var columnsMaster []ExcelColumn
	for i, t := range titleMaster {
		columnsMaster = append(columnsMaster, ExcelColumn{
			ColIndex: fun.GetColName(i),
			ColTitle: t.Title,
			ColSize:  t.Size,
		})
	}
	for _, column := range columnsMaster {
		cell := fmt.Sprintf("%s1", column.ColIndex)
		f.SetCellValue(sheetMaster, cell, column.ColTitle)
		f.SetColWidth(sheetMaster, column.ColIndex, column.ColIndex, column.ColSize)
		f.SetCellStyle(sheetMaster, cell, cell, sheetTitleMaster)
	}

	lastColMaster := fun.GetColName(len(columnsMaster) - 1)
	filterRangeMaster := fmt.Sprintf("A1:%s1", lastColMaster)
	f.AutoFilter(sheetMaster, filterRangeMaster, []excelize.AutoFilterOptions{})

	var daySPKColIndex string
	var companyColIndex, SACColIndex, SPLColIndex string
	var taskTypeColIndex, stageColIndex, slaStatusColIndex, completeWOColIndex string

	for _, col := range columnsMaster {
		if col.ColTitle == "Day SPK" {
			daySPKColIndex = col.ColIndex
		}
		if col.ColTitle == "Company" {
			companyColIndex = col.ColIndex
		}
		if col.ColTitle == "SAC" {
			SACColIndex = col.ColIndex
		}
		if col.ColTitle == "SPL" {
			SPLColIndex = col.ColIndex
		}
		if col.ColTitle == "Task Type" {
			taskTypeColIndex = col.ColIndex
		}
		if col.ColTitle == "Stage" {
			stageColIndex = col.ColIndex
		}
		if col.ColTitle == "SLA Status" {
			slaStatusColIndex = col.ColIndex
		}
		if col.ColTitle == "Complete WO" {
			completeWOColIndex = col.ColIndex
		}
	}

	// Not used, just to avoid compile error
	_ = daySPKColIndex
	_ = companyColIndex
	_ = SACColIndex
	_ = SPLColIndex
	_ = stageColIndex
	_ = taskTypeColIndex
	_ = slaStatusColIndex
	_ = completeWOColIndex

	rowIndex := 2
	if totalCount > 0 {
		for offset := 0; offset < int(totalCount); offset += fetchBatchSize {
			var batchData []reportmodel.MonitoringTicketODOOMS

			if err := dbWeb.
				Offset(offset).
				Limit(fetchBatchSize).
				Order("CASE WHEN task_type = 'Preventive Maintenance' THEN 0 ELSE 1 END, received_spk_at asc").
				Find(&batchData).Error; err != nil {
				return "", fmt.Errorf("failed to fetch MonitoringTicketODOOMS Master batch at offset %d: %v", offset, err)
			}

			// Log progress with ID list
			processed := offset + len(batchData)
			if processed > int(totalCount) {
				processed = int(totalCount)
			}

			batchNumber := (offset / fetchBatchSize) + 1
			totalBatches := (int(totalCount) + fetchBatchSize - 1) / fetchBatchSize

			logrus.Infof("Batch %d/%d: Processing %d records (offset %d-%d), Memory Usage: Alloc = %v MiB, TotalAlloc = %v MiB, Sys = %v MiB, NumGC = %v",
				batchNumber, totalBatches, len(batchData), offset+1, processed,
				memStats.Alloc/1024/1024, memStats.TotalAlloc/1024/1024, memStats.Sys/1024/1024, memStats.NumGC)

			// Process each record in the batch
			for _, record := range batchData {
				for _, column := range columnsMaster {
					cell := fmt.Sprintf("%s%d", column.ColIndex, rowIndex)
					var value interface{} = "N/A"

					var needToSetValue bool = true

					switch column.ColTitle {
					case "SAC":
						needToSetValue = false
						formula := fmt.Sprintf(`IFERROR(VLOOKUP(%s%d, %v!A:C, 3, FALSE), "N/A")`, technicianColIndex, rowIndex, sheetEmployee)
						f.SetCellFormula(sheetMaster, cell, formula)
					case "SPL":
						needToSetValue = false
						formula := fmt.Sprintf(`IFERROR(VLOOKUP(%s%d, %v!A:C, 2, FALSE), "N/A")`, technicianColIndex, rowIndex, sheetEmployee)
						f.SetCellFormula(sheetMaster, cell, formula)
					case "Technician":
						if record.Technician != "" {
							value = record.Technician
						}
					case "SPK Number":
						if record.TicketNumber != "" {
							value = record.TicketNumber
						}
					case "Stage":
						if record.Stage != "" {
							value = record.Stage
						}
					case "Company":
						if record.Company != "" {
							value = record.Company
						}
					case "Task Type":
						if record.TaskType != "" {
							value = record.TaskType
						}
					case "Received SPK at":
						if record.ReceivedSPKAt != nil && !record.ReceivedSPKAt.IsZero() {
							value = record.ReceivedSPKAt.Format("2006-01-02 15:04:05")
						}
					case "SLA Deadline":
						if record.SLADeadline != nil && !record.SLADeadline.IsZero() {
							value = record.SLADeadline.Format("2006-01-02 15:04:05")
						}
					case "Complete WO":
						if record.CompleteWO != nil && !record.CompleteWO.IsZero() {
							value = record.CompleteWO.Format("2006-01-02 15:04:05")
						}
					case "SLA Status":
						if record.SLAStatus != "" {
							value = record.SLAStatus
						}
					case "SLA Expired":
						if record.SLAExpired != "" {
							value = record.SLAExpired
						}
					case "MID":
						if record.MID != "" {
							value = record.MID
						}
					case "TID":
						if record.TID != "" {
							value = record.TID
						}
					case "Merchant":
						if record.Merchant != "" {
							value = record.Merchant
						}
					case "Merchant PIC":
						if record.MerchantPIC != "" {
							value = record.MerchantPIC
						}
					case "Merchant Phone":
						if record.MerchantPhone != "" {
							value = record.MerchantPhone
						}
					case "Merchant Address":
						if record.MerchantAddress != "" {
							value = record.MerchantAddress
						}
					case "Merchant Latitude":
						if record.MerchantLatitude != nil {
							value = *record.MerchantLatitude
						}
					case "Merchant Longitude":
						if record.MerchantLongitude != nil {
							value = *record.MerchantLongitude
						}
					case "Task Count":
						if record.TaskCount != 0 {
							value = record.TaskCount
						}
					case "WO Remark":
						if record.WORemark != "" {
							value = record.WORemark
						}
					case "Reason Code":
						if record.ReasonCode != "" {
							value = record.ReasonCode
						}
					case "First JO Complete Datetime":
						if record.FirstJOCompleteDatetime != nil && !record.FirstJOCompleteDatetime.IsZero() {
							value = record.FirstJOCompleteDatetime.Format("2006-01-02 15:04:05")
						}
					case "First JO Reason":
						if record.FirstJOReason != "" {
							value = record.FirstJOReason
						}
					case "First JO Message":
						if record.FirstJOMessage != "" {
							value = record.FirstJOMessage
						}
					case "First JO Reason Code":
						if record.FirstJOReasonCode != "" {
							value = record.FirstJOReasonCode
						}
					case "Link WO":
						if record.LinkWO != "" {
							wo := record.LinkWO
							f.SetCellHyperLink(sheetMaster, cell, wo, "External")
							value = "Click to see WOD"
						}
					case "WO First":
						if record.WOFirst != "" {
							value = record.WOFirst
						}
					case "WO Last":
						if record.WOLast != "" {
							value = record.WOLast
						}
					case "Status EDC":
						if record.StatusEDC != "" {
							value = record.StatusEDC
						}
					case "Kondisi Merchant":
						if record.KondisiMerchant != "" {
							value = record.KondisiMerchant
						}
					case "EDC Type":
						if record.EDCType != "" {
							value = record.EDCType
						}
					case "EDC Serial":
						if record.EDCSerial != "" {
							value = record.EDCSerial
						}
					case "Source":
						if record.Source != "" {
							value = record.Source
						}
					case "Description":
						if record.Description != "" {
							value = record.Description
						}
					case "Day Complete":
						needToSetValue = false
						if record.CompleteWO != nil && !record.CompleteWO.IsZero() {
							formula := fmt.Sprintf(`=IFERROR(DAY(%s%d), "N/A")`, completeWOColIndex, rowIndex)
							f.SetCellFormula(sheetMaster, cell, formula)
						} else {
							f.SetCellValue(sheetMaster, cell, "N/A")
						}
					case "Day SPK":
						needToSetValue = false
						if record.ReceivedSPKAt != nil && !record.ReceivedSPKAt.IsZero() {
							formula := fmt.Sprintf(`=IFERROR(DAY(%s%d), "N/A")`, receivedDateSPKColIndex, rowIndex)
							f.SetCellFormula(sheetMaster, cell, formula)
						} else {
							f.SetCellValue(sheetMaster, cell, "N/A")
						}
					case "Item":
						if strings.TrimSpace(record.TaskType) == "Preventive Maintenance" {
							value = "PM - " + record.Stage
						} else {
							value = "Non-PM " + record.Stage
						}
					}

					if needToSetValue {
						f.SetCellValue(sheetMaster, cell, value)
						f.SetCellStyle(sheetMaster, cell, cell, style)
					}

				}

				rowIndex++
			}

			// Optional: Force garbage collection every 10 batches to manage memory
			if offset > 0 && offset%(fetchBatchSize*10) == 0 {
				runtime.GC()
			}
		}
	}

	/* PIVOT */
	pvtPMMasterRange := fmt.Sprintf("%s!$A$1:$%s$%d", sheetPM, lastColPM, pmRowIndex-1)
	pvtPMRange := fmt.Sprintf("%s!A8:AB20", sheetPvtPM)
	err = f.AddPivotTable(&excelize.PivotTableOptions{
		Name:            sheetPvtPM,
		DataRange:       pvtPMMasterRange,
		PivotTableRange: pvtPMRange,
		Rows: []excelize.PivotTableField{
			{Data: "Stage"},
		},
		Columns: []excelize.PivotTableField{
			{Data: "Day SPK"},
		},
		Data: []excelize.PivotTableField{
			{Data: "Complete WO", Subtotal: "count", Name: "Total Completed PM"},
		},
		Filter: []excelize.PivotTableField{
			{Data: "Company"},
			{Data: "SAC"},
			{Data: "SPL"},
			{Data: "Technician"},
			{Data: "SLA Status"},
		},
		RowGrandTotals:      true,
		ColGrandTotals:      true,
		ShowDrill:           true,
		ShowRowHeaders:      true,
		ShowColHeaders:      true,
		ShowLastColumn:      true,
		PivotTableStyleName: "PivotStyleMedium8",
	})
	f.SetColWidth(sheetPvtPM, "A", "A", 30)
	if err != nil {
		return "", fmt.Errorf("failed to create pivot table PM: %v", err)
	}

	pvtNonPMMasterRange := fmt.Sprintf("%s!$A$1:$%s$%d", sheetNonPM, lastColNonPM, nonPMRowIndex-1)
	pvtNonPMRange := fmt.Sprintf("%s!A8:AB20", sheetPvtNonPM)
	err = f.AddPivotTable(&excelize.PivotTableOptions{
		Name:            sheetPvtNonPM,
		DataRange:       pvtNonPMMasterRange,
		PivotTableRange: pvtNonPMRange,
		Rows: []excelize.PivotTableField{
			{Data: "Stage"},
		},
		Columns: []excelize.PivotTableField{
			{Data: "Day SPK"},
		},
		Data: []excelize.PivotTableField{
			{Data: "Complete WO", Subtotal: "count", Name: "Total Completed Non-PM"},
		},
		Filter: []excelize.PivotTableField{
			{Data: "Company"},
			{Data: "SAC"},
			{Data: "SPL"},
			{Data: "Technician"},
			{Data: "SLA Status"},
		},
		RowGrandTotals:      true,
		ColGrandTotals:      true,
		ShowDrill:           true,
		ShowRowHeaders:      true,
		ShowColHeaders:      true,
		ShowLastColumn:      true,
		PivotTableStyleName: "PivotStyleMedium13",
	})
	f.SetColWidth(sheetPvtNonPM, "A", "A", 30)
	if err != nil {
		return "", fmt.Errorf("failed to create pivot table Non-PM: %v", err)
	}

	/* RESUME */
	f.SetCellValue(sheetResume, "A1", fmt.Sprintf("SUMMARY REPORT PERFORMANCE %v", strings.ToUpper(now.Format("January 2006"))))
	f.SetCellStyle(sheetResume, "A1", "A1", styleResumeCaption)

	startRowResume := 17
	titleResume := []struct {
		Title string
		Size  float64
	}{
		{"No", 12},
		{"Item", 40},
	}
	for day := 1; day <= numDays; day++ {
		titleResume = append(titleResume, struct {
			Title string
			Size  float64
		}{
			Title: fmt.Sprintf("%d", day),
			Size:  10,
		})
	}
	titleResume = append(titleResume, struct {
		Title string
		Size  float64
	}{
		Title: "Grand Total",
		Size:  20,
	})

	var columnsResume []ExcelColumn
	for i, t := range titleResume {
		columnsResume = append(columnsResume, ExcelColumn{
			ColIndex: fun.GetColName(i),
			ColTitle: t.Title,
			ColSize:  t.Size,
		})
	}
	for _, column := range columnsResume {
		cell := fmt.Sprintf("%s%d", column.ColIndex, startRowResume)
		f.SetCellValue(sheetResume, cell, column.ColTitle)
		f.SetColWidth(sheetResume, column.ColIndex, column.ColIndex, column.ColSize)
		switch column.ColTitle {
		case "Grand Total":
			f.SetCellStyle(sheetResume, cell, cell, styleResumeGrandTotal)
		case "No":
			f.SetCellStyle(sheetResume, cell, cell, styleResumeNo)
		case "Item":
			f.SetCellStyle(sheetResume, cell, cell, styleResumeItem)
		default: // Days
			f.SetCellStyle(sheetResume, cell, cell, styleResumeDay)
		}
	}

	lastColResume := fun.GetColName(len(columnsResume) - 1) // -1 means its got grand total, -2 means last day
	filterRangeResume := fmt.Sprintf("A%d:%s%d", startRowResume, lastColResume, startRowResume)
	f.AutoFilter(sheetResume, filterRangeResume, []excelize.AutoFilterOptions{})

	resumeRowIndex := startRowResume + 1
	no := 1
	// Define items to match GetDataTicketPerformance()
	resumeItems := []string{
		"Total PM",
		"Total CM",
		"Total Non-PM",
		"Total Ticket",
		"Target",
		"Total Achievement",
		"Total Meet SLA",
		"Total Overdue",
		"Total Done",
		"Total Pending On Target",
		"Total Pending Overdue Visited",
		"Total Pending Overdue New",
		"Total Pending Not Visit",
	}

	// Track row indices for items using a map
	itemRows := make(map[string]int)
	for _, item := range resumeItems {
		cellA := fmt.Sprintf("A%d", resumeRowIndex)
		cellB := fmt.Sprintf("B%d", resumeRowIndex)
		f.SetCellValue(sheetResume, cellA, no)
		f.SetCellStyle(sheetResume, cellA, cellA, style)
		f.SetCellValue(sheetResume, cellB, item)
		f.SetCellStyle(sheetResume, cellB, cellB, styleItemTotalTicketTitle) // reuse style

		// Store item row index
		itemRows[item] = resumeRowIndex

		no++
		resumeRowIndex++
	}

	// Filters ####################################################################################################
	// --- Company dropdown (from DB) ---
	f.SetCellValue(sheetResume, "B3", "Company:")
	f.SetCellValue(sheetResume, "C3", "All")
	var companies []string
	err = dbWeb.Model(&reportmodel.MonitoringTicketODOOMS{}).
		Distinct("company").
		Pluck("company", &companies).Error
	if err != nil {
		return "", fmt.Errorf("failed to fetch distinct companies: %v", err)
	}
	companies = append([]string{"All"}, companies...)

	// --- SAC dropdown (col C from EMPLOYEE) ---
	f.SetCellValue(sheetResume, "B4", "SAC:")
	f.SetCellValue(sheetResume, "C4", "All")

	colsEmp, _ := f.GetCols(sheetEmployee)
	var sacs []string
	if len(colsEmp) > 2 { // col C = index 2
		uniq := map[string]bool{}
		sacs = append(sacs, "All")
		for _, v := range colsEmp[2][1:] { // skip header
			if v != "" && !uniq[v] {
				uniq[v] = true
				sacs = append(sacs, v)
			}
		}
	}

	// --- SPL dropdown (col B from EMPLOYEE) ---
	f.SetCellValue(sheetResume, "B5", "SPL:")
	f.SetCellValue(sheetResume, "C5", "All")

	var spls []string
	if len(colsEmp) > 1 { // col B = index 1
		uniq := map[string]bool{}
		spls = append(spls, "All")
		for _, v := range colsEmp[1][1:] { // skip header
			if v != "" && !uniq[v] {
				uniq[v] = true
				spls = append(spls, v)
			}
		}
	}

	// --- Technician dropdown (from DB) ---
	f.SetCellValue(sheetResume, "B6", "Technician:")
	f.SetCellValue(sheetResume, "C6", "All")
	var technicians []string
	err = dbWeb.Model(&reportmodel.MonitoringTicketODOOMS{}).
		Distinct("technician").
		Pluck("technician", &technicians).Error
	if err != nil {
		return "", fmt.Errorf("failed to fetch distinct technicians: %v", err)
	}
	technicians = append([]string{"All"}, technicians...)

	// -------------------------------
	// Write lists into helper columns
	// -------------------------------
	helperStartIndex := len(columnsResume) + 3
	colComp := fun.GetColName(helperStartIndex)     // e.g. Z
	colSac := fun.GetColName(helperStartIndex + 1)  // AA
	colSpl := fun.GetColName(helperStartIndex + 2)  // AB
	colTech := fun.GetColName(helperStartIndex + 3) // AC
	for i, v := range companies {
		f.SetCellValue(sheetResume, fmt.Sprintf("%s%d", colComp, i+1), v)
	}
	for i, v := range sacs {
		f.SetCellValue(sheetResume, fmt.Sprintf("%s%d", colSac, i+1), v)
	}
	for i, v := range spls {
		f.SetCellValue(sheetResume, fmt.Sprintf("%s%d", colSpl, i+1), v)
	}
	for i, v := range technicians {
		f.SetCellValue(sheetResume, fmt.Sprintf("%s%d", colTech, i+1), v)
	}

	// -------------------------------
	// Data Validations (dropdowns)
	// -------------------------------
	// Company
	dv := excelize.NewDataValidation(true)
	dv.Sqref = "C3"
	dv.SetSqrefDropList(fmt.Sprintf("%s!$%s$1:$%s$%d", sheetResume, colComp, colComp, len(companies)))
	f.AddDataValidation(sheetResume, dv)

	// SAC
	dv = excelize.NewDataValidation(true)
	dv.Sqref = "C4"
	dv.SetSqrefDropList(fmt.Sprintf("%s!$%s$1:$%s$%d", sheetResume, colSac, colSac, len(sacs)))
	f.AddDataValidation(sheetResume, dv)

	// SPL
	dv = excelize.NewDataValidation(true)
	dv.Sqref = "C5"
	dv.SetSqrefDropList(fmt.Sprintf("%s!$%s$1:$%s$%d", sheetResume, colSpl, colSpl, len(spls)))
	f.AddDataValidation(sheetResume, dv)

	// Technician
	dv = excelize.NewDataValidation(true)
	dv.Sqref = "C6"
	dv.SetSqrefDropList(fmt.Sprintf("%s!$%s$1:$%s$%d", sheetResume, colTech, colTech, len(technicians)))
	f.AddDataValidation(sheetResume, dv)

	// Hide helper columns
	f.SetColWidth(sheetResume, colComp, colTech, 0)
	// ###############################################################################################################

	// ---------------------------------------------------------------------------------------------------------------
	// Get Count of Tickets by day
	lastMasterDataRow := rowIndex - 1
	companyCol := companyColIndex
	sacCol := SACColIndex
	splCol := SPLColIndex
	techCol := technicianColIndex

	// Calculate cumulative values for each item (skip Target - calculated later)
	for _, item := range resumeItems {
		if item == "Target" {
			continue // Skip Target, it will be calculated after Total Ticket
		}

		row := itemRows[item]
		for day := 1; day <= numDays; day++ {
			cell := fmt.Sprintf("%s%d", fun.GetColName(day+1), row)

			if day == 1 {
				// Day 1: Get the count for day 1
				formula := buildItemFormula(sheetMaster, daySPKColIndex, companyCol, sacCol, splCol, techCol, stageColIndex, taskTypeColIndex, slaStatusColIndex, completeWOColIndex, day, lastMasterDataRow, item)
				if err := f.SetCellFormula(sheetResume, cell, formula); err != nil {
					return "", fmt.Errorf("set %s formula %s failed: %v", item, cell, err)
				}
			} else {
				// Day 2+: Previous day + count for current day
				prevDayCell := fmt.Sprintf("%s%d", fun.GetColName(day), row)
				currentDayFormula := buildItemFormula(sheetMaster, daySPKColIndex, companyCol, sacCol, splCol, techCol, stageColIndex, taskTypeColIndex, slaStatusColIndex, completeWOColIndex, day, lastMasterDataRow, item)
				formula := fmt.Sprintf("%s+%s", prevDayCell, currentDayFormula)
				if err := f.SetCellFormula(sheetResume, cell, formula); err != nil {
					return "", fmt.Errorf("set cumulative %s formula %s failed: %v", item, cell, err)
				}
			}
			f.SetCellStyle(sheetResume, cell, cell, style)
		}

		// Grand total: last day's value
		grandCol := fun.GetColName(numDays + 2)
		cellGrand := fmt.Sprintf("%s%d", grandCol, row)
		lastDayCol := fun.GetColName(numDays + 1)
		formulaGrand := fmt.Sprintf("=%s%d", lastDayCol, row)
		if err := f.SetCellFormula(sheetResume, cellGrand, formulaGrand); err != nil {
			return "", fmt.Errorf("set grand %s formula %s failed: %v", item, cellGrand, err)
		}
		f.SetCellStyle(sheetResume, cellGrand, cellGrand, styleCountGrandTotalTicketStage) // reuse style
	} // ---------------------------------------------------------------------------------------------------------------

	// +++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++
	// Summary section showing key metrics
	summaryRow := 3
	f.SetColWidth(sheetResume, "E", "E", 40)

	// Number of Days
	f.SetCellValue(sheetResume, fmt.Sprintf("E%d", summaryRow), "Number of Days")
	f.SetCellValue(sheetResume, fmt.Sprintf("F%d", summaryRow), numDays)
	summaryRow++

	// Total Ticket (final count from last day)
	totalTicketRow := itemRows["Total Ticket"]
	grandCol := fun.GetColName(numDays + 2)
	f.SetCellValue(sheetResume, fmt.Sprintf("E%d", summaryRow), "Total Ticket")
	f.SetCellFormula(sheetResume, fmt.Sprintf("F%d", summaryRow), fmt.Sprintf("=%s%d", grandCol, totalTicketRow))
	summaryNumDaysRow := 3
	summaryTotalTicketRow := summaryRow
	summaryRow++

	// Target per Day
	f.SetCellValue(sheetResume, fmt.Sprintf("E%d", summaryRow), "Target per Day")
	f.SetCellFormula(sheetResume, fmt.Sprintf("F%d", summaryRow), fmt.Sprintf("=ROUNDUP(F%d/F%d, 0)", summaryTotalTicketRow, summaryNumDaysRow))
	summaryTargetPerDayRow := summaryRow
	// +++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++

	// Calculate Target based on Target per Day from summary
	targetRow := itemRows["Target"]
	targetPerDayCell := fmt.Sprintf("$F$%d", summaryTargetPerDayRow)

	for day := 1; day <= numDays; day++ {
		cell := fmt.Sprintf("%s%d", fun.GetColName(day+1), targetRow)

		if day == 1 {
			formula := fmt.Sprintf("=%s", targetPerDayCell)
			if err := f.SetCellFormula(sheetResume, cell, formula); err != nil {
				return "", fmt.Errorf("set target day 1 formula %s failed: %v", cell, err)
			}
		} else {
			prevDayCell := fmt.Sprintf("%s%d", fun.GetColName(day), targetRow)
			formula := fmt.Sprintf("=%s+%s", prevDayCell, targetPerDayCell)
			if err := f.SetCellFormula(sheetResume, cell, formula); err != nil {
				return "", fmt.Errorf("set target cumulative formula %s failed: %v", cell, err)
			}
		}
		f.SetCellStyle(sheetResume, cell, cell, styleItemTarget)
	}

	// Grand total for TARGET
	cellGrand := fmt.Sprintf("%s%d", grandCol, targetRow)
	formulaGrand := fmt.Sprintf("=%s*%d", targetPerDayCell, numDays)
	if err := f.SetCellFormula(sheetResume, cellGrand, formulaGrand); err != nil {
		return "", fmt.Errorf("set target grand formula %s failed: %v", cellGrand, err)
	}
	f.SetCellStyle(sheetResume, cellGrand, cellGrand, styleItemTarget)

	//////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////
	// Resume Chart
	lastColResume = fun.GetColName(len(columnsResume) - 2) // use for last day column

	lineItemSeriesMap := map[string]MonitoringTicketChartResumeSeries{
		"Total PM":                      {ColorLine: "#FF6B6B", ColorMarker: "#FF5252", MarkerSymbol: "circle", MarkerSize: 8, LineThickness: 2},    // Red
		"Total CM":                      {ColorLine: "#4ECDC4", ColorMarker: "#45B7AA", MarkerSymbol: "circle", MarkerSize: 8, LineThickness: 2},    // Teal
		"Total Non-PM":                  {ColorLine: "#FFA07A", ColorMarker: "#FF8C69", MarkerSymbol: "circle", MarkerSize: 8, LineThickness: 2},    // Light Salmon
		"Total Ticket":                  {ColorLine: "#87CEEB", ColorMarker: "#87CEEB", MarkerSymbol: "diamond", MarkerSize: 10, LineThickness: 3},  // Light blue
		"Target":                        {ColorLine: "#FF0000", ColorMarker: "#FF0000", MarkerSymbol: "square", MarkerSize: 10, LineThickness: 5},   // Red
		"Total Achievement":             {ColorLine: "#000000", ColorMarker: "#000000", MarkerSymbol: "triangle", MarkerSize: 10, LineThickness: 5}, // Black
		"Total Meet SLA":                {ColorLine: "#27AE60", ColorMarker: "#229954", MarkerSymbol: "circle", MarkerSize: 8, LineThickness: 2},    // Green
		"Total Overdue":                 {ColorLine: "#E74C3C", ColorMarker: "#C0392B", MarkerSymbol: "circle", MarkerSize: 8, LineThickness: 2},    // Red
		"Total Done":                    {ColorLine: "#9B59B6", ColorMarker: "#8E44AD", MarkerSymbol: "circle", MarkerSize: 8, LineThickness: 2},    // Purple
		"Total Pending On Target":       {ColorLine: "#F39C12", ColorMarker: "#E67E22", MarkerSymbol: "circle", MarkerSize: 8, LineThickness: 2},    // Orange
		"Total Pending Overdue Visited": {ColorLine: "#16A085", ColorMarker: "#138D75", MarkerSymbol: "circle", MarkerSize: 8, LineThickness: 2},    // Teal
		"Total Pending Overdue New":     {ColorLine: "#95A5A6", ColorMarker: "#7F8C8D", MarkerSymbol: "circle", MarkerSize: 8, LineThickness: 2},    // Gray
		"Total Pending Not Visit":       {ColorLine: "#BDC3C7", ColorMarker: "#A6ACAF", MarkerSymbol: "circle", MarkerSize: 8, LineThickness: 2},    // Light Gray
	}

	// Notes in the resumrowindex + 3
	notesRow := resumeRowIndex + 3
	notesRichText := []excelize.RichTextRun{
		{Text: "Notes:\n", Font: &excelize.Font{Bold: true}},
		{Text: "- The 'Total PM' line represents the cumulative count of Preventive Maintenance tickets.\n", Font: &excelize.Font{Size: 11}},
		{Text: "- The 'Total CM' line represents the cumulative count of Corrective Maintenance tickets.\n", Font: &excelize.Font{Size: 11}},
		{Text: "- The 'Total Non-PM' line represents the cumulative count of Non-Preventive Maintenance tickets.\n", Font: &excelize.Font{Size: 11}},
		{Text: "- The 'Total Ticket' line represents the cumulative count of all tickets.\n", Font: &excelize.Font{Size: 11}},
		{Text: "- The 'Target' line indicates the expected cumulative ticket count.\n", Font: &excelize.Font{Size: 11}},
		{Text: "- The 'Total Achievement' line represents the cumulative count of completed tickets (stage != 'Cancel').\n", Font: &excelize.Font{Size: 11}},
		{Text: "- Other lines show cumulative counts for various SLA and status categories.\n", Font: &excelize.Font{Size: 11}},
	}
	if err := f.SetCellRichText(sheetResume, fmt.Sprintf("A%d", notesRow), notesRichText); err != nil {
		return "", fmt.Errorf("set notes rich text failed: %v", err)
	}

	// Create line chart starting from row resumeRowIndex + 4, column A
	chartStartRow := notesRow + 1

	startColCategory := "C" // Day 1
	categoriesRange := fmt.Sprintf("%s!%s%d:%s%d", sheetResume, startColCategory, startRowResume, lastColResume, startRowResume)

	// Collect series for all resume items
	var allSeries []excelize.ChartSeries
	for _, item := range resumeItems {
		valueRange := fmt.Sprintf("%s!%s%d:%s%d", sheetResume, startColCategory, itemRows[item], lastColResume, itemRows[item])
		nameRange := fmt.Sprintf("%s!$B$%d", sheetResume, itemRows[item])

		dataSeries := lineItemSeriesMap[item]

		var marker excelize.ChartMarker
		if dataSeries.MarkerSymbol != "" {
			marker.Symbol = dataSeries.MarkerSymbol
		}
		marker.Size = dataSeries.MarkerSize
		if dataSeries.ColorMarker != "" {
			marker.Fill = excelize.Fill{
				Type:    "pattern",
				Pattern: 1,
				Color:   []string{dataSeries.ColorMarker},
			}
		}

		var chartSeries excelize.ChartSeries
		chartSeries.Name = nameRange
		chartSeries.Categories = categoriesRange
		chartSeries.Values = valueRange
		chartSeries.Marker = marker
		chartSeries.Line = excelize.ChartLine{
			Width:  dataSeries.LineThickness,
			Smooth: true,
		}
		if dataSeries.ColorLine != "" {
			chartSeries.Fill = excelize.Fill{
				Type:    "pattern",
				Pattern: 1,
				Color:   []string{dataSeries.ColorLine},
			}
		}

		allSeries = append(allSeries, chartSeries)
	}

	chartCell := fmt.Sprintf("A%d", chartStartRow)

	// Create the line chart
	err = f.AddChart(sheetResume, chartCell, &excelize.Chart{
		Type:   excelize.Line,
		Series: allSeries,
		Format: excelize.GraphicOptions{
			OffsetX: 15,
			OffsetY: 10,
			ScaleX:  1.0,
			ScaleY:  1.0,
		},
		Title: []excelize.RichTextRun{
			{Text: fmt.Sprintf("ACHIEVEMENTS %s", strings.ToUpper(now.Format("January 2006")))},
		},
		PlotArea: excelize.ChartPlotArea{
			ShowDataTable:     true,
			ShowDataTableKeys: true,
			ShowSerName:       false,
		},
		ShowBlanksAs: "zero",
		Dimension: excelize.ChartDimension{
			Width:  3300,
			Height: 900,
		},
		Legend: excelize.ChartLegend{
			Position:      "top",
			ShowLegendKey: false,
		},
	})
	if err != nil {
		return "", fmt.Errorf("failed to create resume chart: %v", err)
	}

	//// Copy chart to a new sheet "Chart" ///////////////////////////////////////////////////////////////////////////////////////
	// err = f.AddChart(sheetChart, "A1", &excelize.Chart{
	// 	Type:   excelize.Line,
	// 	Series: allSeries,
	// 	Format: excelize.GraphicOptions{
	// 		OffsetX: 15,
	// 		OffsetY: 10,
	// 		ScaleX:  1.0,
	// 		ScaleY:  0.3,
	// 	},
	// 	Title: []excelize.RichTextRun{
	// 		{Text: fmt.Sprintf("ACHIEVEMENT %s", strings.ToUpper(now.Format("January 2006")))},
	// 	},
	// 	PlotArea: excelize.ChartPlotArea{
	// 		ShowDataTable:     true,
	// 		ShowDataTableKeys: true,
	// 		ShowSerName:       false,
	// 	},
	// 	ShowBlanksAs: "zero",
	// 	Dimension: excelize.ChartDimension{
	// 		Width:  config.GetConfig().Report.MonitoringTicketODOOMS.ChartWidth,
	// 		Height: config.GetConfig().Report.MonitoringTicketODOOMS.ChartHeight,
	// 	},
	// 	Legend: excelize.ChartLegend{
	// 		Position:      "top",
	// 		ShowLegendKey: false,
	// 	},
	// })
	// if err != nil {
	// 	return "", fmt.Errorf("failed to create chart in Chart sheet: %v", err)
	// }
	//////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////

	f.DeleteSheet("Sheet1")
	f.SetActiveSheet(indexSheetResume - 1)
	if err := f.SaveAs(excelFilePath); err != nil {
		return "", fmt.Errorf("failed to save initial Excel file: %v", err)
	}

	return excelFilePath, nil
}

// buildSumproductFormulaOfCountTicketStageByDay builds a single-line SUMPRODUCT formula using bounded ranges.
// dayCol = e.g. "AL", stageCol = e.g. "E" (for stage column)
// func buildSumproductFormulaOfCountTicketStageByDay(sheetMaster, dayCol, companyCol, sacCol, splCol, techCol, stageCol string, row, day, lastRow int, stage string) string {
// 	_ = row // row is not used but kept for signature consistency

// 	// build bounded range strings once
// 	dayRange := fmt.Sprintf("%s!$%s$2:$%s$%d", sheetMaster, dayCol, dayCol, lastRow)
// 	companyRange := fmt.Sprintf("%s!$%s$2:$%s$%d", sheetMaster, companyCol, companyCol, lastRow)
// 	sacRange := fmt.Sprintf("%s!$%s$2:$%s$%d", sheetMaster, sacCol, sacCol, lastRow)
// 	splRange := fmt.Sprintf("%s!$%s$2:$%s$%d", sheetMaster, splCol, splCol, lastRow)
// 	techRange := fmt.Sprintf("%s!$%s$2:$%s$%d", sheetMaster, techCol, techCol, lastRow)
// 	stageRange := fmt.Sprintf("%s!$%s$2:$%s$%d", sheetMaster, stageCol, stageCol, lastRow)

// 	// conditions (use UPPER(TRIM()) to normalize values and use the (>0) trick for OR with "All")
// 	conds := []string{}

// 	// Day condition (only when day>0)
// 	if day > 0 {
// 		conds = append(conds, fmt.Sprintf("--(%s=%d)", dayRange, day))
// 	}

// 	// Stage match (compare normalized stage text in MASTER to the specific stage)
// 	conds = append(conds, fmt.Sprintf("--(UPPER(TRIM(%s))=UPPER(TRIM(\"%s\")))", stageRange, stage))

// 	// Company filter: ( ($C$3="All") OR (normalized company = normalized $C$3) )
// 	conds = append(conds, fmt.Sprintf("--((UPPER(TRIM($C$3))=\"ALL\")+(UPPER(TRIM(%s))=UPPER(TRIM($C$3)))>0)", companyRange))

// 	// SAC filter (cell C4)
// 	conds = append(conds, fmt.Sprintf("--((UPPER(TRIM($C$4))=\"ALL\")+(UPPER(TRIM(%s))=UPPER(TRIM($C$4)))>0)", sacRange))

// 	// SPL filter (cell C5)
// 	conds = append(conds, fmt.Sprintf("--((UPPER(TRIM($C$5))=\"ALL\")+(UPPER(TRIM(%s))=UPPER(TRIM($C$5)))>0)", splRange))

// 	// Technician filter (cell C6)
// 	conds = append(conds, fmt.Sprintf("--((UPPER(TRIM($C$6))=\"ALL\")+(UPPER(TRIM(%s))=UPPER(TRIM($C$6)))>0)", techRange))

// 	// join with * and return single-line SUMPRODUCT
// 	return fmt.Sprintf("SUMPRODUCT(%s)", strings.Join(conds, "*"))
// }

// buildItemFormula builds a SUMPRODUCT formula for the given item
func buildItemFormula(sheetMaster, dayCol, companyCol, sacCol, splCol, techCol, stageCol, taskTypeCol, slaStatusCol, completeWOCol string, day, lastRow int, item string) string {
	// build bounded range strings once
	dayRange := fmt.Sprintf("%s!$%s$2:$%s$%d", sheetMaster, dayCol, dayCol, lastRow)
	companyRange := fmt.Sprintf("%s!$%s$2:$%s$%d", sheetMaster, companyCol, companyCol, lastRow)
	sacRange := fmt.Sprintf("%s!$%s$2:$%s$%d", sheetMaster, sacCol, sacCol, lastRow)
	splRange := fmt.Sprintf("%s!$%s$2:$%s$%d", sheetMaster, splCol, splCol, lastRow)
	techRange := fmt.Sprintf("%s!$%s$2:$%s$%d", sheetMaster, techCol, techCol, lastRow)
	stageRange := fmt.Sprintf("%s!$%s$2:$%s$%d", sheetMaster, stageCol, stageCol, lastRow)
	taskTypeRange := fmt.Sprintf("%s!$%s$2:$%s$%d", sheetMaster, taskTypeCol, taskTypeCol, lastRow)
	slaStatusRange := fmt.Sprintf("%s!$%s$2:$%s$%d", sheetMaster, slaStatusCol, slaStatusCol, lastRow)
	completeWORange := fmt.Sprintf("%s!$%s$2:$%s$%d", sheetMaster, completeWOCol, completeWOCol, lastRow)

	// conditions
	conds := []string{}

	// Day condition (only when day>0)
	if day > 0 {
		conds = append(conds, fmt.Sprintf("--(%s=%d)", dayRange, day))
	}

	// Item-specific conditions
	switch item {
	case "Total PM":
		conds = append(conds, fmt.Sprintf("--(UPPER(TRIM(%s))=UPPER(TRIM(\"Preventive Maintenance\")))", taskTypeRange))
	case "Total CM":
		conds = append(conds, fmt.Sprintf("--(UPPER(TRIM(%s))=UPPER(TRIM(\"Corrective Maintenance\")))", taskTypeRange))
	case "Total Non-PM":
		// Match all tickets where task_type is NOT "Preventive Maintenance"
		conds = append(conds, fmt.Sprintf("--(UPPER(TRIM(%s))<>UPPER(TRIM(\"Preventive Maintenance\")))", taskTypeRange))
	case "Total Ticket":
		// All tickets, no additional condition
	case "Total Achievement":
		conds = append(conds, fmt.Sprintf("--(%s<>\"\")", completeWORange))
		conds = append(conds, fmt.Sprintf("--(UPPER(TRIM(%s))<>UPPER(TRIM(\"Cancel\")))", stageRange))
	case "Total Meet SLA":
		conds = append(conds, fmt.Sprintf("--(%s<>\"\")", completeWORange))
		conds = append(conds, fmt.Sprintf("--(UPPER(TRIM(%s))=UPPER(TRIM(\"On Target Solved\")))", slaStatusRange))
		conds = append(conds, fmt.Sprintf("--(UPPER(TRIM(%s))<>UPPER(TRIM(\"Cancel\")))", stageRange))
	case "Total Overdue":
		// Match any SLA status starting with "Overdue"
		conds = append(conds, fmt.Sprintf("--(%s<>\"\")", completeWORange))
		// Use OR logic: Overdue (Visited) + Overdue (New) + other Overdue variations
		conds = append(conds, fmt.Sprintf("--((UPPER(TRIM(%s))=UPPER(TRIM(\"Overdue (Visited)\")))+(UPPER(TRIM(%s))=UPPER(TRIM(\"Overdue (New)\")))>0)", slaStatusRange, slaStatusRange))
		conds = append(conds, fmt.Sprintf("--(UPPER(TRIM(%s))<>UPPER(TRIM(\"Cancel\")))", stageRange))
	case "Total Done":
		// Match multiple stages - use OR logic with addition
		conds = append(conds, fmt.Sprintf("--(%s<>\"\")", completeWORange))
		// Sum up conditions: (stage=Solved)+(stage=Pending)+(stage=Solved Pending)+... > 0
		doneStages := []string{"Solved", "Pending", "Solved Pending", "Done", "Closed", "Waiting For Verification"}
		var stageChecks []string
		for _, s := range doneStages {
			stageChecks = append(stageChecks, fmt.Sprintf("(UPPER(TRIM(%s))=UPPER(TRIM(\"%s\")))", stageRange, s))
		}
		conds = append(conds, fmt.Sprintf("--(%s>0)", strings.Join(stageChecks, "+")))
	case "Total Pending On Target":
		conds = append(conds, fmt.Sprintf("--(UPPER(TRIM(%s))=UPPER(TRIM(\"Pending\")))", stageRange))
		conds = append(conds, fmt.Sprintf("--(UPPER(TRIM(%s))=UPPER(TRIM(\"On Target Solved\")))", slaStatusRange))
	case "Total Pending Overdue Visited":
		conds = append(conds, fmt.Sprintf("--(UPPER(TRIM(%s))=UPPER(TRIM(\"Pending\")))", stageRange))
		conds = append(conds, fmt.Sprintf("--(UPPER(TRIM(%s))=UPPER(TRIM(\"Overdue (Visited)\")))", slaStatusRange))
	case "Total Pending Overdue New":
		conds = append(conds, fmt.Sprintf("--(UPPER(TRIM(%s))=UPPER(TRIM(\"Pending\")))", stageRange))
		conds = append(conds, fmt.Sprintf("--(UPPER(TRIM(%s))=UPPER(TRIM(\"Overdue (New)\")))", slaStatusRange))
	case "Total Pending Not Visit":
		conds = append(conds, fmt.Sprintf("--(UPPER(TRIM(%s))=UPPER(TRIM(\"Pending\")))", stageRange))
		conds = append(conds, fmt.Sprintf("--(UPPER(TRIM(%s))=UPPER(TRIM(\"Not Visit\")))", slaStatusRange))
	}

	// Filter conditions
	conds = append(conds, fmt.Sprintf("--((UPPER(TRIM($C$3))=\"ALL\")+(UPPER(TRIM(%s))=UPPER(TRIM($C$3)))>0)", companyRange))
	conds = append(conds, fmt.Sprintf("--((UPPER(TRIM($C$4))=\"ALL\")+(UPPER(TRIM(%s))=UPPER(TRIM($C$4)))>0)", sacRange))
	conds = append(conds, fmt.Sprintf("--((UPPER(TRIM($C$5))=\"ALL\")+(UPPER(TRIM(%s))=UPPER(TRIM($C$5)))>0)", splRange))
	conds = append(conds, fmt.Sprintf("--((UPPER(TRIM($C$6))=\"ALL\")+(UPPER(TRIM(%s))=UPPER(TRIM($C$6)))>0)", techRange))

	// join with * and return single-line SUMPRODUCT
	return fmt.Sprintf("SUMPRODUCT(%s)", strings.Join(conds, "*"))
}

func GenerateChartMonitoringTicketPerformanceODOOMSInBackground(excelFilePath string, senderJIDs []string) {
	// This function runs in a separate goroutine to avoid blocking the main flow.
	// It generates a chart image from an Excel file and sends it via WhatsApp.

	// Trying to generate the chart img in background
	selectedMainDir, err := fun.FindValidDirectory([]string{
		"web/file/monitoring_ticket",
		"../web/file/monitoring_ticket",
		"../../web/file/monitoring_ticket",
	})
	if err != nil {
		logrus.Error(err)
		return
	}
	fileReportDir := filepath.Join(selectedMainDir, time.Now().Format("2006-01-02"))
	if err := os.MkdirAll(fileReportDir, 0755); err != nil {
		logrus.Error(err)
		return
	}
	imgOutput := filepath.Join(fileReportDir, fmt.Sprintf("chartReportSummaryTicketAchievement_%s.png", time.Now().Format("02Jan2006")))

	logFile, _ := os.Create(config.GetConfig().Report.MonitoringTicketODOOMS.LogExportChartDebugPath)
	if logFile != nil {
		defer logFile.Close()
	}

	logExport := func(msg string) {
		if logFile != nil {
			logFile.WriteString(time.Now().Format("15:04:05") + " " + msg + "\n")
		}
	}

	runtime.LockOSThread()
	defer runtime.UnlockOSThread()

	var tempExcel, tempPDF string
	defer func() {
		if r := recover(); r != nil {
			logExport(fmt.Sprintf("🔥 Panic: %v", r))
		}
		if tempExcel != "" {
			os.Remove(tempExcel)
		}
		if tempPDF != "" {
			os.Remove(tempPDF)
		}
	}()

	// ✅ Step 1: Copy original Excel to temp file
	tempExcel = filepath.Join(os.TempDir(), fmt.Sprintf("temp_chart_%d.xlsx", time.Now().UnixNano()))
	if err := CopyFile(excelFilePath, tempExcel); err != nil {
		logExport(fmt.Sprintf("❌ Failed to copy excel to temp for chart: %v", err))
		return
	}
	logExport(fmt.Sprintf("📄 Temp Excel for chart created at %s", tempExcel))

	// Wait for Unlock
	if err = WaitForFileUnlock(tempExcel, 10*time.Minute); err != nil {
		logExport(fmt.Sprintf("❌ Failed to wait for file unlock: %v", err))
		return
	}

	logExport("🔓 File is now unlocked, proceeding...")

	// ✅ Step 2: Open temp Excel
	f, err := excelize.OpenFile(tempExcel)
	if err != nil {
		logExport(fmt.Sprintf("❌ Failed to open temp excel file: %v", err))
		return
	}
	defer f.Close()

	// ✅ Step 3: Hide all sheets except for "CHART"
	sheetNameToKeep := "CHART"
	for _, sheet := range f.GetSheetList() {
		if sheet != sheetNameToKeep {
			if err := f.SetSheetVisible(sheet, false); err != nil {
				logExport(fmt.Sprintf("❌ Failed to hide sheet %s: %v", sheet, err))
			}
		}
	}
	logExport(fmt.Sprintf("ሉ All sheets except '%s' have been hidden.", sheetNameToKeep))

	// Set the "CHART" sheet as the active one
	idx, err := f.GetSheetIndex(sheetNameToKeep)
	if err != nil {
		logExport(fmt.Sprintf("❌ Failed to get index for sheet %s: %v", sheetNameToKeep, err))
		return
	}
	f.SetActiveSheet(idx)

	// ✅ Step 4: Set the print area to ensure the chart is captured.
	if err := f.SetDefinedName(&excelize.DefinedName{
		Name:     "_xlnm.Print_Area",
		RefersTo: fmt.Sprintf("'%s'!$A$1:$BZ$50", sheetNameToKeep),
		Scope:    sheetNameToKeep,
	}); err != nil {
		logExport(fmt.Sprintf("❌ Failed to set defined name for print area: %v", err))
		return
	}

	// ✅ Step 5: Set page layout to control size and scaling
	orientation := "landscape"
	paperSize := 8 // A3 paper
	var adjustTo uint = 100
	if err := f.SetPageLayout(sheetNameToKeep, &excelize.PageLayoutOptions{
		Orientation: &orientation,
		Size:        &paperSize,
		AdjustTo:    &adjustTo,
	}); err != nil {
		logExport(fmt.Sprintf("❌ Failed to set page layout: %v", err))
		return
	}
	if err := f.Save(); err != nil {
		logExport(fmt.Sprintf("❌ Failed to save temp excel: %v", err))
		return
	}

	// Convert to PDF with timeout
	baseName := strings.TrimSuffix(filepath.Base(tempExcel), ".xlsx")
	tempPDF = filepath.Join(os.TempDir(), baseName+".pdf")
	libreCmd := exec.Command("libreoffice", "--headless", "--convert-to", "pdf", "--outdir", os.TempDir(), tempExcel)
	libreCmd.Env = append(os.Environ(), "HOME=/tmp")
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()
	libreCmd = exec.CommandContext(ctx, "libreoffice", "--headless", "--convert-to", "pdf", "--outdir", os.TempDir(), tempExcel)
	libreCmd.Env = append(os.Environ(), "HOME=/tmp")

	out, err := libreCmd.CombinedOutput()
	if ctx.Err() == context.DeadlineExceeded {
		logExport("❌ LibreOffice conversion timed out")
		return
	}
	if err != nil {
		logExport(fmt.Sprintf("❌ LibreOffice conversion failed: %v, output: %s", err, string(out)))
		return
	}
	logExport("✅ PDF created via LibreOffice")

	absImg, _ := filepath.Abs(imgOutput)
	magickPath, err := exec.LookPath("convert")
	if err != nil {
		magickPath = config.GetConfig().Default.MagickFullPath
		if _, statErr := os.Stat(magickPath); os.IsNotExist(statErr) {
			logExport("❌ ImageMagick 'convert' not found in PATH or at configured location")
			return
		}
	}

	ctxConvert, cancelConvert := context.WithTimeout(context.Background(), 25*time.Minute)
	defer cancelConvert()
	convertCMD := exec.CommandContext(ctxConvert, magickPath,
		"-density", "300",
		tempPDF+"[0]",
		"-quality", "90",
		"-sharpen", "0x1.0",
		absImg,
	)
	// Set MAGICK_TMPDIR to a writable location
	convertCMD.Env = append(os.Environ(), "MAGICK_TMPDIR="+os.TempDir())

	convertOut, err := convertCMD.CombinedOutput()
	if ctxConvert.Err() == context.DeadlineExceeded {
		logExport("❌ ImageMagick convert timed out")
		return
	}
	if err != nil {
		logExport(fmt.Sprintf("❌ ImageMagick convert failed: %v, output: %s", err, string(convertOut)))
		return
	}
	logExport(fmt.Sprintf("✅ ImageMagick convert succeeded, image at %s", absImg))

	fileInfo, err := os.Stat(absImg)
	if err != nil {
		logExport(fmt.Sprintf("❌ Failed to stat image file: %v", err))
		return
	}
	logExport(fmt.Sprintf("🖼️ Image file size: %d bytes", fileInfo.Size()))

	if fileInfo.Size() == 0 {
		logExport("❌ Image file size is 0 bytes, something went wrong")
		return
	}

	// Send the image via WhatsApp
	idMsg := "Berikut chart Summary Ticket Achievements:"
	enMsg := "Here is the chart of Summary Ticket Achievements:"
	for _, jid := range senderJIDs {
		SendLangDocumentViaBotWhatsapp(jid, idMsg, enMsg, "id", absImg)
		logExport(fmt.Sprintf("✅ Image sent successfully via WhatsApp to %s", jid))
		os.Remove(absImg) // Clean up image after sending
	}

}

func GenerateExcelChartMonitoringTicketPerformanceODOOMSInBackground(excelFilePath string, senderJIDs []string) {
	// This function runs in a separate goroutine to avoid blocking the main flow.
	// It generates a chart image from an Excel file and sends it via WhatsApp.

	// Trying to generate the chart img in background
	selectedMainDir, err := fun.FindValidDirectory([]string{
		"web/file/monitoring_ticket",
		"../web/file/monitoring_ticket",
		"../../web/file/monitoring_ticket",
	})
	if err != nil {
		logrus.Error(err)
		return
	}
	fileReportDir := filepath.Join(selectedMainDir, time.Now().Format("2006-01-02"))
	if err := os.MkdirAll(fileReportDir, 0755); err != nil {
		logrus.Error(err)
		return
	}

	logFile, _ := os.Create(config.GetConfig().Report.MonitoringTicketODOOMS.LogExportChartDebugPath)
	if logFile != nil {
		defer logFile.Close()
	}

	logExport := func(msg string) {
		if logFile != nil {
			logFile.WriteString(time.Now().Format("15:04:05") + " " + msg + "\n")
		}
	}

	runtime.LockOSThread()
	defer runtime.UnlockOSThread()

	var tempExcel, tempPDF string
	defer func() {
		if r := recover(); r != nil {
			logExport(fmt.Sprintf("🔥 Panic: %v", r))
		}
		if tempExcel != "" {
			os.Remove(tempExcel)
		}
		if tempPDF != "" {
			os.Remove(tempPDF)
		}
	}()

	// ✅ Step 1: Copy original Excel to temp file
	tempExcel = filepath.Join(os.TempDir(), fmt.Sprintf("chartReportSummaryTicketAchievement_%v.xlsx", time.Now().Format("02Jan2006")))
	if err := CopyFile(excelFilePath, tempExcel); err != nil {
		logExport(fmt.Sprintf("❌ Failed to copy excel to temp for chart: %v", err))
		return
	}
	logExport(fmt.Sprintf("📄 Temp Excel for chart created at %s", tempExcel))

	// Wait for Unlock
	if err = WaitForFileUnlock(tempExcel, 10*time.Minute); err != nil {
		logExport(fmt.Sprintf("❌ Failed to wait for file unlock: %v", err))
		return
	}

	logExport("🔓 File is now unlocked, proceeding...")

	// ✅ Step 2: Open temp Excel
	f, err := excelize.OpenFile(tempExcel)
	if err != nil {
		logExport(fmt.Sprintf("❌ Failed to open temp excel file: %v", err))
		return
	}
	defer f.Close()

	// ✅ Step 3: Hide all sheets except for "CHART"
	sheetNameToKeep := "CHART"
	for _, sheet := range f.GetSheetList() {
		if sheet != sheetNameToKeep {
			if err := f.SetSheetVisible(sheet, false); err != nil {
				logExport(fmt.Sprintf("❌ Failed to hide sheet %s: %v", sheet, err))
			}
		}
	}
	logExport(fmt.Sprintf("ሉ All sheets except '%s' have been hidden.", sheetNameToKeep))

	// Set the "CHART" sheet as the active one
	idx, err := f.GetSheetIndex(sheetNameToKeep)
	if err != nil {
		logExport(fmt.Sprintf("❌ Failed to get index for sheet %s: %v", sheetNameToKeep, err))
		return
	}
	f.SetActiveSheet(idx)

	// ✅ Step 4: Set the print area to ensure the chart is captured.
	if err := f.SetDefinedName(&excelize.DefinedName{
		Name:     "_xlnm.Print_Area",
		RefersTo: fmt.Sprintf("'%s'!$A$1:$BZ$50", sheetNameToKeep),
		Scope:    sheetNameToKeep,
	}); err != nil {
		logExport(fmt.Sprintf("❌ Failed to set defined name for print area: %v", err))
		return
	}

	// ✅ Step 5: Set page layout to control size and scaling
	orientation := "landscape"
	paperSize := 8 // A3 paper
	var adjustTo uint = 100
	if err := f.SetPageLayout(sheetNameToKeep, &excelize.PageLayoutOptions{
		Orientation: &orientation,
		Size:        &paperSize,
		AdjustTo:    &adjustTo,
	}); err != nil {
		logExport(fmt.Sprintf("❌ Failed to set page layout: %v", err))
		return
	}
	if err := f.Save(); err != nil {
		logExport(fmt.Sprintf("❌ Failed to save temp excel: %v", err))
		return
	}

	idMsg := "Berikut chart Summary Ticket Achievements:"
	enMsg := "Here is the chart of Summary Ticket Achievements:"
	for _, jid := range senderJIDs {
		SendLangDocumentViaBotWhatsapp(jid, idMsg, enMsg, "id", tempExcel)
		logExport(fmt.Sprintf("✅ Excel sent successfully via WhatsApp to %s", jid))
	}
}

func BackupTableMonitoringTicket() error {
	dbWeb := gormdb.Databases.Web
	table := config.GetConfig().Database.TbReportMonitoringTicket

	// Get current month and year
	now := time.Now()
	month := now.Format("Jan") // 01-12
	year := now.Format("2006") // 4-digit year
	backupTable := strings.ToLower(fmt.Sprintf("%s_%s%s", table, month, year))

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

	logrus.Infof("Backup table %s created successfully with %d records", backupTable, count)
	return nil
}
