package controllers

import (
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"service-platform/cmd/web_panel/config"
	"service-platform/cmd/web_panel/fun"
	"service-platform/cmd/web_panel/internal/gormdb"
	"service-platform/cmd/web_panel/model"
	tamodel "service-platform/cmd/web_panel/model/ta_model"
	"strconv"
	"strings"
	"time"
	"unicode"

	"github.com/sirupsen/logrus"
	"go.mau.fi/whatsmeow/types/events"
	"gorm.io/gorm"
)

// fs.technician
type ODOOMSTechnicianItem struct {
	ID                           uint              `json:"id"`
	Email                        nullAbleString    `json:"email"`
	Password                     nullAbleString    `json:"password"`
	NoTelp                       nullAbleString    `json:"x_no_telp"`
	TechnicianName               nullAbleString    `json:"x_technician_name"`
	NameFS                       nullAbleString    `json:"name"`
	Head                         nullAbleString    `json:"x_spl_leader"`
	SPL                          nullAbleString    `json:"technician_code"`
	LoginIDs                     []nullAbleFloat   `json:"login_ids"`
	DownloadIDs                  []nullAbleFloat   `json:"download_ids"`
	EmployeeIDs                  []nullAbleFloat   `json:"employee_ids"`
	CreatedOn                    nullAbleString    `json:"create_date"`
	CreatedUid                   nullAbleInterface `json:"create_uid"`
	WriteDate                    nullAbleString    `json:"write_date"` // last updated time
	WriteUid                     nullAbleInterface `json:"write_uid"`
	JobGroupId                   nullAbleInterface `json:"job_group_id"`
	NIK                          nullAbleString    `json:"nik"`
	Alamat                       nullAbleString    `json:"address"`
	Area                         nullAbleString    `json:"area"`
	TempatTanggalLahir           nullAbleString    `json:"birth_status"`
	StatusPerkawinan             nullAbleString    `json:"marriage_status"`
	BankPenerimaGaji             nullAbleString    `json:"payment_bank"`
	NoRekeningBankPenerimaGaji   nullAbleString    `json:"payment_bank_id"`
	NamaRekeningBankPenerimaGaji nullAbleString    `json:"payment_bank_name"`
	Active                       nullAbleBoolean   `json:"active"`
	EmployeeCode                 nullAbleString    `json:"x_employee_code"`
	TechnicianLocations          []nullAbleFloat   `json:"technician_locations"`
}

func checkExistingTechnicianInODOOMS(name, email, phoneNumber string) (bool, error) {
	odooModel := "fs.technician"
	// Build the OR domain dynamically: x_no_telp ilike phoneNumber OR x_technician_name ilike name OR email ilike email
	odooDomain := []interface{}{
		"|",
		[]interface{}{"x_no_telp", "ilike", phoneNumber},
		"|",
		[]interface{}{"x_technician_name", "ilike", name},
		[]interface{}{"email", "ilike", email},
	}
	odooFields := []string{
		"id",
		"email",
		"x_no_telp",
		"x_technician_name",
	}
	odooOrder := "id desc"

	odooParams := map[string]interface{}{
		"model":  odooModel,
		"domain": odooDomain,
		"fields": odooFields,
		"order":  odooOrder,
	}

	payload := map[string]interface{}{
		"jsonrpc": config.GetConfig().ApiODOO.JSONRPC,
		"params":  odooParams,
	}

	payloadBytes, err := json.Marshal(payload)
	if err != nil {
		return false, err
	}

	url := config.GetConfig().ApiODOO.UrlGetData
	method := "POST"

	body, err := FetchODOOMS(url, method, string(payloadBytes))
	if err != nil {
		return false, err
	}

	var jsonResponse map[string]interface{}
	err = json.Unmarshal(body, &jsonResponse)
	if err != nil {
		return false, err
	}

	if errorResponse, ok := jsonResponse["error"].(map[string]interface{}); ok {
		if errorMessage, ok := errorResponse["message"].(string); ok && errorMessage == "Odoo Session Expired" {
			return false, errors.New("odoo Session Expired")
		} else {
			return false, fmt.Errorf("odoo Error: %v", errorResponse)
		}
	}

	if result, ok := jsonResponse["result"].(map[string]interface{}); ok {
		if message, ok := result["message"].(string); ok {
			success, successOk := result["success"]
			logrus.Infof("ODOO MS Result, message: %v, status: %v", message, successOk && success == true)
		}
	}

	// Check for the existence and validity of the "result" field
	result, resultExists := jsonResponse["result"]
	if !resultExists {
		return false, fmt.Errorf("'result' field not found in the response: %v", jsonResponse)
	}

	// Check if the result is an array and ensure it's not empty
	resultArray, ok := result.([]interface{})
	if !ok || len(resultArray) == 0 {
		return false, fmt.Errorf("'result' is not an array or is empty: %v", result)
	}

	// Take only the first item
	firstItem := resultArray[0]

	// Check that the first item is a map
	itemMap, ok := firstItem.(map[string]interface{})
	if !ok {
		return false, fmt.Errorf("first item is not a map: %v", firstItem)
	}

	// Parse into struct
	var odooData ODOOMSTechnicianItem
	jsonData, err := json.Marshal(itemMap)
	if err != nil {
		return false, fmt.Errorf("error marshalling first item: %v", err)
	}

	err = json.Unmarshal(jsonData, &odooData)
	if err != nil {
		return false, fmt.Errorf("error unmarshalling first item: %v", err)
	}

	// // optionally log or use odooData here
	// logrus.Infof("Parsed first ODOO technician: %+v", odooData)
	// Data exists
	return true, nil
}

func getDataExistingTechnicianInODOOMS(name, email, phoneNumber, woNumber string) (*ODOOMSTechnicianItem, string, string) {
	if name == "" || email == "" || phoneNumber == "" {
		return nil, "Data nama, email atau no telpon teknisi tidak boleh kosong", "Name, email or phone number data cannot be empty"
	}

	odooModel := "fs.technician"
	odooDomain := []interface{}{
		"|",
		[]interface{}{"x_no_telp", "ilike", phoneNumber},
		"|",
		[]interface{}{"x_technician_name", "ilike", name},
		[]interface{}{"email", "ilike", email},
	}
	odooFields := []string{"id", "email", "x_no_telp", "x_technician_name", "name", "x_spl_leader", "technician_code"}
	odooOrder := "id desc"

	odooParams := map[string]interface{}{
		"model":  odooModel,
		"domain": odooDomain,
		"fields": odooFields,
		"order":  odooOrder,
	}

	payload := map[string]interface{}{
		"jsonrpc": config.GetConfig().ApiODOO.JSONRPC,
		"params":  odooParams,
	}

	payloadBytes, err := json.Marshal(payload)
	if err != nil {
		return nil, "Gagal membuat payload JSON", "Failed to create JSON payload"
	}

	url := config.GetConfig().ApiODOO.UrlGetData
	method := "POST"

	body, err := FetchODOOMS(url, method, string(payloadBytes))
	if err != nil {
		return nil, "Gagal memanggil API ODOO", "Failed to call ODOO API"
	}

	var jsonResponse map[string]interface{}
	if err := json.Unmarshal(body, &jsonResponse); err != nil {
		return nil, "Gagal membaca response dari ODOO", "Failed to parse response from ODOO"
	}

	// Handle error from ODOO
	if errorResponse, ok := jsonResponse["error"].(map[string]interface{}); ok {
		if errorMessage, ok := errorResponse["message"].(string); ok && errorMessage == "Odoo Session Expired" {
			return nil, "Sesi ODOO telah kedaluwarsa", "Odoo session expired"
		}
		return nil, fmt.Sprintf("Error dari ODOO: %v", errorResponse), fmt.Sprintf("Error from ODOO: %v", errorResponse)
	}

	// Extract result
	result, ok := jsonResponse["result"]
	if !ok {
		return nil, "'result' tidak ditemukan di response", "'result' field not found in response"
	}

	// If result is array of items
	if resultArray, ok := result.([]interface{}); ok {
		if len(resultArray) == 0 {
			return nil, fmt.Sprintf("Data teknisi %s tidak ditemukan untuk WO Number: %s", name, woNumber), fmt.Sprintf("Technician %s data not found for WO Number: %s", name, woNumber)
		}

		firstItem := resultArray[0]
		itemMap, ok := firstItem.(map[string]interface{})
		if !ok {
			return nil, "Format data teknisi tidak valid", "Invalid technician data format"
		}

		var odooData ODOOMSTechnicianItem
		jsonData, err := json.Marshal(itemMap)
		if err != nil {
			return nil, "Gagal memproses data teknisi", "Failed to process technician data"
		}
		if err := json.Unmarshal(jsonData, &odooData); err != nil {
			return nil, "Gagal membaca data teknisi", "Failed to parse technician data"
		}

		return &odooData, "", "" // success: no error messages
	}

	// Unexpected format
	return nil, "Format data 'result' dari ODOO tidak sesuai", "Unexpected 'result' format from ODOO"
}

func processSingleWONumber(v *events.Message, stanzaID, originalSenderJID, woNumber, userLang string, user *model.WAPhoneUser, startDate, endDate *time.Time) {
	eventToDo := "Get Status from WO Number: " + woNumber

	if strings.TrimSpace(woNumber) == "" {
		id := "Maaf WO Number yang Anda masukkan kosong!"
		en := "Empty WO Number!"
		sendLangMessageWithStanza(v, stanzaID, originalSenderJID, id, en, userLang)
		return
	}
	woNumber = strings.TrimSpace(strings.ToUpper(woNumber))

	// Inform user we've received request
	id, en := informUserRequestReceived(eventToDo)
	sendLangMessageWithStanza(v, stanzaID, originalSenderJID, id, en, userLang)

	var found bool

	DBDataTA := gormdb.Databases.TA

	// 1. Try from LogAct
	var taLog tamodel.LogAct
	tx := DBDataTA.Where("wo LIKE ?", "%"+woNumber+"%").First(&taLog)
	if tx.Error == nil && strings.ToLower(taLog.Method) != "" {
		found = true

		activity := strings.ToLower(taLog.Method)
		var whatTADoID, whatTADoEN string
		switch activity {
		case "submit":
			whatTADoID = "telah mengecek dan meng-submit data ke ODOO."
			whatTADoEN = "has checked and submitted the data to ODOO."
		case "edit":
			whatTADoID = "telah melakukan perubahan data yang telah dikerjakan dan sudah diupload ke ODOO."
			whatTADoEN = "has modified the processed data and uploaded it to ODOO."
		case "delete":
			whatTADoID = "telah menghapus data dari dashboard TA."
			whatTADoEN = "has deleted the data from the TA dashboard."
		}

		if taLog.Wo != nil && *taLog.Wo != "" {
			woNumber = *taLog.Wo
		}
		var sbID, sbEN strings.Builder
		sbID.WriteString(fmt.Sprintf("📌 Rincian WO Number: *%s*\n", woNumber))
		sbEN.WriteString(fmt.Sprintf("📌 Details for WO Number: *%s*\n", woNumber))

		if taLog.Email != "" && whatTADoID != "" {
			DataTA := config.GetConfig().UserTA
			ta, ok := DataTA[taLog.Email]
			if !ok {
				id := fmt.Sprintf("❗️Data TA dengan email %s tidak ditemukan di konfigurasi.", taLog.Email)
				en := fmt.Sprintf("❗️TA data with email %s not found in configuration.", taLog.Email)
				sendLangMessageWithStanza(v, stanzaID, originalSenderJID, id, en, userLang)
				return
			}

			taName := ta.Name
			if taName == "" {
				taName = "N/A"
			}
			sbID.WriteString(fmt.Sprintf("\nTA: _%s_ %s", taName, whatTADoID))
			sbEN.WriteString(fmt.Sprintf("\nTA: _%s_ %s", taName, whatTADoEN))
		}

		if taLog.DateInDashboard != "" {
			sbID.WriteString(fmt.Sprintf("\nMasuk di Dashboard TA per %s", taLog.DateInDashboard))
			sbEN.WriteString(fmt.Sprintf("\nEntered in TA Dashboard on %s", taLog.DateInDashboard))
		}
		if taLog.Date != nil && !taLog.Date.IsZero() {
			sbID.WriteString(fmt.Sprintf("\nTA terakhir melakukan pengecekan pada %v", taLog.Date.Format("2006-01-02 15:04:05")))
			sbEN.WriteString(fmt.Sprintf("\nLast checked by TA on %v", taLog.Date.Format("2006-01-02 15:04:05")))
		}
		if taLog.TaFeedback != "" {
			sbID.WriteString(fmt.Sprintf("\nFeedback dari TA: %s", taLog.TaFeedback))
			sbEN.WriteString(fmt.Sprintf("\nFeedback from TA: %s", taLog.TaFeedback))
		}

		sendLangMessageWithStanza(v, stanzaID, originalSenderJID, sbID.String(), sbEN.String(), userLang)
		return

	} else if tx.Error != nil && !errors.Is(tx.Error, gorm.ErrRecordNotFound) {
		idErr := fmt.Sprintf("❗️Gagal mencari Log TA: %v", tx.Error)
		enErr := fmt.Sprintf("❗️Failed to search TA Log: %v", tx.Error)
		sendLangMessageWithStanza(v, stanzaID, originalSenderJID, idErr, enErr, userLang)
		return
	}

	// 2. Try from Pending
	if !found {
		var taPending tamodel.Pending
		tx = DBDataTA.Where("wo LIKE ?", "%"+woNumber+"%").First(&taPending)
		if tx.Error == nil && taPending.WoNumber != "" {
			found = true

			woNumber = taPending.WoNumber
			var sbID, sbEN strings.Builder
			sbID.WriteString(fmt.Sprintf("🟡 Rincian WO Number: *%s*\n", woNumber))
			sbEN.WriteString(fmt.Sprintf("🟡 Details for WO Number: *%s*\n", woNumber))

			if !taPending.Date.IsZero() {
				sbID.WriteString(fmt.Sprintf("\nMasuk ke Dashboard TA per %v", taPending.Date.Format("2006-01-02 15:04:05")))
				sbEN.WriteString(fmt.Sprintf("\nEntered in TA Dashboard on %v", taPending.Date.Format("2006-01-02 15:04:05")))
			}
			if taPending.TaFeedback != "" {
				sbID.WriteString(fmt.Sprintf("\nFeedback dari TA: %s", taPending.TaFeedback))
				sbEN.WriteString(fmt.Sprintf("\nFeedback from TA: %s", taPending.TaFeedback))
			}

			sendLangMessageWithStanza(v, stanzaID, originalSenderJID, sbID.String(), sbEN.String(), userLang)
			return

		} else if tx.Error != nil && !errors.Is(tx.Error, gorm.ErrRecordNotFound) {
			idErr := fmt.Sprintf("❗️Gagal mencari data Pending TA: %v", tx.Error)
			enErr := fmt.Sprintf("❗️Failed to search Pending TA data: %v", tx.Error)
			sendLangMessageWithStanza(v, stanzaID, originalSenderJID, idErr, enErr, userLang)
			return
		}
	}

	// 3. Try from Error
	if !found {
		var taError tamodel.Error
		tx = DBDataTA.Where("wo LIKE ?", "%"+woNumber+"%").First(&taError)
		if tx.Error == nil && taError.WoNumber != "" {
			found = true

			woNumber = taError.WoNumber
			var sbID, sbEN strings.Builder
			sbID.WriteString(fmt.Sprintf("🔴 Rincian WO Number: *%s*\n", woNumber))
			sbEN.WriteString(fmt.Sprintf("🔴 Details for WO Number: *%s*\n", woNumber))

			if !taError.Date.IsZero() {
				sbID.WriteString(fmt.Sprintf("\nMasuk ke Dashboard TA per %v", taError.Date.Format("2006-01-02 15:04:05")))
				sbEN.WriteString(fmt.Sprintf("\nEntered in TA Dashboard on %v", taError.Date.Format("2006-01-02 15:04:05")))
			}
			if taError.Problem != nil && *taError.Problem != "" {
				sbID.WriteString(fmt.Sprintf("\nProblem: %s", *taError.Problem))
				sbEN.WriteString(fmt.Sprintf("\nProblem: %s", *taError.Problem))
			}
			if taError.TaFeedback != "" {
				sbID.WriteString(fmt.Sprintf("\nFeedback dari TA: %s", taError.TaFeedback))
				sbEN.WriteString(fmt.Sprintf("\nFeedback from TA: %s", taError.TaFeedback))
			}

			sendLangMessageWithStanza(v, stanzaID, originalSenderJID, sbID.String(), sbEN.String(), userLang)
			return

		} else if tx.Error != nil && !errors.Is(tx.Error, gorm.ErrRecordNotFound) {
			idErr := fmt.Sprintf("❗️Gagal mencari data Error TA: %v", tx.Error)
			enErr := fmt.Sprintf("❗️Failed to search TA Error data: %v", tx.Error)
			sendLangMessageWithStanza(v, stanzaID, originalSenderJID, idErr, enErr, userLang)
			return
		}
	}

	// 4. If not found in any table
	idNotFound := fmt.Sprintf("🔍 Data WO Number *%s* tidak ditemukan di database TA (Log, Pending, atau Error). Mungkin belum masuk di dashboard TA atau datanya sudah berhasil dikerjakan.\nKami akan coba mencari statusnya di ODOO, mohon bersabar 🙏🏼", woNumber)
	enNotFound := fmt.Sprintf("🔍 WO Number *%s* was not found in the TA database (Log, Pending, or Error). It may not have been entered into the TA dashboard yet or may have already been processed.\nWe will try to check its status in ODOO, please be patient 🙏🏼", woNumber)
	sendLangMessageWithStanza(v, stanzaID, originalSenderJID, idNotFound, enNotFound, userLang)

	checkWONumberStatusInODOO(v, stanzaID, originalSenderJID, woNumber, userLang, user, startDate, endDate)
}

func checkWONumberStatusInODOO(v *events.Message, stanzaID, originalSenderJID, woNumber, userLang string, user *model.WAPhoneUser, startDate, endDate *time.Time) {
	if woNumber == "" {
		id := "Maaf WO Number yang Anda masukkan kosong!"
		en := "Empty WO Number!"
		sendLangMessageWithStanza(v, stanzaID, originalSenderJID, id, en, userLang)
		return
	}

	woNumber = strings.ToUpper(strings.TrimSpace(woNumber))
	odooModel := "project.task"
	odooOrder := "id desc"
	odooFields := []string{
		"id", "x_merchant", "x_pic_merchant", "x_pic_phone", "partner_street",
		"x_title_cimb", "x_sla_deadline", "create_date", "x_task_type", "company_id",
		"stage_id", "helpdesk_ticket_id", "x_cimb_master_tid", "x_cimb_master_mid",
		"x_source", "x_message_call", "x_no_task", "x_status_merchant", "x_studio_edc",
		"x_product", "x_wo_remark", "x_latitude", "x_longitude", "technician_id",
		"x_received_datetime_spk", "planned_date_begin", "x_reason_code_id",
		"timesheet_timer_last_stop", "worksheet_template_id", "x_ticket_type2",
		"write_uid", "date_last_stage_update",
	}

	var domain []interface{}
	if user.UserType == model.ODOOMSTechnician {
		// Check existing technician data
		dataTechnicianExisting, idText, enText := getDataExistingTechnicianInODOOMS(user.FullName, user.Email, user.PhoneNumber, woNumber)
		if idText != "" || enText != "" {
			sendLangMessageWithStanza(v, stanzaID, originalSenderJID, idText, enText, userLang)
			return
		}

		domain = []interface{}{
			[]interface{}{"active", "=", true},
			[]interface{}{"x_no_task", "=", woNumber},
			[]interface{}{"technician_id", "ilike", dataTechnicianExisting.NameFS.String},
			[]interface{}{"technician_id", "ilike", dataTechnicianExisting.TechnicianName.String},
		}
	} else {
		domain = []interface{}{
			[]interface{}{"active", "=", true},
			[]interface{}{"x_no_task", "=", woNumber},
		}
	}

	if endDate != nil && !endDate.IsZero() && startDate != nil && !startDate.IsZero() {
		// Subtract 7 hours from the dates
		adjustedStartDate := startDate.Add(-7 * time.Hour)
		adjustedEndDate := endDate.Add(-7 * time.Hour)

		// Format the adjusted dates
		domain = append(domain, []interface{}{"timesheet_timer_last_stop", ">=", adjustedStartDate.Format("2006-01-02 15:04:05")})
		domain = append(domain, []interface{}{"timesheet_timer_last_stop", "<=", adjustedEndDate.Format("2006-01-02 15:04:05")})
	}

	if startDate != nil && !startDate.IsZero() && endDate == nil {
		// Get the date at 00:00:00 of startDate's day
		startOfDay := time.Date(
			startDate.Year(),
			startDate.Month(),
			startDate.Day(),
			0, 0, 0, 0,
			startDate.Location(),
		)

		// Get the date at 23:59:59 of startDate's day
		endOfDay := time.Date(
			startDate.Year(),
			startDate.Month(),
			startDate.Day(),
			23, 59, 59, 0,
			startDate.Location(),
		)

		// Subtract 7 hours from both bounds
		adjustedStart := startOfDay.Add(-7 * time.Hour)
		adjustedEnd := endOfDay.Add(-7 * time.Hour)

		// Add to domain
		domain = append(domain, []interface{}{"timesheet_timer_last_stop", ">=", adjustedStart.Format("2006-01-02 15:04:05")})
		domain = append(domain, []interface{}{"timesheet_timer_last_stop", "<=", adjustedEnd.Format("2006-01-02 15:04:05")})
	}

	odooParams := map[string]interface{}{
		"domain": domain,
		"model":  odooModel,
		"fields": odooFields,
		"order":  odooOrder,
	}

	payload := map[string]interface{}{
		"jsonrpc": config.GetConfig().ApiODOO.JSONRPC,
		"params":  odooParams,
	}

	payloadBytes, err := json.Marshal(payload)
	if err != nil {
		id := fmt.Sprintf("⚠ Mohon maaf, terjadi kesalahan saat menyiapkan permintaan untuk WO Number: *%s*.\n~_%v_", woNumber, err)
		en := fmt.Sprintf("⚠ Sorry, an error occurred while preparing request for WO Number: *%s*.\n~_%v_", woNumber, err)
		sendLangMessageWithStanza(v, stanzaID, originalSenderJID, id, en, userLang)
		return
	}

	result, err := GetODOOMSData(string(payloadBytes))
	if err != nil {
		id := fmt.Sprintf("⚠ Mohon maaf, terjadi kesalahan saat mengambil data WO Number: *%s* di ODOO.\n~_%v_", woNumber, err)
		en := fmt.Sprintf("⚠ Sorry, an error occurred while fetching data for WO Number: *%s* in ODOO.\n~_%v_", woNumber, err)
		sendLangMessageWithStanza(v, stanzaID, originalSenderJID, id, en, userLang)
		return
	}

	resultArray, ok := result.([]interface{})
	if !ok || len(resultArray) == 0 {
		id := fmt.Sprintf("🔍 Data WO Number *%s* tidak ditemukan di ODOO.", woNumber)
		en := fmt.Sprintf("🔍 WO Number *%s* was not found in ODOO.", woNumber)
		sendLangMessageWithStanza(v, stanzaID, originalSenderJID, id, en, userLang)
		return
	}

	recordMap, ok := resultArray[0].(map[string]interface{})
	if !ok {
		id := "⚠ Format data ODOO tidak sesuai yang diharapkan."
		en := "⚠ The ODOO data format is not as expected."
		sendLangMessageWithStanza(v, stanzaID, originalSenderJID, id, en, userLang)
		return
	}

	var odooData OdooTaskDataRequestItem
	jsonData, err := json.Marshal(recordMap)
	if err != nil {
		id := fmt.Sprintf("⚠ Mohon maaf, terjadi kesalahan saat membaca data WO Number: *%s*.\n~_%v_", woNumber, err)
		en := fmt.Sprintf("⚠ Sorry, an error occurred while reading data for WO Number: *%s*.\n~_%v_", woNumber, err)
		sendLangMessageWithStanza(v, stanzaID, originalSenderJID, id, en, userLang)
		return
	}
	if err = json.Unmarshal(jsonData, &odooData); err != nil {
		id := fmt.Sprintf("⚠ Mohon maaf, terjadi kesalahan saat memproses data WO Number: *%s*.\n~_%v_", woNumber, err)
		en := fmt.Sprintf("⚠ Sorry, an error occurred while processing data for WO Number: *%s*.\n~_%v_", woNumber, err)
		sendLangMessageWithStanza(v, stanzaID, originalSenderJID, id, en, userLang)
		return
	}

	_, stage, err := parseJSONIDDataCombined(odooData.StageId)
	if err != nil {
		id := fmt.Sprintf("⚠ Mohon maaf, gagal membaca status WO Number: *%s*.\n~_%v_", woNumber, err)
		en := fmt.Sprintf("⚠ Sorry, failed to read stage info for WO Number: *%s*.\n~_%v_", woNumber, err)
		sendLangMessageWithStanza(v, stanzaID, originalSenderJID, id, en, userLang)
		return
	}

	_, edcType, err := parseJSONIDDataCombined(odooData.EdcType)
	if err != nil {
		id := fmt.Sprintf("⚠ Mohon maaf, gagal membaca tipe EDC WO Number: *%s*.\n~_%v_", woNumber, err)
		en := fmt.Sprintf("⚠ Sorry, failed to read EDC type for WO Number: *%s*.\n~_%v_", woNumber, err)
		sendLangMessageWithStanza(v, stanzaID, originalSenderJID, id, en, userLang)
		return
	}

	_, snEdc, err := parseJSONIDDataCombined(odooData.SnEdc)
	if err != nil {
		id := fmt.Sprintf("⚠ Mohon maaf, gagal membaca SN EDC WO Number: *%s*.\n~_%v_", woNumber, err)
		en := fmt.Sprintf("⚠ Sorry, failed to read EDC Serial Number for WO Number: *%s*.\n~_%v_", woNumber, err)
		sendLangMessageWithStanza(v, stanzaID, originalSenderJID, id, en, userLang)
		return
	}

	_, lastUpdateBy, err := parseJSONIDDataCombined(odooData.WriteUid)
	if err != nil {
		id := fmt.Sprintf("⚠ Mohon maaf, gagal membaca data pengguna terakhir WO Number: *%s*.\n~_%v_", woNumber, err)
		en := fmt.Sprintf("⚠ Sorry, failed to read last updated by info for WO Number: *%s*.\n~_%v_", woNumber, err)
		sendLangMessageWithStanza(v, stanzaID, originalSenderJID, id, en, userLang)
		return
	}

	// Build message
	var sbID, sbEN strings.Builder
	sbID.WriteString(fmt.Sprintf("📝 Detail dari WO Number *%s* berdasarkan ODOO\n", woNumber))
	sbEN.WriteString(fmt.Sprintf("📝 Details for WO Number *%s* based on ODOO\n", woNumber))

	if stage != "" {
		sbID.WriteString(fmt.Sprintf("\nStatus: *%s*", stage))
		sbEN.WriteString(fmt.Sprintf("\nStatus: *%s*", stage))
	}
	if odooData.SlaDeadline.Valid {
		slaTime := odooData.SlaDeadline.Time.Add(7 * time.Hour).Format("2006-01-02 15:04:05")
		sbID.WriteString(fmt.Sprintf("\nSLA Deadline: %s WIB", slaTime))
		sbEN.WriteString(fmt.Sprintf("\nSLA Deadline: %s WIB", slaTime))
	}
	if odooData.MerchantName.String != "" {
		sbID.WriteString(fmt.Sprintf("\nMerchant: %s", odooData.MerchantName.String))
		sbEN.WriteString(fmt.Sprintf("\nMerchant: %s", odooData.MerchantName.String))
	}
	if odooData.MerchantAddress.String != "" {
		sbID.WriteString(fmt.Sprintf("\nAlamat Merchant: %s", odooData.MerchantAddress.String))
		sbEN.WriteString(fmt.Sprintf("\nMerchant Address: %s", odooData.MerchantAddress.String))
	}
	if odooData.StatusMerchant.String != "" {
		sbID.WriteString(fmt.Sprintf("\nStatus Merchant: %s", odooData.StatusMerchant.String))
		sbEN.WriteString(fmt.Sprintf("\nMerchant Status: %s", odooData.StatusMerchant.String))
	}
	if odooData.Mid.String != "" {
		sbID.WriteString(fmt.Sprintf("\nMID: %s", odooData.Mid.String))
		sbEN.WriteString(fmt.Sprintf("\nMID: %s", odooData.Mid.String))
	}
	if odooData.Tid.String != "" {
		sbID.WriteString(fmt.Sprintf("\nTID: %s", odooData.Tid.String))
		sbEN.WriteString(fmt.Sprintf("\nTID: %s", odooData.Tid.String))
	}
	if edcType != "" {
		sbID.WriteString(fmt.Sprintf("\nTipe EDC: %s", edcType))
		sbEN.WriteString(fmt.Sprintf("\nEDC Type: %s", edcType))
	}
	if snEdc != "" {
		sbID.WriteString(fmt.Sprintf("\nSN EDC: %s", edcType))
		sbEN.WriteString(fmt.Sprintf("\nEDC Serial Number: %s", edcType))
	}
	if odooData.TimesheetLastStop.Valid {
		ts := odooData.TimesheetLastStop.Time.
			Add(7 * time.Hour).
			Format("2006-01-02 15:04:05")

		sbID.WriteString(fmt.Sprintf("\nTerakhir kunjungan: %s", ts))
		sbEN.WriteString(fmt.Sprintf("\nLast visit: %s", ts))
	}
	if lastUpdateBy != "" {
		sbID.WriteString(fmt.Sprintf("\nTerakhir diubah oleh: %s", lastUpdateBy))
		sbEN.WriteString(fmt.Sprintf("\nLast updated by: %s", lastUpdateBy))
	}
	if odooData.DateLastStageUpdate.Valid {
		ts := odooData.DateLastStageUpdate.Time.Format("2006-01-02 15:04:05")
		sbID.WriteString(fmt.Sprintf("\nStatus terakhir diubah pada: %s", ts))
		sbEN.WriteString(fmt.Sprintf("\nStage last updated on: %s", ts))
	}
	if odooData.Description.String != "" {
		sbID.WriteString(fmt.Sprintf("\nDeskripsi: %s", odooData.Description.String))
		sbEN.WriteString(fmt.Sprintf("\nDescription: %s", odooData.Description.String))
	}

	// Send to user
	sendLangMessageWithStanza(v, stanzaID, originalSenderJID, sbID.String(), sbEN.String(), userLang)
}

func CheckODOOMSTechnicianExists(db *gorm.DB) {
	taskDoing := "Check Existing Technician in ODOO MS"
	if !checkTechnicianExistsInODOOMSMutex.TryLock() {
		logrus.Warnf("Another process is already fetching %s, skipping this request", taskDoing)
		return
	}
	defer checkTechnicianExistsInODOOMSMutex.Unlock()

	var waUser []model.WAPhoneUser
	if err := db.
		Where("is_registered = ?", true).
		Where("is_banned = ?", false).
		Where("user_type = ?", model.ODOOMSTechnician).
		Find(&waUser).
		Error; err != nil {
		logrus.Errorf("failed to check data for existing technician: %v", err)
	}

	var pattern = regexp.MustCompile(`^\[[^\]]+\]\s*-\s*`)

	const markNotExists = "[Technician Not Exists in ODOO MS] - "

	for _, user := range waUser {
		technicianIsExists, err := checkExistingTechnicianInODOOMS(user.FullName, user.Email, user.PhoneNumber)
		if err != nil {
			logrus.Error(err)
			// continue
		}

		if !technicianIsExists {
			newDesc := strings.TrimSpace(user.Description)

			if newDesc == "" {
				newDesc = strings.TrimSpace(strings.ReplaceAll(markNotExists, "-", ""))
			} else if pattern.MatchString(newDesc) {
				// Replace existing [xxx] - with our mark
				newDesc = pattern.ReplaceAllString(newDesc, markNotExists)
			} else {
				// No pattern, prepend ours
				newDesc = markNotExists + newDesc
			}

			// update in DB
			if err := db.Model(&model.WAPhoneUser{}).
				Where("id = ?", user.ID).
				Update("description", newDesc).Error; err != nil {
				logrus.Errorf("failed to update description for user ID %d: %v", user.ID, err)
			} else {
				logrus.Infof("✅ Updated description for user ID %d", user.ID)
			}
		}
	}

}

func checkAndProcessTIDs(cmd string, v *events.Message, stanzaID, originalSenderJID, userLang string, user *model.WAPhoneUser) bool {
	// Regex: tid (optional colon) numbers (digits, spaces/newlines), then optional "date:" and date string
	re := regexp.MustCompile(`(?i)tid\s*:?\s*([\d\s\n]+)(?:date:?\s*(.+))?`)
	matches := re.FindAllStringSubmatch(cmd, -1)

	if len(matches) > 0 {
		for _, m := range matches {
			// m[1] is the TIDs part; m[2] (if exists) is the date part
			tidsRaw := m[1]
			datePart := ""
			if len(m) > 2 {
				datePart = strings.TrimSpace(m[2])
			}

			// Extract individual TID numbers
			tidNumbers := regexp.MustCompile(`\d+`).FindAllString(tidsRaw, -1)

			// Try to parse date(s)
			var startDate, endDate *time.Time
			if datePart != "" {
				sd, ed, found := fun.ExtractDateOrRange(datePart)
				if found {
					startDate = sd
					endDate = ed
				}
			}

			if len(tidNumbers) == 1 {
				processSingleTID(v, stanzaID, originalSenderJID, tidNumbers[0], userLang, user, startDate, endDate)
				return true
			} else if len(tidNumbers) > 1 {
				processMultipleTIDs(v, stanzaID, originalSenderJID, tidNumbers, userLang, user, startDate, endDate)
				return true
			}
		}
	}

	// fallback: continue normal flow if no TIDs found
	return false
}

func getTAInfoForTID(tid string) (sbID string, sbEN string, found bool, collectedWONumber []string, err error) {
	var sbIDBuilder, sbENBuilder strings.Builder
	var localFound bool
	var woList []string

	DBDataTA := gormdb.Databases.TA
	DataTA := config.GetConfig().UserTA

	// 1. LogAct
	var taLogs []tamodel.LogAct
	tx := DBDataTA.Where("tid LIKE ?", "%"+tid+"%").Find(&taLogs)
	if tx.Error != nil && !errors.Is(tx.Error, gorm.ErrRecordNotFound) {
		return "", "", false, nil, fmt.Errorf("failed to search TA Log: %v", tx.Error)
	}

	if len(taLogs) > 0 {
		localFound = true
		for _, log := range taLogs {
			activity := strings.ToLower(log.Method)
			var wo string
			if log.Wo != nil && *log.Wo != "" {
				wo = *log.Wo
				woList = append(woList, wo)
			} else {
				wo = "N/A"
			}

			var whatTADoID, whatTADoEN string
			switch activity {
			case "submit":
				whatTADoID = "telah mengecek dan meng-submit data ke ODOO."
				whatTADoEN = "has checked and submitted the data to ODOO."
			case "edit":
				whatTADoID = "telah melakukan perubahan data yang telah dikerjakan dan sudah diupload ke ODOO."
				whatTADoEN = "has modified the processed data and uploaded it to ODOO."
			case "delete":
				whatTADoID = "telah menghapus data dari dashboard TA."
				whatTADoEN = "has deleted the data from the TA dashboard."
			}

			sbIDBuilder.WriteString(fmt.Sprintf("📌 Rincian WO Number: *%v*\n", wo))
			sbENBuilder.WriteString(fmt.Sprintf("📌 Details for WO Number: *%v*\n", wo))

			if log.Tid != "" {
				sbIDBuilder.WriteString(fmt.Sprintf("\nTID: _%s_", log.Tid))
				sbENBuilder.WriteString(fmt.Sprintf("\nTID: _%s_", log.Tid))
			}

			if log.Email != "" && whatTADoID != "" {
				ta, ok := DataTA[log.Email]
				if !ok {
					return "", "", false, nil, fmt.Errorf("failed to find TA data with email %s in configuration", log.Email)
				}
				taName := ta.Name
				if taName == "" {
					taName = "N/A"
				}

				sbIDBuilder.WriteString(fmt.Sprintf("\nTA: _%s_ %s", taName, whatTADoID))
				sbENBuilder.WriteString(fmt.Sprintf("\nTA: _%s_ %s", taName, whatTADoEN))
			}

			if log.DateInDashboard != "" {
				sbIDBuilder.WriteString(fmt.Sprintf("\nMasuk di Dashboard TA per %s", log.DateInDashboard))
				sbENBuilder.WriteString(fmt.Sprintf("\nEntered in TA Dashboard on %s", log.DateInDashboard))
			}
			if log.Sla != "" {
				sbIDBuilder.WriteString(fmt.Sprintf("\nSLA Deadline: %s WIB", log.Sla))
				sbENBuilder.WriteString(fmt.Sprintf("\nSLA Deadline: %s WIB", log.Sla))
			}
			if log.Date != nil && !log.Date.IsZero() {
				sbIDBuilder.WriteString(fmt.Sprintf("\nTA terakhir melakukan pengecekan pada %v", log.Date.Format("2006-01-02 15:04:05")))
				sbENBuilder.WriteString(fmt.Sprintf("\nLast checked by TA on %v", log.Date.Format("2006-01-02 15:04:05")))
			}
			if log.TaFeedback != "" {
				sbIDBuilder.WriteString(fmt.Sprintf("\nFeedback dari TA: %s", log.TaFeedback))
				sbENBuilder.WriteString(fmt.Sprintf("\nFeedback from TA: %s", log.TaFeedback))
			}
			sbIDBuilder.WriteString("\n\n")
			sbENBuilder.WriteString("\n\n")
		}
	}

	// 2. Pending
	var taPendings []tamodel.Pending
	tx = DBDataTA.Where("tid LIKE ?", "%"+tid+"%").Find(&taPendings)
	if tx.Error != nil && !errors.Is(tx.Error, gorm.ErrRecordNotFound) {
		return "", "", false, nil, fmt.Errorf("failed to search Pending TA: %v", tx.Error)
	}
	if len(taPendings) > 0 {
		localFound = true
		for _, pending := range taPendings {
			if pending.WoNumber != "" {
				woList = append(woList, pending.WoNumber)
			}

			sbIDBuilder.WriteString(fmt.Sprintf("🟡 Rincian WO Number: *%s*\n", pending.WoNumber))
			sbENBuilder.WriteString(fmt.Sprintf("🟡 Details for WO Number: *%s*\n", pending.WoNumber))

			if pending.TID != "" {
				sbIDBuilder.WriteString(fmt.Sprintf("\nTID: %s", pending.TID))
				sbENBuilder.WriteString(fmt.Sprintf("\nTID: %s", pending.TID))
			}

			if !pending.Date.IsZero() {
				sbIDBuilder.WriteString(fmt.Sprintf("\nMasuk ke Dashboard TA per %v", pending.Date.Format("2006-01-02 15:04:05")))
				sbENBuilder.WriteString(fmt.Sprintf("\nEntered in TA Dashboard on %v", pending.Date.Format("2006-01-02 15:04:05")))
			}
			if *pending.Sla != "" {
				sbIDBuilder.WriteString(fmt.Sprintf("\nSLA Deadline: %s WIB", *pending.Sla))
				sbENBuilder.WriteString(fmt.Sprintf("\nSLA Deadline: %s WIB", *pending.Sla))
			}
			if pending.TaFeedback != "" {
				sbIDBuilder.WriteString(fmt.Sprintf("\nFeedback dari TA: %s", pending.TaFeedback))
				sbENBuilder.WriteString(fmt.Sprintf("\nFeedback from TA: %s", pending.TaFeedback))
			}
			sbIDBuilder.WriteString("\n\n")
			sbENBuilder.WriteString("\n\n")
		}
	}

	// 3. Error
	var taErrors []tamodel.Error
	tx = DBDataTA.Where("tid LIKE ?", "%"+tid+"%").Find(&taErrors)
	if tx.Error != nil && !errors.Is(tx.Error, gorm.ErrRecordNotFound) {
		return "", "", false, nil, fmt.Errorf("failed to search Error TA: %v", tx.Error)
	}
	if len(taErrors) > 0 {
		localFound = true
		for _, e := range taErrors {
			if e.WoNumber != "" {
				woList = append(woList, e.WoNumber)
			}

			sbIDBuilder.WriteString(fmt.Sprintf("🔴 Rincian WO Number: *%s*\n", e.WoNumber))
			sbENBuilder.WriteString(fmt.Sprintf("🔴 Details for WO Number: *%s*\n", e.WoNumber))

			if e.TID != "" {
				sbIDBuilder.WriteString(fmt.Sprintf("\nTID: %s", e.TID))
				sbENBuilder.WriteString(fmt.Sprintf("\nTID: %s", e.TID))
			}

			if !e.Date.IsZero() {
				sbIDBuilder.WriteString(fmt.Sprintf("\nMasuk ke Dashboard TA per %v", e.Date.Format("2006-01-02 15:04:05")))
				sbENBuilder.WriteString(fmt.Sprintf("\nEntered in TA Dashboard on %v", e.Date.Format("2006-01-02 15:04:05")))
			}
			if *e.Sla != "" {
				sbIDBuilder.WriteString(fmt.Sprintf("\nSLA Deadline: %s WIB", *e.Sla))
				sbENBuilder.WriteString(fmt.Sprintf("\nSLA Deadline: %s WIB", *e.Sla))
			}
			if e.Problem != nil && *e.Problem != "" {
				sbIDBuilder.WriteString(fmt.Sprintf("\nProblem: %s", *e.Problem))
				sbENBuilder.WriteString(fmt.Sprintf("\nProblem: %s", *e.Problem))
			}
			if e.TaFeedback != "" {
				sbIDBuilder.WriteString(fmt.Sprintf("\nFeedback dari TA: %s", e.TaFeedback))
				sbENBuilder.WriteString(fmt.Sprintf("\nFeedback from TA: %s", e.TaFeedback))
			}
			sbIDBuilder.WriteString("\n\n")
			sbENBuilder.WriteString("\n\n")
		}
	}

	return sbIDBuilder.String(), sbENBuilder.String(), localFound, woList, nil
}

func processSingleTID(v *events.Message, stanzaID, originalSenderJID, tid, userLang string, user *model.WAPhoneUser, startDate, endDate *time.Time) {
	eventToDo := "Processing TID: " + tid
	if strings.TrimSpace(tid) == "" {
		id := "Maaf TID yang Anda masukkan kosong!"
		en := "Empty TID!"
		sendLangMessageWithStanza(v, stanzaID, originalSenderJID, id, en, userLang)
		return
	}

	// Inform user we've received request
	id, en := informUserRequestReceived(eventToDo)
	sendLangMessageWithStanza(v, stanzaID, originalSenderJID, id, en, userLang)

	// use the helper
	sbID, sbEN, found, woList, err := getTAInfoForTID(tid)
	if err != nil {
		idErr := fmt.Sprintf("❗️Gagal memproses TID %s: %v", tid, err)
		enErr := fmt.Sprintf("❗️Failed to process TID %s: %v", tid, err)
		sendLangMessageWithStanza(v, stanzaID, originalSenderJID, idErr, enErr, userLang)
		return
	}

	if found {
		sendLangMessageWithStanza(v, stanzaID, originalSenderJID, sbID, sbEN, userLang)
	} else {
		idNotFound := fmt.Sprintf("🔍 Data TID *%s* tidak ditemukan di database TA (Log, Pending, atau Error).", tid)
		enNotFound := fmt.Sprintf("🔍 TID *%s* was not found in the TA database (Log, Pending, or Error).", tid)
		sendLangMessageWithStanza(v, stanzaID, originalSenderJID, idNotFound, enNotFound, userLang)
	}

	if len(woList) > 0 {
		// trigger ODOO check for each WO number
		for _, wo := range woList {
			checkWONumberStatusInODOO(v, stanzaID, originalSenderJID, wo, userLang, user, startDate, endDate)
		}
	}
}

func processMultipleTIDs(v *events.Message, stanzaID, originalSenderJID string, tids []string, userLang string, user *model.WAPhoneUser, startDate, endDate *time.Time) {
	eventToDo := "Processing TIDs: " + strings.Join(tids, ", ")
	if len(tids) == 0 {
		id := "Maaf TID yang Anda masukkan kosong!"
		en := "Empty TID!"
		sendLangMessageWithStanza(v, stanzaID, originalSenderJID, id, en, userLang)
		return
	}

	// Inform user we've received request
	id, en := informUserRequestReceived(eventToDo)
	sendLangMessageWithStanza(v, stanzaID, originalSenderJID, id, en, userLang)

	var allID strings.Builder
	var allEN strings.Builder
	var anyFound bool

	uniqueMap := make(map[string]struct{})
	var uniqueTIDs []string

	for _, tid := range tids {
		if _, exists := uniqueMap[tid]; !exists {
			uniqueMap[tid] = struct{}{}
			uniqueTIDs = append(uniqueTIDs, tid)
		}
	}

	for _, tid := range uniqueTIDs {
		sbID, sbEN, found, woList, err := getTAInfoForTID(tid)
		if err != nil {
			idErr := fmt.Sprintf("❗️Gagal memproses TID %s: %v", tid, err)
			enErr := fmt.Sprintf("❗️Failed to process TID %s: %v", tid, err)
			sendLangMessageWithStanza(v, stanzaID, originalSenderJID, idErr, enErr, userLang)
			continue
		}
		if found {
			anyFound = true
			allID.WriteString(fmt.Sprintf("📝 *TID %s*\n%s", tid, sbID))
			allEN.WriteString(fmt.Sprintf("📝 *TID %s*\n%s", tid, sbEN))
		} else {
			idNotFound := fmt.Sprintf("🔍 Data TID *%s* tidak ditemukan di database TA (Log, Pending, atau Error).", tid)
			enNotFound := fmt.Sprintf("🔍 TID *%s* was not found in the TA database (Log, Pending, or Error).", tid)
			allID.WriteString(idNotFound + "\n\n")
			allEN.WriteString(enNotFound + "\n\n")
		}

		if len(woList) > 0 {
			// trigger ODOO check for each WO number
			for _, wo := range woList {
				checkWONumberStatusInODOO(v, stanzaID, originalSenderJID, wo, userLang, user, startDate, endDate)
			}
		}
	}

	sendLangMessageWithStanza(v, stanzaID, originalSenderJID, allID.String(), allEN.String(), userLang)

	_ = anyFound
}

// ParseWOWithDate parses input like:
// "wo 123123 2025-01-01"
// "wo: 123123 date: 23 Jul 2025"
// "wo: 123123 2025-01-01 - 2025-01-02"
//
// Returns:
// - woNumber: string
// - startDate: *time.Time (nil if not found)
// - endDate: *time.Time (nil if single date or not found)
// - ok: true if WO number found
func ParseWOWithDate(cmd string) (woNumber string, startDate *time.Time, endDate *time.Time, ok bool) {
	// normalize
	cmd = strings.TrimSpace(cmd)

	// match prefix: wo, wo:, wo :
	re := regexp.MustCompile(`(?i)^wo\s*:?\s*`)
	if !re.MatchString(cmd) {
		return
	}
	rest := re.ReplaceAllString(cmd, "")
	rest = strings.TrimSpace(rest)

	// split rest into tokens
	tokens := strings.Fields(rest)
	if len(tokens) == 0 {
		return
	}

	// first token must be WO number (digits)
	woNumber = tokens[0]

	// join remaining tokens back to look for date
	datePart := strings.Join(tokens[1:], " ")

	// support optional "date:" keyword (e.g., "date: 23 Jul 2025")
	datePart = strings.Replace(datePart, "date:", "", 1)
	datePart = strings.TrimSpace(datePart)

	// check if date exists in datePart
	if datePart != "" {
		sd, ed, found := fun.ExtractDateOrRange(datePart)
		if found {
			startDate = sd
			endDate = ed
		}
	}

	return woNumber, startDate, endDate, true
}

// GetFirstNumberWordTechnician extracts the first "numeric-like" word and its first digit as int
func GetFirstNumberWordTechnician(input string) (firstWord string, firstDigit int, isNumeric bool) {
	words := strings.Fields(input)
	if len(words) == 0 {
		return "", 0, false
	}

	first := words[0]

	// Check if it starts with a digit
	if len(first) > 0 && unicode.IsDigit(rune(first[0])) {
		if _, err := strconv.ParseFloat(first, 64); err == nil {
			// Convert the first digit character to int
			digit, err := strconv.Atoi(string(first[0]))
			if err == nil {
				return first, digit, true
			}
		}
	}

	return "", 0, false
}

func checkAndProcessInfoTIDs(cmd string, v *events.Message, stanzaID, originalSenderJID, userLang string, user *model.WAPhoneUser) bool {
	// Regex: info (optional colon) then one or more TID numbers (digits, possibly separated by spaces)
	re := regexp.MustCompile(`(?i)info\s*:?\s*((?:\d+\s*)+)`)
	matches := re.FindStringSubmatch(cmd)
	if len(matches) > 1 {
		tidsRaw := matches[1]
		// Extract individual TID numbers
		tidNumbers := regexp.MustCompile(`\d+`).FindAllString(tidsRaw, -1)

		if len(tidNumbers) > 0 {
			processInfoTIDs(v, stanzaID, originalSenderJID, userLang, tidNumbers, user)
			return true
		}

	}

	// fallback: continue normal flow if no TIDs found
	return false
}

func processInfoTIDs(v *events.Message, stanzaID, originalSenderJID, userLang string, tid []string, user *model.WAPhoneUser) {
	_ = user // currently unused
	eventToDo := "Processing Info TID: " + strings.Join(tid, ", ")
	if len(tid) == 0 {
		id := "Maaf TID yang Anda masukkan kosong!"
		en := "Empty TID!"
		sendLangMessageWithStanza(v, stanzaID, originalSenderJID, id, en, userLang)
		return
	}

	// Inform user we've received request
	id, en := informUserRequestReceived(eventToDo)
	sendLangMessageWithStanza(v, stanzaID, originalSenderJID, id, en, userLang)

	model := "res.partner"
	order := "id desc"
	fields := []string{
		"active",
		"technician_id",
		"street",
		"name",
		"x_merchant",
		"x_merchant_pic",
		"x_merchant_pic_phone",
		"x_cimb_master_tid",
		"x_cimb_master_mid",
		"partner_latitude",
		"partner_longitude",
		"x_studio_sn_edc",
		"x_product",
		"x_source",
		"x_simcard",
		"x_simcard_provider",
	}

	domain := []interface{}{
		[]interface{}{"x_cimb_master_tid", "=", tid},
	}

	params := map[string]interface{}{
		"domain": domain,
		"model":  model,
		"fields": fields,
		"order":  order,
	}

	payload := map[string]interface{}{
		"jsonrpc": config.GetConfig().ApiODOO.JSONRPC,
		"params":  params,
	}

	payloadBytes, err := json.Marshal(payload)
	if err != nil {
		id := fmt.Sprintf("⚠ Mohon maaf, terjadi kesalahan saat menyiapkan permintaan untuk TID: *%s*.\n~_%v_", strings.Join(tid, ", "), err)
		en := fmt.Sprintf("⚠ Sorry, an error occurred while preparing request for TID: *%s*.\n~_%v_", strings.Join(tid, ", "), err)
		sendLangMessageWithStanza(v, stanzaID, originalSenderJID, id, en, userLang)
		return
	}

	result, err := GetODOOMSData(string(payloadBytes))
	if err != nil {
		id := fmt.Sprintf("⚠ Mohon maaf, terjadi kesalahan saat mengambil data TID: *%s* di ODOO.\n~_%v_", strings.Join(tid, ", "), err)
		en := fmt.Sprintf("⚠ Sorry, an error occurred while fetching data for TID: *%s* in ODOO.\n~_%v_", strings.Join(tid, ", "), err)
		sendLangMessageWithStanza(v, stanzaID, originalSenderJID, id, en, userLang)
		return
	}

	resultArray, ok := result.([]interface{})
	if !ok || len(resultArray) == 0 {
		id := fmt.Sprintf("🔍 Data TID *%s* tidak ditemukan di ODOO.", strings.Join(tid, ", "))
		en := fmt.Sprintf("🔍 TID *%s* was not found in ODOO.", strings.Join(tid, ", "))
		sendLangMessageWithStanza(v, stanzaID, originalSenderJID, id, en, userLang)
		return
	}

	resultBytes, err := json.Marshal(resultArray)
	if err != nil {
		id := fmt.Sprintf("⚠ Mohon maaf, terjadi kesalahan saat membaca data TID: *%s*.\n~_%v_", strings.Join(tid, ", "), err)
		en := fmt.Sprintf("⚠ Sorry, an error occurred while reading data for TID: *%s*.\n~_%v_", strings.Join(tid, ", "), err)
		sendLangMessageWithStanza(v, stanzaID, originalSenderJID, id, en, userLang)
		return
	}

	var odooData []OdooResPartnerItem
	if err := json.Unmarshal(resultBytes, &odooData); err != nil {
		id := fmt.Sprintf("⚠ Mohon maaf, terjadi kesalahan saat memproses data TID: *%s*.\n~_%v_", strings.Join(tid, ", "), err)
		en := fmt.Sprintf("⚠ Sorry, an error occurred while processing data for TID: *%s*.\n~_%v_", strings.Join(tid, ", "), err)
		sendLangMessageWithStanza(v, stanzaID, originalSenderJID, id, en, userLang)
		return
	}

	var sbID, sbEN strings.Builder
	sbID.WriteString(fmt.Sprintf("📝 Rincian dari TID(s) *%s*\n\n", strings.Join(tid, ", ")))
	sbEN.WriteString(fmt.Sprintf("📝 Details for TID(s) *%s*\n\n", strings.Join(tid, ", ")))

	for i, item := range odooData {
		sbID.WriteString(fmt.Sprintf("*%d*) %s: *%s* | %s\n", i+1, item.MIDTID.String, item.TIDMasterOld.String, item.MerchantName.String))
		sbEN.WriteString(fmt.Sprintf("*%d*) %s: *%s* | %s\n", i+1, item.MIDTID.String, item.TIDMasterOld.String, item.MerchantName.String))

		if item.Active.Bool {
			sbID.WriteString("Status: *Aktif*\n")
			sbEN.WriteString("Status: *Active*\n")
		} else {
			sbID.WriteString("Status: *Nonaktif*\n")
			sbEN.WriteString("Status: *Inactive*\n")
		}

		_, technician, err := parseJSONIDDataCombined(item.Technician)
		if err == nil && technician != "" {
			sbID.WriteString(fmt.Sprintf("Teknisi: %s\n", technician))
			sbEN.WriteString(fmt.Sprintf("Technician: %s\n", technician))
		}

		if item.MerchantPIC.String != "" {
			sbID.WriteString(fmt.Sprintf("PIC Merchant: %s\n", item.MerchantPIC.String))
			sbEN.WriteString(fmt.Sprintf("Merchant PIC: %s\n", item.MerchantPIC.String))
		}

		if item.MerchantPICPhoneNumber.String != "" {
			sbID.WriteString(fmt.Sprintf("No. HP PIC Merchant: %s\n", item.MerchantPICPhoneNumber.String))
			sbEN.WriteString(fmt.Sprintf("Merchant PIC Phone Number: %s\n", item.MerchantPICPhoneNumber.String))
		}

		var latitude, longitude *float64
		if item.GeoLatitude.Float != 0.0 {
			latitude = &item.GeoLatitude.Float
		}
		if item.GeoLongitude.Float != 0.0 {
			longitude = &item.GeoLongitude.Float
		}

		if latitude != nil && longitude != nil {
			googleMapsLink := fmt.Sprintf("https://www.google.com/maps/search/?api=1&query=%f,%f", *latitude, *longitude)
			sbID.WriteString(fmt.Sprintf("Lokasi Merchant: %s\n", googleMapsLink))
			sbEN.WriteString(fmt.Sprintf("Merchant Location: %s\n", googleMapsLink))
		}

		_, snEdc, err := parseJSONIDDataCombined(item.SNEDC)
		if err == nil && snEdc != "" {
			sbID.WriteString(fmt.Sprintf("SN EDC: %s\n", snEdc))
			sbEN.WriteString(fmt.Sprintf("EDC Serial Number: %s\n", snEdc))
		}

		_, tipeEdc, err := parseJSONIDDataCombined(item.TipeEDC)
		if err == nil && tipeEdc != "" {
			sbID.WriteString(fmt.Sprintf("Tipe EDC: %s\n", tipeEdc))
			sbEN.WriteString(fmt.Sprintf("EDC Type: %s\n", tipeEdc))
		}

		if item.AlamatPerusahaan.String != "" {
			sbID.WriteString(fmt.Sprintf("Alamat Merchant: %s\n", item.AlamatPerusahaan.String))
			sbEN.WriteString(fmt.Sprintf("Merchant Address: %s\n", item.AlamatPerusahaan.String))
		}

		// Add new line for separation
		sbID.WriteString("\n")
		sbEN.WriteString("\n")
	}

	// Send only one message after the loop
	idMsg := sbID.String()
	enMsg := sbEN.String()
	sendLangMessageWithStanza(v, stanzaID, originalSenderJID, idMsg, enMsg, userLang)
}
