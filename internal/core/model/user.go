package model

import (
	"service-platform/internal/config"
	"time"

	"gorm.io/gorm"
)

type Users struct {
	ID uint `json:"id" form:"id" gorm:"column:id;primaryKey;autoIncrement"`
	gorm.Model

	Fullname  string     `json:"fullname" form:"fullname" gorm:"column:fullname;size:50"`
	Username  string     `json:"username" form:"username" gorm:"column:username;size:50"`
	Phone     string     `json:"phone" form:"phone" gorm:"column:phone;size:20"`
	Email     string     `json:"email" form:"email" gorm:"column:email;size:50"`
	Password  string     `json:"password" form:"password" gorm:"column:password;size:100"`
	Role      int        `json:"role" form:"role" gorm:"column:role;default:0"`
	Status    int        `json:"status" form:"status" gorm:"column:status;default:0"`
	CreateBy  int        `json:"create_by" form:"create_by" gorm:"column:create_by"`
	UpdateBy  int        `json:"update_by" form:"update_by" gorm:"column:update_by"`
	LastLogin *time.Time `json:"last_login" form:"last_login" gorm:"column:last_login"`
	SessionID string     `json:"session_id" form:"session_id" gorm:"column:session_id;size:255"`
	IP        string     `json:"ip" form:"ip" gorm:"column:ip;size:255"`

	ProfileImage   string `json:"profile_image" gorm:"column:profile_image"`
	Type           int    `json:"type" gorm:"column:type"`
	LoginDelay     int64  `json:"login_delay" gorm:"column:login_delay"`
	Session        string `json:"session" gorm:"column:session"`
	SessionExpired int64  `json:"session_expired" gorm:"column:session_expired"`

	LoginAttempts   int        `gorm:"column:login_attempts" json:"login_attempts"`
	MaxRetry        int        `gorm:"column:max_retry;default:5" json:"max_retry"`
	LockUntil       *time.Time `gorm:"column:lock_until" json:"lock_until"`
	LastFailedLogin *time.Time `gorm:"column:last_failed_login" json:"last_failed_login"`
}

func (Users) TableName() string {
	return config.ServicePlatform.Get().Database.TbUser
}

type UserStatus struct {
	ID uint `json:"id" gorm:"column:id;primarykey"`
	gorm.Model

	Title     string `json:"title" gorm:"column:title"`
	ClassName string `json:"class_name" gorm:"column:class_name"`
}

func (UserStatus) TableName() string {
	return config.ServicePlatform.Get().Database.TbUserStatus
}

type UserPasswordChangeLog struct {
	gorm.Model
	Email    string `json:"email" gorm:"column:email;size:100"`
	Password string `json:"password" gorm:"column:password;size:100"`
}

func (UserPasswordChangeLog) TableName() string {
	return config.ServicePlatform.Get().Database.TbUserPasswordChangeLog
}
