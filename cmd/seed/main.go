// Package main is the entry point for the database seeding tool.
//
// It populates the PostgreSQL database with initial/reference data needed by the
// Service Platform (roles, features, privileges, users, WhatsApp/Telegram settings,
// bad-word filters, app config, and Indonesia region data).
//
// Seeds can be run selectively via the -only flag or all at once (default).
//
// Flags:
//
//	-only string   Seed group to run: users, whatsapp, telegram, config, all (default: all)
//
// Usage:
//
//	go run -tags=seed cmd/seed/main.go               # seed everything
//	go run -tags=seed cmd/seed/main.go -only=users    # seed user-related tables only
//	make seed                                          # Makefile shorthand
//	make seed-whatsapp                                 # seed WhatsApp data only
package main

import (
	"flag"
	"fmt"
	"log"
	"service-platform/internal/config"
	"service-platform/internal/database"
	"service-platform/internal/seed"

	"github.com/sirupsen/logrus"
)

// main parses flags, connects to the database, and executes the selected seed
// group(s). The connection is closed automatically on return.
func main() {
	var (
		only = flag.String("only", "", "Run only specific seed group: users, whatsapp, telegram, config, all")
	)
	flag.Parse()

	config.ServicePlatform.MustInit("service-platform") // Load config with name "service-platform.%s.yaml"

	yamlCfg := config.ServicePlatform.Get()

	// Initialize database connection
	db, err := database.InitAndCheckDB(
		yamlCfg.Database.Type,
		yamlCfg.Database.Username,
		yamlCfg.Database.Password,
		yamlCfg.Database.Host,
		yamlCfg.Database.Port,
		yamlCfg.Database.Name,
		yamlCfg.Database.SSLMode,
	)
	if err != nil {
		log.Fatalf("Failed to connect to database: %v", err)
	}

	// Ensure database connection is closed
	sqlDB, _ := db.DB()
	defer sqlDB.Close()

	fmt.Println("🌱 Starting database seeding...")

	switch *only {
	case "users":
		fmt.Println("👥 Seeding user-related data...")
		seed.SeedRoles(db)
		seed.SeedFeature(db)
		seed.SeedRolePrivilege(db)
		seed.SeedUser(db)
		seed.SeedUserStatus(db)
		seed.SeedUserPasswordChangeLog(db)

	case "whatsapp":
		fmt.Println("📱 Seeding WhatsApp data...")
		seed.SeedWhatsappLanguage(db)
		seed.SeedWhatsappUser(db)
		seed.SeedWhatsAppMsgAutoReply(db)

	case "telegram":
		fmt.Println("📱 Seeding Telegram data...")
		seed.SeedTelegramUser(db)
		seed.SeedTelegramUserOfSACMS(db)

	case "config":
		fmt.Println("⚙️ Seeding configuration data...")
		seed.SeedBadWords(db)
		seed.SeedAppConfig(db)
		seed.SeedIndonesiaRegion(db)

	case "all", "":
		fmt.Println("🌱 Running all seeds...")
		seed.SeedRoles(db)
		seed.SeedFeature(db)
		seed.SeedRolePrivilege(db)
		seed.SeedUser(db)
		seed.SeedUserStatus(db)
		seed.SeedUserPasswordChangeLog(db)
		seed.SeedWhatsappLanguage(db)
		seed.SeedWhatsappUser(db)
		seed.SeedBadWords(db)
		seed.SeedWhatsAppMsgAutoReply(db)
		seed.SeedAppConfig(db)
		seed.SeedIndonesiaRegion(db)
		seed.SeedTelegramUser(db)
		seed.SeedTelegramUserOfSACMS(db)

	default:
		logrus.Fatalf("Unknown seed group: %s. Use: users, whatsapp, telegram, config, all", *only)
	}

	fmt.Println("✅ Database seeding completed successfully!")
}
