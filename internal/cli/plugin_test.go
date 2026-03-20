package cli_test

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/jasonwarrenuk/wyrd/internal/cli"
	"github.com/jasonwarrenuk/wyrd/internal/store"
	"github.com/jasonwarrenuk/wyrd/internal/types"
)

// mockPluginStore wraps a real store to satisfy types.PluginStore.
// The real store.Store already implements PluginStore; this helper just
// opens one pointed at a temp directory.
func newPluginTestStore(t *testing.T) (*store.Store, string) {
	t.Helper()
	dir := t.TempDir()
	s, err := store.New(dir, types.RealClock{})
	if err != nil {
		t.Fatalf("opening store: %v", err)
	}
	t.Cleanup(func() { s.Close() })
	return s, dir
}

// ---------------------------------------------------------------------------
// PluginList tests
// ---------------------------------------------------------------------------

func TestPluginList_Empty(t *testing.T) {
	s, _ := newPluginTestStore(t)
	var buf bytes.Buffer

	if err := cli.PluginList(s, &buf); err != nil {
		t.Fatalf("PluginList: %v", err)
	}
	if !strings.Contains(buf.String(), "No plugins installed") {
		t.Errorf("expected 'No plugins installed' in output, got: %q", buf.String())
	}
}

func TestPluginList_WithPlugins(t *testing.T) {
	s, dir := newPluginTestStore(t)

	// Install two plugin manifests directly into the store layout.
	for _, name := range []string{"alpha", "beta"} {
		pluginDir := filepath.Join(dir, "plugins", name)
		if err := os.MkdirAll(pluginDir, 0o755); err != nil {
			t.Fatalf("mkdir %s: %v", pluginDir, err)
		}
		manifest := types.PluginManifest{
			Name:        name,
			Version:     "1.0.0",
			Description: name + " plugin",
			Executable:  name,
		}
		data, _ := json.Marshal(manifest)
		// Write as manifest.jsonc (store reads manifest.jsonc).
		if err := os.WriteFile(filepath.Join(pluginDir, "manifest.jsonc"), data, 0o644); err != nil {
			t.Fatalf("writing manifest: %v", err)
		}
	}

	var buf bytes.Buffer
	if err := cli.PluginList(s, &buf); err != nil {
		t.Fatalf("PluginList: %v", err)
	}
	output := buf.String()
	if !strings.Contains(output, "alpha") {
		t.Errorf("expected 'alpha' in output, got: %q", output)
	}
	if !strings.Contains(output, "beta") {
		t.Errorf("expected 'beta' in output, got: %q", output)
	}
}

// ---------------------------------------------------------------------------
// PluginInstall tests
// ---------------------------------------------------------------------------

func TestPluginInstall_EmptyPath(t *testing.T) {
	s, _ := newPluginTestStore(t)
	var buf bytes.Buffer

	err := cli.PluginInstall(s, "", &buf)
	if err == nil {
		t.Fatal("expected validation error for empty path")
	}
	var ve *types.ValidationError
	if !asValidationError(err, &ve) {
		t.Errorf("expected ValidationError, got %T: %v", err, err)
	}
}

func TestPluginInstall_FromDirectory(t *testing.T) {
	s, storeDir := newPluginTestStore(t)

	// Create a temp plugin directory with a plugin.json manifest.
	pluginSrc := t.TempDir()
	manifest := map[string]interface{}{
		"name":            "testplugin",
		"version":         "0.1.0",
		"description":     "A test plugin",
		"executable":      "testplugin",
		"executable_type": "binary",
		"capabilities":    []string{},
	}
	data, _ := json.Marshal(manifest)
	if err := os.WriteFile(filepath.Join(pluginSrc, "plugin.json"), data, 0o644); err != nil {
		t.Fatalf("writing plugin.json: %v", err)
	}

	var buf bytes.Buffer
	if err := cli.PluginInstall(s, pluginSrc, &buf); err != nil {
		t.Fatalf("PluginInstall: %v", err)
	}

	// Verify the plugin was copied into the store.
	destDir := filepath.Join(storeDir, "plugins", "testplugin")
	if _, err := os.Stat(destDir); err != nil {
		t.Errorf("plugin directory not found at %s: %v", destDir, err)
	}
	if _, err := os.Stat(filepath.Join(destDir, "plugin.json")); err != nil {
		t.Errorf("plugin.json not found in destination: %v", err)
	}
}

// ---------------------------------------------------------------------------
// extractJSONField edge cases
// ---------------------------------------------------------------------------

// TestExtractJSONField exercises the internal extractJSONField helper via
// PluginInstall — we create plugin.json files with various content and
// confirm the name is extracted (or not) as expected.

func TestExtractJSONField_HappyPath(t *testing.T) {
	s, _ := newPluginTestStore(t)

	pluginSrc := t.TempDir()
	content := `{"name": "simple-plugin", "version": "1.0.0", "executable": "x", "capabilities": []}`
	if err := os.WriteFile(filepath.Join(pluginSrc, "plugin.json"), []byte(content), 0o644); err != nil {
		t.Fatalf("writing plugin.json: %v", err)
	}

	var buf bytes.Buffer
	if err := cli.PluginInstall(s, pluginSrc, &buf); err != nil {
		t.Fatalf("PluginInstall: %v", err)
	}
	if !strings.Contains(buf.String(), "simple-plugin") {
		t.Errorf("expected 'simple-plugin' in output, got: %q", buf.String())
	}
}

func TestExtractJSONField_MissingNameField(t *testing.T) {
	s, _ := newPluginTestStore(t)

	pluginSrc := t.TempDir()
	// No "name" field in the manifest.
	content := `{"version": "1.0.0", "executable": "x"}`
	if err := os.WriteFile(filepath.Join(pluginSrc, "plugin.json"), []byte(content), 0o644); err != nil {
		t.Fatalf("writing plugin.json: %v", err)
	}

	var buf bytes.Buffer
	err := cli.PluginInstall(s, pluginSrc, &buf)
	if err == nil {
		t.Fatal("expected error when 'name' field is missing")
	}
}

func TestExtractJSONField_EscapedQuoteInName(t *testing.T) {
	s, storeDir := newPluginTestStore(t)

	pluginSrc := t.TempDir()
	// Valid JSON — name contains an escaped quote character.
	content := `{"name": "test\"plugin", "version": "1.0.0", "executable": "x", "capabilities": []}`
	if err := os.WriteFile(filepath.Join(pluginSrc, "plugin.json"), []byte(content), 0o644); err != nil {
		t.Fatalf("writing plugin.json: %v", err)
	}

	var buf bytes.Buffer
	if err := cli.PluginInstall(s, pluginSrc, &buf); err != nil {
		t.Fatalf("PluginInstall: %v", err)
	}

	// The extracted name should be `test"plugin` (escaped quote resolved).
	destDir := filepath.Join(storeDir, "plugins", `test"plugin`)
	if _, err := os.Stat(destDir); err != nil {
		t.Errorf("plugin directory not found at %s: %v", destDir, err)
	}
}
