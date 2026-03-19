package cli

import (
	"fmt"
	"io"
	"os/exec"

	"github.com/jasonwarrenuk/wyrd/internal/types"
)

// SyncOptions holds optional parameters for the sync command.
type SyncOptions struct {
	// StorePath is the path to the store directory containing the git repo.
	StorePath string
}

// Sync runs the git sync cycle: stage all changes, commit, pull, push.
// Progress is written to out so the caller controls where it goes.
func Sync(store types.StoreFS, opts SyncOptions, out io.Writer) error {
	storePath := opts.StorePath
	if storePath == "" {
		storePath = store.StorePath()
	}

	steps := []struct {
		name string
		fn   func() error
	}{
		{"Staging changes", func() error { return gitRun(storePath, "add", ".") }},
		{"Committing", func() error {
			return gitRun(storePath, "commit", "-m", "wyrd: sync", "--allow-empty")
		}},
		{"Pulling from remote", func() error {
			return gitRun(storePath, "pull", "--rebase", "--autostash")
		}},
		{"Pushing to remote", func() error {
			return gitRun(storePath, "push")
		}},
	}

	for _, step := range steps {
		fmt.Fprintf(out, "  → %s...\n", step.name)
		if err := step.fn(); err != nil {
			return fmt.Errorf("%s failed: %w\n\nCheck that 'sync_remote' is set in your config and the remote is reachable", step.name, err)
		}
	}

	fmt.Fprintln(out, "Sync complete.")
	return nil
}

// gitRun runs a git subcommand in the given directory.
func gitRun(dir string, args ...string) error {
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("git %v: %s", args, string(out))
	}
	return nil
}
