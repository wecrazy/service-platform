package model

import (
	"service-platform/cmd/web_panel/config"
	"time"

	"gorm.io/gorm"
)

type IncomingEmail struct {
	ID uint `gorm:"primaryKey;column:id" json:"id"`
	gorm.Model

	MessageID string    `gorm:"type:varchar(255);uniqueIndex;column:message_id" json:"message_id"`
	Subject   string    `gorm:"column:subject" json:"subject"`
	FromEmail string    `gorm:"column:from_email" json:"from_email"`
	ToEmail   string    `gorm:"column:to_email" json:"to_email"`
	DateEmail time.Time `gorm:"column:date_email" json:"date_email"`

	PlainText string `gorm:"type:text;column:plain_text" json:"plain_text"`
	HTMLText  string `gorm:"type:text;column:html_text" json:"html_text"`
	Links     string `gorm:"type:text;column:links" json:"links"`   // JSON string: []string
	Emails    string `gorm:"type:text;column:emails" json:"emails"` // JSON string: []string

	IsHTML     bool   `gorm:"column:is_html" json:"is_html"`
	RawContent string `gorm:"type:text;column:raw_content" json:"raw_content"`
	IsRead     bool   `gorm:"default:false;column:is_read" json:"is_read"`
}

func (IncomingEmail) TableName() string {
	return config.GetConfig().Database.TbMail
}
