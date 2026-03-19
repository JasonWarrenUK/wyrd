package cli

import (
	"fmt"
	"io"

	"github.com/jasonwarrenuk/wyrd/internal/types"
)

// PullObsidianOptions holds the parameters for the pull obsidian command.
type PullObsidianOptions struct {
	// VaultPath is the path to the Obsidian vault to pull from.
	VaultPath string

	// DryRun logs what would be imported without writing any nodes.
	DryRun bool
}

// PullObsidian imports nodes from an Obsidian vault. Currently a stub —
// the Obsidian sync engine is implemented by a separate agent.
func PullObsidian(store types.StoreFS, opts PullObsidianOptions, out io.Writer) error {
	if opts.VaultPath == "" {
		return &types.ValidationError{Field: "vault", Message: "vault path must not be empty"}
	}

	_ = store // will be used when the Obsidian engine is wired in

	if opts.DryRun {
		fmt.Fprintf(out, "[dry-run] Pull from Obsidian vault %q — no changes written.\n", opts.VaultPath)
		fmt.Fprintln(out, "Obsidian pull is not yet available — coming soon.")
		return nil
	}

	fmt.Fprintf(out, "Pull from Obsidian vault %q — not yet available. Use --dry-run to preview.\n", opts.VaultPath)
	return nil
}
