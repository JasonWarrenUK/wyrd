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

func TestStageGroupValidate(t *testing.T) {
	t.Run("valid terminate group", func(t *testing.T) {
		if err := taskFlow().Validate(); err != nil {
			t.Fatalf("Validate() error = %v; want nil for a well-formed terminate group", err)
		}
	})

	t.Run("valid loop group", func(t *testing.T) {
		if err := habitFlow().Validate(); err != nil {
			t.Fatalf("Validate() error = %v; want nil for a well-formed loop group", err)
		}
	})

	t.Run("valid loop-to-stage group", func(t *testing.T) {
		if err := intakeFlow().Validate(); err != nil {
			t.Fatalf("Validate() error = %v; want nil for a well-formed loop-to-stage group", err)
		}
	})

	t.Run("empty name", func(t *testing.T) {
		g := taskFlow()
		g.Name = ""
		err := g.Validate()
		if err == nil {
			t.Fatal("Validate() = nil; want error for empty name")
		}
		var ve *ValidationError
		if !asValidationError(err, &ve) {
			t.Fatalf("error type = %T; want *ValidationError", err)
		}
		if ve.Field != "name" {
			t.Errorf("ValidationError.Field = %q; want %q", ve.Field, "name")
		}
	})

	t.Run("zero stages", func(t *testing.T) {
		g := StageGroup{Name: "empty-flow", Stages: nil, Cycle: CycleTerminate}
		err := g.Validate()
		if err == nil {
			t.Fatal("Validate() = nil; want error for nil stages")
		}
		var ve *ValidationError
		if !asValidationError(err, &ve) {
			t.Fatalf("error type = %T; want *ValidationError", err)
		}
		if ve.Field != "stages" {
			t.Errorf("ValidationError.Field = %q; want %q", ve.Field, "stages")
		}
	})

	t.Run("loop-to-stage with present target", func(t *testing.T) {
		g := intakeFlow() // LoopTarget = "Active", which is in Stages
		if err := g.Validate(); err != nil {
			t.Fatalf("Validate() error = %v; want nil when loop_target is present in stages", err)
		}
	})

	t.Run("loop-to-stage with absent target", func(t *testing.T) {
		g := intakeFlow()
		g.LoopTarget = "Nonexistent"
		err := g.Validate()
		if err == nil {
			t.Fatal("Validate() = nil; want error when loop_target names a stage absent from stages")
		}
		var ve *ValidationError
		if !asValidationError(err, &ve) {
			t.Fatalf("error type = %T; want *ValidationError", err)
		}
		if ve.Field != "loop_target" {
			t.Errorf("ValidationError.Field = %q; want %q", ve.Field, "loop_target")
		}
	})

	t.Run("loop-to-stage with empty target", func(t *testing.T) {
		g := intakeFlow()
		g.LoopTarget = ""
		if err := g.Validate(); err == nil {
			t.Fatal("Validate() = nil; want error when loop_target is empty for loop-to-stage group")
		}
	})

	t.Run("terminate group ignores loop_target", func(t *testing.T) {
		// A terminate group with a nonsense loop_target should still pass —
		// Validate only checks loop_target for CycleLoopToStage groups.
		g := taskFlow()
		g.LoopTarget = "Nonexistent"
		if err := g.Validate(); err != nil {
			t.Fatalf("Validate() error = %v; want nil (loop_target irrelevant for terminate group)", err)
		}
	})
}

// asValidationError is a helper that avoids an errors import in the test file.
// It checks whether err wraps a *ValidationError and, if so, sets *target.
func asValidationError(err error, target **ValidationError) bool {
	e, ok := err.(*ValidationError)
	if ok {
		*target = e
	}
	return ok
}

func TestContains(t *testing.T) {
	task := taskFlow() // stages: Open, Maybe, Later, Soon, Now, Done

	if !task.Contains("Open") {
		t.Error("Contains(Open) = false; want true (first stage)")
	}
	if !task.Contains("Done") {
		t.Error("Contains(Done) = false; want true (last stage)")
	}
	if !task.Contains("Now") {
		t.Error("Contains(Now) = false; want true (mid stage)")
	}
	if task.Contains("Bogus") {
		t.Error("Contains(Bogus) = true; want false (absent stage)")
	}
	if task.Contains("") {
		t.Error("Contains(\"\") = true; want false (empty string not a stage)")
	}

	empty := StageGroup{Name: "empty", Stages: []string{}}
	if empty.Contains("Open") {
		t.Error("Contains on empty group = true; want false")
	}
}
