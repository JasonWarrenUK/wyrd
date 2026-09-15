package stage_test

import (
	"errors"
	"testing"

	"github.com/jasonwarrenuk/wyrd/internal/stage"
	"github.com/jasonwarrenuk/wyrd/internal/types"
)

// TestNodesHoldingKindIncludesArchivedAndUnstaged is the headline divergence
// from DetectOrphans: a node with no stage, and an archived node, are both
// stranded by a kind deletion regardless of stage or archival status, so
// both must be counted here even though DetectOrphans would skip both.
func TestNodesHoldingKindIncludesArchivedAndUnstaged(t *testing.T) {
	idx := &fakeIndex{nodes: []*types.Node{
		node("n1", "Goblin", "Open"),
		node("n2", "Goblin", ""),             // no stage — DetectOrphans skips this
		archivedNode("n3", "Goblin", "Done"), // archived — DetectOrphans skips this
		node("n4", "Task", "Open"),
	}}

	ids := stage.NodesHoldingKind(idx, "Goblin")

	if len(ids) != 3 {
		t.Fatalf("NodesHoldingKind = %v, want 3 nodes (n1, n2, n3)", ids)
	}
}

func TestNodesHoldingKindSkipsEmptyKind(t *testing.T) {
	idx := &fakeIndex{nodes: []*types.Node{
		node("n1", "", ""),
	}}

	ids := stage.NodesHoldingKind(idx, "")
	if len(ids) != 0 {
		t.Errorf("NodesHoldingKind(\"\") = %v, want empty — an untriaged node should never match a deletion target", ids)
	}
}

func TestKindsReferencingGroupIncludesUntouchedDefaults(t *testing.T) {
	kinds, _ := taskFlowKinds()

	names := stage.KindsReferencingGroup(kinds, "task-flow")

	if len(names) != 2 {
		t.Fatalf("KindsReferencingGroup = %v, want 2 (Task, Goblin)", names)
	}
}

func TestKindsReferencingGroupIgnoresOtherGroups(t *testing.T) {
	kinds, _ := taskFlowKinds()

	names := stage.KindsReferencingGroup(kinds, "content-flow")
	if len(names) != 0 {
		t.Errorf("KindsReferencingGroup = %v, want empty", names)
	}
}

func TestDeleteKindRemovesOnlyTheNamedEntry(t *testing.T) {
	store := newFakeStore()
	store.kindsSeed = []types.Kind{
		{Name: "Goblin", StageGroup: "task-flow"},
		{Name: "Errand", StageGroup: "task-flow"},
	}

	if err := stage.DeleteKind(store, "Goblin"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !store.kindsWritten {
		t.Fatal("WriteKinds should have been called")
	}
	if len(store.lastWrittenKinds) != 1 || store.lastWrittenKinds[0].Name != "Errand" {
		t.Errorf("lastWrittenKinds = %v, want only Errand remaining", store.lastWrittenKinds)
	}
}

// TestDeleteKindExactMatchOnly verifies case-folded names are not treated as
// equal — the caller (the TUI dispatch layer) is responsible for resolving
// a typed name to its canonical form via lookupKindFold before calling
// DeleteKind; this function itself must not paper over a mismatch.
func TestDeleteKindExactMatchOnly(t *testing.T) {
	store := newFakeStore()
	store.kindsSeed = []types.Kind{
		{Name: "Task", StageGroup: "task-flow"},
	}

	err := stage.DeleteKind(store, "task")

	var notFound *types.NotFoundError
	if !errors.As(err, &notFound) {
		t.Fatalf("expected *types.NotFoundError, got %v", err)
	}
	if store.kindsWritten {
		t.Error("WriteKinds should not have been called on a miss")
	}
}

func TestDeleteKindMissingReturnsNotFound(t *testing.T) {
	store := newFakeStore()

	err := stage.DeleteKind(store, "Nonexistent")

	var notFound *types.NotFoundError
	if !errors.As(err, &notFound) {
		t.Fatalf("expected *types.NotFoundError, got %v", err)
	}
}

func TestDeleteStageGroupRemovesOnlyTheNamedEntry(t *testing.T) {
	store := newFakeStore()
	store.stagesSeed = []types.StageGroup{
		{Name: "custom-flow", Stages: []string{"A", "B"}, Cycle: types.CycleTerminate},
		{Name: "other-flow", Stages: []string{"X"}, Cycle: types.CycleTerminate},
	}

	if err := stage.DeleteStageGroup(store, "custom-flow"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !store.stagesWritten {
		t.Fatal("WriteStages should have been called")
	}
	if len(store.lastWrittenStages) != 1 || store.lastWrittenStages[0].Name != "other-flow" {
		t.Errorf("lastWrittenStages = %v, want only other-flow remaining", store.lastWrittenStages)
	}
}

func TestDeleteStageGroupMissingReturnsNotFound(t *testing.T) {
	store := newFakeStore()

	err := stage.DeleteStageGroup(store, "Nonexistent")

	var notFound *types.NotFoundError
	if !errors.As(err, &notFound) {
		t.Fatalf("expected *types.NotFoundError, got %v", err)
	}
}

func TestReassignKindMovesEveryNodeIncludingArchived(t *testing.T) {
	store := newFakeStore()
	idx := &fakeIndex{nodes: []*types.Node{
		node("n1", "Goblin", "Open"),
		archivedNode("n2", "Goblin", "Done"),
		node("n3", "Task", "Open"), // different kind — must not be touched
	}}

	written, err := stage.ReassignKind(store, idx, "Goblin", "Task")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if written != 2 {
		t.Errorf("written = %d, want 2", written)
	}
	for _, id := range []string{"n1", "n2"} {
		got, ok := store.updates[id]
		if !ok {
			t.Fatalf("expected UpdateNode call for %s", id)
		}
		if got["kind"] != "Task" {
			t.Errorf("update for %s = %v, want {kind: Task}", id, got)
		}
	}
	if _, ok := store.updates["n3"]; ok {
		t.Error("n3 has a different kind and should not have been written")
	}
}

// TestReassignKindPartialFailure mirrors TestRenameKindPartialFailure: a
// per-node write failure must not abort the remaining writes, and the
// returned count reflects only the successful ones.
func TestReassignKindPartialFailure(t *testing.T) {
	store := newFakeStore()
	store.failIDs["n2"] = true
	idx := &fakeIndex{nodes: []*types.Node{
		node("n1", "Goblin", "Open"),
		node("n2", "Goblin", "Done"),
		node("n3", "Goblin", "Open"),
	}}

	written, err := stage.ReassignKind(store, idx, "Goblin", "Task")
	if err == nil {
		t.Fatal("expected an error reporting the partial failure")
	}
	if written != 2 {
		t.Errorf("written = %d, want 2 (n1 and n3 succeed despite n2 failing)", written)
	}
	if _, ok := store.updates["n2"]; ok {
		t.Error("n2 should not appear in updates — it failed")
	}
}

func TestReassignKindIgnoresOtherKinds(t *testing.T) {
	store := newFakeStore()
	idx := &fakeIndex{nodes: []*types.Node{
		node("n1", "Task", "Open"),
	}}

	written, err := stage.ReassignKind(store, idx, "Goblin", "Task")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if written != 0 {
		t.Errorf("written = %d, want 0 — no node holds Goblin", written)
	}
}
