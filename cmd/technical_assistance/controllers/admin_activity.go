package controllers

import (
	"encoding/json"
	"fmt"
	"net/http"
	"service-platform/cmd/technical_assistance/fun"
	"service-platform/cmd/technical_assistance/model"
	"strings"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

func TableUserActivities(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		cookies := c.Request.Cookies()

		// Parse JWT token from cookie
		tokenString, err := c.Cookie("token")
		if err != nil {
			fun.ClearCookiesAndRedirect(c, cookies)
			return
		}
		tokenString = strings.ReplaceAll(tokenString, " ", "+")

		decrypted, err := fun.GetAESDecrypted(tokenString)
		if err != nil {
			fmt.Println("Error during decryption", err)
			fun.ClearCookiesAndRedirect(c, cookies)
			return
		}
		var claims map[string]interface{}
		err = json.Unmarshal(decrypted, &claims)
		if err != nil {
			fmt.Printf("Error converting JSON to map: %v", err)
			fun.ClearCookiesAndRedirect(c, cookies)
			return
		}

		var logActivities []model.LogActivity
		result := db.Where("admin_id = ? ", uint(claims["id"].(float64))).Find(&logActivities)

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
