package controllers

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"service-platform/cmd/web_panel/fun"
	"service-platform/cmd/web_panel/internal/gormdb"
	reportmodel "service-platform/cmd/web_panel/model/report_model"
	tamodel "service-platform/cmd/web_panel/model/ta_model"
	"service-platform/internal/config"
	"strings"
	"sync"
	"time"

	"github.com/sirupsen/logrus"
	"github.com/xuri/excelize/v2"
	"go.mau.fi/whatsmeow/types/events"
	"gorm.io/gorm"
)

var (
	generateReportTAMutex        sync.Mutex
	generateTechErrorReportMutex sync.Mutex
	generateReportComparedMutex  sync.Mutex
	getDataTaskComparedMutex     sync.Mutex

	pendingDataLeft                     []tamodel.Pending
	errorDataLeft                       []tamodel.Error
	pendingDataLeftSLAToday             []tamodel.Pending
	errorDataLeftSLAToday               []tamodel.Error
	pendingDataLeftDateInDashboardToday []tamodel.Pending
	errorDataLeftDateInDashboardToday   []tamodel.Error

	pendingLeftFeedbackYetButNoResponse []tamodel.Pending
	errorLeftFeedbackYetButNoResponse   []tamodel.Error

	// Sheet in Report TA
	sheetTAFeedbackAwaitingNextAction = "Awaiting_Next_Action"
	sheetPIVOTAwaitingNextAction      = "PIVOT_Waiting_Next_Action"
	sheetTechnicianNeedToResolve      = "Technician_Need_Resolve"
	sheetPIVOTTechnicianMajorProblem  = "PIVOT_Major_Problems"

	// Timeout
	httpReqTimeout             = 15 * time.Minute
	waitForFileUnlockedTimeout = 5 * time.Minute
	contextCmdTimeout          = 10 * time.Minute
)

type DashboardTACheck struct {
	NameID       string
	NameEN       string
	Model        any
	QueryFunc    func(db *gorm.DB) *gorm.DB
	FormatResult func() (string, string) // returns (idText, enText)
}

type ComparedReportDataExcel struct {
	ExcelName string
	StartDate time.Time
	EndDate   time.Time
}

func ReportTA(v *events.Message, userLang string) {
	eventToDo := "Generate Technical Assistance Report"
	originalSenderJID := NormalizeSenderJID(v.Info.Sender.String())
	stanzaID := v.Info.ID

	if !generateReportTAMutex.TryLock() {
		id := fmt.Sprintf("⚠ Permintaan *%s* sedang diproses. Mohon tunggu beberapa saat.", eventToDo)
		en := fmt.Sprintf("⚠ Your *%s* request is being processed. Please wait a moment.", eventToDo)
		sendLangMessageWithStanza(v, stanzaID, originalSenderJID, id, en, userLang)
		return
	}
	defer generateReportTAMutex.Unlock()

	// Inform user we've received request
	id, en := informUserRequestReceived(eventToDo)
	sendLangMessageWithStanza(v, stanzaID, originalSenderJID, id, en, userLang)

	idText, enText := ShowTAUsersOnline(userLang)
	sendLangMessageWithStanza(v, stanzaID, originalSenderJID, idText, enText, userLang)

	idText, enText = ShowListLeftDataTA(userLang)
	sendLangMessageWithStanza(v, stanzaID, originalSenderJID, idText, enText, userLang)

	idText, enText, excelFilePath := GetFileReportTA(userLang)
	if excelFilePath != "" {
		sendLangMessageWithStanza(v, stanzaID, originalSenderJID, idText, enText, userLang)
		mentions := config.WebPanel.Get().TechnicalAssistanceData.ReportTAMentions

		excelFileCaption := "Report TA"
		SendExcelFileWithStanza(v, stanzaID, originalSenderJID, excelFilePath, excelFileCaption, mentions, userLang)

		idText, enText, imgSS := GetImgPivotReportFirstSheet(excelFilePath, userLang)
		if imgSS != "" {
			SendImgFileWithStanza(v, stanzaID, originalSenderJID, imgSS, excelFileCaption, mentions, userLang)

			// 🧹 Cleanup: remove the image and the original Excel file
			if removeErr := os.Remove(imgSS); removeErr != nil {
				logrus.Errorf("⚠ Gagal menghapus file gambar: %v", removeErr)
			} else {
				logrus.Printf("🧹 Gambar %s berhasil dihapus setelah dikirim.", imgSS)
			}

			if removeErr := os.Remove(excelFilePath); removeErr != nil {
				logrus.Errorf("⚠ Gagal menghapus file Excel: %v", removeErr)
			} else {
				logrus.Printf("🧹 Excel file %s berhasil dihapus setelah digunakan.", excelFilePath)
			}
		} else {
			sendLangMessageWithStanza(v, stanzaID, originalSenderJID, idText, enText, userLang)
		}
	} else {
		sendLangMessageWithStanza(v, stanzaID, originalSenderJID, idText, enText, userLang)
	}

	idText, enText = ShowListLeftDataFeedbackYetButNotResponse(userLang)
	sendLangMessageWithStanza(v, stanzaID, originalSenderJID, idText, enText, userLang)

	idText, enText, excelFilePath = GetFileReportTAFeedbackedAwaitingNextAction(userLang)
	if excelFilePath != "" {
		sendLangMessageWithStanza(v, stanzaID, originalSenderJID, idText, enText, userLang)
		mentions := config.WebPanel.Get().TechnicalAssistanceData.ReportTAMentions

		excelFileCaption := "Report TA Feedbacked, awaiting next action"
		SendExcelFileWithStanza(v, stanzaID, originalSenderJID, excelFilePath, excelFileCaption, mentions, userLang)

		idText, enText, imgSS := GetImgPivotReportFirstSheet(excelFilePath, userLang)
		if imgSS != "" {
			SendImgFileWithStanza(v, stanzaID, originalSenderJID, imgSS, excelFileCaption, mentions, userLang)

			// 🧹 Cleanup: remove the image
			if removeErr := os.Remove(imgSS); removeErr != nil {
				logrus.Errorf("⚠ Gagal menghapus file gambar: %v", removeErr)
			} else {
				logrus.Printf("🧹 Gambar %s berhasil dihapus setelah dikirim.", imgSS)
			}

			// Reorder the PIVOT
			f, err := excelize.OpenFile(excelFilePath)
			if err != nil {
				idText := fmt.Sprintf("Gagal membuka file excel: %v", err)
				enText := fmt.Sprintf("Failed to open excel file: %v", err)
				sendLangMessageWithStanza(v, stanzaID, originalSenderJID, idText, enText, userLang)
			}
			defer f.Close()

			err = f.MoveSheet(sheetPIVOTTechnicianMajorProblem, sheetPIVOTAwaitingNextAction)
			if err != nil {
				idText := fmt.Sprintf("Gagal memindahkan sheet: %v", err)
				enText := fmt.Sprintf("Failed to move sheet: %v", err)
				sendLangMessageWithStanza(v, stanzaID, originalSenderJID, idText, enText, userLang)
			}
			if err := f.Save(); err != nil {
				idText := fmt.Sprintf("Gagal menyimpan file excel: %v", err)
				enText := fmt.Sprintf("Failed to save excel: %v", err)
				sendLangMessageWithStanza(v, stanzaID, originalSenderJID, idText, enText, userLang)
			}

			// Send 2nd PIVOT SS
			idText, enText, imgSS := GetImgPivotReportFirstSheet(excelFilePath, userLang)
			if imgSS != "" {
				excelFileCaption = "Major Problem Technician"
				SendImgFileWithStanza(v, stanzaID, originalSenderJID, imgSS, excelFileCaption, mentions, userLang)

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

		} else {
			sendLangMessageWithStanza(v, stanzaID, originalSenderJID, idText, enText, userLang)
		}
	} else {
		sendLangMessageWithStanza(v, stanzaID, originalSenderJID, idText, enText, userLang)
	}
}

func ShowTAUsersOnline(userLang string) (string, string) {
	loc, err := time.LoadLocation(config.WebPanel.Get().Default.Timezone)
	if err != nil {
		id := fmt.Sprintf("⚠ Gagal memuat zona waktu %s.", config.WebPanel.Get().Default.Timezone)
		en := fmt.Sprintf("⚠ Failed to load zone %s.", config.WebPanel.Get().Default.Timezone)
		return id, en
	}

	now := time.Now().In(loc)
	startOfDay := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, loc)
	endOfDay := startOfDay.Add(24 * time.Hour)

	DBWebTA := gormdb.Databases.WebTA
	// Get Users Online
	var users []tamodel.Admin
	if err := DBWebTA.
		Where("updated_at >= ? AND updated_at < ?", startOfDay, endOfDay).
		Find(&users).Error; err != nil {
		id := fmt.Sprintf("⚠ Gagal mengambil data login hari ini: %v", err)
		en := fmt.Sprintf("⚠ Failed to fetch today's login data: %v", err)
		return id, en
	}

	if len(users) > 0 {
		var sb strings.Builder
		sb.WriteString(fmt.Sprintf("📋 *Daftar %d User Login Hari Ini:*\n\n", len(users)))

		for i, admin := range users {
			sb.WriteString(fmt.Sprintf(
				"%d. %s (Email: %s, Terakhir Login: %s)\n",
				i+1,
				admin.Fullname,
				admin.Email,
				admin.UpdatedAt.In(loc).Format("15:04:05"),
			))
		}

		return sb.String(), sb.String()
	}
	id := "ℹ️ Belum ada user yang login hari ini."
	en := "ℹ️ No users have logged in today."
	return id, en
}

func ShowListLeftDataTA(userLang string) (string, string) {
	loc, err := time.LoadLocation(config.WebPanel.Get().Default.Timezone)
	if err != nil {
		id := fmt.Sprintf("⚠ Gagal memuat zona waktu %s.", config.WebPanel.Get().Default.Timezone)
		en := fmt.Sprintf("⚠ Failed to load zone %s.", config.WebPanel.Get().Default.Timezone)
		return id, en
	}

	now := time.Now().In(loc)
	startOfDay := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, loc)
	endOfDay := startOfDay.Add(24 * time.Hour)
	DBDataTA := gormdb.Databases.TA

	// Left Data
	checks := []DashboardTACheck{
		{
			NameID: "Pending Total",
			NameEN: "Total Pending",
			Model:  &pendingDataLeft,
			QueryFunc: func(db *gorm.DB) *gorm.DB {
				return db.Model(&tamodel.Pending{})
			},
			FormatResult: func() (string, string) {
				if len(pendingDataLeft) == 0 {
					return "", ""
				}
				id := fmt.Sprintf("📌 Total data pending: *%d*", len(pendingDataLeft))
				en := fmt.Sprintf("📌 Total pending data: *%d*", len(pendingDataLeft))
				return id, en
			},
		},
		{
			NameID: "Pending SLA Hari Ini",
			NameEN: "Pending SLA Today",
			Model:  &pendingDataLeftSLAToday,
			QueryFunc: func(db *gorm.DB) *gorm.DB {
				return db.Model(&tamodel.Pending{}).
					Where("STR_TO_DATE(sla, '%Y-%m-%d %H:%i:%s') >= ? AND STR_TO_DATE(sla, '%Y-%m-%d %H:%i:%s') < ?", startOfDay, endOfDay)
			},
			FormatResult: func() (string, string) {
				if len(pendingDataLeftSLAToday) == 0 {
					return "", ""
				}
				var bID, bEN strings.Builder
				bID.WriteString(fmt.Sprintf("📆 *Pending SLA Hari Ini*: %d\n", len(pendingDataLeftSLAToday)))
				bEN.WriteString(fmt.Sprintf("📆 *Pending SLA Today*: %d\n", len(pendingDataLeftSLAToday)))
				for i, d := range pendingDataLeftSLAToday {
					sla := "N/A"
					ticketType := "N/A"
					if d.Sla != nil {
						sla = *d.Sla
					}
					if d.Type2 != nil {
						ticketType = *d.Type2
					}
					// build Indonesian
					bID.WriteString(fmt.Sprintf("%d. ID Task: %s | WO Number: %s | Ticket Subject: %s | Tipe Ticket: %s | SLA: %s | Tanggal di Dashboard TA: %s\n",
						i+1, d.IDTask, d.WoNumber, d.SpkNumber, ticketType, sla, d.Date))
					// build English
					bEN.WriteString(fmt.Sprintf("%d. ID Task: %s | WO Number: %s | Ticket Subject: %s | Ticket Type: %s | SLA: %s | Date in Dashboard TA: %s\n",
						i+1, d.IDTask, d.WoNumber, d.SpkNumber, ticketType, sla, d.Date))
				}
				return bID.String(), bEN.String()
			},
		},
		{
			NameID: "Pending Data Hari Ini",
			NameEN: "Pending Today",
			Model:  &pendingDataLeftDateInDashboardToday,
			QueryFunc: func(db *gorm.DB) *gorm.DB {
				return db.Model(&tamodel.Pending{}).
					Where("date >= ? AND date < ?", startOfDay, endOfDay)
			},
			FormatResult: func() (string, string) {
				if len(pendingDataLeftDateInDashboardToday) == 0 {
					return "", ""
				}
				var bID, bEN strings.Builder
				bID.WriteString(fmt.Sprintf("🕔 *Pending Data Hari Ini*: %d\n", len(pendingDataLeftDateInDashboardToday)))
				bEN.WriteString(fmt.Sprintf("🕔 *Pending Today*: %d\n", len(pendingDataLeftDateInDashboardToday)))
				for i, d := range pendingDataLeftDateInDashboardToday {
					sla := "N/A"
					ticketType := "N/A"
					if d.Sla != nil {
						sla = *d.Sla
					}
					if d.Type2 != nil {
						ticketType = *d.Type2
					}
					// build Indonesian
					bID.WriteString(fmt.Sprintf("%d. ID Task: %s | WO Number: %s | Ticket Subject: %s | Tipe Ticket: %s | SLA: %s | Tanggal di Dashboard TA: %s\n",
						i+1, d.IDTask, d.WoNumber, d.SpkNumber, ticketType, sla, d.Date))
					// build English
					bEN.WriteString(fmt.Sprintf("%d. ID Task: %s | WO Number: %s | Ticket Subject: %s | Ticket Type: %s | SLA: %s | Date in Dashboard TA: %s\n",
						i+1, d.IDTask, d.WoNumber, d.SpkNumber, ticketType, sla, d.Date))
				}
				return bID.String(), bEN.String()
			},
		},
		/*
			Error
		*/
		{
			NameID: "Error Total",
			NameEN: "Total Error",
			Model:  &errorDataLeft,
			QueryFunc: func(db *gorm.DB) *gorm.DB {
				return db.Model(&tamodel.Error{})
			},
			FormatResult: func() (string, string) {
				if len(errorDataLeft) == 0 {
					return "", ""
				}
				id := fmt.Sprintf("📌 Total data error: *%d*", len(errorDataLeft))
				en := fmt.Sprintf("📌 Total error data: *%d*", len(errorDataLeft))
				return id, en
			},
		},
		{
			NameID: "Error SLA Hari Ini",
			NameEN: "Error SLA Today",
			Model:  &errorDataLeftSLAToday,
			QueryFunc: func(db *gorm.DB) *gorm.DB {
				return db.Model(&tamodel.Error{}).
					Where("STR_TO_DATE(sla, '%Y-%m-%d %H:%i:%s') >= ? AND STR_TO_DATE(sla, '%Y-%m-%d %H:%i:%s') < ?", startOfDay, endOfDay)
			},
			FormatResult: func() (string, string) {
				if len(errorDataLeftSLAToday) == 0 {
					return "", ""
				}
				var bID, bEN strings.Builder
				bID.WriteString(fmt.Sprintf("📆 *Error SLA Hari Ini*: %d\n", len(errorDataLeftSLAToday)))
				bEN.WriteString(fmt.Sprintf("📆 *Error SLA Today*: %d\n", len(errorDataLeftSLAToday)))
				for i, d := range errorDataLeftSLAToday {
					sla := "N/A"
					ticketType := "N/A"
					if d.Sla != nil {
						sla = *d.Sla
					}
					if d.Type2 != nil {
						ticketType = *d.Type2
					}
					// build Indonesian
					bID.WriteString(fmt.Sprintf("%d. ID Task: %s | WO Number: %s | Ticket Subject: %s | Tipe Ticket: %s | SLA: %s | Tanggal di Dashboard TA: %s\n",
						i+1, d.IDTask, d.WoNumber, d.SpkNumber, ticketType, sla, d.Date))
					// build English
					bEN.WriteString(fmt.Sprintf("%d. ID Task: %s | WO Number: %s | Ticket Subject: %s | Ticket Type: %s | SLA: %s | Date in Dashboard TA: %s\n",
						i+1, d.IDTask, d.WoNumber, d.SpkNumber, ticketType, sla, d.Date))
				}
				return bID.String(), bEN.String()
			},
		},
		{
			NameID: "Error Data Hari Ini",
			NameEN: "Error Today",
			Model:  &errorDataLeftDateInDashboardToday,
			QueryFunc: func(db *gorm.DB) *gorm.DB {
				return db.Model(&tamodel.Error{}).
					Where("date >= ? AND date < ?", startOfDay, endOfDay)
			},
			FormatResult: func() (string, string) {
				if len(errorDataLeftDateInDashboardToday) == 0 {
					return "", ""
				}
				var bID, bEN strings.Builder
				bID.WriteString(fmt.Sprintf("🕔 *Error Data Hari Ini*: %d\n", len(errorDataLeftDateInDashboardToday)))
				bEN.WriteString(fmt.Sprintf("🕔 *Error Today*: %d\n", len(errorDataLeftDateInDashboardToday)))
				for i, d := range errorDataLeftDateInDashboardToday {
					sla := "N/A"
					ticketType := "N/A"
					if d.Sla != nil {
						sla = *d.Sla
					}
					if d.Type2 != nil {
						ticketType = *d.Type2
					}
					// build Indonesian
					bID.WriteString(fmt.Sprintf("%d. ID Task: %s | WO Number: %s | Ticket Subject: %s | Tipe Ticket: %s | SLA: %s | Tanggal di Dashboard TA: %s\n",
						i+1, d.IDTask, d.WoNumber, d.SpkNumber, ticketType, sla, d.Date))
					// build English
					bEN.WriteString(fmt.Sprintf("%d. ID Task: %s | WO Number: %s | Ticket Subject: %s | Ticket Type: %s | SLA: %s | Date in Dashboard TA: %s\n",
						i+1, d.IDTask, d.WoNumber, d.SpkNumber, ticketType, sla, d.Date))
				}
				return bID.String(), bEN.String()
			},
		},
	}

	var sb strings.Builder
	hasData := false

	// Use channel and goroutines for parallel query execution
	type checkResult struct {
		idText  string
		enText  string
		err     error
		hasData bool
	}

	resultChan := make(chan checkResult, len(checks))

	// Execute all queries in parallel
	for i, check := range checks {
		go func(index int, c DashboardTACheck) {
			defer func() {
				if r := recover(); r != nil {
					resultChan <- checkResult{err: fmt.Errorf("panic in query %d: %v", index, r)}
				}
			}()

			if err := c.QueryFunc(DBDataTA).Find(c.Model).Error; err != nil {
				log.Printf("❌ Error querying (ID: %s | EN: %s): %v", c.NameID, c.NameEN, err)
				resultChan <- checkResult{err: err}
				return
			}

			idText, enText := c.FormatResult()
			hasDataForThisCheck := idText != "" || enText != ""
			resultChan <- checkResult{
				idText:  idText,
				enText:  enText,
				hasData: hasDataForThisCheck,
			}
		}(i, check)
	}

	// Collect results in order
	results := make([]checkResult, len(checks))
	for i := 0; i < len(checks); i++ {
		results[i] = <-resultChan
		if results[i].err != nil {
			continue // Skip failed queries
		}
		if results[i].hasData {
			hasData = true
		}
	}

	// Build response in original order
	for _, result := range results {
		if result.err != nil {
			continue
		}

		var textToAdd string
		if userLang == "id" {
			textToAdd = result.idText
		} else {
			textToAdd = result.enText
		}

		if textToAdd != "" {
			if !hasData {
				// Add header depending on userLang
				if userLang == "id" {
					sb.WriteString(fmt.Sprintf("📊 Data Dashboard TA per %s\n\n", now.Format("02 Jan 2006, 15:04:05")))
				} else {
					sb.WriteString(fmt.Sprintf("📊 Dashboard TA data as of %s\n\n", now.Format("02 Jan 2006, 15:04:05")))
				}
				hasData = true
			}
			sb.WriteString(textToAdd + "\n")
		}
	}

	// Send only in user's language
	if hasData {
		text := sb.String()
		return text, text
	} else {
		id := "🥳 Yeay!! Tidak ada data tersisa di Dashboard TA hari ini."
		en := "🥳 Yay! No remaining data in Dashboard TA today."
		return id, en
	}

}

func ShowListLeftDataFeedbackYetButNotResponse(userLang string) (string, string) {
	loc, err := time.LoadLocation(config.WebPanel.Get().Default.Timezone)
	if err != nil {
		id := fmt.Sprintf("⚠ Gagal memuat zona waktu %s.", config.WebPanel.Get().Default.Timezone)
		en := fmt.Sprintf("⚠ Failed to load zone %s.", config.WebPanel.Get().Default.Timezone)
		return id, en
	}

	now := time.Now().In(loc)
	startOfDay := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, loc)
	endOfDay := startOfDay.Add(24 * time.Hour)
	DBDataTA := gormdb.Databases.TA

	// Not response data
	checks := []DashboardTACheck{
		{
			NameID: "Pending - Feedback TA, menunggu tindak lanjut",
			NameEN: "Pending - TA feedback, awaiting next action",
			Model:  &pendingLeftFeedbackYetButNoResponse,
			QueryFunc: func(db *gorm.DB) *gorm.DB {
				return db.Model(&tamodel.Pending{}).
					Where("(date_on_check < ? OR date_on_check >= ?) AND ta_feedback IS NOT NULL", startOfDay, endOfDay)
			},
			FormatResult: func() (string, string) {
				if len(pendingLeftFeedbackYetButNoResponse) == 0 {
					return "", ""
				}
				var bID, bEN strings.Builder
				bID.WriteString(fmt.Sprintf("⌛ *Pending sudah di feedback, menunggu tindak lanjut*: %d\n", len(pendingLeftFeedbackYetButNoResponse)))
				bEN.WriteString(fmt.Sprintf("⌛ *Pending already got feedback, awaiting next action*: %d\n", len(pendingLeftFeedbackYetButNoResponse)))
				for i, d := range pendingLeftFeedbackYetButNoResponse {
					sla := "N/A"
					ticketType := "N/A"
					if d.Sla != nil {
						sla = *d.Sla
					}
					if d.Type2 != nil {
						ticketType = *d.Type2
					}
					// build Indonesian
					bID.WriteString(fmt.Sprintf("%d) ID Task: %s | WO Number: %s | Ticket Subject: %s | Tipe Ticket: %s | SLA: %s | Tanggal di Dashboard TA: %s | Teknisi: %s | Diperiksa Pada: %v | Feedback TA: %s | TID: %s\n",
						i+1, d.IDTask, d.WoNumber, d.SpkNumber, ticketType, sla, d.Date, d.Teknisi, d.DateOnCheck.Format("2006-01-02 15:04:05"), d.TaFeedback, d.TID))
					// build English
					bEN.WriteString(fmt.Sprintf("%d) ID Task: %s | WO Number: %s | Ticket Subject: %s | Ticket Type: %s | SLA: %s | Date in Dashboard TA: %s | Technician: %s | Checked On: %v | TA Feedback: %s | TID: %s\n",
						i+1, d.IDTask, d.WoNumber, d.SpkNumber, ticketType, sla, d.Date, d.Teknisi, d.DateOnCheck.Format("2006-01-02 15:04:05"), d.TaFeedback, d.TID))
				}
				return bID.String(), bEN.String()
			},
		},
		{
			NameID: "Error - Feedback TA, menunggu tindak lanjut",
			NameEN: "Error - TA feedback, awaiting next action",
			Model:  &errorLeftFeedbackYetButNoResponse,
			QueryFunc: func(db *gorm.DB) *gorm.DB {
				return db.Model(&tamodel.Error{}).
					Where("(date_on_check < ? OR date_on_check >= ?) AND ta_feedback IS NOT NULL", startOfDay, endOfDay)
			},
			FormatResult: func() (string, string) {
				if len(errorLeftFeedbackYetButNoResponse) == 0 {
					return "", ""
				}
				var bID, bEN strings.Builder
				bID.WriteString(fmt.Sprintf("⌛ *Error sudah di feedback, menunggu tindak lanjut*: %d\n", len(errorLeftFeedbackYetButNoResponse)))
				bEN.WriteString(fmt.Sprintf("⌛ *Error already got feedback, awaiting next action*: %d\n", len(errorLeftFeedbackYetButNoResponse)))
				for i, d := range errorLeftFeedbackYetButNoResponse {
					sla := "N/A"
					ticketType := "N/A"
					problem := "N/A"
					if d.Sla != nil {
						sla = *d.Sla
					}
					if d.Problem != nil {
						problem = *d.Problem
					}
					if d.Type2 != nil {
						ticketType = *d.Type2
					}
					// build Indonesian
					bID.WriteString(fmt.Sprintf("%d) ID Task: %s | WO Number: %s | Ticket Subject: %s | Tipe Ticket: %s | SLA: %s | Tanggal di Dashboard TA: %s | Teknisi: %s | Diperiksa Pada: %v | Feedback TA: %s | TID: %s | Kendala: %s\n",
						i+1, d.IDTask, d.WoNumber, d.SpkNumber, ticketType, sla, d.Date, d.Teknisi, d.DateOnCheck.Format("2006-01-02 15:04:05"), d.TaFeedback, d.TID, problem))
					// build English
					bEN.WriteString(fmt.Sprintf("%d) ID Task: %s | WO Number: %s | Ticket Subject: %s | Ticket Type: %s | SLA: %s | Date in Dashboard TA: %s | Technician: %s | Checked On: %v | TA Feedback: %s | TID: %s | Problem: %s\n",
						i+1, d.IDTask, d.WoNumber, d.SpkNumber, ticketType, sla, d.Date, d.Teknisi, d.DateOnCheck.Format("2006-01-02 15:04:05"), d.TaFeedback, d.TID, problem))
				}
				return bID.String(), bEN.String()
			},
		},
	}

	var sb strings.Builder
	hasData := false

	for _, check := range checks {
		if err := check.QueryFunc(DBDataTA).Find(check.Model).Error; err != nil {
			log.Printf("❌ Error querying (ID: %s | EN: %s): %v", check.NameID, check.NameEN, err)
			continue
		}

		idText, enText := check.FormatResult()
		var textToAdd string
		if userLang == "id" {
			textToAdd = idText
		} else {
			textToAdd = enText
		}

		if textToAdd != "" {
			if !hasData {
				// Add header depending on userLang
				if userLang == "id" {
					sb.WriteString(fmt.Sprintf("🗃 Data di Dashboard TA per %s\n\n", now.Format("02 Jan 2006, 15:04:05")))
				} else {
					sb.WriteString(fmt.Sprintf("🗃 Dashboard TA data as of %s\n\n", now.Format("02 Jan 2006, 15:04:05")))
				}
				hasData = true
			}
			sb.WriteString(textToAdd + "\n")
		}
	}

	// Send only in user's language
	if hasData {
		text := sb.String()
		return text, text
	} else {
		id := "🥳 Yeay!! Tidak ada data di Dashboard TA yang terfeedback namun belum diresponse."
		en := "🥳 Yay! There is no data in the TA dashboard that has been feedbacked but not yet responded to."
		return id, en
	}

}

func GetFileReportTA(userLang string) (idText string, enText string, excelFilePath string) {
	// Step 1: Find valid directory to save report
	selectedMainDir, err := fun.FindValidDirectory([]string{
		"web/file/ta_report",
		"../web/file/ta_report",
		"../../web/file/ta_report",
	})
	if err != nil {
		idText = fmt.Sprintf("⚠ Mohon maaf, gagal untuk generate report TA: %v", err)
		enText = fmt.Sprintf("⚠ Sorry, failed to generate TA Report: %v", err)
		return
	}

	// Step 2: Create dated directory
	fileReportDir := filepath.Join(selectedMainDir, time.Now().Format("2006-01-02"))
	if err := os.MkdirAll(fileReportDir, 0755); err != nil {
		idText = fmt.Sprintf("⚠ Mohon maaf, gagal membuat direktori: %v", err)
		enText = fmt.Sprintf("⚠ Sorry, failed to create directory: %v", err)
		return
	}

	// Step 3: Download the report with timeout
	taReportURL := config.WebPanel.Get().TechnicalAssistanceData.PublicURLReportTA

	// Create HTTP client with timeout
	client := &http.Client{
		Timeout: httpReqTimeout,
	}

	resp, err := client.Get(taReportURL)
	if err != nil {
		idText = fmt.Sprintf("❌ Gagal mengakses report dari %s: %v", taReportURL, err)
		enText = fmt.Sprintf("❌ Failed to access report from %s: %v", taReportURL, err)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		idText = fmt.Sprintf("⚠ Gagal mengambil report, status: %d - %s\n%s", resp.StatusCode, http.StatusText(resp.StatusCode), string(body))
		enText = fmt.Sprintf("⚠ Failed to fetch report, status: %d - %s\n%s", resp.StatusCode, http.StatusText(resp.StatusCode), string(body))
		return
	}

	// Step 4: Extract filename
	filename := "ta_report_download"
	if cd := resp.Header.Get("Content-Disposition"); cd != "" {
		if parts := strings.Split(cd, "filename="); len(parts) == 2 {
			filename = strings.Trim(parts[1], `"`)
		}
	}

	// Step 5: Save file
	filePath := filepath.Join(fileReportDir, filename)
	outFile, err := os.Create(filePath)
	if err != nil {
		idText = fmt.Sprintf("❌ Gagal membuat file di %s: %v", filePath, err)
		enText = fmt.Sprintf("❌ Failed to create file at %s: %v", filePath, err)
		return
	}
	defer outFile.Close()

	_, err = io.Copy(outFile, resp.Body)
	if err != nil {
		idText = fmt.Sprintf("❌ Gagal menyimpan report ke %s: %v", filePath, err)
		enText = fmt.Sprintf("❌ Failed to save report to %s: %v", filePath, err)
		return
	}

	// Step 6: Build success message (same meaning, two languages)
	idText = fmt.Sprintf(
		"🎉 Yeay! Report *%s* berhasil digenerate oleh _system_.\n\n"+
			"Untuk melihat feedback tim TA terkait JO yang belum bisa diapprove karena kurang evidence dll, cek sheet *PENDING DATA LEFT* kolom *P* dan *ERROR DATA LEFT* kolom *Q*.\n"+
			"Untuk hasil pengerjaan TA, cek sheet *MASTER*.",
		filename)

	enText = fmt.Sprintf(
		"🎉 Yay! Report *%s* was successfully generated by the _system_.\n\n"+
			"To view feedback from the TA team on JOs that couldn't be approved due to missing evidence etc., check sheet *PENDING DATA LEFT* column *P* and *ERROR DATA LEFT* column *Q*.\n"+
			"For completed TA work, check the *MASTER* sheet.",
		filename)

	// Return file path so caller can attach/send
	excelFilePath = filePath
	return
}

func GetFileReportTAFeedbackedAwaitingNextAction(userLang string) (idText string, enText string, excelFilePath string) {
	taskDoing := "Generate Report TA Feedbacked Awaiting Next Action"

	loc, err := time.LoadLocation(config.WebPanel.Get().Default.Timezone)
	if err != nil {
		idText = fmt.Sprintf("⚠ Gagal memuat zona waktu %s.", config.WebPanel.Get().Default.Timezone)
		enText = fmt.Sprintf("⚠ Failed to load zone %s.", config.WebPanel.Get().Default.Timezone)
		return
	}

	now := time.Now().In(loc)
	startOfDay := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, loc)
	endOfDay := startOfDay.Add(24 * time.Hour)

	selectedMainDir, err := fun.FindValidDirectory([]string{
		"web/file/ta_report",
		"../web/file/ta_report",
		"../../web/file/ta_report",
	})
	if err != nil {
		idText = fmt.Sprintf("⚠ Mohon maaf, gagal untuk %s: %v", taskDoing, err)
		enText = fmt.Sprintf("⚠ Sorry, failed to %s: %v", taskDoing, err)
		return
	}

	fileReportDir := filepath.Join(selectedMainDir, time.Now().Format("2006-01-02"))
	if err := os.MkdirAll(fileReportDir, 0755); err != nil {
		idText = fmt.Sprintf("⚠ Mohon maaf, gagal membuat direktori: %v", err)
		enText = fmt.Sprintf("⚠ Sorry, failed to create directory: %v", err)
		return
	}

	reportName := fmt.Sprintf("Report_With_TA_Feedback_Awaiting_Next_Action_%v", now.Format("02Jan2006_15_04_05.xlsx"))

	// Create report
	f := excelize.NewFile()
	sheetEmployee := "EMPLOYEES"

	f.NewSheet(sheetTAFeedbackAwaitingNextAction)
	f.NewSheet(sheetPIVOTAwaitingNextAction)
	f.NewSheet(sheetTechnicianNeedToResolve)
	f.NewSheet(sheetPIVOTTechnicianMajorProblem)
	f.NewSheet(sheetEmployee)

	f.DeleteSheet("Sheet1")

	/* Styles */
	styleTitle, _ := f.NewStyle(&excelize.Style{
		Alignment: &excelize.Alignment{
			Horizontal: "center",
			Vertical:   "center",
		},
		Fill: excelize.Fill{
			Type:    "pattern",
			Color:   []string{"#0190A0"},
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

	dbDataTA := gormdb.Databases.TA
	if dbDataTA == nil {
		idText = "Gagal terhubung ke DB"
		idText = "Failed to connect to DB"
		return
	}

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
		"jsonrpc": config.WebPanel.Get().ApiODOO.JSONRPC,
		"params":  odooParams,
	}
	payloadBytes, err := json.Marshal(payload)
	if err != nil {
		idText = fmt.Sprintf("Gagal membuat payload JSON: %v", err)
		enText = fmt.Sprintf("Failed to create JSON payload: %v", err)
		return
	}
	ODOOresponse, err := GetODOOMSData(string(payloadBytes))
	if err != nil {
		idText = fmt.Sprintf("Gagal mendapatkan data dari ODOO: %v", err)
		enText = fmt.Sprintf("Failed to get data from ODOO: %v", err)
		return
	}
	ODOOResponseArray, ok := ODOOresponse.([]interface{})
	if !ok {
		idText = "Gagal mengkonversi data ODOO ke array"
		enText = "Failed to convert ODOO data to array"
		return
	}

	ODOOResponseBytes, err := json.Marshal(ODOOResponseArray)
	if err != nil {
		idText = fmt.Sprintf("Gagal mengkonversi data ODOO ke JSON: %v", err)
		enText = fmt.Sprintf("Failed to convert ODOO data to JSON: %v", err)
		return
	}

	var employeeData []ODOOMSTechnicianItem
	if err := json.Unmarshal(ODOOResponseBytes, &employeeData); err != nil {
		idText = fmt.Sprintf("Gagal meng-unmarshal data ODOO: %v", err)
		enText = fmt.Sprintf("Failed to unmarshal ODOO data: %v", err)
		return
	}

	if len(employeeData) == 0 {
		logrus.Warn("No technician data found in ODOO")
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

	titleAwaitingNextResponse := []struct {
		Title string
		Size  float64
	}{
		{"ID Task", 25},
		{"Company", 25},
		{"WO Number", 20},
		{"Ticket Subject", 20},
		{"SPK Received at", 20},
		{"Type", 20},
		{"Type2", 20},
		{"SLA", 20},
		{"Keterangan", 20},
		{"Description", 20},
		{"Reason Code", 20},
		{"Merchant", 20},
		{"Ops Head", 20},
		{"SPL", 20},
		{"Technician", 20},
		{"MID", 20},
		{"TID", 20},
		{"Date in Dashboard TA", 20},
		{"TA Oncheck", 20},
		{"Problem", 20},
		{"TA Feedback", 20},
		{"Case in Technician", 20},
	}

	var columnsAwaitingNextResponse []ExcelColumn
	for i, t := range titleAwaitingNextResponse {
		columnsAwaitingNextResponse = append(columnsAwaitingNextResponse, ExcelColumn{
			ColIndex: fun.GetColName(i),
			ColTitle: t.Title,
			ColSize:  t.Size,
		})
	}
	for _, col := range columnsAwaitingNextResponse {
		cell := fmt.Sprintf("%s2", col.ColIndex)
		f.SetCellValue(sheetTAFeedbackAwaitingNextAction, cell, col.ColTitle)
		f.SetColWidth(sheetTAFeedbackAwaitingNextAction, col.ColIndex, col.ColIndex, col.ColSize)
		f.SetCellStyle(sheetTAFeedbackAwaitingNextAction, cell, cell, styleTitle)
	}
	lastColAwaitingNextResponse := fun.GetColName(len(columnsAwaitingNextResponse) - 1)
	filterRangeAwaitingNextResponse := fmt.Sprintf("A2: %s2", lastColAwaitingNextResponse)
	f.AutoFilter(sheetTAFeedbackAwaitingNextAction, filterRangeAwaitingNextResponse, []excelize.AutoFilterOptions{})

	var DataTAFeedbackedAwaitingNextActionError []tamodel.Error
	var DataTAFeedbackedAwaitingNextActionPending []tamodel.Pending
	if err := dbDataTA.Model(&tamodel.Error{}).
		Where("(date_on_check < ? OR date_on_check >= ?) AND ta_feedback IS NOT NULL", startOfDay, endOfDay).
		Find(&DataTAFeedbackedAwaitingNextActionError).Error; err != nil {
		idText = fmt.Sprintf("Gagal mengambil data error TA feedbacked awaiting next action: %v", err)
		enText = fmt.Sprintf("Failed to fetch TA feedbacked awaiting next action error data: %v", err)
		return
	}
	if err := dbDataTA.Model(&tamodel.Pending{}).
		Where("(date_on_check < ? OR date_on_check >= ?) AND ta_feedback IS NOT NULL", startOfDay, endOfDay).
		Find(&DataTAFeedbackedAwaitingNextActionPending).Error; err != nil {
		idText = fmt.Sprintf("Gagal mengambil data pending TA feedbacked awaiting next action: %v", err)
		enText = fmt.Sprintf("Failed to fetch TA feedbacked awaiting next action pending data: %v", err)
		return
	}

	rowIndex := 3
	if len(DataTAFeedbackedAwaitingNextActionError) > 0 {
		for _, record := range DataTAFeedbackedAwaitingNextActionError {
			for _, column := range columnsAwaitingNextResponse {
				cell := fmt.Sprintf("%s%d", column.ColIndex, rowIndex)
				var value interface{} = "N/A"

				var needToSetValue bool = true

				switch column.ColTitle {
				case "ID Task":
					value = record.IDTask
				case "Company":
					value = record.Company
				case "WO Number":
					value = record.WoNumber
				case "Ticket Subject":
					value = record.SpkNumber
				case "SPK Received at":
					value = record.ReceivedDatetimeSpk
				case "Type":
					if record.Type != nil {
						value = *record.Type
					}
				case "Type2":
					if record.Type2 != nil {
						value = *record.Type2
					}
				case "SLA":
					if record.Sla != nil {
						value = *record.Sla
					}
				case "Keterangan":
					if record.Keterangan != nil {
						value = *record.Keterangan
					}
				case "Description":
					if record.Desc != nil {
						value = *record.Desc
					}
				case "Reason Code":
					value = record.Reason
				case "Merchant":
					if record.Merchant != nil {
						value = *record.Merchant
					}
				case "SPL":
					needToSetValue = false
					formula := fmt.Sprintf(`IFERROR(VLOOKUP(O%d, %v!A:C, 2, FALSE), "N/A")`, rowIndex, sheetEmployee)
					f.SetCellFormula(sheetTAFeedbackAwaitingNextAction, cell, formula)
				case "Ops Head":
					needToSetValue = false
					formula := fmt.Sprintf(`IFERROR(VLOOKUP(O%d, %v!A:C, 3, FALSE), "N/A")`, rowIndex, sheetEmployee)
					f.SetCellFormula(sheetTAFeedbackAwaitingNextAction, cell, formula)
				case "Technician":
					value = record.Teknisi
				case "MID":
					value = record.MID
				case "TID":
					value = record.TID
				case "Date in Dashboard TA":
					value = record.Date.Format("2006-01-02 15:04:05")
				case "TA Oncheck":
					if !record.DateOnCheck.IsZero() {
						value = record.DateOnCheck.Format("2006-01-02 15:04:05")
					}
				case "Problem":
					if record.Problem != nil {
						value = *record.Problem
					}
				case "TA Feedback":
					value = record.TaFeedback
				case "Case in Technician":
					value = "Error"
				}

				if needToSetValue {
					f.SetCellValue(sheetTAFeedbackAwaitingNextAction, cell, value)
					f.SetCellStyle(sheetTAFeedbackAwaitingNextAction, cell, cell, style)
				}
			}
			rowIndex++
		}
	}
	if len(DataTAFeedbackedAwaitingNextActionPending) > 0 {
		for _, record := range DataTAFeedbackedAwaitingNextActionPending {
			for _, column := range columnsAwaitingNextResponse {
				cell := fmt.Sprintf("%s%d", column.ColIndex, rowIndex)
				var value interface{} = "N/A"

				var needToSetValue bool = true

				switch column.ColTitle {
				case "ID Task":
					value = record.IDTask
				case "Company":
					value = record.Company
				case "WO Number":
					value = record.WoNumber
				case "Ticket Subject":
					value = record.SpkNumber
				case "SPK Received at":
					value = record.ReceivedDatetimeSpk
				case "Type":
					if record.Type != nil {
						value = *record.Type
					}
				case "Type2":
					if record.Type2 != nil {
						value = *record.Type2
					}
				case "SLA":
					if record.Sla != nil {
						value = *record.Sla
					}
				case "Keterangan":
					if record.Keterangan != nil {
						value = *record.Keterangan
					}
				case "Description":
					if record.Desc != nil {
						value = *record.Desc
					}
				case "Reason Code":
					value = record.Reason
				case "Merchant":
					if record.Merchant != nil {
						value = *record.Merchant
					}
				case "SPL":
					needToSetValue = false
					formula := fmt.Sprintf(`IFERROR(VLOOKUP(O%d, %v!A:C, 2, FALSE), "N/A")`, rowIndex, sheetEmployee)
					f.SetCellFormula(sheetTAFeedbackAwaitingNextAction, cell, formula)
				case "Ops Head":
					needToSetValue = false
					formula := fmt.Sprintf(`IFERROR(VLOOKUP(O%d, %v!A:C, 3, FALSE), "N/A")`, rowIndex, sheetEmployee)
					f.SetCellFormula(sheetTAFeedbackAwaitingNextAction, cell, formula)
				case "Technician":
					value = record.Teknisi
				case "MID":
					value = record.MID
				case "TID":
					value = record.TID
				case "Date in Dashboard TA":
					value = record.Date.Format("2006-01-02 15:04:05")
				case "TA Oncheck":
					if !record.DateOnCheck.IsZero() {
						value = record.DateOnCheck.Format("2006-01-02 15:04:05")
					}
				case "Problem":
					value = "N/A"
				case "TA Feedback":
					value = record.TaFeedback
				case "Case in Technician":
					value = "Pending"
				}

				if needToSetValue {
					f.SetCellValue(sheetTAFeedbackAwaitingNextAction, cell, value)
					f.SetCellStyle(sheetTAFeedbackAwaitingNextAction, cell, cell, style)
				}
			}
			rowIndex++
		}
	}
	f.SetCellValue(sheetTAFeedbackAwaitingNextAction, "A1", "Master data of JOs already feedbacked by TA and awaiting next action")

	pivotAwaitingNextActionDataRange := fmt.Sprintf("%s!$A$2:$%s$%d", sheetTAFeedbackAwaitingNextAction, lastColAwaitingNextResponse, rowIndex-1)
	pivotAwaitingNextActionRange := fmt.Sprintf("%s!A8:Y200", sheetPIVOTAwaitingNextAction)
	err = f.AddPivotTable(&excelize.PivotTableOptions{
		Name:            sheetPIVOTAwaitingNextAction,
		DataRange:       pivotAwaitingNextActionDataRange,
		PivotTableRange: pivotAwaitingNextActionRange,
		Rows: []excelize.PivotTableField{
			{Data: "TA Oncheck"},
		},
		Columns: []excelize.PivotTableField{
			{Data: "Ops Head"},
		},
		Data: []excelize.PivotTableField{
			{Data: "WO Number", Subtotal: "count", Name: "Count of JOs already feedbacked by TA and Awaiting Next Action"},
		},
		Filter: []excelize.PivotTableField{
			{Data: "Type"},
			{Data: "Type2"},
			{Data: "MID"},
			{Data: "TID"},
			{Data: "SPL"},
			{Data: "Technician"},
		},
		RowGrandTotals:      true,
		ColGrandTotals:      true,
		ShowDrill:           true,
		ShowRowHeaders:      true,
		ShowColHeaders:      true,
		ShowLastColumn:      true,
		PivotTableStyleName: "PivotStyleLight10",
	})
	if err != nil {
		idText = fmt.Sprintf("Gagal membuat pivot table: %v", err)
		enText = fmt.Sprintf("Failed to create pivot table: %v", err)
		return
	}
	f.SetColWidth(sheetPIVOTAwaitingNextAction, "A", "A", 55)
	// f.SetColWidth(sheetPIVOTAwaitingNextAction, "B", "B", 18)
	f.SetCellValue(sheetPIVOTAwaitingNextAction, "C1", fmt.Sprintf("Total Data Needing a Reply: %d", (len(DataTAFeedbackedAwaitingNextActionError)+len(DataTAFeedbackedAwaitingNextActionPending))))

	titleTechNeedToResolve := []struct {
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

	var columnsTechNeedToResolve []ExcelColumn
	for i, t := range titleTechNeedToResolve {
		columnsTechNeedToResolve = append(columnsTechNeedToResolve, ExcelColumn{
			ColIndex: fun.GetColName(i),
			ColTitle: t.Title,
			ColSize:  t.Size,
		})
	}
	for _, col := range columnsTechNeedToResolve {
		cell := fmt.Sprintf("%s2", col.ColIndex)
		f.SetCellValue(sheetTechnicianNeedToResolve, cell, col.ColTitle)
		f.SetColWidth(sheetTechnicianNeedToResolve, col.ColIndex, col.ColIndex, col.ColSize)
		f.SetCellStyle(sheetTechnicianNeedToResolve, cell, cell, styleTitle)
	}
	lastColTechNeedToResolve := fun.GetColName(len(columnsTechNeedToResolve) - 1)
	filterRangeTechNeedToResolve := fmt.Sprintf("A2: %s2", lastColTechNeedToResolve)
	f.AutoFilter(sheetTechnicianNeedToResolve, filterRangeTechNeedToResolve, []excelize.AutoFilterOptions{})

	var taActivityData []tamodel.LogAct
	err = dbDataTA.
		Where("date BETWEEN ? AND ?", startOfDay, endOfDay).
		Where("LOWER(type_case) = ?", "error").
		Where("LOWER(method) = ?", "edit").
		Find(&taActivityData).
		Error
	if err != nil {
		idText = "Gagal mengambil data aktivitas TA"
		enText = "Failed to fetch TA activity data"
		return
	}

	rowIndex = 3
	UserTA := config.WebPanel.Get().UserTA
	if len(taActivityData) > 0 {
		for _, record := range taActivityData {
			for _, column := range columnsTechNeedToResolve {
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
					formula := fmt.Sprintf(`IFERROR(VLOOKUP(G%d, %v!A:C, 2, FALSE), "N/A")`, rowIndex, sheetEmployee)
					f.SetCellFormula(sheetTechnicianNeedToResolve, cell, formula)
				case "Head":
					needToSetValue = false
					formula := fmt.Sprintf(`IFERROR(VLOOKUP(G%d, %v!A:C, 3, FALSE), "N/A")`, rowIndex, sheetEmployee)
					f.SetCellFormula(sheetTechnicianNeedToResolve, cell, formula)
				case "WO Number":
					if record.Wo != nil && *record.Wo != "" && *record.Wo != "0" {
						wo := *record.Wo
						link := fmt.Sprintf("http://smartwebindonesia.com:3405/projectTask/detailWO?wo_number=%s", wo)
						f.SetCellHyperLink(sheetTechnicianNeedToResolve, cell, link, "External")
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
							f.SetCellValue(sheetTechnicianNeedToResolve, cell, value)
							f.SetCellStyle(sheetTechnicianNeedToResolve, cell, cell, styleID)
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
					f.SetCellValue(sheetTechnicianNeedToResolve, cell, value)
					f.SetCellStyle(sheetTechnicianNeedToResolve, cell, cell, style)
				}
			}
			rowIndex++
		}
	}
	f.SetCellValue(sheetTechnicianNeedToResolve, "A1", "This sheet contains data about technician mismatches and items that need to be resolved next time if the same problem occurs.")

	pivotTechNeedToResolveDataRange := fmt.Sprintf("%s!$A$2:$%s$%d", sheetTechnicianNeedToResolve, lastColTechNeedToResolve, rowIndex-1)
	pivotTechMajorProblemRange := fmt.Sprintf("%s!A8:Y200", sheetPIVOTTechnicianMajorProblem)
	err = f.AddPivotTable(&excelize.PivotTableOptions{
		Name:            sheetPIVOTTechnicianMajorProblem,
		DataRange:       pivotTechNeedToResolveDataRange,
		PivotTableRange: pivotTechMajorProblemRange,
		Rows: []excelize.PivotTableField{
			{Data: "Problem"},
		},
		Columns: []excelize.PivotTableField{
			{Data: "Head"},
			{Data: "Technician"},
		},
		Data: []excelize.PivotTableField{
			{Data: "WO Number", Subtotal: "count", Name: "Count of Problem JOs"},
		},
		Filter: []excelize.PivotTableField{
			{Data: "Type"},
			{Data: "Type2"},
			{Data: "TID"},
			{Data: "SPL"},
		},
		RowGrandTotals:      true,
		ColGrandTotals:      true,
		ShowDrill:           true,
		ShowRowHeaders:      true,
		ShowColHeaders:      true,
		ShowLastColumn:      true,
		PivotTableStyleName: "PivotStyleLight10",
	})
	if err != nil {
		idText = fmt.Sprintf("Gagal membuat pivot table: %v", err)
		enText = fmt.Sprintf("Failed to create pivot table: %v", err)
		return
	}
	f.SetColWidth(sheetPIVOTTechnicianMajorProblem, "A", "A", 55)

	f.MoveSheet(sheetPIVOTAwaitingNextAction, sheetTAFeedbackAwaitingNextAction)
	excelFilePath = filepath.Join(fileReportDir, reportName)
	if err := f.SaveAs(excelFilePath); err != nil {
		idText = fmt.Sprintf("Gagal menyimpan file excel: %v", err)
		enText = fmt.Sprintf("Failed to save excel: %v", err)
		return
	}

	var sbID strings.Builder
	sbID.WriteString(fmt.Sprintf("🎉 Yeay! Report: _%s_ berhasil digenerate otomatis oleh _system_.\n\n", reportName))
	sbID.WriteString(fmt.Sprintf("📊 Sheet *%s* digunakan untuk melihat total data yang sudah di-feedback oleh tim TA dan memerlukan tindak lanjut terkait JO tersebut.\n", sheetPIVOTAwaitingNextAction))
	sbID.WriteString(fmt.Sprintf("📑 Sheet *%s* adalah master data dari *%s*.\n", sheetTAFeedbackAwaitingNextAction, sheetPIVOTAwaitingNextAction))
	sbID.WriteString(fmt.Sprintf("📑 Sheet *%s* adalah master data dari sheet *%s*.\n", sheetTechnicianNeedToResolve, sheetPIVOTTechnicianMajorProblem))
	sbID.WriteString(fmt.Sprintf("🛠️ Sheet *%s* list masalah yang sering dilakukan oleh teknisi, yang perlu ditinjau kembali agar dipengerjaan selanjutnya dapat mengatasi masalah yang serupa.\n", sheetPIVOTTechnicianMajorProblem))
	idText = sbID.String()

	var sbEN strings.Builder
	sbEN.WriteString(fmt.Sprintf("🎉 Yay! Report: _%s_ has been successfully auto-generated by the _system_.\n\n", reportName))
	sbEN.WriteString(fmt.Sprintf("📊 Sheet *%s* shows the total data that has already received feedback from the TA team and requires further action related to the JO.\n", sheetPIVOTAwaitingNextAction))
	sbEN.WriteString(fmt.Sprintf("📑 Sheet *%s* is the master data for *%s*.\n", sheetTAFeedbackAwaitingNextAction, sheetPIVOTAwaitingNextAction))
	sbEN.WriteString(fmt.Sprintf("📑 Sheet *%s* is the master data for sheet *%s*.\n", sheetTechnicianNeedToResolve, sheetPIVOTTechnicianMajorProblem))
	sbEN.WriteString(fmt.Sprintf("🛠️ Sheet *%s* lists common problems often made by technicians, which should be reviewed so they can be avoided or solved in future work.\n", sheetPIVOTTechnicianMajorProblem))
	enText = sbEN.String()

	return
}

func GetImgPivotReportFirstSheet(excelFilePath, userLang string) (idText string, enText string, imgFilePath string) {
	loc, err := time.LoadLocation(config.WebPanel.Get().Default.Timezone)
	if err != nil {
		return fmt.Sprintf("⚠ Gagal memuat zona %s.", config.WebPanel.Get().Default.Timezone),
			fmt.Sprintf("⚠ Failed to load timezone %s.", config.WebPanel.Get().Default.Timezone),
			""
	}

	now := time.Now().In(loc)

	selectedMainDir, err := fun.FindValidDirectory([]string{
		"web/file/pivot",
		"../web/file/pivot",
		"../../web/file/pivot",
	})
	if err != nil {
		return fmt.Sprintf("⚠ Mohon maaf, gagal untuk generate pivot: %v", err),
			fmt.Sprintf("⚠ Sorry, failed to generate pivot: %v", err), ""
	}

	fileReportDir := filepath.Join(selectedMainDir, time.Now().Format("2006-01-02"))
	if err := os.MkdirAll(fileReportDir, 0755); err != nil {
		return fmt.Sprintf("⚠ Mohon maaf, gagal membuat direktori: %v", err),
			fmt.Sprintf("⚠ Sorry, failed to create directory: %v", err), ""
	}

	imgOutput := fmt.Sprintf("%s/pivotReportFirstSheet_%s.jpg", fileReportDir, now.Format("02Jan2006_15_04_05"))

	logFile, _ := os.Create(config.WebPanel.Get().TechnicalAssistanceData.LogExportPivotDebugPath)
	defer logFile.Close()

	logExportPivot := func(msg string) {
		logFile.WriteString(time.Now().Format("15:04:05") + " " + msg + "\n")
	}

	runtime.LockOSThread()
	defer runtime.UnlockOSThread()

	var tempExcel, tempPDF string

	defer func() {
		if r := recover(); r != nil {
			logExportPivot(fmt.Sprintf("🔥 Panic: %v", r))
		}
		if tempExcel != "" {
			os.Remove(tempExcel)
		}
		if tempPDF != "" {
			os.Remove(tempPDF)
		}
	}()

	logExportPivot(fmt.Sprintf("🔄 Starting exportPivotToImage from excel: %s", excelFilePath))

	// Wait for unlock
	if err = WaitForFileUnlock(excelFilePath, waitForFileUnlockedTimeout); err != nil {
		logExportPivot(fmt.Sprintf("⛔ File unlock failed: %v", err))
		return "⚠ Gagal membuka file karena sedang digunakan.", "⚠ Failed to open file because it's in use.", ""
	}

	logExportPivot("✅ File unlocked")

	f, err := excelize.OpenFile(excelFilePath)
	if err != nil {
		logExportPivot(fmt.Sprintf("⛔ Failed to open Excel: %v", err))
		return "⚠ Gagal membuka file Excel.", "⚠ Failed to open Excel file.", ""
	}

	sheetName := f.GetSheetName(0) // Index where PIVOT exists

	orientation := "landscape"
	paperSize := 8
	fitToWidth := 1
	fitToHeight := 0
	adjustTo := uint(100)

	if err := f.SetPageLayout(sheetName, &excelize.PageLayoutOptions{
		Orientation: &orientation,
		Size:        &paperSize,
		FitToWidth:  &fitToWidth,
		FitToHeight: &fitToHeight,
		AdjustTo:    &adjustTo,
	}); err != nil {
		logExportPivot(fmt.Sprintf("⛔ Failed to set page layout: %v", err))
		return "⚠ Gagal mengatur layout kertas.", "⚠ Failed to set page layout.", ""
	}

	if err := f.Save(); err != nil {
		logExportPivot(fmt.Sprintf("⛔ Failed to save modified Excel: %v", err))
		return "⚠ Gagal menyimpan file Excel.", "⚠ Failed to save Excel file.", ""
	}

	tempDir := os.TempDir()
	tempExcel = filepath.Join(tempDir, fmt.Sprintf("temp_%d.xlsx", time.Now().UnixNano()))
	if err := CopyFile(excelFilePath, tempExcel); err != nil {
		logExportPivot(fmt.Sprintf("⛔ Failed to copy Excel: %v", err))
		return "⚠ Gagal menyalin file Excel.", "⚠ Failed to copy Excel file.", ""
	}

	logExportPivot("✅ Excel copied to: " + tempExcel)

	// Convert to PDF with timeout
	baseName := strings.TrimSuffix(filepath.Base(tempExcel), ".xlsx")
	tempPDF = filepath.Join(tempDir, baseName+".pdf")
	libreCmd := exec.Command("libreoffice", "--headless", "--convert-to", "pdf", "--outdir", tempDir, tempExcel)
	libreCmd.Env = append(os.Environ(), "HOME=/tmp")

	// Set timeout for LibreOffice command
	ctx, cancel := context.WithTimeout(context.Background(), contextCmdTimeout)
	defer cancel()
	libreCmd = exec.CommandContext(ctx, "libreoffice", "--headless", "--convert-to", "pdf", "--outdir", tempDir, tempExcel)
	libreCmd.Env = append(os.Environ(), "HOME=/tmp")

	out, err := libreCmd.CombinedOutput()
	if err != nil {
		if ctx.Err() == context.DeadlineExceeded {
			logExportPivot(fmt.Sprintf("⛔ LibreOffice timeout after %v seconds", contextCmdTimeout))
			return "⚠ Timeout saat mengkonversi Excel ke PDF.", "⚠ Timeout while converting Excel to PDF.", ""
		}
		logExportPivot(fmt.Sprintf("⛔ LibreOffice failed: %v, Output: %s", err, string(out)))
		return "⚠ Gagal mengkonversi Excel ke PDF.", "⚠ Failed to convert Excel to PDF.", ""
	}

	logExportPivot("✅ PDF created: " + tempPDF)

	absImg, _ := filepath.Abs(imgOutput)
	magickPath, err := exec.LookPath("convert")
	if err != nil {
		// fallback
		magickPath = config.WebPanel.Get().Default.MagickFullPath
		if _, statErr := os.Stat(magickPath); os.IsNotExist(statErr) {
			logExportPivot("⛔ ImageMagick not found")
			return "⚠ ImageMagick tidak ditemukan.", "⚠ ImageMagick not found.", ""
		}
	}

	// convertCmd := exec.Command(
	// 	magickPath,
	// 	"-density", "200",
	// 	"-background", "white",
	// 	"-alpha", "remove",
	// 	"-alpha", "off",
	// 	tempPDF+"[0]",
	// 	"-resize", "2000x",
	// 	"-quality", "100",
	// 	absImg,
	// )

	// Set timeout for ImageMagick command
	ctxConvert, cancelConvert := context.WithTimeout(context.Background(), contextCmdTimeout)
	defer cancelConvert()
	convertCmd := exec.CommandContext(
		ctxConvert,
		magickPath,
		"-density", "200",
		"-background", "white",
		"-alpha", "remove",
		"-alpha", "off",
		tempPDF+"[0]",
		"-resize", "2000x",
		"-quality", "100",
		absImg,
	)

	convertOut, err := convertCmd.CombinedOutput()
	if err != nil {
		if ctxConvert.Err() == context.DeadlineExceeded {
			logExportPivot(fmt.Sprintf("⛔ ImageMagick timeout after %v seconds", contextCmdTimeout))
			return "⚠ Timeout saat mengkonversi PDF ke gambar.", "⚠ Timeout while converting PDF to image.", ""
		}
		logExportPivot(fmt.Sprintf("⛔ convert failed: %v, Output: %s", err, string(convertOut)))
		return "⚠ Gagal mengkonversi PDF ke gambar.", "⚠ Failed to convert PDF to image.", ""
	}

	logExportPivot("✅ Image generated: " + absImg)

	return "✅ Berhasil generate gambar pivot report.",
		"✅ Successfully generated pivot report image.",
		absImg
}

func WaitForFileUnlock(filePath string, timeout time.Duration) error {
	start := time.Now()
	for {
		f, err := os.OpenFile(filePath, os.O_RDWR, 0666)
		if err == nil {
			f.Close()
			return nil
		}
		if time.Since(start) > timeout {
			return fmt.Errorf("file still locked after %v: %v", timeout, err)
		}
		time.Sleep(2 * time.Second)
	}
}

func CopyFile(src, dst string) error {
	srcFile, err := os.Open(src)
	if err != nil {
		return err
	}
	defer srcFile.Close()

	dstFile, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer dstFile.Close()

	_, err = io.Copy(dstFile, srcFile)
	return err
}

func ReportTechError(v *events.Message, userLang string) {
	eventToDo := "Generate Technical Assistance Report of Technician Error"
	originalSenderJID := NormalizeSenderJID(v.Info.Sender.String())
	stanzaID := v.Info.ID

	if !generateTechErrorReportMutex.TryLock() {
		id := fmt.Sprintf("⚠ Permintaan *%s* sedang diproses. Mohon tunggu beberapa saat.", eventToDo)
		en := fmt.Sprintf("⚠ Your *%s* request is being processed. Please wait a moment.", eventToDo)
		sendLangMessageWithStanza(v, stanzaID, originalSenderJID, id, en, userLang)
		return
	}
	defer generateTechErrorReportMutex.Unlock()

	// Inform user we've received request
	id, en := informUserRequestReceived(eventToDo)
	sendLangMessageWithStanza(v, stanzaID, originalSenderJID, id, en, userLang)

	// Get Excel Tech Error
	idText, enText, excelFilePath := GetFileReportTechError(userLang)
	if excelFilePath != "" {
		sendLangMessageWithStanza(v, stanzaID, originalSenderJID, idText, enText, userLang)
		mentions := config.WebPanel.Get().TechnicalAssistanceData.ReportTAMentions

		excelFileCaption := "Report Tech Error"
		SendExcelFileWithStanza(v, stanzaID, originalSenderJID, excelFilePath, excelFileCaption, mentions, userLang)

		idText, enText, imgSS := GetImgPivotReportFirstSheet(excelFilePath, userLang)
		if imgSS != "" {
			// sendLangMessageWithStanza(v, stanzaID, originalSenderJID, idText, enText, userLang)
			SendImgFileWithStanza(v, stanzaID, originalSenderJID, imgSS, excelFileCaption, mentions, userLang)

			// 🧹 Cleanup: remove the image and the original Excel file
			if removeErr := os.Remove(imgSS); removeErr != nil {
				logrus.Errorf("⚠ Gagal menghapus file gambar: %v", removeErr)
			} else {
				logrus.Printf("🧹 Gambar %s berhasil dihapus setelah dikirim.", imgSS)
			}

			if removeErr := os.Remove(excelFilePath); removeErr != nil {
				logrus.Errorf("⚠ Gagal menghapus file Excel: %v", removeErr)
			} else {
				logrus.Printf("🧹 Excel file %s berhasil dihapus setelah digunakan.", excelFilePath)
			}
		} else {
			sendLangMessageWithStanza(v, stanzaID, originalSenderJID, idText, enText, userLang)
		}
	} else {
		sendLangMessageWithStanza(v, stanzaID, originalSenderJID, idText, enText, userLang)
	}
}

// Technican Error / Mismatch
func GetFileReportTechError(userLang string) (idText string, enText string, excelFilePath string) {
	// Step 1: Find valid directory to save report
	selectedMainDir, err := fun.FindValidDirectory([]string{
		"web/file/ta_report",
		"../web/file/ta_report",
		"../../web/file/ta_report",
	})
	if err != nil {
		idText = fmt.Sprintf("⚠ Mohon maaf, gagal untuk generate report tech error: %v", err)
		enText = fmt.Sprintf("⚠ Sorry, failed to generate tech error report: %v", err)
		return
	}

	// Step 2: Create dated directory
	fileReportDir := filepath.Join(selectedMainDir, time.Now().Format("2006-01-02"))
	if err := os.MkdirAll(fileReportDir, 0755); err != nil {
		idText = fmt.Sprintf("⚠ Mohon maaf, gagal membuat direktori: %v", err)
		enText = fmt.Sprintf("⚠ Sorry, failed to create directory: %v", err)
		return
	}

	// Step 3: Download the report with timeout
	reportURL := config.WebPanel.Get().TechnicalAssistanceData.PublicURLReportTechError

	// Create HTTP client with timeout
	client := &http.Client{
		Timeout: httpReqTimeout,
	}

	resp, err := client.Get(reportURL)
	if err != nil {
		idText = fmt.Sprintf("❌ Gagal mengakses report dari %s: %v", reportURL, err)
		enText = fmt.Sprintf("❌ Failed to access report from %s: %v", reportURL, err)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		idText = fmt.Sprintf("⚠ Gagal mengambil report, status: %d - %s\n%s", resp.StatusCode, http.StatusText(resp.StatusCode), string(body))
		enText = fmt.Sprintf("⚠ Failed to fetch report, status: %d - %s\n%s", resp.StatusCode, http.StatusText(resp.StatusCode), string(body))
		return
	}

	// Step 4: Extract filename
	filename := "tech_error_download"
	if cd := resp.Header.Get("Content-Disposition"); cd != "" {
		if parts := strings.Split(cd, "filename="); len(parts) == 2 {
			filename = strings.Trim(parts[1], `"`)
		}
	}

	// Step 5: Save file
	filePath := filepath.Join(fileReportDir, filename)
	outFile, err := os.Create(filePath)
	if err != nil {
		idText = fmt.Sprintf("❌ Gagal membuat file di %s: %v", filePath, err)
		enText = fmt.Sprintf("❌ Failed to create file at %s: %v", filePath, err)
		return
	}
	defer outFile.Close()

	_, err = io.Copy(outFile, resp.Body)
	if err != nil {
		idText = fmt.Sprintf("❌ Gagal menyimpan report ke %s: %v", filePath, err)
		enText = fmt.Sprintf("❌ Failed to save report to %s: %v", filePath, err)
		return
	}

	// Step 6: Build success message (same meaning, two languages)
	idText = fmt.Sprintf(
		"🎉 Yeay! Report *%s* berhasil digenerate oleh _system_.\n\nReport ini adalah report list problem yang didapat teknisi saat upload JO sehingga TA perlu melakukan edit terhadap datanya agar sesuai SOP",
		filename)

	enText = fmt.Sprintf(
		"🎉 Yay! The report *%s* was successfully generated by the _system_.\n\nThis report lists the problems found by the technician when uploading the JO, so that the TA team can review and edit the data to comply with SOP.",
		filename)

	// Return file path so caller can attach/send
	excelFilePath = filePath
	return
}

func ReportCompared(v *events.Message, userLang string) {
	eventToDo := "Generate Compared Report"
	originalSenderJID := NormalizeSenderJID(v.Info.Sender.String())
	stanzaID := v.Info.ID

	if !generateReportComparedMutex.TryLock() {
		id := fmt.Sprintf("⚠ Permintaan *%s* sedang diproses. Mohon tunggu beberapa saat.", eventToDo)
		en := fmt.Sprintf("⚠ Your *%s* request is being processed. Please wait a moment.", eventToDo)
		sendLangMessageWithStanza(v, stanzaID, originalSenderJID, id, en, userLang)
		return
	}
	defer generateReportComparedMutex.Unlock()

	// Inform user we've received request
	id, en := informUserRequestReceived(eventToDo)
	sendLangMessageWithStanza(v, stanzaID, originalSenderJID, id, en, userLang)

	// Get Excel
	idText, enText, excelFilePath := GetFileReportCompared(userLang)
	if excelFilePath != "" {
		sendLangMessageWithStanza(v, stanzaID, originalSenderJID, idText, enText, userLang)
		mentions := config.WebPanel.Get().TechnicalAssistanceData.ReportTAMentions

		excelFileCaption := "Report Compared"
		SendExcelFileWithStanza(v, stanzaID, originalSenderJID, excelFilePath, excelFileCaption, mentions, userLang)

		idText, enText, imgSS := GetImgPivotReportFirstSheet(excelFilePath, userLang)
		if imgSS != "" {
			// sendLangMessageWithStanza(v, stanzaID, originalSenderJID, idText, enText, userLang)
			SendImgFileWithStanza(v, stanzaID, originalSenderJID, imgSS, excelFileCaption, mentions, userLang)

			// 🧹 Cleanup: remove the image and the original Excel file
			if removeErr := os.Remove(imgSS); removeErr != nil {
				logrus.Errorf("⚠ Gagal menghapus file gambar: %v", removeErr)
			} else {
				logrus.Printf("🧹 Gambar %s berhasil dihapus setelah dikirim.", imgSS)
			}

			if removeErr := os.Remove(excelFilePath); removeErr != nil {
				logrus.Errorf("⚠ Gagal menghapus file Excel: %v", removeErr)
			} else {
				logrus.Printf("🧹 Excel file %s berhasil dihapus setelah digunakan.", excelFilePath)
			}
		} else {
			sendLangMessageWithStanza(v, stanzaID, originalSenderJID, idText, enText, userLang)
		}
	} else {
		sendLangMessageWithStanza(v, stanzaID, originalSenderJID, idText, enText, userLang)
	}
}

func GetFileReportCompared(userLang string) (idText string, enText string, excelFilePath string) {
	// Step 1: Find valid directory to save report
	selectedMainDir, err := fun.FindValidDirectory([]string{
		"web/file/ta_report",
		"../web/file/ta_report",
		"../../web/file/ta_report",
	})
	if err != nil {
		idText = fmt.Sprintf("⚠ Mohon maaf, gagal untuk generate report compared: %v", err)
		enText = fmt.Sprintf("⚠ Sorry, failed to generate compared report: %v", err)
		return
	}

	// Step 2: Create dated directory
	fileReportDir := filepath.Join(selectedMainDir, time.Now().Format("2006-01-02"))
	if err := os.MkdirAll(fileReportDir, 0755); err != nil {
		idText = fmt.Sprintf("⚠ Mohon maaf, gagal membuat direktori: %v", err)
		enText = fmt.Sprintf("⚠ Sorry, failed to create directory: %v", err)
		return
	}

	// Step 3: Download the report with timeout
	reportURL := config.WebPanel.Get().TechnicalAssistanceData.PublicURLReportCompared

	// Create HTTP client with timeout
	client := &http.Client{
		Timeout: httpReqTimeout,
	}

	resp, err := client.Get(reportURL)
	if err != nil {
		idText = fmt.Sprintf("❌ Gagal mengakses report dari %s: %v", reportURL, err)
		enText = fmt.Sprintf("❌ Failed to access report from %s: %v", reportURL, err)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		idText = fmt.Sprintf("⚠ Gagal mengambil report, status: %d - %s\n%s", resp.StatusCode, http.StatusText(resp.StatusCode), string(body))
		enText = fmt.Sprintf("⚠ Failed to fetch report, status: %d - %s\n%s", resp.StatusCode, http.StatusText(resp.StatusCode), string(body))
		return
	}

	// Step 4: Extract filename
	filename := "compared_report_download"
	if cd := resp.Header.Get("Content-Disposition"); cd != "" {
		if parts := strings.Split(cd, "filename="); len(parts) == 2 {
			filename = strings.Trim(parts[1], `"`)
		}
	}

	// Step 5: Save file
	filePath := filepath.Join(fileReportDir, filename)
	outFile, err := os.Create(filePath)
	if err != nil {
		idText = fmt.Sprintf("❌ Gagal membuat file di %s: %v", filePath, err)
		enText = fmt.Sprintf("❌ Failed to create file at %s: %v", filePath, err)
		return
	}
	defer outFile.Close()

	_, err = io.Copy(outFile, resp.Body)
	if err != nil {
		idText = fmt.Sprintf("❌ Gagal menyimpan report ke %s: %v", filePath, err)
		enText = fmt.Sprintf("❌ Failed to save report to %s: %v", filePath, err)
		return
	}

	// Step 6: Build success message (same meaning, two languages)
	idText = fmt.Sprintf(
		"🎉 Yeay! Report *%s* berhasil digenerate oleh _system_.\n\nReport ini adalah hasil perbandingan data dari TA dan data di Odoo untuk rentang waktu 3 bulan ke belakang hingga bulan ini.",
		filename)

	enText = fmt.Sprintf(
		"🎉 Yay! The report *%s* was successfully generated by the _system_.\n\nThis report compares data from TA and Odoo for the period from 3 months ago up to the current month.",
		filename)

	// Return file path so caller can attach/send
	excelFilePath = filePath
	return
}

func GetDataTaskCompared() error {
	taskDoing := "Get Data for Compared Report"
	if !getDataTaskComparedMutex.TryLock() {
		return fmt.Errorf("%s already running, please wait a moment", taskDoing)
	}
	defer getDataTaskComparedMutex.Unlock()

	loc, _ := time.LoadLocation(config.WebPanel.Get().Default.Timezone)
	now := time.Now().In(loc)

	startOfMonth := time.Date(now.Year(), now.Month()-3, 1, 0, 0, 0, 0, loc)
	endOfMonth := time.Date(now.Year(), now.Month()+1, 0, 23, 59, 59, 999999999, loc)
	startOfMonth = startOfMonth.Add(-7 * time.Hour)
	endOfMonth = endOfMonth.Add(-7 * time.Hour)

	startDateParam := startOfMonth.Format("2006-01-02 15:04:05")
	endDateParam := endOfMonth.Format("2006-01-02 15:04:05")

	if config.WebPanel.Get().Report.Compared.ActiveDebug {
		startDateParam = config.WebPanel.Get().Report.Compared.StartParam
		endDateParam = config.WebPanel.Get().Report.Compared.EndParam
	}

	ODOOModel := "project.task"
	excludedCompany := config.WebPanel.Get().ApiODOO.CompanyExcluded
	domain := []interface{}{
		[]interface{}{"active", "=", true},
		[]interface{}{"company_id", "!=", excludedCompany},
		[]interface{}{"x_received_datetime_spk", ">=", startDateParam},
		[]interface{}{"x_received_datetime_spk", "<=", endDateParam},
	}

	fieldsID := []string{
		"id",
	}

	fields := []string{
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

	order := "id asc"
	odooParams := map[string]interface{}{
		"model":  ODOOModel,
		"domain": domain,
		"fields": fieldsID,
		"order":  order,
	}

	payload := map[string]interface{}{
		"jsonrpc": config.WebPanel.Get().ApiODOO.JSONRPC,
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
		return errors.New("empty data in ODOO")
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
			logrus.Debugf("Appending %d records from chunk %d", len(result.records), i)
			allRecords = append(allRecords, result.records...)
		}
	}

	logrus.Debugf("Finished processing all chunks, total records collected: %d", len(allRecords))
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
	logrus.Infof("Memory usage before DB operations - Allocated: %d MB, System: %d MB",
		memStats.Alloc/1024/1024, memStats.Sys/1024/1024)

	// Force garbage collection to free up memory before database operations
	runtime.GC()

	// Use a single transaction for all database operations to improve performance
	tx := dbWeb.Begin()
	if tx.Error != nil {
		return fmt.Errorf("failed to begin transaction: %v", tx.Error)
	}

	// Clear table with better performance using TRUNCATE if possible
	if err := tx.Unscoped().Where("id != ?", 0).Delete(&reportmodel.TaskComparedData{}).Error; err != nil {
		tx.Rollback()
		return fmt.Errorf("failed to clear TaskComparedData table: %v", err)
	}

	// Process data in smaller batches to prevent memory issues
	const dbBatchSize = 1000 // Reduced from 2000 to prevent memory pressure
	for i := 0; i < len(listOfData); i += dbBatchSize {
		end := i + dbBatchSize
		if end > len(listOfData) {
			end = len(listOfData)
		}

		var batch []reportmodel.TaskComparedData
		// Pre-allocate slice to avoid repeated memory allocations
		batch = make([]reportmodel.TaskComparedData, 0, end-i)

		for _, data := range listOfData[i:end] {
			// Use optimized parsing function that doesn't log errors for better performance
			_, technicianName := parseJSONIDDataCombinedSafe(data.TechnicianId)
			_, edcType := parseJSONIDDataCombinedSafe(data.EdcType)
			_, snEdc := parseJSONIDDataCombinedSafe(data.SnEdc)
			_, companyName := parseJSONIDDataCombinedSafe(data.CompanyId)
			_, stage := parseJSONIDDataCombinedSafe(data.StageId)
			_, worksheetTemplate := parseJSONIDDataCombinedSafe(data.WorksheetTemplateId)
			_, ticketSubject := parseJSONIDDataCombinedSafe(data.HelpdeskTicketId)
			_, reasonCode := parseJSONIDDataCombinedSafe(data.ReasonCodeId)
			_, lastUpdateBy := parseJSONIDDataCombinedSafe(data.WriteUid)
			_, ticketType2 := parseJSONIDDataCombinedSafe(data.TicketTypeId)

			// Use direct assignments for better performance
			var slaDeadline, createDate, receivedDatetimeSpk, planDate, timesheetLastStop, dateLastStageUpdated *time.Time
			if data.SlaDeadline.Valid {
				slaDeadline = &data.SlaDeadline.Time
			}
			if data.ReceivedDatetimeSpk.Valid {
				receivedDatetimeSpk = &data.ReceivedDatetimeSpk.Time
			}
			if data.CreateDate.Valid {
				createDate = &data.CreateDate.Time
			}
			if data.PlanDate.Valid {
				planDate = &data.PlanDate.Time
			}
			if data.TimesheetLastStop.Valid {
				timesheetLastStop = &data.TimesheetLastStop.Time
			}
			if data.DateLastStageUpdate.Valid {
				dateLastStageUpdated = &data.DateLastStageUpdate.Time
			}

			batch = append(batch, reportmodel.TaskComparedData{
				ID:                  uint(data.ID),
				Merchant:            data.MerchantName.String,
				PicMerchant:         data.PicMerchant.String,
				PicPhone:            data.PicPhone.String,
				MerchantAddress:     data.MerchantAddress.String,
				Description:         data.Description.String,
				SLADeadline:         slaDeadline,
				CreateDate:          createDate,
				ReceivedDatetimeSpk: receivedDatetimeSpk,
				PlanDate:            planDate,
				TimesheetLastStop:   timesheetLastStop,
				TaskType:            data.TaskType.String,
				WorksheetTemplate:   worksheetTemplate,
				TicketType2:         ticketType2,
				Company:             companyName,
				Stage:               stage,
				TicketSubject:       ticketSubject,
				MID:                 data.Mid.String,
				TID:                 data.Tid.String,
				Source:              data.Source.String,
				MessageCallCenter:   data.MessageCC.String,
				WONumber:            data.WoNumber,
				StatusMerchant:      data.StatusMerchant.String,
				SNEDC:               snEdc,
				EDCType:             edcType,
				WORemark:            data.WoRemarkTiket.String,
				Longitude:           data.Longitude.String,
				Latitude:            data.Latitude.String,
				Technician:          technicianName,
				ReasonCode:          reasonCode,
				LastUpdateBy:        lastUpdateBy,
				DateLastStageUpdate: dateLastStageUpdated,
			})
		}

		if err := tx.Model(&reportmodel.TaskComparedData{}).Create(batch).Error; err != nil {
			if rollbackErr := tx.Rollback().Error; rollbackErr != nil {
				logrus.Errorf("Failed to rollback transaction: %v", rollbackErr)
			}
			return fmt.Errorf("failed to insert batch of (%s) data to DB: %v", taskDoing, err)
		}

		// Log progress and force garbage collection periodically
		if (i/dbBatchSize)%5 == 0 { // Every 5 batches
			var memStats runtime.MemStats
			runtime.ReadMemStats(&memStats)
			logrus.Infof("Progress: processed %d/%d records, Memory: %d MB",
				end, len(listOfData), memStats.Alloc/1024/1024)
			runtime.GC() // Force garbage collection to free memory
		}
	}

	// Commit the transaction
	if err := tx.Commit().Error; err != nil {
		if rollbackErr := tx.Rollback().Error; rollbackErr != nil {
			logrus.Errorf("Failed to rollback transaction after commit failure: %v", rollbackErr)
		}
		return fmt.Errorf("failed to commit transaction: %v", err)
	}

	return nil
}

func ReportComparedGenerated(v *events.Message, userLang string) {
	eventToDo := "Generate Compared Report"
	originalSenderJID := NormalizeSenderJID(v.Info.Sender.String())
	stanzaID := v.Info.ID

	if !generateReportComparedMutex.TryLock() {
		id := fmt.Sprintf("⚠ Permintaan *%s* sedang diproses. Mohon tunggu beberapa saat.", eventToDo)
		en := fmt.Sprintf("⚠ Your *%s* request is being processed. Please wait a moment.", eventToDo)
		sendLangMessageWithStanza(v, stanzaID, originalSenderJID, id, en, userLang)
		return
	}
	defer generateReportComparedMutex.Unlock()

	// Inform user we've received request
	id, en := informUserRequestReceived(eventToDo)
	sendLangMessageWithStanza(v, stanzaID, originalSenderJID, id, en, userLang)

	id = "🔄 Sedang mengambil data untuk report dibandingkan, Mohon bersabar..."
	en = "🔄 Fetching data for compared report, Please wait a moment..."
	sendLangMessageWithStanza(v, stanzaID, originalSenderJID, id, en, userLang)

	err := GetDataTaskCompared()
	if err != nil {
		idText := fmt.Sprintf("❌ Gagal mengambil data untuk report dibandingkan: %v", err)
		enText := fmt.Sprintf("❌ Failed to fetch data for compared report: %v", err)
		sendLangMessageWithStanza(v, stanzaID, originalSenderJID, idText, enText, userLang)
		return
	}

	id = "✅ Berhasil mengambil data untuk report dibandingkan. Data akan disimpan di database."
	en = "✅ Successfully fetched data for compared report. Data will be saved to the database."
	sendLangMessageWithStanza(v, stanzaID, originalSenderJID, id, en, userLang)

	// Create Excel Report
	loc, err := time.LoadLocation(config.WebPanel.Get().Default.Timezone)
	if err != nil {
		id := fmt.Sprintf("⚠ Gagal memuat zona %s.", config.WebPanel.Get().Default.Timezone)
		en := fmt.Sprintf("⚠ Failed to load timezone %s.", config.WebPanel.Get().Default.Timezone)
		sendLangMessageWithStanza(v, stanzaID, originalSenderJID, id, en, userLang)
		return
	}

	now := time.Now().In(loc)

	reports := []ComparedReportDataExcel{
		{
			ExcelName: "Current_Month",
			StartDate: time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, loc),
			EndDate:   time.Date(now.Year(), now.Month()+1, 0, 23, 59, 59, 999999999, loc),
		},
		{
			ExcelName: "Last_Month",
			StartDate: time.Date(now.Year(), now.Month()-1, 1, 0, 0, 0, 0, loc),
			EndDate:   time.Date(now.Year(), now.Month(), 0, 23, 59, 59, 999999999, loc),
		},
		{
			ExcelName: "2_Months_Ago",
			StartDate: time.Date(now.Year(), now.Month()-2, 1, 0, 0, 0, 0, loc),
			EndDate:   time.Date(now.Year(), now.Month()-1, 0, 23, 59, 59, 999999999, loc),
		},
		{
			ExcelName: "3_Months_Ago",
			StartDate: time.Date(now.Year(), now.Month()-3, 1, 0, 0, 0, 0, loc),
			EndDate:   time.Date(now.Year(), now.Month()-2, 0, 23, 59, 59, 999999999, loc),
		},
	}

	selectedMainDir, err := fun.FindValidDirectory([]string{
		"web/file/ta_report",
		"../web/file/ta_report",
		"../../web/file/ta_report",
	})
	if err != nil {
		id := fmt.Sprintf("⚠ Mohon maaf, gagal untuk generate report perbandingan : %v", err)
		en := fmt.Sprintf("⚠ Sorry, failed to generate Compared Report : %v", err)
		sendLangMessageWithStanza(v, stanzaID, originalSenderJID, id, en, userLang)
		return
	}

	id = "🔄 Sedang membuat file Excel report dibandingkan, Mohon bersabar..."
	en = "🔄 Creating Excel compared report files, Please wait a moment..."
	sendLangMessageWithStanza(v, stanzaID, originalSenderJID, id, en, userLang)

	// Generate multiple Excel files
	excelFiles, err := GenerateMultipleComparedReports(reports, selectedMainDir)
	if err != nil {
		id := fmt.Sprintf("❌ Gagal membuat excel files: %v", err)
		en := fmt.Sprintf("❌ Failed to create excel files: %v", err)
		sendLangMessageWithStanza(v, stanzaID, originalSenderJID, id, en, userLang)
		return
	}

	if len(excelFiles) > 0 {
		id = fmt.Sprintf("🎉 Berhasil membuat %d file Excel report perbandingan data ODOO dengan data yang masih ada di dashboard TA!", len(excelFiles))
		en = fmt.Sprintf("🎉 Successfully created %d Excel compared report files between ODOO data and data remaining in the TA dashboard!", len(excelFiles))
		sendLangMessageWithStanza(v, stanzaID, originalSenderJID, id, en, userLang)

		// Send all Excel files
		mentions := config.WebPanel.Get().TechnicalAssistanceData.ReportTAMentions
		SendMultipleExcelFilesReportCompared(v, stanzaID, originalSenderJID, excelFiles, mentions, userLang)

		// 🧹 Cleanup: remove the Excel files after sending
		for _, filePath := range excelFiles {
			if removeErr := os.Remove(filePath); removeErr != nil {
				logrus.Errorf("⚠ Gagal menghapus file Excel: %v", removeErr)
			} else {
				logrus.Printf("🧹 Excel file %s berhasil dihapus setelah digunakan.", filePath)
			}
		}
		logrus.Printf("🧹 %d file Excel berhasil dihapus setelah digunakan.", len(excelFiles))
	} else {
		id = "⚠ Tidak ada file Excel yang berhasil dibuat."
		en = "⚠ No Excel files were created successfully."
		sendLangMessageWithStanza(v, stanzaID, originalSenderJID, id, en, userLang)
	}
}

// GenerateMultipleComparedReports creates multiple Excel files based on date ranges
func GenerateMultipleComparedReports(reports []ComparedReportDataExcel, baseDir string) ([]string, error) {
	var excelFiles []string

	// Create dated directory
	fileReportDir := filepath.Join(baseDir, time.Now().Format("2006-01-02"))
	if err := os.MkdirAll(fileReportDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create directory: %v", err)
	}

	for _, report := range reports {
		excelFilePath, err := GenerateSingleComparedReport(report, fileReportDir)
		if err != nil {
			logrus.Errorf("Failed to generate report %s: %v", report.ExcelName, err)
			continue
		}
		excelFiles = append(excelFiles, excelFilePath)
	}

	return excelFiles, nil
}

// GenerateSingleComparedReport creates a single Excel file for a specific date range
func GenerateSingleComparedReport(report ComparedReportDataExcel, outputDir string) (string, error) {
	// Query data from database based on date range
	var taskData []reportmodel.TaskComparedData

	// Query data within the date range using WebTA database
	err := dbWeb.Model(&reportmodel.TaskComparedData{}).Where("received_datetime_spk >= ? AND received_datetime_spk <= ?",
		report.StartDate, report.EndDate).Find(&taskData).Error
	if err != nil {
		return "", fmt.Errorf("failed to query data for %s: %v", report.ExcelName, err)
	}

	if len(taskData) == 0 {
		logrus.Warnf("No data found for report received datetime spk %s between %v and %v",
			report.ExcelName, report.StartDate, report.EndDate)
		return "", fmt.Errorf("no data found for report received datetime spk %s between %v and %v",
			report.ExcelName, report.StartDate, report.EndDate)
	}

	// Create Excel file
	f := excelize.NewFile()
	defer func() {
		if err := f.Close(); err != nil {
			logrus.Errorf("Failed to close Excel file: %v", err)
		}
	}()

	sheetMaster := "PROJECT.TASK"
	sheetEmployee := "EMPLOYEES"
	sheetTALeftData := "LEFT DATA TA (ERROR & PENDING)"
	sheetPVTCompared := "PIVOT - DATA COMPARED"

	f.NewSheet(sheetPVTCompared)
	f.NewSheet(sheetEmployee)
	f.NewSheet(sheetMaster)
	f.NewSheet(sheetTALeftData)

	/* Styles */
	styleTitle, err := f.NewStyle(&excelize.Style{
		Alignment: &excelize.Alignment{
			Horizontal: "center",
			Vertical:   "center",
		},
		Fill: excelize.Fill{
			Type:    "pattern",
			Color:   []string{"#FFD000"},
			Pattern: 1,
		},
		Font: &excelize.Font{
			Bold:  true,
			Color: "#000000",
		},
	})
	if err != nil {
		return "", fmt.Errorf("failed to create style for master title: %v", err)
	}

	style, _ := f.NewStyle(&excelize.Style{
		Alignment: &excelize.Alignment{
			Horizontal: "center",
			Vertical:   "center",
		},
	})

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
		cell := fmt.Sprintf("%s1", column.ColIndex)
		f.SetCellValue(sheetEmployee, cell, column.ColTitle)
		f.SetColWidth(sheetEmployee, column.ColIndex, column.ColIndex, column.ColSize)
		f.SetCellStyle(sheetEmployee, cell, cell, styleTitle)
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
		"jsonrpc": config.WebPanel.Get().ApiODOO.JSONRPC,
		"params":  odooParams,
	}
	payloadBytes, err := json.Marshal(payload)
	if err != nil {
		return "", fmt.Errorf("gagal mengkonversi payload ke JSON: %v", err)
	}
	ODOOresponse, err := GetODOOMSData(string(payloadBytes))
	if err != nil {
		return "", fmt.Errorf("gagal mengambil data teknisi dari ODOO: %v", err)
	}
	ODOOResponseArray, ok := ODOOresponse.([]interface{})
	if !ok {
		return "", fmt.Errorf("gagal mengkonversi data ODOO ke array: %v", ODOOresponse)
	}

	ODOOResponseBytes, err := json.Marshal(ODOOResponseArray)
	if err != nil {
		return "", fmt.Errorf("gagal mengkonversi data ODOO ke JSON: %v", err)
	}

	var employeeData []ODOOMSTechnicianItem
	if err := json.Unmarshal(ODOOResponseBytes, &employeeData); err != nil {
		return "", fmt.Errorf("gagal mengkonversi data ODOO ke struct: %v", err)
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
		Master
	*/
	titlesMaster := []struct {
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
	var columnsMaster []ExcelColumn
	for i, t := range titlesMaster {
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
		f.SetCellStyle(sheetMaster, cell, cell, styleTitle)
	}
	// Set autofilter for the master sheet
	lastColMaster := fun.GetColName(len(columnsMaster) - 1)
	filterRangeMaster := fmt.Sprintf("A1:%s1", lastColMaster)
	f.AutoFilter(sheetMaster, filterRangeMaster, []excelize.AutoFilterOptions{})

	var technicianColIndex string
	for _, col := range columnsMaster {
		if col.ColTitle == "Technician" {
			technicianColIndex = col.ColIndex
			break
		}
	}

	// Fill data into the master sheet
	rowIndex := 2
	for _, record := range taskData {
		for _, column := range columnsMaster {
			cell := fmt.Sprintf("%s%d", column.ColIndex, rowIndex)
			var value interface{} = "N/A" // Default value if nothing is set
			var needToSetValue bool = true

			switch column.ColTitle {
			case "ID":
				value = record.ID
			case "WO Number":
				if record.WONumber != "" {
					value = record.WONumber
				}
			case "Ticket Subject":
				if record.TicketSubject != "" {
					value = CleanSPKNumber(record.TicketSubject)
				}
			case "Stage":
				if record.Stage != "" {
					value = record.Stage
				}
			case "Head":
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
			case "Merchant Name":
				if record.Merchant != "" {
					value = record.Merchant
				}
			case "PIC Merchant":
				if record.PicMerchant != "" {
					value = record.PicMerchant
				}
			case "PIC Phone":
				if record.PicPhone != "" {
					value = record.PicPhone
				}
			case "Merchant Address":
				if record.MerchantAddress != "" {
					value = record.MerchantAddress
				}
			case "Description":
				if record.Description != "" {
					value = record.Description
				}
			case "SLA Deadline":
				if record.SLADeadline != nil {
					value = formatTimePointer(record.SLADeadline)
				}
			case "Create Date":
				if record.CreateDate != nil {
					value = formatTimePointer(record.CreateDate)
				}
			case "Received Datetime SPK":
				if record.ReceivedDatetimeSpk != nil {
					value = formatTimePointer(record.ReceivedDatetimeSpk)
				}
			case "Planned At":
				if record.PlanDate != nil {
					value = formatTimePointer(record.PlanDate)
				}
			case "Timesheet Last Stop":
				if record.TimesheetLastStop != nil {
					value = formatTimePointer(record.TimesheetLastStop)
				}
			case "Task Type":
				if record.TaskType != "" {
					value = record.TaskType
				}
			case "Worksheet Template":
				if record.WorksheetTemplate != "" {
					value = record.WorksheetTemplate
				}
			case "Ticket Type":
				if record.TicketType2 != "" {
					value = record.TicketType2
				}
			case "Company":
				if record.Company != "" {
					value = record.Company
				}
			case "MID":
				if record.MID != "" {
					value = record.MID
				}
			case "TID":
				if record.TID != "" {
					value = record.TID
				}
			case "Source":
				if record.Source != "" {
					value = record.Source
				}
			case "Call Center Message":
				if record.MessageCallCenter != "" {
					value = record.MessageCallCenter
				}
			case "Status Merchant":
				if record.StatusMerchant != "" {
					value = record.StatusMerchant
				}
			case "SN EDC":
				if record.SNEDC != "" {
					value = record.SNEDC
				}
			case "EDC Type":
				if record.EDCType != "" {
					value = record.EDCType
				}
			case "WO Remark (Tiket)":
				if record.WORemark != "" {
					value = record.WORemark
				}
			case "Longitude":
				if record.Longitude != "" {
					value = record.Longitude
				}
			case "Latitude":
				if record.Latitude != "" {
					value = record.Latitude
				}
			case "Reason Code":
				if record.ReasonCode != "" {
					value = record.ReasonCode
				}
			case "Last Updated By":
				if record.LastUpdateBy != "" {
					value = record.LastUpdateBy
				}
			case "Last Stage Updated":
				if record.DateLastStageUpdate != nil {
					value = formatTimePointer(record.DateLastStageUpdate)
				}
			}

			if needToSetValue {
				f.SetCellValue(sheetMaster, cell, value)
				f.SetCellStyle(sheetMaster, cell, cell, style)
			}
		}
		rowIndex++ // increment once per record (row), not per column
	}

	/*
		LEFT DATA TA (ERROR & PENDING)
	*/
	titlesTALeftData := []struct {
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
	var columnsTALeftData []ExcelColumn
	for i, t := range titlesTALeftData {
		columnsTALeftData = append(columnsTALeftData, ExcelColumn{
			ColIndex: fun.GetColName(i),
			ColTitle: t.Title,
			ColSize:  t.Size,
		})
	}
	for _, column := range columnsTALeftData {
		cell := fmt.Sprintf("%s1", column.ColIndex)
		f.SetCellValue(sheetTALeftData, cell, column.ColTitle)
		f.SetColWidth(sheetTALeftData, column.ColIndex, column.ColIndex, column.ColSize)
		f.SetCellStyle(sheetTALeftData, cell, cell, styleTitle)
	}
	// Set autofilter for the left data sheet
	lastColTALeftData := fun.GetColName(len(columnsTALeftData) - 1)
	filterRangeTALeftData := fmt.Sprintf("A1:%s1", lastColTALeftData)
	f.AutoFilter(sheetTALeftData, filterRangeTALeftData, []excelize.AutoFilterOptions{})

	rowIndex = 2
	DBDataTA := gormdb.Databases.TA

	var errorData []tamodel.Error
	err = DBDataTA.Where("1=1").Order("date DESC").Find(&errorData).Error
	if err != nil {
		return "", fmt.Errorf("failed to query error data from WebTA database: %v", err)
	}
	var pendingData []tamodel.Pending
	err = DBDataTA.Where("1=1").Order("date DESC").Find(&pendingData).Error
	if err != nil {
		return "", fmt.Errorf("failed to query pending data from WebTA database: %v", err)
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

	if len(errorData) > 0 {
		for _, record := range errorData {
			for _, column := range columnsTALeftData {
				cell := fmt.Sprintf("%s%d", column.ColIndex, rowIndex)
				var value interface{} = "N/A"

				var needToSetValue bool = true

				// Handle dynamic photo column links
				if photoID, exists := photoColumnLinks[column.ColTitle]; exists {
					// This column is a photo, set the link accordingly
					linkPhoto := fmt.Sprintf("%v/here/file/%v@%v", config.WebPanel.Get().TechnicalAssistanceData.DashboardTAPublicURL, record.IDTask, photoID)
					f.SetCellValue(sheetTALeftData, cell, fmt.Sprintf("View %v", column.ColTitle))
					f.SetCellStyle(sheetTALeftData, cell, cell, style)
					// Add hyperlink to the cell using SetCellHyperlink
					f.SetCellHyperLink(sheetTALeftData, cell, linkPhoto, "External")
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
							link := fmt.Sprintf("%s/projectTask/detailWO?wo_number=%s", config.WebPanel.Get().App.WebPublicURL, wo)
							f.SetCellHyperLink(sheetTALeftData, cell, link, "External")
							value = wo
						}
					case "Ticket Subject":
						if record.SpkNumber != "" {
							value = CleanSPKNumber(record.SpkNumber)
						}
					case "Status in ODOO":
						// Find the matching ID Task in taskDataSheet and get its Stage value
						matchedStage := "N/A"
						for _, task := range taskData {
							if fmt.Sprintf("%v", task.ID) == record.IDTask {
								matchedStage = task.Stage
								break
							}
						}

						idColIndex := ""
						stageColIndex := ""
						for _, col := range columnsMaster {
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
								sheetMaster, stageColIndex, rowIndex,
								sheetMaster, idColIndex, idColIndex,
								sheetMaster, stageColIndex, stageColIndex,
								rowIndex, sheetMaster, idColIndex, idColIndex,
							)
							// fmt.Println("Generated Formula:", formula) // ✅ debug output
							err := f.SetCellFormula(sheetTALeftData, cell, formula)
							if err != nil {
								logrus.Errorf("Failed to set formula for cell %s: %v", cell, err)
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
							logrus.Errorf("Failed to create style for cell %s: %v", cell, err)
						} else {
							f.SetCellStyle(sheetTALeftData, cell, cell, styleID)
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
						for _, col := range columnsTALeftData {
							if col.ColTitle == "Teknisi" {
								teknisiColIndex = col.ColIndex
								break
							}
						}
						if teknisiColIndex != "" {
							formula := fmt.Sprintf(`IFERROR(VLOOKUP(%s%d, %v!A:C, 3, FALSE), "N/A")`, teknisiColIndex, rowIndex, sheetEmployee)
							f.SetCellFormula(sheetTALeftData, cell, formula)
						} else {
							f.SetCellValue(sheetTALeftData, cell, "N/A")
						}
					case "SPL":
						needToSetValue = false
						// Find the column index for "Teknisi"
						teknisiColIndex := ""
						for _, col := range columnsTALeftData {
							if col.ColTitle == "Teknisi" {
								teknisiColIndex = col.ColIndex
								break
							}
						}
						if teknisiColIndex != "" {
							formula := fmt.Sprintf(`IFERROR(VLOOKUP(%s%d, %v!A:C, 2, FALSE), "N/A")`, teknisiColIndex, rowIndex, sheetEmployee)
							f.SetCellFormula(sheetTALeftData, cell, formula)
						} else {
							f.SetCellValue(sheetTALeftData, cell, "N/A")
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
						f.SetCellValue(sheetTALeftData, cell, value)
						f.SetCellStyle(sheetTALeftData, cell, cell, style)
					}
				}
			}
			rowIndex++
		}
	}

	if len(pendingData) > 0 {
		for _, record := range pendingData {
			for _, column := range columnsTALeftData {
				cell := fmt.Sprintf("%s%d", column.ColIndex, rowIndex)
				var value interface{} = "N/A"

				var needToSetValue bool = true

				// Handle dynamic photo column links
				if photoID, exists := photoColumnLinks[column.ColTitle]; exists {
					// This column is a photo, set the link accordingly
					linkPhoto := fmt.Sprintf("%v/here/file/%v@%v", config.WebPanel.Get().TechnicalAssistanceData.DashboardTAPublicURL, record.IDTask, photoID)
					f.SetCellValue(sheetTALeftData, cell, fmt.Sprintf("View %v", column.ColTitle))
					f.SetCellStyle(sheetTALeftData, cell, cell, style)
					// Add hyperlink to the cell using SetCellHyperlink
					f.SetCellHyperLink(sheetTALeftData, cell, linkPhoto, "External")
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
							link := fmt.Sprintf("%s/projectTask/detailWO?wo_number=%s", config.WebPanel.Get().App.WebPublicURL, wo)
							f.SetCellHyperLink(sheetTALeftData, cell, link, "External")
							value = wo
						}
					case "Ticket Subject":
						if record.SpkNumber != "" {
							value = CleanSPKNumber(record.SpkNumber)
						}
					case "Status in ODOO":
						// Find the matching ID Task in master sheet and get its Stage value
						matchedStage := "N/A"
						for _, task := range taskData {
							if fmt.Sprintf("%v", task.ID) == record.IDTask {
								matchedStage = task.Stage
								break
							}
						}

						idColIndex := ""
						stageColIndex := ""
						for _, col := range columnsMaster {
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
								sheetMaster, stageColIndex, rowIndex,
								sheetMaster, idColIndex, idColIndex,
								sheetMaster, stageColIndex, stageColIndex,
								rowIndex, sheetMaster, idColIndex, idColIndex,
							)
							err := f.SetCellFormula(sheetTALeftData, cell, formula)
							if err != nil {
								logrus.Errorf("Failed to set formula for cell %s: %v", cell, err)
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
							logrus.Errorf("Failed to create style for cell %s: %v", cell, err)
						} else {
							f.SetCellStyle(sheetTALeftData, cell, cell, styleID)
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
						for _, col := range columnsTALeftData {
							if col.ColTitle == "Teknisi" {
								teknisiColIndex = col.ColIndex
								break
							}
						}
						if teknisiColIndex != "" {
							formula := fmt.Sprintf(`IFERROR(VLOOKUP(%s%d, %v!A:C, 3, FALSE), "N/A")`, teknisiColIndex, rowIndex, sheetEmployee)
							f.SetCellFormula(sheetTALeftData, cell, formula)
						} else {
							f.SetCellValue(sheetTALeftData, cell, "N/A")
						}
					case "SPL":
						needToSetValue = false
						// Find the column index for "Teknisi"
						teknisiColIndex := ""
						for _, col := range columnsTALeftData {
							if col.ColTitle == "Teknisi" {
								teknisiColIndex = col.ColIndex
								break
							}
						}
						if teknisiColIndex != "" {
							formula := fmt.Sprintf(`IFERROR(VLOOKUP(%s%d, %v!A:C, 2, FALSE), "N/A")`, teknisiColIndex, rowIndex, sheetEmployee)
							f.SetCellFormula(sheetTALeftData, cell, formula)
						} else {
							f.SetCellValue(sheetTALeftData, cell, "N/A")
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
						f.SetCellValue(sheetTALeftData, cell, value)
						f.SetCellStyle(sheetTALeftData, cell, cell, style)
					}
				}
			}
			rowIndex++
		}
	}

	pivotComparedDataRange := fmt.Sprintf("%s!$A$1:$%s$%d", sheetTALeftData, lastColTALeftData, rowIndex-1)
	pivotSheetComparedRange := fmt.Sprintf("%s!A8:AI200", sheetPVTCompared)
	err = f.AddPivotTable(&excelize.PivotTableOptions{
		Name:            sheetPVTCompared,
		DataRange:       pivotComparedDataRange,
		PivotTableRange: pivotSheetComparedRange,
		Rows: []excelize.PivotTableField{
			{Data: "Status in ODOO"},
		},
		Data: []excelize.PivotTableField{
			{Data: "WO Number", Subtotal: "count"},
		},
		Filter: []excelize.PivotTableField{
			{Data: "Head"},
			{Data: "SPL"},
			{Data: "Teknisi"},
			{Data: "Type"},
			{Data: "Type2"},
		},
		RowGrandTotals:      true,
		ColGrandTotals:      true,
		ShowDrill:           true,
		ShowRowHeaders:      true,
		ShowColHeaders:      true,
		ShowLastColumn:      true,
		PivotTableStyleName: "PivotStyleMedium3", // Set your desired style here
	})
	if err != nil {
		return "", fmt.Errorf("failed to create pivot table: %v", err)
	}

	// Delete default sheet
	f.DeleteSheet("Sheet1")

	// Save file
	fileName := fmt.Sprintf("ComparedReport_%s_%s.xlsx",
		report.ExcelName, time.Now().Format("20060102_150405"))
	filePath := filepath.Join(outputDir, fileName)

	if err := f.SaveAs(filePath); err != nil {
		return "", fmt.Errorf("failed to save Excel file: %v", err)
	}

	return filePath, nil
}

// SendMultipleExcelFilesReportCompared sends multiple Excel files via WhatsApp
func SendMultipleExcelFilesReportCompared(v *events.Message, stanzaID, originalSenderJID string,
	excelFiles []string, mentions []string, userLang string) {

	for i, filePath := range excelFiles {
		fileName := filepath.Base(filePath)
		caption := fmt.Sprintf("📊 Report Compared File %d/%d: %s", i+1, len(excelFiles), fileName)

		// Send each file
		SendExcelFileWithStanza(v, stanzaID, originalSenderJID, filePath, caption, mentions, userLang)

		// Deprecated: not used pivot image anymore coz it took too long to process
		// idText, enText, imgSS := GetImgPivotReportFirstSheet(filePath, userLang)
		// if imgSS != "" {
		// 	var imgCaption string
		// 	if userLang == "id" {
		// 		imgCaption = "Pivot perbandinggan data TA"
		// 	} else {
		// 		imgCaption = "Pivot comparison data TA"
		// 	}
		// 	SendImgFileWithStanza(v, stanzaID, originalSenderJID, imgSS, imgCaption, mentions, userLang)
		// 	// Clean up the image file after sending
		// 	if err := os.Remove(imgSS); err != nil {
		// 		logrus.Errorf("Failed to remove image file %s: %v", imgSS, err)
		// 	}
		// } else {
		// 	sendLangMessageWithStanza(v, stanzaID, originalSenderJID, idText, enText, userLang)
		// }

		// Add small delay between sends to avoid overwhelming the system
		if i < len(excelFiles)-1 {
			time.Sleep(1 * time.Second)
		}
	}
}

// formatTimePointer formats a time pointer to string, returns empty string if nil
func formatTimePointer(t *time.Time) string {
	if t == nil {
		return ""
	}
	return t.Format("2006-01-02 15:04:05")
}
