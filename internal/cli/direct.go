package cli

import (
	"fmt"
	"os"
	"os/exec"
	"sort"
	"strings"
)

// RunDirect executes a make target non-interactively, or handles meta-commands
// like "list" and "help". Returns exit code.
//
// Usage from scripts / CI:
//
//	./bin/cli run-api          # runs `make run-api` in foreground
//	./bin/cli build-api        # runs `make build-api`
//	./bin/cli list             # lists every available target
//	./bin/cli list build       # lists targets matching "build"
//	./bin/cli help             # prints usage
func RunDirect(args []string) int {
	if len(args) == 0 || args[0] == "help" || args[0] == "--help" || args[0] == "-h" {
		printUsage()
		return 0
	}

	if args[0] == "list" {
		filter := ""
		if len(args) > 1 {
			filter = strings.ToLower(strings.Join(args[1:], " "))
		}
		printList(filter)
		return 0
	}

	// Build target index
	cats := allCategories()
	index := buildTargetIndex(cats)

	target := args[0]
	info, ok := index[target]
	if !ok {
		fmt.Fprintf(os.Stderr, "❌ Unknown target: %s\n\n", target)
		if suggestions := findSuggestions(target, index); len(suggestions) > 0 {
			fmt.Fprintln(os.Stderr, "Did you mean:")
			for _, s := range suggestions {
				fmt.Fprintf(os.Stderr, "  • %s\n", s)
			}
			fmt.Fprintln(os.Stderr)
		}
		fmt.Fprintln(os.Stderr, "Run './bin/cli list' to see all available targets.")
		return 1
	}

	makeCmd := findMakeCommand()
	if makeCmd == "" {
		fmt.Fprintln(os.Stderr, "❌ 'make' not found in PATH")
		return 1
	}

	// Confirm dangerous commands when stdin is a terminal
	if info.dangerous {
		if !confirmDangerous(info.name, target) {
			fmt.Println("Aborted.")
			return 0
		}
	}

	fmt.Printf("▶ make %s\n", target)
	return execForeground(makeCmd, target)
}

// ── Target index ────────────────────────────────────────────────────────────

// targetInfo holds metadata about a Makefile target, used for lookups in direct mode (e.g. when running `cli build-api`).
type targetInfo struct {
	name      string // Human-readable name (e.g. "API Server")
	desc      string // Description
	cat       string // Category name
	icon      string // Category icon
	dangerous bool
}

// buildTargetIndex creates a map from Makefile targets to their associated info (name, description, category, etc.) for quick lookup when running in direct mode.
func buildTargetIndex(cats []Category) map[string]targetInfo {
	idx := make(map[string]targetInfo, 80)
	for _, cat := range cats {
		for _, item := range cat.Items {
			idx[item.MakeTarget] = targetInfo{
				name:      item.Name,
				desc:      item.Description,
				cat:       cat.Name,
				icon:      cat.Icon,
				dangerous: item.Dangerous,
			}
		}
	}
	return idx
}

// ── Suggestions ─────────────────────────────────────────────────────────────

// findSuggestions returns up to 5 targets whose name contains the query as a
// substring (case-insensitive).
func findSuggestions(query string, index map[string]targetInfo) []string {
	q := strings.ToLower(query)
	var matches []string
	for target := range index {
		if strings.Contains(strings.ToLower(target), q) {
			matches = append(matches, target)
		}
	}
	sort.Strings(matches)
	if len(matches) > 5 {
		matches = matches[:5]
	}
	return matches
}

// ── Dangerous command confirmation ──────────────────────────────────────────

// confirmDangerous prompts the user to confirm execution of a dangerous command.
func confirmDangerous(name, target string) bool {
	fmt.Printf("⚠️  %s (%s) is a dangerous action.\n", name, target)
	fmt.Print("Are you sure? [y/N]: ")
	var answer string
	fmt.Scanln(&answer) //nolint:errcheck
	answer = strings.TrimSpace(strings.ToLower(answer))
	return answer == "y" || answer == "yes"
}

// ── Foreground exec ─────────────────────────────────────────────────────────

// execForeground runs `make <target>` with inherited stdin/stdout/stderr and
// returns its exit code.
func execForeground(makeCmd, target string) int {
	cmd := exec.Command(makeCmd, target) //nolint:gosec
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		if cmd.ProcessState != nil {
			return cmd.ProcessState.ExitCode()
		}
		return 1
	}
	return 0
}

// ── List / Help ─────────────────────────────────────────────────────────────

// printList outputs a list of available targets to stdout, optionally filtered by a search query.
func printList(filter string) {
	cats := allCategories()
	count := 0

	for _, cat := range cats {
		var matched []MenuItem
		for _, item := range cat.Items {
			if filter == "" || matchesFilter(filter, item, cat) {
				matched = append(matched, item)
			}
		}
		if len(matched) == 0 {
			continue
		}
		fmt.Printf("\n%s %s\n", cat.Icon, strings.TrimSpace(cat.Name))
		for _, item := range matched {
			fmt.Printf("  %-28s %s\n", item.MakeTarget, item.Description)
			count++
		}
	}

	fmt.Printf("\n%d target(s)", count)
	if filter != "" {
		fmt.Printf(" matching %q", filter)
	}
	fmt.Println()
}

// matchesFilter checks if the filter string is a case-insensitive substring of the target's MakeTarget, Name, Description, or Category name.
func matchesFilter(filter string, item MenuItem, cat Category) bool {
	q := strings.ToLower(filter)
	return strings.Contains(strings.ToLower(item.MakeTarget), q) ||
		strings.Contains(strings.ToLower(item.Name), q) ||
		strings.Contains(strings.ToLower(item.Description), q) ||
		strings.Contains(strings.ToLower(cat.Name), q)
}

// printUsage prints usage instructions for the CLI.
func printUsage() {
	fmt.Println(`Usage: cli [command]

Interactive mode (TUI):
  cli                       Launch the interactive terminal UI

Non-interactive mode:
  cli <target>              Run a make target directly
  cli list [filter]         List available targets (optionally filtered)
  cli help                  Show this help message

Examples:
  cli run-api               Start the API server
  cli build-api             Build the API binary
  cli list build            Show all build targets
  cli migrate-up            Run database migrations

Run 'cli list' to see every available target.`)
}
