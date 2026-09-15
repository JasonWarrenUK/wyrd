package tui

import (
	"fmt"
	"strings"

	tea "charm.land/bubbletea/v2"
	huh "charm.land/huh/v2"
	"github.com/jasonwarrenuk/wyrd/internal/stage"
	"github.com/jasonwarrenuk/wyrd/internal/types"
)

// kindDeleteMode selects which of SL.18's three user-facing operations a
// kindDeleteFormPane presents, chosen by the caller from the kind's
// provenance before the pane is ever built.
type kindDeleteMode int

const (
	// kindDeleteRevert covers both a tombstone and an ordinary shadowed
	// default (ShadowOf != ""): the user entry is dropped so the embedded
	// default resurfaces through the merge. The two share one flow; only
	// the confirmation copy differs.
	kindDeleteRevert kindDeleteMode = iota

	// kindDeleteCustom is a purely user-authored kind with no live nodes —
	// a plain delete, no target needed.
	kindDeleteCustom

	// kindDeleteReassign is a purely user-authored kind that still has
	// nodes holding it — those nodes must move to another kind before the
	// delete can proceed, or they become Unresolvable.
	kindDeleteReassign
)

// kindDeleteSubmitMsg is emitted after a successful delete/revert/reassign.
// mode distinguishes which cascade the handler should run — only reassign
// hands off to the remap form afterwards (T3: a bare delete or revert never
// routes Unresolvable nodes there, and a bare delete has nothing to remap).
type kindDeleteSubmitMsg struct {
	name       string
	mode       kindDeleteMode
	reassigned int
}

// kindDeleteErrorMsg is emitted when the write (or the reassignment that
// precedes it) fails.
type kindDeleteErrorMsg struct {
	err error
}

// kindDeleteFormPane wraps a huh.Form presenting one confirmation for
// deleting or reverting a kind, following remapFormPane's contract.
type kindDeleteFormPane struct {
	form  *huh.Form
	store types.StoreFS
	index types.GraphIndex
	theme *ActiveTheme

	kind     types.Kind
	mode     kindDeleteMode
	affected int

	target  string
	confirm bool

	width  int
	height int

	done bool
}

var _ PaneModel = kindDeleteFormPane{}
var _ formActivePane = kindDeleteFormPane{}
var _ formMountable = kindDeleteFormPane{}

func (kindDeleteFormPane) isFormActive() {}

func (f kindDeleteFormPane) initForm() tea.Cmd {
	return f.form.Init()
}

// newKindDeleteFormPane builds a kindDeleteFormPane. kinds is the current
// registry, used to populate the reassignment target select — nil for
// modes that don't need one (revert, custom with no nodes).
func newKindDeleteFormPane(theme *ActiveTheme, store types.StoreFS, index types.GraphIndex, kind types.Kind, mode kindDeleteMode, affected int, kinds *types.KindRegistry) kindDeleteFormPane {
	f := kindDeleteFormPane{
		store:    store,
		index:    index,
		theme:    theme,
		kind:     kind,
		mode:     mode,
		affected: affected,
	}

	var title, desc, affirmative string
	fields := make([]huh.Field, 0, 3)

	switch mode {
	case kindDeleteRevert:
		title = fmt.Sprintf("Restore built-in kind %q?", kind.Name)
		affirmative = "Restore"
		if kind.ShadowReason == types.ShadowTombstone {
			desc = fmt.Sprintf(
				"%q is a placeholder keeping the built-in hidden after you renamed it. Restoring it brings the built-in %q back. No nodes are affected.",
				kind.Name, kind.Name)
		} else if kind.ShadowReason == types.ShadowRenameFanOut || kind.ShadowReason == types.ShadowEditedAndRenamed {
			desc = "This entry was written automatically when you renamed a stage group, not edited by hand. Restoring the built-in reverts that change. The built-in may use different stages; you'll be asked to remap if so."
		} else {
			desc = "Your changes are discarded. The built-in may differ from the version you forked from and may use different stages; you'll be asked to remap if so."
		}

	case kindDeleteCustom:
		title = fmt.Sprintf("Delete kind %q?", kind.Name)
		desc = "No nodes currently hold it."
		affirmative = "Delete"

	case kindDeleteReassign:
		title = fmt.Sprintf("Delete kind %q?", kind.Name)
		desc = fmt.Sprintf("%d node%s hold this kind. Choose a kind to move them to, they cannot keep a kind that no longer exists.", affected, plural(affected))
		affirmative = "Reassign and delete"

		opts := make([]huh.Option[string], 0)
		if kinds != nil {
			for _, name := range kinds.Names() {
				if name == kind.Name {
					continue
				}
				opts = append(opts, huh.NewOption(name, name))
			}
		}
		if len(opts) > 0 {
			f.target = opts[0].Value
		}
		fields = append(fields, huh.NewSelect[string]().
			Title("Move nodes to").
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

func (f kindDeleteFormPane) Update(msg tea.Msg) (PaneModel, tea.Cmd) {
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

		name := f.kind.Name
		mode := f.mode
		store := f.store
		index := f.index
		target := f.target

		return f, tea.Batch(cmd, func() tea.Msg {
			reassigned := 0
			if mode == kindDeleteReassign {
				n, err := stage.ReassignKind(store, index, name, target)
				reassigned = n
				if err != nil {
					return kindDeleteErrorMsg{err: err}
				}
			}
			if err := stage.DeleteKind(store, name); err != nil {
				return kindDeleteErrorMsg{err: err}
			}
			return kindDeleteSubmitMsg{name: name, mode: mode, reassigned: reassigned}
		})

	case huh.StateAborted:
		f.done = true
		return f, tea.Batch(cmd, func() tea.Msg { return formCancelMsg{} })
	}

	return f, cmd
}

func (f kindDeleteFormPane) View() string {
	content := strings.TrimRight(f.form.View(), "\n")
	if content == "" {
		content = "Submitting…"
	}
	bg := f.theme.BgPrimary()
	return FillBackground(PadLines(content, f.width, bg), bg)
}

func (f kindDeleteFormPane) KeyBindings() []KeyBinding {
	return []KeyBinding{
		{Key: "tab / shift+tab", Description: "Next / previous field"},
		{Key: "enter", Description: "Next field (submit on last)"},
		{Key: "ctrl+c", Description: "Cancel form"},
	}
}

func (f kindDeleteFormPane) HandleFocusLost() tea.Cmd { return nil }
