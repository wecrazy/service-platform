package model

import (
	"service-platform/internal/config"

	"gorm.io/gorm"
)

type Language struct {
	gorm.Model
	ID   uint   `gorm:"column:id;primaryKey;autoIncrement" json:"id"`
	Name string `gorm:"type:text;column:name;default:NULL" json:"name"`
	Code string `gorm:"type:varchar(10);column:code;default:NULL" json:"code"`
	Flag string `gorm:"-" json:"flag"`
}

func (Language) TableName() string {
	return config.ServicePlatform.Get().Database.TbLanguage
}

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
	Language  string          `gorm:"type:varchar(2);not null;default:'id';index" json:"language"`
	Category  BadWordCategory `json:"category"`
	IsEnabled bool            `gorm:"default:true" json:"is_enabled"`
}

func (BadWord) TableName() string {
	return config.ServicePlatform.Get().Database.TbBadWord
}
