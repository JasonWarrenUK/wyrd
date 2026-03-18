# ADR-003: Cypher Subset as Query Language

**Status:** Accepted

## Context

Saved views, rituals, the commitment tracker, neglect detection, budget views, and the morning schedule proposal all depend on querying the graph. Three options were evaluated.

**JSON filter spec:** zero parsing infrastructure. Queries are JSONC objects with property matchers and an `edges` clause for one-hop traversals. Trivially serialisable, but verbose and structurally mismatched (tree format querying graph data). Multi-hop traversals are inexpressible.

**Custom DSL:** purpose-built for Wyrd's data model. Readable syntax, graph-aware. But it requires designing a grammar, writing a parser, handling errors, writing documentation, and maintaining it indefinitely. Risk of accidental complexity as features accumulate.

**Cypher subset:** graph-native, well-documented, battle-tested syntax. Pattern matching with ASCII-art node/edge notation maps directly to the data model. Multi-hop traversals are first-class. Users familiar with Neo4j already know the syntax.

## Decision

Implement a read-only Cypher subset from day one.

**Included (Layers 1-3):**

Layer 1: `MATCH`, `WHERE`, `RETURN`, `ORDER BY`, `LIMIT`, `AS`. Property filters, boolean logic, comparison operators, date arithmetic. Variables `$today`, `$now`, `$week_start`, `$month_start` with `+Nd`/`-Nd` offsets.

Layer 2: Named edge traversal in both directions. Untyped edges. Variable-length paths `*1..N` with configurable ceiling (default 5).

Layer 3: `count()`, `sum()`, `avg()`, `min()`, `max()`, `collect()`. Implicit grouping by non-aggregated return fields.

**Deferred:** `WITH` (query chaining/piping), `OPTIONAL MATCH`, `UNWIND`, `CASE`, string functions, regex. All mutation operations (`CREATE`, `MERGE`, `DELETE`, `SET`); Wyrd mutates through its own commands.

Parser built with Go's `participle` library. Approximately 15 keywords, 6 aggregation functions, and node/edge pattern syntax. Estimated grammar: ~200 lines.

## Consequences

**Positive:** expressiveness covers every view and ritual query identified in the spec. Multi-hop traversals enable "everything connected to EPA within 3 hops." Existing documentation and community knowledge reduce the learning burden. The parser investment is front-loaded; adding query features later extends an existing grammar rather than building a new system.

**Negative:** implementing the parser is the most technically demanding single phase. Error messages for a hand-built parser are initially worse than JSON schema validation. Users unfamiliar with Cypher face a learning curve. The subset boundary must be enforced; scope creep toward full Cypher would be costly.
