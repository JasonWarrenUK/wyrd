---
description: Wyrd feature roadmap — status lattice, node type expansion, backlog triage, skeins, plus all incomplete tasks carried over from tui.md.
---

# Wyrd: Feature Roadmap

> [!NOTE]
> Capture prefixes were renamed under CP.15 (`s:` → `bs:`, `b:` → `bc:`); the code now uses `bs:` and `bc:`. The `bm:` bookmark prefix arrives with NW.1.

---

## Contents

- [Milestone 3: Capture & Forms](#m3)
- [Milestone 5: Logging & Observability](#m5)
- [Milestone 6: Rituals & Workflows](#m6)
- [Milestone 7: Documentation Assets](#m7)
- [Milestone 8: Compaction](#m8)
- [Milestone G: Spend Depth](#mg)
- [Milestone A: Status Lattice](#ma)
- [Milestone B: Node Types Expansion](#mb)
- [Milestone C: Backlog](#mc)
- [Milestone D: Skeins](#md)
- [Milestone E: Tech Debt](#me)
- [Milestone F: Visual Polish](#mf)
- [Dependency Diagram](#diagram)

---

## Milestone 3: Capture & Forms {#m3}

**Goal:** All node creation flows use `huh` forms inline in the TUI. The capture bar prefix syntax triggers the appropriate form.

- [x] **CP.17** — Dismissable capture-bar messages — added a sticky/transient distinction on `StatusBar`'s capture message: `SetCaptureText` failures (e.g. "Sync failed: …") are now sticky and only cleared by `esc`, while success/info messages still auto-clear after 2s. Fixed a latent race where a stale clear-tick could wipe a newer message before its own timer fired, by adding a generation counter and routing the 8 previously copy-pasted tick literals in `app.go` through one guarded helper. `internal/tui/statusbar.go`, `internal/tui/app.go`
- [x] **CP.15** — Rename capture prefixes — `s:` → `bs:` (spend) and `b:` → `bc:` (budget category) in `parseCapturePrefixes`, the capture hint text, doc comments in `form.go`/`spend_form.go`/`spend_form_test.go`, and the `"s: coffee beans"` literal in `TestCaptureBar_SpendPrefix` (`capture_test.go`), so budget-related prefixes group under `b*`
- [x] **CP.16** — Fix edit-mode node data loss — `(formPane).buildNode` now starts from a `Node.Clone()` of the original node (stashed on the `formPane` by the edit constructors) and overwrites only form-owned fields, so budget `spend_log`, `Date` sub-fields, journal `About`, kind/stage, source, and custom/plugin `Properties` all survive edits. `Clone` added to `types.Node`. Also fixed in passing: `handleEditNode` had no `budget` case (budget nodes fell through to the task edit form, rewriting them as tasks — now wired to `newEditBudgetFormPane`); "Warn at" gained `Validate` (optional, [0–1], blank defaults to 1.0 — the warn_at default changed from 0.8 to 1.0 across form, CLI, and budget view); "Allocated" now accepts 0
- [x] **CP.14** — Budget creation form — `huh`-based form with fields for category name, allocation amount, period select, warn threshold; creates a budget-type node. Shipped on the `b:` prefix; renamed to `bc:` under CP.15 _(depends on CP.13)_
- [x] **CP.13** — Add `budget.jsonc` starter template
- [x] **CP.11** — Edge management in edit form
- [x] **CP.10** — Edit existing node — `ctrl+o` opens pre-populated huh form
- [x] **CP.9** — Allow node creation without linking
- [x] **CP.8** — Wire capture bar focus to open appropriate form based on prefix
- [x] **CP.7** — Spend entry form (`bs:` prefix; formerly `s:`) — delegates to `budget.RecordSpend`
- [x] **CP.6** — Wire link-to-selected on form submit
- [x] **CP.5** — Configure huh textarea in all three forms
- [x] **CP.4** — Note creation form (`n:` prefix)
- [x] **CP.3** — Journal entry form (`j:` prefix)
- [x] **CP.2** — Task creation form (`t:` prefix)
- [x] **CP.1** — Add `charm.land/huh/v2` dependency
- [x] **CP.0** — Wire capture bar

---

## Milestone 5: Logging & Observability {#m5}

**Goal:** Structured logging via `charmbracelet/log` throughout the app. Debug output to log file, never stdout.

- [x] **LG.7** — Add TUI debug overlay (`:log` command in palette) that tails `wyrd.log` in a viewport _(depends on LG.2)_
- [x] **LG.6** — Thread logger through query engine _(depends on LG.2)_
- [x] **LG.5** — Thread logger through sync _(depends on LG.2)_
- [x] **LG.4** — Thread logger through store operations _(depends on LG.2)_
- [x] **LG.3** — Add `--log-level` flag and `WYRD_LOG_LEVEL` env var _(depends on LG.2)_
- [x] **LG.2** — Initialise logger in `main.go`; write to `~/.wyrd/wyrd.log` _(depends on LG.1)_
- [x] **LG.1** — Add `github.com/charmbracelet/log` dependency

---

## Milestone 6: Rituals & Workflows {#m6}

**Goal:** The ritual runner is wired into the TUI. Scheduled rituals trigger on startup. Step sequencing, gate prompts, and deferral are interactive and fluid.

- [ ] **RT.6** — Persist ritual deferral timestamp — the `Esc Esc d` defer sequence and in-session `StateDeferred` are done, but nothing is written to disk; deferrals (and per-day dismissals, currently in-memory in `SchedulerState`) should survive a restart _(depends on RT.5)_
- [ ] **RT.7** — Action step execution — the step type is parsed and rendered but all v1 actions are stubbed (`internal/tui/ritual/runner.go`); implement real actions _(depends on RT.2)_
- [ ] **RT.8** — Palette ritual command — `:ritual <name>` launches a ritual on demand _(depends on RT.2)_
- [x] **RT.5** — Gate step _(depends on RT.2)_
- [x] **RT.4** — Prompt steps — implemented with a `bubbles` textinput rather than huh; submission writes the node and edge _(depends on CP.1, RT.2)_
- [x] **RT.3** — Query steps in ritual — `query_summary` and `query_list` _(depends on RT.2)_
- [x] **RT.2** — Mount ritual runner in a full-screen overlay pane
- [x] **RT.1** — Ritual scheduler on startup _(depends on CP.1)_

---

## Milestone 7: Documentation Assets {#m7}

**Goal:** README and docs include polished screenshots (via `freeze`) and animated gifs (via `vhs`) showing the TUI in action.

- [ ] **DA.1** — Install `freeze` and `vhs` (via Homebrew or Go install); document in README prerequisites
- [ ] **DA.2** — Capture freeze screenshot of main TUI view (node list + detail pane) for README hero _(blocked — depends on CP.15, DA.1, DL.2, DL.5, NW.2, SL.11, SL.12, SL.14, SL.7c, VP.1, VP.2, VP.3, VP.5, VP.6, VP.7, VP.8, MA, MB, MC, MF)_
- [ ] **DA.3** — Capture freeze screenshot of budget view with progress bars _(blocked — depends on DA.1, SP.11, SP.4, SP.6, SP.9, VP.1, VP.2, VP.3, VP.5, VP.6, VP.7, VP.8, MF, MG)_
- [ ] **DA.4** — Capture freeze screenshot of schedule view _(blocked — depends on DA.1, VP.1, VP.2, VP.3, VP.5, VP.6, VP.7, VP.8, MF)_
- [ ] **DA.5** — Write VHS tape for task creation flow (capture bar → huh form → node appears in list) _(blocked — depends on CP.2, DA.1, SL.11, SL.12, SL.14, SL.7c, VP.1, VP.2, VP.3, VP.5, VP.6, VP.7, VP.8, MA, MF)_
- [ ] **DA.6** — Write VHS tape for ritual run (startup prompt → steps → gate → completion) _(blocked — depends on DA.1, RT.5, RT.6, RT.7, RT.8, VP.1, VP.2, VP.3, VP.5, VP.6, VP.7, VP.8, M6, MF)_
- [ ] **DA.7** — Write VHS tape for `wyrd sync` (stage → commit → push with animated spinner) _(blocked — depends on DA.1, VP.1, VP.2, VP.3, VP.5, VP.6, VP.7, VP.8, MF)_
- [ ] **DA.8** — Integrate screenshots and gifs into README.md under a "Screenshots" section _(blocked — depends on DA.2, DA.3, DA.4)_
- [ ] **DA.9** — Store VHS tapes in `docs/vhs/` directory; add make target `make demo` to regenerate all gifs _(blocked — depends on DA.5, DA.6, DA.7)_

---

## Milestone 8: Compaction {#m8}

**Goal:** `wyrd compact` moves archived nodes to `archive/` and handles orphaned edges. A `--dry-run` flag shows what would be moved.

- [ ] **CO.2** — `wyrd compact` — orphan edge handling: detach or archive edges linked to archived nodes _(depends on CO.1)_
- [ ] **CO.3** — TUI compaction — `:compact` palette command; shows dry-run preview in an overlay, confirm executes, reports moved/detached counts _(blocked — depends on CO.2)_
- [x] **CO.1** — `wyrd compact` — move archived nodes to `archive/` directory with `--dry-run` flag

---

## Milestone G: Spend Depth {#mg}

**Goal:** Money movements are first-class nodes (kind `movement`, stage group `expected → cleared`) linked to budgets via `draws_from`/`adds_to` edges. Spends, income, and transfers are edge-topology variants of one model: `draws_from` only = spend; `adds_to` only = income; both = transfer. Bottom-up budgeting derives the envelope from expected movements. A movement is a dated event, not an abstract relationship — hence node plus edges rather than a payload edge.

- [ ] **SP.7** — Movement node data model — register a `movement` kind in `stage.DefaultKinds`; new baked-in `movement-flow` stage group (`expected → cleared`, terminate); add `draws_from` and `adds_to` to the built-in edge types; movement nodes carry the amount in `Properties`, the transaction date in `Date.About`, and the note in `Body` _(depends on SL.3, SL.5)_
- [ ] **SP.2** — Bottom-up budgets — effective allocation = sum of stage-`expected` movements drawing from the category in the upcoming period _(blocked — depends on SP.8)_
- [ ] **SP.4** — Surface bottom-up allocation in TUI — budget detail pane and progress bars use the effective allocation; derived allocations visually distinguished from explicitly set ones _(blocked — depends on SP.2, SP.3)_
- [ ] **SP.5** — Income recording — `wyrd income` CLI subcommand creates a movement node with an `adds_to` edge (mirrors `wyrd spend`); the previous `Direction`-field design is superseded by edge topology _(blocked — depends on SP.8)_
- [ ] **SP.6** — TUI income capture form — `bi:` capture-bar prefix opens a huh movement form (amount, source/note, date); delegates to the SP.5 income path; creates a node, so it carries the SL.7 form pattern _(blocked — depends on SL.7b, SP.5)_
- [ ] **SP.8** — Budget engine over movements — `RecordSpend` creates a movement node plus a `draws_from` edge to the budget instead of appending to `spend_log`; `Compute` derives spent from cleared movements in the current period (net = draws_from − adds_to); the embedded `spend_log` representation is deleted outright — the dual-shape handling in `budget.SpendLog` and the CP.16 spend_log-preservation tests retire with it (pre-production, no migration) _(blocked — depends on SP.7)_
- [ ] **SP.9** — Budget detail pane lists movements — rework SP.3's spend-events section to read movement nodes via edges: amount, date, stage, and counterpart category for transfers _(blocked — depends on SP.8)_
- [ ] **SP.10** — Transfer recording — `wyrd transfer` CLI creates a single movement node with both a `draws_from` and an `adds_to` edge; the unbalanced-transfer state is unrepresentable by construction _(blocked — depends on SP.8)_
- [ ] **SP.11** — TUI transfer capture form — `bt:` capture-bar prefix opens a huh form (from-category, to-category, amount, date); delegates to the SP.10 transfer path _(blocked — depends on SL.7b, SP.10)_
- [x] **SP.3** — Spend events in budget detail pane
- [x] **SP.1** — Dated spend entries — `SpendEntry.Date` across `RecordSpend`, `SpendOptions`, `--date` CLI flag, TUI spend form

---

## Milestone A: Status Lattice {#ma}

**Goal:** Nodes have a `kind` and a `stage`. Stage groups define named progressions. The TUI advances/retreats stage with a keypress. The lattice is fully user-configurable via `kinds.jsonc`.

- [ ] **SL.10** — Create kinds in TUI — `:kind new` palette command opens a huh form (name, glyph, colour, stage group select); writes to `kinds.jsonc` _(depends on SL.4)_
- [ ] **SL.14** — Stage remap on group reassignment — when a kind's stage group changes (via SL.10 kind edit) or a group's stage list is edited in place (via SL.11), existing nodes of that kind may hold a stage absent from the new group; a remap prompt asks the user to map each orphaned stage to a target stage in the new group (default: name-match if one exists, else the group's first stage); nodes are rewritten via `UpdateNode` (the SL.6 stage-write path); until remapped, orphaned stages leave nodes untouched (`StageGroup.Next`/`Prev` already return `ok==false` for unknown stages) _(blocked — depends on CP.16, SL.10, SL.11, SL.13, SL.6)_
- [x] **SL.12** — Stage group view in TUI — bare `:stages` palette command opens a read-only modal overlay listing every stage group (baked-in and user-defined); each row shows the group name, a `(custom)` provenance marker for user-defined groups, the cycle behaviour (`terminate` / `loop ↺` / `loop→<target> ↺`), and the full ordered stage progression (`A → B → C`); scrollable viewport, `esc`/`q` closes; `stagesOverlay` struct in `internal/tui/stages_overlay.go` mirroring `kindsOverlay`; composited via `compositeOverlay`; registry refreshed in-session after `:stages new` submits _(depends on SL.3)_
- [x] **SL.11** — Create stage groups in TUI — `:stages new` palette command opens a two-group `huh` form: group 1 collects name (validated against the merged registry to prevent collision), ordered stages (one per line, `huh.NewText`), and cycle behaviour select; group 2 (hidden unless `loop-to-stage`) offers a loop-target select whose options are dynamically populated from the stages entered in group 1 via `huh.Select.OptionsFunc`; on submit, the form reads existing user groups via `store.ReadStages()`, appends the new `types.StageGroup`, and writes the full slice via a new `store.WriteStages([]types.StageGroup)` (mirrors `WriteConfig`); the in-memory registry is rebuilt in-session by re-merging via `stage.MergeStageGroups`, reassigning `m.stageGroups` and `m.kindsOverlay.stageGroups`; `StoreFS` interface extended with `WriteStages`; 6 test-mock stubs updated; `stage_form.go` is a new non-node form pane modelled on `spend_form.go`; status-bar confirmation with 2s auto-clear; `parseStages` helper; `NewStageFormPane` exported for tests _(depends on SL.13)_
- [x] **SL.13** — User stage-group registry — `stages.jsonc` in the store's parent directory (sibling of `config.jsonc`) holds user-defined stage groups, loaded at startup and merged with the baked-in defaults (user groups shadow defaults of the same name via `MergeStageGroups`); `StageGroup.Validate` added to `internal/types/stage.go` (non-empty name, ≥1 stage, `loop-to-stage` requires a valid `loop_target`); `(*Store).ReadStages()` in `internal/store/store.go` mirrors `ReadKinds` (missing file → empty registry, lenient per-entry skip, whole-file failure → `ParseError`); `StoreFS` interface extended with `ReadStages`; 6 test-mock stubs updated; `main.go` now calls `s.ReadStages()` non-fatally and passes user groups to `MergeStageGroups`; `ResolveStageGroup` and all TUI consumers required no changes — the merged registry was already threaded through _(depends on SL.3)_
- [x] **SL.7c** — TUI: kind selection in edit forms — all four `NewEdit*FormPane` constructors accept `kinds`/`stageGroups` registries and pre-populate `nodeKind` from `node.Kind`; a "— none —" sentinel option preserves the untriaged state; `applyKindStage` helper unifies create and edit stamping (replaces four copy-pasted inline blocks); CP.16 invariant: unchanged kind returns early leaving Kind/Stage untouched; changed kind re-stamps Kind and keeps Stage if the new group `Contains` it, else resets to the group's first stage; `StageGroup.Contains` exported (wraps `indexOf`); registries threaded through `handleEditNode` in `app.go` _(depends on CP.16, SL.7b)_
- [x] **SL.7b** — TUI: kind selection in remaining create forms — `newJournalFormPane`, `newNoteFormPane`, and `newBudgetFormPane` now accept the kind/stage registries, render a `huh.NewSelect` kind field (defaults Journal/Note/Budget respectively), and stamp `node.Kind`/`node.Stage` on create (nil-safe, CP.16 clone invariant preserved); three new baked-in kinds added (Journal→content-flow, Note→content-flow, Budget→budget-flow) plus a new `budget-flow` stage group `[Active, Closed]` (terminate); kind/group counts now 10/6; registries threaded through the three capture-bar dispatch sites in `app.go` _(depends on SL.7a)_
- [x] **SL.7a** — TUI: kind selection in task create form — `newTaskFormPane` accepts `*types.KindRegistry` and `*types.StageGroupRegistry`; a `huh.NewSelect` kind field (options from `kinds.Names()`, default Task) is inserted after Body and before Status; `buildNode` stamps `node.Kind` and initialises `node.Stage` to `group.Stages[0]` on create only (nil-safe; edit mode preserves the clone's existing Kind/Stage via the `f.originalNode == nil` guard, keeping the CP.16 invariant); both registries threaded through the capture-bar dispatch in `app.go` _(depends on CP.16, SL.6)_
- [x] **SL.9** — Kind registry view in TUI — `:kinds` palette command lists registered kinds with glyph, colour, and stage group; `kindsOverlay` struct in `internal/tui/kinds_overlay.go`; inline row format (coloured glyph + kind name + stage-group name + ordered stages, loop groups marked `↺`); composited as a centred Lipgloss layer; nil-registry safe _(depends on SL.15)_
- [x] **SL.6** — TUI: advance stage (`]`) and retreat stage (`[`) keypresses on selected node; wraps per kind's cycle behaviour; refreshes dashboard and detail pane inline (mirrors `handleEditSubmit`; the codebase uses inline refresh rather than a `nodeUpdatedMsg` message type); `handleStageShift` in `internal/tui/app.go`; routes writes through `StoreFS.UpdateNode` (new interface method) to keep the in-memory index live; `types.StageGroupRegistry`, `types.ResolveStageGroup`, `stage.MergeStageGroups` added; registry threaded through `tui.Config.StageGroups` _(depends on SL.15)_
- [x] **SL.15** — TUI: show `Kind` and `Stage` in the detail pane — a kind/stage line renders immediately after the type badges, resolving the node's kind against the merged registry (`DetailRenderer.Kinds`, threaded from `m.kinds`) for glyph and colour; falls back to a plain muted stage string when the kind is empty or unresolved, and omits the line entirely when both fields are empty. `renderKindStageLine` in `internal/tui/detail.go`; nil-registry safe _(depends on SL.5)_
- [x] **SL.5** — Ship default kinds: Task, Goblin, Habit, Event, Travel, Talk, Project — each referencing appropriate stage group (Task/Goblin/Talk → task-flow; Event/Travel → event-flow; Habit → habit-flow; Project → project-flow); two new baked-in stage groups added (habit-flow: loop, project-flow: terminate); `stage.DefaultKinds()` and `stage.MergeKinds()` in `internal/stage/kinds.go`; starter template kind implications recorded as JSONC comments; merged registry threaded through `tui.Config.Kinds` at startup; Bookmark kind deferred to NW.1 _(depends on SL.4)_
- [x] **SL.4** — Add `kinds.jsonc` config file — `types.Kind` struct (name, stage-group ref, glyph, colour) in `internal/types/kind.go`; `KindRegistry` with `Lookup`/`All`/`Names` and last-wins-by-name merge seam for SL.5; `(*Store).ReadKinds()` in `internal/store/` reads `~/wyrd/kinds.jsonc` (store parent, sibling of `config.jsonc`); missing file yields empty registry; individual invalid entries skipped (lenient); `StoreFS` interface extended; 6 test-mock stubs updated _(depends on SL.3)_
- [x] **SL.3** — Ship three baked-in default stage groups as embedded JSONC: `task-flow` (Open→Maybe→Later→Soon→Now→Done), `event-flow` (Scheduled→Now→Finished), `content-flow` (Active→Reference) _(depends on SL.2)_
- [x] **SL.2** — Define stage group data model — `StageGroup` struct (`Name`, ordered `Stages`, `Cycle`, `LoopTarget`) and `CycleBehaviour` type with three constants: `loop` (wrap to first), `terminate` (stay at end, idempotent), `loop-to-stage` (wrap to a named stage, falling back to first if the target is missing). `Next`/`Prev` advance and retreat honouring cycle behaviour at both boundaries — `Prev` wraps to the last stage for both looping modes (the symmetric inverse of advancing off the end). `(stage, ok)` return: `ok == false` means an unknown stage so callers leave the node untouched. `IsTerminal` reports the no-advance-possible stage for DL.1's blocking check. Pure data model in `internal/types/stage.go`; no I/O _(depends on SL.1)_
- [x] **SL.8b** — TUI grouping for kind/stage — `detectGroupCol` recognises `kind` and `stage` columns alongside `category` (alias-triggered, e.g. `RETURN n.kind AS kind`); `toGroupLabel` is column-aware, pluralising kind values like categories and title-casing stage values without a plural (`now` → `Now`). The grouping/render machinery was already generic. No live view drives kind/stage grouping yet — that lands with SL.6/SL.7 _(depends on SL.8)_
- [x] **SL.8** — Query engine: `n.kind` and `n.stage` as first-class queryable properties in WHERE, RETURN, and ORDER BY; the index needed no changes (it stores whole nodes). Pre-lattice nodes return `""` for both, so `WHERE n.kind = ""` finds untriaged nodes. NV.12 grouping split out as SL.8b _(depends on SL.1)_
- [x] **SL.1** — Add `kind` and `stage` fields to Node struct and store serialisation; existing nodes lack both fields, so loading defaults them to empty (back-compat, no migration step); empty fields are omitted on write so legacy files are unchanged on rewrite

---

## Milestone B: Node Types Expansion {#mb}

**Goal:** Bookmark node type and `answers` edge type are first-class, wired into capture and the detail pane.

- [ ] **NW.1** — Add `bookmark` node type with `url` property; `bm:` capture prefix triggers a form (url required, title optional; the form carries the SL.7 kind select from birth). `bm:` does not collide with `bc:`/`bs:` (prefix matching is exact); registering bookmark as a default kind folds into SL.5 _(depends on SL.7b)_
- [ ] **NW.2** — Add `answers` edge type; wire into edge management form (CP.11 done); detail pane renders linked answers under an ANSWERS section _(blocked — depends on NW.1, SL.8)_

---

## Milestone C: Backlog {#mc}

**Goal:** Blocked status is derived from the edge graph. Stale nodes get a visual indicator. The dashboard activates a backlog triage sweep when the active list reaches a calmness threshold.

- [x] **DL.1** — Derive `isBlocked` at query time from `blocks` edges — a node is blocked if any node pointing to it via a `blocks` edge has stage != terminal; expose as `n.isBlocked` computed property in the query engine. Terminality comes from the stage group model (`StageGroup.IsTerminal`, SL.2) _(depends on DL.6, SL.2, SL.8)_
- [ ] **DL.3** — Staleness indicator — compute days since `date.modified`; left pane shows a muted badge on nodes idle > configurable threshold (default 14d); staleness needs nothing from the status lattice
- [ ] **DL.2** — TUI: blocked badge on list items where `n.isBlocked` is true; detail pane shows BLOCKED BY section listing blocking nodes _(depends on DL.1)_
- [ ] **DL.4** — Backlog triage query — surfaces M highest-priority backlog items (low stages, highest staleness) plus one serendipitous pick; implemented as a saved view. Stage-based ranking needs queryable stages _(blocked — depends on DL.3, SL.8)_
- [ ] **DL.5** — Dashboard calmness threshold — when active-stage node count drops below configurable N, dashboard automatically appends backlog triage results as a separate section; N and M configurable in `config.jsonc` _(blocked — depends on DL.4)_
- [x] **DL.6** — Thread Kind + StageGroup registries into the query engine so it can resolve stage terminality — add a `WithStageResolver(kinds, groups)` EngineOption (nil-safe, matching the existing `WithLogger` pattern in `internal/query/engine.go`); update the two CLI query paths (`queryCmd`, `viewCmd` in `cmd/wyrd/main.go`) to build and pass the registries, matching the TUI path, which already builds them. Enables stage-terminality resolution for computed properties like `isBlocked` (DL.1). An unresolvable blocker (empty stage, unknown kind, or no registries wired) is treated as still blocking (presence blocks), surfaced distinctly from a confirmed non-terminal block rather than silently.

---

## Milestone D: Skeins {#md}

**Goal:** Reusable named Cypher fragments stored in `store/skeins/` can be referenced by name inside view files and composed into full queries.

- [ ] **SK.1** — Define skein data model — a named partial Cypher fragment (WHERE clause, ORDER BY, or RETURN projection); stored as JSONC in `store/skeins/`
- [ ] **SK.2** — Store: read/write skeins via StoreFS; expose via GraphIndex as `GetSkein(name)` and `ListSkeins()`; extend the fsnotify watcher (currently `nodes/` and `edges/` only) to cover `skeins/`; sync commit messages should describe skein changes _(blocked — depends on SK.1)_
- [ ] **SK.3** — Query engine: resolve skein references at parse time — interpolated into the containing query before evaluation; circular references are a parse error _(blocked — depends on SK.2)_
- [ ] **SK.4** — TUI: skein management via palette — `:skein list`, `:skein new`, `:skein edit <name>`; edit opens a huh text form _(blocked — depends on SK.3)_

---

## Milestone E: Tech Debt {#me}

**Goal:** Internal infrastructure cleaned up: JSONC parsing consolidated into a single shared package; default-asset lifecycle documented and consistent across the codebase.

- [ ] **TD.1** — Consolidate JSONC parsing — four duplicated `stripComments` scanners exist across `internal/store/jsonc.go`, `internal/tui/theme.go`, `internal/tui/views/loader.go`, and `internal/tui/ritual/loader.go` (SL.3 adds a fourth in `internal/stage/`); only the store variant strips trailing commas; extract into a shared `internal/jsonc` package, repoint all consumers, add a trailing-comma test
- [ ] **TD.2** — ADR: unify default-asset lifecycle — themes ship as embedded starter-copy plus an in-Go fallback; templates/views/config are starter-copy only; stage groups (SL.3) are in-binary only; document which assets should be user-editable-on-disk vs code-owned-in-binary, decide whether any lifecycle should change, record the decision as an ADR in `docs/` _(depends on SL.3)_
- [ ] **TD.3** — Edge `Modified` timestamp — restructure `types.Edge` date properties into an embedded `DateFields`-style block holding the existing `Created` plus a new `Modified`; store write paths stamp `Modified` on every edge update; serialisation changes freely (pre-production, no back-compat constraint). Implementation question to settle: whether `Node`'s top-level `Created`/`Modified` should move into its date block for symmetry
- [x] **TD.4** — `gofmt` cleanup — originally scoped to `cmd/wyrd/main.go` (import-ordering drift plus a trailing-space alignment nit on the `query` command's `Args:` field), but `gofmt -l .` surfaced the same class of drift (struct-field/map-key/const-block alignment, import ordering) across 30 files repo-wide; ran `gofmt -w .` for the full sweep instead of the single-file fix. Whitespace/import-ordering only, no behavioural change

---

## Milestone F: Visual Polish {#mf}

**Goal:** The TUI looks and feels coherent — in-app branding, theme-consistent forms and markdown, considered focus affordances, and honest progress feedback. Visual polish only; no data-model changes. Task ideas VP.2–VP.8 are drawn from a survey of the Charm stack (bubbletea, bubbles, lipgloss, huh, harmonica, glamour). Soft-serve was assessed for `wyrd sync` and deliberately excluded: sync is already a generic git client, so a soft-serve remote needs zero code and adds no UX gain — at most a future docs how-to, not a polish task.

- [ ] **VP.1** — Logo/title pane atop the detail column — split the right column vertically into a fixed-height logo/title pane (top) and the existing detail pane (below). Add a `wyrd` wordmark asset (none exists today); rework `layout.go` `Render` to stack the logo box and detail box with `JoinVertical`, add a height calc reserving the logo's rows, and wire a `logoPane` alongside `rightPane` in `app.go`. Must honour the background-bleed rules (`PadLines`, both fg+bg on every style)
- [ ] **VP.3** — Theme-derived glamour stylesheet — build a glamour `ansi.StyleConfig` from theme colours (headings → accent, code → muted bg, links → accent-secondary) so rendered markdown in the detail pane is visually continuous with its container
- [ ] **VP.4** — Gradient focus border — replace the flat accent border on the focused pane with a subtle `BorderForegroundBlend` gradient (accent → accent-secondary, both already in theme) so focus is unmistakable
- [ ] **VP.6** — Spring-eased pane focus transition — animate the focus-border colour fade (and optional 1-col width nudge) over ~150ms via harmonica `Spring.Update` driven by `tea.Tick`, instead of a hard snap; gate all motion behind a `reduce_motion` config toggle for accessibility. Sequenced after VP.4 because both touch the focused-border render path in `layout.go` _(blocked — depends on VP.4)_
- [ ] **VP.7** — Auto-generated key-hint footer — replace the hand-rolled `keyHints` in `statusbar.go` with the bubbles `help` component generating short/full help from the `key.Binding` set, with a `?`-toggle full view. Sequenced after CP.17 because both rework the statusbar surface and the footer must restore hints after a message dismissal _(depends on CP.17)_
- [ ] **VP.8** — Stepped sync progress bar — `wyrd sync` shows an indeterminate MiniDot today; the git phases (stage → commit → pull → push) are discrete, so drive a determinate bubbles `progress` bar with phase labels from phased messages emitted by `internal/sync`; the bar's terminal failure state is displayed through the CP.17 dismissable-message mechanism _(depends on CP.17)_
- [x] **VP.5** — Floating modal overlays via compositor — overlays are composited via `lipgloss.Place` + `Layer`/`Compositor`, floating over the main frame and centred horizontally and vertically, with content-driven height rather than a fixed clamp; the log, help, and kinds overlays inherit correct sizing from the VP.9 height work _(depends on VP.9)_
- [x] **VP.9** — Fix overlay panel overflow — resolved as part of VP.5: content-driven viewport height clamping was added to all three overlays (`log_overlay.go`, `help_overlay.go`, `kinds_overlay.go`), so they no longer extend past the bottom of the visible TUI
- [x] **VP.2** — Wyrd-themed `huh` forms — `wyrdHuhTheme` fully derives from `ActiveTheme`: all focused/blurred/multi-select/button/help-footer styles carry the Cairn palette and `BgPrimary` on every style to prevent background bleed; the `Blurred` block is set explicitly (huh's `ThemeCharm` copies `Focused → Blurred` before our overrides run, so each field must be set in both blocks) _(depends on SL.7c)_

---

## Dependency Diagram {#diagram}

```mermaid
graph LR
	classDef done fill:#c3e6cb,stroke:#1e7e34
	classDef open fill:#d4edda,stroke:#28a745
	classDef blocked fill:#f8d7da,stroke:#dc3545
	classDef paused fill:#e2e3f3,stroke:#5a6ab0,stroke-dasharray:4 3
	classDef deferred fill:#e2e3e5,stroke:#6c757d,stroke-dasharray:2 4,font-style:italic
	classDef external fill:#fff3cd,stroke:#d39e00,stroke-dasharray:4 3,font-style:italic
	classDef mile fill:#cce5ff,stroke:#004085,font-weight:bold

	M3[M3: Capture & Forms]:::mile
	M5[M5: Logging & Observability]:::mile
	M6[M6: Rituals & Workflows]:::mile
	M7[M7: Documentation Assets]:::mile
	M8[M8: Compaction]:::mile
	MG[MG: Spend Depth]:::mile
	MA[MA: Status Lattice]:::mile
	MB[MB: Node Types Expansion]:::mile
	MC[MC: Backlog]:::mile
	MD[MD: Skeins]:::mile
	ME[ME: Tech Debt]:::mile
	MF[MF: Visual Polish]:::mile

	%% Milestone 3: Capture & Forms
	CP13 --> CP14
	CP0 --> M3
	CP1 --> M3
	CP10 --> M3
	CP11 --> M3
	CP14 --> M3
	CP15 --> M3
	CP16 --> M3
	CP17 --> M3
	CP2 --> M3
	CP3 --> M3
	CP4 --> M3
	CP5 --> M3
	CP6 --> M3
	CP7 --> M3
	CP8 --> M3
	CP9 --> M3

	%% Milestone 5: Logging & Observability
	LG2 --> LG7
	LG2 --> LG6
	LG2 --> LG5
	LG2 --> LG4
	LG2 --> LG3
	LG1 --> LG2
	LG3 --> M5
	LG4 --> M5
	LG5 --> M5
	LG6 --> M5
	LG7 --> M5

	%% Milestone 6: Rituals & Workflows
	RT5 --> RT6
	RT2 --> RT7
	RT2 --> RT8
	RT2 --> RT5
	CP1 --> RT4
	RT2 --> RT4
	RT2 --> RT3
	CP1 --> RT1
	RT1 --> M6
	RT3 --> M6
	RT4 --> M6
	RT6 --> M6
	RT7 --> M6
	RT8 --> M6

	%% Milestone 7: Documentation Assets
	CP15 --> DA2
	DA1 --> DA2
	DL2 --> DA2
	DL5 --> DA2
	NW2 --> DA2
	SL11 --> DA2
	SL12 --> DA2
	SL14 --> DA2
	SL7c --> DA2
	VP1 --> DA2
	VP2 --> DA2
	VP3 --> DA2
	VP5 --> DA2
	VP6 --> DA2
	VP7 --> DA2
	VP8 --> DA2
	DA1 --> DA3
	SP11 --> DA3
	SP4 --> DA3
	SP6 --> DA3
	SP9 --> DA3
	VP1 --> DA3
	VP2 --> DA3
	VP3 --> DA3
	VP5 --> DA3
	VP6 --> DA3
	VP7 --> DA3
	VP8 --> DA3
	DA1 --> DA4
	VP1 --> DA4
	VP2 --> DA4
	VP3 --> DA4
	VP5 --> DA4
	VP6 --> DA4
	VP7 --> DA4
	VP8 --> DA4
	CP2 --> DA5
	DA1 --> DA5
	SL11 --> DA5
	SL12 --> DA5
	SL14 --> DA5
	SL7c --> DA5
	VP1 --> DA5
	VP2 --> DA5
	VP3 --> DA5
	VP5 --> DA5
	VP6 --> DA5
	VP7 --> DA5
	VP8 --> DA5
	DA1 --> DA6
	RT5 --> DA6
	RT6 --> DA6
	RT7 --> DA6
	RT8 --> DA6
	VP1 --> DA6
	VP2 --> DA6
	VP3 --> DA6
	VP5 --> DA6
	VP6 --> DA6
	VP7 --> DA6
	VP8 --> DA6
	DA1 --> DA7
	VP1 --> DA7
	VP2 --> DA7
	VP3 --> DA7
	VP5 --> DA7
	VP6 --> DA7
	VP7 --> DA7
	VP8 --> DA7
	DA2 --> DA8
	DA3 --> DA8
	DA4 --> DA8
	DA5 --> DA9
	DA6 --> DA9
	DA7 --> DA9
	MA --> DA2
	MB --> DA2
	MC --> DA2
	MF --> DA2
	MF --> DA3
	MG --> DA3
	MF --> DA4
	MA --> DA5
	MF --> DA5
	M6 --> DA6
	MF --> DA6
	MF --> DA7
	DA8 --> M7
	DA9 --> M7

	%% Milestone 8: Compaction
	CO1 --> CO2
	CO2 --> CO3
	CO3 --> M8

	%% Milestone G: Spend Depth
	SL3 --> SP7
	SL5 --> SP7
	SP8 --> SP2
	SP2 --> SP4
	SP3 --> SP4
	SP8 --> SP5
	SL7b --> SP6
	SP5 --> SP6
	SP7 --> SP8
	SP8 --> SP9
	SP8 --> SP10
	SL7b --> SP11
	SP10 --> SP11
	SP1 --> MG
	SP11 --> MG
	SP4 --> MG
	SP6 --> MG
	SP9 --> MG

	%% Milestone A: Status Lattice
	SL4 --> SL10
	CP16 --> SL14
	SL10 --> SL14
	SL11 --> SL14
	SL13 --> SL14
	SL6 --> SL14
	SL3 --> SL12
	SL13 --> SL11
	SL3 --> SL13
	CP16 --> SL7c
	SL7b --> SL7c
	SL7a --> SL7b
	CP16 --> SL7a
	SL6 --> SL7a
	SL15 --> SL9
	SL15 --> SL6
	SL5 --> SL15
	SL4 --> SL5
	SL3 --> SL4
	SL2 --> SL3
	SL1 --> SL2
	SL8 --> SL8b
	SL1 --> SL8
	SL12 --> MA
	SL14 --> MA
	SL7c --> MA
	SL8b --> MA
	SL9 --> MA

	%% Milestone B: Node Types Expansion
	SL7b --> NW1
	NW1 --> NW2
	SL8 --> NW2
	NW2 --> MB

	%% Milestone C: Backlog
	DL6 --> DL1
	SL2 --> DL1
	SL8 --> DL1
	DL1 --> DL2
	DL3 --> DL4
	SL8 --> DL4
	DL4 --> DL5
	DL2 --> MC
	DL5 --> MC

	%% Milestone D: Skeins
	SK1 --> SK2
	SK2 --> SK3
	SK3 --> SK4
	SK4 --> MD

	%% Milestone E: Tech Debt
	SL3 --> TD2
	TD1 --> ME
	TD2 --> ME
	TD3 --> ME
	TD4 --> ME

	%% Milestone F: Visual Polish
	VP4 --> VP6
	CP17 --> VP7
	CP17 --> VP8
	VP9 --> VP5
	SL7c --> VP2
	VP1 --> MF
	VP2 --> MF
	VP3 --> MF
	VP5 --> MF
	VP6 --> MF
	VP7 --> MF
	VP8 --> MF

	class CO2 & DA1 & DL3 & DL6 & NW1 & RT6 & RT7 & RT8 & SK1 & SL10 & SP7 & TD1 & TD2 & TD3 & VP1 & VP3 & VP4 & VP7 & VP8 open
	class CO3 & DA2 & DA3 & DA4 & DA5 & DA6 & DA7 & DA8 & DA9 & DL1 & DL2 & DL4 & DL5 & NW2 & SK2 & SK3 & SK4 & SL14 & SP2 & SP4 & SP5 & SP6 & SP8 & SP9 & SP10 & SP11 & VP6 blocked
	class CO1 & CP0 & CP1 & CP2 & CP3 & CP4 & CP5 & CP6 & CP7 & CP8 & CP9 & CP10 & CP11 & CP13 & CP14 & CP15 & CP16 & CP17 & LG1 & LG2 & LG3 & LG4 & LG5 & LG6 & LG7 & RT1 & RT2 & RT3 & RT4 & RT5 & SL1 & SL2 & SL3 & SL4 & SL5 & SL6 & SL7a & SL7b & SL7c & SL8 & SL8b & SL9 & SL11 & SL12 & SL13 & SL15 & SP1 & SP3 & TD4 & VP2 & VP5 & VP9 done
```
