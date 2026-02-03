package model

import (
	"service-platform/cmd/web_panel/config"

	"gorm.io/gorm"
)

type TicketStage struct {
	gorm.Model
	ID   uint   `gorm:"column:id;primaryKey;autoIncrement" json:"id"`
	Name string `gorm:"type:text;column:name;not null" json:"name"`
}

func (TicketStage) TableName() string {
	return config.GetConfig().Database.TbTicketStage
}
