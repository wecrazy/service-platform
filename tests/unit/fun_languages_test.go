package unit

import (
	"sort"
	"testing"

	"service-platform/internal/pkg/fun"

	"github.com/stretchr/testify/assert"
)

func TestGetSupportedLanguages(t *testing.T) {
	langs := fun.GetSupportedLanguages()
	assert.Len(t, langs, 10)

	expected := []string{"id", "en", "es", "fr", "de", "pt", "ru", "jp", "cn", "ar"}
	sort.Strings(expected)

	got := make([]string, len(langs))
	copy(got, langs)
	sort.Strings(got)

	assert.Equal(t, expected, got)
}

func TestIsSupportedLanguage(t *testing.T) {
	assert.True(t, fun.IsSupportedLanguage("id"))
	assert.True(t, fun.IsSupportedLanguage("en"))
	assert.True(t, fun.IsSupportedLanguage("jp"))
	assert.False(t, fun.IsSupportedLanguage("ja")) // alias, not canonical
	assert.False(t, fun.IsSupportedLanguage("xx"))
	assert.False(t, fun.IsSupportedLanguage(""))
}

func TestNormalizeLanguageCode(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"ja", "jp"},
		{"zh", "cn"},
		{"zh-CN", "cn"},
		{"en-US", "en"},
		{"in", "id"},
		{"en", "en"},
		{"unknown", "unknown"},
	}
	for _, tt := range tests {
		assert.Equal(t, tt.want, fun.NormalizeLanguageCode(tt.input))
	}
}

func TestDefaultLang(t *testing.T) {
	assert.Equal(t, "id", fun.DefaultLang)
}

func TestLanguageNameMap(t *testing.T) {
	assert.Equal(t, "Bahasa Indonesia", fun.LanguageNameMap["id"])
	assert.Equal(t, "English", fun.LanguageNameMap["en"])
}
