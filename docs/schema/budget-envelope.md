# Budget Envelope Format

Budget envelopes are nodes with a `budget` type. They use the standard node schema with budget-specific template fields.

## Template

```jsonc
// /store/templates/budget.jsonc
{
  "type_name": "budget",
  "fields": {
    "category": {
      "type": "string",
      "required": true
    },
    "currency": {
      "type": "string",
      "default": "GBP"
    },
    "period": {
      "type": "enum",
      "values": ["week", "month", "quarter", "year"],
      "default": "month"
    },
    "allocated": {
      "type": "number",
      "required": true
    },
    "carry_over": {
      "type": "boolean",
      "default": false
    },
    "warn_at": {
      "type": "number",
      "default": 0.8,
      "description": "Fraction of allocated at which caution status triggers"
    },
    "spend_log": {
      "type": "object",
      "default": []
    }
  }
}
```

## Example Node

```jsonc
{
  "id": "aabb1122-3344-5566-7788-99aabbccddee",
  "body": "Records",
  "types": ["budget"],
  "created": "2026-01-01T00:00:00Z",
  "modified": "2026-03-18T10:00:00Z",
  "category": "records",
  "currency": "GBP",
  "period": "month",
  "allocated": 50,
  "carry_over": false,
  "warn_at": 0.8,
  "spend_log": [
    { "date": "2026-03-04", "amount": 18.50, "note": "Discogs - Bauhaus repress" },
    { "date": "2026-03-12", "amount": 24.00, "note": "Rough Trade" }
  ]
}
```

## CLI

```bash
wyrd spend records 18.50 "Discogs - Bauhaus repress"
```

Matches the budget node by `category` field, appends to `spend_log`, updates `modified`.

## Budget Status

Computed at index time as a derived property. Status is determined by the ratio of current-period spend to allocated amount:

| Status | Condition | Glyph | Colour role |
|--------|-----------|-------|-------------|
| ok | spend < (allocated × warn_at) | ● | `budget.ok` |
| caution | spend ≥ (allocated × warn_at) | ◐ | `budget.caution` |
| over | spend ≥ allocated | ○ | `budget.over` |

## Edges

Budget envelopes can link to other nodes via standard edges. A `related` edge from a budget to a project enables queries like:

```cypher
MATCH (b:budget {category: "baby_prep"})<-[:related]-(t:task)
RETURN t.body, t.status
```
