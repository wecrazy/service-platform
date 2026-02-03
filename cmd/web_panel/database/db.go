package database

import (
	"fmt"
	"service-platform/cmd/web_panel/config"
	"time"

	"gorm.io/driver/mysql"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func InitAndCheckDB(dbUser, dbPass, dbHost, dbPort, dbName string) (*gorm.DB, error) {
	infoSchemaURI := fmt.Sprintf("%s:%s@tcp(%s:%s)/information_schema?charset=utf8&parseTime=True&loc=Local",
		dbUser,
		dbPass,
		dbHost,
		dbPort,
	)

	var infoSchemaDB *gorm.DB
	var err error

	config.LoadConfig()
	maxRetries := config.GetConfig().Database.MaxRetryConnect
	retryDelay := config.GetConfig().Database.RetryDelay

	// Retry connection up to maxRetries with retryDelay seconds
	for attempt := 1; attempt <= maxRetries; attempt++ {
		infoSchemaDB, err = gorm.Open(mysql.Open(infoSchemaURI), &gorm.Config{
			Logger: logger.Default.LogMode(logger.Silent),
		})
		if err == nil {
			break
		}
		fmt.Printf("Attempt %d: failed to connect to information_schema: %v\n", attempt, err)
		time.Sleep(time.Duration(retryDelay) * time.Second)
	}

	// ✅ Check for failure after all attempts
	if err != nil || infoSchemaDB == nil {
		return nil, fmt.Errorf("failed to connect to information_schema after %d attempts: %v", maxRetries, err)
	}

	// Check if the database exists
	var dbExists bool
	query := fmt.Sprintf("SELECT EXISTS(SELECT SCHEMA_NAME FROM SCHEMATA WHERE SCHEMA_NAME = '%s')", dbName)
	err = infoSchemaDB.Raw(query).Scan(&dbExists).Error
	if err != nil {
		return nil, fmt.Errorf("failed to check if database exists: %v", err)
	}

	// Create the database if it does not exist
	if !dbExists {
		createDBQuery := fmt.Sprintf("CREATE DATABASE %s", dbName)
		err = infoSchemaDB.Exec(createDBQuery).Error
		if err != nil {
			return nil, fmt.Errorf("failed to create database: %v", err)
		}
		fmt.Printf("Database %s created successfully\n", dbName)
	}

	// Close the connection to information_schema
	dbSQL, err := infoSchemaDB.DB()
	if err != nil {
		return nil, fmt.Errorf("failed to get database instance: %v", err)
	}
	dbSQL.Close()

	// Connect to the target database
	dbURI := fmt.Sprintf(
		"%s:%s@tcp(%s:%s)/%s?charset=utf8mb4&collation=utf8mb4_general_ci&parseTime=True&loc=Local",
		dbUser,
		dbPass,
		dbHost,
		dbPort,
		dbName,
	)
	db, err := gorm.Open(mysql.Open(dbURI), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	if err != nil {
		return nil, fmt.Errorf("failed to connect to database: %v", err)
	}

	sqlDB, err := db.DB()
	if err != nil {
		return nil, fmt.Errorf("failed to get db instance: %v", err)
	}

	sqlDB.SetMaxIdleConns(config.GetConfig().Database.MaxIdleConns)
	sqlDB.SetMaxOpenConns(config.GetConfig().Database.MaxOpenConns)
	sqlDB.SetConnMaxLifetime(time.Duration(config.GetConfig().Database.MaxLifetimeConns) * time.Hour)

	fmt.Printf("✅ Connected to database: %s\n", dbName)

	return db, nil
}

// TryConnectDB tries to connect to the database only if it exists, otherwise returns error.
func TryConnectDB(dbUser, dbPass, dbHost, dbPort, dbName string) (*gorm.DB, error) {
	infoSchemaURI := fmt.Sprintf("%s:%s@tcp(%s:%s)/information_schema?charset=utf8&parseTime=True&loc=Local",
		dbUser,
		dbPass,
		dbHost,
		dbPort,
	)

	var infoSchemaDB *gorm.DB
	var err error

	config.LoadConfig()
	maxRetries := config.GetConfig().Database.MaxRetryConnect
	retryDelay := config.GetConfig().Database.RetryDelay

	for attempt := 1; attempt <= maxRetries; attempt++ {
		infoSchemaDB, err = gorm.Open(mysql.Open(infoSchemaURI), &gorm.Config{
			Logger: logger.Default.LogMode(logger.Silent),
		})
		if err == nil {
			break
		}
		fmt.Printf("Attempt %d: failed to connect to information_schema: %v\n", attempt, err)
		time.Sleep(time.Duration(retryDelay) * time.Second)
	}

	if err != nil || infoSchemaDB == nil {
		return nil, fmt.Errorf("failed to connect to information_schema after %d attempts: %v", maxRetries, err)
	}

	var dbExists bool
	query := fmt.Sprintf("SELECT EXISTS(SELECT SCHEMA_NAME FROM SCHEMATA WHERE SCHEMA_NAME = '%s')", dbName)
	err = infoSchemaDB.Raw(query).Scan(&dbExists).Error
	if err != nil {
		return nil, fmt.Errorf("failed to check if database exists: %v", err)
	}

	dbSQL, err := infoSchemaDB.DB()
	if err == nil {
		dbSQL.Close()
	}

	if !dbExists {
		return nil, fmt.Errorf("database %s does not exist", dbName)
	}

	dbURI := fmt.Sprintf(
		"%s:%s@tcp(%s:%s)/%s?charset=utf8mb4&collation=utf8mb4_general_ci&parseTime=True&loc=Local",
		dbUser,
		dbPass,
		dbHost,
		dbPort,
		dbName,
	)
	db, err := gorm.Open(mysql.Open(dbURI), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	if err != nil {
		return nil, fmt.Errorf("failed to connect to database: %v", err)
	}

	sqlDB, err := db.DB()
	if err != nil {
		return nil, fmt.Errorf("failed to get db instance: %v", err)
	}

	sqlDB.SetMaxIdleConns(config.GetConfig().Database.MaxIdleConns)
	sqlDB.SetMaxOpenConns(config.GetConfig().Database.MaxOpenConns)
	sqlDB.SetConnMaxLifetime(time.Duration(config.GetConfig().Database.MaxLifetimeConns) * time.Hour)

	fmt.Printf("✅ Connected to database: %s\n", dbName)

	return db, nil
}
