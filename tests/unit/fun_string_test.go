package unit

import (
	"regexp"
	"service-platform/pkg/fun"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ── GenerateRandomString ────────────────────────────────────────────────────

func TestGenerateRandomString_Length(t *testing.T) {
	for _, n := range []int{0, 1, 8, 32, 100} {
		assert.Len(t, fun.GenerateRandomString(n), n)
	}
}

func TestGenerateRandomString_Charset(t *testing.T) {
	re := regexp.MustCompile(`^[a-zA-Z0-9]*$`)
	s := fun.GenerateRandomString(200)
	assert.True(t, re.MatchString(s), "should only contain alphanumeric chars")
}

func TestGenerateRandomString_Unique(t *testing.T) {
	seen := make(map[string]bool, 50)
	for i := 0; i < 50; i++ {
		s := fun.GenerateRandomString(16)
		require.False(t, seen[s], "duplicate on iteration %d", i)
		seen[s] = true
	}
}

// ── GenerateRandomHexaString ────────────────────────────────────────────────

func TestGenerateRandomHexaString(t *testing.T) {
	s := fun.GenerateRandomHexaString(32)
	assert.Len(t, s, 32)
	re := regexp.MustCompile(`^[a-f0-9]+$`)
	assert.True(t, re.MatchString(s))
}

// ── GenerateRandomStringLowerCase ───────────────────────────────────────────

func TestGenerateRandomStringLowerCase(t *testing.T) {
	s := fun.GenerateRandomStringLowerCase(50)
	re := regexp.MustCompile(`^[a-z0-9]+$`)
	assert.True(t, re.MatchString(s))
}

// ── GenerateRandomStringUpperCase ───────────────────────────────────────────

func TestGenerateRandomStringUpperCase(t *testing.T) {
	s := fun.GenerateRandomStringUpperCase(50)
	re := regexp.MustCompile(`^[A-Z0-9]+$`)
	assert.True(t, re.MatchString(s))
}

// ── AddSpaceBeforeUppercase ─────────────────────────────────────────────────

func TestAddSpaceBeforeUppercase(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"HelloWorld", "Hello World"},
		{"helloWorld", "hello World"},
		{"ABC", "A B C"},
		{"hello", "hello"},
		{"", ""},
		{"A", "A"},
		{"MyHTTPServer", "My H T T P Server"},
	}
	for _, tt := range tests {
		assert.Equal(t, tt.want, fun.AddSpaceBeforeUppercase(tt.input))
	}
}

// ── GetSafeString ───────────────────────────────────────────────────────────

func TestGetSafeString(t *testing.T) {
	hello := "hello"
	empty := ""

	assert.Equal(t, "hello", fun.GetSafeString(&hello))
	assert.Equal(t, "", fun.GetSafeString(&empty))
	assert.Equal(t, "", fun.GetSafeString(nil))
}
