package model

import (
	"time"
)

type TeknisiLocation struct {
	ID        uint      `json:"id" gorm:"column:id;primarykey"`
	TeknisiID uint      `json:"teknisi_id" gorm:"column:teknisi_id"`
	No        int       `json:"no" gorm:"column:no"`
	Lat       float64   `json:"lat" gorm:"column:lat;type:decimal(11,8);not null"`
	Long      float64   `json:"long" gorm:"column:long;type:decimal(11,8);not null"`
	LocalTime time.Time `json:"local_time" gorm:"column:local_time;type:timestamp;default:CURRENT_TIMESTAMP"`
	CreatedAt time.Time `json:"created_at" gorm:"column:created_at"`
}

func (TeknisiLocation) TableName() string {
	return "teknisi_location"
}
