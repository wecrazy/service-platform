package dto

// SendTelegramMessageRequest represents the request body for sending a Telegram message
type SendTelegramMessageRequest struct {
	// Chat ID (recipient identifier, can be username or chat ID)
	ChatID string `json:"chat_id" binding:"required" example:"@username or 123456789"`
	// Message text to send
	Text string `json:"text" binding:"required" example:"Hello from Telegram bot!"`
	// Parse mode for the message (optional: Markdown, HTML)
	ParseMode string `json:"parse_mode" example:"Markdown"`
}

// SendTelegramMessageWithKeyboardRequest represents the request body for sending a message with inline keyboard
type SendTelegramMessageWithKeyboardRequest struct {
	// Chat ID (recipient identifier, can be username or chat ID)
	ChatID string `json:"chat_id" binding:"required" example:"@username or 123456789"`
	// Message text to send
	Text string `json:"text" binding:"required" example:"Choose an option:"`
	// Parse mode for the message (optional: Markdown, HTML)
	ParseMode string `json:"parse_mode" example:"Markdown"`
	// Inline keyboard markup
	Keyboard *InlineKeyboardMarkup `json:"keyboard"`
}

// EditTelegramMessageRequest represents the request body for editing a Telegram message
type EditTelegramMessageRequest struct {
	// Chat ID (recipient identifier, must be numeric chat ID)
	ChatID string `json:"chat_id" binding:"required" example:"123456789"`
	// Message ID to edit
	MessageID int64 `json:"message_id" binding:"required" example:"123"`
	// New message text
	Text string `json:"text" binding:"required" example:"Updated message text"`
	// Parse mode for the message (optional: Markdown, HTML)
	ParseMode string `json:"parse_mode" example:"Markdown"`
	// Inline keyboard markup (optional)
	Keyboard *InlineKeyboardMarkup `json:"keyboard"`
}

// TelegramAnswerCallbackQueryRequest represents the request body for answering a callback query
type TelegramAnswerCallbackQueryRequest struct {
	// Callback query ID
	CallbackQueryID string `json:"callback_query_id" binding:"required" example:"1234567890123456789"`
	// Text of the notification (optional)
	Text string `json:"text" example:"Button pressed!"`
	// Show alert instead of notification
	ShowAlert bool `json:"show_alert" example:"false"`
}

// InlineKeyboardMarkup represents an inline keyboard markup
type InlineKeyboardMarkup struct {
	// Array of button rows
	InlineKeyboard []InlineKeyboardButtonRow `json:"inline_keyboard"`
}

// InlineKeyboardButtonRow represents a row of inline keyboard buttons
type InlineKeyboardButtonRow struct {
	// Array of buttons in this row
	Buttons []InlineKeyboardButton `json:"buttons"`
}

// InlineKeyboardButton represents an inline keyboard button
type InlineKeyboardButton struct {
	// Label text on the button
	Text string `json:"text" binding:"required" example:"Button 1"`
	// Callback data sent when button is pressed
	CallbackData string `json:"callback_data" example:"button1"`
	// URL to open when button is pressed (optional)
	URL string `json:"url" example:"https://example.com"`
}
