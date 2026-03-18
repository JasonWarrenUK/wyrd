# ADR-011: Obsidian Asymmetric Sync

**Status:** Accepted

## Context

Obsidian is a primary writing tool. Its vault (markdown files with YAML frontmatter and wikilinks) overlaps with Wyrd's note and journal nodes. Full bidirectional sync would create conflicts over which tool is authoritative and where edits happen.

## Decision

Asymmetric sync with direction determined by content type.

**Journals (push-only, automatic):** when a journal node is created or modified in Wyrd, it is written as a markdown file to a configured Obsidian vault folder. Frontmatter carries the Wyrd node ID, types, date, and linked node titles. Wyrd does not read changes made to these files in Obsidian. Wyrd is the writing surface; Obsidian is the archive.

**Notes (manual, both directions):** `wyrd push <node-id>` writes a note to a configured Obsidian folder. `wyrd pull obsidian --tag "wyrd" --search "keyword"` searches the vault by frontmatter tag, filename, or content and creates/updates note nodes in Wyrd. Wikilinks become candidate `related` edges (confirmed during triage). Tags map to an array property.

Both flows are implemented as an Obsidian plugin with `["sync", "action"]` capabilities.

## Consequences

**Positive:** eliminates the "where do I write?" decision. Journals always start in Wyrd. Notes flow in whichever direction is needed. Obsidian's strong editor is available for long-form note writing; Wyrd's graph handles the connections.

**Negative:** journal edits in Obsidian are silently ignored by Wyrd (push-only). Users must remember that the Obsidian copy is a downstream mirror, not an editable source. Note sync is manual and could drift if the user forgets to pull.
