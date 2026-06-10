package stage_test

import (
	"testing"

	"github.com/jasonwarrenuk/wyrd/internal/stage"
	"github.com/jasonwarrenuk/wyrd/internal/types"
)

func TestDefaultStageGroupsParse(t *testing.T) {
	groups, err := stage.DefaultStageGroups()
	if err != nil {
		t.Fatalf("DefaultStageGroups() error: %v", err)
	}
	if len(groups) != 5 {
		t.Fatalf("expected 5 groups, got %d", len(groups))
	}
}

func TestDefaultStageGroupsContents(t *testing.T) {
	groups, err := stage.DefaultStageGroups()
	if err != nil {
		t.Fatalf("DefaultStageGroups() error: %v", err)
	}

	byName := make(map[string]types.StageGroup, len(groups))
	for _, g := range groups {
		byName[g.Name] = g
	}

	cases := []struct {
		name   string
		stages []string
		cycle  types.CycleBehaviour
	}{
		{
			name:   "task-flow",
			stages: []string{"Open", "Maybe", "Later", "Soon", "Now", "Done"},
			cycle:  types.CycleTerminate,
		},
		{
			name:   "event-flow",
			stages: []string{"Scheduled", "Now", "Finished"},
			cycle:  types.CycleTerminate,
		},
		{
			name:   "content-flow",
			stages: []string{"Active", "Reference"},
			cycle:  types.CycleTerminate,
		},
		{
			name:   "habit-flow",
			stages: []string{"Todo", "Done"},
			cycle:  types.CycleLoop,
		},
		{
			name:   "project-flow",
			stages: []string{"Planning", "Active", "Done"},
			cycle:  types.CycleTerminate,
		},
	}

	for _, tc := range cases {
		g, ok := byName[tc.name]
		if !ok {
			t.Errorf("group %q not found", tc.name)
			continue
		}
		if g.Cycle != tc.cycle {
			t.Errorf("%s: cycle = %q, want %q", tc.name, g.Cycle, tc.cycle)
		}
		if len(g.Stages) != len(tc.stages) {
			t.Errorf("%s: stages len = %d, want %d", tc.name, len(g.Stages), len(tc.stages))
			continue
		}
		for i, want := range tc.stages {
			if g.Stages[i] != want {
				t.Errorf("%s: stages[%d] = %q, want %q", tc.name, i, g.Stages[i], want)
			}
		}
	}
}

func TestDefaultStageGroupsCycles(t *testing.T) {
	groups, err := stage.DefaultStageGroups()
	if err != nil {
		t.Fatalf("DefaultStageGroups() error: %v", err)
	}

	wantCycle := map[string]types.CycleBehaviour{
		"task-flow":    types.CycleTerminate,
		"event-flow":   types.CycleTerminate,
		"content-flow": types.CycleTerminate,
		"habit-flow":   types.CycleLoop,
		"project-flow": types.CycleTerminate,
	}

	for _, g := range groups {
		want, known := wantCycle[g.Name]
		if !known {
			t.Errorf("unexpected group %q — update test", g.Name)
			continue
		}
		if g.Cycle != want {
			t.Errorf("group %q: cycle = %q, want %q", g.Name, g.Cycle, want)
		}
		if g.LoopTarget != "" {
			t.Errorf("group %q: loop_target = %q, want empty", g.Name, g.LoopTarget)
		}
	}
}

func TestTaskFlowRoundTrip(t *testing.T) {
	groups, err := stage.DefaultStageGroups()
	if err != nil {
		t.Fatalf("DefaultStageGroups() error: %v", err)
	}

	var tf *types.StageGroup
	for i := range groups {
		if groups[i].Name == "task-flow" {
			tf = &groups[i]
			break
		}
	}
	if tf == nil {
		t.Fatal("task-flow group not found")
	}

	if next, ok := tf.Next("Open"); !ok || next != "Maybe" {
		t.Errorf("Next(Open) = (%q, %v), want (Maybe, true)", next, ok)
	}
	if next, ok := tf.Next("Done"); !ok || next != "Done" {
		t.Errorf("Next(Done) = (%q, %v), want (Done, true) — terminate idempotent", next, ok)
	}
	if prev, ok := tf.Prev("Open"); !ok || prev != "Open" {
		t.Errorf("Prev(Open) = (%q, %v), want (Open, true) — terminate at lower bound", prev, ok)
	}
	if !tf.IsTerminal("Done") {
		t.Error("IsTerminal(Done) = false, want true")
	}
	if tf.IsTerminal("Now") {
		t.Error("IsTerminal(Now) = true, want false")
	}
	if next, ok := tf.Next("bogus"); ok || next != "" {
		t.Errorf("Next(bogus) = (%q, %v), want (\"\", false)", next, ok)
	}
}

func TestDefaultStageGroupsDefensiveCopy(t *testing.T) {
	groups1, err := stage.DefaultStageGroups()
	if err != nil {
		t.Fatalf("DefaultStageGroups() error: %v", err)
	}

	// Mutate the returned slice.
	groups1[0].Name = "mutated"

	groups2, err := stage.DefaultStageGroups()
	if err != nil {
		t.Fatalf("DefaultStageGroups() second call error: %v", err)
	}
	for _, g := range groups2 {
		if g.Name == "mutated" {
			t.Error("mutation of returned slice affected the cache")
		}
	}
}
