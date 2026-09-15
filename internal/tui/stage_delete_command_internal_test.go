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
// ":stages delete <name>" dispatch tests. stageDeleteFormPane's twin of
// kind_delete_command_internal_test.go — same reasoning for why a dedicated
// model builder is needed instead of newRemapTestModel.
// ---------------------------------------------------------------------------

// newStageDeleteTestModel mirrors newKindDeleteTestModel, seeding user
// stage groups (and optionally user kinds that reference them) directly
// into the merged registry before the model exists.
func newStageDeleteTestModel(t *testing.T, userGroups []types.StageGroup, userKinds []types.Kind, nodesSeed func(s *store.Store)) Model {
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

	if len(userGroups) > 0 {
		if err := s.WriteStages(userGroups); err != nil {
			t.Fatalf("WriteStages: %v", err)
		}
	}
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
	stageGroups := stage.MergeStageGroups(groupDefaults, userGroups)

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

func TestStagesDeleteRejectsUntouchedDefault(t *testing.T) {
	m := newStageDeleteTestModel(t, nil, nil, nil)

	cmd := stagesCommand(t, m)
	m = runCommand(t, m, cmd, []string{"delete", "task-flow"})

	if _, isForm := m.rightPane.(formActivePane); isForm {
		t.Error("expected no form mounted for an untouched default")
	}
	view := m.View().Content
	if !strings.Contains(view, "nothing to delete") || !strings.Contains(view, ":stages edit") {
		t.Errorf("expected a rejection naming :stages edit; got: %q", truncateForTest(view, 300))
	}
}

func TestStagesDeleteNoNameShowsUsage(t *testing.T) {
	m := newStageDeleteTestModel(t, nil, nil, nil)

	cmd := stagesCommand(t, m)
	m = runCommand(t, m, cmd, []string{"delete"})

	if _, isForm := m.rightPane.(formActivePane); isForm {
		t.Error("expected no form mounted when no name is given")
	}
	view := m.View().Content
	if !strings.Contains(view, "Usage: :stages delete") {
		t.Errorf("expected usage message in view; got: %q", truncateForTest(view, 300))
	}
}

func TestStagesDeleteUnknownNameShowsNotFound(t *testing.T) {
	m := newStageDeleteTestModel(t, nil, nil, nil)

	cmd := stagesCommand(t, m)
	m = runCommand(t, m, cmd, []string{"delete", "nonexistent-flow"})

	if _, isForm := m.rightPane.(formActivePane); isForm {
		t.Error("expected no form mounted for an unknown group name")
	}
	view := m.View().Content
	if !strings.Contains(view, `No stage group "nonexistent-flow"`) {
		t.Errorf("expected not-found message in view; got: %q", truncateForTest(view, 300))
	}
}

func TestStagesDeleteBlockedWhenFormActive(t *testing.T) {
	m := newStageDeleteTestModel(t, nil, nil, nil)

	cmd := stagesCommand(t, m)
	m = runCommand(t, m, cmd, []string{"new"})
	if _, isForm := m.rightPane.(formActivePane); !isForm {
		t.Fatal("precondition failed: create form should be mounted")
	}
	createPane := m.rightPane

	m = runCommand(t, m, cmd, []string{"delete", "task-flow"})

	if m.rightPane != createPane {
		t.Error("expected the original create form to remain mounted, guard should have blocked the delete open")
	}
}

func TestStagesDeleteShadowedOffersRevert(t *testing.T) {
	shadow := types.StageGroup{
		Name: "task-flow", Stages: []string{"Open", "Done"}, Cycle: types.CycleTerminate,
		ShadowOf: stage.DefaultStageGroupHash("task-flow"), ShadowReason: types.ShadowEdited,
	}
	m := newStageDeleteTestModel(t, []types.StageGroup{shadow}, nil, nil)

	cmd := stagesCommand(t, m)
	m = runCommand(t, m, cmd, []string{"delete", "task-flow"})

	fp, ok := m.rightPane.(stageDeleteFormPane)
	if !ok {
		t.Fatalf("expected rightPane to be a stageDeleteFormPane, got %T", m.rightPane)
	}
	if fp.mode != stageDeleteRevert {
		t.Errorf("mode = %v, want stageDeleteRevert", fp.mode)
	}
}

func TestStagesDeleteTombstoneOffersRevert(t *testing.T) {
	tombstone := types.StageGroup{
		Name: "task-flow", Stages: []string{"Open", "Maybe", "Later", "Soon", "Now", "Done"}, Cycle: types.CycleTerminate,
		ShadowOf: stage.DefaultStageGroupHash("task-flow"), ShadowReason: types.ShadowTombstone,
	}
	renamed := types.StageGroup{Name: "todo-flow", Stages: []string{"Open", "Done"}, Cycle: types.CycleTerminate}
	m := newStageDeleteTestModel(t, []types.StageGroup{tombstone, renamed}, nil, nil)

	cmd := stagesCommand(t, m)
	m = runCommand(t, m, cmd, []string{"delete", "task-flow"})

	fp, ok := m.rightPane.(stageDeleteFormPane)
	if !ok {
		t.Fatalf("expected rightPane to be a stageDeleteFormPane, got %T", m.rightPane)
	}
	if fp.mode != stageDeleteRevert {
		t.Errorf("mode = %v, want stageDeleteRevert", fp.mode)
	}
	view := m.View().Content
	if !strings.Contains(view, "placeholder") {
		t.Errorf("expected tombstone-specific wording in view; got: %q", truncateForTest(view, 300))
	}
}

func TestStagesDeleteCustomWithoutKindsOffersDirectConfirm(t *testing.T) {
	custom := types.StageGroup{Name: "custom-flow", Stages: []string{"A", "B"}, Cycle: types.CycleTerminate}
	m := newStageDeleteTestModel(t, []types.StageGroup{custom}, nil, nil)

	cmd := stagesCommand(t, m)
	m = runCommand(t, m, cmd, []string{"delete", "custom-flow"})

	fp, ok := m.rightPane.(stageDeleteFormPane)
	if !ok {
		t.Fatalf("expected rightPane to be a stageDeleteFormPane, got %T", m.rightPane)
	}
	if fp.mode != stageDeleteCustom {
		t.Errorf("mode = %v, want stageDeleteCustom", fp.mode)
	}
}

// TestStagesDeleteRepointsReferencingKinds verifies a group referenced by
// kinds offers repoint mode and lists the referencing kinds — covering both
// a user kind and an untouched baked-in default (Task references task-flow
// without any user override), since KindsReferencingGroup must include both.
func TestStagesDeleteRepointsReferencingKinds(t *testing.T) {
	custom := types.StageGroup{Name: "custom-flow", Stages: []string{"A", "B"}, Cycle: types.CycleTerminate}
	userKind := types.Kind{Name: "Errand", StageGroup: "custom-flow", Glyph: "!"}
	m := newStageDeleteTestModel(t, []types.StageGroup{custom}, []types.Kind{userKind}, nil)

	cmd := stagesCommand(t, m)
	m = runCommand(t, m, cmd, []string{"delete", "custom-flow"})

	fp, ok := m.rightPane.(stageDeleteFormPane)
	if !ok {
		t.Fatalf("expected rightPane to be a stageDeleteFormPane, got %T", m.rightPane)
	}
	if fp.mode != stageDeleteRepoint {
		t.Errorf("mode = %v, want stageDeleteRepoint", fp.mode)
	}
	if len(fp.referenced) != 1 || fp.referenced[0] != "Errand" {
		t.Errorf("referenced = %v, want [Errand]", fp.referenced)
	}
}

// TestStagesDeleteRepointHandsOffToRemap verifies that completing a
// repoint-mode delete (driven directly via stageDeleteSubmitMsg) opens the
// remap form once the fan-out has genuinely rewired kinds.jsonc — here
// simulated by seeding the post-repoint state directly (mirroring
// TestRevertShadowedKindWithDifferentGroupMountsRemapForm's approach): a
// kind now pointing at content-flow while a node still holds a stage only
// valid in the deleted group.
func TestStagesDeleteRepointHandsOffToRemap(t *testing.T) {
	repointed := types.Kind{Name: "Errand", StageGroup: "content-flow", Glyph: "!"}
	m := newStageDeleteTestModel(t, nil, []types.Kind{repointed}, func(s *store.Store) {
		node, err := s.CreateNode("stage only valid in the old group", []string{"task"})
		if err != nil {
			t.Fatalf("CreateNode: %v", err)
		}
		// "Maybe" is valid in task-flow (the deleted group) but absent from
		// content-flow (Active/Reference), the repoint target.
		if _, err := s.UpdateNode(node.ID, map[string]interface{}{"kind": "Errand", "stage": "Maybe"}); err != nil {
			t.Fatalf("UpdateNode: %v", err)
		}
	})

	m2 := driveSubmitMsg(t, m, stageDeleteSubmitMsg{name: "custom-flow", mode: stageDeleteRepoint, repointed: 1})

	if _, ok := m2.rightPane.(remapFormPane); !ok {
		t.Errorf("expected rightPane to be a remapFormPane, got %T", m2.rightPane)
	}
}

// TestStagesDeleteOverCapFallsBackToAdvisory verifies the repoint hand-off
// respects maxRemapOrphans exactly as the kind-edit and stage-edit cascades
// do — falling back to a passive advisory rather than mounting a form with
// more fields than fit any reasonable terminal.
func TestStagesDeleteOverCapFallsBackToAdvisory(t *testing.T) {
	kinds := make([]types.Kind, 0, maxRemapOrphans+1)
	for i := 0; i < maxRemapOrphans+1; i++ {
		kinds = append(kinds, types.Kind{
			Name:       "Kind" + string(rune('A'+i)),
			StageGroup: "content-flow",
			Glyph:      "!",
		})
	}
	m := newStageDeleteTestModel(t, nil, kinds, func(s *store.Store) {
		for i := 0; i < maxRemapOrphans+1; i++ {
			node, err := s.CreateNode("orphan candidate", []string{"task"})
			if err != nil {
				t.Fatalf("CreateNode: %v", err)
			}
			if _, err := s.UpdateNode(node.ID, map[string]interface{}{
				"kind": "Kind" + string(rune('A'+i)), "stage": "Maybe",
			}); err != nil {
				t.Fatalf("UpdateNode: %v", err)
			}
		}
	})

	m2 := driveSubmitMsg(t, m, stageDeleteSubmitMsg{name: "custom-flow", mode: stageDeleteRepoint, repointed: maxRemapOrphans + 1})

	if _, ok := m2.rightPane.(remapFormPane); ok {
		t.Error("expected no remap form mounted when the orphan count exceeds maxRemapOrphans")
	}
}
