package model

import (
	"time"
)

type File struct {
	ID          int       `json:"ID" form:"ID" gorm:"column:ID;primaryKey;autoIncrement"`
	AppID       int       `json:"AppID" form:"AppID" gorm:"column:AppID;not null"`
	Filename    string    `json:"Filename" form:"Filename" gorm:"column:Filename;size:100;not null"`
	VersionCode int       `json:"VersionCode" form:"VersionCode" gorm:"column:VersionCode;default:0"`
	VersionName string    `json:"VersionName" form:"VersionName" gorm:"column:VersionName;size:100;not null"`
	Size        string    `json:"Size" form:"Size" gorm:"column:Size;size:50"`
	Path        string    `json:"Path" form:"Path" gorm:"column:Path;size:255;not null"`
	Status      int       `json:"Status" form:"Status" gorm:"column:Status;default:1;not null"`
	InsertDate  time.Time `json:"InsertDate" form:"InsertDate" gorm:"column:InsertDate;not null"`
	CreateBy    int       `json:"CreateBy" form:"CreateBy" gorm:"column:CreateBy;not null"`
	DescApp     string    `json:"DescApp" form:"DescApp" gorm:"column:DescApp;type:longtext"`
}

// TableName overrides the default table name
func (File) TableName() string {
	return "files"
}
