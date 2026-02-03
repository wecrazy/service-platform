package controllers

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"reflect"
	"regexp"
	"service-platform/cmd/technical_assistance/fun"
	"service-platform/cmd/technical_assistance/model"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/xuri/excelize/v2"
	"gorm.io/gorm"
)

func GetTeknisiList(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {

		// var logActivities []model.LogActivity
		// result := db.Find(&logActivities)

		// if result.Error != nil {
		// 	c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to retrieve Data " + result.Error.Error()})
		// 	return
		// }

		// if len(logActivities) == 0 {
		// 	c.JSON(http.StatusOK, gin.H{
		// 		"data": []gin.H{},
		// 	})
		// 	return
		// }

		// // Loop through the lines and process them as needed
		// var data []gin.H
		// for _, logActivity := range logActivities {
		// 	createdAtString := logActivity.CreatedAt.Format("2006-01-02 15:04:05")
		// 	data = append(data, gin.H{
		// 		"date_time": createdAtString,
		// 		"action":    logActivity.Action,
		// 		"fullname":  logActivity.FullName,
		// 		"status":    logActivity.Status,
		// 		"detail":    logActivity.Log,
		// 	})
		// }

		// // Respond with the formatted data
		// c.JSON(http.StatusOK, gin.H{
		// 	"data": data,
		// })
	}
}

// func ListTeknisiName(db *gorm.DB) gin.HandlerFunc {
// 	return func(c *gin.Context) {
// 		var teknisiNames []string

// 		// Use db.Select to fetch only the full_name and map it directly to the slice of strings
// 		if err := db.Model(&model.Teknisi{}).Select("full_name").Pluck("full_name", &teknisiNames).Error; err != nil {
// 			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
// 			return
// 		}

// 		// Respond with the slice of full names
// 		c.JSON(http.StatusOK, teknisiNames)
// 	}
// }
// func ListSerialNumber(db *gorm.DB) gin.HandlerFunc {
// 	return func(c *gin.Context) {
// 		var serial_number []string

// 		// Use db.Select to fetch only the full_name and map it directly to the slice of strings
// 		if err := db.Model(&model.Edc{}).Select("SerialNumber").Pluck("SerialNumber", &serial_number).Error; err != nil {
// 			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
// 			return
// 		}

// 		// Respond with the slice of full names
// 		c.JSON(http.StatusOK, serial_number)
// 	}
// }
// func ListNamaAplikasi(db *gorm.DB) gin.HandlerFunc {
// 	return func(c *gin.Context) {
// 		appName := c.Query("a")
// 		var application model.Application
// 		db.Where("ApplicationName = ?", appName).First(&application)
// 		var response []string
// 		if application.ID == 0 {
// 			c.JSON(http.StatusBadRequest, response)
// 			return

// 		}

// 		// Use db.Select to fetch only the full_name and map it directly to the slice of strings
// 		if err := db.Model(&model.File{}).Select("VersionName").Pluck("VersionName", &response).Error; err != nil {
// 			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
// 			return
// 		}

// 		// Respond with the slice of full names
// 		c.JSON(http.StatusOK, response)
// 	}
// }

func PostUnlockSerialNumber(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		save := c.Query("save")
		var req struct {
			NamaTeknisi   string `json:"nama_teknisi"`
			SPKNumber     string `json:"spk_number"`
			WONumber      string `json:"wo_number"`
			SerialNumber  string `json:"serial_number"`
			NamaAplikasi  string `json:"nama_aplikasi"`
			VersiAplikasi string `json:"versi_aplikasi"`
			TID           string `json:"tid"`
			MID           string `json:"mid"`
			Kunci         string `json:"kunci"`
			Remark        string `json:"remark"`
		}

		// Bind the incoming JSON data to the UnlockRequest struct
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		// Check for empty fields
		if req.NamaTeknisi == "" || req.SerialNumber == "" ||
			req.NamaAplikasi == "" || req.VersiAplikasi == "" || req.Kunci == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Mohon Isi Field yang sesuai"})
			return
		}

		// Retrieve cookies
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

		var admin model.Admin
		db.Where("id = ? ", uint(claims["id"].(float64))).Find(&admin)
		if admin.ID == 0 {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "User ID Not Found"})
			return
		}
		// Generate the unlock code
		kodeBuka := fun.GenerateUnlockSNCode(req.Kunci)
		if save == "1" {
			paramNamaMerchant := ""
			paramAddr1 := ""
			paramAddr2 := ""
			paramAddr3 := ""

			if req.NamaAplikasi != "" && req.SerialNumber != "" {

				var app model.Application
				if err := db.Where("ApplicationName = ?", req.NamaAplikasi).First(&app).Error; err == nil {
					// Fetch param variables
					var param_variables []model.ParamVariable
					if err := db.Where("SerialNumber = ? AND PackageName = ?", req.SerialNumber, app.PackageName).Find(&param_variables).Error; err == nil {
						for _, param := range param_variables {
							lowerKey := strings.ToLower(param.Key)
							// kalau mengandung merchant  | mau merchantX , memerchant, mau memerrrrrmerchant bakal masuk
							if strings.Contains(lowerKey, "merchant") {
								paramNamaMerchant = param.Value
							}
							// kalau mengandung addr dan 1  | mau address1 , addrajiwa1, 1332addr bakal masuk
							if strings.Contains(lowerKey, "addr") && strings.Contains(lowerKey, "1") {
								paramAddr1 = param.Value
							}

							// kalau mengandung addr dan 2  | mau address2 , addrajiwa1, 1332addr bakal masuk
							if strings.Contains(lowerKey, "addr") && strings.Contains(lowerKey, "2") {
								paramAddr2 = param.Value
							}

							// kalau mengandung addr dan 3  | mau address3 , addrajiwa3, 1332addr bakal masuk
							if strings.Contains(lowerKey, "addr") && strings.Contains(lowerKey, "3") {
								paramAddr3 = param.Value
							}
						}
					}

				}

			}

			var edc model.Edc
			db.Where("SerialNumber = ?", req.SerialNumber).First(&edc)
			// Query contains ':' but not '('
			id_employee := req.NamaTeknisi
			nama_teknisi := req.NamaTeknisi
			service_poin := req.NamaTeknisi
			parts := strings.Split(req.NamaTeknisi, ":")
			if len(parts) > 1 {
				id_employee = parts[0]
				parts2 := strings.Split(parts[1], "(")
				nama_teknisi = parts2[0]
				service_poin = strings.ReplaceAll(parts2[1], ")", "")
			}
			kunjungan := model.TeknisiKunjungan{
				NamaAdmin:         admin.Username,
				IdEmployee:        id_employee,
				NamaTeknisi:       nama_teknisi,
				ServicePoin:       service_poin,
				SPKNumber:         req.SPKNumber,
				WONumber:          req.WONumber,
				SerialNumber:      req.SerialNumber,
				NamaAplikasi:      req.NamaAplikasi,
				MerchantName:      edc.TerminalName,
				VersiAplikasi:     req.VersiAplikasi,
				TID:               req.TID,
				MID:               req.MID,
				Kunci:             req.Kunci,
				Remark:            req.Remark,
				ParamNamaMerchant: paramNamaMerchant,
				ParamAddr1:        paramAddr1,
				ParamAddr2:        paramAddr2,
				ParamAddr3:        paramAddr3,
			}

			// Save to the database using GORM
			if err := db.Create(&kunjungan).Error; err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to save data"})
				return
			}
			jsonString := ""
			jsonData, err := json.Marshal(kunjungan)
			if err != nil {
				fmt.Println("Error converting to JSON:", err)
			} else {
				jsonString = string(jsonData)
			}
			db.Create(&model.LogActivity{
				AdminID:   admin.ID,
				FullName:  admin.Fullname,
				Action:    "Save Input Data Kunjungan",
				Status:    "Success",
				Log:       "Data Kunjungan Berhasil Di Simpan : " + jsonString,
				IP:        c.ClientIP(),
				UserAgent: c.Request.UserAgent(),
				ReqMethod: c.Request.Method,
				ReqUri:    c.Request.RequestURI,
			})
		}

		// Respond with the kode_buka
		c.JSON(http.StatusOK, map[string]any{
			"kode_buka": kodeBuka,
		})
	}
}
func ListTeknisiName(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		query := c.Query("q") // Get the query parameter

		// Define a slice to hold the results
		var teknisis []struct {
			ServicePoin string `json:"service_poin" gorm:"column:service_poin"`
			FullName    string `json:"full_name" gorm:"column:full_name"`
			IdEmployee  int    `json:"id_employee" gorm:"column:id_employee"`
		}

		// Start building the query
		tx := db.Model(&model.Teknisi{}).Select("full_name, id_employee, service_poin").Limit(15)

		// Add filters based on the query
		if query != "" {
			re := regexp.MustCompile(`^\d+$`)
			if re.MatchString(query) {
				// Query contains only digits
				tx = tx.Where("id_employee LIKE ?", "%"+query+"%")
				tx = tx.Or("service_poin LIKE ?", "%"+query+"%")
			} else if strings.Contains(query, ":") && !strings.Contains(query, "(") {
				// Query contains ':' but not '('
				parts := strings.Split(query, ":")
				if len(parts) > 1 {
					tx = tx.Where("full_name LIKE ?", "%"+parts[1]+"%")
					tx = tx.Or("id_employee LIKE ?", "%"+parts[0]+"%")
				}
			} else if strings.Contains(query, "(") && !strings.Contains(query, ":") {
				// Query contains '(' but not ':'
				parts := strings.Split(query, "(")
				if len(parts) > 1 {
					tx = tx.Where("full_name LIKE ?", "%"+parts[0]+"%")
					tx = tx.Or("service_poin LIKE ?", "%"+parts[1]+"%")
				}
			} else {
				// Generic case
				tx = tx.Where("full_name LIKE ?", "%"+query+"%")
				tx = tx.Or("id_employee LIKE ?", "%"+query+"%")
				tx = tx.Or("service_poin LIKE ?", "%"+query+"%")
			}
		}

		// Execute the query
		if err := tx.Find(&teknisis).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}

		// Format the response
		var response []string
		for _, teknisi := range teknisis {
			response = append(response, fmt.Sprintf("%d:%s(%s)", teknisi.IdEmployee, teknisi.FullName, teknisi.ServicePoin))
		}

		// Send the response
		c.JSON(http.StatusOK, response)
	}
}

func ListSerialNumber(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		query := c.Query("q") // Get the query parameter
		var serialNumbers []string

		// Start the query
		tx := db.Model(&model.Edc{}).Limit(15) // Limit the result to 15 records

		// If query is provided, filter by SerialNumber
		if query != "" {
			tx = tx.Where("SerialNumber LIKE ?", "%"+query+"%")
		}

		// Execute the query and pluck SerialNumbers
		if err := tx.Pluck("SerialNumber", &serialNumbers).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}

		// Respond with the filtered or all serial numbers
		c.JSON(http.StatusOK, serialNumbers)
	}
}

func ListNamaAplikasi(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		// Create a slice to store results
		var apps []struct {
			ApplicationName string `json:"ApplicationName"`
			PackageName     string `json:"PackageName"`
		}

		// Query to get both ApplicationName and PackageName
		if err := db.Model(&model.Application{}).
			Select("ApplicationName", "PackageName"). // Specify both fields to be selected
			Find(&apps).Error; err != nil {           // Use Find to get all results
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}

		// Respond with the list of ApplicationNames and PackageNames
		c.JSON(http.StatusOK, apps)
	}
}

// func ListVersiAplikasi(db *gorm.DB) gin.HandlerFunc {
// 	return func(c *gin.Context) {
// 		query := c.Query("app")
// 		var versionNames []string

// 		// Use db.Where to search based on the query
// 		if err := db.Model(&model.Application{}).
// 			Where("VersionName LIKE ?", "%"+query+"%").
// 			Pluck("VersionName", &versionNames).Error; err != nil {
// 			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
// 			return
// 		}

//			// Respond with the filtered version names
//			c.JSON(http.StatusOK, versionNames)
//		}
//	}
func GetTidMid(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		appName := c.Query("app")
		serialNumber := c.Query("sn")

		var app model.Application
		db.Where("ApplicationName = ?", appName).First(&app)

		var tid model.ParamVariable
		db.Where("SerialNumber = ? AND PackageName = ? AND `key` = ?", serialNumber, app.PackageName, "TID").First(&tid)
		var mid model.ParamVariable
		db.Where("SerialNumber = ? AND PackageName = ? AND `key` = ?", serialNumber, app.PackageName, "MID").First(&mid)

		// Respond with the slice of full names
		c.JSON(http.StatusOK, gin.H{
			"tid": tid.Value,
			"mid": mid.Value,
		})
	}
}
func GetSnInfo(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		appName := c.Query("app")
		serialNumber := c.Query("sn")

		// Find the application by app name
		var app model.Application
		db.Where("ApplicationName = ?", appName).First(&app)

		// Find the parameters based on the serial number and package name
		var params []model.ParamVariable
		db.Where("SerialNumber = ? AND PackageName = ? ", serialNumber, app.PackageName).Find(&params)

		// Build an HTML table with the parameters
		var info strings.Builder
		info.WriteString("<table class='w-100'>")

		for _, param := range params {
			// Check if param.Key contains "ADDR" or "MERCHANT"
			if strings.Contains(param.Key, "MERCHANT") {
				info.WriteString(`<tr style="border-top: 1px solid #ccc;">`)
				info.WriteString(fmt.Sprintf(
					`<td style="padding: 10px 20px;">%s</td><td style="padding: 10px 20px;">%s</td>`,
					param.Key, param.Value))
				info.WriteString("</tr>")
				break
			}
		}
		for _, param := range params {
			// Check if param.Key contains "ADDR" or "MERCHANT"
			if strings.Contains(param.Key, "ADDR") {
				info.WriteString(`<tr style="border-top: 1px solid #ccc;">`)
				info.WriteString(fmt.Sprintf(
					`<td style="padding: 10px 20px;">%s</td><td style="padding: 10px 20px;">%s</td>`,
					param.Key, param.Value))
				info.WriteString("</tr>")
			}
		}
		info.WriteString("</table>")

		// Respond with the slice of full names
		c.JSON(http.StatusOK, gin.H{
			"info": info.String(),
		})
	}
}
func ListVersiAplikasi(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		app := c.Query("app")

		var application model.Application
		db.Where("ApplicationName = ?", app).First(&application)
		var file []string
		// Use db.Select to fetch only the full_name and map it directly to the slice of strings
		if err := db.Model(&model.File{}).Where("AppID = ?", application.ID).Select("VersionName").Pluck("VersionName", &file).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}

		// Respond with the slice of full names
		c.JSON(http.StatusOK, file)
	}
}

func PostKunjunganTeknisiList(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		var request struct {
			Draw       int    `form:"draw"`
			Start      int    `form:"start"`
			Length     int    `form:"length"`
			Search     string `form:"search[value]"`
			SortColumn int    `form:"order[0][column]"`
			SortDir    string `form:"order[0][dir]"`
		}

		// Bind form data to request struct
		if err := c.Bind(&request); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}

		t := reflect.TypeOf(model.TeknisiKunjungan{})

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
		filteredQuery := db.Model(&model.TeknisiKunjungan{})

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
				if gormTag == "" || gormTag == "-" {
					continue
				}

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

				filteredQuery = filteredQuery.Or("`"+dataField+"` LIKE ?", "%"+request.Search+"%")

			}

		} else {
			for i := 0; i < t.NumField(); i++ {
				field := t.Field(i)
				formKey := field.Tag.Get("form")
				if formKey == "" || formKey == "-" {
					continue
				}
				formValue := c.PostForm(formKey)
				if formValue != "" {
					filteredQuery = filteredQuery.Debug().Or("`"+formKey+"` LIKE ?", "%"+formValue+"%")
				}
			}

		}

		// Count the total number of records
		var totalRecords int64
		db.Model(&model.TeknisiKunjungan{}).Count(&totalRecords)

		// Count the number of filtered records
		var filteredRecords int64
		filteredQuery.Count(&filteredRecords)

		// Apply sorting and pagination to the filtered query
		query := filteredQuery.Order(orderString)
		var TeknisiKunjungans []model.TeknisiKunjungan
		query = query.Offset(request.Start).Limit(request.Length).Find(&TeknisiKunjungans)

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
		for _, person := range TeknisiKunjungans {
			newData := make(map[string]interface{})

			v := reflect.ValueOf(person)

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
					} else {
						newData[theKey] = fieldValue.Interface().(time.Time).Format(fun.T_YYYYMMDD_HHmmss)
					}
				} else if theKey == "full_name" {
					newData[theKey] = fmt.Sprintf(`<a href="#" onclick="Teknisi_ID=%d;fetchAndUpdatePinPoints();">%s</a>`, person.ID, fieldValue.Interface().(string))
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
func PostTeknisiList(db *gorm.DB) gin.HandlerFunc {
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

		t := reflect.TypeOf(model.Teknisi{})

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
		filteredQuery := db.Model(&model.Teknisi{})

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
				if dataType != "string" {
					continue
				}
				// fmt.Printf("Variable Name: %s, Data Type: %s, JSON Key: %s, GORM Column Key: %s\n", varName, dataType, jsonKey, columnKey)

				filteredQuery = filteredQuery.Or("`"+dataField+"` LIKE ?", "%"+request.Search+"%")

			}

		} else {
			for i := 0; i < t.NumField(); i++ {
				field := t.Field(i)
				formKey := field.Tag.Get("form")
				if formKey == "" || formKey == "-" {
					continue
				}
				formValue := c.PostForm(formKey)
				if formValue != "" {
					filteredQuery = filteredQuery.Debug().Or("`"+formKey+"` LIKE ?", "%"+formValue+"%")
				}
			}

		}

		// Count the total number of records
		var totalRecords int64
		db.Model(&model.Teknisi{}).Count(&totalRecords)

		// Count the number of filtered records
		var filteredRecords int64
		filteredQuery.Count(&filteredRecords)

		// Apply sorting and pagination to the filtered query
		query := filteredQuery.Order(orderString)
		var Teknisis []model.Teknisi
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
		for _, person := range Teknisis {
			newData := make(map[string]interface{})

			v := reflect.ValueOf(person)

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
					} else {
						newData[theKey] = fieldValue.Interface().(time.Time).Format(fun.T_YYYYMMDD_HHmmss)
					}
				} else if theKey == "full_name" {
					newData[theKey] = fmt.Sprintf(`<a href="#" onclick="Teknisi_ID=%d;fetchAndUpdatePinPoints();">%s</a>`, person.ID, fieldValue.Interface().(string))
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

func DeleteTeknisi(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		// Get the ID from the URL parameter and convert to integer
		idParam := c.Param("id")
		id, err := strconv.Atoi(idParam)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid Data"})
			return
		}

		// Find the record by ID
		var teknisi model.Teknisi
		if err := db.First(&teknisi, id).Error; err != nil {
			if err == gorm.ErrRecordNotFound {
				// If the record does not exist, return a 404 error
				c.JSON(http.StatusNotFound, gin.H{"error": "Teknisi not found"})
			} else {
				// Handle other potential errors from the database
				c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to find Teknisi"})
			}
			return
		}

		// Perform the deletion
		if err := db.Delete(&teknisi).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to delete Teknisi"})
			return
		}

		// Respond with success
		c.JSON(http.StatusOK, gin.H{"message": "Teknisi deleted successfully"})

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
		jsonData, err := json.Marshal(teknisi)
		if err != nil {
			fmt.Println("Error converting to JSON:", err)
		} else {
			jsonString = string(jsonData)
		}
		db.Create(&model.LogActivity{
			AdminID:   uint(claims["id"].(float64)),
			FullName:  claims["fullname"].(string),
			Action:    "Delete Data Teknisi",
			Status:    "Success",
			Log:       "Data Teknisi Berhasil Di Hapus : " + jsonString,
			IP:        c.ClientIP(),
			UserAgent: c.Request.UserAgent(),
			ReqMethod: c.Request.Method,
			ReqUri:    c.Request.RequestURI,
		})
	}
}

func GetTeknisiMaps(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		// Get query parameters
		id := c.Query("id")
		startDate := c.Query("start") // Format: dd/mm/yyyy
		endDate := c.Query("end")     // Format: dd/mm/yyyy

		var teknisiLocations []model.TeknisiLocation

		// Prepare the query
		query := db.Where("teknisi_id = ?", id)

		// Parse the start and end dates if provided
		if startDate != "" {
			start, err := time.Parse("02/01/2006", startDate)
			if err != nil {
				c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid start date format"})
				return
			}
			if endDate != "" {
				// If end date is provided, filter between start and end dates
				end, err := time.Parse("02/01/2006", endDate)
				if err != nil {
					c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid end date format"})
					return
				}
				query = query.Where("local_time BETWEEN ? AND ?", start, end.Add(24*time.Hour)) // End of day for endDate
			} else {
				// If only start date is provided, filter only for that date
				query = query.Where("DATE(local_time) = ?", start.Format("2006-01-02"))
			}
		}

		// Execute the query
		result := query.Find(&teknisiLocations)
		if result.Error != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to retrieve data: " + result.Error.Error()})
			return
		}

		// If no records are found, return an empty response
		if len(teknisiLocations) == 0 {
			c.JSON(http.StatusOK, gin.H{"message": "No data found"})
			return
		}
		var teknisi model.Teknisi
		db.Where("id = ?", id).First(&teknisi)
		// Loop through the results and populate the response struct
		var response []map[string]any
		for _, location := range teknisiLocations {
			response = append(response, map[string]any{
				"id":         location.ID,
				"name":       teknisi.FullName,
				"no":         location.No,
				"teknisi_id": location.TeknisiID,
				"lat":        location.Lat,
				"long":       location.Long,
				"local_time": location.LocalTime.Format(fun.T_YYYYMMDD_HHmmss),
				"epoch_time": location.CreatedAt.UnixMilli(),
			})
		}

		// Respond with the formatted data
		c.JSON(http.StatusOK, response)
	}
}
func GetBatchTemplate[T any](db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		var tableInstance T
		// Create a new Excel file in memory
		f := excelize.NewFile()
		sheetName := "Sheet1"

		// Use reflection to generate CSV headers
		t := reflect.TypeOf(tableInstance)
		if t.Kind() == reflect.Ptr {
			t = t.Elem()
		}
		structName := t.Name()
		f.SetCellValue(sheetName, "A1", "Batch Upload "+fun.AddSpaceBeforeUppercase(structName))
		// var tableHeaders []string
		for i := 0; i < t.NumField(); i++ {
			field := t.Field(i)
			jsonKey := field.Tag.Get("json")
			if jsonKey == "" || jsonKey == "-" {
				continue
			}
			if i == 0 {
				continue
			}
			fillInfo := ""
			fieldType := field.Type
			if fieldType == reflect.TypeOf(time.Time{}) {
				time_format := field.Tag.Get("time_format")
				if time_format != "" {
					humanReadableFormat := strings.ReplaceAll(time_format, "20", "YY")
					humanReadableFormat = strings.ReplaceAll(humanReadableFormat, "06", "YY")
					humanReadableFormat = strings.ReplaceAll(humanReadableFormat, "15", "HH")
					humanReadableFormat = strings.ReplaceAll(humanReadableFormat, "04", "mm")
					humanReadableFormat = strings.ReplaceAll(humanReadableFormat, "05", "ss")
					humanReadableFormat = strings.ReplaceAll(humanReadableFormat, "01", "MM")
					humanReadableFormat = strings.ReplaceAll(humanReadableFormat, "02", "DD")
					fillInfo = "(" + humanReadableFormat + ")"
				} else {
					fillInfo = "(YYYY-MM-DD)(YYYY-MM-DD HH:mm)"
				}
			}
			// Add data to specific cells
			f.SetCellValue(sheetName, fun.NumberToAlphabet(i)+"2", fun.AddSpaceBeforeUppercase(field.Name)+" "+fillInfo)
		}

		// Write the file content to an in-memory buffer
		var buffer bytes.Buffer
		if err := f.Write(&buffer); err != nil {
			c.String(http.StatusInternalServerError, "Failed to create Excel file: %v", err)
			return
		}

		// Set the necessary headers for file download
		c.Header("Content-Type", "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet")
		c.Header("Content-Disposition", fmt.Sprintf("attachment; filename=batch_upload_%s.xlsx", fun.ToSnakeCase(structName)))

		// Stream the Excel file to the response
		_, err := c.Writer.Write(buffer.Bytes())
		if err != nil {
			c.String(http.StatusInternalServerError, "Failed to write Excel file to response: %v", err)
		}
	}
}

func PostNewTeknisi(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {

		table := "teknisi" // c.Param("table")
		// Check if the table exists
		if !db.Migrator().HasTable(table) {
			fmt.Printf("Table %s does not exist.\n", table)
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid table name, no table named " + table})
			return
		}

		// Use GORM to execute SELECT * LIMIT 1
		var columns map[string]interface{}
		err := db.Raw("SELECT * FROM " + table + " LIMIT 1").Scan(&columns).Error
		if err != nil {
			fmt.Println("Error fetching data:", err)
			return
		}

		// Bind the incoming form data to a map to check keys dynamically
		var formData map[string][]string
		if err := c.Bind(&formData); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid form data " + err.Error()})
			return
		}
		fmt.Println(formData)

		hasCreatedAt := false
		hasUpdatedAt := false
		// Prepare a map for the column names and their nullability status
		// columnMap := make(map[string]bool)
		for column := range columns {
			if column == "created_at" {
				hasCreatedAt = true
			}
			if column == "updated_at" {
				hasUpdatedAt = true
			}
		}

		// Create the struct dynamically based on the table (if possible)
		// NOTE: This part can be simplified if you have a predefined model for the table
		pg_param_db_model := make(map[string]interface{})
		for key, values := range c.Request.Form {
			// Assuming each field has only one value, pick the first one
			if len(values) > 0 {
				pg_param_db_model[key] = values[0] // Add the first value to the map
			}
		}
		if hasCreatedAt {
			pg_param_db_model["created_at"] = time.Now()
		}
		if hasUpdatedAt {
			pg_param_db_model["updated_at"] = time.Now()
		}
		// fmt.Println(pg_param_db_model)
		// Insert data into the table
		if err := db.Table(table).Create(&pg_param_db_model).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to insert data"})
			return
		}

		c.JSON(http.StatusOK, gin.H{"message": "Data inserted successfully"})
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

		db.Create(&model.LogActivity{
			AdminID:   uint(claims["id"].(float64)),
			FullName:  claims["fullname"].(string),
			Email:     claims["email"].(string),
			Action:    "CREATE",
			Status:    "Success",
			Log:       fmt.Sprintf("CREATE New Data @ Table: %s;", table),
			IP:        c.ClientIP(),
			UserAgent: c.Request.UserAgent(),
			ReqMethod: c.Request.Method,
			ReqUri:    c.Request.RequestURI,
		})
	}
}
func PutTeknisiList(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {

		table := "teknisi" // c.Param("table")
		// Check if the table exists
		if !db.Migrator().HasTable(table) {
			fmt.Printf("Table %s does not exist.\n", table)
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid table name, no table named " + table})
			return
		}

		// Use GORM to execute SELECT * LIMIT 1
		var columns map[string]interface{}
		err := db.Raw("SELECT * FROM " + table + " LIMIT 1").Scan(&columns).Error
		if err != nil {
			fmt.Println("Error fetching data:", err)
			return
		}

		// Bind the incoming form data to a map to check keys dynamically
		var jsonBody map[string]string
		if err := c.Bind(&jsonBody); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid form data " + err.Error()})
			return
		}
		fmt.Println(jsonBody)

		hasUpdatedAt := false
		// Prepare a map for the column names and their nullability status
		// columnMap := make(map[string]bool)
		for column := range columns {
			if column == "updated_at" {
				hasUpdatedAt = true
			}
		}

		// Create the struct dynamically based on the table (if possible)
		// NOTE: This part can be simplified if you have a predefined model for the table
		data_map := make(map[string]interface{})
		for key, values := range jsonBody {
			// Assuming each field has only one value, pick the first one
			if len(values) > 0 {
				data_map[key] = values // Add the first value to the map
			}
		}
		if hasUpdatedAt {
			data_map["updated_at"] = time.Now()
		}
		fmt.Println("")
		fmt.Println("")
		fmt.Println("data_map")
		fmt.Println(data_map)

		// Perform the update
		result := db.Table(table).Where("id = ?", data_map["id"]).Updates(data_map)

		// Check for errors
		if result.Error != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update data : " + result.Error.Error()})
			return
		}

		// Check rows affected
		if result.RowsAffected == 0 {
			c.JSON(http.StatusNotFound, gin.H{"error": "No rows were updated"})
			return
		}

		c.JSON(http.StatusOK, gin.H{"message": "Data updated successfully"})

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

		db.Create(&model.LogActivity{
			AdminID:   uint(claims["id"].(float64)),
			FullName:  claims["fullname"].(string),
			Email:     claims["email"].(string),
			Action:    "CREATE",
			Status:    "Success",
			Log:       fmt.Sprintf("CREATE New Data @ Table: %s;", table),
			IP:        c.ClientIP(),
			UserAgent: c.Request.UserAgent(),
			ReqMethod: c.Request.Method,
			ReqUri:    c.Request.RequestURI,
		})
	}
}
func UpdatePatchTeknisi(db *gorm.DB) gin.HandlerFunc {
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
			ID    string `json:"id"`
			Field string `json:"field"`
			Value string `json:"value"`
		}
		if err := c.ShouldBindJSON(&data); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}

		if data.Field == "id" || data.Field == "created_at" || data.Field == "updated_at" || data.Field == "created_by" {
			c.JSON(http.StatusForbidden, gin.H{"error": "Forbidden Field"})
			return
		}
		var manufacture model.Teknisi
		if err := db.Where("id = ?", data.ID).First(&manufacture).Error; err != nil {
			c.JSON(http.StatusNotFound, gin.H{"error": "Record not found"})
			return
		}

		// Update the field with the new value
		if err := db.Model(&manufacture).Update(data.Field, data.Value).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update record"})
			return
		}

		c.JSON(http.StatusOK, gin.H{"msg": "The data was updated successfully!"})

		db.Create(&model.LogActivity{
			AdminID:   uint(claims["id"].(float64)),
			FullName:  claims["fullname"].(string),
			Action:    "PATCH UPDATE",
			Status:    "Success",
			Log:       fmt.Sprintf("UPDATE Manufacture Data By ID: %s; Field : %s; Value: %s; ", data.ID, data.Field, data.Value),
			IP:        c.ClientIP(),
			UserAgent: c.Request.UserAgent(),
			ReqMethod: c.Request.Method,
			ReqUri:    c.Request.RequestURI,
		})
	}
}
