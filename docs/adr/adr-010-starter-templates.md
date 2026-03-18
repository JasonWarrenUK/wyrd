# ADR-010: Starter Templates Ship by Default

**Status:** Accepted

## Context

The original spec stated "the system ships with nothing pre-defined except the base schema and built-in edge types." This was a design-time assumption, not a user requirement. Forcing users to define templates for task, journal, and note before they can write anything adds friction to first use.

## Decision

Starter templates for `task`, `journal`, and `note` ship with Wyrd. Users can modify or delete them.

This extends to other defaults: five built-in edge types (`blocks`, `parent`, `related`, `waiting_on`, `precedes`), a set of starter saved views, and example ritual definitions.

The principle is: useful defaults that the user owns. Wyrd provides a starting point; the user reshapes it.

## Consequences

**Positive:** first-run experience is functional immediately. New users can capture tasks, write journal entries, and take notes without configuration. The templates serve as documentation of the field schema by example.

**Negative:** users who want a fully custom setup must delete or modify the starters. Starter templates may calcify into assumptions if the codebase accidentally couples to their specific field names.
