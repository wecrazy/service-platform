package unit

import (
	"crypto/md5"
	"encoding/hex"
	"service-platform/pkg/fun"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ── InsertStringAtPositions ─────────────────────────────────────────────────

func TestInsertStringAtPositions(t *testing.T) {
	tests := []struct {
		name     string
		original string
		salts    []fun.Salt
		want     string
	}{
		{
			"single insert",
			"abcdef",
			[]fun.Salt{{Salt: "XX", Position: 2}},
			"abXXcdef",
		},
		{
			"multiple inserts",
			"abcdef",
			[]fun.Salt{
				{Salt: "11", Position: 1},
				{Salt: "22", Position: 3},
			},
			"a11bc22def",
		},
		{
			"insert at start",
			"hello",
			[]fun.Salt{{Salt: ">>", Position: 0}},
			">>hello",
		},
	}
	for _, tt := range tests {
		assert.Equal(t, tt.want, fun.InsertStringAtPositions(tt.original, tt.salts...), tt.name)
	}
}

// ── RemoveSubstringAtPositions ──────────────────────────────────────────────

func TestRemoveSubstringAtPositions(t *testing.T) {
	// "abXXcdYYef" → remove 2 chars at pos 2 → "abcdYYef" → remove 2 at pos 4 → "abcdef"
	assert.Equal(t, "abcdef", fun.RemoveSubstringAtPositions("abXXcdYYef", 2, 2, 4))
}

func TestRemoveSubstringAtPositions_Single(t *testing.T) {
	assert.Equal(t, "helloworld", fun.RemoveSubstringAtPositions("helloXXworld", 2, 5))
}

// ── GenerateSaltedPassword / IsPasswordMatched ──────────────────────────────

func TestGenerateSaltedPassword_NotEmpty(t *testing.T) {
	hashed := fun.GenerateSaltedPassword("testpassword123")
	assert.NotEmpty(t, hashed)
	assert.GreaterOrEqual(t, len(hashed), 80) // 16 salt + 64 sha256 + 10 random
}

func TestGenerateSaltedPassword_Empty(t *testing.T) {
	assert.Empty(t, fun.GenerateSaltedPassword(""))
}

func TestGenerateSaltedPassword_DifferentEachTime(t *testing.T) {
	h1 := fun.GenerateSaltedPassword("password123")
	h2 := fun.GenerateSaltedPassword("password123")
	assert.NotEqual(t, h1, h2, "random salts should produce different hashes")
}

func TestIsPasswordMatched_Valid(t *testing.T) {
	pw := "MySecureP@ss123"
	hashed := fun.GenerateSaltedPassword(pw)
	assert.True(t, fun.IsPasswordMatched(pw, hashed))
}

func TestIsPasswordMatched_Invalid(t *testing.T) {
	hashed := fun.GenerateSaltedPassword("correct_password")
	assert.False(t, fun.IsPasswordMatched("wrong_password", hashed))
}

func TestIsPasswordMatched_MultiplePasswords(t *testing.T) {
	// Passwords must be >= 9 chars (InsertStringAtPositions uses position 8)
	passwords := []string{
		"medium-pwd",
		"a-longer-password-here",
		"P@ssw0rd!#$%^&*()",
		"unicode-密码-тест-long",
	}
	for _, pw := range passwords {
		hashed := fun.GenerateSaltedPassword(pw)
		assert.True(t, fun.IsPasswordMatched(pw, hashed), "should match: %s", pw)
		assert.False(t, fun.IsPasswordMatched(pw+"x", hashed), "should NOT match: %s+x", pw)
	}
}

// ── IsPasswordMatchedMd5 ────────────────────────────────────────────────────

func TestIsPasswordMatchedMd5(t *testing.T) {
	password := "testpassword"
	hasher := md5.New()
	hasher.Write([]byte(password))
	md5Hash := hex.EncodeToString(hasher.Sum(nil))

	assert.True(t, fun.IsPasswordMatchedMd5(password, md5Hash))
	assert.False(t, fun.IsPasswordMatchedMd5("wrongpassword", md5Hash))
}

// ── PKCS5Padding / PKCS5UnPadding ───────────────────────────────────────────

func TestPKCS5Padding_Roundtrip(t *testing.T) {
	tests := []struct {
		input     string
		blockSize int
	}{
		{"hello", 16},
		{"exactly16chars!!", 16},
		{"", 16},
		{"a", 8},
		{"abcdefgh", 8},
	}
	for _, tt := range tests {
		padded := fun.PKCS5Padding([]byte(tt.input), tt.blockSize)
		assert.Equal(t, 0, len(padded)%tt.blockSize, "padded should be multiple of block size")
		unpadded, err := fun.PKCS5UnPadding(padded)
		require.NoError(t, err)
		assert.Equal(t, tt.input, string(unpadded))
	}
}

func TestPKCS5UnPadding_Empty(t *testing.T) {
	_, err := fun.PKCS5UnPadding([]byte{})
	assert.Error(t, err)
}

func TestPKCS5UnPadding_InvalidPadding(t *testing.T) {
	data := make([]byte, 16)
	data[15] = 20 // padding larger than data
	_, err := fun.PKCS5UnPadding(data)
	assert.Error(t, err)
}

func TestPKCS5UnPadding_ZeroPadding(t *testing.T) {
	data := make([]byte, 16)
	data[15] = 0
	_, err := fun.PKCS5UnPadding(data)
	assert.Error(t, err)
}
