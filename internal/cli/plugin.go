package cli

import (
	"archive/zip"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/jasonwarrenuk/wyrd/internal/types"
)

// PluginInstall installs a plugin from a directory or zip archive at sourcePath.
func PluginInstall(store types.StoreFS, sourcePath string, out io.Writer) error {
	if sourcePath == "" {
		return &types.ValidationError{Field: "path", Message: "plugin source path must not be empty"}
	}

	info, err := os.Stat(sourcePath)
	if err != nil {
		return fmt.Errorf("cannot access %q: %w", sourcePath, err)
	}

	var pluginDir string
	if info.IsDir() {
		pluginDir = sourcePath
	} else if strings.HasSuffix(sourcePath, ".zip") {
		var err error
		pluginDir, err = extractPluginZip(sourcePath)
		if err != nil {
			return fmt.Errorf("extracting plugin zip: %w", err)
		}
		defer os.RemoveAll(pluginDir)
	} else {
		return fmt.Errorf("unsupported plugin source format — provide a directory or .zip file")
	}

	// Read the manifest to get the plugin name.
	manifestPath := filepath.Join(pluginDir, "plugin.json")
	data, err := os.ReadFile(manifestPath)
	if err != nil {
		return fmt.Errorf("reading plugin manifest from %q: %w", manifestPath, err)
	}

	// Quick name extraction to avoid a full JSON decode dependency here.
	name := extractJSONField(data, "name")
	if name == "" {
		return fmt.Errorf("plugin manifest missing 'name' field at %q", manifestPath)
	}

	destDir := filepath.Join(store.StorePath(), "plugins", name)
	if err := copyDir(pluginDir, destDir); err != nil {
		return fmt.Errorf("copying plugin files: %w", err)
	}

	fmt.Fprintf(out, "Plugin %q installed to %s\n", name, destDir)
	return nil
}

// PluginExport exports a named plugin to a zip archive in the current directory.
func PluginExport(store types.StoreFS, name string, out io.Writer) error {
	if name == "" {
		return &types.ValidationError{Field: "name", Message: "plugin name must not be empty"}
	}

	pluginDir := filepath.Join(store.StorePath(), "plugins", name)
	if _, err := os.Stat(pluginDir); err != nil {
		return fmt.Errorf("plugin %q not found — run 'wyrd plugin list' to see installed plugins", name)
	}

	zipPath := name + ".zip"
	if err := createZip(pluginDir, zipPath); err != nil {
		return fmt.Errorf("creating zip: %w", err)
	}

	fmt.Fprintf(out, "Plugin %q exported to %s\n", name, zipPath)
	return nil
}

// PluginList prints all installed plugins with their version and description.
func PluginList(store types.PluginStore, out io.Writer) error {
	manifests, err := store.AllPluginManifests()
	if err != nil {
		return fmt.Errorf("listing plugins: %w", err)
	}

	if len(manifests) == 0 {
		fmt.Fprintln(out, "No plugins installed. Use 'wyrd plugin install <path>' to add one.")
		return nil
	}

	for _, m := range manifests {
		desc := m.Description
		if desc == "" {
			desc = "(no description)"
		}
		fmt.Fprintf(out, "%-20s %s  %s\n", m.Name, m.Version, desc)
	}
	return nil
}

// extractPluginZip extracts a zip file to a temporary directory and returns the path.
func extractPluginZip(zipPath string) (string, error) {
	r, err := zip.OpenReader(zipPath)
	if err != nil {
		return "", err
	}
	defer r.Close()

	tmpDir, err := os.MkdirTemp("", "wyrd-plugin-*")
	if err != nil {
		return "", err
	}

	for _, f := range r.File {
		dest := filepath.Join(tmpDir, filepath.Clean(f.Name))
		if f.FileInfo().IsDir() {
			os.MkdirAll(dest, 0o755)
			continue
		}
		if err := os.MkdirAll(filepath.Dir(dest), 0o755); err != nil {
			return "", err
		}
		rc, err := f.Open()
		if err != nil {
			return "", err
		}
		out, err := os.Create(dest)
		if err != nil {
			rc.Close()
			return "", err
		}
		_, copyErr := io.Copy(out, rc)
		rc.Close()
		out.Close()
		if copyErr != nil {
			return "", copyErr
		}
	}
	return tmpDir, nil
}

// createZip archives srcDir into zipPath.
func createZip(srcDir, zipPath string) error {
	f, err := os.Create(zipPath)
	if err != nil {
		return err
	}
	defer f.Close()

	w := zip.NewWriter(f)
	defer w.Close()

	return filepath.Walk(srcDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			return nil
		}
		rel, err := filepath.Rel(srcDir, path)
		if err != nil {
			return err
		}
		zf, err := w.Create(rel)
		if err != nil {
			return err
		}
		src, err := os.Open(path)
		if err != nil {
			return err
		}
		defer src.Close()
		_, err = io.Copy(zf, src)
		return err
	})
}

// copyDir recursively copies src to dst.
func copyDir(src, dst string) error {
	return filepath.Walk(src, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(src, path)
		if err != nil {
			return err
		}
		destPath := filepath.Join(dst, rel)
		if info.IsDir() {
			return os.MkdirAll(destPath, 0o755)
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		return os.WriteFile(destPath, data, info.Mode())
	})
}

// extractJSONField performs a minimal extraction of a top-level string field
// from raw JSON without a full unmarshal. Used only for reading the plugin name.
func extractJSONField(data []byte, field string) string {
	key := `"` + field + `"`
	s := string(data)
	idx := strings.Index(s, key)
	if idx < 0 {
		return ""
	}
	rest := s[idx+len(key):]
	// Skip whitespace and colon.
	rest = strings.TrimLeft(rest, " \t\n\r:")
	rest = strings.TrimLeft(rest, " \t\n\r")
	if len(rest) == 0 || rest[0] != '"' {
		return ""
	}
	rest = rest[1:]
	end := strings.IndexByte(rest, '"')
	if end < 0 {
		return ""
	}
	return rest[:end]
}
