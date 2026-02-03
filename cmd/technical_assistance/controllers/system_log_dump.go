package controllers

import (
	"fmt"
	"net/http"
	"os"
	"strings"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

func GetSystemLogFileDump(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		// Set the headers for the CSV download
		c.Header("Content-Description", "File Transfer")
		c.Header("Content-Disposition", "attachment; filename=SystemLog.csv")
		c.Header("Content-Type", "text/csv")

		// Get the file name from the query parameters
		fileToRead := c.Query("v")
		if fileToRead == "" {
			fileToRead = "apps.log"
		}
		fileToRead = strings.ReplaceAll(fileToRead, "/", "")
		fileToRead = strings.ReplaceAll(fileToRead, "..", "")
		fileToRead = strings.ReplaceAll(fileToRead, "\\", "")

		// Construct the file path
		filePath := os.Getenv("APP_LOG_DIR") + "/" + fileToRead

		// Open the file
		file, err := os.Open(filePath)
		if err != nil {
			fmt.Println("Error opening file:", err)
			c.JSON(http.StatusInternalServerError, gin.H{
				"error": "Failed to open file",
			})
			return
		}
		defer file.Close()

		// Serve the file
		c.File(filePath)
	}
}
