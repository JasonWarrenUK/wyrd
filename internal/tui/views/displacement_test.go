package views

import (
	"testing"
	"time"
)

// stubClock satisfies types.Clock for tests.
type stubClock struct{ fixed time.Time }

func (c stubClock) Now() time.Time { return c.fixed }

func makeEntry(id, title string, start time.Time, duration time.Duration, energy EnergyLevel) ScheduleEntry {
	return ScheduleEntry{
		ID:       id,
		Title:    title,
		Start:    start,
		Duration: duration,
		Energy:   energy,
	}
}

func TestCalculate_NoActiveEntry(t *testing.T) {
	now := time.Date(2026, 3, 19, 9, 0, 0, 0, time.UTC)
	entries := []ScheduleEntry{
		makeEntry("a", "Task A", now, 30*time.Minute, EnergyDeep),
		makeEntry("b", "Task B", now.Add(30*time.Minute), 45*time.Minute, EnergyMedium),
	}
	result := Calculate(DisplacementInput{
		Entries: entries,
		Clock:   stubClock{now},
		DayEnd:  now.Add(8 * time.Hour),
	})

	if result.OverrunDuration != 0 {
		t.Errorf("expected zero overrun, got %s", result.OverrunDuration)
	}
	if result.DisplacedCount != 0 {
		t.Errorf("expected zero displaced, got %d", result.DisplacedCount)
	}
	if len(result.Entries) != 2 {
		t.Fatalf("expected 2 entries, got %d", len(result.Entries))
	}
	for i, de := range result.Entries {
		if de.IsDisplaced {
			t.Errorf("entry %d should not be displaced", i)
		}
		if de.CompressedDuration != entries[i].Duration {
			t.Errorf("entry %d: expected %s, got %s", i, entries[i].Duration, de.CompressedDuration)
		}
	}
}

func TestCalculate_OnTime(t *testing.T) {
	now := time.Date(2026, 3, 19, 9, 0, 0, 0, time.UTC)
	entries := []ScheduleEntry{
		makeEntry("a", "Task A", now, 30*time.Minute, EnergyDeep),
		makeEntry("b", "Task B", now.Add(30*time.Minute), 45*time.Minute, EnergyMedium),
	}
	result := Calculate(DisplacementInput{
		Entries:       entries,
		ActiveEntryID: "a",
		Elapsed:       25 * time.Minute,
		Clock:         stubClock{now},
		DayEnd:        now.Add(8 * time.Hour),
	})

	if result.OverrunDuration != 0 {
		t.Errorf("expected zero overrun when on time, got %s", result.OverrunDuration)
	}
	if result.DisplacedCount != 0 {
		t.Errorf("expected no displacement, got %d", result.DisplacedCount)
	}
}

func TestCalculate_OverrunCompressesSubsequentTasks(t *testing.T) {
	now := time.Date(2026, 3, 19, 9, 0, 0, 0, time.UTC)
	entries := []ScheduleEntry{
		makeEntry("active", "EPA review", now, 45*time.Minute, EnergyDeep),
		makeEntry("b", "Rhea sprint", now.Add(45*time.Minute), 30*time.Minute, EnergyMedium),
		makeEntry("c", "Triage", now.Add(75*time.Minute), 30*time.Minute, EnergyLow),
	}
	result := Calculate(DisplacementInput{
		Entries:       entries,
		ActiveEntryID: "active",
		Elapsed:       60 * time.Minute,
		Clock:         stubClock{now},
		DayEnd:        now.Add(8 * time.Hour),
	})

	if result.OverrunDuration != 15*time.Minute {
		t.Errorf("expected 15m overrun, got %s", result.OverrunDuration)
	}
	if result.DisplacedCount != 0 {
		t.Errorf("expected no fully displaced tasks, got %d", result.DisplacedCount)
	}
	for i := 1; i < 3; i++ {
		de := result.Entries[i]
		if de.IsDisplaced {
			t.Errorf("entry %d should not be fully displaced", i)
		}
		if de.CompressedDuration >= entries[i].Duration {
			t.Errorf("entry %d: expected compression, got %s >= %s", i, de.CompressedDuration, entries[i].Duration)
		}
	}
}

func TestCalculate_LargeOverrunDisplacesTasks(t *testing.T) {
	now := time.Date(2026, 3, 19, 9, 0, 0, 0, time.UTC)
	entries := []ScheduleEntry{
		makeEntry("active", "EPA review", now, 30*time.Minute, EnergyDeep),
		makeEntry("b", "Task B", now.Add(30*time.Minute), 30*time.Minute, EnergyMedium),
		makeEntry("c", "Task C", now.Add(60*time.Minute), 30*time.Minute, EnergyLow),
	}
	result := Calculate(DisplacementInput{
		Entries:       entries,
		ActiveEntryID: "active",
		Elapsed:       90 * time.Minute,
		Clock:         stubClock{now},
		DayEnd:        now.Add(8 * time.Hour),
	})

	if result.OverrunDuration != 60*time.Minute {
		t.Errorf("expected 60m overrun, got %s", result.OverrunDuration)
	}
	if result.DisplacedCount != 2 {
		t.Errorf("expected 2 displaced tasks, got %d", result.DisplacedCount)
	}
	for i := 1; i < 3; i++ {
		if !result.Entries[i].IsDisplaced {
			t.Errorf("entry %d should be displaced", i)
		}
	}
}

func TestCalculate_CalendarEventsAreImmovable(t *testing.T) {
	now := time.Date(2026, 3, 19, 9, 0, 0, 0, time.UTC)
	calEntry := ScheduleEntry{
		ID: "cal-standup", Title: "Stand-up",
		Start: now.Add(45 * time.Minute), Duration: 15 * time.Minute,
		IsCalendarEvent: true,
	}
	entries := []ScheduleEntry{
		makeEntry("active", "Deep work", now, 30*time.Minute, EnergyDeep),
		calEntry,
		makeEntry("c", "Triage", now.Add(60*time.Minute), 30*time.Minute, EnergyLow),
	}
	result := Calculate(DisplacementInput{
		Entries: entries, ActiveEntryID: "active",
		Elapsed: 60 * time.Minute, Clock: stubClock{now},
		DayEnd: now.Add(8 * time.Hour),
	})

	calResult := result.Entries[1]
	if calResult.IsDisplaced {
		t.Error("calendar event should not be displaced")
	}
	if calResult.CompressedDuration != calEntry.Duration {
		t.Errorf("calendar duration should be unchanged: got %s", calResult.CompressedDuration)
	}
}

func TestFormatSummaryLine(t *testing.T) {
	cases := []struct {
		r    DisplacementResult
		want string
	}{
		{DisplacementResult{}, ""},
		{DisplacementResult{UnscheduledDuration: 45 * time.Minute}, "◆ 45m unscheduled"},
		{DisplacementResult{UnscheduledDuration: 2*time.Hour + 15*time.Minute, DisplacedCount: 3}, "◆ 2h15m unscheduled · 3 tasks displaced to tomorrow"},
		{DisplacementResult{UnscheduledDuration: 30 * time.Minute, DisplacedCount: 1}, "◆ 30m unscheduled · 1 task displaced to tomorrow"},
	}
	for _, c := range cases {
		got := c.r.FormatSummaryLine()
		if got != c.want {
			t.Errorf("got %q, want %q", got, c.want)
		}
	}
}

func TestFormatStatusBar(t *testing.T) {
	if got := FormatStatusBar("EPA", 30*time.Minute, 45*time.Minute); got != "" {
		t.Errorf("expected empty when on time, got %q", got)
	}
	want := "◆ 47m on EPA (45m planned)"
	if got := FormatStatusBar("EPA", 47*time.Minute, 45*time.Minute); got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestFormatDuration(t *testing.T) {
	cases := []struct {
		d    time.Duration
		want string
	}{
		{47 * time.Minute, "47m"},
		{2*time.Hour + 15*time.Minute, "2h15m"},
		{3 * time.Hour, "3h"},
		{90 * time.Second, "2m"},
	}
	for _, c := range cases {
		got := formatDuration(c.d)
		if got != c.want {
			t.Errorf("formatDuration(%s) = %q, want %q", c.d, got, c.want)
		}
	}
}
