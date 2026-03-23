package views

import (
	"fmt"
	"strings"
	"time"

	"charm.land/lipgloss/v2"
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
	EnergyDeep   string
	EnergyMedium string
	EnergyLow    string
	Muted        string
	Overflow     string
}

// DefaultPalette returns the Cairn theme colours as the default palette.
func DefaultPalette() Palette {
	return Palette{
		EnergyDeep:   "#794aff",
		EnergyMedium: "#b98300",
		EnergyLow:    "#6f6f6f",
		Muted:        "#8b8b8b",
		Overflow:     "#d57300",
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
			Foreground(lipgloss.Color(r.Palette.Muted)).
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
				Foreground(lipgloss.Color(r.Palette.Overflow)).
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
		Foreground(lipgloss.Color(colour)).
		Render(bar)

	return fmt.Sprintf("%s %s %s", timeLabel, styledBar, e.Title)
}

// renderCalendarEvent renders a calendar entry with dashed fill characters.
func (r *ScheduleRenderer) renderCalendarEvent(timeLabel, title string) string {
	dashes := strings.Repeat(r.Glyphs.CalendarDash, fillBarWidth)
	styledDashes := lipgloss.NewStyle().
		Foreground(lipgloss.Color(r.Palette.Muted)).
		Render(dashes)
	return fmt.Sprintf("%s %s %s", timeLabel, styledDashes, title)
}

// renderDisplaced renders a task that has been pushed out of the day.
func (r *ScheduleRenderer) renderDisplaced(timeLabel, title string) string {
	overflow := lipgloss.NewStyle().
		Foreground(lipgloss.Color(r.Palette.Overflow)).
		Render(r.Glyphs.Overflow)
	muted := lipgloss.NewStyle().
		Foreground(lipgloss.Color(r.Palette.Muted))
	return fmt.Sprintf("%s %s %s", muted.Render(timeLabel), overflow, muted.Render(title))
}

// energyStyle returns the fill character and hex colour for an energy level.
func (r *ScheduleRenderer) energyStyle(energy EnergyLevel) (fillChar, colour string) {
	switch energy {
	case EnergyDeep:
		return r.Glyphs.EnergyDeep, r.Palette.EnergyDeep
	case EnergyMedium:
		return r.Glyphs.EnergyMedium, r.Palette.EnergyMedium
	default:
		return r.Glyphs.EnergyLow, r.Palette.EnergyLow
	}
}
