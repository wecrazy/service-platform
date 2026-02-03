package model

type DataTeknisi struct {
	ID         int     `json:"id" form:"id" gorm:"column:id;primaryKey;autoIncrement"`
	Nama       string  `json:"nama" form:"nama" gorm:"column:nama;type:varchar(300);not null"`
	NoHP       string  `json:"no_hp" form:"no_hp" gorm:"column:no_hp;type:varchar(20);not null"`
	Keterangan *string `json:"keterangan" form:"keterangan" gorm:"column:keterangan;type:varchar(250);default:null"`
	SPL        *string `json:"spl" form:"spl" gorm:"column:spl;type:varchar(200);default:null"`
}

// TableName overrides the default table name
func (DataTeknisi) TableName() string {
	return "data_teknisi"
}
