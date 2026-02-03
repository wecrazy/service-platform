package controllers

import (
	"fmt"
	"net/http"
	"reflect"
	"service-platform/cmd/web_panel/config"
	"service-platform/cmd/web_panel/fun"
	"service-platform/cmd/web_panel/model"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/go-redis/redis/v8"
	"gorm.io/gorm"
)

func TableMerchantFastlink(db *gorm.DB) gin.HandlerFunc {
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

		t := reflect.TypeOf(model.MerchantFastlink{})

		// Initialize the map
		columnMap := make(map[int]string)

		// Loop through the fields of the struct
		colNum := 0
		for i := 0; i < t.NumField(); i++ {
			field := t.Field(i)
			// Get the JSON key
			jsonKey := field.Tag.Get("json")
			if jsonKey == "" || jsonKey == "-" || jsonKey == "link_wod" {
				continue
			}
			columnMap[colNum] = jsonKey
			colNum++
		}

		// Get the column name based on SortColumn value
		sortColumnName := columnMap[request.SortColumn]
		orderString := fmt.Sprintf("%s %s", sortColumnName, request.SortDir)

		// Initial query for filtering
		filteredQuery := db.Unscoped().Model(&model.MerchantFastlink{})

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
				// fmt.Printf("Variable Name: %s, Data Type: %s, JSON Key: %s, GORM Column Key: %s\n", varName, dataType, jsonKey, columnKey)

				filteredQuery = filteredQuery.Where("`"+dataField+"` LIKE ?", "%"+request.Search+"%")
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
				// fmt.Print("Form Key: ", formKey)
				// fmt.Print("Form Value: ", formValue)

				// if formValue != "" {
				// 	filteredQuery = filteredQuery.Where("`"+formKey+"` LIKE ?", "%"+formValue+"%")
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

		// Count the total number of records
		var totalRecords int64
		db.Model(&model.MerchantFastlink{}).Count(&totalRecords)

		// Count the number of filtered records
		var filteredRecords int64
		filteredQuery.Unscoped().Count(&filteredRecords)

		// Apply sorting and pagination to the filtered query
		query := filteredQuery.Order(orderString)
		var DbData []model.MerchantFastlink
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

		var data []gin.H
		for _, dataInDB := range DbData {
			newData := make(map[string]interface{})

			v := reflect.ValueOf(dataInDB)

			var idData string
			idField := v.FieldByName("ID")
			if idField.IsValid() && idField.CanInterface() {
				idData = fmt.Sprintf("%v", idField.Interface()) // Convert uint to string
			}

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

				// Handle data
				switch {
				case fieldValue.Type() == reflect.TypeOf(time.Time{}):
					t := fieldValue.Interface().(time.Time)
					switch theKey {
					case "birthdate":
						newData[theKey] = t.Format(fun.T_YYYYMMDD)
					case "date":
						newData[theKey] = t.Add(7 * time.Hour).Format(fun.T_YYYYMMDD_HHmmss)
					default:
						newData[theKey] = t.Format(fun.T_YYYYMMDD_HHmmss)
					}
				case theKey == "status":
					switch fieldValue.Int() {
					case 0:
						newData[theKey] = `<div class="text-truncate badge bg-danger" data-i18n="REJECTED">REJECTED</div>`
					case 1:
						newData[theKey] = `<div class="text-truncate badge bg-secondary" data-i18n="PENDING">PENDING</div>`
					case 2:
						newData[theKey] = `<div class="text-truncate badge bg-warning" data-i18n="WAITING">WAITING</div>`
					case 3:
						newData[theKey] = `<div class="text-truncate badge bg-success" data-i18n="ACCEPTED">ACCEPTED</div>`
					default:
						newData[theKey] = `<div class="text-truncate badge bg-dark"></div>`
					}
				case theKey == "approved_status":
					// Check if deleted_at is not null, then set as "Not Approved"
					deletedAtField := v.FieldByName("DeletedAt")
					if deletedAtField.IsValid() && !deletedAtField.IsZero() {
						newData[theKey] = `<div class="text-truncate badge bg-label-danger" data-i18n="NOT_APPROVED">Not Approved</div>`
					} else {
						newData[theKey] = `<div class="text-truncate badge bg-label-success" data-i18n="APPROVE">Approve</div>`
					}
				case theKey == "photos":
					// Btn photos
					newData[theKey] =
						fmt.Sprintf(
							`
							<div class="card-photos">
								<button class="btn btn-sm btn-info" onclick="openPopupPhotosMerchantFastLink('%s', 'merchant_fastlink')">
									<i class='bx bx-image-alt me-2'></i> Photos
								</button>
							</div>
							`, idData,
						)
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

func LastUpdateMerchantFastlink(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		// Get the latest update timestamp from the database
		var lastUpdatedData time.Time
		if err := db.Model(&model.MerchantFastlink{}).
			Select("updated_at").
			Order("updated_at DESC").
			Limit(1).
			Scan(&lastUpdatedData).
			Error; err != nil {
			// If there's an error during the database query
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Error retrieving the last update timestamp: " + err.Error()})
			return
		}

		// If the lastUpdatedData is still zero, return not found error
		if lastUpdatedData.IsZero() {
			c.JSON(http.StatusNotFound, gin.H{"message": "No last updated timestamp found."})
			return
		}

		// Return the last updated timestamp in the required format
		c.JSON(http.StatusOK, gin.H{
			"lastUpdated": lastUpdatedData.Format("2006-01-02 15:04:05"), // Format timestamp
		})
	}
}

func ShowPhotoByIDForMerchantFastlink(rdb *redis.Client, db *gorm.DB) gin.HandlerFunc {
	return func(ctx *gin.Context) {
		data_id := ctx.Param("id")
		if data_id == "" {
			ctx.JSON(http.StatusBadRequest, gin.H{"message": "photo not found for merchant fastlink"})
			return
		}

		var dbData model.MerchantFastlink
		if err := db.First(&dbData, data_id).Error; err != nil {
			ctx.JSON(http.StatusInternalServerError, gin.H{"message": err.Error()})
			return
		}

		colFoto := []string{
			"foto_lokasi_usaha_depan",
			"foto_lokasi_usaha_belakang",
			"foto_lokasi_usaha_produksi",
			"foto_lokasi_usaha_stok",
			"foto_ktp",
			"foto_ttd",
		}

		judulFoto := []string{
			"Foto Depan Lokasi Usaha",
			"Foto Belakang Lokasi Usaha",
			"Foto Lokasi Usaha Produksi",
			"Foto Lokasi Usaha Stok",
			"Foto KTP",
			"Foto TTD",
		}

		var html strings.Builder
		html.WriteString(`
<!DOCTYPE html>
<html lang="en">
<head>
	<meta charset="UTF-8">
	<meta name="viewport" content="width=device-width, initial-scale=1.0">
	<title>Photo Gallery</title>
	<link href="https://cdn.jsdelivr.net/npm/bootstrap@5.3.0/dist/css/bootstrap.min.css" rel="stylesheet">
	<link href="https://cdn.jsdelivr.net/npm/bootstrap-icons@1.10.5/font/bootstrap-icons.css" rel="stylesheet">
	<style>
		body {
			background-color: #f8f9fa;
		}
		.photo-card {
			transition: transform 0.2s ease-in-out;
		}
		.photo-card:hover {
			transform: scale(1.05);
			box-shadow: 0 4px 20px rgba(0,0,0,0.2);
		}
		.data-label {
			font-weight: 600;
			color: #343a40;
		}
	</style>
</head>
<body>
	<script>
		function openImagePopup(url) {
			const width = 400;
			const height = 700;
			const left = 0;
			const top = 20;

			window.open(
				url,
				'imagePopup',
				'width=' + width + ',height=' + height + ',left=' + left + ',top=' + top + ',resizable=yes,scrollbars=yes'
			);
		}
	</script>

	<div class="container mt-4">
		<h2 class="text-center mb-4">Photo Gallery Fastlink's Merchant for data ID: ` + data_id + `</h2>

		<div class="card mb-4 shadow-sm border-0">
	<div class="card-body">
		<div class="row g-3">
			<div class="col-md-6 d-flex">
				<div class="me-3">
					<i class="bi bi-person-lines-fill fs-3 text-secondary"></i>
				</div>
				<div>
				<div><span class="fw-semibold text-muted">Pemilik Usaha:</span> ` + dbData.PemilikUsaha + `</div>
				<div><span class="fw-semibold text-muted">NPWP Badan Usaha:</span> ` + dbData.NPWPBadanUsaha + `</div>
				<div><span class="fw-semibold text-muted">Nama Usaha:</span> ` + dbData.NamaUsaha + `</div>
				</div>
			</div>
			<div class="col-md-6 d-flex">
				<div class="me-3">
					<i class="bi bi-shop-window fs-3 text-secondary"></i>
				</div>
				<div>
					<div><span class="fw-semibold text-muted">Merchant:</span> ` + dbData.MerchantName + `</div>
					<div><span class="fw-semibold text-muted">NMID:</span> ` + dbData.NMID + `</div>
					<div><span class="fw-semibold text-muted">Email:</span> ` + dbData.Email + `</div>
				</div>
			</div>
		</div>
	</div>
</div>

		<div class="row row-cols-1 row-cols-sm-2 row-cols-md-3 g-4">
`)

		publicURLImg := config.GetConfig().FastLinkData.PublicURLMerchantImage
		v := reflect.ValueOf(dbData)
		t := reflect.TypeOf(dbData)

		for i, fieldName := range colFoto {
			fieldVal := v.FieldByNameFunc(func(name string) bool {
				field, _ := t.FieldByName(name)
				tag := field.Tag.Get("json")
				return tag == fieldName
			})

			// fallback: try directly if name matches struct field
			if !fieldVal.IsValid() {
				fieldVal = v.FieldByName(fun.CapitalizeFirstWord(fieldName)) // assumes exported with same name
			}

			// get string value or fallback to empty
			imgPath := ""
			if fieldVal.IsValid() && fieldVal.Kind() == reflect.String {
				imgPath = fieldVal.String()
			}

			html.WriteString(fmt.Sprintf(`
		<div class="col">
			<div class="card photo-card h-100 text-center">
				<img src="%s%s" 
					class="card-img-top" 
					alt="%s"
					style="height:250px; object-fit:contain; cursor:pointer;"
					onclick="openImagePopup(this.src);"
					onerror="this.onerror=null; this.src='/assets/self/img/no-img.jpg';">
				<div class="card-body">
					<h5 class="card-title">%s</h5>
				</div>
			</div>
		</div>
	`, publicURLImg, imgPath, judulFoto[i], judulFoto[i]))
		}

		html.WriteString(`
		</div>
		<!--
		<div class="text-center mt-4">
			<a href="/" class="btn btn-secondary">
				<i class="bi bi-arrow-left-circle me-1"></i> Back to Home
			</a>
		</div>
		-->
	</div>
	<script src="https://cdn.jsdelivr.net/npm/bootstrap@5.3.0/dist/js/bootstrap.bundle.min.js"></>
</body>
</html>
`)

		htmlRendered := html.String()
		ctx.Header("Content-Type", "text/html")
		ctx.String(http.StatusOK, htmlRendered)
	}
}
