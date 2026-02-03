package fun

import (
	"regexp"
	"strings"
)

// IsValidEmail checks if the provided email has a valid format.
func IsValidEmail(email string) bool {
	email = strings.TrimSpace(email)
	if email == "" {
		return false
	}

	// Very simple regex: covers most common cases, but is not overly strict.
	var emailRegex = regexp.MustCompile(`^[a-zA-Z0-9._%+\-]+@[a-zA-Z0-9.\-]+\.[a-zA-Z]{2,}$`)
	return emailRegex.MatchString(email)
}
