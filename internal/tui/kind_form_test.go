package tui_test

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/jasonwarrenuk/wyrd/internal/tui"
	"github.com/jasonwarrenuk/wyrd/internal/types"
)

// TestKindFormPaneConstructs verifies the constructor returns a non-nil pane
// for a variety of registry states.
func TestKindFormPaneConstructs(t *testing.T) {
	theme, err := tui.LoadTheme(".", "")
	if err != nil {
		t.Fatalf("LoadTheme: %v", err)
	}
	store := newFormTestStore()

	// nil registries are safe (no collision check, no stage-group options).
	fp := tui.NewKindFormPane(theme, store, nil, nil)
	if fp == nil {
		t.Error("expected non-nil PaneModel with nil registries")
	}

	// Non-nil registries with existing entries.
	kinds := types.NewKindRegistry([]types.Kind{
		{Name: "Task", StageGroup: "task-flow", Glyph: "◆", Colour: "#9b70ff"},
	})
	groups := types.NewStageGroupRegistry([]types.StageGroup{
		{Name: "task-flow", Stages: []string{"Open", "Done"}, Cycle: types.CycleTerminate},
	})
	fp2 := tui.NewKindFormPane(theme, store, kinds, groups)
	if fp2 == nil {
		t.Error("expected non-nil PaneModel with existing registries")
	}
}

// TestKindFormPaneViewRenders verifies the form produces a non-empty view.
func TestKindFormPaneViewRenders(t *testing.T) {
	theme, err := tui.LoadTheme(".", "")
	if err != nil {
		t.Fatalf("LoadTheme: %v", err)
	}
	store := newFormTestStore()

	fp := tui.NewKindFormPane(theme, store, nil, nil)
	sized, _ := fp.Update(tea.WindowSizeMsg{Width: 120, Height: 40})
	v := sized.View()
	if v == "" {
		t.Error("expected non-empty view from kindFormPane")
	}
}

// TestKindFormPaneViewContainsFields verifies the rendered view contains the
// expected field titles.
func TestKindFormPaneViewContainsFields(t *testing.T) {
	theme, err := tui.LoadTheme(".", "")
	if err != nil {
		t.Fatalf("LoadTheme: %v", err)
	}
	store := newFormTestStore()

	fp := tui.NewKindFormPane(theme, store, nil, nil)
	sized, _ := fp.Update(tea.WindowSizeMsg{Width: 120, Height: 40})
	v := sized.View()

	for _, want := range []string{"Name", "Glyph", "Colour", "Stage group"} {
		if !strings.Contains(v, want) {
			t.Errorf("view does not contain %q:\n%s", want, v)
		}
	}
}
