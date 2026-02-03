package controllers

import (
	"fmt"
	"net/http"
	"os"
	"reflect"
	"service-platform/cmd/technical_assistance/fun"
	"service-platform/cmd/technical_assistance/model/op_model"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

func TabelPengerjaanLogActivity(db *gorm.DB) gin.HandlerFunc {
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

		t := reflect.TypeOf(op_model.LogAct{})

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
		filteredQuery := db.Model(&op_model.LogAct{})
		woDetailURL := os.Getenv("WO_DETAIL_URL")

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
				if formKey == "" || formKey == "-" {
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
		db.Model(&op_model.LogAct{}).Count(&totalRecords)

		// Count the number of filtered records
		var filteredRecords int64
		filteredQuery.Count(&filteredRecords)

		// Apply sorting and pagination to the filtered query
		query := filteredQuery.Order(orderString)
		var Teknisis []op_model.LogAct
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
		id_foto := []string{"x_foto_bast",
			"x_foto_ceklis",
			"x_foto_edc",
			"x_foto_pic",
			"x_foto_setting",
			"x_foto_thermal",
			"x_foto_toko",
			"x_foto_training",
			"x_foto_transaksi",
			"x_tanda_tangan_pic",
			"x_tanda_tangan_teknisi"}

		judul_foto := []string{"Foto BAST",
			"Foto Media Promo",
			"Foto SN EDC",
			"Foto PIC Merchant",
			"Foto Pengaturan",
			"Foto Thermal",
			"Foto Merchant",
			"Foto Surat Training",
			"Foto Transaksi",
			"Tanda Tangan PIC",
			"Tanda Tangan Teknisi"}
		var data []gin.H
		for _, person := range Teknisis {
			newData := make(map[string]any)

			v := reflect.ValueOf(person)
			id_task := ""
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

				// Handle time.Time fields differently
				if fieldValue.Type() == reflect.TypeOf(time.Time{}) {
					switch theKey {
					case "birthdate":
						newData[theKey] = fieldValue.Interface().(time.Time).Format(fun.T_YYYYMMDD)
					case "date":
						newData[theKey] = fieldValue.Interface().(time.Time).
							Add(7 * time.Hour).
							Format(fun.T_YYYYMMDD_HHmmss)
					default:
						newData[theKey] = fieldValue.Interface().(time.Time).Format(fun.T_YYYYMMDD_HHmmss)
					}
				} else {
					switch theKey {
					case "time_start", "time_stop":
						layout := "2006-01-02 15:04:05"
						parsedTime, err := time.Parse(layout, fieldValue.Interface().(string))
						if err == nil {
							newData[theKey] = parsedTime.Add(7 * time.Hour).Format(layout)
						} else {
							newData[theKey] = fieldValue.Interface().(string)
						}
					case "id_task":
						id_task = fieldValue.Interface().(string)
						newData[theKey] = fieldValue.Interface().(string)
					case "wo":
						if fieldValue.Kind() == reflect.Ptr {
							if fieldValue.IsNil() {
								newData[theKey] = "" // Set empty if nil pointer
							} else {
								// Dereference the pointer safely
								strValue := reflect.Indirect(fieldValue).Interface().(string)
								if strValue != "" {
									woNumber := strValue
									newData[theKey] = fmt.Sprintf(
										`<a href="%s/odooms-project-task/detailWO?wo_number=%v" target="_blank">%v</a>`,
										woDetailURL, woNumber, woNumber,
									)
								} else {
									newData[theKey] = "" // Set empty if string is empty
								}
							}
						} else {
							// If it's NOT a pointer, check if it's a string
							if fieldValue.Kind() == reflect.String {
								strValue := fieldValue.Interface().(string)
								if strValue != "" {
									woNumber := strValue
									newData[theKey] = fmt.Sprintf(
										`<a href="%s/odooms-project-task/detailWO?wo_number=%v" target="_blank">%v</a>`,
										woDetailURL, woNumber, woNumber,
									)
								} else {
									newData[theKey] = "" // Set empty if string is empty
								}
							} else {
								newData[theKey] = "" // If not string, set empty
							}
						}
					case "foto":
						var image_view strings.Builder
						image_view.WriteString(fmt.Sprintf(`<div id="%s__%d" class="d-flex" style="width:400px;overflow:auto;">`, id_task, i))
						for i, id := range id_foto {
							// image := os.Getenv("FILESTORE_URL") +
							image_view.WriteString(fmt.Sprintf(
								`<div class="my-1 p-1" style="width:210px;display:flex;flex-direction:column;justify-content:space-between;">
									<img src="/here/file/%s@%s" style="width:200px;height:auto;" class="card-img-top" alt="%s" onclick="window.open(this.src, '_blank');"/>
									<h5 class="card-title text-center">%s</h5>
								</div>`, id_task, id, judul_foto[i], judul_foto[i]))
						}
						image_view.WriteString(`</div>`)
						newData[theKey] = image_view.String()
					case "konfirmasi":
						newData[theKey] = fmt.Sprintf(
							`<div class="card">
								<div class="card-body">
									<div class="d-flex flex-column">
										<input type="hidden" class="form-control id_task" value="%s">
										<input type="text" class="form-control email" placeholder="email">
										<div class="input-group">
											<input type="password" class="form-control password" placeholder="password">
											<button type="button" class="btn btn-outline-secondary" onclick="togglePasswordInputFromButton(this)">
												<i class="bx bx-show"></i>
											</button>
										</div>
										<button class="btn btn-primary w-100" onclick="sendDataKonfirmasiError(this)">Konfirmasi</button>
									</div>
								</div>
							</div>`, id_task)
					case "cek":
						newData[theKey] = fmt.Sprintf(
							`<div class="card-cek">
								<input type="hidden" class="form-control id_task" value="%s">
								<button class="btn btn-sm btn-warning" onclick="sendCekError(this)">
									<i class='bx bx-refresh'></i>
								</button>
							</div>`, id_task)
					case "hapus":
						newData[theKey] = fmt.Sprintf(
							`<div class="card bg-label-danger">
								<div class="card-body">
									<div class="d-flex flex-column">
										<input type="hidden" class="form-control id_task" value="%s">
										<input type="text" class="form-control email" placeholder="email">
										<div class="input-group">
											<input type="password" class="form-control password" placeholder="password">
											<button type="button" class="btn btn-outline-secondary" onclick="togglePasswordInputFromButton(this)">
												<i class="bx bx-show"></i>
											</button>
										</div>
										<button class="btn btn-danger w-100" onclick="sendDataHapusError(this)">Hapus</button>
									</div>
								</div>
							</div>`, id_task)
					case "log_edit":
						logs := fieldValue.Interface().(string)
						if logs == "" {
							newData[theKey] = `<span class="text-muted">No edit logs available</span>`
						} else {
							var woNumber string
							if person.Wo != nil {
								woNumber = *person.Wo
							}

							newData[theKey] = fmt.Sprintf(
								`<button class="btn btn-sm btn-info" onclick="showEditLogs('%d', '%s', '%s')">
									<i class="bx bx-history me-2"></i> View Changes
								</button>`,
								person.ID,
								woNumber,
								strings.ReplaceAll(logs, "'", "\\'"))
						}
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

func TabelDataFotoError(db *gorm.DB) gin.HandlerFunc {
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

		t := reflect.TypeOf(op_model.Error{})

		// Initialize the map
		columnMap := make(map[int]string)

		// Loop through the fields of the struct
		colNum := 0
		for i, n := 0, t.NumField(); i < n; i++ {
			field := t.Field(i)
			// Get the JSON key
			jsonKey := field.Tag.Get("json")
			if jsonKey == "" || jsonKey == "-" || jsonKey == "foto" ||
				jsonKey == "cek" || jsonKey == "edit" || jsonKey == "konfirmasi" || jsonKey == "hapus" {
				continue
			}
			columnMap[colNum] = jsonKey
			colNum++
		}

		// Get the column name based on SortColumn value
		sortColumnName := columnMap[request.SortColumn]
		orderString := fmt.Sprintf("%s %s", sortColumnName, request.SortDir)

		// Initial query for filtering
		filteredQuery := db.Model(&op_model.Error{})

		// // Apply filters
		if request.Search != "" {
			// var querySearch []string
			// var querySearchParams []interface{}

			// fmt.Println("++++++++++++++++++++++++++++++")
			// fmt.Print("Search: ", request.Search)
			// fmt.Println("++++++++++++++++++++++++++++++")

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

				if jsonKey == "" || jsonKey == "-" || jsonKey == "foto" || jsonKey == "cek" || jsonKey == "edit" || jsonKey == "konfirmasi" || jsonKey == "hapus" {
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
				// fmt.Printf("Variable Name: %s, Data Type: %s, JSON Key: %s, GORM Column Key: %s\n", varName, dataType, jsonKey, columnKey)

				filteredQuery = filteredQuery.Debug().Or("`"+dataField+"` LIKE ?", "%"+request.Search+"%")
			}

		} else {
			for i := 0; i < t.NumField(); i++ {
				field := t.Field(i)
				// formKey := field.Tag.Get("form") // fix this SOON !!
				formKey := field.Tag.Get("json")

				if formKey == "" || formKey == "-" || formKey == "foto" || formKey == "cek" || formKey == "edit" || formKey == "konfirmasi" || formKey == "hapus" {
					continue
				}

				formValue := c.PostForm(formKey)
				// fmt.Print("Form Key: ", formKey)
				// fmt.Print("Form Value: ", formValue)

				// if formValue != "" {
				// 	filteredQuery = filteredQuery.Debug().Where("`"+formKey+"` LIKE ?", "%"+formValue+"%")
				// }
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
		db.Model(&op_model.Error{}).Count(&totalRecords)

		// Count the number of filtered records
		var filteredRecords int64
		filteredQuery.Count(&filteredRecords)

		// Apply sorting and pagination to the filtered query
		query := filteredQuery.Order(orderString)
		var DbData []op_model.Error
		query = query.Offset(request.Start).Limit(request.Length).Find(&DbData)

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

		id_foto := []string{"x_foto_bast",
			"x_foto_ceklis",
			"x_foto_edc",
			"x_foto_pic",
			"x_foto_setting",
			"x_foto_thermal",
			"x_foto_toko",
			"x_foto_training",
			"x_foto_transaksi",
			"x_tanda_tangan_pic",
			"x_tanda_tangan_teknisi",

			// New entries
			"x_foto_sticker_edc",
			"x_foto_screen_guard",
			"x_foto_all_transaction",
			"x_foto_transaksi_bmri",
			"x_foto_transaksi_bni",
			"x_foto_transaksi_bri",
			"x_foto_transaksi_btn",
			"x_foto_transaksi_patch",
			"x_foto_screen_p2g",
			"x_foto_kontak_stiker_pic",

			"x_foto_selfie_video_call", "x_foto_selfie_teknisi_merchant",
		}

		judul_foto := []string{"Foto BAST",
			"Foto Media Promo",
			"Foto SN EDC",
			"Foto PIC Merchant",
			"Foto Pengaturan",
			"Foto Thermal",
			"Foto Merchant",
			"Foto Surat Training",
			"Foto Transaksi",
			"Tanda Tangan PIC",
			"Tanda Tangan Teknisi",

			// New titles
			"Foto Stiker EDC",
			"Foto Screen Gard",
			"Foto Sales Draft All Memberbank",
			"Foto Sales Draft BMRI",
			"Foto Sales Draft BNI",
			"Foto Sales Draft BRI",
			"Foto Sales Draft BTN",
			"Foto Sales Draft Patch L",
			"Foto Screen P2G",
			"Foto Kontak Stiker PIC",

			"Foto Selfie Video Call", "Foto Selfie Teknisi dan Merchant",
		}

		woDetailURL := os.Getenv("WO_DETAIL_URL")

		var data []gin.H
		for _, dataInDB := range DbData {
			newData := make(map[string]interface{})

			v := reflect.ValueOf(dataInDB)

			// Data for using in JS
			var id_task, woNumber string

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

				// Handle time.Time fields differently
				if fieldValue.Type() == reflect.TypeOf(time.Time{}) {
					if theKey == "birthdate" {
						newData[theKey] = fieldValue.Interface().(time.Time).Format(fun.T_YYYYMMDD)
					} else if theKey == "date" {
						newData[theKey] = fieldValue.Interface().(time.Time).
							Add(7 * time.Hour).
							Format(fun.T_YYYYMMDD_HHmmss)
					} else {
						newData[theKey] = fieldValue.Interface().(time.Time).Format(fun.T_YYYYMMDD_HHmmss)
					}
				} else if theKey == "time_start" || theKey == "time_stop" {
					layout := "2006-01-02 15:04:05"
					parsedTime, err := time.Parse(layout, fieldValue.Interface().(string))
					if err == nil {
						newData[theKey] = parsedTime.Add(7 * time.Hour).Format(layout)
					} else {
						newData[theKey] = fieldValue.Interface().(string)
					}
				} else if theKey == "id_task" {
					id_task = fieldValue.Interface().(string)
					newData[theKey] = fieldValue.Interface().(string)
				} else if theKey == "wo" {
					woNumber = fieldValue.Interface().(string)
					if woNumber != "" {
						newData[theKey] = fmt.Sprintf(`<a href="%s/odooms-project-task/detailWO?wo_number=%v" target="_blank">%v</a>`, woDetailURL, woNumber, woNumber)
					} else {
						newData[theKey] = fieldValue.Interface().(string)
					}
				} else if theKey == "foto" {
					var image_view strings.Builder
					image_view.WriteString(fmt.Sprintf(`<div id="%s__%d" class="d-flex" style="width:400px;overflow:auto;">`, id_task, i))
					for i, id := range id_foto {
						// image := os.Getenv("FILESTORE_URL") +
						image_view.WriteString(fmt.Sprintf(
							`<div class="my-1 p-1" style="width:310px;display:flex;flex-direction:column;justify-content:space-between;">
								<img src="/here/file/%s@%s" style="width:300px;height:auto;" class="card-img-top" alt="%s" onclick="window.open(this.src, '_blank');"/>
								<h5 class="card-title text-center">%s</h5>
							</div>`, id_task, id, judul_foto[i], judul_foto[i]))
					}
					image_view.WriteString(`</div>`)

					newData[theKey] = image_view.String()
				} else if theKey == "konfirmasi" || theKey == "cek" || theKey == "edit" || theKey == "hapus" {
					newData[theKey] = `<span class="text-danger">You are not allowed to perform this action!</span>`
				} else {
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

func TabelDataPending(db *gorm.DB) gin.HandlerFunc {
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

		t := reflect.TypeOf(op_model.Pending{})

		// Initialize the map
		columnMap := make(map[int]string)

		// Loop through the fields of the struct
		colNum := 0
		for i, n := 0, t.NumField(); i < n; i++ {
			field := t.Field(i)
			// Get the JSON key
			jsonKey := field.Tag.Get("json")
			if jsonKey == "" || jsonKey == "-" || jsonKey == "foto" ||
				jsonKey == "cek" || jsonKey == "edit" || jsonKey == "konfirmasi" || jsonKey == "hapus" {
				continue
			}
			columnMap[colNum] = jsonKey
			colNum++
		}

		// Get the column name based on SortColumn value
		sortColumnName := columnMap[request.SortColumn]
		orderString := fmt.Sprintf("%s %s", sortColumnName, request.SortDir)

		// Initial query for filtering
		filteredQuery := db.Model(&op_model.Pending{})

		// // Apply filters
		if request.Search != "" {
			// var querySearch []string
			// var querySearchParams []interface{}

			// fmt.Println("++++++++++++++++++++++++++++++")
			// fmt.Print("Search: ", request.Search)
			// fmt.Println("++++++++++++++++++++++++++++++")

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

				if jsonKey == "" || jsonKey == "-" || jsonKey == "foto" || jsonKey == "cek" || jsonKey == "edit" || jsonKey == "konfirmasi" || jsonKey == "hapus" {
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
				// fmt.Printf("Variable Name: %s, Data Type: %s, JSON Key: %s, GORM Column Key: %s\n", varName, dataType, jsonKey, columnKey)

				filteredQuery = filteredQuery.Debug().Or("`"+dataField+"` LIKE ?", "%"+request.Search+"%")
			}

		} else {
			for i := 0; i < t.NumField(); i++ {
				field := t.Field(i)
				// formKey := field.Tag.Get("form") // fix this SOON !!
				formKey := field.Tag.Get("json")

				if formKey == "" || formKey == "-" || formKey == "foto" || formKey == "cek" || formKey == "edit" || formKey == "konfirmasi" || formKey == "hapus" {
					continue
				}

				formValue := c.PostForm(formKey)
				// fmt.Print("Form Key: ", formKey)
				// fmt.Print("Form Value: ", formValue)

				// if formValue != "" {
				// 	filteredQuery = filteredQuery.Debug().Where("`"+formKey+"` LIKE ?", "%"+formValue+"%")
				// }
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
		db.Model(&op_model.Pending{}).Count(&totalRecords)

		// Count the number of filtered records
		var filteredRecords int64
		filteredQuery.Count(&filteredRecords)

		// Apply sorting and pagination to the filtered query
		query := filteredQuery.Order(orderString)
		var DbData []op_model.Pending
		query = query.Offset(request.Start).Limit(request.Length).Find(&DbData)

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

		id_foto := []string{"x_foto_bast",
			"x_foto_ceklis",
			"x_foto_edc",
			"x_foto_pic",
			"x_foto_setting",
			"x_foto_thermal",
			"x_foto_toko",
			"x_foto_training",
			"x_foto_transaksi",
			"x_tanda_tangan_pic",
			"x_tanda_tangan_teknisi",

			// New entries
			"x_foto_sticker_edc",
			"x_foto_screen_guard",
			"x_foto_all_transaction",
			"x_foto_transaksi_bmri",
			"x_foto_transaksi_bni",
			"x_foto_transaksi_bri",
			"x_foto_transaksi_btn",
			"x_foto_transaksi_patch",
			"x_foto_screen_p2g",
			"x_foto_kontak_stiker_pic",
		}

		judul_foto := []string{"Foto BAST",
			"Foto Media Promo",
			"Foto SN EDC",
			"Foto PIC Merchant",
			"Foto Pengaturan",
			"Foto Thermal",
			"Foto Merchant",
			"Foto Surat Training",
			"Foto Transaksi",
			"Tanda Tangan PIC",
			"Tanda Tangan Teknisi",

			// New titles
			"Foto Stiker EDC",
			"Foto Screen Gard",
			"Foto Sales Draft All Memberbank",
			"Foto Sales Draft BMRI",
			"Foto Sales Draft BNI",
			"Foto Sales Draft BRI",
			"Foto Sales Draft BTN",
			"Foto Sales Draft Patch L",
			"Foto Screen P2G",
			"Foto Kontak Stiker PIC",
		}

		woDetailURL := os.Getenv("WO_DETAIL_URL")

		var data []gin.H
		for _, dataInDB := range DbData {
			newData := make(map[string]interface{})

			v := reflect.ValueOf(dataInDB)

			// Data for using in JS
			var id_task, woNumber string

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

				// Handle time.Time fields differently
				if fieldValue.Type() == reflect.TypeOf(time.Time{}) {
					if theKey == "birthdate" {
						newData[theKey] = fieldValue.Interface().(time.Time).Format(fun.T_YYYYMMDD)
					} else if theKey == "date" {
						newData[theKey] = fieldValue.Interface().(time.Time).
							Add(7 * time.Hour).
							Format(fun.T_YYYYMMDD_HHmmss)
					} else {
						newData[theKey] = fieldValue.Interface().(time.Time).Format(fun.T_YYYYMMDD_HHmmss)
					}
				} else if theKey == "time_start" || theKey == "time_stop" {
					layout := "2006-01-02 15:04:05"
					parsedTime, err := time.Parse(layout, fieldValue.Interface().(string))
					if err == nil {
						newData[theKey] = parsedTime.Add(7 * time.Hour).Format(layout)
					} else {
						newData[theKey] = fieldValue.Interface().(string)
					}
				} else if theKey == "id_task" {
					id_task = fieldValue.Interface().(string)
					newData[theKey] = fieldValue.Interface().(string)
				} else if theKey == "wo" {
					woNumber = fieldValue.Interface().(string)
					if woNumber != "" {
						newData[theKey] = fmt.Sprintf(`<a href="%s/odooms-project-task/detailWO?wo_number=%v" target="_blank">%v</a>`, woDetailURL, woNumber, woNumber)
					} else {
						newData[theKey] = fieldValue.Interface().(string)
					}
				} else if theKey == "foto" {
					var image_view strings.Builder
					image_view.WriteString(fmt.Sprintf(`<div id="%s__%d" class="d-flex" style="width:400px;overflow:auto;">`, id_task, i))
					for i, id := range id_foto {
						// image := os.Getenv("FILESTORE_URL") +
						image_view.WriteString(fmt.Sprintf(
							`<div class="my-1 p-1" style="width:310px;display:flex;flex-direction:column;justify-content:space-between;">
								<img src="/here/file/%s@%s" style="width:300px;height:auto;" class="card-img-top" alt="%s" onclick="window.open(this.src, '_blank');"/>
								<h5 class="card-title text-center">%s</h5>
							</div>`, id_task, id, judul_foto[i], judul_foto[i]))
					}
					image_view.WriteString(`</div>`)

					newData[theKey] = image_view.String()
				} else if theKey == "konfirmasi" || theKey == "cek" || theKey == "edit" || theKey == "hapus" {
					newData[theKey] = `<span class="text-danger">You are not allowed to perform this action!</span>`
				} else {
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
