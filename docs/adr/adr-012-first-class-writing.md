# ADR-012: Journals and Notes as First-Class Citizens

**Status:** Accepted

## Context

The original spec treated journals and notes as "just nodes with a type." Technically correct, but the interaction patterns differ from tasks: journals are time-anchored prose written once; notes are reference material refined over time. Both require editor-based input rather than one-line capture, and both need dedicated reading views.

## Decision

Journals and notes receive dedicated capture commands, viewing modes, and starter templates alongside tasks.

**Capture:** `wyrd journal` opens `$EDITOR` with today's date pre-filled, creates a journal-typed node on save. `wyrd note "title"` opens `$EDITOR` for the body, creates a note-typed node. Both support `--link <node>` for creating edges during capture.

**Viewing:** task views show list-style columns (status, energy, due). Journal views show reverse-chronological full-text. Note views show title, body, and edge list (wiki-style; what's connected to this note?). Each is a different rendering of the same saved-view query.

**Templates:** starter templates for all three types ship with Wyrd.

## Consequences

**Positive:** writing in Wyrd is frictionless from first use. The journal command is fast enough to replace a separate journalling habit. Notes accumulate graph connections over time, becoming more valuable as the graph grows. All three content types are queryable, linkable, and visible in rituals.

**Negative:** `$EDITOR` integration means the writing experience is only as good as the user's editor. The TUI will eventually need an inline editor for quick edits, but v1 relies on external editors.
