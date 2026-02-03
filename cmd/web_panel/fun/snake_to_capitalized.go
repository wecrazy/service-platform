package fun

import (
	"strings"
	"unicode"
)

// SnakeToCapitalized converts snake_case to Capitalized Words
func SnakeToCapitalized(s string) string {
	words := strings.Split(s, "_")
	for i, word := range words {
		if len(word) > 0 {
			runes := []rune(word)
			runes[0] = unicode.ToUpper(runes[0])
			words[i] = string(runes)
		}
	}
	return strings.Join(words, " ")
}
