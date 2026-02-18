package migrations

import (
	"service-platform/internal/config"
	"service-platform/internal/core/model"
	telegrammodel "service-platform/internal/core/model/telegram_model"
	whatsnyanmodel "service-platform/internal/core/model/whatsnyan_model"
	"service-platform/internal/migrations"
	"time"

	"gorm.io/gorm"
)

func init() {
	migrations.RegisterMigration(&migrations.Migration{
		ID:        "001_initial_schema",
		Timestamp: time.Now().UTC(),
		Up:        upInitialSchema,
		Down:      downInitialSchema,
	})
}

func upInitialSchema(db *gorm.DB) error {
	// Use GORM AutoMigrate to create tables with correct column names and types
	// This ensures consistency with your existing GORM configuration
	return db.AutoMigrate(
		&model.Users{},
		&model.UserStatus{},
		&model.UserPasswordChangeLog{},
		&model.Role{},
		&model.RolePrivilege{},
		&model.Feature{},
		&model.LogActivity{},
		&model.Language{},
		&model.BadWord{},
		&model.AppConfig{},
		// WhatsApp Models
		&model.WAUsers{},
		&model.WhatsappMessageAutoReply{},
		&whatsnyanmodel.WhatsAppGroup{},
		&whatsnyanmodel.WhatsAppGroupParticipant{},
		&whatsnyanmodel.WhatsAppMsg{},
		&whatsnyanmodel.WhatsAppIncomingMsg{},
		// Telegram Models
		&telegrammodel.TelegramUsers{},
		&telegrammodel.TelegramMsg{},
		&telegrammodel.TelegramIncomingMsg{},
	)
}

func downInitialSchema(db *gorm.DB) error {
	// Drop tables in reverse order to handle foreign key constraints
	// Use config values for table names to match the actual table names used
	tables := []string{
		config.GetConfig().Whatsnyan.Tables.TBWhatsnyanGroupParticipant,
		config.GetConfig().Whatsnyan.Tables.TBWhatsnyanGroup,
		config.GetConfig().Whatsnyan.Tables.TBWhatsnyanIncomingMessage,
		config.GetConfig().Whatsnyan.Tables.TBWhatsnyanMessage,
		config.GetConfig().Telegram.Tables.TBTelegramIncomingMessage,
		config.GetConfig().Telegram.Tables.TBTelegramMessage,
		config.GetConfig().Telegram.Tables.TBTelegramUser,
		config.GetConfig().Database.TbWhatsappMessageAutoReply,
		config.GetConfig().Database.TbWhatsappMessage,
		config.GetConfig().Database.TbWhatsappUser,
		config.GetConfig().Database.TbWebAppConfig,
		config.GetConfig().Database.TbBadWord,
		config.GetConfig().Database.TbLanguage,
		config.GetConfig().Database.TbLogActivity,
		config.GetConfig().Database.TbFeature,
		config.GetConfig().Database.TbRolePrivilege,
		config.GetConfig().Database.TbRole,
		config.GetConfig().Database.TbUserPasswordChangeLog,
		config.GetConfig().Database.TbUserStatus,
		config.GetConfig().Database.TbUser,
	}

	for _, table := range tables {
		if err := db.Migrator().DropTable(table); err != nil {
			return err
		}
	}

	return nil
}
