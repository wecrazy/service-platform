package tests

import (
	"encoding/json"
	"service-platform/cmd/web_panel/database"
	"service-platform/cmd/web_panel/fun"
	"service-platform/cmd/web_panel/model"
	"service-platform/internal/config"
	"testing"

	"gorm.io/datatypes"
	"gorm.io/gorm"
)

// go test -v -timeout 10m ./tests/seedTaToBot_test.go
func TestSeedUserTAToBot(t *testing.T) {
	config.LoadConfig()

	userTA := config.WebPanel.Get().UserTA

	if len(userTA) == 0 {
		t.Error("UserTA map is empty")
		return
	}

	dbName := config.WebPanel.Get().Database.Name
	if dbName != "db_web_panel_gl" {
		t.Errorf("Unexpected database name: %s", dbName)
		return
	}

	dbWeb, err := database.InitAndCheckDB(
		config.WebPanel.Get().Database.Username,
		config.WebPanel.Get().Database.Password,
		config.WebPanel.Get().Database.Host,
		config.WebPanel.Get().Database.Port,
		dbName,
	)

	if err != nil {
		t.Errorf("Failed to connect to database: %v", err)
		return
	}

	allowedTypes := model.AllWAMessageTypes
	jsonBytes, err := json.Marshal(allowedTypes)
	if err != nil {
		t.Errorf("Failed to marshal allowed types: %v", err)
		return
	}

	excludedEmailToCheck := []string{
		"wegirandol@smartwebindonesia.com",
		"admin@swi.com",
		"testmfjr@gmail.com",
		"desta@smartwebindonesia.com",
		"abdu@csnams.com",
		"tetty@csnams.com",
		"callcenter@gmail.com",
		"sri_t@smartwebindonesia.com",
	}

	for emailTA, taData := range userTA {
		found := false
		for _, excluded := range excludedEmailToCheck {
			if emailTA == excluded {
				found = true
				break
			}
		}
		if found {
			continue
		}

		sanitizedPhone, err := fun.SanitizePhoneNumber(taData.Phone)
		if err != nil {
			t.Errorf("Failed to sanitize phone number %s: %v", taData.Phone, err)
			continue
		}
		sanitizedPhone = "62" + sanitizedPhone // Add country code for Indonesia

		var userBotWA model.WAPhoneUser
		result := dbWeb.Where("phone_number = ?", sanitizedPhone).First(&userBotWA)
		if result.Error != nil {
			if result.Error == gorm.ErrRecordNotFound {
				// User not found, create new
				newUser := model.WAPhoneUser{
					FullName:      taData.Name,
					Email:         emailTA,
					PhoneNumber:   sanitizedPhone,
					IsRegistered:  true,
					AllowedChats:  model.BothChat,
					AllowedTypes:  datatypes.JSON(jsonBytes),
					AllowedToCall: false,
					MaxDailyQuota: 250,
					Description:   "Technical Assistant " + config.WebPanel.Get().Default.PT,
					IsBanned:      false,
					UserType:      model.ODOOMSStaff,
					UserOf:        model.UserOfCSNA,
				}

				if err := dbWeb.Create(&newUser).Error; err != nil {
					t.Errorf("Failed to create user %s: %v", taData.Name, err)
				} else {
					t.Logf("Created new user: %s with phone %s", taData.Name, sanitizedPhone)
				}
			} else {
				t.Errorf("Database error when searching for phone %s: %v", sanitizedPhone, result.Error)
			}
		} else {
			t.Logf("User already exists: %s with phone %s", userBotWA.FullName, sanitizedPhone)
		}
	}
}
