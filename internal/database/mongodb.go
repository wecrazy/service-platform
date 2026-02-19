package database

import (
	"context"
	"fmt"
	"log"
	"time"

	"service-platform/internal/config"

	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

// MongoDBClient holds the MongoDB client
var MongoDBClient *mongo.Client

// BuildMongoURI builds MongoDB connection URI from config
func BuildMongoURI() string {
	cfg := config.ServicePlatform.Get()
	uri := fmt.Sprintf("mongodb://%s:%d/%s", cfg.MongoDB.Host, cfg.MongoDB.Port, cfg.MongoDB.Database)

	if cfg.MongoDB.Username != "" && cfg.MongoDB.Password != "" {
		uri = fmt.Sprintf("mongodb://%s:%s@%s:%d/%s",
			cfg.MongoDB.Username,
			cfg.MongoDB.Password,
			cfg.MongoDB.Host,
			cfg.MongoDB.Port,
			cfg.MongoDB.Database)
		if cfg.MongoDB.AuthSource != "" {
			uri += "?authSource=" + cfg.MongoDB.AuthSource
		}
	}

	return uri
}

// InitMongoDB initializes and connects to MongoDB
func InitMongoDB() error {
	uri := BuildMongoURI()

	clientOptions := options.Client().ApplyURI(uri)

	// Set connection pool options
	if cfg := config.ServicePlatform.Get(); cfg.MongoDB.MaxPoolSize > 0 {
		clientOptions.SetMaxPoolSize(cfg.MongoDB.MaxPoolSize)
	}
	if cfg := config.ServicePlatform.Get(); cfg.MongoDB.MinPoolSize > 0 {
		clientOptions.SetMinPoolSize(cfg.MongoDB.MinPoolSize)
	}
	if cfg := config.ServicePlatform.Get(); cfg.MongoDB.MaxIdleTime > 0 {
		clientOptions.SetMaxConnIdleTime(time.Duration(cfg.MongoDB.MaxIdleTime) * time.Second)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	client, err := mongo.Connect(ctx, clientOptions)
	if err != nil {
		log.Printf("Failed to connect to MongoDB: %v", err)
		return err
	}

	// Ping the database
	err = client.Ping(ctx, nil)
	if err != nil {
		log.Printf("Failed to ping MongoDB: %v", err)
		return err
	}

	MongoDBClient = client
	log.Println("Connected to MongoDB successfully")
	return nil
}

// GetMongoDBClient returns the MongoDB client
func GetMongoDBClient() *mongo.Client {
	return MongoDBClient
}

// CloseMongoDB closes the MongoDB connection
func CloseMongoDB() error {
	if MongoDBClient != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		err := MongoDBClient.Disconnect(ctx)
		if err != nil {
			log.Printf("Error disconnecting from MongoDB: %v", err)
			return err
		}
		log.Println("Disconnected from MongoDB")
		return nil
	}
	return nil
}

// HealthCheckMongoDB checks if MongoDB connection is healthy
func HealthCheckMongoDB() error {
	if MongoDBClient == nil {
		return fmt.Errorf("MongoDB client is not initialized")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	err := MongoDBClient.Ping(ctx, nil)
	if err != nil {
		log.Printf("MongoDB health check failed: %v", err)
		return err
	}

	return nil
}

// MonitorMongoDBConnection monitors the MongoDB connection and logs disconnections
func MonitorMongoDBConnection(interval time.Duration) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for range ticker.C {
		if err := HealthCheckMongoDB(); err != nil {
			log.Printf("MongoDB connection lost: %v", err)
			// Attempt to reconnect
			if reconnectErr := InitMongoDB(); reconnectErr != nil {
				log.Printf("Failed to reconnect to MongoDB: %v", reconnectErr)
			} else {
				log.Println("Reconnected to MongoDB successfully")
			}
		}
	}
}
