package cli

import "github.com/charmbracelet/lipgloss"

// ── Colour palette ──────────────────────────────────────────────────────────

var (
	purple   = lipgloss.Color("#A78BFA")
	cyan     = lipgloss.Color("#22D3EE")
	green    = lipgloss.Color("#34D399")
	red      = lipgloss.Color("#F87171")
	yellow   = lipgloss.Color("#FBBF24")
	gray     = lipgloss.Color("#9CA3AF")
	darkGray = lipgloss.Color("#4B5563")
)

// ── Reusable styles ─────────────────────────────────────────────────────────

var (
	// Header / title area
	titleStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(purple).
			Padding(0, 1)

	subtitleStyle = lipgloss.NewStyle().
			Foreground(gray)

	// List items
	cursorStyle = lipgloss.NewStyle().
			Foreground(cyan).
			Bold(true)

	selectedItemStyle = lipgloss.NewStyle().
				Foreground(cyan).
				Bold(true)

	normalItemStyle = lipgloss.NewStyle() // inherits terminal default fg

	descStyle = lipgloss.NewStyle().
			Foreground(darkGray)

	// Multi-select checkboxes
	checkboxOnStyle = lipgloss.NewStyle().
			Foreground(green)

	checkboxOffStyle = lipgloss.NewStyle().
				Foreground(darkGray)

	// Semantic colours
	dangerStyle = lipgloss.NewStyle().
			Foreground(red)

	successStyle = lipgloss.NewStyle().
			Foreground(green)

	warningStyle = lipgloss.NewStyle().
			Foreground(yellow)

	// Running / output area
	outputStyle = lipgloss.NewStyle().
			Foreground(gray)

	countStyle = lipgloss.NewStyle().
			Foreground(yellow).
			Bold(true)

	// Confirm dialog
	confirmBoxStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(red).
			Padding(1, 2).
			MarginTop(1).
			MarginLeft(2)

	// Help bar
	helpKeyStyle = lipgloss.NewStyle().
			Foreground(cyan)

	helpSepStyle = lipgloss.NewStyle().
			Foreground(darkGray)

	helpDescStyle = lipgloss.NewStyle().
			Foreground(gray)

	// Search bar
	searchPromptStyle = lipgloss.NewStyle().
				Foreground(cyan).
				Bold(true)

	searchInputStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("#FFFFFF"))

	searchMatchStyle = lipgloss.NewStyle().
				Foreground(yellow).
				Bold(true)

	searchCatBadgeStyle = lipgloss.NewStyle().
				Foreground(purple)
)
