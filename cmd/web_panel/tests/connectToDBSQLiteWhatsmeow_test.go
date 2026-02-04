package tests

import (
	"fmt"
	"service-platform/cmd/web_panel/controllers"
	"service-platform/internal/config"
	"testing"
)

// go test -v -timeout 60m ./tests/connectToDBSQLiteWhatsmeow_test.go
func TestConnectToDBSQLiteWhatsmeow(t *testing.T) {
	// Example LID JID (should match a record in your whatsmeow_lid_map table for a real test)
	config.LoadConfig()
	lidJIDStr := "127311672299713:18@lid"

	// Use the global DB connection if available, else connect
	db, err := controllers.WhatsmeowDBSQliteConnect()
	if err != nil {
		t.Fatalf("Failed to connect to Whatsmeow SQLite DB: %v", err)
	}

	// Debug: List all tables in the DB
	var tables []string
	err = db.Raw("SELECT name FROM sqlite_master WHERE type='table'").Scan(&tables).Error
	if err != nil {
		t.Fatalf("Failed to list tables: %v", err)
	}
	fmt.Printf("Tables in DB: %v\n", tables)

	phone, err := controllers.GetPhoneNumberFromLID(db, lidJIDStr)
	if err != nil {
		t.Fatalf("Error looking up phone number: %v", err)
	}
	fmt.Printf("LID JID: %s => Phone: %s\n", lidJIDStr, phone)
	// Optionally, assert expected phone number if known
	// expected := "YOUR_EXPECTED_PHONE_NUMBER"
	// if phone != expected {
	//     t.Errorf("Expected %s, got %s", expected, phone)
	// }
}
