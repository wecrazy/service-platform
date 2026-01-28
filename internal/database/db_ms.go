// Package database provides MySQL database connections for Technical Assistance and Manage Service
package database

import (
	"fmt"
	"log"
	"time"

	"service-platform/internal/config"

	"github.com/sirupsen/logrus"
	"gorm.io/gorm"
)

var (
	MySQLDBTA           *gorm.DB // MySQLDBTA holds the GORM DB connection for Dashboard Technical Assistance - Manage Service Integration of MySQL database
	MySQLDBMSMiddleware *gorm.DB // MySQLDBMSMiddleware holds the GORM DB connection for Middleware Microservice of MySQL database
)

// InitDBMS initializes the Manage Service MySQL database connection
func InitDBMS() error {
	cfg := config.GetConfig()

	dbCfgMS := DBConfig{
		Type:              "MySQL",
		User:              cfg.MSMiddleware.MySQLDBUser,
		Password:          cfg.MSMiddleware.MySQLDBPass,
		Host:              cfg.MSMiddleware.MySQLDBHost,
		Port:              cfg.MSMiddleware.MySQLDBPort,
		Database:          cfg.MSMiddleware.MySQLDBName,
		SSLMode:           cfg.MSMiddleware.MySQLDBSSLMode,
		MaxRetryConnect:   cfg.MSMiddleware.MySQLDBMaxRetryConnect,
		RetryDelay:        cfg.MSMiddleware.MySQLDBRetryDelay,
		MaxIdleConnection: cfg.MSMiddleware.MySQLDBIdleConnection,
		MaxOpenConnection: cfg.MSMiddleware.MySQLDBOpenConnection,
		ConnMaxLifeTime:   cfg.MSMiddleware.MySQLDBConnMaxLifetime,
	}

	db, err := InitDBConnection(
		dbCfgMS,
	)
	if err != nil {
		logrus.Errorf("Failed to initialize MS database: %v", err)
		return err
	}

	MySQLDBMSMiddleware = db

	log.Println("✅ MS MySQL database initialized successfully")
	return nil
}

// GetDBMS returns the Manage Service Middleware database connection
func GetDBMS() *gorm.DB {
	if MySQLDBMSMiddleware == nil {
		if err := InitDBMS(); err != nil {
			logrus.Fatalf("Failed to initialize MS database: %v", err)
		}
	}
	return MySQLDBMSMiddleware
}

// CloseDBMS closes the Manage Service database connection
func CloseDBMS() error {
	if MySQLDBMSMiddleware != nil {
		sqlDB, err := MySQLDBMSMiddleware.DB()
		if err != nil {
			return fmt.Errorf("failed to get sql.DB from MS DB: %v", err)
		}
		if err := sqlDB.Close(); err != nil {
			return fmt.Errorf("failed to close MS database: %v", err)
		}
		log.Println("Disconnected from MS MySQL database")
	}
	return nil
}

// HealthCheckDBMS checks if the MS database connection is healthy
func HealthCheckDBMS() error {
	if MySQLDBMSMiddleware == nil {
		return fmt.Errorf("MS database not initialized")
	}
	return HealthCheckDB(
		MySQLDBMSMiddleware,
		"mysql",
		config.GetConfig().MSMiddleware.MySQLDBName,
	)
}

// MonitorDBMSConnection monitors the MS database connection and logs disconnections
func MonitorDBMSConnection(interval time.Duration) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for range ticker.C {
		if err := HealthCheckDBMS(); err != nil {
			logrus.Errorf("MS database connection lost: %v", err)
			// Attempt to reconnect
			if reconnectErr := InitDBMS(); reconnectErr != nil {
				logrus.Errorf("Failed to reconnect to MS database: %v", reconnectErr)
			} else {
				logrus.Info("Reconnected to MS MySQL database successfully")
			}
		}
	}
}

// InitDBTA initializes the Technical Assistance MySQL database connection
func InitDBTA() error {
	cfg := config.GetConfig()

	dbCfgTA := DBConfig{
		Type:              "MySQL",
		User:              cfg.TechnicalAssistance.MySQLDBUser,
		Password:          cfg.TechnicalAssistance.MySQLDBPass,
		Host:              cfg.TechnicalAssistance.MySQLDBHost,
		Port:              cfg.TechnicalAssistance.MySQLDBPort,
		Database:          cfg.TechnicalAssistance.MySQLDBName,
		SSLMode:           cfg.TechnicalAssistance.MySQLDBSSLMode,
		MaxRetryConnect:   cfg.TechnicalAssistance.MySQLDBMaxRetryConnect,
		RetryDelay:        cfg.TechnicalAssistance.MySQLDBRetryDelay,
		MaxIdleConnection: cfg.TechnicalAssistance.MySQLDBIdleConnection,
		MaxOpenConnection: cfg.TechnicalAssistance.MySQLDBOpenConnection,
		ConnMaxLifeTime:   cfg.TechnicalAssistance.MySQLDBConnMaxLifetime,
	}

	db, err := InitDBConnection(
		dbCfgTA,
	)
	if err != nil {
		logrus.Errorf("Failed to initialize TA database: %v", err)
		return err
	}

	MySQLDBTA = db

	log.Println("✅ TA MySQL database initialized successfully")
	return nil
}

// GetDBTA returns the Technical Assistance database connection
func GetDBTA() *gorm.DB {
	if MySQLDBTA == nil {
		if err := InitDBTA(); err != nil {
			logrus.Fatalf("Failed to initialize TA database: %v", err)
		}
	}
	return MySQLDBTA
}

// CloseDBTA closes the Technical Assistance database connection
func CloseDBTA() error {
	if MySQLDBTA != nil {
		sqlDB, err := MySQLDBTA.DB()
		if err != nil {
			return fmt.Errorf("failed to get sql.DB from TA DB: %v", err)
		}
		if err := sqlDB.Close(); err != nil {
			return fmt.Errorf("failed to close TA database: %v", err)
		}
		log.Println("Disconnected from TA MySQL database")
	}
	return nil
}

// HealthCheckDBTA checks if the TA database connection is healthy
func HealthCheckDBTA() error {
	if MySQLDBTA == nil {
		return fmt.Errorf("TA database not initialized")
	}
	return HealthCheckDB(
		MySQLDBTA,
		"mysql",
		config.GetConfig().TechnicalAssistance.MySQLDBName,
	)
}

// MonitorDBTAConnection monitors the TA database connection and logs disconnections
func MonitorDBTAConnection(interval time.Duration) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for range ticker.C {
		if err := HealthCheckDBTA(); err != nil {
			logrus.Errorf("TA database connection lost: %v", err)
			// Attempt to reconnect
			if reconnectErr := InitDBTA(); reconnectErr != nil {
				logrus.Errorf("Failed to reconnect to TA database: %v", reconnectErr)
			} else {
				logrus.Info("Reconnected to TA MySQL database successfully")
			}
		}
	}
}
