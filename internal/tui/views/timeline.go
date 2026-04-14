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
	// timelineSeparator is the horizontal rule drawn between entries.
	timelineSeparator = "─"
)

// TimelinePalette holds the colours used by the timeline renderer.
type TimelinePalette struct {
	// DateHeader is the foreground colour for the date heading.
	DateHeader color.Color
	// Separator is the colour of the horizontal rule between entries.
	Separator color.Color
	// Body is the default body text colour.
	Body color.Color
	// Muted is used for empty-state messaging.
	Muted color.Color
}

// DefaultTimelinePalette returns the default Cairn-themed timeline colours.
func DefaultTimelinePalette() TimelinePalette {
	return TimelinePalette{
		DateHeader: lipgloss.Color("#b98300"),
		Separator:  lipgloss.Color("#3a3a4a"),
		Body:       lipgloss.Color("#e0e0e0"),
		Muted:      lipgloss.Color("#8b8b8b"),
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

// Render produces a styled timeline string from result.
// Entries are sorted newest-first. width is the available terminal width.
func (r *TimelineRenderer) Render(result types.QueryResult, width int) string {
	if len(result.Rows) == 0 {
		return lipgloss.NewStyle().
			Foreground(r.Palette.Muted).
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
		Render(separatorStr)

	dateStyle := lipgloss.NewStyle().
		Foreground(r.Palette.DateHeader).
		Bold(true)

	bodyStyle := lipgloss.NewStyle().
		Foreground(r.Palette.Body)

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
			bg, fg := r.TypeColour(typeName)
			badge := lipgloss.NewStyle().
				Background(lipgloss.Color(bg)).
				Foreground(lipgloss.Color(fg)).
				Padding(0, 1).
				Render(typeName)
			dateRendered = dateRendered + "  " + badge
		}

		sb.WriteString(dateRendered)
		sb.WriteRune('\n')

		// Tint body text with the first type's foreground colour when available.
		entryBodyStyle := bodyStyle
		if r.TypeColour != nil && len(entry.types) > 0 {
			_, fg := r.TypeColour(entry.types[0])
			entryBodyStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color(fg))
		}
		sb.WriteString(entryBodyStyle.Render(entry.body))
	}

	return sb.String()
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
