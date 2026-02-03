package model

import (
	"time"
)

type Application struct {
	ID              int       `json:"ID" form:"ID" gorm:"column:ID;primaryKey;autoIncrement"`
	ApplicationName string    `json:"ApplicationName" form:"ApplicationName" gorm:"column:ApplicationName;size:100;default:''"`
	PackageName     string    `json:"PackageName" form:"PackageName" gorm:"column:PackageName;size:100;default:''"`
	Manufacture     string    `json:"Manufacture" form:"Manufacture" gorm:"column:Manufacture;type:text;default:null"`
	EDCModel        string    `json:"EDCModel" form:"EDCModel" gorm:"column:EDCModel;type:text;default:null"`
	Icon            string    `json:"Icon" form:"Icon" gorm:"column:Icon;type:longtext;default:null"`
	Type            int       `json:"Type" form:"Type" gorm:"column:Type;default:null"`
	Download        int       `json:"Download" form:"Download" gorm:"column:Download;default:0"`
	InsertDate      time.Time `json:"InsertDate" form:"InsertDate" gorm:"column:InsertDate"`
	CreateBy        int       `json:"CreateBy" form:"CreateBy" gorm:"column:CreateBy;default:null"`
	UpdateDate      time.Time `json:"UpdateDate" form:"UpdateDate" gorm:"column:UpdateDate"`
}

// TableName overrides the default table name
func (Application) TableName() string {
	return "applications"
}
