package tui

import (
	"image/color"
	"testing"
)

// TestFocusTransitionProgressClampsBelowZero checks that a spring position
// that dips below 0 (possible with certain damping ratios even though ours
// is critically damped and shouldn't overshoot) is reported as 0 by
// progress(), since callers interpolating colours/widths need a clean [0,1]
// fraction rather than a raw physics value.
func TestFocusTransitionProgressClampsBelowZero(t *testing.T) {
	f := newFocusTransition(0, FocusLeft, FocusRight)
	f.pos = -0.2
	if got := f.progress(); got != 0 {
		t.Errorf("progress() = %v, want 0", got)
	}
}

// TestFocusTransitionProgressClampsAboveOne mirrors the above for the
// overshoot-past-target case.
func TestFocusTransitionProgressClampsAboveOne(t *testing.T) {
	f := newFocusTransition(0, FocusLeft, FocusRight)
	f.pos = 1.2
	if got := f.progress(); got != 1 {
		t.Errorf("progress() = %v, want 1", got)
	}
}

// TestFocusTransitionProgressMidTransition checks the pass-through case: a
// position already inside [0,1] is returned unchanged.
func TestFocusTransitionProgressMidTransition(t *testing.T) {
	f := newFocusTransition(0, FocusLeft, FocusRight)
	f.pos = 0.5
	if got := f.progress(); got != 0.5 {
		t.Errorf("progress() = %v, want 0.5", got)
	}
}

// TestFocusTransitionNotSettledAtStart verifies a freshly-created transition
// (pos=0, target=1) is not yet settled — the tick loop must keep running.
func TestFocusTransitionNotSettledAtStart(t *testing.T) {
	f := newFocusTransition(0, FocusLeft, FocusRight)
	if f.settled() {
		t.Error("settled() = true for a freshly-created transition, want false")
	}
}

// TestFocusTransitionSettledAtRest verifies a transition sitting exactly at
// its equilibrium with zero velocity is reported as settled, so the tick
// loop stops re-arming itself once the spring has actually come to rest.
func TestFocusTransitionSettledAtRest(t *testing.T) {
	f := newFocusTransition(0, FocusLeft, FocusRight)
	f.pos = 1
	f.vel = 0
	if !f.settled() {
		t.Error("settled() = false at exact rest position, want true")
	}
}

// TestFocusTransitionStepMovesTowardTarget checks that a single step()
// advances pos toward the target (1) from the starting position (0) — the
// spring should never step backward from rest.
func TestFocusTransitionStepMovesTowardTarget(t *testing.T) {
	f := newFocusTransition(0, FocusLeft, FocusRight)
	f.step()
	if f.pos <= 0 {
		t.Errorf("pos after one step() = %v, want > 0 (moving toward target)", f.pos)
	}
}

// TestNewFocusTransitionIncrementsGeneration verifies the generation counter
// is bumped relative to whatever generation was passed in, so a superseded
// transition's in-flight focusTickMsg values become stale (see the
// generation-guard test below).
func TestNewFocusTransitionIncrementsGeneration(t *testing.T) {
	f := newFocusTransition(5, FocusLeft, FocusRight)
	if f.gen != 6 {
		t.Errorf("gen = %d, want 6", f.gen)
	}
}

// TestNewFocusTransitionRecordsFromAndTo checks the transition remembers
// which pane it is animating away from and toward, since Render uses these
// to decide which pane's fraction is anim.Progress vs 1-anim.Progress.
func TestNewFocusTransitionRecordsFromAndTo(t *testing.T) {
	f := newFocusTransition(0, FocusLeft, FocusRight)
	if f.from != FocusLeft {
		t.Errorf("from = %v, want FocusLeft", f.from)
	}
	if f.to != FocusRight {
		t.Errorf("to = %v, want FocusRight", f.to)
	}
}

// TestLerpColourAtZeroReturnsFrom checks the t<=0 short-circuit returns the
// exact `from` colour rather than routing it through the colorful blend
// (which could introduce tiny float rounding, breaking the "frac==0 renders
// identically to the old flat border" guarantee paneStyle relies on).
func TestLerpColourAtZeroReturnsFrom(t *testing.T) {
	from := color.RGBA{R: 10, G: 20, B: 30, A: 255}
	to := color.RGBA{R: 200, G: 210, B: 220, A: 255}
	got := lerpColour(from, to, 0)
	if got != color.Color(from) {
		t.Errorf("lerpColour(from, to, 0) = %v, want %v", got, from)
	}
}

// TestLerpColourAtOneReturnsTo mirrors the above for the t>=1 short-circuit.
func TestLerpColourAtOneReturnsTo(t *testing.T) {
	from := color.RGBA{R: 10, G: 20, B: 30, A: 255}
	to := color.RGBA{R: 200, G: 210, B: 220, A: 255}
	got := lerpColour(from, to, 1)
	if got != color.Color(to) {
		t.Errorf("lerpColour(from, to, 1) = %v, want %v", got, to)
	}
}

// TestLerpColourMidpointIsBetween checks that an interior t produces a
// colour whose channels sit strictly between `from` and `to` — a coarse
// sanity check that BlendLuv is actually interpolating rather than, say,
// silently returning one endpoint for every t.
func TestLerpColourMidpointIsBetween(t *testing.T) {
	from := color.RGBA{R: 10, G: 10, B: 10, A: 255}
	to := color.RGBA{R: 250, G: 250, B: 250, A: 255}
	got := lerpColour(from, to, 0.5)

	r, g, b, _ := got.RGBA()
	// RGBA() returns 16-bit channels; from/to are 8-bit, so compare against
	// the 16-bit-expanded bounds.
	fr, fg, fb, _ := color.Color(from).RGBA()
	tr, tg, tb, _ := color.Color(to).RGBA()
	if !between(r, fr, tr) || !between(g, fg, tg) || !between(b, fb, tb) {
		t.Errorf("lerpColour midpoint (%d,%d,%d) not between from (%d,%d,%d) and to (%d,%d,%d)",
			r, g, b, fr, fg, fb, tr, tg, tb)
	}
}

func between(v, lo, hi uint32) bool {
	if lo > hi {
		lo, hi = hi, lo
	}
	return v >= lo && v <= hi
}
