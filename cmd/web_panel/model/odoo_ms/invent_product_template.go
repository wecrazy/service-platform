package odooms

import (
	"service-platform/internal/config"

	"gorm.io/gorm"
)

type InventoryProductTemplate struct {
	ID uint `gorm:"primaryKey;autoIncrement" json:"id"`
	gorm.Model

	Name string `gorm:"column:name;type:text" json:"name"`

	ProductType     string  `gorm:"column:product_type;type:text" json:"product_type"`
	ProductCategory string  `gorm:"column:product_category;type:text" json:"product_category"`
	Company         string  `gorm:"column:company;type:text" json:"company"`
	ListPrice       float64 `gorm:"column:list_price;type:numeric" json:"list_price"`
}

func (InventoryProductTemplate) TableName() string {
	return config.WebPanel.Get().Database.TbProductTemplate
}
