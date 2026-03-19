// Package embed exposes the starter files bundled into the binary via go:embed.
// These are copied into a new store directory on first run.
package embed

import "embed"

// StarterFS contains all files under the starter/ directory.
// These are copied to the store on initialisation.
//
//go:embed starter
var StarterFS embed.FS
