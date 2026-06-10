package types_test

import (
	"testing"

	"github.com/jasonwarrenuk/wyrd/internal/types"
)

func testKindsReg() *types.KindRegistry {
	return types.NewKindRegistry([]types.Kind{
		{Name: "Task", StageGroup: "task-flow"},
		{Name: "Habit", StageGroup: "habit-flow"},
	})
}

func testGroupsReg() *types.StageGroupRegistry {
	return types.NewStageGroupRegistry([]types.StageGroup{
		{Name: "task-flow", Stages: []string{"Open", "Done"}, Cycle: types.CycleTerminate},
		{Name: "habit-flow", Stages: []string{"Todo", "Done"}, Cycle: types.CycleLoop},
	})
}

func TestResolveStageGroup_HappyPath(t *testing.T) {
	node := &types.Node{Kind: "Task", Stage: "Open"}
	g, ok := types.ResolveStageGroup(testKindsReg(), testGroupsReg(), node)
	if !ok {
		t.Fatal("ResolveStageGroup ok = false, want true")
	}
	if g.Name != "task-flow" {
		t.Errorf("group name = %q, want %q", g.Name, "task-flow")
	}
}

func TestResolveStageGroup_EmptyKind(t *testing.T) {
	node := &types.Node{Kind: "", Stage: "Open"}
	_, ok := types.ResolveStageGroup(testKindsReg(), testGroupsReg(), node)
	if ok {
		t.Error("expected ok=false for empty kind, got true")
	}
}

func TestResolveStageGroup_UnknownKind(t *testing.T) {
	node := &types.Node{Kind: "Goblin", Stage: "Open"}
	_, ok := types.ResolveStageGroup(testKindsReg(), testGroupsReg(), node)
	if ok {
		t.Error("expected ok=false for unknown kind, got true")
	}
}

func TestResolveStageGroup_UnknownGroupName(t *testing.T) {
	// Kind resolves but references a group not in the registry.
	kinds := types.NewKindRegistry([]types.Kind{
		{Name: "Task", StageGroup: "no-such-flow"},
	})
	node := &types.Node{Kind: "Task", Stage: "Open"}
	_, ok := types.ResolveStageGroup(kinds, testGroupsReg(), node)
	if ok {
		t.Error("expected ok=false for unknown group name, got true")
	}
}

func TestResolveStageGroup_NilNode(t *testing.T) {
	_, ok := types.ResolveStageGroup(testKindsReg(), testGroupsReg(), nil)
	if ok {
		t.Error("expected ok=false for nil node, got true")
	}
}

func TestResolveStageGroup_NilKindRegistry(t *testing.T) {
	node := &types.Node{Kind: "Task", Stage: "Open"}
	_, ok := types.ResolveStageGroup(nil, testGroupsReg(), node)
	if ok {
		t.Error("expected ok=false for nil kind registry, got true")
	}
}

func TestResolveStageGroup_NilGroupRegistry(t *testing.T) {
	node := &types.Node{Kind: "Task", Stage: "Open"}
	_, ok := types.ResolveStageGroup(testKindsReg(), nil, node)
	if ok {
		t.Error("expected ok=false for nil group registry, got true")
	}
}
