package cli

import (
	"fmt"
	"os/exec"
	"runtime"
	"strings"

	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
)

// ── View states ─────────────────────────────────────────────────────────────

// viewState represents the current screen/view of the TUI (categories, commands, running output, confirmation dialog).
type viewState int

const (
	viewCategories     viewState = iota // Category list view
	viewCommands                        // Command list for selected category
	viewParameterInput                  // Parameter input for commands that need them
	viewRunning                         // Running command output view
	viewConfirm                         // Confirmation dialog for dangerous actions
	viewSearch                          // Global search across all commands
)

// ── Tea messages ────────────────────────────────────────────────────────────

// commandResultMsg is returned when a one-shot make target finishes.
type commandResultMsg struct {
	output   string // Captured stdout+stderr from the command
	exitCode int    // Exit code of the command (0 for success)
	target   string // The make target that was run (for reference)
}

// execDoneMsg is returned when a long-running ExecProcess finishes.
type execDoneMsg struct{ err error }

// ── Model ───────────────────────────────────────────────────────────────────

// searchResult represents a single match in the global search results.
type searchResult struct {
	catIdx  int      // index into m.cats
	itemIdx int      // index into category Items
	item    MenuItem // the matched command
	catName string   // category display name (for breadcrumb)
	catIcon string   // category icon
}

// Model is the top-level Bubbletea model for the CLI.
type Model struct {
	state     viewState    // current view/screen
	cats      []Category   // all command categories
	catIdx    int          // cursor in category list
	cmdIdx    int          // cursor in command list
	activeCat int          // which category is open
	selected  map[int]bool // multi-select toggles

	// Execution state
	running     bool     // is a command currently running?
	targets     []string // queue of make targets to run
	targetIdx   int      // index into targets
	outputLines []string // accumulated output lines
	allSuccess  bool     // did all commands succeed?

	// Confirm dialog
	confirmItem MenuItem

	// Parameter input state
	parameterItem   MenuItem // Item that needs parameter input
	parameterValue  string   // User's entered parameter value
	parameterCursor int      // Cursor position in input field

	// Search state
	searchQuery   string         // current search input text
	searchResults []searchResult // filtered results matching query
	searchIdx     int            // cursor position within search results
	prevState     viewState      // state to return to on esc

	// UI components
	spinner spinner.Model // spinner for visual feedback during command execution
	makeCmd string        // resolved path to make binary

	// Terminal dimensions
	width  int // current terminal width (for responsive layout)
	height int // current terminal height
}

// ── Public helpers ──────────────────────────────────────────────────────────

// CheckDependencies ensures `make` (or equivalent) is available.
func CheckDependencies() error {
	if findMakeCommand() != "" {
		return nil
	}
	switch runtime.GOOS {
	case "windows":
		return fmt.Errorf("'make' not found. Install via:\n  - choco install make\n  - scoop install make\n  - Or use MSYS2 / Git Bash")
	case "darwin":
		return fmt.Errorf("'make' not found. Install via: xcode-select --install")
	default:
		return fmt.Errorf("'make' not found. Install via your package manager (e.g. apt install make)")
	}
}

// renderHelp formats a list of key/description pairs into a help string.
func findMakeCommand() string {
	candidates := []string{"make"}
	if runtime.GOOS == "windows" {
		candidates = append(candidates, "mingw32-make", "gmake")
	}
	for _, c := range candidates {
		if _, err := exec.LookPath(c); err == nil {
			return c
		}
	}
	return ""
}

// ── Constructor ─────────────────────────────────────────────────────────────

// New returns an initialised Model ready for tea.NewProgram.
func New() Model {
	s := spinner.New()
	s.Spinner = spinner.Dot
	s.Style = warningStyle

	return Model{
		state:    viewCategories,
		cats:     allCategories(),
		selected: make(map[int]bool),
		spinner:  s,
		makeCmd:  findMakeCommand(),
	}
}

// ── Bubbletea interface ─────────────────────────────────────────────────────

// Init is the Bubbletea initialization function. It starts the spinner ticking, which is used for visual feedback during command execution. The main view will be rendered immediately, and the spinner will update as needed when running commands.
func (m Model) Init() tea.Cmd {
	return m.spinner.Tick
}

// Update handles incoming messages and updates the model state accordingly. It routes messages to specific handlers based on the current view state (categories, commands, running, confirm). It also handles global messages like window resizing and spinner ticks.
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		return m, nil

	case tea.KeyMsg:
		return m.handleKey(msg)

	case spinner.TickMsg:
		var cmd tea.Cmd
		m.spinner, cmd = m.spinner.Update(msg)
		if m.running {
			return m, cmd
		}
		return m, nil

	case commandResultMsg:
		return m.handleResult(msg)

	case execDoneMsg:
		m.state = viewCommands
		return m, nil
	}

	return m, nil
}

// ── Key dispatch ────────────────────────────────────────────────────────────

// handleKey routes key input to the appropriate handler based on current view state (categories, commands, running, confirm, search). It also handles global keys like Ctrl+C to quit. Each view has its own key handler for context-specific actions.
func (m Model) handleKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if msg.String() == "ctrl+c" {
		return m, tea.Quit
	}
	switch m.state {
	case viewCategories:
		return m.keyCategories(msg.String())
	case viewCommands:
		return m.keyCommands(msg.String())
	case viewParameterInput:
		return m.keyParameterInput(msg.String())
	case viewRunning:
		return m.keyRunning(msg.String())
	case viewConfirm:
		return m.keyConfirm(msg.String())
	case viewSearch:
		return m.keySearch(msg)
	}
	return m, nil
}

// ── Categories ──────────────────────────────────────────────────────────────

// keyCategories handles key input when viewing the category list (navigate, select, search).
func (m Model) keyCategories(key string) (tea.Model, tea.Cmd) {
	switch key {
	case "q", "esc":
		return m, tea.Quit
	case "up", "k":
		if m.catIdx > 0 {
			m.catIdx--
		}
	case "down", "j":
		if m.catIdx < len(m.cats)-1 {
			m.catIdx++
		}
	case "home", "g":
		m.catIdx = 0
	case "end", "G":
		m.catIdx = len(m.cats) - 1
	case "/":
		return m.enterSearch(viewCategories)
	case "enter":
		m.activeCat = m.catIdx
		m.cmdIdx = 0
		m.selected = make(map[int]bool)
		m.state = viewCommands
	}
	return m, nil
}

// ── Commands ────────────────────────────────────────────────────────────────

// keyCommands handles key input when viewing a category's command list (navigate, multi-select, execute, search).
func (m Model) keyCommands(key string) (tea.Model, tea.Cmd) {
	cat := m.cats[m.activeCat]

	switch key {
	case "esc":
		m.state = viewCategories
		return m, nil
	case "q":
		return m, tea.Quit
	case "/":
		return m.enterSearch(viewCommands)
	case "up", "k":
		if m.cmdIdx > 0 {
			m.cmdIdx--
		}
	case "down", "j":
		if m.cmdIdx < len(cat.Items)-1 {
			m.cmdIdx++
		}
	case "home", "g":
		m.cmdIdx = 0
	case "end", "G":
		m.cmdIdx = len(cat.Items) - 1
	case " ":
		if cat.MultiSelect {
			if m.selected[m.cmdIdx] {
				delete(m.selected, m.cmdIdx)
			} else {
				m.selected[m.cmdIdx] = true
			}
		}
	case "a":
		if cat.MultiSelect {
			if len(m.selected) == len(cat.Items) {
				m.selected = make(map[int]bool)
			} else {
				for i := range cat.Items {
					m.selected[i] = true
				}
			}
		}
	case "enter":
		return m.execSelected()
	}
	return m, nil
}

// execSelected figures out what to run based on selection / cursor.
func (m Model) execSelected() (tea.Model, tea.Cmd) {
	cat := m.cats[m.activeCat]

	// Multi-select: batch-run all selected
	if cat.MultiSelect && len(m.selected) > 0 {
		var targets []string
		for i := range cat.Items {
			if m.selected[i] {
				targets = append(targets, cat.Items[i].MakeTarget)
			}
		}
		return m.runBatch(targets)
	}

	item := cat.Items[m.cmdIdx]

	// Parameter input → collect parameter first
	if item.NeedsParameter {
		m.parameterItem = item
		m.parameterValue = ""
		m.parameterCursor = 0
		m.state = viewParameterInput
		return m, nil
	}

	// Dangerous → confirm first
	if item.Dangerous {
		m.confirmItem = item
		m.state = viewConfirm
		return m, nil
	}

	// Long-running → hand off terminal
	if item.LongRunning {
		c := exec.Command(m.makeCmd, item.MakeTarget) //nolint:gosec
		return m, tea.ExecProcess(c, func(err error) tea.Msg {
			return execDoneMsg{err: err}
		})
	}

	// One-shot command
	return m.runBatch([]string{item.MakeTarget})
}

// runBatch starts executing a slice of make targets sequentially.
func (m Model) runBatch(targets []string) (tea.Model, tea.Cmd) {
	m.state = viewRunning
	m.running = true
	m.targets = targets
	m.targetIdx = 0
	m.allSuccess = true
	m.outputLines = []string{
		warningStyle.Render("▶ make " + targets[0]),
		"",
	}
	return m, tea.Batch(m.spinner.Tick, m.runMake(targets[0]))
}

// ── Command result handling ─────────────────────────────────────────────────

// handleResult processes output from a finished command, updates the output log, and starts the next command if applicable.
func (m Model) handleResult(msg commandResultMsg) (tea.Model, tea.Cmd) {
	// Append captured output
	if msg.output != "" {
		for _, line := range strings.Split(strings.TrimRight(msg.output, "\n"), "\n") {
			m.outputLines = append(m.outputLines, outputStyle.Render(line))
		}
	}

	// Status line for this target
	if msg.exitCode == 0 {
		m.outputLines = append(m.outputLines, successStyle.Render("  ✓ Success"))
	} else {
		m.allSuccess = false
		m.outputLines = append(m.outputLines, dangerStyle.Render(
			fmt.Sprintf("  ✗ Failed (exit %d)", msg.exitCode)))
	}

	// Next target?
	m.targetIdx++
	if m.targetIdx < len(m.targets) {
		next := m.targets[m.targetIdx]
		m.outputLines = append(m.outputLines, "", warningStyle.Render("▶ make "+next), "")
		return m, tea.Batch(m.spinner.Tick, m.runMake(next))
	}

	// All done
	m.running = false
	m.outputLines = append(m.outputLines, "")
	if len(m.targets) > 1 {
		if m.allSuccess {
			m.outputLines = append(m.outputLines, successStyle.Render("✓ All tasks completed"))
		} else {
			m.outputLines = append(m.outputLines, dangerStyle.Render("✗ Some tasks failed"))
		}
	}
	return m, nil
}

// ── Parameter input ────────────────────────────────────────────────────────

// keyParameterInput handles text input and submission for commands that need parameters.
func (m Model) keyParameterInput(key string) (tea.Model, tea.Cmd) {
	switch key {
	case "esc":
		// Cancel parameter input, go back to command list
		m.state = viewCommands
		return m, nil
	case "enter":
		// Submit the parameter value
		if m.parameterValue != "" {
			// Build the target with parameter (e.g., "revive PKG='./cmd/api'")
			target := fmt.Sprintf("%s %s='%s'", m.parameterItem.MakeTarget, m.parameterItem.ParameterName, m.parameterValue)
			return m.runBatch([]string{target})
		}
		// Empty input - require at least something
		return m, nil
	case "backspace":
		if m.parameterCursor > 0 {
			m.parameterValue = m.parameterValue[:m.parameterCursor-1] + m.parameterValue[m.parameterCursor:]
			m.parameterCursor--
		}
	default:
		// Regular character input
		if len(key) == 1 {
			m.parameterValue = m.parameterValue[:m.parameterCursor] + key + m.parameterValue[m.parameterCursor:]
			m.parameterCursor++
		}
	}
	return m, nil
}

// ── Running / Confirm key handlers ──────────────────────────────────────────

// keyRunning handles key input while a command is running (mainly to quit or go back when done).
func (m Model) keyRunning(key string) (tea.Model, tea.Cmd) {
	if !m.running {
		switch key {
		case "esc", "enter":
			m.state = viewCommands
			m.outputLines = nil
			return m, nil
		case "q":
			return m, tea.Quit
		}
	}
	return m, nil
}

// keyConfirm handles y/n input in the confirmation dialog.
func (m Model) keyConfirm(key string) (tea.Model, tea.Cmd) {
	switch key {
	case "y", "Y":
		item := m.confirmItem
		if item.LongRunning {
			c := exec.Command(m.makeCmd, item.MakeTarget) //nolint:gosec
			return m, tea.ExecProcess(c, func(err error) tea.Msg {
				return execDoneMsg{err: err}
			})
		}
		return m.runBatch([]string{item.MakeTarget})
	case "n", "N", "esc":
		m.state = viewCommands
	}
	return m, nil
}

// ── Search ──────────────────────────────────────────────────────────────────

// enterSearch transitions into global search mode, remembering the previous view so esc can return to it.
func (m Model) enterSearch(from viewState) (Model, tea.Cmd) {
	m.prevState = from
	m.state = viewSearch
	m.searchQuery = ""
	m.searchIdx = 0
	m.searchResults = m.filterCommands("")
	return m, nil
}

// keySearch handles all key input in search mode: typing, navigation, execution, and escape.
func (m Model) keySearch(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	key := msg.String()

	switch key {
	case "esc":
		m.state = m.prevState
		m.searchQuery = ""
		m.searchResults = nil
		return m, nil

	case "up", "ctrl+k":
		if m.searchIdx > 0 {
			m.searchIdx--
		}
		return m, nil

	case "down", "ctrl+j":
		if m.searchIdx < len(m.searchResults)-1 {
			m.searchIdx++
		}
		return m, nil

	case "ctrl+u":
		// Clear search query
		m.searchQuery = ""
		m.searchIdx = 0
		m.searchResults = m.filterCommands("")
		return m, nil

	case "backspace":
		if len(m.searchQuery) > 0 {
			m.searchQuery = m.searchQuery[:len(m.searchQuery)-1]
			m.searchIdx = 0
			m.searchResults = m.filterCommands(m.searchQuery)
		}
		return m, nil

	case "enter":
		if len(m.searchResults) == 0 {
			return m, nil
		}
		result := m.searchResults[m.searchIdx]
		item := result.item

		// Dangerous → confirm first
		if item.Dangerous {
			m.confirmItem = item
			m.state = viewConfirm
			return m, nil
		}

		// Long-running → hand off terminal
		if item.LongRunning {
			c := exec.Command(m.makeCmd, item.MakeTarget) //nolint:gosec
			return m, tea.ExecProcess(c, func(err error) tea.Msg {
				return execDoneMsg{err: err}
			})
		}

		// One-shot command
		return m.runBatch([]string{item.MakeTarget})

	default:
		// Only accept printable single characters for the search query
		if len(key) == 1 && key[0] >= 32 && key[0] <= 126 {
			m.searchQuery += key
			m.searchIdx = 0
			m.searchResults = m.filterCommands(m.searchQuery)
		}
		return m, nil
	}
}

// filterCommands returns all commands across all categories that match the query.
// Matching is case-insensitive against Name, Description, and MakeTarget.
func (m Model) filterCommands(query string) []searchResult {
	q := strings.ToLower(strings.TrimSpace(query))
	var results []searchResult

	for ci, cat := range m.cats {
		for ii, item := range cat.Items {
			if q == "" || matchesQuery(q, item, cat) {
				results = append(results, searchResult{
					catIdx:  ci,
					itemIdx: ii,
					item:    item,
					catName: cat.Name,
					catIcon: cat.Icon,
				})
			}
		}
	}
	return results
}

// matchesQuery checks if any of the item's fields or its category name contain the query substring.
func matchesQuery(q string, item MenuItem, cat Category) bool {
	fields := []string{
		strings.ToLower(item.Name),
		strings.ToLower(item.Description),
		strings.ToLower(item.MakeTarget),
		strings.ToLower(cat.Name),
	}
	for _, f := range fields {
		if strings.Contains(f, q) {
			return true
		}
	}
	return false
}

// ── Make command helper ─────────────────────────────────────────────────────

// runMake returns a tea.Cmd that executes `make <target>` and captures output.
func (m Model) runMake(target string) tea.Cmd {
	makeCmd := m.makeCmd
	return func() tea.Msg {
		cmd := exec.Command(makeCmd, target) //nolint:gosec
		output, _ := cmd.CombinedOutput()
		exitCode := 0
		if cmd.ProcessState != nil {
			exitCode = cmd.ProcessState.ExitCode()
		}
		return commandResultMsg{
			output:   string(output),
			exitCode: exitCode,
			target:   target,
		}
	}
}
