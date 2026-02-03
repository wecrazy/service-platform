package controllers

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"service-platform/cmd/technical_assistance/fun"
	"service-platform/cmd/technical_assistance/model"
	"strings"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

func GetUserProfile(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Header("Content-Type", "image/jpeg")
		filePath := os.Getenv("APP_STATIC_DIR") + "/assets/img/avatars/default.jpg"

		pathParam := c.Query("f")
		claims, err := fun.GetAESDecryptedURLtoJSON(pathParam)
		if err != nil {
			fmt.Println("Error during decryption", err.Error())
			c.File(filePath)
			return
		}
		var admin model.Admin
		if err := db.Where("id = ?", claims["id"]).First(&admin).Error; err != nil {
			fmt.Println("Error during getting the id")
			c.File(filePath)
			return
		}

		if admin.Session == "" || admin.SessionExpired == 0 {
			fmt.Println("session not active")
			c.File(filePath)
			return
		}
		if admin.ProfileImage == "" {
			c.File(filePath)
			return
		}

		// filePath = os.Getenv("APP_STATIC_DIR") + "/" + admin.ProfileImage
		filePath = admin.ProfileImage
		// Open the file
		file, err := os.Open(filePath)
		if err != nil {
			fmt.Println("cannot opening file", err.Error())
			c.File(filePath)
			return
		}
		defer file.Close()
		// Serve the file
		c.File(filePath)
	}
}

// Function to update the admin profile image
func UpdateAdminProfileImage(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		// Retrieve cookies
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

		var admin model.Admin
		db.Where("id = ? ", uint(claims["id"].(float64))).Find(&admin)
		if admin.ID == 0 {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "User ID Not Found"})
			return
		}

		// Retrieve the file from the request
		file, err := c.FormFile("image")
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "No file is received"})
			return
		}

		var allowedExtensions = map[string]bool{
			".jpg":  true,
			".jpeg": true,
			".png":  true,
		}

		if !allowedExtensions[strings.ToLower(filepath.Ext(file.Filename))] {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid file type. Only JPG and PNG are allowed."})
			return
		}

		// Validate the file type and size
		if file.Header.Get("Content-Type") != "image/jpeg" && file.Header.Get("Content-Type") != "image/png" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid file type. Only JPG and PNG are allowed."})
			return
		}
		if file.Size > 1*1024*1024 {
			c.JSON(http.StatusBadRequest, gin.H{"error": "File size exceeds 1MB"})
			return
		}
		// Open the file
		openedFile, err := file.Open()
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Unable to open the file"})
			return
		}
		defer openedFile.Close()

		// Validate the file type
		isValid, _ := fun.IsValidImage(openedFile)
		if !isValid {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid file type. Only JPG and PNG are allowed."})
			return
		}

		// Save the file to the server
		filename := fmt.Sprintf("%d%s", admin.ID, filepath.Ext(file.Filename))
		filePath := filepath.Join(os.Getenv("APP_UPLOAD_DIR")+"/admin", filename)
		if err := c.SaveUploadedFile(file, filePath); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Unable to save the file"})
			return
		}

		filePath = strings.Trim(filePath, os.Getenv("APP_STATIC_DIR"))
		// Update the admin's profile image path in the database
		admin.ProfileImage = filePath
		if err := db.Save(&admin).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Unable to update profile image"})
			return
		}

		// Respond with a success message
		c.JSON(http.StatusAccepted, gin.H{"success": true, "msg": "Profile image updated successfully"})
	}
}
