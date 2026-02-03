package controllers

import (
	"encoding/csv"
	"fmt"
	"net/http"
	"reflect"
	"service-platform/cmd/technical_assistance/fun"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

func ExportTable[T any](db *gorm.DB, tableName string) gin.HandlerFunc {
	return func(c *gin.Context) {
		var tableInstance T

		// Set the headers for the CSV download
		c.Header("Content-Description", "File Transfer")
		c.Header("Content-Disposition", fmt.Sprintf("attachment; filename=%s.csv", tableName))
		c.Header("Content-Type", "text/csv")

		// Create a CSV writer
		writer := csv.NewWriter(c.Writer)
		defer writer.Flush()

		// Use reflection to generate CSV headers
		t := reflect.TypeOf(tableInstance)
		if t.Kind() == reflect.Ptr {
			t = t.Elem()
		}

		var tableHeaders []string
		for i := 0; i < t.NumField(); i++ {
			field := t.Field(i)
			jsonKey := field.Tag.Get("json")
			if jsonKey == "" || jsonKey == "-" {
				continue
			}
			// Add formatted header
			tableHeaders = append(tableHeaders, fun.AddSpaceBeforeUppercase(field.Name))
		}
		writer.Write(tableHeaders)

		// Fetch data from the database
		var tableData []T
		if err := db.Find(&tableData).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to retrieve data: " + err.Error()})
			return
		}

		// Write data to the CSV
		for _, row := range tableData {
			v := reflect.ValueOf(row)
			if v.Kind() == reflect.Ptr {
				v = v.Elem()
			}

			var csvRow []string
			for i := 0; i < t.NumField(); i++ {
				field := t.Field(i)
				jsonKey := field.Tag.Get("json")
				if jsonKey == "" || jsonKey == "-" {
					continue
				}

				fieldValue := v.Field(i)
				if fieldValue.Kind() == reflect.Ptr && !fieldValue.IsNil() {
					fieldValue = fieldValue.Elem()
				}

				switch fieldValue.Kind() {
				case reflect.String:
					csvRow = append(csvRow, fieldValue.String())
				case reflect.Int, reflect.Int64:
					csvRow = append(csvRow, fmt.Sprintf("%d", fieldValue.Int()))
				case reflect.Float64:
					csvRow = append(csvRow, fmt.Sprintf("%f", fieldValue.Float()))
				case reflect.Struct:
					if fieldValue.Type() == reflect.TypeOf(time.Time{}) {
						csvRow = append(csvRow, fieldValue.Interface().(time.Time).Format(fun.T_YYYYMMDD_HHmmss))
					}
				default:
					csvRow = append(csvRow, fmt.Sprintf("%v", fieldValue.Interface()))
				}
			}
			writer.Write(csvRow)
		}
	}
}
