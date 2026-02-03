package controllers

import (
	"encoding/json"
	"fmt"
	"service-platform/cmd/web_panel/fun"
	"service-platform/cmd/web_panel/internal/gormdb"
	"service-platform/cmd/web_panel/model"
	"sync"

	"github.com/sirupsen/logrus"
	"gorm.io/datatypes"
)

var (
	registODOOMSTechnicianToChatBotMutex sync.Mutex
)

func RegistODOOMSTechnicianToChatbot() error {
	taskDoing := "Register ODOO MS Technician to Chatbot"
	if !registODOOMSTechnicianToChatBotMutex.TryLock() {
		return fmt.Errorf("%s is still running, please wait a moment", taskDoing)
	}
	defer registODOOMSTechnicianToChatBotMutex.Unlock()

	allowedTypes := model.AllWAMessageTypes
	jsonBytes, err := json.Marshal(allowedTypes)
	if err != nil {
		return fmt.Errorf("failed to marshal allowed types: %v", err)
	}

	dbWeb := gormdb.Databases.Web

	GetDataTechnicianODOOMS()

	if len(TechODOOMSData) > 0 {
		for _, data := range TechODOOMSData {
			sanitizedPhone, err := fun.SanitizePhoneNumber(data.NoHP)
			if err != nil {
				logrus.Errorf("failed to sanitize phone number %s of %s: %v", data.NoHP, data.Name, err)
				continue
			}

			var userChatBot model.WAPhoneUser
			res := dbWeb.
				Where("phone_number = ?", "62"+sanitizedPhone).
				First(&userChatBot)
			if res.Error != nil {
				if res.Error != nil && res.RowsAffected == 0 {
					newUser := model.WAPhoneUser{
						FullName:      fun.CapitalizeWord(data.Name),
						Email:         data.Email,
						PhoneNumber:   "62" + sanitizedPhone,
						IsRegistered:  true,
						AllowedChats:  model.DirectChat,
						AllowedTypes:  datatypes.JSON(jsonBytes),
						AllowedToCall: false,
						Description:   "ODOO MS Technician",
						IsBanned:      false,
						UserType:      model.ODOOMSTechnician,
						MaxDailyQuota: 250,
						UserOf:        model.UserOfCSNA,
					}
					if err := dbWeb.Create(&newUser).Error; err != nil {
						logrus.Errorf("failed to create WAPhoneUser for %s (%s): %v", data.Name, data.NoHP, err)
					} else {
						logrus.Infof("successfully created WAPhoneUser for %s (%s)", data.Name, data.NoHP)
					}
				} else {
					logrus.Errorf("failed to query WAPhoneUser for %s (%s): %v", data.Name, data.NoHP, res.Error)
				}
			} else {
				// logrus.Infof("WAPhoneUser for %s (%s) as ODOO MS Technician already exists, skipping", data.Name, data.NoHP)
			}
		}
	}

	return nil
}
