# ADR-004: Unified Plugin Architecture

**Status:** Accepted

## Context

Wyrd integrates with external tools (GitHub, Linear, Gmail, Obsidian, Google Calendar) and supports user-defined actions (launching Claude Code, opening URLs). Early design separated these into "sync integrations" (pull data) and "launch actions" (execute commands on nodes). This created two discovery mechanisms, two config formats, and an awkward migration path when a simple action grew complex enough to need output capture.

## Decision

Collapse everything into a single plugin system. A plugin is any executable that communicates with Wyrd via JSON-lines over stdin/stdout.

**Capabilities:** each plugin declares one or more capabilities in its manifest: `sync` (pulls data from external sources), `action` (executes on a selected node in the TUI). Future capability: `transform` (modifies existing nodes).

**Protocol:** Wyrd sends an operation object to stdin. The plugin responds with a stream of JSON-line operations to stdout.

For sync: `→ {"op": "sync", "config": {...}}` / `← {"op": "upsert_node", "node": {...}}` / `← {"op": "done", ...}`

For actions: `→ {"op": "action", "node": {...}, "config": {...}}` / `← (same response types)`

**Simple actions:** a built-in `wyrd-plugin-shell` executor handles command-template actions (e.g., `open {{source.url}}`). These require only a manifest and config, no custom code.

**Discovery:** plugins live in `/store/plugins/{name}/` (synced via git) or `~/.wyrd/plugins/` (local). Wyrd scans both at startup.

**Export/import:** `wyrd plugin export {name}` produces a `.wyrdplugin` archive. `wyrd plugin install {path|url}` extracts and registers.

## Consequences

**Positive:** single system to learn, document, and maintain. Any language can implement a plugin (just read/write JSON-lines). The shell executor means trivial actions require zero code. Plugin export/import enables sharing.

**Negative:** the JSON-lines protocol adds overhead for actions that could be a simple `exec()` call. The protocol must handle error reporting, timeouts, and malformed output gracefully. Plugin development requires understanding the protocol even for simple cases (unless using the shell executor).
