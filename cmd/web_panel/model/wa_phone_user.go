package model

import (
	"service-platform/cmd/web_panel/config"
	"time"

	"gorm.io/datatypes"
	"gorm.io/gorm"
)

// Project that user work to
type WAUserOf string

const (
	UserOfCSNA     WAUserOf = "pt_csna"
	UserOfHommyPay WAUserOf = "hommy_pay"
)

// WAAllowedChatMode defines where the bot can be contacted
type WAAllowedChatMode string

const (
	DirectChat WAAllowedChatMode = "direct"
	GroupChat  WAAllowedChatMode = "group"
	BothChat   WAAllowedChatMode = "both"
)

// WAMessageType defines what type of messages are allowed
type WAMessageType string

const (
	TextMessage     WAMessageType = "text"
	ImageMessage    WAMessageType = "image"
	VideoMessage    WAMessageType = "video"
	DocumentMessage WAMessageType = "document"
	AudioMessage    WAMessageType = "audio"
	// StickerMessage WAMessageType = "sticker"
	// add more e.g. sticker, document ...
)

// WAUserType defines the type/role of the user
type WAUserType string

const (
	CommonUser       WAUserType = "common"             // Regular user
	ODOOMSTechnician WAUserType = "odoo_ms_technician" // ODOO MS Technician
	OdooManager      WAUserType = "odoo_manager"       // ODOO management service user
	ODOOMSHead       WAUserType = "odoo_ms_head"       // ODOO Head
	ODOOMSSPL        WAUserType = "odoo_ms_spl"        // ODOO SPL
	ODOOMSStaff      WAUserType = "odoo_ms_staff"      // ODOO Staff
	SupportStaff     WAUserType = "support"            // Support team member
	Administrator    WAUserType = "admin"              // System administrator
	ServiceAccount   WAUserType = "service"            // Automated service account
	WaBotSuperUser   WAUserType = "bot_wa_super_user"

	CompanyCEO       WAUserType = "ceo"       // Company CEO
	CompanyCOO       WAUserType = "coo"       // Company COO
	CompanyCBO       WAUserType = "cbo"       // Company CBO
	CompanyPM        WAUserType = "pm"        // Project Manager
	CompanyPMO       WAUserType = "pmo"       // Project Management Officer
	CompanySecretary WAUserType = "secretary" // Company Secretary
	CompanyHR        WAUserType = "hr"        // Company Human Resource
)

var AllWAAllowedChatModes = []WAAllowedChatMode{
	DirectChat, GroupChat, BothChat,
}

var AllWAUserTypes = []WAUserType{
	CommonUser,
	ODOOMSTechnician, ODOOMSHead, ODOOMSSPL,
	ODOOMSStaff,
	SupportStaff,
	// OdooManager, SupportStaff, Administrator, ServiceAccount, WaBotSuperUser,
}

var AllWAMessageTypes = []WAMessageType{
	TextMessage, ImageMessage, VideoMessage, DocumentMessage,
	// AudioMessage, StickerMessage, ...
}

var AllUserOf = []WAUserOf{
	UserOfCSNA, UserOfHommyPay,
}

type WAPhoneUser struct {
	gorm.Model
	ID uint `gorm:"primaryKey;autoIncrement" json:"id"`

	FullName      string            `gorm:"column:full_name;type:varchar(255);not null" json:"full_name"`
	Email         string            `gorm:"column:email;type:varchar(255);not null" json:"email"`
	PhoneNumber   string            `gorm:"column:phone_number;type:varchar(20);uniqueIndex;not null" json:"phone_number"`
	IsRegistered  bool              `gorm:"column:is_registered;type:boolean;not null;default:false" json:"is_registered"`
	AllowedChats  WAAllowedChatMode `gorm:"column:allowed_chats;type:enum('direct','group','both');default:'direct'" json:"allowed_chats"`
	AllowedTypes  datatypes.JSON    `gorm:"column:allowed_types;type:json" json:"allowed_types"`
	AllowedToCall bool              `gorm:"column:allowed_to_call;type:boolean;not null;default:false" json:"allowed_to_call"` // Auto reject Voice & Video Call if using caller Primary Device
	MaxDailyQuota int               `gorm:"column:max_daily_quota;type:int;not null;default:10" json:"max_daily_quota"`
	Description   string            `gorm:"column:description;type:varchar(255)" json:"description"`
	IsBanned      bool              `gorm:"column:is_banned;type:boolean;not null;default:false" json:"is_banned"`
	UserType      WAUserType        `gorm:"column:user_type;type:varchar(20);not null;default:'common'" json:"user_type"`

	QuotaExcedeed *time.Time `gorm:"-" json:"quota_excedeed"`
	UserOf        WAUserOf   `gorm:"column:user_of;type:enum('pt_csna','hommy_pay');default:'pt_csna'" json:"user_of"`
}

func (WAPhoneUser) TableName() string {
	return config.GetConfig().Database.TbWAPhoneUser
}
