package types

// CycleBehaviour describes what happens when a node advances past the final
// stage of its group, or retreats before the first.
type CycleBehaviour string

const (
	// CycleLoop wraps around: advancing past the last stage returns to the
	// first, and retreating before the first returns to the last. Suits
	// recurring progressions like habits that never truly "finish".
	CycleLoop CycleBehaviour = "loop"

	// CycleTerminate stops at the boundary: advancing past the last stage
	// stays on the last, and retreating before the first stays on the first.
	// Suits one-shot progressions like tasks that reach a terminal Done.
	CycleTerminate CycleBehaviour = "terminate"

	// CycleLoopToStage wraps to a named stage rather than the first. Advancing
	// past the last stage returns to LoopTarget. Suits progressions where the
	// reset point is partway through (e.g. skip an initial intake stage).
	CycleLoopToStage CycleBehaviour = "loop-to-stage"
)

// StageGroup is a named, ordered progression of stages with a defined cycle
// behaviour at its boundaries. Kinds reference a stage group to determine which
// stages their nodes may occupy and how the advance/retreat keypresses move
// between them (SL.6). The default groups ship as embedded JSONC in SL.3.
type StageGroup struct {
	// Name is the group's unique identifier (e.g. "task-flow").
	Name string `json:"name"`

	// Stages is the ordered list of stage names. The first element is the
	// initial stage assigned when a kind using this group is selected (SL.7).
	// Must contain at least one stage.
	Stages []string `json:"stages"`

	// Cycle controls boundary behaviour for Next and Prev.
	Cycle CycleBehaviour `json:"cycle"`

	// LoopTarget names the stage that advancing past the last stage returns to.
	// Only consulted when Cycle is CycleLoopToStage; ignored otherwise. Must
	// name a stage present in Stages.
	LoopTarget string `json:"loop_target,omitempty"`
}

// indexOf returns the position of stage in the group, or -1 if absent.
func (g StageGroup) indexOf(stage string) int {
	for i, s := range g.Stages {
		if s == stage {
			return i
		}
	}
	return -1
}

// Contains reports whether stage is present in the group.
func (g StageGroup) Contains(stage string) bool {
	return g.indexOf(stage) >= 0
}

// IsTerminal reports whether stage is a terminal stage of the group: one from
// which advancing cannot move forward. For CycleTerminate groups this is the
// last stage; looping groups have no terminal stage. An unknown stage is not
// terminal. DL.1 uses terminality to decide whether a blocking node still
// blocks its target.
func (g StageGroup) IsTerminal(stage string) bool {
	if g.Cycle != CycleTerminate {
		return false
	}
	i := g.indexOf(stage)
	return i >= 0 && i == len(g.Stages)-1
}

// Next returns the stage that follows current, honouring the group's cycle
// behaviour at the upper boundary. The bool is false when current is unknown to
// the group, in which case the returned string is empty and the caller should
// leave the node's stage unchanged.
//
// For CycleTerminate at the last stage, Next returns the last stage with true:
// the move is valid but idempotent. For CycleLoop it returns the first stage;
// for CycleLoopToStage it returns LoopTarget (falling back to the first stage
// if LoopTarget is empty or not found).
func (g StageGroup) Next(current string) (string, bool) {
	i := g.indexOf(current)
	if i < 0 {
		return "", false
	}
	if i < len(g.Stages)-1 {
		return g.Stages[i+1], true
	}
	// At the last stage; behaviour depends on the cycle.
	switch g.Cycle {
	case CycleLoop:
		return g.Stages[0], true
	case CycleLoopToStage:
		if t := g.indexOf(g.LoopTarget); t >= 0 {
			return g.LoopTarget, true
		}
		return g.Stages[0], true
	default: // CycleTerminate
		return current, true
	}
}

// Prev returns the stage that precedes current, honouring the group's cycle
// behaviour at the lower boundary. The bool is false when current is unknown to
// the group.
//
// For CycleTerminate at the first stage, Prev returns the first stage with
// true. For CycleLoop and CycleLoopToStage it wraps to the last stage —
// retreat is the symmetric inverse of advancing off the end.
func (g StageGroup) Prev(current string) (string, bool) {
	i := g.indexOf(current)
	if i < 0 {
		return "", false
	}
	if i > 0 {
		return g.Stages[i-1], true
	}
	// At the first stage; behaviour depends on the cycle.
	switch g.Cycle {
	case CycleLoop, CycleLoopToStage:
		return g.Stages[len(g.Stages)-1], true
	default: // CycleTerminate
		return current, true
	}
}
