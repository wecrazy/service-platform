// Package telegramcontrollers provides Telegram bot command and message handling logic.
package telegramcontrollers

import (
	"context"
	"fmt"
	odoomscontrollers "service-platform/internal/api/v1/controllers/odooms_controllers"
	"service-platform/internal/config"
	telegrammodel "service-platform/internal/core/model/telegram_model"
	"service-platform/internal/database"
	"service-platform/pkg/fun"
	"strings"
	"time"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"github.com/sirupsen/logrus"
	"gorm.io/gorm"
)

// canAccessCommand checks if user type can access specific commands
func (*TelegramHelper) canAccessCommand(userType telegrammodel.TelegramUserType, allowedTypes []telegrammodel.TelegramUserType) bool {
	for _, allowedType := range allowedTypes {
		if userType == allowedType {
			return true
		}
	}
	return false
}

// isValidCommandForUserType checks if a command is valid for the given user type
func (*TelegramHelper) isValidCommandForUserType(command string, userType telegrammodel.TelegramUserType) bool {
	validCommands := map[telegrammodel.TelegramUserType][]string{
		telegrammodel.CommonUser: {
			"start", "help",
		},
		telegrammodel.SuperUser: {
			"start", "help", "input_wo", "input_tid", "info_tid", "generate_report_ta", "view_status_sp",
		},
		telegrammodel.TechnicianMS: {
			"start", "help", "input_wo", "input_tid", "info_tid",
		},
		telegrammodel.TAMS: {
			"start", "help", "generate_report_ta",
		},
		telegrammodel.SPLMS: {
			"start", "help", "input_wo", "input_tid", "info_tid", "view_status_sp",
		},
		telegrammodel.SACMS: {
			"start", "help", "input_wo", "input_tid", "info_tid", "view_status_sp",
		},
		// TODO: implement different for the UserType = HeadMS, maybe had more features & access
		telegrammodel.HeadMS: {
			"start", "help", "input_wo", "input_tid", "info_tid", "view_status_sp",
		},
	}

	commands, exists := validCommands[userType]
	if !exists {
		return false
	}

	for _, validCmd := range commands {
		if command == validCmd {
			return true
		}
	}
	return false
}

// handleStartCommand handles the /start command
func (h *TelegramHelper) handleStartCommand(message *tgbotapi.Message, telegramUser telegrammodel.TelegramUsers, userLang string) {
	if !telegramUser.VerifiedUser {
		// Start registration process
		h.startRegistration(message.Chat.ID, message.From, userLang)
		return
	}

	// User is verified, show start menu based on user type
	keyboard := h.CreateStartKeyboard(userLang)
	welcomeMsg := h.getLocalizedMessage(userLang, "welcome")
	msg := tgbotapi.NewMessage(message.Chat.ID, welcomeMsg)
	msg.ReplyMarkup = keyboard
	h.bot.Send(msg)
}

// handleHelpCommand handles the /help command with role-based help
func (h *TelegramHelper) handleHelpCommand(message *tgbotapi.Message, telegramUser telegrammodel.TelegramUsers, userLang string) {
	keyboard := h.CreateHelpKeyboardByUserType(telegramUser.UserType, userLang)
	helpMsg := h.getHelpMessageForUserType(telegramUser.UserType, userLang)
	msg := tgbotapi.NewMessage(message.Chat.ID, helpMsg)
	msg.ReplyMarkup = keyboard
	// msg.ParseMode = "Markdown"  // Temporarily disabled to avoid parsing errors
	h.bot.Send(msg)
}

// handleResetCommand handles the /reset command to allow re-registration
func (h *TelegramHelper) handleResetCommand(message *tgbotapi.Message, userLang string) {
	// Use the language code from the message to re-get userLang
	userLang = message.From.LanguageCode
	if userLang == "" {
		userLang = fun.DefaultLang // default fallback
	}

	// Try to find and delete the user record
	var telegramUser telegrammodel.TelegramUsers
	err := h.db.Where("telegram_chat_id = ?", message.Chat.ID).First(&telegramUser).Error
	if err == nil {
		// User exists, delete it
		err = h.db.Unscoped().Delete(&telegramUser).Error
		if err != nil {
			logrus.WithError(err).Error("Failed to delete user record for reset")
			msg := tgbotapi.NewMessage(message.Chat.ID, h.getLocalizedMessage(userLang, "reset_failed"))
			msg.ReplyToMessageID = message.MessageID
			h.bot.Send(msg)
			return
		}
	} // If user not found, continue anyway

	// Clear any Redis keys related to this user
	keys := []string{
		fmt.Sprintf("telegram:user:lang:%d", message.From.ID),
		fmt.Sprintf("telegram:quota:usage:%d", message.Chat.ID),
		fmt.Sprintf("telegram:quota:reset:%d", message.Chat.ID),
		fmt.Sprintf("telegram:unverified:count:%d", message.Chat.ID),
		fmt.Sprintf("telegram:expecting_input:%d", message.Chat.ID),
		fmt.Sprintf("telegram:expecting_sp_name:%d", message.Chat.ID),
		fmt.Sprintf("telegram:registration:step:%d", message.Chat.ID),
		fmt.Sprintf("telegram:registration:fullname:%d", message.Chat.ID),
		fmt.Sprintf("telegram:registration:username:%d", message.Chat.ID),
		fmt.Sprintf("telegram:registration:email:%d", message.Chat.ID),
		fmt.Sprintf("telegram:registration:phone:%d", message.Chat.ID),
		fmt.Sprintf("telegram:registration:usertype:%d", message.Chat.ID),
		fmt.Sprintf("telegram:registration:lang:%d", message.Chat.ID),
	}
	h.redis.Del(context.Background(), keys...)

	// Set language based on current LanguageCode
	h.setUserLanguage(message.From.ID, userLang)

	// Start registration again
	h.startRegistration(message.Chat.ID, message.From, userLang)

	msg := tgbotapi.NewMessage(message.Chat.ID, h.getLocalizedMessage(userLang, "reset_success"))
	msg.ReplyToMessageID = message.MessageID
	h.bot.Send(msg)
}

// HandleCommand processes commands sent by users based on their user type
func (h *TelegramHelper) HandleCommand(message *tgbotapi.Message) {
	command := message.Command()
	userLang := h.getUserLanguage(message.From.ID, message.From.LanguageCode)

	// Allow certain commands even for unverified users
	allowedCommands := []string{"start", "help", "reset"}
	isAllowedCommand := false
	for _, cmd := range allowedCommands {
		if command == cmd {
			isAllowedCommand = true
			break
		}
	}

	// Special handling for reset command - doesn't require user record
	if command == "reset" {
		h.handleResetCommand(message, userLang)
		return
	}

	if !isAllowedCommand {
		// For restricted commands, verify user and check quota/ban status
		allowed, reason := h.verifyAndCheckUser(message.From, message.Chat.ID)
		if !allowed {
			msg := tgbotapi.NewMessage(message.Chat.ID, reason)
			h.bot.Send(msg)
			return
		}
	}

	logrus.WithFields(logrus.Fields{
		"command": command,
		"chat_id": message.Chat.ID,
		"user":    message.From.UserName,
		"lang":    userLang,
	}).Info("Received command")

	// Store command message in database
	h.storeCommandMessage(message)

	// Send typing indicator before responding
	h.sendTypingAction(message.Chat.ID)

	// Get user information to determine available commands
	var telegramUser telegrammodel.TelegramUsers
	err := h.db.Where("telegram_chat_id = ?", message.Chat.ID).First(&telegramUser).Error
	if err != nil {
		// If user not found and command is start, start registration
		if command == "start" {
			h.startRegistration(message.Chat.ID, message.From, userLang)
			return
		}
		logrus.WithError(err).Error("Failed to get user information for command processing")
		msg := tgbotapi.NewMessage(message.Chat.ID, h.getLocalizedMessage(userLang, "database_error"))
		msg.ReplyToMessageID = message.MessageID
		h.bot.Send(msg)
		return
	}

	// Check if user is expecting input (WO, TID, etc.)
	expectedInput, err := h.redis.Get(context.Background(), fmt.Sprintf("telegram:expecting_input:%d", message.Chat.ID)).Result()
	if err == nil && expectedInput != "" {
		h.handleMSExpectedInput(message, expectedInput, userLang)
		return
	}

	// Check if user is expecting SP name input
	expectedSPType, err := h.redis.Get(context.Background(), fmt.Sprintf("telegram:expecting_sp_name:%d", message.Chat.ID)).Result()
	if err == nil && expectedSPType != "" {
		h.handleExpectedSPNameInput(message, expectedSPType, userLang)
		return
	}

	// Route commands based on user type
	h.routeCommand(message, telegramUser, userLang)

	// Increment user quota after successful command processing
	h.incrementUserQuota(message.Chat.ID)
}

// storeCommandMessage stores an incoming command message in the database, avoiding duplicates.
func (h *TelegramHelper) storeCommandMessage(message *tgbotapi.Message) {
	var replyToID *int64
	if message.ReplyToMessage != nil {
		replyID := int64(message.ReplyToMessage.MessageID)
		replyToID = &replyID
	}

	incomingMsg := telegrammodel.TelegramIncomingMsg{
		ChatID:      fmt.Sprintf("%d", message.Chat.ID),
		SenderID:    fmt.Sprintf("%d", message.From.ID),
		SenderName:  message.From.UserName,
		MessageBody: message.Text,
		MessageType: telegrammodel.TelegramTextMessage,
		IsGroup:     message.Chat.IsGroup() || message.Chat.IsSuperGroup(),
		ReceivedAt:  message.Time(),
		MessageID:   int64(message.MessageID),
		ReplyToID:   replyToID,
		MsgStatus:   "seen",
	}

	var existing telegrammodel.TelegramIncomingMsg
	if err := h.db.Where("telegram_chat_id = ? AND telegram_message_id = ?", incomingMsg.ChatID, incomingMsg.MessageID).First(&existing).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			if err := h.db.Create(&incomingMsg).Error; err != nil {
				logrus.WithError(err).Error("Failed to store incoming Telegram command")
			}
		} else {
			logrus.WithError(err).Error("Failed to check existing Telegram command")
		}
	}
}

// routeCommand dispatches the command to the correct handler based on its name.
func (h *TelegramHelper) routeCommand(message *tgbotapi.Message, telegramUser telegrammodel.TelegramUsers, userLang string) {
	command := message.Command()
	switch command {
	case "start":
		h.handleStartCommand(message, telegramUser, userLang)
	case "help":
		h.handleHelpCommand(message, telegramUser, userLang)
	case "input_wo":
		h.handleTechnicianInputWO(message, telegramUser, userLang)
	case "input_tid":
		h.handleTechnicianInputTID(message, telegramUser, userLang)
	case "info_tid":
		h.handleTechnicianInfoTID(message, telegramUser, userLang)
	case "generate_report_ta":
		h.handleTAGenerateReport(message, telegramUser, userLang)
	case "view_status_sp":
		h.handleHeadViewStatusSP(message, telegramUser, userLang)
	default:
		if h.isValidCommandForUserType(command, telegramUser.UserType) {
			msg := tgbotapi.NewMessage(message.Chat.ID, h.getLocalizedMessage(userLang, "command_not_implemented"))
			msg.ReplyToMessageID = message.MessageID
			h.bot.Send(msg)
		} else {
			msg := tgbotapi.NewMessage(message.Chat.ID, h.getLocalizedMessage(userLang, "unknown_command"))
			msg.ReplyToMessageID = message.MessageID
			h.bot.Send(msg)
		}
	}
}

func (h *TelegramHelper) handleTechnicianInputWO(message *tgbotapi.Message, telegramUser telegrammodel.TelegramUsers, userLang string) {
	if !h.canAccessCommand(
		telegramUser.UserType,
		[]telegrammodel.TelegramUserType{
			telegrammodel.SuperUser,
			telegrammodel.TechnicianMS,
			telegrammodel.SPLMS,
			telegrammodel.SACMS,
			// telegrammodel.HeadMS,
		},
	) {
		msg := tgbotapi.NewMessage(message.Chat.ID, h.getLocalizedMessage(userLang, "access_denied"))
		msg.ReplyToMessageID = message.MessageID
		h.bot.Send(msg)
		return
	}

	// TODO: Implement WO input logic
	// - Store state in Redis to expect WO number in next message
	// - Set expectation for WO input
	// - Send prompt message asking for WO number

	key := fmt.Sprintf("telegram:expecting_input:%d", message.Chat.ID)
	h.redis.Set(context.Background(), key, "wo_number", 10*time.Minute) // Expect input for 10 minutes

	msg := tgbotapi.NewMessage(message.Chat.ID, h.getLocalizedMessage(userLang, "input_wo_prompt"))
	msg.ReplyMarkup = tgbotapi.ForceReply{ForceReply: true, InputFieldPlaceholder: h.getLocalizedMessage(userLang, "input_wo_placeholder")}
	msg.ReplyToMessageID = message.MessageID
	h.bot.Send(msg)
}

// handleTechnicianInputTID handles TID input for technician MS
func (h *TelegramHelper) handleTechnicianInputTID(message *tgbotapi.Message, telegramUser telegrammodel.TelegramUsers, userLang string) {
	if !h.canAccessCommand(
		telegramUser.UserType,
		[]telegrammodel.TelegramUserType{
			telegrammodel.SuperUser,
			telegrammodel.TechnicianMS,
			telegrammodel.SPLMS,
			telegrammodel.SACMS,
			// telegrammodel.HeadMS,
		},
	) {
		msg := tgbotapi.NewMessage(message.Chat.ID, h.getLocalizedMessage(userLang, "access_denied"))
		msg.ReplyToMessageID = message.MessageID
		h.bot.Send(msg)
		return
	}

	// TODO: Implement TID input logic
	// - Store state in Redis to expect TID in next message
	// - Send prompt message asking for TID

	key := fmt.Sprintf("telegram:expecting_input:%d", message.Chat.ID)
	h.redis.Set(context.Background(), key, "tid_number", 10*time.Minute)

	msg := tgbotapi.NewMessage(message.Chat.ID, h.getLocalizedMessage(userLang, "input_tid_prompt"))
	msg.ReplyMarkup = tgbotapi.ForceReply{ForceReply: true, InputFieldPlaceholder: h.getLocalizedMessage(userLang, "input_tid_placeholder")}
	msg.ReplyToMessageID = message.MessageID
	h.bot.Send(msg)
}

// handleTechnicianInfoTID handles TID info request for technician MS
func (h *TelegramHelper) handleTechnicianInfoTID(message *tgbotapi.Message, telegramUser telegrammodel.TelegramUsers, userLang string) {
	if !h.canAccessCommand(
		telegramUser.UserType,
		[]telegrammodel.TelegramUserType{
			telegrammodel.SuperUser,
			telegrammodel.TechnicianMS,
			telegrammodel.SPLMS,
			telegrammodel.SACMS,
			// telegrammodel.HeadMS,
		},
	) {
		msg := tgbotapi.NewMessage(message.Chat.ID, h.getLocalizedMessage(userLang, "access_denied"))
		msg.ReplyToMessageID = message.MessageID
		h.bot.Send(msg)
		return
	}

	// TODO: Implement TID info logic
	// - Store state in Redis to expect TID for info lookup
	// - Send prompt message asking for TID to get merchant info

	key := fmt.Sprintf("telegram:expecting_input:%d", message.Chat.ID)
	h.redis.Set(context.Background(), key, "tid_info", 10*time.Minute)

	msg := tgbotapi.NewMessage(message.Chat.ID, h.getLocalizedMessage(userLang, "info_tid_prompt"))
	msg.ReplyMarkup = tgbotapi.ForceReply{ForceReply: true, InputFieldPlaceholder: h.getLocalizedMessage(userLang, "input_tid_info_placeholder")}
	msg.ReplyToMessageID = message.MessageID
	h.bot.Send(msg)
}

// handleTAGenerateReport handles report generation for TA MS
func (h *TelegramHelper) handleTAGenerateReport(message *tgbotapi.Message, telegramUser telegrammodel.TelegramUsers, userLang string) {
	if !h.canAccessCommand(
		telegramUser.UserType,
		[]telegrammodel.TelegramUserType{
			telegrammodel.SuperUser,
			telegrammodel.TAMS,
		},
	) {
		msg := tgbotapi.NewMessage(message.Chat.ID, h.getLocalizedMessage(userLang, "access_denied"))
		msg.ReplyToMessageID = message.MessageID
		h.bot.Send(msg)
		return
	}

	// TODO: Implement TA report generation logic
	// - Generate XLSX report for TA
	// - Send the file to user
	// - Handle report generation errors

	msg := tgbotapi.NewMessage(message.Chat.ID, h.getLocalizedMessage(userLang, "generating_report"))
	msg.ReplyToMessageID = message.MessageID
	h.bot.Send(msg)

	// TODO: Generate and send XLSX report
	// This would involve:
	// 1. Query database for TA report data
	// 2. Generate XLSX file
	// 3. Send file via Telegram
	// 4. Clean up temporary file
}

// handleHeadViewStatusSP handles status SP viewing for head MS
func (h *TelegramHelper) handleHeadViewStatusSP(message *tgbotapi.Message, telegramUser telegrammodel.TelegramUsers, userLang string) {
	if !h.canAccessCommand(
		telegramUser.UserType,
		[]telegrammodel.TelegramUserType{
			telegrammodel.SuperUser,
			telegrammodel.SPLMS,
			telegrammodel.SACMS,
			// telegrammodel.HeadMS,
		},
	) {
		msg := tgbotapi.NewMessage(message.Chat.ID, h.getLocalizedMessage(userLang, "access_denied"))
		msg.ReplyToMessageID = message.MessageID
		h.bot.Send(msg)
		return
	}

	// TODO: for SPL only can check spl or technician SP status
	// TODO: for SAC can check SP status for technician/spl/sac

	// TODO: Implement status SP viewing logic
	// - Show selection keyboard for technician/spl/sac
	// - After selection, prompt for name input
	// - Query and display status information

	keyboard := h.CreateStatusSPSelectionKeyboard(userLang)
	msg := tgbotapi.NewMessage(message.Chat.ID, h.getLocalizedMessage(userLang, "select_sp_type"))
	msg.ReplyMarkup = keyboard
	msg.ReplyToMessageID = message.MessageID
	h.bot.Send(msg)
}

// handleMSExpectedInput processes user input when expecting WO/TID/etc.
func (h *TelegramHelper) handleMSExpectedInput(message *tgbotapi.Message, inputType string, userLang string) {
	// Clear the expectation
	config.ManageService.MustInit("manage-service") // Load config manage-service.%s.yaml
	cfg := config.ManageService.Get()

	key := fmt.Sprintf("telegram:expecting_input:%d", message.Chat.ID)
	h.redis.Del(context.Background(), key)

	userInput := strings.TrimSpace(message.Text)

	switch inputType {
	case "wo_number":
		if userInput == "" {
			msg := tgbotapi.NewMessage(message.Chat.ID, h.getLocalizedMessage(userLang, "wo_number_empty"))
			msg.ReplyToMessageID = message.MessageID
			h.bot.Send(msg)
			return
		}

		responseMsg := fmt.Sprintf(h.getLocalizedMessage(userLang, "wo_number_received"), userInput)
		msg := tgbotapi.NewMessage(message.Chat.ID, responseMsg)
		msg.ReplyToMessageID = message.MessageID
		h.bot.Send(msg)

		processingWOMsg := tgbotapi.NewMessage(message.Chat.ID, fmt.Sprintf(h.getLocalizedMessage(userLang, "wo_number_processing"), userInput))
		processingWOMsg.ReplyToMessageID = message.MessageID
		h.bot.Send(processingWOMsg)

		// Create temporary ODOOMS helper to check WO status
		tempHelper := odoomscontrollers.NewODOOMSAPIHelper(&cfg, database.GetDBTA(), database.GetDBMS())
		resultWOFromTA := tempHelper.CheckWONumberStatusInTAMS(userInput)
		if len(resultWOFromTA) > 0 {
			msgResult := tgbotapi.NewMessage(message.Chat.ID, resultWOFromTA[userLang])
			msgResult.ReplyToMessageID = message.MessageID
			msgResult.ParseMode = "HTML"
			h.bot.Send(msgResult)
		}

	case "tid_number":
		// TODO: Process TID number input
		// - Validate TID format
		// - Store TID number for user
		// - Send confirmation

		if userInput == "" {
			msg := tgbotapi.NewMessage(message.Chat.ID, h.getLocalizedMessage(userLang, "tid_number_empty"))
			msg.ReplyToMessageID = message.MessageID
			h.bot.Send(msg)
			return
		}

		// TODO: Validate and process TID number
		responseMsg := fmt.Sprintf(h.getLocalizedMessage(userLang, "tid_number_received"), userInput)
		msg := tgbotapi.NewMessage(message.Chat.ID, responseMsg)
		msg.ReplyToMessageID = message.MessageID
		h.bot.Send(msg)

	case "tid_info":
		// TODO: Process TID info request
		// - Validate TID format
		// - Query merchant information for TID
		// - Send merchant details

		if userInput == "" {
			msg := tgbotapi.NewMessage(message.Chat.ID, h.getLocalizedMessage(userLang, "tid_info_empty"))
			msg.ReplyToMessageID = message.MessageID
			h.bot.Send(msg)
			return
		}

		// TODO: Query and return TID information
		responseMsg := fmt.Sprintf(h.getLocalizedMessage(userLang, "tid_info_processing"), userInput)
		msg := tgbotapi.NewMessage(message.Chat.ID, responseMsg)
		msg.ReplyToMessageID = message.MessageID
		h.bot.Send(msg)
		// TODO: Send actual merchant information

	default:
		msg := tgbotapi.NewMessage(message.Chat.ID, h.getLocalizedMessage(userLang, "unknown_input_type"))
		msg.ReplyToMessageID = message.MessageID
		h.bot.Send(msg)
	}
}

// handleExpectedSPNameInput processes SP name input for status viewing
func (h *TelegramHelper) handleExpectedSPNameInput(message *tgbotapi.Message, spType string, userLang string) {
	// Clear the expectation
	key := fmt.Sprintf("telegram:expecting_sp_name:%d", message.Chat.ID)
	h.redis.Del(context.Background(), key)

	spName := strings.TrimSpace(message.Text)

	if spName == "" {
		msg := tgbotapi.NewMessage(message.Chat.ID, h.getLocalizedMessage(userLang, "sp_name_empty"))
		msg.ReplyToMessageID = message.MessageID
		h.bot.Send(msg)
		return
	}

	// TODO: Process SP name input
	// - Validate SP name format
	// - Query status information based on SP type and name
	// - Send status details

	responseMsg := fmt.Sprintf(h.getLocalizedMessage(userLang, "sp_status_processing"), spType, spName)
	msg := tgbotapi.NewMessage(message.Chat.ID, responseMsg)
	msg.ReplyToMessageID = message.MessageID
	h.bot.Send(msg)
	// TODO: Query and send actual SP status information
}
