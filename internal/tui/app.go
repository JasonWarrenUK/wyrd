package tui

import (
	"fmt"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/jasonwarrenuk/wyrd/internal/types"
)

// Compile-time check: nodeListPane must satisfy PaneModel.
var _ PaneModel = nodeListPane{}

// switchThemeMsg is an internal message that triggers a runtime theme switch.
type switchThemeMsg struct {
	name string
}

// Model is the root Bubble Tea model for the Wyrd TUI. It owns all mutable
// state; transitions happen in Update and rendering in View. No state is held
// outside this struct.
//
// Following the Elm architecture strictly:
//   - Model holds state
//   - Update handles messages and returns (Model, Cmd)
//   - View renders the model to a string
type Model struct {
	// theme is the currently active colour scheme.
	theme *ActiveTheme

	// storePath is used to resolve theme files at runtime.
	storePath string

	// layout holds terminal dimensions and pane sizing.
	layout Layout

	// leftPane is the left-side content (schedule / view / ritual).
	leftPane PaneModel

	// rightPane is the right-side content (detail / editor).
	rightPane PaneModel

	// focus indicates which pane has keyboard focus.
	focus FocusedPane

	// keyMap handles vim-style key dispatch.
	keyMap *KeyMap

	// palette is the command palette overlay state.
	palette PaletteState

	// statusBar is the bottom status bar.
	statusBar StatusBar

	// ready is set to true once the first WindowSizeMsg has been received.
	ready bool

	// quitting is set when the user has requested an exit.
	quitting bool
}

// Config carries the options for constructing a new App Model.
type Config struct {
	// Store is the StoreFS used to load themes and config.
	// May be nil; the app starts on an empty store without crashing.
	Store types.StoreFS

	// StorePath is the path to the store directory.
	// Used when Store is nil or theme loading needs a direct path.
	StorePath string

	// ThemeName is the theme to load. Defaults to the first available theme.
	ThemeName string

	// Index is the in-memory graph index. When provided alongside QueryRunner,
	// the dashboard left pane is populated on startup.
	Index types.GraphIndex

	// QueryRunner executes Cypher queries against the Index.
	// Used to run the default (or user-configured) dashboard query on launch.
	QueryRunner types.QueryRunner

	// Clock is used for date variable resolution in queries (e.g. $today).
	// Defaults to types.RealClock{} when nil.
	Clock types.Clock
}

// New builds the initial App Model. It may be called with an empty / nil store.
func New(cfg Config) (Model, error) {
	storePath := cfg.StorePath
	if storePath == "" {
		storePath = "."
	}

	// Attempt to read config for theme name.
	themeName := cfg.ThemeName
	if themeName == "" && cfg.Store != nil {
		if appCfg, err := cfg.Store.ReadConfig(); err == nil && appCfg.Theme != "" {
			themeName = appCfg.Theme
		}
	}

	theme, err := LoadTheme(storePath, themeName)
	if err != nil {
		// LoadTheme falls back to the built-in theme internally; this error
		// path should not be reached, but we guard defensively.
		theme, _ = LoadTheme(".", "")
	}

	keyMap := NewKeyMap()
	palette := NewPaletteState(theme)

	// Wire up the built-in "theme" command now that we have a storePath.
	palette.Register(Command{
		Name:        "theme",
		Description: "Switch to a named theme (e.g. theme ember-violet)",
		Execute: func(args []string) tea.Cmd {
			if len(args) == 0 {
				return nil
			}
			name := args[0]
			return func() tea.Msg {
				return switchThemeMsg{name: name}
			}
		},
	})

	statusBar := NewStatusBar(theme)

	// Use a conservative default size; the real size arrives via WindowSizeMsg.
	layout := NewLayout(80, 24, theme)
	statusBar.SetWidth(80)

	// Resolve the clock — default to real wall time when not supplied.
	clock := cfg.Clock
	if clock == nil {
		clock = types.RealClock{}
	}

	// Build the initial left pane. When a QueryRunner is available, run the
	// dashboard query and mount the result. If the query fails (e.g. empty
	// store, no matching nodes), fall back to the empty placeholder so the
	// app still launches cleanly.
	//
	// WL.7: once user-configurable saved views are supported, load the
	// "dashboard" view from the store here and pass its query to RunDashboard
	// instead of DefaultDashboardQuery().
	var leftPane PaneModel = NewEmptyPane(theme)
	if cfg.QueryRunner != nil {
		result, err := RunDashboard(cfg.QueryRunner, clock, DefaultDashboardQuery())
		if err == nil {
			leftPane = newNodeListPane(result)
		}
		// On error: silently use empty pane — a working TUI with no data is
		// better than a crash on first launch.
	}

	m := Model{
		theme:     theme,
		storePath: storePath,
		layout:    layout,
		leftPane:  leftPane,
		rightPane: NewEmptyPane(theme),
		focus:     FocusLeft,
		keyMap:    keyMap,
		palette:   palette,
		statusBar: statusBar,
		ready:     false,
	}

	return m, nil
}

// Init returns the initial command. We request a window-size message so the
// layout is correct on first render.
func (m Model) Init() tea.Cmd {
	return nil
}

// Update is the Elm-style update function. All state changes happen here.
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	// Let the palette handle input first when it is active.
	if m.palette.IsActive() {
		var cmd tea.Cmd
		var remaining bool
		m.palette, cmd, remaining = m.palette.Update(msg)
		if !remaining {
			// Palette was closed — no further routing needed.
			return m, cmd
		}
		// Palette is still open — keep forwarding its commands.
		return m, cmd
	}

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.layout.Resize(msg.Width, msg.Height)
		m.statusBar.SetWidth(msg.Width)
		m.ready = true
		return m, nil

	case switchThemeMsg:
		newTheme, err := LoadTheme(m.storePath, msg.name)
		if err != nil {
			// Ignore bad theme name — keep the current theme.
			return m, nil
		}
		m = m.applyTheme(newTheme)
		return m, nil

	case tea.KeyMsg:
		action := m.keyMap.Dispatch(msg)
		return m.handleAction(action, msg)
	}

	// Forward unhandled messages to the focused pane.
	return m.updateFocusedPane(msg)
}

// handleAction translates a resolved KeyAction into state changes.
func (m Model) handleAction(action KeyAction, msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch action {
	case ActionQuit:
		m.quitting = true
		return m, tea.Quit

	case ActionSwitchPane:
		if m.focus == FocusLeft {
			m.focus = FocusRight
		} else {
			m.focus = FocusLeft
		}
		return m, nil

	case ActionCommandPalette:
		m.palette.Open(PaletteModeCLI)
		return m, nil

	case ActionFuzzyPalette:
		m.palette.Open(PaletteModeFuzzy)
		return m, nil

	case ActionNone:
		// Unrecognised key — forward to the focused pane.
		return m.updateFocusedPane(msg)

	default:
		// Navigation actions are forwarded to the focused pane so it can
		// scroll its content.
		return m.updateFocusedPane(msg)
	}
}

// updateFocusedPane routes a message to whichever pane currently has focus.
func (m Model) updateFocusedPane(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd
	if m.focus == FocusLeft {
		m.leftPane, cmd = m.leftPane.Update(msg)
	} else {
		m.rightPane, cmd = m.rightPane.Update(msg)
	}
	return m, cmd
}

// applyTheme swaps the active theme and propagates it to all sub-components.
func (m Model) applyTheme(t *ActiveTheme) Model {
	m.theme = t
	m.layout.SetTheme(t)
	m.statusBar.SetTheme(t)
	m.palette.theme = t
	// Re-create empty panes with the new theme so their muted text re-renders.
	if _, ok := m.leftPane.(emptyPane); ok {
		m.leftPane = NewEmptyPane(t)
	}
	if _, ok := m.rightPane.(emptyPane); ok {
		m.rightPane = NewEmptyPane(t)
	}
	return m
}

// View renders the full TUI frame to a string.
func (m Model) View() string {
	if m.quitting {
		return ""
	}
	if !m.ready {
		return "Initialising…"
	}

	leftView := m.leftPane.View()
	rightView := m.rightPane.View()
	statusView := m.statusBar.View()

	frame := m.layout.Render(leftView, rightView, statusView, m.focus)

	// If the palette is active, overlay it on top of the frame.
	if m.palette.IsActive() {
		overlay := m.palette.View(m.layout.totalWidth, m.layout.totalHeight)
		if overlay != "" {
			// Simple approach: replace the middle of the frame with the palette.
			// The overlay is centred horizontally; we place it at line 3.
			lines := splitLines(frame)
			overlayLines := splitLines(overlay)
			startLine := 2
			for i, ol := range overlayLines {
				idx := startLine + i
				if idx < len(lines) {
					lines[idx] = ol
				}
			}
			frame = joinLines(lines)
		}
	}

	return frame
}

// MountLeft replaces the left pane content. Phase 4 agents call this to
// mount their view implementations.
func (m *Model) MountLeft(pane PaneModel) {
	m.leftPane = pane
}

// MountRight replaces the right pane content. Phase 4 agents call this to
// mount their view implementations.
func (m *Model) MountRight(pane PaneModel) {
	m.rightPane = pane
}

// RegisterCommand adds a command to the palette. Phase 4 agents call this
// during initialisation to expose their commands.
func (m *Model) RegisterCommand(cmd Command) {
	m.palette.Register(cmd)
}

// RegisterKeyBinding adds a keybinding to the global key map.
// Phase 4 agents call this during initialisation to extend navigation.
func (m *Model) RegisterKeyBinding(binding KeyBinding, action KeyAction) {
	m.keyMap.RegisterCustom(binding, action)
}

// Theme returns the currently active theme. Phase 4 agents use this to
// derive their own Lipgloss styles from the theme colours.
func (m *Model) Theme() *ActiveTheme {
	return m.theme
}

// splitLines splits a string on newlines.
func splitLines(s string) []string {
	var lines []string
	start := 0
	for i := 0; i < len(s); i++ {
		if s[i] == '\n' {
			lines = append(lines, s[start:i])
			start = i + 1
		}
	}
	if start <= len(s) {
		lines = append(lines, s[start:])
	}
	return lines
}

// joinLines joins lines with newlines.
func joinLines(lines []string) string {
	if len(lines) == 0 {
		return ""
	}
	result := lines[0]
	for _, l := range lines[1:] {
		result += "\n" + l
	}
	return result
}

// Ensure Model satisfies tea.Model at compile time.
var _ tea.Model = Model{}

// Run is a convenience function that creates and runs the Bubble Tea program.
func Run(cfg Config) error {
	m, err := New(cfg)
	if err != nil {
		return fmt.Errorf("initialise TUI: %w", err)
	}
	p := tea.NewProgram(m, tea.WithAltScreen())
	_, err = p.Run()
	return err
}
