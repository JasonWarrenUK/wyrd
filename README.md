```ansi
[2m┌──────────────────────────────────────────────────────────────────────────┐[0m
[2m│[0m                                                                          [2m│[0m
[2m│[0m   [38;2;120;120;113m08:00[0m [38;2;217;119;6m▓▓▓▓▓▓▓▓[0m [38;2;231;229;228mDocs update[0m                [38;2;120;120;113m# Client Project[0m             [2m│[0m
[2m│[0m   [38;2;120;120;113m09:30[0m [38;2;217;119;6m████████[0m [38;2;231;229;228mRhea sprint[0m                                             [2m│[0m
[2m│[0m   [38;2;120;120;113m10:00[0m [38;2;120;120;113m┄┄┄┄┄┄┄┄[0m [38;2;231;229;228mStand-up[0m                   [38;2;120;120;113mBody content here...[0m         [2m│[0m
[2m│[0m   [38;2;120;120;113m10:45[0m [38;2;217;119;6m▓▓▓▓▓▓[0m   [38;2;231;229;228mTWCB deep work[0m                                          [2m│[0m
[2m│[0m   [38;2;120;120;113m11:45[0m [38;2;120;120;113m░░[0m       [38;2;231;229;228mTriage[0m                     [38;2;231;229;228mstatus[0m  [38;2;217;119;6min_progress[0m          [2m│[0m
[2m│[0m                                             [38;2;231;229;228menergy[0m  [38;2;146;64;14m▓▓[0m [38;2;217;119;6mdeep[0m             [2m│[0m
[2m│[0m   [38;2;217;119;6m◆[0m [38;2;231;229;228m47m unscheduled · 1 task displaced[0m      [38;2;231;229;228mdue[0m     [38;2;120;120;113m2026-04-01[0m           [2m│[0m
[2m│[0m                                                                          [2m│[0m
[2m│[0m                                             [38;2;146;64;14m||==>> EDGES[0m                 [2m│[0m
[2m│[0m                                             [38;2;220;38;38m←[0m [38;2;231;229;228mblocks:[0m [38;2;120;120;113mPortfolio draft[0m    [2m│[0m
[2m│[0m                                             [38;2;120;120;113m⊘ waiting: Dan · 12d[0m         [2m│[0m
[2m│[0m                                                                          [2m│[0m
[2m│[0m                                             [38;2;146;64;14m||==>> BUDGETS[0m               [2m│[0m
[2m│[0m                                             [38;2;101;163;13m●[0m [38;2;231;229;228mEPA[0m     [38;2;101;163;13m9.5/12h[0m            [2m│[0m
[2m│[0m                                             [38;2;217;119;6m◐[0m [38;2;231;229;228mRecords[0m [38;2;217;119;6m£42/£50[0m            [2m│[0m
[2m│[0m                                             [38;2;220;38;38m○[0m [38;2;231;229;228mBaby[0m    [38;2;220;38;38m£580/£500[0m          [2m│[0m
[2m│[0m                                                                          [2m│[0m
[2m├──────────────────────────────────────────────────────────────────────────┤[0m
[2m│[0m [38;2;120;120;113m[capture bar][0m                                 [38;2;217;119;6m◆ 47m on Project (45m)[0m     [2m│[0m
[2m└──────────────────────────────────────────────────────────────────────────┘[0m
```

# wyrd

A terminal-based personal productivity system backed by a flat-file property graph. Tasks, notes, commitments, habits, and budgets: all connected, all in one place, all running on your machine.

---

## What it does

Most productivity tools address one thing well and ignore the rest. Wyrd's premise is that tasks, notes, journal entries, commitments, and budget envelopes are all nodes in the same graph, connected to each other by typed edges. A task can block another task. A journal entry can relate to a commitment. A budget envelope tracks the actual spend against allocation. The relationships are first-class, not bolted on.

Three principles govern the design:

**It shows consequences, never nags.** Spend too long on a task and the rest of your day visibly compresses; tasks that no longer fit are greyed out and displaced to tomorrow. Overspend a budget envelope and the progress bar turns hollow. The system makes costs visible; you make the decisions.

**It adapts to your shape.** Task types, daily rituals, saved views, and themes are all defined by you in plain JSONC files. The structure is yours.

**It works offline.** No accounts. No cloud dependency. Every bit of data lives under `~/wyrd/store` as files you can read with any text editor.

---

## Installation

```sh
go install github.com/jasonwarrenuk/wyrd/cmd/wyrd@latest
```

On first run, `wyrd` initialises `~/wyrd/store` with starter templates, a default theme, and a git repository for optional sync.

---

## Usage

```
wyrd                          launch TUI
wyrd add "body text"          quick-add a task node
wyrd add --type note "title"  quick-add with type override
wyrd journal                  open $EDITOR, create journal node
wyrd note "title"             open $EDITOR, create note node
wyrd spend {cat} {amt} "note" log a spend entry
wyrd sync                     git sync cycle (stage, commit, pull, push)
wyrd query "MATCH ..."        run a Cypher query, print results
wyrd view {name}              run a saved view, print results
wyrd plugin install {path}    install a plugin
wyrd plugin list              list installed plugins
```

Inside the TUI, navigation follows vim conventions: `hjkl` for movement, `gg`/`G` for top/bottom, `/` for search. `:` opens the command palette; `Ctrl+K` opens it in fuzzy mode. `i` focuses the capture bar from anywhere. `Ctrl+W` switches panes; `q` quits.

---

## Data model

Two entity types: **nodes** and **edges**, each stored as individual `.jsonc` files.

```
~/wyrd/store/
  nodes/{uuid}.jsonc
  edges/{uuid}.jsonc
  templates/{type_name}.jsonc
  themes/{name}.jsonc
  rituals/{name}.jsonc
  views/{name}.jsonc
  plugins/{name}/plugin.jsonc
  config.jsonc
```

Nodes carry a `types` array (`["task", "commitment"]`) and conditional fields contributed by template definitions. Edges are first-class entities with their own UUIDs, types, and properties. Nodes are never deleted; archiving sets `status: "archived"`, preserving the full audit trail in git history.

On startup, all files are loaded into an in-memory graph index with O(1) edge lookups per node. A filesystem watcher incrementally updates the index on file change.

---

## Query language

Views and rituals run queries in a read-only Cypher subset against the in-memory index.

```cypher
-- Tasks blocked by someone you're waiting on
MATCH (t:task)<-[:blocks]-(b:task)-[:waiting_on]->(p)
WHERE t.status = 'ready'
RETURN t.body, p.body AS waiting_on
ORDER BY t.energy DESC

-- Inbox items created this week
MATCH (n:task)
WHERE n.status = 'inbox' AND n.created >= $week_start
RETURN n.body, n.created
LIMIT 20

-- Budget spend by category this month
MATCH (b:budget)
RETURN b.category, b.budget_spent, b.allocated
ORDER BY b.budget_spent / b.allocated DESC
```

Supported: `MATCH`, `WHERE`, `RETURN`, `ORDER BY`, `LIMIT`, `AS`; property filters; boolean logic; comparison operators; edge traversal in both directions; variable-length paths up to a configurable ceiling (default 5 hops); aggregation via `count()`, `sum()`, `avg()`, `min()`, `max()`, `collect()`. Built-in variables: `$today`, `$now`, `$week_start`, `$month_start`, with `+7d`/`-30d` offset arithmetic.

---

## Schedule and time displacement

The schedule view renders today's time blocks as a column of energy-coloured entries:

```
08:00 ▓▓▓▓▓▓▓▓ EPA AM2 revision      deep work
09:30 ████████ Rhea sprint            medium
10:00 ┄┄┄┄┄┄┄┄ Stand-up             [calendar]
10:45 ▓▓▓▓▓▓   TWCB deep work        deep work
11:45 ░░       Triage
```

Energy levels use paired colour and fill-character signals so they read correctly under colour blindness and 16-colour terminals:

| Level | Fill | Colour (Cairn) |
|-------|------|----------------|
| Deep | `▓▓▓▓▓▓▓▓` | violet `#794aff` |
| Medium | `████████` | amber `#b98300` |
| Low | `░░░░░░░░` | grey `#6f6f6f` |

When the active task overruns its allocation, subsequent tasks compress proportionally. The status bar shows the overrun: `◆ 47m on EPA (45m planned)`. Tasks that no longer fit are greyed and displaced, with a summary beneath the schedule: `◆ 2h15m unscheduled · 3 tasks displaced to tomorrow`. No alert, no interruption; the cost is just visible.

Calendar events (from sync plugins) render with dashed lines and cannot be moved: `┄┄┄┄┄┄┄┄`.

---

## Budget envelopes

Budget nodes track spending against a period allocation. Three states:

```
● EPA time   ████████████████░░░░░  9.5/12h     OK: within allocation
◐ Records    ████████████████████░  £42/£50     Caution: approaching limit
○ Baby prep  ██████████████████████ £580/£500   Over: allocation exceeded
```

Log a spend from the command line:

```sh
wyrd spend records 18.50 "Discogs"
wyrd spend baby 45.00 "Pram service"
```

Period filtering (week/month/quarter/year) is per-envelope.

---

## Rituals

Rituals are structured interaction sequences: a morning planning session, a weekly review, an end-of-day sweep. They are defined in JSONC files and run as step sequences in the TUI's left pane.

Step types: `query_summary` (run a query, interpolate results into text), `query_list` (scrollable list with inline editing), `view` (render a saved view), `prompt` (capture input and create a node), `gate` (conditional step based on a query result), and `action` (trigger a built-in system action).

Gate-friction rituals hold the TUI until all steps are complete or explicitly deferred (a deliberate `Esc Esc d` sequence). Nudge-friction rituals show a dismissable banner.

---

## Sync

`wyrd sync` runs a full git cycle against the store directory: stage all changes, commit with an auto-generated message (`wyrd: created 2 nodes, updated 1 edge`), pull with rebase, push. A custom three-way merge driver registered in `.gitattributes` handles JSONC merges at the property level: non-conflicting changes merge cleanly; conflicting scalar values resolve by last-write-wins (by `modified` timestamp); arrays merge by union.

This requires a remote configured in the store's git repository. Sync errors produce actionable messages; raw git stderr is never surfaced directly.

---

## Plugins

Plugins communicate over stdin/stdout via a JSON-lines protocol. A manifest declares capabilities (`sync`, `action`, or both). Sync plugins stream `upsert_node` and `upsert_edge` operations; action plugins execute on the selected node.

```json
{"op": "sync", "config": {...}}
{"op": "upsert_node", "node": {"id": "...", "body": "Fix auth bug", "types": ["task"]}}
{"op": "done", "summary": {"created": 3, "updated": 1, "errors": 0}}
```

Plugins can be written in any language. Synced nodes receive deterministic UUIDs derived from the source ID (UUID v5), so re-running sync updates rather than duplicates. A built-in `wyrd-plugin-shell` executor handles simple command-template actions without custom code: `"command": "open {{source.url}}"`.

Install from a directory or `.wyrdplugin` archive:

```sh
wyrd plugin install ./my-plugin
wyrd plugin install ./calendar-sync.wyrdplugin
```

---

## Themes

Four themes ship with the app, all built from [Reasonable Colors](https://www.reasonable.work/colors/) (LCH colour space, WCAG AA contrast guarantees). Each degrades gracefully to 16 colours using glyphs as the primary differentiating signal.

| Theme | Character |
|-------|-----------|
| **Cairn** | Dark; violet structure with amber warmth. Stacked stone at dusk. |
| **Peat** | Dark; boggy greens and teal. Earthy, damp, quiet. |
| **Kiln** | Light; terracotta. Orange and cinnamon earth tones. |
| **Fell** | Light; cartographic. Cream paper, contour-line greens. |

Switch at runtime via the command palette: `:theme fell`.

---

## Architecture

```
internal/
  types/      core types and interfaces
  store/      flat-file graph store; in-memory index; fsnotify watcher
  query/      Cypher subset parser and evaluator
  sync/       git operations; three-way JSONC merge driver
  plugin/     JSON-lines protocol; process lifecycle; shell executor
  budget/     envelope computation and spend handler
  tui/        Bubble Tea application; theme engine; keyboard model
  tui/views/  list, timeline, prose, budget, and schedule renderers
  tui/ritual/ ritual loader, scheduler, and step-sequencer state machine
  cli/        CLI commands and store initialisation
  embed/      embedded starter assets (templates, themes, views)
cmd/
  wyrd/             main entrypoint
  wyrd-merge-driver git merge driver binary
```

Significant architectural decisions are recorded as ADRs in `docs/adr/`.

---

## The name

Wyrd is an Anglo-Saxon word meaning fate; more precisely, "that which has become." Your present situation is shaped by your past choices. The app makes that visible.

---

## Licence

MIT
