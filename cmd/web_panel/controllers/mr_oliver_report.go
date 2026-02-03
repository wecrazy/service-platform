package controllers

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"service-platform/cmd/web_panel/config"
	"service-platform/cmd/web_panel/fun"
	"service-platform/cmd/web_panel/internal/gormdb"
	reportmodel "service-platform/cmd/web_panel/model/report_model"
	tamodel "service-platform/cmd/web_panel/model/ta_model"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/sirupsen/logrus"
	"github.com/xuri/excelize/v2"
	"go.mau.fi/whatsmeow/types/events"
)

var (
	checkMrOliverReportExistsMutex          sync.Mutex
	generateReportAIErrorMutex              sync.Mutex
	getDataTicketEngineersProductivityMutex sync.Mutex
)

type MrOliverReport struct {
	ReportType  string
	Links       []string
	CanDownload map[string]bool // Stores link -> status
}

type ODOOEngineersProductivityItem struct {
	ID                  uint              `json:"id"`
	TicketNumber        string            `json:"name"`
	TaskType            nullAbleString    `json:"x_task_type"`
	ReasonCode          nullAbleString    `json:"x_reasoncode"`
	Reason              nullAbleString    `json:"x_reason"`
	Source              nullAbleString    `json:"x_source"`
	Merchant            nullAbleString    `json:"x_merchant"`
	MerchantPic         nullAbleString    `json:"x_merchant_pic"`
	MerchantAddress     nullAbleString    `json:"x_studio_alamat"`
	MerchantCity        nullAbleString    `json:"x_studio_kota"`
	Mid                 nullAbleString    `json:"x_master_mid"`
	Tid                 nullAbleString    `json:"x_master_tid"`
	LinkWod             nullAbleString    `json:"x_link"`
	WoNumberFirst       nullAbleString    `json:"x_wo_number"`
	WoNumberLast        nullAbleString    `json:"x_wo_number_last"`
	Description         nullAbleString    `json:"description"`
	WoRemark            nullAbleString    `json:"x_wo_remark"`
	CompanyId           nullAbleInterface `json:"company_id"`
	TechnicianId        nullAbleInterface `json:"technician_id"`
	TicketTypeId        nullAbleInterface `json:"ticket_type_id"`
	WorksheetTemplateId nullAbleInterface `json:"x_worksheet_template_id"`
	StageId             nullAbleInterface `json:"stage_id"`
	ProjectId           nullAbleInterface `json:"project_id"`
	SnEdcId             nullAbleInterface `json:"x_merchant_sn_edc"`
	EdcTypeId           nullAbleInterface `json:"x_merchant_tipe_edc"`
	TaskCount           nullAbleInteger   `json:"fsm_task_count"`
	SLADeadline         nullAbleTime      `json:"x_sla_deadline"`
	ReceivedDatetimeSPK nullAbleTime      `json:"x_received_datetime_spk"`
	CompleteDatetimeWO  nullAbleTime      `json:"complete_datetime_wo"`
	TicketCreatedAt     nullAbleTime      `json:"create_date"`
}

func (t *ODOOEngineersProductivityItem) UnmarshalJSON(data []byte) error {
	type Alias ODOOEngineersProductivityItem // Alias to avoid infinite recursion
	aux := &struct {
		ReceivedDatetimeSPK interface{} `json:"x_received_datetime_spk"`
		SLADeadline         interface{} `json:"x_sla_deadline"`
		CompleteDatetimeWO  interface{} `json:"complete_datetime_wo"`
		TicketCreatedAt     interface{} `json:"create_date"`
		*Alias
	}{
		Alias: (*Alias)(t),
	}

	// Unmarshal JSON into aux struct
	if err := json.Unmarshal(data, &aux); err != nil {
		return fmt.Errorf("failed to unmarshal JSON: %w", err)
	}

	// Helper function to parse nullAbleTime
	parseNullableTime := func(value interface{}) (nullAbleTime, error) {
		switch v := value.(type) {
		case string:
			if v == "" || v == "null" {
				return nullAbleTime{Time: time.Time{}, Valid: false}, nil
			}
			parsedTime, err := time.Parse("2006-01-02 15:04:05", v)
			if err != nil {
				return nullAbleTime{}, fmt.Errorf("failed to parse time: %w", err)
			}
			return nullAbleTime{Time: parsedTime, Valid: true}, nil
		case bool:
			if !v {
				return nullAbleTime{Time: time.Time{}, Valid: false}, nil
			}
			return nullAbleTime{}, errors.New("unexpected boolean value for time field")
		case nil:
			return nullAbleTime{Time: time.Time{}, Valid: false}, nil
		default:
			return nullAbleTime{}, fmt.Errorf("unexpected type: %T", value)
		}
	}

	// Parse all time fields, including TicketCreatedAt
	var err error
	if t.ReceivedDatetimeSPK, err = parseNullableTime(aux.ReceivedDatetimeSPK); err != nil {
		return fmt.Errorf("error parsing ReceivedDatetimeSPK: %w", err)
	}
	if t.SLADeadline, err = parseNullableTime(aux.SLADeadline); err != nil {
		return fmt.Errorf("error parsing SLADeadline: %w", err)
	}
	if t.CompleteDatetimeWO, err = parseNullableTime(aux.CompleteDatetimeWO); err != nil {
		return fmt.Errorf("error parsing CompleteDatetimeWO: %w", err)
	}
	if t.TicketCreatedAt, err = parseNullableTime(aux.TicketCreatedAt); err != nil {
		return fmt.Errorf("error parsing TicketCreatedAt: %w", err)
	}

	return nil
}

type ExcelColumn struct {
	ColIndex string
	ColTitle string
	ColSize  float64
}

func CheckMrOliverReportAvailability(v *events.Message, userLang string) {
	eventToDo := "Check Report Exists for Mr. Oliver"
	originalSenderJID := NormalizeSenderJID(v.Info.Sender.String())
	stanzaID := v.Info.ID

	if !checkMrOliverReportExistsMutex.TryLock() {
		id := fmt.Sprintf("⚠ Permintaan %s sedang diproses. Mohon tunggu beberapa saat.", eventToDo)
		en := fmt.Sprintf("⚠ Your %s request is being processed. Please wait a moment.", eventToDo)
		sendLangMessageWithStanza(v, stanzaID, originalSenderJID, id, en, userLang)
		return
	}
	defer checkMrOliverReportExistsMutex.Unlock()

	// Inform user we've received request
	id, en := informUserRequestReceived(eventToDo)
	sendLangMessageWithStanza(v, stanzaID, originalSenderJID, id, en, userLang)

	reports, err := StatusReportMrOliver()
	if err != nil {
		id := fmt.Sprintf("⚠ Gagal memuat status report: %v", err)
		en := fmt.Sprintf("⚠ Failed to get report status: %v", err)
		SendLangMessage(originalSenderJID, id, en, userLang)
		return
	}

	if len(reports) > 0 {
		var sbID strings.Builder
		var sbEN strings.Builder

		sbID.WriteString("Berikut informasi mengenai list report ke Mr. Oliver:\n")
		sbEN.WriteString("Here is the report list to Mr. Oliver:\n")

		for _, report := range reports {
			sbID.WriteString(fmt.Sprintf("\n📌 *%v*\n", report.ReportType))
			sbEN.WriteString(fmt.Sprintf("\n📌 *%v*\n", report.ReportType))

			for _, link := range report.Links {
				if report.CanDownload[link] {
					sbID.WriteString(fmt.Sprintf("✅ Bisa diunduh: `%s`\n", link))
					sbEN.WriteString(fmt.Sprintf("✅ Downloadable: `%s`\n", link))
				} else {
					sbID.WriteString(fmt.Sprintf("❌ Tidak bisa diunduh: `%s`\n", link))
					sbEN.WriteString(fmt.Sprintf("❌ Cannot Download: `%s`\n", link))
				}
			}
		}

		msgID := sbID.String()
		msgEN := sbEN.String()

		sendLangMessageWithStanza(v, stanzaID, originalSenderJID, msgID, msgEN, userLang)
		return
	}

	id = "⚠ Tidak ada report!"
	en = "⚠ No Report!"
	SendLangMessage(originalSenderJID, id, en, userLang)
}

func StatusReportMrOliver() ([]MrOliverReport, error) {
	// Handle timezone loading
	loc, err := time.LoadLocation(config.GetConfig().Default.Timezone)
	if err != nil {
		return nil, err
	}

	now := time.Now().In(loc)

	var yesterdayHistoryJOPlannedForTechniciansLink string
	testURLyesterdayHistoryJOPlannedForTechniciansLink := fmt.Sprintf("%v:%v/task_schedule/DAILY_TECHNICIAN_REPORT/file/HistoryJOTechnicians_%v_xx_xx_.xlsx",
		config.GetConfig().Default.OdooDashboardReportingPHPServer,
		config.GetConfig().Default.OdooDashboardReportingPHPPort,
		strings.ToUpper(now.AddDate(0, 0, -1).Format("02_Jan_2006")),
	)

	realURLyesterdayHistoryJOPlannedForTechniciansLink := findRealURL(testURLyesterdayHistoryJOPlannedForTechniciansLink)
	if realURLyesterdayHistoryJOPlannedForTechniciansLink != "" {
		yesterdayHistoryJOPlannedForTechniciansLink = realURLyesterdayHistoryJOPlannedForTechniciansLink
	} else {
		yesterdayHistoryJOPlannedForTechniciansLink = testURLyesterdayHistoryJOPlannedForTechniciansLink
	}

	// oldTemplateEngProd := `%v:%v/task_schedule/ENGINEER_PRODUCTIVITY_OLD/log/%v/_Old_Template__%s_EngineerProductivityReport_%v.xlsx`
	// oldTemplateEngProdAllTaskURL := fmt.Sprintf(oldTemplateEngProd,
	// 	config.GetConfig().Default.OdooDashboardReportingPHPServer,
	// 	config.GetConfig().Default.OdooDashboardReportingPHPPort,
	// 	now.Format("2006-01-02"),
	// 	"All_Task_Type",
	// 	now.Format("02Jan2006"),
	// )

	// oldTemplateEngProdPMOnlyURL := fmt.Sprintf(oldTemplateEngProd,
	// 	config.GetConfig().Default.OdooDashboardReportingPHPServer,
	// 	config.GetConfig().Default.OdooDashboardReportingPHPPort,
	// 	now.Format("2006-01-02"),
	// 	"PM_Only",
	// 	now.Format("02Jan2006"),
	// )

	technicianLoginReportDirURL := fmt.Sprintf("%v:%v/webview_odoo/public/files/%v/",
		config.GetConfig().Default.OdooDashboardReportingPHPServer,
		config.GetConfig().Default.OdooDashboardReportingPHPPort,
		now.Format("2006-01-02"),
	)

	var linkForTechnicianLoginReport string
	latestFileURLTechLoginReport, err := getLatestFile(technicianLoginReportDirURL)
	if err != nil {
		linkForTechnicianLoginReport = err.Error()
	} else {
		linkForTechnicianLoginReport = latestFileURLTechLoginReport
	}

	// Declare report status needed
	reports := []MrOliverReport{
		{
			ReportType: fmt.Sprintf("Technician Login Report @%v", strings.ToUpper(now.Format("02 Jan 2006"))),
			Links: []string{
				linkForTechnicianLoginReport,
			},
			CanDownload: make(map[string]bool),
		},
		{
			ReportType: fmt.Sprintf("History JO Planned %v For Technicians", now.AddDate(0, 0, -1).Format("02 January 2006")),
			Links: []string{
				yesterdayHistoryJOPlannedForTechniciansLink,
			},
			CanDownload: make(map[string]bool),
		},
		// {
		// 	ReportType: fmt.Sprintf("Engineers Productivity Report %v", now.Format("02 JAN 2006")),
		// 	Links: []string{
		// 		fmt.Sprintf("%v:%v/task_schedule/ENGINEER_PRODUCTIVITY/log/%v/_%v_AllTicketType_Report.xlsx",
		// 			config.GetConfig().Default.OdooDashboardReportingPHPServer,
		// 			config.GetConfig().Default.OdooDashboardReportingPHPPort,
		// 			now.Format("2006-01-02"),
		// 			now.Format("02January2006"),
		// 		),
		// 		fmt.Sprintf("%v:%v/task_schedule/ENGINEER_PRODUCTIVITY/log/%v/_%v_PMOnly_Report.xlsx",
		// 			config.GetConfig().Default.OdooDashboardReportingPHPServer,
		// 			config.GetConfig().Default.OdooDashboardReportingPHPPort,
		// 			now.Format("2006-01-02"),
		// 			now.Format("02January2006"),
		// 		),
		// 	},
		// 	CanDownload: make(map[string]bool),
		// },
		// {
		// 	ReportType: fmt.Sprintf("[Old Template] Engineer Productivity %v", now.Format("02 JAN 2006")),
		// 	Links: []string{
		// 		oldTemplateEngProdAllTaskURL,
		// 		oldTemplateEngProdPMOnlyURL,
		// 	},
		// 	CanDownload: make(map[string]bool),
		// },
		{
			ReportType: fmt.Sprintf("SLA Report @%v", time.Now().Format("02 Jan 2006")),
			Links: []string{
				// fmt.Sprintf("%v:%v/report/file/sla_report/%v/(%v)SLAReport_Master.xlsx",
				// 	config.GetConfig().Default.OdooDashboardReportingPHPServer,
				// 	config.GetConfig().Default.OdooDashboardReportingGolangPort,
				// 	now.Format("2006-01-02"),
				// 	now.Format("02Jan2006"),
				// ),
				// fmt.Sprintf("%v:%v/report/file/sla_report/%v/(%v)SLAReport_CM.xlsx",
				// 	config.GetConfig().Default.OdooDashboardReportingPHPServer,
				// 	config.GetConfig().Default.OdooDashboardReportingGolangPort,
				// 	now.Format("2006-01-02"),
				// 	now.Format("02Jan2006"),
				// ),
				fmt.Sprintf("%v:%v/report/file/sla_report/%v/(%v)SLAReport_PM.xlsx",
					strings.ReplaceAll(config.GetConfig().Default.OdooDashboardReportingPHPServer, "https", "http"),
					config.GetConfig().Default.OdooDashboardReportingGolangPort,
					now.Format("2006-01-02"),
					now.Format("02Jan2006"),
				),
				fmt.Sprintf("%v:%v/report/file/sla_report/%v/(%v)SLAReport_NonPM.xlsx",
					strings.ReplaceAll(config.GetConfig().Default.OdooDashboardReportingPHPServer, "https", "http"),
					config.GetConfig().Default.OdooDashboardReportingGolangPort,
					now.Format("2006-01-02"),
					now.Format("02Jan2006"),
				),
				// fmt.Sprintf("%v:%v/report/file/sla_report/%v/(%v)SLAReport_SolvedPending.xlsx",
				// 	config.GetConfig().Default.OdooDashboardReportingPHPServer,
				// 	config.GetConfig().Default.OdooDashboardReportingGolangPort,
				// 	now.Format("2006-01-02"),
				// 	now.Format("02Jan2006"),
				// ),
			},
			CanDownload: make(map[string]bool),
		},
		{
			ReportType: fmt.Sprintf("Artajasa ATM Task Data Report @%v", time.Now().Format("02 Jan 2006")),
			Links: []string{
				fmt.Sprintf("%v:%v/report/file/odoo_atm_task_report/%v/(%v)ArtajasaATMReport_Master.xlsx",
					strings.ReplaceAll(config.GetConfig().Default.OdooDashboardReportingPHPServer, "https", "http"),
					config.GetConfig().Default.OdooDashboardReportingGolangPort,
					now.Format("2006-01-02"),
					now.Format("02Jan2006"),
				),
			},
			CanDownload: make(map[string]bool),
		},
		{
			ReportType: fmt.Sprintf("Engineers Productivity Report @%v", time.Now().Format("02 Jan 2006")),
			Links: []string{
				fmt.Sprintf("%v:%v/report/file/engineers_productivity/%v/(%v)EngineersProductivityReport_Master.xlsx",
					strings.ReplaceAll(config.GetConfig().Default.OdooDashboardReportingPHPServer, "https", "http"),
					config.GetConfig().Default.OdooDashboardReportingGolangPort,
					now.Format("2006-01-02"),
					now.Format("02Jan2006"),
				),
			},
			CanDownload: make(map[string]bool),
		},
	}

	// Check which links are downloadable
	for i := range reports {

		for _, link := range reports[i].Links {
			reports[i].CanDownload[link] = checkLinkAvailability(link)
		}
	}

	return reports, nil
}

func findRealURL(baseURL string) string {
	for hour := 0; hour < 24; hour++ {
		for minute := 0; minute < 60; minute++ {
			realTime := fmt.Sprintf("%02d_%02d", hour, minute)        // Format as HH_MM
			testURL := strings.Replace(baseURL, "xx_xx", realTime, 1) // Replace in the URL

			if checkLinkAvailability(testURL) { // Check if it exists
				return testURL // Return the first valid link
			}
		}
	}
	return "" // Return empty if no valid URL found
}

// checkLinkAvailability checks if a link returns a 200 status
func checkLinkAvailability(url string) bool {
	resp, err := http.Head(url) // HEAD request for faster response
	if err != nil {
		return false
	}
	defer resp.Body.Close()
	return resp.StatusCode == http.StatusOK

}

// getLatestFile finds the most recently modified file
func getLatestFile(directoryURL string) (string, error) {
	files, err := fetchFileList(directoryURL)
	if err != nil {
		return "", err
	}

	if len(files) == 0 {
		return "", fmt.Errorf("no files found in directory: %s", directoryURL)
	}

	var latestFile string
	var latestTime time.Time

	// Check each file's "Last-Modified" header
	for _, file := range files {
		fileURL := directoryURL + file
		modifiedTime, err := getLastModified(fileURL)
		if err != nil {
			fmt.Println("⚠ Error checking:", fileURL, "->", err)
			continue
		}

		// Keep the most recent file
		if modifiedTime.After(latestTime) {
			latestTime = modifiedTime
			latestFile = fileURL
		}
	}

	if latestFile == "" {
		return "", fmt.Errorf("no valid files found in directory: %s", directoryURL)
	}

	return latestFile, nil
}

// getLastModified fetches the "Last-Modified" header for a given file URL
func getLastModified(fileURL string) (time.Time, error) {
	resp, err := http.Head(fileURL) // HEAD request for metadata
	if err != nil {
		return time.Time{}, err
	}
	defer resp.Body.Close()

	lastModifiedStr := resp.Header.Get("Last-Modified")
	if lastModifiedStr == "" {
		return time.Time{}, fmt.Errorf("no Last-Modified header for %s", fileURL)
	}

	lastModified, err := time.Parse(time.RFC1123, lastModifiedStr)
	if err != nil {
		return time.Time{}, err
	}

	return lastModified, nil
}

// fetchFileList gets a list of .xlsx files from the directory URL
func fetchFileList(directoryURL string) ([]string, error) {
	resp, err := http.Get(directoryURL)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("server returned status: %v", resp.Status)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	// Extract file names using regex
	fileRegex := regexp.MustCompile(`href="([^"]+\.xlsx)"`)
	matches := fileRegex.FindAllStringSubmatch(string(body), -1)

	var files []string
	for _, match := range matches {
		files = append(files, match[1])
	}

	return files, nil
}

func ReportAIError(v *events.Message, userLang string) {
	eventToDo := "Generate Report AI Error"
	originalSenderJID := NormalizeSenderJID(v.Info.Sender.String())
	stanzaID := v.Info.ID

	if !generateReportAIErrorMutex.TryLock() {
		id := fmt.Sprintf("⚠ Permintaan %s sedang diproses. Mohon tunggu beberapa saat.", eventToDo)
		en := fmt.Sprintf("⚠ Your %s request is being processed. Please wait a moment.", eventToDo)
		sendLangMessageWithStanza(v, stanzaID, originalSenderJID, id, en, userLang)
		return
	}
	defer generateReportAIErrorMutex.Unlock()

	// Inform user we've received request
	id, en := informUserRequestReceived(eventToDo)
	sendLangMessageWithStanza(v, stanzaID, originalSenderJID, id, en, userLang)

	// Get Eng Prod Data
	err := GetDataTicketEngineersProductivity()
	if err != nil {
		id := fmt.Sprintf("Maaf terjadi kesalahan saat coba untuk tarik data Engineers Productivity: %v", err)
		en := fmt.Sprintf("Sorry, something went wrong while trying to get data Engineers Productivity: %v", err)
		sendLangMessageWithStanza(v, stanzaID, originalSenderJID, id, en, userLang)
		return
	}

	// Create Excel Report
	loc, err := time.LoadLocation(config.GetConfig().Default.Timezone)
	if err != nil {
		id := "⚠ Gagal memuat zona waktu " + config.GetConfig().Default.Timezone
		en := "⚠ Failed to load timezone " + config.GetConfig().Default.Timezone
		sendLangMessageWithStanza(v, stanzaID, originalSenderJID, id, en, userLang)
		return
	}

	now := time.Now().In(loc)
	reportName := fmt.Sprintf("Report_AI_ERROR_%v", now.Format("02Jan2006_15_04_05.xlsx"))
	selectedMainDir, err := fun.FindValidDirectory([]string{
		"web/file/ai_error",
		"../web/file/ai_error",
		"../../web/file/ai_error",
	})
	if err != nil {
		id := fmt.Sprintf("⚠ Mohon maaf, gagal untuk generate report %s : %v", reportName, err)
		en := fmt.Sprintf("⚠ Sorry, failed to generate Report %s : %v", reportName, err)
		sendLangMessageWithStanza(v, stanzaID, originalSenderJID, id, en, userLang)
		return
	}

	fileReportDir := filepath.Join(selectedMainDir, time.Now().Format("2006-01-02"))
	if err := os.MkdirAll(fileReportDir, 0755); err != nil {
		id := fmt.Sprintf("⚠ Mohon maaf, gagal untuk generate report %s : %v", reportName, err)
		en := fmt.Sprintf("⚠ Sorry, failed to generate Report %s : %v", reportName, err)
		sendLangMessageWithStanza(v, stanzaID, originalSenderJID, id, en, userLang)
		return
	}

	excelFilePath, id, en := GenerateExcelForReportAIError(fileReportDir, reportName)
	if id != "" && en != "" {
		sendLangMessageWithStanza(v, stanzaID, originalSenderJID, id, en, userLang)
		return
	}

	var excelCaption string
	idExcelCaption := "Berikut lampiran Excel untuk Report AI Error"
	enExcelCaption := "Here is the Excel attachment for Report AI Error"
	if userLang == "en" {
		excelCaption = enExcelCaption
	} else {
		excelCaption = idExcelCaption
	}
	SendExcelFileWithStanza(v, stanzaID, originalSenderJID, excelFilePath, excelCaption, nil, userLang)

	idText, enText, imgSS := GetImgPivotReportFirstSheet(excelFilePath, userLang)
	if imgSS != "" {
		var imgCaption string
		idImgCaption := "PIVOT Performa AI"
		enImgCaption := "PIVOT AI Performance"
		if userLang == "en" {
			imgCaption = enImgCaption
		} else {
			imgCaption = idImgCaption
		}

		SendImgFileWithStanza(v, stanzaID, originalSenderJID, imgSS, imgCaption, nil, userLang)

		// 🧹 Cleanup: remove the image
		if removeErr := os.Remove(imgSS); removeErr != nil {
			logrus.Errorf("⚠ Gagal menghapus file gambar: %v", removeErr)
		} else {
			logrus.Printf("🧹 Gambar %s berhasil dihapus setelah dikirim.", imgSS)
		}
		// Remove the EXCEL
		if removeErr := os.Remove(excelFilePath); removeErr != nil {
			logrus.Errorf("⚠ Gagal menghapus file Excel: %v", removeErr)
		} else {
			logrus.Printf("🧹 Excel file %s berhasil dihapus setelah digunakan.", excelFilePath)
		}
	} else {
		sendLangMessageWithStanza(v, stanzaID, originalSenderJID, idText, enText, userLang)
	}
}

func GetDataTicketEngineersProductivity() error {
	taskDoing := "Get Data Engineers Productivity"
	if !getDataTicketEngineersProductivityMutex.TryLock() {
		return fmt.Errorf("%s already running, please wait a moment", taskDoing)
	}
	defer getDataTicketEngineersProductivityMutex.Unlock()

	loc, _ := time.LoadLocation(config.GetConfig().Default.Timezone)
	now := time.Now().In(loc)

	startOfMonth := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, loc)
	endOfMonth := time.Date(now.Year(), now.Month()+1, 0, 23, 59, 59, 999999999, loc)
	startOfMonth = startOfMonth.Add(-7 * time.Hour)
	endOfMonth = endOfMonth.Add(-7 * time.Hour)

	startDateParam := startOfMonth.Format("2006-01-02 15:04:05")
	endDateParam := endOfMonth.Format("2006-01-02 15:04:05")

	if config.GetConfig().Report.AIError.ActiveDebug {
		startDateParam = config.GetConfig().Report.AIError.StartParam
		endDateParam = config.GetConfig().Report.AIError.EndParam
	}

	ODOOModel := "helpdesk.ticket"
	allowedCompany := config.GetConfig().ApiODOO.CompanyAllowed
	excludedTechnicians := []string{
		"Tes Dev Mfjr",
	}

	domain := []interface{}{
		[]interface{}{"active", "=", true},
		[]interface{}{"technician_id", "!=", excludedTechnicians},
		[]interface{}{"complete_datetime_wo", ">=", startDateParam},
		[]interface{}{"complete_datetime_wo", "<=", endDateParam},
		[]interface{}{"stage_id", "!=", "Cancel"}, // excluded stage = Cancel so its not calculate in the report
		[]interface{}{"company_id", "=", allowedCompany},
	}

	domainNew := []interface{}{
		[]interface{}{"active", "=", true},
		[]interface{}{"technician_id", "!=", excludedTechnicians},
		[]interface{}{"complete_datetime_wo", "=", false},
		[]interface{}{"create_date", ">=", startDateParam},
		[]interface{}{"create_date", "<=", endDateParam},
		[]interface{}{"stage_id", "=", "New"},
		[]interface{}{"company_id", "=", allowedCompany},
	}

	fieldsID := []string{
		"id",
	}

	fields := []string{
		"id",
		"name",
		"company_id",
		"technician_id",
		"ticket_type_id",
		"x_worksheet_template_id",
		"x_task_type",
		"stage_id",
		"x_reasoncode",
		"x_reason",
		"project_id",
		"x_source",
		"x_merchant",
		"x_merchant_pic",
		"x_studio_alamat",
		"x_studio_kota",
		"x_master_mid",
		"x_master_tid",
		"x_merchant_sn_edc",
		"x_merchant_tipe_edc",
		"fsm_task_count",
		"x_link",
		"x_wo_number",
		"x_wo_number_last",
		"description",
		"x_wo_remark",
		"x_sla_deadline",
		"x_received_datetime_spk",
		"complete_datetime_wo",
		"create_date",
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

	odooNewParams := map[string]interface{}{
		"model":  ODOOModel,
		"domain": domainNew,
		"fields": fieldsID,
		"order":  order,
	}

	payloadNew := map[string]interface{}{
		"jsonrpc": config.GetConfig().ApiODOO.JSONRPC,
		"params":  odooNewParams,
	}

	payloadBytesNew, err := json.Marshal(payloadNew)
	if err != nil {
		return err
	}

	/*
		Complete Datetime WO is Set
	*/
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

	/*
		New Data Left
	*/
	ODOOresponseNew, err := GetODOOMSData(string(payloadBytesNew))
	if err != nil {
		errMsg := fmt.Sprintf("failed fetching data from ODOO MS API: %v", err)
		return errors.New(errMsg)
	}

	ODOOResponseArrayNew, ok := ODOOresponseNew.([]interface{})
	if !ok {
		errMsg := "failed to asset results as []interface{}"
		return errors.New(errMsg)
	}

	ids := extractUniqueIDs(ODOOResponseArray, ODOOResponseArrayNew)

	if len(ids) == 0 {
		return errors.New("empty data in ODOO")
	}

	const batchSize = 1000
	chunks := chunkIdsSlice(ids, batchSize)
	var allRecords []interface{}

	for i, chunk := range chunks {
		logrus.Debugf("Processing Engineers Productivity chunk %d of %d (IDs %v to %v)", i+1, len(chunks), chunk[0], chunk[len(chunk)-1])

		chunkDomain := []interface{}{
			[]interface{}{"id", "=", chunk},
			[]interface{}{"active", "=", true},
			// other filters
		}
		// logrus.Debugf("Chunk domain: %+v", chunkDomain)

		odooParams := map[string]interface{}{
			"model":  ODOOModel,
			"domain": chunkDomain,
			"fields": fields,
			"order":  order,
		}
		// logrus.Debugf("Odoo params: %+v", odooParams)

		payload := map[string]interface{}{
			"jsonrpc": config.GetConfig().ApiODOO.JSONRPC,
			"params":  odooParams,
		}

		payloadBytes, err := json.Marshal(payload)
		if err != nil {
			logrus.Error("Failed to marshal payload: ", err)
		}
		// logrus.Debugf("Payload JSON: %s", string(payloadBytes))

		ODOOresponse, err := GetODOOMSData(string(payloadBytes))
		if err != nil {
			errMsg := fmt.Sprintf("failed fetching data from ODOO MS API: %v", err)
			logrus.Error(errMsg)
		}

		// logrus.Debugf("Raw ODOO response: %+v", ODOOresponse)

		ODOOResponseArray, ok := ODOOresponse.([]interface{})
		if !ok {
			logrus.Debug("Type assertion failed: response is not []interface{}, skipping this chunk")
			continue
		}

		// logrus.Debugf("Appending %d records from this chunk", len(ODOOResponseArray))
		allRecords = append(allRecords, ODOOResponseArray...)
	}

	logrus.Debugf("Finished processing all chunks, total records collected: %d", len(allRecords))

	if len(allRecords) == 0 {
		return errors.New("no data found from ODOO in all chunks")
	}

	ODOOResponseBytes, err := json.Marshal(allRecords)
	if err != nil {
		return fmt.Errorf("failed to marshal combined response: %v", err)
	}

	var listOfData []ODOOEngineersProductivityItem
	if err := json.Unmarshal(ODOOResponseBytes, &listOfData); err != nil {
		errMsg := fmt.Sprintf("failed to unmarshal response body: %v", err)
		return errors.New(errMsg)
	}

	tx := dbWeb.Unscoped().Where("id != ?", 0).Delete(&reportmodel.EngineersProductivityData{})
	if tx.Error != nil {
		return tx.Error
	}
	if tx.RowsAffected == 0 {
		logrus.Debug("No rows were deleted (table might already be empty)")
	}

	for i := 0; i < len(listOfData); i += batchSize {
		end := i + batchSize
		if end > len(listOfData) {
			end = len(listOfData)
		}

		var batch []reportmodel.EngineersProductivityData
		for _, data := range listOfData[i:end] {
			_, technicianName, err := parseJSONIDDataCombined(data.TechnicianId)
			if err != nil {
				logrus.Error(err)
			}

			_, edcType, err := parseJSONIDDataCombined(data.EdcTypeId)
			if err != nil {
				logrus.Error(err)
			}

			_, snEdc, err := parseJSONIDDataCombined(data.SnEdcId)
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

			_, worksheetTemplate, err := parseJSONIDDataCombined(data.WorksheetTemplateId)
			if err != nil {
				logrus.Error(err)
			}

			_, ticketType, err := parseJSONIDDataCombined(data.TicketTypeId)
			if err != nil {
				logrus.Error(err)
			}

			_, project, err := parseJSONIDDataCombined(data.ProjectId)
			if err != nil {
				logrus.Error(err)
			}

			techGroup, err := techGroup(technicianName)
			if err != nil {
				logrus.Error(err)
			}

			ticketSLAStatus, firstTaskDatetime, firstTaskReason, firstTaskMessage := setSLAStatus(data.TaskCount.Int, data.SLADeadline, data.CompleteDatetimeWO, data.WoRemark, data.TaskType)

			firstReasonCode := ""
			ticketReasonCodes := parseReasonCode(data.ReasonCode.String)
			if len(ticketReasonCodes) > 0 {
				firstReasonCode = ticketReasonCodes[len(ticketReasonCodes)-1]
			}

			var slaDeadline, receivedDatetimeSpk, completeDatetimeWo, ticketCreatedAt, firstTaskCompleteDatetime *time.Time
			if data.SLADeadline.Valid {
				slaDeadline = &data.SLADeadline.Time
			}
			if data.ReceivedDatetimeSPK.Valid {
				receivedDatetimeSpk = &data.ReceivedDatetimeSPK.Time
			}
			if data.CompleteDatetimeWO.Valid {
				completeDatetimeWo = &data.CompleteDatetimeWO.Time
			}
			if data.TicketCreatedAt.Valid {
				ticketCreatedAt = &data.TicketCreatedAt.Time
			}

			if !firstTaskDatetime.IsZero() {
				firstTaskCompleteDatetime = &firstTaskDatetime
			}

			batch = append(batch, reportmodel.EngineersProductivityData{
				ID:                        data.ID,
				TicketNumber:              data.TicketNumber,
				Company:                   companyName,
				Technician:                technicianName,
				TechnicianGroup:           techGroup,
				TicketType:                ticketType,
				WorksheetTemplate:         worksheetTemplate,
				TaskType:                  data.TaskType.String,
				Stage:                     stage,
				ReasonCode:                data.ReasonCode.String,
				Reason:                    data.Reason.String,
				Project:                   project,
				Source:                    data.Source.String,
				MerchantName:              data.Merchant.String,
				MerchantPic:               data.MerchantPic.String,
				MerchantAddress:           data.MerchantAddress.String,
				MerchantCity:              data.MerchantCity.String,
				Mid:                       data.Mid.String,
				Tid:                       data.Tid.String,
				SnEdc:                     snEdc,
				EdcType:                   edcType,
				TaskCount:                 data.TaskCount.Int,
				LinkWod:                   data.LinkWod.String,
				WoNumberFirst:             data.WoNumberFirst.String,
				WoNumberLast:              data.WoNumberLast.String,
				SlaStatus:                 ticketSLAStatus,
				SlaExpired:                SLAExpired(data.SLADeadline),
				Description:               data.Description.String,
				WoRemark:                  data.WoRemark.String,
				FirstTaskReasonCode:       firstReasonCode,
				FirstTaskReason:           firstTaskReason,
				FirstTaskMessage:          firstTaskMessage,
				FirstTaskCompleteDatetime: firstTaskCompleteDatetime,
				ReceivedDatetimeSpk:       receivedDatetimeSpk,
				SlaDeadline:               slaDeadline,
				CompleteDateWo:            completeDatetimeWo,
				TicketCreatedAt:           ticketCreatedAt,
			})
		}

		if err := dbWeb.Model(&reportmodel.EngineersProductivityData{}).Create(batch).Error; err != nil {
			errMsg := fmt.Sprintf("failed to insert batch of engineers productivity data to DB: %v", err)
			return errors.New(errMsg)
		}
	}

	return nil
}

// validateAndFixFormulas ensures all formulas in AI Success rows are properly formatted
func validateAndFixFormulas(f *excelize.File, sheetName string, aiSuccessRow, perfAIRow int, columns []ExcelColumn) {
	logrus.Info("Starting formula validation and fix for AI Success metrics")

	fixedCount := 0

	// Fix AI Success Detection row
	if aiSuccessRow > 0 {
		for _, col := range columns[1:] {
			cell := fmt.Sprintf("%s%d", col.ColIndex, aiSuccessRow)
			formula, err := f.GetCellFormula(sheetName, cell)
			if err == nil && strings.Contains(formula, "==") {
				fixedFormula := strings.ReplaceAll(formula, "==", "=")
				f.SetCellFormula(sheetName, cell, fixedFormula)
				fixedCount++
				logrus.Debugf("Fixed AI Success formula in cell %s: %s -> %s", cell, formula, fixedFormula)
			}
		}
	}

	// Fix % Performance AI Success row
	if perfAIRow > 0 {
		for _, col := range columns[1:] {
			cell := fmt.Sprintf("%s%d", col.ColIndex, perfAIRow)
			formula, err := f.GetCellFormula(sheetName, cell)
			if err == nil && strings.Contains(formula, "==") {
				fixedFormula := strings.ReplaceAll(formula, "==", "=")
				f.SetCellFormula(sheetName, cell, fixedFormula)
				fixedCount++
				logrus.Debugf("Fixed Performance AI formula in cell %s: %s -> %s", cell, formula, fixedFormula)
			}
		}
	}

	if fixedCount > 0 {
		logrus.Infof("Formula validation completed: fixed %d cells with '==' syntax", fixedCount)
	} else {
		logrus.Info("Formula validation completed: no '==' syntax issues found")
	}
}

// performFinalFormulaCleanup scans all sheets for any remaining == formula issues
func performFinalFormulaCleanup(f *excelize.File) {
	logrus.Info("Performing final comprehensive formula cleanup")

	totalFixed := 0
	sheetList := f.GetSheetList()

	for _, sheetName := range sheetList {
		sheetFixed := 0

		// Get all cells in the sheet
		rows, err := f.GetRows(sheetName)
		if err != nil {
			continue
		}

		for rowIndex, row := range rows {
			for colIndex := range row {
				cellRef, _ := excelize.CoordinatesToCellName(colIndex+1, rowIndex+1)

				// Check if cell has a formula
				formula, err := f.GetCellFormula(sheetName, cellRef)
				if err != nil || formula == "" {
					continue
				}

				// Fix any == issues in formulas
				if strings.Contains(formula, "==") {
					fixedFormula := strings.ReplaceAll(formula, "==", "=")

					// Additional cleanup: ensure proper formula syntax
					fixedFormula = strings.ReplaceAll(fixedFormula, "===", "=")
					fixedFormula = strings.ReplaceAll(fixedFormula, "====", "=")

					err := f.SetCellFormula(sheetName, cellRef, fixedFormula)
					if err == nil {
						sheetFixed++
						totalFixed++
						logrus.Debugf("Final cleanup fixed formula in %s!%s: %s -> %s",
							sheetName, cellRef, formula, fixedFormula)
					}
				}
			}
		}

		if sheetFixed > 0 {
			logrus.Infof("Sheet '%s': fixed %d formula syntax issues", sheetName, sheetFixed)
		}
	}

	if totalFixed > 0 {
		logrus.Infof("Final formula cleanup completed: fixed %d total cells across all sheets", totalFixed)
	} else {
		logrus.Info("Final formula cleanup completed: no issues found")
	}
}

func GenerateExcelForReportAIError(fileReportDir, reportName string) (string, string, string) {
	f := excelize.NewFile()

	sheetMaster := "MASTER_AI_ERROR"
	sheetPvtErr := "PIVOT_AI_ERROR"
	sheetSumErr := "SUM_ERR"
	sheetEngProd := "ENG_PROD"
	sheetPvtProd := "PIVOT_ENG_PROD"
	sheetRate := "RATE"
	sheetEmployee := "EMPLOYEES"

	// sheetERRAI := "ERR_AI"
	// sheetERRAI2 := "ERR2_AI"
	// sheetRateAll := "RATE_ALL"
	sheetImprovAI := "IMPROVMENT_AI"

	_, err := f.NewSheet(sheetMaster)
	if err != nil {
		return "", fmt.Sprintf("Gagal membuat sheet %s: %v", sheetMaster, err), fmt.Sprintf("Failed to create sheet %s: %v", sheetMaster, err)
	}

	f.NewSheet(sheetPvtErr)
	f.NewSheet(sheetSumErr)
	f.NewSheet(sheetEngProd)
	f.NewSheet(sheetPvtProd)
	f.NewSheet(sheetRate)
	f.NewSheet(sheetEmployee)

	// f.NewSheet(sheetERRAI)
	// f.NewSheet(sheetERRAI2)
	// f.NewSheet(sheetRateAll)
	f.NewSheet(sheetImprovAI)

	// (Optional) Delete default "Sheet1"
	f.DeleteSheet("Sheet1")

	/* Styles */
	styleMasterTitle, err := f.NewStyle(&excelize.Style{
		Alignment: &excelize.Alignment{
			Horizontal: "center",
			Vertical:   "center",
		},
		Fill: excelize.Fill{
			Type:    "pattern",
			Color:   []string{"#0070C0"},
			Pattern: 1,
		},
		Font: &excelize.Font{
			Bold:  true,
			Color: "#FFFFFF",
		},
	})
	if err != nil {
		return "", fmt.Sprintf("Gagal membuat style untuk judul Master: %v", err), fmt.Sprintf("Failed to create style for Master title: %v", err)
	}

	style, _ := f.NewStyle(&excelize.Style{
		Alignment: &excelize.Alignment{
			Horizontal: "center",
			Vertical:   "center",
		},
	})

	styleEngProdTitle, err := f.NewStyle(&excelize.Style{
		Alignment: &excelize.Alignment{
			Horizontal: "center",
			Vertical:   "center",
		},
		Fill: excelize.Fill{
			Type:    "pattern",
			Color:   []string{"#11A80E"},
			Pattern: 1,
		},
		Font: &excelize.Font{
			Bold:  true,
			Color: "#FFFFFF",
		},
	})
	if err != nil {
		return "", fmt.Sprintf("Gagal membuat style untuk judul Engineers Productivity: %v", err), fmt.Sprintf("Failed to create style for Engineers Productivity title: %v", err)
	}

	stylePvtAIErr, err := f.NewStyle(&excelize.Style{
		Alignment: &excelize.Alignment{
			Horizontal: "center",
			Vertical:   "center",
		},

		Fill: excelize.Fill{
			Type:    "pattern",
			Color:   []string{"#ff2200"},
			Pattern: 1,
		},
		Font: &excelize.Font{
			Bold:  true,
			Color: "#FFFFFF",
		},
	})
	if err != nil {
		return "", fmt.Sprintf("Gagal membuat style untuk judul Pivot AI Error: %v", err), fmt.Sprintf("Failed to create style for Pivot AI Error title: %v", err)
	}

	stylePvtProd, err := f.NewStyle(&excelize.Style{
		Alignment: &excelize.Alignment{
			Horizontal: "center",
			Vertical:   "center",
		},
		Fill: excelize.Fill{
			Type:    "pattern",
			Color:   []string{"#851707"},
			Pattern: 1,
		},
		Font: &excelize.Font{
			Bold:  true,
			Color: "#FFFFFF",
		},
	})
	if err != nil {
		return "", fmt.Sprintf("Gagal membuat style untuk judul Pivot Engineers Productivity: %v", err), fmt.Sprintf("Failed to create style for Pivot Engineers Productivity title: %v", err)
	}

	styleRateTitle, err := f.NewStyle(&excelize.Style{
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
		return "", fmt.Sprintf("Gagal membuat style untuk judul Rate: %v", err), fmt.Sprintf("Failed to create style for Rate title: %v", err)
	}

	styleGrandTotal, _ := f.NewStyle(&excelize.Style{
		Alignment: &excelize.Alignment{Horizontal: "center", Vertical: "center"},
		Fill:      excelize.Fill{Type: "pattern", Color: []string{"#00FF00"}, Pattern: 1},
		Font:      &excelize.Font{Bold: true, Color: "#000000"},
	})
	stylePercentage, _ := f.NewStyle(&excelize.Style{
		Alignment: &excelize.Alignment{Horizontal: "center", Vertical: "center"},
		Font:      &excelize.Font{Bold: true, Color: "#000000"},
	})
	styleTotalTA, _ := f.NewStyle(&excelize.Style{
		Alignment: &excelize.Alignment{
			Horizontal: "center",
			Vertical:   "center",
		},
		Fill: excelize.Fill{
			Type:    "pattern",
			Color:   []string{"#D9D9D9"}, // light gray background
			Pattern: 1,
		},
		Font: &excelize.Font{
			Bold:  true,
			Color: "#000000",
		},
		Border: []excelize.Border{
			{Type: "left", Color: "000000", Style: 1},
			{Type: "top", Color: "000000", Style: 1},
			{Type: "bottom", Color: "000000", Style: 1},
			{Type: "right", Color: "000000", Style: 1},
		},
	})

	// Get TA Data
	dbDataTA := gormdb.Databases.TA
	if dbDataTA == nil {
		return "", "Gagal mendapatkan koneksi ke database TA", "Failed to get connection to TA database"
	}

	dbWebTA := gormdb.Databases.WebTA
	if dbWebTA == nil {
		return "", "Gagal mendapatkan koneksi ke database Web TA", "Failed to get connection to Web TA database"
	}

	/*
		EMPLOYEES
	*/
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
	for _, column := range columnsEmployee {
		f.SetCellValue(sheetEmployee, fmt.Sprintf("%s1", column.ColIndex), column.ColTitle)
		f.SetColWidth(sheetEmployee, column.ColIndex, column.ColIndex, column.ColSize)
	}

	lastColEmployee := fun.GetColName(len(columnsEmployee) - 1)
	filterRangeEmployee := fmt.Sprintf("A1:%s1", lastColEmployee)
	f.AutoFilter(sheetEmployee, filterRangeEmployee, []excelize.AutoFilterOptions{})

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
		"jsonrpc": config.GetConfig().ApiODOO.JSONRPC,
		"params":  odooParams,
	}
	payloadBytes, err := json.Marshal(payload)
	if err != nil {
		return "", fmt.Sprintf("Gagal membuat payload JSON: %v", err), fmt.Sprintf("Failed to create JSON payload: %v", err)
	}
	ODOOresponse, err := GetODOOMSData(string(payloadBytes))
	if err != nil {
		return "", fmt.Sprintf("Gagal mendapatkan data dari ODOO: %v", err), fmt.Sprintf("Failed to get data from ODOO: %v", err)
	}
	ODOOResponseArray, ok := ODOOresponse.([]interface{})
	if !ok {
		return "", "Gagal mengkonversi data ODOO ke array", "Failed to convert ODOO data to array"
	}

	ODOOResponseBytes, err := json.Marshal(ODOOResponseArray)
	if err != nil {
		return "", fmt.Sprintf("Gagal mengkonversi data ODOO ke JSON: %v", err), fmt.Sprintf("Failed to convert ODOO data to JSON: %v", err)
	}

	var employeeData []ODOOMSTechnicianItem
	if err := json.Unmarshal(ODOOResponseBytes, &employeeData); err != nil {
		return "", fmt.Sprintf("Gagal meng-unmarshal data ODOO: %v", err), fmt.Sprintf("Failed to unmarshal ODOO data: %v", err)
	}

	if len(employeeData) == 0 {
		logrus.Warn("No technician data found in ODOO")
		// return "", "⚠ Tidak ada data teknisi ditemukan di ODOO", "⚠ No technician data found in ODOO"
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
			case "Ops Head":
				if record.Head.String != "" {
					value = record.Head.String
				}
				f.SetCellValue(sheetEmployee, cell, value)
			}
		}
		employeeRowIndex++ // increment once per record (row), not per column
	}

	/*
	**************************************************************************************
	 */

	/*
		MASTER
	*/
	titlesMaster := []struct {
		Title string
		Size  float64
	}{
		{"Start Followed Up at", 25},
		{"End of Followed Up", 25},
		{"Followed Up (Time)", 25},
		{"Date in Dashboard", 25},
		{"Date in Dashboard (dd)", 35},
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
	var columnsMaster []ExcelColumn
	for i, t := range titlesMaster {
		columnsMaster = append(columnsMaster, ExcelColumn{
			ColIndex: fun.GetColName(i),
			ColTitle: t.Title,
			ColSize:  t.Size,
		})
	}

	for _, col := range columnsMaster {
		cell := fmt.Sprintf("%s1", col.ColIndex)
		f.SetCellValue(sheetMaster, cell, col.ColTitle)
		f.SetColWidth(sheetMaster, col.ColIndex, col.ColIndex, col.ColSize)
		f.SetCellStyle(sheetMaster, cell, cell, styleMasterTitle)
	}
	// Set autofilter for the first row
	lastColMaster := fun.GetColName(len(columnsMaster) - 1)
	filterRangeMaster := fmt.Sprintf("A1:%s1", lastColMaster)
	f.AutoFilter(sheetMaster, filterRangeMaster, []excelize.AutoFilterOptions{})

	location, err := time.LoadLocation(config.GetConfig().Default.Timezone)
	if err != nil {
		return "",
			fmt.Sprintf("⚠ Gagal memuat zona %s : %v", config.GetConfig().Default.Timezone, err),
			fmt.Sprintf("⚠ Failed to load timezone %s : %v", config.GetConfig().Default.Timezone, err)
	}

	now := time.Now().In(location)
	// Set startOfDay to the 1st day of the current month at 00:00:00
	startOfDay := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, now.Location()).Add(-7 * time.Hour)
	// Set endOfDay to the last day of the current month at 23:59:59
	endOfDay := time.Date(now.Year(), now.Month()+1, 0, 23, 59, 59, 0, now.Location()).Add(-7 * time.Hour)

	year, month, _ := now.Date()

	// Find number of days in the month
	firstOfMonth := time.Date(year, month, 1, 0, 0, 0, 0, location)
	lastOfMonth := firstOfMonth.AddDate(0, 1, -1)
	numDays := lastOfMonth.Day()

	var taActivityData []tamodel.LogAct
	if err := dbDataTA.Model(&tamodel.LogAct{}).
		Where("date_in_dashboard BETWEEN ? AND ? AND LOWER(method) = ?", startOfDay, endOfDay, "submit").
		Where("LOWER(type_case) = ?", "error").
		Where("problem IS NOT NULL").
		Order("date_in_dashboard ASC").
		Find(&taActivityData).Error; err != nil {
		return "", fmt.Sprintf("Gagal mendapatkan data aktivitas TA: %v", err), fmt.Sprintf("Failed to get TA activity data: %v", err)
	}

	if len(taActivityData) == 0 {
		return "", "⚠ Tidak ada data aktivitas TA ditemukan", "⚠ No TA activity data found"
	}

	rowIndex := 2
	UserTA := config.GetConfig().UserTA

	for _, record := range taActivityData {
		for _, column := range columnsMaster {
			cell := fmt.Sprintf("%s%d", column.ColIndex, rowIndex)
			var value interface{} = "N/A"

			var needToSetValue bool = true

			switch column.ColTitle {
			case "TA":
				if ta, ok := UserTA[record.Email]; ok {
					value = ta.Name
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
				formula := fmt.Sprintf(`IFERROR(VLOOKUP(H%d, %v!A:C, 2, FALSE), "N/A")`, rowIndex, sheetEmployee)
				f.SetCellFormula(sheetMaster, cell, formula)
			case "Head":
				needToSetValue = false
				formula := fmt.Sprintf(`IFERROR(VLOOKUP(H%d, %v!A:C, 3, FALSE), "N/A")`, rowIndex, sheetEmployee)
				f.SetCellFormula(sheetMaster, cell, formula)
			case "WO Number":
				if record.Wo != nil && *record.Wo != "" && *record.Wo != "0" {
					wo := *record.Wo
					link := fmt.Sprintf("http://smartwebindonesia.com:3405/projectTask/detailWO?wo_number=%s", wo)
					f.SetCellHyperLink(sheetMaster, cell, link, "External")
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
						f.SetCellValue(sheetMaster, cell, value)
						f.SetCellStyle(sheetMaster, cell, cell, styleID)
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
			case "Date in Dashboard (dd)":
				if record.DateInDashboard == "" {
					value = "N/A"
				} else {
					parsedTime, err := time.Parse("2006-01-02 15:04:05", record.DateInDashboard)
					if err != nil {
						value = "N/A"
					} else {
						value = parsedTime.Format("02")
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
				f.SetCellValue(sheetMaster, cell, value)
				f.SetCellStyle(sheetMaster, cell, cell, style)
			}

		}
		rowIndex++
	}

	pivotMasterDataRange := fmt.Sprintf("%s!$A$1:$%s$%d", sheetMaster, lastColMaster, rowIndex-1)
	pivotSheetPvtErrRange := fmt.Sprintf("%s!A8:AI200", sheetPvtErr)
	err = f.AddPivotTable(&excelize.PivotTableOptions{
		Name:            sheetPvtErr,
		DataRange:       pivotMasterDataRange,
		PivotTableRange: pivotSheetPvtErrRange,
		Rows: []excelize.PivotTableField{
			{Data: "Problem"},
		},
		Columns: []excelize.PivotTableField{
			{Data: "Date in Dashboard (dd)"},
		},
		Data: []excelize.PivotTableField{
			{Data: "Activity", Subtotal: "count"},
		},
		Filter: []excelize.PivotTableField{
			{Data: "Head"},
			{Data: "SPL"},
			{Data: "Technician"},
			{Data: "Type"},
		},
		RowGrandTotals:      true,
		ColGrandTotals:      true,
		ShowDrill:           true,
		ShowRowHeaders:      true,
		ShowColHeaders:      true,
		ShowLastColumn:      true,
		PivotTableStyleName: "PivotStyleLight10", // Set your desired style here
	})
	if err != nil {
		return "", fmt.Sprintf("Gagal membuat Pivot Table untuk %s: %v", sheetPvtErr, err), fmt.Sprintf("Failed to create Pivot Table for %s: %v", sheetPvtErr, err)
	}
	f.SetColWidth(sheetPvtErr, "A", "A", 35)
	f.SetCellValue(sheetPvtErr, "A1", "Major issues from Technician")
	f.SetCellStyle(sheetPvtErr, "A1", "A1", stylePvtAIErr)

	// Summary ERROR
	// Start with the first static column
	titleProblem := []struct {
		Title string
		Size  float64
	}{
		{"Problem", 55},
	}
	// Add date columns in the middle
	for day := 1; day <= numDays; day++ {
		date := time.Date(year, month, day, 0, 0, 0, 0, location)
		dateStr := date.Format("02")
		titleProblem = append(titleProblem, struct {
			Title string
			Size  float64
		}{
			Title: dateStr,
			Size:  15, // adjust as needed
		})
	}
	// Add the last static columns
	titleProblem = append(titleProblem,
		struct {
			Title string
			Size  float64
		}{"Grand Total", 20},
		struct {
			Title string
			Size  float64
		}{"%", 20},
	)

	// Convert to ExcelColumn
	var columnsSumErr []ExcelColumn
	for i, t := range titleProblem {
		columnsSumErr = append(columnsSumErr, ExcelColumn{
			ColIndex: fun.GetColName(i),
			ColTitle: t.Title,
			ColSize:  t.Size,
		})
	}

	for _, column := range columnsSumErr {
		cell := fmt.Sprintf("%s1", column.ColIndex)
		f.SetCellValue(sheetSumErr, cell, column.ColTitle)
		f.SetColWidth(sheetSumErr, column.ColIndex, column.ColIndex, column.ColSize)

		switch column.ColTitle {
		case "Problem":
			styleNew, _ := f.NewStyle(&excelize.Style{
				Alignment: &excelize.Alignment{
					Horizontal: "center",
					Vertical:   "center",
				},
				Fill: excelize.Fill{
					Type:    "pattern",
					Color:   []string{"#FF0000"}, // red
					Pattern: 1,
				},
				Font: &excelize.Font{
					Bold:  true,
					Color: "#FFFFFF",
				},
			})
			f.SetCellStyle(sheetSumErr, cell, cell, styleNew)
		case "Grand Total":
			styleGrandTotal, _ := f.NewStyle(&excelize.Style{
				Alignment: &excelize.Alignment{
					Horizontal: "center",
					Vertical:   "center",
				},
				Fill: excelize.Fill{
					Type:    "pattern",
					Color:   []string{"#00FF00"}, // green
					Pattern: 1,
				},
				Font: &excelize.Font{
					Bold:  true,
					Color: "#000000",
				},
			})
			f.SetCellStyle(sheetSumErr, cell, cell, styleGrandTotal)
		}
	}

	lastColSumErr := fun.GetColName(len(columnsSumErr) - 1)
	filterRangeSumErr := fmt.Sprintf("A1:%s1", lastColSumErr)
	f.AutoFilter(sheetSumErr, filterRangeSumErr, []excelize.AutoFilterOptions{})

	// Freeze col A
	f.SetPanes(sheetSumErr, &excelize.Panes{
		Freeze:      true,
		Split:       true,
		XSplit:      1, // freeze first column (A)
		YSplit:      1, // freeze first row (header)
		TopLeftCell: "B2",
		ActivePane:  "bottomRight",
	})

	var techProblems []string
	err = dbDataTA.Model(&tamodel.LogAct{}).
		Where("date_in_dashboard BETWEEN ? AND ? AND LOWER(method) = ?", startOfDay, endOfDay, "submit").
		Where("LOWER(type_case) = ?", "error").
		Where("problem IS NOT NULL").
		Distinct("problem").
		Order("problem ASC").
		Pluck("problem", &techProblems).Error
	if err != nil {
		return "", fmt.Sprintf("Gagal mendapatkan daftar masalah teknisi: %v", err), fmt.Sprintf("Failed to get technician problems list: %v", err)
	}

	if len(techProblems) == 0 {
		return "", "⚠ Tidak ada masalah teknisi ditemukan", "⚠ No technician problems found"
	}

	rowIndex = 2
	for _, problem := range techProblems {
		f.SetCellValue(sheetSumErr, fmt.Sprintf("A%d", rowIndex), problem)

		var grandTotalCount int64

		for _, col := range columnsSumErr[1:] {
			cell := fmt.Sprintf("%s%d", col.ColIndex, rowIndex)

			var count int64

			switch col.ColTitle {
			case "Grand Total":
				// Count total occurrences of the problem across full period
				err := dbDataTA.Model(&tamodel.LogAct{}).
					Where("date_in_dashboard BETWEEN ? AND ? AND LOWER(method) = ?", startOfDay, endOfDay, "submit").
					Where("LOWER(type_case) = ?", "error").
					Where("problem = ?", problem).
					Count(&grandTotalCount).Error
				if err != nil {
					logrus.Errorf("Failed to count grand total for problem %s: %v", problem, err)
					grandTotalCount = 0
				}
				f.SetCellValue(sheetSumErr, cell, grandTotalCount)
				f.SetCellStyle(sheetSumErr, cell, cell, styleGrandTotal)

			case "%":
				// Count total problems for percentage base
				var totalCount int64
				err := dbDataTA.Model(&tamodel.LogAct{}).
					Where("date_in_dashboard BETWEEN ? AND ? AND LOWER(method) = ?", startOfDay, endOfDay, "submit").
					Where("LOWER(type_case) = ?", "error").
					Count(&totalCount).Error
				if err != nil {
					logrus.Errorf("Failed to count total problems: %v", err)
					f.SetCellValue(sheetSumErr, cell, "0.00%")
				} else {
					var percent float64 = 0
					if totalCount > 0 {
						percent = float64(grandTotalCount) / float64(totalCount) * 100
					}
					f.SetCellValue(sheetSumErr, cell, fmt.Sprintf("%.2f%%", percent))
				}
				f.SetCellStyle(sheetSumErr, cell, cell, stylePercentage)

			default:
				// col.ColTitle is day as string "01", "02", etc.
				dayNum, err := strconv.Atoi(col.ColTitle)
				if err != nil {
					logrus.Errorf("Failed to parse day %s: %v", col.ColTitle, err)
					count = 0
				} else {
					date := time.Date(year, month, dayNum, 0, 0, 0, 0, location)
					dayStart := date
					dayEnd := date.Add(24*time.Hour - time.Nanosecond)

					err := dbDataTA.Model(&tamodel.LogAct{}).
						Where("date_in_dashboard BETWEEN ? AND ? AND LOWER(method) = ?", dayStart, dayEnd, "submit").
						Where("LOWER(type_case) = ?", "error").
						Where("problem = ?", problem).
						Count(&count).Error
					if err != nil {
						logrus.Errorf("Failed to count problem %s on day %s: %v", problem, col.ColTitle, err)
						count = 0
					}
				}
				f.SetCellValue(sheetSumErr, cell, count)
				f.SetCellStyle(sheetSumErr, cell, cell, style) // your default style
			}
		}

		rowIndex++
	}
	rowIndex++

	// Set TA StandBy
	colIndexTA := 1
	f.SetCellValue(sheetSumErr, fmt.Sprintf("A%d", rowIndex), "Total TA StandBy")
	for _, col := range columnsSumErr[1:] { // skip first column "Problem"
		cell := fmt.Sprintf("%s%d", col.ColIndex, rowIndex)

		switch col.ColTitle {
		default:
			if col.ColTitle == "Grand Total" || col.ColTitle == "%" {
				continue // skip these columns
			}

			dayNum, err := strconv.Atoi(col.ColTitle)
			if err != nil {
				logrus.Errorf("Failed to parse day %s: %v", col.ColTitle, err)
				f.SetCellValue(sheetSumErr, cell, 0)
				continue
			}

			date := time.Date(year, month, dayNum, 0, 0, 0, 0, location)
			dayStart := date
			dayEnd := date.Add(24*time.Hour - time.Nanosecond)

			var totalTAStandBy int64 = 0
			var TAHandledData tamodel.TAHandledData
			err = dbWebTA.Model(&tamodel.TAHandledData{}).
				Where("updated_at BETWEEN ? AND ?", dayStart, dayEnd).
				First(&TAHandledData).Error
			if err != nil {
				logrus.Errorf("Failed to count TA StandBy on %s: %v", date.Format("2006-01-02"), err)
				totalTAStandBy = 0
			}
			totalTAStandBy = int64(TAHandledData.TotalTAStandBy)

			f.SetCellValue(sheetSumErr, cell, totalTAStandBy)
		}

		colIndexTA++
	}
	startCellTAStandBy := fmt.Sprintf("A%d", rowIndex)
	endCellTAStandBy := fmt.Sprintf("%s%d", columnsSumErr[len(columnsSumErr)-3].ColIndex, rowIndex)
	if err := f.SetCellStyle(sheetSumErr, startCellTAStandBy, endCellTAStandBy, styleTotalTA); err != nil {
		logrus.Errorf("Failed to apply style to Total TA StandBy row: %v", err)
	}

	// Chart
	// Build chart after filling data
	rowIndex++ // move after last data row

	startCol := columnsSumErr[1].ColIndex                  // first day column, e.g. "B"
	endCol := columnsSumErr[len(columnsSumErr)-3].ColIndex // last day column, skip "Grand Total" & "%"

	// Categories: day numbers from header row (row 1)
	categoriesRange := fmt.Sprintf("%s!$%s$1:$%s$1", sheetSumErr, startCol, endCol)

	var seriesSumErr []excelize.ChartSeries
	for row := 2; row < rowIndex; row++ { // loop actual data rows
		cell, err := f.GetCellValue(sheetSumErr, fmt.Sprintf("A%d", row))
		if err != nil {
			return "", fmt.Sprintf("Gagal mendapatkan nilai sel A%d: %v", row, err), fmt.Sprintf("Failed to get cell value A%d: %v", row, err)
		}
		if cell == "" {
			continue // skip empty rows
		}

		// Create series: one per problem
		seriesSumErr = append(seriesSumErr, excelize.ChartSeries{
			Name:       fmt.Sprintf("%s!$A$%d", sheetSumErr, row),
			Categories: categoriesRange,
			Values:     fmt.Sprintf("%s!$%s$%d:$%s$%d", sheetSumErr, startCol, row, endCol, row),
		})
	}

	// Define chart
	chartSumErr := &excelize.Chart{
		Type: excelize.Col3DClustered,
		Dimension: excelize.ChartDimension{
			Width:  3000,
			Height: 800,
		},
		Title: []excelize.RichTextRun{
			{Text: "Major Error AI"},
		},
		Legend: excelize.ChartLegend{
			ShowLegendKey: true,
			Position:      "left",
		},
		PlotArea: excelize.ChartPlotArea{
			ShowDataTable:     false,
			ShowDataTableKeys: false,
		},
		Series: seriesSumErr,
	}

	// Place chart below data
	chartCell := fmt.Sprintf("A%d", rowIndex+2)
	if err := f.AddChart(sheetSumErr, chartCell, chartSumErr); err != nil {
		return "", fmt.Sprintf("Gagal menambahkan chart ke %s: %v", sheetSumErr, err), fmt.Sprintf("Failed to add chart to %s: %v", sheetSumErr, err)
	}

	/*
	**************************************************************************************
	 */

	/*
		Engineers Productivity
	*/

	titlesEngProd := []struct {
		Title string
		Size  float64
	}{
		{"Ops Head", 20},
		{"SPL", 20},
		{"Technician Group", 20},
		{"Technician", 25},
		{"Ticket Number", 50},
		{"Stage", 30},
		{"Company", 20},
		{"Project", 20},
		{"Source", 20},
		{"Ticket Type", 20},
		{"Worksheet Template", 20},
		{"Task Type", 20},
		{"Ticket Created at", 20},
		{"Received SPK at", 20},
		{"SLA Deadline", 20},
		{"Complete WO", 20},
		{"Complete WO (dd)", 20},
		{"SLA Status", 20},
		{"SLA Expired", 25},
		{"MID", 30},
		{"TID", 30},
		{"Merchant", 40},
		{"Merchant PIC", 30},
		{"Merchant Address", 50},
		{"Merchant City", 25},
		{"Task Count", 18},
		{"WO Remark", 50},
		{"Reason Code", 20},
		{"Reason", 20},
		{"First JO Complete Datetime", 35},
		{"First JO Reason Code", 30},
		{"First JO Reason", 30},
		{"First JO Message", 40},
		{"Link WO", 40},
		{"WO Number First", 30},
		{"WO Number Last", 30},
		{"EDC Type", 30},
		{"EDC Serial", 30},
		{"Description", 70},
	}

	var columnsEngProd []ExcelColumn
	for i, t := range titlesEngProd {
		columnsEngProd = append(columnsEngProd, ExcelColumn{
			ColIndex: fun.GetColName(i),
			ColTitle: t.Title,
			ColSize:  t.Size,
		})
	}

	for _, col := range columnsEngProd {
		cell := fmt.Sprintf("%s1", col.ColIndex)
		f.SetCellValue(sheetEngProd, cell, col.ColTitle)
		f.SetColWidth(sheetEngProd, col.ColIndex, col.ColIndex, col.ColSize)
		f.SetCellStyle(sheetEngProd, cell, cell, styleEngProdTitle)
	}
	// Set autofilter for the first row
	lastColEngProd := fun.GetColName(len(columnsEngProd) - 1)
	filterRangeEngProd := fmt.Sprintf("A1:%s1", lastColEngProd)
	f.AutoFilter(sheetEngProd, filterRangeEngProd, []excelize.AutoFilterOptions{})

	const batchSize = 1000
	var offset int
	rowIndex = 2

	for {
		var dataBatch []reportmodel.EngineersProductivityData
		if err := dbWeb.Model(&reportmodel.EngineersProductivityData{}).
			Where("1=1").
			Order("complete_date_wo ASC").
			Offset(offset).
			Limit(batchSize).
			Find(&dataBatch).Error; err != nil {
			return "", fmt.Sprintf("Gagal mengambil data Engineers Productivity: %v", err), fmt.Sprintf("Failed to fetch Engineers Productivity data: %v", err)
		}

		if len(dataBatch) == 0 {
			break
		}

		for _, record := range dataBatch {
			for _, column := range columnsEngProd {
				cell := fmt.Sprintf("%s%d", column.ColIndex, rowIndex)
				var value interface{}
				switch column.ColTitle {
				case "Technician Group":
					if derefString(&record.TechnicianGroup) == "" {
						value = "N/A"
					} else {
						value = derefString(&record.TechnicianGroup)
					}
					f.SetCellValue(sheetEngProd, cell, value)
				case "Technician":
					if derefString(&record.Technician) == "" {
						value = "N/A"
					} else {
						value = derefString(&record.Technician)
					}
					f.SetCellValue(sheetEngProd, cell, value)
				case "Ops Head":
					// Lookup column 3 (Ops Head) in EMPLOYEES based on Technician (from column D)
					formula := fmt.Sprintf(`IFERROR(VLOOKUP(D%d, %v!A:C, 3, FALSE), "N/A")`, rowIndex, sheetEmployee)
					f.SetCellFormula(sheetEngProd, cell, formula)
				case "SPL":
					// Lookup column 2 (SPL) in EMPLOYEES based on Technician (from column D)
					formula := fmt.Sprintf(`IFERROR(VLOOKUP(D%d, %v!A:C, 2, FALSE), "N/A")`, rowIndex, sheetEmployee)
					f.SetCellFormula(sheetEngProd, cell, formula)
				case "Ticket Number":
					if derefString(&record.TicketNumber) == "" {
						value = "N/A"
					} else {
						value = derefString(&record.TicketNumber)
					}
					f.SetCellValue(sheetEngProd, cell, value)
				case "Stage":
					if derefString(&record.Stage) == "" {
						value = "N/A"
					} else {
						value = derefString(&record.Stage)
					}
					f.SetCellValue(sheetEngProd, cell, value)
				case "Company":
					if derefString(&record.Company) == "" {
						value = "N/A"
					} else {
						value = derefString(&record.Company)
					}
					f.SetCellValue(sheetEngProd, cell, value)
				case "Project":
					if derefString(&record.Project) == "" {
						value = "N/A"
					} else {
						value = derefString(&record.Project)
					}
					f.SetCellValue(sheetEngProd, cell, value)
				case "Source":
					if derefString(&record.Source) == "" {
						value = "N/A"
					} else {
						value = derefString(&record.Source)
					}
					f.SetCellValue(sheetEngProd, cell, value)
				case "Ticket Type":
					if derefString(&record.TicketType) == "" {
						value = "N/A"
					} else {
						value = derefString(&record.TicketType)
					}
					f.SetCellValue(sheetEngProd, cell, value)
				case "Worksheet Template":
					if derefString(&record.WorksheetTemplate) == "" {
						value = "N/A"
					} else {
						value = derefString(&record.WorksheetTemplate)
					}
					f.SetCellValue(sheetEngProd, cell, value)
				case "Task Type":
					if derefString(&record.TaskType) == "" {
						value = "N/A"
					} else {
						value = derefString(&record.TaskType)
					}
					f.SetCellValue(sheetEngProd, cell, value)
				case "Ticket Created at":
					if derefTime(record.TicketCreatedAt) == "" {
						value = "N/A"
					} else {
						value = derefTime(record.TicketCreatedAt)
					}
					f.SetCellValue(sheetEngProd, cell, value)
				case "Received SPK at":
					if derefTime(record.ReceivedDatetimeSpk) == "" {
						value = "N/A"
					} else {
						value = derefTime(record.ReceivedDatetimeSpk)
					}
					f.SetCellValue(sheetEngProd, cell, value)
				case "SLA Deadline":
					if derefTime(record.SlaDeadline) == "" {
						value = "N/A"
					} else {
						value = derefTime(record.SlaDeadline)
					}
					f.SetCellValue(sheetEngProd, cell, value)
				case "Complete WO":
					if derefDate(record.CompleteDateWo) == "" {
						value = "New"
					} else {
						value = derefDate(record.CompleteDateWo)
					}
					f.SetCellValue(sheetEngProd, cell, value)
				case "Complete WO (dd)":
					if derefDate(record.CompleteDateWo) == "" {
						value = "New"
					} else {
						value = derefDay(record.CompleteDateWo)
					}
					f.SetCellValue(sheetEngProd, cell, value)
				case "SLA Status":
					if derefString(&record.SlaStatus) == "" {
						value = "N/A"
					} else {
						value = derefString(&record.SlaStatus)
					}
					f.SetCellValue(sheetEngProd, cell, value)
				case "SLA Expired":
					if derefString(&record.SlaExpired) == "" {
						value = "N/A"
					} else {
						value = derefString(&record.SlaExpired)
					}
					f.SetCellValue(sheetEngProd, cell, value)
				case "MID":
					if derefString(&record.Mid) == "" {
						value = "N/A"
					} else {
						value = derefString(&record.Mid)
					}
					f.SetCellValue(sheetEngProd, cell, value)
				case "TID":
					if derefString(&record.Tid) == "" {
						value = "N/A"
					} else {
						value = derefString(&record.Tid)
					}
					f.SetCellValue(sheetEngProd, cell, value)
				case "Merchant":
					if derefString(&record.MerchantName) == "" {
						value = "N/A"
					} else {
						value = derefString(&record.MerchantName)
					}
					f.SetCellValue(sheetEngProd, cell, value)
				case "Merchant PIC":
					if derefString(&record.MerchantPic) == "" {
						value = "N/A"
					} else {
						value = derefString(&record.MerchantPic)
					}
					f.SetCellValue(sheetEngProd, cell, value)
				case "Merchant Address":
					if derefString(&record.MerchantAddress) == "" {
						value = "N/A"
					} else {
						value = derefString(&record.MerchantAddress)
					}
					f.SetCellValue(sheetEngProd, cell, value)
				case "Merchant City":
					if derefString(&record.MerchantCity) == "" {
						value = "N/A"
					} else {
						value = derefString(&record.MerchantCity)
					}
					f.SetCellValue(sheetEngProd, cell, value)
				case "Task Count":
					if record.TaskCount == 0 {
						value = "N/A"
					} else {
						value = record.TaskCount
					}
					f.SetCellValue(sheetEngProd, cell, value)
				case "WO Remark":
					if derefString(&record.WoRemark) == "" {
						value = "N/A"
					} else {
						value = derefString(&record.WoRemark)
					}
					f.SetCellValue(sheetEngProd, cell, value)
				case "Reason Code":
					if derefString(&record.ReasonCode) == "" {
						value = "N/A"
					} else {
						value = derefString(&record.ReasonCode)
					}
					f.SetCellValue(sheetEngProd, cell, value)
				case "Reason":
					if derefString(&record.Reason) == "" {
						value = "N/A"
					} else {
						value = derefString(&record.Reason)
					}
					f.SetCellValue(sheetEngProd, cell, value)
				case "First JO Complete Datetime":
					if derefTime(record.FirstTaskCompleteDatetime) == "" {
						value = "N/A"
					} else {
						value = derefTime(record.FirstTaskCompleteDatetime)
					}
					f.SetCellValue(sheetEngProd, cell, value)
				case "First JO Reason Code":
					if derefString(&record.FirstTaskReasonCode) == "" {
						value = "N/A"
					} else {
						value = derefString(&record.FirstTaskReasonCode)
					}
					f.SetCellValue(sheetEngProd, cell, value)
				case "First JO Reason":
					if derefString(&record.FirstTaskReason) == "" {
						value = "N/A"
					} else {
						value = derefString(&record.FirstTaskReason)
					}
					f.SetCellValue(sheetEngProd, cell, value)
				case "First JO Message":
					if derefString(&record.FirstTaskMessage) == "" {
						value = "N/A"
					} else {
						value = derefString(&record.FirstTaskMessage)
					}
					f.SetCellValue(sheetEngProd, cell, value)
				case "Link WO":
					link := derefString(&record.LinkWod)
					if link == "" {
						value = "N/A"
						f.SetCellValue(sheetEngProd, cell, value)
					} else {
						displayText := "Click to see Detail WOD" // or use `link` if you want the URL visible
						err := f.SetCellHyperLink(sheetEngProd, cell, link, "External")
						if err != nil {
							log.Printf("failed to set hyperlink on %s: %v", cell, err)
						}
						f.SetCellValue(sheetEngProd, cell, displayText)
					}

				case "WO Number First":
					if derefString(&record.WoNumberFirst) == "" {
						value = "N/A"
					} else {
						value = derefString(&record.WoNumberFirst)
					}
					f.SetCellValue(sheetEngProd, cell, value)
				case "WO Number Last":
					if derefString(&record.WoNumberLast) == "" {
						value = "N/A"
					} else {
						value = derefString(&record.WoNumberLast)
					}
					f.SetCellValue(sheetEngProd, cell, value)
				case "EDC Type":
					if derefString(&record.EdcType) == "" {
						value = "N/A"
					} else {
						value = derefString(&record.EdcType)
					}
					f.SetCellValue(sheetEngProd, cell, value)
				case "EDC Serial":
					if derefString(&record.SnEdc) == "" {
						value = "N/A"
					} else {
						value = derefString(&record.SnEdc)
					}
					f.SetCellValue(sheetEngProd, cell, value)
				case "Description":
					if derefString(&record.Description) == "" {
						value = "N/A"
					} else {
						value = derefString(&record.Description)
					}
					f.SetCellValue(sheetEngProd, cell, value)
				}
			}
			rowIndex++
		}

		logrus.Debugf("Processed Engineers Productivity batch starting at offset %d, fetched %d records", offset, len(dataBatch))
		offset += batchSize
	}
	pivotEngProdDataRange := fmt.Sprintf("%s!$A$1:$%s$%d", sheetEngProd, lastColEngProd, rowIndex-1)
	pivotSheetEngProdRange := fmt.Sprintf("%s!A8:AJ200", sheetPvtProd)
	err = f.AddPivotTable(&excelize.PivotTableOptions{
		Name:            sheetPvtProd,
		DataRange:       pivotEngProdDataRange,
		PivotTableRange: pivotSheetEngProdRange,
		Rows: []excelize.PivotTableField{
			{Data: "Stage"},
		},
		Columns: []excelize.PivotTableField{
			{Data: "Complete WO (dd)"},
		},
		Data: []excelize.PivotTableField{
			{Data: "SLA Status", Subtotal: "count"},
		},
		Filter: []excelize.PivotTableField{
			{Data: "Ops Head"},
			{Data: "SPL"},
			{Data: "Technician"},
			{Data: "Worksheet Template"},
		},
		RowGrandTotals:      true,
		ColGrandTotals:      true,
		ShowDrill:           true,
		ShowRowHeaders:      true,
		ShowColHeaders:      true,
		ShowLastColumn:      true,
		PivotTableStyleName: "PivotStyleDark3", // Set your desired style here
	})
	if err != nil {
		return "", fmt.Sprintf("Gagal membuat Pivot Table untuk %s: %v", sheetPvtProd, err), fmt.Sprintf("Failed to create Pivot Table for %s: %v", sheetPvtProd, err)
	}
	f.SetColWidth(sheetPvtProd, "A", "A", 40)
	f.SetCellValue(sheetPvtProd, "A1", fmt.Sprintf("ENGINEERS PRODUCTIVITY - %s", strings.ToUpper(now.Format("January 2006"))))
	f.SetCellStyle(sheetPvtProd, "A1", "A1", stylePvtProd)

	/*
	**************************************************************************************
	 */

	/*
		RATE
	*/
	// Start with the first static column
	titlesRate := []struct {
		Title string
		Size  float64
	}{
		{"Metric", 25},
	}

	// Add date columns in the middle
	for day := 1; day <= numDays; day++ {
		date := time.Date(year, month, day, 0, 0, 0, 0, location)
		dateStr := date.Format("02-Jan-06")
		titlesRate = append(titlesRate, struct {
			Title string
			Size  float64
		}{
			Title: dateStr,
			Size:  15, // adjust as needed
		})
	}

	// Add the last static columns
	titlesRate = append(titlesRate,
		struct {
			Title string
			Size  float64
		}{"Grand Total", 20},
	)

	// Convert to ExcelColumn
	var columnsRate []ExcelColumn
	for i, t := range titlesRate {
		columnsRate = append(columnsRate, ExcelColumn{
			ColIndex: fun.GetColName(i),
			ColTitle: t.Title,
			ColSize:  t.Size,
		})
	}

	for _, column := range columnsRate {
		cell := fmt.Sprintf("%s1", column.ColIndex)
		f.SetCellValue(sheetRate, cell, column.ColTitle)
		f.SetColWidth(sheetRate, column.ColIndex, column.ColIndex, column.ColSize)

		// Custom style for and "Grand Total"
		switch column.ColTitle {
		case "Grand Total":
			styleGrandTotal, _ := f.NewStyle(&excelize.Style{
				Alignment: &excelize.Alignment{
					Horizontal: "center",
					Vertical:   "center",
				},
				Fill: excelize.Fill{
					Type:    "pattern",
					Color:   []string{"#00FF00"}, // green
					Pattern: 1,
				},
				Font: &excelize.Font{
					Bold:  true,
					Color: "#000000",
				},
			})
			f.SetCellStyle(sheetRate, cell, cell, styleGrandTotal)
		default:
			f.SetCellStyle(sheetRate, cell, cell, styleRateTitle)
		}
	}

	// Set autofilter for the first row
	lastColRate := fun.GetColName(len(columnsRate) - 1)
	filterRangeRate := fmt.Sprintf("A1:%s1", lastColRate)
	f.AutoFilter(sheetRate, filterRangeRate, []excelize.AutoFilterOptions{})

	// Freeze column A
	f.SetPanes(sheetRate, &excelize.Panes{
		Freeze:      true,
		Split:       true,
		XSplit:      1, // freeze first column (A)
		YSplit:      1, // freeze first row (header)
		TopLeftCell: "B2",
		ActivePane:  "bottomRight",
	})

	metrics := []string{
		"New (No Reply Yet)",
		"Solved",
	}

	rowIndex = 2
	var solvedRow int

	// Loop over your metrics first
	for _, metricName := range metrics {
		f.SetCellValue(sheetRate, fmt.Sprintf("A%d", rowIndex), metricName)

		colIndex := 1
		for _, col := range columnsRate[1:] {
			cell := fmt.Sprintf("%s%d", col.ColIndex, rowIndex)

			var count int64
			switch col.ColTitle {
			case "Grand Total":
				switch metricName {
				case "New (No Reply Yet)":
					var countNoReplyJOError int64
					var countNoReplyJOPending int64
					err := dbDataTA.Model(&tamodel.Error{}).
						Where("ta_feedback IS NOT NULL").
						Count(&countNoReplyJOError).Error
					if err != nil {
						logrus.Error(err)
						countNoReplyJOError = 0
					}
					err = dbDataTA.Model(&tamodel.Pending{}).
						Where("ta_feedback IS NOT NULL").
						Count(&countNoReplyJOPending).Error
					if err != nil {
						logrus.Error(err)
						countNoReplyJOPending = 0
					}

					// Count the un replied Pending + Error JO
					count = countNoReplyJOError + countNoReplyJOPending
				case "Solved":
					err := dbWeb.Model(&reportmodel.EngineersProductivityData{}).Where("LOWER(stage) = ?", "solved").
						Count(&count).Error
					if err != nil {
						logrus.Error(err)
						count = 0
					}
				}
				f.SetCellValue(sheetRate, cell, count)
			default:
				dateParsed, err := time.ParseInLocation("02-Jan-06", col.ColTitle, location)
				if err != nil {
					logrus.Errorf("Failed to parse date col title: %v", err)
					f.SetCellValue(sheetRate, cell, 0)
					continue
				}

				switch metricName {
				case "New (No Reply Yet)":
					var countNoReplyJOError int64
					var countNoReplyJOPending int64
					err := dbDataTA.Model(&tamodel.Error{}).
						Where("ta_feedback IS NOT NULL").
						Where("DATE(date) = ?", dateParsed.Format("2006-01-02")).
						Count(&countNoReplyJOError).Error
					if err != nil {
						logrus.Error(err)
						countNoReplyJOError = 0
					}
					err = dbDataTA.Model(&tamodel.Pending{}).
						Where("ta_feedback IS NOT NULL").
						Count(&countNoReplyJOPending).Error
					if err != nil {
						logrus.Error(err)
						countNoReplyJOPending = 0
					}

					// Count the un replied Pending + Error JO
					count = countNoReplyJOError + countNoReplyJOPending
				case "Solved":
					err := dbWeb.Model(&reportmodel.EngineersProductivityData{}).
						Where("LOWER(stage) = ?", "solved").
						Where("DATE(complete_date_wo) = ?", dateParsed.Format("2006-01-02")).
						Count(&count).Error
					if err != nil {
						logrus.Error(err)
						count = 0
					}
				}

				f.SetCellValue(sheetRate, cell, count)
			}

			colIndex++
		}

		// Remember Solved row index
		if metricName == "Solved" {
			solvedRow = rowIndex
		}

		rowIndex++
	}

	// ============ Add extra metric rows below ============

	// --- 1) ERR AI ---
	f.SetCellValue(sheetRate, fmt.Sprintf("A%d", rowIndex), "ERR AI")
	// errAIRow := rowIndex
	for _, col := range columnsRate[1:] {
		cell := fmt.Sprintf("%s%d", col.ColIndex, rowIndex)

		var count int64
		switch col.ColTitle {
		case "Grand Total":
			err := dbDataTA.Model(&tamodel.LogAct{}).
				Where("LOWER(type_case) = ? AND LOWER(method) = ?", "error", "submit").
				Where("problem IS NOT NULL").
				Count(&count).Error
			if err != nil {
				logrus.Errorf("Failed to count ERR AI grand total: %v", err)
				count = 0
			}
		default:
			dateParsed, err := time.ParseInLocation("02-Jan-06", col.ColTitle, location)
			if err != nil {
				logrus.Errorf("Failed to parse date col title: %v", err)
				count = 0
			} else {
				err = dbDataTA.Model(&tamodel.LogAct{}).
					Where("DATE(date) = ?", dateParsed.Format("2006-01-02")).
					Where("LOWER(type_case) = ? AND LOWER(method) = ?", "error", "submit").
					Where("problem IS NOT NULL").
					Count(&count).Error
				if err != nil {
					logrus.Errorf("Failed to count ERR AI for date %s: %v", dateParsed, err)
					count = 0
				}
			}
		}
		f.SetCellValue(sheetRate, cell, count)
	}
	rowIndex++

	// --- 2) AI Success Detection ---
	f.SetCellValue(sheetRate, fmt.Sprintf("A%d", rowIndex), "AI Success Detection")
	aiSuccessRow := rowIndex

	errAIRow := rowIndex - 1 // Because ERR AI was added right before this row

	for _, col := range columnsRate[1:] {
		cell := fmt.Sprintf("%s%d", col.ColIndex, rowIndex)

		// Build formula: =Solved - ERR AI
		formula := fmt.Sprintf("=%s%d-%s%d", col.ColIndex, solvedRow, col.ColIndex, errAIRow)

		// Ensure formula doesn't have double equals
		formula = strings.ReplaceAll(formula, "==", "=")

		// Validate and set formula with error handling
		if err := f.SetCellFormula(sheetRate, cell, formula); err != nil {
			logrus.Warnf("Failed to set AI Success Detection formula for cell %s: %v", cell, err)
			// Fallback: set as value 0
			f.SetCellValue(sheetRate, cell, 0)
		}
	}
	rowIndex++

	// --- 3) % Performance AI Success ---
	f.SetCellValue(sheetRate, fmt.Sprintf("A%d", rowIndex), "% Performance AI Success")
	perfAIRow := rowIndex
	for _, col := range columnsRate[1:] {
		cell := fmt.Sprintf("%s%d", col.ColIndex, rowIndex)

		// Use direct formula: divide success / solved
		formula := fmt.Sprintf(`=IFERROR(%s%d/%s%d,0)`, col.ColIndex, aiSuccessRow, col.ColIndex, solvedRow)

		// Ensure formula doesn't have double equals
		formula = strings.ReplaceAll(formula, "==", "=")

		// Validate and set formula with error handling
		if err := f.SetCellFormula(sheetRate, cell, formula); err != nil {
			logrus.Warnf("Failed to set Performance AI Success formula for cell %s: %v", cell, err)
			// Fallback: set as value 0
			f.SetCellValue(sheetRate, cell, 0)
		} else {
			// Apply percentage format: 0.00%
			style, _ := f.NewStyle(&excelize.Style{
				NumFmt: 10,
			})
			f.SetCellStyle(sheetRate, cell, cell, style)
		}
	}

	// Additional validation pass: Check and fix any remaining == issues
	validateAndFixFormulas(f, sheetRate, aiSuccessRow, perfAIRow, columnsRate)

	// Chart
	rowIndex++ // last rowIndex from your loop → next empty row

	noteRichText := []excelize.RichTextRun{
		{Text: "Notes:\n", Font: &excelize.Font{Bold: true}},
		{Text: "• New (No Reply Yet): Count of JOs that have been feedbacked by TA and still have no response.\n"},
		{Text: "• Solved: Count of JOs detected by AI with correct photos.\n"},
		{Text: "• ERR AI: Count of JOs detected by AI with incorrect photos.\n"},
		{Text: "• AI Success Detection: Solved minus ERR AI.\n"},
		{Text: "• % Performance AI Success: Percentage of AI Success Detection divided by Solved."},
	}

	if err := f.SetCellRichText(sheetRate, fmt.Sprintf("B%d", rowIndex+1), noteRichText); err != nil {
		return "", fmt.Sprintf("Gagal menambahkan catatan ke sheet %s: %v", sheetRate, err), fmt.Sprintf("Failed to add note to sheet %s: %v", sheetRate, err)
	}

	// Prepare columns
	startCol = columnsRate[1].ColIndex                // usually B
	endCol = columnsRate[len(columnsRate)-2].ColIndex // column before "Grand Total"
	categoriesRange = fmt.Sprintf("%s!$%s$1:$%s$1", sheetRate, startCol, endCol)

	// Collect series for bar (column) chart
	var seriesBar []excelize.ChartSeries
	var seriesLine []excelize.ChartSeries

	for row := 2; row <= rowIndex-1; row++ {
		metricNameCell := fmt.Sprintf("A%d", row)
		cellValue, err := f.GetCellValue(sheetRate, metricNameCell)
		if err != nil {
			logrus.Errorf("Failed to get cell value %s: %v", metricNameCell, err)
			continue
		}

		valueRange := fmt.Sprintf("%s!$%s$%d:$%s$%d", sheetRate, startCol, row, endCol, row)
		nameRange := fmt.Sprintf("%s!$A$%d", sheetRate, row)

		if cellValue == "% Performance AI Success" {
			// Add to line series
			seriesLine = append(seriesLine, excelize.ChartSeries{
				Name:       nameRange,
				Categories: categoriesRange,
				Values:     valueRange,
				Line:       excelize.ChartLine{Width: 2, Smooth: true},
				Marker:     excelize.ChartMarker{Symbol: "circle", Size: 5},
			})
		} else {
			// Add to bar series
			seriesBar = append(seriesBar, excelize.ChartSeries{
				Name:       nameRange,
				Categories: categoriesRange,
				Values:     valueRange,
			})
		}
	}

	// Column (bar) chart: all metrics except % Performance AI Success
	chartBar := &excelize.Chart{
		Dimension: excelize.ChartDimension{
			Width:  2500,
			Height: 500,
		},
		Type: excelize.Col,
		Title: []excelize.RichTextRun{
			{Text: "Performance AI"},
		},
		Legend: excelize.ChartLegend{
			Position:      "top",
			ShowLegendKey: false,
		},
		PlotArea: excelize.ChartPlotArea{
			ShowDataTable:     true,
			ShowDataTableKeys: true,
		},
		Series: seriesBar,
	}

	// Line chart: only % Performance AI Success, enable secondary axis
	chartLine := &excelize.Chart{
		Type:   excelize.Line,
		Series: seriesLine,
		YAxis: excelize.ChartAxis{
			Secondary: true,
			Title: []excelize.RichTextRun{
				{Text: "% Performance AI Success"},
			},
			NumFmt: excelize.ChartNumFmt{
				CustomNumFmt: "0.00%",
			}, // percentage format
		},
	}

	chartCell = fmt.Sprintf("B%d", rowIndex+2)
	if err := f.AddChart(sheetRate, chartCell, chartBar, chartLine); err != nil {
		return "", fmt.Sprintf("Gagal menambahkan chart ke sheet %s: %v", sheetRate, err),
			fmt.Sprintf("Failed to add chart to sheet %s: %v", sheetRate, err)
	}

	/*
	**************************************************************************************
	 */

	// Set "MASTER" as active sheet
	f.SetActiveSheet(0)
	f.MoveSheet(sheetRate, sheetMaster)

	// Final comprehensive formula cleanup for all sheets
	performFinalFormulaCleanup(f)

	// Build full file path
	fullFilePath := filepath.Join(fileReportDir, reportName)

	// Save the Excel file
	if err := f.SaveAs(fullFilePath); err != nil {
		return "", fmt.Sprintf("Gagal menyimpan file Excel: %v", err), fmt.Sprintf("Failed to save Excel file: %v", err)
	}

	return fullFilePath, "", "" // Return the file path and empty error messages
}
