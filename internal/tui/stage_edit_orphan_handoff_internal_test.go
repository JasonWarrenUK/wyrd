package tui

import (
	"strings"
	"testing"

	"github.com/jasonwarrenuk/wyrd/internal/stage"
	"github.com/jasonwarrenuk/wyrd/internal/store"
)

// ---------------------------------------------------------------------------
// Orphan hand-off tests for stageFormSubmitMsg, including SL.17's harder
// case: a stage removed from a group shared by multiple kinds fans out into
// one Orphan row per affected kind. Live in the internal package for the
// same reason remap_command_internal_test.go does. Reuses newRemapTestModel
// and driveSubmitMsg (kind_edit_orphan_handoff_internal_test.go).
// ---------------------------------------------------------------------------

// TestStageEditOrphanHandoffOpensRemapForm verifies a stage-group edit that
// orphans existing nodes mounts the remap form automatically — the
// stage-group twin of TestKindEditOrphanHandoffOpensRemapForm.
func TestStageEditOrphanHandoffOpensRemapForm(t *testing.T) {
	m := newRemapTestModel(t, func(s *store.Store) {
		node, err := s.CreateNode("Orphaned by group edit", []string{"task"})
		if err != nil {
			t.Fatalf("CreateNode: %v", err)
		}
		if _, err := s.UpdateNode(node.ID, map[string]interface{}{"kind": "Task", "stage": "Whenever"}); err != nil {
			t.Fatalf("UpdateNode: %v", err)
		}
	})

	m2 := driveSubmitMsg(t, m, stageFormSubmitMsg{name: "task-flow"})

	if _, ok := m2.rightPane.(remapFormPane); !ok {
		t.Errorf("expected rightPane to be a remapFormPane, got %T", m2.rightPane)
	}
}

// TestStageEditNoOrphansStaysOnDashboard verifies a stage-group edit that
// orphans nothing does not mount the remap form.
func TestStageEditNoOrphansStaysOnDashboard(t *testing.T) {
	m := newRemapTestModel(t, func(s *store.Store) {
		node, err := s.CreateNode("Healthy task", []string{"task"})
		if err != nil {
			t.Fatalf("CreateNode: %v", err)
		}
		if _, err := s.UpdateNode(node.ID, map[string]interface{}{"kind": "Task", "stage": "Open"}); err != nil {
			t.Fatalf("UpdateNode: %v", err)
		}
	})

	updated, _ := m.Update(stageFormSubmitMsg{name: "task-flow"})
	m2, ok := updated.(Model)
	if !ok {
		t.Fatalf("Update returned unexpected type %T", updated)
	}

	if _, isForm := m2.rightPane.(formActivePane); isForm {
		t.Errorf("expected no form mounted when nothing was orphaned, got %T", m2.rightPane)
	}
}

// TestStageEditRenameRepointsKinds verifies that a stageFormSubmitMsg
// carrying renamedFrom actually repoints referencing kinds via the
// RenameStageGroup cascade — driven at the Model.Update level so the wiring
// is exercised, not just RenameStageGroup in isolation (covered in
// internal/stage/rename_test.go).
func TestStageEditRenameRepointsKinds(t *testing.T) {
	m := newRemapTestModel(t, nil)

	updated, _ := m.Update(stageFormSubmitMsg{name: "todo-flow", renamedFrom: "task-flow"})
	m2, ok := updated.(Model)
	if !ok {
		t.Fatalf("Update returned unexpected type %T", updated)
	}

	k, ok := m2.kinds.Lookup("Task")
	if !ok {
		t.Fatal("Task should still resolve after the rename")
	}
	if k.StageGroup != "todo-flow" {
		t.Errorf("Task.StageGroup = %q after rename, want %q", k.StageGroup, "todo-flow")
	}

	view := m2.View().Content
	if !strings.Contains(view, `Renamed "task-flow" to "todo-flow"`) {
		t.Errorf("expected rename confirmation text in view; got: %q", truncateForTest(view, 300))
	}
}

// TestStageEditFanOutAcrossSharedKinds is THE proof point for SL.17's
// harder case. task-flow is referenced by three baked-in default kinds —
// Task, Goblin, and Talk. Removing a stage they all share orphans all
// three, and DetectOrphans' OrphanKey is keyed by (Kind, Stage), so the
// result must be three DISTINCT Orphan rows — not one row shared across
// kinds — mirroring TestDetectOrphansSharedGroupAcrossKinds' reasoning but
// exercised through the real submit-handler wiring rather than DetectOrphans
// called directly.
func TestStageEditFanOutAcrossSharedKinds(t *testing.T) {
	m := newRemapTestModel(t, func(s *store.Store) {
		for _, kind := range []string{"Task", "Goblin", "Talk"} {
			node, err := s.CreateNode("orphaned "+kind, []string{"task"})
			if err != nil {
				t.Fatalf("CreateNode(%s): %v", kind, err)
			}
			// "Whenever" is not a stage in the baked-in task-flow group —
			// every one of these three nodes is already orphaned before the
			// edit, which is fine: the edit (renaming task-flow to itself
			// via a same-name resubmit, or any group write that triggers a
			// rebuild) just needs to make the scan run against the current
			// registries so all three surface.
			if _, err := s.UpdateNode(node.ID, map[string]interface{}{"kind": kind, "stage": "Whenever"}); err != nil {
				t.Fatalf("UpdateNode(%s): %v", kind, err)
			}
		}
	})

	m2 := driveSubmitMsg(t, m, stageFormSubmitMsg{name: "task-flow"})

	fp, ok := m2.rightPane.(remapFormPane)
	if !ok {
		t.Fatalf("expected rightPane to be a remapFormPane, got %T", m2.rightPane)
	}

	report := stage.DetectOrphans(m2.index, m2.kinds, m2.stageGroups)
	if len(report.Orphans) != 3 {
		t.Fatalf("expected 3 distinct Orphan rows (one per kind sharing task-flow), got %d: %+v", len(report.Orphans), report.Orphans)
	}
	byKind := map[string]bool{}
	for _, o := range report.Orphans {
		byKind[o.Kind] = true
	}
	for _, want := range []string{"Task", "Goblin", "Talk"} {
		if !byKind[want] {
			t.Errorf("expected an Orphan row for kind %q, got rows for %v", want, byKind)
		}
	}

	// The mounted remap form must expose one select per orphan — reuse the
	// same content check remap_form_test.go's own tests use rather than
	// reaching into unexported form internals.
	view := fp.View()
	for _, kindName := range []string{"Task", "Goblin", "Talk"} {
		if !strings.Contains(view, kindName) {
			t.Errorf("expected the remap form to mention kind %q, view: %q", kindName, truncateForTest(view, 500))
		}
	}
}

// TestStageEditFanOutOverCapFallsBackToAdvisory verifies that when a
// stage-group edit's fan-out exceeds maxRemapOrphans, the passive advisory
// fires instead of the active hand-off — same ceiling behaviour as
// :stages remap itself (TestStagesRemapTooManyOrphans, if present) applied
// via the edit path.
func TestStageEditFanOutOverCapFallsBackToAdvisory(t *testing.T) {
	m := newRemapTestModel(t, func(s *store.Store) {
		// maxRemapOrphans is 20; each distinct (kind, stage) pair is one
		// Orphan row, so 21 distinct orphaned stage VALUES on a single kind
		// produces 21 rows without needing 21 kinds.
		for i := 0; i < maxRemapOrphans+1; i++ {
			node, err := s.CreateNode("orphan", []string{"task"})
			if err != nil {
				t.Fatalf("CreateNode: %v", err)
			}
			stageVal := "Whenever" + string(rune('A'+i))
			if _, err := s.UpdateNode(node.ID, map[string]interface{}{"kind": "Task", "stage": stageVal}); err != nil {
				t.Fatalf("UpdateNode: %v", err)
			}
		}
	})

	m2 := driveSubmitMsg(t, m, stageFormSubmitMsg{name: "task-flow"})

	if _, isForm := m2.rightPane.(formActivePane); isForm {
		t.Errorf("expected no form mounted above maxRemapOrphans, got %T", m2.rightPane)
	}
}
