package tests

import (
	"encoding/json"
	"fmt"
	"os"
	"regexp"
	"service-platform/cmd/web_panel/config"
	"service-platform/cmd/web_panel/database"
	reportmodel "service-platform/cmd/web_panel/model/report_model"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/sirupsen/logrus"
	"gorm.io/gorm"
)

type WoRemarkEntry struct {
	Datetime time.Time
	Reason   string
	Message  string
}

type nullAbleTime struct {
	Time  time.Time
	Valid bool
}

type nullAbleString struct {
	String string
	Valid  bool
}

func (ns *nullAbleString) UnmarshalJSON(data []byte) error {
	if string(data) == "null" || string(data) == "false" {
		ns.String = ""
		ns.Valid = false
		return nil
	}

	if err := json.Unmarshal(data, &ns.String); err != nil {
		return err
	}
	ns.Valid = true
	return nil
}

// SLA Status with conditions :
// - PM <= 15th set as Meet SLA
// - Overdue : New & Visited
func setSLAStatus(
	taskCount int,
	SLADeadline nullAbleTime,
	CompleteDatetimeWO nullAbleTime,
	WoRemark, taskType nullAbleString) (string, time.Time, string, string) {
	// Special handling for Preventive Maintenance
	// Special handling for Preventive Maintenance
	if taskType.Valid && strings.ToLower(taskType.String) == "preventive maintenance" {
		loc, _ := time.LoadLocation(config.GetConfig().Default.Timezone)
		now := time.Now().In(loc)

		// If SLA deadline is valid and in the current month and <= 15th
		if SLADeadline.Valid && !SLADeadline.Time.IsZero() {
			slaDeadline := SLADeadline.Time
			if slaDeadline.Year() == now.Year() &&
				slaDeadline.Month() == now.Month() &&
				slaDeadline.Day() <= 15 {
				// PM and SLA deadline <= 15th: always On Target Solved
				if taskCount >= 2 && WoRemark.Valid && WoRemark.String != "" {
					entries, err := parseWoRemark(WoRemark.String)
					if err != nil {
						logrus.Print(err)
						return "On Target Solved", time.Time{}, "", ""
					}
					if len(entries) > 0 {
						firstTask := entries[len(entries)-1]
						// PM with valid WoRemark: On Target Solved
						return "On Target Solved", firstTask.Datetime, firstTask.Reason, firstTask.Message
					}
				}
				// PM without valid WoRemark: On Target Solved
				return "On Target Solved", CompleteDatetimeWO.Time, "", ""
			}
		}
		// If SLA deadline is not in current month or > 15th, proceed with normal logic below
	}

	// Check if SLA deadline is past and no completion: Overdue (New)
	now := time.Now()
	if SLADeadline.Valid && SLADeadline.Time.Before(now) && (!CompleteDatetimeWO.Valid || CompleteDatetimeWO.Time.IsZero()) {
		return "Overdue (New)", time.Time{}, "", ""
	}

	// Main logic for Overdue (New) and Overdue (Visited)
	if taskCount >= 2 {
		if WoRemark.Valid && WoRemark.String != "" {
			entries, err := parseWoRemark(WoRemark.String)
			if err != nil {
				logrus.Print(err)
				// No visit info: Not Visit
				return "Not Visit", time.Time{}, "", ""
			}

			if len(entries) > 0 {
				firstTask := entries[len(entries)-1]

				if SLADeadline.Time.IsZero() || firstTask.Datetime.IsZero() {
					// Missing SLA or first task datetime: Not Visit
					return "Not Visit", time.Time{}, firstTask.Reason, firstTask.Message
				}

				if firstTask.Datetime.Before(SLADeadline.Time) {
					// First task before SLA: On Target Solved
					return "On Target Solved", firstTask.Datetime, firstTask.Reason, firstTask.Message
				} else {
					// Overdue: check if visited or new
					if CompleteDatetimeWO.Valid && !CompleteDatetimeWO.Time.IsZero() {
						// Overdue and has CompleteDatetimeWO: Overdue (Visited)
						return "Overdue (Visited)", firstTask.Datetime, firstTask.Reason, firstTask.Message
					} else {
						// Overdue and no CompleteDatetimeWO: Overdue (New)
						return "Overdue (New)", firstTask.Datetime, firstTask.Reason, firstTask.Message
					}
				}
			} else {
				// No WoRemark entries
				if SLADeadline.Time.IsZero() || CompleteDatetimeWO.Time.IsZero() {
					// Missing SLA or CompleteDatetimeWO: Not Visit
					return "Not Visit", time.Time{}, "", ""
				}

				if CompleteDatetimeWO.Time.Before(SLADeadline.Time) {
					// CompleteDatetimeWO before SLA: On Target Solved
					return "On Target Solved", time.Time{}, "", ""
				} else {
					// Overdue: check if visited or new
					if CompleteDatetimeWO.Valid && !CompleteDatetimeWO.Time.IsZero() {
						// Overdue and has CompleteDatetimeWO: Overdue (Visited)
						return "Overdue (Visited)", time.Time{}, "", ""
					} else {
						// Overdue and no CompleteDatetimeWO: Overdue (New)
						return "Overdue (New)", time.Time{}, "", ""
					}
				}
			}
		} else {
			// No WoRemark
			if SLADeadline.Time.IsZero() || CompleteDatetimeWO.Time.IsZero() {
				// Missing SLA or CompleteDatetimeWO: Not Visit
				return "Not Visit", time.Time{}, "", ""
			}

			if CompleteDatetimeWO.Time.Before(SLADeadline.Time) {
				// CompleteDatetimeWO before SLA: On Target Solved
				return "On Target Solved", time.Time{}, "", ""
			} else {
				// Overdue: check if visited or new
				if CompleteDatetimeWO.Valid && !CompleteDatetimeWO.Time.IsZero() {
					// Overdue and has CompleteDatetimeWO: Overdue (Visited)
					return "Overdue (Visited)", time.Time{}, "", ""
				} else {
					// Overdue and no CompleteDatetimeWO: Overdue (New)
					return "Overdue (New)", time.Time{}, "", ""
				}
			}
		}
	} else {
		// taskCount < 2
		if SLADeadline.Time.IsZero() || CompleteDatetimeWO.Time.IsZero() {
			// Missing SLA or CompleteDatetimeWO: Not Visit
			return "Not Visit", time.Time{}, "", ""
		}

		if CompleteDatetimeWO.Time.Before(SLADeadline.Time) {
			// CompleteDatetimeWO before SLA: On Target Solved
			return "On Target Solved", time.Time{}, "", ""
		} else {
			// Overdue: check if visited or new
			if CompleteDatetimeWO.Valid && !CompleteDatetimeWO.Time.IsZero() {
				// Overdue and has CompleteDatetimeWO: Overdue (Visited)
				return "Overdue (Visited)", time.Time{}, "", ""
			} else {
				// Overdue and no CompleteDatetimeWO: Overdue (New)
				return "Overdue (New)", time.Time{}, "", ""
			}
		}
	}
}

func parseWoRemark(remark string) ([]WoRemarkEntry, error) {
	pattern := `(?s)Technical User on\s+(\d{4}-\d{2}-\d{2} \d{2}:\d{2}:\d{2})\s+:\s+([^,]+),\s+(.*?);`

	re := regexp.MustCompile(pattern)
	matches := re.FindAllStringSubmatch(remark, -1)

	var results []WoRemarkEntry
	loc, _ := time.LoadLocation(config.GetConfig().Default.Timezone) // Change to your local timezone

	for _, match := range matches {
		if len(match) == 4 {
			parsedTime, err := time.ParseInLocation("2006-01-02 15:04:05", match[1], loc)
			if err != nil {
				return nil, fmt.Errorf("failed to parse datetime: %v", err)
			}

			results = append(results, WoRemarkEntry{
				Datetime: parsedTime,
				Reason:   match[2],
				Message:  strings.TrimSpace(match[3]),
			})
		}
	}

	return results, nil
}

func processRecord(db *gorm.DB, tableName string, record reportmodel.MonitoringTicketODOOMS) error {
	// Convert fields to nullAble types
	slaDeadline := nullAbleTime{Valid: record.SLADeadline != nil}
	if record.SLADeadline != nil {
		slaDeadline.Time = *record.SLADeadline
	}

	completeWO := nullAbleTime{Valid: record.CompleteWO != nil}
	if record.CompleteWO != nil {
		completeWO.Time = *record.CompleteWO
	}

	woRemark := nullAbleString{String: record.WORemark, Valid: record.WORemark != ""}

	taskType := nullAbleString{String: record.TaskType, Valid: record.TaskType != ""}

	// Call SetSLAStatus
	updatedSlaStatus, _, _, _ := setSLAStatus(
		record.TaskCount,
		slaDeadline,
		completeWO,
		woRemark,
		taskType,
	)

	// Update the record
	return db.Table(tableName).Where("id = ?", record.ID).Update("sla_status", updatedSlaStatus).Error
}

// go test -v -timeout 60m ./tests/changeSLAStatusOldMonitoringTicket_test.go
func TestChangeSLAStatusOldMonitoringTicket(t *testing.T) {
	// Debug: Check environment
	env := os.Getenv("ENV")
	goEnv := os.Getenv("GO_ENV")
	t.Logf("ENV=%s, GO_ENV=%s", env, goEnv)

	err := config.LoadConfig()
	if err != nil {
		t.Fatalf("Failed to load config: %v", err)
	}

	dbWeb, err := database.InitAndCheckDB(
		config.GetConfig().Database.Username,
		config.GetConfig().Database.Password,
		config.GetConfig().Database.Host,
		config.GetConfig().Database.Port,
		config.GetConfig().Database.Name,
	)
	if err != nil {
		t.Fatalf("Failed to connect to database: %v", err)
	}

	if dbWeb == nil {
		t.Fatal("Database connection is nil")
	}

	tableToGet := "_sep2025"
	tableName := config.GetConfig().Database.TbReportMonitoringTicket + tableToGet

	var dbData []reportmodel.MonitoringTicketODOOMS
	err = dbWeb.Table(tableName).Find(&dbData).Error
	if err != nil {
		t.Fatalf("Failed to query data from table %s: %v", tableName, err)
	}

	if len(dbData) == 0 {
		t.Fatalf("No data found in table %s", tableName)
	}

	// Worker pool for updating
	numWorkers := 10
	jobs := make(chan reportmodel.MonitoringTicketODOOMS, len(dbData))
	results := make(chan error, len(dbData))

	var wg sync.WaitGroup

	// Start workers
	for i := 0; i < numWorkers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for record := range jobs {
				err := processRecord(dbWeb, tableName, record)
				results <- err
			}
		}()
	}

	// Send jobs
	for _, record := range dbData {
		jobs <- record
	}
	close(jobs)

	// Wait for workers
	wg.Wait()
	close(results)

	// Check results
	errorCount := 0
	for err := range results {
		if err != nil {
			t.Errorf("Error updating record: %v", err)
			errorCount++
		}
	}

	t.Logf("Updated %d records, %d errors", len(dbData), errorCount)
}

func TestChangeIdsDataForDebugMonitoringTicket(t *testing.T) {
	// Debug: Check environment
	env := os.Getenv("ENV")
	goEnv := os.Getenv("GO_ENV")
	t.Logf("ENV=%s, GO_ENV=%s", env, goEnv)

	err := config.LoadConfig()
	if err != nil {
		t.Fatalf("Failed to load config: %v", err)
	}

	dbWeb, err := database.InitAndCheckDB(
		config.GetConfig().Database.Username,
		config.GetConfig().Database.Password,
		config.GetConfig().Database.Host,
		config.GetConfig().Database.Port,
		config.GetConfig().Database.Name,
	)
	if err != nil {
		t.Fatalf("Failed to connect to database: %v", err)
	}

	if dbWeb == nil {
		t.Fatal("Database connection is nil")
	}

	tableName := config.GetConfig().Database.TbReportMonitoringTicket

	var dbData []reportmodel.MonitoringTicketODOOMS
	ids := []uint{
		4201953,
	}

	err = dbWeb.Table(tableName).Where("id IN ?", ids).Find(&dbData).Error
	if err != nil {
		t.Fatalf("Failed to query data from table %s: %v", tableName, err)
	}

	if len(dbData) == 0 {
		t.Fatalf("No data found in table %s", tableName)
	}

	for _, record := range dbData {
		loc, _ := time.LoadLocation(config.GetConfig().Default.Timezone)
		slaDeadlineTime, err := time.ParseInLocation("2006-01-02 15:04:05", "2025-10-08 15:33:00", loc)
		if err != nil {
			t.Fatalf("Failed to parse time: %v", err)
		}
		slaDeadline := &slaDeadlineTime

		// Compute SLAStatus using setSLAStatus
		slaDeadlineNull := nullAbleTime{Time: *slaDeadline, Valid: true}
		completeWONull := nullAbleTime{Valid: false} // null
		woRemark := nullAbleString{String: record.WORemark, Valid: record.WORemark != ""}
		taskType := nullAbleString{String: record.TaskType, Valid: record.TaskType != ""}

		updatedSlaStatus, _, _, _ := setSLAStatus(
			record.TaskCount,
			slaDeadlineNull,
			completeWONull,
			woRemark,
			taskType,
		)

		// Update the record in DB
		err = dbWeb.Table(tableName).Where("id = ?", record.ID).Updates(map[string]interface{}{
			"sla_deadline": slaDeadline,
			"complete_wo":  nil,
			"sla_status":   updatedSlaStatus,
		}).Error
		if err != nil {
			t.Errorf("Failed to update record %d: %v", record.ID, err)
		} else {
			t.Logf("Updated record %d: SLADeadline=%v, CompleteWO=nil, SLAStatus=%s", record.ID, slaDeadline, updatedSlaStatus)
		}
	}

}
