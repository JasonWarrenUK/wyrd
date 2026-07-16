package tui

import (
	"bytes"
	"fmt"
	"strings"
	"time"

	"charm.land/bubbles/v2/key"
	"charm.land/bubbles/v2/spinner"
	tea "charm.land/bubbletea/v2"
	clog "github.com/charmbracelet/log"
	"github.com/jasonwarrenuk/wyrd/internal/cli"
	"github.com/jasonwarrenuk/wyrd/internal/stage"
	"github.com/jasonwarrenuk/wyrd/internal/tui/ritual"
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

// openHelpOverlayMsg is emitted when the :help command is invoked.
type openHelpOverlayMsg struct{}

// openKindsOverlayMsg is emitted when the :kinds command is invoked.
type openKindsOverlayMsg struct{}

// openStageFormMsg is emitted when the :stages new command is invoked.
type openStageFormMsg struct{}

// openStagesOverlayMsg is emitted when the bare :stages command is invoked.
type openStagesOverlayMsg struct{}

// syncResultMsg carries the outcome of a background sync operation.
type syncResultMsg struct {
	err    error
	output string
}

// captureSubmitMsg is emitted after a successful node creation (from form or
// capture bar) so the dashboard can refresh and the status bar can confirm.
type captureSubmitMsg struct {
	nodeID string
	label  string
}

// captureConfirmClearMsg is emitted after a short delay to clear the
// confirmation text from the status bar. gen is the capture generation the
// tick was scheduled for; the handler ignores the tick if the current
// generation no longer matches (a newer message has since been shown).
type captureConfirmClearMsg struct {
	gen int
}

// clearCaptureCmd schedules a 2s clear guarded by the current generation, so
// a stale tick cannot clear a newer message. Call immediately AFTER the
// SetCaptureText whose message it should clear.
func (m *Model) clearCaptureCmd() tea.Cmd {
	gen := m.statusBar.CaptureGen()
	return tea.Tick(2*time.Second, func(_ time.Time) tea.Msg {
		return captureConfirmClearMsg{gen: gen}
	})
}

// ritualTriggerMsg is sent when a ritual should be presented to the user.
type ritualTriggerMsg struct {
	ritual *types.Ritual
}

// ritualCheckTickMsg fires on a timer to check whether any rituals are due.
type ritualCheckTickMsg struct{}

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

	// ritualOvl is the modal overlay for presenting ritual steps.
	ritualOvl ritualOverlay

	// schedulerState tracks which rituals have been dismissed today.
	schedulerState *ritual.SchedulerState

	// rituals is the list of loaded ritual definitions.
	rituals []*types.Ritual
	// kinds is the merged kind registry (baked-in defaults + user's kinds.jsonc).
	// May be nil when constructed without registry wiring (e.g. tests that don't
	// supply kinds — callers should check for nil before use).
	kinds *types.KindRegistry

	// stageGroups is the merged stage-group registry (baked-in defaults + user's
	// stages.jsonc once SL.13 lands). May be nil; always check before use.
	stageGroups *types.StageGroupRegistry

	// logger is the structured logger. May be nil.
	logger *clog.Logger

	// logOverlay is the debug log viewer overlay.
	logOverlay logOverlay

	// helpOverlay is the key-bindings help overlay.
	helpOverlay helpOverlay

	// kindsOverlay is the kind registry viewer overlay (SL.9).
	kindsOverlay kindsOverlay

	// stagesOverlay is the stage-group list overlay (SL.12).
	stagesOverlay stagesOverlay

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

	// Kinds is the merged kind registry (baked-in defaults + user's kinds.jsonc).
	// May be nil; callers that need kind data should check before use. Downstream
	// TUI tasks (SL.6 stage keypresses, SL.7 kind selection forms) expect this
	// to be populated.
	Kinds *types.KindRegistry

	// StageGroups is the merged stage-group registry (baked-in defaults; user
	// groups from stages.jsonc added by SL.13). May be nil; SL.6 stage keypresses
	// are silently no-ops when nil.
	StageGroups *types.StageGroupRegistry

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

	// Load rituals from the store directory. Errors are non-fatal — the TUI
	// launches without ritual support if loading fails.
	var rituals []*types.Ritual
	loadedRituals, err := ritual.LoadRituals(storePath)
	if err == nil {
		rituals = loadedRituals
	}
	schedulerState := ritual.NewSchedulerState()

	// Register the :ritual palette command.
	palette.Register(Command{
		Name:        "ritual",
		Description: "Trigger a ritual by name (e.g. ritual morning)",
		Execute: func(args []string) tea.Cmd {
			if len(args) == 0 || len(rituals) == 0 {
				return nil
			}
			name := strings.ToLower(args[0])
			for _, r := range rituals {
				if strings.ToLower(r.Name) == name {
					matched := r
					return func() tea.Msg {
						return ritualTriggerMsg{ritual: matched}
					}
				}
			}
			return nil
		},
	})

	// Wire up the "sync" command. The Execute emits a trigger message; the
	// actual sync runs asynchronously in Update so it has access to m.store.
	palette.Register(Command{
		Name:        "sync",
		Description: "Sync with remote (stage, commit, pull, push)",
		Execute: func(args []string) tea.Cmd {
			return func() tea.Msg { return syncResultMsg{output: "__trigger__"} }
		},
	})

	// Wire up the "help" command.
	palette.Register(Command{
		Name:        "help",
		Description: "Show key bindings",
		Execute: func(args []string) tea.Cmd {
			return func() tea.Msg {
				return openHelpOverlayMsg{}
			}
		},
	})

	// Wire up the "kinds" command.
	palette.Register(Command{
		Name:        "kinds",
		Description: "Show registered kinds",
		Execute: func(args []string) tea.Cmd {
			return func() tea.Msg {
				return openKindsOverlayMsg{}
			}
		},
	})

	// Wire up the "stages" command. ":stages" lists all stage groups (SL.12);
	// ":stages new" opens the stage-group creation form (SL.11).
	palette.Register(Command{
		Name:        "stages",
		Description: "List stage groups (stages new to create)",
		Execute: func(args []string) tea.Cmd {
			if len(args) > 0 && args[0] == "new" {
				return func() tea.Msg { return openStageFormMsg{} }
			}
			return func() tea.Msg { return openStagesOverlayMsg{} }
		},
	})

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
		schedulerState: schedulerState,
		rituals:        rituals,
		kinds:          cfg.Kinds,
		stageGroups:    cfg.StageGroups,
		logger:         cfg.Logger,
		logOverlay:     newLogOverlay(theme),
		helpOverlay:    newHelpOverlay(theme),
		kindsOverlay:   newKindsOverlay(theme, cfg.Kinds, cfg.StageGroups),
		stagesOverlay:  newStagesOverlay(theme, cfg.StageGroups),
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

// Init returns the initial command. We fire the ritual check tick immediately
// so any due rituals are presented on launch, then every 60 seconds thereafter.
func (m Model) Init() tea.Cmd {
	return ritualCheckTick()
}

// ritualCheckTick returns a tea.Cmd that fires a ritualCheckTickMsg immediately
// then schedules the next check in 60 seconds.
func ritualCheckTick() tea.Cmd {
	return tea.Tick(0, func(_ time.Time) tea.Msg {
		return ritualCheckTickMsg{}
	})
}

// ritualCheckTickNext returns a tea.Cmd that fires a ritualCheckTickMsg after
// 60 seconds.
func ritualCheckTickNext() tea.Cmd {
	return tea.Tick(60*time.Second, func(_ time.Time) tea.Msg {
		return ritualCheckTickMsg{}
	})
}

// Update is the Elm-style update function. All state changes happen here.
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	// When the ritual overlay is active, route messages to it first.
	if m.ritualOvl.IsActive() {
		cmd, consumed := m.ritualOvl.Update(msg)
		if !m.ritualOvl.IsActive() {
			// Overlay just closed — dismiss the ritual in the scheduler and
			// restart the tick timer.
			if m.ritualOvl.runner != nil {
				r := m.ritualOvl.runner.Ritual()
				m.schedulerState.Dismiss(r.Name, m.clock.Now())
			}
			return m, tea.Batch(cmd, ritualCheckTickNext())
		}
		if consumed {
			return m, cmd
		}
	}

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

	// When the help overlay is active, route input to it.
	if m.helpOverlay.IsActive() {
		cmd, consumed := m.helpOverlay.Update(msg)
		if consumed {
			return m, cmd
		}
	}

	// When the kinds overlay is active, route input to it.
	if m.kindsOverlay.IsActive() {
		cmd, consumed := m.kindsOverlay.Update(msg)
		if consumed {
			return m, cmd
		}
	}

	// When the stages overlay is active, route input to it.
	if m.stagesOverlay.IsActive() {
		cmd, consumed := m.stagesOverlay.Update(msg)
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

	case openHelpOverlayMsg:
		m.helpOverlay.Open(m.layout.totalWidth, m.layout.totalHeight, m.keyMap.AllBindings())
		return m, nil

	case openKindsOverlayMsg:
		m.kindsOverlay.Open(m.layout.totalWidth, m.layout.totalHeight)
		return m, nil

	case openStagesOverlayMsg:
		m.stagesOverlay.Open(m.layout.totalWidth, m.layout.totalHeight)
		return m, nil

	case openStageFormMsg:
		// Guard against clobbering an active form.
		if _, isForm := m.rightPane.(formActivePane); isForm {
			return m, nil
		}
		fp := newStageFormPane(m.theme, m.store, m.stageGroups)
		initCmd := fp.form.Init()
		sized, _ := fp.Update(tea.WindowSizeMsg{
			Width:  m.layout.TotalWidth(),
			Height: m.layout.TotalHeight(),
		})
		m.rightPane = sized
		m.focus = FocusRight
		m.syncKeyHints()
		return m, initCmd

	case syncResultMsg:
		if msg.output == "__trigger__" {
			// Kick off the actual sync in a background goroutine, with a
			// status bar spinner running until the result arrives.
			store := m.store
			logger := m.logger
			spinCmd := m.statusBar.StartSpinner("Syncing…")
			syncCmd := func() tea.Msg {
				if store == nil {
					return syncResultMsg{err: fmt.Errorf("no store available")}
				}
				var buf bytes.Buffer
				err := cli.Sync(store, cli.SyncOptions{Logger: logger}, &buf)
				return syncResultMsg{output: strings.TrimSpace(buf.String()), err: err}
			}
			return m, tea.Batch(spinCmd, syncCmd)
		}
		m.statusBar.StopSpinner()
		if msg.err != nil {
			m.statusBar.SetCaptureText("Sync failed: " + msg.err.Error())
			m.statusBar.MarkCaptureSticky()
			return m, nil
		}
		m.statusBar.SetCaptureText("Sync complete")
		return m, m.clearCaptureCmd()

	case captureSubmitMsg:
		return m.handleCaptureSubmit(msg)
	case captureConfirmClearMsg:
		if msg.gen == m.statusBar.CaptureGen() {
			m.statusBar.SetCaptureText(CaptureBarPlaceholder())
		}
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
	case ritualCheckTickMsg:
		// Check for pending rituals and trigger the first one found.
		pending := ritual.PendingRituals(m.rituals, m.schedulerState, m.clock)
		if len(pending) > 0 {
			r := pending[0]
			return m, func() tea.Msg {
				return ritualTriggerMsg{ritual: r}
			}
		}
		return m, ritualCheckTickNext()

	case ritualTriggerMsg:
		if m.store == nil || m.queryRunner == nil {
			return m, ritualCheckTickNext()
		}
		runner := ritual.NewRunner(msg.ritual, m.store, m.queryRunner, m.clock)
		if err := m.ritualOvl.Open(runner, m.theme, m.layout.totalWidth, m.layout.totalHeight); err != nil {
			// Opening failed — dismiss to avoid retry loops.
			m.schedulerState.Dismiss(msg.ritual.Name, m.clock.Now())
			return m, ritualCheckTickNext()
		}
		return m, nil

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
		return m, m.clearCaptureCmd()

	case stageFormSubmitMsg:
		m.rightPane = NewEmptyPane(m.theme)
		m.focus = FocusLeft
		m.syncKeyHints()
		// Rebuild the in-memory stage-group registry so the new group is
		// usable in-session without restarting. DefaultStageGroups is
		// sync.Once-cached, so re-calling it is cheap.
		if m.store != nil {
			if defaults, err := stage.DefaultStageGroups(); err == nil {
				if userReg, err := m.store.ReadStages(); err == nil {
					m.stageGroups = stage.MergeStageGroups(defaults, userReg.All())
					m.kindsOverlay.stageGroups = m.stageGroups
					m.stagesOverlay.stageGroups = m.stageGroups
				}
			}
		}
		m.statusBar.SetCaptureText(fmt.Sprintf("Created stage group %q", msg.name))
		return m, m.clearCaptureCmd()

	case stageFormErrorMsg:
		m.rightPane = NewEmptyPane(m.theme)
		m.focus = FocusLeft
		m.syncKeyHints()
		m.statusBar.SetCaptureText("Could not save stage group: " + msg.err.Error())
		return m, m.clearCaptureCmd()

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
		if m.store != nil {
			if appCfg, err := m.store.ReadConfig(); err == nil {
				appCfg.Theme = msg.name
				_ = m.store.WriteConfig(appCfg)
			}
		}
		return m, nil

	case tea.KeyPressMsg:
		// When a form is active in the right pane, esc / ctrl+c / q abort the
		// form rather than quitting the app. All other keys still flow to the pane.
		if _, isForm := m.rightPane.(formActivePane); isForm {
			if key.Matches(msg, m.keyMap.Quit) || msg.String() == "esc" {
				return m, func() tea.Msg { return formCancelMsg{} }
			}
			// tab / shift+tab navigate between fields inside the form. Without this
			// guard, FocusLeft (shift+tab) would call handleSwitchPane and move focus
			// to the list pane before huh ever sees the key.
			if m.focus == FocusRight {
				if key.Matches(msg, m.keyMap.FocusRight) || key.Matches(msg, m.keyMap.FocusLeft) {
					return m.updateFocusedPane(msg)
				}
			}
		}

		switch {
		case key.Matches(msg, m.keyMap.Quit):
			m.quitting = true
			return m, tea.Quit
		case key.Matches(msg, m.keyMap.FocusRight):
			// Tab: move focus to the right pane if not already there; otherwise
			// forward to the focused pane (e.g. tab inside a list filter).
			if m.focus != FocusRight {
				return m.handleSwitchPane()
			}
			return m.updateFocusedPane(msg)
		case key.Matches(msg, m.keyMap.FocusLeft):
			// Shift+tab: move focus to the left pane if not already there.
			if m.focus != FocusLeft {
				return m.handleSwitchPane()
			}
			return m.updateFocusedPane(msg)
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
		case key.Matches(msg, m.keyMap.AdvanceStage):
			return m.handleStageShift(+1)
		case key.Matches(msg, m.keyMap.RetreatStage):
			return m.handleStageShift(-1)
		case msg.String() == "esc" && m.statusBar.CaptureSticky():
			m.statusBar.SetCaptureText(CaptureBarPlaceholder())
			return m, nil
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
				return m, m.clearCaptureCmd()
			}
			initCmd := sp.form.Init()
			sized, _ := sp.Update(tea.WindowSizeMsg{
				Width:  m.layout.TotalWidth(),
				Height: m.layout.TotalHeight(),
			})
			m.rightPane = sized
			m.focus = FocusRight
			m.syncKeyHints()
			return m, initCmd
		}

		// Budget form: dispatches to the budget creation form.
		if nodeType == "budget" {
			var selectedID string
			if lp, ok := m.leftPane.(nodeListPane); ok {
				selectedID = lp.SelectedNodeID()
			}
			fp := newBudgetFormPane(m.theme, m.store, m.clock, selectedID, body, m.kinds, m.stageGroups)
			initCmd := fp.form.Init()
			sized, _ := fp.Update(tea.WindowSizeMsg{
				Width:  m.layout.TotalWidth(),
				Height: m.layout.TotalHeight(),
			})
			m.rightPane = sized
			m.focus = FocusRight
			m.syncKeyHints()
			return m, initCmd
		}

		var selectedID string
		if lp, ok := m.leftPane.(nodeListPane); ok {
			selectedID = lp.SelectedNodeID()
		}

		var fp formPane
		switch nodeType {
		case "journal":
			fp = newJournalFormPane(m.theme, m.store, m.clock, selectedID, body, m.kinds, m.stageGroups)
		case "note":
			fp = newNoteFormPane(m.theme, m.store, m.clock, selectedID, body, m.kinds, m.stageGroups)
		default:
			fp = newTaskFormPane(m.theme, m.store, m.clock, selectedID, body, m.kinds, m.stageGroups)
		}
		initCmd := fp.form.Init()
		sized, _ := fp.Update(tea.WindowSizeMsg{
			Width:  m.layout.TotalWidth(),
			Height: m.layout.TotalHeight(),
		})
		m.rightPane = sized
		m.focus = FocusRight
		m.syncKeyHints()
		return m, initCmd

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

	return m, m.clearCaptureCmd()
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
		fp = newEditJournalFormPane(m.theme, m.store, m.clock, m.index, node, m.kinds, m.stageGroups)
	case "note":
		fp = newEditNoteFormPane(m.theme, m.store, m.clock, m.index, node, m.kinds, m.stageGroups)
	case "budget":
		fp = newEditBudgetFormPane(m.theme, m.store, m.clock, m.index, node, m.kinds, m.stageGroups)
	default:
		fp = newEditTaskFormPane(m.theme, m.store, m.clock, m.index, node, m.kinds, m.stageGroups)
	}

	initCmd := fp.form.Init()
	sized, _ := fp.Update(tea.WindowSizeMsg{
		Width:  m.layout.TotalWidth(),
		Height: m.layout.TotalHeight(),
	})
	m.rightPane = sized
	m.focus = FocusRight
	m.syncKeyHints()
	return m, initCmd
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

	clearCmd := m.clearCaptureCmd()

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

	clearCmd := m.clearCaptureCmd()

	return m, clearCmd
}

// handleStageShift advances (dir > 0) or retreats (dir < 0) the selected
// node's stage by one step within its kind's stage group. It is a no-op when:
//   - a form is active
//   - no node is selected
//   - the node has no kind, or the kind is unknown
//   - the kind's stage group is unknown or the current stage is unknown to it
//
// When the node is already at a terminal boundary (CycleTerminate, no advance
// possible), no write is performed but a brief status hint is shown.
//
// The write routes through UpdateNode (not WriteNode) so the in-memory index
// is refreshed synchronously before the detail and dashboard re-render.
func (m Model) handleStageShift(dir int) (tea.Model, tea.Cmd) {
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

	// Resolve kind → stage group; silent no-op on any missing piece.
	group, ok := types.ResolveStageGroup(m.kinds, m.stageGroups, node)
	if !ok {
		return m, nil
	}

	var newStage string
	if dir > 0 {
		newStage, ok = group.Next(node.Stage)
	} else {
		newStage, ok = group.Prev(node.Stage)
	}
	if !ok {
		// Unknown current stage (e.g. legacy node with empty stage field) — no-op.
		return m, nil
	}

	if newStage == node.Stage {
		// Terminal boundary: no write needed, but show a hint so the keypress
		// isn't silent.
		m.statusBar.SetCaptureText("Stage: " + node.Stage + " (terminal)")
		return m, m.clearCaptureCmd()
	}

	oldStage := node.Stage
	if _, err := m.store.UpdateNode(nodeID, map[string]interface{}{"stage": newStage}); err != nil {
		return m, nil
	}
	m.statusBar.SetCaptureText("Stage: " + oldStage + " → " + newStage)

	// Refresh the dashboard so the updated stage is reflected in the list.
	if m.queryRunner != nil {
		dq := DefaultDashboardQuery()
		if m.store != nil {
			if view, err := m.store.ReadView("dashboard"); err == nil {
				dq = DashboardQueryFromView(view)
			}
		}
		if result, err := RunDashboard(m.queryRunner, m.clock, dq); err == nil {
			nlp := newNodeListPane(result, m.theme)
			sized, _ := nlp.Update(tea.WindowSizeMsg{
				Width:  m.layout.TotalWidth(),
				Height: m.layout.TotalHeight(),
			})
			m.leftPane = sized
		}
	}

	// Re-render the detail pane so the right side shows the new stage.
	detailCmd := m.renderDetailAsync(nodeID)
	return m, tea.Batch(detailCmd, m.clearCaptureCmd())
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
	m.logOverlay.theme = t
	m.helpOverlay.theme = t
	m.kindsOverlay.theme = t
	m.stagesOverlay.theme = t

	// Rebuild the node list pane so the delegate's baked-in Lipgloss styles
	// (section headers, row colours) repaint with the new theme.
	if lp, ok := m.leftPane.(nodeListPane); ok {
		m.leftPane = newNodeListPane(lp.result, t)
	} else if _, ok := m.leftPane.(emptyPane); ok {
		m.leftPane = NewEmptyPane(t)
	}

	// Always replace the right pane — never leave a stale viewportPane (old
	// theme bg baked in) or a stale formActivePane (traps ctrl+c as cancel).
	// Re-render the detail for the selected node; fall back to empty pane.
	rerendered := false
	if lp, ok := m.leftPane.(nodeListPane); ok {
		if id := lp.SelectedNodeID(); id != "" {
			m.rightPane = m.renderDetail(id)
			rerendered = true
		}
	}
	if !rerendered {
		m.rightPane = NewEmptyPane(t)
	}

	m.syncKeyHints()
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
		logoView := RenderLogo(m.layout.RightWidth(), m.theme)

		frame = m.layout.Render(leftView, rightView, logoView, statusView, m.focus)

		// Composite whichever overlay is active, horizontally and vertically
		// centred, over the base frame using lipgloss.Place. Only one overlay
		// is active at a time, so a switch is used to make that explicit and
		// avoid double-compositing. The ritual overlay holds its own width/height
		// from Open, so its View takes no size arguments.
		w, h := m.layout.totalWidth, m.layout.totalHeight
		switch {
		case m.palette.IsActive():
			frame = compositeOverlay(frame, m.palette.View(w, h), w, h)
		case m.logOverlay.IsActive():
			frame = compositeOverlay(frame, m.logOverlay.View(w, h), w, h)
		case m.helpOverlay.IsActive():
			frame = compositeOverlay(frame, m.helpOverlay.View(w, h), w, h)
		case m.kindsOverlay.IsActive():
			frame = compositeOverlay(frame, m.kindsOverlay.View(w, h), w, h)
		case m.stagesOverlay.IsActive():
			frame = compositeOverlay(frame, m.stagesOverlay.View(w, h), w, h)
		case m.ritualOvl.IsActive():
			frame = compositeOverlay(frame, m.ritualOvl.View(), w, h)
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
	renderer.Kinds = m.kinds
	renderer.StageGroups = m.stageGroups
	renderer.BlockedGlyph = m.theme.Glyphs().Blocked
	renderer.Colours.BgPrimary = m.theme.BgPrimary()
	renderer.Colours.FGPrimary = m.theme.FgPrimary()
	renderer.Colours.FGMuted = m.theme.FgMuted()
	renderer.Colours.AccentPrimary = m.theme.AccentPrimary()
	renderer.Colours.AccentSecondary = m.theme.AccentSecondary()
	renderer.Colours.BudgetOK = m.theme.BudgetOK()
	renderer.Colours.BudgetCaution = m.theme.BudgetCaution()
	renderer.Colours.BudgetOver = m.theme.BudgetOver()
	renderer.Colours.OverflowWarn = m.theme.OverflowWarn()
	renderer.Colours.OverflowCrit = m.theme.OverflowCritical()

	now := time.Now()
	if m.clock != nil {
		now = m.clock.Now()
	}

	content := renderer.Render(node, edges, nodesByID, nil, now)

	// Size the viewport to the right pane's inner dimensions. Borders consume
	// 2 columns (left+right). The logo pane sits above the detail box, so
	// vpHeight is the detail box's inner content height. PaneHeight() is the
	// outer height available for the whole right column. Subtract the outer logo
	// box (logoH) and the detail box's own 2 border rows to get the inner height.
	// heightOffset is stored on the viewport so subsequent WindowSizeMsg
	// events can keep the logo reservation in sync across terminal resizes.
	rw := m.layout.RightWidth()
	logoH := LogoHeight(rw)
	vpWidth := rw - 2
	vpHeight := m.layout.PaneHeight() - logoH - 2
	if vpWidth < 1 {
		vpWidth = 1
	}
	if vpHeight < 1 {
		vpHeight = 1
	}
	return newViewportPaneWithOffset(vpWidth, vpHeight, content, m.theme.BgPrimary(), logoH)
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
