package tui

import (
	"strings"

	"charm.land/bubbles/v2/viewport"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/jasonwarrenuk/wyrd/internal/stage"
	"github.com/jasonwarrenuk/wyrd/internal/types"
)

// stagesOverlay is a modal overlay that lists all registered stage groups with
// their ordered stages and cycle behaviour. It is toggled via the bare :stages
// command in the palette (SL.12).
type stagesOverlay struct {
	active        bool
	vp            viewport.Model
	theme         *ActiveTheme
	stageGroups   *types.StageGroupRegistry
	width, height int
}

// newStagesOverlay creates an inactive stages overlay. groups may be nil.
func newStagesOverlay(theme *ActiveTheme, groups *types.StageGroupRegistry) stagesOverlay {
	return stagesOverlay{
		theme:       theme,
		stageGroups: groups,
	}
}

// Open populates the viewport with all registered stage groups and makes the
// overlay visible.
func (so *stagesOverlay) Open(width, height int) {
	so.active = true
	so.width = width
	so.height = height

	bg := so.theme.BgSecondary()

	mutedStyle := lipgloss.NewStyle().
		Foreground(so.theme.FgMuted()).
		Background(bg)

	primaryStyle := lipgloss.NewStyle().
		Foreground(so.theme.FgPrimary()).
		Background(bg)

	vpWidth := width * 3 / 4
	if vpWidth < 40 {
		vpWidth = 40
	}

	// Build content lines.
	var lines []string

	if so.stageGroups == nil || len(so.stageGroups.All()) == 0 {
		lines = append(lines, mutedStyle.Render("No stage groups"))
	} else {
		groups := so.stageGroups.All()

		// Build the default-name set for provenance marking. On error we get an
		// empty set, which means no (custom) markers — never blocks the view.
		defaultNames := map[string]bool{}
		if defaults, err := stage.DefaultStageGroups(); err == nil {
			for _, d := range defaults {
				defaultNames[d.Name] = true
			}
		}

		// Measure name column width (min 12, longest name + 2).
		nameColWidth := 12
		for _, g := range groups {
			if len(g.Name)+2 > nameColWidth {
				nameColWidth = len(g.Name) + 2
			}
		}

		// Fixed provenance column width — "(custom)" is 8 chars + 2 padding.
		const provenanceColWidth = 10

		// Measure cycle column width from the possible rendered strings.
		// Longest possible: "loop→<target> ↺" — measure all actual values.
		cycleColWidth := 12
		for _, g := range groups {
			cs := cycleString(g)
			if len(cs)+2 > cycleColWidth {
				cycleColWidth = len(cs) + 2
			}
		}

		for _, g := range groups {
			// Name column.
			nameSeg := primaryStyle.Render(g.Name)
			namePad := nameColWidth - len(g.Name)
			if namePad < 1 {
				namePad = 1
			}

			// Provenance column.
			var provSeg string
			provLen := 0
			if !defaultNames[g.Name] {
				provSeg = mutedStyle.Render("(custom)")
				provLen = len("(custom)")
			}
			provPad := provenanceColWidth - provLen
			if provPad < 1 {
				provPad = 1
			}

			// Cycle column.
			cs := cycleString(g)
			cycleSeg := mutedStyle.Render(cs)
			cyclePad := cycleColWidth - len(cs)
			if cyclePad < 1 {
				cyclePad = 1
			}

			// Stages column (trailing, no right-pad).
			stagesSeg := mutedStyle.Render(strings.Join(g.Stages, " → "))

			line := nameSeg +
				Spacer(namePad, bg) +
				provSeg +
				Spacer(provPad, bg) +
				cycleSeg +
				Spacer(cyclePad, bg) +
				stagesSeg

			lines = append(lines, line)
		}
	}

	// Size the viewport to the actual content line count, clamped so the box
	// never overflows the terminal. Chrome is 6 rows: border (2) + padding (2)
	// + title (1) + divider (1).
	vpHeight := overlayVPHeight(len(lines), height, 6)

	content := PadLines(strings.Join(lines, "\n"), vpWidth, bg)

	so.vp = viewport.New(viewport.WithWidth(vpWidth), viewport.WithHeight(vpHeight))
	if so.theme != nil {
		so.vp.Style = lipgloss.NewStyle().Background(bg)
	}
	so.vp.SetContent(content)
}

// cycleString returns the display string for a group's cycle behaviour.
func cycleString(g types.StageGroup) string {
	switch g.Cycle {
	case types.CycleLoop:
		return "loop ↺"
	case types.CycleLoopToStage:
		target := g.LoopTarget
		if target == "" {
			target = "?"
		}
		return "loop→" + target + " ↺"
	default:
		return "terminate"
	}
}

// Close hides the overlay.
func (so *stagesOverlay) Close() { so.active = false }

// IsActive reports whether the overlay is visible.
func (so *stagesOverlay) IsActive() bool { return so.active }

// Update handles keyboard input while the overlay is active.
func (so *stagesOverlay) Update(msg tea.Msg) (tea.Cmd, bool) {
	if !so.active {
		return nil, false
	}

	if keyMsg, ok := msg.(tea.KeyPressMsg); ok {
		switch keyMsg.String() {
		case "esc", "q":
			so.Close()
			return nil, true
		}
	}

	var cmd tea.Cmd
	so.vp, cmd = so.vp.Update(msg)
	return cmd, true
}

// View renders the overlay as a bordered box centred on the screen.
func (so *stagesOverlay) View(width, height int) string {
	if !so.active {
		return ""
	}

	bg := so.theme.BgSecondary()

	titleStyle := lipgloss.NewStyle().
		Foreground(so.theme.AccentPrimary()).
		Background(bg).
		Bold(true)

	boxWidth := width * 3 / 4
	if boxWidth < 40 {
		boxWidth = 40
	}

	boxStyle := lipgloss.NewStyle().
		Background(bg).
		BorderStyle(lipgloss.RoundedBorder()).
		BorderForeground(so.theme.AccentPrimary()).
		BorderBackground(bg).
		Padding(1, 2).
		Width(boxWidth)

	divStyle := lipgloss.NewStyle().Foreground(so.theme.Border()).Background(bg)

	var sb strings.Builder
	sb.WriteString(titleStyle.Render("STAGES"))
	sb.WriteString("\n")
	sb.WriteString(divStyle.Render(strings.Repeat("─", boxWidth-6)))
	sb.WriteString("\n")
	sb.WriteString(so.vp.View())

	return boxStyle.Render(sb.String())
}
