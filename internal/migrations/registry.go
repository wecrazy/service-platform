package migrations

import (
	"fmt"
	"service-platform/internal/config"
	"sort"
	"strings"
	"time"

	"github.com/sirupsen/logrus"
	"gorm.io/gorm"
)

// Migration represents a database migration
type Migration struct {
	ID        string
	Timestamp time.Time
	Up        func(*gorm.DB) error
	Down      func(*gorm.DB) error
}

// MigrationRegistry holds all registered migrations
var MigrationRegistry = make(map[string]*Migration)

// RegisterMigration adds a migration to the registry
func RegisterMigration(migration *Migration) {
	MigrationRegistry[migration.ID] = migration
}

// GetSortedMigrations returns migrations sorted by ID
func GetSortedMigrations() []*Migration {
	var migrations []*Migration
	for _, m := range MigrationRegistry {
		migrations = append(migrations, m)
	}
	sort.Slice(migrations, func(i, j int) bool {
		return migrations[i].ID < migrations[j].ID
	})
	return migrations
}

// createMigrationsTable creates the schema_migrations table if it doesn't exist
func createMigrationsTable(db *gorm.DB) error {
	dbType := config.GetConfig().Database.Type
	switch strings.ToLower(dbType) {
	case "postgres", "postgresql":
		return db.Exec(`
			CREATE TABLE IF NOT EXISTS schema_migrations (
				id VARCHAR(255) PRIMARY KEY,
				applied_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
			)
		`).Error
	case "mysql", "mariadb":
		return db.Exec(`
			CREATE TABLE IF NOT EXISTS schema_migrations (
				id VARCHAR(255) PRIMARY KEY,
				applied_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
			)
		`).Error
	default:
		logrus.Warnf("createMigrationsTable: unsupported database type %s, defaulting to PostgreSQL syntax", dbType)
		return db.Exec(`
			CREATE TABLE IF NOT EXISTS schema_migrations (
				id VARCHAR(255) PRIMARY KEY,
				applied_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
			)
		`).Error
	}
}

// recordMigration records a migration as applied
func recordMigration(db *gorm.DB, id string) error {
	return db.Exec("INSERT INTO schema_migrations (id) VALUES (?)", id).Error
}

// removeMigrationRecord removes a migration record
func removeMigrationRecord(db *gorm.DB, id string) error {
	return db.Exec("DELETE FROM schema_migrations WHERE id = ?", id).Error
}

// getAppliedMigrations returns a map of applied migration IDs and their timestamps
func getAppliedMigrations(db *gorm.DB) (map[string]time.Time, error) {
	var migrations []struct {
		ID        string
		AppliedAt time.Time
	}
	err := db.Table("schema_migrations").Find(&migrations).Error
	if err != nil {
		return nil, err
	}

	result := make(map[string]time.Time)
	for _, m := range migrations {
		result[m.ID] = m.AppliedAt
	}
	return result, nil
}

// RunMigrations executes all pending migrations
func RunMigrations(db *gorm.DB) error {
	// Create migrations table if it doesn't exist
	if err := createMigrationsTable(db); err != nil {
		return fmt.Errorf("failed to create migrations table: %w", err)
	}

	// Get applied migrations
	applied, err := getAppliedMigrations(db)
	if err != nil {
		return fmt.Errorf("failed to get applied migrations: %w", err)
	}

	// Get all available migrations
	allMigrations := GetSortedMigrations()

	// Run pending migrations
	for _, migration := range allMigrations {
		if _, exists := applied[migration.ID]; !exists {
			logrus.Infof("Running migration: %s", migration.ID)
			if err := migration.Up(db); err != nil {
				return fmt.Errorf("migration %s failed: %w", migration.ID, err)
			}
			// Record migration as applied
			if err := recordMigration(db, migration.ID); err != nil {
				return fmt.Errorf("failed to record migration %s: %w", migration.ID, err)
			}
			logrus.Infof("✅ Migration %s completed successfully", migration.ID)
		}
	}

	logrus.Info("✅ All migrations completed successfully")
	return nil
}

// RollbackMigrations rolls back the last n migrations
func RollbackMigrations(db *gorm.DB, steps int) error {
	// Get applied migrations in reverse order
	applied, err := getAppliedMigrations(db)
	if err != nil {
		return fmt.Errorf("failed to get applied migrations: %w", err)
	}

	allMigrations := GetSortedMigrations()

	// Get migrations to rollback (in reverse order)
	var toRollback []*Migration
	for i := len(allMigrations) - 1; i >= 0; i-- {
		migration := allMigrations[i]
		if _, exists := applied[migration.ID]; exists {
			toRollback = append(toRollback, migration)
			if len(toRollback) >= steps {
				break
			}
		}
	}

	// Rollback migrations
	for _, migration := range toRollback {
		logrus.Infof("Rolling back migration: %s", migration.ID)
		if err := migration.Down(db); err != nil {
			return fmt.Errorf("rollback of migration %s failed: %w", migration.ID, err)
		}
		// Remove migration record
		if err := removeMigrationRecord(db, migration.ID); err != nil {
			return fmt.Errorf("failed to remove migration record %s: %w", migration.ID, err)
		}
		logrus.Infof("✅ Migration %s rolled back successfully", migration.ID)
	}

	return nil
}

// GetMigrationStatus returns the status of all migrations
func GetMigrationStatus(db *gorm.DB) error {
	// Create migrations table if it doesn't exist
	if err := createMigrationsTable(db); err != nil {
		return fmt.Errorf("failed to create migrations table: %w", err)
	}

	// Get applied migrations
	applied, err := getAppliedMigrations(db)
	if err != nil {
		return fmt.Errorf("failed to get applied migrations: %w", err)
	}

	allMigrations := GetSortedMigrations()

	logrus.Info("Migration Status:")
	logrus.Info("=================")
	for _, migration := range allMigrations {
		status := "❌ Pending"
		if _, exists := applied[migration.ID]; exists {
			status = "✅ Applied"
		}
		logrus.Infof("%s: %s", migration.ID, status)
	}

	return nil
}

// GetAppliedMigrations returns a map of applied migration IDs and their timestamps
func GetAppliedMigrations(db *gorm.DB) (map[string]time.Time, error) {
	return getAppliedMigrations(db)
}
