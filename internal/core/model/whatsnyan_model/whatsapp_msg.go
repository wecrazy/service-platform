package whatsnyanmodel

import (
	"service-platform/internal/config"
	"time"

	"gorm.io/gorm"
)

type WhatsAppMsg struct {
	gorm.Model

	WhatsappChatID        string     `gorm:"column:whatsapp_chat_id;type:varchar(255);uniqueIndex" json:"-"`
	WhatsappMessageSentTo string     `gorm:"column:whatsapp_message_sent_to;type:varchar(255)" json:"whatsapp_message_sent_to"`
	WhatsappSentAt        *time.Time `gorm:"column:whatsapp_sent_at" json:"whatsapp_sent_at"`
	WhatsappChatJID       string     `gorm:"column:whatsapp_chat_jid;type:varchar(255)" json:"-"`
	WhatsappSenderJID     string     `gorm:"column:whatsapp_sender_jid;type:varchar(255)" json:"-"`
	WhatsappMessageBody   string     `gorm:"column:whatsapp_message_body;type:text" json:"whatsapp_message_body"`
	WhatsappMessageType   string     `gorm:"column:whatsapp_message_type;type:varchar(50)" json:"-"`
	WhatsappQuotedMsgID   string     `gorm:"column:whatsapp_quoted_msg_id;type:varchar(255)" json:"-"`
	WhatsappReplyText     string     `gorm:"column:whatsapp_reply_text;type:text" json:"whatsapp_reply_text"`
	WhatsappRepliedBy     string     `gorm:"column:whatsapp_replied_by;type:varchar(255)" json:"whatsapp_replied_by"`
	WhatsappRepliedAt     *time.Time `gorm:"column:whatsapp_replied_at" json:"whatsapp_replied_at"`
	WhatsappMentions      string     `gorm:"column:whatsapp_mentions;type:text" json:"whatsapp_mentions"`
	WhatsappIsGroup       bool       `gorm:"column:whatsapp_is_group;not null;default:false" json:"whatsapp_is_group"`
	WhatsappMsgStatus     string     `gorm:"column:whatsapp_msg_status;type:varchar(50)" json:"whatsapp_msg_status"`
	WhatsappReactionEmoji string     `gorm:"column:whatsapp_reaction_emoji;type:varchar(16)" json:"whatsapp_reaction_emoji"`
	WhatsappReactedBy     string     `gorm:"column:whatsapp_reacted_by;type:varchar(255)" json:"whatsapp_reacted_by"`
	WhatsappReactedAt     *time.Time `gorm:"column:whatsapp_reacted_at" json:"whatsapp_reacted_at"`
}

func (WhatsAppMsg) TableName() string {
	return config.GetConfig().Whatsnyan.Tables.TBWhatsnyanMessage
}
