package tui

import (
	"fmt"
	"regexp"
	"strings"
	"unicode/utf8"

	tea "charm.land/bubbletea/v2"
	huh "charm.land/huh/v2"
	"github.com/jasonwarrenuk/wyrd/internal/stage"
	"github.com/jasonwarrenuk/wyrd/internal/types"
)

// kindFormSubmitMsg is emitted by a kindFormPane when the user successfully
// creates or edits a kind. The kind has already been written to kinds.jsonc.
// renamedFrom is non-empty when the submit renamed an existing kind — the
// mount handler uses it to trigger the whole-graph rename cascade
// (stage.RenameKind) before rebuilding the registry.
type kindFormSubmitMsg struct {
	name        string
	renamedFrom string
}

// kindFormErrorMsg is emitted when a kind cannot be saved. The form closes
// and the reason is surfaced in the status bar; nothing is written.
type kindFormErrorMsg struct {
	err error
}

// hexColourPattern matches a plain 6-digit hex colour, e.g. "#9b70ff".
// Matches the convention used throughout the theme system.
var hexColourPattern = regexp.MustCompile(`^#[0-9a-fA-F]{6}$`)

// kindFormPane wraps a huh.Form for creating or editing user-defined kinds.
// It satisfies both PaneModel and formActivePane. Like stageFormPane, it does
// not create a new node — it writes a Kind to the user kind registry
// (kinds.jsonc) in the store's parent directory.
//
// Unlike stageFormPane, the form is a single huh group: every field is
// always visible (plus an optional leading Note in edit mode — see below),
// there is no conditional second group.
type kindFormPane struct {
	form  *huh.Form
	store types.StoreFS
	theme *ActiveTheme

	// kinds is the merged registry, retained so the submit branch can run
	// the same collision check as the huh field validator — belt-and-braces
	// for a name that never passed through huh's own validation (e.g. a
	// test driving StateCompleted directly, matching the existing pattern
	// for the empty-stage-group case below).
	kinds *types.KindRegistry

	// Field values — written by huh via pointer accessors.
	name       string // unique kind name
	glyph      string // single-rune display glyph
	colour     string // hex colour, e.g. "#9b70ff"
	stageGroup string // name of the stage group this kind progresses through

	// originalName is set in edit mode to the kind's name at construction
	// time. Empty in create mode. Used at submit to decide replace-by-name
	// vs append (upsertKind) and to detect a rename (name != originalName).
	originalName string

	// editing mirrors the constructor's editing parameter — nil in create
	// mode, otherwise the entry being edited. Retained (rather than just
	// originalName) so the submit branch's belt-and-braces collision check
	// can reuse checkKindNameCollision, which needs the full types.Kind for
	// its name-exemption logic, not just the name string.
	editing *types.Kind

	// isDefault is true when originalName matches a baked-in default kind.
	// Drives the "overriding a built-in" warning Note and, on rename, the
	// tombstone-shadow handling (see kind_form.go's submit branch).
	isDefault bool

	width  int
	height int

	// done prevents double-emission of submit/cancel messages.
	done bool
}

// Compile-time checks: kindFormPane must satisfy PaneModel and formActivePane.
var _ PaneModel = kindFormPane{}
var _ formActivePane = kindFormPane{}
var _ formMountable = kindFormPane{}

// isFormActive satisfies the formActivePane marker interface.
func (kindFormPane) isFormActive() {}

// initForm satisfies the formMountable interface, returning the huh.Form's
// own init command so app.go's mountForm helper can start it uniformly
// across the five form panes that use that helper.
func (f kindFormPane) initForm() tea.Cmd {
	return f.form.Init()
}

// NewKindFormPane builds a kindFormPane in create mode. Exported for use in
// tests.
func NewKindFormPane(
	theme *ActiveTheme,
	store types.StoreFS,
	kinds *types.KindRegistry,
	groups *types.StageGroupRegistry,
) PaneModel {
	return newKindFormPane(theme, store, kinds, groups, nil)
}

// NewKindEditFormPane builds a kindFormPane in edit mode, pre-populated from
// existing. Exported for use in tests.
func NewKindEditFormPane(
	theme *ActiveTheme,
	store types.StoreFS,
	kinds *types.KindRegistry,
	groups *types.StageGroupRegistry,
	existing types.Kind,
) PaneModel {
	return newKindFormPane(theme, store, kinds, groups, &existing)
}

// newKindFormPane builds a kindFormPane. kinds is the merged kind registry
// (baked-in defaults + existing user kinds); it is used to validate name
// collisions so the user cannot silently shadow a default or overwrite an
// existing custom kind. groups is the merged stage-group registry, used to
// populate the stage-group select. editing is nil in create mode; when
// non-nil, the form seeds every field from *editing and switches submit from
// append to replace-by-name (see upsertKind).
func newKindFormPane(
	theme *ActiveTheme,
	store types.StoreFS,
	kinds *types.KindRegistry,
	groups *types.StageGroupRegistry,
	editing *types.Kind,
) kindFormPane {
	f := kindFormPane{
		store:  store,
		theme:  theme,
		kinds:  kinds,
		glyph:  "·",                   // mirrors the kindsOverlay blank-glyph fallback
		colour: theme.tier.FG.Primary, // mirrors the kindsOverlay blank-colour fallback
	}

	if groups != nil {
		if names := groups.Names(); len(names) > 0 {
			f.stageGroup = names[0]
		}
	}

	if editing != nil {
		// Seed before building the huh fields — huh binds by pointer, so the
		// fields below must be set to their final starting values first
		// (mirrors remap_form.go's f.choices[i] = orphan.Suggested ordering).
		f.name = editing.Name
		f.originalName = editing.Name
		f.editing = editing
		f.stageGroup = editing.StageGroup
		// A hand-edited kinds.jsonc entry can have a blank glyph/colour
		// (Kind.Validate only requires Name and StageGroup) — apply the same
		// fallbacks create mode uses rather than opening the form already
		// failing validateGlyph/validateColour on an unedited field.
		if editing.Glyph != "" {
			f.glyph = editing.Glyph
		}
		if editing.Colour != "" {
			f.colour = editing.Colour
		}

		if defaults, err := stage.DefaultKinds(); err == nil {
			for _, d := range defaults {
				if d.Name == editing.Name {
					f.isDefault = true
					break
				}
			}
		}
	}

	// Name validator: non-empty and not colliding (case-insensitively) with
	// any existing kind name — "Task" and "task" would otherwise coexist as
	// visually confusable kinds. In edit mode, the kind's own current name is
	// exempted from the collision check (exact-match removal — see
	// excludeName's doc comment for why this must not be case-insensitive).
	validateName := func(s string) error {
		s = strings.TrimSpace(s)
		if s == "" {
			return fmt.Errorf("name is required")
		}
		return checkKindNameCollision(s, kinds, editing)
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

	fields := []huh.Field{}

	// Overriding-a-built-in warning: only shown when editing a baked-in
	// default. huh.Note defaults to skip:true (excluded from tab order), so
	// this costs no extra keypress — it's purely informational.
	if f.isDefault {
		fields = append(fields, huh.NewNote().
			Title("Overriding a built-in").
			Description(fmt.Sprintf(
				"%q is built in. Saving writes a full copy to your kinds.jsonc "+
					"that permanently overrides it — including any future "+
					"improvements to the built-in version.", editing.Name)))
	}

	fields = append(fields,
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

	group := huh.NewGroup(fields...)
	if editing != nil {
		group = group.Title(fmt.Sprintf("Edit kind %q", editing.Name))
	}

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
		if err := checkKindNameCollision(kind.Name, f.kinds, f.editing); err != nil {
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

		// Stamp ShadowOf before upsertKind, which has no special cases and
		// stays that way.
		//
		// Re-editing an existing shadow must carry its ShadowOf forward
		// unchanged rather than recomputing against the current default:
		// recomputing would silently resolve any drift the user never
		// reviewed, resetting the TD.5 baseline and making drift detection
		// permanently blind to edits made between the original fork and now.
		//
		// Only a fresh fork of a still-unshadowed default gets a newly
		// computed hash, taken from the pre-edit default under its
		// originalName (not the post-edit kind) — hashing the post-edit
		// values would just record what's already on disk verbatim, which
		// tells TD.5 nothing.
		//
		// On rename, the shadow's new name matches no default and the stored
		// hash becomes uncomparable by name. That's accepted: a rename is a
		// deliberate detachment from the default it forked from.
		switch {
		case f.editing != nil && f.editing.ShadowOf != "":
			kind.ShadowOf = f.editing.ShadowOf
			kind.ShadowReason = f.editing.ShadowReason
			// TD.18b: carry the existing snapshot forward unchanged too —
			// same rule as ShadowOf. A re-edit must not silently drop the
			// combine form's old-default comparison the way recomputing
			// ShadowOf would silently resolve unreviewed drift.
			kind.ShadowSource = f.editing.ShadowSource
		case f.isDefault:
			kind.ShadowOf = stage.DefaultKindHash(f.originalName)
			kind.ShadowReason = types.ShadowEdited
			// TD.18b: snapshot the pre-edit default's content, not the
			// post-edit kind — hashing/storing the just-submitted values
			// would tell TD.18's combine form nothing about what changed.
			kind.ShadowSource = stage.DefaultKind(f.originalName)
		}

		renamed := f.originalName != "" && kind.Name != f.originalName
		existing := upsertKind(reg.All(), kind, f.originalName)

		// Renaming a shadowed default (or a not-yet-shadowed one) would
		// otherwise un-shadow the built-in under its old name — the embedded
		// copy reappears alongside the renamed entry, and every node still
		// holding the old kind name silently snaps back to the pristine
		// default. Write a tombstone: a shadow entry under the old name,
		// unchanged from the built-in, so it stays shadowed. RenameKind (run
		// after this write succeeds) then moves every node off the old name,
		// so the tombstone is inert — it exists purely to keep the registry
		// from resurrecting the default, not for anything to reference.
		//
		// A tombstone is by definition a verbatim shadow of a default, so it
		// gets stamped too — leaving it unstamped would make it read as
		// user-authored. Stamped as ShadowTombstone (TD.5), not the
		// ShadowEdited-equivalent empty value: a tombstone is
		// content-identical to the default it shadows at write time, so
		// TD.5's divergence detection distinguishes it from an ordinary
		// edited shadow rather than surfacing it as diverged the user never
		// consciously chose.
		if renamed && f.isDefault {
			if defaults, derr := stage.DefaultKinds(); derr == nil {
				for _, d := range defaults {
					if d.Name == f.originalName {
						d.ShadowOf = stage.DefaultKindHash(d.Name)
						d.ShadowReason = types.ShadowTombstone
						existing = append(existing, d)
						break
					}
				}
			}
		}

		if err := f.store.WriteKinds(existing); err != nil {
			e := err
			return f, tea.Batch(cmd, func() tea.Msg { return kindFormErrorMsg{err: e} })
		}

		name := kind.Name
		renamedFrom := ""
		if renamed {
			renamedFrom = f.originalName
		}
		return f, tea.Batch(cmd, func() tea.Msg {
			return kindFormSubmitMsg{name: name, renamedFrom: renamedFrom}
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

// checkKindNameCollision applies the same collision rule the huh field
// validator uses, callable as a belt-and-braces check at submit time (huh's
// field validators only run when its own state machine processes field
// advancement — a caller that forces StateCompleted directly, as some tests
// and any future programmatic submit path might, bypasses them entirely).
// editing is nil in create mode; when non-nil its name is exempted from the
// collision set.
func checkKindNameCollision(name string, kinds *types.KindRegistry, editing *types.Kind) error {
	// A case-only rename must be checked against the original name directly,
	// before exemption: excludeName removes the original name from the
	// collision set entirely, so by the time caseInsensitiveNameCollision
	// runs there is nothing left for a case-only variant to collide with —
	// the exemption would otherwise silently swallow the exact case this
	// check exists to catch.
	if editing != nil && name != editing.Name && strings.EqualFold(name, editing.Name) {
		return fmt.Errorf("changing only the capitalisation of %q is not supported", editing.Name)
	}
	if kinds == nil {
		return nil
	}
	names := kinds.Names()
	if editing != nil {
		names = excludeName(names, editing.Name)
	}
	if !caseInsensitiveNameCollision(name, names) {
		return nil
	}
	return fmt.Errorf("%q already exists — choose a different name", name)
}

// upsertKind returns entries with the kind matching originalName replaced in
// place, or kind appended when no entry matches. originalName is "" in
// create mode, which always appends. Matching is exact — the registry keys
// entries by exact name.
//
// Three distinct situations collapse into "found → replace, not found →
// append" with no special-casing needed:
//   - editing a user-defined kind: originalName is in entries → replace at
//     the same index, preserving display order (a remove-then-append would
//     silently reshuffle the kinds overlay on every edit)
//   - editing a baked-in default not yet shadowed: originalName is a default
//     name, not a user entry → not found → append. MergeKinds places user
//     entries after defaults and NewKindRegistry's order tracks first
//     insertion, so the shadow keeps the default's display position
//   - editing an already-shadowed default: originalName is in entries (the
//     prior shadow) → replace
func upsertKind(entries []types.Kind, kind types.Kind, originalName string) []types.Kind {
	if originalName != "" {
		for i, e := range entries {
			if e.Name == originalName {
				out := make([]types.Kind, len(entries))
				copy(out, entries)
				out[i] = kind
				return out
			}
		}
	}
	return append(entries, kind)
}

// excludeName returns names with candidate removed (exact match only — not
// case-insensitive). Used to exempt a kind's own current name from the
// collision validator in edit mode.
//
// Exact match is deliberate: case-insensitive removal would let a rename
// like "Task" -> "task" pass the collision check (since "task" no longer
// collides with the exempted "Task"), while KindRegistry keys entries
// exactly — producing precisely the visually-confusable pair the collision
// validator exists to prevent, arrived at through the back door. With exact
// removal, "task" still collides with "Task" in the name list, and the
// validator's EqualFold branch reports the clearer "capitalisation only" error.
func excludeName(names []string, candidate string) []string {
	out := make([]string, 0, len(names))
	for _, n := range names {
		if n != candidate {
			out = append(out, n)
		}
	}
	return out
}
