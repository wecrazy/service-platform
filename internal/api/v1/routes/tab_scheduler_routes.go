package routes

import (
	"service-platform/internal/api/v1/controllers"

	"github.com/gin-gonic/gin"
)

// RegisterSchedulerRoutes registers all scheduler management routes under /tab-scheduler.
func RegisterSchedulerRoutes(api *gin.RouterGroup) {
	tabScheduler := api.Group("/tab-scheduler")
	{
		tabScheduler.GET("/jobs", controllers.ListScheduledJobs())               // List all jobs
		tabScheduler.GET("/jobs/:name", controllers.GetJobStatus())              // Get specific job status
		tabScheduler.POST("/jobs", controllers.RegisterScheduledJob())           // Register new job
		tabScheduler.POST("/jobs/trigger", controllers.TriggerScheduledJob())    // Trigger job manually
		tabScheduler.DELETE("/jobs/:name", controllers.UnregisterScheduledJob()) // Unregister job
		tabScheduler.POST("/reload", controllers.ReloadScheduler())              // Reload scheduler config
	}
}
