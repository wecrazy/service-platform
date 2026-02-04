package controllers

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"service-platform/cmd/technical_assistance/fun"
	"service-platform/cmd/technical_assistance/model"
	"service-platform/cmd/technical_assistance/model/op_model"
	"service-platform/internal/config"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/go-redis/redis/v8"
	"gorm.io/gorm"
)

type feedbackFromTA struct {
	Teknisi       string
	EmailTa       string
	NamaTA        string
	NomorTA       string
	Tabel         string
	Feedback      string
	WoNumber      string
	TicketSubject string
	Merchant      string
	MID           string
	TID           string
}

func TAFeedback(redisDB *redis.Client, db_pengerjaan *gorm.DB, db_web *gorm.DB) gin.HandlerFunc {
	return func(ctx *gin.Context) {
		/*
			Start get Data TA
		*/
		cookies := ctx.Request.Cookies()
		// Parse JWT token from cookie
		tokenString, err := ctx.Cookie("token")
		if err != nil {
			fun.ClearCookiesAndRedirect(ctx, cookies)

			return
		}
		tokenString = strings.ReplaceAll(tokenString, " ", "+")

		decrypted, err := fun.GetAESDecrypted(tokenString)
		if err != nil {
			fun.ClearCookiesAndRedirect(ctx, cookies)
			ctx.JSON(http.StatusInternalServerError, gin.H{"message": err.Error()})
			return
		}
		var claims map[string]interface{}
		err = json.Unmarshal(decrypted, &claims)
		if err != nil {
			fun.ClearCookiesAndRedirect(ctx, cookies)
			ctx.JSON(http.StatusInternalServerError, gin.H{"message": err.Error()})
			return
		}
		var admin model.Admin
		db_web.Where("id = ?", uint(claims["id"].(float64))).Find(&admin)
		/*
			.end of get Data TA
		*/

		var namaTA string = "N/A"
		if name, ok := taData[admin.Email]; ok {
			namaTA = name
		}

		var jsonData struct {
			Tabel    string `json:"tabel"`
			IdTask   string `json:"id_task"`
			WoNumber string `json:"wo_number"`
			Feedback string `json:"feedback"`
		}

		if err := ctx.ShouldBindJSON(&jsonData); err != nil {
			ctx.JSON(http.StatusBadRequest, gin.H{"error": "Invalid JSON: " + err.Error()})
			return
		}

		if err := db_pengerjaan.Table(strings.ToLower(jsonData.Tabel)).Where("id_task = ?", jsonData.IdTask).Update("ta_feedback", namaTA+" - "+jsonData.Feedback).Error; err != nil {
			ctx.JSON(http.StatusInternalServerError, gin.H{"message": err.Error()})
			return
		}

		ctx.JSON(http.StatusOK, gin.H{"message": fmt.Sprintf("Success add feedback: %v to WO Number: %s", jsonData.Feedback, jsonData.WoNumber)})

		go func() {
			var joData interface{}
			switch jsonData.Tabel {
			case "error":
				var errorData op_model.Error
				db_pengerjaan.Model(&op_model.Error{}).Where(&op_model.Error{IDTask: jsonData.IdTask}).First(&errorData)
				joData = errorData
			case "pending":
				var pendingData op_model.Pending
				db_pengerjaan.Model(&op_model.Pending{}).Where(&op_model.Pending{IDTask: jsonData.IdTask}).First(&pendingData)
				joData = pendingData
			case "temp_submission":
				var tempSubmissionData op_model.TempSubmission
				db_pengerjaan.Model(&op_model.TempSubmission{}).Where(&op_model.TempSubmission{IDTask: jsonData.IdTask}).First(&tempSubmissionData)
				joData = tempSubmissionData
			}

			var teknisi, woNumber, ticketSubject, merchant, mid, tid string
			switch v := joData.(type) {
			case op_model.Error:
				teknisi = v.Teknisi
				woNumber = v.WoNumber
				ticketSubject = v.SpkNumber
				merchant = *v.Merchant
				mid = v.MID
				tid = v.TID
			case op_model.Pending:
				teknisi = v.Teknisi
				woNumber = v.WoNumber
				ticketSubject = v.SpkNumber
				merchant = *v.Merchant
				mid = v.MID
				tid = v.TID
			case op_model.TempSubmission:
				teknisi = v.Teknisi
				woNumber = v.WONumber
				ticketSubject = v.SPKNumber
				merchant = v.Merchant
				mid = v.MID
				tid = v.TID
			}

			dataToSend := feedbackFromTA{
				Teknisi:       teknisi,
				EmailTa:       admin.Email,
				NamaTA:        namaTA,
				NomorTA:       admin.Phone,
				Tabel:         jsonData.Tabel,
				Feedback:      jsonData.Feedback,
				WoNumber:      woNumber,
				TicketSubject: ticketSubject,
				Merchant:      merchant,
				MID:           mid,
				TID:           tid,
			}
			sendTAFeedback(dataToSend)
		}()
	}
}

func sendTAFeedback(data feedbackFromTA) {
	urlToSend := config.TechnicalAssistance.Get().Default.TaFeedbackURL
	if !strings.HasPrefix(urlToSend, "http://") && !strings.HasPrefix(urlToSend, "https://") {
		urlToSend = "http://" + urlToSend
	}

	payload, err := json.Marshal(data)
	if err != nil {
		log.Printf("Failed to marshal feedback data: %v", err)
		return
	}

	req, err := http.NewRequest("POST", urlToSend, bytes.NewBuffer(payload))
	if err != nil {
		log.Printf("Failed to create POST request: %v", err)
		return
	}
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		log.Printf("Failed to send feedback POST request: %v", err)
		return
	}
	defer func() {
		if resp != nil && resp.Body != nil {
			resp.Body.Close()
		}
	}()

	if resp.StatusCode != http.StatusOK {
		log.Printf("Feedback POST request returned status: %s", resp.Status)
	}
}

func GetIDTaskStatusInTableTA(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		var requestData struct {
			Table  string `json:"table"`
			IDTask string `json:"id_task"`
		}

		if err := c.ShouldBindJSON(&requestData); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid JSON: " + err.Error()})
			return
		}

		var isIDExists bool = false
		err := db.Table(requestData.Table).Select("COUNT(*) > 0").Where("id_task = ?", requestData.IDTask).Find(&isIDExists).Error
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Database query error: " + err.Error()})
			return
		}

		c.JSON(http.StatusOK, gin.H{"exists": isIDExists})
	}
}
