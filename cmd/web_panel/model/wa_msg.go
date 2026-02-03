package model

import (
	"service-platform/cmd/web_panel/config"
	"time"

	"gorm.io/gorm"
)

type WAMessage struct {
	gorm.Model

	ID            string     `gorm:"column:id;primaryKey;type:varchar(255)" json:"id"`
	SentAt        time.Time  `gorm:"column:sent_at;type:timestamp;default:CURRENT_TIMESTAMP" json:"sent_at"`
	ChatJID       string     `gorm:"column:chat_jid;type:varchar(255)" json:"chat_jid"`
	SenderJID     string     `gorm:"column:sender_jid;type:varchar(255)" json:"sender_jid"`
	MessageBody   string     `gorm:"column:message_body;type:text" json:"message_body"`
	MessageType   string     `gorm:"column:message_type;type:varchar(50)" json:"message_type"`
	QuotedMsgID   string     `gorm:"column:quoted_msg_id;type:varchar(255)" json:"quoted_msg_id"`
	ReplyText     string     `gorm:"column:reply_text;type:text" json:"reply_text"`
	ReactionEmoji string     `gorm:"column:reaction_emoji;type:varchar(16)" json:"reaction_emoji"`
	Mentions      string     `gorm:"column:mentions;type:text" json:"mentions"`
	IsGroup       bool       `gorm:"column:is_group" json:"is_group"`
	Status        string     `gorm:"column:status;type:varchar(50)" json:"status"`
	RepliedBy     string     `gorm:"column:replied_by;type:varchar(255)" json:"replied_by"`
	RepliedAt     *time.Time `gorm:"column:replied_at" json:"replied_at"`
	ReactedBy     string     `gorm:"column:reacted_by;type:varchar(255)" json:"reacted_by"`
	ReactedAt     *time.Time `gorm:"column:reacted_at" json:"reacted_at"`
}

// TableName sets table name
func (WAMessage) TableName() string {
	return config.GetConfig().Database.TbWAMsg
}
