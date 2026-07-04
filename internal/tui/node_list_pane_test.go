package tui

import (
	"testing"

	"charm.land/bubbles/v2/list"
	tea "charm.land/bubbletea/v2"
	"github.com/jasonwarrenuk/wyrd/internal/types"
)

// ---------------------------------------------------------------------------
// detectGroupCol
// ---------------------------------------------------------------------------

func TestDetectGroupCol_CategoryPresent(t *testing.T) {
	cols := []string{"category", "title", "date"}
	if got := detectGroupCol(cols); got != "category" {
		t.Errorf("expected 'category', got %q", got)
	}
}

func TestDetectGroupCol_KindPresent(t *testing.T) {
	cols := []string{"title", "kind", "date"}
	if got := detectGroupCol(cols); got != "kind" {
		t.Errorf("expected 'kind', got %q", got)
	}
}

func TestDetectGroupCol_StagePresent(t *testing.T) {
	cols := []string{"title", "stage", "date"}
	if got := detectGroupCol(cols); got != "stage" {
		t.Errorf("expected 'stage', got %q", got)
	}
}

func TestDetectGroupCol_FirstGroupableWins(t *testing.T) {
	// When more than one groupable column is present, the first in column
	// order is chosen.
	cols := []string{"title", "kind", "category", "stage"}
	if got := detectGroupCol(cols); got != "kind" {
		t.Errorf("expected 'kind' (first groupable), got %q", got)
	}
}

func TestDetectGroupCol_NoGroupableColumn(t *testing.T) {
	cols := []string{"title", "status", "date"}
	if got := detectGroupCol(cols); got != "" {
		t.Errorf("expected empty string, got %q", got)
	}
}

func TestDetectGroupCol_Empty(t *testing.T) {
	if got := detectGroupCol(nil); got != "" {
		t.Errorf("expected empty string for nil cols, got %q", got)
	}
}

// ---------------------------------------------------------------------------
// toGroupLabel
// ---------------------------------------------------------------------------

func TestToGroupLabel_KnownLabels(t *testing.T) {
	cases := map[string]string{
		"task":    "Tasks",
		"note":    "Notes",
		"journal": "Journals",
		"TASK":    "Tasks",
		"NOTE":    "Notes",
	}
	for input, want := range cases {
		if got := toGroupLabel("category", input); got != want {
			t.Errorf("toGroupLabel(%q, %q) = %q, want %q", "category", input, got, want)
		}
	}
}

func TestToGroupLabel_Unknown(t *testing.T) {
	// Unknown values are capitalised and pluralised.
	got := toGroupLabel("category", "sprint")
	want := "Sprints"
	if got != want {
		t.Errorf("toGroupLabel(%q, %q) = %q, want %q", "category", "sprint", got, want)
	}
}

func TestToGroupLabel_Empty(t *testing.T) {
	if got := toGroupLabel("category", ""); got != "" {
		t.Errorf("expected empty string for empty input, got %q", got)
	}
}

func TestToGroupLabel_Kind(t *testing.T) {
	// Kind values share the category formatting: known values map to their
	// plural label, unknowns are capitalised and pluralised.
	cases := map[string]string{
		"task":  "Tasks",  // known map entry
		"event": "Events", // unknown, pluralised
		"TASK":  "Tasks",  // case-insensitive
		"":      "",       // empty stays empty
	}
	for input, want := range cases {
		if got := toGroupLabel("kind", input); got != want {
			t.Errorf("toGroupLabel(%q, %q) = %q, want %q", "kind", input, got, want)
		}
	}
}

func TestToGroupLabel_Stage(t *testing.T) {
	// Stage values are title-cased without pluralising; underscores become
	// spaces.
	cases := map[string]string{
		"now":         "Now",
		"open":        "Open",
		"later":       "Later",
		"in_progress": "In progress",
		"":            "",
	}
	for input, want := range cases {
		if got := toGroupLabel("stage", input); got != want {
			t.Errorf("toGroupLabel(%q, %q) = %q, want %q", "stage", input, got, want)
		}
	}
}

// ---------------------------------------------------------------------------
// groupHeaderItem
// ---------------------------------------------------------------------------

func TestGroupHeaderItem_FilterValue(t *testing.T) {
	h := groupHeaderItem{label: "Tasks"}
	if got := h.FilterValue(); got != "" {
		t.Errorf("FilterValue should be empty, got %q", got)
	}
}

func TestGroupHeaderItem_Title(t *testing.T) {
	h := groupHeaderItem{label: "Notes"}
	if got := h.Title(); got != "Notes" {
		t.Errorf("Title should be 'Notes', got %q", got)
	}
}

// ---------------------------------------------------------------------------
// rowsToItems — no grouping
// ---------------------------------------------------------------------------

func TestRowsToItems_NoGrouping(t *testing.T) {
	rows := []map[string]interface{}{
		{"id": "a", "title": "Alpha"},
		{"id": "b", "title": "Beta"},
	}
	cols := []string{"title"}
	widths := []int{10}

	items := rowsToItems(rows, cols, widths, "")
	if len(items) != 2 {
		t.Fatalf("expected 2 items, got %d", len(items))
	}
	for _, item := range items {
		if _, ok := item.(nodeListItem); !ok {
			t.Errorf("expected nodeListItem, got %T", item)
		}
	}
}

// ---------------------------------------------------------------------------
// rowsToItems — with grouping
// ---------------------------------------------------------------------------

func TestRowsToItems_WithGrouping_InsertsHeaders(t *testing.T) {
	rows := []map[string]interface{}{
		{"id": "t1", "title": "Task 1", "category": "task"},
		{"id": "t2", "title": "Task 2", "category": "task"},
		{"id": "n1", "title": "Note 1", "category": "note"},
	}
	cols := []string{"category", "title"}
	widths := []int{8, 10}

	items := rowsToItems(rows, cols, widths, "category")
	// Expected: header(Tasks), t1, t2, header(Notes), n1 = 5 items.
	if len(items) != 5 {
		t.Fatalf("expected 5 items (2 headers + 3 data), got %d", len(items))
	}

	if h, ok := items[0].(groupHeaderItem); !ok {
		t.Errorf("items[0] should be groupHeaderItem, got %T", items[0])
	} else if h.label != "Tasks" {
		t.Errorf("expected header 'Tasks', got %q", h.label)
	}
	if _, ok := items[1].(nodeListItem); !ok {
		t.Errorf("items[1] should be nodeListItem, got %T", items[1])
	}
	if _, ok := items[2].(nodeListItem); !ok {
		t.Errorf("items[2] should be nodeListItem, got %T", items[2])
	}
	if h, ok := items[3].(groupHeaderItem); !ok {
		t.Errorf("items[3] should be groupHeaderItem, got %T", items[3])
	} else if h.label != "Notes" {
		t.Errorf("expected header 'Notes', got %q", h.label)
	}
	if _, ok := items[4].(nodeListItem); !ok {
		t.Errorf("items[4] should be nodeListItem, got %T", items[4])
	}
}

func TestRowsToItems_SingleGroup(t *testing.T) {
	rows := []map[string]interface{}{
		{"id": "t1", "title": "Task 1", "category": "task"},
		{"id": "t2", "title": "Task 2", "category": "task"},
	}
	cols := []string{"category", "title"}
	widths := []int{8, 10}

	items := rowsToItems(rows, cols, widths, "category")
	// header + 2 data = 3.
	if len(items) != 3 {
		t.Fatalf("expected 3 items (1 header + 2 data), got %d", len(items))
	}
	if _, ok := items[0].(groupHeaderItem); !ok {
		t.Errorf("items[0] should be groupHeaderItem, got %T", items[0])
	}
}

func TestRowsToItems_PreservesGroupOrder(t *testing.T) {
	// Groups should appear in first-occurrence order, not sorted.
	rows := []map[string]interface{}{
		{"id": "j1", "title": "Journal 1", "category": "journal"},
		{"id": "t1", "title": "Task 1", "category": "task"},
	}
	cols := []string{"category", "title"}
	widths := []int{8, 10}

	items := rowsToItems(rows, cols, widths, "category")
	// Expected: header(Journals), j1, header(Tasks), t1.
	if h, ok := items[0].(groupHeaderItem); !ok || h.label != "Journals" {
		t.Errorf("expected first header 'Journals', got %v", items[0])
	}
	if h, ok := items[2].(groupHeaderItem); !ok || h.label != "Tasks" {
		t.Errorf("expected second header 'Tasks', got %v", items[2])
	}
}

func TestRowsToItems_EmptyRows_WithGrouping(t *testing.T) {
	items := rowsToItems(nil, []string{"category", "title"}, []int{8, 10}, "category")
	if len(items) != 0 {
		t.Errorf("expected 0 items for empty rows, got %d", len(items))
	}
}

func TestRowsToItems_GroupByKind(t *testing.T) {
	rows := []map[string]interface{}{
		{"id": "t1", "title": "Task 1", "kind": "task"},
		{"id": "t2", "title": "Task 2", "kind": "task"},
		{"id": "e1", "title": "Event 1", "kind": "event"},
	}
	cols := []string{"kind", "title"}
	widths := []int{8, 10}

	items := rowsToItems(rows, cols, widths, "kind")
	// Expected: header(Tasks), t1, t2, header(Events), e1 = 5 items.
	if len(items) != 5 {
		t.Fatalf("expected 5 items (2 headers + 3 data), got %d", len(items))
	}
	if h, ok := items[0].(groupHeaderItem); !ok || h.label != "Tasks" {
		t.Errorf("expected header 'Tasks', got %v", items[0])
	}
	if h, ok := items[3].(groupHeaderItem); !ok || h.label != "Events" {
		t.Errorf("expected header 'Events', got %v", items[3])
	}
}

func TestRowsToItems_GroupByStage(t *testing.T) {
	rows := []map[string]interface{}{
		{"id": "a", "title": "Alpha", "stage": "now"},
		{"id": "b", "title": "Beta", "stage": "now"},
		{"id": "c", "title": "Gamma", "stage": "later"},
	}
	cols := []string{"stage", "title"}
	widths := []int{8, 10}

	items := rowsToItems(rows, cols, widths, "stage")
	// Expected: header(Now), a, b, header(Later), c = 5 items.
	if len(items) != 5 {
		t.Fatalf("expected 5 items (2 headers + 3 data), got %d", len(items))
	}
	// Stage headers are title-cased, not pluralised.
	if h, ok := items[0].(groupHeaderItem); !ok || h.label != "Now" {
		t.Errorf("expected header 'Now', got %v", items[0])
	}
	if h, ok := items[3].(groupHeaderItem); !ok || h.label != "Later" {
		t.Errorf("expected header 'Later', got %v", items[3])
	}
}

// ---------------------------------------------------------------------------
// groupHeaderItem is excluded from filter
// ---------------------------------------------------------------------------

func TestGroupHeaderItem_ExcludedFromFilter(t *testing.T) {
	// bubbles/list uses FilterValue() to decide filter matching.
	// groupHeaderItem.FilterValue() must return "" so headers are hidden.
	h := groupHeaderItem{label: "Tasks"}
	var i list.Item = h
	di, ok := i.(list.DefaultItem)
	if !ok {
		t.Fatal("groupHeaderItem should satisfy list.DefaultItem")
	}
	if di.FilterValue() != "" {
		t.Errorf("expected empty FilterValue, got %q", di.FilterValue())
	}
}

// ---------------------------------------------------------------------------
// nodeListItem NodeID
// ---------------------------------------------------------------------------

func TestNodeListItem_NodeID(t *testing.T) {
	item := nodeListItem{id: "abc-123", title: "Test"}
	if got := item.NodeID(); got != "abc-123" {
		t.Errorf("expected 'abc-123', got %q", got)
	}
}

func TestNodeListItem_NodeID_Empty(t *testing.T) {
	item := nodeListItem{title: "No ID"}
	if got := item.NodeID(); got != "" {
		t.Errorf("expected empty NodeID, got %q", got)
	}
}

// ---------------------------------------------------------------------------
// selection preservation across resize
// ---------------------------------------------------------------------------

// TestNodeListPane_ResizePreservesSelection verifies that sending a
// WindowSizeMsg does not reset the list cursor to the first data row.
// Previously, skipInitialHeaders ran unconditionally after SetItems and yanked
// any valid mid-list selection back to index 0.
func TestNodeListPane_ResizePreservesSelection(t *testing.T) {
	rows := []map[string]interface{}{
		{"id": "n1", "title": "Alpha", "category": "task"},
		{"id": "n2", "title": "Beta", "category": "task"},
		{"id": "n3", "title": "Gamma", "category": "task"},
	}
	result := types.QueryResult{
		Columns: []string{"category", "title"},
		Rows:    rows,
	}
	theme := testThemeVP2()
	p := newNodeListPane(result, theme)

	// Move the cursor to the second data row (index 1 in an ungrouped list).
	p.list.Select(1)
	wantID := p.SelectedNodeID()
	if wantID == "" {
		t.Fatal("expected a non-empty SelectedNodeID after Select(1)")
	}

	// Send a resize — this must not reset the selection.
	updated, _ := p.Update(tea.WindowSizeMsg{Width: 120, Height: 40})
	pAfter, ok := updated.(nodeListPane)
	if !ok {
		t.Fatalf("Update returned unexpected type %T", updated)
	}

	if got := pAfter.SelectedNodeID(); got != wantID {
		t.Errorf("selection changed after resize: want %q, got %q", wantID, got)
	}
}
