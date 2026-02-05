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
	"service-platform/internal/pkg/fun"
	"strings"
	"time"

	"github.com/sirupsen/logrus"
	"gorm.io/gorm"
)

// CheckExistingTechnicianInODOOMS checks if a technician exists in ODOOMS based on name, email, or phone number
func CheckExistingTechnicianInODOOMS(name, email, phoneNumber string) (bool, *odoomsmodel.ODOOMSTechnicianItem, error) {
	techExists, techData, err := CheckExistingTechnicianInODOOMSWithConfig(name, email, phoneNumber, config.GetConfig())
	return techExists, techData, err
}

// checkExistingTechnicianInODOOMSWithConfig is the testable version that accepts config as parameter
func CheckExistingTechnicianInODOOMSWithConfig(name, email, phoneNumber string, cfg config.TypeConfig) (bool, *odoomsmodel.ODOOMSTechnicianItem, error) {
	if name == "" && email == "" && phoneNumber == "" {
		return false, nil, errors.New("at least one search parameter (name, email, or phoneNumber) must be provided")
	}

	odooModel := "fs.technician"

	conditions := []any{}

	if phoneNumber != "" {
		conditions = append(conditions,
			[]any{"x_no_telp", "ilike", phoneNumber},
		)
	}

	if name != "" {
		conditions = append(conditions,
			[]any{"x_technician_name", "ilike", name},
		)
	}

	if email != "" {
		conditions = append(conditions,
			[]any{"email", "ilike", email},
		)
	}

	odooDomain := []any{}

	if len(conditions) > 0 {
		// Add N-1 "|" operators
		for i := 0; i < len(conditions)-1; i++ {
			odooDomain = append(odooDomain, "|")
		}

		// Append all conditions
		odooDomain = append(odooDomain, conditions...)
	} else {
		// No search conditions provided
		return false, nil, errors.New("at least one search parameter (name, email, or phoneNumber) must be provided")
	}

	odooFields := []string{
		"id",
		"email",
		"password",
		"x_no_telp",
		"x_technician_name",
		"name",
		"x_spl_leader",
		"technician_code",
		"login_ids",
		"download_ids",
		"employee_ids",
		"create_date",
		"create_uid",
		"write_date",
		"write_uid",
		"job_group_id",
		"nik",
		"address",
		"area",
		"birth_status",
		"marriage_status",
		"payment_bank",
		"payment_bank_id",
		"payment_bank_name",
		"active",
		"x_employee_code",
		"technician_locations",
	}

	odooOrder := "id desc"

	odooParams := map[string]any{
		"model":  odooModel,
		"domain": odooDomain,
		"fields": odooFields,
		"order":  odooOrder,
	}

	payload := map[string]any{
		"jsonrpc": cfg.ODOOManageService.JsonRPCVersion,
		"params":  odooParams,
	}

	payloadBytes, err := json.Marshal(payload)
	if err != nil {
		return false, nil, err
	}

	methodUsed := "POST"
	url := cfg.ODOOManageService.URL + cfg.ODOOManageService.PathGetData

	// Create a temporary helper for this request
	tempHelper := NewODOOMSAPIHelper(&cfg, database.GetDBTA(), database.GetDBMS())

	// Get session cookies
	sessionCookies, err := tempHelper.GetODOOMSCookies(cfg.ODOOManageService.Login, cfg.ODOOManageService.Password)
	if err != nil {
		return false, nil, fmt.Errorf("failed to get session cookies: %w", err)
	}

	// Make the request directly instead of using FetchODOOMS
	maxRetries := cfg.ODOOManageService.MaxRetry
	if maxRetries <= 0 {
		maxRetries = 3
	}

	retryDelay := cfg.ODOOManageService.RetryDelay
	if retryDelay <= 0 {
		retryDelay = 3
	}

	reqTimeout := time.Duration(cfg.ODOOManageService.DataTimeout) * time.Second
	if reqTimeout <= 0 {
		reqTimeout = 5 * time.Minute // 300 seconds default
	}

	tempHelper.client.Timeout = reqTimeout

	var lastErr error
	for attempts := 1; attempts <= maxRetries; attempts++ {
		request, err := http.NewRequest(methodUsed, url, bytes.NewBuffer(payloadBytes))
		if err != nil {
			return false, nil, fmt.Errorf("failed to create request: %w", err)
		}
		request.Header.Set("Content-Type", "application/json")

		// Add session cookies to request
		for _, cookie := range sessionCookies {
			request.AddCookie(cookie)
		}

		response, err := tempHelper.client.Do(request)
		if err != nil {
			logrus.WithError(err).Warningf("Request failed (attempt %d/%d)", attempts, maxRetries)
			lastErr = err
			if attempts < maxRetries {
				time.Sleep(time.Duration(retryDelay) * time.Second)
			}
			continue
		}

		defer response.Body.Close()

		body, err := io.ReadAll(response.Body)
		if err != nil {
			return false, nil, fmt.Errorf("failed to read response body: %w", err)
		}

		if response.StatusCode != http.StatusOK {
			logrus.WithField("status", response.StatusCode).Warningf("Bad response status: %d (attempt %d/%d)", response.StatusCode, attempts, maxRetries)
			if attempts < maxRetries {
				time.Sleep(time.Duration(retryDelay) * time.Second)
				continue
			}
			return false, nil, fmt.Errorf("request failed with status: %d", response.StatusCode)
		}

		// Parse response
		var jsonResponse map[string]any
		err = json.Unmarshal(body, &jsonResponse)
		if err != nil {
			return false, nil, err
		}

		if errorResponse, ok := jsonResponse["error"].(map[string]any); ok {
			if errorMessage, ok := errorResponse["message"].(string); ok && errorMessage == "Odoo Session Expired" {
				return false, nil, errors.New("odoo Session Expired")
			} else {
				return false, nil, fmt.Errorf("odoo Error: %v", errorResponse)
			}
		}

		if result, ok := jsonResponse["result"].(map[string]any); ok {
			if message, ok := result["message"].(string); ok {
				success, successOk := result["success"]
				logrus.Infof("ODOO MS Result, message: %v, status: %v", message, successOk && success == true)
			}
		}

		// Check for the existence and validity of the "result" field
		result, resultExists := jsonResponse["result"]
		if !resultExists {
			return false, nil, fmt.Errorf("'result' field not found in the response: %v", jsonResponse)
		}

		// Check if the result is a map (with data field) or an array directly
		var resultArray []any
		if resultMap, ok := result.(map[string]any); ok {
			// Result is a map, check for data field
			if data, dataExists := resultMap["data"]; dataExists {
				if arr, arrOk := data.([]any); arrOk {
					resultArray = arr
				} else {
					return false, nil, fmt.Errorf("'data' field is not an array: %v", data)
				}
			} else {
				return false, nil, fmt.Errorf("'data' field not found in result map: %v", resultMap)
			}
		} else if arr, ok := result.([]any); ok {
			// Result is directly an array
			resultArray = arr
		} else {
			return false, nil, fmt.Errorf("'result' is neither a map nor an array: %v", result)
		}

		// Ensure the array is not empty
		if len(resultArray) == 0 {
			return false, nil, fmt.Errorf("'result' array is empty: %v", resultArray)
		}

		// Take only the first item
		firstItem := resultArray[0]

		// Check that the first item is a map
		itemMap, ok := firstItem.(map[string]any)
		if !ok {
			return false, nil, fmt.Errorf("first item is not a map: %v", firstItem)
		}

		// Parse into struct
		var odooData odoomsmodel.ODOOMSTechnicianItem
		jsonData, err := json.Marshal(itemMap)
		if err != nil {
			return false, nil, fmt.Errorf("error marshalling first item: %v", err)
		}

		err = json.Unmarshal(jsonData, &odooData)
		if err != nil {
			return false, nil, fmt.Errorf("error unmarshalling first item: %v", err)
		}

		return true, &odooData, nil
	}

	return false, nil, fmt.Errorf("all retry attempts failed, last error: %w", lastErr)
}

func (h *ODOOMSAPIHelper) CheckWONumberStatusInTAMS(woNumber string) map[string]string {
	langMsg := make(map[string]string)

	// Localized messages
	messages := map[string]map[string]string{
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
		},
	}

	// Helper function to get localized message
	getLocalizedMessage := func(lang, key string) string {
		if langMap, ok := messages[lang]; ok {
			if msg, ok := langMap[key]; ok {
				return msg
			}
		}
		return key // Fallback
	}

	if woNumber == "" {
		langMsg[fun.LangID] = "Nomor WO tidak boleh kosong."
		langMsg[fun.LangEN] = "WO number cannot be empty."
		langMsg[fun.LangES] = "El número de WO no puede estar vacío."
		langMsg[fun.LangFR] = "Le numéro de WO ne peut pas être vide."
		langMsg[fun.LangDE] = "WO-Nummer darf nicht leer sein."
		langMsg[fun.LangPT] = "O número do WO não pode estar vazio."
		langMsg[fun.LangRU] = "Номер WO не может быть пустым."
		langMsg[fun.LangJP] = "WO番号は空にできません。"
		langMsg[fun.LangCN] = "工单编号不能为空。"
		langMsg[fun.LangAR] = "لا يمكن أن يكون رقم WO فارغًا."
		return langMsg
	}

	if h.dbTA == nil {
		logrus.Error("Database connection for TA is not initialized")
		langMsg[fun.LangID] = "Koneksi database untuk TA tidak terinisialisasi."
		langMsg[fun.LangEN] = "Database connection for TA is not initialized."
		langMsg[fun.LangES] = "La conexión de base de datos para TA no está inicializada."
		langMsg[fun.LangFR] = "La connexion à la base de données pour TA n'est pas initialisée."
		langMsg[fun.LangDE] = "Datenbankverbindung für TA ist nicht initialisiert."
		langMsg[fun.LangPT] = "A conexão com o banco de dados para TA não está inicializada."
		langMsg[fun.LangRU] = "Соединение с базой данных для TA не инициализировано."
		langMsg[fun.LangJP] = "TAのデータベース接続が初期化されていません。"
		langMsg[fun.LangCN] = "TA的数据库连接未初始化。"
		langMsg[fun.LangAR] = "اتصال قاعدة البيانات لـ TA غير مهيأ."
		return langMsg
	}

	woNumber = strings.TrimSpace(strings.ToUpper(woNumber))

	// Build messages for each language
	supportedLangs := []string{fun.LangID, fun.LangEN, fun.LangES, fun.LangFR, fun.LangDE, fun.LangPT, fun.LangRU, fun.LangJP, fun.LangCN, fun.LangAR}

	found := false
	var resultData struct {
		Type     string
		WoNumber string
		Method   string
		Email    string
		DateIn   string
		Date     *time.Time
		Feedback string
		Problem  *string
	}

	// 1. Try from LogAct
	var taLog tamodel.LogAct
	tx := h.dbMSMiddleware.Where("wo LIKE ?", "%"+woNumber+"%").First(&taLog)
	if tx.Error == nil && strings.ToLower(taLog.Method) != "" {
		found = true
		resultData.Type = "log"
		if taLog.Wo != nil && *taLog.Wo != "" {
			resultData.WoNumber = *taLog.Wo
		} else {
			resultData.WoNumber = woNumber
		}
		resultData.Method = strings.ToLower(taLog.Method)
		resultData.Email = taLog.Email
		resultData.DateIn = taLog.DateInDashboard
		resultData.Date = taLog.Date
		resultData.Feedback = taLog.TaFeedback
	} else if tx.Error != nil && !errors.Is(tx.Error, gorm.ErrRecordNotFound) {
		for _, lang := range supportedLangs {
			langMsg[lang] = fmt.Sprintf(getLocalizedMessage(lang, "db_error"), tx.Error)
		}
		return langMsg
	}

	// 2. Try from Pending
	if !found {
		var taPending tamodel.Pending
		tx = h.dbMSMiddleware.Where("wo LIKE ?", "%"+woNumber+"%").First(&taPending)
		if tx.Error == nil && taPending.WoNumber != "" {
			found = true
			resultData.Type = "pending"
			resultData.WoNumber = taPending.WoNumber
			resultData.Date = &taPending.Date
			resultData.Feedback = taPending.TaFeedback
		} else if tx.Error != nil && !errors.Is(tx.Error, gorm.ErrRecordNotFound) {
			for _, lang := range supportedLangs {
				langMsg[lang] = fmt.Sprintf(getLocalizedMessage(lang, "db_error"), tx.Error)
			}
			return langMsg
		}
	}

	// 3. Try from Error
	if !found {
		var taError tamodel.Error
		tx = h.dbMSMiddleware.Where("wo LIKE ?", "%"+woNumber+"%").First(&taError)
		if tx.Error == nil && taError.WoNumber != "" {
			found = true
			resultData.Type = "error"
			resultData.WoNumber = taError.WoNumber
			resultData.Date = &taError.Date
			resultData.Problem = taError.Problem
			resultData.Feedback = taError.TaFeedback
		} else if tx.Error != nil && !errors.Is(tx.Error, gorm.ErrRecordNotFound) {
			for _, lang := range supportedLangs {
				langMsg[lang] = fmt.Sprintf(getLocalizedMessage(lang, "db_error"), tx.Error)
			}
			return langMsg
		}
	}

	// Build messages for each language
	for _, lang := range supportedLangs {
		if !found {
			langMsg[lang] = fmt.Sprintf(getLocalizedMessage(lang, "wo_not_found"), woNumber)
			continue
		}

		var sb strings.Builder
		switch resultData.Type {
		case "log":
			sb.WriteString(fmt.Sprintf(getLocalizedMessage(lang, "wo_details_header"), resultData.WoNumber))
			if resultData.Email != "" {
				DataTA := config.GetConfig().TechnicalAssistance.UserTA
				ta, ok := DataTA[resultData.Email]
				if ok {
					taName := ta.Name
					if taName == "" {
						taName = "N/A"
					}
					var whatTADo string
					switch resultData.Method {
					case "submit":
						whatTADo = getLocalizedMessage(lang, "ta_action_submit")
					case "edit":
						whatTADo = getLocalizedMessage(lang, "ta_action_edit")
					case "delete":
						whatTADo = getLocalizedMessage(lang, "ta_action_delete")
					}
					sb.WriteString(fmt.Sprintf("\nTA: <i>%s</i> %s", taName, whatTADo))
				}
			}
			if resultData.DateIn != "" {
				sb.WriteString(fmt.Sprintf(getLocalizedMessage(lang, "entered_dashboard"), resultData.DateIn))
			}
			if resultData.Date != nil && !resultData.Date.IsZero() {
				sb.WriteString(fmt.Sprintf(getLocalizedMessage(lang, "last_checked"), resultData.Date.Format("2006-01-02 15:04:05")))
			}
			if resultData.Feedback != "" {
				sb.WriteString(fmt.Sprintf(getLocalizedMessage(lang, "feedback"), resultData.Feedback))
			}
		case "pending":
			sb.WriteString(fmt.Sprintf(getLocalizedMessage(lang, "pending_details"), resultData.WoNumber))
			if resultData.Date != nil && !resultData.Date.IsZero() {
				sb.WriteString(fmt.Sprintf(getLocalizedMessage(lang, "entered_dashboard"), resultData.Date.Format("2006-01-02 15:04:05")))
			}
			if resultData.Feedback != "" {
				sb.WriteString(fmt.Sprintf(getLocalizedMessage(lang, "feedback"), resultData.Feedback))
			}
		case "error":
			sb.WriteString(fmt.Sprintf(getLocalizedMessage(lang, "error_details"), resultData.WoNumber))
			if resultData.Date != nil && !resultData.Date.IsZero() {
				sb.WriteString(fmt.Sprintf(getLocalizedMessage(lang, "entered_dashboard"), resultData.Date.Format("2006-01-02 15:04:05")))
			}
			if resultData.Problem != nil && *resultData.Problem != "" {
				sb.WriteString(fmt.Sprintf(getLocalizedMessage(lang, "problem"), *resultData.Problem))
			}
			if resultData.Feedback != "" {
				sb.WriteString(fmt.Sprintf(getLocalizedMessage(lang, "feedback"), resultData.Feedback))
			}
		}
		langMsg[lang] = sb.String()
	}

	return langMsg
}
