# Wyrd: Practical Guide

*What it is, what it does, and how the pieces fit together.*

## The Short Version

Wyrd is a terminal app for managing everything personal: tasks, notes, journal entries, commitments to other people, habits, and budgets. All of these live as nodes in a directed property graph, stored as individual JSONC files on disk and synced via git. A Cypher subset query engine powers every view, ritual, and report. Four themes ship out of the box. Plugins extend it in any language.

## Data Model

Two entity types exist: **nodes** and **edges**. Every node is a single `.jsonc` file under `/store/nodes/`. Every edge is a single `.jsonc` file under `/store/edges/`. No directory hierarchy; flat namespaces for both.

A node carries a `types` array (e.g. `["task", "commitment"]`) that determines which fields it has. Adding a type merges in that template's fields with sensible defaults. Removing a type hides those fields from the default view but never deletes them from the file. Multiple types on one node are fine; the first type in the array wins for default values when field names collide across templates.

Five base fields appear on every node: `id` (UUID v4, immutable), `body` (markdown content), `types`, `created`, and `modified`.

Edges connect two nodes with a typed, directed relationship. Five built-in types ship by default: `blocks` (A prevents B from starting), `parent` (A contains B), `related` (loose association), `waiting_on` (A is waiting for B, with optional promised/follow-up dates), and `precedes` (soft ordering preference). Users can define additional types freely.

## File Layout

```
/store
  /nodes/{uuid}.jsonc        # one file per node
  /edges/{uuid}.jsonc         # one file per edge
  /templates/{type_name}.jsonc  # field definitions per node type
  /themes/{name}.jsonc        # colour tiers, glyphs
  /rituals/{name}.jsonc       # structured recurring interactions
  /views/{name}.jsonc         # saved Cypher queries with display metadata
  /plugins/{name}/plugin.jsonc  # plugin manifests and config
  /config.jsonc               # global settings
```

## Templates

Templates define what fields a node type contributes. Three ship with Wyrd:

**task:** `status` (inbox/ready/in_progress/blocked/done/dropped), `energy` (low/medium/deep), `contexts` (string array), `people` (string array), `due` (optional date).

**journal:** `mood` (optional enum), `tags` (string array).

**note:** `tags` (string array), `source_url` (optional string).

Supported field types: `string`, `number`, `date`, `datetime`, `boolean`, `enum`, `string_array`, `object`. Users create additional templates as JSONC files; the system picks them up automatically.

## Query Engine

A read-only Cypher subset handles all data access. Pattern matching, property filters, single-hop and variable-length edge traversal (capped at depth 5 by default), date arithmetic, and aggregation (`count`, `sum`, `avg`, `min`, `max`, `collect`). Mutations go through Wyrd commands, never through queries.

Date variables (`$today`, `$now`, `$week_start`, `$month_start`) with offset arithmetic (`+Nd`, `-Nd`) make temporal queries concise:

```cypher
MATCH (p:project) WHERE p.modified < $today - 7d AND p.status <> 'paused'
RETURN p ORDER BY p.modified ASC
```

## Saved Views

A saved view is a named Cypher query with presentation metadata. Four display modes exist:

**list:** tabular with configurable columns. Default for task queries.
**timeline:** reverse-chronological full text. Default for journal queries.
**prose:** single-node detail with body and edge list, wiki-style. Default for note queries.
**budget:** envelope progress bars with status glyphs.

Views can be pinned to the TUI sidebar. Several starter views ship with Wyrd (blocked tasks, waiting on others, journal, recent notes) and all are user-editable.

## Rituals

Rituals are structured recurring interactions: morning planning, weekly review, evening reflection. Each is a JSONC file containing an ordered array of typed steps.

Six step types: `query_summary` (scalar results rendered into a template string), `query_list` (scrollable list with inline editing), `view` (renders a saved view), `action` (triggers a built-in system action), `gate` (conditional; runs a Cypher query, executes a sub-step if true), and `prompt` (free-text input that creates a node).

Two friction levels control how insistently the ritual demands attention: `gate` (must complete or explicitly defer) and `nudge` (prominent suggestion, silently bypassable).

## Budget Envelopes

Budgets are ordinary nodes with a `budget` type. Each envelope tracks a category, period, allocated amount, and a `spend_log` array. Spending is logged via `wyrd spend records 18.50 "Discogs"`. No accounts, no bank import, no transaction nodes; just manual envelope tracking.

Three status indicators, each paired with a glyph for colour-blind accessibility: OK (● full circle), Caution (◐ half circle, triggered at a configurable threshold), Over (○ empty circle).

## The TUI

Split-pane, keyboard-driven. Left pane shows the active view (schedule, saved view, ritual step). Right pane shows detail for the selected node. A status bar spans the bottom with a capture bar for quick thoughts and a time tracker showing actual vs planned duration.

Navigation follows vim conventions: `hjkl` movement, `gg`/`G` for top/bottom, `/` for search, `Ctrl+W` for pane switching. `:` opens the full command palette; `Ctrl+K` opens fuzzy search across all commands.

### Time Displacement

The centrepiece interaction. When you spend longer than planned on a task, remaining tasks visibly compress. Tasks that no longer fit get greyed out with an overflow indicator. A summary line shows the cost: `◆ 2h15m unscheduled · 3 tasks displaced to tomorrow`. No interruption, no nagging; just visible consequences.

### Energy Levels

Three levels (deep, medium, low) use paired colour and fill-character signalling. The fill character varies independently of colour (`▓` dense for deep, `█` solid for medium, `░` light for low), so the signal works under colour blindness or 16-colour terminals.

## Sync

The `/store` directory is a git repository. Sync is explicit: `wyrd sync`. A custom three-way merge driver handles JSONC merges at the property level. Non-conflicting changes merge cleanly. Conflicting scalar changes resolve by last-write-wins (using `modified` timestamps). Arrays merge by union. Nodes are never deleted from disk (ADR-006); archiving sets `status: "archived"`, which eliminates delete/modify conflicts entirely.

## Plugins

Plugins communicate via JSON-lines over stdin/stdout. A plugin declares capabilities in its manifest: `sync` (pulls data from external sources) and/or `action` (executes on a selected node). Any language works; the protocol is just newline-delimited JSON.

A built-in `wyrd-plugin-shell` executor handles simple command-template actions (e.g. `open {{source.url}}`) without writing any code. Plugin manifests and configs sync via git; binaries are installed per-machine.

Planned plugins include GitHub issue sync, Google Calendar integration, Obsidian asymmetric sync (journals push out; notes pull in both directions), and a Claude Code launcher with graph context.

## Obsidian Integration

Asymmetric sync, direction determined by content type. Journals push from Wyrd to Obsidian automatically (Wyrd is the writing surface; Obsidian is the archive). Notes flow manually in both directions via `wyrd push` and `wyrd pull obsidian`. Wikilinks become candidate `related` edges. The integration is a standard plugin with both `sync` and `action` capabilities.

## Writing in Wyrd

Journals and notes are first-class alongside tasks. `wyrd journal` opens `$EDITOR` with today's date pre-filled. `wyrd note "title"` opens `$EDITOR` for the body. Both support `--link <node>` for creating edges during capture. The TUI provides dedicated viewing modes for each content type: list columns for tasks, reverse-chronological prose for journals, wiki-style detail for notes.

## Themes

Four themes ship with Wyrd, all built from Reasonable Colors (LCH colour space, MIT licence). Shade difference of 3 guarantees 4.5:1 contrast (WCAG AA); difference of 4 guarantees 7:1 (WCAG AAA).

**Ember & Violet:** dark, warm. Near-black background, violet accents, cinnamon/amber warmth.
**Peat & Teal:** dark, cool. Near-black with teal secondary, earthy and quiet.
**Kiln:** light, warm. Terracotta backgrounds, dark earth text, red accents.
**Ordnance Survey:** light, cartographic. Cream paper, contour-line greens, footpath pinks.

Each theme defines three colour tiers (truecolor, 256-colour, 16-colour) so the TUI degrades gracefully across terminals. Themes are user-editable JSONC files; creating additional themes is straightforward.

## Stack Summary

| Layer | Technology |
|-------|-----------|
| TUI | Go + Bubble Tea (Elm architecture) |
| Data | JSONC, one file per entity |
| Query | Cypher subset (participle parser) |
| Sync | Git |
| Plugins | Any language (JSON-lines protocol) |
| LLM | Ollama or remote API (optional, via plugin layer) |
