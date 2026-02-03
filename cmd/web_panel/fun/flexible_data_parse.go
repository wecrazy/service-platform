package fun

import (
	"fmt"
	"strconv"
	"strings"
	"time"
)

// ParseFlexibleDuration parses duration strings like "1s", "5m", "2h", "3d", "1w"
func ParseFlexibleDuration(input string) (time.Duration, error) {
	if len(input) < 2 {
		return 0, fmt.Errorf("invalid duration format: %s", input)
	}

	// Extract number and unit
	numStr := input[:len(input)-1]
	unit := input[len(input)-1:]

	value, err := strconv.Atoi(numStr)
	if err != nil {
		return 0, fmt.Errorf("invalid number in duration: %s", numStr)
	}

	switch strings.ToLower(unit) {
	case "s":
		return time.Duration(value) * time.Second, nil
	case "m":
		return time.Duration(value) * time.Minute, nil
	case "h":
		return time.Duration(value) * time.Hour, nil
	case "d":
		return time.Duration(value) * 24 * time.Hour, nil
	case "w":
		return time.Duration(value) * 7 * 24 * time.Hour, nil
	default:
		return 0, fmt.Errorf("unsupported duration unit: %s", unit)
	}
}

// ParseFlexibleDate supports dates like "2025-01-01", "23 Jul 2025", "23 January 2025", with time parts, and also Excel/Unix epoch numbers.
func ParseFlexibleDate(dateStr string) (time.Time, error) {
	dateStr = strings.TrimSpace(dateStr)

	layouts := []string{
		// Numeric month
		"2006-01-02", // 2025-01-01
		"02/01/2006", // 23/07/2025
		"02.01.2006", // 23.07.2025

		// Short month name
		"02 Jan 2006", // 23 Jul 2025
		"2 Jan 2006",  // 3 Jul 2025
		"02-Jan-06",   // 19-Jan-06 (Excel format)
		"2-Jan-06",    // 9-Jan-06 (Excel format)
		"02-Jan-2006", // 19-Jan-2026
		"2-Jan-2006",  // 9-Jan-2026

		// Full month name
		"02 January 2006", // 23 January 2025
		"2 January 2006",  // 3 January 2025

		// Date + time with seconds (numeric month)
		"2006-01-02 15:04:05",
		"02/01/2006 15:04:05",
		"02.01.2006 15:04:05",

		// Date + time with seconds (short month)
		"02 Jan 2006 15:04:05",
		"2 Jan 2006 15:04:05",

		// Date + time with seconds (full month)
		"02 January 2006 15:04:05",
		"2 January 2006 15:04:05",

		// Date + time without seconds (numeric month)
		"2006-01-02 15:04",
		"02/01/2006 15:04",
		"02.01.2006 15:04",

		// Date + time without seconds (short month)
		"02 Jan 2006 15:04",
		"2 Jan 2006 15:04",

		// Date + time without seconds (full month)
		"02 January 2006 15:04",
		"2 January 2006 15:04",

		// Short date formats
		"01/02/06 15:04",
	}

	// Additional layouts with timezone, short formats, and more variations
	additionalLayouts := []string{
		// Numeric month with timezone
		"2006-01-02 15:04:05 -0700",
		"02/01/2006 15:04:05 -0700",
		"02.01.2006 15:04:05 -0700",
		"2006-01-02 15:04:05 -07:00",
		"02/01/2006 15:04:05 -07:00",
		"02.01.2006 15:04:05 -07:00",

		// Short month with timezone
		"02 Jan 2006 15:04:05 -0700",
		"2 Jan 2006 15:04:05 -0700",
		"02 Jan 2006 15:04:05 -07:00",
		"2 Jan 2006 15:04:05 -07:00",

		// Full month with timezone
		"02 January 2006 15:04:05 -0700",
		"2 January 2006 15:04:05 -07:00",
		"02 January 2006 15:04:05 -07:00",
		"2 January 2006 15:04:05 -0700",

		// Short date formats
		"01-02-06",
		"02-01-06",
		"06-01-02",
		"02/01/06",
		"01/02/06",
		"06/01/02",

		// American format variations
		"01/02/2006",
		"1/2/2006",
		"01-02-2006",
		"1-2-2006",

		// ISO-like variations
		"2006/01/02",
		"2006.01.02",

		// With time but no timezone
		"01/02/2006 15:04:05",
		"1/2/2006 15:04:05",
		"01-02-2006 15:04:05",
		"1-2-2006 15:04:05",
		"2006/01/02 15:04:05",
		"2006.01.02 15:04:05",

		// RFC formats
		time.RFC3339,
		time.RFC3339Nano,
		time.RFC822,
		time.RFC822Z,
		time.RFC850,
		time.RFC1123,
		time.RFC1123Z,

		// Custom common formats
		"2006-01-02T15:04:05",
		"2006-01-02T15:04:05Z",
		"Jan 2, 2006",
		"January 2, 2006",
		"Jan 2 2006",
		"January 2 2006",
		"2 Jan, 2006",
		"2 January, 2006",

		// Time only formats (will use today's date)
		"15:04:05",
		"15:04",
		"3:04 PM",
		"3:04:05 PM",
	}

	// Try original layouts first
	for _, layout := range layouts {
		if t, err := time.Parse(layout, dateStr); err == nil {
			return t, nil
		}
	}

	// If original layouts failed, try additional layouts
	for _, layout := range additionalLayouts {
		if t, err := time.Parse(layout, dateStr); err == nil {
			return t, nil
		}
	}

	// Try to parse as DDMMYYYY if 8 digits
	if len(dateStr) == 8 {
		if t, err := time.Parse("02012006", dateStr); err == nil {
			return t, nil
		}
	}

	// Try to parse as float (Excel serial date)
	if f, err := strconv.ParseFloat(dateStr, 64); err == nil {
		// Excel serial date: days since 1899-12-30
		const excelEpoch = "1899-12-30"
		base, _ := time.Parse("2006-01-02", excelEpoch)
		secs := int64((f - float64(int64(f))) * 24 * 60 * 60)
		t := base.AddDate(0, 0, int(f)).Add(time.Duration(secs) * time.Second)
		return t, nil
	}

	// Try to parse as integer (Unix epoch seconds)
	if i, err := strconv.ParseInt(dateStr, 10, 64); err == nil {
		// Heuristic: treat as unix seconds if > 10^9, else as days since 1970-01-01
		if i > 1e9 {
			return time.Unix(i, 0), nil
		}
	}

	return time.Time{}, fmt.Errorf("unknown date format: %s", dateStr)
}

// ExtractDateOrRange detects date string (single or range).
// Returns startDate, endDate, true if found.
func ExtractDateOrRange(input string) (startDate *time.Time, endDate *time.Time, ok bool) {
	input = strings.TrimSpace(input)

	// try date range first: e.g., "2025-01-01 - 2025-01-02"
	if strings.Contains(input, "-") {
		parts := strings.SplitN(input, "-", 2)
		if len(parts) == 2 {
			startStr := strings.TrimSpace(parts[0])
			endStr := strings.TrimSpace(parts[1])

			start, startErr := ParseFlexibleDate(startStr)
			end, endErr := ParseFlexibleDate(endStr)

			if startErr == nil && endErr == nil {
				return &start, &end, true
			}
		}
	}

	// try single date
	date, err := ParseFlexibleDate(input)
	if err == nil {
		return &date, nil, true
	}

	return nil, nil, false
}
