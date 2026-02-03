package sptechnicianmodel

import (
	"service-platform/cmd/web_panel/config"
	"time"

	"gorm.io/gorm"
)

type SPofStockOpname struct {
	ID uint `gorm:"primaryKey;autoIncrement" json:"id"`
	gorm.Model

	Technician string `gorm:"column:technician;type:varchar(255)" json:"technician"`
	Name       string `gorm:"column:name;type:varchar(255)" json:"name"`
	Email      string `gorm:"column:email;type:varchar(255)" json:"email"`
	NoHP       string `gorm:"column:no_hp;type:varchar(50)" json:"no_hp"`
	SPL        string `gorm:"column:spl;type:varchar(255)" json:"spl"`
	SAC        string `gorm:"column:sac;type:varchar(255)" json:"sac"`

	IsGotSP1 bool       `gorm:"column:is_got_sp1;default:false" json:"is_got_sp1"`
	GotSP1At *time.Time `gorm:"column:got_sp1_at" json:"got_sp1_at"`

	IsGotSP2 bool       `gorm:"column:is_got_sp2;default:false" json:"is_got_sp2"`
	GotSP2At *time.Time `gorm:"column:got_sp2_at" json:"got_sp2_at"`

	IsGotSP3 bool       `gorm:"column:is_got_sp3;default:false" json:"is_got_sp3"`
	GotSP3At *time.Time `gorm:"column:got_sp3_at" json:"got_sp3_at"`

	PelanggaranSP1 string `gorm:"column:pelanggaran_sp1;type:longtext" json:"pelanggaran_sp1"`
	PelanggaranSP2 string `gorm:"column:pelanggaran_sp2;type:longtext" json:"pelanggaran_sp2"`
	PelanggaranSP3 string `gorm:"column:pelanggaran_sp3;type:longtext" json:"pelanggaran_sp3"`

	WhatsappMessages    []SPStockOpnameWhatsappMessage `gorm:"foreignKey:TechnicianGotSPID" json:"-"`
	SP1WhatsappMessages []SPStockOpnameWhatsappMessage `gorm:"foreignKey:TechnicianGotSPID;where:number_of_sp = 1" json:"sp1_whatsapp_messages"`
	SP2WhatsappMessages []SPStockOpnameWhatsappMessage `gorm:"foreignKey:TechnicianGotSPID;where:number_of_sp = 2" json:"sp2_whatsapp_messages"`
	SP3WhatsappMessages []SPStockOpnameWhatsappMessage `gorm:"foreignKey:TechnicianGotSPID;where:number_of_sp = 3" json:"sp3_whatsapp_messages"`

	SP1SoundTTSPath string `gorm:"column:sp1_sound_tts_path;type:text" json:"sp1_sound_tts_path"`
	SP2SoundTTSPath string `gorm:"column:sp2_sound_tts_path;type:text" json:"sp2_sound_tts_path"`
	SP3SoundTTSPath string `gorm:"column:sp3_sound_tts_path;type:text" json:"sp3_sound_tts_path"`

	SP1SoundPlayed bool `gorm:"column:sp1_sound_played;default:false" json:"sp1_sound_played"`
	SP2SoundPlayed bool `gorm:"column:sp2_sound_played;default:false" json:"sp2_sound_played"`
	SP3SoundPlayed bool `gorm:"column:sp3_sound_played;default:false" json:"sp3_sound_played"`

	SP1SoundPlayedAt *time.Time `gorm:"column:sp1_sound_played_at" json:"sp1_sound_played_at"`
	SP2SoundPlayedAt *time.Time `gorm:"column:sp2_sound_played_at" json:"sp2_sound_played_at"`
	SP3SoundPlayedAt *time.Time `gorm:"column:sp3_sound_played_at" json:"sp3_sound_played_at"`

	SP1FilePath string `gorm:"column:sp1_file_path;type:text" json:"sp1_file_path"`
	SP2FilePath string `gorm:"column:sp2_file_path;type:text" json:"sp2_file_path"`
	SP3FilePath string `gorm:"column:sp3_file_path;type:text" json:"sp3_file_path"`
}

func (SPofStockOpname) TableName() string {
	return config.GetConfig().StockOpname.TbListSPSO
}
