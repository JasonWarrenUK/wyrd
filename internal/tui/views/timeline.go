package views

import (
	"fmt"
	"image/color"
	"sort"
	"strings"
	"time"

	"charm.land/lipgloss/v2"
	"github.com/jasonwarrenuk/wyrd/internal/types"
)

const (
	// timelineDateFormat is the format used for timeline entry date headers.
	timelineDateFormat = "Monday 2 January 2006"
	// timelineDateFormatShort is the compact date label used in block mode.
	timelineDateFormatShort = "2 Jan 06"
	// timelineSeparator is the horizontal rule drawn between entries.
	timelineSeparator = "─"
	// timelineBlockAccentWidth is the width of the type-accent bar in block mode,
	// matching fillBarWidth in schedule.go.
	timelineBlockAccentWidth = 8
	// timelineLabelWidth is the fixed width of the date label column in block mode.
	timelineLabelWidth = 12
	// timelineBlockAccentChar is the fill character for the accent bar when the
	// entry has a known type with a colour.
	timelineBlockAccentChar = "█"
	// timelineBlockMutedChar is the fill character when no type colour is available.
	timelineBlockMutedChar = "░"
)

// TimelinePalette holds the colours used by the timeline renderer.
type TimelinePalette struct {
	// DateHeader is the foreground colour for the date heading.
	DateHeader color.Color
	// Separator is the colour of the horizontal rule between entries.
	Separator color.Color
	// Body is the default body text colour.
	Body color.Color
	// Muted is used for empty-state messaging and untyped accent bars.
	Muted color.Color
	// Background is the pane background colour used by RenderBlocks for
	// PadLines-style bleed prevention. Set to lipgloss.NoColor{} when no
	// explicit background is in use.
	Background color.Color
}

// DefaultTimelinePalette returns the default Cairn-themed timeline colours.
func DefaultTimelinePalette() TimelinePalette {
	return TimelinePalette{
		DateHeader: lipgloss.Color("#b98300"),
		Separator:  lipgloss.Color("#3a3a4a"),
		Body:       lipgloss.Color("#e0e0e0"),
		Muted:      lipgloss.Color("#8b8b8b"),
		Background: lipgloss.NoColor{},
	}
}

// TimelineRenderer renders a types.QueryResult as a reverse-chronological
// journal-style view. Each row is expected to have a "created" column
// (time.Time or ISO 8601 string) and a "body" column (markdown text).
type TimelineRenderer struct {
	// Palette controls the colour scheme.
	Palette TimelinePalette
	// DateColumn is the column name used for the date header. Defaults to "created".
	DateColumn string
	// BodyColumn is the column name used for body content. Defaults to "body".
	BodyColumn string
	// TypeColour is a callback that returns (bg, fg) hex colours for a given
	// node type name. When nil, entries use the default Body colour.
	TypeColour func(typeName string) (bg, fg string)
	// TypesColumn identifies which result column contains node types.
	// Defaults to "types" if empty.
	TypesColumn string
	// BlockStyle controls which rendering method View() delegates to.
	// When true, View() uses RenderBlocks; when false, it uses Render.
	BlockStyle bool
}

// NewTimelineRenderer returns a renderer with default palette and column names.
func NewTimelineRenderer() *TimelineRenderer {
	return &TimelineRenderer{
		Palette:    DefaultTimelinePalette(),
		DateColumn: "created",
		BodyColumn: "body",
	}
}

// timelineEntry is a resolved, sortable entry extracted from a query row.
type timelineEntry struct {
	date  time.Time
	body  string
	types []string
}

// typesColumn returns the effective types column name, defaulting to "types".
func (r *TimelineRenderer) typesColumn() string {
	if r.TypesColumn != "" {
		return r.TypesColumn
	}
	return "types"
}

// View renders the timeline using RenderBlocks when BlockStyle is true, or
// Render otherwise. width is the available terminal width. result is a pointer
// so that the caller can supply nil to get an empty-state message.
func (r *TimelineRenderer) View(result types.QueryResult, width int) string {
	if r.BlockStyle {
		return r.RenderBlocks(result, width)
	}
	return r.Render(result, width)
}

// Render produces a styled timeline string from result.
// Entries are sorted newest-first. width is the available terminal width.
//
// TD.6: every style below carries both Background and Foreground (rule 1).
// Unlike RenderBlocks, Render does not pad its own lines to width on bg —
// the newlines separating entries and following the date header are bare
// "\n" (see the loop below), and the full-width separator rule is the only
// line here that happens to cover its own row edge-to-edge. This is safe in
// practice because the sole caller, viewPane.View, wraps the whole returned
// string in PadLines before it reaches the screen, but treat that as an
// external guarantee rather than something this function itself provides —
// a future caller that renders Render's output directly would reintroduce
// background bleed.
func (r *TimelineRenderer) Render(result types.QueryResult, width int) string {
	bg := r.Palette.Background

	if len(result.Rows) == 0 {
		return lipgloss.NewStyle().
			Foreground(r.Palette.Muted).
			Background(bg).
			Render("No entries.")
	}

	dateCol := r.DateColumn
	bodyCol := r.BodyColumn
	typesCol := r.typesColumn()

	entries := make([]timelineEntry, 0, len(result.Rows))
	for _, row := range result.Rows {
		t := parseTimeValue(row[dateCol])
		body := formatCellValue(row[bodyCol])
		nodeTypes := extractTypes(row, typesCol)
		entries = append(entries, timelineEntry{date: t, body: body, types: nodeTypes})
	}

	// Sort reverse-chronologically (newest first).
	sort.Slice(entries, func(i, j int) bool {
		return entries[i].date.After(entries[j].date)
	})

	separatorStr := strings.Repeat(timelineSeparator, width)
	separatorStyled := lipgloss.NewStyle().
		Foreground(r.Palette.Separator).
		Background(bg).
		Render(separatorStr)

	dateStyle := lipgloss.NewStyle().
		Foreground(r.Palette.DateHeader).
		Background(bg).
		Bold(true)

	bodyStyle := lipgloss.NewStyle().
		Foreground(r.Palette.Body).
		Background(bg)

	var sb strings.Builder
	for i, entry := range entries {
		if i > 0 {
			sb.WriteRune('\n')
			sb.WriteString(separatorStyled)
			sb.WriteRune('\n')
		}

		var dateStr string
		if entry.date.IsZero() {
			dateStr = "Unknown date"
		} else {
			dateStr = entry.date.Format(timelineDateFormat)
		}

		// Render the date header, optionally with a type badge pill.
		dateRendered := dateStyle.Render(dateStr)
		if r.TypeColour != nil && len(entry.types) > 0 {
			typeName := entry.types[0]
			badgeBg, badgeFg := r.TypeColour(typeName)
			badge := lipgloss.NewStyle().
				Background(lipgloss.Color(badgeBg)).
				Foreground(lipgloss.Color(badgeFg)).
				Padding(0, 1).
				Render(typeName)
			dateRendered = dateRendered + spacer(2, bg) + badge
		}

		sb.WriteString(dateRendered)
		sb.WriteRune('\n')

		// Tint body text with the first type's foreground colour when available.
		entryBodyStyle := bodyStyle
		if r.TypeColour != nil && len(entry.types) > 0 {
			_, fg := r.TypeColour(entry.types[0])
			entryBodyStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color(fg)).
				Background(bg)
		}
		sb.WriteString(entryBodyStyle.Render(entry.body))
	}

	return sb.String()
}

// RenderBlocks produces a block-mode timeline where each entry is a horizontal
// coloured row: a fixed-width date label, a type-coloured accent bar, and the
// body text truncated to fit the remaining width. Blocks are separated by a
// single blank line instead of full-width rules.
//
// Background-bleed rules are observed: every lipgloss.NewStyle() in a block
// carries both .Background() and .Foreground(), and the completed block string
// is padded to width using the palette background colour.
func (r *TimelineRenderer) RenderBlocks(result types.QueryResult, width int) string {
	bg := r.Palette.Background

	if len(result.Rows) == 0 {
		return lipgloss.NewStyle().
			Foreground(r.Palette.Muted).
			Background(bg).
			Render("No entries.")
	}

	dateCol := r.DateColumn
	bodyCol := r.BodyColumn
	typesCol := r.typesColumn()

	entries := make([]timelineEntry, 0, len(result.Rows))
	for _, row := range result.Rows {
		t := parseTimeValue(row[dateCol])
		body := formatCellValue(row[bodyCol])
		nodeTypes := extractTypes(row, typesCol)
		entries = append(entries, timelineEntry{date: t, body: body, types: nodeTypes})
	}

	// Sort reverse-chronologically (newest first).
	sort.Slice(entries, func(i, j int) bool {
		return entries[i].date.After(entries[j].date)
	})

	dateStyle := lipgloss.NewStyle().
		Foreground(r.Palette.DateHeader).
		Background(bg).
		Bold(true)

	var sb strings.Builder
	for i, entry := range entries {
		if i > 0 {
			// Single blank line between blocks (no full-width rule).
			sb.WriteRune('\n')
			sb.WriteString(padLine("", width, bg))
			sb.WriteRune('\n')
		}

		// ── Date header line ──────────────────────────────────────────────────
		var dateStr string
		if entry.date.IsZero() {
			dateStr = "Unknown date"
		} else {
			dateStr = entry.date.Format(timelineDateFormat)
		}
		dateHeader := padLine(dateStyle.Render(dateStr), width, bg)
		sb.WriteString(dateHeader)
		sb.WriteRune('\n')

		// ── Block row: [date label] [accent bar]  [body] ─────────────────────
		blockRow := r.renderTimelineBlock(entry, width, bg)
		sb.WriteString(blockRow)
	}

	return sb.String()
}

// renderTimelineBlock renders a single entry as a horizontal block row:
//
//	"2 Jan 06    " + "████████" + "  " + "body text..."
//
// The accent bar is coloured by node type; the body is truncated to fit.
// Every style carries both Background and Foreground to prevent bleed.
func (r *TimelineRenderer) renderTimelineBlock(entry timelineEntry, width int, bg color.Color) string {
	// ── Date label column (fixed width, left-aligned) ─────────────────────
	var shortDate string
	if entry.date.IsZero() {
		shortDate = "?"
	} else {
		shortDate = entry.date.Format(timelineDateFormatShort)
	}
	// Pad or truncate to labelWidth.
	label := padOrTruncate(shortDate, timelineLabelWidth)
	labelStr := lipgloss.NewStyle().
		Foreground(r.Palette.DateHeader).
		Background(bg).
		Render(label)

	// ── Accent bar column ─────────────────────────────────────────────────
	accentChar := timelineBlockMutedChar
	accentColour := r.Palette.Muted
	if r.TypeColour != nil && len(entry.types) > 0 {
		_, fg := r.TypeColour(entry.types[0])
		accentColour = lipgloss.Color(fg)
		accentChar = timelineBlockAccentChar
	}
	barStr := lipgloss.NewStyle().
		Foreground(accentColour).
		Background(bg).
		Render(strings.Repeat(accentChar, timelineBlockAccentWidth))

	// ── Body column (truncated to remaining width) ────────────────────────
	// Layout: labelWidth + 1 (space) + accentWidth + 2 (double space) + body
	const gap = 3 // 1 between label/bar + 2 between bar/body
	bodyWidth := width - timelineLabelWidth - timelineBlockAccentWidth - gap
	if bodyWidth < 1 {
		bodyWidth = 1
	}

	bodyFg := r.Palette.Body
	if r.TypeColour != nil && len(entry.types) > 0 {
		_, fg := r.TypeColour(entry.types[0])
		bodyFg = lipgloss.Color(fg)
	}
	truncated := truncateToWidth(entry.body, bodyWidth)
	bodyStr := lipgloss.NewStyle().
		Foreground(bodyFg).
		Background(bg).
		Render(truncated)

	// Spacer between label and bar: single space with bg to prevent bleed.
	spacer1 := lipgloss.NewStyle().Background(bg).Foreground(r.Palette.Body).Render(" ")
	// Double space between bar and body.
	spacer2 := lipgloss.NewStyle().Background(bg).Foreground(r.Palette.Body).Render("  ")

	row := fmt.Sprintf("%s%s%s%s%s", labelStr, spacer1, barStr, spacer2, bodyStr)
	return padLine(row, width, bg)
}

// padLine pads a single rendered line to exactly width characters wide using
// bg as the background. This replicates the PadLines logic from tui/render.go
// for use within the views package, which cannot import tui without a circular
// dependency.
func padLine(line string, width int, bg color.Color) string {
	if width <= 0 {
		return line
	}
	return lipgloss.NewStyle().Width(width).Background(bg).Render(line)
}

// truncateToWidth returns s truncated to at most n runes. It does not add an
// ellipsis — the accent colour already signals the entry type. Newlines are
// replaced with spaces so the block row stays single-line.
func truncateToWidth(s string, n int) string {
	// Flatten to a single line.
	s = strings.ReplaceAll(s, "\n", " ")
	runes := []rune(s)
	if len(runes) <= n {
		return s
	}
	return string(runes[:n])
}

// extractTypes pulls a string slice of node types from a query result row.
// The value at the given column key is expected to be []interface{} of strings
// (as returned by the query engine). Returns nil if the column is missing or
// the value cannot be interpreted as a string slice.
func extractTypes(row map[string]interface{}, col string) []string {
	v, ok := row[col]
	if !ok || v == nil {
		return nil
	}

	switch typed := v.(type) {
	case []interface{}:
		result := make([]string, 0, len(typed))
		for _, item := range typed {
			if s, ok := item.(string); ok {
				result = append(result, s)
			} else {
				result = append(result, fmt.Sprintf("%v", item))
			}
		}
		if len(result) == 0 {
			return nil
		}
		return result
	case []string:
		if len(typed) == 0 {
			return nil
		}
		return typed
	case string:
		if typed == "" {
			return nil
		}
		return []string{typed}
	}

	return nil
}

// parseTimeValue attempts to extract a time.Time from a query result cell.
// It handles time.Time values directly, and ISO 8601 strings. Returns the
// zero time when conversion is not possible.
func parseTimeValue(v interface{}) time.Time {
	if v == nil {
		return time.Time{}
	}
	switch t := v.(type) {
	case time.Time:
		return t
	case string:
		// Try common date/time formats.
		formats := []string{
			time.RFC3339,
			time.RFC3339Nano,
			"2006-01-02T15:04:05",
			"2006-01-02",
		}
		for _, f := range formats {
			if parsed, err := time.Parse(f, t); err == nil {
				return parsed
			}
		}
	}
	return time.Time{}
}
