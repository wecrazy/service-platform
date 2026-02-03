package model

import (
	"service-platform/cmd/web_panel/config"
	"time"

	"gorm.io/gorm"
)

type MerchantKresekBag struct {
	gorm.Model
	ID                      uint       `gorm:"column:id;primaryKey;autoIncrement" json:"id"`
	CustomerName            string     `gorm:"type:text;column:name;default:NULL" json:"name"`
	CustomerEmail           string     `gorm:"type:text;column:email;default:NULL" json:"email"`
	CustomerPhone           string     `gorm:"type:text;column:phone;default:NULL" json:"phone"`
	CustomerMobile          string     `gorm:"type:text;column:mobile;default:NULL" json:"mobile"`
	CustomerCity            string     `gorm:"type:text;column:city;default:NULL" json:"city"`
	CustomerAddress         string     `gorm:"type:text;column:address;default:NULL" json:"address"`
	TaxId                   string     `gorm:"type:text;column:tax_id;default:NULL" json:"tax_id"`
	TaxIdentificationNumber string     `gorm:"type:text;column:tin;default:NULL" json:"tin"`
	Agent                   string     `gorm:"type:text;column:agent;default:NULL" json:"agent"`
	DataAgent               string     `gorm:"type:text;column:data_agent;default:NULL" json:"data_agent"`
	AccountNumber           string     `gorm:"type:text;column:acc_holder_number;default:NULL" json:"acc_holder_number"`
	AccountName             string     `gorm:"type:text;column:acc_holder_name;default:NULL" json:"acc_holder_name"`
	MembershipExpiryDate    *time.Time `gorm:"type:date;column:membership_expiry_date" json:"membership_expiry_date"`
	MembershipId            int        `gorm:"type:int;column:membership_id" json:"-"`
	Membership              string     `gorm:"type:text;column:membership;default:NULL" json:"membership"`
	BankId                  int        `gorm:"type:int;column:bank_id" json:"-"`
	Bank                    string     `gorm:"type:text;column:bank;default:NULL" json:"bank"`
	CompanyId               int        `gorm:"type:int;column:company_id" json:"-"`
	Company                 string     `gorm:"type:text;column:company;default:NULL" json:"company"`
	FotoDepan               string     `gorm:"type:text;column:foto_lokasi_usaha_depan;default:NULL" json:"-"`
	FotoBelakang            string     `gorm:"type:text;column:foto_lokasi_usaha_belakang;default:NULL" json:"-"`
	FotoProduksi            string     `gorm:"type:text;column:foto_lokasi_usaha_produksi;default:NULL" json:"-"`
	FotoStok                string     `gorm:"type:text;column:foto_lokasi_usaha_stok;default:NULL" json:"-"`
	FotoKtp                 string     `gorm:"type:text;column:foto_ktp;default:NULL" json:"-"`
	FotoTtd                 string     `gorm:"type:text;column:foto_ttd;default:NULL" json:"-"`
	Foto                    string     `gorm:"-" json:"photos"`
}

func (MerchantKresekBag) TableName() string {
	return config.GetConfig().Database.TbMerchantKresekBag
}
