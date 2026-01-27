package migrations

import (
	"service-platform/internal/migrations"
	"service-platform/internal/seed"
	"time"

	"gorm.io/gorm"
)

func init() {
	migrations.RegisterMigration(&migrations.Migration{
		ID:        "002_initial_seed",
		Timestamp: time.Now().UTC(),
		Up:        upInitialSeed,
		Down:      downInitialSeed,
	})
}

func upInitialSeed(db *gorm.DB) error {
	// Use existing seeding functions to maintain consistency
	seed.SeedRoles(db)
	seed.SeedFeature(db)
	seed.SeedRolePrivilege(db)
	seed.SeedUser(db)
	seed.SeedUserStatus(db)
	seed.SeedUserPasswordChangeLog(db)
	seed.SeedWhatsappLanguage(db)
	seed.SeedWhatsappUser(db)
	seed.SeedBadWords(db)
	seed.SeedAppConfig(db)
	seed.SeedWhatsAppMsgAutoReply(db)
	seed.SeedIndonesiaRegion(db)
	seed.SeedTelegramUser(db)

	return nil
}

func downInitialSeed(db *gorm.DB) error {
	// Note: We don't delete seeded data in down migration
	// as it might be modified by users. Data cleanup should be manual.
	// This is a no-op rollback for seeding.
	return nil
}
