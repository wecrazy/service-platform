package fun

import (
	"fmt"
	"time"
)

// FormatDurationHumanReadable formats a time.Duration into a human-readable string.
// It displays hours, minutes, and seconds in a concise format.
func FormatDurationHumanReadable(d time.Duration) string {
	hours := int(d.Hours())
	minutes := int(d.Minutes()) % 60
	seconds := int(d.Seconds()) % 60

	if hours > 0 {
		return fmt.Sprintf("%dh %dm", hours, minutes)
	} else if minutes > 0 {
		return fmt.Sprintf("%dm %ds", minutes, seconds)
	}

	return fmt.Sprintf("%ds", seconds)
}
