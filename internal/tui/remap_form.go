package tui

import (
	"fmt"
	"strings"

	tea "charm.land/bubbletea/v2"
	huh "charm.land/huh/v2"
	"github.com/jasonwarrenuk/wyrd/internal/stage"
	"github.com/jasonwarrenuk/wyrd/internal/types"
)

// unchangedSentinel is the option value meaning "leave this orphan alone".
// Matches ApplyRemap's contract: an empty target string is skipped.
const unchangedSentinel = ""

// maxRemapOrphans caps how many distinct (kind, stage) orphans a single
// remap form will present. Beyond this, something is structurally wrong
// with the stage config rather than a handful of stray nodes needing a
// decision — direct the user to fix stages.jsonc/kinds.jsonc instead of
// rendering a form with more fields than fit any reasonable terminal.
const maxRemapOrphans = 20

// remapFormSubmitMsg is emitted by a remapFormPane after a successful
// ApplyRemap. remapped is the number of nodes rewritten; unchanged is the
// number of orphaned nodes left as-is (the user chose the sentinel, or
// never made a choice because the field defaulted and was left alone).
type remapFormSubmitMsg struct {
	remapped  int
	unchanged int
}

// remapFormErrorMsg is emitted when ApplyRemap reports a write failure. The
// form closes; whatever writes succeeded before the failure remain applied
// (ApplyRemap does not roll back partial progress — see its doc comment).
type remapFormErrorMsg struct {
	err error
}

// remapFormPane wraps a huh.Form presenting one select field per orphaned
// (kind, stage) pair from a stage.OrphanReport. Unlike stageFormPane and
// kindFormPane, it never touches kinds.jsonc/stages.jsonc — it rewrites node
// Stage fields via stage.ApplyRemap, which routes through store.UpdateNode
// (the same write path handleStageShift uses) so the in-memory index stays
// live.
type remapFormPane struct {
	form  *huh.Form
	store types.StoreFS
	theme *ActiveTheme

	report  stage.OrphanReport
	choices []string // one per report.Orphans entry, aligned by index

	width  int
	height int

	// done prevents double-emission of submit/error messages.
	done bool
}

// Compile-time checks: remapFormPane must satisfy PaneModel and formActivePane.
var _ PaneModel = remapFormPane{}
var _ formActivePane = remapFormPane{}

// isFormActive satisfies the formActivePane marker interface.
func (remapFormPane) isFormActive() {}

// NewRemapFormPane builds a remapFormPane. Exported for use in tests.
// Callers must ensure report is non-empty and within maxRemapOrphans —
// app.go enforces both before mounting.
func NewRemapFormPane(theme *ActiveTheme, store types.StoreFS, report stage.OrphanReport) PaneModel {
	return newRemapFormPane(theme, store, report)
}

// newRemapFormPane builds one huh.NewSelect per orphan in report.Orphans
// order. Each select offers the orphan's current group's stages plus a
// "leave unchanged" sentinel, defaulting to the orphan's suggested target.
func newRemapFormPane(theme *ActiveTheme, store types.StoreFS, report stage.OrphanReport) remapFormPane {
	f := remapFormPane{
		store:   store,
		theme:   theme,
		report:  report,
		choices: make([]string, len(report.Orphans)),
	}

	fields := make([]huh.Field, 0, len(report.Orphans))
	for i, orphan := range report.Orphans {
		f.choices[i] = orphan.Suggested

		opts := make([]huh.Option[string], 0, len(orphan.Group.Stages)+1)
		for _, s := range orphan.Group.Stages {
			opts = append(opts, huh.NewOption(s, s))
		}
		opts = append(opts, huh.NewOption("— leave unchanged —", unchangedSentinel))

		title := fmt.Sprintf("%s · %q (%d node%s)", orphan.Kind, orphan.Stage, len(orphan.NodeIDs), plural(len(orphan.NodeIDs)))
		desc := fmt.Sprintf("Not present in %s. Choose a replacement.", orphan.Group.Name)

		fields = append(fields, huh.NewSelect[string]().
			Title(title).
			Description(desc).
			Options(opts...).
			Value(&f.choices[i]),
		)
	}

	titleText := "Remap orphaned stages"
	if n := len(report.Unresolvable); n > 0 {
		titleText = fmt.Sprintf("Remap orphaned stages (%d node%s unresolvable — kind or group missing, cannot be remapped)", n, plural(n))
	}

	f.form = huh.NewForm(
		huh.NewGroup(fields...).Title(titleText),
	).WithTheme(wyrdHuhTheme(theme)).WithShowHelp(true)

	return f
}

// plural returns "s" when n != 1, matching the convention used for status
// bar messages elsewhere in the package.
func plural(n int) string {
	if n == 1 {
		return ""
	}
	return "s"
}

// Update forwards messages to the huh form and detects completion/abort.
func (f remapFormPane) Update(msg tea.Msg) (PaneModel, tea.Cmd) {
	if f.done {
		return f, nil
	}

	if wmsg, ok := msg.(tea.WindowSizeMsg); ok {
		f.width = wmsg.Width/2 - 2
		if f.width < 1 {
			f.width = 1
		}
		// Mirrors stageFormPane's sizing: bound to the detail box's inner
		// content height so huh scrolls within the pane rather than
		// overflowing past the status/capture bar. The -1 accounts for huh
		// v2 rendering one line more than the height given.
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

		choices := make(map[stage.OrphanKey]string, len(f.report.Orphans))
		for i, orphan := range f.report.Orphans {
			choices[stage.OrphanKey{Kind: orphan.Kind, Stage: orphan.Stage}] = f.choices[i]
		}

		remapped, err := stage.ApplyRemap(f.store, f.report, choices, false)
		if err != nil {
			e := err
			return f, tea.Batch(cmd, func() tea.Msg { return remapFormErrorMsg{err: e} })
		}

		unchanged := f.report.NodeCount() - remapped
		return f, tea.Batch(cmd, func() tea.Msg {
			return remapFormSubmitMsg{remapped: remapped, unchanged: unchanged}
		})

	case huh.StateAborted:
		f.done = true
		return f, tea.Batch(cmd, func() tea.Msg { return formCancelMsg{} })
	}

	return f, cmd
}

// View renders the huh form, padded to the pane width and repainted so the
// primary background extends through every interior cell.
func (f remapFormPane) View() string {
	content := strings.TrimRight(f.form.View(), "\n")
	if content == "" {
		content = "Submitting…"
	}
	bg := f.theme.BgPrimary()
	return FillBackground(PadLines(content, f.width, bg), bg)
}

// KeyBindings returns the help hints shown in the status bar.
func (f remapFormPane) KeyBindings() []KeyBinding {
	return []KeyBinding{
		{Key: "tab / shift+tab", Description: "Next / previous field"},
		{Key: "enter", Description: "Next field (submit on last)"},
		{Key: "ctrl+c", Description: "Cancel form"},
	}
}

// HandleFocusLost is a no-op for remap form panes.
func (f remapFormPane) HandleFocusLost() tea.Cmd { return nil }
