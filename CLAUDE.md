# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

---

## Note on Go

Jason has never used Go before. Explain Go-specific constructs and idioms where relevant rather than assuming familiarity.

---

## Commands

```sh
make build       # build bin/wyrd and bin/wyrd-merge-driver
make test        # go test ./... -v
make vet         # go vet ./...
make lint        # golangci-lint run (requires: brew install golangci-lint)
make install     # install both binaries to $GOPATH/bin
make coverage    # generate coverage.html
```

Single test: `go test ./internal/store -v -run TestCreateAndReadNode`

Module: `github.com/jasonwarrenuk/wyrd` (Go 1.25)

---

## Architecture

Wyrd is a flat-file property graph. Everything is a JSONC file on disk. An in-memory index is built at startup and kept live via a filesystem watcher.

### Data model

- **Node** — a vertex (`internal/types/node.go`). Has `ID` (UUID v4), `Body` (markdown), `Types` (slice, minimum one), timestamps, and a flexible `Properties` map. Nodes are never deleted — archival sets `status: "archived"`.
- **Edge** — a directed relationship (`internal/types/edge.go`). Has `ID`, `Type` (e.g. `blocks`, `waiting_on`, `parent`), `From`/`To` node UUIDs, and optional `Properties`. Edges are truly deleted.
- **SpendEntry** — embeds inside a budget node's `Properties["spend_log"]`.

### Interface seams

All packages talk to each other through interfaces defined in `internal/types/`:

- `StoreFS` — file I/O facade. The `Store` struct satisfies this. CLI functions accept `StoreFS` so they can be tested without touching the real filesystem.
- `GraphIndex` — in-memory graph queries (`GetNode`, `EdgesFrom`, `NodesByType`, etc.). The `memIndex` satisfies this.
- `QueryRunner` — `Run(query, clock) → *QueryResult`. Satisfied by `query.Engine`.
- `Clock` — `Now() time.Time`. `RealClock{}` for production; `StubClock` with a fixed time in tests.
- `PluginStore` — embeds `StoreFS` with additional plugin manifest methods.

### Store (`internal/store/`)

`store.New(path, clock)` creates the directory structure, loads all JSONC files into the `memIndex`, and starts an `fsnotify` watcher. `WriteNode`/`WriteEdge` write atomically (temp file + rename). `store.Store` satisfies both `StoreFS` and `PluginStore`.

Store layout on disk:
```
~/wyrd/store/
  nodes/{uuid}.jsonc
  edges/{uuid}.jsonc
  templates/{type}.jsonc
  views/{name}.jsonc
  rituals/{name}.jsonc
  themes/{name}.jsonc
  plugins/{name}/manifest.jsonc
  config.jsonc
```

### Query engine (`internal/query/`)

**The query language is an implementation of the Cypher graph query language** (as used by Neo4j), restricted to a read-only subset. The goal is Cypher compatibility: new syntax additions should follow the Cypher spec, not invent new conventions.

`query.NewEngine(index, maxDepth)` returns an `Engine`. `Engine.Run(query, clock)` parses with [Participle](https://github.com/alecthomas/participle) (LL(1) parser combinator), then evaluates against the `GraphIndex`. Mutations are rejected. Returns `*QueryResult{Columns []string, Rows []map[string]any}`.

Supported built-in date variables: `$today`, `$now`, `$week_start`, `$month_start` — with optional arithmetic offsets (e.g. `$today + 7d`). Not yet implemented but planned: `UNION` (tracked in roadmap).

### CLI layer (`internal/cli/` + `cmd/wyrd/main.go`)

`cmd/wyrd/main.go` builds the Cobra command tree and wires everything together. Each subcommand opens the store via `openStore()`, then calls a function in `internal/cli/` (e.g. `cli.RunQuery`, `cli.Spend`). The CLI functions accept interfaces (`StoreFS`, `GraphIndex`, `QueryRunner`) rather than concrete types — this is what makes them testable.

`cli.Sync` is a thin wrapper: it calls `wyrdSync.Sync(storePath)` from `internal/sync/`.

### Sync (`internal/sync/`)

Uses the system `git` binary via `exec.Command` (not a Go git library). `sync.Sync(storePath)` stages all changes, generates a descriptive commit message from the git index (e.g. "wyrd: created 2 nodes, updated 1 edge"), pulls with rebase, and pushes. Network errors are pattern-matched from git stderr and returned with actionable messages. A custom JSONC three-way merge driver (`cmd/wyrd-merge-driver`) is registered in `.git/config`.

### TUI (`internal/tui/`)

Bubble Tea + Lipgloss. Wired as the default `wyrd` command (no args launches the TUI). Theme system uses Cairn palette by default. The split-pane layout, keyboard model, command palette, and status bar are all in place; content views implement the `PaneModel` interface.

#### TUI styling rules

Two utilities in `internal/tui/render.go` prevent background-bleed bugs. **Always use them — never use bare strings.**

- **`PadLines(content, width, bg)`** — pads every line to full pane width with `bg`. Call in every `PaneModel.View()` before returning.
- **`Spacer(n, bg)`** — returns `n` spaces with `bg`. Use instead of `" "` or `strings.Repeat(" ", n)` when joining `Render()` output.

Rules (background bleed has been reintroduced 5+ times — these are non-negotiable):
1. Every `lipgloss.NewStyle()` in a themed container must carry both `.Background(bg)` **and** `.Foreground(fg)`.
2. Never concatenate bare string literals between `Render()` calls — use `Spacer(n, bg)`.
3. Never use `strings.Repeat(" ", n)` for display spacers — use `Spacer(n, bg)`.
4. Every `PaneModel.View()` implementation must pass its output through `PadLines()` before returning.

---

## Roadmap files (`docs/roadmaps/`)

**Always analyse the full dependency chain before editing a roadmap file.** Each task carries explicit `depends on` annotations and the Mermaid progress map encodes the same graph. Adding, removing, or re-scoping a task can unblock or block other tasks downstream.

Checklist when editing a roadmap:
1. Identify every task that lists the changed task as a dependency.
2. Update their `depends on` annotations if the dependency is renamed or split.
3. Update the Mermaid graph nodes and edges to match.
4. Re-evaluate blocked/open status for all affected tasks.

---

## Testing patterns

Tests use `t.TempDir()` for ephemeral stores and `types.StubClock` (or a local `fixedClock` struct) for deterministic time. The pattern appears throughout:

```go
func newTestStore(t *testing.T) *store.Store {
    t.Helper()
    s, err := store.New(t.TempDir(), types.RealClock{})
    // ...
}
```

Test files live alongside source. No fixtures directory — test data is constructed inline or via helper functions. Individual test functions (not table-driven) are the norm.

---

## Error types

All in `internal/types/errors.go`. Use these rather than `fmt.Errorf` bare strings when the error has a semantic type:

| Type | When to use |
|------|-------------|
| `ValidationError{Field, Message}` | Bad user input |
| `NotFoundError{Kind, ID}` | Entity missing |
| `ParseError{Source, Message}` | JSONC or query parse failure |
| `ConflictError{Message}` | Data integrity violation |
| `QueryError{Query, Message}` | Query execution failure |

---

## Key dependencies

| Package | Purpose |
|---------|---------|
| `github.com/alecthomas/participle/v2` | Cypher parser |
| `github.com/charmbracelet/bubbletea` | TUI framework |
| `github.com/charmbracelet/lipgloss` | TUI styling |
| `github.com/fsnotify/fsnotify` | Filesystem watcher |
| `github.com/spf13/cobra` | CLI command framework |
| `github.com/google/uuid` | UUID v4 generation |
