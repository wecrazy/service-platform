package controllers

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"reflect"
	"service-platform/cmd/web_panel/config"
	"service-platform/cmd/web_panel/fun"
	"service-platform/cmd/web_panel/internal/gormdb"
	"service-platform/cmd/web_panel/model"
	contracttechnicianmodel "service-platform/cmd/web_panel/model/contract_technician_model"
	odooms "service-platform/cmd/web_panel/model/odoo_ms"
	"strconv"
	"strings"
	"time"

	"codeberg.org/go-pdf/fpdf"
	"github.com/TigorLazuardi/tanggal"
	"github.com/gin-gonic/gin"
	"github.com/sirupsen/logrus"
	"gorm.io/datatypes"
	"gorm.io/gorm"
)

func TableContractTechnicianForHR() gin.HandlerFunc {
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
		t := reflect.TypeOf(contracttechnicianmodel.ContractTechnicianODOO{})

		// Initialize the map
		columnMap := make(map[int]string)

		excludedJsonKeys := config.GetConfig().ContractTechnicianODOO.ExcludedJSONKeysForTable

		// Loop through the fields of the struct
		colNum := 0
		for i := 0; i < t.NumField(); i++ {
			field := t.Field(i)
			// Get the JSON key
			jsonKey := field.Tag.Get("json")
			for _, excludedKey := range excludedJsonKeys {
				if jsonKey == excludedKey {
					continue
				}
			}
			columnMap[colNum] = jsonKey
			colNum++
		}

		// Get the column name based on SortColumn value
		sortColumnName := columnMap[request.SortColumn]
		if sortColumnName == "" {
			sortColumnName = "is_contract_sent"
		}
		if request.SortDir == "" {
			request.SortDir = "asc"
		}
		orderString := fmt.Sprintf("%s %s", sortColumnName, request.SortDir)

		// Initial query for filtering
		filteredQuery := dbWeb.Model(&contracttechnicianmodel.ContractTechnicianODOO{})

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

				skip := false
				for _, excludedKey := range excludedJsonKeys {
					if jsonKey == excludedKey {
						skip = true
						break
					}
				}
				if skip {
					continue
				}

				if jsonKey == "" {
					if columnKey == "" || columnKey == "-" {
						continue
					} else {
						dataField = columnKey
					}
				} else {
					dataField = jsonKey
				}

				switch dataType {
				case "string":
					filteredQuery = filteredQuery.
						Or("`"+dataField+"` LIKE ?", "%"+request.Search+"%")
				case "gorm.io/datatypes.JSON", "datatypes.JSON":
					filteredQuery = filteredQuery.
						Or("JSON_SEARCH(`"+dataField+"`, 'one', ?) IS NOT NULL", "%"+request.Search+"%")
				default:
					filteredQuery = filteredQuery.Or("`"+dataField+"` LIKE ?", "%"+request.Search+"%")
				}

			}
		} else {
			for i := 0; i < t.NumField(); i++ {
				field := t.Field(i)
				formKey := field.Tag.Get("json")

				for _, excludedKey := range excludedJsonKeys {
					if formKey == excludedKey {
						continue
					}
				}

				formValue := c.PostForm(formKey)

				if formValue != "" {
					dataType := field.Type.String()

					switch {
					case field.Type.Kind() == reflect.Bool ||
						(field.Type.Kind() == reflect.Ptr && field.Type.Elem().Kind() == reflect.Bool):
						// Convert formValue "true"/"false" to bool or int
						boolVal := false
						if formValue == "true" {
							boolVal = true
						}
						filteredQuery = filteredQuery.Where("`"+formKey+"` = ?", boolVal)
					case dataType == "gorm.io/datatypes.JSON" || dataType == "datatypes.JSON":
						// For JSON fields, assume formValue is a string to search in the array
						filteredQuery = filteredQuery.Where("JSON_SEARCH(`"+formKey+"`, 'one', ?) IS NOT NULL", "%"+formValue+"%")
					default:
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
							filteredQuery = filteredQuery.Where("`"+formKey+"` LIKE ?", "%"+formValue+"%")
						}
					}
				}
			}
		}

		// Count the total number of records
		var totalRecords int64
		dbWeb.Model(&contracttechnicianmodel.ContractTechnicianODOO{}).Count(&totalRecords)

		// Count the number of filtered records
		var filteredRecords int64
		filteredQuery.Count(&filteredRecords)

		// Apply sorting and pagination to the filtered query
		query := filteredQuery.Order(orderString)
		var Dbdata []contracttechnicianmodel.ContractTechnicianODOO
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

				case "technician":
					tech := fieldValue.Interface().(string)
					htmlRendered := fmt.Sprintf(`<span class='badge bg-info text-white'><i class="fas fa-user me-1"></i> %s</span>`, tech)
					newData[theKey] = htmlRendered

				case "contract_file_path":
					t := fieldValue.Interface().(string)
					var filePath string
					if t == "" {
						filePath = "<span class='text-danger'>Surat kontrak belum dibuat!</span>"
					} else {
						// Check if the file actually exists
						if _, err := os.Stat(t); os.IsNotExist(err) {
							filePath = "<span class='text-warning'>Surat kontrak dihapus atau tidak dibuat</span>"
						} else {
							fileContract := strings.ReplaceAll(t, "web/file/contract_technician/", "")
							fileContractProxyURL := "/proxy-pdf-contract-technician/" + fileContract
							filePath = fmt.Sprintf(`
						<button class="btn btn-sm btn-outline-danger" onclick="openPDFModelForPDFJS('%s')">
						<i class="fal fa-file-pdf me-2"></i> (Preview) Surat Kontrak
						</button>
						`, fileContractProxyURL)
						}
					}
					newData[theKey] = filePath

				case "wo_number", "ticket_subject", "wo_number_already_visit", "ticket_subject_already_visit":
					woDetailURL := config.GetConfig().App.WebPublicURL + "/odooms-project-task/detailWO"
					t := fieldValue.Interface().(datatypes.JSON)
					var arr []interface{}
					if err := json.Unmarshal(t, &arr); err != nil {
						newData[theKey] = "Invalid JSON"
					} else {
						escapedJson := strings.ReplaceAll(string(t), "'", "\\'")
						htmlRendered := fmt.Sprintf(`<button class="btn btn-sm btn-outline-primary" data-title='%s' data-json='%s' data-url='%s' onclick="showJsonListModal(this)">View (%d items)</button>`, theKey, escapedJson, woDetailURL, len(arr))
						newData[theKey] = htmlRendered
					}

				case "regenerate_contract":
					var regenBtn string
					// Check if contract file exists
					if dataInDB.ContractFilePath != "" {
						if _, err := os.Stat(dataInDB.ContractFilePath); err == nil {
							// File exists, show regenerate button
							regenBtn = fmt.Sprintf(`
							<button class="btn btn-sm btn-warning" onclick="regenerateContractTechnician(%d)">
								<i class="fas fa-sync-alt me-2"></i> Regenerate
							</button>
							`, dataInDB.ID)
						} else {
							// File doesn't exist, show generate button
							regenBtn = fmt.Sprintf(`
							<button class="btn btn-sm btn-success" onclick="regenerateContractTechnician(%d)">
								<i class="fas fa-file-pdf me-2"></i> Generate
							</button>
							`, dataInDB.ID)
						}
					} else {
						// No file path set, show generate button
						regenBtn = fmt.Sprintf(`
						<button class="btn btn-sm btn-primary" onclick="regenerateContractTechnician(%d)">
							<i class="fas fa-file-pdf me-2"></i> Generate
						</button>
						`, dataInDB.ID)
					}
					newData[theKey] = regenBtn

				case "send_contract":
					t := fieldValue.Interface().(string)
					var sendBtn string
					if t == "" {
						sendBtn = fmt.Sprintf(`
						<button class="btn btn-sm btn-success" onclick="sendIndividualContractTechnician(%d)">
							<i class="fas fa-share me-2"></i> Send
						</button>
						`, dataInDB.ID)
					} else {
						sendBtn = `<span class="text-success"><i class="fad fa-check-circle me-1"></i> Sent</span>`
					}
					newData[theKey] = sendBtn

				case "whatsapp_conversation":
					var conversationBtn string
					// Check if WhatsApp was sent (has WhatsappChatID)
					if dataInDB.WhatsappChatID != "" {
						conversationBtn = fmt.Sprintf(`
						<button class="btn btn-sm btn-info" onclick="showContractWhatsAppConversation(%d)">
							<i class="fab fa-whatsapp me-2"></i> View
						</button>
						`, dataInDB.ID)
					} else {
						conversationBtn = `<span class="text-muted"><i class="fad fa-ban me-1"></i> No Conversation</span>`
					}
					newData[theKey] = conversationBtn

				default:
					switch fieldValue.Type() {
					case reflect.TypeOf(time.Time{}):
						t := fieldValue.Interface().(time.Time)
						newData[theKey] = t.Format("02/01/2006 15:04:05")
					case reflect.TypeOf((*time.Time)(nil)):
						if fieldValue.IsNil() {
							newData[theKey] = ""
						} else {
							t := fieldValue.Interface().(*time.Time)
							newData[theKey] = t.Format("02/01/2006 15:04:05")
						}
					case reflect.TypeOf(false):
						t := fieldValue.Interface().(bool)
						var htmlRendered string
						if t {
							htmlRendered = "<i class='fad fa-check text-success fs-1'></i>"
						} else {
							htmlRendered = "<i class='fad fa-times text-danger fs-1'></i>"
						}
						newData[theKey] = htmlRendered
					default:
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

func UpdateTableContractTechnicianForHR() gin.HandlerFunc {
	return func(c *gin.Context) {
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
			fmt.Println("Error during decryption", err)
			fun.ClearCookiesAndRedirect(c, cookies)
			return
		}
		var claims map[string]interface{}
		err = json.Unmarshal(decrypted, &claims)
		if err != nil {
			fmt.Printf("Error converting JSON to map: %v", err)
			fun.ClearCookiesAndRedirect(c, cookies)
			return
		}

		var data struct {
			ID    string      `json:"id"`
			Field string      `json:"field"`
			Value interface{} `json:"value"`
		}
		if err := c.ShouldBindJSON(&data); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}

		forbiddenFields := []string{
			"id",
			"is_contract_sent",
			"contract_file_path",
			"contract_send_at",
			// ADD: more

			"wo_number",
			"ticket_subject",
			"wo_number_already_visit",
			"ticket_subject_already_visit",

			"whatsapp_chat_id",
			"whatsapp_sent_at",
			"whatsapp_chat_jid",
			"whatsapp_sender_jid",
			"whatsapp_message_body",
			"whatsapp_message_type",
			"whatsapp_quoted_msg_id",
			"whatsapp_reply_text",
			"whatsapp_reaction_emoji",
			"whatsapp_mentions",
			"whatsapp_is_group",
			"whatsapp_msg_status",
			"whatsapp_replied_by",
			"whatsapp_replied_at",
			"whatsapp_reacted_by",
			"whatsapp_reacted_at",
		}

		// Sanitize value based on field type
		t := reflect.TypeOf(contracttechnicianmodel.ContractTechnicianODOO{})
		for i := 0; i < t.NumField(); i++ {
			field := t.Field(i)
			if field.Tag.Get("json") == data.Field {
				if field.Type.Kind() == reflect.Float64 {
					if strVal, ok := data.Value.(string); ok {
						val, err := fun.SanitizeCurrency(strVal)
						if err != nil {
							c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid number format for field " + data.Field + ": " + err.Error()})
							return
						}
						data.Value = val
					}
				}
				break
			}
		}

		for _, field := range forbiddenFields {
			if data.Field == field {
				c.JSON(http.StatusForbidden, gin.H{"error": "Forbidden Field !"})
				return
			}
		}

		dbWeb := gormdb.Databases.Web

		var manufacture contracttechnicianmodel.ContractTechnicianODOO
		if err := dbWeb.First(&manufacture, data.ID).Error; err != nil {
			c.JSON(http.StatusNotFound, gin.H{"error": "Record not found"})
			return
		}

		// Update the field with the new value
		if err := dbWeb.Model(&manufacture).Update(data.Field, data.Value).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update record"})
			return
		}

		c.JSON(http.StatusOK, gin.H{"msg": fmt.Sprintf("Data of %s updated with value: %v!", data.Field, data.Value)})

		dbWeb.Create(&model.LogActivity{
			AdminID:   uint(claims["id"].(float64)),
			FullName:  claims["fullname"].(string),
			Action:    "PATCH UPDATE",
			Status:    "Success",
			Log:       fmt.Sprintf("UPDATE Manufacture Data By ID: %s; Field : %s; Value: %v; ", data.ID, data.Field, data.Value),
			IP:        c.ClientIP(),
			UserAgent: c.Request.UserAgent(),
			ReqMethod: c.Request.Method,
			ReqUri:    c.Request.RequestURI,
		})
	}
}

func DeleteTableContractTechnicianForHR() gin.HandlerFunc {
	return func(c *gin.Context) {
		// Get the ID from the URL parameter and convert to integer
		idParam := c.Param("id")
		id, err := strconv.Atoi(idParam)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid Data"})
			return
		}

		dbWeb := gormdb.Databases.Web
		var dbData contracttechnicianmodel.ContractTechnicianODOO
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

		if err := dbWeb.Delete(&dbData).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to delete Data, details: " + err.Error()})
			return
		}

		// Respond with success
		c.JSON(http.StatusOK, gin.H{"message": "Data deleted successfully"})

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
			fmt.Println("Error during decryption", err)
			fun.ClearCookiesAndRedirect(c, cookies)
			return
		}
		var claims map[string]interface{}
		err = json.Unmarshal(decrypted, &claims)
		if err != nil {
			fmt.Printf("Error converting JSON to map: %v", err)
			fun.ClearCookiesAndRedirect(c, cookies)
			return
		}
		jsonString := ""
		jsonData, err := json.Marshal(dbData)
		if err != nil {
			fmt.Println("Error converting to JSON:", err)
		} else {
			jsonString = string(jsonData)
		}
		dbWeb.Create(&model.LogActivity{
			AdminID:   uint(claims["id"].(float64)),
			FullName:  claims["fullname"].(string),
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

func LastUpdateContractTechnician() gin.HandlerFunc {
	return func(c *gin.Context) {
		var result struct {
			LastUpdated          string `json:"last_updated"`
			LastUpdatedMonthYear string `json:"last_updated_month_year"`
			AllContractSent      bool   `json:"all_contract_sent"`
		}

		dbWeb := gormdb.Databases.Web
		var lastUpdated time.Time
		err := dbWeb.Model(&contracttechnicianmodel.ContractTechnicianODOO{}).
			Select("MAX(updated_at)").
			Order("updated_at DESC").
			Limit(1).
			Scan(&lastUpdated).Error
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}

		if lastUpdated.IsZero() {
			result.LastUpdated = "N/A"
			result.LastUpdatedMonthYear = ""
		} else {
			result.LastUpdated = lastUpdated.Format("02 Jan 2006 15:04:05 MST")
			result.LastUpdatedMonthYear = lastUpdated.Format("January 2006")
		}

		var unsentCount int64
		err = dbWeb.Model(&contracttechnicianmodel.ContractTechnicianODOO{}).
			Where("is_contract_sent = ?", false).
			Count(&unsentCount).Error
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}

		result.AllContractSent = unsentCount == 0

		c.JSON(http.StatusOK, result)
	}
}

func RefreshDataContractTechnician() gin.HandlerFunc {
	return func(c *gin.Context) {
		err := GetDataTechnicianForContractInODOO()
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}

		c.JSON(http.StatusOK, gin.H{
			"message": fmt.Sprintf("Data successfully refreshed at %s", time.Now().Format("02 Jan 2006 15:04:05 MST")),
		})
	}
}

func RegeneratePDFContractTechnician() gin.HandlerFunc {
	return func(c *gin.Context) {
		id := c.Param("id")
		if id == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "ID parameter is required"})
			return
		}

		selectedMainDir, err := fun.FindValidDirectory([]string{
			"web/file/contract_technician",
			"../web/file/contract_technician",
			"../../web/file/contract_technician",
		})
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to find contract technician directory"})
			return
		}
		pdfFileDir := filepath.Join(selectedMainDir, time.Now().Format("2006-01-02"))
		if err := os.MkdirAll(pdfFileDir, 0755); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create contract technician directory"})
			return
		}

		dbWeb := gormdb.Databases.Web

		var record contracttechnicianmodel.ContractTechnicianODOO
		if err := dbWeb.First(&record, id).Error; err != nil {
			c.JSON(http.StatusNotFound, gin.H{"error": "Record not found"})
			return
		}

		technicianName := record.Technician
		if strings.Contains(technicianName, "*") {
			technicianName = strings.ReplaceAll(technicianName, "*", "(Resigned)")
		}

		pdfFileName := fmt.Sprintf("[Preview]Surat_Kontrak_%s.pdf", strings.ReplaceAll(technicianName, " ", "_"))
		pdfFilePath := filepath.Join(pdfFileDir, pdfFileName)

		err = GeneratePDFContractTechnician(&record, pdfFilePath)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to generate PDF contract: " + err.Error()})
			return
		}

		// // Update the record with the new contract file path and mark as sent
		// record.ContractFilePath = pdfFilePath
		// if err := dbWeb.Save(&record).Error; err != nil {
		// 	c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("Failed to update record with new contract file path got %v", err.Error())})
		// 	return
		// }

		c.JSON(http.StatusOK, gin.H{
			"message":  fmt.Sprintf("Surat kontrak untuk %s berhasil digenerate ulang!", record.Technician),
			"filepath": pdfFilePath,
			"filename": pdfFileName,
		})
	}
}

func GeneratePDFContractTechnician(record *contracttechnicianmodel.ContractTechnicianODOO, outputPath string) error {
	dbWeb := gormdb.Databases.Web

	loc, _ := time.LoadLocation(config.GetConfig().Default.Timezone)
	now := time.Now().In(loc)

	noSurat, err := IncrementNomorSuratContract(dbWeb, "LAST_NOMOR_SURAT_CONTRACT_GENERATED")
	if err != nil {
		return err
	}

	var noSuratStr string
	if noSurat < 1000 {
		noSuratStr = fmt.Sprintf("%03d", noSurat)
	} else {
		noSuratStr = fmt.Sprintf("%d", noSurat)
	}

	monthRoman, err := fun.MonthToRoman(int(now.Month()))
	if err != nil {
		return err
	}

	tglSuratKontrak, err := tanggal.Papar(now, "Jakarta", tanggal.WIB)
	if err != nil {
		return err
	}
	tglSuratKontrakDiterbitkan := tglSuratKontrak.Format(" ", []tanggal.Format{
		tanggal.Hari,      // 27
		tanggal.NamaBulan, // Maret
		tanggal.Tahun,     // 2025
	})

	ODOOMSSAC := config.GetConfig().ODOOMSSAC
	SACData, ok := ODOOMSSAC[record.SAC]
	if !ok {
		return fmt.Errorf("SAC data not found for SAC code: %s", record.SAC)
	}

	var contractStart, contractEnd *time.Time
	if record.UserCreatedOn != nil {
		contractStart = &time.Time{}
		*contractStart = time.Date(now.Year(), record.UserCreatedOn.Month(), record.UserCreatedOn.Day(), 0, 0, 0, 0, loc)
		endDate := time.Date(now.Year()+1, record.UserCreatedOn.Month(), record.UserCreatedOn.Day(), 23, 59, 59, 0, loc).AddDate(0, 0, -1)
		contractEnd = &endDate
	}

	var perjanjianBerlakuStart, perjanjianBerlakuEnd string
	tglPerjanjianBerlaku1, err := tanggal.Papar(*contractStart, "Jakarta", tanggal.WIB)
	if err != nil {
		return err
	}
	perjanjianBerlakuStart = tglPerjanjianBerlaku1.Format(" ", []tanggal.Format{
		tanggal.Hari,      // 27
		tanggal.NamaBulan, // Maret
	})

	tglPerjanjianBerlaku2, err := tanggal.Papar(*contractEnd, "Jakarta", tanggal.WIB)
	if err != nil {
		return err
	}
	perjanjianBerlakuEnd = tglPerjanjianBerlaku2.Format(" ", []tanggal.Format{
		tanggal.Hari,      // 26
		tanggal.NamaBulan, // Maret
	})

	nowYearStr := now.Format("2006")
	nowYearPlus1 := now.AddDate(1, 0, 0).Format("2006")
	perjanjianBerlakuStart += " " + nowYearStr
	perjanjianBerlakuEnd += " " + nowYearPlus1

	var jobGroupData odooms.ODOOMSJobGroups
	if err := dbWeb.Where("id = ?", record.JobGroupID).First(&jobGroupData).Error; err != nil {
		return fmt.Errorf("failed to fetch job group data: %v", err)
	}
	upahPokokStr := fun.FormatRupiah(jobGroupData.BasicSalary * 2) // * 2 coz its got salary 2 times in a month
	pekerjaanPertamaJOStr := fmt.Sprintf("%d", jobGroupData.TaskMax)
	insentifStr := fun.FormatRupiah(jobGroupData.InsentivePerTask)

	var nonWorkPM, nonWorkNONPM string = "Rp. 7.000", "Rp. 7.000"
	var overduePM, overdueNONPM string = "Rp. 5.000", "Rp. 2.000"
	var dataFSParam []odooms.ODOOMSFSParams
	result := dbWeb.Model(&dataFSParam).Where("id != 0").Find(&dataFSParam)
	if result.Error == nil {
		for _, fsParam := range dataFSParam {
			lowerFsParam := strings.ToLower(fsParam.ParamKey)
			if strings.Contains(lowerFsParam, "not_worked_price_pm") && fsParam.ParamValue != "" && fsParam.ParamValue != "0" {
				nonWorkPM, _ = fun.ReturnRupiahFormat(fsParam.ParamValue)
			} else if strings.Contains(lowerFsParam, "not_worked_price_npm") && fsParam.ParamValue != "" && fsParam.ParamValue != "0" {
				nonWorkNONPM, _ = fun.ReturnRupiahFormat(fsParam.ParamValue)
			} else if strings.Contains(lowerFsParam, "overdue_price_pm") && fsParam.ParamValue != "" && fsParam.ParamValue != "0" {
				overduePM, _ = fun.ReturnRupiahFormat(fsParam.ParamValue)
			} else if strings.Contains(lowerFsParam, "overdue_price_npm") && fsParam.ParamValue != "" && fsParam.ParamValue != "0" {
				overdueNONPM, _ = fun.ReturnRupiahFormat(fsParam.ParamValue)
			}
		}
	}

	placeholders := map[string]string{
		"$nomor_surat":                       noSuratStr,
		"$bulan_romawi":                      monthRoman,
		"$tahun_contract":                    now.Format("2006"),
		"$nama_teknisi":                      fun.CapitalizeWord(record.Name),
		"$tanggal_surat_kontrak_diterbitkan": tglSuratKontrakDiterbitkan,
		"$sac_nama":                          SACData.FullName,
		"$sac_ttd":                           SACData.TTDPath,

		// Pihak Kedua Page 1
		"$nik_teknisi":    record.NIK,
		"$alamat_teknisi": record.Alamat,
		"$area_teknisi":   record.Area,
		"$ttl_teknisi":    record.TempatTanggalLahir,
		"$email_teknisi":  record.Email,

		// Pasal 2 perjanjian date range
		"$perjanjian_berlaku_start": perjanjianBerlakuStart,
		"$perjanjian_berlaku_end":   perjanjianBerlakuEnd,

		// Pasal 5 pembayaran upah
		"$upah_pokok":                 upahPokokStr,
		"$pekerjaan_pertama_total_jo": pekerjaanPertamaJOStr,
		"$insentif":                   insentifStr,
		"$penalty_non_worked_pm":      nonWorkPM,
		"$penalty_non_worked_non_pm":  nonWorkNONPM,
		"$penalty_overdue_pm":         overduePM,
		"$penalty_overdue_non_pm":     overdueNONPM,
	}

	imgAssetsDir, err := fun.FindValidDirectory([]string{
		"web/assets/self/img",
		"../web/assets/self/img",
		"../../web/assets/self/img",
	})
	if err != nil {
		return fmt.Errorf("failed to find image assets directory: %v", err)
	}
	imgCSNA := filepath.Join(imgAssetsDir, "csna_small_jpg.jpg")
	imgTTDSAC := filepath.Join(imgAssetsDir, placeholders["$sac_ttd"])

	fontMainDir, err := fun.FindValidDirectory([]string{
		"web/assets/font",
		"../web/assets/font",
		"../../web/assets/font",
	})
	if err != nil {
		return fmt.Errorf("failed to find font directory: %v", err)
	}

	pdf := fpdf.New("P", "mm", "A4", fontMainDir)
	pdf.SetTitle("Surat Kontrak Kerja", true)
	pdf.SetAuthor(fmt.Sprintf("HRD %s", config.GetConfig().Default.PT), true)
	pdf.SetCreator(fmt.Sprintf("Service Report %s", config.GetConfig().Default.PT), true)
	pdf.SetKeywords("kontrak, surat kontrak, teknisi", true)
	pdf.SetSubject("Surat Kontrak Kerja - Atas bergabungnya karyawan ke perusahaan", true)
	pdf.SetCreationDate(time.Now())
	pdf.SetLang("id-ID")

	// Add fonts
	pdf.AddFont("Arial", "", "arial.json")                       // Regular
	pdf.AddFont("Arial", "B", "arialbd.json")                    // Bold
	pdf.AddFont("Arial", "BI", "arialbi.json")                   // Bold Italic
	pdf.AddFont("Arial", "I", "ariali.json")                     // Italic
	pdf.AddFont("Arial", "Blk", "ariblk.json")                   // Black
	pdf.AddFont("CenturyGothic", "", "CenturyGothic.json")       // Regular
	pdf.AddFont("CenturyGothic", "B", "CenturyGothic-Bold.json") // Bold
	pdf.AddFont("Calibri", "", "calibri.json")
	pdf.AddFont("Calibri", "B", "calibrib.json")

	// Set header function for all pages
	pdf.SetHeaderFuncMode(func() {
		// Draw logo at top left
		pdf.ImageOptions(imgCSNA, 13, 5, 50, 0, false, fpdf.ImageOptions{ImageType: "JPG"}, 0, "")
		// // Draw a horizontal line under header
		// pdf.SetDrawColor(220, 220, 220)
		// pdf.SetLineWidth(0.5)
		// pdf.Line(15, 30, 195, 30)
		// pdf.SetY(35) // Move Y below header for content
	}, true)

	// Set footer function for all pages
	pdf.SetFooterFunc(func() {
		footerY := 283.0 // Bottom margin for A4

		// Footer text
		pdf.SetFont("CenturyGothic", "B", 11)
		pdf.SetTextColor(100, 100, 100)

		// Move to right corner with specific positioning
		companyName := config.GetConfig().Default.PT
		textWidth := pdf.GetStringWidth(companyName)
		pageWidth, _ := pdf.GetPageSize()
		rightMargin := 4.0 // Adjust this value to control distance from right edge

		pdf.SetXY(pageWidth-rightMargin-textWidth, footerY-5)
		pdf.CellFormat(textWidth, 5, companyName, "", 0, "L", false, 0, "")

		// Other info details with MultiCell right aligned
		pdf.SetFont("CenturyGothic", "", 7)
		pdf.SetTextColor(120, 120, 120)

		footerText := "Jl. Puri Utama Blok H1 No.19-22, Kel. Petir Kec. Cipondoh\n" +
			"Kota Tangerang, Banten - Indonesia 15147\n" +
			"Tel.: (021)55717377"

		// Right margin position
		marginRight := 4.0

		lines := strings.Split(footerText, "\n")
		y := footerY - 0.5

		for _, line := range lines {
			textWidth := pdf.GetStringWidth(line)
			x := pageWidth - marginRight - textWidth // shift so it's right-aligned
			pdf.SetXY(x, y)
			pdf.CellFormat(textWidth, 3, line, "", 0, "L", false, 0, "")
			y += 3
		}

		pdf.SetTextColor(0, 0, 0)

	})

	// ########################### Page 1 #################################
	pdf.AddPage()
	// ====================== Title with underline ========================
	currentY := 45.0
	pdf.SetY(currentY)
	pdf.SetFont("Arial", "B", 10) // Use same font to measure text width
	titleText := "PERJANJIAN KERJA WAKTU TERTENTU"
	titleWidth := pdf.GetStringWidth(titleText)
	// Calculate center position
	pageWidth, _ := pdf.GetPageSize()
	lineStartX := (pageWidth - titleWidth) / 2
	lineEndX := lineStartX + titleWidth

	// Write the title text first
	currentY = 30.0
	pdf.SetXY(0, currentY)
	pdf.CellFormat(210, 8, titleText, "", 1, "C", false, 0, "")

	// Draw underline
	currentY += 6 // Move down 6mm from current Y position
	pdf.SetLineWidth(0.5)
	pdf.SetDrawColor(0, 0, 0) // black
	pdf.Line(lineStartX, currentY, lineEndX, currentY)

	// Nomor Surat
	currentY -= 1
	pdf.SetFont("Arial", "B", 10)
	pdf.SetXY(0, currentY)
	textToWrite := fmt.Sprintf("Nomor : %s/Teknisi/HRD-CSNA/%s/%s",
		placeholders["$nomor_surat"],
		placeholders["$bulan_romawi"],
		placeholders["$tahun_contract"],
	)
	pdf.CellFormat(210, 8, textToWrite, "", 1, "C", false, 0, "")
	// ====================================================================

	// Body
	pdf.SetFont("Arial", "", 9)
	pdf.SetY(currentY + 15)
	pdf.SetX(20)
	pdf.CellFormat(0, 7, "Yang bertanda tangan dibawah ini :", "", 1, "L", false, 0, "")

	currentY = pdf.GetY()
	currentY += 1
	pdf.SetY(currentY)
	// define fields
	pihakPertamaFields := []pdfField{
		{"Nama Perusahaan", config.GetConfig().Default.PT},
		{"Alamat", config.GetConfig().Default.PTAddress},
		{"Kota", config.GetConfig().Default.PTCity},
	}

	// loop
	for i, f := range pihakPertamaFields {
		pdf.SetX(20)

		if i == 0 {
			// First line has "I." before the label
			pdf.CellFormat(5, 4, "I.", "", 0, "L", false, 0, "")
		} else {
			// Other lines: keep blank space instead of "I."
			pdf.CellFormat(5, 4, "", "", 0, "L", false, 0, "")
		}

		// label
		pdf.CellFormat(46, 4, f.Label, "", 0, "L", false, 0, "")
		// colon
		pdf.CellFormat(5, 4, ":", "", 0, "L", false, 0, "")
		// value (allow wrapping)
		pdf.MultiCell(120, 4, f.Value, "", "L", false)
	}

	currentY = pdf.GetY()
	currentY += 4
	pdf.SetXY(20, currentY)
	pdf.SetFont("Arial", "", 9)
	pdf.CellFormat(pdf.GetStringWidth("Dalam hal ini bertindak sebagai "), 5, "Dalam hal ini bertindak sebagai ", "", 0, "L", false, 0, "")
	pdf.SetFont("Arial", "B", 9)
	pdf.CellFormat(pdf.GetStringWidth("PIHAK PERTAMA"), 5, "PIHAK PERTAMA", "", 0, "L", false, 0, "")

	pdf.Ln(7)
	pdf.SetFont("Arial", "", 9)
	pihakKeduaFields := []pdfField{
		{"Nama", placeholders["$nama_teknisi"]},
		{"NIK", placeholders["$nik_teknisi"]},
		{"Alamat", placeholders["$alamat_teknisi"]},
		{"Area", placeholders["$area_teknisi"]},
		{"Tempat Tanggal Lahir", placeholders["$ttl_teknisi"]},
		{"Email Address", placeholders["$email_teknisi"]},
	}

	// loop
	for i, f := range pihakKeduaFields {
		pdf.SetX(20)

		if i == 0 {
			// First line has "II." before the label
			pdf.CellFormat(5, 4, "II.", "", 0, "L", false, 0, "")
		} else {
			// Other lines: keep blank space instead of "II."
			pdf.CellFormat(5, 4, "", "", 0, "L", false, 0, "")
		}

		// label
		pdf.CellFormat(46, 4, f.Label, "", 0, "L", false, 0, "")
		// colon
		pdf.CellFormat(5, 4, ":", "", 0, "L", false, 0, "")
		// value (allow wrapping)
		pdf.MultiCell(120, 4, f.Value, "", "L", false)
	}

	currentY = pdf.GetY()
	currentY += 4
	pdf.SetXY(20, currentY)
	pdf.SetFont("Arial", "", 9)
	pdf.CellFormat(pdf.GetStringWidth("Dalam hal ini bertindak sebagai "), 5, "Dalam hal ini bertindak sebagai ", "", 0, "L", false, 0, "")
	pdf.SetFont("Arial", "B", 9)
	pdf.CellFormat(pdf.GetStringWidth("PIHAK KEDUA"), 5, "PIHAK KEDUA", "", 0, "L", false, 0, "")

	// Example: "Dengan ini sepakat bahwa PIHAK KEDUA bekerja di tempat PIHAK PERTAMA dalam bidang pekerjaan “Manage Service EDC” dengan jabatan “Teknisi”"
	// Styles: regular, bold, italic, bold italic
	currentY = pdf.GetY() + 7
	pdf.SetLeftMargin(20)   // ensure wrap always starts at 20
	pdf.SetXY(20, currentY) // starting point

	pdf.SetFont("Arial", "", 9)
	pdf.Write(4, "Dengan ini sepakat bahwa ")

	pdf.SetFont("Arial", "B", 9)
	pdf.Write(4, "PIHAK KEDUA ")

	pdf.SetFont("Arial", "", 9)
	pdf.Write(4, "bekerja di tempat ")

	pdf.SetFont("Arial", "B", 9)
	pdf.Write(4, "PIHAK PERTAMA ")

	pdf.SetFont("Arial", "", 9)
	pdf.Write(4, "dalam bidang pekerjaan ")

	pdf.SetFont("Arial", "BI", 9)
	pdf.Write(4, `"Manage Service EDC" `)

	pdf.SetFont("Arial", "", 9)
	pdf.Write(4, "dengan jabatan ")

	pdf.SetFont("Arial", "BI", 9)
	pdf.Write(4, `"Teknisi"`)

	currentY = pdf.GetY() + 7
	pdf.SetXY(20, currentY)
	pdf.SetFont("Arial", "", 9)
	pdf.Write(4, "Maka dari itu, ")

	pdf.SetFont("Arial", "B", 9)
	pdf.Write(4, "PIHAK PERTAMA ")

	pdf.SetFont("Arial", "", 9)
	pdf.Write(4, "dan ")

	pdf.SetFont("Arial", "B", 9)
	pdf.Write(4, "PIHAK KEDUA ")

	pdf.SetFont("Arial", "", 9)
	pdf.Write(4, "sepakat mengadakan Perjanjian Kerja Waktu Tertentu sesuai dengan ketentuan-ketentuan sebagai berikut : ")

	// Pasal 1
	currentY = pdf.GetY() + 7
	pdf.SetFont("Arial", "B", 9)
	pdf.SetXY(0, currentY)
	pdf.MultiCell(210, 4, "PASAL 1\nRUANG LINGKUP PEKERJAAN", "", "C", false)

	pdf.Ln(5)
	pasal1Items := []ListItem{
		{
			Parts: []TextRun{
				{"Bahwa PIHAK KEDUA", ""},
				{" akan bekerja sebagai ", ""},
				{"Teknisi", "BI"},
				{", dengan tanggung jawab sebagai berikut.:", ""},
			},
			Children: [][]TextRun{
				{
					{"Penyelesaian pekerjaan sesuai dengan SLA yang telah ditentukan.", ""},
				},
			},
		},
		{
			Parts: []TextRun{
				{"Pekerjaan yang dimaksud pada ayat (1), adalah :", ""},
			},
			Children: [][]TextRun{
				{{"Instalasi / pemasangan mesin dan perlengkapannya", ""}},
				{{"Memberikan pelatihan kepada customer tentang penggunaan mesin", ""}},
				{{"Penarikan mesin dan perlengkapannya.", ""}},
				{{"Preventive Maintenance", "I"}}, // italic
				{{"Corrective Maintenance", "I"}}, // italic
				{{"Pengiriman material (Thermal, Adaptor, Sticker, dll)", ""}},
				{{"Melakukan stock opname (Asset) sesuai penugasan, Wajib dilaporkan kepada team Asset/Leader.", ""}},
			},
		},
	}

	for i, item := range pasal1Items {
		// parent number (1., 2., …)
		pdf.SetX(20)
		pdf.SetFont("Arial", "", 9)
		pdf.CellFormat(5, 4, fmt.Sprintf("%d.", i+1), "", 0, "L", false, 0, "")

		// main text with mixed styles
		pdf.SetX(25)
		writeFpdfRuns(pdf, item.Parts, 4)
		pdf.Ln(5)

		// children (a., b., …)
		for j, child := range item.Children {
			pdf.SetX(25)
			pdf.SetFont("Arial", "", 9)
			pdf.CellFormat(5, 4, fmt.Sprintf("%c.", rune('a'+j)), "", 0, "L", false, 0, "")

			pdf.SetX(30)
			writeFpdfRuns(pdf, child, 4)
			pdf.Ln(5)
		}

		pdf.Ln(2)
	}

	// Pasal 2
	currentY = pdf.GetY() + 5
	pdf.SetFont("Arial", "B", 9)
	pdf.SetXY(0, currentY)
	pdf.MultiCell(210, 4, "PASAL 2\nJANGKA WAKTU DAN\nPENGAKHIRAN PERJANJIAN", "", "C", false)

	pdf.Ln(5)
	pasal2Items := []ListItem{
		{
			Parts: []TextRun{
				{"Perjanjian ini berlaku dari tanggal ", ""},
				{placeholders["$perjanjian_berlaku_start"], "B"},
				{" sampai dengan tanggal ", ""},
				{placeholders["$perjanjian_berlaku_end"], "B"},
			},
		},
		{
			Parts: []TextRun{
				{"Dengan berakhirnya Perjanjian Kerjasama ini, maka hubungan kerja antara PIHAK PERTAMA dengan PIHAK KEDUA berakhir secara otomatis", ""},
			},
		},
		{
			Parts: []TextRun{
				{"Dengan berakhirnya hubungan kerja antara PIHAK PERTAMA dengan PIHAK KEDUA, maka tidak ada kewajiban PIHAK PERTAMA untuk memberikan pesangon atau/dan ganti rugi berupa apapun kepada PIHAK KEDUA.", ""},
			},
		},
		{
			Parts: []TextRun{
				{"Selama masa berlakunya Perjanjian Kerja Waktu Tertentu ini, setelah dilakukan evaluasi, PIHAK PERTAMA dapat melakukan Pemutusan Hubungan Kerja sewaktu-waktu, apabila PIHAK KEDUA tidak mencapai Performa/Target yang diberikan oleh PIHAK PERTAMA.", ""},
			},
		},
	}

	for i, item := range pasal2Items {
		// Set Y for each item
		curY := pdf.GetY()
		numberX := 20.0
		startX := 25.0
		usableWidth := 170.0
		lineHeight := 5.0

		// Print number using Text (not CellFormat) for precise placement
		pdf.SetFont("Arial", "", 9)
		pdf.Text(numberX, curY+lineHeight, fmt.Sprintf("%d.", i+1))

		// Start text at startX
		curX := startX
		curY = pdf.GetY()

		// Loop styled parts
		for _, r := range item.Parts {
			pdf.SetFont("Arial", r.Style, 9)
			chunks := pdf.SplitLines([]byte(r.Text), usableWidth-(curX-startX))
			for j, chunk := range chunks {
				pdf.Text(curX, curY+lineHeight, string(chunk))
				if j < len(chunks)-1 {
					curY += lineHeight
					curX = startX
				} else {
					curX += pdf.GetStringWidth(string(chunk))
				}
			}
		}
		// Move Y for next item
		pdf.SetY(curY + lineHeight)
	}

	// ############################ Page 2 #################################
	pdf.AddPage()
	// Pasal 3
	currentY = 35.0
	pdf.SetFont("Arial", "B", 9)
	pdf.SetXY(0, currentY)
	pdf.MultiCell(210, 4, "PASAL 3\nHAK DAN KEWAJIBAN", "", "C", false)

	pdf.Ln(3)
	pasal3Items := []ListItem{
		{
			Parts: []TextRun{
				{"Kewajiban Pihak Pertama", ""},
			},
			Children: [][]TextRun{
				{{"Pihak Pertama WAJIB membayarkan upah kepada Pihak Kedua yang dilaksanakan setiap bulannya.", ""}},
				{{"Pihak Pertama WAJIB memberikan Tunjangan Hari Raya kepada Pihak Kedua yang mempunyai masa kerja 1(satu) bulan secara terus menerus atau lebih.", ""}},
			},
		},
		{
			Parts: []TextRun{
				{"Hak Pihak Pertama", ""},
			},
			Children: [][]TextRun{
				{{"Pihak Pertama BERHAK mendapatkan setiap hasil pekerjaan yang diberikan kepada  Pihak Kedua.", ""}},
				{{"Pihak Pertama BERHAK memberhentikan Pihak Kedua dengan alasan tertentu, seperti tidak mentaati aturan perusahaan, merugikan perusahaan serta melanggar norma yang berlaku.", ""}},
			},
		},
		{
			Parts: []TextRun{
				{"Kewajiban Pihak Kedua", ""},
			},
			Children: [][]TextRun{
				{{"Pihak Kedua WAJIB mematuhi aturan dan standard yang telah ditentukan oleh Pihak Pertama.", ""}},
				{{"Pihak Kedua WAJIB menyimpan informasi yang sifatnya rahasia dan tidak membuka rahasia perusahaan kepada pihak lainnya.", ""}},
				{{"Pihak Kedua WAJIB menjaga Asset yang diberikan oleh Pihak Pertama dan WAJIB mengembalikan Asset kepada Pihak Pertama saat masa kerja berakhir/pengakhiran masa kerja.", ""}},
			},
		},
		{
			Parts: []TextRun{
				{"Hak Pihak Kedua", ""},
			},
			Children: [][]TextRun{
				{{"Pihak Kedua BERHAK menyampaikan pendapatnya secara terbuka, sesuai dengan norma dan aturan yang berlaku.", ""}},
				{{"Pihak Kedua BERHAK menerima Tunjangan Hari Raya yang telah bekerja selama 1 (satu) bulan atau lebih, tetapi kurang dari 1 (satu) tahun, THR diberikan secara proporsional.", ""}},
			},
		},
	}

	for i, item := range pasal3Items {
		// parent number (1., 2., …)
		pdf.SetX(20)
		pdf.SetFont("Arial", "", 9)
		pdf.CellFormat(5, 4, fmt.Sprintf("%d.", i+1), "", 0, "L", false, 0, "")

		// main text with mixed styles
		pdf.SetX(25)
		writeFpdfRuns(pdf, item.Parts, 4)
		pdf.Ln(3)

		// children (a., b., …)
		for j, child := range item.Children {
			pdf.SetX(25)
			pdf.SetFont("Arial", "", 9)
			pdf.CellFormat(5, 4, fmt.Sprintf("%c.", rune('a'+j)), "", 0, "L", false, 0, "")

			pdf.SetX(30)
			// Concatenate all TextRun.Text for the child, applying styles if needed
			var childText string
			for _, r := range child {
				childText += r.Text + " "
			}
			// Use MultiCell for wrapping and indentation
			pdf.MultiCell(170, 4, childText, "", "L", false)
		}
	}

	// Pasal 4
	currentY = pdf.GetY() + 5
	pdf.SetFont("Arial", "B", 9)
	pdf.SetXY(0, currentY)
	pdf.MultiCell(210, 4, "PASAL 4\nWAKTU KERJA TEKNISI", "", "C", false)

	pdf.Ln(3)
	pdf.SetX(20)
	pdf.SetFont("Arial", "", 9)
	pdf.Write(4, "Waktu kerja ")
	pdf.SetFont("Arial", "B", 9)
	pdf.Write(4, "PIHAK KEDUA ")
	pdf.SetFont("Arial", "", 9)
	pdf.Write(4, "disesuaikan dengan kondisi di lapangan dengan ketentuan Hari Kerja dan Jam Kerja berdasarkan tugas yang diberikan dengan memperhatikan SLA pekerjaan tersebut, termasuk pada hari libur.")

	// Pasal 5
	currentY = pdf.GetY() + 7
	pdf.SetFont("Arial", "B", 9)
	pdf.SetXY(0, currentY)
	pdf.MultiCell(210, 4, "PASAL 5\nPEMBAYARAN UPAH", "", "C", false)

	pdf.Ln(3)
	pdf.SetFont("Arial", "", 9)
	pdf.SetX(20)
	pdf.MultiCell(170, 5, "PIHAK PERTAMA akan memberikan upah kepada PIHAK KEDUA dengan ketentuan sebagai berikut :", "", "L", false)

	type Pasal5Item struct {
		Letter string
		Label  string
		Value  []TextRun
	}

	pasal5Items := []Pasal5Item{
		{
			"a", "Upah Pokok",
			[]TextRun{
				{"Rp. ", ""},
				{placeholders["$upah_pokok"], "B"},
				{" /bulan untuk pekerjaan ", ""},
				{placeholders["$pekerjaan_pertama_total_jo"], "B"},
				{" JO pertama dengan status Done.", ""},
			},
		},
		{
			"b", "Insentif",
			[]TextRun{
				{fmt.Sprintf("Rp. %s /JO diberikan untuk kelebihan JO yang dikerjakan dengan status Done.", placeholders["$insentif"]), ""},
			},
		},
		{
			"c", "Pinalti",
			[]TextRun{
				{fmt.Sprintf("%s /JO akan di potong untuk JO Non-PM yang tidak dikerjakan.", placeholders["$penalty_non_worked_non_pm"]), ""},
			},
		},
		{
			"d", "Pinalti",
			[]TextRun{
				{fmt.Sprintf("%s /JO akan di potong untuk JO PM yang tidak dikerjakan.", placeholders["$penalty_non_worked_pm"]), ""},
			},
		},
		{
			"e", "Pinalti",
			[]TextRun{
				{fmt.Sprintf("%s /JO akan di potong untuk JO PM (Preventive Maintenance) yang over SLA per periode bulan PM.", placeholders["$penalty_overdue_pm"]), ""},
			},
		},
		{
			"f", "Pinalti",
			[]TextRun{
				{fmt.Sprintf("%s /JO akan di potong untuk JO Non-PM (Corrective Maintenance) yang over SLA.", placeholders["$penalty_overdue_non_pm"]), ""},
			},
		},
	}

	for _, item := range pasal5Items {
		pdf.SetX(25)
		pdf.SetFont("Arial", "", 9)
		pdf.CellFormat(5, 4, fmt.Sprintf("%s.", item.Letter), "", 0, "L", false, 0, "")
		pdf.CellFormat(35, 4, item.Label, "", 0, "L", false, 0, "")
		pdf.CellFormat(5, 4, ":", "", 0, "L", false, 0, "")
		// Use writeFpdfRuns for styled value
		writeFpdfRuns(pdf, item.Value, 4)
		pdf.Ln(4)
	}

	// Use a simple bullet (•) instead of black diamond
	pdf.SetX(25)
	y := pdf.GetY() + 2             // Center vertically with text
	pdf.SetDrawColor(0, 0, 0)       // Black outline
	pdf.SetFillColor(255, 255, 255) // White fill
	pdf.Circle(23, y, 1.5, "D")     // (x, y, radius, "D" for draw/outline only)

	pdf.SetFont("Arial", "BI", 9)
	pdf.MultiCell(165, 4, "Upah sebagaimana tersebut pada pasal ini a.b.c adalah Take Home Pay (sudah dipotong PPH 21)", "", "L", false)

	// Pasal 6
	currentY = pdf.GetY() + 5
	pdf.SetFont("Arial", "B", 9)
	pdf.SetXY(0, currentY)
	pdf.MultiCell(210, 4, "PASAL 6\nSANKSI - SANKSI", "", "C", false)

	pdf.Ln(3)
	pasal6Items := []ListItem{
		{
			Parts: []TextRun{
				{"PIHAK PERTAMA akan memberikan Sanksi berupa teguran Lisan,SP-1, SP-2 sampai dengan SP-3 (Pemutusan Hubungan Kerja)  kepada PIHAK KEDUA apabila PIHAK KEDUA :", ""},
			},
			Children: [][]TextRun{
				{{"Lalai dalam melakukan tugas yang diberikan.", ""}},
				{{"Tidak mematuhi aturan yang diberikan oleh Perusahaan.", ""}},
			},
		},
		{
			Parts: []TextRun{
				{"PIHAK PERTAMA dapat melakukan Pemutusan Hubungan Kerja (PHK) apabila PIHAK KEDUA melakukan kesalahan berat serta merugikan perusahaan ataupun tersangkut masalah hukum pidana baik didalam maupun diluar perusahaan sesuai Undang-Undang Nomor 13 Tahun 2003, tentang Ketenagakerjaan.", ""},
			},
		},
		{
			Parts: []TextRun{
				{"PIHAK KEDUA Wajib membayarkan ganti rugi (sebesar harga Asset yang diberikan) kepada PIHAK PERTAMA atau dilakukan pemotongan gaji pada PIHAK KEDUA jika melanggar Pasal 3 Ayat 3 Poin C.", ""},
			},
		},
		{
			Parts: []TextRun{
				{"PIHAK KEDUA Tidak Diperbolehkan bekerja di 2 (dua) Perusahaan/Vendor lain, apabila melanggar akan dikenakan Pinalty dengan WAJIB membayarkan kepada PIHAK PERTAMA 3 (tiga) kali lipat dari Jumlah Gaji yang diterima PIHAK KEDUA dan Pemutusan Hubungan Kerja (PHK).", ""},
			},
		},
	}

	for i, item := range pasal6Items {
		numberX := 20.0
		textX := 25.0
		usableWidth := 165.0 // 170 - 5 for number width
		lineHeight := 4.0

		// Prepare the full parent text with styles
		var parentText string
		for _, r := range item.Parts {
			parentText += r.Text + " "
		}

		// Split into lines for wrapping
		lines := pdf.SplitLines([]byte(parentText), usableWidth)
		for j, line := range lines {
			if j == 0 {
				pdf.SetXY(numberX, pdf.GetY())
				pdf.SetFont("Arial", "", 9)
				pdf.CellFormat(5, lineHeight, fmt.Sprintf("%d.", i+1), "", 0, "L", false, 0, "")
				pdf.SetXY(textX, pdf.GetY())
				pdf.MultiCell(usableWidth, lineHeight, string(line), "", "L", false)
			} else {
				pdf.SetXY(textX, pdf.GetY())
				pdf.MultiCell(usableWidth, lineHeight, string(line), "", "L", false)
			}
		}

		// children (a., b., …)
		for j, child := range item.Children {
			pdf.SetX(25)
			pdf.SetFont("Arial", "", 9)
			pdf.CellFormat(5, 4, fmt.Sprintf("%c.", rune('a'+j)), "", 0, "L", false, 0, "")

			pdf.SetX(30)
			var childText string
			for _, r := range child {
				childText += r.Text + " "
			}
			pdf.MultiCell(170, 4, childText, "", "L", false)
		}
		// pdf.Ln(2)
	}

	// ############################ Page 3 #################################
	pdf.AddPage()
	// Pasal 7
	currentY = 35.0
	pdf.SetFont("Arial", "B", 9)
	pdf.SetXY(0, currentY)
	pdf.MultiCell(210, 4, "PASAL 7\nPERSELISIHAN", "", "C", false)

	pdf.Ln(3)
	pasal7Items := []ListItem{
		{
			Parts: []TextRun{
				{"Apabila terjadi perselisihan para pihak sepakat diselesaikan secara musyawarah dan mufakat.", ""},
			},
		},
		{
			Parts: []TextRun{
				{"Apabila penyelesaian perselisihan secara musyawarah mufakat mengalami jalan buntu, maka kedua belah pihak sepakat untuk diselesaikan sesuai dengan hukum yang berlaku di NKRI.", ""},
			},
		},
	}

	for i, item := range pasal7Items {
		numberX := 20.0
		textX := 25.0
		usableWidth := 165.0 // 170 - 5 for number width
		lineHeight := 4.0

		// Prepare the full parent text with styles
		var parentText string
		for _, r := range item.Parts {
			parentText += r.Text + " "
		}

		// Split into lines for wrapping
		lines := pdf.SplitLines([]byte(parentText), usableWidth)
		for j, line := range lines {
			if j == 0 {
				pdf.SetXY(numberX, pdf.GetY())
				pdf.SetFont("Arial", "", 9)
				pdf.CellFormat(5, lineHeight, fmt.Sprintf("%d.", i+1), "", 0, "L", false, 0, "")
				pdf.SetXY(textX, pdf.GetY())
				pdf.MultiCell(usableWidth, lineHeight, string(line), "", "L", false)
			} else {
				pdf.SetXY(textX, pdf.GetY())
				pdf.MultiCell(usableWidth, lineHeight, string(line), "", "L", false)
			}
		}

		// children (a., b., …)
		for j, child := range item.Children {
			pdf.SetX(25)
			pdf.SetFont("Arial", "", 9)
			pdf.CellFormat(5, 4, fmt.Sprintf("%c.", rune('a'+j)), "", 0, "L", false, 0, "")

			pdf.SetX(30)
			var childText string
			for _, r := range child {
				childText += r.Text + " "
			}
			pdf.MultiCell(170, 4, childText, "", "L", false)
		}
		// pdf.Ln(2)
	}

	// Pasal 8
	currentY = pdf.GetY() + 5
	pdf.SetFont("Arial", "B", 9)
	pdf.SetXY(0, currentY)
	pdf.MultiCell(210, 4, "PASAL 8\nPENUTUP", "", "C", false)

	pdf.Ln(3)
	pasal8Text := "Demikian Perjanjian Kerja Waktu Tertentu ini dibuat rangkap 2 (dua) bermeterai cukup dan setelah para pihak membaca, mengerti serta menandatanganinya dalam keadaan sadar, sehat jasmani dan rohani tanpa ada paksaan dari siapapun dan dari pihak manapun, kemudian masing-masing pihak memegang 1 (satu) bundel asli."
	pdf.SetFont("Arial", "", 9)
	pdf.MultiCell(170, 4, pasal8Text, "", "J", false)

	// =================================== Signatures ===================================
	currentY += 35
	leftX := 35.0
	rightX := 155.0 // adjust for your page width

	pdf.SetFont("Arial", "", 9)
	pdf.SetXY(20, currentY)
	pdf.CellFormat(80, 5, fmt.Sprintf("Tangerang, %v", placeholders["$tanggal_surat_kontrak_diterbitkan"]), "", 0, "L", false, 0, "")

	currentY += 8
	pdf.SetFont("Calibri", "B", 11)
	pdf.SetXY(leftX, currentY)
	pdf.CellFormat(80, 5, "PIHAK PERTAMA", "", 0, "L", false, 0, "")
	pdf.SetXY(rightX, currentY)
	pdf.CellFormat(80, 5, "PIHAK KEDUA", "", 0, "L", false, 0, "")

	currentY += 35 // space for signatures

	// --- Left signature ---
	ttdSacWidth := 55.0
	leftXForTTD := leftX
	currentYTTDSAC := currentY - 33
	switch placeholders["$sac_ttd"] {
	case "ttd_angga.png":
		leftXForTTD = leftX - 10.0
		currentYTTDSAC = currentY - 25
	case "ttd_osvaldo.png":
		ttdSacWidth = 33.0
		leftXForTTD = leftX - 3.0
		currentYTTDSAC = currentY - 30
	case "ttd_tomi.png":
		leftXForTTD = leftX - 8.0
	case "ttd_burhan.png":
		ttdSacWidth = 13.0
		leftXForTTD = leftX + 8.0
		currentYTTDSAC = currentY - 27
	case "ttd_tetty.png":
		ttdSacWidth = 28.0
		leftXForTTD = leftX + 0.0
		currentYTTDSAC = currentY - 30
	}
	pdf.ImageOptions(imgTTDSAC, leftXForTTD, currentYTTDSAC, ttdSacWidth, 0, false, fpdf.ImageOptions{ImageType: "PNG"}, 0, "")

	labelDiterbitkan := "PIHAK PERTAMA"
	labelWidth := pdf.GetStringWidth(labelDiterbitkan)
	nameWidth := pdf.GetStringWidth(placeholders["$sac_nama"])
	// padding := 4.0

	// Compute X so the name is centered under the label
	centerX := leftX + (labelWidth / 2) - (nameWidth / 2)

	pdf.SetXY(centerX, currentY)
	pdf.CellFormat(nameWidth, 5, placeholders["$sac_nama"], "", 0, "L", false, 0, "")

	// Underline
	pdf.SetDrawColor(0, 0, 0)
	pdf.SetLineWidth(0.5)
	// pdf.Line(centerX, currentY+5, centerX+nameWidth+padding, currentY+5)
	pdf.Line(centerX+2, currentY+5, centerX+nameWidth, currentY+5)

	roleWidth := pdf.GetStringWidth("Service Area Coordinator")
	roleX := leftX + (labelWidth / 2) - (roleWidth / 2)
	pdf.SetXY(roleX, currentY+5)
	pdf.CellFormat(roleWidth, 5, "Service Area Coordinator", "", 0, "L", false, 0, "")

	// --- Right signature ---
	labelMengetahui := "PIHAK KEDUA"
	labelWidthR := pdf.GetStringWidth(labelMengetahui)
	mgrWidth := pdf.GetStringWidth(placeholders["$nama_teknisi"])

	// Compute X so the SAC name is centered under the label
	centerXR := rightX + (labelWidthR / 2) - (mgrWidth / 2)

	pdf.SetXY(centerXR, currentY)
	pdf.CellFormat(mgrWidth, 5, placeholders["$nama_teknisi"], "", 0, "L", false, 0, "")

	// Underline
	pdf.SetDrawColor(0, 0, 0)
	pdf.SetLineWidth(0.5)
	// pdf.Line(centerXR, currentY+5, centerXR+mgrWidth+padding, currentY+5)
	pdf.Line(centerXR+2, currentY+5, centerXR+mgrWidth, currentY+5)

	roleR := "Teknisi"
	roleRWidth := pdf.GetStringWidth(roleR)
	roleRX := rightX + (labelWidthR / 2) - (roleRWidth / 2)

	pdf.SetXY(roleRX, currentY+5)
	pdf.CellFormat(roleRWidth, 5, roleR, "", 0, "L", false, 0, "")
	// ==================================================================================

	if err := pdf.OutputFileAndClose(outputPath); err != nil {
		return err
	}

	record.ContractFilePath = outputPath
	if contractStart != nil {
		record.ContractStartDate = contractStart
	}
	if contractEnd != nil {
		record.ContractEndDate = contractEnd
	}
	if err := dbWeb.Save(&record).Error; err != nil {
		return err
	}

	return nil
}

func SendIndividualContractTechnician() gin.HandlerFunc {
	return func(c *gin.Context) {
		var request struct {
			ID         int    `json:"id" binding:"required"`
			SendOption string `json:"send_option" binding:"required"` // "email" or "whatsapp"
		}

		if err := c.ShouldBindJSON(&request); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}

		// Validate send option
		if request.SendOption != "email" && request.SendOption != "whatsapp" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid send_option. Must be 'email' or 'whatsapp'"})
			return
		}

		var record contracttechnicianmodel.ContractTechnicianODOO
		if err := dbWeb.First(&record, request.ID).Error; err != nil {
			c.JSON(http.StatusNotFound, gin.H{"error": fmt.Sprintf("data not found for id %d", request.ID)})
			return
		}

		if request.SendOption == "email" {
			successSend, err := sendContractToTechnician(request.SendOption, &record)
			if err != nil || !successSend {
				c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("failed to send contract via Email: %v", err)})
				return
			}

			c.JSON(http.StatusOK, gin.H{
				"message":   "Surat kontrak berhasil dikirim via Email !",
				"recipient": record.Email,
			})
		} else {
			successSend, err := sendContractToTechnician(request.SendOption, &record)
			if err != nil || !successSend {
				c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("failed to send contract via WhatsApp: %v", err)})
				return
			}

			c.JSON(http.StatusOK, gin.H{
				"message":   "Surat kontrak berhasil dikirim via WhatsApp !",
				"recipient": record.Phone,
			})
		}
	}
}

func sendContractToTechnician(sendOption string, record *contracttechnicianmodel.ContractTechnicianODOO) (bool, error) {
	loc, _ := time.LoadLocation(config.GetConfig().Default.Timezone)
	now := time.Now().In(loc)
	hour := now.Hour()
	// Greeting logic (ensuring correct 24-hour format)
	var greetingID, greetingEN string
	if hour >= 0 && hour < 4 {
		greetingID = "Selamat Dini Hari" // 00:00 - 03:59
		greetingEN = "Good Early Morning"
	} else if hour >= 4 && hour < 12 {
		greetingID = "Selamat Pagi" // 04:00 - 11:59
		greetingEN = "Good Morning"
	} else if hour >= 12 && hour < 15 {
		greetingID = "Selamat Siang" // 12:00 - 14:59
		greetingEN = "Good Afternoon"
	} else if hour >= 15 && hour < 17 {
		greetingID = "Selamat Sore" // 15:00 - 16:59
		greetingEN = "Good Late Afternoon"
	} else if hour >= 17 && hour < 19 {
		greetingID = "Selamat Petang" // 17:00 - 18:59
		greetingEN = "Good Evening"
	} else {
		greetingID = "Selamat Malam" // 19:00 - 23:59
		greetingEN = "Good Night"
	}

	dbWeb := gormdb.Databases.Web

	var namaTeknisi, noHPTeknisi, emailTeknisi string
	if record.Name != "" {
		namaTeknisi = record.Name
	} else {
		namaTeknisi = record.Technician
	}
	namaTeknisi = strings.ReplaceAll(namaTeknisi, "*", "(Resigned)")

	if record.Email != "" {
		emailTeknisi = record.Email
	}

	kontrakTerkirimTgl, err := tanggal.Papar(time.Now(), "Jakarta", tanggal.WIB)
	if err != nil {
		return false, err
	}
	kontrakTerkirimFormatted := kontrakTerkirimTgl.Format(" ", []tanggal.Format{
		tanggal.NamaHariDenganKoma,
		tanggal.Hari,
		tanggal.NamaBulan,
		tanggal.Tahun,
	})

	// Recreate the contract for finalData
	selectedMainDir, err := fun.FindValidDirectory([]string{
		"web/file/contract_technician",
		"../web/file/contract_technician",
		"../../web/file/contract_technician",
	})
	if err != nil {
		return false, err
	}
	pdfFileDir := filepath.Join(selectedMainDir, time.Now().Format("2006-01-02"))
	if err := os.MkdirAll(pdfFileDir, 0755); err != nil {
		return false, err
	}
	pdfFileName := fmt.Sprintf("Surat_Kontrak_%s.pdf", strings.ReplaceAll(namaTeknisi, " ", "_"))
	pdfFilePath := filepath.Join(pdfFileDir, pdfFileName)
	err = GeneratePDFContractTechnician(record, pdfFilePath)
	if err != nil {
		return false, err
	}
	record.ContractFilePath = pdfFilePath
	if err := dbWeb.Save(&record).Error; err != nil {
		return false, err
	}

	switch strings.ToLower(sendOption) {
	case "email":
		if emailTeknisi == "" {
			return false, errors.New("email cannot be empty")
		}

		CCEmails := config.GetConfig().ContractTechnicianODOO.CCContractEmail

		if config.GetConfig().ContractTechnicianODOO.ActiveDebug {
			emailTeknisi = config.GetConfig().ContractTechnicianODOO.EmailTest
		}

		emailTemplate := createTemplateEmailForContractTechnician(greetingID, namaTeknisi, kontrakTerkirimFormatted)
		if emailTemplate == "" {
			return false, errors.New("empty template email")
		}

		var emailTo []string
		emailTo = append(emailTo, emailTeknisi)
		emailSubject := "Surat Kontrak " + kontrakTerkirimFormatted
		emailAttachments := []fun.EmailAttachment{
			{
				FilePath:    record.ContractFilePath,
				NewFileName: "Surat_Kontrak_" + namaTeknisi + ".pdf",
			},
		}

		err := fun.TrySendEmail(
			emailTo,
			CCEmails,
			nil,
			emailSubject,
			emailTemplate,
			emailAttachments,
		)

		if err != nil {
			return false, err
		} else {
			now := time.Now()
			record.IsContractSent = true
			record.ContractSendAt = &now
			if err := dbWeb.Save(&record).Error; err != nil {
				return false, err
			}

			return true, nil
		}
	case "whatsapp":
		if record.Phone != "" {
			sanitizedPhone, err := fun.SanitizePhoneNumber(record.Phone)
			if err != nil {
				return false, err
			} else {
				if config.GetConfig().ContractTechnicianODOO.ActiveDebug {
					noHPTeknisi = config.GetConfig().ContractTechnicianODOO.PhoneNumberTest
				} else {
					noHPTeknisi = "62" + sanitizedPhone
				}
			}
		}

		jidStr := fmt.Sprintf("%s@s.whatsapp.net", noHPTeknisi)
		originalSenderJID := normalizeJID(jidStr)

		var sbID, sbEN strings.Builder
		sbID.WriteString(fmt.Sprintf("%s Bapak/Ibu %s, berikut kami lampirkan surat kontrak Anda per %s.\n\n", greetingID, namaTeknisi, kontrakTerkirimFormatted))
		sbID.WriteString("_Best Regards,_\n")
		sbID.WriteString(fmt.Sprintf("HR - *%s*", config.GetConfig().Default.PT))

		sbEN.WriteString(fmt.Sprintf("%s Mr/Mrs %s, please find attached your contract letter as of %s.\n\n", greetingEN, namaTeknisi, kontrakTerkirimFormatted))
		sbEN.WriteString("_Best Regards,_\n")
		sbEN.WriteString(fmt.Sprintf("HR - *%s*", config.GetConfig().Default.PT))
		msgID := sbID.String()
		msgEN := sbEN.String()

		sendLangDocumentMessageForContractTechnician(record.ForProject, record.Technician, originalSenderJID, msgID, msgEN, "id", record.ContractFilePath)

		return true, nil
	default:
		return false, fmt.Errorf("invalid send option: %s", sendOption)
	}
}

func createTemplateEmailForContractTechnician(greeting, namaTeknisi, kontrakTerkirimFormatted string) string {
	if greeting == "" || namaTeknisi == "" || kontrakTerkirimFormatted == "" {
		return ""
	}

	var sb strings.Builder
	sb.WriteString("<mjml>")
	sb.WriteString(`
		<mj-head>
			<mj-preview>Surat Kontrak . . .</mj-preview>
			<mj-style inline="inline">
			.body-section {
				background-color: #ffffff;
				padding: 30px;
				border-radius: 12px;
				box-shadow: 0 2px 8px rgba(0, 0, 0, 0.08);
			}
			.footer-text {
				color: #6b7280;
				font-size: 12px;
				text-align: center;
				padding-top: 10px;
				border-top: 1px solid #e5e7eb;
			}
			.header-title {
				font-size: 66px;
				font-weight: bold;
				color: #1E293B;
				text-align: left;
			}
			.cta-button {
				background-color: #6D28D9;
				color: #ffffff;
				padding: 12px 24px;
				border-radius: 8px;
				font-size: 16px;
				font-weight: bold;
				text-align: center;
				display: inline-block;
			}
			.email-info {
				color: #374151;
				font-size: 16px;
			}
			</mj-style>
		</mj-head>`)

	sb.WriteString(fmt.Sprintf(`
		<mj-body background-color="#f8fafc">
			<!-- Main Content -->
			<mj-section css-class="body-section" padding="20px">
			<mj-column>
				<mj-text font-size="20px" color="#1E293B" font-weight="bold">Yth. Sdr(i) %s</mj-text>
				<mj-text font-size="16px" color="#4B5563" line-height="1.6">
					%s, berikut kami lampirkan surat kontrak Anda per %s.
				</mj-text>

				<mj-divider border-color="#e5e7eb"></mj-divider>

				<mj-text font-size="16px" color="#374151">
				Best Regards,<br>
				<b><i>%s</i></b>
				</mj-text>
			</mj-column>
			</mj-section>

			<!-- Footer -->
			<mj-section>
			<mj-column>
				<mj-text css-class="footer-text">
				⚠ This is an automated email. Please do not reply directly.
				</mj-text>
				<mj-text font-size="12px" color="#6b7280">
				<b>HR - %s.</b><br>
				<!--
				<br>
				<a href="wa.me/%v">
				📞 Support
				</a>
				-->
				</mj-text>
			</mj-column>
			</mj-section>

		</mj-body>
		`,
		strings.ToUpper(namaTeknisi),
		greeting,
		kontrakTerkirimFormatted,
		config.GetConfig().Default.PTHRD[0].Name,
		config.GetConfig().Default.PT,
		"+6287883507445",
	))
	sb.WriteString("</mjml>")

	mjmlTemplate := sb.String()

	return mjmlTemplate
}

func GetContractTechnicianWhatsAppConversation() gin.HandlerFunc {
	return func(c *gin.Context) {
		var request struct {
			ID uint `json:"id" binding:"required"`
		}

		if err := c.ShouldBindJSON(&request); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}

		dbWeb := gormdb.Databases.Web

		var record contracttechnicianmodel.ContractTechnicianODOO
		if err := dbWeb.First(&record, request.ID).Error; err != nil {
			c.JSON(http.StatusNotFound, gin.H{"error": fmt.Sprintf("record not found for ID %d", request.ID)})
			return
		}

		// Check if WhatsApp conversation exists
		if record.WhatsappChatID == "" {
			c.JSON(http.StatusNotFound, gin.H{"error": "No WhatsApp conversation found"})
			return
		}

		var destinationName string
		if record.Name != "" {
			destinationName = record.Name
		} else {
			destinationName = record.Technician
		}

		// Format the conversation as an array (similar to SP WhatsApp modal format)
		conversation := []gin.H{
			{
				"whatsapp_message_body":    record.WhatsappMessageBody,
				"whatsapp_message_sent_to": record.Phone,
				"destination_name":         destinationName,
				"whatsapp_sent_at":         record.WhatsappSentAt,
				"whatsapp_reply_text":      record.WhatsappReplyText,
				"whatsapp_replied_by":      record.WhatsappRepliedBy,
				"sender_name":              destinationName,
				"whatsapp_replied_at":      record.WhatsappRepliedAt,
			},
		}

		c.JSON(http.StatusOK, gin.H{
			"success":      true,
			"conversation": conversation,
		})

	}
}

func SendAllContractTechnician() gin.HandlerFunc {
	return func(c *gin.Context) {
		var request struct {
			SendOption string `json:"send_option" binding:"required"` // "email" or "whatsapp"
		}

		if err := c.ShouldBindJSON(&request); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}

		// Validate send option
		if request.SendOption != "email" && request.SendOption != "whatsapp" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid send_option. Must be 'email' or 'whatsapp'"})
			return
		}

		dbWeb := gormdb.Databases.Web
		allSuccessSend, msg, totalSent, successLogs, failedLogs := sendAllContractsToTechnicians(request.SendOption, dbWeb)
		if !allSuccessSend {
			c.JSON(http.StatusInternalServerError, gin.H{"error": msg})
			return
		}

		c.JSON(http.StatusOK, gin.H{
			"message":      msg,
			"total_sent":   totalSent,
			"success_logs": successLogs,
			"failed_logs":  failedLogs,
		})
	}
}

func sendAllContractsToTechnicians(sendOption string, db *gorm.DB) (bool, string, int, []string, []string) {
	if db == nil {
		return false, "database connection is nil", 0, nil, nil
	} else {
		successLogs := []string{}
		failedLogs := []string{}
		totalSent := 0
		batchSize := 100
		offset := 0

		for {
			var records []contracttechnicianmodel.ContractTechnicianODOO
			result := db.Where("is_contract_sent = ?", false).
				Limit(batchSize).
				Offset(offset).
				Find(&records)

			if result.Error != nil {
				if errors.Is(result.Error, gorm.ErrRecordNotFound) {
					break
				}
				return false, fmt.Sprintf("failed to fetch records: %v", result.Error), totalSent, successLogs, failedLogs
			}

			// No more records to process
			if len(records) == 0 {
				break
			}

			for _, record := range records {
				successSend, err := sendContractToTechnician(sendOption, &record)
				if err != nil || !successSend {
					failedLogs = append(failedLogs, fmt.Sprintf("Failed to send contract to %s (ID: %d): %v", record.Technician, record.ID, err))
				} else {
					successLogs = append(successLogs, fmt.Sprintf("Successfully sent contract to %s (ID: %d)", record.Technician, record.ID))
					totalSent++
				}
			}

			// Move to next batch
			offset += batchSize

			// If we got fewer records than batch size, we've reached the end
			if len(records) < batchSize {
				break
			}
		}

		if totalSent == 0 && len(failedLogs) == 0 {
			return true, "all contracts have been sent", 0, nil, nil
		}

		return true,
			fmt.Sprintf("All contracts to technicians is successfully sent via %s", sendOption),
			totalSent, successLogs, failedLogs
	}
}

func NotifyHRDBeforeContractExpired(expiredDaysIn int) error {
	dbWeb := gormdb.Databases.Web
	now := time.Now()
	var upcomingRecords, expiredRecords []contracttechnicianmodel.ContractTechnicianODOO

	// Query for upcoming expiring contracts
	upcomingResult := dbWeb.Where("contract_end_date IS NOT NULL AND contract_end_date > ? AND contract_end_date <= ?", now, now.AddDate(0, 0, expiredDaysIn)).
		Where("is_notified = ?", false).
		Find(&upcomingRecords)

	// Query for already expired contracts
	expiredResult := dbWeb.Where("contract_end_date IS NOT NULL AND contract_end_date < ?", now).
		Where("is_notified = ?", false).
		Find(&expiredRecords)

	if upcomingResult.Error != nil && !errors.Is(upcomingResult.Error, gorm.ErrRecordNotFound) {
		return upcomingResult.Error
	}
	if expiredResult.Error != nil && !errors.Is(expiredResult.Error, gorm.ErrRecordNotFound) {
		return expiredResult.Error
	}

	var sbID, sbEN strings.Builder

	// Upcoming section
	if len(upcomingRecords) > 0 {
		sbID.WriteString(fmt.Sprintf("[%d] Kontrak teknisi yang akan berakhir dalam %d hari ke depan:\n\n", len(upcomingRecords), expiredDaysIn))
		sbEN.WriteString(fmt.Sprintf("[%d] Technician contracts expiring in the next %d days:\n\n", len(upcomingRecords), expiredDaysIn))

		for i, record := range upcomingRecords {
			var namaTeknisi string
			if record.Name != "" {
				namaTeknisi = record.Name
			} else {
				namaTeknisi = record.Technician
			}

			contractStartStr := "N/A"
			contractEndStr := "N/A"
			if record.ContractStartDate != nil {
				contractStartStr = record.ContractStartDate.Format("02 Jan 2006")
			}
			if record.ContractEndDate != nil {
				contractEndStr = record.ContractEndDate.Format("02 Jan 2006")
			}

			sbID.WriteString(fmt.Sprintf("%d) %s - Periode: %s s/d %s\n", i+1, namaTeknisi, contractStartStr, contractEndStr))
			if record.SAC != "" {
				sbID.WriteString(fmt.Sprintf("   SAC: %s\n", record.SAC))
			}
			if record.SPL != "" {
				sbID.WriteString(fmt.Sprintf("   SPL: %s\n", record.SPL))
			}
			if record.Technician != "" {
				sbID.WriteString(fmt.Sprintf("   Name FS: %s\n", record.Technician))
			}

			sbEN.WriteString(fmt.Sprintf("%d) %s - Period: %s to %s\n", i+1, namaTeknisi, contractStartStr, contractEndStr))
			if record.SAC != "" {
				sbEN.WriteString(fmt.Sprintf("   SAC: %s\n", record.SAC))
			}
			if record.SPL != "" {
				sbEN.WriteString(fmt.Sprintf("   SPL: %s\n", record.SPL))
			}
			if record.Technician != "" {
				sbEN.WriteString(fmt.Sprintf("   Name FS: %s\n", record.Technician))
			}
		}
		sbID.WriteString("\n")
		sbEN.WriteString("\n")
	} else {
		// sbID.WriteString(fmt.Sprintf("Tidak ada kontrak yang akan berakhir dalam %d hari ke depan.\n\n", expiredDaysIn))
		// sbEN.WriteString(fmt.Sprintf("No contracts expiring in the next %d days.\n\n", expiredDaysIn))
	}

	// Expired section
	if len(expiredRecords) > 0 {
		sbID.WriteString(fmt.Sprintf("Kontrak teknisi yang sudah berakhir dan belum diperbarui (%d):\n\n", len(expiredRecords)))
		sbEN.WriteString(fmt.Sprintf("Technician contracts that have already expired and not updated (%d):\n\n", len(expiredRecords)))

		for i, record := range expiredRecords {
			var namaTeknisi string
			if record.Name != "" {
				namaTeknisi = record.Name
			} else {
				namaTeknisi = record.Technician
			}

			contractStartStr := "N/A"
			contractEndStr := "N/A"
			if record.ContractStartDate != nil {
				contractStartStr = record.ContractStartDate.Format("02 Jan 2006")
			}
			if record.ContractEndDate != nil {
				contractEndStr = record.ContractEndDate.Format("02 Jan 2006")
			}

			sbID.WriteString(fmt.Sprintf("%d) %s - Periode: %s s/d %s\n", i+1, namaTeknisi, contractStartStr, contractEndStr))
			if record.SAC != "" {
				sbID.WriteString(fmt.Sprintf("   SAC: %s\n", record.SAC))
			}
			if record.SPL != "" {
				sbID.WriteString(fmt.Sprintf("   SPL: %s\n", record.SPL))
			}
			if record.Technician != "" {
				sbID.WriteString(fmt.Sprintf("   Name FS: %s\n", record.Technician))
			}

			sbEN.WriteString(fmt.Sprintf("%d) %s - Period: %s to %s\n", i+1, namaTeknisi, contractStartStr, contractEndStr))
			if record.SAC != "" {
				sbEN.WriteString(fmt.Sprintf("   SAC: %s\n", record.SAC))
			}
			if record.SPL != "" {
				sbEN.WriteString(fmt.Sprintf("   SPL: %s\n", record.SPL))
			}
			if record.Technician != "" {
				sbEN.WriteString(fmt.Sprintf("   Name FS: %s\n", record.Technician))
			}
		}
	} else {
		// sbID.WriteString("Tidak ada kontrak yang sudah berakhir dan belum diperbarui.\n")
		// sbEN.WriteString("No expired contracts that are not updated.\n")
	}

	jidStrHRD := fmt.Sprintf("%s@s.whatsapp.net", config.GetConfig().Default.PTHRD[0].PhoneNumber)
	originalSenderJID := NormalizeSenderJID(jidStrHRD)

	msgID := sbID.String()
	msgEN := sbEN.String()
	if len(msgID) > 0 && len(msgEN) > 0 {
		SendLangMessage(originalSenderJID, sbID.String(), sbEN.String(), "id")

		// Update is_notified to true for all notified records
		var allRecordsToUpdate []contracttechnicianmodel.ContractTechnicianODOO
		allRecordsToUpdate = append(allRecordsToUpdate, upcomingRecords...)
		allRecordsToUpdate = append(allRecordsToUpdate, expiredRecords...)

		if len(allRecordsToUpdate) > 0 {
			var ids []uint
			for _, record := range allRecordsToUpdate {
				ids = append(ids, record.ID)
			}

			if err := dbWeb.Model(&contracttechnicianmodel.ContractTechnicianODOO{}).
				Where("id IN ?", ids).
				Update("is_notified", true).Error; err != nil {
				logrus.Errorf("Failed to update is_notified for contract records: %v", err)
				return err
			}
			logrus.Infof("Updated is_notified to true for %d contract records", len(allRecordsToUpdate))
		}
	}

	return nil
}
