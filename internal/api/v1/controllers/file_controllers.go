package controllers

import (
	"net/http"
	"path/filepath"
	"strings"

	"service-platform/internal/pkg/fun"

	"github.com/gin-gonic/gin"
)

// GetUploadedFile godoc
// @Summary      Get Uploaded File
// @Description  Serves uploaded files securely
// @Tags         Files
// @Produce      octet-stream
// @Param        year     path      string  true  "Year"
// @Param        month    path      string  true  "Month"
// @Param        day      path      string  true  "Day"
// @Param        filename path      string  true  "Filename"
// @Success      200  {file}     file
// @Failure      403  {object}   dto.APIErrorResponse "Invalid file path"
// @Router       /api/v1/{access}/uploads/{year}/{month}/{day}/{filename} [get]
func GetUploadedFile(c *gin.Context) {
	// Extract parameters from the route
	year := c.Param("year")
	month := c.Param("month")
	day := c.Param("day")
	filename := c.Param("filename")

	// Construct the file path
	filePath := filepath.Join("./uploads", year, month, day, filename)

	// Clean the file path to prevent directory traversal
	safePath := filepath.Clean(filePath)

	// Base uploads directory
	uploadsDir := filepath.Clean("./uploads")

	// Ensure safePath is within uploadsDir
	rel, err := filepath.Rel(uploadsDir, safePath)
	if err != nil || strings.HasPrefix(rel, "..") || filepath.IsAbs(rel) {
		fun.HandleAPIErrorSimple(c, http.StatusForbidden, "invalid file path")
		return
	}

	// Serve the file
	c.File(safePath)
}
