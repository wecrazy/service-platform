package telegramcontrollers

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/go-playground/validator/v10"
	"github.com/go-redis/redis/v8"
	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"github.com/nyaruka/phonenumbers"
	"github.com/sirupsen/logrus"
	"gorm.io/gorm"

	"service-platform/internal/config"
	telegrammodel "service-platform/internal/core/model/telegram_model"
	"service-platform/internal/pkg/fun"
	pb "service-platform/proto"
)

const (
	maxUnverifiedMessagesSendByTelegram = 25 // Max unverified messages to send
	maxDailyQuotaForTelegramMsg         = 100
)

// TelegramHelper contains helper functions for Telegram bot operations
type TelegramHelper struct {
	bot         *tgbotapi.BotAPI
	redis       *redis.Client
	db          *gorm.DB
	config      *config.YamlConfig
	defaultLang string
	validator   *validator.Validate
}

// NewTelegramHelper creates a new TelegramHelper instance
func NewTelegramHelper(bot *tgbotapi.BotAPI, redis *redis.Client, db *gorm.DB, config *config.YamlConfig, defaultLang string) *TelegramHelper {
	return &TelegramHelper{
		bot:         bot,
		redis:       redis,
		db:          db,
		config:      config,
		defaultLang: defaultLang,
		validator:   validator.New(),
	}
}

// sendTypingAction sends a typing indicator to the chat
func (h *TelegramHelper) sendTypingAction(chatID int64) {
	action := tgbotapi.NewChatAction(chatID, tgbotapi.ChatTyping)
	h.bot.Send(action)
}

// verifyAndCheckUser verifies user registration and checks quota/ban status
func (h *TelegramHelper) verifyAndCheckUser(user *tgbotapi.User, chatID int64) (bool, string) {
	// Get user's language preference
	userLang := h.getUserLanguage(user.ID)

	// Try to find existing user
	var telegramUser telegrammodel.TelegramUsers
	err := h.db.Where("telegram_chat_id = ?", chatID).First(&telegramUser).Error

	if err != nil {
		if err == gorm.ErrRecordNotFound {
			// User doesn't exist, create new user (unverified by default)
			newUser := telegrammodel.TelegramUsers{
				ChatID:        &chatID,
				FullName:      user.FirstName + " " + user.LastName,
				Username:      user.UserName,
				UserType:      telegrammodel.CommonUser,
				UserOf:        telegrammodel.CompanyEmployee,
				IsBanned:      false,
				VerifiedUser:  false, // New users start as unverified
				MaxDailyQuota: 10,    // Default quota
			}

			if err := h.db.Create(&newUser).Error; err != nil {
				logrus.WithError(err).Error("Failed to create new Telegram user")
				return false, h.getLocalizedMessage(userLang, "user_registration_failed")
			}

			return false, h.getLocalizedMessage(userLang, "user_created_unverified", h.config.Telegram.TechnicalSupportPhone)
		} else {
			logrus.WithError(err).Error("Failed to check user")
			return false, h.getLocalizedMessage(userLang, "database_error")
		}
	}

	// Check if user is banned
	if telegramUser.IsBanned {
		return false, h.getLocalizedMessage(userLang, "user_banned")
	}

	// Check if user is verified
	if !telegramUser.VerifiedUser {
		// Check if we've already sent the unverified message too many times
		unverifiedCount, err := h.getUnverifiedMessageCount(chatID)
		if err != nil {
			logrus.WithError(err).Error("Failed to get unverified message count")
			// Continue with sending message if we can't check
		}

		if unverifiedCount >= maxUnverifiedMessagesSendByTelegram {
			// User has been warned enough, don't send another message
			return false, ""
		}

		// Increment the counter and send the message
		h.incrementUnverifiedMessageCount(chatID)
		return false, h.getLocalizedMessage(userLang, "user_unverified", h.config.Telegram.TechnicalSupportPhone)
	}

	// Check quota using Redis
	if h.isQuotaExceeded(chatID, telegramUser.MaxDailyQuota) {
		usageCount, _ := h.getUserQuotaUsage(chatID)
		timeRemaining := h.getTimeUntilQuotaReset(userLang)

		quotaMsg := h.getLocalizedMessage(userLang, "quota_exceeded")
		timeMsg := h.getLocalizedMessage(userLang, "time_remaining")

		fullMsg := fmt.Sprintf(quotaMsg, usageCount, telegramUser.MaxDailyQuota)
		if timeRemaining != "" {
			fullMsg += "\n" + fmt.Sprintf(timeMsg, timeRemaining)
		}

		return false, fullMsg
	}

	return true, ""
}

// isQuotaExceeded checks if user's daily quota is exceeded using Redis
func (h *TelegramHelper) isQuotaExceeded(chatID int64, maxQuota int) bool {
	resetKey := fmt.Sprintf("telegram:quota:reset:%d", chatID)

	// Check if we need to reset quota (new day)
	resetTimeStr, err := h.redis.Get(context.Background(), resetKey).Result()
	if err != nil || resetTimeStr == "" {
		// No reset time set, initialize
		h.resetUserQuota(chatID)
		return false
	}

	resetTime, err := time.Parse(time.RFC3339, resetTimeStr)
	if err != nil {
		// Invalid reset time, reset
		h.resetUserQuota(chatID)
		return false
	}

	now := time.Now()
	if !h.isSameDay(resetTime, now) {
		// New day, reset quota
		h.resetUserQuota(chatID)
		return false
	}

	// Get current usage
	usageCount, err := h.getUserQuotaUsage(chatID)
	if err != nil {
		logrus.WithError(err).Error("Failed to get user quota usage")
		return false // Allow usage if we can't check
	}

	return usageCount >= maxQuota
}

// isSameDay checks if two timestamps are on the same day
func (h *TelegramHelper) isSameDay(t1, t2 time.Time) bool {
	y1, m1, d1 := t1.Date()
	y2, m2, d2 := t2.Date()
	return y1 == y2 && m1 == m2 && d1 == d2
}

// resetUserQuota resets user's daily quota in Redis
func (h *TelegramHelper) resetUserQuota(chatID int64) {
	usageKey := fmt.Sprintf("telegram:quota:usage:%d", chatID)
	resetKey := fmt.Sprintf("telegram:quota:reset:%d", chatID)

	now := time.Now()

	// Set usage to 0 and reset time to now
	h.redis.Set(context.Background(), usageKey, 0, 25*time.Hour) // Expire after 25 hours to be safe
	h.redis.Set(context.Background(), resetKey, now.Format(time.RFC3339), 25*time.Hour)
}

// getUserQuotaUsage gets current usage count from Redis
func (h *TelegramHelper) getUserQuotaUsage(chatID int64) (int, error) {
	usageKey := fmt.Sprintf("telegram:quota:usage:%d", chatID)

	usageStr, err := h.redis.Get(context.Background(), usageKey).Result()
	if err != nil {
		if err == redis.Nil {
			return 0, nil // No usage recorded yet
		}
		return 0, err
	}

	usage, err := strconv.Atoi(usageStr)
	if err != nil {
		return 0, err
	}

	return usage, nil
}

// incrementUserQuota increments user's daily usage count in Redis
func (h *TelegramHelper) incrementUserQuota(chatID int64) {
	usageKey := fmt.Sprintf("telegram:quota:usage:%d", chatID)

	// Use Redis INCR for atomic increment
	_, err := h.redis.Incr(context.Background(), usageKey).Result()
	if err != nil {
		logrus.WithError(err).Error("Failed to increment user quota in Redis")
	}
}

// getUnverifiedMessageCount gets the count of unverified messages sent to user from Redis
func (h *TelegramHelper) getUnverifiedMessageCount(chatID int64) (int, error) {
	key := fmt.Sprintf("telegram:unverified:count:%d", chatID)

	countStr, err := h.redis.Get(context.Background(), key).Result()
	if err != nil {
		if err == redis.Nil {
			return 0, nil // No count recorded yet
		}
		return 0, err
	}

	count, err := strconv.Atoi(countStr)
	if err != nil {
		return 0, err
	}

	return count, nil
}

// incrementUnverifiedMessageCount increments the unverified message count for user in Redis
func (h *TelegramHelper) incrementUnverifiedMessageCount(chatID int64) {
	key := fmt.Sprintf("telegram:unverified:count:%d", chatID)

	// Use Redis INCR for atomic increment, expire after 24 hours
	_, err := h.redis.Incr(context.Background(), key).Result()
	if err != nil {
		logrus.WithError(err).Error("Failed to increment unverified message count in Redis")
	} else {
		// Set expiration to 24 hours if this is the first increment
		h.redis.Expire(context.Background(), key, 24*time.Hour)
	}
}

// getTimeUntilQuotaReset calculates and formats the time remaining until quota reset
func (h *TelegramHelper) getTimeUntilQuotaReset(lang string) string {
	now := time.Now()

	// Calculate next midnight (quota reset time)
	nextMidnight := time.Date(now.Year(), now.Month(), now.Day()+1, 0, 0, 0, 0, now.Location())

	// Calculate duration until reset
	duration := nextMidnight.Sub(now)

	// Extract hours, minutes, seconds
	hours := int(duration.Hours())
	minutes := int(duration.Minutes()) % 60
	seconds := int(duration.Seconds()) % 60

	// Build the time string with localized units
	var timeParts []string

	if hours > 0 {
		timeParts = append(timeParts, fmt.Sprintf("%d %s", hours, h.getLocalizedMessage(lang, "hours")))
	}
	if minutes > 0 {
		timeParts = append(timeParts, fmt.Sprintf("%d %s", minutes, h.getLocalizedMessage(lang, "minutes")))
	}
	if seconds > 0 || (hours == 0 && minutes == 0) { // Always show seconds if no hours/minutes
		timeParts = append(timeParts, fmt.Sprintf("%d %s", seconds, h.getLocalizedMessage(lang, "seconds")))
	}

	return strings.Join(timeParts, " ")
}

// markMessageAsSeen updates the message status in database to "seen"
func (h *TelegramHelper) markMessageAsSeen(messageID int64, chatID string) {
	err := h.db.Model(&telegrammodel.TelegramIncomingMsg{}).
		Where("telegram_message_id = ? AND telegram_chat_id = ?", messageID, chatID).
		Update("telegram_msg_status", "seen").Error
	if err != nil {
		logrus.WithError(err).Error("Failed to mark message as seen")
	}
}

// BuildInlineKeyboard constructs an InlineKeyboardMarkup from the protobuf definition.
func (h *TelegramHelper) BuildInlineKeyboard(keyboard *pb.InlineKeyboardMarkup) tgbotapi.InlineKeyboardMarkup {
	var rows [][]tgbotapi.InlineKeyboardButton
	for _, row := range keyboard.InlineKeyboard {
		var buttons []tgbotapi.InlineKeyboardButton
		for _, button := range row.Buttons {
			btn := tgbotapi.NewInlineKeyboardButtonData(button.Text, button.CallbackData)
			if button.Url != "" {
				btn = tgbotapi.NewInlineKeyboardButtonURL(button.Text, button.Url)
			}
			buttons = append(buttons, btn)
		}
		rows = append(rows, buttons)
	}
	return tgbotapi.InlineKeyboardMarkup{InlineKeyboard: rows}
}

// getUserLanguage gets the user's preferred language from Redis, with auto-detection fallback
func (h *TelegramHelper) getUserLanguage(userID int64, detectedLangCode ...string) string {
	key := fmt.Sprintf("telegram:user:lang:%d", userID)
	lang, err := h.redis.Get(context.Background(), key).Result()
	if err == nil && lang != "" {
		// Language already set in Redis
		return lang
	}

	// Language not set, try to auto-detect
	var selectedLang string
	if len(detectedLangCode) > 0 && detectedLangCode[0] != "" {
		detected := detectedLangCode[0]

		// Normalize the detected language code (handle aliases like "ja" -> "jp")
		normalized := fun.NormalizeLanguageCode(detected)

		// Check if normalized language is supported
		if fun.IsSupportedLanguage(normalized) {
			selectedLang = normalized
		} else {
			selectedLang = h.defaultLang
		}
	} else {
		selectedLang = h.defaultLang
	}

	// Store the selected language in Redis for future use
	h.redis.Set(context.Background(), key, selectedLang, 24*time.Hour)

	return selectedLang
}

// setUserLanguage sets the user's preferred language in Redis with 24h expiration
func (h *TelegramHelper) setUserLanguage(userID int64, lang string) error {
	key := fmt.Sprintf("telegram:user:lang:%d", userID)
	return h.redis.Set(context.Background(), key, lang, 24*time.Hour).Err()
}

// HandleUpdate processes incoming updates from Telegram
func (h *TelegramHelper) HandleUpdate(update tgbotapi.Update) {
	if update.Message != nil {
		h.HandleMessage(update.Message)
	} else if update.CallbackQuery != nil {
		h.HandleCallbackQuery(update.CallbackQuery)
	}
}

// HandleMessage processes incoming messages from users
func (h *TelegramHelper) HandleMessage(message *tgbotapi.Message) {
	// Check if this is a completely new user (no record in database)
	var telegramUser telegrammodel.TelegramUsers
	err := h.db.Where("telegram_chat_id = ?", message.Chat.ID).First(&telegramUser).Error

	if err != nil && err == gorm.ErrRecordNotFound {
		// Completely new user - show start menu instead of verification message
		userLang := h.getUserLanguage(message.From.ID, message.From.LanguageCode)
		welcomeMsg := h.getLocalizedMessage(userLang, "welcome_new_user")
		keyboard := h.CreateStartKeyboard(userLang)

		msg := tgbotapi.NewMessage(message.Chat.ID, welcomeMsg)
		msg.ReplyMarkup = keyboard
		msg.ParseMode = "Markdown"
		h.bot.Send(msg)

		// Log the new interaction
		logrus.WithFields(logrus.Fields{
			"chat_id": message.Chat.ID,
			"user":    message.From.UserName,
			"action":  "new_user_welcome",
		}).Info("Sent welcome message to new user")

		return
	}

	// Check if in registration process
	step, err := h.redis.Get(context.Background(), fmt.Sprintf("telegram:registration:step:%d", message.Chat.ID)).Result()
	if err == nil && step != "" {
		h.handleRegistrationStep(message, step)
		return
	}

	// Existing user - verify and check quota/ban status
	allowed, reason := h.verifyAndCheckUser(message.From, message.Chat.ID)
	if !allowed {
		msg := tgbotapi.NewMessage(message.Chat.ID, reason)
		h.bot.Send(msg)
		return
	}

	if message.IsCommand() {
		h.HandleCommand(message)
	} else {
		// Handle regular messages if needed
		userLang := h.getUserLanguage(message.From.ID, message.From.LanguageCode)
		logrus.WithFields(logrus.Fields{
			"chat_id": message.Chat.ID,
			"user":    message.From.UserName,
			"text":    message.Text,
			"lang":    userLang,
		}).Info("Received message")

		// Determine message type and body
		var msgType telegrammodel.TelegramMessageType
		var msgBody string

		if message.Text != "" {
			msgType = telegrammodel.TelegramTextMessage
			msgBody = message.Text
		} else if len(message.Photo) > 0 {
			msgType = telegrammodel.TelegramImageMessage
			msgBody = message.Caption // Use caption as body for images
			if msgBody == "" {
				msgBody = "Image"
			}
		} else if message.Document != nil {
			msgType = telegrammodel.TelegramDocumentMessage
			msgBody = message.Document.FileName
			if message.Caption != "" {
				msgBody = message.Caption
			}
		} else if message.Video != nil {
			msgType = telegrammodel.TelegramVideoMessage
			msgBody = message.Caption
			if msgBody == "" {
				msgBody = "Video"
			}
		} else if message.Audio != nil {
			msgType = telegrammodel.TelegramAudioMessage
			msgBody = message.Audio.Title
			if msgBody == "" && message.Audio.FileName != "" {
				msgBody = message.Audio.FileName
			}
			if msgBody == "" {
				msgBody = "Audio"
			}
		} else if message.Sticker != nil {
			msgType = telegrammodel.TelegramStickerMessage
			msgBody = message.Sticker.Emoji
		} else if message.Location != nil {
			msgType = telegrammodel.TelegramLocationMessage
			msgBody = fmt.Sprintf("Location: %.6f, %.6f", message.Location.Latitude, message.Location.Longitude)
		} else if message.Contact != nil {
			msgType = telegrammodel.TelegramContactMessage
			// Extract phone number and other contact details
			phoneNumber := message.Contact.PhoneNumber
			firstName := message.Contact.FirstName
			lastName := message.Contact.LastName

			msgBody = fmt.Sprintf("Contact: %s %s, Phone: %s", firstName, lastName, phoneNumber)

			// You can now use the phone number for verification or other purposes
			logrus.WithFields(logrus.Fields{
				"chat_id":    message.Chat.ID,
				"user_id":    message.From.ID,
				"phone":      phoneNumber,
				"first_name": firstName,
				"last_name":  lastName,
			}).Info("Received contact information")
		} else {
			msgType = telegrammodel.TelegramTextMessage
			msgBody = "Unsupported message type"
		}

		// Store incoming message in database
		var replyToID *int64
		if message.ReplyToMessage != nil {
			replyID := int64(message.ReplyToMessage.MessageID)
			replyToID = &replyID
		}

		incomingMsg := telegrammodel.TelegramIncomingMsg{
			ChatID:      fmt.Sprintf("%d", message.Chat.ID),
			SenderID:    fmt.Sprintf("%d", message.From.ID),
			SenderName:  message.From.UserName,
			MessageBody: msgBody,
			MessageType: msgType,
			IsGroup:     message.Chat.IsGroup() || message.Chat.IsSuperGroup(),
			ReceivedAt:  message.Time(),
			MessageID:   int64(message.MessageID),
			ReplyToID:   replyToID,
			MsgStatus:   "seen", // Mark as seen immediately
		}

		// Check if message already exists to prevent duplicates
		var existing telegrammodel.TelegramIncomingMsg
		if err := h.db.Where("telegram_chat_id = ? AND telegram_message_id = ?", incomingMsg.ChatID, incomingMsg.MessageID).First(&existing).Error; err != nil {
			if err == gorm.ErrRecordNotFound {
				if err := h.db.Create(&incomingMsg).Error; err != nil {
					logrus.WithError(err).Error("Failed to store incoming Telegram message")
				}
			} else {
				logrus.WithError(err).Error("Failed to check existing Telegram message")
			}
		} // else message already exists, skip

		// Send typing indicator before responding
		h.sendTypingAction(message.Chat.ID)

		// Quick response (remove delay for better UX)
		// time.Sleep(1 * time.Second)

		// Acknowledge receipt in user's language
		responseText := fmt.Sprintf(h.getLocalizedMessage(userLang, "message_received"), msgBody)
		msg := tgbotapi.NewMessage(message.Chat.ID, responseText)
		h.bot.Send(msg)

		// Increment user quota after successful processing
		h.incrementUserQuota(message.Chat.ID)
	}
}

// HandleCommand processes commands sent by users
func (h *TelegramHelper) HandleCommand(message *tgbotapi.Message) {
	command := message.Command()
	userLang := h.getUserLanguage(message.From.ID, message.From.LanguageCode)

	// Allow certain commands even for unverified users
	allowedCommands := []string{"start", "help"}
	isAllowedCommand := false
	for _, cmd := range allowedCommands {
		if command == cmd {
			isAllowedCommand = true
			break
		}
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
	var replyToID *int64
	if message.ReplyToMessage != nil {
		replyID := int64(message.ReplyToMessage.MessageID)
		replyToID = &replyID
	}

	incomingMsg := telegrammodel.TelegramIncomingMsg{
		ChatID:      fmt.Sprintf("%d", message.Chat.ID),
		SenderID:    fmt.Sprintf("%d", message.From.ID),
		SenderName:  message.From.UserName,
		MessageBody: message.Text, // Store the full command text
		MessageType: telegrammodel.TelegramTextMessage,
		IsGroup:     message.Chat.IsGroup() || message.Chat.IsSuperGroup(),
		ReceivedAt:  message.Time(),
		MessageID:   int64(message.MessageID),
		ReplyToID:   replyToID,
		MsgStatus:   "seen", // Mark as seen immediately
	}

	// Check if command message already exists to prevent duplicates
	var existing telegrammodel.TelegramIncomingMsg
	if err := h.db.Where("telegram_chat_id = ? AND telegram_message_id = ?", incomingMsg.ChatID, incomingMsg.MessageID).First(&existing).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			if err := h.db.Create(&incomingMsg).Error; err != nil {
				logrus.WithError(err).Error("Failed to store incoming Telegram command")
			}
		} else {
			logrus.WithError(err).Error("Failed to check existing Telegram command")
		}
	} // else command already exists, skip

	// // Send immediate acknowledgment
	// ackMsg := tgbotapi.NewMessage(message.Chat.ID, "✅")
	// h.bot.Send(ackMsg)

	// Send typing indicator before responding
	h.sendTypingAction(message.Chat.ID)

	// Quick acknowledgment (remove delay for better UX)
	// time.Sleep(500 * time.Millisecond)

	switch command {
	case "start":
		// Check if user exists and is verified
		var telegramUser telegrammodel.TelegramUsers
		err := h.db.Where("telegram_chat_id = ?", message.Chat.ID).First(&telegramUser).Error
		if err != nil {
			if err == gorm.ErrRecordNotFound {
				// Create new user record (unverified by default)
				chatID := message.Chat.ID
				newUser := telegrammodel.TelegramUsers{
					ChatID:        &chatID,
					FullName:      message.From.FirstName + " " + message.From.LastName,
					Username:      message.From.UserName,
					UserType:      telegrammodel.CommonUser,
					UserOf:        telegrammodel.CompanyEmployee,
					IsBanned:      false,
					VerifiedUser:  false,
					MaxDailyQuota: maxDailyQuotaForTelegramMsg,
				}
				if err := h.db.Create(&newUser).Error; err != nil {
					logrus.WithError(err).Error("Failed to create new Telegram user")
					msg := tgbotapi.NewMessage(message.Chat.ID, h.getLocalizedMessage(userLang, "database_error"))
					h.bot.Send(msg)
					return
				}
				telegramUser = newUser
			} else {
				logrus.WithError(err).Error("Failed to check Telegram user")
				msg := tgbotapi.NewMessage(message.Chat.ID, h.getLocalizedMessage(userLang, "database_error"))
				h.bot.Send(msg)
				return
			}
		}

		if !telegramUser.VerifiedUser {
			// Start registration process
			h.startRegistration(message.Chat.ID, message.From, userLang)
			return
		}

		// User is verified, show start menu
		keyboard := h.CreateStartKeyboard(userLang)
		welcomeMsg := h.getLocalizedMessage(userLang, "welcome")
		msg := tgbotapi.NewMessage(message.Chat.ID, welcomeMsg)
		msg.ReplyMarkup = keyboard
		h.bot.Send(msg)

	case "help":
		// Create inline keyboard for help menu
		keyboard := h.CreateHelpKeyboard(userLang)
		msg := tgbotapi.NewMessage(message.Chat.ID, h.getLocalizedMessage(userLang, "help"))
		msg.ReplyMarkup = keyboard
		h.bot.Send(msg)

	default:
		msg := tgbotapi.NewMessage(message.Chat.ID, h.getLocalizedMessage(userLang, "unknown_command"))
		h.bot.Send(msg)
	}

	// Increment user quota after successful command processing
	h.incrementUserQuota(message.Chat.ID)
}

// HandleCallbackQuery processes callback queries from inline keyboard buttons
func (h *TelegramHelper) HandleCallbackQuery(callback *tgbotapi.CallbackQuery) {
	userLang := h.getUserLanguage(callback.From.ID, callback.From.LanguageCode)

	// Allow certain actions even for unverified users
	allowedActions := []string{"info", "commands", "language", "start", "help", "lang", "usertype"}
	isAllowedAction := false
	for _, action := range allowedActions {
		if strings.HasPrefix(callback.Data, action) {
			isAllowedAction = true
			break
		}
	}

	if !isAllowedAction {
		// For restricted actions, verify user and check quota/ban status
		allowed, reason := h.verifyAndCheckUser(callback.From, callback.Message.Chat.ID)
		if !allowed {
			// For callback queries, we need to answer them even if user is not allowed
			answer := tgbotapi.NewCallback(callback.ID, reason)
			answer.ShowAlert = true
			h.bot.Request(answer)
			return
		}
	}

	// Check if in registration process
	step, err := h.redis.Get(context.Background(), fmt.Sprintf("telegram:registration:step:%d", callback.Message.Chat.ID)).Result()
	if err == nil && step != "" {
		h.handleRegistrationCallback(callback, step)
		return
	}

	logrus.WithFields(logrus.Fields{
		"callback_data": callback.Data,
		"chat_id":       callback.Message.Chat.ID,
		"user":          callback.From.UserName,
		"lang":          userLang,
	}).Info("Received callback query")

	// // Answer the callback query
	// // You can customize the text if needed, e.g. "Processing..."
	// answer := tgbotapi.NewCallback(callback.ID, "")
	// h.bot.Request(answer)

	// Send typing indicator before responding
	h.sendTypingAction(callback.Message.Chat.ID)

	// Quick response (remove delay for better UX)
	// time.Sleep(300 * time.Millisecond)

	// Handle the callback data
	switch callback.Data {
	case "start":
		// Check if user exists and is verified
		var telegramUser telegrammodel.TelegramUsers
		err := h.db.Where("telegram_chat_id = ?", callback.Message.Chat.ID).First(&telegramUser).Error
		if err != nil {
			if err == gorm.ErrRecordNotFound {
				// Create new user record (unverified by default)
				chatID := callback.Message.Chat.ID
				newUser := telegrammodel.TelegramUsers{
					ChatID:        &chatID,
					FullName:      callback.From.FirstName + " " + callback.From.LastName,
					Username:      callback.From.UserName,
					UserType:      telegrammodel.CommonUser,
					UserOf:        telegrammodel.CompanyEmployee,
					IsBanned:      false,
					VerifiedUser:  false,
					MaxDailyQuota: maxDailyQuotaForTelegramMsg,
				}
				if err := h.db.Create(&newUser).Error; err != nil {
					logrus.WithError(err).Error("Failed to create new Telegram user")
					editMsg := tgbotapi.NewEditMessageText(callback.Message.Chat.ID, callback.Message.MessageID, h.getLocalizedMessage(userLang, "database_error"))
					h.bot.Send(editMsg)
					return
				}
				telegramUser = newUser
			} else {
				logrus.WithError(err).Error("Failed to check Telegram user")
				editMsg := tgbotapi.NewEditMessageText(callback.Message.Chat.ID, callback.Message.MessageID, h.getLocalizedMessage(userLang, "database_error"))
				h.bot.Send(editMsg)
				return
			}
		}

		if !telegramUser.VerifiedUser {
			// Start registration process
			h.startRegistration(callback.Message.Chat.ID, callback.From, userLang)
			return
		}

		// User is verified, show start menu
		keyboard := h.CreateStartKeyboard(userLang)
		editMsg := tgbotapi.NewEditMessageText(callback.Message.Chat.ID, callback.Message.MessageID, h.getLocalizedMessage(userLang, "welcome"))
		editMsg.ReplyMarkup = &keyboard
		h.bot.Send(editMsg)

	case "help":
		keyboard := h.CreateHelpKeyboard(userLang)
		editMsg := tgbotapi.NewEditMessageText(callback.Message.Chat.ID, callback.Message.MessageID, h.getLocalizedMessage(userLang, "help"))
		editMsg.ReplyMarkup = &keyboard
		h.bot.Send(editMsg)

	case "info":
		editMsg := tgbotapi.NewEditMessageText(callback.Message.Chat.ID, callback.Message.MessageID, h.getLocalizedMessage(userLang, "bot_info"))
		editMsg.ParseMode = "Markdown"
		h.bot.Send(editMsg)

	case "commands":
		editMsg := tgbotapi.NewEditMessageText(callback.Message.Chat.ID, callback.Message.MessageID, h.getLocalizedMessage(userLang, "commands_list"))
		editMsg.ParseMode = "Markdown"
		h.bot.Send(editMsg)

	case "language":
		keyboard := h.CreateLanguageKeyboard()
		editMsg := tgbotapi.NewEditMessageText(callback.Message.Chat.ID, callback.Message.MessageID, h.getLocalizedMessage(userLang, "select_language"))
		editMsg.ReplyMarkup = &keyboard
		h.bot.Send(editMsg)

	case "button1":
		editMsg := tgbotapi.NewEditMessageText(callback.Message.Chat.ID, callback.Message.MessageID, h.getLocalizedMessage(userLang, "button1"))
		h.bot.Send(editMsg)

	case "button2":
		editMsg := tgbotapi.NewEditMessageText(callback.Message.Chat.ID, callback.Message.MessageID, h.getLocalizedMessage(userLang, "button2"))
		h.bot.Send(editMsg)

	default:
		// Check if it's a language selection
		if strings.HasPrefix(callback.Data, "lang_") {
			langCode := strings.TrimPrefix(callback.Data, "lang_")

			// Check if user exists and is verified
			var telegramUser telegrammodel.TelegramUsers
			err := h.db.Where("telegram_chat_id = ?", callback.Message.Chat.ID).First(&telegramUser).Error
			if err != nil {
				if err == gorm.ErrRecordNotFound {
					// Create new user record
					fullName := callback.From.FirstName
					if callback.From.LastName != "" {
						fullName += " " + callback.From.LastName
					}
					newUser := telegrammodel.TelegramUsers{
						ChatID:        &callback.Message.Chat.ID,
						FullName:      fullName,
						Username:      callback.From.UserName,
						VerifiedUser:  false,
						IsBanned:      false,
						UserOf:        telegrammodel.CompanyEmployee,
						MaxDailyQuota: maxDailyQuotaForTelegramMsg,
					}
					if createErr := h.db.Create(&newUser).Error; createErr != nil {
						logrus.WithError(createErr).Error("Failed to create user for language selection")
						editMsg := tgbotapi.NewEditMessageText(callback.Message.Chat.ID, callback.Message.MessageID, h.getLocalizedMessage(userLang, "error"))
						h.bot.Send(editMsg)
						return
					}
					telegramUser = newUser
				} else {
					logrus.WithError(err).Error("Failed to find user for language selection")
					editMsg := tgbotapi.NewEditMessageText(callback.Message.Chat.ID, callback.Message.MessageID, h.getLocalizedMessage(userLang, "error"))
					h.bot.Send(editMsg)
					return
				}
			}

			if !telegramUser.VerifiedUser {
				// Start registration process
				h.startRegistration(callback.Message.Chat.ID, callback.From, langCode)
				return
			}

			err = h.setUserLanguage(callback.From.ID, langCode)
			if err != nil {
				logrus.WithError(err).Error("Failed to set user language")
				editMsg := tgbotapi.NewEditMessageText(callback.Message.Chat.ID, callback.Message.MessageID, h.getLocalizedMessage(userLang, "language_error"))
				h.bot.Send(editMsg)
				return
			}

			responseText := h.getLocalizedMessage(langCode, "language_set")
			editMsg := tgbotapi.NewEditMessageText(callback.Message.Chat.ID, callback.Message.MessageID, responseText)
			h.bot.Send(editMsg)
		} else if strings.HasPrefix(callback.Data, "react_") {
			// Handle reaction callback (e.g., "react_emoji_messageid")
			parts := strings.Split(callback.Data, "_")
			if len(parts) >= 3 {
				emoji := parts[1]
				messageIDStr := parts[2]
				messageID, err := strconv.ParseInt(messageIDStr, 10, 64)
				if err != nil {
					logrus.WithError(err).Error("Invalid message ID in reaction callback")
					return
				}

				// Update reaction in database
				if err := h.db.Model(&telegrammodel.TelegramIncomingMsg{}).
					Where("telegram_chat_id = ? AND telegram_message_id = ?", fmt.Sprintf("%d", callback.Message.Chat.ID), messageID).
					Updates(map[string]interface{}{
						"telegram_reaction_emoji": emoji,
						"telegram_reacted_by":     callback.From.UserName,
						"telegram_reacted_at":     time.Now(),
					}).Error; err != nil {
					logrus.WithError(err).Error("Failed to update Telegram reaction")
				}

				// Send confirmation
				reactionMsg := h.getLocalizedMessage(userLang, "reaction_added")
				responseText := fmt.Sprintf(reactionMsg, emoji)
				msg := tgbotapi.NewMessage(callback.Message.Chat.ID, responseText)
				h.bot.Send(msg)
			}
		} else {
			editMsg := tgbotapi.NewEditMessageText(callback.Message.Chat.ID, callback.Message.MessageID, h.getLocalizedMessage(userLang, "unknown_button"))
			h.bot.Send(editMsg)
		}
	}

	// Increment user quota after successful callback processing
	h.incrementUserQuota(callback.Message.Chat.ID)
}

// CreateStartKeyboard creates the inline keyboard for the start menu
func (h *TelegramHelper) CreateStartKeyboard(lang string) tgbotapi.InlineKeyboardMarkup {
	return tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData(h.getLocalizedMessage(lang, "start"), "start"),
			tgbotapi.NewInlineKeyboardButtonData(h.getLocalizedMessage(lang, "info"), "info"),
		),
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData(h.getLocalizedMessage(lang, "commands"), "commands"),
			tgbotapi.NewInlineKeyboardButtonData(h.getLocalizedMessage(lang, "language"), "language"),
		),
	)
}

// CreatePhoneRequestKeyboard creates a keyboard that requests user's phone number
func (h *TelegramHelper) CreatePhoneRequestKeyboard(lang string) tgbotapi.ReplyKeyboardMarkup {
	return tgbotapi.NewReplyKeyboard(
		tgbotapi.NewKeyboardButtonRow(
			tgbotapi.NewKeyboardButtonContact(h.getLocalizedMessage(lang, "share_phone")),
		),
		tgbotapi.NewKeyboardButtonRow(
			tgbotapi.NewKeyboardButton(h.getLocalizedMessage(lang, "cancel")),
		),
	)
}

// CreateHelpKeyboard creates the inline keyboard for the help menu
func (h *TelegramHelper) CreateHelpKeyboard(lang string) tgbotapi.InlineKeyboardMarkup {
	return tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData(h.getLocalizedMessage(lang, "start"), "start"),
			tgbotapi.NewInlineKeyboardButtonData(h.getLocalizedMessage(lang, "info"), "info"),
		),
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData(h.getLocalizedMessage(lang, "commands"), "commands"),
			tgbotapi.NewInlineKeyboardButtonData(h.getLocalizedMessage(lang, "language"), "language"),
		),
	)
}

// CreateLanguageKeyboard creates the inline keyboard for language selection
func (h *TelegramHelper) CreateLanguageKeyboard() tgbotapi.InlineKeyboardMarkup {
	return tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("🇮🇩 Indonesia", "lang_id"),
			tgbotapi.NewInlineKeyboardButtonData("🇺🇸 English", "lang_en"),
		),
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("🇪🇸 Español", "lang_es"),
			tgbotapi.NewInlineKeyboardButtonData("🇫🇷 Français", "lang_fr"),
		),
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("🇩🇪 Deutsch", "lang_de"),
			tgbotapi.NewInlineKeyboardButtonData("🇵🇹 Português", "lang_pt"),
		),
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("🇷🇺 Русский", "lang_ru"),
			tgbotapi.NewInlineKeyboardButtonData("🇯🇵 日本語", "lang_jp"),
		),
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("🇨🇳 中文", "lang_cn"),
			tgbotapi.NewInlineKeyboardButtonData("🇸🇦 العربية", "lang_ar"),
		),
	)
}

// isValidUserType validates if the given usertype string matches one of the defined TelegramUserType constants
func (h *TelegramHelper) isValidUserType(usertype string) bool {
	validTypes := []string{
		string(telegrammodel.CommonUser),
		string(telegrammodel.SuperUser),
		string(telegrammodel.TechnicianMS),
		string(telegrammodel.TAMS),
		string(telegrammodel.HeadMS),
	}

	for _, validType := range validTypes {
		if usertype == validType {
			return true
		}
	}
	return false
}

// getCountryCodeFromLanguage maps language code to country code for phone validation
func (h *TelegramHelper) getCountryCodeFromLanguage(lang string) string {
	countryMap := map[string]string{
		fun.LangID: "ID", // Indonesia
		fun.LangEN: "US", // United States (default for English)
		fun.LangES: "ES", // Spain
		fun.LangFR: "FR", // France
		fun.LangDE: "DE", // Germany
		fun.LangPT: "PT", // Portugal
		fun.LangRU: "RU", // Russia
		fun.LangJP: "JP", // Japan
		fun.LangCN: "CN", // China
		fun.LangAR: "SA", // Saudi Arabia
	}

	if country, exists := countryMap[lang]; exists {
		return country
	}
	return "US" // Default fallback
}

// validateAndFormatPhoneNumber validates and formats a phone number to E.164 format
func (h *TelegramHelper) validateAndFormatPhoneNumber(phoneNumber string, lang string) (string, error) {
	countryCode := h.getCountryCodeFromLanguage(lang)

	// Parse the phone number with country code
	num, err := phonenumbers.Parse(phoneNumber, countryCode)
	if err != nil {
		return "", fmt.Errorf("failed to parse phone number for country %s: %w", countryCode, err)
	}

	// Check if the number is valid
	if !phonenumbers.IsValidNumber(num) {
		return "", fmt.Errorf("invalid phone number for country %s", countryCode)
	}

	// Format to E.164
	formatted := phonenumbers.Format(num, phonenumbers.E164)
	return formatted, nil
}

// validateEmail validates an email address using the validator package
func (h *TelegramHelper) validateEmail(email string) error {
	return h.validator.Var(email, "email")
}

// CreateUsertypeKeyboard creates the inline keyboard for user type selection
func (h *TelegramHelper) CreateUsertypeKeyboard(lang string) tgbotapi.InlineKeyboardMarkup {
	return tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData(h.getLocalizedMessage(lang, "usertype_common"), "usertype_common"),
			tgbotapi.NewInlineKeyboardButtonData(h.getLocalizedMessage(lang, "usertype_super_user"), "usertype_super_user"),
		),
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData(h.getLocalizedMessage(lang, "usertype_technician_ms"), "usertype_technician_ms"),
			tgbotapi.NewInlineKeyboardButtonData(h.getLocalizedMessage(lang, "usertype_tams"), "usertype_tams"),
		),
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData(h.getLocalizedMessage(lang, "usertype_head_ms"), "usertype_head_ms"),
		),
	)
}
