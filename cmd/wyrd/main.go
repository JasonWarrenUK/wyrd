// Command wyrd is the CLI entrypoint for the Wyrd personal productivity app.
package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"

	"github.com/spf13/cobra"

	"github.com/jasonwarrenuk/wyrd/internal/cli"
	"github.com/jasonwarrenuk/wyrd/internal/store"
	"github.com/jasonwarrenuk/wyrd/internal/types"
)

func main() {
	if err := rootCmd().Execute(); err != nil {
		os.Exit(1)
	}
}

// defaultStorePath returns ~/wyrd/store as the default store location.
func defaultStorePath() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return filepath.Join(".", "store")
	}
	return filepath.Join(home, "wyrd", "store")
}

// openStore initialises the store at storePath, running Init on first use.
func openStore(storePath string) (*store.FileStore, error) {
	if !cli.IsInitialised(storePath) {
		fmt.Fprintf(os.Stderr, "Initialising new Wyrd store at %s...\n", storePath)
		if err := cli.Init(storePath); err != nil {
			return nil, fmt.Errorf("initialising store: %w", err)
		}
	}
	return store.New(storePath), nil
}

func rootCmd() *cobra.Command {
	var storePath string

	root := &cobra.Command{
		Use:   "wyrd",
		Short: "Wyrd — a flat-file graph-based personal productivity tool",
		Long: `Wyrd is a terminal-based personal productivity tool backed by a flat-file
property graph. Run without arguments to launch the TUI.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			fmt.Fprintln(os.Stdout, "TUI coming soon")
			return nil
		},
	}

	root.PersistentFlags().StringVar(&storePath, "store", defaultStorePath(), "path to the Wyrd store directory")

	root.AddCommand(addCmd(&storePath))
	root.AddCommand(journalCmd(&storePath))
	root.AddCommand(noteCmd(&storePath))
	root.AddCommand(spendCmd(&storePath))
	root.AddCommand(syncCmd(&storePath))
	root.AddCommand(queryCmd(&storePath))
	root.AddCommand(viewCmd(&storePath))
	root.AddCommand(pushCmd(&storePath))
	root.AddCommand(pullCmd(&storePath))
	root.AddCommand(pluginCmd(&storePath))
	root.AddCommand(compactCmd(&storePath))

	return root
}

// addCmd implements `wyrd add`.
func addCmd(storePath *string) *cobra.Command {
	var nodeType string
	var linkID string

	cmd := &cobra.Command{
		Use:   "add <body>",
		Short: "Quick-add a task node",
		Long: `Create a new node from a body text argument.
Defaults to type 'task' with status 'inbox'.`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			s, err := openStore(*storePath)
			if err != nil {
				return err
			}
			id, err := cli.Add(s, cli.AddOptions{
				Body:     args[0],
				NodeType: nodeType,
				LinkID:   linkID,
			})
			if err != nil {
				return err
			}
			fmt.Fprintf(os.Stdout, "Created node %s\n", id)
			return nil
		},
	}

	cmd.Flags().StringVar(&nodeType, "type", "", "node type (default: task)")
	cmd.Flags().StringVar(&linkID, "link", "", "create a 'related' edge to this node ID")
	return cmd
}

// journalCmd implements `wyrd journal`.
func journalCmd(storePath *string) *cobra.Command {
	var linkID string

	cmd := &cobra.Command{
		Use:   "journal",
		Short: "Open $EDITOR and create a journal node",
		RunE: func(cmd *cobra.Command, args []string) error {
			s, err := openStore(*storePath)
			if err != nil {
				return err
			}
			id, err := cli.Journal(s, cli.JournalOptions{LinkID: linkID})
			if err != nil {
				return err
			}
			fmt.Fprintf(os.Stdout, "Created journal node %s\n", id)
			return nil
		},
	}

	cmd.Flags().StringVar(&linkID, "link", "", "create a 'related' edge to this node ID")
	return cmd
}

// noteCmd implements `wyrd note`.
func noteCmd(storePath *string) *cobra.Command {
	var linkID string

	cmd := &cobra.Command{
		Use:   "note <title>",
		Short: "Open $EDITOR and create a note node",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			s, err := openStore(*storePath)
			if err != nil {
				return err
			}
			id, err := cli.Note(s, cli.NoteOptions{
				Title:  args[0],
				LinkID: linkID,
			})
			if err != nil {
				return err
			}
			fmt.Fprintf(os.Stdout, "Created note node %s\n", id)
			return nil
		},
	}

	cmd.Flags().StringVar(&linkID, "link", "", "create a 'related' edge to this node ID")
	return cmd
}

// spendCmd implements `wyrd spend`.
func spendCmd(storePath *string) *cobra.Command {
	return &cobra.Command{
		Use:   "spend <category> <amount> <note>",
		Short: "Log a spend entry",
		Long: `Record a spending event under a budget category.
Amount must be a positive decimal number.`,
		Args: cobra.ExactArgs(3),
		RunE: func(cmd *cobra.Command, args []string) error {
			amount, err := strconv.ParseFloat(args[1], 64)
			if err != nil {
				return fmt.Errorf("invalid amount %q: must be a number", args[1])
			}
			s, err := openStore(*storePath)
			if err != nil {
				return err
			}
			id, err := cli.Spend(s, cli.SpendOptions{
				Category: args[0],
				Amount:   amount,
				Note:     args[2],
			})
			if err != nil {
				return err
			}
			fmt.Fprintf(os.Stdout, "Logged spend %s\n", id)
			return nil
		},
	}
}

// syncCmd implements `wyrd sync`.
func syncCmd(storePath *string) *cobra.Command {
	return &cobra.Command{
		Use:   "sync",
		Short: "Run the git sync cycle (stage, commit, pull, push)",
		RunE: func(cmd *cobra.Command, args []string) error {
			s, err := openStore(*storePath)
			if err != nil {
				return err
			}
			return cli.Sync(s, cli.SyncOptions{}, os.Stdout)
		},
	}
}

// queryCmd implements `wyrd query`.
func queryCmd(storePath *string) *cobra.Command {
	return &cobra.Command{
		Use:   "query <cypher>",
		Short: "Run a Cypher query and print results",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			_, err := openStore(*storePath)
			if err != nil {
				return err
			}
			// The query engine is implemented by a separate agent.
			// Stub: print a helpful message until it is wired in.
			return runQueryStub(args[0])
		},
	}
}

// runQueryStub prints a stub message for the query command.
func runQueryStub(query string) error {
	fmt.Fprintf(os.Stdout, "Query engine not yet available.\nQuery: %s\n", query)
	return nil
}

// viewCmd implements `wyrd view`.
func viewCmd(storePath *string) *cobra.Command {
	return &cobra.Command{
		Use:   "view <name>",
		Short: "Run a saved view and print results",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			_, err := openStore(*storePath)
			if err != nil {
				return err
			}
			// The query engine is implemented by a separate agent.
			// Stub: print a helpful message until it is wired in.
			fmt.Fprintf(os.Stdout, "Query engine not yet available.\nView: %s\n", args[0])
			return nil
		},
	}
}

// pushCmd implements `wyrd push`.
func pushCmd(storePath *string) *cobra.Command {
	return &cobra.Command{
		Use:   "push <node-id>",
		Short: "Push a node to Obsidian",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			s, err := openStore(*storePath)
			if err != nil {
				return err
			}
			return cli.Push(s, cli.PushOptions{NodeID: args[0]}, os.Stdout)
		},
	}
}

// pullCmd implements `wyrd pull`.
func pullCmd(storePath *string) *cobra.Command {
	var dryRun bool

	pull := &cobra.Command{
		Use:   "pull",
		Short: "Pull content from external sources",
	}

	obsidian := &cobra.Command{
		Use:   "obsidian <vault-path>",
		Short: "Pull notes from an Obsidian vault",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			s, err := openStore(*storePath)
			if err != nil {
				return err
			}
			return cli.PullObsidian(s, cli.PullObsidianOptions{
				VaultPath: args[0],
				DryRun:    dryRun,
			}, os.Stdout)
		},
	}

	obsidian.Flags().BoolVar(&dryRun, "dry-run", false, "preview what would be imported without writing nodes")
	pull.AddCommand(obsidian)
	return pull
}

// pluginCmd implements `wyrd plugin`.
func pluginCmd(storePath *string) *cobra.Command {
	plugin := &cobra.Command{
		Use:   "plugin",
		Short: "Manage Wyrd plugins",
	}

	install := &cobra.Command{
		Use:   "install <path>",
		Short: "Install a plugin from a directory or zip archive",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			s, err := openStore(*storePath)
			if err != nil {
				return err
			}
			return cli.PluginInstall(s, args[0], os.Stdout)
		},
	}

	export := &cobra.Command{
		Use:   "export <name>",
		Short: "Export an installed plugin to a zip archive",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			s, err := openStore(*storePath)
			if err != nil {
				return err
			}
			return cli.PluginExport(s, args[0], os.Stdout)
		},
	}

	list := &cobra.Command{
		Use:   "list",
		Short: "List installed plugins",
		RunE: func(cmd *cobra.Command, args []string) error {
			s, err := openStore(*storePath)
			if err != nil {
				return err
			}
			return cli.PluginList(s, os.Stdout)
		},
	}

	plugin.AddCommand(install, export, list)
	return plugin
}

// compactCmd implements `wyrd compact`.
func compactCmd(storePath *string) *cobra.Command {
	return &cobra.Command{
		Use:   "compact",
		Short: "Archive old nodes to reduce store size (coming soon)",
		RunE: func(cmd *cobra.Command, args []string) error {
			_ = storePath
			fmt.Fprintln(os.Stdout, "Compact is not yet available — coming in a future release.")
			return nil
		},
	}
}

// Ensure FileStore satisfies both StoreFS and PluginStore.
var _ types.StoreFS = (*store.FileStore)(nil)
var _ types.PluginStore = (*store.FileStore)(nil)
