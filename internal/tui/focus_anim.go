package tui

import (
	"image/color"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/harmonica"
	"github.com/lucasb-eyer/go-colorful"
)

// focusAnimFPS is the tick rate driving the spring simulation. 60fps keeps a
// ~150ms transition to a handful of visually smooth steps without spamming
// the terminal with redraws.
const focusAnimFPS = 60

// focusAnimFreq and focusAnimDamping tune the spring for a ~150ms settle with
// no overshoot — critically damped (damping ratio 1) reaches equilibrium as
// fast as possible without oscillating, which reads as a clean fade rather
// than a bouncy one for a border colour/width transition.
const (
	focusAnimFreq    = 18.0
	focusAnimDamping = 1.0
)

// focusAnimSettleThreshold is how close position and velocity must be to
// their resting values before the transition is considered complete and the
// tick loop stops re-arming itself.
const focusAnimSettleThreshold = 0.001

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
