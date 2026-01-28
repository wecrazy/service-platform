package telegrammodel

import (
	"service-platform/internal/config"

	"gorm.io/gorm"
)

// TelegramUserOf defines the type of Telegram user
type TelegramUserOf string

const (
	CompanyEmployee TelegramUserOf = "company_employee"
	ClientEmployee  TelegramUserOf = "client_employee"
)

// TelegramUserType defines the role of Telegram user
type TelegramUserType string

const (
	CommonUser   TelegramUserType = "common"
	SuperUser    TelegramUserType = "super_user"
	TechnicianMS TelegramUserType = "technician_ms"
	SPLMS        TelegramUserType = "spl_ms"
	SACMS        TelegramUserType = "sac_ms"
	TAMS         TelegramUserType = "technical_assistance_ms"
	HeadMS       TelegramUserType = "head_ms"
)

// TelegramUsers represents a Telegram user record in the database
type TelegramUsers struct {
	ID uint `gorm:"primaryKey;autoIncrement" json:"id"`
	gorm.Model

	ChatID      *int64 `gorm:"column:telegram_chat_id;uniqueIndex" json:"telegram_chat_id"` // Chat ID that uniquely identifies the user
	FullName    string `gorm:"column:full_name;not null" json:"full_name"`
	Username    string `gorm:"column:username;type:varchar(255)" json:"username"`
	PhoneNumber string `gorm:"column:phone_number;type:varchar(30)" json:"phone_number"`
	Email       string `gorm:"column:email;type:varchar(255)" json:"email"`
	Description string `gorm:"column:description;type:text" json:"description"`

	UserType     TelegramUserType `gorm:"column:telegram_user_type;type:varchar(100)" json:"telegram_user_type"`
	UserOf       TelegramUserOf   `gorm:"column:telegram_user_of;type:varchar(100)" json:"telegram_user_of"`
	IsBanned     bool             `gorm:"column:is_banned;not null;default:false" json:"is_banned"`
	VerifiedUser bool             `gorm:"column:verified_user;not null;default:false" json:"verified_user"`

	MaxDailyQuota int `gorm:"column:max_daily_quota;type:int;not null;default:100" json:"max_daily_quota"`
	// Removed DailyUsageCount, LastQuotaReset - now stored in Redis
}

func (TelegramUsers) TableName() string {
	return config.GetConfig().Telegram.Tables.TBTelegramUser
}
