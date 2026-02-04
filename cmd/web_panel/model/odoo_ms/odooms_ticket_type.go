package odooms

import (
	"service-platform/internal/config"

	"gorm.io/datatypes"
	"gorm.io/gorm"
)

type ODOOMSTicketType struct {
	ID uint `gorm:"column:id;primaryKey;autoIncrement" json:"id"`
	gorm.Model

	Sequence                 int            `gorm:"column:sequence" json:"sequence"`
	Type                     string         `gorm:"column:type" json:"type"`
	TaskType                 string         `gorm:"column:task_type" json:"task_type"`
	WorksheetTemplate        string         `gorm:"column:worksheet_template" json:"worksheet_template"`
	WorksheetTemplateId      int            `gorm:"column:worksheet_template_id" json:"worksheet_template_id"`
	DirectlyPlanIntervantion bool           `gorm:"column:directly_plan_intervention" json:"directly_plan_intervention"`
	MultiCompany             datatypes.JSON `gorm:"column:multi_company" json:"multi_company"`
}

func (ODOOMSTicketType) TableName() string {
	return config.WebPanel.Get().Database.TbTicketTypeODOOMS
}
