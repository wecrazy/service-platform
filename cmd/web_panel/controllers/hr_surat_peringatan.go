package controllers

import (
	"encoding/json"
	"fmt"
	"net/http"
	"path/filepath"
	"reflect"
	"regexp"
	"service-platform/cmd/web_panel/fun"
	"service-platform/cmd/web_panel/internal/gormdb"
	"service-platform/cmd/web_panel/model"
	sptechnicianmodel "service-platform/cmd/web_panel/model/sp_technician_model"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/sirupsen/logrus"
	"github.com/xuri/excelize/v2"
	"gorm.io/gorm"
)

func TableSuratPeringatanTechnicianForHR() gin.HandlerFunc {
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
		t := reflect.TypeOf(sptechnicianmodel.TechnicianGotSP{})

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
		filteredQuery := dbWeb.Model(&sptechnicianmodel.TechnicianGotSP{})

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

		// Count the total number of records
		var totalRecords int64
		dbWeb.Model(&sptechnicianmodel.TechnicianGotSP{}).Count(&totalRecords)

		// Count the number of filtered records
		var filteredRecords int64
		filteredQuery.Count(&filteredRecords)

		// Apply sorting and pagination to the filtered query
		query := filteredQuery.Order(orderString)
		var Dbdata []sptechnicianmodel.TechnicianGotSP
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
						fileSP := strings.ReplaceAll(t, "web/file/sp_technician/", "")
						fileSPURL := "/proxy-pdf-sp-technician/" + fileSP
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
						fileTTS := strings.ReplaceAll(t, "web/file/sounding_sp_technician/", "")
						fileTTSURL := "/proxy-mp3-sp-technician/" + fileTTS
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
					var messages []sptechnicianmodel.SPWhatsAppMessage

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
						Where("what_sp = ?", "SP_TECHNICIAN").
						Where("for_project = ?", dataInDB.ForProject).
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

func TableSuratPeringatanSPLForHR() gin.HandlerFunc {
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
		t := reflect.TypeOf(sptechnicianmodel.SPLGotSP{})

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
		filteredQuery := dbWeb.Model(&sptechnicianmodel.SPLGotSP{})

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

		// Count the total number of records
		var totalRecords int64
		dbWeb.Model(&sptechnicianmodel.SPLGotSP{}).Count(&totalRecords)

		// Count the number of filtered records
		var filteredRecords int64
		filteredQuery.Count(&filteredRecords)

		// Apply sorting and pagination to the filtered query
		query := filteredQuery.Order(orderString)
		var Dbdata []sptechnicianmodel.SPLGotSP
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

				case "spl":
					t := fieldValue.Interface().(string)
					var technician string
					if t == "" {
						technician = "<span class='text-danger'>N/A</span>"
					} else {
						technician = fmt.Sprintf("<span class='badge bg-primary'><i class='fal fa-user me-2'></i>%s</span>", t)
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
						fileSP := strings.ReplaceAll(t, "web/file/sp_spl/", "")
						fileSPURL := "/proxy-pdf-sp-spl/" + fileSP
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
						fileTTS := strings.ReplaceAll(t, "web/file/sounding_sp_spl/", "")
						fileTTSURL := "/proxy-mp3-sp-spl/" + fileTTS
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
					var messages []sptechnicianmodel.SPWhatsAppMessage

					// Extract SP number from theKey (e.g., "sp1_whatsapp_messages" -> 1)
					spNumberStr := strings.TrimSuffix(strings.TrimPrefix(theKey, "sp"), "_whatsapp_messages")
					spNumber, err := strconv.Atoi(spNumberStr)
					if err != nil {
						logrus.Errorf("failed to parse SP number from key %s: %v", theKey, err)
						newData[theKey] = "<span class='text-danger'>Invalid Key</span>"
						continue
					}

					if err := dbWeb.
						Where("spl_got_sp_id = ?", dataInDB.ID).
						Where("number_of_sp = ?", spNumber).
						Where("what_sp = ?", "SP_SPL").
						Where("for_project = ?", dataInDB.ForProject).
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

func TableSuratPeringatanSACForHR() gin.HandlerFunc {
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
		t := reflect.TypeOf(sptechnicianmodel.SACGotSP{})

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
		filteredQuery := dbWeb.Model(&sptechnicianmodel.SACGotSP{})

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

		// Count the total number of records
		var totalRecords int64
		dbWeb.Model(&sptechnicianmodel.SACGotSP{}).Count(&totalRecords)

		// Count the number of filtered records
		var filteredRecords int64
		filteredQuery.Count(&filteredRecords)

		// Apply sorting and pagination to the filtered query
		query := filteredQuery.Order(orderString)
		var Dbdata []sptechnicianmodel.SACGotSP
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

				case "sac":
					t := fieldValue.Interface().(string)
					var technician string
					if t == "" {
						technician = "<span class='text-danger'>N/A</span>"
					} else {
						technician = fmt.Sprintf("<span class='badge bg-danger'><i class='fal fa-user-tie me-2'></i>%s</span>", t)
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
						fileSP := strings.ReplaceAll(t, "web/file/sp_sac/", "")
						fileSPURL := "/proxy-pdf-sp-sac/" + fileSP
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
						fileTTS := strings.ReplaceAll(t, "web/file/sounding_sp_sac/", "")
						fileTTSURL := "/proxy-mp3-sp-sac/" + fileTTS
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
					var messages []sptechnicianmodel.SPWhatsAppMessage

					// Extract SP number from theKey (e.g., "sp1_whatsapp_messages" -> 1)
					spNumberStr := strings.TrimSuffix(strings.TrimPrefix(theKey, "sp"), "_whatsapp_messages")
					spNumber, err := strconv.Atoi(spNumberStr)
					if err != nil {
						logrus.Errorf("failed to parse SP number from key %s: %v", theKey, err)
						newData[theKey] = "<span class='text-danger'>Invalid Key</span>"
						continue
					}

					if err := dbWeb.
						Where("sac_got_sp_id = ?", dataInDB.ID).
						Where("number_of_sp = ?", spNumber).
						Where("what_sp = ?", "SP_SAC").
						Where("for_project = ?", dataInDB.ForProject).
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

func DeleteSuratPeringatanTechnician() gin.HandlerFunc {
	return func(c *gin.Context) {
		// Get the ID from the URL parameter and convert to integer
		idParam := c.Param("id")
		id, err := strconv.Atoi(idParam)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid Data"})
			return
		}

		// Find the record by ID
		var dbData sptechnicianmodel.TechnicianGotSP
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

		// 1. Delete associated SPWhatsAppMessage records
		if err := tx.Where("technician_got_sp_id = ?", dbData.ID).
			Where("what_sp = ?", "SP_TECHNICIAN").
			Where("for_project = ?", "ODOO MS").
			Delete(&sptechnicianmodel.SPWhatsAppMessage{}).Error; err != nil {
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

func DeleteSuratPeringatanSPL() gin.HandlerFunc {
	return func(c *gin.Context) {
		// Get the ID from the URL parameter and convert to integer
		idParam := c.Param("id")
		id, err := strconv.Atoi(idParam)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid Data"})
			return
		}

		if id == 1 {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "You cannot delete this data!!"})
			return
		}

		// Find the record by ID
		var dbData sptechnicianmodel.SPLGotSP
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

		if dbData.IsGotSP3 {
			c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("Data %s - %s has received SP3, cannot be deleted", dbData.SPL, dbData.Name)})
			return
		}

		// Use a transaction to ensure atomicity
		tx := dbWeb.Begin()
		if tx.Error != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to start transaction, details: " + tx.Error.Error()})
			return
		}

		// 1. Delete associated SPWhatsAppMessage records
		if err := tx.Where("technician_got_sp_id = ?", dbData.ID).
			Where("what_sp = ?", "SP_SPL").
			Where("for_project = ?", dbData.ForProject).
			Delete(&sptechnicianmodel.SPWhatsAppMessage{}).Error; err != nil {
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

func DeleteSuratPeringatanSAC() gin.HandlerFunc {
	return func(c *gin.Context) {
		// Get the ID from the URL parameter and convert to integer
		idParam := c.Param("id")
		id, err := strconv.Atoi(idParam)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid Data"})
			return
		}

		if id == 1 {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "You cannot delete this data!!"})
			return
		}

		// Find the record by ID
		var dbData sptechnicianmodel.SACGotSP
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
			c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("Data %s - %s has received SP3, cannot be deleted", dbData.SAC, dbData.Name)})
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

		// 1. Delete associated SPWhatsAppMessage records
		if err := tx.Where("technician_got_sp_id = ?", dbData.ID).
			Where("what_sp = ?", "SP_SAC").
			Where("for_project = ?", dbData.ForProject).
			Delete(&sptechnicianmodel.SPWhatsAppMessage{}).Error; err != nil {
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

// DeleteAllSPTechnician - Delete all SP records for Technicians
func DeleteAllSPTechnician() gin.HandlerFunc {
	return func(c *gin.Context) {
		cookies := c.Request.Cookies()
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
		allowedUsersToDelete := []string{"developer", "admin", "human resource", "csna"}
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
			c.JSON(http.StatusForbidden, gin.H{"error": "You are not authorized to delete this data"})
			return
		}
		dbWeb := gormdb.Databases.Web
		var count int64
		if err := dbWeb.Model(&sptechnicianmodel.TechnicianGotSP{}).Count(&count).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"success": false, "message": "Failed to count records: " + err.Error()})
			return
		}
		if count == 0 {
			c.JSON(http.StatusOK, gin.H{"success": false, "message": "No records to delete", "deleted_count": 0})
			return
		}
		// // Check if any records have received SP3
		// var sp3Count int64
		// if err := dbWeb.Model(&sptechnicianmodel.TechnicianGotSP{}).Where("is_got_sp3 = ?", true).Count(&sp3Count).Error; err != nil {
		// 	c.JSON(http.StatusInternalServerError, gin.H{"success": false, "message": "Failed to check SP3 records: " + err.Error()})
		// 	return
		// }
		// if sp3Count > 0 {
		// 	c.JSON(http.StatusForbidden, gin.H{"error": fmt.Sprintf("Cannot delete all records: %d technician(s) have received SP3 and cannot be deleted", sp3Count)})
		// 	return
		// }
		tx := dbWeb.Begin()
		if tx.Error != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to start transaction, details: " + tx.Error.Error()})
			return
		}
		if err := tx.Where("what_sp = ?", "SP_TECHNICIAN").Where("for_project = ?", "ODOO MS").Delete(&sptechnicianmodel.SPWhatsAppMessage{}).Error; err != nil {
			tx.Rollback()
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to delete associated WhatsApp messages, details: " + err.Error()})
			return
		}
		// Delete all technician SP records using GORM
		if err := tx.Where("1=1").Delete(&sptechnicianmodel.TechnicianGotSP{}).Error; err != nil {
			tx.Rollback()
			c.JSON(http.StatusInternalServerError, gin.H{"success": false, "message": "Failed to delete records: " + err.Error()})
			return
		}
		if err := tx.Commit().Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to commit transaction, details: " + err.Error()})
			return
		}
		logrus.Infof("Deleted %d SP Technician records by %s", count, userFullName)
		dbWeb.Create(&model.LogActivity{AdminID: userID, FullName: userFullName, Action: "Delete All Data", Status: "Success", Log: fmt.Sprintf("Deleted %d SP Technician records", count), IP: c.ClientIP(), UserAgent: c.Request.UserAgent(), ReqMethod: c.Request.Method, ReqUri: c.Request.RequestURI})
		c.JSON(http.StatusOK, gin.H{"success": true, "message": fmt.Sprintf("Successfully deleted %d SP Technician records", count), "deleted_count": count})
	}
}

// DeleteAllSPSPL - Delete all SP records for SPL
func DeleteAllSPSPL() gin.HandlerFunc {
	return func(c *gin.Context) {
		cookies := c.Request.Cookies()
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
		allowedUsersToDelete := []string{"developer", "admin", "human resource", "csna"}
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
			c.JSON(http.StatusForbidden, gin.H{"error": "You are not authorized to delete this data"})
			return
		}
		dbWeb := gormdb.Databases.Web
		var count int64
		if err := dbWeb.Model(&sptechnicianmodel.SPLGotSP{}).Count(&count).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"success": false, "message": "Failed to count records: " + err.Error()})
			return
		}
		if count == 0 {
			c.JSON(http.StatusOK, gin.H{"success": false, "message": "No records to delete", "deleted_count": 0})
			return
		}
		// // Check if any records have received SP3
		// var sp3Count int64
		// if err := dbWeb.Model(&sptechnicianmodel.SPLGotSP{}).Where("is_got_sp3 = ?", true).Count(&sp3Count).Error; err != nil {
		// 	c.JSON(http.StatusInternalServerError, gin.H{"success": false, "message": "Failed to check SP3 records: " + err.Error()})
		// 	return
		// }
		// if sp3Count > 0 {
		// 	c.JSON(http.StatusForbidden, gin.H{"error": fmt.Sprintf("Cannot delete all records: %d SPL(s) have received SP3 and cannot be deleted", sp3Count)})
		// 	return
		// }
		tx := dbWeb.Begin()
		if tx.Error != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to start transaction, details: " + tx.Error.Error()})
			return
		}
		if err := tx.Where("what_sp = ?", "SP_SPL").Where("for_project = ?", "ODOO MS").Delete(&sptechnicianmodel.SPWhatsAppMessage{}).Error; err != nil {
			tx.Rollback()
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to delete associated WhatsApp messages, details: " + err.Error()})
			return
		}
		// Delete all SPL SP records using GORM
		if err := tx.Where("1=1").Delete(&sptechnicianmodel.SPLGotSP{}).Error; err != nil {
			tx.Rollback()
			c.JSON(http.StatusInternalServerError, gin.H{"success": false, "message": "Failed to delete records: " + err.Error()})
			return
		}
		if err := tx.Commit().Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to commit transaction, details: " + err.Error()})
			return
		}
		logrus.Infof("Deleted %d SP SPL records by %s", count, userFullName)
		dbWeb.Create(&model.LogActivity{AdminID: userID, FullName: userFullName, Action: "Delete All Data", Status: "Success", Log: fmt.Sprintf("Deleted %d SP SPL records", count), IP: c.ClientIP(), UserAgent: c.Request.UserAgent(), ReqMethod: c.Request.Method, ReqUri: c.Request.RequestURI})
		c.JSON(http.StatusOK, gin.H{"success": true, "message": fmt.Sprintf("Successfully deleted %d SP SPL records", count), "deleted_count": count})
	}
}

// DeleteAllSPSAC - Delete all SP records for SAC
func DeleteAllSPSAC() gin.HandlerFunc {
	return func(c *gin.Context) {
		cookies := c.Request.Cookies()
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
		allowedUsersToDelete := []string{"developer", "admin", "human resource", "csna"}
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
			c.JSON(http.StatusForbidden, gin.H{"error": "You are not authorized to delete this data"})
			return
		}
		dbWeb := gormdb.Databases.Web
		var count int64
		if err := dbWeb.Model(&sptechnicianmodel.SACGotSP{}).Count(&count).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"success": false, "message": "Failed to count records: " + err.Error()})
			return
		}
		if count == 0 {
			c.JSON(http.StatusOK, gin.H{"success": false, "message": "No records to delete", "deleted_count": 0})
			return
		}
		// // Check if any records have received SP3
		// var sp3Count int64
		// if err := dbWeb.Model(&sptechnicianmodel.SACGotSP{}).Where("is_got_sp3 = ?", true).Count(&sp3Count).Error; err != nil {
		// 	c.JSON(http.StatusInternalServerError, gin.H{"success": false, "message": "Failed to check SP3 records: " + err.Error()})
		// 	return
		// }
		// if sp3Count > 0 {
		// 	c.JSON(http.StatusForbidden, gin.H{"error": fmt.Sprintf("Cannot delete all records: %d SAC(s) have received SP3 and cannot be deleted", sp3Count)})
		// 	return
		// }
		tx := dbWeb.Begin()
		if tx.Error != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to start transaction, details: " + tx.Error.Error()})
			return
		}
		if err := tx.Where("what_sp = ?", "SP_SAC").Where("for_project = ?", "ODOO MS").Delete(&sptechnicianmodel.SPWhatsAppMessage{}).Error; err != nil {
			tx.Rollback()
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to delete associated WhatsApp messages, details: " + err.Error()})
			return
		}
		// Delete all SAC SP records using GORM
		if err := tx.Where("1=1").Delete(&sptechnicianmodel.SACGotSP{}).Error; err != nil {
			tx.Rollback()
			c.JSON(http.StatusInternalServerError, gin.H{"success": false, "message": "Failed to delete records: " + err.Error()})
			return
		}
		if err := tx.Commit().Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to commit transaction, details: " + err.Error()})
			return
		}
		logrus.Infof("Deleted %d SP SAC records by %s", count, userFullName)
		dbWeb.Create(&model.LogActivity{AdminID: userID, FullName: userFullName, Action: "Delete All Data", Status: "Success", Log: fmt.Sprintf("Deleted %d SP SAC records", count), IP: c.ClientIP(), UserAgent: c.Request.UserAgent(), ReqMethod: c.Request.Method, ReqUri: c.Request.RequestURI})
		c.JSON(http.StatusOK, gin.H{"success": true, "message": fmt.Sprintf("Successfully deleted %d SP SAC records", count), "deleted_count": count})
	}
}

// GetSPSACGroups returns a list of SAC groups with SP records that need reply
func GetSPSACGroups() gin.HandlerFunc {
	return func(c *gin.Context) {
		dbWeb := gormdb.Databases.Web

		// Map to store unique SAC groups with counts
		sacGroups := make(map[string]int)

		// Count Technician SPs needing reply
		var techSPs []sptechnicianmodel.TechnicianGotSP
		dbWeb.Find(&techSPs)
		for _, sp := range techSPs {
			spNumbers := []int{}
			if sp.NoSP1 > 0 {
				spNumbers = append(spNumbers, 1)
			}
			if sp.NoSP2 > 0 {
				spNumbers = append(spNumbers, 2)
			}
			if sp.NoSP3 > 0 {
				spNumbers = append(spNumbers, 3)
			}

			for _, spNum := range spNumbers {
				var msgCount int64
				dbWeb.Model(&sptechnicianmodel.SPWhatsAppMessage{}).
					Where("technician_got_sp_id = ?", sp.ID).
					Where("number_of_sp = ?", spNum).
					Where("what_sp = ?", "SP_TECHNICIAN").
					Where("(whatsapp_reply_text IS NULL OR whatsapp_reply_text = '')").
					Count(&msgCount)

				if msgCount > 0 {
					sacName := ""
					if techData, exists := TechODOOMSData[sp.Technician]; exists {
						sacName = techData.SAC
					}
					if sacName == "" {
						sacName = "(Kosong)"
					}
					sacGroups[sacName]++
				}
			}
		}

		// Count SPL SPs needing reply
		var splSPs []sptechnicianmodel.SPLGotSP
		dbWeb.Find(&splSPs)
		for _, sp := range splSPs {
			spNumbers := []int{}
			if sp.NoSP1 > 0 {
				spNumbers = append(spNumbers, 1)
			}
			if sp.NoSP2 > 0 {
				spNumbers = append(spNumbers, 2)
			}
			if sp.NoSP3 > 0 {
				spNumbers = append(spNumbers, 3)
			}

			for _, spNum := range spNumbers {
				var msgCount int64
				dbWeb.Model(&sptechnicianmodel.SPWhatsAppMessage{}).
					Where("spl_got_sp_id = ?", sp.ID).
					Where("number_of_sp = ?", spNum).
					Where("what_sp = ?", "SP_SPL").
					Where("(whatsapp_reply_text IS NULL OR whatsapp_reply_text = '')").
					Count(&msgCount)

				if msgCount > 0 {
					sacName := ""
					for techCode, techData := range TechODOOMSData {
						if techData.SPL == sp.SPL {
							sacName = techData.SAC
							break
						}
						_ = techCode
					}
					if sacName == "" {
						sacName = "(Kosong)"
					}
					sacGroups[sacName]++
				}
			}
		}

		// Count SAC SPs needing reply
		var sacSPs []sptechnicianmodel.SACGotSP
		dbWeb.Find(&sacSPs)
		for _, sp := range sacSPs {
			spNumbers := []int{}
			if sp.NoSP1 > 0 {
				spNumbers = append(spNumbers, 1)
			}
			if sp.NoSP2 > 0 {
				spNumbers = append(spNumbers, 2)
			}
			if sp.NoSP3 > 0 {
				spNumbers = append(spNumbers, 3)
			}

			for _, spNum := range spNumbers {
				var msgCount int64
				dbWeb.Model(&sptechnicianmodel.SPWhatsAppMessage{}).
					Where("sac_got_sp_id = ?", sp.ID).
					Where("number_of_sp = ?", spNum).
					Where("what_sp = ?", "SP_SAC").
					Where("(whatsapp_reply_text IS NULL OR whatsapp_reply_text = '')").
					Count(&msgCount)

				if msgCount > 0 {
					sacName := sp.SAC
					if sacName == "" {
						sacName = "(Kosong)"
					}
					sacGroups[sacName]++
				}
			}
		}

		// Convert map to slice for JSON response
		type SACGroup struct {
			Name  string `json:"name"`
			Count int    `json:"count"`
		}

		groups := []SACGroup{}
		for sac, count := range sacGroups {
			groups = append(groups, SACGroup{Name: sac, Count: count})
		}

		c.JSON(http.StatusOK, gin.H{
			"success": true,
			"groups":  groups,
			"total":   len(groups),
		})
	}
}

// DownloadSPReplySimulationTemplate generates and downloads an Excel file with real SP data that needs to be replied
func DownloadSPReplySimulationTemplate() gin.HandlerFunc {
	return func(c *gin.Context) {
		dbWeb := gormdb.Databases.Web

		// Get SAC filter from query parameter
		sacFilter := c.Query("sac")
		if sacFilter == "(Kosong)" {
			sacFilter = ""
		}

		// Create new Excel file
		f := excelize.NewFile()
		sheetName := "Sheet1"
		f.SetSheetName("Sheet1", sheetName)

		// Define headers - added pelanggaran columns
		headers := []string{
			"nomor_surat_sp1",
			"nomor_surat_sp2",
			"nomor_surat_sp3",
			"pelanggaran_sp1",
			"pelanggaran_sp2",
			"pelanggaran_sp3",
			"sp_untuk",
			"sac",
			"balasan",
			"disanggah_pada",
		}

		// Create header style (green background, white text, bold)
		headerStyle, err := f.NewStyle(&excelize.Style{
			Font: &excelize.Font{
				Bold:  true,
				Color: "#FFFFFF",
				Size:  11,
			},
			Fill: excelize.Fill{
				Type:    "pattern",
				Color:   []string{"#35F101"},
				Pattern: 1,
			},
			Alignment: &excelize.Alignment{
				Horizontal: "center",
				Vertical:   "center",
			},
			Border: []excelize.Border{
				{Type: "left", Color: "#000000", Style: 1},
				{Type: "top", Color: "#000000", Style: 1},
				{Type: "bottom", Color: "#000000", Style: 1},
				{Type: "right", Color: "#000000", Style: 1},
			},
		})
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create header style: " + err.Error()})
			return
		}

		// Create data style
		dataStyle, err := f.NewStyle(&excelize.Style{
			Border: []excelize.Border{
				{Type: "left", Color: "#CCCCCC", Style: 1},
				{Type: "top", Color: "#CCCCCC", Style: 1},
				{Type: "bottom", Color: "#CCCCCC", Style: 1},
				{Type: "right", Color: "#CCCCCC", Style: 1},
			},
		})
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create data style: " + err.Error()})
			return
		}

		// Write headers
		for i, header := range headers {
			cell := fmt.Sprintf("%s1", string(rune('A'+i)))
			f.SetCellValue(sheetName, cell, header)
			f.SetCellStyle(sheetName, cell, cell, headerStyle)
		}

		// Struct to hold SP data that needs reply
		type SPNeedReply struct {
			NomorSuratSP1  int
			NomorSuratSP2  int
			NomorSuratSP3  int
			PelanggaranSP1 string
			PelanggaranSP2 string
			PelanggaranSP3 string
			SPUntuk        string
			SAC            string
			SPType         string
			SPNumber       int
		}

		var spList []SPNeedReply

		// Fetch Technician SP records that need reply
		var techSPs []sptechnicianmodel.TechnicianGotSP
		dbWeb.Find(&techSPs)
		for _, sp := range techSPs {
			// Get SAC name from TechODOOMSData
			sacName := ""
			if techData, exists := TechODOOMSData[sp.Technician]; exists {
				sacName = techData.SAC
			}

			// Apply SAC filter if specified
			if sacFilter != "" && sacName != sacFilter {
				continue
			}

			// Check each SP level for unreplied messages
			spNumbers := []int{}
			if sp.NoSP1 > 0 {
				spNumbers = append(spNumbers, 1)
			}
			if sp.NoSP2 > 0 {
				spNumbers = append(spNumbers, 2)
			}
			if sp.NoSP3 > 0 {
				spNumbers = append(spNumbers, 3)
			}

			for _, spNum := range spNumbers {
				// Check if this SP level has unreplied WhatsApp messages
				var msgCount int64
				dbWeb.Model(&sptechnicianmodel.SPWhatsAppMessage{}).
					Where("technician_got_sp_id = ?", sp.ID).
					Where("number_of_sp = ?", spNum).
					Where("what_sp = ?", "SP_TECHNICIAN").
					Where("(whatsapp_reply_text IS NULL OR whatsapp_reply_text = '')").
					Count(&msgCount)

				if msgCount > 0 {
					needReply := SPNeedReply{
						SPUntuk:        sp.Technician,
						SAC:            sacName,
						SPType:         "TECH",
						SPNumber:       spNum,
						PelanggaranSP1: sp.PelanggaranSP1,
						PelanggaranSP2: sp.PelanggaranSP2,
						PelanggaranSP3: sp.PelanggaranSP3,
					}
					switch spNum {
					case 1:
						needReply.NomorSuratSP1 = sp.NoSP1
					case 2:
						needReply.NomorSuratSP2 = sp.NoSP2
					case 3:
						needReply.NomorSuratSP3 = sp.NoSP3
					}
					spList = append(spList, needReply)
				}
			}
		}

		// Fetch SPL SP records that need reply
		var splSPs []sptechnicianmodel.SPLGotSP
		dbWeb.Find(&splSPs)
		for _, sp := range splSPs {
			// Get SAC name from TechODOOMSData by looking up SPL's technicians
			sacName := ""
			for techCode, techData := range TechODOOMSData {
				if techData.SPL == sp.SPL {
					sacName = techData.SAC
					break
				}
				_ = techCode // avoid unused variable
			}

			// Apply SAC filter if specified
			if sacFilter != "" && sacName != sacFilter {
				continue
			}

			spNumbers := []int{}
			if sp.NoSP1 > 0 {
				spNumbers = append(spNumbers, 1)
			}
			if sp.NoSP2 > 0 {
				spNumbers = append(spNumbers, 2)
			}
			if sp.NoSP3 > 0 {
				spNumbers = append(spNumbers, 3)
			}

			for _, spNum := range spNumbers {
				var msgCount int64
				dbWeb.Model(&sptechnicianmodel.SPWhatsAppMessage{}).
					Where("spl_got_sp_id = ?", sp.ID).
					Where("number_of_sp = ?", spNum).
					Where("what_sp = ?", "SP_SPL").
					Where("(whatsapp_reply_text IS NULL OR whatsapp_reply_text = '')").
					Count(&msgCount)

				if msgCount > 0 {
					needReply := SPNeedReply{
						SPUntuk:        sp.SPL,
						SAC:            sacName,
						SPType:         "SPL",
						SPNumber:       spNum,
						PelanggaranSP1: sp.PelanggaranSP1,
						PelanggaranSP2: sp.PelanggaranSP2,
						PelanggaranSP3: sp.PelanggaranSP3,
					}
					switch spNum {
					case 1:
						needReply.NomorSuratSP1 = sp.NoSP1
					case 2:
						needReply.NomorSuratSP2 = sp.NoSP2
					case 3:
						needReply.NomorSuratSP3 = sp.NoSP3
					}
					spList = append(spList, needReply)
				}
			}
		}

		// Fetch SAC SP records that need reply
		var sacSPs []sptechnicianmodel.SACGotSP
		dbWeb.Find(&sacSPs)
		for _, sp := range sacSPs {
			sacName := sp.SAC
			if sacName == "" {
				sacName = ""
			}

			// Apply SAC filter if specified
			if sacFilter != "" && sacName != sacFilter {
				continue
			}

			spNumbers := []int{}
			if sp.NoSP1 > 0 {
				spNumbers = append(spNumbers, 1)
			}
			if sp.NoSP2 > 0 {
				spNumbers = append(spNumbers, 2)
			}
			if sp.NoSP3 > 0 {
				spNumbers = append(spNumbers, 3)
			}

			for _, spNum := range spNumbers {
				var msgCount int64
				dbWeb.Model(&sptechnicianmodel.SPWhatsAppMessage{}).
					Where("sac_got_sp_id = ?", sp.ID).
					Where("number_of_sp = ?", spNum).
					Where("what_sp = ?", "SP_SAC").
					Where("(whatsapp_reply_text IS NULL OR whatsapp_reply_text = '')").
					Count(&msgCount)

				if msgCount > 0 {
					needReply := SPNeedReply{
						SPUntuk:        sp.SAC,
						SAC:            sp.SAC, // SAC is the identifier itself
						SPType:         "SAC",
						SPNumber:       spNum,
						PelanggaranSP1: sp.PelanggaranSP1,
						PelanggaranSP2: sp.PelanggaranSP2,
						PelanggaranSP3: sp.PelanggaranSP3,
					}
					switch spNum {
					case 1:
						needReply.NomorSuratSP1 = sp.NoSP1
					case 2:
						needReply.NomorSuratSP2 = sp.NoSP2
					case 3:
						needReply.NomorSuratSP3 = sp.NoSP3
					}
					spList = append(spList, needReply)
				}
			}
		}

		// Check if there's any data to export
		if len(spList) == 0 {
			// No data found, return HTML with SweetAlert
			c.Header("Content-Type", "text/html; charset=utf-8")
			c.String(http.StatusOK, `
				<!DOCTYPE html>
				<html>
				<head>
					<title>No Data SP</title>
					<script src="https://cdn.jsdelivr.net/npm/sweetalert2@11"></script>
				</head>
				<body>
					<script>
						Swal.fire({
							icon: 'info',
							title: 'Tidak Ada Data',
							text: 'Tidak ada data SP yang perlu dibalas. Semua pesan SP sudah dibalas / tidak ada pesan SP yang masuk.',
							confirmButtonText: 'OK'
						}).then(() => {
							window.close();
							if (window.history.length > 1) {
								window.history.back();
							}
						});
					</script>
				</body>
				</html>
			`)
			return
		}

		// Write SP data to Excel
		for idx, sp := range spList {
			rowNum := idx + 2
			row := []interface{}{
				sp.NomorSuratSP1,
				sp.NomorSuratSP2,
				sp.NomorSuratSP3,
				sp.PelanggaranSP1,
				sp.PelanggaranSP2,
				sp.PelanggaranSP3,
				sp.SPUntuk,
				sp.SAC,
				"", // balasan - empty for user to fill
				"", // disanggah_pada - empty for user to fill
			}

			for colIdx, value := range row {
				cell := fmt.Sprintf("%s%d", string(rune('A'+colIdx)), rowNum)
				// Don't write 0 values for nomor_surat columns
				if colIdx < 3 && value == 0 {
					f.SetCellValue(sheetName, cell, "")
				} else {
					f.SetCellValue(sheetName, cell, value)
				}
				f.SetCellStyle(sheetName, cell, cell, dataStyle)
			}
		}

		// Set column widths
		f.SetColWidth(sheetName, "A", "A", 18) // nomor_surat_sp1
		f.SetColWidth(sheetName, "B", "B", 18) // nomor_surat_sp2
		f.SetColWidth(sheetName, "C", "C", 18) // nomor_surat_sp3
		f.SetColWidth(sheetName, "D", "D", 40) // pelanggaran_sp1
		f.SetColWidth(sheetName, "E", "E", 40) // pelanggaran_sp2
		f.SetColWidth(sheetName, "F", "F", 40) // pelanggaran_sp3
		f.SetColWidth(sheetName, "G", "G", 25) // sp_untuk
		f.SetColWidth(sheetName, "H", "H", 20) // sac
		f.SetColWidth(sheetName, "I", "I", 40) // balasan
		f.SetColWidth(sheetName, "J", "J", 22) // disanggah_pada

		// Add comments/notes to header cells for guidance
		f.AddComment(sheetName, excelize.Comment{
			Cell:   "A1",
			Author: "System",
			Paragraph: []excelize.RichTextRun{
				{Text: "Nomor Surat SP1 (auto-filled from database)", Font: &excelize.Font{Color: "#000000"}},
			},
		})
		f.AddComment(sheetName, excelize.Comment{
			Cell:   "B1",
			Author: "System",
			Paragraph: []excelize.RichTextRun{
				{Text: "Nomor Surat SP2 (auto-filled from database)", Font: &excelize.Font{Color: "#000000"}},
			},
		})
		f.AddComment(sheetName, excelize.Comment{
			Cell:   "C1",
			Author: "System",
			Paragraph: []excelize.RichTextRun{
				{Text: "Nomor Surat SP3 (auto-filled from database)", Font: &excelize.Font{Color: "#000000"}},
			},
		})
		f.AddComment(sheetName, excelize.Comment{
			Cell:   "D1",
			Author: "System",
			Paragraph: []excelize.RichTextRun{
				{Text: "Pelanggaran untuk SP1 (auto-filled from database)", Font: &excelize.Font{Color: "#000000"}},
			},
		})
		f.AddComment(sheetName, excelize.Comment{
			Cell:   "E1",
			Author: "System",
			Paragraph: []excelize.RichTextRun{
				{Text: "Pelanggaran untuk SP2 (auto-filled from database)", Font: &excelize.Font{Color: "#000000"}},
			},
		})
		f.AddComment(sheetName, excelize.Comment{
			Cell:   "F1",
			Author: "System",
			Paragraph: []excelize.RichTextRun{
				{Text: "Pelanggaran untuk SP3 (auto-filled from database)", Font: &excelize.Font{Color: "#000000"}},
			},
		})
		f.AddComment(sheetName, excelize.Comment{
			Cell:   "G1",
			Author: "System",
			Paragraph: []excelize.RichTextRun{
				{Text: "SP Untuk: Technician/SPL/SAC identifier (auto-filled from database)", Font: &excelize.Font{Color: "#000000"}},
			},
		})
		f.AddComment(sheetName, excelize.Comment{
			Cell:   "H1",
			Author: "System",
			Paragraph: []excelize.RichTextRun{
				{Text: "SAC: Penanggung jawab / person responsible (auto-filled from TechODOOMSData)", Font: &excelize.Font{Color: "#000000"}},
			},
		})
		f.AddComment(sheetName, excelize.Comment{
			Cell:   "I1",
			Author: "System",
			Paragraph: []excelize.RichTextRun{
				{Text: "Balasan: Reply text / message content - FILL THIS COLUMN", Font: &excelize.Font{Color: "#FF0000", Bold: true}},
			},
		})
		f.AddComment(sheetName, excelize.Comment{
			Cell:   "J1",
			Author: "System",
			Paragraph: []excelize.RichTextRun{
				{Text: "Disanggah Pada: Datetime when replied (flexible formats accepted). Leave empty to use current time.\nExamples: 2025-01-15 14:30:00 or 15/01/2025 14:30", Font: &excelize.Font{Color: "#000000"}},
			},
		})

		// Freeze the header row
		f.SetPanes(sheetName, &excelize.Panes{
			Freeze:      true,
			Split:       false,
			XSplit:      0,
			YSplit:      1,
			TopLeftCell: "A2",
			ActivePane:  "bottomLeft",
		})

		// Write to buffer
		buffer, err := f.WriteToBuffer()
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to write Excel file: " + err.Error()})
			return
		}

		// Set response headers
		sacSuffix := ""
		if sacFilter != "" {
			// Clean SAC name for filename (remove special characters)
			cleanSAC := regexp.MustCompile(`[^a-zA-Z0-9_-]+`).ReplaceAllString(sacFilter, "_")
			sacSuffix = fmt.Sprintf("_SAC_%s", cleanSAC)
		}
		filename := fmt.Sprintf("sp_reply_simulation_template%s_%s.xlsx", sacSuffix, time.Now().Format("20060102_150405"))
		c.Header("Content-Type", "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet")
		c.Header("Content-Disposition", fmt.Sprintf("attachment; filename=%s", filename))
		c.Header("Content-Length", fmt.Sprintf("%d", buffer.Len()))

		// Stream the file
		_, err = c.Writer.Write(buffer.Bytes())
		if err != nil {
			logrus.Errorf("Failed to write Excel file to response: %v", err)
		}
	}
}

// UploadSPReplySimulation processes an uploaded Excel file to simulate WhatsApp replies for SP records
func UploadSPReplySimulation(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		dbWeb := gormdb.Databases.Web

		// Parse JWT token from cookie
		tokenString, err := c.Cookie("token")
		if err != nil {
			c.JSON(http.StatusUnauthorized, gin.H{"success": false, "message": "Unauthorized: No token provided"})
			return
		}
		tokenString = strings.ReplaceAll(tokenString, " ", "+")

		decrypted, err := fun.GetAESDecrypted(tokenString)
		if err != nil {
			c.JSON(http.StatusUnauthorized, gin.H{"success": false, "message": "Unauthorized: Invalid token"})
			return
		}

		var claims map[string]interface{}
		err = json.Unmarshal(decrypted, &claims)
		if err != nil {
			c.JSON(http.StatusUnauthorized, gin.H{"success": false, "message": "Unauthorized: Invalid token claims"})
			return
		}

		// Authorization check
		allowedUsers := []string{"developer", "admin", "human resource", "csna"}
		userID := uint(claims["id"].(float64))
		userFullName := claims["fullname"].(string)
		userFullNameLower := strings.ToLower(userFullName)

		isAllowed := false
		for _, allowedUser := range allowedUsers {
			if strings.Contains(userFullNameLower, allowedUser) {
				isAllowed = true
				break
			}
		}

		if !isAllowed {
			c.JSON(http.StatusForbidden, gin.H{"success": false, "message": "Forbidden: You don't have permission to upload SP reply simulations"})
			return
		}

		// Get uploaded file
		file, header, err := c.Request.FormFile("file")
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": "Failed to retrieve file: " + err.Error()})
			return
		}
		defer file.Close()

		// Validate file extension
		filename := header.Filename
		ext := filepath.Ext(filename)
		if ext != ".xlsx" && ext != ".xls" {
			c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": "Invalid file type. Only .xlsx and .xls files are allowed"})
			return
		}

		// Validate file size (20MB max)
		maxSize := int64(20 * 1024 * 1024) // 20MB
		if header.Size > maxSize {
			c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": fmt.Sprintf("File size exceeds maximum limit of 20MB (your file: %.2f MB)", float64(header.Size)/(1024*1024))})
			return
		}

		// Read the Excel file
		xlsx, err := excelize.OpenReader(file)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": "Failed to parse Excel file: " + err.Error()})
			return
		}

		// Get rows from Sheet1
		rows, err := xlsx.GetRows("Sheet1")
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": "Failed to read Excel rows: " + err.Error()})
			return
		}

		if len(rows) <= 1 {
			c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": "Excel file is empty or contains only headers"})
			return
		}

		// Process rows (skip header row and example rows if they exist)
		type ReplySimulation struct {
			NomorSuratSP1 int
			NomorSuratSP2 int
			NomorSuratSP3 int
			SPUntuk       string
			Balasan       string
			DisanggahPada time.Time
			SPNumber      int    // Determined from which nomor_surat is filled
			SPType        string // TECH, SPL, or SAC (determined by search)
			ParentID      uint   // ID of the parent record (determined by search)
			RowNumber     int
		}

		var simulations []ReplySimulation
		var errors []string
		successCount := 0
		skipCount := 0

		// Start from row index 1 (skip header at index 0)
		for i := 1; i < len(rows); i++ {
			row := rows[i]
			rowNumber := i + 1

			// Skip empty rows
			if len(row) == 0 || (len(row) > 0 && strings.TrimSpace(row[0]) == "") {
				skipCount++
				continue
			}

			// Skip example rows (check if identifier contains "Example")
			// Updated column index for sp_untuk (now column 6, index 6)
			if len(row) > 6 && strings.Contains(strings.ToLower(row[6]), "example") {
				skipCount++
				continue
			}

			// Ensure row has minimum required columns
			// We need at least 7 columns (up to sp_untuk at index 6)
			// Columns: sp1(0), sp2(1), sp3(2), pelanggaran1(3), pelanggaran2(4), pelanggaran3(5), sp_untuk(6), [sac(7)], [balasan(8)], [disanggah_pada(9)]
			if len(row) < 7 {
				errors = append(errors, fmt.Sprintf("Row %d: Insufficient columns (expected at least 7, got %d)", rowNumber, len(row)))
				continue
			}

			// Safely get values with bounds checking
			getSafeValue := func(idx int) string {
				if idx < len(row) {
					return strings.TrimSpace(row[idx])
				}
				return ""
			}

			sim := ReplySimulation{
				SPUntuk:   getSafeValue(6), // sp_untuk column
				Balasan:   getSafeValue(8), // balasan column
				RowNumber: rowNumber,
			}

			// Skip rows where balasan (reply text) is empty - these are rows user hasn't filled yet
			if sim.Balasan == "" {
				skipCount++
				continue
			}

			// Parse nomor_surat_sp columns with safe access
			spVal1 := getSafeValue(0)
			if spVal1 != "" {
				val, err := strconv.Atoi(spVal1)
				if err == nil {
					sim.NomorSuratSP1 = val
				}
			}
			spVal2 := getSafeValue(1)
			if spVal2 != "" {
				val, err := strconv.Atoi(spVal2)
				if err == nil {
					sim.NomorSuratSP2 = val
				}
			}
			spVal3 := getSafeValue(2)
			if spVal3 != "" {
				val, err := strconv.Atoi(spVal3)
				if err == nil {
					sim.NomorSuratSP3 = val
				}
			}

			// Determine SP number based on which nomor_surat is filled
			spCount := 0
			if sim.NomorSuratSP1 > 0 {
				sim.SPNumber = 1
				spCount++
			}
			if sim.NomorSuratSP2 > 0 {
				sim.SPNumber = 2
				spCount++
			}
			if sim.NomorSuratSP3 > 0 {
				sim.SPNumber = 3
				spCount++
			}

			if spCount == 0 {
				errors = append(errors, fmt.Sprintf("Row %d: No SP number provided (nomor_surat_sp1/2/3 are all empty)", rowNumber))
				continue
			}
			if spCount > 1 {
				errors = append(errors, fmt.Sprintf("Row %d: Multiple SP numbers provided (only one of nomor_surat_sp1/2/3 should be filled)", rowNumber))
				continue
			}

			// Validate sp_untuk
			if sim.SPUntuk == "" {
				errors = append(errors, fmt.Sprintf("Row %d: sp_untuk cannot be empty", rowNumber))
				continue
			}

			// Parse disanggah_pada datetime (optional, flexible format) - column index 9
			disanggahPadaStr := getSafeValue(9)
			if disanggahPadaStr != "" {
				parsedTime, err := fun.ParseFlexibleDate(disanggahPadaStr)
				if err != nil {
					errors = append(errors, fmt.Sprintf("Row %d: Invalid disanggah_pada format '%s' (use flexible date format)", rowNumber, disanggahPadaStr))
					continue
				}
				sim.DisanggahPada = parsedTime
			} else {
				// Use current time if disanggah_pada is empty
				sim.DisanggahPada = time.Now()
			}

			simulations = append(simulations, sim)
		}

		// If all rows failed validation, return errors
		if len(simulations) == 0 {
			c.JSON(http.StatusBadRequest, gin.H{
				"success": false,
				"message": "No valid rows to process",
				"errors":  errors,
				"skipped": skipCount,
			})
			return
		}

		// Begin transaction
		tx := dbWeb.Begin()
		defer func() {
			if r := recover(); r != nil {
				tx.Rollback()
			}
		}()

		// Process each simulation
		for _, sim := range simulations {
			var whatSP string
			var nomorSurat int
			found := false

			// Get the appropriate nomor_surat based on SP number
			switch sim.SPNumber {
			case 1:
				nomorSurat = sim.NomorSuratSP1
			case 2:
				nomorSurat = sim.NomorSuratSP2
			case 3:
				nomorSurat = sim.NomorSuratSP3
			}

			// Search across all three tables to find the SP record
			// Try Technician first
			var techSP sptechnicianmodel.TechnicianGotSP
			var query string
			switch sim.SPNumber {
			case 1:
				query = "technician = ? AND nomor_surat_sp1 = ?"
			case 2:
				query = "technician = ? AND nomor_surat_sp2 = ?"
			case 3:
				query = "technician = ? AND nomor_surat_sp3 = ?"
			}
			result := tx.Where(query, sim.SPUntuk, nomorSurat).First(&techSP)
			if result.Error == nil {
				sim.ParentID = techSP.ID
				sim.SPType = "TECH"
				whatSP = "SP_TECHNICIAN"
				found = true
			}

			// If not found in Technician, try SPL
			if !found {
				var splSP sptechnicianmodel.SPLGotSP
				switch sim.SPNumber {
				case 1:
					query = "spl = ? AND nomor_surat_sp1 = ?"
				case 2:
					query = "spl = ? AND nomor_surat_sp2 = ?"
				case 3:
					query = "spl = ? AND nomor_surat_sp3 = ?"
				}
				result = tx.Where(query, sim.SPUntuk, nomorSurat).First(&splSP)
				if result.Error == nil {
					sim.ParentID = splSP.ID
					sim.SPType = "SPL"
					whatSP = "SP_SPL"
					found = true
				}
			}

			// If not found in SPL, try SAC
			if !found {
				var sacSP sptechnicianmodel.SACGotSP
				switch sim.SPNumber {
				case 1:
					query = "sac = ? AND nomor_surat_sp1 = ?"
				case 2:
					query = "sac = ? AND nomor_surat_sp2 = ?"
				case 3:
					query = "sac = ? AND nomor_surat_sp3 = ?"
				}
				result = tx.Where(query, sim.SPUntuk, nomorSurat).First(&sacSP)
				if result.Error == nil {
					sim.ParentID = sacSP.ID
					sim.SPType = "SAC"
					whatSP = "SP_SAC"
					found = true
				}
			}

			// If not found in any table, log error
			if !found {
				errors = append(errors, fmt.Sprintf("Row %d: SP record for '%s' with nomor_surat_sp%d = %d not found in any table (Technician/SPL/SAC)", sim.RowNumber, sim.SPUntuk, sim.SPNumber, nomorSurat))
				continue
			}

			// Find WhatsApp message(s) for this SP
			var messages []sptechnicianmodel.SPWhatsAppMessage
			msgQuery := tx.Where("what_sp = ? AND number_of_sp = ?", whatSP, sim.SPNumber)

			// Add foreign key condition based on type
			switch sim.SPType {
			case "TECH":
				msgQuery = msgQuery.Where("technician_got_sp_id = ?", sim.ParentID)
			case "SPL":
				msgQuery = msgQuery.Where("spl_got_sp_id = ?", sim.ParentID)
			case "SAC":
				msgQuery = msgQuery.Where("sac_got_sp_id = ?", sim.ParentID)
			}

			msgResult := msgQuery.Find(&messages)
			if msgResult.Error != nil || len(messages) == 0 {
			}

			// Update all messages for this SP
			updatedCount := 0
			for _, msg := range messages {
				// Check if already replied
				if msg.WhatsappRepliedBy != "" && msg.WhatsappRepliedAt != nil {
					// Skip already replied messages (optional: could add a force flag)
					continue
				}

				// Update reply fields
				// Use the original WhatsappMessageSentTo as the replier (the person who received the SP)
				msg.WhatsappRepliedBy = msg.WhatsappMessageSentTo
				msg.WhatsappRepliedAt = &sim.DisanggahPada
				msg.WhatsappReplyText = sim.Balasan

				// Save the message
				if err := tx.Save(&msg).Error; err != nil {
					errors = append(errors, fmt.Sprintf("Row %d: Failed to update message: %v", sim.RowNumber, err))
					continue
				}
				updatedCount++
			}

			if updatedCount > 0 {
				successCount++
			}
		}

		// Commit transaction
		if err := tx.Commit().Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{
				"success": false,
				"message": "Failed to commit transaction: " + err.Error(),
			})
			return
		}

		// Log activity
		logMessage := fmt.Sprintf("Simulated %d SP WhatsApp replies from file '%s'", successCount, filename)
		if len(errors) > 0 {
			logMessage += fmt.Sprintf(" (%d errors)", len(errors))
		}

		dbWeb.Create(&model.LogActivity{
			AdminID:   userID,
			FullName:  userFullName,
			Action:    "Upload SP Reply Simulation",
			Status:    "Success",
			Log:       logMessage,
			IP:        c.ClientIP(),
			UserAgent: c.Request.UserAgent(),
			ReqMethod: c.Request.Method,
			ReqUri:    c.Request.RequestURI,
		})

		logrus.Infof("SP Reply Simulation: %d successful, %d errors, %d skipped by %s", successCount, len(errors), skipCount, userFullName)

		// Return response
		response := gin.H{
			"success":       true,
			"message":       fmt.Sprintf("Successfully simulated %d SP replies", successCount),
			"success_count": successCount,
			"skipped_count": skipCount,
			"total_rows":    len(rows) - 1, // Exclude header
		}

		if len(errors) > 0 {
			response["errors"] = errors
			response["error_count"] = len(errors)
		}

		c.JSON(http.StatusOK, response)
	}
}
