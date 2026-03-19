package views

import (
	"fmt"
	"strings"
	"time"

	"github.com/jasonwarrenuk/wyrd/internal/types"
)

// EnergyLevel represents the three supported task energy tiers.
type EnergyLevel string

const (
	EnergyDeep   EnergyLevel = "deep"
	EnergyMedium EnergyLevel = "medium"
	EnergyLow    EnergyLevel = "low"
)

// ScheduleEntry is a single item in the day's schedule — either a task or a
// calendar event imported by a sync plugin.
type ScheduleEntry struct {
	// ID is the node UUID for tasks, or the external event ID for calendar entries.
	ID string

	// Title is the display label.
	Title string

	// Start is the planned start time.
	Start time.Time

	// Duration is the planned allocation. Zero means the entry fills remaining time.
	Duration time.Duration

	// Energy is the task's energy level. Ignored for calendar events.
	Energy EnergyLevel

	// IsCalendarEvent marks entries sourced from a calendar sync plugin.
	// Calendar events render with dashed lines and cannot be displaced.
	IsCalendarEvent bool
}

// DisplacedEntry wraps a ScheduleEntry with displacement metadata.
type DisplacedEntry struct {
	Entry ScheduleEntry

	// CompressedDuration is the time remaining after displacement compression.
	// Equal to Entry.Duration when the entry is not affected.
	CompressedDuration time.Duration

	// IsDisplaced is true when the entry no longer fits within the day window.
	IsDisplaced bool

	// IsOverflowing is true when the entry is partially displaced.
	IsOverflowing bool
}

// DisplacementResult holds the outcome of a displacement calculation.
type DisplacementResult struct {
	// Entries is the full list of entries, each annotated with displacement state.
	Entries []DisplacedEntry

	// OverrunDuration is how much the active task has exceeded its allocation.
	OverrunDuration time.Duration

	// UnscheduledDuration is the total time that can no longer fit in the day.
	UnscheduledDuration time.Duration

	// DisplacedCount is the number of tasks fully pushed out of the day.
	DisplacedCount int

	// ActiveEntryID is the ID of the currently active (being timed) entry.
	ActiveEntryID string
}

// FormatSummaryLine returns the displacement summary shown beneath the schedule.
// Returns an empty string when there is no displacement.
func (r DisplacementResult) FormatSummaryLine() string {
	if r.UnscheduledDuration <= 0 && r.DisplacedCount == 0 {
		return ""
	}
	if r.DisplacedCount == 0 {
		return fmt.Sprintf("◆ %s unscheduled", formatDuration(r.UnscheduledDuration))
	}
	noun := "task"
	if r.DisplacedCount != 1 {
		noun = "tasks"
	}
	return fmt.Sprintf("◆ %s unscheduled · %d %s displaced to tomorrow",
		formatDuration(r.UnscheduledDuration), r.DisplacedCount, noun)
}

// FormatStatusBar returns the status bar overrun indicator for the active task.
// Returns an empty string when on time.
func FormatStatusBar(activeTitle string, elapsed, planned time.Duration) string {
	if elapsed <= planned {
		return ""
	}
	return fmt.Sprintf("◆ %s on %s (%s planned)",
		formatDuration(elapsed), activeTitle, formatDuration(planned))
}

// DisplacementInput carries everything Calculate needs.
type DisplacementInput struct {
	// Entries is the ordered list of today's schedule entries.
	Entries []ScheduleEntry

	// ActiveEntryID is the ID of the task currently being timed.
	ActiveEntryID string

	// Elapsed is how long the active task has been running.
	Elapsed time.Duration

	// DayEnd is the end of the scheduling window.
	DayEnd time.Time

	// Clock is an injectable time source.
	Clock types.Clock
}

// Calculate computes time displacement for the given schedule.
// Displacement is a pure view computation — it does not modify node data.
func Calculate(input DisplacementInput) DisplacementResult {
	overrun := time.Duration(0)
	if input.ActiveEntryID != "" {
		for _, e := range input.Entries {
			if e.ID == input.ActiveEntryID && !e.IsCalendarEvent {
				if input.Elapsed > e.Duration {
					overrun = input.Elapsed - e.Duration
				}
				break
			}
		}
	}

	result := DisplacementResult{
		ActiveEntryID:   input.ActiveEntryID,
		OverrunDuration: overrun,
	}

	if overrun <= 0 {
		for _, e := range input.Entries {
			result.Entries = append(result.Entries, DisplacedEntry{
				Entry:              e,
				CompressedDuration: e.Duration,
			})
		}
		return result
	}

	// Compute remaining pool: sum of durations of non-calendar tasks after active.
	remainingPool := time.Duration(0)
	pastActive := false
	for _, e := range input.Entries {
		if e.ID == input.ActiveEntryID {
			pastActive = true
			continue
		}
		if pastActive && !e.IsCalendarEvent {
			remainingPool += e.Duration
		}
	}

	toShed := overrun
	if toShed > remainingPool {
		toShed = remainingPool
	}

	pastActive = false
	for _, e := range input.Entries {
		if e.ID == input.ActiveEntryID {
			pastActive = true
			result.Entries = append(result.Entries, DisplacedEntry{
				Entry:              e,
				CompressedDuration: e.Duration,
			})
			continue
		}

		if !pastActive || e.IsCalendarEvent {
			result.Entries = append(result.Entries, DisplacedEntry{
				Entry:              e,
				CompressedDuration: e.Duration,
			})
			continue
		}

		if remainingPool == 0 {
			result.Entries = append(result.Entries, DisplacedEntry{
				Entry:              e,
				CompressedDuration: 0,
				IsDisplaced:        true,
			})
			result.DisplacedCount++
			result.UnscheduledDuration += e.Duration
			continue
		}

		share := time.Duration(float64(e.Duration) / float64(remainingPool) * float64(toShed))
		compressed := e.Duration - share

		if compressed <= 0 {
			result.Entries = append(result.Entries, DisplacedEntry{
				Entry:              e,
				CompressedDuration: 0,
				IsDisplaced:        true,
			})
			result.DisplacedCount++
			result.UnscheduledDuration += e.Duration
		} else {
			result.Entries = append(result.Entries, DisplacedEntry{
				Entry:              e,
				CompressedDuration: compressed,
			})
		}
	}

	return result
}

// formatDuration renders a duration as a compact human-readable string.
func formatDuration(d time.Duration) string {
	d = d.Round(time.Minute)
	h := int(d.Hours())
	m := int(d.Minutes()) % 60
	if h > 0 && m > 0 {
		return fmt.Sprintf("%dh%dm", h, m)
	}
	if h > 0 {
		return fmt.Sprintf("%dh", h)
	}
	return fmt.Sprintf("%dm", m)
}

// ensure strings is imported (used by schedule.go in the same package).
var _ = strings.Contains
