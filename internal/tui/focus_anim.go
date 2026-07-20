package tui

import (
	"image/color"
	"regexp"
	"strconv"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/harmonica"
	"github.com/lucasb-eyer/go-colorful"
)

// focusAnimFPS is the tick rate driving the spring simulation. 60fps keeps a
// ~150ms transition to a handful of visually smooth steps without spamming
// the terminal with redraws.
const focusAnimFPS = 60

// focusAnimFreq and focusAnimDamping tune the spring — critically damped
// (damping ratio 1) reaches equilibrium without oscillating, which reads as
// a clean fade rather than a bouncy one for a border colour/width
// transition. freq=18 (VP.6's original tuning) reaches 90% of the way to
// the target by ~217ms but only reaches full stop past 500ms — the loud,
// visible part of the motion was over almost as soon as it started, then a
// long imperceptible tail ran on doing nothing, which read as "instant"
// rather than as an eased transition. freq=11 spreads that same visible
// 0→90% sweep across ~367ms, which is slow enough to clearly read as a
// fade without dragging.
const (
	focusAnimFreq    = 11.0
	focusAnimDamping = 1.0
)

// focusAnimSettleThreshold is how close position and velocity must be to
// their resting values before the transition is considered complete and the
// tick loop stops re-arming itself. A critically-damped spring's tail decays
// slowly regardless of this threshold (tightening it below ~0.01 buys
// negligible perceptual difference but costs real extra ticks), so this is
// set loose enough to stop the loop promptly once motion is no longer
// visible rather than chasing mathematical exactness nobody can see.
const focusAnimSettleThreshold = 0.01

// focusTickMsg drives one step of the focus-transition spring. gen is
// generation-guarded (mirrors captureConfirmClearMsg) so that if focus flips
// again mid-transition, the superseded tick loop's stale messages are
// dropped rather than stepping a spring nobody cares about anymore.
type focusTickMsg struct {
	gen int
}

// focusTransition holds the spring simulation for the pane focus-border
// animation. A single instance is shared across both panes: pos 0 means
// "fully at the from-pane's rest state", pos 1 means "fully at the to-pane's
// rest state" (i.e. the gaining pane's gradient border + width nudge fully
// applied, the losing pane's flat border + width fully restored).
type focusTransition struct {
	spring harmonica.Spring
	pos    float64
	vel    float64

	// from and to are the panes losing and gaining focus.
	from, to FocusedPane

	// gen increments every time a new transition starts, invalidating any
	// in-flight tick loop for a previous transition.
	gen int
}

// newFocusTransition starts a fresh spring animating focus from `from` to
// `to`. gen is bumped so any previously scheduled focusTickMsg is ignored.
func newFocusTransition(prevGen int, from, to FocusedPane) *focusTransition {
	return &focusTransition{
		spring: harmonica.NewSpring(harmonica.FPS(focusAnimFPS), focusAnimFreq, focusAnimDamping),
		pos:    0,
		vel:    0,
		from:   from,
		to:     to,
		gen:    prevGen + 1,
	}
}

// tick returns the tea.Cmd that fires the next simulation step.
func (f *focusTransition) tick() tea.Cmd {
	gen := f.gen
	return tea.Tick(time.Second/focusAnimFPS, func(_ time.Time) tea.Msg {
		return focusTickMsg{gen: gen}
	})
}

// step advances the spring by one tick toward equilibrium (pos=1).
func (f *focusTransition) step() {
	f.pos, f.vel = f.spring.Update(f.pos, f.vel, 1)
}

// settled reports whether the spring has effectively reached equilibrium, at
// which point the tick loop should stop re-arming itself.
func (f *focusTransition) settled() bool {
	return abs(1-f.pos) < focusAnimSettleThreshold && abs(f.vel) < focusAnimSettleThreshold
}

// progress returns the animation position clamped to [0,1] for rendering —
// the raw spring position can briefly dip outside that range with certain
// damping ratios, and callers interpolating colours/widths need a clean
// fraction.
func (f *focusTransition) progress() float64 {
	switch {
	case f.pos < 0:
		return 0
	case f.pos > 1:
		return 1
	default:
		return f.pos
	}
}

func abs(v float64) float64 {
	if v < 0 {
		return -v
	}
	return v
}

// lerpColour blends from toward to at fraction t (0=from, 1=to) in the Luv
// colour space via go-colorful's BlendLuv, which interpolates perceptually
// rather than naively averaging RGB channels (avoids the muddy/grey midpoint
// naive RGB lerp produces between saturated theme colours).
func lerpColour(from, to color.Color, t float64) color.Color {
	if t <= 0 {
		return from
	}
	if t >= 1 {
		return to
	}
	fc, _ := colorful.MakeColor(from)
	tc, _ := colorful.MakeColor(to)
	return fc.BlendLuv(tc, t).Clamped()
}

// crossDissolveRendered blends two fully-rendered ANSI strings (the pane's
// flat-border rest state and its gradient-border rest state, both rendered
// at the SAME box dimensions so their glyphs line up 1:1) at fraction t,
// interpolating each character's own truecolor foreground independently.
//
// This exists because BorderForegroundBlend spreads a single gradient
// across the WHOLE border perimeter (lipgloss.Blend1D under the hood), so
// re-deriving that gradient's 3 anchor stops from lerpColour every frame
// (VP.6's original approach) let the anchors drift out of sync with each
// other whenever a theme's border colour sits closer to one accent than the
// other — visually, a colour patch would appear to detach from the
// perimeter and travel, most visibly in the "peat" theme (border sits very
// close to accent.primary but far from accent.secondary). Blending each
// border position independently between its OWN two fixed endpoint colours
// makes that desync structurally impossible: a position that starts near
// its resting colour stays near it throughout, regardless of how far any
// other position has to travel.
//
// Only the border glyphs carry per-character truecolor codes (see
// paneStyle/detailPaneStyle) — content lines are unstyled at this level and
// pass through unchanged, so it's correct to fall back to `from`'s raw text
// whenever a position has no parseable foreground code on either side.
func crossDissolveRendered(from, to string, t float64) string {
	if t <= 0 {
		return from
	}
	if t >= 1 {
		return to
	}

	fromCells := ansiColourCells(from)
	toCells := ansiColourCells(to)

	var out []byte
	n := len(fromCells)
	if len(toCells) < n {
		n = len(toCells)
	}
	for i := 0; i < n; i++ {
		fc, tc := fromCells[i], toCells[i]
		switch {
		case fc.colour != nil && tc.colour != nil:
			blended := lerpColour(fc.colour, tc.colour, t)
			// Substitute only the foreground triple within fc's original
			// escape sequence, leaving any background (BorderBackground sets
			// a combined "38;2;...;48;2;...m" SGR — see the
			// crossDissolveRendered doc) or other attributes untouched.
			r, g, b, _ := blended.RGBA()
			newFg := "38;2;" + strconv.Itoa(int(r>>8)) + ";" + strconv.Itoa(int(g>>8)) + ";" + strconv.Itoa(int(b>>8))
			blendedEsc := ansiTruecolourFgRe.ReplaceAllString(fc.escape, newFg)
			out = append(out, blendedEsc...)
			out = append(out, string(fc.char)...)
		default:
			// Non-border (uncoloured) cell, or a cell only one side
			// coloured — pass the `from` side's original escape+char
			// through untouched rather than guess at a blend with no
			// matching endpoint.
			out = append(out, fc.escape...)
			out = append(out, string(fc.char)...)
		}
	}
	// If the two renders had a differing cell count (should not happen when
	// called with matching dimensions, but defensive), append whatever
	// remains of the longer one unblended rather than truncate the frame.
	if len(fromCells) > n {
		for _, fc := range fromCells[n:] {
			out = append(out, fc.escape...)
			out = append(out, string(fc.char)...)
		}
	}
	out = append(out, "\x1b[m"...)

	return string(out)
}

// ansiEscapeRe matches a single ANSI CSI escape sequence (e.g. "\x1b[38;2;
// 0;158;140m" or the bare reset "\x1b[m").
var ansiEscapeRe = regexp.MustCompile("\x1b\\[[0-9;]*m")

// ansiTruecolourFgRe extracts the r;g;b triple from a truecolor foreground
// escape's parameter list (e.g. matches inside "38;2;0;158;140").
var ansiTruecolourFgRe = regexp.MustCompile(`38;2;(\d+);(\d+);(\d+)`)

// ansiRun is one leading-escape-sequence-plus-literal-text unit as emitted
// by lipgloss's border renderer: each border character gets its own fresh
// foreground escape immediately before it (see the package doc on
// crossDissolveRendered), so splitting on escape boundaries yields exactly
// one run per rendered character (or per uncoloured span of plain text).
type ansiRun struct {
	raw    string      // the run exactly as it appeared (escape + text)
	escape string      // just the leading escape sequence, if any
	text   string      // just the literal text portion (no escapes)
	colour color.Color // the run's truecolor foreground, or nil if none
}

// ansiColourRuns splits s into ansiRun units at each ANSI escape boundary.
// Any run whose leading escape(s) include a "38;2;r;g;b" truecolor
// foreground gets that colour attached; runs are additionally forced to
// close at the next escape (rather than accumulating unbounded literal
// text) so this cleanly matches lipgloss's one-escape-per-border-char
// output without needing to track SGR reset state across runs.
func ansiColourRuns(s string) []ansiRun {
	locs := ansiEscapeRe.FindAllStringIndex(s, -1)
	if len(locs) == 0 {
		return []ansiRun{{raw: s, text: s}}
	}

	var runs []ansiRun
	// Any literal text before the first escape (shouldn't happen in
	// lipgloss's border output, but handled defensively).
	if locs[0][0] > 0 {
		runs = append(runs, ansiRun{raw: s[:locs[0][0]], text: s[:locs[0][0]]})
	}

	for i, loc := range locs {
		escStart, escEnd := loc[0], loc[1]
		textEnd := len(s)
		if i+1 < len(locs) {
			textEnd = locs[i+1][0]
		}
		esc := s[escStart:escEnd]
		text := s[escEnd:textEnd]
		run := ansiRun{raw: s[escStart:textEnd], escape: esc, text: text}
		if m := ansiTruecolourFgRe.FindStringSubmatch(esc); m != nil {
			r, _ := strconv.Atoi(m[1])
			g, _ := strconv.Atoi(m[2])
			b, _ := strconv.Atoi(m[3])
			run.colour = color.RGBA{R: uint8(r), G: uint8(g), B: uint8(b), A: 255}
		}
		runs = append(runs, run)
	}

	return runs
}

// ansiCell is one displayed character (rune) plus the escape sequence that
// applies to it — lipgloss's flat BorderForeground collapses a whole run of
// same-coloured characters under a single leading escape (see the
// crossDissolveRendered doc for why this distinction matters), so an
// ansiRun with multi-rune text expands to one ansiCell per rune, each
// carrying a COPY of that run's escape/colour. This gives both the flat and
// gradient renders the same per-position addressing, so crossDissolveRendered
// can zip them by index without the run-count mismatch that a flat border's
// single collapsed run would otherwise cause.
type ansiCell struct {
	escape string
	char   rune
	colour color.Color
}

// ansiColourCells expands s's ansiColourRuns into one ansiCell per rune.
func ansiColourCells(s string) []ansiCell {
	runs := ansiColourRuns(s)
	var cells []ansiCell
	for _, run := range runs {
		for _, r := range run.text {
			cells = append(cells, ansiCell{escape: run.escape, char: r, colour: run.colour})
		}
	}
	return cells
}
