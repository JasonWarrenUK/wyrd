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

// TestCrossDissolveAtZeroReturnsFromExactly checks the t<=0 short-circuit
// returns `from` byte-for-byte, so a settled/never-animated pane renders
// identically to the pre-VP.6 flat-border output.
func TestCrossDissolveAtZeroReturnsFromExactly(t *testing.T) {
	from := "\x1b[38;2;10;20;30mA\x1b[m"
	to := "\x1b[38;2;200;210;220mA\x1b[m"
	if got := crossDissolveRendered(from, to, 0); got != from {
		t.Errorf("crossDissolveRendered(from, to, 0) = %q, want %q", got, from)
	}
}

// TestCrossDissolveAtOneReturnsToExactly mirrors the above for t>=1.
func TestCrossDissolveAtOneReturnsToExactly(t *testing.T) {
	from := "\x1b[38;2;10;20;30mA\x1b[m"
	to := "\x1b[38;2;200;210;220mA\x1b[m"
	if got := crossDissolveRendered(from, to, 1); got != to {
		t.Errorf("crossDissolveRendered(from, to, 1) = %q, want %q", got, to)
	}
}

// TestCrossDissolveFlatVsGradientDoesNotCollapse is a regression test for
// the anchor-desync bug (VP.6 follow-up): a flat BorderForeground render
// collapses same-coloured consecutive characters into ONE escape run
// covering the whole span, while a BorderForegroundBlend render emits one
// escape PER character. Zipping by escape-run index (the first
// implementation of crossDissolveRendered) paired the flat side's single
// multi-character run against only the gradient side's first character,
// so the entire span rendered as one uniform blended colour instead of a
// per-position fade — exactly the "colour patch detaches and travels"
// artefact this function exists to prevent. Expanding to one cell per rune
// before zipping (ansiColourCells) fixes this: each of the 3 characters
// below must end up a DIFFERENT colour, tracking its own position in the
// gradient side, not one shared colour for the whole run.
func TestCrossDissolveFlatVsGradientDoesNotCollapse(t *testing.T) {
	flat := "\x1b[38;2;0;68;60mABC\x1b[m"
	gradient := "\x1b[38;2;0;158;140mA\x1b[38;2;90;150;110mB\x1b[38;2;190;127;46mC\x1b[m"

	got := crossDissolveRendered(flat, gradient, 0.5)
	cells := ansiColourCells(got)

	var coloured []color.Color
	for _, c := range cells {
		if c.colour != nil {
			coloured = append(coloured, c.colour)
		}
	}
	if len(coloured) != 3 {
		t.Fatalf("expected 3 coloured cells in blended output, got %d (%v)", len(coloured), coloured)
	}
	if coloured[0] == coloured[1] || coloured[1] == coloured[2] || coloured[0] == coloured[2] {
		t.Errorf("blended cells collapsed to a shared colour instead of tracking each position independently: %v", coloured)
	}
}

// TestCrossDissolveEachCellStaysBetweenItsOwnEndpoints checks that no
// blended cell's colour falls outside the range spanned by that SAME
// position's own from/to colours — the core correctness guarantee that
// prevents any position's colour from racing ahead of or lagging behind
// its neighbours in a way that would look like a travelling patch.
func TestCrossDissolveEachCellStaysBetweenItsOwnEndpoints(t *testing.T) {
	from := "\x1b[38;2;0;68;60mAB\x1b[m"
	to := "\x1b[38;2;0;158;140mA\x1b[38;2;190;127;46mB\x1b[m"

	for _, frac := range []float64{0.1, 0.3, 0.5, 0.7, 0.9} {
		got := crossDissolveRendered(from, to, frac)
		gotCells := ansiColourCells(got)
		fromCells := ansiColourCells(from)
		toCells := ansiColourCells(to)

		for i := range gotCells {
			gr, gg, gb, _ := gotCells[i].colour.RGBA()
			fr, fg, fb, _ := fromCells[i].colour.RGBA()
			tr, tg, tb, _ := toCells[i].colour.RGBA()
			if !between(gr, fr, tr) || !between(gg, fg, tg) || !between(gb, fb, tb) {
				t.Errorf("frac=%.1f cell[%d] colour out of its own [from,to] range: got=(%d,%d,%d) from=(%d,%d,%d) to=(%d,%d,%d)",
					frac, i, gr, gg, gb, fr, fg, fb, tr, tg, tb)
			}
		}
	}
}
