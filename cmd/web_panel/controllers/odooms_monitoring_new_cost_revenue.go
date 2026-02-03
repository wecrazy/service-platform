package controllers

import (
	"fmt"
	"net/http"
	"strings"
	"time"

	"service-platform/cmd/web_panel/config"
	"service-platform/cmd/web_panel/fun"
	"service-platform/cmd/web_panel/internal/gormdb"
	odooms "service-platform/cmd/web_panel/model/odoo_ms"

	"github.com/gin-gonic/gin"
)

// GetNewCostRevenueODOOMSChart renders the HTML page for Cost vs Revenue chart
func GetNewCostRevenueODOOMSChart() gin.HandlerFunc {
	return func(c *gin.Context) {
		importPath := config.GetConfig().App.Logo
		lastSlash := strings.LastIndex(importPath, "/")
		var newLogoPath string
		if lastSlash >= 0 {
			newLogoPath = importPath[:lastSlash+1] + "csna.png"
		} else {
			newLogoPath = "csna.png"
		}
		c.HTML(http.StatusOK, "tab-new-cost-revenue.html", gin.H{
			"GLOBAL_URL": fun.GLOBAL_URL,
			"APP_LOGO":   newLogoPath,
			"ACCESS":     true,
		})
	}
}

// GetAvailableYears returns list of available years based on monthly tables
func GetAvailableYears() gin.HandlerFunc {
	return func(c *gin.Context) {
		dbWeb := gormdb.Databases.Web
		table := config.GetConfig().Database.TbReportMonitoringTicket

		yearsMap := make(map[int]bool)
		currentYear := time.Now().Year()

		// Check for tables from 2024 to current year
		for year := 2024; year <= currentYear; year++ {
			for month := 1; month <= 12; month++ {
				monthName := time.Month(month).String()[:3]
				tableName := fmt.Sprintf("%s_%s%d", table, strings.ToLower(monthName), year)

				var count int64
				if err := dbWeb.Table(tableName).Limit(1).Count(&count).Error; err == nil {
					yearsMap[year] = true
					break // Found at least one table for this year
				}
			}
		}

		// Always include current year
		yearsMap[currentYear] = true

		// Convert map to sorted slice
		var years []int
		for year := range yearsMap {
			years = append(years, year)
		}

		// Sort years in descending order
		for i := 0; i < len(years)-1; i++ {
			for j := i + 1; j < len(years); j++ {
				if years[i] < years[j] {
					years[i], years[j] = years[j], years[i]
				}
			}
		}

		c.JSON(http.StatusOK, gin.H{
			"success": true,
			"data":    years,
		})
	}
}

// GetAvailableMonths returns list of available months for a given year
func GetAvailableMonths() gin.HandlerFunc {
	return func(c *gin.Context) {
		dbWeb := gormdb.Databases.Web
		table := config.GetConfig().Database.TbReportMonitoringTicket

		yearParam := c.Query("year")
		var selectedYear int
		if yearParam != "" {
			fmt.Sscanf(yearParam, "%d", &selectedYear)
		}
		if selectedYear == 0 {
			selectedYear = time.Now().Year()
		}

		currentYear := time.Now().Year()
		currentMonth := int(time.Now().Month())

		type MonthInfo struct {
			Value int    `json:"value"`
			Name  string `json:"name"`
		}

		var months []MonthInfo

		for month := 1; month <= 12; month++ {
			monthName := time.Month(month).String()[:3]
			var tableName string

			if month == currentMonth && selectedYear == currentYear {
				// Current month always available
				months = append(months, MonthInfo{
					Value: month,
					Name:  monthName,
				})
			} else {
				tableName = fmt.Sprintf("%s_%s%d", table, strings.ToLower(monthName), selectedYear)
				var count int64
				if err := dbWeb.Table(tableName).Limit(1).Count(&count).Error; err == nil {
					months = append(months, MonthInfo{
						Value: month,
						Name:  monthName,
					})
				}
			}
		}

		c.JSON(http.StatusOK, gin.H{
			"success": true,
			"data":    months,
		})
	}
}

// GetAvailableCompanies returns list of unique companies from the monitoring table
func GetAvailableCompanies() gin.HandlerFunc {
	return func(c *gin.Context) {
		dbWeb := gormdb.Databases.Web
		table := config.GetConfig().Database.TbReportMonitoringTicket

		yearParam := c.Query("year")
		var selectedYear int
		if yearParam != "" {
			fmt.Sscanf(yearParam, "%d", &selectedYear)
		}
		if selectedYear == 0 {
			selectedYear = time.Now().Year()
		}

		// Determine which tables to check
		currentYear := time.Now().Year()
		currentMonth := int(time.Now().Month())

		type CompanyResult struct {
			Company string
		}
		companiesMap := make(map[string]bool)

		// Check each month's table for the selected year
		for month := 1; month <= 12; month++ {
			var tableName string
			if selectedYear == currentYear && month == currentMonth {
				tableName = table
			} else {
				monthName := time.Month(month).String()[:3]
				tableName = fmt.Sprintf("%s_%s%d", table, strings.ToLower(monthName), selectedYear)
			}

			// Check if table exists
			if !dbWeb.Migrator().HasTable(tableName) {
				continue
			}

			var companies []CompanyResult
			dbWeb.Table(tableName).
				Select("DISTINCT company").
				Where("company IS NOT NULL AND company != ''").
				Find(&companies)

			for _, c := range companies {
				companiesMap[c.Company] = true
			}
		}

		// Convert map to sorted slice
		var companies []string
		for company := range companiesMap {
			companies = append(companies, company)
		}

		c.JSON(http.StatusOK, gin.H{
			"success": true,
			"data":    companies,
		})
	}
}

// GetDataNewCostRevenueYearly returns yearly stacked bar data for Cost vs Revenue
func GetDataNewCostRevenueYearly() gin.HandlerFunc {
	return func(c *gin.Context) {
		dbWeb := gormdb.Databases.Web
		table := config.GetConfig().Database.TbReportMonitoringTicket
		tableBALost := config.GetConfig().Database.TbBALost

		// Get year from query parameter, default to current year
		yearParam := c.Query("year")
		var selectedYear int
		if yearParam != "" {
			fmt.Sscanf(yearParam, "%d", &selectedYear)
		}
		if selectedYear == 0 {
			selectedYear = time.Now().Year()
		}

		// Get selected months from query parameter (comma-separated)
		monthsParam := c.Query("months")
		var selectedMonths []int
		if monthsParam != "" {
			monthStrs := strings.Split(monthsParam, ",")
			for _, ms := range monthStrs {
				var m int
				fmt.Sscanf(strings.TrimSpace(ms), "%d", &m)
				if m >= 1 && m <= 12 {
					selectedMonths = append(selectedMonths, m)
				}
			}
		}
		// If no months selected, show all months
		if len(selectedMonths) == 0 {
			for i := 1; i <= 12; i++ {
				selectedMonths = append(selectedMonths, i)
			}
		}

		// Get selected companies from query parameter (comma-separated)
		companiesParam := c.Query("companies")
		var selectedCompanies []string
		if companiesParam != "" {
			selectedCompanies = strings.Split(companiesParam, ",")
			for i := range selectedCompanies {
				selectedCompanies[i] = strings.TrimSpace(selectedCompanies[i])
			}
		}

		currentYear := time.Now().Year()
		currentMonth := int(time.Now().Month())

		type MonthData struct {
			Month              string  `json:"month"`
			Revenue            float64 `json:"revenue"`
			RevenueTicketCount int     `json:"revenue_ticket_count"`
			Payroll            float64 `json:"payroll"`
			PayrollTicketCount int     `json:"payroll_ticket_count"`
			Profit             float64 `json:"profit"`
			BALost             float64 `json:"ba_lost"`
			BALostTicketCount  int     `json:"ba_lost_ticket_count"`
			OverSLA            float64 `json:"over_sla"`
			OverSLATicketCount int     `json:"over_sla_ticket_count"`
			CostPenalties      float64 `json:"cost_penalties"`
			CostPenaltiesCount int     `json:"cost_penalties_count"`
			GrossProfit        float64 `json:"gross_profit"`
		}

		var yearlyData []MonthData

		// Default prices
		defaultTechnicianVisitsPrice := config.GetConfig().ODOOMSParam.DefaultPrice
		defaultBALostPrice := config.GetConfig().ODOOMSParam.DefaultEDCLostFee

		// Build price maps
		revenuePriceMap := make(map[string]float64) // key: "company|task_type" - from InventoryProductTemplate
		payrollPriceMap := make(map[string]float64) // key: "company|task_type" - from ODOOMSFSParamPayment

		// Get all inventory product templates for revenue pricing
		var productTemplates []odooms.InventoryProductTemplate
		dbWeb.Model(&odooms.InventoryProductTemplate{}).
			Where("product_type = ? AND product_category = ?", "service", "Manage Service").
			Find(&productTemplates)

		for _, pt := range productTemplates {
			key := fmt.Sprintf("%s|%s", pt.Company, pt.Name)
			revenuePriceMap[key] = pt.ListPrice
		}

		// Get all payment parameters for payroll pricing
		var paymentParams []odooms.ODOOMSFSParamPayment
		dbWeb.Model(&odooms.ODOOMSFSParamPayment{}).Find(&paymentParams)

		for _, pp := range paymentParams {
			key := fmt.Sprintf("%s|%s", pp.ParamCompany, pp.ParamKey)
			if pp.ParamType == "Price" {
				payrollPriceMap[key] = float64(pp.ParamPrice)
			}
		}

		// Loop through selected months
		for _, month := range selectedMonths {
			monthName := time.Month(month).String()[:3]
			var tableName string

			// Use base table for current month of current year, otherwise use monthly table
			if month == currentMonth && selectedYear == currentYear {
				tableName = table // Current month uses base table
			} else {
				tableName = fmt.Sprintf("%s_%s%d", table, strings.ToLower(monthName), selectedYear)
				// Check if table exists
				var count int64
				if err := dbWeb.Table(tableName).Limit(1).Count(&count).Error; err != nil {
					continue // Skip if table doesn't exist
				}
			}

			monthData := MonthData{
				Month: monthName,
			}

			type TicketCount struct {
				Company  string
				TaskType string
				Count    int
			}

			// 1. Revenue - ALL tickets received (no filters - represents total potential income)
			// Price from InventoryProductTemplate (list_price)
			var revenueTickets []TicketCount
			revenueQuery := dbWeb.Table(tableName).
				Select("company, task_type, COUNT(*) as count")
			if len(selectedCompanies) > 0 {
				revenueQuery = revenueQuery.Where("company IN ?", selectedCompanies)
			}
			revenueQuery.Group("company, task_type").Find(&revenueTickets)

			for _, ticket := range revenueTickets {
				key := fmt.Sprintf("%s|%s", ticket.Company, ticket.TaskType)
				price := revenuePriceMap[key]
				monthData.Revenue += price * float64(ticket.Count)
				monthData.RevenueTicketCount += ticket.Count
			}

			// 2. Payroll (Cost to Technicians) - completed tickets with specific sla_status logic
			// Price from ODOOMSFSParamPayment (param_type = 'Price') or default
			var payrollTickets []TicketCount
			payrollQuery := dbWeb.Table(tableName).
				Select("company, task_type, COUNT(*) as count").
				Where("complete_wo IS NOT NULL").
				Where("sla_status = ?", "On Target Solved").
				Where("(stage IN (?, ?, ?, ?, ?) OR stage = ?)",
					"Done", "Waiting For Verification", "Closed", "Solved", "Solved Pending", "Pending")
			if len(selectedCompanies) > 0 {
				payrollQuery = payrollQuery.Where("company IN ?", selectedCompanies)
			}
			payrollQuery.Group("company, task_type").Find(&payrollTickets)

			for _, ticket := range payrollTickets {
				key := fmt.Sprintf("%s|%s", ticket.Company, ticket.TaskType)
				payrollPrice := payrollPriceMap[key]
				if payrollPrice == 0 {
					payrollPrice = defaultTechnicianVisitsPrice
				}
				monthData.Payroll += payrollPrice * float64(ticket.Count)
				monthData.PayrollTicketCount += ticket.Count
			}

			// 3. Profit = Revenue - Payroll
			monthData.Profit = monthData.Revenue - monthData.Payroll

			// 4. BA Lost - Count distinct serialnumber from tableBALost monthly table where link_foto IS NULL AND note_all IS NULL
			// Price: 2,000,000 per ticket
			var baLostTableName string
			if selectedYear == currentYear && month == currentMonth {
				baLostTableName = tableBALost
			} else {
				monthName := time.Month(month).String()[:3]
				baLostTableName = fmt.Sprintf("%s_%s%d", tableBALost, strings.ToLower(monthName), selectedYear)
			}

			// Check if BA Lost table exists
			if dbWeb.Migrator().HasTable(baLostTableName) {
				var baLostCount int64
				dbWeb.Table(baLostTableName).
					Where("(link_foto IS NULL OR link_foto = '') AND (note_all IS NULL OR note_all = '')").
					Distinct("serialnumber").
					Count(&baLostCount)

				monthData.BALost = defaultBALostPrice * float64(baLostCount)
				monthData.BALostTicketCount = int(baLostCount)
			} // 5. Over SLA - Overdue tickets (sla_status = 'Overdue (New)' OR 'Overdue (Visited)')
			// Price: Same as revenue (from InventoryProductTemplate)
			var overSLATickets []TicketCount
			overSLAQuery := dbWeb.Table(tableName).
				Select("company, task_type, COUNT(*) as count").
				Where("sla_status IN (?, ?)", "Overdue (New)", "Overdue (Visited)")
			if len(selectedCompanies) > 0 {
				overSLAQuery = overSLAQuery.Where("company IN ?", selectedCompanies)
			}
			overSLAQuery.Group("company, task_type").Find(&overSLATickets)

			for _, ticket := range overSLATickets {
				key := fmt.Sprintf("%s|%s", ticket.Company, ticket.TaskType)
				price := revenuePriceMap[key]
				monthData.OverSLA += price * float64(ticket.Count)
				monthData.OverSLATicketCount += ticket.Count
			}

			// 6. Cost of Penalties (From Customers) - stage = 'New'
			// Price: Same as revenue (from InventoryProductTemplate)
			var penaltyTickets []TicketCount
			penaltyQuery := dbWeb.Table(tableName).
				Select("company, task_type, COUNT(*) as count").
				Where("stage = ? AND sla_status = ? AND complete_wo IS NULL", "New", "Overdue (New)")
			if len(selectedCompanies) > 0 {
				penaltyQuery = penaltyQuery.Where("company IN ?", selectedCompanies)
			}
			penaltyQuery.Group("company, task_type").Find(&penaltyTickets)

			for _, ticket := range penaltyTickets {
				key := fmt.Sprintf("%s|%s", ticket.Company, ticket.TaskType)
				price := revenuePriceMap[key]
				monthData.CostPenalties += price * float64(ticket.Count)
				monthData.CostPenaltiesCount += ticket.Count
			}

			// 7. Gross Profit = Profit + BA Lost + Over SLA + Cost of Penalties
			// (These are potential revenues if all issues are fixed)
			monthData.GrossProfit = monthData.Profit + monthData.BALost + monthData.OverSLA + monthData.CostPenalties

			yearlyData = append(yearlyData, monthData)
		}

		c.JSON(http.StatusOK, gin.H{
			"success": true,
			"data":    yearlyData,
		})
	}
}

// GetDrillDownData returns detailed breakdown by company and task_type
func GetDrillDownData() gin.HandlerFunc {
	return func(c *gin.Context) {
		dbWeb := gormdb.Databases.Web
		table := config.GetConfig().Database.TbReportMonitoringTicket
		currentYear := time.Now().Year()

		var requestBody struct {
			Month    string `json:"month"`
			Category string `json:"category"` // "revenue", "payroll", "ba_lost", "over_sla", "penalties"
		}

		if err := c.ShouldBindJSON(&requestBody); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{
				"success": false,
				"error":   "Invalid request body: " + err.Error(),
			})
			return
		}

		// Parse month name to get table name
		monthNum := 0
		monthNames := []string{"Jan", "Feb", "Mar", "Apr", "May", "Jun", "Jul", "Aug", "Sep", "Oct", "Nov", "Dec"}
		for i, name := range monthNames {
			if name == requestBody.Month {
				monthNum = i + 1
				break
			}
		}

		var tableName string
		currentMonth := int(time.Now().Month())
		if monthNum == currentMonth {
			tableName = table
		} else {
			tableName = fmt.Sprintf("%s_%s%d", table, strings.ToLower(requestBody.Month), currentYear)
		}

		type DrillDownRow struct {
			Company   string  `json:"company"`
			TaskType  string  `json:"task_type"`
			SLAStatus string  `json:"sla_status,omitempty"`
			Count     int     `json:"count"`
			Price     float64 `json:"price"`
			Total     float64 `json:"total"`
		}

		var drillData []DrillDownRow

		// Handle BA Lost separately as it uses different table
		if requestBody.Category == "ba_lost" {
			// Get tableBALost
			tableBALost := config.GetConfig().Database.TbBALost
			var baLostTableName string
			if monthNum == currentMonth {
				baLostTableName = tableBALost
			} else {
				baLostTableName = fmt.Sprintf("%s_%s%d", tableBALost, strings.ToLower(requestBody.Month), currentYear)
			}

			// Check if BA Lost table exists
			if !dbWeb.Migrator().HasTable(baLostTableName) {
				c.JSON(http.StatusOK, gin.H{
					"success": true,
					"data":    []interface{}{},
				})
				return
			}

			// Get detailed BA Lost records
			var baLostRecords []odooms.CSNABALost
			if err := dbWeb.Table(baLostTableName).
				Where("(link_foto IS NULL OR link_foto = '') AND (note_all IS NULL OR note_all = '')").
				Order("serialnumber ASC").
				Find(&baLostRecords).Error; err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{
					"success": false,
					"error":   "Database error: " + err.Error(),
				})
				return
			}

			defaultBALostPrice := config.GetConfig().ODOOMSParam.DefaultEDCLostFee

			// Return detailed BA Lost data
			type BALostDetail struct {
				SerialNumber string  `json:"serial_number"`
				Vendor       string  `json:"vendor"`
				Location     string  `json:"location"`
				Merk         string  `json:"merk"`
				EDCType      string  `json:"edc_type"`
				Device       string  `json:"device"`
				StatusEDC    string  `json:"status_edc"`
				Head         string  `json:"head"`
				SP           string  `json:"sp"`
				Region       string  `json:"region"`
				Price        float64 `json:"price"`
			}

			var baLostDetails []BALostDetail
			for _, record := range baLostRecords {
				baLostDetails = append(baLostDetails, BALostDetail{
					SerialNumber: record.SerialNumber,
					Vendor:       record.Vendor,
					Location:     record.Location,
					Merk:         record.Merk,
					EDCType:      record.EDCType,
					Device:       record.Device,
					StatusEDC:    record.StatusEDC,
					Head:         record.Head,
					SP:           record.SP,
					Region:       record.Region,
					Price:        defaultBALostPrice,
				})
			}

			c.JSON(http.StatusOK, gin.H{
				"success": true,
				"data":    baLostDetails,
				"total":   defaultBALostPrice * float64(len(baLostDetails)),
				"count":   len(baLostDetails),
			})
			return
		}

		// Build query based on category (for non-BA Lost)
		selectFields := "company, task_type, COUNT(*) as count"
		if requestBody.Category == "over_sla" {
			selectFields = "company, task_type, sla_status, COUNT(*) as count"
		}
		query := dbWeb.Table(tableName).Select(selectFields)

		switch requestBody.Category {
		case "revenue":
			// No filters - ALL tickets received
		case "payroll":
			query = query.
				Where("complete_wo IS NOT NULL").
				Where("sla_status = ?", "On Target Solved").
				Where("(stage IN (?, ?, ?, ?, ?) OR stage = ?)",
					"Done", "Waiting For Verification", "Closed", "Solved", "Solved Pending", "Pending")
		case "over_sla":
			query = query.Where("sla_status IN (?, ?)", "Overdue (New)", "Overdue (Visited)")
		case "penalties":
			query = query.
				Where("stage = ? AND sla_status = ? AND complete_wo IS NULL", "New", "Overdue (New)")
		default:
			c.JSON(http.StatusBadRequest, gin.H{
				"success": false,
				"error":   "Invalid category",
			})
			return
		}

		// Group by fields
		if requestBody.Category == "over_sla" {
			query = query.Group("company, task_type, sla_status")
		} else {
			query = query.Group("company, task_type")
		}

		type TicketCount struct {
			Company   string
			TaskType  string
			SLAStatus string
			Count     int
		}

		var tickets []TicketCount
		if err := query.Find(&tickets).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{
				"success": false,
				"error":   "Database error: " + err.Error(),
			})
			return
		} // Get price maps
		defaultTechnicianVisitsPrice := config.GetConfig().ODOOMSParam.DefaultPrice
		defaultBALostPrice := config.GetConfig().ODOOMSParam.DefaultEDCLostFee

		revenuePriceMap := make(map[string]float64)
		payrollPriceMap := make(map[string]float64)

		var productTemplates []odooms.InventoryProductTemplate
		dbWeb.Model(&odooms.InventoryProductTemplate{}).
			Where("product_type = ? AND product_category = ?", "service", "Manage Service").
			Find(&productTemplates)

		for _, pt := range productTemplates {
			key := fmt.Sprintf("%s|%s", pt.Company, pt.Name)
			revenuePriceMap[key] = pt.ListPrice
		}

		var paymentParams []odooms.ODOOMSFSParamPayment
		dbWeb.Model(&odooms.ODOOMSFSParamPayment{}).Find(&paymentParams)

		for _, pp := range paymentParams {
			key := fmt.Sprintf("%s|%s", pp.ParamCompany, pp.ParamKey)
			if pp.ParamType == "Price" {
				payrollPriceMap[key] = float64(pp.ParamPrice)
			}
		}

		// Calculate prices based on category
		for _, ticket := range tickets {
			key := fmt.Sprintf("%s|%s", ticket.Company, ticket.TaskType)
			var price float64

			switch requestBody.Category {
			case "revenue":
				price = revenuePriceMap[key] // Revenue from InventoryProductTemplate
			case "payroll":
				price = payrollPriceMap[key]
				if price == 0 {
					price = defaultTechnicianVisitsPrice // Default 12,000
				}
			case "ba_lost":
				price = defaultBALostPrice // 2,000,000 per edc lost
			case "over_sla":
				price = revenuePriceMap[key] // Same as revenue
			case "penalties":
				price = revenuePriceMap[key] // Same as revenue
			}

			drillData = append(drillData, DrillDownRow{
				Company:   ticket.Company,
				TaskType:  ticket.TaskType,
				SLAStatus: ticket.SLAStatus,
				Count:     ticket.Count,
				Price:     price,
				Total:     price * float64(ticket.Count),
			})
		}

		c.JSON(http.StatusOK, gin.H{
			"success": true,
			"data":    drillData,
		})
	}
}
