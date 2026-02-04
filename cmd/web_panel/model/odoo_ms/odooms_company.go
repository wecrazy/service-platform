package odooms

import (
	"service-platform/internal/config"

	"gorm.io/gorm"
)

type ODOOMSCompany struct {
	ID uint `gorm:"column:id;primaryKey;autoIncrement" json:"id"`
	gorm.Model
	Name string `gorm:"column:name" json:"name"`
}

func (ODOOMSCompany) TableName() string {
	return config.WebPanel.Get().Database.TbCompany
}
