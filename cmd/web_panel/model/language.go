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
	return config.WebPanel.Get().Database.TbLanguage
}
