package fun

import (
	"strings"

	"github.com/microcosm-cc/bluemonday"
)

// SanitizeString replaces special characters in the input string with their corresponding HTML entities to prevent XSS attacks.
func SanitizeString(value string) string {
	var sanitized string
	for _, char := range value {
		switch char {
		case '<':
			sanitized += "&lt;"
		case '>':
			sanitized += "&gt;"
		default:
			sanitized += string(char)
		}
	}
	return sanitized
}

// SanitizeJSONStrings recursively sanitizes all string values in a map
func SanitizeJSONStrings(data interface{}, p *bluemonday.Policy) interface{} {
	switch v := data.(type) {
	case map[string]interface{}:
		for key, value := range v {
			v[key] = SanitizeJSONStrings(value, p)
		}
	case []interface{}:
		for i, value := range v {
			v[i] = SanitizeJSONStrings(value, p)
		}
	case string:
		v = SanitizeString(v)
		return p.Sanitize(v)
	}
	return data
}

// SanitizeCsvString applies a simple prefix strategy to avoid CSV injection
func SanitizeCsvString(value string) string {
	if strings.HasPrefix(value, "=") || strings.HasPrefix(value, "+") || strings.HasPrefix(value, "-") || strings.HasPrefix(value, "@") {
		return "'" + value
	}
	return value
}

// SanitizeJSONCsvStrings recursively sanitizes all string values in a map
func SanitizeJSONCsvStrings(data interface{}) interface{} {
	switch v := data.(type) {
	case map[string]interface{}:
		for key, value := range v {
			v[key] = SanitizeJSONCsvStrings(value)
		}
	case []interface{}:
		for i, value := range v {
			v[i] = SanitizeJSONCsvStrings(value)
		}
	case string:
		return SanitizeCsvString(v)
	}
	return data
}
