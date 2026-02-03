package models

import "gorm.io/gorm"

type TTS struct {
	gorm.Model
	TextToSpeech string
	StatusTTS    string `gorm:"type:varchar(50);default:null"`
}
