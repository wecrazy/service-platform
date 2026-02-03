package tests

import (
	"fmt"
	"os"
	"service-platform/cmd/technical_assistance/config"
	"service-platform/cmd/technical_assistance/controllers"
	"service-platform/cmd/technical_assistance/database"
	"testing"

	"github.com/joho/godotenv"
	"github.com/stretchr/testify/assert"
)

func LoadEnvFromPaths(paths []string) (string, error) {
	for _, path := range paths {
		if _, err := os.Stat(path); err == nil {
			// If file exists, load it
			if loadErr := godotenv.Load(path); loadErr != nil {
				return "", fmt.Errorf("failed to load .env file at %s: %w", path, loadErr)
			}
			fmt.Printf("✅ Loaded .env from: %s", path)
			return path, nil
		}
	}
	return "", fmt.Errorf("❌ no .env file found in any of the specified paths")
}

func TestGenerateDailyReportTAActivity(t *testing.T) {
	// Load YAML config
	if err := config.LoadConfig(); err != nil {
		t.Fatalf("❌ Error loading .yaml config: %v", err)
	}

	// Load environment variables from possible .env paths
	if _, err := LoadEnvFromPaths([]string{
		"../.env",
		"/home/administrator/technical_assistance/.env",
	}); err != nil {
		t.Fatalf("❌ Failed to load .env file: %v", err)
	}

	// Initialize database connection
	db, err := database.InitAndCheckDB(
		os.Getenv("MYSQL_USER_DB_KONFIRMASI_PENGERJAAN"),
		os.Getenv("MYSQL_PASS_DB_KONFIRMASI_PENGERJAAN"),
		os.Getenv("MYSQL_HOST_DB_KONFIRMASI_PENGERJAAN"),
		os.Getenv("MYSQL_PORT_DB_KONFIRMASI_PENGERJAAN"),
		os.Getenv("MYSQL_NAME_DB_KONFIRMASI_PENGERJAAN"),
	)
	if err != nil {
		t.Fatalf("❌ Database setup failed: %v", err)
	}

	// Run report generation
	report, err := controllers.GenerateDailyReportTAActivity(db, nil)

	// Assertions
	assert.NoError(t, err, "❌ Report generation should not return an error")
	assert.True(t, report, "❌ Generated report should not be false")
}
