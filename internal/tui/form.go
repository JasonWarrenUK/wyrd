package tui

import (
	"errors"
	"fmt"
	"image/color"
	"strconv"
	"strings"

	huh "charm.land/huh/v2"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/google/uuid"
	"github.com/jasonwarrenuk/wyrd/internal/budget"
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

// edgeEntry describes a single edge connected to the node being edited.
// Used to populate the multi-select field that lets the user keep or remove
// existing edges.
type edgeEntry struct {
	ID        string // edge UUID
	Direction string // "→" (outgoing) or "←" (incoming)
	EdgeType  string // e.g. "blocks", "related"
	TargetID  string // the other node's UUID
	Label     string // human-readable label shown in the multi-select
}

// formPane wraps a huh.Form and satisfies PaneModel. It is mounted in the
// right pane when the capture bar dispatches a creation form.
type formPane struct {
	form           *huh.Form
	kind           formKind
	store          types.StoreFS
	index          types.GraphIndex
	clock          types.Clock
	theme          *ActiveTheme
	selectedNodeID string // used to create a "related" edge on submit
	linkToSelected bool   // set by huh.Confirm; only meaningful when selectedNodeID != ""

	// kinds and stageGroups are the merged registries used to populate the
	// kind-select field and, on create, to resolve the selected kind's first
	// stage. Either may be nil (tests, or app built without registry wiring);
	// the constructor and buildNode nil-guard before use. `kind` (the formKind
	// discriminator) is a distinct field — no collision.
	kinds       *types.KindRegistry
	stageGroups *types.StageGroupRegistry

	// originalNode is non-nil when editing an existing node. buildNode starts
	// from a clone of it so everything outside the form's own fields (custom
	// properties, spend_log, date sub-fields, kind/stage, source) survives the
	// edit. Stashed as a clone so later caller mutations cannot leak in.
	originalNode *types.Node

	// Edge management (edit mode only). existingEdges holds all edges found
	// when the edit form was constructed. keptEdgeIDs is bound to the
	// multi-select — unchecked IDs will be deleted on submit. newEdgeType and
	// newEdgeTarget allow creating a single new edge.
	existingEdges []edgeEntry
	keptEdgeIDs   []string
	newEdgeType   string
	newEdgeTarget string

	// Field values — written by huh via pointer accessors.
	title     string
	body      string
	nodeKind  string // selected Kind registry name (e.g. "Task"); bound to the kind huh.Select
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

// editingID returns the ID of the node being edited, or "" in create mode.
func (f formPane) editingID() string {
	if f.originalNode == nil {
		return ""
	}
	return f.originalNode.ID
}

// NewTaskFormPane builds a formPane for task creation. prefillTitle is the
// text the user typed in the capture bar after the "t:" prefix (may be empty).
// Exported for use in tests and by external callers.
func NewTaskFormPane(
	theme *ActiveTheme,
	store types.StoreFS,
	clock types.Clock,
	selectedNodeID string,
	prefillTitle string,
	kinds *types.KindRegistry,
	stageGroups *types.StageGroupRegistry,
) PaneModel {
	return newTaskFormPane(theme, store, clock, selectedNodeID, prefillTitle, kinds, stageGroups)
}

// newTaskFormPane is the internal constructor.
func newTaskFormPane(
	theme *ActiveTheme,
	store types.StoreFS,
	clock types.Clock,
	selectedNodeID string,
	prefillTitle string,
	kinds *types.KindRegistry,
	stageGroups *types.StageGroupRegistry,
) formPane {
	f := formPane{
		kind:           formTask,
		store:          store,
		clock:          clock,
		theme:          theme,
		kinds:          kinds,
		stageGroups:    stageGroups,
		selectedNodeID: selectedNodeID,
		title:          prefillTitle,
		status:         "inbox",
		energy:         "medium",
		nodeKind:       "Task", // default selection; also the value buildNode reads when the select is omitted
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
	}
	if kinds != nil {
		names := kinds.Names()
		if len(names) > 0 {
			opts := make([]huh.Option[string], len(names))
			for i, name := range names {
				opts[i] = huh.NewOption(name, name)
			}
			fields = append(fields, huh.NewSelect[string]().
				Title("Kind").
				Options(opts...).
				Value(&f.nodeKind),
			)
		}
	}
	fields = append(fields,
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
	)
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
	kinds *types.KindRegistry,
	stageGroups *types.StageGroupRegistry,
) PaneModel {
	return newJournalFormPane(theme, store, clock, selectedNodeID, prefillTitle, kinds, stageGroups)
}

// newJournalFormPane is the internal constructor.
func newJournalFormPane(
	theme *ActiveTheme,
	store types.StoreFS,
	clock types.Clock,
	selectedNodeID string,
	prefillTitle string,
	kinds *types.KindRegistry,
	stageGroups *types.StageGroupRegistry,
) formPane {
	f := formPane{
		kind:           formJournal,
		store:          store,
		clock:          clock,
		theme:          theme,
		selectedNodeID: selectedNodeID,
		linkToSelected: true,
		nodeKind:       "Journal",
		kinds:          kinds,
		stageGroups:    stageGroups,
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
	if kinds != nil {
		names := kinds.Names()
		if len(names) > 0 {
			opts := make([]huh.Option[string], len(names))
			for i, name := range names {
				opts[i] = huh.NewOption(name, name)
			}
			fields = append(fields, huh.NewSelect[string]().
				Title("Kind").
				Options(opts...).
				Value(&f.nodeKind),
			)
		}
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
	kinds *types.KindRegistry,
	stageGroups *types.StageGroupRegistry,
) PaneModel {
	return newNoteFormPane(theme, store, clock, selectedNodeID, prefillTitle, kinds, stageGroups)
}

// newNoteFormPane is the internal constructor.
func newNoteFormPane(
	theme *ActiveTheme,
	store types.StoreFS,
	clock types.Clock,
	selectedNodeID string,
	prefillTitle string,
	kinds *types.KindRegistry,
	stageGroups *types.StageGroupRegistry,
) formPane {
	f := formPane{
		kind:           formNote,
		store:          store,
		clock:          clock,
		theme:          theme,
		selectedNodeID: selectedNodeID,
		title:          prefillTitle,
		linkToSelected: true,
		nodeKind:       "Note",
		kinds:          kinds,
		stageGroups:    stageGroups,
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
	if kinds != nil {
		names := kinds.Names()
		if len(names) > 0 {
			opts := make([]huh.Option[string], len(names))
			for i, name := range names {
				opts[i] = huh.NewOption(name, name)
			}
			fields = append(fields, huh.NewSelect[string]().
				Title("Kind").
				Options(opts...).
				Value(&f.nodeKind),
			)
		}
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
	kinds *types.KindRegistry,
	stageGroups *types.StageGroupRegistry,
) PaneModel {
	return newBudgetFormPane(theme, store, clock, selectedNodeID, prefillCategory, kinds, stageGroups)
}

// newBudgetFormPane is the internal constructor.
func newBudgetFormPane(
	theme *ActiveTheme,
	store types.StoreFS,
	clock types.Clock,
	selectedNodeID string,
	prefillCategory string,
	kinds *types.KindRegistry,
	stageGroups *types.StageGroupRegistry,
) formPane {
	f := formPane{
		kind:           formBudget,
		store:          store,
		clock:          clock,
		theme:          theme,
		selectedNodeID: selectedNodeID,
		category:       prefillCategory,
		allocated:      "",
		warnAt:         "",
		period:         "month",
		linkToSelected: true,
		nodeKind:       "Budget",
		kinds:          kinds,
		stageGroups:    stageGroups,
	}

	fields := []huh.Field{
		huh.NewInput().
			Title("Category").
			Value(&f.category).
			Validate(notEmpty("category")),
	}
	if kinds != nil {
		names := kinds.Names()
		if len(names) > 0 {
			opts := make([]huh.Option[string], len(names))
			for i, name := range names {
				opts[i] = huh.NewOption(name, name)
			}
			fields = append(fields, huh.NewSelect[string]().
				Title("Kind").
				Options(opts...).
				Value(&f.nodeKind),
			)
		}
	}
	fields = append(fields,
		huh.NewInput().
			Title("Allocated").
			Value(&f.allocated).
			Placeholder("0.00").
			Validate(validateNonNegativeNumber("allocated")),

		huh.NewInput().
			Title("Warn at (fraction 0–1)").
			Value(&f.warnAt).
			Placeholder("1").
			Validate(validateOptionalFraction("warn_at")),

		huh.NewSelect[string]().
			Title("Period").
			Options(
				huh.NewOption("Weekly", "week"),
				huh.NewOption("Monthly", "month"),
				huh.NewOption("Quarterly", "quarter"),
				huh.NewOption("Yearly", "year"),
			).
			Value(&f.period),
	)
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
	index types.GraphIndex,
	node *types.Node,
	kinds *types.KindRegistry,
	stageGroups *types.StageGroupRegistry,
) PaneModel {
	return newEditTaskFormPane(theme, store, clock, index, node, kinds, stageGroups)
}

// newEditTaskFormPane is the internal constructor.
func newEditTaskFormPane(
	theme *ActiveTheme,
	store types.StoreFS,
	clock types.Clock,
	index types.GraphIndex,
	node *types.Node,
	kinds *types.KindRegistry,
	stageGroups *types.StageGroupRegistry,
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
		kind:         formTask,
		store:        store,
		index:        index,
		clock:        clock,
		theme:        theme,
		title:        node.Title,
		body:         node.Body,
		status:       status,
		energy:       energy,
		nodeKind:     node.Kind,
		kinds:        kinds,
		stageGroups:  stageGroups,
		originalNode: node.Clone(),
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
	}
	if kinds != nil {
		names := kinds.Names()
		if len(names) > 0 {
			opts := make([]huh.Option[string], 0, len(names)+1)
			opts = append(opts, huh.NewOption("— none —", ""))
			for _, name := range names {
				opts = append(opts, huh.NewOption(name, name))
			}
			fields = append(fields, huh.NewSelect[string]().
				Title("Kind").
				Options(opts...).
				Value(&f.nodeKind),
			)
		}
	}
	fields = append(fields,
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
	)

	fields = appendEdgeFields(&f, index, node, fields)

	f.form = huh.NewForm(
		huh.NewGroup(fields...),
	).WithTheme(wyrdHuhTheme(theme)).WithShowHelp(true)

	return f
}

// NewEditJournalFormPane builds a formPane for editing an existing journal node.
// Exported for use in tests.
func NewEditJournalFormPane(
	theme *ActiveTheme,
	store types.StoreFS,
	clock types.Clock,
	index types.GraphIndex,
	node *types.Node,
	kinds *types.KindRegistry,
	stageGroups *types.StageGroupRegistry,
) PaneModel {
	return newEditJournalFormPane(theme, store, clock, index, node, kinds, stageGroups)
}

// newEditJournalFormPane is the internal constructor.
func newEditJournalFormPane(
	theme *ActiveTheme,
	store types.StoreFS,
	clock types.Clock,
	index types.GraphIndex,
	node *types.Node,
	kinds *types.KindRegistry,
	stageGroups *types.StageGroupRegistry,
) formPane {
	f := formPane{
		kind:         formJournal,
		store:        store,
		index:        index,
		clock:        clock,
		theme:        theme,
		title:        node.Title,
		body:         node.Body,
		nodeKind:     node.Kind,
		kinds:        kinds,
		stageGroups:  stageGroups,
		originalNode: node.Clone(),
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
	if kinds != nil {
		names := kinds.Names()
		if len(names) > 0 {
			opts := make([]huh.Option[string], 0, len(names)+1)
			opts = append(opts, huh.NewOption("— none —", ""))
			for _, name := range names {
				opts = append(opts, huh.NewOption(name, name))
			}
			fields = append(fields, huh.NewSelect[string]().
				Title("Kind").
				Options(opts...).
				Value(&f.nodeKind),
			)
		}
	}

	fields = appendEdgeFields(&f, index, node, fields)

	f.form = huh.NewForm(
		huh.NewGroup(fields...),
	).WithTheme(wyrdHuhTheme(theme)).WithShowHelp(true)

	return f
}

// NewEditNoteFormPane builds a formPane for editing an existing note node.
// Exported for use in tests.
func NewEditNoteFormPane(
	theme *ActiveTheme,
	store types.StoreFS,
	clock types.Clock,
	index types.GraphIndex,
	node *types.Node,
	kinds *types.KindRegistry,
	stageGroups *types.StageGroupRegistry,
) PaneModel {
	return newEditNoteFormPane(theme, store, clock, index, node, kinds, stageGroups)
}

// newEditNoteFormPane is the internal constructor.
func newEditNoteFormPane(
	theme *ActiveTheme,
	store types.StoreFS,
	clock types.Clock,
	index types.GraphIndex,
	node *types.Node,
	kinds *types.KindRegistry,
	stageGroups *types.StageGroupRegistry,
) formPane {
	f := formPane{
		kind:         formNote,
		store:        store,
		index:        index,
		clock:        clock,
		theme:        theme,
		title:        node.Title,
		body:         node.Body,
		nodeKind:     node.Kind,
		kinds:        kinds,
		stageGroups:  stageGroups,
		originalNode: node.Clone(),
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
	if kinds != nil {
		names := kinds.Names()
		if len(names) > 0 {
			opts := make([]huh.Option[string], 0, len(names)+1)
			opts = append(opts, huh.NewOption("— none —", ""))
			for _, name := range names {
				opts = append(opts, huh.NewOption(name, name))
			}
			fields = append(fields, huh.NewSelect[string]().
				Title("Kind").
				Options(opts...).
				Value(&f.nodeKind),
			)
		}
	}

	fields = appendEdgeFields(&f, index, node, fields)

	f.form = huh.NewForm(
		huh.NewGroup(fields...),
	).WithTheme(wyrdHuhTheme(theme)).WithShowHelp(true)

	return f
}

// NewEditBudgetFormPane builds a formPane for editing an existing budget node.
// All fields are pre-filled from the node. Exported for use in tests.
func NewEditBudgetFormPane(
	theme *ActiveTheme,
	store types.StoreFS,
	clock types.Clock,
	index types.GraphIndex,
	node *types.Node,
	kinds *types.KindRegistry,
	stageGroups *types.StageGroupRegistry,
) PaneModel {
	return newEditBudgetFormPane(theme, store, clock, index, node, kinds, stageGroups)
}

// newEditBudgetFormPane is the internal constructor.
func newEditBudgetFormPane(
	theme *ActiveTheme,
	store types.StoreFS,
	clock types.Clock,
	index types.GraphIndex,
	node *types.Node,
	kinds *types.KindRegistry,
	stageGroups *types.StageGroupRegistry,
) formPane {
	category := node.Title
	if v, ok := node.Properties["category"].(string); ok && v != "" {
		category = v
	}

	allocated := ""
	if v, ok := node.Properties["allocated"].(float64); ok {
		allocated = strconv.FormatFloat(v, 'f', -1, 64)
	}

	warnAt := ""
	if v, ok := node.Properties["warn_at"].(float64); ok {
		warnAt = strconv.FormatFloat(v, 'f', -1, 64)
	}

	period := "month"
	if v, ok := node.Properties["period"].(string); ok && v != "" {
		period = budget.NormalisePeriod(v)
	}

	f := formPane{
		kind:         formBudget,
		store:        store,
		index:        index,
		clock:        clock,
		theme:        theme,
		category:     category,
		allocated:    allocated,
		warnAt:       warnAt,
		period:       period,
		nodeKind:     node.Kind,
		kinds:        kinds,
		stageGroups:  stageGroups,
		originalNode: node.Clone(),
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
			Validate(validateNonNegativeNumber("allocated")),

		huh.NewInput().
			Title("Warn at (fraction 0–1)").
			Value(&f.warnAt).
			Placeholder("1").
			Validate(validateOptionalFraction("warn_at")),

		huh.NewSelect[string]().
			Title("Period").
			Options(
				huh.NewOption("Weekly", "week"),
				huh.NewOption("Monthly", "month"),
				huh.NewOption("Quarterly", "quarter"),
				huh.NewOption("Yearly", "year"),
			).
			Value(&f.period),
	}
	if kinds != nil {
		names := kinds.Names()
		if len(names) > 0 {
			opts := make([]huh.Option[string], 0, len(names)+1)
			opts = append(opts, huh.NewOption("— none —", ""))
			for _, name := range names {
				opts = append(opts, huh.NewOption(name, name))
			}
			fields = append(fields, huh.NewSelect[string]().
				Title("Kind").
				Options(opts...).
				Value(&f.nodeKind),
			)
		}
	}

	fields = appendEdgeFields(&f, index, node, fields)

	f.form = huh.NewForm(
		huh.NewGroup(fields...),
	).WithTheme(wyrdHuhTheme(theme)).WithShowHelp(true)

	return f
}

// buildEdgeEntries queries the index for all edges connected to nodeID and
// returns them as edgeEntry values with human-readable labels.
func buildEdgeEntries(index types.GraphIndex, nodeID string) []edgeEntry {
	if index == nil {
		return nil
	}

	var entries []edgeEntry

	for _, e := range index.EdgesFrom(nodeID) {
		label := fmt.Sprintf("→ %s → %s", e.Type, shortNodeLabel(index, e.To))
		entries = append(entries, edgeEntry{
			ID:        e.ID,
			Direction: "→",
			EdgeType:  e.Type,
			TargetID:  e.To,
			Label:     label,
		})
	}

	for _, e := range index.EdgesTo(nodeID) {
		label := fmt.Sprintf("← %s ← %s", e.Type, shortNodeLabel(index, e.From))
		entries = append(entries, edgeEntry{
			ID:        e.ID,
			Direction: "←",
			EdgeType:  e.Type,
			TargetID:  e.From,
			Label:     label,
		})
	}

	return entries
}

// shortNodeLabel returns a truncated title for the given node ID, falling back
// to the raw ID when the node is missing or untitled.
func shortNodeLabel(index types.GraphIndex, nodeID string) string {
	if index == nil {
		return nodeID[:8]
	}
	n, err := index.GetNode(nodeID)
	if err != nil || n.Title == "" {
		if len(nodeID) > 8 {
			return nodeID[:8] + "…"
		}
		return nodeID
	}
	title := n.Title
	if len(title) > 30 {
		title = title[:27] + "…"
	}
	return title
}

// appendEdgeFields builds the edge management form fields (multi-select for
// existing edges, select for new edge type, input for new edge target) and
// appends them to the provided fields slice. It also populates the formPane's
// existingEdges and keptEdgeIDs fields. Returns the updated fields slice.
func appendEdgeFields(f *formPane, index types.GraphIndex, node *types.Node, fields []huh.Field) []huh.Field {
	entries := buildEdgeEntries(index, node.ID)
	f.existingEdges = entries

	if len(entries) > 0 {
		// Pre-select all existing edges (user unchecks to remove).
		allIDs := make([]string, len(entries))
		opts := make([]huh.Option[string], len(entries))
		for i, entry := range entries {
			allIDs[i] = entry.ID
			opts[i] = huh.NewOption(entry.Label, entry.ID)
		}
		f.keptEdgeIDs = allIDs

		fields = append(fields, huh.NewMultiSelect[string]().
			Title("Existing Edges (uncheck to remove)").
			Value(&f.keptEdgeIDs).
			Options(opts...).
			Height(min(len(entries)+2, 8)),
		)
	}

	// New edge creation fields — always shown in edit mode.
	f.newEdgeType = "(none)"
	fields = append(fields,
		huh.NewSelect[string]().
			Title("Add Edge Type").
			Options(
				huh.NewOption("(none)", "(none)"),
				huh.NewOption("Related", string(types.EdgeRelated)),
				huh.NewOption("Blocks", string(types.EdgeBlocks)),
				huh.NewOption("Waiting On", string(types.EdgeWaitingOn)),
				huh.NewOption("Parent", string(types.EdgeParent)),
				huh.NewOption("Precedes", string(types.EdgePrecedes)),
			).
			Value(&f.newEdgeType),

		huh.NewInput().
			Title("New Edge Target (node ID)").
			Value(&f.newEdgeTarget).
			Placeholder("Paste a node UUID to create a new edge").
			Validate(validateEdgeTarget(f, index)),
	)

	return fields
}

// validateEdgeTarget returns a validation function for the new-edge target
// input. It accepts an empty string (no new edge) or a valid UUID v4 that
// resolves to a node in the index. The newEdgeType on f is read at validation
// time so the check is skipped when type is "(none)".
func validateEdgeTarget(f *formPane, index types.GraphIndex) func(string) error {
	return func(s string) error {
		s = strings.TrimSpace(s)
		// No target — fine as long as edge type is also "(none)".
		if s == "" {
			if f.newEdgeType != "(none)" {
				return errors.New("a target node ID is required when an edge type is selected")
			}
			return nil
		}

		// Must be a valid UUID.
		if _, err := uuid.Parse(s); err != nil {
			return errors.New("must be a valid UUID (e.g. 550e8400-e29b-41d4-a716-446655440000)")
		}

		// Must refer to an existing node in the index.
		if index != nil {
			if _, err := index.GetNode(s); err != nil {
				return fmt.Errorf("node %s…%s not found in index", s[:4], s[len(s)-4:])
			}
		}

		return nil
	}
}

// Update forwards messages to the huh form and detects completion/abort.
func (f formPane) Update(msg tea.Msg) (PaneModel, tea.Cmd) {
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
		f.height = wmsg.Height - 4 - LogoHeight(f.width+2)
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
		node := f.buildNode()
		if err := f.store.WriteNode(node); err != nil {
			// Non-fatal — emit cancel so the pane is restored.
			return f, func() tea.Msg { return formCancelMsg{} }
		}
		if f.originalNode == nil && f.selectedNodeID != "" && f.linkToSelected {
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
		// Edge management: diff kept vs existing to delete unchecked edges,
		// then create new edge if specified.
		if f.originalNode != nil {
			f.applyEdgeChanges()
		}
		label := node.Types[0] + ": " + node.Title
		if node.Title == "" {
			label = node.Types[0] + ": " + node.Body
		}
		if len(label) > 40 {
			label = label[:37] + "…"
		}
		if f.originalNode != nil {
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

// applyEdgeChanges deletes unchecked existing edges and creates a new edge
// if the user filled in both the type and target fields. The target UUID is
// validated as an existing node before the write; invalid targets are silently
// skipped (huh validation should have caught them already, but defensive here).
func (f formPane) applyEdgeChanges() {
	// Build a set of kept edge IDs for fast lookup.
	kept := make(map[string]bool, len(f.keptEdgeIDs))
	for _, id := range f.keptEdgeIDs {
		kept[id] = true
	}

	// Delete any existing edge that was unchecked.
	for _, entry := range f.existingEdges {
		if !kept[entry.ID] {
			_ = f.store.DeleteEdge(entry.ID) // non-fatal
		}
	}

	// Create a new edge if both type and target are specified.
	target := strings.TrimSpace(f.newEdgeTarget)
	if f.newEdgeType == "(none)" || target == "" {
		return
	}

	// Validate the target UUID is well-formed before writing.
	if _, err := uuid.Parse(target); err != nil {
		return
	}

	// Confirm the target node exists in the index (defence-in-depth; the huh
	// validator should have already caught this).
	if f.index != nil {
		if _, err := f.index.GetNode(target); err != nil {
			return
		}
	}

	now := f.clock.Now()
	edge := &types.Edge{
		ID:      uuid.New().String(),
		Type:    f.newEdgeType,
		From:    f.editingID(),
		To:      target,
		Created: now,
	}
	_ = f.store.WriteEdge(edge) // non-fatal
}

// View renders the huh form, padded to the pane width and repainted so the
// primary background extends through every interior cell. PadLines squares
// each line to f.width (including the right-margin run); FillBackground then
// repaints the backgroundless padding cells that the bubbles viewport and
// huh's field separator emit inside the already-rendered string, which
// PadLines alone cannot reach.
func (f formPane) View() string {
	content := f.form.View()
	if content == "" {
		content = "Submitting…"
	}
	bg := f.theme.BgPrimary()
	return FillBackground(PadLines(content, f.width, bg), bg)
}

// KeyBindings returns the help hints shown in the command palette.
func (f formPane) KeyBindings() []KeyBinding {
	return []KeyBinding{
		{Key: "tab / shift+tab", Description: "Next / previous field"},
		{Key: "enter", Description: "Next field (submit on last)"},
		{Key: "alt+enter", Description: "New line in text field"},
		{Key: "ctrl+e", Description: "Open external editor"},
		{Key: "esc / ctrl+c", Description: "Cancel form"},
	}
}

// HandleFocusLost is a no-op for form panes.
func (f formPane) HandleFocusLost() tea.Cmd { return nil }

// isFormActive satisfies the formActivePane marker interface.
func (formPane) isFormActive() {}

// buildNode constructs a types.Node from the captured form field values.
// In edit mode it starts from a clone of the original node, so everything the
// form doesn't own (custom properties, spend_log, date sub-fields, kind/stage,
// source) is preserved; only form-owned fields are overwritten.
// applyKindStage stamps node.Kind/node.Stage according to the selected kind,
// honouring the CP.16 clone invariant:
//   - create (originalNode == nil): stamp Kind, initialise Stage to the group's
//     first stage.
//   - edit, kind unchanged: leave Kind/Stage untouched.
//   - edit, kind changed: re-stamp Kind; keep Stage if the new kind's group
//     contains it, else reset to the group's first stage.
func (f formPane) applyKindStage(node *types.Node) {
	if f.nodeKind == "" || f.kinds == nil {
		return
	}
	creating := f.originalNode == nil
	if !creating && f.nodeKind == f.originalNode.Kind {
		return // CP.16: unchanged kind leaves kind/stage untouched
	}
	k, ok := f.kinds.Lookup(f.nodeKind)
	if !ok {
		return
	}
	node.Kind = k.Name
	if f.stageGroups == nil {
		return
	}
	g, ok := f.stageGroups.Lookup(k.StageGroup)
	if !ok || len(g.Stages) == 0 {
		return
	}
	if creating {
		node.Stage = g.Stages[0]
		return
	}
	// Edit, changed kind: keep stage if valid in the new group, else reset.
	if !g.Contains(node.Stage) {
		node.Stage = g.Stages[0]
	}
}

func (f formPane) buildNode() *types.Node {
	now := f.clock.Now()

	node := f.originalNode.Clone() // nil in create mode (Clone is nil-safe)
	if node == nil {
		node = &types.Node{
			ID:      uuid.New().String(),
			Created: now,
		}
	}
	node.Title = f.title
	node.Body = f.body
	node.Modified = now
	node.Date.Created = node.Created
	node.Date.Modified = now
	if node.Properties == nil {
		node.Properties = make(map[string]interface{})
	}

	// Types are not editable in forms; only stamp the form's type when the
	// node doesn't already carry one (i.e. creation).
	switch f.kind {
	case formTask:
		if len(node.Types) == 0 {
			node.Types = []string{"task"}
		}
		if f.status != "" {
			node.Properties["status"] = f.status
		}
		if f.energy != "" {
			node.Properties["energy"] = f.energy
		}
		f.applyKindStage(node)

	case formJournal:
		if len(node.Types) == 0 {
			node.Types = []string{"journal"}
		}
		// Stamp About only on creation — an edit must not move the date the
		// entry is about.
		if f.originalNode == nil {
			node.Date.About = &now
		}
		f.applyKindStage(node)

	case formNote:
		if len(node.Types) == 0 {
			node.Types = []string{"note"}
		}
		f.applyKindStage(node)

	case formBudget:
		if len(node.Types) == 0 {
			node.Types = []string{"budget"}
		}
		node.Title = f.category
		node.Properties["category"] = f.category
		if alloc, err := strconv.ParseFloat(f.allocated, 64); err == nil {
			node.Properties["allocated"] = alloc
		}
		if warnAt, err := strconv.ParseFloat(f.warnAt, 64); err == nil {
			node.Properties["warn_at"] = warnAt
		} else {
			node.Properties["warn_at"] = 1.0
		}
		node.Properties["period"] = f.period
		f.applyKindStage(node)
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

// validateNonNegativeNumber returns a validation function that rejects empty,
// non-numeric, and negative values. Zero is allowed.
func validateNonNegativeNumber(fieldName string) func(string) error {
	return func(s string) error {
		if s == "" {
			return fmt.Errorf("%s is required", fieldName)
		}
		v, err := strconv.ParseFloat(s, 64)
		if err != nil {
			return fmt.Errorf("%s must be a number", fieldName)
		}
		if v < 0 {
			return fmt.Errorf("%s must not be negative", fieldName)
		}
		return nil
	}
}

// validateOptionalFraction returns a validation function that accepts blank
// (the field has a default) but otherwise requires a number in [0, 1].
func validateOptionalFraction(fieldName string) func(string) error {
	return func(s string) error {
		if s == "" {
			return nil
		}
		v, err := strconv.ParseFloat(s, 64)
		if err != nil {
			return fmt.Errorf("%s must be a number", fieldName)
		}
		if v < 0 || v > 1 {
			return fmt.Errorf("%s must be between 0 and 1", fieldName)
		}
		return nil
	}
}

// wyrdHuhTheme derives a huh.Theme from the active Wyrd theme so every form
// field, button, and the help footer use the Cairn palette. Every style carries
// both a foreground and the primary background to avoid background bleed when the
// form is padded in View.
//
// Important: huh.ThemeCharm copies Focused → Blurred before this function runs,
// so Blurred carries Charm colours at entry. Every Blurred field must be set
// explicitly here; edits to Focused do not propagate.
func wyrdHuhTheme(t *ActiveTheme) huh.Theme {
	if t == nil {
		return huh.ThemeFunc(func(isDark bool) *huh.Styles { return huh.ThemeCharm(isDark) })
	}

	return huh.ThemeFunc(func(isDark bool) *huh.Styles {
		base := huh.ThemeCharm(isDark)

		bg := t.BgPrimary()
		bg2 := t.BgSecondary()
		fg := t.FgPrimary()
		muted := t.FgMuted()
		accent := t.AccentPrimary()
		accent2 := t.AccentSecondary()
		errCol := t.OverflowCritical()

		// themed returns a fresh lipgloss style with the given foreground and the
		// primary background. Use for styles that carry no inherited metadata
		// (glyph strings, padding, border). For fields that do, use the chained
		// .Foreground(...).Background(...) form on the existing base.X.Y value.
		themed := func(fg color.Color) lipgloss.Style {
			return lipgloss.NewStyle().Foreground(fg).Background(bg)
		}

		// Form / group chrome.
		base.Form.Base = base.Form.Base.
			Background(bg).
			Foreground(fg)
		base.Group.Base = base.Group.Base.
			Background(bg)
		base.Group.Title = base.Group.Title.
			Foreground(accent).
			Background(bg)
		base.Group.Description = base.Group.Description.
			Foreground(muted).
			Background(bg)

		// Field separator (ThemeBase sets "\n\n" string; preserve it).
		base.FieldSeparator = base.FieldSeparator.Background(bg)

		// ── Focused ──────────────────────────────────────────────────────────
		// ThemeCharm sets Focused.Card = Focused.Base before our overrides, so
		// update Card explicitly after Base.
		base.Focused.Base = base.Focused.Base.
			Background(bg).
			BorderForeground(accent)
		base.Focused.Card = base.Focused.Card.
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
		base.Focused.NextIndicator = base.Focused.NextIndicator.
			Foreground(accent).
			Background(bg)
		base.Focused.PrevIndicator = base.Focused.PrevIndicator.
			Foreground(accent).
			Background(bg)
		// Multi-select: cursor purple, checked rows orange, unchecked normal/muted.
		base.Focused.MultiSelectSelector = base.Focused.MultiSelectSelector.
			Foreground(accent).
			Background(bg)
		base.Focused.SelectedOption = base.Focused.SelectedOption.
			Foreground(accent2).
			Background(bg)
		base.Focused.SelectedPrefix = base.Focused.SelectedPrefix.
			Foreground(accent2).
			Background(bg)
		base.Focused.UnselectedOption = base.Focused.UnselectedOption.
			Foreground(fg).
			Background(bg)
		base.Focused.UnselectedPrefix = base.Focused.UnselectedPrefix.
			Foreground(muted).
			Background(bg)
		// Text input.
		base.Focused.TextInput.Text = themed(fg)
		base.Focused.TextInput.Placeholder = themed(muted)
		base.Focused.TextInput.Cursor = themed(accent)
		base.Focused.TextInput.Prompt = themed(accent)
		// Buttons: active = accent fill, inactive = fg on secondary bg.
		base.Focused.FocusedButton = base.Focused.FocusedButton.
			Background(accent).
			Foreground(bg)
		base.Focused.Next = base.Focused.Next.
			Background(accent).
			Foreground(bg)
		base.Focused.BlurredButton = base.Focused.BlurredButton.
			Foreground(fg).
			Background(bg2)

		// ── Blurred ───────────────────────────────────────────────────────────
		// Blurred = recession: everything muted-on-bg. Errors stay red even when
		// a field is blurred so validation is never visually hidden.
		base.Blurred.Base = base.Blurred.Base.
			Background(bg)
		base.Blurred.Card = base.Blurred.Card.
			Background(bg)
		base.Blurred.Title = base.Blurred.Title.
			Foreground(muted).
			Background(bg)
		base.Blurred.Description = base.Blurred.Description.
			Foreground(muted).
			Background(bg)
		base.Blurred.ErrorIndicator = base.Blurred.ErrorIndicator.
			Foreground(errCol).
			Background(bg)
		base.Blurred.ErrorMessage = base.Blurred.ErrorMessage.
			Foreground(errCol).
			Background(bg)
		base.Blurred.SelectSelector = base.Blurred.SelectSelector.
			Foreground(muted).
			Background(bg)
		base.Blurred.Option = base.Blurred.Option.
			Foreground(muted).
			Background(bg)
		base.Blurred.MultiSelectSelector = base.Blurred.MultiSelectSelector.
			Foreground(muted).
			Background(bg)
		base.Blurred.SelectedOption = base.Blurred.SelectedOption.
			Foreground(muted).
			Background(bg)
		base.Blurred.SelectedPrefix = base.Blurred.SelectedPrefix.
			Foreground(muted).
			Background(bg)
		base.Blurred.UnselectedOption = base.Blurred.UnselectedOption.
			Foreground(muted).
			Background(bg)
		base.Blurred.UnselectedPrefix = base.Blurred.UnselectedPrefix.
			Foreground(muted).
			Background(bg)
		base.Blurred.TextInput.Text = themed(muted)
		base.Blurred.TextInput.Placeholder = themed(muted)
		base.Blurred.TextInput.Cursor = themed(muted)
		base.Blurred.TextInput.Prompt = themed(muted)
		base.Blurred.FocusedButton = base.Blurred.FocusedButton.
			Foreground(fg).
			Background(bg)
		base.Blurred.BlurredButton = base.Blurred.BlurredButton.
			Foreground(muted).
			Background(bg)

		// ── Help footer ───────────────────────────────────────────────────────
		// WithShowHelp(true) renders key hints below the form. Default styles are
		// foreground-only, causing a terminal-bg stripe inside the padded pane.
		// Keys get accent (matches the palette command-hint convention), everything
		// else muted; all carry bg.
		base.Help.ShortKey = themed(accent)
		base.Help.FullKey = themed(accent)
		base.Help.ShortDesc = themed(muted)
		base.Help.FullDesc = themed(muted)
		base.Help.ShortSeparator = themed(muted)
		base.Help.FullSeparator = themed(muted)
		base.Help.Ellipsis = themed(muted)

		return base
	})
}
