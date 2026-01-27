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
)

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
	h.bot.Send(msg)
}

// handleRegistrationStep handles the registration steps for messages
func (h *TelegramHelper) handleRegistrationStep(message *tgbotapi.Message, step string) {
	userLang := h.getUserLanguage(message.From.ID)
	switch step {
	case "fullname":
		fullname := strings.TrimSpace(message.Text)
		if fullname == "" {
			msg := tgbotapi.NewMessage(message.Chat.ID, h.getLocalizedMessage(userLang, "registration_fullname_required"))
			h.bot.Send(msg)
			return
		}
		h.redis.Set(context.Background(), fmt.Sprintf("telegram:registration:fullname:%d", message.Chat.ID), fullname, time.Hour)
		h.redis.Set(context.Background(), fmt.Sprintf("telegram:registration:step:%d", message.Chat.ID), "username", time.Hour)
		msgText := h.getLocalizedMessage(userLang, "registration_username_prompt")
		msg := tgbotapi.NewMessage(message.Chat.ID, msgText)
		if message.From.UserName != "" {
			msg.ReplyMarkup = tgbotapi.ForceReply{ForceReply: true, InputFieldPlaceholder: message.From.UserName}
		} else {
			msg.ReplyMarkup = tgbotapi.ForceReply{ForceReply: true}
		}
		h.bot.Send(msg)

	case "username":
		username := strings.TrimSpace(message.Text)
		if username == "" {
			msg := tgbotapi.NewMessage(message.Chat.ID, h.getLocalizedMessage(userLang, "registration_username_required"))
			h.bot.Send(msg)
			return
		}
		h.redis.Set(context.Background(), fmt.Sprintf("telegram:registration:username:%d", message.Chat.ID), username, time.Hour)
		h.redis.Set(context.Background(), fmt.Sprintf("telegram:registration:step:%d", message.Chat.ID), "email", time.Hour)
		msgText := h.getLocalizedMessage(userLang, "registration_email_prompt")
		msg := tgbotapi.NewMessage(message.Chat.ID, msgText)
		msg.ReplyMarkup = tgbotapi.ForceReply{ForceReply: true}
		h.bot.Send(msg)

	case "email":
		email := strings.TrimSpace(message.Text)
		if email == "" {
			msg := tgbotapi.NewMessage(message.Chat.ID, h.getLocalizedMessage(userLang, "registration_invalid_email"))
			h.bot.Send(msg)
			return
		}

		// Validate email using validator package
		if err := h.validateEmail(email); err != nil {
			errorMsg := fmt.Sprintf(h.getLocalizedMessage(userLang, "registration_invalid_email_detailed"), email)
			msg := tgbotapi.NewMessage(message.Chat.ID, errorMsg)
			h.bot.Send(msg)
			return
		}

		h.redis.Set(context.Background(), fmt.Sprintf("telegram:registration:email:%d", message.Chat.ID), email, time.Hour)
		h.redis.Set(context.Background(), fmt.Sprintf("telegram:registration:step:%d", message.Chat.ID), "phone", time.Hour)
		msgText := h.getLocalizedMessage(userLang, "registration_phone_prompt")
		msg := tgbotapi.NewMessage(message.Chat.ID, msgText)
		msg.ReplyMarkup = h.CreatePhoneRequestKeyboard(userLang)
		h.bot.Send(msg)

	case "phone":
		// Check if user pressed cancel button
		cancelText := h.getLocalizedMessage(userLang, "cancel")
		if strings.TrimSpace(message.Text) == cancelText {
			// User cancelled phone input, go back to email step
			h.redis.Set(context.Background(), fmt.Sprintf("telegram:registration:step:%d", message.Chat.ID), "email", time.Hour)
			msgText := h.getLocalizedMessage(userLang, "registration_email_prompt")
			msg := tgbotapi.NewMessage(message.Chat.ID, msgText)
			msg.ReplyMarkup = tgbotapi.ForceReply{ForceReply: true}
			h.bot.Send(msg)
			return
		}

		// Get phone number from contact or text
		var phone string
		if message.Contact != nil && message.Contact.PhoneNumber != "" {
			phone = message.Contact.PhoneNumber
		} else if message.Text != "" {
			phone = strings.TrimSpace(message.Text)
		} else {
			msg := tgbotapi.NewMessage(message.Chat.ID, h.getLocalizedMessage(userLang, "registration_phone_required"))
			h.bot.Send(msg)
			return
		}

		// Validate and format phone number to E.164
		formattedPhone, err := h.validateAndFormatPhoneNumber(phone, userLang)
		if err != nil {
			// Get country code for better error message
			countryCode := h.getCountryCodeFromLanguage(userLang)
			errorMsg := fmt.Sprintf(h.getLocalizedMessage(userLang, "registration_invalid_phone_detailed"), phone, countryCode)
			msg := tgbotapi.NewMessage(message.Chat.ID, errorMsg)
			h.bot.Send(msg)
			return
		}

		h.redis.Set(context.Background(), fmt.Sprintf("telegram:registration:phone:%d", message.Chat.ID), formattedPhone, time.Hour)
		h.redis.Set(context.Background(), fmt.Sprintf("telegram:registration:step:%d", message.Chat.ID), "usertype", time.Hour)
		msgText := h.getLocalizedMessage(userLang, "registration_usertype_prompt")
		msg := tgbotapi.NewMessage(message.Chat.ID, msgText)
		msg.ReplyMarkup = h.CreateUsertypeKeyboard(userLang)
		h.bot.Send(msg)

	case "usertype":
		msg := tgbotapi.NewMessage(message.Chat.ID, h.getLocalizedMessage(userLang, "registration_select_usertype"))
		h.bot.Send(msg)
	}
}

// handleRegistrationCallback handles the registration steps for callbacks
func (h *TelegramHelper) handleRegistrationCallback(callback *tgbotapi.CallbackQuery, step string) {
	if step == "usertype" && strings.HasPrefix(callback.Data, "usertype_") {
		usertype := strings.TrimPrefix(callback.Data, "usertype_")

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
		editMsg := tgbotapi.NewEditMessageReplyMarkup(callback.Message.Chat.ID, callback.Message.MessageID, tgbotapi.InlineKeyboardMarkup{})
		h.bot.Send(editMsg)
	}
}

// completeRegistration completes the registration and updates the user
func (h *TelegramHelper) completeRegistration(chatID int64, userID int64) {
	// Get all data
	fullname, _ := h.redis.Get(context.Background(), fmt.Sprintf("telegram:registration:fullname:%d", chatID)).Result()
	username, _ := h.redis.Get(context.Background(), fmt.Sprintf("telegram:registration:username:%d", chatID)).Result()
	email, _ := h.redis.Get(context.Background(), fmt.Sprintf("telegram:registration:email:%d", chatID)).Result()
	phone, _ := h.redis.Get(context.Background(), fmt.Sprintf("telegram:registration:phone:%d", chatID)).Result()
	usertype, _ := h.redis.Get(context.Background(), fmt.Sprintf("telegram:registration:usertype:%d", chatID)).Result()
	lang, _ := h.redis.Get(context.Background(), fmt.Sprintf("telegram:registration:lang:%d", chatID)).Result()

	// Handle registration based on user type
	switch usertype {
	case string(telegrammodel.SuperUser):
		// For SuperUser, check if phone number already exists with SuperUser type
		// Since phone may be stored in different formats, check by formatting each SuperUser's phone
		var superUsers []telegrammodel.TelegramUsers
		h.db.Where("telegram_user_type = ?", telegrammodel.SuperUser).Find(&superUsers)
		var existing telegrammodel.TelegramUsers
		found := false
		for _, su := range superUsers {
			if su.PhoneNumber == phone {
				existing = su
				found = true
				break
			}
		}
		if found {
			// Check if chat_id is already in use by another user
			var otherUser telegrammodel.TelegramUsers
			if err := h.db.Unscoped().Where("telegram_chat_id = ? AND id != ?", chatID, existing.ID).First(&otherUser).Error; err == nil {
				// Another user has this chat_id, check if it's soft deleted or unverified common user
				if otherUser.DeletedAt.Valid || (otherUser.UserType == telegrammodel.CommonUser && !otherUser.VerifiedUser) {
					// Set the conflicting user's chat_id to null to free it
					h.db.Model(&otherUser).Update("telegram_chat_id", nil)
				} else {
					// Other user is verified or not common, cannot take chat_id
					userLang := h.getUserLanguage(userID)
					errorMsg := h.getLocalizedMessage(userLang, "registration_chat_id_in_use")
					msg := tgbotapi.NewMessage(chatID, errorMsg)
					h.bot.Send(msg)
					// Clear Redis keys and return without confirmation
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
					return
				}
			}
			// Now update the SuperUser
			err := h.db.Model(&existing).Updates(map[string]interface{}{
				"telegram_chat_id": chatID,
				"full_name":        fullname,
				"username":         username,
				"email":            email,
				"verified_user":    true,
			}).Error
			if err != nil {
				// Update failed, show error
				userLang := h.getUserLanguage(userID)
				errorMsg := fmt.Sprintf(h.getLocalizedMessage(userLang, "registration_superuser_update_failed"), err.Error())
				msg := tgbotapi.NewMessage(chatID, errorMsg)
				h.bot.Send(msg)
				// Clear Redis keys and return without confirmation
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
				return
			}
		} else {
			// Phone not exists, show error
			userLang := h.getUserLanguage(userID)
			errorMsg := h.getLocalizedMessage(userLang, "registration_superuser_not_allowed")
			msg := tgbotapi.NewMessage(chatID, errorMsg)
			h.bot.Send(msg)
			// Clear Redis keys and return without confirmation
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
			return
		}

	case string(telegrammodel.TAMS):
		// For TAMS, check if phone number already exists with TAMS type
		var tamsUsers []telegrammodel.TelegramUsers
		h.db.Where("telegram_user_type = ?", telegrammodel.TAMS).Find(&tamsUsers)
		var existing telegrammodel.TelegramUsers
		found := false
		for _, tu := range tamsUsers {
			if tu.PhoneNumber == phone {
				existing = tu
				found = true
				break
			}
		}
		if found {
			// Check if chat_id is already in use by another user
			var otherUser telegrammodel.TelegramUsers
			if err := h.db.Unscoped().Where("telegram_chat_id = ? AND id != ?", chatID, existing.ID).First(&otherUser).Error; err == nil {
				// Another user has this chat_id, check if it's soft deleted or unverified common user
				if otherUser.DeletedAt.Valid || (otherUser.UserType == telegrammodel.CommonUser && !otherUser.VerifiedUser) {
					// Set the conflicting user's chat_id to null to free it
					h.db.Model(&otherUser).Update("telegram_chat_id", nil)
				} else {
					// Other user is verified or not common, cannot take chat_id
					userLang := h.getUserLanguage(userID)
					errorMsg := h.getLocalizedMessage(userLang, "registration_chat_id_in_use")
					msg := tgbotapi.NewMessage(chatID, errorMsg)
					h.bot.Send(msg)
					// Clear Redis keys and return without confirmation
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
					return
				}
			}
			// Now update the TAMS
			err := h.db.Model(&existing).Updates(map[string]interface{}{
				"telegram_chat_id": chatID,
				"full_name":        fullname,
				"username":         username,
				"email":            email,
				"verified_user":    true,
			}).Error
			if err != nil {
				// Update failed, show error
				userLang := h.getUserLanguage(userID)
				errorMsg := fmt.Sprintf(h.getLocalizedMessage(userLang, "registration_tams_update_failed"), err.Error())
				msg := tgbotapi.NewMessage(chatID, errorMsg)
				h.bot.Send(msg)
				// Clear Redis keys and return without confirmation
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
				return
			}
		} else {
			// Phone not exists, show error
			userLang := h.getUserLanguage(userID)
			errorMsg := h.getLocalizedMessage(userLang, "registration_tams_not_allowed")
			msg := tgbotapi.NewMessage(chatID, errorMsg)
			h.bot.Send(msg)
			// Clear Redis keys and return without confirmation
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
			return
		}

	case string(telegrammodel.HeadMS):
		// For HeadMS, check if phone number already exists with HeadMS type
		var headmsUsers []telegrammodel.TelegramUsers
		h.db.Where("telegram_user_type = ?", telegrammodel.HeadMS).Find(&headmsUsers)
		var existing telegrammodel.TelegramUsers
		found := false
		for _, hu := range headmsUsers {
			if hu.PhoneNumber == phone {
				existing = hu
				found = true
				break
			}
		}
		if found {
			// Check if chat_id is already in use by another user
			var otherUser telegrammodel.TelegramUsers
			if err := h.db.Unscoped().Where("telegram_chat_id = ? AND id != ?", chatID, existing.ID).First(&otherUser).Error; err == nil {
				// Another user has this chat_id, check if it's soft deleted or unverified common user
				if otherUser.DeletedAt.Valid || (otherUser.UserType == telegrammodel.CommonUser && !otherUser.VerifiedUser) {
					// Set the conflicting user's chat_id to null to free it
					h.db.Model(&otherUser).Update("telegram_chat_id", nil)
				} else {
					// Other user is verified or not common, cannot take chat_id
					userLang := h.getUserLanguage(userID)
					errorMsg := h.getLocalizedMessage(userLang, "registration_chat_id_in_use")
					msg := tgbotapi.NewMessage(chatID, errorMsg)
					h.bot.Send(msg)
					// Clear Redis keys and return without confirmation
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
					return
				}
			}
			// Now update the HeadMS
			err := h.db.Model(&existing).Updates(map[string]interface{}{
				"telegram_chat_id": chatID,
				"full_name":        fullname,
				"username":         username,
				"email":            email,
				"verified_user":    true,
			}).Error
			if err != nil {
				// Update failed, show error
				userLang := h.getUserLanguage(userID)
				errorMsg := fmt.Sprintf(h.getLocalizedMessage(userLang, "registration_headms_update_failed"), err.Error())
				msg := tgbotapi.NewMessage(chatID, errorMsg)
				h.bot.Send(msg)
				// Clear Redis keys and return without confirmation
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
				return
			}
		} else {
			// Phone not exists, show error
			userLang := h.getUserLanguage(userID)
			errorMsg := h.getLocalizedMessage(userLang, "registration_headms_not_allowed")
			msg := tgbotapi.NewMessage(chatID, errorMsg)
			h.bot.Send(msg)
			// Clear Redis keys and return without confirmation
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
			return
		}

	case string(telegrammodel.TechnicianMS):
		// Parse phone to get national number for Indonesia
		parsedPhone, err := phonenumbers.Parse(phone, "ID")
		if err != nil {
			userLang := h.getUserLanguage(userID)
			errorMsg := fmt.Sprintf(h.getLocalizedMessage(userLang, "registration_technicianms_check_failed"), err.Error())
			msg := tgbotapi.NewMessage(chatID, errorMsg)
			h.bot.Send(msg)
			// Clear Redis keys and return without confirmation
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
			return
		}
		nationalNumber := parsedPhone.GetNationalNumber()
		nationalPhoneStr := strconv.FormatUint(nationalNumber, 10)

		technicianExists, err := odoomscontrollers.CheckExistingTechnicianInODOOMS("", email, nationalPhoneStr)
		if err != nil {
			userLang := h.getUserLanguage(userID)
			errorMsg := fmt.Sprintf(h.getLocalizedMessage(userLang, "registration_technicianms_check_failed"), err.Error())
			msg := tgbotapi.NewMessage(chatID, errorMsg)
			h.bot.Send(msg)
			// Clear Redis keys and return without confirmation
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
			return
		}
		if !technicianExists {
			// Technician does not exist in ODOOMS, show error
			userLang := h.getUserLanguage(userID)
			errorMsg := h.getLocalizedMessage(userLang, "registration_technicianms_not_registered_odooms")
			msg := tgbotapi.NewMessage(chatID, errorMsg)
			h.bot.Send(msg)
			// Clear Redis keys and return without confirmation
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
			return
		}

		// For TechnicianMS, check if phone number already exists with TechnicianMS type
		var technicianUsers []telegrammodel.TelegramUsers
		h.db.Where("telegram_user_type = ?", telegrammodel.TechnicianMS).Find(&technicianUsers)
		var existing telegrammodel.TelegramUsers
		found := false
		for _, tu := range technicianUsers {
			if tu.PhoneNumber == phone {
				existing = tu
				found = true
				break
			}
		}
		if found {
			// Check if chat_id is already in use by another user
			var otherUser telegrammodel.TelegramUsers
			if err := h.db.Unscoped().Where("telegram_chat_id = ? AND id != ?", chatID, existing.ID).First(&otherUser).Error; err == nil {
				// Another user has this chat_id, check if it's soft deleted or unverified common user
				if otherUser.DeletedAt.Valid || (otherUser.UserType == telegrammodel.CommonUser && !otherUser.VerifiedUser) {
					// Set the conflicting user's chat_id to null to free it
					h.db.Model(&otherUser).Update("telegram_chat_id", nil)
				} else {
					// Other user is verified or not common, cannot take chat_id
					userLang := h.getUserLanguage(userID)
					errorMsg := h.getLocalizedMessage(userLang, "registration_chat_id_in_use")
					msg := tgbotapi.NewMessage(chatID, errorMsg)
					h.bot.Send(msg)
					// Clear Redis keys and return without confirmation
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
					return
				}
			}
			// Now update the TechnicianMS
			err := h.db.Model(&existing).Updates(map[string]interface{}{
				"telegram_chat_id": chatID,
				"full_name":        fullname,
				"username":         username,
				"email":            email,
				"verified_user":    true,
				"telegram_user_of": telegrammodel.CompanyEmployee,
			}).Error
			if err != nil {
				// Update failed, show error
				userLang := h.getUserLanguage(userID)
				errorMsg := fmt.Sprintf(h.getLocalizedMessage(userLang, "registration_technicianms_update_failed"), err.Error())
				msg := tgbotapi.NewMessage(chatID, errorMsg)
				h.bot.Send(msg)
				// Clear Redis keys and return without confirmation
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
				return
			}
		} else {
			// Phone not exists in local DB, create new TechnicianMS record
			// Check if chat_id is already in use by another user
			var otherUser telegrammodel.TelegramUsers
			if err := h.db.Where("telegram_chat_id = ?", chatID).First(&otherUser).Error; err == nil {
				// Another user has this chat_id, check if it's an unverified common user
				if otherUser.UserType == telegrammodel.CommonUser && !otherUser.VerifiedUser {
					// Delete the unverified common user to allow TechnicianMS to take the chat_id
					h.db.Unscoped().Delete(&otherUser)
				} else {
					// Other user is verified or not common, cannot take chat_id
					userLang := h.getUserLanguage(userID)
					errorMsg := h.getLocalizedMessage(userLang, "registration_chat_id_in_use")
					msg := tgbotapi.NewMessage(chatID, errorMsg)
					h.bot.Send(msg)
					// Clear Redis keys and return without confirmation
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
					return
				}
			}
			telegramUser := telegrammodel.TelegramUsers{
				ChatID:       &chatID,
				FullName:     fullname,
				Username:     username,
				PhoneNumber:  phone,
				Email:        email,
				UserType:     telegrammodel.TechnicianMS,
				UserOf:       telegrammodel.CompanyEmployee,
				VerifiedUser: true,
			}
			err = h.db.Create(&telegramUser).Error
			if err != nil {
				userLang := h.getUserLanguage(userID)
				errorMsg := fmt.Sprintf(h.getLocalizedMessage(userLang, "registration_technicianms_update_failed"), err.Error())
				msg := tgbotapi.NewMessage(chatID, errorMsg)
				h.bot.Send(msg)
				// Clear Redis keys and return without confirmation
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
				return
			}
		}

	default:
		// For other user types, create new user
		telegramUser := telegrammodel.TelegramUsers{
			ChatID:       &chatID,
			FullName:     fullname,
			Username:     username,
			PhoneNumber:  phone,
			Email:        email,
			UserType:     telegrammodel.TelegramUserType(usertype),
			VerifiedUser: true,
		}
		h.db.Create(&telegramUser)
	}

	// Set language
	h.setUserLanguage(userID, lang)

	// Clear Redis keys
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

	// Send confirmation with registration details
	userLang := h.getUserLanguage(userID)

	// Get the user type display name
	var userTypeDisplay string
	switch usertype {
	case "common":
		userTypeDisplay = h.getLocalizedMessage(userLang, "usertype_common")
	case "super_user":
		userTypeDisplay = h.getLocalizedMessage(userLang, "usertype_super_user")
	case "technician_ms":
		userTypeDisplay = h.getLocalizedMessage(userLang, "usertype_technician_ms")
	case "tams":
		userTypeDisplay = h.getLocalizedMessage(userLang, "usertype_tams")
	case "head_ms":
		userTypeDisplay = h.getLocalizedMessage(userLang, "usertype_head_ms")
	default:
		userTypeDisplay = usertype
	}

	// Create detailed confirmation message using localized keys
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
