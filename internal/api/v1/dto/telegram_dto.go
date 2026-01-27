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

// SendTelegramVoiceRequest represents the request body for sending a voice message
type SendTelegramVoiceRequest struct {
	// Chat ID (recipient identifier, can be username or chat ID)
	ChatID string `json:"chat_id" binding:"required" example:"@username or 123456789"`
	// Voice file to send (URL, file_id, or file path)
	Voice string `json:"voice" binding:"required" example:"https://example.com/voice.ogg"`
	// Caption for the voice message (optional)
	Caption string `json:"caption" example:"Voice message"`
	// Parse mode for the caption (optional: Markdown, HTML)
	ParseMode string `json:"parse_mode" example:"Markdown"`
	// Duration of the voice message in seconds (optional)
	Duration int32 `json:"duration" example:"10"`
}

// SendTelegramDocumentRequest represents the request body for sending a document
type SendTelegramDocumentRequest struct {
	// Chat ID (recipient identifier, can be username or chat ID)
	ChatID string `json:"chat_id" binding:"required" example:"@username or 123456789"`
	// Document file to send (URL, file_id, or file path)
	Document string `json:"document" binding:"required" example:"https://example.com/document.pdf"`
	// Caption for the document (optional)
	Caption string `json:"caption" example:"Document"`
	// Parse mode for the caption (optional: Markdown, HTML)
	ParseMode string `json:"parse_mode" example:"Markdown"`
}

// SendTelegramPhotoRequest represents the request body for sending a photo
type SendTelegramPhotoRequest struct {
	// Chat ID (recipient identifier, can be username or chat ID)
	ChatID string `json:"chat_id" binding:"required" example:"@username or 123456789"`
	// Photo file to send (URL, file_id, or file path)
	Photo string `json:"photo" binding:"required" example:"https://example.com/photo.jpg"`
	// Caption for the photo (optional)
	Caption string `json:"caption" example:"Photo"`
	// Parse mode for the caption (optional: Markdown, HTML)
	ParseMode string `json:"parse_mode" example:"Markdown"`
}

// SendTelegramAudioRequest represents the request body for sending an audio file
type SendTelegramAudioRequest struct {
	// Chat ID (recipient identifier, can be username or chat ID)
	ChatID string `json:"chat_id" binding:"required" example:"@username or 123456789"`
	// Audio file to send (URL, file_id, or file path)
	Audio string `json:"audio" binding:"required" example:"https://example.com/audio.mp3"`
	// Caption for the audio (optional)
	Caption string `json:"caption" example:"Audio"`
	// Parse mode for the caption (optional: Markdown, HTML)
	ParseMode string `json:"parse_mode" example:"Markdown"`
	// Duration of the audio in seconds (optional)
	Duration int32 `json:"duration" example:"180"`
	// Performer of the audio (optional)
	Performer string `json:"performer" example:"Artist Name"`
	// Title of the audio (optional)
	Title string `json:"title" example:"Song Title"`
}

// SendTelegramVideoRequest represents the request body for sending a video
type SendTelegramVideoRequest struct {
	// Chat ID (recipient identifier, can be username or chat ID)
	ChatID string `json:"chat_id" binding:"required" example:"@username or 123456789"`
	// Video file to send (URL, file_id, or file path)
	Video string `json:"video" binding:"required" example:"https://example.com/video.mp4"`
	// Caption for the video (optional)
	Caption string `json:"caption" example:"Video"`
	// Parse mode for the caption (optional: Markdown, HTML)
	ParseMode string `json:"parse_mode" example:"Markdown"`
	// Duration of the video in seconds (optional)
	Duration int32 `json:"duration" example:"60"`
	// Video width (optional)
	Width int32 `json:"width" example:"640"`
	// Video height (optional)
	Height int32 `json:"height" example:"480"`
}
