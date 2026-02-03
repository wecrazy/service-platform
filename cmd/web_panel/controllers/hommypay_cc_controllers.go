package controllers

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"mime/multipart"
	"net/http"
	"path/filepath"
	"reflect"
	"runtime/debug"
	"service-platform/cmd/web_panel/config"
	"service-platform/cmd/web_panel/fun"
	"service-platform/cmd/web_panel/model"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/sirupsen/logrus"
	"github.com/xuri/excelize/v2"
	"go.mau.fi/whatsmeow/proto/waE2E"
	"go.mau.fi/whatsmeow/types"
	"gorm.io/gorm"
)

type InsertedDataTriggerItem struct {
	Database *gorm.DB
	IDinDB   uint
}

var TriggerInsertDatatoODOO = make(chan InsertedDataTriggerItem)
var (
	getDataTicketTypeMutex  sync.Mutex
	getDataTicketStageMutex sync.Mutex
	getDataMerchantMutex    sync.Mutex
	getListTicketMutex      sync.Mutex
)

type OdooHommyPayCCTicketTypeItem struct {
	ID   uint   `json:"id"`
	Name string `json:"name"`
}

type OdooHommyPayCCTicketStageItem struct {
	ID   uint   `json:"id"`
	Name string `json:"name"`
}

type OdooHommyPayCCMerchantItem struct {
	ID      uint           `json:"id"`
	Name    string         `json:"name"`
	Phone   nullAbleString `json:"phone"`
	Address nullAbleString `json:"contact_address"`
	City    nullAbleString `json:"city"`
	Email   nullAbleString `json:"email"`
	// ADD Owner
	Sn        nullAbleString `json:"x_serial_number"`
	Product   nullAbleString `json:"x_product"`
	Longitude nullAbleFloat  `json:"partner_longitude"`
	Latitude  nullAbleFloat  `json:"partner_latitude"`
}

type OdooHommyPayCCTicketItem struct {
	ID            uint              `json:"id"`
	Subject       nullAbleString    `json:"name"`
	Priority      nullAbleString    `json:"priority"`
	CustomerPhone nullAbleString    `json:"partner_phone"`
	CustomerEmail nullAbleString    `json:"partner_email"`
	Description   nullAbleString    `json:"description"`
	Sn            nullAbleString    `json:"x_serial_number"`
	Product       nullAbleString    `json:"x_product"`
	TicketTypeId  nullAbleInterface `json:"ticket_type_id"`
	StageId       nullAbleInterface `json:"stage_id"`
	TechnicianId  nullAbleInterface `json:"technician_id"`
	CustomerId    nullAbleInterface `json:"partner_id"`
	SlaDeadline   nullAbleTime      `json:"sla_deadline"`
	// SnId          nullAbleInterface `json:"x_serial_number"` // FIX this soon. Coz now its using char in odoo
	// ProductId nullAbleInterface `json:"x_product"`
}

func (t *OdooHommyPayCCTicketItem) UnmarshalJSON(data []byte) error {
	type Alias OdooHommyPayCCTicketItem // Create an alias to avoid recursion
	aux := &struct {
		SlaDeadline interface{} `json:"sla_deadline"`
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

	if t.SlaDeadline, err = parseTimeField(aux.SlaDeadline); err != nil {
		return fmt.Errorf("SlaDeadline: %v", err)
	}

	return nil
}

func RefreshTicketHommyPay(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		result, err := GetListTicketHommyPayCC(db)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}

		c.JSON(http.StatusOK, gin.H{"message": result})
	}
}

func TableTicketHommyPayCC(db *gorm.DB) gin.HandlerFunc {
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

		t := reflect.TypeOf(model.TicketHommyPayCC{})

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
		filteredQuery := db.Model(&model.TicketHommyPayCC{})

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

				if formKey == "" || formKey == "-" || formKey == "location" {
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
		db.Model(&model.TicketHommyPayCC{}).Count(&totalRecords)

		// Count the number of filtered records
		var filteredRecords int64
		filteredQuery.Count(&filteredRecords)

		// Apply sorting and pagination to the filtered query
		query := filteredQuery.Order(orderString)
		var Dbdata []model.TicketHommyPayCC
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
					t := fieldValue.Interface().(time.Time)

					switch theKey {
					case "birthdate":
						newData[theKey] = t.Format(fun.T_YYYYMMDD)

					case "date":
						newData[theKey] = t.Add(7 * time.Hour).Format(fun.T_YYYYMMDD_HHmmss)

					default:
						newData[theKey] = t.Format(fun.T_YYYYMMDD_HHmmss)
					}
				} else if theKey == "stage" {
					newData[theKey] = fun.OdooStageHTML(fieldValue.String())
				} else if theKey == "priority" {
					newData[theKey] = fun.Priority3StarsHTML(fieldValue.String())
				} else if theKey == "sla_deadline" {
					value := "N/A"
					if fieldValue.Type() == reflect.TypeOf((*time.Time)(nil)) {
						if ptr, ok := fieldValue.Interface().(*time.Time); ok && ptr != nil && !ptr.IsZero() {
							value = ptr.Format(fun.T_YYYYMMDD_HHmmss)
						}
					}
					newData[theKey] = value
				} else {
					if fieldValue.Interface() == "" {
						newData[theKey] = "N/A"
					} else {
						newData[theKey] = fieldValue.Interface()
					}
				}
				// else if theKey == "sla_deadline" {
				// 	value := "N/A"
				// 	if fieldValue.Type().String() == "sql.NullTime" {
				// 		nullTime := fieldValue.Interface().(sql.NullTime)
				// 		if nullTime.Valid {
				// 			value = nullTime.Time.Format(fun.T_YYYYMMDD_HHmmss)
				// 		}
				// 	}
				// 	newData[theKey] = value
				// }
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

func PostNewTicketHommyPayCC(db *gorm.DB) gin.HandlerFunc {
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

		table := config.GetConfig().Database.TbTicketHommyPayCC
		// Check if the table exists
		if !db.Migrator().HasTable(table) {
			fmt.Printf("Table %s does not exist.\n", table)
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid table name, no table named " + table})
			return
		}

		// Use GORM to execute SELECT * LIMIT 1
		var columns map[string]interface{}
		err = db.Raw("SELECT * FROM " + table + " LIMIT 1").Scan(&columns).Error
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

		now := time.Now()
		// if hasCreatedAt {
		// 	pg_param_db_model["created_at"] = now
		// }
		// if hasUpdatedAt {
		// 	pg_param_db_model["updated_at"] = now
		// }

		// Default values for fields that are not provided
		defaultParam := map[string]interface{}{
			"stage":          config.GetConfig().Database.DefaultValues.Stage,
			"priority":       config.GetConfig().Database.DefaultValues.Priority,
			"keterangan":     config.GetConfig().Database.DefaultValues.Keterangan,
			"status_in_odoo": config.GetConfig().Database.DefaultValues.StatusInOdoo,
			"admin_id":       claims["id"].(float64),
			"created_at":     now,
			"updated_at":     now,
		}
		// Fill in default values for missing fields
		for key, value := range defaultParam {
			if _, exists := pg_param_db_model[key]; !exists {
				pg_param_db_model[key] = value
			}
		}

		// fmt.Println(pg_param_db_model)
		// Insert data into the table using the model struct to get the inserted ID
		var ticket model.TicketHommyPayCC
		// Map pg_param_db_model to the struct
		jsonBody, _ := json.Marshal(pg_param_db_model)
		if err := json.Unmarshal(jsonBody, &ticket); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to map data to struct: " + err.Error()})
			return
		}
		if err := db.Table(table).Create(&ticket).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to insert data"})
			return
		}

		c.JSON(http.StatusOK, gin.H{"message": "Data inserted successfully", "id": ticket.ID})

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

		// Trigger insert data to Odoo
		TriggerInsertDatatoODOO <- InsertedDataTriggerItem{
			Database: db,
			IDinDB:   ticket.ID,
		}

	}
}

func LastUpdateTicketHommyPayCC(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		// Get the latest update timestamp from the database
		var lastUpdatedData time.Time
		if err := db.Model(&model.TicketHommyPayCC{}).
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

func PutDataTicketHommyPayCC(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {

		table := config.GetConfig().Database.TbTicketHommyPayCC
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

func DeleteDataTicketHommyPayCC(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		// Get the ID from the URL parameter and convert to integer
		idParam := c.Param("id")
		id, err := strconv.Atoi(idParam)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid Data"})
			return
		}

		// Find the record by ID
		var dbData model.TicketHommyPayCC
		if err := db.First(&dbData, id).Error; err != nil {
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
		if err := db.Delete(&dbData).Error; err != nil {
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
		db.Create(&model.LogActivity{
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

func GetBatchTemplateDataTicket[T any](db *gorm.DB) gin.HandlerFunc {
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
		var titleInExcel string
		if structName == "TicketHommyPayCC" {
			titleInExcel = "Batch Upload Data Ticket Hommy Pay CC"
		} else {
			titleInExcel = "Batch Upload"
		}

		titleTextStyle, _ := f.NewStyle(&excelize.Style{
			Font: &excelize.Font{
				Bold:   true,
				Color:  "#FFFFFF",
				Family: "Arial",
				Size:   9,
			},
			Fill: excelize.Fill{
				Type:    "pattern",
				Color:   []string{"#00B050"},
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
				"", "-", "ticket_id", "ticket_type_id", "stage_id", "stage",
				"technician_id", "customer_id", "keterangan", "admin_id", "status_in_odoo",
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

		if structName == "TicketHommyPayCC" {
			err := f.RemoveCol(sheetName, "A")
			if err != nil {
				c.String(http.StatusInternalServerError, "Failed to create Excel file: %v", err)
				return
			}

			// Set the title cell value to uppercase
			f.SetCellValue(sheetName, "A1", strings.ToUpper(titleInExcel))
			f.SetCellStyle(sheetName, "A1", "A1", titleTextStyle)

			// Dummy template data
			dummyTemplateData := []struct {
				Cell  string
				Value string
			}{
				{"A3", "Contoh: Preventive Maintenance, Corrective Maintenance, etc"},
				{"B3", "Contoh: Teknisi ABC"},
				{"C3", "Priority diisi dengan rentang 0 - 3"},
				{"D3", "Nama Customer"},
				{"E3", "Nomor Telepon Customer"},
				{"F3", "Email Customer"},
				{"G3", "Alamat Customer"},
				{"H3", "Contoh: ingin dilakukan pengecekan di lokasi customer hari ini"},
			}
			for _, d := range dummyTemplateData {
				f.SetCellValue(sheetName, d.Cell, d.Value)
				f.SetCellStyle(sheetName, d.Cell, d.Cell, textStyle)
			}

			columns := []string{"A", "B", "C", "D", "E", "F", "G", "H"}
			for _, col := range columns {
				err := f.SetColWidth(sheetName, col, col, 40)
				if err != nil {
					c.String(http.StatusInternalServerError, "Failed to create Excel file: %v", err)
					return
				}
			}
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

func PostBatchUploadDataTicket[T any](db *gorm.DB) gin.HandlerFunc {
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
		msg, err := parseAndProcessExcelDataTicket[T](file, db)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to parse file: " + err.Error()})
			return
		}

		c.JSON(http.StatusOK, msg)
	}
}

// Generic Excel parsing and processing function
func parseAndProcessExcelDataTicket[T any](file *multipart.FileHeader, db *gorm.DB) (map[string]interface{}, error) {
	// Open the uploaded file
	f, err := file.Open()
	if err != nil {
		return nil, err
	}
	defer f.Close()

	// Read the Excel file using excelize
	xlsx, err := excelize.OpenReader(f)
	if err != nil {
		return nil, err
	}

	// Retrieve rows from "Sheet1"
	rows, err := xlsx.GetRows("Sheet1")
	if err != nil {
		return nil, err
	}

	// Dynamically get the struct type using reflection
	var model T
	modelType := reflect.TypeOf(model)
	tableName := getTableName(model)

	// Slices to hold the data for batch operations
	var warning []string
	uniqueCheck := make(map[string][]string)
	var insertRecords []map[string]interface{}
	// Start processing from row 3
	for i, row := range rows {
		skipThis := false
		if i < 2 {
			continue
		}
		record := make(map[string]interface{})
		// rowStep := 0
		// Header-based column mapping (should be done once, before parsing rows)
		headerRow := rows[1]
		fmt.Println("Header Row:", headerRow)
		fieldColumnMap := make(map[string]int)
		for j := 0; j < modelType.NumField(); j++ {
			field := modelType.Field(j)
			jsonKey := field.Tag.Get("json")
			if jsonKey == "" || jsonKey == "-" || jsonKey == "id" {
				continue
			}
			for idx, cell := range headerRow {
				if strings.EqualFold(strings.TrimSpace(cell), jsonKey) {
					fieldColumnMap[jsonKey] = idx
					break
				}
			}
		}

		for j := 0; j < modelType.NumField(); j++ {
			field := modelType.Field(j)
			jsonKey := field.Tag.Get("json")
			timeFormat := field.Tag.Get("time_format")
			gormTag := field.Tag.Get("gorm")

			if jsonKey == "" || jsonKey == "-" || jsonKey == "id" {
				continue
			}

			colIdx, ok := fieldColumnMap[jsonKey]
			if !ok || colIdx >= len(row) {
				continue
			}

			cell := strings.TrimSpace(row[colIdx])

			// Time format validation
			if timeFormat != "" {
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

			// Unique constraint check
			if strings.Contains(gormTag, "unique") {
				if fun.StringContains(uniqueCheck[jsonKey], cell) {
					skipThis = true
					warning = append(warning, fmt.Sprintf("Duplicate in Excel → Field: %s, Value: %s", jsonKey, cell))
				}
				uniqueCheck[jsonKey] = append(uniqueCheck[jsonKey], cell)
			}

			// Set parsed value
			if cell != "" {
				record[jsonKey] = cell
			}

			// Debugging output
			fmt.Println("jsonKey:", jsonKey, "| Column:", colIdx, "| Value:", cell)
		}

		if !skipThis {
			insertRecords = append(insertRecords, record)
		}
	}
	for u_key, u_value := range uniqueCheck {
		var results []string
		if err := db.Select(u_key).Table(tableName).Where(u_key+" IN ?", u_value).Scan(&results).Error; err != nil {
			logrus.Errorf("Error querying database: %v", err)
		}
		if len(results) > 0 {
			// Loop through and remove elements containing the target string
			for i := 0; i < len(insertRecords); {
				found := false
				for key, value := range insertRecords[i] {
					if key == u_key {
						found = fun.StringContains(results, value.(string))
						warning = append(warning, "Duplicate "+key+" : "+value.(string))
						break
					}
				}

				if found {
					// Remove the current element by slicing
					insertRecords = append(insertRecords[:i], insertRecords[i+1:]...)
				} else {
					// Increment index only if no removal occurs
					i++
				}
			}
		}
	}

	// Check if table has created_at and updated_at columns
	hasCreatedAt := db.Migrator().HasColumn(tableName, "created_at")
	hasUpdatedAt := db.Migrator().HasColumn(tableName, "updated_at")

	now := time.Now()
	for i := range insertRecords {
		if hasCreatedAt {
			insertRecords[i]["created_at"] = now
		}
		if hasUpdatedAt {
			insertRecords[i]["updated_at"] = now
		}
	}

	// Perform database operations
	return map[string]interface{}{"warning": warning}, db.Transaction(func(tx *gorm.DB) error {
		if len(insertRecords) > 0 {
			if err := tx.Table(tableName).Create(&insertRecords).Error; err != nil {
				return fmt.Errorf("failed to batch insert records: %w", err)
			}
		}
		return nil
	})
}

func isDisallowedKey(key string, disallowed []string) bool {
	for _, d := range disallowed {
		if key == d {
			return true
		}
	}
	return false
}

func UpdateDatatoODOOFromInsertedTicketHommyPayCC() {
	go func() {
		defer func() {
			if r := recover(); r != nil {
				logrus.Warnf("Recovered from panic in UpdateDatatoODOOFromInsertedTicketHommyPayCC: %v", r)
				debug.Stack()
				// logrus.Warnf("Stack trace: %s", string(debug.Stack()))
			}
		}()

		for data := range TriggerInsertDatatoODOO {
			logrus.Infof("Triggering Odoo Insert new Data for ID: %d", data.IDinDB)
			insertNewTicketInODOO(data)
		}
	}()
}

func insertNewTicketInODOO(data InsertedDataTriggerItem) {
	logrus.Infof("Inserting new Ticket Hommy Pay CC into Odoo for ID: %d", data.IDinDB)
	table := config.GetConfig().Database.TbTicketHommyPayCC

	var modelData model.TicketHommyPayCC
	var keteranganMsg string
	if err := data.Database.Table(table).Where("id = ? AND status_in_odoo = ?", data.IDinDB, config.GetConfig().Database.DefaultValues.StatusInOdoo).First(&modelData).Error; err != nil {
		keteranganMsg = "Failed to find data in DB for Odoo insert: " + err.Error()
		logrus.Error(keteranganMsg)
		_ = data.Database.Table(table).Where("id = ?", data.IDinDB).Update("keterangan", keteranganMsg).Error
		return
	}

	odooModel := "helpdesk.ticket"
	odooParams := map[string]interface{}{
		"company_id": config.GetConfig().Default.CompanyId,
		"active":     true,
	}
	odooParams["model"] = odooModel
	odooParams["name"] = modelData.TicketNumber
	odooParams["priority"] = modelData.Priority

	// REMOVE: only for debug and testing purpose -> use ID 99 as Ipal's Customer and User ID ipal
	odooParams["partner_id"] = int(99)
	odooParams["user_id"] = int(7)
	odooParams["x_project_id"] = int(3)
	odooParams["x_kategori"] = "1. Informasi HommyPay"
	odooParams["x_eskalasi"] = "Ya"
	odooParams["x_dept_divisi"] = "Merchant"
	odooParams["ticket_type_id"] = int(1)

	// if modelData.Sn != "" {
	// 	odooParams["x_serial_number"] = modelData.Sn
	// }

	if modelData.TicketType != "" {
		var ticketType model.TicketType

		// Debug: Log what we're searching for
		logrus.Infof("Searching for TicketType with name containing: '%s'", modelData.TicketType)

		// Try case-insensitive search first
		if err := data.Database.Model(&model.TicketType{}).
			Where("LOWER(name) LIKE LOWER(?)", "%"+modelData.TicketType+"%").
			First(&ticketType).Error; err != nil {

			// If not found, let's see what ticket types are available
			var allTicketTypes []model.TicketType
			if debugErr := data.Database.Model(&model.TicketType{}).Find(&allTicketTypes).Error; debugErr == nil {
				logrus.Infof("Available TicketTypes in database: %+v", allTicketTypes)
			}

			keteranganMsg = "Failed to find Ticket Type in DB for Odoo insert. Searched for: '" + modelData.TicketType + "'. Error: " + err.Error()
			logrus.Error(keteranganMsg)
			_ = data.Database.Table(table).Where("id = ?", data.IDinDB).Update("keterangan", keteranganMsg).Error
			return
		}

		logrus.Infof("Found TicketType: ID=%d, Name='%s'", ticketType.ID, ticketType.Name)
		odooParams["ticket_type_id"] = ticketType.ID
	}

	// FIX soon
	if modelData.CustomerPhone != "" {
		odooParams["partner_phone"] = modelData.CustomerPhone
	}

	// // ADD sla deadline existing
	// var slaDeadlinetimeTime *time.Time

	// if modelData.SlaDeadline != nil && !modelData.SlaDeadline.IsZero() {
	// 	slaDeadlinetimeTime = modelData.SlaDeadline
	// }

	// if slaDeadlinetimeTime == nil || slaDeadlinetimeTime.IsZero() {
	// 	slaDeadlineStr := config.GetConfig().Database.DefaultValues.SLADeadline
	// 	duration, err := fun.ParseFlexibleDuration(slaDeadlineStr)
	// 	if err != nil {
	// 		logrus.Errorf("error while parsing default sla deadline: %v", err)
	// 	} else {
	// 		slaDeadlineTime := time.Now().Add(duration).Add(-7 * time.Hour)
	// 		// odooParams["sla_deadline"] = slaDeadlineTime // FIX this coz this work for insert data but got error response in ODOO
	// 		// REMOVE: try with this datetime format so ODOO API from HOMMY PAY not return response 400
	// 		slaFormattedDate := slaDeadlineTime.Format("2006-01-02 15:04:05")
	// 		odooParams["sla_deadline"] = slaFormattedDate
	// 	}
	// }

	// ADD description
	if modelData.Description != "" {
		odooParams["description"] = modelData.Description
	}

	payload := map[string]interface{}{
		"jsonrpc": config.GetConfig().ApiODOO.JSONRPC,
		"params":  odooParams,
	}

	payloadBytes, err := json.Marshal(payload)
	if err != nil {
		keteranganMsg = "Error marshalling payload: " + err.Error()
		logrus.Error(keteranganMsg)
		_ = data.Database.Table(table).Where("id = ?", data.IDinDB).Update("keterangan", keteranganMsg).Error
		return
	}

	url := config.GetConfig().ApiODOO.UrlCreateDataHommyPay
	method := "POST"

	body, err := FetchODOOHommyPay(url, method, string(payloadBytes))
	if err != nil {
		keteranganMsg = "Error sending request to Odoo: " + err.Error()
		logrus.Error(keteranganMsg)
		_ = data.Database.Table(table).Where("id = ?", data.IDinDB).Update("keterangan", keteranganMsg).Error
		return
	}

	var jsonResponse map[string]interface{}
	err = json.Unmarshal(body, &jsonResponse)
	if err != nil {
		keteranganMsg = "Error decoding Odoo response: " + err.Error()
		logrus.Error(keteranganMsg)
		_ = data.Database.Table(table).Where("id = ?", data.IDinDB).Update("keterangan", keteranganMsg).Error
		return
	}

	// Handle Odoo error
	if errorResponse, ok := jsonResponse["error"].(map[string]interface{}); ok {
		if errorMessage, ok := errorResponse["message"].(string); ok {
			keteranganMsg = "Odoo error: " + errorMessage
			logrus.Error(keteranganMsg)
			_ = data.Database.Table(table).Where("id = ?", data.IDinDB).Update("keterangan", keteranganMsg).Error
			return
		}
	}

	// Handle Odoo success response
	var ticketID int
	if message, ok := jsonResponse["message"].(string); ok && message == "Success" {
		if success, ok := jsonResponse["success"].(bool); ok && success {
			if status, ok := jsonResponse["status"].(float64); ok && status == 200 {
				if id, ok := jsonResponse["response"].(float64); ok {
					logrus.Infof("ODOO Success: ticket created with ID %d", int(id))
					ticketID = int(id)
					// Use Updates to update multiple fields at once
					updateFields := map[string]interface{}{
						"ticket_id":      int(id),
						"keterangan":     "Successfully synced to Odoo",
						"status_in_odoo": "New",
					}
					if err := data.Database.Table(table).Where("id = ?", data.IDinDB).Updates(updateFields).Error; err != nil {
						keteranganMsg = "Failed to update Odoo ID in DB: " + err.Error()
						logrus.Error(keteranganMsg)
						_ = data.Database.Table(table).Where("id = ?", data.IDinDB).Update("keterangan", keteranganMsg).Error
						return
					}
				} else {
					keteranganMsg = "ID not found in Odoo response"
					logrus.Error(keteranganMsg)
					_ = data.Database.Table(table).Where("id = ?", data.IDinDB).Update("keterangan", keteranganMsg).Error
					return
				}
			} else {
				keteranganMsg = "Invalid status in Odoo response"
				logrus.Error(keteranganMsg)
				_ = data.Database.Table(table).Where("id = ?", data.IDinDB).Update("keterangan", keteranganMsg).Error
				return
			}
		} else {
			keteranganMsg = "Success flag not true in Odoo response"
			logrus.Error(keteranganMsg)
			_ = data.Database.Table(table).Where("id = ?", data.IDinDB).Update("keterangan", keteranganMsg).Error
			return
		}
	} else {
		keteranganMsg = "Unexpected response format from Odoo: " + fmt.Sprintf("%v", jsonResponse)
		logrus.Error(keteranganMsg)
		_ = data.Database.Table(table).Where("id = ?", data.IDinDB).Update("keterangan", keteranganMsg).Error
		return
	}

	jid := types.NewJID(strings.ReplaceAll(modelData.CustomerPhone, "+", ""), types.DefaultUserServer)
	// msgToSend := fmt.Sprintf("Tiket dengan ID: %d berhasil dibuat!", ticketID+1)
	_ = ticketID
	msgToSend := "Tiket berhasil dibuat!"

	// Before the SendMessage call
	if WhatsappClient == nil {
		logrus.Error("WhatsApp client is nil")
		return
	}
	if !WhatsappClient.IsConnected() {
		logrus.Error("WhatsApp client is not connected")
		return
	}
	if !WhatsappClient.IsLoggedIn() {
		logrus.Error("WhatsApp client is not logged in")
		return
	}

	_, err = WhatsappClient.SendMessage(context.Background(), jid, &waE2E.Message{
		Conversation: &msgToSend,
	})
	if err != nil {
		logrus.Errorf("Failed to send WhatsApp message to user: %v", err)
	}
}

func GetTicketTypeHommyPayCC(db *gorm.DB) {
	if !getDataTicketTypeMutex.TryLock() {
		logrus.Warn("Another process is already fetching Ticket Type Hommy Pay CC from Odoo, skipping this request.")
		return
	}
	defer getDataTicketTypeMutex.Unlock()

	logrus.Info("Fetching Ticket Type Hommy Pay CC from Odoo...")

	odooModel := "helpdesk.ticket.type"
	odooDomain := []interface{}{
		[]interface{}{"id", "!=", 0},
	}
	odooFields := []string{
		"id",
		"name",
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
		logrus.Errorf("error marshalling payload: %v", err)
		return
	}
	url := config.GetConfig().ApiODOO.UrlGetDataHommyPay
	method := "POST"

	body, err := FetchODOOHommyPay(url, method, string(payloadBytes))
	if err != nil {
		logrus.Error(err)
		return
	}

	var jsonResponse map[string]interface{}
	err = json.Unmarshal(body, &jsonResponse)
	if err != nil {
		logrus.Errorf("error unmarshalling JSON response: %v", err)
		return
	}

	// Check for error response from Odoo
	if errorResponse, ok := jsonResponse["error"].(map[string]interface{}); ok {
		if errorMessage, ok := errorResponse["message"].(string); ok && errorMessage == "Odoo Session Expired" {
			logrus.Errorf("error code: %v, message: %v", errorResponse["code"], errorMessage)
			return
		}
	}

	// Check for the result in JSON response
	if result, ok := jsonResponse["result"].(map[string]interface{}); ok {
		// Log the message and success status if they exist
		if message, ok := result["message"].(string); ok {
			success, successOk := result["success"]
			logrus.Infof("ODOO Result, message: %v, status: %v", message, successOk && success == true)
		}
	}

	// Check for the existence and validity of the "result" field
	result, resultExists := jsonResponse["result"]
	if !resultExists {
		logrus.Errorf("'result' field not found in the response: %v", jsonResponse)
		return
	}

	// Check if the result is an array and ensure it's not empty
	resultArray, ok := result.([]interface{})
	if !ok || len(resultArray) == 0 {
		logrus.Errorf("'result' is not an array or is empty: %v", result)
		return
	}

	if !ok {
		logrus.Errorf("unexpected response format: %v", jsonResponse)
		return
	}

	if len(resultArray) == 0 {
		logrus.Errorf("No data found in the ODOO response")
		return
	}

	for i, item := range resultArray {
		itemMap, ok := item.(map[string]interface{})
		if !ok {
			logrus.Errorf("item %d is not a map: %v", i, item)
			continue
		}

		var odooData OdooHommyPayCCTicketTypeItem
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

		if err := db.First(&model.TicketType{}, "id = ?", odooData.ID).Error; err != nil {
			if err == gorm.ErrRecordNotFound {
				// If the record does not exist, create a new one
				newTicketType := model.TicketType{
					ID:   odooData.ID,
					Name: odooData.Name,
				}
				if err := db.Create(&newTicketType).Error; err != nil {
					logrus.Errorf("Failed to create new Ticket Type Hommy Pay CC: %v", err)
				} else {
					logrus.Infof("Created new Ticket Type Hommy Pay CC with ID: %d", odooData.ID)
				}
			} else {
				logrus.Errorf("Failed to check or create Ticket Type Hommy Pay CC: %v", err)
			}
		} else {
			// If the record exists, update it
			if err := db.Model(&model.TicketType{}).Where("id = ?", odooData.ID).Updates(map[string]interface{}{
				"name": odooData.Name,
			}).Error; err != nil {
				logrus.Errorf("Failed to update Ticket Type Hommy Pay CC with ID %d: %v", odooData.ID, err)
			} else {
				// logrus.Infof("Updated Ticket Type Hommy Pay CC with ID: %d", odooData.ID)
			}
		}
	}

	logrus.Infof("Successfully processed %d Ticket Type Hommy Pay CC records from Odoo", len(resultArray))
}

func GetTicketStageHommyPayCC(db *gorm.DB) {
	if !getDataTicketStageMutex.TryLock() {
		logrus.Warn("Another process is already fetching Ticket Stage Hommy Pay CC from Odoo, skipping this request.")
		return
	}
	defer getDataTicketStageMutex.Unlock()

	logrus.Info("Fetching Ticket Stage Hommy Pay CC from Odoo...")

	odooModel := "helpdesk.stage"
	odooDomain := []interface{}{
		[]interface{}{"active", "=", true},
		[]interface{}{"id", "!=", 0},
	}
	odooFields := []string{
		"id",
		"name",
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
		logrus.Errorf("error marshalling payload: %v", err)
		return
	}
	url := config.GetConfig().ApiODOO.UrlGetDataHommyPay
	method := "POST"

	body, err := FetchODOOHommyPay(url, method, string(payloadBytes))
	if err != nil {
		logrus.Error(err)
		return
	}

	var jsonResponse map[string]interface{}
	err = json.Unmarshal(body, &jsonResponse)
	if err != nil {
		logrus.Errorf("error unmarshalling JSON response: %v", err)
		return
	}

	if errorResponse, ok := jsonResponse["error"].(map[string]interface{}); ok {
		if errorMessage, ok := errorResponse["message"].(string); ok && errorMessage == "Odoo Session Expired" {
			logrus.Errorf("error code: %v, message: %v", errorResponse["code"], errorMessage)
			return
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
		logrus.Errorf("'result' field not found in the response: %v", jsonResponse)
		return
	}
	// Check if the result is an array and ensure it's not empty
	resultArray, ok := result.([]interface{})
	if !ok || len(resultArray) == 0 {
		logrus.Errorf("'result' is not an array or is empty: %v", result)
		return
	}
	if !ok {
		logrus.Errorf("unexpected response format: %v", jsonResponse)
		return
	}
	if len(resultArray) == 0 {
		logrus.Errorf("No data found in the ODOO response")
		return
	}
	for i, item := range resultArray {
		itemMap, ok := item.(map[string]interface{})
		if !ok {
			logrus.Errorf("item %d is not a map: %v", i, item)
			continue
		}

		var odooData OdooHommyPayCCTicketStageItem
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

		if err := db.First(&model.TicketStage{}, "id = ?", odooData.ID).Error; err != nil {
			if err == gorm.ErrRecordNotFound {
				newTicketStage := model.TicketStage{
					ID:   odooData.ID,
					Name: odooData.Name,
				}
				if err := db.Create(&newTicketStage).Error; err != nil {
					logrus.Errorf("Failed to create new Ticket Stage Hommy Pay CC: %v", err)
				} else {
					logrus.Infof("Created new Ticket Stage Hommy Pay CC with ID: %d", odooData.ID)
				}
			} else {
				logrus.Errorf("Failed to check or create Ticket Stage Hommy Pay CC: %v", err)
			}
		} else {
			if err := db.Model(&model.TicketStage{}).Where("id = ?", odooData.ID).Updates(map[string]interface{}{
				"name": odooData.Name,
			}).Error; err != nil {
				logrus.Errorf("Failed to update Ticket Stage Hommy Pay CC with ID %d: %v", odooData.ID, err)
			} else {
				// logrus.Infof("Updated Ticket Stage Hommy Pay CC with ID: %d", odooData.ID)
			}
		}
	}
	logrus.Infof("Successfully processed %d Ticket Stage Hommy Pay CC records from Odoo", len(resultArray))
}

func GetMerchantHommyPayCC(db *gorm.DB) (string, error) {
	if !getDataMerchantMutex.TryLock() {
		return "", errors.New("another process is already fetching Merchant Hommy Pay CC from Odoo, skipping this request")
	}
	defer getDataMerchantMutex.Unlock()

	logrus.Info("Fetching Merchant Hommy Pay CC from Odoo...")

	odooModel := "res.partner"
	odooDomain := []interface{}{
		[]interface{}{"id", "!=", 0},
		[]interface{}{"active", "=", true},
	}
	odooFields := []string{
		"id",
		"name",
		"phone",
		"contact_address",
		"city",
		"email",
		"x_serial_number",
		"x_product",
		"partner_longitude",
		"partner_latitude",
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
	url := config.GetConfig().ApiODOO.UrlGetDataHommyPay
	method := "POST"

	body, err := FetchODOOHommyPay(url, method, string(payloadBytes))
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

		var odooData OdooHommyPayCCMerchantItem
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

		var strLongitude, strLatitude string
		if odooData.Longitude.Float != 0 {
			strLongitude = fmt.Sprintf("%f", odooData.Longitude.Float)
		} else {
			strLongitude = "0.0"
		}
		if odooData.Latitude.Float != 0 {
			strLatitude = fmt.Sprintf("%f", odooData.Latitude.Float)
		} else {
			strLatitude = "0.0"
		}

		if err := db.First(&model.MerchantHommyPayCC{}, "merchant_id = ?", odooData.ID).Error; err != nil {
			if err == gorm.ErrRecordNotFound {
				newMerchant := model.MerchantHommyPayCC{
					ID:              odooData.ID,
					MerchantId:      int(odooData.ID),
					MerchantName:    odooData.Name,
					MerchantPhone:   odooData.Phone.String,
					MerchantAddress: odooData.Address.String,
					MerchantCity:    odooData.City.String,
					MerchantEmail:   odooData.Email.String,
					Sn:              odooData.Sn.String,
					Product:         odooData.Product.String,
					Longitude:       strLongitude,
					Latitude:        strLatitude,
				}
				if err := db.Create(&newMerchant).Error; err != nil {
					logrus.Errorf("Failed to create new Merchant Hommy Pay CC: %v", err)
				} else {
					logrus.Infof("Created new Merchant Hommy Pay CC with ID: %d", odooData.ID)
				}
			} else {
				logrus.Errorf("Failed to check or create Merchant Hommy Pay CC: %v", err)
			}
		} else {
			if err := db.Model(&model.MerchantHommyPayCC{}).Where("merchant_id = ?", odooData.ID).Updates(map[string]interface{}{
				"merchant_name":    odooData.Name,
				"merchant_phone":   odooData.Phone.String,
				"merchant_address": odooData.Address.String,
				"merchant_city":    odooData.City.String,
				"merchant_email":   odooData.Email.String,
				"sn":               odooData.Sn.String,
				"product":          odooData.Product.String,
				"longitude":        strLongitude,
				"latitude":         strLatitude,
			}).Error; err != nil {
				logrus.Errorf("Failed to update Merchant Hommy Pay CC with ID %d: %v", odooData.ID, err)
			} else {
				logrus.Infof("Updated Merchant Hommy Pay CC with ID: %d", odooData.ID)
			}
		}
	}
	return "Merchant Hommy Pay CC data successfully updated @" + time.Now().Format("15:04:05, 02 Jan 2006"), nil
}

func GetListTicketHommyPayCC(db *gorm.DB) (string, error) {
	if !getListTicketMutex.TryLock() {
		return "", errors.New("another process is already fetching Ticket Hommy Pay CC from Odoo, skipping this request")
	}
	defer getListTicketMutex.Unlock()

	logrus.Info("Fetching Ticket Hommy Pay CC from Odoo...")

	odooModel := "helpdesk.ticket"
	odooDomain := []interface{}{
		[]interface{}{"id", "!=", 0},
		[]interface{}{"active", "=", true},
	}

	odooFields := []string{
		"id",
		"name",
		"ticket_type_id",
		"stage_id",
		// ADD "technician_id",
		"priority",
		"partner_id",
		"partner_phone",
		"partner_email",
		"x_serial_number",
		"x_product",
		"description",
		"sla_deadline",
	}

	odooOrder := "id desc"

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
	url := config.GetConfig().ApiODOO.UrlGetDataHommyPay
	method := "POST"

	body, err := FetchODOOHommyPay(url, method, string(payloadBytes))
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

		var odooData OdooHommyPayCCTicketItem
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

		ticketTypeID, ticketType, err := parseJSONIDDataCombined(odooData.TicketTypeId)
		if err != nil {
			logrus.Errorf("error parsing Ticket Type ID for item %d: %v, from data: %v", i, err, odooData.TicketTypeId)
			continue
		}

		stageID, stage, err := parseJSONIDDataCombined(odooData.StageId)
		if err != nil {
			logrus.Errorf("error parsing Stage ID for item %d: %v, from data: %v", i, err, odooData.StageId)
			continue
		}

		technicianID, technician, err := parseJSONIDDataCombined(odooData.TechnicianId)
		if err != nil {
			logrus.Errorf("error parsing Technician ID for item %d: %v, from data: %v", i, err, odooData.TechnicianId)
			continue
		}

		customerID, customer, err := parseJSONIDDataCombined(odooData.CustomerId)
		if err != nil {
			logrus.Errorf("error parsing Customer ID for item %d: %v, from data: %v", i, err, odooData.CustomerId)
			continue
		}

		// FIX this soon coz now using char
		// snID, sn, err := parseJSONIDDataCombined(odooData.SnId)
		// if err != nil {
		// 	logrus.Errorf("error parsing SN ID for item %d: %v, from data: %v", i, err, odooData.SnId)
		// 	continue
		// }

		// productID, product, err := parseJSONIDDataCombined(odooData.ProductId)
		// if err != nil {
		// 	logrus.Errorf("error parsing Product ID for item %d: %v, from data: %v", i, err, odooData.ProductId)
		// 	continue
		// }

		var slaDeadline *time.Time
		if !odooData.SlaDeadline.Time.IsZero() {
			slaDeadline = &odooData.SlaDeadline.Time
		}

		// if err := db.First(&model.TicketHommyPayCC{}, "ticket_id = ?", odooData.ID).Error; err != nil {
		if err := db.First(&model.TicketHommyPayCC{}, "ticket_number = ?", odooData.Subject.String).Error; err != nil {
			if err == gorm.ErrRecordNotFound {
				newData := model.TicketHommyPayCC{
					ID:             odooData.ID,
					TicketId:       int(odooData.ID),
					TicketNumber:   odooData.Subject.String,
					TicketTypeId:   ticketTypeID,
					TicketType:     ticketType,
					StageId:        stageID,
					Stage:          stage,
					TechnicianId:   technicianID,
					TechnicianName: technician,
					Priority:       odooData.Priority.String,
					CustomerId:     customerID,
					Customer:       customer,
					CustomerPhone:  odooData.CustomerPhone.String,
					CustomerEmail:  odooData.CustomerEmail.String,
					// ADD Customer Address field
					// SnId:         snID, // ADD soon if its already using array not char
					// ProductId:    productID,
					SlaDeadline:  slaDeadline,
					Sn:           odooData.Sn.String,
					Product:      odooData.Product.String,
					Description:  odooData.Description.String,
					AdminId:      1, // default by System
					Keterangan:   config.GetConfig().HommyPayCCData.KetFromOdoo,
					StatusInOdoo: stage, // FIX this coz Odoo Stage is the same as StatusInOdoo
				}
				if err := db.Create(&newData).Error; err != nil {
					logrus.Errorf("Failed to create new Ticket Hommy Pay CC: %v", err)
				} else {
					logrus.Infof("Created new Ticket Hommy Pay CC with ID: %d", odooData.ID)
				}
			} else {
				logrus.Errorf("Failed to check or create Ticket Hommy Pay CC: %v", err)
			}
		} else {
			// if err := db.Model(&model.TicketHommyPayCC{}).Where("ticket_id = ?", odooData.ID).Updates(map[string]interface{}{
			if err := db.Model(&model.TicketHommyPayCC{}).Where("ticket_number = ?", odooData.Subject.String).Updates(map[string]interface{}{
				"ticket_id":       odooData.ID,
				"ticket_number":   odooData.Subject.String,
				"ticket_type_id":  ticketTypeID,
				"ticket_type":     ticketType,
				"stage_id":        stageID,
				"stage":           stage,
				"technician_id":   technicianID,
				"technician_name": technician,
				"priority":        odooData.Priority.String,
				"customer_id":     customerID,
				"customer":        customer,
				"customer_phone":  odooData.CustomerPhone.String,
				"customer_email":  odooData.CustomerEmail.String,
				// ADD Customer Address
				// FIX this soon if already using array coz now its using char
				// "sn_id":       snID,
				// "product_id":  productID,
				"sla_deadline": slaDeadline,
				"sn":           odooData.Sn.String,
				"product":      odooData.Product.String,
				"description":  odooData.Description.String,
				// "admin_id":    1, // default by System
				"keterangan":     config.GetConfig().HommyPayCCData.KetFromOdoo,
				"status_in_odoo": stage, // FIX this coz Odoo Stage is the same as StatusInOdoo
			}).Error; err != nil {
				logrus.Errorf("Failed to update Ticket Hommy Pay CC with ID %d: %v", odooData.ID, err)
			} else {
				logrus.Infof("Updated Ticket Hommy Pay CC with ID: %d", odooData.ID)
			}
		}
	}

	return "Ticket Hommy Pay CC data successfully updated @" + time.Now().Format("15:04:05, 02 Jan 2006"), nil
}

func RefreshMerchantHommyPay(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		result, err := GetMerchantHommyPayCC(db)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}

		c.JSON(http.StatusOK, gin.H{"message": result})
	}
}

func LastUpdateMerchantHommyPay(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		// Get the latest update timestamp from the database
		var lastUpdatedData time.Time
		if err := db.Model(&model.MerchantHommyPayCC{}).
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

func TableMerchantHommyPay(db *gorm.DB) gin.HandlerFunc {
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

		t := reflect.TypeOf(model.MerchantHommyPayCC{})

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
		filteredQuery := db.Model(&model.MerchantHommyPayCC{})

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
		db.Model(&model.MerchantHommyPayCC{}).Count(&totalRecords)

		// Count the number of filtered records
		var filteredRecords int64
		filteredQuery.Count(&filteredRecords)

		// Apply sorting and pagination to the filtered query
		query := filteredQuery.Order(orderString)
		var DbData []model.MerchantHommyPayCC
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
					t := fieldValue.Interface().(time.Time)

					switch theKey {
					case "birthdate":
						newData[theKey] = t.Format(fun.T_YYYYMMDD)

					case "date":
						newData[theKey] = t.Add(7 * time.Hour).Format(fun.T_YYYYMMDD_HHmmss)

					default:
						newData[theKey] = t.Format(fun.T_YYYYMMDD_HHmmss)
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
