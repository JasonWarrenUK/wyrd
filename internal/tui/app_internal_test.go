package tui

import (
	"testing"
	"time"

	"github.com/jasonwarrenuk/wyrd/internal/stage"
)

// TestDelayToNextMinute covers TD.16: the status-bar clock tick aligns to
// the wall-clock minute boundary rather than free-running on a flat 60s
// interval, which would leave the displayed HH:MM up to 59s stale relative
// to the real minute change.
func TestDelayToNextMinute(t *testing.T) {
	tests := []struct {
		name string
		now  time.Time
		want time.Duration
	}{
		{
			name: "mid-minute rounds up to the boundary",
			now:  time.Date(2026, 3, 23, 9, 4, 17, 0, time.UTC),
			want: 43 * time.Second,
		},
		{
			name: "one second into the minute",
			now:  time.Date(2026, 3, 23, 9, 4, 1, 0, time.UTC),
			want: 59 * time.Second,
		},
		{
			name: "exactly on the boundary waits a full minute, not zero",
			now:  time.Date(2026, 3, 23, 9, 5, 0, 0, time.UTC),
			want: time.Minute,
		},
		{
			name: "sub-second remainder still rounds up to the boundary",
			now:  time.Date(2026, 3, 23, 9, 4, 59, 500_000_000, time.UTC),
			want: 500 * time.Millisecond,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := delayToNextMinute(tt.now)
			if got != tt.want {
				t.Errorf("delayToNextMinute(%v) = %v, want %v", tt.now, got, tt.want)
			}
			if got <= 0 {
				t.Errorf("delayToNextMinute(%v) = %v, want > 0 (a non-positive tea.Tick delay fires immediately in a tight loop)", tt.now, got)
			}
		})
	}
}

// TestDivergenceAdvisory covers TD.5's startup message: empty report is
// silent, an ordinary divergence names the count, and SchemaDrift gets a
// distinct message that doesn't imply the user edited anything.
func TestDivergenceAdvisory(t *testing.T) {
	tests := []struct {
		name    string
		report  stage.DivergenceReport
		want    string
		wantNil bool // true when the message should be empty
	}{
		{
			name:    "empty report is silent",
			report:  stage.DivergenceReport{},
			wantNil: true,
		},
		{
			name:   "single divergence uses singular wording",
			report: stage.DivergenceReport{Diverged: []stage.DivergedEntry{{Name: "Task", Kind: true}}},
			want:   "1 shadowed kind/stage-group entry diverged from upstream defaults — see :kinds / :stages",
		},
		{
			name: "multiple divergences use plural wording",
			report: stage.DivergenceReport{Diverged: []stage.DivergedEntry{
				{Name: "Task", Kind: true},
				{Name: "task-flow", Kind: false},
			}},
			want: "2 shadowed kind/stage-group entries diverged from upstream defaults — see :kinds / :stages",
		},
		{
			name:   "schema drift gets a distinct message, not a divergence count",
			report: stage.DivergenceReport{SchemaDrift: true},
			want:   "Shadow-provenance hashes are stale after an app update; re-save a kind/stage-group edit to refresh them",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := divergenceAdvisory(tt.report)
			if tt.wantNil {
				if got != "" {
					t.Errorf("divergenceAdvisory(%+v) = %q, want empty", tt.report, got)
				}
				return
			}
			if got != tt.want {
				t.Errorf("divergenceAdvisory(%+v) = %q, want %q", tt.report, got, tt.want)
			}
		})
	}
}
