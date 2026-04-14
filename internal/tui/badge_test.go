package tui

import (
	"strings"
	"testing"

	"charm.land/lipgloss/v2"
)

func TestTypeBadge_KnownType(t *testing.T) {
	badge := TypeBadge("task")
	if badge == "" {
		t.Fatal("expected non-empty badge for known type 'task'")
	}
	if !strings.Contains(badge, "task") {
		t.Errorf("expected badge to contain 'task', got: %q", badge)
	}
}

func TestTypeBadge_UnknownType(t *testing.T) {
	badge := TypeBadge("custom_type")
	if badge == "" {
		t.Fatal("expected non-empty badge for unknown type 'custom_type'")
	}
	if !strings.Contains(badge, "custom_type") {
		t.Errorf("expected badge to contain 'custom_type', got: %q", badge)
	}
}

func TestBadgeColourFor_KnownTypes(t *testing.T) {
	cases := []struct {
		typeName string
		wantBG   string
	}{
		{"task", "#794aff"},
		{"note", "#009e8c"},
		{"journal", "#b98300"},
		{"budget", "#65a30d"},
		{"project", "#cd3c00"},
		{"person", "#82002c"},
	}
	for _, tc := range cases {
		got := BadgeColourFor(tc.typeName)
		if got.BG != tc.wantBG {
			t.Errorf("BadgeColourFor(%q).BG = %q, want %q", tc.typeName, got.BG, tc.wantBG)
		}
	}
}

func TestBadgeColourFor_Deterministic(t *testing.T) {
	c1 := BadgeColourFor("my_custom_type")
	c2 := BadgeColourFor("my_custom_type")
	if c1 != c2 {
		t.Errorf("BadgeColourFor is non-deterministic: %+v != %+v", c1, c2)
	}
}

func TestTypeBadges_Empty(t *testing.T) {
	result := TypeBadges(nil, lipgloss.NoColor{})
	if result != "" {
		t.Errorf("expected empty string for nil types, got: %q", result)
	}
	result = TypeBadges([]string{}, lipgloss.NoColor{})
	if result != "" {
		t.Errorf("expected empty string for empty types, got: %q", result)
	}
}

func TestTypeBadges_MultipleBadges(t *testing.T) {
	result := TypeBadges([]string{"task", "note"}, lipgloss.NoColor{})
	if result == "" {
		t.Fatal("expected non-empty result for multiple types")
	}
	if !strings.Contains(result, "task") {
		t.Errorf("result missing 'task': %q", result)
	}
	if !strings.Contains(result, "note") {
		t.Errorf("result missing 'note': %q", result)
	}
}
