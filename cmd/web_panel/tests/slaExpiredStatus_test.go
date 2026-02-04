package tests

import (
	"fmt"
	"service-platform/internal/config"
	"testing"
	"time"
)

// type nullAbleTime struct {
// 	Time  time.Time
// 	Valid bool // Valid is true if Time is not NULL
// }

// go test -v -timeout 60m ./tests/slaExpiredStatus_test.go
func TestSLAStatusExpired(t *testing.T) {
	slaDeadlines := []string{
		"2025-10-08 23:59:59",
		"2025-10-08 13:59:59",
		"2025-10-08 19:59:59",
		"2025-10-08 17:59:59",
		"2025-10-09 23:59:59",
		"2025-10-09 01:59:59",
		"2025-10-09 00:59:59",
		"2025-10-10 23:59:59",
	}

	for _, deadline := range slaDeadlines {
		parsedTime, err := time.Parse("2006-01-02 15:04:05", deadline)
		if err != nil {
			t.Errorf("Failed to parse deadline %s: %v", deadline, err)
			continue
		}
		slaDeadline := nullAbleTime{Time: parsedTime, Valid: true}
		status := SLAExpired(slaDeadline)
		t.Logf("Deadline: %s, Status: %s", deadline, status)
	}
}

func SLAExpired(slaDeadline nullAbleTime) string {
	// Check if the value is null or invalid
	if !slaDeadline.Valid {
		return "SLA Not Found!"
	}

	loc, _ := time.LoadLocation(config.WebPanel.Get().Default.Timezone)
	// now := time.Now().In(loc)
	now := time.Date(time.Now().Year(), time.Now().Month(), time.Now().Day(), 2, 0, 0, 0, loc)
	slaTime := slaDeadline.Time.In(loc)

	// Truncate times to midnight for date-only comparison
	nowDate := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, loc)
	slaDate := time.Date(slaDeadline.Time.Year(), slaDeadline.Time.Month(), slaDeadline.Time.Day(), 0, 0, 0, 0, slaDeadline.Time.Location())

	// Calculate the difference in days
	daysDiff := int(slaDate.Sub(nowDate).Hours() / 24)

	// Generate the response based on the difference
	switch {
	case daysDiff == 0:
		return fmt.Sprintf("SLA expires today at %s", slaTime.Format("15:04"))
	case daysDiff == 1:
		return "SLA expires tomorrow"
	case daysDiff > 1:
		return fmt.Sprintf("SLA expires in %d days", daysDiff)
	case daysDiff == -1:
		return "SLA expired yesterday"
	default: // daysDiff < -1
		return fmt.Sprintf("SLA expired %d days ago", -daysDiff)
	}
}
