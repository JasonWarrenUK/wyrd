package tui

import (
	"strings"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/jasonwarrenuk/wyrd/internal/query"
	"github.com/jasonwarrenuk/wyrd/internal/stage"
	"github.com/jasonwarrenuk/wyrd/internal/store"
	"github.com/jasonwarrenuk/wyrd/internal/types"
)

// ---------------------------------------------------------------------------
// ":stages remap" dispatch tests. These live in the internal package because
// PaletteState.commands and Model.palette are unexported — the same reason
// palette_view_test.go operates on PaletteState directly rather than driving
// keystrokes through the CLI input. Testing at this level exercises the real
// registered Execute closure (the args[0] == "remap" branch added for SL.14)
// together with the real openRemapFormMsg handling in Update, without
// choreographing a full ":"-mode keystroke sequence that no existing palette
// command test does either.
// ---------------------------------------------------------------------------

// truncateForTest returns the first n bytes of s for use in test error
// messages. A local copy of stage_shift_test.go's truncate: that one lives
// in package tui_test and isn't visible from this internal-package file.
func truncateForTest(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n]
}

// stagesCommand locates the registered "stages" command by name.
func stagesCommand(t *testing.T, m Model) Command {
	t.Helper()
	for _, c := range m.palette.commands {
		if c.Name == "stages" {
			return c
		}
	}
	t.Fatal("no \"stages\" command registered")
	return Command{}
}

// newRemapTestModel builds a real store + registries, seeding zero or more
// nodes whose kind/stage the caller controls, and returns the constructed
// Model sized to a usable terminal.
func newRemapTestModel(t *testing.T, seed func(s *store.Store)) Model {
	t.Helper()
	dir := t.TempDir()
	clock := types.StubClock{Fixed: time.Date(2026, 3, 23, 9, 0, 0, 0, time.UTC)}
	s, err := store.New(dir, clock)
	if err != nil {
		t.Fatalf("store.New: %v", err)
	}
	t.Cleanup(func() {
		if err := s.Close(); err != nil {
			t.Logf("store.Close: %v", err)
		}
	})

	if seed != nil {
		seed(s)
	}

	kindDefaults, err := stage.DefaultKinds()
	if err != nil {
		t.Fatalf("DefaultKinds: %v", err)
	}
	kinds := stage.MergeKinds(kindDefaults, nil)

	groupDefaults, err := stage.DefaultStageGroups()
	if err != nil {
		t.Fatalf("DefaultStageGroups: %v", err)
	}
	stageGroups := stage.MergeStageGroups(groupDefaults, nil)

	engine := query.NewEngine(s.Index(), 10)
	m, err := New(Config{
		Store:       s,
		StorePath:   dir,
		Index:       s.Index(),
		QueryRunner: engine,
		Clock:       clock,
		Kinds:       kinds,
		StageGroups: stageGroups,
	})
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	sized, _ := m.Update(tea.WindowSizeMsg{Width: 120, Height: 40})
	return sized.(Model)
}

// runCommand executes cmd.Execute(args) and, if it returns a tea.Cmd, runs
// it and feeds the resulting message through m.Update.
func runCommand(t *testing.T, m Model, cmd Command, args []string) Model {
	t.Helper()
	teaCmd := cmd.Execute(args)
	if teaCmd == nil {
		t.Fatal("Execute returned nil tea.Cmd")
	}
	msg := teaCmd()
	updated, _ := m.Update(msg)
	result, ok := updated.(Model)
	if !ok {
		t.Fatalf("Update returned unexpected type %T", updated)
	}
	return result
}

// TestStagesRemapNoOrphansShowsStatusMessage verifies that running the
// remap command against a healthy graph reports "No orphaned stages" and
// does not mount a form.
func TestStagesRemapNoOrphansShowsStatusMessage(t *testing.T) {
	m := newRemapTestModel(t, func(s *store.Store) {
		node, err := s.CreateNode("Healthy task", []string{"task"})
		if err != nil {
			t.Fatalf("CreateNode: %v", err)
		}
		if _, err := s.UpdateNode(node.ID, map[string]interface{}{"kind": "Task", "stage": "Open"}); err != nil {
			t.Fatalf("UpdateNode: %v", err)
		}
	})

	cmd := stagesCommand(t, m)
	m = runCommand(t, m, cmd, []string{"remap"})

	if _, isForm := m.rightPane.(formActivePane); isForm {
		t.Error("expected no form mounted when there are no orphans")
	}
	view := m.View().Content
	if !strings.Contains(view, "No orphaned stages") {
		t.Errorf("expected 'No orphaned stages' in view; got: %q", truncateForTest(view, 300))
	}
}

// TestStagesRemapMountsFormWhenOrphansExist verifies that an orphaned node
// (stage hand-edited to a value absent from its kind's group) causes
// ":stages remap" to mount a remap form in the right pane.
func TestStagesRemapMountsFormWhenOrphansExist(t *testing.T) {
	m := newRemapTestModel(t, func(s *store.Store) {
		node, err := s.CreateNode("Orphaned task", []string{"task"})
		if err != nil {
			t.Fatalf("CreateNode: %v", err)
		}
		// "Whenever" is not a stage in the baked-in task-flow group.
		if _, err := s.UpdateNode(node.ID, map[string]interface{}{"kind": "Task", "stage": "Whenever"}); err != nil {
			t.Fatalf("UpdateNode: %v", err)
		}
	})

	cmd := stagesCommand(t, m)
	m = runCommand(t, m, cmd, []string{"remap"})

	if _, isForm := m.rightPane.(formActivePane); !isForm {
		t.Errorf("expected a form mounted in the right pane, got %T", m.rightPane)
	}
	if _, ok := m.rightPane.(remapFormPane); !ok {
		t.Errorf("expected rightPane to be a remapFormPane, got %T", m.rightPane)
	}
}

// TestStageFormSubmitOpensRemapWhenOrphansAppear verifies that creating a
// new stage group whose write orphans existing nodes actively opens the
// :stages remap form, rather than only appending a passive hint — SL.17's
// active hand-off (see kindFormSubmitMsg/stageFormSubmitMsg's handlers).
// Renamed from ...Advises...: the passive orphanAdvisory() text is now only
// the fallback above maxRemapOrphans; the primary path opens the form.
func TestStageFormSubmitOpensRemapWhenOrphansAppear(t *testing.T) {
	m := newRemapTestModel(t, func(s *store.Store) {
		node, err := s.CreateNode("Orphaned by group edit", []string{"task"})
		if err != nil {
			t.Fatalf("CreateNode: %v", err)
		}
		if _, err := s.UpdateNode(node.ID, map[string]interface{}{"kind": "Task", "stage": "Whenever"}); err != nil {
			t.Fatalf("UpdateNode: %v", err)
		}
	})

	// Simulate the outcome of a stageFormSubmitMsg without driving the full
	// creation form — the hand-off only depends on the message arriving
	// after a write, not on how the write happened. driveSubmitMsg (in
	// kind_edit_orphan_handoff_internal_test.go) executes the returned
	// tea.Cmd and delivers its message, since the hand-off returns the
	// mount as a chained command rather than synchronously.
	m2 := driveSubmitMsg(t, m, stageFormSubmitMsg{name: "some-group"})

	if _, ok := m2.rightPane.(remapFormPane); !ok {
		t.Errorf("expected rightPane to be a remapFormPane, got %T", m2.rightPane)
	}
}
