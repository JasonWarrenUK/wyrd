package views

import (
	"fmt"
	"image/color"
	"strings"
	"time"

	"charm.land/lipgloss/v2"
	"github.com/jasonwarrenuk/wyrd/internal/types"
)

const (
	fillBarWidth = 8
	timeFormat   = "15:04"
)

// Glyphs holds the Unicode characters used for schedule rendering.
type Glyphs struct {
	EnergyDeep   string
	EnergyMedium string
	EnergyLow    string
	CalendarDash string
	Overflow     string
}

// DefaultGlyphs returns the canonical glyph set used when no theme is loaded.
func DefaultGlyphs() Glyphs {
	return Glyphs{
		EnergyDeep:   "▓",
		EnergyMedium: "█",
		EnergyLow:    "░",
		CalendarDash: "┄",
		Overflow:     "◆",
	}
}

// Palette holds the Lipgloss colours used by the schedule renderer.
type Palette struct {
	EnergyDeep   color.Color
	EnergyMedium color.Color
	EnergyLow    color.Color
	Muted        color.Color
	Overflow     color.Color
}

// DefaultPalette returns the Cairn theme colours as the default palette.
func DefaultPalette() Palette {
	return Palette{
		EnergyDeep:   lipgloss.Color("#794aff"),
		EnergyMedium: lipgloss.Color("#b98300"),
		EnergyLow:    lipgloss.Color("#6f6f6f"),
		Muted:        lipgloss.Color("#8b8b8b"),
		Overflow:     lipgloss.Color("#d57300"),
	}
}

// ScheduleRenderer renders a day's schedule as a Lipgloss-styled string.
type ScheduleRenderer struct {
	Glyphs  Glyphs
	Palette Palette
	// Width is the available column width in characters.
	Width int
}

// NewScheduleRenderer constructs a renderer with default glyphs and the Cairn
// colour palette.
func NewScheduleRenderer() *ScheduleRenderer {
	return &ScheduleRenderer{
		Glyphs:  DefaultGlyphs(),
		Palette: DefaultPalette(),
		Width:   40,
	}
}

// Render produces the full schedule view string from a DisplacementResult.
func (r *ScheduleRenderer) Render(result DisplacementResult) string {
	if len(result.Entries) == 0 {
		return lipgloss.NewStyle().
			Foreground(r.Palette.Muted).
			Render("No tasks scheduled for today.")
	}

	var sb strings.Builder
	for _, de := range result.Entries {
		sb.WriteString(r.renderEntry(de))
		sb.WriteRune('\n')
	}

	summary := result.FormatSummaryLine()
	if summary != "" {
		sb.WriteRune('\n')
		sb.WriteString(
			lipgloss.NewStyle().
				Foreground(r.Palette.Overflow).
				Render(summary),
		)
	}

	return strings.TrimRight(sb.String(), "\n")
}

// renderEntry renders a single schedule row.
func (r *ScheduleRenderer) renderEntry(de DisplacedEntry) string {
	e := de.Entry
	timeLabel := e.Start.Format(timeFormat)

	if e.IsCalendarEvent {
		return r.renderCalendarEvent(timeLabel, e.Title)
	}
	if de.IsDisplaced {
		return r.renderDisplaced(timeLabel, e.Title)
	}
	return r.renderTask(timeLabel, e, de.CompressedDuration)
}

// renderTask renders a normal task entry with an energy fill bar.
func (r *ScheduleRenderer) renderTask(timeLabel string, e ScheduleEntry, duration time.Duration) string {
	fillChar, colour := r.energyStyle(e.Energy)

	barLen := fillBarWidth
	if e.Duration > 0 && duration < e.Duration {
		barLen = int(float64(fillBarWidth) * float64(duration) / float64(e.Duration))
		if barLen < 1 {
			barLen = 1
		}
	}

	bar := strings.Repeat(fillChar, barLen)
	styledBar := lipgloss.NewStyle().
		Foreground(colour).
		Render(bar)

	return fmt.Sprintf("%s %s %s", timeLabel, styledBar, e.Title)
}

// renderCalendarEvent renders a calendar entry with dashed fill characters.
func (r *ScheduleRenderer) renderCalendarEvent(timeLabel, title string) string {
	dashes := strings.Repeat(r.Glyphs.CalendarDash, fillBarWidth)
	styledDashes := lipgloss.NewStyle().
		Foreground(r.Palette.Muted).
		Render(dashes)
	return fmt.Sprintf("%s %s %s", timeLabel, styledDashes, title)
}

// renderDisplaced renders a task that has been pushed out of the day.
func (r *ScheduleRenderer) renderDisplaced(timeLabel, title string) string {
	overflow := lipgloss.NewStyle().
		Foreground(r.Palette.Overflow).
		Render(r.Glyphs.Overflow)
	muted := lipgloss.NewStyle().
		Foreground(r.Palette.Muted)
	return fmt.Sprintf("%s %s %s", muted.Render(timeLabel), overflow, muted.Render(title))
}

// Default column names read by EntriesFromQueryResult. A saved view with
// DisplaySchedule is expected to RETURN columns under these names; use
// ScheduleColumns to override any of them per view.
const (
	scheduleIDColumn       = "id"
	scheduleTitleColumn    = "title"
	scheduleStartColumn    = "start"
	scheduleDurationColumn = "duration"
	scheduleEnergyColumn   = "energy"
	scheduleCalendarColumn = "is_calendar_event"
)

// ScheduleColumns names the query result columns EntriesFromQueryResult reads
// from each row. Any field left empty falls back to its default column name
// (see the schedule*Column constants).
type ScheduleColumns struct {
	ID              string
	Title           string
	Start           string
	Duration        string
	Energy          string
	IsCalendarEvent string
}

// column returns name if set, otherwise the given default.
func (c ScheduleColumns) column(name, def string) string {
	if name != "" {
		return name
	}
	return def
}

// EntriesFromQueryResult adapts a types.QueryResult into the []ScheduleEntry
// shape Calculate expects. This is the query -> ScheduleEntry data source
// TD.13 left unbuilt: nothing else in the codebase constructs a ScheduleEntry
// from a query row.
//
// Expected columns (overridable via cols): id, title, start (time.Time or an
// ISO 8601 string, via the same parseTimeValue timeline.go uses), duration
// (minutes as a number, or a Go duration string like "30m"), energy ("deep",
// "medium", or "low" — unrecognised values fall back to EnergyLow, matching
// ScheduleRenderer.energyStyle's own default), and is_calendar_event (bool).
// Rows missing id or start are skipped rather than erroring: a malformed row
// silently omitted from the schedule is preferable to the whole view
// crashing, matching this package's other adapters (extractTypes,
// parseTimeValue) which all degrade gracefully on unexpected shapes.
func EntriesFromQueryResult(result types.QueryResult, cols ScheduleColumns) []ScheduleEntry {
	idCol := cols.column(cols.ID, scheduleIDColumn)
	titleCol := cols.column(cols.Title, scheduleTitleColumn)
	startCol := cols.column(cols.Start, scheduleStartColumn)
	durationCol := cols.column(cols.Duration, scheduleDurationColumn)
	energyCol := cols.column(cols.Energy, scheduleEnergyColumn)
	calendarCol := cols.column(cols.IsCalendarEvent, scheduleCalendarColumn)

	entries := make([]ScheduleEntry, 0, len(result.Rows))
	for _, row := range result.Rows {
		id := formatCellValue(row[idCol])
		start := parseTimeValue(row[startCol])
		if id == "" || start.IsZero() {
			continue
		}

		entries = append(entries, ScheduleEntry{
			ID:              id,
			Title:           formatCellValue(row[titleCol]),
			Start:           start,
			Duration:        parseScheduleDuration(row[durationCol]),
			Energy:          parseEnergyLevel(row[energyCol]),
			IsCalendarEvent: parseBoolValue(row[calendarCol]),
		})
	}
	return entries
}

// parseScheduleDuration reads a duration cell as either a bare number of
// minutes (the shape JSONC round-trips numeric properties as — float64) or a
// Go duration string such as "30m". Returns 0 (fill remaining time, per
// ScheduleEntry.Duration's own doc comment) on anything else.
func parseScheduleDuration(v interface{}) time.Duration {
	switch t := v.(type) {
	case time.Duration:
		return t
	case float64:
		return time.Duration(t) * time.Minute
	case int:
		return time.Duration(t) * time.Minute
	case string:
		if d, err := time.ParseDuration(t); err == nil {
			return d
		}
	}
	return 0
}

// parseEnergyLevel maps a cell value onto EnergyLevel, defaulting to
// EnergyLow for anything unrecognised — the same default
// ScheduleRenderer.energyStyle falls back to.
func parseEnergyLevel(v interface{}) EnergyLevel {
	switch EnergyLevel(formatCellValue(v)) {
	case EnergyDeep:
		return EnergyDeep
	case EnergyMedium:
		return EnergyMedium
	default:
		return EnergyLow
	}
}

// parseBoolValue reads a cell as a bool, tolerating the string forms JSONC
// or a hand-written query might produce. Anything else is false.
func parseBoolValue(v interface{}) bool {
	switch t := v.(type) {
	case bool:
		return t
	case string:
		return t == "true"
	default:
		return false
	}
}

// energyStyle returns the fill character and colour for an energy level.
func (r *ScheduleRenderer) energyStyle(energy EnergyLevel) (fillChar string, colour color.Color) {
	switch energy {
	case EnergyDeep:
		return r.Glyphs.EnergyDeep, r.Palette.EnergyDeep
	case EnergyMedium:
		return r.Glyphs.EnergyMedium, r.Palette.EnergyMedium
	default:
		return r.Glyphs.EnergyLow, r.Palette.EnergyLow
	}
}
