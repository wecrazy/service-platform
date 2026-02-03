package model

import (
	"time"
)

type Edc struct {
	ID             int       `json:"ID" form:"ID" gorm:"column:ID;primaryKey;autoIncrement"`
	SerialNumber   string    `json:"SerialNumber" form:"SerialNumber" gorm:"column:SerialNumber;unique;size:50;not null;default:''"`
	TerminalName   string    `json:"TerminalName" form:"TerminalName" gorm:"column:TerminalName;size:500;default:null"`
	Manufacture    string    `json:"Manufacture" form:"Manufacture" gorm:"column:Manufacture;size:100;default:''"`
	Model          string    `json:"Model" form:"Model" gorm:"column:Model;size:50;default:''"`
	Reseller       string    `json:"Reseller" form:"Reseller" gorm:"column:Reseller;size:100;default:''"`
	DateInsert     time.Time `json:"DateInsert" form:"DateInsert" gorm:"column:DateInsert;autoCreateTime"`
	Status         int       `json:"Status" form:"Status" gorm:"column:Status"`
	LastOnline     time.Time `json:"LastOnline" form:"LastOnline" gorm:"column:LastOnline"`
	Settings       string    `json:"Settings" form:"Settings" gorm:"column:Settings;type:longtext"`
	UpdateDate     time.Time `json:"UpdateDate" form:"UpdateDate" gorm:"column:UpdateDate"`
	Remark         string    `json:"Remark" form:"Remark" gorm:"column:Remark;type:mediumtext"`
	TopicBroadcast string    `json:"TopicBroadcast" form:"TopicBroadcast" gorm:"column:TopicBroadcast;size:225"`
	TopicListen    string    `json:"TopicListen" form:"TopicListen" gorm:"column:TopicListen;size:225"`
}

// TableName overrides the default table name
func (Edc) TableName() string {
	return "edc"
}
