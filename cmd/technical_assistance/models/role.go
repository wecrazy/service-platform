package models

import "gorm.io/gorm"

type Role struct {
	gorm.Model
	Updated_By string `gorm:"type:varchar(100)"`
	Name       string `gorm:"type:varchar(50)"`
}
