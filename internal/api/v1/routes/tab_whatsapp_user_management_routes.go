package routes

import (
	"service-platform/internal/api/v1/controllers"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

// RegisterWhatsAppUserManagementRoutes registers all WhatsApp user CRUD routes under /tab-whatsapp-user-management.
func RegisterWhatsAppUserManagementRoutes(api *gin.RouterGroup, db *gorm.DB) {
	tabWhatsappUserManagement := api.Group("/tab-whatsapp-user-management")
	{
		// Statistics
		tabWhatsappUserManagement.GET("/statistics", controllers.GetWhatsAppUserStatistics(db))

		// Export & Import (must be before :id routes to avoid conflicts)
		tabWhatsappUserManagement.GET("/users/export", controllers.ExportWhatsAppUsers(db))
		tabWhatsappUserManagement.GET("/users/:id", controllers.GetWhatsAppUser(db))
		tabWhatsappUserManagement.POST("/users", controllers.CreateWhatsAppUser(db))
		tabWhatsappUserManagement.PUT("/users/:id", controllers.UpdateWhatsAppUser(db))
		tabWhatsappUserManagement.PATCH("/users/:id/ban", controllers.ToggleBanWhatsAppUser(db))
		tabWhatsappUserManagement.DELETE("/users/:id", controllers.DeleteWhatsAppUser(db))
	}
}
