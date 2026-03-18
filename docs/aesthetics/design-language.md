# Design Language

Visual and interaction conventions used across Wyrd's TUI.

## Layout

Split-pane, keyboard-driven. Default configuration:

```
┌──────────────────────────┬─────────────────────────┐
│                          │                         │
│   Schedule / View        │   Node Detail / Editor  │
│                          │                         │
│   08:00 ▓▓▓▓▓▓▓▓ EPA    │   # EPA AM2 Revision    │
│   09:30 ████████ Rhea    │                         │
│   10:00 ┄┄┄┄┄┄ Standup  │   Body content here...  │
│   10:45 ▓▓▓▓▓▓ TWCB     │                         │
│   11:45 ░░░░░░ Triage    │   status  in_progress   │
│                          │   energy  deep           │
│   ◆ 2h unscheduled      │   due     2026-04-01    │
│                          │                         │
│                          │   EDGES                 │
│                          │   ← blocks: Portfolio   │
│                          │   ⊘ waiting: Dan · 12d  │
│                          │                         │
│                          │   BUDGETS               │
│                          │   ● EPA     9.5/12h     │
│                          │   ◐ Records £42/£50     │
│                          │   ○ Baby    £580/£500   │
│                          │                         │
├──────────────────────────┴─────────────────────────┤
│ [capture bar]                  ◆ 47m on EPA (45m)  │
└────────────────────────────────────────────────────┘
```

The left pane shows the active view (schedule, saved view, ritual step). The right pane shows detail for the selected node. The status bar spans the full width.

## Energy Indicators

Three energy levels use paired colour + fill character signals.

| Level | Fill | Colour (Cairn) | Terminal appearance |
|-------|------|------------------------|-------------------|
| Deep | `██████` | violet-4 (`#794aff`) | Solid, saturated |
| Medium | `▓▓▓▓▓▓` | amber-3 (`#b98300`) | Dense, warm |
| Low | `░░░░░░` | gray-4 (`#6f6f6f`) | Light, muted |

The fill character varies independently of colour, ensuring legibility under colour blindness or 16-colour terminals.

## Time Displacement

The centrepiece interaction. When elapsed time on the current task exceeds the planned allocation:

1. The status bar shows actual vs planned: `◆ 47m on EPA (45m planned)`
2. Remaining tasks in the schedule visibly compress (shorter fill bars)
3. Tasks that no longer fit are greyed or marked with the overflow indicator
4. A summary shows below the schedule: `◆ 2h15m unscheduled · 3 tasks displaced to tomorrow`

The user is never interrupted. The cost of continuing is visible; the decision remains theirs.

## Budget Status

Envelope progress uses three states with paired glyph + colour signals.

| Status | Glyph | Meaning | Visual metaphor |
|--------|-------|---------|-----------------|
| OK | ● | Well within allocation | Full circle: envelope is full |
| Caution | ◐ | Approaching limit (≥ warn_at threshold) | Half circle: running low |
| Over | ○ | Exceeded allocation | Empty circle: envelope spent |

Progress bars accompany the glyphs in budget views:

```
● EPA time   ████████████████░░░░░  9.5/12h
◐ Records    ████████████████████░  £42/£50
○ Baby prep  ██████████████████████ £580/£500
```

## Edge Display

Edges in the detail pane use directional arrows and type-specific glyphs.

```
← blocks: Portfolio site draft      // inbound blocks edge
→ parent: EPA preparation           // outbound parent edge
⊘ waiting_on: Dan (feedback) · 12d  // waiting_on with age
◇ related: Cypher syntax notes      // loose association
```

The `⊘` glyph and ageing suffix (`12d`) draw attention to stale commitments. Age is computed from the edge's `created` date. Ageing commitments use progressively warmer colours as they get older (configurable thresholds).

## Calendar Events

Calendar events from Google Calendar appear as immovable blocks in the schedule, visually distinct from task blocks:

```
10:15 ┄┄┄┄┄┄┄ Stand-up
```

Dashed lines (`┄`) and italic text (where terminal supports it) distinguish calendar events from tasks. Events cannot be moved, rescheduled, or edited from within Wyrd; they're read-only reference data.

## Keyboard Model

**Navigation:** vim motions. `hjkl` for movement, `gg`/`G` for top/bottom, `/` for search, `Ctrl+W` for pane switching.

**Commands:** dual-trigger command palette. `:` opens the full command palette; `Ctrl+K` opens fuzzy search over all commands. The palette supports tab completion and shows keybinding hints.

**Capture:** single keystroke focuses the capture bar. Type, press enter, focus returns to the previous view.

**Plugin actions:** keybindings declared in plugin manifests. Shown in the command palette when the selected node matches the action's `applicable_types`.

## Section Headers

In-TUI section headers use uppercase with letter-spacing, styled in the muted text colour:

```
TODAY'S SCHEDULE
EDGES
BUDGETS
```

This convention is consistent across all views and themes.

## Empty States

When a view returns no results, a message appears in muted text rather than a blank pane. Starter views include default empty messages. Rituals support a custom `empty_message` per `query_list` step.
