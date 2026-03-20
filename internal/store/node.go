package store

import (
	"encoding/json"
	"fmt"
	"path/filepath"
	"time"

	"github.com/google/uuid"
	"github.com/jasonwarrenuk/wyrd/internal/types"
)

// nodeFile is the on-disk representation of a node, with Properties inlined.
type nodeFile struct {
	ID         string                 `json:"id"`
	Body       string                 `json:"body"`
	Types      []string               `json:"types"`
	Created    time.Time              `json:"created"`
	Modified   time.Time              `json:"modified"`
	Source     *types.Source          `json:"source,omitempty"`
	Properties map[string]interface{} `json:"-"`
	// raw holds all fields including Properties for round-trip serialisation
	raw map[string]interface{}
}

// nodePath returns the file path for a given node ID.
func (s *Store) nodePath(id string) string {
	return filepath.Join(s.path, "nodes", id+".jsonc")
}

// CreateNode creates a new node, merges template fields, and writes it to disk.
func (s *Store) CreateNode(body string, nodeTypes []string) (*types.Node, error) {
	if body == "" {
		return nil, &types.ValidationError{Field: "body", Message: "body is required"}
	}
	if len(nodeTypes) == 0 {
		return nil, &types.ValidationError{Field: "types", Message: "at least one type is required"}
	}

	id := uuid.New().String()
	now := s.clock.Now()

	raw := map[string]interface{}{
		"id":       id,
		"body":     body,
		"types":    nodeTypes,
		"created":  now.UTC().Format(time.RFC3339),
		"modified": now.UTC().Format(time.RFC3339),
	}

	// Merge template fields with defaults (first type takes precedence).
	seen := map[string]bool{}
	for _, typeName := range nodeTypes {
		tmpl, err := s.loadTemplate(typeName)
		if err != nil {
			// Unknown template is not fatal — proceed without its fields.
			continue
		}
		for fieldName, fieldDef := range tmpl.Fields {
			if seen[fieldName] {
				continue
			}
			seen[fieldName] = true
			if fieldDef.Default != nil {
				raw[fieldName] = fieldDef.Default
			}
		}
	}

	path := s.nodePath(id)
	if err := writeJSONC(path, raw); err != nil {
		return nil, fmt.Errorf("creating node: %w", err)
	}

	node, err := s.ReadNode(id)
	if err != nil {
		return nil, err
	}
	s.index.upsertNode(node)
	return node, nil
}

// ReadNode reads a node from disk by its UUID string.
func (s *Store) ReadNode(id string) (*types.Node, error) {
	path := s.nodePath(id)
	data, err := readJSONC(path)
	if err != nil {
		return nil, &types.NotFoundError{Kind: "node", ID: id}
	}
	return parseNode(id, data)
}

// parseNode decodes raw JSONC bytes into a types.Node.
func parseNode(id string, data []byte) (*types.Node, error) {
	var raw map[string]interface{}
	if err := json.Unmarshal(data, &raw); err != nil {
		return nil, &types.ParseError{Source: id, Message: err.Error()}
	}

	node := &types.Node{Properties: make(map[string]interface{})}
	coreFields := map[string]bool{
		"id": true, "body": true, "title": true, "types": true,
		"created": true, "modified": true, "source": true,
	}

	for k, v := range raw {
		if !coreFields[k] {
			node.Properties[k] = v
		}
	}

	// Re-marshal just the core fields into a struct.
	coreData, _ := json.Marshal(raw)
	type coreNode struct {
		ID       string        `json:"id"`
		Body     string        `json:"body"`
		Title    string        `json:"title,omitempty"`
		Types    []string      `json:"types"`
		Created  time.Time     `json:"created"`
		Modified time.Time     `json:"modified"`
		Source   *types.Source `json:"source,omitempty"`
	}
	var core coreNode
	if err := json.Unmarshal(coreData, &core); err != nil {
		return nil, &types.ParseError{Source: id, Message: err.Error()}
	}

	node.ID = core.ID
	node.Body = core.Body
	node.Title = core.Title
	node.Types = core.Types
	node.Created = core.Created
	node.Modified = core.Modified
	node.Source = core.Source
	return node, nil
}

// UpdateNode applies a map of updates to a node and persists it.
func (s *Store) UpdateNode(id string, updates map[string]interface{}) (*types.Node, error) {
	path := s.nodePath(id)
	data, err := readJSONC(path)
	if err != nil {
		if isNotExist(err) {
			return nil, &types.NotFoundError{Kind: "node", ID: id}
		}
		return nil, fmt.Errorf("reading node %s for update: %w", id, err)
	}

	var raw map[string]interface{}
	if err := json.Unmarshal(data, &raw); err != nil {
		return nil, &types.ParseError{Source: id, Message: err.Error()}
	}

	for k, v := range updates {
		if k == "id" || k == "created" {
			continue // immutable fields
		}
		raw[k] = v
	}
	raw["modified"] = s.clock.Now().UTC().Format(time.RFC3339)

	if err := writeJSONC(path, raw); err != nil {
		return nil, fmt.Errorf("updating node %s: %w", id, err)
	}

	node, err := s.ReadNode(id)
	if err != nil {
		return nil, err
	}
	s.index.upsertNode(node)
	return node, nil
}

// ArchiveNode sets status to "archived" on the node. Never deletes the file.
func (s *Store) ArchiveNode(id string) error {
	_, err := s.UpdateNode(id, map[string]interface{}{"status": "archived"})
	return err
}

// AddType adds a type to a node and merges in the template's fields with defaults.
func (s *Store) AddType(id string, typeName string) error {
	path := s.nodePath(id)
	data, err := readJSONC(path)
	if err != nil {
		if isNotExist(err) {
			return &types.NotFoundError{Kind: "node", ID: id}
		}
		return fmt.Errorf("reading node %s: %w", id, err)
	}

	var raw map[string]interface{}
	if err := json.Unmarshal(data, &raw); err != nil {
		return &types.ParseError{Source: id, Message: err.Error()}
	}

	// Add type to types array if not present.
	nodeTypes := toStringSlice(raw["types"])
	for _, t := range nodeTypes {
		if t == typeName {
			return nil // Already present.
		}
	}
	nodeTypes = append(nodeTypes, typeName)
	raw["types"] = nodeTypes

	// Merge template fields.
	tmpl, err := s.loadTemplate(typeName)
	if err == nil {
		for fieldName, fieldDef := range tmpl.Fields {
			if _, exists := raw[fieldName]; !exists && fieldDef.Default != nil {
				raw[fieldName] = fieldDef.Default
			}
		}
	}

	raw["modified"] = s.clock.Now().UTC().Format(time.RFC3339)
	if err := writeJSONC(path, raw); err != nil {
		return fmt.Errorf("adding type to node %s: %w", id, err)
	}

	node, err := s.ReadNode(id)
	if err != nil {
		return err
	}
	s.index.upsertNode(node)
	return nil
}

// RemoveType removes a type from the node's types array. The type's fields
// remain in the file but are no longer surfaced by default.
func (s *Store) RemoveType(id string, typeName string) error {
	path := s.nodePath(id)
	data, err := readJSONC(path)
	if err != nil {
		if isNotExist(err) {
			return &types.NotFoundError{Kind: "node", ID: id}
		}
		return fmt.Errorf("reading node %s: %w", id, err)
	}

	var raw map[string]interface{}
	if err := json.Unmarshal(data, &raw); err != nil {
		return &types.ParseError{Source: id, Message: err.Error()}
	}

	nodeTypes := toStringSlice(raw["types"])
	filtered := nodeTypes[:0]
	for _, t := range nodeTypes {
		if t != typeName {
			filtered = append(filtered, t)
		}
	}
	if len(filtered) == 0 {
		return &types.ValidationError{Field: "types", Message: "cannot remove the last type — a node must have at least one type"}
	}
	raw["types"] = filtered
	raw["modified"] = s.clock.Now().UTC().Format(time.RFC3339)

	if err := writeJSONC(path, raw); err != nil {
		return fmt.Errorf("removing type from node %s: %w", id, err)
	}

	node, err := s.ReadNode(id)
	if err != nil {
		return err
	}
	s.index.upsertNode(node)
	return nil
}

// toStringSlice converts an interface{} (from JSON unmarshalling) to []string.
func toStringSlice(v interface{}) []string {
	if v == nil {
		return nil
	}
	arr, ok := v.([]interface{})
	if !ok {
		return nil
	}
	out := make([]string, 0, len(arr))
	for _, item := range arr {
		if s, ok := item.(string); ok {
			out = append(out, s)
		}
	}
	return out
}
