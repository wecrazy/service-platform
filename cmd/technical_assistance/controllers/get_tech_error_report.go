package controllers

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"service-platform/cmd/technical_assistance/model/op_model"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/xuri/excelize/v2"
	"gorm.io/gorm"
)

func GetTechErrorReport(db *gorm.DB, dbWeb *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		excelFileName, excelFilePath, err := GenerateTechErrorReport(db, dbWeb)
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

func GenerateTechErrorReport(db *gorm.DB, dbWeb *gorm.DB) (string, string, error) {
	taskDoing := "Tech Error Report"

	now := time.Now()
	startOfDay := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location()).Add(-7 * time.Hour)
	endOfDay := time.Date(now.Year(), now.Month(), now.Day(), 23, 59, 59, 0, now.Location()).Add(-7 * time.Hour)

	var taActivityData []op_model.LogAct
	err := db.
		Where("date BETWEEN ? AND ?", startOfDay, endOfDay).
		Where("LOWER(type_case) = ?", "error").
		Where("LOWER(method) = ?", "edit").
		Find(&taActivityData).
		Error
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
		return "", "", fmt.Errorf("no data found for generating the %s", taskDoing)
	}

	excelFileName := fmt.Sprintf("TechErrorReport(%v)Report.xlsx", now.Add(7*time.Hour).Format("02Jan2006_15-04-05"))
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

	problemSheet := "Tech Major Problems"
	f.NewSheet(problemSheet)
	f.AddPivotTable(&excelize.PivotTableOptions{
		DataRange:       pivotDataRange,
		PivotTableRange: problemSheet + "!A9:C200",
		Rows: []excelize.PivotTableField{
			{Data: "Problem", Name: "Problem"},
		},
		Data: []excelize.PivotTableField{
			{
				Data:     "Technician",
				Subtotal: "count",
			},
		},
		Filter: []excelize.PivotTableField{
			{Data: "Head", Name: "Head"},
			{Data: "SPL", Name: "SPL"},
			{Data: "Technician", Name: "Technician"},
			{Data: "SPK Number", Name: "SPK Number"},
			{Data: "WO Number", Name: "WO Number"},
			{Data: "Type2", Name: "Type2"},
		},
		RowGrandTotals: true,
		ColGrandTotals: true,
		ShowDrill:      true,
		ShowRowHeaders: true,
		ShowColHeaders: true,
		ShowLastColumn: true,
	})
	f.SetCellValue(problemSheet, "A1", "Major technician photo problems. You can filter the data by Head, SPL, Technician, SPK Number, WO Number & Ticket Type.")
	f.SetColWidth(problemSheet, "A", "A", 92)

	f.AddPivotTable(&excelize.PivotTableOptions{
		DataRange:       pivotDataRange,
		PivotTableRange: problemSheet + "!D9:M200",
		Rows: []excelize.PivotTableField{
			{Data: "Head", Name: "Head"},
		},
		Data: []excelize.PivotTableField{
			{
				Data:     "Problem",
				Subtotal: "count",
			},
		},
		Filter: []excelize.PivotTableField{
			{Data: "SPL", Name: "SPL"},
			{Data: "Technician", Name: "Technician"},
		},
		RowGrandTotals: true,
		ColGrandTotals: true,
		ShowDrill:      true,
		ShowRowHeaders: true,
		ShowColHeaders: true,
		ShowLastColumn: true,
	})
	f.SetColWidth(problemSheet, "D", "D", 18)

	f.MoveSheet(problemSheet, employeeSheet)
	f.SetActiveSheet(0)

	/* SAVE EXCEL */
	if err := f.SaveAs(excelFilePath); err != nil {
		return "", "", err
	}

	return excelFileName, excelFilePath, nil
}
