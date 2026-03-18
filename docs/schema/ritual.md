# Ritual Format

Stored at `/store/rituals/{name}.jsonc`.

## Structure

```jsonc
{
  "name": "morning_planning",
  "schedule": { "days": ["mon", "tue", "wed", "thu", "fri"], "time": "08:00" },
  "friction": "gate",
  "steps": [
    {
      "type": "query_summary",
      "label": "Inbox",
      "query": "MATCH (n {status: 'inbox'}) RETURN count(n) AS total",
      "template": "{{total}} items waiting for triage"
    },
    {
      "type": "view",
      "view": "today_calendar"
    },
    {
      "type": "action",
      "action": "propose_schedule"
    },
    {
      "type": "gate",
      "condition": "MATCH (n {status: 'inbox'}) RETURN count(n) > 0 AS should_triage",
      "field": "should_triage",
      "then": { "type": "action", "action": "open_triage" }
    },
    {
      "type": "query_list",
      "label": "Neglected projects",
      "query": "MATCH (p:project) WHERE p.modified < date() - duration('P7D') AND p.status <> 'paused' RETURN p ORDER BY p.modified ASC",
      "empty_message": "Nothing neglected. Suspicious, but fine."
    },
    {
      "type": "prompt",
      "text": "Anything on your mind before you start?",
      "creates": { "types": ["journal"], "edge": { "type": "related", "to": "$today_schedule" } }
    }
  ]
}
```

## Step Types

**`query_summary`**: runs a Cypher query returning scalars, renders the result into a template string using `{{field}}` interpolation. Read-only.

**`query_list`**: runs a Cypher query returning nodes, renders as a scrollable list. Supports inline editing (update properties, change status, create edges). Displays `empty_message` when the query returns no results.

**`view`**: renders a named saved view or built-in view (e.g., `today_calendar`).

**`action`**: triggers a built-in system action. Valid actions: `propose_schedule`, `open_triage`, `open_journal`, `sync`, `rebuild_index`.

**`gate`**: conditional step. Runs a Cypher query returning a boolean field. If the named `field` is true, executes the `then` step. If false, skips silently.

**`prompt`**: free-text input. The `creates` object defines the resulting node's types and any automatic edges.

## Friction Levels

**`gate`**: when triggered, the TUI enters ritual mode. The user must complete all steps or explicitly defer the ritual.

**`nudge`**: appears as a prominent suggestion that can be silently bypassed.

## Schedule Format

```jsonc
// Specific days and time
{ "days": ["mon", "wed", "fri"], "time": "09:00" }

// Daily
{ "days": ["mon", "tue", "wed", "thu", "fri", "sat", "sun"], "time": "21:00" }

// Weekly
{ "day": "sun", "time": "09:00" }
```
