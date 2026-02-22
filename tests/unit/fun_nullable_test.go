package unit

import (
	"encoding/json"
	"testing"

	"service-platform/internal/pkg/fun"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ── NullAbleString ──────────────────────────────────────────────────────────

func TestNullAbleString_ValidString(t *testing.T) {
	var ns fun.NullAbleString
	require.NoError(t, json.Unmarshal([]byte(`"hello"`), &ns))
	assert.True(t, ns.Valid)
	assert.Equal(t, "hello", ns.String)
}

func TestNullAbleString_Null(t *testing.T) {
	var ns fun.NullAbleString
	require.NoError(t, json.Unmarshal([]byte(`null`), &ns))
	assert.False(t, ns.Valid)
}

func TestNullAbleString_False(t *testing.T) {
	var ns fun.NullAbleString
	require.NoError(t, json.Unmarshal([]byte(`false`), &ns))
	assert.False(t, ns.Valid)
}

// ── NullAbleFloat ───────────────────────────────────────────────────────────

func TestNullAbleFloat_ValidFloat(t *testing.T) {
	var nf fun.NullAbleFloat
	require.NoError(t, json.Unmarshal([]byte(`42.5`), &nf))
	assert.True(t, nf.Valid)
	assert.Equal(t, 42.5, nf.Float)
}

func TestNullAbleFloat_Null(t *testing.T) {
	var nf fun.NullAbleFloat
	require.NoError(t, json.Unmarshal([]byte(`null`), &nf))
	assert.False(t, nf.Valid)
}

// ── NullAbleInteger ─────────────────────────────────────────────────────────

func TestNullAbleInteger_ValidInt(t *testing.T) {
	var ni fun.NullAbleInteger
	require.NoError(t, json.Unmarshal([]byte(`99`), &ni))
	assert.True(t, ni.Valid)
	assert.Equal(t, 99, ni.Int)
}

func TestNullAbleInteger_Null(t *testing.T) {
	var ni fun.NullAbleInteger
	require.NoError(t, json.Unmarshal([]byte(`null`), &ni))
	assert.False(t, ni.Valid)
}

func TestNullAbleInteger_IsEmpty(t *testing.T) {
	assert.False(t, fun.NullAbleInteger{Int: 1, Valid: true}.IsEmpty())
	assert.True(t, fun.NullAbleInteger{Int: 0, Valid: false}.IsEmpty())
}

func TestNullAbleInteger_InvalidJSON(t *testing.T) {
	var ni fun.NullAbleInteger
	assert.Error(t, json.Unmarshal([]byte(`"not_an_int"`), &ni))
}

// ── NullAbleBoolean ─────────────────────────────────────────────────────────

func TestNullAbleBoolean_True(t *testing.T) {
	var nb fun.NullAbleBoolean
	require.NoError(t, json.Unmarshal([]byte(`true`), &nb))
	assert.True(t, nb.Valid)
	assert.True(t, nb.Bool)
}

func TestNullAbleBoolean_False(t *testing.T) {
	var nb fun.NullAbleBoolean
	require.NoError(t, json.Unmarshal([]byte(`false`), &nb))
	assert.True(t, nb.Valid)
	assert.False(t, nb.Bool)
}

func TestNullAbleBoolean_Null(t *testing.T) {
	var nb fun.NullAbleBoolean
	require.NoError(t, json.Unmarshal([]byte(`null`), &nb))
	assert.False(t, nb.Valid)
}

func TestNullAbleBoolean_IsEmpty(t *testing.T) {
	assert.False(t, fun.NullAbleBoolean{Bool: true, Valid: true}.IsEmpty())
	assert.True(t, fun.NullAbleBoolean{Bool: false, Valid: false}.IsEmpty())
}

// ── NullAbleArrayInteger ────────────────────────────────────────────────────

func TestNullAbleArrayInteger_Valid(t *testing.T) {
	var nai fun.NullAbleArrayInteger
	require.NoError(t, json.Unmarshal([]byte(`[1,2,3]`), &nai))
	assert.True(t, nai.Valid)
	assert.Equal(t, []int{1, 2, 3}, nai.Ints)
}

func TestNullAbleArrayInteger_Null(t *testing.T) {
	var nai fun.NullAbleArrayInteger
	require.NoError(t, json.Unmarshal([]byte(`null`), &nai))
	assert.False(t, nai.Valid)
}

func TestNullAbleArrayInteger_IsEmpty(t *testing.T) {
	assert.True(t, fun.NullAbleArrayInteger{Ints: nil, Valid: false}.IsEmpty())
	assert.True(t, fun.NullAbleArrayInteger{Ints: []int{}, Valid: true}.IsEmpty())
	assert.False(t, fun.NullAbleArrayInteger{Ints: []int{1}, Valid: true}.IsEmpty())
}

// ── NullAbleInterface ───────────────────────────────────────────────────────

func TestNullAbleInterface_ValidArray(t *testing.T) {
	var ni fun.NullAbleInterface
	require.NoError(t, json.Unmarshal([]byte(`[1, 2, 3]`), &ni))
	assert.True(t, ni.Valid)
	assert.NotNil(t, ni.Data)
}

func TestNullAbleInterface_Null(t *testing.T) {
	var ni fun.NullAbleInterface
	require.NoError(t, json.Unmarshal([]byte(`null`), &ni))
	assert.False(t, ni.Valid)
	assert.True(t, ni.IsEmpty())
}

func TestNullAbleInterface_ToIntSlice(t *testing.T) {
	ni := fun.NullAbleInterface{
		Data:  []interface{}{float64(1), float64(2), float64(3)},
		Valid: true,
	}
	assert.Equal(t, []int{1, 2, 3}, ni.ToIntSlice())
}

func TestNullAbleInterface_ToIntSlice_Invalid(t *testing.T) {
	ni := fun.NullAbleInterface{Data: nil, Valid: false}
	assert.Empty(t, ni.ToIntSlice())
}

func TestNullAbleInterface_ToIntSlice_NonSlice(t *testing.T) {
	ni := fun.NullAbleInterface{Data: "not a slice", Valid: true}
	assert.Empty(t, ni.ToIntSlice())
}

// ── ParseJSONIDDataCombined ─────────────────────────────────────────────────

func TestParseJSONIDDataCombined_Valid(t *testing.T) {
	ni := fun.NullAbleInterface{
		Data:  []interface{}{float64(42), "hello"},
		Valid: true,
	}
	id, str, err := fun.ParseJSONIDDataCombined(ni)
	require.NoError(t, err)
	assert.Equal(t, 42, id)
	assert.Equal(t, "hello", str)
}

func TestParseJSONIDDataCombined_Empty(t *testing.T) {
	ni := fun.NullAbleInterface{Data: nil, Valid: false}
	id, str, err := fun.ParseJSONIDDataCombined(ni)
	require.NoError(t, err)
	assert.Equal(t, 0, id)
	assert.Equal(t, "", str)
}

func TestParseJSONIDDataCombined_InvalidArray(t *testing.T) {
	ni := fun.NullAbleInterface{Data: "not array", Valid: true}
	_, _, err := fun.ParseJSONIDDataCombined(ni)
	assert.Error(t, err)
}

func TestParseJSONIDDataCombined_ShortArray(t *testing.T) {
	ni := fun.NullAbleInterface{Data: []interface{}{float64(1)}, Valid: true}
	_, _, err := fun.ParseJSONIDDataCombined(ni)
	assert.Error(t, err)
}

func TestParseJSONIDDataCombinedSafe_Valid(t *testing.T) {
	ni := fun.NullAbleInterface{
		Data:  []interface{}{float64(10), "world"},
		Valid: true,
	}
	id, str := fun.ParseJSONIDDataCombinedSafe(ni)
	assert.Equal(t, 10, id)
	assert.Equal(t, "world", str)
}

func TestParseJSONIDDataCombinedSafe_Invalid(t *testing.T) {
	ni := fun.NullAbleInterface{Data: nil, Valid: false}
	id, str := fun.ParseJSONIDDataCombinedSafe(ni)
	assert.Equal(t, 0, id)
	assert.Equal(t, "", str)
}
