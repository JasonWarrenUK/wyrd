package tui

import (
	"errors"
	"strings"
	"testing"

	"github.com/jasonwarrenuk/wyrd/internal/store"
	"github.com/jasonwarrenuk/wyrd/internal/types"
)

// ---------------------------------------------------------------------------
// Rename-failure tests for kindFormSubmitMsg / stageFormSubmitMsg. Covers the
// bug flagged in PR #52 review 4882856996: on a partial/full rename cascade
// failure, the orphan scan and active remap hand-off used to run
// unconditionally on `refreshed`, ignoring renameErr entirely. Nodes/kinds
// still holding the old (now-gone) name resolve as Unresolvable rather than
// Orphaned — a class nothing downstream (ApplyRemap) can repair — so the
// hand-off must never fire on a rename failure, and the sticky error message
// must survive rather than being skipped by the hand-off's early return.
//
// Live in the internal package for the same reason remap_command_internal_
// test.go does. Reuses newRemapTestModel and driveSubmitMsg from that file
// and kind_edit_orphan_handoff_internal_test.go.
// ---------------------------------------------------------------------------

// failingStore wraps a real *store.Store and fails specific write calls,
// mirroring the failIDs/kindsWriteErr injection pattern in
// internal/stage/rename_test.go's fakeStore rather than inventing a second
// mock shape. Everything else delegates to the wrapped store, so the rest of
// Model's plumbing (ReadKinds, node reads, etc.) behaves normally.
type failingStore struct {
	types.StoreFS
	failNodeID    string // UpdateNode fails for this node ID only; "" disables
	failWriteKind bool   // WriteKinds always fails when true
}

func (f *failingStore) UpdateNode(id string, updates map[string]interface{}) (*types.Node, error) {
	if id == f.failNodeID {
		return nil, errors.New("simulated write failure")
	}
	return f.StoreFS.UpdateNode(id, updates)
}

func (f *failingStore) WriteKinds(kinds []types.Kind) error {
	if f.failWriteKind {
		return errors.New("simulated write failure")
	}
	return f.StoreFS.WriteKinds(kinds)
}

// TestKindEditRenameFailureSkipsHandoff verifies that a kind rename whose
// RenameKind cascade partially fails does not auto-open the remap form, even
// though the failure leaves an orphan-shaped scan result available.
//
// Renames a CUSTOM kind ("Errand"), not a baked-in default — see
// TestKindEditRenameFailureMarksSticky's doc comment for why a default kind
// like "Task" doesn't actually go Unresolvable when its user-file entry is
// renamed away (MergeKinds always keeps the embedded default resolvable).
func TestKindEditRenameFailureSkipsHandoff(t *testing.T) {
	var failID string
	m := newRemapTestModel(t, func(s *store.Store) {
		// kindFormPane.Update already wrote kinds.jsonc before emitting
		// kindFormSubmitMsg (see kind_form.go's submit branch) — by the time
		// this handler runs, the registry on disk already reflects the
		// rename (Errand replaced by Chore via upsertKind). Seed that
		// post-submit state directly, since this test drives
		// kindFormSubmitMsg straight into Model.Update rather than through
		// the real form.
		if err := s.WriteKinds([]types.Kind{
			{Name: "Chore", StageGroup: "task-flow", Glyph: "!"},
		}); err != nil {
			t.Fatalf("WriteKinds: %v", err)
		}
		node, err := s.CreateNode("stuck node", []string{"task"})
		if err != nil {
			t.Fatalf("CreateNode: %v", err)
		}
		failID = node.ID
		if _, err := s.UpdateNode(node.ID, map[string]interface{}{"kind": "Errand", "stage": "Open"}); err != nil {
			t.Fatalf("UpdateNode: %v", err)
		}
	})
	realStore := m.store
	m.store = &failingStore{StoreFS: realStore, failNodeID: failID}

	m2 := driveSubmitMsg(t, m, kindFormSubmitMsg{name: "Chore", renamedFrom: "Errand"})

	if _, isForm := m2.rightPane.(formActivePane); isForm {
		t.Errorf("expected no form mounted on a rename failure, got %T", m2.rightPane)
	}
}

// TestKindEditRenameFailureMarksSticky verifies the rename-failure message
// stays on screen (MarkCaptureSticky fires) and reports the unresolvable
// count, rather than being clobbered by the hand-off's early return path.
// This is the regression test for the bypass found during planning: the
// early return inside the orphan-scan block used to skip the renameErr
// handling entirely whenever orphans were also present.
//
// Renames a CUSTOM kind ("Errand"), not a baked-in default: MergeKinds
// always appends the embedded defaults ahead of the user file, so renaming a
// default like "Task" away leaves "Task" still resolvable from the defaults
// list regardless of what kinds.jsonc says — the node wouldn't actually go
// Unresolvable in that case. A custom kind has no such fallback, so once
// upsertKind replaces its entry, a node stuck on the old name has nothing
// left to resolve against.
func TestKindEditRenameFailureMarksSticky(t *testing.T) {
	var failID string
	m := newRemapTestModel(t, func(s *store.Store) {
		// kindFormPane.Update already wrote kinds.jsonc before emitting
		// kindFormSubmitMsg (see kind_form.go's submit branch) — by the time
		// this handler runs, the registry on disk already reflects the
		// rename (Errand replaced by Chore via upsertKind). Seed that
		// post-submit state directly, since this test drives
		// kindFormSubmitMsg straight into Model.Update rather than through
		// the real form.
		if err := s.WriteKinds([]types.Kind{
			{Name: "Chore", StageGroup: "task-flow", Glyph: "!"},
		}); err != nil {
			t.Fatalf("WriteKinds: %v", err)
		}
		node, err := s.CreateNode("stuck node", []string{"task"})
		if err != nil {
			t.Fatalf("CreateNode: %v", err)
		}
		failID = node.ID
		if _, err := s.UpdateNode(node.ID, map[string]interface{}{"kind": "Errand", "stage": "Open"}); err != nil {
			t.Fatalf("UpdateNode: %v", err)
		}
	})
	realStore := m.store
	m.store = &failingStore{StoreFS: realStore, failNodeID: failID}

	m2 := driveSubmitMsg(t, m, kindFormSubmitMsg{name: "Chore", renamedFrom: "Errand"})

	if !m2.statusBar.CaptureSticky() {
		t.Error("expected the rename-failure message to be sticky")
	}
	view := m2.View().Content
	if !strings.Contains(view, "simulated write failure") {
		t.Errorf("expected the failure reason in view; got: %q", truncateForTest(view, 300))
	}
	if !strings.Contains(view, "unresolvable") {
		t.Errorf("expected an unresolvable-node count in view; got: %q", truncateForTest(view, 300))
	}
}

// TestStageEditRenameFailureSkipsHandoff mirrors
// TestKindEditRenameFailureSkipsHandoff for stageFormSubmitMsg /
// RenameStageGroup: a failed WriteKinds during the group-rename cascade
// leaves referencing kinds still pointing at the old (now-gone) group name.
//
// Renames a CUSTOM group ("errand-flow"), not a baked-in default —
// MergeStageGroups, like MergeKinds, always appends the embedded defaults
// ahead of the user file, so renaming a default like "task-flow" away
// leaves it still resolvable from the defaults list regardless of what
// stages.jsonc says (see TestKindEditRenameFailureMarksSticky's doc comment
// for the kind-side version of this). A custom group has no such fallback.
//
// stageFormPane.Update already wrote stages.jsonc before emitting
// stageFormSubmitMsg (see stage_form.go's submit branch) — by the time this
// handler runs, the registry on disk already reflects the rename
// (errand-flow replaced by repaired-flow via upsertStageGroup). Seed that
// post-submit state directly, since this test drives stageFormSubmitMsg
// straight into Model.Update rather than through the real form. kinds.jsonc
// is deliberately seeded with Chore still pointing at "errand-flow":
// RenameStageGroup's own WriteKinds call is what's failing, so that
// reference is never repointed to "repaired-flow".
func TestStageEditRenameFailureSkipsHandoff(t *testing.T) {
	m := newRemapTestModel(t, func(s *store.Store) {
		if err := s.WriteStages([]types.StageGroup{
			{Name: "repaired-flow", Stages: []string{"Open", "Done"}, Cycle: types.CycleTerminate},
		}); err != nil {
			t.Fatalf("WriteStages: %v", err)
		}
		if err := s.WriteKinds([]types.Kind{
			{Name: "Chore", StageGroup: "errand-flow", Glyph: "!"},
		}); err != nil {
			t.Fatalf("WriteKinds: %v", err)
		}
		node, err := s.CreateNode("stuck node", []string{"task"})
		if err != nil {
			t.Fatalf("CreateNode: %v", err)
		}
		if _, err := s.UpdateNode(node.ID, map[string]interface{}{"kind": "Chore", "stage": "Open"}); err != nil {
			t.Fatalf("UpdateNode: %v", err)
		}
	})
	realStore := m.store
	m.store = &failingStore{StoreFS: realStore, failWriteKind: true}

	m2 := driveSubmitMsg(t, m, stageFormSubmitMsg{name: "repaired-flow", renamedFrom: "errand-flow"})

	if _, isForm := m2.rightPane.(formActivePane); isForm {
		t.Errorf("expected no form mounted on a rename failure, got %T", m2.rightPane)
	}
}

// TestStageEditRenameFailureMarksSticky mirrors
// TestKindEditRenameFailureMarksSticky for the stage-group path. See
// TestStageEditRenameFailureSkipsHandoff's doc comment for why a custom
// group is used and why stages.jsonc/kinds.jsonc are seeded with the
// post-submit (renamed) state.
func TestStageEditRenameFailureMarksSticky(t *testing.T) {
	m := newRemapTestModel(t, func(s *store.Store) {
		if err := s.WriteStages([]types.StageGroup{
			{Name: "repaired-flow", Stages: []string{"Open", "Done"}, Cycle: types.CycleTerminate},
		}); err != nil {
			t.Fatalf("WriteStages: %v", err)
		}
		if err := s.WriteKinds([]types.Kind{
			{Name: "Chore", StageGroup: "errand-flow", Glyph: "!"},
		}); err != nil {
			t.Fatalf("WriteKinds: %v", err)
		}
		node, err := s.CreateNode("stuck node", []string{"task"})
		if err != nil {
			t.Fatalf("CreateNode: %v", err)
		}
		if _, err := s.UpdateNode(node.ID, map[string]interface{}{"kind": "Chore", "stage": "Open"}); err != nil {
			t.Fatalf("UpdateNode: %v", err)
		}
	})
	realStore := m.store
	m.store = &failingStore{StoreFS: realStore, failWriteKind: true}

	m2 := driveSubmitMsg(t, m, stageFormSubmitMsg{name: "repaired-flow", renamedFrom: "errand-flow"})

	if !m2.statusBar.CaptureSticky() {
		t.Error("expected the rename-failure message to be sticky")
	}
	view := m2.View().Content
	if !strings.Contains(view, "simulated write failure") {
		t.Errorf("expected the failure reason in view; got: %q", truncateForTest(view, 300))
	}
	if !strings.Contains(view, "unresolvable") {
		t.Errorf("expected an unresolvable-node count in view; got: %q", truncateForTest(view, 300))
	}
}

// TestOrphanAdvisoryReportsUnresolvableAlone verifies orphanAdvisory no
// longer returns "" when a report has zero Orphans but nonzero Unresolvable
// — the success-path counterpart to the failure-path fix above. Drives
// DetectOrphans indirectly via a node whose Kind isn't in the registry at
// all, which is the same shape RenameKind's partial-failure leaves behind.
func TestOrphanAdvisoryReportsUnresolvableAlone(t *testing.T) {
	m := newRemapTestModel(t, func(s *store.Store) {
		node, err := s.CreateNode("orphaned by nothing resolving", []string{"task"})
		if err != nil {
			t.Fatalf("CreateNode: %v", err)
		}
		if _, err := s.UpdateNode(node.ID, map[string]interface{}{"kind": "NoSuchKind", "stage": "Open"}); err != nil {
			t.Fatalf("UpdateNode: %v", err)
		}
	})

	advisory := m.orphanAdvisory()
	if advisory == "" {
		t.Fatal("expected a non-empty advisory for an all-unresolvable report")
	}
	if !strings.Contains(advisory, "unresolvable") {
		t.Errorf("expected the advisory to mention unresolvable nodes, got %q", advisory)
	}
	if strings.Contains(advisory, "orphaned stages") {
		t.Errorf("expected no orphaned-stages text when Orphans is empty, got %q", advisory)
	}
}
