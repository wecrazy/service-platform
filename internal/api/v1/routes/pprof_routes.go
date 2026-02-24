// Package routes defines the HTTP route registration for the service-platform API.
package routes

import (
	"net/http"
	"service-platform/internal/api/v1/controllers"

	"github.com/gin-gonic/gin"
)

// RegisterPprofRoutes registers all pprof debug routes under /debug/pprof.
// To view in charts mode: go tool pprof -http=:2222 http://localhost:2221/debug/pprof/profile
func RegisterPprofRoutes(router *gin.Engine, globalURL string) {
	pprofGroup := router.Group(globalURL + "debug/pprof")
	{
		pprofGroup.GET("/", gin.WrapF(http.HandlerFunc(controllers.PprofIndex)))
		pprofGroup.GET("/heap", gin.WrapF(http.HandlerFunc(controllers.PprofHeap)))
		pprofGroup.GET("/profile", gin.WrapF(http.HandlerFunc(controllers.PprofProfile)))
		pprofGroup.GET("/block", gin.WrapF(http.HandlerFunc(controllers.PprofBlock)))
		pprofGroup.GET("/goroutine", gin.WrapF(http.HandlerFunc(controllers.PprofGoroutine)))
		pprofGroup.GET("/threadcreate", gin.WrapF(http.HandlerFunc(controllers.PprofThreadcreate)))
		pprofGroup.GET("/cmdline", gin.WrapF(http.HandlerFunc(controllers.PprofCmdline)))
		pprofGroup.GET("/symbol", gin.WrapF(http.HandlerFunc(controllers.PprofSymbol)))
		pprofGroup.POST("/symbol", gin.WrapF(http.HandlerFunc(controllers.PprofSymbol)))
		pprofGroup.GET("/trace", gin.WrapF(http.HandlerFunc(controllers.PprofTrace)))
		pprofGroup.GET("/allocs", gin.WrapF(http.HandlerFunc(controllers.PprofAllocs)))
		pprofGroup.GET("/mutex", gin.WrapF(http.HandlerFunc(controllers.PprofMutex)))
	}
}
