package controllers

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"service-platform/cmd/web_panel/fun"
	"service-platform/cmd/web_panel/model"
	reportmodel "service-platform/cmd/web_panel/model/report_model"
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
	processingReportPenarikanMTIMutex sync.Mutex
	comparingReportPenarikanMTIMutex  sync.Mutex
)

var titleMTIReportPenarikan = []ExcelColumn{
	{ColIndex: "", ColTitle: "Work Order Number", ColSize: 20},
	{ColIndex: "", ColTitle: "Work Type", ColSize: 20},
	{ColIndex: "", ColTitle: "MID", ColSize: 20},
	{ColIndex: "", ColTitle: "TID", ColSize: 20},
	{ColIndex: "", ColTitle: "TID (Previous)", ColSize: 20},
	{ColIndex: "", ColTitle: "Merchant Official Name", ColSize: 20},
	{ColIndex: "", ColTitle: "Merchant Name", ColSize: 20},
	{ColIndex: "", ColTitle: "Address 1-3", ColSize: 20},
	{ColIndex: "", ColTitle: "Contact Person", ColSize: 20},
	{ColIndex: "", ColTitle: "Phone Number", ColSize: 20},
	{ColIndex: "", ColTitle: "Region", ColSize: 20},
	{ColIndex: "", ColTitle: "City", ColSize: 20},
	{ColIndex: "", ColTitle: "ZIP/Postal Code", ColSize: 20},
	{ColIndex: "", ColTitle: "Mobile Number", ColSize: 20},
	{ColIndex: "", ColTitle: "Merchant Segment", ColSize: 20},
	{ColIndex: "", ColTitle: "EDC Type", ColSize: 20},
	{ColIndex: "", ColTitle: "S/N EDC", ColSize: 20},
	{ColIndex: "", ColTitle: "S/N Simcard", ColSize: 20},
	{ColIndex: "", ColTitle: "SIM Card Provider", ColSize: 20},
	{ColIndex: "", ColTitle: "S/N Samcard", ColSize: 20},
	{ColIndex: "", ColTitle: "Vendor", ColSize: 20},
	{ColIndex: "", ColTitle: "EDC Feature", ColSize: 20},
	{ColIndex: "", ColTitle: "EDC Conn Type", ColSize: 20},
	{ColIndex: "", ColTitle: "Work Order Start Date", ColSize: 20},
	{ColIndex: "", ColTitle: "SLA Target Date", ColSize: 20},
	{ColIndex: "", ColTitle: "Work Order End Date", ColSize: 20},
	{ColIndex: "", ColTitle: "EDC Status", ColSize: 20},
	{ColIndex: "", ColTitle: "Version", ColSize: 20},
	{ColIndex: "", ColTitle: "Work Order Status", ColSize: 20},
	{ColIndex: "", ColTitle: "Remarks", ColSize: 20},
	{ColIndex: "", ColTitle: "Pending Owner", ColSize: 20},
	{ColIndex: "", ColTitle: "Pending Reason", ColSize: 20},
	{ColIndex: "", ColTitle: "Engineer", ColSize: 20},
	{ColIndex: "", ColTitle: "Service Point", ColSize: 20},
	{ColIndex: "", ColTitle: "Work Request Number", ColSize: 20},
	{ColIndex: "", ColTitle: "Warehouse", ColSize: 20},
	// Red Mark => Means compared with ODOO
	{ColIndex: "", ColTitle: "STAGE", ColSize: 20},
	{ColIndex: "", ColTitle: "MEMBER BANK", ColSize: 20},
	{ColIndex: "", ColTitle: "REMARK", ColSize: 20},
	{ColIndex: "", ColTitle: "ROOT CAUSE", ColSize: 20},
	{ColIndex: "", ColTitle: "TGL TARIK", ColSize: 20},
	{ColIndex: "", ColTitle: "LINK WOD", ColSize: 20},
}

// saveMTIReportPenarikanBatch inserts a batch of MTIReportPenarikan records into the database using the specified batch size.
// The operation is performed within a single transaction for atomicity.
// If the data slice is empty, it logs a warning and returns an error.
// Logs are generated for the start, success, and failure of the batch insert operation.
//
// Parameters:
//   - db:        The GORM database connection.
//   - data:      Slice of MTIReportPenarikan records to be inserted.
//   - batchSize: Number of records to insert per batch.
//
// Returns:
//   - error: An error if the operation fails, or nil on success.
func saveMTIReportPenarikanBatch(db *gorm.DB, data []reportmodel.MTIReportPenarikan, batchSize int) error {
	if len(data) == 0 {
		logrus.Warn("No data to insert for MTI withdrawal report")
		return errors.New("no data to insert")
	}

	logrus.Infof("Starting batch insert for MTI withdrawal report, total: %d rows, batch size: %d", len(data), batchSize)

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
		return fmt.Errorf("failed to save MTI report penarikan: %w", err)
	}

	logrus.Infof("Successfully inserted %d MTI withdrawal report rows", len(data))
	return nil
}

// processExcelReportPenarikanMTI processes an incoming WhatsApp message containing an Excel report file
// for MTI withdrawal ("penarikan") and saves its contents to the database. The function performs the following steps:
//  1. Validates the sender and ensures only one report is processed at a time using a mutex.
//  2. Notifies the user that the request has been received.
//  3. Validates the presence and structure of the attached document message.
//  4. Downloads the Excel file from WhatsApp and saves it temporarily to disk.
//  5. Opens and validates the Excel file, ensuring the correct sheet and header columns are present.
//  6. Iterates through the rows, mapping each row to the MTIReportPenarikan struct, parsing dates, and collecting any missing or invalid columns.
//  7. Deletes all existing MTIReportPenarikan records from the database before inserting the new batch.
//  8. Optionally notifies the user about any incomplete rows.
//  9. Sends a success message to the user upon completion.
//
// Parameters:
//   - v:        Pointer to the WhatsApp message event containing the document.
//   - user:     Pointer to the user model representing the sender.
//   - userLang: Language code for user-facing messages.
//
// Returns:
//   - error:    An error if any step fails, otherwise nil.
//
// Side Effects:
//   - Sends WhatsApp messages to the user for status updates and error notifications.
//   - Writes and deletes temporary files on disk.
//   - Modifies the MTIReportPenarikan table in the database.
func processExcelReportPenarikanMTI(v *events.Message, user *model.WAPhoneUser, userLang string) error {
	eventToDo := "Processing MTI Report Penarikan"
	originalSenderJID := NormalizeSenderJID(v.Info.Sender.String())
	stanzaID := v.Info.ID

	if originalSenderJID == "" {
		id := "pengirim tidak valid"
		en := "invalid sender"
		sendLangMessageWithStanza(v, stanzaID, originalSenderJID, id, en, userLang)
		logrus.Errorf("%s: %s", eventToDo, "invalid sender JID")
		return errors.New("invalid sender JID")
	}

	if !processingReportPenarikanMTIMutex.TryLock() {
		id := "sedang memproses report penarikan MTI, silakan tunggu sebentar"
		en := "currently processing MTI withdrawal report, please wait a moment"
		sendLangMessageWithStanza(v, stanzaID, originalSenderJID, id, en, userLang)
		logrus.Warnf("%s: %s", eventToDo, "already processing")
		return fmt.Errorf("%s already running, skipping execution", eventToDo)
	}
	defer processingReportPenarikanMTIMutex.Unlock()

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
	directories = append(directories, config.WebPanel.Get().ReportMTI.ReportDir)

	fileReportDir, err := fun.FindValidDirectory(directories)
	if err != nil {
		id := fmt.Sprintf("gagal menemukan direktori yang valid: %v", err)
		en := fmt.Sprintf("failed to find valid directory: %v", err)
		sendLangMessageWithStanza(v, stanzaID, originalSenderJID, id, en, userLang)
		return fmt.Errorf("failed to find valid directory: %v", err)
	}

	fileName := fmt.Sprintf("%s_%v.xlsx", "report_penarikan_mti", fun.GenerateRandomString(20))
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

	findSheet := "list"
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

	var titleMTIReportPenarikanNoRedMark []ExcelColumn
	for _, col := range titleMTIReportPenarikan {
		if col.ColTitle != "STAGE" && col.ColTitle != "MEMBER BANK" && col.ColTitle != "REMARK" && col.ColTitle != "ROOT CAUSE" && col.ColTitle != "TGL TARIK" && col.ColTitle != "LINK WOD" {
			titleMTIReportPenarikanNoRedMark = append(titleMTIReportPenarikanNoRedMark, col)
		}
	}

	if len(headerRow) == 0 || len(headerRow[0]) < len(titleMTIReportPenarikanNoRedMark) {
		id := "baris header tidak valid dalam file Excel untuk report penarikan MTI"
		en := "invalid header row in Excel file for MTI report penarikan"
		sendLangMessageWithStanza(v, stanzaID, originalSenderJID, id, en, userLang)
		_ = os.Remove(tempPath) // Clean up the file if the header row is invalid
		_ = f.Close()
		// Return an error if the header row is invalid
		return errors.New("invalid header row in Excel file for MTI report penarikan")
	}

	// Check if the header row matches the expected titles
	for i, col := range titleMTIReportPenarikanNoRedMark {
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
	var dataForMTIReportPenarikan []reportmodel.MTIReportPenarikan
	for rowIndex, row := range headerRow {
		if rowIndex == 0 {
			continue // Skip the header row
		}
		// Check completeness based on expected columns
		missingCols := []string{}
		for _, col := range titleMTIReportPenarikanNoRedMark {
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
		for _, col := range titleMTIReportPenarikanNoRedMark {
			idx := headerMap[col.ColTitle]
			rowData[col.ColTitle] = row[idx]
		}

		var woStartDate, slaTargetDate, woEndDate *time.Time
		if rowData["Work Order Start Date"] != "" {
			timeValue, err := fun.ParseFlexibleDate(rowData["Work Order Start Date"])
			if err != nil {
				logrus.Errorf("Failed to parse Work Order Start Date for row %d: %v", rowIndex+1, err)
				allMissingCols = append(allMissingCols, fmt.Sprintf("Row %d: Invalid Work Order Start Date format", rowIndex+1))
			} else {
				woStartDate = &timeValue
			}
		}
		if rowData["SLA Target Date"] != "" {
			timeValue, err := fun.ParseFlexibleDate(rowData["SLA Target Date"])
			if err != nil {
				logrus.Errorf("Failed to parse SLA Target Date for row %d: %v", rowIndex+1, err)
				allMissingCols = append(allMissingCols, fmt.Sprintf("Row %d: Invalid SLA Target Date format", rowIndex+1))
			} else {
				slaTargetDate = &timeValue
			}
		}
		if rowData["Work Order End Date"] != "" {
			timeValue, err := fun.ParseFlexibleDate(rowData["Work Order End Date"])
			if err != nil {
				logrus.Errorf("Failed to parse Work Order End Date for row %d: %v", rowIndex+1, err)
				allMissingCols = append(allMissingCols, fmt.Sprintf("Row %d: Invalid Work Order End Date format", rowIndex+1))
			} else {
				woEndDate = &timeValue
			}
		}
		// Subtract 7 hours from the parsed dates if they are not nil
		if woStartDate != nil {
			t := woStartDate.Add(-7 * time.Hour)
			woStartDate = &t
		}
		if slaTargetDate != nil {
			t := slaTargetDate.Add(-7 * time.Hour)
			slaTargetDate = &t
		}
		if woEndDate != nil {
			t := woEndDate.Add(-7 * time.Hour)
			woEndDate = &t
		}

		// Map rowData to MTIReportPenarikan struct
		report := reportmodel.MTIReportPenarikan{
			WorkOrderNumber:      rowData["Work Order Number"],
			WorkType:             rowData["Work Type"],
			MID:                  rowData["MID"],
			TID:                  rowData["TID"],
			TIDPrevious:          rowData["TID (Previous)"],
			MerchantOfficialName: rowData["Merchant Official Name"],
			MerchantName:         rowData["Merchant Name"],
			Address123:           rowData["Address 1-3"],
			ContactPerson:        rowData["Contact Person"],
			PhoneNumber:          rowData["Phone Number"],
			Region:               rowData["Region"],
			City:                 rowData["City"],
			ZipPostalCode:        rowData["ZIP/Postal Code"],
			MobileNumber:         rowData["Mobile Number"],
			MerchantSegment:      rowData["Merchant Segment"],
			EDCType:              rowData["EDC Type"],
			SNEDC:                rowData["S/N EDC"],
			SNSimcard:            rowData["S/N Simcard"],
			SNSamcard:            rowData["S/N Samcard"],
			SimcardProvider:      rowData["SIM Card Provider"],
			Vendor:               rowData["Vendor"],
			EDCFeature:           rowData["EDC Feature"],
			EDCConnType:          rowData["EDC Conn Type"],
			WorkOrderStartDate:   woStartDate,
			SLATargetDate:        slaTargetDate,
			WorkOrderEndDate:     woEndDate,
			EDCStatus:            rowData["EDC Status"],
			Version:              rowData["Version"],
			WorkOrderStatus:      rowData["Work Order Status"],
			Remarks:              rowData["Remarks"],
			PendingOwner:         rowData["Pending Owner"],
			PendingReason:        rowData["Pending Reason"],
			Engineer:             rowData["Engineer"],
			ServicePoint:         rowData["Service Point"],
			WorkRequestNumber:    rowData["Work Request Number"],
			Warehouse:            rowData["Warehouse"],
		}

		dataForMTIReportPenarikan = append(dataForMTIReportPenarikan, report)
	}

	if len(dataForMTIReportPenarikan) == 0 {
		id := "tidak ada data yang ditemukan dalam file report penarikan MTI"
		en := "no data found in MTI withdrawal report file"
		sendLangMessageWithStanza(v, stanzaID, originalSenderJID, id, en, userLang)
		logrus.Warnf("%s: %s", eventToDo, "no data found in file")
		return errors.New("no data found in file")
	}

	batchSize := 1000 // or tune as needed, e.g., 500 or 2000
	tx := dbWeb.Unscoped().Where("id != ?", 0).Delete(&reportmodel.MTIReportPenarikan{})
	if tx.Error != nil {
		return tx.Error
	}
	if tx.RowsAffected == 0 {
		logrus.Debug("No rows were deleted (table might already be empty)")
	}

	if err := saveMTIReportPenarikanBatch(dbWeb, dataForMTIReportPenarikan, batchSize); err != nil {
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

	id = fmt.Sprintf("✅ File report penarikan MTI '%s' berhasil diproses!\n🖥 Data telah disimpan kedalam database.", filename)
	en = fmt.Sprintf("✅ Withdrawal report MTI file '%s' processed successfully!\n🖥 Data has been saved to database.", filename)
	sendLangMessageWithStanza(v, stanzaID, originalSenderJID, id, en, userLang)

	// Yokke x ODOO comparison
	compareMTIReportPenarikanYokkeWithODOO(v, userLang)
	return nil
}

func compareMTIReportPenarikanYokkeWithODOO(v *events.Message, userLang string) {
	eventToDo := ""
	if userLang == "id" {
		eventToDo = "Membandingkan Report Penarikan MTI dengan ODOO"
	} else {
		eventToDo = "Comparing MTI Withdrawal Report with ODOO"
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

	if !comparingReportPenarikanMTIMutex.TryLock() {
		id := "sedang membandingkan report penarikan MTI dengan ODOO, silakan tunggu sebentar"
		en := "currently comparing MTI withdrawal report with ODOO, please wait a moment"
		sendLangMessageWithStanza(v, stanzaID, originalSenderJID, id, en, userLang)
		logrus.Warnf("%s: %s", eventToDo, "already comparing")
		return
	}
	defer comparingReportPenarikanMTIMutex.Unlock()

	// Inform user we've received request
	sendLangMessageWithStanza(
		v,
		stanzaID,
		originalSenderJID,
		"🔍 Memulai perbandingan data report penarikan MTI dengan ODOO...",
		"🔍 Starting comparison of MTI withdrawal report data with ODOO...",
		userLang,
	)

	var yokkeDataMTIReportPenarikan []reportmodel.MTIReportPenarikan
	if err := dbWeb.Find(&yokkeDataMTIReportPenarikan).Error; err != nil {
		id := fmt.Sprintf("gagal mengambil data report penarikan MTI dari database: %v", err)
		en := fmt.Sprintf("failed to retrieve MTI withdrawal report data from database: %v", err)
		sendLangMessageWithStanza(v, stanzaID, originalSenderJID, id, en, userLang)
		logrus.Errorf("%s: %s", eventToDo, "failed to retrieve MTI report penarikan data from database")
		return
	}
	if len(yokkeDataMTIReportPenarikan) == 0 {
		id := "tidak ada data report penarikan MTI yang ditemukan di database"
		en := "no MTI withdrawal report data found in database"
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

	sheetMaster := "list"
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
			Color:   []string{"#f2fce8"},
			Pattern: 1,
		},
		Font: &excelize.Font{
			Bold:  true,
			Color: "#000000",
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
	for i, t := range titleMTIReportPenarikan {
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
		if col.ColTitle == "STAGE" || col.ColTitle == "MEMBER BANK" || col.ColTitle == "REMARK" || col.ColTitle == "ROOT CAUSE" || col.ColTitle == "TGL TARIK" || col.ColTitle == "LINK WOD" {
			f.SetCellStyle(sheetMaster, cell, cell, styleMasterTitleRedMark)
		} else {
			f.SetCellStyle(sheetMaster, cell, cell, styleMasterTitle)
		}
	}
	lastColMaster := fun.GetColName(len(columnsMaster) - 1)
	filterRangeMaster := fmt.Sprintf("A1:%s1", lastColMaster)
	f.AutoFilter(sheetMaster, filterRangeMaster, []excelize.AutoFilterOptions{})

	batchSize := 500
	totalRows := len(yokkeDataMTIReportPenarikan)

	// Process batches sequentially but with timeout protection
	for start := 0; start < totalRows; start += batchSize {
		end := start + batchSize
		if end > totalRows {
			end = totalRows
		}
		batch := yokkeDataMTIReportPenarikan[start:end]

		if len(batch) == 0 {
			logrus.Warnf("No data in batch %d-%d, skipping", start+1, end)
			continue
		}

		// Create timeout protection (can be added to GetODOOMSData function later)

		func() {
			var ODOOJobID []string
			for _, data := range batch {
				if data.WorkOrderNumber != "" {
					ODOOJobID = append(ODOOJobID, data.WorkOrderNumber)
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
				"jsonrpc": config.WebPanel.Get().ApiODOO.JSONRPC,
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

			// Collect all updates for batch processing
			var batchUpdates []map[string]interface{}

			for _, data := range odooData {
				// Find matching MTI report data by Job ID
				for _, mtiData := range batch {
					if mtiData.WorkOrderNumber == data.JobId.String {
						// Found a match - write to Excel
						rowIndex := start + 2 // +2 because Excel is 1-indexed and we have a header row
						for i, yokkeData := range yokkeDataMTIReportPenarikan {
							if yokkeData.WorkOrderNumber == data.JobId.String {
								rowIndex = i + 2 // Found the correct row position
								break
							}
						}

						// Write MTI data to Excel using columnsMaster loop
						mtiValues := []interface{}{
							mtiData.WorkOrderNumber,
							mtiData.WorkType,
							mtiData.MID,
							mtiData.TID,
							mtiData.TIDPrevious,
							mtiData.MerchantOfficialName,
							mtiData.MerchantName,
							mtiData.Address123,
							mtiData.ContactPerson,
							mtiData.PhoneNumber,
							mtiData.Region,
							mtiData.City,
							mtiData.ZipPostalCode,
							mtiData.MobileNumber,
							mtiData.MerchantSegment,
							mtiData.EDCType,
							mtiData.SNEDC,
							mtiData.SNSimcard,
							mtiData.SNSamcard,
							mtiData.SimcardProvider,
							mtiData.Vendor,
							mtiData.EDCFeature,
							mtiData.EDCConnType,
						}

						// Add formatted dates
						if mtiData.WorkOrderStartDate != nil {
							mtiValues = append(mtiValues, mtiData.WorkOrderStartDate.Format("2006-01-02 15:04:05"))
						} else {
							mtiValues = append(mtiValues, "")
						}
						if mtiData.SLATargetDate != nil {
							mtiValues = append(mtiValues, mtiData.SLATargetDate.Format("2006-01-02 15:04:05"))
						} else {
							mtiValues = append(mtiValues, "")
						}
						if mtiData.WorkOrderEndDate != nil {
							mtiValues = append(mtiValues, mtiData.WorkOrderEndDate.Format("2006-01-02 15:04:05"))
						} else {
							mtiValues = append(mtiValues, "")
						}

						// Add remaining fields
						mtiValues = append(mtiValues,
							mtiData.EDCStatus,
							mtiData.Version,
							mtiData.WorkOrderStatus,
							mtiData.Remarks,
							mtiData.PendingOwner,
							mtiData.PendingReason,
							mtiData.Engineer,
							mtiData.ServicePoint,
							mtiData.WorkRequestNumber,
							mtiData.Warehouse,
						)

						// Prepare batch update data
						updateData := map[string]interface{}{
							"work_order_number": mtiData.WorkOrderNumber, // Primary key for identification
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
							// // default:
							// case "pending":
							// case "done":
							// case "cancel":
							// case "cancel new":
							// case "closed":
						}

						if err != nil {
							logrus.Errorf("%s: failed to parse stage ID data for job ID %s: %v", eventToDo, data.JobId.String, err)
						} else {
							updateData["stage"] = odooStage
						}

						_, _, firstTaskReason, _ := setSLAStatus(data.TaskCount.Int, data.SlaDeadline, data.CompleteDatetimeWo, data.WoRemarkTiket, data.TaskType)

						if data.Source.Valid && data.Source.String != "" {
							updateData["member_bank"] = data.Source.String
						}

						if data.WoRemarkTiket.Valid && data.WoRemarkTiket.String != "" {
							updateData["remark"] = data.WoRemarkTiket.String
						}
						if firstTaskReason != "" {
							updateData["root_cause"] = firstTaskReason
						}
						var tglTarikUpdated *time.Time
						if data.CompleteDatetimeWo.Valid && !data.CompleteDatetimeWo.Time.IsZero() {
							tglTarikUpdated = &data.CompleteDatetimeWo.Time
						}
						updateData["tgl_tarik"] = tglTarikUpdated
						if data.LinkWO.Valid && data.LinkWO.String != "" {
							updateData["link_wod"] = data.LinkWO.String
						}

						// Add to batch if there's data to update
						if len(updateData) > 1 { // More than just the primary key
							batchUpdates = append(batchUpdates, updateData)
						}

						// Use columnsMaster loop to write data
						for colIndex, col := range columnsMaster {
							cell := fmt.Sprintf("%s%d", col.ColIndex, rowIndex)

							switch col.ColTitle {
							case "STAGE":
								f.SetCellValue(sheetMaster, cell, odooStage)
							case "MEMBER BANK":
								f.SetCellValue(sheetMaster, cell, data.Source.String)
							case "REMARK":
								if data.WoRemarkTiket.Valid && data.WoRemarkTiket.String != "" {
									f.SetCellValue(sheetMaster, cell, data.WoRemarkTiket.String)
								}
							case "ROOT CAUSE":
								if firstTaskReason != "" {
									f.SetCellValue(sheetMaster, cell, firstTaskReason)
								}
							case "TGL TARIK":
								if tglTarikUpdated != nil {
									f.SetCellValue(sheetMaster, cell, tglTarikUpdated.Format("2006-01-02"))
								}
							case "LINK WOD":
								if data.LinkWO.Valid && data.LinkWO.String != "" {
									f.SetCellHyperLink(sheetMaster, cell, data.LinkWO.String, "External")
									f.SetCellValue(sheetMaster, cell, data.LinkWO.String)
								}
							default:
								// For other columns, set the value from mtiValues if within bounds
								if colIndex < len(mtiValues) && mtiValues[colIndex] != nil && mtiValues[colIndex] != "" {
									f.SetCellValue(sheetMaster, cell, mtiValues[colIndex])
								}
							}
						}

						// Apply style to the row using columnsMaster
						for _, col := range columnsMaster {
							cell := fmt.Sprintf("%s%d", col.ColIndex, rowIndex)
							f.SetCellStyle(sheetMaster, cell, cell, style)
						}

						break // Found the match, no need to continue inner loop
					}
				}
			}

			// Perform batch database updates in a transaction
			if len(batchUpdates) > 0 {
				err := dbWeb.Transaction(func(tx *gorm.DB) error {
					for _, updateData := range batchUpdates {
						workOrderNumber := updateData["work_order_number"]
						delete(updateData, "work_order_number") // Remove primary key from update data

						if err := tx.Model(&reportmodel.MTIReportPenarikan{}).
							Where("work_order_number = ?", workOrderNumber).
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
	pivotMasterDataRange := fmt.Sprintf("%s!$A$1:$%s$%d", sheetMaster, lastColMaster, len(yokkeDataMTIReportPenarikan)+1)
	pivotRange1 := fmt.Sprintf("%s!A4:B50", sheetPivot)
	f.AddPivotTable(&excelize.PivotTableOptions{
		Name:            sheetPivot,
		DataRange:       pivotMasterDataRange,
		PivotTableRange: pivotRange1,
		Rows: []excelize.PivotTableField{
			{Data: "ROOT CAUSE"},
		},
		Columns: []excelize.PivotTableField{
			{Data: "STAGE"},
		},
		Data: []excelize.PivotTableField{
			{Data: "Work Order Number", Subtotal: "count"},
		},
		Filter: []excelize.PivotTableField{
			{Data: "MEMBER BANK"},
		},
		RowGrandTotals:      true,
		ColGrandTotals:      true,
		ShowDrill:           true,
		ShowRowHeaders:      true,
		ShowColHeaders:      true,
		ShowLastColumn:      true,
		PivotTableStyleName: "PivotStyleLight16", // Set your desired style here
	})

	f.SetColWidth(sheetPivot, "A", "A", 30)

	// Set Excel document properties (author, etc.)
	f.SetDocProps(&excelize.DocProperties{
		Creator:        config.WebPanel.Get().Default.PT,
		Title:          "MTI Withdrawal Report Comparison",
		Description:    "Comparison report between Yokke and ODOO for data MTI Withdrawal",
		LastModifiedBy: config.WebPanel.Get().Default.PT + " Service Report",
		Keywords:       "MTI, Report, Withdrawal, Comparison, Yokke, ODOO",
	})

	reportName := "MTI_Report_Penarikan"
	reportFileName := fmt.Sprintf("%s_%s.xlsx", reportName, time.Now().Format("20060102_150405"))
	fileReportDir := filepath.Join(config.WebPanel.Get().ReportMTI.ReportDir, time.Now().Format("2006-01-02"))
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
		excelCaption = "📊 Berikut adalah lampiran dari Report Penarikan MTI"
	} else {
		excelCaption = "📊 Here is the attachment of MTI Withdrawal Report"
	}

	SendExcelFileWithStanza(v, stanzaID, originalSenderJID, excelFilePath, excelCaption, nil, userLang)

	// Add scheduled report Penarikan
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
			imgCaption = "Pivot Report Penarikan MTI"
		} else {
			imgCaption = "MTI Withdrawal Pivot Report"
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
