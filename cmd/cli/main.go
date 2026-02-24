// Package main is the entry point for the service-platform interactive CLI.
//
// When invoked without arguments it launches a full-screen TUI (built with
// Bubbletea) for managing the service-platform: WhatsApp integration, database
// operations, configuration, and more. When invoked with arguments it dispatches
// to the corresponding command in non-interactive (direct) mode.
//
// Build:
//
//	make build-cli   # produces ./bin/cli
//
// Usage:
//
//	./bin/cli                 # interactive TUI
//	./bin/cli <command> ...   # non-interactive mode
//	go run cmd/cli/main.go    # run without building
package main

import (
	"fmt"
	"os"

	"service-platform/internal/cli"

	tea "github.com/charmbracelet/bubbletea"
)

// main checks runtime dependencies, then either launches the interactive TUI
// (no arguments) or delegates to cli.RunDirect for non-interactive execution.
func main() {
	if err := cli.CheckDependencies(); err != nil {
		fmt.Fprintf(os.Stderr, "❌ %v\n", err)
		os.Exit(1)
	}

	// Non-interactive mode: if any arguments are passed, run directly.
	if len(os.Args) > 1 {
		os.Exit(cli.RunDirect(os.Args[1:]))
	}

	// Interactive mode: launch TUI.
	p := tea.NewProgram(cli.New(), tea.WithAltScreen())
	if _, err := p.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}
