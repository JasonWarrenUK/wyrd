package tui

import (
	"errors"
	"testing"

	tea "charm.land/bubbletea/v2"
	huh "charm.land/huh/v2"
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

	f := newKindFormPane(theme, store, nil, nil)
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

	f := newKindFormPane(theme, store, nil, nil)
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

	f := newKindFormPane(theme, store, nil, nil)
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

	f := newKindFormPane(theme, store, nil, groups)

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

	f := newKindFormPane(theme, store, nil, nil) // nil groups -> f.stageGroup stays ""
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
