package views

import (
	"strings"
	"testing"
	"time"
)

func baseResult(entries []ScheduleEntry) DisplacementResult {
	displaced := make([]DisplacedEntry, len(entries))
	for i, e := range entries {
		displaced[i] = DisplacedEntry{Entry: e, CompressedDuration: e.Duration}
	}
	return DisplacementResult{Entries: displaced}
}

func TestRender_EmptySchedule(t *testing.T) {
	r := NewScheduleRenderer()
	out := r.Render(DisplacementResult{})
	if !strings.Contains(out, "No tasks scheduled") {
		t.Errorf("expected empty-state message, got %q", out)
	}
}

func TestRender_BasicSchedule(t *testing.T) {
	now := time.Date(2026, 3, 19, 8, 0, 0, 0, time.UTC)
	entries := []ScheduleEntry{
		makeEntry("a", "EPA review", now, 90*time.Minute, EnergyDeep),
		makeEntry("b", "Rhea sprint", now.Add(90*time.Minute), 60*time.Minute, EnergyMedium),
		makeEntry("c", "Triage", now.Add(150*time.Minute), 45*time.Minute, EnergyLow),
	}
	r := NewScheduleRenderer()
	out := r.Render(baseResult(entries))

	if !strings.Contains(out, "08:00") {
		t.Errorf("expected 08:00 time label in output")
	}
	if !strings.Contains(out, "EPA review") {
		t.Errorf("expected 'EPA review' in output")
	}
}

func TestRender_CalendarEvent(t *testing.T) {
	now := time.Date(2026, 3, 19, 10, 0, 0, 0, time.UTC)
	cal := ScheduleEntry{
		ID: "cal-standup", Title: "Stand-up",
		Start: now, Duration: 15 * time.Minute, IsCalendarEvent: true,
	}
	r := NewScheduleRenderer()
	out := r.Render(baseResult([]ScheduleEntry{cal}))

	if !strings.Contains(out, "┄") {
		t.Errorf("expected dashed line ┄ for calendar event")
	}
	if !strings.Contains(out, "Stand-up") {
		t.Errorf("expected 'Stand-up' in output")
	}
}

func TestRender_DisplacedEntry(t *testing.T) {
	now := time.Date(2026, 3, 19, 9, 0, 0, 0, time.UTC)
	result := DisplacementResult{
		Entries: []DisplacedEntry{
			{Entry: makeEntry("a", "Active", now, 30*time.Minute, EnergyDeep), CompressedDuration: 30 * time.Minute},
			{Entry: makeEntry("b", "Bumped task", now.Add(30*time.Minute), 30*time.Minute, EnergyLow), CompressedDuration: 0, IsDisplaced: true},
		},
		DisplacedCount:      1,
		UnscheduledDuration: 30 * time.Minute,
	}
	r := NewScheduleRenderer()
	out := r.Render(result)

	if !strings.Contains(out, "◆") {
		t.Errorf("expected overflow glyph ◆ for displaced task")
	}
	if !strings.Contains(out, "Bumped task") {
		t.Errorf("expected 'Bumped task' in output")
	}
	if !strings.Contains(out, "30m unscheduled") {
		t.Errorf("expected summary line, got: %s", out)
	}
}

func TestRender_CompressedFillBar(t *testing.T) {
	now := time.Date(2026, 3, 19, 9, 0, 0, 0, time.UTC)
	entry := makeEntry("a", "Compressed", now, 60*time.Minute, EnergyDeep)
	result := DisplacementResult{
		Entries: []DisplacedEntry{
			{Entry: entry, CompressedDuration: 30 * time.Minute},
		},
	}
	r := NewScheduleRenderer()
	out := r.Render(result)

	deepFills := strings.Count(out, "▓")
	if deepFills != 4 {
		t.Errorf("expected 4 fill chars for 50%% compression, got %d", deepFills)
	}
}
