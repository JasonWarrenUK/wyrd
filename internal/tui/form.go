package tui

import (
	"errors"
	"fmt"
	"strconv"
	"time"

	huh "charm.land/huh/v2"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/google/uuid"
	"github.com/jasonwarrenuk/wyrd/internal/types"
)

// formActivePane is implemented by any pane that represents an active form.
// The root model uses this interface to guard against replacing the right pane
// (with a detail view, new form, etc.) while the user is filling in a form.
type formActivePane interface {
	PaneModel
	isFormActive() // marker method
}

// formKind identifies which node type a form creates.
type formKind int

const (
	formTask    formKind = iota
	formJournal formKind = iota
	formNote    formKind = iota
	formBudget  formKind = iota
)

// formSubmitMsg is emitted by a formPane when the user completes and submits
// the form. The new node has already been written to the store.
type formSubmitMsg struct {
	nodeID string
	label  string
}

// formCancelMsg is emitted by a formPane when the user aborts with Escape.
type formCancelMsg struct{}

// editSubmitMsg is emitted by a formPane in edit mode when the user completes
// and submits the form. The updated node has already been written to the store.
type editSubmitMsg struct {
	nodeID string
	label  string
}

// formPane wraps a huh.Form and satisfies PaneModel. It is mounted in the
// right pane when the capture bar dispatches a creation form.
type formPane struct {
	form           *huh.Form
	kind           formKind
	store          types.StoreFS
	clock          types.Clock
	theme          *ActiveTheme
	selectedNodeID string // used to create a "related" edge on submit
	linkToSelected bool   // set by huh.Confirm; only meaningful when selectedNodeID != ""

	// editingNodeID is non-empty when editing an existing node. It carries the
	// original node ID so buildNode preserves it instead of generating a new UUID.
	editingNodeID  string
	editingCreated time.Time // original Created timestamp, preserved on update

	// Field values — written by huh via pointer accessors.
	title     string
	body      string
	status    string
	energy    string
	category  string // budget category
	allocated string // budget allocated amount (string for huh input, parsed on submit)
	warnAt    string // budget warn_at fraction (string for huh input, parsed on submit)
	period    string // budget period

	width  int
	height int

	// done prevents double-emission of submit/cancel messages.
	done bool
}

// Compile-time checks: formPane must satisfy both PaneModel and formActivePane.
var _ PaneModel = formPane{}
var _ formActivePane = formPane{}

// NewTaskFormPane builds a formPane for task creation. prefillTitle is the
// text the user typed in the capture bar after the "t:" prefix (may be empty).
// Exported for use in tests and by external callers.
func NewTaskFormPane(
	theme *ActiveTheme,
	store types.StoreFS,
	clock types.Clock,
	selectedNodeID string,
	prefillTitle string,
) PaneModel {
	return newTaskFormPane(theme, store, clock, selectedNodeID, prefillTitle)
}

// newTaskFormPane is the internal constructor.
func newTaskFormPane(
	theme *ActiveTheme,
	store types.StoreFS,
	clock types.Clock,
	selectedNodeID string,
	prefillTitle string,
) formPane {
	f := formPane{
		kind:           formTask,
		store:          store,
		clock:          clock,
		theme:          theme,
		selectedNodeID: selectedNodeID,
		title:          prefillTitle,
		status:         "inbox",
		energy:         "medium",
		linkToSelected: true,
	}

	fields := []huh.Field{
		huh.NewInput().
			Title("Title").
			Value(&f.title).
			Validate(notEmpty("title")),

		huh.NewText().
			Title("Body").
			Value(&f.body).
			Lines(6).
			Placeholder("Describe the task (alt+enter for new line, ctrl+e for editor)"),

		huh.NewSelect[string]().
			Title("Status").
			Options(
				huh.NewOption("Inbox", "inbox"),
				huh.NewOption("Active", "active"),
				huh.NewOption("Waiting", "waiting"),
			).
			Value(&f.status),

		huh.NewSelect[string]().
			Title("Energy").
			Options(
				huh.NewOption("Deep", "deep"),
				huh.NewOption("Medium", "medium"),
				huh.NewOption("Low", "low"),
			).
			Value(&f.energy),
	}
	if selectedNodeID != "" {
		fields = append(fields, huh.NewConfirm().
			Title("Link to selected node?").
			Value(&f.linkToSelected).
			Affirmative("Yes").
			Negative("No"),
		)
	}

	f.form = huh.NewForm(
		huh.NewGroup(fields...),
	).WithTheme(wyrdHuhTheme(theme)).WithShowHelp(true)

	return f
}

// NewJournalFormPane builds a formPane for journal entry creation.
// Exported for use in tests.
func NewJournalFormPane(
	theme *ActiveTheme,
	store types.StoreFS,
	clock types.Clock,
	selectedNodeID string,
	prefillTitle string,
) PaneModel {
	return newJournalFormPane(theme, store, clock, selectedNodeID, prefillTitle)
}

// newJournalFormPane is the internal constructor.
func newJournalFormPane(
	theme *ActiveTheme,
	store types.StoreFS,
	clock types.Clock,
	selectedNodeID string,
	prefillTitle string,
) formPane {
	f := formPane{
		kind:           formJournal,
		store:          store,
		clock:          clock,
		theme:          theme,
		selectedNodeID: selectedNodeID,
		linkToSelected: true,
	}
	if prefillTitle != "" {
		f.title = prefillTitle
	} else if clock != nil {
		f.title = clock.Now().Format("2006-01-02")
	}

	fields := []huh.Field{
		huh.NewInput().
			Title("Title").
			Value(&f.title),

		huh.NewText().
			Title("Body").
			Value(&f.body).
			Lines(12).
			Placeholder("Write your entry (alt+enter for new line, ctrl+e for editor)").
			Validate(notEmpty("body")),
	}
	if selectedNodeID != "" {
		fields = append(fields, huh.NewConfirm().
			Title("Link to selected node?").
			Value(&f.linkToSelected).
			Affirmative("Yes").
			Negative("No"),
		)
	}

	f.form = huh.NewForm(
		huh.NewGroup(fields...),
	).WithTheme(wyrdHuhTheme(theme)).WithShowHelp(true)

	return f
}

// NewNoteFormPane builds a formPane for note creation.
// Exported for use in tests.
func NewNoteFormPane(
	theme *ActiveTheme,
	store types.StoreFS,
	clock types.Clock,
	selectedNodeID string,
	prefillTitle string,
) PaneModel {
	return newNoteFormPane(theme, store, clock, selectedNodeID, prefillTitle)
}

// newNoteFormPane is the internal constructor.
func newNoteFormPane(
	theme *ActiveTheme,
	store types.StoreFS,
	clock types.Clock,
	selectedNodeID string,
	prefillTitle string,
) formPane {
	f := formPane{
		kind:           formNote,
		store:          store,
		clock:          clock,
		theme:          theme,
		selectedNodeID: selectedNodeID,
		title:          prefillTitle,
		linkToSelected: true,
	}

	fields := []huh.Field{
		huh.NewInput().
			Title("Title").
			Value(&f.title).
			Validate(notEmpty("title")),

		huh.NewText().
			Title("Body").
			Value(&f.body).
			Lines(8).
			Placeholder("Write your note (alt+enter for new line, ctrl+e for editor)"),
	}
	if selectedNodeID != "" {
		fields = append(fields, huh.NewConfirm().
			Title("Link to selected node?").
			Value(&f.linkToSelected).
			Affirmative("Yes").
			Negative("No"),
		)
	}

	f.form = huh.NewForm(
		huh.NewGroup(fields...),
	).WithTheme(wyrdHuhTheme(theme)).WithShowHelp(true)

	return f
}

// NewBudgetFormPane builds a formPane for budget creation.
// prefillCategory is the text the user typed after the "b:" prefix (may be empty).
// Exported for use in tests.
func NewBudgetFormPane(
	theme *ActiveTheme,
	store types.StoreFS,
	clock types.Clock,
	selectedNodeID string,
	prefillCategory string,
) PaneModel {
	return newBudgetFormPane(theme, store, clock, selectedNodeID, prefillCategory)
}

// newBudgetFormPane is the internal constructor.
func newBudgetFormPane(
	theme *ActiveTheme,
	store types.StoreFS,
	clock types.Clock,
	selectedNodeID string,
	prefillCategory string,
) formPane {
	f := formPane{
		kind:           formBudget,
		store:          store,
		clock:          clock,
		theme:          theme,
		selectedNodeID: selectedNodeID,
		category:       prefillCategory,
		allocated:      "",
		warnAt:         "0.8",
		period:         "monthly",
		linkToSelected: true,
	}

	fields := []huh.Field{
		huh.NewInput().
			Title("Category").
			Value(&f.category).
			Validate(notEmpty("category")),

		huh.NewInput().
			Title("Allocated").
			Value(&f.allocated).
			Placeholder("0.00").
			Validate(validatePositiveNumber("allocated")),

		huh.NewInput().
			Title("Warn at (fraction 0–1)").
			Value(&f.warnAt).
			Placeholder("0.8"),

		huh.NewSelect[string]().
			Title("Period").
			Options(
				huh.NewOption("Weekly", "weekly"),
				huh.NewOption("Monthly", "monthly"),
				huh.NewOption("Quarterly", "quarterly"),
				huh.NewOption("Yearly", "yearly"),
			).
			Value(&f.period),
	}
	if selectedNodeID != "" {
		fields = append(fields, huh.NewConfirm().
			Title("Link to selected node?").
			Value(&f.linkToSelected).
			Affirmative("Yes").
			Negative("No"),
		)
	}

	f.form = huh.NewForm(
		huh.NewGroup(fields...),
	).WithTheme(wyrdHuhTheme(theme)).WithShowHelp(true)

	return f
}

// NewEditTaskFormPane builds a formPane for editing an existing task node.
// All fields are pre-filled from the node. Exported for use in tests.
func NewEditTaskFormPane(
	theme *ActiveTheme,
	store types.StoreFS,
	clock types.Clock,
	node *types.Node,
) PaneModel {
	return newEditTaskFormPane(theme, store, clock, node)
}

// newEditTaskFormPane is the internal constructor.
func newEditTaskFormPane(
	theme *ActiveTheme,
	store types.StoreFS,
	clock types.Clock,
	node *types.Node,
) formPane {
	status := "inbox"
	if v, ok := node.Properties["status"].(string); ok && v != "" {
		status = v
	}
	energy := "medium"
	if v, ok := node.Properties["energy"].(string); ok && v != "" {
		energy = v
	}

	f := formPane{
		kind:           formTask,
		store:          store,
		clock:          clock,
		theme:          theme,
		title:          node.Title,
		body:           node.Body,
		status:         status,
		energy:         energy,
		editingNodeID:  node.ID,
		editingCreated: node.Created,
	}

	f.form = huh.NewForm(
		huh.NewGroup(
			huh.NewInput().
				Title("Title").
				Value(&f.title).
				Validate(notEmpty("title")),

			huh.NewText().
				Title("Body").
				Value(&f.body).
				Lines(6).
				Placeholder("Describe the task (alt+enter for new line, ctrl+e for editor)"),

			huh.NewSelect[string]().
				Title("Status").
				Options(
					huh.NewOption("Inbox", "inbox"),
					huh.NewOption("Active", "active"),
					huh.NewOption("Waiting", "waiting"),
				).
				Value(&f.status),

			huh.NewSelect[string]().
				Title("Energy").
				Options(
					huh.NewOption("Deep", "deep"),
					huh.NewOption("Medium", "medium"),
					huh.NewOption("Low", "low"),
				).
				Value(&f.energy),
		),
	).WithTheme(wyrdHuhTheme(theme)).WithShowHelp(true)

	return f
}

// NewEditJournalFormPane builds a formPane for editing an existing journal node.
// Exported for use in tests.
func NewEditJournalFormPane(
	theme *ActiveTheme,
	store types.StoreFS,
	clock types.Clock,
	node *types.Node,
) PaneModel {
	return newEditJournalFormPane(theme, store, clock, node)
}

// newEditJournalFormPane is the internal constructor.
func newEditJournalFormPane(
	theme *ActiveTheme,
	store types.StoreFS,
	clock types.Clock,
	node *types.Node,
) formPane {
	f := formPane{
		kind:           formJournal,
		store:          store,
		clock:          clock,
		theme:          theme,
		title:          node.Title,
		body:           node.Body,
		editingNodeID:  node.ID,
		editingCreated: node.Created,
	}

	f.form = huh.NewForm(
		huh.NewGroup(
			huh.NewInput().
				Title("Title").
				Value(&f.title),

			huh.NewText().
				Title("Body").
				Value(&f.body).
				Lines(12).
				Placeholder("Write your entry (alt+enter for new line, ctrl+e for editor)").
				Validate(notEmpty("body")),
		),
	).WithTheme(wyrdHuhTheme(theme)).WithShowHelp(true)

	return f
}

// NewEditNoteFormPane builds a formPane for editing an existing note node.
// Exported for use in tests.
func NewEditNoteFormPane(
	theme *ActiveTheme,
	store types.StoreFS,
	clock types.Clock,
	node *types.Node,
) PaneModel {
	return newEditNoteFormPane(theme, store, clock, node)
}

// newEditNoteFormPane is the internal constructor.
func newEditNoteFormPane(
	theme *ActiveTheme,
	store types.StoreFS,
	clock types.Clock,
	node *types.Node,
) formPane {
	f := formPane{
		kind:           formNote,
		store:          store,
		clock:          clock,
		theme:          theme,
		title:          node.Title,
		body:           node.Body,
		editingNodeID:  node.ID,
		editingCreated: node.Created,
	}

	f.form = huh.NewForm(
		huh.NewGroup(
			huh.NewInput().
				Title("Title").
				Value(&f.title).
				Validate(notEmpty("title")),

			huh.NewText().
				Title("Body").
				Value(&f.body).
				Lines(8).
				Placeholder("Write your note (alt+enter for new line, ctrl+e for editor)"),
		),
	).WithTheme(wyrdHuhTheme(theme)).WithShowHelp(true)

	return f
}

// Update forwards messages to the huh form and detects completion/abort.
func (f formPane) Update(msg tea.Msg) (PaneModel, tea.Cmd) {
	if f.done {
		return f, nil
	}

	if wmsg, ok := msg.(tea.WindowSizeMsg); ok {
		f.width = wmsg.Width/2 - 2
		f.height = wmsg.Height - 3
		f.form.WithWidth(f.width).WithHeight(f.height)
	}

	model, cmd := f.form.Update(msg)
	if updated, ok := model.(*huh.Form); ok {
		f.form = updated
	}

	switch f.form.State {
	case huh.StateCompleted:
		f.done = true
		node := f.buildNode()
		if err := f.store.WriteNode(node); err != nil {
			// Non-fatal — emit cancel so the pane is restored.
			return f, func() tea.Msg { return formCancelMsg{} }
		}
		if f.editingNodeID == "" && f.selectedNodeID != "" && f.linkToSelected {
			now := f.clock.Now()
			edge := &types.Edge{
				ID:      uuid.New().String(),
				Type:    string(types.EdgeRelated),
				From:    node.ID,
				To:      f.selectedNodeID,
				Created: now,
			}
			_ = f.store.WriteEdge(edge) // non-fatal
		}
		label := node.Types[0] + ": " + node.Title
		if node.Title == "" {
			label = node.Types[0] + ": " + node.Body
		}
		if len(label) > 40 {
			label = label[:37] + "…"
		}
		if f.editingNodeID != "" {
			return f, tea.Batch(cmd, func() tea.Msg {
				return editSubmitMsg{nodeID: node.ID, label: label}
			})
		}
		return f, tea.Batch(cmd, func() tea.Msg {
			return formSubmitMsg{nodeID: node.ID, label: label}
		})

	case huh.StateAborted:
		f.done = true
		return f, tea.Batch(cmd, func() tea.Msg { return formCancelMsg{} })
	}

	return f, cmd
}

// View renders the huh form, padded to fill the pane.
func (f formPane) View() string {
	content := f.form.View()
	if content == "" {
		content = "Submitting…"
	}
	bg := f.theme.BgPrimary()
	return PadLines(content, f.width, bg)
}

// KeyBindings returns the help hints shown in the command palette.
func (f formPane) KeyBindings() []KeyBinding {
	return []KeyBinding{
		{Key: "tab / shift+tab", Description: "Next / previous field"},
		{Key: "enter", Description: "Next field (submit on last)"},
		{Key: "alt+enter", Description: "New line in text field"},
		{Key: "ctrl+e", Description: "Open external editor"},
		{Key: "ctrl+c", Description: "Cancel form"},
	}
}

// HandleFocusLost is a no-op for form panes.
func (f formPane) HandleFocusLost() tea.Cmd { return nil }

// isFormActive satisfies the formActivePane marker interface.
func (formPane) isFormActive() {}

// buildNode constructs a types.Node from the captured form field values.
// When editingNodeID is set, the original ID and Created timestamp are preserved.
func (f formPane) buildNode() *types.Node {
	now := f.clock.Now()
	id := uuid.New().String()
	created := now
	if f.editingNodeID != "" {
		id = f.editingNodeID
		created = f.editingCreated
	}
	node := &types.Node{
		ID:         id,
		Title:      f.title,
		Body:       f.body,
		Created:    created,
		Modified:   now,
		Properties: make(map[string]interface{}),
	}

	switch f.kind {
	case formTask:
		node.Types = []string{"task"}
		if f.status != "" {
			node.Properties["status"] = f.status
		}
		if f.energy != "" {
			node.Properties["energy"] = f.energy
		}

	case formJournal:
		node.Types = []string{"journal"}
		node.Date.About = &now

	case formNote:
		node.Types = []string{"note"}

	case formBudget:
		node.Types = []string{"budget"}
		node.Title = f.category
		node.Properties["category"] = f.category
		if alloc, err := strconv.ParseFloat(f.allocated, 64); err == nil {
			node.Properties["allocated"] = alloc
		}
		if warnAt, err := strconv.ParseFloat(f.warnAt, 64); err == nil {
			node.Properties["warn_at"] = warnAt
		} else {
			node.Properties["warn_at"] = 0.8
		}
		node.Properties["period"] = f.period
	}

	return node
}

// notEmpty returns a validation function that rejects blank strings.
func notEmpty(fieldName string) func(string) error {
	return func(s string) error {
		if s == "" {
			return errors.New(fieldName + " is required")
		}
		return nil
	}
}

// validatePositiveNumber returns a validation function that rejects empty,
// non-numeric, zero, and negative values.
func validatePositiveNumber(fieldName string) func(string) error {
	return func(s string) error {
		if s == "" {
			return fmt.Errorf("%s is required", fieldName)
		}
		v, err := strconv.ParseFloat(s, 64)
		if err != nil {
			return fmt.Errorf("%s must be a number", fieldName)
		}
		if v <= 0 {
			return fmt.Errorf("%s must be greater than zero", fieldName)
		}
		return nil
	}
}

// wyrdHuhTheme returns a huh.Theme derived from the active Wyrd theme.
// This is a minimal mapping — full polish is deferred to VS.8.
func wyrdHuhTheme(t *ActiveTheme) huh.Theme {
	if t == nil {
		return huh.ThemeFunc(func(isDark bool) *huh.Styles { return huh.ThemeCharm(isDark) })
	}

	return huh.ThemeFunc(func(isDark bool) *huh.Styles {
		base := huh.ThemeCharm(isDark)

		bg := t.BgPrimary()
		fg := t.FgPrimary()
		muted := t.FgMuted()
		accent := t.AccentPrimary()
		errCol := t.OverflowCritical()

		// Form base container.
		base.Form.Base = base.Form.Base.
			Background(bg).
			Foreground(fg)

		// Group titles.
		base.Group.Title = base.Group.Title.
			Foreground(accent).
			Background(bg)
		base.Group.Description = base.Group.Description.
			Foreground(muted).
			Background(bg)

		// Focused field styles.
		base.Focused.Base = base.Focused.Base.
			Background(bg).
			BorderForeground(accent)
		base.Focused.Title = base.Focused.Title.
			Foreground(accent).
			Background(bg)
		base.Focused.Description = base.Focused.Description.
			Foreground(muted).
			Background(bg)
		base.Focused.ErrorIndicator = base.Focused.ErrorIndicator.
			Foreground(errCol).
			Background(bg)
		base.Focused.ErrorMessage = base.Focused.ErrorMessage.
			Foreground(errCol).
			Background(bg)
		base.Focused.SelectSelector = base.Focused.SelectSelector.
			Foreground(accent).
			Background(bg)
		base.Focused.Option = base.Focused.Option.
			Foreground(fg).
			Background(bg)
		base.Focused.TextInput.Text = lipgloss.NewStyle().
			Foreground(fg).
			Background(bg)
		base.Focused.TextInput.Placeholder = lipgloss.NewStyle().
			Foreground(muted).
			Background(bg)
		base.Focused.TextInput.Cursor = lipgloss.NewStyle().
			Foreground(accent).
			Background(bg)
		base.Focused.FocusedButton = base.Focused.FocusedButton.
			Background(accent).
			Foreground(bg)

		// Blurred field styles.
		base.Blurred.Base = base.Blurred.Base.
			Background(bg)
		base.Blurred.Title = base.Blurred.Title.
			Foreground(muted).
			Background(bg)
		base.Blurred.Description = base.Blurred.Description.
			Foreground(muted).
			Background(bg)
		base.Blurred.Option = base.Blurred.Option.
			Foreground(muted).
			Background(bg)
		base.Blurred.TextInput.Text = lipgloss.NewStyle().
			Foreground(muted).
			Background(bg)

		return base
	})
}
