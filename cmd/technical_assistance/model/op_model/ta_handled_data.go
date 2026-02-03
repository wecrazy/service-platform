package op_model

import "gorm.io/gorm"

type TAHandledData struct {
	ID uint `gorm:"column:id;primaryKey;autoIncrement" json:"id"`
	gorm.Model
	CountFollowedUp      int `gorm:"column:count_followed_up" json:"count_followed_up"`
	CountPendingDataLeft int `gorm:"column:count_pending_data_left" json:"count_pending_data_left"`
	CountErrorDataLeft   int `gorm:"column:count_error_data_left" json:"count_error_data_left"`
	TotalTAStandBy       int `gorm:"column:total_ta_stand_by" json:"total_ta_stand_by"`
}

func (TAHandledData) TableName() string {
	return "ta_handled_data"
}
