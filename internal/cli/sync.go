package cli

import (
	"fmt"
	"io"

	clog "github.com/charmbracelet/log"
	wyrdSync "github.com/jasonwarrenuk/wyrd/internal/sync"
	"github.com/jasonwarrenuk/wyrd/internal/types"
)

// SyncOptions holds optional parameters for the sync command.
type SyncOptions struct {
	// StorePath is the path to the store directory containing the git repo.
	StorePath string

	// Logger is the structured logger. May be nil.
	Logger *clog.Logger
}

// Sync runs the git sync cycle: stage all changes, commit, pull, push.
// Progress is written to out so the caller controls where it goes.
func Sync(store types.StoreFS, opts SyncOptions, out io.Writer) error {
	storePath := opts.StorePath
	if storePath == "" {
		storePath = store.StorePath()
	}

	fmt.Fprintln(out, "Syncing...")
	if err := wyrdSync.Sync(storePath, opts.Logger); err != nil {
		return err
	}
	fmt.Fprintln(out, "Sync complete.")
	return nil
}
