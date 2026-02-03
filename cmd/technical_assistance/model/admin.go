package model

import (
	"time"

	"gorm.io/gorm"
)

type Admin struct {
	ID        uint           `json:"ID" form:"ID" gorm:"column:ID;primaryKey;autoIncrement"`
	Fullname  string         `json:"Fullname" form:"Fullname" gorm:"column:Fullname;size:50"`
	Username  string         `json:"Username" form:"Username" gorm:"column:Username;size:50"`
	Phone     string         `json:"Phone" form:"Phone" gorm:"column:Phone;size:20"`
	Email     string         `json:"Email" form:"Email" gorm:"column:Email;size:50"`
	Password  string         `json:"Password" form:"Password" gorm:"column:Password;size:100"`
	Role      int            `json:"Role" form:"Role" gorm:"column:Role;default:0"`
	Status    int            `json:"Status" form:"Status" gorm:"column:Status;default:0"`
	UpdatedAt time.Time      `gorm:"column:updated_at"`
	CreatedAt time.Time      `gorm:"column:created_at"`
	CreateBy  int            `json:"CreateBy" form:"CreateBy" gorm:"column:CreateBy"`
	UpdateBy  int            `json:"UpdateBy" form:"UpdateBy" gorm:"column:UpdateBy"`
	LastLogin time.Time      `json:"LastLogin" form:"LastLogin" gorm:"column:LastLogin"`
	SessionID string         `json:"SessionID" form:"SessionID" gorm:"column:SessionID;size:255"`
	IP        string         `json:"IP" form:"IP" gorm:"column:IP;size:255"`
	DeletedAt gorm.DeletedAt `gorm:"index"`

	ProfileImage   string `json:"ProfileImage" gorm:"column:ProfileImage"`
	Type           int    `json:"Type" gorm:"column:Type"`
	LoginDelay     int64  `json:"LoginDelay" gorm:"column:LoginDelay"`
	Session        string `json:"session" gorm:"column:session"`
	SessionExpired int64  `json:"session_expired" gorm:"column:session_expired"`
}

func (Admin) TableName() string {
	return "admins"
}
