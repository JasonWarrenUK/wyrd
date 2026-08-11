package tui

import (
	"strings"

	"charm.land/bubbles/v2/viewport"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/jasonwarrenuk/wyrd/internal/stage"
	"github.com/jasonwarrenuk/wyrd/internal/types"
)

// kindsOverlay is a modal overlay that lists all registered kinds with their
// glyph, colour, stage-group name, and ordered stages. It is toggled via the
// :kinds command in the palette (SL.9).
type kindsOverlay struct {
	active      bool
	vp          viewport.Model
	theme       *ActiveTheme
	kinds       *types.KindRegistry
	stageGroups *types.StageGroupRegistry

	width, height int
}

// newKindsOverlay creates an inactive kinds overlay. Both registries may be
// nil. The (custom)/(edited) provenance marker (see provenanceMarker's doc
// comment) is driven directly by kinds.IsUserDefined (TD.15) rather than a
// separately-threaded userNames map — a nil kinds registry renders no
// markers, matching the old nil-userNames behaviour.
func newKindsOverlay(theme *ActiveTheme, kinds *types.KindRegistry, groups *types.StageGroupRegistry) kindsOverlay {
	return kindsOverlay{
		theme:       theme,
		kinds:       kinds,
		stageGroups: groups,
	}
}

// Open populates the viewport with all registered kinds and makes the overlay
// visible.
func (ko *kindsOverlay) Open(width, height int) {
	ko.active = true
	ko.width = width
	ko.height = height

	bg := ko.theme.BgSecondary()

	mutedStyle := lipgloss.NewStyle().
		Foreground(ko.theme.FgMuted()).
		Background(bg)

	primaryStyle := lipgloss.NewStyle().
		Foreground(ko.theme.FgPrimary()).
		Background(bg)

	vpWidth := width * 3 / 4
	if vpWidth < 40 {
		vpWidth = 40
	}

	// Build content lines.
	var lines []string

	if ko.kinds == nil || len(ko.kinds.All()) == 0 {
		lines = append(lines, mutedStyle.Render("No kinds registered"))
	} else {
		kinds := ko.kinds.All()

		// Build the default-name set for provenance marking, mirroring
		// stagesOverlay. On error we get an empty set, which means no
		// markers at all — never blocks the view.
		defaultNames := map[string]bool{}
		if defaults, err := stage.DefaultKinds(); err == nil {
			for _, d := range defaults {
				defaultNames[d.Name] = true
			}
		}

		// TD.5: recomputed fresh on every Open rather than threaded through
		// the constructor — ko.kinds/ko.stageGroups are always current by
		// the time Open runs (every rebuild site re-points them before the
		// overlay is next opened), so there's no staleness risk, and this
		// avoids reintroducing the exact kind of parallel side-channel
		// TD.15 just removed.
		divergedNames := map[string]bool{}
		for _, d := range stage.DetectDiverged(ko.kinds, ko.stageGroups).Diverged {
			if d.Kind {
				divergedNames[d.Name] = true
			}
		}

		// Measure name column width for alignment (min 12, max longest name +
		// 2). Use lipgloss.Width so multi-byte runes (e.g. CJK) are counted by
		// cell width, not byte length.
		nameColWidth := 12
		for _, k := range kinds {
			if w := lipgloss.Width(k.Name) + 2; w > nameColWidth {
				nameColWidth = w
			}
		}

		// Provenance column width: the widest possible marker is "(diverged)"
		// at 10 display cells + 2 padding. "(edited)"/"(custom)" (8 cells)
		// pad out to the same width.
		const provenanceColWidth = 12

		// Measure stage-group column width similarly.
		groupColWidth := 12
		for _, k := range kinds {
			if w := lipgloss.Width(k.StageGroup) + 2; w > groupColWidth {
				groupColWidth = w
			}
		}

		for _, k := range kinds {
			// Glyph in the kind's own colour; fall back to FgPrimary when blank.
			glyphColour := ko.theme.FgPrimary()
			if k.Colour != "" {
				glyphColour = lipgloss.Color(k.Colour)
			}
			glyphStyle := lipgloss.NewStyle().
				Foreground(glyphColour).
				Background(bg)

			glyph := k.Glyph
			if glyph == "" {
				glyph = "·"
			}

			// Name column, padded to nameColWidth with Spacer.
			nameSeg := primaryStyle.Render(k.Name)
			namePad := nameColWidth - lipgloss.Width(k.Name)
			if namePad < 1 {
				namePad = 1
			}

			// Provenance column. A diverged entry is necessarily also an
			// edited shadow (only shadowed entries can diverge), so
			// (diverged) takes priority over (edited) rather than the two
			// stacking — it's the more actionable state to surface.
			marker := provenanceMarker(k.Name, ko.kinds.IsUserDefined, defaultNames)
			if divergedNames[k.Name] {
				marker = "(diverged)"
			}
			provSeg := mutedStyle.Render(marker)
			provPad := provenanceColWidth - lipgloss.Width(marker)
			if provPad < 1 {
				provPad = 1
			}

			// Stage-group name column, padded to groupColWidth.
			groupSeg := mutedStyle.Render(k.StageGroup)
			groupPad := groupColWidth - lipgloss.Width(k.StageGroup)
			if groupPad < 1 {
				groupPad = 1
			}

			// Stages: resolve group and join with arrows.
			var stagesSeg string
			if ko.stageGroups != nil && k.StageGroup != "" {
				if g, ok := ko.stageGroups.Lookup(k.StageGroup); ok {
					stageStr := strings.Join(g.Stages, " → ")
					if g.Cycle == types.CycleLoop || g.Cycle == types.CycleLoopToStage {
						stageStr += " ↺"
					}
					stagesSeg = mutedStyle.Render(stageStr)
				}
			}

			line := glyphStyle.Render(glyph) +
				Spacer(1, bg) +
				nameSeg +
				Spacer(namePad, bg) +
				provSeg +
				Spacer(provPad, bg) +
				groupSeg +
				Spacer(groupPad, bg) +
				stagesSeg

			lines = append(lines, line)
		}
	}

	// Size the viewport to the actual content line count, clamped so the box
	// never overflows the terminal. Chrome is 6 rows: border (2) + padding (2)
	// + title (1) + divider (1).
	vpHeight := overlayVPHeight(len(lines), height, 6)

	content := PadLines(strings.Join(lines, "\n"), vpWidth, bg)

	ko.vp = viewport.New(viewport.WithWidth(vpWidth), viewport.WithHeight(vpHeight))
	if ko.theme != nil {
		ko.vp.Style = lipgloss.NewStyle().Background(bg)
	}
	ko.vp.SetContent(content)
}

// Close hides the overlay.
func (ko *kindsOverlay) Close() { ko.active = false }

// IsActive reports whether the overlay is visible.
func (ko *kindsOverlay) IsActive() bool { return ko.active }

// Update handles keyboard and mouse input while the overlay is active. Any
// other message type (ritual ticks, window resizes, spinner ticks, …) is
// declined with (nil, false) so it falls through to the root switch — see
// keyOverlay's doc comment for why that matters.
func (ko *kindsOverlay) Update(msg tea.Msg) (tea.Cmd, bool) {
	if !ko.active {
		return nil, false
	}

	if keyMsg, ok := msg.(tea.KeyPressMsg); ok {
		switch keyMsg.String() {
		case "esc", "q":
			ko.Close()
			return nil, true
		}
	}

	// Mouse messages fall through to the viewport too, so wheel scroll keeps
	// working; everything else (ticks, resize, …) is declined below.
	switch msg.(type) {
	case tea.KeyPressMsg, tea.MouseMsg:
		var cmd tea.Cmd
		ko.vp, cmd = ko.vp.Update(msg)
		return cmd, true
	default:
		return nil, false
	}
}

// Compile-time check: kindsOverlay must satisfy keyOverlay.
var _ keyOverlay = (*kindsOverlay)(nil)

// View renders the overlay as a bordered box centred on the screen.
func (ko *kindsOverlay) View(width, height int) string {
	if !ko.active {
		return ""
	}

	bg := ko.theme.BgSecondary()

	titleStyle := lipgloss.NewStyle().
		Foreground(ko.theme.AccentPrimary()).
		Background(bg).
		Bold(true)

	boxWidth := width * 3 / 4
	if boxWidth < 40 {
		boxWidth = 40
	}

	boxStyle := lipgloss.NewStyle().
		Background(bg).
		BorderStyle(lipgloss.RoundedBorder()).
		BorderForeground(ko.theme.AccentPrimary()).
		BorderBackground(bg).
		Padding(1, 2).
		Width(boxWidth)

	divStyle := lipgloss.NewStyle().Foreground(ko.theme.Border()).Background(bg)

	var sb strings.Builder
	sb.WriteString(titleStyle.Render("KINDS"))
	sb.WriteString("\n")
	sb.WriteString(divStyle.Render(strings.Repeat("─", boxWidth-6)))
	sb.WriteString("\n")
	sb.WriteString(ko.vp.View())

	return boxStyle.Render(sb.String())
}
