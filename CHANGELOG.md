<!-- doc-changelog: generated 2026-09-15. Delete this line once you hand-edit this file. -->
# Changelog

All notable changes to Wyrd are documented in this file. The format follows [Keep a Changelog](https://keepachangelog.com/en/1.1.0/).

## [Unreleased]

## [0.16.0] - 2026-09-15

### Added

- `:kinds delete <name>` and `:stages delete <name>` commands: delete a custom kind or stage group, or revert a shadowed default back to its built-in form. A purely custom entry offers deletion (blocked if any node would be left with an unresolvable kind/stage); a shadowed default offers "revert to default" instead, restoring the embedded default through the existing merge.
- Shadow snapshots (`ShadowSource`) are stamped automatically whenever a kind or stage group is edited from a form, capturing the pre-edit default so later combine/revert operations have real data to work from.

## [0.15.0] - 2026-08-25

### Added

- Budget display view in the TUI, showing envelope spend against budget for the selected period.

## [0.14.0] - 2026-08-25

### Added

- Schedule display view in the TUI, showing scheduled and due nodes ahead of the current date.

<details>
<summary>0.13.0 and earlier</summary>

## [0.13.0] - 2026-08-25

### Added

- `:stages remap` gains a documented spike (TD.18a) resolving how shadow-tracked defaults will support a future three-way combine between a user's edit, the old default and the new default.

## [0.12.0] - 2026-08-11

### Fixed

- Status bar clock now uses the injected clock instead of the system clock, fixing time display in tests and anywhere else a stub clock is used.
- Divergence between a user's kind/stage edits and the shipped defaults now surfaces as an alert instead of silently going unnoticed.
- `wyrd view` command now actually runs the named view instead of no-op.

## [0.11.0] - 2026-08-07

### Added

- Edit existing kinds and stage groups directly from the TUI (`:kinds edit`, `:stages edit`), rather than hand-editing `kinds.jsonc`/`stages.jsonc`.

### Fixed

- Node editing no longer overwrites the original creation timestamp.
- Config and node timestamps consolidated onto a consistent JSONC representation, removing a class of round-trip inconsistencies.
- Overlay routing (forms, ritual runner, palette) fixed to stop misdirected key input between overlays.

## [0.10.0] - 2026-08-07

### Added

- Automatic stage-remap engine: when a stage group's stages change, nodes holding an orphaned stage combination are offered a guided remap instead of being silently stranded.

## [0.9.0] - 2026-08-05

### Added

- Create new kinds directly from the TUI via `:kinds new`, without hand-editing `kinds.jsonc`.

## [0.8.0] - 2026-08-05

### Added

- Movement node data model: the budget engine gains a movement-based representation (`draws_from`/`adds_to` edges) as groundwork for retiring the embedded spend log.

## [0.7.2] - 2026-08-04

### Fixed

- Small roadmap-driven fixes and leak patches across the TUI and CLI surfaced during a documentation audit.

## [0.7.1] - 2026-08-04

### Fixed

- `wyrd spend` no longer mutates the live in-memory node before writing, closing a data race against concurrent readers and an index/disk divergence on write failure.
- Spend entries dated in a future period are now correctly excluded from the current period's total instead of being counted early.
- Plugin manifest filename unified on `plugin.jsonc` across install, list and discovery, so an installed plugin actually shows up in `wyrd plugin list`. Zip extraction now rejects path-escaping ("zip-slip") entries.
- Fixed a background-colour bleed in the empty right-hand pane that appeared on startup, after closing a form and during async loads.
- Custom dashboard columns configured on a saved view no longer revert to the default columns after the first capture, edit, archive, spend or stage change.

## [0.7.0] - 2026-07-20

### Changed

- Focus-transition animation tuned: border colour now cross-dissolves per-cell instead of interpolating between fixed gradient stops, removing a visible desync on some themes. Pane width no longer shifts when focus changes; only the border colour animates.
- Wordmark now renders with an accent gradient sweep instead of a flat colour.

### Fixed

- Kiln theme's red glaze accent swapped for plum to stop clashing with the terracotta base.
- Group header rows in the node list are now padded to full width, fixing a background-bleed bar on light themes.

## [0.6.0] - 2026-07-19

### Added

- Pane focus changes now animate with a spring-eased colour transition, and a `reduce_motion` config toggle turns the animation off entirely for accessibility.

## [0.5.0] - 2026-07-19

### Fixed

- Stale-node glyphs in the node list no longer misalign the row layout, and list truncation now measures by display width instead of character count, fixing overflow with wide glyphs and fullwidth characters.

## [0.4.0] - 2026-07-16

### Added

- Blocked nodes now show a `BLOCKED` badge and a `BLOCKED BY` section in the detail pane, and a blocked-glyph prefix in list rows, wherever a node has one or more unresolved blockers.

## [0.3.0] - 2026-07-16

### Added

- `n.isBlocked` computed query property: derives whether a node is blocked from its incoming `blocks` edges and each blocker's stage terminality, available in queries, views and the TUI without any extra wiring per node.

## [0.2.0] - 2026-07-13

### Changed

- Budget-related capture-bar prefixes renamed for consistency: `s:` becomes `bs:` (spend entries) and `b:` becomes `bc:` (budget categories), freeing the `b*:` namespace for future budget prefixes.

### Added

- Capture-bar sync-failure messages can now be dismissed with Esc instead of persisting until the next action.

## [0.1.0] - 2026-07-04

Initial release.

### Added

- Flat-file property graph store: nodes and edges as JSONC files on disk, with an in-memory index kept live via a filesystem watcher.
- Cypher-inspired read-only query engine (`wyrd query`, `wyrd view`), supporting `UNION`/`UNION ALL` and built-in date variables (`$today`, `$now`, `$week_start`, `$month_start` with arithmetic offsets).
- CLI commands: `init`, `add`, `journal`, `note`, `spend`, `query`, `view`, `sync`, `plugin`.
- Terminal UI (`wyrd` with no arguments): split-pane layout with a node list, scrollable detail pane, fuzzy filtering, command palette and status bar.
- Interactive capture forms (task, journal, note, spend) with theme-aware styling, optional linking to the currently selected node, and configurable body field sizes.
- Node type badges, a configurable dashboard backed by a saved view, and grouped list sections.
- Git-backed sync (`wyrd sync`): stages changes, generates a commit message, pulls with rebase and pushes, with a custom JSONC three-way merge driver for conflict-free node/edge merges.
- Four starter themes (including the default Cairn palette).
- Plugin system: manifest-driven install and discovery.

</details>

[Unreleased]: https://github.com/JasonWarrenUK/wyrd/compare/v0.16.0...HEAD
[0.16.0]: https://github.com/JasonWarrenUK/wyrd/compare/v0.15.0...v0.16.0
[0.15.0]: https://github.com/JasonWarrenUK/wyrd/compare/v0.14.0...v0.15.0
[0.14.0]: https://github.com/JasonWarrenUK/wyrd/compare/v0.13.0...v0.14.0
[0.13.0]: https://github.com/JasonWarrenUK/wyrd/compare/v0.12.0...v0.13.0
[0.12.0]: https://github.com/JasonWarrenUK/wyrd/compare/v0.11.0...v0.12.0
[0.11.0]: https://github.com/JasonWarrenUK/wyrd/compare/v0.10.0...v0.11.0
[0.10.0]: https://github.com/JasonWarrenUK/wyrd/compare/v0.9.0...v0.10.0
[0.9.0]: https://github.com/JasonWarrenUK/wyrd/compare/v0.8.0...v0.9.0
[0.8.0]: https://github.com/JasonWarrenUK/wyrd/compare/v0.7.2...v0.8.0
[0.7.2]: https://github.com/JasonWarrenUK/wyrd/compare/v0.7.1...v0.7.2
[0.7.1]: https://github.com/JasonWarrenUK/wyrd/compare/v0.7.0...v0.7.1
[0.7.0]: https://github.com/JasonWarrenUK/wyrd/compare/v0.6.0...v0.7.0
[0.6.0]: https://github.com/JasonWarrenUK/wyrd/compare/v0.5.0...v0.6.0
[0.5.0]: https://github.com/JasonWarrenUK/wyrd/compare/v0.4.0...v0.5.0
[0.4.0]: https://github.com/JasonWarrenUK/wyrd/compare/v0.3.0...v0.4.0
[0.3.0]: https://github.com/JasonWarrenUK/wyrd/compare/v0.2.0...v0.3.0
[0.2.0]: https://github.com/JasonWarrenUK/wyrd/compare/v0.1.0...v0.2.0
[0.1.0]: https://github.com/JasonWarrenUK/wyrd/releases/tag/v0.1.0
