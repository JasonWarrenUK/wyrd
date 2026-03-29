package tui

import (
	"testing"

	"charm.land/bubbles/v2/list"
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

func TestDetectGroupCol_NoCategoryColumn(t *testing.T) {
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
		if got := toGroupLabel(input); got != want {
			t.Errorf("toGroupLabel(%q) = %q, want %q", input, got, want)
		}
	}
}

func TestToGroupLabel_Unknown(t *testing.T) {
	// Unknown values are capitalised and pluralised.
	got := toGroupLabel("sprint")
	want := "Sprints"
	if got != want {
		t.Errorf("toGroupLabel(%q) = %q, want %q", "sprint", got, want)
	}
}

func TestToGroupLabel_Empty(t *testing.T) {
	if got := toGroupLabel(""); got != "" {
		t.Errorf("expected empty string for empty input, got %q", got)
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
