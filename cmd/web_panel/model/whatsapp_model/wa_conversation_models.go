package whatsappmodel

import (
	"service-platform/cmd/web_panel/config"
	"service-platform/cmd/web_panel/model"
	"time"

	"gorm.io/gorm"
)

// WAConversation represents a WhatsApp conversation (group or private)
type WAConversation struct {
	ID              uint           `gorm:"primaryKey;column:id" json:"id"`
	UserID          uint           `gorm:"not null;index;column:user_id" json:"user_id"`
	ContactJID      string         `gorm:"not null;index;column:contact_jid" json:"contact_jid"`
	ContactName     string         `gorm:"not null;column:contact_name" json:"contact_name"`
	ContactPhone    string         `gorm:"column:contact_phone" json:"contact_phone"`
	IsGroup         bool           `gorm:"default:false;column:is_group" json:"is_group"`
	GroupSubject    string         `gorm:"column:group_subject" json:"group_subject"`
	Avatar          string         `gorm:"column:avatar" json:"avatar"`
	LastMessage     string         `gorm:"column:last_message" json:"last_message"`
	LastMessageTime *time.Time     `gorm:"column:last_message_time" json:"last_message_time"`
	UnreadCount     int            `gorm:"default:0;column:unread_count" json:"unread_count"`
	IsMuted         bool           `gorm:"default:false;column:is_muted" json:"is_muted"`
	IsArchived      bool           `gorm:"default:false;column:is_archived" json:"is_archived"`
	IsBlocked       bool           `gorm:"default:false;column:is_blocked" json:"is_blocked"`
	IsPinned        bool           `gorm:"default:false;column:is_pinned" json:"is_pinned"`
	CreatedAt       time.Time      `gorm:"column:created_at" json:"created_at"`
	UpdatedAt       time.Time      `gorm:"column:updated_at" json:"updated_at"`
	DeletedAt       gorm.DeletedAt `gorm:"index;column:deleted_at" json:"deleted_at,omitempty"`

	// Relationships
	User         model.Admin          `gorm:"foreignKey:UserID"`
	Messages     []WAChatMessage      `gorm:"foreignKey:ConversationID"`
	GroupMembers []WAGroupParticipant `gorm:"foreignKey:ConversationID"`
}

// WAChatMessage represents a WhatsApp message in conversations
type WAChatMessage struct {
	ID              uint           `gorm:"primaryKey;column:id" json:"id"`
	ConversationID  uint           `gorm:"not null;index;column:conversation_id" json:"conversation_id"`
	UserID          uint           `gorm:"not null;index;column:user_id" json:"user_id"`
	MessageID       string         `gorm:"unique;not null;column:message_id" json:"message_id"`
	FromJID         string         `gorm:"not null;column:from_jid" json:"from_jid"`
	ToJID           string         `gorm:"not null;column:to_jid" json:"to_jid"`
	MessageType     string         `gorm:"not null;column:message_type" json:"message_type"`
	MessageContent  string         `gorm:"type:text;column:message_content" json:"message_content"`
	MediaURL        string         `gorm:"column:media_url" json:"media_url"`
	MediaFileName   string         `gorm:"column:media_filename" json:"media_filename"`
	MediaMimeType   string         `gorm:"column:media_mime_type" json:"media_mime_type"`
	MediaSize       int64          `gorm:"column:media_size" json:"media_size"`
	ThumbnailURL    string         `gorm:"column:thumbnail_url" json:"thumbnail_url"`
	IsOutgoing      bool           `gorm:"not null;column:is_outgoing" json:"is_outgoing"`
	MessageStatus   string         `gorm:"default:'sent';column:message_status" json:"message_status"`
	IsDeleted       bool           `gorm:"default:false;column:is_deleted" json:"is_deleted"`
	IsStarred       bool           `gorm:"default:false;column:is_starred" json:"is_starred"`
	IsForwarded     bool           `gorm:"default:false;column:is_forwarded" json:"is_forwarded"`
	ForwardedFrom   string         `gorm:"column:forwarded_from" json:"forwarded_from"`
	QuotedMessageID *uint          `gorm:"column:quoted_message_id" json:"quoted_message_id"`
	QuotedContent   string         `gorm:"column:quoted_content" json:"quoted_content"`
	Location        string         `gorm:"column:location" json:"location"`
	LocationName    string         `gorm:"column:location_name" json:"location_name"`
	ContactVCard    string         `gorm:"column:contact_vcard" json:"contact_vcard"`
	Mentions        string         `gorm:"column:mentions" json:"mentions"`
	Reactions       string         `gorm:"column:reactions" json:"reactions"`
	EditedAt        *time.Time     `gorm:"column:edited_at" json:"edited_at"`
	Timestamp       time.Time      `gorm:"not null;index;column:timestamp" json:"timestamp"`
	CreatedAt       time.Time      `gorm:"column:created_at" json:"created_at"`
	UpdatedAt       time.Time      `gorm:"column:updated_at" json:"updated_at"`
	DeletedAt       gorm.DeletedAt `gorm:"index;column:deleted_at" json:"deleted_at,omitempty"`

	// Relationships
	Conversation WAConversation `gorm:"foreignKey:ConversationID"`
	User         model.Admin    `gorm:"foreignKey:UserID"`
}

// WAGroupParticipant represents members of a WhatsApp group
type WAGroupParticipant struct {
	ID              uint           `gorm:"primaryKey;column:id" json:"id"`
	ConversationID  uint           `gorm:"not null;index;column:conversation_id" json:"conversation_id"`
	UserID          uint           `gorm:"not null;index;column:user_id" json:"user_id"`
	ParticipantJID  string         `gorm:"not null;column:participant_jid" json:"participant_jid"`
	ParticipantName string         `gorm:"column:participant_name" json:"participant_name"`
	ParticipantRole string         `gorm:"default:'member';column:participant_role" json:"participant_role"`
	Avatar          string         `gorm:"column:avatar" json:"avatar"`
	JoinedAt        time.Time      `gorm:"column:joined_at" json:"joined_at"`
	LeftAt          *time.Time     `gorm:"column:left_at" json:"left_at"`
	IsActive        bool           `gorm:"default:true;column:is_active" json:"is_active"`
	AddedBy         string         `gorm:"column:added_by" json:"added_by"`
	CreatedAt       time.Time      `gorm:"column:created_at" json:"created_at"`
	UpdatedAt       time.Time      `gorm:"column:updated_at" json:"updated_at"`
	DeletedAt       gorm.DeletedAt `gorm:"index;column:deleted_at" json:"deleted_at,omitempty"`

	// Relationships
	Conversation WAConversation `gorm:"foreignKey:ConversationID"`
	User         model.Admin    `gorm:"foreignKey:UserID"`
}

// WAContactInfo represents a WhatsApp contact
type WAContactInfo struct {
	ID           uint           `gorm:"primaryKey;column:id" json:"id"`
	UserID       uint           `gorm:"not null;index;column:user_id" json:"user_id"`
	ContactJID   string         `gorm:"not null;index;column:contact_jid" json:"contact_jid"`
	ContactName  string         `gorm:"column:contact_name" json:"contact_name"`
	PhoneNumber  string         `gorm:"column:phone_number" json:"phone_number"`
	Avatar       string         `gorm:"column:avatar" json:"avatar"`
	StatusText   string         `gorm:"column:status_text" json:"status_text"`
	IsBlocked    bool           `gorm:"default:false;column:is_blocked" json:"is_blocked"`
	IsFavorite   bool           `gorm:"default:false;column:is_favorite" json:"is_favorite"`
	IsOnline     bool           `gorm:"default:false;column:is_online" json:"is_online"`
	LastSeen     *time.Time     `gorm:"column:last_seen" json:"last_seen"`
	PushName     string         `gorm:"column:push_name" json:"push_name"`
	BusinessInfo string         `gorm:"column:business_info" json:"business_info"`
	CreatedAt    time.Time      `gorm:"column:created_at" json:"created_at"`
	UpdatedAt    time.Time      `gorm:"column:updated_at" json:"updated_at"`
	DeletedAt    gorm.DeletedAt `gorm:"index;column:deleted_at" json:"deleted_at,omitempty"`

	// Relationships
	User model.Admin `gorm:"foreignKey:UserID"`
}

// WAMessageDeliveryStatus represents delivery status of messages
type WAMessageDeliveryStatus struct {
	ID           uint           `gorm:"primaryKey;column:id" json:"id"`
	MessageID    uint           `gorm:"not null;index;column:message_id" json:"message_id"`
	UserID       uint           `gorm:"not null;index;column:user_id" json:"user_id"`
	RecipientJID string         `gorm:"not null;column:recipient_jid" json:"recipient_jid"`
	Status       string         `gorm:"not null;column:status" json:"status"`
	StatusTime   time.Time      `gorm:"not null;column:status_time" json:"status_time"`
	CreatedAt    time.Time      `gorm:"column:created_at" json:"created_at"`
	UpdatedAt    time.Time      `gorm:"column:updated_at" json:"updated_at"`
	DeletedAt    gorm.DeletedAt `gorm:"index;column:deleted_at" json:"deleted_at,omitempty"`

	// Relationships
	Message WAChatMessage `gorm:"foreignKey:MessageID"`
	User    model.Admin   `gorm:"foreignKey:UserID"`
}

// WAMediaFile represents uploaded media files
type WAMediaFile struct {
	ID            uint           `gorm:"primaryKey;column:id" json:"id"`
	UserID        uint           `gorm:"not null;index;column:user_id" json:"user_id"`
	MessageID     uint           `gorm:"index;column:message_id" json:"message_id"`
	FileName      string         `gorm:"not null;column:filename" json:"filename"`
	FilePath      string         `gorm:"not null;column:file_path" json:"file_path"`
	FileURL       string         `gorm:"column:file_url" json:"file_url"`
	MimeType      string         `gorm:"column:mime_type" json:"mime_type"`
	FileSize      int64          `gorm:"column:file_size" json:"file_size"`
	ThumbnailPath string         `gorm:"column:thumbnail_path" json:"thumbnail_path"`
	ThumbnailURL  string         `gorm:"column:thumbnail_url" json:"thumbnail_url"`
	Width         int            `gorm:"column:width" json:"width"`
	Height        int            `gorm:"column:height" json:"height"`
	Duration      int            `gorm:"column:duration" json:"duration"`
	IsUploaded    bool           `gorm:"default:false;column:is_uploaded" json:"is_uploaded"`
	UploadedAt    *time.Time     `gorm:"column:uploaded_at" json:"uploaded_at"`
	CreatedAt     time.Time      `gorm:"column:created_at" json:"created_at"`
	UpdatedAt     time.Time      `gorm:"column:updated_at" json:"updated_at"`
	DeletedAt     gorm.DeletedAt `gorm:"index;column:deleted_at" json:"deleted_at,omitempty"`

	// Relationships
	User    model.Admin   `gorm:"foreignKey:UserID"`
	Message WAChatMessage `gorm:"foreignKey:MessageID"`
}

// Table name methods
func (WAConversation) TableName() string {
	return config.GetConfig().Whatsmeow.WhatsappModel.TBConversation
}

func (WAChatMessage) TableName() string {
	return config.GetConfig().Whatsmeow.WhatsappModel.TBChatMessage
}

func (WAGroupParticipant) TableName() string {
	return config.GetConfig().Whatsmeow.WhatsappModel.TBGroupParticipant
}

func (WAContactInfo) TableName() string {
	return config.GetConfig().Whatsmeow.WhatsappModel.TBContactInfo
}

func (WAMessageDeliveryStatus) TableName() string {
	return config.GetConfig().Whatsmeow.WhatsappModel.TBMessageDeliveryStatus
}

func (WAMediaFile) TableName() string {
	return config.GetConfig().Whatsmeow.WhatsappModel.TBMediaFile
}
