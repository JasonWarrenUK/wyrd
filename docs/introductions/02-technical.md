# Wyrd: Technical Overview

*For developers and contributors.*

## Architecture

Wyrd is a directed property graph stored as flat JSONC files, with a terminal UI built in Go using Bubble Tea, and a Cypher subset query engine for data access.

### Data Model

Two entity types: **nodes** and **edges**, each stored as individual `.jsonc` files under `/store/nodes/` and `/store/edges/`. Flat namespaces; no directory hierarchy. Nodes carry a `types` array (e.g., `["task", "commitment"]`) and conditional fields contributed by templates. Edges are first-class entities with their own UUIDs and properties.

```
/store
  /nodes/{uuid}.jsonc
  /edges/{uuid}.jsonc
  /templates/{type_name}.jsonc
  /themes/{name}.jsonc
  /rituals/{name}.jsonc
  /views/{name}.jsonc
  /plugins/{name}/plugin.jsonc
  /integrations/{name}.jsonc
  /config.jsonc
```

### In-Memory Index

On startup, all files are read into an in-memory graph index supporting O(1) edge lookups per node, property-based filtering, and graph traversal. The index rebuilds on file change (filesystem watcher) and on `wyrd sync`.

### Query Engine

A read-only Cypher subset. Pattern matching, property filters, single-hop and variable-length edge traversal (capped at configurable depth, default 5), date arithmetic, and aggregation (`sum`, `count`, `avg`, `min`, `max`, `collect`). Mutations go through Wyrd commands, not queries. Parser built with Go's `participle` library.

### Sync

The `/store` directory is a git repository. Sync is explicit (`wyrd sync`). A custom three-way merge driver registered in `.gitattributes` handles JSONC merges at the property level: non-conflicting changes merge cleanly; conflicting scalar changes resolve by last-write-wins; arrays merge by union.

### Plugin System

Plugins communicate via JSON-lines over stdin/stdout. A plugin declares capabilities (`sync`, `action`, or both) in its manifest. Sync plugins pull data from external sources and stream node/edge upserts. Action plugins execute on a selected node (e.g., launching a Claude Code session with graph context). A built-in `wyrd-plugin-shell` executor handles simple command-template actions without custom code.

## Stack

| Layer | Technology | Rationale |
|-------|-----------|-----------|
| TUI | Go + Bubble Tea | Elm architecture for state; richest TUI ecosystem |
| Data | JSONC (one file per entity) | Human-annotatable; JSON Schema validation; git-friendly |
| Query | Cypher subset | Graph-native; proven syntax; handles traversals natively |
| Sync | Git | Offline-first; history; no server dependency |
| Plugins | Any language (JSON-lines protocol) | Language-agnostic; trivial to author |
| LLM (optional) | Ollama or remote API | Plugin layer; system fully functional without it |

## Key Design Decisions

Decisions are recorded as ADRs in `/adr/`. The most consequential:

**Graph as flat files** (ADR-001): nodes carry types as array properties, not directories. Any axis chosen for directory structure makes every other axis require a query.

**Cypher subset over JSON filters** (ADR-003): graph-native query syntax; multi-hop traversals are first-class. JSON filter spec was considered for v1 simplicity but rejected in favour of building the right foundation.

**Unified plugin model** (ADR-004): launch actions and sync integrations use the same plugin contract. A Claude Code launch and a GitHub sync are both plugins with different capabilities.

**Append-only nodes** (ADR-006): nodes are never deleted from disk. Archiving sets `status: "archived"`. This sidesteps git delete/modify conflicts entirely and provides full audit history.
