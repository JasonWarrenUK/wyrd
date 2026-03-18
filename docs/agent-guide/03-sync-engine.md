# Phase 2A: Sync Engine

*Agent brief. Read this, then read the referenced docs.*

## Your Mission

Build the git-based sync system and the custom three-way JSONC merge driver. This is what makes Wyrd work across multiple machines without a server.

## Required Reading

- [ADR-005: Conflict resolution](../adr/adr-005-conflict-resolution.md)
- [ADR-006: Append-only nodes](../adr/adr-006-append-only-nodes.md)
- [ADR-001: Graph as flat files](../adr/adr-001-graph-as-flat-files.md)

## Deliverables

### 1. Git Operations

Wrap git commands for the `/store` directory. These can shell out to `git` or use `go-git`; the orchestrator's preference is shelling out for simplicity unless you have a strong reason not to.

- `Init()`: initialise `/store` as a git repo if it isn't already. Set up `.gitattributes` for the merge driver.
- `Sync()`: the full `wyrd sync` cycle: stage all changes, commit with auto-generated message, pull with rebase, push. Handle each step's failure modes explicitly.
- `Status()`: report uncommitted changes (for the TUI status bar).

### 2. Three-Way JSONC Merge Driver

A Go binary registered as a custom merge driver in `.gitattributes`. Git calls it with three file paths: base (common ancestor), ours (local), theirs (remote).

The merge rules from ADR-005:

**Scalar fields** changed on one side only: take the change. Changed on both sides to the same value: take either. Changed on both sides to different values: last-write-wins by `modified` timestamp; ties favour local (ours).

**Scalar deleted** on one side, changed on the other: change wins over delete.

**Simple arrays** (arrays of strings): union of additions from both sides. Deletions require both sides to agree (intersection of deletions).

**Arrays of objects** (e.g. `spend_log`): deduplicate by key field (typically `date` or composite of `date` + `note`). Conflicting entries for the same key: last-write-wins.

The merge driver writes the merged result to the "ours" file path and exits 0 on success, non-zero on unresolvable conflict (which should be rare given these rules).

### 3. Git Configuration

Generate or validate `.gitattributes`:

```
*.jsonc merge=wyrd-merge
```

Register the merge driver in `.git/config`:

```
[merge "wyrd-merge"]
    name = Wyrd JSONC three-way merge
    driver = wyrd merge-driver %O %A %B
```

### 4. Auto-Commit Messages

Generate commit messages from the staged changes. Format: `wyrd: {summary}` where summary lists the operations (e.g. "created 2 nodes, updated 1 node, created 3 edges"). Keep it concise; these are machine-generated messages for sync, not human-authored commits.

## Constraints

- The merge driver must handle JSONC comments. If the base has comments and both sides preserve them, the merged output retains them. If comments conflict (same line, different text), favour local.
- Nodes are never deleted. If git detects a delete/modify conflict (which shouldn't happen given ADR-006, but might if someone manually deletes a file), the merge driver should restore the file and set `status: "archived"`.
- The merge driver must be a standalone binary (it's invoked by git, not by the Wyrd process directly). It shares the JSONC parsing code from the store layer.
- Network errors during push/pull must produce clear, actionable messages ("push failed: remote rejected; run `git pull` first" rather than a raw git stderr dump).

## Testing Requirements

- Unit tests for every merge rule: scalar one-side change, both-side same change, both-side different change (LWW), delete vs change, array union, array-of-objects dedup.
- Integration test: two separate clones, concurrent edits to the same node, sync both, verify merged result.
- Test the pathological case: both sides add a type to the same node, each type contributing different fields. The merge should union the types array and include both sets of template fields.
- Test spend_log merging: both sides add entries on different dates (clean merge) and same date with different amounts (LWW).

## Output Structure

```
internal/sync/
  git.go            # Git operations (init, sync, status)
  merge.go          # Three-way JSONC merge logic
  merge_test.go
  git_test.go
cmd/
  wyrd-merge-driver/
    main.go         # Standalone merge driver binary
```
