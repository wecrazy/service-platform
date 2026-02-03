package whatsappmodel

import (
	"encoding/json"
	"fmt"
	"time"
)

// WAConversation helper methods
func (c *WAConversation) MarkAsRead() error {
	c.UnreadCount = 0
	return nil
}

func (c *WAConversation) IncrementUnread() {
	c.UnreadCount++
}

func (c *WAConversation) UpdateLastMessage(message string, timestamp time.Time) {
	c.LastMessage = message
	c.LastMessageTime = &timestamp
}

func (c *WAConversation) IsGroupChat() bool {
	return c.IsGroup
}

func (c *WAConversation) GetDisplayName() string {
	if c.IsGroup && c.GroupSubject != "" {
		return c.GroupSubject
	}
	return c.ContactName
}

// WAChatMessage helper methods
func (m *WAChatMessage) MarkAsDelivered() error {
	m.MessageStatus = "delivered"
	return nil
}

func (m *WAChatMessage) MarkAsRead() error {
	m.MessageStatus = "read"
	return nil
}

func (m *WAChatMessage) MarkAsFailed() error {
	m.MessageStatus = "failed"
	return nil
}

func (m *WAChatMessage) SoftDelete() error {
	m.IsDeleted = true
	return nil
}

func (m *WAChatMessage) ToggleStar() error {
	m.IsStarred = !m.IsStarred
	return nil
}

func (m *WAChatMessage) IsTextMessage() bool {
	return m.MessageType == "text"
}

func (m *WAChatMessage) IsMediaMessage() bool {
	mediaTypes := []string{"image", "video", "audio", "document", "sticker"}
	for _, t := range mediaTypes {
		if m.MessageType == t {
			return true
		}
	}
	return false
}

func (m *WAChatMessage) IsLocationMessage() bool {
	return m.MessageType == "location"
}

func (m *WAChatMessage) IsContactMessage() bool {
	return m.MessageType == "contact"
}

func (m *WAChatMessage) HasQuotedMessage() bool {
	return m.QuotedMessageID != nil
}

func (m *WAChatMessage) GetMentions() []string {
	var mentions []string
	if m.Mentions != "" {
		json.Unmarshal([]byte(m.Mentions), &mentions)
	}
	return mentions
}

func (m *WAChatMessage) SetMentions(mentions []string) error {
	mentionsJson, err := json.Marshal(mentions)
	if err != nil {
		return err
	}
	m.Mentions = string(mentionsJson)
	return nil
}

func (m *WAChatMessage) GetReactions() map[string]interface{} {
	var reactions map[string]interface{}
	if m.Reactions != "" {
		json.Unmarshal([]byte(m.Reactions), &reactions)
	}
	return reactions
}

func (m *WAChatMessage) SetReactions(reactions map[string]interface{}) error {
	reactionsJson, err := json.Marshal(reactions)
	if err != nil {
		return err
	}
	m.Reactions = string(reactionsJson)
	return nil
}

func (m *WAChatMessage) GetFormattedTimestamp() string {
	return m.Timestamp.Format("15:04")
}

func (m *WAChatMessage) GetFormattedDate() string {
	return m.Timestamp.Format("02/01/2006")
}

func (m *WAChatMessage) GetPreviewText(maxLength int) string {
	if m.IsMediaMessage() {
		switch m.MessageType {
		case "image":
			return "📷 Image"
		case "video":
			return "🎥 Video"
		case "audio":
			return "🎵 Audio"
		case "document":
			return "📄 Document"
		case "sticker":
			return "🏷️ Sticker"
		}
	} else if m.IsLocationMessage() {
		return "📍 Location"
	} else if m.IsContactMessage() {
		return "👤 Contact"
	}

	content := m.MessageContent
	if len(content) > maxLength {
		return content[:maxLength] + "..."
	}
	return content
}

// WAGroupParticipant helper methods
func (p *WAGroupParticipant) IsAdmin() bool {
	return p.ParticipantRole == "admin" || p.ParticipantRole == "superadmin"
}

func (p *WAGroupParticipant) IsSuperAdmin() bool {
	return p.ParticipantRole == "superadmin"
}

func (p *WAGroupParticipant) PromoteToAdmin() error {
	p.ParticipantRole = "admin"
	return nil
}

func (p *WAGroupParticipant) DemoteToMember() error {
	p.ParticipantRole = "member"
	return nil
}

func (p *WAGroupParticipant) LeaveGroup() error {
	now := time.Now()
	p.LeftAt = &now
	p.IsActive = false
	return nil
}

func (p *WAGroupParticipant) RejoinGroup() error {
	p.LeftAt = nil
	p.IsActive = true
	return nil
}

// WAContactInfo helper methods
func (c *WAContactInfo) Block() error {
	c.IsBlocked = true
	return nil
}

func (c *WAContactInfo) Unblock() error {
	c.IsBlocked = false
	return nil
}

func (c *WAContactInfo) AddToFavorites() error {
	c.IsFavorite = true
	return nil
}

func (c *WAContactInfo) RemoveFromFavorites() error {
	c.IsFavorite = false
	return nil
}

func (c *WAContactInfo) SetOnlineStatus(isOnline bool) error {
	c.IsOnline = isOnline
	if !isOnline {
		now := time.Now()
		c.LastSeen = &now
	}
	return nil
}

func (c *WAContactInfo) GetFormattedLastSeen() string {
	if c.IsOnline {
		return "Online"
	}
	if c.LastSeen != nil {
		now := time.Now()
		diff := now.Sub(*c.LastSeen)

		if diff < time.Minute {
			return "Last seen just now"
		} else if diff < time.Hour {
			minutes := int(diff.Minutes())
			return fmt.Sprintf("Last seen %d minutes ago", minutes)
		} else if diff < 24*time.Hour {
			hours := int(diff.Hours())
			return fmt.Sprintf("Last seen %d hours ago", hours)
		} else {
			days := int(diff.Hours() / 24)
			return fmt.Sprintf("Last seen %d days ago", days)
		}
	}
	return "Last seen unknown"
}

func (c *WAContactInfo) GetDisplayName() string {
	if c.ContactName != "" {
		return c.ContactName
	}
	if c.PushName != "" {
		return c.PushName
	}
	return c.PhoneNumber
}

// WAMediaFile helper methods
func (m *WAMediaFile) MarkAsUploaded() error {
	m.IsUploaded = true
	now := time.Now()
	m.UploadedAt = &now
	return nil
}

func (m *WAMediaFile) IsImage() bool {
	return m.MimeType != "" && (m.MimeType[:5] == "image")
}

func (m *WAMediaFile) IsVideo() bool {
	return m.MimeType != "" && (m.MimeType[:5] == "video")
}

func (m *WAMediaFile) IsAudio() bool {
	return m.MimeType != "" && (m.MimeType[:5] == "audio")
}

func (m *WAMediaFile) IsDocument() bool {
	return !m.IsImage() && !m.IsVideo() && !m.IsAudio()
}

func (m *WAMediaFile) GetFormattedSize() string {
	size := float64(m.FileSize)
	units := []string{"B", "KB", "MB", "GB"}

	for i, unit := range units {
		if size < 1024.0 || i == len(units)-1 {
			return fmt.Sprintf("%.1f %s", size, unit)
		}
		size /= 1024.0
	}
	return fmt.Sprintf("%.1f GB", size)
}

func (m *WAMediaFile) GetFormattedDuration() string {
	if m.Duration <= 0 {
		return ""
	}

	hours := m.Duration / 3600
	minutes := (m.Duration % 3600) / 60
	seconds := m.Duration % 60

	if hours > 0 {
		return fmt.Sprintf("%02d:%02d:%02d", hours, minutes, seconds)
	}
	return fmt.Sprintf("%02d:%02d", minutes, seconds)
}
