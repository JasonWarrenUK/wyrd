# ADR-014: Default Asset Lifecycle

**Status:** Accepted

## Context

Wyrd ships two structurally different mechanisms for built-in defaults, and neither was a deliberate choice — they accreted independently as themes, templates, views, config, stage groups, and kinds were added.

**Copy-to-disk** (themes, templates, views, config): embedded in `internal/embed.StarterFS`, materialised onto disk once at `cli.Init`, overridden by editing the copied file, granularity is the whole file. Themes additionally fall back to a hardcoded `builtinTheme()` if the disk copy goes missing.

**In-binary plus shadow** (stage groups, kinds): never materialised; the embedded defaults (`stage/defaults.go`, `stage/kinds.go`) are merged at load time with a user file of the same shape, and a user entry with a matching name shadows the default (last-wins-by-name). Granularity is per-entry, and there is no on-disk fallback because there is no on-disk default to fall back to.

The two models were reasoned about independently task-by-task (SL.3, SL.10, SL.16/SL.17) with no shared vocabulary, which produced avoidable inconsistencies:

- `config.jsonc` was documented and implemented as living at the store root, but `ReadConfig`/`WriteConfig` read and wrote one directory up (fixed under TD.11 alongside this ADR; symmetric bug, so round-trip tests never caught it).
- `cli/init.go` and `store/store.go` keep two separate lists of store subdirectories, already out of sync on `archive/`.
- `builtinTheme()` duplicates `cairn.jsonc` by hand with no derivation link between the two.
- No starter rituals ship, despite ADR-010 claiming they do and `ensureDirs` creating `rituals/` for them.
- Neither model self-heals a deleted file. `ensureDirs` recreates directories on every `New()`, but `IsInitialised` only checks that `nodes/` exists, so a user who deletes `themes/cairn.jsonc` or a template loses it permanently with no repair path.
- `WriteKinds`/`WriteStages` overwrite their file wholesale, destroying any hand-added comments.
- Shadowing has an invisible fan-out: `stage.RenameStageGroup` writes shadow copies of every kind that referenced the renamed group, none of which the user consciously chose to fork (`rename.go:105-119`).

## Decision

Keep both mechanisms; they suit genuinely different asset shapes. **Copy-to-disk** fits assets a user is expected to hand-edit as a whole document (a theme's full palette, a template's field list). **In-binary-plus-shadow** fits assets that are one entry among many, where forcing a full-file copy for a one-field change would bury the user's actual edit in boilerplate they didn't write.

Resolve the inconsistencies above rather than introduce a third model:

1. `config.jsonc` lives at the store root — already fixed (TD.11/A1). Kinds and stage groups stay at the store's parent directory; that placement is documented and test-guarded, and unifying it with `config.jsonc` is out of scope here.
2. Unify the two store-directory lists into one source of truth that both `cli/init.go` and `store/store.go` read.
3. Derive `builtinTheme()` from `cairn.jsonc` at build time (or delete the hardcoded fallback and accept that a deleted theme file is a `cli init`-repairable state) rather than maintain two copies by hand.
4. Ship starter rituals or correct ADR-010 to say none ship yet — the claim and the code must agree.
5. TD.14's content-hash stamping (`ShadowOf`) is accepted as the right shape for shadow provenance specifically: it is cheap, additive, and needs no schema beyond one string field. It does not generalise to copy-to-disk assets, which have no equivalent "entry" granularity to hash.
6. Shadow fan-out (rename cascades writing shadows the user never asked for) is accepted as an inherent cost of per-entry shadowing, not a bug to fix here — it is named explicitly in Consequences so TD.5 designs its reconciliation UI around it rather than being surprised by it.

## Consequences

**Positive:** the two-model split now has a stated rationale (whole-document edits vs per-entry overrides) instead of being an artefact of implementation order. TD.14's `ShadowOf` hash gives TD.5 a comparison baseline without inventing a new asset-lifecycle mechanism. Naming the store-directory-list drift and the `config.jsonc` path bug in one place means future asset additions have a checklist to follow instead of repeating the same two mistakes.

**Negative:** copy-to-disk assets still have no repair path for accidental deletion short of `cli init` into a fresh store; fixing that is a separate task, not resolved by this ADR. Shadow fan-out remains real: renaming a stage group can silently fork several kinds' worth of defaults into permanent user-owned copies, and TD.5's reconciliation flow must treat those as first-class diverged entries, not edge cases. The two-model split itself is now a permanent asymmetry a contributor must learn — a unified model was considered and rejected as over-engineering for the one shape (stage groups, kinds) that currently needs per-entry granularity.
