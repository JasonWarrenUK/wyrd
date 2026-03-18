# ADR-001: Graph Stored as Flat JSONC Files

**Status:** Accepted

## Context

Wyrd's data model is a directed property graph. Nodes can belong to multiple projects and carry multiple types simultaneously. The storage layer needs to support this without imposing a tree structure on graph data.

Options considered: a database (SQLite, Neo4j embedded), nested directory structures mirroring projects or types, and flat file namespaces.

## Decision

Each node and each edge is stored as an individual JSONC file in flat namespaces (`/store/nodes/{uuid}.jsonc`, `/store/edges/{uuid}.jsonc`). Types are an array property on the node, not a directory path.

Edges are first-class files with their own UUIDs and properties rather than embedded arrays within node files.

## Consequences

**Positive:** git produces clean diffs (one file per entity). Concurrent edge creation on two machines merges without conflict. Any query axis works equally well since nothing is privileged by directory structure. JSONC allows inline human annotations via comments. Edges can carry rich metadata (reasons, timestamps, resolved dates).

**Negative:** the app must build an in-memory index at startup; the filesystem alone doesn't support efficient querying. Large stores (thousands of nodes) will have slow startup until index caching is implemented. Filename-based discovery is useless; all access goes through the index.

**Mitigations:** filesystem watcher handles incremental index updates after startup. Future work: serialised index cache that validates against file modification times.
