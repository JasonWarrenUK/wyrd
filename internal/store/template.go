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

// loadTemplate reads and parses a template for the given type name,
// caching the result in s.templates.
//
// The cache is never invalidated: it is populated lazily on first read and
// kept for the lifetime of the Store. Editing a template file on disk after
// it has been loaded has no effect until the process restarts. This is a
// known, deliberate constraint rather than a bug — templates change rarely
// and a live-invalidating cache (e.g. wired to the fsnotify watcher) is out
// of scope here. ReadTemplate deliberately bypasses this cache read (it
// always hits disk) but still populates it on the way out, so a caller that
// wants a guaranteed-fresh read should call ReadTemplate directly. Negative
// results (unknown template) are intentionally *not* cached — see
// FieldsForType — because caching a miss with no invalidation would make an
// unknown-template error permanent for the process lifetime, which is worse
// than the occasional extra disk read.
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
			s.logWarn("skipping unreadable template", "type", typeName, "err", err)
			continue
		}
		templates = append(templates, tmpl)
	}
	return templates, nil
}

// FieldsForType returns the merged field definitions for a node type.
// If the template is not found, returns an empty map (not an error).
//
// The returned map is a copy, not the cache's live map — loadTemplate
// hands back a pointer to the cached *types.Template, and tmpl.Fields is
// that same map. Without copying, a caller mutating the returned map would
// corrupt the cache for every subsequent call.
func (s *Store) FieldsForType(typeName string) (map[string]types.FieldDef, error) {
	tmpl, err := s.loadTemplate(typeName)
	if err != nil {
		if _, ok := err.(*types.NotFoundError); ok {
			return map[string]types.FieldDef{}, nil
		}
		return nil, err
	}
	fields := make(map[string]types.FieldDef, len(tmpl.Fields))
	for k, v := range tmpl.Fields {
		fields[k] = v
	}
	return fields, nil
}
