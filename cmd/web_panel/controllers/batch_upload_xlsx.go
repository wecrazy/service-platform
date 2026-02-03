package controllers

import (
	"fmt"
	"mime/multipart"
	"net/http"
	"path/filepath"
	"reflect"
	"service-platform/cmd/web_panel/fun"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/sirupsen/logrus"
	"github.com/xuri/excelize/v2"
	"gorm.io/gorm"
)

// Generic function to handle batch uploads
func PostBatchUpload[T any](db *gorm.DB) gin.HandlerFunc {
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
		msg, err := parseAndProcessExcel[T](file, db)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to parse file: " + err.Error()})
			return
		}

		c.JSON(http.StatusOK, msg)
	}
}

// Generic Excel parsing and processing function
func parseAndProcessExcel[T any](file *multipart.FileHeader, db *gorm.DB) (map[string]interface{}, error) {
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
		rowStep := 0
		for j := 0; j < modelType.NumField(); j++ {
			field := modelType.Field(j)
			jsonKey := field.Tag.Get("json")
			time_format := field.Tag.Get("time_format")
			gorm := field.Tag.Get("gorm")
			if jsonKey == "" || jsonKey == "-" || jsonKey == "id" {
				continue
			}
			if i == 0 {
				continue
			}
			if time_format != "" {
				if _, err := time.Parse(time_format, row[rowStep]); err != nil {
					skipThis = true
					warning = append(warning, "Invalid DateTime Format (Example : "+time_format+") Current "+jsonKey+" : "+row[rowStep])
					continue
				}
			}
			if strings.Contains(gorm, "not null") && row[rowStep] == "" {
				skipThis = true
				warning = append(warning, "Field "+jsonKey+" : "+row[rowStep]+" is Empty, Must Not Null")
				continue
			}

			if j <= len(row) && row[rowStep] != "" { // Ensure no out-of-range errors
				if strings.Contains(gorm, "unique") {
					if fun.StringContains(uniqueCheck[jsonKey], row[rowStep]) {
						skipThis = true
						warning = append(warning, "Duplicate in Excel "+jsonKey+" : "+row[rowStep])
					}
					uniqueCheck[jsonKey] = append(uniqueCheck[jsonKey], row[rowStep])
				}
				record[jsonKey] = row[rowStep]
			}
			rowStep++
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

// getTableName dynamically calls the TableName() method of a GORM model
func getTableName(model interface{}) string {
	// Check if the TableName method exists
	if tableNamer, ok := model.(interface {
		TableName() string
	}); ok {
		return tableNamer.TableName()
	}
	// If TableName method doesn't exist, return a default name (snake_case of struct name)
	return strings.ToLower(reflect.TypeOf(model).Name())
}
