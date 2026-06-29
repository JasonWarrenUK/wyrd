package tui

import (
	"testing"
)

// TestParseStages verifies the parseStages helper splits, trims, and drops
// blank lines correctly. Order must be preserved.
func TestParseStages(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  []string
	}{
		{
			name:  "simple newlines",
			input: "Open\nIn Progress\nDone",
			want:  []string{"Open", "In Progress", "Done"},
		},
		{
			name:  "trims surrounding whitespace per line",
			input: "  Open  \n  Done  ",
			want:  []string{"Open", "Done"},
		},
		{
			name:  "drops blank lines",
			input: "Open\n\n\nDone",
			want:  []string{"Open", "Done"},
		},
		{
			name:  "drops whitespace-only lines",
			input: "Open\n   \nDone",
			want:  []string{"Open", "Done"},
		},
		{
			name:  "empty string yields empty slice",
			input: "",
			want:  []string{},
		},
		{
			name:  "all blank yields empty slice",
			input: "\n   \n\n",
			want:  []string{},
		},
		{
			name:  "single stage",
			input: "Now",
			want:  []string{"Now"},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := parseStages(tc.input)
			if len(got) != len(tc.want) {
				t.Errorf("len = %d, want %d; got %v", len(got), len(tc.want), got)
				return
			}
			for i, s := range got {
				if s != tc.want[i] {
					t.Errorf("[%d] = %q, want %q", i, s, tc.want[i])
				}
			}
		})
	}
}
