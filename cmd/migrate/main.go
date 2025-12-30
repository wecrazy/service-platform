package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"service-platform/internal/config"
	"service-platform/internal/database"
	"service-platform/internal/migrations"

	"github.com/sirupsen/logrus"
)

// Main entry point for the migration CLI tool
func main() {
	var (
		action = flag.String("action", "up", "Migration action: up, down, status, reset")
		steps  = flag.Int("steps", 1, "Number of steps to rollback (for down action)")
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

	// Initialize migration service with dependency injection
	migrationService := migrations.NewMigrationService(db, config.GetConfig(), logrus.StandardLogger())

	// Execute migration action
	switch *action {
	case "up":
		fmt.Println("🚀 Running database migrations...")
		if err := migrationService.RunMigrations(); err != nil {
			logrus.Fatalf("Migration failed: %v", err)
		}
		fmt.Println("✅ Migrations completed successfully")

	case "down":
		fmt.Printf("⬇️ Rolling back %d migration(s)...\n", *steps)
		if err := migrationService.RollbackMigrations(*steps); err != nil {
			logrus.Fatalf("Rollback failed: %v", err)
		}
		fmt.Println("✅ Rollback completed successfully")

	case "status":
		fmt.Println("📊 Checking migration status...")
		if err := migrationService.GetMigrationStatus(); err != nil {
			logrus.Fatalf("Failed to get migration status: %v", err)
		}

	case "reset":
		fmt.Println("⚠️ Resetting all migrations (this will drop all tables)...")
		fmt.Print("Are you sure? Type 'yes' to confirm: ")
		var confirmation string
		fmt.Scanln(&confirmation)
		if confirmation != "yes" {
			fmt.Println("Reset cancelled")
			os.Exit(0)
		}

		// Get all applied migrations and rollback them
		applied, err := migrationService.GetAppliedMigrations()
		if err != nil {
			logrus.Fatalf("Failed to get applied migrations: %v", err)
		}

		if len(applied) == 0 {
			fmt.Println("No migrations to reset")
			return
		}

		if err := migrationService.RollbackMigrations(len(applied)); err != nil {
			logrus.Fatalf("Reset failed: %v", err)
		}
		fmt.Println("✅ Reset completed successfully")

	default:
		logrus.Fatalf("Unknown action: %s. Use: up, down, status, reset", *action)
	}
}
