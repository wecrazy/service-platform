package model

import "gorm.io/gorm"

// Optional constants to avoid magic strings
// const (
// 	LanguageID = "id"
// 	LanguageEN = "en"
// )

type BadWordCategory string

const (
	CategoryUmpatan BadWordCategory = "umpatan"
	CategoryRasis   BadWordCategory = "rasis"
	CategorySexual  BadWordCategory = "sexual"
	CategoryGeneral BadWordCategory = "general"
)

type BadWord struct {
	ID uint `gorm:"primaryKey" json:"id"`
	gorm.Model
	Word      string          `gorm:"type:varchar(100);uniqueIndex" json:"word"`
	Language  string          `gorm:"type:enum('id','en');not null;default:'en';index" json:"language"`
	Category  BadWordCategory `json:"category"`
	IsEnabled bool            `gorm:"default:true" json:"is_enabled"`
}
