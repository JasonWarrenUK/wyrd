# ADR-006: Append-Only Nodes

**Status:** Accepted

## Context

Git handles file deletion differently from file modification. If machine A deletes a node file and machine B edits it, git flags a delete/modify conflict that the custom merge driver cannot resolve (the file doesn't exist on one side).

## Decision

Wyrd never deletes node files from disk. "Deleting" a node sets `status: "archived"`. The node remains in `/store/nodes/` indefinitely.

Edge files pointing to archived nodes become inert: the index skips them in standard queries, but they remain accessible via explicit `status: "archived"` filters.

## Consequences

**Positive:** delete/modify conflicts are eliminated entirely. Git history is append-only for nodes, providing full audit trail. Accidental deletions are impossible. Archived nodes can be restored by changing their status back.

**Negative:** the store grows monotonically. Old nodes accumulate. Mitigation: a periodic `wyrd compact` command (future) could move archived nodes older than a threshold to a separate directory or git-ignored archive, but this is not planned for any current phase.

## Amendment (2026-08-04)

The mitigation shipped: `wyrd compact` (roadmap CO.1/CO.2) moves archived nodes to an `archive/` directory alongside the edges that reference them, with a `--dry-run` preview. The "not planned for any current phase" clause above is stale. The core decision is unchanged — nodes are still never deleted, only archived and (optionally) relocated.
