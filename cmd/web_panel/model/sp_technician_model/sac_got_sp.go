package sptechnicianmodel

import (
	"service-platform/cmd/web_panel/config"
	"time"

	"gorm.io/gorm"
)

type SACGotSP struct {
	ID uint `gorm:"primaryKey;autoIncrement" json:"id"`
	gorm.Model

	SAC        string `gorm:"column:sac;type:varchar(255)" json:"sac"`
	Name       string `gorm:"column:name;type:varchar(255)" json:"name"`
	ForProject string `gorm:"column:for_project;type:varchar(255)" json:"for_project"`

	IsGotSP1 bool       `gorm:"column:is_got_sp1;default:false" json:"is_got_sp1"`
	GotSP1At *time.Time `gorm:"column:got_sp1_at" json:"got_sp1_at"`

	IsGotSP2 bool       `gorm:"column:is_got_sp2;default:false" json:"is_got_sp2"`
	GotSP2At *time.Time `gorm:"column:got_sp2_at" json:"got_sp2_at"`

	IsGotSP3 bool       `gorm:"column:is_got_sp3;default:false" json:"is_got_sp3"`
	GotSP3At *time.Time `gorm:"column:got_sp3_at" json:"got_sp3_at"`

	NoSP1 int `gorm:"column:nomor_surat_sp1" json:"nomor_surat_sp1"`
	NoSP2 int `gorm:"column:nomor_surat_sp2" json:"nomor_surat_sp2"`
	NoSP3 int `gorm:"column:nomor_surat_sp3" json:"nomor_surat_sp3"`

	PelanggaranSP1 string `gorm:"column:pelanggaran_sp1;type:text" json:"pelanggaran_sp1"`
	PelanggaranSP2 string `gorm:"column:pelanggaran_sp2;type:text" json:"pelanggaran_sp2"`
	PelanggaranSP3 string `gorm:"column:pelanggaran_sp3;type:text" json:"pelanggaran_sp3"`

	TechnicianNameCausedGotSP1 string `gorm:"column:technician_name_caused_got_sp1;type:varchar(255)" json:"technician_name_caused_got_sp1"`
	TechnicianNameCausedGotSP2 string `gorm:"column:technician_name_caused_got_sp2;type:varchar(255)" json:"technician_name_caused_got_sp2"`
	TechnicianNameCausedGotSP3 string `gorm:"column:technician_name_caused_got_sp3;type:varchar(255)" json:"technician_name_caused_got_sp3"`

	WhatsappMessages    []SPWhatsAppMessage `gorm:"foreignKey:SACGotSPID" json:"-"`
	SP1WhatsappMessages []SPWhatsAppMessage `gorm:"foreignKey:SACGotSPID;where:number_of_sp = 1" json:"sp1_whatsapp_messages"`
	SP2WhatsappMessages []SPWhatsAppMessage `gorm:"foreignKey:SACGotSPID;where:number_of_sp = 2" json:"sp2_whatsapp_messages"`
	SP3WhatsappMessages []SPWhatsAppMessage `gorm:"foreignKey:SACGotSPID;where:number_of_sp = 3" json:"sp3_whatsapp_messages"`

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

func (SACGotSP) TableName() string {
	return config.GetConfig().SPTechnician.TBSACGotSP
}
