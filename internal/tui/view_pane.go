package tui

import (
	"image/color"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/jasonwarrenuk/wyrd/internal/tui/views"
	"github.com/jasonwarrenuk/wyrd/internal/types"
)

// viewPane is a PaneModel that renders a types.SavedView's Display mode
// (TD.13) — mounting the internal/tui/views renderers, which until now had
// no importer anywhere in the binary. It replaces the left (node-list) pane
// when a saved view is opened via the :view palette command.
//
// DisplayList, DisplayTimeline, and DisplaySchedule (TD.19) are wired here.
// DisplayList and DisplayTimeline take a types.QueryResult directly, the
// same shape the dashboard already produces, so they mount with no data
// adapter. DisplaySchedule needs views.EntriesFromQueryResult to bridge
// types.QueryResult to the []views.ScheduleEntry displacement.Calculate
// requires; there is no "currently being timed" concept anywhere in the
// codebase yet (see docs/agent-guide/06-views-schedule.md), so
// ActiveEntryID and Elapsed are always zero-valued here — displacement never
// triggers until that lands separately. DisplayProse (a single node + its
// edges — a right-pane shape, not a left-pane one) and DisplayBudget
// ([]*types.Node — needs a row-id-to-node hydration step) remain
// deliberately out of scope — see the roadmap for their follow-up tasks.
//
// views cannot import package tui (circular — views/timeline.go's padLine
// exists specifically because of this), so the palette-to-theme adaptation
// and the final PadLines wrap both live here rather than in the renderers
// themselves.
type viewPane struct {
	view   *types.SavedView
	result types.QueryResult
	theme  *ActiveTheme
	width  int
	clock  types.Clock
}

// Compile-time check: viewPane must satisfy PaneModel.
var _ PaneModel = viewPane{}

// newViewPane constructs a viewPane for the given saved view and query
// result. theme may be nil (renders with zero-value colours, matching
// emptyPane's nil-theme tolerance); width is the initial pane width, before
// any WindowSizeMsg — 80 matches nodeListPane's initialWidth constant. clock
// defaults to types.RealClock{} — only DisplaySchedule rendering uses it, to
// compute DayEnd.
func newViewPane(view *types.SavedView, result types.QueryResult, theme *ActiveTheme) viewPane {
	return viewPane{view: view, result: result, theme: theme, width: 80, clock: types.RealClock{}}
}

// Update tracks the pane width from resize events. The rendered content is
// recomputed from result on every View() call rather than cached, mirroring
// how the renderers themselves are stateless — cheap for the row counts a
// saved view realistically returns, and it avoids a second copy of the
// resize-then-rebuild logic nodeListPane needs for its stateful
// bubbles/list component (this pane has no equivalent stateful child).
func (v viewPane) Update(msg tea.Msg) (PaneModel, tea.Cmd) {
	if wmsg, ok := msg.(tea.WindowSizeMsg); ok {
		v.width = wmsg.Width / 2
		if v.width < 1 {
			v.width = 1
		}
	}
	return v, nil
}

// View dispatches on v.view.Display and renders the result through the
// matching views renderer, building its palette from the active theme
// (TD.6) rather than the renderer's own hardcoded Default*Palette().
// Falls back to list rendering for a nil view, an empty Display value, or
// an unrecognised one, matching the DisplayList behaviour today.jsonc
// already ships with. The nil case is unreachable via newViewPane today
// (Store.ReadView never returns a nil view with a nil error) but every
// other pane in this package tolerates its nil inputs, so View does too.
func (v viewPane) View() string {
	bg, _ := v.themeColours()

	var content string
	switch {
	case v.view == nil:
		content = ""
	case v.view.Display == types.DisplayTimeline:
		content = v.renderTimeline()
	case v.view.Display == types.DisplaySchedule:
		content = v.renderSchedule()
	default:
		content = v.renderList()
	}

	return PadLines(content, v.width, bg)
}

// themeColours resolves the pane background/foreground, tolerating a nil
// theme the same way nodeListPane.View does.
func (v viewPane) themeColours() (bg, fg color.Color) {
	if v.theme == nil {
		return nil, nil
	}
	return v.theme.BgPrimary(), v.theme.FgPrimary()
}

// listPalette builds a views.ListPalette from the active theme. Header uses
// AccentPrimary (matching the dashboard's own column-header colouring
// convention elsewhere in the package), Selection the theme's Selection
// colour, Foreground/Muted/Background straight off their theme
// counterparts.
func (v viewPane) listPalette() views.ListPalette {
	if v.theme == nil {
		return views.DefaultListPalette()
	}
	return views.ListPalette{
		Header:     v.theme.AccentPrimary(),
		Selection:  v.theme.Selection(),
		Foreground: v.theme.FgPrimary(),
		Muted:      v.theme.FgMuted(),
		Background: v.theme.BgPrimary(),
	}
}

// timelinePalette builds a views.TimelinePalette from the active theme.
func (v viewPane) timelinePalette() views.TimelinePalette {
	if v.theme == nil {
		return views.DefaultTimelinePalette()
	}
	return views.TimelinePalette{
		DateHeader: v.theme.AccentPrimary(),
		Separator:  v.theme.Border(),
		Body:       v.theme.FgPrimary(),
		Muted:      v.theme.FgMuted(),
		Background: v.theme.BgPrimary(),
	}
}

// renderList renders v.result via views.ListRenderer. selectedIdx is always
// -1: the view pane has no row-selection concept of its own (unlike
// nodeListPane's bubbles/list cursor) — selecting into a saved view's rows
// is out of scope for this initial mount.
func (v viewPane) renderList() string {
	r := views.NewListRenderer(v.view.Columns)
	r.Palette = v.listPalette()
	return r.Render(v.result, -1, v.width)
}

// renderTimeline renders v.result via views.TimelineRenderer in its plain
// (non-block) mode.
func (v viewPane) renderTimeline() string {
	r := views.NewTimelineRenderer()
	r.Palette = v.timelinePalette()
	return r.View(v.result, v.width)
}

// renderSchedule renders v.result via views.EntriesFromQueryResult and
// views.Calculate. ActiveEntryID and Elapsed are always zero-valued: no part
// of the codebase yet tracks "the task currently being timed" (see
// docs/agent-guide/06-views-schedule.md), so displacement compression never
// triggers until that timer concept is built separately. DayEnd is the next
// local midnight after clock.Now() — there is no work-day-boundary config to
// source it from, so this is the simplest correct default: the schedule
// window covers the rest of today.
func (v viewPane) renderSchedule() string {
	entries := views.EntriesFromQueryResult(v.result, views.ScheduleColumns{})

	clock := v.clock
	if clock == nil {
		clock = types.RealClock{}
	}
	now := clock.Now()
	dayEnd := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location()).AddDate(0, 0, 1)

	result := views.Calculate(views.DisplacementInput{
		Entries: entries,
		DayEnd:  dayEnd,
		Clock:   clock,
	})

	// r.Width is left at NewScheduleRenderer's default: ScheduleRenderer.Render
	// doesn't currently read it (fill bars size off the fixed fillBarWidth
	// constant), so setting it from v.width here would claim a layout effect
	// that doesn't exist. Revisit once the renderer actually consumes Width.
	r := views.NewScheduleRenderer()
	return r.Render(result)
}

// KeyBindings returns an empty slice — the view pane has no keys of its own
// yet (no scrolling, no row selection); it is read-only content.
func (v viewPane) KeyBindings() []KeyBinding {
	return nil
}

// HandleFocusLost is a no-op for the view pane.
func (v viewPane) HandleFocusLost() tea.Cmd { return nil }
