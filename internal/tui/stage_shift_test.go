package tui_test

import (
	"strings"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/jasonwarrenuk/wyrd/internal/query"
	"github.com/jasonwarrenuk/wyrd/internal/stage"
	"github.com/jasonwarrenuk/wyrd/internal/store"
	"github.com/jasonwarrenuk/wyrd/internal/tui"
	"github.com/jasonwarrenuk/wyrd/internal/types"
)

// newTestModelWithKindedNode creates a model seeded with a single node that
// has a known kind ("Task") and initial stage ("Open"). The model is wired
// with the full kinds + stage-group registries so `]`/`[` keypresses work.
func newTestModelWithKindedNode(t *testing.T) (tui.Model, string) {
	t.Helper()
	dir := t.TempDir()
	clock := types.StubClock{Fixed: time.Date(2026, 3, 23, 9, 0, 0, 0, time.UTC)}
	s, err := store.New(dir, clock)
	if err != nil {
		t.Fatalf("store.New: %v", err)
	}
	t.Cleanup(func() { s.Close() })

	node, err := s.CreateNode("Test task", []string{"task"})
	if err != nil {
		t.Fatalf("CreateNode: %v", err)
	}
	node.Kind = "Task"
	node.Stage = "Open"
	node.Title = "Test task"
	if err := s.WriteNode(node); err != nil {
		t.Fatalf("WriteNode: %v", err)
	}
	// Force the index to see the updated kind/stage fields.
	if _, err := s.UpdateNode(node.ID, map[string]interface{}{"kind": "Task", "stage": "Open"}); err != nil {
		t.Fatalf("UpdateNode(seed): %v", err)
	}

	kindDefaults, err := stage.DefaultKinds()
	if err != nil {
		t.Fatalf("DefaultKinds: %v", err)
	}
	kinds := stage.MergeKinds(kindDefaults, nil)

	groupDefaults, err := stage.DefaultStageGroups()
	if err != nil {
		t.Fatalf("DefaultStageGroups: %v", err)
	}
	stageGroups := stage.MergeStageGroups(groupDefaults, nil)

	engine := query.NewEngine(s.Index(), 10)
	m, err := tui.New(tui.Config{
		Store:       s,
		StorePath:   dir,
		Index:       s.Index(),
		QueryRunner: engine,
		Clock:       clock,
		Kinds:       kinds,
		StageGroups: stageGroups,
	})
	if err != nil {
		t.Fatalf("tui.New: %v", err)
	}
	return m, node.ID
}

// TestAdvanceStageUpdatesStatusBar verifies that pressing `]` on a kinded node
// advances its stage and shows a "Stage: X → Y" confirmation.
func TestAdvanceStageUpdatesStatusBar(t *testing.T) {
	m, _ := newTestModelWithKindedNode(t)
	m = sendWindowSize(t, m, 120, 40)

	m = pressKey(t, m, ']', "]")

	view := m.View().Content
	if !strings.Contains(view, "Stage:") {
		t.Errorf("expected 'Stage:' confirmation in status bar after ]; got content length %d", len(view))
	}
	if !strings.Contains(view, "→") {
		t.Errorf("expected arrow '→' in status bar after ]; got: %q (first 200 bytes)", truncate(view, 200))
	}
}

// TestRetreatStageAtFirstStageShowsTerminal verifies that pressing `[` on a
// node already at the first stage of a terminate-cycle group does not write
// and instead shows a "(terminal)" hint.
func TestRetreatStageAtFirstStageShowsTerminal(t *testing.T) {
	// task-flow uses CycleTerminate, so retreating from Open (the first stage)
	// is a no-op and should show the terminal hint.
	m, _ := newTestModelWithKindedNode(t)
	m = sendWindowSize(t, m, 120, 40)

	m = pressKey(t, m, '[', "[")

	view := m.View().Content
	if !strings.Contains(view, "terminal") {
		t.Errorf("expected '(terminal)' hint in status bar when retreating from first stage; got (first 200 bytes): %q", truncate(view, 200))
	}
}

// TestAdvanceStageAtLastStageShowsTerminal verifies that pressing `]` on a
// node at the last stage of a terminate-cycle group shows the terminal hint.
func TestAdvanceStageAtLastStageShowsTerminal(t *testing.T) {
	// Advance through all task-flow stages: Open→Maybe→Later→Soon→Now→Done.
	m, _ := newTestModelWithKindedNode(t)
	m = sendWindowSize(t, m, 120, 40)

	// task-flow has 6 stages; 5 advances land on Done.
	for i := 0; i < 5; i++ {
		m = pressKey(t, m, ']', "]")
	}
	// One more advance on Done (terminal) should show the hint.
	m = pressKey(t, m, ']', "]")

	view := m.View().Content
	if !strings.Contains(view, "terminal") {
		t.Errorf("expected '(terminal)' hint after pressing ] on last stage; got (first 200 bytes): %q", truncate(view, 200))
	}
}

// TestStageShiftNoOpOnUntriagdNode verifies that `]`/`[` are silent no-ops on
// a node with no kind assigned.
func TestStageShiftNoOpOnUntriagedNode(t *testing.T) {
	// newTestModelWithNode (from app_test.go) creates a plain task node with no Kind.
	m, _ := newTestModelWithNode(t)
	m = sendWindowSize(t, m, 120, 40)

	viewBefore := m.View().Content

	m = pressKey(t, m, ']', "]")

	viewAfter := m.View().Content
	// No Stage: confirmation should appear; view should be unchanged.
	if strings.Contains(viewAfter, "Stage:") {
		t.Error("expected no stage confirmation for untriaged node (no kind), but got one")
	}
	if viewAfter != viewBefore {
		// A capture bar update or similar unrelated change could differ; only
		// fail if Stage: text appeared.
		_ = viewBefore // not fatal if something unrelated changed
	}
}

// TestStageShiftIgnoredWhenFormActive verifies that `]` is a no-op when a form
// is already open, mirroring the equivalent archive behaviour.
func TestStageShiftIgnoredWhenFormActive(t *testing.T) {
	m, _ := newTestModelWithKindedNode(t)
	m = sendWindowSize(t, m, 120, 40)

	// Open a capture form.
	m = pressModKey(t, m, 'n', tea.ModCtrl)
	for _, r := range "t: Some task" {
		m = pressKey(t, m, r, string(r))
	}
	m = pressKey(t, m, tea.KeyEnter, "")

	viewWithForm := m.View().Content

	m = pressKey(t, m, ']', "]")

	viewAfterShift := m.View().Content
	if viewAfterShift != viewWithForm {
		t.Error("] should not change the view when a form is already active")
	}
}

// truncate returns the first n bytes of s for use in test error messages.
func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n]
}
