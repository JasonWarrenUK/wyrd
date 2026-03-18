# ADR-009: Rituals as JSONC with Cypher Queries

**Status:** Accepted

## Context

Rituals (morning planning, weekly review, evening reflection) are recurring structured interactions. The step definitions need to reference data (inbox count, neglected projects, budget status) and branch conditionally.

Options: a mini-DSL for step sequencing, executable scripts, or structured JSONC with Cypher queries.

## Decision

Rituals are JSONC files containing an ordered array of typed steps. Data-dependent steps use Cypher queries for both display and conditions.

**Step types:**
- `query_summary`: runs a Cypher query returning scalars, renders into a template string.
- `query_list`: runs a Cypher query returning nodes, renders as a scrollable list with inline editing.
- `view`: renders a named saved view or built-in view.
- `action`: triggers a built-in system action (`propose_schedule`, `open_triage`, `open_journal`, `sync`, `rebuild_index`).
- `gate`: conditional step; runs a Cypher query returning a boolean, executes `then` step if true.
- `prompt`: free-text input that creates a node of a specified type.

**Friction levels:** `gate` (must complete or explicitly defer) and `nudge` (prominent suggestion, silently bypassable).

**Inline editing:** allowed on `query_list` steps. Rituals are for orientation, but the ability to act immediately (mark a project as paused, update a status) prevents the ritual from generating a secondary to-do list.

## Consequences

**Positive:** rituals compose existing capabilities (views, queries, actions) rather than introducing new primitives. Cypher queries mean ritual steps can express anything the query engine supports. Users define rituals entirely through config; no code required.

**Negative:** rituals depend on the Cypher engine; they can't be implemented until the parser exists. Complex ritual flows (branching, looping) aren't expressible; the step model is strictly sequential with conditional skips.
