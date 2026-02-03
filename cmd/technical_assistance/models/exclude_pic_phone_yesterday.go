package models

import "gorm.io/gorm"

type ExcludePICPhoneYesterday struct {
	gorm.Model
	PhoneNumber *string `gorm:"type:varchar(50);column:phone_number;default:NULL" json:"phone_number"`
}
