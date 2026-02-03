package controllers

import (
	"context"
	"fmt"
	"path/filepath"
	"service-platform/cmd/web_panel/config"
	"service-platform/cmd/web_panel/grpc/telegram"
	"service-platform/cmd/web_panel/internal/gormdb"
	sptechnicianmodel "service-platform/cmd/web_panel/model/sp_technician_model"
	"time"

	"github.com/sirupsen/logrus"

	pb "service-platform/proto"
)

// SendSPDocumentViaTelegram sends an SP document via Telegram
// This is a simplified version that stores the intent in database
// The actual sending will be implemented when proto files are ready
func SendSPDocumentViaTelegram(
	forProject string,
	recipientType string, // "technician", "spl", "sac", "hrd"
	recipientName string,
	chatID string, // Will be phone number for now, converted to chat_id later
	messageText string,
	spFilePath string,
	spNumber int,
	phoneNumber string,
	technicianID string,
	technicianName string,
	splID string,
	splName string,
	sacID string,
	sacName string,
	pelanggaran string,
	noSurat int,
	technicianGotSPID *uint,
	splGotSPID *uint,
	sacGotSPID *uint,
) error {
	dbWeb := gormdb.Databases.Web
	if dbWeb == nil {
		return fmt.Errorf("database connection is nil")
	}

	// For now, use phone number as chat_id placeholder
	// In production, this should be looked up from telegram_users table
	if chatID == "" {
		chatID = phoneNumber
	}

	// Prepare document URL
	cfg := config.GetConfig()
	baseURL := cfg.App.WebPublicURL
	if baseURL == "" {
		baseURL = fmt.Sprintf("http://%s:%s", cfg.App.Host, cfg.App.Port)
	}

	fileName := filepath.Base(spFilePath)
	documentURL := fmt.Sprintf("%s/uploads/sp/%s", baseURL, fileName)

	// Calculate response deadline (2 working days)
	now := time.Now()
	deadline := calculateWorkingDayDeadline(now, 2)

	// Store the message in database for tracking
	telegramMsg := sptechnicianmodel.SPTelegramMessage{
		TechnicianGotSPID: technicianGotSPID,
		SPLGotSPID:        splGotSPID,
		SACGotSPID:        sacGotSPID,
		RecipientType:     recipientType,
		RecipientName:     recipientName,
		ChatID:            chatID,
		PhoneNumber:       phoneNumber,
		ForProject:        forProject,
		NumberOfSP:        spNumber,
		SPFilePath:        spFilePath,
		MessageText:       messageText,
		Pelanggaran:       pelanggaran,
		NoSurat:           noSurat,
		TechnicianID:      technicianID,
		TechnicianName:    technicianName,
		SPLID:             splID,
		SPLName:           splName,
		SACID:             sacID,
		SACName:           sacName,
		SentAt:            &now,
		SentSuccess:       false, // Will be updated after actual send
		ResponseDeadline:  &deadline,
		ResponseStatus:    "pending",
	}

	// Send via Telegram gRPC service
	if telegram.GetConnection() == nil {
		// Try to reconnect if connection wasn't established at startup
		if err := telegram.EnsureConnection(); err != nil {
			logrus.Warn("⚠️ Telegram gRPC not connected, logging only")
			telegramMsg.SentSuccess = false
			telegramMsg.ErrorMessage = "Telegram gRPC connection failed"
		}
	}

	if telegram.GetConnection() != nil {
		// Create gRPC client
		client := telegram.GetClient()

		// Get request timeout from config
		cfg := config.GetConfig()
		reqTimeout := time.Duration(cfg.TelegramService.RequestTimeout) * time.Second
		if reqTimeout == 0 {
			reqTimeout = 30 * time.Second
		}

		ctx, cancel := context.WithTimeout(context.Background(), reqTimeout)
		defer cancel()

		// Format caption with SP details
		caption := fmt.Sprintf("📄 *SP-%d - %s*\n\n%s", spNumber, forProject, messageText)

		// Call gRPC SendDocument method
		resp, err := client.SendDocument(ctx, &pb.SendTelegramDocumentRequest{
			ChatId:    chatID,
			Document:  documentURL,
			Caption:   caption,
			ParseMode: "Markdown",
		})

		if err != nil {
			logrus.WithError(err).Errorf("❌ Failed to send SP-%d via Telegram gRPC to %s", spNumber, recipientName)
			telegramMsg.SentSuccess = false
			telegramMsg.ErrorMessage = fmt.Sprintf("gRPC error: %v", err)
		} else if resp != nil {
			if resp.Success {
				logrus.Infof("✅ SP-%d sent successfully to %s (%s) via Telegram - MsgID: %d",
					spNumber, recipientName, recipientType, resp.MessageId)
				telegramMsg.SentSuccess = true
				telegramMsg.TelegramMessageID = resp.MessageId
			} else {
				logrus.Warnf("⚠️ Telegram send reported failure for SP-%d: %s", spNumber, resp.Message)
				telegramMsg.SentSuccess = false
				telegramMsg.ErrorMessage = resp.Message
			}
		}
	}

	// Save to database
	if err := dbWeb.Create(&telegramMsg).Error; err != nil {
		logrus.WithError(err).Error("Failed to save Telegram message record")
		return fmt.Errorf("failed to save telegram message: %w", err)
	}

	logrus.Infof("✅ Telegram send logged: SP-%d to %s (%s)", spNumber, recipientName, recipientType)
	return nil
}

// calculateWorkingDayDeadline calculates a deadline N working days from start
func calculateWorkingDayDeadline(startTime time.Time, workingDays int) time.Time {
	loc, _ := time.LoadLocation(config.GetConfig().Default.Timezone)
	deadline := startTime.In(loc)

	daysAdded := 0
	for daysAdded < workingDays {
		deadline = deadline.Add(24 * time.Hour)
		// Skip weekends
		if deadline.Weekday() != time.Saturday && deadline.Weekday() != time.Sunday {
			daysAdded++
		}
	}

	// Set to end of working day (19:00)
	deadline = time.Date(deadline.Year(), deadline.Month(), deadline.Day(), 19, 0, 0, 0, loc)
	return deadline
}

// GetChatIDFromPhone looks up Telegram chat_id from phone number
// This should query telegram_users table when available
func GetChatIDFromPhone(phoneNumber string) (string, error) {
	// TODO: Implement database lookup
	// SELECT chat_id FROM telegram_users WHERE phone_number = ? AND is_verified = true

	// For now, return phone as placeholder
	// In production, users must register with bot first
	return phoneNumber, nil
}
