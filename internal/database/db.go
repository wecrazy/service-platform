package database

import (
	"errors"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"service-platform/internal/config"
	"service-platform/internal/core/model"
	whatsnyanmodel "service-platform/internal/core/model/whatsnyan_model"
	"service-platform/internal/pkg/fun"
	"strings"
	"time"

	"github.com/sirupsen/logrus"
	"gopkg.in/natefinch/lumberjack.v2"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

type DBUsed struct {
	Main *gorm.DB
}

// Global databases
var DBList *DBUsed

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

		return db, nil
	default:
		return nil, errors.New("unsupported database type: " + dbType)
	}
}

func AutoMigrateDB(db *gorm.DB) {
	// Run migrations
	if err := db.AutoMigrate(
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

		// Whatsapp Models
		&model.WAUsers{},
		&model.WhatsappMessageAutoReply{},
		&whatsnyanmodel.WhatsAppGroup{},
		&whatsnyanmodel.WhatsAppGroupParticipant{},
		&whatsnyanmodel.WhatsAppMsg{},
		&whatsnyanmodel.WhatsAppIncomingMsg{},
	); err != nil {
		logrus.Fatalf("error while trying to automigrate db %v", err)
	}

	seedRoles(db)
	seedFeature(db)
	seedRolePrivilege(db)
	seedUser(db)
	seedUserStatus(db)
	seedUserPasswordChangeLog(db)
	seedWhatsappLanguage(db)
	seedWhatsappUser(db)
	seedBadWords(db)
	seedAppConfig(db)
	seedWhatsAppMsgAutoReply(db)
	seedIndonesiaRegion(db)
}

func tableExists(db *gorm.DB, tableName string) bool {
	dbType := config.GetConfig().Database.Type
	switch strings.ToLower(dbType) {
	case "postgres", "postgresql":
		var exists bool
		query := fmt.Sprintf("SELECT to_regclass('%s') IS NOT NULL", tableName)
		err := db.Raw(query).Scan(&exists).Error
		if err != nil {
			logrus.Errorf("tableExists: failed to check if table %s exists: %v", tableName, err)
			return false
		}
		return exists
	default:
		logrus.Warnf("tableExists: unsupported database type %s", dbType)
		return false
	}
}
