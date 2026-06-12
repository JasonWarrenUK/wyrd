package tui

import (
	"testing"
	"time"

	"github.com/jasonwarrenuk/wyrd/internal/types"
)

// These tests cover CP.16: edit-mode buildNode must merge into the original
// node rather than rebuilding from scratch, so fields the form doesn't own
// survive an edit. They live in the internal package to reach buildNode.

func internalTestClock() types.Clock {
	return types.StubClock{Fixed: time.Date(2026, 6, 11, 9, 0, 0, 0, time.UTC)}
}

// seedRichNode returns a node carrying every category of data the old
// buildNode used to destroy.
func seedRichNode(nodeTypes ...string) *types.Node {
	created := time.Date(2026, 1, 5, 8, 0, 0, 0, time.UTC)
	due := time.Date(2026, 7, 1, 0, 0, 0, 0, time.UTC)
	about := time.Date(2026, 1, 4, 0, 0, 0, 0, time.UTC)

	n := &types.Node{
		ID:       "11111111-1111-4111-8111-111111111111",
		Title:    "Original title",
		Body:     "Original body",
		Types:    nodeTypes,
		Kind:     "Task",
		Stage:    "Now",
		Created:  created,
		Modified: created,
		Properties: map[string]interface{}{
			"custom_field": "keep me",
		},
		Source: &types.Source{Type: "github", ID: "7"},
	}
	n.Date.Due = &due
	n.Date.About = &about
	return n
}

func TestEditTaskBuildNodePreservesUnownedFields(t *testing.T) {
	original := seedRichNode("task")
	original.Properties["status"] = "inbox"
	original.Properties["energy"] = "low"

	f := newEditTaskFormPane(nil, nil, internalTestClock(), nil, original, nil, nil)
	f.title = "New title"
	f.body = "New body"
	f.status = "active"
	f.energy = "deep"

	got := f.buildNode()

	if got.ID != original.ID {
		t.Errorf("ID = %q, want %q", got.ID, original.ID)
	}
	if !got.Created.Equal(original.Created) {
		t.Errorf("Created = %v, want %v", got.Created, original.Created)
	}
	if !got.Modified.Equal(internalTestClock().Now()) {
		t.Errorf("Modified = %v, want clock now", got.Modified)
	}
	if got.Title != "New title" || got.Body != "New body" {
		t.Errorf("form-owned fields not applied: title=%q body=%q", got.Title, got.Body)
	}
	if got.Properties["status"] != "active" || got.Properties["energy"] != "deep" {
		t.Errorf("status/energy not applied: %v / %v", got.Properties["status"], got.Properties["energy"])
	}

	// Everything the form doesn't own must survive.
	if got.Kind != "Task" || got.Stage != "Now" {
		t.Errorf("kind/stage lost: kind=%q stage=%q", got.Kind, got.Stage)
	}
	if got.Properties["custom_field"] != "keep me" {
		t.Errorf("custom property lost: %v", got.Properties["custom_field"])
	}
	if got.Date.Due == nil || !got.Date.Due.Equal(*original.Date.Due) {
		t.Errorf("Date.Due lost: %v", got.Date.Due)
	}
	if got.Source == nil || got.Source.ID != "7" {
		t.Errorf("Source lost: %v", got.Source)
	}
	if len(got.Types) != 1 || got.Types[0] != "task" {
		t.Errorf("Types changed: %v", got.Types)
	}
}

func TestEditBudgetBuildNodePreservesSpendLog(t *testing.T) {
	original := seedRichNode("budget")
	original.Properties["category"] = "groceries"
	original.Properties["allocated"] = 300.0
	original.Properties["warn_at"] = 0.5
	original.Properties["period"] = "month"
	original.Properties["spend_log"] = []interface{}{
		map[string]interface{}{"date": "2026-06-01", "amount": 25.0},
		map[string]interface{}{"date": "2026-06-08", "amount": 40.0},
	}

	f := newEditBudgetFormPane(nil, nil, internalTestClock(), nil, original, nil, nil)
	f.category = "food"
	f.allocated = "350"
	f.warnAt = "0.9"
	f.period = "week"

	got := f.buildNode()

	log, ok := got.Properties["spend_log"].([]interface{})
	if !ok || len(log) != 2 {
		t.Fatalf("spend_log lost on budget edit: %v", got.Properties["spend_log"])
	}
	if got.Properties["category"] != "food" {
		t.Errorf("category = %v, want food", got.Properties["category"])
	}
	if got.Properties["allocated"] != 350.0 {
		t.Errorf("allocated = %v, want 350", got.Properties["allocated"])
	}
	if got.Properties["warn_at"] != 0.9 {
		t.Errorf("warn_at = %v, want 0.9", got.Properties["warn_at"])
	}
	if got.Properties["period"] != "week" {
		t.Errorf("period = %v, want week", got.Properties["period"])
	}
	if len(got.Types) != 1 || got.Types[0] != "budget" {
		t.Errorf("Types changed: %v", got.Types)
	}
}

func TestEditBudgetWarnAtBlankDefaultsToOne(t *testing.T) {
	original := seedRichNode("budget")
	original.Properties["allocated"] = 100.0

	f := newEditBudgetFormPane(nil, nil, internalTestClock(), nil, original, nil, nil)
	f.category = "misc"
	f.allocated = "100"
	f.warnAt = ""

	got := f.buildNode()
	if got.Properties["warn_at"] != 1.0 {
		t.Errorf("warn_at = %v, want 1 (blank opt-out default)", got.Properties["warn_at"])
	}
}

func TestEditJournalBuildNodePreservesAbout(t *testing.T) {
	original := seedRichNode("journal")
	originalAbout := *original.Date.About

	f := newEditJournalFormPane(nil, nil, internalTestClock(), nil, original, nil, nil)
	f.body = "Edited entry"

	got := f.buildNode()
	if got.Date.About == nil || !got.Date.About.Equal(originalAbout) {
		t.Errorf("journal About moved on edit: %v, want %v", got.Date.About, originalAbout)
	}
}

func TestCreateJournalBuildNodeStampsAbout(t *testing.T) {
	clock := internalTestClock()
	f := newJournalFormPane(nil, nil, clock, "", "", nil, nil)
	f.body = "New entry"

	got := f.buildNode()
	if got.Date.About == nil || !got.Date.About.Equal(clock.Now()) {
		t.Errorf("journal create should stamp About with now: %v", got.Date.About)
	}
}

func TestValidateNonNegativeNumber(t *testing.T) {
	v := validateNonNegativeNumber("allocated")
	if err := v("0"); err != nil {
		t.Errorf("zero should be valid: %v", err)
	}
	if err := v("12.5"); err != nil {
		t.Errorf("positive should be valid: %v", err)
	}
	if err := v(""); err == nil {
		t.Error("blank should be rejected")
	}
	if err := v("-1"); err == nil {
		t.Error("negative should be rejected")
	}
	if err := v("abc"); err == nil {
		t.Error("non-numeric should be rejected")
	}
}

func TestValidateOptionalFraction(t *testing.T) {
	v := validateOptionalFraction("warn_at")
	for _, ok := range []string{"", "0", "0.5", "1"} {
		if err := v(ok); err != nil {
			t.Errorf("%q should be valid: %v", ok, err)
		}
	}
	for _, bad := range []string{"1.5", "-0.1", "abc"} {
		if err := v(bad); err == nil {
			t.Errorf("%q should be rejected", bad)
		}
	}
}

// SL.7a: task creation must stamp Kind and Stage from the selected kind's group.

func TestCreateTaskBuildNodeStampsKindAndStage(t *testing.T) {
	clock := internalTestClock()
	kinds := types.NewKindRegistry([]types.Kind{
		{Name: "Task", StageGroup: "task-flow", Glyph: "◆", Colour: "#9b70ff"},
	})
	groups := types.NewStageGroupRegistry([]types.StageGroup{
		{Name: "task-flow", Stages: []string{"Open", "Maybe", "Later", "Soon", "Now", "Done"}, Cycle: types.CycleTerminate},
	})

	f := newTaskFormPane(nil, nil, clock, "", "Buy milk", kinds, groups)
	got := f.buildNode()

	if got.Kind != "Task" {
		t.Errorf("Kind = %q, want %q", got.Kind, "Task")
	}
	if got.Stage != "Open" {
		t.Errorf("Stage = %q, want %q (first stage of task-flow)", got.Stage, "Open")
	}
}

func TestCreateTaskBuildNodeNilRegistriesLeaveKindStageEmpty(t *testing.T) {
	f := newTaskFormPane(nil, nil, internalTestClock(), "", "Buy milk", nil, nil)
	got := f.buildNode() // must not panic

	if got.Kind != "" {
		t.Errorf("Kind = %q, want empty (nil registry → untriaged)", got.Kind)
	}
	if got.Stage != "" {
		t.Errorf("Stage = %q, want empty (nil registry → untriaged)", got.Stage)
	}
}

// SL.7b: journal/note/budget creation must stamp Kind and Stage.

func TestCreateJournalBuildNodeStampsKindAndStage(t *testing.T) {
	clock := internalTestClock()
	kinds := types.NewKindRegistry([]types.Kind{
		{Name: "Journal", StageGroup: "content-flow", Glyph: "✎", Colour: "#794aff"},
	})
	groups := types.NewStageGroupRegistry([]types.StageGroup{
		{Name: "content-flow", Stages: []string{"Active", "Reference"}, Cycle: types.CycleTerminate},
	})

	f := newJournalFormPane(nil, nil, clock, "", "", kinds, groups)
	got := f.buildNode()

	if got.Kind != "Journal" {
		t.Errorf("Kind = %q, want Journal", got.Kind)
	}
	if got.Stage != "Active" {
		t.Errorf("Stage = %q, want Active (first stage of content-flow)", got.Stage)
	}
}

func TestCreateNoteBuildNodeStampsKindAndStage(t *testing.T) {
	clock := internalTestClock()
	kinds := types.NewKindRegistry([]types.Kind{
		{Name: "Note", StageGroup: "content-flow", Glyph: "▪", Colour: "#009e8c"},
	})
	groups := types.NewStageGroupRegistry([]types.StageGroup{
		{Name: "content-flow", Stages: []string{"Active", "Reference"}, Cycle: types.CycleTerminate},
	})

	f := newNoteFormPane(nil, nil, clock, "", "", kinds, groups)
	got := f.buildNode()

	if got.Kind != "Note" {
		t.Errorf("Kind = %q, want Note", got.Kind)
	}
	if got.Stage != "Active" {
		t.Errorf("Stage = %q, want Active (first stage of content-flow)", got.Stage)
	}
}

func TestCreateBudgetBuildNodeStampsKindAndStage(t *testing.T) {
	clock := internalTestClock()
	kinds := types.NewKindRegistry([]types.Kind{
		{Name: "Budget", StageGroup: "budget-flow", Glyph: "❖", Colour: "#b98300"},
	})
	groups := types.NewStageGroupRegistry([]types.StageGroup{
		{Name: "budget-flow", Stages: []string{"Active", "Closed"}, Cycle: types.CycleTerminate},
	})

	f := newBudgetFormPane(nil, nil, clock, "", "", kinds, groups)
	f.category = "Groceries"
	f.allocated = "300"
	f.warnAt = ""
	got := f.buildNode()

	if got.Kind != "Budget" {
		t.Errorf("Kind = %q, want Budget", got.Kind)
	}
	if got.Stage != "Active" {
		t.Errorf("Stage = %q, want Active (first stage of budget-flow)", got.Stage)
	}
}

func TestCreateBudgetBuildNodeNilRegistriesLeaveKindStageEmpty(t *testing.T) {
	f := newBudgetFormPane(nil, nil, internalTestClock(), "", "", nil, nil)
	f.category = "Misc"
	f.allocated = "100"
	f.warnAt = ""
	got := f.buildNode() // must not panic

	if got.Kind != "" {
		t.Errorf("Kind = %q, want empty (nil registry → untriaged)", got.Kind)
	}
	if got.Stage != "" {
		t.Errorf("Stage = %q, want empty (nil registry → untriaged)", got.Stage)
	}
}

// SL.7c: edit forms — kind/stage stamping rules.

// sharedKindStageRegistries returns a two-kind registry for SL.7c tests:
//   - "Task" → task-flow (stages: Open, Now, Done)
//   - "Note" → content-flow (stages: Active, Reference)
//
// "Now" is present in task-flow but absent from content-flow, exercising the
// stage-reset path when changing kinds.
func sharedKindStageRegistries() (*types.KindRegistry, *types.StageGroupRegistry) {
	kinds := types.NewKindRegistry([]types.Kind{
		{Name: "Task", StageGroup: "task-flow", Glyph: "◆", Colour: "#9b70ff"},
		{Name: "Note", StageGroup: "content-flow", Glyph: "▪", Colour: "#009e8c"},
	})
	groups := types.NewStageGroupRegistry([]types.StageGroup{
		{Name: "task-flow", Stages: []string{"Open", "Now", "Done"}, Cycle: types.CycleTerminate},
		{Name: "content-flow", Stages: []string{"Active", "Reference"}, Cycle: types.CycleTerminate},
	})
	return kinds, groups
}

// TestEditTaskUnchangedKindPreservesStage confirms that leaving the kind select
// on its current value (CP.16) leaves Kind and Stage untouched.
func TestEditTaskUnchangedKindPreservesStage(t *testing.T) {
	original := seedRichNode("task") // Kind="Task", Stage="Now"
	kinds, groups := sharedKindStageRegistries()

	f := newEditTaskFormPane(nil, nil, internalTestClock(), nil, original, kinds, groups)
	// nodeKind pre-populated from node.Kind ("Task"); do not change it.
	f.title = "Updated title"

	got := f.buildNode()

	if got.Kind != "Task" {
		t.Errorf("Kind = %q, want Task (unchanged kind must not alter Kind)", got.Kind)
	}
	if got.Stage != "Now" {
		t.Errorf("Stage = %q, want Now (unchanged kind must not alter Stage)", got.Stage)
	}
}

// TestEditTaskChangedKindStageAbsentResetsToFirst confirms that switching to a
// kind whose group does not contain the current stage resets Stage to the
// group's first stage.
func TestEditTaskChangedKindStageAbsentResetsToFirst(t *testing.T) {
	original := seedRichNode("task") // Kind="Task", Stage="Now"
	kinds, groups := sharedKindStageRegistries()

	f := newEditTaskFormPane(nil, nil, internalTestClock(), nil, original, kinds, groups)
	f.nodeKind = "Note" // content-flow has no "Now"

	got := f.buildNode()

	if got.Kind != "Note" {
		t.Errorf("Kind = %q, want Note", got.Kind)
	}
	if got.Stage != "Active" {
		t.Errorf("Stage = %q, want Active (first stage of content-flow; Now absent)", got.Stage)
	}
}

// TestEditTaskChangedKindStagePresent confirms that switching to a kind whose
// group contains the current stage keeps Stage unchanged.
func TestEditTaskChangedKindStagePresent(t *testing.T) {
	// Build a node with Stage="Active" so content-flow (Active, Reference) keeps it.
	original := seedRichNode("task")
	original.Stage = "Active"

	kinds, groups := sharedKindStageRegistries()

	f := newEditTaskFormPane(nil, nil, internalTestClock(), nil, original, kinds, groups)
	f.nodeKind = "Note" // content-flow contains "Active"

	got := f.buildNode()

	if got.Kind != "Note" {
		t.Errorf("Kind = %q, want Note", got.Kind)
	}
	if got.Stage != "Active" {
		t.Errorf("Stage = %q, want Active (present in content-flow; should be kept)", got.Stage)
	}
}

// TestEditTaskEmptyKindNodeUntriaged confirms that a node with an empty Kind
// and a select left at "" leaves Kind/Stage empty after edit.
func TestEditTaskEmptyKindNodeUntriaged(t *testing.T) {
	original := seedRichNode("task")
	original.Kind = ""
	original.Stage = ""

	kinds, groups := sharedKindStageRegistries()

	f := newEditTaskFormPane(nil, nil, internalTestClock(), nil, original, kinds, groups)
	// nodeKind is "" (set from node.Kind); user did not choose a kind.

	got := f.buildNode()

	if got.Kind != "" {
		t.Errorf("Kind = %q, want empty (untriaged node with unchanged empty kind)", got.Kind)
	}
	if got.Stage != "" {
		t.Errorf("Stage = %q, want empty", got.Stage)
	}
}

// TestEditTaskNilRegistriesPreservesKindStage confirms that passing nil
// registries to an edit form is safe and does not wipe Kind/Stage.
func TestEditTaskNilRegistriesPreservesKindStage(t *testing.T) {
	original := seedRichNode("task") // Kind="Task", Stage="Now"

	f := newEditTaskFormPane(nil, nil, internalTestClock(), nil, original, nil, nil)
	// With nil registries, nodeKind=="Task" but applyKindStage returns early.

	got := f.buildNode()

	if got.Kind != "Task" {
		t.Errorf("Kind = %q, want Task (nil registry must not wipe Kind)", got.Kind)
	}
	if got.Stage != "Now" {
		t.Errorf("Stage = %q, want Now (nil registry must not wipe Stage)", got.Stage)
	}
}
