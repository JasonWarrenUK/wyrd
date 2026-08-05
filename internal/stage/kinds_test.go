package stage_test

import (
	"testing"

	"github.com/jasonwarrenuk/wyrd/internal/stage"
	"github.com/jasonwarrenuk/wyrd/internal/types"
)

func TestDefaultKindsParse(t *testing.T) {
	kinds, err := stage.DefaultKinds()
	if err != nil {
		t.Fatalf("DefaultKinds() error: %v", err)
	}
	if len(kinds) != 11 {
		t.Fatalf("expected 11 kinds, got %d", len(kinds))
	}
}

func TestDefaultKindsContents(t *testing.T) {
	kinds, err := stage.DefaultKinds()
	if err != nil {
		t.Fatalf("DefaultKinds() error: %v", err)
	}

	byName := make(map[string]types.Kind, len(kinds))
	for _, k := range kinds {
		byName[k.Name] = k
	}

	cases := []struct {
		name       string
		stageGroup string
	}{
		{name: "Task", stageGroup: "task-flow"},
		{name: "Goblin", stageGroup: "task-flow"},
		{name: "Habit", stageGroup: "habit-flow"},
		{name: "Event", stageGroup: "event-flow"},
		{name: "Travel", stageGroup: "event-flow"},
		{name: "Talk", stageGroup: "task-flow"},
		{name: "Project", stageGroup: "project-flow"},
		{name: "Journal", stageGroup: "content-flow"},
		{name: "Note", stageGroup: "content-flow"},
		{name: "Budget", stageGroup: "budget-flow"},
		{name: "Movement", stageGroup: "movement-flow"},
	}

	for _, tc := range cases {
		k, ok := byName[tc.name]
		if !ok {
			t.Errorf("kind %q not found", tc.name)
			continue
		}
		if k.StageGroup != tc.stageGroup {
			t.Errorf("%s: stage_group = %q, want %q", tc.name, k.StageGroup, tc.stageGroup)
		}
	}
}

func TestDefaultKindsValidate(t *testing.T) {
	kinds, err := stage.DefaultKinds()
	if err != nil {
		t.Fatalf("DefaultKinds() error: %v", err)
	}
	for _, k := range kinds {
		if err := k.Validate(); err != nil {
			t.Errorf("kind %q failed Validate(): %v", k.Name, err)
		}
	}
}

func TestDefaultKindsReferToRealGroups(t *testing.T) {
	kinds, err := stage.DefaultKinds()
	if err != nil {
		t.Fatalf("DefaultKinds() error: %v", err)
	}
	groups, err := stage.DefaultStageGroups()
	if err != nil {
		t.Fatalf("DefaultStageGroups() error: %v", err)
	}

	groupSet := make(map[string]struct{}, len(groups))
	for _, g := range groups {
		groupSet[g.Name] = struct{}{}
	}

	for _, k := range kinds {
		if _, ok := groupSet[k.StageGroup]; !ok {
			t.Errorf("kind %q references stage group %q which is not in DefaultStageGroups()", k.Name, k.StageGroup)
		}
	}
}

func TestDefaultKindsDefensiveCopy(t *testing.T) {
	kinds1, err := stage.DefaultKinds()
	if err != nil {
		t.Fatalf("DefaultKinds() error: %v", err)
	}

	// Mutate the returned slice.
	kinds1[0].Name = "mutated"

	kinds2, err := stage.DefaultKinds()
	if err != nil {
		t.Fatalf("DefaultKinds() second call error: %v", err)
	}
	for _, k := range kinds2 {
		if k.Name == "mutated" {
			t.Error("mutation of returned slice affected the cache")
		}
	}
}

func TestMergeKindsUserShadowsDefault(t *testing.T) {
	defaults := []types.Kind{
		{Name: "Task", StageGroup: "task-flow", Glyph: "◆", Colour: "#9b70ff"},
		{Name: "Goblin", StageGroup: "task-flow", Glyph: "◈", Colour: "#d57300"},
	}
	user := []types.Kind{
		// Shadows the default Task; user has changed the stage group.
		{Name: "Task", StageGroup: "event-flow", Glyph: "X", Colour: "#ffffff"},
		// A wholly new kind not in defaults.
		{Name: "Ritual", StageGroup: "habit-flow", Glyph: "✦", Colour: "#009e8c"},
	}

	reg := stage.MergeKinds(defaults, user)

	task, ok := reg.Lookup("Task")
	if !ok {
		t.Fatal("Task not found in merged registry")
	}
	if task.StageGroup != "event-flow" {
		t.Errorf("Task.StageGroup = %q, want event-flow (user should shadow default)", task.StageGroup)
	}

	goblin, ok := reg.Lookup("Goblin")
	if !ok {
		t.Fatal("Goblin not found in merged registry")
	}
	if goblin.StageGroup != "task-flow" {
		t.Errorf("Goblin.StageGroup = %q, want task-flow (unaffected default)", goblin.StageGroup)
	}

	ritual, ok := reg.Lookup("Ritual")
	if !ok {
		t.Fatal("Ritual (user-only kind) not found in merged registry")
	}
	if ritual.StageGroup != "habit-flow" {
		t.Errorf("Ritual.StageGroup = %q, want habit-flow", ritual.StageGroup)
	}

	names := reg.Names()
	if len(names) != 3 {
		t.Errorf("registry has %d names, want 3 (Task, Goblin, Ritual)", len(names))
	}
}

func TestMergeKindsInputsUnmutated(t *testing.T) {
	defaults := []types.Kind{
		{Name: "Task", StageGroup: "task-flow", Glyph: "◆", Colour: "#9b70ff"},
	}
	user := []types.Kind{
		{Name: "Custom", StageGroup: "habit-flow", Glyph: "✦", Colour: "#009e8c"},
	}

	defaultsCopy := make([]types.Kind, len(defaults))
	copy(defaultsCopy, defaults)
	userCopy := make([]types.Kind, len(user))
	copy(userCopy, user)

	stage.MergeKinds(defaults, user)

	for i, k := range defaults {
		if k.Name != defaultsCopy[i].Name || k.StageGroup != defaultsCopy[i].StageGroup {
			t.Errorf("defaults slice mutated at index %d: got %+v, want %+v", i, k, defaultsCopy[i])
		}
	}
	for i, k := range user {
		if k.Name != userCopy[i].Name || k.StageGroup != userCopy[i].StageGroup {
			t.Errorf("user slice mutated at index %d: got %+v, want %+v", i, k, userCopy[i])
		}
	}
}
