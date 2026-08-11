// Package store implements the flat-file graph storage layer for Wyrd.
// Nodes and edges are stored as individual JSONC files. Comments in JSONC
// files are preserved through read/write cycles.
package store

import (
	"github.com/jasonwarrenuk/wyrd/internal/jsonc"
)

// readJSONC, writeJSONC and stripComments are package-local aliases over
// internal/jsonc (TD.1), kept so the ~20 in-package call sites across
// store.go, node.go, edge.go and template.go don't need to churn.
func readJSONC(path string) ([]byte, error) {
	return jsonc.ReadFile(path)
}

func writeJSONC(path string, data interface{}) error {
	return jsonc.WriteFile(path, data)
}

func stripComments(src []byte) []byte {
	return jsonc.Strip(src)
}
