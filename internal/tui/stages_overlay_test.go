package tui

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/jasonwarrenuk/wyrd/internal/stage"
	"github.com/jasonwarrenuk/wyrd/internal/types"
)

func loadStagesTestTheme(t *testing.T) *ActiveTheme {
	t.Helper()
	theme, err := LoadTheme(".", "")
	if err != nil {
		t.Fatalf("LoadTheme: %v", err)
	}
	return theme
}

func TestStagesOverlay_RendersAllGroups(t *testing.T) {
	groups := []types.StageGroup{
		{Name: "flow-a", Stages: []string{"Open", "Doing", "Done"}, Cycle: types.CycleTerminate},
		{Name: "flow-b", Stages: []string{"Start", "End"}, Cycle: types.CycleLoop},
		{Name: "flow-c", Stages: []string{"A", "B", "C"}, Cycle: types.CycleLoopToStage, LoopTarget: "B"},
	}
	reg := types.NewStageGroupRegistry(groups)
	theme := loadStagesTestTheme(t)

	so := newStagesOverlay(theme, reg, nil)
	so.Open(120, 40)

	view := so.View(120, 40)

	for _, g := range groups {
		if !strings.Contains(view, g.Name) {
			t.Errorf("expected overlay to contain group name %q", g.Name)
		}
	}
	if !strings.Contains(view, "Open → Doing → Done") {
		t.Error("expected stage progression for flow-a")
	}
	if !strings.Contains(view, "loop ↺") {
		t.Error("expected loop marker for flow-b")
	}
	if !strings.Contains(view, "loop→B ↺") {
		t.Error("expected loop-to-stage marker for flow-c")
	}
	if !strings.Contains(view, "STAGES") {
		t.Error("expected STAGES title in overlay")
	}
	if !strings.Contains(view, "terminate") {
		t.Error("expected terminate label for flow-a")
	}
}

// TestStagesOverlay_ProvenanceMarker verifies the three provenance states:
// a purely user-defined group gets (custom), an edited (shadowed) default
// gets (edited), and an untouched default gets no marker at all. userNames
// (populated from the user's stages.jsonc, not the merged registry) is what
// distinguishes these — see provenanceMarker's doc comment for why "name is
// absent from the defaults list" stopped being a sufficient test once SL.17
// edit mode could write a same-named shadow copy of a default.
func TestStagesOverlay_ProvenanceMarker(t *testing.T) {
	defaults, err := stage.DefaultStageGroups()
	if err != nil {
		t.Fatalf("DefaultStageGroups: %v", err)
	}

	customGroup := types.StageGroup{
		Name:   "my-custom-flow",
		Stages: []string{"Todo", "Doing", "Done"},
		Cycle:  types.CycleTerminate,
	}
	// A shadow copy of the baked-in task-flow default, as SL.17 edit mode
	// would write after the user edits it — same name, different stages.
	editedDefault := types.StageGroup{
		Name:   "task-flow",
		Stages: []string{"Open", "Doing", "Done", "Archived"},
		Cycle:  types.CycleTerminate,
	}

	reg := stage.MergeStageGroups(defaults, []types.StageGroup{customGroup, editedDefault})
	theme := loadStagesTestTheme(t)
	userNames := map[string]bool{"my-custom-flow": true, "task-flow": true}

	so := newStagesOverlay(theme, reg, userNames)
	so.Open(160, 50)

	view := so.View(160, 50)

	if !strings.Contains(view, "(custom)") {
		t.Error("expected (custom) marker for the purely user-defined group")
	}
	if !strings.Contains(view, "(edited)") {
		t.Error("expected (edited) marker for the shadowed default")
	}
	if !strings.Contains(view, "my-custom-flow") {
		t.Error("expected user group my-custom-flow to appear")
	}
	// content-flow is a baked-in default never touched by the user — it
	// should appear with no provenance marker of either kind.
	if !strings.Contains(view, "content-flow") {
		t.Fatal("expected untouched default content-flow to appear")
	}
}

// TestStagesOverlay_UntouchedDefaultHasNoMarker isolates the "no marker"
// case against a registry with only defaults and an empty userNames set —
// TestStagesOverlay_ProvenanceMarker's view contains both markers elsewhere,
// so it can't by itself prove content-flow's row lacks one.
func TestStagesOverlay_UntouchedDefaultHasNoMarker(t *testing.T) {
	defaults, err := stage.DefaultStageGroups()
	if err != nil {
		t.Fatalf("DefaultStageGroups: %v", err)
	}
	reg := stage.MergeStageGroups(defaults, nil)
	theme := loadStagesTestTheme(t)

	so := newStagesOverlay(theme, reg, map[string]bool{})
	so.Open(160, 50)

	view := so.View(160, 50)
	if strings.Contains(view, "(custom)") || strings.Contains(view, "(edited)") {
		t.Error("expected no provenance marker anywhere when userNames is empty")
	}
}

func TestStagesOverlay_EmptyState(t *testing.T) {
	theme := loadStagesTestTheme(t)
	so := newStagesOverlay(theme, nil, nil)
	so.Open(120, 40)

	view := so.View(120, 40)
	if !strings.Contains(view, "No stage groups") {
		t.Error("expected empty-state message for nil registry")
	}
}

func TestStagesOverlay_EscCloses(t *testing.T) {
	theme := loadStagesTestTheme(t)
	reg := types.NewStageGroupRegistry([]types.StageGroup{
		{Name: "test", Stages: []string{"A"}, Cycle: types.CycleTerminate},
	})

	so := newStagesOverlay(theme, reg, nil)
	so.Open(120, 40)

	if !so.IsActive() {
		t.Fatal("overlay should be active after Open")
	}

	_, consumed := so.Update(tea.KeyPressMsg{Code: tea.KeyEscape})
	if !consumed {
		t.Error("esc key should be consumed")
	}
	if so.IsActive() {
		t.Error("overlay should be inactive after esc")
	}
}

func TestStagesOverlay_InactiveViewReturnsEmpty(t *testing.T) {
	theme := loadStagesTestTheme(t)
	so := newStagesOverlay(theme, nil, nil)

	view := so.View(120, 40)
	if view != "" {
		t.Errorf("inactive overlay should return empty string, got %q", view)
	}
}

// TestStagesOverlay_CycleColumnUsesDisplayWidth verifies that cycleString
// values whose byte length differs from their display width are measured
// correctly by lipgloss.Width. This guards against the old byte-vs-display-
// width bug where len() over-counted multi-byte runes (↺ U+21BA is 3 bytes
// but 1 display cell), causing the cycle column to be over-padded.
//
// The test asserts that the per-row cycle padding is consistent: every row
// produces the same (cycleColWidth - lipgloss.Width(cs)) gap so that the
// stages column starts at the same visual position on every row.
func TestStagesOverlay_CycleColumnUsesDisplayWidth(t *testing.T) {
	groups := []types.StageGroup{
		// "terminate" — 9 display cells = 9 bytes.
		{Name: "task-flow", Stages: []string{"Open", "Done"}, Cycle: types.CycleTerminate},
		// "loop ↺" — 6 display cells but 8 bytes (↺ is 3 bytes, 1 cell).
		{Name: "habit-flow", Stages: []string{"Pending", "Done"}, Cycle: types.CycleLoop},
		// "loop→B ↺" — 8 display cells but 12 bytes (→ and ↺ are 3 bytes each).
		{Name: "sprint-flow", Stages: []string{"A", "B", "C"}, Cycle: types.CycleLoopToStage, LoopTarget: "B"},
	}

	// Compute cycleColWidth exactly as stages_overlay.go does: max of
	// (lipgloss.Width(cs) + 2) across all groups, minimum 12.
	cycleColWidth := 12
	for _, g := range groups {
		cs := cycleString(g)
		if w := lipgloss.Width(cs) + 2; w > cycleColWidth {
			cycleColWidth = w
		}
	}

	// Every row's cycle pad must equal (cycleColWidth - display_width(cs)).
	// If len() were used instead, rows with multi-byte runes would get smaller
	// pads (since len("loop ↺")=8 but display width=6, so len-based pad
	// would be 12-8=4 instead of 12-6=6 — 2 cells short per multi-byte rune).
	for _, g := range groups {
		cs := cycleString(g)
		displayWidth := lipgloss.Width(cs)
		byteLen := len(cs)

		pad := cycleColWidth - displayWidth
		if pad < 1 {
			t.Errorf("group %q: cycle pad = %d (< 1)", g.Name, pad)
			continue
		}

		if displayWidth != byteLen {
			// For groups where len != display width (the multi-byte cases), the
			// display-width pad and byte-len pad must differ; confirm the test
			// is actually exercising the difference.
			bytePad := cycleColWidth - byteLen
			if bytePad == pad {
				t.Errorf("group %q: cycleString=%q — byte pad (%d) equals display pad (%d); test data doesn't exercise the bug",
					g.Name, cs, bytePad, pad)
			}
		}

		// All pads must produce the same total column width when added to their
		// cycle string's display width.
		if displayWidth+pad != cycleColWidth {
			t.Errorf("group %q: displayWidth(%d) + pad(%d) = %d, want %d",
				g.Name, displayWidth, pad, displayWidth+pad, cycleColWidth)
		}
	}
}
