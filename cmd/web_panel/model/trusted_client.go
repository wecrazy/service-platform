package model

import (
	"service-platform/cmd/web_panel/config"

	"gorm.io/gorm"
)

type TrustedClient struct {
	ID uint `gorm:"primaryKey;column:id" json:"id"`
	gorm.Model

	ClientName   string `gorm:"type:varchar(255);column:client_name;default:NULL" json:"client_name"`
	FullName     string `gorm:"type:varchar(355);column:full_name;default:NULL" json:"full_name"`
	PhoneNumbers string `gorm:"type:text;column:phone_numbers;default:NULL" json:"phone_numbers"` // JSON string: []string use comma to separate
	Emails       string `gorm:"type:text;column:emails;default:NULL" json:"emails"`               // JSON string: []string use comma to separate
	MerchantName string `gorm:"type:varchar(255);column:merchant_name;default:NULL" json:"merchant_name"`

	Address   string `gorm:"type:text;column:address;default:NULL" json:"address"`
	City      string `gorm:"type:varchar(128);column:city;default:NULL" json:"city"`
	Country   string `gorm:"type:varchar(128);column:country;default:NULL" json:"country"`
	Status    string `gorm:"type:varchar(64);column:status;default:'active'" json:"status"`
	Notes     string `gorm:"type:text;column:notes;default:NULL" json:"notes"`
	CreatedBy string `gorm:"type:varchar(128);column:created_by;default:NULL" json:"created_by"`
	UpdatedBy string `gorm:"type:varchar(128);column:updated_by;default:NULL" json:"updated_by"`
}

func (TrustedClient) TableName() string {
	return config.GetConfig().Database.TbTrustedClient
}
