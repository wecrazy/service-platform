package controllers

import (
	"context"
	"encoding/csv"
	"fmt"
	"net/http"
	"regexp"
	"service-platform/internal/core/model"
	"service-platform/internal/whatsapp"
	pb "service-platform/proto"
	"strings"
	"sync"

	"github.com/gin-gonic/gin"
	"github.com/sirupsen/logrus"
	"gorm.io/gorm"
)

// batchRowResult represents the result of processing a single row in the WhatsApp user import batch. It includes the status of the operation (created, updated, failed), the row number for reference, and an optional message for failure cases to provide more context on what went wrong during processing.
type batchRowResult struct {
	status  string
	rowNum  int
	message string
}

// ImportWhatsAppUsersBatched godoc
// @Summary      Import WhatsApp Users from CSV with Batching
// @Description  Imports users from CSV file with concurrent batched processing for large files (100k+ rows)
// @Tags         WhatsApp User Management
// @Accept       multipart/form-data
// @Produce      json
// @Param        file formData file true "CSV file"
// @Success      200  {object}   map[string]interface{}
// @Router       /api/v1/{access}/tab-whatsapp-user-management/users/import-batched [post]
func ImportWhatsAppUsersBatched(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		file, err := c.FormFile("file")
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": "No file uploaded"})
			return
		}

		src, err := file.Open()
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"success": false, "message": "Failed to open file"})
			return
		}
		defer src.Close()

		records, err := csv.NewReader(src).ReadAll()
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": "Failed to parse CSV file"})
			return
		}
		if len(records) < 2 {
			c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": "CSV file is empty or has no data rows"})
			return
		}

		if whatsapp.Client == nil {
			logrus.Warn("WhatsApp gRPC client not available")
		}

		ctx := c.Request.Context()
		dataRows := records[1:]

		const batchSize = 100
		const numWorkers = 10

		type rowJob struct {
			index  int
			record []string
		}

		jobs := make(chan rowJob, batchSize)
		results := make(chan batchRowResult, len(dataRows))

		var wg sync.WaitGroup
		for w := 1; w <= numWorkers; w++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				re := regexp.MustCompile(`\D`)
				for job := range jobs {
					results <- processBatchedWAImportRow(ctx, db, job.record, job.index+2, re)
				}
			}()
		}

		go func() {
			for i, record := range dataRows {
				jobs <- rowJob{index: i, record: record}
			}
			close(jobs)
		}()
		go func() {
			wg.Wait()
			close(results)
		}()

		created, updated, failed := 0, 0, 0
		for res := range results {
			switch res.status {
			case "created":
				created++
			case "updated":
				updated++
			case "failed":
				failed++
				if res.message != "" {
					logrus.Warnf("Row %d: %s", res.rowNum, res.message)
				}
			}
		}

		c.JSON(http.StatusOK, gin.H{
			"success": true,
			"message": fmt.Sprintf("Import completed: %d created, %d updated, %d failed", created, updated, failed),
			"created": created,
			"updated": updated,
			"failed":  failed,
		})
	}
}

// processBatchedWAImportRow processes a single row of the WhatsApp user import CSV. It validates the data, checks if the phone number is registered on WhatsApp, and either creates a new user or updates an existing one in the database. It returns a batchRowResult indicating the outcome of the operation for that row.
func processBatchedWAImportRow(ctx context.Context, db *gorm.DB, record []string, rowNum int, phoneRegex *regexp.Regexp) batchRowResult {
	fail := func(msg string) batchRowResult { return batchRowResult{status: "failed", rowNum: rowNum, message: msg} }

	if len(record) < 3 {
		return fail("insufficient columns")
	}

	fullName := strings.TrimSpace(record[0])
	email := strings.TrimSpace(record[1])
	phoneNumber := strings.TrimSpace(record[2])
	if fullName == "" || email == "" || phoneNumber == "" {
		return fail("missing required fields")
	}

	validatedPhone, ok := normalizeBatchWAPhone(ctx, phoneNumber, phoneRegex)
	if !ok {
		return fail(fmt.Sprintf("not on WhatsApp: %s", phoneNumber))
	}

	user := buildWABatchedUser(record, validatedPhone, fullName, email)

	var existing model.WAUsers
	switch err := db.Where("phone_number = ?", validatedPhone).First(&existing).Error; err {
	case gorm.ErrRecordNotFound:
		if err := db.Create(&user).Error; err != nil {
			return fail(fmt.Sprintf("create failed: %v", err))
		}
		return batchRowResult{status: "created", rowNum: rowNum}
	case nil:
		user.ID = existing.ID
		if err := db.Model(&existing).Updates(&user).Error; err != nil {
			return fail(fmt.Sprintf("update failed: %v", err))
		}
		return batchRowResult{status: "updated", rowNum: rowNum}
	default:
		return fail(fmt.Sprintf("database error: %v", err))
	}
}

// normalizeBatchWAPhone normalizes phone format and validates it is on WhatsApp.
// Returns (validatedPhone, true) on success.
func normalizeBatchWAPhone(ctx context.Context, phoneNumber string, phoneRegex *regexp.Regexp) (string, bool) {
	phoneNumber = phoneRegex.ReplaceAllString(phoneNumber, "")
	if phoneNumber == "" {
		return "", false
	}
	switch {
	case strings.HasPrefix(phoneNumber, "0"):
		phoneNumber = "62" + phoneNumber[1:]
	case strings.HasPrefix(phoneNumber, "8"):
		phoneNumber = "62" + phoneNumber
	case !strings.HasPrefix(phoneNumber, "62"):
		phoneNumber = "62" + phoneNumber
	}

	if whatsapp.Client == nil {
		return phoneNumber, true
	}

	waResp, err := whatsapp.Client.IsOnWhatsApp(ctx, &pb.IsOnWhatsAppRequest{
		PhoneNumbers: []string{phoneNumber},
	})
	if err != nil || !waResp.Success || len(waResp.Results) == 0 || !waResp.Results[0].IsRegistered {
		return "", false
	}

	jidParts := strings.Split(waResp.Results[0].Jid, "@")
	if len(jidParts) > 0 {
		return jidParts[0], true
	}
	return phoneNumber, true
}

// buildWABatchedUser constructs a model.WAUsers from a CSV record (IsRegistered=true for batch imports).
func buildWABatchedUser(record []string, phone, fullName, email string) model.WAUsers {
	user := model.WAUsers{FullName: fullName, Email: email, PhoneNumber: phone, IsRegistered: true}
	applyWAUserOptionalFields(record, &user)
	return user
}
