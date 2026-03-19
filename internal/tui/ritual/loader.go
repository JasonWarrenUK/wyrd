// Package ritual implements the ritual runner — a step sequencer for
// structured interactions such as morning planning check-ins.
package ritual

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/jasonwarrenuk/wyrd/internal/types"
)

// LoadRituals reads all ritual JSONC files from {storePath}/rituals/ and
// returns them as a slice. JSONC comments are stripped before parsing.
func LoadRituals(storePath string) ([]*types.Ritual, error) {
	dir := filepath.Join(storePath, "rituals")

	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("reading rituals directory: %w", err)
	}

	var rituals []*types.Ritual

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}

		name := entry.Name()
		if !strings.HasSuffix(name, ".jsonc") && !strings.HasSuffix(name, ".json") {
			continue
		}

		path := filepath.Join(dir, name)
		ritual, err := loadRitualFile(path)
		if err != nil {
			return nil, fmt.Errorf("loading ritual %q: %w", name, err)
		}
		rituals = append(rituals, ritual)
	}

	return rituals, nil
}

// loadRitualFile reads a single ritual JSONC file from disk.
func loadRitualFile(path string) (*types.Ritual, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("reading file: %w", err)
	}

	cleaned := stripJSONCComments(data)

	var ritual types.Ritual
	if err := json.Unmarshal(cleaned, &ritual); err != nil {
		return nil, fmt.Errorf("parsing JSON: %w", err)
	}

	return &ritual, nil
}

// stripJSONCComments removes // and /* */ style comments from a JSONC byte
// slice. It respects string literals so comment-like sequences inside strings
// are preserved.
func stripJSONCComments(src []byte) []byte {
	out := make([]byte, 0, len(src))
	i := 0
	for i < len(src) {
		switch {
		case src[i] == '"':
			// Copy the entire string literal verbatim, handling escapes.
			out = append(out, src[i])
			i++
			for i < len(src) {
				ch := src[i]
				out = append(out, ch)
				i++
				if ch == '\\' && i < len(src) {
					out = append(out, src[i])
					i++
				} else if ch == '"' {
					break
				}
			}

		case i+1 < len(src) && src[i] == '/' && src[i+1] == '/':
			// Line comment — skip to end of line.
			for i < len(src) && src[i] != '\n' {
				i++
			}

		case i+1 < len(src) && src[i] == '/' && src[i+1] == '*':
			// Block comment — skip to closing */.
			i += 2
			for i+1 < len(src) {
				if src[i] == '*' && src[i+1] == '/' {
					i += 2
					break
				}
				i++
			}

		default:
			out = append(out, src[i])
			i++
		}
	}
	return out
}

// DueRituals filters the supplied rituals and returns those whose schedule
// matches the time provided by clock. A ritual is considered due when:
//   - Its scheduled weekday(s) include the current weekday, and
//   - The current wall-clock time is on or after the ritual's scheduled time.
//
// Rituals that have already been dismissed for the current day are NOT
// filtered here — that responsibility belongs to the caller (scheduler).
func DueRituals(rituals []*types.Ritual, clock types.Clock) []*types.Ritual {
	now := clock.Now()
	var due []*types.Ritual

	for _, r := range rituals {
		if isScheduledNow(r.Schedule, now) {
			due = append(due, r)
		}
	}
	return due
}

// isScheduledNow reports whether schedule matches the given time.
func isScheduledNow(schedule types.RitualSchedule, now time.Time) bool {
	// Parse the scheduled time.
	scheduled, err := parseScheduleTime(schedule.Time, now)
	if err != nil {
		return false
	}

	// The ritual window opens at the scheduled time and never closes within
	// the same day — the scheduler handles deduplication.
	if now.Before(scheduled) {
		return false
	}

	// Collect the active day set.
	days := activeDays(schedule)
	currentDay := strings.ToLower(now.Weekday().String()[:3])
	for _, d := range days {
		if strings.ToLower(d) == currentDay {
			return true
		}
	}
	return false
}

// activeDays returns the effective days list for a schedule.
// If Days is set it takes precedence; otherwise Day is used.
func activeDays(schedule types.RitualSchedule) []string {
	if len(schedule.Days) > 0 {
		return schedule.Days
	}
	if schedule.Day != "" {
		return []string{schedule.Day}
	}
	return nil
}

// parseScheduleTime parses a "HH:MM" time string and returns a time.Time on
// the same calendar date as ref.
func parseScheduleTime(hhmm string, ref time.Time) (time.Time, error) {
	parts := strings.SplitN(hhmm, ":", 2)
	if len(parts) != 2 {
		return time.Time{}, fmt.Errorf("invalid time format %q", hhmm)
	}

	var hour, minute int
	if _, err := fmt.Sscanf(parts[0], "%d", &hour); err != nil {
		return time.Time{}, fmt.Errorf("invalid hour in %q", hhmm)
	}
	if _, err := fmt.Sscanf(parts[1], "%d", &minute); err != nil {
		return time.Time{}, fmt.Errorf("invalid minute in %q", hhmm)
	}

	return time.Date(ref.Year(), ref.Month(), ref.Day(), hour, minute, 0, 0, ref.Location()), nil
}
