package model

import (
	"service-platform/internal/config"

	"gorm.io/gorm"
)

type Feature struct {
	ID uint `json:"id" gorm:"column:id;primary_key;autoincrement"`
	gorm.Model

	ParentID  uint   `json:"parent_id" gorm:"column:parent_id"`
	Title     string `json:"title" gorm:"column:title"`
	Path      string `json:"path" gorm:"column:path"`
	MenuOrder uint   `json:"menu_order" gorm:"column:menu_order"`
	Status    uint   `json:"status" gorm:"column:status"`
	Level     uint   `json:"level" gorm:"column:level"`
	Icon      string `json:"icon" gorm:"column:icon"`
}

func (Feature) TableName() string {
	return config.ServicePlatform.Get().Database.TbFeature
}
