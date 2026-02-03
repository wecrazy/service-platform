package odooms

import (
	"service-platform/cmd/web_panel/config"

	"gorm.io/gorm"
)

type ODOOMSTicketField struct {
	ID uint `gorm:"primaryKey;autoIncrement" json:"id"`
	gorm.Model
	Name        string `gorm:"column:name;type:text;not null" json:"name"`
	Description string `gorm:"column:description;type:text;not null" json:"description"`
	Type        string `gorm:"column:type;type:text" json:"type"`
}

func (ODOOMSTicketField) TableName() string {
	return config.GetConfig().Database.TbODOOMSTicketField
}
