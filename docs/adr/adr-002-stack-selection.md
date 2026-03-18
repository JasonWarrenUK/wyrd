# ADR-002: Stack Selection

**Status:** Accepted

## Context

Wyrd is a terminal-first personal productivity tool. The stack must support: a rich TUI with split panes, vim-like navigation, and real-time updates; fast startup for a tool opened multiple times daily; a robust parser for the Cypher subset query language; and cross-platform operation.

## Decision

**TUI:** Go with Bubble Tea. The Elm architecture (Model-Update-View) imposes discipline on state management. The Bubbles component library and Lipgloss styling library provide the richest TUI ecosystem available. Proven at scale by lazygit, gh-dash, and similar tools.

**Data format:** JSONC (JSON with comments). One file per entity. Full JSON Schema validation. Human-annotatable via inline comments. Git-friendly diffs.

**Sync:** Git. No server, no account, no cloud dependency. History and branching for free. Works with existing tooling (GitHub, GitLab, bare repos).

**Future web dashboard:** SvelteKit. Not in any current build phase. Planned as a read/write companion once the data model stabilises.

## Consequences

**Positive:** Go compiles to a single binary with no runtime dependencies. Bubble Tea's architecture prevents the state management tangles common in TUI apps. JSONC's comment support is rare among data formats and genuinely useful for a human-editable store.

**Negative:** Go's type system is less expressive than TypeScript's for complex domain modelling. The Cypher parser must be built from scratch (no mature Go Cypher library for arbitrary in-memory graphs). Bubble Tea's Elm architecture has a learning curve if unfamiliar.
