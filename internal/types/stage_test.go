package types

import "testing"

// taskFlow is the terminating task progression from SL.3.
func taskFlow() StageGroup {
	return StageGroup{
		Name:   "task-flow",
		Stages: []string{"Open", "Maybe", "Later", "Soon", "Now", "Done"},
		Cycle:  CycleTerminate,
	}
}

// habitFlow is a looping progression with no terminal stage.
func habitFlow() StageGroup {
	return StageGroup{
		Name:   "habit-flow",
		Stages: []string{"Pending", "Done"},
		Cycle:  CycleLoop,
	}
}

// intakeFlow loops back to a stage partway through, skipping an intake stage.
func intakeFlow() StageGroup {
	return StageGroup{
		Name:       "intake-flow",
		Stages:     []string{"Inbox", "Active", "Resting", "Done"},
		Cycle:      CycleLoopToStage,
		LoopTarget: "Active",
	}
}

func TestNextAdvancesThroughStages(t *testing.T) {
	g := taskFlow()
	got, ok := g.Next("Open")
	if !ok || got != "Maybe" {
		t.Fatalf("Next(Open) = %q, %v; want Maybe, true", got, ok)
	}
	got, ok = g.Next("Now")
	if !ok || got != "Done" {
		t.Fatalf("Next(Now) = %q, %v; want Done, true", got, ok)
	}
}

func TestNextTerminateStaysAtLast(t *testing.T) {
	g := taskFlow()
	got, ok := g.Next("Done")
	if !ok || got != "Done" {
		t.Fatalf("Next(Done) = %q, %v; want Done, true (idempotent)", got, ok)
	}
}

func TestNextLoopWrapsToFirst(t *testing.T) {
	g := habitFlow()
	got, ok := g.Next("Done")
	if !ok || got != "Pending" {
		t.Fatalf("Next(Done) = %q, %v; want Pending, true", got, ok)
	}
}

func TestNextLoopToStageWrapsToTarget(t *testing.T) {
	g := intakeFlow()
	got, ok := g.Next("Done")
	if !ok || got != "Active" {
		t.Fatalf("Next(Done) = %q, %v; want Active, true", got, ok)
	}
}

func TestNextLoopToStageFallsBackToFirst(t *testing.T) {
	g := intakeFlow()
	g.LoopTarget = "Nonexistent"
	got, ok := g.Next("Done")
	if !ok || got != "Inbox" {
		t.Fatalf("Next(Done) with bad LoopTarget = %q, %v; want Inbox, true", got, ok)
	}
}

func TestNextUnknownStageReturnsFalse(t *testing.T) {
	g := taskFlow()
	got, ok := g.Next("Bogus")
	if ok || got != "" {
		t.Fatalf("Next(Bogus) = %q, %v; want \"\", false", got, ok)
	}
}

func TestPrevRetreatsThroughStages(t *testing.T) {
	g := taskFlow()
	got, ok := g.Prev("Done")
	if !ok || got != "Now" {
		t.Fatalf("Prev(Done) = %q, %v; want Now, true", got, ok)
	}
}

func TestPrevTerminateStaysAtFirst(t *testing.T) {
	g := taskFlow()
	got, ok := g.Prev("Open")
	if !ok || got != "Open" {
		t.Fatalf("Prev(Open) = %q, %v; want Open, true (idempotent)", got, ok)
	}
}

func TestPrevLoopWrapsToLast(t *testing.T) {
	g := habitFlow()
	got, ok := g.Prev("Pending")
	if !ok || got != "Done" {
		t.Fatalf("Prev(Pending) = %q, %v; want Done, true", got, ok)
	}
}

func TestPrevLoopToStageWrapsToLast(t *testing.T) {
	g := intakeFlow()
	got, ok := g.Prev("Inbox")
	if !ok || got != "Done" {
		t.Fatalf("Prev(Inbox) = %q, %v; want Done, true", got, ok)
	}
}

func TestPrevUnknownStageReturnsFalse(t *testing.T) {
	g := taskFlow()
	got, ok := g.Prev("Bogus")
	if ok || got != "" {
		t.Fatalf("Prev(Bogus) = %q, %v; want \"\", false", got, ok)
	}
}

func TestIsTerminal(t *testing.T) {
	task := taskFlow()
	if !task.IsTerminal("Done") {
		t.Error("IsTerminal(Done) = false; want true for terminating group's last stage")
	}
	if task.IsTerminal("Now") {
		t.Error("IsTerminal(Now) = true; want false for non-last stage")
	}
	if task.IsTerminal("Bogus") {
		t.Error("IsTerminal(Bogus) = true; want false for unknown stage")
	}

	// Looping groups have no terminal stage, even at the last position.
	habit := habitFlow()
	if habit.IsTerminal("Done") {
		t.Error("IsTerminal(Done) = true; want false for looping group")
	}
}
