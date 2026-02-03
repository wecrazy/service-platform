package controllers

import (
	"fmt"
	"net/http"
	"service-platform/cmd/web_panel/config"
	whatsappmodel "service-platform/cmd/web_panel/model/whatsapp_model"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

// CheckDatabaseTables checks if the WhatsApp conversation tables exist and their structure
func CheckDatabaseTables(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		result := gin.H{}

		// Check if tables exist by trying to query them
		tables := map[string]interface{}{
			config.GetConfig().Whatsmeow.WhatsappModel.TBConversation:          &whatsappmodel.WAConversation{},
			config.GetConfig().Whatsmeow.WhatsappModel.TBChatMessage:           &whatsappmodel.WAChatMessage{},
			config.GetConfig().Whatsmeow.WhatsappModel.TBContactInfo:           &whatsappmodel.WAContactInfo{},
			config.GetConfig().Whatsmeow.WhatsappModel.TBGroupParticipant:      &whatsappmodel.WAGroupParticipant{},
			config.GetConfig().Whatsmeow.WhatsappModel.TBMessageDeliveryStatus: &whatsappmodel.WAMessageDeliveryStatus{},
			config.GetConfig().Whatsmeow.WhatsappModel.TBMediaFile:             &whatsappmodel.WAMediaFile{},
		}

		for tableName, model := range tables {
			// Check if table exists
			if db.Migrator().HasTable(model) {
				result[tableName] = "exists"

				// Get table info
				var count int64
				db.Model(model).Count(&count)
				result[tableName+"_count"] = count
			} else {
				result[tableName] = "not_exists"
			}
		}

		// Try to get raw table info
		var tableList []string
		tbPluck := fmt.Sprintf("Tables_in_%s", config.GetConfig().Database.Name)
		db.Raw("SHOW TABLES LIKE 'wa_%'").Pluck(tbPluck, &tableList)
		result["raw_tables"] = tableList

		c.JSON(http.StatusOK, result)
	}
}

// ForceMigrateWhatsappTables forces migration of WhatsApp tables
func ForceMigrateWhatsappTables(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		// Drop existing tables if they exist (be careful with this!)
		dropTables := c.Query("drop") == "true"

		if dropTables {
			db.Migrator().DropTable(&whatsappmodel.WAMessageDeliveryStatus{})
			db.Migrator().DropTable(&whatsappmodel.WAMediaFile{})
			db.Migrator().DropTable(&whatsappmodel.WAGroupParticipant{})
			db.Migrator().DropTable(&whatsappmodel.WAChatMessage{})
			db.Migrator().DropTable(&whatsappmodel.WAConversation{})
			db.Migrator().DropTable(&whatsappmodel.WAContactInfo{})
		}

		// Recreate tables in order
		errors := []string{}

		// Stage 1: Independent tables
		if err := db.Migrator().CreateTable(&whatsappmodel.WAContactInfo{}); err != nil {
			errors = append(errors, "WAContactInfo: "+err.Error())
		}

		if err := db.Migrator().CreateTable(&whatsappmodel.WAConversation{}); err != nil {
			errors = append(errors, "WAConversation: "+err.Error())
		}

		// Stage 2: Dependent tables
		if err := db.Migrator().CreateTable(&whatsappmodel.WAChatMessage{}); err != nil {
			errors = append(errors, "WAChatMessage: "+err.Error())
		}

		if err := db.Migrator().CreateTable(&whatsappmodel.WAGroupParticipant{}); err != nil {
			errors = append(errors, "WAGroupParticipant: "+err.Error())
		}

		// Stage 3: Most dependent tables
		if err := db.Migrator().CreateTable(&whatsappmodel.WAMediaFile{}); err != nil {
			errors = append(errors, "WAMediaFile: "+err.Error())
		}

		if err := db.Migrator().CreateTable(&whatsappmodel.WAMessageDeliveryStatus{}); err != nil {
			errors = append(errors, "WAMessageDeliveryStatus: "+err.Error())
		}

		if len(errors) > 0 {
			c.JSON(http.StatusInternalServerError, gin.H{
				"success": false,
				"errors":  errors,
			})
		} else {
			c.JSON(http.StatusOK, gin.H{
				"success": true,
				"message": "WhatsApp tables migrated successfully",
			})
		}
	}
}
