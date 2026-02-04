package controllers

import (
	"encoding/json"
	"errors"
	"fmt"
	"service-platform/cmd/web_panel/fun"
	"service-platform/cmd/web_panel/internal/gormdb"
	"service-platform/cmd/web_panel/model"
	"service-platform/internal/config"
	"strings"
	"sync"

	"github.com/sirupsen/logrus"
	"gorm.io/datatypes"
	"gorm.io/gorm"
)

var (
	registODOOMSHeadAndSPLToChatBotMutex sync.Mutex
)

func RegistODOOMSHeadAndSPLToChatbot() error {
	taskDoing := "Register ODOO MS Head and SPL to Chatbot"
	if !registODOOMSHeadAndSPLToChatBotMutex.TryLock() {
		return fmt.Errorf("%s is still running, please wait a moment", taskDoing)
	}
	defer registODOOMSHeadAndSPLToChatBotMutex.Unlock()

	dbWeb := gormdb.Databases.Web
	ODOOMSSAC := config.WebPanel.Get().ODOOMSSAC

	allowedTypes := model.AllWAMessageTypes
	jsonBytes, err := json.Marshal(allowedTypes)
	if err != nil {
		return fmt.Errorf("failed to marshal allowed types: %v", err)
	}

	// Regist ODOO MS SAC
	for name, SAC := range ODOOMSSAC {
		var userChatBot model.WAPhoneUser
		res := dbWeb.
			Where("phone_number = ?", SAC.PhoneNumber).
			First(&userChatBot)
		if res.Error != nil {
			if errors.Is(res.Error, gorm.ErrRecordNotFound) {
				newUser := model.WAPhoneUser{
					FullName:      SAC.FullName,
					Email:         SAC.Email,
					PhoneNumber:   SAC.PhoneNumber,
					IsRegistered:  true,
					AllowedChats:  model.DirectChat,
					AllowedTypes:  datatypes.JSON(jsonBytes),
					AllowedToCall: true,
					Description:   "Head of SPL & Technician (SAC)",
					IsBanned:      false,
					UserType:      model.ODOOMSHead,
					MaxDailyQuota: 250,
					UserOf:        model.UserOfCSNA,
				}
				if err := dbWeb.Create(&newUser).Error; err != nil {
					return fmt.Errorf("failed to create new ODOO MS Head user %s: %v", name, err)
				}
				logrus.Infof("✅ Successfully registered new ODOO MS Head user %s to chatbot with phone number: %s", name, SAC.PhoneNumber)
			} else {
				return fmt.Errorf("failed to query ODOO MS Head user %s: %v", name, res.Error)
			}
		}

	}

	// Regist ODOO MS SPL
	GetDataTechnicianODOOMS()

	if len(TechODOOMSData) > 0 {
		for name, dataTech := range TechODOOMSData {
			if strings.Contains(name, "SPL") {
				if name == dataTech.SPL {
					splCity := getSPLCity(name)
					if splCity == "" {
						splCity = "Unknown"
					}

					sanitizedPhone, err := fun.SanitizePhoneNumber(dataTech.NoHP)
					if err != nil {
						logrus.Errorf("❌ Failed to sanitize phone number for ODOO MS SPL %s: %v", name, err)
						continue
					}

					var userChatBot model.WAPhoneUser
					res := dbWeb.
						Where("phone_number = ?", "62"+sanitizedPhone).
						First(&userChatBot)
					if res.Error != nil {
						if errors.Is(res.Error, gorm.ErrRecordNotFound) {
							newUser := model.WAPhoneUser{
								FullName:      fun.CapitalizeWord(dataTech.Name),
								Email:         dataTech.Email,
								PhoneNumber:   "62" + sanitizedPhone,
								IsRegistered:  true,
								AllowedChats:  model.DirectChat,
								AllowedTypes:  datatypes.JSON(jsonBytes),
								AllowedToCall: false,
								Description:   fmt.Sprintf("ODOO MS Service Point Leader (SPL) - %s", splCity),
								IsBanned:      false,
								UserType:      model.ODOOMSSPL,
								MaxDailyQuota: 250,
								UserOf:        model.UserOfCSNA,
							}
							if err := dbWeb.Create(&newUser).Error; err != nil {
								return fmt.Errorf("failed to create new ODOO MS SPL user %s: %v", name, err)
							}
							logrus.Infof("✅ Successfully registered new ODOO MS SPL user %s to chatbot with phone number: %s", name, dataTech.NoHP)
						} else {
							return fmt.Errorf("failed to query ODOO MS SPL user %s: %v", name, res.Error)
						}
					} else {
						logrus.Infof("WAPhoneUser for %s (%s) as ODOO MS SPL already exists, skipping", dataTech.Name, dataTech.NoHP)
					}
				}
			}
		}
	}

	return nil
}
