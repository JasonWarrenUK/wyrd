package tui

import (
	"testing"

	tea "charm.land/bubbletea/v2"
	huh "charm.land/huh/v2"
	"github.com/jasonwarrenuk/wyrd/internal/stage"
	"github.com/jasonwarrenuk/wyrd/internal/types"
)

// ---------------------------------------------------------------------------
// remapFormPane tests. Live in the internal package to manipulate
// huh.Form.State directly and to inspect the pane's unexported choices,
// mirroring the stageFormPane internal test conventions.
// ---------------------------------------------------------------------------

func testOrphanReport() stage.OrphanReport {
	group := types.StageGroup{Name: "task-flow", Stages: []string{"Open", "In Progress", "Done"}, Cycle: types.CycleTerminate}
	return stage.OrphanReport{
		Orphans: []stage.Orphan{
			{Kind: "Task", Stage: "Maybe", Group: group, NodeIDs: []string{"n1", "n2"}, Suggested: "Open"},
			{Kind: "Goblin", Stage: "Someday", Group: group, NodeIDs: []string{"n3"}, Suggested: "In Progress"},
		},
	}
}

// driveRemapToCompleted forces f.form.State to StateCompleted and drives an
// Update so the submit branch runs, mirroring driveToCompleted for
// stageFormPane.
func driveRemapToCompleted(f remapFormPane) (remapFormPane, tea.Cmd) {
	f.form.State = huh.StateCompleted
	updated, cmd := f.Update(tea.KeyPressMsg{})
	return updated.(remapFormPane), cmd
}

func TestRemapFormDefaultsToSuggested(t *testing.T) {
	theme, err := LoadTheme(".", "")
	if err != nil {
		t.Fatalf("LoadTheme: %v", err)
	}
	store := &errStoreFS{}

	f := newRemapFormPane(theme, store, testOrphanReport())

	if f.choices[0] != "Open" {
		t.Errorf("choices[0] = %q, want suggested %q", f.choices[0], "Open")
	}
	if f.choices[1] != "In Progress" {
		t.Errorf("choices[1] = %q, want suggested %q", f.choices[1], "In Progress")
	}
}

func TestRemapFormSubmitAppliesChoicesViaUpdateNode(t *testing.T) {
	theme, err := LoadTheme(".", "")
	if err != nil {
		t.Fatalf("LoadTheme: %v", err)
	}
	store := &errStoreFS{}

	f := newRemapFormPane(theme, store, testOrphanReport())
	// Simulate the user picking targets different from the defaults.
	f.choices[0] = "Done"
	f.choices[1] = "Done"

	_, cmd := driveRemapToCompleted(f)
	msg := collectMsg(cmd)

	submit, ok := unwrapRemapMsg[remapFormSubmitMsg](msg)
	if !ok {
		t.Fatalf("expected remapFormSubmitMsg, got %#v", msg)
	}
	if submit.remapped != 3 {
		t.Errorf("remapped = %d, want 3 (2 Task nodes + 1 Goblin node)", submit.remapped)
	}
	if submit.unchanged != 0 {
		t.Errorf("unchanged = %d, want 0", submit.unchanged)
	}
	if len(store.updateCalls) != 3 {
		t.Errorf("expected 3 UpdateNode calls, got %d: %v", len(store.updateCalls), store.updateCalls)
	}
}

func TestRemapFormSubmitSentinelLeavesNodeUnchanged(t *testing.T) {
	theme, err := LoadTheme(".", "")
	if err != nil {
		t.Fatalf("LoadTheme: %v", err)
	}
	store := &errStoreFS{}

	f := newRemapFormPane(theme, store, testOrphanReport())
	f.choices[0] = "" // leave the Task/Maybe orphan unchanged
	f.choices[1] = "Done"

	_, cmd := driveRemapToCompleted(f)
	msg := collectMsg(cmd)

	submit, ok := unwrapRemapMsg[remapFormSubmitMsg](msg)
	if !ok {
		t.Fatalf("expected remapFormSubmitMsg, got %#v", msg)
	}
	if submit.remapped != 1 {
		t.Errorf("remapped = %d, want 1 (only the Goblin node)", submit.remapped)
	}
	if submit.unchanged != 2 {
		t.Errorf("unchanged = %d, want 2 (both Task/Maybe nodes)", submit.unchanged)
	}
	for _, id := range []string{"n1", "n2"} {
		for _, called := range store.updateCalls {
			if called == id {
				t.Errorf("node %s should not have been written (sentinel chosen)", id)
			}
		}
	}
}

func TestRemapFormSubmitWriteFailureEmitsErrorMsg(t *testing.T) {
	theme, err := LoadTheme(".", "")
	if err != nil {
		t.Fatalf("LoadTheme: %v", err)
	}
	store := &errStoreFS{updateErrIDs: map[string]bool{"n2": true}}

	f := newRemapFormPane(theme, store, testOrphanReport())

	_, cmd := driveRemapToCompleted(f)
	msg := collectMsg(cmd)

	if _, ok := unwrapRemapMsg[remapFormSubmitMsg](msg); ok {
		t.Error("remapFormSubmitMsg should not be emitted when a write fails")
	}
	errMsg, ok := unwrapRemapMsg[remapFormErrorMsg](msg)
	if !ok {
		t.Fatalf("expected remapFormErrorMsg, got %#v", msg)
	}
	if errMsg.err == nil {
		t.Error("remapFormErrorMsg.err should be non-nil")
	}
}

func TestRemapFormCancelEmitsFormCancelMsg(t *testing.T) {
	theme, err := LoadTheme(".", "")
	if err != nil {
		t.Fatalf("LoadTheme: %v", err)
	}
	store := &errStoreFS{}

	f := newRemapFormPane(theme, store, testOrphanReport())
	f.form.State = huh.StateAborted
	_, cmd := f.Update(tea.KeyPressMsg{})
	msg := collectMsg(cmd)

	if _, ok := unwrapRemapMsg[formCancelMsg](msg); !ok {
		t.Fatalf("expected formCancelMsg on abort, got %#v", msg)
	}
}

// unwrapRemapMsg unwraps a tea.BatchMsg (or accepts a bare message) looking
// for one of type T, mirroring the batch-unwrapping convention established
// in TestStageFormErrorOnReadFailure.
func unwrapRemapMsg[T any](msg tea.Msg) (T, bool) {
	var zero T
	if batch, ok := msg.(tea.BatchMsg); ok {
		for _, fn := range batch {
			if m := fn(); m != nil {
				if t, ok := m.(T); ok {
					return t, true
				}
			}
		}
		return zero, false
	}
	t, ok := msg.(T)
	return t, ok
}
