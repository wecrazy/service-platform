package controllers

import (
	"fmt"
	"net/http"
	"os"
	"reflect"
	"service-platform/cmd/technical_assistance/fun"
	"service-platform/cmd/technical_assistance/model/cc_model"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

func TableMerchantH_1CallLog(db *gorm.DB) gin.HandlerFunc {
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

		t := reflect.TypeOf(cc_model.JOMerchantHmin1CallLog{})

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
		filteredQuery := db.Model(&cc_model.JOMerchantHmin1CallLog{})

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
				switch jsonKey {
				case "cs_name":
					var csData cc_model.CS
					if err := db.Find(&csData).Where("username LIKE ?", "%"+request.Search+"%").Error; err != nil {
						fmt.Print(err)
						continue
					}
					dataField = "id_cs"
					request.Search = strconv.Itoa(csData.ID)
				case "id_cs":
					var csData cc_model.CS
					if err := db.Find(&csData).Where("username LIKE ?", "%"+request.Search+"%").Error; err != nil {
						fmt.Print(err)
						continue
					}
					dataField = "id_cs"
					request.Search = strconv.Itoa(csData.ID)
				}

				filteredQuery = filteredQuery.Or("`"+dataField+"` LIKE ?", "%"+request.Search+"%")

				// fmt.Print("======================\n")
				// fmt.Print("\n")
				// fmt.Print(varName)
				// fmt.Print("\n")
				// fmt.Print(dataType)
				// fmt.Print("\n")
				// fmt.Print(jsonKey)
				// fmt.Print("\n")
				// fmt.Print(gormTag)
				// fmt.Print("======================\n")
				// fmt.Printf("Variable Name: %s, Data Type: %s, JSON Key: %s, GORM Column Key: %s\n", varName, dataType, jsonKey, columnKey)

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
		db.Model(&cc_model.JOMerchantHmin1CallLog{}).Count(&totalRecords)

		// Count the number of filtered records
		var filteredRecords int64
		filteredQuery.Count(&filteredRecords)

		// Apply sorting and pagination to the filtered query
		query := filteredQuery.Order(orderString)
		var Teknisis []cc_model.JOMerchantHmin1CallLog
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

			idCs := 0
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
					} else if theKey == "reschedule" {
						newData[theKey] = fieldValue.Interface().(time.Time).Format(fun.T_DD_MMMM_YYYY)
					} else {
						newData[theKey] = fieldValue.Interface().(time.Time).Format(fun.T_YYYYMMDD_HHmmss)
					}
				} else if theKey == "file_path" {
					path := c.Request.URL.Path

					// Split the path into segments
					segments := strings.Split(strings.TrimPrefix(path, "/"), "/")

					// Check if there are at least two segments
					if len(segments) < 2 {
						c.JSON(400, gin.H{"error": "Path does not have enough segments"})
						return
					}

					// Extract the first two segments
					// firstSegment := segments[0] // "web"
					// secondSegment := segments[1] // "halo"

					newData[theKey] = fmt.Sprintf(`<a href="%s/%s">DOWNLOAD</a>`, segments[0]+"/"+segments[1], fieldValue.Interface().(string))
				} else if strings.Contains(theKey, "img_") {
					if fieldValue.Interface().(string) != "" {
						newData[theKey] = fmt.Sprintf(`<img src='data:image/jpeg;base64, %s' style='display:block; max-width:100px;'/>`, fieldValue.Interface().(string))
					} else {
						newData[theKey] = ""
					}
				} else if strings.Contains(theKey, "_path") {
					if fieldValue.Interface().(string) != "" {
						dir := strings.ReplaceAll(fieldValue.Interface().(string), "\\", `/`)
						newData[theKey] = fmt.Sprintf(`<img src='%s' style='display:block; max-width:100px;'/>`, "https://ms_cc.csna4u.com/"+dir)
					} else {
						newData[theKey] = ""
					}
				} else if theKey == "is_reschedule" {
					if fieldValue.Interface().(bool) {
						newData[theKey] = `<span class="badge rounded-pill bg-label-success">Reschedule</span>`
					} else {
						newData[theKey] = `<span class="badge rounded-pill bg-label-danger">Not Reschedule</span>`
					}
				} else if theKey == "wonumber" {
					newData[theKey] = fmt.Sprintf(`<a href="%v/odooms-project-task/detailWO?wo_number=%v" target="_blank">%v</a>`,
						os.Getenv("WO_DETAIL_URL"),
						fieldValue.Interface().(string),
						fieldValue.Interface().(string))
				} else if theKey == "update_to_odoo" {
					if fieldValue.Interface().(string) != "" {
						switch strings.ToLower(fieldValue.Interface().(string)) {
						case "success":
							newData[theKey] = fmt.Sprintf(`<span class="badge rounded-pill bg-success">%v</span>`, fieldValue.Interface().(string))
						default:
							newData[theKey] = fmt.Sprintf(`<span class="badge rounded-pill bg-secondary">%v</span>`, fieldValue.Interface().(string))
						}
					} else {
						newData[theKey] = ""
					}
				} else if theKey == "id_cs" {
					idCs = fieldValue.Interface().(int)
					// newData[theKey] = fieldValue.Interface().(int)
					var csData cc_model.CS
					if err := db.Where("id = ?", idCs).First(&csData).Error; err != nil {
						newData[theKey] = err.Error()
					} else {
						newData[theKey] = csData.Username
					}
				} else if theKey == "cs_name" {
					var csData cc_model.CS
					if err := db.Where("id = ?", idCs).First(&csData).Error; err != nil {
						newData[theKey] = err.Error()
					} else {
						newData[theKey] = csData.Username
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
