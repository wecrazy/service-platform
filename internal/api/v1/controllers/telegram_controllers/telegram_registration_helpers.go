package telegramcontrollers

import (
	"context"
	"fmt"
	odoomscontrollers "service-platform/internal/api/v1/controllers/odooms_controllers"
	telegrammodel "service-platform/internal/core/model/telegram_model"
	"strconv"
	"strings"
	"time"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"github.com/nyaruka/phonenumbers"
	"github.com/sirupsen/logrus"
)

// clearRegistrationKeys deletes all temporary Redis keys used during the registration flow.
func (h *TelegramHelper) clearRegistrationKeys(chatID int64) {
	keys := []string{
		fmt.Sprintf("telegram:registration:step:%d", chatID),
		fmt.Sprintf("telegram:registration:fullname:%d", chatID),
		fmt.Sprintf("telegram:registration:username:%d", chatID),
		fmt.Sprintf("telegram:registration:email:%d", chatID),
		fmt.Sprintf("telegram:registration:phone:%d", chatID),
		fmt.Sprintf("telegram:registration:usertype:%d", chatID),
		fmt.Sprintf("telegram:registration:lang:%d", chatID),
	}
	h.redis.Del(context.Background(), keys...)
}

// startRegistration starts the user registration process
func (h *TelegramHelper) startRegistration(chatID int64, user *tgbotapi.User, langCode string) {
	// Store the intended language
	h.redis.Set(context.Background(), fmt.Sprintf("telegram:registration:lang:%d", chatID), langCode, time.Hour)

	// Set step to fullname
	h.redis.Set(context.Background(), fmt.Sprintf("telegram:registration:step:%d", chatID), "fullname", time.Hour)

	// Get current name
	currentName := user.FirstName
	if user.LastName != "" {
		currentName += " " + user.LastName
	}

	// Send message
	userLang := h.getUserLanguage(user.ID)
	msgText := h.getLocalizedMessage(userLang, "registration_fullname_prompt")
	msg := tgbotapi.NewMessage(chatID, msgText)
	msg.ReplyMarkup = tgbotapi.ForceReply{ForceReply: true, InputFieldPlaceholder: currentName}
	if _, err := h.bot.Send(msg); err != nil {
		logrus.WithError(err).Error("Failed to send registration fullname prompt")
	}
}

// handleRegistrationStep handles the registration steps for messages
func (h *TelegramHelper) handleRegistrationStep(message *tgbotapi.Message, step string) {
	userLang := h.getUserLanguage(message.From.ID, message.From.LanguageCode)
	switch step {
	case "fullname":
		h.handleFullnameStep(message, userLang)
	case "username":
		h.handleUsernameStep(message, userLang)
	case "email":
		h.handleEmailStep(message, userLang)
	case "phone":
		h.handlePhoneStep(message, userLang)
	case "usertype":
		h.handleUsertypeStep(message, userLang)
	}
}

// isRegistrationReset reports whether the message text is a /reset or /start command.
func isRegistrationReset(text string) bool {
	t := strings.TrimSpace(text)
	return t == "/reset" || t == "/start"
}

// handleFullnameStep processes the fullname registration step.
func (h *TelegramHelper) handleFullnameStep(message *tgbotapi.Message, userLang string) {
	if isRegistrationReset(message.Text) {
		h.clearRegistrationKeys(message.Chat.ID)
		h.startRegistration(message.Chat.ID, message.From, message.From.LanguageCode)
		return
	}
	fullname := strings.TrimSpace(message.Text)
	if fullname == "" {
		h.bot.Send(tgbotapi.NewMessage(message.Chat.ID, h.getLocalizedMessage(userLang, "registration_fullname_required")))
		return
	}
	h.redis.Set(context.Background(), fmt.Sprintf("telegram:registration:fullname:%d", message.Chat.ID), fullname, time.Hour)
	h.redis.Set(context.Background(), fmt.Sprintf("telegram:registration:step:%d", message.Chat.ID), "username", time.Hour)
	msg := tgbotapi.NewMessage(message.Chat.ID, h.getLocalizedMessage(userLang, "registration_username_prompt"))
	if message.From.UserName != "" {
		msg.ReplyMarkup = tgbotapi.ForceReply{ForceReply: true, InputFieldPlaceholder: message.From.UserName}
	} else {
		msg.ReplyMarkup = tgbotapi.ForceReply{ForceReply: true}
	}
	if _, err := h.bot.Send(msg); err != nil {
		logrus.WithError(err).Error("Failed to send username prompt")
	}
}

// handleUsernameStep processes the username registration step.
func (h *TelegramHelper) handleUsernameStep(message *tgbotapi.Message, userLang string) {
	if isRegistrationReset(message.Text) {
		h.clearRegistrationKeys(message.Chat.ID)
		h.startRegistration(message.Chat.ID, message.From, message.From.LanguageCode)
		return
	}
	username := strings.TrimSpace(message.Text)
	if username == "" {
		msg := tgbotapi.NewMessage(message.Chat.ID, h.getLocalizedMessage(userLang, "registration_username_required"))
		if _, err := h.bot.Send(msg); err != nil {
			logrus.WithError(err).Error("Failed to send username required message")
		}
		return
	}
	h.redis.Set(context.Background(), fmt.Sprintf("telegram:registration:username:%d", message.Chat.ID), username, time.Hour)
	h.redis.Set(context.Background(), fmt.Sprintf("telegram:registration:step:%d", message.Chat.ID), "email", time.Hour)
	msg := tgbotapi.NewMessage(message.Chat.ID, h.getLocalizedMessage(userLang, "registration_email_prompt"))
	msg.ReplyMarkup = tgbotapi.ForceReply{ForceReply: true}
	h.bot.Send(msg)
}

// handleEmailStep processes the email registration step.
func (h *TelegramHelper) handleEmailStep(message *tgbotapi.Message, userLang string) {
	if isRegistrationReset(message.Text) {
		h.clearRegistrationKeys(message.Chat.ID)
		h.startRegistration(message.Chat.ID, message.From, message.From.LanguageCode)
		return
	}
	email := strings.TrimSpace(message.Text)
	if email == "" {
		h.bot.Send(tgbotapi.NewMessage(message.Chat.ID, h.getLocalizedMessage(userLang, "registration_invalid_email")))
		return
	}
	if err := h.validateEmail(email); err != nil {
		errorMsg := fmt.Sprintf(h.getLocalizedMessage(userLang, "registration_invalid_email_detailed"), email)
		h.bot.Send(tgbotapi.NewMessage(message.Chat.ID, errorMsg))
		return
	}
	h.redis.Set(context.Background(), fmt.Sprintf("telegram:registration:email:%d", message.Chat.ID), email, time.Hour)
	h.redis.Set(context.Background(), fmt.Sprintf("telegram:registration:step:%d", message.Chat.ID), "phone", time.Hour)
	msg := tgbotapi.NewMessage(message.Chat.ID, h.getLocalizedMessage(userLang, "registration_phone_prompt"))
	msg.ReplyMarkup = h.CreatePhoneRequestKeyboard(userLang)
	h.bot.Send(msg)
}

// handlePhoneStep processes the phone registration step.
func (h *TelegramHelper) handlePhoneStep(message *tgbotapi.Message, userLang string) {
	cancelText := h.getLocalizedMessage(userLang, "cancel")
	if strings.TrimSpace(message.Text) == cancelText {
		h.redis.Set(context.Background(), fmt.Sprintf("telegram:registration:step:%d", message.Chat.ID), "email", time.Hour)
		msg := tgbotapi.NewMessage(message.Chat.ID, h.getLocalizedMessage(userLang, "registration_email_prompt"))
		msg.ReplyMarkup = tgbotapi.ForceReply{ForceReply: true}
		h.bot.Send(msg)
		return
	}
	if isRegistrationReset(message.Text) {
		h.clearRegistrationKeys(message.Chat.ID)
		h.startRegistration(message.Chat.ID, message.From, message.From.LanguageCode)
		return
	}
	phone, ok := h.extractPhoneNumber(message, userLang)
	if !ok {
		return
	}
	formattedPhone, err := h.validateAndFormatPhoneNumber(phone, userLang)
	if err != nil {
		countryCode := h.getCountryCodeFromLanguage(userLang)
		errorMsg := fmt.Sprintf(h.getLocalizedMessage(userLang, "registration_invalid_phone_detailed"), phone, countryCode)
		h.bot.Send(tgbotapi.NewMessage(message.Chat.ID, errorMsg))
		return
	}
	h.redis.Set(context.Background(), fmt.Sprintf("telegram:registration:phone:%d", message.Chat.ID), formattedPhone, time.Hour)
	h.redis.Set(context.Background(), fmt.Sprintf("telegram:registration:step:%d", message.Chat.ID), "usertype", time.Hour)
	msg := tgbotapi.NewMessage(message.Chat.ID, h.getLocalizedMessage(userLang, "registration_usertype_prompt"))
	msg.ReplyMarkup = h.CreateUsertypeKeyboard(userLang)
	h.bot.Send(msg)
}

// extractPhoneNumber extracts a phone number from a message contact or text.
// Sends an error reply and returns false if no phone available.
func (h *TelegramHelper) extractPhoneNumber(message *tgbotapi.Message, userLang string) (string, bool) {
	if message.Contact != nil && message.Contact.PhoneNumber != "" {
		return message.Contact.PhoneNumber, true
	}
	if message.Text != "" {
		return strings.TrimSpace(message.Text), true
	}
	h.bot.Send(tgbotapi.NewMessage(message.Chat.ID, h.getLocalizedMessage(userLang, "registration_phone_required")))
	return "", false
}

// handleUsertypeStep processes the usertype registration step.
func (h *TelegramHelper) handleUsertypeStep(message *tgbotapi.Message, userLang string) {
	if isRegistrationReset(message.Text) {
		h.clearRegistrationKeys(message.Chat.ID)
		h.startRegistration(message.Chat.ID, message.From, message.From.LanguageCode)
		return
	}
	h.bot.Send(tgbotapi.NewMessage(message.Chat.ID, h.getLocalizedMessage(userLang, "registration_select_usertype")))
}

// handleRegistrationCallback handles the registration steps for callbacks
func (h *TelegramHelper) handleRegistrationCallback(callback *tgbotapi.CallbackQuery, step string) {
	if step == "usertype" && strings.HasPrefix(callback.Data, "usertype_") {
		usertype := strings.TrimPrefix(callback.Data, "usertype_")

		// Map short names to full usertype strings
		switch usertype {
		// case "common":
		// 	usertype = string(telegrammodel.CommonUser)
		case "super_user":
			usertype = string(telegrammodel.SuperUser)
		case "technician_ms":
			usertype = string(telegrammodel.TechnicianMS)
		case "tams":
			usertype = string(telegrammodel.TAMS)
		case "splms":
			usertype = string(telegrammodel.SPLMS)
		case "sacms":
			usertype = string(telegrammodel.SACMS)
		case "head_ms":
			usertype = string(telegrammodel.HeadMS)
		}

		// Validate usertype against model constants
		if !h.isValidUserType(usertype) {
			userLang := h.getUserLanguage(callback.From.ID)
			msg := tgbotapi.NewMessage(callback.Message.Chat.ID, h.getLocalizedMessage(userLang, "registration_invalid_usertype"))
			h.bot.Send(msg)
			// Answer callback
			answer := tgbotapi.NewCallback(callback.ID, "")
			h.bot.Request(answer)
			return
		}

		h.redis.Set(context.Background(), fmt.Sprintf("telegram:registration:usertype:%d", callback.Message.Chat.ID), usertype, time.Hour)
		h.completeRegistration(callback.Message.Chat.ID, callback.From.ID)
		// Answer callback
		answer := tgbotapi.NewCallback(callback.ID, "")
		h.bot.Request(answer)
		// Remove the keyboard from the message
		editMsg := tgbotapi.NewEditMessageReplyMarkup(callback.Message.Chat.ID, callback.Message.MessageID, tgbotapi.InlineKeyboardMarkup{InlineKeyboard: [][]tgbotapi.InlineKeyboardButton{}})
		if _, err := h.bot.Send(editMsg); err != nil {
			logrus.WithError(err).Error("Failed to remove keyboard from message")
		}
	}
}

// completeRegistration completes the registration and updates the user
// sendErrorAndClear sends a localized error message and clears registration Redis keys.
func (h *TelegramHelper) sendErrorAndClear(chatID, userID int64, msgKey string) {
	userLang := h.getUserLanguage(userID)
	h.bot.Send(tgbotapi.NewMessage(chatID, h.getLocalizedMessage(userLang, msgKey)))
	h.clearRegistrationKeys(chatID)
}

// sendFormattedErrorAndClear sends a formatted localized error and clears registration Redis keys.
func (h *TelegramHelper) sendFormattedErrorAndClear(chatID, userID int64, msgKey string, args ...interface{}) {
	userLang := h.getUserLanguage(userID)
	h.bot.Send(tgbotapi.NewMessage(chatID, fmt.Sprintf(h.getLocalizedMessage(userLang, msgKey), args...)))
	h.clearRegistrationKeys(chatID)
}

// findUserByTypeAndPhone finds a TelegramUser by type and phone number from the DB.
func (h *TelegramHelper) findUserByTypeAndPhone(userType telegrammodel.TelegramUserType, phone string) (telegrammodel.TelegramUsers, bool) {
	var users []telegrammodel.TelegramUsers
	h.db.Where("telegram_user_type = ?", userType).Find(&users)
	for _, u := range users {
		if u.PhoneNumber == phone {
			return u, true
		}
	}
	return telegrammodel.TelegramUsers{}, false
}

// resolveExistingUserChatID frees the chatID if a conflicting user record can be cleared.
// Returns false (and sends an error) if the conflict cannot be resolved.
func (h *TelegramHelper) resolveExistingUserChatID(chatID, userID int64, existingID uint) bool {
	var otherUser telegrammodel.TelegramUsers
	if err := h.db.Unscoped().Where("telegram_chat_id = ? AND id != ?", chatID, existingID).First(&otherUser).Error; err == nil {
		if otherUser.DeletedAt.Valid || (otherUser.UserType == telegrammodel.CommonUser && !otherUser.VerifiedUser) {
			h.db.Model(&otherUser).Update("telegram_chat_id", nil)
		} else {
			h.sendErrorAndClear(chatID, userID, "registration_chat_id_in_use")
			return false
		}
	}
	return true
}

// resolveNewUserChatID ensures the chatID is free for a newly-created user record.
// Returns false (and sends an error) if the conflict cannot be resolved.
func (h *TelegramHelper) resolveNewUserChatID(chatID, userID int64) bool {
	var otherUser telegrammodel.TelegramUsers
	if err := h.db.Where("telegram_chat_id = ?", chatID).First(&otherUser).Error; err == nil {
		if otherUser.UserType == telegrammodel.CommonUser && !otherUser.VerifiedUser {
			h.db.Unscoped().Delete(&otherUser)
		} else {
			h.sendErrorAndClear(chatID, userID, "registration_chat_id_in_use")
			return false
		}
	}
	return true
}

// updateUserRegistration resolves any chatID conflict then applies the given field updates.
// Returns false on failure (error already communicated to user).
func (h *TelegramHelper) updateUserRegistration(chatID, userID int64, existing telegrammodel.TelegramUsers, updates map[string]interface{}, failMsgKey string) bool {
	if !h.resolveExistingUserChatID(chatID, userID, existing.ID) {
		return false
	}
	if err := h.db.Model(&existing).Updates(updates).Error; err != nil {
		h.sendFormattedErrorAndClear(chatID, userID, failMsgKey, err.Error())
		return false
	}
	return true
}

// completeRegistrationForSuperUser handles registration for the SuperUser type.
func (h *TelegramHelper) completeRegistrationForSuperUser(chatID, userID int64, fullname, username, email, phone string) bool {
	existing, found := h.findUserByTypeAndPhone(telegrammodel.SuperUser, phone)
	if !found {
		h.sendErrorAndClear(chatID, userID, "registration_superuser_not_allowed")
		return false
	}
	return h.updateUserRegistration(chatID, userID, existing, map[string]interface{}{
		"telegram_chat_id": chatID, "full_name": fullname, "username": username, "email": email, "verified_user": true,
	}, "registration_superuser_update_failed")
}

// completeRegistrationForTAMS handles registration for the TAMS type.
func (h *TelegramHelper) completeRegistrationForTAMS(chatID, userID int64, fullname, username, email, phone string) bool {
	existing, found := h.findUserByTypeAndPhone(telegrammodel.TAMS, phone)
	if !found {
		h.sendErrorAndClear(chatID, userID, "registration_tams_not_allowed")
		return false
	}
	return h.updateUserRegistration(chatID, userID, existing, map[string]interface{}{
		"telegram_chat_id": chatID, "full_name": fullname, "username": username, "email": email, "verified_user": true,
	}, "registration_tams_update_failed")
}

// completeRegistrationForHeadMS handles registration for the HeadMS type.
func (h *TelegramHelper) completeRegistrationForHeadMS(chatID, userID int64, fullname, username, email, phone string) bool {
	existing, found := h.findUserByTypeAndPhone(telegrammodel.HeadMS, phone)
	if !found {
		h.sendErrorAndClear(chatID, userID, "registration_headms_not_allowed")
		return false
	}
	return h.updateUserRegistration(chatID, userID, existing, map[string]interface{}{
		"telegram_chat_id": chatID, "full_name": fullname, "username": username, "email": email, "verified_user": true,
	}, "registration_headms_update_failed")
}

// completeRegistrationForSACMS handles registration for the SACMS type.
func (h *TelegramHelper) completeRegistrationForSACMS(chatID, userID int64, _, _, _, phone string) bool {
	existing, found := h.findUserByTypeAndPhone(telegrammodel.SACMS, phone)
	if !found {
		h.sendErrorAndClear(chatID, userID, "registration_sacms_not_allowed")
		return false
	}
	return h.updateUserRegistration(chatID, userID, existing, map[string]interface{}{
		"telegram_chat_id": chatID, "verified_user": true,
	}, "registration_sacms_update_failed")
}

// parseNationalPhoneID parses an E.164 phone and returns the national number string for Indonesia.
// On error it sends an error message, clears Redis keys and returns "", false.
func (h *TelegramHelper) parseNationalPhoneID(chatID, userID int64, phone, failMsgKey string) (string, bool) {
	parsed, err := phonenumbers.Parse(phone, "ID")
	if err != nil {
		h.sendFormattedErrorAndClear(chatID, userID, failMsgKey, "")
		return "", false
	}
	return strconv.FormatUint(parsed.GetNationalNumber(), 10), true
}

// completeRegistrationForTechnicianMS handles registration for the TechnicianMS type.
func (h *TelegramHelper) completeRegistrationForTechnicianMS(chatID, userID int64, fullname, username, email, phone string) bool {
	nationalPhone, ok := h.parseNationalPhoneID(chatID, userID, phone, "registration_technicianms_check_failed")
	if !ok {
		return false
	}
	exists, techData, err := odoomscontrollers.CheckExistingTechnicianInODOOMS("", email, nationalPhone)
	if err != nil {
		h.sendFormattedErrorAndClear(chatID, userID, "registration_technicianms_check_failed", "")
		return false
	}
	if !exists {
		h.sendErrorAndClear(chatID, userID, "registration_technicianms_not_registered_odooms")
		return false
	}
	existing, found := h.findUserByTypeAndPhone(telegrammodel.TechnicianMS, phone)
	if found {
		return h.updateUserRegistration(chatID, userID, existing, map[string]interface{}{
			"telegram_chat_id": chatID, "full_name": fullname, "username": username, "email": email,
			"verified_user": true, "telegram_user_of": telegrammodel.CompanyEmployee,
			"description": fmt.Sprintf("%s", techData.NameFS.String),
		}, "registration_technicianms_update_failed")
	}
	if !h.resolveNewUserChatID(chatID, userID) {
		return false
	}
	if err := h.db.Create(&telegrammodel.TelegramUsers{
		ChatID: &chatID, FullName: fullname, Username: username, PhoneNumber: phone,
		Email: email, UserType: telegrammodel.TechnicianMS, UserOf: telegrammodel.CompanyEmployee, VerifiedUser: true,
	}).Error; err != nil {
		h.sendFormattedErrorAndClear(chatID, userID, "registration_technicianms_update_failed", err.Error())
		return false
	}
	return true
}

// completeRegistrationForSPLMS handles registration for the SPLMS type.
func (h *TelegramHelper) completeRegistrationForSPLMS(chatID, userID int64, fullname, username, email, phone string) bool {
	nationalPhone, ok := h.parseNationalPhoneID(chatID, userID, phone, "registration_splms_check_failed")
	if !ok {
		return false
	}
	exists, splData, err := odoomscontrollers.CheckExistingTechnicianInODOOMS("", email, nationalPhone)
	if err != nil {
		h.sendFormattedErrorAndClear(chatID, userID, "registration_splms_check_failed", err.Error())
		return false
	}
	if !exists {
		h.sendErrorAndClear(chatID, userID, "registration_splms_not_registered_odooms")
		return false
	}
	existing, found := h.findUserByTypeAndPhone(telegrammodel.SPLMS, phone)
	if found {
		return h.updateUserRegistration(chatID, userID, existing, map[string]interface{}{
			"telegram_chat_id": chatID, "full_name": fullname, "username": username, "email": email,
			"verified_user": true, "telegram_user_of": telegrammodel.CompanyEmployee,
			"description": fmt.Sprintf("%s", splData.NameFS.String),
		}, "registration_splms_update_failed")
	}
	if !h.resolveNewUserChatID(chatID, userID) {
		return false
	}
	if err := h.db.Create(&telegrammodel.TelegramUsers{
		ChatID: &chatID, FullName: fullname, Username: username, PhoneNumber: phone,
		Email: email, UserType: telegrammodel.SPLMS, UserOf: telegrammodel.CompanyEmployee, VerifiedUser: true,
	}).Error; err != nil {
		h.sendFormattedErrorAndClear(chatID, userID, "registration_splms_update_failed", err.Error())
		return false
	}
	return true
}

// completeRegistrationByType dispatches to the appropriate per-type registration handler.
func (h *TelegramHelper) completeRegistrationByType(chatID, userID int64, userType telegrammodel.TelegramUserType, fullname, username, email, phone string) bool {
	switch string(userType) {
	case string(telegrammodel.SuperUser):
		return h.completeRegistrationForSuperUser(chatID, userID, fullname, username, email, phone)
	case string(telegrammodel.TAMS):
		return h.completeRegistrationForTAMS(chatID, userID, fullname, username, email, phone)
	case string(telegrammodel.HeadMS):
		return h.completeRegistrationForHeadMS(chatID, userID, fullname, username, email, phone)
	case string(telegrammodel.TechnicianMS):
		return h.completeRegistrationForTechnicianMS(chatID, userID, fullname, username, email, phone)
	case string(telegrammodel.SPLMS):
		return h.completeRegistrationForSPLMS(chatID, userID, fullname, username, email, phone)
	case string(telegrammodel.SACMS):
		return h.completeRegistrationForSACMS(chatID, userID, fullname, username, email, phone)
	default:
		h.db.Create(&telegrammodel.TelegramUsers{
			ChatID: &chatID, FullName: fullname, Username: username, PhoneNumber: phone,
			Email: email, UserType: userType, VerifiedUser: true,
		})
		return true
	}
}

// getUserTypeDisplayName returns a localized display string for the given usertype string.
func (h *TelegramHelper) getUserTypeDisplayName(userLang, usertype string) string {
	key := map[string]string{
		string(telegrammodel.CommonUser):   "usertype_common",
		string(telegrammodel.SuperUser):    "usertype_super_user",
		string(telegrammodel.TechnicianMS): "usertype_technician_ms",
		string(telegrammodel.SPLMS):        "usertype_splms",
		string(telegrammodel.SACMS):        "usertype_sacms",
		string(telegrammodel.TAMS):         "usertype_tams",
		string(telegrammodel.HeadMS):       "usertype_head_ms",
	}[usertype]
	if key != "" {
		return h.getLocalizedMessage(userLang, key)
	}
	return usertype
}

// completeRegistration completes the registration flow after user-type selection.
func (h *TelegramHelper) completeRegistration(chatID int64, userID int64) {
	fullname, _ := h.redis.Get(context.Background(), fmt.Sprintf("telegram:registration:fullname:%d", chatID)).Result()
	username, _ := h.redis.Get(context.Background(), fmt.Sprintf("telegram:registration:username:%d", chatID)).Result()
	email, _ := h.redis.Get(context.Background(), fmt.Sprintf("telegram:registration:email:%d", chatID)).Result()
	phone, _ := h.redis.Get(context.Background(), fmt.Sprintf("telegram:registration:phone:%d", chatID)).Result()
	usertype, _ := h.redis.Get(context.Background(), fmt.Sprintf("telegram:registration:usertype:%d", chatID)).Result()
	lang, _ := h.redis.Get(context.Background(), fmt.Sprintf("telegram:registration:lang:%d", chatID)).Result()

	if !h.completeRegistrationByType(chatID, userID, telegrammodel.TelegramUserType(usertype), fullname, username, email, phone) {
		return
	}

	h.setUserLanguage(userID, lang)
	h.clearRegistrationKeys(chatID)

	userLang := h.getUserLanguage(userID)
	userTypeDisplay := h.getUserTypeDisplayName(userLang, usertype)
	confirmMsg := fmt.Sprintf(`%s
%s %s
%s %s
%s %s
%s %s
%s %s

%s`,
		h.getLocalizedMessage(userLang, "registration_success_title"),
		h.getLocalizedMessage(userLang, "registration_details_fullname"), fullname,
		h.getLocalizedMessage(userLang, "registration_details_username"), username,
		h.getLocalizedMessage(userLang, "registration_details_email"), email,
		h.getLocalizedMessage(userLang, "registration_details_phone"), phone,
		h.getLocalizedMessage(userLang, "registration_details_usertype"), userTypeDisplay,
		h.getLocalizedMessage(userLang, "registration_welcome_message"))
	msg := tgbotapi.NewMessage(chatID, confirmMsg)
	msg.ParseMode = "Markdown"
	h.bot.Send(msg)
}
