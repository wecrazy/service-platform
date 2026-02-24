package model

import (
	"service-platform/internal/config"

	"gorm.io/gorm"
)

// LogActivity represents a log of user activities in the application, including details such as the user ID, full name, email, action performed, status of the action, log message, user agent, request method, IP address, and request URI. The TableName method specifies the database table name for this model, which is defined in the configuration file under Database.TbLogActivity.
type LogActivity struct {
	ID uint `json:"id" gorm:"column:id;primarykey"`
	gorm.Model

	UserID    uint   `json:"user_id" gorm:"column:user_id"`
	FullName  string `json:"full_name" gorm:"column:full_name"`
	Email     string `json:"email" gorm:"column:email"`
	Action    string `json:"action" gorm:"column:action"`
	Status    string `json:"status" gorm:"column:status"`
	Log       string `json:"log" gorm:"column:log"`
	UserAgent string `json:"user_agent" gorm:"column:user_agent"`
	ReqMethod string `json:"req_method" gorm:"column:req_method"`
	IP        string `json:"ip" gorm:"column:ip"`
	ReqURI    string `json:"req_uri" gorm:"column:req_uri"`
}

// TableName specifies the database table name for the LogActivity model, which is defined in the configuration file under Database.TbLogActivity.
func (LogActivity) TableName() string {
	return config.ServicePlatform.Get().Database.TbLogActivity
}
