package fun

import (
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"
)

// MonthToRoman converts various month representations to Roman numerals
func MonthToRoman(month interface{}) (string, error) {
	var monthNum int

	switch m := month.(type) {
	case time.Month:
		monthNum = int(m)
	case string:
		// Handle string representations
		m = strings.ToLower(strings.TrimSpace(m))

		// Check for numeric strings like "01", "02", etc.
		if num, err := strconv.Atoi(m); err == nil {
			if num < 1 || num > 12 {
				return "", errors.New("month number must be between 1-12")
			}
			monthNum = num
			break
		}

		// Check for month names
		monthMap := map[string]int{
			"january": 1, "jan": 1,
			"february": 2, "feb": 2,
			"march": 3, "mar": 3,
			"april": 4, "apr": 4,
			"may":  5,
			"june": 6, "jun": 6,
			"july": 7, "jul": 7,
			"august": 8, "aug": 8,
			"september": 9, "sep": 9, "sept": 9,
			"october": 10, "oct": 10,
			"november": 11, "nov": 11,
			"december": 12, "dec": 12,
		}

		if num, exists := monthMap[m]; exists {
			monthNum = num
		} else {
			return "", fmt.Errorf("invalid month string: %s", m)
		}

	case int:
		if m < 1 || m > 12 {
			return "", errors.New("month number must be between 1-12")
		}
		monthNum = m

	default:
		return "", fmt.Errorf("unsupported type: %T", month)
	}

	// Convert month number to Roman numeral
	romanNumerals := []string{
		"I", "II", "III", "IV", "V", "VI",
		"VII", "VIII", "IX", "X", "XI", "XII",
	}

	return romanNumerals[monthNum-1], nil
}
