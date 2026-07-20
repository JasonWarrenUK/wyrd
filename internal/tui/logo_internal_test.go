package tui

import (
	"image/color"
	"strings"
	"testing"
)

// TestGradientTextLeftEdgeMatchesFirstStop checks that the leftmost
// character of each row carries (approximately) the first gradient stop —
// the wordmark's gradient must actually start at its intended colour, not
// somewhere partway along the blend.
func TestGradientTextLeftEdgeMatchesFirstStop(t *testing.T) {
	block := "AAA\nAAA"
	bg := color.RGBA{R: 0, G: 0, B: 0, A: 255}
	stop1 := color.RGBA{R: 213, G: 115, B: 0, A: 255}   // orange
	stop2 := color.RGBA{R: 155, G: 112, B: 255, A: 255} // purple

	out := gradientText(block, bg, stop1, stop2, stop1)
	lines := strings.Split(out, "\n")
	if len(lines) != 2 {
		t.Fatalf("expected 2 output lines, got %d", len(lines))
	}

	cells := ansiColourCells(lines[0])
	if len(cells) == 0 {
		t.Fatal("no coloured cells found in gradient output")
	}
	got := cells[0].colour
	r, g, b, _ := got.RGBA()
	wr, wg, wb, _ := color.Color(stop1).RGBA()
	if r>>8 != wr>>8 || g>>8 != wg>>8 || b>>8 != wb>>8 {
		t.Errorf("leftmost cell colour = %v, want first stop %v", got, stop1)
	}
}

// TestGradientTextRightEdgeMatchesLastStop mirrors the above for the
// rightmost character, confirming the gradient reaches its final stop by
// the end of the row rather than stopping short. This is also a regression
// test for a real bug hit while building this: an earlier implementation
// used lipgloss.Blend2D, whose centred/diagonal-length-normalised 2D
// sampling maps a plain horizontal sweep's last column to a gradientPos
// just short of 1.0 — the gradient reached its middle stop exactly but
// never quite returned to the first stop by the final column (~84 units
// off on one channel in manual testing). Blend1D (one sample per column,
// no centring) fixes this — see gradientText's doc.
func TestGradientTextRightEdgeMatchesLastStop(t *testing.T) {
	block := "AAAAAAAAAA"
	bg := color.RGBA{R: 0, G: 0, B: 0, A: 255}
	stop1 := color.RGBA{R: 213, G: 115, B: 0, A: 255}
	stop2 := color.RGBA{R: 155, G: 112, B: 255, A: 255}

	out := gradientText(block, bg, stop1, stop2, stop1)
	cells := ansiColourCells(out)
	if len(cells) == 0 {
		t.Fatal("no coloured cells found in gradient output")
	}
	got := cells[len(cells)-1].colour
	r, g, b, _ := got.RGBA()
	wr, wg, wb, _ := color.Color(stop1).RGBA()
	if r>>8 != wr>>8 || g>>8 != wg>>8 || b>>8 != wb>>8 {
		t.Errorf("rightmost cell colour = %v, want last stop %v", got, stop1)
	}
}

// TestGradientTextMiddleDiffersFromEdges is a coarse sanity check that the
// gradient is actually blending through its middle stop rather than, say,
// silently collapsing to a flat colour across the whole row.
func TestGradientTextMiddleDiffersFromEdges(t *testing.T) {
	block := strings.Repeat("A", 21)
	bg := color.RGBA{R: 0, G: 0, B: 0, A: 255}
	stop1 := color.RGBA{R: 213, G: 115, B: 0, A: 255}
	stop2 := color.RGBA{R: 155, G: 112, B: 255, A: 255}

	out := gradientText(block, bg, stop1, stop2, stop1)
	cells := ansiColourCells(out)
	if len(cells) < 3 {
		t.Fatalf("expected at least 3 cells, got %d", len(cells))
	}

	first := cells[0].colour
	mid := cells[len(cells)/2].colour
	if first == mid {
		t.Error("middle cell colour equals the first stop — gradient did not blend")
	}
}

// TestGradientTextRowsShareSameColumnColours checks that every row of a
// multi-line block gets the SAME colour at a given column — the gradient
// is one flat horizontal sweep shared across all rows, not computed
// independently per row (which would still be visually plausible for a
// pure-horizontal sweep, but isn't what gradientText's doc promises, and
// diverging per-row was the exact shape of the Blend2D bug this function
// was rewritten to avoid).
func TestGradientTextRowsShareSameColumnColours(t *testing.T) {
	block := "AAAAA\nAAAAA\nAAAAA"
	bg := color.RGBA{A: 255}
	stop1 := color.RGBA{R: 213, G: 115, B: 0, A: 255}
	stop2 := color.RGBA{R: 155, G: 112, B: 255, A: 255}

	out := gradientText(block, bg, stop1, stop2, stop1)
	lines := strings.Split(out, "\n")
	if len(lines) != 3 {
		t.Fatalf("expected 3 output lines, got %d", len(lines))
	}

	rowCells := make([][]ansiCell, len(lines))
	for i, line := range lines {
		rowCells[i] = ansiColourCells(line)
	}
	for col := 0; col < len(rowCells[0]); col++ {
		want := rowCells[0][col].colour
		for row := 1; row < len(rowCells); row++ {
			if rowCells[row][col].colour != want {
				t.Errorf("column %d: row 0 colour %v != row %d colour %v",
					col, want, row, rowCells[row][col].colour)
			}
		}
	}
}

// TestGradientTextEmptyBlockReturnsUnchanged guards the width==0 edge case
// (an empty or blank block) against a panic from indexing an empty
// Blend2D result.
func TestGradientTextEmptyBlockReturnsUnchanged(t *testing.T) {
	bg := color.RGBA{A: 255}
	stop := color.RGBA{R: 1, G: 2, B: 3, A: 255}
	if got := gradientText("", bg, stop, stop); got != "" {
		t.Errorf("gradientText(\"\", ...) = %q, want \"\"", got)
	}
}

// TestLogoThickBorderUsesThickEdgesSquareCorners checks the logo border
// struct pairs ThickBorder's heavy edge/line glyphs with matching heavy
// square corner glyphs, so the box reads as one consistent weight rather
// than mixing a heavy edge with a thin rounded corner.
func TestLogoThickBorderUsesThickEdgesSquareCorners(t *testing.T) {
	b := logoThickBorder
	if b.Top != "━" || b.Left != "┃" {
		t.Errorf("expected thick edge glyphs (━, ┃), got Top=%q Left=%q", b.Top, b.Left)
	}
	if b.TopLeft != "┏" || b.TopRight != "┓" || b.BottomLeft != "┗" || b.BottomRight != "┛" {
		t.Errorf("expected thick square corner glyphs (┏┓┗┛), got %q %q %q %q",
			b.TopLeft, b.TopRight, b.BottomLeft, b.BottomRight)
	}
}
