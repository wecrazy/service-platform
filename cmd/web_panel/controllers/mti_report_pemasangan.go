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
	processingReportPemasanganMTIMutex sync.Mutex
	comparingReportPemasanganMTIMutex  sync.Mutex
)

var titleMTIReportPemasangan = []ExcelColumn{
	{ColIndex: "", ColTitle: "No. Perintah Kerja", ColSize: 20},
	{ColIndex: "", ColTitle: "Aktivitas Kerja", ColSize: 20},
	{ColIndex: "", ColTitle: "MID", ColSize: 20},
	{ColIndex: "", ColTitle: "TID", ColSize: 20},
	{ColIndex: "", ColTitle: "TID (Sebelum)", ColSize: 20},
	{ColIndex: "", ColTitle: "Nama Resmi Merchant", ColSize: 30},
	{ColIndex: "", ColTitle: "Nama Merchant", ColSize: 30},
	{ColIndex: "", ColTitle: "Alamat 1-3", ColSize: 40},
	{ColIndex: "", ColTitle: "Contact Person", ColSize: 25},
	{ColIndex: "", ColTitle: "No. Telepon", ColSize: 20},
	{ColIndex: "", ColTitle: "Region", ColSize: 20},
	{ColIndex: "", ColTitle: "Kota", ColSize: 20},
	{ColIndex: "", ColTitle: "Kode Pos", ColSize: 15},
	{ColIndex: "", ColTitle: "No. HP", ColSize: 20},
	{ColIndex: "", ColTitle: "Merchant Segment", ColSize: 20},
	{ColIndex: "", ColTitle: "Tipe EDC", ColSize: 20},
	{ColIndex: "", ColTitle: "S/N EDC", ColSize: 25},
	{ColIndex: "", ColTitle: "S/N Simcard", ColSize: 25},
	{ColIndex: "", ColTitle: "Penyedia Simcard", ColSize: 20},
	{ColIndex: "", ColTitle: "S/N Samcard", ColSize: 25},
	{ColIndex: "", ColTitle: "Vendor", ColSize: 20},
	{ColIndex: "", ColTitle: "Fitur EDC", ColSize: 20},
	{ColIndex: "", ColTitle: "Tipe koneksi EDC", ColSize: 20},
	{ColIndex: "", ColTitle: "Tgl. Mulai Kerja", ColSize: 20},
	{ColIndex: "", ColTitle: "Tgl. SLA Target", ColSize: 20},
	{ColIndex: "", ColTitle: "Tgl. Selesai Kerja", ColSize: 20},
	{ColIndex: "", ColTitle: "Status EDC", ColSize: 20},
	{ColIndex: "", ColTitle: "Versi", ColSize: 15},
	{ColIndex: "", ColTitle: "Status Perintah Kerja", ColSize: 20},
	{ColIndex: "", ColTitle: "Remarks", ColSize: 30},
	{ColIndex: "", ColTitle: "Pemilik Tertunda", ColSize: 20},
	{ColIndex: "", ColTitle: "Alasan Tertunda", ColSize: 30},
	{ColIndex: "", ColTitle: "Teknisi", ColSize: 20},
	{ColIndex: "", ColTitle: "Service Point", ColSize: 20},
	{ColIndex: "", ColTitle: "No. Permintaan Kerja", ColSize: 25},
	{ColIndex: "", ColTitle: "Gudang", ColSize: 20},
	// Red Mark => Means compared with ODOO
	{ColIndex: "", ColTitle: "STATUS", ColSize: 20},
	{ColIndex: "", ColTitle: "REMARK", ColSize: 20},
	{ColIndex: "", ColTitle: "ROOT CAUSE", ColSize: 20},
	{ColIndex: "", ColTitle: "TGL PASANG", ColSize: 20},
	{ColIndex: "", ColTitle: "LINK WOD", ColSize: 20},
}

// processExcelReportPemasanganMTI processes the uploaded report pemasangan Excel file
func processExcelReportPemasanganMTI(v *events.Message, user *model.WAPhoneUser, userLang string) error {
	eventToDo := "Processing MTI Report Pemasangan"
	originalSenderJID := NormalizeSenderJID(v.Info.Sender.String())
	stanzaID := v.Info.ID

	if originalSenderJID == "" {
		id := "pengirim tidak valid"
		en := "invalid sender"
		sendLangMessageWithStanza(v, stanzaID, originalSenderJID, id, en, userLang)
		logrus.Errorf("%s: %s", eventToDo, "invalid sender JID")
		return fmt.Errorf("%s: invalid sender JID", eventToDo)
	}

	if !processingReportPemasanganMTIMutex.TryLock() {
		id := "sedang memproses report pemasangan MTI, silakan tunggu sebentar"
		en := "currently processing MTI installation report, please wait a moment"
		sendLangMessageWithStanza(v, stanzaID, originalSenderJID, id, en, userLang)
		logrus.Warnf("%s: %s", eventToDo, "already processing")
		return fmt.Errorf("%s already running, skipping execution", eventToDo)
	}
	defer processingReportPemasanganMTIMutex.Unlock()

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

	fileName := fmt.Sprintf("%s_%v.xlsx", "report_pemasangan_mti", fun.GenerateRandomString(20))
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

	findSheet := "Data"
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

	var titleMTIReportPemasanganNoRedMark []ExcelColumn
	for _, col := range titleMTIReportPemasangan {
		if col.ColTitle != "STATUS" && col.ColTitle != "REMARK" && col.ColTitle != "ROOT CAUSE" && col.ColTitle != "TGL PASANG" && col.ColTitle != "LINK WOD" {
			titleMTIReportPemasanganNoRedMark = append(titleMTIReportPemasanganNoRedMark, col)
		}
	}

	if len(headerRow) == 0 || len(headerRow[0]) < len(titleMTIReportPemasanganNoRedMark) {
		id := "baris header tidak valid dalam file Excel untuk report pemasangan MTI"
		en := "invalid header row in Excel file for MTI report pemasangan"
		sendLangMessageWithStanza(v, stanzaID, originalSenderJID, id, en, userLang)
		_ = os.Remove(tempPath) // Clean up the file if the header row is invalid
		_ = f.Close()
		// Return an error if the header row is invalid
		return errors.New("invalid header row in Excel file for MTI report pemasangan")
	}

	// Check if the header row matches the expected titles
	for i, col := range titleMTIReportPemasanganNoRedMark {
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
	var dataForMTIReportPemasangan []reportmodel.MTIReportPemasangan
	for rowIndex, row := range headerRow {
		if rowIndex == 0 {
			continue // Skip the header row
		}
		// Check completeness based on expected columns
		missingCols := []string{}
		for _, col := range titleMTIReportPemasanganNoRedMark {
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
		for _, col := range titleMTIReportPemasanganNoRedMark {
			idx := headerMap[col.ColTitle]
			rowData[col.ColTitle] = row[idx]
		}

		var tglMulaiKerja, tglSLATarget, tglSelesaiKerja *time.Time
		if rowData["Tgl. Mulai Kerja"] != "" {
			timeValue, err := fun.ParseFlexibleDate(rowData["Tgl. Mulai Kerja"])
			if err != nil {
				logrus.Errorf("Failed to parse Tgl. Mulai Kerja for row %d: %v", rowIndex+1, err)
				allMissingCols = append(allMissingCols, fmt.Sprintf("Row %d: Invalid Tgl. Mulai Kerja format", rowIndex+1))
			} else {
				tglMulaiKerja = &timeValue
			}
		}
		if rowData["Tgl. SLA Target"] != "" {
			timeValue, err := fun.ParseFlexibleDate(rowData["Tgl. SLA Target"])
			if err != nil {
				logrus.Errorf("Failed to parse Tgl. SLA Target for row %d: %v", rowIndex+1, err)
				allMissingCols = append(allMissingCols, fmt.Sprintf("Row %d: Invalid Tgl. SLA Target format", rowIndex+1))
			} else {
				tglSLATarget = &timeValue
			}
		}
		if rowData["Tgl. Selesai Kerja"] != "" {
			timeValue, err := fun.ParseFlexibleDate(rowData["Tgl. Selesai Kerja"])
			if err != nil {
				logrus.Errorf("Failed to parse Tgl. Selesai Kerja for row %d: %v", rowIndex+1, err)
				allMissingCols = append(allMissingCols, fmt.Sprintf("Row %d: Invalid Tgl. Selesai Kerja format", rowIndex+1))
			} else {
				tglSelesaiKerja = &timeValue
			}
		}
		// Subtract 7 hours from the parsed dates if they are not nil
		if tglMulaiKerja != nil {
			t := tglMulaiKerja.Add(-7 * time.Hour)
			tglMulaiKerja = &t
		}
		if tglSLATarget != nil {
			t := tglSLATarget.Add(-7 * time.Hour)
			tglSLATarget = &t
		}
		if tglSelesaiKerja != nil {
			t := tglSelesaiKerja.Add(-7 * time.Hour)
			tglSelesaiKerja = &t
		}

		// Map rowData to MTIReportPemasangan struct
		report := reportmodel.MTIReportPemasangan{
			NoPerintahKerja:     rowData["No. Perintah Kerja"],
			AktivitasKerja:      rowData["Aktivitas Kerja"],
			MID:                 rowData["MID"],
			TID:                 rowData["TID"],
			TIDSebelum:          rowData["TID (Sebelum)"],
			NamaResmiMerchant:   rowData["Nama Resmi Merchant"],
			NamaMerchant:        rowData["Nama Merchant"],
			Alamat123:           rowData["Alamat 1-3"],
			ContactPerson:       rowData["Contact Person"],
			NoTelepon:           rowData["No. Telepon"],
			Region:              rowData["Region"],
			Kota:                rowData["Kota"],
			KodePos:             rowData["Kode Pos"],
			NoHP:                rowData["No. HP"],
			MerchantSegment:     rowData["Merchant Segment"],
			TipeEDC:             rowData["Tipe EDC"],
			SNEDC:               rowData["S/N EDC"],
			SNSimcard:           rowData["S/N Simcard"],
			PenyediaSimcard:     rowData["Penyedia Simcard"],
			SNSamcard:           rowData["S/N Samcard"],
			Vendor:              rowData["Vendor"],
			FiturEDC:            rowData["Fitur EDC"],
			TipeKoneksiEDC:      rowData["Tipe koneksi EDC"],
			TglMulaiKerja:       tglMulaiKerja,
			TglSLATarget:        tglSLATarget,
			TglSelesaiKerja:     tglSelesaiKerja,
			StatusEDC:           rowData["Status EDC"],
			Versi:               rowData["Versi"],
			StatusPerintahKerja: rowData["Status Perintah Kerja"],
			Remarks:             rowData["Remarks"],
			PemilikTertunda:     rowData["Pemilik Tertunda"],
			AlasanTertunda:      rowData["Alasan Tertunda"],
			Teknisi:             rowData["Teknisi"],
			ServicePoint:        rowData["Service Point"],
			NoPermintaanKerja:   rowData["No. Permintaan Kerja"],
			Gudang:              rowData["Gudang"],
		}

		dataForMTIReportPemasangan = append(dataForMTIReportPemasangan, report)
	}

	if len(dataForMTIReportPemasangan) == 0 {
		id := "tidak ada data yang ditemukan dalam file report pemasangan MTI"
		en := "no data found in MTI report pemasangan file"
		sendLangMessageWithStanza(v, stanzaID, originalSenderJID, id, en, userLang)
		logrus.Warnf("%s: %s", eventToDo, "no data found in file")
		return fmt.Errorf("%s: no data found in file", eventToDo)
	}

	batchSize := 1000 // or tune as needed, e.g., 500 or 2000
	tx := dbWeb.Unscoped().Where("id != ?", 0).Delete(&reportmodel.MTIReportPemasangan{})
	if tx.Error != nil {
		return tx.Error
	}
	if tx.RowsAffected == 0 {
		logrus.Debug("No rows were deleted (table might already be empty)")
	}
	if err := saveMTIReportPemasanganBatch(dbWeb, dataForMTIReportPemasangan, batchSize); err != nil {
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

	id = fmt.Sprintf("✅ File report pemasangan MTI '%s' berhasil diproses!\n🖥 Data telah disimpan kedalam database.", filename)
	en = fmt.Sprintf("✅ Installation report MTI file '%s' processed successfully!\n🖥 Data has been saved to database.", filename)
	sendLangMessageWithStanza(v, stanzaID, originalSenderJID, id, en, userLang)

	// Yokke x ODOO comparison
	compareMTIReportPemasanganYokkeWithODOO(v, userLang)
	return nil
}

func saveMTIReportPemasanganBatch(db *gorm.DB, data []reportmodel.MTIReportPemasangan, batchSize int) error {
	if len(data) == 0 {
		logrus.Warn("No data to insert for MTI report pemasangan")
		return errors.New("no data to insert")
	}

	logrus.Infof("Starting batch insert for MTI report pemasangan, total: %d rows, batch size: %d", len(data), batchSize)

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
		return fmt.Errorf("failed to save MTI report pemasangan: %w", err)
	}

	logrus.Infof("Successfully inserted %d MTI report pemasangan rows", len(data))
	return nil
}

func compareMTIReportPemasanganYokkeWithODOO(v *events.Message, userLang string) {
	eventToDo := ""
	if userLang == "id" {
		eventToDo = "Perbandingan Data Report Pemasangan MTI dengan ODOO"
	} else {
		eventToDo = "Comparing MTI Installation Report Data with ODOO"
	}
	originalSenderJID := NormalizeSenderJID(v.Info.Sender.String())
	stanzaID := v.Info.ID

	if originalSenderJID == "" {
		id := "pengirim tidak valid"
		en := "invalid sender"
		sendLangMessageWithStanza(v, stanzaID, originalSenderJID, id, en, userLang)
		logrus.Errorf("%s: %s", eventToDo, "invalid sender JID")
	}

	if !comparingReportPemasanganMTIMutex.TryLock() {
		id := "sedang melakukan perbandingan report pemasangan MTI dengan ODOO, silakan tunggu sebentar"
		en := "currently comparing MTI report pemasangan with ODOO, please wait a moment"
		sendLangMessageWithStanza(v, stanzaID, originalSenderJID, id, en, userLang)
		logrus.Warnf("%s: %s", eventToDo, "already processing")
	}
	defer comparingReportPemasanganMTIMutex.Unlock()

	// Inform user we've received request
	sendLangMessageWithStanza(
		v,
		stanzaID,
		originalSenderJID,
		"🔍 Memulai perbandingan data report pemasangan MTI dengan ODOO...",
		"🔍 Starting comparison of MTI installation report data with ODOO...",
		userLang,
	)

	var yokkeDataMTIReportPemasangan []reportmodel.MTIReportPemasangan
	if err := dbWeb.Find(&yokkeDataMTIReportPemasangan).Error; err != nil {
		id := fmt.Sprintf("gagal mengambil data report pemasangan MTI dari database: %v", err)
		en := fmt.Sprintf("failed to retrieve MTI report pemasangan data from database: %v", err)
		sendLangMessageWithStanza(v, stanzaID, originalSenderJID, id, en, userLang)
		logrus.Errorf("%s: %v", eventToDo, err)
		return
	}
	if len(yokkeDataMTIReportPemasangan) == 0 {
		id := "tidak ada data report pemasangan MTI yang ditemukan di database"
		en := "no MTI report pemasangan data found in database"
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
			logrus.Errorf("%s: failed to close Excel file: %v", eventToDo, err)
		}
		// if err := os.Remove(f.Path); err != nil {
		// 	logrus.Errorf("Failed to remove temporary Excel file: %v", err)
		// }
	}()

	sheetMaster := "Data"
	sheetPivot := "PIVOT"
	f.NewSheet(sheetPivot)
	f.NewSheet(sheetMaster)
	f.DeleteSheet("Sheet1") // Delete the default sheet

	styleMasterTitle, _ := f.NewStyle(&excelize.Style{
		Alignment: &excelize.Alignment{
			Horizontal: "center",
			Vertical:   "center",
		},
		Fill: excelize.Fill{
			Type:    "pattern",
			Color:   []string{"#46CA12"},
			Pattern: 1,
		},
		Font: &excelize.Font{
			Bold:  true,
			Color: "#FFFFFF",
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
	for i, t := range titleMTIReportPemasangan {
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
		if col.ColTitle == "STATUS" || col.ColTitle == "REMARK" || col.ColTitle == "ROOT CAUSE" || col.ColTitle == "TGL PASANG" || col.ColTitle == "LINK WOD" {
			f.SetCellStyle(sheetMaster, cell, cell, styleMasterTitleRedMark)
		} else {
			f.SetCellStyle(sheetMaster, cell, cell, styleMasterTitle)
		}
	}
	lastColMaster := fun.GetColName(len(columnsMaster) - 1)
	filterRangeMaster := fmt.Sprintf("A1:%s1", lastColMaster)
	f.AutoFilter(sheetMaster, filterRangeMaster, []excelize.AutoFilterOptions{})

	batchSize := 500
	totalRows := len(yokkeDataMTIReportPemasangan)
	for start := 0; start < totalRows; start += batchSize {
		end := start + batchSize
		if end > totalRows {
			end = totalRows
		}
		batch := yokkeDataMTIReportPemasangan[start:end]

		if len(batch) == 0 {
			logrus.Warnf("No data in batch %d-%d, skipping", start+1, end)
			continue
		}

		var ODOOJobID []string
		for _, data := range batch {
			if data.NoPerintahKerja != "" {
				ODOOJobID = append(ODOOJobID, data.NoPerintahKerja)
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
			continue
		}

		ODOOResponse, err := GetODOOMSData(string(payloadBytes))
		if err != nil {
			logrus.Errorf("%s: %v", eventToDo, err)
			continue
		}
		ODOOResponseArray, ok := ODOOResponse.([]interface{})
		if !ok {
			logrus.Errorf("%s: expected ODOO response to be an array, got %v", eventToDo, ODOOResponse)
			continue
		}
		ODOOResponseBytes, err := json.Marshal(ODOOResponseArray)
		if err != nil {
			logrus.Errorf("%s: failed to marshal ODOO response: %v", eventToDo, err)
			continue
		}

		var odooData []OdooTicketDataRequestItem
		if err := json.Unmarshal(ODOOResponseBytes, &odooData); err != nil {
			logrus.Errorf("%s: failed to unmarshal ODOO response: %v", eventToDo, err)
			continue
		}

		if len(odooData) == 0 {
			logrus.Warnf("%s: no ODOO data found for batch %d-%d", eventToDo, start+1, end)
			continue
		}

		// Collect all updates for batch processing
		var batchUpdates []map[string]interface{}

		for _, data := range odooData {
			// Find matching MTI report data by Job ID
			for _, mtiData := range batch {
				if mtiData.NoPerintahKerja == data.JobId.String {
					// Found a match - write to Excel
					rowIndex := start + 2 // +2 because Excel is 1-indexed and we have a header row
					for i, yokkeData := range yokkeDataMTIReportPemasangan {
						if yokkeData.NoPerintahKerja == data.JobId.String {
							rowIndex = i + 2 // Found the correct row position
							break
						}
					}

					// Write MTI data to Excel using columnsMaster loop
					mtiValues := []interface{}{
						mtiData.NoPerintahKerja,
						mtiData.AktivitasKerja,
						mtiData.MID,
						mtiData.TID,
						mtiData.TIDSebelum,
						mtiData.NamaResmiMerchant,
						mtiData.NamaMerchant,
						mtiData.Alamat123,
						mtiData.ContactPerson,
						mtiData.NoTelepon,
						mtiData.Region,
						mtiData.Kota,
						mtiData.KodePos,
						mtiData.NoHP,
						mtiData.MerchantSegment,
						mtiData.TipeEDC,
						mtiData.SNEDC,
						mtiData.SNSimcard,
						mtiData.PenyediaSimcard,
						mtiData.SNSamcard,
						mtiData.Vendor,
						mtiData.FiturEDC,
						mtiData.TipeKoneksiEDC,
					}

					// Add formatted dates
					if mtiData.TglMulaiKerja != nil {
						mtiValues = append(mtiValues, mtiData.TglMulaiKerja.Format("2006-01-02 15:04:05"))
					} else {
						mtiValues = append(mtiValues, "")
					}
					if mtiData.TglSLATarget != nil {
						mtiValues = append(mtiValues, mtiData.TglSLATarget.Format("2006-01-02"))
					} else {
						mtiValues = append(mtiValues, "")
					}
					if mtiData.TglSelesaiKerja != nil {
						mtiValues = append(mtiValues, mtiData.TglSelesaiKerja.Format("2006-01-02 15:04:05"))
					} else {
						mtiValues = append(mtiValues, "")
					}

					// Add remaining fields
					mtiValues = append(mtiValues,
						mtiData.StatusEDC,
						mtiData.Versi,
						mtiData.StatusPerintahKerja,
						mtiData.Remarks,
						mtiData.PemilikTertunda,
						mtiData.AlasanTertunda,
						mtiData.Teknisi,
						mtiData.ServicePoint,
						mtiData.NoPermintaanKerja,
						mtiData.Gudang,
					)

					// Prepare batch update data
					updateData := map[string]interface{}{
						"no_perintah_kerja": mtiData.NoPerintahKerja, // Primary key for identification
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
						updateData["status"] = odooStage
					}

					_, _, firstTaskReason, _ := setSLAStatus(data.TaskCount.Int, data.SlaDeadline, data.CompleteDatetimeWo, data.WoRemarkTiket, data.TaskType)

					if data.WoRemarkTiket.Valid && data.WoRemarkTiket.String != "" {
						updateData["remark"] = data.WoRemarkTiket.String
					}
					if firstTaskReason != "" {
						updateData["root_cause"] = firstTaskReason
					}
					var tglPasangUpdated *time.Time
					if data.CompleteDatetimeWo.Valid && !data.CompleteDatetimeWo.Time.IsZero() {
						tglPasangUpdated = &data.CompleteDatetimeWo.Time
					}
					updateData["tgl_pemasangan"] = tglPasangUpdated
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
						case "STATUS":
							f.SetCellValue(sheetMaster, cell, odooStage)
						case "REMARK":
							if data.WoRemarkTiket.Valid && data.WoRemarkTiket.String != "" {
								f.SetCellValue(sheetMaster, cell, data.WoRemarkTiket.String)
							}
						case "ROOT CAUSE":
							if firstTaskReason != "" {
								f.SetCellValue(sheetMaster, cell, firstTaskReason)
							}
						case "TGL PASANG":
							if tglPasangUpdated != nil {
								f.SetCellValue(sheetMaster, cell, tglPasangUpdated.Format("2006-01-02"))
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
					noPerintahKerja := updateData["no_perintah_kerja"]
					delete(updateData, "no_perintah_kerja") // Remove primary key from update data

					if err := tx.Model(&reportmodel.MTIReportPemasangan{}).
						Where("no_perintah_kerja = ?", noPerintahKerja).
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
	}

	/*
		PIVOT
	*/
	pivotMasterDataRange := fmt.Sprintf("%s!$A$1:$%s$%d", sheetMaster, lastColMaster, len(yokkeDataMTIReportPemasangan)+1)
	pivotRange1 := fmt.Sprintf("%s!A4:B50", sheetPivot)
	pivotRange2 := fmt.Sprintf("%s!D4:E50", sheetPivot)
	f.AddPivotTable(&excelize.PivotTableOptions{
		Name:            sheetPivot,
		DataRange:       pivotMasterDataRange,
		PivotTableRange: pivotRange1,
		Rows: []excelize.PivotTableField{
			{Data: "STATUS"},
		},
		Data: []excelize.PivotTableField{
			{Data: "No. Perintah Kerja", Subtotal: "count"},
		},
		RowGrandTotals:      true,
		ColGrandTotals:      true,
		ShowDrill:           true,
		ShowRowHeaders:      true,
		ShowColHeaders:      true,
		ShowLastColumn:      true,
		PivotTableStyleName: "PivotStyleLight10", // Set your desired style here
	})
	f.AddPivotTable(&excelize.PivotTableOptions{
		Name:            sheetPivot,
		DataRange:       pivotMasterDataRange,
		PivotTableRange: pivotRange2,
		Rows: []excelize.PivotTableField{
			{Data: "ROOT CAUSE"},
		},
		Data: []excelize.PivotTableField{
			{Data: "No. Perintah Kerja", Subtotal: "count"},
		},
		RowGrandTotals:      true,
		ColGrandTotals:      true,
		ShowDrill:           true,
		ShowRowHeaders:      true,
		ShowColHeaders:      true,
		ShowLastColumn:      true,
		PivotTableStyleName: "PivotStyleDark9", // Set your desired style here
	})

	f.SetColWidth(sheetPivot, "A", "A", 30)
	f.SetColWidth(sheetPivot, "B", "B", 10)
	f.SetColWidth(sheetPivot, "D", "D", 55)
	f.SetColWidth(sheetPivot, "E", "E", 10)

	// Set Excel document properties (author, etc.)
	f.SetDocProps(&excelize.DocProperties{
		Creator:        config.GetConfig().Default.PT,
		Title:          "MTI Installation Report Comparison",
		Description:    "Comparison report between Yokke and ODOO for data MTI Installation",
		LastModifiedBy: config.GetConfig().Default.PT + " Service Report",
		Keywords:       "MTI, Report, Installation, Comparison, Yokke, ODOO",
	})

	reportName := "MTI_Report_Pemasangan"
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
		excelCaption = "📊 Berikut adalah lampiran dari Report Pemasangan MTI"
	} else {
		excelCaption = "📊 Here is the attachment of MTI Installation Report"
	}

	SendExcelFileWithStanza(v, stanzaID, originalSenderJID, excelFilePath, excelCaption, nil, userLang)

	// Add scheduled report Pemasangan
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
			imgCaption = "Pivot Report Pemasangan MTI"
		} else {
			imgCaption = "MTI Installation Pivot Report"
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
