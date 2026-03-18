# Template Format

Stored at `/store/templates/{type_name}.jsonc`.

Templates define which fields a node type contributes, their data types, default values, and valid enums.

## Structure

```jsonc
{
  "type_name": "task",
  "fields": {
    "status": {
      "type": "enum",
      "values": ["inbox", "ready", "in_progress", "blocked", "done", "dropped"],
      "default": "inbox"
    },
    "energy": {
      "type": "enum",
      "values": ["low", "medium", "deep"],
      "default": "medium"
    },
    "contexts": {
      "type": "string_array",
      "default": []
    },
    "people": {
      "type": "string_array",
      "default": []
    },
    "due": {
      "type": "date",
      "optional": true
    }
  }
}
```

## Supported Field Types

| Type | Description | Example default |
|------|-------------|----------------|
| `string` | Free text | `""` |
| `number` | Integer or float | `0` |
| `date` | ISO 8601 date | `null` |
| `datetime` | ISO 8601 datetime | `null` |
| `boolean` | True/false | `false` |
| `enum` | One of a defined set | First value in `values` |
| `string_array` | Array of strings | `[]` |
| `object` | Nested structure | `{}` |

## Starter Templates

Three ship with Wyrd.

**task.jsonc:** `status`, `energy`, `contexts`, `people`, `due`.

**journal.jsonc:** `mood` (optional enum), `tags` (string_array).

**note.jsonc:** `tags` (string_array), `source_url` (optional string).

## Behaviour

When a type is added to a node, the template's fields merge in with defaults. When a type is removed, fields are hidden from the default view but never deleted from the file. Re-adding the type restores visibility.

Multiple types on a single node merge all their template fields. Field name collisions across templates are resolved by the first type in the `types` array taking precedence for defaults.
