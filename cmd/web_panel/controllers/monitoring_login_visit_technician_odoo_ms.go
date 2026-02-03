package controllers

// Import markerSymbolUnicode from marker_symbol_unicode.go

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"runtime"
	"service-platform/cmd/web_panel/config"
	"service-platform/cmd/web_panel/fun"
	"service-platform/cmd/web_panel/internal/gormdb"
	"service-platform/cmd/web_panel/model"
	odooms "service-platform/cmd/web_panel/model/odoo_ms"
	sptechnicianmodel "service-platform/cmd/web_panel/model/sp_technician_model"
	"strings"
	"sync"
	"time"

	"github.com/sirupsen/logrus"
	"github.com/xuri/excelize/v2"
)

var (
	getMonitoringVisitAndLoginTechnicianMutex sync.Mutex
	getDataOfTechnicianODOOMSMutex            sync.Mutex
)

type MonitoringLoginVisitTechnicianChartResumeSeries struct {
	ColorLine     string  // Hex color code (6 digits)
	ColorMarker   string  // Hex color code (6 digits)
	MarkerSymbol  string  // Marker symbol for the chart e.g. "circle", "square", "diamond", "triangle", "cross"
	MarkerSize    int     // Size of the marker
	LineThickness float64 // Thickness of the line
}

func MonitoringVisitAndLoginTechnicianODOOMS() (string, error) {
	taskDoing := "Show Summary Report of Visit & Login Technician ODOO MS"
	if !getMonitoringVisitAndLoginTechnicianMutex.TryLock() {
		return "", fmt.Errorf("%s still running, please wait until it's finished", taskDoing)
	}
	defer getMonitoringVisitAndLoginTechnicianMutex.Unlock()

	if err := GetDataOfTechnicianODOOMSForMonitoring(); err != nil {
		return "", fmt.Errorf("%s failed: %v", taskDoing, err)
	}

	loc, _ := time.LoadLocation(config.GetConfig().Default.Timezone)
	now := time.Now().In(loc)
	numDays := time.Date(now.Year(), now.Month()+1, 0, 0, 0, 0, 0, loc).Day()

	reportName := fmt.Sprintf("Monitoring_Visit_and_Login_Technician_%v.xlsx", now.Format("02Jan2006"))
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
	sheetDaysWork := "Days Work"
	sheetMaster := "MASTER"
	sheetPivot := "PIVOT"
	sheetSPPivot := "SP_PIVOT"
	sheetActive := "ACTIVE"
	sheetResume := "RESUME"
	sheetChart := "CHART"

	f.NewSheet(sheetDaysWork)
	f.NewSheet(sheetMaster)
	f.NewSheet(sheetPivot)
	f.NewSheet(sheetSPPivot)
	f.NewSheet(sheetActive)
	f.NewSheet(sheetChart)
	indexSheetResume, err := f.NewSheet(sheetResume)
	if err != nil {
		return "", fmt.Errorf("failed to create resume sheet: %v", err)
	}
	_ = indexSheetResume

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
		return "", fmt.Errorf("failed to create general style: %v", err)
	}
	styleBold, err := f.NewStyle(&excelize.Style{
		Font: &excelize.Font{
			Bold: true,
		},
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
		return "", fmt.Errorf("failed to create bold style: %v", err)
	}

	styleTitleMaster, err := f.NewStyle(styleExcelTitle("#0ADAFF", false, "#000000"))
	if err != nil {
		return "", fmt.Errorf("failed to create master title style: %v", err)
	}

	styleTitleActive, err := f.NewStyle(styleExcelTitle("#FFFF00", false, "#000000"))
	if err != nil {
		return "", fmt.Errorf("failed to create active title style: %v", err)
	}

	styleCountActiveTechnicians, err := f.NewStyle(styleExcelTitle("#03B727", true, "#FFFFFF"))
	if err != nil {
		return "", fmt.Errorf("failed to create count active technician style: %v", err)
	}

	styleCountRegisteredTechniciansInODOOMS, err := f.NewStyle(styleExcelTitle("#000000", true, "#FFFFFF"))
	if err != nil {
		return "", fmt.Errorf("failed to create count registered technician style: %v", err)
	}

	styleResumeNo, err := f.NewStyle(styleExcelTitle("#9BBB59", false, "#FFFFFF"))
	if err != nil {
		return "", fmt.Errorf("failed to create resume no style: %v", err)
	}
	styleResumeActivity, err := f.NewStyle(styleExcelTitle("#FFFF00", false, "#000000"))
	if err != nil {
		return "", fmt.Errorf("failed to create resume activity style: %v", err)
	}
	styleResumeDay, err := f.NewStyle(styleExcelTitle("#00B0F0", false, "#FFFFFF"))
	if err != nil {
		return "", fmt.Errorf("failed to create resume day style: %v", err)
	}
	styleResumeGrandTotal, err := f.NewStyle(styleExcelTitle("#8CA8CC", false, "#FFFFFF"))
	if err != nil {
		return "", fmt.Errorf("failed to create resume grand total style: %v", err)
	}

	// Days Work
	f.SetCellValue(sheetDaysWork, "A1", fmt.Sprintf("Working Days (%s)", strings.ToUpper(now.Format("January 2006"))))
	styleDaysWorkTitle, err := f.NewStyle(&excelize.Style{
		Font: &excelize.Font{
			Bold:      true,
			Underline: "single",
			Size:      13,
		},
	})
	if err != nil {
		return "", fmt.Errorf("failed to create style for days work title: %v", err)
	}
	f.SetCellStyle(sheetDaysWork, "A1", "A1", styleDaysWorkTitle)

	titleDaysWork := []struct {
		Title    string
		ColWidth float64
	}{
		{"Day", 15},
		{"Work", 15},
	}
	var columnsDayWorks []ExcelColumn
	for i, t := range titleDaysWork {
		columnsDayWorks = append(columnsDayWorks, ExcelColumn{
			ColIndex: fun.GetColName(i),
			ColTitle: t.Title,
			ColSize:  t.ColWidth,
		})
	}

	for _, col := range columnsDayWorks {
		cell := fmt.Sprintf("%s3", col.ColIndex)
		f.SetColWidth(sheetDaysWork, col.ColIndex, col.ColIndex, col.ColSize)
		f.SetCellValue(sheetDaysWork, cell, col.ColTitle)
		f.SetCellStyle(sheetDaysWork, cell, cell, style)
	}

	rowIndex := 4
	for day := 1; day <= numDays; day++ {
		// Column A: show the day number as before
		cellDay := fmt.Sprintf("A%d", rowIndex)
		f.SetCellValue(sheetDaysWork, cellDay, day)
		f.SetCellStyle(sheetDaysWork, cellDay, cellDay, style)

		// Column B: only background color, no text
		cellWork := fmt.Sprintf("B%d", rowIndex)

		currentDate := time.Date(now.Year(), now.Month(), day, 0, 0, 0, 0, loc)

		// Check for public holidays first
		dataLibur, err := fun.GetHolidaysBasedOnYearMonthAndDay(
			now.Year(), int(now.Month()), day,
		)
		if err != nil {
			return "", fmt.Errorf("failed to get public holiday: %v", err)
		}

		// Determine color based on weekends OR public holidays
		workColor := "#00FF00" // default green for working days
		isPublicHoliday := false
		commentText := ""

		// Check if it's a weekend
		if currentDate.Weekday() == time.Saturday || currentDate.Weekday() == time.Sunday {
			workColor = "#FF0000" // red for weekends
		}

		// Check if it's a public holiday (regardless of weekend)
		if dataLibur.IsHoliday && len(dataLibur.HolidayList) > 0 {
			workColor = "#FF0000" // red for public holidays
			isPublicHoliday = true
			holidays := strings.Join(dataLibur.HolidayList, ", ")
			commentText = holidays
		}

		// Add comment ONLY for public holidays (not weekends)
		if isPublicHoliday && commentText != "" {
			if err := f.AddComment(sheetDaysWork, excelize.Comment{
				Cell:   cellWork,
				Author: "Report Service",
				Paragraph: []excelize.RichTextRun{
					{Text: commentText, Font: &excelize.Font{Bold: true}},
				},
				Width:  300,
				Height: 50,
			}); err != nil {
				return "", fmt.Errorf("failed to add comment for public holiday: %v", err)
			}
		}

		// Apply fill color without setting a value
		styleWork, err := f.NewStyle(styleExcelTitle(workColor, true, "#000000"))
		if err != nil {
			return "", fmt.Errorf("failed to create work status style: %v", err)
		}
		f.SetCellStyle(sheetDaysWork, cellWork, cellWork, styleWork)

		rowIndex++
	}

	// Master
	titleMaster := []struct {
		Title    string
		ColWidth float64
	}{
		{"Data Created on", 35},
		{"Data Created on (Day)", 35},
		{"Technician", 40},
		{"Technician Group", 20},
		{"City", 40},
		{"Province", 40},
		{"Full Name", 35},
		{"SPL", 35},
		{"SAC", 35},
		{"Last Login", 35},
		{"Last Download JO", 35},
		{"First Uploaded JO", 35},
		{"Latest Visit", 35},
		{"WO Numbers (Planned)", 35},
		{"WO Numbers (Visited)", 35},
		{"Link Photos (WO Planned)", 35},
		{"Link Photos (WO Visited)", 35},
		{"Is Got SP 1", 15},
		{"SP 1 Replied", 35},
		{"Is Got SP 2", 15},
		{"SP 2 Replied", 35},
		{"Is Got SP 3", 15},
		{"SP 3 Replied", 35},
	}
	var columnsMaster []ExcelColumn
	for i, t := range titleMaster {
		columnsMaster = append(columnsMaster, ExcelColumn{
			ColIndex: fun.GetColName(i),
			ColTitle: t.Title,
			ColSize:  t.ColWidth,
		})
	}

	var colMasterSACIndex,
		colMasterSPLIndex,
		colMasterTechnicianIndex,
		colMasterSP1GivenIndex,
		colMasterSP2GivenIndex,
		colMasterSP3GivenIndex,
		colMasterDataCreatedOnDayIndex,
		colMasterLatestVisitIndex,
		colMasterLastLoginIndex,
		colWONumbersPlannedIndex,
		colWONumbersVisitedIndex,
		colLinkPhotosWOPlannedIndex,
		colLinkPhotosWOVisitedIndex,
		colSP1RepliedIndex,
		colSP2RepliedIndex,
		colSP3RepliedIndex string

	for _, col := range columnsMaster {
		cell := fmt.Sprintf("%s1", col.ColIndex)
		f.SetColWidth(sheetMaster, col.ColIndex, col.ColIndex, col.ColSize)
		f.SetCellValue(sheetMaster, cell, col.ColTitle)
		f.SetCellStyle(sheetMaster, cell, cell, styleTitleMaster)

		switch col.ColTitle {
		case "SAC":
			colMasterSACIndex = col.ColIndex
		case "SPL":
			colMasterSPLIndex = col.ColIndex
		case "Technician":
			colMasterTechnicianIndex = col.ColIndex
		case "Is Got SP 1":
			colMasterSP1GivenIndex = col.ColIndex
		case "Is Got SP 2":
			colMasterSP2GivenIndex = col.ColIndex
		case "Is Got SP 3":
			colMasterSP3GivenIndex = col.ColIndex
		case "Data Created on (Day)":
			colMasterDataCreatedOnDayIndex = col.ColIndex
		case "Latest Visit":
			colMasterLatestVisitIndex = col.ColIndex
		case "Last Login":
			colMasterLastLoginIndex = col.ColIndex
		case "WO Numbers (Planned)":
			colWONumbersPlannedIndex = col.ColIndex
		case "WO Numbers (Visited)":
			colWONumbersVisitedIndex = col.ColIndex
		case "Link Photos (WO Planned)":
			colLinkPhotosWOPlannedIndex = col.ColIndex
		case "Link Photos (WO Visited)":
			colLinkPhotosWOVisitedIndex = col.ColIndex
		case "SP 1 Replied":
			colSP1RepliedIndex = col.ColIndex
		case "SP 2 Replied":
			colSP2RepliedIndex = col.ColIndex
		case "SP 3 Replied":
			colSP3RepliedIndex = col.ColIndex
		}
	}
	// For now we are not using these variables, but we keep them for future use
	_ = colWONumbersPlannedIndex
	_ = colWONumbersVisitedIndex
	_ = colLinkPhotosWOPlannedIndex
	_ = colLinkPhotosWOVisitedIndex
	_ = colSP1RepliedIndex
	_ = colSP2RepliedIndex
	_ = colSP3RepliedIndex

	lastColMaster := fun.GetColName(len(columnsMaster) - 1)
	filterRangeMaster := fmt.Sprintf("A1:%s1", lastColMaster)
	f.AutoFilter(sheetMaster, filterRangeMaster, []excelize.AutoFilterOptions{})

	dbWeb := gormdb.Databases.Web

	// Filter for current month data only
	monthStart := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, loc)
	monthEnd := time.Date(now.Year(), now.Month()+1, 0, 23, 59, 59, 0, loc)

	var countData int64
	if err := dbWeb.Model(&odooms.ODOOMSTechnicianData{}).
		Where("created_at >= ? AND created_at <= ?", monthStart, monthEnd).
		Count(&countData).Error; err != nil {
		return "", fmt.Errorf("failed to count technician data: %v", err)
	}

	if countData == 0 {
		return "", fmt.Errorf("no technician data found for current month (%s)", now.Format("January 2006"))
	}

	const batchSize = 2000 // Increased batch size significantly for 100k rows
	var offset int
	rowIndex = 2

	// Preload all SP data for the month in parallel
	logrus.Info("Pre-loading SP, WhatsApp messages, and users...")
	var allTechnicianSPs []sptechnicianmodel.TechnicianGotSP
	var allSPLSPs []sptechnicianmodel.SPLGotSP
	var allWAMessages []sptechnicianmodel.SPWhatsAppMessage
	var allWAUsers []model.WAPhoneUser
	var wg sync.WaitGroup
	wg.Add(4)

	go func() {
		defer wg.Done()
		dbWeb.Where("DATE(created_at) >= DATE(?) AND DATE(created_at) <= DATE(?)", monthStart, monthEnd).
			Find(&allTechnicianSPs)
		logrus.Infof("Loaded %d technician SP records", len(allTechnicianSPs))
	}()

	go func() {
		defer wg.Done()
		dbWeb.Where("DATE(created_at) >= DATE(?) AND DATE(created_at) <= DATE(?)", monthStart, monthEnd).
			Find(&allSPLSPs)
		logrus.Infof("Loaded %d SPL SP records", len(allSPLSPs))
	}()

	go func() {
		defer wg.Done()
		dbWeb.
			Unscoped(). // include soft-deleted messages
			Where("created_at >= ? AND created_at <= ?", monthStart, monthEnd).
			Find(&allWAMessages)
		logrus.Infof("Loaded %d WhatsApp messages", len(allWAMessages))
	}()

	go func() {
		defer wg.Done()
		dbWeb.Find(&allWAUsers)
		logrus.Infof("Loaded %d WhatsApp users", len(allWAUsers))
	}()

	wg.Wait()

	// Build lookup maps
	technicianSPMap := make(map[string]sptechnicianmodel.TechnicianGotSP, len(allTechnicianSPs))
	splSPMap := make(map[string]sptechnicianmodel.SPLGotSP, len(allSPLSPs))
	waMessageMap := make(map[string][]sptechnicianmodel.SPWhatsAppMessage, len(allWAMessages))
	waUserMap := make(map[string]model.WAPhoneUser, len(allWAUsers))

	for _, sp := range allTechnicianSPs {
		key := fmt.Sprintf("%s_%s", sp.Technician, sp.CreatedAt.Format("2006-01-02"))
		technicianSPMap[key] = sp
	}
	for _, sp := range allSPLSPs {
		key := fmt.Sprintf("%s_%s", sp.SPL, sp.CreatedAt.Format("2006-01-02"))
		splSPMap[key] = sp
	}
	for _, msg := range allWAMessages {
		// Use composite key for technician/SPL and SP number
		var key string
		if msg.TechnicianGotSPID != nil && *msg.TechnicianGotSPID != 0 {
			key = fmt.Sprintf("TECH_%d_%d_%s", *msg.TechnicianGotSPID, msg.NumberOfSP, msg.WhatSP)
		} else if msg.SPLGotSPID != nil && *msg.SPLGotSPID != 0 {
			key = fmt.Sprintf("SPL_%d_%d_%s", *msg.SPLGotSPID, msg.NumberOfSP, msg.WhatSP)
		}
		if key != "" {
			waMessageMap[key] = append(waMessageMap[key], msg)
		}
	}
	for _, user := range allWAUsers {
		waUserMap[user.PhoneNumber] = user
	}

	// Pre-load and cache Indonesia region data with optimized mapping
	logrus.Info("Pre-loading region data...")
	var allRegions []model.IndonesiaRegion
	dbWeb.Select("province, district, subdistrict, area").Find(&allRegions) // Only select needed fields

	// Create comprehensive region mapping for faster lookups
	regionCache := make(map[string]string, 1000) // Pre-allocate capacity

	// Build region index for O(1) lookups
	for _, region := range allRegions {
		province := region.Province
		// Index by district
		if region.District != "" {
			regionCache[strings.ToLower(region.District)] = province
		}
		// Index by subdistrict
		if region.Subdistrict != "" {
			regionCache[strings.ToLower(region.Subdistrict)] = province
		}
		// Index by area
		if region.Area != "" {
			regionCache[strings.ToLower(region.Area)] = province
		}
	}
	logrus.Infof("Built region cache with %d entries", len(regionCache))

	// Cache for parsed technician data to avoid repeated parsing
	technicianParseCache := make(map[string]*DataTechnicianODOOMSBasedOnName, 1000)

	// City name completion map
	completedCity := config.GetConfig().Indonesia.CompletedCity

	// Pre-compile strings.ToLower for SPL check to avoid repeated computation
	splCheckMap := make(map[string]bool, 1000)

	// Progress tracking for large datasets
	totalProcessed := 0
	startTime := time.Now()

	// Use existing month variables for data filtering
	for {
		batchStartTime := time.Now()
		var dataBatch []odooms.ODOOMSTechnicianData
		if err := dbWeb.
			Where("created_at >= ? AND created_at <= ?", monthStart, monthEnd).
			Order("created_at asc").
			Order("technician asc").
			Limit(batchSize).
			Offset(offset).Find(&dataBatch).Error; err != nil {
			return "", fmt.Errorf("failed to fetch technician data batch: %v", err)
		}

		if len(dataBatch) == 0 {
			break // No more data to process
		}

		// Process batch with optimized loops
		for _, data := range dataBatch {
			// Use cached SP lookup instead of individual queries
			dateKey := data.CreatedAt.Format("2006-01-02")
			spKey := fmt.Sprintf("%s_%s", data.Technician, dateKey)

			var technicianSP sptechnicianmodel.TechnicianGotSP
			var splSP sptechnicianmodel.SPLGotSP
			var spFound bool

			// Optimize SPL check with caching
			var isSPL bool
			if cached, exists := splCheckMap[data.Technician]; exists {
				isSPL = cached
			} else {
				isSPL = strings.Contains(strings.ToLower(data.Technician), "spl")
				splCheckMap[data.Technician] = isSPL
			}

			if isSPL {
				if cachedSP, exists := splSPMap[spKey]; exists {
					splSP = cachedSP
					spFound = true
				}
			} else {
				if cachedSP, exists := technicianSPMap[spKey]; exists {
					technicianSP = cachedSP
					spFound = true
				}
			} // Cache technician parsing results
			var dataTechParsed *DataTechnicianODOOMSBasedOnName
			if cached, exists := technicianParseCache[data.Technician]; exists {
				dataTechParsed = cached
			} else {
				dataTechParsed = ParsedDataTechnicianODOOMS(data.Technician)
				technicianParseCache[data.Technician] = dataTechParsed
			}

			// Optimize city/province lookup with caching
			var city, province string
			if dataTechParsed != nil && dataTechParsed.City != "" {
				city = dataTechParsed.City
				if val, exists := completedCity[strings.ToLower(city)]; exists {
					city = val
				}

				// Check cache first
				if cachedProvince, exists := regionCache[strings.ToLower(city)]; exists {
					province = cachedProvince
				} else {
					// Find province from pre-loaded regions
					for _, region := range allRegions {
						cityLower := strings.ToLower(city)
						if strings.Contains(strings.ToLower(region.District), cityLower) ||
							strings.Contains(strings.ToLower(region.Subdistrict), cityLower) ||
							strings.Contains(strings.ToLower(region.Area), cityLower) {
							province = region.Province
							regionCache[cityLower] = province // Cache the result
							break
						}
					}
				}
			}

			for _, column := range columnsMaster {
				cell := fmt.Sprintf("%s%d", column.ColIndex, rowIndex)
				var value interface{}
				var needToSetValue bool = true

				// SP Data
				var techSP1, techSP2, techSP3 bool = false, false, false
				if spFound {
					if strings.Contains(strings.ToLower(data.Technician), "spl") {
						techSP1 = splSP.IsGotSP1
						techSP2 = splSP.IsGotSP2
						techSP3 = splSP.IsGotSP3
					} else {
						techSP1 = technicianSP.IsGotSP1
						techSP2 = technicianSP.IsGotSP2
						techSP3 = technicianSP.IsGotSP3
					}
				}

				var techSP1Replied, techSP2Replied, techSP3Replied string
				// Use waMessageMap and waUserMap for fast lookup
				if techSP1 {
					var key string
					if strings.Contains(strings.ToLower(data.Technician), "spl") {
						key = fmt.Sprintf("SPL_%d_%d_%s", splSP.ID, 1, "SP_SPL")
					} else {
						key = fmt.Sprintf("TECH_%d_%d_%s", technicianSP.ID, 1, "SP_TECHNICIAN")
					}
					if msgs, exists := waMessageMap[key]; exists && len(msgs) > 0 {
						var messages []string
						for _, msg := range msgs {
							if msg.WhatsappReplyText != "" {
								senderPhone := extractPhoneFromJID(msg.WhatsappChatJID)
								senderName := senderPhone
								if user, ok := waUserMap[senderPhone]; ok {
									senderName = fmt.Sprintf("(%s %s)", user.FullName, user.PhoneNumber)
								}
								messages = append(messages,
									fmt.Sprintf("@%v From: %s ~%s",
										msg.CreatedAt.Format("Monday, 02 Jan 2006 15:04:05 MST"),
										senderName,
										msg.WhatsappReplyText,
									),
								)
							}
						}
						if len(messages) > 0 {
							techSP1Replied = strings.Join(messages, fmt.Sprintf("%s ", config.GetConfig().Default.Delimiter))
						}
					}
				}
				if techSP2 {
					var key string
					if strings.Contains(strings.ToLower(data.Technician), "spl") {
						key = fmt.Sprintf("SPL_%d_%d_%s", splSP.ID, 2, "SP_SPL")
					} else {
						key = fmt.Sprintf("TECH_%d_%d_%s", technicianSP.ID, 2, "SP_TECHNICIAN")
					}
					if msgs, exists := waMessageMap[key]; exists && len(msgs) > 0 {
						var messages []string
						for _, msg := range msgs {
							if msg.WhatsappReplyText != "" {
								senderPhone := extractPhoneFromJID(msg.WhatsappChatJID)
								senderName := senderPhone
								if user, ok := waUserMap[senderPhone]; ok {
									senderName = fmt.Sprintf("(%s %s)", user.FullName, user.PhoneNumber)
								}
								messages = append(messages,
									fmt.Sprintf("@%v From: %s ~%s",
										msg.CreatedAt.Format("Monday, 02 Jan 2006 15:04:05 MST"),
										senderName,
										msg.WhatsappReplyText,
									),
								)
							}
						}
						if len(messages) > 0 {
							techSP2Replied = strings.Join(messages, fmt.Sprintf("%s ", config.GetConfig().Default.Delimiter))
						}
					}
				}
				if techSP3 {
					var key string
					if strings.Contains(strings.ToLower(data.Technician), "spl") {
						key = fmt.Sprintf("SPL_%d_%d_%s", splSP.ID, 3, "SP_SPL")
					} else {
						key = fmt.Sprintf("TECH_%d_%d_%s", technicianSP.ID, 3, "SP_TECHNICIAN")
					}
					if msgs, exists := waMessageMap[key]; exists && len(msgs) > 0 {
						var messages []string
						for _, msg := range msgs {
							if msg.WhatsappReplyText != "" {
								senderPhone := extractPhoneFromJID(msg.WhatsappChatJID)
								senderName := senderPhone
								if user, ok := waUserMap[senderPhone]; ok {
									senderName = fmt.Sprintf("(%s %s)", user.FullName, user.PhoneNumber)
								}
								messages = append(messages,
									fmt.Sprintf("@%v From: %s ~%s",
										msg.CreatedAt.Format("Monday, 02 Jan 2006 15:04:05 MST"),
										senderName,
										msg.WhatsappReplyText,
									),
								)
							}
						}
						if len(messages) > 0 {
							techSP3Replied = strings.Join(messages, fmt.Sprintf("%s ", config.GetConfig().Default.Delimiter))
						}
					}
				}

				switch column.ColTitle {
				case "Data Created on":
					if !data.CreatedAt.IsZero() {
						value = data.CreatedAt.Format("2006-01-02 15:04:05")
					}
				case "Data Created on (Day)":
					if !data.CreatedAt.IsZero() {
						value = data.CreatedAt.Format("02 Jan")
					}
				case "Technician":
					value = data.Technician
				case "Technician Group":
					value = data.TechnicianGroup
				case "City":
					value = city
				case "Province":
					value = province
				case "Full Name":
					value = data.Name
				case "SPL":
					value = data.SPL
				case "SAC":
					value = data.SAC
				case "Last Login":
					if data.LastLogin != nil && !data.LastLogin.IsZero() {
						value = data.LastLogin.Format("2006-01-02 15:04:05")
					}
				case "Last Download JO":
					if data.LastDownloadJO != nil && !data.LastDownloadJO.IsZero() {
						value = data.LastDownloadJO.Format("2006-01-02 15:04:05")
					}
				case "First Uploaded JO":
					if data.FirstUpload != nil && !data.FirstUpload.IsZero() {
						value = data.FirstUpload.Format("2006-01-02 15:04:05")
					}
				case "Latest Visit":
					if data.LastVisit != nil && !data.LastVisit.IsZero() {
						value = data.LastVisit.Format("2006-01-02 15:04:05")
					}
				case "Is Got SP 1":
					value = techSP1
				case "Is Got SP 2":
					value = techSP2
				case "Is Got SP 3":
					value = techSP3
				case "WO Numbers (Planned)":
					if len(data.WONumber) > 0 {
						var woNumbers []string
						if err := json.Unmarshal([]byte(data.WONumber), &woNumbers); err == nil {
							value = strings.Join(woNumbers, fmt.Sprintf("%s ", config.GetConfig().Default.Delimiter))
						}

						f.AddComment(sheetMaster, excelize.Comment{
							Cell:   cell,
							Author: "Report Service",
							Paragraph: []excelize.RichTextRun{
								{
									Text: fmt.Sprintf("You can separate the WO Numbers using formula =TEXTSPLIT(%s, \"%s\") ", cell, config.GetConfig().Default.Delimiter),
								},
							},
							Width:  300,
							Height: 50,
						})
					}
				case "WO Numbers (Visited)":
					if len(data.WONumberVisited) > 0 {
						var woNumbers []string
						if err := json.Unmarshal([]byte(data.WONumberVisited), &woNumbers); err == nil {
							value = strings.Join(woNumbers, fmt.Sprintf("%s ", config.GetConfig().Default.Delimiter))
						}

						f.AddComment(sheetMaster, excelize.Comment{
							Cell:   cell,
							Author: "Report Service",
							Paragraph: []excelize.RichTextRun{
								{
									Text: fmt.Sprintf("You can separate the WO Numbers visited using formula =TEXTSPLIT(%s, \"%s\") ", cell, config.GetConfig().Default.Delimiter),
								},
							},
							Width:  300,
							Height: 50,
						})
					}
				case "Link Photos (WO Planned)":
					if len(data.WOLinkPhotos) > 0 {
						var photoLinks []string
						if err := json.Unmarshal([]byte(data.WOLinkPhotos), &photoLinks); err == nil {
							value = strings.Join(photoLinks, fmt.Sprintf("%s ", config.GetConfig().Default.Delimiter))
						}

						f.AddComment(sheetMaster, excelize.Comment{
							Cell:   cell,
							Author: "Report Service",
							Paragraph: []excelize.RichTextRun{
								{
									Text: fmt.Sprintf("You can separate the photo links of wo planned using formula =TEXTSPLIT(%s, \"%s\") ", cell, config.GetConfig().Default.Delimiter),
								},
							},
							Width:  300,
							Height: 50,
						})
					}
				case "Link Photos (WO Visited)":
					if len(data.WOVisitedLinkPhotos) > 0 {
						var photoLinks []string
						if err := json.Unmarshal([]byte(data.WOVisitedLinkPhotos), &photoLinks); err == nil {
							value = strings.Join(photoLinks, fmt.Sprintf("%s ", config.GetConfig().Default.Delimiter))
						}

						f.AddComment(sheetMaster, excelize.Comment{
							Cell:   cell,
							Author: "Report Service",
							Paragraph: []excelize.RichTextRun{
								{
									Text: fmt.Sprintf("You can separate the photo links of wo visited using formula =TEXTSPLIT(%s, \"%s\") ", cell, config.GetConfig().Default.Delimiter),
								},
							},
							Width:  300,
							Height: 50,
						})
					}

				case "SP 1 Replied":
					value = techSP1Replied
				case "SP 2 Replied":
					value = techSP2Replied
				case "SP 3 Replied":
					value = techSP3Replied
				}

				if needToSetValue {
					// Only convert boolean values to Yes/No, leave other values as-is
					if value != nil && reflect.TypeOf(value).Kind() == reflect.Bool {
						boolVal := value.(bool)
						if boolVal {
							value = "Yes"
						} else {
							value = "No"
						}
					}

					if value != nil && value != "" {
						f.SetCellValue(sheetMaster, cell, value)
						f.SetCellStyle(sheetMaster, cell, cell, style)
					}
				}
			}
			rowIndex++
			totalProcessed++
		}

		// Log batch progress
		batchDuration := time.Since(batchStartTime)
		logrus.Debugf("Processed %s batch starting at offset %d, fetched %d records in %v", taskDoing, offset, len(dataBatch), batchDuration)

		// Log overall progress for large datasets
		if totalProcessed%5000 == 0 {
			elapsed := time.Since(startTime)
			logrus.Infof("Progress: processed %d records in %v (avg: %.2f records/sec)",
				totalProcessed, elapsed, float64(totalProcessed)/elapsed.Seconds())
		}

		offset += batchSize
	}

	// Pivot
	f.SetCellValue(sheetPivot, "A1", fmt.Sprintf("Count of Visit Technicians (%s)", strings.ToUpper(now.Format("January 2006"))))
	stylePivotTitle, err := f.NewStyle(&excelize.Style{
		Font: &excelize.Font{
			Bold:      true,
			Underline: "single",
			Size:      13,
		},
	})
	if err != nil {
		return "", fmt.Errorf("failed to create style for pivot title: %v", err)
	}
	f.SetCellStyle(sheetPivot, "A1", "A1", stylePivotTitle)
	f.SetColWidth(sheetPivot, "A", "A", 50)

	lastRowMasterData := rowIndex - 1
	pivotMasterDataRange := fmt.Sprintf("%s!$A$1:%s$%d", sheetMaster, lastColMaster, lastRowMasterData)
	pivotSheetRange := fmt.Sprintf("%s!A7:AA3000", sheetPivot)
	err = f.AddPivotTable(&excelize.PivotTableOptions{
		Name:            sheetPivot,
		DataRange:       pivotMasterDataRange,
		PivotTableRange: pivotSheetRange,
		Rows: []excelize.PivotTableField{
			{Data: "Technician"},
		},
		Columns: []excelize.PivotTableField{
			{Data: "Data Created on (Day)", Compact: true},
		},
		Data: []excelize.PivotTableField{
			{Data: "Latest Visit", Subtotal: "Count"},
		},
		Filter: []excelize.PivotTableField{
			{Data: "SAC"},
			{Data: "SPL"},
			{Data: "Technician Group"},
		},
		RowGrandTotals:      true,
		ColGrandTotals:      true,
		ShowDrill:           true,
		ShowRowHeaders:      true,
		ShowColHeaders:      true,
		ShowLastColumn:      true,
		PivotTableStyleName: "PivotStyleMedium9",
	})

	if err != nil {
		return "", fmt.Errorf("failed to create pivot table: %v", err)
	}

	// SP Pivot
	f.SetCellValue(sheetSPPivot, "A1", fmt.Sprintf("Count of SP Given (%s)", strings.ToUpper(now.Format("January 2006"))))
	styleSPPivotTitle, err := f.NewStyle(&excelize.Style{
		Font: &excelize.Font{
			Bold:      true,
			Underline: "single",
			Size:      13,
		},
	})
	if err != nil {
		return "", fmt.Errorf("failed to create style for SP pivot title: %v", err)
	}
	f.SetCellStyle(sheetSPPivot, "A1", "A1", styleSPPivotTitle)
	f.SetColWidth(sheetSPPivot, "A", "A", 50)

	pivotSPSheetRange := fmt.Sprintf("%s!A7:AA3000", sheetSPPivot)
	err = f.AddPivotTable(&excelize.PivotTableOptions{
		Name:            sheetSPPivot,
		DataRange:       pivotMasterDataRange,
		PivotTableRange: pivotSPSheetRange,
		Rows: []excelize.PivotTableField{
			{Data: "Technician"},
		},
		Columns: []excelize.PivotTableField{
			{Data: "Data Created on (Day)", Compact: true},
		},
		Data: []excelize.PivotTableField{
			{Data: "SP 1 Replied", Subtotal: "Count"},
			{Data: "SP 2 Replied", Subtotal: "Count"},
			{Data: "SP 3 Replied", Subtotal: "Count"},
		},
		Filter: []excelize.PivotTableField{
			{Data: "SAC"},
			{Data: "SPL"},
			{Data: "Technician Group"},
		},
		RowGrandTotals:      true,
		ColGrandTotals:      true,
		ShowDrill:           true,
		ShowRowHeaders:      true,
		ShowColHeaders:      true,
		ShowLastColumn:      true,
		PivotTableStyleName: "PivotStyleMedium10",
	})
	if err != nil {
		return "", fmt.Errorf("failed to create SP pivot table: %v", err)
	}

	// Active
	titleActive := []struct {
		Title    string
		ColWidth float64
	}{
		{"Technician", 50},
	}
	for day := 1; day <= numDays; day++ {
		titleActive = append(titleActive, struct {
			Title    string
			ColWidth float64
		}{
			Title:    fmt.Sprintf("%d %v", day, now.Format("Jan 2006")),
			ColWidth: 28,
		})
	}
	titleActive = append(titleActive, struct {
		Title    string
		ColWidth float64
	}{
		Title:    "Grand Total",
		ColWidth: 25,
	})

	var columnsActive []ExcelColumn
	for i, t := range titleActive {
		columnsActive = append(columnsActive, ExcelColumn{
			ColIndex: fun.GetColName(i),
			ColTitle: t.Title,
			ColSize:  t.ColWidth,
		})
	}

	for _, col := range columnsActive {
		cell := fmt.Sprintf("%s1", col.ColIndex)
		f.SetColWidth(sheetActive, col.ColIndex, col.ColIndex, col.ColSize)
		f.SetCellValue(sheetActive, cell, col.ColTitle)
		f.SetCellStyle(sheetActive, cell, cell, styleTitleActive)
	}
	lastColActive := fun.GetColName(len(columnsActive) - 1)
	filterRangeActive := fmt.Sprintf("A1:%s1", lastColActive)
	f.AutoFilter(sheetActive, filterRangeActive, []excelize.AutoFilterOptions{})

	// Freeze column A (first column) so it stays visible when scrolling horizontally
	f.SetPanes(sheetActive, &excelize.Panes{
		Freeze:      true,
		Split:       false,
		XSplit:      1,    // Freeze after column A (1 column from left)
		YSplit:      1,    // Freeze after row 1 (header row)
		TopLeftCell: "B2", // First visible cell in the unfrozen area
		ActivePane:  "bottomRight",
	})

	var distinctTechnicians []string
	if err := dbWeb.Model(&odooms.ODOOMSTechnicianData{}).
		Where("created_at >= ? AND created_at <= ?", monthStart, monthEnd).
		Distinct("technician").
		Order("technician asc").
		Pluck("technician", &distinctTechnicians).Error; err != nil {
		return "", fmt.Errorf("failed to fetch distinct technicians: %v", err)
	}
	rowIndex = 2
	for _, technician := range distinctTechnicians {
		cell := fmt.Sprintf("A%d", rowIndex)
		f.SetCellValue(sheetActive, cell, technician)

		// For each technician, check their activity for each day
		for day := 1; day <= numDays; day++ {
			currentDate := time.Date(now.Year(), now.Month(), day, 0, 0, 0, 0, loc)
			var isActive int64
			if err := dbWeb.Model(&odooms.ODOOMSTechnicianData{}).
				Where("technician = ?", technician).
				Where("DATE(last_login) = ?", currentDate).
				Where("DATE(last_visit) = ?", currentDate).
				Where("created_at >= ? AND created_at <= ?", monthStart, monthEnd).
				Count(&isActive).Error; err != nil {
				return "", fmt.Errorf("failed to check activity for technician %s on %s: %v", technician, currentDate.Format("2006-01-02"), err)
			}

			cell := fmt.Sprintf("%s%d", fun.GetColName(day), rowIndex)
			if isActive > 0 {
				f.SetCellValue(sheetActive, cell, 1) // Active = 1
			} else {
				f.SetCellValue(sheetActive, cell, 0) // Not active = 0
			}
			f.SetCellStyle(sheetActive, cell, cell, style)
		}
		rowIndex++
	}

	// Set formula for Grand Total column
	for r := 2; r < rowIndex; r++ {
		startCol := fun.GetColName(1) // Column B
		endCol := fun.GetColName(numDays)
		cell := fmt.Sprintf("%s%d", fun.GetColName(numDays+1), r) // Grand Total column
		formula := fmt.Sprintf("SUM(%s%d:%s%d)", startCol, r, endCol, r)
		if err := f.SetCellFormula(sheetActive, cell, formula); err != nil {
			return "", fmt.Errorf("failed to set formula for grand total at %s: %v", cell, err)
		}
		f.SetCellStyle(sheetActive, cell, cell, styleBold)
	}

	f.SetCellValue(sheetActive, fmt.Sprintf("A%d", rowIndex), "Count of Active Technicians")
	f.SetCellStyle(sheetActive, fmt.Sprintf("A%d", rowIndex), fmt.Sprintf("A%d", rowIndex), styleCountActiveTechnicians)
	for col := 1; col <= numDays+1; col++ {
		cell := fmt.Sprintf("%s%d", fun.GetColName(col), rowIndex)
		columnLetter := fun.GetColName(col)
		formula := fmt.Sprintf("SUM(%s2:%s%d)", columnLetter, columnLetter, rowIndex-1)
		if err := f.SetCellFormula(sheetActive, cell, formula); err != nil {
			return "", fmt.Errorf("failed to set formula for count of active technicians at %s: %v", cell, err)
		}
		f.SetCellStyle(sheetActive, cell, cell, styleCountActiveTechnicians)
	}

	rowIndex++

	f.SetCellValue(sheetActive, fmt.Sprintf("A%d", rowIndex), "Count of Registered Technicians in ODOO")
	f.SetCellStyle(sheetActive, fmt.Sprintf("A%d", rowIndex), fmt.Sprintf("A%d", rowIndex), styleCountRegisteredTechniciansInODOOMS)
	for col := 1; col <= numDays+1; col++ {
		cell := fmt.Sprintf("%s%d", fun.GetColName(col), rowIndex)
		columnLetter := fun.GetColName(col)
		formula := fmt.Sprintf("COUNTA(%s2:%s%d)", columnLetter, columnLetter, rowIndex-2)
		if err := f.SetCellFormula(sheetActive, cell, formula); err != nil {
			return "", fmt.Errorf("failed to set formula for count of registered technicians at %s: %v", cell, err)
		}
		f.SetCellStyle(sheetActive, cell, cell, styleCountRegisteredTechniciansInODOOMS)
	}

	// RESUME
	f.SetCellValue(sheetResume, "A1", fmt.Sprintf("Technician Attendance Summary %s", strings.ToUpper(now.Format("January 2006"))))
	styleResumeTitle, err := f.NewStyle(&excelize.Style{
		Font: &excelize.Font{
			Bold:      true,
			Underline: "single",
			Size:      13,
		},
	})
	if err != nil {
		return "", fmt.Errorf("failed to create style for resume title: %v", err)
	}
	f.SetCellStyle(sheetResume, "A1", "A1", styleResumeTitle)

	titleResume := []struct {
		Title    string
		ColWidth float64
	}{
		{"No", 8},
		{"Activity", 40},
	}
	for day := 1; day <= numDays; day++ {
		titleResume = append(titleResume, struct {
			Title    string
			ColWidth float64
		}{
			Title:    fmt.Sprintf("%d", day),
			ColWidth: 15,
		})
	}
	titleResume = append(titleResume, struct {
		Title    string
		ColWidth float64
	}{
		Title:    "Grand Total",
		ColWidth: 25,
	})

	var columnsResume []ExcelColumn
	for i, t := range titleResume {
		columnsResume = append(columnsResume, ExcelColumn{
			ColIndex: fun.GetColName(i),
			ColTitle: t.Title,
			ColSize:  t.ColWidth,
		})
	}

	for _, col := range columnsResume {
		cell := fmt.Sprintf("%s7", col.ColIndex)
		f.SetColWidth(sheetResume, col.ColIndex, col.ColIndex, col.ColSize)
		f.SetCellValue(sheetResume, cell, col.ColTitle)

		switch col.ColTitle {
		case "No":
			f.SetCellStyle(sheetResume, cell, cell, styleResumeNo)
		case "Activity":
			f.SetCellStyle(sheetResume, cell, cell, styleResumeActivity)
		case "Grand Total":
			f.SetCellStyle(sheetResume, cell, cell, styleResumeGrandTotal)
		default:
			f.SetCellStyle(sheetResume, cell, cell, styleResumeDay)
		}
	}
	lastColResume := fun.GetColName(len(columnsResume) - 1)
	filterRangeResume := fmt.Sprintf("A7:%s7", lastColResume)
	f.AutoFilter(sheetResume, filterRangeResume, []excelize.AutoFilterOptions{})

	rowIndex = 8
	no := 1
	activities := []string{
		"Count of Active Technicians",
		"Count of Login Technicians",
		"Count of Visit Technicians",
		"Count of SP 1 Given",
		"Count of SP 2 Given",
		"Count of SP 3 Given",
	}
	for _, activity := range activities {
		f.SetCellValue(sheetResume, fmt.Sprintf("A%d", rowIndex), no)
		f.SetCellStyle(sheetResume, fmt.Sprintf("A%d", rowIndex), fmt.Sprintf("A%d", rowIndex), style)

		f.SetCellValue(sheetResume, fmt.Sprintf("B%d", rowIndex), activity)

		no++
		rowIndex++
	}

	// Filters
	f.SetCellValue(sheetResume, "B3", "SAC:")
	f.SetCellValue(sheetResume, "C3", "All")
	colsMaster, _ := f.GetCols(sheetMaster)
	var sacs []string
	if len(colsMaster) > 8 { // Col I is index 8
		uniq := map[string]bool{}
		sacs = append(sacs, "All")
		for _, v := range colsMaster[8][1:] { // Skip header
			if v != "" && !uniq[v] {
				uniq[v] = true
				sacs = append(sacs, v)
			}
		}
	}
	f.SetCellValue(sheetResume, "B4", "SPL:")
	f.SetCellValue(sheetResume, "C4", "All")
	var spls []string
	if len(colsMaster) > 7 { // Col H is index 7
		uniq := map[string]bool{}
		spls = append(spls, "All")
		for _, v := range colsMaster[7][1:] { // Skip header
			if v != "" && !uniq[v] {
				uniq[v] = true
				spls = append(spls, v)
			}
		}
	}

	f.SetCellValue(sheetResume, "B5", "Technician:")
	f.SetCellValue(sheetResume, "C5", "All")

	helperStartIndex := len(columnsResume) + 3
	colSAC := fun.GetColName(helperStartIndex)
	colSPL := fun.GetColName(helperStartIndex + 1)
	colTechnician := fun.GetColName(helperStartIndex + 2)
	for i, v := range sacs {
		cell := fmt.Sprintf("%s%d", colSAC, i+1)
		f.SetCellValue(sheetResume, cell, v)
	}
	for i, v := range spls {
		cell := fmt.Sprintf("%s%d", colSPL, i+1)
		f.SetCellValue(sheetResume, cell, v)
	}
	distinctTechnicians = append([]string{"All"}, distinctTechnicians...)
	for i, v := range distinctTechnicians {
		cell := fmt.Sprintf("%s%d", colTechnician, i+1)
		f.SetCellValue(sheetResume, cell, v)
	}

	dv := excelize.NewDataValidation(true)
	dv.Sqref = "C3"
	dv.SetSqrefDropList(fmt.Sprintf("%s!$%s$1:$%s$%d", sheetResume, colSAC, colSAC, len(sacs)))
	f.AddDataValidation(sheetResume, dv)

	dv = excelize.NewDataValidation(true)
	dv.Sqref = "C4"
	dv.SetSqrefDropList(fmt.Sprintf("%s!$%s$1:$%s$%d", sheetResume, colSPL, colSPL, len(spls)))
	f.AddDataValidation(sheetResume, dv)

	dv = excelize.NewDataValidation(true)
	dv.Sqref = "C5"
	dv.SetSqrefDropList(fmt.Sprintf("%s!$%s$1:$%s$%d", sheetResume, colTechnician, colTechnician, len(distinctTechnicians)))
	f.AddDataValidation(sheetResume, dv)

	// Hide helper columns
	f.SetColVisible(sheetResume, colSAC, false)
	f.SetColVisible(sheetResume, colSPL, false)
	f.SetColVisible(sheetResume, colTechnician, false)

	rowIndex = 8
	for i := range activities {
		colN := 2 // Starting from column C
		for day := 1; day <= numDays; day++ {
			colIndex := fun.GetColName(colN)
			dayDate := time.Date(now.Year(), now.Month(), day, 0, 0, 0, 0, loc)
			dayStr := dayDate.Format("02 Jan")
			dayStrFull := dayDate.Format("2006-01-02")

			masterStartRow := 2
			masterEndRow := lastRowMasterData

			// Use absolute references to MASTER sheet
			colDay := fmt.Sprintf("%s!$%s$%d:$%s$%d", sheetMaster, colMasterDataCreatedOnDayIndex, masterStartRow, colMasterDataCreatedOnDayIndex, masterEndRow)
			colSAC := fmt.Sprintf("%s!$%s$%d:$%s$%d", sheetMaster, colMasterSACIndex, masterStartRow, colMasterSACIndex, masterEndRow)
			colSPL := fmt.Sprintf("%s!$%s$%d:$%s$%d", sheetMaster, colMasterSPLIndex, masterStartRow, colMasterSPLIndex, masterEndRow)
			colTechnician := fmt.Sprintf("%s!$%s$%d:$%s$%d", sheetMaster, colMasterTechnicianIndex, masterStartRow, colMasterTechnicianIndex, masterEndRow)
			colSP1 := fmt.Sprintf("%s!$%s$%d:$%s$%d", sheetMaster, colMasterSP1GivenIndex, masterStartRow, colMasterSP1GivenIndex, masterEndRow)
			colSP2 := fmt.Sprintf("%s!$%s$%d:$%s$%d", sheetMaster, colMasterSP2GivenIndex, masterStartRow, colMasterSP2GivenIndex, masterEndRow)
			colSP3 := fmt.Sprintf("%s!$%s$%d:$%s$%d", sheetMaster, colMasterSP3GivenIndex, masterStartRow, colMasterSP3GivenIndex, masterEndRow)
			colLatestVisit := fmt.Sprintf("%s!$%s$%d:$%s$%d", sheetMaster, colMasterLatestVisitIndex, masterStartRow, colMasterLatestVisitIndex, masterEndRow)
			colLastLogin := fmt.Sprintf("%s!$%s$%d:$%s$%d", sheetMaster, colMasterLastLoginIndex, masterStartRow, colMasterLastLoginIndex, masterEndRow)

			var formula string
			switch i {
			case 0: // Count of Active Technicians
				formula = fmt.Sprintf(
					"=SUMPRODUCT((%s=\"%s\")*(($C$3=\"All\")+(%s=$C$3))*(($C$4=\"All\")+(%s=$C$4))*(($C$5=\"All\")+(%s=$C$5)))",
					colDay, dayStr,
					colSAC, colSPL, colTechnician,
				)
			case 2:
				// Count of Visit Technician
				formula = fmt.Sprintf(
					"=SUMPRODUCT((%s=\"%s\")*(ISNUMBER(SEARCH(\"%s\",%s)))*(($C$3=\"All\")+(%s=$C$3))*(($C$4=\"All\")+(%s=$C$4))*(($C$5=\"All\")+(%s=$C$5)))",
					colDay, dayStr,
					dayStrFull, colLatestVisit,
					colSAC, colSPL, colTechnician,
				)
			case 1:
				// Count of Login Technician
				formula = fmt.Sprintf(
					"=SUMPRODUCT((%s=\"%s\")*(ISNUMBER(SEARCH(\"%s\",%s)))*(($C$3=\"All\")+(%s=$C$3))*(($C$4=\"All\")+(%s=$C$4))*(($C$5=\"All\")+(%s=$C$5)))",
					colDay, dayStr,
					dayStrFull, colLastLogin,
					colSAC, colSPL, colTechnician,
				)
			case 3: // Count of SP 1 Given
				formula = fmt.Sprintf(
					"=SUMPRODUCT((%s=\"%s\")*(%s=\"Yes\")*(($C$3=\"All\")+(%s=$C$3))*(($C$4=\"All\")+(%s=$C$4))*(($C$5=\"All\")+(%s=$C$5)))",
					colDay, dayStr,
					colSP1,
					colSAC, colSPL, colTechnician,
				)
			case 4: // Count of SP 2 Given
				formula = fmt.Sprintf(
					"=SUMPRODUCT((%s=\"%s\")*(%s=\"Yes\")*(($C$3=\"All\")+(%s=$C$3))*(($C$4=\"All\")+(%s=$C$4))*(($C$5=\"All\")+(%s=$C$5)))",
					colDay, dayStr,
					colSP2,
					colSAC, colSPL, colTechnician,
				)
			case 5: // Count of SP 3 Given
				formula = fmt.Sprintf(
					"=SUMPRODUCT((%s=\"%s\")*(%s=\"Yes\")*(($C$3=\"All\")+(%s=$C$3))*(($C$4=\"All\")+(%s=$C$4))*(($C$5=\"All\")+(%s=$C$5)))",
					colDay, dayStr,
					colSP3,
					colSAC, colSPL, colTechnician,
				)
			}
			f.SetCellFormula(sheetResume, fmt.Sprintf("%s%d", colIndex, rowIndex), formula)
			f.SetCellStyle(sheetResume, fmt.Sprintf("%s%d", colIndex, rowIndex), fmt.Sprintf("%s%d", colIndex, rowIndex), style)
			colN++
		}
		rowIndex++
	}
	// Create the Grand Total formula for each activity
	rowIndex = 8
	for range activities {
		colIndex := fun.GetColName(len(titleResume) - 1) // Grand Total column
		startCol := fun.GetColName(2)                    // Starting from column C
		endCol := fun.GetColName(numDays + 1)            // Up to the last day column
		formula := fmt.Sprintf("SUM(%s%d:%s%d)", startCol, rowIndex, endCol, rowIndex)
		f.SetCellFormula(sheetResume, fmt.Sprintf("%s%d", colIndex, rowIndex), formula)
		f.SetCellStyle(sheetResume, fmt.Sprintf("%s%d", colIndex, rowIndex), fmt.Sprintf("%s%d", colIndex, rowIndex), styleResumeGrandTotal)
		rowIndex++
	}

	// Freeze panes to keep headers visible
	if err := f.SetPanes(sheetResume, &excelize.Panes{
		Freeze:      true,
		Split:       false,
		XSplit:      2,    // Freeze after column B (2 columns from left)
		YSplit:      7,    // Freeze after row 7 (header row)
		TopLeftCell: "C8", // First visible cell in the unfrozen area
		ActivePane:  "bottomRight",
	}); err != nil {
		return "", fmt.Errorf("failed to set panes for resume sheet: %v", err)
	}

	// Line Chart
	lineItemSeriesMap := map[string]MonitoringLoginVisitTechnicianChartResumeSeries{
		"Active Technicians": {
			ColorLine:     "#90EE90", // light green
			ColorMarker:   "#90EE90",
			MarkerSymbol:  "triangle",
			MarkerSize:    6,
			LineThickness: 2.0,
		},
		"Login Technicians": {
			ColorLine:     "#E615B5", // magenta
			ColorMarker:   "#E615B5",
			MarkerSymbol:  "diamond",
			MarkerSize:    8,
			LineThickness: 3.0,
		},
		"Visit Technicians": {
			ColorLine:     "#09F0D9", // teal/cyan
			ColorMarker:   "#09F0D9",
			MarkerSymbol:  "square",
			MarkerSize:    10,
			LineThickness: 5.0,
		},
		"SP 1 Given": {
			ColorLine:     "#FFFF00", // yellow
			ColorMarker:   "#FFFF00",
			MarkerSymbol:  "circle",
			MarkerSize:    5,
			LineThickness: 1.0,
		},
		"SP 2 Given": {
			ColorLine:     "#9467BD", // purple
			ColorMarker:   "#9467BD",
			MarkerSymbol:  "circle",
			MarkerSize:    5,
			LineThickness: 1.0,
		},
		"SP 3 Given": {
			ColorLine:     "#FF0000", // red
			ColorMarker:   "#FF0000",
			MarkerSymbol:  "circle",
			MarkerSize:    5,
			LineThickness: 1.0,
		},
	}

	// Add notes with marker symbol and color for each activity
	notesRow := rowIndex + 3
	notesRichText := []excelize.RichTextRun{
		{Text: "Notes:\n", Font: &excelize.Font{Bold: true, Size: 12}},
	}
	// Map of activity to display name and description
	activityNotes := []struct {
		Name        string
		DisplayName string
		Desc        string
	}{
		{"Active Technicians", "Active Technicians", "= Count of technicians registered on the same day."},
		{"Login Technicians", "Login Technicians", "= Count of technicians who logged in at least once on that day."},
		{"Visit Technicians", "Visit Technicians", "= Count of technicians who had at least one visit on that day."},
		{"SP 1 Given", "SP 1 Given", "= Count of technicians who were given SP 1 on that day."},
		{"SP 2 Given", "SP 2 Given", "= Count of technicians who were given SP 2 on that day."},
		{"SP 3 Given", "SP 3 Given", "= Count of technicians who were given SP 3 on that day."},
	}
	for _, note := range activityNotes {
		series := lineItemSeriesMap[note.Name]
		markerText := fmt.Sprintf("%s ", fun.MarkerSymbolUnicode(series.MarkerSymbol))
		notesRichText = append(notesRichText,
			excelize.RichTextRun{Text: markerText, Font: &excelize.Font{Color: series.ColorMarker, Size: 12, Family: "Arial Unicode MS"}},
			excelize.RichTextRun{Text: note.DisplayName, Font: &excelize.Font{Bold: true, Size: 10}},
			excelize.RichTextRun{Text: " " + note.Desc + "\n", Font: &excelize.Font{Size: 10}},
		)
	}
	f.SetCellRichText(sheetResume, fmt.Sprintf("C%d", notesRow), notesRichText)
	styleNotes, err := f.NewStyle(&excelize.Style{
		Alignment: &excelize.Alignment{
			WrapText: true,
			Vertical: "top",
		},
	})
	if err != nil {
		return "", fmt.Errorf("failed to create style for notes: %v", err)
	}
	f.SetCellStyle(sheetResume, fmt.Sprintf("C%d", notesRow), fmt.Sprintf("C%d", notesRow), styleNotes)
	f.SetRowHeight(sheetResume, notesRow, 110)
	f.MergeCell(sheetResume, fmt.Sprintf("C%d", notesRow), fmt.Sprintf("%s%d", lastColResume, notesRow))

	chartStartRow := notesRow + 1
	startColCategory := "C"                                // Starting from column C day 1
	lastColResume = fun.GetColName(len(columnsResume) - 2) // Exclude grand total column
	categoriesRange := fmt.Sprintf("%s!%s%d:%s%d", sheetResume, startColCategory, 7, lastColResume, 7)

	// Collect series
	var activitySeries []excelize.ChartSeries
	for i, activity := range activities {
		nameCell := strings.ReplaceAll(activity, "Count of ", "")
		dataRow := 8 + i
		valueRange := fmt.Sprintf("%s!%s%d:%s%d", sheetResume, startColCategory, dataRow, lastColResume, dataRow)
		nameRange := fmt.Sprintf("%s!$B$%d", sheetResume, dataRow)

		dataActivitySeries := lineItemSeriesMap[nameCell]
		var activityChartSeries excelize.ChartSeries
		activityChartSeries.Name = nameRange
		activityChartSeries.Categories = categoriesRange
		activityChartSeries.Values = valueRange
		activityChartSeries.Marker = excelize.ChartMarker{
			Symbol: dataActivitySeries.MarkerSymbol,
			Size:   dataActivitySeries.MarkerSize,
			Fill: excelize.Fill{
				Type:    "pattern",
				Pattern: 1,
				Color:   []string{dataActivitySeries.ColorMarker},
			},
		}
		activityChartSeries.Line = excelize.ChartLine{
			Width:  dataActivitySeries.LineThickness,
			Smooth: false,
		}
		activityChartSeries.Fill = excelize.Fill{
			Type:    "pattern",
			Pattern: 1,
			Color:   []string{dataActivitySeries.ColorLine},
		}

		activitySeries = append(activitySeries, activityChartSeries)
	}

	chartCell := fmt.Sprintf("C%d", chartStartRow)
	err = f.AddChart(sheetResume, chartCell, &excelize.Chart{
		Type:   excelize.Line,
		Series: activitySeries,
		Format: excelize.GraphicOptions{
			OffsetX: 15,
			OffsetY: 10,
			ScaleX:  1.0,
			ScaleY:  1.0,
		},
		Title: []excelize.RichTextRun{
			{Text: "Technician Attendance " + strings.ToUpper(now.Format("January 2006")), Font: &excelize.Font{Bold: true, Size: 15}},
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
		return "", fmt.Errorf("failed to add line chart: %v", err)
	}

	// Copy chart to a new sheet 'CHART'
	err = f.AddChart(sheetChart, "A1", &excelize.Chart{
		Type:   excelize.Line,
		Series: activitySeries,
		Format: excelize.GraphicOptions{
			OffsetX: 15,
			OffsetY: 10,
			ScaleX:  1.0,
			ScaleY:  0.3,
		},
		Title: []excelize.RichTextRun{
			{Text: "Technician Attendance " + strings.ToUpper(now.Format("January 2006")), Font: &excelize.Font{Bold: true, Size: 15}},
		},
		PlotArea: excelize.ChartPlotArea{
			ShowDataTable:     true,
			ShowDataTableKeys: true,
			ShowSerName:       false,
		},
		ShowBlanksAs: "zero",
		Dimension: excelize.ChartDimension{
			Width:  config.GetConfig().Report.MonitoringLoginVisitTechnician.ChartWidth,
			Height: config.GetConfig().Report.MonitoringLoginVisitTechnician.ChartHeight,
		},
		Legend: excelize.ChartLegend{
			Position:      "top",
			ShowLegendKey: false,
		},
	})
	if err != nil {
		return "", fmt.Errorf("failed to add line chart: %v", err)
	}

	f.DeleteSheet("Sheet1")
	// f.SetActiveSheet(indexSheetResume - 1)
	f.SetActiveSheet(1) // Master sheet
	// Hide RESUME chart
	f.SetSheetVisible(sheetResume, false)
	f.SetSheetVisible(sheetChart, false)

	// Log final processing summary
	totalElapsed := time.Since(startTime)
	logrus.Infof("Completed processing %d records in %v (avg: %.2f records/sec)",
		totalProcessed, totalElapsed, float64(totalProcessed)/totalElapsed.Seconds())

	if err := f.SaveAs(excelFilePath); err != nil {
		return "", fmt.Errorf("failed to save initial Excel file: %v", err)
	}

	return excelFilePath, nil
}

func GetDataOfTechnicianODOOMSForMonitoring() error {
	taskDoing := "Get Data of Technician ODOO MS"
	getDataOfTechnicianODOOMSMutex.Lock()
	defer getDataOfTechnicianODOOMSMutex.Unlock()

	GetDataTechnicianODOOMS()

	if len(TechODOOMSData) == 0 {
		return fmt.Errorf("%s: no technician data found from ODOO MS", taskDoing)
	}

	loc, _ := time.LoadLocation(config.GetConfig().Default.Timezone)
	now := time.Now().In(loc)

	startOfToday := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, loc)
	endOfToday := startOfToday.Add(24*time.Hour - 1*time.Nanosecond)

	dbWeb := gormdb.Databases.Web

	// Delete today's data before inserting new data
	if err := dbWeb.Where("created_at BETWEEN ? AND ?", startOfToday, endOfToday).Delete(&odooms.ODOOMSTechnicianData{}).Error; err != nil {
		return fmt.Errorf("failed to delete today's technician data: %v", err)
	}

	const batchSize = 1000

	var techniciansToCreate []odooms.ODOOMSTechnicianData
	for techName, techData := range TechODOOMSData {
		technicianGroup, _ := techGroup(techName)

		techniciansToCreate = append(techniciansToCreate, odooms.ODOOMSTechnicianData{
			Technician:      techName,
			Name:            techData.Name,
			TechnicianGroup: technicianGroup,
			SPL:             techData.SPL,
			SAC:             techData.SAC,
			Email:           techData.Email,
			NoHP:            techData.NoHP,
			JobGroupID:      techData.JobGroupID,
			NIK:             techData.NIK,
			Address:         techData.Address,
			Area:            techData.Area,
			BirthStatus:     techData.TTL,
			UserCreatedOn:   techData.UserCreatedOn,
			LastLogin:       techData.LastLogin,
			LastDownloadJO:  techData.LastDownloadJO,
			EmployeeCode:    techData.EmployeeCode,
		})
	}

	if len(techniciansToCreate) > 0 {
		if err := dbWeb.CreateInBatches(techniciansToCreate, batchSize).Error; err != nil {
			return fmt.Errorf("failed to insert technician data: %v", err)
		}
	}

	// Get Data Planned for Today
	startOfTodayMin7H := startOfToday.Add(-7 * time.Hour)
	endOfTodayMin7H := endOfToday.Add(-7 * time.Hour)
	startDateParam := startOfTodayMin7H.Format("2006-01-02 15:04:05")
	endDateParam := endOfTodayMin7H.Format("2006-01-02 15:04:05")

	ODOOModel := "project.task"
	domain := []interface{}{
		[]interface{}{"planned_date_begin", ">=", startDateParam},
		[]interface{}{"planned_date_begin", "<=", endDateParam},
	}

	fieldID := []string{"id"}
	fields := []string{
		"planned_date_begin",
		"technician_id",
		"helpdesk_ticket_id",
		"x_no_task",
		"timesheet_timer_last_stop",
		"stage_id",
		"x_link_photo",
	}
	order := "planned_date_begin asc"
	odooParams := map[string]interface{}{
		"model":  ODOOModel,
		"domain": domain,
		"fields": fieldID,
		"order":  order,
	}

	payload := map[string]interface{}{
		"jsonrpc": config.GetConfig().ApiODOO.JSONRPC,
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
		return fmt.Errorf("%s: no planned tasks found from ODOO MS", taskDoing)
	}

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
					logrus.Errorf("panic in chunk processing %d: %v", chunkIndex, r)
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
		return fmt.Errorf("%s: no detailed task records found from ODOO MS after processing all chunks", taskDoing)
	}

	ODOOResponseBytes, err := json.Marshal(allRecords)
	if err != nil {
		return fmt.Errorf("failed to marshal all ODOO records: %v", err)
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
		return fmt.Errorf("failed to unmarshal ODOO records into struct: %v", err)
	}

	// Log memory usage for monitoring
	var memStats runtime.MemStats
	runtime.ReadMemStats(&memStats)
	// logrus.Infof("Memory usage before DB operations - Allocated: %d MB, System: %d MB",
	// 	memStats.Alloc/1024/1024, memStats.Sys/1024/1024)

	// Force garbage collection to free up memory before database operations
	runtime.GC()

	// // Group data by technician and create aggregated records
	// groupedData := technicianODOOMSGroupedData(listOfData)

	if err := updateTechnicianODOOMSDataLastVisitFromBatchData(listOfData); err != nil {
		logrus.Errorf("Failed to update Last Visit data: %v", err)
		// Continue even if this fails
	}

	if err := updateTechnicianODOOMSDataFirstUploadedFromBatchData(listOfData); err != nil {
		logrus.Errorf("Failed to update First Uploaded data: %v", err)
		// Continue even if this fails
	}

	if err := updateTechnicianODOOMSDataWONumberAndTicketSubjectFromBatchData(listOfData); err != nil {
		logrus.Errorf("Failed to update WO Number and Ticket Subject data: %v", err)
		// Continue even if this fails
	}

	return nil
}

func updateTechnicianODOOMSDataLastVisitFromBatchData(listOfData []OdooTaskDataRequestItem) error {
	loc, _ := time.LoadLocation(config.GetConfig().Default.Timezone)
	now := time.Now().In(loc)
	startOfDay := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, loc)
	endOfDay := time.Date(now.Year(), now.Month(), now.Day(), 23, 59, 59, 0, loc)

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
		if err := dbWeb.Model(&odooms.ODOOMSTechnicianData{}).
			Where("technician = ?", technician).
			Where("created_at >= ? AND created_at <= ?", startOfDay, endOfDay).
			Update("last_visit", latestVisit).Error; err != nil {
			logrus.Errorf("Failed to update last visit for technician %s: %v", technician, err)
		}
	}

	return nil

}

func updateTechnicianODOOMSDataFirstUploadedFromBatchData(listOfData []OdooTaskDataRequestItem) error {
	loc, _ := time.LoadLocation(config.GetConfig().Default.Timezone)
	now := time.Now().In(loc)
	startOfDay := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, loc)
	endOfDay := time.Date(now.Year(), now.Month(), now.Day(), 23, 59, 59, 0, loc)

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
		if err := dbWeb.Model(&odooms.ODOOMSTechnicianData{}).
			Where("technician = ?", technician).
			Where("created_at >= ? AND created_at <= ?", startOfDay, endOfDay).
			Update("first_upload", firstUpload).Error; err != nil {
			logrus.Errorf("Failed to update first upload for technician %s: %v", technician, err)
		}
	}

	return nil
}

func updateTechnicianODOOMSDataWONumberAndTicketSubjectFromBatchData(listOfData []OdooTaskDataRequestItem) error {
	loc, _ := time.LoadLocation(config.GetConfig().Default.Timezone)
	now := time.Now().In(loc)
	startOfDay := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, loc)
	endOfDay := time.Date(now.Year(), now.Month(), now.Day(), 23, 59, 59, 0, loc)

	// Create a map to track the earliest upload time for each technician
	technicianWONumbers := make(map[string][]string)
	technicianTicketSubjects := make(map[string][]string)
	technicianWOLinkedPhotos := make(map[string][]string)
	technicianWOStages := make(map[string][]string)

	technicianWONUmberVisited := make(map[string][]string)
	technicianTicketSubjectVisited := make(map[string][]string)
	technicianWOLinkedPhotosVisited := make(map[string][]string)
	technicianWOStagesVisited := make(map[string][]string)

	// Process the batch data to find the earliest upload time for each technician
	for _, data := range listOfData {
		_, technicianName := parseJSONIDDataCombinedSafe(data.TechnicianId)
		if technicianName == "" {
			continue
		}

		_, ticketSubjectUncleaned := parseJSONIDDataCombinedSafe(data.HelpdeskTicketId)
		ticketSubject := CleanSPKNumber(ticketSubjectUncleaned)

		_, stage := parseJSONIDDataCombinedSafe(data.StageId)

		var linkPhoto string = "N/A"
		if data.LinkPhoto.Valid && data.LinkPhoto.String != "" {
			linkPhoto = data.LinkPhoto.String
		}

		if data.WoNumber != "" {
			technicianWONumbers[technicianName] = append(technicianWONumbers[technicianName], data.WoNumber)
			if stage != "" {
				technicianWOStages[technicianName] = append(technicianWOStages[technicianName], stage)
			}
			technicianWOLinkedPhotos[technicianName] = append(technicianWOLinkedPhotos[technicianName], linkPhoto)

			// *** Visited
			if data.TimesheetLastStop.Valid {
				technicianWONUmberVisited[technicianName] = append(technicianWONUmberVisited[technicianName], data.WoNumber)
				if stage != "" {
					technicianWOStagesVisited[technicianName] = append(technicianWOStagesVisited[technicianName], stage)
				}
				technicianWOLinkedPhotos[technicianName] = append(technicianWOLinkedPhotos[technicianName], linkPhoto)
			}
		}
		if ticketSubject != "" {
			technicianTicketSubjects[technicianName] = append(technicianTicketSubjects[technicianName], ticketSubject)
			if data.TimesheetLastStop.Valid {
				technicianTicketSubjectVisited[technicianName] = append(technicianTicketSubjectVisited[technicianName], ticketSubject)
			}
		}
	}

	for technician, woNumbers := range technicianWONumbers {
		woNumbersJSON, _ := json.Marshal(woNumbers)
		if err := dbWeb.Model(&odooms.ODOOMSTechnicianData{}).
			Where("technician = ?", technician).
			Where("created_at >= ? AND created_at <= ?", startOfDay, endOfDay).
			Update("wo_number", woNumbersJSON).Error; err != nil {
			logrus.Errorf("Failed to update WO numbers for technician %s: %v", technician, err)
		}
	}

	for technician, ticketSubjects := range technicianTicketSubjects {
		ticketSubjectsJSON, _ := json.Marshal(ticketSubjects)
		if err := dbWeb.Model(&odooms.ODOOMSTechnicianData{}).
			Where("technician = ?", technician).
			Where("created_at >= ? AND created_at <= ?", startOfDay, endOfDay).
			Update("ticket_subject", ticketSubjectsJSON).Error; err != nil {
			logrus.Errorf("Failed to update ticket subjects for technician %s: %v", technician, err)
		}
	}

	for technician, linkedPhotos := range technicianWOLinkedPhotos {
		linkedPhotosJSON, _ := json.Marshal(linkedPhotos)
		if err := dbWeb.Model(&odooms.ODOOMSTechnicianData{}).
			Where("technician = ?", technician).
			Where("created_at >= ? AND created_at <= ?", startOfDay, endOfDay).
			Update("wo_link_photos", linkedPhotosJSON).Error; err != nil {
			logrus.Errorf("Failed to update WO linked photos for technician %s: %v", technician, err)
		}
	}

	for technician, stages := range technicianWOStages {
		stagesJSON, _ := json.Marshal(stages)
		if err := dbWeb.Model(&odooms.ODOOMSTechnicianData{}).
			Where("technician = ?", technician).
			Where("created_at >= ? AND created_at <= ?", startOfDay, endOfDay).
			Update("wo_stages", stagesJSON).Error; err != nil {
			logrus.Errorf("Failed to update WO stages for technician %s: %v", technician, err)
		}
	}

	// *** Visited
	for technician, woNumbers := range technicianWONUmberVisited {
		woNumbersJSON, _ := json.Marshal(woNumbers)
		if err := dbWeb.Model(&odooms.ODOOMSTechnicianData{}).
			Where("technician = ?", technician).
			Where("created_at >= ? AND created_at <= ?", startOfDay, endOfDay).
			Update("wo_number_visited", woNumbersJSON).Error; err != nil {
			logrus.Errorf("Failed to update WO numbers visited for technician %s: %v", technician, err)
		}
	}

	for technician, ticketSubjects := range technicianTicketSubjectVisited {
		ticketSubjectsJSON, _ := json.Marshal(ticketSubjects)
		if err := dbWeb.Model(&odooms.ODOOMSTechnicianData{}).
			Where("technician = ?", technician).
			Where("created_at >= ? AND created_at <= ?", startOfDay, endOfDay).
			Update("ticket_subject_visited", ticketSubjectsJSON).Error; err != nil {
			logrus.Errorf("Failed to update ticket subjects visited for technician %s: %v", technician, err)
		}
	}

	for technician, linkedPhotos := range technicianWOLinkedPhotosVisited {
		linkedPhotosJSON, _ := json.Marshal(linkedPhotos)
		if err := dbWeb.Model(&odooms.ODOOMSTechnicianData{}).
			Where("technician = ?", technician).
			Where("created_at >= ? AND created_at <= ?", startOfDay, endOfDay).
			Update("wo_visited_link_photos", linkedPhotosJSON).Error; err != nil {
			logrus.Errorf("Failed to update WO linked photos visited for technician %s: %v", technician, err)
		}
	}

	for technician, stages := range technicianWOStagesVisited {
		stagesJSON, _ := json.Marshal(stages)
		if err := dbWeb.Model(&odooms.ODOOMSTechnicianData{}).
			Where("technician = ?", technician).
			Where("created_at >= ? AND created_at <= ?", startOfDay, endOfDay).
			Update("wo_visited_stages", stagesJSON).Error; err != nil {
			logrus.Errorf("Failed to update WO stages visited for technician %s: %v", technician, err)
		}
	}

	return nil
}

// styleExcelTitle builds a style struct with a customizable fill color.
func styleExcelTitle(hexColor string, userBorder bool, fontColor string) *excelize.Style {
	style := &excelize.Style{
		Alignment: &excelize.Alignment{
			Horizontal: "center",
			Vertical:   "center",
		},
		Fill: excelize.Fill{
			Type:    "pattern",
			Color:   []string{hexColor}, // use the passed-in color
			Pattern: 1,
		},
		Font: &excelize.Font{
			Bold:  true,
			Color: fontColor,
		},
	}

	if userBorder {
		style.Border = []excelize.Border{
			{Type: "left", Color: "000000", Style: 1},
			{Type: "right", Color: "000000", Style: 1},
			{Type: "top", Color: "000000", Style: 1},
			{Type: "bottom", Color: "000000", Style: 1},
		}
	}

	return style
}

func GenerateChartMonitoringLoginVisitTechnicianODOOMSInBackground(excelFilePath string, senderJIDs []string) {
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
	imgOutput := filepath.Join(fileReportDir, fmt.Sprintf("chartReportSummaryTechnicianAttendance_%s.png", time.Now().Format("02Jan2006")))

	logFile, _ := os.Create(config.GetConfig().Report.MonitoringLoginVisitTechnician.LogExportChartDebugPath)
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

	// ✅ Step 4: Set the print area to ensure the chart and table are captured (reduce height for better PDF output).
	if err := f.SetDefinedName(&excelize.DefinedName{
		Name:     "_xlnm.Print_Area",
		RefersTo: fmt.Sprintf("'%s'!$A$1:$BZ$70", sheetNameToKeep),
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
	idMsg := "Berikut Laporan Kehadiran dan Kunjungan Teknisi"
	enMsg := "Here is the Technician Attendance and Visit Report"
	for _, jid := range senderJIDs {
		SendLangDocumentViaBotWhatsapp(jid, idMsg, enMsg, "id", absImg)
		logExport(fmt.Sprintf("✅ Image sent successfully via WhatsApp to %s", jid))
		os.Remove(absImg) // Clean up the image after sending
	}

}

func GenerateExcelChartMonitoringLoginVisitTechnicianODOOMSInBackground(excelFilePath string, senderJIDs []string) {
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

	logFile, _ := os.Create(config.GetConfig().Report.MonitoringLoginVisitTechnician.LogExportChartDebugPath)
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
	tempExcel = filepath.Join(os.TempDir(), fmt.Sprintf("chartReportSummaryTechnicianAttendance_%v.xlsx", time.Now().Format("02Jan2006")))
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

	// ✅ Step 4: Set the print area to ensure the chart and table are captured (reduce height for better PDF output).
	if err := f.SetDefinedName(&excelize.DefinedName{
		Name:     "_xlnm.Print_Area",
		RefersTo: fmt.Sprintf("'%s'!$A$1:$BZ$70", sheetNameToKeep),
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

	idMsg := "Berikut Laporan Kehadiran dan Kunjungan Teknisi"
	enMsg := "Here is the Technician Attendance and Visit Report"
	for _, jid := range senderJIDs {
		SendLangDocumentViaBotWhatsapp(jid, idMsg, enMsg, "id", tempExcel)
		logExport(fmt.Sprintf("✅ Excel sent successfully via WhatsApp to %s", jid))
	}
}

// func technicianODOOMSGroupedData(listOfData []OdooTaskDataRequestItem) []odooms.ODOOMSTechnicianData {
// 	// Map to group data by technician
// 	technicianMap := make(map[string]*TechnicianAggregateData)

// 	// Process each data item
// 	for _, data := range listOfData {
// 		_, technicianName := parseJSONIDDataCombinedSafe(data.TechnicianId)
// 		if technicianName == "" {
// 			continue // skip records without technician
// 		}

// 		_, ticketSubjectUncleaned := parseJSONIDDataCombinedSafe(data.HelpdeskTicketId)
// 		ticketSubject := CleanSPKNumber(ticketSubjectUncleaned)

// 		// Get or create technician aggregate data
// 		if technicianMap[technicianName] == nil {
// 			technicianMap[technicianName] = &TechnicianAggregateData{
// 				TechnicianName: technicianName,
// 				WONumbers:      []string{},
// 				TicketSubjects: []string{},
// 				FirstUploaded:  nil,
// 				LatestVisit:    nil,
// 			}
// 		}

// 		// Add WO number to array (just the string value)
// 		if data.WoNumber != "" {
// 			technicianMap[technicianName].WONumbers = append(technicianMap[technicianName].WONumbers, data.WoNumber)
// 		}

// 		// Add ticket subject to array (just the string value)
// 		if ticketSubject != "" {
// 			technicianMap[technicianName].TicketSubjects = append(technicianMap[technicianName].TicketSubjects, ticketSubject)
// 		}

// 		// Update latest visit time
// 		if data.TimesheetLastStop.Valid {
// 			if technicianMap[technicianName].LatestVisit == nil || data.TimesheetLastStop.Time.After(*technicianMap[technicianName].LatestVisit) {
// 				technicianMap[technicianName].LatestVisit = &data.TimesheetLastStop.Time
// 			}
// 		}
// 	}

// 	// Convert map to slice of database records
// 	var result []odooms.ODOOMSTechnicianData
// 	for _, aggData := range technicianMap {
// 		// Convert arrays to JSON
// 		woNumbersJSON, _ := json.Marshal(aggData.WONumbers)
// 		ticketSubjectsJSON, _ := json.Marshal(aggData.TicketSubjects)

// 		// Get login and download times from TechODOOMSData
// 		var lastLogin, lastDownload *time.Time
// 		if techData, exists := TechODOOMSData[aggData.TechnicianName]; exists {
// 			lastLogin = techData.LastLogin
// 			lastDownload = techData.LastDownloadJO
// 		}

// 		record := odooms.ODOOMSTechnicianData{
// 			WONumber:       woNumbersJSON,
// 			TicketSubject:  ticketSubjectsJSON,
// 			FirstUpload:    aggData.FirstUploaded,
// 			LastVisit:      aggData.LatestVisit,
// 			LastLogin:      lastLogin,
// 			LastDownloadJO: lastDownload,
// 		}

// 		result = append(result, record)
// 	}

// 	return result
// }
