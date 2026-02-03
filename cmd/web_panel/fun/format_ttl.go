package fun

import (
	"fmt"
	"time"
)

func FormatTTL(ttl time.Duration) string {
	if ttl <= 0 {
		return "a few moments"
	}
	hours := int(ttl.Hours())
	minutes := int(ttl.Minutes()) % 60

	if hours > 0 {
		return fmt.Sprintf("%dh %dm", hours, minutes)
	}
	return fmt.Sprintf("%dm", minutes)
}

func IsSameDay(a, b time.Time) bool {
	return a.Year() == b.Year() &&
		a.Month() == b.Month() &&
		a.Day() == b.Day()
}
