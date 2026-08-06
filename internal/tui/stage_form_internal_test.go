package tui

import (
	"errors"
	"testing"

	tea "charm.land/bubbletea/v2"
	huh "charm.land/huh/v2"
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
func (s *errStoreFS) StorePath() string                      { return "/tmp/err-store" }

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

	f := newStageFormPane(theme, store, nil)
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

	f := newStageFormPane(theme, store, nil)
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
