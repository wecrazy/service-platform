// Package database provides database connection management and schema migration functionality.
//
// This package handles:
// - PostgreSQL and MySQL database connection initialization with retry logic
// - Database creation and timezone configuration
// - Connection pooling and logging setup
// - Version-controlled database migrations using GORM AutoMigrate
// - Database health checks and connection validation
//
// The package uses GORM as the ORM and supports PostgreSQL and MySQL databases.
// Database configuration is read from the config package.
package database

import (
	"context"
	"errors"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"service-platform/internal/config"

	// _ "service-platform/internal/database/migrations" // Import migrations to register them
	"service-platform/internal/migrations"
	"service-platform/internal/pkg/fun"
	"strings"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"github.com/sirupsen/logrus"
	"gopkg.in/natefinch/lumberjack.v2"
	"gorm.io/driver/mysql"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// DBUsed holds references to different database connections used by the application.
// Currently supports a main database connection, with potential for future expansion
// to include read replicas, analytics databases, or other specialized connections.
type DBUsed struct {
	Main *gorm.DB // Main database connection for all application operations
}

// DBList is the global instance holding all database connections.
// This is initialized during application startup and used throughout the application.
var DBList *DBUsed

// Prometheus metrics for database monitoring
var (
	dbConnectionsTotal = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "db_connections_total",
		Help: "Total number of database connections established",
	}, []string{"db_type", "db_name"})

	dbConnectionErrorsTotal = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "db_connection_errors_total",
		Help: "Total number of database connection errors",
	}, []string{"db_type", "db_name"})

	dbHealthCheckDuration = promauto.NewHistogramVec(prometheus.HistogramOpts{
		Name:    "db_health_check_duration_seconds",
		Help:    "Duration of database health checks",
		Buckets: prometheus.DefBuckets,
	}, []string{"db_type", "db_name"})
)

// DBConfig holds the configuration parameters for database connections.
type DBConfig struct {
	Type              string
	User              string
	Password          string
	Host              string
	Port              int
	Database          string
	SSLMode           string
	MaxRetryConnect   int
	RetryDelay        int
	MaxIdleConnection int
	MaxOpenConnection int
	ConnMaxLifeTime   int
}

// InitAndCheckDB initializes and validates a PostgreSQL database connection.
//
// This function performs the following operations:
// 1. Connects to the default 'postgres' database to check/create the target database
// 2. Creates the target database if it doesn't exist
// 3. Sets the database timezone to UTC
// 4. Establishes connection to the target database with proper configuration
// 5. Sets up connection pooling and logging
//
// Parameters:
//   - dbType: Database type (currently only "postgres" is supported)
//   - dbUser: Database username
//   - dbPass: Database password
//   - dbHost: Database host address
//   - dbPort: Database port number
//   - dbName: Target database name
//   - dbSSLMode: SSL mode for database connection
//
// Returns:
//   - *gorm.DB: Configured GORM database instance
//   - error: Any error encountered during initialization
//
// The function implements retry logic for initial connection attempts and
// configures GORM with appropriate logging and connection pooling settings.
func InitAndCheckDB(
	dbType,
	dbUser,
	dbPass,
	dbHost string,
	dbPort int,
	dbName,
	dbSSLMode string,
) (*gorm.DB, error) {
	switch strings.ToLower(dbType) {
	case "postgres", "postgresql":
		// Connect to the default 'postgres' database to check and create the target database
		defaultDBURI := fmt.Sprintf(
			"host=%s user=%s password=%s dbname=postgres port=%d sslmode=%s TimeZone=UTC",
			dbHost, dbUser, dbPass, dbPort, dbSSLMode,
		)

		var defaultDB *gorm.DB
		var err error

		maxRetries := config.GetConfig().Database.MaxRetryConnect
		retryDelay := config.GetConfig().Database.RetryDelay

		// Retry connection to default database
		for attempt := 1; attempt <= maxRetries; attempt++ {
			defaultDB, err = gorm.Open(postgres.Open(defaultDBURI), &gorm.Config{})
			if err == nil {
				break
			}
			fmt.Printf("Attempt %d: failed to connect to postgres database: %v\n", attempt, err)
			time.Sleep(time.Duration(retryDelay) * time.Second)
		}

		// Check for failure after all attempts
		if err != nil || defaultDB == nil {
			return nil, fmt.Errorf("failed to connect to postgres database after %d attempts: %v", maxRetries, err)
		}

		// Check if the target db exists
		var dbExists bool
		query := fmt.Sprintf("SELECT EXISTS(SELECT 1 FROM pg_database WHERE datname = '%s')", dbName)
		err = defaultDB.Raw(query).Scan(&dbExists).Error
		if err != nil {
			return nil, fmt.Errorf("failed to check if database exists: %v", err)
		}

		if !dbExists {
			createDBQuery := fmt.Sprintf("CREATE DATABASE %s", dbName)
			err = defaultDB.Exec(createDBQuery).Error
			if err != nil {
				return nil, fmt.Errorf("failed to create database %s: %v", dbName, err)
			}
			fmt.Printf("Database %s created successfully.\n", dbName)
		}

		// Set the database's default timezone to UTC (for both new and existing databases)
		alterTZQuery := fmt.Sprintf("ALTER DATABASE %s SET timezone = 'UTC'", dbName)
		err = defaultDB.Exec(alterTZQuery).Error
		if err != nil {
			return nil, fmt.Errorf("failed to set database timezone to UTC: %v", err)
		}
		fmt.Printf("✅ Database %s timezone set to UTC.\n", dbName)

		// Close the connection to the default database
		dbSQL, err := defaultDB.DB()
		if err != nil {
			return nil, fmt.Errorf("failed to get sql.DB from gorm DB: %v", err)
		}
		defer dbSQL.Close()

		// Connect to the actual target database
		dbURI := fmt.Sprintf(
			"host=%s user=%s password=%s dbname=%s port=%d sslmode=%s TimeZone=UTC",
			dbHost, dbUser, dbPass, dbName, dbPort, dbSSLMode,
		)

		// Configure GORM logger
		logDir := config.GetConfig().App.LogDir
		if _, err := os.Stat(logDir); os.IsNotExist(err) {
			// fmt.Println("Cannot find the directory log try dynamic searching")
			logDir, err = fun.FindValidDirectory([]string{
				"log",
				"../log",
				"../../log",
				"../../../log",
			})
			if err != nil {
				fmt.Printf("Failed to find a valid log directory for db logger: %v\n", err)
				os.Exit(1)
			}
		}

		// Ensure log directory exists
		if err := os.MkdirAll(logDir, 0755); err != nil {
			fmt.Printf("Failed to create log directory: %v\n", err)
		}

		logFilePath := filepath.Join(logDir, "gorm_query.log")

		lumberjackLogger := &lumberjack.Logger{
			Filename:   logFilePath,
			MaxSize:    config.GetConfig().Default.LogMaxSize,
			MaxBackups: config.GetConfig().Default.LogMaxBackups,
			MaxAge:     config.GetConfig().Default.LogMaxAge,
			Compress:   config.GetConfig().Default.LogCompress,
		}

		logLevel := logger.Error
		ignoreRecordNotFoundError := true
		includeParams := true
		if config.GetConfig().App.Debug {
			logLevel = logger.Info
			ignoreRecordNotFoundError = false
			includeParams = false
		}

		newLogger := logger.New(
			log.New(lumberjackLogger, "\r\n", log.LstdFlags), // io writer
			logger.Config{
				SlowThreshold:             time.Second,               // Slow SQL threshold
				LogLevel:                  logLevel,                  // Log level
				IgnoreRecordNotFoundError: ignoreRecordNotFoundError, // Ignore ErrRecordNotFound error for logger
				ParameterizedQueries:      includeParams,             // Include params in the SQL log
				Colorful:                  false,                     // Disable color
			},
		)

		db, err := gorm.Open(postgres.Open(dbURI), &gorm.Config{
			Logger: newLogger,
		})
		if err != nil {
			dbConnectionErrorsTotal.WithLabelValues("postgres", dbName).Inc()
			return nil, fmt.Errorf("failed to connect to database %s: %v", dbName, err)
		}

		// Ensure timezone is set to UTC
		if err := db.Exec("SET timezone = 'UTC'").Error; err != nil {
			return nil, fmt.Errorf("failed to set timezone to UTC: %v", err)
		}

		sqlDB, err := db.DB()
		if err != nil {
			return nil, fmt.Errorf("failed to get sql.DB from gorm DB: %v", err)
		}

		// Set connection pool settings
		sqlDB.SetMaxIdleConns(config.GetConfig().Database.MaxIdleConnection)
		sqlDB.SetMaxOpenConns(config.GetConfig().Database.MaxOpenConnection)
		sqlDB.SetConnMaxLifetime(time.Duration(config.GetConfig().Database.ConnMaxLifeTime) * time.Minute)
		fmt.Println("✅ Connected to database: " + dbName)

		// Record successful connection
		dbConnectionsTotal.WithLabelValues("postgres", dbName).Inc()

		return db, nil
	case "mysql":
		// MySQL connection string
		dbURI := fmt.Sprintf(
			"%s:%s@tcp(%s:%d)/%s?charset=utf8mb4&parseTime=True&loc=Local",
			dbUser, dbPass, dbHost, dbPort, dbName,
		)

		// Configure GORM logger
		logDir := config.GetConfig().App.LogDir
		if _, err := os.Stat(logDir); os.IsNotExist(err) {
			logDir, err = fun.FindValidDirectory([]string{
				"log",
				"../log",
				"../../log",
				"../../../log",
			})
			if err != nil {
				fmt.Printf("Failed to find a valid log directory for db logger: %v\n", err)
				os.Exit(1)
			}
		}

		// Ensure log directory exists
		if err := os.MkdirAll(logDir, 0755); err != nil {
			fmt.Printf("Failed to create log directory: %v\n", err)
		}

		logFilePath := filepath.Join(logDir, "gorm_mysql_query.log")

		lumberjackLogger := &lumberjack.Logger{
			Filename:   logFilePath,
			MaxSize:    config.GetConfig().Default.LogMaxSize,
			MaxBackups: config.GetConfig().Default.LogMaxBackups,
			MaxAge:     config.GetConfig().Default.LogMaxAge,
			Compress:   config.GetConfig().Default.LogCompress,
		}

		logLevel := logger.Error
		ignoreRecordNotFoundError := true
		includeParams := true
		if config.GetConfig().App.Debug {
			logLevel = logger.Info
			ignoreRecordNotFoundError = false
			includeParams = false
		}

		newLogger := logger.New(
			log.New(lumberjackLogger, "\r\n", log.LstdFlags),
			logger.Config{
				SlowThreshold:             time.Second,
				LogLevel:                  logLevel,
				IgnoreRecordNotFoundError: ignoreRecordNotFoundError,
				ParameterizedQueries:      includeParams,
				Colorful:                  false,
			},
		)

		var db *gorm.DB
		var err error
		maxRetries := config.GetConfig().Database.MaxRetryConnect
		retryDelay := config.GetConfig().Database.RetryDelay

		for attempt := 1; attempt <= maxRetries; attempt++ {
			db, err = gorm.Open(mysql.Open(dbURI), &gorm.Config{
				Logger: newLogger,
			})
			if err == nil {
				break
			}
			fmt.Printf("Attempt %d: failed to connect to MySQL database: %v\n", attempt, err)
			dbConnectionErrorsTotal.WithLabelValues("mysql", dbName).Inc()
			time.Sleep(time.Duration(retryDelay) * time.Second)
		}

		if err != nil {
			return nil, fmt.Errorf("failed to connect to MySQL database after %d attempts: %v", maxRetries, err)
		}

		sqlDB, err := db.DB()
		if err != nil {
			return nil, fmt.Errorf("failed to get sql.DB from gorm DB: %v", err)
		}

		// Set connection pool settings
		sqlDB.SetMaxIdleConns(config.GetConfig().Database.MaxIdleConnection)
		sqlDB.SetMaxOpenConns(config.GetConfig().Database.MaxOpenConnection)
		sqlDB.SetConnMaxLifetime(time.Duration(config.GetConfig().Database.ConnMaxLifeTime) * time.Minute)
		fmt.Println("✅ Connected to MySQL database: " + dbName)

		// Record successful connection
		dbConnectionsTotal.WithLabelValues("mysql", dbName).Inc()

		return db, nil
	default:
		return nil, errors.New("unsupported database type: " + dbType)
	}
}

// InitDBConnection initializes a database connection without creating the database.
// This is suitable for MySQL databases where the database should already exist.
func InitDBConnection(
	dbCfg DBConfig,
) (*gorm.DB, error) {
	switch strings.ToLower(dbCfg.Type) {
	case "mysql":
		// MySQL connection string
		dbURI := fmt.Sprintf(
			"%s:%s@tcp(%s:%d)/%s?charset=utf8mb4&parseTime=True&loc=Local",
			dbCfg.User, dbCfg.Password, dbCfg.Host, dbCfg.Port, dbCfg.Database,
		)

		// Configure GORM logger
		logDir := config.GetConfig().App.LogDir
		if _, err := os.Stat(logDir); os.IsNotExist(err) {
			logDir, err = fun.FindValidDirectory([]string{
				"log",
				"../log",
				"../../log",
				"../../../log",
			})
			if err != nil {
				fmt.Printf("Failed to find a valid log directory for db logger: %v\n", err)
				os.Exit(1)
			}
		}

		// Ensure log directory exists
		if err := os.MkdirAll(logDir, 0755); err != nil {
			fmt.Printf("Failed to create log directory: %v\n", err)
		}

		logFilePath := filepath.Join(logDir, "gorm_mysql_query.log")

		lumberjackLogger := &lumberjack.Logger{
			Filename:   logFilePath,
			MaxSize:    config.GetConfig().Default.LogMaxSize,
			MaxBackups: config.GetConfig().Default.LogMaxBackups,
			MaxAge:     config.GetConfig().Default.LogMaxAge,
			Compress:   config.GetConfig().Default.LogCompress,
		}

		logLevel := logger.Error
		ignoreRecordNotFoundError := true
		includeParams := true
		if config.GetConfig().App.Debug {
			logLevel = logger.Info
			ignoreRecordNotFoundError = false
			includeParams = false
		}

		newLogger := logger.New(
			log.New(lumberjackLogger, "\r\n", log.LstdFlags),
			logger.Config{
				SlowThreshold:             time.Second,
				LogLevel:                  logLevel,
				IgnoreRecordNotFoundError: ignoreRecordNotFoundError,
				ParameterizedQueries:      includeParams,
				Colorful:                  false,
			},
		)

		var db *gorm.DB
		var err error
		maxRetries := dbCfg.MaxRetryConnect
		retryDelay := dbCfg.RetryDelay

		for attempt := 1; attempt <= maxRetries; attempt++ {
			db, err = gorm.Open(mysql.Open(dbURI), &gorm.Config{
				Logger: newLogger,
			})
			if err == nil {
				break
			}
			fmt.Printf("Attempt %d: failed to connect to MySQL database: %v\n", attempt, err)
			dbConnectionErrorsTotal.WithLabelValues("mysql", dbCfg.Database).Inc()
			time.Sleep(time.Duration(retryDelay) * time.Second)
		}

		if err != nil {
			return nil, fmt.Errorf("failed to connect to MySQL database after %d attempts: %v", maxRetries, err)
		}

		sqlDB, err := db.DB()
		if err != nil {
			return nil, fmt.Errorf("failed to get sql.DB from gorm DB: %v", err)
		}

		// Set connection pool settings
		sqlDB.SetMaxIdleConns(dbCfg.MaxIdleConnection)
		sqlDB.SetMaxOpenConns(dbCfg.MaxOpenConnection)
		sqlDB.SetConnMaxLifetime(time.Duration(dbCfg.ConnMaxLifeTime) * time.Minute)
		fmt.Println("✅ Connected to MySQL database: " + dbCfg.Database)

		// Record successful connection
		dbConnectionsTotal.WithLabelValues("mysql", dbCfg.Database).Inc()

		return db, nil
	default:
		return nil, errors.New("unsupported database type for InitDBConnection: " + dbCfg.Type)
	}
}

// This function replaces the traditional GORM AutoMigrate approach with a
// version-controlled migration system that provides:
//
// - Schema change tracking and versioning
// - Reversible migrations (up/down operations)
// - Migration status monitoring
// - Safe rollback capabilities
// - Integration with existing GORM AutoMigrate logic
//
// The function runs all pending migrations in order, ensuring that:
// 1. Database schema is created/updated using GORM AutoMigrate within migrations
// 2. Initial data seeding is performed through migration-based seeding
// 3. All changes are tracked in the schema_migrations table
//
// Parameters:
//   - db: GORM database instance to migrate
//
// The function will terminate the application with a fatal error if migrations fail,
// ensuring that the application only runs with a properly migrated database.
//
// Note: This replaces the old direct AutoMigrate + seeding approach with
// a more robust, version-controlled system.
func AutoMigrateDB(db *gorm.DB) {
	// Run version-controlled migrations instead of AutoMigrate
	// This ensures schema changes are tracked and reversible
	if err := migrations.RunMigrations(db); err != nil {
		logrus.Fatalf("error while running database migrations: %v", err)
	}

	// Note: Seeding is now handled by migrations
	// The old seeding functions are kept for backward compatibility
	// but should be removed once migration-based seeding is confirmed working
}

// GetMainDB returns the main database connection
func GetMainDB() *gorm.DB {
	if DBList == nil || DBList.Main == nil {
		logrus.Fatal("Main database not initialized")
	}
	return DBList.Main
}

// HealthCheckDB performs a health check on a database connection with Prometheus metrics
func HealthCheckDB(db *gorm.DB, dbType, dbName string) error {
	timer := prometheus.NewTimer(dbHealthCheckDuration.WithLabelValues(dbType, dbName))
	defer timer.ObserveDuration()

	sqlDB, err := db.DB()
	if err != nil {
		return fmt.Errorf("failed to get sql.DB: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := sqlDB.PingContext(ctx); err != nil {
		return fmt.Errorf("database ping failed: %v", err)
	}

	return nil
}
