package controllers

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"service-platform/cmd/technical_assistance/model"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

func SubmitCcFile(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		// Check custom header
		headerValue := c.GetHeader("Masuk")
		expectedHeaderValue := "WWYJR5TlPdoiIPyKCnGl4rkhlFD28GCAl5qibtukZGsOgJ5aF6H0XUfD0sIDHdZh"
		if headerValue != expectedHeaderValue {
			c.JSON(401, gin.H{"error": "Unauthorized: Invalid header value"})
			return
		}

		// Parse the uploaded file
		file, err := c.FormFile("file")
		if err != nil {
			c.JSON(400, gin.H{"error": fmt.Sprintf("Failed to parse file: %v", err)})
			return
		}

		// Open the uploaded file
		src, err := file.Open()
		if err != nil {
			c.JSON(500, gin.H{"error": fmt.Sprintf("Failed to open file: %v", err)})
			return
		}
		defer src.Close()

		// Generate the folder path based on the current date
		now := time.Now()
		folderPath := fmt.Sprintf("./uploads/%04d/%02d/%02d", now.Year(), now.Month(), now.Day())

		// Create the folder structure if it doesn't exist
		if err := os.MkdirAll(folderPath, os.ModePerm); err != nil {
			c.JSON(500, gin.H{"error": fmt.Sprintf("Failed to create folder: %v", err)})
			return
		}

		// Define the destination file path
		dstPath := filepath.Join(folderPath, file.Filename)

		// Create the destination file
		dst, err := os.Create(dstPath)
		if err != nil {
			c.JSON(500, gin.H{"error": fmt.Sprintf("Failed to create file: %v", err)})
			return
		}
		defer dst.Close()

		// Copy the file content to the destination
		if _, err := io.Copy(dst, src); err != nil {
			c.JSON(500, gin.H{"error": fmt.Sprintf("Failed to save file: %v", err)})
			return
		}

		// Save file metadata to the database
		uploadedFile := model.UploadedFiles{
			FileName: file.Filename,
			FilePath: dstPath,
			FileSize: file.Size,
			UserID:   1, // Replace with actual user ID if needed
		}
		if err := db.Create(&uploadedFile).Error; err != nil {
			c.JSON(500, gin.H{"error": fmt.Sprintf("Failed to save file metadata: %v", err)})
			return
		}

		c.JSON(200, gin.H{
			"message":  "File uploaded successfully",
			"file_id":  uploadedFile.ID,
			"file_url": dstPath,
		})
	}
}
