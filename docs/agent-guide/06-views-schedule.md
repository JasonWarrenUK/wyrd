# Phase 4A: Views + Schedule

*Agent brief. Read this, then read the referenced docs.*

## Your Mission

Build the saved view renderer (all four display modes) and the schedule view with time displacement. Every query result the user sees passes through one of these renderers.

## Required Reading

- [Saved view format](../formats/saved-view.md)
- [Design language](../aesthetics/design-language.md) (time displacement section, energy indicators, edge display)
- [Theme palettes](../aesthetics/theme-palettes.md)
- [Node schema](../schema/node.md)
- [Edge schema](../schema/edge.md)

## Deliverables

### 1. View Loader

Read saved view JSONC files from `/store/views/`. Parse the `query`, `display`, `columns`, and `pinned` fields. Populate the TUI sidebar with pinned views.

### 2. List Display Mode

Tabular rendering of query results. Configurable columns. One row per node. Columns auto-size to content width (up to a maximum, then truncate with ellipsis). Selected row is highlighted using the theme's `selection` colour.

### 3. Timeline Display Mode

Reverse-chronological full-text rendering. Each entry shows the node's `created` date as a header, followed by the `body` content rendered as markdown (or as close to markdown rendering as Lipgloss/Glamour allows). Used primarily for journal views.

### 4. Prose Display Mode

Single-node detail view, wiki-style. Shows the node's `body` as the main content, followed by metadata fields, followed by a list of connected edges with directional arrows and type glyphs (see design language: `←`, `→`, `⊘`, `◇`).

### 5. Budget Display Mode

Envelope progress bars with status glyphs. Each budget node renders as:

```
● Records    ████████████████░░░░  £42/£50
```

Status determined by spend ratio to allocated amount. Glyphs: `●` OK, `◐` Caution, `○` Over. Colours from the theme's `budget.ok`, `budget.caution`, `budget.over` roles.

### 6. Schedule View

The day's schedule, rendered as a column of time-blocked entries. Each entry shows the time, an energy-coloured fill bar, and the task name. Calendar events (from sync plugins) render with dashed lines (`┄`) to distinguish them from tasks.

```
08:00 ▓▓▓▓▓▓▓▓ EPA review
09:30 ████████ Rhea sprint
10:00 ┄┄┄┄┄┄ Stand-up
10:45 ▓▓▓▓▓▓ TWCB deep work
11:45 ░░░░░░ Triage
```

Energy levels use the paired colour + fill character system from the design language: `▓` deep, `█` medium, `░` low.

### 7. Time Displacement

The centrepiece. When elapsed time on the current task exceeds its allocation:

1. Status bar shows actual vs planned: `◆ 47m on EPA (45m planned)`
2. Remaining tasks compress (shorter fill bars proportional to remaining time)
3. Tasks that no longer fit display greyed out with the overflow indicator `◆`
4. Summary line: `◆ 2h15m unscheduled · 3 tasks displaced to tomorrow`

The schedule must recalculate displacement in real-time (on a tick, once per minute is sufficient).

## Constraints

- All data comes through the query engine. Views execute their Cypher query via the `QueryRunner` interface. Never read files directly.
- Display modes must be a pluggable interface so new modes can be added later.
- The schedule view needs a concept of "current task" (the one being actively timed). This state lives in the TUI app model and is set by user action (selecting a task and pressing a "start" key).
- Time displacement calculations must not modify the underlying node data. Displacement is a view concern, computed on the fly.

## Testing Requirements

- Each display mode renders correctly with the seed data fixture.
- Time displacement: mock the clock, advance time past a task's allocation, verify the schedule re-renders with compressed remaining tasks.
- Empty view: a query that returns no results shows the view's name and an empty-state message.
- Budget display: test all three status thresholds (OK, Caution, Over) with known spend amounts.

## Output Structure

```
internal/tui/views/
  loader.go         # View file loading + sidebar population
  list.go           # List display mode
  timeline.go       # Timeline display mode
  prose.go          # Prose display mode
  budget.go         # Budget display mode
  schedule.go       # Schedule view + time displacement
  displacement.go   # Time displacement calculation logic
  list_test.go
  schedule_test.go
  displacement_test.go
```
