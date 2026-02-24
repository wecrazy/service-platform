package unit

import (
	"service-platform/pkg/fun"
	"testing"

	"github.com/stretchr/testify/assert"
)

// ── SanitizeIndonesiaPhoneNumber ────────────────────────────────────────────

func TestSanitizeIndonesiaPhoneNumber_Standard(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"08123456789", "8123456789"},
		{"+6281234567890", "81234567890"},
		{"6281234567890", "81234567890"},
		{"81234567890", "81234567890"},
		{"0812-3456-7890", "81234567890"},
		{"0812 3456 7890", "81234567890"},
		{"+62 812 3456 7890", "81234567890"},
	}
	for _, tt := range tests {
		got, err := fun.SanitizeIndonesiaPhoneNumber(tt.input)
		assert.NoError(t, err, "input: %s", tt.input)
		assert.Equal(t, tt.want, got, "input: %s", tt.input)
	}
}

func TestSanitizeIndonesiaPhoneNumber_Multiple(t *testing.T) {
	got, err := fun.SanitizeIndonesiaPhoneNumber("invalid / 08123456789")
	assert.NoError(t, err)
	assert.Equal(t, "8123456789", got)
}

func TestSanitizeIndonesiaPhoneNumber_Invalid(t *testing.T) {
	invalids := []string{
		"",
		"123",
		"+1234567890",
		"notanumber",
		"00001234567",
	}
	for _, input := range invalids {
		_, err := fun.SanitizeIndonesiaPhoneNumber(input)
		assert.Error(t, err, "input: %s", input)
	}
}

func TestSanitizeIndonesiaPhoneNumber_Delimiters(t *testing.T) {
	got, err := fun.SanitizeIndonesiaPhoneNumber("invalid atau 08123456789")
	assert.NoError(t, err)
	assert.Equal(t, "8123456789", got)
}

// ── IsValidIndonesiaPhoneNumber ─────────────────────────────────────────────

func TestIsValidIndonesiaPhoneNumber(t *testing.T) {
	tests := []struct {
		input string
		want  bool
	}{
		{"08123456789", true},
		{"08517320755", true},
		{"08953456789", true},
		{"08771234567", true},
		{"08811234567", true},
		{"02112345678", true},
		{"0123", false},
		{"000", false},
		{"", false},
	}
	for _, tt := range tests {
		assert.Equal(t, tt.want, fun.IsValidIndonesiaPhoneNumber(tt.input), "input: %s", tt.input)
	}
}

// ── SanitizeAllIndonesiaPhoneNumbers ────────────────────────────────────────

func TestSanitizeAllIndonesiaPhoneNumbers(t *testing.T) {
	numbers, err := fun.SanitizeAllIndonesiaPhoneNumbers("08123456789 / 08517320755 / invalid")
	assert.NoError(t, err)
	assert.Len(t, numbers, 2)
}

func TestSanitizeAllIndonesiaPhoneNumbers_NoneValid(t *testing.T) {
	_, err := fun.SanitizeAllIndonesiaPhoneNumbers("invalid / also_invalid")
	assert.Error(t, err)
}
