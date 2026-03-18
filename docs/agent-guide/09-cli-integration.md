# Phase 5: CLI + Integration

*Agent brief. Read this, then read the referenced docs.*

## Your Mission

Build the CLI command surface (`wyrd add`, `wyrd journal`, `wyrd sync`, etc.), the `$EDITOR` integration for writing, and the end-to-end workflows that prove all subsystems work together. You are the last agent; your output is the shippable binary.

## Required Reading

- [02-intro-technical](../02-intro-technical.md) (full architecture overview)
- [03-intro-practical](../03-intro-practical.md) (how everything fits together)
- [ADR-012: First-class writing](../adr/adr-012-first-class-writing.md)
- [ADR-011: Obsidian integration](../adr/adr-011-obsidian-integration.md) (for the push/pull commands)
- [Budget envelope format](../formats/budget-envelope.md) (for the spend command)
- [Plugin manifest format](../formats/plugin-manifest.md) (for plugin commands)

## Deliverables

### 1. CLI Entrypoint

Use a Go CLI library (cobra or kong; orchestrator's preference) for the command tree:

```
wyrd                          # launches TUI
wyrd add "body text"          # quick-add task node
wyrd add --type note "title"  # quick-add with type
wyrd journal                  # open $EDITOR, create journal node
wyrd note "title"             # open $EDITOR, create note node
wyrd spend {cat} {amt} "note" # log spending
wyrd sync                     # git sync cycle
wyrd plugin install {path}    # install plugin
wyrd plugin export {name}     # export plugin
wyrd plugin list              # list installed plugins
wyrd query "MATCH ..."        # run a Cypher query, print results
wyrd view {name}              # run a saved view, print results
wyrd push {node-id}           # push node to Obsidian
wyrd pull obsidian [flags]    # pull from Obsidian vault
wyrd compact                  # (stub) future archive compaction
wyrd help                     # help text
```

### 2. Editor Integration

`wyrd journal` and `wyrd note "title"` open the user's `$EDITOR` (falling back to `vi` if unset). For journals, pre-fill the buffer with today's date as a heading. For notes, pre-fill with the title as a heading.

On save and exit, read the file contents and create the node via the store layer. Support `--link {node-id}` flag to create a `related` edge during capture.

Handle the editor failing to launch (EDITOR not set, binary not found) with a clear error message.

### 3. Quick-Add

`wyrd add "body text"` creates a task node (type: task, status: inbox) with the given body. `--type` flag overrides the type. `--link` creates an edge.

### 4. Query CLI

`wyrd query "MATCH (t:task) RETURN t"` runs the query through the engine and prints results as a table to stdout. Use tab-separated columns for machine readability, or a formatted table for TTY output (detect whether stdout is a terminal).

### 5. Sync CLI

`wyrd sync` wraps the sync engine. Print progress: staging, committing, pulling, pushing. Handle conflicts by reporting which files had merge issues. Handle network errors with specific guidance.

### 6. Store Initialisation

On first run (no `/store` directory), `wyrd` should initialise:

1. Create the `/store` directory structure.
2. Copy starter templates (task, journal, note) into `/store/templates/`.
3. Copy starter views into `/store/views/`.
4. Copy the default theme files into `/store/themes/`.
5. Create a minimal `config.jsonc`.
6. Initialise git in `/store`.
7. Create `.gitattributes` with the merge driver registration.

### 7. End-to-End Workflows

Verify these complete workflows work:

1. **First run:** `wyrd` → initialises store → launches TUI with empty schedule and starter views.
2. **Capture loop:** user adds tasks via capture bar → tasks appear in views → user selects a task → detail pane shows it.
3. **Morning planning:** ritual triggers → user steps through → creates journal entry → reviews inbox → sees schedule.
4. **Sync:** `wyrd sync` → commits local changes → pulls remote changes → merges → index rebuilds → TUI reflects merged state.
5. **Budget:** `wyrd spend records 18.50 "Discogs"` → budget view updates → progress bar changes → status glyph updates if threshold crossed.
6. **Writing:** `wyrd journal` → editor opens → user writes → saves → journal node created → journal view shows new entry.
7. **Plugin action:** user selects a node → triggers a plugin action via keybinding → plugin executes.

## Constraints

- The binary must be a single Go binary with no runtime dependencies. All assets (starter templates, themes) are embedded via `embed.FS`.
- Startup time target: under 500ms for a store with 500 nodes and 1500 edges. Profile and optimise if needed.
- The CLI must work without the TUI for scripting: `wyrd add`, `wyrd query`, `wyrd spend`, and `wyrd sync` should all work headlessly.
- Error messages should be actionable. "Failed to sync" is not acceptable. "Sync failed: push rejected because remote contains changes not present locally. Run `wyrd sync` again to pull and retry." is acceptable.

## Testing Requirements

- Test every CLI command with valid and invalid arguments.
- Test editor integration: mock $EDITOR with a script that writes known content, verify the created node.
- Test store initialisation: start from an empty directory, verify all starter files are created.
- End-to-end: run through each workflow listed above against a real (not mocked) store on the filesystem.
- Test headless operation: run CLI commands in a pipeline, verify stdout output is parseable.

## Output Structure

```
cmd/
  wyrd/
    main.go           # CLI entrypoint, command registration
internal/
  cli/
    add.go            # wyrd add
    journal.go        # wyrd journal (editor integration)
    note.go           # wyrd note (editor integration)
    spend.go          # wyrd spend
    sync.go           # wyrd sync
    query.go          # wyrd query
    view.go           # wyrd view
    plugin.go         # wyrd plugin install/export/list
    push.go           # wyrd push
    pull.go           # wyrd pull
    init.go           # Store initialisation
    editor.go         # $EDITOR integration helper
    add_test.go
    journal_test.go
    sync_test.go
    init_test.go
  embed/
    starter/          # Embedded starter templates, themes, views
```
