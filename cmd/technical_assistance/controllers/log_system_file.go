package controllers

import (
	"log"
	"net/http"
	"os"
	"service-platform/cmd/technical_assistance/model"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

func GetSystemLogFiles(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		loginCookie, err := c.Request.Cookie("credentials")
		if err != nil || loginCookie == nil || loginCookie.Value == "" {
			// If cookie doesn't exist or is empty, redirect to login
			expiredCookie := http.Cookie{
				Name:    "credentials",
				Expires: time.Now().AddDate(0, 0, -1),
			}
			http.SetCookie(c.Writer, &expiredCookie)
			c.Redirect(http.StatusFound, "/login")
			return
		}

		var adminLogin model.Admin
		if err := db.Where("session = ?", loginCookie.Value).First(&adminLogin).Error; err != nil || adminLogin.ID == 0 {
			loginCookie.Expires = time.Now().AddDate(0, 0, -1)
			http.SetCookie(c.Writer, loginCookie)
			c.Redirect(302, "/login")
			return
		}

		// Open the folder
		dir, err := os.Open(os.Getenv("APP_LOG_DIR"))
		if err != nil {
			log.Fatal(err)
			c.JSON(http.StatusOK, gin.H{
				"data": []gin.H{},
			})
			return
		}
		defer dir.Close()

		// Read the contents of the folder
		files, err := dir.ReadDir(0) // 0 for no sorting
		if err != nil {
			log.Fatal(err)
			c.JSON(http.StatusOK, gin.H{
				"data": []gin.H{},
			})
			return
		}

		var filesData []string
		for _, file := range files {
			filesData = append(filesData, file.Name())
		}

		// Respond with the formatted data
		c.JSON(http.StatusOK, gin.H{
			"data": filesData,
		})
	}
}
