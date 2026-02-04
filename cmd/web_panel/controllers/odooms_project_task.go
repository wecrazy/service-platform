package controllers

import (
	"encoding/json"
	"net/http"
	"path/filepath"
	"service-platform/cmd/web_panel/fun"
	"service-platform/internal/config"
	"strings"

	"github.com/gin-gonic/gin"
)

func GetODOOMSProjectTaskDetail() gin.HandlerFunc {
	return func(c *gin.Context) {
		woNumber := c.Query("wo_number")

		var errorMsg string
		var projectTaskData []OdooTaskDataRequestItem

		if woNumber == "" {
			errorMsg = "wo_number parameter is required"
		} else {

			ODOOModel := "project.task"
			domain := []interface{}{
				[]interface{}{"active", "=", true},
				[]interface{}{"x_no_task", "=", woNumber},
			}
			fields := []string{
				"id",
				"company_id",
				"planned_date_begin",
				"x_sla_deadline",
				"x_no_task",
				"x_received_datetime_spk",
				"stage_id",
				"helpdesk_ticket_id",
				"x_task_type",
				"x_merchant",
				"x_merchant_group_code",
				"x_studio_kota",
				"partner_street",
				"x_pic_merchant",
				"x_pic_phone",
				"x_title_cimb",
				"technician_id",
				"x_cimb_master_mid",
				"x_cimb_master_tid",
				"x_product",
				"x_studio_edc",
				"x_simcard",
				"x_msisdn_sim_card",
				"iccid_simcard",
				"x_history",
				"x_source",
				"x_message_call",
				"x_dependency",
				"x_verified_uid",
				"timesheet_timer_last_stop",
				"x_status_merchant",
				"x_status_alamat_merchant",
				"x_latitude",
				"x_longitude",
				"x_communication_line",
				"x_reason_code_id",
				"x_keterangan",
				"x_wo_remark",
				"x_message",

				// Tanda-tanda EDC RUSAK
				"x_txn_debit",           // Baterai drop menggantikan Trx Debit On Us
				"x_txn_debit_off_us",    // Layar EDC kedip-kedip/bergaris menggantikan Trx Debit Off Us
				"x_txn_kredit",          // Tombol sudah lepas/keras/susah ditekan/tidak nampak angka menggantikan Trx Credit On Us
				"x_txn_jcb",             // Hasil print EDC kurang jelas menggantikan Trx Credit Off Us
				"x_txn_prepaid",         // EDC sering restart menggantikan Trx Prepaid
				"x_txn_jcb_contactless", // Fisik tidak sempurna/rusak menggantikan Trx Contactless
				"x_txn_qr",              // Port charger EDC bermasalah (tidak ada aliran listrik masuk) menggantikan Trx Qr
				"x_txn_ptp",             // Card Reader EDC tidak bisa membaca / sering gagal / no respon menggantikan Trx Push To Pay
				"x_condition_edc",       // Kondisi EDC Baik

				// Problem EDC
				"x_problem_edc",
			}

			prodODOOLinks := []string{
				"gsa4u",
				"https://192.168.110.48:8069",
			}
			for _, link := range prodODOOLinks {
				if strings.Contains(strings.ToLower(config.WebPanel.Get().ApiODOO.UrlGetData), link) {
					fields = append(fields, "x_link_photo")
					break
				}
			}

			order := "id desc"

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
				errorMsg = "failed to marshal payload: " + err.Error()
			} else {
				ODOOresponse, err := GetODOOMSData(string(payloadBytes))
				if err != nil {
					errorMsg = "failed fetching data from ODOO MS API: " + err.Error()
				} else {
					ODOOResponseArray, ok := ODOOresponse.([]interface{})
					if !ok {
						errorMsg = "failed to assert results as []interface{}"
					} else {
						ODOOResponseBytes, err := json.Marshal(ODOOResponseArray)
						if err != nil {
							errorMsg = "failed to marshal combined response: " + err.Error()
						} else {
							err = json.Unmarshal(ODOOResponseBytes, &projectTaskData)
							if err != nil {
								errorMsg = "failed to unmarshal response to struct: " + err.Error()
							}
						}
					}
				}
			}
		}

		importPath := config.WebPanel.Get().App.Logo
		newLogoPath := importPath[:len(importPath)-len(filepath.Base(importPath))] + "csna.png"
		odooLogoPath := importPath[:len(importPath)-len(filepath.Base(importPath))] + "logo_odoo.png"
		odooLogoDark := importPath[:len(importPath)-len(filepath.Base(importPath))] + "logo_odoo_light.png"

		// Build render-friendly data for template to avoid accessing interface fields directly in templates
		renderData := make([]map[string]interface{}, 0, len(projectTaskData))
		for _, it := range projectTaskData {
			m := map[string]interface{}{}
			m["WoNumber"] = it.WoNumber
			m["MerchantName"] = it.MerchantName.String
			m["PicMerchant"] = it.PicMerchant.String
			m["PicPhone"] = it.PicPhone.String
			m["MerchantAddress"] = it.MerchantAddress.String
			m["MerchantCity"] = it.MerchantCity.String
			m["Description"] = it.Description.String
			m["TaskType"] = it.TaskType.String
			m["Mid"] = it.Mid.String
			m["Tid"] = it.Tid.String
			m["Source"] = it.Source.String
			m["StatusMerchant"] = it.StatusMerchant.String
			m["WoRemarkTiket"] = it.WoRemarkTiket.String
			m["Longitude"] = it.Longitude.String
			m["Latitude"] = it.Latitude.String
			m["LinkPhoto"] = it.LinkPhoto.String
			// times as formatted strings (empty if invalid)
			if it.SlaDeadline.Valid {
				m["SlaDeadlineStr"] = it.SlaDeadline.Time.Format("2006-01-02 15:04:05")
			} else {
				m["SlaDeadlineStr"] = ""
			}
			if it.ReceivedDatetimeSpk.Valid {
				m["ReceivedDatetimeSpkStr"] = it.ReceivedDatetimeSpk.Time.Format("2006-01-02 15:04:05")
			} else {
				m["ReceivedDatetimeSpkStr"] = ""
			}
			if it.PlanDate.Valid {
				m["PlanDateStr"] = it.PlanDate.Time.Format("2006-01-02 15:04:05")
			} else {
				m["PlanDateStr"] = ""
			}
			if it.TimesheetLastStop.Valid {
				m["TimesheetLastStopStr"] = it.TimesheetLastStop.Time.Format("2006-01-02 15:04:05")
			} else {
				m["TimesheetLastStopStr"] = ""
			}
			m["MessageCC"] = it.MessageCC.String
			m["XCommunicationLine"] = it.CommunicationLine.String
			m["XKeterangan"] = it.WoRemark.String
			m["XWoRemark"] = it.WoRemarkTiket.String
			m["XMessage"] = it.Message.String
			// parse interface fields safely
			_, reasonStr := parseJSONIDDataCombinedSafe(it.ReasonCodeId)
			m["ReasonCodeId"] = reasonStr

			// Tanda-tanda EDC RUSAK
			m["KondisiEDC"] = it.KondisiEDC.String
			m["BateraiDrop"] = it.TxnDebitOnUs.Valid && it.TxnDebitOnUs.Int != 0
			m["LayarEDCGaris"] = it.TxnDebitOffUs.Valid && it.TxnDebitOffUs.Int != 0
			m["TombolRusak"] = it.TxnCreditOnUs.Valid && it.TxnCreditOnUs.Int != 0
			m["PrintKurangJelas"] = it.TxnCreditOffUs.Valid && it.TxnCreditOffUs.Int != 0
			m["EDCSeringRestart"] = it.TxnPrepaid.Valid && it.TxnPrepaid.Int != 0
			m["FisikRusak"] = it.TxnContactless.Valid && it.TxnContactless.Int != 0
			m["PortChargerRusak"] = it.TxnQR.Valid && it.TxnQR.Int != 0
			m["CardReaderGagal"] = it.TxnPushToPay.Valid && it.TxnPushToPay.Int != 0

			// Problem EDC
			problemEDCData := map[string]bool{
				"PergantianEDC":                     false,
				"KendalaSimCard":                    false,
				"KendalaSamCard":                    false,
				"HanyaButuhRestartEDC":              false,
				"HanyaButuhRefreshJaringan":         false,
				"HanyaButuhMerubahSettingGPRS":      false,
				"HanyaButuhBersihkanChipSimSamCard": false,
				"HanyaButuhBersihkanSlotCardReader": false,
				"KendalaAplikasi":                   false,
				"Others":                            false,
			}
			othersText := ""

			if it.ProblemEDC.Valid && it.ProblemEDC.String != "" {
				// Parse the problem EDC string
				pairs := strings.Split(it.ProblemEDC.String, ";")
				for _, pair := range pairs {
					if pair == "" {
						continue
					}
					parts := strings.Split(pair, "=")
					if len(parts) != 2 {
						continue
					}
					key := strings.TrimSpace(parts[0])
					value := strings.TrimSpace(parts[1])

					// Map the keys to our problem EDC data
					switch key {
					case "Pergantian EDC":
						problemEDCData["PergantianEDC"] = value == "Yes"
					case "Kendala SIMCARD":
						problemEDCData["KendalaSimCard"] = value == "Yes"
					case "Kendala SAMCARD":
						problemEDCData["KendalaSamCard"] = value == "Yes"
					case "Hanya butuh restart EDC":
						problemEDCData["HanyaButuhRestartEDC"] = value == "Yes"
					case "Hanya butuh refresh jaringan":
						problemEDCData["HanyaButuhRefreshJaringan"] = value == "Yes"
					case "Hanya butuh merubah setting GPRS(4G Only/AUTO)":
						problemEDCData["HanyaButuhMerubahSettingGPRS"] = value == "Yes"
					case "Hanya butuh bersihkan chip SIM/SAM CARD":
						problemEDCData["HanyaButuhBersihkanChipSimSamCard"] = value == "Yes"
					case "Hanya butuh bersihkan slot card reader EDC":
						problemEDCData["HanyaButuhBersihkanSlotCardReader"] = value == "Yes"
					case "Kendala Aplikasi":
						problemEDCData["KendalaAplikasi"] = value == "Yes"
					case "Others":
						// Example value: "Yes:Testing"
						if strings.Contains(value, ":") {
							parts2 := strings.SplitN(value, ":", 2)
							if len(parts2) == 2 {
								yesNo := parts2[0]
								text := parts2[1]

								problemEDCData["Others"] = yesNo == "Yes"
								othersText = text
							}
						} else {
							// Old format: Others=Testing or Others=No
							othersText = value
							if value != "No" && value != "" {
								problemEDCData["Others"] = true
							}
						}
					}
				}
			}
			m["ProblemEDC"] = problemEDCData
			m["ProblemEDCOthersText"] = othersText

			// push
			renderData = append(renderData, m)
		}

		// stringify renderData for debug view
		renderDataJsonBytes, _ := json.Marshal(renderData)

		c.HTML(http.StatusOK, "tab-odooms-project-task.html", gin.H{
			"WO_NUMBER":        woNumber,
			"GLOBAL_URL":       fun.GLOBAL_URL,
			"APP_LOGO":         newLogoPath,
			"ODOO_LOGO":        odooLogoPath,
			"ODOO_LOGO_DARK":   odooLogoDark,
			"DATA_RENDER":      renderData,
			"DATA_RENDER_JSON": string(renderDataJsonBytes),
			"ERROR":            errorMsg,
			"DEBUG":            c.Query("debug"),
		})
	}
}
