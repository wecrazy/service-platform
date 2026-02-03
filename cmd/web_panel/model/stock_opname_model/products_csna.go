package stockopnamemodel

import (
	"service-platform/cmd/web_panel/config"
	"time"

	"gorm.io/gorm"
)

type ProductEDCCSNA struct {
	ID uint `gorm:"column:id;primaryKey;autoIncrement" json:"id"`
	gorm.Model

	Company           string `gorm:"column:company" json:"company"`
	SN_EDC            string `gorm:"column:sn_edc" json:"sn_edc"`
	Product           string `gorm:"column:product" json:"product"`
	ProductID         int    `gorm:"column:product_id" json:"product_id"`
	ProductCategory   string `gorm:"column:product_category" json:"product_category"`
	ProductCategoryID int    `gorm:"column:product_category_id" json:"product_category_id"`

	DateProductMove *time.Time `gorm:"column:date_product_move" json:"date_product_move"`
	Reference       string     `gorm:"column:reference" json:"reference"`
	Source          string     `gorm:"column:source" json:"source"`
	From            string     `gorm:"column:from_location" json:"from_location"`
	To              string     `gorm:"column:to_location" json:"to_location"`
}

func (ProductEDCCSNA) TableName() string {
	return config.GetConfig().StockOpname.TbListProductEDC
}
