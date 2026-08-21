package tui

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/jasonwarrenuk/wyrd/internal/types"
	"github.com/mattn/go-runewidth"
)

// newTestViewPane builds a viewPane with the built-in fallback theme, sized
// via a WindowSizeMsg — mirrors newTestEmptyPane in pane_internal_test.go.
func newTestViewPane(t *testing.T, view *types.SavedView, result types.QueryResult, termWidth, termHeight int) viewPane {
	t.Helper()
	theme, err := LoadTheme(".", "")
	if err != nil {
		t.Fatalf("LoadTheme: %v", err)
	}
	p := newViewPane(view, result, theme)
	updated, _ := p.Update(tea.WindowSizeMsg{Width: termWidth, Height: termHeight})
	pane, ok := updated.(viewPane)
	if !ok {
		t.Fatalf("Update returned %T, want viewPane", updated)
	}
	return pane
}

func listResult() types.QueryResult {
	return types.QueryResult{
		Columns: []string{"title", "status"},
		Rows: []map[string]interface{}{
			{"title": "First task", "status": "open"},
			{"title": "Second task", "status": "closed"},
		},
	}
}

// TestViewPane_ViewPadsToPaneWidth mirrors TestEmptyPane_ViewPadsToPaneWidth:
// the view pane is a LEFT-column pane, so its width follows
// nodeListPane/Layout.LeftWidth's convention (totalWidth/2, no -2 border
// subtraction) rather than viewportPane's right-column -2. Unlike the
// single-line empty-pane placeholder, list rendering is multi-line, so each
// line is measured individually — summing runewidth.StringWidth across
// embedded newlines would overcount.
func TestViewPane_ViewPadsToPaneWidth(t *testing.T) {
	view := &types.SavedView{Name: "today", Display: types.DisplayList}
	p := newTestViewPane(t, view, listResult(), 80, 24)
	out := p.View()

	want := 80 / 2
	for i, line := range strings.Split(stripANSI(out), "\n") {
		if got := runewidth.StringWidth(line); got != want {
			t.Errorf("line %d: expected padded width %d, got %d (line %q)", i, want, got, line)
		}
	}
}

// TestViewPane_PaddingCarriesThemeBackground mirrors
// TestEmptyPane_PaddingCarriesThemeBackground — the class of background-bleed
// bug CLAUDE.md flags as reintroduced 5+ times.
func TestViewPane_PaddingCarriesThemeBackground(t *testing.T) {
	view := &types.SavedView{Name: "today", Display: types.DisplayList}
	p := newTestViewPane(t, view, listResult(), 80, 24)
	out := p.View()

	if !ansiTruecolourBgRe.MatchString(out) {
		t.Fatalf("expected a truecolour background in output, got %q", out)
	}
	if strings.HasSuffix(out, " ") {
		t.Errorf("padding spaces trail the final ANSI sequence unstyled: %q", out)
	}
}

// TestViewPane_TimelinePaddingCarriesThemeBackground covers the same bleed
// contract for the timeline dispatch branch, not just list.
func TestViewPane_TimelinePaddingCarriesThemeBackground(t *testing.T) {
	view := &types.SavedView{Name: "journal", Display: types.DisplayTimeline}
	result := types.QueryResult{
		Columns: []string{"created", "body"},
		Rows: []map[string]interface{}{
			{"created": "2026-03-01T09:00:00Z", "body": "An entry"},
		},
	}
	p := newTestViewPane(t, view, result, 80, 24)
	out := p.View()

	if !ansiTruecolourBgRe.MatchString(out) {
		t.Fatalf("expected a truecolour background in timeline output, got %q", out)
	}
}

// scheduleResult returns a QueryResult shaped for DisplaySchedule — the
// default column names views.EntriesFromQueryResult reads.
func scheduleResult() types.QueryResult {
	return types.QueryResult{
		Columns: []string{"id", "title", "start", "duration", "energy"},
		Rows: []map[string]interface{}{
			{"id": "task-1", "title": "EPA review", "start": "2026-03-01T08:00:00Z", "duration": float64(90), "energy": "deep"},
		},
	}
}

// TestViewPane_SchedulePaddingCarriesThemeBackground covers the same bleed
// contract (CLAUDE.md's TUI styling rules) for the DisplaySchedule dispatch
// branch (TD.19), matching TestViewPane_TimelinePaddingCarriesThemeBackground.
func TestViewPane_SchedulePaddingCarriesThemeBackground(t *testing.T) {
	view := &types.SavedView{Name: "today-schedule", Display: types.DisplaySchedule}
	p := newTestViewPane(t, view, scheduleResult(), 80, 24)
	out := p.View()

	if !ansiTruecolourBgRe.MatchString(out) {
		t.Fatalf("expected a truecolour background in schedule output, got %q", out)
	}
}

// TestViewPane_DispatchesToSchedule extends the TD.13 dispatch contract test
// to DisplaySchedule (TD.19): the same result must render distinctly from
// list mode, and the adapted entry's title must actually reach the renderer
// — proving the query -> ScheduleEntry -> DisplacementResult pipeline wires
// end to end, not just that the switch branch is taken.
func TestViewPane_DispatchesToSchedule(t *testing.T) {
	result := scheduleResult()

	listView := &types.SavedView{Name: "v", Display: types.DisplayList, Columns: []string{"id", "title"}}
	scheduleView := &types.SavedView{Name: "v", Display: types.DisplaySchedule}

	listOut := stripANSI(newTestViewPane(t, listView, result, 80, 24).View())
	scheduleOut := stripANSI(newTestViewPane(t, scheduleView, result, 80, 24).View())

	if listOut == scheduleOut {
		t.Errorf("expected list and schedule rendering to differ, both produced:\n%s", listOut)
	}
	if !strings.Contains(scheduleOut, "EPA review") {
		t.Errorf("expected schedule mode to show the adapted entry title, got %q", scheduleOut)
	}
	if !strings.Contains(scheduleOut, "08:00") {
		t.Errorf("expected schedule mode to show the parsed start time, got %q", scheduleOut)
	}
}

// TestViewPane_ScheduleEmptyResultShowsPlaceholder covers a saved view whose
// query returns no rows: the adapter must produce zero entries rather than
// panicking, and the renderer falls back to its own empty-state message.
func TestViewPane_ScheduleEmptyResultShowsPlaceholder(t *testing.T) {
	view := &types.SavedView{Name: "empty-schedule", Display: types.DisplaySchedule}
	p := newTestViewPane(t, view, types.QueryResult{}, 80, 24)
	out := stripANSI(p.View())

	if !strings.Contains(out, "No tasks scheduled") {
		t.Errorf("expected empty-state message, got %q", out)
	}
}

// TestViewPane_ZeroWidthBeforeFirstResize covers the pre-resize case: before
// any WindowSizeMsg, the pane still uses its constructor default (80,
// matching nodeListPane's initialWidth) rather than zero or a negative
// width — View must not panic.
func TestViewPane_ZeroWidthBeforeFirstResize(t *testing.T) {
	theme, err := LoadTheme(".", "")
	if err != nil {
		t.Fatalf("LoadTheme: %v", err)
	}
	view := &types.SavedView{Name: "today", Display: types.DisplayList}
	p := newViewPane(view, listResult(), theme)

	out := stripANSI(p.View())
	if !strings.Contains(out, "First task") {
		t.Errorf("expected row content before first resize, got %q", out)
	}
}

// TestViewPane_DispatchesOnDisplayMode is the core TD.13 contract test:
// DisplayList and DisplayTimeline on the SAME QueryResult must produce
// visibly different output — proving SavedView.Display actually selects a
// renderer rather than the field being read and ignored (its state before
// this task).
func TestViewPane_DispatchesOnDisplayMode(t *testing.T) {
	result := types.QueryResult{
		Columns: []string{"created", "body"},
		Rows: []map[string]interface{}{
			{"created": "2026-03-01T09:00:00Z", "body": "An entry"},
		},
	}

	listView := &types.SavedView{Name: "v", Display: types.DisplayList, Columns: []string{"created", "body"}}
	timelineView := &types.SavedView{Name: "v", Display: types.DisplayTimeline}

	listOut := stripANSI(newTestViewPane(t, listView, result, 80, 24).View())
	timelineOut := stripANSI(newTestViewPane(t, timelineView, result, 80, 24).View())

	if listOut == timelineOut {
		t.Errorf("expected list and timeline rendering to differ, both produced:\n%s", listOut)
	}
	// List mode renders a column header row ("created" / "body" as text);
	// timeline mode never emits the raw column name as a literal token.
	if !strings.Contains(listOut, "created") {
		t.Errorf("expected list mode to show the column header, got %q", listOut)
	}
}

// TestViewPane_UnrecognisedDisplayFallsBackToList covers the documented
// fallback: an empty or unrecognised Display value renders as a list,
// matching today.jsonc's "display": "list" and any pre-TD.13 view file that
// predates the enum being read at all.
func TestViewPane_UnrecognisedDisplayFallsBackToList(t *testing.T) {
	view := &types.SavedView{Name: "today", Display: ""}
	p := newTestViewPane(t, view, listResult(), 80, 24)
	out := stripANSI(p.View())

	if !strings.Contains(out, "First task") {
		t.Errorf("expected list-mode row content for an empty Display, got %q", out)
	}
}

// TestViewPane_KeyBindingsEmpty documents the current (no scrolling, no row
// selection) contract — a deliberate scope boundary for this initial mount,
// not an oversight, so a future addition changes this test rather than
// silently drifting.
func TestViewPane_KeyBindingsEmpty(t *testing.T) {
	view := &types.SavedView{Name: "today", Display: types.DisplayList}
	p := newTestViewPane(t, view, listResult(), 80, 24)
	if len(p.KeyBindings()) != 0 {
		t.Errorf("expected no key bindings yet, got %v", p.KeyBindings())
	}
}

// TestViewPane_HandleFocusLostNoop guards against a panic on the no-op path.
func TestViewPane_HandleFocusLostNoop(t *testing.T) {
	view := &types.SavedView{Name: "today", Display: types.DisplayList}
	p := newTestViewPane(t, view, listResult(), 80, 24)
	if cmd := p.HandleFocusLost(); cmd != nil {
		t.Errorf("expected nil cmd from HandleFocusLost, got %v", cmd)
	}
}
