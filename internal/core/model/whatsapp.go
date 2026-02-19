package model

import (
	"service-platform/internal/config"
	"time"

	"gorm.io/datatypes"
	"gorm.io/gorm"
)

type WAUserOf string

const (
	CompanyEmployee       WAUserOf = "company_employee"
	ClientCompanyEmployee WAUserOf = "client_company_employee"
)

type WAAllowedChatMode string

const (
	DirectChat WAAllowedChatMode = "direct"
	GroupChat  WAAllowedChatMode = "group"
	BothChat   WAAllowedChatMode = "both"
)

type WAMessageType string

const (
	TextMessage         WAMessageType = "text"
	ImageMessage        WAMessageType = "image"
	VideoMessage        WAMessageType = "video"
	DocumentMessage     WAMessageType = "document"
	AudioMessage        WAMessageType = "audio"
	StickerMessage      WAMessageType = "sticker"
	LocationMessage     WAMessageType = "location"
	LiveLocationMessage WAMessageType = "live_location"
	ContactMessage      WAMessageType = "contact"
	ReactionMessage     WAMessageType = "reaction"
	// add more e.g. poll, button, list, product, group management ...
)

type WAUserType string

const (
	CommonUser        WAUserType = "common"
	SuperUser         WAUserType = "super_user"
	ClientUser        WAUserType = "client_user"
	AdministratorUser WAUserType = "user_administrator"
)

// All WA Allowed Chat Modes
var AllWAAllowedChatModes = []WAAllowedChatMode{DirectChat, GroupChat, BothChat}

// All WA Message Types
var AllWAMessageTypes = []WAMessageType{
	TextMessage,
	ImageMessage,
	VideoMessage,
	DocumentMessage,
	AudioMessage,
	// add more if needed
}

// All WA User Of
var AllWAUserOf = []WAUserOf{CompanyEmployee, ClientCompanyEmployee}

type WAUsers struct {
	ID uint `gorm:"primaryKey;autoIncrement" json:"id"`
	gorm.Model

	FullName      string            `gorm:"column:full_name;not null" json:"full_name"`
	Email         string            `gorm:"column:email;not null" json:"email"`
	PhoneNumber   string            `gorm:"column:phone_number;uniqueIndex;not null" json:"phone_number"`
	IsRegistered  bool              `gorm:"column:is_registered;not null;default:false" json:"is_registered"`
	AllowedChats  WAAllowedChatMode `gorm:"column:allowed_chats;type:varchar(20);default:'direct'" json:"allowed_chats"`
	AllowedTypes  datatypes.JSON    `gorm:"column:allowed_types;type:jsonb" json:"allowed_types"`
	AllowedToCall bool              `gorm:"column:allowed_to_call;not null;default:false" json:"allowed_to_call"`
	MaxDailyQuota int               `gorm:"column:max_daily_quota;type:int;not null;default:10" json:"max_daily_quota"`
	Description   string            `gorm:"column:description;type:text" json:"description"`
	IsBanned      bool              `gorm:"column:is_banned;not null;default:false" json:"is_banned"`
	UseBot        bool              `gorm:"column:use_bot;not null;default:true" json:"use_bot"`
	UserType      WAUserType        `gorm:"column:user_type;type:varchar(20);not null;default:'common'" json:"user_type"`

	QuotaExceeded *time.Time `gorm:"-" json:"quota_exceeded"`
	UserOf        WAUserOf   `gorm:"column:user_of;type:varchar(20);not null;default:'company_employee'" json:"user_of"`
}

func (WAUsers) TableName() string {
	return config.ServicePlatform.Get().Database.TbWhatsappUser
}

type WhatsappMessageAutoReply struct {
	gorm.Model

	ID          uint   `gorm:"column:id;primaryKey;autoIncrement" json:"id"`
	LanguageID  uint   `gorm:"column:language_id" json:"language_id"`
	Language    string `gorm:"-" json:"language"`
	Keywords    string `gorm:"column:keywords;type:text" json:"keywords"`
	ReplyText   string `gorm:"column:reply_text;type:text" json:"reply_text"`
	ForUserType string `gorm:"column:for_user_type;type:varchar(20);not null;default:'common'" json:"for_user_type"`
	UserOf      string `gorm:"column:user_of;type:varchar(20);not null;default:'company_employee'" json:"user_of"`
}

func (WhatsappMessageAutoReply) TableName() string {
	return config.ServicePlatform.Get().Database.TbWhatsappMessageAutoReply
}
