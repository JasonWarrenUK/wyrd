package stage_test

import (
	"testing"

	"github.com/jasonwarrenuk/wyrd/internal/stage"
	"github.com/jasonwarrenuk/wyrd/internal/types"
)

// TestDetectDivergedCleanRegistryIsEmpty covers the common no-op case: a
// merged registry with no shadowed entries at all reports nothing.
func TestDetectDivergedCleanRegistryIsEmpty(t *testing.T) {
	defaults, err := stage.DefaultKinds()
	if err != nil {
		t.Fatalf("DefaultKinds: %v", err)
	}
	groupDefaults, err := stage.DefaultStageGroups()
	if err != nil {
		t.Fatalf("DefaultStageGroups: %v", err)
	}

	kinds := stage.MergeKinds(defaults, nil)
	groups := stage.MergeStageGroups(groupDefaults, nil)

	report := stage.DetectDiverged(kinds, groups)
	if !report.IsEmpty() {
		t.Errorf("expected an empty report for an all-defaults registry, got %+v", report)
	}
}

// TestDetectDivergedUnchangedShadowNotDiverged covers a fresh, faithful
// fork: ShadowOf matches the current default's hash exactly (as it would
// immediately after kindFormPane stamps it), so nothing is diverged.
func TestDetectDivergedUnchangedShadowNotDiverged(t *testing.T) {
	defaults, err := stage.DefaultKinds()
	if err != nil {
		t.Fatalf("DefaultKinds: %v", err)
	}

	shadow := types.Kind{
		Name: "Task", StageGroup: "task-flow", Glyph: "◆", Colour: "#9b70ff",
		ShadowOf: stage.DefaultKindHash("Task"),
	}
	kinds := stage.MergeKinds(defaults, []types.Kind{shadow})
	groups := stage.MergeStageGroups(nil, nil)

	report := stage.DetectDiverged(kinds, groups)
	if !report.IsEmpty() {
		t.Errorf("expected no divergence for a shadow whose content matches its stamped hash, got %+v", report)
	}
}

// TestDetectDivergedChangedDefaultIsDiverged is the core positive case: a
// shadow forked from an old version of a default, where the embedded
// default's content has since moved on, must be reported. Simulated by
// stamping a ShadowOf that does not match the current DefaultKindHash — the
// same effect an upstream release changing the default would have.
func TestDetectDivergedChangedDefaultIsDiverged(t *testing.T) {
	defaults, err := stage.DefaultKinds()
	if err != nil {
		t.Fatalf("DefaultKinds: %v", err)
	}

	shadow := types.Kind{
		Name: "Task", StageGroup: "task-flow", Glyph: "★", Colour: "#9b70ff",
		ShadowOf: "sha256:0000000000000000", // stale — does not match the current default
	}
	// A second, unrelated shadowed kind stays faithful, to prove the report
	// only flags the one that actually diverged.
	faithful := types.Kind{
		Name: "Goblin", StageGroup: "task-flow", Glyph: "◈", Colour: "#d57300",
		ShadowOf: stage.DefaultKindHash("Goblin"),
	}

	kinds := stage.MergeKinds(defaults, []types.Kind{shadow, faithful})
	groups := stage.MergeStageGroups(nil, nil)

	report := stage.DetectDiverged(kinds, groups)
	if report.SchemaDrift {
		t.Fatal("did not expect SchemaDrift for a single genuine divergence")
	}
	if len(report.Diverged) != 1 {
		t.Fatalf("expected exactly 1 diverged entry, got %d: %+v", len(report.Diverged), report.Diverged)
	}
	if report.Diverged[0].Name != "Task" {
		t.Errorf("diverged entry = %q, want Task", report.Diverged[0].Name)
	}
	if !report.Diverged[0].Kind {
		t.Error("expected Diverged[0].Kind = true for a Kind entry")
	}
}

// TestDetectDivergedTombstoneExcluded covers the tombstone trap: a verbatim
// shadow of a default, stamped ShadowTombstone, whose ShadowOf happens to
// still be stale (as it would be if the default changed after the rename
// that created the tombstone) must NOT be reported — the user never
// consciously edited it, and RenameKind has already moved every node off
// the old name it shadows.
func TestDetectDivergedTombstoneExcluded(t *testing.T) {
	defaults, err := stage.DefaultKinds()
	if err != nil {
		t.Fatalf("DefaultKinds: %v", err)
	}

	tombstone := types.Kind{
		Name: "Task", StageGroup: "task-flow", Glyph: "◆", Colour: "#9b70ff",
		ShadowOf:     "sha256:0000000000000000", // stale, same as the positive case
		ShadowReason: types.ShadowTombstone,
	}
	kinds := stage.MergeKinds(defaults, []types.Kind{tombstone})
	groups := stage.MergeStageGroups(nil, nil)

	report := stage.DetectDiverged(kinds, groups)
	if !report.IsEmpty() {
		t.Errorf("expected a tombstone to be excluded from divergence reporting, got %+v", report)
	}
}

// TestDetectDivergedRenameFanOutExcluded covers the rename-fan-out trap: a
// shadow RenameStageGroup created automatically for a default kind that
// referenced the renamed group, permanently divergent by construction
// (its StageGroup no longer matches the default it forked from), must not
// be reported as ordinary drift the user should review.
func TestDetectDivergedRenameFanOutExcluded(t *testing.T) {
	defaults, err := stage.DefaultKinds()
	if err != nil {
		t.Fatalf("DefaultKinds: %v", err)
	}

	fanOut := types.Kind{
		Name: "Talk", StageGroup: "renamed-task-flow", Glyph: "◆", Colour: "#9b70ff",
		ShadowOf:     stage.DefaultKindHash("Talk"), // hash of the PRISTINE default (old StageGroup)
		ShadowReason: types.ShadowRenameFanOut,
	}
	kinds := stage.MergeKinds(defaults, []types.Kind{fanOut})
	groups := stage.MergeStageGroups(nil, nil)

	report := stage.DetectDiverged(kinds, groups)
	if !report.IsEmpty() {
		t.Errorf("expected a rename-fan-out shadow to be excluded from divergence reporting, got %+v", report)
	}
}

// TestDetectDivergedEmptyShadowReasonTreatedAsEdited covers backward
// compatibility: an entry stamped before ShadowReason existed (empty value)
// must still be reported when genuinely diverged — the default,
// most-common case this whole mechanism exists for.
func TestDetectDivergedEmptyShadowReasonTreatedAsEdited(t *testing.T) {
	defaults, err := stage.DefaultKinds()
	if err != nil {
		t.Fatalf("DefaultKinds: %v", err)
	}

	legacyShadow := types.Kind{
		Name: "Task", StageGroup: "task-flow", Glyph: "★", Colour: "#9b70ff",
		ShadowOf: "sha256:0000000000000000",
		// ShadowReason deliberately left as the zero value.
	}
	kinds := stage.MergeKinds(defaults, []types.Kind{legacyShadow})
	groups := stage.MergeStageGroups(nil, nil)

	report := stage.DetectDiverged(kinds, groups)
	if len(report.Diverged) != 1 {
		t.Fatalf("expected the legacy (empty-reason) shadow to be reported as diverged, got %+v", report)
	}
}

// TestDetectDivergedRenamedAwayNotReported covers the "detached by rename"
// case: an entry whose Name no longer matches any current default (because
// the user renamed their fork, per kindFormPane's documented "deliberate
// detachment" behaviour) has no current hash to compare against and must
// not be reported.
func TestDetectDivergedRenamedAwayNotReported(t *testing.T) {
	defaults, err := stage.DefaultKinds()
	if err != nil {
		t.Fatalf("DefaultKinds: %v", err)
	}

	renamed := types.Kind{
		Name: "MyTask", StageGroup: "task-flow", Glyph: "★", Colour: "#9b70ff",
		ShadowOf: stage.DefaultKindHash("Task"), // stale name reference — "MyTask" has no default
	}
	kinds := stage.MergeKinds(defaults, []types.Kind{renamed})
	groups := stage.MergeStageGroups(nil, nil)

	report := stage.DetectDiverged(kinds, groups)
	if !report.IsEmpty() {
		t.Errorf("expected a renamed-away shadow to be excluded (no current default under its new name), got %+v", report)
	}
}

// TestDetectDivergedSchemaDriftGuard covers the schema-drift trap: when
// every comparable shadowed entry mismatches simultaneously, DetectDiverged
// must report SchemaDrift rather than flooding the user with a divergence
// notice for every customised kind at once — the signature of a
// types.Kind/types.StageGroup field changing, not independent edits.
func TestDetectDivergedSchemaDriftGuard(t *testing.T) {
	defaults, err := stage.DefaultKinds()
	if err != nil {
		t.Fatalf("DefaultKinds: %v", err)
	}

	// Two independently shadowed kinds, both stale against the SAME bogus
	// hash — simulating "every stored hash is wrong at once".
	shadow1 := types.Kind{
		Name: "Task", StageGroup: "task-flow", Glyph: "★", Colour: "#9b70ff",
		ShadowOf: "sha256:0000000000000000",
	}
	shadow2 := types.Kind{
		Name: "Goblin", StageGroup: "task-flow", Glyph: "◈", Colour: "#d57300",
		ShadowOf: "sha256:0000000000000000",
	}
	kinds := stage.MergeKinds(defaults, []types.Kind{shadow1, shadow2})
	groups := stage.MergeStageGroups(nil, nil)

	report := stage.DetectDiverged(kinds, groups)
	if !report.SchemaDrift {
		t.Error("expected SchemaDrift = true when 100% of comparable entries mismatch")
	}
	if len(report.Diverged) != 0 {
		t.Errorf("expected Diverged to be empty under SchemaDrift, got %+v", report.Diverged)
	}
}

// TestDetectDivergedSingleMismatchNotSchemaDrift guards the schema-drift
// guard's lower bound: with only one comparable entry in scope, a mismatch
// cannot be statistically distinguished from a schema change, so
// DetectDiverged must not withhold reporting the single genuine divergence
// — the common case of a user with exactly one customised kind.
func TestDetectDivergedSingleMismatchNotSchemaDrift(t *testing.T) {
	defaults, err := stage.DefaultKinds()
	if err != nil {
		t.Fatalf("DefaultKinds: %v", err)
	}

	shadow := types.Kind{
		Name: "Task", StageGroup: "task-flow", Glyph: "★", Colour: "#9b70ff",
		ShadowOf: "sha256:0000000000000000",
	}
	kinds := stage.MergeKinds(defaults, []types.Kind{shadow})
	groups := stage.MergeStageGroups(nil, nil)

	report := stage.DetectDiverged(kinds, groups)
	if report.SchemaDrift {
		t.Error("did not expect SchemaDrift for a lone comparable entry")
	}
	if len(report.Diverged) != 1 {
		t.Errorf("expected the single divergence to be reported, got %+v", report.Diverged)
	}
}

// TestDetectDivergedStageGroups mirrors the Kind cases for StageGroup, to
// confirm the second half of DetectDiverged behaves identically.
func TestDetectDivergedStageGroups(t *testing.T) {
	defaults, err := stage.DefaultStageGroups()
	if err != nil {
		t.Fatalf("DefaultStageGroups: %v", err)
	}

	diverged := types.StageGroup{
		Name: "task-flow", Stages: []string{"Open", "Doing", "Done", "Archived"}, Cycle: types.CycleTerminate,
		ShadowOf: "sha256:0000000000000000",
	}
	faithful := types.StageGroup{
		Name: "habit-flow", Stages: []string{"Todo", "Done"}, Cycle: types.CycleLoop,
		ShadowOf: stage.DefaultStageGroupHash("habit-flow"),
	}

	kinds := stage.MergeKinds(nil, nil)
	groups := stage.MergeStageGroups(defaults, []types.StageGroup{diverged, faithful})

	report := stage.DetectDiverged(kinds, groups)
	if len(report.Diverged) != 1 {
		t.Fatalf("expected exactly 1 diverged stage group, got %d: %+v", len(report.Diverged), report.Diverged)
	}
	if report.Diverged[0].Name != "task-flow" {
		t.Errorf("diverged entry = %q, want task-flow", report.Diverged[0].Name)
	}
	if report.Diverged[0].Kind {
		t.Error("expected Diverged[0].Kind = false for a StageGroup entry")
	}
}

// TestDetectDivergedNilRegistriesNoPanic covers the defensive nil guard:
// both registries may be nil (e.g. no store configured), and DetectDiverged
// must return an empty report rather than panicking.
func TestDetectDivergedNilRegistriesNoPanic(t *testing.T) {
	report := stage.DetectDiverged(nil, nil)
	if !report.IsEmpty() {
		t.Errorf("expected an empty report for nil registries, got %+v", report)
	}
}

// TestDetectDivergedDeterministicOrder covers ordering: results are sorted
// Kind-before-StageGroup, then by Name — so a rendered advisory or overlay
// row order is stable across runs rather than depending on map iteration.
//
// Also exercises the schema-drift guard's boundary from the other side:
// three comparable entries, but a faithful (matching) one is mixed in, so
// the mismatch rate is under 100% and this must NOT be classified as
// schema drift.
func TestDetectDivergedDeterministicOrder(t *testing.T) {
	defaults, err := stage.DefaultKinds()
	if err != nil {
		t.Fatalf("DefaultKinds: %v", err)
	}
	groupDefaults, err := stage.DefaultStageGroups()
	if err != nil {
		t.Fatalf("DefaultStageGroups: %v", err)
	}

	kindB := types.Kind{Name: "Talk", StageGroup: "task-flow", ShadowOf: "sha256:0000000000000000"}
	kindA := types.Kind{Name: "Goblin", StageGroup: "task-flow", ShadowOf: "sha256:1111111111111111"}
	group := types.StageGroup{Name: "habit-flow", Stages: []string{"Todo", "Done"}, Cycle: types.CycleLoop, ShadowOf: "sha256:2222222222222222"}
	// A faithful shadow keeps the mismatch rate below 100%, so this batch
	// isn't itself mistaken for schema drift.
	faithful := types.Kind{Name: "Project", StageGroup: "project-flow", ShadowOf: stage.DefaultKindHash("Project")}

	kinds := stage.MergeKinds(defaults, []types.Kind{kindB, kindA, faithful})
	groups := stage.MergeStageGroups(groupDefaults, []types.StageGroup{group})

	report := stage.DetectDiverged(kinds, groups)
	if len(report.Diverged) != 3 {
		t.Fatalf("expected 3 diverged entries, got %d: %+v", len(report.Diverged), report.Diverged)
	}
	// Kinds first, alphabetically (Goblin, Talk), then stage groups.
	wantOrder := []string{"Goblin", "Talk", "habit-flow"}
	for i, want := range wantOrder {
		if report.Diverged[i].Name != want {
			t.Errorf("Diverged[%d].Name = %q, want %q", i, report.Diverged[i].Name, want)
		}
	}
}
