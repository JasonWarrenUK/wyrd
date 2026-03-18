# Wyrd Documentation Suite

*v0.2 · March 2026*

## Introductions

| Doc | Audience | Purpose |
|-----|----------|---------|
| [01-intro-general](./01-intro-general.md) | Non-technical | What Wyrd does and why it exists |
| [02-intro-technical](./02-intro-technical.md) | Developers | Architecture, stack, data model |

## Architecture Decision Records

| ADR | Title | Status |
|-----|-------|--------|
| [001](./adr/adr-001-graph-as-flat-files.md) | Graph stored as flat JSONC files | Accepted |
| [002](./adr/adr-002-stack-selection.md) | Stack: Go, Bubble Tea, JSONC, Git | Accepted |
| [003](./adr/adr-003-cypher-subset.md) | Cypher subset as query language | Accepted |
| [004](./adr/adr-004-unified-plugin-architecture.md) | Unified plugin architecture | Accepted |
| [005](./adr/adr-005-conflict-resolution.md) | Three-way merge conflict resolution | Accepted |
| [006](./adr/adr-006-append-only-nodes.md) | Append-only nodes; archive, never delete | Accepted |
| [007](./adr/adr-007-theme-system.md) | Theme system built on Reasonable Colors | Accepted |
| [008](./adr/adr-008-envelope-budgeting.md) | Envelope budgeting without financial tracking | Accepted |
| [009](./adr/adr-009-ritual-definitions.md) | Rituals as JSONC with Cypher queries | Accepted |
| [010](./adr/adr-010-starter-templates.md) | Starter templates ship by default | Accepted |
| [011](./adr/adr-011-obsidian-integration.md) | Obsidian asymmetric sync | Accepted |
| [012](./adr/adr-012-first-class-writing.md) | Journals and notes as first-class citizens | Accepted |
| [013](./adr/adr-013-plugin-sync-strategy.md) | Plugin manifests sync; binaries per-machine | Accepted |

## File Format Guides

| Doc | Describes |
|-----|-----------|
| [Node schema](./formats/node-schema.md) | Base node fields and conditional type fields |
| [Edge schema](./formats/edge-schema.md) | Edge structure and built-in types |
| [Template format](./formats/template-format.md) | User-defined type templates |
| [Plugin manifest](./formats/plugin-manifest.md) | Plugin discovery, capabilities, config schema |
| [Theme format](./formats/theme-format.md) | Colour tiers, glyph definitions |
| [Ritual format](./formats/ritual-format.md) | Ritual steps, schedules, friction levels |
| [Saved view format](./formats/saved-view-format.md) | Cypher-based view definitions |
| [Budget envelope format](./formats/budget-envelope-format.md) | Spending allocation and logging |

## Aesthetic Design

| Doc | Covers |
|-----|--------|
| [Theme palettes](./aesthetics/theme-palettes.md) | All four themes with hex values and sources |
| [Design language](./aesthetics/design-language.md) | Glyphs, TUI layout, energy indicators, status signals |
