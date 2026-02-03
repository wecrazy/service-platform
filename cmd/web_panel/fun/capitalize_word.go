package fun

import (
	"strings"
	"unicode"
)

// Capitalize Word returns a string with each word capitalized
func CapitalizeWord(s string) string {
	words := strings.Fields(s) // Split into words
	for i, word := range words {
		if len(word) > 0 {
			// Capitalize first letter
			runes := []rune(word)
			runes[0] = unicode.ToUpper(runes[0])
			// Lowercase the rest if needed
			for j := 1; j < len(runes); j++ {
				runes[j] = unicode.ToLower(runes[j])
			}
			words[i] = string(runes)
		}
	}
	return strings.Join(words, " ")
}
