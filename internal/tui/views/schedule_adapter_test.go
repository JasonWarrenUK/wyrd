package views

import (
	"testing"
	"time"

	"github.com/jasonwarrenuk/wyrd/internal/types"
)

func TestEntriesFromQueryResult_DefaultColumns(t *testing.T) {
	result := types.QueryResult{
		Columns: []string{"id", "title", "start", "duration", "energy", "is_calendar_event"},
		Rows: []map[string]interface{}{
			{
				"id":                "task-1",
				"title":             "EPA review",
				"start":             "2026-03-19T08:00:00",
				"duration":          float64(90),
				"energy":            "deep",
				"is_calendar_event": false,
			},
		},
	}

	entries := EntriesFromQueryResult(result, ScheduleColumns{})
	if len(entries) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(entries))
	}

	e := entries[0]
	if e.ID != "task-1" {
		t.Errorf("ID = %q, want task-1", e.ID)
	}
	if e.Title != "EPA review" {
		t.Errorf("Title = %q, want EPA review", e.Title)
	}
	wantStart := time.Date(2026, 3, 19, 8, 0, 0, 0, time.UTC)
	if !e.Start.Equal(wantStart) {
		t.Errorf("Start = %v, want %v", e.Start, wantStart)
	}
	if e.Duration != 90*time.Minute {
		t.Errorf("Duration = %v, want 90m", e.Duration)
	}
	if e.Energy != EnergyDeep {
		t.Errorf("Energy = %q, want deep", e.Energy)
	}
	if e.IsCalendarEvent {
		t.Errorf("IsCalendarEvent = true, want false")
	}
}

func TestEntriesFromQueryResult_DurationString(t *testing.T) {
	result := types.QueryResult{
		Rows: []map[string]interface{}{
			{"id": "task-1", "start": "2026-03-19T08:00:00", "duration": "45m"},
		},
	}
	entries := EntriesFromQueryResult(result, ScheduleColumns{})
	if entries[0].Duration != 45*time.Minute {
		t.Errorf("Duration = %v, want 45m", entries[0].Duration)
	}
}

func TestEntriesFromQueryResult_MissingIDOrStartSkipsRow(t *testing.T) {
	result := types.QueryResult{
		Rows: []map[string]interface{}{
			{"title": "no id or start"},
			{"id": "task-2"}, // no start
			{"start": "2026-03-19T08:00:00"}, // no id
			{"id": "task-3", "start": "2026-03-19T09:00:00"},
		},
	}
	entries := EntriesFromQueryResult(result, ScheduleColumns{})
	if len(entries) != 1 {
		t.Fatalf("expected 1 valid entry, got %d", len(entries))
	}
	if entries[0].ID != "task-3" {
		t.Errorf("ID = %q, want task-3", entries[0].ID)
	}
}

func TestEntriesFromQueryResult_UnrecognisedEnergyDefaultsLow(t *testing.T) {
	result := types.QueryResult{
		Rows: []map[string]interface{}{
			{"id": "task-1", "start": "2026-03-19T08:00:00", "energy": "extreme"},
			{"id": "task-2", "start": "2026-03-19T09:00:00"},
		},
	}
	entries := EntriesFromQueryResult(result, ScheduleColumns{})
	for _, e := range entries {
		if e.Energy != EnergyLow {
			t.Errorf("Energy = %q, want low default for entry %q", e.Energy, e.ID)
		}
	}
}

func TestEntriesFromQueryResult_CustomColumns(t *testing.T) {
	result := types.QueryResult{
		Rows: []map[string]interface{}{
			{"node_id": "task-1", "label": "Custom", "when": "2026-03-19T08:00:00"},
		},
	}
	entries := EntriesFromQueryResult(result, ScheduleColumns{ID: "node_id", Title: "label", Start: "when"})
	if len(entries) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(entries))
	}
	if entries[0].ID != "task-1" || entries[0].Title != "Custom" {
		t.Errorf("got ID=%q Title=%q, want task-1/Custom", entries[0].ID, entries[0].Title)
	}
}

func TestEntriesFromQueryResult_CalendarEventBoolFromString(t *testing.T) {
	result := types.QueryResult{
		Rows: []map[string]interface{}{
			{"id": "cal-1", "start": "2026-03-19T08:00:00", "is_calendar_event": "true"},
		},
	}
	entries := EntriesFromQueryResult(result, ScheduleColumns{})
	if !entries[0].IsCalendarEvent {
		t.Errorf("expected IsCalendarEvent true from string \"true\"")
	}
}

func TestEntriesFromQueryResult_EmptyRows(t *testing.T) {
	entries := EntriesFromQueryResult(types.QueryResult{}, ScheduleColumns{})
	if len(entries) != 0 {
		t.Errorf("expected 0 entries for empty result, got %d", len(entries))
	}
}
