package tui

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
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

	so := newStagesOverlay(theme, reg)
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

	reg := stage.MergeStageGroups(defaults, []types.StageGroup{customGroup})
	theme := loadStagesTestTheme(t)

	so := newStagesOverlay(theme, reg)
	so.Open(160, 50)

	view := so.View(160, 50)

	if !strings.Contains(view, "(custom)") {
		t.Error("expected (custom) marker for user-defined group")
	}
	if !strings.Contains(view, "task-flow") {
		t.Error("expected baked-in task-flow group to appear")
	}
	if !strings.Contains(view, "my-custom-flow") {
		t.Error("expected user group my-custom-flow to appear")
	}
}

func TestStagesOverlay_EmptyState(t *testing.T) {
	theme := loadStagesTestTheme(t)
	so := newStagesOverlay(theme, nil)
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

	so := newStagesOverlay(theme, reg)
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
	so := newStagesOverlay(theme, nil)

	view := so.View(120, 40)
	if view != "" {
		t.Errorf("inactive overlay should return empty string, got %q", view)
	}
}
