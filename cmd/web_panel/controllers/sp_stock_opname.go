package controllers

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"mime"
	"net/http"
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"runtime"
	"service-platform/cmd/web_panel/config"
	"service-platform/cmd/web_panel/fun"
	"service-platform/cmd/web_panel/internal/gormdb"
	"service-platform/cmd/web_panel/model"
	sptechnicianmodel "service-platform/cmd/web_panel/model/sp_technician_model"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"codeberg.org/go-pdf/fpdf"
	"github.com/TigorLazuardi/tanggal"
	"github.com/gin-gonic/gin"
	htgotts "github.com/hegedustibor/htgo-tts"
	"github.com/hegedustibor/htgo-tts/handlers"
	"github.com/hegedustibor/htgo-tts/voices"
	"github.com/samber/lo"
	"github.com/sirupsen/logrus"
	"go.mau.fi/whatsmeow"
	"go.mau.fi/whatsmeow/proto/waE2E"
	"go.mau.fi/whatsmeow/types"
	"go.mau.fi/whatsmeow/types/events"
	"google.golang.org/protobuf/proto"
	"gorm.io/gorm"
)

var (
	checkSPStockOpnameMutex               sync.Mutex
	WhatsappEventMsgForSPStockOpname      *events.Message
	WhatsappEventsReceiptForSPStockOpname *events.Receipt
)

type DetailedOperationsAPKStockOpname struct {
	Product string
	SN      string
	From    string
	To      string
	Status  string
}

type DataStockOpnameAggregate struct {
	ID                  uint
	TicketSO            string
	ScheduledDate       *time.Time
	Responsible         string
	SourceDocument      string
	SourceLocation      string
	LocDestinationCat   string
	DestLocation        string
	Company             string
	LinkBAST            string
	Status              string
	DetailOperationsAPK []DetailedOperationsAPKStockOpname
}

type TechnicianStockOpnameAggregateData struct {
	TechnicianName string
	Name           string
	SPL            string
	SAC            string
	TechEmail      string
	TechNoHP       string
	DataSO         []DataStockOpnameAggregate
}

type ReasonSPSOCannotBeSend struct {
	Technician string
	Reason     string
}

// helper to check duplicate SO ID
func containsSO(list []DataStockOpnameAggregate, id uint) bool {
	for _, item := range list {
		if item.ID == id {
			return true
		}
	}
	return false
}

// sort DataStockOpnameAggregate by ScheduledDate ascending, with nil dates at the end
func sortDataSOByDateAsc(data []DataStockOpnameAggregate) {
	sort.Slice(data, func(i, j int) bool {

		// Both nil → consider equal (keep order)
		if data[i].ScheduledDate == nil && data[j].ScheduledDate == nil {
			return false
		}

		// Nil is considered "later" → non-nil should come first
		if data[i].ScheduledDate == nil {
			return false
		}
		if data[j].ScheduledDate == nil {
			return true
		}

		// Normal compare
		return data[i].ScheduledDate.Before(*data[j].ScheduledDate)
	})
}

func resetSPStockOpnameForTechnician(technicians []string) error {
	if len(technicians) == 0 {
		return errors.New("no technician found to reset his sp status")
	}

	dbWeb := gormdb.Databases.Web

	var spList []sptechnicianmodel.SPofStockOpname
	err := dbWeb.Model(&sptechnicianmodel.SPofStockOpname{}).
		Where("technician NOT IN ?", technicians).
		Where("is_got_sp3 = ?", false).
		Find(&spList).Error

	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil
		}
		return err
	}

	if len(spList) == 0 {
		return nil
	}

	for _, spItem := range spList {
		var repliedMsg sptechnicianmodel.SPStockOpnameWhatsappMessage
		err := dbWeb.Model(&sptechnicianmodel.SPStockOpnameWhatsappMessage{}).
			Where("technician_got_sp_id = ?", spItem.ID).
			Where("whatsapp_reply_text IS NOT NULL AND whatsapp_reply_text != ''").
			First(&repliedMsg).Error

		if err == nil {
			if delErr := dbWeb.Delete(&spItem).Error; delErr != nil {
				logrus.Errorf("Failed to reset SP Stock Opname ID %d for technician %s: %v", spItem.ID, spItem.Technician, delErr)
			}
			logrus.Infof("Reset SP Stock Opname ID %d for technician %s as they have replied to WhatsApp messages", spItem.ID, spItem.Technician)
		}
	}

	return nil
}

func formatTanggalSOScheduled(t *time.Time) string {
	tanggalJadwal, err := tanggal.Papar(*t, "Jakarta", tanggal.WIB)
	if err != nil {
		return t.Format("02 January 2006 15:04")
	}

	return tanggalJadwal.Format(" ", []tanggal.Format{
		tanggal.NamaHariDenganKoma,
		tanggal.Hari,
		tanggal.NamaBulan,
		tanggal.Tahun,
		tanggal.Pukul,
	})
}

func BuildPelanggaranSentence(datas []DataStockOpnameAggregate) string {
	if len(datas) == 0 {
		return ""
	}

	// Case: Only one SO → follow your original grammar style
	if len(datas) == 1 {
		so := datas[0]
		if so.ScheduledDate == nil {
			return ""
		}

		tglFormatted := formatTanggalSOScheduled(so.ScheduledDate)

		sentence := fmt.Sprintf(
			"Tidak melakukan Stock Opname yang dijadwalkan pada %s",
			tglFormatted,
		)

		if so.SourceDocument != "" {
			sentence += fmt.Sprintf(" untuk project %s", so.SourceDocument)
		}
		if so.DestLocation != "" {
			sentence += fmt.Sprintf(", dengan lokasi tujuan %s", so.DestLocation)
		}

		return sentence
	}

	// Case: MULTIPLE SO → bullet list, clean grammar
	var listItems []string

	for _, so := range datas {
		if so.ScheduledDate == nil {
			continue
		}

		tgl := formatTanggalSOScheduled(so.ScheduledDate)

		detail := tgl

		if so.SourceDocument != "" || so.DestLocation != "" {
			detail += " ("
			if so.SourceDocument != "" {
				detail += fmt.Sprintf("Project: %s", so.SourceDocument)
			}
			if so.SourceDocument != "" && so.DestLocation != "" {
				detail += ", "
			}
			if so.DestLocation != "" {
				detail += fmt.Sprintf("Lokasi Tujuan: %s", so.DestLocation)
			}
			detail += ")"
		}

		listItems = append(listItems, "- "+detail)
	}

	if len(listItems) == 0 {
		return ""
	}

	return fmt.Sprintf("Tidak melakukan %d Stock Opname yang dijadwalkan pada:\n", len(listItems)) +
		strings.Join(listItems, "\n")
}

func CheckSPStockOpname() error {
	taskDoing := "Check SP of Stock Opname that technician/SPL not doing"
	logrus.Infof("Starting task: %s", taskDoing)

	if !checkSPStockOpnameMutex.TryLock() {
		logrus.Infof("Task %s is already running, skipping this run", taskDoing)
		return nil
	}
	defer checkSPStockOpnameMutex.Unlock()

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
		return err
	}

	ODOOResp, err := GetODOOMSData(string(payloadBytes))
	if err != nil {
		return err
	}

	ODOORespArray, ok := ODOOResp.([]any)
	if !ok {
		errMsg := "Failed to convert ODOO response to array / []any"
		return errors.New(errMsg)
	}

	ids := extractUniqueIDs(ODOORespArray)

	if len(ids) == 0 {
		logrus.Infof("No Stock Opname found that match the criteria")
		return nil
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

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Minute)
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
		return errors.New(errMsg)
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
		return fmt.Errorf("failed to get Technician ODOO MS data: %v", err)
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
		return nil
	}

	for _, tech := range technicianMap {
		sortDataSOByDateAsc(tech.DataSO)
	}

	// Greeting logic (ensuring correct 24-hour format)
	hour := timeNow.Hour()
	var greetingID string
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

	techniciansGotSP = lo.Uniq(techniciansGotSP)
	sort.Strings(techniciansGotSP) // ASC order

	if err := resetSPStockOpnameForTechnician(techniciansGotSP); err != nil {
		return fmt.Errorf("failed to reset SP Stock Opname for technicians: %v", err)
	}

	dbWeb := gormdb.Databases.Web
	maxResponseSPAtHour := config.GetConfig().StockOpname.MaxResponseSPStockOpnameAtHour

	technicianListCannotGetSP := []ReasonSPSOCannotBeSend{}

	// Try iterate each technician to send its SP of not doing Stock Opname
	for _, techData := range technicianMap {
		var alreadyGotSPToday, isSkipped, needSendSPToSPL bool = false, false, false

		var dataSP sptechnicianmodel.SPofStockOpname
		err := dbWeb.Model(&sptechnicianmodel.SPofStockOpname{}).
			Where("technician = ?", techData.TechnicianName).
			First(&dataSP).Error
		if err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				// Skip logging
			} else {
				logrus.Errorf("Failed to fetch SPofStockOpname for technician %s: %v", techData.TechnicianName, err)
			}
		}

		if dataSP.GotSP1At != nil {
			if fun.IsSameDay(*dataSP.GotSP1At, timeNow) {
				alreadyGotSPToday = true
			}
		}
		if dataSP.GotSP2At != nil {
			if fun.IsSameDay(*dataSP.GotSP2At, timeNow) {
				alreadyGotSPToday = true
			}
		}
		if dataSP.GotSP3At != nil {
			if fun.IsSameDay(*dataSP.GotSP3At, timeNow) {
				alreadyGotSPToday = true
			}
		}

		skippedTechnician := []string{
			"",
		}

		for _, skipTech := range skippedTechnician {
			if techData.TechnicianName == skipTech {
				isSkipped = true
				break
			}
		}

		if alreadyGotSPToday {
			isSkipped = true
		}

		if isSkipped {
			logrus.Infof("Skipping technician %s for Stock Opname SP check", techData.TechnicianName)
			continue
		}

		// Get SAC data from config
		var jidStrSAC, jidStrSPL, namaSPL string
		ODOOMSSAC := config.GetConfig().ODOOMSSAC
		SACData, ok := ODOOMSSAC[techData.SAC]
		if !ok {
			logrus.Errorf("SAC data not found for SAC %s of technician %s", techData.SAC, techData.TechnicianName)
			// technicianListCannotGetSP = append(technicianListCannotGetSP, techData.TechnicianName)
			technicianListCannotGetSP = append(technicianListCannotGetSP, ReasonSPSOCannotBeSend{
				Technician: techData.TechnicianName,
				Reason:     "Data SAC tidak ditemukan",
			})
			continue
		} else {
			jidStrSAC = fmt.Sprintf("%s@s.whatsapp.net", SACData.PhoneNumber)
		}

		if techData.TechnicianName != techData.SPL {
			needSendSPToSPL = true
		}

		sanitizedPhoneTech, err := fun.SanitizePhoneNumber(techData.TechNoHP)
		if err != nil {
			logrus.Errorf("got invalid phone number : %v", err)
			technicianListCannotGetSP = append(technicianListCannotGetSP, ReasonSPSOCannotBeSend{
				Technician: techData.TechnicianName,
				Reason:     "No HP tidak valid",
			})
			continue
		}

		if !needSendSPToSPL {
			jidStrSPL = fmt.Sprintf("62%s@s.whatsapp.net", sanitizedPhoneTech)
			namaSPL = techData.Name
		} else {
			if splData, exists := TechODOOMSData[techData.SPL]; exists {
				sanitizedPhoneSPL, err := fun.SanitizePhoneNumber(splData.NoHP)
				if err != nil {
					logrus.Errorf("got invalid phone number : %v", err)
					technicianListCannotGetSP = append(technicianListCannotGetSP, ReasonSPSOCannotBeSend{
						Technician: techData.TechnicianName,
						Reason:     "No HP SPL tidak valid",
					})
					continue
				}
				jidStrSPL = fmt.Sprintf("62%s@s.whatsapp.net", sanitizedPhoneSPL)
				namaSPL = splData.Name
			}
		}

		var pelanggaranID string
		pelanggaranID = BuildPelanggaranSentence(techData.DataSO)
		if pelanggaranID == "" {
			logrus.Infof("No pelanggaran sentence built for technician %s, skipping SP creation", techData.TechnicianName)
			technicianListCannotGetSP = append(technicianListCannotGetSP, ReasonSPSOCannotBeSend{
				Technician: techData.TechnicianName,
				Reason:     "Tidak ada data pelanggaran",
			})
			continue
		}

		gotSPDate := time.Now()
		dataTechUpdated := map[string]any{}
		dataSPTechNeedToUpdate := true

		t := time.Date(
			time.Now().Year(),
			time.Now().Month(),
			time.Now().Day(),
			maxResponseSPAtHour, 0, 0, 0,
			time.Now().Location(),
		)
		var tglMaxResponSP string
		tgl, err := tanggal.Papar(t, "Jakarta", tanggal.WIB)
		if err != nil {
			tglMaxResponSP = fmt.Sprintf("%d:00", maxResponseSPAtHour)
		}
		tglMaxResponSP = tgl.Format(" ", []tanggal.Format{
			tanggal.PukulDenganDetik,
		})

		var processedSP int

		switch {
		// Surat Peringatan 1
		case dataSP.IsGotSP1 == false && dataSP.IsGotSP2 == false && dataSP.IsGotSP3 == false:
			dataSPTechNeedToUpdate = false

			err, fileTTS := CreateNotifSoundForSPStockOpname(techData.Name, 1)
			if err != nil {
				logrus.Errorf("failed to generate mp3 tts for sp-1 %s : %v", techData.TechnicianName, err)
				continue
			}

			// Placeholders for SP pdf
			noSurat, err := IncrementNomorSuratSP(dbWeb, "LAST_NOMOR_SURAT_SP1_GENERATED")
			if err != nil {
				logrus.Errorf("Failed to increment nomor surat SP-1: %v", err)
				continue
			}
			var noSuratStr string
			if noSurat < 1000 {
				noSuratStr = fmt.Sprintf("%03d", noSurat)
			} else {
				noSuratStr = fmt.Sprintf("%d", noSurat)
			}
			monthRoman, err := fun.MonthToRoman(int(time.Now().Month()))
			if err != nil {
				logrus.Errorf("Failed to convert month to roman numeral: %v", err)
				continue
			}
			splCity := getSPLCity(techData.SPL)
			if splCity == "" {
				logrus.Warnf("SPL city not found for SPL %s, defaulting to 'Unknown'", techData.SPL)
				splCity = "Unknown"
			}
			tanggalSP1Terbit, err := tanggal.Papar(time.Now(), "Jakarta", tanggal.WIB)
			if err != nil {
				logrus.Errorf("Failed to get formatted date for SP1: %v", err)
				continue
			}
			tglSP1Diterbitkan := tanggalSP1Terbit.Format(" ", []tanggal.Format{
				tanggal.Hari,      // 27
				tanggal.NamaBulan, // Maret
				tanggal.Tahun,     // 2025
			})

			placeholdersSP := map[string]string{
				"$nomor_surat":            noSuratStr,
				"$bulan_romawi":           monthRoman,
				"$tahun_sp":               time.Now().Format("2006"),
				"$nama_spl":               namaSPL,
				"$jabatan_spl":            fmt.Sprintf("Service Point Leader %s", splCity),
				"$pelanggaran_karyawan":   pelanggaranID,
				"$nama_teknisi":           namaSPL,
				"$tanggal_sp_diterbitkan": tglSP1Diterbitkan,
				"$personalia_name":        config.GetConfig().Default.PTHRD[0].Name, // Assuming the 1st HRD is Personalia
				"$sac_name":               SACData.FullName,
				"$sac_ttd":                SACData.TTDPath,
				"$record_technician":      techData.TechnicianName,
			}

			pdfSP, err := GeneratePDFForSPStockOpname(1, placeholdersSP)
			if err != nil {
				logrus.Errorf("failed to generate pdf for sp 1 of %s : %v", techData.TechnicianName, err)
				continue
			}

			spSO := sptechnicianmodel.SPofStockOpname{
				Technician:      techData.TechnicianName,
				Name:            techData.Name,
				Email:           techData.TechEmail,
				NoHP:            techData.TechNoHP,
				SPL:             techData.SPL,
				SAC:             techData.SAC,
				IsGotSP1:        true,
				GotSP1At:        &gotSPDate,
				PelanggaranSP1:  pelanggaranID,
				SP1SoundTTSPath: fileTTS,
				SP1FilePath:     pdfSP,
			}

			err = dbWeb.Model(&sptechnicianmodel.SPofStockOpname{}).Create(&spSO).Error
			if err != nil {
				logrus.Errorf("got error while trying to create sp stock opname for %s : %v", techData.TechnicianName, err)
				continue
			}

			var sbID strings.Builder
			sbID.WriteString(fmt.Sprintf("Dengan ini, kami menyampaikan bahwa saudara %s menerima Surat Peringatan (SP) 1.\n", techData.Name))
			sbID.WriteString(fmt.Sprintf("Sehubungan dengan sikap tidak disiplin/ pelanggaran terhadap tata tertib Perusahaan yang Karyawan lakukan yaitu:\n\n%s\n\n", pelanggaranID))
			sbID.WriteString("Mohon menjadi perhatian dan segera melakukan perbaikan yang diperlukan.\n")
			sbID.WriteString(fmt.Sprintf("Maksimal respon sampai %v\n\n", tglMaxResponSP))
			sbID.WriteString("Terima kasih.")

			if needSendSPToSPL {
				var msgIDSb strings.Builder
				msgIDSb.WriteString(fmt.Sprintf("Halo, %s Pak %s.\n\n", greetingID, namaSPL))
				msgIDSb.WriteString(sbID.String())
				msgWA := msgIDSb.String()
				sendDocumentViaBotForSPStockOpname(techData.TechnicianName, jidStrSPL, msgWA, pdfSP, 1, strings.ReplaceAll(jidStrSPL, "@s.whatsapp.net", ""))
			}

			if jidStrSAC != "" {
				if strings.Contains(strings.ToLower(techData.SAC), "tetty") {
					var msgIDSb strings.Builder
					msgIDSb.WriteString(fmt.Sprintf("Halo, %s Kak %s.\n\n", greetingID, techData.SAC))
					msgIDSb.WriteString(sbID.String())
					msgWA := msgIDSb.String()
					sendDocumentViaBotForSPStockOpname(techData.TechnicianName, jidStrSAC, msgWA, pdfSP, 1, strings.ReplaceAll(jidStrSAC, "@s.whatsapp.net", ""))
				} else {
					var msgIDSb strings.Builder
					msgIDSb.WriteString(fmt.Sprintf("Halo, %s Pak %s.\n\n", greetingID, techData.SAC))
					msgIDSb.WriteString(sbID.String())
					msgWA := msgIDSb.String()
					sendDocumentViaBotForSPStockOpname(techData.TechnicianName, jidStrSAC, msgWA, pdfSP, 1, strings.ReplaceAll(jidStrSAC, "@s.whatsapp.net", ""))
				}
			}

			// ADD: send to HRD to if needed !!
		// Surat Peringatan 2
		case dataSP.IsGotSP1 == true && dataSP.IsGotSP2 == false && dataSP.IsGotSP3 == false:
			err, fileTTS := CreateNotifSoundForSPStockOpname(techData.Name, 2)
			if err != nil {
				logrus.Errorf("failed to generate mp3 tts for sp-2 %s : %v", techData.TechnicianName, err)
				continue
			}

			// Placeholders for SP pdf
			noSurat, err := IncrementNomorSuratSP(dbWeb, "LAST_NOMOR_SURAT_SP2_GENERATED")
			if err != nil {
				logrus.Errorf("Failed to increment nomor surat SP-2: %v", err)
				continue
			}
			var noSuratStr string
			if noSurat < 1000 {
				noSuratStr = fmt.Sprintf("%03d", noSurat)
			} else {
				noSuratStr = fmt.Sprintf("%d", noSurat)
			}
			monthRoman, err := fun.MonthToRoman(int(time.Now().Month()))
			if err != nil {
				logrus.Errorf("Failed to convert month to roman numeral: %v", err)
				continue
			}
			splCity := getSPLCity(techData.SPL)
			if splCity == "" {
				logrus.Warnf("SPL city not found for SPL %s, defaulting to 'Unknown'", techData.SPL)
				splCity = "Unknown"
			}
			tanggalSP2Terbit, err := tanggal.Papar(time.Now(), "Jakarta", tanggal.WIB)
			if err != nil {
				logrus.Errorf("Failed to get formatted date for SP2: %v", err)
				continue
			}
			tglSP2Diterbitkan := tanggalSP2Terbit.Format(" ", []tanggal.Format{
				tanggal.Hari,      // 27
				tanggal.NamaBulan, // Maret
				tanggal.Tahun,     // 2025
			})

			placeholdersSP := map[string]string{
				"$nomor_surat":            noSuratStr,
				"$bulan_romawi":           monthRoman,
				"$tahun_sp":               time.Now().Format("2006"),
				"$nama_spl":               namaSPL,
				"$jabatan_spl":            fmt.Sprintf("Service Point Leader %s", splCity),
				"$pelanggaran_karyawan":   pelanggaranID,
				"$nama_teknisi":           namaSPL,
				"$tanggal_sp_diterbitkan": tglSP2Diterbitkan,
				"$personalia_name":        config.GetConfig().Default.PTHRD[0].Name, // Assuming the 1st HRD is Personalia
				"$sac_name":               SACData.FullName,
				"$sac_ttd":                SACData.TTDPath,
				"$record_technician":      techData.TechnicianName,
			}

			pdfSP, err := GeneratePDFForSPStockOpname(2, placeholdersSP)
			if err != nil {
				logrus.Errorf("failed to generate pdf for sp 2 of %s : %v", techData.TechnicianName, err)
				continue
			}

			dataTechUpdated["is_got_sp2"] = true
			dataTechUpdated["got_sp2_at"] = &gotSPDate
			dataTechUpdated["pelanggaran_sp2"] = pelanggaranID
			dataTechUpdated["sp2_sound_tts_path"] = fileTTS
			dataTechUpdated["sp2_file_path"] = pdfSP

			processedSP = 2
		// Surat Peringatan 3
		case dataSP.IsGotSP1 == true && dataSP.IsGotSP2 == true && dataSP.IsGotSP3 == false:
			err, fileTTS := CreateNotifSoundForSPStockOpname(techData.Name, 3)
			if err != nil {
				logrus.Errorf("failed to generate mp3 tts for sp-3 %s : %v", techData.TechnicianName, err)
				continue
			}

			// Placeholders for SP pdf
			noSurat, err := IncrementNomorSuratSP(dbWeb, "LAST_NOMOR_SURAT_SP3_GENERATED")
			if err != nil {
				logrus.Errorf("Failed to increment nomor surat SP-3: %v", err)
				continue
			}
			var noSuratStr string
			if noSurat < 1000 {
				noSuratStr = fmt.Sprintf("%03d", noSurat)
			} else {
				noSuratStr = fmt.Sprintf("%d", noSurat)
			}
			monthRoman, err := fun.MonthToRoman(int(time.Now().Month()))
			if err != nil {
				logrus.Errorf("Failed to convert month to roman numeral: %v", err)
				continue
			}
			splCity := getSPLCity(techData.SPL)
			if splCity == "" {
				logrus.Warnf("SPL city not found for SPL %s, defaulting to 'Unknown'", techData.SPL)
				splCity = "Unknown"
			}
			tanggalSP3Terbit, err := tanggal.Papar(time.Now(), "Jakarta", tanggal.WIB)
			if err != nil {
				logrus.Errorf("Failed to get formatted date for SP3: %v", err)
				continue
			}
			tglSP3Diterbitkan := tanggalSP3Terbit.Format(" ", []tanggal.Format{
				tanggal.Hari,      // 27
				tanggal.NamaBulan, // Maret
				tanggal.Tahun,     // 3035
			})

			placeholdersSP := map[string]string{
				"$nomor_surat":            noSuratStr,
				"$bulan_romawi":           monthRoman,
				"$tahun_sp":               time.Now().Format("3006"),
				"$nama_spl":               namaSPL,
				"$jabatan_spl":            fmt.Sprintf("Service Point Leader %s", splCity),
				"$pelanggaran_karyawan":   pelanggaranID,
				"$nama_teknisi":           namaSPL,
				"$tanggal_sp_diterbitkan": tglSP3Diterbitkan,
				"$personalia_name":        config.GetConfig().Default.PTHRD[0].Name, // Assuming the 1st HRD is Personalia
				"$sac_name":               SACData.FullName,
				"$sac_ttd":                SACData.TTDPath,
				"$record_technician":      techData.TechnicianName,
			}

			pdfSP, err := GeneratePDFForSPStockOpname(3, placeholdersSP)
			if err != nil {
				logrus.Errorf("failed to generate pdf for sp 3 of %s : %v", techData.TechnicianName, err)
				continue
			}

			dataTechUpdated["is_got_sp3"] = true
			dataTechUpdated["got_sp3_at"] = &gotSPDate
			dataTechUpdated["pelanggaran_sp3"] = pelanggaranID
			dataTechUpdated["sp3_sound_tts_path"] = fileTTS
			dataTechUpdated["sp3_file_path"] = pdfSP

			processedSP = 3
		}

		if len(dataTechUpdated) > 0 && dataSPTechNeedToUpdate {
			switch processedSP {
			case 2:
				var dataSPCheck sptechnicianmodel.SPofStockOpname
				err := dbWeb.Model(&sptechnicianmodel.SPofStockOpname{}).
					Where("technician = ?", techData.TechnicianName).
					First(&dataSPCheck).Error
				if err != nil {
					if errors.Is(err, gorm.ErrRecordNotFound) {
						// Skip
					}
					logrus.Errorf("failed to query data sp of %s : %v", techData.TechnicianName, err)
					continue
				}

				sp1SentAt := *dataSPCheck.GotSP1At
				deadline := time.Date(sp1SentAt.Year(), sp1SentAt.Month(), sp1SentAt.Day(), maxResponseSPAtHour, 0, 0, 0, sp1SentAt.Location())
				var onTimeReplyCount int64
				dbWeb.Model(&sptechnicianmodel.SPStockOpnameWhatsappMessage{}).
					Where("technician_got_sp_id = ?", dataSPCheck.ID).
					Where("number_of_sp = ?", 1).
					Where("whatsapp_reply_text IS NOT NULL AND whatsapp_reply_text != ''").
					Where("whatsapp_replied_at <= ?", deadline).
					Count(&onTimeReplyCount)

				if onTimeReplyCount == 0 {
					// Technician did not respond on time to SP-1 make it send SP - 2
					if err := dbWeb.Model(&sptechnicianmodel.SPofStockOpname{}).
						Where("technician = ?", techData.TechnicianName).
						Where("is_got_sp3 = ?", false).
						Updates(&dataTechUpdated).Error; err != nil {
						logrus.Errorf("cannot update got sp of SO for %s : %v", techData.TechnicianName, err)
						continue
					}

					var dataSPLatest sptechnicianmodel.SPofStockOpname
					err := dbWeb.Model(&sptechnicianmodel.SPofStockOpname{}).
						Where("technician = ?", techData.TechnicianName).
						First(&dataSPLatest).Error
					if err != nil {
						if errors.Is(err, gorm.ErrRecordNotFound) {
							// Skip
						}
						logrus.Errorf("failed to query data sp of %s : %v", techData.TechnicianName, err)
					}

					var sbID strings.Builder
					sbID.WriteString(fmt.Sprintf("Dengan ini, kami menyampaikan bahwa saudara *%s* menerima Surat Peringatan (SP) 2.\n", techData.Name))
					sbID.WriteString(fmt.Sprintf("Sehubungan dengan SP 1 yang sebelumnya disampaikan kepada saudara %s pada %s dan belum ada perbaikan yang memadai, maka perusahaan memutuskan untuk menindaklanjuti melalui SP 2.\n\n", techData.Name, dataSP.GotSP1At.Format("Monday, 02 January 2006 15:04")))
					sbID.WriteString("Mohon menjadi perhatian dan segera melakukan perbaikan yang diperlukan.\n")
					sbID.WriteString(fmt.Sprintf("Maksimal respon sampai %v\n\n", tglMaxResponSP))
					sbID.WriteString("Terima kasih.")

					if needSendSPToSPL {
						var msgIDSb strings.Builder
						msgIDSb.WriteString(fmt.Sprintf("Halo, %s Pak %s.\n\n", greetingID, namaSPL))
						msgIDSb.WriteString(sbID.String())
						msgWA := msgIDSb.String()
						sendDocumentViaBotForSPStockOpname(techData.TechnicianName, jidStrSPL, msgWA, dataSPLatest.SP2FilePath, 2, strings.ReplaceAll(jidStrSPL, "@s.whatsapp.net", ""))
					}

					if jidStrSAC != "" {
						if strings.Contains(strings.ToLower(techData.SAC), "tetty") {
							var msgIDSb strings.Builder
							msgIDSb.WriteString(fmt.Sprintf("Halo, %s Kak %s.\n\n", greetingID, techData.SAC))
							msgIDSb.WriteString(sbID.String())
							msgWA := msgIDSb.String()
							sendDocumentViaBotForSPStockOpname(techData.TechnicianName, jidStrSAC, msgWA, dataSPLatest.SP2FilePath, 2, strings.ReplaceAll(jidStrSAC, "@s.whatsapp.net", ""))
						} else {
							var msgIDSb strings.Builder
							msgIDSb.WriteString(fmt.Sprintf("Halo, %s Pak %s.\n\n", greetingID, techData.SAC))
							msgIDSb.WriteString(sbID.String())
							msgWA := msgIDSb.String()
							sendDocumentViaBotForSPStockOpname(techData.TechnicianName, jidStrSAC, msgWA, dataSPLatest.SP2FilePath, 2, strings.ReplaceAll(jidStrSAC, "@s.whatsapp.net", ""))
						}
					}

					// ADD: send to HRD too if needed
				}
			case 3:
				var dataSPCheck sptechnicianmodel.SPofStockOpname
				err := dbWeb.Model(&sptechnicianmodel.SPofStockOpname{}).
					Where("technician = ?", techData.TechnicianName).
					First(&dataSPCheck).Error
				if err != nil {
					if errors.Is(err, gorm.ErrRecordNotFound) {
						// Skip
					}
					logrus.Errorf("failed to query data sp of %s : %v", techData.TechnicianName, err)
					continue
				}

				sp2SentAt := *dataSPCheck.GotSP2At
				deadline := time.Date(sp2SentAt.Year(), sp2SentAt.Month(), sp2SentAt.Day(), maxResponseSPAtHour, 0, 0, 0, sp2SentAt.Location())
				var onTimeReplyCount int64
				dbWeb.Model(&sptechnicianmodel.SPStockOpnameWhatsappMessage{}).
					Where("technician_got_sp_id = ?", dataSPCheck.ID).
					Where("number_of_sp = ?", 2).
					Where("whatsapp_reply_text IS NOT NULL AND whatsapp_reply_text != ''").
					Where("whatsapp_replied_at <= ?", deadline).
					Count(&onTimeReplyCount)

				if onTimeReplyCount == 0 {
					// Technician did not respond on time to SP-2 make it send SP - 3
					if err := dbWeb.Model(&sptechnicianmodel.SPofStockOpname{}).
						Where("technician = ?", techData.TechnicianName).
						Where("is_got_sp3 = ?", false).
						Updates(&dataTechUpdated).Error; err != nil {
						logrus.Errorf("cannot update got sp of SO for %s : %v", techData.TechnicianName, err)
						continue
					}

					var dataSPLatest sptechnicianmodel.SPofStockOpname
					err := dbWeb.Model(&sptechnicianmodel.SPofStockOpname{}).
						Where("technician = ?", techData.TechnicianName).
						First(&dataSPLatest).Error
					if err != nil {
						if errors.Is(err, gorm.ErrRecordNotFound) {
							// Skip
						}
						logrus.Errorf("failed to query data sp of %s : %v", techData.TechnicianName, err)
					}

					var sbID strings.Builder
					sbID.WriteString(fmt.Sprintf("Dengan ini, kami menyampaikan bahwa saudara *%s* menerima Surat Peringatan (SP) 3.\n", techData.Name))
					sbID.WriteString(fmt.Sprintf("Merujuk pada SP 2 yang sebelumnya disampaikan kepada saudara %s pada %s dan tidak terdapat perbaikan yang memadai,", techData.Name, dataSP.GotSP2At.Format("Monday, 02 January 2006 15:04")))
					sbID.WriteString(" perusahaan memutuskan untuk menerbitkan SP 3 sebagai peringatan terakhir.\n")
					sbID.WriteString("Surat ini juga menyatakan berakhirnya hubungan kerja antara perusahaan dan saudara.\n")
					sbID.WriteString("Keputusan berlaku sejak tanggal ditetapkannya surat ini, sesuai dengan ketentuan perusahaan.\n\n")
					sbID.WriteString("Demikian untuk menjadi perhatian serius.\n")
					sbID.WriteString("Terima kasih.")

					if needSendSPToSPL {
						var msgIDSb strings.Builder
						msgIDSb.WriteString(fmt.Sprintf("Halo, %s Pak %s.\n\n", greetingID, namaSPL))
						msgIDSb.WriteString(sbID.String())
						msgWA := msgIDSb.String()
						sendDocumentViaBotForSPStockOpname(techData.TechnicianName, jidStrSPL, msgWA, dataSP.SP3FilePath, 3, strings.ReplaceAll(jidStrSPL, "@s.whatsapp.net", ""))
					}

					if jidStrSAC != "" {
						if strings.Contains(strings.ToLower(techData.SAC), "tetty") {
							var msgIDSb strings.Builder
							msgIDSb.WriteString(fmt.Sprintf("Halo, %s Kak %s.\n\n", greetingID, techData.SAC))
							msgIDSb.WriteString(sbID.String())
							msgWA := msgIDSb.String()
							sendDocumentViaBotForSPStockOpname(techData.TechnicianName, jidStrSAC, msgWA, dataSP.SP3FilePath, 3, strings.ReplaceAll(jidStrSAC, "@s.whatsapp.net", ""))
						} else {
							var msgIDSb strings.Builder
							msgIDSb.WriteString(fmt.Sprintf("Halo, %s Pak %s.\n\n", greetingID, techData.SAC))
							msgIDSb.WriteString(sbID.String())
							msgWA := msgIDSb.String()
							sendDocumentViaBotForSPStockOpname(techData.TechnicianName, jidStrSAC, msgWA, dataSP.SP3FilePath, 3, strings.ReplaceAll(jidStrSAC, "@s.whatsapp.net", ""))
						}
					}

					// ADD: send to HRD too if needed
				}
			}
		}
	}

	if len(technicianListCannotGetSP) > 0 {
		// TODO: try to send to real need information / report about technician cant be send its SP e.g. SAC or HRD
		// send it through email / wa
	}

	timeStart := timeNow
	defer func() {
		duration := time.Since(timeStart)
		logrus.Infof("Finished task: %s in %s", taskDoing, duration)
	}()

	return nil
}

func CreateNotifSoundForSPStockOpname(techName string, spNumber int) (error, string) {
	audioSPDir, err := fun.FindValidDirectory([]string{
		"web/file/sounding_sp_so",
		"../web/file/sounding_sp_so",
		"../../web/file/sounding_sp_so",
	})
	if err != nil {
		return err, ""
	}
	audioForSPDir := filepath.Join(audioSPDir, time.Now().Format("2006-01-02"))
	if err := os.MkdirAll(audioForSPDir, 0755); err != nil {
		return err, ""
	}

	maxResponseSPAtHour := config.GetConfig().StockOpname.MaxResponseSPStockOpnameAtHour
	now := time.Now()
	t := time.Date(
		now.Year(),
		now.Month(),
		now.Day(),
		maxResponseSPAtHour, 0, 0, 0,
		now.Location(),
	)

	tgl, err := tanggal.Papar(t, "Jakarta", tanggal.WIB)
	if err != nil {
		return err, ""
	}

	tglFormatted := tgl.Format(" ", []tanggal.Format{
		tanggal.PukulDenganDetik,
	})

	speech := htgotts.Speech{Folder: audioForSPDir, Language: voices.Indonesian, Handler: &handlers.Native{}}

	spTextPart1 := "Berikut kami sampaikan bahwa"
	spTextPart2 := fmt.Sprintf(" saudara %s", techName)
	spTextPart3 := fmt.Sprintf(" menerima Surat Peringatan (SP-%d)", spNumber)
	spTextPart4 := " terkait tidak dilakukannya Stock Opname"
	spTextPart5 := ". Mohon menjadi perhatian dan segera melakukan perbaikan yang diperlukan."
	spTextPart6 := fmt.Sprintf("Maksimal respon pada %s", tglFormatted)
	spTextPart7 := " terima kasih ..."

	fileNameForSP := fmt.Sprintf("%s_SP%d-SO", strings.ReplaceAll(techName, "*", "Resigned"), spNumber)

	fileTTS, err := fun.CreateRobustTTS(speech, audioForSPDir, []string{
		spTextPart1,
		spTextPart2,
		spTextPart3,
		spTextPart4,
		spTextPart5,
		spTextPart6,
		spTextPart7,
	}, fileNameForSP)
	if err != nil {
		return err, ""
	}

	if fileTTS != "" {
		fileInfo, statErr := os.Stat(fileTTS)
		if statErr == nil {
			logrus.Debugf("🔊 SP%d merged TTS file created: %s, Size: %d bytes", spNumber, fileTTS, fileInfo.Size())
		} else {
			return statErr, ""
		}
	}

	return nil, fileTTS
}

func GeneratePDFForSPStockOpname(spNumber int, placeholders map[string]string) (string, error) {
	imgDir, err := fun.FindValidDirectory([]string{
		"web/assets/self/img",
		"../web/assets/self/img",
		"../../web/assets/self/img",
	})
	if err != nil {
		return "", err
	}

	imgCSNA := filepath.Join(imgDir, "csna.png")
	imgTTDHRD := filepath.Join(imgDir, "ttd_daniella.png")
	imgTTDSAC := filepath.Join(imgDir, placeholders["$sac_ttd"])

	fontMainDir, err := fun.FindValidDirectory([]string{
		"web/assets/font",
		"../web/assets/font",
		"../../web/assets/font",
	})
	if err != nil {
		return "", err
	}

	pdfDir, err := fun.FindValidDirectory([]string{
		"web/file/sp_so",
		"../web/file/sp_so",
		"../../web/file/sp_so",
	})
	if err != nil {
		return "", err
	}

	pdfFileDir := filepath.Join(pdfDir, time.Now().Format("2006-01-02"))
	if err := os.MkdirAll(pdfFileDir, 0755); err != nil {
		return "", err
	}

	pdfFileName := fmt.Sprintf("SP_%d_%s_%s.pdf", spNumber, strings.ReplaceAll(placeholders["$record_technician"], "*", "Resigned"), time.Now().Format("2006-01-02"))
	pdfFilePath := filepath.Join(pdfFileDir, pdfFileName)

	pdf := fpdf.New("P", "mm", "A4", fontMainDir)
	pdf.SetTitle(fmt.Sprintf("Surat Peringatan %d", spNumber), true)
	pdf.SetAuthor(fmt.Sprintf("HRD %s", config.GetConfig().Default.PT), true)
	pdf.SetCreator(fmt.Sprintf("Service Report %s", config.GetConfig().Default.PT), true)
	pdf.SetKeywords(fmt.Sprintf("SP%d, surat peringatan, teknisi, login", spNumber), true)
	pdf.SetSubject(fmt.Sprintf("Surat Peringatan %d - Pemberitahuan untuk Teknisi", spNumber), true)
	pdf.SetCreationDate(time.Now())
	pdf.SetLang("id-ID")

	pdf.AddPage()

	// Add fonts
	pdf.AddFont("Arial", "", "arial.json")
	pdf.AddFont("CenturyGothic", "", "CenturyGothic.json")       // Regular
	pdf.AddFont("CenturyGothic", "B", "CenturyGothic-Bold.json") // Bold

	// Draw border
	pdf.SetLineWidth(0.5)
	pdf.Rect(10, 10, 190, 277, "")

	// ====================== Start of Header Layout ======================
	leftX := 20.0
	// reservedImageWidth := 20.0
	lineHeight1 := 1.0 // first line
	lineHeightOther := 7.0
	numInfoLines := 4
	infoHeight := lineHeight1 + lineHeightOther*float64(numInfoLines-1)

	// Top Y for header
	y := pdf.GetY() + 1

	// Draw logo (height = total info block height)
	logoHeight := infoHeight * 0.8 // 40% bigger than text block height
	pdf.ImageOptions(imgCSNA, leftX, y+3, 0, logoHeight, false, fpdf.ImageOptions{ImageType: "PNG"}, 0, "")

	// Prepare info lines
	infoLines := []struct {
		Text     string
		FontSize float64
		Bold     bool
	}{
		{config.GetConfig().Default.PT, 10.5, true}, // bold
		{"Rukan Crown Blok J No. 008, Green Lake City", 7, false},
		{"Kel. Petir Kec. Cipondoh, Tangerang, Banten - Indonesia 15146", 7, false},
		{"Tel.: (021) 22521101 / 5504722 / 5504723", 7, false},
	}

	// Calculate total height of all info lines (tighter spacing)
	totalInfoHeight := 0.0
	for _, l := range infoLines {
		totalInfoHeight += l.FontSize * 0.6 // reduced spacing
	}

	// Starting Y for first info line to center block vertically relative to logo
	textStartY := y + (infoHeight-totalInfoHeight)/2

	pdf.SetY(textStartY)
	pageW, _ := pdf.GetPageSize()

	for _, l := range infoLines {
		style := ""
		if l.Bold {
			style = "B"
		}
		pdf.SetFont("CenturyGothic", style, l.FontSize)

		// Calculate text width
		strW := pdf.GetStringWidth(l.Text)
		textX := (pageW - strW) / 2 // center on full page width

		pdf.SetXY(textX, pdf.GetY())
		pdf.CellFormat(strW, l.FontSize, l.Text, "", 1, "C", false, 0, "")

		// Move Y down with tighter spacing
		pdf.SetY(pdf.GetY() - l.FontSize + (l.FontSize * 0.6))
	}
	// ====================== End of Header Layout ======================

	// Form code box (top right)
	pdf.SetXY(170, 1.7)
	pdf.SetFont("Arial", "", 7)
	pdf.CellFormat(30, 8, "FM-HRD.07.00.01", "1", 0, "C", false, 0, "")

	// Horizontal line
	pdf.SetDrawColor(0, 0, 0)
	pdf.SetLineWidth(0.5)
	pdf.Line(15, 35, 195, 35)

	// Underline using line drawing
	pdf.SetY(55)
	pdf.SetFont("Arial", "B", 12) // Use same font to measure text width

	// Title
	var titleText string
	switch spNumber {
	case 1:
		titleText = "SURAT PERINGATAN PERTAMA (SP-1)"
	case 2:
		titleText = "SURAT PERINGATAN KEDUA (SP-2)"
	case 3:
		titleText = "SURAT PERINGATAN KETIGA (SP-3)"
	}

	titleWidth := pdf.GetStringWidth(titleText)

	// Calculate center position
	pageWidth, _ := pdf.GetPageSize()
	lineStartX := (pageWidth - titleWidth) / 2
	lineEndX := lineStartX + titleWidth

	// Write the title text first
	currentY := 40.0
	pdf.SetXY(0, currentY)
	pdf.CellFormat(210, 8, titleText, "", 1, "C", false, 0, "")

	// Draw underline
	currentY += 6 // Move down 6mm from current Y position
	pdf.SetLineWidth(0.5)
	pdf.SetDrawColor(0, 0, 0)                          // black
	pdf.Line(lineStartX, currentY, lineEndX, currentY) // 2mm below text baseline

	// Nomor
	currentY += 1 // Move down another 1mm
	pdf.SetFont("Arial", "B", 12)
	pdf.SetXY(0, currentY)
	var sbNomor strings.Builder
	sbNomor.WriteString("Nomor : " + placeholders["$nomor_surat"])
	switch spNumber {
	case 1:
		sbNomor.WriteString("/SP.I-CSNA/")
	case 2:
		sbNomor.WriteString("/SP.II-CSNA/")
	case 3:
		sbNomor.WriteString("/SP.III-CSNA/")
	}
	sbNomor.WriteString(placeholders["$bulan_romawi"] + "/" + placeholders["$tahun_sp"])
	nomor := sbNomor.String()

	pdf.CellFormat(210, 8, nomor, "", 1, "C", false, 0, "")

	// Body
	currentY += 14 // Move down 13mm from current position
	pdf.SetFont("Arial", "", 10)
	pdf.SetXY(15, currentY)
	pdf.CellFormat(0, 8, "Dibuat oleh Perusahaan, dalam hal ini ditujukan kepada:", "", 1, "L", false, 0, "")

	currentY += 10
	pdf.SetXY(15, currentY)
	pdf.CellFormat(0, 8, "Nama", "", 0, "L", false, 0, "")
	pdf.SetX(45)
	pdf.CellFormat(0, 8, ": "+placeholders["$nama_teknisi"], "", 1, "L", false, 0, "")
	currentY += 5
	pdf.SetXY(15, currentY)
	var jabatan string
	if strings.Contains(strings.ToLower(placeholders["$record_technician"]), "spl") {
		jabatan = "Service Point Leader"
	} else {
		jabatan = "Teknisi"
	}
	pdf.CellFormat(0, 8, "Jabatan", "", 0, "L", false, 0, "")
	pdf.SetX(45)
	pdf.CellFormat(0, 8, ": "+jabatan, "", 1, "L", false, 0, "")

	// SP Body text
	dbWeb := gormdb.Databases.Web
	currentY += 10
	pdf.SetFont("Arial", "", 10)
	pdf.SetXY(15, currentY)

	switch spNumber {
	case 1:
		textToWrite := "Sehubungan dengan sikap tidak disiplin / pelanggaran terhadap tata tertib Perusahaan yang Karyawan lakukan yaitu:"
		pdf.MultiCell(0, 5, textToWrite, "", "J", false)

		currentY = pdf.GetY() + 2

		pdf.SetFont("Arial", "B", 10)

		borderLeft := 10.0  // your rectangle starts here
		borderRight := 10.0 // your rectangle ends here
		indent := 10.0      // extra indent

		pageW, _ := pdf.GetPageSize()
		usableW := pageW - borderLeft - borderRight - indent

		pelanggaran := placeholders["$pelanggaran_karyawan"]

		// Set X inside the border + indent
		pdf.SetXY(borderLeft+indent, currentY)

		// MultiCell WRAPS and stays inside box
		pdf.MultiCell(usableW, 5, pelanggaran, "", "J", false)

		// Update cursor
		currentY = pdf.GetY()

		currentY += 3
		pdf.SetFont("Arial", "", 10)
		pdf.SetXY(15, currentY)
		textToWrite = "Atas perbuatan pelanggaran Peraturan Perusahaan yang dilakukan oleh $nama_teknisi, maka dengan ini Perusahaan memberikan Surat Peringatan Pertama (SP-1) kepada Karyawan, agar Karyawan dapat melakukan introspeksi dan memperbaiki diri sehingga Karyawan tidak lagi melakukan pelanggaran atas Peraturan Perusahaan dalam bentuk apapun. SP-1 ini diberikan kepada Karyawan dengan ketentuan sebagai berikut :"
		textToWrite = strings.ReplaceAll(textToWrite, "$nama_teknisi", placeholders["$nama_teknisi"])
		pdf.MultiCell(0, 5, textToWrite, "", "J", false)

		// Numbered list, indented and wrapped
		currentY = pdf.GetY() + 2
		pdf.SetFont("Arial", "", 10)
		listItems := []string{
			"Surat Peringatan Pertama berlaku untuk 1 (satu) hari kedepan sejak diterbitkan.",
			"Apabila dalam kurun waktu 1 (satu) hari kedepan sejak tanggal diterbitkan Surat Peringatan Pertama Karyawan tidak melakukan tindak pelanggaran yang menjadi dasar atas diterbitkannya surat peringatan pertama ini, maka Surat Peringatan Pertama Karyawan dinyatakan sudah tidak berlaku.",
			"Jika dalam kurun waktu 1 (satu) hari kedepan sejak Surat Peringatan Pertama diterbitkan Karyawan didapati kembali melakukan tindakan pelanggaran, maka perusahaan akan memberikan surat peringatan ke-2 untuk Karyawan.",
		}
		indentList := 10.0
		maxLen := 98 // match pelanggaran wrapping
		for i, item := range listItems {
			// Word wrap by splitting into words
			words := strings.Fields(item)
			var lines []string
			line := ""
			for _, word := range words {
				if len(line)+len(word)+1 > maxLen && line != "" {
					lines = append(lines, line)
					line = word
				} else {
					if line != "" {
						line += " "
					}
					line += word
				}
			}
			if line != "" {
				lines = append(lines, line)
			}
			// Print first line with number and indent
			pdf.SetXY(15+indentList, currentY)
			pdf.CellFormat(0, 5, fmt.Sprintf("%d. %s", i+1, lines[0]), "", 1, "L", false, 0, "")
			currentY = pdf.GetY()
			// Print remaining lines, aligned with text (number skipped, same margin)
			for _, line := range lines[1:] {
				pdf.SetXY(18+indentList, currentY)
				pdf.CellFormat(0, 5, line, "", 1, "L", false, 0, "")
				currentY = pdf.GetY()
			}
		}

		// Final paragraph: 'Demikian Surat Peringatan ...' all on one line, correct bold
		currentY += 2
		pdf.SetXY(15, currentY)
		pdf.SetFont("Arial", "", 10)
		pdf.CellFormat(pdf.GetStringWidth("Demikian "), 5, "Demikian ", "", 0, "L", false, 0, "")
		pdf.SetFont("Arial", "B", 10)
		pdf.CellFormat(pdf.GetStringWidth("Surat Peringatan"), 5, "Surat Peringatan", "", 0, "L", false, 0, "")
		pdf.SetFont("Arial", "", 10)
		pdf.CellFormat(0, 5, " ini dibuat agar dapat diperhatikan dan ditaati sebaik mungkin oleh yang bersangkutan.", "", 1, "L", false, 0, "")
	case 2:
		var dataSP sptechnicianmodel.SPofStockOpname
		err := dbWeb.Where("technician = ?", placeholders["$record_technician"]).
			Model(&sptechnicianmodel.SPofStockOpname{}).First(&dataSP).Error
		if err != nil {
			return "", fmt.Errorf("failed to fetch data sp 2 of %s : %v", placeholders["$record_technician"], err)
		}

		textToWrite := "Sehubungan dengan Surat Peringatan Pertama (SP-1) yang sebelumnya disampaikan kepada Sdr. $nama_teknisi, perusahaan kemudian memutuskan untuk menindaklanjuti melalui Surat Peringatan Kedua (SP-2). Hal ini didasari Sdr. $nama_teknisi yang tidak menunjukkan sikap disiplin/pelanggaran terhadap Tata Tertib Perusahaan yang Sdr. $nama_teknisi lakukan yaitu:"
		textToWrite = strings.ReplaceAll(textToWrite, "$nama_teknisi", placeholders["$nama_teknisi"])
		pdf.MultiCell(0, 5, textToWrite, "", "J", false)

		// List Pelanggaran
		currentY = pdf.GetY() + 2
		pdf.SetFont("Arial", "B", 10)
		listPelanggaran := []string{}
		if dataSP.PelanggaranSP1 != "" {
			listPelanggaran = append(listPelanggaran, dataSP.PelanggaranSP1)
		}
		if placeholders["$pelanggaran_karyawan"] != "" {
			listPelanggaran = append(listPelanggaran, placeholders["$pelanggaran_karyawan"])
		}

		borderLeft := 10.0
		borderRight := 10.0
		indent := 10.0
		pageW, _ := pdf.GetPageSize()
		usableW := pageW - borderLeft - borderRight - indent

		indentPelanggaran := indent

		for i, item := range listPelanggaran {
			if item == "" {
				continue
			}
			pdf.SetXY(borderLeft+indentPelanggaran, currentY)
			pdf.MultiCell(usableW, 5, fmt.Sprintf("%d. %s", i+1, item), "", "L", false)
			currentY = pdf.GetY()
		}

		currentY += 3
		pdf.SetFont("Arial", "", 10)
		pdf.SetXY(15, currentY)
		textToWrite = "Atas perbuatan pelanggaran Tata Tertib Perusahaan yang dilakukan oleh Sdr. $nama_teknisi, maka dengan ini Perusahaan memberikan Surat Peringatan Kedua (SP-2) kepada Sdr. $nama_teknisi agar Karyawan dapat melakukan introspeksi dan memperbaiki diri sehingga Karyawan tidak lagi melakukan pelanggaran atas Tata Tertib Perusahaan dalam bentuk apapun. SP-2 ini diberikan kepada Karyawan dengan ketentuan sebagai berikut :"
		textToWrite = strings.ReplaceAll(textToWrite, "$nama_teknisi", placeholders["$nama_teknisi"])
		pdf.MultiCell(0, 5, textToWrite, "", "J", false)

		// Numbered list, indented and wrapped
		currentY = pdf.GetY() + 2
		pdf.SetFont("Arial", "", 10)
		listItems := []string{
			"Surat Peringatan Kedua berlaku untuk 1 (satu) hari kedepan sejak diterbitkan.",
			"Apabila dalam kurun waktu 1 (satu) hari kedepan sejak tanggal diterbitkan Surat Peringatan Kedua Karyawan tidak melakukan tindak pelanggaran yang menjadi dasar atas diterbitkannya surat Peringatan Kedua ini, maka Surat Peringatan Kedua Karyawan dinyatakan sudah tidak berlaku.",
			"Jika dalam kurun waktu 1 (satu) hari kedepan sejak Surat Peringatan Kedua diterbitkan Karyawan didapati kembali melakukan tindakan pelanggaran, maka perusahaan akan memberikan Surat Peringatan Ketiga (SP-3) atau Pemutusan Hubungan Kerja.",
		}
		indentList := indent
		for i, item := range listItems {
			pdf.SetXY(borderLeft+indentList, currentY)
			pdf.MultiCell(usableW, 5, fmt.Sprintf("%d. %s", i+1, item), "", "L", false)
			currentY = pdf.GetY()
		}

		// Final paragraph: 'Demikian Surat Peringatan ...' all on one line, correct bold
		currentY += 2
		pdf.SetXY(15, currentY)
		pdf.SetFont("Arial", "", 10)
		pdf.CellFormat(pdf.GetStringWidth("Demikian "), 5, "Demikian ", "", 0, "L", false, 0, "")
		pdf.SetFont("Arial", "B", 10)
		pdf.CellFormat(pdf.GetStringWidth("Surat Peringatan"), 5, "Surat Peringatan", "", 0, "L", false, 0, "")
		pdf.SetFont("Arial", "", 10)
		pdf.CellFormat(0, 5, " ini dibuat agar dapat diperhatikan dan ditaati sebaik mungkin oleh yang bersangkutan.", "", 1, "L", false, 0, "")
	case 3:
		var dataSP sptechnicianmodel.SPofStockOpname
		err := dbWeb.Where("technician = ?", placeholders["$record_technician"]).
			Model(&sptechnicianmodel.SPofStockOpname{}).First(&dataSP).Error
		if err != nil {
			return "", fmt.Errorf("failed to fetch data sp 3 of %s : %v", placeholders["$record_technician"], err)
		}

		textToWrite := "Sehubungan dengan Surat Peringatan Pertama (SP-1) dan Surat Peringatan Kedua (SP-2) yang telah sebelumnya diberikan kepada Sdr. $nama_teknisi, namun yang bersangkutan tetap tidak menunjukkan perubahan sikap dan perbaikan diri, serta masih melakukan pelanggaran terhadap Tata Tertib Perusahaan, maka dengan ini Perusahaan menyampaikan Surat Peringatan Ketiga (SP-3). Adapun pelanggaran yang dilakukan oleh Sdr. $nama_teknisi adalah sebagai berikut :"
		textToWrite = strings.ReplaceAll(textToWrite, "$nama_teknisi", placeholders["$nama_teknisi"])
		pdf.MultiCell(0, 5, textToWrite, "", "J", false)

		// List Pelanggaran
		currentY = pdf.GetY() + 2
		pdf.SetFont("Arial", "B", 10)
		listPelanggaran := []string{}
		if dataSP.PelanggaranSP1 != "" {
			listPelanggaran = append(listPelanggaran, dataSP.PelanggaranSP1)
		}
		if dataSP.PelanggaranSP2 != "" {
			listPelanggaran = append(listPelanggaran, dataSP.PelanggaranSP2)
		}
		if placeholders["$pelanggaran_karyawan"] != "" {
			listPelanggaran = append(listPelanggaran, placeholders["$pelanggaran_karyawan"])
		}

		borderLeft := 10.0
		borderRight := 10.0
		indent := 10.0
		pageW, _ := pdf.GetPageSize()
		usableW := pageW - borderLeft - borderRight - indent

		indentPelanggaran := indent

		for i, item := range listPelanggaran {
			if item == "" {
				continue
			}
			pdf.SetXY(borderLeft+indentPelanggaran, currentY)
			pdf.MultiCell(usableW, 5, fmt.Sprintf("%d. %s", i+1, item), "", "L", false)
			currentY = pdf.GetY()
		}

		currentY += 3
		pdf.SetFont("Arial", "", 10)
		pdf.SetXY(15, currentY)
		textToWrite = "Surat Peringatan Ketiga (SP-3) ini merupakan peringatan terakhir yang sekaligus menjadi dasar pemutusan hubungan kerja (PHK) terhadap Sdr. $nama_teknisi karena telah berulang kali melakukan pelanggaran yang merugikan kedisiplinan dan tata tertib kerja di lingkungan Perusahaan."
		textToWrite = strings.ReplaceAll(textToWrite, "$nama_teknisi", placeholders["$nama_teknisi"])
		pdf.MultiCell(0, 5, textToWrite, "", "J", false)

		currentY += 20
		pdf.SetFont("Arial", "", 10)
		pdf.SetXY(15, currentY)
		textToWrite = "Dengan ini Perusahaan menyatakan bahwa hubungan kerja dengan Sdr. $nama_teknisi dinyatakan berakhir sejak tanggal $tanggal_sp_diterbitkan."
		textToWrite = strings.ReplaceAll(textToWrite, "$nama_teknisi", placeholders["$nama_teknisi"])
		textToWrite = strings.ReplaceAll(textToWrite, "$tanggal_sp_diterbitkan", placeholders["$tanggal_sp_diterbitkan"])
		pdf.MultiCell(0, 5, textToWrite, "", "J", false)

		currentY += 10
		pdf.SetFont("Arial", "", 10)
		pdf.SetXY(15, currentY)
		textToWrite = "Keputusan ini diambil dengan pertimbangan:"
		pdf.MultiCell(0, 5, textToWrite, "", "J", false)

		// Numbered list, indented and wrapped
		currentY = pdf.GetY() + 3
		pdf.SetFont("Arial", "", 10)
		listItems := []string{
			fmt.Sprintf("Sdr. %s telah menerima SP-1 dan SP-2, namun tidak menunjukkan adanya perbaikan.", placeholders["$nama_teknisi"]),
			"Pelanggaran yang dilakukan telah mengganggu kelancaran operasional dan kedisiplinan Perusahaan.",
			"Sesuai dengan Peraturan Perusahaan dan ketentuan ketenagakerjaan yang berlaku, maka SP-3 ini berlaku sekaligus sebagai surat pemutusan hubungan kerja.",
		}
		indentList := indent
		for i, item := range listItems {
			pdf.SetXY(borderLeft+indentList, currentY)
			pdf.MultiCell(usableW, 5, fmt.Sprintf("%d. %s", i+1, item), "", "L", false)
			currentY = pdf.GetY()
		}

		// Final paragraph with inline bold (Demikian + Surat Peringatan + rest)
		currentY += 2
		marginLeft := 15.0
		pdf.SetXY(marginLeft, currentY)

		// Define parts
		part1 := "Demikian "
		part2 := "Surat Peringatan"
		part3 := " ini dibuat untuk dapat dipahami dan dijadikan dasar pelaksanaan tindak lanjut oleh Perusahaan."

		// Build the full sentence (for wrapping width calculation)
		fullSentence := part1 + part2 + part3

		// Define maximum width (page width minus margins)
		marginRight := 15.0
		maxWidth := pageWidth - marginLeft - marginRight

		// Split into words for wrapping
		words := strings.Split(fullSentence, " ")
		line := ""
		for _, w := range words {
			testLine := strings.TrimSpace(line + " " + w)
			width := pdf.GetStringWidth(testLine)

			if width > maxWidth {
				// Print current line
				pdf.SetX(marginLeft) // <-- ensure every new line starts at same left margin
				pdf.SetFont("Arial", "", 10)

				if strings.Contains(line, part2) {
					before := strings.Split(line, part2)[0]
					after := strings.Split(line, part2)[1]

					// Print before
					pdf.CellFormat(pdf.GetStringWidth(before), 5, before, "", 0, "", false, 0, "")
					// Print bold
					pdf.SetFont("Arial", "B", 10)
					pdf.CellFormat(pdf.GetStringWidth(part2), 5, part2, "", 0, "", false, 0, "")
					// Back to normal
					pdf.SetFont("Arial", "", 10)
					pdf.CellFormat(pdf.GetStringWidth(after), 5, after, "", 0, "", false, 0, "")
				} else {
					pdf.CellFormat(pdf.GetStringWidth(line), 5, line, "", 0, "", false, 0, "")
				}

				pdf.Ln(5)
				line = w
			} else {
				line = testLine
			}
		}

		// Print the last line
		if line != "" {
			pdf.SetX(marginLeft) // <-- align last line too
			pdf.SetFont("Arial", "", 10)

			if strings.Contains(line, part2) {
				before := strings.Split(line, part2)[0]
				after := strings.Split(line, part2)[1]

				pdf.CellFormat(pdf.GetStringWidth(before), 5, before, "", 0, "", false, 0, "")
				pdf.SetFont("Arial", "B", 10)
				pdf.CellFormat(pdf.GetStringWidth(part2), 5, part2, "", 0, "", false, 0, "")
				pdf.SetFont("Arial", "", 10)
				pdf.CellFormat(pdf.GetStringWidth(after), 5, after, "", 0, "", false, 0, "")
			} else {
				pdf.CellFormat(pdf.GetStringWidth(line), 5, line, "", 0, "", false, 0, "")
			}
		}
	}

	// =================================== Signatures ===================================
	currentY += 10
	leftX = 35.0
	rightX := 155.0 // adjust for your page width

	pdf.SetFont("Arial", "", 10)
	pdf.SetXY(15, currentY)
	pdf.CellFormat(80, 5, fmt.Sprintf("Tangerang, %v", placeholders["$tanggal_sp_diterbitkan"]), "", 0, "L", false, 0, "")

	currentY += 8
	pdf.SetXY(leftX, currentY)
	pdf.CellFormat(80, 5, "Diterbitkan,", "", 0, "L", false, 0, "")
	pdf.SetXY(rightX, currentY)
	pdf.CellFormat(80, 5, "Mengetahui,", "", 0, "L", false, 0, "")

	currentY += 20 // space for signatures

	// --- Left signature: Personalia ---
	ttdHRDWidth := 20.0
	pdf.ImageOptions(imgTTDHRD, leftX, currentY-15, ttdHRDWidth, 0, false, fpdf.ImageOptions{ImageType: "PNG"}, 0, "")

	// Center Personalia name below "Diterbitkan,"
	labelDiterbitkan := "Diterbitkan,"
	labelWidth := pdf.GetStringWidth(labelDiterbitkan)
	nameWidth := pdf.GetStringWidth(placeholders["$personalia_name"])
	padding := 4.0

	// Compute X so the name is centered under the label
	centerX := leftX + (labelWidth / 2) - (nameWidth / 2)

	// Personalia Name (bold, underline)
	pdf.SetFont("Arial", "B", 10)
	pdf.SetXY(centerX, currentY)
	pdf.CellFormat(nameWidth, 5, placeholders["$personalia_name"], "", 0, "L", false, 0, "")

	// Underline
	pdf.SetDrawColor(0, 0, 0)
	pdf.SetLineWidth(0.5)
	pdf.Line(centerX, currentY+5, centerX+nameWidth+padding, currentY+5)

	// Role text ("Personalia"), centered under the name
	roleWidth := pdf.GetStringWidth("Personalia")
	roleX := leftX + (labelWidth / 2) - (roleWidth / 2)
	pdf.SetFont("Arial", "", 10)
	pdf.SetXY(roleX, currentY+5)
	pdf.CellFormat(roleWidth, 5, "Personalia", "", 0, "L", false, 0, "")

	// --- Right signature: SAC ---
	ttdSacWidth := 18.0
	rightXForTTD := rightX + 3.0
	switch placeholders["$sac_ttd"] {
	case "ttd_angga.png":
		ttdSacWidth = 30.0 // Budi's signature is wider
		rightXForTTD = rightX - 4.0
	case "ttd_tomi.png":
		rightXForTTD = rightX - 3.0
		ttdSacWidth = 25.0 // Tomi's signature is wider
	case "ttd_burhan.png":
		rightXForTTD = rightX + 5.0
		ttdSacWidth = 11.0 // Burhan's signature
	}

	pdf.ImageOptions(imgTTDSAC, rightXForTTD, currentY-15, ttdSacWidth, 0, false, fpdf.ImageOptions{ImageType: "PNG"}, 0, "")

	// Center SAC name below "Mengetahui,"
	labelMengetahui := "Mengetahui,"
	labelWidthR := pdf.GetStringWidth(labelMengetahui)
	mgrWidth := pdf.GetStringWidth(placeholders["$sac_name"])

	// Compute X so the SAC name is centered under the label
	centerXR := rightX + (labelWidthR / 2) - (mgrWidth / 2)

	pdf.SetFont("Arial", "B", 10)
	pdf.SetXY(centerXR, currentY)
	pdf.CellFormat(mgrWidth, 5, placeholders["$sac_name"], "", 0, "L", false, 0, "")

	// Underline
	pdf.SetDrawColor(0, 0, 0)
	pdf.SetLineWidth(0.5)
	pdf.Line(centerXR, currentY+5, centerXR+mgrWidth+padding, currentY+5)

	// Role text ("Service Area Coordinator"), centered
	roleR := "Service Area Coordinator"
	roleRWidth := pdf.GetStringWidth(roleR)
	roleRX := rightX + (labelWidthR / 2) - (roleRWidth / 2)

	pdf.SetFont("Arial", "", 10)
	pdf.SetXY(roleRX+2, currentY+5)
	pdf.CellFormat(roleRWidth, 5, roleR, "", 0, "L", false, 0, "")
	// ==================================================================================

	if err := pdf.OutputFileAndClose(pdfFilePath); err != nil {
		return "", err
	}

	return pdfFilePath, nil
}

/*
	WhatsApp sp Stock Opname controllers
*/

func sendDocumentViaBotForSPStockOpname(
	technician,
	jidStr,
	idMsg,
	filePath string,
	spNumber int,
	spSentTo string,
) {
	dbWeb := gormdb.Databases.Web

	parsedJID, err := types.ParseJID(jidStr)
	if err != nil {
		logrus.Errorf("failed to parse JID %s : %v", jidStr, err)
		return
	}

	// Remove device part if present
	userJID := types.JID{
		User:   parsedJID.User,
		Server: parsedJID.Server,
	}

	fileData, err := os.ReadFile(filePath)
	if err != nil {
		logrus.Errorf("Failed to read file %s: %v\n", filePath, err)
		return
	}

	fileInfo, err := os.Stat(filePath)
	if err != nil {
		logrus.Errorf("Failed to get file info for %s: %v\n", filePath, err)
		return
	}

	if fileInfo.Size() == 0 {
		logrus.Errorf("File %s is empty, cannot send message", filePath)
		return
	}

	// Detect MIME type from file data
	mimeType := http.DetectContentType(fileData)
	if mimeType == "" {
		// Fallback to extension-based detection
		ext := filepath.Ext(filePath)
		mimeType = mime.TypeByExtension(ext)
		if mimeType == "" {
			mimeType = "application/octet-stream" // Default fallback
		}
	}

	// Get filename from path
	fileName := filepath.Base(filePath)

	uploaded, err := WhatsappClient.Upload(
		context.Background(),
		fileData,
		whatsmeow.MediaDocument)
	if err != nil {
		logrus.Errorf("Failed to upload file %s: %v\n", filePath, err)
		return
	}
	if uploaded.URL == "" || uploaded.DirectPath == "" {
		logrus.Errorf("Upload response is missing URL or DirectPath for file %s", filePath)
		return
	}

	resp, err := WhatsappClient.SendMessage(context.Background(), userJID, &waE2E.Message{
		DocumentMessage: &waE2E.DocumentMessage{
			URL:           proto.String(uploaded.URL),
			DirectPath:    proto.String(uploaded.DirectPath),
			MediaKey:      uploaded.MediaKey,
			Mimetype:      proto.String(mimeType),
			FileEncSHA256: uploaded.FileEncSHA256,
			FileSHA256:    uploaded.FileSHA256,
			FileLength:    proto.Uint64(uint64(len(fileData))),
			FileName:      proto.String(fileName),
			Caption:       proto.String(idMsg), // Your message becomes the caption
		},
	})

	if err != nil {
		logrus.Errorf("Failed to send message to %s: %v\n", userJID.String(), err)
		// Do not return here, as we still want to log the attempt if needed,
		// but the resp object will be nil, so handle that.
	}

	go func() {
		var spSO sptechnicianmodel.SPofStockOpname
		if err := dbWeb.Where("technician = ?", technician).First(&spSO).Error; err != nil {
			logrus.Errorf("failed to find record sp so for technician %s : %v", technician, err)
			return
		}

		var respSentAt *time.Time
		if !resp.Timestamp.IsZero() {
			respSentAt = &resp.Timestamp
		} else {
			t := time.Now()
			respSentAt = &t
		}

		newSPMsg := sptechnicianmodel.SPStockOpnameWhatsappMessage{
			TechnicianGotSPID:     &spSO.ID,
			NumberOfSP:            spNumber,
			WhatsappMessageSentTo: spSentTo,
			WhatsappChatID:        resp.ID,
			WhatsappSentAt:        respSentAt,
			WhatsappChatJID:       userJID.String(),
			WhatsappSenderJID:     resp.Sender.String(),
			WhatsappMessageBody:   idMsg,
			WhatsappMessageType:   "document",
			WhatsappIsGroup:       userJID.Server == "g.us",
			WhatsappMsgStatus:     "sent",
		}

		if err := dbWeb.Create(&newSPMsg).Error; err != nil {
			logrus.Errorf("failed to create sp wa msg for sp so of %s : %v", technician, err)
			return
		}

		logrus.Infof("Successfully saved sp %d whatsapp msg for technician %s to db", spNumber, technician)
	}()
}

func handleRepliesReactionsAndEditedMsgForSPStockOpname() {
	e := WhatsappEventMsgForSPStockOpname
	dbWeb := gormdb.Databases.Web
	dataHRD := config.GetConfig().Default.PTHRD

	cwd, err := os.Getwd()
	if err != nil {
		logrus.Errorf("Failed to get working directory: %v", err)
		return
	}

	baseDir := filepath.Join(cwd, "web", "file")
	if _, err := os.Stat(baseDir); os.IsNotExist(err) {
		logrus.Errorf("Directory does not exist: %s\n", baseDir)
		return
	}

	loc, _ := time.LoadLocation(config.GetConfig().Default.Timezone)
	now := time.Now().In(loc)

	today := now.Format("2006-01-02")
	uploadDir := filepath.Join(baseDir, "wa_reply", today)
	if err := os.MkdirAll(uploadDir, os.ModePerm); err != nil {
		logrus.Errorf("Failed to create upload directory: %v", err)
		return
	}

	// 🔧 Extract context info from any type
	var ctxInfo *waE2E.ContextInfo

	switch {
	case e.Message.ExtendedTextMessage != nil:
		ctxInfo = e.Message.ExtendedTextMessage.GetContextInfo()

	case e.Message.ImageMessage != nil:
		ctxInfo = e.Message.ImageMessage.GetContextInfo()

	case e.Message.VideoMessage != nil:
		ctxInfo = e.Message.VideoMessage.GetContextInfo()

	case e.Message.DocumentMessage != nil:
		ctxInfo = e.Message.DocumentMessage.GetContextInfo()

	case e.Message.AudioMessage != nil:
		ctxInfo = e.Message.AudioMessage.GetContextInfo()

	case e.Message.StickerMessage != nil:
		ctxInfo = e.Message.StickerMessage.GetContextInfo()
	}

	// ✅ Handle replies
	if ctxInfo != nil && ctxInfo.QuotedMessage != nil && ctxInfo.StanzaID != nil && *ctxInfo.StanzaID != "" {
		var replyText string
		waReplyPublicURL := config.GetConfig().Whatsmeow.WAReplyPublicURL + "/" + time.Now().Format("2006-01-02")

		switch {

		case e.Message.Conversation != nil:
			replyText = *e.Message.Conversation

		case e.Message.ExtendedTextMessage != nil && e.Message.ExtendedTextMessage.Text != nil:
			replyText = *e.Message.ExtendedTextMessage.Text

		case e.Message.ImageMessage != nil:
			msg := e.Message.ImageMessage
			data, err := WhatsappClient.Download(context.Background(), msg)
			if err != nil {
				logrus.Error("Failed to download image:", err)
				break
			}
			mimeType := getSafeString(msg.Mimetype)
			ext := getFileExtension(mimeType)
			filename := fmt.Sprintf("img_%d%s", time.Now().UnixNano(), ext)
			savePath := filepath.Join(uploadDir, filename)
			os.WriteFile(savePath, data, 0644)
			publicURL := fmt.Sprintf("%s/%s", waReplyPublicURL, filename)
			caption := getSafeString(msg.Caption)
			replyText = fmt.Sprintf("📷 %s %s", caption, publicURL)

		case e.Message.VideoMessage != nil:
			msg := e.Message.VideoMessage
			data, err := WhatsappClient.Download(context.Background(), msg)
			if err != nil {
				logrus.Info("Failed to download video:", err)
				break
			}
			mimeType := getSafeString(msg.Mimetype)
			ext := getFileExtension(mimeType)
			filename := fmt.Sprintf("vid_%d%s", time.Now().UnixNano(), ext)
			savePath := filepath.Join(uploadDir, filename)
			os.WriteFile(savePath, data, 0644)
			publicURL := fmt.Sprintf("%s/%s", waReplyPublicURL, filename)
			caption := getSafeString(msg.Caption)
			replyText = fmt.Sprintf("🎥 %s %s", caption, publicURL)

		case e.Message.AudioMessage != nil:
			msg := e.Message.AudioMessage
			data, err := WhatsappClient.Download(context.Background(), msg)
			if err != nil {
				logrus.Error("Failed to download audio:", err)
				break
			}
			mimeType := getSafeString(msg.Mimetype)
			ext := getFileExtension(mimeType)
			filename := fmt.Sprintf("aud_%d%s", time.Now().UnixNano(), ext)
			savePath := filepath.Join(uploadDir, filename)
			os.WriteFile(savePath, data, 0644)
			publicURL := fmt.Sprintf("%s/%s", waReplyPublicURL, filename)
			replyText = fmt.Sprintf("🎧 Audio message: %s", publicURL)

		case e.Message.DocumentMessage != nil:
			msg := e.Message.DocumentMessage
			data, err := WhatsappClient.Download(context.Background(), msg)
			if err != nil {
				logrus.Error("Failed to download document:", err)
				break
			}
			mimeType := getSafeString(msg.Mimetype)
			ext := getFileExtension(mimeType)
			filename := fmt.Sprintf("doc_%d%s", time.Now().UnixNano(), ext)
			savePath := filepath.Join(uploadDir, filename)
			os.WriteFile(savePath, data, 0644)
			publicURL := fmt.Sprintf("%s/%s", waReplyPublicURL, filename)
			caption := getSafeString(msg.Caption)
			replyText = fmt.Sprintf("📄 %s %s", caption, publicURL)

		case e.Message.StickerMessage != nil:
			msg := e.Message.StickerMessage
			data, err := WhatsappClient.Download(context.Background(), msg)
			if err != nil {
				logrus.Error("Failed to download sticker:", err)
				break
			}
			mimeType := getSafeString(msg.Mimetype)
			ext := getFileExtension(mimeType)
			filename := fmt.Sprintf("stk_%d%s", time.Now().UnixNano(), ext)
			savePath := filepath.Join(uploadDir, filename)
			os.WriteFile(savePath, data, 0644)
			publicURL := fmt.Sprintf("%s/%s", waReplyPublicURL, filename)
			replyText = fmt.Sprintf("🖼️ Sticker: %s", publicURL)

		default:
			replyText = "(non-text or unknown reply)"
		}

		stanzaID := *ctxInfo.StanzaID
		stanzaID = strings.TrimSpace(stanzaID)

		var spMessage sptechnicianmodel.SPStockOpnameWhatsappMessage
		result := dbWeb.Where("whatsapp_chat_id = ?", stanzaID).First(&spMessage)
		if result.Error != nil {
			if result.Error == gorm.ErrRecordNotFound {
				// No matching record found, skip processing
				return
			}
			logrus.Errorf("Database error while fetching SPStockOpnameWhatsappMessage for whatsapp_chat_id '%s': %v", stanzaID, result.Error)
			return
		}

		// We found the message, now update it
		t := time.Now()
		spMessage.WhatsappRepliedBy = e.Info.Sender.String()
		spMessage.WhatsappRepliedAt = &t
		spMessage.WhatsappReplyText = replyText

		if err := dbWeb.Save(&spMessage).Error; err != nil {
			logrus.Errorf("Failed to update SP reply info for message ID %d: %v", spMessage.ID, err)
			return
		}

		// For the notification, we need details from the parent SPofStockOpname record
		var tech sptechnicianmodel.SPofStockOpname
		if spMessage.TechnicianGotSPID != nil {
			if err := dbWeb.First(&tech, *spMessage.TechnicianGotSPID).Error; err == nil {
				sentAt := "N/A"
				if spMessage.WhatsappSentAt != nil {
					sentAt = spMessage.WhatsappSentAt.Format("02 Jan 2006 15:04")
				}

				var userReplyData model.WAPhoneUser
				var userReplied string
				if err := dbWeb.Model(&model.WAPhoneUser{}).
					Where("phone_number = ?", e.Info.Sender.User).
					First(&userReplyData).Error; err != nil {
					logrus.Errorf("Failed to find WAPhoneUser for phone number %s: %v", e.Info.Sender.User, err)
					userReplied = e.Info.Sender.String()
				} else {
					userReplied = fmt.Sprintf("_%s_ (%s)", userReplyData.FullName, userReplyData.PhoneNumber)
				}

				// Send replied text to Whatsapp HRD
				idText := fmt.Sprintf("SP-%d (Inventory: Stock Opname) untuk %s yang dikirim pada %v mendapatkan respon dari %s, yakni %s",
					spMessage.NumberOfSP,
					tech.Technician,
					sentAt,
					userReplied,
					UnboldedLinkWAMsg(replyText),
				)
				enText := fmt.Sprintf("SP-%d (Inventory: Stock Opname) for %s sent at %v received a reply from %s, which is %s",
					spMessage.NumberOfSP,
					tech.Technician,
					sentAt,
					userReplied,
					UnboldedLinkWAMsg(replyText),
				)

				for _, hrd := range dataHRD {
					jid := fmt.Sprintf("%s@s.whatsapp.net", hrd.PhoneNumber)
					SendLangMessage(jid, idText, enText, "id")
				}
			}
		}
	}

	// 🤖 Handle reactions
	if r := e.Message.GetReactionMessage(); r != nil {
		stanzaID := r.GetKey().GetID()
		stanzaID = strings.TrimSpace(stanzaID)

		var spMessage sptechnicianmodel.SPStockOpnameWhatsappMessage
		if err := dbWeb.Where("whatsapp_chat_id = ?", stanzaID).First(&spMessage).Error; err != nil {
			logrus.Warnf("No SPStockOpnameWhatsappMessage matched for reaction on whatsapp_chat_id '%s': %v", stanzaID, err)
			return
		}

		t := time.Now()
		spMessage.WhatsappReactionEmoji = r.GetText()
		spMessage.WhatsappRepliedBy = e.Info.Sender.String() // Using RepliedBy for reactions as well
		spMessage.WhatsappReactedAt = &t

		if err := dbWeb.Save(&spMessage).Error; err != nil {
			logrus.Errorf("Failed to update SP reaction info for message ID %d: %v", spMessage.ID, err)
			return
		}
	}

	// ✏️ Edited Message
	if pm := e.Message.GetProtocolMessage(); pm != nil {
		if pm.GetType() == waE2E.ProtocolMessage_MESSAGE_EDIT {
			var replyText string
			var messageIDToUpdate string
			edited := pm.GetEditedMessage()

			// Case: edited reply (ExtendedTextMessage with context)
			if etm := edited.GetExtendedTextMessage(); etm != nil {
				replyText = etm.GetText()
				if ctx := etm.GetContextInfo(); ctx != nil && ctx.GetStanzaID() != "" {
					messageIDToUpdate = ctx.GetStanzaID()
				}
			}

			// Case: plain edited message (no reply, just a text change)
			if replyText == "" && edited.GetConversation() != "" {
				replyText = edited.GetConversation()
				if pm.GetKey() != nil && pm.GetKey().GetID() != "" {
					messageIDToUpdate = pm.GetKey().GetID()
				}
			}

			if messageIDToUpdate == "" {
				logrus.Println("❌ No message ID to update - skipping edit processing")
				return
			}

			var spMessage sptechnicianmodel.SPStockOpnameWhatsappMessage
			if err := dbWeb.Where("whatsapp_chat_id = ?", messageIDToUpdate).First(&spMessage).Error; err != nil {
				logrus.Warnf("No SPStockOpnameWhatsappMessage matched for edited message on whatsapp_chat_id '%s' : %v", messageIDToUpdate, err)
				return
			}

			t := time.Now()
			spMessage.WhatsappReplyText = replyText
			spMessage.WhatsappRepliedBy = e.Info.Sender.String()
			spMessage.WhatsappRepliedAt = &t // Using RepliedAt for edits

			if err := dbWeb.Save(&spMessage).Error; err != nil {
				logrus.Errorf("Failed to update SP edited message reply for message ID %d: %v", spMessage.ID, err)
				return
			}

			// For the notification, we need details from the parent SPofStockOpname record
			var tech sptechnicianmodel.SPofStockOpname
			if spMessage.TechnicianGotSPID != nil {
				if err := dbWeb.First(&tech, *spMessage.TechnicianGotSPID).Error; err == nil {
					sentAt := "N/A"
					if spMessage.WhatsappSentAt != nil {
						sentAt = spMessage.WhatsappSentAt.Format("02 Jan 2006 15:04")
					}

					var userReplyData model.WAPhoneUser
					var userReplied string
					if err := dbWeb.Model(&model.WAPhoneUser{}).
						Where("phone_number = ?", e.Info.Sender.User).
						First(&userReplyData).Error; err != nil {
						logrus.Errorf("Failed to find WAPhoneUser for phone number %s: %v", e.Info.Sender.User, err)
						userReplied = e.Info.Sender.String()
					} else {
						userReplied = fmt.Sprintf("_%s_ (%s)", userReplyData.FullName, userReplyData.PhoneNumber)
					}

					// Send replied text to Whatsapp HRD
					idText := fmt.Sprintf("SP-%d (Inventory: Stock Opname) untuk %s yang dikirim pada %v mendapatkan respon (edit) dari %s, yakni %s",
						spMessage.NumberOfSP,
						tech.Technician,
						sentAt,
						userReplied,
						UnboldedLinkWAMsg(replyText),
					)
					enText := fmt.Sprintf("SP-%d (Inventory: Stock Opname) for %s sent at %v received a reply (edit) from %s, which is %s",
						spMessage.NumberOfSP,
						tech.Technician,
						sentAt,
						userReplied,
						UnboldedLinkWAMsg(replyText),
					)

					for _, hrd := range dataHRD {
						jid := fmt.Sprintf("%s@s.whatsapp.net", hrd.PhoneNumber)
						userLang, err := GetUserLang(jid)
						if err != nil {
							userLang = "id"
						}
						SendLangMessage(jid, idText, enText, userLang)
					}
				}
			}
		}
	}
}

func handleReceiptForSPStockOpname() {
	e := WhatsappEventsReceiptForSPStockOpname
	dbWeb := gormdb.Databases.Web

	for _, msgID := range e.MessageIDs {
		if string(e.Type) != "" {
			var spMessage sptechnicianmodel.SPStockOpnameWhatsappMessage
			if err := dbWeb.Where("whatsapp_chat_id = ?", msgID).First(&spMessage).Error; err != nil {
				// Skip logging to reduce noise for messages not related to SP
				continue
			}

			// Update the status and save
			spMessage.WhatsappMsgStatus = string(e.Type)
			if err := dbWeb.Save(&spMessage).Error; err != nil {
				logrus.Errorf("Failed to update receipt status for message ID %d: %v", spMessage.ID, err)
			}
		}
	}
}

/*
Web Controllers
*/
func DeleteSuratPeringatanSO() gin.HandlerFunc {
	return func(c *gin.Context) {
		// Get the ID from the URL parameter and convert to integer
		idParam := c.Param("id")
		id, err := strconv.Atoi(idParam)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid Data"})
			return
		}

		// Find the record by ID
		var dbData sptechnicianmodel.SPofStockOpname
		if err := dbWeb.First(&dbData, id).Error; err != nil {
			if err == gorm.ErrRecordNotFound {
				// If the record does not exist, return a 404 error
				c.JSON(http.StatusNotFound, gin.H{"error": "Data not found"})
			} else {
				// Handle other potential errors from the database
				c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to find Data, details: " + err.Error()})
			}
			return
		}

		cookies := c.Request.Cookies()

		// Parse JWT token from cookie
		tokenString, err := c.Cookie("token")
		if err != nil {
			fun.ClearCookiesAndRedirect(c, cookies)
			return
		}
		tokenString = strings.ReplaceAll(tokenString, " ", "+")

		decrypted, err := fun.GetAESDecrypted(tokenString)
		if err != nil {
			logrus.Errorln("failed during decryption", err)
			fun.ClearCookiesAndRedirect(c, cookies)
			return
		}
		var claims map[string]interface{}
		err = json.Unmarshal(decrypted, &claims)
		if err != nil {
			logrus.Errorf("failed converting JSON to map: %v", err)
			fun.ClearCookiesAndRedirect(c, cookies)
			return
		}
		jsonString := ""
		jsonData, err := json.Marshal(dbData)
		if err != nil {
			logrus.Errorf("failed converting to JSON: %v", err)
		} else {
			jsonString = string(jsonData)
		}

		if dbData.IsGotSP3 {
			c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("Data %s - %s has received SP3, cannot be deleted", dbData.Technician, dbData.Name)})
			return
		}

		allowedUsersToDelete := []string{
			"developer",
			"admin",
			"human resource",
			"csna",
		}

		userID := uint(claims["id"].(float64))
		userFullName := claims["fullname"].(string)
		userFullNameLower := strings.ToLower(userFullName)
		isAllowed := false
		for _, allowedUser := range allowedUsersToDelete {
			if strings.Contains(userFullNameLower, allowedUser) {
				isAllowed = true
				break
			}
		}

		if !isAllowed {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "You are not authorized to delete this data"})
			return
		}

		// Use a transaction to ensure atomicity
		tx := dbWeb.Begin()
		if tx.Error != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to start transaction, details: " + tx.Error.Error()})
			return
		}

		// 1. Delete associated SPStockOpnameWhatsappMessage records
		if err := tx.Where("technician_got_sp_id = ?", dbData.ID).
			Delete(&sptechnicianmodel.SPStockOpnameWhatsappMessage{}).Error; err != nil {
			tx.Rollback()
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to delete associated WhatsApp messages, details: " + err.Error()})
			return
		}

		// 2. Perform the deletion of the main record
		if err := tx.Delete(&dbData).Error; err != nil {
			tx.Rollback()
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to delete Data, details: " + err.Error()})
			return
		}

		// Commit the transaction
		if err := tx.Commit().Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to commit transaction, details: " + err.Error()})
			return
		}

		// Respond with success
		c.JSON(http.StatusOK, gin.H{"message": "Data deleted successfully"})

		dbWeb.Create(&model.LogActivity{
			AdminID:   userID,
			FullName:  userFullName,
			Action:    "Delete Data",
			Status:    "Success",
			Log:       "Data Berhasil Di Hapus : " + jsonString,
			IP:        c.ClientIP(),
			UserAgent: c.Request.UserAgent(),
			ReqMethod: c.Request.Method,
			ReqUri:    c.Request.RequestURI,
		})
	}
}

func TableSuratPeringatanStockOpnameForHR() gin.HandlerFunc {
	return func(c *gin.Context) {
		var request struct {
			Draw       int    `form:"draw"`
			Start      int    `form:"start"`
			Length     int    `form:"length"`
			Search     string `form:"search[value]"`
			SortColumn int    `form:"order[0][column]"`
			SortDir    string `form:"order[0][dir]"`

			No       string `form:"no" json:"no"`
			FullName string `form:"full_name" json:"full_name" gorm:"column:full_name"`
		}

		// Bind form data to request struct
		if err := c.Bind(&request); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}

		dbWeb := gormdb.Databases.Web
		t := reflect.TypeOf(sptechnicianmodel.SPofStockOpname{})

		// Initialize the map
		columnMap := make(map[int]string)

		// Loop through the fields of the struct
		colNum := 0
		for i := 0; i < t.NumField(); i++ {
			field := t.Field(i)
			// Get the JSON key
			jsonKey := field.Tag.Get("json")
			if jsonKey == "" || jsonKey == "-" {
				continue
			}
			columnMap[colNum] = jsonKey
			colNum++
		}

		// Get the column name based on SortColumn value
		sortColumnName := columnMap[request.SortColumn]
		orderString := fmt.Sprintf("%s %s", sortColumnName, request.SortDir)

		// Initial query for filtering
		filteredQuery := dbWeb.Model(&sptechnicianmodel.SPofStockOpname{})

		// Apply filters
		if request.Search != "" {
			for i := 0; i < t.NumField(); i++ {
				dataField := ""
				field := t.Field(i)
				dataType := field.Type.String()
				jsonKey := field.Tag.Get("json")
				gormTag := field.Tag.Get("gorm")

				// Initialize a variable to hold the column key
				columnKey := ""

				// Manually parse the gorm tag to find the column value
				tags := strings.Split(gormTag, ";")
				for _, tag := range tags {
					if strings.HasPrefix(tag, "column:") {
						columnKey = strings.TrimPrefix(tag, "column:")
						break
					}
				}
				if jsonKey == "" || jsonKey == "-" {
					if columnKey == "" || columnKey == "-" {
						continue
					} else {
						dataField = columnKey
					}
				} else {
					dataField = jsonKey
				}
				if jsonKey == "" {
					continue
				}
				if dataType != "string" {
					continue
				}

				filteredQuery = filteredQuery.Or("`"+dataField+"` LIKE ?", "%"+request.Search+"%")
			}

		} else {
			for i := 0; i < t.NumField(); i++ {
				field := t.Field(i)
				formKey := field.Tag.Get("json")

				if formKey == "" || formKey == "-" {
					continue
				}

				// AllowedTypes
				if formKey == "allowed_types" {
					allowedTypes := c.PostFormArray("allowed_types[]")
					if len(allowedTypes) > 0 {
						for _, typ := range allowedTypes {
							jsonFilter, _ := json.Marshal([]string{typ})
							filteredQuery = filteredQuery.Where("JSON_CONTAINS(allowed_types, ?)", string(jsonFilter))
						}
					}
					continue
				}

				formValue := c.PostForm(formKey)

				if formValue != "" {
					isHandled := false

					if strings.Contains(formValue, " to ") {
						// Attempt to parse date range
						dates := strings.Split(formValue, " to ")
						if len(dates) == 2 {
							from, err1 := time.Parse("02/01/2006", strings.TrimSpace(dates[0]))
							to, err2 := time.Parse("02/01/2006", strings.TrimSpace(dates[1]))
							if err1 == nil && err2 == nil {
								filteredQuery = filteredQuery.Where(
									"DATE(`"+formKey+"`) BETWEEN ? AND ?",
									from.Format("2006-01-02"),
									to.Format("2006-01-02"),
								)
								isHandled = true
							}
						}
					} else {
						// Attempt to parse single date
						if date, err := time.Parse("02/01/2006", formValue); err == nil {
							filteredQuery = filteredQuery.Where(
								"DATE(`"+formKey+"`) = ?",
								date.Format("2006-01-02"),
							)
							isHandled = true
						}
					}

					if !isHandled {
						// Fallback to LIKE if no valid date
						if field.Type.Kind() == reflect.Bool ||
							(field.Type.Kind() == reflect.Ptr && field.Type.Elem().Kind() == reflect.Bool) {
							// Convert formValue "true"/"false" to bool or int
							boolVal := false
							if formValue == "true" {
								boolVal = true
							}
							filteredQuery = filteredQuery.Where("`"+formKey+"` = ?", boolVal)
						} else {
							// Other fields: use LIKE
							filteredQuery = filteredQuery.Where("`"+formKey+"` LIKE ?", "%"+formValue+"%")
						}
					}
				}
			}
		}

		// Count the total number of records
		var totalRecords int64
		dbWeb.Model(&sptechnicianmodel.SPofStockOpname{}).Count(&totalRecords)

		// Count the number of filtered records
		var filteredRecords int64
		filteredQuery.Count(&filteredRecords)

		// Apply sorting and pagination to the filtered query
		query := filteredQuery.Order(orderString)
		var Dbdata []sptechnicianmodel.SPofStockOpname
		query = query.Offset(request.Start).Limit(request.Length).Find(&Dbdata)

		if query.Error != nil {
			c.JSON(http.StatusInternalServerError, gin.H{
				"draw":            request.Draw,
				"recordsTotal":    totalRecords,
				"recordsFiltered": 0,
				"data":            []gin.H{},
				"error":           query.Error.Error(),
			})
			return
		}

		var data []gin.H
		for _, dataInDB := range Dbdata {
			newData := make(map[string]interface{})
			v := reflect.ValueOf(dataInDB)

			for i := 0; i < t.NumField(); i++ {
				field := t.Field(i)
				fieldValue := v.Field(i)

				// Get the JSON key
				theKey := field.Tag.Get("json")
				if theKey == "" {
					theKey = field.Tag.Get("form")
					if theKey == "" {
						continue
					}
				}

				// Handle data rendered in col
				switch theKey {
				case "birthdate", "date":
					if fieldValue.Type() == reflect.TypeOf(time.Time{}) {
						t := fieldValue.Interface().(time.Time)
						switch theKey {
						case "birthdate":
							newData[theKey] = t.Format(fun.T_YYYYMMDD)
						case "date":
							newData[theKey] = t.Add(7 * time.Hour).Format(fun.T_YYYYMMDD_HHmmss)
						}
					} else {
						newData[theKey] = fieldValue.Interface()
					}

				case "is_got_sp1", "is_got_sp2", "is_got_sp3":
					t := fieldValue.Interface().(bool)
					var gotSPStatus string
					if t {
						gotSPStatus = "<i class='fad fa-check text-success fs-1'></i>"
					} else {
						gotSPStatus = "<i class='fad fa-times text-danger fs-1'></i>"
					}
					newData[theKey] = gotSPStatus

				case "sp1_sound_played", "sp2_sound_played", "sp3_sound_played":
					t := fieldValue.Interface().(bool)
					var soundPlayedStatus string
					if t {
						soundPlayedStatus = "<i class='fad fa-check text-info fs-1'></i>"
					} else {
						soundPlayedStatus = "<i class='fad fa-times text-danger fs-1'></i>"
					}
					newData[theKey] = soundPlayedStatus

				case "technician":
					t := fieldValue.Interface().(string)
					var technician string
					if t == "" {
						technician = "<span class='text-danger'>N/A</span>"
					} else {
						technician = fmt.Sprintf("<span class='badge bg-label-secondary'><i class='fal fa-user-hard-hat me-2'></i>%s</span>", t)
					}
					newData[theKey] = technician

				case "name":
					t := fieldValue.Interface().(string)
					var name string
					if t == "" {
						name = "<span class='text-danger'>N/A</span>"
					} else {
						name = fun.CapitalizeWord(t)
					}
					newData[theKey] = name

				case "sp1_whatsapp_sent_at", "sp2_whatsapp_sent_at", "sp3_whatsapp_sent_at",
					"sp1_whatsapp_replied_at", "sp2_whatsapp_replied_at", "sp3_whatsapp_replied_at",
					"sp1_whatsapp_reacted_at", "sp2_whatsapp_reacted_at", "sp3_whatsapp_reacted_at",
					"sp1_sound_played_at", "sp2_sound_played_at", "sp3_sound_played_at",
					"got_sp1_at", "got_sp2_at", "got_sp3_at":
					t := fieldValue.Interface().(*time.Time)
					if t != nil {
						newData[theKey] = t.Format("Monday, 02 January 2006 15:04:05")
					} else {
						newData[theKey] = "-"
					}

				case "pelanggaran_sp1", "pelanggaran_sp2", "pelanggaran_sp3":
					t := fieldValue.Interface().(string)
					var pelanggaran string
					if t == "" {
						pelanggaran = "<span class='text-danger'>N/A</span>"
					} else {
						// Truncate text to 30 characters and add Bootstrap tooltip
						truncatedText := t
						if len(t) > 30 {
							truncatedText = t[:30] + "..."
						}
						pelanggaran = fmt.Sprintf(
							`<span data-bs-toggle="tooltip" data-bs-placement="top" title="%s" style="cursor: help;">%s</span>`,
							strings.ReplaceAll(t, `"`, `&quot;`), // Escape quotes for HTML attribute
							truncatedText,
						)
					}
					newData[theKey] = pelanggaran

				case "sp1_whatsapp_message_body", "sp2_whatsapp_message_body", "sp3_whatsapp_message_body":
					t := fieldValue.Interface().(string)
					var messageBody string
					if t == "" {
						messageBody = "<span class='text-danger'>-</span>"
					} else {
						msgText := strings.ReplaceAll(strings.ReplaceAll(t, "*", ""), "_", "")
						messageBody = fmt.Sprintf(
							`<span data-bs-toggle="tooltip" data-bs-placement="top" title="Message text sent to SPL, SAC & HRD" style="cursor: help;">%s</span>`,
							strings.ReplaceAll(msgText, `"`, `&quot;`), // Escape quotes for HTML attribute
						)
					}
					newData[theKey] = messageBody

				case "sp1_file_path", "sp2_file_path", "sp3_file_path":
					t := fieldValue.Interface().(string)
					var spFile string
					if t == "" {
						spFile = "<span class='text-danger'>N/A</span>"
					} else {
						fileSP := strings.ReplaceAll(t, "web/file/sp_so/", "")
						fileSPURL := "/proxy-pdf-sp-so/" + fileSP
						spFile = fmt.Sprintf(`
						<button class="btn btn-sm btn-danger" onclick="openPDFModelForPDFJS('%s')">
						<i class="fal fa-file-pdf me-2"></i> View PDF
						</button>
						`, fileSPURL)
					}
					newData[theKey] = spFile

				case "sp1_sound_tts_path", "sp2_sound_tts_path", "sp3_sound_tts_path":
					t := fieldValue.Interface().(string)
					var soundTTS string
					if t == "" {
						soundTTS = "<span class='text-danger'>N/A</span>"
					} else {
						fileTTS := strings.ReplaceAll(t, "web/file/sounding_sp_so/", "")
						fileTTSURL := "/proxy-mp3-sp-so/" + fileTTS
						soundTTS = fmt.Sprintf(`
						<audio controls>
							<source src="%s" type="audio/mpeg">
							Your browser does not support the audio element.
						</audio>
						`, fileTTSURL)
					}
					newData[theKey] = soundTTS

				case "sp1_whatsapp_replied_by", "sp2_whatsapp_replied_by", "sp3_whatsapp_replied_by":
					t := fieldValue.Interface().(string)
					var repliedBy string
					if t == "" {
						repliedBy = "<span class='text-danger'>N/A</span>"
					} else {
						dbWeb := gormdb.Databases.Web
						var userChatBot model.WAPhoneUser
						normalJID := NormalizeJID(t)
						extractedPhone := extractPhoneFromJID(normalJID)
						repliedBy = extractedPhone
						if err := dbWeb.Where("phone_number = ?", extractedPhone).First(&userChatBot).Error; err != nil {
							logrus.Errorf("failed to find user chat bot with phone number %s: %v", extractedPhone, err)
						}
						repliedBy = fmt.Sprintf("%s (<b>%s</b>)", extractedPhone, userChatBot.FullName)
					}
					newData[theKey] = repliedBy

				case "sp1_whatsapp_reply_text", "sp2_whatsapp_reply_text", "sp3_whatsapp_reply_text":
					t := fieldValue.Interface().(string)
					var replyText string
					if t == "" {
						replyText = "<span class='text-danger'>N/A</span>"
					} else {
						urlPattern := `(https?://[\w\-\.\:]+(/[\w\-\.\?\=\&%/]*)?)`
						re := regexp.MustCompile(urlPattern)
						urls := re.FindAllString(t, -1)
						if len(urls) > 0 {
							replyText = t
							for _, url := range urls {
								anchor := fmt.Sprintf(`<a href="%s" target="_blank" class="text-primary">%s</a>`, url, url)
								replyText = strings.Replace(replyText, url, anchor, 1)
							}
						} else {
							replyText = strings.ReplaceAll(strings.ReplaceAll(t, "*", ""), "_", "")
						}
					}
					newData[theKey] = replyText

				case "sp1_whatsapp_messages", "sp2_whatsapp_messages", "sp3_whatsapp_messages":
					var messages []sptechnicianmodel.SPStockOpnameWhatsappMessage

					// Extract SP number from theKey (e.g., "sp1_whatsapp_messages" -> 1)
					spNumberStr := strings.TrimSuffix(strings.TrimPrefix(theKey, "sp"), "_whatsapp_messages")
					spNumber, err := strconv.Atoi(spNumberStr)
					if err != nil {
						logrus.Errorf("failed to parse SP number from key %s: %v", theKey, err)
						newData[theKey] = "<span class='text-danger'>Invalid Key</span>"
						continue
					}

					if err := dbWeb.
						Where("technician_got_sp_id = ?", dataInDB.ID).
						Where("number_of_sp = ?", spNumber).
						Find(&messages).Error; err != nil {
						logrus.Errorf("failed to fetch whatsapp messages: %v", err)
					}

					if len(messages) == 0 {
						newData[theKey] = "<span class='text-danger'>N/A</span>"
					} else {
						// Enrich messages with sender and destination names
						for i := range messages {
							// Get Sender Name
							if messages[i].WhatsappChatJID != "" {
								var senderUser model.WAPhoneUser
								senderPhone := extractPhoneFromJID(messages[i].WhatsappChatJID)
								if err := dbWeb.Where("phone_number = ?", senderPhone).First(&senderUser).Error; err == nil {
									messages[i].SenderName = senderUser.FullName
								} else {
									messages[i].SenderName = senderPhone // Fallback to phone number
								}
							}

							// Get Destination Name
							if messages[i].WhatsappMessageSentTo != "" {
								var destUser model.WAPhoneUser
								destPhone := extractPhoneFromJID(messages[i].WhatsappMessageSentTo)
								if err := dbWeb.Where("phone_number = ?", destPhone).First(&destUser).Error; err == nil {
									messages[i].DestinationName = destUser.FullName
								} else {
									messages[i].DestinationName = destPhone // Fallback to phone number
								}
							}

							// Create a map for character sanitization
							sanitizeChars := map[string]string{
								"*": "",
								// "_": "",
								"'": "",
							}

							// Sanitize message body
							if messages[i].WhatsappMessageBody != "" {
								messageBody := messages[i].WhatsappMessageBody
								for old, new := range sanitizeChars {
									messageBody = strings.ReplaceAll(messageBody, old, new)
								}
								messages[i].WhatsappMessageBody = messageBody
							}

							// Sanitize reply text if it exists
							if messages[i].WhatsappReplyText != "" {
								replyText := messages[i].WhatsappReplyText
								for old, new := range sanitizeChars {
									replyText = strings.ReplaceAll(replyText, old, new)
								}
								messages[i].WhatsappReplyText = replyText
							}
						}

						messagesJSON, err := json.Marshal(messages)
						if err != nil {
							logrus.Errorf("failed to marshal whatsapp messages: %v", err)
							newData[theKey] = "<span class='text-danger'>Err</span>"
						} else {
							// Use single quotes for the onclick attribute's value.
							// This avoids conflicts with the double quotes inside the JSON string.
							buttonHTML := fmt.Sprintf(`<button class="btn btn-info btn-sm" onclick='showWhatsAppMessageModal(%s)'>Show <i class="fab fa-whatsapp me-2 ms-2"></i> Messages</button>`, string(messagesJSON))
							newData[theKey] = buttonHTML
						}
					}

				default:
					if fieldValue.Type() == reflect.TypeOf(time.Time{}) {
						t := fieldValue.Interface().(time.Time)
						newData[theKey] = t.Format(fun.T_YYYYMMDD_HHmmss)
					} else {
						newData[theKey] = fieldValue.Interface()
					}
				}

			}

			data = append(data, gin.H(newData))
		}

		// Respond with the formatted data for DataTables
		c.JSON(http.StatusOK, gin.H{
			"draw":            request.Draw,
			"recordsTotal":    totalRecords,
			"recordsFiltered": filteredRecords,
			"data":            data,
		})
	}
}
