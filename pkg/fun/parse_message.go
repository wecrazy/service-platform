package fun

import (
	"regexp"
	"strconv"
)

// ExtractFirstInteger extracts the first integer found in the message text.
// For example, "get pprof 10 1000" will return 10.
// If no integer is found, returns 0.
func ExtractFirstInteger(message string) int {
	re := regexp.MustCompile(`\d+`)
	match := re.FindString(message)
	if match == "" {
		return 0
	}
	interval, err := strconv.Atoi(match)
	if err != nil {
		return 0
	}
	return interval
}
