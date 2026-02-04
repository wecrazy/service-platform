package controllers

import (
	"fmt"
	"html/template"
	"net/http"
	"reflect"
	"service-platform/cmd/technical_assistance/fun"
	"service-platform/cmd/technical_assistance/model"
	"service-platform/cmd/technical_assistance/model/op_model"
	"service-platform/internal/config"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

func TablePengerjaanTeknisiPending(db *gorm.DB, dbWeb *gorm.DB) gin.HandlerFunc {
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
		for i := 0; i < t.NumField(); i++ {
			field := t.Field(i)
			// Get the JSON key
			jsonKey := field.Tag.Get("json")
			// if jsonKey == "" || jsonKey == "-" {
			// 	continue
			// }
			if jsonKey == "" || jsonKey == "-" || jsonKey == "foto" || jsonKey == "cek" || jsonKey == "konfirmasi" || jsonKey == "hapus" {
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
				if jsonKey == "" || jsonKey == "-" || jsonKey == "foto" || jsonKey == "konfirmasi" || jsonKey == "hapus" || jsonKey == "cek" || jsonKey == "edit" {
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

				filteredQuery = filteredQuery.Or("`"+dataField+"` LIKE ?", "%"+request.Search+"%")

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
		db.Model(&op_model.Pending{}).Count(&totalRecords)

		// Count the number of filtered records
		var filteredRecords int64
		filteredQuery.Count(&filteredRecords)

		// Apply sorting and pagination to the filtered query
		query := filteredQuery.Order(orderString)
		var Teknisis []op_model.Pending
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
		var data []gin.H
		// id_foto := []string{"x_foto_bast",
		// 	"x_foto_ceklis",
		// 	"x_foto_edc",
		// 	"x_foto_pic",
		// 	"x_foto_setting",
		// 	"x_foto_thermal",
		// 	"x_foto_toko",
		// 	"x_foto_training",
		// 	"x_foto_transaksi",
		// 	"x_tanda_tangan_pic",
		// 	"x_tanda_tangan_teknisi",

		// 	// New entries
		// 	"x_foto_sticker_edc",
		// 	"x_foto_screen_guard",
		// 	"x_foto_all_transaction",
		// 	"x_foto_transaksi_bmri",
		// 	"x_foto_transaksi_bni",
		// 	"x_foto_transaksi_bri",
		// 	"x_foto_transaksi_btn",
		// 	"x_foto_transaksi_patch",
		// 	"x_foto_screen_p2g",
		// 	"x_foto_kontak_stiker_pic",
		// }

		// judul_foto := []string{"Foto BAST",
		// 	"Foto Media Promo",
		// 	"Foto SN EDC",
		// 	"Foto PIC Merchant",
		// 	"Foto Pengaturan",
		// 	"Foto Thermal",
		// 	"Foto Merchant",
		// 	"Foto Surat Training",
		// 	"Foto Transaksi",
		// 	"Tanda Tangan PIC",
		// 	"Tanda Tangan Teknisi",

		// 	// New titles
		// 	"Foto Stiker EDC",
		// 	"Foto Screen Gard",
		// 	"Foto Sales Draft All Memberbank",
		// 	"Foto Sales Draft BMRI",
		// 	"Foto Sales Draft BNI",
		// 	"Foto Sales Draft BRI",
		// 	"Foto Sales Draft BTN",
		// 	"Foto Sales Draft Patch L",
		// 	"Foto Screen P2G",
		// 	"Foto Kontak Stiker PIC",
		// }

		woDetailURL := config.TechnicalAssistance.Get().WO_DETAIL_URL

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
				} else if theKey == "company" {
					company = fieldValue.Interface().(string)
					newData[theKey] = fieldValue.Interface().(string)
				} else if theKey == "reason" {
					reasonCode = fieldValue.Interface().(string)
					newData[theKey] = fieldValue.Interface().(string)
				} else if theKey == "keterangan" {
					var dataValue *string
					if fieldValue.IsNil() {
						dataValue = nil
					} else {
						dataValue = fieldValue.Interface().(*string)
					}

					if dataValue == nil {
						woRemark = ""
						newData[theKey] = ""
					} else {
						woRemark = *dataValue
						newData[theKey] = *dataValue
					}
				} else if theKey == "wo" {
					woNumber = fieldValue.Interface().(string)
					if woNumber != "" {
						newData[theKey] = fmt.Sprintf(`<a href="%s/odooms-project-task/detailWO?wo_number=%v" target="_blank">%v</a>`, woDetailURL, woNumber, woNumber)
					} else {
						newData[theKey] = fieldValue.Interface().(string)
					}
				} else if theKey == "teknisi" {
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
				} else if theKey == "foto" {
					// var image_view strings.Builder
					// image_view.WriteString(fmt.Sprintf(`<div id="%s__%d" class="d-flex" style="width:400px;overflow:auto;">`, id_task, i))
					// for i, id := range id_foto {
					// 	// image := config.TechnicalAssistance.Get().FILESTORE_URL +
					// 	// image_view.WriteString(fmt.Sprintf(
					// 	// 	`<div class="my-1 p-1" style="width:210px;display:flex;flex-direction:column;justify-content:space-between;">
					// 	// 		<img src="/here/file/%s@%s" style="width:200px;height:auto;" class="card-img-top" alt="%s" onclick="window.open(this.src, '_blank');"/>
					// 	// 		<h5 class="card-title text-center">%s</h5>
					// 	// 	</div>`, id_task, id, judul_foto[i], judul_foto[i]))
					// 	image_view.WriteString(fmt.Sprintf(
					// 		`<div class="my-1 p-1" style="width:210px;display:flex;flex-direction:column;justify-content:space-between;">
					// 		<img src="/here/file/%s@%s"
					// 			style="width:200px;height:200px;object-fit:contain;cursor:pointer;"
					// 			class="card-img-top"
					// 			alt="%s"
					// 			onclick="window.open(this.src, '_blank');"
					// 			onerror="this.onerror=null; this.src='/assets/self/img/no-img.jpg';"/>
					// 		<h5 class="card-title text-center">%s</h5>
					// 	</div>`, id_task, id, judul_foto[i], judul_foto[i]))
					// }
					// image_view.WriteString(`</div>`)

					// // var image_view strings.Builder
					// // image_view.WriteString(fmt.Sprintf(`<div id="%s__%d" class="d-flex" style="width:400px;overflow:auto;">`, id_task, i))

					// // for i, id := range id_foto {
					// // 	imageURL := fmt.Sprintf("/here/file/%s@%s", id_task, id)
					// // 	randomStr := fun.GenerateRandomString(100)
					// // 	containerID := fmt.Sprintf("image-container-%s_%d_%s", id_task, i, randomStr)

					// // 	image_view.WriteString(fmt.Sprintf(
					// // 		`<div class="my-1 p-1" id="%s" style="width:210px;display:flex;flex-direction:column;justify-content:space-between;">
					// // 			<h5 class="card-title text-center">%s</h5>
					// // 		</div>
					// // 		<script>
					// // 			getImage("%s", "%s", "%s", "%s");
					// // 		</script>`,
					// // 		containerID, judul_foto[i], imageURL, containerID, judul_foto[i], "/assets/img/misc/no-img.jpg",
					// // 	))
					// // }

					// // image_view.WriteString(`</div>`)

					// newData[theKey] = image_view.String()

					// Btn photos
					newData[theKey] =
						fmt.Sprintf(
							`
							<div class="card-cek">
								<button class="btn btn-sm btn-info" onclick="openPopupPhotos('%s', 'pending')">
									<i class='bx bx-image-alt me-2'></i> Lihat Foto & Tambahan Data
								</button>
							</div>
							`, id_task,
						)
				} else if theKey == "cek" {
					newData[theKey] =
						fmt.Sprintf(
							`
							<div class="card-cek">
								<input type="hidden" class="form-control id_task" value="%s">
								<button class="btn btn-sm btn-warning" onclick="sendCekPending(this)">
									<i class='bx bx-refresh'></i>
								</button>
							</div>
							`, id_task,
						)
				} else if theKey == "ta_feedback" {
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
					`, escapedValue, "pending", id_task, woNumber, "ta_feedback", escapedValue)
				} else if theKey == "edit" {
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
				} else if theKey == "konfirmasi" {
					newData[theKey] =
						fmt.Sprintf(
							`<div class="card">
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

									<div class="form-check d-flex align-items-center mt-2 mb-2">
										<input 
											class="form-check-input is-paid" 
											type="checkbox"
											checked
											id="is-paid"
											data-bs-toggle="tooltip" 
											data-bs-placement="right" 
											title="Jika dicentang, pengerjaan teknisi nantinya akan dibayarkan"
										>
										<label for="is-paid" class="form-check-label ms-2">Paid?</label>
									</div>

									<div class="form-check d-flex align-items-center mb-2">
										<input 
											class="form-check-input keep-data" 
											type="checkbox"
											id="keep-data"
											data-bs-toggle="tooltip" 
											data-bs-placement="right" 
											title="Jika dicentang, data akan tetap muncul di Dashboard TA 😃"
										>
										<label for="keep-data" class="form-check-label ms-2">Tetap Simpan Data?</label>
									</div>

									<button class="btn btn-primary w-100" onclick="sendDataKonfirmasiPending(this)">Konfirmasi</button>
								</div>
							</div>
						</div>`, id_task,
						)
				} else if theKey == "hapus" {
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
									<button class="btn btn-danger w-100" onclick="sendDataHapusPending(this)">Hapus</button>
								</div>
							</div>
						</div>`, id_task,
						)
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
