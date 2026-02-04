package controllers

import (
	"bufio"
	"net/http"
	"os"
	"service-platform/cmd/web_panel/model"
	"service-platform/internal/config"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/sirupsen/logrus"
	"gorm.io/gorm"
)

func GetSystemLog(db *gorm.DB) gin.HandlerFunc {
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

		fileToRead := c.Query("v")
		if fileToRead == "" {
			fileToRead = config.WebPanel.Get().App.AppLogFilename
		}
		fileToRead = strings.ReplaceAll(fileToRead, "/", "")
		fileToRead = strings.ReplaceAll(fileToRead, "..", "")
		fileToRead = strings.ReplaceAll(fileToRead, "\\", "")

		file, err := os.Open(config.WebPanel.Get().App.LogDir + "/" + fileToRead)
		if err != nil {
			logrus.Fatal(err)
			c.JSON(http.StatusOK, gin.H{
				"data": []gin.H{},
			})
			return
		}
		defer file.Close()
		// Create a scanner to read the file line by line
		scanner := bufio.NewScanner(file)

		// Create a slice to store the lines in reverse order
		var lines []string

		// Read the file line by line
		for scanner.Scan() {
			// Prepend each line to the slice
			lines = append([]string{scanner.Text()}, lines...)
		}

		// Check for any errors during scanning
		if err := scanner.Err(); err != nil {
			logrus.Fatal(err)
			c.JSON(http.StatusOK, gin.H{
				"data": []gin.H{},
			})
			return
		}
		if len(lines) == 0 {
			c.JSON(http.StatusOK, gin.H{
				"data": []gin.H{},
			})
			return
		}

		// Loop through the lines and process them as needed
		// Format the result as needed
		var data []gin.H
		for _, line := range lines {
			if strings.HasPrefix(line, "[LOG]") {
				data = append(data, gin.H{
					"l": line,
				})
			}
		}

		// Respond with the formatted data
		c.JSON(http.StatusOK, gin.H{
			"data": data,
		})
	}
}
