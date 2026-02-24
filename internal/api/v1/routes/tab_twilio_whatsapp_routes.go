package routes

import (
	"service-platform/internal/api/v1/controllers"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

// RegisterTwilioWhatsAppRoutes registers all Twilio WhatsApp messaging routes under /tab-twilio-whatsapp.
func RegisterTwilioWhatsAppRoutes(api *gin.RouterGroup, db *gorm.DB) {
	tabTwilioWhatsApp := api.Group("/tab-twilio-whatsapp")
	{
		// Messaging
		tabTwilioWhatsApp.POST("/send_message", controllers.SendTwilioWhatsAppMessage(db))
		tabTwilioWhatsApp.POST("/send_media_message", controllers.SendTwilioWhatsAppMediaMessage(db))

		// Message history
		tabTwilioWhatsApp.GET("/messages", controllers.GetTwilioWhatsAppMessages(db))
		tabTwilioWhatsApp.GET("/incoming", controllers.GetTwilioWhatsAppIncomingMessages(db))
	}
}
