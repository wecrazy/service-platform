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

func main() {
	var (
		only = flag.String("only", "", "Run only specific seed group: users, whatsapp, telegram, config, all")
	)
	flag.Parse()

	// Initialize configuration
	if err := config.LoadConfig(); err != nil {
		log.Fatalf("Failed to initialize config: %v", err)
	}

	// Initialize database connection
	db, err := database.InitAndCheckDB(
		config.GetConfig().Database.Type,
		config.GetConfig().Database.Username,
		config.GetConfig().Database.Password,
		config.GetConfig().Database.Host,
		config.GetConfig().Database.Port,
		config.GetConfig().Database.Name,
		config.GetConfig().Database.SSLMode,
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

	default:
		logrus.Fatalf("Unknown seed group: %s. Use: users, whatsapp, telegram, config, all", *only)
	}

	fmt.Println("✅ Database seeding completed successfully!")
}
