package tui

import (
	"testing"

	"charm.land/bubbles/v2/spinner"
	tea "charm.land/bubbletea/v2"
)

// overlayHarness names one of the five overlays under test, together with the
// operations needed to open it and check whether it is currently active. Used
// to drive the same assertions across all five without duplicating the
// open/check boilerplate per overlay — the five differ in how they open
// (Open(w,h) vs a triggered ritualTriggerMsg) but the routing contract being
// tested (declining a tick/resize/spinner message) is identical across all of
// them.
type overlayHarness struct {
	name     string
	open     func(t *testing.T, m Model) Model
	isActive func(m Model) bool
}

func overlayHarnesses(t *testing.T) []overlayHarness {
	t.Helper()
	return []overlayHarness{
		{
			name: "log",
			open: func(t *testing.T, m Model) Model {
				m.logOverlay.Open(m.layout.totalWidth, m.layout.totalHeight)
				return m
			},
			isActive: func(m Model) bool { return m.logOverlay.IsActive() },
		},
		{
			name: "help",
			open: func(t *testing.T, m Model) Model {
				m.helpOverlay.Open(m.layout.totalWidth, m.layout.totalHeight, m.keyMap.AllBindings())
				return m
			},
			isActive: func(m Model) bool { return m.helpOverlay.IsActive() },
		},
		{
			name: "kinds",
			open: func(t *testing.T, m Model) Model {
				m.kindsOverlay.Open(m.layout.totalWidth, m.layout.totalHeight)
				return m
			},
			isActive: func(m Model) bool { return m.kindsOverlay.IsActive() },
		},
		{
			name: "stages",
			open: func(t *testing.T, m Model) Model {
				m.stagesOverlay.Open(m.layout.totalWidth, m.layout.totalHeight)
				return m
			},
			isActive: func(m Model) bool { return m.stagesOverlay.IsActive() },
		},
		{
			name: "ritual",
			open: func(t *testing.T, m Model) Model {
				t.Helper()
				updated, _ := m.Update(ritualTriggerMsg{ritual: nudgePromptRitual()})
				got, ok := updated.(Model)
				if !ok {
					t.Fatalf("ritualTriggerMsg Update returned %T, want Model", updated)
				}
				if !got.ritualOvl.IsActive() {
					t.Fatal("ritual overlay did not activate from ritualTriggerMsg")
				}
				return got
			},
			isActive: func(m Model) bool { return m.ritualOvl.IsActive() },
		},
	}
}

// TestRitualTickSurvivesOpenOverlay is the regression test that matters most
// for TD.12: ritualCheckTickMsg is the only thing that re-arms the ritual
// scheduler (see ritualCheckTickMsg's handler), so any overlay that swallows
// it kills the scheduler for the rest of the session. Table over all five
// overlays — before the fix, the four viewport overlays consumed
// unconditionally and only the ritual overlay itself correctly declined.
func TestRitualTickSurvivesOpenOverlay(t *testing.T) {
	for _, h := range overlayHarnesses(t) {
		t.Run(h.name, func(t *testing.T) {
			m := newRitualTestModel(t)
			m = h.open(t, m)
			if !h.isActive(m) {
				t.Fatalf("%s overlay did not open", h.name)
			}

			updated, cmd := m.Update(ritualCheckTickMsg{})
			got, ok := updated.(Model)
			if !ok {
				t.Fatalf("Update returned %T, want Model", updated)
			}

			// The ritualCheckTickMsg handler always returns a non-nil command
			// (either a pending ritual trigger or the next tick), so a nil
			// command here means the message was swallowed by the overlay
			// guard rather than reaching the switch below it.
			if cmd == nil {
				t.Errorf("%s overlay swallowed ritualCheckTickMsg — scheduler tick did not reach its handler", h.name)
			}
			// The overlay that was already open must still be open — a
			// message it declines must not have side-effects on its own
			// active state.
			if !h.isActive(got) {
				t.Errorf("%s overlay closed unexpectedly after a declined message", h.name)
			}
		})
	}
}

// TestWindowSizeAppliesWhileOverlayOpen asserts a resize reaches the root
// tea.WindowSizeMsg handler (which updates m.layout) even while an overlay is
// open, rather than being consumed by the overlay guard.
func TestWindowSizeAppliesWhileOverlayOpen(t *testing.T) {
	for _, h := range overlayHarnesses(t) {
		t.Run(h.name, func(t *testing.T) {
			m := newRitualTestModel(t)
			m = h.open(t, m)
			if !h.isActive(m) {
				t.Fatalf("%s overlay did not open", h.name)
			}

			updated, _ := m.Update(tea.WindowSizeMsg{Width: 111, Height: 41})
			got, ok := updated.(Model)
			if !ok {
				t.Fatalf("Update returned %T, want Model", updated)
			}

			if got.layout.totalWidth != 111 || got.layout.totalHeight != 41 {
				t.Errorf("%s overlay blocked resize: layout is %dx%d, want 111x41",
					h.name, got.layout.totalWidth, got.layout.totalHeight)
			}
		})
	}
}

// TestSpinnerTickSurvivesOpenOverlay asserts a spinner.TickMsg reaches
// StatusBar.Update (driving a :sync spinner) even while an overlay is open.
func TestSpinnerTickSurvivesOpenOverlay(t *testing.T) {
	for _, h := range overlayHarnesses(t) {
		t.Run(h.name, func(t *testing.T) {
			m := newRitualTestModel(t)
			spinCmd := m.statusBar.StartSpinner("Syncing…")
			if spinCmd == nil {
				t.Fatal("StartSpinner returned a nil command")
			}
			m = h.open(t, m)
			if !h.isActive(m) {
				t.Fatalf("%s overlay did not open", h.name)
			}

			updated, cmd := m.Update(spinner.TickMsg{})
			if _, ok := updated.(Model); !ok {
				t.Fatalf("Update returned %T, want Model", updated)
			}
			if cmd == nil {
				t.Errorf("%s overlay swallowed spinner.TickMsg — spinner did not reach StatusBar.Update", h.name)
			}
		})
	}
}

// TestOverlaysStillConsumeKeys is the negative control for the routing fix:
// confirms that a recognised key (esc) is still consumed by whichever overlay
// is open, rather than the "decline non-key/mouse" change accidentally
// widening to decline everything.
func TestOverlaysStillConsumeKeys(t *testing.T) {
	for _, h := range overlayHarnesses(t) {
		t.Run(h.name, func(t *testing.T) {
			m := newRitualTestModel(t)
			m = h.open(t, m)
			if !h.isActive(m) {
				t.Fatalf("%s overlay did not open", h.name)
			}

			updated, _ := m.Update(tea.KeyPressMsg{Code: tea.KeyEsc})
			got, ok := updated.(Model)
			if !ok {
				t.Fatalf("Update returned %T, want Model", updated)
			}

			if h.isActive(got) {
				t.Errorf("%s overlay still active after Escape — key input is no longer being consumed", h.name)
			}
		})
	}
}

// TestPaletteStillDispatchesKindsCommand guards the load-bearing position of
// the palette guard (after the overlay-open switch, before the general
// switch): typing "kinds" and confirming must still open the kinds overlay on
// the *next* Update call, once the palette's own openKindsOverlayMsg command
// has run and the palette has closed. See the comment on the palette guard in
// update for why the ordering can't move.
func TestPaletteStillDispatchesKindsCommand(t *testing.T) {
	m := newRitualTestModel(t)

	// Open the CLI palette (ctrl+p) and type "kinds".
	updated, _ := m.Update(tea.KeyPressMsg{Code: 'p', Mod: tea.ModCtrl})
	m = updated.(Model)
	if !m.palette.IsActive() {
		t.Fatal("palette did not open on ctrl+p")
	}

	for _, r := range "kinds" {
		updated, _ = m.Update(tea.KeyPressMsg{Code: r, Text: string(r)})
		m = updated.(Model)
	}

	// Confirm — this runs the "kinds" command's Execute, which returns a cmd
	// emitting openKindsOverlayMsg; the palette itself closes.
	updated, cmd := m.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	m = updated.(Model)
	if m.palette.IsActive() {
		t.Fatal("palette still active after confirming a no-arg command")
	}
	if cmd == nil {
		t.Fatal("confirming \"kinds\" produced no command")
	}

	// Running the returned command yields the openKindsOverlayMsg, which the
	// next Update call routes to switch #1's case openKindsOverlayMsg.
	msg := cmd()
	updated, _ = m.Update(msg)
	m = updated.(Model)

	if !m.kindsOverlay.IsActive() {
		t.Fatal("kinds overlay did not open after dispatching the \"kinds\" palette command")
	}
}
