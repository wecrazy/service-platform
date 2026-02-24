package fun

import (
	"math/rand"
	"time"
	"unicode"
)

// GenerateRandomString generates a random alphanumeric string of the specified length.
func GenerateRandomString(charNum int) string {
	const charset = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"

	r := rand.New(rand.NewSource(time.Now().UnixNano()))

	token := make([]byte, charNum)

	for i := range token {
		token[i] = charset[r.Intn(len(charset))]
	}

	return string(token)
}

// GenerateRandomHexaString generates a random hexadecimal string of the specified length.
func GenerateRandomHexaString(charNum int) string {
	const charset = "abcdef0123456789"

	r := rand.New(rand.NewSource(time.Now().UnixNano()))

	token := make([]byte, charNum)

	for i := range token {
		token[i] = charset[r.Intn(len(charset))]
	}

	return string(token)
}

// GenerateRandomStringLowerCase generates a random lowercase alphanumeric string of the specified length.
func GenerateRandomStringLowerCase(charNum int) string {
	const charset = "abcdefghijklmnopqrstuvwxyz0123456789"

	r := rand.New(rand.NewSource(time.Now().UnixNano()))

	token := make([]byte, charNum)

	for i := range token {
		token[i] = charset[r.Intn(len(charset))]
	}

	return string(token)
}

// GenerateRandomStringUpperCase generates a random uppercase alphanumeric string of the specified length.
func GenerateRandomStringUpperCase(charNum int) string {
	const charset = "ABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"

	r := rand.New(rand.NewSource(time.Now().UnixNano()))

	token := make([]byte, charNum)

	for i := range token {
		token[i] = charset[r.Intn(len(charset))]
	}

	return string(token)
}

// AddSpaceBeforeUppercase adds a space before each uppercase letter in the input string, except for the first character.
func AddSpaceBeforeUppercase(s string) string {
	var result []rune
	for i, r := range s {
		if unicode.IsUpper(r) && i != 0 {
			result = append(result, ' ')
		}
		result = append(result, r)
	}
	return string(result)
}

// GetSafeString safely dereferences a string pointer and returns its value.
// If the pointer is nil or points to an empty string, it returns an empty string instead.
func GetSafeString(s *string) string {
	if s != nil && *s != "" {
		return *s
	}
	return ""
}
