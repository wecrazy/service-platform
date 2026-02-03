package controllers

import (
	"strings"
)

// NormalizeJID normalizes WhatsApp JID formats to ensure consistency
// WhatsApp uses different formats:
// - Individual contacts: number@s.whatsapp.net (new format)
// - Individual contacts: number@c.us (old format)
// - Groups: groupid@g.us
func NormalizeJID(jid string) string {
	return normalizeJID(jid)
}

// normalizeJID normalizes WhatsApp JID formats to ensure consistency
// WhatsApp uses different formats:
// - Individual contacts: number@s.whatsapp.net (new format)
// - Individual contacts: number@c.us (old format)
// - Groups: groupid@g.us
func normalizeJID(jid string) string {
	if jid == "" {
		return jid
	}

	// Handle group JIDs - keep as is
	if strings.Contains(jid, "@g.us") {
		return jid
	}

	// Handle individual contact JIDs
	// Convert both @s.whatsapp.net and @c.us to @s.whatsapp.net (current standard)
	if strings.Contains(jid, "@c.us") {
		// Convert old format to new format
		phone := strings.Split(jid, "@")[0]
		// Remove device suffix if present (e.g., 6281802066490:21 -> 6281802066490)
		if strings.Contains(phone, ":") {
			phone = strings.Split(phone, ":")[0]
		}
		return phone + "@s.whatsapp.net"
	}

	if strings.Contains(jid, "@s.whatsapp.net") {
		// Remove device suffix if present (e.g., 6281802066490:21@s.whatsapp.net -> 6281802066490@s.whatsapp.net)
		phone := strings.Split(jid, "@")[0]
		if strings.Contains(phone, ":") {
			phone = strings.Split(phone, ":")[0]
		}
		return phone + "@s.whatsapp.net"
	}

	// If it's just a phone number, add the domain
	if !strings.Contains(jid, "@") {
		return jid + "@s.whatsapp.net"
	}

	return jid
}

// extractPhoneFromJID extracts the phone number from a JID
func extractPhoneFromJID(jid string) string {
	if jid == "" {
		return ""
	}

	parts := strings.Split(jid, "@")
	if len(parts) > 0 {
		phone := parts[0]
		// Remove device suffix if present (e.g., 6281802066490:21 -> 6281802066490)
		if strings.Contains(phone, ":") {
			phone = strings.Split(phone, ":")[0]
		}
		return phone
	}

	return jid
}

// isGroupJID checks if a JID represents a group
func isGroupJID(jid string) bool {
	return strings.Contains(jid, "@g.us")
}

// formatPhoneDisplay formats a phone number for display
func formatPhoneDisplay(phone string) string {
	if phone == "" {
		return ""
	}

	// Add + prefix if not present
	if !strings.HasPrefix(phone, "+") {
		return "+" + phone
	}

	return phone
}
