package fun

import (
	"fmt"
	"math"
)

// ConvertToDMS converts a decimal degree to DMS format
func ConvertToDMS(decimal float64, isLatitude bool) string {
	degrees := int(math.Floor(decimal))
	minutes := int(math.Floor((decimal - float64(degrees)) * 60))
	seconds := (decimal - float64(degrees) - float64(minutes)/60) * 3600

	var direction string
	if isLatitude {
		if degrees >= 0 {
			direction = "N"
		} else {
			direction = "S"
		}
	} else {
		if degrees >= 0 {
			direction = "E"
		} else {
			direction = "W"
		}
	}

	// Make degrees positive for formatting
	if degrees < 0 {
		degrees = -degrees
	}

	// Format the DMS string
	return fmt.Sprintf("%d°%d'%.1f\"%s", degrees, minutes, seconds, direction)
}
