package telegrammodel

import (
	"service-platform/internal/config"
	"time"

	"gorm.io/gorm"
)

// TelegramMessageType defines the type of Telegram message
type TelegramMessageType string

const (
	TelegramTextMessage     TelegramMessageType = "text"
	TelegramImageMessage    TelegramMessageType = "image"
	TelegramVideoMessage    TelegramMessageType = "video"
	TelegramDocumentMessage TelegramMessageType = "document"
	TelegramAudioMessage    TelegramMessageType = "audio"
	TelegramVoiceMessage    TelegramMessageType = "voice"
	TelegramStickerMessage  TelegramMessageType = "sticker"
	TelegramLocationMessage TelegramMessageType = "location"
	TelegramContactMessage  TelegramMessageType = "contact"
	// add more as needed
)

// TelegramMsg represents a Telegram message record in the database
type TelegramMsg struct {
	gorm.Model

	ChatID        string              `gorm:"column:telegram_chat_id;type:varchar(255);uniqueIndex:idx_telegram_sent_chat_msg" json:"-"`
	MessageSentTo string              `gorm:"column:telegram_message_sent_to;type:varchar(255)" json:"telegram_message_sent_to"`
	SentAt        *time.Time          `gorm:"column:telegram_sent_at" json:"telegram_sent_at"`
	SenderID      string              `gorm:"column:telegram_sender_id;type:varchar(255)" json:"-"`
	MessageBody   string              `gorm:"column:telegram_message_body;type:text" json:"telegram_message_body"`
	MessageType   TelegramMessageType `gorm:"column:telegram_message_type;type:varchar(50)" json:"-"`
	QuotedMsgID   string              `gorm:"column:telegram_quoted_msg_id;type:varchar(255)" json:"-"`
	ReplyText     string              `gorm:"column:telegram_reply_text;type:text" json:"telegram_reply_text"`
	RepliedBy     string              `gorm:"column:telegram_replied_by;type:varchar(255)" json:"telegram_replied_by"`
	RepliedAt     *time.Time          `gorm:"column:telegram_replied_at" json:"telegram_replied_at"`
	Mentions      string              `gorm:"column:telegram_mentions;type:text" json:"telegram_mentions"`
	IsGroup       bool                `gorm:"column:telegram_is_group;not null;default:false" json:"telegram_is_group"`
	MsgStatus     string              `gorm:"column:telegram_msg_status;type:varchar(50)" json:"telegram_msg_status"`
	MessageID     int64               `gorm:"column:telegram_message_id;uniqueIndex:idx_telegram_sent_chat_msg" json:"telegram_message_id"`
	ReactionEmoji string              `gorm:"column:telegram_reaction_emoji;type:varchar(16)" json:"telegram_reaction_emoji"`
	ReactedBy     string              `gorm:"column:telegram_reacted_by;type:varchar(255)" json:"telegram_reacted_by"`
	ReactedAt     *time.Time          `gorm:"column:telegram_reacted_at" json:"telegram_reacted_at"`
}

func (TelegramMsg) TableName() string {
	return config.GetConfig().Telegram.Tables.TBTelegramMessage
}

// TelegramIncomingMsg represents an incoming Telegram message record in the database
// Note: For Telegram bots, chat_id is the unique identifier for the chat.
// In private chats, chat_id equals the user_id of the other participant.
// In group chats, chat_id is the group ID.
// Message IDs are unique within each chat.
type TelegramIncomingMsg struct {
	gorm.Model

	ChatID      string              `gorm:"column:telegram_chat_id;type:varchar(255);uniqueIndex:idx_telegram_chat_msg" json:"telegram_chat_id"`
	SenderID    string              `gorm:"column:telegram_sender_id;type:varchar(255)" json:"telegram_sender_id"`
	SenderName  string              `gorm:"column:telegram_sender_name;type:varchar(255)" json:"telegram_sender_name"`
	MessageBody string              `gorm:"column:telegram_message_body;type:text" json:"telegram_message_body"`
	MessageType TelegramMessageType `gorm:"column:telegram_message_type;type:varchar(50)" json:"telegram_message_type"`
	IsGroup     bool                `gorm:"column:telegram_is_group;not null;default:false" json:"telegram_is_group"`
	ReceivedAt  time.Time           `gorm:"column:telegram_received_at" json:"telegram_received_at"`
	MessageID   int64               `gorm:"column:telegram_message_id;uniqueIndex:idx_telegram_chat_msg" json:"telegram_message_id"`
	ReplyToID   *int64              `gorm:"column:telegram_reply_to_id" json:"telegram_reply_to_id"`
	MsgStatus   string              `gorm:"column:telegram_msg_status;type:varchar(50)" json:"telegram_msg_status"`
	// Note: Telegram bots cannot access user phone numbers due to privacy restrictions
	// Reactions are handled by updating the message record
	ReactionEmoji string     `gorm:"column:telegram_reaction_emoji;type:varchar(16)" json:"telegram_reaction_emoji"`
	ReactedBy     string     `gorm:"column:telegram_reacted_by;type:varchar(255)" json:"telegram_reacted_by"`
	ReactedAt     *time.Time `gorm:"column:telegram_reacted_at" json:"telegram_reacted_at"`
}

func (TelegramIncomingMsg) TableName() string {
	return config.GetConfig().Telegram.Tables.TBTelegramIncomingMessage
}
