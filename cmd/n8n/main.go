package main

import (
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"service-platform/internal/config"
	"service-platform/internal/pkg/fun"
)

func main() {
	if err := config.LoadConfig(); err != nil {
		log.Fatalf("Error loading .yaml conf :%v", err)
	}

	if len(os.Args) > 1 {
		switch os.Args[1] {
		case "--start":
			startN8n()
		case "--stop":
			stopN8n()
		case "--help", "-h":
			printHelp()
		default:
			fmt.Printf("Unknown argument: %s\n", os.Args[1])
			printHelp()
			os.Exit(1)
		}
	} else {
		startN8n()
	}
}

func startN8n() {
	fmt.Println("🚀 Starting N8N workflow automation...")
	// Check if Podman is available
	if !fun.IsPodmanAvailable() {
		log.Fatal("Podman is not available. Please install Podman to run N8N.")
	}

	// Check if N8N container is already running
	if fun.IsContainerRunning("n8n") {
		fmt.Println("✅ N8N is already running")
		return
	}

	// Get current working directory for mounting workflows
	workFlowDir, err := fun.FindValidDirectory([]string{
		"internal/n8n/workflows",
		"../internal/n8n/workflows",
		"../../internal/n8n/workflows",
	})
	if err != nil {
		log.Fatalf("Failed to find workflows directory: %v", err)
	}

	absWorkFlowDir, err := filepath.Abs(workFlowDir)
	if err != nil {
		log.Fatalf("Failed to get absolute path for workflows directory: %v", err)
	}

	// Create n8n network if it doesn't exist
	exec.Command("podman", "network", "create", "n8n-net").Run()

	// Start Postgres for n8n
	startPostgres()

	// Run N8N Podman container
	n8nPort := config.GetConfig().N8N.Port
	args := []string{
		"run", "-d", "--name", "service-platform-n8n", "--replace",
		"--network", "n8n-net",
		"-p", fmt.Sprintf("%d:%d", n8nPort, n8nPort),
		"-e", fmt.Sprintf("N8N_PORT=%d", n8nPort),
		"-e", "N8N_METRICS=true",
		"-e", "N8N_PERSONALIZATION_ENABLED=false", // Disable telemetry/data collection
		"-e", "N8N_DIAGNOSTICS_ENABLED=false", // Disable diagnostics
		"-e", "DB_TYPE=postgresdb",
		"-e", "DB_POSTGRESDB_HOST=n8n-postgres",
		"-e", "DB_POSTGRESDB_PORT=5432",
		"-e", "DB_POSTGRESDB_DATABASE=n8n",
		"-e", "DB_POSTGRESDB_USER=n8n",
		"-e", "DB_POSTGRESDB_PASSWORD=n8n",
		"-v", "n8n_data:/home/node/.n8n",
		"-v", fmt.Sprintf("%s:/home/node/workflows", absWorkFlowDir),
		"n8nio/n8n:latest",
	}
	cmd := exec.Command("podman", args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		log.Fatalf("Failed to start N8N: %v", err)
	}
	fmt.Printf("✅ N8N started successfully on http://localhost:%d\n", n8nPort)
}

func startPostgres() {
	if fun.IsContainerRunning("n8n-postgres") {
		fmt.Println("✅ N8N Postgres is already running")
		return
	}

	fmt.Println("🚀 Starting N8N Postgres database...")
	args := []string{
		"run", "-d", "--name", "n8n-postgres", "--replace",
		"--network", "n8n-net",
		"-e", "POSTGRES_DB=n8n",
		"-e", "POSTGRES_USER=n8n",
		"-e", "POSTGRES_PASSWORD=n8n",
		"-v", "n8n_postgres_data:/var/lib/postgresql/data",
		"postgres:16-alpine",
	}
	cmd := exec.Command("podman", args...)
	if err := cmd.Run(); err != nil {
		log.Fatalf("Failed to start N8N Postgres: %v", err)
	}
	// Give Postgres some time to start
	exec.Command("sleep", "5").Run()
}

func stopN8n() {
	fmt.Println("🛑 Stopping N8N...")
	if err := fun.StopContainer("service-platform-n8n"); err != nil {
		log.Printf("Failed to stop N8N: %v", err)
	}
	if err := fun.StopContainer("n8n-postgres"); err != nil {
		log.Printf("Failed to stop N8N Postgres: %v", err)
	}
	fmt.Println("✅ N8N stopped successfully")
}

func printHelp() {
	fmt.Println("Service Platform - N8N Workflow Automation Tool")
	fmt.Println()
	fmt.Println("Usage:")
	fmt.Println("  n8n [command]")
	fmt.Println()
	fmt.Println("Commands:")
	fmt.Println("  --start          Start N8N service")
	fmt.Println("  --stop           Stop N8N service")
	fmt.Println("  --help, -h       Show this help message")
	fmt.Println()
	fmt.Println("Examples:")
	fmt.Println("  n8n --start")
	fmt.Println("  n8n --stop")
}
