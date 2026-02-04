package controllers

import (
	"bytes"
	"encoding/json"
	"fmt"
	"mime/multipart"
	"net/http"
	"path/filepath"
	"reflect"
	"service-platform/cmd/web_panel/fun"
	"service-platform/cmd/web_panel/model"
	"service-platform/internal/config"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/sirupsen/logrus"
	"github.com/xuri/excelize/v2"
	"gorm.io/gorm"
)

func TableWhatsappBotMessageReply() gin.HandlerFunc {
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

		t := reflect.TypeOf(model.WAMessageReply{})

		// Initialize the map
		columnMap := make(map[int]string)

		// Loop through the fields of the struct
		colNum := 0
		for i := 0; i < t.NumField(); i++ {
			field := t.Field(i)
			// Get the JSON key
			jsonKey := field.Tag.Get("json")
			// fmt.Println("jsonKey: ", jsonKey)
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
		filteredQuery := dbWeb.Model(&model.WAMessageReply{})

		// Apply filters
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
		dbWeb.Model(&model.WAMessageReply{}).Count(&totalRecords)

		// Count the number of filtered records
		var filteredRecords int64
		filteredQuery.Count(&filteredRecords)

		// Apply sorting and pagination to the filtered query
		query := filteredQuery.Order(orderString)
		var Dbdata []model.WAMessageReply
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

			// Extract langCode before the loop
			langCode := ""
			for i := 0; i < t.NumField(); i++ {
				field := t.Field(i)
				theKey := field.Tag.Get("json")
				if theKey == "" {
					theKey = field.Tag.Get("form")
				}
				if theKey == "language_id" {
					fieldValue := v.Field(i)
					langCode = fmt.Sprintf("%d", fieldValue.Uint())
					break
				}
			}

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

				// Handle time.Time fields differently
				if fieldValue.Type() == reflect.TypeOf(time.Time{}) {
					t := fieldValue.Interface().(time.Time)

					switch theKey {
					case "birthdate":
						newData[theKey] = t.Format(fun.T_YYYYMMDD)

					case "date":
						newData[theKey] = t.Add(7 * time.Hour).Format(fun.T_YYYYMMDD_HHmmss)

					default:
						newData[theKey] = t.Format(fun.T_YYYYMMDD_HHmmss)
					}
				} else if theKey == "language" {
					if langCode != "" {
						var languageData model.Language
						dbWeb.Where("id = ?", langCode).First(&languageData)
						newData[theKey] = fun.GetFlag(languageData.Code) + " " + languageData.Name
					} else {
						newData[theKey] = ""
					}
				} else if theKey == "keywords" {
					// Pretty print keywords as a list, truncate if more than 5, show all in tooltip
					separator := config.WebPanel.Get().Whatsmeow.KeywordSeparator
					keywordStr := fieldValue.String()
					if keywordStr != "" {
						keywords := strings.Split(keywordStr, separator)
						for i := range keywords {
							keywords[i] = strings.TrimSpace(keywords[i])
						}
						displayKeywords := keywords
						truncated := false
						if len(keywords) > 2 {
							displayKeywords = keywords[:2]
							truncated = true
						}
						// Prepare tooltip HTML if truncated
						if truncated {
							// Join all keywords for tooltip
							allKeywords := strings.Join(keywords, ", ")
							// You can use data-bs-toggle="tooltip" for Bootstrap 5
							tooltip := fmt.Sprintf(`<span data-bs-toggle="tooltip" title="%s">%s, ...</span>`, allKeywords, strings.Join(displayKeywords, ", "))
							newData[theKey] = tooltip
						} else {
							newData[theKey] = displayKeywords
						}
					} else {
						newData[theKey] = []string{}
					}
				} else if theKey == "for_user_type" {
					t := fieldValue.Interface().(model.WAUserType)

					// Map user types to icons and labels
					userTypeInfo := map[model.WAUserType]struct {
						Icon  string
						Label string
					}{
						model.CommonUser:       {"fad fa-user-alt", "Common User"},
						model.ODOOMSTechnician: {"fad fa-user-hard-hat", "Odoo MS Technician"},
						model.OdooManager:      {"fad fa-user-tie", "Odoo Manager"},
						model.SupportStaff:     {"fad fa-user-shiled", "Support Staff"},
						model.Administrator:    {"fad fa-user-cog", "Administrator"},
						model.ServiceAccount:   {"fad fa-user-secret", "Service Account"},
						model.WaBotSuperUser:   {"fad fa-user-astronaut", "WA Bot Super User"},
					}

					info, ok := userTypeInfo[t]
					if ok {
						htmlRendered := fmt.Sprintf(`<i class="%s me-1"></i> %s`, info.Icon, info.Label)
						newData[theKey] = htmlRendered
					} else {
						// fallback
						newData[theKey] = string(t)
					}
				} else if theKey == "user_of" {
					t := fieldValue.Interface().(model.WAUserOf)
					var htmlRendered string
					switch t {
					case model.UserOfCSNA:
						htmlRendered = config.WebPanel.Get().Default.PT
					case model.UserOfHommyPay:
						htmlRendered = "Hommy Pay"
					}
					newData[theKey] = htmlRendered
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

func PutDataWhatsappBotMessageReply() gin.HandlerFunc {
	return func(c *gin.Context) {

		table := config.WebPanel.Get().Database.TbWAMsgReply
		// Check if the table exists
		if !dbWeb.Migrator().HasTable(table) {
			fmt.Printf("Table %s does not exist.\n", table)
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid table name, no table named " + table})
			return
		}

		// Use GORM to execute SELECT * LIMIT 1
		var columns map[string]interface{}
		err := dbWeb.Raw("SELECT * FROM " + table + " LIMIT 1").Scan(&columns).Error
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
		// fmt.Println(jsonBody)

		// hasUpdatedAt := false
		// // Prepare a map for the column names and their nullability status
		// // columnMap := make(map[string]bool)
		// for column := range columns {
		// 	if column == "updated_at" {
		// 		hasUpdatedAt = true
		// 	}
		// }

		// Create the struct dynamically based on the table (if possible)
		// NOTE: This part can be simplified if you have a predefined model for the table
		data_map := make(map[string]interface{})
		for key, values := range jsonBody {
			// Assuming each field has only one value, pick the first one
			if len(values) > 0 {
				data_map[key] = values // Add the first value to the map
			}
		}
		// if hasUpdatedAt {
		// 	data_map["updated_at"] = time.Now()
		// }
		// fmt.Println("")
		// fmt.Println("")
		// fmt.Println("data_map")
		// fmt.Println(data_map)

		// Perform the update
		result := dbWeb.Table(table).Where("id = ?", data_map["id"]).Updates(data_map)

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

		dbWeb.Create(&model.LogActivity{
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

func DeleteDataWhatsappBotMessageReply() gin.HandlerFunc {
	return func(c *gin.Context) {
		// Get the ID from the URL parameter and convert to integer
		idParam := c.Param("id")
		id, err := strconv.Atoi(idParam)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid Data"})
			return
		}

		// Find the record by ID
		var dbData model.WAMessageReply
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

		// Perform the deletion
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

func PostNewWhatsappBotMessageReply() gin.HandlerFunc {
	return func(c *gin.Context) {
		// Parse JWT token from cookie
		cookies := c.Request.Cookies()
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
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized, details: " + err.Error()})
			return
		}
		var claims map[string]interface{}
		err = json.Unmarshal(decrypted, &claims)
		if err != nil {
			fmt.Printf("Error converting JSON to map: %v", err)
			fun.ClearCookiesAndRedirect(c, cookies)
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized, details: " + err.Error()})
			return
		}

		table := config.WebPanel.Get().Database.TbWAMsgReply
		// Check if the table exists
		if !dbWeb.Migrator().HasTable(table) {
			fmt.Printf("Table %s does not exist.\n", table)
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid table name, no table named " + table})
			return
		}

		// Use GORM to execute SELECT * LIMIT 1
		var columns map[string]interface{}
		err = dbWeb.Raw("SELECT * FROM " + table + " LIMIT 1").Scan(&columns).Error
		if err != nil {
			fmt.Println("Error fetching data:", err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch table columns: " + err.Error()})
		}

		// Bind the incoming form data to a map to check keys dynamically
		var formData map[string][]string
		if err := c.Bind(&formData); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid form data " + err.Error()})
			return
		}

		// hasCreatedAt := false
		// hasUpdatedAt := false
		// // Prepare a map for the column names and their nullability status
		// // columnMap := make(map[string]bool)
		// for column := range columns {
		// 	if column == "created_at" {
		// 		hasCreatedAt = true
		// 	}
		// 	if column == "updated_at" {
		// 		hasUpdatedAt = true
		// 	}
		// }

		// Create the struct dynamically based on the table (if possible)
		// NOTE: This part can be simplified if you have a predefined model for the table
		pg_param_db_model := make(map[string]interface{})
		for key, values := range c.Request.Form {
			// Assuming each field has only one value, pick the first one
			// fmt.Println("key: ", key)
			// fmt.Println("values: ", values)

			if len(values) > 0 {
				switch key {
				case "language":
					langID, err := strconv.ParseUint(values[0], 10, 64)
					if err == nil {
						pg_param_db_model["language_id"] = uint(langID)
					} else {
						pg_param_db_model["language_id"] = 0
					}
				default:
					pg_param_db_model[key] = values[0] // Add the first value to the map
				}
			}
		}

		// if hasCreatedAt {
		// 	pg_param_db_model["created_at"] = now
		// }
		// if hasUpdatedAt {
		// 	pg_param_db_model["updated_at"] = now
		// }

		// fmt.Println(pg_param_db_model)
		// Insert data into the table using the model struct to get the inserted ID
		var dbData model.WAMessageReply
		// Map pg_param_db_model to the struct
		jsonBody, _ := json.Marshal(pg_param_db_model)
		if err := json.Unmarshal(jsonBody, &dbData); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to map data to struct: " + err.Error()})
			return
		}
		if err := dbWeb.Table(table).Create(&dbData).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to insert data"})
			return
		}

		c.JSON(http.StatusOK, gin.H{"message": "Data inserted successfully", "id": dbData.ID})

		dbWeb.Create(&model.LogActivity{
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

func LastUpdateTableWhatsappBotMessageReply() gin.HandlerFunc {
	return func(c *gin.Context) {
		// Get the latest update timestamp from the database
		var lastUpdatedData time.Time
		if err := dbWeb.Model(&model.WAMessageReply{}).
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

func GetBatchTemplateWhatsappBotMessageReply[T any]() gin.HandlerFunc {
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
		// structName := t.Name()
		structName := "Whatsapp Bot Message Reply"

		titleTextStyle, _ := f.NewStyle(&excelize.Style{
			Font: &excelize.Font{
				Bold:   true,
				Color:  "#FFFFFF",
				Family: "Arial",
				Size:   9,
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
		})

		textFormat := "@"
		textStyle, _ := f.NewStyle(&excelize.Style{
			CustomNumFmt: &textFormat,
			Font: &excelize.Font{
				Bold: false,
			},
		})
		headTextStyle, _ := f.NewStyle(&excelize.Style{
			CustomNumFmt: &textFormat,
			Font: &excelize.Font{
				Bold: true,
			},
		})
		const maxRows = 1000

		// var tableHeaders []string
		colIndex := 0
		for i := 0; i < t.NumField(); i++ {
			field := t.Field(i)
			jsonKey := field.Tag.Get("json")
			disallowedKeys := []string{
				"", "-", "created_at", "updated_at", "deleted_at",
			}

			if isDisallowedKey(jsonKey, disallowedKeys) || i == 0 {
				continue
			}

			// Compute the correct column letter based on valid column index
			col := fun.NumberToAlphabet(colIndex)
			colIndex++ // only increment when we keep the field

			// Set fill info for time fields
			fillInfo := ""
			if field.Type == reflect.TypeOf(time.Time{}) {
				timeFormat := field.Tag.Get("time_format")
				if timeFormat != "" {
					humanReadableFormat := strings.ReplaceAll(timeFormat, "2006", "YYYY")
					humanReadableFormat = strings.ReplaceAll(humanReadableFormat, "01", "MM")
					humanReadableFormat = strings.ReplaceAll(humanReadableFormat, "02", "DD")
					humanReadableFormat = strings.ReplaceAll(humanReadableFormat, "15", "HH")
					humanReadableFormat = strings.ReplaceAll(humanReadableFormat, "04", "mm")
					humanReadableFormat = strings.ReplaceAll(humanReadableFormat, "05", "ss")
					fillInfo = "(" + humanReadableFormat + ")"
				} else {
					fillInfo = "(YYYY-MM-DD)(YYYY-MM-DD HH:mm)"
				}
			}

			startCell := fmt.Sprintf("%s2", col)
			endCell := fmt.Sprintf("%s%d", col, maxRows)

			f.SetCellValue(sheetName, startCell, fun.AddSpaceBeforeUppercase(field.Name)+" "+fillInfo)
			f.SetCellStyle(sheetName, startCell, endCell, headTextStyle)
		}

		err := f.RemoveCol(sheetName, "A")
		if err != nil {
			c.String(http.StatusInternalServerError, "Failed to create Excel file: %v", err)
			return
		}

		// Set the title cell value to uppercase
		f.SetCellValue(sheetName, "A1", strings.ToUpper(structName))
		f.SetCellStyle(sheetName, "A1", "A1", titleTextStyle)
		// Notes
		f.SetCellValue(sheetName, "B1", fmt.Sprintf("*separator keywords using: %s", config.WebPanel.Get().Whatsmeow.KeywordSeparator))

		// Dummy template data
		dataSeparator := config.WebPanel.Get().Whatsmeow.KeywordSeparator
		userTypes := model.AllWAUserTypes
		userOfs := model.AllUserOf
		userTypeStrs := make([]string, len(userTypes))
		for i, v := range userTypes {
			userTypeStrs[i] = string(v)
		}
		userTypeList := "'" + strings.Join(userTypeStrs, fmt.Sprintf("'%s'", ",")) + "'"

		userOfStrs := make([]string, len(userOfs))
		for i, v := range userOfs {
			userOfStrs[i] = string(v)
		}
		allowedUserOfList := "'" + strings.Join(userOfStrs, fmt.Sprintf("'%s'", ",")) + "'"

		// Example dummy data as rows
		dummyRows := [][]string{
			{
				"Contoh: Bahasa Indonesia atau English",
				fmt.Sprintf("Contoh: Sisa saldo %s cek saldo %s saldo saya", dataSeparator, dataSeparator),
				"Contoh: Sisa saldo Anda adalah Rp. 2.025,-",
				fmt.Sprintf("e.g. : %s (Select 1 from [%s])", model.CommonUser, userTypeList),
				fmt.Sprintf("e.g. : %s (Select 1 from [%s])", model.UserOfHommyPay, allowedUserOfList),
			},
			{
				"Bahasa Indonesia",
				fmt.Sprintf("tolong dibantu %s tolong %s mohon dibantu %s saya ingin tanya %s saya ingin menanyakan",
					dataSeparator, dataSeparator, dataSeparator, dataSeparator),
				fmt.Sprintf("Halo! Untuk bantuan lebih lanjut, silakan hubungi Technical Support kami di +%s. Kami siap membantu Anda! 😊", config.WebPanel.Get().Whatsmeow.WaTechnicalSupport),
				string(model.CommonUser),
				string(model.UserOfCSNA),
			},
			{
				"English",
				fmt.Sprintf("please help %s help me %s need assistance %s I want to ask %s I’d like to inquire",
					dataSeparator, dataSeparator, dataSeparator, dataSeparator),
				fmt.Sprintf("Hello! For further assistance, please contact our Technical Support at +%s. We’ll be happy to help! 😊", config.WebPanel.Get().Whatsmeow.WaTechnicalSupport),
				string(model.CommonUser),
				string(model.UserOfCSNA),
			},
		}

		startRow := 3 // start from row 3

		for i, row := range dummyRows {
			rowIndex := startRow + i
			for j, value := range row {
				// Convert column index to letter (e.g., 0 -> A, 1 -> B, ...)
				colLetter, _ := excelize.ColumnNumberToName(j + 1)
				cell := fmt.Sprintf("%s%d", colLetter, rowIndex)

				// Set value & style
				f.SetCellValue(sheetName, cell, value)
				f.SetCellStyle(sheetName, cell, cell, textStyle)
			}
		}

		columns := []string{"A", "B", "C", "D", "E"}
		for _, col := range columns {
			err := f.SetColWidth(sheetName, col, col, 60)
			if err != nil {
				c.String(http.StatusInternalServerError, "Failed to create Excel file: %v", err)
				return
			}
		}

		// Write the file content to an in-memory buffer
		var buffer bytes.Buffer
		if err := f.Write(&buffer); err != nil {
			c.String(http.StatusInternalServerError, "Failed to create Excel file: %v", err)
			return
		}

		// Set the necessary headers for file download
		fileName := fmt.Sprintf("batch_upload_%s.xlsx", fun.ToSnakeCase(structName))
		fileName = strings.ReplaceAll(fileName, " ", "")
		c.Header("Content-Type", "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet")
		c.Header("Content-Disposition", fmt.Sprintf("attachment; filename=%s", fileName))

		// Stream the Excel file to the response
		_, err = c.Writer.Write(buffer.Bytes())
		if err != nil {
			c.String(http.StatusInternalServerError, "Failed to write Excel file to response: %v", err)
		}
	}
}

func PostBatchUploadDataWhatsappBotMessageReply[T any]() gin.HandlerFunc {
	return func(c *gin.Context) {
		// Get the file from the form-data
		file, err := c.FormFile("file")
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Failed to get file: " + err.Error()})
			return
		}

		// Ensure the file is an Excel file by checking the extension
		if filepath.Ext(file.Filename) != ".xlsx" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "File must be an .xlsx file"})
			return
		}

		// Parse the file and extract data dynamically
		msg, err := parseAndProcessExcelDataWhatsappBotMessageReply[T](file)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to parse file: " + err.Error()})
			return
		}

		c.JSON(http.StatusOK, msg)
	}
}

// Generic function to parse and process Excel data for WhatsappBotMessageReply
func parseAndProcessExcelDataWhatsappBotMessageReply[T any](file *multipart.FileHeader) (map[string]interface{}, error) {
	// Open the uploaded file
	f, err := file.Open()
	if err != nil {
		return nil, err
	}
	defer f.Close()

	// Read Excel
	xlsx, err := excelize.OpenReader(f)
	if err != nil {
		return nil, err
	}

	// Read rows
	rows, err := xlsx.GetRows("Sheet1")
	if err != nil {
		return nil, err
	}

	// Get model type & table name
	var tableModel T
	modelType := reflect.TypeOf(tableModel)
	tableName := getTableName(tableModel)

	var warning []string
	uniqueCheck := make(map[string][]string)
	var insertRecords []map[string]interface{}

	for i, row := range rows {
		if i < 2 {
			continue // Skip header
		}
		skipThis := false
		record := make(map[string]interface{})
		headerRow := rows[1]

		// Build fieldColumnMap: json key → Excel column index
		fieldColumnMap := make(map[string]int)
		for j := 0; j < modelType.NumField(); j++ {
			field := modelType.Field(j)
			jsonKey := field.Tag.Get("json")
			if jsonKey == "" || jsonKey == "-" || jsonKey == "id" {
				continue
			}
			for idx, cell := range headerRow {
				if normalize(cell) == normalize(jsonKey) {
					fieldColumnMap[jsonKey] = idx
					break
				}
			}
		}

		// === Added: capture and validate user_of & for_user_type ===
		var userOfName string
		var userOfColIdx int = -1
		var userTypeName string
		var userTypeColIdx int = -1

		for jsonKey, idx := range fieldColumnMap {
			if idx < len(row) {
				cell := strings.TrimSpace(row[idx])
				if jsonKey == "user_of" {
					userOfColIdx = idx
					userOfName = cell
				}
				if jsonKey == "for_user_type" {
					userTypeColIdx = idx
					userTypeName = cell
				}
			}
		}

		// Validate user_of
		if userOfColIdx != -1 && userOfName != "" {
			valid := false
			for _, allowed := range model.AllUserOf {
				if string(allowed) == userOfName {
					valid = true
					break
				}
			}
			if !valid {
				skipThis = true
				warning = append(warning, fmt.Sprintf("Invalid user_of: %s", userOfName))
			}
		}

		// Validate for_user_type
		if userTypeColIdx != -1 && userTypeName != "" {
			valid := false
			for _, allowed := range model.AllWAUserTypes {
				if string(allowed) == userTypeName {
					valid = true
					break
				}
			}
			if !valid {
				skipThis = true
				warning = append(warning, fmt.Sprintf("Invalid for_user_type: %s", userTypeName))
			}
		}
		// === End added ===

		// Process language column
		var languageName string
		var languageColIdx int = -1
		for jsonKey, idx := range fieldColumnMap {
			if jsonKey == "language" && idx < len(row) {
				languageColIdx = idx
				languageName = strings.TrimSpace(row[idx])
				break
			}
		}
		var languageID uint
		if languageColIdx != -1 && languageName != "" {
			var lang model.Language
			if err := dbWeb.Where("LOWER(name) = ?", strings.ToLower(languageName)).First(&lang).Error; err == nil {
				languageID = lang.ID
			} else {
				skipThis = true
				warning = append(warning, fmt.Sprintf("Language not found: %s", languageName))
			}
		}

		// Loop fields to build record
		for j := 0; j < modelType.NumField(); j++ {
			field := modelType.Field(j)
			jsonKey := field.Tag.Get("json")
			timeFormat := field.Tag.Get("time_format")
			gormTag := field.Tag.Get("gorm")

			if jsonKey == "" || jsonKey == "-" || jsonKey == "id" {
				continue
			}

			// Special handling
			if jsonKey == "language_id" && languageID != 0 {
				record["language_id"] = languageID
				continue
			}
			if jsonKey == "language" {
				continue
			}
			// === Added: special handling to set user_of & for_user_type ===
			if jsonKey == "user_of" && userOfName != "" {
				record["user_of"] = userOfName
				continue
			}
			if jsonKey == "for_user_type" && userTypeName != "" {
				record["for_user_type"] = userTypeName
				continue
			}
			// === End added ===

			// Normal columns
			colIdx, ok := fieldColumnMap[jsonKey]
			if !ok || colIdx >= len(row) {
				continue
			}
			cell := strings.TrimSpace(row[colIdx])

			// Time format validation
			if timeFormat != "" && cell != "" {
				if _, err := time.Parse(timeFormat, cell); err != nil {
					skipThis = true
					warning = append(warning, fmt.Sprintf("Invalid DateTime Format (Expected: %s) → Field: %s, Value: %s", timeFormat, jsonKey, cell))
					continue
				}
			}

			// Not null validation
			if strings.Contains(gormTag, "not null") && cell == "" {
				skipThis = true
				warning = append(warning, fmt.Sprintf("Field %s is Empty, Must Not Be Null", jsonKey))
				continue
			}

			// Unique check within Excel
			if strings.Contains(gormTag, "unique") {
				if fun.StringContains(uniqueCheck[jsonKey], cell) {
					skipThis = true
					warning = append(warning, fmt.Sprintf("Duplicate in Excel → Field: %s, Value: %s", jsonKey, cell))
				}
				uniqueCheck[jsonKey] = append(uniqueCheck[jsonKey], cell)
			}

			// Duplicate reply_text in Excel (case-insensitive)
			if jsonKey == "reply_text" && cell != "" {
				duplicate := false
				lowerCell := strings.ToLower(cell)
				for _, rec := range insertRecords {
					if val, ok := rec["reply_text"]; ok && strings.ToLower(fmt.Sprintf("%v", val)) == lowerCell {
						duplicate = true
						break
					}
				}
				if duplicate {
					skipThis = true
					warning = append(warning, fmt.Sprintf("Duplicate reply_text in Excel (case-insensitive): %s", cell))
					continue
				}
			}

			// Add to record
			if cell != "" {
				record[jsonKey] = cell
			}
		}

		if !skipThis {
			insertRecords = append(insertRecords, record)
		}
	}

	// Check unique constraints in DB
	for u_key, u_value := range uniqueCheck {
		var results []string
		if err := dbWeb.Select(u_key).Table(tableName).Where(u_key+" IN ?", u_value).Scan(&results).Error; err != nil {
			logrus.Errorf("Error querying database: %v", err)
		}
		if len(results) > 0 {
			for i := 0; i < len(insertRecords); {
				found := false
				for key, value := range insertRecords[i] {
					if key == u_key && fun.StringContains(results, value.(string)) {
						found = true
						warning = append(warning, "Duplicate "+key+" : "+value.(string))
						break
					}
				}
				if found {
					insertRecords = append(insertRecords[:i], insertRecords[i+1:]...)
				} else {
					i++
				}
			}
		}
	}

	// created_at & updated_at
	now := time.Now()
	if dbWeb.Migrator().HasColumn(tableName, "created_at") {
		for i := range insertRecords {
			insertRecords[i]["created_at"] = now
		}
	}
	if dbWeb.Migrator().HasColumn(tableName, "updated_at") {
		for i := range insertRecords {
			insertRecords[i]["updated_at"] = now
		}
	}

	// Remove reply_text already in DB
	if len(insertRecords) > 0 {
		var replyTexts []string
		for _, rec := range insertRecords {
			if val, ok := rec["reply_text"]; ok && val != "" {
				replyTexts = append(replyTexts, strings.ToLower(fmt.Sprintf("%v", val)))
			}
		}
		if len(replyTexts) > 0 {
			var existing []string
			if err := dbWeb.Table(tableName).Where("LOWER(reply_text) IN ?", replyTexts).Pluck("reply_text", &existing).Error; err == nil && len(existing) > 0 {
				existingMap := make(map[string]struct{})
				for _, e := range existing {
					existingMap[strings.ToLower(e)] = struct{}{}
				}
				for i := 0; i < len(insertRecords); {
					val, ok := insertRecords[i]["reply_text"]
					if ok && val != "" {
						if _, found := existingMap[strings.ToLower(fmt.Sprintf("%v", val))]; found {
							warning = append(warning, fmt.Sprintf("reply_text already exists in DB: %s", val))
							insertRecords = append(insertRecords[:i], insertRecords[i+1:]...)
							continue
						}
					}
					i++
				}
			}
		}
	}

	// Insert to DB
	return map[string]interface{}{"warning": warning}, dbWeb.Transaction(func(tx *gorm.DB) error {
		if len(insertRecords) > 0 {
			if err := tx.Table(tableName).Create(&insertRecords).Error; err != nil {
				return fmt.Errorf("failed to batch insert records: %w", err)
			}
		}
		return nil
	})
}

func normalize(s string) string {
	return strings.ToLower(strings.ReplaceAll(strings.TrimSpace(s), " ", "_"))
}
