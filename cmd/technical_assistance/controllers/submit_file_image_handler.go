package controllers

import (
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"service-platform/cmd/technical_assistance/model/cc_model"
	"strings"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

func SubmitCcImageFile(db_call_center *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		headerValue := c.GetHeader("upload")
		expectedHeaderValue := "WWYJR5TlPdoiIPyKCnGl4rkhlFD28GCAl5qibtukZGsOgJ5aF6H0XUfD0sIDHdZh"
		// var tokenStruct cc_model.TokenStruct
		// db_call_center.Where("id = 1").First(&tokenStruct)
		// expectedHeaderValue = tokenStruct.Token
		if headerValue != expectedHeaderValue {
			c.JSON(401, gin.H{"error": "Unauthorized: Invalid header value"})
			return
		}

		id := c.PostForm("idJO")
		fmt.Println("Received idJO:", id) // Debug print

		var cc_models cc_model.JOMerchantHmin1CallLog
		if err := db_call_center.Where("id = ?", id).First(&cc_models).Error; err != nil {
			fmt.Println("Error fetching record from DB:", err) // Debug print
			c.JSON(http.StatusBadRequest, gin.H{"error": "Failed to get file"})
			return
		}

		if cc_models.ID == 0 {
			fmt.Println("Record not found for ID:", id) // Debug print
			c.JSON(http.StatusBadRequest, gin.H{"error": "Failed to get file"})
			return
		}

		// Extract and create directories
		directories := []string{cc_models.ImgWaPath, cc_models.ImgMerchant, cc_models.ImgSnEdcPath}
		for i, dir := range directories {
			// Convert to Ubuntu/Linux style
			dir = strings.ReplaceAll(dir, "\\", `/`)
			// Add "./" to make it relative
			directories[i] = dir
			dir := "./" + dir
			dirPath := filepath.Dir(dir)
			if err := os.MkdirAll(dirPath, os.ModePerm); err != nil {
				fmt.Println("Failed to create directory:", dirPath, "Error:", err) // Debug print
				c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create directories"})
				return
			}
		}
		// Handle file uploads
		files := map[string]string{
			"imgWA":       "./" + strings.ReplaceAll(cc_models.ImgWaPath, "\\", `/`),
			"imgMerchant": "./" + strings.ReplaceAll(cc_models.ImgMerchantPath, "\\", `/`),
			"imgSNEDC":    "./" + strings.ReplaceAll(cc_models.ImgSnEdcPath, "\\", `/`),
		}

		for formKey, savePath := range files {
			// fmt.Println("formKey")
			// fmt.Println(formKey)
			// fmt.Println("savePath")
			// fmt.Println(savePath)
			file, err := c.FormFile(formKey)
			if err != nil {
				// fmt.Println("Failed to get file:", formKey, "Error:", err) // Debug print
				// c.JSON(http.StatusBadRequest, gin.H{"error": "Failed to get file"})
				continue
			}

			if err := c.SaveUploadedFile(file, savePath); err != nil {
				// fmt.Println("Failed to save file:", savePath, "Error:", err) // Debug print
				// c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to save file"})
				continue
			}
		}

		c.JSON(200, gin.H{
			"message":       "File uploaded successfully",
			"id_jo":         id,
			"file_wa":       "https://ms_cc.csna4u.com/" + cc_models.ImgWaPath,
			"file_merchant": "https://ms_cc.csna4u.com/" + cc_models.ImgMerchant,
			"file_sn_edc":   "https://ms_cc.csna4u.com/" + cc_models.ImgSnEdcPath,
		})
	}
}
