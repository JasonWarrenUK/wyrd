package tui

import (
	"errors"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	huh "charm.land/huh/v2"
	"github.com/jasonwarrenuk/wyrd/internal/stage"
	"github.com/jasonwarrenuk/wyrd/internal/types"
)

// ---------------------------------------------------------------------------
// Error-path tests for kindFormPane.Update (StateCompleted branch).
// These live in the internal package so they can manipulate huh.Form.State
// directly, which is the simplest way to exercise the error branches without
// driving a full UI key sequence through the huh state machine. Reuses the
// errStoreFS mock defined in stage_form_internal_test.go.
// ---------------------------------------------------------------------------

// driveKindFormToCompleted pre-populates the form fields on f with the
// standard test values and forces form.State to StateCompleted, then calls
// f.Update with a no-op message so the StateCompleted branch runs. Returns
// the emitted tea.Cmd.
func driveKindFormToCompleted(f kindFormPane) (kindFormPane, tea.Cmd) {
	return driveKindFormToCompletedWith(f, "Errand", "!", "#9b70ff", "task-flow")
}

// driveKindFormToCompletedWith is the parameterised form of
// driveKindFormToCompleted, letting edit-mode tests drive the form to
// completion with arbitrary field values (e.g. re-submitting an existing
// kind's name to exercise the replace-by-name path, or a different name to
// exercise rename).
func driveKindFormToCompletedWith(f kindFormPane, name, glyph, colour, stageGroup string) (kindFormPane, tea.Cmd) {
	f.name = name
	f.glyph = glyph
	f.colour = colour
	f.stageGroup = stageGroup
	f.form.State = huh.StateCompleted
	updated, cmd := f.Update(tea.KeyPressMsg{}) // msg type doesn't matter; State drives the branch
	return updated.(kindFormPane), cmd
}

// TestKindFormErrorOnReadFailure verifies that a ReadKinds failure during
// submit emits kindFormErrorMsg (not formCancelMsg) and aborts the write.
func TestKindFormErrorOnReadFailure(t *testing.T) {
	readErr := errors.New("disk full")
	store := &errKindsStoreFS{readErr: readErr}

	theme, err := LoadTheme(".", "")
	if err != nil {
		t.Fatalf("LoadTheme: %v", err)
	}

	f := newKindFormPane(theme, store, nil, nil, nil)
	_, cmd := driveKindFormToCompleted(f)

	msg := collectMsg(cmd)
	if store.written {
		t.Error("WriteKinds should not be called when ReadKinds fails")
	}
	if batch, ok := msg.(tea.BatchMsg); ok {
		var found bool
		for _, fn := range batch {
			if m := fn(); m != nil {
				if errMsg, ok := m.(kindFormErrorMsg); ok {
					found = true
					if errMsg.err == nil {
						t.Error("kindFormErrorMsg.err should be non-nil")
					}
				}
				if _, ok := m.(formCancelMsg); ok {
					t.Error("formCancelMsg emitted on read failure; expected kindFormErrorMsg")
				}
			}
		}
		if !found {
			t.Error("expected kindFormErrorMsg in Batch, none found")
		}
	} else {
		if _, ok := msg.(kindFormErrorMsg); !ok {
			t.Errorf("expected kindFormErrorMsg, got %T", msg)
		}
	}
}

// TestKindFormErrorOnWriteFailure verifies that a WriteKinds failure during
// submit emits kindFormErrorMsg and does not emit kindFormSubmitMsg.
func TestKindFormErrorOnWriteFailure(t *testing.T) {
	writeErr := errors.New("permission denied")
	store := &errKindsStoreFS{writeErr: writeErr}

	theme, err := LoadTheme(".", "")
	if err != nil {
		t.Fatalf("LoadTheme: %v", err)
	}

	f := newKindFormPane(theme, store, nil, nil, nil)
	_, cmd := driveKindFormToCompleted(f)

	msg := collectMsg(cmd)
	if batch, ok := msg.(tea.BatchMsg); ok {
		var foundErr, foundSubmit bool
		for _, fn := range batch {
			if m := fn(); m != nil {
				if _, ok := m.(kindFormErrorMsg); ok {
					foundErr = true
				}
				if _, ok := m.(kindFormSubmitMsg); ok {
					foundSubmit = true
				}
				if _, ok := m.(formCancelMsg); ok {
					t.Error("formCancelMsg emitted on write failure; expected kindFormErrorMsg")
				}
			}
		}
		if foundSubmit {
			t.Error("kindFormSubmitMsg emitted despite write failure")
		}
		if !foundErr {
			t.Error("expected kindFormErrorMsg in Batch, none found")
		}
	} else {
		if _, ok := msg.(kindFormErrorMsg); !ok {
			t.Errorf("expected kindFormErrorMsg, got %T", msg)
		}
	}
}

// TestKindFormSubmitSuccess verifies a valid submission writes the kind and
// emits kindFormSubmitMsg with the kind's name.
func TestKindFormSubmitSuccess(t *testing.T) {
	store := &errKindsStoreFS{}

	theme, err := LoadTheme(".", "")
	if err != nil {
		t.Fatalf("LoadTheme: %v", err)
	}

	f := newKindFormPane(theme, store, nil, nil, nil)
	_, cmd := driveKindFormToCompleted(f)

	if !store.written {
		t.Error("WriteKinds should have been called on successful submit")
	}
	if len(store.lastWritten) != 1 || store.lastWritten[0].Name != "Errand" {
		t.Errorf("lastWritten = %v, want single kind named Errand", store.lastWritten)
	}

	msg := collectMsg(cmd)
	var found bool
	if batch, ok := msg.(tea.BatchMsg); ok {
		for _, fn := range batch {
			if m := fn(); m != nil {
				if sub, ok := m.(kindFormSubmitMsg); ok {
					found = true
					if sub.name != "Errand" {
						t.Errorf("kindFormSubmitMsg.name = %q, want %q", sub.name, "Errand")
					}
				}
			}
		}
	} else if sub, ok := msg.(kindFormSubmitMsg); ok {
		found = true
		if sub.name != "Errand" {
			t.Errorf("kindFormSubmitMsg.name = %q, want %q", sub.name, "Errand")
		}
	}
	if !found {
		t.Errorf("expected kindFormSubmitMsg, got %T", msg)
	}
}

// TestKindFormDefaults verifies the constructor seeds sensible neutral
// defaults for glyph, colour, and stage group, matching the kindsOverlay
// blank-field fallbacks.
func TestKindFormDefaults(t *testing.T) {
	theme, err := LoadTheme(".", "")
	if err != nil {
		t.Fatalf("LoadTheme: %v", err)
	}
	store := &errKindsStoreFS{}
	groups := types.NewStageGroupRegistry([]types.StageGroup{
		{Name: "task-flow", Stages: []string{"Open", "Done"}, Cycle: types.CycleTerminate},
		{Name: "event-flow", Stages: []string{"Upcoming", "Past"}, Cycle: types.CycleTerminate},
	})

	f := newKindFormPane(theme, store, nil, groups, nil)

	if f.glyph != "·" {
		t.Errorf("default glyph = %q, want %q", f.glyph, "·")
	}
	if f.colour == "" {
		t.Error("default colour should be non-empty")
	}
	if f.stageGroup != "task-flow" {
		t.Errorf("default stage group = %q, want %q (first in registry)", f.stageGroup, "task-flow")
	}
}

// TestKindFormRejectsEmptyStageGroup verifies that submitting with no stage
// group selected (the state left behind when groups is nil or empty, since
// the huh.Select then has no options to seed a default from) is rejected by
// kind.Validate() before any write reaches the store. The stage-group select
// itself also carries a field validator for the same case (see
// validateStageGroup in kind_form.go); this test locks in the belt-and-braces
// fallback that catches it regardless.
func TestKindFormRejectsEmptyStageGroup(t *testing.T) {
	store := &errKindsStoreFS{}

	theme, err := LoadTheme(".", "")
	if err != nil {
		t.Fatalf("LoadTheme: %v", err)
	}

	f := newKindFormPane(theme, store, nil, nil, nil) // nil groups -> f.stageGroup stays ""
	f.name = "Errand"
	f.glyph = "!"
	f.colour = "#9b70ff"
	f.form.State = huh.StateCompleted
	_, cmd := f.Update(tea.KeyPressMsg{})

	if store.written {
		t.Error("WriteKinds should not be called when stage group is empty")
	}

	msg := collectMsg(cmd)
	var found bool
	if batch, ok := msg.(tea.BatchMsg); ok {
		for _, fn := range batch {
			if m := fn(); m != nil {
				if _, ok := m.(kindFormErrorMsg); ok {
					found = true
				}
				if _, ok := m.(kindFormSubmitMsg); ok {
					t.Error("kindFormSubmitMsg emitted despite empty stage group")
				}
			}
		}
	} else if _, ok := msg.(kindFormErrorMsg); ok {
		found = true
	}
	if !found {
		t.Errorf("expected kindFormErrorMsg for empty stage group, got %T", msg)
	}
}

// ---------------------------------------------------------------------------
// upsertKind — pure unit tests, no huh involved.
// ---------------------------------------------------------------------------

func TestUpsertKindCreateAppends(t *testing.T) {
	existing := []types.Kind{{Name: "Task", StageGroup: "task-flow"}}
	kind := types.Kind{Name: "Errand", StageGroup: "task-flow"}

	got := upsertKind(existing, kind, "")

	if len(got) != 2 {
		t.Fatalf("len = %d, want 2", len(got))
	}
	if got[1].Name != "Errand" {
		t.Errorf("appended entry = %v, want Errand", got[1])
	}
}

func TestUpsertKindEditReplacesAtSameIndex(t *testing.T) {
	existing := []types.Kind{
		{Name: "Alpha", StageGroup: "task-flow"},
		{Name: "Errand", StageGroup: "task-flow", Glyph: "!"},
		{Name: "Zeta", StageGroup: "task-flow"},
	}
	edited := types.Kind{Name: "Errand", StageGroup: "task-flow", Glyph: "?"}

	got := upsertKind(existing, edited, "Errand")

	if len(got) != 3 {
		t.Fatalf("len = %d, want 3 (replace, not append)", len(got))
	}
	if got[1].Name != "Errand" || got[1].Glyph != "?" {
		t.Errorf("got[1] = %v, want the edited Errand at the same index", got[1])
	}
	if got[0].Name != "Alpha" || got[2].Name != "Zeta" {
		t.Errorf("order disturbed: got %v", got)
	}
}

func TestUpsertKindEditDefaultNotYetShadowedAppends(t *testing.T) {
	// originalName ("Task") is a baked-in default, not present in the user
	// file — upsertKind can't distinguish this from create; both append,
	// which is exactly the shadowing behaviour SL.16 requires.
	existing := []types.Kind{{Name: "Errand", StageGroup: "task-flow"}}
	edited := types.Kind{Name: "Task", StageGroup: "task-flow", Glyph: "★"}

	got := upsertKind(existing, edited, "Task")

	if len(got) != 2 {
		t.Fatalf("len = %d, want 2 (shadow appended)", len(got))
	}
	if got[1].Name != "Task" || got[1].Glyph != "★" {
		t.Errorf("appended shadow = %v, want edited Task", got[1])
	}
}

func TestUpsertKindEditAlreadyShadowedDefaultReplaces(t *testing.T) {
	existing := []types.Kind{{Name: "Task", StageGroup: "task-flow", Glyph: "★"}}
	edited := types.Kind{Name: "Task", StageGroup: "task-flow", Glyph: "☆"}

	got := upsertKind(existing, edited, "Task")

	if len(got) != 1 {
		t.Fatalf("len = %d, want 1 (replace, not a second shadow)", len(got))
	}
	if got[0].Glyph != "☆" {
		t.Errorf("got[0].Glyph = %q, want %q", got[0].Glyph, "☆")
	}
}

func TestUpsertKindEmptySliceAppends(t *testing.T) {
	edited := types.Kind{Name: "Task", StageGroup: "task-flow"}

	got := upsertKind(nil, edited, "Task")

	if len(got) != 1 || got[0].Name != "Task" {
		t.Errorf("got = %v, want single Task entry", got)
	}
}

// ---------------------------------------------------------------------------
// excludeName — pure unit tests.
// ---------------------------------------------------------------------------

func TestExcludeNameRemovesExactMatch(t *testing.T) {
	got := excludeName([]string{"Task", "task", "Errand"}, "Task")
	want := []string{"task", "Errand"}
	if len(got) != len(want) {
		t.Fatalf("got %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("[%d] = %q, want %q", i, got[i], want[i])
		}
	}
}

func TestExcludeNameCaseSensitive(t *testing.T) {
	// "task" (lowercase) must survive excluding "Task" — removal is exact
	// match, not EqualFold, so a case-only rename still trips the collision
	// validator's EqualFold check afterwards rather than silently coexisting.
	got := excludeName([]string{"task"}, "Task")
	if len(got) != 1 || got[0] != "task" {
		t.Errorf("got %v, want [\"task\"] unchanged", got)
	}
}

// ---------------------------------------------------------------------------
// Edit-mode form-level tests.
// ---------------------------------------------------------------------------

// TestKindEditFormSeedsFields verifies the edit constructor seeds all four
// fields from the existing entry, plus originalName and isDefault.
func TestKindEditFormSeedsFields(t *testing.T) {
	theme, err := LoadTheme(".", "")
	if err != nil {
		t.Fatalf("LoadTheme: %v", err)
	}
	store := &errKindsStoreFS{}
	existing := types.Kind{Name: "Errand", StageGroup: "task-flow", Glyph: "!", Colour: "#123456"}

	f := newKindFormPane(theme, store, nil, nil, &existing)

	if f.name != "Errand" {
		t.Errorf("f.name = %q, want %q", f.name, "Errand")
	}
	if f.glyph != "!" {
		t.Errorf("f.glyph = %q, want %q", f.glyph, "!")
	}
	if f.colour != "#123456" {
		t.Errorf("f.colour = %q, want %q", f.colour, "#123456")
	}
	if f.stageGroup != "task-flow" {
		t.Errorf("f.stageGroup = %q, want %q", f.stageGroup, "task-flow")
	}
	if f.originalName != "Errand" {
		t.Errorf("f.originalName = %q, want %q", f.originalName, "Errand")
	}
}

// TestKindEditFormBlankGlyphColourGetFallbacks verifies a hand-edited entry
// missing glyph/colour (Kind.Validate only requires Name and StageGroup)
// still gets the same neutral fallbacks create mode uses, rather than
// opening the form already failing its own field validators.
func TestKindEditFormBlankGlyphColourGetFallbacks(t *testing.T) {
	theme, err := LoadTheme(".", "")
	if err != nil {
		t.Fatalf("LoadTheme: %v", err)
	}
	store := &errKindsStoreFS{}
	existing := types.Kind{Name: "Errand", StageGroup: "task-flow"} // blank glyph/colour

	f := newKindFormPane(theme, store, nil, nil, &existing)

	if f.glyph != "·" {
		t.Errorf("f.glyph = %q, want fallback %q", f.glyph, "·")
	}
	if f.colour == "" {
		t.Error("f.colour should have received the fallback, got empty")
	}
}

// TestKindEditFormMarksIsDefault verifies isDefault is set when the edited
// kind's name matches a baked-in default.
func TestKindEditFormMarksIsDefault(t *testing.T) {
	theme, err := LoadTheme(".", "")
	if err != nil {
		t.Fatalf("LoadTheme: %v", err)
	}
	store := &errKindsStoreFS{}
	existing := types.Kind{Name: "Task", StageGroup: "task-flow"} // "Task" is a baked-in default

	f := newKindFormPane(theme, store, nil, nil, &existing)

	if !f.isDefault {
		t.Error("expected isDefault = true for a kind named Task (a baked-in default)")
	}
}

// TestKindEditFormNotDefaultForCustomName verifies isDefault stays false for
// a name that isn't a baked-in default.
func TestKindEditFormNotDefaultForCustomName(t *testing.T) {
	theme, err := LoadTheme(".", "")
	if err != nil {
		t.Fatalf("LoadTheme: %v", err)
	}
	store := &errKindsStoreFS{}
	existing := types.Kind{Name: "Errand", StageGroup: "task-flow"}

	f := newKindFormPane(theme, store, nil, nil, &existing)

	if f.isDefault {
		t.Error("expected isDefault = false for a custom kind name")
	}
}

// TestKindEditFormSubmitReplacesExisting verifies submitting in edit mode
// (name unchanged) writes the replaced slice, not an appended one.
func TestKindEditFormSubmitReplacesExisting(t *testing.T) {
	store := &errKindsStoreFS{seed: []types.Kind{
		{Name: "Alpha", StageGroup: "task-flow"},
		{Name: "Errand", StageGroup: "task-flow", Glyph: "!"},
	}}
	theme, err := LoadTheme(".", "")
	if err != nil {
		t.Fatalf("LoadTheme: %v", err)
	}
	existing := types.Kind{Name: "Errand", StageGroup: "task-flow", Glyph: "!"}

	f := newKindFormPane(theme, store, nil, nil, &existing)
	_, cmd := driveKindFormToCompletedWith(f, "Errand", "?", "#9b70ff", "task-flow")
	collectMsg(cmd)

	if !store.written {
		t.Fatal("WriteKinds should have been called")
	}
	if len(store.lastWritten) != 2 {
		t.Fatalf("lastWritten len = %d, want 2 (replace, not append)", len(store.lastWritten))
	}
	if store.lastWritten[0].Name != "Alpha" {
		t.Errorf("lastWritten[0] = %v, want Alpha unchanged", store.lastWritten[0])
	}
	if store.lastWritten[1].Name != "Errand" || store.lastWritten[1].Glyph != "?" {
		t.Errorf("lastWritten[1] = %v, want edited Errand with glyph ?", store.lastWritten[1])
	}
}

// TestKindEditFormRenameEmitsRenamedFrom verifies that submitting a changed
// name in edit mode sets renamedFrom on kindFormSubmitMsg, so the mount
// handler knows to run the RenameKind cascade.
func TestKindEditFormRenameEmitsRenamedFrom(t *testing.T) {
	store := &errKindsStoreFS{seed: []types.Kind{
		{Name: "Errand", StageGroup: "task-flow"},
	}}
	theme, err := LoadTheme(".", "")
	if err != nil {
		t.Fatalf("LoadTheme: %v", err)
	}
	existing := types.Kind{Name: "Errand", StageGroup: "task-flow"}

	f := newKindFormPane(theme, store, nil, nil, &existing)
	_, cmd := driveKindFormToCompletedWith(f, "Chore", "!", "#9b70ff", "task-flow")

	msg := collectMsg(cmd)
	sub := findSubmitMsg(t, msg)
	if sub.name != "Chore" {
		t.Errorf("sub.name = %q, want %q", sub.name, "Chore")
	}
	if sub.renamedFrom != "Errand" {
		t.Errorf("sub.renamedFrom = %q, want %q", sub.renamedFrom, "Errand")
	}

	if len(store.lastWritten) != 1 || store.lastWritten[0].Name != "Chore" {
		t.Errorf("lastWritten = %v, want single Chore entry", store.lastWritten)
	}
}

// TestKindEditFormUnchangedNameNoRename verifies renamedFrom stays empty
// when the name is resubmitted unchanged.
func TestKindEditFormUnchangedNameNoRename(t *testing.T) {
	store := &errKindsStoreFS{seed: []types.Kind{
		{Name: "Errand", StageGroup: "task-flow"},
	}}
	theme, err := LoadTheme(".", "")
	if err != nil {
		t.Fatalf("LoadTheme: %v", err)
	}
	existing := types.Kind{Name: "Errand", StageGroup: "task-flow"}

	f := newKindFormPane(theme, store, nil, nil, &existing)
	_, cmd := driveKindFormToCompletedWith(f, "Errand", "!", "#9b70ff", "task-flow")

	msg := collectMsg(cmd)
	sub := findSubmitMsg(t, msg)
	if sub.renamedFrom != "" {
		t.Errorf("sub.renamedFrom = %q, want empty (name unchanged)", sub.renamedFrom)
	}
}

// TestKindEditFormRenameDefaultWritesTombstone verifies that renaming a
// baked-in default writes both the renamed entry AND a tombstone shadow
// under the old name (an unchanged copy of the default), so the embedded
// default doesn't resurrect under its old name once the registry merges
// baked-in defaults with the user file again.
func TestKindEditFormRenameDefaultWritesTombstone(t *testing.T) {
	store := &errKindsStoreFS{} // "Task" not yet shadowed in the user file
	theme, err := LoadTheme(".", "")
	if err != nil {
		t.Fatalf("LoadTheme: %v", err)
	}
	existing := types.Kind{Name: "Task", StageGroup: "task-flow", Glyph: "◆", Colour: "#9b70ff"}

	f := newKindFormPane(theme, store, nil, nil, &existing)
	if !f.isDefault {
		t.Fatal("precondition failed: Task should be recognised as a default")
	}
	_, cmd := driveKindFormToCompletedWith(f, "Errand", "!", "#123456", "task-flow")
	collectMsg(cmd)

	if len(store.lastWritten) != 2 {
		t.Fatalf("lastWritten len = %d, want 2 (renamed entry + tombstone), got %v", len(store.lastWritten), store.lastWritten)
	}
	var sawRenamed, sawTombstone bool
	for _, k := range store.lastWritten {
		if k.Name == "Errand" {
			sawRenamed = true
		}
		if k.Name == "Task" {
			sawTombstone = true
			// The tombstone must match the embedded default exactly — it
			// exists only to keep the default shadowed, not to diverge.
			if k.Glyph != "◆" {
				t.Errorf("tombstone glyph = %q, want the default's original %q", k.Glyph, "◆")
			}
		}
	}
	if !sawRenamed {
		t.Error("expected the renamed Errand entry in lastWritten")
	}
	if !sawTombstone {
		t.Error("expected a Task tombstone entry in lastWritten so the default stays shadowed")
	}
}

// TestKindEditFormRenameCustomKindNoTombstone verifies that renaming a
// purely custom (non-default) kind does NOT write a tombstone — there's no
// baked-in default to resurrect, so it would just be a stray leftover entry.
func TestKindEditFormRenameCustomKindNoTombstone(t *testing.T) {
	store := &errKindsStoreFS{seed: []types.Kind{
		{Name: "Errand", StageGroup: "task-flow"},
	}}
	theme, err := LoadTheme(".", "")
	if err != nil {
		t.Fatalf("LoadTheme: %v", err)
	}
	existing := types.Kind{Name: "Errand", StageGroup: "task-flow"}

	f := newKindFormPane(theme, store, nil, nil, &existing)
	_, cmd := driveKindFormToCompletedWith(f, "Chore", "!", "#9b70ff", "task-flow")
	collectMsg(cmd)

	if len(store.lastWritten) != 1 {
		t.Fatalf("lastWritten len = %d, want 1 (renamed only, no tombstone), got %v", len(store.lastWritten), store.lastWritten)
	}
	if store.lastWritten[0].Name != "Chore" {
		t.Errorf("lastWritten[0].Name = %q, want %q", store.lastWritten[0].Name, "Chore")
	}
}

// ---------------------------------------------------------------------------
// TD.14 — ShadowOf provenance stamping.
// ---------------------------------------------------------------------------

// TestKindFormCreateLeavesShadowOfEmpty verifies a brand-new, purely
// user-authored kind is never stamped.
func TestKindFormCreateLeavesShadowOfEmpty(t *testing.T) {
	store := &errKindsStoreFS{}
	theme, err := LoadTheme(".", "")
	if err != nil {
		t.Fatalf("LoadTheme: %v", err)
	}

	f := newKindFormPane(theme, store, nil, nil, nil)
	_, cmd := driveKindFormToCompleted(f)
	collectMsg(cmd)

	if len(store.lastWritten) != 1 {
		t.Fatalf("lastWritten len = %d, want 1", len(store.lastWritten))
	}
	if store.lastWritten[0].ShadowOf != "" {
		t.Errorf("ShadowOf = %q, want empty for a create-mode kind", store.lastWritten[0].ShadowOf)
	}
}

// TestKindEditFormStampsShadowOfOnDefault verifies editing a still-unshadowed
// baked-in default stamps ShadowOf with that default's content hash.
func TestKindEditFormStampsShadowOfOnDefault(t *testing.T) {
	store := &errKindsStoreFS{} // "Task" not yet shadowed
	theme, err := LoadTheme(".", "")
	if err != nil {
		t.Fatalf("LoadTheme: %v", err)
	}
	existing := types.Kind{Name: "Task", StageGroup: "task-flow", Glyph: "◆", Colour: "#9b70ff"}

	f := newKindFormPane(theme, store, nil, nil, &existing)
	if !f.isDefault {
		t.Fatal("precondition failed: Task should be recognised as a default")
	}
	_, cmd := driveKindFormToCompletedWith(f, "Task", "◆", "#123456", "task-flow")
	collectMsg(cmd)

	if len(store.lastWritten) != 1 {
		t.Fatalf("lastWritten len = %d, want 1", len(store.lastWritten))
	}
	want := stage.DefaultKindHash("Task")
	if want == "" {
		t.Fatal("precondition failed: DefaultKindHash(Task) should be non-empty")
	}
	if got := store.lastWritten[0].ShadowOf; got != want {
		t.Errorf("ShadowOf = %q, want %q", got, want)
	}
}

// TestKindEditFormNoShadowOfOnCustomKind verifies editing a purely
// user-authored (non-default) kind never stamps ShadowOf.
func TestKindEditFormNoShadowOfOnCustomKind(t *testing.T) {
	store := &errKindsStoreFS{seed: []types.Kind{
		{Name: "Errand", StageGroup: "task-flow"},
	}}
	theme, err := LoadTheme(".", "")
	if err != nil {
		t.Fatalf("LoadTheme: %v", err)
	}
	existing := types.Kind{Name: "Errand", StageGroup: "task-flow"}

	f := newKindFormPane(theme, store, nil, nil, &existing)
	_, cmd := driveKindFormToCompletedWith(f, "Errand", "!", "#9b70ff", "task-flow")
	collectMsg(cmd)

	if len(store.lastWritten) != 1 {
		t.Fatalf("lastWritten len = %d, want 1", len(store.lastWritten))
	}
	if store.lastWritten[0].ShadowOf != "" {
		t.Errorf("ShadowOf = %q, want empty for a custom kind", store.lastWritten[0].ShadowOf)
	}
}

// TestKindEditFormReEditPreservesOriginalShadowOf is the critical regression
// test: re-editing an entry that is already a shadow must carry its existing
// ShadowOf forward unchanged, never recomputing against the current default.
// Recomputing would silently resolve drift the user never reviewed, resetting
// the TD.5 baseline.
func TestKindEditFormReEditPreservesOriginalShadowOf(t *testing.T) {
	sentinel := "sha256:deadbeefdeadbeef"
	existing := types.Kind{Name: "Task", StageGroup: "task-flow", Glyph: "◆", ShadowOf: sentinel}
	store := &errKindsStoreFS{seed: []types.Kind{existing}}
	theme, err := LoadTheme(".", "")
	if err != nil {
		t.Fatalf("LoadTheme: %v", err)
	}

	f := newKindFormPane(theme, store, nil, nil, &existing)
	// Edit an unrelated field (glyph) — the shadow's ShadowOf must not move.
	_, cmd := driveKindFormToCompletedWith(f, "Task", "★", "#9b70ff", "task-flow")
	collectMsg(cmd)

	if len(store.lastWritten) != 1 {
		t.Fatalf("lastWritten len = %d, want 1", len(store.lastWritten))
	}
	got := store.lastWritten[0].ShadowOf
	if got != sentinel {
		t.Errorf("ShadowOf = %q, want preserved sentinel %q (not recomputed against the current default %q)",
			got, sentinel, stage.DefaultKindHash("Task"))
	}
}

// TestKindEditFormReEditPreservesShadowReason covers TD.5's addition
// alongside the ShadowOf invariant above: a re-edit must also carry the
// existing entry's ShadowReason forward unchanged, not reset it to the
// ShadowEdited-equivalent zero value. Losing ShadowReason on re-edit would
// make a tombstone or rename-fan-out shadow start reading as an ordinary
// hand-edit the moment the user touches any unrelated field, defeating
// DetectDiverged's exclusion of those two cases.
func TestKindEditFormReEditPreservesShadowReason(t *testing.T) {
	sentinel := "sha256:deadbeefdeadbeef"
	existing := types.Kind{
		Name: "Talk", StageGroup: "task-flow", Glyph: "◆",
		ShadowOf:     sentinel,
		ShadowReason: types.ShadowRenameFanOut,
	}
	store := &errKindsStoreFS{seed: []types.Kind{existing}}
	theme, err := LoadTheme(".", "")
	if err != nil {
		t.Fatalf("LoadTheme: %v", err)
	}

	f := newKindFormPane(theme, store, nil, nil, &existing)
	_, cmd := driveKindFormToCompletedWith(f, "Talk", "★", "#9b70ff", "task-flow")
	collectMsg(cmd)

	if len(store.lastWritten) != 1 {
		t.Fatalf("lastWritten len = %d, want 1", len(store.lastWritten))
	}
	got := store.lastWritten[0].ShadowReason
	if got != types.ShadowRenameFanOut {
		t.Errorf("ShadowReason = %q, want preserved %q (not reset to ShadowEdited)", got, types.ShadowRenameFanOut)
	}
}

// TestKindEditFormRenameDefaultStampsBothShadowAndTombstone verifies that
// renaming a baked-in default stamps ShadowOf on both the renamed entry and
// the tombstone left under the old name.
func TestKindEditFormRenameDefaultStampsBothShadowAndTombstone(t *testing.T) {
	store := &errKindsStoreFS{} // "Task" not yet shadowed
	theme, err := LoadTheme(".", "")
	if err != nil {
		t.Fatalf("LoadTheme: %v", err)
	}
	existing := types.Kind{Name: "Task", StageGroup: "task-flow", Glyph: "◆", Colour: "#9b70ff"}

	f := newKindFormPane(theme, store, nil, nil, &existing)
	_, cmd := driveKindFormToCompletedWith(f, "Errand", "!", "#123456", "task-flow")
	collectMsg(cmd)

	if len(store.lastWritten) != 2 {
		t.Fatalf("lastWritten len = %d, want 2 (renamed entry + tombstone)", len(store.lastWritten))
	}
	wantHash := stage.DefaultKindHash("Task")
	if wantHash == "" {
		t.Fatal("precondition failed: DefaultKindHash(Task) should be non-empty")
	}
	var sawRenamed, sawTombstone bool
	for _, k := range store.lastWritten {
		if k.Name == "Errand" {
			sawRenamed = true
			if k.ShadowOf != wantHash {
				t.Errorf("renamed entry ShadowOf = %q, want %q", k.ShadowOf, wantHash)
			}
			// TD.5: a fresh fork of a default gets ShadowEdited, not
			// ShadowTombstone — it's the entry the user is actively editing.
			if k.ShadowReason != types.ShadowEdited {
				t.Errorf("renamed entry ShadowReason = %q, want %q", k.ShadowReason, types.ShadowEdited)
			}
		}
		if k.Name == "Task" {
			sawTombstone = true
			if k.ShadowOf != wantHash {
				t.Errorf("tombstone ShadowOf = %q, want %q", k.ShadowOf, wantHash)
			}
			// TD.5: the tombstone is content-identical to the default it
			// shadows, so DetectDiverged must be able to tell it apart from
			// an ordinary hand-edited shadow via ShadowReason.
			if k.ShadowReason != types.ShadowTombstone {
				t.Errorf("tombstone ShadowReason = %q, want %q", k.ShadowReason, types.ShadowTombstone)
			}
		}
	}
	if !sawRenamed {
		t.Error("expected the renamed Errand entry in lastWritten")
	}
	if !sawTombstone {
		t.Error("expected a Task tombstone entry in lastWritten")
	}
}

// TestKindEditFormOwnNameDoesNotCollide verifies that resubmitting a kind's
// own unchanged name passes the collision validator in edit mode.
func TestKindEditFormOwnNameDoesNotCollide(t *testing.T) {
	kinds := types.NewKindRegistry([]types.Kind{
		{Name: "Errand", StageGroup: "task-flow"},
		{Name: "Task", StageGroup: "task-flow"},
	})
	theme, err := LoadTheme(".", "")
	if err != nil {
		t.Fatalf("LoadTheme: %v", err)
	}
	store := &errKindsStoreFS{seed: kinds.All()}
	existing := types.Kind{Name: "Errand", StageGroup: "task-flow"}

	f := newKindFormPane(theme, store, kinds, nil, &existing)
	_, cmd := driveKindFormToCompletedWith(f, "Errand", "!", "#9b70ff", "task-flow")

	msg := collectMsg(cmd)
	if _, ok := findErrorMsg(msg); ok {
		t.Error("resubmitting the kind's own unchanged name should not collide")
	}
}

// TestKindEditFormOtherNameStillCollides verifies edit mode still rejects a
// name belonging to a different existing kind.
func TestKindEditFormOtherNameStillCollides(t *testing.T) {
	kinds := types.NewKindRegistry([]types.Kind{
		{Name: "Errand", StageGroup: "task-flow"},
		{Name: "Task", StageGroup: "task-flow"},
	})
	theme, err := LoadTheme(".", "")
	if err != nil {
		t.Fatalf("LoadTheme: %v", err)
	}
	store := &errKindsStoreFS{seed: kinds.All()}
	existing := types.Kind{Name: "Errand", StageGroup: "task-flow"}

	f := newKindFormPane(theme, store, kinds, nil, &existing)
	_, cmd := driveKindFormToCompletedWith(f, "Task", "!", "#9b70ff", "task-flow")

	msg := collectMsg(cmd)
	if _, ok := findErrorMsg(msg); !ok {
		t.Error("renaming to another existing kind's name should collide")
	}
	if store.written {
		t.Error("WriteKinds should not be called when the new name collides")
	}
}

// TestKindEditFormCaseOnlyRenameRejected verifies the specific
// capitalisation-only error message fires rather than the generic collision
// message, and that the write is aborted either way.
func TestKindEditFormCaseOnlyRenameRejected(t *testing.T) {
	kinds := types.NewKindRegistry([]types.Kind{{Name: "Errand", StageGroup: "task-flow"}})
	theme, err := LoadTheme(".", "")
	if err != nil {
		t.Fatalf("LoadTheme: %v", err)
	}
	store := &errKindsStoreFS{seed: kinds.All()}
	existing := types.Kind{Name: "Errand", StageGroup: "task-flow"}

	f := newKindFormPane(theme, store, kinds, nil, &existing)
	_, cmd := driveKindFormToCompletedWith(f, "errand", "!", "#9b70ff", "task-flow")

	msg := collectMsg(cmd)
	errMsg, ok := findErrorMsg(msg)
	if !ok {
		t.Fatal("expected a validation error for a case-only rename")
	}
	if !strings.Contains(errMsg.err.Error(), "capitalisation") {
		t.Errorf("error = %q, want it to mention capitalisation", errMsg.err.Error())
	}
	if store.written {
		t.Error("WriteKinds should not be called on a rejected case-only rename")
	}
}

// findSubmitMsg unwraps a tea.BatchMsg (or bare msg) looking for a
// kindFormSubmitMsg, failing the test if none is found.
func findSubmitMsg(t *testing.T, msg tea.Msg) kindFormSubmitMsg {
	t.Helper()
	if batch, ok := msg.(tea.BatchMsg); ok {
		for _, fn := range batch {
			if m := fn(); m != nil {
				if sub, ok := m.(kindFormSubmitMsg); ok {
					return sub
				}
			}
		}
	} else if sub, ok := msg.(kindFormSubmitMsg); ok {
		return sub
	}
	t.Fatalf("expected kindFormSubmitMsg, got %T", msg)
	return kindFormSubmitMsg{}
}

// findErrorMsg unwraps a tea.BatchMsg (or bare msg) looking for a
// kindFormErrorMsg.
func findErrorMsg(msg tea.Msg) (kindFormErrorMsg, bool) {
	if batch, ok := msg.(tea.BatchMsg); ok {
		for _, fn := range batch {
			if m := fn(); m != nil {
				if errMsg, ok := m.(kindFormErrorMsg); ok {
					return errMsg, true
				}
			}
		}
		return kindFormErrorMsg{}, false
	}
	errMsg, ok := msg.(kindFormErrorMsg)
	return errMsg, ok
}

// errKindsStoreFS is a minimal StoreFS implementation whose ReadKinds and
// WriteKinds return the configured errors, and whose WriteKinds records
// whether it was called and what was written. seed, when non-nil, is what
// ReadKinds returns instead of an empty registry — edit-mode tests use this
// to exercise replace-by-name against a populated user file.
type errKindsStoreFS struct {
	readErr     error
	writeErr    error
	seed        []types.Kind
	written     bool
	lastWritten []types.Kind
}

func (s *errKindsStoreFS) ReadNode(_ string) (*types.Node, error) { return nil, nil }
func (s *errKindsStoreFS) WriteNode(_ *types.Node) error          { return nil }
func (s *errKindsStoreFS) DeleteEdge(_ string) error              { return nil }
func (s *errKindsStoreFS) WriteEdge(_ *types.Edge) error          { return nil }
func (s *errKindsStoreFS) ReadEdge(_ string) (*types.Edge, error) { return nil, nil }
func (s *errKindsStoreFS) ArchiveNode(_ string) error             { return nil }
func (s *errKindsStoreFS) UpdateNode(_ string, _ map[string]interface{}) (*types.Node, error) {
	return nil, nil
}
func (s *errKindsStoreFS) ReadTemplate(_ string) (*types.Template, error) { return nil, nil }
func (s *errKindsStoreFS) AllTemplates() ([]*types.Template, error)       { return nil, nil }
func (s *errKindsStoreFS) ReadView(_ string) (*types.SavedView, error)    { return nil, nil }
func (s *errKindsStoreFS) AllViews() ([]*types.SavedView, error)          { return nil, nil }
func (s *errKindsStoreFS) ReadRitual(_ string) (*types.Ritual, error)     { return nil, nil }
func (s *errKindsStoreFS) AllRituals() ([]*types.Ritual, error)           { return nil, nil }
func (s *errKindsStoreFS) ReadTheme(_ string) (*types.Theme, error)       { return nil, nil }
func (s *errKindsStoreFS) ReadConfig() (*types.Config, error)             { return nil, nil }
func (s *errKindsStoreFS) WriteConfig(_ *types.Config) error              { return nil }
func (s *errKindsStoreFS) ReadKinds() (*types.KindRegistry, error) {
	if s.readErr != nil {
		return nil, s.readErr
	}
	return types.NewKindRegistry(s.seed), nil
}
func (s *errKindsStoreFS) WriteKinds(kinds []types.Kind) error {
	s.written = true
	s.lastWritten = kinds
	return s.writeErr
}
func (s *errKindsStoreFS) ReadStages() (*types.StageGroupRegistry, error) {
	return types.NewStageGroupRegistry(nil), nil
}
func (s *errKindsStoreFS) WriteStages(_ []types.StageGroup) error { return nil }
func (s *errKindsStoreFS) StorePath() string                      { return "/tmp/err-kinds-store" }
