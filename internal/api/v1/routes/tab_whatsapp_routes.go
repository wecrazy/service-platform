package routes

import (
	"service-platform/internal/api/v1/controllers"

	"github.com/gin-gonic/gin"
	"github.com/go-redis/redis/v8"
	"gorm.io/gorm"
)

// RegisterWhatsAppRoutes registers all WhatsApp bot & messaging routes under /tab-whatsapp.
func RegisterWhatsAppRoutes(api *gin.RouterGroup, db *gorm.DB, redisDB *redis.Client) {
	tabWhatsapp := api.Group("/tab-whatsapp")
	{
		// Connection management
		tabWhatsapp.GET("/status", controllers.GetWhatsAppStatus)
		tabWhatsapp.POST("/connect", controllers.ConnectWhatsApp(redisDB))
		tabWhatsapp.POST("/disconnect", controllers.DisconnectWhatsApp)
		tabWhatsapp.POST("/logout", controllers.LogoutWhatsApp)
		tabWhatsapp.POST("/refresh_qr", controllers.RefreshWhatsAppQR(redisDB))
		tabWhatsapp.GET("/qr/:token", controllers.ServeQRImage(redisDB))

		// Messaging
		tabWhatsapp.POST("/send_message", controllers.SendWhatsAppMessage(db))
		tabWhatsapp.POST("/create_status", controllers.CreateStatus(db))

		// Data tables
		tabWhatsapp.GET("/messages", controllers.GetWhatsAppMessages(db))
		tabWhatsapp.GET("/incoming", controllers.GetWhatsAppIncomingMessages(db))
		tabWhatsapp.GET("/groups", controllers.GetWhatsAppGroups(db))
		tabWhatsapp.POST("/groups/datatable", controllers.GetWhatsAppGroupsDataTable(db))
		tabWhatsapp.GET("/groups/count", controllers.GetWhatsAppGroupsCount(db))
		tabWhatsapp.GET("/groups/:jid", controllers.GetWhatsAppGroupByJID(db))
		tabWhatsapp.POST("/groups/sync", controllers.SyncWhatsAppGroups(db))
		tabWhatsapp.GET("/profile-picture/:jid", controllers.GetWhatsAppProfilePicture)
		tabWhatsapp.GET("/auto-reply", controllers.GetWhatsAppAutoReplyRules(db))
		tabWhatsapp.GET("/auto-reply/:id", controllers.GetWhatsAppAutoReplyRule(db))
		tabWhatsapp.POST("/auto-reply", controllers.CreateWhatsAppAutoReplyRule(db))
		tabWhatsapp.PUT("/auto-reply/:id", controllers.UpdateWhatsAppAutoReplyRule(db))
		tabWhatsapp.DELETE("/auto-reply/:id", controllers.DeleteWhatsAppAutoReplyRule(db))

		// Language support
		tabWhatsapp.GET("/languages/count", controllers.GetWhatsAppLanguagesCount(db))
		tabWhatsapp.GET("/languages", controllers.GetWhatsAppLanguages(db))

		// Phone & Contacts status
		tabWhatsapp.GET("/contacts-count", controllers.GetWhatsAppContactsCount)
		tabWhatsapp.GET("/phone-status", controllers.GetWhatsAppPhoneStatus)

		// Configuration
		tabWhatsapp.GET("/data-separator", controllers.GetDataSeparator)
	}
}
