# Edge Schema

Stored at `/store/edges/{uuid}.jsonc`.

## Base Fields

```jsonc
{
  "id": "7c9e6679-7425-40de-944b-e07fc1f90ae7",  // UUID v4, auto-generated, immutable
  "type": "blocks",  // Built-in or user-defined
  "from": "550e8400-e29b-41d4-a716-446655440000",  // Source node UUID
  "to": "6ba7b810-9dad-11d1-80b4-00c04fd430c8",    // Target node UUID
  "created": "2026-03-17T10:35:00Z"  // Auto-generated, immutable
}
```

## Built-in Edge Types

| Type | Semantics | Optional Properties |
|------|-----------|-------------------|
| `blocks` | A prevents B from starting | `reason` (string) |
| `parent` | A contains B | — |
| `related` | Loose association | — |
| `waiting_on` | A is waiting for B (implies a person) | `promised_date` (date), `follow_up_date` (date) |
| `precedes` | A should happen before B (soft preference) | — |

## Edge Properties by Type

Properties vary by edge type. Any edge can carry additional user-defined properties.

```jsonc
// blocks edge with reason
{
  "id": "7c9e6679-7425-40de-944b-e07fc1f90ae7",
  "type": "blocks",
  "from": "550e8400-e29b-41d4-a716-446655440000",
  "to": "6ba7b810-9dad-11d1-80b4-00c04fd430c8",
  "created": "2026-03-17T10:35:00Z",
  "reason": "Can't start API integration until auth service is deployed"
}
```

```jsonc
// waiting_on edge with dates
{
  "id": "8a7b1234-5678-90ab-cdef-1234567890ab",
  "type": "waiting_on",
  "from": "550e8400-e29b-41d4-a716-446655440000",
  "to": "aabbccdd-1122-3344-5566-778899001122",
  "created": "2026-03-05T09:00:00Z",
  "promised_date": "2026-03-12",
  "follow_up_date": "2026-03-19"
}
```

## User-Defined Edge Types

Users may define additional edge types. Custom types follow the same schema; the `type` field is a free string. Custom edge type definitions can be documented in templates, though this is optional.
