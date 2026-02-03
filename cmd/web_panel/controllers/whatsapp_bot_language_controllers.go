package controllers

import (
	"encoding/json"
	"fmt"
	"net/http"
	"reflect"
	"service-platform/cmd/web_panel/config"
	"service-platform/cmd/web_panel/fun"
	"service-platform/cmd/web_panel/model"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

func TableWhatsappBotLanguage() gin.HandlerFunc {
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

		t := reflect.TypeOf(model.Language{})

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
		filteredQuery := dbWeb.Model(&model.Language{})

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

				formValue := c.PostForm(formKey)
				// fmt.Print("Form Key: ", formKey)
				// fmt.Print("Form Value: ", formValue)

				if formValue != "" {
					filteredQuery = filteredQuery.Where("`"+formKey+"` LIKE ?", "%"+formValue+"%")
				}
			}
		}

		// Count the total number of records
		var totalRecords int64
		dbWeb.Model(&model.Language{}).Count(&totalRecords)

		// Count the number of filtered records
		var filteredRecords int64
		filteredQuery.Count(&filteredRecords)

		// Apply sorting and pagination to the filtered query
		query := filteredQuery.Order(orderString)
		var Dbdata []model.Language
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
				if theKey == "code" {
					fieldValue := v.Field(i)
					if fieldValue.Kind() == reflect.String {
						langCode = fieldValue.String()
					}
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
				} else if theKey == "flag" {
					if langCode != "" {
						newData[theKey] = fun.GetFlag(langCode)
					} else {
						newData[theKey] = ""
					}
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

func PutDataWhatsappBotLanguage() gin.HandlerFunc {
	return func(c *gin.Context) {

		table := config.GetConfig().Database.TbLanguage
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
		fmt.Println(jsonBody)

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
		fmt.Println("")
		fmt.Println("")
		fmt.Println("data_map")
		fmt.Println(data_map)

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

func DeleteDataWhatsappBotLanguage() gin.HandlerFunc {
	return func(c *gin.Context) {
		// Get the ID from the URL parameter and convert to integer
		idParam := c.Param("id")
		id, err := strconv.Atoi(idParam)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid Data"})
			return
		}

		// Find the record by ID
		var dbData model.Language
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

func PostNewWhatsappBotLanguage() gin.HandlerFunc {
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

		table := config.GetConfig().Database.TbLanguage
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
		fmt.Println(formData)

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
			if len(values) > 0 {
				pg_param_db_model[key] = values[0] // Add the first value to the map
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
		var dbData model.Language
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

func LastUpdateTableWhatsappBotLanguage() gin.HandlerFunc {
	return func(c *gin.Context) {
		// Get the latest update timestamp from the database
		var lastUpdatedData time.Time
		if err := dbWeb.Model(&model.Language{}).
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
