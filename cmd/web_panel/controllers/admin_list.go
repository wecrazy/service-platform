package controllers

import (
	"net/http"
	"service-platform/cmd/web_panel/model"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

func GetUserAdmin(db *gorm.DB) gin.HandlerFunc {
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

		var adminsWithRoles []struct {
			model.Admin
			RoleName string `json:"role_name" gorm:"column:role_name"`
		}

		if err := db.
			Table("admins a").
			Unscoped(). // Disable soft deletes for this query
			Select("a.*, b.role_name").
			Joins("LEFT JOIN roles b ON a.role = b.id").
			// Offset(0).
			// Limit(1).
			Find(&adminsWithRoles).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{
				"error": "Error querying database",
			})
			return
		}
		// Format the result as needed
		var data []gin.H
		for _, admin := range adminsWithRoles {
			data = append(data, gin.H{
				"id":        admin.ID,
				"full_name": admin.Fullname,
				"role":      admin.RoleName,
				"username":  admin.Username,
				"email":     admin.Email,
				"status":    admin.Status + 1,
				"avatar":    "",
			})
		}

		// Respond with the formatted data
		c.JSON(http.StatusOK, gin.H{
			"data": data,
		})
		// Save the newLog to the database
		db.Create(&model.LogActivity{
			AdminID:   adminLogin.ID,
			FullName:  adminLogin.Fullname,
			Action:    "GET",
			Status:    "Success",
			Log:       "GET List Admin Data",
			IP:        c.ClientIP(),
			UserAgent: c.Request.UserAgent(),
			ReqMethod: c.Request.Method,
			ReqUri:    c.Request.RequestURI,
		})

	}
}
