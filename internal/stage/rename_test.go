package stage_test

import (
	"errors"
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

// ---------------------------------------------------------------------------
// RenameStageGroup — the harder fan-out case: a group rename touches every
// KIND referencing it (not nodes directly), including built-in kinds that
// need a fresh shadow copy since the embedded default can't be edited.
// ---------------------------------------------------------------------------

// TestRenameStageGroupRewritesUserKind verifies a user-defined kind
// referencing the renamed group gets rewritten in place.
func TestRenameStageGroupRewritesUserKind(t *testing.T) {
	store := newFakeStore()
	store.kindsSeed = []types.Kind{
		{Name: "Errand", StageGroup: "old-flow", Glyph: "!"},
	}

	rewritten, err := stage.RenameStageGroup(store, "old-flow", "new-flow")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if rewritten != 1 {
		t.Errorf("rewritten = %d, want 1", rewritten)
	}
	if !store.kindsWritten {
		t.Fatal("WriteKinds should have been called")
	}
	if len(store.lastWrittenKinds) != 1 || store.lastWrittenKinds[0].StageGroup != "new-flow" {
		t.Errorf("lastWrittenKinds = %v, want single kind with StageGroup new-flow", store.lastWrittenKinds)
	}
	if store.lastWrittenKinds[0].Name != "Errand" || store.lastWrittenKinds[0].Glyph != "!" {
		t.Errorf("lastWrittenKinds[0] = %v, other fields should be untouched", store.lastWrittenKinds[0])
	}
}

// TestRenameStageGroupIgnoresUnrelatedUserKind verifies a user kind
// referencing a DIFFERENT group is left alone.
func TestRenameStageGroupIgnoresUnrelatedUserKind(t *testing.T) {
	store := newFakeStore()
	store.kindsSeed = []types.Kind{
		{Name: "Note", StageGroup: "content-flow"},
	}

	rewritten, err := stage.RenameStageGroup(store, "old-flow", "new-flow")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if rewritten != 0 {
		t.Errorf("rewritten = %d, want 0", rewritten)
	}
	if len(store.lastWrittenKinds) != 1 || store.lastWrittenKinds[0].StageGroup != "content-flow" {
		t.Errorf("lastWrittenKinds = %v, Note's StageGroup should be untouched", store.lastWrittenKinds)
	}
}

// TestRenameStageGroupShadowsBuiltinKinds is the fan-out test: task-flow
// (a baked-in default) is referenced by Task, Goblin, and Talk. Renaming it
// with no prior user kinds must append THREE shadow copies, one per
// referencing default kind, each with StageGroup updated to the new name —
// mirroring DetectOrphans' own fan-out reasoning for why a group edit is
// scoped per-kind rather than per-group.
func TestRenameStageGroupShadowsBuiltinKinds(t *testing.T) {
	store := newFakeStore() // no seed — nothing shadowed yet

	rewritten, err := stage.RenameStageGroup(store, "task-flow", "todo-flow")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	defaults, err := stage.DefaultKinds()
	if err != nil {
		t.Fatalf("DefaultKinds: %v", err)
	}
	wantCount := 0
	wantNames := map[string]bool{}
	for _, d := range defaults {
		if d.StageGroup == "task-flow" {
			wantCount++
			wantNames[d.Name] = true
		}
	}
	if wantCount < 2 {
		t.Fatalf("test precondition failed: expected at least 2 default kinds referencing task-flow, got %d", wantCount)
	}

	if rewritten != wantCount {
		t.Errorf("rewritten = %d, want %d (one shadow per default kind referencing task-flow)", rewritten, wantCount)
	}
	if len(store.lastWrittenKinds) != wantCount {
		t.Fatalf("lastWrittenKinds len = %d, want %d", len(store.lastWrittenKinds), wantCount)
	}
	for _, k := range store.lastWrittenKinds {
		if !wantNames[k.Name] {
			t.Errorf("unexpected shadow for kind %q", k.Name)
		}
		if k.StageGroup != "todo-flow" {
			t.Errorf("shadow for %q has StageGroup %q, want %q", k.Name, k.StageGroup, "todo-flow")
		}
	}
}

// TestRenameStageGroupSkipsAlreadyShadowedDefault verifies that when a
// default kind is already shadowed in the user file, RenameStageGroup
// rewrites that existing shadow rather than appending a second one.
func TestRenameStageGroupSkipsAlreadyShadowedDefault(t *testing.T) {
	store := newFakeStore()
	// Task is a baked-in default referencing task-flow; here it's already
	// shadowed by the user with a custom glyph.
	store.kindsSeed = []types.Kind{
		{Name: "Task", StageGroup: "task-flow", Glyph: "★"},
	}

	rewritten, err := stage.RenameStageGroup(store, "task-flow", "todo-flow")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Task itself: rewritten in place (1). Every OTHER default kind
	// referencing task-flow still needs a fresh shadow.
	defaults, err := stage.DefaultKinds()
	if err != nil {
		t.Fatalf("DefaultKinds: %v", err)
	}
	otherDefaultsReferencing := 0
	for _, d := range defaults {
		if d.Name != "Task" && d.StageGroup == "task-flow" {
			otherDefaultsReferencing++
		}
	}
	wantTotal := 1 + otherDefaultsReferencing
	if rewritten != wantTotal {
		t.Errorf("rewritten = %d, want %d", rewritten, wantTotal)
	}

	var taskEntries int
	for _, k := range store.lastWrittenKinds {
		if k.Name == "Task" {
			taskEntries++
			if k.StageGroup != "todo-flow" {
				t.Errorf("Task.StageGroup = %q, want %q", k.StageGroup, "todo-flow")
			}
			if k.Glyph != "★" {
				t.Errorf("Task.Glyph = %q, want preserved %q (rewritten in place, not duplicated)", k.Glyph, "★")
			}
		}
	}
	if taskEntries != 1 {
		t.Errorf("expected exactly 1 Task entry in lastWrittenKinds, got %d — should rewrite the existing shadow, not append a second", taskEntries)
	}
}

// TestRenameStageGroupReadFailureAborts verifies a ReadKinds failure is
// fatal and no write is attempted — mirrors the fatal-read reasoning in
// kind_form.go's submit branch (WriteKinds overwrites the whole file, so
// proceeding on an empty base would destroy every existing user kind).
func TestRenameStageGroupReadFailureAborts(t *testing.T) {
	store := newFakeStore()
	store.kindsReadErr = errors.New("disk full")

	_, err := stage.RenameStageGroup(store, "task-flow", "todo-flow")
	if err == nil {
		t.Fatal("expected an error when ReadKinds fails")
	}
	if store.kindsWritten {
		t.Error("WriteKinds should not be called when ReadKinds fails")
	}
}

// TestRenameStageGroupWriteFailure verifies a WriteKinds failure surfaces as
// an error.
func TestRenameStageGroupWriteFailure(t *testing.T) {
	store := newFakeStore()
	store.kindsWriteErr = errors.New("permission denied")
	store.kindsSeed = []types.Kind{{Name: "Errand", StageGroup: "old-flow"}}

	_, err := stage.RenameStageGroup(store, "old-flow", "new-flow")
	if err == nil {
		t.Fatal("expected an error when WriteKinds fails")
	}
}
