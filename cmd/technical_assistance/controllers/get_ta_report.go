package controllers

import (
	"bytes"
	"crypto/tls"
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"service-platform/cmd/technical_assistance/config"
	"service-platform/cmd/technical_assistance/model"
	"service-platform/cmd/technical_assistance/model/op_model"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/xuri/excelize/v2"
	"gorm.io/gorm"
)

func chunkIdsSlice(ids []uint64, size int) [][]uint64 {
	if size <= 0 {
		return nil
	}

	var chunks [][]uint64
	for i := 0; i < len(ids); i += size {
		end := i + size
		if end > len(ids) {
			end = len(ids)
		}
		chunks = append(chunks, ids[i:end])
	}
	return chunks
}

type NullableString struct {
	String string
	Valid  bool
}

func (ns *NullableString) UnmarshalJSON(data []byte) error {
	if string(data) == "null" {
		ns.String = ""
		ns.Valid = false
		return nil
	}

	if data[0] == '"' {
		var str string
		if err := json.Unmarshal(data, &str); err != nil {
			return err
		}
		ns.String = str
		ns.Valid = true
		return nil
	}

	if data[0] == 'f' || data[0] == 't' {
		var b bool
		if err := json.Unmarshal(data, &b); err != nil {
			return err
		}
		if b {
			ns.String = "true"
		} else {
			ns.String = ""
		}
		ns.Valid = b
		return nil
	}

	return errors.New("invalid type for NullableString")
}

func (ns NullableString) IsEmpty() bool {
	return !ns.Valid
}

type NullableTime struct {
	Time  time.Time
	Valid bool
}

// Nullable Interface (For arrays or mixed types)
type NullableInterface struct {
	Data  interface{}
	Valid bool
}

func (ni *NullableInterface) UnmarshalJSON(data []byte) error {
	if string(data) == "null" || string(data) == "false" {
		ni.Data = nil
		ni.Valid = false
		return nil
	}

	var temp interface{}
	if err := json.Unmarshal(data, &temp); err != nil {
		return err
	}

	ni.Data = temp
	ni.Valid = true
	return nil
}

func (ni NullableInterface) IsEmpty() bool {
	return !ni.Valid || ni.Data == nil
}

func (ni NullableInterface) ToIntSlice() []int {
	if ni.Data == nil || !ni.Valid {
		return []int{}
	}

	// Try to assert the data as a slice of interfaces
	if dataSlice, ok := ni.Data.([]interface{}); ok {
		intSlice := make([]int, len(dataSlice))
		for i, v := range dataSlice {
			// Convert each value to int
			if num, ok := v.(float64); ok {
				intSlice[i] = int(num) // Convert float64 to int
			}
		}
		return intSlice
	}

	// Return empty slice if conversion fails
	return []int{}
}

func parseJSONIDDataCombined(nullableData NullableInterface) (int, string, error) {
	if nullableData.IsEmpty() {
		return 0, "", nil // Return default values for empty data
	}

	arrayData, ok := nullableData.Data.([]interface{})
	if !ok || len(arrayData) < 2 {
		return 0, "", errors.New("invalid array data")
	}

	dataIDFloat, ok := arrayData[0].(float64)
	if !ok {
		return 0, "", errors.New("invalid type for data ID; expected float64")
	}
	dataID := int(dataIDFloat)

	dataString, ok := arrayData[1].(string)
	if !ok {
		return 0, "", errors.New("invalid type for data string; expected string")
	}

	return dataID, dataString, nil
}

type Column struct {
	ColIndex string
	ColTitle string
	ColSize  float64
}

type DataProjectTask struct {
	ID                  uint              `json:"id"`
	MerchantName        NullableString    `json:"x_merchant"`
	PicMerchant         NullableString    `json:"x_pic_merchant"`
	PicPhone            NullableString    `json:"x_pic_phone"`
	MerchantAddress     NullableString    `json:"partner_street"`
	Description         NullableString    `json:"x_title_cimb"` // "description"
	SlaDeadline         NullableTime      `json:"x_sla_deadline"`
	CreateDate          NullableTime      `json:"create_date"`
	ReceivedDatetimeSpk NullableTime      `json:"x_received_datetime_spk"`
	PlanDate            NullableTime      `json:"planned_date_begin"`
	TimesheetLastStop   NullableTime      `json:"timesheet_timer_last_stop"`
	DateLastStageUpdate NullableTime      `json:"date_last_stage_update"`
	TaskType            NullableString    `json:"x_task_type"`
	WorksheetTemplateId NullableInterface `json:"worksheet_template_id"`
	TicketTypeId        NullableInterface `json:"x_ticket_type2"`
	CompanyId           NullableInterface `json:"company_id"`
	StageId             NullableInterface `json:"stage_id"`
	HelpdeskTicketId    NullableInterface `json:"helpdesk_ticket_id"`
	Mid                 NullableString    `json:"x_cimb_master_mid"`
	Tid                 NullableString    `json:"x_cimb_master_tid"`
	Source              NullableString    `json:"x_source"`
	MessageCC           NullableString    `json:"x_message_call"`
	WoNumber            NullableString    `json:"x_no_task"`
	StatusMerchant      NullableString    `json:"x_status_merchant"`
	SnEdc               NullableInterface `json:"x_studio_edc"`
	EdcType             NullableInterface `json:"x_product"`
	WoRemarkTiket       NullableString    `json:"x_wo_remark"`
	Longitude           NullableString    `json:"x_longitude"`
	Latitude            NullableString    `json:"x_latitude"`
	TechnicianId        NullableInterface `json:"technician_id"`
	ReasonCodeId        NullableInterface `json:"x_reason_code_id"`
	WriteUid            NullableInterface `json:"write_uid"`
}

func (t *DataProjectTask) UnmarshalJSON(data []byte) error {
	type Alias DataProjectTask // Create an alias to avoid recursion
	aux := &struct {
		SlaDeadline         interface{} `json:"x_sla_deadline"`
		CreateDate          interface{} `json:"create_date"`
		ReceivedDatetimeSpk interface{} `json:"x_received_datetime_spk"`
		PlanDate            interface{} `json:"planned_date_begin"`
		TimesheetLastStop   interface{} `json:"timesheet_timer_last_stop"`
		DateLastStageUpdate interface{} `json:"date_last_stage_update"`
		*Alias
	}{
		Alias: (*Alias)(t),
	}

	// Unmarshal into the auxiliary structure
	if err := json.Unmarshal(data, &aux); err != nil {
		return fmt.Errorf("failed to unmarshal JSON: %v", err)
	}

	// Function to parse time fields
	parseTimeField := func(value interface{}) (NullableTime, error) {
		switch v := value.(type) {
		case string:
			if v == "" || v == "null" {
				return NullableTime{Time: time.Time{}, Valid: false}, nil
			}
			parsedTime, err := time.Parse("2006-01-02 15:04:05", v)
			if err != nil {
				return NullableTime{}, fmt.Errorf("failed to parse time: %v", err)
			}
			return NullableTime{Time: parsedTime, Valid: true}, nil
		case bool:
			if !v {
				return NullableTime{Time: time.Time{}, Valid: false}, nil
			}
			return NullableTime{}, fmt.Errorf("unexpected boolean value: true")
		case nil:
			return NullableTime{Time: time.Time{}, Valid: false}, nil
		default:
			return NullableTime{}, fmt.Errorf("unexpected type: %T", value)
		}
	}

	// Parse each time field separately
	var err error

	if t.PlanDate, err = parseTimeField(aux.PlanDate); err != nil {
		return fmt.Errorf("PlanDate: %v", err)
	}

	if t.SlaDeadline, err = parseTimeField(aux.SlaDeadline); err != nil {
		return fmt.Errorf("SlaDeadline: %v", err)
	}

	if t.CreateDate, err = parseTimeField(aux.CreateDate); err != nil {
		return fmt.Errorf("CreateDate: %v", err)
	}

	if t.ReceivedDatetimeSpk, err = parseTimeField(aux.ReceivedDatetimeSpk); err != nil {
		return fmt.Errorf("ReceivedDatetimeSpk: %v", err)
	}

	if t.TimesheetLastStop, err = parseTimeField(aux.TimesheetLastStop); err != nil {
		return fmt.Errorf("TimesheetLastStop: %v", err)
	}

	if t.DateLastStageUpdate, err = parseTimeField(aux.DateLastStageUpdate); err != nil {
		return fmt.Errorf("DateLastStageUpdate: %v", err)
	}

	return nil
}

type DataFSTechnician struct {
	Technician NullableString `json:"name"`
	SPL        NullableString `json:"technician_code"`
	OpsHead    NullableString `json:"x_spl_leader"`
}

func SafeValue(val interface{}) string {
	switch v := val.(type) {
	case string:
		if v == "" || v == "0" {
			return "N/A"
		}
		return v
	case time.Time:
		if v.IsZero() {
			return "N/A"
		}
		return v.Add(7 * time.Hour).Format("2006-01-02 15:04:05")
	default:
		return "N/A"
	}
}

func CleanSPKNumber(spk string) string {
	re := regexp.MustCompile(`\s*\(.*?\)`)
	return re.ReplaceAllString(spk, "")
}

var taData = map[string]string{
	// "komalaadw22@gmail.com":                "Technical Assistance 2 - Komala Dewi",
	// "wanto@csnams.com":                     "Technical Assistance 4 - Budi Purwanto",
	// "steven@csnams.com":                    "Technical Assistance 9 - Steven",
	// "monitoringteknisi@gmail.com":          "Technical Assistance 9 - Steven",
	// "yudha@csnams.com":                     "Technical Assistance 3 - M Angga Yudha",
	// "thessalonica_a@smartwebindonesia.com": "Assistant - Thessa",
	// "abdu@csnams.com":                      "Technical Assistance 6 - Abdu",
	"wegirandol@smartwebindonesia.com":     "Dev RM",
	"admin@swi.com":                        "Administrator",
	"testmfjr@gmail.com":                   "Tes Dev Mfjr",
	"desta@smartwebindonesia.com":          "Admin RM Dev Ipal",
	"ramaadelins@csna4u.com":               "Technical Assistance 1 - Rama Adelins",
	"naominaomiyns@gmail.com":              "Technical Assistance 2 - Naomi",
	"thessalonica_a@smartwebindonesia.com": "Technical Assistance 3 - Thessa",
	"suhendrik.180189@gmail.com":           "Technical Assistance 4 - Suhendrik Zakaria",
	"mukti@csnams.com":                     "Technical Assistance 5 - Arif Arya M.",
	"triyanawirda910@gmail.com":            "Technical Assistance 6 - Wiwi",
	"iin_inayah@smartwebindonesia.com":     "Technical Assistance 7 - Iin",
	"callcenter@gmail.com":                 "Team Call Center",
	"tetty@csnams.com":                     "HEAD - Tetty Manurung",
	"sri_t@smartwebindonesia.com":          "Assistant - Sri",
}

func GenerateTAExcelReport(db *gorm.DB, dbWeb *gorm.DB) (string, string, error) {
	now := time.Now()
	startOfDay := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location()).Add(-7 * time.Hour)
	endOfDay := time.Date(now.Year(), now.Month(), now.Day(), 23, 59, 59, 0, now.Location()).Add(-7 * time.Hour)

	var taActivityData []op_model.LogAct
	err := db.Where("date BETWEEN ? AND ?", startOfDay, endOfDay).Find(&taActivityData).Error
	if err != nil {
		return "", "", err
	}

	mainDirPaths := []string{
		"web/file/report/ta_activity",
		"../web/file/report/ta_activity",
		"../../web/file/report/ta_activity",
		"/home/administrator/technical_assistance/web/file/report/ta_activity",
	}

	var selectedMainDir string
	for _, mainDir := range mainDirPaths {
		if _, err := os.Stat(mainDir); err == nil {
			selectedMainDir = mainDir
			break
		}
	}

	if selectedMainDir == "" {
		return "", "", fmt.Errorf("no valid report file dir in: %v", mainDirPaths)
	}

	fileReportDir := filepath.Join(selectedMainDir, now.Format("2006-01-02"))
	if err := os.MkdirAll(fileReportDir, os.ModePerm); err != nil {
		return "", "", err
	}

	if len(taActivityData) == 0 {
		return "", "", fmt.Errorf("%v", "no data found for generating the TA activity report")
	}

	excelFileName := fmt.Sprintf("TALogActivity(%v)Report.xlsx", now.Add(7*time.Hour).Format("02Jan2006_15-04-05"))
	excelFilePath := filepath.Join(fileReportDir, excelFileName)

	titles := []struct {
		Title string
		Size  float64
	}{
		{"Start Followed Up at", 25},
		{"End of Followed Up", 25},
		{"Followed Up (Time)", 25},
		{"Date in Dashboard", 25},
		{"TA", 18},
		{"Email TA", 20},
		{"Technician", 45},
		{"SPL", 45},
		{"Head", 35},
		{"WO Number", 25},
		{"SPK Number", 25},
		{"Type", 25},
		{"Type2", 25},
		{"SLA Deadline", 25},
		{"TID", 25},
		{"Reason Code", 25},
		{"Case in Technician", 25},
		{"Problem", 25},
		{"Activity", 20},
		{"TA Remark (During Deletion)", 50},
		{"TA Feedback", 50},
	}

	var columns []Column
	for i, t := range titles {
		columns = append(columns, Column{
			ColIndex: getColName(i),
			ColTitle: t.Title,
			ColSize:  t.Size,
		})
	}

	f := excelize.NewFile()

	employeeSheet := "EMPLOYEES"
	f.NewSheet(employeeSheet)
	titlesEmployee := []struct {
		Title string
		Size  float64
	}{
		{"Technician", 25},
		{"SPL", 20},
		{"Ops Head", 20},
	}
	var columnsEmployee []Column
	for i, t := range titlesEmployee {
		columnsEmployee = append(columnsEmployee, Column{
			ColIndex: getColName(i),
			ColTitle: t.Title,
			ColSize:  t.Size,
		})
	}
	for _, column := range columnsEmployee {
		f.SetCellValue(employeeSheet, fmt.Sprintf("%s1", column.ColIndex), column.ColTitle)
		f.SetColWidth(employeeSheet, column.ColIndex, column.ColIndex, column.ColSize)
	}

	lastColEmployee := getColName(len(columnsEmployee) - 1)
	filterRangeEmployee := fmt.Sprintf("A1:%s1", lastColEmployee)
	f.AutoFilter(employeeSheet, filterRangeEmployee, []excelize.AutoFilterOptions{})

	/*

		.start insert data for EMPLOYEES

	*/

	ODOOModel := "fs.technician"
	domain := []interface{}{
		[]interface{}{"active", "=", true},
	}
	fields := []string{"id", "name", "technician_code", "x_spl_leader"}
	order := "name asc"

	ODOOResponse, err := ODOOAPI("GetData", domain, ODOOModel, fields, order)
	if err != nil {
		log.Println(err)
	}

	ODOOResponseArray, ok := ODOOResponse.([]interface{})
	if !ok || len(ODOOResponseArray) == 0 {
		log.Println("ODOOResponse is not array")
	}

	ODOOResponseBytes, err := json.Marshal(ODOOResponseArray)
	if err != nil {
		log.Println(err)
	}

	var employeeODOOData []DataFSTechnician
	if err := json.Unmarshal(ODOOResponseBytes, &employeeODOOData); err != nil {
		log.Println(err)
	}

	if len(employeeODOOData) == 0 {
		log.Println("No data employee found")
	}

	employeeRowIndex := 2
	for _, record := range employeeODOOData {
		for _, column := range columnsEmployee {
			cell := fmt.Sprintf("%s%d", column.ColIndex, employeeRowIndex)
			var value string = "N/A"
			switch column.ColTitle {
			case "Technician":
				if record.Technician.String != "" {
					value = record.Technician.String
				}
				f.SetCellValue(employeeSheet, cell, value)
			case "SPL":
				if record.SPL.String != "" {
					value = record.SPL.String
				}
				f.SetCellValue(employeeSheet, cell, value)
			case "Ops Head":
				if record.OpsHead.String != "" {
					value = record.OpsHead.String
				}
				f.SetCellValue(employeeSheet, cell, value)
			}
		}
		employeeRowIndex++ // increment once per record (row), not per column
	}

	/*

		.end of data inserted for EMPLOYEES

	*/

	masterSheet := "MASTER"
	f.NewSheet(masterSheet)

	style, _ := f.NewStyle(&excelize.Style{
		Alignment: &excelize.Alignment{
			Horizontal: "center",
			Vertical:   "center",
		},
	})

	// Header setup
	for _, col := range columns {
		cell := fmt.Sprintf("%s1", col.ColIndex)
		f.SetCellValue(masterSheet, cell, col.ColTitle)
		f.SetColWidth(masterSheet, col.ColIndex, col.ColIndex, col.ColSize)
		f.SetCellStyle(masterSheet, cell, cell, style)
	}

	woDetailURL := os.Getenv("WO_DETAIL_URL")

	rowIndex := 2
	for _, record := range taActivityData {
		for _, column := range columns {
			cell := fmt.Sprintf("%s%d", column.ColIndex, rowIndex)
			var value interface{} = "N/A"

			var needToSetValue bool = true

			switch column.ColTitle {
			case "TA":
				if name, ok := taData[record.Email]; ok {
					value = name
				}
			case "Email TA":
				if record.Email != "" && record.Email != "0" {
					value = record.Email
				}
			case "Technician":
				if record.Teknisi != "" && record.Teknisi != "0" {
					value = record.Teknisi
				}
			case "SPL":
				needToSetValue = false
				formula := fmt.Sprintf(`IFERROR(VLOOKUP(G%d, %v!A:C, 2, FALSE), "N/A")`, rowIndex, employeeSheet)
				f.SetCellFormula(masterSheet, cell, formula)
			case "Head":
				needToSetValue = false
				formula := fmt.Sprintf(`IFERROR(VLOOKUP(G%d, %v!A:C, 3, FALSE), "N/A")`, rowIndex, employeeSheet)
				f.SetCellFormula(masterSheet, cell, formula)
			case "WO Number":
				if record.Wo != nil && *record.Wo != "" && *record.Wo != "0" {
					wo := *record.Wo
					link := fmt.Sprintf("%s/odooms-project-task/detailWO?wo_number=%s", woDetailURL, wo)
					f.SetCellHyperLink(masterSheet, cell, link, "External")
					value = wo
				}
			case "SPK Number":
				if record.SpkNumber != "" && record.SpkNumber != "0" {
					value = CleanSPKNumber(record.SpkNumber)
				}
			case "Type":
				if record.Type != "" && record.Type != "0" {
					value = record.Type
				}
			case "Type2":
				if record.Type2 != "" && record.Type2 != "0" {
					value = record.Type2
				}
			case "SLA Deadline":
				if record.Sla != "" && record.Sla != "0" {
					value = record.Sla
				}
			case "TID":
				if record.Tid != "" && record.Tid != "0" {
					value = record.Tid
				}
			case "Reason Code":
				if record.Rc != "" && record.Rc != "0" {
					value = record.Rc
				}
			case "Case in Technician":
				if record.TypeCase != "" && record.TypeCase != "0" {
					value = record.TypeCase
				}
			case "Problem":
				if record.Problem != "" && record.Problem != "0" {
					value = record.Problem
				}
			case "Activity":
				if record.Method != "" && record.Method != "0" {
					value = record.Method
				}
			case "Start Followed Up at":
				if record.DateOnCheck != nil && !record.DateOnCheck.IsZero() {
					value = record.DateOnCheck.Add(7 * time.Hour).Format("2006-01-02 15:04:05")
				} else {
					value = "N/A"
				}
			case "End of Followed Up":
				if !record.Date.IsZero() {
					value = record.Date.Add(7 * time.Hour).Format("2006-01-02 15:04:05")
				} else {
					value = "N/A"
				}
			case "Followed Up (Time)":
				if record.DateOnCheck != nil && !record.DateOnCheck.IsZero() && !record.Date.IsZero() {
					duration := record.Date.Add(7 * time.Hour).Sub(record.DateOnCheck.Add(7 * time.Hour))
					h := int(duration.Hours())
					m := int(duration.Minutes()) % 60
					s := int(duration.Seconds()) % 60
					value = fmt.Sprintf("%02d:%02d:%02d", h, m, s)

					// Check if duration exceeds 15 minutes
					if duration > 15*time.Minute {
						needToSetValue = false
						styleID, err := f.NewStyle(&excelize.Style{
							Font: &excelize.Font{
								Color: "FF0000", // red
							},
							Alignment: &excelize.Alignment{
								Horizontal: "center",
								Vertical:   "center",
							},
						})
						if err != nil {
							log.Print(err)
						}
						f.SetCellValue(masterSheet, cell, value)
						f.SetCellStyle(masterSheet, cell, cell, styleID)
						break
					}
				} else {
					value = "N/A"
				}
			case "Date in Dashboard":
				if record.DateInDashboard == "" {
					value = "N/A"
				} else {
					value = record.DateInDashboard
				}
			case "TA Remark (During Deletion)":
				if record.Reason != nil && *record.Reason != "" && *record.Reason != "0" {
					value = *record.Reason
				}
			case "TA Feedback":
				if record.TaFeedback == "" {
					value = "N/A"
				} else {
					value = record.TaFeedback
				}
			}

			if needToSetValue {
				f.SetCellValue(masterSheet, cell, value)
				f.SetCellStyle(masterSheet, cell, cell, style)
			}

		}
		rowIndex++
	}

	lastCol := getColName(len(columns) - 1)
	filterRange := fmt.Sprintf("A1:%s1", lastCol)

	f.AutoFilter(masterSheet, filterRange, []excelize.AutoFilterOptions{})
	f.DeleteSheet("Sheet1")

	pivotDataRange := fmt.Sprintf("%s!$A$1:$%s$%d", masterSheet, lastCol, rowIndex-1)
	pivotSheet := "PIVOT"
	f.NewSheet(pivotSheet)
	f.SetColWidth(pivotSheet, "A", "A", 42)

	if err := f.AddPivotTable(&excelize.PivotTableOptions{
		DataRange:       pivotDataRange,
		PivotTableRange: pivotSheet + "!A1:J20",
		Rows: []excelize.PivotTableField{
			{Data: "TA", Name: "TA"},
		},
		Columns: []excelize.PivotTableField{
			{Data: "Case in Technician", Name: "Case in Technician"},
			{Data: "Activity", Name: "Activity"},
		},
		Data: []excelize.PivotTableField{
			{Data: "WO Number", Name: fmt.Sprintf("Count of TA Activity @%v", now.Add(7*time.Hour).Format("02/Jan/2006 15:04:05")), Subtotal: "count"},
		},
		// PivotTableStyleName: "PivotStyleLight20",
		RowGrandTotals: true,
		ColGrandTotals: true,
		ShowDrill:      true,
		ShowRowHeaders: true,
		ShowColHeaders: true,
		ShowLastColumn: true,
		// ShowColStripes:      true,
	}); err != nil {
		log.Println("Error adding pivot table:", err)
	}

	// Notes in PIVOT
	// Only the date part is bold, the rest is normal
	noteText := "*Note: Date in Dashboard "
	dateText := time.Now().Format("2006-01-02")

	// Set the full value first
	f.SetCellValue(pivotSheet, "K1", noteText+dateText)

	// Apply bold only to the date part using RichText
	rich := []excelize.RichTextRun{
		{Text: noteText},
		{Text: dateText, Font: &excelize.Font{Bold: true}},
	}
	f.SetCellRichText(pivotSheet, "K1", rich)
	f.SetCellValue(pivotSheet, "K2", "Metric")
	f.SetCellValue(pivotSheet, "L2", "Value")
	f.SetCellValue(pivotSheet, "M2", "Description")

	// Set summary metric labels in column K
	f.SetCellValue(pivotSheet, "K3", "Total Pending Left")
	f.SetCellValue(pivotSheet, "K4", "Total Error Left")
	f.SetCellValue(pivotSheet, "K5", "Total Followed Up")
	f.SetCellValue(pivotSheet, "K6", "Total JO in Dashboard TA")
	f.SetCellValue(pivotSheet, "K7", "Total TA StandBy")
	f.SetCellValue(pivotSheet, "K8", "% Handled by TA")

	f.SetCellValue(pivotSheet, "M3", "Total data pending left in dashboard TA (Reason Code not A00)")
	f.SetCellValue(pivotSheet, "M4", "Total data with photo error by AI left in dashboard TA")
	f.SetCellValue(pivotSheet, "M5", "Total followed up pending & error data by TA")
	f.SetCellValue(pivotSheet, "M6", "Total JO @"+time.Now().Format("02 January 2006"))
	f.SetCellValue(pivotSheet, "M7", "Total person")
	f.SetCellValue(pivotSheet, "M8", "'=Followed Up / Total JO in Dashboard")

	var totalPendingLeft, totalErrorLeft, totalFollowedUp, totalTAStandBy int64
	_ = db.Model(&op_model.Pending{}).
		Where("date BETWEEN ? AND ?", startOfDay, endOfDay).
		Where("ta_feedback IS NULL").
		Count(&totalPendingLeft)
	_ = db.Model(&op_model.Error{}).
		Where("date BETWEEN ? AND ?", startOfDay, endOfDay).
		Where("ta_feedback IS NULL").
		Count(&totalErrorLeft)
	_ = db.Model(&op_model.LogAct{}).
		Where("date BETWEEN ? AND ?", startOfDay, endOfDay).
		Count(&totalFollowedUp)

	// Count totalTAStandBy using dbWeb.Model(&model.Admin{}) where LastLogin is today (00:00:00 to 23:59:59)
	startOfDayWeb := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
	endOfDayWeb := time.Date(now.Year(), now.Month(), now.Day(), 23, 59, 59, 0, now.Location())
	_ = dbWeb.Model(&model.Admin{}).
		Where("updated_at BETWEEN ? AND ?", startOfDayWeb, endOfDayWeb).
		Where("LOWER(fullname) LIKE ?", "%technical assistance%").
		Count(&totalTAStandBy)

	f.SetCellValue(pivotSheet, "L3", totalPendingLeft)
	f.SetCellValue(pivotSheet, "L4", totalErrorLeft)
	f.SetCellValue(pivotSheet, "L5", totalFollowedUp)
	f.SetCellValue(pivotSheet, "L7", totalTAStandBy)

	formulaL6 := fmt.Sprintf("=SUM(%s:%s)", "L3", "L5")
	formulaL6 = strings.ReplaceAll(formulaL6, "==", "=")
	if err := f.SetCellFormula(pivotSheet, "L6", formulaL6); err != nil {
		log.Println(err)
	}

	formulaL8 := fmt.Sprintf("=IF(%s=0,0,%s/%s)", "L6", "L5", "L6")
	formulaL8 = strings.ReplaceAll(formulaL8, "==", "=")
	if err := f.SetCellFormula(pivotSheet, "L8", formulaL8); err != nil {
		log.Println(err)
	}

	styleBold, _ := f.NewStyle(&excelize.Style{
		Font: &excelize.Font{
			Bold: true,
		},
		Alignment: &excelize.Alignment{
			Horizontal: "center",
			Vertical:   "center",
		},
	})
	styleBoldNotCenter, _ := f.NewStyle(&excelize.Style{
		Font: &excelize.Font{
			Bold: true,
		},
	})
	styleCenter, _ := f.NewStyle(&excelize.Style{
		Alignment: &excelize.Alignment{
			Horizontal: "center",
			Vertical:   "center",
		},
	})

	f.SetCellStyle(pivotSheet, "K2", "K2", styleBold)
	f.SetCellStyle(pivotSheet, "K8", "K8", styleBoldNotCenter)
	f.SetCellStyle(pivotSheet, "L2", "L2", styleBold)
	f.SetCellStyle(pivotSheet, "L8", "L8", styleBold)
	f.SetCellStyle(pivotSheet, "M2", "M2", styleBold)
	f.SetCellStyle(pivotSheet, "L3", "L3", styleCenter)
	f.SetCellStyle(pivotSheet, "L4", "L4", styleCenter)
	f.SetCellStyle(pivotSheet, "L5", "L5", styleCenter)
	f.SetCellStyle(pivotSheet, "L6", "L6", styleCenter)
	f.SetCellStyle(pivotSheet, "L7", "L7", styleCenter)
	// Set cell style for L8 as percentage with 2 decimal places and bold
	stylePercent, _ := f.NewStyle(&excelize.Style{
		NumFmt: 10, // 0.00%
		Font: &excelize.Font{
			Bold: true,
		},
		Alignment: &excelize.Alignment{
			Horizontal: "center",
			Vertical:   "center",
		},
	})
	f.SetCellStyle(pivotSheet, "L8", "L8", stylePercent)
	f.SetColWidth(pivotSheet, "K", "K", 25)
	f.SetColWidth(pivotSheet, "L", "L", 14)
	f.SetColWidth(pivotSheet, "M", "M", 60)

	// Technician pivot
	techSheet := "DATA TECHNICIAN"
	f.NewSheet(techSheet)
	f.AddPivotTable(&excelize.PivotTableOptions{
		DataRange:       pivotDataRange,
		PivotTableRange: techSheet + "!A1:O200",
		Rows: []excelize.PivotTableField{
			{Data: "Head", Name: "Head"},
			{Data: "SPL", Name: "SPL"},
			{Data: "Technician", Name: "Technician"},
		},
		Columns: []excelize.PivotTableField{
			{Data: "Case in Technician", Name: "Case in Technician"},
			{Data: "Problem", Name: "Case"},
		},
		Data: []excelize.PivotTableField{
			{Data: "Case in Technician", Name: fmt.Sprintf("Count of Pending/Error by Technician @%v", now.Add(7*time.Hour).Format("02/Jan/2006 15:04:05")), Subtotal: "count"},
		},
		RowGrandTotals: true,
		ColGrandTotals: true,
		ShowDrill:      true,
		ShowRowHeaders: true,
		ShowColHeaders: true,
		ShowLastColumn: true,
	})

	techMismatchSheet := "Technician Work Mismatch"
	f.NewSheet(techMismatchSheet)
	f.AddPivotTable(&excelize.PivotTableOptions{
		DataRange:       pivotDataRange,
		PivotTableRange: techMismatchSheet + "!A4:O200",
		Rows: []excelize.PivotTableField{
			{Data: "Head", Name: "Head"},
			{Data: "SPL", Name: "SPL"},
			{Data: "Technician", Name: "Technician"},
		},
		Columns: []excelize.PivotTableField{
			{Data: "Case in Technician", Name: "Case in Technician"},
		},
		Data: []excelize.PivotTableField{
			{
				Data:     "Case in Technician",
				Name:     fmt.Sprintf("Count of Technician Work Mismatch @%v", now.Add(7*time.Hour).Format("02/Jan/2006 15:04:05")),
				Subtotal: "count",
			},
		},
		Filter: []excelize.PivotTableField{
			{Data: "Activity", Name: "Activity"},
		},
		RowGrandTotals: true,
		ColGrandTotals: true,
		ShowDrill:      true,
		ShowRowHeaders: true,
		ShowColHeaders: true,
		ShowLastColumn: true,
	})
	f.SetCellValue(techMismatchSheet, "A1", "*Note: please select activity = edit to see the count of technician mismatch work")
	f.SetColWidth(techMismatchSheet, "A", "A", 65)

	// Mapping for photo columns
	photoColumnLinks := map[string]string{
		"Foto BAST":            "x_foto_bast",
		"Foto Media Promo":     "x_foto_ceklis",
		"Foto SN EDC":          "x_foto_edc",
		"Foto PIC Merchant":    "x_foto_pic",
		"Foto Pengaturan":      "x_foto_setting",
		"Foto Thermal":         "x_foto_thermal",
		"Foto Merchant":        "x_foto_toko",
		"Foto Surat Training":  "x_foto_training",
		"Foto Transaksi":       "x_foto_transaksi",
		"Tanda Tangan PIC":     "x_tanda_tangan_pic",
		"Tanda Tangan Teknisi": "x_tanda_tangan_teknisi",
		// New entries
		"Foto Stiker EDC":                 "x_foto_sticker_edc",
		"Foto Screen Gard":                "x_foto_screen_guard",
		"Foto Sales Draft All Memberbank": "x_foto_all_transaction",
		"Foto Sales Draft BMRI":           "x_foto_transaksi_bmri",
		"Foto Sales Draft BNI":            "x_foto_transaksi_bni",
		"Foto Sales Draft BRI":            "x_foto_transaksi_bri",
		"Foto Sales Draft BTN":            "x_foto_transaksi_btn",
		"Foto Sales Draft Patch L":        "x_foto_transaksi_patch",
		"Foto Screen P2G":                 "x_foto_screen_p2g",
		"Foto Kontak Stiker PIC":          "x_foto_kontak_stiker_pic",

		"Foto Selfie Video Call":           "x_foto_selfie_video_call",
		"Foto Selfie Teknisi dan Merchant": "x_foto_selfie_teknisi_merchant",
	}

	// Pending Sheet
	pendingSheet := "PENDING DATA LEFT"
	f.NewSheet(pendingSheet)
	pendingColumns := []struct {
		ColIndex string
		ColTitle string
		ColSize  float64
	}{
		{"A", "ID Task", 15},
		{"B", "WO Number", 28},
		{"C", "SPK", 35},
		{"D", "Received Date SPK", 35},
		{"E", "Company", 15},
		{"F", "Type", 35},
		{"G", "Type2", 35},
		{"H", "SLA Deadline", 35},
		{"I", "Keterangan", 35},
		{"J", "Description", 35},
		{"K", "Reason Code", 35},
		{"L", "TID", 35},
		{"M", "Merchant", 35},
		{"N", "Teknisi", 35},
		{"O", "Date in Dashboard", 35},
		{"P", "TA Feedback", 50},
		{"Q", "Foto BAST", 35},
		{"R", "Foto Media Promo", 35},
		{"S", "Foto SN EDC", 35},
		{"T", "Foto PIC Merchant", 35},
		{"U", "Foto Pengaturan", 35},
		{"V", "Foto Thermal", 35},
		{"W", "Foto Merchant", 35},
		{"X", "Foto Surat Training", 35},
		{"Y", "Foto Transaksi", 35},
		{"Z", "Tanda Tangan PIC", 35},
		{"AA", "Tanda Tangan Teknisi", 35},
		{"AB", "Foto Stiker EDC", 35},
		{"AC", "Foto Screen Gard", 35},
		{"AD", "Foto Sales Draft All Memberbank", 35},
		{"AE", "Foto Sales Draft BMRI", 35},
		{"AF", "Foto Sales Draft BNI", 35},
		{"AG", "Foto Sales Draft BRI", 35},
		{"AH", "Foto Sales Draft BTN", 35},
		{"AI", "Foto Sales Draft Patch L", 35},
		{"AJ", "Foto Screen P2G", 35},
		{"AK", "Foto Kontak Stiker PIC", 35},
		{"AL", "Foto Selfie Video Call", 35},
		{"AM", "Foto Selfie Teknisi dan Merchant", 35},
	}
	// Header setup
	for _, col := range pendingColumns {
		cell := fmt.Sprintf("%s1", col.ColIndex)
		f.SetCellValue(pendingSheet, cell, col.ColTitle)
		f.SetColWidth(pendingSheet, col.ColIndex, col.ColIndex, col.ColSize)
		f.SetCellStyle(pendingSheet, cell, cell, style)
	}
	f.AutoFilter(pendingSheet, "A1:AK1", nil)

	var pendingData []op_model.Pending
	err = db.Where("1=1").Find(&pendingData).Error
	if err != nil {
		return "", "", err
	}

	if len(pendingData) > 0 {
		rowIndex := 2
		for _, record := range pendingData {
			for _, column := range pendingColumns {
				cell := fmt.Sprintf("%s%d", column.ColIndex, rowIndex)
				var value interface{} = "N/A"

				// Handle dynamic photo column links
				if photoID, exists := photoColumnLinks[column.ColTitle]; exists {
					// This column is a photo, set the link accordingly
					linkPhoto := fmt.Sprintf("%v/here/file/%v@%v", os.Getenv("WEB_PUBLIC_URL"), record.IDTask, photoID)
					f.SetCellValue(pendingSheet, cell, fmt.Sprintf("View %v", column.ColTitle))
					f.SetCellStyle(pendingSheet, cell, cell, style)
					// Add hyperlink to the cell using SetCellHyperlink
					f.SetCellHyperLink(pendingSheet, cell, linkPhoto, "External")
				} else {
					switch column.ColTitle {
					case "ID Task":
						value = SafeValue(record.IDTask)
					case "WO Number":
						value = SafeValue(record.WoNumber)
					case "SPK":
						value = CleanSPKNumber(SafeValue(record.SpkNumber))
					case "Received Date SPK":
						value = SafeValue(record.ReceivedDatetimeSpk)
					case "Company":
						value = SafeValue(record.Company)
					case "Type":
						value = SafeValue(*record.Type)
					case "Type2":
						value = SafeValue(*record.Type2)
					case "SLA Deadline":
						value = SafeValue(*record.Sla)
					case "Keterangan":
						value = SafeValue(*record.Keterangan)
					case "Description":
						value = SafeValue(*record.Desc)
					case "Reason Code":
						value = SafeValue(record.Reason)
					case "TID":
						value = SafeValue(record.TID)
					case "Merchant":
						value = SafeValue(*record.Merchant)
					case "Teknisi":
						value = SafeValue(record.Teknisi)
					case "Date in Dashboard":
						value = SafeValue(record.Date)
					case "TA Feedback":
						value = SafeValue(record.TaFeedback)
					}
					f.SetCellValue(pendingSheet, cell, value)
					f.SetCellStyle(pendingSheet, cell, cell, style)
				}

			}
			rowIndex++
		}
	}

	// Error Sheet
	errorSheet := "ERROR DATA LEFT"
	f.NewSheet(errorSheet)

	errorColumns := []struct {
		ColIndex string
		ColTitle string
		ColSize  float64
	}{
		{"A", "ID Task", 15},
		{"B", "WO Number", 28},
		{"C", "SPK", 35},
		{"D", "Received Date SPK", 35},
		{"E", "Company", 15},
		{"F", "Type", 35},
		{"G", "Type2", 35},
		{"H", "SLA Deadline", 35},
		{"I", "Keterangan", 35},
		{"J", "Description", 35},
		{"K", "Reason Code", 35},
		{"L", "TID", 35},
		{"M", "Merchant", 35},
		{"N", "Teknisi", 35},
		{"O", "Problem", 35},
		{"P", "Date in Dashboard", 35},
		{"Q", "TA Feedback", 50},
		{"R", "Foto BAST", 35},
		{"S", "Foto Media Promo", 35},
		{"T", "Foto SN EDC", 35},
		{"U", "Foto PIC Merchant", 35},
		{"V", "Foto Pengaturan", 35},
		{"W", "Foto Thermal", 35},
		{"X", "Foto Merchant", 35},
		{"Y", "Foto Surat Training", 35},
		{"Z", "Foto Transaksi", 35},
		{"AA", "Tanda Tangan PIC", 35},
		{"AB", "Tanda Tangan Teknisi", 35},
		{"AC", "Foto Stiker EDC", 35},
		{"AD", "Foto Screen Gard", 35},
		{"AE", "Foto Sales Draft All Memberbank", 35},
		{"AF", "Foto Sales Draft BMRI", 35},
		{"AG", "Foto Sales Draft BNI", 35},
		{"AH", "Foto Sales Draft BRI", 35},
		{"AI", "Foto Sales Draft BTN", 35},
		{"AJ", "Foto Sales Draft Patch L", 35},
		{"AK", "Foto Screen P2G", 35},
		{"AL", "Foto Kontak Stiker PIC", 35},
		{"AM", "Foto Selfie Video Call", 35},
		{"AN", "Foto Selfie Teknisi dan Merchant", 35},
	}

	// Header setup
	for _, col := range errorColumns {
		cell := fmt.Sprintf("%s1", col.ColIndex)
		f.SetCellValue(errorSheet, cell, col.ColTitle)
		f.SetColWidth(errorSheet, col.ColIndex, col.ColIndex, col.ColSize)
		f.SetCellStyle(errorSheet, cell, cell, style)
	}

	f.AutoFilter(errorSheet, "A1:AL1", nil)

	var errorData []op_model.Error
	err = db.Where("1=1").Find(&errorData).Error
	if err != nil {
		return "", "", err
	}

	if len(errorData) > 0 {
		rowIndex := 2
		for _, record := range errorData {
			for _, column := range errorColumns {
				cell := fmt.Sprintf("%s%d", column.ColIndex, rowIndex)
				var value interface{} = "N/A"

				// Handle dynamic photo column links
				if photoID, exists := photoColumnLinks[column.ColTitle]; exists {
					// This column is a photo, set the link accordingly
					linkPhoto := fmt.Sprintf("%v/here/file/%v@%v", os.Getenv("WEB_PUBLIC_URL"), record.IDTask, photoID)
					f.SetCellValue(errorSheet, cell, fmt.Sprintf("View %v", column.ColTitle))
					f.SetCellStyle(errorSheet, cell, cell, style)
					// Add hyperlink to the cell using SetCellHyperlink
					f.SetCellHyperLink(errorSheet, cell, linkPhoto, "External")
				} else {
					// Handle regular data columns
					switch column.ColTitle {
					case "ID Task":
						value = SafeValue(record.IDTask)
					case "WO Number":
						value = SafeValue(record.WoNumber)
					case "SPK":
						value = CleanSPKNumber(SafeValue(record.SpkNumber))
					case "Received Date SPK":
						value = SafeValue(record.ReceivedDatetimeSpk)
					case "Company":
						value = SafeValue(record.Company)
					case "Type":
						value = SafeValue(*record.Type)
					case "Type2":
						value = SafeValue(*record.Type2)
					case "SLA Deadline":
						value = SafeValue(*record.Sla)
					case "Keterangan":
						value = SafeValue(*record.Keterangan)
					case "Description":
						value = SafeValue(*record.Desc)
					case "Reason Code":
						value = SafeValue(record.Reason)
					case "TID":
						value = SafeValue(record.TID)
					case "Merchant":
						value = SafeValue(*record.Merchant)
					case "Teknisi":
						value = SafeValue(record.Teknisi)
					case "Problem":
						value = SafeValue(*record.Problem)
					case "Date in Dashboard":
						value = SafeValue(record.Date)
					case "TA Feedback":
						value = SafeValue(record.TaFeedback)
					}
					f.SetCellValue(errorSheet, cell, value)
					f.SetCellStyle(errorSheet, cell, cell, style)
				}
			}
			rowIndex++
		}
	}

	// TA Feedback but got no response
	taFeedbackNoResponseSheet := "TA Feedback History"
	f.NewSheet(taFeedbackNoResponseSheet)
	titlesTaFeedbackNoResp := []struct {
		Title string
		Size  float64
	}{
		{"Sent at", 25},
		{"Sender", 25},
		{"Replied at", 25},
		{"Replied (Time)", 25},
		{"Replied by", 25},
		{"Reacted at", 25},
		{"Reacted (Time)", 25},
		{"Reacted by", 25},
		{"Message", 75},
		{"Reply Text", 75},
		{"Reaction Emoji", 75},
		{"Mentions", 45},
		{"Stanza ID", 35},
	}

	var columnsTaFeedbackNoResp []Column
	for i, t := range titlesTaFeedbackNoResp {
		columnsTaFeedbackNoResp = append(columnsTaFeedbackNoResp, Column{
			ColIndex: getColName(i),
			ColTitle: t.Title,
			ColSize:  t.Size,
		})
	}
	for _, col := range columnsTaFeedbackNoResp {
		cell := fmt.Sprintf("%s1", col.ColIndex)
		f.SetCellValue(taFeedbackNoResponseSheet, cell, col.ColTitle)
		f.SetColWidth(taFeedbackNoResponseSheet, col.ColIndex, col.ColIndex, col.ColSize)
		f.SetCellStyle(taFeedbackNoResponseSheet, cell, cell, style)
	}

	var dataWaMsg []op_model.WAMessage
	if err := dbWeb.
		// Where("message_type = ? AND replied_at IS NULL AND reacted_at IS NULL", "text").
		Where("message_type = ? AND sent_at BETWEEN ? AND ?", "text", startOfDay, endOfDay).
		Order("sent_at ASC").
		Find(&dataWaMsg).Error; err != nil {
		log.Print(err)
	}

	if len(dataWaMsg) > 0 {
		rowIndex := 2

		styleTaFeedbackNoResp, _ := f.NewStyle(&excelize.Style{
			Alignment: &excelize.Alignment{
				Horizontal: "center",
				Vertical:   "center",
				WrapText:   true,
			},
		})

		for _, record := range dataWaMsg {
			for _, column := range columnsTaFeedbackNoResp {
				cell := fmt.Sprintf("%s%d", column.ColIndex, rowIndex)
				var value interface{} = "N/A"

				var needToSetValue bool = true

				switch column.ColTitle {
				case "Sent at":
					if !record.SentAt.IsZero() {
						value = record.SentAt.Format("2006-01-02 15:04:05")
					}
				case "Replied at":
					if record.RepliedAt != nil && !record.RepliedAt.IsZero() {
						value = record.RepliedAt.Format("2006-01-02 15:04:05")
					}
				case "Reacted at":
					if record.ReactedAt != nil && !record.ReactedAt.IsZero() {
						value = record.ReactedAt.Format("2006-01-02 15:04:05")
					}
				case "Replied (Time)":
					if !record.SentAt.IsZero() && record.RepliedAt != nil && !record.RepliedAt.IsZero() {
						duration := record.RepliedAt.Add(7 * time.Hour).Sub(record.SentAt.Add(7 * time.Hour))
						h := int(duration.Hours())
						m := int(duration.Minutes()) % 60
						s := int(duration.Seconds()) % 60
						value = fmt.Sprintf("%02d:%02d:%02d", h, m, s)

						if duration > 15*time.Minute {
							needToSetValue = false
							styleID, err := f.NewStyle(&excelize.Style{
								Font: &excelize.Font{
									Color: "FF0000", // red
								},
								Alignment: &excelize.Alignment{
									Horizontal: "center",
									Vertical:   "center",
								},
							})
							if err != nil {
								log.Print(err)
							}
							f.SetCellValue(taFeedbackNoResponseSheet, cell, value)
							f.SetCellStyle(taFeedbackNoResponseSheet, cell, cell, styleID)
							break
						}
					} else {
						value = "N/A"
					}

				case "Reacted (Time)":
					if !record.SentAt.IsZero() && record.ReactedAt != nil && !record.ReactedAt.IsZero() {
						duration := record.ReactedAt.Add(7 * time.Hour).Sub(record.SentAt.Add(7 * time.Hour))
						h := int(duration.Hours())
						m := int(duration.Minutes()) % 60
						s := int(duration.Seconds()) % 60
						value = fmt.Sprintf("%02d:%02d:%02d", h, m, s)

						if duration > 15*time.Minute {
							needToSetValue = false
							styleID, err := f.NewStyle(&excelize.Style{
								Font: &excelize.Font{
									Color: "FF0000", // red
								},
								Alignment: &excelize.Alignment{
									Horizontal: "center",
									Vertical:   "center",
								},
							})
							if err != nil {
								log.Print(err)
							}
							f.SetCellValue(taFeedbackNoResponseSheet, cell, value)
							f.SetCellStyle(taFeedbackNoResponseSheet, cell, cell, styleID)
							break
						}
					} else {
						value = "N/A"
					}

				case "Sender":
					if record.SenderJID != "" {
						// Split at '@' to get the raw ID
						parts := strings.Split(record.SenderJID, "@")
						if len(parts) > 0 {
							rawID := parts[0]

							// Remove any colon suffix (e.g., :16)
							phoneParts := strings.Split(rawID, ":")
							cleanPhone := phoneParts[0]

							value = cleanPhone
						} else {
							value = ""
						}
					}
				case "Replied by":
					if record.RepliedBy != "" {
						// Split at '@' to get the raw ID
						parts := strings.Split(record.RepliedBy, "@")
						if len(parts) > 0 {
							rawID := parts[0]

							// Remove any colon suffix (e.g., :16)
							phoneParts := strings.Split(rawID, ":")
							cleanPhone := phoneParts[0]

							value = cleanPhone
						} else {
							value = ""
						}
					}
				case "Reacted by":
					if record.ReactedBy != "" {
						// Split at '@' to get the raw ID
						parts := strings.Split(record.ReactedBy, "@")
						if len(parts) > 0 {
							rawID := parts[0]

							// Remove any colon suffix (e.g., :16)
							phoneParts := strings.Split(rawID, ":")
							cleanPhone := phoneParts[0]

							value = cleanPhone
						} else {
							value = ""
						}
					}
				case "Reply Text":
					value = record.ReplyText
				case "Reaction Emoji":
					value = record.ReactionEmoji
				case "Stanza ID":
					value = record.ID
				case "Message":
					value = record.MessageBody
				case "Mentions":
					value = record.Mentions
				}

				if needToSetValue {
					f.SetCellValue(taFeedbackNoResponseSheet, cell, value)
					f.SetCellStyle(taFeedbackNoResponseSheet, cell, cell, styleTaFeedbackNoResp)
				}
			}
			rowIndex++
		}
	}

	lastColTaFeedbackNoResp := getColName(len(columnsTaFeedbackNoResp) - 1)
	filterRangeTaFeedbackNoResp := fmt.Sprintf("A1:%s1", lastColTaFeedbackNoResp)
	f.AutoFilter(taFeedbackNoResponseSheet, filterRangeTaFeedbackNoResp, []excelize.AutoFilterOptions{})

	// // Delete unneeded sheet
	// f.DeleteSheet(techSheet)
	// f.DeleteSheet(pendingSheet)
	// f.DeleteSheet(errorSheet)
	// f.DeleteSheet(techMismatchSheet)

	f.MoveSheet(pivotSheet, employeeSheet)
	f.SetActiveSheet(0)

	/* SAVE EXCEL */
	if err := f.SaveAs(excelFilePath); err != nil {
		return "", "", err
	}

	return excelFileName, excelFilePath, nil
}

func GetTAReport(db *gorm.DB, dbWeb *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		excelFileName, excelFilePath, err := GenerateTAExcelReport(db, dbWeb)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}

		if excelFileName == "" && excelFilePath == "" {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "File excel not found"})
			return
		}

		if _, err := os.Stat(excelFilePath); os.IsNotExist(err) {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "File not found"})
			return
		}

		c.Header("Content-Disposition", fmt.Sprintf("attachment; filename=%s", excelFileName))
		c.Header("Content-Type", "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet")
		c.File(excelFilePath)
	}
}

func GetTAMonthlyReport(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		excelFileName, excelFilePath, err := GenerateTAMonthlyExcelReport(db)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}

		if excelFileName == "" && excelFilePath == "" {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "File excel not found"})
			return
		}

		if _, err := os.Stat(excelFilePath); os.IsNotExist(err) {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "File not found"})
			return
		}

		c.Header("Content-Disposition", fmt.Sprintf("attachment; filename=%s", excelFileName))
		c.Header("Content-Type", "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet")
		c.File(excelFilePath)
	}
}

func GetTAComparedReport(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		excelFileName, excelFilePath, err := GenerateTAComparedReport(db)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}

		if excelFileName == "" && excelFilePath == "" {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "File excel not found"})
			return
		}

		if _, err := os.Stat(excelFilePath); os.IsNotExist(err) {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "File not found"})
			return
		}

		c.Header("Content-Disposition", fmt.Sprintf("attachment; filename=%s", excelFileName))
		c.Header("Content-Type", "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet")
		c.File(excelFilePath)
	}
}

// Generate Excel-style column names: A, B, ..., Z, AA, AB, ...
func getColName(n int) string {
	name := ""
	for n >= 0 {
		// name = string('A'+(n%26)) + name
		name = string(rune('A'+(n%26))) + name
		n = n/26 - 1
	}
	return name
}

func ODOOAPI(APIReq string, domain interface{}, model string, fields []string, order string) (interface{}, error) {
	fieldsJSON, err := json.Marshal(fields)
	if err != nil {
		return nil, fmt.Errorf("error marshaling fields: %v", err)
	}

	domainJSON, err := json.Marshal(domain)
	if err != nil {
		return nil, fmt.Errorf("error marshaling domain: %v", err)
	}

	requestJSON := `{
		"jsonrpc": "2.0", 
		"params": {
			"model": "%s",  
			"fields": %s,
			"domain": %s,
			"order": "%s"
		}
	}`

	rawJSON := fmt.Sprintf(requestJSON, model, string(fieldsJSON), string(domainJSON), order)

	switch APIReq {
	case "GetData":
		return ODOOGetData(rawJSON)
	// case "GetATMData":
	// 	return ODOOGetATMData(rawJSON)
	default:
		return nil, fmt.Errorf("unknown API request type: %s", APIReq)
	}
}

func ODOOGetData(req string) (interface{}, error) {
	yamlFilePaths := []string{
		"/config/conf.yaml",
		"config/conf.yaml",
		"../config/conf.yaml",
		"/../config/conf.yaml",
		"../../config/conf.yaml",
		"/../../config/conf.yaml",
	}

	var loadedConfig *config.YamlConfig
	var err error

	for _, filePath := range yamlFilePaths {
		if _, err := os.Stat(filePath); err == nil { // File exists
			// log.Printf("Attempting to load configuration from '%s'", filePath)
			loadedConfig, err = config.YAMLLoad(filePath)
			if err != nil {
				log.Printf("Failed to load configuration from '%s': %v", filePath, err)
				continue
			}
			// log.Printf("Configuration successfully loaded from '%s'", filePath)
			break
		} else if os.IsNotExist(err) {
			// log.Printf("Configuration file '%s' does not exist. Skipping.", filePath)
		} else {
			log.Printf("Error checking file '%s': %v", filePath, err)
		}
	}

	if loadedConfig == nil {
		log.Fatalf("Failed to load configuration: no valid configuration file found in paths: %v", yamlFilePaths)
	}

	urlGetData := loadedConfig.Odoo.UrlGetData

	maxRetriesStr := loadedConfig.Odoo.MaxRetry
	maxRetries, err := strconv.Atoi(maxRetriesStr)
	if err != nil {
		log.Printf("Invalid ODOO_MAX_RETRY value: %v", err)
		return nil, err
	}

	retryDelayStr := loadedConfig.Odoo.RetryDelay
	retryDelay, err := strconv.ParseInt(retryDelayStr, 0, 64)
	if err != nil {
		log.Printf("Invalid ODOO_RETRY_DELAY value: %v", err)
		return nil, err
	}

	reqTimeout, err := time.ParseDuration(loadedConfig.Odoo.GetDataTimeout)
	if err != nil {
		log.Printf("Invalid ODOO_GETDATA_TIMEOUT value: %v", err)
		return nil, err
	}

	var response *http.Response
	cookieODOO, err := GetSessionODOO()
	if err != nil {
		log.Printf("Got error while trying to get session ODOO: %v", err)
		return nil, err
	}
	// log.Printf("Cookies: %v", cookieODOO)

	for attempts := 1; attempts <= maxRetries; attempts++ {
		request, err := http.NewRequest("POST", urlGetData, bytes.NewBufferString(req))
		if err != nil {
			log.Printf("Error creating request: %v", err)
			return nil, err
		}

		request.Header.Set("Content-Type", "application/json")

		for _, cookie := range cookieODOO {
			request.AddCookie(cookie)
		}

		// Custom HTTP client with TLS verification disabled
		// client := &http.Client{
		// 	Timeout: reqTimeout,
		// 	Transport: &http.Transport{
		// 		TLSClientConfig: &tls.Config{InsecureSkipVerify: true}, // Skips TLS verification
		// 	},
		// }

		client := &http.Client{
			Timeout: reqTimeout,
			Transport: &http.Transport{
				DisableKeepAlives: true,
				TLSClientConfig:   &tls.Config{InsecureSkipVerify: true},
			},
		}

		// Send the request
		response, err = client.Do(request)
		if err != nil {
			log.Printf("Error making POST request (attempt %d/%d): %v", attempts, maxRetries, err)
			if attempts < maxRetries {
				time.Sleep(time.Duration(retryDelay) * time.Second) // Wait before retrying
				continue
			}
			return nil, err // Return error after final retry
		}

		// Check if the response is successful
		if response.StatusCode == http.StatusOK {
			break
		} else {
			log.Printf("Bad response, status code: %d (attempt %d/%d)", response.StatusCode, attempts, maxRetries)
			if attempts < maxRetries {
				response.Body.Close() // Close the body before retrying
				time.Sleep(time.Duration(retryDelay) * time.Second)
				continue
			}
			return nil, err // Return error if all attempts fail
		}
	}

	defer response.Body.Close()

	if response.StatusCode != http.StatusOK {
		log.Printf("POST request failed with status code: %v", response.StatusCode)
		return nil, err
	}

	body, err := ioutil.ReadAll(response.Body)
	if err != nil {
		errMsg := fmt.Errorf("error reading response body: %v from req: %v", err, req)
		log.Print(errMsg)
		return nil, errMsg
	}

	// fmt.Println("Response Body:", string(body))

	var jsonResponse map[string]interface{}
	err = json.Unmarshal(body, &jsonResponse)
	if err != nil {
		log.Printf("Error parsing JSON Response: %v", err)
		return nil, err
	}

	// Check for error response from Odoo
	if errorResponse, ok := jsonResponse["error"].(map[string]interface{}); ok {
		if errorMessage, ok := errorResponse["message"].(string); ok && errorMessage == "Odoo Session Expired" {
			log.Printf("Error code: %v, message: %v", errorResponse["code"], errorMessage)
			return nil, fmt.Errorf("error code: %v, message: %v", errorResponse["code"], errorMessage)
		}
	}

	// Check for the result in JSON response
	if result, ok := jsonResponse["result"].(map[string]interface{}); ok {
		// Log the message and success status if they exist
		if message, ok := result["message"].(string); ok {
			success, successOk := result["success"]
			log.Printf("ODOO Result, message: %v, status: %v", message, successOk && success == true)
		}
	}

	// Check for the existence and validity of the "result" field
	result, resultExists := jsonResponse["result"]
	if !resultExists {
		log.Print("Result field missing in the response!")
		log.Printf("Error with params: %v", bytes.NewBufferString(req))
		return nil, nil
	}

	// Check if the result is an array and ensure it's not empty
	resultArray, ok := result.([]interface{})
	if !ok || len(resultArray) == 0 {
		log.Print("Unexpected result format or empty result!")
		log.Printf("Error with params: %v", bytes.NewBufferString(req))
		return nil, nil
	}

	return result, nil
}

func GetSessionODOO() ([]*http.Cookie, error) {
	yamlFilePaths := []string{
		"/config/conf.yaml",
		"config/conf.yaml",
		"../config/conf.yaml",
		"/../config/conf.yaml",
		"../../config/conf.yaml",
		"/../../config/conf.yaml",
	}

	var loadedConfig *config.YamlConfig
	var err error

	for _, filePath := range yamlFilePaths {
		if _, err := os.Stat(filePath); err == nil { // File exists
			// log.Printf("Attempting to load configuration from '%s'", filePath)
			loadedConfig, err = config.YAMLLoad(filePath)
			if err != nil {
				log.Printf("Failed to load configuration from '%s': %v", filePath, err)
				continue
			}
			// log.Printf("Configuration successfully loaded from '%s'", filePath)
			break
		} else if os.IsNotExist(err) {
			// log.Printf("Configuration file '%s' does not exist. Skipping.", filePath)
		} else {
			log.Printf("Error checking file '%s': %v", filePath, err)
		}
	}

	if loadedConfig == nil {
		log.Fatalf("Failed to load configuration: no valid configuration file found in paths: %v", yamlFilePaths)
	}

	db := loadedConfig.Odoo.Db
	login := loadedConfig.Odoo.Login
	password := loadedConfig.Odoo.Password
	urlSession := loadedConfig.Odoo.UrlSession
	jsonRPC := loadedConfig.Odoo.JSONRPC

	requestJSON := `{
		"jsonrpc": %v,
		"params": {
			"db": "%s",
			"login": "%s",
			"password": "%s"
		}
	}`
	rawJSON := fmt.Sprintf(requestJSON, jsonRPC, db, login, password)

	maxRetriesStr := loadedConfig.Odoo.MaxRetry
	maxRetries, err := strconv.Atoi(maxRetriesStr)
	if err != nil {
		log.Printf("Invalid ODOO_MAX_RETRY value: %v", err)
		return nil, err
	}

	retryDelayStr := loadedConfig.Odoo.RetryDelay
	retryDelay, err := strconv.ParseInt(retryDelayStr, 0, 64)
	if err != nil {
		log.Printf("Invalid ODOO_RETRY_DELAY value: %v", err)
		return nil, err
	}

	reqTimeout, err := time.ParseDuration(loadedConfig.Odoo.SessionTimeout)
	if err != nil {
		log.Printf("Invalid ODOO_SESSION_TIMEOUT value: %v", err)
		return nil, err
	}

	var response *http.Response

	for attempts := 1; attempts <= maxRetries; attempts++ {
		request, err := http.NewRequest("POST", urlSession, bytes.NewBufferString(rawJSON))
		if err != nil {
			log.Printf("Error creating request: %v", err)
			// return nil, err
		}

		request.Header.Set("Content-Type", "application/json")

		// Custom HTTP client with TLS verification disabled
		client := &http.Client{
			Timeout: reqTimeout,
			Transport: &http.Transport{
				TLSClientConfig: &tls.Config{InsecureSkipVerify: true}, // Skips TLS verification
			},
		}

		// Send the request
		response, err = client.Do(request)
		if err != nil {
			log.Printf("Error making POST request (attempt %d/%d): %v", attempts, maxRetries, err)
			if attempts < maxRetries {
				time.Sleep(time.Duration(retryDelay) * time.Second) // Wait before retrying
				continue
			}
			return nil, err // Return error after final retry
		}

		// Check if the response is successful
		if response.StatusCode == http.StatusOK {
			break
		} else {
			log.Printf("Bad response, status code: %d (attempt %d/%d)", response.StatusCode, attempts, maxRetries)
			if attempts < maxRetries {
				response.Body.Close() // Close the body before retrying
				time.Sleep(time.Duration(retryDelay) * time.Second)
				continue
			}
			return nil, err // Return error if all attempts fail
		}
	}

	defer response.Body.Close()

	if response.StatusCode != http.StatusOK {
		log.Printf("POST request failed with status code: %v", response.StatusCode)
		return nil, err
	}

	_, err = ioutil.ReadAll(response.Body)
	if err != nil {
		log.Printf("Error reading response body: %v", err)
		return nil, err
	}

	// Store and return the cookies
	cookieODOO := response.Cookies()
	// log.Print("ODOO session obtained successfully.")
	return cookieODOO, nil
}

func GenerateTAMonthlyExcelReport(db *gorm.DB) (string, string, error) {
	now := time.Now()
	// Set startOfDay to the 1st day of the current month at 00:00:00
	startOfDay := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, now.Location()).Add(-7 * time.Hour)
	// Set endOfDay to the last day of the current month at 23:59:59
	endOfDay := time.Date(now.Year(), now.Month()+1, 0, 23, 59, 59, 0, now.Location()).Add(-7 * time.Hour)

	var monthlyData []op_model.LogAct
	err := db.Where("date_in_dashboard BETWEEN ? AND ? AND LOWER(method) = ?", startOfDay, endOfDay, "edit").
		Order("date_in_dashboard ASC").
		Find(&monthlyData).Error
	if err != nil {
		return "", "", err
	}
	if len(monthlyData) == 0 {
		return "", "", fmt.Errorf("no data found for generating the TA monthly report")
	}
	taActivityData := monthlyData

	mainDirPaths := []string{
		"web/file/report/monthly_data",
		"../web/file/report/monthly_data",
		"../../web/file/report/monthly_data",
		"/home/administrator/technical_assistance/web/file/report/monthly_data",
	}

	var selectedMainDir string
	for _, mainDir := range mainDirPaths {
		if _, err := os.Stat(mainDir); err == nil {
			selectedMainDir = mainDir
			break
		}
	}

	if selectedMainDir == "" {
		return "", "", fmt.Errorf("no valid report file dir in: %v", mainDirPaths)
	}

	fileReportDir := filepath.Join(selectedMainDir, now.Format("2006-01-02"))
	if err := os.MkdirAll(fileReportDir, os.ModePerm); err != nil {
		return "", "", err
	}

	if len(taActivityData) == 0 {
		return "", "", fmt.Errorf("%v", "no data found for generating the TA monthly report")
	}

	excelFileName := fmt.Sprintf("MonthlyTechnicianDataMisMatch(%v)Report.xlsx", now.Add(7*time.Hour).Format("02Jan2006_15-04-05"))
	excelFilePath := filepath.Join(fileReportDir, excelFileName)

	titles := []struct {
		Title string
		Size  float64
	}{
		{"Start Followed Up at", 25},
		{"End of Followed Up", 25},
		{"Followed Up (Time)", 25},
		{"Date in Dashboard", 25},
		{"Date in Dashboard (dd-mm-yyyy)", 35},
		{"TA", 18},
		{"Email TA", 20},
		{"Technician", 45},
		{"SPL", 45},
		{"Head", 35},
		{"WO Number", 25},
		{"SPK Number", 25},
		{"Type", 25},
		{"Type2", 25},
		{"SLA Deadline", 25},
		{"TID", 25},
		{"Reason Code", 25},
		{"Case in Technician", 25},
		{"Problem", 25},
		{"Activity", 20},
		{"TA Remark (During Deletion)", 50},
		{"TA Feedback", 50},
	}

	var columns []Column
	for i, t := range titles {
		columns = append(columns, Column{
			ColIndex: getColName(i),
			ColTitle: t.Title,
			ColSize:  t.Size,
		})
	}

	f := excelize.NewFile()

	employeeSheet := "EMPLOYEES"
	f.NewSheet(employeeSheet)
	titlesEmployee := []struct {
		Title string
		Size  float64
	}{
		{"Technician", 25},
		{"SPL", 20},
		{"Ops Head", 20},
	}
	var columnsEmployee []Column
	for i, t := range titlesEmployee {
		columnsEmployee = append(columnsEmployee, Column{
			ColIndex: getColName(i),
			ColTitle: t.Title,
			ColSize:  t.Size,
		})
	}
	for _, column := range columnsEmployee {
		f.SetCellValue(employeeSheet, fmt.Sprintf("%s1", column.ColIndex), column.ColTitle)
		f.SetColWidth(employeeSheet, column.ColIndex, column.ColIndex, column.ColSize)
	}

	lastColEmployee := getColName(len(columnsEmployee) - 1)
	filterRangeEmployee := fmt.Sprintf("A1:%s1", lastColEmployee)
	f.AutoFilter(employeeSheet, filterRangeEmployee, []excelize.AutoFilterOptions{})

	/*

		.start insert data for EMPLOYEES

	*/

	ODOOModel := "fs.technician"
	domain := []interface{}{
		[]interface{}{"active", "=", true},
	}
	fields := []string{"id", "name", "technician_code", "x_spl_leader"}
	order := "name asc"

	ODOOResponse, err := ODOOAPI("GetData", domain, ODOOModel, fields, order)
	if err != nil {
		log.Println(err)
	}

	ODOOResponseArray, ok := ODOOResponse.([]interface{})
	if !ok || len(ODOOResponseArray) == 0 {
		log.Println("ODOOResponse is not array")
	}

	ODOOResponseBytes, err := json.Marshal(ODOOResponseArray)
	if err != nil {
		log.Println(err)
	}

	var employeeODOOData []DataFSTechnician
	if err := json.Unmarshal(ODOOResponseBytes, &employeeODOOData); err != nil {
		log.Println(err)
	}

	if len(employeeODOOData) == 0 {
		log.Println("No data employee found")
	}

	employeeRowIndex := 2
	for _, record := range employeeODOOData {
		for _, column := range columnsEmployee {
			cell := fmt.Sprintf("%s%d", column.ColIndex, employeeRowIndex)
			var value string = "N/A"
			switch column.ColTitle {
			case "Technician":
				if record.Technician.String != "" {
					value = record.Technician.String
				}
				f.SetCellValue(employeeSheet, cell, value)
			case "SPL":
				if record.SPL.String != "" {
					value = record.SPL.String
				}
				f.SetCellValue(employeeSheet, cell, value)
			case "Ops Head":
				if record.OpsHead.String != "" {
					value = record.OpsHead.String
				}
				f.SetCellValue(employeeSheet, cell, value)
			}
		}
		employeeRowIndex++ // increment once per record (row), not per column
	}

	/*

		.end of data inserted for EMPLOYEES

	*/

	masterSheet := "MASTER"
	f.NewSheet(masterSheet)

	style, _ := f.NewStyle(&excelize.Style{
		Alignment: &excelize.Alignment{
			Horizontal: "center",
			Vertical:   "center",
		},
	})

	// Header setup
	for _, col := range columns {
		cell := fmt.Sprintf("%s1", col.ColIndex)
		f.SetCellValue(masterSheet, cell, col.ColTitle)
		f.SetColWidth(masterSheet, col.ColIndex, col.ColIndex, col.ColSize)
		f.SetCellStyle(masterSheet, cell, cell, style)
	}

	woDetailURL := os.Getenv("WO_DETAIL_URL")

	rowIndex := 2
	for _, record := range taActivityData {
		for _, column := range columns {
			cell := fmt.Sprintf("%s%d", column.ColIndex, rowIndex)
			var value interface{} = "N/A"

			var needToSetValue bool = true

			switch column.ColTitle {
			case "TA":
				if name, ok := taData[record.Email]; ok {
					value = name
				}
			case "Email TA":
				if record.Email != "" && record.Email != "0" {
					value = record.Email
				}
			case "Technician":
				if record.Teknisi != "" && record.Teknisi != "0" {
					value = record.Teknisi
				}
			case "SPL":
				needToSetValue = false
				formula := fmt.Sprintf(`IFERROR(VLOOKUP(G%d, %v!A:C, 2, FALSE), "N/A")`, rowIndex, employeeSheet)
				f.SetCellFormula(masterSheet, cell, formula)
			case "Head":
				needToSetValue = false
				formula := fmt.Sprintf(`IFERROR(VLOOKUP(G%d, %v!A:C, 3, FALSE), "N/A")`, rowIndex, employeeSheet)
				f.SetCellFormula(masterSheet, cell, formula)
			case "WO Number":
				if record.Wo != nil && *record.Wo != "" && *record.Wo != "0" {
					wo := *record.Wo
					link := fmt.Sprintf("%s/odooms-project-task/detailWO?wo_number=%s", woDetailURL, wo)
					f.SetCellHyperLink(masterSheet, cell, link, "External")
					value = wo
				}
			case "SPK Number":
				if record.SpkNumber != "" && record.SpkNumber != "0" {
					value = CleanSPKNumber(record.SpkNumber)
				}
			case "Type":
				if record.Type != "" && record.Type != "0" {
					value = record.Type
				}
			case "Type2":
				if record.Type2 != "" && record.Type2 != "0" {
					value = record.Type2
				}
			case "SLA Deadline":
				if record.Sla != "" && record.Sla != "0" {
					value = record.Sla
				}
			case "TID":
				if record.Tid != "" && record.Tid != "0" {
					value = record.Tid
				}
			case "Reason Code":
				if record.Rc != "" && record.Rc != "0" {
					value = record.Rc
				}
			case "Case in Technician":
				if record.TypeCase != "" && record.TypeCase != "0" {
					value = record.TypeCase
				}
			case "Problem":
				if record.Problem != "" && record.Problem != "0" {
					value = record.Problem
				}
			case "Activity":
				if record.Method != "" && record.Method != "0" {
					value = record.Method
				}
			case "Start Followed Up at":
				if record.DateOnCheck != nil && !record.DateOnCheck.IsZero() {
					value = record.DateOnCheck.Add(7 * time.Hour).Format("2006-01-02 15:04:05")
				} else {
					value = "N/A"
				}
			case "End of Followed Up":
				if !record.Date.IsZero() {
					value = record.Date.Add(7 * time.Hour).Format("2006-01-02 15:04:05")
				} else {
					value = "N/A"
				}
			case "Followed Up (Time)":
				if record.DateOnCheck != nil && !record.DateOnCheck.IsZero() && !record.Date.IsZero() {
					duration := record.Date.Add(7 * time.Hour).Sub(record.DateOnCheck.Add(7 * time.Hour))
					h := int(duration.Hours())
					m := int(duration.Minutes()) % 60
					s := int(duration.Seconds()) % 60
					value = fmt.Sprintf("%02d:%02d:%02d", h, m, s)

					// Check if duration exceeds 15 minutes
					if duration > 15*time.Minute {
						needToSetValue = false
						styleID, err := f.NewStyle(&excelize.Style{
							Font: &excelize.Font{
								Color: "FF0000", // red
							},
							Alignment: &excelize.Alignment{
								Horizontal: "center",
								Vertical:   "center",
							},
						})
						if err != nil {
							log.Print(err)
						}
						f.SetCellValue(masterSheet, cell, value)
						f.SetCellStyle(masterSheet, cell, cell, styleID)
						break
					}
				} else {
					value = "N/A"
				}
			case "Date in Dashboard":
				if record.DateInDashboard == "" {
					value = "N/A"
				} else {
					value = record.DateInDashboard
				}
			case "Date in Dashboard (dd-mm-yyyy)":
				if record.DateInDashboard == "" {
					value = "N/A"
				} else {
					parsedTime, err := time.Parse("2006-01-02 15:04:05", record.DateInDashboard)
					if err != nil {
						value = "N/A"
					} else {
						value = parsedTime.Format("02-Jan-2006")
					}
				}
			case "TA Remark (During Deletion)":
				if record.Reason != nil && *record.Reason != "" && *record.Reason != "0" {
					value = *record.Reason
				}
			case "TA Feedback":
				if record.TaFeedback == "" {
					value = "N/A"
				} else {
					value = record.TaFeedback
				}
			}

			if needToSetValue {
				f.SetCellValue(masterSheet, cell, value)
				f.SetCellStyle(masterSheet, cell, cell, style)
			}

		}
		rowIndex++
	}

	lastCol := getColName(len(columns) - 1)
	filterRange := fmt.Sprintf("A1:%s1", lastCol)

	f.AutoFilter(masterSheet, filterRange, []excelize.AutoFilterOptions{})
	f.DeleteSheet("Sheet1")

	pivotDataRange := fmt.Sprintf("%s!$A$1:$%s$%d", masterSheet, lastCol, rowIndex-1)
	monthlyPivotSheet := "Monthly PIVOT (Tech Mismatch)"
	f.NewSheet(monthlyPivotSheet)
	f.AddPivotTable(&excelize.PivotTableOptions{
		DataRange:       pivotDataRange,
		PivotTableRange: monthlyPivotSheet + "!A7:O200",
		Rows: []excelize.PivotTableField{
			{Data: "Technician", Name: "Technician"},
			{Data: "Problem", Name: "Problem"},
		},
		Columns: []excelize.PivotTableField{
			{Data: "Date in Dashboard (dd-mm-yyyy)", Name: "Date in Dashboard (dd-mm-yyyy)"},
		},
		Data: []excelize.PivotTableField{
			{
				Data:     "Case in Technician",
				Name:     fmt.Sprintf("Count of Technician Work Mismatch @%v", now.Add(7*time.Hour).Format("02/Jan/2006 15:04:05")),
				Subtotal: "count",
			},
		},
		Filter: []excelize.PivotTableField{
			{Data: "Head", Name: "Head"},
			{Data: "SPL", Name: "SPL"},
			{Data: "Case in Technician", Name: "Case in Technician"},
		},
		RowGrandTotals: true,
		ColGrandTotals: true,
		ShowDrill:      true,
		ShowRowHeaders: true,
		ShowColHeaders: true,
		ShowLastColumn: true,
	})
	styleNote, _ := f.NewStyle(&excelize.Style{
		Font: &excelize.Font{
			Bold: false,
			Size: 12,
		},
		Alignment: &excelize.Alignment{
			WrapText: true,
		},
	})
	f.SetCellValue(monthlyPivotSheet, "A1", `*Note:
	- Select "Head & SPL" to view the hierarchy: "Head" refers to Head of SPL (Service Point Leader).
	- Select "Case in Technician = error" to view photo upload issues detected by AI.
	- Select "Case in Technician = pending" to find technician uploads missing reason code [A00] Done.`)
	f.SetCellStyle(monthlyPivotSheet, "A1", "A1", styleNote)
	f.SetColWidth(monthlyPivotSheet, "A", "A", 56)
	f.SetColWidth(monthlyPivotSheet, "B", "B", 40)

	/* SAVE EXCEL */
	if err := f.SaveAs(excelFilePath); err != nil {
		return "", "", err
	}

	return excelFileName, excelFilePath, nil
}

func GenerateTAComparedReport(db *gorm.DB) (string, string, error) {
	now := time.Now()

	mainDirPaths := []string{
		"web/file/report/compared_report",
		"../web/file/report/compared_report",
		"../../web/file/report/compared_report",
		"/home/administrator/technical_assistance/web/file/report/compared_report",
	}

	var selectedMainDir string
	for _, mainDir := range mainDirPaths {
		if _, err := os.Stat(mainDir); err == nil {
			selectedMainDir = mainDir
			break
		}
	}

	if selectedMainDir == "" {
		return "", "", fmt.Errorf("no valid report file dir in: %v", mainDirPaths)
	}

	fileReportDir := filepath.Join(selectedMainDir, now.Format("2006-01-02"))
	if err := os.MkdirAll(fileReportDir, os.ModePerm); err != nil {
		return "", "", err
	}

	excelFileName := fmt.Sprintf("ComparedTAData(%v)Report.xlsx", now.Add(7*time.Hour).Format("02Jan2006_15-04-05"))
	excelFilePath := filepath.Join(fileReportDir, excelFileName)

	f := excelize.NewFile()

	style, _ := f.NewStyle(&excelize.Style{
		Alignment: &excelize.Alignment{
			Horizontal: "center",
			Vertical:   "center",
		},
	})

	employeeSheet := "EMPLOYEES"
	f.NewSheet(employeeSheet)
	titlesEmployee := []struct {
		Title string
		Size  float64
	}{
		{"Technician", 25},
		{"SPL", 20},
		{"Ops Head", 20},
	}
	var columnsEmployee []Column
	for i, t := range titlesEmployee {
		columnsEmployee = append(columnsEmployee, Column{
			ColIndex: getColName(i),
			ColTitle: t.Title,
			ColSize:  t.Size,
		})
	}
	for _, column := range columnsEmployee {
		f.SetCellValue(employeeSheet, fmt.Sprintf("%s1", column.ColIndex), column.ColTitle)
		f.SetColWidth(employeeSheet, column.ColIndex, column.ColIndex, column.ColSize)
	}

	lastColEmployee := getColName(len(columnsEmployee) - 1)
	filterRangeEmployee := fmt.Sprintf("A1:%s1", lastColEmployee)
	f.AutoFilter(employeeSheet, filterRangeEmployee, []excelize.AutoFilterOptions{})

	/*

		.start insert data for EMPLOYEES

	*/

	ODOOModel := "fs.technician"
	domain := []interface{}{
		[]interface{}{"active", "=", true},
	}
	fields := []string{"id", "name", "technician_code", "x_spl_leader"}
	order := "name asc"

	ODOOResponse, err := ODOOAPI("GetData", domain, ODOOModel, fields, order)
	if err != nil {
		log.Println(err)
	}

	ODOOResponseArray, ok := ODOOResponse.([]interface{})
	if !ok || len(ODOOResponseArray) == 0 {
		log.Println("ODOOResponse is not array")
	}

	ODOOResponseBytes, err := json.Marshal(ODOOResponseArray)
	if err != nil {
		log.Println(err)
	}

	var employeeODOOData []DataFSTechnician
	if err := json.Unmarshal(ODOOResponseBytes, &employeeODOOData); err != nil {
		log.Println(err)
	}

	if len(employeeODOOData) == 0 {
		log.Println("No data employee found")
	}

	employeeRowIndex := 2
	for _, record := range employeeODOOData {
		for _, column := range columnsEmployee {
			cell := fmt.Sprintf("%s%d", column.ColIndex, employeeRowIndex)
			var value string = "N/A"
			switch column.ColTitle {
			case "Technician":
				if record.Technician.String != "" {
					value = record.Technician.String
				}
				f.SetCellValue(employeeSheet, cell, value)
			case "SPL":
				if record.SPL.String != "" {
					value = record.SPL.String
				}
				f.SetCellValue(employeeSheet, cell, value)
			case "Ops Head":
				if record.OpsHead.String != "" {
					value = record.OpsHead.String
				}
				f.SetCellValue(employeeSheet, cell, value)
			}
		}
		employeeRowIndex++ // increment once per record (row), not per column
	}

	/*

		.end of data inserted for EMPLOYEES

	*/

	/*
	* .start get ODOO Data SLA Today
	 */

	// startOfDay := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location()).Add(-7 * time.Hour)
	// endOfDay := time.Date(now.Year(), now.Month(), now.Day(), 23, 59, 59, 0, now.Location()).Add(-7 * time.Hour)

	// Calculate the first day of two months ago at 01:00:00
	// startOfDay := time.Date(now.Year(), now.Month()-2, 1, 1, 0, 0, 0, now.Location()).Add(-7 * time.Hour)
	startOfDay := time.Date(now.Year(), now.Month()-1, 1, 1, 0, 0, 0, now.Location()).Add(-7 * time.Hour)
	// Calculate the last day of the current month at 23:59:59
	endOfDay := time.Date(now.Year(), now.Month()+1, 0, 23, 59, 59, 0, now.Location()).Add(-7 * time.Hour)
	// Format to "2006-01-02 15:04:05"

	startOfDayStr := startOfDay.Format("2006-01-02 15:04:05")
	endOfDayStr := endOfDay.Format("2006-01-02 15:04:05")

	ODOOModel = "project.task"
	excludedCompanyID := []int{
		1,  // PT. Global Smartweb Asia
		2,  // CIMB NIAGA
		4,  // MAP
		5,  // bRI
		6,  // test
		7,  // bNi Syariah
		8,  // ARRANET
		9,  // ovo CM
		14, // MCPAY
		16, // _export_.stock .............
		17, // HAUSJO
		18, // BCA
		23, // jangan
		24, // bandu
		25, // boleh
	}

	domain = []interface{}{
		[]interface{}{"active", "=", true},
		[]interface{}{"company_id", "!=", excludedCompanyID},
		[]interface{}{"x_received_datetime_spk", ">=", startOfDayStr},
		[]interface{}{"x_received_datetime_spk", "<=", endOfDayStr},
	}

	fieldsID := []string{
		"id",
	}

	fields = []string{
		"id",
		"x_merchant",
		"x_pic_merchant",
		"x_pic_phone",
		"partner_street",
		"x_title_cimb",
		"x_sla_deadline",
		"create_date",
		"x_received_datetime_spk",
		"planned_date_begin",
		"timesheet_timer_last_stop",
		"x_task_type",
		"worksheet_template_id",
		"x_ticket_type2",
		"company_id",
		"stage_id",
		"helpdesk_ticket_id",
		"x_cimb_master_mid",
		"x_cimb_master_tid",
		"x_source",
		"x_message_call",
		"x_no_task",
		"x_status_merchant",
		"x_studio_edc",
		"x_product",
		"x_wo_remark",
		"x_longitude",
		"x_latitude",
		"technician_id",
		"x_reason_code_id",
		"write_uid",
		"date_last_stage_update",
	}

	order = "id asc"

	ODOOResponse, err = ODOOAPI("GetData", domain, ODOOModel, fieldsID, order)
	if err != nil {
		log.Println(err)
	}

	ODOOResponseArray, ok = ODOOResponse.([]interface{})
	if !ok {
		log.Println("failed to asset results as []interface{}")
	}

	var ids []uint64

	for _, item := range ODOOResponseArray {
		// Assert item is a map
		recordMap, ok := item.(map[string]interface{})
		if !ok {
			continue // skip invalid records
		}

		// Extract and convert ID
		rawID, exists := recordMap["id"]
		if !exists {
			continue
		}

		// Convert to float64 first (JSON default), then to uint64
		floatID, ok := rawID.(float64)
		if !ok {
			continue // skip if not a number
		}

		uintID := uint64(floatID)
		ids = append(ids, uintID)
	}

	if len(ids) == 0 {
		log.Println("empty data in ODOO")
	}

	const batchSize = 1000
	chunks := chunkIdsSlice(ids, batchSize)

	var allRecords []interface{}

	log.Printf("Total IDs for Data Report: %d\n", len(ids))
	for i, chunk := range chunks {
		log.Printf("Processing Data chunk %d of %d (IDs %d to %d)\n", i+1, len(chunks), chunk[0], chunk[len(chunk)-1])

		chunkDomain := []interface{}{
			[]interface{}{"id", "=", chunk},
			[]interface{}{"active", "=", true},
		}

		ODOOResponse, err := ODOOAPI("GetData", chunkDomain, ODOOModel, fields, order)
		if err != nil {
			continue
		}

		ODOOResponseArray, ok := ODOOResponse.([]interface{})
		if !ok {
			continue
		}

		allRecords = append(allRecords, ODOOResponseArray...)
	}

	if len(allRecords) == 0 {
		log.Println("no data found from ODOO in all chunks")
	}

	ODOOResponseBytes, err = json.Marshal(allRecords)
	if err != nil {
		log.Println(err)
	}

	var taskDataODOO []DataProjectTask
	if err := json.Unmarshal(ODOOResponseBytes, &taskDataODOO); err != nil {
		log.Println(err)
	}

	taskDataSheet := "ODOO DATA (PROJECT.TASK)"
	f.NewSheet(taskDataSheet)
	titlesTaskData := []struct {
		Title string
		Size  float64
	}{
		{"ID", 12},
		{"WO Number", 26},
		{"Ticket Subject", 58},
		{"Stage", 32},
		{"Head", 26},
		{"SPL", 26},
		{"Technician", 28},
		{"Merchant Name", 35},
		{"PIC Merchant", 18},
		{"PIC Phone", 16},
		{"Merchant Address", 38},
		{"Description", 46},
		{"SLA Deadline", 35},
		{"Create Date", 35},
		{"Received Datetime SPK", 35},
		{"Planned At", 26},
		{"Timesheet Last Stop", 28},
		{"Task Type", 34},
		{"Worksheet Template", 48},
		{"Ticket Type", 44},
		{"Company", 24},
		{"MID", 34},
		{"TID", 34},
		{"Source", 22},
		{"Call Center Message", 58},
		{"Status Merchant", 26},
		{"SN EDC", 34},
		{"EDC Type", 24},
		{"WO Remark (Tiket)", 58},
		{"Longitude", 22},
		{"Latitude", 22},
		{"Reason Code", 14},
		{"Last Updated By", 24},
		{"Last Stage Updated", 24},
	}

	var columnsTaskData []Column
	for i, t := range titlesTaskData {
		columnsTaskData = append(columnsTaskData, Column{
			ColIndex: getColName(i),
			ColTitle: t.Title,
			ColSize:  t.Size,
		})
	}
	for _, column := range columnsTaskData {
		f.SetCellValue(taskDataSheet, fmt.Sprintf("%s1", column.ColIndex), column.ColTitle)
		f.SetColWidth(taskDataSheet, column.ColIndex, column.ColIndex, column.ColSize)
	}

	lastColTaskData := getColName(len(columnsTaskData) - 1)
	filterRangeTaskData := fmt.Sprintf("A1:%s1", lastColTaskData)
	f.AutoFilter(taskDataSheet, filterRangeTaskData, []excelize.AutoFilterOptions{})

	if len(taskDataODOO) > 0 {
		rowIndex := 2
		for _, record := range taskDataODOO {
			for _, column := range columnsTaskData {
				cell := fmt.Sprintf("%s%d", column.ColIndex, rowIndex)
				var value interface{} = "N/A"

				var needToSetValue bool = true

				var cleanedTicketNumber string = "N/A"
				_, ticketNumber, err := parseJSONIDDataCombined(record.HelpdeskTicketId)
				if err != nil {
					log.Println(err)
				}
				if ticketNumber != "" {
					cleanedTicketNumber = CleanSPKNumber(ticketNumber)
				}

				var stage string = "N/A"
				_, stage, err = parseJSONIDDataCombined(record.StageId)
				if err != nil {
					log.Println(err)
				}

				var technician string = "N/A"
				_, technician, err = parseJSONIDDataCombined(record.TechnicianId)
				if err != nil {
					log.Println(err)
				}

				var worksheetTemplate string = "N/A"
				_, worksheetTemplate, err = parseJSONIDDataCombined(record.WorksheetTemplateId)
				if err != nil {
					log.Println(err)
				}

				var ticketType string = "N/A"
				_, ticketType, err = parseJSONIDDataCombined(record.TicketTypeId)
				if err != nil {
					log.Println(err)
				}

				var company string = "N/A"
				_, company, err = parseJSONIDDataCombined(record.CompanyId)
				if err != nil {
					log.Println(err)
				}

				var snEdc string = "N/A"
				_, snEdc, err = parseJSONIDDataCombined(record.SnEdc)
				if err != nil {
					log.Println(err)
				}

				var edcType string = "N/A"
				_, edcType, err = parseJSONIDDataCombined(record.EdcType)
				if err != nil {
					log.Println(err)
				}

				var reasonCode string = "N/A"
				_, reasonCode, err = parseJSONIDDataCombined(record.ReasonCodeId)
				if err != nil {
					log.Println(err)
				}

				var lastUpdateBy string = "N/A"
				_, lastUpdateBy, err = parseJSONIDDataCombined(record.WriteUid)
				if err != nil {
					log.Println(err)
				}

				switch column.ColTitle {
				case "ID":
					if record.ID != 0 {
						value = record.ID
					}
				case "WO Number":
					value = record.WoNumber.String
				case "Ticket Subject":
					value = cleanedTicketNumber
				case "Stage":
					value = stage
				case "Head":
					// Try to get the head value directly from employeeODOOData
					headValue := "N/A"
					for _, emp := range employeeODOOData {
						if emp.Technician.String == technician {
							if emp.OpsHead.String != "" {
								headValue = emp.OpsHead.String
							}
							break
						}
					}
					value = headValue
				case "SPL":
					// Try to get the SPL value directly from employeeODOOData
					splValue := "N/A"
					for _, emp := range employeeODOOData {
						if emp.Technician.String == technician {
							if emp.SPL.String != "" {
								splValue = emp.SPL.String
							}
							break
						}
					}
					value = splValue
				case "Technician":
					value = technician
				case "Merchant Name":
					value = record.MerchantName.String
				case "PIC Merchant":
					value = record.PicMerchant.String
				case "PIC Phone":
					value = record.PicPhone.String
				case "Merchant Address":
					value = record.MerchantAddress.String
				case "Description":
					value = record.Description.String
				case "SLA Deadline":
					if !record.SlaDeadline.Time.IsZero() {
						value = record.SlaDeadline.Time.Format("2006-01-02 15:04:05")
					}
				case "Create Date":
					if !record.CreateDate.Time.IsZero() {
						value = record.CreateDate.Time.Format("2006-01-02 15:04:05")
					}
				case "Received Datetime SPK":
					if !record.ReceivedDatetimeSpk.Time.IsZero() {
						value = record.ReceivedDatetimeSpk.Time.Format("2006-01-02 15:04:05")
					}
				case "Planned At":
					if !record.PlanDate.Time.IsZero() {
						value = record.PlanDate.Time.Format("2006-01-02 15:04:05")
					}
				case "Timesheet Last Stop":
					if !record.TimesheetLastStop.Time.IsZero() {
						value = record.TimesheetLastStop.Time.Format("2006-01-02 15:04:05")
					}
				case "Task Type":
					value = record.TaskType.String
				case "Worksheet Template":
					value = worksheetTemplate
				case "Ticket Type":
					value = ticketType
				case "Company":
					value = company
				case "MID":
					value = record.Mid.String
				case "TID":
					value = record.Tid.String
				case "Source":
					value = record.Source.String
				case "Call Center Message":
					value = record.MessageCC.String
				case "Status Merchant":
					value = record.StatusMerchant.String
				case "SN EDC":
					value = snEdc
				case "EDC Type":
					value = edcType
				case "WO Remark (Tiket)":
					value = record.WoRemarkTiket.String
				case "Longitude":
					value = record.Longitude.String
				case "Latitude":
					value = record.Latitude.String
				case "Reason Code":
					value = reasonCode
				case "Last Updated By":
					value = lastUpdateBy
				case "Last Stage Updated":
					if !record.DateLastStageUpdate.Time.IsZero() {
						value = record.DateLastStageUpdate.Time.Format("2006-01-02 15:04:05")
					}
				}
				if needToSetValue {
					f.SetCellValue(taskDataSheet, cell, value)
					f.SetCellStyle(taskDataSheet, cell, cell, style)
				}
			}
			rowIndex++
		}
	}

	/*
	* .end of get ODOO Data SLA Today
	 */

	masterSheet := "LEFT DATA (ERROR & PENDING)"
	f.NewSheet(masterSheet)

	titles := []struct {
		Title string
		Size  float64
	}{
		{"ID Task", 15},
		{"SLA Deadline", 35},
		{"Date in Dashboard", 35},
		{"WO Number", 28},
		{"Ticket Subject", 35},
		{"Status in ODOO", 35},
		{"Received Date SPK", 35},
		{"Company", 15},
		{"Type", 35},
		{"Type2", 35},
		{"Keterangan", 35},
		{"Description", 35},
		{"Reason Code", 35},
		{"TID", 35},
		{"Merchant", 35},
		{"Head", 35},
		{"SPL", 35},
		{"Teknisi", 35},
		{"Problem", 55},
		{"TA Feedback", 50},
		{"Foto BAST", 35},
		{"Foto Media Promo", 35},
		{"Foto SN EDC", 35},
		{"Foto PIC Merchant", 35},
		{"Foto Pengaturan", 35},
		{"Foto Thermal", 35},
		{"Foto Merchant", 35},
		{"Foto Surat Training", 35},
		{"Foto Transaksi", 35},
		{"Tanda Tangan PIC", 35},
		{"Tanda Tangan Teknisi", 35},
		{"Foto Stiker EDC", 35},
		{"Foto Screen Gard", 35},
		{"Foto Sales Draft All Memberbank", 35},
		{"Foto Sales Draft BMRI", 35},
		{"Foto Sales Draft BNI", 35},
		{"Foto Sales Draft BRI", 35},
		{"Foto Sales Draft BTN", 35},
		{"Foto Sales Draft Patch L", 35},
		{"Foto Screen P2G", 35},
		{"Foto Kontak Stiker PIC", 35},
		{"Foto Selfie Video Call", 35},
		{"Foto Selfie Teknisi dan Merchant", 35},
	}

	var columns []Column
	for i, t := range titles {
		columns = append(columns, Column{
			ColIndex: getColName(i),
			ColTitle: t.Title,
			ColSize:  t.Size,
		})
	}

	// Header setup
	for _, col := range columns {
		cell := fmt.Sprintf("%s1", col.ColIndex)
		f.SetCellValue(masterSheet, cell, col.ColTitle)
		f.SetColWidth(masterSheet, col.ColIndex, col.ColIndex, col.ColSize)
		f.SetCellStyle(masterSheet, cell, cell, style)
	}

	rowIndex := 2

	// Get data left in Error
	var errorData []op_model.Error
	err = db.Where("1=1").Order("date DESC").Find(&errorData).Error
	if err != nil {
		return "", "", err
	}

	// Get data left in Pending
	var pendingData []op_model.Pending
	err = db.Where("1=1").Order("date DESC").Find(&pendingData).Error
	if err != nil {
		return "", "", err
	}

	// Mapping for photo columns
	photoColumnLinks := map[string]string{
		"Foto BAST":            "x_foto_bast",
		"Foto Media Promo":     "x_foto_ceklis",
		"Foto SN EDC":          "x_foto_edc",
		"Foto PIC Merchant":    "x_foto_pic",
		"Foto Pengaturan":      "x_foto_setting",
		"Foto Thermal":         "x_foto_thermal",
		"Foto Merchant":        "x_foto_toko",
		"Foto Surat Training":  "x_foto_training",
		"Foto Transaksi":       "x_foto_transaksi",
		"Tanda Tangan PIC":     "x_tanda_tangan_pic",
		"Tanda Tangan Teknisi": "x_tanda_tangan_teknisi",
		// New entries
		"Foto Stiker EDC":                 "x_foto_sticker_edc",
		"Foto Screen Gard":                "x_foto_screen_guard",
		"Foto Sales Draft All Memberbank": "x_foto_all_transaction",
		"Foto Sales Draft BMRI":           "x_foto_transaksi_bmri",
		"Foto Sales Draft BNI":            "x_foto_transaksi_bni",
		"Foto Sales Draft BRI":            "x_foto_transaksi_bri",
		"Foto Sales Draft BTN":            "x_foto_transaksi_btn",
		"Foto Sales Draft Patch L":        "x_foto_transaksi_patch",
		"Foto Screen P2G":                 "x_foto_screen_p2g",
		"Foto Kontak Stiker PIC":          "x_foto_kontak_stiker_pic",

		"Foto Selfie Video Call":           "x_foto_selfie_video_call",
		"Foto Selfie Teknisi dan Merchant": "x_foto_selfie_teknisi_merchant",
	}

	woDetailURL := os.Getenv("WO_DETAIL_URL")

	if len(errorData) > 0 {
		for _, record := range errorData {
			for _, column := range columns {
				cell := fmt.Sprintf("%s%d", column.ColIndex, rowIndex)
				var value interface{} = "N/A"

				var needToSetValue bool = true

				// Handle dynamic photo column links
				if photoID, exists := photoColumnLinks[column.ColTitle]; exists {
					// This column is a photo, set the link accordingly
					linkPhoto := fmt.Sprintf("%v/here/file/%v@%v", os.Getenv("WEB_PUBLIC_URL"), record.IDTask, photoID)
					f.SetCellValue(masterSheet, cell, fmt.Sprintf("View %v", column.ColTitle))
					f.SetCellStyle(masterSheet, cell, cell, style)
					// Add hyperlink to the cell using SetCellHyperlink
					f.SetCellHyperLink(masterSheet, cell, linkPhoto, "External")
				} else {
					switch column.ColTitle {
					case "ID Task":
						if record.IDTask != "" {
							value = record.IDTask
						}
					case "SLA Deadline":
						if record.Sla != nil && *record.Sla != "" {
							value = *record.Sla
						}
					case "Date in Dashboard":
						if !record.Date.IsZero() {
							value = record.Date.Add(7 * time.Hour).Format("2006-01-02 15:04:05")
						}
					case "WO Number":
						if record.WoNumber != "" {
							wo := record.WoNumber
							link := fmt.Sprintf("%s/odooms-project-task/detailWO?wo_number=%s", woDetailURL, wo)
							f.SetCellHyperLink(masterSheet, cell, link, "External")
							value = wo
						}
					case "Ticket Subject":
						if record.SpkNumber != "" {
							value = CleanSPKNumber(record.SpkNumber)
						}
					case "Status in ODOO":
						// Find the matching ID Task in taskDataSheet and get its Stage value
						matchedStage := "N/A"
						for _, task := range taskDataODOO {
							if fmt.Sprintf("%v", task.ID) == record.IDTask {
								_, stage, err := parseJSONIDDataCombined(task.StageId)
								if err == nil && stage != "" {
									matchedStage = stage
								}
								break
							}
						}

						idColIndex := ""
						stageColIndex := ""
						for _, col := range columnsTaskData {
							if col.ColTitle == "ID" {
								idColIndex = col.ColIndex
							}
							if col.ColTitle == "Stage" {
								stageColIndex = col.ColIndex
							}
						}

						if idColIndex != "" && stageColIndex != "" {
							formula := fmt.Sprintf(
								`=IFERROR(HYPERLINK("#'%s'!%s"&MATCH(--$A%d,'%s'!$%s:$%s,0), INDEX('%s'!$%s:$%s, MATCH(--$A%d, '%s'!$%s:$%s, 0))), "N/A")`,
								taskDataSheet, stageColIndex, rowIndex,
								taskDataSheet, idColIndex, idColIndex,
								taskDataSheet, stageColIndex, stageColIndex,
								rowIndex, taskDataSheet, idColIndex, idColIndex,
							)
							// fmt.Println("Generated Formula:", formula) // ✅ debug output
							err := f.SetCellFormula(masterSheet, cell, formula)
							if err != nil {
								fmt.Println("SetCellFormula error:", err)
							}
							needToSetValue = false
						}
						value = matchedStage

						// Set background fill color based on stage
						var fillColor string
						switch matchedStage {
						case "New":
							fillColor = "#FFFF00"
						case "Cancel":
							fillColor = "#FF0000"
						case "Done":
							fillColor = "#00B050"
						case "Verified":
							fillColor = "#99FF99"
						case "Open Pending":
							fillColor = "#FFA500"
						default:
							fillColor = "#Ffffff"
						}

						styleID, err := f.NewStyle(&excelize.Style{
							Fill: excelize.Fill{
								Type:    "pattern",
								Color:   []string{fillColor},
								Pattern: 1,
							},
						})
						if err != nil {
							fmt.Println("Style error:", err)
						} else {
							f.SetCellStyle(masterSheet, cell, cell, styleID)
						}
					case "Received Date SPK":
						if record.ReceivedDatetimeSpk != "" {
							value = record.ReceivedDatetimeSpk
						}
					case "Company":
						if record.Company != "" {
							value = record.Company
						}
					case "Type":
						if record.Type != nil && *record.Type != "" {
							value = *record.Type
						}
					case "Type2":
						if record.Type2 != nil && *record.Type2 != "" {
							value = *record.Type2
						}
					case "Keterangan":
						if record.Keterangan != nil && *record.Keterangan != "" {
							value = *record.Keterangan
						}
					case "Description":
						if record.Desc != nil && *record.Desc != "" {
							value = *record.Desc
						}
					case "Reason Code":
						if record.Reason != "" {
							value = record.Reason
						}
					case "TID":
						if record.TID != "" {
							value = record.TID
						}
					case "Merchant":
						if record.Merchant != nil && *record.Merchant != "" {
							value = *record.Merchant
						}
					case "Head":
						needToSetValue = false
						// Find the column index for "Teknisi"
						teknisiColIndex := ""
						for _, col := range columns {
							if col.ColTitle == "Teknisi" {
								teknisiColIndex = col.ColIndex
								break
							}
						}
						if teknisiColIndex != "" {
							formula := fmt.Sprintf(`IFERROR(VLOOKUP(%s%d, %v!A:C, 3, FALSE), "N/A")`, teknisiColIndex, rowIndex, employeeSheet)
							f.SetCellFormula(masterSheet, cell, formula)
						} else {
							f.SetCellValue(masterSheet, cell, "N/A")
						}
					case "SPL":
						needToSetValue = false
						// Find the column index for "Teknisi"
						teknisiColIndex := ""
						for _, col := range columns {
							if col.ColTitle == "Teknisi" {
								teknisiColIndex = col.ColIndex
								break
							}
						}
						if teknisiColIndex != "" {
							formula := fmt.Sprintf(`IFERROR(VLOOKUP(%s%d, %v!A:C, 2, FALSE), "N/A")`, teknisiColIndex, rowIndex, employeeSheet)
							f.SetCellFormula(masterSheet, cell, formula)
						} else {
							f.SetCellValue(masterSheet, cell, "N/A")
						}
					case "Teknisi":
						if record.Teknisi != "" {
							value = record.Teknisi
						}
					case "Problem":
						if record.Problem != nil && *record.Problem != "" {
							value = *record.Problem
						}
					case "TA Feedback":
						if record.TaFeedback != "" {
							value = record.TaFeedback
						}
					}
					if needToSetValue {
						f.SetCellValue(masterSheet, cell, value)
						f.SetCellStyle(masterSheet, cell, cell, style)
					}
				}
			}
			rowIndex++
		}
	}

	if len(pendingData) > 0 {
		for _, record := range pendingData {
			for _, column := range columns {
				cell := fmt.Sprintf("%s%d", column.ColIndex, rowIndex)
				var value interface{} = "N/A"

				var needToSetValue bool = true

				// Handle dynamic photo column links
				if photoID, exists := photoColumnLinks[column.ColTitle]; exists {
					// This column is a photo, set the link accordingly
					linkPhoto := fmt.Sprintf("%v/here/file/%v@%v", os.Getenv("WEB_PUBLIC_URL"), record.IDTask, photoID)
					f.SetCellValue(masterSheet, cell, fmt.Sprintf("View %v", column.ColTitle))
					f.SetCellStyle(masterSheet, cell, cell, style)
					// Add hyperlink to the cell using SetCellHyperlink
					f.SetCellHyperLink(masterSheet, cell, linkPhoto, "External")
				} else {
					switch column.ColTitle {
					case "ID Task":
						if record.IDTask != "" {
							value = record.IDTask
						}
					case "SLA Deadline":
						if record.Sla != nil && *record.Sla != "" {
							value = *record.Sla
						}
					case "Date in Dashboard":
						if !record.Date.IsZero() {
							value = record.Date.Add(7 * time.Hour).Format("2006-01-02 15:04:05")
						}
					case "WO Number":
						if record.WoNumber != "" {
							wo := record.WoNumber
							link := fmt.Sprintf("%s/odooms-project-task/detailWO?wo_number=%s", woDetailURL, wo)
							f.SetCellHyperLink(masterSheet, cell, link, "External")
							value = wo
						}
					case "Ticket Subject":
						if record.SpkNumber != "" {
							value = CleanSPKNumber(record.SpkNumber)
						}
					case "Status in ODOO":
						// Find the matching ID Task in taskDataSheet and get its Stage value
						matchedStage := "N/A"
						for _, task := range taskDataODOO {
							if fmt.Sprintf("%v", task.ID) == record.IDTask {
								_, stage, err := parseJSONIDDataCombined(task.StageId)
								if err == nil && stage != "" {
									matchedStage = stage
								}
								break
							}
						}

						idColIndex := ""
						stageColIndex := ""
						for _, col := range columnsTaskData {
							if col.ColTitle == "ID" {
								idColIndex = col.ColIndex
							}
							if col.ColTitle == "Stage" {
								stageColIndex = col.ColIndex
							}
						}

						if idColIndex != "" && stageColIndex != "" {
							formula := fmt.Sprintf(
								`=IFERROR(HYPERLINK("#'%s'!%s"&MATCH(--$A%d,'%s'!$%s:$%s,0), INDEX('%s'!$%s:$%s, MATCH(--$A%d, '%s'!$%s:$%s, 0))), "N/A")`,
								taskDataSheet, stageColIndex, rowIndex,
								taskDataSheet, idColIndex, idColIndex,
								taskDataSheet, stageColIndex, stageColIndex,
								rowIndex, taskDataSheet, idColIndex, idColIndex,
							)
							// fmt.Println("Generated Formula:", formula) // ✅ debug output
							err := f.SetCellFormula(masterSheet, cell, formula)
							if err != nil {
								fmt.Println("SetCellFormula error:", err)
							}
							needToSetValue = false
						}
						value = matchedStage

						// Set background fill color based on stage
						var fillColor string
						switch matchedStage {
						case "New":
							fillColor = "#FFFF00"
						case "Cancel":
							fillColor = "#FF0000"
						case "Done":
							fillColor = "#00B050"
						case "Verified":
							fillColor = "#99FF99"
						case "Open Pending":
							fillColor = "#FFA500"
						default:
							fillColor = "#Ffffff"
						}

						styleID, err := f.NewStyle(&excelize.Style{
							Fill: excelize.Fill{
								Type:    "pattern",
								Color:   []string{fillColor},
								Pattern: 1,
							},
						})
						if err != nil {
							fmt.Println("Style error:", err)
						} else {
							f.SetCellStyle(masterSheet, cell, cell, styleID)
						}
					case "Received Date SPK":
						if record.ReceivedDatetimeSpk != "" {
							value = record.ReceivedDatetimeSpk
						}
					case "Company":
						if record.Company != "" {
							value = record.Company
						}
					case "Type":
						if record.Type != nil && *record.Type != "" {
							value = *record.Type
						}
					case "Type2":
						if record.Type2 != nil && *record.Type2 != "" {
							value = *record.Type2
						}
					case "Keterangan":
						if record.Keterangan != nil && *record.Keterangan != "" {
							value = *record.Keterangan
						}
					case "Description":
						if record.Desc != nil && *record.Desc != "" {
							value = *record.Desc
						}
					case "Reason Code":
						if record.Reason != "" {
							value = record.Reason
						}
					case "TID":
						if record.TID != "" {
							value = record.TID
						}
					case "Merchant":
						if record.Merchant != nil && *record.Merchant != "" {
							value = *record.Merchant
						}
					case "Head":
						needToSetValue = false
						// Find the column index for "Teknisi"
						teknisiColIndex := ""
						for _, col := range columns {
							if col.ColTitle == "Teknisi" {
								teknisiColIndex = col.ColIndex
								break
							}
						}
						if teknisiColIndex != "" {
							formula := fmt.Sprintf(`IFERROR(VLOOKUP(%s%d, %v!A:C, 3, FALSE), "N/A")`, teknisiColIndex, rowIndex, employeeSheet)
							f.SetCellFormula(masterSheet, cell, formula)
						} else {
							f.SetCellValue(masterSheet, cell, "N/A")
						}
					case "SPL":
						needToSetValue = false
						// Find the column index for "Teknisi"
						teknisiColIndex := ""
						for _, col := range columns {
							if col.ColTitle == "Teknisi" {
								teknisiColIndex = col.ColIndex
								break
							}
						}
						if teknisiColIndex != "" {
							formula := fmt.Sprintf(`IFERROR(VLOOKUP(%s%d, %v!A:C, 2, FALSE), "N/A")`, teknisiColIndex, rowIndex, employeeSheet)
							f.SetCellFormula(masterSheet, cell, formula)
						} else {
							f.SetCellValue(masterSheet, cell, "N/A")
						}
					case "Teknisi":
						if record.Teknisi != "" {
							value = record.Teknisi
						}
					case "Problem":
						value = "N/A"
					case "TA Feedback":
						if record.TaFeedback != "" {
							value = record.TaFeedback
						}
					}
					if needToSetValue {
						f.SetCellValue(masterSheet, cell, value)
						f.SetCellStyle(masterSheet, cell, cell, style)
					}
				}
			}
			rowIndex++
		}
	}

	lastCol := getColName(len(columns) - 1)
	filterRange := fmt.Sprintf("A1:%s1", lastCol)

	f.AutoFilter(masterSheet, filterRange, []excelize.AutoFilterOptions{})
	f.DeleteSheet("Sheet1")

	// pivotDataRange := fmt.Sprintf("%s!$A$1:$%s$%d", masterSheet, lastCol, rowIndex-1)

	/* SAVE EXCEL */
	if err := f.SaveAs(excelFilePath); err != nil {
		return "", "", err
	}

	return excelFileName, excelFilePath, nil
}
