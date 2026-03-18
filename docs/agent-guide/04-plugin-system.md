# Phase 2B: Plugin System

*Agent brief. Read this, then read the referenced docs.*

## Your Mission

Build the plugin discovery, lifecycle management, JSON-lines protocol handler, and the built-in shell executor. Plugins are how Wyrd talks to the outside world (GitHub, Calendar, Obsidian) and how users trigger custom actions on nodes.

## Required Reading

- [Plugin manifest format](../formats/plugin-manifest.md)
- [ADR-004: Unified plugin architecture](../adr/adr-004-unified-plugin-architecture.md)
- [ADR-013: Plugin sync strategy](../adr/adr-013-plugin-sync-strategy.md)
- [ADR-011: Obsidian integration](../adr/adr-011-obsidian-integration.md) (as a concrete plugin example)

## Deliverables

### 1. Plugin Discovery

On startup, scan two locations for plugin manifests:

- `/store/plugins/{name}/plugin.jsonc` (synced via git)
- `~/.wyrd/plugins/{name}/plugin.jsonc` (local-only)

Parse each manifest. Validate its structure. Build a registry of available plugins with their capabilities.

### 2. Executable Resolution

The manifest references an executable by name. Resolve it by searching:

1. `PATH` (for globally installed binaries)
2. `~/.wyrd/bin/` (per-machine plugin binaries)
3. The plugin's own directory (for script plugins where the script lives alongside the manifest)

For `executable_type: "script"`, use the appropriate interpreter (detect from shebang or file extension). For `executable_type: "binary"`, execute directly.

### 3. JSON-Lines Protocol

Communication with plugins is JSON-lines (one JSON object per line) over stdin/stdout.

**Sync flow:**
```
Wyrd → Plugin: {"op": "sync", "config": {...}}
Plugin → Wyrd: {"op": "upsert_node", "node": {...}}
Plugin → Wyrd: {"op": "upsert_edge", "edge": {...}}
Plugin → Wyrd: {"op": "done", "summary": {"created": N, "updated": N, "errors": N}}
```

**Action flow:**
```
Wyrd → Plugin: {"op": "action", "node": {...}, "config": {...}}
Plugin → Wyrd: (optional) {"op": "upsert_node", ...}
Plugin → Wyrd: {"op": "done", ...}
```

Handle malformed JSON-lines gracefully: log the error, skip the line, continue reading. A single bad line should not crash the protocol.

Handle plugin timeout: if no output for a configurable duration (default 30 seconds), kill the process and report the timeout.

Handle plugin crash: if the process exits non-zero before sending `done`, report the error with whatever stderr the plugin produced.

### 4. Upsert Handling

Sync plugins produce `upsert_node` and `upsert_edge` operations. For nodes from external sources:

- Node IDs for synced content use deterministic UUIDs derived from the source ID (UUID v5 with a Wyrd namespace). Re-running sync updates existing nodes rather than creating duplicates.
- Synced nodes carry a `source` property (see node schema).
- Upserts go through the store layer's `UpsertNode` / `UpsertEdge` methods.

### 5. Shell Executor

The built-in `wyrd-plugin-shell` handles simple command-template actions. It reads the `config_schema.command` field, interpolates `{{property.path}}` tokens from the target node's properties, and executes the result via `os/exec`.

Example: `"command": "open {{source.url}}"` on a node with `source.url: "https://github.com/..."` executes `open https://github.com/...`.

The shell executor is itself registered as a plugin with its own manifest; it just happens to be built into the Wyrd binary.

### 6. Plugin Install/Export

- `wyrd plugin install {path|url}`: extracts a `.wyrdplugin` archive (a zip/tar containing the plugin directory), places it in `/store/plugins/`, and installs any binary to `~/.wyrd/bin/`.
- `wyrd plugin export {name}`: packages the plugin directory into a `.wyrdplugin` archive.

## Constraints

- Plugin processes must be isolated. A crashing plugin cannot take down Wyrd. Use `os/exec` with proper process group management so child processes can be killed cleanly.
- Plugin stderr is captured for error reporting but not displayed to the user by default (only on error).
- The manifest `config_schema` is validated with JSON Schema. If a user's `config.jsonc` doesn't match the schema, report the specific validation error at startup rather than failing at runtime.

## Testing Requirements

- Test discovery: plant mock plugin directories, verify they're found and parsed.
- Test protocol: write a mock plugin (a simple Go program or shell script) that responds to sync and action operations. Verify upserts are applied to the store.
- Test shell executor: `{{property.path}}` interpolation with nested properties, missing properties (should error, not silently produce empty string), and special characters in values.
- Test timeout and crash handling.
- Test deterministic UUID generation: same source ID always produces the same node ID.

## Output Structure

```
internal/plugin/
  manager.go        # Discovery, registry, lifecycle
  protocol.go       # JSON-lines reader/writer
  shell.go          # wyrd-plugin-shell executor
  install.go        # Plugin install/export
  manager_test.go
  protocol_test.go
  shell_test.go
  testdata/
    mock-sync-plugin/
    mock-action-plugin/
```
