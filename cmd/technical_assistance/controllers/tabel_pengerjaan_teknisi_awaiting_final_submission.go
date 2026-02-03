package controllers

import (
	"fmt"
	"html/template"
	"net/http"
	"os"
	"reflect"
	"service-platform/cmd/technical_assistance/fun"
	"service-platform/cmd/technical_assistance/model"
	"service-platform/cmd/technical_assistance/model/op_model"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

func TablePengerjaanTeknisiSubmission(db *gorm.DB, dbWeb *gorm.DB) gin.HandlerFunc {
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

		t := reflect.TypeOf(op_model.TempSubmission{})

		// Initialize the map
		columnMap := make(map[int]string)

		// Loop through the fields of the struct
		colNum := 0
		for i := 0; i < t.NumField(); i++ {
			field := t.Field(i)
			// Get the JSON key
			jsonKey := field.Tag.Get("json")
			if jsonKey == "" || jsonKey == "-" || jsonKey == "foto" || jsonKey == "cek" || jsonKey == "edit" || jsonKey == "hapus" {
				continue
			}
			columnMap[colNum] = jsonKey
			colNum++
		}

		// Get the column name based on SortColumn value
		sortColumnName := columnMap[request.SortColumn]
		orderString := fmt.Sprintf("%s %s", sortColumnName, request.SortDir)

		// Default to id_task desc if no sort column is specified
		if sortColumnName == "" {
			orderString = "id_task desc"
		}

		// Initial query for filtering
		filteredQuery := db.Model(&op_model.TempSubmission{})

		// // Apply filters
		if request.Search != "" {
			// var querySearch []string
			// var querySearchParams []interface{}

			for i := 0; i < t.NumField(); i++ {
				dataField := ""
				field := t.Field(i)
				// Get the variable name
				// varName := field.Name
				// Get the data type
				dataType := field.Type.String()
				// Get the JSON key
				jsonKey := field.Tag.Get("json")
				// Get the GORM tag
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
				if jsonKey == "" || jsonKey == "-" || jsonKey == "foto" || jsonKey == "hapus" || jsonKey == "cek" || jsonKey == "edit" {
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
				if dataType != "string" && dataType != "*string" {
					continue
				}
				// fmt.Printf("Variable Name: %s, Data Type: %s, JSON Key: %s, GORM Column Key: %s\n", varName, dataType, jsonKey, columnKey)

				filteredQuery = filteredQuery.Debug().Or("`"+dataField+"` LIKE ?", "%"+request.Search+"%")

			}

		} else {
			for i := 0; i < t.NumField(); i++ {
				field := t.Field(i)
				// formKey := field.Tag.Get("form")
				formKey := field.Tag.Get("json")
				if formKey == "" || formKey == "-" || formKey == "foto" || formKey == "hapus" || formKey == "cek" || formKey == "edit" {
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
								filteredQuery = filteredQuery.Debug().Where(
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
							filteredQuery = filteredQuery.Debug().Where(
								"DATE(`"+formKey+"`) = ?",
								date.Format("2006-01-02"),
							)
							isHandled = true
						}
					}

					if !isHandled {
						// Fallback to LIKE if no valid date
						filteredQuery = filteredQuery.Debug().Where("`"+formKey+"` LIKE ?", "%"+formValue+"%")
					}
				}
			}

		}

		// Count the total number of records
		var totalRecords int64
		db.Model(&op_model.TempSubmission{}).Count(&totalRecords)

		// Count the number of filtered records
		var filteredRecords int64
		filteredQuery.Count(&filteredRecords)

		// Apply sorting and pagination to the filtered query
		query := filteredQuery.Order(orderString)
		var Teknisis []op_model.TempSubmission
		query = query.Offset(request.Start).Limit(request.Length).Find(&Teknisis)

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

		woDetailURL := os.Getenv("WO_DETAIL_URL")

		var data []gin.H
		for _, dbData := range Teknisis {
			newData := make(map[string]interface{})

			v := reflect.ValueOf(dbData)

			// Data for using in JS
			var id_task, woNumber, company, reasonCode, woRemark, taFeedbackValue string

			for i := 0; i < t.NumField(); i++ {
				field := t.Field(i)
				fieldValue := v.Field(i)

				// varName := field.Name

				// Get the JSON key
				theKey := field.Tag.Get("json")
				if theKey == "" {
					theKey = field.Tag.Get("form")
					if theKey == "" {
						continue
					}
				}

				switch theKey {
				case "birthdate":
					if fieldValue.Type() == reflect.TypeOf(time.Time{}) {
						newData[theKey] = fieldValue.Interface().(time.Time).Format(fun.T_YYYYMMDD)
					} else if fieldValue.Kind() == reflect.Ptr && fieldValue.Type().Elem() == reflect.TypeOf(time.Time{}) {
						if fieldValue.IsNil() {
							newData[theKey] = "N/A"
						} else {
							timeValue := fieldValue.Interface().(*time.Time)
							newData[theKey] = timeValue.Format(fun.T_YYYYMMDD)
						}
					}
				case "date":
					if fieldValue.Type() == reflect.TypeOf(time.Time{}) {
						newData[theKey] = fieldValue.Interface().(time.Time).
							Add(7 * time.Hour).
							Format(fun.T_YYYYMMDD_HHmmss)
					} else if fieldValue.Kind() == reflect.Ptr && fieldValue.Type().Elem() == reflect.TypeOf(time.Time{}) {
						if fieldValue.IsNil() {
							newData[theKey] = "N/A"
						} else {
							timeValue := fieldValue.Interface().(*time.Time)
							newData[theKey] = timeValue.Add(7 * time.Hour).Format(fun.T_YYYYMMDD_HHmmss)
						}
					}
				case "time_start", "time_stop":
					if fieldValue.Kind() == reflect.Ptr && fieldValue.Type().Elem() == reflect.TypeOf(time.Time{}) {
						if fieldValue.IsNil() {
							newData[theKey] = "N/A"
						} else {
							timeValue := fieldValue.Interface().(*time.Time)
							newData[theKey] = timeValue.Format("2006-01-02 15:04:05")
						}
					}
				case "id_task":
					id_task = fieldValue.Interface().(string)
					newData[theKey] = fieldValue.Interface().(string)
				case "company":
					company = fieldValue.Interface().(string)
					newData[theKey] = fieldValue.Interface().(string)
				case "reason":
					reasonCode = fieldValue.Interface().(string)
					newData[theKey] = fieldValue.Interface().(string)
				case "keterangan":
					woRemark = fieldValue.Interface().(string)
					newData[theKey] = woRemark
				case "wo":
					woNumber = fieldValue.Interface().(string)
					if woNumber != "" {
						newData[theKey] = fmt.Sprintf(`<a href="%s/odooms-project-task/detailWO?wo_number=%v" target="_blank">%v</a>`, woDetailURL, woNumber, woNumber)
					} else {
						newData[theKey] = fieldValue.Interface().(string)
					}
				case "teknisi":
					namaTeknisi := fieldValue.Interface().(string)

					if namaTeknisi != "" {
						var nomorTeknisi string
						var dataTeknisi model.DataTeknisi
						if err := dbWeb.Where("nama LIKE ?", "%"+namaTeknisi+"%").First(&dataTeknisi).Error; err != nil {
							// log.Print(err)
							nomorTeknisi = "87883507445"
						}
						nomorTeknisi = dataTeknisi.NoHP

						newData[theKey] = fmt.Sprintf(`<a href="http://127.0.0.1:2500/telpon?nama=%v&nomor=%v" target="_blank">%v</a>`, namaTeknisi, nomorTeknisi, namaTeknisi)
					} else {
						newData[theKey] = fieldValue.Interface().(string)
					}
				case "foto":
					// Btn photos
					newData[theKey] =
						fmt.Sprintf(
							`
							<div class="card-cek">
								<button class="btn btn-sm btn-info" onclick="openPopupPhotos('%s', 'temp_submission')">
									<i class='bx bx-image-alt me-2'></i> Lihat Foto & Tambahan Data
								</button>
							</div>
							`, id_task,
						)
				case "ta_feedback":
					taFeedback := fieldValue.Interface().(string)
					escapedValue := template.HTMLEscapeString(taFeedback)
					taFeedbackValue = escapedValue

					newData[theKey] = fmt.Sprintf(`
						<div class="card">
							<div class="card-body">
								<textarea class="form-control editable-feedback"
									style="cursor: pointer;" readonly
									data-value="%s"
									onclick="editFeedback(this, '%s', '%s', '%s', '%s')">%s</textarea>
							</div>
						</div>
					`, escapedValue, "temp_submission", id_task, woNumber, "ta_feedback", escapedValue)
				case "cek":
					newData[theKey] =
						fmt.Sprintf(
							`
							<div class="card-cek">
								<input type="hidden" class="form-control id_task" value="%s">
								<button class="btn btn-sm btn-warning" onclick="sendCekSubmission(this)">
									<i class='bx bx-refresh'></i>
								</button>
							</div>
							`, id_task,
						)
				case "edit":
					newData[theKey] =
						fmt.Sprintf(
							`
							<div class="card-edit-data">
								<input type="hidden" class="form-control id_task" value="%s">
								<input type="hidden" class="form-control wo_number" value="%s">
								<input type="hidden" class="form-control company" value="%s">
								<input type="hidden" class="form-control reason_code" value="%s">
								<input type="hidden" class="form-control wo_remark" value="%s">
								<input type="hidden" class="form-control editable-feedback" value="%s">
								<button class="btn btn-sm btn-success" onclick="sendEditData(this)">
									<i class='bx bx-edit'></i>
								</button>
							</div>
							`,
							id_task,
							woNumber,
							company,
							reasonCode,
							woRemark,
							taFeedbackValue,
						)
				case "hapus":
					newData[theKey] =
						fmt.Sprintf(
							`<div class="card bg-label-danger">
								<div class="card-body">
									<div class="d-flex flex-column">
										<input type="hidden" class="form-control id_task" value="%s">
										<input type="text" class="form-control email" placeholder="Masukkan email di ODOO">
										<div class="input-group">
											<input type="password" class="form-control password" placeholder="Masukkan password Anda">
											<button type="button" class="btn btn-outline-secondary" onclick="togglePasswordInputFromButton(this)">
												<i class="bx bx-show"></i>
											</button>
										</div>
										<textarea class="form-control ta_remark" rows="4" placeholder="Alasan data JO dihapus dari dashboard . . ."></textarea>
										<button class="btn btn-danger w-100" onclick="sendDataHapusSubmission(this)">Hapus</button>
									</div>
								</div>
							</div>`, id_task,
						)
				case "log_edit":
					logs := fieldValue.Interface().(string)
					if logs == "" {
						newData[theKey] = `<span class="text-muted">No edit logs available</span>`
					} else {
						var woNumber string
						if dbData.WONumber != "" {
							woNumber = dbData.WONumber
						} else {
							woNumber = "N/A"
						}

						newData[theKey] = fmt.Sprintf(
							`<button class="btn btn-sm btn-info" onclick="showEditLogs('%d', '%s', '%s')">
									<i class="bx bx-history me-2"></i> View Changes
								</button>`,
							dbData.ID,
							woNumber,
							strings.ReplaceAll(logs, "'", "\\'"))
					}
				default:
					newData[theKey] = fieldValue.Interface()
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
