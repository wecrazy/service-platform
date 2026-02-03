package odooms

import (
	"service-platform/cmd/web_panel/config"

	"gorm.io/gorm"
)

type ODOOMSFSParams struct {
	ID uint `gorm:"primaryKey;autoIncrement" json:"id"`
	gorm.Model

	ParamKey   string `gorm:"column:param_key;type:text" json:"param_key"`
	ParamValue string `gorm:"column:param_value;type:text" json:"param_value"`
	Logs       string `gorm:"column:logs;type:text" json:"logs"`
}

func (ODOOMSFSParams) TableName() string {
	return config.GetConfig().Database.TbFSParams
}
