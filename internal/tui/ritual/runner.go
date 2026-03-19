package ritual

import (
	"fmt"

	"github.com/google/uuid"
	"github.com/jasonwarrenuk/wyrd/internal/types"
)

// RunnerState enumerates the overall state the runner can be in.
type RunnerState int

const (
	// StateIdle means no ritual is active.
	StateIdle RunnerState = iota

	// StateRunning means a ritual is in progress and the user is interacting
	// with the current step.
	StateRunning

	// StateDeferred means the user has invoked the defer sequence on a gate
	// ritual. The ritual is paused and the deferral is recorded.
	StateDeferred

	// StateComplete means all steps have been processed successfully.
	StateComplete
)

// DeferSequence is the key sequence a user must type to defer a gate ritual.
// Pressing Esc twice followed by 'd' is intentionally awkward so that
// deferral requires a deliberate action.
const DeferSequence = "Esc Esc d"

// Runner is the state machine that advances a ritual through its step array.
// It is designed for pure unit testing — it holds no bubbletea model and
// makes no direct I/O calls. The TUI layer drives it by calling Step methods
// and reading output via CurrentOutput.
type Runner struct {
	ritual  *types.Ritual
	store   types.StoreFS
	query   types.QueryRunner
	clock   types.Clock

	// stepIndex is the 0-based index of the step currently being executed.
	stepIndex int

	// state is the overall runner state.
	state RunnerState

	// deferKeyBuffer accumulates key presses for the defer sequence check.
	// It is reset on any non-sequence key.
	deferKeyBuffer string

	// currentOutput holds the rendered string for the current step so the TUI
	// can display it without re-running the step.
	currentOutput string

	// currentItems holds the mutable list items for a query_list step.
	currentItems []*ListItem

	// selectedItemIndex tracks the cursor in a query_list step.
	selectedItemIndex int

	// edgePicking is true when the user has pressed 'e' and we are waiting for
	// a target node to be selected.
	edgePicking bool

	// edgeTypeIndex is the index into EdgeTypes for the in-progress edge.
	edgeTypeIndex int
}

// NewRunner creates a Runner for the given ritual.
func NewRunner(
	ritual *types.Ritual,
	store types.StoreFS,
	query types.QueryRunner,
	clock types.Clock,
) *Runner {
	return &Runner{
		ritual: ritual,
		store:  store,
		query:  query,
		clock:  clock,
		state:  StateIdle,
	}
}

// Start activates the runner and executes the first step.
func (r *Runner) Start() error {
	r.stepIndex = 0
	r.state = StateRunning
	return r.executeCurrentStep()
}

// CurrentState returns the runner's overall state.
func (r *Runner) CurrentState() RunnerState {
	return r.state
}

// CurrentOutput returns the rendered string for the current step.
func (r *Runner) CurrentOutput() string {
	return r.currentOutput
}

// CurrentItems returns the mutable list items for a query_list step.
// Returns nil when the current step is not a query_list.
func (r *Runner) CurrentItems() []*ListItem {
	return r.currentItems
}

// SelectedIndex returns the cursor index within the current list.
func (r *Runner) SelectedIndex() int {
	return r.selectedItemIndex
}

// Ritual returns the ritual being run.
func (r *Runner) Ritual() *types.Ritual {
	return r.ritual
}

// IsGate reports whether the ritual has gate friction, meaning the user must
// complete or explicitly defer before the runner releases focus.
func (r *Runner) IsGate() bool {
	return r.ritual.Friction == types.FrictionGate
}

// Advance moves to the next step. If there are no more steps the runner
// transitions to StateComplete and commits any pending list edits.
func (r *Runner) Advance() error {
	if r.state != StateRunning {
		return nil
	}

	// Commit any pending list edits before leaving the current step.
	if err := r.commitCurrentListEdits(); err != nil {
		// Non-fatal: log the errors but continue advancing.
		_ = err
	}

	r.stepIndex++

	steps := r.effectiveSteps()
	if r.stepIndex >= len(steps) {
		r.state = StateComplete
		r.currentOutput = ""
		r.currentItems = nil
		return nil
	}

	return r.executeCurrentStep()
}

// HandleKey processes a key press from the TUI. It dispatches to the
// relevant step handler and detects the defer sequence for gate rituals.
// Returns an action hint string that the TUI can use to update its display
// ("advance", "redraw", "none", "deferred").
func (r *Runner) HandleKey(key string) (string, error) {
	if r.state != StateRunning {
		return "none", nil
	}

	// Gate friction: accumulate defer-sequence presses.
	if r.IsGate() {
		action, err := r.checkDeferSequence(key)
		if err != nil {
			return "none", err
		}
		if action == "deferred" {
			return "deferred", nil
		}
	}

	step := r.currentStep()
	if step == nil {
		return "none", nil
	}

	switch step.Type {
	case types.StepQueryList:
		return r.handleListKey(key)
	case types.StepPrompt:
		// Prompt input is managed by the TUI layer; Runner only processes submit.
		if key == "enter" {
			return "advance", nil
		}
	default:
		// Summary, view, action, gate — any key advances.
		if key == "enter" || key == " " {
			return "advance", nil
		}
	}

	return "none", nil
}

// SubmitPrompt handles the submission of a prompt step's text input. It
// creates a new node with the specified types and optional edge.
func (r *Runner) SubmitPrompt(text string) error {
	step := r.currentStep()
	if step == nil || step.Type != types.StepPrompt {
		return fmt.Errorf("current step is not a prompt")
	}

	if step.Creates == nil || len(step.Creates.Types) == 0 {
		// Nothing to create; just advance.
		return r.Advance()
	}

	now := r.clock.Now()
	node := &types.Node{
		ID:         uuid.New().String(),
		Body:       text,
		Types:      step.Creates.Types,
		Created:    now,
		Modified:   now,
		Properties: make(map[string]interface{}),
	}

	if err := r.store.WriteNode(node); err != nil {
		return fmt.Errorf("writing prompt node: %w", err)
	}

	if step.Creates.Edge != nil && step.Creates.Edge.To != "" {
		edge := &types.Edge{
			ID:      uuid.New().String(),
			Type:    step.Creates.Edge.Type,
			From:    node.ID,
			To:      step.Creates.Edge.To,
			Created: now,
		}
		if err := r.store.WriteEdge(edge); err != nil {
			return fmt.Errorf("writing prompt edge: %w", err)
		}
	}

	return r.Advance()
}

// TryDefer attempts to defer the ritual. Gate rituals require the full defer
// sequence; nudge rituals can be deferred with a single keypress.
// Returns true if the deferral succeeded.
func (r *Runner) TryDefer() bool {
	if r.ritual.Friction == types.FrictionNudge {
		r.state = StateDeferred
		return true
	}
	// Gate ritual: deferral must come through the key sequence.
	return false
}

// ForceDefer unconditionally defers the ritual regardless of friction level.
// Used after the defer key sequence has been confirmed.
func (r *Runner) ForceDefer() {
	r.state = StateDeferred
}

// ---- private helpers --------------------------------------------------------

// effectiveSteps returns the ritual's steps, expanding any gate sub-steps
// inline based on their condition results. For simplicity the gate expansion
// happens lazily in executeCurrentStep; this method returns the base slice.
func (r *Runner) effectiveSteps() []types.RitualStep {
	return r.ritual.Steps
}

// currentStep returns a pointer to the currently active RitualStep, or nil
// if the index is out of bounds.
func (r *Runner) currentStep() *types.RitualStep {
	steps := r.effectiveSteps()
	if r.stepIndex < 0 || r.stepIndex >= len(steps) {
		return nil
	}
	s := steps[r.stepIndex]
	return &s
}

// executeCurrentStep runs the step at stepIndex and populates currentOutput /
// currentItems.
func (r *Runner) executeCurrentStep() error {
	step := r.currentStep()
	if step == nil {
		r.state = StateComplete
		return nil
	}

	r.currentItems = nil
	r.selectedItemIndex = 0
	r.edgePicking = false

	switch step.Type {
	case types.StepQuerySummary:
		return r.executeQuerySummary(step)
	case types.StepQueryList:
		return r.executeQueryList(step)
	case types.StepView:
		return r.executeView(step)
	case types.StepAction:
		return r.executeAction(step)
	case types.StepGate:
		return r.executeGate(step)
	case types.StepPrompt:
		r.currentOutput = step.Text
		return nil
	default:
		return fmt.Errorf("unknown step type: %q", step.Type)
	}
}

func (r *Runner) executeQuerySummary(step *types.RitualStep) error {
	result, err := r.query.Run(step.Query, r.clock)
	if err != nil {
		return fmt.Errorf("query_summary query: %w", err)
	}

	r.currentOutput = InterpolateTemplate(step.Template, result)
	return nil
}

func (r *Runner) executeQueryList(step *types.RitualStep) error {
	result, err := r.query.Run(step.Query, r.clock)
	if err != nil {
		return fmt.Errorf("query_list query: %w", err)
	}

	nodeLoader := func(id string) (*types.Node, error) {
		return r.store.ReadNode(id)
	}

	r.currentItems = BuildListItems(result, nodeLoader)

	if len(r.currentItems) == 0 {
		r.currentOutput = step.EmptyMessage
		return nil
	}

	r.currentOutput = step.Label
	return nil
}

func (r *Runner) executeView(step *types.RitualStep) error {
	// Stub: load the saved view definition and render as a list label.
	view, err := r.store.ReadView(step.View)
	if err != nil {
		r.currentOutput = fmt.Sprintf("[view %q not found]", step.View)
		return nil
	}

	// Run the view's query to get results.
	result, err := r.query.Run(view.Query, r.clock)
	if err != nil {
		r.currentOutput = fmt.Sprintf("[view %q query error: %v]", step.View, err)
		return nil
	}

	nodeLoader := func(id string) (*types.Node, error) {
		return r.store.ReadNode(id)
	}

	r.currentItems = BuildListItems(result, nodeLoader)
	r.currentOutput = view.Name
	return nil
}

func (r *Runner) executeAction(step *types.RitualStep) error {
	// All v1 actions are stubbed.
	switch step.Action {
	case "propose_schedule":
		r.currentOutput = "[propose_schedule: not yet implemented]"
	case "open_triage":
		r.currentOutput = "[open_triage: not yet implemented]"
	case "open_journal":
		r.currentOutput = "[open_journal: not yet implemented]"
	case "sync":
		r.currentOutput = "[sync: not yet implemented]"
	case "rebuild_index":
		r.currentOutput = "[rebuild_index: not yet implemented]"
	default:
		r.currentOutput = fmt.Sprintf("[unknown action: %q]", step.Action)
	}
	return nil
}

func (r *Runner) executeGate(step *types.RitualStep) error {
	if step.Condition == "" {
		return fmt.Errorf("gate step missing condition")
	}

	result, err := r.query.Run(step.Condition, r.clock)
	if err != nil {
		return fmt.Errorf("gate condition query: %w", err)
	}

	// The gate passes if the result has at least one row and the boolean field
	// evaluates to true.
	passed := evaluateGateResult(result, step.Field)

	if passed && step.Then != nil {
		// Inline-execute the then sub-step.
		return r.executeSubStep(step.Then)
	}

	// Gate did not pass — advance automatically.
	r.currentOutput = ""
	return r.Advance()
}

// executeSubStep runs a sub-step (the Then branch of a gate).
func (r *Runner) executeSubStep(step *types.RitualStep) error {
	switch step.Type {
	case types.StepQuerySummary:
		return r.executeQuerySummary(step)
	case types.StepQueryList:
		return r.executeQueryList(step)
	case types.StepPrompt:
		r.currentOutput = step.Text
	default:
		r.currentOutput = fmt.Sprintf("[sub-step type %q not supported in gate.then]", step.Type)
	}
	return nil
}

// evaluateGateResult inspects a QueryResult for a truthy boolean value in
// field. Returns false if the result is empty or the field is absent.
func evaluateGateResult(result *types.QueryResult, field string) bool {
	if result == nil || len(result.Rows) == 0 {
		return false
	}

	row := result.Rows[0]

	if field == "" {
		// No specific field requested — treat any non-empty row as true.
		return len(row) > 0
	}

	val, ok := row[field]
	if !ok {
		return false
	}

	switch v := val.(type) {
	case bool:
		return v
	case int:
		return v != 0
	case int64:
		return v != 0
	case float64:
		return v != 0
	case string:
		return v != "" && v != "false"
	}
	return false
}

// handleListKey handles keyboard input for a query_list step.
func (r *Runner) handleListKey(key string) (string, error) {
	items := r.currentItems
	if len(items) == 0 {
		if key == "enter" {
			return "advance", nil
		}
		return "none", nil
	}

	switch key {
	case "up", "k":
		if r.selectedItemIndex > 0 {
			r.selectedItemIndex--
		}
		return "redraw", nil

	case "down", "j":
		if r.selectedItemIndex < len(items)-1 {
			r.selectedItemIndex++
		}
		return "redraw", nil

	case "s":
		// Cycle status on selected item.
		CycleStatus(items[r.selectedItemIndex])
		return "redraw", nil

	case "x":
		// Quick-archive selected item.
		QuickArchive(items[r.selectedItemIndex])
		return "redraw", nil

	case "e":
		// Begin edge creation: cycle to next edge type.
		if !r.edgePicking {
			r.edgePicking = true
			r.edgeTypeIndex = 0
		} else {
			r.edgeTypeIndex = (r.edgeTypeIndex + 1) % len(EdgeTypes)
		}
		return "redraw", nil

	case "enter":
		if r.edgePicking {
			// Confirm edge type; target selection happens outside the runner.
			r.edgePicking = false
		}
		return "advance", nil
	}

	return "none", nil
}

// commitCurrentListEdits writes any pending item mutations to the store.
func (r *Runner) commitCurrentListEdits() []error {
	if len(r.currentItems) == 0 {
		return nil
	}
	return CommitListEdits(r.currentItems, r.store, r.clock)
}

// checkDeferSequence checks whether the key presses so far match the beginning
// or end of DeferSequence. Returns "deferred" if the full sequence is complete.
func (r *Runner) checkDeferSequence(key string) (string, error) {
	// The defer sequence tokens separated by space: ["Esc", "Esc", "d"]
	tokens := []string{"Esc", "Esc", "d"}

	if r.deferKeyBuffer == "" {
		if key == tokens[0] {
			r.deferKeyBuffer = key
		}
		return "none", nil
	}

	// Determine what token we expect next.
	parts := splitTokens(r.deferKeyBuffer)
	expected := tokens[len(parts)]

	if key == expected {
		r.deferKeyBuffer += " " + key
		if len(splitTokens(r.deferKeyBuffer)) == len(tokens) {
			// Full sequence matched.
			r.deferKeyBuffer = ""
			r.ForceDefer()
			return "deferred", nil
		}
		return "none", nil
	}

	// Sequence broken — reset.
	r.deferKeyBuffer = ""
	if key == tokens[0] {
		r.deferKeyBuffer = key
	}
	return "none", nil
}

// splitTokens splits a space-separated token buffer into individual tokens.
func splitTokens(buf string) []string {
	if buf == "" {
		return nil
	}
	parts := make([]string, 0)
	current := ""
	for _, ch := range buf {
		if ch == ' ' {
			if current != "" {
				parts = append(parts, current)
				current = ""
			}
		} else {
			current += string(ch)
		}
	}
	if current != "" {
		parts = append(parts, current)
	}
	return parts
}
