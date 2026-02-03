package sptechnicianmodel

import (
	"time"

	"gorm.io/gorm"
)

// SPTelegramMessage tracks all SP documents sent via Telegram
type SPTelegramMessage struct {
	ID uint `gorm:"primaryKey;autoIncrement" json:"id"`
	gorm.Model

	// Foreign keys for relationship tracking
	TechnicianGotSPID *uint `gorm:"column:technician_got_sp_id;index" json:"technician_got_sp_id"`
	SPLGotSPID        *uint `gorm:"column:spl_got_sp_id;index" json:"spl_got_sp_id"`
	SACGotSPID        *uint `gorm:"column:sac_got_sp_id;index" json:"sac_got_sp_id"`

	// Basic info
	RecipientType string `gorm:"column:recipient_type;type:varchar(50);index" json:"recipient_type"` // technician | spl | sac | hrd
	RecipientName string `gorm:"column:recipient_name;type:varchar(255);index" json:"recipient_name"`
	ChatID        string `gorm:"column:chat_id;type:varchar(100);index" json:"chat_id"` // Telegram chat ID
	PhoneNumber   string `gorm:"column:phone_number;type:varchar(50)" json:"phone_number"`
	ForProject    string `gorm:"column:for_project;type:varchar(255)" json:"for_project"`

	// SP details
	NumberOfSP   int    `gorm:"column:number_of_sp;type:int;index" json:"number_of_sp"`            // 1, 2, or 3
	SPFilePath   string `gorm:"column:sp_file_path;type:text" json:"sp_file_path"`                 // Path to the SP PDF file
	MessageText  string `gorm:"column:message_text;type:text" json:"message_text"`                 // Text message sent
	Pelanggaran  string `gorm:"column:pelanggaran;type:text" json:"pelanggaran"`                   // Violation description
	NoSurat      int    `gorm:"column:nomor_surat;type:int" json:"nomor_surat"`                    // Letter number
	TechnicianID string `gorm:"column:technician_id;type:varchar(255);index" json:"technician_id"` // Original technician who got the SP
	SPLID        string `gorm:"column:spl_id;type:varchar(255);index" json:"spl_id"`               // SPL name
	SACID        string `gorm:"column:sac_id;type:varchar(255);index" json:"sac_id"`               // SAC name

	// Telegram message details
	TelegramMessageID int64      `gorm:"column:telegram_message_id;type:bigint" json:"telegram_message_id"` // Message ID from Telegram
	SentAt            *time.Time `gorm:"column:sent_at;index" json:"sent_at"`
	SentSuccess       bool       `gorm:"column:sent_success;default:false;index" json:"sent_success"`
	ErrorMessage      string     `gorm:"column:error_message;type:text" json:"error_message"`

	// Response tracking
	HasResponse    bool       `gorm:"column:has_response;default:false;index" json:"has_response"`
	ResponseAt     *time.Time `gorm:"column:response_at" json:"response_at"`
	ResponseText   string     `gorm:"column:response_text;type:text" json:"response_text"`
	ResponseStatus string     `gorm:"column:response_status;type:varchar(50)" json:"response_status"` // acknowledged | pending | expired

	// Deadline and status
	ResponseDeadline     *time.Time `gorm:"column:response_deadline" json:"response_deadline"` // Deadline for response
	DeadlineExpired      bool       `gorm:"column:deadline_expired;default:false" json:"deadline_expired"`
	DeadlineExpiredCheck *time.Time `gorm:"column:deadline_expired_check" json:"deadline_expired_check"`

	// Additional metadata
	SPLName        string `gorm:"column:spl_name;type:varchar(255)" json:"spl_name"`
	SACName        string `gorm:"column:sac_name;type:varchar(255)" json:"sac_name"`
	TechnicianName string `gorm:"column:technician_name;type:varchar(255)" json:"technician_name"`
}

func (SPTelegramMessage) TableName() string {
	return "sp_telegram_messages"
}
