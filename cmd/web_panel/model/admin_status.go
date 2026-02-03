package model

import "service-platform/cmd/web_panel/config"

type AdminStatus struct {
	ID        uint   `json:"id" gorm:"column:id;primarykey"`
	Title     string `json:"title" gorm:"column:title"`
	ClassName string `json:"class_name" gorm:"column:class_name"`
}

func (AdminStatus) TableName() string {
	return config.GetConfig().Database.TbAdminStatus
}
