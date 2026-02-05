package migrations

import (
	"fmt"
	"service-platform/internal/config"
	"strings"
	"time"

	"github.com/sirupsen/logrus"
	"gorm.io/gorm"
)

// MigrationService provides database migration functionality with dependency injection.
// It encapsulates all migration operations and uses injected dependencies instead of global state.
type MigrationService struct {
	db     *gorm.DB          // Database connection
	config config.TypeConfig // Application configuration
	logger *logrus.Logger    // Logger instance
}

// NewMigrationService creates a new MigrationService with injected dependencies.
func NewMigrationService(db *gorm.DB, cfg config.TypeConfig, logger *logrus.Logger) *MigrationService {
	return &MigrationService{
		db:     db,
		config: cfg,
		logger: logger,
	}
}

// createMigrationsTable creates the schema_migrations table if it doesn't exist
func (s *MigrationService) createMigrationsTable() error {
	dbType := s.config.Database.Type
	switch strings.ToLower(dbType) {
	case "postgres", "postgresql":
		return s.db.Exec(`
			CREATE TABLE IF NOT EXISTS schema_migrations (
				id VARCHAR(255) PRIMARY KEY,
				applied_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
			)
		`).Error
	case "mysql", "mariadb":
		return s.db.Exec(`
			CREATE TABLE IF NOT EXISTS schema_migrations (
				id VARCHAR(255) PRIMARY KEY,
				applied_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
			)
		`).Error
	case "sqlite", "sqlite3":
		return s.db.Exec(`
			CREATE TABLE IF NOT EXISTS schema_migrations (
				id TEXT PRIMARY KEY,
				applied_at DATETIME DEFAULT CURRENT_TIMESTAMP
			)
		`).Error
	default:
		s.logger.Warnf("createMigrationsTable: unsupported database type %s, defaulting to PostgreSQL syntax", dbType)
		return s.db.Exec(`
			CREATE TABLE IF NOT EXISTS schema_migrations (
				id VARCHAR(255) PRIMARY KEY,
				applied_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
			)
		`).Error
	}
}

// recordMigration records a migration as applied
func (s *MigrationService) recordMigration(id string) error {
	return s.db.Exec("INSERT INTO schema_migrations (id) VALUES (?)", id).Error
}

// removeMigrationRecord removes a migration record
func (s *MigrationService) removeMigrationRecord(id string) error {
	return s.db.Exec("DELETE FROM schema_migrations WHERE id = ?", id).Error
}

// getAppliedMigrations returns a map of applied migration IDs and their timestamps
func (s *MigrationService) getAppliedMigrations() (map[string]time.Time, error) {
	var migrations []struct {
		ID        string
		AppliedAt time.Time
	}
	err := s.db.Table("schema_migrations").Find(&migrations).Error
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
func (s *MigrationService) RunMigrations() error {
	// Create migrations table if it doesn't exist
	if err := s.createMigrationsTable(); err != nil {
		return fmt.Errorf("failed to create migrations table: %w", err)
	}

	// Get applied migrations
	applied, err := s.getAppliedMigrations()
	if err != nil {
		return fmt.Errorf("failed to get applied migrations: %w", err)
	}

	// Get all available migrations
	allMigrations := GetSortedMigrations()

	// Run pending migrations
	for _, migration := range allMigrations {
		if _, exists := applied[migration.ID]; !exists {
			s.logger.Infof("Running migration: %s", migration.ID)
			if err := migration.Up(s.db); err != nil {
				return fmt.Errorf("migration %s failed: %w", migration.ID, err)
			}
			// Record migration as applied
			if err := s.recordMigration(migration.ID); err != nil {
				return fmt.Errorf("failed to record migration %s: %w", migration.ID, err)
			}
			s.logger.Infof("✅ Migration %s completed successfully", migration.ID)
		}
	}

	s.logger.Info("✅ All migrations completed successfully")
	return nil
}

// RollbackMigrations rolls back the last n migrations
func (s *MigrationService) RollbackMigrations(steps int) error {
	// Get applied migrations in reverse order
	applied, err := s.getAppliedMigrations()
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
		s.logger.Infof("Rolling back migration: %s", migration.ID)
		if err := migration.Down(s.db); err != nil {
			return fmt.Errorf("rollback of migration %s failed: %w", migration.ID, err)
		}
		// Remove migration record
		if err := s.removeMigrationRecord(migration.ID); err != nil {
			return fmt.Errorf("failed to remove migration record %s: %w", migration.ID, err)
		}
		s.logger.Infof("✅ Migration %s rolled back successfully", migration.ID)
	}

	return nil
}

// GetMigrationStatus returns the status of all migrations
func (s *MigrationService) GetMigrationStatus() error {
	// Create migrations table if it doesn't exist
	if err := s.createMigrationsTable(); err != nil {
		return fmt.Errorf("failed to create migrations table: %w", err)
	}

	// Get applied migrations
	applied, err := s.getAppliedMigrations()
	if err != nil {
		return fmt.Errorf("failed to get applied migrations: %w", err)
	}

	allMigrations := GetSortedMigrations()

	s.logger.Info("Migration Status:")
	s.logger.Info("=================")
	for _, migration := range allMigrations {
		status := "❌ Pending"
		if _, exists := applied[migration.ID]; exists {
			status = "✅ Applied"
		}
		s.logger.Infof("%s: %s", migration.ID, status)
	}

	return nil
}

// GetAppliedMigrations returns a map of applied migration IDs and their timestamps
func (s *MigrationService) GetAppliedMigrations() (map[string]time.Time, error) {
	return s.getAppliedMigrations()
}
