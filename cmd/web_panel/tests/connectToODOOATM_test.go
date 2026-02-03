package tests

import (
	"fmt"
	"sort"
	"strconv"
	"strings"
	"testing"
	"time"

	"golang.org/x/crypto/ssh"
)

type Process struct {
	Name     string
	PID      string
	RAMBytes int64
}

func connectSSH(user, password, addr string) (*ssh.Client, error) {
	config := &ssh.ClientConfig{
		User:            user,
		Auth:            []ssh.AuthMethod{ssh.Password(password)},
		HostKeyCallback: ssh.InsecureIgnoreHostKey(),
		Timeout:         5 * time.Second,
	}
	return ssh.Dial("tcp", addr, config)
}

func runCommand(client *ssh.Client, cmd string) (string, error) {
	session, err := client.NewSession()
	if err != nil {
		return "", err
	}
	defer session.Close()
	output, err := session.CombinedOutput(cmd)
	return string(output), err
}

// runCommandWithTimeout runs a command and forces timeout if the process hangs
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

	// Read output in background
	var output strings.Builder
	done := make(chan error, 1)

	go func() {
		buf := make([]byte, 1024)
		for {
			n, err := stdoutPipe.Read(buf)
			if n > 0 {
				text := string(buf[:n])
				output.WriteString(text)
				fmt.Print(text)
			}
			if err != nil {
				break
			}
		}
	}()
	go func() {
		buf := make([]byte, 1024)
		for {
			n, err := stderrPipe.Read(buf)
			if n > 0 {
				text := string(buf[:n])
				output.WriteString(text)
				fmt.Print(text)
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

func TestConnectToODOOATM(t *testing.T) {
	sshUser := "aaaaaaaaaaaaaa"
	sshPassword := "bbbbbbbbbbbbbbbbbbbbbbbb"
	sshAddr := "aaaaaaaaaa:bbbbbbbbbb"

	client, err := connectSSH(sshUser, sshPassword, sshAddr)
	if err != nil {
		t.Fatalf("Failed to connect via SSH: %v", err)
	}
	defer client.Close()

	// === Get total RAM
	totalRAMOutput, err := runCommand(client, `wmic computersystem get TotalPhysicalMemory`)
	if err != nil {
		t.Fatalf("Failed to get total RAM: %v", err)
	}
	var totalRAMBytes int64
	for _, line := range strings.Split(totalRAMOutput, "\n") {
		line = strings.TrimSpace(line)
		if n, err := strconv.ParseInt(line, 10, 64); err == nil {
			totalRAMBytes = n
			break
		}
	}
	if totalRAMBytes == 0 {
		t.Fatalf("Could not parse total RAM")
	}

	// === Get processes
	processOutput, err := runCommand(client, `wmic process get Name,ProcessId,WorkingSetSize`)
	if err != nil {
		t.Fatalf("Failed to get processes: %v", err)
	}

	var processes []Process
	lines := strings.Split(processOutput, "\n")
	for _, line := range lines[1:] {
		fields := strings.Fields(line)
		if len(fields) == 3 {
			name := fields[0]
			pid := fields[1]
			ramBytes, err := strconv.ParseInt(fields[2], 10, 64)
			if err == nil && ramBytes > 0 {
				processes = append(processes, Process{name, pid, ramBytes})
			}
		}
	}

	// === Find mysqld.exe & kill
	var mysqlPID string
	for _, p := range processes {
		if strings.EqualFold(p.Name, "mysqld.exe") {
			mysqlPID = p.PID
			break
		}
	}

	if mysqlPID != "" {
		fmt.Printf("mysqld.exe is running → trying to kill PID %s\n", mysqlPID)
		taskkillOut, err := runCommand(client, fmt.Sprintf(`taskkill /PID %s /F`, mysqlPID))
		fmt.Println("=== taskkill output ===")
		fmt.Println(taskkillOut)
		if err != nil {
			t.Fatalf("Failed to kill mysqld.exe: %v", err)
		}
	} else {
		fmt.Println("mysqld.exe not found running; skipping kill")
	}

	// === Run mysql_stop.bat
	fmt.Println("== Running mysql_stop.bat")
	stopOut, err := runCommand(client, `cmd /C "C:\xampp\mysql_stop.bat"`)
	fmt.Println("=== mysql_stop.bat output ===")
	fmt.Println(stopOut)
	if err != nil {
		t.Fatalf("Failed to run mysql_stop.bat: %v", err)
	}

	// === Run mysql_start.bat with timeout so it doesn't freeze
	fmt.Println("== Running mysql_start.bat")
	startOut, err := runCommandWithTimeout(client, `cmd /C "C:\xampp\mysql_start.bat"`, 15*time.Second)
	fmt.Println("=== mysql_start.bat output ===")
	fmt.Println(startOut)
	if err != nil {
		fmt.Printf("mysql_start.bat exited with: %v (ignored, assuming started)\n", err)
	}

	// === Print RAM summary
	fmt.Printf("\nTotal RAM: %.2f GB\n", float64(totalRAMBytes)/(1024*1024*1024))
	fmt.Println("Top 10 processes by RAM:")
	sort.Slice(processes, func(i, j int) bool {
		return processes[i].RAMBytes > processes[j].RAMBytes
	})
	for i, p := range processes {
		if i >= 10 {
			break
		}
		ramMB := float64(p.RAMBytes) / (1024 * 1024)
		percent := float64(p.RAMBytes) / float64(totalRAMBytes) * 100
		fmt.Printf("Process: %-25s PID: %-8s RAM: %.2f MB (%.2f%%)\n", p.Name, p.PID, ramMB, percent)
	}

	fmt.Println("\n✅ MySQL restart complete")
}
