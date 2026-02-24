package model

import (
	"service-platform/internal/config"
	"time"

	"gorm.io/datatypes"
	"gorm.io/gorm"
)

// WAUserOf defines the type of user that can be associated with WhatsApp functionality.
type WAUserOf string

// Define constants for WAUserOf to represent the different types of users that can be associated with WhatsApp functionality in the application. These constants can be used to classify WhatsApp users based on their relationship to the company, such as employees of the company or employees of client companies.
const (
	CompanyEmployee       WAUserOf = "company_employee"
	ClientCompanyEmployee WAUserOf = "client_company_employee"
)

// WAAllowedChatMode defines the allowed chat modes for WhatsApp users.
type WAAllowedChatMode string

// Define constants for WAAllowedChatMode to represent the different chat modes that can be allowed for WhatsApp users in the application. These constants can be used to specify whether a WhatsApp user is allowed to participate in direct chats, group chats, or both types of chats.
const (
	DirectChat WAAllowedChatMode = "direct"
	GroupChat  WAAllowedChatMode = "group"
	BothChat   WAAllowedChatMode = "both"
)

// WAMessageType defines the types of messages that can be sent or received via WhatsApp.
type WAMessageType string

// Define constants for WAMessageType to represent the different types of messages that can be sent and received through WhatsApp in the application. These constants can be used to specify the types of messages that a WhatsApp user is allowed to send or receive, such as text messages, images, videos, documents, audio files, stickers, locations, contacts, reactions, and more.
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

// WAUserType defines the role/type of a WhatsApp user in the application.
type WAUserType string

// Define constants for WAUserType to represent the different types of WhatsApp users in the application. These constants can be used to classify WhatsApp users based on their roles and permissions, such as common users, super users, client users, and administrator users.
const (
	CommonUser        WAUserType = "common"
	SuperUser         WAUserType = "super_user"
	ClientUser        WAUserType = "client_user"
	AdministratorUser WAUserType = "user_administrator"
)

// AllWAAllowedChatModes is the list of all allowed WhatsApp chat modes.
var AllWAAllowedChatModes = []WAAllowedChatMode{DirectChat, GroupChat, BothChat}

// AllWAMessageTypes is the list of all valid WhatsApp message types.
var AllWAMessageTypes = []WAMessageType{
	TextMessage,
	ImageMessage,
	VideoMessage,
	DocumentMessage,
	AudioMessage,
	// add more if needed
}

// AllWAUserOf is the list of all valid WAUserOf values.
var AllWAUserOf = []WAUserOf{CompanyEmployee, ClientCompanyEmployee}

// WAUsers represents a user of the WhatsApp functionality in the application, with fields for full name, email, phone number, registration status, allowed chat modes, allowed message types, call permissions, daily quota, description, ban status, bot usage, user type, and user association. The TableName method specifies the database table name for this model, which is defined in the configuration file under Database.TbWhatsappUser.
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

// TableName specifies the database table name for the WAUsers model, which is defined in the configuration file under Database.TbWhatsappUser.
func (WAUsers) TableName() string {
	return config.ServicePlatform.Get().Database.TbWhatsappUser
}

// WhatsappMessageAutoReply represents an auto-reply message configuration for WhatsApp, with fields for language ID, keywords, reply text, user type, and user association. The TableName method specifies the database table name for this model, which is defined in the configuration file under Database.TbWhatsappMessageAutoReply.
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

// TableName specifies the database table name for the WhatsappMessageAutoReply model, which is defined in the configuration file under Database.TbWhatsappMessageAutoReply.
func (WhatsappMessageAutoReply) TableName() string {
	return config.ServicePlatform.Get().Database.TbWhatsappMessageAutoReply
}
