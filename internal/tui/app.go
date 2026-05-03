package tui

import (
	"fmt"
	"strings"
	"time"

	"charm.land/bubbles/v2/key"
	"charm.land/bubbles/v2/spinner"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	clog "github.com/charmbracelet/log"
	"github.com/jasonwarrenuk/wyrd/internal/types"
)

// Compile-time check: nodeListPane must satisfy PaneModel.
var _ PaneModel = nodeListPane{}

// switchThemeMsg is an internal message that triggers a runtime theme switch.
type switchThemeMsg struct {
	name string
}

// openLogOverlayMsg is emitted when the :log command is invoked.
type openLogOverlayMsg struct{}

// captureSubmitMsg is emitted after a successful node creation (from form or
// capture bar) so the dashboard can refresh and the status bar can confirm.
type captureSubmitMsg struct {
	nodeID string
	label  string
}

// captureConfirmClearMsg is emitted after a short delay to clear the
// confirmation text from the status bar.
type captureConfirmClearMsg struct{}

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

	// keyMap holds the application-level key bindings.
	keyMap AppKeyMap

	// palette is the command palette overlay state.
	palette PaletteState

	// statusBar is the bottom status bar.
	statusBar StatusBar

	// store is retained for the capture bar and form panes to write nodes.
	store types.StoreFS

	// captureBar handles rapid node creation from the status bar area.
	captureBar *CaptureBar

	// queryRunner is stored so the dashboard can be refreshed after capture.
	queryRunner types.QueryRunner

	// index is the in-memory graph, used to fetch node detail on selection.
	index types.GraphIndex

	// clock is used for age calculations in the detail renderer.
	clock types.Clock

	// detailRenderer is reused across node selections so the expensive
	// glamour.NewTermRenderer() call only happens once (or on resize).
	detailRenderer *DetailRenderer

	// logger is the structured logger. May be nil.
	logger *clog.Logger

	// logOverlay is the debug log viewer overlay.
	logOverlay logOverlay

	// ready is set to true once the first WindowSizeMsg has been received.
	ready bool

	// quitting is set when the user has requested an exit.
	quitting bool
}

// detailReadyMsg carries the result of an async detail render.
type detailReadyMsg struct {
	nodeID string
	pane   PaneModel
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

	// Logger is the structured logger. May be nil.
	Logger *clog.Logger
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

	keyMap := DefaultAppKeyMap()
	palette := NewPaletteState(theme, cfg.Index)

	// Wire up the built-in "theme" command now that we have a storePath.
	palette.Register(Command{
		Name:        "theme",
		Description: "Switch to a named theme (e.g. theme peat)",
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

	// Wire up the "log" command.
	palette.Register(Command{
		Name:        "log",
		Description: "Show the debug log overlay",
		Execute: func(args []string) tea.Cmd {
			return func() tea.Msg {
				return openLogOverlayMsg{}
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
	// A saved view named "dashboard" in the store overrides the default queries
	// and columns. Individual keys in view.Queries override only the matching
	// category; missing keys fall back to DefaultDashboardQuery.
	var leftPane PaneModel = NewEmptyPane(theme)
	if cfg.QueryRunner != nil {
		dq := DefaultDashboardQuery()
		cols := dashboardColumns
		if cfg.Store != nil {
			if view, err := cfg.Store.ReadView("dashboard"); err == nil {
				dq = DashboardQueryFromView(view)
				if len(view.Columns) > 0 {
					cols = view.Columns
				}
			}
		}
		result, err := RunDashboard(cfg.QueryRunner, clock, dq, cols)
		if err == nil {
			leftPane = newNodeListPane(result, theme)
		}
		// On error: silently use empty pane — a working TUI with no data is
		// better than a crash on first launch.
	}

	// Create the capture bar when a store is available.
	var captureBar *CaptureBar
	if cfg.Store != nil {
		captureBar = NewCaptureBar(cfg.Store, clock)
	}

	m := Model{
		theme:          theme,
		storePath:      storePath,
		layout:         layout,
		leftPane:       leftPane,
		rightPane:      NewEmptyPane(theme),
		focus:          FocusLeft,
		keyMap:         keyMap,
		palette:        palette,
		statusBar:      statusBar,
		store:          cfg.Store,
		captureBar:     captureBar,
		queryRunner:    cfg.QueryRunner,
		index:          cfg.Index,
		clock:          clock,
		detailRenderer: NewDetailRenderer(),
		logger:         cfg.Logger,
		logOverlay:     newLogOverlay(theme),
		ready:          false,
	}

	// Pre-populate the right pane with the first selected item so the detail
	// pane is not blank on startup.
	if lp, ok := leftPane.(nodeListPane); ok {
		if id := lp.SelectedNodeID(); id != "" {
			m.rightPane = m.renderDetail(id)
			if m.index != nil {
				if node, err := m.index.GetNode(id); err == nil {
					edgeCount := len(m.index.EdgesFrom(id)) + len(m.index.EdgesTo(id))
					m.statusBar.SetNodeInfo(node.ID, node.Types, edgeCount)
				}
			}
		}
	}

	// Populate initial keybind hints for the focused (left) pane.
	m.syncKeyHints()

	return m, nil
}

// Init returns the initial command. We request a window-size message so the
// layout is correct on first render.
func (m Model) Init() tea.Cmd {
	return nil
}

// Update is the Elm-style update function. All state changes happen here.
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	// When the capture bar is focused, key input goes exclusively to it.
	// Non-key messages (resize, spinner ticks, etc.) still fall through.
	if m.captureBar != nil && m.captureBar.IsFocused() {
		if keyMsg, ok := msg.(tea.KeyPressMsg); ok {
			return m.handleCaptureKey(keyMsg)
		}
	}

	// When the log overlay is active, route input to it.
	if m.logOverlay.IsActive() {
		cmd, consumed := m.logOverlay.Update(msg)
		if consumed {
			return m, cmd
		}
	}

	// When the node list is actively filtering, key input goes exclusively to
	// it — same pattern as the capture bar. ctrl+c is checked first so the
	// user can always quit, even mid-filter.
	if keyMsg, ok := msg.(tea.KeyPressMsg); ok {
		if lp, ok := m.leftPane.(nodeListPane); ok && m.focus == FocusLeft && lp.IsFiltering() {
			if key.Matches(keyMsg, m.keyMap.Quit) {
				m.quitting = true
				return m, tea.Quit
			}
			return m.updateFocusedPane(keyMsg)
		}
	}

	// Handle async capture messages regardless of capture bar focus state.
	switch msg := msg.(type) {
	case openLogOverlayMsg:
		m.logOverlay.Open(m.layout.totalWidth, m.layout.totalHeight)
		return m, nil
	case captureSubmitMsg:
		return m.handleCaptureSubmit(msg)
	case captureConfirmClearMsg:
		m.statusBar.SetCaptureText(CaptureBarPlaceholder())
		return m, nil
	}

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
		m.leftPane, _ = m.leftPane.Update(msg)
		m.rightPane, _ = m.rightPane.Update(msg)
		m.ready = true
		return m, nil

	case nodeSelectedMsg:
		// Don't overwrite the right pane with detail if a form is active.
		if _, isForm := m.rightPane.(formActivePane); !isForm {
			// Render detail asynchronously so Glamour initialisation doesn't
			// block the event loop. The right pane shows a placeholder until
			// the detailReadyMsg arrives.
			m.rightPane = NewEmptyPane(m.theme)
			cmd := m.renderDetailAsync(msg.nodeID)
			if m.index != nil {
				if node, err := m.index.GetNode(msg.nodeID); err == nil {
					edgeCount := len(m.index.EdgesFrom(msg.nodeID)) + len(m.index.EdgesTo(msg.nodeID))
					m.statusBar.SetNodeInfo(node.ID, node.Types, edgeCount)
				}
			}
			return m, cmd
		}
		if m.index != nil {
			if node, err := m.index.GetNode(msg.nodeID); err == nil {
				edgeCount := len(m.index.EdgesFrom(msg.nodeID)) + len(m.index.EdgesTo(msg.nodeID))
				m.statusBar.SetNodeInfo(node.ID, node.Types, edgeCount)
			}
		}
		return m, nil

	case detailReadyMsg:
		// Result of async detail rendering. Mount the pane unless a form
		// has taken over the right pane in the meantime.
		if _, isForm := m.rightPane.(formActivePane); !isForm {
			m.rightPane = msg.pane
		}
		return m, nil

	case formSubmitMsg:
		m.rightPane = NewEmptyPane(m.theme)
		m.focus = FocusLeft
		m.syncKeyHints()
		return m.handleCaptureSubmit(captureSubmitMsg{nodeID: msg.nodeID, label: msg.label})

	case formCancelMsg:
		m.focus = FocusLeft
		m.syncKeyHints()
		if lp, ok := m.leftPane.(nodeListPane); ok {
			if id := lp.SelectedNodeID(); id != "" {
				m.rightPane = m.renderDetail(id)
				return m, nil
			}
		}
		m.rightPane = NewEmptyPane(m.theme)
		return m, nil

	case editSubmitMsg:
		m.rightPane = NewEmptyPane(m.theme)
		m.focus = FocusLeft
		m.syncKeyHints()
		return m.handleEditSubmit(msg)

	case spendSubmitMsg:
		m.rightPane = NewEmptyPane(m.theme)
		m.focus = FocusLeft
		m.syncKeyHints()
		m.statusBar.SetCaptureText(fmt.Sprintf("Recorded %.2f to %s", msg.amount, msg.category))
		if m.queryRunner != nil {
			dq := DefaultDashboardQuery()
			if m.store != nil {
				if view, err := m.store.ReadView("dashboard"); err == nil {
					dq = DashboardQueryFromView(view)
				}
			}
			if result, err := RunDashboard(m.queryRunner, m.clock, dq); err == nil {
				lp := newNodeListPane(result, m.theme)
				sized, _ := lp.Update(tea.WindowSizeMsg{
					Width:  m.layout.TotalWidth(),
					Height: m.layout.TotalHeight(),
				})
				m.leftPane = sized
			}
		}
		return m, tea.Tick(2*time.Second, func(_ time.Time) tea.Msg {
			return captureConfirmClearMsg{}
		})

	case filterStateChangedMsg:
		m.syncKeyHints()
		return m, nil

	case switchThemeMsg:
		newTheme, err := LoadTheme(m.storePath, msg.name)
		if err != nil {
			// Ignore bad theme name — keep the current theme.
			return m, nil
		}
		m = m.applyTheme(newTheme)
		return m, nil

	case tea.KeyPressMsg:
		switch {
		case key.Matches(msg, m.keyMap.Quit):
			m.quitting = true
			return m, tea.Quit
		case key.Matches(msg, m.keyMap.SwitchPane):
			return m.handleSwitchPane()
		case key.Matches(msg, m.keyMap.CommandPalette):
			m.palette.Open(PaletteModeCLI)
			return m, nil
		case key.Matches(msg, m.keyMap.FuzzyPalette):
			m.palette.Open(PaletteModeFuzzy)
			return m, nil
		case key.Matches(msg, m.keyMap.Capture):
			return m.handleCapture(msg)
		case key.Matches(msg, m.keyMap.EditNode):
			return m.handleEditNode()
		case key.Matches(msg, m.keyMap.ArchiveNode):
			return m.handleArchiveNode()
		default:
			return m.updateFocusedPane(msg)
		}

	case tea.KeyReleaseMsg:
		return m.updateFocusedPane(msg)

	case spinner.TickMsg:
		cmd := m.statusBar.Update(msg)
		return m, cmd
	}

	// Broadcast non-key messages to both panes (e.g. tick, window resize already
	// handled above, custom domain messages).
	return m.updateBothPanes(msg)
}

// handleSwitchPane toggles focus between the left and right panes, notifying
// the losing pane via HandleFocusLost before toggling.
func (m Model) handleSwitchPane() (tea.Model, tea.Cmd) {
	var lostCmd tea.Cmd
	if m.focus == FocusLeft {
		lostCmd = m.leftPane.HandleFocusLost()
		m.focus = FocusRight
	} else {
		lostCmd = m.rightPane.HandleFocusLost()
		m.focus = FocusLeft
	}
	m.syncKeyHints()
	return m, lostCmd
}

// handleCapture focuses the capture bar unless a form is active in the right
// pane, in which case the key is forwarded to the focused pane instead.
func (m Model) handleCapture(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	if m.captureBar == nil {
		return m, nil
	}
	// Don't steal focus if a form is active in the right pane.
	if _, isForm := m.rightPane.(formActivePane); isForm {
		return m.updateFocusedPane(msg)
	}
	m.captureBar.Focus("")
	m.statusBar.SetCaptureText(captureDisplayText(""))
	return m, nil
}

// handleCaptureKey processes a key press while the capture bar is focused.
// Escape cancels, Enter dispatches a form, all other keys accumulate input.
func (m Model) handleCaptureKey(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc":
		m.captureBar.Blur()
		m.statusBar.SetCaptureText(CaptureBarPlaceholder())
		return m, nil

	case "enter":
		input := strings.TrimSpace(m.captureBar.Input())
		m.captureBar.Blur()
		if input == "" {
			m.statusBar.SetCaptureText(CaptureBarPlaceholder())
			return m, nil
		}
		nodeType, body := parseCapturePrefixes(input)
		m.statusBar.SetCaptureText(CaptureBarPlaceholder())

		// Spend form has its own dispatch path: it needs the index and returns
		// an error when no budget categories exist.
		if nodeType == "spend" {
			sp, err := newSpendFormPane(m.theme, m.store, m.index, m.clock, body)
			if err != nil {
				m.statusBar.SetCaptureText("No budget categories found")
				return m, tea.Tick(2*time.Second, func(_ time.Time) tea.Msg {
					return captureConfirmClearMsg{}
				})
			}
			m.rightPane = sp
			m.focus = FocusRight
			m.syncKeyHints()
			return m, sp.form.Init()
		}

		var selectedID string
		if lp, ok := m.leftPane.(nodeListPane); ok {
			selectedID = lp.SelectedNodeID()
		}

		var fp formPane
		switch nodeType {
		case "journal":
			fp = newJournalFormPane(m.theme, m.store, m.clock, selectedID, body)
		case "note":
			fp = newNoteFormPane(m.theme, m.store, m.clock, selectedID, body)
		default:
			fp = newTaskFormPane(m.theme, m.store, m.clock, selectedID, body)
		}
		m.rightPane = fp
		m.focus = FocusRight
		m.syncKeyHints()
		return m, fp.form.Init()

	case "backspace":
		m.captureBar.Backspace()
		m.statusBar.SetCaptureText(captureDisplayText(m.captureBar.Input()))
		return m, nil

	default:
		if msg.Text != "" {
			for _, r := range msg.Text {
				m.captureBar.AppendRune(r)
			}
			m.statusBar.SetCaptureText(captureDisplayText(m.captureBar.Input()))
		}
		return m, nil
	}
}

// handleCaptureSubmit refreshes the dashboard after a node is created and
// shows a brief confirmation in the status bar.
func (m Model) handleCaptureSubmit(msg captureSubmitMsg) (tea.Model, tea.Cmd) {
	m.statusBar.SetCaptureText("Created " + msg.label)

	if m.queryRunner != nil {
		dq := DefaultDashboardQuery()
		if m.store != nil {
			if view, err := m.store.ReadView("dashboard"); err == nil {
				dq = DashboardQueryFromView(view)
			}
		}
		if result, err := RunDashboard(m.queryRunner, m.clock, dq); err == nil {
			lp := newNodeListPane(result, m.theme)
			sized, _ := lp.Update(tea.WindowSizeMsg{
				Width:  m.layout.TotalWidth(),
				Height: m.layout.TotalHeight(),
			})
			m.leftPane = sized
		}
	}

	return m, tea.Tick(2*time.Second, func(_ time.Time) tea.Msg {
		return captureConfirmClearMsg{}
	})
}

// handleEditNode opens an edit form for the currently selected node. It is a
// no-op when a form is already active, when no node is selected, or when the
// store is unavailable.
func (m Model) handleEditNode() (tea.Model, tea.Cmd) {
	// Guard: form already open.
	if _, isForm := m.rightPane.(formActivePane); isForm {
		return m, nil
	}
	// Guard: store unavailable.
	if m.store == nil {
		return m, nil
	}
	// Guard: no node selected.
	lp, ok := m.leftPane.(nodeListPane)
	if !ok {
		return m, nil
	}
	nodeID := lp.SelectedNodeID()
	if nodeID == "" {
		return m, nil
	}
	// Read the full node from the store for reliable Properties.
	node, err := m.store.ReadNode(nodeID)
	if err != nil || node == nil {
		return m, nil
	}

	primaryType := ""
	if len(node.Types) > 0 {
		primaryType = node.Types[0]
	}
	var fp formPane
	switch primaryType {
	case "journal":
		fp = newEditJournalFormPane(m.theme, m.store, m.clock, node)
	case "note":
		fp = newEditNoteFormPane(m.theme, m.store, m.clock, node)
	default:
		fp = newEditTaskFormPane(m.theme, m.store, m.clock, node)
	}

	m.rightPane = fp
	m.focus = FocusRight
	m.syncKeyHints()
	return m, fp.form.Init()
}

// handleEditSubmit refreshes the dashboard and detail pane after a node is
// updated, and shows a brief confirmation in the status bar.
func (m Model) handleEditSubmit(msg editSubmitMsg) (tea.Model, tea.Cmd) {
	m.statusBar.SetCaptureText("Updated " + msg.label)

	if m.queryRunner != nil {
		dq := DefaultDashboardQuery()
		if m.store != nil {
			if view, err := m.store.ReadView("dashboard"); err == nil {
				dq = DashboardQueryFromView(view)
			}
		}
		if result, err := RunDashboard(m.queryRunner, m.clock, dq); err == nil {
			lp := newNodeListPane(result, m.theme)
			sized, _ := lp.Update(tea.WindowSizeMsg{
				Width:  m.layout.TotalWidth(),
				Height: m.layout.TotalHeight(),
			})
			m.leftPane = sized
		}
	}

	// Re-render the detail pane so the right side shows the updated content.
	detailCmd := m.renderDetailAsync(msg.nodeID)

	clearCmd := tea.Tick(2*time.Second, func(_ time.Time) tea.Msg {
		return captureConfirmClearMsg{}
	})

	return m, tea.Batch(detailCmd, clearCmd)
}

// handleArchiveNode archives the currently selected node. It is a no-op when a
// form is active, when no node is selected, or when the store is unavailable.
func (m Model) handleArchiveNode() (tea.Model, tea.Cmd) {
	// Guard: form already open.
	if _, isForm := m.rightPane.(formActivePane); isForm {
		return m, nil
	}
	// Guard: store unavailable.
	if m.store == nil {
		return m, nil
	}
	// Guard: no node selected.
	lp, ok := m.leftPane.(nodeListPane)
	if !ok {
		return m, nil
	}
	nodeID := lp.SelectedNodeID()
	if nodeID == "" {
		return m, nil
	}

	node, err := m.store.ReadNode(nodeID)
	if err != nil || node == nil {
		return m, nil
	}

	if err := m.store.ArchiveNode(nodeID); err != nil {
		return m, nil
	}

	label := node.Title
	if label == "" {
		label = nodeID
	}
	if len(label) > 40 {
		label = label[:37] + "…"
	}
	m.statusBar.SetCaptureText("Archived " + label)

	// Refresh the dashboard so the archived node disappears from the list.
	if m.queryRunner != nil {
		dq := DefaultDashboardQuery()
		if view, err := m.store.ReadView("dashboard"); err == nil {
			dq = DashboardQueryFromView(view)
		}
		if result, err := RunDashboard(m.queryRunner, m.clock, dq); err == nil {
			lp := newNodeListPane(result, m.theme)
			sized, _ := lp.Update(tea.WindowSizeMsg{
				Width:  m.layout.TotalWidth(),
				Height: m.layout.TotalHeight(),
			})
			m.leftPane = sized
		}
	}

	// Clear the right pane and return focus to the list.
	m.rightPane = NewEmptyPane(m.theme)
	m.focus = FocusLeft
	m.syncKeyHints()

	clearCmd := tea.Tick(2*time.Second, func(_ time.Time) tea.Msg {
		return captureConfirmClearMsg{}
	})

	return m, clearCmd
}

// captureDisplayText formats capture bar input for display in the status bar,
// appending a lightweight cursor character.
func captureDisplayText(input string) string {
	return input + "▌"
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

// updateBothPanes sends msg to both panes and batches their commands.
func (m Model) updateBothPanes(msg tea.Msg) (tea.Model, tea.Cmd) {
	var leftCmd, rightCmd tea.Cmd
	m.leftPane, leftCmd = m.leftPane.Update(msg)
	m.rightPane, rightCmd = m.rightPane.Update(msg)
	return m, tea.Batch(leftCmd, rightCmd)
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

// View renders the full TUI frame.
func (m Model) View() tea.View {
	var frame string
	if m.quitting {
		frame = ""
	} else if !m.ready {
		frame = "Initialising…"
	} else {
		leftView := m.leftPane.View()
		rightView := m.rightPane.View()
		statusView := m.statusBar.View()

		frame = m.layout.Render(leftView, rightView, statusView, m.focus)

		// If the palette is active, composite it on top of the frame using the
		// lipgloss v2 Compositor so there is no brittle line-manipulation.
		if m.palette.IsActive() {
			overlay := m.palette.View(m.layout.totalWidth, m.layout.totalHeight)
			if overlay != "" {
				overlayWidth := lipgloss.Width(overlay)
				centreX := (m.layout.totalWidth - overlayWidth) / 2
				if centreX < 0 {
					centreX = 0
				}
				frameLayer := lipgloss.NewLayer(frame).Z(0)
				overlayLayer := lipgloss.NewLayer(overlay).X(centreX).Y(2).Z(1)
				frame = lipgloss.NewCompositor(frameLayer, overlayLayer).Render()
			}
		}

		// If the log overlay is active, composite it on top using the same
		// pattern as the palette.
		if m.logOverlay.IsActive() {
			overlay := m.logOverlay.View(m.layout.totalWidth, m.layout.totalHeight)
			if overlay != "" {
				overlayWidth := lipgloss.Width(overlay)
				centreX := (m.layout.totalWidth - overlayWidth) / 2
				if centreX < 0 {
					centreX = 0
				}
				frameLayer := lipgloss.NewLayer(frame).Z(0)
				overlayLayer := lipgloss.NewLayer(overlay).X(centreX).Y(2).Z(1)
				frame = lipgloss.NewCompositor(frameLayer, overlayLayer).Render()
			}
		}
	}

	v := tea.NewView(frame)
	v.AltScreen = true
	return v
}

// MountLeft replaces the left pane content. Phase 4 agents call this to
// mount their view implementations.
func (m *Model) MountLeft(pane PaneModel) {
	m.leftPane = pane
	m.syncKeyHints()
}

// MountRight replaces the right pane content. Phase 4 agents call this to
// mount their view implementations.
func (m *Model) MountRight(pane PaneModel) {
	m.rightPane = pane
	if m.focus == FocusRight {
		m.syncKeyHints()
	}
}

// syncKeyHints pushes the focused pane's keybindings to the status bar.
func (m *Model) syncKeyHints() {
	if m.focus == FocusLeft {
		m.statusBar.SetKeyHints(m.leftPane.KeyBindings())
	} else {
		m.statusBar.SetKeyHints(m.rightPane.KeyBindings())
	}
}

// RegisterCommand adds a command to the palette. Phase 4 agents call this
// during initialisation to expose their commands.
func (m *Model) RegisterCommand(cmd Command) {
	m.palette.Register(cmd)
}

// Theme returns the currently active theme. Phase 4 agents use this to
// derive their own Lipgloss styles from the theme colours.
func (m *Model) Theme() *ActiveTheme {
	return m.theme
}

// StatusBar returns a pointer to the status bar so callers can start/stop
// the spinner or update its text content.
func (m *Model) StatusBar() *StatusBar {
	return &m.statusBar
}

// renderDetailAsync returns a tea.Cmd that renders the detail pane in a
// goroutine, sending a detailReadyMsg when complete. This keeps the event
// loop responsive while Glamour processes markdown.
func (m Model) renderDetailAsync(nodeID string) tea.Cmd {
	return func() tea.Msg {
		pane := m.renderDetail(nodeID)
		return detailReadyMsg{nodeID: nodeID, pane: pane}
	}
}

// renderDetail fetches a node by ID and renders it into a detailPane.
// Returns an emptyPane when the index is unavailable or the node is not found.
func (m Model) renderDetail(nodeID string) PaneModel {
	if m.index == nil || nodeID == "" {
		return NewEmptyPane(m.theme)
	}
	node, err := m.index.GetNode(nodeID)
	if err != nil {
		return NewEmptyPane(m.theme)
	}

	// Collect all edges connected to this node.
	edges := append(m.index.EdgesFrom(nodeID), m.index.EdgesTo(nodeID)...)

	// Build a lookup map for resolving edge targets in the renderer.
	allNodes := m.index.AllNodes()
	nodesByID := make(map[string]*types.Node, len(allNodes))
	for _, n := range allNodes {
		nodesByID[n.ID] = n
	}

	renderer := m.detailRenderer
	renderer.Width = m.layout.totalWidth / 2
	renderer.Colours.BgPrimary       = m.theme.BgPrimary()
	renderer.Colours.FGPrimary       = m.theme.FgPrimary()
	renderer.Colours.FGMuted         = m.theme.FgMuted()
	renderer.Colours.AccentPrimary   = m.theme.AccentPrimary()
	renderer.Colours.AccentSecondary = m.theme.AccentSecondary()
	renderer.Colours.BudgetOK        = m.theme.BudgetOK()
	renderer.Colours.BudgetCaution   = m.theme.BudgetCaution()
	renderer.Colours.BudgetOver      = m.theme.BudgetOver()
	renderer.Colours.OverflowWarn    = m.theme.OverflowWarn()
	renderer.Colours.OverflowCrit    = m.theme.OverflowCritical()

	now := time.Now()
	if m.clock != nil {
		now = m.clock.Now()
	}

	content := renderer.Render(node, edges, nodesByID, nil, now)

	// Size the viewport to the right pane's inner dimensions.
	// Borders consume 2 columns (left+right) and the layout accounts for
	// the status bar; paneHeight gives usable inner rows.
	vpWidth := m.layout.RightWidth() - 2
	vpHeight := m.layout.PaneHeight()
	if vpWidth < 1 {
		vpWidth = 1
	}
	return newViewportPane(vpWidth, vpHeight, content, m.theme.BgPrimary())
}


// Ensure Model satisfies tea.Model at compile time.
var _ tea.Model = Model{}

// Run is a convenience function that creates and runs the Bubble Tea program.
func Run(cfg Config) error {
	m, err := New(cfg)
	if err != nil {
		return fmt.Errorf("initialise TUI: %w", err)
	}
	// Drop unsolicited cursor-position reports — we never request them and
	// they arrive as noise after some terminal interactions.
	p := tea.NewProgram(m, tea.WithFilter(func(_ tea.Model, msg tea.Msg) tea.Msg {
		if _, ok := msg.(tea.CursorPositionMsg); ok {
			return nil
		}
		return msg
	}))
	_, err = p.Run()
	return err
}
