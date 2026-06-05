package tui

import (
	"strings"

	"charm.land/bubbles/v2/viewport"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
)

// helpOverlay is a modal overlay that lists all active key bindings.
// It is toggled via the :help command in the palette.
type helpOverlay struct {
	active        bool
	vp            viewport.Model
	theme         *ActiveTheme
	width, height int
}

// newHelpOverlay creates an inactive help overlay.
func newHelpOverlay(theme *ActiveTheme) helpOverlay {
	return helpOverlay{theme: theme}
}

// Open populates the viewport with key bindings and makes the overlay visible.
func (ho *helpOverlay) Open(width, height int, bindings []KeyBinding) {
	ho.active = true
	ho.width = width
	ho.height = height

	var sb strings.Builder
	for _, b := range bindings {
		sb.WriteString(b.Key)
		sb.WriteString("\t")
		sb.WriteString(b.Description)
		sb.WriteString("\n")
	}

	vpWidth := width * 3 / 4
	vpHeight := height - 6
	if vpWidth < 40 {
		vpWidth = 40
	}
	if vpHeight < 5 {
		vpHeight = 5
	}

	ho.vp = viewport.New(viewport.WithWidth(vpWidth), viewport.WithHeight(vpHeight))
	if ho.theme != nil {
		ho.vp.Style = lipgloss.NewStyle().Background(ho.theme.BgSecondary())
	}
	ho.vp.SetContent(sb.String())
}

// Close hides the overlay.
func (ho *helpOverlay) Close() { ho.active = false }

// IsActive reports whether the overlay is visible.
func (ho *helpOverlay) IsActive() bool { return ho.active }

// Update handles keyboard input while the overlay is active.
func (ho *helpOverlay) Update(msg tea.Msg) (tea.Cmd, bool) {
	if !ho.active {
		return nil, false
	}

	if keyMsg, ok := msg.(tea.KeyPressMsg); ok {
		switch keyMsg.String() {
		case "esc", "q":
			ho.Close()
			return nil, true
		}
	}

	var cmd tea.Cmd
	ho.vp, cmd = ho.vp.Update(msg)
	return cmd, true
}

// View renders the overlay as a bordered box centred on the screen.
func (ho *helpOverlay) View(width, height int) string {
	if !ho.active {
		return ""
	}

	bg := ho.theme.BgSecondary()

	titleStyle := lipgloss.NewStyle().
		Foreground(ho.theme.AccentPrimary()).
		Background(bg).
		Bold(true)

	boxWidth := width * 3 / 4
	if boxWidth < 40 {
		boxWidth = 40
	}

	boxStyle := lipgloss.NewStyle().
		Background(bg).
		BorderStyle(lipgloss.RoundedBorder()).
		BorderForeground(ho.theme.AccentPrimary()).
		BorderBackground(bg).
		Padding(1, 2).
		Width(boxWidth)

	divStyle := lipgloss.NewStyle().Foreground(ho.theme.Border()).Background(bg)

	var sb strings.Builder
	sb.WriteString(titleStyle.Render("KEY BINDINGS"))
	sb.WriteString("\n")
	sb.WriteString(divStyle.Render(strings.Repeat("─", boxWidth-6)))
	sb.WriteString("\n")
	sb.WriteString(ho.vp.View())

	return boxStyle.Render(sb.String())
}
