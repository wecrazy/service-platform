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

	// TODO: faiz
	// pakai pakai koneksi db web panel ini untuk langsung query ke db WebPanel.service
	MySQLDBWebPanel *gorm.DB // MySQLDBWebPanel holds the GORM DB connection for Reporting, Dashboard, Other Function in MySQL Database
)

// InitDBMS initializes the Manage Service MySQL database connection
func InitDBMS() error {
	cfg := config.ManageService.Get()

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
		config.ManageService.Get().MSMiddleware.MySQLDBName,
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
	cfg := config.ManageService.Get()

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
		config.ManageService.Get().TechnicalAssistance.MySQLDBName,
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

// InitDBWebPanel initializes the WebPanel MySQL database connection
func InitDBWebPanel() error {
	cfg := config.ManageService.Get()

	dbCfgWebPanel := DBConfig{
		Type:              "MySQL",
		User:              cfg.WebPanelService.MySQLDBUser,
		Password:          cfg.WebPanelService.MySQLDBPass,
		Host:              cfg.WebPanelService.MySQLDBHost,
		Port:              cfg.WebPanelService.MySQLDBPort,
		Database:          cfg.WebPanelService.MySQLDBName,
		SSLMode:           cfg.WebPanelService.MySQLDBSSLMode,
		MaxRetryConnect:   cfg.WebPanelService.MySQLDBMaxRetryConnect,
		RetryDelay:        cfg.WebPanelService.MySQLDBRetryDelay,
		MaxIdleConnection: cfg.WebPanelService.MySQLDBIdleConnection,
		MaxOpenConnection: cfg.WebPanelService.MySQLDBOpenConnection,
		ConnMaxLifeTime:   cfg.WebPanelService.MySQLDBConnMaxLifetime,
	}

	db, err := InitDBConnection(dbCfgWebPanel)
	if err != nil {
		logrus.Errorf("Failed to initialize WebPanel database: %v", err)
		return err
	}

	MySQLDBWebPanel = db

	log.Println("✅ WebPanel MySQL database initialized successfully")
	return nil
}

// GetDBWebPanel returns the WebPanel database connection
func GetDBWebPanel() *gorm.DB {
	if MySQLDBWebPanel == nil {
		if err := InitDBWebPanel(); err != nil {
			logrus.Fatalf("Failed to initialize WebPanel database: %v", err)
		}
	}
	return MySQLDBWebPanel
}

// CloseDBWebPanel closes the WebPanel database connection
func CloseDBWebPanel() error {
	if MySQLDBWebPanel != nil {
		sqlDB, err := MySQLDBWebPanel.DB()
		if err != nil {
			return fmt.Errorf("failed to get sql.DB from WebPanel DB: %v", err)
		}
		if err := sqlDB.Close(); err != nil {
			return fmt.Errorf("failed to close WebPanel database: %v", err)
		}
		log.Println("Disconnected from WebPanel MySQL database")
	}
	return nil
}

// HealthCheckDBWebPanel checks if the WebPanel database connection is healthy
func HealthCheckDBWebPanel() error {
	if MySQLDBWebPanel == nil {
		return fmt.Errorf("WebPanel database not initialized")
	}
	return HealthCheckDB(
		MySQLDBWebPanel,
		"mysql",
		config.ManageService.Get().WebPanelService.MySQLDBName,
	)
}

// MonitorDBWebPanelConnection monitors the WebPanel database connection and logs disconnections
func MonitorDBWebPanelConnection(interval time.Duration) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for range ticker.C {
		if err := HealthCheckDBWebPanel(); err != nil {
			logrus.Errorf("WebPanel database connection lost: %v", err)
			// Attempt to reconnect
			if reconnectErr := InitDBWebPanel(); reconnectErr != nil {
				logrus.Errorf("Failed to reconnect to WebPanel database: %v", reconnectErr)
			} else {
				logrus.Info("Reconnected to WebPanel MySQL database successfully")
			}
		}
	}
}
