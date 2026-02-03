package model

import (
	"time"
)

// ParamVariable represents the `param_variables` table in the database.
type ParamVariable struct {
	ID           int       `gorm:"column:ID;primaryKey;autoIncrement" json:"ID" form:"ID"`
	SerialNumber string    `gorm:"column:SerialNumber" json:"SerialNumber" form:"SerialNumber"`
	PackageName  string    `gorm:"column:PackageName" json:"PackageName" form:"PackageName"`
	Key          string    `gorm:"column:Key" json:"Key" form:"Key"`
	Value        string    `gorm:"column:Value" json:"Value" form:"Value"`
	InsertDate   time.Time `gorm:"column:InsertDate" json:"InsertDate" form:"InsertDate"`
	CreateBy     int       `gorm:"column:CreateBy" json:"CreateBy" form:"CreateBy"`
	UpdateDate   time.Time `gorm:"column:UpdateDate" json:"UpdateDate" form:"UpdateDate"`
	UpdateBy     int       `gorm:"column:UpdateBy" json:"UpdateBy" form:"UpdateBy"`
}

// TableName sets the name of the table in the database.
func (ParamVariable) TableName() string {
	return "param_variables"
}
