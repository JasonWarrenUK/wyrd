package tui

import (
	"strings"

	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/jasonwarrenuk/wyrd/internal/types"
)

// CommandFunc is the function executed when a palette command is invoked.
// args contains the tokens after the command name.
type CommandFunc func(args []string) tea.Cmd

// Command represents a single entry in the command palette.
type Command struct {
	// Name is the primary command token (e.g. "quit", "theme").
	Name string

	// Description is shown next to the name in the palette.
	Description string

	// Hint is the keyboard shortcut, if any.
	Hint string

	// Execute is called when the command is confirmed.
	Execute CommandFunc
}

// PaletteMode distinguishes between the typed command mode and the fuzzy
// search mode opened with Ctrl+K.
type PaletteMode int

const (
	// PaletteModeCLI is the ":" triggered mode — behaves like a command line.
	PaletteModeCLI PaletteMode = iota

	// PaletteModeFuzzy is the Ctrl+K triggered mode — shows a filtered list.
	PaletteModeFuzzy
)

// PaletteState holds all mutable state for the command palette overlay.
// It is embedded in the root App model and is zero-value safe (inactive).
type PaletteState struct {
	active   bool
	mode     PaletteMode
	input    textinput.Model
	commands []Command
	filtered []Command
	cursor   int
	theme    *ActiveTheme

	// index is the in-memory graph, used for node and edge search in fuzzy mode.
	// May be nil; search degrades to commands-only when absent.
	index types.GraphIndex

	// results holds the unified search results for the current fuzzy query.
	// confirm() reads from this; filtered is populated from it for View().
	results []SearchResult
}

// NewPaletteState builds an inactive PaletteState pre-loaded with the
// built-in commands. Additional commands are registered via Register.
// index may be nil; search degrades to commands-only when absent.
func NewPaletteState(theme *ActiveTheme, index types.GraphIndex) PaletteState {
	ti := textinput.New()
	ti.Prompt = ": "
	ti.CharLimit = 256

	ps := PaletteState{
		input:    ti,
		theme:    theme,
		commands: builtinCommands(),
		index:    index,
	}
	return ps
}

// builtinCommands returns the initial command set for the palette.
func builtinCommands() []Command {
	return []Command{
		{
			Name:        "quit",
			Description: "Exit Wyrd",
			Hint:        "ctrl+c",
			Execute: func(_ []string) tea.Cmd {
				return tea.Quit
			},
		},
		{
			Name:        "theme",
			Description: "Switch to a named theme (e.g. theme peat)",
			Hint:        "",
			Execute:     nil, // wired up in app.go via RegisterCommand
		},
		{
			Name:        "sync",
			Description: "Sync with remote",
			Hint:        "",
			Execute:     nil, // placeholder for Phase 4
		},
		{
			Name:        "help",
			Description: "Show help overlay",
			Hint:        "?",
			Execute:     nil, // placeholder for Phase 4
		},
	}
}

// Register adds a command to the palette. If a command with the same name
// already exists it is replaced.
func (ps *PaletteState) Register(cmd Command) {
	for i, c := range ps.commands {
		if c.Name == cmd.Name {
			ps.commands[i] = cmd
			return
		}
	}
	ps.commands = append(ps.commands, cmd)
}

// Open activates the palette in the given mode and clears any previous input.
func (ps *PaletteState) Open(mode PaletteMode) {
	ps.active = true
	ps.mode = mode
	ps.cursor = 0
	ps.input.Reset()
	if mode == PaletteModeFuzzy {
		ps.input.Prompt = "  "
		ps.input.Placeholder = "Search nodes, edges, commands…"
	} else {
		ps.input.Prompt = ": "
		ps.input.Placeholder = ""
	}
	ps.input.Focus()
	if mode == PaletteModeFuzzy {
		ps.filterFuzzy("")
	} else {
		ps.filter("")
	}
}

// Close deactivates the palette.
func (ps *PaletteState) Close() {
	ps.active = false
	ps.input.Blur()
}

// IsActive reports whether the palette overlay is visible.
func (ps *PaletteState) IsActive() bool {
	return ps.active
}

// filterFuzzy runs a unified search across commands, nodes, and edges and
// populates both ps.results (for confirm dispatch) and ps.filtered (for
// View rendering until NV.17 replaces it with kind-aware row rendering).
func (ps *PaletteState) filterFuzzy(query string) {
	ps.results = searchAll(query, ps.commands, ps.index)

	// Populate ps.filtered with synthetic Command entries so View() works
	// without modification. NV.17 will replace this with direct results rendering.
	ps.filtered = ps.filtered[:0]
	for _, r := range ps.results {
		ps.filtered = append(ps.filtered, Command{
			Name:        r.Title,
			Description: r.Description,
			Hint:        r.Hint,
		})
	}
}

// resultCount returns the number of visible results for the current mode.
// In fuzzy mode this is len(ps.results); in CLI mode it is len(ps.filtered).
func (ps *PaletteState) resultCount() int {
	if ps.mode == PaletteModeFuzzy {
		return len(ps.results)
	}
	return len(ps.filtered)
}

// filter updates the filtered command list based on the current query.
func (ps *PaletteState) filter(query string) {
	query = strings.TrimSpace(strings.ToLower(query))
	if query == "" {
		ps.filtered = make([]Command, len(ps.commands))
		copy(ps.filtered, ps.commands)
		return
	}
	ps.filtered = ps.filtered[:0]
	for _, c := range ps.commands {
		if strings.Contains(strings.ToLower(c.Name), query) ||
			strings.Contains(strings.ToLower(c.Description), query) {
			ps.filtered = append(ps.filtered, c)
		}
	}
}

// Update handles keyboard input while the palette is active.
// Returns the updated state, an optional result command, and whether the
// palette should remain open.
func (ps PaletteState) Update(msg tea.Msg) (PaletteState, tea.Cmd, bool) {
	if !ps.active {
		return ps, nil, false
	}

	switch m := msg.(type) {
	case tea.KeyPressMsg:
		switch m.String() {
		case "esc", "ctrl+c":
			ps.Close()
			return ps, nil, false

		case "enter":
			cmd := ps.confirm()
			ps.Close()
			return ps, cmd, false

		case "up", "ctrl+p":
			if ps.cursor > 0 {
				ps.cursor--
			}
			return ps, nil, true

		case "down", "ctrl+n", "tab":
			if ps.cursor < ps.resultCount()-1 {
				ps.cursor++
			}
			return ps, nil, true

		default:
			var inputCmd tea.Cmd
			ps.input, inputCmd = ps.input.Update(msg)
			query := ps.input.Value()
			if ps.mode == PaletteModeCLI {
				// In CLI mode we filter by the first token.
				tokens := strings.Fields(query)
				if len(tokens) > 0 {
					ps.filter(tokens[0])
				} else {
					ps.filter("")
				}
			} else {
				ps.filterFuzzy(query)
			}
			if ps.cursor >= ps.resultCount() {
				ps.cursor = max(0, ps.resultCount()-1)
			}
			return ps, inputCmd, true
		}
	}

	var inputCmd tea.Cmd
	ps.input, inputCmd = ps.input.Update(msg)
	return ps, inputCmd, true
}

// confirm attempts to execute the selected or typed command.
func (ps *PaletteState) confirm() tea.Cmd {
	raw := strings.TrimSpace(ps.input.Value())

	if ps.mode == PaletteModeFuzzy {
		// Fuzzy mode — dispatch based on the result kind.
		if ps.cursor >= len(ps.results) {
			return nil
		}
		r := ps.results[ps.cursor]
		switch r.Kind {
		case SearchResultCommand:
			if r.Command != nil && r.Command.Execute != nil {
				return r.Command.Execute(nil)
			}
		case SearchResultNode:
			if r.NodeID != "" {
				nodeID := r.NodeID
				return func() tea.Msg { return nodeSelectedMsg{nodeID: nodeID} }
			}
		case SearchResultEdge:
			// Navigate to the From node of the edge.
			if r.NodeID != "" {
				nodeID := r.NodeID
				return func() tea.Msg { return nodeSelectedMsg{nodeID: nodeID} }
			}
		}
		return nil
	}

	// CLI mode — parse "name arg1 arg2 …"
	tokens := strings.Fields(raw)
	if len(tokens) == 0 {
		return nil
	}
	name := tokens[0]
	args := tokens[1:]
	for _, c := range ps.commands {
		if c.Name == name {
			if c.Execute != nil {
				return c.Execute(args)
			}
			return nil
		}
	}
	return nil
}

// View renders the palette overlay as a centred modal box.
func (ps *PaletteState) View(width, height int) string {
	if !ps.active {
		return ""
	}

	boxWidth := width * 2 / 3
	if boxWidth < 40 {
		boxWidth = 40
	}
	if boxWidth > 80 {
		boxWidth = 80
	}

	bg := ps.theme.BgSecondary()

	var sb strings.Builder

	// Header — background must match box bg to prevent bleed at ANSI resets.
	titleStyle := lipgloss.NewStyle().
		Foreground(ps.theme.AccentPrimary()).
		Background(bg).
		Bold(true).
		Width(boxWidth - 4)

	title := "COMMAND PALETTE"
	if ps.mode == PaletteModeFuzzy {
		title = "SEARCH COMMANDS"
	}
	sb.WriteString(titleStyle.Render(title))
	sb.WriteString("\n")

	// Input field.
	inputStyle := lipgloss.NewStyle().
		Width(boxWidth - 4).
		Background(bg).
		Foreground(ps.theme.FgPrimary())
	sb.WriteString(inputStyle.Render(ps.input.View()))
	sb.WriteString("\n")

	// Divider.
	divStyle := lipgloss.NewStyle().Foreground(ps.theme.Border()).Background(bg)
	sb.WriteString(divStyle.Render(strings.Repeat("─", boxWidth-4)))
	sb.WriteString("\n")

	// Filtered command list.
	maxVisible := 10
	for i, cmd := range ps.filtered {
		if i >= maxVisible {
			break
		}
		// cursor prefix and name/description share the same bg.
		cursorStyle := lipgloss.NewStyle().Background(bg).Foreground(ps.theme.FgMuted())
		cursorText := "  "
		nameStyle := lipgloss.NewStyle().
			Width(boxWidth - 10).
			MaxWidth(boxWidth - 10).
			Background(bg).
			Foreground(ps.theme.FgPrimary())
		hintStyle := lipgloss.NewStyle().
			Background(bg).
			Foreground(ps.theme.FgMuted())

		if i == ps.cursor {
			selBg := ps.theme.Selection()
			cursorText = "> "
			cursorStyle = cursorStyle.Background(selBg).Foreground(ps.theme.AccentPrimary())
			nameStyle = nameStyle.Background(selBg).Foreground(ps.theme.AccentPrimary()).Bold(true)
			hintStyle = hintStyle.Background(selBg).Foreground(ps.theme.FgPrimary())
		}

		line := cursorStyle.Render(cursorText) + nameStyle.Render(cmd.Name+" — "+cmd.Description)
		if cmd.Hint != "" {
			rowBg := bg
			if i == ps.cursor {
				rowBg = ps.theme.Selection()
			}
			line += Spacer(1, rowBg) + hintStyle.Render("["+cmd.Hint+"]")
		}
		sb.WriteString(line)
		sb.WriteString("\n")
	}

	boxStyle := lipgloss.NewStyle().
		Background(bg).
		BorderStyle(lipgloss.RoundedBorder()).
		BorderForeground(ps.theme.AccentPrimary()).
		BorderBackground(bg).
		Padding(1, 2).
		Width(boxWidth)

	// Return the rendered box without any horizontal padding. Centring is
	// handled by the Compositor in app.go via Layer.X(), which positions the
	// overlay relative to the full-width frame layer.
	return boxStyle.Render(sb.String())
}

// max returns the larger of two ints.
func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}
