package migrations

import (
	"testing"
	"time"

	"service-platform/internal/config"

	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/suite"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

// MigrationTestSuite provides comprehensive testing for the MigrationService.
// It tests all migration operations including running, rolling back, and status checking.
type MigrationTestSuite struct {
	suite.Suite
	db           *gorm.DB
	config       config.TypeServicePlatform
	logger       *logrus.Logger
	migrationSvc *MigrationService
}

// SetupTest initializes the test suite with an in-memory SQLite database
// and creates a MigrationService instance for testing.
func (suite *MigrationTestSuite) SetupTest() {
	var err error

	// Clear the global migration registry for each test
	MigrationRegistry = make(map[string]*Migration)

	// Setup in-memory SQLite database
	suite.db, err = gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	suite.Require().NoError(err)

	// Setup test config
	suite.config = config.TypeServicePlatform{}
	suite.config.Database.Type = "sqlite"

	// Setup logger
	suite.logger = logrus.New()
	suite.logger.SetLevel(logrus.InfoLevel)

	// Create migration service
	suite.migrationSvc = NewMigrationService(suite.db, suite.config, suite.logger)
}

// TearDownTest cleans up after each test.
func (suite *MigrationTestSuite) TearDownTest() {
	sqlDB, _ := suite.db.DB()
	sqlDB.Close()
}

// TestNewMigrationService tests the constructor for MigrationService.
func (suite *MigrationTestSuite) TestNewMigrationService() {
	svc := NewMigrationService(suite.db, suite.config, suite.logger)
	suite.NotNil(svc)
	suite.Equal(suite.db, svc.db)
	suite.Equal(suite.config, svc.config)
	suite.Equal(suite.logger, svc.logger)
}

// TestCreateMigrationsTable tests the creation of the schema_migrations table.
func (suite *MigrationTestSuite) TestCreateMigrationsTable() {
	err := suite.migrationSvc.createMigrationsTable()
	suite.NoError(err)

	// Verify table was created
	var count int64
	err = suite.db.Table("schema_migrations").Count(&count).Error
	suite.NoError(err)
	suite.Equal(int64(0), count)
}

// TestRecordMigration tests recording a migration as applied.
func (suite *MigrationTestSuite) TestRecordMigration() {
	// Create migrations table first
	err := suite.migrationSvc.createMigrationsTable()
	suite.NoError(err)

	// Record a migration
	err = suite.migrationSvc.recordMigration("test_migration_001")
	suite.NoError(err)

	// Verify migration was recorded
	var migrations []struct {
		ID        string
		AppliedAt time.Time
	}
	err = suite.db.Table("schema_migrations").Find(&migrations).Error
	suite.NoError(err)
	suite.Len(migrations, 1)
	suite.Equal("test_migration_001", migrations[0].ID)
	suite.NotZero(migrations[0].AppliedAt)
}

// TestRemoveMigrationRecord tests removing a migration record.
func (suite *MigrationTestSuite) TestRemoveMigrationRecord() {
	// Create migrations table and record a migration
	err := suite.migrationSvc.createMigrationsTable()
	suite.NoError(err)
	err = suite.migrationSvc.recordMigration("test_migration_001")
	suite.NoError(err)

	// Remove the migration record
	err = suite.migrationSvc.removeMigrationRecord("test_migration_001")
	suite.NoError(err)

	// Verify migration was removed
	var count int64
	err = suite.db.Table("schema_migrations").Count(&count).Error
	suite.NoError(err)
	suite.Equal(int64(0), count)
}

// TestGetAppliedMigrations tests retrieving applied migrations.
func (suite *MigrationTestSuite) TestGetAppliedMigrations() {
	// Create migrations table and record some migrations
	err := suite.migrationSvc.createMigrationsTable()
	suite.NoError(err)
	err = suite.migrationSvc.recordMigration("migration_001")
	suite.NoError(err)
	err = suite.migrationSvc.recordMigration("migration_002")
	suite.NoError(err)

	// Get applied migrations
	applied, err := suite.migrationSvc.getAppliedMigrations()
	suite.NoError(err)
	suite.Len(applied, 2)
	suite.Contains(applied, "migration_001")
	suite.Contains(applied, "migration_002")
}

// TestGetMigrationStatus tests the migration status functionality.
func (suite *MigrationTestSuite) TestGetMigrationStatus() {
	// Register a test migration
	testMigration := &Migration{
		ID:        "test_status_migration",
		Timestamp: time.Now(),
		Up: func(db *gorm.DB) error {
			return db.Exec("CREATE TABLE test_table (id INTEGER PRIMARY KEY)").Error
		},
		Down: func(db *gorm.DB) error {
			return db.Exec("DROP TABLE test_table").Error
		},
	}
	RegisterMigration(testMigration)

	// Get migration status (should show as pending)
	err := suite.migrationSvc.GetMigrationStatus()
	suite.NoError(err)

	// Apply the migration
	err = suite.migrationSvc.RunMigrations()
	suite.NoError(err)

	// Get migration status again (should show as applied)
	err = suite.migrationSvc.GetMigrationStatus()
	suite.NoError(err)
}

// TestRunMigrations tests running migrations.
func (suite *MigrationTestSuite) TestRunMigrations() {
	// Register a test migration
	testMigration := &Migration{
		ID:        "test_run_migration",
		Timestamp: time.Now(),
		Up: func(db *gorm.DB) error {
			return db.Exec("CREATE TABLE test_run_table (id INTEGER PRIMARY KEY, name TEXT)").Error
		},
		Down: func(db *gorm.DB) error {
			return db.Exec("DROP TABLE test_run_table").Error
		},
	}
	RegisterMigration(testMigration)

	// Run migrations
	err := suite.migrationSvc.RunMigrations()
	suite.NoError(err)

	// Verify migration was applied
	var count int64
	err = suite.db.Table("schema_migrations").Where("id = ?", "test_run_migration").Count(&count).Error
	suite.NoError(err)
	suite.Equal(int64(1), count)

	// Verify table was created
	err = suite.db.Table("test_run_table").Count(&count).Error
	suite.NoError(err) // Should not error if table exists
}

// TestRollbackMigrations tests rolling back migrations.
func (suite *MigrationTestSuite) TestRollbackMigrations() {
	// Register a test migration
	testMigration := &Migration{
		ID:        "test_rollback_migration",
		Timestamp: time.Now(),
		Up: func(db *gorm.DB) error {
			return db.Exec("CREATE TABLE test_rollback_table (id INTEGER PRIMARY KEY)").Error
		},
		Down: func(db *gorm.DB) error {
			return db.Exec("DROP TABLE test_rollback_table").Error
		},
	}
	RegisterMigration(testMigration)

	// Run migration first
	err := suite.migrationSvc.RunMigrations()
	suite.NoError(err)

	// Verify migration was applied
	var count int64
	err = suite.db.Table("schema_migrations").Where("id = ?", "test_rollback_migration").Count(&count).Error
	suite.NoError(err)
	suite.Equal(int64(1), count)

	// Rollback migration
	err = suite.migrationSvc.RollbackMigrations(1)
	suite.NoError(err)

	// Verify migration was removed from schema_migrations
	err = suite.db.Table("schema_migrations").Where("id = ?", "test_rollback_migration").Count(&count).Error
	suite.NoError(err)
	suite.Equal(int64(0), count)
}

// TestRollbackMultipleMigrations tests rolling back multiple migrations.
func (suite *MigrationTestSuite) TestRollbackMultipleMigrations() {
	// Register multiple test migrations
	migration1 := &Migration{
		ID:        "test_multi_rollback_001",
		Timestamp: time.Now().Add(-time.Hour),
		Up: func(db *gorm.DB) error {
			return db.Exec("CREATE TABLE test_multi_001 (id INTEGER PRIMARY KEY)").Error
		},
		Down: func(db *gorm.DB) error {
			return db.Exec("DROP TABLE test_multi_001").Error
		},
	}
	migration2 := &Migration{
		ID:        "test_multi_rollback_002",
		Timestamp: time.Now(),
		Up: func(db *gorm.DB) error {
			return db.Exec("CREATE TABLE test_multi_002 (id INTEGER PRIMARY KEY)").Error
		},
		Down: func(db *gorm.DB) error {
			return db.Exec("DROP TABLE test_multi_002").Error
		},
	}
	RegisterMigration(migration1)
	RegisterMigration(migration2)

	// Run migrations
	err := suite.migrationSvc.RunMigrations()
	suite.NoError(err)

	// Verify both migrations were applied
	var count int64
	err = suite.db.Table("schema_migrations").Count(&count).Error
	suite.NoError(err)
	suite.Equal(int64(2), count)

	// Rollback 2 migrations
	err = suite.migrationSvc.RollbackMigrations(2)
	suite.NoError(err)

	// Verify all migrations were removed
	err = suite.db.Table("schema_migrations").Count(&count).Error
	suite.NoError(err)
	suite.Equal(int64(0), count)
}

// TestMigrationFailureHandling tests that migration failures are handled properly.
func (suite *MigrationTestSuite) TestMigrationFailureHandling() {
	// Register a migration that will fail
	failingMigration := &Migration{
		ID:        "test_failing_migration_unique",
		Timestamp: time.Now(),
		Up: func(db *gorm.DB) error {
			return db.Exec("INVALID SQL STATEMENT").Error // This will fail
		},
		Down: func(db *gorm.DB) error {
			return nil
		},
	}
	RegisterMigration(failingMigration)

	// Attempt to run migrations (should fail)
	err := suite.migrationSvc.RunMigrations()
	suite.Error(err)
	suite.Contains(err.Error(), "test_failing_migration_unique failed")

	// Verify migration was not recorded as applied
	var count int64
	err = suite.db.Table("schema_migrations").Where("id = ?", "test_failing_migration_unique").Count(&count).Error
	suite.NoError(err)
	suite.Equal(int64(0), count)
}

// TestGetAppliedMigrationsPublic tests the public GetAppliedMigrations method.
func (suite *MigrationTestSuite) TestGetAppliedMigrationsPublic() {
	// Create migrations table and record a migration
	err := suite.migrationSvc.createMigrationsTable()
	suite.NoError(err)
	err = suite.migrationSvc.recordMigration("public_test_migration")
	suite.NoError(err)

	// Test public method
	applied, err := suite.migrationSvc.GetAppliedMigrations()
	suite.NoError(err)
	suite.Len(applied, 1)
	suite.Contains(applied, "public_test_migration")
}

// TestSuite runs the test suite.
func TestMigrationTestSuite(t *testing.T) {
	suite.Run(t, new(MigrationTestSuite))
}
