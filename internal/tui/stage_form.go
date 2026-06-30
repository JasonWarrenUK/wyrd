package tui

import (
	"fmt"
	"strings"

	tea "charm.land/bubbletea/v2"
	huh "charm.land/huh/v2"
	"github.com/jasonwarrenuk/wyrd/internal/types"
)

// stageFormSubmitMsg is emitted by a stageFormPane when the user successfully
// creates a stage group. The group has already been written to stages.jsonc.
type stageFormSubmitMsg struct {
	name string
}

// stageFormPane wraps a huh.Form for creating user-defined stage groups. It
// satisfies both PaneModel and formActivePane. Unlike formPane, it does not
// create a new node — it writes a StageGroup to the user stage-group registry
// (stages.jsonc) in the store's parent directory.
//
// The form uses two huh groups: group 1 collects the name, stages, and cycle
// behaviour; group 2 (hidden unless cycle == loop-to-stage) collects the loop
// target from a dynamic select populated from the stages entered in group 1.
type stageFormPane struct {
	form  *huh.Form
	store types.StoreFS
	theme *ActiveTheme

	// Field values — written by huh via pointer accessors.
	name       string // unique stage group name
	stagesRaw  string // newline-separated stage names, entered by the user
	cycle      string // CycleBehaviour constant string
	loopTarget string // only used when cycle == loop-to-stage

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

// NewStageFormPane builds a stageFormPane. Exported for use in tests.
func NewStageFormPane(
	theme *ActiveTheme,
	store types.StoreFS,
	groups *types.StageGroupRegistry,
) PaneModel {
	return newStageFormPane(theme, store, groups)
}

// newStageFormPane builds a stageFormPane. groups is the merged stage-group
// registry (baked-in defaults + existing user groups); it is used to validate
// name collisions so the user cannot silently shadow a default or overwrite an
// existing custom group.
func newStageFormPane(
	theme *ActiveTheme,
	store types.StoreFS,
	groups *types.StageGroupRegistry,
) stageFormPane {
	f := stageFormPane{
		store: store,
		theme: theme,
		cycle: string(types.CycleTerminate), // sensible default
	}

	// Name validator: non-empty and not colliding with any existing group name.
	validateName := func(s string) error {
		s = strings.TrimSpace(s)
		if s == "" {
			return fmt.Errorf("name is required")
		}
		if groups != nil {
			if _, exists := groups.Lookup(s); exists {
				return fmt.Errorf("%q already exists — choose a different name", s)
			}
		}
		return nil
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

	// Group 1: name, stages (one per line), cycle behaviour.
	group1 := huh.NewGroup(
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
			return f, tea.Batch(cmd, func() tea.Msg { return formCancelMsg{} })
		}

		// Read existing user groups, append the new one, and write the full slice.
		var existing []types.StageGroup
		if reg, err := f.store.ReadStages(); err == nil {
			existing = reg.All()
		}
		existing = append(existing, group)
		if err := f.store.WriteStages(existing); err != nil {
			return f, tea.Batch(cmd, func() tea.Msg { return formCancelMsg{} })
		}

		name := group.Name
		return f, tea.Batch(cmd, func() tea.Msg {
			return stageFormSubmitMsg{name: name}
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
