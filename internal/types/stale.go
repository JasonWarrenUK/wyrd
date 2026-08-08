package types

import "time"

// DefaultStalenessThresholdDays is the fallback threshold used whenever a
// caller passes thresholdDays <= 0 — covering both a missing config.jsonc
// (Config zero value) and a present file that omits the field (omitempty
// unmarshals absent fields to 0).
const DefaultStalenessThresholdDays = 14

// DaysSince returns the whole number of days elapsed between t and now,
// floored. A now before t (clock skew, or a node modified "in the future")
// clamps to 0 rather than going negative.
//
// No TUI or query-engine dependencies; safe to call from the query
// evaluator, TUI rendering, and tests alike.
func DaysSince(t, now time.Time) int {
	days := int(now.Sub(t).Hours() / 24)
	if days < 0 {
		return 0
	}
	return days
}

// IsStale reports whether node has been idle (no modification) for longer
// than thresholdDays. thresholdDays <= 0 resolves to
// DefaultStalenessThresholdDays, so a zero-valued config field still means
// 14 days rather than "everything is always stale".
//
// Unlike EvalBlockers, staleness is a presentation-boundary judgement, not a
// graph fact — this is deliberately threshold-free at the query-engine layer
// (DL.3); only the caller (TUI, tests) supplies a threshold.
func IsStale(node *Node, now time.Time, thresholdDays int) bool {
	if node == nil {
		return false
	}
	if thresholdDays <= 0 {
		thresholdDays = DefaultStalenessThresholdDays
	}
	return DaysSince(node.Date.Modified, now) > thresholdDays
}
