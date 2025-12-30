package fun

import (
	"service-platform/internal/api/v1/dto"

	"github.com/gin-gonic/gin"
	"github.com/sirupsen/logrus"
)

// HandleAPIError sends a standardized error response and logs the error
func HandleAPIError(c *gin.Context, statusCode int, message string, details string, code string) {
	response := dto.APIErrorResponse{
		Error:   message,
		Details: details,
		Code:    code,
	}

	logrus.Errorf("API Error - Status: %d, Code: %s, Message: %s, Details: %s", statusCode, code, message, details)

	c.JSON(statusCode, response)
}

// HandleAPIErrorSimple sends a simple error response without details or code
func HandleAPIErrorSimple(c *gin.Context, statusCode int, message string) {
	HandleAPIError(c, statusCode, message, "", "")
}
