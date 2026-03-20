package tui

import (
	"fmt"
	"sort"
	"time"

	"github.com/jasonwarrenuk/wyrd/internal/types"
)

// dashboardColumns defines the display columns for the dashboard pane.
// id is intentionally excluded from display — it is carried in row data for
// future selection/navigation but not shown in the list. When WL.9 lands and
// nodes gain a first-class title field, replace "body" with "title" here.
var dashboardColumns = []string{"category", "body", "date"}

// DashboardQuery holds the three Cypher queries that together produce the
// default startup view. Each field can be replaced independently so that
// WL.7 (user-configurable dashboard via saved view) can override them.
//
// When WL.7 is implemented, a saved view named "dashboard" in the store
// should take precedence over these defaults.
type DashboardQuery struct {
	// Tasks selects active tasks due today or earlier.
	// Expected columns: id, body, date, category.
	Tasks string

	// Notes selects notes dated today.
	// Expected columns: id, body, date, category.
	Notes string

	// Journals selects the most recent journals.
	// Expected columns: id, body, date, category.
	Journals string
}

// DefaultDashboardQuery returns the hardcoded default queries used when no
// saved "dashboard" view is present in the store.
//
// The three queries are run separately because the query engine does not yet
// support UNION (see QE.1 in the roadmap). Once QE.1 lands, these can be
// collapsed into a single query and DashboardQuery can become a single string.
//
// Note: journals use n.created as the date because journal nodes do not yet
// have a dedicated date property (see WL.8). Once WL.8 is done, the journal
// query should prefer n.date with n.created as fallback.
func DefaultDashboardQuery() DashboardQuery {
	return DashboardQuery{
		Tasks: `MATCH (n:task)
WHERE n.status <> "archived" AND n.date <= $today
RETURN n.id AS id, n.body AS body, n.date AS date, "task" AS category
ORDER BY n.date`,

		Notes: `MATCH (n:note)
WHERE n.date = $today
RETURN n.id AS id, n.body AS body, n.date AS date, "note" AS category
ORDER BY n.date`,

		Journals: `MATCH (n:journal)
RETURN n.id AS id, n.body AS body, n.created AS date, "journal" AS category
ORDER BY n.created DESC
LIMIT 5`,
	}
}

// RunDashboard executes the three dashboard queries and merges their results
// into a single QueryResult grouped by category (tasks → notes → journals)
// with tasks and notes sorted by ascending date. Journal order is preserved
// as returned by the query (most-recent-first DESC from the store).
//
// If a query returns no rows or fails, that category is silently omitted
// rather than surfacing an error — a partially populated dashboard is more
// useful than a blank screen.
func RunDashboard(runner types.QueryRunner, clock types.Clock, cfg DashboardQuery) (types.QueryResult, error) {
	tasks, err := runCategory(runner, clock, cfg.Tasks)
	if err != nil {
		return types.QueryResult{}, fmt.Errorf("dashboard tasks query: %w", err)
	}

	notes, err := runCategory(runner, clock, cfg.Notes)
	if err != nil {
		return types.QueryResult{}, fmt.Errorf("dashboard notes query: %w", err)
	}

	journals, err := runCategory(runner, clock, cfg.Journals)
	if err != nil {
		return types.QueryResult{}, fmt.Errorf("dashboard journals query: %w", err)
	}

	// Sort tasks and notes by date ascending.
	sortByDate(tasks)
	sortByDate(notes)
	// Journals are already in DESC order from the query; reverse for most-recent-last
	// display so the grouping reads chronologically within the list.
	reverseRows(journals)

	merged := make([]map[string]interface{}, 0, len(tasks)+len(notes)+len(journals))
	merged = append(merged, tasks...)
	merged = append(merged, notes...)
	merged = append(merged, journals...)

	return types.QueryResult{
		Columns: dashboardColumns,
		Rows:    projectColumns(merged, dashboardColumns),
	}, nil
}

// runCategory executes a single category query. Returns nil rows (not an
// error) if the query yields no results.
func runCategory(runner types.QueryRunner, clock types.Clock, q string) ([]map[string]interface{}, error) {
	if q == "" {
		return nil, nil
	}
	result, err := runner.Run(q, clock)
	if err != nil {
		return nil, err
	}
	return result.Rows, nil
}

// sortByDate sorts rows ascending by their "date" field. Rows with no date
// (nil or unparseable) are placed after dated rows.
func sortByDate(rows []map[string]interface{}) {
	sort.SliceStable(rows, func(i, j int) bool {
		ti := parseDate(rows[i]["date"])
		tj := parseDate(rows[j]["date"])
		if ti == nil && tj == nil {
			return false
		}
		if ti == nil {
			return false
		}
		if tj == nil {
			return true
		}
		return ti.Before(*tj)
	})
}

// reverseRows reverses a slice of rows in-place.
func reverseRows(rows []map[string]interface{}) {
	for i, j := 0, len(rows)-1; i < j; i, j = i+1, j-1 {
		rows[i], rows[j] = rows[j], rows[i]
	}
}

// parseDate attempts to extract a time.Time from a row value. Returns nil if
// the value is absent or cannot be interpreted as a time.
func parseDate(v interface{}) *time.Time {
	if v == nil {
		return nil
	}
	switch t := v.(type) {
	case time.Time:
		return &t
	case *time.Time:
		return t
	case string:
		// Try common date formats.
		for _, layout := range []string{"2006-01-02", time.RFC3339} {
			if parsed, err := time.Parse(layout, t); err == nil {
				return &parsed
			}
		}
	}
	return nil
}

// projectColumns returns a new slice of rows containing only the specified
// columns, preserving order. Extra columns in the source rows are dropped.
func projectColumns(rows []map[string]interface{}, cols []string) []map[string]interface{} {
	out := make([]map[string]interface{}, len(rows))
	for i, row := range rows {
		projected := make(map[string]interface{}, len(cols))
		for _, col := range cols {
			projected[col] = row[col]
		}
		out[i] = projected
	}
	return out
}
