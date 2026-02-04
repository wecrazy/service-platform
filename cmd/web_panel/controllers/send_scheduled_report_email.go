// Package controllers provides functionality for sending scheduled email reports
//
// This file contains functions to handle automated email sending for MTI reports.
// It supports Excel file validation, different report types with custom email configurations,
// and handles both batch processing and individual report sending.
//
// Usage examples:
//   - SendScheduledReportMTI() - Process all pending reports
//   - SendScheduledReportByID("MTI_Report_VTR") - Send specific report
//
// Supported report types:
//   - MTI_Report_VTR: VTR team reports
//   - MTI_Report_Penarikan: Withdrawal reports
//   - MTI_Report_Pemasangan: Installation reports
//
// Email configurations can be customized by modifying getReportEmailConfig function.
// Make sure to update the email addresses to actual recipients before deployment.

package controllers

import (
	"fmt"
	"os"
	"path/filepath"
	"service-platform/cmd/web_panel/fun"
	"service-platform/cmd/web_panel/internal/gormdb"
	reportmodel "service-platform/cmd/web_panel/model/report_model"
	"service-platform/internal/config"
	"strings"
	"time"

	"github.com/sirupsen/logrus"
	"github.com/xuri/excelize/v2"
)

// ReportEmailConfig holds email configuration for different report types
type ReportEmailConfig struct {
	Subject string
	To      []string
	CC      []string
	BCC     []string
	Body    string
}

// UpdateReportEmailConfig allows updating email configuration for a specific report type
func UpdateReportEmailConfig(reportID string, config ReportEmailConfig) {
	// This could be extended to store configurations in database or config files
	// For now, it's a placeholder for future enhancements
	logrus.Infof("Email config updated for report type: %s", reportID)
}

// getReportEmailConfig returns email configuration based on report ID
func getReportEmailConfig(reportID string) ReportEmailConfig {
	switch reportID {
	case "MTI_Report_VTR":
		var sb strings.Builder

		loc, err := time.LoadLocation(config.WebPanel.Get().Default.Timezone)
		if err != nil {
			logrus.Errorf("Failed to load location: %v", err)
			loc = time.UTC // Fallback to UTC if location loading fails
		}

		now := time.Now().In(loc)
		sb.WriteString(`<mjml>
		<mj-head>
			<mj-title>MTI VTR Report</mj-title>
			<mj-preview> This is a preview of the MTI VTR report email. Please find the attached report for details ..............`)
		sb.WriteString(`</mj-preview>
			<mj-style>
			.report-text {
				font-family: Arial, sans-serif;
				color: #333333;
				line-height: 1.5;
			}
			</mj-style>
		</mj-head>
		<mj-body background-color="#f4f4f4">
			<mj-section background-color="#ffffff" padding="20px">
			<mj-column>
				<mj-text align="center" font-size="20px" font-weight="bold" color="#005288">
				MTI VTR Report
				</mj-text>
				<mj-divider border-color="#cccccc" border-width="1px" />
				<mj-text css-class="report-text" font-size="16px">
				<p>Dear Team,</p>
				<p>Please find attached the <strong>MTI VTR report</strong> for your review.</p>
				<p><strong>Report generated on:</strong> `)
		sb.WriteString(now.Format("January 2, 2006 15:04:05"))
		sb.WriteString(`</p>
				<br><br><p>Best regards,<br/>Service Report</p>
				</mj-text>
			</mj-column>
			</mj-section>
			<mj-section background-color="#e0e0e0" padding="10px">
			<mj-column>
				<mj-text font-size="12px" align="center" color="#555555">
				© `)
		sb.WriteString(now.Format("2006"))
		sb.WriteString(fmt.Sprintf(` %s. All rights reserved.
				</mj-text>
			</mj-column>
			</mj-section>
		</mj-body>
		</mjml>`, config.WebPanel.Get().Default.PT))

		// Set this to your email body
		body := sb.String()

		return ReportEmailConfig{
			Subject: "MTI VTR Report - " + time.Now().Format("2006-01-02"),
			To:      config.WebPanel.Get().Report.MTI.VTR.To,
			CC:      config.WebPanel.Get().Report.MTI.VTR.Cc,
			BCC:     config.WebPanel.Get().Report.MTI.VTR.Bcc,
			Body:    body,
		}
	case "MTI_Report_Penarikan":
		var sb strings.Builder

		loc, err := time.LoadLocation(config.WebPanel.Get().Default.Timezone)
		if err != nil {
			logrus.Errorf("Failed to load location: %v", err)
			loc = time.UTC // Fallback to UTC if location loading fails
		}

		now := time.Now().In(loc)
		sb.WriteString(`<mjml>
		<mj-head>
			<mj-title>MTI Withdrawal Report</mj-title>
			<mj-preview> This is a preview of the MTI Withdrawal report email. Please find the attached report for details ..............`)
		sb.WriteString(`</mj-preview>
			<mj-style>
			.report-text {
				font-family: Arial, sans-serif;
				color: #333333;
				line-height: 1.5;
			}
			</mj-style>
		</mj-head>
		<mj-body background-color="#f4f4f4">
			<mj-section background-color="#ffffff" padding="20px">
			<mj-column>
				<mj-text align="center" font-size="20px" font-weight="bold" color="#005288">
				MTI Withdrawal Report
				</mj-text>
				<mj-divider border-color="#cccccc" border-width="1px" />
				<mj-text css-class="report-text" font-size="16px">
				<p>Dear Team,</p>
				<p>Please find attached the <strong>MTI Withdrawal Report</strong> for your review.</p>
				<p><strong>Report generated on:</strong> `)
		sb.WriteString(now.Format("January 2, 2006 15:04:05"))
		sb.WriteString(`</p>
				<br><br><p>Best regards,<br/>Service Report</p>
				</mj-text>
			</mj-column>
			</mj-section>
			<mj-section background-color="#e0e0e0" padding="10px">
			<mj-column>
				<mj-text font-size="12px" align="center" color="#555555">
				© `)
		sb.WriteString(now.Format("2006"))
		sb.WriteString(fmt.Sprintf(` %s. All rights reserved.
				</mj-text>
			</mj-column>
			</mj-section>
		</mj-body>
		</mjml>`, config.WebPanel.Get().Default.PT))

		// Set this to your email body
		body := sb.String()

		return ReportEmailConfig{
			Subject: "MTI Penarikan Report - " + time.Now().Format("2006-01-02"),
			To:      config.WebPanel.Get().Report.MTI.Penarikan.To,
			CC:      config.WebPanel.Get().Report.MTI.Penarikan.Cc,
			BCC:     config.WebPanel.Get().Report.MTI.Penarikan.Bcc,
			Body:    body,
		}
	case "MTI_Report_Pemasangan":
		var sb strings.Builder

		loc, err := time.LoadLocation(config.WebPanel.Get().Default.Timezone)
		if err != nil {
			logrus.Errorf("Failed to load location: %v", err)
			loc = time.UTC // Fallback to UTC if location loading fails
		}

		now := time.Now().In(loc)
		sb.WriteString(`<mjml>
		<mj-head>
			<mj-title>MTI Installation Report</mj-title>
			<mj-preview> This is a preview of the MTI Installation report email. Please find the attached report for details ..............`)
		sb.WriteString(`</mj-preview>
			<mj-style>
			.report-text {
				font-family: Arial, sans-serif;
				color: #333333;
				line-height: 1.5;
			}
			</mj-style>
		</mj-head>
		<mj-body background-color="#f4f4f4">
			<mj-section background-color="#ffffff" padding="20px">
			<mj-column>
				<mj-text align="center" font-size="20px" font-weight="bold" color="#005288">
				MTI Installation Report
				</mj-text>
				<mj-divider border-color="#cccccc" border-width="1px" />
				<mj-text css-class="report-text" font-size="16px">
				<p>Dear Team,</p>
				<p>Please find attached the <strong>MTI Installation Report</strong> for your review.</p>
				<p><strong>Report generated on:</strong> `)
		sb.WriteString(now.Format("January 2, 2006 15:04:05"))
		sb.WriteString(`</p>
				<br><br><p>Best regards,<br/>Service Report</p>
				</mj-text>
			</mj-column>
			</mj-section>
			<mj-section background-color="#e0e0e0" padding="10px">
			<mj-column>
				<mj-text font-size="12px" align="center" color="#555555">
				© `)
		sb.WriteString(now.Format("2006"))
		sb.WriteString(fmt.Sprintf(` %s. All rights reserved.
				</mj-text>
			</mj-column>
			</mj-section>
		</mj-body>
		</mjml>`, config.WebPanel.Get().Default.PT))

		// Set this to your email body
		body := sb.String()

		return ReportEmailConfig{
			Subject: "MTI Pemasangan Report - " + time.Now().Format("2006-01-02"),
			To:      config.WebPanel.Get().Report.MTI.Pemasangan.To,
			CC:      config.WebPanel.Get().Report.MTI.Pemasangan.Cc,
			BCC:     config.WebPanel.Get().Report.MTI.Pemasangan.Bcc,
			Body:    body,
		}
	default:
		return ReportEmailConfig{
			Subject: "Scheduled Report - " + time.Now().Format("2006-01-02"),
			To:      []string{"admin@company.com"}, // Change this email to actual recipient
			CC:      []string{},
			BCC:     []string{},
			Body: `
				<mjml>
					<mj-body>
						<mj-section>
							<mj-column>
								<mj-text font-size="16px" color="#333">
									<h2>Scheduled Report</h2>
									<p>Please find attached the scheduled report.</p>
									<p>Report generated on: ` + time.Now().Format("January 2, 2006 15:04:05") + `</p>
									<p>Best regards,<br>Automated Report System</p>
								</mj-text>
							</mj-column>
						</mj-section>
					</mj-body>
				</mjml>
			`,
		}
	}
}

// isValidExcelFile checks if the file exists and is a valid Excel file
func isValidExcelFile(filePath string) error {
	// Check if file exists
	if _, err := os.Stat(filePath); os.IsNotExist(err) {
		return fmt.Errorf("file does not exist: %s", filePath)
	}

	// Check file extension
	ext := strings.ToLower(filepath.Ext(filePath))
	if ext != ".xlsx" && ext != ".xls" {
		return fmt.Errorf("file is not an Excel file: %s", filePath)
	}

	// Try to open the Excel file to validate it
	f, err := excelize.OpenFile(filePath)
	if err != nil {
		return fmt.Errorf("failed to open Excel file: %w", err)
	}
	defer f.Close()

	// Check if the file has at least one sheet
	sheets := f.GetSheetList()
	if len(sheets) == 0 {
		return fmt.Errorf("excel file has no sheets: %s", filePath)
	}

	// Try to read at least one cell to ensure file is readable
	firstSheet := sheets[0]
	_, err = f.GetCellValue(firstSheet, "A1")
	if err != nil {
		return fmt.Errorf("excel file is corrupted or unreadable: %w", err)
	}

	return nil
}

// SendScheduledReportMTI processes and sends all pending scheduled reports
func SendScheduledReportMTI() error {
	dbWeb := gormdb.Databases.Web
	var data []reportmodel.ReportScheduled
	if err := dbWeb.Where("already_sent_via_email = ?", false).Find(&data).Error; err != nil {
		logrus.Errorf("Failed to fetch scheduled reports: %v", err)
		return err
	}

	if len(data) == 0 {
		logrus.Info("No pending scheduled reports to send")
		return nil
	}

	logrus.Infof("Found %d pending scheduled reports to process", len(data))

	for _, report := range data {
		// Check ID if contains MTI Report
		if strings.Contains(strings.ToLower(report.ID), "mti_report") {
			if err := processSingleReport(report); err != nil {
				logrus.Errorf("Failed to process report ID %s: %v", report.ID, err)
				// Continue processing other reports even if one fails
				continue
			}
		}

	}

	return nil
}

// SendScheduledReportByID sends a specific report by its ID
func SendScheduledReportByID(reportID string) error {
	var report reportmodel.ReportScheduled
	if err := dbWeb.Where("id = ?", reportID).First(&report).Error; err != nil {
		logrus.Errorf("Failed to find report with ID %s: %v", reportID, err)
		return fmt.Errorf("report not found: %s", reportID)
	}

	logrus.Infof("Manually sending report: %s", reportID)
	return processSingleReport(report)
}

// processSingleReport handles sending a single report via email
func processSingleReport(report reportmodel.ReportScheduled) error {
	logrus.Infof("Processing report: %s, File: %s", report.ID, report.FilePath)

	// Validate Excel file
	if err := isValidExcelFile(report.FilePath); err != nil {
		logrus.Errorf("Excel validation failed for %s: %v", report.FilePath, err)
		return err
	}

	// Get email configuration based on report type
	emailConfig := getReportEmailConfig(report.ID)

	// Prepare attachment
	fileName := filepath.Base(report.FilePath)
	attachments := []fun.EmailAttachment{
		{
			FilePath:    report.FilePath,
			NewFileName: fileName,
		},
	}

	// Send email
	err := fun.TrySendEmail(
		emailConfig.To,
		emailConfig.CC,
		emailConfig.BCC,
		emailConfig.Subject,
		emailConfig.Body,
		attachments,
	)

	if err != nil {
		logrus.Errorf("Failed to send email for report %s: %v", report.ID, err)
		return err
	}

	// Update database to mark as sent
	updateMap := map[string]interface{}{
		"already_sent_via_email": true,
		"updated_at":             time.Now(),
	}

	if err := dbWeb.Model(&reportmodel.ReportScheduled{}).Where("id = ?", report.ID).Updates(updateMap).Error; err != nil {
		logrus.Errorf("Failed to update report status for %s: %v", report.ID, err)
		return err
	}

	// Remove the file after sending
	if err := os.Remove(report.FilePath); err != nil {
		logrus.Errorf("Failed to remove file %s after sending: %v", report.FilePath, err)
		return fmt.Errorf("failed to remove file %s: %w", report.FilePath, err)
	}

	logrus.Infof("Successfully sent report %s to %v", report.ID, emailConfig.To)
	return nil
}
