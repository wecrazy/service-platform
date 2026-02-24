package model

import (
	"service-platform/internal/config"

	"gorm.io/gorm"
)

// Language represents a language supported by the application, with fields for the name, code, and an optional flag. The TableName method specifies the database table name for this model, which is defined in the configuration file under Database.TbLanguage.
type Language struct {
	gorm.Model
	ID   uint   `gorm:"column:id;primaryKey;autoIncrement" json:"id"`
	Name string `gorm:"type:text;column:name;default:NULL" json:"name"`
	Code string `gorm:"type:varchar(10);column:code;default:NULL" json:"code"`
	Flag string `gorm:"-" json:"flag"`
}

// TableName specifies the database table name for the Language model, which is defined in the configuration file under Database.TbLanguage.
func (Language) TableName() string {
	return config.ServicePlatform.Get().Database.TbLanguage
}

// BadWordCategory defines a category of prohibited words for content moderation.
type BadWordCategory string

// Define constants for the bad word categories to ensure consistency and avoid typos when assigning categories to bad words in the application. These categories can be used to classify bad words based on their nature, such as offensive language, racial slurs, sexual content, or general profanity.
const (
	CategoryUmpatan BadWordCategory = "umpatan"
	CategoryRasis   BadWordCategory = "rasis"
	CategorySexual  BadWordCategory = "sexual"
	CategoryGeneral BadWordCategory = "general"
)

// BadWord represents a bad word or phrase that is considered inappropriate or offensive in the application. It includes fields for the word itself, the language it belongs to, its category, and whether it is currently enabled for filtering. The TableName method specifies the database table name for this model, which is defined in the configuration file under Database.TbBadWord.
type BadWord struct {
	ID uint `gorm:"primaryKey" json:"id"`
	gorm.Model
	Word      string          `gorm:"type:varchar(100);uniqueIndex" json:"word"`
	Language  string          `gorm:"type:varchar(2);not null;default:'id';index" json:"language"`
	Category  BadWordCategory `json:"category"`
	IsEnabled bool            `gorm:"default:true" json:"is_enabled"`
}

// TableName specifies the database table name for the BadWord model, which is defined in the configuration file under Database.TbBadWord.
func (BadWord) TableName() string {
	return config.ServicePlatform.Get().Database.TbBadWord
}
