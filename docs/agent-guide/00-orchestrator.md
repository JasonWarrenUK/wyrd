# Wyrd: Agent Orchestration Guide

*For an orchestrating agent managing a swarm of developer agents building Wyrd.*

## Purpose of This Document

You are an orchestrating agent. Your job is to decompose the Wyrd build into work units, assign them to developer agents, manage dependencies between those units, and verify that completed work integrates correctly. You do not write code yourself. You coordinate.

The architecture overview, dependency graph, interface contracts, and build phases are all here. Phase-specific briefs (linked below) give each developer agent exactly what it needs for its assigned work.

## System Overview

Wyrd is a terminal-based personal productivity tool. Its architecture has five major subsystems, each buildable by a separate agent with well-defined interfaces between them:

1. **Store Layer** (file I/O, JSONC parsing, schema validation, in-memory graph index)
2. **Query Engine** (Cypher subset parser, pattern matcher, evaluator)
3. **Sync Engine** (git operations, three-way JSONC merge driver)
4. **Plugin System** (JSON-lines protocol, process management, shell executor)
5. **TUI** (Bubble Tea app, views, ritual runner, schedule engine, capture bar)

## Technology Stack

| Concern | Choice | Notes |
|---------|--------|-------|
| Language | Go | Single binary, no runtime deps |
| TUI framework | Bubble Tea v2 (`charm.land/bubbletea/v2`) | Elm architecture (Model-Update-View) |
| TUI styling | Lipgloss v2 (`charm.land/lipgloss/v2`) | Colour tiers, box rendering |
| TUI components | Bubbles v2 (`charm.land/bubbles/v2`) | Inputs, lists, viewports, spinners |
| Data format | JSONC | JSON with comments; one file per entity |
| Schema validation | JSON Schema (via Go library) | Validate nodes against template defs |
| Parser | participle (Go library) | For the Cypher subset grammar |
| Sync | Git (shelled out or go-git) | `wyrd sync` wraps add/commit/pull/push |

## Dependency Graph Between Subsystems

```
Store Layer ──────────┐
                      ├──→ TUI
Query Engine ─────────┤
                      │
Sync Engine ──────────┘
Plugin System ────────────→ TUI (action triggers)
                      ┌────→ Store Layer (upserts from sync plugins)
Plugin System ────────┘
```

The Store Layer and Query Engine have no upstream dependencies; they can be built first and in parallel. The Sync Engine depends on the Store Layer (it needs to read/write JSONC files). The Plugin System depends on the Store Layer (sync plugins produce node/edge upserts). The TUI depends on everything else.

## Build Phases

Work is organised into five phases. Phases 1 and 2 can run in parallel. Each subsequent phase depends on its predecessors.

### Phase 1: Store Layer + Query Engine (parallel)

Two agents work simultaneously. No dependencies between them.

**Agent A: Store Layer** → [01-store-layer.md](./01-store-layer.md)
**Agent B: Query Engine** → [02-query-engine.md](./02-query-engine.md)

**Integration checkpoint:** both agents deliver packages with test suites. The orchestrator verifies that the query engine can operate on an in-memory index produced by the store layer. This is the critical integration point; if these two don't mesh, everything downstream breaks.

### Phase 2: Sync Engine + Plugin System (parallel, after Phase 1)

Both depend on the Store Layer from Phase 1. They don't depend on each other.

**Agent C: Sync Engine** → [03-sync-engine.md](./03-sync-engine.md)
**Agent D: Plugin System** → [04-plugin-system.md](./04-plugin-system.md)

**Integration checkpoint:** the sync engine can round-trip a store through git (commit, clone, merge, verify index integrity). The plugin system can receive JSON-lines from a mock plugin and apply upserts to the store.

### Phase 3: TUI Shell

One agent builds the TUI framework: the Bubble Tea application skeleton, pane layout, theme engine, keyboard model, and the status bar. This phase produces a working but content-sparse terminal app.

**Agent E: TUI Shell** → [05-tui-shell.md](./05-tui-shell.md)

**Integration checkpoint:** the TUI boots, renders the split-pane layout with a loaded theme, responds to keyboard navigation, and displays placeholder content from a test store.

### Phase 4: TUI Features (parallel)

Multiple agents work on distinct TUI features simultaneously, all building on the shell from Phase 3.

**Agent F: Views + Schedule** → [06-views-schedule.md](./06-views-schedule.md)
**Agent G: Rituals + Capture** → [07-rituals-capture.md](./07-rituals-capture.md)
**Agent H: Budget + Detail Pane** → [08-budget-detail.md](./08-budget-detail.md)

**Integration checkpoint:** each feature agent's work compiles and renders within the TUI shell. Views display query results. The schedule shows time displacement. Rituals step through their sequence. The capture bar creates nodes.

### Phase 5: Integration + CLI

One agent wires everything together: CLI commands (`wyrd add`, `wyrd journal`, `wyrd note`, `wyrd spend`, `wyrd sync`, `wyrd plugin`), the `$EDITOR` integration for writing, and end-to-end workflows.

**Agent I: CLI + Integration** → [09-cli-integration.md](./09-cli-integration.md)

## Orchestrator Responsibilities

### Before Assigning Work

1. Read the full documentation suite (the `/docs/` directory). Every ADR, every format spec, every schema document. The phase briefs reference these; you need the context to resolve ambiguities.
2. Create a test store with representative data: several task nodes, a few journal entries, notes, edges of each built-in type, a budget envelope, and a ritual definition. This store will be used for integration testing at every checkpoint.

### During Each Phase

1. Assign the relevant phase brief to each developer agent.
2. Point each agent at the specific documentation files it needs (listed in its brief).
3. Monitor for cross-cutting concerns: if Agent A makes a decision about the index API shape, Agent B needs to know before it builds query evaluation against that index.
4. When an agent asks a design question not covered by the docs, make the call and record it. Don't let agents block on ambiguity.

### At Each Integration Checkpoint

1. Pull all agent outputs into a single workspace.
2. Run each agent's test suite.
3. Run integration tests that cross subsystem boundaries (e.g. "load store → run query → verify results").
4. If integration fails, identify which agent's output is the source of the mismatch and send it back with specific instructions.

### Cross-Cutting Concerns to Watch

**UUID generation:** use `github.com/google/uuid` consistently. Every agent generating UUIDs should use v4.

**Time handling:** `time.Time` in Go, ISO 8601 strings in JSONC. The store layer should expose a `Now()` function that tests can override.

**Error handling:** Go convention (return errors, don't panic). Agents should define typed errors for their domain (e.g. `QueryParseError`, `MergeConflictError`).

**JSONC parsing:** choose one Go JSONC library and mandate it across all agents. Candidates: `github.com/nicholasgasior/gojsonc` or strip comments then use `encoding/json`. The store layer agent decides; all others follow.

**Logging:** structured logging via `log/slog`. All agents use the same logger interface.

## File Structure for the Built Application

```
wyrd/
  cmd/
    wyrd/main.go              # CLI entrypoint
  internal/
    store/                    # Store Layer (Agent A)
      store.go                # Store interface
      index.go                # In-memory graph index
      node.go                 # Node CRUD
      edge.go                 # Edge CRUD
      template.go             # Template loading + field merging
      jsonc.go                # JSONC parsing utilities
    query/                    # Query Engine (Agent B)
      parser.go               # Cypher subset parser (participle)
      ast.go                  # AST node types
      evaluator.go            # Query evaluation against index
      functions.go            # Aggregation functions
      variables.go            # $today, $now, date arithmetic
    sync/                     # Sync Engine (Agent C)
      git.go                  # Git operations (add, commit, pull, push)
      merge.go                # Three-way JSONC merge driver
    plugin/                   # Plugin System (Agent D)
      manager.go              # Plugin discovery, lifecycle
      protocol.go             # JSON-lines protocol handling
      shell.go                # wyrd-plugin-shell executor
    tui/                      # TUI (Agents E-H)
      app.go                  # Bubble Tea application root
      theme.go                # Theme loading + colour tier detection
      layout.go               # Split-pane layout
      keyboard.go             # Keyboard model + command palette
      views/
        list.go               # List display mode
        timeline.go           # Timeline display mode
        prose.go              # Prose display mode
        budget.go             # Budget display mode
        schedule.go           # Schedule view + time displacement
      ritual/
        runner.go             # Ritual step sequencer
        steps.go              # Step type implementations
      capture.go              # Capture bar
      detail.go               # Node detail pane
      statusbar.go            # Status bar + time tracker
    cli/                      # CLI Commands (Agent I)
      add.go
      journal.go
      note.go
      spend.go
      sync.go
      plugin.go
  store/                      # User's data directory (git repo)
    nodes/
    edges/
    templates/
    themes/
    rituals/
    views/
    plugins/
    config.jsonc
```

## Interface Contracts

These are the critical interfaces that multiple agents depend on. Define them early and freeze them before Phase 2 begins.

### Store → Query Engine

```go
// The query engine needs these from the store
type GraphIndex interface {
    AllNodes() []Node
    NodeByID(id uuid.UUID) (Node, bool)
    EdgesFrom(id uuid.UUID) []Edge
    EdgesTo(id uuid.UUID) []Edge
    EdgesByType(edgeType string) []Edge
    NodesWithType(nodeType string) []Node
    NodesMatching(predicate func(Node) bool) []Node
}
```

### Store → Sync Engine

```go
type StoreFS interface {
    StorePath() string           // path to /store directory
    ReadNode(id uuid.UUID) (Node, error)
    WriteNode(node Node) error
    ReadEdge(id uuid.UUID) (Edge, error)
    WriteEdge(edge Edge) error
    AllNodeFiles() ([]string, error)
    AllEdgeFiles() ([]string, error)
}
```

### Plugin System → Store

```go
type PluginStore interface {
    UpsertNode(node Node) error
    UpsertEdge(edge Edge) error
    DeleteEdge(id uuid.UUID) error   // edges can be deleted; nodes cannot
    NodeByID(id uuid.UUID) (Node, bool)
}
```

### Query Engine → TUI

```go
type QueryRunner interface {
    Execute(cypher string) (QueryResult, error)
}

type QueryResult struct {
    Columns []string
    Rows    []map[string]interface{}
}
```

## Test Store Seed Data

Create this before any agent begins work. Save it as a fixture that all test suites can load.

Minimum contents: 8 task nodes (various statuses, energies, and edge relationships), 3 journal nodes, 2 note nodes, 1 project node, 1 budget envelope with a spend log, edges covering all 5 built-in types, the 3 starter templates, 1 theme file, 1 ritual definition, and 2 saved views.

This seed data is the shared reality that integration tests validate against.
