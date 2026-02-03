package odooms

import (
	"service-platform/cmd/web_panel/config"

	"gorm.io/gorm"
)

type ODOOMSFSParamPayment struct {
	ID uint `gorm:"primaryKey;autoIncrement" json:"id"`
	gorm.Model

	ParamType    string `gorm:"column:param_type;type:text" json:"param_type"`
	ParamKey     string `gorm:"column:param_key;type:text" json:"param_key"`
	ParamPrice   int    `gorm:"column:param_price;type:int" json:"param_price"`
	ParamCompany string `gorm:"column:param_company;type:text" json:"param_company"`
}

func (ODOOMSFSParamPayment) TableName() string {
	return config.GetConfig().Database.TbFSParamsPayment
}
