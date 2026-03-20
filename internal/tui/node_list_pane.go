package tui

import (
	tea "github.com/charmbracelet/bubbletea"
	"github.com/jasonwarrenuk/wyrd/internal/tui/views"
	"github.com/jasonwarrenuk/wyrd/internal/types"
)

// nodeListPane is a PaneModel that renders a QueryResult as a scrollable list
// using the views.ListRenderer. It supports basic j/k keyboard navigation.
//
// This is an intentionally minimal implementation. NV.1 (bubbles/list
// integration) will replace the raw renderer with a proper Bubbles component
// that adds filtering, momentum scrolling, and mouse support.
type nodeListPane struct {
	renderer    *views.ListRenderer
	result      types.QueryResult
	selectedIdx int
	width       int
	height      int
}

// newNodeListPane constructs a pane from a QueryResult. The renderer uses the
// dashboard column set and default palette.
func newNodeListPane(result types.QueryResult) nodeListPane {
	return nodeListPane{
		renderer:    views.NewListRenderer(dashboardColumns),
		result:      result,
		selectedIdx: 0,
		width:       80,
		height:      24,
	}
}

// Update handles window resize and j/k navigation keys.
func (p nodeListPane) Update(msg tea.Msg) (PaneModel, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		p.width = msg.Width / 2 // left pane gets roughly half the terminal width
		p.height = msg.Height
		return p, nil

	case tea.KeyMsg:
		switch msg.String() {
		case "j", "down":
			if p.selectedIdx < len(p.result.Rows)-1 {
				p.selectedIdx++
			}
		case "k", "up":
			if p.selectedIdx > 0 {
				p.selectedIdx--
			}
		}
	}
	return p, nil
}

// View renders the list using the ListRenderer.
func (p nodeListPane) View() string {
	return p.renderer.Render(p.result, p.selectedIdx, p.width)
}

// KeyBindings advertises the navigation keys this pane handles.
func (p nodeListPane) KeyBindings() []KeyBinding {
	return []KeyBinding{
		{Key: "j / ↓", Description: "Move down"},
		{Key: "k / ↑", Description: "Move up"},
	}
}
