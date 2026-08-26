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
	// shadow (above) carries no ShadowSource, matching every shadow stamped
	// before TD.18b existed — OldKind must be nil, not a zero-value struct,
	// so TD.18's combine form can tell "no snapshot available" apart from
	// "the snapshot is the zero Kind" and degrade to a two-way flow for
	// this entry rather than rendering nonsense.
	if report.Diverged[0].OldKind != nil {
		t.Errorf("expected OldKind = nil for a shadow with no ShadowSource, got %+v", report.Diverged[0].OldKind)
	}
}

// TestDetectDivergedCarriesOldKindFromShadowSource covers TD.18b: a diverged
// entry whose shadow carries a ShadowSource snapshot surfaces it as OldKind,
// unmodified, so TD.18's combine form doesn't need a second lookup.
func TestDetectDivergedCarriesOldKindFromShadowSource(t *testing.T) {
	defaults, err := stage.DefaultKinds()
	if err != nil {
		t.Fatalf("DefaultKinds: %v", err)
	}

	snapshot := &types.Kind{Name: "Task", StageGroup: "task-flow", Glyph: "●", Colour: "#7755ee"}
	shadow := types.Kind{
		Name: "Task", StageGroup: "task-flow", Glyph: "★", Colour: "#9b70ff",
		ShadowOf:     "sha256:0000000000000000",
		ShadowSource: snapshot,
	}

	kinds := stage.MergeKinds(defaults, []types.Kind{shadow})
	groups := stage.MergeStageGroups(nil, nil)

	report := stage.DetectDiverged(kinds, groups)
	if len(report.Diverged) != 1 {
		t.Fatalf("expected exactly 1 diverged entry, got %d: %+v", len(report.Diverged), report.Diverged)
	}
	if report.Diverged[0].OldKind == nil {
		t.Fatal("expected OldKind to be populated from ShadowSource")
	}
	if *report.Diverged[0].OldKind != *snapshot {
		t.Errorf("OldKind = %+v, want %+v", *report.Diverged[0].OldKind, *snapshot)
	}
	if report.Diverged[0].OldGroup != nil {
		t.Errorf("expected OldGroup = nil for a Kind entry, got %+v", report.Diverged[0].OldGroup)
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

// TestDetectDivergedCarriesOldGroupFromShadowSource is
// TestDetectDivergedCarriesOldKindFromShadowSource's StageGroup mirror
// (TD.18b): a diverged StageGroup whose shadow carries a ShadowSource
// snapshot surfaces it as OldGroup, unmodified.
func TestDetectDivergedCarriesOldGroupFromShadowSource(t *testing.T) {
	defaults, err := stage.DefaultStageGroups()
	if err != nil {
		t.Fatalf("DefaultStageGroups: %v", err)
	}

	snapshot := &types.StageGroup{Name: "task-flow", Stages: []string{"Open", "Done"}, Cycle: types.CycleTerminate}
	diverged := types.StageGroup{
		Name: "task-flow", Stages: []string{"Open", "Doing", "Done", "Archived"}, Cycle: types.CycleTerminate,
		ShadowOf:     "sha256:0000000000000000",
		ShadowSource: snapshot,
	}

	kinds := stage.MergeKinds(nil, nil)
	groups := stage.MergeStageGroups(defaults, []types.StageGroup{diverged})

	report := stage.DetectDiverged(kinds, groups)
	if len(report.Diverged) != 1 {
		t.Fatalf("expected exactly 1 diverged entry, got %d: %+v", len(report.Diverged), report.Diverged)
	}
	if report.Diverged[0].OldGroup == nil {
		t.Fatal("expected OldGroup to be populated from ShadowSource")
	}
	got := report.Diverged[0].OldGroup
	if got.Name != snapshot.Name || got.Cycle != snapshot.Cycle || len(got.Stages) != len(snapshot.Stages) {
		t.Errorf("OldGroup = %+v, want %+v", *got, *snapshot)
	}
	if report.Diverged[0].OldKind != nil {
		t.Errorf("expected OldKind = nil for a StageGroup entry, got %+v", report.Diverged[0].OldKind)
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

// TestDetectDivergedNilKindsChangesSchemaDriftOutcome pins the exact hazard
// that used to make stagesOverlay and kindsOverlay disagree on identical
// on-disk state: checked/mismatched are pooled ACROSS both registries, so
// passing nil for kinds does not just omit kinds from the result — it
// changes the denominator the schema-drift guard divides by, and can flip
// SchemaDrift on or off relative to a call that passes both registries.
//
// This is why the fix (TD.5 follow-up) is at the CALL SITES — both TUI
// overlays now read a single stage.DivergenceReport computed once with both
// registries (see Model.divergence) rather than each recomputing its own —
// not in DetectDiverged itself, whose behaviour here is working as
// documented (see its "pooled across kinds and groups" framing) but is easy
// to call unsafely.
func TestDetectDivergedNilKindsChangesSchemaDriftOutcome(t *testing.T) {
	groupDefaults, err := stage.DefaultStageGroups()
	if err != nil {
		t.Fatalf("DefaultStageGroups: %v", err)
	}
	defaults, err := stage.DefaultKinds()
	if err != nil {
		t.Fatalf("DefaultKinds: %v", err)
	}

	// Two independently shadowed stage groups, both mismatching — 100% of
	// the groups-only comparable set.
	group1 := types.StageGroup{
		Name: "task-flow", Stages: []string{"Open", "Done"}, Cycle: types.CycleTerminate,
		ShadowOf: "sha256:0000000000000000",
	}
	group2 := types.StageGroup{
		Name: "habit-flow", Stages: []string{"Todo", "Done"}, Cycle: types.CycleLoop,
		ShadowOf: "sha256:0000000000000000",
	}
	groups := stage.MergeStageGroups(groupDefaults, []types.StageGroup{group1, group2})

	// A faithfully-shadowed kind, included only when kinds is non-nil. It
	// matches its default exactly, so it contributes to `checked` without
	// contributing to `mismatched` — enough on its own to pull the combined
	// mismatch rate under 100%.
	faithfulKind := types.Kind{
		Name: "Task", StageGroup: "task-flow", Glyph: "◆", Colour: "#9b70ff",
		ShadowOf: stage.DefaultKindHash("Task"),
	}
	kinds := stage.MergeKinds(defaults, []types.Kind{faithfulKind})

	groupsOnly := stage.DetectDiverged(nil, groups)
	both := stage.DetectDiverged(kinds, groups)

	if !groupsOnly.SchemaDrift {
		t.Error("expected SchemaDrift = true when kinds is nil (2 checked, 2 mismatched, 100%)")
	}
	if len(groupsOnly.Diverged) != 0 {
		t.Errorf("expected Diverged empty under SchemaDrift (nil kinds), got %+v", groupsOnly.Diverged)
	}

	if both.SchemaDrift {
		t.Error("expected SchemaDrift = false once the faithful kind is included (3 checked, 2 mismatched)")
	}
	if len(both.Diverged) != 2 {
		t.Errorf("expected both diverged stage groups reported once kinds is included, got %+v", both.Diverged)
	}
}
