package whatsnyanmodel

import (
	"service-platform/internal/config"
	"time"
)

// WhatsAppGroup represents a WhatsApp group.
type WhatsAppGroup struct {
	JID               string    `gorm:"primaryKey;column:jid;type:varchar(100)" json:"jid"`
	Name              string    `gorm:"column:name;type:varchar(255)" json:"name"`
	OwnerJID          string    `gorm:"column:owner_jid;type:varchar(100)" json:"owner_jid"`
	Topic             string    `gorm:"column:topic;type:text" json:"topic"`
	TopicSetAt        time.Time `gorm:"column:topic_set_at" json:"topic_set_at"`
	TopicSetBy        string    `gorm:"column:topic_set_by;type:varchar(100)" json:"topic_set_by"`
	LinkedParentJID   string    `gorm:"column:linked_parent_jid;type:varchar(100)" json:"linked_parent_jid"`
	IsDefaultSubGroup bool      `gorm:"column:is_default_sub_group" json:"is_default_sub_group"`
	IsParent          bool      `gorm:"column:is_parent" json:"is_parent"`
	CreatedAt         time.Time `gorm:"column:created_at;autoCreateTime" json:"created_at"`
	UpdatedAt         time.Time `gorm:"column:updated_at;autoUpdateTime" json:"updated_at"`

	Participants []WhatsAppGroupParticipant `gorm:"foreignKey:GroupJID;references:JID;constraint:OnUpdate:CASCADE,OnDelete:CASCADE;" json:"participants"`
}

// TableName overrides the table name used by User to `whatsapp_groups`.
func (WhatsAppGroup) TableName() string {
	return config.GetConfig().Whatsnyan.Tables.TBWhatsnyanGroup
}

// WhatsAppGroupParticipant represents a participant in a WhatsApp group.
type WhatsAppGroupParticipant struct {
	ID                uint      `gorm:"primaryKey;autoIncrement" json:"id"`
	GroupJID          string    `gorm:"column:group_jid;type:varchar(100);index" json:"group_jid"`
	UserJID           string    `gorm:"column:user_jid;type:varchar(100);index" json:"user_jid"`
	LID               string    `gorm:"column:lid;type:varchar(100)" json:"lid"`
	DisplayName       string    `gorm:"column:display_name;type:varchar(255)" json:"display_name"`
	PhoneNumber       string    `gorm:"column:phone_number;type:varchar(50)" json:"phone_number"`
	ProfilePictureURL string    `gorm:"column:profile_picture_url;type:text" json:"profile_picture_url"`
	IsAdmin           bool      `gorm:"column:is_admin" json:"is_admin"`
	IsSuperAdmin      bool      `gorm:"column:is_super_admin" json:"is_super_admin"`
	CreatedAt         time.Time `gorm:"column:created_at;autoCreateTime" json:"created_at"`
	UpdatedAt         time.Time `gorm:"column:updated_at;autoUpdateTime" json:"updated_at"`
}

// TableName overrides the table name used by User to `whatsapp_group_participants`.
func (WhatsAppGroupParticipant) TableName() string {
	return config.GetConfig().Whatsnyan.Tables.TBWhatsnyanGroupParticipant
}
