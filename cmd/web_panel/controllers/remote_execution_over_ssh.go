package controllers

import (
	"fmt"
	"service-platform/internal/config"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"go.mau.fi/whatsmeow/types/events"
	"golang.org/x/crypto/ssh"
)

const sshCommandTimeout = 30 * time.Second

var (
	showStatusODOODashboardMutex   sync.Mutex
	restartMySQLODOODashboardMutex sync.Mutex
)

type WindowsProcess struct {
	Name       string
	PID        string
	RAMBytes   int64
	CPUPercent float64
}

type NetworkInterface struct {
	Name          string
	BytesSent     int64
	BytesReceived int64
}

type WindowsStatus struct {
	TotalRAMBytes int64
	Processes     []WindowsProcess
	Network       []NetworkInterface
}

func ConnectSSH(user, password, addr string) (*ssh.Client, error) {
	config := &ssh.ClientConfig{
		User:            user,
		Auth:            []ssh.AuthMethod{ssh.Password(password)},
		HostKeyCallback: ssh.InsecureIgnoreHostKey(),
		Timeout:         sshCommandTimeout,
	}
	return ssh.Dial("tcp", addr, config)
}

func runCommandWithTimeout(client *ssh.Client, cmd string, timeout time.Duration) (string, error) {
	session, err := client.NewSession()
	if err != nil {
		return "", err
	}
	defer session.Close()

	stdoutPipe, _ := session.StdoutPipe()
	stderrPipe, _ := session.StderrPipe()

	if err := session.Start(cmd); err != nil {
		return "", err
	}

	var output strings.Builder
	done := make(chan error, 1)

	// Read stdout
	go func() {
		buf := make([]byte, 1024)
		for {
			n, err := stdoutPipe.Read(buf)
			if n > 0 {
				output.WriteString(string(buf[:n]))
			}
			if err != nil {
				break
			}
		}
	}()

	// Read stderr
	go func() {
		buf := make([]byte, 1024)
		for {
			n, err := stderrPipe.Read(buf)
			if n > 0 {
				output.WriteString(string(buf[:n]))
			}
			if err != nil {
				break
			}
		}
	}()

	go func() { done <- session.Wait() }()

	select {
	case <-time.After(timeout):
		return output.String(), fmt.Errorf("timeout after %v", timeout)
	case err := <-done:
		return output.String(), err
	}
}

func GetWindowsNetworkStatus(client *ssh.Client) ([]NetworkInterface, error) {
	cmd := `wmic path Win32_PerfFormattedData_Tcpip_NetworkInterface get Name,BytesSentPersec,BytesReceivedPersec`
	output, err := runCommandWithTimeout(client, cmd, sshCommandTimeout)
	if err != nil {
		return nil, fmt.Errorf("failed to get network stats: %v", err)
	}

	var interfaces []NetworkInterface
	lines := strings.Split(output, "\n")
	for _, line := range lines[1:] { // skip header
		fields := strings.Fields(line)
		if len(fields) < 3 {
			continue
		}
		name := strings.Join(fields[:len(fields)-2], " ")
		bytesSent, _ := strconv.ParseInt(fields[len(fields)-2], 10, 64)
		bytesReceived, _ := strconv.ParseInt(fields[len(fields)-1], 10, 64)
		interfaces = append(interfaces, NetworkInterface{
			Name:          name,
			BytesSent:     bytesSent,
			BytesReceived: bytesReceived,
		})
	}
	return interfaces, nil
}

func GetWindowsStatus(client *ssh.Client) (*WindowsStatus, error) {
	var status WindowsStatus

	// Get total RAM
	totalRAMOutput, err := runCommandWithTimeout(client, `wmic computersystem get TotalPhysicalMemory`, sshCommandTimeout)
	if err != nil {
		return nil, fmt.Errorf("failed to get total RAM: %v", err)
	}
	for _, line := range strings.Split(totalRAMOutput, "\n") {
		line = strings.TrimSpace(line)
		if n, err := strconv.ParseInt(line, 10, 64); err == nil {
			status.TotalRAMBytes = n
			break
		}
	}
	if status.TotalRAMBytes == 0 {
		return nil, fmt.Errorf("failed to parse total RAM from output: %s", totalRAMOutput)
	}

	// Get processes
	processOutput, err := runCommandWithTimeout(client, `wmic path Win32_PerfFormattedData_PerfProc_Process get Name,IDProcess,PercentProcessorTime,WorkingSet`, sshCommandTimeout)
	if err != nil {
		return nil, fmt.Errorf("failed to get process list: %v", err)
	}
	lines := strings.Split(processOutput, "\n")
	for _, line := range lines[1:] {
		fields := strings.Fields(line)
		if len(fields) >= 4 {
			name := fields[0]
			pid := fields[1]
			cpuPercent, _ := strconv.ParseFloat(fields[2], 64)
			ramBytes, _ := strconv.ParseInt(fields[3], 10, 64)
			if ramBytes > 0 || cpuPercent > 0 {
				status.Processes = append(status.Processes, WindowsProcess{
					Name:       name,
					PID:        pid,
					RAMBytes:   ramBytes,
					CPUPercent: cpuPercent,
				})
			}
		}
	}

	// Get network stats
	networkStats, err := GetWindowsNetworkStatus(client)
	if err != nil {
		return nil, fmt.Errorf("failed to get network status: %v", err)
	}
	status.Network = networkStats

	return &status, nil
}

func ShowStatusVMODOODashboard(v *events.Message, userLang string) {
	eventToDo := "Show Status from VM ODOO Dashboard"
	originalSenderJID := NormalizeSenderJID(v.Info.Sender.String())
	stanzaID := v.Info.ID

	if !showStatusODOODashboardMutex.TryLock() {
		id := fmt.Sprintf("⚠ Permintaan %s sedang diproses. Mohon tunggu sebentar.", eventToDo)
		en := fmt.Sprintf("⚠ Your %s request is being processed. Please wait a moment.", eventToDo)
		sendLangMessageWithStanza(v, stanzaID, originalSenderJID, id, en, userLang)
		return
	}
	defer showStatusODOODashboardMutex.Unlock()

	id, en := informUserRequestReceived(eventToDo)
	sendLangMessageWithStanza(v, stanzaID, originalSenderJID, id, en, userLang)

	sshUser := config.WebPanel.Get().VMOdooDashboard.SSHUser
	sshPassword := config.WebPanel.Get().VMOdooDashboard.SSHPwd
	sshAddr := config.WebPanel.Get().VMOdooDashboard.SSHAddr

	client, err := ConnectSSH(sshUser, sshPassword, sshAddr)
	if err != nil {
		id := "❌ Gagal terhubung ke VM ODOO Dashboard."
		en := "❌ Failed to connect to VM ODOO Dashboard."
		sendLangMessageWithStanza(v, stanzaID, originalSenderJID, id, en, userLang)
		return
	}
	defer client.Close()

	status, err := GetWindowsStatus(client)
	if err != nil {
		id := fmt.Sprintf("❌ Gagal mendapatkan status: %v", err)
		en := fmt.Sprintf("❌ Failed to get status: %v", err)
		sendLangMessageWithStanza(v, stanzaID, originalSenderJID, id, en, userLang)
		return
	}

	ramGB := float64(status.TotalRAMBytes) / (1024 * 1024 * 1024)
	msgID := fmt.Sprintf("💻 *Total RAM*: *%.2f GB*\n", ramGB)
	msgEN := fmt.Sprintf("💻 *Total RAM*: *%.2f GB*\n", ramGB)

	// Calculate total RAM used
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

	sendLangMessageWithStanza(
		v, stanzaID, originalSenderJID,
		"✅ *Status VM ODOO Dashboard:*\n\n"+msgID,
		"✅ *VM ODOO Dashboard Status:*\n\n"+msgEN,
		userLang,
	)
}

func RestartMySQLVMODOODashboard(v *events.Message, userLang string) {
	eventToDo := "Restart MySQL VM ODOO Dashboard"
	originalSenderJID := NormalizeSenderJID(v.Info.Sender.String())
	stanzaID := v.Info.ID

	if !restartMySQLODOODashboardMutex.TryLock() {
		id := fmt.Sprintf("⚠ Permintaan %s sedang diproses. Mohon tunggu sebentar.", eventToDo)
		en := fmt.Sprintf("⚠ Your %s request is being processed. Please wait a moment.", eventToDo)
		sendLangMessageWithStanza(v, stanzaID, originalSenderJID, id, en, userLang)
		return
	}
	defer restartMySQLODOODashboardMutex.Unlock()

	id, en := informUserRequestReceived(eventToDo)
	sendLangMessageWithStanza(v, stanzaID, originalSenderJID, id, en, userLang)

	sshUser := config.WebPanel.Get().VMOdooDashboard.SSHUser
	sshPassword := config.WebPanel.Get().VMOdooDashboard.SSHPwd
	sshAddr := config.WebPanel.Get().VMOdooDashboard.SSHAddr

	client, err := ConnectSSH(sshUser, sshPassword, sshAddr)
	if err != nil {
		id := "❌ Gagal terhubung ke VM ODOO Dashboard."
		en := "❌ Failed to connect to VM ODOO Dashboard."
		sendLangMessageWithStanza(v, stanzaID, originalSenderJID, id, en, userLang)
		return
	}
	defer client.Close()

	// Step 1: Find mysqld.exe and kill it
	searchCmd := `wmic process where "name='mysqld.exe'" get ProcessId`
	output, _ := runCommandWithTimeout(client, searchCmd, sshCommandTimeout)

	pids := []string{}
	for _, line := range strings.Split(output, "\n") {
		line = strings.TrimSpace(line)
		if line != "" && line != "ProcessId" {
			pids = append(pids, line)
		}
	}

	for _, pid := range pids {
		killCmd := fmt.Sprintf(`taskkill /PID %s /F`, pid)
		_, _ = runCommandWithTimeout(client, killCmd, 5*time.Second)
	}

	// Step 2: Run mysql_stop.bat
	stopCmd := `cmd /c "cd /d C:\xampp && mysql_stop.bat"`
	_, _ = runCommandWithTimeout(client, stopCmd, 10*time.Second)
	// ignore error if already stopped

	// Step 3: Run mysql_start.bat with longer timeout
	startCmd := `cmd /c "cd /d C:\xampp && mysql_start.bat"`
	_, err = runCommandWithTimeout(client, startCmd, 15*time.Second)
	if err != nil {
		id := fmt.Sprintf("❌ Gagal menjalankan mysql_start.bat: %v", err)
		en := fmt.Sprintf("❌ Failed to run mysql_start.bat: %v", err)
		sendLangMessageWithStanza(v, stanzaID, originalSenderJID, id, en, userLang)
	}

	// Step 4: Wait a few seconds, retry checking mysqld.exe
	found := false
	for i := 0; i < 3; i++ {
		time.Sleep(3 * time.Second)
		verifyOutput, _ := runCommandWithTimeout(client, searchCmd, sshCommandTimeout)
		if len(parseProcessIDs(verifyOutput)) > 0 {
			found = true
			break
		}
	}

	if !found {
		id := "❌ mysql_start.bat sudah dijalankan, tapi MySQL tidak berhasil jalan."
		en := "❌ mysql_start.bat was run, but MySQL is not running."
		sendLangMessageWithStanza(v, stanzaID, originalSenderJID, id, en, userLang)
		return
	}

	// Step 5: Success
	id = "✅ Berhasil me-restart MySQL di VM ODOO Dashboard."
	en = "✅ Successfully restarted MySQL on VM ODOO Dashboard."
	sendLangMessageWithStanza(v, stanzaID, originalSenderJID, id, en, userLang)
}

func parseProcessIDs(output string) []string {
	var pids []string
	for _, line := range strings.Split(output, "\n") {
		line = strings.TrimSpace(line)
		if line != "" && line != "ProcessId" {
			pids = append(pids, line)
		}
	}
	return pids
}
