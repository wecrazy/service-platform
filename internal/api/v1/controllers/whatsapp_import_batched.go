package controllers

import (
	"encoding/csv"
	"encoding/json"
	"fmt"
	"net/http"
	"regexp"
	"service-platform/internal/core/model"
	"service-platform/internal/whatsapp"
	pb "service-platform/proto"
	"strconv"
	"strings"
	"sync"

	"github.com/gin-gonic/gin"
	"github.com/sirupsen/logrus"
	"gorm.io/gorm"
)

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
			c.JSON(http.StatusBadRequest, gin.H{
				"success": false,
				"message": "No file uploaded",
			})
			return
		}

		// Open the uploaded file
		src, err := file.Open()
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{
				"success": false,
				"message": "Failed to open file",
			})
			return
		}
		defer src.Close()

		// Parse CSV
		csvReader := csv.NewReader(src)
		records, err := csvReader.ReadAll()
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{
				"success": false,
				"message": "Failed to parse CSV file",
			})
			return
		}

		if len(records) < 2 {
			c.JSON(http.StatusBadRequest, gin.H{
				"success": false,
				"message": "CSV file is empty or has no data rows",
			})
			return
		}

		// Check if gRPC client is available
		if whatsapp.Client == nil {
			logrus.Warn("WhatsApp gRPC client not available")
		}

		ctx := c.Request.Context()
		dataRows := records[1:] // Skip header

		// Batching configuration
		const batchSize = 100
		const numWorkers = 10

		// Channels for job distribution and result collection
		type rowJob struct {
			index  int
			record []string
		}

		type rowResult struct {
			status  string
			rowNum  int
			message string
		}

		jobs := make(chan rowJob, batchSize)
		results := make(chan rowResult, len(dataRows))

		var wg sync.WaitGroup

		// Worker function
		worker := func(_ int) {
			defer wg.Done()
			phoneRegex := regexp.MustCompile(`\D`)

			for job := range jobs {
				rowNum := job.index + 2
				record := job.record

				// Process row inline
				result := func() rowResult {
					// Validate columns
					if len(record) < 3 {
						return rowResult{status: "failed", rowNum: rowNum, message: "insufficient columns"}
					}

					fullName := strings.TrimSpace(record[0])
					email := strings.TrimSpace(record[1])
					phoneNumber := strings.TrimSpace(record[2])

					// Validate required fields
					if fullName == "" || email == "" || phoneNumber == "" {
						return rowResult{status: "failed", rowNum: rowNum, message: "missing required fields"}
					}

					// Format phone number
					phoneNumber = phoneRegex.ReplaceAllString(phoneNumber, "")
					if phoneNumber == "" {
						return rowResult{status: "failed", rowNum: rowNum, message: "invalid phone format"}
					}

					// Handle Indonesian formats
					if strings.HasPrefix(phoneNumber, "0") {
						phoneNumber = "62" + phoneNumber[1:]
					} else if strings.HasPrefix(phoneNumber, "8") {
						phoneNumber = "62" + phoneNumber
					} else if !strings.HasPrefix(phoneNumber, "62") {
						phoneNumber = "62" + phoneNumber
					}

					// Validate with WhatsApp
					var validatedPhone string
					if whatsapp.Client != nil {
						waResp, err := whatsapp.Client.IsOnWhatsApp(ctx, &pb.IsOnWhatsAppRequest{
							PhoneNumbers: []string{phoneNumber},
						})

						if err != nil || !waResp.Success || len(waResp.Results) == 0 || !waResp.Results[0].IsRegistered {
							return rowResult{status: "failed", rowNum: rowNum, message: fmt.Sprintf("not on WhatsApp: %s", phoneNumber)}
						}

						jidParts := strings.Split(waResp.Results[0].Jid, "@")
						if len(jidParts) > 0 {
							validatedPhone = jidParts[0]
						} else {
							validatedPhone = phoneNumber
						}
					} else {
						validatedPhone = phoneNumber
					}

					// Build user object
					user := model.WAUsers{
						FullName:     fullName,
						Email:        email,
						PhoneNumber:  validatedPhone,
						IsRegistered: true,
					}

					// Parse optional fields
					if len(record) > 3 && strings.TrimSpace(record[3]) != "" {
						user.UserType = model.WAUserType(strings.TrimSpace(record[3]))
					} else {
						user.UserType = model.CommonUser
					}

					if len(record) > 4 && strings.TrimSpace(record[4]) != "" {
						user.UserOf = model.WAUserOf(strings.TrimSpace(record[4]))
					} else {
						user.UserOf = model.CompanyEmployee
					}

					if len(record) > 5 && strings.TrimSpace(record[5]) != "" {
						user.AllowedChats = model.WAAllowedChatMode(strings.TrimSpace(record[5]))
					} else {
						user.AllowedChats = model.BothChat
					}

					if len(record) > 6 && strings.TrimSpace(record[6]) != "" {
						if quota, parseErr := strconv.Atoi(strings.TrimSpace(record[6])); parseErr == nil {
							user.MaxDailyQuota = quota
						}
					} else {
						user.MaxDailyQuota = 10
					}

					if len(record) > 7 && strings.TrimSpace(record[7]) != "" {
						user.AllowedToCall = strings.ToLower(strings.TrimSpace(record[7])) == "true"
					}

					if len(record) > 8 && strings.TrimSpace(record[8]) != "" {
						user.UseBot = strings.ToLower(strings.TrimSpace(record[8])) == "true"
					} else {
						user.UseBot = true
					}

					// Handle AllowedTypes as JSON
					if len(record) > 9 && strings.TrimSpace(record[9]) != "" {
						allowedTypesStr := strings.TrimSpace(record[9])
						types := strings.Split(strings.ReplaceAll(allowedTypesStr, "|", ","), ",")
						for i := range types {
							types[i] = strings.TrimSpace(types[i])
						}
						if typesJSON, jsonErr := json.Marshal(types); jsonErr == nil {
							user.AllowedTypes = typesJSON
						} else {
							defaultTypes, _ := json.Marshal([]string{"text"})
							user.AllowedTypes = defaultTypes
						}
					} else {
						defaultTypes, _ := json.Marshal([]string{"text"})
						user.AllowedTypes = defaultTypes
					}

					if len(record) > 10 {
						user.Description = strings.TrimSpace(record[10])
					}

					// Check if exists and create or update
					var existingUser model.WAUsers
					dbErr := db.Where("phone_number = ?", validatedPhone).First(&existingUser).Error

					switch dbErr {
					case gorm.ErrRecordNotFound:
						if createErr := db.Create(&user).Error; createErr != nil {
							return rowResult{status: "failed", rowNum: rowNum, message: fmt.Sprintf("create failed: %v", createErr)}
						}
						return rowResult{status: "created", rowNum: rowNum}
					case nil:
						user.ID = existingUser.ID
						if updateErr := db.Model(&existingUser).Updates(&user).Error; updateErr != nil {
							return rowResult{status: "failed", rowNum: rowNum, message: fmt.Sprintf("update failed: %v", updateErr)}
						}
						return rowResult{status: "updated", rowNum: rowNum}
					default:
						return rowResult{status: "failed", rowNum: rowNum, message: fmt.Sprintf("database error: %v", dbErr)}
					}
				}()

				results <- result
			}
		}

		// Start workers
		for w := 1; w <= numWorkers; w++ {
			wg.Add(1)
			go worker(w)
		}

		// Send jobs
		go func() {
			for i, record := range dataRows {
				jobs <- rowJob{index: i, record: record}
			}
			close(jobs)
		}()

		// Wait for all workers to finish and close results
		go func() {
			wg.Wait()
			close(results)
		}()

		// Collect results
		created := 0
		updated := 0
		failed := 0

		for result := range results {
			switch result.status {
			case "created":
				created++
			case "updated":
				updated++
			case "failed":
				failed++
				if result.message != "" {
					logrus.Warnf("Row %d: %s", result.rowNum, result.message)
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
