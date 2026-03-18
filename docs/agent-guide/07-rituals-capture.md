# Phase 4B: Rituals + Capture

*Agent brief. Read this, then read the referenced docs.*

## Your Mission

Build the ritual runner (the step sequencer that drives structured interactions like morning planning) and the capture bar (quick node creation from anywhere in the TUI).

## Required Reading

- [Ritual format](../formats/ritual.md)
- [Design language](../aesthetics/design-language.md) (capture bar section, keyboard model)
- [ADR-009: Ritual definitions](../adr/adr-009-ritual-definitions.md)
- [ADR-012: First-class writing](../adr/adr-012-first-class-writing.md)

## Deliverables

### 1. Ritual Loader

Read ritual JSONC files from `/store/rituals/`. Parse the schedule, friction level, and step array. Determine which rituals should trigger based on the current day and time.

### 2. Ritual Runner

A state machine that advances through the step array. The runner takes over the left pane of the TUI when a ritual is active. It renders the current step and handles transitions.

**Step rendering by type:**

- `query_summary`: execute the Cypher query via `QueryRunner`, interpolate the result into the template string using `{{field}}` syntax, display the rendered text.
- `query_list`: execute the query, render results as a scrollable list. Support inline editing: the user can change a node's status, update properties, or create edges without leaving the ritual.
- `view`: render a named saved view (delegate to the view renderer from Phase 4A).
- `action`: trigger a built-in system action. Valid actions for v1: `propose_schedule`, `open_triage`, `open_journal`, `sync`, `rebuild_index`. These actions are hooks; some (like `propose_schedule`) will need stub implementations initially.
- `gate`: execute the condition query, check the named boolean field. If true, execute the `then` sub-step. If false, skip silently (no visual indication of skipping).
- `prompt`: display the text, show a multi-line input area. On submit, create a node with the specified types and automatic edges.

**Friction enforcement:**

- `gate` friction: when the ritual triggers, the TUI enters ritual mode. The user can advance through steps with Enter/Space, but cannot leave until all steps are complete or they explicitly defer (a dedicated keybinding, e.g. `Esc` followed by a confirmation).
- `nudge` friction: the ritual appears as a prominent banner that can be dismissed with a single keypress.

### 3. Ritual Scheduling

Check whether any ritual should trigger based on the current time and configured schedule. This runs once at startup and on a periodic tick (every minute). If a `gate`-friction ritual's scheduled time has passed but the ritual hasn't been completed or deferred today, it triggers immediately on next check.

Schedule format supports: specific days with time (`{"days": ["mon", "wed"], "time": "09:00"}`), daily (`{"days": [...all days...], "time": "21:00"}`), and weekly (`{"day": "sun", "time": "09:00"}`).

### 4. Capture Bar

A persistent text input at the bottom of the TUI (in the status bar area). A single keystroke (configurable, default `i`) focuses the capture bar from anywhere. The user types a quick thought, presses Enter, and a node is created. Focus returns to whatever was active before.

**Node creation from capture:**

- Default type: `task` with status `inbox`.
- The input text becomes the node's `body`.
- Optional prefix syntax for quick type selection: `j:` for journal, `n:` for note, `t:` for task (default). E.g. `j: Feeling good about the EPA progress` creates a journal node.
- If a node is selected in the right pane when capture is used, optionally create a `related` edge to that node (configurable behaviour).

### 5. Inline Editing in Ritual Lists

When a `query_list` step is active, the user can:

- Change a node's `status` by pressing a key (e.g. `s` cycles through statuses, or a popup offers the enum values).
- Quick-archive with `x` (sets status to "archived").
- Create an edge by pressing `e`, selecting a type, and picking a target node.

These edits go through the store layer and trigger an index update. The query list re-evaluates after each edit.

## Constraints

- Rituals must not block the main Bubble Tea event loop. Long-running queries execute asynchronously; the step shows a loading indicator while waiting.
- The capture bar must never lose typed text if the user accidentally presses a navigation key. Capture mode should intercept all input until Enter (submit) or Esc (cancel).
- Gate friction must be genuinely difficult to bypass. The defer action should require a deliberate keypress sequence (not just Esc), so the user can't accidentally skip a morning planning ritual.

## Testing Requirements

- Test ritual loading and schedule matching for each schedule format.
- Test the runner: step through a complete morning planning ritual with mock query results. Verify each step type renders and transitions correctly.
- Test gate friction: attempt to leave ritual mode without completing; verify the block.
- Test capture: create nodes via the capture bar with each prefix type. Verify the created node has the correct type and status.
- Test inline editing: change a node's status in a query_list step, verify the store is updated and the list re-renders.

## Output Structure

```
internal/tui/ritual/
  loader.go         # Ritual file loading + schedule checking
  runner.go         # Step sequencer state machine
  steps.go          # Step type implementations
  scheduler.go      # Periodic schedule checker
  loader_test.go
  runner_test.go
  steps_test.go
internal/tui/
  capture.go        # Capture bar
  capture_test.go
```
