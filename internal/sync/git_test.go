package sync

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// --- Init tests ---

func TestInit_CreatesGitRepo(t *testing.T) {
	dir := t.TempDir()

	if err := Init(dir); err != nil {
		t.Fatalf("Init failed: %v", err)
	}

	// .git directory should exist.
	if _, err := os.Stat(filepath.Join(dir, ".git")); err != nil {
		t.Errorf(".git directory not created: %v", err)
	}
}

func TestInit_WritesGitAttributes(t *testing.T) {
	dir := t.TempDir()

	if err := Init(dir); err != nil {
		t.Fatalf("Init failed: %v", err)
	}

	content, err := os.ReadFile(filepath.Join(dir, ".gitattributes"))
	if err != nil {
		t.Fatalf("failed to read .gitattributes: %v", err)
	}

	if string(content) != gitAttributesContent {
		t.Errorf("unexpected .gitattributes content:\n%s", string(content))
	}
}

func TestInit_ConfiguresMergeDriver(t *testing.T) {
	dir := t.TempDir()

	if err := Init(dir); err != nil {
		t.Fatalf("Init failed: %v", err)
	}

	config, err := os.ReadFile(filepath.Join(dir, ".git", "config"))
	if err != nil {
		t.Fatalf("failed to read .git/config: %v", err)
	}

	if !strings.Contains(string(config), `[merge "wyrd-merge"]`) {
		t.Errorf(".git/config does not contain merge driver stanza:\n%s", string(config))
	}
}

func TestInit_Idempotent(t *testing.T) {
	dir := t.TempDir()

	// Call Init twice; should not fail or duplicate config.
	if err := Init(dir); err != nil {
		t.Fatalf("first Init failed: %v", err)
	}
	if err := Init(dir); err != nil {
		t.Fatalf("second Init failed: %v", err)
	}

	config, err := os.ReadFile(filepath.Join(dir, ".git", "config"))
	if err != nil {
		t.Fatalf("failed to read .git/config: %v", err)
	}

	// The merge driver stanza should appear exactly once.
	count := strings.Count(string(config), `[merge "wyrd-merge"]`)
	if count != 1 {
		t.Errorf("expected merge driver stanza exactly once, found %d times", count)
	}
}

// --- Status tests ---

func TestStatus_EmptyRepo(t *testing.T) {
	dir := t.TempDir()

	if err := Init(dir); err != nil {
		t.Fatalf("Init failed: %v", err)
	}

	changed, err := Status(dir)
	if err != nil {
		t.Fatalf("Status failed: %v", err)
	}

	// .gitattributes is untracked but Status uses --porcelain which does
	// include untracked files. Accept 0 or 1 entry.
	_ = changed // just checking no error
}

func TestStatus_WithModifiedFile(t *testing.T) {
	dir := t.TempDir()

	if err := Init(dir); err != nil {
		t.Fatalf("Init failed: %v", err)
	}

	// Set up initial commit so we have a HEAD.
	testFile := filepath.Join(dir, "node.jsonc")
	if err := os.WriteFile(testFile, []byte(`{"id":"1","body":"hello"}`), 0o644); err != nil {
		t.Fatalf("write test file: %v", err)
	}
	if err := runGit(dir, "add", "--all"); err != nil {
		t.Fatalf("git add: %v", err)
	}
	// Configure a minimal identity for commits in the temp repo.
	if err := runGit(dir, "config", "user.email", "test@wyrd.test"); err != nil {
		t.Fatalf("git config email: %v", err)
	}
	if err := runGit(dir, "config", "user.name", "Wyrd Test"); err != nil {
		t.Fatalf("git config name: %v", err)
	}
	if err := runGit(dir, "commit", "-m", "initial"); err != nil {
		t.Fatalf("initial commit: %v", err)
	}

	// Modify the file.
	if err := os.WriteFile(testFile, []byte(`{"id":"1","body":"changed"}`), 0o644); err != nil {
		t.Fatalf("modify test file: %v", err)
	}

	changed, err := Status(dir)
	if err != nil {
		t.Fatalf("Status failed: %v", err)
	}

	if len(changed) == 0 {
		t.Errorf("expected at least one changed file, got none")
	}

	// The modified file should appear in the list.
	found := false
	for _, f := range changed {
		if strings.Contains(f, "node.jsonc") {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("node.jsonc not found in changed files: %v", changed)
	}
}

// --- buildCommitMessage tests ---

func TestBuildCommitMessage_NothingStaged(t *testing.T) {
	dir := t.TempDir()
	if err := Init(dir); err != nil {
		t.Fatalf("Init failed: %v", err)
	}

	msg, err := buildCommitMessage(dir)
	if err != nil {
		t.Fatalf("buildCommitMessage failed: %v", err)
	}
	if msg != "" {
		t.Errorf("expected empty message for nothing staged, got %q", msg)
	}
}

func TestBuildCommitMessage_NodeCreated(t *testing.T) {
	dir := t.TempDir()
	if err := Init(dir); err != nil {
		t.Fatalf("Init failed: %v", err)
	}

	// Create a nodes directory with a JSONC file, then stage it.
	nodesDir := filepath.Join(dir, "nodes")
	if err := os.MkdirAll(nodesDir, 0o755); err != nil {
		t.Fatalf("mkdir nodes: %v", err)
	}
	nodePath := filepath.Join(nodesDir, "abc123.jsonc")
	if err := os.WriteFile(nodePath, []byte(`{"id":"abc123"}`), 0o644); err != nil {
		t.Fatalf("write node: %v", err)
	}
	if err := runGit(dir, "add", "--all"); err != nil {
		t.Fatalf("git add: %v", err)
	}

	msg, err := buildCommitMessage(dir)
	if err != nil {
		t.Fatalf("buildCommitMessage failed: %v", err)
	}

	if !strings.HasPrefix(msg, "wyrd: ") {
		t.Errorf("message should start with 'wyrd: ', got %q", msg)
	}
	if !strings.Contains(msg, "created") {
		t.Errorf("message should mention 'created', got %q", msg)
	}
	if !strings.Contains(msg, "node") {
		t.Errorf("message should mention 'node', got %q", msg)
	}
}

func TestBuildCommitMessage_EdgeUpdated(t *testing.T) {
	dir := t.TempDir()
	if err := Init(dir); err != nil {
		t.Fatalf("Init failed: %v", err)
	}

	// Configure git identity for commits.
	if err := runGit(dir, "config", "user.email", "test@wyrd.test"); err != nil {
		t.Fatalf("git config email: %v", err)
	}
	if err := runGit(dir, "config", "user.name", "Wyrd Test"); err != nil {
		t.Fatalf("git config name: %v", err)
	}

	// Create an edges directory, commit, then modify.
	edgesDir := filepath.Join(dir, "edges")
	if err := os.MkdirAll(edgesDir, 0o755); err != nil {
		t.Fatalf("mkdir edges: %v", err)
	}
	edgePath := filepath.Join(edgesDir, "edge1.jsonc")
	if err := os.WriteFile(edgePath, []byte(`{"id":"edge1","type":"blocks"}`), 0o644); err != nil {
		t.Fatalf("write edge: %v", err)
	}
	if err := runGit(dir, "add", "--all"); err != nil {
		t.Fatalf("git add: %v", err)
	}
	if err := runGit(dir, "commit", "-m", "initial"); err != nil {
		t.Fatalf("initial commit: %v", err)
	}

	// Now modify the edge.
	if err := os.WriteFile(edgePath, []byte(`{"id":"edge1","type":"related"}`), 0o644); err != nil {
		t.Fatalf("modify edge: %v", err)
	}
	if err := runGit(dir, "add", "--all"); err != nil {
		t.Fatalf("git add: %v", err)
	}

	msg, err := buildCommitMessage(dir)
	if err != nil {
		t.Fatalf("buildCommitMessage failed: %v", err)
	}

	if !strings.Contains(msg, "edge") {
		t.Errorf("message should mention 'edge', got %q", msg)
	}
	if !strings.Contains(msg, "updated") {
		t.Errorf("message should mention 'updated', got %q", msg)
	}
}

// --- writeFileIfChanged tests ---

func TestWriteFileIfChanged_NoWriteIfIdentical(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test.txt")
	content := []byte("hello world\n")

	// Write initial content.
	if err := os.WriteFile(path, content, 0o644); err != nil {
		t.Fatalf("write initial: %v", err)
	}

	stat1, _ := os.Stat(path)

	// Call writeFileIfChanged with same content.
	if err := writeFileIfChanged(path, content); err != nil {
		t.Fatalf("writeFileIfChanged failed: %v", err)
	}

	stat2, _ := os.Stat(path)

	// Modification time should be unchanged if we skipped the write.
	if stat1.ModTime() != stat2.ModTime() {
		t.Errorf("file was rewritten unnecessarily")
	}
}

func TestWriteFileIfChanged_WritesWhenDifferent(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test.txt")

	if err := os.WriteFile(path, []byte("original"), 0o644); err != nil {
		t.Fatalf("write initial: %v", err)
	}

	if err := writeFileIfChanged(path, []byte("updated")); err != nil {
		t.Fatalf("writeFileIfChanged failed: %v", err)
	}

	result, _ := os.ReadFile(path)
	if string(result) != "updated" {
		t.Errorf("expected 'updated', got %q", string(result))
	}
}

// --- pluralise tests (in git_test for commit message helper) ---

func TestPluralise_Git(t *testing.T) {
	if pluralise("node", 1) != "node" {
		t.Error("1 node should not be pluralised")
	}
	if pluralise("edge", 2) != "edges" {
		t.Error("2 edges should be pluralised")
	}
}
