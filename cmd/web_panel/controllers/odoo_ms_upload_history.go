package controllers

import (
	"html"
	"net/http"
	odooms "service-platform/cmd/web_panel/model/odoo_ms"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/sirupsen/logrus"
	"gorm.io/gorm"
)

// GetUploadHistoryByEmail gets all upload history for a specific user email
func GetUploadHistoryByEmail(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		email := c.Query("email")
		if email == "" {
			c.JSON(http.StatusBadRequest, gin.H{
				"status":  "error",
				"message": "Email parameter is required",
			})
			return
		}

		page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
		if page < 1 {
			page = 1
		}

		limit, _ := strconv.Atoi(c.DefaultQuery("limit", "10"))
		if limit < 1 || limit > 100 {
			limit = 10
		}

		offset := (page - 1) * limit

		var uploads []odooms.UploadedExcelToODOOMS
		var total int64

		// Get total count - include soft deleted
		if err := db.Unscoped().Model(&odooms.UploadedExcelToODOOMS{}).Where("email = ?", email).Count(&total).Error; err != nil {
			logrus.Errorf("Failed to count upload history: %v", err)
			c.JSON(http.StatusInternalServerError, gin.H{
				"status":  "error",
				"message": "Failed to retrieve upload count",
			})
			return
		}

		// Get paginated results ordered by creation date (newest first) - include soft deleted
		if err := db.Unscoped().Where("email = ?", email).
			Order("created_at DESC").
			Limit(limit).
			Offset(offset).
			Find(&uploads).Error; err != nil {
			logrus.Errorf("Failed to get upload history: %v", err)
			c.JSON(http.StatusInternalServerError, gin.H{
				"status":  "error",
				"message": "Failed to retrieve upload history",
			})
			return
		}

		// Calculate pagination info
		totalPages := int((total + int64(limit) - 1) / int64(limit))
		hasNext := page < totalPages
		hasPrev := page > 1

		c.JSON(http.StatusOK, gin.H{
			"status": "success",
			"data": gin.H{
				"uploads": uploads,
				"pagination": gin.H{
					"current_page": page,
					"total_pages":  totalPages,
					"total_items":  total,
					"per_page":     limit,
					"has_next":     hasNext,
					"has_prev":     hasPrev,
				},
			},
		})
	}
}

// GetUploadHistoryTable returns HTML table data for DataTables
func GetUploadHistoryTable(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		// For POST requests, try to get email from form data first, then query
		email := c.PostForm("email")
		if email == "" {
			email = c.Query("email")
		}
		if email == "" {
			c.JSON(http.StatusBadRequest, gin.H{
				"error": "Email parameter is required",
			})
			return
		}

		// DataTables parameters - check both POST form and query parameters
		draw, _ := strconv.Atoi(c.DefaultPostForm("draw", c.DefaultQuery("draw", "1")))
		start, _ := strconv.Atoi(c.DefaultPostForm("start", c.DefaultQuery("start", "0")))
		length, _ := strconv.Atoi(c.DefaultPostForm("length", c.DefaultQuery("length", "10")))

		// Search parameter - check both POST form and query parameters
		searchValue := c.DefaultPostForm("search[value]", c.DefaultQuery("search[value]", ""))

		// Handle ordering
		orderColumn := c.DefaultPostForm("order[0][column]", "5") // Default to column 5 (created_at)
		orderDir := c.DefaultPostForm("order[0][dir]", "desc")

		// Map column numbers to database columns
		columnMap := map[string]string{
			"0": "id",
			"1": "ori_filename",
			"2": "template",
			"3": "status",
			"4": "total_success", // Progress column
			"5": "created_at",
			"6": "complete_time",
			"7": "id", // Actions column - default to id
		}

		orderColumnName, exists := columnMap[orderColumn]
		if !exists {
			orderColumnName = "created_at"
		}

		// Ensure order direction is valid
		if orderDir != "asc" && orderDir != "desc" {
			orderDir = "desc"
		}

		orderClause := orderColumnName + " " + strings.ToUpper(orderDir)

		var uploads []odooms.UploadedExcelToODOOMS
		var filteredRecords int64
		var totalRecords int64

		// Get total records count - include soft deleted
		if err := db.Unscoped().Model(&odooms.UploadedExcelToODOOMS{}).Where("email = ?", email).Count(&totalRecords).Error; err != nil {
			logrus.Errorf("Failed to count total records: %v", err)
			c.JSON(http.StatusInternalServerError, gin.H{
				"error": "Failed to count total records",
			})
			return
		}

		// Build query - include soft deleted
		query := db.Unscoped().Model(&odooms.UploadedExcelToODOOMS{}).Where("email = ?", email)

		// Apply search filter if provided
		if searchValue != "" {
			searchPattern := "%" + searchValue + "%"
			query = query.Where("ori_filename LIKE ? OR status LIKE ? OR log LIKE ?",
				searchPattern, searchPattern, searchPattern)
		}

		// Get filtered count
		if err := query.Count(&filteredRecords).Error; err != nil {
			logrus.Errorf("Failed to count filtered records: %v", err)
			c.JSON(http.StatusInternalServerError, gin.H{
				"error": "Failed to count filtered records",
			})
			return
		}

		// Apply pagination and ordering
		query = query.Order(orderClause).Offset(start).Limit(length)

		// Execute query
		if err := query.Find(&uploads).Error; err != nil {
			logrus.Errorf("Failed to get upload history table: %v", err)
			c.JSON(http.StatusInternalServerError, gin.H{
				"error": "Failed to retrieve data",
			})
			return
		}

		// Transform data for DataTables
		var data [][]interface{}
		for i, upload := range uploads {
			// Format dates
			createdAt := upload.CreatedAt.Format("2006-01-02 15:04:05")
			completedAt := ""
			if upload.CompleteTime != nil {
				completedAt = upload.CompleteTime.Format("2006-01-02 15:04:05")
			}

			// Status badge HTML with deleted indicator
			statusClass := "secondary"
			statusText := upload.Status
			switch upload.Status {
			case "Completed":
				statusClass = "success"
			case "Failed":
				statusClass = "danger"
			case "Processing":
				statusClass = "info"
			case "Pending":
				statusClass = "warning"
			}

			// Add deleted indicator if soft deleted
			if upload.DeletedAt.Valid {
				statusText += " (Deleted)"
				statusClass = "dark"
			}

			statusBadge := `<span class="badge bg-` + statusClass + `">` + statusText + `</span>`

			// Progress info
			progressText := ""
			if upload.TotalRow > 0 {
				successRate := float64(upload.TotalSuccess) / float64(upload.TotalRow) * 100
				progressText = strconv.Itoa(upload.TotalSuccess) + "/" + strconv.Itoa(upload.TotalRow) +
					" (" + strconv.FormatFloat(successRate, 'f', 1, 64) + "%)"
			}

			// Safely escape logs data for HTML attribute
			escapedLogs := html.EscapeString(upload.Logs)
			// Replace any remaining quotes to prevent breaking HTML attributes
			escapedLogs = strings.ReplaceAll(escapedLogs, `"`, `&quot;`)
			escapedLogs = strings.ReplaceAll(escapedLogs, `'`, `&#39;`)

			// Action buttons
			actions := `<div class="btn-group" role="group">
				<button type="button" class="btn btn-sm btn-outline-info view-logs" 
					data-id="` + strconv.Itoa(int(upload.ID)) + `" 
					data-filename="` + html.EscapeString(upload.OriginalFilename) + `" 
					data-logs="` + escapedLogs + `">
					<i class="fal fa-info-square me-2"></i> Logs
				</button>
			</div>`

			row := []interface{}{
				start + i + 1,           // Row number
				upload.OriginalFilename, // Original filename
				upload.Template,         // Template
				statusBadge,             // Status
				progressText,            // Progress
				createdAt,               // Created at
				completedAt,             // Completed at
				actions,                 // Actions
			}
			data = append(data, row)
		}

		// Return DataTables format
		c.JSON(http.StatusOK, gin.H{
			"draw":            draw,
			"recordsTotal":    totalRecords,
			"recordsFiltered": filteredRecords,
			"data":            data,
		})
	}
}

// GetUploadDetails gets detailed information about a specific upload
func GetUploadDetails(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		idStr := c.Param("id")
		id, err := strconv.ParseUint(idStr, 10, 32)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{
				"status":  "error",
				"message": "Invalid upload ID",
			})
			return
		}

		var upload odooms.UploadedExcelToODOOMS
		if err := db.Unscoped().First(&upload, uint(id)).Error; err != nil {
			if err == gorm.ErrRecordNotFound {
				c.JSON(http.StatusNotFound, gin.H{
					"status":  "error",
					"message": "Upload not found",
				})
			} else {
				logrus.Errorf("Failed to get upload details: %v", err)
				c.JSON(http.StatusInternalServerError, gin.H{
					"status":  "error",
					"message": "Failed to retrieve upload details",
				})
			}
			return
		}

		c.JSON(http.StatusOK, gin.H{
			"status": "success",
			"data":   upload,
		})
	}
}
