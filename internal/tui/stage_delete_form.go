package tui

import (
	"fmt"
	"strings"

	tea "charm.land/bubbletea/v2"
	huh "charm.land/huh/v2"
	"github.com/jasonwarrenuk/wyrd/internal/stage"
	"github.com/jasonwarrenuk/wyrd/internal/types"
)

// stageDeleteMode is stageDeleteFormPane's twin of kindDeleteMode.
type stageDeleteMode int

const (
	// stageDeleteRevert covers both a tombstone and an ordinary shadowed
	// default: drop the user entry so the embedded default resurfaces.
	// Stage groups have no outward reference (unlike a kind's StageGroup
	// field), so unlike kinds there is no fan-out hazard to guard against
	// here — see the kindDeleteRevert doc comment for that hazard.
	stageDeleteRevert stageDeleteMode = iota

	// stageDeleteCustom is a purely user-authored group no kind references
	// — a plain delete.
	stageDeleteCustom

	// stageDeleteRepoint is a purely user-authored group still referenced
	// by one or more kinds — those kinds must be repointed at another
	// group before the delete can proceed, or every node of every
	// referencing kind becomes Unresolvable.
	stageDeleteRepoint
)

// stageDeleteSubmitMsg is emitted after a successful delete/revert/repoint.
type stageDeleteSubmitMsg struct {
	name      string
	mode      stageDeleteMode
	repointed int
}

// stageDeleteErrorMsg is emitted when the write (or the repoint that
// precedes it) fails.
type stageDeleteErrorMsg struct {
	err error
}

// stageDeleteFormPane is kindDeleteFormPane's twin for stage groups.
type stageDeleteFormPane struct {
	form  *huh.Form
	store types.StoreFS
	theme *ActiveTheme

	group      types.StageGroup
	mode       stageDeleteMode
	referenced []string // kind names referencing this group, for kindDeleteRepoint's copy

	target  string
	confirm bool

	width  int
	height int

	done bool
}

var _ PaneModel = stageDeleteFormPane{}
var _ formActivePane = stageDeleteFormPane{}
var _ formMountable = stageDeleteFormPane{}

func (stageDeleteFormPane) isFormActive() {}

func (f stageDeleteFormPane) initForm() tea.Cmd {
	return f.form.Init()
}

// newStageDeleteFormPane builds a stageDeleteFormPane. groups is the
// current registry, used to populate the repoint target select — nil for
// modes that don't need one.
func newStageDeleteFormPane(theme *ActiveTheme, store types.StoreFS, group types.StageGroup, mode stageDeleteMode, referenced []string, groups *types.StageGroupRegistry) stageDeleteFormPane {
	f := stageDeleteFormPane{
		store:      store,
		theme:      theme,
		group:      group,
		mode:       mode,
		referenced: referenced,
	}

	var title, desc, affirmative string
	fields := make([]huh.Field, 0, 3)

	switch mode {
	case stageDeleteRevert:
		title = fmt.Sprintf("Restore built-in stage group %q?", group.Name)
		affirmative = "Restore"
		if group.ShadowReason == types.ShadowTombstone {
			desc = fmt.Sprintf(
				"%q is a placeholder keeping the built-in hidden after you renamed it. Restoring it brings the built-in %q back. Nodes may need a new stage if the built-in's stages differ.",
				group.Name, group.Name)
		} else {
			desc = "Your changes are discarded. The built-in may differ from the version you forked from; you'll be asked to remap any nodes holding a stage it doesn't have."
		}

	case stageDeleteCustom:
		title = fmt.Sprintf("Delete stage group %q?", group.Name)
		desc = "No kinds reference it."
		affirmative = "Delete"

	case stageDeleteRepoint:
		title = fmt.Sprintf("Delete stage group %q?", group.Name)
		desc = fmt.Sprintf("%d kind%s reference this group (%s). Choose a group to repoint them to.",
			len(referenced), plural(len(referenced)), strings.Join(referenced, ", "))
		affirmative = "Repoint and delete"

		opts := make([]huh.Option[string], 0)
		if groups != nil {
			for _, name := range groups.Names() {
				if name == group.Name {
					continue
				}
				opts = append(opts, huh.NewOption(name, name))
			}
		}
		if len(opts) > 0 {
			f.target = opts[0].Value
		}
		fields = append(fields, huh.NewSelect[string]().
			Title("Repoint referencing kinds to").
			Options(opts...).
			Value(&f.target),
		)
	}

	fields = append(fields, huh.NewNote().Title(title).Description(desc))
	fields = append(fields, huh.NewConfirm().
		Title("Proceed?").
		Value(&f.confirm).
		Affirmative(affirmative).
		Negative("Cancel"),
	)

	f.form = huh.NewForm(
		huh.NewGroup(fields...),
	).WithTheme(wyrdHuhTheme(theme)).WithShowHelp(true)

	return f
}

func (f stageDeleteFormPane) Update(msg tea.Msg) (PaneModel, tea.Cmd) {
	if f.done {
		return f, nil
	}

	if wmsg, ok := msg.(tea.WindowSizeMsg); ok {
		f.width = wmsg.Width/2 - 2
		if f.width < 1 {
			f.width = 1
		}
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

		if !f.confirm {
			return f, tea.Batch(cmd, func() tea.Msg { return formCancelMsg{} })
		}

		name := f.group.Name
		mode := f.mode
		store := f.store
		target := f.target

		return f, tea.Batch(cmd, func() tea.Msg {
			repointed := 0
			if mode == stageDeleteRepoint {
				n, err := stage.RenameStageGroup(store, name, target)
				repointed = n
				if err != nil {
					return stageDeleteErrorMsg{err: err}
				}
			}
			if err := stage.DeleteStageGroup(store, name); err != nil {
				return stageDeleteErrorMsg{err: err}
			}
			return stageDeleteSubmitMsg{name: name, mode: mode, repointed: repointed}
		})

	case huh.StateAborted:
		f.done = true
		return f, tea.Batch(cmd, func() tea.Msg { return formCancelMsg{} })
	}

	return f, cmd
}

func (f stageDeleteFormPane) View() string {
	content := strings.TrimRight(f.form.View(), "\n")
	if content == "" {
		content = "Submitting…"
	}
	bg := f.theme.BgPrimary()
	return FillBackground(PadLines(content, f.width, bg), bg)
}

func (f stageDeleteFormPane) KeyBindings() []KeyBinding {
	return []KeyBinding{
		{Key: "tab / shift+tab", Description: "Next / previous field"},
		{Key: "enter", Description: "Next field (submit on last)"},
		{Key: "ctrl+c", Description: "Cancel form"},
	}
}

func (f stageDeleteFormPane) HandleFocusLost() tea.Cmd { return nil }
