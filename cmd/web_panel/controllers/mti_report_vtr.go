package controllers

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"service-platform/cmd/web_panel/config"
	"service-platform/cmd/web_panel/fun"
	"service-platform/cmd/web_panel/model"
	reportmodel "service-platform/cmd/web_panel/model/report_model"
	"strings"
	"sync"
	"time"

	"github.com/sirupsen/logrus"
	"github.com/xuri/excelize/v2"
	"go.mau.fi/whatsmeow/types/events"
	"gorm.io/gorm"
)

var (
	processingReportVTRMutex sync.Mutex
	comparingReportVTRMutex  sync.Mutex
)

var titleMTIReportVTR = []ExcelColumn{
	{ColIndex: "", ColTitle: "SYS_ID_TR_CASE_ID", ColSize: 20},
	{ColIndex: "", ColTitle: "ACCEPT_DATE", ColSize: 15},
	{ColIndex: "", ColTitle: "ACCEPT_TIME", ColSize: 10},
	{ColIndex: "", ColTitle: "ACTIVE", ColSize: 8},
	{ColIndex: "", ColTitle: "ACTUAL_DATE", ColSize: 15},
	{ColIndex: "", ColTitle: "ADDRESS", ColSize: 30},
	{ColIndex: "", ColTitle: "ADO", ColSize: 10},
	{ColIndex: "", ColTitle: "AGING", ColSize: 8},
	{ColIndex: "", ColTitle: "ALERT", ColSize: 10},
	{ColIndex: "", ColTitle: "AOM", ColSize: 10},
	{ColIndex: "", ColTitle: "NAME_R_USER", ColSize: 15},
	{ColIndex: "", ColTitle: "NAME_R_GROUP", ColSize: 15},
	{ColIndex: "", ColTitle: "NAME_TR_CATEGORY", ColSize: 15},
	{ColIndex: "", ColTitle: "CASE_ID", ColSize: 20},
	{ColIndex: "", ColTitle: "CID", ColSize: 12},
	{ColIndex: "", ColTitle: "CASE_ID_SLA", ColSize: 20},
	{ColIndex: "", ColTitle: "CASE_TYPE", ColSize: 12},
	{ColIndex: "", ColTitle: "CHANNEL", ColSize: 12},
	{ColIndex: "", ColTitle: "NAME_R_CITY", ColSize: 15},
	{ColIndex: "", ColTitle: "COMMENTS", ColSize: 30},
	{ColIndex: "", ColTitle: "CREATED", ColSize: 20},
	{ColIndex: "", ColTitle: "DATA_CHNG_MST", ColSize: 20},
	{ColIndex: "", ColTitle: "DATE_CHNG_DTTM", ColSize: 20},
	{ColIndex: "", ColTitle: "EMAIL_VENDOR", ColSize: 25},
	{ColIndex: "", ColTitle: "FL_ACTIVE", ColSize: 10},
	{ColIndex: "", ColTitle: "IDENTIFIER", ColSize: 15},
	{ColIndex: "", ColTitle: "NAME_R_TYPE", ColSize: 12},
	{ColIndex: "", ColTitle: "MEMBER_BANK", ColSize: 15},
	{ColIndex: "", ColTitle: "MERCHANT_NAME", ColSize: 25},
	{ColIndex: "", ColTitle: "MERCHANT_TYPE", ColSize: 12},
	{ColIndex: "", ColTitle: "MID", ColSize: 18},
	{ColIndex: "", ColTitle: "MID_ASTRAPAY", ColSize: 18},
	{ColIndex: "", ColTitle: "MID_BNI", ColSize: 18},
	{ColIndex: "", ColTitle: "MID_BRI", ColSize: 18},
	{ColIndex: "", ColTitle: "MID_BTN", ColSize: 18},
	{ColIndex: "", ColTitle: "NAME_R_REGION", ColSize: 15},
	{ColIndex: "", ColTitle: "REGULAR", ColSize: 10},
	{ColIndex: "", ColTitle: "REGULAR_THERMAL", ColSize: 15},
	{ColIndex: "", ColTitle: "SEGMENT", ColSize: 12},
	{ColIndex: "", ColTitle: "SERIAL_NUMBER", ColSize: 20},
	{ColIndex: "", ColTitle: "NAME_R_STATE", ColSize: 12},
	{ColIndex: "", ColTitle: "STATUS_REPLACE", ColSize: 12},
	{ColIndex: "", ColTitle: "NAME_TR_SUB_CATEGORY", ColSize: 15},
	{ColIndex: "", ColTitle: "TABEL_NAME", ColSize: 15},
	{ColIndex: "", ColTitle: "TANGGAL_VISIT", ColSize: 15},
	{ColIndex: "", ColTitle: "TID", ColSize: 18},
	{ColIndex: "", ColTitle: "TID_ASTRAPAY", ColSize: 18},
	{ColIndex: "", ColTitle: "TID_BTN", ColSize: 18},
	{ColIndex: "", ColTitle: "TID_BRI", ColSize: 18},
	{ColIndex: "", ColTitle: "TID_BTN_1", ColSize: 18},
	{ColIndex: "", ColTitle: "UPDATED", ColSize: 20},
	{ColIndex: "", ColTitle: "NAME_R_COMPANY", ColSize: 20},
	{ColIndex: "", ColTitle: "VOC", ColSize: 12},
	// Red Mark => Means compared with ODOO
	{ColIndex: "", ColTitle: "STAGE", ColSize: 12},
	{ColIndex: "", ColTitle: "REMARK", ColSize: 25},
	{ColIndex: "", ColTitle: "TANGGAL VISIT", ColSize: 15},
	{ColIndex: "", ColTitle: "LINK WOD", ColSize: 25},
	{ColIndex: "", ColTitle: "ROOTCAUSE", ColSize: 25},
}

func saveMTIReportVTRBatch(db *gorm.DB, data []reportmodel.MTIReportVTR, batchSize int) error {
	if len(data) == 0 {
		logrus.Warn("No data to insert for MTI VTR report")
		return errors.New("no data to insert")
	}

	logrus.Infof("Starting batch insert for MTI VTR report, total: %d rows, batch size: %d", len(data), batchSize)

	// Optionally, run all inserts in a single transaction
	err := db.Transaction(func(tx *gorm.DB) error {
		if err := tx.CreateInBatches(data, batchSize).Error; err != nil {
			logrus.Errorf("Failed to insert batch: %v", err)
			return err
		}
		return nil
	})
	if err != nil {
		logrus.Errorf("Transaction failed: %v", err)
		return fmt.Errorf("failed to save MTI VTR report: %w", err)
	}

	logrus.Infof("Successfully inserted %d MTI VTR report rows", len(data))
	return nil
}

func processExcelReportVTRMTI(v *events.Message, user *model.WAPhoneUser, userLang string) error {
	eventToDo := "Processing MTI Report VTR"
	originalSenderJID := NormalizeSenderJID(v.Info.Sender.String())
	stanzaID := v.Info.ID

	if originalSenderJID == "" {
		id := "pengirim tidak valid"
		en := "invalid sender"
		sendLangMessageWithStanza(v, stanzaID, originalSenderJID, id, en, userLang)
		logrus.Errorf("%s: %s", eventToDo, "invalid sender JID")
		return errors.New("invalid sender JID")
	}

	if !processingReportVTRMutex.TryLock() {
		id := "sedang memproses report vtr MTI, silakan tunggu sebentar"
		en := "currently processing MTI vtr report, please wait a moment"
		sendLangMessageWithStanza(v, stanzaID, originalSenderJID, id, en, userLang)
		logrus.Warnf("%s: %s", eventToDo, "already processing")
		return fmt.Errorf("%s already running, skipping execution", eventToDo)
	}
	defer processingReportVTRMutex.Unlock()

	// Inform user we've received request
	id, en := informUserRequestReceived(eventToDo)
	sendLangMessageWithStanza(v, stanzaID, originalSenderJID, id, en, userLang)

	if v == nil || v.Message.DocumentMessage == nil || v.Message.DocumentMessage.FileName == nil {
		id := "dokumen tidak valid atau tidak ditemukan"
		en := "invalid document or not found"
		sendLangMessageWithStanza(v, stanzaID, originalSenderJID, id, en, userLang)
		return errors.New("document message or filename is nil")
	}

	filename := *v.Message.DocumentMessage.FileName
	logrus.Infof("%s: %s from user: %s", eventToDo, filename, user.PhoneNumber)

	// Get file information
	var fileSize int64
	var mimeType string

	if v.Message.DocumentMessage.FileLength != nil {
		fileSize = int64(*v.Message.DocumentMessage.FileLength)
	}

	if v.Message.DocumentMessage.Mimetype != nil {
		mimeType = *v.Message.DocumentMessage.Mimetype
	}

	// Log file details
	logrus.Infof("File details - Name: %s, Size: %d bytes, MIME: %s", filename, fileSize, mimeType)

	doc := v.Message.DocumentMessage
	if doc == nil {
		id := "pesan dokumen tidak ditemukan"
		en := "document message not found"
		sendLangMessageWithStanza(v, stanzaID, originalSenderJID, id, en, userLang)
		return errors.New("document message is nil")
	}

	fileBytes, err := WhatsappClient.Download(contx, doc)
	if err != nil {
		id := fmt.Sprintf("gagal mengunduh file: %v", err)
		en := fmt.Sprintf("failed to download file: %v", err)
		sendLangMessageWithStanza(v, stanzaID, originalSenderJID, id, en, userLang)
		return fmt.Errorf("failed to download file: %v", err)
	}

	// Check if file is empty
	if len(fileBytes) == 0 {
		id := "file yang diterima kosong"
		en := "received empty file"
		sendLangMessageWithStanza(v, stanzaID, originalSenderJID, id, en, userLang)
		return errors.New("downloaded file is empty")
	}

	directories := []string{}
	directories = append(directories, config.GetConfig().ReportMTI.ReportDir)

	fileReportDir, err := fun.FindValidDirectory(directories)
	if err != nil {
		id := fmt.Sprintf("gagal menemukan direktori yang valid: %v", err)
		en := fmt.Sprintf("failed to find valid directory: %v", err)
		sendLangMessageWithStanza(v, stanzaID, originalSenderJID, id, en, userLang)
		return fmt.Errorf("failed to find valid directory: %v", err)
	}

	fileName := fmt.Sprintf("%s_%v.xlsx", "report_vtr_mti", fun.GenerateRandomString(20))
	tempPath := filepath.Join(fileReportDir, fileName)
	// Save the file to the temporary path
	if err := os.WriteFile(tempPath, fileBytes, 0644); err != nil {
		id := fmt.Sprintf("gagal menyimpan file ke %s: %v", tempPath, err)
		en := fmt.Sprintf("failed to save file to %s: %v", tempPath, err)
		sendLangMessageWithStanza(v, stanzaID, originalSenderJID, id, en, userLang)
		return fmt.Errorf("failed to save file to %s: %v", tempPath, err)
	}
	// Log the file save operation
	logrus.Infof("Report file saved to %s", tempPath)

	f, err := excelize.OpenFile(tempPath)
	if err != nil {
		_ = os.Remove(tempPath) // Clean up the file if it cannot be opened
		id := fmt.Sprintf("gagal membuka file Excel: %v", err)
		en := fmt.Sprintf("failed to open Excel file: %v", err)
		sendLangMessageWithStanza(v, stanzaID, originalSenderJID, id, en, userLang)
		return fmt.Errorf("failed to open Excel file: %v", err)
	}
	defer func() {
		f.Close()
		_ = os.Remove(tempPath) // Clean up the file after processing
	}()

	findSheet := "DATA"
	sheetName := f.GetSheetName(1)
	if sheetName != findSheet {
		id := fmt.Sprintf("nama sheet yang diharapkan '%s', tetapi ditemukan '%s'", findSheet, sheetName)
		en := fmt.Sprintf("expected sheet name '%s', but found '%s'", findSheet, sheetName)
		sendLangMessageWithStanza(v, stanzaID, originalSenderJID, id, en, userLang)
		_ = os.Remove(tempPath) // Clean up the file if the sheet name is incorrect
		_ = f.Close()
		// Return an error if the sheet name does not match
		return fmt.Errorf("expected sheet name '%s', but found '%s'", findSheet, sheetName)
	}

	// Validate the header row
	headerRow, err := f.GetRows(sheetName, excelize.Options{})
	if err != nil {
		id := fmt.Sprintf("gagal mendapatkan baris dari sheet: %v", err)
		en := fmt.Sprintf("failed to get rows from sheet: %v", err)
		sendLangMessageWithStanza(v, stanzaID, originalSenderJID, id, en, userLang)
		_ = os.Remove(tempPath) // Clean up the file if there is an error
		_ = f.Close()
		// Return an error if there is an issue getting the rows
		return fmt.Errorf("failed to get rows from sheet: %v", err)
	}

	var titleMTIReportVTRNoRedMark []ExcelColumn
	for _, col := range titleMTIReportVTR {
		if col.ColTitle != "STAGE" && col.ColTitle != "REMARK" && col.ColTitle != "TANGGAL VISIT" &&
			col.ColTitle != "LINK WOD" && col.ColTitle != "ROOTCAUSE" {
			titleMTIReportVTRNoRedMark = append(titleMTIReportVTRNoRedMark, col)
		}
	}

	if len(headerRow) == 0 || len(headerRow[0]) < len(titleMTIReportVTRNoRedMark) {
		id := "baris header tidak valid dalam file Excel untuk report vtr MTI"
		en := "invalid header row in Excel file for MTI vtr report"
		sendLangMessageWithStanza(v, stanzaID, originalSenderJID, id, en, userLang)
		_ = os.Remove(tempPath) // Clean up the file if the header row is invalid
		_ = f.Close()
		// Return an error if the header row is invalid
		return errors.New("invalid header row in Excel file for MTI report VTR")
	}

	// Check if the header row matches the expected titles
	for i, col := range titleMTIReportVTRNoRedMark {
		if i >= len(headerRow[0]) || headerRow[0][i] != col.ColTitle {
			id := fmt.Sprintf("kolom header tidak sesuai pada kolom %s: %s, diharapkan: %s", fun.GetColName(i), headerRow[0][i], col.ColTitle)
			en := fmt.Sprintf("header column mismatch at column %s: %s, expected: %s", fun.GetColName(i), headerRow[0][i], col.ColTitle)
			sendLangMessageWithStanza(v, stanzaID, originalSenderJID, id, en, userLang)
			_ = os.Remove(tempPath) // Clean up the file if the header row does not match
			_ = f.Close()
			// Return an error if the header row does not match
			return fmt.Errorf("header column mismatch at column %s: %s, expected: %s", fun.GetColName(i), headerRow[0][i], col.ColTitle)
		}
	}

	// Map header titles to their column indexes for flexible matching
	headerMap := make(map[string]int)
	for i, title := range headerRow[0] {
		headerMap[title] = i
	}

	// Collect all missing columns for all rows, and send a single message at the end
	var allMissingCols []string

	// Process the data from the Excel file
	var dataForMTIReportVTR []reportmodel.MTIReportVTR
	for rowIndex, row := range headerRow {
		if rowIndex == 0 {
			continue // Skip the header row
		}
		// Check completeness based on expected columns
		missingCols := []string{}
		for _, col := range titleMTIReportVTRNoRedMark {
			if idx, ok := headerMap[col.ColTitle]; !ok || idx >= len(row) || row[idx] == "" {
				missingCols = append(missingCols, fmt.Sprintf("Row %d: %s", rowIndex+1, col.ColTitle))
			}
		}
		if len(missingCols) > 0 {
			allMissingCols = append(allMissingCols, missingCols...)
			// continue // Skip this row if it is incomplete
		}

		// Build a map of column title to value for this row
		rowData := make(map[string]string)
		for _, col := range titleMTIReportVTRNoRedMark {
			idx := headerMap[col.ColTitle]
			rowData[col.ColTitle] = row[idx]
		}

		var accDate, accTime, actualDate, dataChangedMST, dataChangedDatetime, tanggalVisit, updated *time.Time
		if rowData["ACCEPT_DATE"] != "" {
			timeValue, err := fun.ParseFlexibleDate(rowData["ACCEPT_DATE"])
			if err != nil {
				logrus.Errorf("Failed to parse Accept Date for row %d: %v", rowIndex+1, err)
				allMissingCols = append(allMissingCols, fmt.Sprintf("Row %d: Invalid Accept Date format", rowIndex+1))
			} else {
				accDate = &timeValue
			}
		}
		if rowData["ACCEPT_TIME"] != "" {
			timeValue, err := fun.ParseFlexibleDate(rowData["ACCEPT_TIME"])
			if err != nil {
				logrus.Errorf("Failed to parse Accept Time for row %d: %v", rowIndex+1, err)
				allMissingCols = append(allMissingCols, fmt.Sprintf("Row %d: Invalid Accept Time format", rowIndex+1))
			} else {
				accTime = &timeValue
			}
		}
		if rowData["ACTUAL_DATE"] != "" {
			timeValue, err := fun.ParseFlexibleDate(rowData["ACTUAL_DATE"])
			if err != nil {
				logrus.Errorf("Failed to parse Actual Date for row %d: %v", rowIndex+1, err)
				allMissingCols = append(allMissingCols, fmt.Sprintf("Row %d: Invalid Actual Date format", rowIndex+1))
			} else {
				actualDate = &timeValue
			}
		}
		if rowData["DATA_CHNG_MST"] != "" {
			timeValue, err := fun.ParseFlexibleDate(rowData["DATA_CHNG_MST"])
			if err != nil {
				logrus.Errorf("Failed to parse Data Changed MST for row %d: %v", rowIndex+1, err)
				allMissingCols = append(allMissingCols, fmt.Sprintf("Row %d: Invalid Data Changed MST format", rowIndex+1))
			} else {
				dataChangedMST = &timeValue
			}
		}
		if rowData["DATE_CHNG_DTTM"] != "" {
			timeValue, err := fun.ParseFlexibleDate(rowData["DATE_CHNG_DTTM"])
			if err != nil {
				logrus.Errorf("Failed to parse Date Changed DTTM for row %d: %v", rowIndex+1, err)
				allMissingCols = append(allMissingCols, fmt.Sprintf("Row %d: Invalid Date Changed DTTM format", rowIndex+1))
			} else {
				dataChangedDatetime = &timeValue
			}
		}
		if rowData["TANGGAL_VISIT"] != "" {
			timeValue, err := fun.ParseFlexibleDate(rowData["TANGGAL_VISIT"])
			if err != nil {
				logrus.Errorf("Failed to parse Tanggal Visit for row %d: %v", rowIndex+1, err)
				allMissingCols = append(allMissingCols, fmt.Sprintf("Row %d: Invalid Tanggal Visit format", rowIndex+1))
			} else {
				tanggalVisit = &timeValue
			}
		}
		if rowData["UPDATED"] != "" {
			timeValue, err := fun.ParseFlexibleDate(rowData["UPDATED"])
			if err != nil {
				logrus.Errorf("Failed to parse Updated for row %d: %v", rowIndex+1, err)
				allMissingCols = append(allMissingCols, fmt.Sprintf("Row %d: Invalid Updated format", rowIndex+1))
			} else {
				updated = &timeValue
			}
		}

		// Subtract 7 hours from the parsed dates if they are not nil
		if accDate != nil {
			t := accDate.Add(-7 * time.Hour)
			accDate = &t
		}
		if accTime != nil {
			t := accTime.Add(-7 * time.Hour)
			accTime = &t
		}
		if actualDate != nil {
			t := actualDate.Add(-7 * time.Hour)
			actualDate = &t
		}
		if dataChangedMST != nil {
			t := dataChangedMST.Add(-7 * time.Hour)
			dataChangedMST = &t
		}
		if dataChangedDatetime != nil {
			t := dataChangedDatetime.Add(-7 * time.Hour)
			dataChangedDatetime = &t
		}
		if tanggalVisit != nil {
			t := tanggalVisit.Add(-7 * time.Hour)
			tanggalVisit = &t
		}
		if updated != nil {
			t := updated.Add(-7 * time.Hour)
			updated = &t
		}

		// Map rowData to MTIReportVTR struct
		report := reportmodel.MTIReportVTR{
			SysIDTrCaseID:     rowData["SYS_ID_TR_CASE_ID"],
			AcceptDate:        accDate,
			AcceptTime:        accTime,
			Active:            rowData["ACTIVE"],
			ActualDate:        actualDate,
			Address:           rowData["ADDRESS"],
			Ado:               rowData["ADO"],
			Aging:             rowData["AGING"],
			Alert:             rowData["ALERT"],
			Aom:               rowData["AOM"],
			NameRUser:         rowData["NAME_R_USER"],
			NameRGroup:        rowData["NAME_R_GROUP"],
			NameTrCategory:    rowData["NAME_TR_CATEGORY"],
			CaseID:            rowData["CASE_ID"],
			Cid:               rowData["CID"],
			CaseIDSla:         rowData["CASE_ID_SLA"],
			CaseType:          rowData["CASE_TYPE"],
			Channel:           rowData["CHANNEL"],
			NameRCity:         rowData["NAME_R_CITY"],
			Comments:          rowData["COMMENTS"],
			Created:           rowData["CREATED"],
			DataChngMst:       dataChangedMST,
			DateChngDttm:      dataChangedDatetime,
			EmailVendor:       rowData["EMAIL_VENDOR"],
			FlActive:          rowData["FL_ACTIVE"],
			Identifier:        rowData["IDENTIFIER"],
			NameRType:         rowData["NAME_R_TYPE"],
			MemberBank:        rowData["MEMBER_BANK"],
			MerchantName:      rowData["MERCHANT_NAME"],
			MerchantType:      rowData["MERCHANT_TYPE"],
			Mid:               rowData["MID"],
			MidAstrapay:       rowData["MID_ASTRAPAY"],
			MidBni:            rowData["MID_BNI"],
			MidBri:            rowData["MID_BRI"],
			MidBtn:            rowData["MID_BTN"],
			NameRRegion:       rowData["NAME_R_REGION"],
			Regular:           rowData["REGULAR"],
			RegularThermal:    rowData["REGULAR_THERMAL"],
			Segment:           rowData["SEGMENT"],
			SerialNumber:      rowData["SERIAL_NUMBER"],
			NameRState:        rowData["NAME_R_STATE"],
			StatusReplace:     rowData["STATUS_REPLACE"],
			NameTrSubCategory: rowData["NAME_TR_SUB_CATEGORY"],
			TabelName:         rowData["TABEL_NAME"],
			TanggalVisit:      tanggalVisit,
			Tid:               rowData["TID"],
			TidAstrapay:       rowData["TID_ASTRAPAY"],
			TidBtn:            rowData["TID_BTN"],
			TidBri:            rowData["TID_BRI"],
			TidBtn1:           rowData["TID_BTN_1"],
			Updated:           updated,
			NameRCompany:      rowData["NAME_R_COMPANY"],
			Voc:               rowData["VOC"],
		}

		dataForMTIReportVTR = append(dataForMTIReportVTR, report)
	}

	if len(dataForMTIReportVTR) == 0 {
		id := "tidak ada data yang ditemukan dalam file report VTR MTI"
		en := "no data found in MTI VTR report file"
		sendLangMessageWithStanza(v, stanzaID, originalSenderJID, id, en, userLang)
		logrus.Warnf("%s: %s", eventToDo, "no data found in file")
		return errors.New("no data found in MTI VTR report file")
	}

	batchSize := 1000 // or tune as needed, e.g., 500 or 2000
	tx := dbWeb.Unscoped().Where("id != ?", 0).Delete(&reportmodel.MTIReportVTR{})
	if tx.Error != nil {
		return tx.Error
	}
	if tx.RowsAffected == 0 {
		logrus.Debug("No rows were deleted (table might already be empty)")
	}

	if err := saveMTIReportVTRBatch(dbWeb, dataForMTIReportVTR, batchSize); err != nil {
		id := fmt.Sprintf("gagal menyimpan data ke database: %v", err)
		en := fmt.Sprintf("failed to save data to database: %v", err)
		sendLangMessageWithStanza(v, stanzaID, originalSenderJID, id, en, userLang)
		return err
	}

	// Send a single message if there are missing columns
	if len(allMissingCols) > 0 {
		// ADD this if you want to show whats empty columns being uploaded
		_ = allMissingCols
		// id := fmt.Sprintf("Beberapa baris tidak lengkap, kolom hilang:\n%s", fun.JoinStringSlice(allMissingCols, "\n"))
		// en := fmt.Sprintf("Some rows are incomplete, missing columns:\n%s", fun.JoinStringSlice(allMissingCols, "\n"))
		// sendLangMessageWithStanza(v, stanzaID, originalSenderJID, id, en, userLang)
		// Optional return here if you want to stop processing after this
		// return fmt.Errorf("some rows are incomplete, missing columns: %s", fun.JoinStringSlice(allMissingCols, ", "))
	}

	id = fmt.Sprintf("✅ File report VTR MTI '%s' berhasil diproses!\n🖥 Data telah disimpan kedalam database.", filename)
	en = fmt.Sprintf("✅ VTR report MTI file '%s' processed successfully!\n🖥 Data has been saved to database.", filename)
	sendLangMessageWithStanza(v, stanzaID, originalSenderJID, id, en, userLang)

	// Yokke x ODOO comparison
	compareMTIReportVTRYokkeWithODOO(v, userLang)
	return nil
}

// FIX: its comparing with ODOO data !!!!!!!!!!!!!
func compareMTIReportVTRYokkeWithODOO(v *events.Message, userLang string) {
	eventToDo := ""
	if userLang == "id" {
		eventToDo = "Membandingkan Report VTR MTI dengan ODOO"
	} else {
		eventToDo = "Comparing MTI VTR Report with ODOO"
	}
	originalSenderJID := NormalizeSenderJID(v.Info.Sender.String())
	stanzaID := v.Info.ID

	if originalSenderJID == "" {
		id := "pengirim tidak valid"
		en := "invalid sender"
		sendLangMessageWithStanza(v, stanzaID, originalSenderJID, id, en, userLang)
		logrus.Errorf("%s: %s", eventToDo, "invalid sender JID")
		return
	}

	if !comparingReportVTRMutex.TryLock() {
		id := "sedang membandingkan report VTR MTI dengan ODOO, silakan tunggu sebentar"
		en := "currently comparing MTI VTR report with ODOO, please wait a moment"
		sendLangMessageWithStanza(v, stanzaID, originalSenderJID, id, en, userLang)
		logrus.Warnf("%s: %s", eventToDo, "already comparing")
		return
	}
	defer comparingReportVTRMutex.Unlock()

	// Inform user we've received request
	sendLangMessageWithStanza(
		v,
		stanzaID,
		originalSenderJID,
		"🔍 Memulai perbandingan data report VTR MTI dengan ODOO...",
		"🔍 Starting comparison of MTI VTR report data with ODOO...",
		userLang,
	)

	var yokkeDataMTIReportVTR []reportmodel.MTIReportVTR
	if err := dbWeb.Find(&yokkeDataMTIReportVTR).Error; err != nil {
		id := fmt.Sprintf("gagal mengambil data report VTR MTI dari database: %v", err)
		en := fmt.Sprintf("failed to retrieve MTI VTR report data from database: %v", err)
		sendLangMessageWithStanza(v, stanzaID, originalSenderJID, id, en, userLang)
		logrus.Errorf("%s: %s", eventToDo, "failed to retrieve MTI report VTR data from database")
		return
	}
	if len(yokkeDataMTIReportVTR) == 0 {
		id := "tidak ada data report VTR MTI yang ditemukan di database"
		en := "no MTI VTR report data found in database"
		sendLangMessageWithStanza(v, stanzaID, originalSenderJID, id, en, userLang)
		logrus.Warnf("%s: %s", eventToDo, "no data found in database")
		return
	}

	/*
		Init excel for comparison
	*/
	f := excelize.NewFile()
	defer func() {
		if err := f.Close(); err != nil {
			logrus.Errorf("Failed to close Excel file: %v", err)
		}
		// if err := os.Remove(f.Path); err != nil {
		// 	logrus.Errorf("Failed to remove temporary Excel file: %v", err)
		// }
	}()

	sheetMaster := "DATA"
	sheetPivot := "PIVOT"
	f.NewSheet(sheetPivot)
	f.NewSheet(sheetMaster)
	f.DeleteSheet("Sheet1") // Remove default sheet

	styleMasterTitle, _ := f.NewStyle(&excelize.Style{
		Alignment: &excelize.Alignment{
			Horizontal: "center",
			Vertical:   "center",
		},
		Fill: excelize.Fill{
			Type:    "pattern",
			Color:   []string{"#FCD202"},
			Pattern: 1,
		},
		Font: &excelize.Font{
			Bold:  true,
			Color: "#FC0202",
			Size:  11,
		},
	})
	styleMasterTitleRedMark, _ := f.NewStyle(&excelize.Style{
		Alignment: &excelize.Alignment{
			Horizontal: "center",
			Vertical:   "center",
		},
		Fill: excelize.Fill{
			Type:    "pattern",
			Color:   []string{"#ff0000"},
			Pattern: 1,
		},
		Font: &excelize.Font{
			Bold:  true,
			Color: "#000000",
			Size:  11,
		},
	})

	style, _ := f.NewStyle(&excelize.Style{
		Alignment: &excelize.Alignment{
			Horizontal: "center",
			Vertical:   "center",
		},
		Font: &excelize.Font{
			Size: 11,
		},
	})

	var columnsMaster []ExcelColumn
	for i, t := range titleMTIReportVTR {
		columnsMaster = append(columnsMaster, ExcelColumn{
			ColIndex: fun.GetColName(i),
			ColTitle: t.ColTitle,
			ColSize:  t.ColSize,
		})
	}

	// Set the title row in the master sheet
	for _, col := range columnsMaster {
		cell := fmt.Sprintf("%s1", col.ColIndex)
		f.SetCellValue(sheetMaster, cell, col.ColTitle)
		f.SetColWidth(sheetMaster, col.ColIndex, col.ColIndex, col.ColSize)
		if col.ColTitle == "STAGE" || col.ColTitle == "REMARK" || col.ColTitle == "ROOTCAUSE" || col.ColTitle == "TANGGAL VISIT" || col.ColTitle == "LINK WOD" {
			f.SetCellStyle(sheetMaster, cell, cell, styleMasterTitleRedMark)
		} else {
			f.SetCellStyle(sheetMaster, cell, cell, styleMasterTitle)
		}
	}
	lastColMaster := fun.GetColName(len(columnsMaster) - 1)
	filterRangeMaster := fmt.Sprintf("A1:%s1", lastColMaster)
	f.AutoFilter(sheetMaster, filterRangeMaster, []excelize.AutoFilterOptions{})

	batchSize := 500
	totalRows := len(yokkeDataMTIReportVTR)

	// Create a map for faster row index lookup
	rowIndexMap := make(map[string]int)
	for i, data := range yokkeDataMTIReportVTR {
		// Use CID as the unique key for matching
		key := data.Cid
		rowIndexMap[key] = i + 2 // +2 for Excel 1-indexing and header row
	}

	// First, populate Excel rows for ALL MTI data (base data)
	logrus.Infof("Populating Excel with all MTI data (%d records)", len(yokkeDataMTIReportVTR))
	for i, mtiData := range yokkeDataMTIReportVTR {
		rowIndex := i + 2 // +2 for Excel 1-indexing and header row

		// Write all standard MTI data to Excel
		for _, col := range columnsMaster {
			cell := fmt.Sprintf("%s%d", col.ColIndex, rowIndex)
			var cellValue interface{}

			// Set all the MTI data including existing ODOO comparison data
			switch col.ColTitle {
			case "STAGE":
				// Use existing database value if available
				if mtiData.Stage != "" {
					cellValue = mtiData.Stage
				}
			case "REMARK":
				// Use existing database value if available
				if mtiData.Remark != "" {
					cellValue = mtiData.Remark
				}
			case "ROOTCAUSE":
				// Use existing database value if available
				if mtiData.RootCause != "" {
					cellValue = mtiData.RootCause
				}
			case "TANGGAL VISIT":
				// Use existing database value if available (from ODOO comparison)
				if mtiData.TanggalVisit2 != nil {
					cellValue = mtiData.TanggalVisit2.Format("2006-01-02")
				}
			case "LINK WOD":
				// Use existing database value if available
				if mtiData.LinkWod != "" {
					if err := f.SetCellHyperLink(sheetMaster, cell, mtiData.LinkWod, "External"); err != nil {
						logrus.Warnf("Failed to set hyperlink for cell %s: %v", cell, err)
					}
					cellValue = mtiData.LinkWod
				}
			case "SYS_ID_TR_CASE_ID":
				cellValue = mtiData.SysIDTrCaseID
			case "ACCEPT_DATE":
				if mtiData.AcceptDate != nil {
					cellValue = mtiData.AcceptDate.Format("2006-01-02")
				}
			case "ACCEPT_TIME":
				if mtiData.AcceptTime != nil {
					cellValue = mtiData.AcceptTime.Format("15:04:05")
				}
			case "ACTIVE":
				cellValue = mtiData.Active
			case "ACTUAL_DATE":
				if mtiData.ActualDate != nil {
					cellValue = mtiData.ActualDate.Format("2006-01-02")
				}
			case "ADDRESS":
				cellValue = mtiData.Address
			case "ADO":
				cellValue = mtiData.Ado
			case "AGING":
				cellValue = mtiData.Aging
			case "ALERT":
				cellValue = mtiData.Alert
			case "AOM":
				cellValue = mtiData.Aom
			case "NAME_R_USER":
				cellValue = mtiData.NameRUser
			case "NAME_R_GROUP":
				cellValue = mtiData.NameRGroup
			case "NAME_TR_CATEGORY":
				cellValue = mtiData.NameTrCategory
			case "CASE_ID":
				cellValue = mtiData.CaseID
			case "CID":
				cellValue = mtiData.Cid
			case "CASE_ID_SLA":
				cellValue = mtiData.CaseIDSla
			case "CASE_TYPE":
				cellValue = mtiData.CaseType
			case "CHANNEL":
				cellValue = mtiData.Channel
			case "NAME_R_CITY":
				cellValue = mtiData.NameRCity
			case "COMMENTS":
				cellValue = mtiData.Comments
			case "CREATED":
				cellValue = mtiData.Created
			case "DATA_CHNG_MST":
				if mtiData.DataChngMst != nil {
					cellValue = mtiData.DataChngMst.Format("2006-01-02 15:04:05")
				}
			case "DATE_CHNG_DTTM":
				if mtiData.DateChngDttm != nil {
					cellValue = mtiData.DateChngDttm.Format("2006-01-02 15:04:05")
				}
			case "EMAIL_VENDOR":
				cellValue = mtiData.EmailVendor
			case "FL_ACTIVE":
				cellValue = mtiData.FlActive
			case "IDENTIFIER":
				cellValue = mtiData.Identifier
			case "NAME_R_TYPE":
				cellValue = mtiData.NameRType
			case "MEMBER_BANK":
				cellValue = mtiData.MemberBank
			case "MERCHANT_NAME":
				cellValue = mtiData.MerchantName
			case "MERCHANT_TYPE":
				cellValue = mtiData.MerchantType
			case "MID":
				cellValue = mtiData.Mid
			case "MID_ASTRAPAY":
				cellValue = mtiData.MidAstrapay
			case "MID_BNI":
				cellValue = mtiData.MidBni
			case "MID_BRI":
				cellValue = mtiData.MidBri
			case "MID_BTN":
				cellValue = mtiData.MidBtn
			case "NAME_R_REGION":
				cellValue = mtiData.NameRRegion
			case "REGULAR":
				cellValue = mtiData.Regular
			case "REGULAR_THERMAL":
				cellValue = mtiData.RegularThermal
			case "SEGMENT":
				cellValue = mtiData.Segment
			case "SERIAL_NUMBER":
				cellValue = mtiData.SerialNumber
			case "NAME_R_STATE":
				cellValue = mtiData.NameRState
			case "STATUS_REPLACE":
				cellValue = mtiData.StatusReplace
			case "NAME_TR_SUB_CATEGORY":
				cellValue = mtiData.NameTrSubCategory
			case "TABEL_NAME":
				cellValue = mtiData.TabelName
			case "TANGGAL_VISIT":
				if mtiData.TanggalVisit != nil {
					cellValue = mtiData.TanggalVisit.Format("2006-01-02")
				}
			case "TID":
				cellValue = mtiData.Tid
			case "TID_ASTRAPAY":
				cellValue = mtiData.TidAstrapay
			case "TID_BTN":
				cellValue = mtiData.TidBtn
			case "TID_BRI":
				cellValue = mtiData.TidBri
			case "TID_BTN_1":
				cellValue = mtiData.TidBtn1
			case "UPDATED":
				if mtiData.Updated != nil {
					cellValue = mtiData.Updated.Format("2006-01-02 15:04:05")
				}
			case "NAME_R_COMPANY":
				cellValue = mtiData.NameRCompany
			case "VOC":
				cellValue = mtiData.Voc
			}

			// Set cell value if not nil/empty
			if cellValue != nil && cellValue != "" {
				if err := f.SetCellValue(sheetMaster, cell, cellValue); err != nil {
					logrus.Warnf("Failed to set cell value for %s: %v", cell, err)
				}
			}

			// Apply style to the cell
			if err := f.SetCellStyle(sheetMaster, cell, cell, style); err != nil {
				logrus.Warnf("Failed to set cell style for %s: %v", cell, err)
			}
		}
	}

	// Process batches sequentially but with timeout protection
	for start := 0; start < totalRows; start += batchSize {
		end := start + batchSize
		if end > totalRows {
			end = totalRows
		}
		batch := yokkeDataMTIReportVTR[start:end]

		if len(batch) == 0 {
			logrus.Warnf("No data in batch %d-%d, skipping", start+1, end)
			continue
		}

		func() {
			var ODOOJobID []string
			for _, data := range batch {
				if data.Cid != "" {
					ODOOJobID = append(ODOOJobID, data.Cid)
				}
			}

			ODOOMOdel := "helpdesk.ticket"
			domain := []interface{}{
				[]interface{}{"active", "=", true},
				[]interface{}{"x_job_id", "=", ODOOJobID},
			}
			fields := []string{
				"id",
				"name",
				"x_job_id",
				"stage_id",
				"x_wo_remark",
				"x_link",
				"complete_datetime_wo",
				"x_sla_deadline",
				"fsm_task_count",
				"x_source",
				"x_task_type",
			}
			order := "id asc"
			odooParams := map[string]interface{}{
				"model":  ODOOMOdel,
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
				logrus.Errorf("%s: %v", eventToDo, err)
				return
			}

			// Use existing ODOO call (we'll add timeout to this function separately)
			ODOOResponse, err := GetODOOMSData(string(payloadBytes))
			if err != nil {
				logrus.Errorf("%s: %v", eventToDo, err)
				return
			}

			// Rest of the processing logic continues as before...
			ODOOResponseArray, ok := ODOOResponse.([]interface{})
			if !ok {
				logrus.Errorf("%s: expected ODOO response to be an array, got %v", eventToDo, ODOOResponse)
				return
			}
			ODOOResponseBytes, err := json.Marshal(ODOOResponseArray)
			if err != nil {
				logrus.Errorf("%s: failed to marshal ODOO response: %v", eventToDo, err)
				return
			}

			var odooData []OdooTicketDataRequestItem
			if err := json.Unmarshal(ODOOResponseBytes, &odooData); err != nil {
				logrus.Errorf("%s: failed to unmarshal ODOO response: %v", eventToDo, err)
				return
			}

			if len(odooData) == 0 {
				logrus.Warnf("%s: no ODOO data found for batch %d-%d", eventToDo, start+1, end)
				return
			}

			logrus.Infof("%s: found %d ODOO records for batch %d-%d", eventToDo, len(odooData), start+1, end)

			// Collect all updates for batch processing
			var batchUpdates []map[string]interface{}

			for _, data := range odooData {
				// Find matching MTI report data by Job ID (which should match CID)
				for _, mtiData := range batch {
					if mtiData.Cid == data.JobId.String {
						// logrus.Infof("%s: Found match - CID: %s, ODOO JobId: %s", eventToDo, mtiData.Cid, data.JobId.String)
						// Found a match - prepare for both Excel writing and database update
						// Use the pre-calculated row index map for better performance
						key := mtiData.Cid
						rowIndex, exists := rowIndexMap[key]
						if !exists {
							logrus.Warnf("Row index not found for CID %s", mtiData.Cid)
							continue
						}

						// Prepare batch update data for database
						updateData := map[string]interface{}{
							"cid": mtiData.Cid, // Primary key for identification
						}

						_, odooStage, err := parseJSONIDDataCombined(data.StageId)
						switch strings.ToLower(odooStage) {
						case "new":
							odooStage = "Scheduled"
						case "solved":
							odooStage = "Done"
						case "solved pending":
							odooStage = "Pending"
						case "waiting for verification":
							odooStage = "Close"
						}

						if err != nil {
							logrus.Errorf("%s: failed to parse stage ID data for job ID %s: %v", eventToDo, data.JobId.String, err)
						} else {
							updateData["stage"] = odooStage
						}

						_, _, firstTaskReason, _ := setSLAStatus(data.TaskCount.Int, data.SlaDeadline, data.CompleteDatetimeWo, data.WoRemarkTiket, data.TaskType)

						if data.WoRemarkTiket.Valid && data.WoRemarkTiket.String != "" {
							updateData["remark"] = data.WoRemarkTiket.String
						}
						if firstTaskReason != "" {
							updateData["rootcause"] = firstTaskReason
						}
						var tglVisit *time.Time
						if data.CompleteDatetimeWo.Valid && !data.CompleteDatetimeWo.Time.IsZero() {
							tglVisit = &data.CompleteDatetimeWo.Time
						}
						if tglVisit != nil {
							updateData["tanggal_visit2"] = tglVisit
						}
						if data.LinkWO.Valid && data.LinkWO.String != "" {
							updateData["link_wod"] = data.LinkWO.String
						}

						// Add to batch if there's data to update
						if len(updateData) > 1 { // More than just the primary key
							batchUpdates = append(batchUpdates, updateData)
						}

						// Update ONLY the ODOO comparison columns in Excel
						odooColumns := map[string]interface{}{
							"STAGE": odooStage,
						}
						if data.WoRemarkTiket.Valid && data.WoRemarkTiket.String != "" {
							odooColumns["REMARK"] = data.WoRemarkTiket.String
						}
						if firstTaskReason != "" {
							odooColumns["ROOTCAUSE"] = firstTaskReason
						}
						if tglVisit != nil {
							odooColumns["TANGGAL VISIT"] = tglVisit.Format("2006-01-02")
						}
						if data.LinkWO.Valid && data.LinkWO.String != "" {
							odooColumns["LINK WOD"] = data.LinkWO.String
						}

						// Update only the ODOO comparison columns
						for _, col := range columnsMaster {
							if value, exists := odooColumns[col.ColTitle]; exists && value != nil && value != "" {
								cell := fmt.Sprintf("%s%d", col.ColIndex, rowIndex)

								// Handle hyperlinks for LINK WOD
								if col.ColTitle == "LINK WOD" {
									if err := f.SetCellHyperLink(sheetMaster, cell, value.(string), "External"); err != nil {
										logrus.Warnf("Failed to set hyperlink for cell %s: %v", cell, err)
									}
								}

								if err := f.SetCellValue(sheetMaster, cell, value); err != nil {
									logrus.Warnf("Failed to update ODOO cell value for %s: %v", cell, err)
								}

								// Apply style to the updated cell
								if err := f.SetCellStyle(sheetMaster, cell, cell, style); err != nil {
									logrus.Warnf("Failed to set cell style for %s: %v", cell, err)
								}
							}
						}

						break // Found the match, no need to continue inner loop
					}
				}
			}

			// Perform batch database updates in a transaction
			if len(batchUpdates) > 0 {
				err := dbWeb.Transaction(func(tx *gorm.DB) error {
					for _, updateData := range batchUpdates {
						CID := updateData["cid"]
						delete(updateData, "cid") // Remove primary key from update data

						if err := tx.Model(&reportmodel.MTIReportVTR{}).
							Where("cid = ?", CID).
							Updates(updateData).Error; err != nil {
							return err
						}
					}
					return nil
				})

				if err != nil {
					logrus.Errorf("%s: failed to perform batch update for batch %d-%d: %v", eventToDo, start+1, end, err)
				} else {
					logrus.Infof("%s: successfully updated %d MTI records in batch %d-%d", eventToDo, len(batchUpdates), start+1, end)
				}
			}
		}()
	}

	/*
		PIVOT
	*/
	pivotMasterDataRange := fmt.Sprintf("%s!$A$1:$%s$%d", sheetMaster, lastColMaster, len(yokkeDataMTIReportVTR)+1)
	pivotRange1 := fmt.Sprintf("%s!A4:B50", sheetPivot)
	f.AddPivotTable(&excelize.PivotTableOptions{
		Name:            sheetPivot,
		DataRange:       pivotMasterDataRange,
		PivotTableRange: pivotRange1,
		Rows: []excelize.PivotTableField{
			{Data: "STAGE"},
			{Data: "ROOTCAUSE"},
		},
		Columns: []excelize.PivotTableField{
			{Data: "NAME_R_TYPE"},
		},
		Data: []excelize.PivotTableField{
			{Data: "STAGE", Subtotal: "count"},
		},
		Filter: []excelize.PivotTableField{
			{Data: "MEMBER_BANK"},
		},
		RowGrandTotals:      true,
		ColGrandTotals:      true,
		ShowDrill:           true,
		ShowRowHeaders:      true,
		ShowColHeaders:      true,
		ShowLastColumn:      true,
		PivotTableStyleName: "PivotStyleMedium8", // Set your desired style here
	})

	f.SetColWidth(sheetPivot, "A", "A", 30)

	// Set Excel document properties (author, etc.)
	f.SetDocProps(&excelize.DocProperties{
		Creator:        config.GetConfig().Default.PT,
		Title:          "MTI VTR Report Comparison",
		Description:    "Comparison report between Yokke and ODOO for data MTI VTR",
		LastModifiedBy: config.GetConfig().Default.PT + " Service Report",
		Keywords:       "MTI, Report, VTR, Comparison, Yokke, ODOO",
	})

	reportName := "MTI_Report_VTR"
	reportFileName := fmt.Sprintf("%s_%s.xlsx", reportName, time.Now().Format("20060102_150405"))
	fileReportDir := filepath.Join(config.GetConfig().ReportMTI.ReportDir, time.Now().Format("2006-01-02"))
	if err := os.MkdirAll(fileReportDir, 0755); err != nil {
		logrus.Errorf("%s: failed to create report directory: %v", eventToDo, err)
		id := fmt.Sprintf("gagal membuat direktori report: %v", err)
		en := fmt.Sprintf("failed to create report directory: %v", err)
		sendLangMessageWithStanza(v, stanzaID, originalSenderJID, id, en, userLang)
		return
	}
	excelFilePath := filepath.Join(fileReportDir, reportFileName)
	if err := f.SaveAs(excelFilePath); err != nil {
		logrus.Errorf("%s: failed to save report file: %v", eventToDo, err)
		id := fmt.Sprintf("gagal menyimpan file report: %v", err)
		en := fmt.Sprintf("failed to save report file: %v", err)
		sendLangMessageWithStanza(v, stanzaID, originalSenderJID, id, en, userLang)
		return
	}

	var excelCaption string
	if userLang == "id" {
		excelCaption = "📊 Berikut adalah lampiran dari Report VTR MTI"
	} else {
		excelCaption = "📊 Here is the attachment of MTI VTR Report"
	}

	SendExcelFileWithStanza(v, stanzaID, originalSenderJID, excelFilePath, excelCaption, nil, userLang)

	// Add scheduled report VTR
	var scheduledReportData reportmodel.ReportScheduled
	if err := dbWeb.Where("id = ?", reportName).First(&scheduledReportData).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			// Not found, create new
			scheduledReportData = reportmodel.ReportScheduled{
				ID:                  reportName,
				FilePath:            excelFilePath,
				ScheduledBy:         v.Info.Sender.String(),
				AlreadySentViaEmail: false,
			}
			if err := dbWeb.Create(&scheduledReportData).Error; err != nil {
				logrus.Errorf("%s: failed to create scheduled report: %v", eventToDo, err)
			}
		} else {
			logrus.Errorf("%s: failed to find scheduled report: %v", eventToDo, err)
		}
	} else {
		// Found, update it
		updateMap := map[string]interface{}{
			"file_path":              excelFilePath,
			"scheduled_by":           v.Info.Sender.String(),
			"already_sent_via_email": false,
		}
		if err := dbWeb.Model(&reportmodel.ReportScheduled{}).Where("id = ?", reportName).Updates(updateMap).Error; err != nil {
			logrus.Errorf("%s: failed to update scheduled report: %v", eventToDo, err)
		}
	}

	idText, enText, imgSS := GetImgPivotReportFirstSheet(excelFilePath, userLang)
	if imgSS != "" {
		var imgCaption string
		if userLang == "id" {
			imgCaption = "Pivot Report VTR MTI"
		} else {
			imgCaption = "MTI VTR Pivot Report"
		}
		SendImgFileWithStanza(v, stanzaID, originalSenderJID, imgSS, imgCaption, nil, userLang)

		// 🧹 Cleanup: remove the image
		if removeErr := os.Remove(imgSS); removeErr != nil {
			logrus.Errorf("⚠ Gagal menghapus file gambar: %v", removeErr)
		} else {
			logrus.Printf("🧹 Gambar %s berhasil dihapus setelah dikirim.", imgSS)
		}
		// Remove the EXCEL will cause the file to be deleted before its send via report scheduled
		// if removeErr := os.Remove(excelFilePath); removeErr != nil {
		// 	logrus.Errorf("⚠ Gagal menghapus file Excel: %v", removeErr)
		// } else {
		// 	logrus.Printf("🧹 Excel file %s berhasil dihapus setelah digunakan.", excelFilePath)
		// }
	} else {
		sendLangMessageWithStanza(v, stanzaID, originalSenderJID, idText, enText, userLang)
	}
}
