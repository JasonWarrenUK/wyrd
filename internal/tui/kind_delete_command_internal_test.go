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
// ":kinds delete <name>" dispatch tests. Live in the internal package for the
// same reason remap_command_internal_test.go does. Reuses kindsCommand,
// runCommand, truncateForTest and driveSubmitMsg from the existing suite.
// ---------------------------------------------------------------------------

// newKindDeleteTestModel is newRemapTestModel's twin for delete tests that
// need a user kind already present in the merged registry before the model
// exists — dispatch reads m.kinds.Lookup directly, so a kind only written to
// disk via the seed callback (newRemapTestModel's pattern) would not be
// visible to it. userKinds is merged over the baked-in defaults exactly as
// buildRegistries does in production; nodesSeed additionally seeds the
// store/index with nodes.
func newKindDeleteTestModel(t *testing.T, userKinds []types.Kind, nodesSeed func(s *store.Store)) Model {
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

	if len(userKinds) > 0 {
		if err := s.WriteKinds(userKinds); err != nil {
			t.Fatalf("WriteKinds: %v", err)
		}
	}
	if nodesSeed != nil {
		nodesSeed(s)
	}

	kindDefaults, err := stage.DefaultKinds()
	if err != nil {
		t.Fatalf("DefaultKinds: %v", err)
	}
	kinds := stage.MergeKinds(kindDefaults, userKinds)

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

func TestKindsDeleteRejectsUntouchedDefault(t *testing.T) {
	m := newKindDeleteTestModel(t, nil, nil)

	cmd := kindsCommand(t, m)
	m = runCommand(t, m, cmd, []string{"delete", "Task"})

	if _, isForm := m.rightPane.(formActivePane); isForm {
		t.Error("expected no form mounted for an untouched default")
	}
	view := m.View().Content
	if !strings.Contains(view, "nothing to delete") || !strings.Contains(view, ":kinds edit") {
		t.Errorf("expected a rejection naming :kinds edit; got: %q", truncateForTest(view, 300))
	}
}

func TestKindsDeleteNoNameShowsUsage(t *testing.T) {
	m := newKindDeleteTestModel(t, nil, nil)

	cmd := kindsCommand(t, m)
	m = runCommand(t, m, cmd, []string{"delete"})

	if _, isForm := m.rightPane.(formActivePane); isForm {
		t.Error("expected no form mounted when no name is given")
	}
	view := m.View().Content
	if !strings.Contains(view, "Usage: :kinds delete") {
		t.Errorf("expected usage message in view; got: %q", truncateForTest(view, 300))
	}
}

func TestKindsDeleteUnknownNameShowsNotFound(t *testing.T) {
	m := newKindDeleteTestModel(t, nil, nil)

	cmd := kindsCommand(t, m)
	m = runCommand(t, m, cmd, []string{"delete", "Sasquatch"})

	if _, isForm := m.rightPane.(formActivePane); isForm {
		t.Error("expected no form mounted for an unknown kind name")
	}
	view := m.View().Content
	if !strings.Contains(view, `No kind "Sasquatch"`) {
		t.Errorf("expected not-found message in view; got: %q", truncateForTest(view, 300))
	}
}

func TestKindsDeleteMultiWordNameJoins(t *testing.T) {
	m := newKindDeleteTestModel(t, nil, nil)

	cmd := kindsCommand(t, m)
	m = runCommand(t, m, cmd, []string{"delete", "My", "Kind"})

	view := m.View().Content
	if !strings.Contains(view, `"My Kind"`) {
		t.Errorf("expected the not-found message to echo the joined name %q; got: %q", "My Kind", truncateForTest(view, 300))
	}
}

func TestKindsDeleteBlockedWhenFormActive(t *testing.T) {
	m := newKindDeleteTestModel(t, nil, nil)

	cmd := kindsCommand(t, m)
	m = runCommand(t, m, cmd, []string{"new"})
	if _, isForm := m.rightPane.(formActivePane); !isForm {
		t.Fatal("precondition failed: create form should be mounted")
	}
	createPane := m.rightPane

	m = runCommand(t, m, cmd, []string{"delete", "Task"})

	if m.rightPane != createPane {
		t.Error("expected the original create form to remain mounted, guard should have blocked the delete open")
	}
}

// TestKindsDeleteShadowedOffersRevertForEditedShadow covers a plain
// ShadowEdited entry (a hand-edited default).
func TestKindsDeleteShadowedOffersRevertForEditedShadow(t *testing.T) {
	shadow := types.Kind{
		Name: "Task", StageGroup: "task-flow", Glyph: "!",
		ShadowOf: stage.DefaultKindHash("Task"), ShadowReason: types.ShadowEdited,
	}
	m := newKindDeleteTestModel(t, []types.Kind{shadow}, nil)

	cmd := kindsCommand(t, m)
	m = runCommand(t, m, cmd, []string{"delete", "Task"})

	fp, ok := m.rightPane.(kindDeleteFormPane)
	if !ok {
		t.Fatalf("expected rightPane to be a kindDeleteFormPane, got %T", m.rightPane)
	}
	if fp.mode != kindDeleteRevert {
		t.Errorf("mode = %v, want kindDeleteRevert", fp.mode)
	}
}

// TestKindsDeleteShadowedOffersRevertForLegacyEmptyShadowReason covers the
// backward-compat case: a non-empty ShadowOf with an empty ShadowReason is
// treated identically to ShadowEdited.
func TestKindsDeleteShadowedOffersRevertForLegacyEmptyShadowReason(t *testing.T) {
	shadow := types.Kind{
		Name: "Task", StageGroup: "task-flow", Glyph: "!",
		ShadowOf: stage.DefaultKindHash("Task"),
	}
	m := newKindDeleteTestModel(t, []types.Kind{shadow}, nil)

	cmd := kindsCommand(t, m)
	m = runCommand(t, m, cmd, []string{"delete", "Task"})

	fp, ok := m.rightPane.(kindDeleteFormPane)
	if !ok {
		t.Fatalf("expected rightPane to be a kindDeleteFormPane, got %T", m.rightPane)
	}
	if fp.mode != kindDeleteRevert {
		t.Errorf("mode = %v, want kindDeleteRevert", fp.mode)
	}
}

// TestKindsDeleteTombstoneOffersRevertWithZeroAffectedNodes is the flagged
// case: a tombstone lives under the default's own name, written only
// alongside a RenameKind that already moved every node off that name. A
// non-zero affected count would mean the rename cascade left something
// behind — so this asserts the count is exactly zero.
func TestKindsDeleteTombstoneOffersRevertWithZeroAffectedNodes(t *testing.T) {
	tombstone := types.Kind{
		Name: "Task", StageGroup: "task-flow", Glyph: "★", Colour: "#9b70ff",
		ShadowOf: stage.DefaultKindHash("Task"), ShadowReason: types.ShadowTombstone,
	}
	// The renamed-to kind, holding the node that used to be "Task".
	renamed := types.Kind{Name: "Chore", StageGroup: "task-flow", Glyph: "!"}

	m := newKindDeleteTestModel(t, []types.Kind{tombstone, renamed}, func(s *store.Store) {
		node, err := s.CreateNode("moved off Task already", []string{"task"})
		if err != nil {
			t.Fatalf("CreateNode: %v", err)
		}
		if _, err := s.UpdateNode(node.ID, map[string]interface{}{"kind": "Chore", "stage": "Open"}); err != nil {
			t.Fatalf("UpdateNode: %v", err)
		}
	})

	cmd := kindsCommand(t, m)
	m = runCommand(t, m, cmd, []string{"delete", "Task"})

	fp, ok := m.rightPane.(kindDeleteFormPane)
	if !ok {
		t.Fatalf("expected rightPane to be a kindDeleteFormPane, got %T", m.rightPane)
	}
	if fp.mode != kindDeleteRevert {
		t.Errorf("mode = %v, want kindDeleteRevert", fp.mode)
	}
	if fp.affected != 0 {
		t.Errorf("affected = %d, want 0 — RenameKind should already have moved every node off the tombstoned name", fp.affected)
	}
	view := m.View().Content
	if !strings.Contains(view, "placeholder") {
		t.Errorf("expected tombstone-specific wording in view; got: %q", truncateForTest(view, 300))
	}
}

// TestKindsDeleteRevertBlockedWhenDefaultGroupMissing covers the fan-out
// hazard: a ShadowRenameFanOut shadow's embedded default still names
// whatever group it pointed at before the fan-out rewrite (Goblin's
// built-in default points at task-flow). If that group has since been
// renamed away — simulated here by omitting task-flow from the model's
// stage-group registry entirely — reverting would resurrect a StageGroup
// reference that cannot resolve, stranding every node of the kind as
// Unresolvable. The handler must refuse rather than allow that.
func TestKindsDeleteRevertBlockedWhenDefaultGroupMissing(t *testing.T) {
	shadow := types.Kind{
		Name: "Goblin", StageGroup: "renamed-away-flow", Glyph: "!",
		ShadowOf: "sha256:0000000000000000", ShadowReason: types.ShadowRenameFanOut,
	}
	m := newKindDeleteTestModel(t, []types.Kind{shadow}, nil)
	m.stageGroups = types.NewStageGroupRegistry([]types.StageGroup{
		{Name: "renamed-away-flow", Stages: []string{"Open", "Done"}, Cycle: types.CycleTerminate},
	})

	cmd := kindsCommand(t, m)
	m = runCommand(t, m, cmd, []string{"delete", "Goblin"})

	if _, isForm := m.rightPane.(formActivePane); isForm {
		t.Error("expected no form mounted — the revert should be refused")
	}
	view := m.View().Content
	if !strings.Contains(view, "Cannot restore") {
		t.Errorf("expected a refusal naming the missing group; got: %q", truncateForTest(view, 300))
	}
}

// TestRevertShadowedKindWithDifferentGroupMountsRemapForm verifies revert is
// not a safe no-op: Task's built-in default points at task-flow
// (Open/Maybe/Later/Soon/Now/Done). Here Task is shadowed pointing at
// content-flow (Active/Reference) instead, and a node holds "Reference" — a
// stage valid in content-flow but absent from task-flow. Reverting drops the
// shadow, so Task's resolved group snaps back to task-flow, and the node's
// stage becomes an Orphan (not Unresolvable — the kind itself still
// resolves), which the remap form can repair.
//
// Driven directly via kindDeleteSubmitMsg (mirroring
// TestKindEditOrphanHandoffOpensRemapForm's simulation style), which is
// emitted only AFTER the pane has already called stage.DeleteKind — so, as
// TestKindEditRenameFailureSkipsHandoff's setup comment explains for the
// equivalent rename case, this test pre-writes kinds.jsonc to the
// post-delete state (Task's shadow gone) rather than seeding the
// pre-delete shadow the way the dispatch tests above do.
func TestRevertShadowedKindWithDifferentGroupMountsRemapForm(t *testing.T) {
	// No user kinds seeded: an empty kinds.jsonc is exactly what DeleteKind
	// leaves behind once it removes Task's only shadow entry.
	m := newKindDeleteTestModel(t, nil, func(s *store.Store) {
		node, err := s.CreateNode("valid in content-flow only", []string{"task"})
		if err != nil {
			t.Fatalf("CreateNode: %v", err)
		}
		if _, err := s.UpdateNode(node.ID, map[string]interface{}{"kind": "Task", "stage": "Reference"}); err != nil {
			t.Fatalf("UpdateNode: %v", err)
		}
	})
	m2 := driveSubmitMsg(t, m, kindDeleteSubmitMsg{name: "Task", mode: kindDeleteRevert})

	if _, ok := m2.rightPane.(remapFormPane); !ok {
		t.Errorf("expected rightPane to be a remapFormPane, got %T", m2.rightPane)
	}
}

// TestKindsDeleteResolvesNameCaseInsensitively is the canonical-name trap:
// every downstream step (the node count, the WriteKinds filter) must use the
// registry's canonical name, never the raw typed argument.
func TestKindsDeleteResolvesNameCaseInsensitively(t *testing.T) {
	custom := types.Kind{Name: "Goblin", StageGroup: "task-flow", Glyph: "!"}
	m := newKindDeleteTestModel(t, []types.Kind{custom}, func(s *store.Store) {
		for _, title := range []string{"one", "two", "three"} {
			node, err := s.CreateNode(title, []string{"task"})
			if err != nil {
				t.Fatalf("CreateNode: %v", err)
			}
			if _, err := s.UpdateNode(node.ID, map[string]interface{}{"kind": "Goblin", "stage": "Open"}); err != nil {
				t.Fatalf("UpdateNode: %v", err)
			}
		}
	})

	cmd := kindsCommand(t, m)
	m = runCommand(t, m, cmd, []string{"delete", "goblin"})

	fp, ok := m.rightPane.(kindDeleteFormPane)
	if !ok {
		t.Fatalf("expected rightPane to be a kindDeleteFormPane, got %T", m.rightPane)
	}
	if fp.mode != kindDeleteReassign {
		t.Errorf("mode = %v, want kindDeleteReassign", fp.mode)
	}
	if fp.affected != 3 {
		t.Errorf("affected = %d, want 3", fp.affected)
	}
	if fp.kind.Name != "Goblin" {
		t.Errorf("fp.kind.Name = %q, want canonical %q", fp.kind.Name, "Goblin")
	}
}

func TestKindsDeleteCustomWithoutNodesOffersDirectConfirm(t *testing.T) {
	custom := types.Kind{Name: "Goblin", StageGroup: "task-flow", Glyph: "!"}
	m := newKindDeleteTestModel(t, []types.Kind{custom}, nil)

	cmd := kindsCommand(t, m)
	m = runCommand(t, m, cmd, []string{"delete", "Goblin"})

	fp, ok := m.rightPane.(kindDeleteFormPane)
	if !ok {
		t.Fatalf("expected rightPane to be a kindDeleteFormPane, got %T", m.rightPane)
	}
	if fp.mode != kindDeleteCustom {
		t.Errorf("mode = %v, want kindDeleteCustom", fp.mode)
	}
}

func TestKindsDeleteCustomWithNodesOffersReassignment(t *testing.T) {
	custom := types.Kind{Name: "Goblin", StageGroup: "task-flow", Glyph: "!"}
	m := newKindDeleteTestModel(t, []types.Kind{custom}, func(s *store.Store) {
		node, err := s.CreateNode("held by Goblin", []string{"task"})
		if err != nil {
			t.Fatalf("CreateNode: %v", err)
		}
		if _, err := s.UpdateNode(node.ID, map[string]interface{}{"kind": "Goblin", "stage": "Open"}); err != nil {
			t.Fatalf("UpdateNode: %v", err)
		}
	})

	cmd := kindsCommand(t, m)
	m = runCommand(t, m, cmd, []string{"delete", "Goblin"})

	fp, ok := m.rightPane.(kindDeleteFormPane)
	if !ok {
		t.Fatalf("expected rightPane to be a kindDeleteFormPane, got %T", m.rightPane)
	}
	if fp.mode != kindDeleteReassign {
		t.Errorf("mode = %v, want kindDeleteReassign", fp.mode)
	}
	if fp.affected != 1 {
		t.Errorf("affected = %d, want 1", fp.affected)
	}
}

// TestKindsDeleteReassignHandsOffToRemap verifies that completing a
// reassign-mode delete (driven directly via kindDeleteSubmitMsg, mirroring
// TestKindEditOrphanHandoffOpensRemapForm's simulation style) opens the
// remap form when the reassignment orphans nodes.
func TestKindsDeleteReassignHandsOffToRemap(t *testing.T) {
	custom := types.Kind{Name: "Goblin", StageGroup: "task-flow", Glyph: "!"}
	m := newKindDeleteTestModel(t, []types.Kind{custom}, func(s *store.Store) {
		node, err := s.CreateNode("will become orphaned", []string{"task"})
		if err != nil {
			t.Fatalf("CreateNode: %v", err)
		}
		// A stage absent from Task's group, but held under Goblin here —
		// after ReassignKind moves it to Task it becomes an Orphan.
		if _, err := s.UpdateNode(node.ID, map[string]interface{}{"kind": "Goblin", "stage": "Whenever"}); err != nil {
			t.Fatalf("UpdateNode: %v", err)
		}
	})

	m2 := driveSubmitMsg(t, m, kindDeleteSubmitMsg{name: "Goblin", mode: kindDeleteReassign, reassigned: 1})

	if _, ok := m2.rightPane.(remapFormPane); !ok {
		t.Errorf("expected rightPane to be a remapFormPane, got %T", m2.rightPane)
	}
}

// TestKindsDeleteBareDoesNotHandOffToRemap verifies a bare custom delete
// (no reassignment) never chains into the remap form — T3: there is nothing
// left that could be an Orphan once the kind itself is gone.
func TestKindsDeleteBareDoesNotHandOffToRemap(t *testing.T) {
	m := newKindDeleteTestModel(t, nil, nil)

	m2 := driveSubmitMsg(t, m, kindDeleteSubmitMsg{name: "Goblin", mode: kindDeleteCustom})

	if _, ok := m2.rightPane.(remapFormPane); ok {
		t.Error("expected no remap hand-off after a bare custom delete")
	}
}
