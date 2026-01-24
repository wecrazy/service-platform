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
	TelegramStickerMessage  TelegramMessageType = "sticker"
	TelegramLocationMessage TelegramMessageType = "location"
	TelegramContactMessage  TelegramMessageType = "contact"
	// add more as needed
)

// TelegramMsg represents a Telegram message record in the database
type TelegramMsg struct {
	gorm.Model

	TelegramChatID        string              `gorm:"column:telegram_chat_id;type:varchar(255);uniqueIndex:idx_telegram_sent_chat_msg" json:"-"`
	TelegramMessageSentTo string              `gorm:"column:telegram_message_sent_to;type:varchar(255)" json:"telegram_message_sent_to"`
	TelegramSentAt        *time.Time          `gorm:"column:telegram_sent_at" json:"telegram_sent_at"`
	TelegramSenderID      string              `gorm:"column:telegram_sender_id;type:varchar(255)" json:"-"`
	TelegramMessageBody   string              `gorm:"column:telegram_message_body;type:text" json:"telegram_message_body"`
	TelegramMessageType   TelegramMessageType `gorm:"column:telegram_message_type;type:varchar(50)" json:"-"`
	TelegramQuotedMsgID   string              `gorm:"column:telegram_quoted_msg_id;type:varchar(255)" json:"-"`
	TelegramReplyText     string              `gorm:"column:telegram_reply_text;type:text" json:"telegram_reply_text"`
	TelegramRepliedBy     string              `gorm:"column:telegram_replied_by;type:varchar(255)" json:"telegram_replied_by"`
	TelegramRepliedAt     *time.Time          `gorm:"column:telegram_replied_at" json:"telegram_replied_at"`
	TelegramMentions      string              `gorm:"column:telegram_mentions;type:text" json:"telegram_mentions"`
	TelegramIsGroup       bool                `gorm:"column:telegram_is_group;not null;default:false" json:"telegram_is_group"`
	TelegramMsgStatus     string              `gorm:"column:telegram_msg_status;type:varchar(50)" json:"telegram_msg_status"`
	TelegramMessageID     int64               `gorm:"column:telegram_message_id;uniqueIndex:idx_telegram_sent_chat_msg" json:"telegram_message_id"`
	TelegramReactionEmoji string              `gorm:"column:telegram_reaction_emoji;type:varchar(16)" json:"telegram_reaction_emoji"`
	TelegramReactedBy     string              `gorm:"column:telegram_reacted_by;type:varchar(255)" json:"telegram_reacted_by"`
	TelegramReactedAt     *time.Time          `gorm:"column:telegram_reacted_at" json:"telegram_reacted_at"`
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

	TelegramChatID      string              `gorm:"column:telegram_chat_id;type:varchar(255);uniqueIndex:idx_telegram_chat_msg" json:"telegram_chat_id"`
	TelegramSenderID    string              `gorm:"column:telegram_sender_id;type:varchar(255)" json:"telegram_sender_id"`
	TelegramSenderName  string              `gorm:"column:telegram_sender_name;type:varchar(255)" json:"telegram_sender_name"`
	TelegramMessageBody string              `gorm:"column:telegram_message_body;type:text" json:"telegram_message_body"`
	TelegramMessageType TelegramMessageType `gorm:"column:telegram_message_type;type:varchar(50)" json:"telegram_message_type"`
	TelegramIsGroup     bool                `gorm:"column:telegram_is_group;not null;default:false" json:"telegram_is_group"`
	TelegramReceivedAt  time.Time           `gorm:"column:telegram_received_at" json:"telegram_received_at"`
	TelegramMessageID   int64               `gorm:"column:telegram_message_id;uniqueIndex:idx_telegram_chat_msg" json:"telegram_message_id"`
	TelegramReplyToID   *int64              `gorm:"column:telegram_reply_to_id" json:"telegram_reply_to_id"`
	TelegramMsgStatus   string              `gorm:"column:telegram_msg_status;type:varchar(50)" json:"telegram_msg_status"`
	// Note: Telegram bots cannot access user phone numbers due to privacy restrictions
	// Reactions are handled by updating the message record
	TelegramReactionEmoji string     `gorm:"column:telegram_reaction_emoji;type:varchar(16)" json:"telegram_reaction_emoji"`
	TelegramReactedBy     string     `gorm:"column:telegram_reacted_by;type:varchar(255)" json:"telegram_reacted_by"`
	TelegramReactedAt     *time.Time `gorm:"column:telegram_reacted_at" json:"telegram_reacted_at"`
}

func (TelegramIncomingMsg) TableName() string {
	return config.GetConfig().Telegram.Tables.TBTelegramIncomingMessage
}
