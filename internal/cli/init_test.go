package cli_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/jasonwarrenuk/wyrd/internal/cli"
)

func TestInit_CreatesDirectoryStructure(t *testing.T) {
	storeDir := t.TempDir()

	if err := cli.Init(storeDir); err != nil {
		t.Fatalf("Init returned unexpected error: %v", err)
	}

	expectedDirs := []string{
		"nodes", "edges", "templates", "themes", "rituals", "views", "plugins",
	}
	for _, dir := range expectedDirs {
		path := filepath.Join(storeDir, dir)
		info, err := os.Stat(path)
		if err != nil {
			t.Errorf("expected directory %q to exist, got error: %v", path, err)
			continue
		}
		if !info.IsDir() {
			t.Errorf("expected %q to be a directory", path)
		}
	}
}

func TestInit_CopiesStarterFiles(t *testing.T) {
	storeDir := t.TempDir()

	if err := cli.Init(storeDir); err != nil {
		t.Fatalf("Init returned unexpected error: %v", err)
	}

	expectedFiles := []string{
		filepath.Join("templates", "task.jsonc"),
		filepath.Join("templates", "journal.jsonc"),
		filepath.Join("templates", "note.jsonc"),
		filepath.Join("themes", "cairn.jsonc"),
		filepath.Join("views", "today.jsonc"),
		"config.jsonc",
	}
	for _, rel := range expectedFiles {
		path := filepath.Join(storeDir, rel)
		if _, err := os.Stat(path); err != nil {
			t.Errorf("expected file %q to exist: %v", path, err)
		}
	}
}

func TestInit_CreatesGitAttributes(t *testing.T) {
	storeDir := t.TempDir()

	if err := cli.Init(storeDir); err != nil {
		t.Fatalf("Init returned unexpected error: %v", err)
	}

	path := filepath.Join(storeDir, ".gitattributes")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf(".gitattributes not created: %v", err)
	}
	content := string(data)
	if content != "*.jsonc merge=wyrd-merge\n" {
		t.Errorf(".gitattributes content = %q, want %q", content, "*.jsonc merge=wyrd-merge\n")
	}
}

func TestIsInitialised_EmptyDir(t *testing.T) {
	dir := t.TempDir()
	if cli.IsInitialised(dir) {
		t.Fatal("expected IsInitialised to return false for an empty directory")
	}
}

func TestIsInitialised_AfterInit(t *testing.T) {
	dir := t.TempDir()

	if err := cli.Init(dir); err != nil {
		t.Fatalf("Init failed: %v", err)
	}

	if !cli.IsInitialised(dir) {
		t.Fatal("expected IsInitialised to return true after Init")
	}
}

func TestInit_IdempotentOnSecondCall(t *testing.T) {
	dir := t.TempDir()

	if err := cli.Init(dir); err != nil {
		t.Fatalf("first Init failed: %v", err)
	}

	// Second call should return an error because the store already exists.
	err := cli.Init(dir)
	if err == nil {
		t.Fatal("expected error on second Init call, got nil")
	}
}
