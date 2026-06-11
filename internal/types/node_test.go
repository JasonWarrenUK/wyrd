package types_test

import (
	"testing"
	"time"

	"github.com/jasonwarrenuk/wyrd/internal/types"
)

// TestNodeClone verifies that Clone produces a copy whose mutable parts
// (Types, Properties, Date pointers, Source) are independent of the original.
func TestNodeClone(t *testing.T) {
	due := time.Date(2026, 4, 1, 0, 0, 0, 0, time.UTC)
	about := time.Date(2026, 3, 20, 0, 0, 0, 0, time.UTC)

	original := &types.Node{
		ID:    "node-1",
		Title: "Original",
		Body:  "body",
		Types: []string{"task", "project"},
		Kind:  "Task",
		Stage: "Now",
		Properties: map[string]interface{}{
			"status":    "active",
			"spend_log": []interface{}{map[string]interface{}{"amount": 12.5}},
		},
		Source: &types.Source{Type: "github", ID: "42"},
	}
	original.Date.Due = &due
	original.Date.About = &about

	clone := original.Clone()

	// Mutate the clone everywhere it could share memory with the original.
	clone.Types[0] = "note"
	clone.Properties["status"] = "archived"
	delete(clone.Properties, "spend_log")
	*clone.Date.Due = clone.Date.Due.AddDate(1, 0, 0)
	clone.Date.About = nil
	clone.Source.ID = "99"

	if original.Types[0] != "task" {
		t.Errorf("original Types mutated: %v", original.Types)
	}
	if original.Properties["status"] != "active" {
		t.Errorf("original Properties mutated: %v", original.Properties["status"])
	}
	if _, ok := original.Properties["spend_log"]; !ok {
		t.Error("original spend_log removed by clone mutation")
	}
	if !original.Date.Due.Equal(due) {
		t.Errorf("original Due mutated: %v", original.Date.Due)
	}
	if original.Date.About == nil || !original.Date.About.Equal(about) {
		t.Errorf("original About mutated: %v", original.Date.About)
	}
	if original.Source.ID != "42" {
		t.Errorf("original Source mutated: %v", original.Source.ID)
	}
}

// TestNodeCloneNil verifies the nil-receiver contract used by buildNode.
func TestNodeCloneNil(t *testing.T) {
	var n *types.Node
	if n.Clone() != nil {
		t.Error("Clone of nil node should be nil")
	}
}
