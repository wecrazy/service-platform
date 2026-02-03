package controllers

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"reflect"
	"service-platform/cmd/web_panel/config"
	"service-platform/cmd/web_panel/fun"
	"service-platform/cmd/web_panel/model"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/go-redis/redis/v8"
	"github.com/sirupsen/logrus"
	"gorm.io/gorm"
)

var (
	getDataMerchantKresekBagMutex sync.Mutex
)

type OdooKresekBagMerchantItem struct {
	ID                   uint              `json:"id"`
	Name                 nullAbleString    `json:"name"`
	Email                nullAbleString    `json:"email"`
	Agent                nullAbleString    `json:"agent"`
	DataAgent            nullAbleString    `json:"data_agent"`
	VAT                  nullAbleString    `json:"vat"`
	Phone                nullAbleString    `json:"phone"`
	Mobile               nullAbleString    `json:"mobile"`
	TIN                  nullAbleString    `json:"tin"`
	AccHolderNumber      nullAbleString    `json:"acc_holder_number"`
	AccHolderName        nullAbleString    `json:"acc_holder_name"`
	FotoDepan            nullAbleString    `json:"foto_lokasi_usaha_depan"`
	FotoBelakang         nullAbleString    `json:"foto_lokasi_usaha_belakang"`
	FotoProduksi         nullAbleString    `json:"foto_lokasi_usaha_produksi"`
	FotoStok             nullAbleString    `json:"foto_lokasi_usaha_stok"`
	FotoKTP              nullAbleString    `json:"foto_ktp"`
	FotoTTD              nullAbleString    `json:"foto_ttd"`
	ContactAddress       nullAbleString    `json:"contact_address"`
	City                 nullAbleString    `json:"city"`
	MembershipExpiryDate nullAbleTime      `json:"membership_expiry_date"`
	MembershipId         nullAbleInterface `json:"membership_id"`
	BankID               nullAbleInterface `json:"bank_id"`
	CompanyID            nullAbleInterface `json:"company_id"`
}

func (t *OdooKresekBagMerchantItem) UnmarshalJSON(data []byte) error {
	type Alias OdooKresekBagMerchantItem // Create an alias to avoid recursion
	aux := &struct {
		MembershipExpiryDate interface{} `json:"membership_expiry_date"`
		*Alias
	}{
		Alias: (*Alias)(t),
	}

	// Unmarshal into the auxiliary structure
	if err := json.Unmarshal(data, &aux); err != nil {
		return fmt.Errorf("failed to unmarshal JSON: %v", err)
	}

	// Function to parse time fields
	parseTimeField := func(value interface{}) (nullAbleTime, error) {
		switch v := value.(type) {
		case string:
			if v == "" || v == "null" {
				return nullAbleTime{Time: time.Time{}, Valid: false}, nil
			}
			parsedTime, err := time.Parse("2006-01-02 15:04:05", v)
			if err != nil {
				return nullAbleTime{}, fmt.Errorf("failed to parse time: %v", err)
			}
			return nullAbleTime{Time: parsedTime, Valid: true}, nil
		case bool:
			if !v {
				return nullAbleTime{Time: time.Time{}, Valid: false}, nil
			}
			return nullAbleTime{}, errors.New("unexpected boolean value for time field")
		case nil:
			return nullAbleTime{Time: time.Time{}, Valid: false}, nil
		default:
			return nullAbleTime{}, fmt.Errorf("unexpected type: %T", value)
		}
	}

	// Parse each time field separately
	var err error

	if t.MembershipExpiryDate, err = parseTimeField(aux.MembershipExpiryDate); err != nil {
		return fmt.Errorf("MembershipExpiryDate: %v", err)
	}

	return nil
}

func GetMerchantKresekBag(db *gorm.DB) (string, error) {
	// taskDoing := "Get Merchant KresekBag Data"
	taskDoing := "Get Merchant Data"

	if !getDataMerchantKresekBagMutex.TryLock() {
		return "", fmt.Errorf("process %s already running, skipping request", taskDoing)
	}
	defer getDataMerchantKresekBagMutex.Unlock()

	logrus.Infof("Fetching %s", taskDoing)

	odooModel := "res.partner"
	odooDomain := []interface{}{
		[]interface{}{"id", "!=", 0},
		[]interface{}{"active", "=", true},
		// FIX: this maybe not using agent ?
		[]interface{}{"agent", "=", "soundbox"},
	}
	odooFields := []string{
		"id",                     // uint
		"name",                   // char
		"email",                  // char
		"agent",                  // char
		"data_agent",             // text
		"vat",                    // char
		"phone",                  // char
		"mobile",                 // char
		"email",                  // char
		"tin",                    // char
		"membership_id",          // many2one
		"membership_expiry_date", // date
		// "acc_number", use res.parter.bank
		"bank_id",                    // many2one
		"acc_holder_number",          // char
		"acc_holder_name",            // char
		"foto_lokasi_usaha_depan",    // text
		"foto_lokasi_usaha_belakang", // text
		"foto_lokasi_usaha_produksi", // text
		"foto_lokasi_usaha_stok",     // text
		"foto_ktp",                   // text
		"foto_ttd",                   // text
		"contact_address",            // char
		"city",                       // char
		"company_id",                 // many2one
	}

	odooOrder := "id asc"

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
		return "", fmt.Errorf("error marshalling payload: %v", err)
	}
	url := config.GetConfig().ApiODOO.UrlGetDataKresekBag
	method := "POST"

	body, err := FetchODOOKresekBag(url, method, string(payloadBytes))
	if err != nil {
		return "", fmt.Errorf("%v", err)
	}

	var jsonResponse map[string]interface{}
	err = json.Unmarshal(body, &jsonResponse)
	if err != nil {
		return "", fmt.Errorf("error unmarshalling JSON response: %v", err)
	}

	if errorResponse, ok := jsonResponse["error"].(map[string]interface{}); ok {
		if errorMessage, ok := errorResponse["message"].(string); ok && errorMessage == "Odoo Session Expired" {
			return "", errors.New("odoo error: session expired")
		} else {
			return "", fmt.Errorf("odoo Error: %v", errorResponse)
		}
	}

	if result, ok := jsonResponse["result"].(map[string]interface{}); ok {
		if message, ok := result["message"].(string); ok {
			success, successOk := result["success"]
			logrus.Infof("ODOO Result, message: %v, status: %v", message, successOk && success == true)
		}
	}
	// Check for the existence and validity of the "result" field
	result, resultExists := jsonResponse["result"]
	if !resultExists {
		return "", fmt.Errorf("'result' field not found in the response: %v", jsonResponse)
	}
	// Check if the result is an array and ensure it's not empty
	resultArray, ok := result.([]interface{})
	if !ok {
		return "", fmt.Errorf("'result' is not an array or is empty: %v", result)
	}
	if len(resultArray) == 0 {
		return "", errors.New("no data found in the ODOO response")
	}
	for i, item := range resultArray {
		itemMap, ok := item.(map[string]interface{})
		if !ok {
			logrus.Errorf("item %d is not a map: %v", i, item)
			continue
		}

		var odooData OdooKresekBagMerchantItem
		jsonData, err := json.Marshal(itemMap)
		if err != nil {
			logrus.Errorf("error marshalling item %d: %v", i, err)
			continue
		}

		err = json.Unmarshal(jsonData, &odooData)
		if err != nil {
			logrus.Errorf("error unmarshalling item %d: %v", i, err)
			continue
		}

		companyId, company, err := parseJSONIDDataCombined(odooData.CompanyID)
		if err != nil {
			logrus.Errorf("error parsing company, from: %v got %v", odooData.CompanyID, err)
		}
		bankId, bank, err := parseJSONIDDataCombined(odooData.BankID)
		if err != nil {
			logrus.Errorf("error parsing bank, from: %v got %v", odooData.BankID, err)
		}
		membershipId, membership, err := parseJSONIDDataCombined(odooData.MembershipId)
		if err != nil {
			logrus.Errorf("error parsing membership, from: %v got %v", odooData.MembershipId, err)
		}

		var membershipExpiryDate *time.Time
		if !odooData.MembershipExpiryDate.Time.IsZero() {
			membershipExpiryDate = &odooData.MembershipExpiryDate.Time
		}

		dataMerchant := model.MerchantKresekBag{
			ID:                      odooData.ID,
			CustomerName:            odooData.Name.String,
			CustomerEmail:           odooData.Email.String,
			CustomerPhone:           odooData.Phone.String,
			CustomerCity:            odooData.City.String,
			CustomerAddress:         odooData.ContactAddress.String,
			TaxId:                   odooData.VAT.String,
			TaxIdentificationNumber: odooData.TIN.String,
			Agent:                   odooData.Agent.String,
			AccountNumber:           odooData.AccHolderNumber.String,
			AccountName:             odooData.AccHolderName.String,
			MembershipExpiryDate:    membershipExpiryDate,
			MembershipId:            membershipId,
			Membership:              membership,
			BankId:                  bankId,
			Bank:                    bank,
			CompanyId:               companyId,
			Company:                 company,
			FotoDepan:               odooData.FotoDepan.String,
			FotoBelakang:            odooData.FotoBelakang.String,
			FotoProduksi:            odooData.FotoProduksi.String,
			FotoStok:                odooData.FotoStok.String,
			FotoKtp:                 odooData.FotoKTP.String,
			FotoTtd:                 odooData.FotoTTD.String,
		}

		if err := db.First(&model.MerchantKresekBag{}, "id = ?", odooData.ID).Error; err != nil {
			if err == gorm.ErrRecordNotFound {
				if err := db.Create(&dataMerchant).Error; err != nil {
					logrus.Errorf("Failed to create new data of %s : %v", taskDoing, err)
				} else {
					logrus.Infof("Created new Data for %s with ID: %d", taskDoing, odooData.ID)
				}
			} else {
				logrus.Errorf("Failed to check for %s : %v", taskDoing, err)
			}
		} else {
			if err := db.Model(&model.MerchantKresekBag{}).
				Where("id = ?", odooData.ID).
				Updates(dataMerchant).Error; err != nil {
				logrus.Errorf("Failed to update Data for %s %d: %v", taskDoing, odooData.ID, err)
			} else {
				// logrus.Infof("Updated Data for %s with ID: %d", taskDoing, odooData.ID)
			}
		}

	}
	return fmt.Sprintf("%s successfully updated @%v", taskDoing, time.Now().Format("15:04:05, 02 Jan 2006")), nil
}

func RefreshMerchantKresekBag(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		result, err := GetMerchantKresekBag(db)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}

		c.JSON(http.StatusOK, gin.H{"message": result})
	}
}

func LastUpdateMerchantKresekBag(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		// Get the latest update timestamp from the database
		var lastUpdatedData time.Time
		if err := db.Model(&model.MerchantKresekBag{}).
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

func TableMerchantKresekBag(db *gorm.DB) gin.HandlerFunc {
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

		t := reflect.TypeOf(model.MerchantKresekBag{})

		// Initialize the map
		columnMap := make(map[int]string)

		// Loop through the fields of the struct
		colNum := 0
		for i := 0; i < t.NumField(); i++ {
			field := t.Field(i)
			// Get the JSON key
			jsonKey := field.Tag.Get("json")
			if jsonKey == "" || jsonKey == "-" || jsonKey == "link_wod" || jsonKey == "photos" {
				continue
			}
			columnMap[colNum] = jsonKey
			colNum++
		}

		// Get the column name based on SortColumn value
		sortColumnName := columnMap[request.SortColumn]
		orderString := fmt.Sprintf("%s %s", sortColumnName, request.SortDir)

		// Initial query for filtering
		filteredQuery := db.Unscoped().Model(&model.MerchantKresekBag{})

		// // Apply filters
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
				if jsonKey == "" || jsonKey == "-" || jsonKey == "photos" {
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
		db.Model(&model.MerchantKresekBag{}).Count(&totalRecords)

		// Count the number of filtered records
		var filteredRecords int64
		filteredQuery.Unscoped().Count(&filteredRecords)

		// Apply sorting and pagination to the filtered query
		query := filteredQuery.Order(orderString)
		var DbData []model.MerchantKresekBag
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
				case theKey == "photos":
					// Btn photos
					newData[theKey] =
						fmt.Sprintf(
							`
							<div class="card-photos">
								<button class="btn btn-sm btn-info" onclick="openPopupPhotosMerchantKresekBag('%s')">
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

func ShowPhotoByIDForMerchantKresekBag(rdb *redis.Client, db *gorm.DB) gin.HandlerFunc {
	return func(ctx *gin.Context) {
		data_id := ctx.Param("id")
		if data_id == "" {
			ctx.JSON(http.StatusBadRequest, gin.H{"message": "photo not found for merchant KresekBag"})
			return
		}

		var dbData model.MerchantKresekBag
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
		<h2 class="text-center mb-4">Photo Gallery Merchant for data ID: ` + data_id + `</h2>

		<div class="card mb-4 shadow-sm border-0">
	<div class="card-body">
		<div class="row g-3">
			<div class="col-md-6 d-flex">
				<div class="me-3">
					<i class="bi bi-person-lines-fill fs-3 text-secondary"></i>
				</div>
				<div>
				<div><span class="fw-semibold text-muted">Agen:</span> ` + dbData.Agent + `</div>
				<div><span class="fw-semibold text-muted">Bank:</span> ` + dbData.Bank + `</div>
				<div><span class="fw-semibold text-muted">Kota:</span> ` + dbData.CustomerCity + `</div>
				</div>
			</div>
			<div class="col-md-6 d-flex">
				<div class="me-3">
					<i class="bi bi-shop-window fs-3 text-secondary"></i>
				</div>
				<div>
					<div><span class="fw-semibold text-muted">Customer:</span> ` + dbData.CustomerName + `</div>
					<div><span class="fw-semibold text-muted">TIN:</span> ` + dbData.TaxIdentificationNumber + `</div>
					<div><span class="fw-semibold text-muted">Email:</span> ` + dbData.CustomerEmail + `</div>
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
