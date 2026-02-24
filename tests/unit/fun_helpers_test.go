package unit

import (
	"service-platform/pkg/fun"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

// ── ValidatePassword ────────────────────────────────────────────────────────

func TestValidatePassword_Valid(t *testing.T) {
	valid := []string{
		"Abcdef1234!@",
		"MyP@ssw0rd123",
		"Str0ng!Pass99",
		"C0mpl3x#Pwd!!",
		"aB3$56789012",
	}
	for _, pw := range valid {
		assert.NoError(t, fun.ValidatePassword(pw), "should be valid: %s", pw)
	}
}

func TestValidatePassword_TooShort(t *testing.T) {
	err := fun.ValidatePassword("Ab1!")
	assert.EqualError(t, err, "password must be at least 12 characters long")
}

func TestValidatePassword_NoUppercase(t *testing.T) {
	assert.Error(t, fun.ValidatePassword("abcdef1234!@"))
}

func TestValidatePassword_NoLowercase(t *testing.T) {
	assert.Error(t, fun.ValidatePassword("ABCDEF1234!@"))
}

func TestValidatePassword_NoDigit(t *testing.T) {
	assert.Error(t, fun.ValidatePassword("Abcdef!@#$%^"))
}

func TestValidatePassword_NoSpecial(t *testing.T) {
	assert.Error(t, fun.ValidatePassword("Abcdefgh1234"))
}

func TestValidatePassword_ExactlyMinLength(t *testing.T) {
	assert.NoError(t, fun.ValidatePassword("Abcdefgh12!@"))
}

func TestValidatePassword_Empty(t *testing.T) {
	assert.Error(t, fun.ValidatePassword(""))
}

// ── FormatDurationHumanReadable ─────────────────────────────────────────────

func TestFormatDurationHumanReadable(t *testing.T) {
	tests := []struct {
		input time.Duration
		want  string
	}{
		{0, "0s"},
		{5 * time.Second, "5s"},
		{59 * time.Second, "59s"},
		{60 * time.Second, "1m 0s"},
		{90 * time.Second, "1m 30s"},
		{5*time.Minute + 30*time.Second, "5m 30s"},
		{time.Hour, "1h 0m"},
		{time.Hour + 30*time.Minute, "1h 30m"},
		{2*time.Hour + 45*time.Minute, "2h 45m"},
	}
	for _, tt := range tests {
		assert.Equal(t, tt.want, fun.FormatDurationHumanReadable(tt.input))
	}
}

// ── ExtractFirstInteger ─────────────────────────────────────────────────────

func TestExtractFirstInteger(t *testing.T) {
	tests := []struct {
		input string
		want  int
	}{
		{"get pprof 10 1000", 10},
		{"no numbers here", 0},
		{"", 0},
		{"abc123def456", 123},
		{"42", 42},
		{"test 7 end", 7},
	}
	for _, tt := range tests {
		assert.Equal(t, tt.want, fun.ExtractFirstInteger(tt.input), "input: %s", tt.input)
	}
}
