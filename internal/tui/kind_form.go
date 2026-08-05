package tui

import (
	"fmt"
	"regexp"
	"strings"
	"unicode/utf8"

	tea "charm.land/bubbletea/v2"
	huh "charm.land/huh/v2"
	"github.com/jasonwarrenuk/wyrd/internal/types"
)

// kindFormSubmitMsg is emitted by a kindFormPane when the user successfully
// creates a kind. The kind has already been written to kinds.jsonc.
type kindFormSubmitMsg struct {
	name string
}

// kindFormErrorMsg is emitted when a kind cannot be saved. The form closes
// and the reason is surfaced in the status bar; nothing is written.
type kindFormErrorMsg struct {
	err error
}

// hexColourPattern matches a plain 6-digit hex colour, e.g. "#9b70ff".
// Matches the convention used throughout the theme system.
var hexColourPattern = regexp.MustCompile(`^#[0-9a-fA-F]{6}$`)

// kindFormPane wraps a huh.Form for creating user-defined kinds. It satisfies
// both PaneModel and formActivePane. Like stageFormPane, it does not create a
// new node — it writes a Kind to the user kind registry (kinds.jsonc) in the
// store's parent directory.
//
// Unlike stageFormPane, the form is a single huh group: every field is
// always visible, there is no conditional second group.
type kindFormPane struct {
	form  *huh.Form
	store types.StoreFS
	theme *ActiveTheme

	// Field values — written by huh via pointer accessors.
	name       string // unique kind name
	glyph      string // single-rune display glyph
	colour     string // hex colour, e.g. "#9b70ff"
	stageGroup string // name of the stage group this kind progresses through

	width  int
	height int

	// done prevents double-emission of submit/cancel messages.
	done bool
}

// Compile-time checks: kindFormPane must satisfy PaneModel and formActivePane.
var _ PaneModel = kindFormPane{}
var _ formActivePane = kindFormPane{}

// isFormActive satisfies the formActivePane marker interface.
func (kindFormPane) isFormActive() {}

// NewKindFormPane builds a kindFormPane. Exported for use in tests.
func NewKindFormPane(
	theme *ActiveTheme,
	store types.StoreFS,
	kinds *types.KindRegistry,
	groups *types.StageGroupRegistry,
) PaneModel {
	return newKindFormPane(theme, store, kinds, groups)
}

// newKindFormPane builds a kindFormPane. kinds is the merged kind registry
// (baked-in defaults + existing user kinds); it is used to validate name
// collisions so the user cannot silently shadow a default or overwrite an
// existing custom kind. groups is the merged stage-group registry, used to
// populate the stage-group select.
func newKindFormPane(
	theme *ActiveTheme,
	store types.StoreFS,
	kinds *types.KindRegistry,
	groups *types.StageGroupRegistry,
) kindFormPane {
	f := kindFormPane{
		store:  store,
		theme:  theme,
		glyph:  "·",                   // mirrors the kindsOverlay blank-glyph fallback
		colour: theme.tier.FG.Primary, // mirrors the kindsOverlay blank-colour fallback
	}

	if groups != nil {
		if names := groups.Names(); len(names) > 0 {
			f.stageGroup = names[0]
		}
	}

	// Name validator: non-empty and not colliding with any existing kind name.
	validateName := func(s string) error {
		s = strings.TrimSpace(s)
		if s == "" {
			return fmt.Errorf("name is required")
		}
		if kinds != nil {
			if _, exists := kinds.Lookup(s); exists {
				return fmt.Errorf("%q already exists — choose a different name", s)
			}
		}
		return nil
	}

	// Glyph validator: exactly one rune (glyphs may be multi-byte, so count
	// runes rather than bytes).
	validateGlyph := func(s string) error {
		if utf8.RuneCountInString(s) != 1 {
			return fmt.Errorf("glyph must be exactly one character")
		}
		return nil
	}

	// Colour validator: plain 6-digit hex, matching the theme convention.
	validateColour := func(s string) error {
		if !hexColourPattern.MatchString(s) {
			return fmt.Errorf("colour must be a hex value like #9b70ff")
		}
		return nil
	}

	groupOptions := func() []huh.Option[string] {
		if groups == nil {
			return nil
		}
		names := groups.Names()
		opts := make([]huh.Option[string], len(names))
		for i, n := range names {
			opts[i] = huh.NewOption(n, n)
		}
		return opts
	}

	// Stage group validator: catches the empty-registry case at the field
	// itself, rather than leaving it to surface only from kind.Validate() at
	// submit time once every other field has already been filled in.
	validateStageGroup := func(s string) error {
		if s == "" {
			return fmt.Errorf("no stage groups available — create one with :stages new first")
		}
		return nil
	}

	group := huh.NewGroup(
		huh.NewInput().
			Title("Name").
			Description("Unique identifier for this kind (e.g. Errand)").
			Value(&f.name).
			Validate(validateName),

		huh.NewInput().
			Title("Glyph").
			Description("Single character shown beside nodes of this kind.").
			Value(&f.glyph).
			Validate(validateGlyph),

		huh.NewInput().
			Title("Colour").
			Description("Hex colour for the glyph, e.g. #9b70ff.").
			Value(&f.colour).
			Validate(validateColour),

		huh.NewSelect[string]().
			Title("Stage group").
			Description("The progression this kind's nodes move through.").
			Options(groupOptions()...).
			Value(&f.stageGroup).
			Validate(validateStageGroup),
	)

	f.form = huh.NewForm(group).
		WithTheme(wyrdHuhTheme(theme)).
		WithShowHelp(true)

	return f
}

// Update forwards messages to the huh form and detects completion/abort.
func (f kindFormPane) Update(msg tea.Msg) (PaneModel, tea.Cmd) {
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

		kind := types.Kind{
			Name:       strings.TrimSpace(f.name),
			StageGroup: f.stageGroup,
			Glyph:      f.glyph,
			Colour:     f.colour,
		}

		// Final structural validation — belt-and-braces after huh field validators.
		if err := kind.Validate(); err != nil {
			e := err
			return f, tea.Batch(cmd, func() tea.Msg { return kindFormErrorMsg{err: e} })
		}

		// Read existing user kinds. A failed read is treated as fatal rather
		// than silently falling back to an empty slice — WriteKinds overwrites
		// the whole file, so proceeding on an empty base would destroy every
		// existing user kind (and, worse, if we fell back to the merged
		// registry instead, would bake the eleven built-in defaults into the
		// user's kinds.jsonc). Surface the error and abort the write.
		reg, err := f.store.ReadKinds()
		if err != nil {
			e := fmt.Errorf("reading existing kinds: %w", err)
			return f, tea.Batch(cmd, func() tea.Msg { return kindFormErrorMsg{err: e} })
		}
		existing := append(reg.All(), kind)
		if err := f.store.WriteKinds(existing); err != nil {
			e := err
			return f, tea.Batch(cmd, func() tea.Msg { return kindFormErrorMsg{err: e} })
		}

		name := kind.Name
		return f, tea.Batch(cmd, func() tea.Msg {
			return kindFormSubmitMsg{name: name}
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
func (f kindFormPane) View() string {
	content := strings.TrimRight(f.form.View(), "\n")
	if content == "" {
		content = "Submitting…"
	}
	bg := f.theme.BgPrimary()
	return FillBackground(PadLines(content, f.width, bg), bg)
}

// KeyBindings returns the help hints shown in the status bar.
func (f kindFormPane) KeyBindings() []KeyBinding {
	return []KeyBinding{
		{Key: "tab / shift+tab", Description: "Next / previous field"},
		{Key: "enter", Description: "Next field (submit on last)"},
		{Key: "ctrl+c", Description: "Cancel form"},
	}
}

// HandleFocusLost is a no-op for kind form panes.
func (f kindFormPane) HandleFocusLost() tea.Cmd { return nil }
