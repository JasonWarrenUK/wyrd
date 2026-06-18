package tui

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	huh "charm.land/huh/v2"
	tea "charm.land/bubbletea/v2"
	"github.com/jasonwarrenuk/wyrd/internal/budget"
	"github.com/jasonwarrenuk/wyrd/internal/types"
)

// spendSubmitMsg is emitted by a spendFormPane when the user successfully
// records a spend entry. The entry has already been written to the store.
type spendSubmitMsg struct {
	category string
	amount   float64
}

// spendFormPane wraps a huh.Form for recording spend entries. It satisfies
// both PaneModel and formActivePane. Unlike formPane, it does not create a
// new node — it appends a SpendEntry to an existing budget node.
type spendFormPane struct {
	form  *huh.Form
	store types.StoreFS
	index types.GraphIndex
	clock types.Clock
	theme *ActiveTheme

	// Field values — written by huh via pointer accessors.
	category string // selected budget category
	amount   string // monetary amount (string input, parsed on submit)
	note     string // optional description
	date     string // optional explicit date (YYYY-MM-DD); empty means today

	width  int
	height int

	// done prevents double-emission of submit/cancel messages.
	done bool
}

// Compile-time checks: spendFormPane must satisfy PaneModel and formActivePane.
var _ PaneModel = spendFormPane{}
var _ formActivePane = spendFormPane{}

// isFormActive satisfies the formActivePane marker interface.
func (spendFormPane) isFormActive() {}

// NewSpendFormPane builds a spendFormPane. prefillNote is the text the user
// typed after the "s:" prefix in the capture bar (may be empty). Exported for
// use in tests.
//
// Returns an error when no budget category nodes exist in the store.
func NewSpendFormPane(
	theme *ActiveTheme,
	store types.StoreFS,
	index types.GraphIndex,
	clock types.Clock,
	prefillNote string,
) (PaneModel, error) {
	return newSpendFormPane(theme, store, index, clock, prefillNote)
}

// newSpendFormPane is the internal constructor.
func newSpendFormPane(
	theme *ActiveTheme,
	store types.StoreFS,
	index types.GraphIndex,
	clock types.Clock,
	prefillNote string,
) (spendFormPane, error) {
	f := spendFormPane{
		store: store,
		index: index,
		clock: clock,
		theme: theme,
		note:  prefillNote,
	}

	// Build category options from the budget nodes currently in the index.
	var opts []huh.Option[string]
	if index != nil {
		for _, n := range index.NodesByType("budget") {
			if n.Properties == nil {
				continue
			}
			cat, ok := n.Properties["category"].(string)
			if !ok || cat == "" {
				continue
			}
			label := cat
			if n.Title != "" && n.Title != cat {
				label = fmt.Sprintf("%s (%s)", cat, n.Title)
			}
			opts = append(opts, huh.NewOption(label, cat))
		}
	}
	if len(opts) == 0 {
		return spendFormPane{}, &types.NotFoundError{Kind: "budget category", ID: "(none)"}
	}

	f.category = opts[0].Value

	fields := []huh.Field{
		huh.NewSelect[string]().
			Title("Category").
			Options(opts...).
			Value(&f.category),

		huh.NewInput().
			Title("Amount").
			Value(&f.amount).
			Placeholder("0.00").
			Validate(validateAmount),

		huh.NewInput().
			Title("Note (optional)").
			Value(&f.note),

		huh.NewInput().
			Title("Date (optional)").
			Value(&f.date).
			Placeholder("YYYY-MM-DD — leave blank for today").
			Validate(validateDate),
	}

	f.form = huh.NewForm(
		huh.NewGroup(fields...),
	).WithTheme(wyrdHuhTheme(theme)).WithShowHelp(true)

	return f, nil
}

// validateDate accepts an empty string (meaning "use today") or a valid YYYY-MM-DD date.
func validateDate(s string) error {
	if s == "" {
		return nil
	}
	if _, err := time.Parse("2006-01-02", s); err != nil {
		return fmt.Errorf("date must be YYYY-MM-DD (e.g. 2026-01-15)")
	}
	return nil
}

// validateAmount rejects empty, non-numeric, zero, and negative values.
func validateAmount(s string) error {
	if s == "" {
		return fmt.Errorf("amount is required")
	}
	v, err := strconv.ParseFloat(s, 64)
	if err != nil {
		return fmt.Errorf("amount must be a number")
	}
	if v <= 0 {
		return fmt.Errorf("amount must be greater than zero")
	}
	return nil
}

// Update forwards messages to the huh form and detects completion/abort.
func (f spendFormPane) Update(msg tea.Msg) (PaneModel, tea.Cmd) {
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
		amount, err := strconv.ParseFloat(f.amount, 64)
		if err != nil || amount <= 0 {
			// Validation already ran, so this should not happen. Emit cancel
			// as a safe fallback.
			return f, func() tea.Msg { return formCancelMsg{} }
		}
		if err := budget.RecordSpend(f.store, f.index, f.category, amount, f.note, f.date, f.clock.Now()); err != nil {
			return f, func() tea.Msg { return formCancelMsg{} }
		}
		return f, tea.Batch(cmd, func() tea.Msg {
			return spendSubmitMsg{category: f.category, amount: amount}
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
func (f spendFormPane) View() string {
	content := strings.TrimRight(f.form.View(), "\n")
	if content == "" {
		content = "Submitting…"
	}
	bg := f.theme.BgPrimary()
	return FillBackground(PadLines(content, f.width, bg), bg)
}

// KeyBindings returns the help hints shown in the status bar.
func (f spendFormPane) KeyBindings() []KeyBinding {
	return []KeyBinding{
		{Key: "tab / shift+tab", Description: "Next / previous field"},
		{Key: "enter", Description: "Next field (submit on last)"},
		{Key: "ctrl+c", Description: "Cancel form"},
	}
}

// HandleFocusLost is a no-op for spend form panes.
func (f spendFormPane) HandleFocusLost() tea.Cmd { return nil }
