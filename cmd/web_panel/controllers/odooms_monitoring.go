package controllers

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"time"

	"service-platform/cmd/web_panel/config"
	"service-platform/cmd/web_panel/fun"
	"service-platform/cmd/web_panel/internal/gormdb"
	odooms "service-platform/cmd/web_panel/model/odoo_ms"
	reportmodel "service-platform/cmd/web_panel/model/report_model"
	sptechnicianmodel "service-platform/cmd/web_panel/model/sp_technician_model"

	"github.com/gin-gonic/gin"
	"github.com/sirupsen/logrus"
	"github.com/xuri/excelize/v2"
	"gorm.io/gorm"
)

func GetTicketODOOMSPerformanceAchivementsChart() gin.HandlerFunc {
	return func(c *gin.Context) {
		importPath := config.GetConfig().App.Logo
		newLogoPath := importPath[:len(importPath)-len(filepath.Base(importPath))] + "csna.png"
		c.HTML(http.StatusOK, "tab-ticket-performance.html", gin.H{
			"GLOBAL_URL": fun.GLOBAL_URL,
			"APP_LOGO":   newLogoPath,
			"ACCESS":     true,
		})
	}
}

// Route to serve chart data as JSON
func GetDataTicketPerformance() gin.HandlerFunc {
	return func(c *gin.Context) {
		dbWeb := gormdb.Databases.Web

		// Chart row struct for aggregation (date as string for category xAxis)
		type ChartRow struct {
			Date                       string `json:"date"` // e.g. "01 Sep"
			TotalPM                    int64  `json:"Total_PM"`
			TotalPMPerDay              int64  `json:"Total_PM_Per_Day"`
			TotalCM                    int64  `json:"Total_CM"`
			TotalNonPM                 int64  `json:"Total_Non_PM"`
			TotalNonPMPerDay           int64  `json:"Total_Non_PM_Per_Day"`
			TotalNonPMOverdue          int64  `json:"Total_Non_PM_Overdue"`
			TotalNonPMOverduePerDay    int64  `json:"Total_Non_PM_Overdue_Per_Day"`
			Target                     int64  `json:"Target"`
			TotalTicket                int64  `json:"Total_Ticket"`
			TotalTicketPerDay          int64  `json:"Total_Ticket_Per_Day"`
			TotalAchievement           int64  `json:"Total_Achievement"`
			TotalAchievementPerDay     int64  `json:"Total_Achievement_Per_Day"`
			TotalMeetSLA               int64  `json:"Total_Meet_SLA"`
			TotalOverdue               int64  `json:"Total_Overdue"`
			TotalDone                  int64  `json:"Total_Done"`
			TotalDonePerDay            int64  `json:"Total_Done_Per_Day"`
			TotalPendingOnTarget       int64  `json:"Total_Pending_On_Target"`
			TotalPendingOverdueVisited int64  `json:"Total_Pending_Overdue_Visited"`
			TotalPendingOverdueNew     int64  `json:"Total_Pending_Overdue_New"`
			TotalPendingNotVisit       int64  `json:"Total_Pending_Not_Visit"`
		}

		// Read filter values from POST (form or JSON)
		dataDate := c.PostForm("data_date")
		sac := c.PostForm("sac")
		spl := c.PostForm("spl")
		technician := c.PostForm("technician")
		company := c.PostForm("company")
		slaStatus := c.PostForm("sla_status")
		taskType := c.PostForm("task_type")
		// Also support JSON body
		if dataDate == "" || sac == "" || spl == "" || technician == "" || company == "" || slaStatus == "" || taskType == "" {
			var jsonBody struct {
				DataDate   string `json:"data_date"`
				SAC        string `json:"sac"`
				SPL        string `json:"spl"`
				Technician string `json:"technician"`
				Company    string `json:"company"`
				SLAStatus  string `json:"sla_status"`
				TaskType   string `json:"task_type"`
			}
			if err := c.ShouldBindJSON(&jsonBody); err == nil {
				if dataDate == "" {
					dataDate = jsonBody.DataDate
				}
				if sac == "" {
					sac = jsonBody.SAC
				}
				if spl == "" {
					spl = jsonBody.SPL
				}
				if technician == "" {
					technician = jsonBody.Technician
				}
				if company == "" {
					company = jsonBody.Company
				}
				if slaStatus == "" {
					slaStatus = jsonBody.SLAStatus
				}
				if taskType == "" {
					taskType = jsonBody.TaskType
				}
			}
		}

		// Parse comma-separated values for multiple selections
		sacList := []string{}
		if sac != "" {
			sacList = strings.Split(sac, ",")
			// Trim whitespace
			for i := range sacList {
				sacList[i] = strings.TrimSpace(sacList[i])
			}
		}
		splList := []string{}
		if spl != "" {
			splList = strings.Split(spl, ",")
			for i := range splList {
				splList[i] = strings.TrimSpace(splList[i])
			}
		}
		technicianList := []string{}
		if technician != "" {
			technicianList = strings.Split(technician, ",")
			for i := range technicianList {
				technicianList[i] = strings.TrimSpace(technicianList[i])
			}
		}
		companyList := []string{}
		if company != "" {
			companyList = strings.Split(company, ",")
			for i := range companyList {
				companyList[i] = strings.TrimSpace(companyList[i])
			}
		}
		slaStatusList := []string{}
		if slaStatus != "" {
			slaStatusList = strings.Split(slaStatus, ",")
			for i := range slaStatusList {
				slaStatusList[i] = strings.TrimSpace(slaStatusList[i])
			}
		}
		taskTypeList := []string{}
		isNonPMSelected := false
		if taskType != "" {
			taskTypeList = strings.Split(taskType, ",")
			for i := range taskTypeList {
				taskTypeList[i] = strings.TrimSpace(taskTypeList[i])
				if taskTypeList[i] == "Non-PM" {
					isNonPMSelected = true
				}
			}
		}

		table := config.GetConfig().Database.TbReportMonitoringTicket
		// For current month (dataDate empty), use base table
		// For specific months, use monthly tables if they exist
		if dataDate != "" {
			// Parse selected data_date to get month and year
			parts := strings.Split(dataDate, " ")
			if len(parts) == 2 {
				var targetYear int
				var targetMonth time.Month
				if parsedYear, err := strconv.Atoi(parts[1]); err == nil {
					targetYear = parsedYear
				}
				// Parse month name to month number
				for m := 1; m <= 12; m++ {
					if strings.EqualFold(time.Month(m).String()[:3], parts[0]) {
						targetMonth = time.Month(m)
						break
					}
				}
				if targetYear != 0 && targetMonth != 0 {
					// Construct table name with month suffix
					monthStr := strings.ToLower(targetMonth.String()[:3])
					yearStr := strconv.Itoa(targetYear)
					table = fmt.Sprintf("%s_%s%s", table, monthStr, yearStr)
				}
			}
		}
		// Build dynamic WHERE clause for filters
		var filterWhere []string
		var filterArgs []interface{}

		// Build technician list based on SAC/SPL/Technician filters
		technicianFilterList := []string{}
		if len(sacList) > 0 || len(splList) > 0 {
			for tech, techData := range TechODOOMSData {
				matchSAC := len(sacList) == 0
				matchSPL := len(splList) == 0

				// Check if technician matches any selected SAC
				for _, s := range sacList {
					if techData.SAC == s {
						matchSAC = true
						break
					}
				}
				// Check if technician matches any selected SPL
				for _, s := range splList {
					if techData.SPL == s {
						matchSPL = true
						break
					}
				}
				// Add technician if matches both SAC and SPL filters (AND logic)
				if (len(sacList) == 0 || matchSAC) && (len(splList) == 0 || matchSPL) {
					technicianFilterList = append(technicianFilterList, tech)
				}
			}
		}
		// If specific technicians are selected, use only those
		if len(technicianList) > 0 {
			technicianFilterList = technicianList
		}
		if len(technicianFilterList) > 0 {
			qs := strings.Repeat("?,", len(technicianFilterList))
			qs = strings.TrimRight(qs, ",")
			filterWhere = append(filterWhere, fmt.Sprintf("technician IN (%s)", qs))
			for _, t := range technicianFilterList {
				filterArgs = append(filterArgs, t)
			}
		}
		if len(companyList) > 0 {
			qs := strings.Repeat("?,", len(companyList))
			qs = strings.TrimRight(qs, ",")
			filterWhere = append(filterWhere, fmt.Sprintf("company IN (%s)", qs))
			for _, co := range companyList {
				filterArgs = append(filterArgs, co)
			}
		}
		if len(slaStatusList) > 0 {
			qs := strings.Repeat("?,", len(slaStatusList))
			qs = strings.TrimRight(qs, ",")
			filterWhere = append(filterWhere, fmt.Sprintf("sla_status IN (%s)", qs))
			for _, sla := range slaStatusList {
				filterArgs = append(filterArgs, sla)
			}
		}
		// Task Type filter
		if len(taskTypeList) > 0 {
			if isNonPMSelected {
				// If Non-PM is selected, filter task_type != 'Preventive Maintenance'
				filterWhere = append(filterWhere, "task_type != ?")
				filterArgs = append(filterArgs, "Preventive Maintenance")
			} else {
				// Regular filter for specific task types
				qs := strings.Repeat("?,", len(taskTypeList))
				qs = strings.TrimRight(qs, ",")
				filterWhere = append(filterWhere, fmt.Sprintf("task_type IN (%s)", qs))
				for _, tt := range taskTypeList {
					filterArgs = append(filterArgs, tt)
				}
			}
		}
		filterWhereStr := ""
		if len(filterWhere) > 0 {
			filterWhereStr = " AND " + strings.Join(filterWhere, " AND ")
		}

		// 1. Get all dates in selected month (or current month if not specified)
		now := time.Now()
		year, month := now.Year(), now.Month()
		if dataDate != "" {
			// Parse selected data_date to get year and month
			parts := strings.Split(dataDate, " ")
			if len(parts) == 2 {
				if parsedYear, err := strconv.Atoi(parts[1]); err == nil {
					year = parsedYear
				}
				// Parse month name to month number
				for m := 1; m <= 12; m++ {
					if strings.EqualFold(time.Month(m).String()[:3], parts[0]) {
						month = time.Month(m)
						break
					}
				}
			}
		}
		firstOfMonth := time.Date(year, month, 1, 0, 0, 0, 0, now.Location())
		lastOfMonth := firstOfMonth.AddDate(0, 1, -1)
		daysInMonth := lastOfMonth.Day()
		dateLabels := make([]string, 0, daysInMonth)
		for d := 1; d <= daysInMonth; d++ {
			dateLabels = append(dateLabels, fmt.Sprintf("%02d %s", d, month.String()[:3]))
		}

		// 2. Query for PM, CM, Non-PM, and daily ticket (all based on received_spk_at)
		var rows []struct {
			Date        string
			TotalPM     int64
			TotalCM     int64
			TotalNonPM  int64
			DailyTicket int64
		}
		pmcmWhere := fmt.Sprintf("received_spk_at >= ? AND received_spk_at <= ?%s", filterWhereStr)
		pmcmArgs := append([]interface{}{firstOfMonth, lastOfMonth}, filterArgs...)
		sqlPMCM := `
			SELECT
				DATE_FORMAT(received_spk_at, '%d %b') as date,
				SUM(CASE WHEN task_type = 'Preventive Maintenance' THEN 1 ELSE 0 END) as total_pm,
				SUM(CASE WHEN task_type = 'Corrective Maintenance' THEN 1 ELSE 0 END) as total_cm,
				SUM(CASE WHEN task_type != 'Preventive Maintenance' THEN 1 ELSE 0 END) as total_non_pm,
				COUNT(*) as daily_ticket
			FROM ` + table + `
			WHERE ` + pmcmWhere + `
			GROUP BY date
		`
		dbWeb.Raw(sqlPMCM, pmcmArgs...).Scan(&rows)
		// Map for quick lookup
		rowMap := make(map[string]struct {
			TotalPM, TotalCM, TotalNonPM, DailyTicket int64
		})
		for _, r := range rows {
			rowMap[r.Date] = struct {
				TotalPM, TotalCM, TotalNonPM, DailyTicket int64
			}{r.TotalPM, r.TotalCM, r.TotalNonPM, r.DailyTicket}
		}

		// 3. Query for Total Achievement, Meet SLA, and Overdue (all based on complete_wo in current month, exclude stage = 'Cancel')
		var achRows []struct {
			Date        string
			Achievement int64
		}
		achWhere := fmt.Sprintf("complete_wo >= ? AND complete_wo <= ? AND stage != 'Cancel'%s", filterWhereStr)
		achArgs := append([]interface{}{firstOfMonth, lastOfMonth}, filterArgs...)
		sqlAch := `
			SELECT
				DATE_FORMAT(complete_wo, '%d %b') as date,
				COUNT(*) as achievement
			FROM ` + table + `
			WHERE ` + achWhere + `
			GROUP BY date
		`
		dbWeb.Raw(sqlAch, achArgs...).Scan(&achRows)
		achMap := make(map[string]int64)
		for _, a := range achRows {
			achMap[a.Date] = a.Achievement
		}

		// Query for Meet SLA (sla_status = 'On Target Solved')
		var meetSlaRows []struct {
			Date    string
			MeetSLA int64
		}
		meetSlaWhere := fmt.Sprintf("complete_wo >= ? AND complete_wo <= ? AND sla_status = 'On Target Solved' AND stage != 'Cancel'%s", filterWhereStr)
		meetSlaArgs := append([]interface{}{firstOfMonth, lastOfMonth}, filterArgs...)
		sqlMeetSla := `
			SELECT
				DATE_FORMAT(complete_wo, '%d %b') as date,
				COUNT(*) as meet_sla
			FROM ` + table + `
			WHERE ` + meetSlaWhere + `
			GROUP BY date
		`
		dbWeb.Raw(sqlMeetSla, meetSlaArgs...).Scan(&meetSlaRows)
		meetSlaMap := make(map[string]int64)
		for _, m := range meetSlaRows {
			meetSlaMap[m.Date] = m.MeetSLA
		}

		// Query for Overdue (sla_status LIKE 'Overdue%')
		var overdueRows []struct {
			Date    string
			Overdue int64
		}
		overdueWhere := fmt.Sprintf("complete_wo >= ? AND complete_wo <= ? AND sla_status LIKE 'Overdue%%' AND stage != 'Cancel'%s", filterWhereStr)
		overdueArgs := append([]interface{}{firstOfMonth, lastOfMonth}, filterArgs...)
		sqlOverdue := `
			SELECT
				DATE_FORMAT(complete_wo, '%d %b') as date,
				COUNT(*) as overdue
			FROM ` + table + `
			WHERE ` + overdueWhere + `
			GROUP BY date
		`
		dbWeb.Raw(sqlOverdue, overdueArgs...).Scan(&overdueRows)
		overdueMap := make(map[string]int64)
		for _, o := range overdueRows {
			overdueMap[o.Date] = o.Overdue
		}

		// Query for Non-PM Overdue (task_type != 'Preventive Maintenance' AND received_spk_at in current month AND sla_status LIKE 'Overdue%')
		var nonPMOverdueRows []struct {
			Date         string
			NonPMOverdue int64
		}
		nonPMOverdueWhere := fmt.Sprintf("received_spk_at >= ? AND received_spk_at <= ? AND task_type != 'Preventive Maintenance' AND sla_status LIKE 'Overdue%%'%s", filterWhereStr)
		nonPMOverdueArgs := append([]interface{}{firstOfMonth, lastOfMonth}, filterArgs...)
		sqlNonPMOverdue := `
			SELECT
				DATE_FORMAT(received_spk_at, '%d %b') as date,
				COUNT(*) as non_pm_overdue
			FROM ` + table + `
			WHERE ` + nonPMOverdueWhere + `
			GROUP BY date
		`
		dbWeb.Raw(sqlNonPMOverdue, nonPMOverdueArgs...).Scan(&nonPMOverdueRows)
		nonPMOverdueMap := make(map[string]int64)
		for _, no := range nonPMOverdueRows {
			nonPMOverdueMap[no.Date] = no.NonPMOverdue
		}

		// Query for Total Done (stage in specified list, based on complete_wo)
		var doneRows []struct {
			Date string
			Done int64
		}
		doneWhere := fmt.Sprintf("complete_wo >= ? AND complete_wo <= ? AND stage IN ('Solved', 'Pending', 'Solved Pending', 'Done', 'Closed', 'Waiting For Verification') AND complete_wo IS NOT NULL%s", filterWhereStr)
		doneArgs := append([]interface{}{firstOfMonth, lastOfMonth}, filterArgs...)
		sqlDone := `
			SELECT
				DATE_FORMAT(complete_wo, '%d %b') as date,
				COUNT(*) as done
			FROM ` + table + `
			WHERE ` + doneWhere + `
			GROUP BY date
		`
		dbWeb.Raw(sqlDone, doneArgs...).Scan(&doneRows)
		doneMap := make(map[string]int64)
		for _, d := range doneRows {
			doneMap[d.Date] = d.Done
		}

		// Query for Total Pending (stage = 'Pending', based on received_spk_at, grouped by sla_status)
		var pendingRows []struct {
			Date      string
			SLAStatus string
			Pending   int64
		}
		pendingWhere := fmt.Sprintf("received_spk_at >= ? AND received_spk_at <= ? AND stage = 'Pending'%s", filterWhereStr)
		pendingArgs := append([]interface{}{firstOfMonth, lastOfMonth}, filterArgs...)
		sqlPending := `
			SELECT
				DATE_FORMAT(received_spk_at, '%d %b') as date,
				sla_status,
				COUNT(*) as pending
			FROM ` + table + `
			WHERE ` + pendingWhere + `
			GROUP BY date, sla_status
		`
		dbWeb.Raw(sqlPending, pendingArgs...).Scan(&pendingRows)
		pendingMap := make(map[string]map[string]int64)
		for _, p := range pendingRows {
			if pendingMap[p.Date] == nil {
				pendingMap[p.Date] = make(map[string]int64)
			}
			pendingMap[p.Date][p.SLAStatus] = p.Pending
		}

		// 4. Build chartData for each date in current month with cumulative logic and per-day achievement
		chartData := make([]ChartRow, 0, daysInMonth)
		var cumPM int64 = 0
		var cumCM int64 = 0
		var cumNonPM int64 = 0
		var cumNonPMOverdue int64 = 0
		var cumTicket int64 = 0
		var cumAchievement int64 = 0
		var cumMeetSLA int64 = 0
		var cumOverdue int64 = 0
		var cumDone int64 = 0
		var cumPendingOnTarget int64 = 0
		var cumPendingOverdueVisited int64 = 0
		var cumPendingOverdueNew int64 = 0
		var cumPendingNotVisit int64 = 0
		// Get last date's cumulative ticket for target calculation
		var lastCumTicket int64 = 0
		for _, date := range dateLabels {
			pmcm := rowMap[date]
			cumTicket += pmcm.DailyTicket
			lastCumTicket = cumTicket
		}
		// Calculate dailyTargetValue (rounded up)
		dailyTargetValue := int64(0)
		if daysInMonth > 0 {
			dailyTargetValue = (lastCumTicket + int64(daysInMonth) - 1) / int64(daysInMonth)
		}
		// Now build chartData with cumulative logic for all series
		cumPM = 0
		cumCM = 0
		cumNonPM = 0
		cumNonPMOverdue = 0
		cumTicket = 0
		cumAchievement = 0
		cumMeetSLA = 0
		cumOverdue = 0
		cumDone = 0
		cumPendingOnTarget = 0
		cumPendingOverdueVisited = 0
		cumPendingOverdueNew = 0
		cumPendingNotVisit = 0
		cumTarget := int64(0)
		for _, date := range dateLabels {
			pmcm := rowMap[date]
			cumPM += pmcm.TotalPM
			cumCM += pmcm.TotalCM
			cumNonPM += pmcm.TotalNonPM
			cumTicket += pmcm.DailyTicket
			cumTarget += dailyTargetValue
			ach := achMap[date]
			cumAchievement += ach
			meetSLA := meetSlaMap[date]
			cumMeetSLA += meetSLA
			overdue := overdueMap[date]
			cumOverdue += overdue
			done := doneMap[date]
			cumDone += done
			nonPMOverdue := nonPMOverdueMap[date]
			cumNonPMOverdue += nonPMOverdue
			// Add pending counts based on sla_status
			if pendingData, exists := pendingMap[date]; exists {
				cumPendingOnTarget += pendingData["On Target Solved"]
				cumPendingOverdueVisited += pendingData["Overdue (Visited)"]
				cumPendingOverdueNew += pendingData["Overdue (New)"]
				cumPendingNotVisit += pendingData["Not Visit"]
			}
			chartData = append(chartData, ChartRow{
				Date:                       date,
				TotalPM:                    cumPM,
				TotalPMPerDay:              pmcm.TotalPM,
				TotalCM:                    cumCM,
				TotalNonPM:                 cumNonPM,
				TotalNonPMPerDay:           pmcm.TotalNonPM,
				TotalNonPMOverdue:          cumNonPMOverdue,
				TotalNonPMOverduePerDay:    nonPMOverdue,
				Target:                     cumTarget,
				TotalTicket:                cumTicket,
				TotalTicketPerDay:          pmcm.DailyTicket,
				TotalAchievement:           cumAchievement,
				TotalAchievementPerDay:     ach,
				TotalMeetSLA:               cumMeetSLA,
				TotalOverdue:               cumOverdue,
				TotalDone:                  cumDone,
				TotalDonePerDay:            done,
				TotalPendingOnTarget:       cumPendingOnTarget,
				TotalPendingOverdueVisited: cumPendingOverdueVisited,
				TotalPendingOverdueNew:     cumPendingOverdueNew,
				TotalPendingNotVisit:       cumPendingNotVisit,
			})
		}

		var lastUpdatedData struct{ LastUpdate time.Time }
		if err := dbWeb.Table(table).
			Select("MAX(updated_at) as last_update").
			Scan(&lastUpdatedData).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{
				"success": false,
				"error":   err.Error(),
			})
			return
		}

		c.JSON(http.StatusOK, gin.H{
			"success":     true,
			"data":        chartData,
			"chart_title": fmt.Sprintf("ACHIEVEMENTS %v", strings.ToUpper(firstOfMonth.Format("January 2006"))),
			"last_update": lastUpdatedData.LastUpdate.Format("Monday, 02 January 2006 15:04 PM"),
		})
	}
}

func GetDataTicketPerformanceCostRevenue() gin.HandlerFunc {
	return func(c *gin.Context) {
		dbWeb := gormdb.Databases.Web

		// Chart row struct for daily aggregation
		type ChartRow struct {
			Date                         string  `json:"date"` // e.g. "01 Sep"
			DailyRevenue                 int64   `json:"Daily_Revenue"`
			DailyCostToTechnicians       int64   `json:"Daily_Cost_To_Technicians"`
			DailyCostOfPenalty           int64   `json:"Daily_Cost_Of_Penalty"`
			AccumulatedRevenue           int64   `json:"Accumulated_Revenue"`
			AccumulatedCostToTechnicians int64   `json:"Accumulated_Cost_To_Technicians"`
			AccumulatedCostOfPenalty     int64   `json:"Accumulated_Cost_Of_Penalty"`
			AccumulatedProfit            int64   `json:"Accumulated_Profit"`
			RevenuePrice                 float64 `json:"Revenue_Price"`
			CostToTechniciansPrice       float64 `json:"Cost_To_Technicians_Price"`
			CostOfPenaltyPrice           float64 `json:"Cost_Of_Penalty_Price"`
			ProfitPrice                  float64 `json:"Profit_Price"`
		}

		// Read filter values from POST (form or JSON)
		dataDate := c.PostForm("data_date")
		sac := c.PostForm("sac")
		spl := c.PostForm("spl")
		technician := c.PostForm("technician")
		company := c.PostForm("company")

		// Parse comma-separated values for multiple selections
		sacList := []string{}
		if sac != "" {
			sacList = strings.Split(sac, ",")
			for i := range sacList {
				sacList[i] = strings.TrimSpace(sacList[i])
			}
		}
		splList := []string{}
		if spl != "" {
			splList = strings.Split(spl, ",")
			for i := range splList {
				splList[i] = strings.TrimSpace(splList[i])
			}
		}
		technicianList := []string{}
		if technician != "" {
			technicianList = strings.Split(technician, ",")
			for i := range technicianList {
				technicianList[i] = strings.TrimSpace(technicianList[i])
			}
		}
		companyList := []string{}
		if company != "" {
			companyList = strings.Split(company, ",")
			for i := range companyList {
				companyList[i] = strings.TrimSpace(companyList[i])
			}
		}

		table := config.GetConfig().Database.TbReportMonitoringTicket
		// For current month (dataDate empty), use base table
		// For specific months, use monthly tables if they exist
		if dataDate != "" {
			// Parse selected data_date to get month and year
			parts := strings.Split(dataDate, " ")
			if len(parts) == 2 {
				var targetYear int
				var targetMonth time.Month
				if parsedYear, err := strconv.Atoi(parts[1]); err == nil {
					targetYear = parsedYear
				}
				// Parse month name to month number
				for m := 1; m <= 12; m++ {
					if strings.EqualFold(time.Month(m).String()[:3], parts[0]) {
						targetMonth = time.Month(m)
						break
					}
				}
				if targetYear != 0 && targetMonth != 0 {
					// Construct table name with month suffix
					monthStr := strings.ToLower(targetMonth.String()[:3])
					yearStr := strconv.Itoa(targetYear)
					table = fmt.Sprintf("%s_%s%s", table, monthStr, yearStr)
				}
			}
		}

		// Build dynamic WHERE clause for filters
		var filterWhere []string
		var filterArgs []interface{}

		// Build technician list based on SAC/SPL/Technician filters
		technicianFilterList := []string{}
		if len(sacList) > 0 || len(splList) > 0 {
			for tech, techData := range TechODOOMSData {
				matchSAC := len(sacList) == 0
				matchSPL := len(splList) == 0

				// Check if technician matches any selected SAC
				for _, s := range sacList {
					if techData.SAC == s {
						matchSAC = true
						break
					}
				}
				// Check if technician matches any selected SPL
				for _, s := range splList {
					if techData.SPL == s {
						matchSPL = true
						break
					}
				}
				// Add technician if matches both SAC and SPL filters (AND logic)
				if (len(sacList) == 0 || matchSAC) && (len(splList) == 0 || matchSPL) {
					technicianFilterList = append(technicianFilterList, tech)
				}
			}
		}
		// If specific technicians are selected, use only those
		if len(technicianList) > 0 {
			technicianFilterList = technicianList
		}
		if len(technicianFilterList) > 0 {
			qs := strings.Repeat("?,", len(technicianFilterList))
			qs = strings.TrimRight(qs, ",")
			filterWhere = append(filterWhere, fmt.Sprintf("technician IN (%s)", qs))
			for _, t := range technicianFilterList {
				filterArgs = append(filterArgs, t)
			}
		}
		if len(companyList) > 0 {
			qs := strings.Repeat("?,", len(companyList))
			qs = strings.TrimRight(qs, ",")
			filterWhere = append(filterWhere, fmt.Sprintf("company IN (%s)", qs))
			for _, co := range companyList {
				filterArgs = append(filterArgs, co)
			}
		}
		filterWhereStr := ""
		if len(filterWhere) > 0 {
			filterWhereStr = " AND " + strings.Join(filterWhere, " AND ")
		}

		defaultPriceforCost := config.GetConfig().ODOOMSParam.DefaultPrice
		defaultPenaltyPrice := config.GetConfig().ODOOMSParam.DefaultPenalty

		// 1. Get all dates in selected month (or current month if not specified)
		now := time.Now()
		year, month := now.Year(), now.Month()
		if dataDate != "" {
			// Parse selected data_date to get year and month
			parts := strings.Split(dataDate, " ")
			if len(parts) == 2 {
				if parsedYear, err := strconv.Atoi(parts[1]); err == nil {
					year = parsedYear
				}
				// Parse month name to month number
				for m := 1; m <= 12; m++ {
					if strings.EqualFold(time.Month(m).String()[:3], parts[0]) {
						month = time.Month(m)
						break
					}
				}
			}
		}
		firstOfMonth := time.Date(year, month, 1, 0, 0, 0, 0, now.Location())
		lastOfMonth := firstOfMonth.AddDate(0, 1, -1)
		daysInMonth := lastOfMonth.Day()
		dateLabels := make([]string, 0, daysInMonth)
		for d := 1; d <= daysInMonth; d++ {
			dateLabels = append(dateLabels, fmt.Sprintf("%02d %s", d, month.String()[:3]))
		}

		// 2. Query for daily revenue counts and prices (based on received_spk_at)
		var revenueRows []struct {
			Date     string
			Company  string
			TaskType string
			Count    int64
		}
		revenueWhere := fmt.Sprintf(`received_spk_at >= ? AND received_spk_at <= ? AND (
			(stage IN ('Done', 'Waiting For Verification', 'Closed', 'Solved Pending') AND complete_wo IS NOT NULL)
			OR (stage = 'Pending' AND sla_status = 'On Target Solved')
		)%s`, filterWhereStr)
		revenueArgs := append([]interface{}{firstOfMonth, lastOfMonth}, filterArgs...)
		sqlRevenue := `
			SELECT
				DATE_FORMAT(received_spk_at, '%d %b') as date,
				company,
				task_type,
				COUNT(*) as count
			FROM ` + table + `
			WHERE ` + revenueWhere + `
			GROUP BY date, company, task_type
		`
		dbWeb.Raw(sqlRevenue, revenueArgs...).Scan(&revenueRows)

		// Build price map for company-task combinations
		priceMap := make(map[string]float64)
		for _, row := range revenueRows {
			key := fmt.Sprintf("%s|%s", row.Company, row.TaskType)
			if _, exists := priceMap[key]; !exists {
				var productTemplate odooms.InventoryProductTemplate
				err := dbWeb.Model(&odooms.InventoryProductTemplate{}).
					Select("list_price").
					Where("product_type = ? AND product_category = ?", "service", "Manage Service").
					Where("company = ? AND name = ?", row.Company, row.TaskType).
					First(&productTemplate).Error
				if err == nil {
					priceMap[key] = productTemplate.ListPrice
				} else {
					priceMap[key] = 0
				}
			}
		}

		// Map for daily revenue
		revenueMap := make(map[string]struct {
			Count int64
			Price float64
		})
		for _, r := range revenueRows {
			key := fmt.Sprintf("%s|%s", r.Company, r.TaskType)
			price := priceMap[key]
			existing := revenueMap[r.Date]
			existing.Count += r.Count
			existing.Price += float64(r.Count) * price
			revenueMap[r.Date] = existing
		}

		// 3. Query for daily Cost to Technicians (based on received_spk_at)
		// Cost to Technicians: complete_wo IS NOT NULL AND (stage IN ('Done', 'Waiting For Verification', 'Closed', 'Solved Pending') OR (stage = 'Pending' AND sla_status = 'On Target Solved'))
		var costToTechRows []struct {
			Date  string
			Count int64
		}
		costToTechWhere := fmt.Sprintf(`received_spk_at >= ? AND received_spk_at <= ? AND complete_wo IS NOT NULL AND (
			stage IN ('Done', 'Waiting For Verification', 'Closed', 'Solved Pending')
			OR (stage = 'Pending' AND sla_status = 'On Target Solved')
		)%s`, filterWhereStr)
		costToTechArgs := append([]interface{}{firstOfMonth, lastOfMonth}, filterArgs...)
		sqlCostToTech := `
			SELECT
				DATE_FORMAT(received_spk_at, '%d %b') as date,
				COUNT(*) as count
			FROM ` + table + `
			WHERE ` + costToTechWhere + `
			GROUP BY date
		`
		dbWeb.Raw(sqlCostToTech, costToTechArgs...).Scan(&costToTechRows)

		// Map for daily cost to technicians
		costToTechMap := make(map[string]int64)
		for _, c := range costToTechRows {
			costToTechMap[c.Date] = c.Count
		}

		// 4. Query for daily Cost of Penalty (based on received_spk_at)
		// Cost of Penalty: stage = 'New' AND (sla_status = 'Overdue (New)' OR sla_status = 'Overdue (Visited)')
		var costPenaltyRows []struct {
			Date  string
			Count int64
		}
		costPenaltyWhere := fmt.Sprintf(`received_spk_at >= ? AND received_spk_at <= ? AND stage = 'New' AND (
			sla_status = 'Overdue (New)' OR sla_status = 'Overdue (Visited)'
		)%s`, filterWhereStr)
		costPenaltyArgs := append([]interface{}{firstOfMonth, lastOfMonth}, filterArgs...)
		sqlCostPenalty := `
			SELECT
				DATE_FORMAT(received_spk_at, '%d %b') as date,
				COUNT(*) as count
			FROM ` + table + `
			WHERE ` + costPenaltyWhere + `
			GROUP BY date
		`
		dbWeb.Raw(sqlCostPenalty, costPenaltyArgs...).Scan(&costPenaltyRows)

		// Map for daily cost of penalty
		costPenaltyMap := make(map[string]int64)
		for _, c := range costPenaltyRows {
			costPenaltyMap[c.Date] = c.Count
		}

		// 5. Build chartData with cumulative logic
		chartData := make([]ChartRow, 0, daysInMonth)
		var cumRevenueCount int64 = 0
		var cumCostToTechCount int64 = 0
		var cumCostPenaltyCount int64 = 0
		var cumRevenuePrice float64 = 0
		var cumCostToTechPrice float64 = 0
		var cumCostPenaltyPrice float64 = 0

		for _, date := range dateLabels {
			revenueData := revenueMap[date]
			costToTechCount := costToTechMap[date]
			costPenaltyCount := costPenaltyMap[date]

			cumRevenueCount += revenueData.Count
			cumCostToTechCount += costToTechCount
			cumCostPenaltyCount += costPenaltyCount
			cumRevenuePrice += revenueData.Price
			cumCostToTechPrice += float64(costToTechCount) * defaultPriceforCost
			cumCostPenaltyPrice += float64(costPenaltyCount) * defaultPenaltyPrice

			totalCostPrice := cumCostToTechPrice + cumCostPenaltyPrice
			cumProfit := cumRevenuePrice - totalCostPrice
			cumProfitCount := cumRevenueCount - (cumCostToTechCount + cumCostPenaltyCount)

			chartData = append(chartData, ChartRow{
				Date:                         date,
				DailyRevenue:                 revenueData.Count,
				DailyCostToTechnicians:       costToTechCount,
				DailyCostOfPenalty:           costPenaltyCount,
				AccumulatedRevenue:           cumRevenueCount,
				AccumulatedCostToTechnicians: cumCostToTechCount,
				AccumulatedCostOfPenalty:     cumCostPenaltyCount,
				AccumulatedProfit:            cumProfitCount,
				RevenuePrice:                 cumRevenuePrice,
				CostToTechniciansPrice:       cumCostToTechPrice,
				CostOfPenaltyPrice:           cumCostPenaltyPrice,
				ProfitPrice:                  cumProfit,
			})
		}

		// Prepare drill-down data structures
		type DrillItem struct {
			Date        string  `json:"date"`
			Name        string  `json:"name"`
			Y           float64 `json:"y"`
			Count       int64   `json:"count"`
			Price       float64 `json:"price"`
			Description string  `json:"description,omitempty"`
		}

		// Get Revenue drill data: distinct date, company and task_type with count and calculated price
		var revenueDrillRows []struct {
			Date     string
			Company  string
			TaskType string
			Count    int64
		}
		revenueDrillWhere := fmt.Sprintf(`received_spk_at >= ? AND received_spk_at <= ? AND (
		(stage IN ('Done', 'Waiting For Verification', 'Closed', 'Solved Pending') AND complete_wo IS NOT NULL)
		OR (stage = 'Pending' AND sla_status = 'On Target Solved')
	)%s`, filterWhereStr)
		revenueDrillArgs := append([]interface{}{firstOfMonth, lastOfMonth}, filterArgs...)
		sqlRevenueDrill := `
		SELECT DATE_FORMAT(received_spk_at, '%d %b') as date, company, task_type, COUNT(*) as count
		FROM ` + table + `
		WHERE ` + revenueDrillWhere + `
		GROUP BY DATE_FORMAT(received_spk_at, '%d %b'), company, task_type
	`
		dbWeb.Raw(sqlRevenueDrill, revenueDrillArgs...).Scan(&revenueDrillRows) // Calculate revenue drill data with prices
		var revenueDrillData []DrillItem
		for _, row := range revenueDrillRows {
			key := fmt.Sprintf("%s|%s", row.Company, row.TaskType)
			price := priceMap[key]
			calculatedValue := float64(row.Count) * price

			revenueDrillData = append(revenueDrillData, DrillItem{
				Date:        row.Date,
				Name:        fmt.Sprintf("%s - %s", row.Company, row.TaskType),
				Y:           calculatedValue,
				Count:       row.Count,
				Price:       price,
				Description: fmt.Sprintf("Company: %s | Task: %s | Tickets: %d | List Price: Rp %.0f | Total Revenue: Rp %.0f", row.Company, row.TaskType, row.Count, price, calculatedValue),
			})
		} // Get Cost to Technicians drill data: distinct date, company and task_type with count
		var costToTechDrillRows []struct {
			Date     string
			Company  string
			TaskType string
			Count    int64
		}
		costToTechDrillWhere := fmt.Sprintf(`received_spk_at >= ? AND received_spk_at <= ? AND complete_wo IS NOT NULL AND (
			stage IN ('Done', 'Waiting For Verification', 'Closed', 'Solved Pending')
			OR (stage = 'Pending' AND sla_status = 'On Target Solved')
		)%s`, filterWhereStr)
		costToTechDrillArgs := append([]interface{}{firstOfMonth, lastOfMonth}, filterArgs...)
		sqlCostToTechDrill := `
		SELECT DATE_FORMAT(received_spk_at, '%d %b') as date, company, task_type, COUNT(*) as count
		FROM ` + table + `
		WHERE ` + costToTechDrillWhere + `
		GROUP BY DATE_FORMAT(received_spk_at, '%d %b'), company, task_type
	`
		dbWeb.Raw(sqlCostToTechDrill, costToTechDrillArgs...).Scan(&costToTechDrillRows) // Calculate cost to technicians drill data
		var costToTechDrillData []DrillItem
		for _, row := range costToTechDrillRows {
			calculatedValue := float64(row.Count) * defaultPriceforCost

			costToTechDrillData = append(costToTechDrillData, DrillItem{
				Date:        row.Date,
				Name:        fmt.Sprintf("%s - %s", row.Company, row.TaskType),
				Y:           calculatedValue,
				Count:       row.Count,
				Price:       defaultPriceforCost,
				Description: fmt.Sprintf("Company: %s | Task: %s | Tickets: %d | Cost per ticket: Rp %.0f | Total Cost: Rp %.0f", row.Company, row.TaskType, row.Count, defaultPriceforCost, calculatedValue),
			})
		}

		// Get Cost of Penalty drill data: distinct date, company and task_type with count
		var costPenaltyDrillRows []struct {
			Date     string
			Company  string
			TaskType string
			Count    int64
		}
		costPenaltyDrillWhere := fmt.Sprintf(`received_spk_at >= ? AND received_spk_at <= ? AND stage = 'New' AND (
		sla_status = 'Overdue (New)' OR sla_status = 'Overdue (Visited)'
	)%s`, filterWhereStr)
		costPenaltyDrillArgs := append([]interface{}{firstOfMonth, lastOfMonth}, filterArgs...)
		sqlCostPenaltyDrill := `
		SELECT DATE_FORMAT(received_spk_at, '%d %b') as date, company, task_type, COUNT(*) as count
		FROM ` + table + `
		WHERE ` + costPenaltyDrillWhere + `
		GROUP BY DATE_FORMAT(received_spk_at, '%d %b'), company, task_type
	`
		dbWeb.Raw(sqlCostPenaltyDrill, costPenaltyDrillArgs...).Scan(&costPenaltyDrillRows) // Calculate cost of penalty drill data
		var costPenaltyDrillData []DrillItem
		for _, row := range costPenaltyDrillRows {
			calculatedValue := float64(row.Count) * defaultPenaltyPrice

			costPenaltyDrillData = append(costPenaltyDrillData, DrillItem{
				Date:        row.Date,
				Name:        fmt.Sprintf("%s - %s", row.Company, row.TaskType),
				Y:           calculatedValue,
				Count:       row.Count,
				Price:       defaultPenaltyPrice,
				Description: fmt.Sprintf("Company: %s | Task: %s | Tickets: %d | Penalty per ticket: Rp %.0f | Total Penalty: Rp %.0f", row.Company, row.TaskType, row.Count, defaultPenaltyPrice, calculatedValue),
			})
		} // Get last update time
		var lastUpdatedData struct{ LastUpdate time.Time }
		if err := dbWeb.Table(table).
			Select("MAX(updated_at) as last_update").
			Scan(&lastUpdatedData).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{
				"success": false,
				"error":   err.Error(),
			})
			return
		}

		c.JSON(http.StatusOK, gin.H{
			"success":     true,
			"data":        chartData,
			"chart_title": fmt.Sprintf("COST vs REVENUE %v", strings.ToUpper(firstOfMonth.Format("January 2006"))),
			"last_update": lastUpdatedData.LastUpdate.Format("Monday, 02 January 2006 15:04 PM"),
			"drill_data": gin.H{
				"revenue_drill":      revenueDrillData,
				"cost_to_tech_drill": costToTechDrillData,
				"cost_penalty_drill": costPenaltyDrillData,
			},
			"summary": gin.H{
				"cost_to_technicians_price": defaultPriceforCost,
				"cost_penalty_price":        defaultPenaltyPrice,
				"total_revenue_count":       cumRevenueCount,
				"total_cost_to_tech_count":  cumCostToTechCount,
				"total_cost_penalty_count":  cumCostPenaltyCount,
				"total_revenue_price":       cumRevenuePrice,
				"total_cost_to_tech_price":  cumCostToTechPrice,
				"total_cost_penalty_price":  cumCostPenaltyPrice,
				"total_cost_price":          cumCostToTechPrice + cumCostPenaltyPrice,
				"total_profit":              cumRevenuePrice - (cumCostToTechPrice + cumCostPenaltyPrice),
			},
		})
	}
}

func GetDataDateRangeTicketPerformance() gin.HandlerFunc {
	return func(c *gin.Context) {
		var dateRanges []string
		dbWeb := gormdb.Databases.Web

		table := config.GetConfig().Database.TbReportMonitoringTicket
		currentYear := time.Now().Year()

		// Check tables from 2024 to current year (or extend range as needed)
		// Loop backwards from current year to show newest data first
		for year := currentYear; year >= 2024; year-- {
			// Loop through all months
			for month := 12; month >= 1; month-- {
				// Create table name with month suffix (e.g., "_sep2025")
				monthName := time.Month(month).String()[:3] // Get first 3 letters (Sep, Oct, etc.)
				tableName := fmt.Sprintf("%s_%s%d", table, strings.ToLower(monthName), year)

				// Check if table exists by trying to query it
				var count int64
				if err := dbWeb.Table(tableName).Limit(1).Count(&count).Error; err == nil {
					// Table exists, add to date ranges
					dateRange := fmt.Sprintf("%s %d", monthName, year)
					dateRanges = append(dateRanges, dateRange)
				}
			}
		}

		c.JSON(http.StatusOK, gin.H{
			"success": true,
			"data":    dateRanges,
		})
	}
}

func GetDataSACTicketPerformance() gin.HandlerFunc {
	return func(c *gin.Context) {
		sacData := config.GetConfig().ODOOMSSAC
		sacList := []string{}
		for sac := range sacData {
			sacList = append(sacList, sac)
		}

		c.JSON(http.StatusOK, gin.H{
			"success": true,
			"data":    sacList,
		})
	}
}

func GetDataSPLTicketPerformance() gin.HandlerFunc {
	return func(c *gin.Context) {
		odooMSTech := TechODOOMSData
		if len(odooMSTech) == 0 {
			c.JSON(http.StatusNotFound, gin.H{
				"success": false,
				"data":    []string{},
				"error":   "No technician data available",
			})
			return
		}

		// Accept both GET query params and POST JSON body
		var sacList []string

		// Try POST body first
		var requestBody struct {
			SAC []string `json:"sac"`
		}
		if err := c.ShouldBindJSON(&requestBody); err == nil && len(requestBody.SAC) > 0 {
			sacList = requestBody.SAC
		} else {
			// Fallback to query parameter for backward compatibility
			sacName := c.Query("sac")
			if sacName != "" {
				sacList = strings.Split(sacName, ",")
				for i := range sacList {
					sacList[i] = strings.TrimSpace(sacList[i])
				}
			}
		}

		var spls []string
		if len(sacList) == 0 {
			// No SAC filter, return all SPLs
			for _, techData := range odooMSTech {
				spls = append(spls, techData.SPL)
			}
		} else {
			// Filter SPLs by SAC list
			for _, techData := range odooMSTech {
				for _, sac := range sacList {
					if techData.SAC == sac {
						spls = append(spls, techData.SPL)
						break
					}
				}
			}
		}

		// Remove duplicate SPLs
		uniqueSPLs := make(map[string]struct{})
		var result []string
		for _, spl := range spls {
			if _, exists := uniqueSPLs[spl]; !exists && spl != "" {
				uniqueSPLs[spl] = struct{}{}
				result = append(result, spl)
			}
		}
		sort.Strings(result)

		if len(spls) == 0 {
			c.JSON(http.StatusNotFound, gin.H{
				"success": false,
				"data":    []string{},
				"error":   "No SPL data available",
			})
			return
		}

		c.JSON(http.StatusOK, gin.H{
			"success": true,
			"data":    result,
		})
	}
}

func GetDataTechnicianTicketPerformance() gin.HandlerFunc {
	return func(c *gin.Context) {
		odooMSTech := TechODOOMSData

		if len(odooMSTech) == 0 {
			c.JSON(http.StatusNotFound, gin.H{
				"success": false,
				"data":    []string{},
				"error":   "No technician data available",
			})
			return
		}

		// Accept both GET query params and POST JSON body
		var sacList []string
		var splList []string

		// Try POST body first
		var requestBody struct {
			SAC []string `json:"sac"`
			SPL []string `json:"spl"`
		}
		if err := c.ShouldBindJSON(&requestBody); err == nil {
			sacList = requestBody.SAC
			splList = requestBody.SPL
		} else {
			// Fallback to query parameters for backward compatibility
			sacName := c.Query("sac")
			if sacName != "" {
				sacList = strings.Split(sacName, ",")
				for i := range sacList {
					sacList[i] = strings.TrimSpace(sacList[i])
				}
			}
			splName := c.Query("spl")
			if splName != "" {
				splList = strings.Split(splName, ",")
				for i := range splList {
					splList[i] = strings.TrimSpace(splList[i])
				}
			}
		}

		var technicians []string
		if len(sacList) == 0 && len(splList) == 0 {
			// No filters, return all technicians
			for technician := range odooMSTech {
				technicians = append(technicians, technician)
			}
		} else {
			// Filter technicians by SAC and SPL lists
			for technician, techData := range odooMSTech {
				matchSAC := len(sacList) == 0
				matchSPL := len(splList) == 0

				// Check if technician matches any selected SAC
				for _, sac := range sacList {
					if techData.SAC == sac {
						matchSAC = true
						break
					}
				}
				// Check if technician matches any selected SPL
				for _, spl := range splList {
					if techData.SPL == spl {
						matchSPL = true
						break
					}
				}
				// Add technician if matches both SAC and SPL filters (AND logic)
				if (len(sacList) == 0 || matchSAC) && (len(splList) == 0 || matchSPL) {
					technicians = append(technicians, technician)
				}
			}
		}

		// Remove duplicate technicians
		uniqueTechs := make(map[string]struct{})
		var result []string
		for _, tech := range technicians {
			if _, exists := uniqueTechs[tech]; !exists && tech != "" {
				uniqueTechs[tech] = struct{}{}
				result = append(result, tech)
			}
		}
		sort.Strings(result)

		if len(technicians) == 0 {
			c.JSON(http.StatusNotFound, gin.H{
				"success": false,
				"data":    []string{},
				"error":   "No technician data available",
			})
			return
		}

		c.JSON(http.StatusOK, gin.H{
			"success": true,
			"data":    result,
		})
	}
}

func GetDataCompanyTicketPerformance() gin.HandlerFunc {
	return func(c *gin.Context) {
		dbWeb := gormdb.Databases.Web

		// Get data_date parameter
		dataDate := c.Query("data_date")

		var companies []string
		table := config.GetConfig().Database.TbReportMonitoringTicket

		// For current month (dataDate empty), use base table
		// For specific months, use monthly tables if they exist
		if dataDate != "" {
			// Parse selected data_date to get month and year
			parts := strings.Split(dataDate, " ")
			if len(parts) == 2 {
				var targetYear int
				var targetMonth time.Month
				if parsedYear, err := strconv.Atoi(parts[1]); err == nil {
					targetYear = parsedYear
				}
				// Parse month name to month number
				for m := 1; m <= 12; m++ {
					if strings.EqualFold(time.Month(m).String()[:3], parts[0]) {
						targetMonth = time.Month(m)
						break
					}
				}
				if targetYear != 0 && targetMonth != 0 {
					// Construct table name with month suffix
					monthStr := strings.ToLower(targetMonth.String()[:3])
					yearStr := strconv.Itoa(targetYear)
					table = fmt.Sprintf("%s_%s%s", table, monthStr, yearStr)
				}
			}
		}

		result := dbWeb.Table(table).
			Distinct("company").
			Order("company ASC").
			Pluck("company", &companies)

		if result.Error != nil {
			c.JSON(http.StatusInternalServerError, gin.H{
				"success": false,
				"error":   result.Error.Error(),
			})
			return
		}

		if len(companies) == 0 {
			c.JSON(http.StatusNotFound, gin.H{
				"success": false,
				"data":    []string{},
				"error":   "No company data available",
			})
			return
		}

		c.JSON(http.StatusOK, gin.H{
			"success": true,
			"data":    companies,
		})
	}
}

func GetDataSLAStatusTicketPerformance() gin.HandlerFunc {
	return func(c *gin.Context) {
		dbWeb := gormdb.Databases.Web

		// Get data_date parameter
		dataDate := c.Query("data_date")

		var slaStatuses []string
		table := config.GetConfig().Database.TbReportMonitoringTicket

		// For current month (dataDate empty), use base table
		// For specific months, use monthly tables if they exist
		if dataDate != "" {
			// Parse selected data_date to get month and year
			parts := strings.Split(dataDate, " ")
			if len(parts) == 2 {
				var targetYear int
				var targetMonth time.Month
				if parsedYear, err := strconv.Atoi(parts[1]); err == nil {
					targetYear = parsedYear
				}
				// Parse month name to month number
				for m := 1; m <= 12; m++ {
					if strings.EqualFold(time.Month(m).String()[:3], parts[0]) {
						targetMonth = time.Month(m)
						break
					}
				}
				if targetYear != 0 && targetMonth != 0 {
					// Construct table name with month suffix
					monthStr := strings.ToLower(targetMonth.String()[:3])
					yearStr := strconv.Itoa(targetYear)
					table = fmt.Sprintf("%s_%s%s", table, monthStr, yearStr)
				}
			}
		}

		result := dbWeb.Table(table).
			Distinct("sla_status").
			Where("sla_status IS NOT NULL AND sla_status != ''").
			Order("sla_status ASC").
			Pluck("sla_status", &slaStatuses)

		if result.Error != nil {
			c.JSON(http.StatusInternalServerError, gin.H{
				"success": false,
				"error":   result.Error.Error(),
			})
			return
		}

		if len(slaStatuses) == 0 {
			c.JSON(http.StatusNotFound, gin.H{
				"success": false,
				"data":    []string{},
				"error":   "No SLA status data available",
			})
			return
		}

		c.JSON(http.StatusOK, gin.H{
			"success": true,
			"data":    slaStatuses,
		})
	}
}

func GetDataTaskTypeTicketPerformance() gin.HandlerFunc {
	return func(c *gin.Context) {
		dbWeb := gormdb.Databases.Web

		// Get data_date parameter
		dataDate := c.Query("data_date")

		var taskTypes []string
		table := config.GetConfig().Database.TbReportMonitoringTicket

		// For current month (dataDate empty), use base table
		// For specific months, use monthly tables if they exist
		if dataDate != "" {
			// Parse selected data_date to get month and year
			parts := strings.Split(dataDate, " ")
			if len(parts) == 2 {
				var targetYear int
				var targetMonth time.Month
				if parsedYear, err := strconv.Atoi(parts[1]); err == nil {
					targetYear = parsedYear
				}
				// Parse month name to month number
				for m := 1; m <= 12; m++ {
					if strings.EqualFold(time.Month(m).String()[:3], parts[0]) {
						targetMonth = time.Month(m)
						break
					}
				}
				if targetYear != 0 && targetMonth != 0 {
					// Construct table name with month suffix
					monthStr := strings.ToLower(targetMonth.String()[:3])
					yearStr := strconv.Itoa(targetYear)
					table = fmt.Sprintf("%s_%s%s", table, monthStr, yearStr)
				}
			}
		}

		result := dbWeb.Table(table).
			Distinct("task_type").
			Where("task_type IS NOT NULL AND task_type != ''").
			Order("task_type ASC").
			Pluck("task_type", &taskTypes)

		if result.Error != nil {
			c.JSON(http.StatusInternalServerError, gin.H{
				"success": false,
				"error":   result.Error.Error(),
			})
			return
		}

		if len(taskTypes) == 0 {
			c.JSON(http.StatusNotFound, gin.H{
				"success": false,
				"data":    []string{},
				"error":   "No task type data available",
			})
			return
		}

		// Add "Non-PM" option at the beginning
		taskTypesWithNonPM := append([]string{"Non-PM"}, taskTypes...)

		c.JSON(http.StatusOK, gin.H{
			"success": true,
			"data":    taskTypesWithNonPM,
		})
	}
}

func DownloadReportMasterTicketPerformance() gin.HandlerFunc {
	return func(c *gin.Context) {
		loc, _ := time.LoadLocation(config.GetConfig().Default.Timezone)
		now := time.Now().In(loc)

		reportMainDir, err := fun.FindValidDirectory([]string{
			"web/file/monitoring_ticket",
			"../web/file/monitoring_ticket",
			"../../web/file/monitoring_ticket",
		})
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{
				"success": false,
				"error":   "Report directory not found",
			})
			return
		}

		dateDir := now.Format("2006-01-02")
		targetDir := filepath.Join(reportMainDir, dateDir)
		files, err := filepath.Glob(filepath.Join(targetDir, "*.xlsx"))
		if err != nil || len(files) == 0 {
			c.JSON(http.StatusNotFound, gin.H{
				"success": false,
				"error":   "No report file found for today",
			})
			return
		}

		// Only consider files matching Monitoring_Ticket_DDMonYYYY.xlsx (date in 02Jan2006 format)
		var matchedFiles []string
		// Regex: Monitoring_Ticket_\d{2}[A-Z][a-z]{2}\d{4}\.xlsx
		pattern := `^Monitoring_Ticket_\d{2}[A-Z][a-z]{2}\d{4}\.xlsx$`
		re := regexp.MustCompile(pattern)
		for _, f := range files {
			base := filepath.Base(f)
			if re.MatchString(base) {
				matchedFiles = append(matchedFiles, f)
			}
		}
		if len(matchedFiles) == 0 {
			c.JSON(http.StatusNotFound, gin.H{
				"success": false,
				"error":   "No valid report file found (pattern not matched)",
			})
			return
		}

		// Find the latest file by mod time among matchedFiles
		var latestFile string
		var latestModTime int64
		for _, f := range matchedFiles {
			info, err := os.Stat(f)
			if err == nil && info.ModTime().Unix() > latestModTime {
				latestFile = f
				latestModTime = info.ModTime().Unix()
			}
		}
		if latestFile == "" {
			c.JSON(http.StatusNotFound, gin.H{
				"success": false,
				"error":   "No valid report file found (stat error)",
			})
			return
		}

		c.FileAttachment(latestFile, filepath.Base(latestFile))
	}
}

func DownloadReportFilteredTicketPerformance() gin.HandlerFunc {
	return func(c *gin.Context) {
		dbWeb := gormdb.Databases.Web

		// Read filter values from POST form data
		dataDate := c.PostForm("data_date")
		sac := c.PostForm("sac")
		spl := c.PostForm("spl")
		technician := c.PostForm("technician")
		company := c.PostForm("company")
		slaStatus := c.PostForm("sla_status")
		taskType := c.PostForm("task_type")

		// Debug logging to see received filters
		logrus.Infof("Received filters - data_date: '%s', sac: '%s', spl: '%s', technician: '%s', company: '%s', sla_status: '%s', task_type: '%s'",
			dataDate, sac, spl, technician, company, slaStatus, taskType)

		// Parse comma-separated values for multiple selections
		sacList := []string{}
		if sac != "" {
			for _, item := range strings.Split(sac, ",") {
				sacList = append(sacList, strings.TrimSpace(item))
			}
		}
		splList := []string{}
		if spl != "" {
			for _, item := range strings.Split(spl, ",") {
				splList = append(splList, strings.TrimSpace(item))
			}
		}
		technicianList := []string{}
		if technician != "" {
			for _, item := range strings.Split(technician, ",") {
				technicianList = append(technicianList, strings.TrimSpace(item))
			}
		}
		companyList := []string{}
		if company != "" {
			for _, item := range strings.Split(company, ",") {
				companyList = append(companyList, strings.TrimSpace(item))
			}
		}
		slaStatusList := []string{}
		if slaStatus != "" {
			for _, item := range strings.Split(slaStatus, ",") {
				slaStatusList = append(slaStatusList, strings.TrimSpace(item))
			}
		}
		taskTypeList := []string{}
		isNonPMSelected := false
		if taskType != "" {
			for _, item := range strings.Split(taskType, ",") {
				taskTypeList = append(taskTypeList, strings.TrimSpace(item))
				if item == "Non-PM" {
					isNonPMSelected = true
				}
			}
		}

		table := config.GetConfig().Database.TbReportMonitoringTicket
		if dataDate != "" {
			table = fmt.Sprintf("%s_%s", table, strings.ToLower(strings.ReplaceAll(dataDate, " ", "")))
		}

		// Build dynamic WHERE clause for filters
		var filterWhere []string
		var filterArgs []interface{}

		// Build technician list based on SAC/SPL/Technician filters
		technicianFilterList := []string{}
		if len(sacList) > 0 || len(splList) > 0 {
			odooMSTech := TechODOOMSData
			logrus.Infof("TechODOOMSData has %d technicians available for SAC/SPL filtering", len(odooMSTech))
			for _, tech := range odooMSTech {
				// Check SAC filter
				if len(sacList) > 0 {
					sacMatch := false
					for _, filterSAC := range sacList {
						if tech.SAC == filterSAC {
							sacMatch = true
							break
						}
					}
					if !sacMatch {
						continue
					}
				}

				// Check SPL filter
				if len(splList) > 0 {
					splMatch := false
					for _, filterSPL := range splList {
						if tech.SPL == filterSPL {
							splMatch = true
							break
						}
					}
					if !splMatch {
						continue
					}
				}

				technicianFilterList = append(technicianFilterList, tech.Name)
				logrus.Infof("Added technician '%s' (SAC: %s, SPL: %s) to filter list", tech.Name, tech.SAC, tech.SPL)
			}
		}

		// If specific technicians are selected, use only those
		if len(technicianList) > 0 {
			technicianFilterList = technicianList
			logrus.Infof("Using directly selected technicians: %v", technicianList)
		}

		logrus.Infof("Final technician filter list has %d technicians: %v", len(technicianFilterList), technicianFilterList)

		if len(technicianFilterList) > 0 {
			placeholders := make([]string, len(technicianFilterList))
			for i := range placeholders {
				placeholders[i] = "?"
			}
			filterWhere = append(filterWhere, fmt.Sprintf("technician IN (%s)", strings.Join(placeholders, ",")))
			for _, tech := range technicianFilterList {
				filterArgs = append(filterArgs, tech)
			}
		}

		if len(companyList) > 0 {
			placeholders := make([]string, len(companyList))
			for i := range placeholders {
				placeholders[i] = "?"
			}
			filterWhere = append(filterWhere, fmt.Sprintf("company IN (%s)", strings.Join(placeholders, ",")))
			for _, comp := range companyList {
				filterArgs = append(filterArgs, comp)
			}
		}

		if len(slaStatusList) > 0 {
			placeholders := make([]string, len(slaStatusList))
			for i := range placeholders {
				placeholders[i] = "?"
			}
			filterWhere = append(filterWhere, fmt.Sprintf("sla_status IN (%s)", strings.Join(placeholders, ",")))
			for _, status := range slaStatusList {
				filterArgs = append(filterArgs, status)
			}
		}

		// Task Type filter
		if len(taskTypeList) > 0 {
			if isNonPMSelected {
				// If Non-PM is selected, filter task_type != 'Preventive Maintenance'
				filterWhere = append(filterWhere, "task_type != ?")
				filterArgs = append(filterArgs, "Preventive Maintenance")
			} else {
				// Regular filter for specific task types
				placeholders := make([]string, len(taskTypeList))
				for i := range placeholders {
					placeholders[i] = "?"
				}
				filterWhere = append(filterWhere, fmt.Sprintf("task_type IN (%s)", strings.Join(placeholders, ",")))
				for _, tt := range taskTypeList {
					filterArgs = append(filterArgs, tt)
				}
			}
		}

		// Query filtered data from database
		var countData int64
		countQuery := dbWeb.Table(table)
		if len(filterWhere) > 0 {
			countQuery = countQuery.Where(strings.Join(filterWhere, " AND "), filterArgs...)
			logrus.Infof("Applied filters: %s with args: %v", strings.Join(filterWhere, " AND "), filterArgs)
		} else {
			logrus.Infof("No filters applied, querying all data from table: %s", table)
		}
		countQuery.Count(&countData)

		logrus.Infof("QUERY COUNT RESULT: Found %d total records from table '%s' with applied filters", countData, table)

		if countData == 0 {
			filterDetails := "No filters applied"
			var filterParts []string
			if dataDate != "" {
				filterParts = append(filterParts, fmt.Sprintf("data_date: %s", dataDate))
			}
			if sac != "" {
				filterParts = append(filterParts, fmt.Sprintf("sac: %s", sac))
			}
			if spl != "" {
				filterParts = append(filterParts, fmt.Sprintf("spl: %s", spl))
			}
			if technician != "" {
				filterParts = append(filterParts, fmt.Sprintf("technician: %s", technician))
			}
			if company != "" {
				filterParts = append(filterParts, fmt.Sprintf("company: %s", company))
			}
			if slaStatus != "" {
				filterParts = append(filterParts, fmt.Sprintf("sla_status: %s", slaStatus))
			}
			if len(filterParts) > 0 {
				filterDetails = fmt.Sprintf("Applied filters: %s", strings.Join(filterParts, ". "))
			}
			logrus.Warnf("No data found for filters. Table: %s, Filter count: %d, Filters: %s", table, len(filterWhere), filterDetails)
			c.JSON(http.StatusNotFound, gin.H{
				"success": false,
				"error":   fmt.Sprintf("No data found for the selected filters: %s", filterDetails),
			})
			return
		}

		logrus.Infof("Found %d records matching filters", countData)

		// Create Excel file using excelize
		f := excelize.NewFile()

		sheetEmployee := "EMPLOYEES"
		sheetMaster := "FILTERED"
		sheetPivot := "PIVOT"
		f.NewSheet(sheetEmployee)
		sheetMasterIndex, err := f.NewSheet(sheetMaster)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{
				"success": false,
				"error":   "Failed to create Excel sheet: " + err.Error(),
			})
			return
		}
		f.NewSheet(sheetPivot)

		// Create header style
		headerStyle, err := f.NewStyle(&excelize.Style{
			Alignment: &excelize.Alignment{
				Horizontal: "center",
				Vertical:   "center",
			},
			Fill: excelize.Fill{
				Type:    "pattern",
				Color:   []string{"#4472C4"},
				Pattern: 1,
			},
			Font: &excelize.Font{
				Bold:  true,
				Color: "#FFFFFF",
			},
		})
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{
				"success": false,
				"error":   "Failed to create header style: " + err.Error(),
			})
			return
		}

		// Create body cell style
		style, err := f.NewStyle(&excelize.Style{
			Alignment: &excelize.Alignment{
				Horizontal: "left",
				Vertical:   "center",
			},
		})
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{
				"success": false,
				"error":   "Failed to create body style: " + err.Error(),
			})
			return
		}

		// Create employees sheet for technician lookup
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
			f.SetCellStyle(sheetEmployee, cell, cell, style)
		}

		lastColEmployee := fun.GetColName(len(columnsEmployee) - 1)
		filterRangeEmployee := fmt.Sprintf("A1:%s1", lastColEmployee)
		f.AutoFilter(sheetEmployee, filterRangeEmployee, []excelize.AutoFilterOptions{})

		// Populate employee data
		employeeRow := 2
		for technician, data := range TechODOOMSData {
			for _, column := range columnsEmployee {
				cell := fmt.Sprintf("%s%d", column.ColIndex, employeeRow)
				var value string
				switch column.ColTitle {
				case "Technician":
					value = technician
				case "SPL":
					value = data.SPL
				case "SAC":
					value = data.SAC
				}
				f.SetCellValue(sheetEmployee, cell, value)
				f.SetCellStyle(sheetEmployee, cell, cell, style)
			}
			employeeRow++
		}

		// Memory stats for monitoring performance
		var memStats runtime.MemStats
		runtime.ReadMemStats(&memStats)
		startTime := time.Now()

		// Total count for batching
		totalCount := countData

		logrus.Infof("Starting Excel generation for %d records with batched writing optimization", totalCount)

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
			f.SetCellStyle(sheetMaster, cell, cell, headerStyle)
		}

		lastColMaster := fun.GetColName(len(columnsMaster) - 1)
		filterRangeMaster := fmt.Sprintf("A1:%s1", lastColMaster)
		f.AutoFilter(sheetMaster, filterRangeMaster, []excelize.AutoFilterOptions{})

		// Find column indices for formulas
		var technicianColIndex, completeWOColIndex, receivedDateSPKColIndex string
		for _, col := range columnsMaster {
			switch col.ColTitle {
			case "Technician":
				technicianColIndex = col.ColIndex
			case "Complete WO":
				completeWOColIndex = col.ColIndex
			case "Received SPK at":
				receivedDateSPKColIndex = col.ColIndex
			}
		}

		rowIndex := 2

		const fetchBatchSize = 1000
		if countData > 0 {
			for offset := 0; offset < int(totalCount); offset += fetchBatchSize {
				var batchData []reportmodel.MonitoringTicketODOOMS

				dataQuery := dbWeb.Table(table)
				if len(filterWhere) > 0 {
					dataQuery = dataQuery.Where(strings.Join(filterWhere, " AND "), filterArgs...)
				}
				if err := dataQuery.
					Offset(offset).
					Limit(fetchBatchSize).
					Order("CASE WHEN task_type = 'Preventive Maintenance' THEN 0 ELSE 1 END, received_spk_at asc").
					Find(&batchData).Error; err != nil {
					c.JSON(http.StatusInternalServerError, gin.H{
						"success": false,
						"error":   fmt.Sprintf("Failed to fetch batch at offset %d: %v", offset, err),
					})
					return
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

				// Prepare batch data for bulk writing
				batchRows := make([][]interface{}, 0, len(batchData))
				formulaCells := make(map[string]string) // cell -> formula mapping
				linkCells := make(map[string]string)    // cell -> link mapping

				// Process each record in the batch and prepare row data
				for _, record := range batchData {
					rowData := make([]interface{}, len(columnsMaster))

					for colIdx, column := range columnsMaster {
						var value interface{} = "N/A"

						switch column.ColTitle {
						case "SAC":
							// Will be handled as formula after batch write
							formula := fmt.Sprintf(`IFERROR(VLOOKUP(%s%d, %v!A:C, 3, FALSE), "N/A")`, technicianColIndex, rowIndex, sheetEmployee)
							formulaCells[fmt.Sprintf("%s%d", column.ColIndex, rowIndex)] = formula
							value = "N/A" // placeholder
						case "SPL":
							// Will be handled as formula after batch write
							formula := fmt.Sprintf(`IFERROR(VLOOKUP(%s%d, %v!A:C, 2, FALSE), "N/A")`, technicianColIndex, rowIndex, sheetEmployee)
							formulaCells[fmt.Sprintf("%s%d", column.ColIndex, rowIndex)] = formula
							value = "N/A" // placeholder
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
								linkCells[fmt.Sprintf("%s%d", column.ColIndex, rowIndex)] = record.LinkWO
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
							if record.CompleteWO != nil && !record.CompleteWO.IsZero() {
								formula := fmt.Sprintf(`=IFERROR(DAY(%s%d), "N/A")`, completeWOColIndex, rowIndex)
								formulaCells[fmt.Sprintf("%s%d", column.ColIndex, rowIndex)] = formula
								value = "N/A" // placeholder
							} else {
								value = "N/A"
							}
						case "Day SPK":
							if record.ReceivedSPKAt != nil && !record.ReceivedSPKAt.IsZero() {
								formula := fmt.Sprintf(`=IFERROR(DAY(%s%d), "N/A")`, receivedDateSPKColIndex, rowIndex)
								formulaCells[fmt.Sprintf("%s%d", column.ColIndex, rowIndex)] = formula
								value = "N/A" // placeholder
							} else {
								value = "N/A"
							}
						case "Item":
							if strings.TrimSpace(record.TaskType) == "Preventive Maintenance" {
								value = "PM - " + record.Stage
							} else {
								value = "Non-PM " + record.Stage
							}
						}

						rowData[colIdx] = value
					}

					batchRows = append(batchRows, rowData)
					rowIndex++
				}

				// Write all batch rows at once using SetSheetRow (much faster than individual cell writes)
				startRowForBatch := rowIndex - len(batchData)
				for i, rowData := range batchRows {
					currentRow := startRowForBatch + i
					if err := f.SetSheetRow(sheetMaster, fmt.Sprintf("A%d", currentRow), &rowData); err != nil {
						logrus.Errorf("Failed to set row %d: %v", currentRow, err)
					}
				}

				// Apply formulas after batch write (formulas need individual cell operations)
				for cell, formula := range formulaCells {
					f.SetCellFormula(sheetMaster, cell, formula)
				}

				// Apply hyperlinks after batch write
				for cell, link := range linkCells {
					f.SetCellHyperLink(sheetMaster, cell, link, "External")
				}

				// Apply styles to the batch range (faster than individual cell styling)
				if len(batchRows) > 0 {
					batchStartRow := startRowForBatch
					batchEndRow := rowIndex - 1

					// Apply style to the entire range at once
					cells, err := excelize.CoordinatesToCellName(1, batchStartRow)
					if err == nil {
						endCell, err := excelize.CoordinatesToCellName(len(columnsMaster), batchEndRow)
						if err == nil {
							f.SetCellStyle(sheetMaster, cells, endCell, style)
						}
					}
				}

				// Memory optimization: Force garbage collection and log memory stats every 5 batches
				if (offset/fetchBatchSize+1)%5 == 0 {
					runtime.GC()
					runtime.ReadMemStats(&memStats)
					logrus.Infof("Memory stats after batch %d: Alloc = %v MiB, TotalAlloc = %v MiB, Sys = %v MiB, NumGC = %v",
						(offset/fetchBatchSize)+1,
						memStats.Alloc/1024/1024,
						memStats.TotalAlloc/1024/1024,
						memStats.Sys/1024/1024,
						memStats.NumGC)
				}
			}
		}

		/* PIVOT */
		pvtMasterRange := fmt.Sprintf("%s!$A$1:$%s$%d", sheetMaster, lastColMaster, rowIndex-1)
		pvtRange := fmt.Sprintf("%s!A8:AB20", sheetPivot)
		err = f.AddPivotTable(&excelize.PivotTableOptions{
			Name:            sheetPivot,
			DataRange:       pvtMasterRange,
			PivotTableRange: pvtRange,
			Rows: []excelize.PivotTableField{
				{Data: "Stage"},
			},
			Columns: []excelize.PivotTableField{
				{Data: "Day SPK"},
			},
			Data: []excelize.PivotTableField{
				{Data: "Complete WO", Subtotal: "count", Name: "Total Completed JO"},
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
		f.SetColWidth(sheetPivot, "A", "A", 30)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{
				"success": false,
				"error":   "Failed to create pivot table: " + err.Error(),
			})
			return
		}

		// Create temporary file with proper extension
		tempFile, err := os.CreateTemp("", "filtered_ticket_performance_*")
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{
				"success": false,
				"error":   "Failed to create temp file: " + err.Error(),
			})
			return
		}
		tempFile.Close() // Close the file handle so excelize can write to it

		// Create the actual Excel file path with .xlsx extension
		tempExcelPath := tempFile.Name() + ".xlsx"
		defer os.Remove(tempFile.Name()) // Clean up temp file after response
		defer os.Remove(tempExcelPath)   // Clean up Excel file after response

		// Set active sheet and save
		f.DeleteSheet("Sheet1")
		f.SetActiveSheet(sheetMasterIndex)

		// Log completion time
		runtime.ReadMemStats(&memStats)
		elapsedTime := time.Since(startTime)
		logrus.Infof("Excel generation completed in %v. Final memory: Alloc = %v MiB, TotalAlloc = %v MiB, Sys = %v MiB",
			elapsedTime,
			memStats.Alloc/1024/1024,
			memStats.TotalAlloc/1024/1024,
			memStats.Sys/1024/1024)

		// Save to temp file
		if err := f.SaveAs(tempExcelPath); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{
				"success": false,
				"error":   "Failed to save Excel file: " + err.Error(),
			})
			return
		}

		// Generate filename with current date and filter info
		loc, _ := time.LoadLocation(config.GetConfig().Default.Timezone)
		now := time.Now().In(loc)
		filename := fmt.Sprintf("(Filtered)Ticket_Achievements_%s.xlsx", now.Format("02Jan2006_150405"))

		// Set headers for file download
		c.Header("Content-Description", "File Transfer")
		c.Header("Content-Disposition", fmt.Sprintf("attachment; filename=%s", filename))
		c.Header("Content-Type", "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet")
		c.Header("Content-Transfer-Encoding", "binary")

		// Serve the file
		c.File(tempExcelPath)
	}
}

func GetLoginVisitTechnicianChart() gin.HandlerFunc {
	return func(c *gin.Context) {
		importPath := config.GetConfig().App.Logo
		newLogoPath := importPath[:len(importPath)-len(filepath.Base(importPath))] + "csna.png"
		c.HTML(http.StatusOK, "tab-login-visit-technicians.html", gin.H{
			"GLOBAL_URL": fun.GLOBAL_URL,
			"APP_LOGO":   newLogoPath,
			"ACCESS":     true,
		})
	}
}

func GetDataSACLoginVisitTechnician() gin.HandlerFunc {
	return func(c *gin.Context) {
		dbWeb := gormdb.Databases.Web
		var sacList []string
		if err := dbWeb.Model(&odooms.ODOOMSTechnicianData{}).
			Distinct("sac").
			Order("sac ASC").
			Pluck("sac", &sacList).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{
				"success": false,
				"error":   err.Error(),
			})
			return
		}

		if len(sacList) == 0 {
			c.JSON(http.StatusNotFound, gin.H{
				"success": false,
				"data":    []string{},
				"error":   "No SAC data available",
			})
			return
		}

		c.JSON(http.StatusOK, gin.H{
			"success": true,
			"data":    sacList,
		})
	}
}

func GetDataSPLLoginVisitTechnician() gin.HandlerFunc {
	return func(c *gin.Context) {
		sacName := c.Query("sac")
		dbWeb := gormdb.Databases.Web
		var spls []string
		query := dbWeb.Model(&odooms.ODOOMSTechnicianData{})
		if sacName != "" {
			query = query.Where("sac = ?", sacName)
		}
		if err := query.Distinct("spl").Order("spl ASC").Pluck("spl", &spls).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{
				"success": false,
				"error":   err.Error(),
			})
			return
		}

		c.JSON(http.StatusOK, gin.H{
			"success": true,
			"data":    spls,
		})
	}
}

func GetDataTechnicianLoginVisitTechnician() gin.HandlerFunc {
	return func(c *gin.Context) {
		sacName := c.Query("sac")
		splName := c.Query("spl")
		dbWeb := gormdb.Databases.Web
		var technicians []string
		query := dbWeb.Model(&odooms.ODOOMSTechnicianData{})
		if sacName != "" {
			query = query.Where("sac = ?", sacName)
		}
		if splName != "" {
			query = query.Where("spl = ?", splName)
		}
		if err := query.Distinct("technician").Order("technician ASC").Pluck("technician", &technicians).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{
				"success": false,
				"error":   err.Error(),
			})
			return
		}
		if len(technicians) == 0 {
			c.JSON(http.StatusNotFound, gin.H{
				"success": false,
				"data":    []string{},
				"error":   "No technician data available",
			})
			return
		}

		c.JSON(http.StatusOK, gin.H{
			"success": true,
			"data":    technicians,
		})
	}
}

// Route to serve chart data as JSON
func GetDataLoginVisitTechnician() gin.HandlerFunc {
	return func(c *gin.Context) {
		dbWeb := gormdb.Databases.Web

		type DaysWork struct {
			IsWeekends bool     `json:"is_weekends"`
			IsHoliday  bool     `json:"is_holiday"`
			Holiday    []string `json:"holiday"`
			Date       string   `json:"date"`
		}

		type ChartRow struct {
			Date                   DaysWork `json:"date"`
			TotalActiveTechnicians int64    `json:"Total_Active_Technicians"`
			TotalLoginTechnicians  int64    `json:"Total_Login_Technicians"`
			TotalVisitTechnicians  int64    `json:"Total_Visit_Technicians"`
			TotalSP1Given          int64    `json:"Total_SP1_Given"`
			TotalSP2Given          int64    `json:"Total_SP2_Given"`
			TotalSP3Given          int64    `json:"Total_SP3_Given"`
			TotalSP1Replied        int64    `json:"Total_SP1_Replied"`
			TotalSP2Replied        int64    `json:"Total_SP2_Replied"`
			TotalSP3Replied        int64    `json:"Total_SP3_Replied"`
			CumulativeVisit        int64    `json:"Cumulative_Visit"`
			CumulativeSPGiven      int64    `json:"Cumulative_SP_Given"`
			CumulativeSPReplied    int64    `json:"Cumulative_SP_Replied"`
		}

		sac := c.PostForm("sac")
		spl := c.PostForm("spl")
		technician := c.PostForm("technician")
		if sac == "" || spl == "" || technician == "" {
			var jsonBody struct {
				SAC        string `json:"sac"`
				SPL        string `json:"spl"`
				Technician string `json:"technician"`
			}
			if err := c.ShouldBindJSON(&jsonBody); err == nil {
				if sac == "" {
					sac = jsonBody.SAC
				}
				if spl == "" {
					spl = jsonBody.SPL
				}
				if technician == "" {
					technician = jsonBody.Technician
				}
			}
		}

		// table variable not needed
		tableTechSPGiven := config.GetConfig().SPTechnician.TBTechGotSP
		tableSPLSPGiven := config.GetConfig().SPTechnician.TBSPLGotSP

		var filterWhere []string
		var filterArgs []interface{}
		var technicianList []string
		query := dbWeb.Model(&odooms.ODOOMSTechnicianData{})
		if sac != "" {
			query = query.Where("sac = ?", sac)
		}
		if spl != "" {
			query = query.Where("spl = ?", spl)
		}
		if technician != "" {
			query = query.Where("technician = ?", technician)
		}
		if err := query.Distinct("technician").Pluck("technician", &technicianList).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{
				"success": false,
				"error":   err.Error(),
			})
			return
		}

		if len(technicianList) > 0 {
			qs := strings.Repeat("?,", len(technicianList))
			qs = strings.TrimRight(qs, ",")
			filterWhere = append(filterWhere, fmt.Sprintf("technician IN (%s)", qs))
			for _, t := range technicianList {
				filterArgs = append(filterArgs, t)
			}
		}
		filterWhereStr := ""
		if len(filterWhere) > 0 {
			filterWhereStr = " AND " + strings.Join(filterWhere, " AND ")
		}

		// 1. Get all dates in current month (e.g. 01 Sep to end of month)
		now := time.Now()
		year, month := now.Year(), now.Month()
		firstOfMonth := time.Date(year, month, 1, 0, 0, 0, 0, now.Location())
		lastOfMonth := firstOfMonth.AddDate(0, 1, -1)
		daysInMonth := lastOfMonth.Day()

		holidays, _ := fun.GetHolidaysBasedOnYearAndMonth(year, int(month))
		holidayMap := make(map[string][]string)
		for _, h := range holidays {
			holidayMap[h.Date] = append(holidayMap[h.Date], h.Name)
		}

		chartData := make([]ChartRow, 0, daysInMonth)
		var cumulativeVisit, cumulativeSPGiven, cumulativeSPReplied int64

		for d := 1; d <= daysInMonth; d++ {
			dayDate := time.Date(year, month, d, 0, 0, 0, 0, now.Location())
			dateStr := fmt.Sprintf("%02d %s", d, month.String()[:3])
			isoDate := dayDate.Format("2006-01-02")
			isWeekend := dayDate.Weekday() == time.Saturday || dayDate.Weekday() == time.Sunday
			isHoliday := false
			holidayDesc := []string{}
			if desc, ok := holidayMap[isoDate]; ok {
				isHoliday = true
				holidayDesc = desc
			}

			var totalActive int64
			args := append([]interface{}{isoDate}, filterArgs...)
			dbWeb.Model(&odooms.ODOOMSTechnicianData{}).
				Where("DATE(created_at) = ?"+filterWhereStr, args...).
				Count(&totalActive)

			var totalLogin int64
			argsLogin := append([]interface{}{isoDate, isoDate}, filterArgs...)
			dbWeb.Model(&odooms.ODOOMSTechnicianData{}).
				Where("DATE(created_at) = ? AND DATE(last_login) = ?"+filterWhereStr, argsLogin...).
				Count(&totalLogin)

			var totalVisit int64
			argsVisit := append([]interface{}{isoDate, isoDate}, filterArgs...)
			dbWeb.Model(&odooms.ODOOMSTechnicianData{}).
				Where("DATE(created_at) = ? AND DATE(last_visit) = ?"+filterWhereStr, argsVisit...).
				Count(&totalVisit)
			cumulativeVisit += totalVisit

			var totalSP1Given, totalSP2Given, totalSP3Given int64
			argsSP1 := append([]interface{}{isoDate}, filterArgs...)
			dbWeb.Table(tableTechSPGiven).
				Where("DATE(created_at) = ? AND is_got_sp1 = true"+filterWhereStr, argsSP1...).
				Count(&totalSP1Given)
			dbWeb.Table(tableTechSPGiven).
				Where("DATE(created_at) = ? AND is_got_sp2 = true"+filterWhereStr, argsSP1...).
				Count(&totalSP2Given)
			dbWeb.Table(tableTechSPGiven).
				Where("DATE(created_at) = ? AND is_got_sp3 = true"+filterWhereStr, argsSP1...).
				Count(&totalSP3Given)
			var splSP1, splSP2, splSP3 int64
			dbWeb.Table(tableSPLSPGiven).
				Where("DATE(created_at) = ? AND is_got_sp1 = true"+filterWhereStr, argsSP1...).
				Count(&splSP1)
			dbWeb.Table(tableSPLSPGiven).
				Where("DATE(created_at) = ? AND is_got_sp2 = true"+filterWhereStr, argsSP1...).
				Count(&splSP2)
			dbWeb.Table(tableSPLSPGiven).
				Where("DATE(created_at) = ? AND is_got_sp3 = true"+filterWhereStr, argsSP1...).
				Count(&splSP3)
			totalSP1Given += splSP1
			totalSP2Given += splSP2
			totalSP3Given += splSP3
			cumulativeSPGiven += totalSP1Given + totalSP2Given + totalSP3Given

			// Get TechnicianGotSP IDs matching filters
			var techGotSPIDs []uint
			dbWeb.Model(&sptechnicianmodel.TechnicianGotSP{}).
				Where("DATE(created_at) = ?"+filterWhereStr, argsSP1...).Pluck("id", &techGotSPIDs)
			// Get SPLGotSP IDs matching filters
			var splGotSPIDs []uint
			dbWeb.Model(&sptechnicianmodel.SPLGotSP{}).
				Where("DATE(created_at) = ?"+filterWhereStr, argsSP1...).Pluck("id", &splGotSPIDs)

			var totalSP1Replied, totalSP2Replied, totalSP3Replied int64
			dbWeb.Model(&sptechnicianmodel.SPWhatsAppMessage{}).
				Where("DATE(created_at) = ? AND what_sp = 'SP_TECHNICIAN' AND number_of_sp = 1 AND technician_got_sp_id IN ? AND whatsapp_reply_text IS NOT NULL AND whatsapp_reply_text != ''", isoDate, techGotSPIDs).
				Count(&totalSP1Replied)
			dbWeb.Model(&sptechnicianmodel.SPWhatsAppMessage{}).
				Where("DATE(created_at) = ? AND what_sp = 'SP_TECHNICIAN' AND number_of_sp = 2 AND technician_got_sp_id IN ? AND whatsapp_reply_text IS NOT NULL AND whatsapp_reply_text != ''", isoDate, techGotSPIDs).
				Count(&totalSP2Replied)
			dbWeb.Model(&sptechnicianmodel.SPWhatsAppMessage{}).
				Where("DATE(created_at) = ? AND what_sp = 'SP_TECHNICIAN' AND number_of_sp = 3 AND technician_got_sp_id IN ? AND whatsapp_reply_text IS NOT NULL AND whatsapp_reply_text != ''", isoDate, techGotSPIDs).
				Count(&totalSP3Replied)
			var splSP1Replied, splSP2Replied, splSP3Replied int64
			dbWeb.Model(&sptechnicianmodel.SPWhatsAppMessage{}).
				Where("DATE(created_at) = ? AND what_sp = 'SP_SPL' AND number_of_sp = 1 AND spl_got_sp_id IN ? AND whatsapp_reply_text IS NOT NULL AND whatsapp_reply_text != ''", isoDate, splGotSPIDs).
				Count(&splSP1Replied)
			dbWeb.Model(&sptechnicianmodel.SPWhatsAppMessage{}).
				Where("DATE(created_at) = ? AND what_sp = 'SP_SPL' AND number_of_sp = 2 AND spl_got_sp_id IN ? AND whatsapp_reply_text IS NOT NULL AND whatsapp_reply_text != ''", isoDate, splGotSPIDs).
				Count(&splSP2Replied)
			dbWeb.Model(&sptechnicianmodel.SPWhatsAppMessage{}).
				Where("DATE(created_at) = ? AND what_sp = 'SP_SPL' AND number_of_sp = 3 AND spl_got_sp_id IN ? AND whatsapp_reply_text IS NOT NULL AND whatsapp_reply_text != ''", isoDate, splGotSPIDs).
				Count(&splSP3Replied)
			totalSP1Replied += splSP1Replied
			totalSP2Replied += splSP2Replied
			totalSP3Replied += splSP3Replied
			cumulativeSPReplied += totalSP1Replied + totalSP2Replied + totalSP3Replied

			chartData = append(chartData, ChartRow{
				Date: DaysWork{
					IsWeekends: isWeekend,
					IsHoliday:  isHoliday,
					Holiday:    holidayDesc,
					Date:       dateStr,
				},
				TotalActiveTechnicians: totalActive,
				TotalLoginTechnicians:  totalLogin,
				TotalVisitTechnicians:  totalVisit,
				TotalSP1Given:          totalSP1Given,
				TotalSP2Given:          totalSP2Given,
				TotalSP3Given:          totalSP3Given,
				TotalSP1Replied:        totalSP1Replied,
				TotalSP2Replied:        totalSP2Replied,
				TotalSP3Replied:        totalSP3Replied,
				CumulativeVisit:        cumulativeVisit,
				CumulativeSPGiven:      cumulativeSPGiven,
				CumulativeSPReplied:    cumulativeSPReplied,
			})
		}

		var lastUpdatedData struct{ LastUpdate time.Time }
		if err := dbWeb.Model(&odooms.ODOOMSTechnicianData{}).
			Select("MAX(updated_at) as last_update").
			Scan(&lastUpdatedData).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{
				"success": false,
				"error":   err.Error(),
			})
			return
		}

		c.JSON(http.StatusOK, gin.H{
			"success":     true,
			"data":        chartData,
			"chart_title": fmt.Sprintf("TECHNICIAN ATTENDANCE %v", strings.ToUpper(time.Now().Format("January 2006"))),
			"last_update": lastUpdatedData.LastUpdate.Format("Monday, 02 January 2006 15:04 PM"),
		})
	}
}

func DownloadReportMasterLoginVisitTechnician() gin.HandlerFunc {
	return func(c *gin.Context) {
		loc, _ := time.LoadLocation(config.GetConfig().Default.Timezone)
		now := time.Now().In(loc)

		reportMainDir, err := fun.FindValidDirectory([]string{
			"web/file/monitoring_ticket",
			"../web/file/monitoring_ticket",
			"../../web/file/monitoring_ticket",
		})
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{
				"success": false,
				"error":   "Report directory not found",
			})
			return
		}

		dateDir := now.Format("2006-01-02")
		targetDir := filepath.Join(reportMainDir, dateDir)
		files, err := filepath.Glob(filepath.Join(targetDir, "*.xlsx"))
		if err != nil || len(files) == 0 {
			c.JSON(http.StatusNotFound, gin.H{
				"success": false,
				"error":   "No report file found for today",
			})
			return
		}

		// Only consider files matching Monitoring_Visit_and_Login_Technician_DDMonYYYY.xlsx (date in 02Jan2006 format)
		var matchedFiles []string
		// Regex: Monitoring_Visit_and_Login_Technician_\d{2}[A-Z][a-z]{2}\d{4}\.xlsx
		pattern := `^Monitoring_Visit_and_Login_Technician_\d{2}[A-Z][a-z]{2}\d{4}\.xlsx$`
		re := regexp.MustCompile(pattern)
		for _, f := range files {
			base := filepath.Base(f)
			if re.MatchString(base) {
				matchedFiles = append(matchedFiles, f)
			}
		}
		if len(matchedFiles) == 0 {
			c.JSON(http.StatusNotFound, gin.H{
				"success": false,
				"error":   "No valid report file found (pattern not matched)",
			})
			return
		}

		// Find the latest file by mod time among matchedFiles
		var latestFile string
		var latestModTime int64
		for _, f := range matchedFiles {
			info, err := os.Stat(f)
			if err == nil && info.ModTime().Unix() > latestModTime {
				latestFile = f
				latestModTime = info.ModTime().Unix()
			}
		}
		if latestFile == "" {
			c.JSON(http.StatusNotFound, gin.H{
				"success": false,
				"error":   "No valid report file found (stat error)",
			})
			return
		}

		c.FileAttachment(latestFile, filepath.Base(latestFile))
	}
}

// HighchartsExportProxy proxies requests to Highcharts export server to avoid CORS issues
func HighchartsExportProxy() gin.HandlerFunc {
	return func(c *gin.Context) {
		// Handle preflight OPTIONS request
		if c.Request.Method == "OPTIONS" {
			c.Header("Access-Control-Allow-Origin", "*")
			c.Header("Access-Control-Allow-Methods", "POST, OPTIONS")
			c.Header("Access-Control-Allow-Headers", "Content-Type, Authorization")
			c.Status(http.StatusOK)
			return
		}

		// Get the request body - handle both JSON and form data
		var body []byte
		var err error

		contentType := c.GetHeader("Content-Type")
		if strings.Contains(contentType, "multipart/form-data") || strings.Contains(contentType, "application/x-www-form-urlencoded") {
			// For form data, read the raw body and forward it as-is
			body, err = io.ReadAll(c.Request.Body)
			if err != nil {
				c.JSON(http.StatusBadRequest, gin.H{
					"success": false,
					"error":   "Failed to read request body",
				})
				return
			}
		} else {
			// For raw JSON, we need to convert it to form data format
			jsonBody, err := io.ReadAll(c.Request.Body)
			if err != nil {
				c.JSON(http.StatusBadRequest, gin.H{
					"success": false,
					"error":   "Failed to read request body",
				})
				return
			}

			// Convert JSON to form data format that Highcharts expects
			// The export server expects 'options' field with the chart config
			formData := "options=" + string(jsonBody)
			body = []byte(formData)
		}

		// Create a new request to Highcharts export server
		exportURL := "https://export.highcharts.com/"
		req, err := http.NewRequest("POST", exportURL, bytes.NewBuffer(body))
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{
				"success": false,
				"error":   "Failed to create export request",
			})
			return
		}

		// Set content type based on the body format
		if strings.Contains(c.GetHeader("Content-Type"), "application/x-www-form-urlencoded") || (!strings.Contains(c.GetHeader("Content-Type"), "multipart/form-data") && !strings.Contains(c.GetHeader("Content-Type"), "application/json")) {
			req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		}

		// Set required headers for Highcharts export server (mimic browser request)
		req.Header.Set("Content-Type", c.GetHeader("Content-Type")) // Forward the original content type
		req.Header.Set("Accept", "*/*")
		req.Header.Set("Accept-Encoding", "gzip, deflate, br")
		req.Header.Set("Accept-Language", "en-US,en;q=0.9")
		req.Header.Set("Cache-Control", "no-cache")
		req.Header.Set("Pragma", "no-cache")
		req.Header.Set("Sec-Ch-Ua", `"Not_A Brand";v="8", "Chromium";v="120", "Google Chrome";v="120"`)
		req.Header.Set("Sec-Ch-Ua-Mobile", "?0")
		req.Header.Set("Sec-Ch-Ua-Platform", `"Linux"`)
		req.Header.Set("Sec-Fetch-Dest", "empty")
		req.Header.Set("Sec-Fetch-Mode", "cors")
		req.Header.Set("Sec-Fetch-Site", "same-origin")
		req.Header.Set("User-Agent", "Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36")
		req.Header.Set("Referer", "https://www.highcharts.com/")
		req.Header.Set("Origin", "https://www.highcharts.com")

		// Make the request to Highcharts export server
		client := &http.Client{
			Timeout: 30 * time.Second,
		}
		resp, err := client.Do(req)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{
				"success": false,
				"error":   "Failed to connect to export server",
			})
			return
		}
		defer resp.Body.Close()

		// Check if the response is successful
		if resp.StatusCode != http.StatusOK {
			// Log the response for debugging
			responseBody, _ := io.ReadAll(resp.Body)
			c.JSON(http.StatusBadGateway, gin.H{
				"success": false,
				"error":   fmt.Sprintf("Export server returned status %d: %s", resp.StatusCode, string(responseBody)),
			})
			return
		}

		// Set CORS headers
		c.Header("Access-Control-Allow-Origin", "*")
		c.Header("Access-Control-Allow-Methods", "POST, OPTIONS")
		c.Header("Access-Control-Allow-Headers", "Content-Type")

		// Copy response headers
		for key, values := range resp.Header {
			for _, value := range values {
				c.Header(key, value)
			}
		}

		// Stream the response body
		c.DataFromReader(resp.StatusCode, resp.ContentLength, resp.Header.Get("Content-Type"), resp.Body, nil)
	}
}

func GetListPriceTaskType() gin.HandlerFunc {
	return func(c *gin.Context) {
		dbWeb := gormdb.Databases.Web

		var requestBody struct {
			Company  string `json:"company"`
			TaskType string `json:"task_type"`
		}
		if err := c.ShouldBindJSON(&requestBody); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{
				"success": false,
				"error":   "Invalid request body: " + err.Error(),
			})
			return
		}

		var odooMSProductTemplate []odooms.InventoryProductTemplate
		if err := dbWeb.Model(&odooms.InventoryProductTemplate{}).
			Select("id", "name", "list_price", "company").
			Where("product_type = ? AND product_category = ?", "service", "Manage Service").
			Where("company = ? AND name = ?", requestBody.Company, requestBody.TaskType).
			Find(&odooMSProductTemplate).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				c.JSON(http.StatusNotFound, gin.H{
					"success": false,
					"error":   "No product template found for the given company and task type",
				})
				return
			}
			c.JSON(http.StatusInternalServerError, gin.H{
				"success": false,
				"error":   "Database error: " + err.Error(),
			})
			return
		}

		c.JSON(http.StatusOK, gin.H{
			"success": true,
			"data": gin.H{
				"product_templates": odooMSProductTemplate,
			},
		})
	}
}

func GetListPriceSalesPayment() gin.HandlerFunc {
	return func(c *gin.Context) {
		dbWeb := gormdb.Databases.Web

		var requestBody struct {
			Key     string `json:"key"`
			Company string `json:"company"`
			Type    string `json:"type"`
		}
		if err := c.ShouldBindJSON(&requestBody); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{
				"success": false,
				"error":   "Invalid request body: " + err.Error(),
			})
			return
		}

		var odooMSFsParamPayment []odooms.ODOOMSFSParamPayment
		if err := dbWeb.Model(&odooms.ODOOMSFSParamPayment{}).
			Select("id", "param_type", "param_key", "param_price", "param_company").
			// TODO: change it with the real param for FS Param Payment
			Where(&odooms.ODOOMSFSParamPayment{
				ParamCompany: requestBody.Company,
				ParamType:    requestBody.Type,
				ParamKey:     requestBody.Key,
			}).
			Find(&odooMSFsParamPayment).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				c.JSON(http.StatusNotFound, gin.H{
					"success": false,
					"error":   "No payment parameter found for the given company and type",
				})
				return
			}
			c.JSON(http.StatusInternalServerError, gin.H{
				"success": false,
				"error":   "Database error: " + err.Error(),
			})
			return
		}

		c.JSON(http.StatusOK, gin.H{
			"success": true,
			"data": gin.H{
				"payment_parameters": odooMSFsParamPayment,
			},
		})
	}
}

func GetCostRevenueODOOMSChart() gin.HandlerFunc {
	return func(c *gin.Context) {
		importPath := config.GetConfig().App.Logo
		newLogoPath := importPath[:len(importPath)-len(filepath.Base(importPath))] + "csna.png"
		c.HTML(http.StatusOK, "tab-cost-revenue.html", gin.H{
			"GLOBAL_URL": fun.GLOBAL_URL,
			"APP_LOGO":   newLogoPath,
			"ACCESS":     true,
		})
	}
}
