package controllers

import (
	"net/http"
	"service-platform/pkg/fun"

	"github.com/dchest/captcha"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

// GetHealthCheck godoc
// @Summary      System Health Check
// @Description  Returns the health status of the system, database, and resources
// @Tags         System
// @Produce      json
// @Success      200  {object}   map[string]interface{}
// @Failure      503  {object}   map[string]interface{}
// @Router       /health [get]
func GetHealthCheck(db *gorm.DB, systemMonitor *fun.SystemResourceMonitor) gin.HandlerFunc {
	return func(c *gin.Context) {
		health := systemMonitor.GetHealthStatus(db)

		// Check database connections
		dbStatus := "healthy"
		if db == nil {
			dbStatus = "disconnected"
		} else {
			sqlDB, err := db.DB()
			if err != nil || sqlDB.Ping() != nil {
				dbStatus = "unhealthy"
			}
		}
		health["database"] = dbStatus

		if dbStatus != "healthy" {
			health["status"] = "degraded"
			c.JSON(http.StatusServiceUnavailable, health)
			return
		}

		// Check if status is critical (from system monitor)
		if status, ok := health["status"].(string); ok && status == "critical" {
			c.JSON(http.StatusServiceUnavailable, health)
			return
		}

		c.JSON(http.StatusOK, health)
	}
}

// GetHello godoc
// @Summary      Hello World
// @Description  Returns a simple hello world message
// @Tags         System
// @Produce      json
// @Success      200  {object}   map[string]string
// @Router       /hello [get]
func GetHello(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"message": "Hello, World!"})
}

// GetNewCaptcha godoc
// @Summary      Get New Captcha
// @Description  Generates a new captcha ID
// @Tags         System
// @Produce      json
// @Success      200  {object}   map[string]string
// @Router       /captcha/new [get]
func GetNewCaptcha(c *gin.Context) {
	id := captcha.New()
	c.JSON(http.StatusOK, gin.H{"captcha_id": id})
}

// GetCaptchaImage godoc
// @Summary      Get Captcha Image
// @Description  Returns the captcha image for a given ID
// @Tags         System
// @Produce      image/png
// @Param        id   path      string  true  "Captcha ID"
// @Success      200  {file}     file
// @Router       /captcha/{id}.png [get]
func GetCaptchaImage(_ *gin.Context) {
	// This is a wrapper, actual implementation is handled by captcha.Server
}
