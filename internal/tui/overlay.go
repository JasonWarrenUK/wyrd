package tui

import "charm.land/lipgloss/v2"

// compositeOverlay places overlay horizontally and vertically centred over
// frame using lipgloss.Place, compositing the result at Z(1) over the base
// frame at Z(0) via the lipgloss Compositor. Returns frame unchanged when
// overlay is empty or width/height are zero.
func compositeOverlay(frame, overlay string, width, height int) string {
	if overlay == "" || width <= 0 || height <= 0 {
		return frame
	}

	// Place centres the overlay across the full terminal dimensions. The
	// whitespace padding it adds around the box overwrites the frame behind
	// it, so the visible backdrop is the terminal background rather than a
	// see-through view of the pane layout — consistent with the user's choice
	// of literal Place centring over a transparent dim.
	placed := lipgloss.Place(width, height, lipgloss.Center, lipgloss.Center, overlay)

	base := lipgloss.NewLayer(frame).Z(0)
	top := lipgloss.NewLayer(placed).Z(1)
	return lipgloss.NewCompositor(base, top).Render()
}

// overlayVPHeight computes a content-driven viewport height for an overlay,
// clamping to a safe maximum so the rendered box never overflows the terminal.
//
//   - contentLines: the number of lines the overlay content contains.
//   - termHeight:   the full terminal height.
//   - chromeRows:   rows consumed by the box chrome (border, padding, title,
//     divider, footer — typically 6 or 7).
//
// A margin of 2 rows is reserved so the box never touches the terminal edges.
// The returned height is at least 1.
func overlayVPHeight(contentLines, termHeight, chromeRows int) int {
	const margin = 2
	maxVP := termHeight - chromeRows - margin
	if maxVP < 1 {
		maxVP = 1
	}
	h := contentLines
	if h > maxVP {
		h = maxVP
	}
	if h < 1 {
		h = 1
	}
	return h
}
