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
		config.ServicePlatform.Get().Whatsnyan.Tables.TBWhatsnyanGroupParticipant,
		config.ServicePlatform.Get().Whatsnyan.Tables.TBWhatsnyanGroup,
		config.ServicePlatform.Get().Whatsnyan.Tables.TBWhatsnyanIncomingMessage,
		config.ServicePlatform.Get().Whatsnyan.Tables.TBWhatsnyanMessage,
		config.ServicePlatform.Get().Telegram.Tables.TBTelegramIncomingMessage,
		config.ServicePlatform.Get().Telegram.Tables.TBTelegramMessage,
		config.ServicePlatform.Get().Telegram.Tables.TBTelegramUser,
		config.ServicePlatform.Get().Database.TbWhatsappMessageAutoReply,
		config.ServicePlatform.Get().Database.TbWhatsappMessage,
		config.ServicePlatform.Get().Database.TbWhatsappUser,
		config.ServicePlatform.Get().Database.TbWebAppConfig,
		config.ServicePlatform.Get().Database.TbBadWord,
		config.ServicePlatform.Get().Database.TbLanguage,
		config.ServicePlatform.Get().Database.TbLogActivity,
		config.ServicePlatform.Get().Database.TbFeature,
		config.ServicePlatform.Get().Database.TbRolePrivilege,
		config.ServicePlatform.Get().Database.TbRole,
		config.ServicePlatform.Get().Database.TbUserPasswordChangeLog,
		config.ServicePlatform.Get().Database.TbUserStatus,
		config.ServicePlatform.Get().Database.TbUser,
	}

	for _, table := range tables {
		if err := db.Migrator().DropTable(table); err != nil {
			return err
		}
	}

	return nil
}
