package controllers

import (
	"net/http"
	"service-platform/internal/api/v1/dto"
	"service-platform/internal/pkg/fun"
	"service-platform/internal/telegram"

	pb "service-platform/proto"

	"github.com/gin-gonic/gin"
	"github.com/sirupsen/logrus"
	"google.golang.org/grpc/status"
)

// SendTelegramMessage godoc
// @Summary      Send Telegram Message
// @Description  Sends a text message via Telegram bot
// @Tags         Telegram
// @Accept       json
// @Produce      json
// @Param        request body dto.SendTelegramMessageRequest true "Message Request"
// @Success      200  {object}   map[string]interface{}
// @Failure      503  {object}   dto.APIErrorResponse "Service Unavailable"
// @Router       /api/v1/{access}/tab-telegram/send_message [post]
func SendTelegramMessage(db interface{}) gin.HandlerFunc {
	return func(c *gin.Context) {
		if telegram.Client == nil {
			fun.HandleAPIErrorSimple(c, http.StatusServiceUnavailable, "Telegram service not available")
			return
		}

		var req dto.SendTelegramMessageRequest

		if err := c.ShouldBindJSON(&req); err != nil {
			fun.HandleAPIErrorSimple(c, http.StatusBadRequest, err.Error())
			return
		}

		// Call the Telegram gRPC service
		resp, err := telegram.Client.SendMessage(c.Request.Context(), &pb.SendTelegramMessageRequest{
			ChatId:    req.ChatID,
			Text:      req.Text,
			ParseMode: req.ParseMode,
		})

		if err != nil {
			if grpcErr, ok := status.FromError(err); ok {
				logrus.WithError(err).Error("Failed to send Telegram message via gRPC")
				fun.HandleAPIErrorSimple(c, http.StatusInternalServerError, grpcErr.Message())
				return
			}
			logrus.WithError(err).Error("Failed to send Telegram message")
			fun.HandleAPIErrorSimple(c, http.StatusInternalServerError, err.Error())
			return
		}

		if !resp.Success {
			logrus.WithField("message", resp.Message).Error("Telegram service returned failure")
			fun.HandleAPIErrorSimple(c, http.StatusInternalServerError, resp.Message)
			return
		}

		// Return success response
		c.JSON(http.StatusOK, gin.H{
			"success":    true,
			"message":    resp.Message,
			"message_id": resp.MessageId,
		})
	}
}

// SendMessageWithKeyboard godoc
// @Summary      Send Telegram Message with Keyboard
// @Description  Sends a message with an inline keyboard via Telegram bot
// @Tags         Telegram
// @Accept       json
// @Produce      json
// @Param        request body dto.SendTelegramMessageWithKeyboardRequest true "Message with Keyboard Request"
// @Success      200  {object}   map[string]interface{}
// @Failure      503  {object}   dto.APIErrorResponse "Service Unavailable"
// @Router       /api/v1/{access}/tab-telegram/send_message_with_keyboard [post]
func SendMessageWithKeyboard(db interface{}) gin.HandlerFunc {
	return func(c *gin.Context) {
		if telegram.Client == nil {
			fun.HandleAPIErrorSimple(c, http.StatusServiceUnavailable, "Telegram service not available")
			return
		}

		var req dto.SendTelegramMessageWithKeyboardRequest

		if err := c.ShouldBindJSON(&req); err != nil {
			fun.HandleAPIErrorSimple(c, http.StatusBadRequest, err.Error())
			return
		}

		// Convert DTO keyboard to protobuf keyboard
		var keyboard *pb.InlineKeyboardMarkup
		if req.Keyboard != nil {
			keyboard = convertDTOKeyboardToProto(req.Keyboard)
		}

		// Call the Telegram gRPC service
		resp, err := telegram.Client.SendMessageWithKeyboard(c.Request.Context(), &pb.SendTelegramMessageWithKeyboardRequest{
			ChatId:    req.ChatID,
			Text:      req.Text,
			ParseMode: req.ParseMode,
			Keyboard:  keyboard,
		})

		if err != nil {
			if grpcErr, ok := status.FromError(err); ok {
				logrus.WithError(err).Error("Failed to send Telegram message with keyboard via gRPC")
				fun.HandleAPIErrorSimple(c, http.StatusInternalServerError, grpcErr.Message())
				return
			}
			logrus.WithError(err).Error("Failed to send Telegram message with keyboard")
			fun.HandleAPIErrorSimple(c, http.StatusInternalServerError, err.Error())
			return
		}

		if !resp.Success {
			logrus.WithField("message", resp.Message).Error("Telegram service returned failure")
			fun.HandleAPIErrorSimple(c, http.StatusInternalServerError, resp.Message)
			return
		}

		// Return success response
		c.JSON(http.StatusOK, gin.H{
			"success":    true,
			"message":    resp.Message,
			"message_id": resp.MessageId,
		})
	}
}

// EditTelegramMessage godoc
// @Summary      Edit Telegram Message
// @Description  Edits an existing Telegram message
// @Tags         Telegram
// @Accept       json
// @Produce      json
// @Param        request body dto.EditTelegramMessageRequest true "Edit Message Request"
// @Success      200  {object}   map[string]interface{}
// @Failure      503  {object}   dto.APIErrorResponse "Service Unavailable"
// @Router       /api/v1/{access}/tab-telegram/edit_message [post]
func EditTelegramMessage(db interface{}) gin.HandlerFunc {
	return func(c *gin.Context) {
		if telegram.Client == nil {
			fun.HandleAPIErrorSimple(c, http.StatusServiceUnavailable, "Telegram service not available")
			return
		}

		var req dto.EditTelegramMessageRequest

		if err := c.ShouldBindJSON(&req); err != nil {
			fun.HandleAPIErrorSimple(c, http.StatusBadRequest, err.Error())
			return
		}

		// Convert DTO keyboard to protobuf keyboard
		var keyboard *pb.InlineKeyboardMarkup
		if req.Keyboard != nil {
			keyboard = convertDTOKeyboardToProto(req.Keyboard)
		}

		// Call the Telegram gRPC service
		resp, err := telegram.Client.EditMessage(c.Request.Context(), &pb.EditTelegramMessageRequest{
			ChatId:    req.ChatID,
			MessageId: req.MessageID,
			Text:      req.Text,
			ParseMode: req.ParseMode,
			Keyboard:  keyboard,
		})

		if err != nil {
			if grpcErr, ok := status.FromError(err); ok {
				logrus.WithError(err).Error("Failed to edit Telegram message via gRPC")
				fun.HandleAPIErrorSimple(c, http.StatusInternalServerError, grpcErr.Message())
				return
			}
			logrus.WithError(err).Error("Failed to edit Telegram message")
			fun.HandleAPIErrorSimple(c, http.StatusInternalServerError, err.Error())
			return
		}

		if !resp.Success {
			logrus.WithField("message", resp.Message).Error("Telegram service returned failure")
			fun.HandleAPIErrorSimple(c, http.StatusInternalServerError, resp.Message)
			return
		}

		// Return success response
		c.JSON(http.StatusOK, gin.H{
			"success": true,
			"message": resp.Message,
		})
	}
}

// AnswerCallbackQuery godoc
// @Summary      Answer Callback Query
// @Description  Answers a callback query from an inline keyboard button
// @Tags         Telegram
// @Accept       json
// @Produce      json
// @Param        request body dto.TelegramAnswerCallbackQueryRequest true "Answer Callback Query Request"
// @Success      200  {object}   map[string]interface{}
// @Failure      503  {object}   dto.APIErrorResponse "Service Unavailable"
// @Router       /api/v1/{access}/tab-telegram/answer_callback_query [post]
func AnswerCallbackQuery(db interface{}) gin.HandlerFunc {
	return func(c *gin.Context) {
		if telegram.Client == nil {
			fun.HandleAPIErrorSimple(c, http.StatusServiceUnavailable, "Telegram service not available")
			return
		}

		var req dto.TelegramAnswerCallbackQueryRequest

		if err := c.ShouldBindJSON(&req); err != nil {
			fun.HandleAPIErrorSimple(c, http.StatusBadRequest, err.Error())
			return
		}

		// Call the Telegram gRPC service
		resp, err := telegram.Client.AnswerCallbackQuery(c.Request.Context(), &pb.TelegramAnswerCallbackQueryRequest{
			CallbackQueryId: req.CallbackQueryID,
			Text:            req.Text,
			ShowAlert:       req.ShowAlert,
		})

		if err != nil {
			if grpcErr, ok := status.FromError(err); ok {
				logrus.WithError(err).Error("Failed to answer callback query via gRPC")
				fun.HandleAPIErrorSimple(c, http.StatusInternalServerError, grpcErr.Message())
				return
			}
			logrus.WithError(err).Error("Failed to answer callback query")
			fun.HandleAPIErrorSimple(c, http.StatusInternalServerError, err.Error())
			return
		}

		if !resp.Success {
			logrus.WithField("message", resp.Message).Error("Telegram service returned failure")
			fun.HandleAPIErrorSimple(c, http.StatusInternalServerError, resp.Message)
			return
		}

		// Return success response
		c.JSON(http.StatusOK, gin.H{
			"success": true,
			"message": resp.Message,
		})
	}
}

// convertDTOKeyboardToProto converts DTO keyboard to protobuf keyboard
func convertDTOKeyboardToProto(dtoKeyboard *dto.InlineKeyboardMarkup) *pb.InlineKeyboardMarkup {
	if dtoKeyboard == nil {
		return nil
	}

	protoKeyboard := &pb.InlineKeyboardMarkup{}
	for _, row := range dtoKeyboard.InlineKeyboard {
		protoRow := &pb.InlineKeyboardButtonRow{}
		for _, button := range row.Buttons {
			protoButton := &pb.InlineKeyboardButton{
				Text:         button.Text,
				CallbackData: button.CallbackData,
				Url:          button.URL,
			}
			protoRow.Buttons = append(protoRow.Buttons, protoButton)
		}
		protoKeyboard.InlineKeyboard = append(protoKeyboard.InlineKeyboard, protoRow)
	}

	return protoKeyboard
}
