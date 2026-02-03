package controllers

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math/rand"
	"os"
	"path/filepath"
	"runtime"
	"service-platform/cmd/web_panel/config"
	"service-platform/cmd/web_panel/fun"
	"service-platform/cmd/web_panel/internal/gormdb"
	sptechnicianmodel "service-platform/cmd/web_panel/model/sp_technician_model"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/TigorLazuardi/tanggal"
	htgotts "github.com/hegedustibor/htgo-tts"
	"github.com/hegedustibor/htgo-tts/handlers"
	"github.com/hegedustibor/htgo-tts/voices"
	"github.com/samber/lo"
	"github.com/sirupsen/logrus"
	"github.com/xuri/excelize/v2"
	"go.mau.fi/whatsmeow/types"
	"gorm.io/gorm"
)

var (
	TechODOOMSData                        = make(map[string]TechnicianODOOData)
	getDataTechnicianPlannedForTodayMutex sync.Mutex
	checkSPTechnicianMutex                sync.Mutex
	createTechnicianLoginReportMutex      sync.Mutex
	sendTechnicianLoginReportMutex        sync.Mutex
	getDataTechnicianFromODOOMSMutex      sync.Mutex

	checkingSPTechnicianV2Mutex sync.Mutex
	getDataOfSOTechnicianMutex  sync.Mutex
)

// TechnicianAggregateData holds aggregated data for a technician
type TechnicianAggregateData struct {
	TechnicianName        string
	SPL                   string
	SAC                   string
	WONumbers             []string
	TicketSubjects        []string
	WONumbersVisited      []string
	TicketSubjectsVisited []string
	FirstUploaded         *time.Time
	LatestVisit           *time.Time
	Email                 string
	NoHP                  string
	Name                  string
}

type TechnicianODOOData struct {
	TechnicianID                uint
	SPL                         string
	SAC                         string
	LastLogin                   *time.Time
	LastDownloadJO              *time.Time
	Email                       string
	NoHP                        string
	Name                        string
	UserCreatedOn               *time.Time
	JobGroupID                  int
	NIK                         string
	Address                     string
	Area                        string
	TTL                         string
	EmployeeCode                string
	TechnicianInventoryLocation map[string]string // Location from inventory : each company each stock location
}

type ExcelColumnWithComment struct {
	ColIndex   string
	ColTitle   string
	ColSize    float64
	ColComment string
}

type SPInfoForSPL struct {
	GotSPToday bool
	SPNumber   int
}

// updateTechnicianLastVisitFromBatch updates technician last visit times using data from the current batch
func updateTechnicianLastVisitFromBatch(listOfData []OdooTaskDataRequestItem) error {
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
		if err := dbWeb.Model(&sptechnicianmodel.JOPlannedForTechnicianODOOMS{}).
			Where("technician = ?", technician).
			Where("created_at >= ? AND created_at <= ?", startOfDay, endOfDay).
			Update("technician_last_visit", latestVisit).Error; err != nil {
			logrus.Errorf("Failed to update last visit for technician %s: %v", technician, err)
		} else {
			// logrus.Debugf("Updated last visit for technician %s: %v", technician, latestVisit)
		}
	}

	return nil
}

// updateTechnicianFirstUploadFromBatch updates technician first upload times using the earliest timesheetlaststop from the batch
func updateTechnicianFirstUploadFromBatch(listOfData []OdooTaskDataRequestItem) error {
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
		if err := dbWeb.Model(&sptechnicianmodel.JOPlannedForTechnicianODOOMS{}).
			Where("technician = ?", technician).
			Where("created_at >= ? AND created_at <= ?", startOfDay, endOfDay).
			Update("technician_first_upload", firstUpload).Error; err != nil {
			logrus.Errorf("Failed to update first upload for technician %s: %v", technician, err)
		} else {
			// logrus.Debugf("Updated first upload for technician %s: %v", technician, firstUpload)
		}
	}

	return nil
}

// clearOldTechnicianData removes old technician data for the current day to avoid duplicates
func clearOldTechnicianData() error {
	loc, _ := time.LoadLocation(config.GetConfig().Default.Timezone)
	now := time.Now().In(loc)

	startOfDay := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, loc)
	endOfDay := time.Date(now.Year(), now.Month(), now.Day(), 23, 59, 59, 0, loc)

	// Delete records that were created today
	result := dbWeb.Where("created_at >= ? AND created_at <= ?", startOfDay, endOfDay).
		Delete(&sptechnicianmodel.JOPlannedForTechnicianODOOMS{})

	if result.Error != nil {
		return fmt.Errorf("failed to clear old technician data: %v", result.Error)
	}

	// logrus.Infof("Cleared %d old technician records for today", result.RowsAffected)
	return nil
}

// groupDataByTechnician groups the ODOO data by technician and creates aggregated records
func groupDataByTechnician(listOfData []OdooTaskDataRequestItem) []sptechnicianmodel.JOPlannedForTechnicianODOOMS {
	// Map to group data by technician
	technicianMap := make(map[string]*TechnicianAggregateData)

	// Process each data item
	for _, data := range listOfData {
		_, technicianName := parseJSONIDDataCombinedSafe(data.TechnicianId)
		if technicianName == "" {
			continue // skip records without technician
		}

		_, ticketSubjectUncleaned := parseJSONIDDataCombinedSafe(data.HelpdeskTicketId)
		ticketSubject := CleanSPKNumber(ticketSubjectUncleaned)

		// Get or create technician aggregate data
		if technicianMap[technicianName] == nil {
			// Get SPL and SAC from TechODOOMSData if available
			var spl, sac, techEmail, techNoHP, techName string
			if techData, exists := TechODOOMSData[technicianName]; exists {
				spl = techData.SPL
				sac = techData.SAC
				techEmail = techData.Email
				techNoHP = techData.NoHP
				techName = techData.Name
			}

			technicianMap[technicianName] = &TechnicianAggregateData{
				TechnicianName:        technicianName,
				SPL:                   spl,
				SAC:                   sac,
				WONumbers:             []string{},
				TicketSubjects:        []string{},
				WONumbersVisited:      []string{},
				TicketSubjectsVisited: []string{},
				FirstUploaded:         nil,
				LatestVisit:           nil,
				Email:                 techEmail,
				NoHP:                  techNoHP,
				Name:                  techName,
			}
		}

		// Add WO number to array (just the string value)
		if data.WoNumber != "" {
			technicianMap[technicianName].WONumbers = append(technicianMap[technicianName].WONumbers, data.WoNumber)
			if data.TimesheetLastStop.Valid {
				technicianMap[technicianName].WONumbersVisited = append(technicianMap[technicianName].WONumbersVisited, data.WoNumber)
			}
		}

		// Add ticket subject to array (just the string value)
		if ticketSubject != "" {
			technicianMap[technicianName].TicketSubjects = append(technicianMap[technicianName].TicketSubjects, ticketSubject)
			if data.TimesheetLastStop.Valid {
				technicianMap[technicianName].TicketSubjectsVisited = append(technicianMap[technicianName].TicketSubjectsVisited, ticketSubject)
			}
		}

		// Update latest visit time
		if data.TimesheetLastStop.Valid {
			if technicianMap[technicianName].LatestVisit == nil || data.TimesheetLastStop.Time.After(*technicianMap[technicianName].LatestVisit) {
				technicianMap[technicianName].LatestVisit = &data.TimesheetLastStop.Time
			}
		}
	}

	// Convert map to slice of database records
	var result []sptechnicianmodel.JOPlannedForTechnicianODOOMS
	for _, aggData := range technicianMap {
		// Convert arrays to JSON
		woNumbersJSON, _ := json.Marshal(aggData.WONumbers)
		ticketSubjectsJSON, _ := json.Marshal(aggData.TicketSubjects)
		woNumbersVisitedJSON, _ := json.Marshal(aggData.WONumbersVisited)
		ticketSubjectsVisitedJSON, _ := json.Marshal(aggData.TicketSubjectsVisited)

		// Get login and download times from TechODOOMSData
		var lastLogin, lastDownload *time.Time
		if techData, exists := TechODOOMSData[aggData.TechnicianName]; exists {
			lastLogin = techData.LastLogin
			lastDownload = techData.LastDownloadJO
		}

		record := sptechnicianmodel.JOPlannedForTechnicianODOOMS{
			Technician:               aggData.TechnicianName,
			SPL:                      aggData.SPL,
			SAC:                      aggData.SAC,
			WONumber:                 woNumbersJSON,
			TicketSubject:            ticketSubjectsJSON,
			WONumberVisited:          woNumbersVisitedJSON,
			TicketSubjectVisited:     ticketSubjectsVisitedJSON,
			TechnicianLastLogin:      lastLogin,
			TechnicianLastDownloadJO: lastDownload,
			TechnicianFirstUpload:    aggData.FirstUploaded,
			TechnicianLastVisit:      aggData.LatestVisit,
			EmailTechnician:          aggData.Email,
			NoHPTechnician:           aggData.NoHP,
			Name:                     aggData.Name,
		}

		result = append(result, record)
	}

	return result
}

func GetDataTechnicianODOOMS() error {
	getDataTechnicianFromODOOMSMutex.Lock()
	defer getDataTechnicianFromODOOMSMutex.Unlock()

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
		"nik",
		"address",
		"area",
		"birth_status",
		"create_date",
		"active",
		"x_employee_code",
		"technician_locations",
	}
	order := "name asc"

	odooParams := map[string]interface{}{
		"model":  ODOOModel,
		"domain": domain,
		"fields": fields,
		"order":  order,
	}
	payload := map[string]interface{}{
		"jsonrpc": config.GetConfig().ApiODOO.JSONRPC,
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
		return errors.New("failed to assert results as []interface{}")
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
		return errors.New("empty data technician in ODOO MS")
	}

	// Collect all login and download IDs for batch processing
	var allLoginIDs []float64
	var allDownloadIDs []float64
	technicianLoginMap := make(map[string]float64)    // technician name -> latest login ID
	technicianDownloadMap := make(map[string]float64) // technician name -> latest download ID

	var allTechnicianLocationIDS []float64

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

		// Find technician locations IDs
		if len(emp.TechnicianLocations) > 0 {
			for _, loc := range emp.TechnicianLocations {
				if loc.Valid {
					allTechnicianLocationIDS = append(allTechnicianLocationIDS, loc.Float)
				}
			}
		}

		var phoneNumberUsed string
		if config.GetConfig().SPTechnician.ActiveDebug {
			phoneNumberUsed = config.GetConfig().SPTechnician.PhoneNumberUsedForTest
		} else {
			sanitizedPhone, err := fun.SanitizePhoneNumber(emp.NoTelp.String)
			if err != nil {
				logrus.Errorf("Failed to sanitize phone number %s from %s: %v", emp.NoTelp.String, emp.NameFS.String, err)
				phoneNumberUsed = emp.NoTelp.String
			} else {
				phoneNumberUsed = "62" + sanitizedPhone
			}
		}

		// Initialize technician data with basic info
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

		TechODOOMSData[technicianName] = TechnicianODOOData{
			TechnicianID:   emp.ID,
			SPL:            emp.SPL.String,
			SAC:            emp.Head.String,
			LastLogin:      nil,
			LastDownloadJO: nil,
			Email:          emp.Email.String,
			NoHP:           phoneNumberUsed,
			Name:           emp.TechnicianName.String,
			NIK:            emp.NIK.String,
			Address:        emp.Alamat.String,
			Area:           emp.Area.String,
			TTL:            emp.TempatTanggalLahir.String,
			UserCreatedOn:  userCreatedOn,
			EmployeeCode:   emp.EmployeeCode.String,
		}
	}

	// Batch get technician locations
	technicianLocations, err := getBatchTechnicianLocations(allTechnicianLocationIDS)
	if err != nil {
		logrus.Errorf("Failed to get batch technician locations: %v", err)
	}

	// Batch get all login and download times
	loginTimes, downloadTimes, err := getBatchLoginAndDownloadTimes(allLoginIDs, allDownloadIDs)
	if err != nil {
		logrus.Errorf("Failed to get batch login/download times: %v", err)
		// Continue without login/download times
	}

	// Update technician data with login/download times
	for technicianName, data := range TechODOOMSData {
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
		TechODOOMSData[technicianName] = data
	}

	// Update technician data with locations
	for _, emp := range employeeData {
		technicianName := emp.NameFS.String
		if technicianName == "" {
			continue
		}

		if data, exists := TechODOOMSData[technicianName]; exists {
			inventoryLocations := make(map[string]string)
			for _, loc := range emp.TechnicianLocations {
				if loc.Valid {
					locID := int(loc.Float)
					if locInfo, found := technicianLocations[locID]; found {
						inventoryLocations[locInfo.CompanyName] = locInfo.LocationName
					}
				}
			}
			data.TechnicianInventoryLocation = inventoryLocations
			TechODOOMSData[technicianName] = data
		}
	}

	return nil
}

// func getLastLoginAndLastDownloadTechnician(lastLoginID, lastDownloadID float64) (*time.Time, *time.Time, error) {
// 	var lastLoginTime, lastDownloadTime *time.Time

// 	if lastLoginID > 0 {
// 		ODOOModel := "technician.login"
// 		domain := []interface{}{
// 			[]interface{}{"id", "=", lastLoginID},
// 		}
// 		fields := []string{
// 			"id",
// 			"login_time",
// 		}
// 		order := "id desc"
// 		odooParams := map[string]interface{}{
// 			"model":  ODOOModel,
// 			"domain": domain,
// 			"fields": fields,
// 			"order":  order,
// 		}
// 		payload := map[string]interface{}{
// 			"jsonrpc": config.GetConfig().ApiODOO.JSONRPC,
// 			"params":  odooParams,
// 		}
// 		payloadBytes, err := json.Marshal(payload)
// 		if err != nil {
// 			return nil, nil, fmt.Errorf("failed to marshal ODOO MS request payload: %v", err)
// 		}
// 		ODOOresponse, err := GetODOOMSData(string(payloadBytes))
// 		if err != nil {
// 			return nil, nil, fmt.Errorf("failed fetching data from ODOO MS API: %v", err)
// 		}
// 		ODOOResponseArray, ok := ODOOresponse.([]interface{})
// 		if !ok {
// 			return nil, nil, errors.New("failed to assert results as []interface{}")
// 		}

// 		ODOOResponseBytes, err := json.Marshal(ODOOResponseArray)
// 		if err != nil {
// 			return nil, nil, fmt.Errorf("failed to marshal ODOO response: %v", err)
// 		}

// 		var techLoginData []OdooTechnicianLoginItem
// 		if err := json.Unmarshal(ODOOResponseBytes, &techLoginData); err != nil {
// 			return nil, nil, fmt.Errorf("failed to unmarshal ODOO response body: %v", err)
// 		}

// 		if len(techLoginData) > 0 && techLoginData[0].LoginTime.Valid {
// 			lastLoginTime = &techLoginData[0].LoginTime.Time
// 		}
// 	}

// 	if lastDownloadID > 0 {
// 		ODOOModel := "technician.download"
// 		domain := []interface{}{
// 			[]interface{}{"id", "=", lastDownloadID},
// 		}
// 		fields := []string{
// 			"id",
// 			"download_time",
// 		}
// 		order := "id desc"
// 		odooParams := map[string]interface{}{
// 			"model":  ODOOModel,
// 			"domain": domain,
// 			"fields": fields,
// 			"order":  order,
// 		}
// 		payload := map[string]interface{}{
// 			"jsonrpc": config.GetConfig().ApiODOO.JSONRPC,
// 			"params":  odooParams,
// 		}
// 		payloadBytes, err := json.Marshal(payload)
// 		if err != nil {
// 			return nil, nil, fmt.Errorf("failed to marshal ODOO MS request payload: %v", err)
// 		}
// 		ODOOresponse, err := GetODOOMSData(string(payloadBytes))
// 		if err != nil {
// 			return nil, nil, fmt.Errorf("failed fetching data from ODOO MS API: %v", err)
// 		}
// 		ODOOResponseArray, ok := ODOOresponse.([]interface{})
// 		if !ok {
// return nil, nil, errors.New("failed to assert results as []interface{}")
// 		}

// 		ODOOResponseBytes, err := json.Marshal(ODOOResponseArray)
// 		if err != nil {
// 			return nil, nil, fmt.Errorf("failed to marshal ODOO response: %v", err)
// 		}

// 		var techDownloadData []OdooTechnicianDownloadItem
// 		if err := json.Unmarshal(ODOOResponseBytes, &techDownloadData); err != nil {
// 			return nil, nil, fmt.Errorf("failed to unmarshal ODOO response body: %v", err)
// 		}

// 		if len(techDownloadData) > 0 && techDownloadData[0].DownloadTime.Valid {
// 			lastDownloadTime = &techDownloadData[0].DownloadTime.Time
// 		}
// 	}

// 	return lastLoginTime, lastDownloadTime, nil
// }

// getBatchLoginAndDownloadTimes gets login and download times for multiple IDs in batch
func getBatchLoginAndDownloadTimes(loginIDs, downloadIDs []float64) (map[float64]*time.Time, map[float64]*time.Time, error) {
	loginTimes := make(map[float64]*time.Time)
	downloadTimes := make(map[float64]*time.Time)

	// Batch get login times
	if len(loginIDs) > 0 {
		ODOOModel := "technician.login"
		domain := []interface{}{
			[]interface{}{"id", "in", loginIDs},
		}
		fields := []string{
			"id",
			"login_time",
		}
		order := "id desc"
		odooParams := map[string]interface{}{
			"model":  ODOOModel,
			"domain": domain,
			"fields": fields,
			"order":  order,
		}
		payload := map[string]interface{}{
			"jsonrpc": config.GetConfig().ApiODOO.JSONRPC,
			"params":  odooParams,
		}
		payloadBytes, err := json.Marshal(payload)
		if err != nil {
			return nil, nil, fmt.Errorf("failed to marshal login request: %v", err)
		}

		ODOOresponse, err := GetODOOMSData(string(payloadBytes))
		if err != nil {
			logrus.Errorf("Failed to fetch login data: %v", err)
		} else {
			ODOOResponseArray, ok := ODOOresponse.([]interface{})
			if ok {
				ODOOResponseBytes, _ := json.Marshal(ODOOResponseArray)
				var techLoginData []OdooTechnicianLoginItem
				if json.Unmarshal(ODOOResponseBytes, &techLoginData) == nil {
					for _, login := range techLoginData {
						if login.LoginTime.Valid {
							loginTimes[float64(login.ID)] = &login.LoginTime.Time
						}
					}
				}
			}
		}
	}

	// Batch get download times
	if len(downloadIDs) > 0 {
		ODOOModel := "technician.download"
		domain := []interface{}{
			[]interface{}{"id", "in", downloadIDs},
		}
		fields := []string{
			"id",
			"download_time",
		}
		order := "id desc"
		odooParams := map[string]interface{}{
			"model":  ODOOModel,
			"domain": domain,
			"fields": fields,
			"order":  order,
		}
		payload := map[string]interface{}{
			"jsonrpc": config.GetConfig().ApiODOO.JSONRPC,
			"params":  odooParams,
		}
		payloadBytes, err := json.Marshal(payload)
		if err != nil {
			return nil, nil, fmt.Errorf("failed to marshal download request: %v", err)
		}

		ODOOresponse, err := GetODOOMSData(string(payloadBytes))
		if err != nil {
			logrus.Errorf("Failed to fetch download data: %v", err)
		} else {
			ODOOResponseArray, ok := ODOOresponse.([]interface{})
			if ok {
				ODOOResponseBytes, _ := json.Marshal(ODOOResponseArray)
				var techDownloadData []OdooTechnicianDownloadItem
				if json.Unmarshal(ODOOResponseBytes, &techDownloadData) == nil {
					for _, download := range techDownloadData {
						if download.DownloadTime.Valid {
							downloadTimes[float64(download.ID)] = &download.DownloadTime.Time
						}
					}
				}
			}
		}
	}

	return loginTimes, downloadTimes, nil
}

// getBatchTechnicianLocations fetches location details for a batch of location IDs
func getBatchTechnicianLocations(locationIDs []float64) (map[int]TechnicianLocationInfo, error) {
	if len(locationIDs) == 0 {
		return nil, nil
	}

	ODOOModel := "fs.technician.location"

	// Convert float64 IDs to int for the domain
	ids := make([]int, len(locationIDs))
	for i, id := range locationIDs {
		ids[i] = int(id)
	}

	domain := []any{
		[]any{"id", "=", ids},
	}

	fields := []string{
		"id",
		"company_id",
		"location_id",
	}

	odooParams := map[string]any{
		"model":  ODOOModel,
		"domain": domain,
		"fields": fields,
	}

	payload := map[string]any{
		"jsonrpc": config.GetConfig().ApiODOO.JSONRPC,
		"params":  odooParams,
	}

	payloadBytes, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal ODOO MS request payload: %v", err)
	}

	ODOOresponse, err := GetODOOMSData(string(payloadBytes))
	if err != nil {
		return nil, fmt.Errorf("failed fetching data from ODOO MS API: %v", err)
	}

	ODOOResponseArray, ok := ODOOresponse.([]interface{})
	if !ok {
		return nil, fmt.Errorf("failed to assert results as []interface{}")
	}

	ODOOResponseBytes, err := json.Marshal(ODOOResponseArray)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal ODOO response: %v", err)
	}

	var locationData []TechnicianLocationItem
	if err := json.Unmarshal(ODOOResponseBytes, &locationData); err != nil {
		return nil, fmt.Errorf("failed to unmarshal ODOO response body: %v", err)
	}

	result := make(map[int]TechnicianLocationInfo)
	for _, item := range locationData {
		_, companyName := parseJSONIDDataCombinedSafe(item.CompanyID)
		_, locationName := parseJSONIDDataCombinedSafe(item.LocationID)

		result[item.ID] = TechnicianLocationInfo{
			CompanyName:  companyName,
			LocationName: locationName,
		}
	}

	return result, nil
}

func CreateTechnicianLoginReport() (string, error) {
	taskDoing := "Create Login Technician Report"
	if !createTechnicianLoginReportMutex.TryLock() {
		return "", fmt.Errorf("%s is already in progress, please wait", taskDoing)
	}
	defer createTechnicianLoginReportMutex.Unlock()

	// TODO: change it soon coz its using for SP check data !!!!
	if err := GetDataTechnicianPlannedForToday(); err != nil {
		return "", fmt.Errorf("failed to get data technician planned for today: %v", err)
	}

	loc, _ := time.LoadLocation(config.GetConfig().Default.Timezone)
	now := time.Now().In(loc)

	startOfDay := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, loc)
	endOfDay := time.Date(now.Year(), now.Month(), now.Day(), 23, 59, 59, 0, loc)

	var technicianRecords []sptechnicianmodel.JOPlannedForTechnicianODOOMS
	result := dbWeb.
		Where("created_at >= ? AND created_at <= ?", startOfDay, endOfDay).
		Order("technician asc").
		Find(&technicianRecords)

	if result.Error != nil {
		return "", fmt.Errorf("failed to fetch technician records: %w", result.Error)
	}

	if result.RowsAffected == 0 {
		return "", fmt.Errorf("no technician records found in range %s → %s: %w",
			startOfDay.Format("2006-01-02 15:04:05"),
			endOfDay.Format("2006-01-02 15:04:05"),
			gorm.ErrRecordNotFound,
		)
	}

	selectedMainDir, err := fun.FindValidDirectory([]string{
		"web/file/technician_report",
		"../web/file/technician_report",
		"../../web/file/technician_report",
	})
	if err != nil {
		return "", fmt.Errorf("failed to find valid directory for technician report: %v", err)
	}

	fileReportDir := filepath.Join(selectedMainDir, now.Format("2006-01-02"))
	if err := os.MkdirAll(fileReportDir, 0755); err != nil {
		return "", fmt.Errorf("failed to create report directory %s: %v", fileReportDir, err)
	}

	reportName := fmt.Sprintf("Technician_Login_Report_%s.xlsx", now.Format("2006-01-02_15-04-05"))

	f := excelize.NewFile()
	sheetMaster := now.Format("02Jan2006_15-04-05")

	f.NewSheet(sheetMaster)
	f.DeleteSheet("Sheet1")

	/* Styles */
	styleTitle, _ := f.NewStyle(&excelize.Style{
		Alignment: &excelize.Alignment{
			Horizontal: "center",
			Vertical:   "center",
		},
		Fill: excelize.Fill{
			Type:    "pattern",
			Color:   []string{"#245F21"},
			Pattern: 1,
		},
		Font: &excelize.Font{
			Bold:  true,
			Color: "#FFFFFF",
		},
	})

	style, _ := f.NewStyle(&excelize.Style{
		Alignment: &excelize.Alignment{
			Horizontal: "center",
			Vertical:   "center",
		},
	})

	titlesMaster := []struct {
		Title   string
		Width   float64
		Comment string
	}{
		{"Name", 35, "Nama lengkap teknisi"},
		{"Technician", 35, "Nama teknisi di aplikasi FS"},
		{"Region", 15, "Wilayah teknisi"},
		{"SPL", 35, "Supervisor"},
		{"SAC", 35, "Leader"},
		{"Last Download JO", 27, "Waktu terakhir teknisi mengunduh JO / sync data"},
		{"Last Login", 27, "Waktu terakhir teknisi login ke aplikasi FS"},
		{"Status", 30, "Status login teknisi"},
		{"First Upload", 27, "Waktu pertama kali teknisi mengunggah data pengerjaan dari APK FS / kunjungan pertama kali"},
		{"Last Visit", 27, "Waktu terakhir teknisi melakukan kunjungan"},
	}
	var columnsMaster []ExcelColumnWithComment
	for i, t := range titlesMaster {
		columnsMaster = append(columnsMaster, ExcelColumnWithComment{
			ColIndex:   fun.GetColName(i),
			ColTitle:   t.Title,
			ColSize:    t.Width,
			ColComment: t.Comment,
		})
	}
	for _, col := range columnsMaster {
		cell := fmt.Sprintf("%s1", col.ColIndex)
		f.SetCellValue(sheetMaster, cell, col.ColTitle)
		f.SetColWidth(sheetMaster, col.ColIndex, col.ColIndex, col.ColSize)
		f.SetCellStyle(sheetMaster, cell, cell, styleTitle)
		if col.ColComment != "" {
			if err := f.AddComment(sheetMaster, excelize.Comment{
				Cell:   cell,
				Author: "Service Report",
				Paragraph: []excelize.RichTextRun{
					{Text: "Service Report:\n", Font: &excelize.Font{Bold: true, Color: "#000000"}},
					{Text: col.ColComment, Font: &excelize.Font{Color: "#000000"}},
				},
				// Width:  300,
				Height: 250,
			}); err != nil {
				logrus.Errorf("Failed to add comment to cell %s: %v", cell, err)
			}
		}
	}

	lastColMaster := fun.GetColName(len(columnsMaster) - 1)
	filterRangeMaster := fmt.Sprintf("A1:%s1", lastColMaster)
	f.AutoFilter(sheetMaster, filterRangeMaster, []excelize.AutoFilterOptions{})

	rowIndex := 2
	for _, record := range technicianRecords {
		for _, column := range columnsMaster {
			cell := fmt.Sprintf("%s%d", column.ColIndex, rowIndex)
			var value interface{} = "N/A"
			var needToSetValue bool = true

			switch column.ColTitle {
			case "Name":
				value = record.Name
			case "Technician":
				value = record.Technician
			case "Region":
				techGroup, err := techGroup(record.Technician)
				if err != nil {
					logrus.Errorf("Failed to get technician group for %s: %v", record.Technician, err)
					value = "N/A"
				} else {
					value = techGroup
				}
			case "SPL":
				value = record.SPL
			case "SAC":
				value = record.SAC
			case "Last Download JO":
				if record.TechnicianLastDownloadJO != nil {
					value = record.TechnicianLastDownloadJO.Format("2006-01-02 15:04:05")
				}
			case "Last Login":
				if record.TechnicianLastLogin != nil {
					value = record.TechnicianLastLogin.Format("2006-01-02 15:04:05")
				}
			case "Status":
				var status string
				var styleID int

				todayStart := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, loc)
				lastLogin := record.TechnicianLastLogin
				lastDownload := record.TechnicianLastDownloadJO

				// Default style (normal)
				styleID = style

				// Red style for warning cases
				redStyle, err := f.NewStyle(&excelize.Style{
					Alignment: &excelize.Alignment{
						Horizontal: "center",
						Vertical:   "center",
					},
					Font: &excelize.Font{
						Color: "#FF0000",
					},
				})
				if err != nil {
					logrus.Errorf("Failed to create red style: %v", err)
				}

				switch {
				case lastLogin == nil && lastDownload == nil:
					status = "This technician has never logged in."
					styleID = redStyle
				case lastLogin == nil && lastDownload != nil:
					status = "Technician has never logged in, but has a download history."
					styleID = redStyle
					if lastDownload.Before(todayStart) {
						status += fmt.Sprintf(" Last download was on: %s", lastDownload.Format("2006-01-02 15:04:05"))
					} else {
						status += " Last download was today."
					}
				case lastLogin != nil:
					if lastLogin.Before(todayStart) {
						if lastDownload != nil && lastDownload.After(todayStart) {
							status = "Logged in"
						} else {
							status = fmt.Sprintf("This technician did not log in today. Last login was on: %s",
								lastLogin.Format("2006-01-02 15:04:05"))
							styleID = redStyle
						}
					} else {
						status = "Logged in"
					}
				}

				value = status
				f.SetCellValue(sheetMaster, cell, value)
				f.SetCellStyle(sheetMaster, cell, cell, styleID)
				needToSetValue = false // prevent double-setting later

			case "First Upload":
				if record.TechnicianFirstUpload != nil {
					value = record.TechnicianFirstUpload.Format("2006-01-02 15:04:05")
				}
			case "Last Visit":
				if record.TechnicianLastVisit != nil {
					value = record.TechnicianLastVisit.Format("2006-01-02 15:04:05")
				}
			}

			if value == nil || value == "" {
				value = "N/A"
			}

			if needToSetValue {
				f.SetCellValue(sheetMaster, cell, value)
				f.SetCellStyle(sheetMaster, cell, cell, style)
			}
		}
		rowIndex++
	}

	excelFilePath := filepath.Join(fileReportDir, reportName)
	if err := f.SaveAs(excelFilePath); err != nil {
		return "", fmt.Errorf("failed to save Excel file %s: %v", excelFilePath, err)
	}

	return excelFilePath, nil
}

func SendTechnicianLoginReport() error {
	taskDoing := "Send Technician Login Report"
	if !sendTechnicianLoginReportMutex.TryLock() {
		return fmt.Errorf("%s is already in progress, please wait", taskDoing)
	}
	defer sendTechnicianLoginReportMutex.Unlock()

	reportFile, err := CreateTechnicianLoginReport()
	if err != nil {
		return fmt.Errorf("failed to create technician login report: %v", err)
	}

	var sb strings.Builder
	sb.WriteString("<mjml>")
	sb.WriteString(`
  <mj-head>
    <mj-preview>Technician Login Report</mj-preview>
    <mj-style inline="inline">
    .body-section {
      background-color: #ffffff;
      padding: 30px;
      border-radius: 12px;
      box-shadow: 0 2px 8px rgba(0, 0, 0, 0.08);
    }
    .footer-text {
      color: #6b7280;
      font-size: 12px;
      text-align: center;
      padding-top: 10px;
      border-top: 1px solid #e5e7eb;
    }
    .header-title {
      font-size: 32px;
      font-weight: bold;
      color: #1E293B;
      text-align: left;
    }
    .cta-button {
      background-color: #6D28D9;
      color: #ffffff;
      padding: 12px 24px;
      border-radius: 8px;
      font-weight: bold;
      text-align: center;
      display: inline-block;
    }
    .email-info {
      color: #374151;
      font-size: 16px;
    }
    </mj-style>
  </mj-head>
`)

	sb.WriteString(fmt.Sprintf(`
  <mj-body background-color="#f8fafc">
    <mj-section css-class="body-section" padding="20px">
      <mj-column>
        <mj-text font-size="20px" color="#1E293B" font-weight="bold">Dear All,</mj-text>
        <mj-text font-size="16px" color="#4B5563" line-height="1.6">
          Attached is the technician login report for %s.<br>
          Please see the attached file for full details.
        </mj-text>

        <mj-divider border-color="#e5e7eb"></mj-divider>

        <mj-text font-size="16px" color="#374151">
          Best Regards,<br>
          <b><i>%s - Service Report</i></b>
        </mj-text>
      </mj-column>
    </mj-section>

    <mj-section>
      <mj-column>
        <mj-text css-class="footer-text">
          ⚠ This is an automated email. Please do not reply directly.
        </mj-text>
      </mj-column>
    </mj-section>
  </mj-body>
  `,
		time.Now().Format("2006-01-02"),
		config.GetConfig().Default.PT,
	))
	sb.WriteString("</mjml>")

	mjmlTemplate := sb.String()

	emailTo := config.GetConfig().Report.TechnicianLogin.To
	emailCc := config.GetConfig().Report.TechnicianLogin.Cc
	subject := fmt.Sprintf("Technician Login Report - %s", time.Now().Format("2006-01-02 15:04:05"))

	attachments := []fun.EmailAttachment{
		{
			FilePath:    reportFile,
			NewFileName: fmt.Sprintf("Technician_Login_Report_%s.xlsx", time.Now().Format("2006-01-02_15-04-05")),
		},
	}

	err = fun.TrySendEmail(emailTo, emailCc, nil, subject, mjmlTemplate, attachments)
	if err != nil {
		return fmt.Errorf("failed to send technician login report email: %v", err)
	}

	// Cleanup: remove the report file after sending
	if err := os.Remove(reportFile); err != nil {
		logrus.Errorf("Failed to remove report file %s: %v", reportFile, err)
	}

	return nil
}

func IncrementNomorSuratSP(db *gorm.DB, id string) (int, error) {
	var nomorSurat sptechnicianmodel.NomorSuratSP

	// Find or create the row for given ID
	if err := db.FirstOrCreate(&nomorSurat, sptechnicianmodel.NomorSuratSP{
		ID: id,
	}).Error; err != nil {
		return 0, err
	}

	// Increment
	nomorSurat.LastNomorSuratSP++

	// Save update
	if err := db.Save(&nomorSurat).Error; err != nil {
		return 0, err
	}

	return nomorSurat.LastNomorSuratSP, nil
}

// DEPRECATED: using the new v2
// func CheckSPTechnician() error {
// 	taskDoing := "Check SP for Technician"
// 	logrus.Infof("Starting task: %s", taskDoing)

// 	if !checkSPTechnicianMutex.TryLock() {
// 		return fmt.Errorf("%s is already in progress, please wait", taskDoing)
// 	}
// 	defer checkSPTechnicianMutex.Unlock()

// 	if err := GetDataTechnicianPlannedForToday(); err != nil {
// 		return fmt.Errorf("failed to get data technician planned for today: %v", err)
// 	}

// 	dbWeb := gormdb.Databases.Web

// 	loc, _ := time.LoadLocation(config.GetConfig().Default.Timezone)
// 	now := time.Now().In(loc)
// 	hour := now.Hour()
// 	// Greeting logic (ensuring correct 24-hour format)
// 	var greetingID, greetingEN string
// 	if hour >= 0 && hour < 4 {
// 		greetingID = "Selamat Dini Hari" // 00:00 - 03:59
// 		greetingEN = "Good Early Morning"
// 	} else if hour >= 4 && hour < 12 {
// 		greetingID = "Selamat Pagi" // 04:00 - 11:59
// 		greetingEN = "Good Morning"
// 	} else if hour >= 12 && hour < 15 {
// 		greetingID = "Selamat Siang" // 12:00 - 14:59
// 		greetingEN = "Good Afternoon"
// 	} else if hour >= 15 && hour < 17 {
// 		greetingID = "Selamat Sore" // 15:00 - 16:59
// 		greetingEN = "Good Late Afternoon"
// 	} else if hour >= 17 && hour < 19 {
// 		greetingID = "Selamat Petang" // 17:00 - 18:59
// 		greetingEN = "Good Evening"
// 	} else {
// 		greetingID = "Selamat Malam" // 19:00 - 23:59
// 		greetingEN = "Good Night"
// 	}

// 	startOfDay := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, loc)
// 	endOfDay := time.Date(now.Year(), now.Month(), now.Day(), 23, 59, 59, 0, loc)

// 	var dataJOPlanned []sptechnicianmodel.JOPlannedForTechnicianODOOMS
// 	result := dbWeb.
// 		Where("created_at >= ? AND created_at <= ?", startOfDay, endOfDay).
// 		Order("technician asc").
// 		Find(&dataJOPlanned)

// 	if result.Error != nil {
// 		return fmt.Errorf("failed to fetch JO Planned for Technician: %v", result.Error)
// 	}

// 	if result.RowsAffected == 0 {
// 		return fmt.Errorf("no JO Planned for Technician found in range %s → %s: %w",
// 			startOfDay.Format("2006-01-02 15:04:05"),
// 			endOfDay.Format("2006-01-02 15:04:05"),
// 			gorm.ErrRecordNotFound,
// 		)
// 	}

// 	forProject := "ODOO MS"
// 	SPLGotSPToday := make(map[string]SPInfoForSPL)
// 	for _, record := range dataJOPlanned {
// 		if record.Technician == "" {
// 			continue // SKIP if technician name is empty
// 		}

// 		excludedTechnicians := []string{
// 			"*",
// 			"inhouse",
// 			"in house",
// 			"in-house",
// 		}

// 		atmDedicatedTechnician := config.GetConfig().SPTechnician.ATMDedicatedTechnician
// 		if len(atmDedicatedTechnician) > 0 {
// 			excludedTechnicians = append(excludedTechnicians, atmDedicatedTechnician...)
// 		}

// 		skip := false
// 		for _, exclude := range excludedTechnicians {
// 			if strings.Contains(strings.TrimSpace(strings.ToLower(record.Technician)), strings.ToLower(exclude)) {
// 				skip = true
// 				break
// 			}
// 		}

// 		if skip {
// 			continue // SKIP this record
// 		}

// 		// Reset SP status before checking, coz technician might have back the SP 1
// 		if err := ResetTechnicianSP(record.Technician, forProject); err != nil {
// 			if !errors.Is(err, gorm.ErrRecordNotFound) {
// 				logrus.Warnf("Failed to reset SP for technician %s: %v", record.Technician, err)
// 			}
// 		}

// 		// Reset SP status for SPL too
// 		if record.SPL != "" {
// 			if err := ResetSPLSP(record.SPL, forProject); err != nil {
// 				if !errors.Is(err, gorm.ErrRecordNotFound) {
// 					logrus.Warnf("Failed to reset SP for SPL %s: %v", record.SPL, err)
// 				}
// 			}
// 		}

// 		var totalJO int
// 		if len(record.WONumber) > 0 {
// 			var woNumbers []string
// 			if err := json.Unmarshal(record.WONumber, &woNumbers); err == nil {
// 				totalJO = len(woNumbers)
// 			} else {
// 				logrus.Errorf("Failed to unmarshal WONumber for technician %s: %v", record.Technician, err)
// 			}
// 		}

// 		if totalJO == 0 {
// 			continue // SKIP if no JO planned today
// 		}

// 		todayStart := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, loc)
// 		lastLogin := record.TechnicianLastLogin
// 		lastDownload := record.TechnicianLastDownloadJO

// 		if record.SPL == "" {
// 			// logrus.Warnf("Technician %s has no SPL assigned, skipping SP check", record.Technician)
// 			continue // SKIP if technician has no SPL assigned
// 		}
// 		var tech sptechnicianmodel.JOPlannedForTechnicianODOOMS
// 		result = dbWeb.
// 			Where("LOWER(technician) = ?", strings.ToLower(record.SPL)).
// 			Where("created_at >= ? AND created_at <= ?", startOfDay, endOfDay).
// 			First(&tech)
// 		if result.Error != nil {
// 			if errors.Is(result.Error, gorm.ErrRecordNotFound) {
// 				continue // Skip if no data found for SPL
// 			} else {
// 				logrus.Errorf("Failed to fetch data for SPL %s: %v", record.SPL, result.Error)
// 				continue // Skip if error occurs
// 			}
// 		}

// 		var namaSPL string
// 		if tech.Name != "" {
// 			namaSPL = tech.Name
// 		} else {
// 			namaSPL = tech.Technician
// 		}

// 		var namaTeknisi string
// 		if record.Name != "" {
// 			namaTeknisi = record.Name
// 		} else {
// 			namaTeknisi = record.Technician
// 		}

// 		if namaTeknisi == "" || namaSPL == "" {
// 			continue // Skip if technician or SPL name is empty
// 		}

// 		if namaTeknisi == namaSPL {
// 			// logrus.Warnf("Technician %s is also the SPL, skipping SP check", namaTeknisi)
// 			continue // SKIP if technician is also the SPL
// 		}

// 		namaTeknisi = fun.CapitalizeWord(namaTeknisi)
// 		namaSPL = fun.CapitalizeWord(namaSPL)

// 		noHPSPL := tech.NoHPTechnician // Assuming NoHPTechnician is the phone number for SPL
// 		var sanitizedNOHPSPL string
// 		if noHPSPL != "" {
// 			sanitizedPhoneNumber, err := fun.SanitizePhoneNumber(noHPSPL)
// 			if err != nil {
// 				logrus.Errorf("Failed to sanitize phone number for SPL %s: %v", record.SPL, err)
// 				continue // SKIP if phone number is invalid
// 			}
// 			sanitizedNOHPSPL = sanitizedPhoneNumber
// 		}

// 		var technicianIsLoginToday bool = false
// 		var pelanggaranID string
// 		var pelanggaranEN string
// 		switch {
// 		case lastLogin == nil && lastDownload == nil:
// 			technicianIsLoginToday = false
// 			pelanggaranID = "tidak pernah login ke aplikasi FS & tidak pernah mengunduh data JO."
// 			pelanggaranEN = "never logged in to the FS application & never downloaded JO data."
// 		case lastLogin == nil && lastDownload != nil:
// 			if lastDownload.Before(todayStart) {
// 				technicianIsLoginToday = false
// 				pelanggaranID = fmt.Sprintf("tidak pernah login ke aplikasi FS. Terakhir mengunduh data JO pada: %s.", lastDownload.Format("2006-01-02 15:04:05"))
// 				pelanggaranEN = fmt.Sprintf("never logged in to the FS application. Last downloaded JO data on: %s.", lastDownload.Format("2006-01-02 15:04:05"))
// 			} else {
// 				technicianIsLoginToday = true
// 			}
// 		case lastLogin != nil:
// 			if lastLogin.Before(todayStart) {
// 				if lastDownload != nil && lastDownload.After(todayStart) {
// 					technicianIsLoginToday = true
// 				} else {
// 					technicianIsLoginToday = false
// 					var tglTidakLoginFormatted string
// 					tglTidakLogin, err := tanggal.Papar(now, "Jakarta", tanggal.WIB)
// 					if err != nil {
// 						logrus.Errorf("Failed to format date for SP message: %v", err)
// 						continue
// 					}
// 					tglTidakLoginFormatted = tglTidakLogin.Format(" ", []tanggal.Format{
// 						tanggal.Hari,      // 27
// 						tanggal.NamaBulan, // Maret
// 						tanggal.Tahun,     // 2025
// 					})

// 					pelanggaranID = fmt.Sprintf("tidak login pada %s. Terakhir login: %s.", tglTidakLoginFormatted, lastLogin.Format("2006-01-02 15:04"))
// 					pelanggaranEN = fmt.Sprintf("did not log in on %s. Last login: %s.", tglTidakLoginFormatted, lastLogin.Format("2006-01-02 15:04"))
// 				}
// 			} else {
// 				technicianIsLoginToday = true
// 			}
// 		}

// 		if record.TechnicianLastVisit == nil {
// 			var tglTidakKerja string
// 			tgl, err := tanggal.Papar(now, "Jakarta", tanggal.WIB)
// 			if err != nil {
// 				logrus.Errorf("Failed to format date for SP message: %v", err)
// 				continue
// 			}
// 			tglTidakKerja = tgl.Format(" ", []tanggal.Format{
// 				tanggal.NamaHariDenganKoma, // Kamis,
// 				tanggal.Hari,               // 27
// 				tanggal.NamaBulan,          // Maret
// 				tanggal.Tahun,              // 2025
// 			})

// 			pelanggaranID += fmt.Sprintf(" Serta tidak melakukan kunjungan kerja pada %s.", tglTidakKerja)
// 			pelanggaranEN += fmt.Sprintf(" Also, did not make a work visit on %s.", tglTidakKerja)
// 		}

// 		pelanggaranID = fun.CapitalizeFirstWord(pelanggaranID)
// 		pelanggaranEN = fun.CapitalizeFirstWord(pelanggaranEN)

// 		var totalJOVisited int
// 		if len(record.WONumberVisited) > 0 {
// 			var woVisited []string
// 			if err := json.Unmarshal(record.WONumberVisited, &woVisited); err == nil {
// 				totalJOVisited = len(woVisited)
// 			} else {
// 				logrus.Errorf("Failed to unmarshal WOVisited for technician %s: %v", record.Technician, err)
// 			}
// 		}
// 		minJoVisited := config.GetConfig().SPTechnician.MinimumJOVisited
// 		if minJoVisited <= 0 {
// 			minJoVisited = 1 // default minimum JO visited
// 		}

// 		if technicianIsLoginToday {
// 			if totalJOVisited >= minJoVisited {
// 				logrus.Infof("Technician %s has visited %d JO(s) from %d JO(s) planned, which meets or exceeds the minimum requirement of %d JO(s). No SP needed.",
// 					record.Technician,
// 					totalJOVisited,
// 					totalJO,
// 					minJoVisited,
// 				)
// 				continue // SKIP if technician has visited enough JO
// 			}
// 		}

// 		// Create audio directory for SP technician
// 		// Assuming the audio files are stored in a specific directory structure
// 		audioSPMainDir, err := fun.FindValidDirectory([]string{
// 			"web/file/sounding_sp_technician",
// 			"../web/file/sounding_sp_technician",
// 			"../../web/file/sounding_sp_technician",
// 		})
// 		if err != nil {
// 			logrus.Errorf("Failed to find valid directory for SP audio files: %v", err)
// 			continue
// 		}
// 		audioForSPDir := filepath.Join(audioSPMainDir, now.Format("2006-01-02"))
// 		if err := os.MkdirAll(audioForSPDir, 0755); err != nil {
// 			logrus.Errorf("Failed to create audio directory %s: %v", audioForSPDir, err)
// 			continue
// 		}
// 		speech := htgotts.Speech{Folder: audioForSPDir, Language: voices.Indonesian, Handler: &handlers.Native{}}

// 		maxResponseSPAtHour := config.GetConfig().SPTechnician.MaxResponseSPAtHour // (7 PM on the same day)
// 		dataHRD := config.GetConfig().Default.PTHRD
// 		var jidStrSAC string
// 		ODOOMSSAC := config.GetConfig().ODOOMSSAC
// 		SACData, ok := ODOOMSSAC[record.SAC]
// 		if !ok {
// 			logrus.Warnf("No SAC data found for SAC username %s, skipping WhatsApp notification", record.SAC)
// 			// continue // Skip if no SAC data found
// 		} else {
// 			jidStrSAC = fmt.Sprintf("%s@%s", SACData.PhoneNumber, "s.whatsapp.net")
// 		}
// 		// ADD: sent to Mr. Oliver too if needed
// 		// jidStrOliver := fmt.Sprintf("%s@%s", config.GetConfig().Whatsmeow.WaOliver, "s.whatsapp.net")

// 		var spIsProcessing = false
// 		var dataSP sptechnicianmodel.TechnicianGotSP
// 		result := dbWeb.Where("technician = ? AND for_project = ?", record.Technician, forProject).First(&dataSP)
// 		if result.Error != nil {
// 			if errors.Is(result.Error, gorm.ErrRecordNotFound) {
// 				// No data SP found for technician, proceed to check if SP 1 must be sent
// 				// Technician GOT SP 1
// 				if !technicianIsLoginToday {
// 					// Create a sound for SP 1
// 					SP1TextPart1 := fmt.Sprintf("Berikut kami sampaikan bahwa saudara %s.", namaTeknisi)
// 					SP1TextPart2 := "Menerima Surat Peringatan (SP-1)."
// 					SP1TextPart3 := "Mohon menjadi perhatian dan segera melakukan perbaikan yang diperlukan."
// 					SP1TextPart4 := "terima kasih..."

// 					fileNameForSP1 := fmt.Sprintf("%s_SP1", strings.ReplaceAll(record.Technician, "*", "Resigned"))

// 					fileTTS, err := fun.CreateRobustTTS(speech, audioForSPDir, []string{SP1TextPart1, SP1TextPart2, SP1TextPart3, SP1TextPart4}, fileNameForSP1)
// 					if err != nil {
// 						logrus.Errorf("Failed to create merged SP1 TTS file: %v", err)
// 						continue
// 					}

// 					// Debug: Check the created file
// 					if fileTTS != "" {
// 						fileInfo, statErr := os.Stat(fileTTS)
// 						if statErr == nil {
// 							logrus.Debugf("🔊 SP1 merged TTS file created: %s, Size: %d bytes", fileTTS, fileInfo.Size())
// 						} else {
// 							logrus.Errorf("🔊 SP1 TTS file stat error: %v", statErr)
// 						}
// 					}

// 					// For SP-1
// 					noSurat, err := IncrementNomorSuratSP(dbWeb, "LAST_NOMOR_SURAT_SP1_GENERATED")
// 					if err != nil {
// 						logrus.Errorf("Failed to increment nomor surat SP-1: %v", err)
// 						continue
// 					}
// 					var noSuratStr string
// 					if noSurat < 1000 {
// 						noSuratStr = fmt.Sprintf("%03d", noSurat)
// 					} else {
// 						noSuratStr = fmt.Sprintf("%d", noSurat)
// 					}

// 					monthRoman, err := fun.MonthToRoman(int(now.Month()))
// 					if err != nil {
// 						logrus.Errorf("Failed to convert month to roman numeral: %v", err)
// 						continue
// 					}
// 					splCity := getSPLCity(record.SPL)
// 					if splCity == "" {
// 						logrus.Warnf("SPL city not found for SPL %s, defaulting to 'Unknown'", record.SPL)
// 						splCity = "Unknown"
// 					}
// 					tanggalSP1Terbit, err := tanggal.Papar(now, "Jakarta", tanggal.WIB)
// 					if err != nil {
// 						logrus.Errorf("Failed to get formatted date for SP1: %v", err)
// 						continue
// 					}
// 					tglSP1Diterbitkan := tanggalSP1Terbit.Format(" ", []tanggal.Format{
// 						tanggal.Hari,      // 27
// 						tanggal.NamaBulan, // Maret
// 						tanggal.Tahun,     // 2025
// 					})

// 					// Simple replacements map
// 					placeholdersSP1 := map[string]string{
// 						"$nomor_surat":            noSuratStr,
// 						"$bulan_romawi":           monthRoman,
// 						"$tahun_sp":               now.Format("2006"),
// 						"$nama_spl":               namaSPL,
// 						"$jabatan_spl":            fmt.Sprintf("Service Point Leader %s", splCity),
// 						"$pelanggaran_karyawan":   pelanggaranID,
// 						"$nama_teknisi":           namaTeknisi,
// 						"$tanggal_sp_diterbitkan": tglSP1Diterbitkan,
// 						"$personalia_name":        config.GetConfig().Default.PTHRD[1].Name, // Assuming the 2nd HRD is Personalia
// 						"$sac_name":               SACData.FullName,
// 						"$sac_ttd":                SACData.TTDPath,
// 					}
// 					selectedMainDir, err := fun.FindValidDirectory([]string{
// 						"web/file/sp_technician",
// 						"../web/file/sp_technician",
// 						"../../web/file/sp_technician",
// 					})

// 					if err != nil {
// 						logrus.Errorf("Failed to find valid directory for SP technician PDF files: %v", err)
// 						continue
// 					}
// 					fileDir := filepath.Join(selectedMainDir, time.Now().Format("2006-01-02"))
// 					if err := os.MkdirAll(fileDir, 0755); err != nil {
// 						logrus.Errorf("Failed to create PDF directory %s: %v", fileDir, err)
// 						continue
// 					}

// 					pdfFileName := fmt.Sprintf("SP_1_%s_%s.pdf",
// 						strings.ReplaceAll(record.Technician, "*", "Resigned"),
// 						now.Format("2006-01-02"),
// 					)
// 					pdfSP1FilePath := filepath.Join(fileDir, pdfFileName)

// 					if err := CreatePDFSP1ForTechnician(placeholdersSP1, pdfSP1FilePath); err != nil {
// 						logrus.Errorf("Failed to create PDF for SP 1: %v", err)
// 						continue
// 					}

// 					spIsProcessing = true // Mark as processing SP for the 1st time
// 					gotSP1At := time.Now()

// 					dataSPTechnician := sptechnicianmodel.TechnicianGotSP{
// 						Technician:      record.Technician,
// 						Name:            record.Name,
// 						ForProject:      forProject,
// 						IsGotSP1:        true, // Assume technician should get SP 1
// 						GotSP1At:        &gotSP1At,
// 						PelanggaranSP1:  pelanggaranID,
// 						SP1SoundTTSPath: fileTTS,
// 						SP1FilePath:     pdfSP1FilePath,
// 					}

// 					if err := dbWeb.Create(&dataSPTechnician).Error; err != nil {
// 						logrus.Errorf("Failed to create SP data for technician %s: %v", record.Technician, err)
// 						continue
// 					}

// 					if sanitizedNOHPSPL != "" {
// 						// Make sure your phone number is Indonesian format
// 						jidStrSPL := fmt.Sprintf("62%s@%s", sanitizedNOHPSPL, "s.whatsapp.net")

// 						var sbID strings.Builder
// 						sbID.WriteString(fmt.Sprintf("Dengan ini, kami menyampaikan bahwa saudara *%s* menerima Surat Peringatan (SP) 1.\n", namaTeknisi))
// 						sbID.WriteString(fmt.Sprintf("Sehubungan dengan sikap tidak disiplin/ pelanggaran terhadap tata tertib Perusahaan yang Karyawan lakukan yaitu:\n\n*%s*\n\n", pelanggaranID))
// 						sbID.WriteString("Mohon menjadi perhatian dan segera melakukan perbaikan yang diperlukan.\n")
// 						sbID.WriteString("Terima kasih.")

// 						var sbEN strings.Builder
// 						sbEN.WriteString(fmt.Sprintf("We hereby inform you that Mr. *%s* has received Warning Letter (SP) 1.\n", namaTeknisi))
// 						sbEN.WriteString(fmt.Sprintf("In connection with the attitude of indiscipline/ violation of the Company regulations that the Employee has committed, namely:\n\n*%s*\n\n", pelanggaranEN))
// 						sbEN.WriteString("Please pay attention and immediately make the necessary improvements.\n")
// 						sbEN.WriteString("Thank you.")

// 						var msgIDsb strings.Builder
// 						var msgENsb strings.Builder
// 						msgIDsb.WriteString(fmt.Sprintf("Halo, %s Pak %s.\n\n", greetingID, namaSPL))
// 						msgIDsb.WriteString(sbID.String())
// 						msgENsb.WriteString(fmt.Sprintf("Hello, %s Mr. %s.\n\n", greetingEN, namaSPL))
// 						msgENsb.WriteString(sbEN.String())

// 						idMsg := msgIDsb.String()
// 						enMsg := msgENsb.String()

// 						sendLangDocumentMessageForSPTechnician(
// 							forProject,
// 							record.Technician,
// 							jidStrSPL,
// 							idMsg,
// 							enMsg,
// 							"id",
// 							pdfSP1FilePath,
// 							1,
// 							"62"+sanitizedNOHPSPL,
// 						)

// 						// ADD: SP send to Mr. Oliver if needed
// 						if !config.GetConfig().SPTechnician.ActiveDebug {
// 							for _, hrd := range dataHRD {
// 								var msgIDsb strings.Builder
// 								var msgENsb strings.Builder
// 								msgIDsb.WriteString(fmt.Sprintf("Halo, %s Kak %s.\n\n", greetingID, hrd.Name))
// 								msgIDsb.WriteString(sbID.String())
// 								msgENsb.WriteString(fmt.Sprintf("Hello, %s Sist %s.\n\n", greetingEN, hrd.Name))
// 								msgENsb.WriteString(sbEN.String())

// 								idMsg := msgIDsb.String()
// 								enMsg := msgENsb.String()

// 								jidStrHRD := fmt.Sprintf("%s@%s", hrd.PhoneNumber, "s.whatsapp.net")
// 								sendLangDocumentMessageForSPTechnician(
// 									forProject,
// 									record.Technician,
// 									jidStrHRD,
// 									idMsg, enMsg, "id",
// 									pdfSP1FilePath, 1,
// 									hrd.PhoneNumber)
// 							}
// 							if jidStrSAC != "" {
// 								if strings.Contains(strings.ToLower(SACData.FullName), "tetty") {
// 									var msgIDsb strings.Builder
// 									var msgENsb strings.Builder
// 									msgIDsb.WriteString(fmt.Sprintf("Halo, %s Kak %s.\n\n", greetingID, SACData.FullName))
// 									msgIDsb.WriteString(sbID.String())
// 									msgENsb.WriteString(fmt.Sprintf("Hello, %s Sist %s.\n\n", greetingEN, SACData.FullName))
// 									msgENsb.WriteString(sbEN.String())

// 									idMsg := msgIDsb.String()
// 									enMsg := msgENsb.String()

// 									jidStrSAC := fmt.Sprintf("%s@%s", SACData.PhoneNumber, "s.whatsapp.net")
// 									sendLangDocumentMessageForSPTechnician(forProject, record.Technician, jidStrSAC, idMsg, enMsg, "id", pdfSP1FilePath, 1, SACData.PhoneNumber)
// 								} else {
// 									var msgIDsb strings.Builder
// 									var msgENsb strings.Builder
// 									msgIDsb.WriteString(fmt.Sprintf("Halo, %s Pak %s.\n\n", greetingID, SACData.FullName))
// 									msgIDsb.WriteString(sbID.String())
// 									msgENsb.WriteString(fmt.Sprintf("Hello, %s Mr. %s.\n\n", greetingEN, SACData.FullName))
// 									msgENsb.WriteString(sbEN.String())

// 									idMsg := msgIDsb.String()
// 									enMsg := msgENsb.String()

// 									jidStrSAC := fmt.Sprintf("%s@%s", SACData.PhoneNumber, "s.whatsapp.net")
// 									sendLangDocumentMessageForSPTechnician(forProject, record.Technician, jidStrSAC, idMsg, enMsg, "id", pdfSP1FilePath, 1, SACData.PhoneNumber)
// 								}
// 							}
// 						}

// 						logrus.Infof("SP 1 of Technician %s has been sent", record.Technician)
// 					} else {
// 						logrus.Warnf("No valid phone number for SPL %s, cannot send SP 1", record.SPL)
// 						continue
// 					}
// 				}
// 			} else {
// 				logrus.Errorf("Failed to fetch SP data for technician %s: %v", record.Technician, result.Error)
// 				continue
// 			}
// 		}

// 		if !spIsProcessing {
// 			// Check if technician has already got SP before
// 			techGotSP1 := dataSP.IsGotSP1
// 			techGotSP2 := dataSP.IsGotSP2
// 			techGotSP3 := dataSP.IsGotSP3

// 			// Technician GOT SP 2
// 			if techGotSP1 &&
// 				!techGotSP2 &&
// 				!techGotSP3 {

// 				// 1. Find the first SP1 message to get the sent time
// 				var firstSP1Message sptechnicianmodel.SPWhatsAppMessage
// 				if err := dbWeb.
// 					Where("technician_got_sp_id = ? AND number_of_sp = ?", dataSP.ID, 1).
// 					Order("whatsapp_sent_at asc").
// 					First(&firstSP1Message).Error; err != nil {
// 					logrus.Warnf("Could not find the first SP1 message for technician %s to determine sent time: %v", record.Technician, err)
// 					continue
// 				}

// 				if firstSP1Message.WhatsappSentAt == nil {
// 					logrus.Warnf("SP1 Whatsapp sent time is nil for technician %s, cannot create SP2", record.Technician)
// 					continue
// 				}

// 				// 2. Calculate the reply deadline
// 				sentAt := *firstSP1Message.WhatsappSentAt
// 				deadline := time.Date(sentAt.Year(), sentAt.Month(), sentAt.Day(), maxResponseSPAtHour, 0, 0, 0, sentAt.Location())

// 				// 3. Count replies received before the deadline
// 				var onTimeReplyCount int64
// 				dbWeb.Model(&sptechnicianmodel.SPWhatsAppMessage{}).
// 					Where("technician_got_sp_id = ?", dataSP.ID).
// 					Where("number_of_sp = ?", 1).
// 					Where("for_project = ?", forProject).
// 					Where("whatsapp_reply_text IS NOT NULL AND whatsapp_reply_text != ''").
// 					Where("whatsapp_replied_at <= ?", deadline).
// 					Count(&onTimeReplyCount)

// 				// 4. Proceed only if no on-time replies were found
// 				if onTimeReplyCount == 0 {
// 					// Technician did not reply in time, so we can proceed with SP2.
// 					if !technicianIsLoginToday {
// 						tgl, err := tanggal.Papar(sentAt, "Jakarta", tanggal.WIB)
// 						if err != nil {
// 							logrus.Errorf("Failed to format date for SP 2: %v", err)
// 							continue
// 						}
// 						tanggalSP1Terkirim := tgl.Format(" ", []tanggal.Format{
// 							tanggal.NamaHariDenganKoma, // Kamis,
// 							tanggal.Hari,               // 27
// 							tanggal.NamaBulan,          // Maret
// 							tanggal.Tahun,              // 2025
// 							tanggal.PukulDenganDetik,
// 							tanggal.ZonaWaktu,
// 						})

// 						SP2TextPart1 := fmt.Sprintf("Sehubungan dengan Surat Peringatan (SP-1) yang sebelumnya disampaikan kepada saudara %s pada %s", namaTeknisi, tanggalSP1Terkirim)
// 						SP2TextPart2 := "perusahaan kemudian memutuskan untuk menindaklanjuti melalui Surat Peringatan (SP-2)."
// 						SP2TextPart3 := "Hal ini didasari oleh belum adanya perbaikan atau tindakan korektif yang memadai terkait pelanggaran yang telah dilaporkan."
// 						SP2TextPart4 := "Mohon menjadi perhatian dan segera melakukan perbaikan yang diperlukan. terima kasih..."

// 						fileNameForSP2 := fmt.Sprintf("%s_SP2", strings.ReplaceAll(record.Technician, "*", "Resigned"))

// 						// Use robust TTS creation for multiple parts
// 						fileTTS, err := fun.CreateRobustTTS(speech, audioForSPDir, []string{SP2TextPart1, SP2TextPart2, SP2TextPart3, SP2TextPart4}, fileNameForSP2)
// 						if err != nil {
// 							logrus.Errorf("Failed to create merged SP2 TTS file: %v", err)
// 							continue
// 						}

// 						// Debug: Check the created file
// 						if fileTTS != "" {
// 							fileInfo, statErr := os.Stat(fileTTS)
// 							if statErr == nil {
// 								logrus.Debugf("🔊 SP2 merged TTS file created: %s, Size: %d bytes", fileTTS, fileInfo.Size())
// 							} else {
// 								logrus.Errorf("🔊 SP2 TTS file stat error: %v", statErr)
// 							}
// 						}

// 						// For SP-2
// 						noSurat, err := IncrementNomorSuratSP(dbWeb, "LAST_NOMOR_SURAT_SP2_GENERATED")
// 						if err != nil {
// 							logrus.Errorf("Failed to increment nomor surat SP-2: %v", err)
// 							continue
// 						}
// 						var noSuratStr string
// 						if noSurat < 1000 {
// 							noSuratStr = fmt.Sprintf("%03d", noSurat)
// 						} else {
// 							noSuratStr = fmt.Sprintf("%d", noSurat)
// 						}

// 						monthRoman, err := fun.MonthToRoman(int(now.Month()))
// 						if err != nil {
// 							logrus.Errorf("Failed to convert month to roman numeral: %v", err)
// 							continue
// 						}
// 						splCity := getSPLCity(record.SPL)
// 						if splCity == "" {
// 							logrus.Warnf("SPL city not found for SPL %s, defaulting to 'Unknown'", record.SPL)
// 							splCity = "Unknown"
// 						}
// 						tanggalSP2Terbit, err := tanggal.Papar(now, "Jakarta", tanggal.WIB)
// 						if err != nil {
// 							logrus.Errorf("Failed to get formatted date for SP2: %v", err)
// 							continue
// 						}
// 						tglSP2Diterbitkan := tanggalSP2Terbit.Format(" ", []tanggal.Format{
// 							tanggal.Hari,      // 27
// 							tanggal.NamaBulan, // Maret
// 							tanggal.Tahun,     // 2025
// 						})

// 						// Simple replacements map
// 						placeholdersSP2 := map[string]string{
// 							"$nomor_surat":            noSuratStr,
// 							"$bulan_romawi":           monthRoman,
// 							"$tahun_sp":               now.Format("2006"),
// 							"$nama_spl":               namaSPL,
// 							"$jabatan_spl":            fmt.Sprintf("Service Point Leader %s", splCity),
// 							"$pelanggaran_karyawan":   pelanggaranID,
// 							"$nama_teknisi":           namaTeknisi,
// 							"$tanggal_sp_diterbitkan": tglSP2Diterbitkan,
// 							"$personalia_name":        config.GetConfig().Default.PTHRD[1].Name,
// 							"$sac_name":               SACData.FullName,
// 							"$sac_ttd":                SACData.TTDPath,
// 							"$record_technician":      record.Technician,
// 							"$for_project":            forProject,
// 						}
// 						selectedMainDir, err := fun.FindValidDirectory([]string{
// 							"web/file/sp_technician",
// 							"../web/file/sp_technician",
// 							"../../web/file/sp_technician",
// 						})

// 						if err != nil {
// 							logrus.Errorf("Failed to find valid directory for SP technician PDF files: %v", err)
// 							continue
// 						}
// 						fileDir := filepath.Join(selectedMainDir, time.Now().Format("2006-01-02"))
// 						if err := os.MkdirAll(fileDir, 0755); err != nil {
// 							logrus.Errorf("Failed to create PDF directory %s: %v", fileDir, err)
// 							continue
// 						}

// 						pdfFileName := fmt.Sprintf("SP_2_%s_%s.pdf",
// 							strings.ReplaceAll(record.Technician, "*", "Resigned"),
// 							now.Format("2006-01-02"),
// 						)
// 						pdfSP2FilePath := filepath.Join(fileDir, pdfFileName)

// 						if err := CreatePDFSP2ForTechnician(placeholdersSP2, pdfSP2FilePath); err != nil {
// 							logrus.Errorf("Failed to create PDF for SP 2: %v", err)
// 							continue
// 						}

// 						gotSP2At := time.Now()
// 						dataSPTechnician := sptechnicianmodel.TechnicianGotSP{
// 							IsGotSP2:        true, // Technician got SP 2
// 							GotSP2At:        &gotSP2At,
// 							PelanggaranSP2:  pelanggaranID,
// 							SP2SoundTTSPath: fileTTS,
// 							SP2FilePath:     pdfSP2FilePath,
// 						}

// 						if err := dbWeb.
// 							Where("for_project = ? AND technician = ? AND is_got_sp1 = ?", forProject, record.Technician, true).
// 							Updates(&dataSPTechnician).Error; err != nil {
// 							logrus.Errorf("Failed to create SP data for technician %s: %v", record.Technician, err)
// 							continue
// 						}

// 						if sanitizedNOHPSPL != "" {
// 							jidStrSPL := fmt.Sprintf("62%s@%s", sanitizedNOHPSPL, "s.whatsapp.net")

// 							var sbID strings.Builder
// 							sbID.WriteString(fmt.Sprintf("Dengan ini, kami menyampaikan bahwa saudara *%s* menerima Surat Peringatan (SP) 2.\n", namaTeknisi))
// 							sbID.WriteString(fmt.Sprintf("Sehubungan dengan SP 1 yang sebelumnya disampaikan kepada saudara %s pada %s dan belum ada perbaikan yang memadai, maka perusahaan memutuskan untuk menindaklanjuti melalui SP 2.\n\n", namaTeknisi, tanggalSP1Terkirim))
// 							sbID.WriteString("Mohon menjadi perhatian dan segera melakukan perbaikan yang diperlukan.\n")
// 							sbID.WriteString("Terima kasih.")

// 							var sbEN strings.Builder
// 							sbEN.WriteString(fmt.Sprintf("We hereby inform you that Mr. *%s* has received Warning Letter (SP) 2.\n", namaTeknisi))
// 							sbEN.WriteString(fmt.Sprintf("In connection with the SP 1 that was previously sent to Mr. %s on %s and there has been no adequate improvement, the company has decided to follow up with SP 2.\n\n", namaTeknisi, tanggalSP1Terkirim))
// 							sbEN.WriteString("Please pay attention and immediately make the necessary improvements.\n")
// 							sbEN.WriteString("Thank you.")

// 							var msgIDsb strings.Builder
// 							var msgENsb strings.Builder
// 							msgIDsb.WriteString(fmt.Sprintf("Halo, %s Pak %s.\n\n", greetingID, namaSPL))
// 							msgIDsb.WriteString(sbID.String())
// 							msgENsb.WriteString(fmt.Sprintf("Hello, %s Mr. %s.\n\n", greetingEN, namaSPL))
// 							msgENsb.WriteString(sbEN.String())

// 							idMsg := msgIDsb.String()
// 							enMsg := msgENsb.String()

// 							sendLangDocumentMessageForSPTechnician(
// 								forProject,
// 								record.Technician,
// 								jidStrSPL,
// 								idMsg,
// 								enMsg,
// 								"id",
// 								pdfSP2FilePath,
// 								2,
// 								"62"+sanitizedNOHPSPL,
// 							)

// 							// ADD: SP 2 send to Mr. Oliver if needed
// 							if !config.GetConfig().SPTechnician.ActiveDebug {
// 								for _, hrd := range dataHRD {
// 									var msgIDsb strings.Builder
// 									var msgENsb strings.Builder
// 									msgIDsb.WriteString(fmt.Sprintf("Halo, %s Kak %s.\n\n", greetingID, hrd.Name))
// 									msgIDsb.WriteString(sbID.String())
// 									msgENsb.WriteString(fmt.Sprintf("Hello, %s Sist %s.\n\n", greetingEN, hrd.Name))
// 									msgENsb.WriteString(sbEN.String())

// 									idMsg := msgIDsb.String()
// 									enMsg := msgENsb.String()

// 									jidStrHRD := fmt.Sprintf("%s@%s", hrd.PhoneNumber, "s.whatsapp.net")
// 									sendLangDocumentMessageForSPTechnician(
// 										forProject,
// 										record.Technician,
// 										jidStrHRD,
// 										idMsg, enMsg, "id",
// 										pdfSP2FilePath, 2,
// 										hrd.PhoneNumber)
// 								}
// 								if jidStrSAC != "" {
// 									if strings.Contains(strings.ToLower(SACData.FullName), "tetty") {
// 										var msgIDsb strings.Builder
// 										var msgENsb strings.Builder
// 										msgIDsb.WriteString(fmt.Sprintf("Halo, %s Kak %s.\n\n", greetingID, SACData.FullName))
// 										msgIDsb.WriteString(sbID.String())
// 										msgENsb.WriteString(fmt.Sprintf("Hello, %s Sist %s.\n\n", greetingEN, SACData.FullName))
// 										msgENsb.WriteString(sbEN.String())

// 										idMsg := msgIDsb.String()
// 										enMsg := msgENsb.String()

// 										jidStrSAC := fmt.Sprintf("%s@%s", SACData.PhoneNumber, "s.whatsapp.net")
// 										sendLangDocumentMessageForSPTechnician(forProject, record.Technician, jidStrSAC, idMsg, enMsg, "id", pdfSP2FilePath, 2, SACData.PhoneNumber)
// 									} else {
// 										var msgIDsb strings.Builder
// 										var msgENsb strings.Builder
// 										msgIDsb.WriteString(fmt.Sprintf("Halo, %s Pak %s.\n\n", greetingID, SACData.FullName))
// 										msgIDsb.WriteString(sbID.String())
// 										msgENsb.WriteString(fmt.Sprintf("Hello, %s Mr. %s.\n\n", greetingEN, SACData.FullName))
// 										msgENsb.WriteString(sbEN.String())

// 										idMsg := msgIDsb.String()
// 										enMsg := msgENsb.String()

// 										jidStrSAC := fmt.Sprintf("%s@%s", SACData.PhoneNumber, "s.whatsapp.net")
// 										sendLangDocumentMessageForSPTechnician(forProject, record.Technician, jidStrSAC, idMsg, enMsg, "id", pdfSP2FilePath, 2, SACData.PhoneNumber)
// 									}
// 								}
// 								// sendLangDocumentMessageForSPTechnician(forProject, record.Technician, jidStrOliver, idMsg, enMsg, "id", WordSP2, 2)
// 							}

// 							logrus.Infof("SP 2 of Technician %s has been sent", record.Technician)
// 						} else {
// 							logrus.Warnf("No valid phone number for SPL %s, cannot send SP 2", record.SPL)
// 							continue
// 						}

// 					} // .end of technician is not login today
// 				} // .end of SPL did not respond to SP 1
// 			} // .end of technician got SP 1 but not SP 2

// 			// Technician GOT SP 3
// 			if techGotSP1 &&
// 				techGotSP2 &&
// 				!techGotSP3 {

// 				// 1. Find the first SP2 message to get the sent time
// 				var firstSP2Message sptechnicianmodel.SPWhatsAppMessage
// 				if err := dbWeb.
// 					Where("technician_got_sp_id = ? AND number_of_sp = ?", dataSP.ID, 2).
// 					Order("whatsapp_sent_at asc").
// 					First(&firstSP2Message).Error; err != nil {
// 					logrus.Warnf("Could not find the first SP2 message for technician %s to determine sent time for SP3 escalation: %v", record.Technician, err)
// 					continue
// 				}

// 				if firstSP2Message.WhatsappSentAt == nil {
// 					logrus.Warnf("SP2 Whatsapp sent time is nil for technician %s, cannot create SP3", record.Technician)
// 					continue
// 				}

// 				// 2. Calculate the reply deadline (7 PM on the same day)
// 				sentAt := *firstSP2Message.WhatsappSentAt
// 				deadline := time.Date(sentAt.Year(), sentAt.Month(), sentAt.Day(), maxResponseSPAtHour, 0, 0, 0, sentAt.Location())

// 				// 3. Count replies received before the deadline
// 				var onTimeReplyCount int64
// 				dbWeb.Model(&sptechnicianmodel.SPWhatsAppMessage{}).
// 					Where("technician_got_sp_id = ?", dataSP.ID).
// 					Where("number_of_sp = ?", 2).
// 					Where("for_project = ?", forProject).
// 					Where("whatsapp_reply_text IS NOT NULL AND whatsapp_reply_text != ''").
// 					Where("whatsapp_replied_at <= ?", deadline).
// 					Count(&onTimeReplyCount)

// 				// 4. Proceed only if no on-time replies were found
// 				if onTimeReplyCount == 0 {
// 					// Technician did not reply in time, so we can proceed with SP3.
// 					if !technicianIsLoginToday {
// 						tgl, err := tanggal.Papar(sentAt, "Jakarta", tanggal.WIB)
// 						if err != nil {
// 							logrus.Errorf("Failed to format date for SP 3: %v", err)
// 							continue
// 						}
// 						tanggalSP2Terkirim := tgl.Format(" ", []tanggal.Format{
// 							tanggal.NamaHariDenganKoma, // Kamis,
// 							tanggal.Hari,               // 27
// 							tanggal.NamaBulan,          // Maret
// 							tanggal.Tahun,              // 2025
// 							tanggal.PukulDenganDetik,
// 							tanggal.ZonaWaktu,
// 						})

// 						tglSP3, err := tanggal.Papar(now, "Jakarta", tanggal.WIB)
// 						if err != nil {
// 							logrus.Errorf("Failed to format current date for SP 3: %v", err)
// 							continue
// 						}
// 						tanggalSP3Diterbitkan := tglSP3.Format(" ", []tanggal.Format{
// 							tanggal.NamaHariDenganKoma, // Kamis,
// 							tanggal.Hari,               // 27
// 							tanggal.NamaBulan,          // Maret
// 							tanggal.Tahun,              // 2025
// 						})

// 						SP3TextPart1 := fmt.Sprintf("Merujuk pada Surat Peringatan (SP-2) yang telah disampaikan kepada Saudara %s", namaTeknisi)
// 						SP3TextPart2 := fmt.Sprintf("pada tanggal %s,", tanggalSP2Terkirim)
// 						SP3TextPart3 := "perusahaan menilai bahwa pelanggaran yang Saudara lakukan tidak kunjung diperbaiki."
// 						SP3TextPart4 := "Kesempatan yang telah diberikan sebelumnya tidak dimanfaatkan dengan baik."
// 						SP3TextPart5 := "Oleh karena itu, dengan berat hati,"
// 						SP3TextPart6 := "perusahaan mengambil langkah tegas."
// 						SP3TextPart7 := "Dengan ini diterbitkan Surat Peringatan (SP-3) sebagai peringatan terakhir."
// 						SP3TextPart8 := "Surat ini juga menyatakan berakhirnya hubungan kerja antara perusahaan dan Saudara."
// 						SP3TextPart9 := "Keputusan berlaku sejak tanggal surat ini diterbitkan,"
// 						SP3TextPart10 := fmt.Sprintf("yakni pada %s.", tanggalSP3Diterbitkan)
// 						SP3TextPart11 := "Hal ini sesuai dengan ketentuan perusahaan."
// 						SP3TextPart12 := "Atas perhatiannya, kami ucapkan terima kasih..."

// 						fileNameForSP3 := fmt.Sprintf("%s_SP3", strings.ReplaceAll(record.Technician, "*", "Resigned"))

// 						// Use robust TTS creation for multiple parts
// 						fileTTS, err := fun.CreateRobustTTS(speech, audioForSPDir, []string{
// 							SP3TextPart1,
// 							SP3TextPart2,
// 							SP3TextPart3,
// 							SP3TextPart4,
// 							SP3TextPart5,
// 							SP3TextPart6,
// 							SP3TextPart7,
// 							SP3TextPart8,
// 							SP3TextPart9,
// 							SP3TextPart10,
// 							SP3TextPart11,
// 							SP3TextPart12,
// 						}, fileNameForSP3)
// 						if err != nil {
// 							logrus.Errorf("Failed to create merged SP3 TTS file: %v", err)
// 							continue
// 						}

// 						// Debug: Check the created file
// 						if fileTTS != "" {
// 							fileInfo, statErr := os.Stat(fileTTS)
// 							if statErr == nil {
// 								logrus.Debugf("🔊 SP3 merged TTS file created: %s, Size: %d bytes", fileTTS, fileInfo.Size())
// 							} else {
// 								logrus.Errorf("🔊 SP3 TTS file stat error: %v", statErr)
// 							}
// 						}

// 						// For SP-3
// 						noSurat, err := IncrementNomorSuratSP(dbWeb, "LAST_NOMOR_SURAT_SP3_GENERATED")
// 						if err != nil {
// 							logrus.Errorf("Failed to increment nomor surat SP-3: %v", err)
// 							continue
// 						}
// 						var noSuratStr string
// 						if noSurat < 1000 {
// 							noSuratStr = fmt.Sprintf("%03d", noSurat)
// 						} else {
// 							noSuratStr = fmt.Sprintf("%d", noSurat)
// 						}

// 						monthRoman, err := fun.MonthToRoman(int(now.Month()))
// 						if err != nil {
// 							logrus.Errorf("Failed to convert month to roman numeral: %v", err)
// 							continue
// 						}
// 						splCity := getSPLCity(record.SPL)
// 						if splCity == "" {
// 							logrus.Warnf("SPL city not found for SPL %s, defaulting to 'Unknown'", record.SPL)
// 							splCity = "Unknown"
// 						}
// 						tanggalSP3Terbit, err := tanggal.Papar(now, "Jakarta", tanggal.WIB)
// 						if err != nil {
// 							logrus.Errorf("Failed to get formatted date for SP3: %v", err)
// 							continue
// 						}
// 						tglSP3Diterbitkan := tanggalSP3Terbit.Format(" ", []tanggal.Format{
// 							tanggal.Hari,      // 27
// 							tanggal.NamaBulan, // Maret
// 							tanggal.Tahun,     // 2025
// 						})

// 						// Simple replacements map
// 						placeholdersSP3 := map[string]string{
// 							"$nomor_surat":            noSuratStr,
// 							"$bulan_romawi":           monthRoman,
// 							"$tahun_sp":               now.Format("2006"),
// 							"$nama_spl":               namaSPL,
// 							"$jabatan_spl":            fmt.Sprintf("Service Point Leader %s", splCity),
// 							"$pelanggaran_karyawan":   pelanggaranID,
// 							"$nama_teknisi":           namaTeknisi,
// 							"$tanggal_sp_diterbitkan": tglSP3Diterbitkan,
// 							"$personalia_name":        config.GetConfig().Default.PTHRD[1].Name,
// 							"$sac_name":               SACData.FullName,
// 							"$sac_ttd":                SACData.TTDPath,
// 							"$record_technician":      record.Technician,
// 							"$for_project":            forProject,
// 						}
// 						selectedMainDir, err := fun.FindValidDirectory([]string{
// 							"web/file/sp_technician",
// 							"../web/file/sp_technician",
// 							"../../web/file/sp_technician",
// 						})

// 						if err != nil {
// 							logrus.Errorf("Failed to find valid directory for SP technician PDF files: %v", err)
// 							continue
// 						}
// 						fileDir := filepath.Join(selectedMainDir, time.Now().Format("2006-01-02"))
// 						if err := os.MkdirAll(fileDir, 0755); err != nil {
// 							logrus.Errorf("Failed to create PDF directory %s: %v", fileDir, err)
// 							continue
// 						}

// 						pdfFileName := fmt.Sprintf("SP_3_%s_%s.pdf",
// 							strings.ReplaceAll(record.Technician, "*", "Resigned"),
// 							now.Format("2006-01-02"),
// 						)
// 						pdfSP3FilePath := filepath.Join(fileDir, pdfFileName)

// 						if err := CreatePDFSP3ForTechnician(placeholdersSP3, pdfSP3FilePath); err != nil {
// 							logrus.Errorf("Failed to create PDF for SP 3: %v", err)
// 							continue
// 						}

// 						gotSP3At := time.Now()
// 						dataSPTechnician := sptechnicianmodel.TechnicianGotSP{
// 							IsGotSP3:        true, // Technician got SP 3
// 							GotSP3At:        &gotSP3At,
// 							PelanggaranSP3:  pelanggaranID,
// 							SP3SoundTTSPath: fileTTS,
// 							SP3FilePath:     pdfSP3FilePath,
// 						}

// 						if err := dbWeb.
// 							Where("for_project = ? AND technician = ? AND is_got_sp2 = ?", forProject, record.Technician, true).
// 							Updates(&dataSPTechnician).Error; err != nil {
// 							logrus.Errorf("Failed to create SP data for technician %s: %v", record.Technician, err)
// 							continue
// 						}

// 						if sanitizedNOHPSPL != "" {
// 							jidStrSPL := fmt.Sprintf("62%s@%s", sanitizedNOHPSPL, "s.whatsapp.net")

// 							var sbID strings.Builder
// 							sbID.WriteString(fmt.Sprintf("Dengan ini, kami menyampaikan bahwa saudara *%s* menerima Surat Peringatan (SP) 3.\n", namaTeknisi))
// 							sbID.WriteString(fmt.Sprintf("Merujuk pada SP 2 yang sebelumnya disampaikan kepada saudara %s pada %s dan tidak terdapat perbaikan yang memadai,", namaTeknisi, tanggalSP2Terkirim))
// 							sbID.WriteString(" perusahaan memutuskan untuk menerbitkan SP 3 sebagai peringatan terakhir.\n")
// 							sbID.WriteString("Surat ini juga menyatakan berakhirnya hubungan kerja antara perusahaan dan saudara.\n")
// 							sbID.WriteString("Keputusan berlaku sejak tanggal ditetapkannya surat ini, sesuai dengan ketentuan perusahaan.\n\n")
// 							sbID.WriteString("Demikian untuk menjadi perhatian serius.\n")
// 							sbID.WriteString("Terima kasih.")

// 							var sbEN strings.Builder
// 							sbEN.WriteString(fmt.Sprintf("We hereby inform you that Mr. *%s* has received Warning Letter (SP) 3.\n", namaTeknisi))
// 							sbEN.WriteString(fmt.Sprintf("Referring to SP 2 that was previously sent to Mr. %s on %s and with no adequate corrective action taken,", namaTeknisi, tanggalSP2Terkirim))
// 							sbEN.WriteString(" the company has decided to issue SP 3 as the final warning.\n")
// 							sbEN.WriteString(fmt.Sprintf("This letter also formally states the termination of the employment relationship between the company and Mr. %s.\n", namaTeknisi))
// 							sbEN.WriteString("This decision takes effect from the date of this letter, in accordance with company regulations.\n\n")
// 							sbEN.WriteString("Please treat this matter with the utmost seriousness.\n")
// 							sbEN.WriteString("Thank you.")

// 							var msgIDsb strings.Builder
// 							var msgENsb strings.Builder
// 							msgIDsb.WriteString(fmt.Sprintf("Halo, %s Pak %s.\n\n", greetingID, namaSPL))
// 							msgIDsb.WriteString(sbID.String())
// 							msgENsb.WriteString(fmt.Sprintf("Hello, %s Mr. %s.\n\n", greetingEN, namaSPL))
// 							msgENsb.WriteString(sbEN.String())

// 							idMsg := msgIDsb.String()
// 							enMsg := msgENsb.String()

// 							sendLangDocumentMessageForSPTechnician(
// 								forProject,
// 								record.Technician,
// 								jidStrSPL,
// 								idMsg,
// 								enMsg,
// 								"id",
// 								pdfSP3FilePath,
// 								3,
// 								"62"+sanitizedNOHPSPL,
// 							)

// 							// ADD: SP 3 send to Mr. Oliver if needed
// 							if !config.GetConfig().SPTechnician.ActiveDebug {
// 								for _, hrd := range dataHRD {
// 									var msgIDsb strings.Builder
// 									var msgENsb strings.Builder
// 									msgIDsb.WriteString(fmt.Sprintf("Halo, %s Kak %s.\n\n", greetingID, hrd.Name))
// 									msgIDsb.WriteString(sbID.String())
// 									msgENsb.WriteString(fmt.Sprintf("Hello, %s Sist %s.\n\n", greetingEN, hrd.Name))
// 									msgENsb.WriteString(sbEN.String())

// 									idMsg := msgIDsb.String()
// 									enMsg := msgENsb.String()

// 									jidStrHRD := fmt.Sprintf("%s@%s", hrd.PhoneNumber, "s.whatsapp.net")
// 									sendLangDocumentMessageForSPTechnician(
// 										forProject,
// 										record.Technician,
// 										jidStrHRD,
// 										idMsg, enMsg, "id",
// 										pdfSP3FilePath, 3,
// 										hrd.PhoneNumber)
// 								}
// 								if jidStrSAC != "" {
// 									if strings.Contains(strings.ToLower(SACData.FullName), "tetty") {
// 										var msgIDsb strings.Builder
// 										var msgENsb strings.Builder
// 										msgIDsb.WriteString(fmt.Sprintf("Halo, %s Kak %s.\n\n", greetingID, SACData.FullName))
// 										msgIDsb.WriteString(sbID.String())
// 										msgENsb.WriteString(fmt.Sprintf("Hello, %s Sist %s.\n\n", greetingEN, SACData.FullName))
// 										msgENsb.WriteString(sbEN.String())

// 										idMsg := msgIDsb.String()
// 										enMsg := msgENsb.String()

// 										jidStrSAC := fmt.Sprintf("%s@%s", SACData.PhoneNumber, "s.whatsapp.net")
// 										sendLangDocumentMessageForSPTechnician(forProject, record.Technician, jidStrSAC, idMsg, enMsg, "id", pdfSP3FilePath, 3, SACData.PhoneNumber)
// 									} else {
// 										var msgIDsb strings.Builder
// 										var msgENsb strings.Builder
// 										msgIDsb.WriteString(fmt.Sprintf("Halo, %s Pak %s.\n\n", greetingID, SACData.FullName))
// 										msgIDsb.WriteString(sbID.String())
// 										msgENsb.WriteString(fmt.Sprintf("Hello, %s Mr. %s.\n\n", greetingEN, SACData.FullName))
// 										msgENsb.WriteString(sbEN.String())

// 										idMsg := msgIDsb.String()
// 										enMsg := msgENsb.String()

// 										jidStrSAC := fmt.Sprintf("%s@%s", SACData.PhoneNumber, "s.whatsapp.net")
// 										sendLangDocumentMessageForSPTechnician(forProject, record.Technician, jidStrSAC, idMsg, enMsg, "id", pdfSP3FilePath, 3, SACData.PhoneNumber)
// 									}
// 								}
// 							}
// 							logrus.Infof("SP 3 of Technician %s has been sent", record.Technician)
// 						} else {
// 							logrus.Warnf("No valid phone number for SPL %s, cannot send SP 3", record.SPL)
// 							continue
// 						}

// 					} // .end of technician is not login today
// 				} // .end of SPL did not respond to SP 2
// 			} // .end of technician got SP 2 but not SP 3

// 			// #######################################################################################################################################################
// 			// SPL/SAC Got SP
// 			var dataSPLatest sptechnicianmodel.TechnicianGotSP
// 			result := dbWeb.Where("for_project = ? AND technician = ?", forProject, record.Technician).First(&dataSPLatest)
// 			if result.Error == nil {
// 				techGotSP1 = dataSPLatest.IsGotSP1
// 				techGotSP2 = dataSPLatest.IsGotSP2
// 				techGotSP3 = dataSPLatest.IsGotSP3
// 			}

// 			audioSPMainDir, err := fun.FindValidDirectory([]string{
// 				"web/file/sounding_sp_spl",
// 				"../web/file/sounding_sp_spl",
// 				"../../web/file/sounding_sp_spl",
// 			})
// 			if err != nil {
// 				logrus.Errorf("Failed to find valid directory for SP technician SPL audio files: %v", err)
// 				continue
// 			}
// 			audioForSPDir := filepath.Join(audioSPMainDir, now.Format("2006-01-02"))
// 			if err := os.MkdirAll(audioForSPDir, 0755); err != nil {
// 				logrus.Errorf("Failed to create audio directory %s: %v", audioForSPDir, err)
// 				continue
// 			}

// 			if techGotSP1 && techGotSP2 && techGotSP3 {
// 				spl := record.SPL

// 				if strings.Contains(spl, "*") {
// 					continue // SKIP resigned SPL
// 				}

// 				var spForSPLisProcessing bool = false

// 				var dataSPSPL sptechnicianmodel.SPLGotSP
// 				result := dbWeb.Where("for_project = ? AND spl = ?", forProject, spl).First(&dataSPSPL)
// 				if result.Error != nil {
// 					if errors.Is(result.Error, gorm.ErrRecordNotFound) {
// 						// SPL never got SP before, create new record as SP-1
// 						SP1TextPart1 := fmt.Sprintf("Sehubungan dengan Surat Peringatan (SP-3) yang telah disampaikan kepada teknisi: %s", namaTeknisi)
// 						SP1TextPart2 := fmt.Sprintf("dibawah naungan saudara %s", namaSPL)
// 						SP1TextPart3 := "maka perusahaan menilai perlu untuk menindaklanjuti,"
// 						SP1TextPart4 := "dengan menerbitkan Surat Peringatan (SP-1) kepada saudara selaku Service Point Leader (SPL)."
// 						SP1TextPart5 := "Hal ini didasari oleh tanggung jawab saudara sebagai SPL"
// 						SP1TextPart6 := "dalam mengawasi dan membina teknisi di bawah naungan saudara."
// 						SP1TextPart7 := "Mohon menjadi perhatian dan segera melakukan perbaikan yang diperlukan. terima kasih..."

// 						fileNameForSPLSP1 := fmt.Sprintf("%s_SP1_SPL", strings.ReplaceAll(spl, "*", "Resigned"))
// 						// Use robust TTS creation for multiple parts
// 						fileTTS, err := fun.CreateRobustTTS(speech, audioForSPDir, []string{
// 							SP1TextPart1,
// 							SP1TextPart2,
// 							SP1TextPart3,
// 							SP1TextPart4,
// 							SP1TextPart5,
// 							SP1TextPart6,
// 							SP1TextPart7,
// 						}, fileNameForSPLSP1)
// 						if err != nil {
// 							logrus.Errorf("Failed to create merged SPL SP1 TTS file: %v", err)
// 							continue
// 						}

// 						// Debug: Check the created file
// 						if fileTTS != "" {
// 							fileInfo, statErr := os.Stat(fileTTS)
// 							if statErr == nil {
// 								logrus.Debugf("🔊 SPL SP1 merged TTS file created: %s, Size: %d bytes", fileTTS, fileInfo.Size())
// 							} else {
// 								logrus.Errorf("🔊 SPL SP1 TTS file stat error: %v", statErr)
// 							}
// 						}

// 						noSurat, err := IncrementNomorSuratSP(dbWeb, "LAST_NOMOR_SURAT_SP1_GENERATED")
// 						if err != nil {
// 							logrus.Errorf("Failed to increment nomor surat SP-1 for SPL: %v", err)
// 							continue
// 						}
// 						var noSuratStr string
// 						if noSurat < 1000 {
// 							noSuratStr = fmt.Sprintf("%03d", noSurat)
// 						} else {
// 							noSuratStr = fmt.Sprintf("%d", noSurat)
// 						}

// 						monthRoman, err := fun.MonthToRoman(int(now.Month()))
// 						if err != nil {
// 							logrus.Errorf("Failed to convert month to roman numeral: %v", err)
// 							continue
// 						}
// 						splCity := getSPLCity(spl)
// 						if splCity == "" {
// 							logrus.Warnf("SPL city not found for SPL %s, defaulting to 'Unknown'", spl)
// 							splCity = "Unknown"
// 						}
// 						tanggalSP1Terbit, err := tanggal.Papar(now, "Jakarta", tanggal.WIB)
// 						if err != nil {
// 							logrus.Errorf("Failed to get formatted date for SPL SP1: %v", err)
// 							continue
// 						}
// 						tglSP1Diterbitkan := tanggalSP1Terbit.Format(" ", []tanggal.Format{
// 							tanggal.Hari,      // 27
// 							tanggal.NamaBulan, // Maret
// 							tanggal.Tahun,     // 2025
// 						})

// 						var tglSp3Ts time.Time
// 						var sp3WAMsgOfTechnician sptechnicianmodel.SPWhatsAppMessage
// 						if err := dbWeb.
// 							Where("technician_got_sp_id = ? AND number_of_sp = ?", dataSPLatest.ID, 3).
// 							Where("for_project = ?", forProject).
// 							Order("whatsapp_sent_at asc").
// 							First(&sp3WAMsgOfTechnician).Error; err != nil {
// 							logrus.Warnf("Could not find the SP3 message for technician %s to determine sent time for SPL SP1 escalation: %v", record.Technician, err)
// 							tglSp3Ts = now
// 						}

// 						if sp3WAMsgOfTechnician.WhatsappSentAt != nil {
// 							tglSp3Ts = *sp3WAMsgOfTechnician.WhatsappSentAt
// 						} else {
// 							tglSp3Ts = now
// 						}

// 						tglSP3Teknisi, err := tanggal.Papar(tglSp3Ts, "Jakarta", tanggal.WIB)
// 						if err != nil {
// 							logrus.Errorf("Failed to format SP3 sent date for technician %s: %v", record.Technician, err)
// 							continue
// 						}
// 						tanggalSP3Terkirim := tglSP3Teknisi.Format(" ", []tanggal.Format{
// 							tanggal.NamaHariDenganKoma, // Kamis,
// 							tanggal.Hari,               // 27
// 							tanggal.NamaBulan,          // Maret
// 							tanggal.Tahun,              // 2025
// 							tanggal.Pukul,
// 							tanggal.ZonaWaktu,
// 						})

// 						pelanggaranSP1SPLID := fmt.Sprintf("Kurangnya pengawasan dan pembinaan terhadap teknisi %s, sehingga teknisi tersebut melakukan pelanggaran yang berujung pada penerbitan Surat Peringatan (SP-3) pada %v.", namaTeknisi, tanggalSP3Terkirim)
// 						// pelanggaranSP1SPLEN := fmt.Sprintf("Insufficient supervision and guidance of technician %s, which resulted in the technician committing violations that led to the issuance of a Warning Letter (SP-3) on %v.", namaTeknisi, tanggalSP3Terkirim)

// 						// Simple replacements map
// 						placeholdersSPLSP1 := map[string]string{
// 							"$nomor_surat":            noSuratStr,
// 							"$bulan_romawi":           monthRoman,
// 							"$tahun_sp":               now.Format("2006"),
// 							"$nama_spl":               namaSPL,
// 							"$jabatan_spl":            fmt.Sprintf("Service Point Leader %s", splCity),
// 							"$pelanggaran_karyawan":   pelanggaranSP1SPLID,
// 							"$nama_teknisi":           namaTeknisi,
// 							"$tanggal_sp_diterbitkan": tglSP1Diterbitkan,
// 							"$personalia_name":        config.GetConfig().Default.PTHRD[1].Name,
// 							"$sac_name":               SACData.FullName,
// 							"$sac_ttd":                SACData.TTDPath,
// 							"$record_spl":             spl,
// 							"$for_project":            forProject,
// 						}
// 						selectedMainDir, err := fun.FindValidDirectory([]string{
// 							"web/file/sp_spl",
// 							"../web/file/sp_spl",
// 							"../../web/file/sp_spl",
// 						})
// 						if err != nil {
// 							logrus.Errorf("Failed to find valid directory for SP technician PDF files: %v", err)
// 							continue
// 						}
// 						fileDir := filepath.Join(selectedMainDir, time.Now().Format("2006-01-02"))
// 						if err := os.MkdirAll(fileDir, 0755); err != nil {
// 							logrus.Errorf("Failed to create PDF directory %s: %v", fileDir, err)
// 							continue
// 						}

// 						pdfFileName := fmt.Sprintf("SP_1_%s_%s.pdf",
// 							strings.ReplaceAll(spl, "*", "Resigned"),
// 							now.Format("2006-01-02"),
// 						)
// 						pdfSPLSP1FilePath := filepath.Join(fileDir, pdfFileName)

// 						if err := CreatePDFSP1ForSPL(placeholdersSPLSP1, pdfSPLSP1FilePath); err != nil {
// 							logrus.Errorf("Failed to create PDF for SPL SP 1: %v", err)
// 							continue
// 						}

// 						spForSPLisProcessing = true // Mark as processing SP for SPL for the 1st time

// 						dataSPSPL = sptechnicianmodel.SPLGotSP{
// 							ForProject:                 forProject,
// 							SPL:                        spl,
// 							IsGotSP1:                   true, // SPL got SP 1
// 							PelanggaranSP1:             pelanggaranSP1SPLID,
// 							TechnicianNameCausedGotSP1: record.Technician,
// 							SP1SoundTTSPath:            fileTTS,
// 							SP1FilePath:                pdfSPLSP1FilePath,
// 						}

// 						if err := dbWeb.Create(&dataSPSPL).Error; err != nil {
// 							logrus.Errorf("Failed to create SP data for SPL %s: %v", spl, err)
// 							continue
// 						}

// 						if sanitizedNOHPSPL != "" {
// 							jidStrSPL := fmt.Sprintf("62%s@%s", sanitizedNOHPSPL, "s.whatsapp.net")

// 							var sbID strings.Builder
// 							sbID.WriteString("Dengan ini, kami menyampaikan bahwa saudara menerima Surat Peringatan (SP) 1.\n")
// 							sbID.WriteString(fmt.Sprintf("Sehubungan dengan SP 3 yang telah disampaikan kepada teknisi: %s dibawah naungan saudara %s,\n", namaTeknisi, namaSPL))
// 							sbID.WriteString("maka perusahaan menilai perlu untuk menindaklanjuti dengan menerbitkan SP 1 kepada saudara selaku Service Point Leader (SPL).\n")
// 							sbID.WriteString("Hal ini didasari oleh tanggung jawab saudara sebagai SPL dalam mengawasi dan membina teknisi di bawah naungan saudara.\n\n")
// 							sbID.WriteString("Mohon menjadi perhatian dan segera melakukan perbaikan yang diperlukan.\n")
// 							sbID.WriteString("Terima kasih.")

// 							var sbEN strings.Builder
// 							sbEN.WriteString("We hereby inform you that you have received Warning Letter (SP) 1.\n")
// 							sbEN.WriteString(fmt.Sprintf("In connection with the SP 3 that has been conveyed to the technician: %s under your supervision,\n", namaTeknisi))
// 							sbEN.WriteString("the company deems it necessary to follow up by issuing SP 1 to you as the Service Point Leader (SPL).\n")
// 							sbEN.WriteString("This is based on your responsibility as an SPL in supervising and fostering the technicians under your supervision.\n\n")
// 							sbEN.WriteString("Please pay attention and immediately make the necessary improvements.\n")
// 							sbEN.WriteString("Thank you.")

// 							var msgIDsb strings.Builder
// 							var msgENsb strings.Builder
// 							msgIDsb.WriteString(fmt.Sprintf("Halo, %s Pak %s.\n\n", greetingID, namaSPL))
// 							msgIDsb.WriteString(sbID.String())
// 							msgENsb.WriteString(fmt.Sprintf("Hello, %s Mr. %s.\n\n", greetingEN, namaSPL))
// 							msgENsb.WriteString(sbEN.String())

// 							idMsg := msgIDsb.String()
// 							enMsg := msgENsb.String()

// 							sendLangDocumentMessageForSPSPL(
// 								forProject,
// 								spl,
// 								jidStrSPL,
// 								idMsg,
// 								enMsg,
// 								"id",
// 								pdfSPLSP1FilePath,
// 								1,
// 								"62"+sanitizedNOHPSPL,
// 							)

// 							// ADD: SPL SP 1 send to Mr. Oliver if needed
// 							if !config.GetConfig().SPTechnician.ActiveDebug {
// 								for _, hrd := range dataHRD {
// 									var msgIDsb strings.Builder
// 									var msgENsb strings.Builder
// 									msgIDsb.WriteString(fmt.Sprintf("Halo, %s Kak %s.\n\n", greetingID, hrd.Name))
// 									msgIDsb.WriteString(sbID.String())
// 									msgENsb.WriteString(fmt.Sprintf("Hello, %s Sist %s.\n\n", greetingEN, hrd.Name))
// 									msgENsb.WriteString(sbEN.String())

// 									idMsg := msgIDsb.String()
// 									enMsg := msgENsb.String()

// 									jidStrHRD := fmt.Sprintf("%s@%s", hrd.PhoneNumber, "s.whatsapp.net")
// 									sendLangDocumentMessageForSPSPL(
// 										forProject,
// 										spl,
// 										jidStrHRD,
// 										idMsg,
// 										enMsg,
// 										"id",
// 										pdfSPLSP1FilePath,
// 										1,
// 										hrd.PhoneNumber,
// 									)
// 								}
// 								if jidStrSAC != "" {
// 									if strings.Contains(strings.ToLower(SACData.FullName), "tetty") {
// 										var msgIDsb strings.Builder
// 										var msgENsb strings.Builder
// 										msgIDsb.WriteString(fmt.Sprintf("Halo, %s Kak %s.\n\n", greetingID, SACData.FullName))
// 										msgIDsb.WriteString(sbID.String())
// 										msgENsb.WriteString(fmt.Sprintf("Hello, %s Sist %s.\n\n", greetingEN, SACData.FullName))
// 										msgENsb.WriteString(sbEN.String())

// 										idMsg := msgIDsb.String()
// 										enMsg := msgENsb.String()

// 										jidStrSAC := fmt.Sprintf("%s@%s", SACData.PhoneNumber, "s.whatsapp.net")
// 										sendLangDocumentMessageForSPSPL(
// 											forProject,
// 											spl,
// 											jidStrSAC,
// 											idMsg,
// 											enMsg,
// 											"id",
// 											pdfSPLSP1FilePath,
// 											1,
// 											SACData.PhoneNumber,
// 										)
// 									} else {
// 										var msgIDsb strings.Builder
// 										var msgENsb strings.Builder
// 										msgIDsb.WriteString(fmt.Sprintf("Halo, %s Pak %s.\n\n", greetingID, SACData.FullName))
// 										msgIDsb.WriteString(sbID.String())
// 										msgENsb.WriteString(fmt.Sprintf("Hello, %s Mr. %s.\n\n", greetingEN, SACData.FullName))
// 										msgENsb.WriteString(sbEN.String())

// 										idMsg := msgIDsb.String()
// 										enMsg := msgENsb.String()

// 										jidStrSAC := fmt.Sprintf("%s@%s", SACData.PhoneNumber, "s.whatsapp.net")
// 										sendLangDocumentMessageForSPSPL(
// 											forProject,
// 											spl,
// 											jidStrSAC,
// 											idMsg,
// 											enMsg,
// 											"id",
// 											pdfSPLSP1FilePath,
// 											1,
// 											SACData.PhoneNumber,
// 										)
// 									}
// 								}
// 							}

// 							if _, exists := SPLGotSPToday[spl]; !exists {
// 								SPLGotSPToday[spl] = SPInfoForSPL{
// 									GotSPToday: true,
// 									SPNumber:   1,
// 								}
// 							}

// 						} else {
// 							logrus.Warnf("No valid phone number for SPL %s, cannot send SPL SP 1", spl)
// 							continue
// 						}
// 					} else {
// 						// Some other error occurred while fetching SPL data
// 						logrus.Errorf("Failed to fetch SP data for SPL %s: %v", spl, result.Error)
// 						continue
// 					}
// 				}

// 				if !spForSPLisProcessing {
// 					splGotSP1 := dataSPSPL.IsGotSP1
// 					splGotSP2 := dataSPSPL.IsGotSP2
// 					splGotSP3 := dataSPSPL.IsGotSP3

// 					// SPL GOT SP 2
// 					if splGotSP1 && !splGotSP2 && !splGotSP3 {
// 						// 1. Find the first SP1 msg of SPL
// 						var firstSP1MsgSPL sptechnicianmodel.SPWhatsAppMessage
// 						if err := dbWeb.
// 							Where("spl_got_sp_id = ? AND number_of_sp = ?", dataSPSPL.ID, 1).
// 							Where("for_project = ?", forProject).
// 							Order("whatsapp_sent_at asc").
// 							First(&firstSP1MsgSPL).Error; err != nil {
// 							logrus.Warnf("Could not find the first SP1 message for SPL %s to determine sent time for SP2 issuance: %v", spl, err)
// 							continue
// 						}

// 						if firstSP1MsgSPL.WhatsappSentAt == nil {
// 							logrus.Warnf("First SP1 message sent time is nil for SPL %s, cannot create SP 2", spl)
// 							continue
// 						}

// 						// 2. Calculate the reply deadline
// 						sentAt := *firstSP1MsgSPL.WhatsappSentAt
// 						deadline := time.Date(sentAt.Year(), sentAt.Month(), sentAt.Day(), maxResponseSPAtHour, 0, 0, 0, sentAt.Location())

// 						// 3. Count replies received before the deadline
// 						var onTimeReplyCount int64
// 						dbWeb.Model(&sptechnicianmodel.SPWhatsAppMessage{}).
// 							Where("spl_got_sp_id = ?", dataSPSPL.ID).
// 							Where("number_of_sp = ?", 1).
// 							Where("for_project = ?", forProject).
// 							Where("whatsapp_reply_text IS NOT NULL AND whatsapp_reply_text != ''").
// 							Where("whatsapp_replied_at <= ?", deadline).
// 							Count(&onTimeReplyCount)

// 						// 4. Proceed only if no on-time replies were found
// 						if onTimeReplyCount == 0 {
// 							if firstSP1MsgSPL.WhatsappSentAt == nil {
// 								logrus.Warnf("First SP1 message sent time is nil for SPL %s, cannot create SP 2", spl)
// 								continue
// 							}

// 							// Check if technician name caused SPL got SP1 is the same as current technician
// 							if dataSPSPL.TechnicianNameCausedGotSP1 == record.Technician {
// 								// SKIP because already got SP 1 for the same technician
// 								logrus.Infof("SPL %s already got SP 1 for technician %s, skipping SP 2 issuance", spl, record.Technician)
// 								continue
// 							}

// 							if _, exists := SPLGotSPToday[spl]; exists && SPLGotSPToday[spl].GotSPToday && SPLGotSPToday[spl].SPNumber == 1 {
// 								// SKIP SPL already got SP today, skip SP 2 issuance
// 								logrus.Infof("SPL %s already got SP today, skipping SP 2 issuance", spl)
// 								continue
// 							}

// 							if dataSPSPL.SP2FilePath != "" {
// 								// SKIP because already got SP 2
// 								logrus.Infof("SPL %s already got SP 2, skipping SP 2 issuance", spl)
// 								continue
// 							}

// 							tgl, err := tanggal.Papar(*firstSP1MsgSPL.WhatsappSentAt, "Jakarta", tanggal.WIB)
// 							if err != nil {
// 								logrus.Errorf("Failed to format date for SPL SP 2: %v", err)
// 								continue
// 							}
// 							tanggalSP1Terkirim := tgl.Format(" ", []tanggal.Format{
// 								tanggal.NamaHariDenganKoma, // Kamis,
// 								tanggal.Hari,               // 27
// 								tanggal.NamaBulan,          // Maret
// 								tanggal.Tahun,              // 2025
// 								tanggal.Pukul,
// 								tanggal.ZonaWaktu,
// 							})

// 							SP2TextPart1 := fmt.Sprintf("Merujuk pada SP-1 yang telah diberikan kepada Saudara %s", namaSPL)
// 							SP2TextPart2 := fmt.Sprintf("pada tanggal %s.", tanggalSP1Terkirim)
// 							SP2TextPart3 := "Sampai saat ini, tidak ada tanggapan dari Saudara."
// 							SP2TextPart4 := "Hal tersebut menunjukkan kelalaian atas peringatan yang diberikan."
// 							SP2TextPart5 := fmt.Sprintf("Selain itu, teknisi %s di bawah pengawasan Saudara", namaTeknisi)
// 							SP2TextPart6 := "kembali melakukan pelanggaran."
// 							SP2TextPart7 := "Pelanggaran tersebut berujung pada penerbitan SP-3."
// 							SP2TextPart8 := "Dengan demikian, perusahaan menerbitkan SP-2 untuk Saudara."
// 							SP2TextPart9 := "Mohon menjadi perhatian serius. terima kasih..."

// 							fileNameForSPLSP2 := fmt.Sprintf("%s_SP2_SPL", strings.ReplaceAll(spl, "*", "Resigned"))

// 							fileTTS, err := fun.CreateRobustTTS(speech, audioForSPDir, []string{
// 								SP2TextPart1,
// 								SP2TextPart2,
// 								SP2TextPart3,
// 								SP2TextPart4,
// 								SP2TextPart5,
// 								SP2TextPart6,
// 								SP2TextPart7,
// 								SP2TextPart8,
// 								SP2TextPart9,
// 							}, fileNameForSPLSP2)
// 							if err != nil {
// 								logrus.Errorf("Failed to create merged SPL SP2 TTS file: %v", err)
// 								continue
// 							}

// 							// Debug: Check the created file
// 							if fileTTS != "" {
// 								fileInfo, statErr := os.Stat(fileTTS)
// 								if statErr == nil {
// 									logrus.Debugf("🔊 SPL SP2 merged TTS file created: %s, Size: %d bytes", fileTTS, fileInfo.Size())
// 								} else {
// 									logrus.Errorf("🔊 SPL SP2 TTS file stat error: %v", statErr)
// 								}
// 							}

// 							noSurat, err := IncrementNomorSuratSP(dbWeb, "LAST_NOMOR_SURAT_SP2_GENERATED")
// 							if err != nil {
// 								logrus.Errorf("Failed to increment nomor surat SP-2 for SPL: %v", err)
// 								continue
// 							}
// 							var noSuratStr string
// 							if noSurat < 1000 {
// 								noSuratStr = fmt.Sprintf("%03d", noSurat)
// 							} else {
// 								noSuratStr = fmt.Sprintf("%d", noSurat)
// 							}

// 							monthRoman, err := fun.MonthToRoman(int(now.Month()))
// 							if err != nil {
// 								logrus.Errorf("Failed to convert month to roman numeral: %v", err)
// 								continue
// 							}
// 							splCity := getSPLCity(spl)
// 							if splCity == "" {
// 								logrus.Warnf("SPL city not found for SPL %s, defaulting to 'Unknown'", spl)
// 								splCity = "Unknown"
// 							}
// 							tanggalSP2Terbit, err := tanggal.Papar(now, "Jakarta", tanggal.WIB)
// 							if err != nil {
// 								logrus.Errorf("Failed to get formatted date for SPL SP2: %v", err)
// 								continue
// 							}
// 							tglSP2Diterbitkan := tanggalSP2Terbit.Format(" ", []tanggal.Format{
// 								tanggal.Hari,      // 27
// 								tanggal.NamaBulan, // Maret
// 								tanggal.Tahun,     // 2025
// 							})

// 							var tglSp3Ts time.Time
// 							var sp3WAMsgOfTechnician sptechnicianmodel.SPWhatsAppMessage
// 							if err := dbWeb.
// 								Where("technician_got_sp_id = ? AND number_of_sp = ?", dataSPLatest.ID, 3).
// 								Where("for_project = ?", forProject).
// 								Order("whatsapp_sent_at asc").
// 								First(&sp3WAMsgOfTechnician).Error; err != nil {
// 								logrus.Warnf("Could not find the SP3 message for technician %s to determine sent time for SPL SP2 escalation: %v", record.Technician, err)
// 								tglSp3Ts = now
// 							}

// 							if sp3WAMsgOfTechnician.WhatsappSentAt != nil {
// 								tglSp3Ts = *sp3WAMsgOfTechnician.WhatsappSentAt
// 							}

// 							tglSP3Teknisi, err := tanggal.Papar(tglSp3Ts, "Jakarta", tanggal.WIB)
// 							if err != nil {
// 								logrus.Errorf("Failed to format SP3 sent date for technician %s: %v", record.Technician, err)
// 								continue
// 							}
// 							tanggalSP3Terkirim := tglSP3Teknisi.Format(" ", []tanggal.Format{
// 								tanggal.NamaHariDenganKoma, // Kamis,
// 								tanggal.Hari,               // 27
// 								tanggal.NamaBulan,          // Maret
// 								tanggal.Tahun,              // 2025
// 								tanggal.Pukul,
// 								tanggal.ZonaWaktu,
// 							})

// 							pelanggaranSP2SPLID := fmt.Sprintf("Kurangnya pengawasan dan pembinaan terhadap teknisi %s, sehingga teknisi tersebut melakukan pelanggaran yang berujung pada penerbitan Surat Peringatan (SP-3) pada %v.", namaTeknisi, tanggalSP3Terkirim)

// 							// Simple replacements map
// 							placeholdersSPLSP2 := map[string]string{
// 								"$nomor_surat":            noSuratStr,
// 								"$bulan_romawi":           monthRoman,
// 								"$tahun_sp":               now.Format("2006"),
// 								"$nama_spl":               namaSPL,
// 								"$jabatan_spl":            fmt.Sprintf("Service Point Leader %s", splCity),
// 								"$pelanggaran_karyawan":   pelanggaranSP2SPLID,
// 								"$nama_teknisi":           namaTeknisi,
// 								"$tanggal_sp_diterbitkan": tglSP2Diterbitkan,
// 								"$personalia_name":        config.GetConfig().Default.PTHRD[1].Name,
// 								"$sac_name":               SACData.FullName,
// 								"$sac_ttd":                SACData.TTDPath,
// 								"$record_spl":             spl,
// 								"$for_project":            forProject,
// 							}
// 							selectedMainDir, err := fun.FindValidDirectory([]string{
// 								"web/file/sp_spl",
// 								"../web/file/sp_spl",
// 								"../../web/file/sp_spl",
// 							})
// 							if err != nil {
// 								logrus.Errorf("Failed to find valid directory for SP technician PDF files: %v", err)
// 								continue
// 							}
// 							fileDir := filepath.Join(selectedMainDir, time.Now().Format("2006-01-02"))
// 							if err := os.MkdirAll(fileDir, 0755); err != nil {
// 								logrus.Errorf("Failed to create PDF directory %s: %v", fileDir, err)
// 								continue
// 							}

// 							pdfFileName := fmt.Sprintf("SP_2_%s_%s.pdf",
// 								strings.ReplaceAll(spl, "*", "Resigned"),
// 								now.Format("2006-01-02"),
// 							)
// 							pdfSPLSP2FilePath := filepath.Join(fileDir, pdfFileName)
// 							if err := CreatePDFSP2ForSPL(placeholdersSPLSP2, pdfSPLSP2FilePath); err != nil {
// 								logrus.Errorf("Failed to create PDF for SPL SP 2: %v", err)
// 								continue
// 							}

// 							dataSPSPLUpdate := sptechnicianmodel.SPLGotSP{
// 								IsGotSP2:                   true, // SPL got SP 2
// 								PelanggaranSP2:             pelanggaranSP2SPLID,
// 								TechnicianNameCausedGotSP2: record.Technician,
// 								SP2SoundTTSPath:            fileTTS,
// 								SP2FilePath:                pdfSPLSP2FilePath,
// 							}

// 							if err := dbWeb.
// 								Model(&dataSPSPL).
// 								Where("for_project = ? AND spl = ? AND is_got_sp1 = ?", forProject, spl, true).
// 								Updates(&dataSPSPLUpdate).Error; err != nil {
// 								logrus.Errorf("Failed to update SP 2 data for SPL %s: %v", spl, err)
// 								continue
// 							}

// 							if sanitizedNOHPSPL != "" {
// 								jidStrSPL := fmt.Sprintf("62%s@%s", sanitizedNOHPSPL, "s.whatsapp.net")

// 								var sbID strings.Builder
// 								sbID.WriteString(fmt.Sprintf("Merujuk pada SP-1 yang telah diberikan kepada Saudara %s pada tanggal %s,\n", namaSPL, tanggalSP1Terkirim))
// 								sbID.WriteString("sampai saat ini, tidak ada tanggapan dari Saudara.\n")
// 								sbID.WriteString("Hal tersebut menunjukkan kelalaian atas peringatan yang diberikan.\n")
// 								sbID.WriteString("Selain itu, teknisi di bawah pengawasan Saudara kembali melakukan pelanggaran.\n")
// 								sbID.WriteString("Pelanggaran tersebut berujung pada penerbitan SP-3.\n")
// 								sbID.WriteString("Dengan demikian, perusahaan menerbitkan SP-2 untuk Saudara.\n\n")
// 								sbID.WriteString("Mohon menjadi perhatian serius.\n")
// 								sbID.WriteString("Terima kasih.")

// 								var sbEN strings.Builder
// 								sbEN.WriteString(fmt.Sprintf("Referring to the SP-1 that has been given to you %s on %s,\n", namaSPL, tanggalSP1Terkirim))
// 								sbEN.WriteString("to date, there has been no response from you.\n")
// 								sbEN.WriteString("This indicates negligence of the warning given.\n")
// 								sbEN.WriteString("Furthermore, the technicians under your supervision have committed violations again.\n")
// 								sbEN.WriteString("These violations resulted in the issuance of SP-3.\n")
// 								sbEN.WriteString("Therefore, the company issues SP-2 to you.\n\n")
// 								sbEN.WriteString("Please treat this matter with utmost seriousness.\n")
// 								sbEN.WriteString("Thank you.")

// 								var msgIDsb strings.Builder
// 								var msgENsb strings.Builder
// 								msgIDsb.WriteString(fmt.Sprintf("Halo, %s Pak %s.\n\n", greetingID, namaSPL))
// 								msgIDsb.WriteString(sbID.String())
// 								msgENsb.WriteString(fmt.Sprintf("Hello, %s Mr. %s.\n\n", greetingEN, namaSPL))
// 								msgENsb.WriteString(sbEN.String())

// 								idMsg := msgIDsb.String()
// 								enMsg := msgENsb.String()

// 								sendLangDocumentMessageForSPSPL(forProject, spl, jidStrSPL, idMsg, enMsg, "id", pdfSPLSP2FilePath, 2, "62"+sanitizedNOHPSPL)

// 								// ADD: SPL SP 2 send to Mr. Oliver if needed
// 								if !config.GetConfig().SPTechnician.ActiveDebug {
// 									for _, hrd := range dataHRD {
// 										var msgIDsb strings.Builder
// 										var msgENsb strings.Builder
// 										msgIDsb.WriteString(fmt.Sprintf("Halo, %s Kak %s.\n\n", greetingID, hrd.Name))
// 										msgIDsb.WriteString(sbID.String())
// 										msgENsb.WriteString(fmt.Sprintf("Hello, %s Sist %s.\n\n", greetingEN, hrd.Name))
// 										msgENsb.WriteString(sbEN.String())

// 										idMsg := msgIDsb.String()
// 										enMsg := msgENsb.String()

// 										jidStrHRD := fmt.Sprintf("%s@%s", hrd.PhoneNumber, "s.whatsapp.net")
// 										sendLangDocumentMessageForSPSPL(
// 											forProject,
// 											spl,
// 											jidStrHRD,
// 											idMsg,
// 											enMsg,
// 											"id",
// 											pdfSPLSP2FilePath,
// 											2,
// 											hrd.PhoneNumber,
// 										)
// 									}
// 									if jidStrSAC != "" {
// 										if strings.Contains(strings.ToLower(SACData.FullName), "tetty") {
// 											var msgIDsb strings.Builder
// 											var msgENsb strings.Builder
// 											msgIDsb.WriteString(fmt.Sprintf("Halo, %s Kak %s.\n\n", greetingID, SACData.FullName))
// 											msgIDsb.WriteString(sbID.String())
// 											msgENsb.WriteString(fmt.Sprintf("Hello, %s Sist %s.\n\n", greetingEN, SACData.FullName))
// 											msgENsb.WriteString(sbEN.String())

// 											idMsg := msgIDsb.String()
// 											enMsg := msgENsb.String()

// 											jidStrSAC := fmt.Sprintf("%s@%s", SACData.PhoneNumber, "s.whatsapp.net")
// 											sendLangDocumentMessageForSPSPL(
// 												forProject,
// 												spl,
// 												jidStrSAC,
// 												idMsg,
// 												enMsg,
// 												"id",
// 												pdfSPLSP2FilePath,
// 												2,
// 												SACData.PhoneNumber,
// 											)
// 										} else {
// 											var msgIDsb strings.Builder
// 											var msgENsb strings.Builder
// 											msgIDsb.WriteString(fmt.Sprintf("Halo, %s Pak %s.\n\n", greetingID, SACData.FullName))
// 											msgIDsb.WriteString(sbID.String())
// 											msgENsb.WriteString(fmt.Sprintf("Hello, %s Mr. %s.\n\n", greetingEN, SACData.FullName))
// 											msgENsb.WriteString(sbEN.String())

// 											idMsg := msgIDsb.String()
// 											enMsg := msgENsb.String()

// 											jidStrSAC := fmt.Sprintf("%s@%s", SACData.PhoneNumber, "s.whatsapp.net")
// 											sendLangDocumentMessageForSPSPL(
// 												forProject,
// 												spl,
// 												jidStrSAC,
// 												idMsg,
// 												enMsg,
// 												"id",
// 												pdfSPLSP2FilePath,
// 												2,
// 												SACData.PhoneNumber,
// 											)
// 										}
// 									}
// 								}

// 								if existingInfo, exists := SPLGotSPToday[spl]; exists {
// 									// Update existing record
// 									existingInfo.GotSPToday = true
// 									existingInfo.SPNumber = 2
// 									SPLGotSPToday[spl] = existingInfo
// 								} else {
// 									// Create new record
// 									SPLGotSPToday[spl] = SPInfoForSPL{
// 										GotSPToday: true,
// 										SPNumber:   2,
// 									}
// 								}
// 							} else {
// 								logrus.Warnf("No valid phone number for SPL %s, cannot send SPL SP 2", spl)
// 								continue
// 							}
// 						} // .end of no on-time replies of SPL's SP 1
// 					} // .end of SPL get SP1 but not SP2

// 					// SPL GOT SP 3
// 					if splGotSP1 && splGotSP2 && !splGotSP3 {
// 						// 1. Find the SP2 msg of SPL
// 						var firstSP2MsgSPL sptechnicianmodel.SPWhatsAppMessage
// 						if err := dbWeb.
// 							Where("spl_got_sp_id = ? AND number_of_sp = ?", dataSPSPL.ID, 2).
// 							Where("for_project = ?", forProject).
// 							Order("whatsapp_sent_at asc").
// 							First(&firstSP2MsgSPL).Error; err != nil {
// 							logrus.Warnf("Could not find the SP2 message for SPL %s to determine sent time for SP3 issuance: %v", spl, err)
// 							continue
// 						}
// 						if firstSP2MsgSPL.WhatsappSentAt == nil {
// 							logrus.Warnf("First SP2 message sent time is nil for SPL %s, cannot create SP 3", spl)
// 							continue
// 						}

// 						// 2. Calculate the reply deadline
// 						sentAt := *firstSP2MsgSPL.WhatsappSentAt
// 						deadline := time.Date(sentAt.Year(), sentAt.Month(), sentAt.Day(), maxResponseSPAtHour, 0, 0, 0, sentAt.Location())

// 						// 3. Count replies received before the deadline
// 						var onTimeReplyCount int64
// 						dbWeb.Model(&sptechnicianmodel.SPWhatsAppMessage{}).
// 							Where("spl_got_sp_id = ?", dataSPSPL.ID).
// 							Where("number_of_sp = ?", 2).
// 							Where("for_project = ?", forProject).
// 							Where("whatsapp_reply_text IS NOT NULL AND whatsapp_reply_text != ''").
// 							Where("whatsapp_replied_at <= ?", deadline).
// 							Count(&onTimeReplyCount)

// 						// 4. Proceed only if no on-time replies were found
// 						if onTimeReplyCount == 0 {
// 							if firstSP2MsgSPL.WhatsappSentAt == nil {
// 								logrus.Warnf("First SP2 message sent time is nil for SPL %s, cannot create SP 3", spl)
// 								continue
// 							}

// 							// Check if technician name caused SPL got SP2 is the same as current technician
// 							if dataSPSPL.TechnicianNameCausedGotSP2 == record.Technician || dataSPSPL.TechnicianNameCausedGotSP1 == record.Technician {
// 								// SKIP because already got SP 2 for the same technician
// 								logrus.Infof("SPL %s already got SP 2 for technician %s, skipping SP 3 issuance", spl, record.Technician)
// 								continue
// 							}

// 							if _, exists := SPLGotSPToday[spl]; exists && SPLGotSPToday[spl].GotSPToday && SPLGotSPToday[spl].SPNumber == 2 {
// 								// SKIP SPL already got SP today, skip SP 3 issuance
// 								logrus.Infof("SPL %s already got SP today, skipping SP 3 issuance", spl)
// 								continue
// 							}

// 							if dataSPSPL.SP3FilePath != "" {
// 								// SKIP because already got SP 3
// 								logrus.Infof("SPL %s already got SP 3, skipping SP 3 issuance", spl)
// 								continue
// 							}

// 							tgl, err := tanggal.Papar(*firstSP2MsgSPL.WhatsappSentAt, "Jakarta", tanggal.WIB)
// 							if err != nil {
// 								logrus.Errorf("Failed to format date for SPL SP 3: %v", err)
// 								continue
// 							}
// 							tanggalSP2Terkirim := tgl.Format(" ", []tanggal.Format{
// 								tanggal.NamaHariDenganKoma, // Kamis,
// 								tanggal.Hari,               // 27
// 								tanggal.NamaBulan,          // Maret
// 								tanggal.Tahun,              // 2025
// 								tanggal.Pukul,
// 								tanggal.ZonaWaktu,
// 							})

// 							tglSP3, err := tanggal.Papar(now, "Jakarta", tanggal.WIB)
// 							if err != nil {
// 								logrus.Errorf("Failed to get formatted date for SPL SP3: %v", err)
// 								continue
// 							}
// 							tglSP3Diterbitkan := tglSP3.Format(" ", []tanggal.Format{
// 								tanggal.Hari,      // 27
// 								tanggal.NamaBulan, // Maret
// 								tanggal.Tahun,     // 2025
// 							})

// 							SP3TextPart1 := fmt.Sprintf("Merujuk pada Surat Peringatan (SP-2) yang telah disampaikan kepada Saudara %s", namaSPL)
// 							SP3TextPart2 := fmt.Sprintf("pada tanggal %s.", tanggalSP2Terkirim)
// 							SP3TextPart3 := "perusahaan menilai bahwa pelanggaran yang saudara lakukan tidak kunjung diperbaiki."
// 							SP3TextPart4 := "Hal ini menunjukkan sikap yang tidak responsif terhadap peringatan yang telah diberikan."
// 							SP3TextPart5 := fmt.Sprintf("Selain itu, teknisi %s di bawah pengawasan Saudara", namaTeknisi)
// 							SP3TextPart6 := "kembali melakukan pelanggaran yang berujung pada penerbitan SP-3."
// 							SP3TextPart7 := "Dengan demikian, perusahaan dengan berat hati menerbitkan SP-3 untuk Saudara."
// 							SP3TextPart8 := "Surat ini juga menyatakan berakhirnya hubungan kerja Saudara dengan perusahaan."
// 							SP3TextPart9 := "Keputusan berlaku efektif sejak tanggal diterbitkannya surat ini."
// 							SP3TextPart10 := fmt.Sprintf("yakni pada tanggal %s.", tglSP3Diterbitkan)
// 							SP3TextPart11 := "Kami mengucapkan terima kasih atas kontribusi Saudara selama ini."
// 							SP3TextPart12 := "Semoga Saudara mendapatkan kesuksesan di masa depan. terima kasih..."

// 							fileNameForSPLSP3 := fmt.Sprintf("%s_SP3_SPL", strings.ReplaceAll(spl, "*", "Resigned"))
// 							fileTTS, err := fun.CreateRobustTTS(speech, audioForSPDir, []string{
// 								SP3TextPart1,
// 								SP3TextPart2,
// 								SP3TextPart3,
// 								SP3TextPart4,
// 								SP3TextPart5,
// 								SP3TextPart6,
// 								SP3TextPart7,
// 								SP3TextPart8,
// 								SP3TextPart9,
// 								SP3TextPart10,
// 								SP3TextPart11,
// 								SP3TextPart12,
// 							}, fileNameForSPLSP3)
// 							if err != nil {
// 								logrus.Errorf("Failed to create merged SPL SP3 TTS file: %v", err)
// 								continue
// 							}

// 							// Debug: Check the created file
// 							if fileTTS != "" {
// 								fileInfo, statErr := os.Stat(fileTTS)
// 								if statErr == nil {
// 									logrus.Debugf("🔊 SPL SP3 merged TTS file created: %s, Size: %d bytes", fileTTS, fileInfo.Size())
// 								} else {
// 									logrus.Errorf("🔊 SPL SP3 TTS file stat error: %v", statErr)
// 								}
// 							}

// 							noSurat, err := IncrementNomorSuratSP(dbWeb, "LAST_NOMOR_SURAT_SP3_GENERATED")
// 							if err != nil {
// 								logrus.Errorf("Failed to increment nomor surat SP-3 for SPL: %v", err)
// 								continue
// 							}
// 							var noSuratStr string
// 							if noSurat < 1000 {
// 								noSuratStr = fmt.Sprintf("%03d", noSurat)
// 							} else {
// 								noSuratStr = fmt.Sprintf("%d", noSurat)
// 							}

// 							monthRoman, err := fun.MonthToRoman(int(now.Month()))
// 							if err != nil {
// 								logrus.Errorf("Failed to convert month to roman numeral: %v", err)
// 								continue
// 							}
// 							splCity := getSPLCity(spl)
// 							if splCity == "" {
// 								logrus.Warnf("SPL city not found for SPL %s, defaulting to 'Unknown'", spl)
// 								splCity = "Unknown"
// 							}

// 							var tglSp3Ts time.Time
// 							var sp3WAMsgOfTechnician sptechnicianmodel.SPWhatsAppMessage
// 							if err := dbWeb.
// 								Where("technician_got_sp_id = ? AND number_of_sp = ?", dataSPLatest.ID, 3).
// 								Where("for_project = ?", forProject).
// 								Order("whatsapp_sent_at asc").
// 								First(&sp3WAMsgOfTechnician).Error; err != nil {
// 								logrus.Warnf("Could not find the SP3 message for technician %s to determine sent time for SPL SP2 escalation: %v", record.Technician, err)
// 								tglSp3Ts = now
// 							}

// 							if sp3WAMsgOfTechnician.WhatsappSentAt != nil {
// 								tglSp3Ts = *sp3WAMsgOfTechnician.WhatsappSentAt
// 							}

// 							tglSP3Teknisi, err := tanggal.Papar(tglSp3Ts, "Jakarta", tanggal.WIB)
// 							if err != nil {
// 								logrus.Errorf("Failed to format SP3 sent date for technician %s: %v", record.Technician, err)
// 								continue
// 							}
// 							tanggalSP3Terkirim := tglSP3Teknisi.Format(" ", []tanggal.Format{
// 								tanggal.NamaHariDenganKoma, // Kamis,
// 								tanggal.Hari,               // 27
// 								tanggal.NamaBulan,          // Maret
// 								tanggal.Tahun,              // 2025
// 								tanggal.Pukul,
// 								tanggal.ZonaWaktu,
// 							})

// 							pelanggaranSP3SPLID := fmt.Sprintf("Kurangnya pengawasan dan pembinaan terhadap teknisi %s, sehingga teknisi tersebut melakukan pelanggaran yang berujung pada penerbitan Surat Peringatan (SP-3) pada %v.", namaTeknisi, tanggalSP3Terkirim)

// 							// Simple replacements map
// 							placeholdersSPLSP3 := map[string]string{
// 								"$nomor_surat":            noSuratStr,
// 								"$bulan_romawi":           monthRoman,
// 								"$tahun_sp":               now.Format("2006"),
// 								"$nama_spl":               namaSPL,
// 								"$jabatan_spl":            fmt.Sprintf("Service Point Leader %s", splCity),
// 								"$pelanggaran_karyawan":   pelanggaranSP3SPLID,
// 								"$nama_teknisi":           namaTeknisi,
// 								"$tanggal_sp_diterbitkan": tglSP3Diterbitkan,
// 								"$personalia_name":        config.GetConfig().Default.PTHRD[1].Name,
// 								"$sac_name":               SACData.FullName,
// 								"$sac_ttd":                SACData.TTDPath,
// 								"$record_spl":             spl,
// 								"$for_project":            forProject,
// 							}
// 							selectedMainDir, err := fun.FindValidDirectory([]string{
// 								"web/file/sp_spl",
// 								"../web/file/sp_spl",
// 								"../../web/file/sp_spl",
// 							})
// 							if err != nil {
// 								logrus.Errorf("Failed to find valid directory for SP technician PDF files: %v", err)
// 								continue
// 							}
// 							fileDir := filepath.Join(selectedMainDir, time.Now().Format("2006-01-02"))
// 							if err := os.MkdirAll(fileDir, 0755); err != nil {
// 								logrus.Errorf("Failed to create PDF directory %s: %v", fileDir, err)
// 								continue
// 							}

// 							pdfFileName := fmt.Sprintf("SP_3_%s_%s.pdf",
// 								strings.ReplaceAll(spl, "*", "Resigned"),
// 								now.Format("2006-01-02"),
// 							)
// 							pdfSPLSP3FilePath := filepath.Join(fileDir, pdfFileName)
// 							if err := CreatePDFSP3ForSPL(placeholdersSPLSP3, pdfSPLSP3FilePath); err != nil {
// 								logrus.Errorf("Failed to create PDF for SPL SP 3: %v", err)
// 								continue
// 							}

// 							dataSPSPLUpdate := sptechnicianmodel.SPLGotSP{
// 								IsGotSP3:                   true, // SPL got SP 3
// 								PelanggaranSP3:             pelanggaranSP3SPLID,
// 								TechnicianNameCausedGotSP3: record.Technician,
// 								SP3SoundTTSPath:            fileTTS,
// 								SP3FilePath:                pdfSPLSP3FilePath,
// 							}

// 							if err := dbWeb.
// 								Model(&dataSPSPL).
// 								Where("for_project = ? AND spl = ? AND is_got_sp2 = ?", forProject, spl, true).
// 								Updates(&dataSPSPLUpdate).Error; err != nil {
// 								logrus.Errorf("Failed to update SP 3 data for SPL %s: %v", spl, err)
// 								continue
// 							}

// 							if sanitizedNOHPSPL != "" {
// 								jidStrSPL := fmt.Sprintf("62%s@%s", sanitizedNOHPSPL, "s.whatsapp.net")

// 								var sbID strings.Builder
// 								sbID.WriteString(fmt.Sprintf("Merujuk pada Surat Peringatan (SP-2) yang telah disampaikan kepada Saudara %s pada tanggal %s,\n", namaSPL, tanggalSP2Terkirim))
// 								sbID.WriteString("perusahaan menilai bahwa pelanggaran yang saudara lakukan tidak kunjung diperbaiki.\n")
// 								sbID.WriteString("Hal ini menunjukkan sikap yang tidak responsif terhadap peringatan yang telah diberikan.\n")
// 								sbID.WriteString("Selain itu, teknisi di bawah pengawasan Saudara kembali melakukan pelanggaran yang berujung pada penerbitan SP-3.\n")
// 								sbID.WriteString("Dengan demikian, perusahaan dengan berat hati menerbitkan SP-3 untuk Saudara.\n")
// 								sbID.WriteString("Surat ini juga menyatakan berakhirnya hubungan kerja Saudara dengan perusahaan.\n")
// 								sbID.WriteString("Keputusan berlaku efektif sejak tanggal diterbitkannya surat ini, yakni pada tanggal " + tglSP3Diterbitkan + ".\n\n")
// 								sbID.WriteString("Kami mengucapkan terima kasih atas kontribusi Saudara selama ini.\n")
// 								sbID.WriteString("Semoga Saudara mendapatkan kesuksesan di masa depan.\n")
// 								sbID.WriteString("Terima kasih.")

// 								var sbEN strings.Builder
// 								sbEN.WriteString(fmt.Sprintf("Referring to the Warning Letter (SP-2) that has been conveyed to you %s on %s,\n", namaSPL, tanggalSP2Terkirim))
// 								sbEN.WriteString("the company assesses that the violations you have committed have not been corrected.\n")
// 								sbEN.WriteString("This indicates an unresponsive attitude towards the warnings that have been given.\n")
// 								sbEN.WriteString("Furthermore, the technicians under your supervision have committed violations again that resulted in the issuance of SP-3.\n")
// 								sbEN.WriteString("Therefore, the company with a heavy heart issues SP-3 to you.\n")
// 								sbEN.WriteString("This letter also states the termination of your employment with the company.\n")
// 								sbEN.WriteString("The decision is effective from the date of issuance of this letter, namely on " + tglSP3Diterbitkan + ".\n\n")
// 								sbEN.WriteString("We thank you for your contributions thus far.\n")
// 								sbEN.WriteString("We wish you success in the future.\n")
// 								sbEN.WriteString("Thank you.")

// 								var msgIDsb strings.Builder
// 								var msgENsb strings.Builder
// 								msgIDsb.WriteString(fmt.Sprintf("Halo, %s Pak %s.\n\n", greetingID, namaSPL))
// 								msgIDsb.WriteString(sbID.String())
// 								msgENsb.WriteString(fmt.Sprintf("Hello, %s Mr. %s.\n\n", greetingEN, namaSPL))
// 								msgENsb.WriteString(sbEN.String())

// 								idMsg := msgIDsb.String()
// 								enMsg := msgENsb.String()

// 								sendLangDocumentMessageForSPSPL(forProject, spl, jidStrSPL, idMsg, enMsg, "id", pdfSPLSP3FilePath, 3, "62"+sanitizedNOHPSPL)

// 								// ADD: SPL SP 3 send to Mr. Oliver if needed
// 								if !config.GetConfig().SPTechnician.ActiveDebug {
// 									for _, hrd := range dataHRD {
// 										var msgIDsb strings.Builder
// 										var msgENsb strings.Builder
// 										msgIDsb.WriteString(fmt.Sprintf("Halo, %s Kak %s.\n\n", greetingID, hrd.Name))
// 										msgIDsb.WriteString(sbID.String())
// 										msgENsb.WriteString(fmt.Sprintf("Hello, %s Sist %s.\n\n", greetingEN, hrd.Name))
// 										msgENsb.WriteString(sbEN.String())

// 										idMsg := msgIDsb.String()
// 										enMsg := msgENsb.String()

// 										jidStrHRD := fmt.Sprintf("%s@%s", hrd.PhoneNumber, "s.whatsapp.net")
// 										sendLangDocumentMessageForSPSPL(
// 											forProject,
// 											spl,
// 											jidStrHRD,
// 											idMsg,
// 											enMsg,
// 											"id",
// 											pdfSPLSP3FilePath,
// 											3,
// 											hrd.PhoneNumber,
// 										)
// 									}
// 									if jidStrSAC != "" {
// 										if strings.Contains(strings.ToLower(SACData.FullName), "tetty") {
// 											var msgIDsb strings.Builder
// 											var msgENsb strings.Builder
// 											msgIDsb.WriteString(fmt.Sprintf("Halo, %s Kak %s.\n\n", greetingID, SACData.FullName))
// 											msgIDsb.WriteString(sbID.String())
// 											msgENsb.WriteString(fmt.Sprintf("Hello, %s Sist %s.\n\n", greetingEN, SACData.FullName))
// 											msgENsb.WriteString(sbEN.String())

// 											idMsg := msgIDsb.String()
// 											enMsg := msgENsb.String()

// 											jidStrSAC := fmt.Sprintf("%s@%s", SACData.PhoneNumber, "s.whatsapp.net")
// 											sendLangDocumentMessageForSPSPL(
// 												forProject,
// 												spl,
// 												jidStrSAC,
// 												idMsg,
// 												enMsg,
// 												"id",
// 												pdfSPLSP3FilePath,
// 												3,
// 												SACData.PhoneNumber,
// 											)
// 										} else {
// 											var msgIDsb strings.Builder
// 											var msgENsb strings.Builder
// 											msgIDsb.WriteString(fmt.Sprintf("Halo, %s Pak %s.\n\n", greetingID, SACData.FullName))
// 											msgIDsb.WriteString(sbID.String())
// 											msgENsb.WriteString(fmt.Sprintf("Hello, %s Mr. %s.\n\n", greetingEN, SACData.FullName))
// 											msgENsb.WriteString(sbEN.String())

// 											idMsg := msgIDsb.String()
// 											enMsg := msgENsb.String()

// 											jidStrSAC := fmt.Sprintf("%s@%s", SACData.PhoneNumber, "s.whatsapp.net")
// 											sendLangDocumentMessageForSPSPL(
// 												forProject,
// 												spl,
// 												jidStrSAC,
// 												idMsg,
// 												enMsg,
// 												"id",
// 												pdfSPLSP3FilePath,
// 												3,
// 												SACData.PhoneNumber,
// 											)
// 										}
// 									}
// 								}

// 								if existingInfo, exists := SPLGotSPToday[spl]; exists {
// 									// Update existing record
// 									existingInfo.GotSPToday = true
// 									existingInfo.SPNumber = 3
// 									SPLGotSPToday[spl] = existingInfo
// 								} else {
// 									// Create new record
// 									SPLGotSPToday[spl] = SPInfoForSPL{
// 										GotSPToday: true,
// 										SPNumber:   3,
// 									}
// 								}
// 							}
// 						} // .end of no on-time replies of SPL's SP 2
// 					} // .end of SPL get SP2 but not SP3
// 				}
// 			}
// 			// #######################################################################################################################################################
// 		}
// 	}

// 	return nil
// }

func GetDataTechnicianPlannedForToday() error {
	taskDoing := "Get Data Technician Planned For Today"
	if !getDataTechnicianPlannedForTodayMutex.TryLock() {
		return fmt.Errorf("%s is already in progress, please wait", taskDoing)
	}
	defer getDataTechnicianPlannedForTodayMutex.Unlock()

	GetDataTechnicianODOOMS()

	loc, _ := time.LoadLocation(config.GetConfig().Default.Timezone)
	now := time.Now().In(loc)

	startOfDay := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, loc)
	endOfDay := time.Date(now.Year(), now.Month(), now.Day(), 23, 59, 59, 0, loc)
	startOfDay = startOfDay.Add(-7 * time.Hour) // Adjust to UTC-7
	endOfDay = endOfDay.Add(-7 * time.Hour)     // Adjust to UTC-7

	startDateParam := startOfDay.Format("2006-01-02 15:04:05")
	endDateParam := endOfDay.Format("2006-01-02 15:04:05")

	ODOOModel := "project.task"
	excludedStages := config.GetConfig().SPTechnician.ExcludeStages

	domain := []interface{}{
		[]interface{}{"planned_date_begin", ">=", startDateParam},
		[]interface{}{"planned_date_begin", "<=", endDateParam},
	}

	if len(excludedStages) > 0 {
		domain = append(domain, []interface{}{"stage_id", "!=", excludedStages})
	}

	if config.GetConfig().SPTechnician.UncheckLinkPhoto {
		domain = append(domain, []interface{}{"x_link_photo", "=", false}) // // no link photo, coz sometimes its stage New but got photo, means the data already uploaded by Technician but didint feedbacked yet by TA
	}

	fieldsID := []string{
		"id",
	}

	fields := []string{
		"planned_date_begin",
		"technician_id",
		"helpdesk_ticket_id",
		"x_no_task",
		"timesheet_timer_last_stop",
		"x_latitude",
		"x_longitude",
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
		return err
	}

	ODOOresponse, err := GetODOOMSData(string(payloadBytes))
	if err != nil {
		errMsg := fmt.Sprintf("failed fetching data from ODOO MS API: %v", err)
		return errors.New(errMsg)
	}

	ODOOResponseArray, ok := ODOOresponse.([]interface{})
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
	semaphore := make(chan struct{}, 5) // Limit to 5 concurrent goroutines

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
		return errors.New("no data found from ODOO in all chunks")
	}

	ODOOResponseBytes, err := json.Marshal(allRecords)
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
	if err := clearOldTechnicianData(); err != nil {
		logrus.Errorf("Failed to clear old technician data: %v", err)
		// Continue anyway - we might be updating existing data
	}

	// Group data by technician and create aggregated records
	groupedData := groupDataByTechnician(listOfData)

	// Use a single transaction for all database operations to improve performance
	tx := dbWeb.Begin()
	if tx.Error != nil {
		return fmt.Errorf("failed to begin transaction: %v", tx.Error)
	}

	// Process grouped data in batches
	const dbBatchSize = 1000
	var batch []sptechnicianmodel.JOPlannedForTechnicianODOOMS
	batchCount := 0

	for _, record := range groupedData {
		batch = append(batch, record)
		batchCount++

		// Insert batch when it reaches the batch size or at the end
		if len(batch) >= dbBatchSize || batchCount == len(groupedData) {
			if err := tx.Model(&sptechnicianmodel.JOPlannedForTechnicianODOOMS{}).Create(batch).Error; err != nil {
				if rollbackErr := tx.Rollback().Error; rollbackErr != nil {
					logrus.Errorf("Failed to rollback transaction: %v", rollbackErr)
				}
				return fmt.Errorf("failed to insert batch of (%s) data to DB: %v", taskDoing, err)
			}

			// Log progress
			// logrus.Infof("Progress: processed %d/%d technician records", batchCount, len(groupedData))

			// Reset batch
			batch = make([]sptechnicianmodel.JOPlannedForTechnicianODOOMS, 0, dbBatchSize)
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
	if err := updateTechnicianLastVisitFromBatch(listOfData); err != nil {
		logrus.Errorf("Failed to update technician last visit times: %v", err)
		// Don't return error as the main data insertion was successful
	}

	// Update technician first uploaded times from the batch data
	if err := updateTechnicianFirstUploadFromBatch(listOfData); err != nil {
		logrus.Errorf("Failed to update technician first upload times: %v", err)
		// Don't return error as the main data insertion was successful
	}

	return nil
}

func ResetNomorSuratSP() error {
	dbWeb := gormdb.Databases.Web
	err := dbWeb.Model(&sptechnicianmodel.NomorSuratSP{}).
		Where("1 = 1"). // force all rows
		UpdateColumn("last_nomor_surat_sp", 1).Error
	if err != nil {
		return fmt.Errorf("failed to reset NomorSuratSP: %v", err)
	}

	return nil
}

// ResetTechnicianSP resets SP status for a technician if replied to previous SP
func ResetTechnicianSP(technician string, forProject string) error {
	dbWeb := gormdb.Databases.Web

	// 1. Find the active SP record for the technician.
	var sp sptechnicianmodel.TechnicianGotSP
	err := dbWeb.
		Where("technician = ?", technician).
		Where("for_project = ?", forProject).
		Where("is_got_sp3 = ?", false).
		First(&sp).Error

	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			// If no record is found, there's nothing to reset. This is normal.
			// logrus.Infof("No SP record found for technician %s to reset, which is fine.", technician)
			return nil
		}
		return err // Handle other potential database errors.
	}

	// 2. Check if ANY associated message has been replied to.
	// We only need to find one. The database will stop searching as soon as it finds a match.
	var repliedMessage sptechnicianmodel.SPWhatsAppMessage
	err = dbWeb.
		Where("technician_got_sp_id = ?", sp.ID).
		Where("whatsapp_reply_text IS NOT NULL AND whatsapp_reply_text != ''").
		First(&repliedMessage).Error

	// 3. Decide what to do based on the result.
	if err == nil {
		// A replied message WAS found (err is nil).
		// This means the technician has acknowledged the SP.
		// Now, we delete the SP record to reset their status.
		if deleteErr := dbWeb.Delete(&sp).Error; deleteErr != nil {
			return fmt.Errorf("failed to delete TechnicianGotSP record after finding reply: %v", deleteErr)
		}
		logrus.Infof("Successfully reset SP for technician %s by deleting the record.", technician)
		return nil
	}

	if errors.Is(err, gorm.ErrRecordNotFound) {
		// No replied message was found. This is the "not replied yet" case.
		// We do nothing because the technician has not acknowledged the SP yet.
		return nil
	}

	// A different error occurred while checking for messages.
	return fmt.Errorf("failed to check for replied messages: %v", err)
}

// ResetSPLSP resets SP status for a SPL if replied to previous SP
func ResetSPLSP(spl string, forProject string) error {
	dbWeb := gormdb.Databases.Web

	var sp sptechnicianmodel.SPLGotSP
	err := dbWeb.
		Where("spl = ?", spl).
		Where("for_project = ?", forProject).
		Where("is_got_sp3 = ?", false).
		First(&sp).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			// If no record is found, there's nothing to reset. This is normal.
			// logrus.Infof("No SP record found for SPL %s to reset, which is fine.", spl)
			return nil
		}
		return err // This will correctly return gorm.ErrRecordNotFound if no record is found
	}

	// Check if ANY associated message has been replied to.
	var repliedMessageCount int64
	err = dbWeb.Model(&sptechnicianmodel.SPWhatsAppMessage{}).
		Where("spl_got_sp_id = ?", sp.ID).
		Where("whatsapp_reply_text IS NOT NULL AND whatsapp_reply_text != ''").
		Count(&repliedMessageCount).Error

	if err == nil {
		// A replied message WAS found (err is nil).
		if repliedMessageCount > 0 {
			// This means the SPL has acknowledged the SP.
			// Now, we delete the SP record to reset their status.
			if deleteErr := dbWeb.Delete(&sp).Error; deleteErr != nil {
				return fmt.Errorf("failed to delete SPLGotSP record after finding reply: %v", deleteErr)
			}
			logrus.Infof("Successfully reset SP for SPL %s by deleting the record.", spl)
		}
		return nil
	}

	if errors.Is(err, gorm.ErrRecordNotFound) || repliedMessageCount == 0 {
		// No replied message was found. This is the "not replied yet" case.
		// We do nothing because the SPL has not acknowledged the SP yet.
		return nil
	}

	// A different error occurred while checking for messages.
	return fmt.Errorf("failed to check for replied messages: %v", err)
}

// ResetSACSP resets SP status for a SAC if replied to previous SP
func ResetSACSP(sac string, forProject string) error {
	dbWeb := gormdb.Databases.Web

	var sp sptechnicianmodel.SACGotSP
	err := dbWeb.
		Where("sac = ?", sac).
		Where("for_project = ?", forProject).
		Where("is_got_sp3 = ?", false).
		First(&sp).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			// If no record is found, there's nothing to reset. This is normal.
			// logrus.Infof("No SP record found for SAC %s to reset, which is fine.", sac)
			return nil
		}
		return err // This will correctly return gorm.ErrRecordNotFound if no record is found
	}

	// Check if ANY associated message has been replied to.
	var repliedMessageCount int64
	err = dbWeb.Model(&sptechnicianmodel.SPWhatsAppMessage{}).
		Where("sac_got_sp_id = ?", sp.ID).
		Where("whatsapp_reply_text IS NOT NULL AND whatsapp_reply_text != ''").
		Count(&repliedMessageCount).Error

	if err == nil {
		// A replied message WAS found (err is nil).
		if repliedMessageCount > 0 {
			// This means the SAC has acknowledged the SP.
			// Now, we delete the SP record to reset their status.
			if deleteErr := dbWeb.Delete(&sp).Error; deleteErr != nil {
				return fmt.Errorf("failed to delete SACGotSP record after finding reply: %v", deleteErr)
			}
			logrus.Infof("Successfully reset SP for SAC %s by deleting the record.", sac)
		}
		return nil
	}

	if errors.Is(err, gorm.ErrRecordNotFound) || repliedMessageCount == 0 {
		// No replied message was found. This is the "not replied yet" case.
		// We do nothing because the SAC has not acknowledged the SP yet.
		return nil
	}

	// A different error occurred while checking for messages.
	return fmt.Errorf("failed to check for replied messages: %v", err)
}

// GetTechnicianStockOpnameData retrieves stock opname data for technicians.
// stock opname data range based on parameter in config: NUMBER_OF_DAYS_JO_NOT_SO_YET
func GetTechnicianStockOpnameData() ([]string, map[string]*TechnicianStockOpnameAggregateData, error) {
	taskDoing := "Getting stock opname data for technicians"
	startTime := time.Now()
	logrus.Infof("starting task: %s @%v", taskDoing, startTime)

	if !getDataOfSOTechnicianMutex.TryLock() {
		return nil, nil, fmt.Errorf("another process is still running for task: %s", taskDoing)
	}
	defer getDataOfSOTechnicianMutex.Unlock()

	loc, _ := time.LoadLocation(config.GetConfig().Default.Timezone)
	timeNow := time.Now().In(loc)

	startDayScheduled := config.GetConfig().StockOpname.NumberOfDaysJONotSOYet
	startOfDay := time.Date(timeNow.Year(), timeNow.Month(), timeNow.Day(), 0, 0, 0, 0, loc)
	if startDayScheduled > 0 {
		startOfDay = startOfDay.AddDate(0, 0, -startDayScheduled)
	}
	endOfDay := time.Date(timeNow.Year(), timeNow.Month(), timeNow.Day(), 23, 59, 59, 0, loc)
	startOfDay = startOfDay.Add(-7 * time.Hour) // Adjust to UTC-7
	endOfDay = endOfDay.Add(-7 * time.Hour)     // Adjust to UTC-7

	startDateParam := startOfDay.Format("2006-01-02 15:04:05")
	endDateParam := endOfDay.Format("2006-01-02 15:04:05")

	ODOOModel := "stock.picking"

	domain := []any{
		[]any{"origin", "ilike", "STOCK OPNAME"},
		[]any{"state", "=", "draft"},
		[]any{"scheduled_date", ">=", startDateParam},
		[]any{"scheduled_date", "<=", endDateParam},
	}

	fieldID := []string{"id"}
	fields := []string{
		"id",
		"name",
		"partner_id",
		"picking_type_id",
		"location_id",
		"location_dest_id",
		"company_id",
		"user_id",
		"technician_id",
		"technician_id_fs",
		"scheduled_date",
		"origin",
		"location_dest_categ",
		"carrier",
		"tracking_number",
		"x_link",
		"state",
	}
	order := "technician_id_fs asc"
	odooParams := map[string]any{
		"model":  ODOOModel,
		"domain": domain,
		"fields": fieldID,
		"order":  order,
	}

	payload := map[string]any{
		"jsonrpc": config.GetConfig().ApiODOO.JSONRPC,
		"params":  odooParams,
	}

	payloadBytes, err := json.Marshal(payload)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to marshal payload: %v", err)
	}

	ODOOResp, err := GetODOOMSData(string(payloadBytes))
	if err != nil {
		return nil, nil, err
	}

	ODOORespArray, ok := ODOOResp.([]any)
	if !ok {
		errMsg := "Failed to convert ODOO response to array / []any"
		return nil, nil, errors.New(errMsg)
	}

	ids := extractUniqueIDs(ODOORespArray)

	if len(ids) == 0 {
		logrus.Infof("No Stock Opname found that match the criteria")
		return nil, nil, nil
	}

	const batchSize = 100
	chunks := chunkIdsSlice(ids, batchSize)
	var allRecords []any

	// Use workers to process chunks concurrently
	type chunkRes struct {
		records []any
		err     error
		index   int
	}

	resChan := make(chan chunkRes, len(chunks))
	semaphore := make(chan struct{}, 5) // Limit to 5 concurrent workers

	ctx, cancel := context.WithTimeout(context.Background(), 25*time.Minute)
	defer cancel()

	for i, chunk := range chunks {
		go func(chunkIndex int, chunkData []uint64) {
			defer func() {
				if r := recover(); r != nil {
					logrus.Errorf("Panic in chunk processing %d: %v", chunkIndex, r)
					resChan <- chunkRes{nil, fmt.Errorf("panic in chunk %d: %v", chunkIndex, r), chunkIndex}
				}
			}()

			select {
			case semaphore <- struct{}{}: // Acquire semaphore with timeout
			case <-ctx.Done():
				resChan <- chunkRes{nil, fmt.Errorf("chunk %d timeout", chunkIndex), chunkIndex}
				return
			}
			defer func() { <-semaphore }() // Release semaphore

			// logrus.Debugf("Processing (%s) chunk %d of %d (IDs %v to %v)", taskDoing, chunkIndex+1, len(chunks), chunkData[0], chunkData[len(chunkData)-1])

			chunkDomain := []interface{}{
				[]interface{}{"id", "=", chunkData},
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
				resChan <- chunkRes{nil, fmt.Errorf("failed to marshal payload for chunk %d: %v", chunkIndex+1, err), chunkIndex}
				return
			}

			ODOOresponse, err := GetODOOMSData(string(payloadBytes))
			if err != nil {
				resChan <- chunkRes{nil, fmt.Errorf("failed fetching data from ODOO MS API for chunk %d: %v", chunkIndex+1, err), chunkIndex}
				return
			}

			ODOOResponseArray, ok := ODOOresponse.([]interface{})
			if !ok {
				resChan <- chunkRes{nil, fmt.Errorf("type assertion failed for chunk %d", chunkIndex+1), chunkIndex}
				return
			}

			resChan <- chunkRes{ODOOResponseArray, nil, chunkIndex}
		}(i, chunk)
	}

	// Collect results from all goroutines with timeout protection
	results := make([]chunkRes, len(chunks))
	for i := 0; i < len(chunks); i++ {
		select {
		case result := <-resChan:
			if result.index < len(results) {
				results[result.index] = result
			}
		case <-ctx.Done():
			logrus.Errorf("Timeout waiting for chunk results")
			return nil, nil, errors.New("timeout waiting for chunk results")
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
		logrus.Infof("No Stock Opname records retrieved after processing all chunks")
		return nil, nil, nil
	}

	ODOOResponseBytes, err := json.Marshal(allRecords)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to marshal combined response: %v", err)
	}

	// Pre-allocate slice with estimated capacity to reduce memory allocations
	var listOfData []OdooStockPickingItem
	estimatedCapacity := len(allRecords) * 10 // Reduced from 50 to prevent over-allocation
	if estimatedCapacity > 50000 {            // Cap maximum pre-allocation
		estimatedCapacity = 50000
	}
	if estimatedCapacity > 0 {
		listOfData = make([]OdooStockPickingItem, 0, estimatedCapacity)
	}

	if err := json.Unmarshal(ODOOResponseBytes, &listOfData); err != nil {
		errMsg := fmt.Sprintf("failed to unmarshal response body: %v", err)
		return nil, nil, errors.New(errMsg)
	}

	// Log memory usage for monitoring
	var memStats runtime.MemStats
	runtime.ReadMemStats(&memStats)
	logrus.Infof("Memory Usage before processing Stock Opname: Alloc = %v MiB, TotalAlloc = %v MiB, Sys = %v MiB, NumGC = %v",
		memStats.Alloc/1024/1024,
		memStats.TotalAlloc/1024/1024,
		memStats.Sys/1024/1024,
		memStats.NumGC,
	)
	runtime.GC()

	if err := GetDataTechnicianODOOMS(); err != nil {
		return nil, nil, fmt.Errorf("failed to get Technician ODOO MS data: %v", err)
	}

	technicianMap := make(map[string]*TechnicianStockOpnameAggregateData)

	var techniciansGotSP []string

	// Process each Stock Opname record
	for _, odooData := range listOfData {
		var scheduledDate *time.Time
		if odooData.ScheduledDate.String != "" {
			parsedTime, err := time.Parse("2006-01-02 15:04:05", odooData.ScheduledDate.String)
			if err != nil {
				logrus.Errorf("Failed to parse scheduled_date for Stock Opname ID %d: %v", odooData.ID, err)
			} else {
				parsedTime = parsedTime.Add(7 * time.Hour)
				scheduledDate = &parsedTime
			}
		}

		_, technician := parseJSONIDDataCombinedSafe(odooData.TechnicianDestinationLocation)
		var spl, sac, techEmail, techNoHP, techName string
		if techData, exists := TechODOOMSData[technician]; exists {
			spl = techData.SPL
			sac = techData.SAC
			techEmail = techData.Email
			techNoHP = techData.NoHP
			if techData.Name != "" {
				techName = techData.Name
			} else {
				techName = technician
			}
		}

		_, responsible := parseJSONIDDataCombinedSafe(odooData.Responsible)
		_, sourceLoc := parseJSONIDDataCombinedSafe(odooData.SourceLocation)
		_, destLoc := parseJSONIDDataCombinedSafe(odooData.DestionationLocation)
		_, company := parseJSONIDDataCombinedSafe(odooData.Company)

		if technicianMap[technician] == nil {
			technicianMap[technician] = &TechnicianStockOpnameAggregateData{
				TechnicianName: technician,
				Name:           techName,
				SPL:            spl,
				SAC:            sac,
				TechEmail:      techEmail,
				TechNoHP:       techNoHP,
				DataSO:         []DataStockOpnameAggregate{},
			}
		}

		tech := technicianMap[technician]

		dataSO := DataStockOpnameAggregate{
			ID:                odooData.ID,
			TicketSO:          odooData.Name.String,
			ScheduledDate:     scheduledDate,
			Responsible:       responsible,
			SourceDocument:    odooData.SourceDocument.String,
			SourceLocation:    sourceLoc,
			LocDestinationCat: odooData.LocationDestinationCategory.String,
			DestLocation:      destLoc,
			Company:           company,
			LinkBAST:          odooData.LinkBast.String,
		}

		if !containsSO(tech.DataSO, odooData.ID) {
			tech.DataSO = append(tech.DataSO, dataSO)
		}

		techniciansGotSP = append(techniciansGotSP, strings.TrimSpace(technician))
	}

	if len(technicianMap) == 0 {
		logrus.Infof("No technicians found who need SP for Stock Opname")
		return nil, nil, nil
	}

	for _, tech := range technicianMap {
		sortDataSOByDateAsc(tech.DataSO)
	}

	techniciansGotSP = lo.Uniq(techniciansGotSP)
	sort.Strings(techniciansGotSP) // ASC order

	return techniciansGotSP, technicianMap, nil
}

// Check SP Technician, SPL & SAC
// It checks if technician not visit with minimum %d JO, not doing stock opname and optional: if its stock opname doing, had missing data
func CheckSPTechnicianV2() error {
	taskDoing := "v2 Checking technician/SPL/SAC got Surat Peringatan(SP)"
	startTime := time.Now()
	logrus.Infof("starting task: %s @%v", taskDoing, startTime)

	if !checkingSPTechnicianV2Mutex.TryLock() {
		return fmt.Errorf("another process is still running for task: %s", taskDoing)
	}
	defer checkingSPTechnicianV2Mutex.Unlock()

	dbWeb := gormdb.Databases.Web

	errDataPlanForToday := GetDataTechnicianPlannedForToday()
	if errDataPlanForToday != nil {
		logrus.Errorf("failed to get data of technician planned for today: %v", errDataPlanForToday)
	}

	// Start checking SP
	loc, _ := time.LoadLocation(config.GetConfig().Default.Timezone)
	now := time.Now().In(loc)

	tahunSP := now.Format("2006")
	monthRoman, err := fun.MonthToRoman(int(now.Month()))
	if err != nil {
		return fmt.Errorf("failed to convert month to roman numeral: %v", err)
	}

	startOfDay := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, loc)
	endOfDay := time.Date(now.Year(), now.Month(), now.Day(), 23, 59, 59, 0, loc)

	var dataJOPlanned []sptechnicianmodel.JOPlannedForTechnicianODOOMS
	result := dbWeb.
		Where("created_at >= ? AND created_at <= ?", startOfDay, endOfDay).
		Order("technician asc").
		Find(&dataJOPlanned)

	if result.Error != nil {
		if errors.Is(result.Error, gorm.ErrRecordNotFound) {
			logrus.Infof("no JO Planned for Technician found in range %s → %s", startOfDay.Format("2006-01-02 15:04:05"),
				endOfDay.Format("2006-01-02 15:04:05"))
		}
		return fmt.Errorf("failed to fetch JO Planned for Technician: %v", result.Error)
	}

	excludedTechnicians := []string{
		"*",
		"inhouse",
		"in house",
		"in-house",
		"vendor",
		"pameran",
		"tes dev",
		"admin",
		"edi purwanto",
	}
	atmDedicatedTechnician := config.GetConfig().SPTechnician.ATMDedicatedTechnician
	if len(atmDedicatedTechnician) > 0 {
		excludedTechnicians = append(excludedTechnicians, atmDedicatedTechnician...)
	}

	var tanggalIndoFormatted string
	tglNowIndo, err := tanggal.Papar(now, "Jakarta", tanggal.WIB)
	if err != nil {
		logrus.Errorf("failed to formatted current date into indonesia format: %v", err)
		tanggalIndoFormatted = now.Format("02 January 2006")
	} else {
		tanggalIndoFormatted = tglNowIndo.Format(" ", []tanggal.Format{
			tanggal.Hari,
			tanggal.NamaBulan,
			tanggal.Tahun,
		})
	}

	// Audio SP Directories
	audioSPTechDir, err := fun.FindValidDirectory([]string{
		"web/file/sounding_sp_technician",
		"../web/file/sounding_sp_technician",
		"../../web/file/sounding_sp_technician",
		"../../../web/file/sounding_sp_technician",
	})
	if err != nil {
		return fmt.Errorf("failed to find valid directory for sounding SP technician files: %v", err)
	}
	audioDirForSPTechnician := filepath.Join(audioSPTechDir, now.Format("2006-01-02"))
	if err := os.MkdirAll(audioDirForSPTechnician, 0755); err != nil {
		return fmt.Errorf("failed to create directory for sounding SP technician files: %v", err)
	}

	audioSPSPLDir, err := fun.FindValidDirectory([]string{
		"web/file/sounding_sp_spl",
		"../web/file/sounding_sp_spl",
		"../../web/file/sounding_sp_spl",
		"../../../web/file/sounding_sp_spl",
	})
	if err != nil {
		return fmt.Errorf("failed to find valid directory for sounding SP spl files: %v", err)
	}
	audioDirForSPSPL := filepath.Join(audioSPSPLDir, now.Format("2006-01-02"))
	if err := os.MkdirAll(audioDirForSPSPL, 0755); err != nil {
		return fmt.Errorf("failed to create directory for sounding SP SPL files: %v", err)
	}

	audioSPSACDir, err := fun.FindValidDirectory([]string{
		"web/file/sounding_sp_sac",
		"../web/file/sounding_sp_sac",
		"../../web/file/sounding_sp_sac",
		"../../../web/file/sounding_sp_sac",
	})
	if err != nil {
		return fmt.Errorf("failed to find valid directory for sounding SP SAC files: %v", err)
	}
	audioDirForSPSAC := filepath.Join(audioSPSACDir, now.Format("2006-01-02"))
	if err := os.MkdirAll(audioDirForSPSAC, 0755); err != nil {
		return fmt.Errorf("failed to create directory for sounding SP SAC files: %v", err)
	}

	// PDF SP Directories
	pdfTechDir, err := fun.FindValidDirectory([]string{
		"web/file/sp_technician",
		"../web/file/sp_technician",
		"../../web/file/sp_technician",
		"../../../web/file/sp_technician",
	})
	if err != nil {
		return fmt.Errorf("failed to find valid directory for SP technician files: %v", err)
	}
	pdfDirForSPTechnician := filepath.Join(pdfTechDir, now.Format("2006-01-02"))
	if err := os.MkdirAll(pdfDirForSPTechnician, 0755); err != nil {
		return fmt.Errorf("failed to create directory for SP technician files: %v", err)
	}

	pdfSPLDir, err := fun.FindValidDirectory([]string{
		"web/file/sp_spl",
		"../web/file/sp_spl",
		"../../web/file/sp_spl",
		"../../../web/file/sp_spl",
	})
	if err != nil {
		return fmt.Errorf("failed to find valid directory for SP spl files: %v", err)
	}
	pdfDirForSPSPL := filepath.Join(pdfSPLDir, now.Format("2006-01-02"))
	if err := os.MkdirAll(pdfDirForSPSPL, 0755); err != nil {
		return fmt.Errorf("failed to create directory for SP spl files: %v", err)
	}

	pdfSACDir, err := fun.FindValidDirectory([]string{
		"web/file/sp_sac",
		"../web/file/sp_sac",
		"../../web/file/sp_sac",
		"../../../web/file/sp_sac",
	})
	if err != nil {
		return fmt.Errorf("failed to find valid directory for SP sac files: %v", err)
	}
	pdfDirForSPSAC := filepath.Join(pdfSACDir, now.Format("2006-01-02"))
	if err := os.MkdirAll(pdfDirForSPSAC, 0755); err != nil {
		return fmt.Errorf("failed to create directory for SP sac files: %v", err)
	}

	excelReportDir, err := fun.FindValidDirectory([]string{
		"web/file/excel_report",
		"../web/file/excel_report",
		"../../web/file/excel_report",
		"../../../web/file/excel_report",
	})
	if err != nil {
		return fmt.Errorf("failed to find valid directory for SP excel report files: %v", err)
	}
	excelReportDirUsed := filepath.Join(excelReportDir, now.Format("2006-01-02"))
	if err := os.MkdirAll(excelReportDirUsed, 0755); err != nil {
		return fmt.Errorf("failed to create directory for SP excel report files: %v", err)
	}

	// model used to send through email
	type EmailRecipient struct {
		Name  string
		Email string
	}
	sendUsing := "telegram" // whatsapp | email | telegram
	// Changed from whatsapp to telegram for SP delivery
	forProject := "ODOO MS" // Mark all technicians is on ODOO Manage Service
	minJOVisited := config.GetConfig().SPTechnician.MinimumJOVisited
	if minJOVisited <= 0 {
		minJOVisited = 1
	}
	maxResponseSPAtHour := config.GetConfig().SPTechnician.MaxResponseSPAtHour
	if maxResponseSPAtHour <= 0 {
		maxResponseSPAtHour = 19
	}

	dataHRD := config.GetConfig().Default.PTHRD
	if len(dataHRD) == 0 {
		return errors.New("no data found for HRD")
	}
	hrdPersonaliaName := dataHRD[0].Name
	hrdTTDPath := dataHRD[0].TTDPath
	hrdPhoneNumber := dataHRD[0].PhoneNumber

	if hrdTTDPath == "" || hrdPersonaliaName == "" || hrdPhoneNumber == "" {
		return errors.New("data for HRD is incomplete")
	}

	ODOOMSSAC := config.GetConfig().ODOOMSSAC
	if len(ODOOMSSAC) == 0 {
		return errors.New("no data found for SAC ODOO Manage Service")
	}

	needToSendTheSPTechnicianThroughWhatsapp := make(map[string]int) // e.g. needToSendTheSPTechnicianThroughWhatsapp["1.1 Jakbar ini itu"] = 1 means technician 1.1 need notif of his sp - 1 to his spl/sac
	needToSendTheSPSPLThroughWhatsapp := make(map[string]int)        // e.g. needToSendTheSPSPLThroughWhatsapp["1.1 SPL Tangerang Sniki Damaskus"] = 1 soon this sp will be sent to SAC
	needToSendTheSPSACThroughWhatsapp := make(map[string]int)        // e.g. needToSendTheSPSACThroughWhatsapp["Osvaldo"] = 1 soon this sp will be send directly to the SAC

	// (1) Check SP Status - technician not visits
	resignTechnicianReplacer := "Resigned"
	if result.RowsAffected > 0 {
		startTimeProcessCheckSPNotLogin := time.Now()
		logrus.Infof("[%s] start processing check SP not login & visit for %d technicians @%v", taskDoing, len(dataJOPlanned), startTimeProcessCheckSPNotLogin)
		for _, record := range dataJOPlanned {
			if record.Technician == "" {
				continue
			}
			isSkipped := false
			for _, exclude := range excludedTechnicians {
				if strings.Contains(strings.TrimSpace(strings.ToLower(record.Technician)), strings.TrimSpace(exclude)) {
					isSkipped = true
					break
				}
			}

			if isSkipped {
				logrus.Warnf("technician %s is excluded from SP checking", record.Technician)
				continue
			}

			var totalJOPlanned, totalJOVisited int
			if len(record.WONumber) > 0 {
				var woNumbers []string
				if err := json.Unmarshal(record.WONumber, &woNumbers); err == nil {
					totalJOPlanned = len(woNumbers)
				} else {
					logrus.Errorf("Failed to unmarshal WONumber for technician %s: %v", record.Technician, err)
				}
			}

			if len(record.WONumberVisited) > 0 {
				var woVisited []string
				if err := json.Unmarshal(record.WONumberVisited, &woVisited); err == nil {
					totalJOVisited = len(woVisited)
				} else {
					logrus.Errorf("Failed to unmarshal WOVisited for technician %s: %v", record.Technician, err)
				}
			}

			if totalJOPlanned == 0 {
				logrus.Warnf("technician %s has 0 JO planned today, skipping SP check", record.Technician)
				continue
			}

			todayStart := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, loc)
			lastLogin := record.TechnicianLastLogin
			lastDownload := record.TechnicianLastDownloadJO

			if record.SPL == "" {
				logrus.Warnf("technician %s has no SPL assigned, skipping SP check", record.Technician)
				continue
			}

			splData, exists := TechODOOMSData[record.SPL]
			if !exists {
				logrus.Warnf("no data for spl %s from tech %s", record.SPL, record.Technician)
				continue
			}

			var namaTeknisi, namaSPL, sanitizedNoHPSPL, splCity string
			if record.Name != "" {
				namaTeknisi = record.Name
			} else {
				namaTeknisi = record.Technician
			}
			if splData.Name != "" {
				namaSPL = splData.Name
			} else {
				namaSPL = splData.SPL
			}
			if splData.NoHP != "" {
				sanitizedNoHPSPL, err = fun.SanitizePhoneNumber(splData.NoHP)
				if err != nil {
					logrus.Warnf("failed to sanitize phone number for SPL %s: %v", namaSPL, err)
					continue
				}
			}

			SACDataTechnician, ok := ODOOMSSAC[record.SAC]
			if !ok {
				logrus.Errorf("no SAC data found for technician: %s", record.Technician)
				continue
			}
			var namaSAC string
			if SACDataTechnician.FullName != "" {
				namaSAC = SACDataTechnician.FullName
			} else {
				namaSAC = record.SAC
			}

			splCity = getSPLCity(record.SPL)
			if splCity == "" {
				splCity = "Unknown"
			}

			// Reset SP Status before do checking, coz might the SP back set to the SP-1
			if err := ResetTechnicianSP(record.Technician, forProject); err != nil {
				if !errors.Is(err, gorm.ErrRecordNotFound) {
					logrus.Warnf("Failed to reset SP for technician %s: %v", record.Technician, err)
				}
			}
			if err := ResetSPLSP(record.SPL, forProject); err != nil {
				if !errors.Is(err, gorm.ErrRecordNotFound) {
					logrus.Warnf("Failed to reset SP for SPL %s: %v", record.SPL, err)
				}
			}
			if err := ResetSACSP(record.SAC, forProject); err != nil {
				if !errors.Is(err, gorm.ErrRecordNotFound) {
					logrus.Warnf("Failed to reset SP for SAC %s: %v", record.SAC, err)
				}
			}

			if sanitizedNoHPSPL == "" {
				logrus.Warnf("SPL %s has no valid phone number, skipping SP check", namaSPL)
				continue
			}

			if namaTeknisi == "" || namaSPL == "" {
				continue // Skip if technician or SPL name is empty
			}

			if namaTeknisi == namaSPL {
				logrus.Warnf("technician %s is the same as SPL %s, skipping SP check. It will be handle soon", namaTeknisi, namaSPL)
				continue
			}

			namaTeknisi = fun.CapitalizeWord(namaTeknisi)
			namaSPL = fun.CapitalizeWord(namaSPL)

			var technicianIsLoginToday bool = false
			var pelanggaranID string

			switch {
			case lastLogin == nil && lastDownload == nil:
				technicianIsLoginToday = false
				pelanggaranID = "tidak pernah login ke aplikasi FS & tidak pernah mengunduh JO."
			case lastLogin == nil && lastDownload != nil:
				if lastDownload.Before(todayStart) {
					technicianIsLoginToday = false
					pelanggaranID = fmt.Sprintf("tidak pernah login ke aplikasi FS. Terakhir mengunduh JO pada: %s.", lastDownload.Format("2006-01-02 15:04:05"))
				} else {
					technicianIsLoginToday = true
				}
			case lastLogin != nil:
				if lastLogin.Before(todayStart) {
					if lastDownload != nil && lastDownload.After(todayStart) {
						technicianIsLoginToday = true
					} else {
						technicianIsLoginToday = false
						pelanggaranID = fmt.Sprintf("tidak login pada %s. Terakhir login: %s.", tanggalIndoFormatted, lastLogin.Format("2006-01-02 15:04"))
					}
				} else {
					technicianIsLoginToday = true
				}
			} // .end of switch case condition to check if technician is login today or not

			// Check last visit of technician
			if record.TechnicianLastVisit == nil {
				var tglTidakKerjaTeknisiFormatted string
				tglTidakKerja, err := tanggal.Papar(now, "Jakarta", tanggal.WIB)
				if err != nil {
					logrus.Errorf("failed to format date of tgl tidak kerja teknisi %s : %v", record.Technician, err)
					tglTidakKerjaTeknisiFormatted = now.Format("Monday, 02 January 2006")
				}
				tglTidakKerjaTeknisiFormatted = tglTidakKerja.Format(" ", []tanggal.Format{
					tanggal.NamaHariDenganKoma,
					tanggal.Hari,
					tanggal.NamaBulan,
					tanggal.Tahun,
				})

				pelanggaranID += fmt.Sprintf(" Juga tidak melakukan kunjungan kerja pada %s.", tglTidakKerjaTeknisiFormatted)
			}

			pelanggaranID = fun.CapitalizeFirstWord(pelanggaranID)

			// Check if technician already login today and its visited jo is more than minimum
			if technicianIsLoginToday {
				if totalJOVisited >= minJOVisited {
					logrus.Infof("Technician %s has visited %d JO(s) from %d JO(s) planned, which meets or exceeds the minimum requirement of %d JO(s). No SP needed.",
						record.Technician,
						totalJOVisited,
						totalJOPlanned,
						minJOVisited,
					)
					continue
				}
			}

			speech := htgotts.Speech{Folder: audioDirForSPTechnician, Language: voices.Indonesian, Handler: &handlers.Native{}}

			spIsProcessedToday := false
			var dataSPTechnician sptechnicianmodel.TechnicianGotSP
			result := dbWeb.Where("technician = ? AND for_project = ?", record.Technician, forProject).First(&dataSPTechnician)
			if result.Error != nil {
				if errors.Is(result.Error, gorm.ErrRecordNotFound) {
					// No data SP found for technician %s , proceed to set SP 1
					if !technicianIsLoginToday {
						// Create sound sp 1 for technician
						SP1TechnicianTextPart1 := "Berikut kami sampaikan bahwa "
						SP1TechnicianTextPart2 := fmt.Sprintf(" saudara %s.", namaTeknisi)
						SP1TechnicianTextPart3 := "Menerima Surat Peringatan (SP-1)."
						SP1TechnicianTextPart4 := "Mohon menjadi perhatian dan segera melakukan perbaikan yang diperlukan."
						SP1TechnicianTextPart5 := "terima kasih..."
						sp1TechFilenameSound := fmt.Sprintf("%s_SP1", strings.ReplaceAll(record.Technician, "*", resignTechnicianReplacer))
						fileTTSSP1Technician, err := fun.CreateRobustTTS(speech, audioDirForSPTechnician, []string{
							SP1TechnicianTextPart1,
							SP1TechnicianTextPart2,
							SP1TechnicianTextPart3,
							SP1TechnicianTextPart4,
							SP1TechnicianTextPart5,
						}, sp1TechFilenameSound)
						if err != nil {
							logrus.Errorf("failed to create merged SP1 TTS file for technician %s : %v", record.Technician, err)
							continue
						}

						if fileTTSSP1Technician != "" {
							fileInfo, statErr := os.Stat(fileTTSSP1Technician)
							if statErr == nil {
								logrus.Debugf("🔊 SP-1 merged TTS for %s - %s, Size: %d bytes", record.Technician, fileTTSSP1Technician, fileInfo.Size())
							} else {
								logrus.Errorf("🔇 SP-1 TTS for %s got stat error : %v", record.Technician, statErr)
							}
						}

						// Set SP - 1
						noSuratSP1, err := IncrementNomorSuratSP(dbWeb, "LAST_NOMOR_SURAT_SP1_GENERATED")
						if err != nil {
							logrus.Errorf("Failed to increment nomor surat SP-1 for technician %s : %v", record.Technician, err)
							continue
						}
						var nomorSuratSP1Str string
						if noSuratSP1 < 1000 {
							nomorSuratSP1Str = fmt.Sprintf("%03d", noSuratSP1)
						} else {
							nomorSuratSP1Str = fmt.Sprintf("%d", noSuratSP1)
						}

						// Placeholder for replace data in pdf SP - 1 Technician
						placeholderSP1Teknisi := map[string]string{
							"$nomor_surat":            nomorSuratSP1Str,
							"$bulan_romawi":           monthRoman,
							"$tahun_sp":               tahunSP,
							"$nama_spl":               namaSPL,
							"$jabatan_spl":            fmt.Sprintf("Service Point Leader %s", splCity),
							"$pelanggaran_karyawan":   pelanggaranID,
							"$nama_teknisi":           namaTeknisi,
							"$tanggal_sp_diterbitkan": tanggalIndoFormatted,
							"$personalia_name":        hrdPersonaliaName,
							"$personalia_ttd":         hrdTTDPath,
							"$personalia_phone":       hrdPhoneNumber,
							"$sac_name":               SACDataTechnician.FullName,
							"$sac_ttd":                SACDataTechnician.TTDPath,
						}
						pdfSP1FilenameTechnician := fmt.Sprintf("SP_1_%s_%s.pdf", strings.ReplaceAll(record.Technician, "*", resignTechnicianReplacer), now.Format("2006-01-02"))
						pdfSP1TechnicianFilePath := filepath.Join(pdfDirForSPTechnician, pdfSP1FilenameTechnician)

						if err := CreatePDFSP1ForTechnician(placeholderSP1Teknisi, pdfSP1TechnicianFilePath); err != nil {
							logrus.Errorf("failed to create the pdf for sp - 1 technician %s : %v", record.Technician, err)
							continue
						}

						spIsProcessedToday = true // Set that technician already got the SP today, so the SP cannot be continue
						technicianGotSP1At := time.Now()
						dataSP1Technician := sptechnicianmodel.TechnicianGotSP{
							Technician:      record.Technician,
							Name:            namaTeknisi,
							ForProject:      forProject,
							IsGotSP1:        true,
							GotSP1At:        &technicianGotSP1At,
							NoSP1:           noSuratSP1,
							PelanggaranSP1:  pelanggaranID,
							SP1SoundTTSPath: fileTTSSP1Technician,
							SP1FilePath:     pdfSP1TechnicianFilePath,
						}

						if err := dbWeb.Create(&dataSP1Technician).Error; err != nil {
							logrus.Errorf("failed to create the SP - 1 for technician %s : %v", record.Technician, err)
							continue
						}

						if _, exists := needToSendTheSPTechnicianThroughWhatsapp[record.Technician]; !exists {
							needToSendTheSPTechnicianThroughWhatsapp[record.Technician] = 1
						}
						logrus.Infof("SP - 1 of Technician %s successfully created", record.Technician)
					} // .end of set SP - 1 for technician if not had before
				} else {
					logrus.Errorf("Failed to fecth data SP Technician %s : %v", record.Technician, err)
					continue
				}
			} // .end of check technician had data sp 1 or not. If not create his SP - 1

			// SP status is not being processing yet, means technician can get the SP - 2 or SP -3
			if !spIsProcessedToday {
				// Check data SP technician before starting
				technicianGotSP1 := dataSPTechnician.IsGotSP1
				technicianGotSP2 := dataSPTechnician.IsGotSP2
				technicianGotSP3 := dataSPTechnician.IsGotSP3

				// Check if technician will got the SP - 2
				if technicianGotSP1 && !technicianGotSP2 && !technicianGotSP3 {
					// 1) First get the sp-1 sent at time
					sp1SentAt := dataSPTechnician.GotSP1At
					if sp1SentAt == nil {
						// Try to find the sp - 1 sent at using the first whatsapp message sent
						var firstSP1TechnicianMsg sptechnicianmodel.SPWhatsAppMessage
						if err := dbWeb.Where("technician_got_sp_id = ? AND number_of_sp = ?", dataSPTechnician.ID, 1).
							Order("whatsapp_sent_at asc").
							First(&firstSP1TechnicianMsg).
							Error; err != nil {
							logrus.Errorf("could not find the first sp-1 msg of technician %s to determine sent time : %v", record.Technician, err)
							continue
						}
						if firstSP1TechnicianMsg.WhatsappSentAt == nil {
							logrus.Errorf("SP-1 whatsapp msg sent time is nil for technician %s, cannot continue to create the sp-2 for technician", record.Technician)
							continue
						}
						// Assign the whatsapp sent at time to sp1SentAt
						sp1SentAt = firstSP1TechnicianMsg.WhatsappSentAt
					}
					// 2) Calculate the sp - 1 reply deadline
					deadlineSP1 := time.Date(sp1SentAt.Year(), sp1SentAt.Month(), sp1SentAt.Day(), maxResponseSPAtHour, 0, 0, 0, sp1SentAt.Location())
					// 3) Count replies received before or equal to deadline
					var onTimeSP1RepliedCount int64
					dbWeb.Model(&sptechnicianmodel.SPWhatsAppMessage{}).
						Where("technician_got_sp_id = ?", dataSPTechnician.ID).
						Where("number_of_sp = ?", 1).
						Where("for_project = ?", forProject).
						Where("whatsapp_reply_text IS NOT NULL AND whatsapp_reply_text != ''").
						Where("whatsapp_replied_at <= ?", deadlineSP1).
						Count(&onTimeSP1RepliedCount)
					// 4) Proceed only if no on-time replies were found
					if onTimeSP1RepliedCount == 0 {
						if !technicianIsLoginToday {
							var tglSP1TeknisiTerkirimFormatted string
							tglSP1TeknisiTerkirim, err := tanggal.Papar(*sp1SentAt, "Jakarta", tanggal.WIB)
							if err != nil {
								logrus.Errorf("failed to format date of tgl sp - 1 terkirim of technician %s : %v", record.Technician, err)
								continue
							}
							tglSP1TeknisiTerkirimFormatted = tglSP1TeknisiTerkirim.Format(" ", []tanggal.Format{
								tanggal.NamaHariDenganKoma,
								tanggal.Hari,
								tanggal.NamaBulan,
								tanggal.Tahun,
								tanggal.PukulDenganDetik,
								tanggal.ZonaWaktu,
							})

							// 5) Build text to sound for sp - 2 technician
							SP2TechnicianTextPart1 := "Sehubungan dengan Surat Peringatan (SP-1) yang sebelumnya disampaikan"
							SP2TechnicianTextPart2 := fmt.Sprintf(" kepada saudara %s pada %s", namaTeknisi, tglSP1TeknisiTerkirimFormatted)
							SP2TechnicianTextPart3 := "perusahaan kemudian memutuskan untuk menindaklanjuti melalui Surat Peringatan (SP-2)."
							SP2TechnicianTextPart4 := "Hal ini didasari oleh belum adanya perbaikan"
							SP2TechnicianTextPart5 := " atau tindakan korektif yang memadai terkait pelanggaran yang telah dilaporkan."
							SP2TechnicianTextPart6 := "Mohon menjadi perhatian dan segera melakukan perbaikan yang diperlukan, terima kasih..."
							sp2TechnicianFilenameSound := fmt.Sprintf("%s_SP2", strings.ReplaceAll(record.Technician, "*", resignTechnicianReplacer))
							fileTTSSP2Technician, err := fun.CreateRobustTTS(speech, audioDirForSPTechnician, []string{
								SP2TechnicianTextPart1,
								SP2TechnicianTextPart2,
								SP2TechnicianTextPart3,
								SP2TechnicianTextPart4,
								SP2TechnicianTextPart5,
								SP2TechnicianTextPart6,
							}, sp2TechnicianFilenameSound)
							if err != nil {
								logrus.Errorf("Failed to create merged SP-2 TTS file for technician %s : %v", record.Technician, err)
								continue
							}

							if fileTTSSP2Technician != "" {
								fileInfo, statErr := os.Stat(fileTTSSP2Technician)
								if statErr == nil {
									logrus.Debugf("🔊 SP-2 merged TTS for %s - %s, Size: %d bytes", record.Technician, fileTTSSP2Technician, fileInfo.Size())
								} else {
									logrus.Errorf("🔇 SP-2 TTS for %s got stat error : %v", record.Technician, statErr)
								}
							}

							// 6) Set SP - 2
							noSuratSP2, err := IncrementNomorSuratSP(dbWeb, "LAST_NOMOR_SURAT_SP2_GENERATED")
							if err != nil {
								logrus.Errorf("Failed to increment nomor surat SP-2 for technician %s: %v", record.Technician, err)
								continue
							}
							var nomorSuratSP2Str string
							if noSuratSP2 < 1000 {
								nomorSuratSP2Str = fmt.Sprintf("%03d", noSuratSP2)
							} else {
								nomorSuratSP2Str = fmt.Sprintf("%d", noSuratSP2)
							}

							// SP - 2 placeholder for pdf replacements
							placeholderSP2Teknisi := map[string]string{
								"$nomor_surat":            nomorSuratSP2Str,
								"$bulan_romawi":           monthRoman,
								"$tahun_sp":               tahunSP,
								"$nama_spl":               namaSPL,
								"$jabatan_spl":            fmt.Sprintf("Service Point Leader %s", splCity),
								"$pelanggaran_karyawan":   pelanggaranID,
								"$nama_teknisi":           namaTeknisi,
								"$tanggal_sp_diterbitkan": tanggalIndoFormatted,
								"$personalia_name":        hrdPersonaliaName,
								"$personalia_ttd":         hrdTTDPath,
								"$personalia_phone":       hrdPhoneNumber,
								"$sac_name":               SACDataTechnician.FullName,
								"$sac_ttd":                SACDataTechnician.TTDPath,
								"$record_technician":      record.Technician,
								"$for_project":            forProject,
							}
							pdfSP2FilenameTechnician := fmt.Sprintf("SP_2_%s_%s.pdf", strings.ReplaceAll(record.Technician, "*", resignTechnicianReplacer), now.Format("2006-01-02"))
							pdfSP2TechnicianFilePath := filepath.Join(pdfDirForSPTechnician, pdfSP2FilenameTechnician)

							if err := CreatePDFSP2ForTechnician(placeholderSP2Teknisi, pdfSP2TechnicianFilePath); err != nil {
								logrus.Errorf("failed to create the pdf for sp - 2 technician %s : %v", record.Technician, err)
								continue
							}

							technicianGotSP2At := time.Now()
							dataSP2Technician := sptechnicianmodel.TechnicianGotSP{
								IsGotSP2:        true,
								GotSP2At:        &technicianGotSP2At,
								NoSP2:           noSuratSP2,
								PelanggaranSP2:  pelanggaranID,
								SP2SoundTTSPath: fileTTSSP2Technician,
								SP2FilePath:     pdfSP2TechnicianFilePath,
							}

							if err := dbWeb.Where("for_project = ? AND technician = ? AND is_got_sp1 = ?", forProject, record.Technician, true).
								Updates(&dataSP2Technician).Error; err != nil {
								logrus.Errorf("failed to update the SP - 2 for technician %s : %v", record.Technician, err)
								continue
							}

							if _, exists := needToSendTheSPTechnicianThroughWhatsapp[record.Technician]; !exists {
								needToSendTheSPTechnicianThroughWhatsapp[record.Technician] = 2
							}

							logrus.Infof("SP - 2 of Technician %s successfully created", record.Technician)
						} // .end of technician not login today so will got the sp - 2
					} // .end of SP - 1 for technician got no reply on time

				} // .end of check if technician will got the sp - 2

				// Check if technician already got SP - 1 & SP - 2 so will get the SP - 3
				if technicianGotSP1 && technicianGotSP2 && !technicianGotSP3 {
					// 1) First get the sp-2 sent at time
					sp2SentAt := dataSPTechnician.GotSP2At
					if sp2SentAt == nil {
						// Try to find the sp - 2 sent at using the first whatsapp message sent
						var firstSP2TechnicianMsg sptechnicianmodel.SPWhatsAppMessage
						if err := dbWeb.Where("technician_got_sp_id = ? AND number_of_sp = ?", dataSPTechnician.ID, 2).
							Order("whatsapp_sent_at asc").
							First(&firstSP2TechnicianMsg).
							Error; err != nil {
							logrus.Errorf("could not find the first sp-2 msg of technician %s to determine sent time : %v", record.Technician, err)
							continue
						}
						if firstSP2TechnicianMsg.WhatsappSentAt == nil {
							logrus.Errorf("SP-2 whatsapp msg sent time is nil for technician %s, cannot continue to create the sp-3 for technician", record.Technician)
							continue
						}
						// Assign the whatsapp sent at time to sp2SentAt
						sp2SentAt = firstSP2TechnicianMsg.WhatsappSentAt
					}
					// 2) Calculate the sp - 2 reply deadline
					deadlineSP2 := time.Date(sp2SentAt.Year(), sp2SentAt.Month(), sp2SentAt.Day(), maxResponseSPAtHour, 0, 0, 0, sp2SentAt.Location())
					// 3) Count replies received before or equal to deadline
					var onTimeSP2RepliedCount int64
					dbWeb.Model(&sptechnicianmodel.SPWhatsAppMessage{}).
						Where("technician_got_sp_id = ?", dataSPTechnician.ID).
						Where("number_of_sp = ?", 2).
						Where("for_project = ?", forProject).
						Where("whatsapp_reply_text IS NOT NULL AND whatsapp_reply_text != ''").
						Where("whatsapp_replied_at <= ?", deadlineSP2).
						Count(&onTimeSP2RepliedCount)
					// 4) Proceed only if no on-time replies were found
					if onTimeSP2RepliedCount == 0 {
						if !technicianIsLoginToday {
							var tglSP2TeknisiTerkirimFormatted string
							tglSP2TeknisiTerkirim, err := tanggal.Papar(*sp2SentAt, "Jakarta", tanggal.WIB)
							if err != nil {
								logrus.Errorf("failed to format date of tgl sp - 2 terkirim of technician %s : %v", record.Technician, err)
								continue
							}
							tglSP2TeknisiTerkirimFormatted = tglSP2TeknisiTerkirim.Format(" ", []tanggal.Format{
								tanggal.NamaHariDenganKoma,
								tanggal.Hari,
								tanggal.NamaBulan,
								tanggal.Tahun,
								tanggal.PukulDenganDetik,
								tanggal.ZonaWaktu,
							})

							// 5) Build text to sound for sp - 3
							SP3TechnicianTextPart1 := "Merujuk pada Surat Peringatan (SP-2)"
							SP3TechnicianTextPart2 := fmt.Sprintf(" yang telah disampaikan kepada Saudara %s", namaTeknisi)
							SP3TechnicianTextPart3 := fmt.Sprintf("pada tanggal %s,", tglSP2TeknisiTerkirimFormatted)
							SP3TechnicianTextPart4 := "perusahaan menilai bahwa pelanggaran yang Saudara lakukan tidak kunjung diperbaiki."
							SP3TechnicianTextPart5 := "Kesempatan yang telah diberikan sebelumnya tidak dimanfaatkan dengan baik."
							SP3TechnicianTextPart6 := "Oleh karena itu, dengan berat hati,"
							SP3TechnicianTextPart7 := "perusahaan mengambil langkah tegas."
							SP3TechnicianTextPart8 := "Dengan ini diterbitkan Surat Peringatan (SP-3) sebagai peringatan terakhir."
							SP3TechnicianTextPart9 := "Surat ini juga menyatakan berakhirnya hubungan kerja antara perusahaan dan Saudara."
							SP3TechnicianTextPart10 := "Keputusan berlaku sejak tanggal surat ini diterbitkan,"
							SP3TechnicianTextPart11 := fmt.Sprintf("yakni pada %s.", tanggalIndoFormatted)
							SP3TechnicianTextPart12 := "Hal ini sesuai dengan ketentuan perusahaan."
							SP3TechnicianTextPart13 := "Atas perhatiannya, kami ucapkan terima kasih..."
							sp3TechnicianFilenameSound := fmt.Sprintf("%s_SP3", strings.ReplaceAll(record.Technician, "*", resignTechnicianReplacer))
							fileTTSSP3Technician, err := fun.CreateRobustTTS(speech, audioDirForSPTechnician, []string{
								SP3TechnicianTextPart1,
								SP3TechnicianTextPart2,
								SP3TechnicianTextPart3,
								SP3TechnicianTextPart4,
								SP3TechnicianTextPart5,
								SP3TechnicianTextPart6,
								SP3TechnicianTextPart7,
								SP3TechnicianTextPart8,
								SP3TechnicianTextPart9,
								SP3TechnicianTextPart10,
								SP3TechnicianTextPart11,
								SP3TechnicianTextPart12,
								SP3TechnicianTextPart13,
							}, sp3TechnicianFilenameSound)
							if err != nil {
								logrus.Errorf("Failed to create merged SP-3 TTS file for technician %s : %v", record.Technician, err)
								continue
							}

							if fileTTSSP3Technician != "" {
								fileInfo, statErr := os.Stat(fileTTSSP3Technician)
								if statErr == nil {
									logrus.Debugf("🔊 SP-3 merged TTS for %s - %s, Size: %d bytes", record.Technician, fileTTSSP3Technician, fileInfo.Size())
								} else {
									logrus.Errorf("🔇 SP-3 TTS for %s got stat error : %v", record.Technician, statErr)
								}
							}

							// 6) Set SP - 3
							noSuratSP3, err := IncrementNomorSuratSP(dbWeb, "LAST_NOMOR_SURAT_SP3_GENERATED")
							if err != nil {
								logrus.Errorf("Failed to increment nomor surat SP-3 for technician %s: %v", record.Technician, err)
								continue
							}
							var nomorSuratSP3Str string
							if noSuratSP3 < 1000 {
								nomorSuratSP3Str = fmt.Sprintf("%03d", noSuratSP3)
							} else {
								nomorSuratSP3Str = fmt.Sprintf("%d", noSuratSP3)
							}

							// Make placeholder for replacements in SP - 3 pdf
							placeholdersSP3Teknisi := map[string]string{
								"$nomor_surat":            nomorSuratSP3Str,
								"$bulan_romawi":           monthRoman,
								"$tahun_sp":               tahunSP,
								"$nama_spl":               namaSPL,
								"$jabatan_spl":            fmt.Sprintf("Service Point Leader %s", splCity),
								"$pelanggaran_karyawan":   pelanggaranID,
								"$nama_teknisi":           namaTeknisi,
								"$tanggal_sp_diterbitkan": tanggalIndoFormatted,
								"$personalia_name":        hrdPersonaliaName,
								"$personalia_ttd":         hrdTTDPath,
								"$personalia_phone":       hrdPhoneNumber,
								"$sac_name":               SACDataTechnician.FullName,
								"$sac_ttd":                SACDataTechnician.TTDPath,
								"$record_technician":      record.Technician,
								"$for_project":            forProject,
							}
							pdfSP3FilenameTechnician := fmt.Sprintf("SP_3_%s_%s.pdf", strings.ReplaceAll(record.Technician, "*", resignTechnicianReplacer), now.Format("2006-01-02"))
							pdfSP3TechnicianFilePath := filepath.Join(pdfDirForSPTechnician, pdfSP3FilenameTechnician)
							if err := CreatePDFSP3ForTechnician(placeholdersSP3Teknisi, pdfSP3TechnicianFilePath); err != nil {
								logrus.Errorf("failed to create the pdf for sp - 3 technician %s : %v", record.Technician, err)
								continue
							}

							technicianGotSP3At := time.Now()
							dataSP3Technician := sptechnicianmodel.TechnicianGotSP{
								IsGotSP3:        true,
								GotSP3At:        &technicianGotSP3At,
								NoSP3:           noSuratSP3,
								PelanggaranSP3:  pelanggaranID,
								SP3SoundTTSPath: fileTTSSP3Technician,
								SP3FilePath:     pdfSP3TechnicianFilePath,
							}

							if err := dbWeb.Where("for_project = ? AND technician = ? AND is_got_sp2 = ?", forProject, record.Technician, true).
								Updates(&dataSP3Technician).Error; err != nil {
								logrus.Errorf("failed to update the SP - 3 for technician %s : %v", record.Technician, err)
								continue
							}

							if _, exists := needToSendTheSPTechnicianThroughWhatsapp[record.Technician]; !exists {
								needToSendTheSPTechnicianThroughWhatsapp[record.Technician] = 3
							}

							logrus.Infof("SP - 3 of Technician %s successfully created", record.Technician)
						} // .end of technician not login today so will got the sp - 3
					} // .end of got no on time reply for sp - 2
				} // .end of check if technician will got the sp - 3

				// ############################################################################################################
				// 						 Run SPL & SAC processing in PARALLEL when technician got SP-3
				// ############################################################################################################
				var latestDataSP sptechnicianmodel.TechnicianGotSP
				result := dbWeb.Where("for_project = ? AND technician = ?", forProject, record.Technician).First(&latestDataSP)
				if result.Error == nil {
					technicianGotSP1 = latestDataSP.IsGotSP1
					technicianGotSP2 = latestDataSP.IsGotSP2
					technicianGotSP3 = latestDataSP.IsGotSP3
				}

				if technicianGotSP1 && technicianGotSP2 && technicianGotSP3 {
					var wg sync.WaitGroup
					wg.Add(2) // We'll process SPL and SAC in parallel

					// (1) Process SPL in goroutine
					go func() {
						defer wg.Done()
						processSPForSPL(
							dbWeb, record, forProject, namaTeknisi, namaSPL, splCity,
							tanggalIndoFormatted, monthRoman, tahunSP, hrdPersonaliaName, hrdTTDPath, hrdPhoneNumber,
							SACDataTechnician, audioDirForSPSPL, pdfDirForSPSPL, maxResponseSPAtHour,
							now, resignTechnicianReplacer, needToSendTheSPSPLThroughWhatsapp,
						)
					}()

					// (2) Process SAC in goroutine (parallel with SPL)
					go func() {
						defer wg.Done()
						processSPForSAC(
							dbWeb, record, forProject, namaTeknisi, namaSAC,
							tanggalIndoFormatted, monthRoman, tahunSP, hrdPersonaliaName, hrdTTDPath, hrdPhoneNumber,
							SACDataTechnician, audioDirForSPSAC, pdfDirForSPSAC, maxResponseSPAtHour,
							now, resignTechnicianReplacer, needToSendTheSPSACThroughWhatsapp,
						)
					}()

					// Wait for both SPL and SAC processing to complete
					wg.Wait()
					logrus.Debugf("Completed parallel SP processing for technician %s (SPL: %s, SAC: %s)",
						record.Technician, record.SPL, record.SAC)
				} // .end of check technician already got sp 3
				// ############################################################################################################

			} // .end of checking status of SP technician that sp is not checking yet

		} // .end of looping data jo planned today to check sp status

		totalDurationProcessCheckSPNotLogin := time.Since(startTimeProcessCheckSPNotLogin)
		logrus.Infof("[%s] total duration process check SP not login technicians: %v", taskDoing, totalDurationProcessCheckSPNotLogin)
	} // .end of jo plan for technicians today is exists
	// .end of (1)

	// (2) Check SP Status - technician not doing SO or got miss data of doing SO
	startTimeProcessCheckSPStockOpname := time.Now()
	logrus.Infof("[%s] start process check SP Stock Opname technicians @%v", taskDoing, startTimeProcessCheckSPStockOpname)
	excelSO, errProcessSPStockOpname := processSPofStockOpnameTechnician(
		dbWeb,
		forProject,
		hrdPersonaliaName,
		hrdTTDPath,
		hrdPhoneNumber,
		excludedTechnicians,
		needToSendTheSPTechnicianThroughWhatsapp,
		needToSendTheSPSPLThroughWhatsapp,
		resignTechnicianReplacer,
		audioDirForSPTechnician,
		pdfDirForSPTechnician,
		audioDirForSPSPL,
		pdfDirForSPSPL,
		excelReportDirUsed,
	)
	if errProcessSPStockOpname != nil {
		logrus.Errorf("got error while trying process the SP from Stock Opname: %v", errProcessSPStockOpname)
	}
	totalDurationProcessCheckSPStockOpname := time.Since(startTimeProcessCheckSPStockOpname)
	logrus.Infof("[%s] total duration process check SP Stock Opname: %v", taskDoing, totalDurationProcessCheckSPStockOpname)

	// Greeting logic (ensuring correct 24-hour format)
	var greetingID string
	hour := time.Now().Hour()
	if hour >= 0 && hour < 4 {
		greetingID = "Selamat Dini Hari" // 00:00 - 03:59
	} else if hour >= 4 && hour < 12 {
		greetingID = "Selamat Pagi" // 04:00 - 11:59
	} else if hour >= 12 && hour < 15 {
		greetingID = "Selamat Siang" // 12:00 - 14:59
	} else if hour >= 15 && hour < 17 {
		greetingID = "Selamat Sore" // 15:00 - 16:59
	} else if hour >= 17 && hour < 19 {
		greetingID = "Selamat Petang" // 17:00 - 18:59
	} else {
		greetingID = "Selamat Malam" // 19:00 - 23:59
	}

	// Stock Opname report sending via Telegram
	// TODO: Implement Excel SO report sending via Telegram
	if excelSO == "" {
		logrus.Info("no stock opname report generated, so no need to send the report")
	} else {
		var sb strings.Builder
		sb.WriteString(fmt.Sprintf("%s,\n\n", greetingID))
		sb.WriteString("Berikut terlampir laporan Surat Peringatan (SP) untuk teknisi yang tidak melakukan Stock Opname atau terdapat data yang tidak sesuai pada pelaksanaan Stock Opname.\n")
		sb.WriteString("Mohon untuk dapat ditindaklanjuti.\n\n")
		sb.WriteString("Terima kasih.\n")
		msgID := sb.String()
		formattedMsgID := strings.ReplaceAll(msgID, "\n", "<br>")
		if sendUsing == "email" {
			sendToList := config.GetConfig().StockOpname.SOReportSendToEmail
			if len(sendToList) == 0 {
				logrus.Error("send to list for stock opname report email is empty, cannot send the report")
			} else {
				var sendList []EmailRecipient
				for i, email := range sendToList {
					sendList = append(sendList, EmailRecipient{
						Name:  fmt.Sprintf("Report SO Recipient - %d", i+1),
						Email: email,
					})
				} // .end of for looping the sendtolist of emails

				for i, data := range sendList {
					logrus.Infof("Processing index-%d [%s - %s] to send the report SO", i+1, data.Name, data.Email)
					mjmlTemplate, emailSubject := generateMJMLTemplateForReportSOWithSPOrWarningLetter("report_so", "", 0, data.Name, formattedMsgID)

					var emailTo []string
					emailTo = append(emailTo, data.Email)
					var attachments []fun.EmailAttachment
					attachments = append(attachments, fun.EmailAttachment{
						FilePath:    excelSO,
						NewFileName: fmt.Sprintf("Laporan_Stock_Opname_SP_%s.xlsx", time.Now().Format("2006-01-02")),
					})

					err := fun.TrySendEmail(emailTo, nil, nil, emailSubject, mjmlTemplate, attachments)
					if err != nil {
						logrus.Errorf("got error while trying to send the report so to %s : %v", data.Email, err)
					} else {
						logrus.Infof("Report SO successfully sendto : %s", data.Email)
					}
				}
			}
		} else if sendUsing == "telegram" {
			// TODO: Implement Telegram sending for Stock Opname Excel report
			logrus.Info("📋 Stock Opname Excel report via Telegram - Implementation pending")
			// When ready:
			// 1. Get chat IDs from telegram_users table for SO recipients
			// 2. Call SendSOReportViaTelegram() with Excel file
			// 3. Track in separate so_telegram_messages table
		} else {
			sendToList := config.GetConfig().StockOpname.SOReportSendTo
			if len(sendToList) == 0 {
				logrus.Error("send to list for stock opname report is empty, cannot send the report")
			} else {
				for _, phoneNumber := range sendToList {
					jidStr := fmt.Sprintf("%s@%s", phoneNumber, types.DefaultUserServer)
					SendLangDocumentViaBotWhatsapp(jidStr, msgID, msgID, "id", excelSO)
				} // .end of for looping the send to phone number list about the SO report
			}
		} // .end of send using email/telegram/whatsapp
	}
	// .end of (2)

	var deadlineSPFormatted string
	deadlineSP := time.Date(
		time.Now().Year(),
		time.Now().Month(),
		time.Now().Day(),
		maxResponseSPAtHour, 0, 0, 0,
		time.Now().Location(),
	)
	tgldeadlineSP, err := tanggal.Papar(deadlineSP, "Jakarta", tanggal.WIB)
	if err != nil {
		logrus.Warnf("failed to formatted indo date of deadline SP : %v", err)
		deadlineSPFormatted = deadlineSP.Format("Monday, 02 January 2006 15:04:05 MST")
	} else {
		deadlineSPFormatted = tgldeadlineSP.Format(" ", []tanggal.Format{
			tanggal.NamaHariDenganKoma,
			tanggal.Hari,
			tanggal.NamaBulan,
			tanggal.Tahun,
			tanggal.PukulDenganDetik,
			tanggal.ZonaWaktu,
		})
	}
	deadlineSPFooter := fmt.Sprintf("⚠️ *_Batas untuk menyanggah SP ini sampai : %s._*\nMohon untuk membalas pesan ini dengan fitur ```Reply```.", deadlineSPFormatted)

	/*
		Send SP to WhatsApp or Email (for Debug)
	*/
	sendUsingEmail := config.GetConfig().SPTechnician.EmailUsedForTest

	batchSize := 50
	timeSleepStartDuration := 20

	/*
		Sending SP via Telegram Implementation:
		1. New model SPTelegramMessage tracks all sent SPs via Telegram
		2. Functions: SendSPViaTelegram, SendSPDocumentTelegram handle Telegram sending
		3. Chat ID mapping required - users must interact with bot first to get chat_id
		4. Response tracking with deadline management implemented
		5. service-platform telegram bot will monitor incoming messages and update response status

		Variables updated:
		- needToSendTheSPTechnicianThroughWhatsapp -> now uses Telegram
		- needToSendTheSPSPLThroughWhatsapp -> now uses Telegram
		- needToSendTheSPSACThroughWhatsapp -> now uses Telegram
	*/

	if len(needToSendTheSPTechnicianThroughWhatsapp) > 0 {
		startTimeProcessSendSPTechnician := time.Now()
		logrus.Infof("[%s] start process send SP of technicians via WhatsApp or Email @%v", taskDoing, startTimeProcessSendSPTechnician)

		// Convert map to slice for batch processing
		type technicianSPPair struct {
			technician string
			spNumber   int
		}
		var technicianList []technicianSPPair
		for technician, spNumber := range needToSendTheSPTechnicianThroughWhatsapp {
			technicianList = append(technicianList, technicianSPPair{technician: technician, spNumber: spNumber})
		}

		// Process in batches
		for i := 0; i < len(technicianList); i += batchSize {
			end := i + batchSize
			if end > len(technicianList) {
				end = len(technicianList)
			}
			batch := technicianList[i:end]

			logrus.Infof("[%s] Processing batch %d/%d (items %d-%d of %d)",
				taskDoing, (i/batchSize)+1, (len(technicianList)+batchSize-1)/batchSize, i+1, end, len(technicianList))

			for _, pair := range batch {
				technician := pair.technician
				spNumber := pair.spNumber
				var spl, sac string
				var techName, splName, sacName string
				var splPhoneNumber, sacPhoneNumber string
				if techData, exists := TechODOOMSData[technician]; exists {
					spl = techData.SPL
					sac = techData.SAC
					if techData.Name != "" {
						techName = techData.Name
					} else {
						techName = technician
					}
				}

				if spl == "" || sac == "" {
					logrus.Warnf("failed to send WhatsApp warning letter of technician %s, coz its spl or sac name is empty", technician)
					continue
				}
				if splData, exists := TechODOOMSData[spl]; exists {
					if splData.Name != "" {
						splName = splData.Name
					} else {
						splName = spl
					}
					splPhoneNumber = splData.NoHP
				}
				sanitizedPhoneSPL, err := fun.SanitizePhoneNumber(splPhoneNumber)
				if err != nil {
					logrus.Errorf("failed to sanitized phone number spl %s : %v", spl, err)
					continue
				} else {
					splPhoneNumber = "62" + sanitizedPhoneSPL
				}

				SACDataTechnician, ok := ODOOMSSAC[sac]
				if !ok {
					logrus.Errorf("no SAC data found for technician: %s", technician)
					continue
				}
				if SACDataTechnician.FullName != "" {
					sacName = SACDataTechnician.FullName
				} else {
					sacName = sac
				}
				if SACDataTechnician.PhoneNumber != "" {
					sacPhoneNumber = SACDataTechnician.PhoneNumber
				}
				if sacPhoneNumber == "" {
					logrus.Warnf("cannot find the phone number for SAC %s", sacName)
					continue
				}
				sanitizedSACPhone, err := fun.SanitizePhoneNumber(sacPhoneNumber)
				if err != nil {
					logrus.Errorf("cannot sanitize SAC %s phone : %v", sacName, err)
					continue
				} else {
					sacPhoneNumber = "62" + sanitizedSACPhone
				}

				var jidStrSPL, jidStrSAC string
				jidStrSPL = fmt.Sprintf("%s@%s", splPhoneNumber, types.DefaultUserServer)
				jidStrSAC = fmt.Sprintf("%s@%s", sacPhoneNumber, types.DefaultUserServer)
				if jidStrSPL == "" || jidStrSAC == "" {
					logrus.Warnf("cannot get jid for SPL %s and SAC %s", splName, sacName)
					continue
				}
				techName = fun.CapitalizeWord(techName)
				splName = fun.CapitalizeWord(splName)
				sacName = fun.CapitalizeWord(sacName)

				var spTechnician sptechnicianmodel.TechnicianGotSP
				if err := dbWeb.Where("technician = ?", technician).First(&spTechnician).Error; err != nil {
					logrus.Errorf("failed to fetch sp of technician %s : %v", technician, err)
					continue
				}

				var spFilePathTechnician string

				var sbID strings.Builder // Store the message of introduce the warning letter

				switch spNumber {
				case 1:
					spFilePathTechnician = spTechnician.SP1FilePath

					sbID.WriteString(fmt.Sprintf("Dengan ini, kami menyampaikan bahwa saudara *%s* menerima Surat Peringatan (SP - 1).\n", techName))
					sbID.WriteString(fmt.Sprintf("Sehubungan dengan sikap tidak disiplin / pelanggaran terhadap tata tertib Perusahaan yang Karyawan lakukan yaitu:\n\n%s\n\n", spTechnician.PelanggaranSP1))
					sbID.WriteString("Mohon menjadi perhatian dan segera melakukan perbaikan yang diperlukan.\n")
					sbID.WriteString("Terima kasih.\n\n")
					sbID.WriteString(deadlineSPFooter)
				case 2:
					spFilePathTechnician = spTechnician.SP2FilePath

					sbID.WriteString(fmt.Sprintf(
						"Dengan ini, kami menyampaikan bahwa saudara *%s* menerima Surat Peringatan (SP-2).\n\n",
						techName,
					))

					sbID.WriteString("📌 *Riwayat Pelanggaran:*\n")
					var gotSP1AtStr string
					if spTechnician.GotSP1At == nil || spTechnician.GotSP1At.IsZero() {
						gotSP1AtStr = "N/A"
					} else {
						gotSP1AtStr = spTechnician.GotSP1At.Format("02 January 2006 15:04 MST")
					}
					sbID.WriteString(fmt.Sprintf(
						"• _SP-1_: %s (pada %s)\n\n",
						spTechnician.PelanggaranSP1,
						gotSP1AtStr,
					))

					sbID.WriteString(fmt.Sprintf(
						"Saudara kembali melakukan pelanggaran *%s*, dan hingga saat ini belum terdapat perbaikan yang memadai.\n\n",
						spTechnician.PelanggaranSP2,
					))

					sbID.WriteString("Maka, perusahaan memutuskan untuk menindaklanjuti melalui penerbitan *Surat Peringatan 2 (SP-2)*.\n\n")
					sbID.WriteString("Mohon menjadi perhatian dan segera melakukan perbaikan yang diperlukan.\n")
					sbID.WriteString("Terima kasih.\n\n")
					sbID.WriteString(deadlineSPFooter)

				case 3:
					spFilePathTechnician = spTechnician.SP3FilePath

					sbID.WriteString(fmt.Sprintf("Sehubungan dengan telah dikeluarkannya Surat Peringatan Pertama (SP-1) dan Surat Peringatan Kedua (SP-2) yang telah sebelumnya diberikan kepada Sdr. %s, namun yang bersangkutan tetap tidak menunjukkan perubahan sikap dan perbaikan diri, serta masih melakukan pelanggaran terhadap Tata Tertib Perusahaan, maka dengan ini Perusahaan menyampaikan Surat Peringatan Ketiga (SP-3). Adapun pelanggaran yang dilakukan oleh Sdr. %s adalah sebagai berikut :\n\n", techName, techName))
					sbID.WriteString(fmt.Sprintf("1. %s\n", spTechnician.PelanggaranSP1))
					sbID.WriteString(fmt.Sprintf("2. %s\n", spTechnician.PelanggaranSP2))
					sbID.WriteString(fmt.Sprintf("3. %s\n\n", spTechnician.PelanggaranSP3))
					sbID.WriteString(fmt.Sprintf("Surat Peringatan Ketiga (SP-3) ini merupakan peringatan terakhir yang sekaligus menjadi dasar bagi Perusahaan terhadap Sdr. %s untuk meminta HRD melakukan tindak-lanjut sesuai peraturan karena telah berulang kali melakukan pelanggaran yang merugikan Perusahaan.\n\n", techName))
					sbID.WriteString(fmt.Sprintf("Demikian Surat Peringatan Ketiga terakhir ini disampaikan agar selanjutnya dapat menghubungi pihak HRD untuk melakukan klarifikasi lebih lanjut. Jika dalam jangka waktu 2 (dua) hari kerja dari SP-3 diterbitkan, Sdr. %s tidak melakukan sanggahan, maka dianggap Sdr. %s Menyetujui penerbitan SP-3 ini dan Perusahaan berhak menerbitkan Surat Pemutusan Hubungan Kerja (S-PHK).\n\n", techName, techName))
					sbID.WriteString(fmt.Sprintf("Untuk klarifikasi lebih lanjut, hubungi *%s* di +%s\n\n~ (HRD *%s*)", hrdPersonaliaName, hrdPhoneNumber, config.GetConfig().Default.PT))
				default:
					logrus.Warnf("cannot process of sp : %d for technician %s", spNumber, technician)
					continue
				}

				// Build personalized messages for SPL, SAC, and HRD
				var splMessage, sacMessage string
				var sbSPL strings.Builder
				sbSPL.WriteString(fmt.Sprintf("Halo, %s Pak %s.\n\n", greetingID, splName))
				sbSPL.WriteString(sbID.String())
				splMessage = sbSPL.String()
				var sbSAC strings.Builder
				switch {
				case strings.Contains(strings.ToLower(sacName), "tetty"):
					sbSAC.WriteString(fmt.Sprintf("Halo, %s Kak %s.\n\n", greetingID, sacName))
				default:
					sbSAC.WriteString(fmt.Sprintf("Halo, %s Pak %s.\n\n", greetingID, sacName))
				}
				sbSAC.WriteString(sbID.String())
				sacMessage = sbSAC.String()

				// Used if sendUsing == email
				var attachments []fun.EmailAttachment
				attachments = append(attachments, fun.EmailAttachment{
					FilePath:    spFilePathTechnician,
					NewFileName: fmt.Sprintf("Surat_Peringatan_%d_%s.pdf", spNumber, technician),
				})

				// Send to HRD
				dataHRD := config.GetConfig().Default.PTHRD
				for _, hrd := range dataHRD {
					if hrd.Name == "" || hrd.PhoneNumber == "" {
						continue // Skip if no name or phone
					}
					hrdName := fun.CapitalizeWord(hrd.Name)
					sanitizedHRDPhone, err := fun.SanitizePhoneNumber(hrd.PhoneNumber)
					if err != nil {
						logrus.Errorf("cannot sanitize HRD %s phone : %v", hrdName, err)
						continue
					}
					hrdPhoneNumber := "62" + sanitizedHRDPhone
					jidStrHRD := fmt.Sprintf("%s@%s", hrdPhoneNumber, types.DefaultUserServer)
					hrdMessage := fmt.Sprintf("Halo, %s Kak %s.\n\n%s", greetingID, hrdName, sbID.String())

					if sendUsing == "email" {
						formattedMsgID := strings.ReplaceAll(hrdMessage, "\n", "<br>")
						mjmlTemplate, emailSubject := generateMJMLTemplateForReportSOWithSPOrWarningLetter("sp_technician", technician, spNumber, hrd.Name, formattedMsgID)
						var emailTo []string
						emailTo = append(emailTo, hrd.Email)

						err := fun.TrySendEmail(emailTo, nil, nil, emailSubject, mjmlTemplate, attachments)
						if err != nil {
							logrus.Errorf("got error while trying to send the SP technician %s to HRD %s : %v", technician, hrd.Name, err)
						} else {
							logrus.Infof("SP technician %s successfully sendto HRD : %s", technician, hrd.Name)
						}
						// .end of send technician's SP through email HRD
					} else if sendUsing == "telegram" {
						// Send via Telegram
						chatID, _ := GetChatIDFromPhone(hrdPhoneNumber)
						err := SendSPDocumentViaTelegram(
							forProject, "hrd", hrdName, chatID, hrdMessage,
							spFilePathTechnician, spNumber, hrdPhoneNumber,
							technician, techName, spl, splName, sac, sacName,
							getPelanggaranByNumber(spNumber, spTechnician),
							getNoSuratByNumber(spNumber, spTechnician),
							&spTechnician.ID, nil, nil,
						)
						if err != nil {
							logrus.Errorf("Failed to send SP via Telegram to HRD %s: %v", hrd.Name, err)
						} else {
							logrus.Infof("✅ SP-%d queued for Telegram to HRD %s", spNumber, hrd.Name)
						}
					} else {
						sendLangDocumentMessageForSPTechnician(
							forProject, technician, jidStrHRD,
							hrdMessage, hrdMessage, "id",
							spFilePathTechnician, spNumber, hrdPhoneNumber,
						)
					} // .end of send SP through Telegram/WhatsApp HRD
					if sendUsing == "email" {
						formattedMsgID := strings.ReplaceAll(splMessage, "\n", "<br>")
						mjmlTemplate, emailSubject := generateMJMLTemplateForReportSOWithSPOrWarningLetter("sp_technician", technician, spNumber, splName, formattedMsgID)
						var emailTo []string
						emailTo = append(emailTo, sendUsingEmail)
						err := fun.TrySendEmail(emailTo, nil, nil, emailSubject, mjmlTemplate, attachments)
						if err != nil {
							logrus.Errorf("got error while trying to send the SP technician %s to SPL %s : %v", technician, splName, err)
						} else {
							logrus.Infof("SP technician %s successfully sendto SPL : %s", technician, splName)
						}
						// .end of send SP technician through email SPL
					} else if sendUsing == "telegram" {
						// Send via Telegram
						chatID, _ := GetChatIDFromPhone(splPhoneNumber)
						err := SendSPDocumentViaTelegram(
							forProject, "spl", splName, chatID, splMessage,
							spFilePathTechnician, spNumber, splPhoneNumber,
							technician, techName, spl, splName, sac, sacName,
							getPelanggaranByNumber(spNumber, spTechnician),
							getNoSuratByNumber(spNumber, spTechnician),
							&spTechnician.ID, nil, nil,
						)
						if err != nil {
							logrus.Errorf("Failed to send SP via Telegram to SPL %s: %v", splName, err)
						} else {
							logrus.Infof("✅ SP-%d queued for Telegram to SPL %s", spNumber, splName)
						}
					} else {
						sendLangDocumentMessageForSPTechnician(
							forProject, technician, jidStrSPL,
							splMessage, splMessage, "id",
							spFilePathTechnician, spNumber, splPhoneNumber,
						)
					}
				}

				// Send to SAC
				if jidStrSAC != "" {
					if sendUsing == "email" {
						formattedMsgID := strings.ReplaceAll(sacMessage, "\n", "<br>")
						mjmlTemplate, emailSubject := generateMJMLTemplateForReportSOWithSPOrWarningLetter("sp_technician", technician, spNumber, sacName, formattedMsgID)
						var emailTo []string
						emailTo = append(emailTo, sendUsingEmail)
						err := fun.TrySendEmail(emailTo, nil, nil, emailSubject, mjmlTemplate, attachments)
						if err != nil {
							logrus.Errorf("got error while trying to send the SP technician %s to SAC %s : %v", technician, sacName, err)
						} else {
							logrus.Infof("SP technician %s successfully sendto SAC : %s", technician, sacName)
						}
						// .end of send SP through email SAC
					} else if sendUsing == "telegram" {
						// Send via Telegram
						chatID, _ := GetChatIDFromPhone(sacPhoneNumber)
						err := SendSPDocumentViaTelegram(
							forProject, "sac", sacName, chatID, sacMessage,
							spFilePathTechnician, spNumber, sacPhoneNumber,
							technician, techName, spl, splName, sac, sacName,
							getPelanggaranByNumber(spNumber, spTechnician),
							getNoSuratByNumber(spNumber, spTechnician),
							&spTechnician.ID, nil, nil,
						)
						if err != nil {
							logrus.Errorf("Failed to send SP via Telegram to SAC %s: %v", sacName, err)
						} else {
							logrus.Infof("✅ SP-%d queued for Telegram to SAC %s", spNumber, sacName)
						}
					} else {
						sendLangDocumentMessageForSPTechnician(
							forProject, technician, jidStrSAC,
							sacMessage, sacMessage, "id",
							spFilePathTechnician, spNumber, sacPhoneNumber,
						)
					}
				}

				// Add random sleep between 20-50 seconds at the end of each item in the batch
				sleepDuration := time.Duration(timeSleepStartDuration+rand.Intn(31)) * time.Second
				logrus.Infof("[%s] Sleeping for %v before next item", taskDoing, sleepDuration)
				time.Sleep(sleepDuration)
			} // .end of for looping items in current batch

			// Sleep between batches if not the last batch
			if end < len(technicianList) {
				batchSleepDuration := time.Duration(1+rand.Intn(5)) * time.Second
				logrus.Infof("[%s] Batch completed. Sleeping for %v before next batch", taskDoing, batchSleepDuration)
				time.Sleep(batchSleepDuration)
			}
		} // .end of batch processing loop

		totalDurationProcessSendSPTechnician := time.Since(startTimeProcessSendSPTechnician)
		logrus.Infof("[%s] total duration process send SP technician : %v", taskDoing, totalDurationProcessSendSPTechnician)
	} else {
		logrus.Error("no SP technician need to send via WhatsApp or Email")
	} // .end of check if the Technician got sp

	if len(needToSendTheSPSPLThroughWhatsapp) > 0 {
		startTimeProcessSendSPSPL := time.Now()
		logrus.Infof("[%s] start process send SP of SPLs via WhatsApp or Email @%v", taskDoing, startTimeProcessSendSPSPL)

		// Convert map to slice for batch processing
		type splSPPair struct {
			spl      string
			spNumber int
		}
		var splList []splSPPair
		for spl, spNumber := range needToSendTheSPSPLThroughWhatsapp {
			splList = append(splList, splSPPair{spl: spl, spNumber: spNumber})
		}

		// Process in batches
		for i := 0; i < len(splList); i += batchSize {
			end := i + batchSize
			if end > len(splList) {
				end = len(splList)
			}
			batch := splList[i:end]

			logrus.Infof("[%s] Processing SPL batch %d/%d (items %d-%d of %d)",
				taskDoing, (i/batchSize)+1, (len(splList)+batchSize-1)/batchSize, i+1, end, len(splList))

			for _, pair := range batch {
				spl := pair.spl
				spNumber := pair.spNumber
				var splName, sac, sacName, sacPhoneNumber, jidStrSAC string
				if splData, exists := TechODOOMSData[spl]; exists {
					if splData.Name != "" {
						splName = splData.Name
					} else {
						splName = spl
					}
					sac = splData.SAC
				}

				if sac == "" {
					logrus.Warnf("failed to send WhatsApp sp of spl %s, coz its SAC is not empty", spl)
					continue
				}
				SACData, ok := ODOOMSSAC[sac]
				if !ok {
					logrus.Errorf("no SAC data found for spl %s", spl)
					continue
				}
				if SACData.FullName != "" {
					sacName = SACData.FullName
				} else {
					sacName = sac
				}
				if SACData.PhoneNumber != "" {
					sanitizedPhone, err := fun.SanitizePhoneNumber(SACData.PhoneNumber)
					if err != nil {
						logrus.Errorf("failed to sanitized phone number of sac %s : %v", sac, err)
						continue
					}
					sacPhoneNumber = "62" + sanitizedPhone
					jidStrSAC = fmt.Sprintf("%s@%s", sacPhoneNumber, types.DefaultUserServer)
				} else {
					logrus.Warnf("failed to get phone number of sac %s from spl %s", sac, spl)
					continue
				}
				splName = fun.CapitalizeWord(splName)
				sacName = fun.CapitalizeWord(sacName)

				var spSPL sptechnicianmodel.SPLGotSP
				if err := dbWeb.Where("spl = ?", spl).First(&spSPL).Error; err != nil {
					logrus.Errorf("failed to fetch data sp spl %s : %v", spl, err)
					continue
				}

				var spFilePathSPL string
				var sbID strings.Builder

				switch spNumber {
				case 1:
					spFilePathSPL = spSPL.SP1FilePath

					sbID.WriteString(fmt.Sprintf(
						"Dengan ini, kami menyampaikan bahwa saudara *%s* menerima Surat Peringatan (SP-1) sebagai Service Point Leader (SPL).\n\n",
						splName,
					))
					sbID.WriteString("📌 *Dasar Penerbitan SP-1:*\n")

					if spSPL.TechnicianNameCausedGotSP1 != "" {
						sbID.WriteString(fmt.Sprintf("• Teknisi *%s* di bawah naungan saudara\n", spSPL.TechnicianNameCausedGotSP1))
						sbID.WriteString("• Telah menerima *Surat Peringatan 3 (SP-3)*.\n\n")
						sbID.WriteString("Sebagai SPL, saudara bertanggung jawab dalam melakukan pembinaan, pengawasan, dan pengendalian kinerja teknisi. Kegagalan dalam pengawasan tersebut menjadi dasar diterbitkannya SP ini.\n\n")
					} else {
						sbID.WriteString(fmt.Sprintf("• Pelanggaran langsung oleh saudara: %s.\n\n", spSPL.PelanggaranSP1))
						sbID.WriteString("Sebagai SPL, saudara berkewajiban menjalankan tugas sesuai ketentuan dan standar perusahaan.\n\n")
					}

					sbID.WriteString("Mohon menjadi perhatian dan segera melakukan pembinaan serta pengawasan yang lebih optimal.\n")
					sbID.WriteString("Terima kasih.\n\n")
					sbID.WriteString(deadlineSPFooter)

				case 2:
					spFilePathSPL = spSPL.SP2FilePath

					var gotSP1AtStr string
					if spSPL.GotSP1At == nil || spSPL.GotSP1At.IsZero() {
						gotSP1AtStr = "N/A"
					} else {
						gotSP1AtStr = spSPL.GotSP1At.Format("02 January 2006 15:04 MST")
					}

					sbID.WriteString(fmt.Sprintf(
						"Merujuk pada Surat Peringatan (SP-1) yang telah diberikan kepada saudara *%s* pada %s,\n\n",
						splName,
						gotSP1AtStr,
					))
					sbID.WriteString("📌 *Fakta dan Kondisi:* \n")

					if spSPL.TechnicianNameCausedGotSP2 != "" {
						sbID.WriteString(fmt.Sprintf("• Pada SP-1, saudara dianggap lalai dalam melakukan pembinaan terhadap teknisi *%s* yang menerima *SP-3*.\n", spSPL.TechnicianNameCausedGotSP1))
						sbID.WriteString(fmt.Sprintf("• Saat ini, teknisi di bawah pengawasan saudara kembali melakukan pelanggaran berat hingga diterbitkannya *SP-3* untuk teknisi *%s*.\n", spSPL.TechnicianNameCausedGotSP2))
					} else {
						sbID.WriteString(fmt.Sprintf("• Pelanggaran kembali dilakukan oleh saudara: %s.\n", spSPL.PelanggaranSP2))
						sbID.WriteString("• Tidak terdapat perbaikan setelah diterbitkannya SP-1.\n")
					}

					sbID.WriteString("• Pengawasan serta pembinaan tidak menunjukkan peningkatan yang memadai.\n\n")
					sbID.WriteString("Dengan mempertimbangkan hal tersebut, perusahaan memutuskan untuk menerbitkan *Surat Peringatan 2 (SP-2)* kepada saudara.\n\n")
					sbID.WriteString("Mohon menjadi perhatian serius dan segera melakukan pengawasan serta pembinaan secara lebih efektif.\n")
					sbID.WriteString("Terima kasih.\n\n")
					sbID.WriteString(deadlineSPFooter)

				case 3:
					spFilePathSPL = spSPL.SP3FilePath

					sbID.WriteString(fmt.Sprintf("Sehubungan dengan telah dikeluarkannya Surat Peringatan Pertama (SP-1) dan Surat Peringatan Kedua (SP-2) yang telah sebelumnya diberikan kepada Sdr. %s, namun yang bersangkutan tetap tidak menunjukkan perubahan sikap dan perbaikan diri, serta masih melakukan pelanggaran terhadap Tata Tertib Perusahaan, maka dengan ini Perusahaan menyampaikan Surat Peringatan Ketiga (SP-3). Adapun pelanggaran yang dilakukan oleh Sdr. %s adalah sebagai berikut :\n\n", splName, splName))
					sbID.WriteString(fmt.Sprintf("1. %s\n", spSPL.PelanggaranSP1))
					sbID.WriteString(fmt.Sprintf("2. %s\n", spSPL.PelanggaranSP2))
					sbID.WriteString(fmt.Sprintf("3. %s\n\n", spSPL.PelanggaranSP3))
					sbID.WriteString(fmt.Sprintf("Surat Peringatan Ketiga (SP-3) ini merupakan peringatan terakhir yang sekaligus menjadi dasar bagi Perusahaan terhadap Sdr. %s untuk meminta HRD melakukan tindak-lanjut sesuai peraturan karena telah berulang kali melakukan pelanggaran yang merugikan Perusahaan.\n\n", splName))
					sbID.WriteString(fmt.Sprintf("Demikian Surat Peringatan Ketiga terakhir ini disampaikan agar selanjutnya dapat menghubungi pihak HRD untuk melakukan klarifikasi lebih lanjut. Jika dalam jangka waktu 2 (dua) hari kerja dari SP-3 diterbitkan, Sdr. %s tidak melakukan sanggahan, maka dianggap Sdr. %s Menyetujui penerbitan SP-3 ini dan Perusahaan berhak menerbitkan Surat Pemutusan Hubungan Kerja (S-PHK).\n\n", splName, splName))
					sbID.WriteString(fmt.Sprintf("Untuk klarifikasi lebih lanjut, hubungi *%s* di +%s\n\n~ (HRD *%s*)", hrdPersonaliaName, hrdPhoneNumber, config.GetConfig().Default.PT))

				default:
					logrus.Warnf("cannot process of sp : %d for spl %s", spNumber, spl)
					continue
				}

				// Build personalized messages for SAC and HRD
				var sacMessage string

				var sbSAC strings.Builder
				switch {
				case strings.Contains(strings.ToLower(sacName), "tetty"):
					sbSAC.WriteString(fmt.Sprintf("Halo, %s Kak %s.\n\n", greetingID, sacName))
				default:
					sbSAC.WriteString(fmt.Sprintf("Halo, %s Pak %s.\n\n", greetingID, sacName))
				}
				sbSAC.WriteString(sbID.String())
				sacMessage = sbSAC.String()

				// Used if sendUsing == email
				var attachments []fun.EmailAttachment
				attachments = append(attachments, fun.EmailAttachment{
					FilePath:    spFilePathSPL,
					NewFileName: fmt.Sprintf("Surat_Peringatan_SPL_%d_%s.pdf", spNumber, spl),
				})

				// Send to HRD
				dataHRD := config.GetConfig().Default.PTHRD
				for _, hrd := range dataHRD {
					if hrd.Name == "" || hrd.PhoneNumber == "" {
						continue // Skip if no name or phone
					}
					hrdName := fun.CapitalizeWord(hrd.Name)
					sanitizedHRDPhone, err := fun.SanitizePhoneNumber(hrd.PhoneNumber)
					if err != nil {
						logrus.Errorf("cannot sanitize HRD %s phone : %v", hrdName, err)
						continue
					}
					hrdPhoneNumber := "62" + sanitizedHRDPhone
					jidStrHRD := fmt.Sprintf("%s@%s", hrdPhoneNumber, types.DefaultUserServer)
					hrdMessage := fmt.Sprintf("Halo, %s Kak %s.\n\n%s", greetingID, hrdName, sbID.String())

					if sendUsing == "email" {
						formattedMsgID := strings.ReplaceAll(hrdMessage, "\n", "<br>")
						mjmlTemplate, emailSubject := generateMJMLTemplateForReportSOWithSPOrWarningLetter("sp_spl", spl, spNumber, hrd.Name, formattedMsgID)
						var emailTo []string
						emailTo = append(emailTo, hrd.Email)

						err := fun.TrySendEmail(emailTo, nil, nil, emailSubject, mjmlTemplate, attachments)
						if err != nil {
							logrus.Errorf("got error while trying to send the SP SPL %s to HRD %s : %v", spl, hrd.Name, err)
						} else {
							logrus.Infof("SP SPL %s successfully sendto HRD : %s", spl, hrd.Name)
						}
						// .end of send SP SPL through HRD email
					} else {
						sendLangDocumentMessageForSPSPL(
							forProject,
							spl,
							jidStrHRD,
							hrdMessage,
							hrdMessage,
							"id",
							spFilePathSPL,
							spNumber,
							hrdPhoneNumber,
						)
					} // .end of send SP SPL through HRD whatsapp
				}

				// Send to SAC
				if jidStrSAC != "" {
					if sendUsing == "email" {
						formattedMsgID := strings.ReplaceAll(sacMessage, "\n", "<br>")
						mjmlTemplate, emailSubject := generateMJMLTemplateForReportSOWithSPOrWarningLetter("sp_spl", spl, spNumber, sacName, formattedMsgID)
						var emailTo []string
						if SACData.Email != "" {
							emailTo = append(emailTo, SACData.Email)
						} else {
							emailTo = append(emailTo, sendUsingEmail)
						}

						err := fun.TrySendEmail(emailTo, nil, nil, emailSubject, mjmlTemplate, attachments)
						if err != nil {
							logrus.Errorf("got error while trying to send the SP SPL %s to SAC %s : %v", spl, sacName, err)
						} else {
							logrus.Infof("SP SPL %s successfully sent to SAC : %s", spl, sacName)
						}
					} else if sendUsing == "telegram" {
						chatID, _ := GetChatIDFromPhone(sacPhoneNumber)
						// Note: In batch mode spl is just a string name, not full object
						err := SendSPDocumentViaTelegram(
							forProject, "sac", sacName, chatID, sacMessage,
							spFilePathSPL, spNumber, sacPhoneNumber,
							"", "", "", spl, "", sacName,
							"", 0, // pelanggaran and noSurat not available in batch mode
							nil, nil, nil,
						)
						if err != nil {
							logrus.Errorf("Error sending SP SPL to SAC via Telegram: %v", err)
						} else {
							logrus.Infof("✅ SP-%d queued for Telegram to SAC %s", spNumber, sacName)
						}
					} else {
						sendLangDocumentMessageForSPSPL(
							forProject,
							spl,
							jidStrSAC,
							sacMessage,
							sacMessage,
							"id",
							spFilePathSPL,
							spNumber,
							sacPhoneNumber,
						)
						// .end of send SP SPL through SAC whatsapp
					} // .end of send SP SPL through SAC whatsapp/telegram
				}

				// Add random sleep between 20-50 seconds at the end of each item in the batch
				sleepDuration := time.Duration(timeSleepStartDuration+rand.Intn(31)) * time.Second
				logrus.Infof("[%s] Sleeping for %v before next SPL item", taskDoing, sleepDuration)
				time.Sleep(sleepDuration)
			} // .end of for looping items in current SPL batch

			// Sleep between batches if not the last batch
			if end < len(splList) {
				batchSleepDuration := time.Duration(1+rand.Intn(5)) * time.Second
				logrus.Infof("[%s] SPL batch completed. Sleeping for %v before next batch", taskDoing, batchSleepDuration)
				time.Sleep(batchSleepDuration)
			}
		} // .end of SPL batch processing loop

		totalDurationProcessSendSPSPL := time.Since(startTimeProcessSendSPSPL)
		logrus.Infof("[%s] total duration process send SP SPL : %v", taskDoing, totalDurationProcessSendSPSPL)
	} else {
		logrus.Error("no SP SPL need to send via WhatsApp or Email")
	} // .end of check if the SPL got sp

	if len(needToSendTheSPSACThroughWhatsapp) > 0 {
		startTimeProcessSendSPSAC := time.Now()
		logrus.Infof("[%s] start process send SP of SACs via WhatsApp or Email @%v", taskDoing, startTimeProcessSendSPSAC)

		// Convert map to slice for batch processing
		type sacSPPair struct {
			sac      string
			spNumber int
		}
		var sacList []sacSPPair
		for sac, spNumber := range needToSendTheSPSACThroughWhatsapp {
			sacList = append(sacList, sacSPPair{sac: sac, spNumber: spNumber})
		}

		// Process in batches
		for i := 0; i < len(sacList); i += batchSize {
			end := i + batchSize
			if end > len(sacList) {
				end = len(sacList)
			}
			batch := sacList[i:end]

			logrus.Infof("[%s] Processing SAC batch %d/%d (items %d-%d of %d)",
				taskDoing, (i/batchSize)+1, (len(sacList)+batchSize-1)/batchSize, i+1, end, len(sacList))

			for _, pair := range batch {
				sac := pair.sac
				spNumber := pair.spNumber
				var sacName, sacPhoneNumber, jidStrSAC string
				var sacRegion int

				SACData, ok := ODOOMSSAC[sac]
				if !ok {
					logrus.Errorf("no SAC data found of sac %s", sac)
					continue
				}
				if SACData.FullName != "" {
					sacName = SACData.FullName
				} else {
					sacName = sac
				}
				sacRegion = SACData.Region
				if SACData.PhoneNumber != "" {
					sanitizedPhone, err := fun.SanitizePhoneNumber(SACData.PhoneNumber)
					if err != nil {
						logrus.Errorf("failed to sanitized phone number of sac %s : %v", sac, err)
						continue
					}
					sacPhoneNumber = "62" + sanitizedPhone
					jidStrSAC = fmt.Sprintf("%s@%s", sacPhoneNumber, types.DefaultUserServer)
				} else {
					logrus.Warnf("failed to get phone number of sac %s", sac)
					continue
				}
				sacName = fun.CapitalizeWord(sacName)

				var spSAC sptechnicianmodel.SACGotSP
				if err := dbWeb.Where("sac = ?", sac).First(&spSAC).Error; err != nil {
					logrus.Errorf("failed to fetch data sp sac %s : %v", sac, err)
					continue
				}

				var spFilePathSAC string
				var sbID strings.Builder

				switch spNumber {
				case 1:
					spFilePathSAC = spSAC.SP1FilePath

					sbID.WriteString(fmt.Sprintf(
						"Dengan ini, kami menyampaikan bahwa saudara(i) *%s* menerima Surat Peringatan (SP-1) sebagai Service Area Coordinator (SAC) - Region %d.\n\n",
						sacName, sacRegion,
					))

					sbID.WriteString("📌 *Dasar Penerbitan SP-1:*\n")

					if spSAC.TechnicianNameCausedGotSP1 != "" {
						sbID.WriteString(fmt.Sprintf("• Teknisi *%s* di bawah naungan saudara(i)\n", spSAC.TechnicianNameCausedGotSP1))
						sbID.WriteString("• Telah menerima *Surat Peringatan 3 (SP-3)*.\n\n")
						sbID.WriteString("Sebagai SAC, saudara(i) bertanggung jawab dalam melakukan pembinaan, pengawasan, dan pengendalian kinerja teknisi. Kegagalan dalam pengawasan tersebut menjadi dasar diterbitkannya SP ini.\n\n")
					} else {
						sbID.WriteString(fmt.Sprintf("• Pelanggaran langsung oleh saudara(i): %s.\n\n", spSAC.PelanggaranSP1))
						sbID.WriteString("Sebagai SAC, saudara(i) berkewajiban menjalankan tugas sesuai ketentuan dan standar perusahaan.\n\n")
					}

					sbID.WriteString("Mohon menjadi perhatian dan segera melakukan pembinaan serta pengawasan yang lebih optimal.\n")
					sbID.WriteString("Terima kasih.\n\n")
					sbID.WriteString(deadlineSPFooter)

				case 2:
					spFilePathSAC = spSAC.SP2FilePath

					var gotSP1AtStr string
					if spSAC.GotSP1At == nil || spSAC.GotSP1At.IsZero() {
						gotSP1AtStr = "N/A"
					} else {
						gotSP1AtStr = spSAC.GotSP1At.Format("02 January 2006 15:04 MST")
					}

					sbID.WriteString(fmt.Sprintf(
						"Merujuk pada Surat Peringatan (SP-1) yang telah diberikan kepada saudara(i) *%s* pada %s,\n\n",
						sacName,
						gotSP1AtStr,
					))
					sbID.WriteString("📌 *Fakta dan Kondisi:* \n")

					if spSAC.TechnicianNameCausedGotSP2 != "" {
						sbID.WriteString(fmt.Sprintf("• Pada SP-1, saudara(i) dianggap lalai dalam melakukan pembinaan terhadap teknisi *%s* yang menerima *SP-3*.\n", spSAC.TechnicianNameCausedGotSP1))
						sbID.WriteString(fmt.Sprintf("• Saat ini, teknisi di bawah pengawasan saudara(i) kembali melakukan pelanggaran berat hingga diterbitkannya *SP-3* untuk teknisi *%s*.\n", spSAC.TechnicianNameCausedGotSP2))
					} else {
						sbID.WriteString(fmt.Sprintf("• Pelanggaran kembali dilakukan oleh saudara(i): %s.\n", spSAC.PelanggaranSP2))
						sbID.WriteString("• Tidak terdapat perbaikan setelah diterbitkannya SP-1.\n")
					}

					sbID.WriteString("• Pengawasan serta pembinaan tidak menunjukkan peningkatan yang memadai.\n\n")
					sbID.WriteString("Dengan mempertimbangkan hal tersebut, perusahaan memutuskan untuk menerbitkan *Surat Peringatan 2 (SP-2)* kepada saudara(i).\n\n")
					sbID.WriteString("Mohon menjadi perhatian serius dan segera melakukan pengawasan serta pembinaan secara lebih efektif.\n")
					sbID.WriteString("Terima kasih.\n\n")
					sbID.WriteString(deadlineSPFooter)

				case 3:
					spFilePathSAC = spSAC.SP3FilePath

					sbID.WriteString(fmt.Sprintf("Sehubungan dengan telah dikeluarkannya Surat Peringatan Pertama (SP-1) dan Surat Peringatan Kedua (SP-2) yang telah sebelumnya diberikan kepada Sdr. %s, namun yang bersangkutan tetap tidak menunjukkan perubahan sikap dan perbaikan diri, serta masih melakukan pelanggaran terhadap Tata Tertib Perusahaan, maka dengan ini Perusahaan menyampaikan Surat Peringatan Ketiga (SP-3). Adapun pelanggaran yang dilakukan oleh Sdr. %s adalah sebagai berikut :\n\n", sacName, sacName))
					sbID.WriteString(fmt.Sprintf("1. %s\n", spSAC.PelanggaranSP1))
					sbID.WriteString(fmt.Sprintf("2. %s\n", spSAC.PelanggaranSP2))
					sbID.WriteString(fmt.Sprintf("3. %s\n\n", spSAC.PelanggaranSP3))
					sbID.WriteString(fmt.Sprintf("Surat Peringatan Ketiga (SP-3) ini merupakan peringatan terakhir yang sekaligus menjadi dasar bagi Perusahaan terhadap Sdr. %s untuk meminta HRD melakukan tindak-lanjut sesuai peraturan karena telah berulang kali melakukan pelanggaran yang merugikan Perusahaan.\n\n", sacName))
					sbID.WriteString(fmt.Sprintf("Demikian Surat Peringatan Ketiga terakhir ini disampaikan agar selanjutnya dapat menghubungi pihak HRD untuk melakukan klarifikasi lebih lanjut. Jika dalam jangka waktu 2 (dua) hari kerja dari SP-3 diterbitkan, Sdr. %s tidak melakukan sanggahan, maka dianggap Sdr. %s Menyetujui penerbitan SP-3 ini dan Perusahaan berhak menerbitkan Surat Pemutusan Hubungan Kerja (S-PHK).\n\n", sacName, sacName))
					sbID.WriteString(fmt.Sprintf("Untuk klarifikasi lebih lanjut, hubungi *%s* di +%s\n\n~ (HRD *%s*)", hrdPersonaliaName, hrdPhoneNumber, config.GetConfig().Default.PT))
				default:
					logrus.Warnf("cannot process of sp : %d for sac %s", spNumber, sac)
					continue
				}

				// Build personalized messages for SAC and HRD
				var sacMessage string
				var sbSAC strings.Builder
				switch {
				case strings.Contains(strings.ToLower(sacName), "tetty"):
					sbSAC.WriteString(fmt.Sprintf("Halo, %s Kak %s.\n\n", greetingID, sacName))
				default:
					sbSAC.WriteString(fmt.Sprintf("Halo, %s Pak %s.\n\n", greetingID, sacName))
				}
				sbSAC.WriteString(sbID.String())
				sacMessage = sbSAC.String()

				// Used if sendUsing == email
				var attachments []fun.EmailAttachment
				attachments = append(attachments, fun.EmailAttachment{
					FilePath:    spFilePathSAC,
					NewFileName: fmt.Sprintf("Surat_Peringatan_%d_%s.pdf", spNumber, sac),
				})

				// Send to HRD
				dataHRD := config.GetConfig().Default.PTHRD
				for _, hrd := range dataHRD {
					if hrd.Name == "" || hrd.PhoneNumber == "" {
						continue // Skip if no name or phone
					}
					hrdName := fun.CapitalizeWord(hrd.Name)
					sanitizedHRDPhone, err := fun.SanitizePhoneNumber(hrd.PhoneNumber)
					if err != nil {
						logrus.Errorf("cannot sanitize HRD %s phone : %v", hrdName, err)
						continue
					}
					hrdPhoneNumber := "62" + sanitizedHRDPhone
					jidStrHRD := fmt.Sprintf("%s@%s", hrdPhoneNumber, types.DefaultUserServer)
					hrdMessage := fmt.Sprintf("Halo, %s Kak %s.\n\n%s", greetingID, hrdName, sbID.String())

					if sendUsing == "email" {
						formattedMsgID := strings.ReplaceAll(hrdMessage, "\n", "<br>")
						mjmlTemplate, emailSubject := generateMJMLTemplateForReportSOWithSPOrWarningLetter("sp_sac", sac, spNumber, hrd.Name, formattedMsgID)
						var emailTo []string
						emailTo = append(emailTo, hrd.Email)

						err := fun.TrySendEmail(emailTo, nil, nil, emailSubject, mjmlTemplate, attachments)
						if err != nil {
							logrus.Errorf("got error while trying to send the SP SAC %s to HRD %s : %v", sac, hrd.Name, err)
						} else {
							logrus.Infof("SP SAC %s successfully sendto HRD : %s", sac, hrd.Name)
						}
						// .end of send SP SAC through HRD email
					} else {
						sendLangDocumentMessageForSPSAC(
							forProject,
							sac,
							jidStrHRD,
							hrdMessage,
							hrdMessage,
							"id",
							spFilePathSAC,
							spNumber,
							hrdPhoneNumber,
						)
					} // .end of send SP SAC through HRD whatsapp
				}

				// Send to SAC
				if jidStrSAC != "" {
					if sendUsing == "email" {
						formattedMsgID := strings.ReplaceAll(sacMessage, "\n", "<br>")
						mjmlTemplate, emailSubject := generateMJMLTemplateForReportSOWithSPOrWarningLetter("sp_sac", sac, spNumber, sacName, formattedMsgID)
						var emailTo []string
						if SACData.Email != "" {
							emailTo = append(emailTo, SACData.Email)
						} else {
							emailTo = append(emailTo, sendUsingEmail)
						}

						err := fun.TrySendEmail(emailTo, nil, nil, emailSubject, mjmlTemplate, attachments)
						if err != nil {
							logrus.Errorf("got error while trying to send the SP SAC %s to SAC %s : %v", sac, sacName, err)
						} else {
							logrus.Infof("SP SAC %s successfully sent to SAC : %s", sac, sacName)
						}
					} else if sendUsing == "telegram" {
						chatID, _ := GetChatIDFromPhone(sacPhoneNumber)
						// Note: In batch mode sac is just a string name, not full object
						err := SendSPDocumentViaTelegram(
							forProject, "sac", sacName, chatID, sacMessage,
							spFilePathSAC, spNumber, sacPhoneNumber,
							"", "", "", "", "", sacName,
							"", 0, // pelanggaran and noSurat not available in batch mode
							nil, nil, nil,
						)
						if err != nil {
							logrus.Errorf("Error sending SP SAC to SAC via Telegram: %v", err)
						} else {
							logrus.Infof("✅ SP-%d queued for Telegram to SAC %s", spNumber, sacName)
						}
					} else {
						sendLangDocumentMessageForSPSAC(
							forProject,
							sac,
							jidStrSAC,
							sacMessage,
							sacMessage,
							"id",
							spFilePathSAC,
							spNumber,
							sacPhoneNumber,
						)
					} // .end of send SP SAC through SAC whatsapp/telegram
				}

				// Add random sleep between 20-50 seconds at the end of each item in the batch
				sleepDuration := time.Duration(timeSleepStartDuration+rand.Intn(31)) * time.Second
				logrus.Infof("[%s] Sleeping for %v before next SAC item", taskDoing, sleepDuration)
				time.Sleep(sleepDuration)
			} // .end of for looping items in current SAC batch

			// Sleep between batches if not the last batch
			if end < len(sacList) {
				batchSleepDuration := time.Duration(1+rand.Intn(5)) * time.Second
				logrus.Infof("[%s] SAC batch completed. Sleeping for %v before next batch", taskDoing, batchSleepDuration)
				time.Sleep(batchSleepDuration)
			}
		} // .end of SAC batch processing loop

		totalDurationProcessSendSPSAC := time.Since(startTimeProcessSendSPSAC)
		logrus.Infof("[%s] total duration process send SP SAC : %v", taskDoing, totalDurationProcessSendSPSAC)
	} else {
		logrus.Error("no SP SAC need to send via WhatsApp or Email")
	} // .end of check if the SAC got sp

	// ADD: gave warning letter for ATM / Dedicated ATM technicians if needed !

	totalDuration := time.Since(startTime)
	logrus.Infof("%s completed in %v", taskDoing, totalDuration)

	return nil
}
