package tui_test

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/jasonwarrenuk/wyrd/internal/tui"
	"github.com/jasonwarrenuk/wyrd/internal/types"
)

// TestStageFormPaneConstructs verifies the constructor returns a non-nil pane
// for a variety of registry states.
func TestStageFormPaneConstructs(t *testing.T) {
	theme, err := tui.LoadTheme(".", "")
	if err != nil {
		t.Fatalf("LoadTheme: %v", err)
	}
	store := newFormTestStore()

	// nil registry is safe (no collision check).
	fp := tui.NewStageFormPane(theme, store, nil)
	if fp == nil {
		t.Error("expected non-nil PaneModel with nil registry")
	}

	// Non-nil registry with existing group.
	reg := types.NewStageGroupRegistry([]types.StageGroup{
		{Name: "task-flow", Stages: []string{"Open", "Done"}, Cycle: types.CycleTerminate},
	})
	fp2 := tui.NewStageFormPane(theme, store, reg)
	if fp2 == nil {
		t.Error("expected non-nil PaneModel with existing registry")
	}
}

// TestStageFormPaneViewRenders verifies the form produces a non-empty view.
func TestStageFormPaneViewRenders(t *testing.T) {
	theme, err := tui.LoadTheme(".", "")
	if err != nil {
		t.Fatalf("LoadTheme: %v", err)
	}
	store := newFormTestStore()

	fp := tui.NewStageFormPane(theme, store, nil)
	sized, _ := fp.Update(tea.WindowSizeMsg{Width: 120, Height: 40})
	v := sized.View()
	if v == "" {
		t.Error("expected non-empty view from stageFormPane")
	}
}

// TestStageFormPaneViewContainsFields verifies the rendered view contains the
// expected field titles from group 1.
func TestStageFormPaneViewContainsFields(t *testing.T) {
	theme, err := tui.LoadTheme(".", "")
	if err != nil {
		t.Fatalf("LoadTheme: %v", err)
	}
	store := newFormTestStore()

	fp := tui.NewStageFormPane(theme, store, nil)
	sized, _ := fp.Update(tea.WindowSizeMsg{Width: 120, Height: 40})
	v := sized.View()

	for _, want := range []string{"Name", "Stages", "Cycle"} {
		if !strings.Contains(v, want) {
			t.Errorf("view does not contain %q:\n%s", want, v)
		}
	}
}
