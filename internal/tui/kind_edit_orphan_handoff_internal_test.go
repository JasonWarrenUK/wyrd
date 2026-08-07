package tui

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/jasonwarrenuk/wyrd/internal/store"
)

// ---------------------------------------------------------------------------
// Orphan hand-off tests for kindFormSubmitMsg. Live in the internal package
// for the same reason remap_command_internal_test.go does. Reuses
// newRemapTestModel from that file.
// ---------------------------------------------------------------------------

// driveSubmitMsg runs msg through m.Update, then executes the returned
// tea.Cmd and feeds every resulting message back through Update in turn,
// folding the model forward. This is needed wherever a handler chains a
// tea.Batch into a follow-up message (as kindFormSubmitMsg's orphan
// hand-off does with openRemapFormMsg) — a single m.Update call only
// produces the *command*, not the mounted form; the command still has to be
// executed and its message delivered, exactly as the real Bubble Tea
// runtime does on the next event-loop tick.
func driveSubmitMsg(t *testing.T, m Model, msg tea.Msg) Model {
	t.Helper()
	updated, cmd := m.Update(msg)
	m2, ok := updated.(Model)
	if !ok {
		t.Fatalf("Update returned unexpected type %T", updated)
	}
	if cmd == nil {
		return m2
	}
	result := cmd()
	batch, ok := result.(tea.BatchMsg)
	if !ok {
		updated, _ := m2.Update(result)
		out, ok := updated.(Model)
		if !ok {
			t.Fatalf("Update returned unexpected type %T", updated)
		}
		return out
	}
	for _, fn := range batch {
		if fn == nil {
			continue
		}
		m2 = driveSubmitMsg(t, m2, fn())
	}
	return m2
}

// TestKindEditOrphanHandoffOpensRemapForm verifies that a kind edit which
// orphans existing nodes (here: simulated by driving kindFormSubmitMsg after
// hand-editing a node to hold a stage absent from the kind's group — the
// same simulation TestStageFormSubmitAdvisesWhenOrphansAppear uses) mounts
// the remap form automatically rather than only appending a passive advisory.
func TestKindEditOrphanHandoffOpensRemapForm(t *testing.T) {
	m := newRemapTestModel(t, func(s *store.Store) {
		node, err := s.CreateNode("Orphaned by kind edit", []string{"task"})
		if err != nil {
			t.Fatalf("CreateNode: %v", err)
		}
		if _, err := s.UpdateNode(node.ID, map[string]interface{}{"kind": "Task", "stage": "Whenever"}); err != nil {
			t.Fatalf("UpdateNode: %v", err)
		}
	})

	m2 := driveSubmitMsg(t, m, kindFormSubmitMsg{name: "Task"})

	if _, ok := m2.rightPane.(remapFormPane); !ok {
		t.Errorf("expected rightPane to be a remapFormPane, got %T", m2.rightPane)
	}
}

// TestKindEditNoOrphansStaysOnDashboard verifies a kind edit that orphans
// nothing does not mount the remap form.
func TestKindEditNoOrphansStaysOnDashboard(t *testing.T) {
	m := newRemapTestModel(t, func(s *store.Store) {
		node, err := s.CreateNode("Healthy task", []string{"task"})
		if err != nil {
			t.Fatalf("CreateNode: %v", err)
		}
		if _, err := s.UpdateNode(node.ID, map[string]interface{}{"kind": "Task", "stage": "Open"}); err != nil {
			t.Fatalf("UpdateNode: %v", err)
		}
	})

	updated, _ := m.Update(kindFormSubmitMsg{name: "Task"})
	m2, ok := updated.(Model)
	if !ok {
		t.Fatalf("Update returned unexpected type %T", updated)
	}

	if _, isForm := m2.rightPane.(formActivePane); isForm {
		t.Errorf("expected no form mounted when nothing was orphaned, got %T", m2.rightPane)
	}
}

// TestKindEditRenameMovesNodes verifies that a kindFormSubmitMsg carrying
// renamedFrom actually moves nodes via the RenameKind cascade — driven at
// the Model.Update level so the wiring between the message and
// stage.RenameKind is exercised, not just RenameKind in isolation (already
// covered in internal/stage/rename_test.go).
func TestKindEditRenameMovesNodes(t *testing.T) {
	var nodeID string
	m := newRemapTestModel(t, func(s *store.Store) {
		node, err := s.CreateNode("To be renamed", []string{"task"})
		if err != nil {
			t.Fatalf("CreateNode: %v", err)
		}
		nodeID = node.ID
		if _, err := s.UpdateNode(node.ID, map[string]interface{}{"kind": "Task", "stage": "Open"}); err != nil {
			t.Fatalf("UpdateNode: %v", err)
		}
	})

	updated, _ := m.Update(kindFormSubmitMsg{name: "Errand", renamedFrom: "Task"})
	m2, ok := updated.(Model)
	if !ok {
		t.Fatalf("Update returned unexpected type %T", updated)
	}

	got, err := m2.index.GetNode(nodeID)
	if err != nil {
		t.Fatalf("GetNode: %v", err)
	}
	if got.Kind != "Errand" {
		t.Errorf("node.Kind = %q after rename, want %q", got.Kind, "Errand")
	}

	view := m2.View().Content
	if !strings.Contains(view, `Renamed "Task" to "Errand"`) {
		t.Errorf("expected rename confirmation text in view; got: %q", truncateForTest(view, 300))
	}
}

// TestKindEditNoRenameLeavesNodesAlone is the control for
// TestKindEditRenameMovesNodes: renamedFrom empty must not touch any node.
func TestKindEditNoRenameLeavesNodesAlone(t *testing.T) {
	var nodeID string
	m := newRemapTestModel(t, func(s *store.Store) {
		node, err := s.CreateNode("Untouched", []string{"task"})
		if err != nil {
			t.Fatalf("CreateNode: %v", err)
		}
		nodeID = node.ID
		if _, err := s.UpdateNode(node.ID, map[string]interface{}{"kind": "Task", "stage": "Open"}); err != nil {
			t.Fatalf("UpdateNode: %v", err)
		}
	})

	updated, _ := m.Update(kindFormSubmitMsg{name: "Task"})
	m2, ok := updated.(Model)
	if !ok {
		t.Fatalf("Update returned unexpected type %T", updated)
	}

	got, err := m2.index.GetNode(nodeID)
	if err != nil {
		t.Fatalf("GetNode: %v", err)
	}
	if got.Kind != "Task" {
		t.Errorf("node.Kind = %q, want unchanged %q", got.Kind, "Task")
	}
}

// TestKindEditRenameFanOutAcrossMultipleNodes verifies RenameKind moves every
// node holding the old kind, not just one — the same fan-out concern SL.17's
// stage-group rename faces, checked here at the single-kind level.
func TestKindEditRenameFanOutAcrossMultipleNodes(t *testing.T) {
	var ids []string
	m := newRemapTestModel(t, func(s *store.Store) {
		for _, title := range []string{"one", "two", "three"} {
			node, err := s.CreateNode(title, []string{"task"})
			if err != nil {
				t.Fatalf("CreateNode: %v", err)
			}
			if _, err := s.UpdateNode(node.ID, map[string]interface{}{"kind": "Task", "stage": "Open"}); err != nil {
				t.Fatalf("UpdateNode: %v", err)
			}
			ids = append(ids, node.ID)
		}
	})

	updated, _ := m.Update(kindFormSubmitMsg{name: "Errand", renamedFrom: "Task"})
	m2, ok := updated.(Model)
	if !ok {
		t.Fatalf("Update returned unexpected type %T", updated)
	}

	for _, id := range ids {
		got, err := m2.index.GetNode(id)
		if err != nil {
			t.Fatalf("GetNode(%s): %v", id, err)
		}
		if got.Kind != "Errand" {
			t.Errorf("node %s Kind = %q, want %q", id, got.Kind, "Errand")
		}
	}
}
