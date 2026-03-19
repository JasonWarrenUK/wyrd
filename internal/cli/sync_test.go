package cli_test

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"github.com/jasonwarrenuk/wyrd/internal/cli"
	"github.com/jasonwarrenuk/wyrd/internal/store"
	wyrdSync "github.com/jasonwarrenuk/wyrd/internal/sync"
	"github.com/jasonwarrenuk/wyrd/internal/types"
)

func TestSync_NoRemote(t *testing.T) {
	// Initialise a real git repo so that git stage and commit succeed,
	// but there is no remote — push should fail with an actionable error.
	storeDir := t.TempDir()
	s, err := store.New(storeDir, types.RealClock{})
	if err != nil {
		t.Fatalf("opening store: %v", err)
	}

	// Initialise git in the store directory (store.New already created dirs,
	// so use the sync package directly rather than cli.Init which would reject
	// an already-initialised store).
	if err := wyrdSync.Init(storeDir); err != nil {
		t.Fatalf("sync.Init failed: %v", err)
	}

	// Set git user config so commit doesn't fail on CI.
	setGitConfig(t, storeDir, "user.email", "test@example.com")
	setGitConfig(t, storeDir, "user.name", "Test")

	var out bytes.Buffer
	err = cli.Sync(s, cli.SyncOptions{StorePath: storeDir}, &out)
	if err == nil {
		t.Log("Sync output:", out.String())
		t.Fatal("expected error when no remote is configured, got nil")
	}
	// Error message should be actionable.
	if len(err.Error()) == 0 {
		t.Error("error message is empty")
	}
}

// setGitConfig sets a git config value in the given directory.
func setGitConfig(t *testing.T, dir, key, value string) {
	t.Helper()
	script := filepath.Join(t.TempDir(), "gitconfig.sh")
	content := "#!/bin/sh\ngit -C " + dir + " config " + key + " \"" + value + "\"\n"
	if err := os.WriteFile(script, []byte(content), 0o755); err != nil {
		t.Fatalf("writing git config script: %v", err)
	}
	if err := os.Chmod(script, 0o755); err != nil {
		t.Fatalf("chmod script: %v", err)
	}
	// Run via sh to avoid exec dependency.
	cmd := commandFromPath(script)
	if err := cmd.Run(); err != nil {
		t.Logf("git config %s %s: %v (non-fatal)", key, value, err)
	}
}
