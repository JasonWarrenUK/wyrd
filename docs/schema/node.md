# Node Schema

Stored at `/store/nodes/{uuid}.jsonc`.

## Base Fields

Every node has these fields regardless of type.

```jsonc
{
  "id": "550e8400-e29b-41d4-a716-446655440000",  // UUID v4, auto-generated, immutable
  "body": "Markdown content.\n\nSupports **formatting**.",  // Primary content, required
  "types": ["task", "commitment"],  // Minimum one type, determines active template fields
  "created": "2026-03-17T10:30:00Z",  // Auto-generated, immutable
  "modified": "2026-03-17T14:22:00Z"  // Auto-updated on any field change
}
```

## Conditional Fields

When a type is added to a node, the corresponding template's fields merge in with their defaults. When a type is removed, its fields are retained but hidden from the default view (never silently deleted).

```jsonc
{
  "id": "550e8400-e29b-41d4-a716-446655440000",
  "body": "Review slide deck structure for AM2 presentation.",
  "types": ["task", "commitment"],
  "created": "2026-03-17T10:30:00Z",
  "modified": "2026-03-17T14:22:00Z",

  // From "task" template
  "status": "in_progress",
  "energy": "deep",
  "contexts": ["at_computer"],
  "people": ["dan"],
  "due": "2026-04-01",

  // From "commitment" template (additional fields that "task" doesn't provide)
  "promised_to": "dan",
  "promised_date": "2026-03-10"
}
```

## Synced Nodes

Nodes created by sync plugins carry a `source` property.

```jsonc
{
  "source": {
    "type": "github",
    "repo": "JasonWarrenUK/rhea",
    "id": "issues/42",
    "url": "https://github.com/JasonWarrenUK/rhea/issues/42",
    "last_synced": "2026-03-18T14:00:00Z"
  }
}
```

## Archived Nodes

Nodes are never deleted from disk. Archiving sets `status: "archived"`. The node remains queryable but excluded from standard views unless explicitly filtered.
