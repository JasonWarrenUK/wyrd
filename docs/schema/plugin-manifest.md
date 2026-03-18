# Plugin Manifest Format

Stored at `/store/plugins/{name}/plugin.jsonc`.

## Structure

```jsonc
{
  "name": "github",
  "version": "0.1.0",
  "description": "Sync issues and PRs from GitHub repositories",
  "executable": "wyrd-plugin-github",
  "executable_type": "binary",  // "binary" or "script"
  "capabilities": ["sync"],

  // Capability-specific config (include sections matching declared capabilities)

  "sync": {
    "triggers": ["manual", "schedule"],
    "node_types": ["github_issue", "github_pr"],
    "edge_types": []
  },

  "config_schema": {
    "repos": {
      "type": "array",
      "description": "Repositories to sync",
      "items": {
        "repo": { "type": "string", "description": "owner/repo" },
        "project": { "type": "string", "description": "Wyrd project node to link" },
        "sync": {
          "type": "object",
          "properties": {
            "issues": { "type": "object" },
            "prs": { "type": "object" }
          }
        }
      }
    }
  }
}
```

## Capabilities

A plugin declares one or more capabilities.

**`sync`:** pulls data from external sources. Wyrd sends `{"op": "sync", "config": {...}}`. The plugin streams back `upsert_node`, `upsert_edge`, and `delete_edge` operations.

**`action`:** executes on a selected node in the TUI. Wyrd sends `{"op": "action", "node": {...}, "config": {...}}`. The plugin may stream back node/edge operations or simply execute a side effect.

```jsonc
// Action capability block
"action": {
  "label": "Claude Code session",
  "keybinding": "gc",
  "applicable_types": ["task", "note", "project"]
}
```

A plugin can declare both: the Obsidian plugin is `["sync", "action"]` (syncs notes in, pushes journals out).

## Config Schema

The `config_schema` field describes the plugin's configuration shape. The TUI config manager uses this to render a configuration form. Standard JSON Schema types: `string`, `number`, `boolean`, `array`, `object`. Each field supports `description`, `default`, and `required`.

## Shell Executor

The built-in `wyrd-plugin-shell` handles simple command-template actions.

```jsonc
{
  "name": "open-in-github",
  "version": "0.1.0",
  "executable": "wyrd-plugin-shell",
  "executable_type": "binary",
  "capabilities": ["action"],
  "action": {
    "label": "Open in GitHub",
    "keybinding": "go",
    "applicable_types": ["github_issue", "github_pr"]
  },
  "config_schema": {
    "command": {
      "type": "string",
      "default": "open {{source.url}}"
    }
  }
}
```

`{{property.path}}` tokens are interpolated from the target node's properties.

## Protocol

Communication is JSON-lines over stdin/stdout.

```
// Wyrd → Plugin
{"op": "sync", "config": {"repos": [...]}}

// Plugin → Wyrd
{"op": "upsert_node", "node": {"id": "...", "body": "...", "types": ["github_issue"], ...}}
{"op": "upsert_edge", "edge": {"type": "related", "from": "...", "to": "..."}}
{"op": "done", "summary": {"created": 3, "updated": 1, "errors": 0}}
```

Node IDs for synced content use deterministic UUIDs derived from the source ID, so re-running sync updates rather than duplicates.

## File Layout

```
/store/plugins/github/
  plugin.jsonc     # manifest (syncs via git)
  config.jsonc     # user config (syncs via git)

~/.wyrd/bin/
  wyrd-plugin-github   # binary (installed per-machine)
```

Script plugins may include the script in the plugin directory:

```
/store/plugins/obsidian/
  plugin.jsonc
  config.jsonc
  sync.py          # script (syncs via git)
```
