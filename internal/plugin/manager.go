// Package plugin implements the Wyrd plugin system: discovery, lifecycle
// management, the JSON-lines protocol handler, and the built-in shell executor.
package plugin

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jasonwarrenuk/wyrd/internal/types"
)

// wyrdNamespace is the UUID v5 namespace used to derive deterministic IDs for
// nodes produced by sync plugins. Using a fixed namespace ensures that the same
// external source ID always maps to the same Wyrd node ID across runs.
var wyrdNamespace = uuid.MustParse("6ba7b810-9dad-11d1-80b4-00c04fd430c8")

// Registry holds all discovered plugins indexed by name.
type Registry struct {
	plugins map[string]*PluginEntry
}

// PluginEntry bundles a manifest with its resolved executable path.
type PluginEntry struct {
	Manifest *types.PluginManifest
	// ExecPath is the absolute path to the executable (or interpreter for scripts).
	ExecPath string
	// ScriptPath is populated for script-type plugins; it is the path to the script
	// file itself, passed as the first argument to the interpreter.
	ScriptPath string
}

// Manager orchestrates plugin discovery, registration, and invocation.
type Manager struct {
	store    types.PluginStore
	registry *Registry
}

// NewManager creates a Manager backed by the given PluginStore.
func NewManager(store types.PluginStore) *Manager {
	return &Manager{
		store: store,
		registry: &Registry{
			plugins: make(map[string]*PluginEntry),
		},
	}
}

// Discover scans both the store plugins directory and the local ~/.wyrd/plugins
// directory, parses manifests, and populates the registry.
func (m *Manager) Discover() error {
	storePath := m.store.StorePath()
	storePluginsDir := filepath.Join(storePath, "plugins")
	localPluginsDir := filepath.Join(localWyrdDir(), "plugins")

	for _, dir := range []string{storePluginsDir, localPluginsDir} {
		if err := m.scanPluginDirectory(dir); err != nil {
			// Non-fatal: log and continue so a missing directory doesn't abort startup.
			log.Printf("plugin discovery: skipping %s: %v", dir, err)
		}
	}
	return nil
}

// scanPluginDirectory walks a top-level plugins directory, looking for
// subdirectories that each contain a plugin.jsonc manifest.
func (m *Manager) scanPluginDirectory(dir string) error {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return err
	}

	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		manifestPath := filepath.Join(dir, entry.Name(), "plugin.jsonc")
		manifest, err := parseManifestFile(manifestPath)
		if err != nil {
			log.Printf("plugin discovery: skipping %s: %v", manifestPath, err)
			continue
		}

		execPath, scriptPath, err := resolveExecutable(manifest, filepath.Join(dir, entry.Name()))
		if err != nil {
			log.Printf("plugin discovery: could not resolve executable for %s: %v", manifest.Name, err)
			continue
		}

		m.registry.plugins[manifest.Name] = &PluginEntry{
			Manifest:   manifest,
			ExecPath:   execPath,
			ScriptPath: scriptPath,
		}
	}
	return nil
}

// parseManifestFile reads and parses a plugin.jsonc file.
// JSONC comments are stripped before unmarshalling.
func parseManifestFile(path string) (*types.PluginManifest, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read manifest: %w", err)
	}

	clean := stripJSONC(data)

	var manifest types.PluginManifest
	if err := json.Unmarshal(clean, &manifest); err != nil {
		return nil, fmt.Errorf("parse manifest %s: %w", path, err)
	}
	if manifest.Name == "" {
		return nil, fmt.Errorf("manifest %s: missing name field", path)
	}
	return &manifest, nil
}

// stripJSONC removes single-line (//) and block (/* */) comments from JSON bytes.
func stripJSONC(data []byte) []byte {
	// Remove single-line comments.
	singleLine := regexp.MustCompile(`(?m)//.*$`)
	result := singleLine.ReplaceAll(data, nil)
	// Remove block comments.
	block := regexp.MustCompile(`(?s)/\*.*?\*/`)
	return block.ReplaceAll(result, nil)
}

// resolveExecutable locates the executable for a plugin, searching:
//  1. PATH
//  2. ~/.wyrd/bin/
//  3. The plugin's own directory
//
// For script-type plugins the interpreter is located via shebang or file extension,
// and the script file path is also returned.
func resolveExecutable(manifest *types.PluginManifest, pluginDir string) (execPath, scriptPath string, err error) {
	switch manifest.ExecutableType {
	case types.ExecutableTypeBinary:
		return resolveBinary(manifest.Executable, pluginDir)
	case types.ExecutableTypeScript:
		return resolveScript(manifest.Executable, pluginDir)
	default:
		return resolveBinary(manifest.Executable, pluginDir)
	}
}

// resolveBinary finds a binary by searching PATH, ~/.wyrd/bin/, and pluginDir.
func resolveBinary(name, pluginDir string) (execPath, scriptPath string, err error) {
	// 1. Search PATH.
	if p, lookErr := lookPath(name); lookErr == nil {
		return p, "", nil
	}

	// 2. Check ~/.wyrd/bin/.
	wyrdBin := filepath.Join(localWyrdDir(), "bin", name)
	if fileExists(wyrdBin) {
		return wyrdBin, "", nil
	}

	// 3. Check the plugin's own directory.
	localBin := filepath.Join(pluginDir, name)
	if fileExists(localBin) {
		return localBin, "", nil
	}

	return "", "", fmt.Errorf("binary %q not found in PATH, ~/.wyrd/bin/, or %s", name, pluginDir)
}

// resolveScript locates the script file and determines the correct interpreter.
func resolveScript(name, pluginDir string) (interpreter, script string, err error) {
	// Locate the script file.
	scriptCandidates := []string{
		filepath.Join(pluginDir, name),
	}
	var scriptFile string
	for _, c := range scriptCandidates {
		if fileExists(c) {
			scriptFile = c
			break
		}
	}
	if scriptFile == "" {
		return "", "", fmt.Errorf("script %q not found in %s", name, pluginDir)
	}

	interp, err := detectInterpreter(scriptFile)
	if err != nil {
		return "", "", err
	}

	// Resolve the interpreter binary via the same search order.
	interpExec, _, err := resolveBinary(interp, pluginDir)
	if err != nil {
		return "", "", fmt.Errorf("interpreter %q: %w", interp, err)
	}

	return interpExec, scriptFile, nil
}

// detectInterpreter reads the shebang line or falls back to file extension.
func detectInterpreter(scriptPath string) (string, error) {
	f, err := os.Open(scriptPath)
	if err != nil {
		return "", err
	}
	defer f.Close()

	buf := make([]byte, 256)
	n, _ := f.Read(buf)
	content := string(buf[:n])

	if strings.HasPrefix(content, "#!") {
		line := strings.SplitN(content, "\n", 2)[0]
		line = strings.TrimPrefix(line, "#!")
		parts := strings.Fields(line)
		if len(parts) > 0 {
			return filepath.Base(parts[0]), nil
		}
	}

	// Fall back to extension.
	ext := strings.ToLower(filepath.Ext(scriptPath))
	switch ext {
	case ".sh":
		return "sh", nil
	case ".py":
		return "python3", nil
	case ".rb":
		return "ruby", nil
	case ".js":
		return "node", nil
	default:
		return "", fmt.Errorf("cannot detect interpreter for %s", scriptPath)
	}
}

// Registry returns the plugin registry.
func (m *Manager) Registry() *Registry {
	return m.registry
}

// Get returns a PluginEntry by name, or an error if not found.
func (r *Registry) Get(name string) (*PluginEntry, error) {
	entry, ok := r.plugins[name]
	if !ok {
		return nil, &types.NotFoundError{Kind: "plugin", ID: name}
	}
	return entry, nil
}

// All returns all registered PluginEntries.
func (r *Registry) All() []*PluginEntry {
	entries := make([]*PluginEntry, 0, len(r.plugins))
	for _, e := range r.plugins {
		entries = append(entries, e)
	}
	return entries
}

// RunSync invokes a sync-capable plugin and applies resulting upserts to the store.
func (m *Manager) RunSync(name string, timeout time.Duration) error {
	entry, err := m.registry.Get(name)
	if err != nil {
		return err
	}

	manifest := entry.Manifest
	if !hasCapability(manifest, types.PluginCapabilitySync) {
		return &types.PluginError{Plugin: name, Message: "plugin does not support sync"}
	}

	config, err := m.store.ReadPluginConfig(name)
	if err != nil {
		config = make(map[string]interface{})
	}

	handler := &syncHandler{store: m.store, pluginName: name}
	runner := NewProtocolRunner(entry, timeout)
	return runner.RunSync(config, handler)
}

// RunAction invokes an action-capable plugin for the given node.
func (m *Manager) RunAction(name string, node *types.Node, timeout time.Duration) error {
	entry, err := m.registry.Get(name)
	if err != nil {
		return err
	}

	manifest := entry.Manifest
	if !hasCapability(manifest, types.PluginCapabilityAction) {
		return &types.PluginError{Plugin: name, Message: "plugin does not support action"}
	}

	config, err := m.store.ReadPluginConfig(name)
	if err != nil {
		config = make(map[string]interface{})
	}

	handler := &syncHandler{store: m.store, pluginName: name}
	runner := NewProtocolRunner(entry, timeout)
	return runner.RunAction(node, config, handler)
}

// UpsertNode writes a node to the store. If a node with the same ID already
// exists on disk it is overwritten (upsert semantics). For synced nodes the
// caller should derive the ID via DeterministicNodeID before calling this.
func (m *Manager) UpsertNode(node *types.Node) error {
	return m.store.WriteNode(node)
}

// UpsertEdge writes an edge to the store with upsert semantics.
func (m *Manager) UpsertEdge(edge *types.Edge) error {
	return m.store.WriteEdge(edge)
}

// DeterministicNodeID derives a UUID v5 from the given source type and external ID.
// The same inputs always produce the same output, ensuring idempotent syncs.
func DeterministicNodeID(sourceType, externalID string) string {
	name := sourceType + ":" + externalID
	return uuid.NewSHA1(wyrdNamespace, []byte(name)).String()
}

// syncHandler routes upsert messages from the protocol runner into the store.
type syncHandler struct {
	store      types.PluginStore
	pluginName string
}

func (h *syncHandler) HandleUpsertNode(node *types.Node) error {
	return h.store.WriteNode(node)
}

func (h *syncHandler) HandleUpsertEdge(edge *types.Edge) error {
	return h.store.WriteEdge(edge)
}

// hasCapability checks whether a manifest declares a given capability.
func hasCapability(manifest *types.PluginManifest, cap types.PluginCapability) bool {
	for _, c := range manifest.Capabilities {
		if c == cap {
			return true
		}
	}
	return false
}

// localWyrdDir returns the path to ~/.wyrd.
func localWyrdDir() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return filepath.Join("~", ".wyrd")
	}
	return filepath.Join(home, ".wyrd")
}

// fileExists reports whether a regular file exists at path.
func fileExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && !info.IsDir()
}

// lookPath is a thin wrapper around os.LookupEnv / exec.LookPath to allow
// testing without real binaries. Defaults to the standard implementation.
var lookPath = func(name string) (string, error) {
	return lookPathImpl(name)
}

func lookPathImpl(name string) (string, error) {
	// Use filepath.Abs only for names without separators (i.e., bare names).
	// os/exec.LookPath is unavailable here so we search PATH manually.
	pathEnv := os.Getenv("PATH")
	for _, dir := range filepath.SplitList(pathEnv) {
		full := filepath.Join(dir, name)
		if fileExists(full) {
			return full, nil
		}
	}
	return "", fmt.Errorf("%q: not found in PATH", name)
}
