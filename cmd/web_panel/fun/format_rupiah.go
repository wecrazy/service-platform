package fun

import (
	"fmt"
	"strconv"
	"strings"
)

// FormatRupiah formats an integer as Indonesian Rupiah, e.g. 1000000 -> "1.000.000"
func FormatRupiah(amount int) string {
	s := fmt.Sprintf("%d", amount)
	n := len(s)
	if n <= 3 {
		return s
	}
	var result []byte
	for i, c := range s {
		if (n-i)%3 == 0 && i != 0 {
			result = append(result, '.')
		}
		result = append(result, byte(c))
	}
	return string(result)
}

// ReturnRupiahFormat formats a string number as Indonesian Rupiah with "Rp. " prefix
// Example: "12000" -> "Rp. 12.000"
// It handles both integer and float strings
func ReturnRupiahFormat(value string) (string, error) {
	// Remove any whitespace
	value = strings.TrimSpace(value)

	// Handle empty string
	if value == "" {
		return "Rp. 0", nil
	}

	// Check if the value contains a decimal point
	var intPart, decPart string
	if strings.Contains(value, ".") {
		parts := strings.Split(value, ".")
		intPart = parts[0]
		if len(parts) > 1 {
			decPart = parts[1]
		}
	} else {
		intPart = value
	}

	// Parse the integer part to validate it's a number
	num, err := strconv.ParseInt(intPart, 10, 64)
	if err != nil {
		return "", fmt.Errorf("invalid number format: %v", err)
	}

	// Format the integer part with thousand separators
	formatted := FormatRupiah(int(num))

	// Add decimal part if exists
	if decPart != "" {
		// Trim trailing zeros for cleaner display (optional)
		decPart = strings.TrimRight(decPart, "0")
		if decPart != "" {
			formatted = formatted + "," + decPart
		}
	}

	return "Rp. " + formatted, nil
}

// ParseRupiah parses various Indonesian Rupiah formats into float64
// Handles formats like "12000", "Rp 12,000", "Rp. 12.000", etc.
// Assumes no decimal places in the input (common for payroll amounts)
func ParseRupiah(value string) (float64, error) {
	// Remove currency symbols and spaces
	value = strings.TrimSpace(value)
	value = strings.TrimPrefix(value, "Rp")
	value = strings.TrimPrefix(value, "Rp.")
	value = strings.TrimSpace(value)

	// Remove thousand separators and keep only digits
	var numStr strings.Builder
	for _, r := range value {
		if r >= '0' && r <= '9' {
			numStr.WriteRune(r)
		}
	}

	if numStr.Len() == 0 {
		return 0, fmt.Errorf("no number found in %s", value)
	}

	return strconv.ParseFloat(numStr.String(), 64)
}
