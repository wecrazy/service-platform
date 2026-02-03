package fun

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

func ConvertStringToFloat64(str string) float64 {
	result, err := strconv.ParseFloat(str, 64)
	if err != nil {
		return 0.0
	}
	return result
}

func SanitizeCurrency(value string) (float64, error) {
	// Remove "Rp." and "Rp"
	value = strings.ReplaceAll(value, "Rp.", "")
	value = strings.ReplaceAll(value, "Rp", "")
	// Trim spaces
	value = strings.TrimSpace(value)
	// Find the first number pattern (including negative and decimal)
	re := regexp.MustCompile(`-?\d+(\.\d+)?`)
	match := re.FindString(value)
	if match == "" {
		return 0.0, fmt.Errorf("no valid number found in input")
	}
	result, err := strconv.ParseFloat(match, 64)
	if err != nil {
		return 0.0, err
	}
	return result, nil
}
