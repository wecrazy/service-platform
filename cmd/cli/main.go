package main

import (
	"fmt"
	"os"

	"service-platform/internal/cli"

	tea "github.com/charmbracelet/bubbletea"
)

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
