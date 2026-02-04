package odooms

import (
	"service-platform/internal/config"

	"gorm.io/gorm"
)

type ODOOMSJobGroups struct {
	ID uint `gorm:"primaryKey;autoIncrement" json:"id"`
	gorm.Model

	Name             string `gorm:"column:name;type:varchar(255)" json:"name"`
	BasicSalary      int    `gorm:"column:basic_salary;type:int" json:"basic_salary"`
	TaskMax          int    `gorm:"column:task_max;type:int" json:"task_max"`
	InsentivePerTask int    `gorm:"column:incentive_per_task;type:int" json:"incentive_per_task"`
}

func (ODOOMSJobGroups) TableName() string {
	return config.WebPanel.Get().Database.TbODOOMSJobGroup
}
