package controllers

import (
	"bytes"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"service-platform/cmd/web_panel/fun"
	"service-platform/cmd/web_panel/internal/gormdb"
	odooms "service-platform/cmd/web_panel/model/odoo_ms"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/go-redis/redis/v8"
	"github.com/sirupsen/logrus"
	"gorm.io/gorm"
)

type ReadCloser struct {
	*bytes.Reader
}

func (rc *ReadCloser) Close() error {
	// No real closing needed, just return nil
	return nil
}

func UploadMustUpdatedTicket(db *gorm.DB, redisDB *redis.Client) gin.HandlerFunc {
	return func(c *gin.Context) {
		templateIDStr := c.PostForm("templateId")
		emailUploadedBy := c.PostForm("uploadedBy")
		password := c.PostForm("password")

		if password == "" {
			c.JSON(http.StatusBadRequest, gin.H{
				"status":  "error",
				"message": "Password cannot be empty!",
			})
			return
		}

		hashPwd, err := fun.GetAESEncrypted(password)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{
				"status":  "error",
				"message": err.Error(),
			})
			return
		}

		if emailUploadedBy == "" {
			c.JSON(http.StatusBadRequest, gin.H{
				"status":  "error",
				"message": "Unknown uploader!",
			})
			return
		}

		templateID, err := strconv.Atoi(templateIDStr)
		if err != nil || templateID == 0 {
			c.JSON(http.StatusBadRequest, gin.H{
				"status":  "error",
				"message": "Invalid template ID",
			})
			return
		}

		file, header, err := c.Request.FormFile("file")
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"status": "error", "message": "Failed to retrieve file"})
			return
		}
		defer file.Close()

		// Read the file into memory
		fileBytes, err := io.ReadAll(file)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"status": "error", "message": "Failed to read file"})
			return
		}

		// Get the filename and extension
		filename := header.Filename
		ext := filepath.Ext(filename)
		if ext != ".xlsx" && ext != ".xls" {
			c.JSON(http.StatusBadRequest, gin.H{"status": "error", "message": "Invalid file type"})
			return
		}

		// Create a new reader wrapped in ReadCloser
		reader := &ReadCloser{bytes.NewReader(fileBytes)}

		// Validate the Excel file
		valid, err := fun.ValidateExcelFile(reader, filename)
		if !valid {
			c.JSON(http.StatusBadRequest, gin.H{"status": "error", "message": err.Error()})
			return
		}

		// Reset the reader (so it can be read again)
		reader.Seek(0, io.SeekStart)

		// Get total row count
		totalRows, err := fun.SheetTotalLenRow(reader)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"status": "error", "message": err.Error()})
			return
		}

		if totalRows == 0 {
			c.JSON(http.StatusBadRequest, gin.H{"status": "error", "message": fmt.Sprintf("No data found in imported Excel: %v", filename)})
			return
		}

		// Define the file storage path
		oriFilename := filename
		filename = fun.GenerateRandomString(50) + ext // random filename
		selectedMainDir, err := fun.FindValidDirectory([]string{
			"web/file/uploaded_excel_to_odoo_ms",
			"../web/file/uploaded_excel_to_odoo_ms",
			"../../web/file/uploaded_excel_to_odoo_ms",
		})
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"status": "error", "message": "Failed to find valid directory: " + err.Error()})
			return
		}
		savePath := filepath.Join(selectedMainDir, filename)

		// Save the file
		if err := os.WriteFile(savePath, fileBytes, 0644); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"status": "error", "message": "Failed to save file"})
			return
		}
		// Store metadata in Redis (Optional)
		if redisDB != nil {
			err := redisDB.Set(c, "last_uploaded_file", savePath, 24*time.Hour).Err()
			if err != nil {
				logrus.Errorf("Failed to set last uploaded file in Redis: %v", err)
			}
		}

		importRecord := odooms.UploadedExcelToODOOMS{
			Email:            emailUploadedBy,
			Password:         hashPwd,
			OriginalFilename: oriFilename,
			Filename:         filename,
			Status:           "Pending",
			Template:         templateID,
			TotalRow:         totalRows,
		}

		if err := db.Create(&importRecord).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"status": "error", "message": "Database error: " + err.Error()})
			return
		}

		// Non-blocking trigger send
		select {
		case TriggerProcessUploadedExcelforUpdateTicketODOOMS <- struct{}{}:
			logrus.Info("Processing trigger sent successfully")
		default:
			logrus.Warn("Processing trigger channel is full, but file is queued")
		}

		c.JSON(http.StatusOK, gin.H{
			"status":  "success",
			"message": "File uploaded successfully and queued for processing",
			"path":    savePath,
		})
	}
}

func UploadODOOMSNewTicket(redisDB *redis.Client) gin.HandlerFunc {
	return func(c *gin.Context) {
		templateIDStr := c.PostForm("templateId")
		emailUploadedBy := c.PostForm("uploadedBy")
		password := c.PostForm("password")

		if password == "" {
			c.JSON(http.StatusBadRequest, gin.H{
				"status":  "error",
				"message": "Password cannot be empty!",
			})
			return
		}

		hashPwd, err := fun.GetAESEncrypted(password)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{
				"status":  "error",
				"message": err.Error(),
			})
			return
		}

		if emailUploadedBy == "" {
			c.JSON(http.StatusBadRequest, gin.H{
				"status":  "error",
				"message": "Unknown uploader!",
			})
			return
		}

		templateID, err := strconv.Atoi(templateIDStr)
		if err != nil || templateID == 0 {
			c.JSON(http.StatusBadRequest, gin.H{
				"status":  "error",
				"message": "Invalid template ID",
			})
			return
		}

		file, header, err := c.Request.FormFile("file")
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"status": "error", "message": "Failed to retrieve file"})
			return
		}
		defer file.Close()

		// Read the file into memory
		fileBytes, err := io.ReadAll(file)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"status": "error", "message": "Failed to read file"})
			return
		}

		// Get the filename and extension
		filename := header.Filename
		ext := filepath.Ext(filename)
		if ext != ".xlsx" && ext != ".xls" {
			c.JSON(http.StatusBadRequest, gin.H{"status": "error", "message": "Invalid file type"})
			return
		}

		// Create a new reader wrapped in ReadCloser
		reader := &ReadCloser{bytes.NewReader(fileBytes)}

		// Validate the Excel file
		valid, err := fun.ValidateExcelFile(reader, filename)
		if !valid {
			c.JSON(http.StatusBadRequest, gin.H{"status": "error", "message": err.Error()})
			return
		}

		// Reset the reader (so it can be read again)
		reader.Seek(0, io.SeekStart)

		// Get total row count
		totalRows, err := fun.SheetTotalLenRow(reader)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"status": "error", "message": err.Error()})
			return
		}

		if totalRows == 0 {
			c.JSON(http.StatusBadRequest, gin.H{"status": "error", "message": fmt.Sprintf("No data found in imported Excel: %v", filename)})
			return
		}

		// Define the file storage path
		oriFilename := filename
		filename = fun.GenerateRandomString(50) + ext // random filename
		selectedMainDir, err := fun.FindValidDirectory([]string{
			"web/file/uploaded_excel_to_odoo_ms",
			"../web/file/uploaded_excel_to_odoo_ms",
			"../../web/file/uploaded_excel_to_odoo_ms",
		})
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"status": "error", "message": "Failed to find valid directory: " + err.Error()})
			return
		}
		savePath := filepath.Join(selectedMainDir, filename)

		// Save the file
		if err := os.WriteFile(savePath, fileBytes, 0644); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"status": "error", "message": "Failed to save file"})
			return
		}
		// Store metadata in Redis (Optional)
		if redisDB != nil {
			err := redisDB.Set(c, "last_uploaded_file", savePath, 24*time.Hour).Err()
			if err != nil {
				logrus.Errorf("Failed to set last uploaded file in Redis: %v", err)
			}
		}

		db := gormdb.Databases.Web

		importRecord := odooms.UploadedExcelToODOOMS{
			Email:            emailUploadedBy,
			Password:         hashPwd,
			OriginalFilename: oriFilename,
			Filename:         filename,
			Status:           "Pending",
			Template:         templateID,
			TotalRow:         totalRows,
		}

		if err := db.Create(&importRecord).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"status": "error", "message": "Database error: " + err.Error()})
			return
		}

		// Non-blocking trigger send
		select {
		case TriggerProcessUploadExcelforCreateNewTicketODOOMS <- struct{}{}:
			logrus.Info("Processing trigger sent successfully")
		default:
			logrus.Warn("Processing trigger channel is full, but file is queued")
		}

		c.JSON(http.StatusOK, gin.H{
			"status":  "success",
			"message": "File uploaded successfully and queued for processing",
			"path":    savePath,
		})
	}
}

func UploadCSNABALost(redisDB *redis.Client) gin.HandlerFunc {
	return func(c *gin.Context) {
		templateIDStr := c.PostForm("templateId")
		emailUploadedBy := c.PostForm("uploadedBy")
		password := c.PostForm("password")

		if password == "" {
			c.JSON(http.StatusBadRequest, gin.H{
				"status":  "error",
				"message": "Password cannot be empty!",
			})
			return
		}

		hashPwd, err := fun.GetAESEncrypted(password)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{
				"status":  "error",
				"message": err.Error(),
			})
			return
		}

		if emailUploadedBy == "" {
			c.JSON(http.StatusBadRequest, gin.H{
				"status":  "error",
				"message": "Unknown uploader!",
			})
			return
		}

		templateID, err := strconv.Atoi(templateIDStr)
		if err != nil || templateID == 0 {
			c.JSON(http.StatusBadRequest, gin.H{
				"status":  "error",
				"message": "Invalid template ID",
			})
			return
		}

		file, header, err := c.Request.FormFile("file")
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"status": "error", "message": "Failed to retrieve file"})
			return
		}
		defer file.Close()

		// Read the file into memory
		fileBytes, err := io.ReadAll(file)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"status": "error", "message": "Failed to read file"})
			return
		}

		// Get the filename and extension
		filename := header.Filename
		ext := filepath.Ext(filename)
		if ext != ".xlsx" && ext != ".xls" {
			c.JSON(http.StatusBadRequest, gin.H{"status": "error", "message": "Invalid file type"})
			return
		}

		// Create a new reader wrapped in ReadCloser
		reader := &ReadCloser{bytes.NewReader(fileBytes)}

		// Validate the Excel file
		valid, err := fun.ValidateExcelFile(reader, filename)
		if !valid {
			c.JSON(http.StatusBadRequest, gin.H{"status": "error", "message": err.Error()})
			return
		}

		// Reset the reader (so it can be read again)
		reader.Seek(0, io.SeekStart)

		// Get total row count
		totalRows, err := fun.SheetTotalLenRow(reader)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"status": "error", "message": err.Error()})
			return
		}

		if totalRows == 0 {
			c.JSON(http.StatusBadRequest, gin.H{"status": "error", "message": fmt.Sprintf("No data found in imported Excel: %v", filename)})
			return
		}

		// Define the file storage path
		oriFilename := filename
		filename = fun.GenerateRandomString(50) + ext // random filename
		selectedMainDir, err := fun.FindValidDirectory([]string{
			"web/file/uploaded_excel_to_odoo_ms",
			"../web/file/uploaded_excel_to_odoo_ms",
			"../../web/file/uploaded_excel_to_odoo_ms",
		})
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"status": "error", "message": "Failed to find valid directory: " + err.Error()})
			return
		}
		savePath := filepath.Join(selectedMainDir, filename)

		// Save the file
		if err := os.WriteFile(savePath, fileBytes, 0644); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"status": "error", "message": "Failed to save file"})
			return
		}
		// Store metadata in Redis (Optional)
		if redisDB != nil {
			err := redisDB.Set(c, "last_uploaded_file", savePath, 24*time.Hour).Err()
			if err != nil {
				logrus.Errorf("Failed to set last uploaded file in Redis: %v", err)
			}
		}

		db := gormdb.Databases.Web

		importRecord := odooms.UploadedExcelToODOOMS{
			Email:            emailUploadedBy,
			Password:         hashPwd,
			OriginalFilename: oriFilename,
			Filename:         filename,
			Status:           "Pending",
			Template:         templateID,
			TotalRow:         totalRows,
		}

		if err := db.Create(&importRecord).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"status": "error", "message": "Database error: " + err.Error()})
			return
		}

		// Non-blocking trigger send
		select {
		case TriggerProcessUploadExcelforCreateDataCSNABALost <- struct{}{}:
			logrus.Info("Processing trigger sent successfully")
		default:
			logrus.Warn("Processing trigger channel is full, but file is queued")
		}

		c.JSON(http.StatusOK, gin.H{
			"status":  "success",
			"message": "File uploaded successfully and queued for processing",
			"path":    savePath,
		})
	}
}

func UploadTechnicianPayrollIntoPayslip(redisDB *redis.Client) gin.HandlerFunc {
	return func(c *gin.Context) {
		templateIDStr := c.PostForm("templateId")
		emailUploadedBy := c.PostForm("uploadedBy")
		password := c.PostForm("password")

		if password == "" {
			c.JSON(http.StatusBadRequest, gin.H{
				"status":  "error",
				"message": "Password cannot be empty!",
			})
			return
		}

		hashPwd, err := fun.GetAESEncrypted(password)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{
				"status":  "error",
				"message": err.Error(),
			})
			return
		}

		if emailUploadedBy == "" {
			c.JSON(http.StatusBadRequest, gin.H{
				"status":  "error",
				"message": "Unknown uploader!",
			})
			return
		}

		templateID, err := strconv.Atoi(templateIDStr)
		if err != nil || templateID == 0 {
			c.JSON(http.StatusBadRequest, gin.H{
				"status":  "error",
				"message": "Invalid template ID",
			})
			return
		}

		file, header, err := c.Request.FormFile("file")
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"status": "error", "message": "Failed to retrieve file"})
			return
		}
		defer file.Close()

		// Read the file into memory
		fileBytes, err := io.ReadAll(file)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"status": "error", "message": "Failed to read file"})
			return
		}

		// Get the filename and extension
		filename := header.Filename
		ext := filepath.Ext(filename)
		if ext != ".xlsx" && ext != ".xls" {
			c.JSON(http.StatusBadRequest, gin.H{"status": "error", "message": "Invalid file type"})
			return
		}

		// Create a new reader wrapped in ReadCloser
		reader := &ReadCloser{bytes.NewReader(fileBytes)}

		// Validate the Excel file
		valid, err := fun.ValidateExcelFile(reader, filename)
		if !valid {
			c.JSON(http.StatusBadRequest, gin.H{"status": "error", "message": err.Error()})
			return
		}

		// Reset the reader (so it can be read again)
		reader.Seek(0, io.SeekStart)

		// Get total row count
		totalRows, err := fun.SheetTotalLenRow(reader)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"status": "error", "message": err.Error()})
			return
		}

		if totalRows == 0 {
			c.JSON(http.StatusBadRequest, gin.H{"status": "error", "message": fmt.Sprintf("No data found in imported Excel: %v", filename)})
			return
		}

		// Define the file storage path
		oriFilename := filename
		filename = fun.GenerateRandomString(50) + ext // random filename
		selectedMainDir, err := fun.FindValidDirectory([]string{
			"web/file/uploaded_excel_to_odoo_ms",
			"../web/file/uploaded_excel_to_odoo_ms",
			"../../web/file/uploaded_excel_to_odoo_ms",
		})
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"status": "error", "message": "Failed to find valid directory: " + err.Error()})
			return
		}
		savePath := filepath.Join(selectedMainDir, filename)

		// Save the file
		if err := os.WriteFile(savePath, fileBytes, 0644); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"status": "error", "message": "Failed to save file"})
			return
		}
		// Store metadata in Redis (Optional)
		if redisDB != nil {
			err := redisDB.Set(c, "last_uploaded_file", savePath, 24*time.Hour).Err()
			if err != nil {
				logrus.Errorf("Failed to set last uploaded file in Redis: %v", err)
			}
		}

		db := gormdb.Databases.Web

		importRecord := odooms.UploadedExcelToODOOMS{
			Email:            emailUploadedBy,
			Password:         hashPwd,
			OriginalFilename: oriFilename,
			Filename:         filename,
			Status:           "Pending",
			Template:         templateID,
			TotalRow:         totalRows,
		}

		if err := db.Create(&importRecord).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"status": "error", "message": "Database error: " + err.Error()})
			return
		}

		// Non-blocking trigger send
		select {
		case TriggerProcessUploadExcelforCreateTechnicianPayslip <- struct{}{}:
			logrus.Info("Processing trigger sent successfully")
		default:
			logrus.Warn("Processing trigger channel is full, but file is queued")
		}

		c.JSON(http.StatusOK, gin.H{
			"status":  "success",
			"message": "File uploaded successfully and queued for processing",
			"path":    savePath,
		})
	}
}

func UploadMustUpdatedTask(db *gorm.DB, redisDB *redis.Client) gin.HandlerFunc {
	return func(c *gin.Context) {
		templateIDStr := c.PostForm("templateId")
		emailUploadedBy := c.PostForm("uploadedBy")
		password := c.PostForm("password")

		if password == "" {
			c.JSON(http.StatusBadRequest, gin.H{
				"status":  "error",
				"message": "Password cannot be empty!",
			})
			return
		}

		hashPwd, err := fun.GetAESEncrypted(password)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{
				"status":  "error",
				"message": err.Error(),
			})
			return
		}

		if emailUploadedBy == "" {
			c.JSON(http.StatusBadRequest, gin.H{
				"status":  "error",
				"message": "Unknown uploader!",
			})
			return
		}

		templateID, err := strconv.Atoi(templateIDStr)
		if err != nil || templateID == 0 {
			c.JSON(http.StatusBadRequest, gin.H{
				"status":  "error",
				"message": "Invalid template ID",
			})
			return
		}

		file, header, err := c.Request.FormFile("file")
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"status": "error", "message": "Failed to retrieve file"})
			return
		}
		defer file.Close()

		// Read the file into memory
		fileBytes, err := io.ReadAll(file)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"status": "error", "message": "Failed to read file"})
			return
		}

		// Get the filename and extension
		filename := header.Filename
		ext := filepath.Ext(filename)
		if ext != ".xlsx" && ext != ".xls" {
			c.JSON(http.StatusBadRequest, gin.H{"status": "error", "message": "Invalid file type"})
			return
		}

		// Create a new reader wrapped in ReadCloser
		reader := &ReadCloser{bytes.NewReader(fileBytes)}

		// Validate the Excel file
		valid, err := fun.ValidateExcelFile(reader, filename)
		if !valid {
			c.JSON(http.StatusBadRequest, gin.H{"status": "error", "message": err.Error()})
			return
		}

		// Reset the reader (so it can be read again)
		reader.Seek(0, io.SeekStart)

		// Get total row count
		totalRows, err := fun.SheetTotalLenRow(reader)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"status": "error", "message": err.Error()})
			return
		}

		if totalRows == 0 {
			c.JSON(http.StatusBadRequest, gin.H{"status": "error", "message": fmt.Sprintf("No data found in imported Excel: %v", filename)})
			return
		}

		// Define the file storage path
		oriFilename := filename
		filename = fun.GenerateRandomString(50) + ext // random filename
		selectedMainDir, err := fun.FindValidDirectory([]string{
			"web/file/uploaded_excel_to_odoo_ms",
			"../web/file/uploaded_excel_to_odoo_ms",
			"../../web/file/uploaded_excel_to_odoo_ms",
		})
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"status": "error", "message": "Failed to find valid directory: " + err.Error()})
			return
		}
		savePath := filepath.Join(selectedMainDir, filename)

		// Save the file
		if err := os.WriteFile(savePath, fileBytes, 0644); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"status": "error", "message": "Failed to save file"})
			return
		}
		// Store metadata in Redis (Optional)
		if redisDB != nil {
			err := redisDB.Set(c, "last_uploaded_file", savePath, 24*time.Hour).Err()
			if err != nil {
				logrus.Errorf("Failed to set last uploaded file in Redis: %v", err)
			}
		}

		importRecord := odooms.UploadedExcelToODOOMS{
			Email:            emailUploadedBy,
			Password:         hashPwd,
			OriginalFilename: oriFilename,
			Filename:         filename,
			Status:           "Pending",
			Template:         templateID,
			TotalRow:         totalRows,
		}

		if err := db.Create(&importRecord).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"status": "error", "message": "Database error: " + err.Error()})
			return
		}

		// Non-blocking trigger send
		select {
		case TriggerProcessUploadedExcelforUpdateTaskODOOMS <- struct{}{}:
			logrus.Info("Processing trigger sent successfully")
		default:
			logrus.Warn("Processing trigger channel is full, but file is queued")
		}

		c.JSON(http.StatusOK, gin.H{
			"status":  "success",
			"message": "File uploaded successfully and queued for processing",
			"path":    savePath,
		})
	}
}
