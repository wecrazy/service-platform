package model

import (
	"time"

	"gorm.io/gorm"
)

type Teknisi struct {
	ID          uint           `form:"id" json:"id" gorm:"column:id;primarykey"`
	FullName    string         `form:"full_name" json:"full_name" gorm:"column:full_name"`
	IdEmployee  int            `form:"id_employee" json:"id_employee" gorm:"column:id_employee;unique"`
	Birthdate   time.Time      `form:"birthdate" json:"birthdate" gorm:"column:birthdate" time_format:"2006-01-02" `
	Status      string         `form:"status" json:"status" gorm:"column:status"`
	NoKTP       string         `form:"no_ktp" json:"no_ktp" gorm:"column:no_ktp"`
	NoHp        string         `form:"no_hp" json:"no_hp" gorm:"column:no_hp"`
	ServicePoin string         `form:"service_poin" json:"service_poin" gorm:"column:service_poin"`
	Address     string         `form:"address" json:"address" gorm:"column:address"`
	UpdatedAt   time.Time      `gorm:"column:updated_at"`
	CreatedAt   time.Time      `gorm:"column:created_at"`
	DeletedAt   gorm.DeletedAt `gorm:"index"`
}

func (Teknisi) TableName() string {
	return "teknisi"
}
