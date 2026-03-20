package ritual_test

import (
	"testing"
	"time"

	"github.com/jasonwarrenuk/wyrd/internal/tui/ritual"
	"github.com/jasonwarrenuk/wyrd/internal/types"
)

// ---------------------------------------------------------------------------
// SchedulerState tests
// ---------------------------------------------------------------------------

func TestSchedulerState_Dismiss(t *testing.T) {
	state := ritual.NewSchedulerState()
	now := time.Date(2026, 3, 19, 9, 0, 0, 0, time.UTC)

	state.Dismiss("morning", now)

	if !state.IsDismissed("morning", now) {
		t.Error("expected morning to be dismissed after Dismiss call")
	}
}

func TestSchedulerState_NotDismissed(t *testing.T) {
	state := ritual.NewSchedulerState()
	now := time.Date(2026, 3, 19, 9, 0, 0, 0, time.UTC)

	if state.IsDismissed("morning", now) {
		t.Error("expected morning to not be dismissed before any Dismiss call")
	}
}

func TestSchedulerState_NextDay(t *testing.T) {
	state := ritual.NewSchedulerState()

	today := time.Date(2026, 3, 19, 9, 0, 0, 0, time.UTC)
	tomorrow := time.Date(2026, 3, 20, 9, 0, 0, 0, time.UTC)

	state.Dismiss("morning", today)

	// Still dismissed on the same day.
	if !state.IsDismissed("morning", today) {
		t.Error("expected morning to be dismissed on the same day")
	}

	// No longer dismissed the next day.
	if state.IsDismissed("morning", tomorrow) {
		t.Error("expected morning to not be dismissed the following day")
	}
}

func TestSchedulerState_DifferentRituals(t *testing.T) {
	state := ritual.NewSchedulerState()
	now := time.Date(2026, 3, 19, 9, 0, 0, 0, time.UTC)

	state.Dismiss("morning", now)

	// Dismissing morning should not affect evening.
	if state.IsDismissed("evening", now) {
		t.Error("dismissing 'morning' should not dismiss 'evening'")
	}
}

// ---------------------------------------------------------------------------
// PendingRituals tests
// ---------------------------------------------------------------------------

func makeRitual(name string, days []string, timeStr string) *types.Ritual {
	return &types.Ritual{
		Name:     name,
		Friction: types.FrictionNudge,
		Schedule: types.RitualSchedule{
			Days: days,
			Time: timeStr,
		},
	}
}

// thursday09h is a Thursday at 09:00 UTC.
var thursday09h = time.Date(2026, 3, 19, 9, 0, 0, 0, time.UTC) // Thursday

func TestPendingRituals_FiltersDismissed(t *testing.T) {
	rituals := []*types.Ritual{
		makeRitual("morning", []string{"mon", "tue", "wed", "thu", "fri"}, "09:00"),
		makeRitual("midday", []string{"mon", "tue", "wed", "thu", "fri"}, "12:00"),
	}

	state := ritual.NewSchedulerState()
	clock := stubClock(thursday09h.Add(3 * time.Hour)) // 12:00, both due

	// Dismiss morning.
	state.Dismiss("morning", clock.Now())

	pending := ritual.PendingRituals(rituals, state, clock)

	for _, r := range pending {
		if r.Name == "morning" {
			t.Error("dismissed ritual 'morning' should not appear in pending")
		}
	}

	foundMidday := false
	for _, r := range pending {
		if r.Name == "midday" {
			foundMidday = true
		}
	}
	if !foundMidday {
		t.Error("undismissed ritual 'midday' should appear in pending")
	}
}

func TestPendingRituals_AllDue(t *testing.T) {
	rituals := []*types.Ritual{
		makeRitual("morning", []string{"thu"}, "09:00"),
		makeRitual("standup", []string{"thu"}, "09:30"),
	}

	state := ritual.NewSchedulerState()
	clock := stubClock(thursday09h.Add(time.Hour)) // 10:00, both due

	pending := ritual.PendingRituals(rituals, state, clock)
	if len(pending) != 2 {
		t.Errorf("expected 2 pending rituals when none dismissed, got %d", len(pending))
	}
}

func TestPendingRituals_EmptyWhenAllDismissed(t *testing.T) {
	rituals := []*types.Ritual{
		makeRitual("morning", []string{"thu"}, "09:00"),
	}

	state := ritual.NewSchedulerState()
	clock := stubClock(thursday09h.Add(time.Hour)) // 10:00, due

	state.Dismiss("morning", clock.Now())

	pending := ritual.PendingRituals(rituals, state, clock)
	if len(pending) != 0 {
		t.Errorf("expected 0 pending rituals when all dismissed, got %d", len(pending))
	}
}
