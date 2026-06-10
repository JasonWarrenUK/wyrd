package types_test

import (
	"testing"

	"github.com/jasonwarrenuk/wyrd/internal/types"
)

func taskFlowGroup() types.StageGroup {
	return types.StageGroup{
		Name:   "task-flow",
		Stages: []string{"Open", "Maybe", "Later", "Soon", "Now", "Done"},
		Cycle:  types.CycleTerminate,
	}
}

func habitFlowGroup() types.StageGroup {
	return types.StageGroup{
		Name:   "habit-flow",
		Stages: []string{"Todo", "Done"},
		Cycle:  types.CycleLoop,
	}
}

// ---- StageGroupRegistry -------------------------------------------------------

func TestStageGroupRegistryLookup(t *testing.T) {
	groups := []types.StageGroup{taskFlowGroup(), habitFlowGroup()}
	reg := types.NewStageGroupRegistry(groups)

	g, ok := reg.Lookup("task-flow")
	if !ok {
		t.Fatal("Lookup(task-flow) ok = false, want true")
	}
	if g.Name != "task-flow" {
		t.Errorf("Lookup(task-flow).Name = %q, want %q", g.Name, "task-flow")
	}
	if len(g.Stages) != 6 {
		t.Errorf("Lookup(task-flow).Stages len = %d, want 6", len(g.Stages))
	}

	_, ok = reg.Lookup("nope")
	if ok {
		t.Error("Lookup(nope) ok = true, want false")
	}
}

func TestStageGroupRegistryOverrideByName(t *testing.T) {
	// Two groups with the same name — last one wins.
	v1 := types.StageGroup{Name: "task-flow", Stages: []string{"A", "B"}, Cycle: types.CycleTerminate}
	v2 := types.StageGroup{Name: "task-flow", Stages: []string{"X", "Y", "Z"}, Cycle: types.CycleLoop}
	reg := types.NewStageGroupRegistry([]types.StageGroup{v1, v2})

	g, ok := reg.Lookup("task-flow")
	if !ok {
		t.Fatal("Lookup(task-flow) ok = false")
	}
	if len(g.Stages) != 3 {
		t.Errorf("last entry should win: Stages len = %d, want 3", len(g.Stages))
	}
	if len(reg.All()) != 1 {
		t.Errorf("len(All()) = %d, want 1 (duplicates should not inflate the list)", len(reg.All()))
	}
}

func TestStageGroupRegistryOrderStable(t *testing.T) {
	names := []string{"habit-flow", "task-flow", "event-flow", "content-flow"}
	groups := make([]types.StageGroup, len(names))
	for i, n := range names {
		groups[i] = types.StageGroup{Name: n, Stages: []string{"A"}, Cycle: types.CycleTerminate}
	}
	reg := types.NewStageGroupRegistry(groups)

	got := reg.Names()
	if len(got) != len(names) {
		t.Fatalf("len(Names()) = %d, want %d", len(got), len(names))
	}
	for i, want := range names {
		if got[i] != want {
			t.Errorf("Names()[%d] = %q, want %q", i, got[i], want)
		}
	}
}

func TestStageGroupRegistryEmpty(t *testing.T) {
	reg := types.NewStageGroupRegistry(nil)

	if all := reg.All(); len(all) != 0 {
		t.Errorf("All() len = %d, want 0", len(all))
	}
	if names := reg.Names(); len(names) != 0 {
		t.Errorf("Names() len = %d, want 0", len(names))
	}
	_, ok := reg.Lookup("anything")
	if ok {
		t.Error("Lookup on empty registry returned ok = true")
	}
}

func TestStageGroupRegistryAllDefensiveCopy(t *testing.T) {
	reg := types.NewStageGroupRegistry([]types.StageGroup{taskFlowGroup()})

	first := reg.All()
	first[0].Name = "Mutated"

	second := reg.All()
	if second[0].Name == "Mutated" {
		t.Error("mutation of All() slice affected registry internals")
	}
}
