package controllers

import (
	"encoding/csv"
	"net/http"
	"service-platform/cmd/web_panel/model"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

func GetActivityLog(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		var logActivities []model.LogActivity
		result := db.Find(&logActivities)

		if result.Error != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to retrieve Data " + result.Error.Error()})
			return
		}

		if len(logActivities) == 0 {
			c.JSON(http.StatusOK, gin.H{
				"data": []gin.H{},
			})
			return
		}

		// Loop through the lines and process them as needed
		var data []gin.H
		for _, logActivity := range logActivities {
			createdAtString := logActivity.CreatedAt.Format("2006-01-02 15:04:05")
			data = append(data, gin.H{
				"date_time": createdAtString,
				"action":    logActivity.Action,
				"fullname":  logActivity.FullName,
				"status":    logActivity.Status,
				"detail":    logActivity.Log,
			})
		}

		// Respond with the formatted data
		c.JSON(http.StatusOK, gin.H{
			"data": data,
		})
	}
}
func DumpActivityLog(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		// Set the headers for the CSV download
		c.Header("Content-Description", "File Transfer")
		c.Header("Content-Disposition", "attachment; filename=activity_log.csv")
		c.Header("Content-Type", "text/csv")

		// Create a CSV writer
		writer := csv.NewWriter(c.Writer)
		defer writer.Flush()

		// Write CSV headers
		writer.Write([]string{
			"Date/Time", "Action", "Full Name", "Status", "Detail",
		})

		// Fetch log activities from the database
		var logActivities []model.LogActivity
		if err := db.Find(&logActivities).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to retrieve data: " + err.Error()})
			return
		}

		// Check if there are any log activities
		if len(logActivities) == 0 {
			c.JSON(http.StatusOK, gin.H{"data": []gin.H{}})
			return
		}

		// Write log activity data to the CSV
		for _, logActivity := range logActivities {
			createdAtString := logActivity.CreatedAt.Format("2006-01-02 15:04:05")
			writer.Write([]string{
				createdAtString,
				logActivity.Action,
				logActivity.FullName,
				logActivity.Status,
				logActivity.Log,
			})
		}
	}
}
