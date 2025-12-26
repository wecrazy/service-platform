package fun

import (
	"math/rand"
	"time"
	"unicode"
)

func GenerateRandomString(charNum int) string {
	const charset = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"

	r := rand.New(rand.NewSource(time.Now().UnixNano()))

	token := make([]byte, charNum)

	for i := range token {
		token[i] = charset[r.Intn(len(charset))]
	}

	return string(token)
}

func GenerateRandomHexaString(charNum int) string {
	const charset = "abcdef0123456789"

	r := rand.New(rand.NewSource(time.Now().UnixNano()))

	token := make([]byte, charNum)

	for i := range token {
		token[i] = charset[r.Intn(len(charset))]
	}

	return string(token)
}

func GenerateRandomStringLowerCase(charNum int) string {
	const charset = "abcdefghijklmnopqrstuvwxyz0123456789"

	r := rand.New(rand.NewSource(time.Now().UnixNano()))

	token := make([]byte, charNum)

	for i := range token {
		token[i] = charset[r.Intn(len(charset))]
	}

	return string(token)
}

func GenerateRandomStringUpperCase(charNum int) string {
	const charset = "ABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"

	r := rand.New(rand.NewSource(time.Now().UnixNano()))

	token := make([]byte, charNum)

	for i := range token {
		token[i] = charset[r.Intn(len(charset))]
	}

	return string(token)
}

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
