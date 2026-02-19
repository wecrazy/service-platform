package model

import "service-platform/internal/config"

// IndonesiaRegion represents the indonesia_region table structure
type IndonesiaRegion struct {
	RegionID      uint   `gorm:"primaryKey;autoIncrement;column:region_id" json:"region_id"`
	Province      string `gorm:"size:150;not null;column:province" json:"province"`
	District      string `gorm:"size:150;not null;column:district" json:"district"`
	Subdistrict   string `gorm:"size:150;not null;column:subdistrict" json:"subdistrict"`
	Area          string `gorm:"size:150;column:area" json:"area"`
	PostCode      string `gorm:"size:10;column:post_code" json:"post_code"`
	Longitude     string `gorm:"size:20;column:longitude" json:"longitude"`
	Latitude      string `gorm:"size:20;column:latitude" json:"latitude"`
	ProvinceRO    *int   `gorm:"column:province_ro" json:"province_ro"`
	CityRO        *int   `gorm:"column:city_ro" json:"city_ro"`
	SubdistrictRO *int   `gorm:"column:subdistrict_ro" json:"subdistrict_ro"`
}

func (IndonesiaRegion) TableName() string {
	return config.ServicePlatform.Get().Database.TbIndonesiaRegion
}
