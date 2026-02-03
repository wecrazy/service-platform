package fun

import "strings"

// Use https://flagicons.lipis.dev/ free country flags
func GetFlag(langCode string) string {
	if langCode == "" {
		return ""
	}

	switch langCode {
	default:
		return "<span class='shadow-sm fi fi-" + strings.TrimSpace(strings.ToLower(langCode)) + "'></span>"
	}
}
