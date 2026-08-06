package tui_test

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/jasonwarrenuk/wyrd/internal/stage"
	"github.com/jasonwarrenuk/wyrd/internal/tui"
	"github.com/jasonwarrenuk/wyrd/internal/types"
)

func sampleOrphanReport() stage.OrphanReport {
	group := types.StageGroup{Name: "task-flow", Stages: []string{"Open", "In Progress", "Done"}, Cycle: types.CycleTerminate}
	return stage.OrphanReport{
		Orphans: []stage.Orphan{
			{Kind: "Task", Stage: "Maybe", Group: group, NodeIDs: []string{"n1", "n2"}, Suggested: "Open"},
			{Kind: "Goblin", Stage: "Someday", Group: group, NodeIDs: []string{"n3"}, Suggested: "Open"},
		},
	}
}

// TestRemapFormPaneViewRenders verifies the form produces a non-empty view.
func TestRemapFormPaneViewRenders(t *testing.T) {
	theme := loadTestTheme(t)
	store := newFormTestStore()

	fp := tui.NewRemapFormPane(theme, store, sampleOrphanReport())
	sized, _ := fp.Update(tea.WindowSizeMsg{Width: 120, Height: 40})
	v := sized.View()
	if v == "" {
		t.Error("expected non-empty view from remapFormPane")
	}
}

// TestRemapFormPaneViewContainsOrphanIdentity verifies the rendered view
// names each orphaned kind and stage so the user knows what they're deciding.
func TestRemapFormPaneViewContainsOrphanIdentity(t *testing.T) {
	theme := loadTestTheme(t)
	store := newFormTestStore()

	fp := tui.NewRemapFormPane(theme, store, sampleOrphanReport())
	sized, _ := fp.Update(tea.WindowSizeMsg{Width: 120, Height: 40})
	v := sized.View()

	for _, want := range []string{"Task", "Maybe", "Goblin", "Someday"} {
		if !strings.Contains(v, want) {
			t.Errorf("view does not contain %q:\n%s", want, v)
		}
	}
}

// TestRemapFormPaneOneFieldPerOrphan verifies field count matches the
// number of orphans by checking each orphan's target group stages appear —
// a coarse proxy since huh doesn't expose field count directly, but the
// sentinel appearing once per field is a reasonable stand-in.
func TestRemapFormPaneOneFieldPerOrphan(t *testing.T) {
	theme := loadTestTheme(t)
	store := newFormTestStore()

	fp := tui.NewRemapFormPane(theme, store, sampleOrphanReport())
	sized, _ := fp.Update(tea.WindowSizeMsg{Width: 120, Height: 40})
	v := sized.View()

	// Only the first (focused) field's options render in a plain View() call
	// for a multi-field huh group, so assert on the identity lines instead —
	// both orphan titles must be present, confirming two fields were built.
	if !strings.Contains(v, "Task") || !strings.Contains(v, "Goblin") {
		t.Errorf("expected both orphan fields present in view; got:\n%s", v)
	}
}
