package store

import (
	"fmt"
	"path/filepath"
	"time"

	"github.com/google/uuid"
	"github.com/jasonwarrenuk/wyrd/internal/types"
)

// edgeCoreFields lists the JSONC keys that belong to Edge's struct fields
// rather than its Properties map. Used both to split raw JSON on read
// (parseEdge) and to guard against a user-defined property silently
// shadowing a core field on write (CreateEdge, WriteEdge). Includes the
// legacy flat "created" key the on-disk format no longer writes but may
// still encounter reading a file from an older binary, alongside "date"
// and "modified" for the nested date block (TD.3).
var edgeCoreFields = map[string]bool{
	"id": true, "type": true, "from": true, "to": true,
	"created": true, "modified": true, "date": true,
}

// edgePath returns the file path for a given edge ID.
func (s *Store) edgePath(id string) string {
	return filepath.Join(s.path, "edges", id+".jsonc")
}

// CreateEdge creates a new edge and writes it to disk.
func (s *Store) CreateEdge(edgeType string, from string, to string, properties map[string]interface{}) (*types.Edge, error) {
	if edgeType == "" {
		return nil, &types.ValidationError{Field: "type", Message: "edge type is required"}
	}
	if from == "" {
		return nil, &types.ValidationError{Field: "from", Message: "from node ID is required"}
	}
	if to == "" {
		return nil, &types.ValidationError{Field: "to", Message: "to node ID is required"}
	}

	id := uuid.New().String()
	now := s.clock.Now()
	nowStr := now.UTC().Format(time.RFC3339)

	raw := map[string]interface{}{
		"id":   id,
		"type": edgeType,
		"from": from,
		"to":   to,
		"date": map[string]interface{}{
			"created":  nowStr,
			"modified": nowStr,
		},
	}
	for k, v := range properties {
		if edgeCoreFields[k] {
			s.logWarn("dropping property that collides with a core edge field", "key", k)
			continue
		}
		raw[k] = v
	}

	path := s.edgePath(id)
	if err := writeJSONC(path, raw); err != nil {
		return nil, fmt.Errorf("creating edge: %w", err)
	}

	edge, err := s.ReadEdge(id)
	if err != nil {
		return nil, err
	}
	s.index.upsertEdge(edge)
	return edge, nil
}

// ReadEdge reads an edge from disk by its UUID string.
func (s *Store) ReadEdge(id string) (*types.Edge, error) {
	path := s.edgePath(id)
	data, err := readJSONC(path)
	if err != nil {
		if isNotExist(err) {
			return nil, &types.NotFoundError{Kind: "edge", ID: id}
		}
		return nil, fmt.Errorf("reading edge %s: %w", id, err)
	}
	return parseEdge(id, data)
}

// parseEdge decodes raw JSONC bytes into a types.Edge.
func parseEdge(id string, data []byte) (*types.Edge, error) {
	var raw map[string]interface{}
	if err := unmarshalJSON(data, &raw); err != nil {
		return nil, &types.ParseError{Source: id, Message: err.Error()}
	}

	edge := &types.Edge{Properties: make(map[string]interface{})}

	for k, v := range raw {
		if !edgeCoreFields[k] {
			edge.Properties[k] = v
		}
	}

	edge.ID = stringField(raw, "id")
	edge.Type = stringField(raw, "type")
	edge.From = stringField(raw, "from")
	edge.To = stringField(raw, "to")

	// On-disk format is nested-only (TD.3): the date block is the sole
	// source of Created/Modified. A file from a binary that predates this
	// restructure (flat top-level "created", no "date" object) will parse
	// with a zero Date — there is no migration path pre-production.
	if dateRaw, ok := raw["date"].(map[string]interface{}); ok {
		if createdStr, ok := dateRaw["created"].(string); ok {
			if t, err := time.Parse(time.RFC3339, createdStr); err == nil {
				edge.Date.Created = t
			}
		}
		if modifiedStr, ok := dateRaw["modified"].(string); ok {
			if t, err := time.Parse(time.RFC3339, modifiedStr); err == nil {
				edge.Date.Modified = t
			}
		}
	}

	return edge, nil
}

// DeleteEdge removes an edge file from disk. Edges (unlike nodes) are truly deleted.
func (s *Store) DeleteEdge(id string) error {
	path := s.edgePath(id)
	if err := removeFile(path); err != nil {
		if isNotExist(err) {
			return &types.NotFoundError{Kind: "edge", ID: id}
		}
		return fmt.Errorf("deleting edge %s: %w", id, err)
	}
	s.index.removeEdge(id)
	s.logDebug("deleted edge", "id", id)
	return nil
}

// stringField extracts a string from a raw map, returning "" if absent or wrong type.
func stringField(m map[string]interface{}, key string) string {
	v, ok := m[key]
	if !ok {
		return ""
	}
	s, _ := v.(string)
	return s
}
