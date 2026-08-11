package types_test

import (
	"testing"
	"time"

	"github.com/jasonwarrenuk/wyrd/internal/types"
)

func TestDaysSince_ExactDays(t *testing.T) {
	now := time.Date(2026, 7, 17, 12, 0, 0, 0, time.UTC)
	modified := now.AddDate(0, 0, -10)

	if got := types.DaysSince(modified, now); got != 10 {
		t.Errorf("expected 10 days, got %d", got)
	}
}

func TestDaysSince_SubDayFloorsToZero(t *testing.T) {
	now := time.Date(2026, 7, 17, 12, 0, 0, 0, time.UTC)
	modified := now.Add(-1 * time.Hour)

	if got := types.DaysSince(modified, now); got != 0 {
		t.Errorf("expected 0 days for a sub-day gap, got %d", got)
	}
}

func TestDaysSince_FutureTimestampClampsToZero(t *testing.T) {
	now := time.Date(2026, 7, 17, 12, 0, 0, 0, time.UTC)
	modified := now.Add(24 * time.Hour) // "modified" in the future

	if got := types.DaysSince(modified, now); got != 0 {
		t.Errorf("expected 0 days for a future timestamp, got %d", got)
	}
}

func TestIsStale_BelowThreshold(t *testing.T) {
	now := time.Date(2026, 7, 17, 0, 0, 0, 0, time.UTC)
	node := &types.Node{Date: types.DateFields{Modified: now.AddDate(0, 0, -5)}}

	if types.IsStale(node, now, 14) {
		t.Error("expected node modified 5 days ago not to be stale at a 14d threshold")
	}
}

func TestIsStale_AtThresholdIsNotStale(t *testing.T) {
	// IsStale is a strict ">" comparison — exactly at the threshold is not stale.
	now := time.Date(2026, 7, 17, 0, 0, 0, 0, time.UTC)
	node := &types.Node{Date: types.DateFields{Modified: now.AddDate(0, 0, -14)}}

	if types.IsStale(node, now, 14) {
		t.Error("expected a node exactly at the threshold not to be stale")
	}
}

func TestIsStale_AboveThreshold(t *testing.T) {
	now := time.Date(2026, 7, 17, 0, 0, 0, 0, time.UTC)
	node := &types.Node{Date: types.DateFields{Modified: now.AddDate(0, 0, -15)}}

	if !types.IsStale(node, now, 14) {
		t.Error("expected a node modified 15 days ago to be stale at a 14d threshold")
	}
}

func TestIsStale_ZeroThresholdResolvesToDefault(t *testing.T) {
	now := time.Date(2026, 7, 17, 0, 0, 0, 0, time.UTC)
	// 10 days idle: stale at the implicit default (14) only if > 14, so this
	// should NOT be stale — proving thresholdDays=0 resolved to 14, not 0
	// (which would make every idle node stale).
	node := &types.Node{Date: types.DateFields{Modified: now.AddDate(0, 0, -10)}}

	if types.IsStale(node, now, 0) {
		t.Error("expected thresholdDays<=0 to resolve to DefaultStalenessThresholdDays (14), not 0")
	}

	staleNode := &types.Node{Date: types.DateFields{Modified: now.AddDate(0, 0, -15)}}
	if !types.IsStale(staleNode, now, 0) {
		t.Error("expected a node idle 15 days to be stale under the resolved default of 14")
	}
}

func TestIsStale_NegativeThresholdResolvesToDefault(t *testing.T) {
	now := time.Date(2026, 7, 17, 0, 0, 0, 0, time.UTC)
	node := &types.Node{Date: types.DateFields{Modified: now.AddDate(0, 0, -10)}}

	if types.IsStale(node, now, -1) {
		t.Error("expected a negative thresholdDays to resolve to the default, not disable staleness entirely")
	}
}

func TestIsStale_NilNode(t *testing.T) {
	if types.IsStale(nil, time.Now(), 14) {
		t.Error("expected a nil node never to be stale")
	}
}
