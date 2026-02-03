package database

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"service-platform/cmd/web_panel/config"
	"service-platform/cmd/web_panel/fun"
	"strings"
	"time"
)

func buildDBDefaultYAML() string {
	var b strings.Builder

	dbBackupDest := config.GetConfig().Database.DBBackupDestinationDir
	dbBackupName := fmt.Sprintf("database_dumped_%s_%d.sql", config.GetConfig().Database.Name, time.Now().Unix())

	b.WriteString("jobs:\n")
	b.WriteString("- name: local-dump\n")
	b.WriteString("  dbdriver: mysql\n")
	b.WriteString(fmt.Sprintf("  dbdsn: root@tcp(%s:%s)/%s\n", config.GetConfig().Database.Host, config.GetConfig().Database.Port, config.GetConfig().Database.Name))
	b.WriteString(fmt.Sprintf("  gzip: %v\n", false))
	b.WriteString("  storage:\n")
	b.WriteString("    local:\n")
	b.WriteString(fmt.Sprintf("      - path: %s/%s\n", dbBackupDest, dbBackupName))
	// b.WriteString("      - path: /Users/jack/Desktop/mydb2.sql\n")
	// b.WriteString("  options:\n")
	// b.WriteString("      - --skip-comments 		# Optional: keeps the dump clean\n")
	// b.WriteString("      - --no-create-info		# to allow full data dump\n")

	return b.String()
}

func DumpDatabase() error {
	configPath := config.GetConfig().Database.DBConfigPath

	configYAML := buildDBDefaultYAML()

	// Ensure parent directory exists
	dir := filepath.Dir(configPath)
	if err := os.MkdirAll(dir, os.ModePerm); err != nil {
		return fmt.Errorf("failed to create directory %s: %w", dir, err)
	}

	// Write (or overwrite) the config file
	err := os.WriteFile(configPath, []byte(configYAML), 0644)
	if err != nil {
		return fmt.Errorf("failed to write config file: %w", err)
	}

	// ⚙ Now trigger the dump process (using "onedump" or your own tool)
	serviceRunInWindows := fun.IsWindows()
	var cmd *exec.Cmd
	if serviceRunInWindows {
		cmd = exec.Command("bin/onedump.exe", "-f", configPath)
		output, err := cmd.CombinedOutput()
		if err != nil {
			return fmt.Errorf("backup failed: %v\nOutput: %s", err, output)
		}
		fmt.Println("✅ Database dump completed (Windows):\n", string(output))
	} else {
		// On Linux: check if mysqldump exists
		path, err := exec.LookPath("mysqldump")
		if err != nil {
			return fmt.Errorf("mysqldump not found in PATH: %w", err)
		}
		fmt.Println("Found mysqldump at:", path)

		// Build the command
		cmd = exec.Command(path, "-u", config.GetConfig().Database.Username, fmt.Sprintf("-p%s", config.GetConfig().Database.Password), config.GetConfig().Database.Name)

		// Redirect output to dump file
		dumpFile := config.GetConfig().Database.DBBackupDestinationDir + "/dump/db_dumped_" + time.Now().Format("2006_01_02") + ".sql"

		outFile, err := os.Create(dumpFile)
		if err != nil {
			return fmt.Errorf("failed to create dump file: %w", err)
		}
		defer outFile.Close()

		cmd := exec.Command(path,
			"-h", config.GetConfig().Database.Host,
			"-u", config.GetConfig().Database.Username,
			fmt.Sprintf("-p%s", config.GetConfig().Database.Password),
			config.GetConfig().Database.Name,
		)

		cmd.Stdout = outFile
		cmd.Stderr = os.Stderr // optional: show errors in console

		err = cmd.Run()
		if err != nil {
			return fmt.Errorf("mysqldump failed: %w", err)
		}

		fmt.Println("✅ Database dump completed (Linux): dump saved to", dumpFile)

	}

	return nil
}
