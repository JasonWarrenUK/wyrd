package tui

import (
	"image/color"
	"regexp"
	"strconv"
	"strings"
	"testing"

	"charm.land/bubbles/v2/list"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/jasonwarrenuk/wyrd/internal/types"
	"github.com/mattn/go-runewidth"
)

// ansiTruecolourBgRe extracts the r;g;b triple from a truecolor BACKGROUND
// SGR component (e.g. matches inside "48;2;255;248;245"). Distinct from
// ansiTruecolourFgRe (focus_anim.go), which matches the foreground ("38;2;
// ...") component — an escape can carry both in one sequence, so the two
// must not be conflated.
var ansiTruecolourBgRe = regexp.MustCompile(`48;2;(\d+);(\d+);(\d+)`)

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

	items := rowsToItems(rows, cols, widths, "", "", "", 0)
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
// formatRowTitle / isBlockedRow — blocked glyph prefix (DL.2)
// ---------------------------------------------------------------------------

func TestFormatRowTitle_BlockedRowGetsGlyphPrefix(t *testing.T) {
	row := map[string]interface{}{"title": "Blocked task", "isBlocked": true}
	cols := []string{"title"}
	widths := []int{20}

	got := formatRowTitle(row, cols, widths, "✖", "", 0)
	if !strings.HasPrefix(got, "✖") {
		t.Errorf("expected title to start with blocked glyph, got %q", got)
	}
}

func TestFormatRowTitle_UnblockedRowNoPrefix(t *testing.T) {
	row := map[string]interface{}{"title": "Free task", "isBlocked": false}
	cols := []string{"title"}
	widths := []int{20}

	got := formatRowTitle(row, cols, widths, "✖", "", 0)
	if strings.HasPrefix(got, "✖") {
		t.Errorf("unblocked row should not carry the blocked glyph, got %q", got)
	}
}

func TestFormatRowTitle_EmptyGlyphDisablesPrefix(t *testing.T) {
	row := map[string]interface{}{"title": "Blocked task", "isBlocked": true}
	cols := []string{"title"}
	widths := []int{20}

	got := formatRowTitle(row, cols, widths, "", "", 0)
	if strings.HasPrefix(got, "✖") {
		t.Errorf("empty blockedGlyph should disable the prefix, got %q", got)
	}
}

func TestFormatRowTitle_IsBlockedColumnNotRenderedAsCell(t *testing.T) {
	row := map[string]interface{}{"title": "Task", "isBlocked": true}
	cols := []string{"title", "isBlocked"}
	widths := []int{20, 4}

	// blockedGlyph is "" here, so the row carries a blank gutter, not the glyph.
	got := formatRowTitle(row, cols, widths, "", "", 0)
	want := glyphGutter(row, "", "", 0) + listPadOrTruncate("Task", 20)
	if got != want {
		t.Errorf("isBlocked should never render as a text cell, got %q, want %q", got, want)
	}
}

func TestIsBlockedRow(t *testing.T) {
	if isBlockedRow(map[string]interface{}{"isBlocked": true}) != true {
		t.Error("expected true for isBlocked=true")
	}
	if isBlockedRow(map[string]interface{}{"isBlocked": false}) != false {
		t.Error("expected false for isBlocked=false")
	}
	if isBlockedRow(map[string]interface{}{}) != false {
		t.Error("expected false when isBlocked is absent")
	}
	if isBlockedRow(map[string]interface{}{"isBlocked": "not-a-bool"}) != false {
		t.Error("expected false for non-bool isBlocked value")
	}
}

// ---------------------------------------------------------------------------
// formatRowTitle / isStaleRow — stale glyph prefix (DL.3)
// ---------------------------------------------------------------------------

func TestFormatRowTitle_StaleRowGetsGlyphPrefix(t *testing.T) {
	row := map[string]interface{}{"title": "Idle task", "daysSinceModified": 21}
	cols := []string{"title"}
	widths := []int{20}

	got := formatRowTitle(row, cols, widths, "", "◌", 14)
	if !strings.HasPrefix(got, "◌") {
		t.Errorf("expected title to start with stale glyph, got %q", got)
	}
}

func TestFormatRowTitle_FreshRowNoStalePrefix(t *testing.T) {
	row := map[string]interface{}{"title": "Fresh task", "daysSinceModified": 3}
	cols := []string{"title"}
	widths := []int{20}

	got := formatRowTitle(row, cols, widths, "", "◌", 14)
	if strings.HasPrefix(got, "◌") {
		t.Errorf("fresh row should not carry the stale glyph, got %q", got)
	}
}

func TestFormatRowTitle_EmptyStaleGlyphDisablesPrefix(t *testing.T) {
	row := map[string]interface{}{"title": "Idle task", "daysSinceModified": 21}
	cols := []string{"title"}
	widths := []int{20}

	got := formatRowTitle(row, cols, widths, "", "", 14)
	if strings.HasPrefix(got, "◌") {
		t.Errorf("empty staleGlyph should disable the prefix, got %q", got)
	}
}

func TestFormatRowTitle_DaysSinceModifiedColumnNotRenderedAsCell(t *testing.T) {
	row := map[string]interface{}{"title": "Task", "daysSinceModified": 21}
	cols := []string{"title", "daysSinceModified"}
	widths := []int{20, 4}

	// staleGlyph is "" here, so the row carries a blank gutter, not the glyph.
	got := formatRowTitle(row, cols, widths, "", "", 0)
	want := glyphGutter(row, "", "", 0) + listPadOrTruncate("Task", 20)
	if got != want {
		t.Errorf("daysSinceModified should never render as a text cell, got %q, want %q", got, want)
	}
}

func TestFormatRowTitle_BlockedTakesPrecedenceOverStale(t *testing.T) {
	// Blocked is the louder signal; a row that is both blocked and stale shows
	// only the blocked glyph in the single fixed-width gutter slot — the two
	// glyphs no longer stack, since stacking made the row wider than the
	// width budget accounted for (the source of the misalignment/"..." bugs).
	row := map[string]interface{}{"title": "Idle blocked task", "isBlocked": true, "daysSinceModified": 21}
	cols := []string{"title"}
	widths := []int{20}

	got := formatRowTitle(row, cols, widths, "✖", "◌", 14)
	if !strings.HasPrefix(got, "✖ ") {
		t.Errorf("expected blocked glyph, got %q", got)
	}
	if strings.Contains(got, "◌") {
		t.Errorf("expected stale glyph to be suppressed when row is also blocked, got %q", got)
	}
}

func TestFormatRowTitle_GlyphedAndNonGlyphedRowsAlignColumns(t *testing.T) {
	// The category/title columns must start at the same offset regardless of
	// whether the row carries a glyph — the gutter is fixed-width so a
	// non-glyphed row's text isn't shifted left relative to a glyphed one.
	cols := []string{"title"}
	widths := []int{20}

	plain := formatRowTitle(map[string]interface{}{"title": "Plain task"}, cols, widths, "✖", "◌", 14)
	blocked := formatRowTitle(map[string]interface{}{"title": "Blocked task", "isBlocked": true}, cols, widths, "✖", "◌", 14)
	stale := formatRowTitle(map[string]interface{}{"title": "Stale task", "daysSinceModified": 21}, cols, widths, "✖", "◌", 14)

	// Measure the display-column offset of the title text, not the byte
	// index: the glyphs are multi-byte runes, so strings.Index would
	// misreport alignment even when the gutter is correctly fixed-width.
	plainOffset := runewidth.StringWidth(plain[:strings.IndexRune(plain, 'P')])
	blockedOffset := runewidth.StringWidth(blocked[:strings.IndexRune(blocked, 'B')])
	staleOffset := runewidth.StringWidth(stale[:strings.IndexRune(stale, 'S')])

	if plainOffset != blockedOffset || plainOffset != staleOffset {
		t.Errorf("expected all rows to start their title text at the same offset, got plain=%d blocked=%d stale=%d (plain=%q blocked=%q stale=%q)",
			plainOffset, blockedOffset, staleOffset, plain, blocked, stale)
	}
}

func TestGlyphGutter_FixedWidthRegardlessOfGlyph(t *testing.T) {
	blocked := glyphGutter(map[string]interface{}{"isBlocked": true}, "✖", "◌", 14)
	stale := glyphGutter(map[string]interface{}{"daysSinceModified": 21}, "✖", "◌", 14)
	blank := glyphGutter(map[string]interface{}{}, "✖", "◌", 14)

	bw := runewidth.StringWidth(blocked)
	sw := runewidth.StringWidth(stale)
	kw := runewidth.StringWidth(blank)

	if bw != sw || bw != kw {
		t.Errorf("expected all gutters to occupy the same display width, got blocked=%d stale=%d blank=%d (blocked=%q stale=%q blank=%q)",
			bw, sw, kw, blocked, stale, blank)
	}
}

// ---------------------------------------------------------------------------
// calculateColWidths / listPadOrTruncate
// ---------------------------------------------------------------------------

func TestCalculateColWidths_FullwidthCellNotUnderMeasured(t *testing.T) {
	// A fullwidth-heavy value (CJK) has a rune count far below its true
	// display width. If calculateColWidths measured by rune count, it would
	// hand back a budget too narrow for the value's actual rendered width —
	// the same overflow-then-wrap bug the glyph gutter fix addressed for the
	// blocked/stale glyphs, just triggered by ordinary cell content instead.
	rows := []map[string]interface{}{
		{"title": "日本語タスク"}, // 6 runes, 12 display cells
	}
	cols := []string{"title"}
	widths := calculateColWidths(rows, cols, 80)

	rendered := listPadOrTruncate("日本語タスク", widths[0])
	if got := runewidth.StringWidth(rendered); got > widths[0] {
		t.Errorf("rendered display width %d exceeds allocated column budget %d — row would overflow the pane and wrap under PadLines", got, widths[0])
	}
}

func TestListPadOrTruncate_PadsAndTruncatesByDisplayWidth(t *testing.T) {
	// Padding a fullwidth string to a display-cell width must not add extra
	// runes beyond what's needed to reach that width in display cells.
	padded := listPadOrTruncate("日本語", 10) // 3 runes, 6 display cells
	if got := runewidth.StringWidth(padded); got != 10 {
		t.Errorf("expected padded display width 10, got %d (%q)", got, padded)
	}

	// Truncating a fullwidth string must cut at a display-cell boundary and
	// never exceed the requested width once the ellipsis is appended.
	truncated := listPadOrTruncate("日本語タスク", 8) // true width 12, budget 8
	if got := runewidth.StringWidth(truncated); got > 8 {
		t.Errorf("truncated display width %d exceeds requested width 8 (%q)", got, truncated)
	}
	if !strings.HasSuffix(truncated, listColEllipsis) {
		t.Errorf("expected truncated string to end with ellipsis, got %q", truncated)
	}
}

func TestIsStaleRow(t *testing.T) {
	if isStaleRow(map[string]interface{}{"daysSinceModified": 21}, 14) != true {
		t.Error("expected true when daysSinceModified exceeds the threshold")
	}
	if isStaleRow(map[string]interface{}{"daysSinceModified": 14}, 14) != false {
		t.Error("expected false at exactly the threshold (strict >)")
	}
	if isStaleRow(map[string]interface{}{"daysSinceModified": 3}, 14) != false {
		t.Error("expected false when daysSinceModified is below the threshold")
	}
	if isStaleRow(map[string]interface{}{}, 14) != false {
		t.Error("expected false when daysSinceModified is absent")
	}
	if isStaleRow(map[string]interface{}{"daysSinceModified": "not-an-int"}, 14) != false {
		t.Error("expected false for non-int daysSinceModified value")
	}
}

func TestIsStaleRow_ZeroThresholdResolvesToDefault(t *testing.T) {
	// 10 days idle should not be stale under the resolved default (14), only
	// under a threshold of exactly 0.
	if isStaleRow(map[string]interface{}{"daysSinceModified": 10}, 0) != false {
		t.Error("expected thresholdDays<=0 to resolve to the 14d default, not 0")
	}
	if isStaleRow(map[string]interface{}{"daysSinceModified": 15}, 0) != true {
		t.Error("expected a node idle 15 days to be stale under the resolved default of 14")
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

	items := rowsToItems(rows, cols, widths, "category", "", "", 0)
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

	items := rowsToItems(rows, cols, widths, "category", "", "", 0)
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

	items := rowsToItems(rows, cols, widths, "category", "", "", 0)
	// Expected: header(Journals), j1, header(Tasks), t1.
	if h, ok := items[0].(groupHeaderItem); !ok || h.label != "Journals" {
		t.Errorf("expected first header 'Journals', got %v", items[0])
	}
	if h, ok := items[2].(groupHeaderItem); !ok || h.label != "Tasks" {
		t.Errorf("expected second header 'Tasks', got %v", items[2])
	}
}

func TestRowsToItems_EmptyRows_WithGrouping(t *testing.T) {
	items := rowsToItems(nil, []string{"category", "title"}, []int{8, 10}, "category", "", "", 0)
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

	items := rowsToItems(rows, cols, widths, "kind", "", "", 0)
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

	items := rowsToItems(rows, cols, widths, "stage", "", "", 0)
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
	p := newNodeListPane(result, theme, 0)

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

// TestGroupedDelegateHeaderNoBackgroundBleed is a regression test: the group
// header row ("Tasks" etc.) is only as wide as its own text when the
// delegate hands it to bubbles/list — list.Model never pads delegate output
// to its own width. Left unpadded, nodeListPane.View()'s later
// lipgloss.JoinVertical (stacking this row under the wider column-header
// row) fills the gap with BARE, unstyled spaces to match the widest
// sibling line. Those spaces take on the terminal's default background
// rather than the theme's, which is invisible on dark themes (default bg
// happens to look black-ish already) but shows as a stark black bar on
// light themes (kiln, fell — both have a near-white bg.primary). The fix
// pads the header to the list's width, with the header's own background,
// inside the delegate itself, before JoinVertical ever sees it.
func TestGroupedDelegateHeaderNoBackgroundBleed(t *testing.T) {
	theme := testThemeVP2()
	delegate := newGroupedDelegate(theme)

	const listWidth = 40
	l := list.New([]list.Item{groupHeaderItem{label: "Tasks"}}, delegate, listWidth, 10)

	var buf strings.Builder
	delegate.Render(&buf, l, 0, groupHeaderItem{label: "Tasks"})
	out := buf.String()

	if got := lipgloss.Width(out); got != listWidth {
		t.Fatalf("rendered header width = %d, want %d (list width) — padding did not reach the full row",
			got, listWidth)
	}

	wantBg := theme.BgPrimary()
	cells := ansiBgCells(out)
	if len(cells) == 0 {
		t.Fatal("no cells parsed from rendered header — test setup problem")
	}
	for i, c := range cells {
		if c.colour == nil {
			t.Errorf("cell[%d] (char %q) has no background colour set — bare unstyled space, will bleed the terminal default", i, c.char)
			continue
		}
		if c.colour != wantBg {
			t.Errorf("cell[%d] (char %q) background = %v, want theme bg %v", i, c.char, c.colour, wantBg)
		}
	}
}

// ansiBgCells expands s into one entry per displayed rune, each carrying
// that position's BACKGROUND colour (nil if the position has no background
// escape active) — the background analogue of ansiColourCells
// (focus_anim.go), which tracks foreground.
func ansiBgCells(s string) []ansiCell {
	runs := ansiColourRuns(s)
	var cells []ansiCell
	for _, run := range runs {
		var bg color.Color
		if m := ansiTruecolourBgRe.FindStringSubmatch(run.escape); m != nil {
			r, _ := strconv.Atoi(m[1])
			g, _ := strconv.Atoi(m[2])
			b, _ := strconv.Atoi(m[3])
			bg = color.RGBA{R: uint8(r), G: uint8(g), B: uint8(b), A: 255}
		}
		for _, r := range run.text {
			cells = append(cells, ansiCell{escape: run.escape, char: r, colour: bg})
		}
	}
	return cells
}
