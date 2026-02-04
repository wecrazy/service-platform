package controllers

import (
	"bytes"
	"encoding/json"
	"errors"
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
	"gorm.io/datatypes"
	"gorm.io/gorm"
)

func TableWhatsappUserManagement() gin.HandlerFunc {
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

		t := reflect.TypeOf(model.WAPhoneUser{})

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
		filteredQuery := dbWeb.Model(&model.WAPhoneUser{})

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
		dbWeb.Model(&model.WAPhoneUser{}).Count(&totalRecords)

		// Count the number of filtered records
		var filteredRecords int64
		filteredQuery.Count(&filteredRecords)

		// Apply sorting and pagination to the filtered query
		query := filteredQuery.Order(orderString)
		var Dbdata []model.WAPhoneUser
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

				case "is_registered":
					t := fieldValue.Interface().(bool)
					var registeredStatus string
					if t {
						registeredStatus = "<span class='badge bg-label-success'>User has been registered</span>"
					} else {
						registeredStatus = "<span class='badge bg-danger'>User not registered</span>"
					}
					newData[theKey] = registeredStatus
				case "is_banned":
					t := fieldValue.Interface().(bool)
					var bannedStatus string
					if t {
						var sb strings.Builder
						sb.WriteString("<span class='badge bg-danger'>Banned User</span>")
						sb.WriteString(fmt.Sprintf(`
						<button 
							onclick="unbanUser('%d')" 
							class="btn btn-sm btn-label-danger ms-2" 
							title="Unban User">
							<i class="fas fa-unlock"></i>
						</button>
						`, dataInDB.ID))
						bannedStatus = sb.String()
					} else {
						bannedStatus = "<span class='badge bg-success'>Allowed User</span>"
					}
					newData[theKey] = bannedStatus
				case "allowed_to_call":
					t := fieldValue.Interface().(bool)
					if t {
						newData[theKey] = "<span class='badge bg-label-success'>Voice & Video Call Allowed</span>"
					} else {
						newData[theKey] = "<span class='badge bg-secondary'>Call Rejected</span>"
					}
				case "allowed_chats":
					t := fieldValue.Interface().(model.WAAllowedChatMode)
					var htmlRendered string
					switch t {
					case model.DirectChat:
						htmlRendered = `<span class="badge bg-warning"><i class="fad fa-comment-dots me-1"></i> Private Message</span>`
					case model.GroupChat:
						htmlRendered = `<span class="badge bg-success"><i class="fad fa-speakers me-1"></i> Group Chat</span>`
					case model.BothChat:
						htmlRendered = `<span class="badge bg-info"><i class="fad fa-comments-alt me-1"></i> Both</span>`
					}
					newData[theKey] = htmlRendered
				case "allowed_types":
					t := fieldValue.Interface().(datatypes.JSON)

					// Try to unmarshal as []string
					var types []string
					if err := json.Unmarshal(t, &types); err != nil {
						newData[theKey] = "Invalid JSON"
					} else {
						// Join into a single string, e.g., "image, video, document"
						newData[theKey] = strings.Join(types, ", ")
					}
				case "user_type":
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
				case "quota_excedeed":
					// Call your helper to get quota reset time
					resetTime, err := GetQuotaResetTime(dataInDB.ID)
					if err != nil {
						logrus.Errorf("error fetching quota exceeded: %v", err)
						newData[theKey] = "Error fetching quota"
					} else if resetTime != nil {
						// format as string (e.g., "2025-07-11 17:00")
						var sb strings.Builder
						sb.WriteString(fmt.Sprintf(`
						<span id="resetTime-%d">
							%s
						</span>
						<button 
							onclick="resetQuotaWhatsappPrompt('%d')" 
							class="btn btn-sm btn-label-danger ms-2" 
							title="Reset quota">
							<i class="fas fa-sync-alt"></i>
						</button>
						`, dataInDB.ID, resetTime.Format("2006-01-02 15:04"), dataInDB.ID))
						value := sb.String()
						newData[theKey] = value
					} else {
						// No quota exceeded now
						newData[theKey] = "N/A"
					}

				case "user_of":
					t := fieldValue.Interface().(model.WAUserOf)
					var htmlRendered string
					switch t {
					case model.UserOfCSNA:
						htmlRendered = config.WebPanel.Get().Default.PT
					case model.UserOfHommyPay:
						htmlRendered = "Hommy Pay"
					}
					newData[theKey] = htmlRendered
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

func DeleteDataFromTableWhatsappUserManagement() gin.HandlerFunc {
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
		var dbData model.WAPhoneUser
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

		if dbData.UserType == model.WaBotSuperUser {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "You cannot delete this data!!"})
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

func CreateNewDataTableWhatsappUserManagement() gin.HandlerFunc {
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
			logrus.Errorln("failed during decryption", err)
			fun.ClearCookiesAndRedirect(c, cookies)
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized, details: " + err.Error()})
			return
		}
		var claims map[string]interface{}
		err = json.Unmarshal(decrypted, &claims)
		if err != nil {
			logrus.Errorf("failed converting JSON to map: %v", err)
			fun.ClearCookiesAndRedirect(c, cookies)
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized, details: " + err.Error()})
			return
		}

		table := config.WebPanel.Get().Database.TbWAPhoneUser
		// Check if the table exists
		if !dbWeb.Migrator().HasTable(table) {
			logrus.Errorf("Table %s does not exist.\n", table)
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid table name, no table named " + table})
			return
		}

		// Use GORM to execute SELECT * LIMIT 1
		var columns map[string]interface{}
		err = dbWeb.Raw("SELECT * FROM " + table + " LIMIT 1").Scan(&columns).Error
		if err != nil {
			logrus.Errorln("failed fetching data:", err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch table columns: " + err.Error()})
			return
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
			if len(values) > 0 {
				if strings.HasSuffix(key, "[]") {
					// multi-select: keep all as []string
					pg_param_db_model[key] = values
				} else {
					// normal field: just keep the first
					pg_param_db_model[key] = values[0]
				}
			}
		}

		// if hasCreatedAt {
		// 	pg_param_db_model["created_at"] = now
		// }
		// if hasUpdatedAt {
		// 	pg_param_db_model["updated_at"] = now
		// }

		dataPgParamDBModel, err := sanitizeAndValidateInputWAUserManagement(pg_param_db_model)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}

		// Insert data into the table using the model struct to get the inserted ID
		var dbData model.WAPhoneUser
		jsonBody, _ := json.Marshal(dataPgParamDBModel)
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

		// Try send message to New User Registered
		if dbData.IsRegistered {
			indonesianMsg := config.WebPanel.Get().Whatsmeow.WelcomingUserID
			englishMsg := config.WebPanel.Get().Whatsmeow.WelcomingUserEN
			jid := "62" + dbData.PhoneNumber + "@s.whatsapp.net"
			SendLangMessage(jid, indonesianMsg, englishMsg, "id")
		}
	}
}

func sanitizeAndValidateInputWAUserManagement(data map[string]interface{}) (map[string]interface{}, error) {
	var fullName, email, phoneNumber, userType string

	if fullNameRaw, ok := data["full_name"].(string); ok && fullNameRaw != "" {
		fullName = fullNameRaw
	}

	if emailRaw, ok := data["email"].(string); ok && emailRaw != "" {
		data["email"] = strings.ToLower(strings.TrimSpace(emailRaw))
		if !fun.IsValidEmail(emailRaw) {
			return nil, errors.New("invalid email format")
		}
		email = emailRaw
	}

	if phoneRaw, ok := data["phone_number"].(string); ok && phoneRaw != "" {
		sanitizedNumber, err := fun.SanitizePhoneNumber(phoneRaw)
		if err != nil {
			return nil, err
		}

		validWANumber, err := CheckValidWhatsappPhoneNumber(phoneRaw)
		if err != nil {
			return nil, err
		}
		data["phone_number"] = validWANumber
		phoneNumber = sanitizedNumber
	}

	if userTypeRaw, ok := data["user_type"]; ok && userTypeRaw != nil {
		userTypeStr, ok := userTypeRaw.(string)
		if !ok {
			return nil, errors.New("user_type must be a string")
		}
		userTypeStr = strings.ToLower(strings.TrimSpace(userTypeStr))
		if !isValidWAUserType(userTypeStr) {
			return nil, fmt.Errorf("invalid user_type: %s", userTypeStr)
		}
		data["user_type"] = userTypeStr
		userType = userTypeStr
	}

	// Validate & sanitize max_daily_quota
	if maxDailyQuotaRaw, ok := data["max_daily_quota"]; ok {
		switch v := maxDailyQuotaRaw.(type) {
		case string:
			if quota, err := strconv.Atoi(v); err == nil && quota > 0 {
				if quota > config.WebPanel.Get().Whatsmeow.WhatsappMaxDailyQuota {
					return nil, fmt.Errorf("max daily quota cannot more than %d", config.WebPanel.Get().Whatsmeow.WhatsappMaxDailyQuota)
				}
				data["max_daily_quota"] = quota
			} else {
				return nil, errors.New("invalid max_daily_quota: must be a positive integer")
			}
		case float64:
			if int(v) > 0 {
				if int(v) > config.WebPanel.Get().Whatsmeow.WhatsappMaxDailyQuota {
					return nil, fmt.Errorf("max daily quota cannot more than %d", config.WebPanel.Get().Whatsmeow.WhatsappMaxDailyQuota)
				}
				data["max_daily_quota"] = int(v)
			} else {
				return nil, errors.New("invalid max_daily_quota: must be > 0")
			}
		case int:
			if v > 0 {
				if v > config.WebPanel.Get().Whatsmeow.WhatsappMaxDailyQuota {
					return nil, fmt.Errorf("max daily quota cannot more than %d", config.WebPanel.Get().Whatsmeow.WhatsappMaxDailyQuota)
				}
				data["max_daily_quota"] = v
			} else {
				return nil, errors.New("invalid max_daily_quota: must be > 0")
			}
		default:
			return nil, errors.New("invalid type for max_daily_quota: must be a positive integer")
		}
	}

	if isRegisteredRaw, ok := data["is_registered"]; ok {
		var isRegistered bool

		switch v := isRegisteredRaw.(type) {
		case bool:
			isRegistered = v
		case string:
			switch strings.ToLower(v) {
			case "true", "1", "yes", "y":
				isRegistered = true
			case "false", "0", "no", "n":
				isRegistered = false
			default:
				return nil, fmt.Errorf("invalid boolean value for is_registered: %v", v)
			}
		case int:
			isRegistered = v != 0
		case float64:
			isRegistered = v != 0.0
		default:
			return nil, errors.New("invalid type for is_registered, expected boolean")
		}

		data["is_registered"] = isRegistered
	}

	if allowedToCallRaw, ok := data["allowed_to_call"]; ok {
		var allowedToCall bool

		switch v := allowedToCallRaw.(type) {
		case bool:
			allowedToCall = v
		case string:
			switch strings.ToLower(v) {
			case "true", "1", "yes", "y":
				allowedToCall = true
			case "false", "0", "no", "n":
				allowedToCall = false
			default:
				return nil, fmt.Errorf("invalid boolean value for allowed_to_call: %v", v)
			}
		case int:
			allowedToCall = v != 0
		case float64:
			allowedToCall = v != 0.0
		default:
			return nil, errors.New("invalid type for allowed_to_call, expected boolean")
		}

		data["allowed_to_call"] = allowedToCall
	}

	if rawAllowedTypes, ok := data["allowed_types[]"]; ok {
		// Handle empty/nil case first
		if rawAllowedTypes == nil {
			return nil, errors.New("allowed_types cannot be null")
		}

		switch v := rawAllowedTypes.(type) {
		case []string:
			if len(v) == 0 {
				return nil, errors.New("allowed_types cannot be an empty array")
			}
			// Optional: Validate each element isn't empty
			for _, t := range v {
				if strings.TrimSpace(t) == "" {
					return nil, errors.New("allowed_types cannot contain empty strings")
				}
			}
			data["allowed_types"] = v

		case string:
			if strings.TrimSpace(v) == "" {
				return nil, errors.New("allowed_types cannot be an empty string")
			}
			data["allowed_types"] = []string{v}

		default:
			return nil, errors.New("invalid type for allowed_types, expected array of strings or single string")
		}

		delete(data, "allowed_types[]")
	} else {
		// If field is required but missing entirely
		return nil, errors.New("allowed_types is required")
	}

	// Check if technician exists in ODOO MS
	if userType == string(model.ODOOMSTechnician) {
		technicianIsExists, err := checkExistingTechnicianInODOOMS(fullName, email, phoneNumber)
		if err != nil {
			return nil, fmt.Errorf("user: %s, email: %s, phone: %s is not exists as technician in ODOO MS. Details: %v", fullName, email, phoneNumber, err)
		}
		if !technicianIsExists {
			return nil, fmt.Errorf("user: %s, email: %s, phone: %s is not exists as technician in ODOO MS", fullName, email, phoneNumber)
		}
	}

	// Check existing data
	sanitizedPhoneNumber, err := fun.SanitizePhoneNumber(phoneNumber)
	if err != nil {
		return nil, err
	}
	var existingData model.WAPhoneUser
	if err := dbWeb.
		Where("email LIKE ? OR phone_number LIKE ?", "%"+email+"%", "%"+sanitizedPhoneNumber+"%").
		First(&existingData).Error; err != nil {

		if errors.Is(err, gorm.ErrRecordNotFound) {
			// not found: that's okay
		} else {
			// other DB error
			return nil, err
		}
	} else {
		// existing data found
		return nil, fmt.Errorf("user with this email: %s or phone: %s already exists", email, phoneNumber)
	}

	return data, nil
}

func isValidWAUserType(userType string) bool {
	for _, allowed := range model.AllWAUserTypes {
		if string(allowed) == userType {
			return true
		}
	}
	return false
}

func PutUpdatedWhatsappUserManagement() gin.HandlerFunc {
	return func(c *gin.Context) {

		table := config.WebPanel.Get().Database.TbWAPhoneUser
		// Check if the table exists
		if !dbWeb.Migrator().HasTable(table) {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid table name, no table named " + table})
			return
		}

		// Use GORM to execute SELECT * LIMIT 1
		var columns map[string]interface{}
		err := dbWeb.Raw("SELECT * FROM " + table + " LIMIT 1").Scan(&columns).Error
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("Failed to fetch data from tb %s, %v", table, err)})
			return
		}

		// Bind the incoming form data to a map to check keys dynamically
		var jsonBody map[string]string
		if err := c.Bind(&jsonBody); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid form data " + err.Error()})
			return
		}

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
			if len(values) > 0 {
				// Assuming each field has only one value, pick the first one
				// if strings.HasSuffix(key, "[]") {
				// 	// multi-select: keep all as []string
				// 	data_map[key] = values
				// } else {
				// 	// normal field: just keep the first
				// 	data_map[key] = values[0]
				// }
				data_map[key] = values
			}
		}

		now := time.Now()
		if hasUpdatedAt {
			data_map["updated_at"] = now
		}

		dataMapped, err := sanitizeAndValidateEditedWAUserManagement(data_map)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}

		// Perform the update
		result := dbWeb.Table(table).Where("id = ?", data_map["id"]).Updates(dataMapped)

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
			logrus.Errorf("failed during decryption: %v", err)
			fun.ClearCookiesAndRedirect(c, cookies)
			return
		}
		var claims map[string]interface{}
		err = json.Unmarshal(decrypted, &claims)
		if err != nil {
			logrus.Errorf("Error converting JSON to map: %v", err)
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

func sanitizeAndValidateEditedWAUserManagement(data map[string]interface{}) (map[string]interface{}, error) {
	var fullName, email, phoneNumber, userType string

	if fullNameRaw, ok := data["full_name"].(string); ok && fullNameRaw != "" {
		fullName = fullNameRaw
	}

	if emailRaw, ok := data["email"].(string); ok && emailRaw != "" {
		data["email"] = strings.ToLower(strings.TrimSpace(emailRaw))
		if !fun.IsValidEmail(emailRaw) {
			return nil, fmt.Errorf("invalid email format: %s", emailRaw)
		}
		email = emailRaw
	}

	if phoneRaw, ok := data["phone_number"].(string); ok && phoneRaw != "" {
		sanitizedNumber, err := fun.SanitizePhoneNumber(phoneRaw)
		if err != nil {
			return nil, err
		}

		validWANumber, err := CheckValidWhatsappPhoneNumber(phoneRaw)
		if err != nil {
			return nil, err
		}
		data["phone_number"] = validWANumber
		phoneNumber = sanitizedNumber
	}

	if userTypeRaw, ok := data["user_type"]; ok && userTypeRaw != nil {
		userTypeStr, ok := userTypeRaw.(string)
		if !ok {
			return nil, errors.New("user_type must be a string")
		}
		userTypeStr = strings.ToLower(strings.TrimSpace(userTypeStr))
		if !isValidWAUserType(userTypeStr) {
			return nil, fmt.Errorf("invalid user_type: %s", userTypeStr)
		}
		data["user_type"] = userTypeStr
		userType = userTypeStr
	}

	// Validate & sanitize max_daily_quota
	if maxDailyQuotaRaw, ok := data["max_daily_quota"]; ok {
		switch v := maxDailyQuotaRaw.(type) {
		case string:
			if quota, err := strconv.Atoi(v); err == nil && quota > 0 {
				if quota > config.WebPanel.Get().Whatsmeow.WhatsappMaxDailyQuota {
					return nil, fmt.Errorf("max daily quota cannot more than %d", config.WebPanel.Get().Whatsmeow.WhatsappMaxDailyQuota)
				}
				data["max_daily_quota"] = quota
			} else {
				return nil, errors.New("invalid max_daily_quota: must be a positive integer")
			}
		case float64:
			if int(v) > 0 {
				if int(v) > config.WebPanel.Get().Whatsmeow.WhatsappMaxDailyQuota {
					return nil, fmt.Errorf("max daily quota cannot more than %d", config.WebPanel.Get().Whatsmeow.WhatsappMaxDailyQuota)
				}
				data["max_daily_quota"] = int(v)
			} else {
				return nil, errors.New("invalid max_daily_quota: must be > 0")
			}
		case int:
			if v > 0 {
				if v > config.WebPanel.Get().Whatsmeow.WhatsappMaxDailyQuota {
					return nil, fmt.Errorf("max daily quota cannot more than %d", config.WebPanel.Get().Whatsmeow.WhatsappMaxDailyQuota)
				}
				data["max_daily_quota"] = v
			} else {
				return nil, errors.New("invalid max_daily_quota: must be > 0")
			}
		default:
			return nil, errors.New("invalid type for max_daily_quota: must be a positive integer")
		}
	}

	if allowedToCallRaw, ok := data["allowed_to_call"]; ok {
		var allowedToCall bool

		switch v := allowedToCallRaw.(type) {
		case bool:
			allowedToCall = v
		case string:
			switch strings.ToLower(v) {
			case "true", "1", "yes", "y":
				allowedToCall = true
			case "false", "0", "no", "n":
				allowedToCall = false
			default:
				return nil, fmt.Errorf("invalid boolean value for allowed_to_call: %v", v)
			}
		case int:
			allowedToCall = v != 0
		case float64:
			allowedToCall = v != 0.0
		default:
			return nil, errors.New("invalid type for allowed_to_call, expected boolean")
		}

		data["allowed_to_call"] = allowedToCall
	}

	// ADD if u want to use it
	_ = fullName
	_ = userType

	// Check existing data
	sanitizedPhoneNumber, err := fun.SanitizePhoneNumber(phoneNumber)
	if err != nil {
		return nil, err
	}
	var existingData []model.WAPhoneUser // Use a slice instead of a single struct
	// Find all matching records (not just the first one)
	if err := dbWeb.
		Where("email LIKE ? OR phone_number LIKE ?", "%"+email+"%", "%"+sanitizedPhoneNumber+"%").
		Find(&existingData).Error; err != nil {
		// Handle DB error (other than "not found")
		return nil, err
	}

	if len(existingData) > 0 {
		// At least one record exists
		if len(existingData) > 1 {
			// Multiple records found
			return nil, fmt.Errorf("multiple users found with email: %s or phone: %s", email, phoneNumber)
		}
		// Only one record exists
		// return nil, fmt.Errorf("user with this email: %s or phone: %s already exists", email, phoneNumber)
	}

	// Return data mapped
	return data, nil
}

func GetBatchTemplateWhatsappUserManagement[T any]() gin.HandlerFunc {
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
		structName := "Whatsapp User Management"

		titleTextStyle, _ := f.NewStyle(&excelize.Style{
			Font: &excelize.Font{
				Bold:   true,
				Color:  "#FFFFFF",
				Family: "Arial",
				Size:   9,
			},
			Fill: excelize.Fill{
				Type:    "pattern",
				Color:   []string{"#EE0000"},
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

		disallowedKeys := map[string]bool{
			"":               true,
			"-":              true,
			"is_registered":  true,
			"is_banned":      true,
			"quota_excedeed": true,
		}

		colIndex := 0
		for i := 0; i < t.NumField(); i++ {
			field := t.Field(i)
			jsonKey := field.Tag.Get("json")

			// Skip disallowed fields
			if disallowedKeys[jsonKey] {
				continue
			}

			// Compute the correct column letter
			col := fun.NumberToAlphabet(colIndex + 1) // +1 because Excel columns start at 1 (A)
			colIndex++

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
		f.SetCellValue(sheetName, "B1", fmt.Sprintf("*separator for allowed types must: %s", config.WebPanel.Get().Whatsmeow.KeywordSeparator))

		// Dummy data values in rows (each inner slice is a row)
		dataSeparator := config.WebPanel.Get().Whatsmeow.KeywordSeparator
		// Prepare list of allowed values
		messageTypes := model.AllWAMessageTypes
		userTypes := model.AllWAUserTypes
		allowedChats := model.AllWAAllowedChatModes
		userOfs := model.AllUserOf

		// Join them into a single string using separator if you have (or comma as default)
		messageTypeStrs := make([]string, len(messageTypes))
		for i, v := range messageTypes {
			messageTypeStrs[i] = string(v)
		}
		messageTypeList := "'" + strings.Join(messageTypeStrs, fmt.Sprintf("'%s'", ",")+" '") + "'"

		userTypeStrs := make([]string, len(userTypes))
		for i, v := range userTypes {
			userTypeStrs[i] = string(v)
		}
		userTypeList := "'" + strings.Join(userTypeStrs, fmt.Sprintf("'%s'", ",")) + "'"

		allowedChatStrs := make([]string, len(allowedChats))
		for i, v := range allowedChats {
			allowedChatStrs[i] = string(v)
		}
		allowedChatList := "'" + strings.Join(allowedChatStrs, fmt.Sprintf("'%s'", ",")) + "'"

		userOfStrs := make([]string, len(userOfs))
		for i, v := range userOfs {
			userOfStrs[i] = string(v)
		}
		allowedUserOfList := "'" + strings.Join(userOfStrs, fmt.Sprintf("'%s'", ",")) + "'"

		dummyValues := [][]string{
			{
				"e.g. : Developer Rawamangun (Max. 255)",
				"e.g. : rawamangun@dbest.yeah (Max. 255)",
				"e.g. : 08512345678911111111 (Unique & Max. 20)",
				fmt.Sprintf("e.g. : %s (Select 1 from [%s])", model.DirectChat, allowedChatList),
				fmt.Sprintf(
					"e.g. : %s %s %s (Multiple values allowed, separate with %s, choose from [%s])",
					model.TextMessage, dataSeparator, model.ImageMessage, dataSeparator, messageTypeList,
				),
				"e.g. : true (true or false)",
				"e.g. : 10 (Limit message can be send)",
				"e.g. : Pretty user (Max. 255)",
				fmt.Sprintf("e.g. : %s (Select 1 from [%s])", model.CommonUser, userTypeList),
				fmt.Sprintf("e.g. : %s (Select 1 from [%s])", model.UserOfHommyPay, allowedUserOfList),
			},
			{
				"Teknisi Jakarta",
				"teknisi@jakarta.odooms",
				config.WebPanel.Get().Whatsmeow.WaSupport,
				string(model.DirectChat),
				string(model.TextMessage),
				"false",
				"25",
				"Technician in ODOO MS",
				string(model.ODOOMSTechnician),
				string(model.UserOfCSNA),
			},
			{
				"User Biasa",
				"biasa@aja.com",
				config.WebPanel.Get().Whatsmeow.WaTechnicalSupport,
				string(model.BothChat),
				string(model.TextMessage),
				"false",
				"10",
				"User biasa yang hanya perlu chat biasa saja",
				string(model.CommonUser),
				string(model.UserOfHommyPay),
			},
		}

		// Start writing from row index 3
		startRow := 3

		for rowOffset, row := range dummyValues {
			rowNumber := startRow + rowOffset
			for colOffset, value := range row {
				colLetter := fun.NumberToAlphabet(colOffset + 1) // +1 because Excel columns start at 1 (A)
				cell := fmt.Sprintf("%s%d", colLetter, rowNumber)
				f.SetCellValue(sheetName, cell, value)
				f.SetCellStyle(sheetName, cell, cell, textStyle)
			}
		}

		// Generate column letters from A to I
		columns := []string{}
		for i := 1; i <= 9; i++ { // 1→A, 2→B, ..., 9→I
			col := fun.NumberToAlphabet(i)
			columns = append(columns, col)
		}

		for _, col := range columns {
			err := f.SetColWidth(sheetName, col, col, 35)
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

func PostBatchUploadDataWhatsappUserManagement[T any]() gin.HandlerFunc {
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
		msg, err := parseAndProcessExcelFromTemplateWhatsappUserManagement[T](file)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to parse file: " + err.Error()})
			return
		}

		c.JSON(http.StatusOK, msg)
	}
}

func parseAndProcessExcelFromTemplateWhatsappUserManagement[T any](file *multipart.FileHeader) (map[string]interface{}, error) {
	f, err := file.Open()
	if err != nil {
		return nil, err
	}
	defer f.Close()

	xlsx, err := excelize.OpenReader(f)
	if err != nil {
		return nil, err
	}

	rows, err := xlsx.GetRows("Sheet1")
	if err != nil {
		return nil, err
	}

	var tableModel T
	modelType := reflect.TypeOf(tableModel)
	if modelType.Kind() == reflect.Ptr {
		modelType = modelType.Elem()
	}
	tableName := getTableName(tableModel)

	var warnings []string
	uniqueCheck := make(map[string]map[string]bool) // field -> set
	var insertRecords []map[string]interface{}

	if len(rows) < 3 {
		return map[string]interface{}{"warning": []string{"Excel has too few rows"}}, nil
	}

	headerRow := rows[1]
	fieldColumnMap := make(map[string]int)

	// build column map
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

	// prepare unique maps
	for j := 0; j < modelType.NumField(); j++ {
		field := modelType.Field(j)
		jsonKey := field.Tag.Get("json")
		if jsonKey != "" && strings.Contains(field.Tag.Get("gorm"), "unique") {
			uniqueCheck[jsonKey] = make(map[string]bool)
		}
	}

	now := time.Now()

	for i := 2; i < len(rows); i++ {
		row := rows[i]
		record := make(map[string]interface{})
		skip := false

		// Collect values we’ll need after the loop
		var fullName, email, phoneNumber, userType, maxDailyQuota string

		for j := 0; j < modelType.NumField(); j++ {
			field := modelType.Field(j)
			jsonKey := field.Tag.Get("json")
			if jsonKey == "" || jsonKey == "-" || jsonKey == "id" {
				continue
			}

			colIdx, ok := fieldColumnMap[jsonKey]
			if !ok || colIdx >= len(row) {
				continue
			}
			cell := strings.TrimSpace(row[colIdx])

			// check NOT NULL
			gormTag := field.Tag.Get("gorm")
			if strings.Contains(gormTag, "not null") && cell == "" {
				warnings = append(warnings, fmt.Sprintf("Row %d: %s must not be empty", i+1, jsonKey))
				skip = true
				continue
			}

			// remember jsonKey for later use
			switch jsonKey {
			case "full_name":
				fullName = cell
			case "email":
				email = cell
			case "phone_number":
				phoneNumber = cell
			case "user_type":
				userType = cell
			case "max_daily_quota":
				maxDailyQuota = cell
			}

			// check unique in Excel
			if cell != "" && uniqueCheck[jsonKey] != nil {
				if uniqueCheck[jsonKey][cell] {
					warnings = append(warnings, fmt.Sprintf("Row %d: Duplicate in Excel for %s: %s", i+1, jsonKey, cell))
					skip = true
					continue
				}
				uniqueCheck[jsonKey][cell] = true
			}

			// type-aware parsing
			switch field.Type {
			case reflect.TypeOf(datatypes.JSON{}):
				if cell != "" {
					items := strings.Split(cell, config.WebPanel.Get().Whatsmeow.KeywordSeparator)
					for k := range items {
						items[k] = strings.TrimSpace(items[k])
					}
					b, err := json.Marshal(items)
					if err != nil {
						warnings = append(warnings, fmt.Sprintf("Row %d: Invalid JSON in %s: %v", i+1, jsonKey, err))
						skip = true
						continue
					}
					record[jsonKey] = datatypes.JSON(b)
				}

			case reflect.TypeOf(model.WAAllowedChatMode("")):
				candidate := model.WAAllowedChatMode(cell)
				valid := map[model.WAAllowedChatMode]bool{
					model.DirectChat: true,
					model.GroupChat:  true,
					model.BothChat:   true,
				}
				if !valid[candidate] {
					warnings = append(warnings, fmt.Sprintf("Row %d: Invalid allowed_chats value: %s", i+1, cell))
					skip = true
					continue
				}
				record[jsonKey] = candidate

			case reflect.TypeOf(model.WAUserOf("")):
				candidate := model.WAUserOf(cell)
				valid := map[model.WAUserOf]bool{
					model.UserOfCSNA:     true,
					model.UserOfHommyPay: true,
				}
				if !valid[candidate] {
					warnings = append(warnings, fmt.Sprintf("Row %d: Invalid user_of value: %s", i+1, cell))
					skip = true
					continue
				}
				record[jsonKey] = candidate

			case reflect.TypeOf(model.WAUserType("")):
				candidate := model.WAUserType(cell)
				valid := map[model.WAUserType]bool{
					model.CommonUser:       true,
					model.ODOOMSTechnician: true,
					model.OdooManager:      true,
					model.SupportStaff:     true,
					model.Administrator:    true,
					model.ServiceAccount:   true,
					model.WaBotSuperUser:   true,
				}
				if !valid[candidate] {
					warnings = append(warnings, fmt.Sprintf("Row %d: Invalid user_type value: %s", i+1, cell))
					skip = true
					continue
				}
				record[jsonKey] = candidate

			default:
				// fallback by kind
				switch field.Type.Kind() {
				case reflect.Int, reflect.Int64:
					if cell != "" {
						n, convErr := strconv.ParseInt(cell, 10, 64)
						if convErr != nil {
							warnings = append(warnings, fmt.Sprintf("Row %d: Invalid number in %s: %s", i+1, jsonKey, cell))
							skip = true
							continue
						}
						record[jsonKey] = n
					}
				case reflect.Float32, reflect.Float64:
					if cell != "" {
						f, convErr := strconv.ParseFloat(cell, 64)
						if convErr != nil {
							warnings = append(warnings, fmt.Sprintf("Row %d: Invalid float in %s: %s", i+1, jsonKey, cell))
							skip = true
							continue
						}
						record[jsonKey] = f
					}
				case reflect.Bool:
					record[jsonKey] = strings.EqualFold(cell, "true")
				default:
					record[jsonKey] = cell
				}
			}
		}

		// ✅ After parsing fields: do cross-field checks
		userIsRegistered := false
		if phoneNumber != "" {
			activeWAPhoneNumber, err := CheckValidWhatsappPhoneNumber(phoneNumber)
			if err != nil {
				warnings = append(warnings, fmt.Sprintf("Row %d: phone_number is not registered in WhatsApp: %s", i+1, err))
				skip = true
			} else {
				sanitizedPhoneNumber, err := fun.SanitizePhoneNumber(activeWAPhoneNumber)
				if err != nil {
					warnings = append(warnings, fmt.Sprintf("Row %d: phone_number is not valid number: %s", i+1, err))
					skip = true
				} else {
					// Use sanitized for phone number coz its start with 8xxxx
					phoneNumber = sanitizedPhoneNumber

					// check if phone number already exists in DB
					var countData int64
					err = dbWeb.Table(tableName).
						Where("phone_number LIKE ? OR phone_number LIKE ? OR phone_number LIKE ?",
							"%"+phoneNumber+"%",
							"%"+activeWAPhoneNumber+"%",
							"%"+sanitizedPhoneNumber+"%").
						Count(&countData).Error
					if err != nil {
						return nil, fmt.Errorf("DB check failed: %w", err)
					}
					if countData > 0 {
						warnings = append(warnings, fmt.Sprintf("Row %d: phone_number already exists in DB: %s", i+1, phoneNumber))
						skip = true
					} else {
						userIsRegistered = true
					}
				}
			}
		}

		// example: if user_type == technician, do more check (e.g. ODOO check)
		if userType == string(model.ODOOMSTechnician) {
			technicianExist, err := checkExistingTechnicianInODOOMS(fullName, email, phoneNumber)
			if err != nil {
				warnings = append(warnings, fmt.Sprintf("Technician %s - %s (%s) does not exist in ODOO Manage Service. Please add them first! Details: %v", fullName, email, phoneNumber, err))
				userIsRegistered = false
			}
			if technicianExist {
				userIsRegistered = true
			}
		}

		if maxDailyQuota != "" {
			quota, convErr := strconv.Atoi(maxDailyQuota)
			if convErr != nil {
				warnings = append(warnings, fmt.Sprintf("Row %d: Invalid number in max_daily_quota: %s", i+1, maxDailyQuota))
				skip = true
			} else {
				if quota > config.WebPanel.Get().Whatsmeow.WhatsappMaxDailyQuota {
					warnings = append(warnings, fmt.Sprintf("Row %d: Max daily quota cannot more than: %d", i+1, config.WebPanel.Get().Whatsmeow.WhatsappMaxDailyQuota))
					skip = true
				}
				record["max_daily_quota"] = quota
			}
		}

		// add is_registered flag
		record["is_registered"] = userIsRegistered

		if !skip {
			insertRecords = append(insertRecords, record)
		}
	}

	// unique in DB
	for jsonKey, values := range uniqueCheck {
		var valList []string
		for v := range values {
			valList = append(valList, v)
		}
		if len(valList) == 0 {
			continue
		}

		var existing []string
		err := dbWeb.Table(tableName).Select(jsonKey).Where(fmt.Sprintf("%s IN ?", jsonKey), valList).Scan(&existing).Error
		if err != nil {
			return nil, fmt.Errorf("failed DB unique check for %s: %w", jsonKey, err)
		}

		existingSet := make(map[string]struct{})
		for _, e := range existing {
			existingSet[fmt.Sprintf("%v", e)] = struct{}{}
		}

		newInsert := insertRecords[:0]
		for _, rec := range insertRecords {
			if val, ok := rec[jsonKey]; ok {
				if _, found := existingSet[fmt.Sprintf("%v", val)]; found {
					warnings = append(warnings, fmt.Sprintf("Duplicate in DB for %s: %v", jsonKey, val))
					continue
				}
			}
			newInsert = append(newInsert, rec)
		}
		insertRecords = newInsert
	}

	// created_at / updated_at
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

	// insert
	err = dbWeb.Transaction(func(tx *gorm.DB) error {
		if len(insertRecords) > 0 {
			if err := tx.Table(tableName).Create(&insertRecords).Error; err != nil {
				return fmt.Errorf("failed batch insert: %w", err)
			}
		}
		return nil
	})
	if err != nil {
		return nil, err
	}

	// Try send message to New User Registered
	for _, rec := range insertRecords {
		if isReg, ok := rec["is_registered"].(bool); ok && isReg {
			// fullName, _ := rec["full_name"].(string)
			phoneNumber, _ := rec["phone_number"].(string)
			sanitizedPhoneNumber, err := fun.SanitizePhoneNumber(phoneNumber)
			if err != nil {
				logrus.Errorf("Failed to sanitize phone number %s: %v", phoneNumber, err)
				continue
			}

			indonesianMsg := config.WebPanel.Get().Whatsmeow.WelcomingUserID
			englishMsg := config.WebPanel.Get().Whatsmeow.WelcomingUserEN

			jid := "62" + sanitizedPhoneNumber + "@s.whatsapp.net"
			SendLangMessage(jid, indonesianMsg, englishMsg, "id")
		}
	}

	return map[string]interface{}{
		"success": len(insertRecords),
		"warning": warnings,
	}, nil
}

func ResetQuotaWhatsappPrompt() gin.HandlerFunc {
	return func(c *gin.Context) {
		var req struct {
			ID string `json:"id"`
		}

		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{
				"success": false,
				"message": "Invalid request payload",
			})
			return
		}

		// Parse string ID into uint64 first
		idUint64, err := strconv.ParseUint(req.ID, 10, 64)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{
				"success": false,
				"message": "Invalid ID format",
			})
			return
		}

		// Convert to uint (platform-dependent: usually uint == uint32)
		id := uint(idUint64)

		// Now use id as uint
		err = ResetQuotaExceeded(id)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{
				"success": false,
				"message": err.Error(),
			})
			return
		}

		c.JSON(http.StatusOK, gin.H{
			"success": true,
		})
	}
}

func UnbanUser() gin.HandlerFunc {
	return func(c *gin.Context) {
		var req struct {
			ID string `json:"id"`
		}

		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{
				"success": false,
				"message": "Invalid request payload",
			})
			return
		}

		// Parse string ID into uint64 first
		idUint64, err := strconv.ParseUint(req.ID, 10, 64)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{
				"success": false,
				"message": "Invalid ID format",
			})
			return
		}

		// Convert to uint (platform-dependent: usually uint == uint32)
		id := uint(idUint64)

		// Now use id as uint
		err = UnbanAndUnlockUser(id)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{
				"success": false,
				"message": err.Error(),
			})
			return
		}

		c.JSON(http.StatusOK, gin.H{
			"success": true,
		})
	}
}
