package tui

import (
	"bytes"
	"fmt"
	"strings"
	"time"

	"charm.land/bubbles/v2/cursor"
	"charm.land/bubbles/v2/key"
	"charm.land/bubbles/v2/spinner"
	tea "charm.land/bubbletea/v2"
	clog "github.com/charmbracelet/log"
	"github.com/jasonwarrenuk/wyrd/internal/cli"
	"github.com/jasonwarrenuk/wyrd/internal/stage"
	"github.com/jasonwarrenuk/wyrd/internal/tui/ritual"
	"github.com/jasonwarrenuk/wyrd/internal/tui/views"
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

// openKindFormMsg is emitted when the :kinds new command is invoked.
type openKindFormMsg struct{}

// openKindEditFormMsg is emitted when the :kinds edit <name> command is
// invoked. name is the raw, possibly-empty argument text — resolution
// (lookup, case-insensitive fallback, not-found handling) happens in the
// mount handler, which has the registries the command closure doesn't.
type openKindEditFormMsg struct {
	name string
}

// openStageFormMsg is emitted when the :stages new command is invoked.
type openStageFormMsg struct{}

// openStageEditFormMsg is emitted when the :stages edit <name> command is
// invoked. name is the raw, possibly-empty argument text — resolution
// happens in the mount handler, mirroring openKindEditFormMsg.
type openStageEditFormMsg struct {
	name string
}

// openStagesOverlayMsg is emitted when the bare :stages command is invoked.
type openStagesOverlayMsg struct{}

// openViewMsg is emitted when the :view <name> command is invoked (TD.13).
// name is the raw, possibly-empty argument text — resolution (store read,
// query execution, not-found handling) happens in the mount handler, which
// has access to m.store/m.queryRunner the command closure doesn't.
type openViewMsg struct {
	name string
}

// openRemapFormMsg is emitted when the :stages remap command is invoked.
type openRemapFormMsg struct{}

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

// lookupKindFold finds a kind by case-insensitive name match, for command
// entry points like ":kinds edit task" where a user types casually rather
// than matching the registry's exact stored casing. On ambiguity (two
// entries differing only by case — possible via a hand-edited kinds.jsonc)
// prefers an exact match if one somehow exists, else the first match in
// registry order.
func lookupKindFold(kinds *types.KindRegistry, name string) (types.Kind, bool) {
	for _, n := range kinds.Names() {
		if strings.EqualFold(n, name) {
			return kinds.Lookup(n)
		}
	}
	return types.Kind{}, false
}

// lookupStageGroupFold is lookupKindFold's twin for stage groups, used by
// ":stages edit" so a wrong-case name (":stages edit task-flow" typed as
// "Task-Flow") still resolves.
func lookupStageGroupFold(groups *types.StageGroupRegistry, name string) (types.StageGroup, bool) {
	for _, n := range groups.Names() {
		if strings.EqualFold(n, name) {
			return groups.Lookup(n)
		}
	}
	return types.StageGroup{}, false
}

// orphanAdvisory re-scans the graph against the current registries and, if
// the edit that just landed produced orphaned or unresolvable nodes, returns
// a suffix reporting them. Returns "" when there is nothing to flag or no
// index to scan. Call only after m.kinds/m.stageGroups have been reassigned
// to the freshly-merged registries — scanning against the stale ones reports
// nothing even when the edit just orphaned nodes.
//
// Orphans and Unresolvable are reported independently — report.IsEmpty()
// only checks Orphans, so relying on it here would silently drop a report
// that is all-unresolvable (e.g. nodes stranded by a partially failed
// rename cascade; see stage.RenameKind/RenameStageGroup). Nothing downstream
// can repair an Unresolvable node (ApplyRemap iterates report.Orphans only),
// so the wording deliberately doesn't point at :stages remap for that part.
func (m *Model) orphanAdvisory() string {
	if m.index == nil {
		return ""
	}
	report := stage.DetectOrphans(m.index, m.kinds, m.stageGroups)

	var advisory string
	if n := report.NodeCount(); n > 0 {
		advisory += fmt.Sprintf(" — %d node%s now hold orphaned stages; run :stages remap", n, plural(n))
	}
	if n := len(report.Unresolvable); n > 0 {
		advisory += fmt.Sprintf(" — %d node%s unresolvable (kind or group missing)", n, plural(n))
	}
	return advisory
}

// divergenceAdvisory turns a TD.5 stage.DivergenceReport into a startup
// status-bar message, or "" when there's nothing to flag. Unlike
// orphanAdvisory, this doesn't re-scan — the report is computed once by
// buildRegistries (cmd/wyrd/main.go) and passed straight through
// Config.Divergence, since nothing in a TUI session can itself cause new
// divergence (that only happens when the embedded defaults change, which
// requires a new binary).
//
// SchemaDrift is reported as a distinct, softer message: it means the
// Kind/StageGroup struct shape changed, not that the user edited anything,
// so pointing them at :kinds/:stages to "review drift" would send them
// looking for a divergence that isn't really there in the way the message
// implies.
func divergenceAdvisory(report stage.DivergenceReport) string {
	if report.SchemaDrift {
		return "Shadow-provenance hashes are stale after an app update; re-save a kind/stage-group edit to refresh them"
	}
	n := len(report.Diverged)
	if n == 0 {
		return ""
	}
	suffix := "ies"
	if n == 1 {
		suffix = "y"
	}
	return fmt.Sprintf("%d shadowed kind/stage-group entr%s diverged from upstream defaults — see :kinds / :stages", n, suffix)
}

// ritualTriggerMsg is sent when a ritual should be presented to the user.
type ritualTriggerMsg struct {
	ritual *types.Ritual
}

// ritualCheckTickMsg fires on a timer to check whether any rituals are due.
type ritualCheckTickMsg struct{}

// clockTickMsg fires once per wall-clock minute boundary so the status bar's
// HH:MM clock (TD.16) stays live rather than only updating on the next
// unrelated render. The handler carries no payload and re-arms itself.
type clockTickMsg struct{}

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

	// leftPaneIsProseView is true when the current left pane is a
	// DisplayProse saved view's row list (TD.20). It is a nodeListPane like
	// the ordinary dashboard, so currentSelectedID() and nodeSelectedMsg
	// work unchanged — this flag exists solely to route nodeSelectedMsg to
	// renderProseAsync/proseReadyMsg instead of the default
	// renderDetailAsync/detailReadyMsg pair, so the right pane renders via
	// ProseRenderer (with resolved edge titles) rather than DetailRenderer
	// for this saved view's rows. Reset to false by MountLeft and
	// refreshDashboard (every ordinary left-pane mount); set true
	// explicitly by openViewMsg's DisplayProse branch, the only place that
	// mounts a prose-backed nodeListPane.
	leftPaneIsProseView bool

	// dashboardCols is the column set resolved at startup (saved dashboard
	// view columns when present, package default otherwise). Dashboard
	// refreshes re-read the view but fall back to this, so custom columns
	// survive captures, edits, archives and stage shifts.
	dashboardCols []string

	// rightPane is the right-side content (detail / editor).
	rightPane PaneModel

	// focus indicates which pane has keyboard focus.
	focus FocusedPane

	// prevFocus is the focus value as of the end of the previous Update call.
	// Update (the exported wrapper) diffs focus against prevFocus after every
	// message is handled, regardless of which internal path changed it, and
	// kicks off focusAnim from that single choke-point (VP.6).
	prevFocus FocusedPane

	// focusAnim holds the in-flight spring-eased focus-border transition, or
	// nil when no transition is running (rendered as a hard, un-animated
	// state). See focus_anim.go.
	focusAnim *focusTransition

	// reduceMotion disables focusAnim entirely (VP.6 accessibility gate) —
	// read once at startup from config.jsonc's reduce_motion.
	reduceMotion bool

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

	// staleThresholdDays is the DL.3 idle-days threshold read from
	// config.jsonc (staleness_threshold_days); <= 0 resolves to
	// types.DefaultStalenessThresholdDays via types.IsStale.
	staleThresholdDays int

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

	// divergence is the TD.5 report of shadowed kinds/stage groups that have
	// drifted from the upstream default they were forked from. Computed once
	// by buildRegistries and carried on the Model so the startup advisory and
	// both overlays agree — previously each overlay recomputed its own
	// partial report (stagesOverlay passing nil for kinds), which pooled
	// checked/mismatched counts differently and could disagree with the
	// startup advisory on the very same on-disk state. Re-pointed alongside
	// kinds/stageGroups at every rebuild site.
	divergence stage.DivergenceReport

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

// proseReadyMsg is detailReadyMsg's TD.20 sibling: the result of an async
// ProseRenderer render, delivered when the left pane is a DisplayProse
// saved view's row list (leftPaneIsProseView) rather than the ordinary
// dashboard.
type proseReadyMsg struct {
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

	// Divergence reports which shadowed kinds/stage groups have drifted from
	// the upstream default they were forked from (TD.5). Computed by
	// buildRegistries alongside Kinds/StageGroups — the zero value (an empty
	// report) is safe and renders no advisory or overlay markers.
	Divergence stage.DivergenceReport

	// Logger is the structured logger. May be nil.
	Logger *clog.Logger
}

// New builds the initial App Model. It may be called with an empty / nil store.
func New(cfg Config) (Model, error) {
	storePath := cfg.StorePath
	if storePath == "" {
		storePath = "."
	}

	// Attempt to read config for theme name, the DL.3 staleness threshold,
	// and the VP.6 reduce_motion accessibility gate.
	themeName := cfg.ThemeName
	staleThresholdDays := 0
	reduceMotion := false
	if cfg.Store != nil {
		if appCfg, err := cfg.Store.ReadConfig(); err == nil {
			if themeName == "" && appCfg.Theme != "" {
				themeName = appCfg.Theme
			}
			staleThresholdDays = appCfg.StalenessThresholdDays
			reduceMotion = appCfg.ReduceMotion
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
	statusBar.SetClock(clock)

	// Build the initial left pane. When a QueryRunner is available, run the
	// dashboard query and mount the result. If the query fails (e.g. empty
	// store, no matching nodes), fall back to the empty placeholder so the
	// app still launches cleanly.
	//
	// A saved view named "dashboard" in the store overrides the default queries
	// and columns. Individual keys in view.Queries override only the matching
	// category; missing keys fall back to DefaultDashboardQuery.
	leftPane := NewEmptyPane(theme)
	cols := dashboardColumns
	if cfg.QueryRunner != nil {
		dq := DefaultDashboardQuery()
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
			leftPane = newNodeListPane(result, theme, staleThresholdDays)
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

	// Wire up the "kinds" command. ":kinds" lists all kinds (SL.9); ":kinds
	// new" opens the kind creation form (SL.10); ":kinds edit <name>" opens
	// the kind edit form pre-populated from the existing entry (SL.16).
	palette.Register(Command{
		Name:        "kinds",
		Description: "List kinds (kinds new | kinds edit <name>)",
		Execute: func(args []string) tea.Cmd {
			if len(args) > 0 && args[0] == "new" {
				return func() tea.Msg { return openKindFormMsg{} }
			}
			if len(args) > 0 && args[0] == "edit" {
				// strings.Join collapses runs of internal whitespace in a
				// multi-word name, which is acceptable — strings.Fields
				// tokenising the raw command line has already destroyed that
				// information by the time args reaches here.
				name := strings.Join(args[1:], " ")
				return func() tea.Msg { return openKindEditFormMsg{name: name} }
			}
			return func() tea.Msg { return openKindsOverlayMsg{} }
		},
	})

	// Wire up the "stages" command. ":stages" lists all stage groups (SL.12);
	// ":stages new" opens the stage-group creation form (SL.11); ":stages
	// remap" scans for orphaned stages and opens the remap form (SL.14);
	// ":stages edit <name>" opens the stage-group edit form pre-populated
	// from the existing entry (SL.17).
	palette.Register(Command{
		Name:        "stages",
		Description: "List stage groups (stages new | stages edit <name> | stages remap)",
		Execute: func(args []string) tea.Cmd {
			if len(args) > 0 && args[0] == "new" {
				return func() tea.Msg { return openStageFormMsg{} }
			}
			if len(args) > 0 && args[0] == "remap" {
				return func() tea.Msg { return openRemapFormMsg{} }
			}
			if len(args) > 0 && args[0] == "edit" {
				// See the matching comment on the "kinds" command: strings.Join
				// collapses internal whitespace runs in a multi-word name,
				// acceptable since strings.Fields already destroyed that
				// information tokenising the raw command line.
				name := strings.Join(args[1:], " ")
				return func() tea.Msg { return openStageEditFormMsg{name: name} }
			}
			return func() tea.Msg { return openStagesOverlayMsg{} }
		},
	})

	// Wire up the "view" command (TD.13): ":view <name>" loads a saved view
	// from the store, runs its query, and mounts the left pane using the
	// renderer matching its Display mode. A bare ":view" (no argument)
	// restores the node-list dashboard instead of doing nothing — msg.name
	// stays "" and the openViewMsg handler branches on that, mirroring the
	// esc key's restore path below. Returning a real command here (rather
	// than nil) also sidesteps PaletteState's typed-input path, which has no
	// needs-args recovery and would otherwise just close the palette
	// silently on a bare ":view" + Enter.
	palette.Register(Command{
		Name:        "view",
		Description: "Open a saved view by name (e.g. view today); bare view restores the dashboard",
		Execute: func(args []string) tea.Cmd {
			name := strings.Join(args, " ")
			return func() tea.Msg { return openViewMsg{name: name} }
		},
	})

	m := Model{
		theme:              theme,
		storePath:          storePath,
		layout:             layout,
		leftPane:           leftPane,
		dashboardCols:      cols,
		rightPane:          NewEmptyPane(theme),
		focus:              FocusLeft,
		prevFocus:          FocusLeft,
		reduceMotion:       reduceMotion,
		keyMap:             keyMap,
		palette:            palette,
		statusBar:          statusBar,
		store:              cfg.Store,
		captureBar:         captureBar,
		queryRunner:        cfg.QueryRunner,
		index:              cfg.Index,
		clock:              clock,
		staleThresholdDays: staleThresholdDays,
		detailRenderer:     NewDetailRenderer(),
		schedulerState:     schedulerState,
		rituals:            rituals,
		kinds:              cfg.Kinds,
		stageGroups:        cfg.StageGroups,
		divergence:         cfg.Divergence,
		logger:             cfg.Logger,
		logOverlay:         newLogOverlay(theme),
		helpOverlay:        newHelpOverlay(theme),
		kindsOverlay:       newKindsOverlay(theme, cfg.Kinds, cfg.StageGroups),
		stagesOverlay:      newStagesOverlay(theme, cfg.StageGroups),
		ready:              false,
	}
	m.kindsOverlay.divergence = cfg.Divergence
	m.stagesOverlay.divergence = cfg.Divergence

	// Pre-populate the right pane with the first selected item so the detail
	// pane is not blank on startup.
	if id := m.currentSelectedID(); id != "" {
		m.rightPane = m.renderDetail(id)
		if m.index != nil {
			if node, err := m.index.GetNode(id); err == nil {
				edgeCount := len(m.index.EdgesFrom(id)) + len(m.index.EdgesTo(id))
				m.statusBar.SetNodeInfo(node.ID, node.Types, edgeCount)
			}
		}
	}

	// Populate initial keybind hints for the focused (left) pane.
	m.syncKeyHints()

	// TD.5 startup advisory: surface upstream-default divergence, if any.
	// Sticky (not the usual 2s auto-clear) since this is worth reading
	// rather than a transient confirmation, and only set when there's
	// something to say — an empty advisory would otherwise stomp on
	// whatever placeholder text the capture bar already shows.
	if advisory := divergenceAdvisory(cfg.Divergence); advisory != "" {
		m.statusBar.SetCaptureText(advisory)
		m.statusBar.MarkCaptureSticky()
	}

	return m, nil
}

// Init returns the initial command. We fire the ritual check tick immediately
// so any due rituals are presented on launch, then every 60 seconds thereafter.
// The clock tick (TD.16) is seeded alongside it so the status-bar clock
// starts ticking from launch.
func (m Model) Init() tea.Cmd {
	return tea.Batch(ritualCheckTick(), m.clockTick())
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

// delayToNextMinute returns how long to wait from now until the next
// wall-clock minute boundary. Split out from clockTick as a pure function so
// the boundary-alignment maths is unit-testable without going through
// tea.Tick, whose returned tea.Cmd is an opaque closure.
func delayToNextMinute(now time.Time) time.Duration {
	next := now.Truncate(time.Minute).Add(time.Minute)
	delay := next.Sub(now)
	if delay <= 0 {
		delay = time.Minute
	}
	return delay
}

// clockTick returns a tea.Cmd that fires a clockTickMsg at the next
// wall-clock minute boundary (per m.clock, so it honours an injected
// types.StubClock in tests), rather than a flat 60s interval — a flat
// interval free-runs from whenever it happened to be armed and can leave
// the displayed HH:MM up to 59s stale relative to the real minute change.
// Not gated behind reduce_motion: that flag is documented as disabling
// spring-eased animation (VP.6), and gating a once-a-minute digit change
// behind it would leave those users a permanently stale clock — a
// correctness regression, not an accessibility win.
func (m Model) clockTick() tea.Cmd {
	now := time.Now()
	if m.clock != nil {
		now = m.clock.Now()
	}
	return tea.Tick(delayToNextMinute(now), func(_ time.Time) tea.Msg {
		return clockTickMsg{}
	})
}

// Update is the Elm-style update function's public entry point. It delegates
// to update for all actual message handling, then acts as the single VP.6
// choke-point: focus is set directly in roughly a dozen places scattered
// through update's many early-return branches (handleSwitchPane, capture/
// edit/archive flow returns, etc.), so rather than instrument every one of
// them, this wrapper diffs the resulting focus against prevFocus exactly
// once per call and kicks off (or re-targets) the spring transition from
// here — the only place guaranteed to run after every focus change.
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	newModel, cmd := m.update(msg)

	next, ok := newModel.(Model)
	if !ok {
		// Should not happen — every internal path returns a Model — but fall
		// back to passing the result through unanimated rather than panic.
		return newModel, cmd
	}

	if !next.reduceMotion && next.focus != next.prevFocus {
		gen := 0
		if next.focusAnim != nil {
			gen = next.focusAnim.gen
		}
		next.focusAnim = newFocusTransition(gen, next.prevFocus, next.focus)
		cmd = tea.Batch(cmd, next.focusAnim.tick())
	}
	next.prevFocus = next.focus

	return next, cmd
}

// viewportOverlayActive reports whether any of the four viewport overlays
// (log/help/kinds/stages) is currently open. The message-open cases below
// (openKindFormMsg and friends) must check this before mounting a pane into
// rightPane: those messages aren't consumed by the overlays' Update (see
// keyOverlay's contract), so they fall through the dispatch loop in update
// and reach this switch while an overlay is still active. Without this
// guard the form mounts underneath the overlay — View composites the
// overlay over whatever rightView already rendered, so the form is
// invisible but still focused, silently eating keystrokes.
func (m Model) viewportOverlayActive() bool {
	return m.logOverlay.IsActive() || m.helpOverlay.IsActive() ||
		m.kindsOverlay.IsActive() || m.stagesOverlay.IsActive()
}

// update is the Elm-style update function. All state changes happen here.
//
// Overlay routing order: ritual overlay, then capture bar, then the four
// viewport overlays (log/help/kinds/stages), then the command palette — each
// a guard that intercepts and returns early while active. This is the
// *reverse* of View's compositing order (palette-first, ritual-last — see
// View's switch), which looks like a mismatch but isn't a bug: only one
// overlay is ever active at a time (mounting a second viewport overlay while
// one is open is guarded via viewportOverlayActive at each open-message
// case below; the palette and ritual overlay are only reachable from key
// presses the active viewport overlay would otherwise have consumed, so they
// need no separate guard), so the two orderings never actually compete for
// the same message. Don't reorder either side to "fix" this — a reorder
// changes real precedence when two guards' activation conditions ever do
// overlap (e.g. a future overlay stacking feature) and neither order has been
// asked for over the other. TestAtMostOneOverlayActive-style assertions are
// the correct investment here, not a reorder.
func (m Model) update(msg tea.Msg) (tea.Model, tea.Cmd) {
	// When the ritual overlay is active, route messages to it first. Kept
	// bespoke rather than folded into the []keyOverlay loop below: unlike the
	// four viewport overlays, closing this one has a side effect beyond
	// hiding the box — the post-Update close-detection here calls
	// m.schedulerState.Dismiss and re-arms the scheduler via
	// ritualCheckTickNext(), which is the *other* site (besides the
	// ritualCheckTickMsg handler itself) responsible for keeping the ritual
	// timer alive. A generic loop has nowhere to hang that logic without
	// either special-casing ritualOverlay inside the loop (defeating the
	// point of making it generic) or leaking scheduler concerns into
	// keyOverlay's contract.
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

	// When one of the four viewport overlays (log, help, kinds, stages) is
	// active, route input to it first. These are pure UI with no cross-overlay
	// state, so a single ranged dispatch is safe — unlike the ritual and
	// palette guards below, which stay bespoke for reasons documented at each.
	// Each overlay declines (nil, false) for anything it doesn't handle (see
	// keyOverlay's doc comment), so a ritual tick, resize or spinner tick
	// still reaches the switch below even while one of these is open.
	for _, ovl := range []keyOverlay{&m.logOverlay, &m.helpOverlay, &m.kindsOverlay, &m.stagesOverlay} {
		if !ovl.IsActive() {
			continue
		}
		cmd, consumed := ovl.Update(msg)
		if consumed {
			return m, cmd
		}
		break
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
		// Guard against opening a second overlay on top of one already active.
		if m.viewportOverlayActive() {
			return m, nil
		}
		m.logOverlay.Open(m.layout.totalWidth, m.layout.totalHeight)
		return m, nil

	case openHelpOverlayMsg:
		if m.viewportOverlayActive() {
			return m, nil
		}
		m.helpOverlay.Open(m.layout.totalWidth, m.layout.totalHeight, m.keyMap.AllBindings())
		return m, nil

	case openKindsOverlayMsg:
		if m.viewportOverlayActive() {
			return m, nil
		}
		m.kindsOverlay.Open(m.layout.totalWidth, m.layout.totalHeight)
		return m, nil

	case openStagesOverlayMsg:
		if m.viewportOverlayActive() {
			return m, nil
		}
		m.stagesOverlay.Open(m.layout.totalWidth, m.layout.totalHeight)
		return m, nil

	case openViewMsg:
		// Guard against clobbering an active form, or mounting a viewPane
		// invisibly underneath a still-open viewport overlay (see
		// viewportOverlayActive's doc comment) — the same guard every other
		// open-message handler in this switch carries. A viewPane mounting
		// under an open form is worse here than for the sibling form-open
		// messages: it would also silently pull the rug from under the
		// selectedID lookups a couple of the capture-form branches above
		// depend on (m.leftPane.(nodeListPane) stops matching).
		if _, isForm := m.rightPane.(formActivePane); isForm || m.viewportOverlayActive() {
			return m, nil
		}

		// A bare ":view" (no argument) restores the node-list dashboard
		// rather than trying to look up a view named "". This is the
		// palette-driven twin of the esc restore path below — both exist
		// because mounting a viewPane replaces the left pane wholesale, and
		// nothing else in the app can put a nodeListPane back once that
		// happens (see the leftPaneIsView/esc handling for the fuller
		// rationale).
		if msg.name == "" {
			m.refreshDashboard()
			m.statusBar.SetCaptureText("Restored dashboard")
			return m, m.clearCaptureCmd()
		}

		// TD.13: load a saved view, run its query, and mount the left pane
		// with the renderer matching its Display mode. Read through
		// StoreFS.ReadView — the same interface seam refreshDashboard and
		// cli.RunView already use — rather than views.LoadViews, which
		// bypasses StoreFS's typed errors and diverges on which file
		// extensions it accepts.
		if m.store == nil || m.queryRunner == nil {
			m.statusBar.SetCaptureText("View unavailable: no store or query runner")
			return m, m.clearCaptureCmd()
		}
		view, err := m.store.ReadView(msg.name)
		if err != nil {
			m.statusBar.SetCaptureText(fmt.Sprintf("No view %q", msg.name))
			return m, m.clearCaptureCmd()
		}
		result, err := m.queryRunner.Run(view.Query, m.clock)
		if err != nil {
			m.statusBar.SetCaptureText(fmt.Sprintf("View %q query failed: %v", msg.name, err))
			m.statusBar.MarkCaptureSticky()
			return m, nil
		}

		// DisplayProse (TD.20) mounts a nodeListPane, not a viewPane: the
		// left pane shows the query's rows exactly like the ordinary
		// dashboard does, and the right pane renders the selected row's
		// full prose via the parallel nodeSelectedMsg route below
		// (leftPaneIsProseView) rather than viewPane's own single-QueryResult
		// rendering, which has no per-row "selected node" concept at all.
		// This also means DisplayProse rows need real node IDs: a query
		// must alias a scalar id column (RETURN n.id AS id, ...), the same
		// convention DefaultDashboardQuery uses — a bare "RETURN n" (binding
		// the whole node under its match variable, not under "id") produces
		// rows nodeListPane can't resolve to a selectable ID.
		if view.Display == types.DisplayProse {
			lp := newNodeListPane(*result, m.theme, m.staleThresholdDays)
			sized, _ := lp.Update(tea.WindowSizeMsg{
				Width:  m.layout.TotalWidth(),
				Height: m.layout.TotalHeight(),
			})
			m.MountLeft(sized)
			m.leftPaneIsProseView = true
			m.statusBar.SetCaptureText(fmt.Sprintf("Opened view %q", msg.name))
			return m, m.clearCaptureCmd()
		}

		vp := newViewPane(view, *result, m.theme)
		sized, _ := vp.Update(tea.WindowSizeMsg{
			Width:  m.layout.TotalWidth(),
			Height: m.layout.TotalHeight(),
		})
		m.MountLeft(sized)

		// DisplayBudget isn't wired to a renderer yet (see viewPane's doc
		// comment) and silently falls back to list rendering. Still mount
		// so the user's data is visible, but flag the mode by name via a
		// sticky message rather than leaving the fallback unexplained —
		// mirrors the sticky error pattern just above for the query-failure
		// case. DisplayProse came out of this switch in TD.20: it now has
		// a real renderer via the nodeListPane branch above.
		switch view.Display {
		case types.DisplayBudget:
			m.statusBar.SetCaptureText(fmt.Sprintf(
				"View %q uses %s display, not yet supported — showing as list", msg.name, view.Display))
			m.statusBar.MarkCaptureSticky()
			return m, nil
		}

		m.statusBar.SetCaptureText(fmt.Sprintf("Opened view %q", msg.name))
		return m, m.clearCaptureCmd()

	case openKindFormMsg:
		// Guard against clobbering an active form, or mounting a form
		// invisibly underneath a still-open viewport overlay (see
		// viewportOverlayActive's doc comment).
		if _, isForm := m.rightPane.(formActivePane); isForm || m.viewportOverlayActive() {
			return m, nil
		}
		fp := newKindFormPane(m.theme, m.store, m.kinds, m.stageGroups, nil)
		return m.mountForm(fp)

	case openKindEditFormMsg:
		// Guard against clobbering an active form, or mounting a form
		// invisibly underneath a still-open viewport overlay.
		if _, isForm := m.rightPane.(formActivePane); isForm || m.viewportOverlayActive() {
			return m, nil
		}
		if msg.name == "" {
			m.statusBar.SetCaptureText("Usage: :kinds edit <name>")
			return m, m.clearCaptureCmd()
		}
		if m.kinds == nil {
			m.statusBar.SetCaptureText("Edit unavailable: no kind registry")
			return m, m.clearCaptureCmd()
		}
		k, ok := m.kinds.Lookup(msg.name)
		if !ok {
			// Exact lookup failed — try case-insensitive, matching the
			// collision validator's own case-insensitivity, so ":kinds edit
			// task" finds "Task" the way a user typing casually would expect.
			k, ok = lookupKindFold(m.kinds, msg.name)
		}
		if !ok {
			m.statusBar.SetCaptureText(fmt.Sprintf("No kind %q — see :kinds", msg.name))
			return m, m.clearCaptureCmd()
		}
		fp := newKindFormPane(m.theme, m.store, m.kinds, m.stageGroups, &k)
		return m.mountForm(fp)

	case openStageFormMsg:
		// Guard against clobbering an active form, or mounting a form
		// invisibly underneath a still-open viewport overlay.
		if _, isForm := m.rightPane.(formActivePane); isForm || m.viewportOverlayActive() {
			return m, nil
		}
		fp := newStageFormPane(m.theme, m.store, m.stageGroups, nil)
		return m.mountForm(fp)

	case openStageEditFormMsg:
		// Guard against clobbering an active form, or mounting a form
		// invisibly underneath a still-open viewport overlay.
		if _, isForm := m.rightPane.(formActivePane); isForm || m.viewportOverlayActive() {
			return m, nil
		}
		if msg.name == "" {
			m.statusBar.SetCaptureText("Usage: :stages edit <name>")
			return m, m.clearCaptureCmd()
		}
		if m.stageGroups == nil {
			m.statusBar.SetCaptureText("Edit unavailable: no stage-group registry")
			return m, m.clearCaptureCmd()
		}
		g, ok := m.stageGroups.Lookup(msg.name)
		if !ok {
			g, ok = lookupStageGroupFold(m.stageGroups, msg.name)
		}
		if !ok {
			m.statusBar.SetCaptureText(fmt.Sprintf("No stage group %q — see :stages", msg.name))
			return m, m.clearCaptureCmd()
		}
		fp := newStageFormPane(m.theme, m.store, m.stageGroups, &g)
		return m.mountForm(fp)

	case openRemapFormMsg:
		// Guard against clobbering an active form, or mounting a form
		// invisibly underneath a still-open viewport overlay.
		if _, isForm := m.rightPane.(formActivePane); isForm || m.viewportOverlayActive() {
			return m, nil
		}
		if m.store == nil || m.index == nil {
			m.statusBar.SetCaptureText("Remap unavailable: no store or index")
			return m, m.clearCaptureCmd()
		}
		report := stage.DetectOrphans(m.index, m.kinds, m.stageGroups)
		if report.IsEmpty() {
			text := "No orphaned stages"
			if n := len(report.Unresolvable); n > 0 {
				text += fmt.Sprintf(" (%d node%s unresolvable — kind or group missing)", n, plural(n))
			}
			m.statusBar.SetCaptureText(text)
			return m, m.clearCaptureCmd()
		}
		if len(report.Orphans) > maxRemapOrphans {
			m.statusBar.SetCaptureText(fmt.Sprintf(
				"Too many orphaned stage combinations (%d) to remap here — fix stages.jsonc/kinds.jsonc directly",
				len(report.Orphans),
			))
			m.statusBar.MarkCaptureSticky()
			return m, nil
		}
		fp := newRemapFormPane(m.theme, m.store, report)
		return m.mountForm(fp)

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

	case focusTickMsg:
		// Stale tick from a transition that's since been superseded by
		// another focus change (see Update's choke-point) — drop it.
		if m.focusAnim == nil || msg.gen != m.focusAnim.gen {
			return m, nil
		}
		m.focusAnim.step()
		if m.focusAnim.settled() {
			m.focusAnim = nil
			return m, nil
		}
		return m, m.focusAnim.tick()
	}

	// Let the palette handle input first when it is active. Position matters:
	// this sits after switch #1 above and before switch #2 below — load-bearing
	// for ":kinds" and friends, whose Execute functions emit an
	// openKindsOverlayMsg etc. that switch #1 handles on the *next* Update
	// call, once the palette has closed and stopped intercepting.
	//
	// Only key presses and the textinput cursor blink are intercepted here;
	// everything else (ritual ticks, window resize, spinner ticks) falls
	// through to the switch below so those keep working while the palette is
	// open. PaletteState.Update's own non-key tail unconditionally forwards
	// to ps.input.Update, which would otherwise swallow those messages too —
	// see the matching keyOverlay contract this mirrors.
	if m.palette.IsActive() {
		switch msg.(type) {
		case tea.KeyPressMsg, cursor.BlinkMsg:
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

	case clockTickMsg:
		// No state to update — StatusBar.View reads m.clock live on every
		// render. This tick's only job is to cause a render at all, then
		// re-arm itself for the next minute boundary.
		return m, m.clockTick()

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
			// Render asynchronously so Glamour initialisation doesn't block
			// the event loop. The right pane shows a placeholder until the
			// detailReadyMsg (or, for a DisplayProse saved view, TD.20's
			// proseReadyMsg) arrives.
			m.rightPane = m.sizedEmptyPane(m.theme)
			var cmd tea.Cmd
			if m.leftPaneIsProseView {
				cmd = m.renderProseAsync(msg.nodeID)
			} else {
				cmd = m.renderDetailAsync(msg.nodeID)
			}
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
		// Result of async detail rendering. Fast j/j/j selection spawns
		// multiple render goroutines with no ordering guarantee, so a slow
		// render for an earlier selection can land after a later one's —
		// drop any result that no longer matches the current selection.
		// msg.nodeID == "" is a deliberate pass-through: renderDetail
		// handles an empty ID by returning an empty pane, and that path
		// must keep working (e.g. an empty node list).
		if msg.nodeID != "" && msg.nodeID != m.currentSelectedID() {
			return m, nil
		}
		// Mount the pane unless a form has taken over the right pane in the
		// meantime.
		if _, isForm := m.rightPane.(formActivePane); !isForm {
			m.rightPane = msg.pane
		}
		return m, nil

	case proseReadyMsg:
		// TD.20 sibling of detailReadyMsg — same staleness guard, same
		// form guard. Kept as a separate message type (rather than
		// dispatching inside detailReadyMsg on leftPaneIsProseView) so a
		// slow detail render that outlives a fast switch to/from a prose
		// view can never land in the wrong renderer's slot.
		if msg.nodeID != "" && msg.nodeID != m.currentSelectedID() {
			return m, nil
		}
		if _, isForm := m.rightPane.(formActivePane); !isForm {
			m.rightPane = msg.pane
		}
		return m, nil

	case formSubmitMsg:
		m.unmountForm()
		return m.handleCaptureSubmit(captureSubmitMsg(msg))

	case formCancelMsg:
		m.unmountFormToDetail()
		return m, nil

	case editSubmitMsg:
		m.unmountForm()
		return m.handleEditSubmit(msg)

	case spendSubmitMsg:
		m.unmountForm()
		captureText := fmt.Sprintf("Recorded %.2f to %s", msg.amount, msg.category)
		if msg.warning != "" {
			captureText += "; warning: " + msg.warning
		}
		m.statusBar.SetCaptureText(captureText)
		m.refreshDashboard()
		return m, m.clearCaptureCmd()

	case kindFormSubmitMsg:
		m.unmountForm()

		// A rename must move every node off the old kind name before
		// anything below scans for orphans — the registry no longer has an
		// entry named renamedFrom (upsertKind removed/replaced it), so any
		// node still holding that name would resolve as Unresolvable, not
		// Orphaned, and nothing downstream can repair that class (see
		// stage.RenameKind's doc comment). A partial failure here is
		// reported but does not block the registry refresh below — the
		// nodes that did move are correctly reflected either way.
		var renameErr error
		var renameCount int
		if msg.renamedFrom != "" && m.store != nil && m.index != nil {
			renameCount, renameErr = stage.RenameKind(m.store, m.index, msg.renamedFrom, msg.name)
		}

		// Rebuild the in-memory kind registry so the edit is usable
		// in-session without restarting. DefaultKinds is sync.Once-cached, so
		// re-calling it is cheap.
		refreshed := false
		if m.store != nil {
			if defaults, err := stage.DefaultKinds(); err == nil {
				if userReg, err := m.store.ReadKinds(); err == nil {
					// MergeKinds' returned registry tracks which names came
					// from userReg itself (TD.15), so re-pointing the
					// overlay at the freshly-merged registry is enough to
					// keep its (custom)/(edited) markers correct post-write
					// — no separate userNames rebuild needed here anymore.
					m.kinds = stage.MergeKinds(defaults, userReg.All())
					m.kindsOverlay.kinds = m.kinds
					refreshed = true
				}
			}
		}

		// Recompute the single divergence report alongside the registry
		// rebuild above, and re-point both overlays at it — see the
		// Model.divergence doc comment for why this must stay one shared
		// report rather than each overlay recomputing its own.
		if refreshed {
			m.divergence = stage.DetectDiverged(m.kinds, m.stageGroups)
			m.kindsOverlay.divergence = m.divergence
			m.stagesOverlay.divergence = m.divergence
		}

		// renamedFrom empty covers both a same-name edit and a genuine
		// create — kindFormSubmitMsg doesn't distinguish them, and the
		// remap hand-off below doesn't need to either, so both keep the
		// existing "Created" text rather than threading an extra flag
		// through the message for a distinction nothing downstream uses.
		text := fmt.Sprintf("Created kind %q", msg.name)
		if msg.renamedFrom != "" {
			text = fmt.Sprintf("Renamed %q to %q", msg.renamedFrom, msg.name)
		}
		if renameErr != nil {
			text = fmt.Sprintf("Renamed %q to %q, but %v", msg.renamedFrom, msg.name, renameErr)
		}

		// Only scan for orphans once m.kinds actually reflects the write, and
		// only hand off to the remap form on a clean rename — a partial
		// RenameKind failure leaves some nodes still holding renamedFrom,
		// which no longer resolves against the rebuilt registry at all. Those
		// nodes land in report.Unresolvable, not report.Orphans (nothing
		// downstream can repair that class), so auto-opening the remap form
		// here would either miss them entirely or route the user into a form
		// that can't fix the actual problem. On a rename failure, report the
		// unresolvable count via orphanAdvisory instead and let the sticky
		// error message (below) stay on screen.
		if refreshed && renameErr == nil {
			report := stage.DetectOrphans(m.index, m.kinds, m.stageGroups)
			if !report.IsEmpty() && len(report.Orphans) <= maxRemapOrphans {
				n := report.NodeCount()
				text = fmt.Sprintf("%s — %d node%s need a new stage", text, n, plural(n))
				m.statusBar.SetCaptureText(text)
				// Safe to chain: m.rightPane was set to an empty pane above,
				// so openRemapFormMsg's formActivePane guard passes when
				// this message is processed on the next Update call.
				return m, tea.Batch(m.clearCaptureCmd(), func() tea.Msg { return openRemapFormMsg{} })
			}
			text += m.orphanAdvisory()
		} else if refreshed && renameErr != nil {
			text += m.orphanAdvisory()
		}
		if renameCount > 0 && renameErr == nil {
			text += fmt.Sprintf(" (%d node%s moved)", renameCount, plural(renameCount))
		}
		m.statusBar.SetCaptureText(text)
		if renameErr != nil {
			m.statusBar.MarkCaptureSticky()
			return m, nil
		}
		return m, m.clearCaptureCmd()

	case kindFormErrorMsg:
		m.unmountForm()
		m.statusBar.SetCaptureText("Could not save kind: " + msg.err.Error())
		return m, m.clearCaptureCmd()

	case stageFormSubmitMsg:
		m.unmountForm()

		// A rename must repoint every kind referencing the old group name
		// before anything below rebuilds the kind registry — the group
		// registry no longer has an entry named renamedFrom (upsertStageGroup
		// removed/replaced it), so any kind still pointing at that name would
		// fail to resolve its stage group at all (types.ResolveStageGroup
		// returns false), and every node of that kind becomes Unresolvable
		// rather than a fixable Orphan. See stage.RenameStageGroup's doc
		// comment for the full fan-out reasoning: this touches every KIND
		// referencing the group, not nodes directly.
		var renameErr error
		var renameCount int
		if msg.renamedFrom != "" && m.store != nil {
			renameCount, renameErr = stage.RenameStageGroup(m.store, msg.renamedFrom, msg.name)
		}

		// Rebuild the in-memory kind registry too — RenameStageGroup writes
		// to kinds.jsonc (rewriting/shadowing every referencing kind), so a
		// group rename can change m.kinds even though the edited entity was
		// a group. Cheap to always attempt: a no-op ReadKinds when nothing
		// changed just re-derives the same registry.
		if renameErr == nil && msg.renamedFrom != "" && m.store != nil {
			if kindDefaults, err := stage.DefaultKinds(); err == nil {
				if userKindReg, err := m.store.ReadKinds(); err == nil {
					// See the matching TD.15 note in kindFormSubmitMsg:
					// MergeKinds' registry now tracks provenance itself.
					m.kinds = stage.MergeKinds(kindDefaults, userKindReg.All())
					m.kindsOverlay.kinds = m.kinds
				}
			}
		}

		// Rebuild the in-memory stage-group registry so the edit is usable
		// in-session without restarting. DefaultStageGroups is
		// sync.Once-cached, so re-calling it is cheap.
		refreshed := false
		if m.store != nil {
			if defaults, err := stage.DefaultStageGroups(); err == nil {
				if userReg, err := m.store.ReadStages(); err == nil {
					// MergeStageGroups' registry tracks provenance itself
					// (TD.15) — no separate userNames rebuild needed.
					m.stageGroups = stage.MergeStageGroups(defaults, userReg.All())
					m.kindsOverlay.stageGroups = m.stageGroups
					m.stagesOverlay.stageGroups = m.stageGroups
					refreshed = true
				}
			}
		}

		// Recompute the shared divergence report now that both kinds (above,
		// on a group rename's fan-out) and stage groups are current — see
		// the Model.divergence doc comment.
		if refreshed {
			m.divergence = stage.DetectDiverged(m.kinds, m.stageGroups)
			m.kindsOverlay.divergence = m.divergence
			m.stagesOverlay.divergence = m.divergence
		}

		text := fmt.Sprintf("Created stage group %q", msg.name)
		if msg.renamedFrom != "" {
			text = fmt.Sprintf("Renamed %q to %q", msg.renamedFrom, msg.name)
		}
		if renameErr != nil {
			text = fmt.Sprintf("Renamed %q to %q, but %v", msg.renamedFrom, msg.name, renameErr)
		}

		// See the matching guard in kindFormSubmitMsg: only scan once
		// m.stageGroups actually reflects the write, and only hand off to the
		// remap form on a clean rename — a partial RenameStageGroup failure
		// leaves some kinds still pointing at renamedFrom, which no longer
		// resolves at all, so those kinds' nodes land in report.Unresolvable
		// rather than report.Orphans (nothing downstream can repair that
		// class). This scan also picks up the fan-out case: removing or
		// renaming a stage that several kinds share (task-flow referenced by
		// Task, Goblin, and Talk) produces one Orphan entry per affected kind
		// — DetectOrphans' OrphanKey is keyed by (Kind, Stage), so the shared
		// group's edit surfaces as multiple rows in the remap form rather
		// than one.
		if refreshed && renameErr == nil {
			report := stage.DetectOrphans(m.index, m.kinds, m.stageGroups)
			if !report.IsEmpty() && len(report.Orphans) <= maxRemapOrphans {
				n := report.NodeCount()
				text = fmt.Sprintf("%s — %d node%s need a new stage", text, n, plural(n))
				m.statusBar.SetCaptureText(text)
				// Safe to chain: m.rightPane was set to an empty pane above,
				// so openRemapFormMsg's formActivePane guard passes when
				// this message is processed on the next Update call.
				return m, tea.Batch(m.clearCaptureCmd(), func() tea.Msg { return openRemapFormMsg{} })
			}
			text += m.orphanAdvisory()
		} else if refreshed && renameErr != nil {
			text += m.orphanAdvisory()
		}
		if renameCount > 0 && renameErr == nil {
			text += fmt.Sprintf(" (%d kind%s repointed)", renameCount, plural(renameCount))
		}
		m.statusBar.SetCaptureText(text)
		if renameErr != nil {
			m.statusBar.MarkCaptureSticky()
			return m, nil
		}
		return m, m.clearCaptureCmd()

	case stageFormErrorMsg:
		m.unmountForm()
		m.statusBar.SetCaptureText("Could not save stage group: " + msg.err.Error())
		return m, m.clearCaptureCmd()

	case remapFormSubmitMsg:
		m.unmountForm()
		text := fmt.Sprintf("Remapped %d node%s", msg.remapped, plural(msg.remapped))
		if msg.unchanged > 0 {
			text += fmt.Sprintf(", left %d unchanged", msg.unchanged)
		}
		m.statusBar.SetCaptureText(text)
		m.refreshDashboard()
		return m, m.clearCaptureCmd()

	case remapFormErrorMsg:
		m.unmountForm()
		text := "Remap failed: " + msg.err.Error()
		if msg.remapped > 0 {
			text = fmt.Sprintf("Remapped %d node%s before failing: %s", msg.remapped, plural(msg.remapped), msg.err.Error())
		}
		m.statusBar.SetCaptureText(text)
		m.statusBar.MarkCaptureSticky()
		m.refreshDashboard()
		return m, nil

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
		case msg.String() == "esc" && m.leftPaneNeedsRestore():
			// Mounting a viewPane replaces the left pane wholesale (see
			// openViewMsg), and every route back to the node list —
			// ctrl+o/ctrl+d/[/] edit/archive/stage-shift, theme rebuilds —
			// is gated on a nodeListPane type assertion that silently fails
			// once it's gone. Without this, the only way back was creating
			// a throwaway node (refreshDashboard runs as a side effect of
			// capture) or restarting. Ordered after the CaptureSticky case
			// above so dismissing a sticky message always takes priority.
			m.refreshDashboard()
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

// mountForm sizes fp to the current layout, mounts it in the right pane, and
// gives it keyboard focus — the shared prologue that used to be duplicated at
// each of the five sites that open a kindFormPane, stageFormPane or
// remapFormPane (see formMountable's doc comment for why exactly these five
// and not the task/journal/note/budget/spend forms). initForm() is called
// after Update/resize, matching the call order every duplicated site already
// used, so this is a pure extraction rather than a behaviour change.
func (m Model) mountForm(fp formMountable) (tea.Model, tea.Cmd) {
	initCmd := fp.initForm()
	sized, _ := fp.Update(tea.WindowSizeMsg{
		Width:  m.layout.TotalWidth(),
		Height: m.layout.TotalHeight(),
	})
	m.rightPane = sized
	m.focus = FocusRight
	m.syncKeyHints()
	return m, initCmd
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
	m.refreshDashboard()
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
	m.refreshDashboard()

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
	label = truncateDisplay(label, 40)
	m.statusBar.SetCaptureText("Archived " + label)

	// Refresh the dashboard so the archived node disappears from the list.
	m.refreshDashboard()

	// Clear the right pane and return focus to the list.
	m.rightPane = m.sizedEmptyPane(m.theme)
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
	m.refreshDashboard()

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
		m.leftPane = newNodeListPane(lp.result, t, lp.staleThresholdDays)
	} else if _, ok := m.leftPane.(emptyPane); ok {
		m.leftPane = m.sizedEmptyPane(t)
	} else if vp, ok := m.leftPane.(viewPane); ok {
		// viewPane retains view/result specifically so it can be rebuilt
		// with a new theme without re-running the saved view's query —
		// previously there was no branch for it here at all, so a viewPane
		// kept pointing at the pre-switch *ActiveTheme (viewPane.theme is
		// only read by its own palette builders) and rendered in the old
		// theme's colours while the rest of the frame repainted. Set theme
		// directly rather than going through newViewPane, which would reset
		// width back to its 80-column default and undo the last resize.
		vp.theme = t
		m.leftPane = vp
	}

	// Always replace the right pane — never leave a stale viewportPane (old
	// theme bg baked in) or a stale formActivePane (traps ctrl+c as cancel).
	// Re-render the detail for the selected node; fall back to empty pane.
	rerendered := false
	if id := m.currentSelectedID(); id != "" {
		m.rightPane = m.renderDetail(id)
		rerendered = true
	}
	if !rerendered {
		m.rightPane = m.sizedEmptyPane(t)
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

		var anim *FocusAnimState
		if m.focusAnim != nil {
			anim = &FocusAnimState{
				From:     m.focusAnim.from,
				To:       m.focusAnim.to,
				Progress: m.focusAnim.progress(),
			}
		}

		frame = m.layout.Render(leftView, rightView, logoView, statusView, m.focus, anim)

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
// mount their view implementations. Always clears leftPaneIsProseView
// (TD.20) — the DisplayProse branch in openViewMsg's handler sets it back
// to true immediately after calling this, since that is the only mount
// site that needs it set.
func (m *Model) MountLeft(pane PaneModel) {
	m.leftPane = pane
	m.leftPaneIsProseView = false
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

// unmountForm clears an active form from the right pane back to the app's
// resting state: an empty pane, focus returned to the left, and key hints
// re-synced. This is the shared tail for the eight form-completion messages
// (formSubmitMsg, editSubmitMsg, spendSubmitMsg, kindFormSubmitMsg,
// kindFormErrorMsg, stageFormSubmitMsg, stageFormErrorMsg,
// remapFormSubmitMsg, remapFormErrorMsg) whose next step is either a no-op
// right pane or a caller-specific status message — every one of them wants
// exactly this prologue and nothing else. formCancelMsg is the odd one out
// (see unmountFormToDetail) because cancelling restores the node detail view
// rather than leaving the pane empty.
func (m *Model) unmountForm() {
	m.rightPane = m.sizedEmptyPane(m.theme)
	m.focus = FocusLeft
	m.syncKeyHints()
}

// unmountFormToDetail is formCancelMsg's variant of unmountForm: instead of
// always mounting an empty pane, it restores the currently selected node's
// detail view when there is a selection, falling back to empty only when
// there isn't. Uses currentSelectedID (TD.7) rather than a cached ID, for the
// same staleness reasons that helper's doc comment gives.
func (m *Model) unmountFormToDetail() {
	m.focus = FocusLeft
	m.syncKeyHints()
	if id := m.currentSelectedID(); id != "" {
		m.rightPane = m.renderDetail(id)
		return
	}
	m.rightPane = m.sizedEmptyPane(m.theme)
}

// renderDetailAsync returns a tea.Cmd that renders the detail pane in a
// goroutine, sending a detailReadyMsg when complete. This keeps the event
// loop responsive while Glamour processes markdown.
// refreshDashboard re-runs the dashboard queries and remounts the node list
// pane sized to the current layout. It re-reads the saved dashboard view for
// both the queries and the column set, falling back to the columns resolved
// at startup — previously the refresh sites dropped the columns argument, so
// custom view columns silently reverted to the defaults after any capture,
// edit, archive or stage shift. No-op when no query runner is wired, or when
// the refreshed query fails (the stale list beats an empty one).
func (m *Model) refreshDashboard() {
	if m.queryRunner == nil {
		return
	}
	dq := DefaultDashboardQuery()
	cols := m.dashboardCols
	if m.store != nil {
		if view, err := m.store.ReadView("dashboard"); err == nil {
			dq = DashboardQueryFromView(view)
			if len(view.Columns) > 0 {
				cols = view.Columns
			}
		}
	}
	if result, err := RunDashboard(m.queryRunner, m.clock, dq, cols); err == nil {
		lp := newNodeListPane(result, m.theme, m.staleThresholdDays)
		sized, _ := lp.Update(tea.WindowSizeMsg{
			Width:  m.layout.TotalWidth(),
			Height: m.layout.TotalHeight(),
		})
		// MountLeft rather than a direct assignment: it also calls
		// syncKeyHints, which matters whenever the previous left pane was a
		// non-nodeListPane (e.g. a viewPane restored via esc/bare :view) —
		// those panes report no key hints of their own, and a direct
		// assignment here would leave the status bar showing them stale
		// until the next unrelated focus change.
		m.MountLeft(sized)
	}
}

// sizedEmptyPane returns an empty pane pre-sized from the current layout, so
// placeholders mounted between resize events (form close, node selection,
// theme switch) still pad their background to the pane width.
func (m Model) sizedEmptyPane(theme *ActiveTheme) PaneModel {
	p := NewEmptyPane(theme)
	if w := m.layout.TotalWidth(); w > 0 {
		p, _ = p.Update(tea.WindowSizeMsg{Width: w, Height: m.layout.TotalHeight()})
	}
	return p
}

// renderProseAsync is renderDetailAsync's TD.20 sibling for a DisplayProse
// saved view's row list.
func (m Model) renderProseAsync(nodeID string) tea.Cmd {
	return func() tea.Msg {
		pane := m.renderProse(nodeID)
		return proseReadyMsg{nodeID: nodeID, pane: pane}
	}
}

func (m Model) renderDetailAsync(nodeID string) tea.Cmd {
	return func() tea.Msg {
		pane := m.renderDetail(nodeID)
		return detailReadyMsg{nodeID: nodeID, pane: pane}
	}
}

// currentSelectedID returns the UUID of the currently highlighted node in
// the left pane, or an empty string when the left pane isn't a nodeListPane
// or has no selection. Deliberately not mirrored into a Model field: the
// list pane mutates its own selection internally without emitting a
// root-visible message on every move, so a cached field would drift from
// the real selection — reintroducing the staleness class this helper exists
// to close.
func (m Model) currentSelectedID() string {
	if lp, ok := m.leftPane.(nodeListPane); ok {
		return lp.SelectedNodeID()
	}
	return ""
}

// leftPaneNeedsRestore reports whether the left pane is currently something
// other than the node-list dashboard and an emptyPane — i.e. a viewPane
// mounted via :view. Only viewPane is checked explicitly, rather than
// negating nodeListPane, so a future non-dashboard, non-view pane type
// doesn't silently pick up an esc-restore behaviour nobody asked for.
func (m Model) leftPaneNeedsRestore() bool {
	_, ok := m.leftPane.(viewPane)
	return ok
}

// renderDetail fetches a node by ID and renders it into a detailPane.
// Returns an emptyPane when the index is unavailable or the node is not found.
func (m Model) renderDetail(nodeID string) PaneModel {
	if m.index == nil || nodeID == "" {
		return m.sizedEmptyPane(m.theme)
	}
	node, err := m.index.GetNode(nodeID)
	if err != nil {
		return m.sizedEmptyPane(m.theme)
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
	renderer.StaleGlyph = m.theme.Glyphs().Stale
	renderer.StaleThresholdDays = m.staleThresholdDays
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

// renderProse is renderDetail's TD.20 sibling: same viewport sizing and
// nodesByID/edges collection, but rendered via views.ProseRenderer instead
// of DetailRenderer, for a DisplayProse saved view's selected row. Edge
// ageing uses the theme's FgMuted/OverflowWarn/OverflowCritical trio,
// matching renderDetail's own choice — there is no separate "prose ageing"
// concept in the theme.
func (m Model) renderProse(nodeID string) PaneModel {
	if m.index == nil || nodeID == "" {
		return m.sizedEmptyPane(m.theme)
	}
	node, err := m.index.GetNode(nodeID)
	if err != nil {
		return m.sizedEmptyPane(m.theme)
	}

	edges := append(m.index.EdgesFrom(nodeID), m.index.EdgesTo(nodeID)...)

	allNodes := m.index.AllNodes()
	nodesByID := make(map[string]*types.Node, len(allNodes))
	for _, n := range allNodes {
		nodesByID[n.ID] = n
	}

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

	renderer := views.NewProseRenderer()
	renderer.Palette.Background = m.theme.BgPrimary()
	renderer.Palette.Title = m.theme.AccentPrimary()
	renderer.Palette.Body = m.theme.FgPrimary()
	renderer.Palette.MetaKey = m.theme.AccentSecondary()
	renderer.Palette.MetaValue = m.theme.FgPrimary()
	renderer.Palette.EdgeType = m.theme.AccentSecondary()
	renderer.Palette.EdgeGlyph = m.theme.FgMuted()
	renderer.Palette.Separator = m.theme.Border()
	renderer.Palette.Muted = m.theme.FgMuted()
	renderer.Palette.AgeWarn = m.theme.OverflowWarn()
	renderer.Palette.AgeCritical = m.theme.OverflowCritical()

	now := time.Now()
	if m.clock != nil {
		now = m.clock.Now()
	}

	// FillBackground repaints Glamour's un-backgrounded blank lines and
	// inter-block gaps: ProseRenderer deliberately doesn't call this itself
	// (package views cannot import package tui, where FillBackground
	// lives), so the caller applies it here, exactly as renderDetail's own
	// renderMarkdown does for DetailRenderer.
	content := FillBackground(renderer.Render(node, edges, nodesByID, now, vpWidth), m.theme.BgPrimary())

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
