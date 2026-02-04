package controllers

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"service-platform/cmd/web_panel/fun"
	"service-platform/cmd/web_panel/internal/gormdb"
	reportmodel "service-platform/cmd/web_panel/model/report_model"
	"service-platform/internal/config"
	"sync"
	"time"

	"github.com/sirupsen/logrus"
	"github.com/xuri/excelize/v2"
)

var (
	getDataODOOMSSLAReportMutex sync.Mutex
)

func GetDataSLAReportODOOMS() error {
	taskDoing := "get Data for SLA Report ODOOMS"
	if !getDataODOOMSSLAReportMutex.TryLock() {
		return fmt.Errorf("%s is still running, please wait until it's finished", taskDoing)
	}
	defer getDataODOOMSSLAReportMutex.Unlock()

	loc, _ := time.LoadLocation(config.WebPanel.Get().Default.Timezone)
	now := time.Now().In(loc)
	startOfMonth := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, loc)
	endOfMonth := time.Date(now.Year(), now.Month()+1, 0, 23, 59, 59, 999999999, loc)
	startOfMonth = startOfMonth.Add(-7 * time.Hour)
	endOfMonth = endOfMonth.Add(-7 * time.Hour)
	startDateParam := startOfMonth.Format("2006-01-02 15:04:05")
	endDateParam := endOfMonth.Format("2006-01-02 15:04:05")

	if config.WebPanel.Get().Report.SLA.ActiveDebug {
		startDateParam = config.WebPanel.Get().Report.SLA.StartParam
		endDateParam = config.WebPanel.Get().Report.SLA.EndParam
	}

	ODOOModel := "helpdesk.ticket"
	excludedCompanies := config.WebPanel.Get().ApiODOO.CompanyExcluded
	excludedTechnicians := []string{
		"Tes Dev Mfjr",
		"Asset Edi Purwanto",
	}

	domain := []interface{}{
		[]interface{}{"active", "=", true},
		[]interface{}{"technician_id", "!=", excludedTechnicians},
		[]interface{}{"x_sla_deadline", ">=", startDateParam},
		[]interface{}{"x_sla_deadline", "<=", endDateParam},
		[]interface{}{"company_id", "!=", excludedCompanies},
	}

	fieldID := []string{"id"}

	fields := []string{
		"id",
		"name",
		"create_date",
		"stage_id",
		"technician_id",
		"company_id",
		"x_task_type",
		"x_received_datetime_spk",
		"x_sla_deadline",
		"complete_datetime_wo",
		"x_master_mid",
		"x_master_tid",
		"fsm_task_count",
		"x_merchant",
		"x_merchant_pic",
		"x_merchant_pic_phone",
		"x_studio_alamat",
		"x_partner_latitude",
		"x_partner_longitude",
		"x_link",
		"x_wo_remark",
		"x_wo_number",
		"x_wo_number_last",
		"x_status_edc",
		"x_status_merchant",
		"x_merchant_tipe_edc",
		"x_merchant_sn_edc",
		"x_source",
		"x_reasoncode",
		"description",
		"ticket_type_id",
	}

	order := "id asc"
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
		return fmt.Errorf("failed to marshal ODOO payload: %v", err)
	}

	ODOOresponse, err := GetODOOMSData(string(payloadBytes))
	if err != nil {
		return fmt.Errorf("failed fetching data from ODOO MS API: %v", err)
	}

	ODOOResponseArray, ok := ODOOresponse.([]interface{})
	if !ok {
		return errors.New("type assertion failed for ODOO response")
	}

	ids := extractUniqueIDs(ODOOResponseArray)

	if len(ids) == 0 {
		return fmt.Errorf("%s - no data found, exiting", taskDoing)
	}

	const batchSize = 1000
	chunks := chunkIdsSlice(ids, batchSize)
	var allRecords []interface{}
	var allRecordsMutex sync.Mutex
	var wg sync.WaitGroup
	semaphore := make(chan struct{}, 4) // Increase worker pool for more concurrency if memory allows

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Minute)
	defer cancel()

	var chunkErrors []error
	var chunkErrorsMutex sync.Mutex

	for i, chunk := range chunks {
		wg.Add(1)
		go func(chunkIdx int, chunkData []uint64) {
			logrus.Debugf("Starting chunk %d/%d", chunkIdx+1, len(chunks))
			defer wg.Done()
			select {
			case semaphore <- struct{}{}:
			case <-ctx.Done():
				chunkErrorsMutex.Lock()
				chunkErrors = append(chunkErrors, fmt.Errorf("timeout acquiring semaphore"))
				chunkErrorsMutex.Unlock()
				logrus.Debugf("Chunk %d/%d: timeout acquiring semaphore", chunkIdx+1, len(chunks))
				return
			}
			defer func() { <-semaphore }()

			defer func() {
				if r := recover(); r != nil {
					chunkErrorsMutex.Lock()
					chunkErrors = append(chunkErrors, fmt.Errorf("panic in chunk: %v", r))
					chunkErrorsMutex.Unlock()
					logrus.Debugf("Chunk %d/%d: panic: %v", chunkIdx+1, len(chunks), r)
				}
			}()

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
				chunkErrorsMutex.Lock()
				chunkErrors = append(chunkErrors, fmt.Errorf("failed to marshal payload: %v", err))
				chunkErrorsMutex.Unlock()
				logrus.Debugf("Chunk %d/%d: failed to marshal payload: %v", chunkIdx+1, len(chunks), err)
				return
			}

			ODOOresponse, err := GetODOOMSData(string(payloadBytes))
			if err != nil {
				chunkErrorsMutex.Lock()
				chunkErrors = append(chunkErrors, fmt.Errorf("failed fetching data from ODOO MS API: %v", err))
				chunkErrorsMutex.Unlock()
				logrus.Debugf("Chunk %d/%d: failed fetching data from ODOO MS API: %v", chunkIdx+1, len(chunks), err)
				return
			}

			ODOOResponseArray, ok := ODOOresponse.([]interface{})
			if !ok {
				chunkErrorsMutex.Lock()
				chunkErrors = append(chunkErrors, fmt.Errorf("type assertion failed for chunk"))
				chunkErrorsMutex.Unlock()
				logrus.Debugf("Chunk %d/%d: type assertion failed for chunk", chunkIdx+1, len(chunks))
				return
			}

			allRecordsMutex.Lock()
			allRecords = append(allRecords, ODOOResponseArray...)
			allRecordsMutex.Unlock()
			logrus.Debugf("Finished chunk %d/%d: processed %d records", chunkIdx+1, len(chunks), len(ODOOResponseArray))
		}(i, chunk)
	}

	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		// All goroutines finished
	case <-ctx.Done():
		logrus.Errorf("Timeout waiting for chunk results")
		return errors.New("timeout waiting for chunk results")
	}

	if len(allRecords) == 0 {
		return fmt.Errorf("%s - no valid data retrieved after processing all chunks, exiting", taskDoing)
	}

	ODOOResponseBytes, err := json.Marshal(allRecords)
	if err != nil {
		return fmt.Errorf("failed to marshal all ODOO records: %v", err)
	}

	var listOfData []OdooTicketDataRequestItem
	if err := json.Unmarshal(ODOOResponseBytes, &listOfData); err != nil {
		return fmt.Errorf("failed to unmarshal ODOO records into struct: %v", err)
	}

	dbWeb := gormdb.Databases.Web
	// Clean up existing records for the month before inserting new data
	tx := dbWeb.Unscoped().Where("1=1").Delete(&reportmodel.ODOOMSSLAReport{})
	if tx.Error != nil {
		return fmt.Errorf("failed to truncate table sla_report: %v", tx.Error)
	}
	logrus.Infof("Deleted %d existing records from sla_report", tx.RowsAffected)
	logrus.Infof("Trying to insert %d records into sla_report", len(listOfData))

	dataCount := 0 // Trace inserted data count
	for i := 0; i < len(listOfData); i += batchSize {
		end := i + batchSize
		if end > len(listOfData) {
			end = len(listOfData)
		}

		var batch []reportmodel.ODOOMSSLAReport
		for _, data := range listOfData[i:end] {
			var ticketCreatedAt, receivedSPK, slaDeadline, completeDatetimeWO, firstTaskCompleteDate *time.Time
			if data.CreateDate.Valid {
				ticketCreatedAt = &data.CreateDate.Time
			}
			if data.ReceivedDatetimeSpk.Valid {
				receivedSPK = &data.ReceivedDatetimeSpk.Time
			}
			if data.SlaDeadline.Valid {
				slaDeadline = &data.SlaDeadline.Time
			}
			if data.CompleteDatetimeWo.Valid {
				completeDatetimeWO = &data.CompleteDatetimeWo.Time
			}

			slaStatus, firstTaskDatetime, firstTaskReason, firstTaskMsg := setSLAStatus(
				data.TaskCount.Int,
				data.SlaDeadline,
				data.CompleteDatetimeWo,
				data.WoRemarkTiket,
				data.TaskType,
			)

			if !firstTaskDatetime.IsZero() {
				firstTaskCompleteDate = &firstTaskDatetime
			}

			_, stage := parseJSONIDDataCombinedSafe(data.StageId)
			_, technician := parseJSONIDDataCombinedSafe(data.TechnicianId)
			_, company := parseJSONIDDataCombinedSafe(data.CompanyId)
			_, edcType := parseJSONIDDataCombinedSafe(data.EdcType)
			_, edcSerial := parseJSONIDDataCombinedSafe(data.SnEdc)
			_, ticketType := parseJSONIDDataCombinedSafe(data.TicketTypeId)

			var technicianGroup string = "N/A"
			techGroup, err := techGroup(technician)
			if err != nil {
				logrus.Errorf("failed to get tech group for technician %s: %v", technician, err)
			} else {
				technicianGroup = techGroup
			}

			firstRC := ""
			ticketReasonCodes := parseReasonCode(data.ReasonCode.String)
			if len(ticketReasonCodes) > 0 {
				firstRC = ticketReasonCodes[len(ticketReasonCodes)-1]
			}

			var merchantLong, merchantLang *float64
			if data.Longitude.Valid {
				merchantLong = &data.Longitude.Float
			}
			if data.Latitude.Valid {
				merchantLang = &data.Latitude.Float
			}

			isExcludedDataforSLAReport := excludeDataForSLAReport(data, slaStatus)
			if isExcludedDataforSLAReport {
				continue // Skip this record and do not add to batch
			}

			batch = append(batch, reportmodel.ODOOMSSLAReport{
				ID:                        data.ID,
				TicketNumber:              data.TicketSubject.String,
				TicketCreatedAt:           ticketCreatedAt,
				Stage:                     stage,
				Technician:                technician,
				Company:                   company,
				TaskType:                  data.TaskType.String,
				ReceivedDatetimeSPK:       receivedSPK,
				SLADeadline:               slaDeadline,
				SLAStatus:                 slaStatus,
				SLAExpired:                SLAExpired(data.SlaDeadline),
				CompleteDatetimeWO:        completeDatetimeWO,
				TechnicianGroup:           technicianGroup,
				MID:                       data.Mid.String,
				TID:                       data.Tid.String,
				TaskCount:                 data.TaskCount.Int,
				Merchant:                  data.MerchantName.String,
				MerchantPIC:               data.PicMerchant.String,
				MerchantPhone:             data.PicPhone.String,
				MerchantAddress:           data.MerchantAddress.String,
				MerchantLongitude:         merchantLong,
				MerchantLatitude:          merchantLang,
				LinkWO:                    data.LinkWO.String,
				WORemark:                  data.WoRemarkTiket.String,
				WOFirst:                   data.WOFirst.String,
				WOLast:                    data.WoNumberLast.String,
				StatusEDC:                 data.StatusEDC.String,
				KondisiMerchant:           data.StatusMerchant.String,
				EDCType:                   edcType,
				EDCSerial:                 edcSerial,
				Source:                    data.Source.String,
				ReasonCode:                data.ReasonCode.String,
				FirstTaskCompleteDatetime: firstTaskCompleteDate,
				FirstTaskReason:           firstTaskReason,
				FirstTaskReasonCode:       firstRC,
				FirstTaskMessage:          firstTaskMsg,
				Description:               data.Description.String,
				TicketType:                ticketType,
			})

			dataCount++
		}

		if err := dbWeb.Model(&reportmodel.ODOOMSSLAReport{}).Create(batch).Error; err != nil {
			return fmt.Errorf("failed to insert batch into sla_report: %v", err)
		}
	}

	logrus.Infof("Successfully inserted %d records into sla_report that already pass the filtering criteria", dataCount)

	return nil
}

func GenerateSLAReportODOOMS() ([]string, error) {
	taskDoing := "generate SLA Report ODOOMS"

	if err := GetDataSLAReportODOOMS(); err != nil {
		logrus.Errorf("%s - error: %v", taskDoing, err)
		return nil, err
	}

	var generatedFiles []string
	reportTypes := config.WebPanel.Get().Report.SLA.GeneratedTypes

	for _, reportType := range reportTypes {
		filePath, err := GenerateExcelSLAReportODOOMS(reportType)
		if err != nil {
			logrus.Errorf("%s - error generating %s report: %v", taskDoing, reportType, err)
			continue
		}
		generatedFiles = append(generatedFiles, filePath)
	}

	if len(generatedFiles) == 0 {
		return nil, fmt.Errorf("%s - no reports were generated successfully", taskDoing)
	}

	return generatedFiles, nil
}

func GenerateExcelSLAReportODOOMS(excelRequest string) (string, error) {
	taskDoing := "generate Excel SLA Report ODOOMS"

	var excelNameNew string
	switch excelRequest {
	case "Non-PM":
		excelNameNew = "NonPM"
	case "Solved Pending":
		excelNameNew = "SolvedPending"
	default:
		excelNameNew = excelRequest
	}

	loc, _ := time.LoadLocation(config.WebPanel.Get().Default.Timezone)
	now := time.Now().In(loc)

	excelFileName := fmt.Sprintf("(%v)SLAReport_%v.xlsx", time.Now().Format("02Jan2006"), excelNameNew)
	selectedMainDir, err := fun.FindValidDirectory([]string{
		"web/file/sla_report",
		"../web/file/sla_report",
		"../../web/file/sla_report",
	})

	if err != nil {
		logrus.Errorf("%s - error: %v", taskDoing, err)
		return "", err
	}

	fileReportDir := filepath.Join(selectedMainDir, now.Format("2006-01-02"))
	if err := os.MkdirAll(fileReportDir, 0755); err != nil {
		return "", fmt.Errorf("failed to create directory %s: %v", fileReportDir, err)
	}

	dbWeb := gormdb.Databases.Web
	var count int64
	switch excelRequest {
	case "PM":
		if err := dbWeb.Model(&reportmodel.ODOOMSSLAReport{}).
			Where("task_type = ?", "Preventive Maintenance").Count(&count).Error; err != nil {
			return "", err
		}
	case "CM":
		if err := dbWeb.Model(&reportmodel.ODOOMSSLAReport{}).
			Where("task_type = ?", "Corrective Maintenance").Count(&count).Error; err != nil {
			return "", err
		}
	case "Non-PM":
		if err := dbWeb.Model(&reportmodel.ODOOMSSLAReport{}).
			Where("task_type != ?", "Preventive Maintenance").
			Count(&count).Error; err != nil {
			return "", err
		}
	case "Solved Pending":
		if err := dbWeb.Model(&reportmodel.ODOOMSSLAReport{}).
			Where("stage = ?", "Solved Pending").Count(&count).Error; err != nil {
			return "", err
		}
	case "Master":
		if err := dbWeb.Model(&reportmodel.ODOOMSSLAReport{}).
			Count(&count).Error; err != nil {
			return "", err
		}
	default:
		return "", fmt.Errorf("invalid excel request type: %s", excelRequest)
	}

	excelFilePath := filepath.Join(fileReportDir, excelFileName)

	if count == 0 {
		errMsg := fmt.Sprintf("no data found for %v new SLA Report", excelRequest)
		return "", errors.New(errMsg)
	}

	f := excelize.NewFile()
	sheetEmployees := "EMPLOYEES"
	sheetMaster := excelNameNew
	sheetPivot := "PIVOT"
	sheetPvtHelpdesk := "HELPDESK.TICKET"
	sheetPvtClient := "CLIENT PIVOT"
	sheetPvtSLAExpired := "SLA EXPIRES"

	f.NewSheet(sheetEmployees)
	f.NewSheet(sheetMaster)
	f.NewSheet(sheetPivot)
	f.NewSheet(sheetPvtHelpdesk)
	f.NewSheet(sheetPvtClient)
	f.NewSheet(sheetPvtSLAExpired)

	// Employees
	titlesEmployee := []struct {
		Title string
		Size  float64
	}{
		{"Technician", 25},
		{"SPL", 20},
		{"Ops Head", 20},
	}
	var columnsEmployee []ExcelColumn
	for i, t := range titlesEmployee {
		columnsEmployee = append(columnsEmployee, ExcelColumn{
			ColIndex: fun.GetColName(i),
			ColTitle: t.Title,
			ColSize:  t.Size,
		})
	}
	for _, col := range columnsEmployee {
		cell := fmt.Sprintf("%s1", col.ColIndex)
		f.SetCellValue(sheetEmployees, cell, col.ColTitle)
		f.SetColWidth(sheetEmployees, col.ColIndex, col.ColIndex, col.ColSize)
	}
	lastColEmployee := fun.GetColName(len(columnsEmployee) - 1)
	filterRangeEmployee := fmt.Sprintf("A1:%s1", lastColEmployee)
	f.AutoFilter(sheetEmployees, filterRangeEmployee, []excelize.AutoFilterOptions{})

	ODOOModel := "fs.technician"
	excludedTechnicians := []string{
		"Tes Dev Mfjr",
		"Call Center",
		"Teknisi Pameran",
		"Asset Edi Purwanto",
	}
	domain := []interface{}{
		[]interface{}{"active", "=", true},
		[]interface{}{"name", "!=", excludedTechnicians},
	}
	fields := []string{"id", "name", "technician_code", "x_spl_leader"}
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
		return "", fmt.Errorf("failed to marshal ODOO payload: %v", err)
	}
	ODOOresponse, err := GetODOOMSData(string(payloadBytes))
	if err != nil {
		return "", fmt.Errorf("failed fetching data from ODOO MS API: %v", err)
	}
	ODOOResponseArray, ok := ODOOresponse.([]interface{})
	if !ok {
		return "", errors.New("type assertion failed for ODOO response")
	}
	ODOOResponseBytes, err := json.Marshal(ODOOResponseArray)
	if err != nil {
		return "", fmt.Errorf("failed to marshal all ODOO records: %v", err)
	}
	var employeeData []ODOOMSTechnicianItem
	if err := json.Unmarshal(ODOOResponseBytes, &employeeData); err != nil {
		return "", fmt.Errorf("failed to unmarshal ODOO records into struct: %v", err)
	}

	if len(employeeData) == 0 {
		errMsg := "no data found for employees"
		return "", errors.New(errMsg)
	}

	employeeRowIndex := 2
	for _, record := range employeeData {
		for _, col := range columnsEmployee {
			cell := fmt.Sprintf("%s%d", col.ColIndex, employeeRowIndex)
			var value interface{} = "N/A"
			switch col.ColTitle {
			case "Technician":
				if record.NameFS.String != "" {
					value = record.NameFS.String
				}
				f.SetCellValue(sheetEmployees, cell, value)
			case "SPL":
				if record.SPL.String != "" {
					value = record.SPL.String
				}
				f.SetCellValue(sheetEmployees, cell, value)
			case "Ops Head":
				if record.Head.String != "" {
					value = record.Head.String
				}
				f.SetCellValue(sheetEmployees, cell, value)
			}
		}
		employeeRowIndex++
	}

	titles := []struct {
		Title string
		Size  float64
	}{
		{"Ops Head", 20},
		{"SPL", 30},
		// {"Technician Group", 20},
		{"Technician", 25},
		{"SPK Number", 50},
		{"Stage", 30},
		{"Company", 20},
		{"Task Type", 20},
		{"Received SPK at", 20},
		{"SLA Deadline", 20},
		{"Complete WO", 20},
		{"SLA Status", 20},
		{"SLA Expired", 25},
		{"MID", 30},
		{"TID", 30},
		{"Merchant", 40},
		{"Merchant PIC", 30},
		{"Merchant Phone", 30},
		{"Merchant Address", 50},
		{"Merchant Latitude", 25},
		{"Merchant Longitude", 25},
		{"Task Count", 18},
		{"WO Remark", 50},
		{"Reason Code", 20},
		{"First JO Complete Datetime", 35},
		{"First JO Reason", 30},
		{"First JO Message", 40},
		{"First JO Reason Code", 30},
		{"Link WO", 40},
		{"WO First", 30},
		{"WO Last", 30},
		{"Status EDC", 30},
		{"Kondisi Merchant", 30},
		{"EDC Type", 30},
		{"EDC Serial", 30},
		{"Source", 30},
		{"Description", 70},
		{"Ticket Type", 50},
	}
	var columns []ExcelColumn
	for i, t := range titles {
		columns = append(columns, ExcelColumn{
			ColIndex: fun.GetColName(i),
			ColTitle: t.Title,
			ColSize:  t.Size,
		})
	}
	for _, col := range columns {
		cell := fmt.Sprintf("%s1", col.ColIndex)
		f.SetCellValue(sheetMaster, cell, col.ColTitle)
		f.SetColWidth(sheetMaster, col.ColIndex, col.ColIndex, col.ColSize)
	}
	lastCol := fun.GetColName(len(columns) - 1)
	filterRange := fmt.Sprintf("A1:%s1", lastCol)
	f.AutoFilter(sheetMaster, filterRange, []excelize.AutoFilterOptions{})

	batchSize := 5000

	type RowData struct {
		Record reportmodel.ODOOMSSLAReport
	}

	rowChan := make(chan RowData, batchSize*2)
	var wg sync.WaitGroup
	var writeWg sync.WaitGroup
	var fetchErr error

	// Writer goroutine with shared rowIndex
	writeWg.Add(1)
	go func() {
		defer writeWg.Done()
		rowIndex := 2
		for row := range rowChan {
			for _, col := range columns {
				cell := fmt.Sprintf("%s%d", col.ColIndex, rowIndex)
				var value interface{} = ""
				switch col.ColTitle {
				case "Ops Head":
					formula := fmt.Sprintf(`IFERROR(VLOOKUP(C%d, %s!A:C, 3, FALSE), "N/A")`, rowIndex, sheetEmployees)
					f.SetCellFormula(sheetMaster, cell, formula)
					continue
				case "SPL":
					formula := fmt.Sprintf(`IFERROR(VLOOKUP(C%d, %s!A:C, 2, FALSE), "N/A")`, rowIndex, sheetEmployees)
					f.SetCellFormula(sheetMaster, cell, formula)
					continue
				case "Technician":
					value = row.Record.Technician
				case "SPK Number":
					value = row.Record.TicketNumber
				case "Stage":
					value = row.Record.Stage
				case "Company":
					value = row.Record.Company
				case "Task Type":
					value = row.Record.TaskType
				case "Received SPK at":
					if row.Record.ReceivedDatetimeSPK != nil {
						value = row.Record.ReceivedDatetimeSPK.Format("2006-01-02 15:04:05")
					}
				case "SLA Deadline":
					if row.Record.SLADeadline != nil {
						value = row.Record.SLADeadline.Format("2006-01-02 15:04:05")
					}
				case "Complete WO":
					if row.Record.CompleteDatetimeWO != nil {
						value = row.Record.CompleteDatetimeWO.Format("2006-01-02 15:04:05")
					}
				case "SLA Status":
					value = row.Record.SLAStatus
				case "SLA Expired":
					value = row.Record.SLAExpired
				case "MID":
					value = row.Record.MID
				case "TID":
					value = row.Record.TID
				case "Merchant":
					value = row.Record.Merchant
				case "Merchant PIC":
					value = row.Record.MerchantPIC
				case "Merchant Phone":
					value = row.Record.MerchantPhone
				case "Merchant Address":
					value = row.Record.MerchantAddress
				case "Merchant Latitude":
					if row.Record.MerchantLatitude != nil {
						value = *row.Record.MerchantLatitude
					}
				case "Merchant Longitude":
					if row.Record.MerchantLongitude != nil {
						value = *row.Record.MerchantLongitude
					}
				case "Task Count":
					value = row.Record.TaskCount
				case "WO Remark":
					value = row.Record.WORemark
				case "Reason Code":
					value = row.Record.ReasonCode
				case "First JO Complete Datetime":
					if row.Record.FirstTaskCompleteDatetime != nil {
						value = row.Record.FirstTaskCompleteDatetime.Format("2006-01-02 15:04:05")
					}
				case "First JO Reason":
					value = row.Record.FirstTaskReason
				case "First JO Message":
					value = row.Record.FirstTaskMessage
				case "First JO Reason Code":
					value = row.Record.FirstTaskReasonCode
				case "Link WO":
					if row.Record.LinkWO != "" {
						f.SetCellHyperLink(sheetMaster, cell, row.Record.LinkWO, "External")
						f.SetCellValue(sheetMaster, cell, row.Record.LinkWO)
						continue
					}
					value = row.Record.LinkWO
				case "WO First":
					value = row.Record.WOFirst
				case "WO Last":
					value = row.Record.WOLast
				case "Status EDC":
					value = row.Record.StatusEDC
				case "Kondisi Merchant":
					value = row.Record.KondisiMerchant
				case "EDC Type":
					value = row.Record.EDCType
				case "EDC Serial":
					value = row.Record.EDCSerial
				case "Source":
					value = row.Record.Source
				case "Description":
					value = row.Record.Description
				case "Ticket Type":
					value = row.Record.TicketType
				}
				f.SetCellValue(sheetMaster, cell, value)
			}
			rowIndex++
		}
	}()

	// Fetcher goroutines (use new query per goroutine, add debug logging)
	for offset := 0; offset < int(count); offset += batchSize {
		wg.Add(1)
		go func(offset int) {
			defer wg.Done()
			var batch []reportmodel.ODOOMSSLAReport
			// Build a new query for each goroutine
			query := dbWeb.Model(&reportmodel.ODOOMSSLAReport{})
			switch excelRequest {
			case "PM":
				query = query.Where("task_type = ?", "Preventive Maintenance")
			case "CM":
				query = query.Where("task_type = ?", "Corrective Maintenance")
			case "Non-PM":
				query = query.Where("task_type != ?", "Preventive Maintenance")
			case "Solved Pending":
				query = query.Where("stage = ?", "Solved Pending")
			case "Master":
				// no filter
			}
			err := query.Offset(offset).Limit(batchSize).Find(&batch).Error
			logrus.Debugf("Excel fetch batch offset %d, got %d rows, err: %v", offset, len(batch), err)
			if err != nil {
				fetchErr = err
				return
			}
			for _, rec := range batch {
				rowChan <- RowData{Record: rec}
			}
		}(offset)
	}

	wg.Wait()
	close(rowChan)
	writeWg.Wait()
	if fetchErr != nil {
		return "", fetchErr
	}

	pivotDataRange := fmt.Sprintf("%s!A1:%s%d", sheetMaster, lastCol, 1+int(count))

	// Pivot Technician Group
	pvtTechGroupRange := fmt.Sprintf("%v!A8:S200", sheetPivot)
	err = f.AddPivotTable(&excelize.PivotTableOptions{
		Name:            sheetPivot,
		DataRange:       pivotDataRange,
		PivotTableRange: pvtTechGroupRange,
		Rows: []excelize.PivotTableField{
			{Data: "Ops Head"},
			{Data: "SPL"},
		},
		Columns: []excelize.PivotTableField{
			{Data: "SLA Status"},
		},
		Data: []excelize.PivotTableField{
			{Data: "SLA Status", Name: "Count of SPL", Subtotal: "Count"},
		},
		Filter: []excelize.PivotTableField{
			{Data: "Description"},
			{Data: "Task Type"},
			{Data: "SPK Number"},
		},
		RowGrandTotals: true,
		ColGrandTotals: true,
		ShowDrill:      true,
		ShowRowHeaders: true,
		ShowColHeaders: true,
		ShowLastColumn: true,
	})
	if err != nil {
		return "", err
	}
	f.SetColWidth(sheetPivot, "A", "A", 25)
	f.SetColWidth(sheetPivot, "B", "B", 32)

	// Pivot HELPDESK.TICKET
	pivotHelpdeskTicketRange := fmt.Sprintf("%v!A1:U2000", sheetPvtHelpdesk)
	err = f.AddPivotTable(&excelize.PivotTableOptions{
		Name:            sheetPvtHelpdesk,
		DataRange:       pivotDataRange,
		PivotTableRange: pivotHelpdeskTicketRange,
		Rows: []excelize.PivotTableField{
			{Data: "Company"},
			{Data: "Ops Head"},
			{Data: "SPL"},
			{Data: "Technician"},
		},
		Columns: []excelize.PivotTableField{
			{Data: "Task Type", DefaultSubtotal: true},
			{Data: "SLA Status", DefaultSubtotal: true},
		},
		Data: []excelize.PivotTableField{
			{Data: "SLA Status", Name: "Details Pivot", Subtotal: "Count"},
		},
		RowGrandTotals: true,
		ColGrandTotals: true,
		ShowDrill:      true,
		ShowRowHeaders: true,
		ShowColHeaders: true,
		ShowLastColumn: true,
	})
	if err != nil {
		return "", err
	}
	f.SetColWidth(sheetPvtHelpdesk, "A", "A", 20)
	f.SetColWidth(sheetPvtHelpdesk, "B", "B", 20)
	f.SetColWidth(sheetPvtHelpdesk, "C", "C", 35)
	f.SetColWidth(sheetPvtHelpdesk, "D", "D", 25)

	// Client Pivot
	pivotClientRange := fmt.Sprintf("%v!A1:S200", sheetPvtClient)
	err = f.AddPivotTable(&excelize.PivotTableOptions{
		Name:            sheetPvtClient,
		DataRange:       pivotDataRange,
		PivotTableRange: pivotClientRange,
		Rows: []excelize.PivotTableField{
			{Data: "SLA Status"},
			{Data: "Company"},
		},
		Columns: []excelize.PivotTableField{
			{Data: "Task Type"},
		},
		Data: []excelize.PivotTableField{
			{Data: "SLA Status", Name: "Count of SLA Status - Client", Subtotal: "Count"},
		},
		RowGrandTotals: true,
		ColGrandTotals: true,
		ShowDrill:      true,
		ShowRowHeaders: true,
		ShowColHeaders: true,
		ShowLastColumn: true,
	})
	if err != nil {
		return "", err
	}
	f.SetColWidth(sheetPvtClient, "A", "A", 40)
	f.SetColWidth(sheetPvtClient, "B", "B", 25)

	// SLA EXPIRES
	pivotSLAExpiresRange := fmt.Sprintf("%v!A1:S200", sheetPvtSLAExpired)
	err = f.AddPivotTable(&excelize.PivotTableOptions{
		Name:            sheetPvtSLAExpired,
		DataRange:       pivotDataRange,
		PivotTableRange: pivotSLAExpiresRange,
		Rows: []excelize.PivotTableField{
			{Data: "Company"},
			{Data: "Stage"},
			{Data: "Task Type"},
			{Data: "Technician"},
		},
		Columns: []excelize.PivotTableField{
			{Data: "SLA Expired"},
		},
		Data: []excelize.PivotTableField{
			{Data: "SLA Expired", Name: "Count of SLA Expired", Subtotal: "Count"},
		},
		RowGrandTotals: true,
		ColGrandTotals: true,
		ShowDrill:      true,
		ShowRowHeaders: true,
		ShowColHeaders: true,
		ShowLastColumn: true,
	})
	if err != nil {
		return "", err
	}
	f.SetColWidth(sheetPvtSLAExpired, "A", "A", 40)
	f.SetColWidth(sheetPvtSLAExpired, "B", "B", 25)

	f.DeleteSheet("Sheet1")
	f.SetActiveSheet(1)
	if err := f.SaveAs(excelFilePath); err != nil {
		return "", err
	}

	logrus.Infof("successfully created excel of sla %s report with total %d rows", excelRequest, count)
	return excelFilePath, nil
}
