package models

import "gorm.io/gorm"

type User struct {
	gorm.Model
	UpdatedBy string `gorm:"type:varchar(100);default:null"`
	UserName  string `gorm:"type:varchar(100);not null"`
	FirstName string `gorm:"type:varchar(50)"`
	LastName  string `gorm:"type:varchar(50)"`
	Email     string `gorm:"type:varchar(200);not null"`
	Password  string
	RoleId    int `gorm:"type:int(3)"`
	Img       string
}
