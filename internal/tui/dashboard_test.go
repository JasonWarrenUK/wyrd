package tui

import (
	"testing"
	"time"

	"github.com/jasonwarrenuk/wyrd/internal/types"
)

// stubRunner is a minimal QueryRunner for dashboard tests.
// It maps query strings to canned results.
type stubRunner struct {
	results map[string]*types.QueryResult
	err     error
}

func (s *stubRunner) Run(q string, _ types.Clock) (*types.QueryResult, error) {
	if s.err != nil {
		return nil, s.err
	}
	if r, ok := s.results[q]; ok {
		return r, nil
	}
	// No match: return empty result.
	return &types.QueryResult{Columns: []string{}, Rows: nil}, nil
}

// row is a helper to build a result row.
func row(category, title string, date interface{}) map[string]interface{} {
	return map[string]interface{}{
		"category": category,
		"title":    title,
		"date":     date,
		"id":       "dummy-id",
	}
}

// rowT is an alias for row used in tests that reference the updated query strings.
var rowT = row

// date is a helper to create a time.Time from a date string.
func date(s string) time.Time {
	t, _ := time.Parse("2006-01-02", s)
	return t
}

// TestRunDashboard_MergesAndOrders verifies that tasks, notes, and journals are
// merged in the correct order with tasks+notes sorted by ascending date and
// journals appended in ascending chronological order (i.e. reversed from the
// DESC query result).
func TestRunDashboard_MergesAndOrders(t *testing.T) {
	cfg := DefaultDashboardQuery()
	clock := types.StubClock{Fixed: date("2026-03-20")}

	runner := &stubRunner{
		results: map[string]*types.QueryResult{
			cfg.Tasks: {
				Columns: []string{"id", "title", "date", "category"},
				Rows: []map[string]interface{}{
					rowT("task", "Task B", date("2026-03-18")),
					rowT("task", "Task A", date("2026-03-10")),
					rowT("task", "Task Undated", nil),
				},
			},
			cfg.Notes: {
				Columns: []string{"id", "title", "date", "category"},
				Rows: []map[string]interface{}{
					rowT("note", "Note today", nil),
				},
			},
			cfg.Journals: {
				Columns: []string{"id", "title", "date", "category"},
				// DESC order from query — most recent first.
				Rows: []map[string]interface{}{
					rowT("journal", "Journal 3", date("2026-03-19")),
					rowT("journal", "Journal 2", date("2026-03-15")),
					rowT("journal", "Journal 1", date("2026-03-01")),
				},
			},
		},
	}

	result, err := RunDashboard(runner, clock, cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Expect: 3 tasks + 1 note + 3 journals = 7 rows.
	if len(result.Rows) != 7 {
		t.Fatalf("expected 7 rows, got %d", len(result.Rows))
	}

	// Tasks sorted ascending by date; undated task sorts after dated tasks.
	if result.Rows[0]["title"] != "Task A" {
		t.Errorf("row 0: expected 'Task A', got %q", result.Rows[0]["title"])
	}
	if result.Rows[1]["title"] != "Task B" {
		t.Errorf("row 1: expected 'Task B', got %q", result.Rows[1]["title"])
	}
	if result.Rows[2]["title"] != "Task Undated" {
		t.Errorf("row 2: expected 'Task Undated', got %q", result.Rows[2]["title"])
	}

	// Note follows tasks.
	if result.Rows[3]["title"] != "Note today" {
		t.Errorf("row 3: expected 'Note today', got %q", result.Rows[3]["title"])
	}

	// Journals reversed to ascending chronological order.
	if result.Rows[4]["title"] != "Journal 1" {
		t.Errorf("row 4: expected 'Journal 1', got %q", result.Rows[4]["title"])
	}
	if result.Rows[6]["title"] != "Journal 3" {
		t.Errorf("row 6: expected 'Journal 3', got %q", result.Rows[6]["title"])
	}
}

// TestRunDashboard_Columns verifies that only the display columns are present
// in the result (id is excluded from display rows).
func TestRunDashboard_Columns(t *testing.T) {
	cfg := DefaultDashboardQuery()
	clock := types.StubClock{Fixed: date("2026-03-20")}

	runner := &stubRunner{
		results: map[string]*types.QueryResult{
			cfg.Tasks: {
				Columns: []string{"id", "title", "date", "category"},
				Rows:    []map[string]interface{}{row("task", "A task", date("2026-03-20"))},
			},
		},
	}

	result, err := RunDashboard(runner, clock, cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(result.Columns) != len(dashboardColumns) {
		t.Fatalf("expected %d columns, got %d", len(dashboardColumns), len(result.Columns))
	}

	for i, col := range dashboardColumns {
		if result.Columns[i] != col {
			t.Errorf("column %d: expected %q, got %q", i, col, result.Columns[i])
		}
	}

	// id must not appear in the projected row.
	if _, ok := result.Rows[0]["id"]; ok {
		t.Error("'id' should not appear in projected display rows")
	}
}

// TestRunDashboard_EmptyStore verifies that all three categories returning no
// rows still produces a valid (empty) result, not an error.
func TestRunDashboard_EmptyStore(t *testing.T) {
	cfg := DefaultDashboardQuery()
	clock := types.StubClock{Fixed: date("2026-03-20")}
	runner := &stubRunner{results: map[string]*types.QueryResult{}}

	result, err := RunDashboard(runner, clock, cfg)
	if err != nil {
		t.Fatalf("unexpected error on empty store: %v", err)
	}
	if len(result.Rows) != 0 {
		t.Errorf("expected 0 rows, got %d", len(result.Rows))
	}
}
