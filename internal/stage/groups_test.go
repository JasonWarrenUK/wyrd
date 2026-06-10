package stage_test

import (
	"testing"

	"github.com/jasonwarrenuk/wyrd/internal/stage"
	"github.com/jasonwarrenuk/wyrd/internal/types"
)

func TestMergeStageGroupsUserShadowsDefault(t *testing.T) {
	defaults := []types.StageGroup{
		{Name: "task-flow", Stages: []string{"Open", "Done"}, Cycle: types.CycleTerminate},
		{Name: "habit-flow", Stages: []string{"Todo", "Done"}, Cycle: types.CycleLoop},
	}
	// User overrides task-flow with a different stage list.
	user := []types.StageGroup{
		{Name: "task-flow", Stages: []string{"Backlog", "In Progress", "Done"}, Cycle: types.CycleTerminate},
	}

	reg := stage.MergeStageGroups(defaults, user)

	taskFlow, ok := reg.Lookup("task-flow")
	if !ok {
		t.Fatal("task-flow not found in merged registry")
	}
	if len(taskFlow.Stages) != 3 {
		t.Errorf("user task-flow should have 3 stages, got %d", len(taskFlow.Stages))
	}
	if taskFlow.Stages[0] != "Backlog" {
		t.Errorf("task-flow.Stages[0] = %q, want %q (user should shadow default)", taskFlow.Stages[0], "Backlog")
	}

	habitFlow, ok := reg.Lookup("habit-flow")
	if !ok {
		t.Fatal("habit-flow not found in merged registry")
	}
	if len(habitFlow.Stages) != 2 {
		t.Errorf("unaffected habit-flow should still have 2 stages, got %d", len(habitFlow.Stages))
	}

	names := reg.Names()
	if len(names) != 2 {
		t.Errorf("registry has %d names, want 2 (task-flow, habit-flow)", len(names))
	}
}

func TestMergeStageGroupsNilUser(t *testing.T) {
	defaults := []types.StageGroup{
		{Name: "task-flow", Stages: []string{"Open", "Done"}, Cycle: types.CycleTerminate},
	}
	reg := stage.MergeStageGroups(defaults, nil)

	_, ok := reg.Lookup("task-flow")
	if !ok {
		t.Fatal("task-flow not found when user groups are nil")
	}
	if len(reg.Names()) != 1 {
		t.Errorf("expected 1 group, got %d", len(reg.Names()))
	}
}

func TestMergeStageGroupsWithDefaultStageGroups(t *testing.T) {
	// Integration smoke test: MergeStageGroups(DefaultStageGroups(), nil) produces
	// a registry with all five baked-in groups.
	defaults, err := stage.DefaultStageGroups()
	if err != nil {
		t.Fatalf("DefaultStageGroups() error: %v", err)
	}
	reg := stage.MergeStageGroups(defaults, nil)

	for _, name := range []string{"task-flow", "event-flow", "content-flow", "habit-flow", "project-flow"} {
		if _, ok := reg.Lookup(name); !ok {
			t.Errorf("expected group %q in merged registry", name)
		}
	}
}

func TestMergeStageGroupsInputsUnmutated(t *testing.T) {
	defaults := []types.StageGroup{
		{Name: "task-flow", Stages: []string{"A"}, Cycle: types.CycleTerminate},
	}
	user := []types.StageGroup{
		{Name: "custom", Stages: []string{"B"}, Cycle: types.CycleLoop},
	}

	defaultsCopy := make([]types.StageGroup, len(defaults))
	copy(defaultsCopy, defaults)
	userCopy := make([]types.StageGroup, len(user))
	copy(userCopy, user)

	stage.MergeStageGroups(defaults, user)

	for i, g := range defaults {
		if g.Name != defaultsCopy[i].Name {
			t.Errorf("defaults slice mutated at index %d", i)
		}
	}
	for i, g := range user {
		if g.Name != userCopy[i].Name {
			t.Errorf("user slice mutated at index %d", i)
		}
	}
}
