package database

import (
	"fmt"
	"time"

	"gorm.io/driver/mysql"
	"gorm.io/gorm"
)

// InitDB initializes and returns a database connection
func InitDB(dbURI string) (*gorm.DB, error) {
	db, err := gorm.Open(mysql.Open(dbURI), &gorm.Config{})
	if err != nil {
		return nil, err
	}

	return db, nil
}

func InitWebDB(dbURI string) (*gorm.DB, error) {
	db, err := gorm.Open(mysql.Open(dbURI), &gorm.Config{})
	if err != nil {
		return nil, err
	}

	return db, nil
}

func InitAndCheckDB(dbUser, dbPass, dbHost, dbPort, dbName string) (*gorm.DB, error) {
	// Connect to information_schema
	infoSchemaURI := fmt.Sprintf("%s:%s@tcp(%s:%s)/information_schema?charset=utf8&parseTime=True&loc=Local",
		dbUser,
		dbPass,
		dbHost,
		dbPort,
	)
	infoSchemaDB, err := gorm.Open(mysql.Open(infoSchemaURI), &gorm.Config{})
	if err != nil {
		return nil, fmt.Errorf("failed to connect to information_schema: %v", err)
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

	// Connect to the specified database
	dbURI := fmt.Sprintf("%s:%s@tcp(%s:%s)/%s?charset=utf8&parseTime=True&loc=Local",
		dbUser,
		dbPass,
		dbHost,
		dbPort,
		dbName,
	)
	db, err := gorm.Open(mysql.Open(dbURI), &gorm.Config{})
	if err != nil {
		return nil, fmt.Errorf("failed to connect to database: %v", err)
	}

	// Get the underlying sql.DB object
	sqlDB, err := db.DB()
	if err != nil {
		return nil, fmt.Errorf("failed to get db instance: %v", err)
	}

	// Set connection pool parameters
	sqlDB.SetMaxIdleConns(10)           // Set the maximum number of idle connections
	sqlDB.SetMaxOpenConns(100)          // Set the maximum number of open connections
	sqlDB.SetConnMaxLifetime(time.Hour) // Set the maximum lifetime of a connection

	return db, nil
}
