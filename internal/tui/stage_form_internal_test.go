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

// TestParseStages verifies the parseStages helper splits, trims, and drops
// blank lines correctly. Order must be preserved.
func TestParseStages(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  []string
	}{
		{
			name:  "simple newlines",
			input: "Open\nIn Progress\nDone",
			want:  []string{"Open", "In Progress", "Done"},
		},
		{
			name:  "trims surrounding whitespace per line",
			input: "  Open  \n  Done  ",
			want:  []string{"Open", "Done"},
		},
		{
			name:  "drops blank lines",
			input: "Open\n\n\nDone",
			want:  []string{"Open", "Done"},
		},
		{
			name:  "drops whitespace-only lines",
			input: "Open\n   \nDone",
			want:  []string{"Open", "Done"},
		},
		{
			name:  "empty string yields empty slice",
			input: "",
			want:  []string{},
		},
		{
			name:  "all blank yields empty slice",
			input: "\n   \n\n",
			want:  []string{},
		},
		{
			name:  "single stage",
			input: "Now",
			want:  []string{"Now"},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := parseStages(tc.input)
			if len(got) != len(tc.want) {
				t.Errorf("len = %d, want %d; got %v", len(got), len(tc.want), got)
				return
			}
			for i, s := range got {
				if s != tc.want[i] {
					t.Errorf("[%d] = %q, want %q", i, s, tc.want[i])
				}
			}
		})
	}
}

// ---------------------------------------------------------------------------
// Error-path tests for stageFormPane.Update (StateCompleted branch).
// These live in the internal package so they can manipulate huh.Form.State
// directly, which is the simplest way to exercise the error branches without
// driving a full UI key sequence through the huh state machine.
// ---------------------------------------------------------------------------

// errStoreFS is a minimal StoreFS implementation whose ReadStages and
// WriteStages return the configured errors, and whose WriteStages records
// whether it was called and what was written. seed, when non-nil, is what
// ReadStages returns instead of an empty registry — edit-mode tests use this
// to exercise replace-by-name against a populated user file. updateErrIDs/
// updateCalls support remapFormPane tests: a node ID present in updateErrIDs
// fails on UpdateNode, and every call (successful or not) is recorded in
// updateCalls in order.
type errStoreFS struct {
	readErr     error
	writeErr    error
	seed        []types.StageGroup
	written     bool
	lastWritten []types.StageGroup

	updateErrIDs map[string]bool
	updateCalls  []string
}

func (s *errStoreFS) ReadNode(_ string) (*types.Node, error) { return nil, nil }
func (s *errStoreFS) WriteNode(_ *types.Node) error          { return nil }
func (s *errStoreFS) DeleteEdge(_ string) error              { return nil }
func (s *errStoreFS) WriteEdge(_ *types.Edge) error          { return nil }
func (s *errStoreFS) ReadEdge(_ string) (*types.Edge, error) { return nil, nil }
func (s *errStoreFS) ArchiveNode(_ string) error             { return nil }
func (s *errStoreFS) UpdateNode(id string, _ map[string]interface{}) (*types.Node, error) {
	s.updateCalls = append(s.updateCalls, id)
	if s.updateErrIDs[id] {
		return nil, errors.New("simulated UpdateNode failure")
	}
	return &types.Node{ID: id}, nil
}
func (s *errStoreFS) ReadTemplate(_ string) (*types.Template, error) { return nil, nil }
func (s *errStoreFS) AllTemplates() ([]*types.Template, error)       { return nil, nil }
func (s *errStoreFS) ReadView(_ string) (*types.SavedView, error)    { return nil, nil }
func (s *errStoreFS) AllViews() ([]*types.SavedView, error)          { return nil, nil }
func (s *errStoreFS) ReadRitual(_ string) (*types.Ritual, error)     { return nil, nil }
func (s *errStoreFS) AllRituals() ([]*types.Ritual, error)           { return nil, nil }
func (s *errStoreFS) ReadTheme(_ string) (*types.Theme, error)       { return nil, nil }
func (s *errStoreFS) ReadConfig() (*types.Config, error)             { return nil, nil }
func (s *errStoreFS) WriteConfig(_ *types.Config) error              { return nil }
func (s *errStoreFS) ReadKinds() (*types.KindRegistry, error)        { return types.NewKindRegistry(nil), nil }
func (s *errStoreFS) WriteKinds(_ []types.Kind) error                { return nil }
func (s *errStoreFS) ReadStages() (*types.StageGroupRegistry, error) {
	if s.readErr != nil {
		return nil, s.readErr
	}
	return types.NewStageGroupRegistry(s.seed), nil
}
func (s *errStoreFS) WriteStages(groups []types.StageGroup) error {
	s.written = true
	s.lastWritten = groups
	return s.writeErr
}
func (s *errStoreFS) StorePath() string { return "/tmp/err-store" }

// driveToCompleted pre-populates the form fields on f with the standard test
// values and forces form.State to StateCompleted, then calls f.Update with a
// no-op message so the StateCompleted branch runs. Returns the emitted
// tea.Cmd.
func driveToCompleted(f stageFormPane) (stageFormPane, tea.Cmd) {
	return driveToCompletedWith(f, "test-flow", "Open\nDone", string(types.CycleTerminate), "")
}

// driveToCompletedWith is the parameterised form of driveToCompleted, letting
// edit-mode tests drive the form to completion with arbitrary field values
// (e.g. re-submitting an existing group's name to exercise the
// replace-by-name path, or a different name to exercise rename).
func driveToCompletedWith(f stageFormPane, name, stagesRaw, cycle, loopTarget string) (stageFormPane, tea.Cmd) {
	f.name = name
	f.stagesRaw = stagesRaw
	f.cycle = cycle
	f.loopTarget = loopTarget
	f.form.State = huh.StateCompleted
	updated, cmd := f.Update(tea.KeyPressMsg{}) // msg type doesn't matter; State drives the branch
	return updated.(stageFormPane), cmd
}

// collectMsgs runs cmd and collects the first message it produces.
func collectMsg(cmd tea.Cmd) tea.Msg {
	if cmd == nil {
		return nil
	}
	return cmd()
}

// TestStageFormErrorOnReadFailure verifies that a ReadStages failure during
// submit emits stageFormErrorMsg (not formCancelMsg) and aborts the write.
func TestStageFormErrorOnReadFailure(t *testing.T) {
	readErr := errors.New("disk full")
	store := &errStoreFS{readErr: readErr}

	theme, err := LoadTheme(".", "")
	if err != nil {
		t.Fatalf("LoadTheme: %v", err)
	}

	f := newStageFormPane(theme, store, nil, nil)
	_, cmd := driveToCompleted(f)

	// The command is a tea.Batch; run it to collect all messages.
	msg := collectMsg(cmd)
	if store.written {
		t.Error("WriteStages should not be called when ReadStages fails")
	}
	// tea.Batch returns a BatchMsg; unwrap one level.
	if batch, ok := msg.(tea.BatchMsg); ok {
		var found bool
		for _, fn := range batch {
			if m := fn(); m != nil {
				if errMsg, ok := m.(stageFormErrorMsg); ok {
					found = true
					if errMsg.err == nil {
						t.Error("stageFormErrorMsg.err should be non-nil")
					}
				}
				if _, ok := m.(formCancelMsg); ok {
					t.Error("formCancelMsg emitted on read failure; expected stageFormErrorMsg")
				}
			}
		}
		if !found {
			t.Error("expected stageFormErrorMsg in Batch, none found")
		}
	} else {
		// Non-batch cmd — check directly.
		if _, ok := msg.(stageFormErrorMsg); !ok {
			t.Errorf("expected stageFormErrorMsg, got %T", msg)
		}
	}
}

// TestStageFormErrorOnWriteFailure verifies that a WriteStages failure during
// submit emits stageFormErrorMsg and does not emit stageFormSubmitMsg.
func TestStageFormErrorOnWriteFailure(t *testing.T) {
	writeErr := errors.New("permission denied")
	store := &errStoreFS{
		readErr:  nil, // ReadStages succeeds (returns empty registry)
		writeErr: writeErr,
	}
	// Make ReadStages return a valid empty registry instead of an error.
	store.readErr = nil

	theme, err := LoadTheme(".", "")
	if err != nil {
		t.Fatalf("LoadTheme: %v", err)
	}

	f := newStageFormPane(theme, store, nil, nil)
	_, cmd := driveToCompleted(f)

	// Unwrap the batch to find the error message.
	msg := collectMsg(cmd)
	if batch, ok := msg.(tea.BatchMsg); ok {
		var foundErr, foundSubmit bool
		for _, fn := range batch {
			if m := fn(); m != nil {
				if _, ok := m.(stageFormErrorMsg); ok {
					foundErr = true
				}
				if _, ok := m.(stageFormSubmitMsg); ok {
					foundSubmit = true
				}
				if _, ok := m.(formCancelMsg); ok {
					t.Error("formCancelMsg emitted on write failure; expected stageFormErrorMsg")
				}
			}
		}
		if foundSubmit {
			t.Error("stageFormSubmitMsg emitted despite write failure")
		}
		if !foundErr {
			t.Error("expected stageFormErrorMsg in Batch, none found")
		}
	} else {
		if _, ok := msg.(stageFormErrorMsg); !ok {
			t.Errorf("expected stageFormErrorMsg, got %T", msg)
		}
	}
}

// ---------------------------------------------------------------------------
// upsertStageGroup — pure unit tests, no huh involved. Mirrors
// kind_form_internal_test.go's TestUpsertKind* suite.
// ---------------------------------------------------------------------------

func TestUpsertStageGroupCreateAppends(t *testing.T) {
	existing := []types.StageGroup{{Name: "task-flow", Stages: []string{"Open", "Done"}}}
	group := types.StageGroup{Name: "review-flow", Stages: []string{"Draft", "Merged"}}

	got := upsertStageGroup(existing, group, "")

	if len(got) != 2 {
		t.Fatalf("len = %d, want 2", len(got))
	}
	if got[1].Name != "review-flow" {
		t.Errorf("appended entry = %v, want review-flow", got[1])
	}
}

func TestUpsertStageGroupEditReplacesAtSameIndex(t *testing.T) {
	existing := []types.StageGroup{
		{Name: "alpha-flow", Stages: []string{"A"}},
		{Name: "review-flow", Stages: []string{"Draft", "Merged"}},
		{Name: "zeta-flow", Stages: []string{"Z"}},
	}
	edited := types.StageGroup{Name: "review-flow", Stages: []string{"Draft", "Review", "Merged"}}

	got := upsertStageGroup(existing, edited, "review-flow")

	if len(got) != 3 {
		t.Fatalf("len = %d, want 3 (replace, not append)", len(got))
	}
	if got[1].Name != "review-flow" || len(got[1].Stages) != 3 {
		t.Errorf("got[1] = %v, want the edited review-flow at the same index", got[1])
	}
	if got[0].Name != "alpha-flow" || got[2].Name != "zeta-flow" {
		t.Errorf("order disturbed: got %v", got)
	}
}

func TestUpsertStageGroupEditDefaultNotYetShadowedAppends(t *testing.T) {
	existing := []types.StageGroup{{Name: "review-flow", Stages: []string{"Draft"}}}
	edited := types.StageGroup{Name: "task-flow", Stages: []string{"Open", "Doing", "Done", "Archived"}}

	got := upsertStageGroup(existing, edited, "task-flow")

	if len(got) != 2 {
		t.Fatalf("len = %d, want 2 (shadow appended)", len(got))
	}
	if got[1].Name != "task-flow" || len(got[1].Stages) != 4 {
		t.Errorf("appended shadow = %v, want edited task-flow", got[1])
	}
}

func TestUpsertStageGroupEmptySliceAppends(t *testing.T) {
	edited := types.StageGroup{Name: "task-flow", Stages: []string{"Open"}}

	got := upsertStageGroup(nil, edited, "task-flow")

	if len(got) != 1 || got[0].Name != "task-flow" {
		t.Errorf("got = %v, want single task-flow entry", got)
	}
}

// ---------------------------------------------------------------------------
// Edit-mode form-level tests. Mirrors kind_form_internal_test.go's
// TestKindEditForm* suite.
// ---------------------------------------------------------------------------

// TestStageEditFormSeedsFields verifies the edit constructor seeds all
// fields from the existing entry, including the stagesRaw round-trip.
func TestStageEditFormSeedsFields(t *testing.T) {
	theme, err := LoadTheme(".", "")
	if err != nil {
		t.Fatalf("LoadTheme: %v", err)
	}
	store := &errStoreFS{}
	existing := types.StageGroup{
		Name:       "review-flow",
		Stages:     []string{"Draft", "Review", "Merged"},
		Cycle:      types.CycleLoopToStage,
		LoopTarget: "Review",
	}

	f := newStageFormPane(theme, store, nil, &existing)

	if f.name != "review-flow" {
		t.Errorf("f.name = %q, want %q", f.name, "review-flow")
	}
	if f.stagesRaw != "Draft\nReview\nMerged" {
		t.Errorf("f.stagesRaw = %q, want %q", f.stagesRaw, "Draft\nReview\nMerged")
	}
	if f.cycle != string(types.CycleLoopToStage) {
		t.Errorf("f.cycle = %q, want %q", f.cycle, string(types.CycleLoopToStage))
	}
	if f.loopTarget != "Review" {
		t.Errorf("f.loopTarget = %q, want %q", f.loopTarget, "Review")
	}
	if f.originalName != "review-flow" {
		t.Errorf("f.originalName = %q, want %q", f.originalName, "review-flow")
	}
}

// TestStageEditFormStagesRawRoundTrips verifies parseStages(join(stages,
// "\n")) reproduces the original slice — the seeding conversion is the only
// lossy-looking one in the edit path, so it's worth its own test beyond the
// single case in TestStageEditFormSeedsFields.
func TestStageEditFormStagesRawRoundTrips(t *testing.T) {
	stages := []string{"Backlog", "In Progress", "Done"}
	raw := strings.Join(stages, "\n")
	got := parseStages(raw)

	if len(got) != len(stages) {
		t.Fatalf("len = %d, want %d", len(got), len(stages))
	}
	for i := range stages {
		if got[i] != stages[i] {
			t.Errorf("[%d] = %q, want %q", i, got[i], stages[i])
		}
	}
}

// TestStageEditFormMarksIsDefault verifies isDefault is set when the edited
// group's name matches a baked-in default.
func TestStageEditFormMarksIsDefault(t *testing.T) {
	theme, err := LoadTheme(".", "")
	if err != nil {
		t.Fatalf("LoadTheme: %v", err)
	}
	store := &errStoreFS{}
	existing := types.StageGroup{Name: "task-flow", Stages: []string{"Open", "Done"}}

	f := newStageFormPane(theme, store, nil, &existing)

	if !f.isDefault {
		t.Error("expected isDefault = true for a group named task-flow (a baked-in default)")
	}
}

// TestStageEditFormNotDefaultForCustomName verifies isDefault stays false
// for a name that isn't a baked-in default.
func TestStageEditFormNotDefaultForCustomName(t *testing.T) {
	theme, err := LoadTheme(".", "")
	if err != nil {
		t.Fatalf("LoadTheme: %v", err)
	}
	store := &errStoreFS{}
	existing := types.StageGroup{Name: "review-flow", Stages: []string{"Draft"}}

	f := newStageFormPane(theme, store, nil, &existing)

	if f.isDefault {
		t.Error("expected isDefault = false for a custom group name")
	}
}

// TestStageEditFormSubmitReplacesExisting verifies submitting in edit mode
// (name unchanged) writes the replaced slice, not an appended one.
func TestStageEditFormSubmitReplacesExisting(t *testing.T) {
	store := &errStoreFS{seed: []types.StageGroup{
		{Name: "alpha-flow", Stages: []string{"A"}},
		{Name: "review-flow", Stages: []string{"Draft", "Merged"}},
	}}
	theme, err := LoadTheme(".", "")
	if err != nil {
		t.Fatalf("LoadTheme: %v", err)
	}
	existing := types.StageGroup{Name: "review-flow", Stages: []string{"Draft", "Merged"}}

	f := newStageFormPane(theme, store, nil, &existing)
	_, cmd := driveToCompletedWith(f, "review-flow", "Draft\nReview\nMerged", string(types.CycleTerminate), "")
	collectMsg(cmd)

	if !store.written {
		t.Fatal("WriteStages should have been called")
	}
	if len(store.lastWritten) != 2 {
		t.Fatalf("lastWritten len = %d, want 2 (replace, not append)", len(store.lastWritten))
	}
	if store.lastWritten[0].Name != "alpha-flow" {
		t.Errorf("lastWritten[0] = %v, want alpha-flow unchanged", store.lastWritten[0])
	}
	if store.lastWritten[1].Name != "review-flow" || len(store.lastWritten[1].Stages) != 3 {
		t.Errorf("lastWritten[1] = %v, want edited review-flow with 3 stages", store.lastWritten[1])
	}
}

// TestStageEditFormRenameEmitsRenamedFrom verifies that submitting a changed
// name in edit mode sets renamedFrom on stageFormSubmitMsg.
func TestStageEditFormRenameEmitsRenamedFrom(t *testing.T) {
	store := &errStoreFS{seed: []types.StageGroup{
		{Name: "review-flow", Stages: []string{"Draft", "Merged"}},
	}}
	theme, err := LoadTheme(".", "")
	if err != nil {
		t.Fatalf("LoadTheme: %v", err)
	}
	existing := types.StageGroup{Name: "review-flow", Stages: []string{"Draft", "Merged"}}

	f := newStageFormPane(theme, store, nil, &existing)
	_, cmd := driveToCompletedWith(f, "pr-flow", "Draft\nMerged", string(types.CycleTerminate), "")

	msg := collectMsg(cmd)
	sub := findStageSubmitMsg(t, msg)
	if sub.name != "pr-flow" {
		t.Errorf("sub.name = %q, want %q", sub.name, "pr-flow")
	}
	if sub.renamedFrom != "review-flow" {
		t.Errorf("sub.renamedFrom = %q, want %q", sub.renamedFrom, "review-flow")
	}

	if len(store.lastWritten) != 1 || store.lastWritten[0].Name != "pr-flow" {
		t.Errorf("lastWritten = %v, want single pr-flow entry", store.lastWritten)
	}
}

// TestStageEditFormUnchangedNameNoRename verifies renamedFrom stays empty
// when the name is resubmitted unchanged.
func TestStageEditFormUnchangedNameNoRename(t *testing.T) {
	store := &errStoreFS{seed: []types.StageGroup{
		{Name: "review-flow", Stages: []string{"Draft", "Merged"}},
	}}
	theme, err := LoadTheme(".", "")
	if err != nil {
		t.Fatalf("LoadTheme: %v", err)
	}
	existing := types.StageGroup{Name: "review-flow", Stages: []string{"Draft", "Merged"}}

	f := newStageFormPane(theme, store, nil, &existing)
	_, cmd := driveToCompletedWith(f, "review-flow", "Draft\nMerged", string(types.CycleTerminate), "")

	msg := collectMsg(cmd)
	sub := findStageSubmitMsg(t, msg)
	if sub.renamedFrom != "" {
		t.Errorf("sub.renamedFrom = %q, want empty (name unchanged)", sub.renamedFrom)
	}
}

// TestStageEditFormRenameDefaultWritesTombstone verifies that renaming a
// baked-in default group writes both the renamed entry AND a tombstone
// shadow under the old name, mirroring kindFormPane's identical mechanism.
func TestStageEditFormRenameDefaultWritesTombstone(t *testing.T) {
	store := &errStoreFS{} // "task-flow" not yet shadowed
	theme, err := LoadTheme(".", "")
	if err != nil {
		t.Fatalf("LoadTheme: %v", err)
	}
	defaults, err := stage.DefaultStageGroups()
	if err != nil {
		t.Fatalf("DefaultStageGroups: %v", err)
	}
	var taskFlowDefault types.StageGroup
	for _, d := range defaults {
		if d.Name == "task-flow" {
			taskFlowDefault = d
			break
		}
	}
	if taskFlowDefault.Name == "" {
		t.Fatal("precondition failed: no baked-in task-flow default found")
	}

	f := newStageFormPane(theme, store, nil, &taskFlowDefault)
	if !f.isDefault {
		t.Fatal("precondition failed: task-flow should be recognised as a default")
	}
	_, cmd := driveToCompletedWith(f, "todo-flow", "Open\nDoing\nDone", string(types.CycleTerminate), "")
	collectMsg(cmd)

	if len(store.lastWritten) != 2 {
		t.Fatalf("lastWritten len = %d, want 2 (renamed entry + tombstone), got %v", len(store.lastWritten), store.lastWritten)
	}
	var sawRenamed, sawTombstone bool
	for _, g := range store.lastWritten {
		if g.Name == "todo-flow" {
			sawRenamed = true
		}
		if g.Name == "task-flow" {
			sawTombstone = true
			// The tombstone must match the embedded default exactly — it's
			// an unmodified copy, not a re-derivation, so compare stage-for-stage.
			if len(g.Stages) != len(taskFlowDefault.Stages) {
				t.Errorf("tombstone has %d stages, want %d matching the default", len(g.Stages), len(taskFlowDefault.Stages))
			}
			for i := range taskFlowDefault.Stages {
				if i < len(g.Stages) && g.Stages[i] != taskFlowDefault.Stages[i] {
					t.Errorf("tombstone stage[%d] = %q, want %q", i, g.Stages[i], taskFlowDefault.Stages[i])
				}
			}
		}
	}
	if !sawRenamed {
		t.Error("expected the renamed todo-flow entry in lastWritten")
	}
	if !sawTombstone {
		t.Error("expected a task-flow tombstone entry in lastWritten so the default stays shadowed")
	}
}

// TestStageEditFormRenameCustomGroupNoTombstone verifies renaming a purely
// custom group does NOT write a tombstone.
func TestStageEditFormRenameCustomGroupNoTombstone(t *testing.T) {
	store := &errStoreFS{seed: []types.StageGroup{
		{Name: "review-flow", Stages: []string{"Draft", "Merged"}},
	}}
	theme, err := LoadTheme(".", "")
	if err != nil {
		t.Fatalf("LoadTheme: %v", err)
	}
	existing := types.StageGroup{Name: "review-flow", Stages: []string{"Draft", "Merged"}}

	f := newStageFormPane(theme, store, nil, &existing)
	_, cmd := driveToCompletedWith(f, "pr-flow", "Draft\nMerged", string(types.CycleTerminate), "")
	collectMsg(cmd)

	if len(store.lastWritten) != 1 {
		t.Fatalf("lastWritten len = %d, want 1 (renamed only, no tombstone), got %v", len(store.lastWritten), store.lastWritten)
	}
	if store.lastWritten[0].Name != "pr-flow" {
		t.Errorf("lastWritten[0].Name = %q, want %q", store.lastWritten[0].Name, "pr-flow")
	}
}

// ---------------------------------------------------------------------------
// TD.14 — ShadowOf provenance stamping.
// ---------------------------------------------------------------------------

// TestStageFormCreateLeavesShadowOfEmpty verifies a brand-new, purely
// user-authored stage group is never stamped.
func TestStageFormCreateLeavesShadowOfEmpty(t *testing.T) {
	store := &errStoreFS{}
	theme, err := LoadTheme(".", "")
	if err != nil {
		t.Fatalf("LoadTheme: %v", err)
	}

	f := newStageFormPane(theme, store, nil, nil)
	_, cmd := driveToCompleted(f)
	collectMsg(cmd)

	if len(store.lastWritten) != 1 {
		t.Fatalf("lastWritten len = %d, want 1", len(store.lastWritten))
	}
	if store.lastWritten[0].ShadowOf != "" {
		t.Errorf("ShadowOf = %q, want empty for a create-mode group", store.lastWritten[0].ShadowOf)
	}
}

// TestStageEditFormStampsShadowOfOnDefault verifies editing a still-unshadowed
// baked-in default stamps ShadowOf with that default's content hash.
func TestStageEditFormStampsShadowOfOnDefault(t *testing.T) {
	store := &errStoreFS{} // "task-flow" not yet shadowed
	theme, err := LoadTheme(".", "")
	if err != nil {
		t.Fatalf("LoadTheme: %v", err)
	}
	defaults, err := stage.DefaultStageGroups()
	if err != nil {
		t.Fatalf("DefaultStageGroups: %v", err)
	}
	var taskFlowDefault types.StageGroup
	for _, d := range defaults {
		if d.Name == "task-flow" {
			taskFlowDefault = d
			break
		}
	}
	if taskFlowDefault.Name == "" {
		t.Fatal("precondition failed: no baked-in task-flow default found")
	}

	f := newStageFormPane(theme, store, nil, &taskFlowDefault)
	if !f.isDefault {
		t.Fatal("precondition failed: task-flow should be recognised as a default")
	}
	_, cmd := driveToCompletedWith(f, "task-flow", "Open\nDoing\nDone", string(types.CycleTerminate), "")
	collectMsg(cmd)

	if len(store.lastWritten) != 1 {
		t.Fatalf("lastWritten len = %d, want 1", len(store.lastWritten))
	}
	want := stage.DefaultStageGroupHash("task-flow")
	if want == "" {
		t.Fatal("precondition failed: DefaultStageGroupHash(task-flow) should be non-empty")
	}
	if got := store.lastWritten[0].ShadowOf; got != want {
		t.Errorf("ShadowOf = %q, want %q", got, want)
	}
}

// TestStageEditFormNoShadowOfOnCustomGroup verifies editing a purely
// user-authored (non-default) group never stamps ShadowOf.
func TestStageEditFormNoShadowOfOnCustomGroup(t *testing.T) {
	store := &errStoreFS{seed: []types.StageGroup{
		{Name: "review-flow", Stages: []string{"Draft", "Merged"}},
	}}
	theme, err := LoadTheme(".", "")
	if err != nil {
		t.Fatalf("LoadTheme: %v", err)
	}
	existing := types.StageGroup{Name: "review-flow", Stages: []string{"Draft", "Merged"}}

	f := newStageFormPane(theme, store, nil, &existing)
	_, cmd := driveToCompletedWith(f, "review-flow", "Draft\nMerged", string(types.CycleTerminate), "")
	collectMsg(cmd)

	if len(store.lastWritten) != 1 {
		t.Fatalf("lastWritten len = %d, want 1", len(store.lastWritten))
	}
	if store.lastWritten[0].ShadowOf != "" {
		t.Errorf("ShadowOf = %q, want empty for a custom group", store.lastWritten[0].ShadowOf)
	}
}

// TestStageEditFormReEditPreservesOriginalShadowOf is the critical regression
// test: re-editing an entry that is already a shadow must carry its existing
// ShadowOf forward unchanged, never recomputing against the current default.
func TestStageEditFormReEditPreservesOriginalShadowOf(t *testing.T) {
	sentinel := "sha256:deadbeefdeadbeef"
	existing := types.StageGroup{Name: "task-flow", Stages: []string{"Open", "Done"}, Cycle: types.CycleTerminate, ShadowOf: sentinel}
	store := &errStoreFS{seed: []types.StageGroup{existing}}
	theme, err := LoadTheme(".", "")
	if err != nil {
		t.Fatalf("LoadTheme: %v", err)
	}

	f := newStageFormPane(theme, store, nil, &existing)
	// Edit an unrelated field (add a stage) — the shadow's ShadowOf must not move.
	_, cmd := driveToCompletedWith(f, "task-flow", "Open\nDoing\nDone", string(types.CycleTerminate), "")
	collectMsg(cmd)

	if len(store.lastWritten) != 1 {
		t.Fatalf("lastWritten len = %d, want 1", len(store.lastWritten))
	}
	got := store.lastWritten[0].ShadowOf
	if got != sentinel {
		t.Errorf("ShadowOf = %q, want preserved sentinel %q (not recomputed against the current default %q)",
			got, sentinel, stage.DefaultStageGroupHash("task-flow"))
	}
}

// TestStageEditFormRenameDefaultStampsBothShadowAndTombstone verifies that
// renaming a baked-in default stamps ShadowOf on both the renamed entry and
// the tombstone left under the old name.
func TestStageEditFormRenameDefaultStampsBothShadowAndTombstone(t *testing.T) {
	store := &errStoreFS{} // "task-flow" not yet shadowed
	theme, err := LoadTheme(".", "")
	if err != nil {
		t.Fatalf("LoadTheme: %v", err)
	}
	defaults, err := stage.DefaultStageGroups()
	if err != nil {
		t.Fatalf("DefaultStageGroups: %v", err)
	}
	var taskFlowDefault types.StageGroup
	for _, d := range defaults {
		if d.Name == "task-flow" {
			taskFlowDefault = d
			break
		}
	}
	if taskFlowDefault.Name == "" {
		t.Fatal("precondition failed: no baked-in task-flow default found")
	}

	f := newStageFormPane(theme, store, nil, &taskFlowDefault)
	_, cmd := driveToCompletedWith(f, "todo-flow", "Open\nDoing\nDone", string(types.CycleTerminate), "")
	collectMsg(cmd)

	if len(store.lastWritten) != 2 {
		t.Fatalf("lastWritten len = %d, want 2 (renamed entry + tombstone)", len(store.lastWritten))
	}
	wantHash := stage.DefaultStageGroupHash("task-flow")
	if wantHash == "" {
		t.Fatal("precondition failed: DefaultStageGroupHash(task-flow) should be non-empty")
	}
	var sawRenamed, sawTombstone bool
	for _, g := range store.lastWritten {
		if g.Name == "todo-flow" {
			sawRenamed = true
			if g.ShadowOf != wantHash {
				t.Errorf("renamed entry ShadowOf = %q, want %q", g.ShadowOf, wantHash)
			}
		}
		if g.Name == "task-flow" {
			sawTombstone = true
			if g.ShadowOf != wantHash {
				t.Errorf("tombstone ShadowOf = %q, want %q", g.ShadowOf, wantHash)
			}
		}
	}
	if !sawRenamed {
		t.Error("expected the renamed todo-flow entry in lastWritten")
	}
	if !sawTombstone {
		t.Error("expected a task-flow tombstone entry in lastWritten")
	}
}

// TestStageEditFormOwnNameDoesNotCollide verifies that resubmitting a
// group's own unchanged name passes the collision validator in edit mode.
func TestStageEditFormOwnNameDoesNotCollide(t *testing.T) {
	groups := types.NewStageGroupRegistry([]types.StageGroup{
		{Name: "review-flow", Stages: []string{"Draft"}},
		{Name: "task-flow", Stages: []string{"Open"}},
	})
	theme, err := LoadTheme(".", "")
	if err != nil {
		t.Fatalf("LoadTheme: %v", err)
	}
	store := &errStoreFS{seed: groups.All()}
	existing := types.StageGroup{Name: "review-flow", Stages: []string{"Draft"}}

	f := newStageFormPane(theme, store, groups, &existing)
	_, cmd := driveToCompletedWith(f, "review-flow", "Draft", string(types.CycleTerminate), "")

	msg := collectMsg(cmd)
	if _, ok := findStageErrorMsg(msg); ok {
		t.Error("resubmitting the group's own unchanged name should not collide")
	}
}

// TestStageEditFormOtherNameStillCollides verifies edit mode still rejects a
// name belonging to a different existing group.
func TestStageEditFormOtherNameStillCollides(t *testing.T) {
	groups := types.NewStageGroupRegistry([]types.StageGroup{
		{Name: "review-flow", Stages: []string{"Draft"}},
		{Name: "task-flow", Stages: []string{"Open"}},
	})
	theme, err := LoadTheme(".", "")
	if err != nil {
		t.Fatalf("LoadTheme: %v", err)
	}
	store := &errStoreFS{seed: groups.All()}
	existing := types.StageGroup{Name: "review-flow", Stages: []string{"Draft"}}

	f := newStageFormPane(theme, store, groups, &existing)
	_, cmd := driveToCompletedWith(f, "task-flow", "Draft", string(types.CycleTerminate), "")

	msg := collectMsg(cmd)
	if _, ok := findStageErrorMsg(msg); !ok {
		t.Error("renaming to another existing group's name should collide")
	}
	if store.written {
		t.Error("WriteStages should not be called when the new name collides")
	}
}

// TestStageEditFormCaseOnlyRenameRejected verifies the specific
// capitalisation-only error message fires.
func TestStageEditFormCaseOnlyRenameRejected(t *testing.T) {
	groups := types.NewStageGroupRegistry([]types.StageGroup{{Name: "review-flow", Stages: []string{"Draft"}}})
	theme, err := LoadTheme(".", "")
	if err != nil {
		t.Fatalf("LoadTheme: %v", err)
	}
	store := &errStoreFS{seed: groups.All()}
	existing := types.StageGroup{Name: "review-flow", Stages: []string{"Draft"}}

	f := newStageFormPane(theme, store, groups, &existing)
	_, cmd := driveToCompletedWith(f, "Review-Flow", "Draft", string(types.CycleTerminate), "")

	msg := collectMsg(cmd)
	errMsg, ok := findStageErrorMsg(msg)
	if !ok {
		t.Fatal("expected a validation error for a case-only rename")
	}
	if !strings.Contains(errMsg.err.Error(), "capitalisation") {
		t.Errorf("error = %q, want it to mention capitalisation", errMsg.err.Error())
	}
	if store.written {
		t.Error("WriteStages should not be called on a rejected case-only rename")
	}
}

// TestStageEditFormLoopToStageSeeding verifies edit mode seeds f.cycle
// correctly when the existing group uses loop-to-stage — WithHideFunc
// (unchanged by edit mode) reads f.cycle only at group-navigation time, not
// at render, so the meaningful check here is that the seeded value reaches
// the field at all, which TestStageEditFormSeedsFields already covers for
// f.loopTarget too. This test isolates the cycle value specifically, since
// it's what the (untouched) hide-func decision depends on.
func TestStageEditFormLoopToStageSeeding(t *testing.T) {
	theme, err := LoadTheme(".", "")
	if err != nil {
		t.Fatalf("LoadTheme: %v", err)
	}
	store := &errStoreFS{}
	existing := types.StageGroup{
		Name:       "sprint-flow",
		Stages:     []string{"A", "B", "C"},
		Cycle:      types.CycleLoopToStage,
		LoopTarget: "B",
	}

	f := newStageFormPane(theme, store, nil, &existing)

	if f.cycle != string(types.CycleLoopToStage) {
		t.Errorf("f.cycle = %q, want %q — group 2's WithHideFunc reads this at navigation time", f.cycle, string(types.CycleLoopToStage))
	}
}

// findStageSubmitMsg unwraps a tea.BatchMsg (or bare msg) looking for a
// stageFormSubmitMsg, failing the test if none is found.
func findStageSubmitMsg(t *testing.T, msg tea.Msg) stageFormSubmitMsg {
	t.Helper()
	if batch, ok := msg.(tea.BatchMsg); ok {
		for _, fn := range batch {
			if m := fn(); m != nil {
				if sub, ok := m.(stageFormSubmitMsg); ok {
					return sub
				}
			}
		}
	} else if sub, ok := msg.(stageFormSubmitMsg); ok {
		return sub
	}
	t.Fatalf("expected stageFormSubmitMsg, got %T", msg)
	return stageFormSubmitMsg{}
}

// findStageErrorMsg unwraps a tea.BatchMsg (or bare msg) looking for a
// stageFormErrorMsg.
func findStageErrorMsg(msg tea.Msg) (stageFormErrorMsg, bool) {
	if batch, ok := msg.(tea.BatchMsg); ok {
		for _, fn := range batch {
			if m := fn(); m != nil {
				if errMsg, ok := m.(stageFormErrorMsg); ok {
					return errMsg, true
				}
			}
		}
		return stageFormErrorMsg{}, false
	}
	errMsg, ok := msg.(stageFormErrorMsg)
	return errMsg, ok
}
