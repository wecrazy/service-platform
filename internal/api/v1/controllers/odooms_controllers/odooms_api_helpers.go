package odoomscontrollers

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"service-platform/internal/config"
	odoomsmodel "service-platform/internal/core/model/odooms_model"
	tamodel "service-platform/internal/core/model/odooms_model/ta_model"
	"service-platform/internal/database"
	"service-platform/pkg/fun"
	"strings"
	"time"

	"github.com/sirupsen/logrus"
	"gorm.io/gorm"
)

// CheckExistingTechnicianInODOOMS checks if a technician exists in ODOOMS based on name, email, or phone number
func CheckExistingTechnicianInODOOMS(name, email, phoneNumber string) (bool, *odoomsmodel.ODOOMSTechnicianItem, error) {
	config.ManageService.MustInit("manage-service") // Load config manage-service.%s.yaml
	techExists, techData, err := CheckExistingTechnicianInODOOMSWithConfig(name, email, phoneNumber, config.ManageService.Get())
	return techExists, techData, err
}

// CheckExistingTechnicianInODOOMSWithConfig is the testable version that accepts config as parameter
func CheckExistingTechnicianInODOOMSWithConfig(name, email, phoneNumber string, cfg config.TypeManageService) (bool, *odoomsmodel.ODOOMSTechnicianItem, error) {
	if name == "" && email == "" && phoneNumber == "" {
		return false, nil, errors.New("at least one search parameter (name, email, or phoneNumber) must be provided")
	}

	odooDomain, err := buildODOOSearchDomain(name, email, phoneNumber)
	if err != nil {
		return false, nil, err
	}

	odooFields := []string{
		"id", "email", "password", "x_no_telp", "x_technician_name", "name",
		"x_spl_leader", "technician_code", "login_ids", "download_ids",
		"employee_ids", "create_date", "create_uid", "write_date", "write_uid",
		"job_group_id", "nik", "address", "area", "birth_status",
		"marriage_status", "payment_bank", "payment_bank_id", "payment_bank_name",
		"active", "x_employee_code", "technician_locations",
	}

	payload := map[string]any{
		"jsonrpc": cfg.ODOOMS.JSONRPCVersion,
		"params": map[string]any{
			"model":  "fs.technician",
			"domain": odooDomain,
			"fields": odooFields,
			"order":  "id desc",
		},
	}

	payloadBytes, err := json.Marshal(payload)
	if err != nil {
		return false, nil, err
	}

	url := cfg.ODOOMS.URL + cfg.ODOOMS.PathGetData
	tempHelper := NewODOOMSAPIHelper(&cfg, database.GetDBTA(), database.GetDBMS())

	sessionCookies, err := tempHelper.GetODOOMSCookies(cfg.ODOOMS.Login, cfg.ODOOMS.Password)
	if err != nil {
		return false, nil, fmt.Errorf("failed to get session cookies: %w", err)
	}

	maxRetries := cfg.ODOOMS.MaxRetry
	if maxRetries <= 0 {
		maxRetries = 3
	}
	retryDelay := cfg.ODOOMS.RetryDelay
	if retryDelay <= 0 {
		retryDelay = 3
	}
	reqTimeout := time.Duration(cfg.ODOOMS.DataTimeout) * time.Second
	if reqTimeout <= 0 {
		reqTimeout = 5 * time.Minute
	}
	tempHelper.client.Timeout = reqTimeout

	body, err := executeODOOMSRequest(tempHelper, "POST", url, payloadBytes, sessionCookies, maxRetries, retryDelay)
	if err != nil {
		return false, nil, err
	}

	item, err := parseODOOMSTechnicianResponse(body)
	if err != nil {
		return false, nil, err
	}
	return true, item, nil
}

// buildODOOSearchDomain builds the Odoo domain filter for technician search.
func buildODOOSearchDomain(name, email, phoneNumber string) ([]any, error) {
	conditions := []any{}
	if phoneNumber != "" {
		conditions = append(conditions, []any{"x_no_telp", "ilike", phoneNumber})
	}
	if name != "" {
		conditions = append(conditions, []any{"x_technician_name", "ilike", name})
	}
	if email != "" {
		conditions = append(conditions, []any{"email", "ilike", email})
	}
	if len(conditions) == 0 {
		return nil, errors.New("at least one search parameter (name, email, or phoneNumber) must be provided")
	}
	domain := []any{}
	for i := 0; i < len(conditions)-1; i++ {
		domain = append(domain, "|")
	}
	domain = append(domain, conditions...)
	return domain, nil
}

// executeODOOMSRequest sends payloadBytes to url with retries and returns the raw response body.
func executeODOOMSRequest(h *ODOOMSAPIHelper, method, url string, payloadBytes []byte, sessionCookies []*http.Cookie, maxRetries, retryDelay int) ([]byte, error) {
	var lastErr error
	for attempt := 1; attempt <= maxRetries; attempt++ {
		req, err := http.NewRequest(method, url, bytes.NewBuffer(payloadBytes))
		if err != nil {
			return nil, fmt.Errorf("failed to create request: %w", err)
		}
		req.Header.Set("Content-Type", "application/json")
		for _, c := range sessionCookies {
			req.AddCookie(c)
		}

		resp, err := h.client.Do(req)
		if err != nil {
			logrus.WithError(err).Warningf("Request failed (attempt %d/%d)", attempt, maxRetries)
			lastErr = err
			if attempt < maxRetries {
				time.Sleep(time.Duration(retryDelay) * time.Second)
			}
			continue
		}
		defer resp.Body.Close()

		body, err := io.ReadAll(resp.Body)
		if err != nil {
			return nil, fmt.Errorf("failed to read response body: %w", err)
		}
		if resp.StatusCode != http.StatusOK {
			logrus.WithField("status", resp.StatusCode).Warningf("Bad response status: %d (attempt %d/%d)", resp.StatusCode, attempt, maxRetries)
			if attempt < maxRetries {
				time.Sleep(time.Duration(retryDelay) * time.Second)
				continue
			}
			return nil, fmt.Errorf("request failed with status: %d", resp.StatusCode)
		}
		return body, nil
	}
	return nil, fmt.Errorf("all retry attempts failed, last error: %w", lastErr)
}

// parseODOOMSTechnicianResponse parses the JSON body returned by ODOOMS into ODOOMSTechnicianItem.
func parseODOOMSTechnicianResponse(body []byte) (*odoomsmodel.ODOOMSTechnicianItem, error) {
	var jsonResponse map[string]any
	if err := json.Unmarshal(body, &jsonResponse); err != nil {
		return nil, err
	}
	if errorResponse, ok := jsonResponse["error"].(map[string]any); ok {
		if msg, ok := errorResponse["message"].(string); ok && msg == "Odoo Session Expired" {
			return nil, errors.New("odoo Session Expired")
		}
		return nil, fmt.Errorf("odoo Error: %v", errorResponse)
	}
	if result, ok := jsonResponse["result"].(map[string]any); ok {
		if message, ok := result["message"].(string); ok {
			success, successOk := result["success"]
			logrus.Infof("ODOO MS Result, message: %v, status: %v", message, successOk && success == true)
		}
	}

	resultRaw, ok := jsonResponse["result"]
	if !ok {
		return nil, fmt.Errorf("'result' field not found in the response: %v", jsonResponse)
	}

	resultArray, err := extractODOOMSResultArray(resultRaw)
	if err != nil {
		return nil, err
	}
	if len(resultArray) == 0 {
		return nil, fmt.Errorf("'result' array is empty: %v", resultArray)
	}

	itemMap, ok := resultArray[0].(map[string]any)
	if !ok {
		return nil, fmt.Errorf("first item is not a map: %v", resultArray[0])
	}

	jsonData, err := json.Marshal(itemMap)
	if err != nil {
		return nil, fmt.Errorf("error marshalling first item: %v", err)
	}
	var odooData odoomsmodel.ODOOMSTechnicianItem
	if err := json.Unmarshal(jsonData, &odooData); err != nil {
		return nil, fmt.Errorf("error unmarshalling first item: %v", err)
	}
	return &odooData, nil
}

// extractODOOMSResultArray converts the "result" field (map or slice) into a []any.
func extractODOOMSResultArray(result any) ([]any, error) {
	switch v := result.(type) {
	case map[string]any:
		data, ok := v["data"]
		if !ok {
			return nil, fmt.Errorf("'data' field not found in result map: %v", v)
		}
		arr, ok := data.([]any)
		if !ok {
			return nil, fmt.Errorf("'data' field is not an array: %v", data)
		}
		return arr, nil
	case []any:
		return v, nil
	default:
		return nil, fmt.Errorf("'result' is neither a map nor an array: %v", result)
	}
}

// CheckWONumberStatusInTAMS checks the WO number status in the TAMS system.
func (h *ODOOMSAPIHelper) CheckWONumberStatusInTAMS(woNumber string) map[string]string {
	config.ManageService.MustInit("manage-service") // Load config manage-service.%s.yaml

	langMsg := make(map[string]string)
	messages := buildWOLocalizedMessages()
	getMsg := func(lang, key string) string {
		if langMap, ok := messages[lang]; ok {
			if msg, ok := langMap[key]; ok {
				return msg
			}
		}
		return key
	}

	if woNumber == "" {
		for _, lang := range woSupportedLangs() {
			langMsg[lang] = getMsg(lang, "wo_empty")
		}
		return langMsg
	}

	if h.dbTA == nil {
		logrus.Error("Database connection for TA is not initialized")
		for _, lang := range woSupportedLangs() {
			langMsg[lang] = getMsg(lang, "db_not_init")
		}
		return langMsg
	}

	woNumber = strings.TrimSpace(strings.ToUpper(woNumber))
	resultData, dbErr := findWOInTADatabases(h.dbMSMiddleware, woNumber)
	if dbErr != nil {
		for _, lang := range woSupportedLangs() {
			langMsg[lang] = fmt.Sprintf(getMsg(lang, "db_error"), dbErr)
		}
		return langMsg
	}

	for _, lang := range woSupportedLangs() {
		if resultData == nil {
			langMsg[lang] = fmt.Sprintf(getMsg(lang, "wo_not_found"), woNumber)
			continue
		}
		langMsg[lang] = buildWOResultMessage(lang, resultData, getMsg)
	}
	return langMsg
}

// woResultData holds normalised data found from one of the TA database tables.
type woResultData struct {
	Type     string // "log", "pending", or "error"
	WoNumber string
	Method   string
	Email    string
	DateIn   string
	Date     *time.Time
	Feedback string
	Problem  *string
}

// woSupportedLangs returns the slice of language codes supported by CheckWONumberStatusInTAMS.
func woSupportedLangs() []string {
	return []string{fun.LangID, fun.LangEN, fun.LangES, fun.LangFR, fun.LangDE, fun.LangPT, fun.LangRU, fun.LangJP, fun.LangCN, fun.LangAR}
}

// findWOInTADatabases searches LogAct, Pending, and Error tables for the given WO number.
// Returns (data, nil) when found, (nil, nil) when not found, or (nil, err) on DB error.
func findWOInTADatabases(db *gorm.DB, woNumber string) (*woResultData, error) {
	// 1. LogAct
	var taLog tamodel.LogAct
	tx := db.Where("wo LIKE ?", "%"+woNumber+"%").First(&taLog)
	if tx.Error == nil && strings.ToLower(taLog.Method) != "" {
		d := &woResultData{Type: "log", Method: strings.ToLower(taLog.Method), Email: taLog.Email, DateIn: taLog.DateInDashboard, Date: taLog.Date, Feedback: taLog.TaFeedback}
		if taLog.Wo != nil && *taLog.Wo != "" {
			d.WoNumber = *taLog.Wo
		} else {
			d.WoNumber = woNumber
		}
		return d, nil
	}
	if tx.Error != nil && !errors.Is(tx.Error, gorm.ErrRecordNotFound) {
		return nil, tx.Error
	}

	// 2. Pending
	var taPending tamodel.Pending
	tx = db.Where("wo LIKE ?", "%"+woNumber+"%").First(&taPending)
	if tx.Error == nil && taPending.WoNumber != "" {
		return &woResultData{Type: "pending", WoNumber: taPending.WoNumber, Date: &taPending.Date, Feedback: taPending.TaFeedback}, nil
	}
	if tx.Error != nil && !errors.Is(tx.Error, gorm.ErrRecordNotFound) {
		return nil, tx.Error
	}

	// 3. Error
	var taError tamodel.Error
	tx = db.Where("wo LIKE ?", "%"+woNumber+"%").First(&taError)
	if tx.Error == nil && taError.WoNumber != "" {
		return &woResultData{Type: "error", WoNumber: taError.WoNumber, Date: &taError.Date, Problem: taError.Problem, Feedback: taError.TaFeedback}, nil
	}
	if tx.Error != nil && !errors.Is(tx.Error, gorm.ErrRecordNotFound) {
		return nil, tx.Error
	}

	return nil, nil // not found
}

// buildWOResultMessage produces the localized message string for one language.
func buildWOResultMessage(lang string, d *woResultData, getMsg func(lang, key string) string) string {
	switch d.Type {
	case "log":
		return buildWOLogMessage(lang, d, getMsg)
	case "pending":
		return buildWOPendingMessage(lang, d, getMsg)
	case "error":
		return buildWOErrorMessage(lang, d, getMsg)
	}
	return ""
}

func buildWOLogMessage(lang string, d *woResultData, getMsg func(lang, key string) string) string {
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf(getMsg(lang, "wo_details_header"), d.WoNumber))
	if d.Email != "" {
		DataTA := config.ManageService.Get().TechnicalAssistance.UserTA
		if ta, ok := DataTA[d.Email]; ok {
			taName := ta.Name
			if taName == "" {
				taName = "N/A"
			}
			action := woTAAction(lang, d.Method, getMsg)
			sb.WriteString(fmt.Sprintf("\nTA: <i>%s</i> %s", taName, action))
		}
	}
	if d.DateIn != "" {
		sb.WriteString(fmt.Sprintf(getMsg(lang, "entered_dashboard"), d.DateIn))
	}
	if d.Date != nil && !d.Date.IsZero() {
		sb.WriteString(fmt.Sprintf(getMsg(lang, "last_checked"), d.Date.Format("2006-01-02 15:04:05")))
	}
	if d.Feedback != "" {
		sb.WriteString(fmt.Sprintf(getMsg(lang, "feedback"), d.Feedback))
	}
	return sb.String()
}

func woTAAction(lang, method string, getMsg func(lang, key string) string) string {
	switch method {
	case "submit":
		return getMsg(lang, "ta_action_submit")
	case "edit":
		return getMsg(lang, "ta_action_edit")
	case "delete":
		return getMsg(lang, "ta_action_delete")
	}
	return ""
}

func buildWOPendingMessage(lang string, d *woResultData, getMsg func(lang, key string) string) string {
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf(getMsg(lang, "pending_details"), d.WoNumber))
	if d.Date != nil && !d.Date.IsZero() {
		sb.WriteString(fmt.Sprintf(getMsg(lang, "entered_dashboard"), d.Date.Format("2006-01-02 15:04:05")))
	}
	if d.Feedback != "" {
		sb.WriteString(fmt.Sprintf(getMsg(lang, "feedback"), d.Feedback))
	}
	return sb.String()
}

func buildWOErrorMessage(lang string, d *woResultData, getMsg func(lang, key string) string) string {
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf(getMsg(lang, "error_details"), d.WoNumber))
	if d.Date != nil && !d.Date.IsZero() {
		sb.WriteString(fmt.Sprintf(getMsg(lang, "entered_dashboard"), d.Date.Format("2006-01-02 15:04:05")))
	}
	if d.Problem != nil && *d.Problem != "" {
		sb.WriteString(fmt.Sprintf(getMsg(lang, "problem"), *d.Problem))
	}
	if d.Feedback != "" {
		sb.WriteString(fmt.Sprintf(getMsg(lang, "feedback"), d.Feedback))
	}
	return sb.String()
}

// buildWOLocalizedMessages returns the full map of localized message templates used by CheckWONumberStatusInTAMS.
func buildWOLocalizedMessages() map[string]map[string]string {
	return map[string]map[string]string{
		fun.LangID: {
			"wo_details_header": "📌 Rincian WO Number: <b>%s</b>\n",
			"entered_dashboard": "\nMasuk di Dashboard TA per %s",
			"last_checked":      "\nTA terakhir melakukan pengecekan pada %v",
			"feedback":          "\nFeedback dari TA: %s",
			"ta_action_submit":  "telah mengecek dan meng-submit data ke ODOO.",
			"ta_action_edit":    "telah melakukan perubahan data yang telah dikerjakan dan sudah diupload ke ODOO.",
			"ta_action_delete":  "telah menghapus data dari dashboard TA.",
			"wo_not_found":      "🔍 Data WO Number <b>%s</b> tidak ditemukan di database TA (Log, Pending, atau Error). Mungkin belum masuk di dashboard TA atau datanya sudah berhasil dikerjakan.\nKami akan coba mencari statusnya di ODOO, mohon bersabar 🙏🏼",
			"db_error":          "❗️Gagal mencari data TA: %v",
			"pending_details":   "🟡 Rincian WO Number: <b>%s</b>\n",
			"error_details":     "🔴 Rincian WO Number: <b>%s</b>\n",
			"problem":           "\nMasalah: %s",
			"wo_empty":          "Nomor WO tidak boleh kosong.",
			"db_not_init":       "Koneksi database untuk TA tidak terinisialisasi.",
		},
		fun.LangEN: {
			"wo_details_header": "📌 Details for WO Number: <b>%s</b>\n",
			"entered_dashboard": "\nEntered in TA Dashboard on %s",
			"last_checked":      "\nLast checked by TA on %v",
			"feedback":          "\nFeedback from TA: %s",
			"ta_action_submit":  "has checked and submitted the data to ODOO.",
			"ta_action_edit":    "has modified the processed data and uploaded it to ODOO.",
			"ta_action_delete":  "has deleted the data from the TA dashboard.",
			"wo_not_found":      "🔍 WO Number <b>%s</b> was not found in the TA database (Log, Pending, or Error). It may not have been entered into the TA dashboard yet or may have already been processed.\nWe will try to check its status in ODOO, please be patient 🙏🏼",
			"db_error":          "❗️Failed to search TA data: %v",
			"pending_details":   "🟡 Details for WO Number: <b>%s</b>\n",
			"error_details":     "🔴 Details for WO Number: <b>%s</b>\n",
			"problem":           "\nProblem: %s",
			"wo_empty":          "WO number cannot be empty.",
			"db_not_init":       "Database connection for TA is not initialized.",
		},
		fun.LangES: {
			"wo_details_header": "📌 Detalles del Número de WO: <b>%s</b>\n",
			"entered_dashboard": "\nIngresado en el Dashboard de TA el %s",
			"last_checked":      "\nÚltima verificación por TA en %v",
			"feedback":          "\nComentarios de TA: %s",
			"ta_action_submit":  "ha verificado y enviado los datos a ODOO.",
			"ta_action_edit":    "ha modificado los datos procesados y los ha subido a ODOO.",
			"ta_action_delete":  "ha eliminado los datos del dashboard de TA.",
			"wo_not_found":      "🔍 El Número de WO <b>%s</b> no se encontró en la base de datos de TA (Log, Pending o Error). Puede que aún no haya sido ingresado en el dashboard de TA o ya haya sido procesado.\nIntentaremos verificar su estado en ODOO, por favor sea paciente 🙏🏼",
			"db_error":          "❗️Error al buscar datos de TA: %v",
			"pending_details":   "🟡 Detalles del Número de WO: <b>%s</b>\n",
			"error_details":     "🔴 Detalles del Número de WO: <b>%s</b>\n",
			"problem":           "\nProblema: %s",
			"wo_empty":          "El número de WO no puede estar vacío.",
			"db_not_init":       "La conexión de base de datos para TA no está inicializada.",
		},
		fun.LangFR: {
			"wo_details_header": "📌 Détails du numéro WO : <b>%s</b>\n",
			"entered_dashboard": "\nEntré dans le tableau de bord TA le %s",
			"last_checked":      "\nDernière vérification par TA le %v",
			"feedback":          "\nCommentaires de TA : %s",
			"ta_action_submit":  "a vérifié et soumis les données à ODOO.",
			"ta_action_edit":    "a modifié les données traitées et les a téléchargées vers ODOO.",
			"ta_action_delete":  "a supprimé les données du tableau de bord TA.",
			"wo_not_found":      "🔍 Le numéro WO <b>%s</b> n'a pas été trouvé dans la base de données TA (Log, Pending ou Error). Il se peut qu'il n'ait pas encore été saisi dans le tableau de bord TA ou qu'il ait déjà été traité.\nNous allons essayer de vérifier son statut dans ODOO, veuillez patienter 🙏🏼",
			"db_error":          "❗️Échec de la recherche des données TA : %v",
			"pending_details":   "🟡 Détails du numéro WO : <b>%s</b>\n",
			"error_details":     "🔴 Détails du numéro WO : <b>%s</b>\n",
			"problem":           "\nProblème : %s",
			"wo_empty":          "Le numéro de WO ne peut pas être vide.",
			"db_not_init":       "La connexion à la base de données pour TA n'est pas initialisée.",
		},
		fun.LangDE: {
			"wo_details_header": "📌 Details zur WO-Nummer: <b>%s</b>\n",
			"entered_dashboard": "\nEingegeben im TA-Dashboard am %s",
			"last_checked":      "\nZuletzt überprüft von TA am %v",
			"feedback":          "\nFeedback von TA: %s",
			"ta_action_submit":  "hat die Daten überprüft und an ODOO übermittelt.",
			"ta_action_edit":    "hat die bearbeiteten Daten geändert und zu ODOO hochgeladen.",
			"ta_action_delete":  "hat die Daten aus dem TA-Dashboard gelöscht.",
			"wo_not_found":      "🔍 Die WO-Nummer <b>%s</b> wurde in der TA-Datenbank (Log, Pending oder Error) nicht gefunden. Möglicherweise wurde sie noch nicht in das TA-Dashboard eingegeben oder bereits bearbeitet.\nWir werden versuchen, den Status in ODOO zu überprüfen, bitte haben Sie Geduld 🙏🏼",
			"db_error":          "❗️Fehler beim Suchen der TA-Daten: %v",
			"pending_details":   "🟡 Details zur WO-Nummer: <b>%s</b>\n",
			"error_details":     "🔴 Details zur WO-Nummer: <b>%s</b>\n",
			"problem":           "\nProblem: %s",
			"wo_empty":          "WO-Nummer darf nicht leer sein.",
			"db_not_init":       "Datenbankverbindung für TA ist nicht initialisiert.",
		},
		fun.LangPT: {
			"wo_details_header": "📌 Detalhes do Número WO: <b>%s</b>\n",
			"entered_dashboard": "\nInserido no Dashboard TA em %s",
			"last_checked":      "\nÚltima verificação por TA em %v",
			"feedback":          "\nComentários de TA: %s",
			"ta_action_submit":  "verificado e submeteu os dados para ODOO.",
			"ta_action_edit":    "modificou os dados processados e os enviou para ODOO.",
			"ta_action_delete":  "removido os dados do dashboard TA.",
			"wo_not_found":      "🔍 O Número WO <b>%s</b> não foi encontrado na base de dados TA (Log, Pending ou Error). Pode ainda não ter sido inserido no dashboard TA ou já ter sido processado.\nVamos tentar verificar o seu status no ODOO, por favor aguarde 🙏🏼",
			"db_error":          "❗️Falha ao procurar dados TA: %v",
			"pending_details":   "🟡 Detalhes do Número WO: <b>%s</b>\n",
			"error_details":     "🔴 Detalhes do Número WO: <b>%s</b>\n",
			"problem":           "\nProblema: %s",
			"wo_empty":          "O número do WO não pode estar vazio.",
			"db_not_init":       "A conexão com o banco de dados para TA não está inicializada.",
		},
		fun.LangRU: {
			"wo_details_header": "📌 Детали номера WO: <b>%s</b>\n",
			"entered_dashboard": "\nВведено в панель TA %s",
			"last_checked":      "\nПоследняя проверка TA %v",
			"feedback":          "\nОтзыв от TA: %s",
			"ta_action_submit":  "проверил и отправил данные в ODOO.",
			"ta_action_edit":    "изменил обработанные данные и загрузил их в ODOO.",
			"ta_action_delete":  "удалил данные из панели TA.",
			"wo_not_found":      "🔍 Номер WO <b>%s</b> не найден в базе данных TA (Log, Pending или Error). Возможно, он еще не введен в панель TA или уже обработан.\nМы попробуем проверить его статус в ODOO, пожалуйста, подождите 🙏🏼",
			"db_error":          "❗️Не удалось найти данные TA: %v",
			"pending_details":   "🟡 Детали номера WO: <b>%s</b>\n",
			"error_details":     "🔴 Детали номера WO: <b>%s</b>\n",
			"problem":           "\nПроблема: %s",
			"wo_empty":          "Номер WO не может быть пустым.",
			"db_not_init":       "Соединение с базой данных для TA не инициализировано.",
		},
		fun.LangJP: {
			"wo_details_header": "📌 WO番号の詳細: <b>%s</b>\n",
			"entered_dashboard": "\nTAダッシュボードに入力日 %s",
			"last_checked":      "\nTAによる最終確認 %v",
			"feedback":          "\nTAからのフィードバック: %s",
			"ta_action_submit":  "データを確認し、ODOOに送信しました。",
			"ta_action_edit":    "処理済みデータを変更し、ODOOにアップロードしました。",
			"ta_action_delete":  "TAダッシュボードからデータを削除しました。",
			"wo_not_found":      "🔍 WO番号 <b>%s</b> がTAデータベース (Log、Pending、またはError) に見つかりません。TAダッシュボードにまだ入力されていないか、すでに処理されている可能性があります。\nODOOでステータスを確認してみますので、お待ちください 🙏🏼",
			"db_error":          "❗️TAデータの検索に失敗しました: %v",
			"pending_details":   "🟡 WO番号の詳細: <b>%s</b>\n",
			"error_details":     "🔴 WO番号の詳細: <b>%s</b>\n",
			"problem":           "\n問題: %s",
			"wo_empty":          "WO番号は空にできません。",
			"db_not_init":       "TAのデータベース接続が初期化されていません。",
		},
		fun.LangCN: {
			"wo_details_header": "📌 工单编号详情: <b>%s</b>\n",
			"entered_dashboard": "\n于 %s 输入TA仪表板",
			"last_checked":      "\nTA最后检查时间 %v",
			"feedback":          "\nTA反馈: %s",
			"ta_action_submit":  "已检查并将数据提交到ODOO。",
			"ta_action_edit":    "已修改已处理数据并上传到ODOO。",
			"ta_action_delete":  "已从TA仪表板删除数据。",
			"wo_not_found":      "🔍 在TA数据库 (Log、Pending 或 Error) 中未找到工单编号 <b>%s</b>。可能尚未输入TA仪表板或已处理。\n我们将尝试在ODOO中检查其状态，请耐心等待 🙏🏼",
			"db_error":          "❗️搜索TA数据失败: %v",
			"pending_details":   "🟡 工单编号详情: <b>%s</b>\n",
			"error_details":     "🔴 工单编号详情: <b>%s</b>\n",
			"problem":           "\n问题: %s",
			"wo_empty":          "工单编号不能为空。",
			"db_not_init":       "TA的数据库连接未初始化。",
		},
		fun.LangAR: {
			"wo_details_header": "📌 تفاصيل رقم WO: <b>%s</b>\n",
			"entered_dashboard": "\nتم إدخاله في لوحة تحكم TA في %s",
			"last_checked":      "\nآخر فحص بواسطة TA في %v",
			"feedback":          "\nتعليقات TA: %s",
			"ta_action_submit":  "تم التحقق وإرسال البيانات إلى ODOO.",
			"ta_action_edit":    "تم تعديل البيانات المعالجة وتحميلها إلى ODOO.",
			"ta_action_delete":  "تم حذف البيانات من لوحة تحكم TA.",
			"wo_not_found":      "🔍 لم يتم العثور على رقم WO <b>%s</b> في قاعدة بيانات TA (Log أو Pending أو Error). قد لم يتم إدخاله بعد في لوحة تحكم TA أو تم معالجته بالفعل.\nسنحاول التحقق من حالته في ODOO، يرجى الانتظار 🙏🏼",
			"db_error":          "❗️فشل في البحث عن بيانات TA: %v",
			"pending_details":   "🟡 تفاصيل رقم WO: <b>%s</b>\n",
			"error_details":     "🔴 تفاصيل رقم WO: <b>%s</b>\n",
			"problem":           "\nالمشكلة: %s",
			"wo_empty":          "لا يمكن أن يكون رقم WO فارغًا.",
			"db_not_init":       "اتصال قاعدة البيانات لـ TA غير مهيأ.",
		},
	}
}
