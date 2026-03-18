# Phase 4C: Budget + Detail Pane

*Agent brief. Read this, then read the referenced docs.*

## Your Mission

Build the budget interaction layer (spending commands, budget status computation) and the right-pane node detail view with edge display and metadata rendering.

## Required Reading

- [Budget envelope format](../formats/budget-envelope.md)
- [Design language](../aesthetics/design-language.md) (budget status section, edge display section)
- [ADR-008: Envelope budgeting](../adr/adr-008-envelope-budgeting.md)
- [Node schema](../schema/node.md) (conditional fields, synced nodes)
- [Edge schema](../schema/edge.md) (built-in types, display conventions)

## Deliverables

### 1. Budget Status Computation

A derived property computed at index time (or query time) from the node's `spend_log`, `allocated`, and `warn_at` fields. The computation:

1. Filter `spend_log` entries to the current period (week/month/quarter/year, based on the envelope's `period` field).
2. Sum the `amount` values.
3. Compare to `allocated` and `warn_at`:
   - spend < (allocated × warn_at) → OK
   - spend ≥ (allocated × warn_at) → Caution
   - spend ≥ allocated → Over

Expose this as a computed field (e.g. `budget_status`, `budget_spent`, `budget_remaining`) accessible to the query engine and the TUI.

### 2. Spend Command Handler

The CLI command `wyrd spend {category} {amount} "{note}"` resolves to a store operation:

1. Find the budget node matching the `category` field.
2. Append `{"date": "today", "amount": N, "note": "..."}` to the `spend_log` array.
3. Update `modified`.

Handle errors: category not found, invalid amount, duplicate entries for the same date+note (warn but allow).

### 3. Node Detail Pane

The right pane of the TUI. Shows full detail for whatever node is selected in the left pane. Layout:

**Title:** node body (first line or first N characters), styled with `accent.primary`.

**Body:** full markdown content of the node.

**Metadata:** template fields rendered as key-value pairs below the body. Use `fg.muted` for keys, `fg.primary` for values. Skip fields that are null/empty unless the user has toggled "show all fields."

**Edges section:** header `EDGES`, then a list of connected edges with directional arrows and type-specific rendering:

```
← blocks: Portfolio site draft
→ parent: EPA preparation
⊘ waiting_on: Dan (feedback) · 12d
◇ related: Cypher syntax notes
```

The `waiting_on` edge shows the promised person (from the target node) and the age computed from `edge.created`. Age uses progressively warmer colours as it increases (thresholds configurable in config.jsonc).

**Budget section** (only if budget nodes exist): header `BUDGETS`, then a compact summary of each budget envelope with its progress bar and glyph.

### 4. Edge Age Rendering

For `waiting_on` edges, compute the age in days from `edge.created` to now. Render as a suffix: `· 3d`, `· 12d`, `· 45d`. Colour the age suffix using a gradient from the theme:

- 0-7 days: `fg.muted` (normal)
- 8-14 days: `overflow.warn` (getting old)
- 15+ days: `overflow.critical` (stale)

These thresholds should be configurable.

### 5. Synced Node Display

Nodes with a `source` property show a sync indicator in the detail pane: the source type, repo/service name, and a link to the original URL. Style this in `fg.muted` to keep it secondary to the node's own content.

### 6. Archived Node Handling

If the user navigates to an archived node (e.g. via a query that explicitly includes archived items), the detail pane should show a clear visual indicator that the node is archived. Use the `fg.muted` colour for the entire pane content, with an `ARCHIVED` label.

## Constraints

- The detail pane must update instantly when the selection changes in the left pane. No loading states for local data; the index is in memory.
- Edge display must handle nodes with many edges (20+) gracefully. Show a scrollable list, not a truncated one.
- Budget progress bars must render correctly at various terminal widths. Calculate bar length from available width minus the label and amount text.
- Spend log entries are ordered by date descending. The most recent entries appear first.

## Testing Requirements

- Budget status: test all three thresholds with known spend amounts and allocated values.
- Spend command: test appending to the log, verify `modified` is bumped, verify period filtering.
- Detail pane: render a node with edges of each built-in type. Verify directional arrows and glyphs.
- Edge age: mock the clock, test each age threshold colour.
- Archived node: verify the visual indicator and muted styling.

## Output Structure

```
internal/tui/
  detail.go         # Node detail pane
  detail_test.go
internal/
  budget/
    compute.go      # Budget status computation
    spend.go        # Spend command handler
    compute_test.go
    spend_test.go
```
