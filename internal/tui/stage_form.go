package tui

import (
	"fmt"
	"strings"

	tea "charm.land/bubbletea/v2"
	huh "charm.land/huh/v2"
	"github.com/jasonwarrenuk/wyrd/internal/stage"
	"github.com/jasonwarrenuk/wyrd/internal/types"
)

// stageFormSubmitMsg is emitted by a stageFormPane when the user successfully
// creates or edits a stage group. The group has already been written to
// stages.jsonc. renamedFrom is non-empty when the submit renamed an existing
// group — the mount handler uses it to trigger the whole-registry rename
// cascade (stage.RenameStageGroup) before rebuilding the registries.
type stageFormSubmitMsg struct {
	name        string
	renamedFrom string
}

// stageFormErrorMsg is emitted when a stage group cannot be saved. The form
// closes and the reason is surfaced in the status bar; nothing is written.
type stageFormErrorMsg struct {
	err error
}

// stageFormPane wraps a huh.Form for creating or editing user-defined stage
// groups. It satisfies both PaneModel and formActivePane. Unlike formPane,
// it does not create a new node — it writes a StageGroup to the user
// stage-group registry (stages.jsonc) in the store's parent directory.
//
// The form uses two huh groups: group 1 collects the name, stages, and cycle
// behaviour; group 2 (hidden unless cycle == loop-to-stage) collects the loop
// target from a dynamic select populated from the stages entered in group 1.
// In edit mode, an optional leading Note (see kindFormPane's isDefault Note)
// is prepended to group 1 when the entry being edited shadows a built-in.
type stageFormPane struct {
	form  *huh.Form
	store types.StoreFS
	theme *ActiveTheme

	// groups is the merged registry, retained so the submit branch can run
	// the same collision check as the huh field validator — belt-and-braces
	// for a name that never passed through huh's own validation (mirrors
	// kindFormPane.kinds).
	groups *types.StageGroupRegistry

	// Field values — written by huh via pointer accessors.
	name       string // unique stage group name
	stagesRaw  string // newline-separated stage names, entered by the user
	cycle      string // CycleBehaviour constant string
	loopTarget string // only used when cycle == loop-to-stage

	// originalName is set in edit mode to the group's name at construction
	// time. Empty in create mode. Used at submit to decide replace-by-name
	// vs append (upsertStageGroup) and to detect a rename
	// (name != originalName).
	originalName string

	// isDefault is true when originalName matches a baked-in default group.
	// Drives the "overriding a built-in" warning Note.
	isDefault bool

	// editing mirrors the constructor's editing parameter — nil in create
	// mode, otherwise the entry being edited. Retained (rather than just
	// originalName) so the submit branch's belt-and-braces collision check
	// can reuse checkStageGroupNameCollision.
	editing *types.StageGroup

	width  int
	height int

	// done prevents double-emission of submit/cancel messages.
	done bool
}

// Compile-time checks: stageFormPane must satisfy PaneModel and formActivePane.
var _ PaneModel = stageFormPane{}
var _ formActivePane = stageFormPane{}

// isFormActive satisfies the formActivePane marker interface.
func (stageFormPane) isFormActive() {}

// NewStageFormPane builds a stageFormPane in create mode. Exported for use
// in tests.
func NewStageFormPane(
	theme *ActiveTheme,
	store types.StoreFS,
	groups *types.StageGroupRegistry,
) PaneModel {
	return newStageFormPane(theme, store, groups, nil)
}

// NewStageEditFormPane builds a stageFormPane in edit mode, pre-populated
// from existing. Exported for use in tests.
func NewStageEditFormPane(
	theme *ActiveTheme,
	store types.StoreFS,
	groups *types.StageGroupRegistry,
	existing types.StageGroup,
) PaneModel {
	return newStageFormPane(theme, store, groups, &existing)
}

// newStageFormPane builds a stageFormPane. groups is the merged stage-group
// registry (baked-in defaults + existing user groups); it is used to validate
// name collisions so the user cannot silently shadow a default or overwrite an
// existing custom group. editing is nil in create mode; when non-nil, the
// form seeds every field from *editing and switches submit from append to
// replace-by-name (see upsertStageGroup).
func newStageFormPane(
	theme *ActiveTheme,
	store types.StoreFS,
	groups *types.StageGroupRegistry,
	editing *types.StageGroup,
) stageFormPane {
	f := stageFormPane{
		store:  store,
		theme:  theme,
		groups: groups,
		cycle:  string(types.CycleTerminate), // sensible default
	}

	if editing != nil {
		// Seed before building the huh fields — huh binds by pointer, so the
		// fields below must be set to their final starting values first
		// (mirrors remap_form.go's f.choices[i] = orphan.Suggested ordering
		// and kindFormPane's identical seed-before-build sequencing).
		f.name = editing.Name
		f.originalName = editing.Name
		f.editing = editing
		f.stagesRaw = strings.Join(editing.Stages, "\n") // inverse of parseStages
		f.cycle = string(editing.Cycle)
		f.loopTarget = editing.LoopTarget

		if defaults, err := stage.DefaultStageGroups(); err == nil {
			for _, d := range defaults {
				if d.Name == editing.Name {
					f.isDefault = true
					break
				}
			}
		}
	}

	// Name validator: non-empty and not colliding (case-insensitively) with
	// any existing group name — "Active" and "active" would otherwise coexist
	// as visually confusable groups. In edit mode, the group's own current
	// name is exempted from the collision check (exact-match removal — see
	// excludeName's doc comment for why this must not be case-insensitive).
	validateName := func(s string) error {
		s = strings.TrimSpace(s)
		if s == "" {
			return fmt.Errorf("name is required")
		}
		return checkStageGroupNameCollision(s, groups, editing)
	}

	// Stages validator: at least one non-blank stage, no duplicates.
	validateStages := func(s string) error {
		parsed := parseStages(s)
		if len(parsed) == 0 {
			return fmt.Errorf("enter at least one stage")
		}
		seen := make(map[string]bool, len(parsed))
		for _, stage := range parsed {
			if seen[stage] {
				return fmt.Errorf("duplicate stage %q", stage)
			}
			seen[stage] = true
		}
		return nil
	}

	group1Fields := []huh.Field{}

	// Overriding-a-built-in warning: only shown when editing a baked-in
	// default. huh.Note defaults to skip:true (excluded from tab order), so
	// this costs no extra keypress — it's purely informational. Mirrors
	// kindFormPane's identical Note.
	if f.isDefault {
		group1Fields = append(group1Fields, huh.NewNote().
			Title("Overriding a built-in").
			Description(fmt.Sprintf(
				"%q is built in. Saving writes a full copy to your stages.jsonc "+
					"that permanently overrides it — including any future "+
					"improvements to the built-in version. Every kind still "+
					"pointing at the built-in version is shadowed too.", editing.Name)))
	}

	group1Fields = append(group1Fields,
		huh.NewInput().
			Title("Name").
			Description("Unique identifier for this stage group (e.g. review-flow)").
			Value(&f.name).
			Validate(validateName),

		huh.NewText().
			Title("Stages (one per line, in order)").
			Description("First stage is the starting stage when a kind is assigned.").
			Value(&f.stagesRaw).
			Lines(6).
			Placeholder("Open\nIn Progress\nDone").
			Validate(validateStages),

		huh.NewSelect[string]().
			Title("Cycle behaviour").
			Description("What happens at the last stage when advancing further.").
			Options(
				huh.NewOption("Terminate (stop at end)", string(types.CycleTerminate)),
				huh.NewOption("Loop (wrap to first stage)", string(types.CycleLoop)),
				huh.NewOption("Loop to stage (wrap to a chosen stage)", string(types.CycleLoopToStage)),
			).
			Value(&f.cycle),
	)

	// Group 1: name, stages (one per line), cycle behaviour.
	group1 := huh.NewGroup(group1Fields...)
	if editing != nil {
		group1 = group1.Title(fmt.Sprintf("Edit stage group %q", editing.Name))
	}

	// Group 2: loop target — shown only when cycle == loop-to-stage.
	// OptionsFunc re-evaluates the options whenever f.stagesRaw changes, so
	// the dropdown reflects whatever stages the user typed in group 1.
	group2 := huh.NewGroup(
		huh.NewSelect[string]().
			Title("Loop target").
			Description("The stage advancing past the last returns to.").
			OptionsFunc(func() []huh.Option[string] {
				stages := parseStages(f.stagesRaw)
				if len(stages) == 0 {
					return []huh.Option[string]{huh.NewOption("(no stages entered)", "")}
				}
				opts := make([]huh.Option[string], len(stages))
				for i, s := range stages {
					opts[i] = huh.NewOption(s, s)
				}
				return opts
			}, &f.stagesRaw).
			Value(&f.loopTarget),
	).WithHideFunc(func() bool {
		return f.cycle != string(types.CycleLoopToStage)
	})

	f.form = huh.NewForm(group1, group2).
		WithTheme(wyrdHuhTheme(theme)).
		WithShowHelp(true)

	return f
}

// parseStages splits a newline-separated string into a slice of trimmed,
// non-empty stage names, preserving order.
func parseStages(raw string) []string {
	lines := strings.Split(raw, "\n")
	out := make([]string, 0, len(lines))
	for _, l := range lines {
		if s := strings.TrimSpace(l); s != "" {
			out = append(out, s)
		}
	}
	return out
}

// Update forwards messages to the huh form and detects completion/abort.
func (f stageFormPane) Update(msg tea.Msg) (PaneModel, tea.Cmd) {
	if f.done {
		return f, nil
	}

	if wmsg, ok := msg.(tea.WindowSizeMsg); ok {
		f.width = wmsg.Width/2 - 2
		if f.width < 1 {
			f.width = 1
		}
		// Bound the form to the detail box's inner content height so huh scrolls
		// within the pane instead of overflowing past the status/capture bar.
		// status bar (2) + detail box borders (2) + outer logo box height.
		// Subtract 1 extra: huh v2 renders one line more than the height given.
		f.height = wmsg.Height - 4 - LogoHeight(f.width+2) - 1
		if f.height < 1 {
			f.height = 1
		}
		f.form = f.form.WithWidth(f.width).WithHeight(f.height)
	}

	model, cmd := f.form.Update(msg)
	if updated, ok := model.(*huh.Form); ok {
		f.form = updated
	}

	switch f.form.State {
	case huh.StateCompleted:
		f.done = true

		stages := parseStages(f.stagesRaw)
		group := types.StageGroup{
			Name:   strings.TrimSpace(f.name),
			Stages: stages,
			Cycle:  types.CycleBehaviour(f.cycle),
		}
		if group.Cycle == types.CycleLoopToStage {
			group.LoopTarget = f.loopTarget
		}

		// Final structural validation — belt-and-braces after huh field validators.
		if err := group.Validate(); err != nil {
			e := err
			return f, tea.Batch(cmd, func() tea.Msg { return stageFormErrorMsg{err: e} })
		}
		if err := checkStageGroupNameCollision(group.Name, f.groups, f.editing); err != nil {
			e := err
			return f, tea.Batch(cmd, func() tea.Msg { return stageFormErrorMsg{err: e} })
		}

		// Read existing user groups. A failed read is treated as fatal rather
		// than silently falling back to an empty slice — WriteStages overwrites
		// the whole file, so proceeding on an empty base would destroy every
		// existing user group. Surface the error and abort the write.
		reg, err := f.store.ReadStages()
		if err != nil {
			e := fmt.Errorf("reading existing stage groups: %w", err)
			return f, tea.Batch(cmd, func() tea.Msg { return stageFormErrorMsg{err: e} })
		}

		renamed := f.originalName != "" && group.Name != f.originalName
		existing := upsertStageGroup(reg.All(), group, f.originalName)

		// Renaming a shadowed default (or a not-yet-shadowed one) would
		// otherwise un-shadow the built-in group under its old name — see
		// kindFormPane's identical tombstone reasoning. RenameStageGroup
		// (run after this write succeeds, in the mount handler) repoints
		// every kind off the old group name, so the tombstone is inert once
		// that cascade lands — it exists purely to keep the group registry
		// from resurrecting the default.
		if renamed && f.isDefault {
			if defaults, derr := stage.DefaultStageGroups(); derr == nil {
				for _, d := range defaults {
					if d.Name == f.originalName {
						existing = append(existing, d)
						break
					}
				}
			}
		}

		if err := f.store.WriteStages(existing); err != nil {
			e := err
			return f, tea.Batch(cmd, func() tea.Msg { return stageFormErrorMsg{err: e} })
		}

		name := group.Name
		renamedFrom := ""
		if renamed {
			renamedFrom = f.originalName
		}
		return f, tea.Batch(cmd, func() tea.Msg {
			return stageFormSubmitMsg{name: name, renamedFrom: renamedFrom}
		})

	case huh.StateAborted:
		f.done = true
		return f, tea.Batch(cmd, func() tea.Msg { return formCancelMsg{} })
	}

	return f, cmd
}

// View renders the huh form, padded to the pane width and repainted so the
// primary background extends through every interior cell. PadLines squares
// each line to f.width; FillBackground repaints the backgroundless padding
// cells emitted inside the bubbles viewport and huh's field separator.
func (f stageFormPane) View() string {
	content := strings.TrimRight(f.form.View(), "\n")
	if content == "" {
		content = "Submitting…"
	}
	bg := f.theme.BgPrimary()
	return FillBackground(PadLines(content, f.width, bg), bg)
}

// KeyBindings returns the help hints shown in the status bar.
func (f stageFormPane) KeyBindings() []KeyBinding {
	return []KeyBinding{
		{Key: "tab / shift+tab", Description: "Next / previous field"},
		{Key: "enter", Description: "Next field (submit on last)"},
		{Key: "ctrl+c", Description: "Cancel form"},
	}
}

// HandleFocusLost is a no-op for stage form panes.
func (f stageFormPane) HandleFocusLost() tea.Cmd { return nil }

// checkStageGroupNameCollision applies the same collision rule the huh field
// validator uses, callable as a belt-and-braces check at submit time —
// mirrors checkKindNameCollision in kind_form.go; see that function's doc
// comment for why both the huh validator and a submit-time check are needed,
// and for the case-only-rename-must-check-before-exemption reasoning.
func checkStageGroupNameCollision(name string, groups *types.StageGroupRegistry, editing *types.StageGroup) error {
	if editing != nil && name != editing.Name && strings.EqualFold(name, editing.Name) {
		return fmt.Errorf("changing only the capitalisation of %q is not supported", editing.Name)
	}
	if groups == nil {
		return nil
	}
	names := groups.Names()
	if editing != nil {
		names = excludeName(names, editing.Name)
	}
	if !caseInsensitiveNameCollision(name, names) {
		return nil
	}
	return fmt.Errorf("%q already exists — choose a different name", name)
}

// upsertStageGroup returns entries with the group matching originalName
// replaced in place, or group appended when no entry matches. originalName
// is "" in create mode, which always appends. Matching is exact — the
// registry keys entries by exact name. Mirrors upsertKind in kind_form.go;
// see that function's doc comment for the full reasoning (replace-in-place
// preserves display order; "not found" naturally covers both create and
// shadowing a not-yet-overridden default with no special-casing needed).
func upsertStageGroup(entries []types.StageGroup, group types.StageGroup, originalName string) []types.StageGroup {
	if originalName != "" {
		for i, e := range entries {
			if e.Name == originalName {
				out := make([]types.StageGroup, len(entries))
				copy(out, entries)
				out[i] = group
				return out
			}
		}
	}
	return append(entries, group)
}
