package plugin

import (
	"archive/zip"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

// pluginArchiveExtension is the file extension for Wyrd plugin archives.
const pluginArchiveExtension = ".wyrdplugin"

// Install extracts a .wyrdplugin archive into {storePath}/plugins/ and installs
// any binaries found in the archive's bin/ directory into ~/.wyrd/bin/.
func Install(archivePath string, storePath string) error {
	if !strings.HasSuffix(archivePath, pluginArchiveExtension) {
		return fmt.Errorf("install: %q does not have a %s extension", archivePath, pluginArchiveExtension)
	}

	r, err := zip.OpenReader(archivePath)
	if err != nil {
		return fmt.Errorf("install: open archive %s: %w", archivePath, err)
	}
	defer r.Close()

	// Determine the plugin name from the archive (expect a single top-level directory).
	pluginName, err := archivePluginName(r)
	if err != nil {
		return fmt.Errorf("install: %w", err)
	}

	destDir := filepath.Join(storePath, "plugins", pluginName)
	if err := os.MkdirAll(destDir, 0o755); err != nil {
		return fmt.Errorf("install: create plugin directory: %w", err)
	}

	wyrdBinDir := filepath.Join(localWyrdDir(), "bin")
	if err := os.MkdirAll(wyrdBinDir, 0o755); err != nil {
		return fmt.Errorf("install: create bin directory: %w", err)
	}

	for _, f := range r.File {
		if err := extractFile(f, pluginName, destDir, wyrdBinDir); err != nil {
			return fmt.Errorf("install: extract %s: %w", f.Name, err)
		}
	}

	return nil
}

// Export zips the plugin directory at {storePath}/plugins/{name} into a
// .wyrdplugin archive at outPath.
func Export(name string, storePath string, outPath string) error {
	pluginDir := filepath.Join(storePath, "plugins", name)
	if _, err := os.Stat(pluginDir); err != nil {
		return fmt.Errorf("export: plugin directory %s: %w", pluginDir, err)
	}

	if !strings.HasSuffix(outPath, pluginArchiveExtension) {
		outPath += pluginArchiveExtension
	}

	f, err := os.Create(outPath)
	if err != nil {
		return fmt.Errorf("export: create archive %s: %w", outPath, err)
	}
	defer f.Close()

	w := zip.NewWriter(f)
	defer w.Close()

	// Walk the plugin directory, adding files relative to the parent of pluginDir
	// so that the archive contains a top-level {name}/ directory.
	parentDir := filepath.Dir(pluginDir)
	err = filepath.Walk(pluginDir, func(path string, info os.FileInfo, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if info.IsDir() {
			return nil
		}

		rel, err := filepath.Rel(parentDir, path)
		if err != nil {
			return err
		}
		// Use forward slashes inside the archive (zip spec).
		rel = filepath.ToSlash(rel)

		entry, err := w.Create(rel)
		if err != nil {
			return err
		}

		src, err := os.Open(path)
		if err != nil {
			return err
		}
		defer src.Close()

		_, err = io.Copy(entry, src)
		return err
	})

	return err
}

// archivePluginName inspects the archive entries to determine the top-level
// plugin directory name. All entries must be under a single top-level directory.
func archivePluginName(r *zip.ReadCloser) (string, error) {
	var name string
	for _, f := range r.File {
		parts := strings.SplitN(filepath.ToSlash(f.Name), "/", 2)
		topLevel := parts[0]
		if name == "" {
			name = topLevel
		} else if name != topLevel {
			return "", fmt.Errorf("archive contains multiple top-level directories (%s, %s)", name, topLevel)
		}
	}
	if name == "" {
		return "", fmt.Errorf("archive is empty")
	}
	return name, nil
}

// extractFile extracts a single zip entry. Binaries under bin/ are placed in
// wyrdBinDir; all other files go under destDir (relative to the top-level dir).
func extractFile(f *zip.File, pluginName, destDir, wyrdBinDir string) error {
	// Strip the top-level plugin directory prefix.
	rel := filepath.ToSlash(f.Name)
	prefix := pluginName + "/"
	rel = strings.TrimPrefix(rel, prefix)

	var destPath string
	if strings.HasPrefix(rel, "bin/") {
		// Install binary to ~/.wyrd/bin/.
		binName := strings.TrimPrefix(rel, "bin/")
		if binName == "" || strings.Contains(binName, "/") {
			// Skip sub-directories within bin/.
			return nil
		}
		destPath = filepath.Join(wyrdBinDir, filepath.FromSlash(binName))
	} else {
		destPath = filepath.Join(destDir, filepath.FromSlash(rel))
	}

	// Guard against zip-slip: ensure the destination is within the expected tree.
	if err := validateDestination(destPath, destDir, wyrdBinDir); err != nil {
		return err
	}

	if f.FileInfo().IsDir() {
		return os.MkdirAll(destPath, 0o755)
	}

	if err := os.MkdirAll(filepath.Dir(destPath), 0o755); err != nil {
		return err
	}

	rc, err := f.Open()
	if err != nil {
		return err
	}
	defer rc.Close()

	mode := f.Mode()
	dst, err := os.OpenFile(destPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, mode)
	if err != nil {
		return err
	}
	defer dst.Close()

	_, err = io.Copy(dst, rc)
	return err
}

// validateDestination checks that path is rooted under one of the allowed
// base directories, preventing zip-slip attacks.
func validateDestination(path, destDir, wyrdBinDir string) error {
	absPath, err := filepath.Abs(path)
	if err != nil {
		return err
	}
	absDestDir, err := filepath.Abs(destDir)
	if err != nil {
		return err
	}
	absWyrdBinDir, err := filepath.Abs(wyrdBinDir)
	if err != nil {
		return err
	}

	if strings.HasPrefix(absPath, absDestDir+string(os.PathSeparator)) ||
		strings.HasPrefix(absPath, absWyrdBinDir+string(os.PathSeparator)) ||
		absPath == absDestDir ||
		absPath == absWyrdBinDir {
		return nil
	}
	return fmt.Errorf("zip-slip detected: %q is outside allowed directories", path)
}
