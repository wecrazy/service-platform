package op_model

import (
	"time"

	"gorm.io/gorm"
)

type WAMessage struct {
	ID string `gorm:"primaryKey;type:varchar(255) CHARACTER SET utf8mb4 COLLATE utf8mb4_0900_ai_ci"`

	gorm.Model

	ChatJID       string `gorm:"type:varchar(255) CHARACTER SET utf8mb4 COLLATE utf8mb4_0900_ai_ci"`
	SenderJID     string `gorm:"type:varchar(255) CHARACTER SET utf8mb4 COLLATE utf8mb4_0900_ai_ci"`
	MessageBody   string `gorm:"type:text CHARACTER SET utf8mb4 COLLATE utf8mb4_0900_ai_ci"`
	MessageType   string `gorm:"type:varchar(50) CHARACTER SET utf8mb4 COLLATE utf8mb4_0900_ai_ci"`
	StanzaID      string `gorm:"type:varchar(255) CHARACTER SET utf8mb4 COLLATE utf8mb4_0900_ai_ci"`
	QuotedMsgID   string `gorm:"type:varchar(255) CHARACTER SET utf8mb4 COLLATE utf8mb4_0900_ai_ci"`
	ReplyText     string `gorm:"type:text CHARACTER SET utf8mb4 COLLATE utf8mb4_0900_ai_ci"`
	ReactionEmoji string `gorm:"type:varchar(16) CHARACTER SET utf8mb4 COLLATE utf8mb4_0900_ai_ci"`
	Mentions      string `gorm:"type:text CHARACTER SET utf8mb4 COLLATE utf8mb4_0900_ai_ci"`
	IsGroup       bool
	Status        string `gorm:"type:varchar(50) CHARACTER SET utf8mb4 COLLATE utf8mb4_0900_ai_ci"`

	RepliedBy string `gorm:"type:varchar(255) CHARACTER SET utf8mb4 COLLATE utf8mb4_0900_ai_ci"`
	RepliedAt *time.Time
	ReactedBy string `gorm:"type:varchar(255) CHARACTER SET utf8mb4 COLLATE utf8mb4_0900_ai_ci"`
	ReactedAt *time.Time

	SentAt    time.Time
	CreatedAt time.Time
	UpdatedAt time.Time
}

// TableName sets table name
func (WAMessage) TableName() string {
	return "wa_message"
}

// TableOptions sets default charset/collation
func (WAMessage) TableOptions() string {
	return "CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci"
}
