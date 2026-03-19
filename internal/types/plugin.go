package types

// PluginCapability enumerates the capabilities a plugin can declare.
type PluginCapability string

const (
	// PluginCapabilitySync indicates the plugin can pull data from external sources.
	PluginCapabilitySync PluginCapability = "sync"

	// PluginCapabilityAction indicates the plugin can act on a selected node in the TUI.
	PluginCapabilityAction PluginCapability = "action"
)

// ExecutableType enumerates how the plugin binary is invoked.
type ExecutableType string

const (
	ExecutableTypeBinary ExecutableType = "binary"
	ExecutableTypeScript ExecutableType = "script"
)

// PluginManifest describes a plugin's identity, capabilities, and configuration shape.
// Stored at /store/plugins/{name}/plugin.jsonc.
type PluginManifest struct {
	// Name is the plugin's unique identifier.
	Name string `json:"name"`

	// Version is the semantic version string.
	Version string `json:"version"`

	// Description is a human-readable summary of what the plugin does.
	Description string `json:"description,omitempty"`

	// Executable is the name of the binary or script to invoke.
	Executable string `json:"executable"`

	// ExecutableType indicates whether the plugin is a compiled binary or a script.
	ExecutableType ExecutableType `json:"executable_type"`

	// Capabilities lists the operations this plugin supports.
	Capabilities []PluginCapability `json:"capabilities"`

	// Sync holds sync-capability configuration. Present when "sync" is in Capabilities.
	Sync *PluginSyncConfig `json:"sync,omitempty"`

	// Action holds action-capability configuration. Present when "action" is in Capabilities.
	Action *PluginActionConfig `json:"action,omitempty"`

	// ConfigSchema describes the plugin's user configuration shape.
	ConfigSchema map[string]interface{} `json:"config_schema,omitempty"`
}

// PluginSyncConfig describes sync-capability configuration.
type PluginSyncConfig struct {
	// Triggers lists when the sync can be invoked ("manual", "schedule").
	Triggers []string `json:"triggers"`

	// NodeTypes lists the node types this plugin produces.
	NodeTypes []string `json:"node_types"`

	// EdgeTypes lists the edge types this plugin produces.
	EdgeTypes []string `json:"edge_types"`
}

// PluginActionConfig describes action-capability configuration.
type PluginActionConfig struct {
	// Label is the display name shown in the TUI.
	Label string `json:"label"`

	// Keybinding is the key sequence that triggers this action.
	Keybinding string `json:"keybinding,omitempty"`

	// ApplicableTypes lists the node types this action can act upon.
	// Empty means the action applies to all types.
	ApplicableTypes []string `json:"applicable_types,omitempty"`
}
