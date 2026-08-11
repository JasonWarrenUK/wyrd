package tui

import (
	"testing"
	"time"
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
