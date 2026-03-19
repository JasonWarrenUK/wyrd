package types

// Config holds the application-level configuration.
// Stored at /store/config.jsonc.
type Config struct {
	// StorePath is the absolute path to the /store directory.
	// Defaults to ~/wyrd/store.
	StorePath string `json:"store_path,omitempty"`

	// Theme is the name of the active theme file (without extension).
	Theme string `json:"theme,omitempty"`

	// Editor is the command used to open nodes in an external editor.
	// Defaults to $EDITOR, then "vi".
	Editor string `json:"editor,omitempty"`

	// DefaultView is the view name displayed on startup.
	DefaultView string `json:"default_view,omitempty"`

	// SyncRemote is the git remote URL used by wyrd sync.
	SyncRemote string `json:"sync_remote,omitempty"`

	// MaxTraversalDepth caps variable-length edge traversal in queries.
	// Defaults to 5.
	MaxTraversalDepth int `json:"max_traversal_depth,omitempty"`
}
