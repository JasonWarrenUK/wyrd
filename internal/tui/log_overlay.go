package tui

import (
	"os"
	"path/filepath"
	"strings"

	"charm.land/bubbles/v2/viewport"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
)

// logOverlay is a modal overlay that displays the tail of the debug log file.
// It is toggled via the `:log` command in the palette.
type logOverlay struct {
	active        bool
	vp            viewport.Model
	theme         *ActiveTheme
	width, height int
}

// newLogOverlay creates an inactive log overlay.
func newLogOverlay(theme *ActiveTheme) logOverlay {
	return logOverlay{
		theme: theme,
	}
}

// logFilePath returns the path to the wyrd log file (~/.wyrd/wyrd.log).
func overlayLogFilePath() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return filepath.Join(".", "wyrd.log")
	}
	return filepath.Join(home, ".wyrd", "wyrd.log")
}

// Open reads the last 100 lines from the log file and displays them in the
// viewport. The overlay becomes active.
func (lo *logOverlay) Open(width, height int) {
	lo.active = true
	lo.width = width
	lo.height = height

	content := lo.readLogTail(100)

	vpWidth := width * 3 / 4
	vpHeight := height - 6 // borders + title + padding
	if vpWidth < 40 {
		vpWidth = 40
	}
	if vpHeight < 5 {
		vpHeight = 5
	}

	lo.vp = viewport.New(viewport.WithWidth(vpWidth), viewport.WithHeight(vpHeight))
	if lo.theme != nil {
		lo.vp.Style = lipgloss.NewStyle().Background(lo.theme.BgSecondary())
	}
	lo.vp.SetContent(content)
	// Scroll to the bottom so the most recent entries are visible.
	lo.vp.GotoBottom()
}

// Close hides the overlay.
func (lo *logOverlay) Close() {
	lo.active = false
}

// IsActive reports whether the overlay is currently visible.
func (lo *logOverlay) IsActive() bool {
	return lo.active
}

// Update handles keyboard input while the overlay is active.
// Returns the updated overlay, an optional tea.Cmd, and whether the event
// was consumed (true = consumed, caller should not route further).
func (lo *logOverlay) Update(msg tea.Msg) (tea.Cmd, bool) {
	if !lo.active {
		return nil, false
	}

	if keyMsg, ok := msg.(tea.KeyPressMsg); ok {
		switch keyMsg.String() {
		case "esc", "q":
			lo.Close()
			return nil, true
		}
	}

	var cmd tea.Cmd
	lo.vp, cmd = lo.vp.Update(msg)
	return cmd, true
}

// View renders the overlay as a bordered box centred on the screen.
func (lo *logOverlay) View(width, height int) string {
	if !lo.active {
		return ""
	}

	bg := lo.theme.BgSecondary()

	titleStyle := lipgloss.NewStyle().
		Foreground(lo.theme.AccentPrimary()).
		Background(bg).
		Bold(true)

	boxWidth := width * 3 / 4
	if boxWidth < 40 {
		boxWidth = 40
	}

	boxStyle := lipgloss.NewStyle().
		Background(bg).
		BorderStyle(lipgloss.RoundedBorder()).
		BorderForeground(lo.theme.AccentPrimary()).
		BorderBackground(bg).
		Padding(1, 2).
		Width(boxWidth)

	var sb strings.Builder
	sb.WriteString(titleStyle.Render("DEBUG LOG"))
	sb.WriteString("\n")

	divStyle := lipgloss.NewStyle().Foreground(lo.theme.Border()).Background(bg)
	sb.WriteString(divStyle.Render(strings.Repeat("─", boxWidth-6)))
	sb.WriteString("\n")
	sb.WriteString(lo.vp.View())

	return boxStyle.Render(sb.String())
}

// readLogTail reads the last n lines from the log file.
func (lo *logOverlay) readLogTail(n int) string {
	logPath := overlayLogFilePath()
	data, err := os.ReadFile(logPath)
	if err != nil {
		return "(no log file found at " + logPath + ")"
	}

	lines := strings.Split(string(data), "\n")

	// Remove trailing empty line from final newline.
	if len(lines) > 0 && lines[len(lines)-1] == "" {
		lines = lines[:len(lines)-1]
	}

	if len(lines) > n {
		lines = lines[len(lines)-n:]
	}

	if len(lines) == 0 {
		return "(log file is empty)"
	}

	return strings.Join(lines, "\n")
}
