// Command wyrd-merge-driver is invoked by git as a custom three-way merge
// driver for JSONC files in the Wyrd store.
//
// Usage (configured automatically by wyrd sync init):
//
//	wyrd merge-driver %O %A %B
//
// Arguments (passed by git):
//
//	%O — base (common ancestor) version
//	%A — ours (current branch) version — the result is written back here
//	%B — theirs (incoming branch) version
//
// The driver exits 0 on a successful merge and non-zero when the merge cannot
// be resolved automatically. Per the git merge driver contract, the merged
// result must be written to the %A path on success.
package main

import (
	"fmt"
	"os"

	"github.com/jasonwarrenuk/wyrd/internal/sync"
)

func main() {
	if len(os.Args) != 4 {
		fmt.Fprintf(os.Stderr, "usage: wyrd-merge-driver <base> <ours> <theirs>\n")
		fmt.Fprintf(os.Stderr, "  base   — common ancestor file path (%%O)\n")
		fmt.Fprintf(os.Stderr, "  ours   — current branch file path  (%%A) — result written here\n")
		fmt.Fprintf(os.Stderr, "  theirs — incoming branch file path  (%%B)\n")
		os.Exit(2)
	}

	basePath := os.Args[1]
	oursPath := os.Args[2]
	theirsPath := os.Args[3]

	if err := sync.MergeFiles(basePath, oursPath, theirsPath); err != nil {
		fmt.Fprintf(os.Stderr, "wyrd-merge-driver: merge failed: %v\n", err)
		os.Exit(1)
	}

	os.Exit(0)
}
