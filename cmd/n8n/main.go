package main

import (
	"fmt"
	"log"
	"os"
	"os/exec"
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

	// Run N8N Podman container
	n8nPort := config.GetConfig().N8N.Port
	cmd := exec.Command("podman", "run", "-d", "--name", "n8n", "-p", fmt.Sprintf("%d:%d", n8nPort, n8nPort), "-e", fmt.Sprintf("N8N_PORT=%d", n8nPort), "-e", "N8N_METRICS=true", "-v", "n8n_data:/home/node/.n8n", "n8nio/n8n")
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		log.Fatalf("Failed to start N8N: %v", err)
	}
	fmt.Printf("✅ N8N started successfully on http://localhost:%d\n", n8nPort)
}

func stopN8n() {
	fmt.Println("🛑 Stopping N8N...")
	if err := fun.StopContainer("n8n"); err != nil {
		log.Printf("Failed to stop N8N: %v", err)
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
