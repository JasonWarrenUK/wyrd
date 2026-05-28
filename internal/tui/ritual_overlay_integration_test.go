package tui

import (
	"strings"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/jasonwarrenuk/wyrd/internal/query"
	"github.com/jasonwarrenuk/wyrd/internal/store"
	"github.com/jasonwarrenuk/wyrd/internal/types"
)

// newRitualTestModel builds a Model wired to a real store and query engine so
// the ritual subsystem has the dependencies ritualTriggerMsg requires. It then
// delivers a window-size message so the layout has non-zero dimensions, which
// the overlay needs in order to render.
//
// This is a white-box (package tui) test because the ritual wiring it exercises
// — ritualTriggerMsg, the ritualOvl field, and schedulerState — is unexported.
func newRitualTestModel(t *testing.T) Model {
	t.Helper()
	dir := t.TempDir()
	clock := types.StubClock{Fixed: time.Date(2026, 3, 23, 9, 0, 0, 0, time.UTC)}
	s, err := store.New(dir, clock)
	if err != nil {
		t.Fatalf("store.New: %v", err)
	}
	t.Cleanup(func() { s.Close() })

	engine := query.NewEngine(s.Index(), 10)
	m, err := New(Config{
		Store:       s,
		StorePath:   dir,
		Index:       s.Index(),
		QueryRunner: engine,
		Clock:       clock,
	})
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	updated, _ := m.Update(tea.WindowSizeMsg{Width: 100, Height: 40})
	return updated.(Model)
}

// nudgePromptRitual returns a single-step nudge ritual whose only step is a
// prompt. A prompt step has no query or store dependency — executeCurrentStep
// simply sets the output to step.Text — so it is the least-coupled way to drive
// the overlay end to end.
func nudgePromptRitual() *types.Ritual {
	return &types.Ritual{
		Name:     "morning-check",
		Friction: types.FrictionNudge,
		Steps: []types.RitualStep{
			{
				Type: types.StepPrompt,
				Text: "How are you feeling today?",
			},
		},
	}
}

// TestRitualOverlay_TriggerRendersOverlay drives a ritualTriggerMsg through the
// merged Update loop and asserts the overlay both activates AND renders into the
// composed frame. The View() assertion is the regression guard for the bug where
// Model.View() never called m.ritualOvl.View(): the overlay was active and ate
// keystrokes while drawing nothing.
func TestRitualOverlay_TriggerRendersOverlay(t *testing.T) {
	m := newRitualTestModel(t)
	r := nudgePromptRitual()

	updated, _ := m.Update(ritualTriggerMsg{ritual: r})
	m = updated.(Model)

	if !m.ritualOvl.IsActive() {
		t.Fatal("expected ritual overlay to be active after ritualTriggerMsg")
	}

	view := stripANSI(m.View().Content)
	if !strings.Contains(view, r.Name) {
		t.Errorf("expected View() to render the ritual name %q; overlay is active but not composited into the frame", r.Name)
	}
	if !strings.Contains(view, "How are you feeling today?") {
		t.Errorf("expected View() to render the prompt step text; got:\n%s", view)
	}
}

// TestRitualOverlay_TriggerReachesHandler confirms that experiment's rewritten
// Update routing does not intercept a ritualTriggerMsg before it reaches its
// case. ritualTriggerMsg is a non-key message, so the capture-bar, log-overlay,
// filter, and palette guards must all fall through to the ritual handler.
func TestRitualOverlay_TriggerReachesHandler(t *testing.T) {
	m := newRitualTestModel(t)

	if m.ritualOvl.IsActive() {
		t.Fatal("overlay should be inactive before trigger")
	}

	updated, _ := m.Update(ritualTriggerMsg{ritual: nudgePromptRitual()})
	m = updated.(Model)

	if !m.ritualOvl.IsActive() {
		t.Fatal("ritualTriggerMsg did not reach its handler — overlay never opened")
	}
}

// TestRitualOverlay_DismissClosesAndRecords drives the full lifecycle: trigger,
// then dismiss a nudge ritual with Escape. It asserts the overlay closes, the
// scheduler records the dismissal (so the ritual is not re-triggered the same
// day), and the Update returns a non-nil command (the tick-restart batch).
func TestRitualOverlay_DismissClosesAndRecords(t *testing.T) {
	m := newRitualTestModel(t)
	r := nudgePromptRitual()

	updated, _ := m.Update(ritualTriggerMsg{ritual: r})
	m = updated.(Model)
	if !m.ritualOvl.IsActive() {
		t.Fatal("expected overlay active before dismiss")
	}

	// Escape dismisses a nudge ritual (non-gate), routing through the overlay's
	// TryDefer path.
	updated, cmd := m.Update(tea.KeyPressMsg{Code: tea.KeyEsc})
	m = updated.(Model)

	if m.ritualOvl.IsActive() {
		t.Error("expected overlay to close after Escape on a nudge ritual")
	}
	if cmd == nil {
		t.Error("expected a non-nil command (tick restart) after overlay close")
	}
	if !m.schedulerState.IsDismissed(r.Name, m.clock.Now()) {
		t.Error("expected the scheduler to record the ritual as dismissed after close")
	}
}

// TestRitualOverlay_ConsumesKeyInput confirms that while the overlay is active,
// key input is routed to it first (the IsActive guard at the top of Update) and
// does not leak into experiment's capture/form routing.
func TestRitualOverlay_ConsumesKeyInput(t *testing.T) {
	m := newRitualTestModel(t)

	updated, _ := m.Update(ritualTriggerMsg{ritual: nudgePromptRitual()})
	m = updated.(Model)

	// A printable key on a prompt step is captured by the overlay's text input;
	// it must not trigger any pane-level navigation. The overlay stays active.
	updated, _ = m.Update(tea.KeyPressMsg{Code: 'x'})
	m = updated.(Model)

	if !m.ritualOvl.IsActive() {
		t.Fatal("overlay should remain active after a non-dismiss key")
	}
}
