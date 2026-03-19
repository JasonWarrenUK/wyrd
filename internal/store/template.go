package store

import (
	"fmt"
	"path/filepath"

	"github.com/jasonwarrenuk/wyrd/internal/types"
)

// templatePath returns the file path for a given template type name.
func (s *Store) templatePath(typeName string) string {
	return filepath.Join(s.path, "templates", typeName+".jsonc")
}

// loadTemplate reads and parses a template for the given type name.
func (s *Store) loadTemplate(typeName string) (*types.Template, error) {
	s.templateMu.RLock()
	if tmpl, ok := s.templates[typeName]; ok {
		s.templateMu.RUnlock()
		return tmpl, nil
	}
	s.templateMu.RUnlock()

	return s.ReadTemplate(typeName)
}

// ReadTemplate reads a template definition by type name from disk.
func (s *Store) ReadTemplate(typeName string) (*types.Template, error) {
	path := s.templatePath(typeName)
	data, err := readJSONC(path)
	if err != nil {
		if isNotExist(err) {
			return nil, &types.NotFoundError{Kind: "template", ID: typeName}
		}
		return nil, fmt.Errorf("reading template %s: %w", typeName, err)
	}

	var tmpl types.Template
	if err := unmarshalJSON(data, &tmpl); err != nil {
		return nil, &types.ParseError{Source: typeName, Message: err.Error()}
	}

	s.templateMu.Lock()
	s.templates[typeName] = &tmpl
	s.templateMu.Unlock()

	return &tmpl, nil
}

// AllTemplates returns all template definitions found in the store.
func (s *Store) AllTemplates() ([]*types.Template, error) {
	dir := filepath.Join(s.path, "templates")
	entries, err := readdirNames(dir)
	if err != nil {
		return nil, fmt.Errorf("listing templates: %w", err)
	}

	var templates []*types.Template
	for _, name := range entries {
		if filepath.Ext(name) != ".jsonc" {
			continue
		}
		typeName := name[:len(name)-len(".jsonc")]
		tmpl, err := s.ReadTemplate(typeName)
		if err != nil {
			continue // Skip invalid templates.
		}
		templates = append(templates, tmpl)
	}
	return templates, nil
}

// FieldsForType returns the merged field definitions for a node type.
// If the template is not found, returns an empty map (not an error).
func (s *Store) FieldsForType(typeName string) (map[string]types.FieldDef, error) {
	tmpl, err := s.loadTemplate(typeName)
	if err != nil {
		if _, ok := err.(*types.NotFoundError); ok {
			return map[string]types.FieldDef{}, nil
		}
		return nil, err
	}
	return tmpl.Fields, nil
}
