# Phase 1A: Store Layer

*Agent brief. Read this, then read the referenced docs.*

## Your Mission

Build the data storage and in-memory indexing layer. This is the foundation everything else sits on. The query engine, sync engine, plugin system, and TUI all depend on your work.

## Required Reading

Before writing any code, read these documents in full:

- [Node schema](../schema/node.md)
- [Edge schema](../schema/edge.md)
- [Template format](../formats/template.md)
- [ADR-001: Graph as flat files](../adr/adr-001-graph-as-flat-files.md)
- [ADR-006: Append-only nodes](../adr/adr-006-append-only-nodes.md)
- [ADR-010: Starter templates](../adr/adr-010-starter-templates.md)

## Deliverables

### 1. JSONC Parser/Writer

Read and write JSONC files (JSON with `//` and `/* */` comments). Comments must be preserved through read/write cycles; silently stripping them is not acceptable (the user annotates their data files with comments). Choose one Go JSONC library and commit to it; every other agent will use the same one.

### 2. Node CRUD

- `CreateNode(body string, types []string) (Node, error)`: generates UUID v4, sets `created` and `modified`, merges template fields for each type with their defaults.
- `UpdateNode(id uuid.UUID, updates map[string]interface{}) (Node, error)`: applies updates, bumps `modified`. Validate updates against template field types.
- `ArchiveNode(id uuid.UUID) error`: sets `status: "archived"`, bumps `modified`. Never deletes the file.
- `ReadNode(id uuid.UUID) (Node, error)`: reads from disk.
- `AddType(id uuid.UUID, typeName string) error`: adds a type to the `types` array, merges in that template's fields with defaults.
- `RemoveType(id uuid.UUID, typeName string) error`: removes a type from `types`, retains the template's fields in the file (hidden from default view, never deleted).

### 3. Edge CRUD

- `CreateEdge(edgeType string, from uuid.UUID, to uuid.UUID, properties map[string]interface{}) (Edge, error)`: generates UUID v4, sets `created`.
- `DeleteEdge(id uuid.UUID) error`: edges (unlike nodes) are actually deleted from disk.
- `ReadEdge(id uuid.UUID) (Edge, error)`: reads from disk.

### 4. Template Loading

- Scan `/store/templates/` on startup.
- Parse each template file. Validate the structure.
- Provide a `FieldsForType(typeName string) (map[string]FieldDef, error)` function that the node CRUD operations use to merge fields.
- Handle the case where multiple types on a node have overlapping field names (first type in the `types` array takes precedence for defaults).

### 5. In-Memory Graph Index

Build on startup by scanning all node and edge files. The index supports:

- O(1) node lookup by ID
- O(1) edge lookup from/to a given node
- Filter nodes by type
- Filter nodes by arbitrary predicate
- List all edges of a given type
- Rebuild incrementally via filesystem watcher (use `fsnotify`)

The index must implement the `GraphIndex` interface defined in the orchestrator guide. This is the contract the query engine builds against.

### 6. File Watching

Use `fsnotify` to watch `/store/nodes/` and `/store/edges/`. On file change, update the in-memory index incrementally (don't rebuild the entire index for a single file change).

## Constraints

- Nodes are never deleted from disk. `ArchiveNode` sets status; it does not remove the file.
- Comments in JSONC files must survive read/write. If the chosen library can't do this, wrap it.
- File writes must be atomic (write to temp file, then rename) to prevent corruption on crash.
- `modified` timestamps use `time.Now()` via an injectable clock (for testability).
- All file I/O errors must be wrapped with context (which file, what operation).

## Testing Requirements

- Unit tests for every CRUD operation.
- Test template field merging with single-type and multi-type nodes.
- Test the index: load a seed store, verify lookups, verify incremental updates.
- Test that archiving a node makes it invisible to default type filters but visible to explicit `status: "archived"` queries.
- Test comment preservation in JSONC round-trips.

## Output Structure

```
internal/store/
  store.go          # Public interface
  index.go          # GraphIndex implementation
  node.go           # Node operations
  edge.go           # Edge operations
  template.go       # Template loading + field merging
  jsonc.go          # JSONC read/write with comment preservation
  watcher.go        # fsnotify integration
  store_test.go
  index_test.go
  node_test.go
  edge_test.go
  template_test.go
  jsonc_test.go
  testdata/         # Seed store for tests
```
