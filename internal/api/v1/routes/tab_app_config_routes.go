package routes

import (
	"service-platform/internal/api/v1/controllers"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

// RegisterAppConfigRoutes registers all app config routes under /tab-app-config.
func RegisterAppConfigRoutes(api *gin.RouterGroup, db *gorm.DB) {
	tabAppConfig := api.Group("/tab-app-config")
	{
		tabAppConfig.POST("/table", controllers.TableAppConfig(db))
	}
}
