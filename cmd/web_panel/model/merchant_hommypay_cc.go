package model

import (
	"service-platform/cmd/web_panel/config"

	"gorm.io/gorm"
)

type MerchantHommyPayCC struct {
	gorm.Model
	ID              uint   `gorm:"column:id;primaryKey;autoIncrement" json:"id"`
	MerchantId      int    `gorm:"type:int;column:merchant_id" json:"-"`
	MerchantName    string `gorm:"type:text;column:merchant_name;default:NULL" json:"merchant_name"`
	MerchantPhone   string `gorm:"type:text;column:merchant_phone;default:NULL" json:"merchant_phone"`
	MerchantAddress string `gorm:"type:text;column:merchant_address;default:NULL" json:"merchant_address"`
	MerchantCity    string `gorm:"type:text;column:merchant_city;default:NULL" json:"merchant_city"`
	MerchantEmail   string `gorm:"type:text;column:merchant_email;default:NULL" json:"merchant_email"`
	MerchantOwner   string `gorm:"type:text;column:merchant_owner;default:NULL" json:"-"`
	SnId            int    `gorm:"type:int;column:sn_id" json:"-"`
	Sn              string `gorm:"type:text;column:sn;default:NULL" json:"sn"`
	ProductId       int    `gorm:"type:int;column:product_id" json:"-"`
	Product         string `gorm:"type:text;column:product;default:NULL" json:"product"`
	Longitude       string `gorm:"type:text;column:longitude;default:NULL" json:"longitude"`
	Latitude        string `gorm:"type:text;column:latitude;default:NULL" json:"latitude"`
}

func (MerchantHommyPayCC) TableName() string {
	return config.GetConfig().Database.TbMerchant
}
