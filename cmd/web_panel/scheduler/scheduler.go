package scheduler

import (
	"fmt"
	"os"
	"path/filepath"
	"service-platform/cmd/web_panel/config"
	"service-platform/cmd/web_panel/controllers"
	"service-platform/cmd/web_panel/database"
	"service-platform/cmd/web_panel/fun"
	"service-platform/cmd/web_panel/internal/gormdb"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/go-co-op/gocron"
	"github.com/sirupsen/logrus"
	"github.com/xuri/excelize/v2"
	"go.mau.fi/whatsmeow/types"
	"gorm.io/gorm"
)

var jakartaLoc *time.Location

// loadTimezone loads the timezone from config
func loadTimezone() {
	var err error
	jakartaLoc, err = time.LoadLocation(config.GetConfig().Default.Timezone)
	if err != nil {
		logrus.Fatalf("Failed to load timezone %s: %v", config.GetConfig().Default.Timezone, err)
	}
	logrus.Infof("Scheduler timezone loaded: %s", jakartaLoc)
}

// ReloadTimezone reloads the timezone (for config changes)
func ReloadTimezone() {
	loadTimezone()
}

var jobMap = map[string]func(){
	"CheckTechnicianExistsODOOMS": func() {
		db := gormdb.Databases.Web
		controllers.CheckODOOMSTechnicianExists(db)
	},
	"GetTicketTypeHommyPayCC": func() {
		db := gormdb.Databases.Web
		controllers.GetTicketTypeHommyPayCC(db)
	},
	"GetTicketStageHommyPayCC": func() {
		db := gormdb.Databases.Web
		controllers.GetTicketStageHommyPayCC(db)
	},
	"GetListTicketHommyPayCC": func() {
		db := gormdb.Databases.Web
		msg, err := controllers.GetListTicketHommyPayCC(db)
		if err != nil {
			logrus.Warnf("Error: %v", err)
			return
		} else {
			logrus.Info(msg)
		}
	},
	"GetMerchantHommyPayCC": func() {
		db := gormdb.Databases.Web
		msg, err := controllers.GetMerchantHommyPayCC(db)
		if err != nil {
			logrus.Warnf("Error: %v", err)
			return
		} else {
			logrus.Info(msg)
		}
	},
	"GetMerchantKresekBag": func() {
		db := gormdb.Databases.Web
		msg, err := controllers.GetMerchantKresekBag(db)
		if err != nil {
			logrus.Warnf("Error: %v", err)
			return
		} else {
			logrus.Info(msg)
		}
	},
	"DumpDatabase": func() {
		if err := database.DumpDatabase(); err != nil {
			logrus.Warnf("Error dumping database: %v", err)
			return
		} else {
			logrus.Info("Database dumped successfully")
		}
	},
	"MrOliverReportStatus": func() {
		originalSenderJIDStr := config.GetConfig().Whatsmeow.WAGTestJID            // e.g., "120363201154381780"
		originalSenderJID := types.NewJID(originalSenderJIDStr, types.GroupServer) // creates "120363201154381780@g.us"
		normalizedJID := controllers.NormalizeSenderJID(originalSenderJID.String())

		reports, err := controllers.StatusReportMrOliver()
		if err != nil {
			logrus.Error(err)
			return
		}

		var sbID strings.Builder
		var sbEN strings.Builder

		sbID.WriteString("Berikut informasi mengenai list report ke Mr. Oliver:\n")
		sbEN.WriteString("Here is the report list to Mr. Oliver:\n")

		for _, report := range reports {
			sbID.WriteString(fmt.Sprintf("\n📌 *%v*\n", report.ReportType))
			sbEN.WriteString(fmt.Sprintf("\n📌 *%v*\n", report.ReportType))

			for _, link := range report.Links {
				if report.CanDownload[link] {
					sbID.WriteString(fmt.Sprintf("✅ Bisa diunduh: `%s`\n", link))
					sbEN.WriteString(fmt.Sprintf("✅ Downloadable: `%s`\n", link))
				} else {
					sbID.WriteString(fmt.Sprintf("❌ Tidak bisa diunduh: `%s`\n", link))
					sbEN.WriteString(fmt.Sprintf("❌ Cannot Download: `%s`\n", link))
				}
			}
		}

		msgID := sbID.String()
		msgEN := sbEN.String()
		controllers.SendLangMessage(normalizedJID, msgID, msgEN, "id")
	},
	"ReportAIError": func() {
		phoneNumbersSend := config.GetConfig().Report.AIError.WhatsappSendToIfGotError
		if len(phoneNumbersSend) == 0 {
			logrus.Warnf("Skipping ReportAIError since no phone numbers configured to send to")
			return
		}

		senderJIDs := controllers.ConvertPhoneNumbersToJIDs(phoneNumbersSend)

		err := controllers.GetDataTicketEngineersProductivity()
		if err != nil {
			logrus.Error(err)
			return
		}

		// Create Excel Report
		loc, err := time.LoadLocation(config.GetConfig().Default.Timezone)
		if err != nil {
			logrus.Error(err)
			return
		}

		now := time.Now().In(loc)
		reportName := fmt.Sprintf("Report_AI_ERROR_%v", now.Format("02Jan2006_15_04_05.xlsx"))
		selectedMainDir, err := fun.FindValidDirectory([]string{
			"web/file/ai_error",
			"../web/file/ai_error",
			"../../web/file/ai_error",
		})
		if err != nil {
			logrus.Error(err)
			return
		}

		fileReportDir := filepath.Join(selectedMainDir, time.Now().Format("2006-01-02"))
		if err := os.MkdirAll(fileReportDir, 0755); err != nil {
			logrus.Error(err)
			return
		}

		excelFilePath, id, en := controllers.GenerateExcelForReportAIError(fileReportDir, reportName)
		if id != "" && en != "" {
			for _, jid := range senderJIDs {
				controllers.SendLangMessage(jid, id, en, "id")
			}
			return
		}

		if excelFilePath == "" {
			id := fmt.Sprintf("⚠ Mohon maaf, gagal untuk generate report %s", reportName)
			en := fmt.Sprintf("⚠ Sorry, failed to generate Report %s", reportName)
			for _, jid := range senderJIDs {
				controllers.SendLangMessage(jid, id, en, "id")
			}
			return
		}

		// Generate the proxy link for the report
		proxyLink := config.GetConfig().App.WebPublicURL + "/report-ai-error/" + filepath.Base(fileReportDir) + "/" + filepath.Base(excelFilePath)

		var sb strings.Builder
		sb.WriteString("<mjml>")
		sb.WriteString(`
		<mj-head>
			<mj-preview>Report AI Error ...</mj-preview>
			<mj-style inline="inline">
			.body-section {
				background-color: #ffffff;
				padding: 30px;
				border-radius: 12px;
				box-shadow: 0 2px 8px rgba(0, 0, 0, 0.08);
			}
			.footer-text {
				color: #6b7280;
				font-size: 12px;
				text-align: center;
				padding-top: 10px;
				border-top: 1px solid #e5e7eb;
			}
			.header-title {
				font-size: 66px;
				font-weight: bold;
				color: #1E293B;
				text-align: left;
			}
			.cta-button {
				background-color: #6D28D9;
				color: #ffffff;
				padding: 12px 24px;
				border-radius: 8px;
				font-size: 16px;
				font-weight: bold;
				text-align: center;
				display: inline-block;
			}
			.email-info {
				color: #374151;
				font-size: 16px;
			}
			</mj-style>
		</mj-head>`)

		sb.WriteString(fmt.Sprintf(`
		<mj-body background-color="#f8fafc">
			<!-- Main Content -->
			<mj-section css-class="body-section" padding="20px">
			<mj-column>
				<mj-text font-size="20px" color="#1E293B" font-weight="bold">Dear ALL,</mj-text>
				<mj-text font-size="16px" color="#4B5563" line-height="1.6">
				With this email, we would like to inform you that the AI Error Report has been successfully generated.<br>
				You can download the report from the following link:<br>
				<a href="%s" style="color:#6D28D9;font-weight:bold;">Download AI Error Report</a>
				</mj-text>

				<mj-divider border-color="#e5e7eb"></mj-divider>

				<mj-text font-size="16px" color="#374151">
				Best Regards,<br>
				<b><i>%v</i></b>
				</mj-text>
			</mj-column>
			</mj-section>

			<!-- Footer -->
			<mj-section>
			<mj-column>
				<mj-text css-class="footer-text">
				⚠ This is an automated email. Please do not reply directly.
				</mj-text>
				<mj-text font-size="12px" color="#6b7280">
				<b>📞 Technical Support: +%s</b><br>
				<!--
				<br>
				<a href="wa.me/%v">
				📞 Support
				</a>
				-->
				</mj-text>
			</mj-column>
			</mj-section>

		</mj-body>
		`,
			proxyLink,
			config.GetConfig().Default.PT,
			config.GetConfig().Whatsmeow.WaTechnicalSupport,
			"085123456789",
		))
		sb.WriteString("</mjml>")

		mjmlTemplate := sb.String()

		if err := fun.TrySendEmail(
			config.GetConfig().Report.AIError.To,
			config.GetConfig().Report.AIError.Cc,
			config.GetConfig().Report.AIError.Bcc,
			fmt.Sprintf("Report AI Error - %s", now.Format("02 Jan 2006")),
			mjmlTemplate,
			nil, // No attachments
		); err != nil {
			id := fmt.Sprintf("⚠ Mohon maaf, gagal untuk mengirim email report %s : %v", reportName, err)
			en := fmt.Sprintf("⚠ Sorry, failed to send Report %s : %v", reportName, err)
			for _, jid := range senderJIDs {
				controllers.SendLangMessage(jid, id, en, "id")
			}
			return
		}

		// // 🧹 Cleanup: remove excel report
		// if err := os.Remove(excelFilePath); err != nil {
		// 	logrus.Errorf("Failed to remove excel file %s: %v", excelFilePath, err)
		// 	return
		// }
	},
	"ShowStatusVMODOODashboard": func() {
		originalSenderJIDStr := config.GetConfig().Whatsmeow.WAGTestJID            // e.g., "120363201154381780"
		originalSenderJID := types.NewJID(originalSenderJIDStr, types.GroupServer) // creates "120363201154381780@g.us"
		normalizedJID := controllers.NormalizeSenderJID(originalSenderJID.String())

		sshUser := config.GetConfig().VMOdooDashboard.SSHUser
		sshPassword := config.GetConfig().VMOdooDashboard.SSHPwd
		sshAddr := config.GetConfig().VMOdooDashboard.SSHAddr

		client, err := controllers.ConnectSSH(sshUser, sshPassword, sshAddr)
		if err != nil {
			logrus.Error(err)
			return
		}
		defer client.Close()

		status, err := controllers.GetWindowsStatus(client)
		if err != nil {
			id := fmt.Sprintf("❌ Gagal mendapatkan status: %v", err)
			en := fmt.Sprintf("❌ Failed to get status: %v", err)
			controllers.SendLangMessage(normalizedJID, id, en, "id")
			return
		}

		ramGB := float64(status.TotalRAMBytes) / (1024 * 1024 * 1024)
		msgID := fmt.Sprintf("💻 *Total RAM*: *%.2f GB*\n", ramGB)
		msgEN := fmt.Sprintf("💻 *Total RAM*: *%.2f GB*\n", ramGB)

		var totalRAMUsedBytes int64
		for _, p := range status.Processes {
			totalRAMUsedBytes += p.RAMBytes
		}
		totalRAMUsedGB := float64(totalRAMUsedBytes) / (1024 * 1024 * 1024)
		msgID += fmt.Sprintf("\n💽 *Total RAM Terpakai*: *%.2f GB*\n", totalRAMUsedGB)
		msgEN += fmt.Sprintf("\n💽 *Total RAM Used*: *%.2f GB*\n", totalRAMUsedGB)

		// Top 10 by RAM
		sort.Slice(status.Processes, func(i, j int) bool {
			return status.Processes[i].RAMBytes > status.Processes[j].RAMBytes
		})
		msgID += "\n🔝 *10 Proses Teratas (RAM)*:\n"
		msgEN += "\n🔝 *Top 10 Processes (RAM)*:\n"
		for i, p := range status.Processes {
			if i >= 10 {
				break
			}
			ramMB := float64(p.RAMBytes) / (1024 * 1024)
			percent := float64(p.RAMBytes) / float64(status.TotalRAMBytes) * 100
			msgID += fmt.Sprintf("• *%s* (PID: `%s`) — _%.2f MB_ (%.2f%%)\n", p.Name, p.PID, ramMB, percent)
			msgEN += fmt.Sprintf("• *%s* (PID: `%s`) — _%.2f MB_ (%.2f%%)\n", p.Name, p.PID, ramMB, percent)
		}

		var totalCPUPercent float64
		for _, p := range status.Processes {
			totalCPUPercent += p.CPUPercent
		}
		msgID += fmt.Sprintf("\n⚙️ *Total CPU Terpakai (jumlah dari proses)*: *%.2f%%*\n", totalCPUPercent)
		msgEN += fmt.Sprintf("\n⚙️ *Total CPU Used (sum of processes)*: *%.2f%%*\n", totalCPUPercent)

		// Top 10 by CPU
		sort.Slice(status.Processes, func(i, j int) bool {
			return status.Processes[i].CPUPercent > status.Processes[j].CPUPercent
		})
		msgID += "\n🧠 *10 Proses Teratas (CPU %)*:\n"
		msgEN += "\n🧠 *Top 10 Processes (CPU %)*:\n"
		for i, p := range status.Processes {
			if i >= 10 {
				break
			}
			msgID += fmt.Sprintf("• *%s* (PID: `%s`) — _%.2f%%_\n", p.Name, p.PID, p.CPUPercent)
			msgEN += fmt.Sprintf("• *%s* (PID: `%s`) — _%.2f%%_\n", p.Name, p.PID, p.CPUPercent)
		}

		// Network status
		if len(status.Network) > 0 {
			msgID += "\n🌐 *Status Jaringan*:\n"
			msgEN += "\n🌐 *Network Status*:\n"
			for _, ni := range status.Network {
				sentMB := float64(ni.BytesSent) / (1024 * 1024)
				recvMB := float64(ni.BytesReceived) / (1024 * 1024)
				msgID += fmt.Sprintf("• *%s*: Kirim: _%.2f MB_, Terima: _%.2f MB_\n", ni.Name, sentMB, recvMB)
				msgEN += fmt.Sprintf("• *%s*: Sent: _%.2f MB_, Received: _%.2f MB_\n", ni.Name, sentMB, recvMB)
			}
		}

		controllers.SendLangMessage(
			normalizedJID,
			"✅ *Status VM ODOO Dashboard:*\n\n_Untuk restart MySQL, Anda bisa input `restart mysql vm odoo dashboard`_\n\n"+msgID,
			"✅ *VM ODOO Dashboard Status:*\n\n_To restart MySQL, you can input `restart mysql vm odoo dashboard`_\n\n"+msgEN,
			"id",
		)
	},
	"GetHelpdeskTicketFields": func() {
		if err := controllers.GetHelpdeskTicketFields(); err != nil {
			logrus.Error(err)
			return
		}
	},
	"GetProjectTaskFields": func() {
		if err := controllers.GetProjectTaskFields(); err != nil {
			logrus.Error(err)
			return
		}
	},
	"GetODOOMSFSParams": func() {
		if err := controllers.GetODOOMSFSParams(); err != nil {
			logrus.Error(err)
			return
		}
	},
	"GetODOOMSFSParamPayment": func() {
		if err := controllers.GetODOOMSFSParamPayment(); err != nil {
			logrus.Error(err)
			return
		}
	},
	"GetODOOMSInventoryProducts": func() {
		if err := controllers.GetODOOMSInventoryProducts(); err != nil {
			logrus.Error(err)
			return
		}
	},
	"SendScheduledReportMTI": func() {
		if err := controllers.SendScheduledReportMTI(); err != nil {
			logrus.Error(err)
			return
		}
	},
	"StatusSPTechnicianODOO": func() {
		now := time.Now().In(jakartaLoc)
		logrus.Infof("Running StatusSPTechnicianODOO check at %s", now.Format(time.RFC3339))

		if !config.GetConfig().SPTechnician.RunOnWeekends {
			if now.Weekday() == time.Saturday || now.Weekday() == time.Sunday {
				// Skip on weekends
				logrus.Infof("Skipping StatusSPTechnicianODOO on weekend (%s)", now.Weekday())
				return
			}
		}

		if !config.GetConfig().SPTechnician.RunOnHolidays {
			todayData, err := fun.GetLibur("today")
			if err != nil {
				logrus.Errorf("Failed to get holiday info: %v", err)
			}
			if todayData.IsHoliday && len(todayData.HolidayList) > 0 {
				// Skip on public holidays
				logrus.Infof("Skipping StatusSPTechnicianODOO on public holiday (%s)",
					strings.Join(todayData.HolidayList, ", "))
				return
			}
		}

		// if err := controllers.CheckSPTechnician(); err != nil {
		// Refactor: using v2 coz sp now is combination of technicians not visits, not doing so, and had missing data while doing so
		if err := controllers.CheckSPTechnicianV2(); err != nil {
			logrus.Error(err)
			return
		}
	},
	"RepliesReportOfSPTechnicianODOO": func() {
		// TODO: Create function that will get data from table sp_whatsapp_message ..
		// you will filter the data based on the message sent, if its message's pelanggaran contains stock opname it's replied will be processed here
	},
	"SendTechnicianLoginReport": func() {
		err := controllers.SendTechnicianLoginReport()
		if err != nil {
			logrus.Error(err)
			return
		}
	},
	"RegistODOOMSHeadAndSPLToChatbot": func() {
		if err := controllers.RegistODOOMSHeadAndSPLToChatbot(); err != nil {
			logrus.Error(err)
			return
		}
	},
	"RegistODOOMSTechnicianToChatbot": func() {
		if err := controllers.RegistODOOMSTechnicianToChatbot(); err != nil {
			logrus.Error(err)
			return
		}
	},
	"ContractTechnicianODOO": func() {
		// Deprecated: not used soon coz its system will get daily the full data of fs.technician coz it can be refreshed from ODOO MS
		// if err := controllers.CheckAvailableForContractTechnicianODOO(); err != nil {
		if err := controllers.GetDataTechnicianForContractInODOO(); err != nil {
			logrus.Error(err)
			return
		}

		// ADD: sendAllContractsToTechnicians(sendOption, db) each 18 on the month if needed !!!!!!
	},
	"NotifyHRDBeforeContractExpired": func() {
		expiredInDays := config.GetConfig().ContractTechnicianODOO.NotifyHRDBeforeContractExpiredDays
		if expiredInDays <= 0 {
			expiredInDays = 30 // default 30 days
		}
		if err := controllers.NotifyHRDBeforeContractExpired(expiredInDays); err != nil {
			logrus.Error(err)
			return
		}
	},
	"ResetNomorSuratSP": func() {
		if err := controllers.ResetNomorSuratSP(); err != nil {
			logrus.Error(err)
			return
		} else {
			logrus.Info("NomorSuratSP has been reset successfully")
		}
	},
	"GetODOOMSJobGroups": func() {
		if err := controllers.GetJobGroupsODOOMS(); err != nil {
			logrus.Error(err)
			return
		}
	},
	"GetTechnicianODOOMSData": func() {
		if err := controllers.GetDataTechnicianODOOMS(); err != nil {
			logrus.Error(err)
			return
		}

		if err := controllers.GetDataOfTechnicianODOOMSForMonitoring(); err != nil {
			logrus.Error(err)
			return
		}
	},
	"GenerateMonitoringReportData": func() {
		var wg sync.WaitGroup
		wg.Add(2)

		var ticketAchievementReport, loginVisitReport string
		var ticketErr, loginVisitErr error

		// Parallel: generate both reports
		go func() {
			defer wg.Done()
			ticketAchievementReport, ticketErr = controllers.MonitoringTicketODOOMS()
		}()
		go func() {
			defer wg.Done()
			loginVisitReport, loginVisitErr = controllers.MonitoringVisitAndLoginTechnicianODOOMS()
		}()

		wg.Wait()

		if ticketErr != nil {
			logrus.Error(ticketErr)
		}
		if loginVisitErr != nil {
			logrus.Error(loginVisitErr)
		}

		// Remove last sheet of ticketAchievementReport before sending via email
		if ticketAchievementReport != "" {
			func() {
				f, err := excelize.OpenFile(ticketAchievementReport)
				if err != nil {
					logrus.Error(err)
					return
				}
				defer f.Close()
				if err := f.DeleteSheet("CHART"); err != nil {
					logrus.Error(err)
					return
				}
				if err := f.Save(); err != nil {
					logrus.Error(err)
					return
				}
			}()
		}

		// Remove last sheet of loginVisitReport before sending via email
		if loginVisitReport != "" {
			func() {
				f, err := excelize.OpenFile(loginVisitReport)
				if err != nil {
					logrus.Error(err)
					return
				}
				defer f.Close()
				if err := f.DeleteSheet("CHART"); err != nil {
					logrus.Error(err)
					return
				}
				if err := f.Save(); err != nil {
					logrus.Error(err)
					return
				}
			}()
		}

		logrus.Infof("GenerateMonitoringReportData completed: TicketReport=%s (err=%v), LoginVisitReport=%s (err=%v)",
			ticketAchievementReport, ticketErr, loginVisitReport, loginVisitErr)
	},
	"MonitoringReport": func() {
		var phoneNumbersSendTo []string
		phoneNumbersSendTo = append(phoneNumbersSendTo, config.GetConfig().Report.MonitoringTicketODOOMS.WhatsappSendToIfGotError...)
		phoneNumbersSendTo = append(phoneNumbersSendTo, config.GetConfig().Report.MonitoringLoginVisitTechnician.WhatsappSendToIfGotError...)
		// Make unique
		uniqueMap := make(map[string]struct{})
		var uniqueNumbers []string
		for _, num := range phoneNumbersSendTo {
			if _, exists := uniqueMap[num]; !exists {
				uniqueMap[num] = struct{}{}
				uniqueNumbers = append(uniqueNumbers, num)
			}
		}
		phoneNumbersSendTo = uniqueNumbers

		if len(phoneNumbersSendTo) == 0 {
			logrus.Warnf("Skipping MonitoringReport since no phone numbers configured to send to")
			return
		}

		receiverJIDs := controllers.ConvertPhoneNumbersToJIDs(phoneNumbersSendTo)

		var wg sync.WaitGroup
		wg.Add(2)

		var ticketAchievementReport, loginVisitReport string
		var ticketErr, loginVisitErr error

		// Parallel: generate both reports
		go func() {
			defer wg.Done()
			ticketAchievementReport, ticketErr = controllers.MonitoringTicketODOOMS()
		}()
		go func() {
			defer wg.Done()
			loginVisitReport, loginVisitErr = controllers.MonitoringVisitAndLoginTechnicianODOOMS()
		}()

		wg.Wait()

		// Handle ticket report result
		if ticketErr != nil {
			logrus.Error(ticketErr)
		}
		if ticketAchievementReport == "" && ticketErr == nil {
			id := "⚠ Mohon maaf, gagal untuk generate Summary Ticket Achievements karena tidak ada data."
			en := "⚠ Sorry, failed to generate Summary Ticket Achievements because there is no data."
			for _, jid := range receiverJIDs {
				controllers.SendLangMessage(jid, id, en, "id")
			}
		}

		// // Send chart images in background
		// if ticketAchievementReport != "" {
		// 	go controllers.GenerateExcelChartMonitoringTicketPerformanceODOOMSInBackground(ticketAchievementReport, receiverJIDs)
		// 	// go controllers.GenerateChartMonitoringTicketPerformanceODOOMSInBackground(ticketAchievementReport, receiverJIDs)
		// }
		// if loginVisitReport != "" {
		// 	go controllers.GenerateExcelChartMonitoringLoginVisitTechnicianODOOMSInBackground(loginVisitReport, receiverJIDs)
		// 	// go controllers.GenerateChartMonitoringLoginVisitTechnicianODOOMSInBackground(loginVisitReport, receiverJIDs)
		// }

		// // Remove last sheet of ticketAchievementReport before sending via email
		// if ticketAchievementReport != "" {
		// 	func() {
		// 		f, err := excelize.OpenFile(ticketAchievementReport)
		// 		if err != nil {
		// 			logrus.Error(err)
		// 			return
		// 		}
		// 		defer f.Close()
		// 		if err := f.DeleteSheet("CHART"); err != nil {
		// 			logrus.Error(err)
		// 			return
		// 		}
		// 		if err := f.Save(); err != nil {
		// 			logrus.Error(err)
		// 			return
		// 		}
		// 	}()
		// }

		// Handle login visit report result
		if loginVisitErr != nil {
			logrus.Error(loginVisitErr)
		}
		if loginVisitReport == "" && loginVisitErr == nil {
			id := "⚠ Mohon maaf, gagal untuk generate Summary Login Visit Technician karena tidak ada data."
			en := "⚠ Sorry, failed to generate Summary Login Visit Technician because there is no data."
			for _, jid := range receiverJIDs {
				controllers.SendLangMessage(jid, id, en, "id")
			}
		}
		// Remove last sheet of loginVisitReport before sending via email
		if loginVisitReport != "" {
			func() {
				f, err := excelize.OpenFile(loginVisitReport)
				if err != nil {
					logrus.Error(err)
					return
				}
				defer f.Close()
				if err := f.DeleteSheet("CHART"); err != nil {
					logrus.Error(err)
					return
				}
				if err := f.Save(); err != nil {
					logrus.Error(err)
					return
				}
			}()
		}

		// Check file size and WhatsApp fallback for ticket report
		if ticketAchievementReport != "" {
			fileInfo, err := os.Stat(ticketAchievementReport)
			if err != nil {
				logrus.Error(err)
				ticketAchievementReport = ""
			} else if fileInfo.Size() > config.GetConfig().Email.MaxAttachmentSize*1024*1024 {
				id := fmt.Sprintf("⚠ Mohon maaf, gagal untuk mengirim email Summary Ticket Achievements karena ukuran file %d bytes melebihi batas maksimum %d MB", fileInfo.Size(), config.GetConfig().Email.MaxAttachmentSize)
				en := fmt.Sprintf("⚠ Sorry, failed to send Summary Ticket Achievements because file size %d bytes exceeds maximum limit of %d MB", fileInfo.Size(), config.GetConfig().Email.MaxAttachmentSize)
				for _, jid := range receiverJIDs {
					controllers.SendLangMessage(jid, id, en, "id")
				}
				id = fmt.Sprintf("Berikut kami lampirkan file Summary Ticket Achievements melalui WhatsApp. Dikarenakan ukuran file yang cukup besar (%v), sehingga tidak dapat kami lampirkan melalui email.",
					strconv.FormatInt(fileInfo.Size()/1024, 10)+" KB",
				)
				en = fmt.Sprintf("Herewith we attach the Summary Ticket Achievements file via WhatsApp. Due to the rather large file size (%v), it cannot be attached via email.",
					strconv.FormatInt(fileInfo.Size()/1024, 10)+" KB",
				)
				for _, jid := range receiverJIDs {
					controllers.SendLangDocumentViaBotWhatsapp(jid, id, en, "id", ticketAchievementReport)
				}
				ticketAchievementReport = ""
			}
		}

		// Check file size and WhatsApp fallback for login visit report
		if loginVisitReport != "" {
			fileInfo2, err := os.Stat(loginVisitReport)
			if err != nil {
				logrus.Error(err)
				loginVisitReport = ""
			} else if fileInfo2.Size() > config.GetConfig().Email.MaxAttachmentSize*1024*1024 {
				id := fmt.Sprintf("⚠ Mohon maaf, gagal untuk mengirim email Summary Technician Attendance karena ukuran file %d bytes melebihi batas maksimum %d MB", fileInfo2.Size(), config.GetConfig().Email.MaxAttachmentSize)
				en := fmt.Sprintf("⚠ Sorry, failed to send Summary Technician Attendance because file size %d bytes exceeds maximum limit of %d MB", fileInfo2.Size(), config.GetConfig().Email.MaxAttachmentSize)
				for _, jid := range receiverJIDs {
					controllers.SendLangMessage(jid, id, en, "id")
				}
				id = fmt.Sprintf("Berikut kami lampirkan file Summary Technician Attendance melalui WhatsApp. Dikarenakan ukuran file yang cukup besar (%v), sehingga tidak dapat kami lampirkan melalui email.",
					strconv.FormatInt(fileInfo2.Size()/1024, 10)+" KB",
				)
				en = fmt.Sprintf("Herewith we attach the Summary Technician Attendance file via WhatsApp. Due to the rather large file size (%v), it cannot be attached via email.",
					strconv.FormatInt(fileInfo2.Size()/1024, 10)+" KB",
				)
				for _, jid := range receiverJIDs {
					controllers.SendLangDocumentViaBotWhatsapp(jid, id, en, "id", loginVisitReport)
				}
				loginVisitReport = ""
			}
		}

		// Prepare attachments for email
		var attachments []fun.EmailAttachment
		if ticketAchievementReport != "" {
			attachments = append(attachments, fun.EmailAttachment{
				FilePath:    ticketAchievementReport,
				NewFileName: fmt.Sprintf("Summary_Ticket_Achievements_%v.xlsx", time.Now().Format("02Jan2006")),
			})
		}
		if loginVisitReport != "" {
			attachments = append(attachments, fun.EmailAttachment{
				FilePath:    loginVisitReport,
				NewFileName: fmt.Sprintf("Summary_Technician_Attendance_%v.xlsx", time.Now().Format("02Jan2006")),
			})
		}

		if len(attachments) == 0 {
			// No valid report to send
			return
		}

		// Build MJML template (reuse your previous logic or keep simple)
		var sb strings.Builder
		sb.WriteString("<mjml>")
		sb.WriteString(`
	   <mj-head>
		   <mj-preview>Monitoring Reports ...</mj-preview>
		   <mj-style inline="inline">
		   .body-section {
			   background-color: #ffffff;
			   padding: 30px;
			   border-radius: 12px;
			   box-shadow: 0 2px 8px rgba(0, 0, 0, 0.08);
		   }
		   .footer-text {
			   color: #6b7280;
			   font-size: 12px;
			   text-align: center;
			   padding-top: 10px;
			   border-top: 1px solid #e5e7eb;
		   }
		   .header-title {
			   font-size: 66px;
			   font-weight: bold;
			   color: #1E293B;
			   text-align: left;
		   }
		   .cta-button {
			   background-color: #6D28D9;
			   color: #ffffff;
			   padding: 12px 24px;
			   border-radius: 8px;
			   font-size: 16px;
			   font-weight: bold;
			   text-align: center;
			   display: inline-block;
		   }
		   .email-info {
			   color: #374151;
			   font-size: 16px;
		   }
		   </mj-style>
	   </mj-head>`)
		sb.WriteString(fmt.Sprintf(`
	   <mj-body background-color="#f8fafc">
		   <!-- Main Content -->
		   <mj-section css-class="body-section" padding="20px">
		   <mj-column>
			   <mj-text font-size="20px" color="#1E293B" font-weight="bold">Dear ALL,</mj-text>
			   <mj-text font-size="16px" color="#4B5563" line-height="1.6">
			   With this email, we would like to inform you that the Monitoring Reports have been successfully generated.
			   Please find the report(s) attached below.
			   </mj-text>

			   <mj-divider border-color="#e5e7eb"></mj-divider>

			   <mj-text font-size="16px" color="#374151">
			   Best Regards,<br>
			   <b><i>%v</i></b>
			   </mj-text>
		   </mj-column>
		   </mj-section>

		   <!-- Footer -->
		   <mj-section>
		   <mj-column>
			   <mj-text css-class="footer-text">
			   ⚠ This is an automated email. Please do not reply directly.
			   </mj-text>
			   <mj-text font-size="12px" color="#6b7280">
			   <b>📞 Technical Support: +%s</b><br>
			   </mj-text>
		   </mj-column>
		   </mj-section>

	   </mj-body>
	   `,
			config.GetConfig().Default.PT,
			config.GetConfig().Whatsmeow.WaTechnicalSupport,
		))
		sb.WriteString("</mjml>")
		mjmlTemplate := sb.String()

		var monitoringReportsTo, monitoringReportsCc, monitoringReportsBcc []string
		monitoringReportsTo = append(monitoringReportsTo, config.GetConfig().Report.MonitoringTicketODOOMS.To...)
		monitoringReportsTo = append(monitoringReportsTo, config.GetConfig().Report.MonitoringLoginVisitTechnician.To...)
		monitoringReportsCc = append(monitoringReportsCc, config.GetConfig().Report.MonitoringTicketODOOMS.Cc...)
		monitoringReportsCc = append(monitoringReportsCc, config.GetConfig().Report.MonitoringLoginVisitTechnician.Cc...)
		monitoringReportsBcc = append(monitoringReportsBcc, config.GetConfig().Report.MonitoringTicketODOOMS.Bcc...)
		monitoringReportsBcc = append(monitoringReportsBcc, config.GetConfig().Report.MonitoringLoginVisitTechnician.Bcc...)

		// Make unique
		toMap := make(map[string]struct{})
		var uniqueTo []string
		for _, addr := range monitoringReportsTo {
			if _, exists := toMap[addr]; !exists {
				toMap[addr] = struct{}{}
				uniqueTo = append(uniqueTo, addr)
			}
		}
		monitoringReportsTo = uniqueTo

		ccMap := make(map[string]struct{})
		var uniqueCc []string
		for _, addr := range monitoringReportsCc {
			if _, exists := ccMap[addr]; !exists {
				ccMap[addr] = struct{}{}
				uniqueCc = append(uniqueCc, addr)
			}
		}
		monitoringReportsCc = uniqueCc

		bccMap := make(map[string]struct{})
		var uniqueBcc []string
		for _, addr := range monitoringReportsBcc {
			if _, exists := bccMap[addr]; !exists {
				bccMap[addr] = struct{}{}
				uniqueBcc = append(uniqueBcc, addr)
			}
		}
		monitoringReportsBcc = uniqueBcc

		if err := fun.TrySendEmail(
			monitoringReportsTo,
			monitoringReportsCc,
			monitoringReportsBcc,
			fmt.Sprintf("Monitoring Reports - %s", time.Now().Format("02 Jan 2006")),
			mjmlTemplate,
			attachments,
		); err != nil {
			id := fmt.Sprintf("⚠ Mohon maaf, gagal untuk mengirim email Monitoring Reports : %v", err)
			en := fmt.Sprintf("⚠ Sorry, failed to send Monitoring Reports : %v", err)
			for _, jid := range receiverJIDs {
				controllers.SendLangMessage(jid, id, en, "id")
			}
			return
		}
	},
	"RemoveOldFilesDirectory": func() {
		folderNeeds := config.GetConfig().FolderFileNeeds
		if len(folderNeeds) == 0 {
			logrus.Warn("No folders configured to clean old files")
			return
		}

		// This dir will be manual deleted for its old data
		skippedDir := []string{
			"sp_technician",
			"sp_spl",
			"sp_sac",
			"payslip_technician",
		}

		for _, folder := range folderNeeds {
			if len(skippedDir) > 0 {
				for _, skip := range skippedDir {
					if folder == skip {
						logrus.Infof("Skipping cleanup for folder %s as it's in skippedDir list", folder)
						continue
					}
				}
			}

			selectedDir, err := fun.FindValidDirectory([]string{
				"web/file/" + folder,
				"../web/file/" + folder,
				"../../web/file/" + folder,
			})
			if err != nil {
				logrus.Errorf("Failed to find valid directory for folder %s: %v", folder, err)
				continue
			}
			dateDirFormat := "2006-01-02"
			thresholdRange := "-3Days" // can be "-1Month", "-3Days", etc. -> means 7 days ago and older will be removed
			if err := fun.RemoveExistingDirectory(selectedDir, thresholdRange, dateDirFormat); err != nil {
				logrus.Errorf("Failed to remove old directories in %s: %v", selectedDir, err)
			} else {
				logrus.Infof("Old directories in %s older than or equal to %s have been removed", selectedDir, thresholdRange)
			}
		}
	},
	"CleanupUploadedExcelFiles": func() {
		// Clean up old uploaded Excel files in web/file/uploaded_excel_to_odoo_ms
		selectedDir, err := fun.FindValidDirectory([]string{
			"web/file/uploaded_excel_to_odoo_ms",
			"../web/file/uploaded_excel_to_odoo_ms",
			"../../web/file/uploaded_excel_to_odoo_ms",
		})
		if err != nil {
			logrus.Errorf("Failed to find uploaded_excel_to_odoo_ms directory: %v", err)
			return
		}

		thresholdRange := config.GetConfig().UploadedExcelForODOOMS.ThresholdPurgeFile // Can be "-2days", "-1month", etc.
		if err := fun.RemoveOldFiles(selectedDir, thresholdRange); err != nil {
			logrus.Errorf("Failed to cleanup old Excel files in %s: %v", selectedDir, err)
		} else {
			logrus.Infof("✅ Successfully cleaned up old Excel files in %s (threshold: %s)", selectedDir, thresholdRange)
		}
	},
	"PurgeOldDatabaseLogs": func() {
		dayInt := config.GetConfig().Database.PurgeLogOlderThanDays
		if dayInt <= 0 {
			logrus.Warn("Skipping PurgeOldDatabaseLogs since Database.PurgeLogOlderThanDays is not set or <= 0")
			return
		}

		if err := controllers.RemoveLogMySQL(dayInt); err != nil {
			logrus.Error(err)
			return
		}
	},
	"BackupTableMonitoringTicket": func() {
		if err := controllers.BackupTableMonitoringTicket(); err != nil {
			logrus.Error(err)
			return
		}
	},
	"BackupTableBALostPrevMonth": func() {
		if err := controllers.BackupTableBALostPrevMonth(); err != nil {
			logrus.Error(err)
			return
		}
	},
	"SLAReportODOOMS": func() {
		excelFilePaths, err := controllers.GenerateSLAReportODOOMS()
		if err != nil {
			logrus.Errorf("SLA Report generation failed completely: %v", err)
			return
		}

		if len(excelFilePaths) == 0 {
			logrus.Warn("No SLA report generated successfully, skipping email sending")
			return
		}

		logrus.Infof("SLA Report generation completed, %d files returned by controller", len(excelFilePaths))

		slaTypesRaw := config.GetConfig().Report.SLA.GeneratedTypes
		var slaTypes []string
		for _, typ := range slaTypesRaw {
			typClean := strings.ReplaceAll(typ, " ", "")
			typClean = strings.ReplaceAll(typClean, "-", "")
			slaTypes = append(slaTypes, typClean)
		}

		logrus.Infof("Expected report types: %v", slaTypes)
		logrus.Infof("Generated file paths: %v", excelFilePaths)

		typeToFile := make(map[string]string)
		var availableReports []string
		var missingReports []string

		// Map type to file path by checking if type name is in file name AND file actually exists
		for _, filePath := range excelFilePaths {
			// First check if the file actually exists
			if _, err := os.Stat(filePath); os.IsNotExist(err) {
				logrus.Errorf("Generated file does not exist on filesystem: %s", filePath)
				continue
			}

			base := strings.ToLower(filepath.Base(filePath))
			logrus.Debugf("Processing file: %s (basename: %s)", filePath, base)

			matched := false
			for _, typ := range slaTypes {
				typLower := strings.ToLower(typ)

				// Use precise matching based on actual filename patterns: (03Oct2025)SLAReport_TYPE.xlsx
				var isMatch bool
				switch typLower {
				case "pm":
					// PM should match exactly "slareport_pm.xlsx" but NOT "slareport_nonpm.xlsx"
					isMatch = strings.Contains(base, "slareport_pm.xlsx")
				case "nonpm":
					// NonPM should match "slareport_nonpm.xlsx"
					isMatch = strings.Contains(base, "slareport_nonpm.xlsx")
				case "cm":
					// CM should match exactly "slareport_cm.xlsx"
					isMatch = strings.Contains(base, "slareport_cm.xlsx")
				case "solvedpending":
					// SolvedPending should match "slareport_solvedpending.xlsx"
					isMatch = strings.Contains(base, "slareport_solvedpending.xlsx")
				case "master":
					// Master should match "slareport_master.xlsx"
					isMatch = strings.Contains(base, "slareport_master.xlsx")
				default:
					// Fallback: check for pattern "slareport_TYPE.xlsx"
					pattern := fmt.Sprintf("slareport_%s.xlsx", typLower)
					isMatch = strings.Contains(base, pattern)
				}

				if isMatch {
					typeToFile[typ] = filePath
					availableReports = append(availableReports, typ)
					logrus.Infof("✅ SLA Report found and verified: %s -> %s", typ, filepath.Base(filePath))
					matched = true
					break
				}
			}

			if !matched {
				logrus.Warnf("Generated file doesn't match any expected report type: %s", filepath.Base(filePath))
			}
		}

		// Find missing reports
		for _, typ := range slaTypes {
			if _, exists := typeToFile[typ]; !exists {
				missingReports = append(missingReports, typ)
				logrus.Warnf("❌ SLA Report missing: %s (not generated or no data available)", typ)
			}
		}

		logrus.Infof("📊 SLA Report Summary: %d available, %d missing out of %d total types",
			len(availableReports), len(missingReports), len(slaTypes))

		var sb strings.Builder
		sb.WriteString("<mjml>")
		sb.WriteString(`
	<mj-head>
		<mj-preview>SLA Report ...</mj-preview>
		<mj-style inline="inline">
		.body-section {
			background-color: #ffffff;
			padding: 30px;
			border-radius: 12px;
			box-shadow: 0 2px 8px rgba(0, 0, 0, 0.08);
		}
		.footer-text {
			color: #6b7280;
			font-size: 12px;
			text-align: center;
			padding-top: 10px;
			border-top: 1px solid #e5e7eb;
		}
		.header-title {
			font-size: 66px;
			font-weight: bold;
			color: #1E293B;
			text-align: left;
		}
		.cta-button {
			background-color: #6D28D9;
			color: #ffffff;
			padding: 12px 24px;
			border-radius: 8px;
			font-size: 16px;
			font-weight: bold;
			text-align: center;
			display: inline-block;
		}
		.email-info {
			color: #374151;
			font-size: 16px;
		}
		.success-text {
			color: #059669;
			font-weight: bold;
		}
		.warning-text {
			color: #d97706;
			font-weight: bold;
		}
		</mj-style>
	</mj-head>`)

		sb.WriteString(fmt.Sprintf(`
	<mj-body background-color="#f8fafc">
		<mj-section css-class="body-section" padding="20px">
			<mj-column>
				<mj-text font-size="20px" color="#1E293B" font-weight="bold">Dear ALL,</mj-text>
				<mj-text font-size="16px" color="#4B5563" line-height="1.6">
				With this email, we would like to inform you that the SLA Report generation has been completed.<br><br>
				<strong>📊 Report Summary:</strong><br>
				• <span class="success-text">%d of %d reports generated successfully</span><br>
				• <span class="success-text">%d reports available for download</span>%s
				</mj-text>

				<mj-divider border-color="#e5e7eb"></mj-divider>`,
			len(availableReports), len(slaTypes), len(availableReports),
			func() string {
				if len(missingReports) > 0 {
					return fmt.Sprintf("<br>• <span class=\"warning-text\">%d reports could not be generated</span>", len(missingReports))
				}
				return ""
			}()))

		if len(availableReports) > 0 {
			sb.WriteString(`
				<mj-text font-size="16px" color="#374151" font-weight="bold">
				✅ Available Reports for Download:
				</mj-text>
				<mj-text font-size="16px" color="#374151">
				<ul style="padding-left:16px; margin-top:12px;">`)

			// Add links for available reports
			for _, typ := range availableReports {
				if filePath, ok := typeToFile[typ]; ok {
					fileReportDir := filepath.Dir(filePath)
					proxyLink := config.GetConfig().App.WebPublicURL + "/report-sla/" + filepath.Base(fileReportDir) + "/" + filepath.Base(filePath)
					sb.WriteString(fmt.Sprintf(
						`<li><b>%s</b>: <a href="%s" style="color:#6D28D9;font-weight:bold;">Download %s Report</a></li>`,
						typ, proxyLink, typ,
					))
				}
			}
			sb.WriteString("</ul></mj-text>")
		}

		if len(missingReports) > 0 {
			sb.WriteString(`
				<mj-text font-size="16px" color="#d97706" font-weight="bold">
				⚠️ Reports Not Available:
				</mj-text>
				<mj-text font-size="16px" color="#374151">
				<ul style="padding-left:16px;">`)

			for _, typ := range missingReports {
				sb.WriteString(fmt.Sprintf(`<li><b>%s</b> - Could not be generated (no data or error occurred)</li>`, typ))
			}
			sb.WriteString("</ul></mj-text>")
		}

		sb.WriteString(fmt.Sprintf(`
				<mj-divider border-color="#e5e7eb"></mj-divider>
				<mj-text font-size="16px" color="#374151">
				Best Regards,<br>
				<b><i>%s</i></b>
				</mj-text>
			</mj-column>
		</mj-section>

		<mj-section>
			<mj-column>
				<mj-text css-class="footer-text">
					⚠ This is an automated email. Please do not reply directly.
				</mj-text>
				<mj-text font-size="12px" color="#6b7280">
					<b>📞 Technical Support: +%s</b><br>
				</mj-text>
			</mj-column>
		</mj-section>
	</mj-body>`,
			config.GetConfig().Default.PT,
			config.GetConfig().Whatsmeow.WaTechnicalSupport,
		))

		sb.WriteString("</mjml>")

		mjmlTemplate := sb.String()

		// ⚠ If TrySendEmail expects HTML, convert MJML -> HTML first
		// htmlOutput := mjml.ToHTML(mjmlTemplate)

		if err := fun.TrySendEmail(
			config.GetConfig().Report.SLA.To,
			config.GetConfig().Report.SLA.Cc,
			config.GetConfig().Report.SLA.Bcc,
			fmt.Sprintf("SLA Report %v", time.Now().Format("02 Jan 2006")),
			// fmt.Sprintf("SLA Report %s (%d/%d Generated)", time.Now().Format("02 Jan 2006"), len(availableReports), len(slaTypes)),
			mjmlTemplate, // change to htmlOutput if conversion needed
			nil,
		); err != nil {
			logrus.Errorf("Failed to send SLA report email: %v", err)
		}
	},
	"GetTicketDKI": func() {
		msg, err := controllers.GetTicketDKIODOOMS()
		if err != nil {
			logrus.Error(err)
			return
		}
		logrus.Info(msg)
	},
	"GetTicketDSP": func() {
		msg, err := controllers.GetTicketDSPODOOMS()
		if err != nil {
			logrus.Error(err)
			return
		}
		logrus.Info(msg)
	},
	"GetTaskODOOMSMTI": func() {
		if err := controllers.GetTaskODOOMSMTI(); err != nil {
			logrus.Error(err)
			return
		}
	},
	"GetTaskODOOMSBNI": func() {
		if err := controllers.GetTaskODOOMSBNI(); err != nil {
			logrus.Error(err)
			return
		}
	},
	"GetODOOMSCompany": func() {
		if err := controllers.GetCompanyODOOMS(); err != nil {
			logrus.Error(err)
			return
		}
	},
	"GetODOOMSTicketType": func() {
		if err := controllers.GetTicketTypeODOOMS(); err != nil {
			logrus.Error(err)
			return
		}
	},
	"PrepareStockOpnameData": func() {
		if err := controllers.GetDataProductEDCCSNA(); err != nil {
			logrus.Error(err)
			return
		}
	},

	// "StatusSPStockOpname": func() {
	// 	now := time.Now().In(jakartaLoc)
	// 	logrus.Infof("Running StatusSPStockOpname check at %s", now.Format(time.RFC3339))

	// 	if !config.GetConfig().SPTechnician.SORunOnWeekends {
	// 		if now.Weekday() == time.Saturday || now.Weekday() == time.Sunday {
	// 			// Skip on weekends
	// 			logrus.Infof("Skipping StatusSPStockOpname on weekend (%s)", now.Weekday())
	// 			return
	// 		}
	// 	}

	// 	if !config.GetConfig().SPTechnician.SORunOnHolidays {
	// 		todayData, err := fun.GetLibur("today")
	// 		if err != nil {
	// 			logrus.Errorf("Failed to get holiday info: %v", err)
	// 		}
	// 		if todayData.IsHoliday && len(todayData.HolidayList) > 0 {
	// 			// Skip on public holidays
	// 			logrus.Infof("Skipping StatusSPStockOpname on public holiday (%s)",
	// 				strings.Join(todayData.HolidayList, ", "))
	// 			return
	// 		}
	// 	}

	// 	if err := controllers.CheckSPStockOpname(); err != nil {
	// 		logrus.Error(err)
	// 		return
	// 	}
	// },

	// // // REMOVE: soon if you not use it anymore, this method is only for testing purpose !!
	// "RemoveSoonYGY": func() {

	// },

	// ###################################################################
	// ########### Deprecated / Not used anymore #########################
	// ###################################################################
	// "MonitoringTicketODOOMS": func() {
	// 	phoneNumbersSend := config.GetConfig().Report.MonitoringTicketODOOMS.WhatsappSendToIfGotError
	// 	if len(phoneNumbersSend) == 0 {
	// 		logrus.Warnf("Skipping MonitoringTicketODOOMS since no phone numbers configured to send to")
	// 		return
	// 	}

	// 	senderJIDs := controllers.ConvertPhoneNumbersToJIDs(phoneNumbersSend)

	// 	excelFilePath, err := controllers.MonitoringTicketODOOMS()
	// 	if err != nil {
	// 		logrus.Error(err)
	// 		return
	// 	}

	// 	if excelFilePath == "" {
	// 		id := "⚠ Mohon maaf, gagal untuk generate Summary Ticket Achievements karena tidak ada data."
	// 		en := "⚠ Sorry, failed to generate Summary Ticket Achievements because there is no data."
	// 		for _, jid := range senderJIDs {
	// 			controllers.SendLangMessage(jid, id, en, "id")
	// 		}
	// 		return
	// 	}

	// 	go controllers.GenerateChartMonitoringTicketPerformanceODOOMSInBackground(excelFilePath, senderJIDs)

	// 	// Remove last sheet of excelFilePath before sending via email
	// 	func() {
	// 		f, err := excelize.OpenFile(excelFilePath)
	// 		if err != nil {
	// 			logrus.Error(err)
	// 			return
	// 		}
	// 		defer f.Close()

	// 		// Delete 'CHART' sheet if exists
	// 		if err := f.DeleteSheet("CHART"); err != nil {
	// 			logrus.Error(err)
	// 			return
	// 		}

	// 		if err := f.Save(); err != nil {
	// 			logrus.Error(err)
	// 			return
	// 		}
	// 	}()

	// 	fileInfo, err := os.Stat(excelFilePath)
	// 	if err != nil {
	// 		logrus.Error(err)
	// 		return
	// 	}

	// 	if fileInfo.Size() > config.GetConfig().Email.MaxAttachmentSize*1024*1024 {
	// 		id := fmt.Sprintf("⚠ Mohon maaf, gagal untuk mengirim email Summary Ticket Achievements karena ukuran file %d bytes melebihi batas maksimum %d MB", fileInfo.Size(), config.GetConfig().Email.MaxAttachmentSize)
	// 		en := fmt.Sprintf("⚠ Sorry, failed to send Summary Ticket Achievements because file size %d bytes exceeds maximum limit of %d MB", fileInfo.Size(), config.GetConfig().Email.MaxAttachmentSize)
	// 		for _, jid := range senderJIDs {
	// 			controllers.SendLangMessage(jid, id, en, "id")
	// 		}

	// 		id = fmt.Sprintf("Berikut kami lampirkan file Summary Ticket Achievements melalui WhatsApp. Dikarenakan ukuran file yang cukup besar (%v), sehingga tidak dapat kami lampirkan melalui email.",
	// 			strconv.FormatInt(fileInfo.Size()/1024, 10)+" KB",
	// 		)
	// 		en = fmt.Sprintf("Herewith we attach the Summary Ticket Achievements file via WhatsApp. Due to the rather large file size (%v), it cannot be attached via email.",
	// 			strconv.FormatInt(fileInfo.Size()/1024, 10)+" KB",
	// 		)
	// 		for _, jid := range senderJIDs {
	// 			controllers.SendLangDocumentViaBotWhatsapp(jid, id, en, "id", excelFilePath)
	// 		}
	// 		return
	// 	}

	// 	var sb strings.Builder
	// 	sb.WriteString("<mjml>")
	// 	sb.WriteString(`
	// 	<mj-head>
	// 		<mj-preview>Report Ticket Achievement ...</mj-preview>
	// 		<mj-style inline="inline">
	// 		.body-section {
	// 			background-color: #ffffff;
	// 			padding: 30px;
	// 			border-radius: 12px;
	// 			box-shadow: 0 2px 8px rgba(0, 0, 0, 0.08);
	// 		}
	// 		.footer-text {
	// 			color: #6b7280;
	// 			font-size: 12px;
	// 			text-align: center;
	// 			padding-top: 10px;
	// 			border-top: 1px solid #e5e7eb;
	// 		}
	// 		.header-title {
	// 			font-size: 66px;
	// 			font-weight: bold;
	// 			color: #1E293B;
	// 			text-align: left;
	// 		}
	// 		.cta-button {
	// 			background-color: #6D28D9;
	// 			color: #ffffff;
	// 			padding: 12px 24px;
	// 			border-radius: 8px;
	// 			font-size: 16px;
	// 			font-weight: bold;
	// 			text-align: center;
	// 			display: inline-block;
	// 		}
	// 		.email-info {
	// 			color: #374151;
	// 			font-size: 16px;
	// 		}
	// 		</mj-style>
	// 	</mj-head>`)

	// 	sb.WriteString(fmt.Sprintf(`
	// 	<mj-body background-color="#f8fafc">
	// 		<!-- Main Content -->
	// 		<mj-section css-class="body-section" padding="20px">
	// 		<mj-column>
	// 			<mj-text font-size="20px" color="#1E293B" font-weight="bold">Dear ALL,</mj-text>
	// 			<mj-text font-size="16px" color="#4B5563" line-height="1.6">
	// 			With this email, we would like to inform you that the Summary Ticket Achievement has been successfully generated.
	// 			Please find the report attached below.
	// 			</mj-text>

	// 			<mj-divider border-color="#e5e7eb"></mj-divider>

	// 			<mj-text font-size="16px" color="#374151">
	// 			Best Regards,<br>
	// 			<b><i>%v</i></b>
	// 			</mj-text>
	// 		</mj-column>
	// 		</mj-section>

	// 		<!-- Footer -->
	// 		<mj-section>
	// 		<mj-column>
	// 			<mj-text css-class="footer-text">
	// 			⚠ This is an automated email. Please do not reply directly.
	// 			</mj-text>
	// 			<mj-text font-size="12px" color="#6b7280">
	// 			<b>📞 Technical Support: +%s</b><br>
	// 			<!--
	// 			<br>
	// 			<a href="wa.me/%v">
	// 			📞 Support
	// 			</a>
	// 			-->
	// 			</mj-text>
	// 		</mj-column>
	// 		</mj-section>

	// 	</mj-body>
	// 	`,
	// 		config.GetConfig().Default.PT,
	// 		config.GetConfig().Whatsmeow.WaTechnicalSupport,
	// 		"085123456789",
	// 	))
	// 	sb.WriteString("</mjml>")

	// 	mjmlTemplate := sb.String()
	// 	reportName := fmt.Sprintf("Summary_Ticket_Achievements_%s", time.Now().Format("02Jan2006.xlsx"))
	// 	attachments := []fun.EmailAttachment{
	// 		{
	// 			FilePath:    excelFilePath,
	// 			NewFileName: reportName,
	// 		},
	// 	}

	// 	if err := fun.TrySendEmail(
	// 		config.GetConfig().Report.MonitoringTicketODOOMS.To,
	// 		config.GetConfig().Report.MonitoringTicketODOOMS.Cc,
	// 		config.GetConfig().Report.MonitoringTicketODOOMS.Bcc,
	// 		fmt.Sprintf("Summary Ticket Achievements - %s", time.Now().Format("02 Jan 2006")),
	// 		mjmlTemplate,
	// 		attachments,
	// 	); err != nil {
	// 		id := fmt.Sprintf("⚠ Mohon maaf, gagal untuk mengirim email Summary Ticket Achievements : %v", err)
	// 		en := fmt.Sprintf("⚠ Sorry, failed to send Summary Ticket Achievements : %v", err)
	// 		for _, jid := range senderJIDs {
	// 			controllers.SendLangMessage(jid, id, en, "id")
	// 		}
	// 		return
	// 	}

	// 	// // 🧹 Cleanup: remove excel report
	// 	// if err := os.Remove(excelFilePath); err != nil {
	// 	// 	logrus.Errorf("Failed to remove excel file %s: %v", excelFilePath, err)
	// 	// 	return
	// 	// }
	// },
	// "MonitoringLoginVisitTechnicianODOOMS": func() {
	// 	excelFilePath, err := controllers.MonitoringVisitAndLoginTechnicianODOOMS()
	// 	if err != nil {
	// 		logrus.Error(err)
	// 		return
	// 	}
	// },
}

func StartSchedulers(db *gorm.DB, cfg *config.YamlConfig) *gocron.Scheduler {
	loadTimezone()
	scheduler := gocron.NewScheduler(jakartaLoc)

	for _, sched := range cfg.Schedules {
		name := sched.Name

		if sched.Every != "" {
			fmt.Printf("⏱ Trying to run scheduler: %s every %v\n", name, sched.Every)
			dur, err := time.ParseDuration(sched.Every)
			if err != nil {
				logrus.Warnf("Invalid duration for %s: %v", name, err)
				continue
			}
			_, err = scheduler.Every(dur).Do(func() {
				runJob(name)
			})
			if err != nil {
				logrus.Warnf("Failed to schedule job %s: %v", name, err)
			} else {
				logrus.Infof("Scheduled job %s to run every %s", name, dur)
			}

		} else if len(sched.At) > 0 {
			// sched.At is a []string of times, e.g. ["11:02", "11:03"]
			for _, atTime := range sched.At {
				fmt.Printf("⏰ Trying to run scheduler: %s daily at %v\n", name, atTime)
				if !isValidTime(atTime) {
					logrus.Warnf("Invalid time format for %s: %s", name, atTime)
					continue
				}
				_, err := scheduler.Every(1).Day().At(atTime).Do(func() {
					runJob(name)
				})
				if err != nil {
					logrus.Warnf("Failed to schedule job %s: %v", name, err)
				} else {
					logrus.Infof("Scheduled job %s to run daily at %s", name, atTime)
				}
			}
		} else if sched.Weekly != "" {
			fmt.Printf("🕰 Trying to run scheduler: %s weekly at %v\n", name, sched.Weekly)
			parts := strings.Split(sched.Weekly, "@")
			if len(parts) != 2 || !isValidTime(parts[1]) {
				logrus.Warnf("Invalid weekly format for %s: %s", name, sched.Weekly)
				continue
			}
			weekdayStr := strings.ToLower(parts[0])
			timePart := parts[1]

			weekdayMap := map[string]time.Weekday{
				"sunday":    time.Sunday,
				"monday":    time.Monday,
				"tuesday":   time.Tuesday,
				"wednesday": time.Wednesday,
				"thursday":  time.Thursday,
				"friday":    time.Friday,
				"saturday":  time.Saturday,
				"sun":       time.Sunday,
				"mon":       time.Monday,
				"tue":       time.Tuesday,
				"wed":       time.Wednesday,
				"thu":       time.Thursday,
				"fri":       time.Friday,
				"sat":       time.Saturday,
			}
			weekday, ok := weekdayMap[weekdayStr]
			if !ok {
				logrus.Warnf("Invalid weekday for %s: %s", name, weekdayStr)
				continue
			}

			_, err := scheduler.Every(1).Week().Weekday(weekday).At(timePart).Do(func() {
				runJob(name)
			})
			if err != nil {
				logrus.Warnf("Failed to schedule weekly job %s: %v", name, err)
			} else {
				logrus.Infof("Scheduled job %s to run weekly on %s at %s", name, weekday, timePart)
			}

		} else if sched.Monthly != "" {
			fmt.Printf("⏳ Trying to run scheduler: %s monthly at %v\n", name, sched.Monthly)
			parts := strings.Split(sched.Monthly, "@")
			if len(parts) != 2 || !isValidTime(parts[1]) {
				logrus.Warnf("Invalid monthly format for %s: %s", name, sched.Monthly)
				continue
			}
			dayPart := parts[0]
			timePart := parts[1]

			if dayPart == "last" {
				// Run daily at given time, but check if today is last day of month in Jakarta time
				_, err := scheduler.Every(1).Day().At(timePart).Do(func() {
					now := time.Now().In(jakartaLoc)
					tomorrow := now.AddDate(0, 0, 1)
					if tomorrow.Month() != now.Month() {
						runJob(name)
					}
				})
				if err != nil {
					logrus.Warnf("Failed to schedule last-day monthly job %s: %v", name, err)
				} else {
					logrus.Infof("Scheduled job %s to run monthly on last day at %s", name, timePart)
				}
			} else {
				dayInt, err := strconv.Atoi(dayPart)
				if err != nil || dayInt < 1 || dayInt > 31 {
					logrus.Warnf("Invalid day for monthly job %s: %s", name, dayPart)
					continue
				}
				// Run daily at timePart, but only on day == dayInt in Jakarta time
				_, err = scheduler.Every(1).Day().At(timePart).Do(func() {
					if time.Now().In(jakartaLoc).Day() == dayInt {
						runJob(name)
					}
				})
				if err != nil {
					logrus.Warnf("Failed to schedule monthly job %s: %v", name, err)
				} else {
					logrus.Infof("Scheduled job %s to run monthly on day %d at %s", name, dayInt, timePart)
				}
			}
		} else if sched.Yearly != "" {
			fmt.Printf("📅 Trying to run scheduler: %s yearly at %v\n", name, sched.Yearly)
			parts := strings.Split(sched.Yearly, "@")
			if len(parts) != 2 || !isValidTime(parts[1]) {
				logrus.Warnf("Invalid yearly format for %s: %s", name, sched.Yearly)
				continue
			}
			dayPart := parts[0] // e.g. "01" for January 1st
			timePart := parts[1]

			dayInt, err := strconv.Atoi(dayPart)
			if err != nil || dayInt < 1 || dayInt > 31 {
				logrus.Warnf("Invalid day for yearly job %s: %s", name, dayPart)
				continue
			}

			_, err = scheduler.Every(1).Day().At(timePart).Do(func() {
				now := time.Now().In(jakartaLoc)
				if now.Month() == time.January && now.Day() == dayInt {
					runJob(name)
				}
			})
			if err != nil {
				logrus.Warnf("Failed to schedule yearly job %s: %v", name, err)
			} else {
				logrus.Infof("Scheduled job %s to run yearly on Jan %d at %s", name, dayInt, timePart)
			}
		}
	}

	scheduler.StartAsync()
	logrus.Infof("✅ All schedulers started (%s timezone).", jakartaLoc)
	return scheduler
}

func runJob(name string) {
	if job, ok := jobMap[name]; ok {
		logrus.Debugf("Scheduled running job: %s @ %v (%s timezone)", name, time.Now().In(jakartaLoc), jakartaLoc)
		job()
	} else {
		logrus.Warnf("Unknown job: %s", name)
	}
}

func isValidTime(t string) bool {
	_, err := time.Parse("15:04", t)
	return err == nil
}
