package stage_test

import (
	"testing"

	"github.com/jasonwarrenuk/wyrd/internal/stage"
	"github.com/jasonwarrenuk/wyrd/internal/types"
)

// TestRenameKindRewritesMatchingNodes verifies every node holding oldName
// gets rewritten to newName via UpdateNode, and nodes of other kinds are
// left alone.
func TestRenameKindRewritesMatchingNodes(t *testing.T) {
	store := newFakeStore()
	idx := &fakeIndex{nodes: []*types.Node{
		node("n1", "Errand", "Open"),
		node("n2", "Errand", "Done"),
		node("n3", "Task", "Open"), // different kind — must not be touched
	}}

	written, err := stage.RenameKind(store, idx, "Errand", "Chore")
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
		if len(got) != 1 || got["kind"] != "Chore" {
			t.Errorf("update map for %s = %v, want exactly {kind: Chore}", id, got)
		}
	}
	if _, ok := store.updates["n3"]; ok {
		t.Error("n3 has a different kind and should not have been written")
	}
}

// TestRenameKindNoMatchesWritesNothing covers the case where no node holds
// oldName — a rename of a kind with no live nodes yet.
func TestRenameKindNoMatchesWritesNothing(t *testing.T) {
	store := newFakeStore()
	idx := &fakeIndex{nodes: []*types.Node{
		node("n1", "Task", "Open"),
	}}

	written, err := stage.RenameKind(store, idx, "Errand", "Chore")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if written != 0 {
		t.Errorf("written = %d, want 0", written)
	}
	if len(store.updates) != 0 {
		t.Errorf("expected no writes, got %v", store.updates)
	}
}

// TestRenameKindArchivedNodesAreRewritten verifies archived nodes are not
// skipped — archival is a status property, not deletion, and an archived
// node holding a stale kind name is stranded the moment it's unarchived.
func TestRenameKindArchivedNodesAreRewritten(t *testing.T) {
	store := newFakeStore()
	idx := &fakeIndex{nodes: []*types.Node{
		archivedNode("n1", "Errand", "Open"),
	}}

	written, err := stage.RenameKind(store, idx, "Errand", "Chore")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if written != 1 {
		t.Errorf("written = %d, want 1 (archived nodes are still rewritten)", written)
	}
	if got := store.updates["n1"]; got["kind"] != "Chore" {
		t.Errorf("archived node update = %v, want {kind: Chore}", got)
	}
}

// TestRenameKindPartialFailure verifies RenameKind continues past a failed
// write rather than aborting, and reports the correct partial count plus a
// non-nil error — mirroring ApplyRemap's partial-failure contract so the
// existing status-bar handling for partial remap failures applies here too.
func TestRenameKindPartialFailure(t *testing.T) {
	store := newFakeStore()
	store.failIDs["n2"] = true
	idx := &fakeIndex{nodes: []*types.Node{
		node("n1", "Errand", "Open"),
		node("n2", "Errand", "Done"),
		node("n3", "Errand", "Open"),
	}}

	written, err := stage.RenameKind(store, idx, "Errand", "Chore")
	if err == nil {
		t.Fatal("expected an error reporting the partial failure")
	}
	if written != 2 {
		t.Errorf("written = %d, want 2 (n1 and n3 succeed despite n2 failing)", written)
	}
	if _, ok := store.updates["n1"]; !ok {
		t.Error("n1 should have been written")
	}
	if _, ok := store.updates["n3"]; !ok {
		t.Error("n3 should have been written")
	}
	if _, ok := store.updates["n2"]; ok {
		t.Error("n2 should not appear in updates — it failed")
	}
}
