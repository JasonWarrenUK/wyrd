// Package cli provides the command-line interface for Wyrd.
package cli

import (
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"

	wyrdEmbed "github.com/jasonwarrenuk/wyrd/internal/embed"
)

// storeDirs are the subdirectories created inside the store on initialisation.
var storeDirs = []string{
	"nodes",
	"edges",
	"templates",
	"themes",
	"rituals",
	"views",
	"plugins",
}

// Init initialises a new Wyrd store at storePath.
// It creates the directory structure, copies embedded starter files,
// writes a default config.jsonc, initialises git, and creates .gitattributes.
// Returns an error if the store is already initialised.
func Init(storePath string) error {
	if IsInitialised(storePath) {
		return fmt.Errorf("store already initialised at %q — run 'wyrd' to get started", storePath)
	}

	// Create the store root and all subdirectories.
	for _, sub := range storeDirs {
		dir := filepath.Join(storePath, sub)
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return fmt.Errorf("creating directory %q: %w", dir, err)
		}
	}

	// Copy embedded starter files into the store.
	if err := copyStarterFiles(storePath); err != nil {
		return fmt.Errorf("copying starter files: %w", err)
	}

	// Initialise git in the store directory.
	if err := initGit(storePath); err != nil {
		return fmt.Errorf("initialising git: %w", err)
	}

	// Write .gitattributes for the wyrd-merge driver.
	if err := writeGitAttributes(storePath); err != nil {
		return fmt.Errorf("writing .gitattributes: %w", err)
	}

	return nil
}

// IsInitialised reports whether storePath has already been set up as a Wyrd store.
// It checks for the existence of the nodes/ subdirectory as a sentinel.
func IsInitialised(storePath string) bool {
	nodesDir := filepath.Join(storePath, "nodes")
	info, err := os.Stat(nodesDir)
	return err == nil && info.IsDir()
}

// copyStarterFiles walks the embedded starter/ directory and copies each file
// to the corresponding path within storePath.
func copyStarterFiles(storePath string) error {
	return fs.WalkDir(wyrdEmbed.StarterFS, "starter", func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		// Compute the destination path by stripping the leading "starter/" prefix.
		rel, err := filepath.Rel("starter", path)
		if err != nil {
			return err
		}
		dest := filepath.Join(storePath, rel)

		if d.IsDir() {
			return os.MkdirAll(dest, 0o755)
		}

		data, err := wyrdEmbed.StarterFS.ReadFile(path)
		if err != nil {
			return fmt.Errorf("reading embedded file %q: %w", path, err)
		}

		// Do not overwrite files the user may have edited.
		if _, statErr := os.Stat(dest); statErr == nil {
			return nil
		}

		return os.WriteFile(dest, data, 0o644)
	})
}

// initGit runs `git init` inside storePath.
func initGit(storePath string) error {
	cmd := exec.Command("git", "init", storePath)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("git init failed: %w", err)
	}
	return nil
}

// writeGitAttributes writes a .gitattributes file that registers the
// wyrd-merge driver for all JSONC files.
func writeGitAttributes(storePath string) error {
	content := "*.jsonc merge=wyrd-merge\n"
	path := filepath.Join(storePath, ".gitattributes")
	return os.WriteFile(path, []byte(content), 0o644)
}
