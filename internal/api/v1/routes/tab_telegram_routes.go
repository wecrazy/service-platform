package routes

import (
	telegramcontrollers "service-platform/internal/api/v1/controllers/telegram_controllers"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

// RegisterTelegramRoutes registers all Telegram bot & messaging routes under /tab-telegram.
func RegisterTelegramRoutes(api *gin.RouterGroup, db *gorm.DB) {
	tabTelegram := api.Group("/tab-telegram")
	{
		// Messaging
		tabTelegram.POST("/send_message", telegramcontrollers.SendTelegramMessage(db))
		tabTelegram.POST("/send_message_with_keyboard", telegramcontrollers.SendMessageWithKeyboard(db))
		tabTelegram.POST("/edit_message", telegramcontrollers.EditTelegramMessage(db))
		tabTelegram.POST("/answer_callback_query", telegramcontrollers.AnswerCallbackQuery(db))

		// Media
		tabTelegram.POST("/send_voice", telegramcontrollers.SendTelegramVoice(db))
		tabTelegram.POST("/send_document", telegramcontrollers.SendTelegramDocument(db))
		tabTelegram.POST("/send_photo", telegramcontrollers.SendTelegramPhoto(db))
		tabTelegram.POST("/send_audio", telegramcontrollers.SendTelegramAudio(db))
		tabTelegram.POST("/send_video", telegramcontrollers.SendTelegramVideo(db))
	}
}
