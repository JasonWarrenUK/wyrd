package tui

import (
	"fmt"
	"image/color"
	"strings"

	"charm.land/bubbles/v2/textinput"
	"charm.land/bubbles/v2/viewport"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/jasonwarrenuk/wyrd/internal/tui/ritual"
	"github.com/jasonwarrenuk/wyrd/internal/types"
)

// ritualOverlay is the TUI modal that wraps a ritual.Runner, presenting each
// step to the user in a bordered overlay box similar to the command palette.
type ritualOverlay struct {
	active    bool
	runner    *ritual.Runner
	width     int
	height    int
	vp        viewport.Model
	textInput textinput.Model
	theme     *ActiveTheme

	// stepTracker tracks the current step index separately since the runner
	// does not expose it. Incremented on each advance.
	stepTracker int
}

// Open activates the overlay, starts the runner, and initialises the viewport
// and text input components.
func (o *ritualOverlay) Open(runner *ritual.Runner, theme *ActiveTheme, w, h int) error {
	o.active = true
	o.runner = runner
	o.theme = theme
	o.width = w
	o.height = h
	o.stepTracker = 0

	// Initialise viewport for scrollable step content.
	vpWidth := o.boxInnerWidth()
	vpHeight := o.boxInnerHeight()
	o.vp = viewport.New(viewport.WithWidth(vpWidth), viewport.WithHeight(vpHeight))
	o.vp.Style = lipgloss.NewStyle().Background(theme.BgSecondary())

	// Initialise text input for prompt steps.
	o.textInput = textinput.New()
	o.textInput.Prompt = "> "
	o.textInput.CharLimit = 512

	if err := runner.Start(); err != nil {
		o.active = false
		return err
	}

	// Focus text input if the first step is a prompt.
	if step := o.currentStep(); step != nil && step.Type == types.StepPrompt {
		o.textInput.Focus()
	}

	o.syncViewport()
	return nil
}

// Close deactivates the overlay.
func (o *ritualOverlay) Close() {
	o.active = false
	o.textInput.Blur()
}

// IsActive reports whether the overlay is currently visible.
func (o *ritualOverlay) IsActive() bool {
	return o.active
}

// Update handles a Bubble Tea message while the overlay is active. It returns
// any command to execute and whether the message was consumed (true) or should
// fall through to the app (false).
func (o *ritualOverlay) Update(msg tea.Msg) (tea.Cmd, bool) {
	if !o.active || o.runner == nil {
		return nil, false
	}

	keyMsg, isKey := msg.(tea.KeyPressMsg)
	if !isKey {
		// Non-key messages are not consumed by the overlay.
		return nil, false
	}

	step := o.currentStep()
	if step == nil {
		o.Close()
		return nil, true
	}

	// Nudge rituals can be dismissed with Escape.
	if keyMsg.String() == "esc" && !o.runner.IsGate() {
		if o.runner.TryDefer() {
			o.Close()
			return nil, true
		}
	}

	// Route input based on step type.
	switch step.Type {
	case types.StepPrompt:
		return o.handlePromptKey(keyMsg)
	case types.StepQueryList:
		return o.handleListKey(keyMsg)
	default:
		return o.handleGenericKey(keyMsg)
	}
}

// handlePromptKey routes keyboard input to the text input for prompt steps.
// Enter submits the prompt text to the runner.
func (o *ritualOverlay) handlePromptKey(msg tea.KeyPressMsg) (tea.Cmd, bool) {
	switch msg.String() {
	case "enter":
		text := strings.TrimSpace(o.textInput.Value())
		if text == "" {
			return nil, true
		}
		if err := o.runner.SubmitPrompt(text); err != nil {
			// Non-fatal: the runner logs internally.
			_ = err
		}
		o.textInput.Reset()
		o.stepTracker++
		return o.afterRunnerAction()

	default:
		var cmd tea.Cmd
		o.textInput, cmd = o.textInput.Update(msg)
		return cmd, true
	}
}

// handleListKey forwards list navigation keys to the runner for query_list
// steps: j/k for movement, s for status cycle, x for archive, e for edge,
// enter to advance.
func (o *ritualOverlay) handleListKey(msg tea.KeyPressMsg) (tea.Cmd, bool) {
	key := msg.String()

	hint, err := o.runner.HandleKey(key)
	if err != nil {
		_ = err
	}

	return o.processHint(hint)
}

// handleGenericKey forwards any key to the runner for summary, view, action,
// and gate steps. Enter and space advance; the defer sequence is checked
// internally by the runner for gate rituals.
func (o *ritualOverlay) handleGenericKey(msg tea.KeyPressMsg) (tea.Cmd, bool) {
	key := msg.String()

	// Map Escape to "Esc" for the runner's defer-sequence detection.
	if key == "esc" {
		key = "Esc"
	}

	hint, err := o.runner.HandleKey(key)
	if err != nil {
		_ = err
	}

	return o.processHint(hint)
}

// processHint reacts to the action hint returned by runner.HandleKey().
func (o *ritualOverlay) processHint(hint string) (tea.Cmd, bool) {
	switch hint {
	case "advance":
		if err := o.runner.Advance(); err != nil {
			_ = err
		}
		o.stepTracker++
		return o.afterRunnerAction()

	case "deferred":
		o.Close()
		return nil, true

	case "redraw":
		o.syncViewport()
		return nil, true

	default:
		return nil, true
	}
}

// afterRunnerAction checks the runner state after an advance or submit.
// If the ritual is complete or deferred, the overlay closes.
func (o *ritualOverlay) afterRunnerAction() (tea.Cmd, bool) {
	switch o.runner.CurrentState() {
	case ritual.StateComplete, ritual.StateDeferred:
		o.Close()
		return nil, true
	default:
		o.syncViewport()
		// Focus text input if the new step is a prompt.
		if step := o.currentStep(); step != nil && step.Type == types.StepPrompt {
			o.textInput.Focus()
		}
		return nil, true
	}
}

// syncViewport rebuilds the viewport content from the runner's current state.
func (o *ritualOverlay) syncViewport() {
	bg := o.theme.BgSecondary()
	w := o.boxInnerWidth()

	var content string
	step := o.currentStep()
	if step == nil {
		content = ""
	} else {
		switch step.Type {
		case types.StepQueryList:
			content = o.renderList(w, bg)
		case types.StepPrompt:
			content = o.renderPrompt(step, bg)
		default:
			content = o.runner.CurrentOutput()
		}
	}

	padded := PadLines(content, w, bg)
	o.vp.SetContent(padded)
}

// renderList builds the display string for a query_list step, showing each
// item with a cursor indicator and any pending status changes.
func (o *ritualOverlay) renderList(width int, bg color.Color) string {
	items := o.runner.CurrentItems()
	if len(items) == 0 {
		emptyStyle := lipgloss.NewStyle().
			Background(bg).
			Foreground(o.theme.FgMuted())
		return emptyStyle.Render(o.runner.CurrentOutput())
	}

	selected := o.runner.SelectedIndex()
	var sb strings.Builder

	// Label line.
	labelStyle := lipgloss.NewStyle().
		Background(bg).
		Foreground(o.theme.AccentPrimary()).
		Bold(true)
	sb.WriteString(labelStyle.Render(o.runner.CurrentOutput()))
	sb.WriteString("\n")

	for i, item := range items {
		rowBg := bg
		if i == selected {
			rowBg = o.theme.Selection()
		}

		cursorText := "  "
		if i == selected {
			cursorText = "> "
		}
		cursorStyle := lipgloss.NewStyle().
			Background(rowBg).
			Foreground(o.theme.AccentPrimary())

		// Display: title or body from the row data.
		text := ritual.FormatListRow(item, nil)
		if item.Archived {
			text = "✗ " + text
		} else if item.PendingStatus != "" {
			text = "[" + item.PendingStatus + "] " + text
		}

		textStyle := lipgloss.NewStyle().
			Background(rowBg).
			Foreground(o.theme.FgPrimary()).
			Width(width - 4).
			MaxWidth(width - 4)
		if i == selected {
			textStyle = textStyle.Bold(true)
		}

		sb.WriteString(cursorStyle.Render(cursorText))
		sb.WriteString(textStyle.Render(text))
		sb.WriteString("\n")
	}

	return sb.String()
}

// renderPrompt builds the display for a prompt step: the step label followed
// by the text input widget.
func (o *ritualOverlay) renderPrompt(step *types.RitualStep, bg color.Color) string {
	var sb strings.Builder

	labelStyle := lipgloss.NewStyle().
		Background(bg).
		Foreground(o.theme.FgPrimary())
	sb.WriteString(labelStyle.Render(step.Text))
	sb.WriteString("\n\n")

	inputStyle := lipgloss.NewStyle().
		Background(bg).
		Foreground(o.theme.FgPrimary())
	sb.WriteString(inputStyle.Render(o.textInput.View()))

	return sb.String()
}

// View renders the overlay as a bordered modal box centred over the TUI frame.
func (o *ritualOverlay) View() string {
	if !o.active || o.runner == nil {
		return ""
	}

	bg := o.theme.BgSecondary()
	boxWidth := o.boxWidth()

	var sb strings.Builder

	// Title: "RitualName — Step N/M"
	r := o.runner.Ritual()
	stepNum := o.stepTracker + 1
	totalSteps := len(r.Steps)
	titleText := fmt.Sprintf("%s — Step %d/%d", r.Name, stepNum, totalSteps)

	titleStyle := lipgloss.NewStyle().
		Foreground(o.theme.AccentPrimary()).
		Background(bg).
		Bold(true).
		Width(boxWidth - 4)
	sb.WriteString(titleStyle.Render(titleText))
	sb.WriteString("\n")

	// Divider.
	divStyle := lipgloss.NewStyle().
		Foreground(o.theme.Border()).
		Background(bg)
	sb.WriteString(divStyle.Render(strings.Repeat("─", boxWidth-4)))
	sb.WriteString("\n")

	// Content — from the viewport.
	sb.WriteString(o.vp.View())
	sb.WriteString("\n")

	// Footer: keybinding hints.
	sb.WriteString(o.footerHints(bg, boxWidth))

	boxStyle := lipgloss.NewStyle().
		Background(bg).
		BorderStyle(lipgloss.RoundedBorder()).
		BorderForeground(o.theme.AccentPrimary()).
		BorderBackground(bg).
		Padding(1, 2).
		Width(boxWidth)

	return boxStyle.Render(sb.String())
}

// footerHints returns a styled line of keybinding hints appropriate for the
// current step type.
func (o *ritualOverlay) footerHints(bg color.Color, width int) string {
	step := o.currentStep()
	if step == nil {
		return ""
	}

	hintStyle := lipgloss.NewStyle().
		Background(bg).
		Foreground(o.theme.FgMuted()).
		Width(width - 4)

	var hints string
	switch step.Type {
	case types.StepQueryList:
		hints = "j/k: move  s: status  x: archive  e: edge  enter: next"
	case types.StepPrompt:
		hints = "enter: submit"
	case types.StepGate:
		hints = "enter/space: next  Esc Esc d: defer"
	default:
		hints = "enter/space: next"
		if !o.runner.IsGate() {
			hints += "  esc: dismiss"
		}
	}

	return hintStyle.Render(hints)
}

// currentStep returns the current step from the runner's ritual, or nil.
func (o *ritualOverlay) currentStep() *types.RitualStep {
	if o.runner == nil {
		return nil
	}
	r := o.runner.Ritual()
	idx := o.stepTracker
	if idx < 0 || idx >= len(r.Steps) {
		return nil
	}
	step := r.Steps[idx]
	return &step
}

// boxWidth returns the width of the overlay box.
func (o *ritualOverlay) boxWidth() int {
	w := o.width * 2 / 3
	if w < 40 {
		w = 40
	}
	if w > 100 {
		w = 100
	}
	return w
}

// boxInnerWidth returns the content width inside the box (accounting for
// border, padding).
func (o *ritualOverlay) boxInnerWidth() int {
	// Border (2) + padding (4) = 6 columns consumed.
	w := o.boxWidth() - 6
	if w < 10 {
		w = 10
	}
	return w
}

// boxInnerHeight returns the content height for the viewport, leaving room
// for the title, divider, and footer.
func (o *ritualOverlay) boxInnerHeight() int {
	// Title (1) + divider (1) + footer (1) + padding (2) + border (2) = 7.
	h := o.height/2 - 7
	if h < 4 {
		h = 4
	}
	return h
}
