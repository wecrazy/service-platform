package unit

import (
	"service-platform/pkg/fun"
	"testing"

	"github.com/microcosm-cc/bluemonday"
	"github.com/stretchr/testify/assert"
)

// ── SanitizeString ──────────────────────────────────────────────────────────

func TestSanitizeString(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"hello", "hello"},
		{"<script>alert(1)</script>", "&lt;script&gt;alert(1)&lt;/script&gt;"},
		{"a < b > c", "a &lt; b &gt; c"},
		{"no special chars", "no special chars"},
		{"", ""},
		{"<>", "&lt;&gt;"},
	}
	for _, tt := range tests {
		assert.Equal(t, tt.want, fun.SanitizeString(tt.input))
	}
}

// ── SanitizeCsvString ───────────────────────────────────────────────────────

func TestSanitizeCsvString(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"normal", "normal"},
		{"=cmd|'/C calc'!A0", "'=cmd|'/C calc'!A0"},
		{"+cmd", "'+cmd"},
		{"-cmd", "'-cmd"},
		{"@SUM(A1:A10)", "'@SUM(A1:A10)"},
		{"", ""},
	}
	for _, tt := range tests {
		assert.Equal(t, tt.want, fun.SanitizeCsvString(tt.input))
	}
}

// ── SanitizeJSONStrings ─────────────────────────────────────────────────────

func TestSanitizeJSONStrings_Map(t *testing.T) {
	p := bluemonday.StrictPolicy()
	input := map[string]interface{}{
		"name":  "<b>bold</b>",
		"count": float64(42),
	}

	result := fun.SanitizeJSONStrings(input, p)
	m := result.(map[string]interface{})
	assert.NotEqual(t, "<b>bold</b>", m["name"], "HTML should be sanitized")
	assert.Equal(t, float64(42), m["count"], "non-string should be preserved")
}

func TestSanitizeJSONStrings_NestedArray(t *testing.T) {
	p := bluemonday.StrictPolicy()
	input := []interface{}{
		"<script>evil</script>",
		map[string]interface{}{
			"inner": "<img onerror=alert(1)>",
		},
	}

	result := fun.SanitizeJSONStrings(input, p)
	arr := result.([]interface{})
	assert.NotEqual(t, "<script>evil</script>", arr[0])

	inner := arr[1].(map[string]interface{})["inner"].(string)
	assert.NotEqual(t, "<img onerror=alert(1)>", inner)
}

// ── SanitizeJSONCsvStrings ──────────────────────────────────────────────────

func TestSanitizeJSONCsvStrings_Map(t *testing.T) {
	input := map[string]interface{}{
		"cell1": "=SUM(A1:A10)",
		"cell2": "normal",
		"cell3": "+danger",
	}

	result := fun.SanitizeJSONCsvStrings(input)
	m := result.(map[string]interface{})
	assert.Equal(t, "'=SUM(A1:A10)", m["cell1"])
	assert.Equal(t, "normal", m["cell2"])
	assert.Equal(t, "'+danger", m["cell3"])
}

func TestSanitizeJSONCsvStrings_Array(t *testing.T) {
	input := []interface{}{"@evil", "safe", "-formula"}

	result := fun.SanitizeJSONCsvStrings(input)
	arr := result.([]interface{})
	assert.Equal(t, "'@evil", arr[0])
	assert.Equal(t, "safe", arr[1])
	assert.Equal(t, "'-formula", arr[2])
}
