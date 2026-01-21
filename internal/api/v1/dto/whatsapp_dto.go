package dto

// SendWhatsAppMessageRequest represents the request body for sending a WhatsApp message
type SendWhatsAppMessageRequest struct {
	// Recipient phone number or Group JID
	Recipient string `json:"recipient" binding:"required" example:"6281234567890"`
	// Set to true if the recipient is a group
	IsGroup bool `json:"is_group" example:"false"`
	// Message type: text, image, video, audio, document, location, live_location, poll, contact
	Type string `json:"type" example:"text"`

	// Text content (required for text type)
	Message string `json:"message" example:"Hello World"`

	// Media data in Base64 format (required for image, video, audio, document)
	MediaData string `json:"media_data"`
	// Filename for the media
	Filename string `json:"filename" example:"image.jpg"`
	// Caption for the media
	Caption string `json:"caption" example:"Check this out!"`
	// MimeType of the media
	MimeType string `json:"mime_type" example:"image/jpeg"`
	// ViewOnce for media messages
	ViewOnce bool `json:"view_once" example:"false"`

	// Latitude for location messages
	Latitude float64 `json:"latitude" example:"-6.200000"`
	// Longitude for location messages
	Longitude float64 `json:"longitude" example:"106.816666"`
	// Location name
	LocName string `json:"loc_name" example:"Jakarta"`
	// Address for location
	Address string `json:"address" example:"Jalan Jendral Sudirman"`

	// Accuracy for live location
	Accuracy uint32 `json:"accuracy"`
	// Speed for live location
	Speed float32 `json:"speed"`
	// Degrees for live location
	Degrees uint32 `json:"degrees"`
	// Sequence for live location
	Sequence int64 `json:"sequence"`
	// TimeOffset for live location
	TimeOffset uint32 `json:"time_offset"`

	// Poll Name
	PollName string `json:"poll_name" example:"What is your favorite color?"`
	// Poll Options
	PollOptions []string `json:"poll_options" example:"Red,Blue,Green"`
	// Selectable Count for poll
	SelectableCount uint32 `json:"selectable_count" example:"1"`

	// Contact Name
	ContactName string `json:"contact_name" example:"John Doe"`
	// Vcard data
	Vcard string `json:"vcard"`
}

// CreateWhatsAppStatusRequest represents the request body for creating a WhatsApp status
type CreateWhatsAppStatusRequest struct {
	// Status type: text, image, video, audio
	Type string `json:"type" binding:"required" example:"text"`

	// Text content for text status
	Text string `json:"text" example:"Hello Status"`
	// Background color for text status (Hex code)
	BackgroundColor string `json:"background_color" example:"#FF0000"`
	// Font for text status
	Font int32 `json:"font" example:"1"`

	// Media data in Base64 format
	MediaData string `json:"media_data"`
	// Filename for media status
	Filename string `json:"filename" example:"status.jpg"`
	// Caption for media status
	Caption string `json:"caption" example:"My Status"`
	// MimeType for media status
	MimeType string `json:"mime_type" example:"image/jpeg"`
}

// DisconnectWhatsAppRequest represents the request body for disconnecting WhatsApp
type DisconnectWhatsAppRequest struct {
	// Phone Number to disconnect (optional)
	PhoneNumber string `json:"phone_number" example:"6281234567890"`
}

// CheckWARegisteredRequest represents the query parameters for checking if a phone number is registered on WhatsApp
type CheckWARegisteredRequest struct {
	// Phone number to check
	Phone string `form:"phone" binding:"required" example:"081234567890"`
}
