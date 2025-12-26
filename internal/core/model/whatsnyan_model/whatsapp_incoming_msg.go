package whatsnyanmodel

import (
	"service-platform/internal/config"
	"time"

	"gorm.io/gorm"
)

type WhatsAppIncomingMsg struct {
	gorm.Model

	WhatsappChatID      string    `gorm:"column:whatsapp_chat_id;type:varchar(255);uniqueIndex" json:"whatsapp_chat_id"`
	WhatsappSenderJID   string    `gorm:"column:whatsapp_sender_jid;type:varchar(255)" json:"whatsapp_sender_jid"`
	WhatsappSenderName  string    `gorm:"column:whatsapp_sender_name;type:varchar(255)" json:"whatsapp_sender_name"`
	WhatsappChatJID     string    `gorm:"column:whatsapp_chat_jid;type:varchar(255)" json:"whatsapp_chat_jid"`
	WhatsappMessageBody string    `gorm:"column:whatsapp_message_body;type:text" json:"whatsapp_message_body"`
	WhatsappMessageType string    `gorm:"column:whatsapp_message_type;type:varchar(50)" json:"whatsapp_message_type"`
	WhatsappIsGroup     bool      `gorm:"column:whatsapp_is_group;not null;default:false" json:"whatsapp_is_group"`
	WhatsappReceivedAt  time.Time `gorm:"column:whatsapp_received_at" json:"whatsapp_received_at"`
}

func (WhatsAppIncomingMsg) TableName() string {
	return config.GetConfig().Whatsnyan.Tables.TBWhatsnyanIncomingMessage
}
