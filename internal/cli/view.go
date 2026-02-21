package cli

import (
	"fmt"
	"strings"
)

// ── View (renders the current state) ────────────────────────────────────────

// View renders the current UI based on the state. It delegates to specific render methods for each view (categories, commands, running output, confirm dialog). The output is a string that Bubbletea will display in the terminal.
func (m Model) View() string {
	if m.width == 0 {
		return "  Initializing...\n"
	}

	switch m.state {
	case viewCategories:
		return m.renderCategories()
	case viewCommands:
		return m.renderCommands()
	case viewRunning:
		return m.renderRunning()
	case viewConfirm:
		return m.renderConfirm()
	case viewSearch:
		return m.renderSearch()
	}
	return ""
}

// ── Category list ───────────────────────────────────────────────────────────

// renderCategories builds the string output for the category selection view. It highlights the currently selected category and shows a brief description for each. The user can navigate with arrow keys and select a category to view its commands.
func (m Model) renderCategories() string {
	var s strings.Builder

	s.WriteString("\n")
	s.WriteString(titleStyle.Render("  🚀 Service Platform CLI"))
	s.WriteString("\n")
	s.WriteString(subtitleStyle.Render("     Interactive development toolkit"))
	s.WriteString("\n\n")

	for i, cat := range m.cats {
		label := fmt.Sprintf("%s  %s", cat.Icon, cat.Name)
		desc := descStyle.Render("  " + cat.Description)

		if i == m.catIdx {
			s.WriteString(fmt.Sprintf("  %s %s%s\n",
				cursorStyle.Render("▸"),
				selectedItemStyle.Render(label),
				desc,
			))
		} else {
			s.WriteString(fmt.Sprintf("    %s%s\n",
				normalItemStyle.Render(label),
				desc,
			))
		}
	}

	s.WriteString("\n")
	s.WriteString(renderHelp(
		helpPair{"↑/↓", "navigate"},
		helpPair{"enter", "select"},
		helpPair{"/", "search"},
		helpPair{"q", "quit"},
	))
	s.WriteString("\n")

	return s.String()
}

// ── Command list (inside a category) ────────────────────────────────────────

// renderCommands builds the string output for the command selection view within a category. It shows each command with its description, highlights the current selection, and indicates if it's dangerous. For multi-select categories, it also shows checkboxes and a count of selected items.
func (m Model) renderCommands() string {
	var s strings.Builder
	cat := m.cats[m.activeCat]

	// Breadcrumb header
	s.WriteString("\n")
	s.WriteString(subtitleStyle.Render(fmt.Sprintf("  ← %s  %s", cat.Icon, cat.Name)))
	s.WriteString("\n\n")

	for i, item := range cat.Items {
		cursor := "    "
		nameStyle := normalItemStyle
		if i == m.cmdIdx {
			cursor = "  " + cursorStyle.Render("▸") + " "
			nameStyle = selectedItemStyle
		}

		// Checkbox (multi-select only)
		checkbox := ""
		if cat.MultiSelect {
			if m.selected[i] {
				checkbox = checkboxOnStyle.Render("[✓] ")
			} else {
				checkbox = checkboxOffStyle.Render("[ ] ")
			}
		}

		// Name + optional danger marker
		name := nameStyle.Render(item.Name)
		if item.Dangerous {
			name = dangerStyle.Render(item.Name + " ⚠")
		}

		desc := descStyle.Render("  " + item.Description)

		s.WriteString(fmt.Sprintf("%s%s%s%s\n", cursor, checkbox, name, desc))
	}

	// Show selection count
	if cat.MultiSelect && len(m.selected) > 0 {
		s.WriteString("\n")
		s.WriteString(countStyle.Render(fmt.Sprintf("    %d selected", len(m.selected))))
		s.WriteString("\n")
	}

	// Help bar
	s.WriteString("\n")
	entries := []helpPair{{"↑/↓", "navigate"}}
	if cat.MultiSelect {
		entries = append(entries, helpPair{"space", "toggle"}, helpPair{"a", "all"})
	}
	entries = append(entries, helpPair{"enter", "run"}, helpPair{"/", "search"}, helpPair{"esc", "back"})
	s.WriteString(renderHelp(entries...))
	s.WriteString("\n")

	return s.String()
}

// ── Running view (shows command output) ─────────────────────────────────────

// renderRunning builds the string output for the running view, which displays real-time output from the executing command. It shows a spinner and progress indicator while running, auto-scrolls the output, and provides help options once done.
func (m Model) renderRunning() string {
	var s strings.Builder

	s.WriteString("\n")

	// Spinner header while executing
	if m.running {
		target := m.targets[m.targetIdx]
		progress := ""
		if len(m.targets) > 1 {
			progress = fmt.Sprintf(" [%d/%d]", m.targetIdx+1, len(m.targets))
		}
		s.WriteString(warningStyle.Render(
			fmt.Sprintf("  %s Running: make %s%s", m.spinner.View(), target, progress),
		))
		s.WriteString("\n")
	}

	s.WriteString("\n")

	// Output lines – auto-scroll to bottom
	maxLines := m.height - 8
	if maxLines < 5 {
		maxLines = 5
	}
	lines := m.outputLines
	start := 0
	if len(lines) > maxLines {
		start = len(lines) - maxLines
	}
	for _, line := range lines[start:] {
		s.WriteString("  " + line + "\n")
	}

	// Help (only when done)
	if !m.running {
		s.WriteString("\n")
		s.WriteString(renderHelp(
			helpPair{"esc/enter", "back"},
			helpPair{"q", "quit"},
		))
		s.WriteString("\n")
	}

	return s.String()
}

// ── Confirm dialog ──────────────────────────────────────────────────────────

// renderConfirm builds the string output for the confirmation dialog when a dangerous command is selected. It shows the command details and asks the user to confirm with y/n before proceeding.
func (m Model) renderConfirm() string {
	var s strings.Builder

	s.WriteString("\n")

	content := fmt.Sprintf(
		"%s\n\n%s\n%s\n\n%s",
		dangerStyle.Render("⚠  Confirm Dangerous Action"),
		normalItemStyle.Render(m.confirmItem.Name),
		descStyle.Render("Command: make "+m.confirmItem.MakeTarget),
		warningStyle.Render("Proceed? ")+helpKeyStyle.Render("y")+" / "+helpKeyStyle.Render("n"),
	)

	s.WriteString(confirmBoxStyle.Render(content))
	s.WriteString("\n")

	return s.String()
}

// ── Search view ─────────────────────────────────────────────────────────────

// renderSearch builds the string output for the global search view. It shows a text input at the top and a filtered list of all commands across categories. Each result shows the category badge and command details, with match count.
func (m Model) renderSearch() string {
	var s strings.Builder

	s.WriteString("\n")
	s.WriteString(titleStyle.Render("  🔍 Search Commands"))
	s.WriteString("\n\n")

	// Search input bar
	cursor := searchInputStyle.Render("█")
	queryDisplay := searchInputStyle.Render(m.searchQuery)
	s.WriteString(fmt.Sprintf("  %s %s%s\n",
		searchPromptStyle.Render("/"),
		queryDisplay,
		cursor,
	))
	s.WriteString("\n")

	// Result count
	total := m.totalCommandCount()
	if m.searchQuery == "" {
		s.WriteString(descStyle.Render(fmt.Sprintf("  Showing all %d commands", total)))
	} else {
		s.WriteString(descStyle.Render(fmt.Sprintf("  %d of %d commands match", len(m.searchResults), total)))
	}
	s.WriteString("\n\n")

	// Results list (scrollable)
	if len(m.searchResults) == 0 {
		s.WriteString(descStyle.Render("  No commands found matching your query."))
		s.WriteString("\n")
	} else {
		maxVisible := m.height - 12
		if maxVisible < 3 {
			maxVisible = 3
		}

		// Calculate scroll window
		start := 0
		if m.searchIdx >= maxVisible {
			start = m.searchIdx - maxVisible + 1
		}
		end := start + maxVisible
		if end > len(m.searchResults) {
			end = len(m.searchResults)
		}

		// Show scroll indicator if needed
		if start > 0 {
			s.WriteString(descStyle.Render("  ↑ more results above"))
			s.WriteString("\n")
		}

		for i := start; i < end; i++ {
			r := m.searchResults[i]

			cursor := "    "
			nameStyle := normalItemStyle
			if i == m.searchIdx {
				cursor = "  " + cursorStyle.Render("▸") + " "
				nameStyle = selectedItemStyle
			}

			// Category badge
			badge := searchCatBadgeStyle.Render(fmt.Sprintf("[%s %s]", r.catIcon, strings.TrimSpace(r.catName)))

			// Name + danger marker
			name := nameStyle.Render(r.item.Name)
			if r.item.Dangerous {
				name = dangerStyle.Render(r.item.Name + " ⚠")
			}

			desc := descStyle.Render("  " + r.item.Description)

			s.WriteString(fmt.Sprintf("%s%s %s%s\n", cursor, badge, name, desc))
		}

		// Show scroll indicator if needed
		if end < len(m.searchResults) {
			s.WriteString(descStyle.Render(fmt.Sprintf("  ↓ %d more results below", len(m.searchResults)-end)))
			s.WriteString("\n")
		}
	}

	// Help bar
	s.WriteString("\n")
	s.WriteString(renderHelp(
		helpPair{"type", "filter"},
		helpPair{"↑/↓", "navigate"},
		helpPair{"enter", "run"},
		helpPair{"ctrl+u", "clear"},
		helpPair{"esc", "back"},
	))
	s.WriteString("\n")

	return s.String()
}

// totalCommandCount returns the total number of commands across all categories.
func (m Model) totalCommandCount() int {
	n := 0
	for _, cat := range m.cats {
		n += len(cat.Items)
	}
	return n
}

// ── Help bar rendering ──────────────────────────────────────────────────────

// helpPair is a simple struct to hold a key and its description for rendering the help bar.
type helpPair struct {
	key  string // the key or key combination (e.g. "enter", "space", "↑/↓")
	desc string // the description of what the key does (e.g. "select", "toggle", "navigate")
}

// renderHelp takes a variable number of helpPair entries and formats them into a single string for the help bar at the bottom of the UI. Each key is styled, and entries are separated by a vertical bar.
func renderHelp(entries ...helpPair) string {
	parts := make([]string, len(entries))
	for i, e := range entries {
		parts[i] = helpKeyStyle.Render(e.key) + " " + helpDescStyle.Render(e.desc)
	}
	sep := helpSepStyle.Render(" │ ")
	return "  " + strings.Join(parts, sep)
}
