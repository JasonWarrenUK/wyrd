# Saved View Format

Stored at `/store/views/{name}.jsonc`.

## Structure

A saved view is a named Cypher query with optional presentation metadata.

```jsonc
{
  "name": "Deep work queue",
  "pinned": true,
  "query": "MATCH (t:task {status: 'ready', energy: 'deep'}) WHERE NOT (t)<-[:blocks]-({status: 'in_progress'}) RETURN t ORDER BY t.due ASC",
  "display": "list",
  "columns": ["body", "due", "contexts"]
}
```

## Fields

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `name` | string | yes | Display name in sidebar and command palette |
| `pinned` | boolean | no | Whether the view appears in the TUI sidebar (default: false) |
| `query` | string | yes | Cypher query |
| `display` | enum | no | Rendering mode: `list`, `timeline`, `prose`, `budget` (default: `list`) |
| `columns` | string_array | no | Which node fields to show in list mode |

## Display Modes

**`list`**: tabular view with columns. Default for task queries. Shows one row per node with selected fields.

**`timeline`**: reverse-chronological full text. Default for journal queries. Shows body content with date headers.

**`prose`**: single-node detail with body and edge list. Default for note queries. Wiki-style presentation.

**`budget`**: envelope progress bars with status glyphs. Used for budget views. Expects nodes with `allocated` and `spend_log` properties.

## Starter Views

These ship with Wyrd and can be modified or deleted.

```jsonc
// blocked-tasks.jsonc
{
  "name": "Blocked tasks",
  "pinned": true,
  "query": "MATCH (t:task)<-[:blocks]-(b) WHERE b.status <> 'done' AND b.status <> 'dropped' RETURN t, b.body AS blocked_by",
  "display": "list"
}
```

```jsonc
// waiting-on-others.jsonc
{
  "name": "Waiting on others",
  "pinned": true,
  "query": "MATCH (t)-[:waiting_on]->(p) RETURN t, p.body AS waiting_for, t.created AS since ORDER BY t.created ASC",
  "display": "list"
}
```

```jsonc
// journal.jsonc
{
  "name": "Journal",
  "pinned": true,
  "query": "MATCH (j:journal) RETURN j ORDER BY j.created DESC",
  "display": "timeline"
}
```

```jsonc
// recent-notes.jsonc
{
  "name": "Recent notes",
  "pinned": false,
  "query": "MATCH (n:note) RETURN n ORDER BY n.modified DESC LIMIT 20",
  "display": "list",
  "columns": ["body", "tags", "modified"]
}
```
