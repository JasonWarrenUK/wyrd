# ADR-005: Three-Way Merge Conflict Resolution

**Status:** Accepted

## Context

Wyrd syncs via git. Two machines editing the same node file create merge conflicts. JSONC files need property-level merging, not line-level.

## Decision

A custom Go merge driver registered in `.gitattributes` performs three-way merges at the JSON property level.

**Rules:**

1. **Scalars** changed on one side only: take the change.
2. **Scalars** changed on both sides to the same value: take either.
3. **Scalars** changed on both sides to different values: last-write-wins by `modified` timestamp. Ties favour local.
4. **Scalars** deleted on one side, changed on the other: change wins over delete.
5. **Simple arrays** (strings): union of additions from both sides. Deletions require both sides to agree (intersection of deletions).
6. **Arrays of objects** (e.g., habit logs, spend entries): deduplicate by key field (typically `date` or `id`). Conflicting entries for the same key: last-write-wins.

```
# .gitattributes
*.jsonc merge=wyrd-merge

# .git/config
[merge "wyrd-merge"]
    name = Wyrd JSONC three-way merge
    driver = wyrd merge-driver %O %A %B
```

## Consequences

**Positive:** most concurrent edits merge automatically. Edge files (one per edge) almost never conflict. The conservative array union prevents accidental data loss. Git's reflog provides recovery for any merge result.

**Negative:** the merge driver is a custom binary that must be installed on every machine. The "change wins over delete" rule means removed fields can reappear after a sync (low-friction to re-delete, but surprising). Array union can produce duplicates if both sides add semantically identical entries with different representations.

## Amendment (2026-08-04)

Two notes. First, the `.git/config` snippet above names a `wyrd merge-driver` subcommand that has never existed; the driver is the separate `wyrd-merge-driver` binary, and the 2026-08-04 audit found the stanza is not written by the live init path at all — roadmap Milestone H (SY.1) wires the registration correctly. Second, rule 6's canonical example (spend entries) retires when SP.8 replaces the embedded `spend_log` with movement nodes; the rule itself remains for other object arrays (e.g. habit logs).
