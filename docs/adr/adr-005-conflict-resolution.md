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
