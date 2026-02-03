package controllers

import (
	"path/filepath"
	"service-platform/cmd/web_panel/config"
	"service-platform/cmd/web_panel/fun"
	"strings"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

var WhatsmeowDB *gorm.DB

type WhatsmeowLIDMap struct {
	LID         string `gorm:"column:lid"`
	PhoneNumber string `gorm:"column:pn"`
}

func (WhatsmeowLIDMap) TableName() string {
	return config.GetConfig().Whatsmeow.DBSQLiteModel.TBLIDMap
}

func WhatsmeowDBSQliteConnect() (*gorm.DB, error) {
	sqlSource := config.GetConfig().Whatsmeow.SqlSource
	sqlSourceParts := strings.Split(sqlSource, "/")

	// fmt.Println("WhatsmeowDBSQliteConnect: sqlSource =", sqlSource)

	dbMainDir, err := fun.FindValidDirectory([]string{
		sqlSourceParts[0],
		"../" + sqlSourceParts[0],
		"../../" + sqlSourceParts[0],
	})
	if err != nil {
		return nil, err
	}
	dbPath := filepath.Join(dbMainDir, sqlSourceParts[1])

	db, err := gorm.Open(sqlite.Open(dbPath), &gorm.Config{})
	if err != nil {
		return nil, err
	}
	return db, nil
}

func GetPhoneNumberFromLID(db *gorm.DB, jid string) (string, error) {
	// e.g. jid = 127311672299713@s.whatsapp.net | 127311672299713:18@s.whatsapp.net | 127311672299713@lid
	// Extract the part before '@' (and before ':')
	lid := jid
	// Remove suffix after '@'
	if idx := strings.Index(lid, "@"); idx != -1 {
		lid = lid[:idx]
	}
	// Remove device ID if present (colon and after)
	if idx := strings.Index(lid, ":"); idx != -1 {
		lid = lid[:idx]
	}

	var rec WhatsmeowLIDMap
	if err := db.
		Where("lid = ?", lid).
		First(&rec).Error; err != nil {
		return "", err
	}
	return rec.PhoneNumber, nil
}
