// Command wyrd is the CLI entrypoint for the Wyrd personal productivity app.
package main

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	clog "github.com/charmbracelet/log"
	huh "charm.land/huh/v2"
	"github.com/spf13/cobra"

	"github.com/jasonwarrenuk/wyrd/internal/cli"
	"github.com/jasonwarrenuk/wyrd/internal/query"
	"github.com/jasonwarrenuk/wyrd/internal/store"
	"github.com/jasonwarrenuk/wyrd/internal/tui"
	"github.com/jasonwarrenuk/wyrd/internal/types"
)

// logger is the application-wide structured logger, initialised in
// PersistentPreRunE. Nil until the root command runs.
var appLogger *clog.Logger

// logFilePath returns ~/.wyrd/wyrd.log.
func logFilePath() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return filepath.Join(".", "wyrd.log")
	}
	return filepath.Join(home, ".wyrd", "wyrd.log")
}

// parseLogLevel maps a string to a charmbracelet/log level.
// Returns InfoLevel as the default.
func parseLogLevel(s string) clog.Level {
	switch strings.ToLower(s) {
	case "debug":
		return clog.DebugLevel
	case "info":
		return clog.InfoLevel
	case "warn":
		return clog.WarnLevel
	case "error":
		return clog.ErrorLevel
	default:
		return clog.InfoLevel
	}
}

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
func openStore(storePath string) (*store.Store, error) {
	if !cli.IsInitialised(storePath) {
		fmt.Fprintf(os.Stderr, "Initialising new Wyrd store at %s...\n", storePath)
		if err := cli.Init(storePath); err != nil {
			return nil, fmt.Errorf("initialising store: %w", err)
		}
	}
	var opts []store.Option
	if appLogger != nil {
		opts = append(opts, store.WithLogger(appLogger))
	}
	return store.New(storePath, types.RealClock{}, opts...)
}

func rootCmd() *cobra.Command {
	var storePath string
	var logLevel string

	root := &cobra.Command{
		Use:   "wyrd",
		Short: "Wyrd — a flat-file graph-based personal productivity tool",
		Long: `Wyrd is a terminal-based personal productivity tool backed by a flat-file
property graph. Run without arguments to launch the TUI.`,
		PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
			// Resolve log level: flag > env var > default (info).
			level := logLevel
			if level == "" {
				level = os.Getenv("WYRD_LOG_LEVEL")
			}
			if level == "" {
				level = "info"
			}

			// Ensure ~/.wyrd/ exists.
			logPath := logFilePath()
			if err := os.MkdirAll(filepath.Dir(logPath), 0o755); err != nil {
				return fmt.Errorf("creating log directory: %w", err)
			}

			// Open the log file (append mode).
			f, err := os.OpenFile(logPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
			if err != nil {
				return fmt.Errorf("opening log file: %w", err)
			}

			appLogger = clog.New(f)
			appLogger.SetLevel(parseLogLevel(level))
			appLogger.SetReportTimestamp(true)

			return nil
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			s, err := openStore(storePath)
			if err != nil {
				return err
			}
			defer s.Close()
			var engineOpts []query.EngineOption
			if appLogger != nil {
				engineOpts = append(engineOpts, query.WithLogger(appLogger))
			}
			return tui.Run(tui.Config{
				Store:       s,
				StorePath:   storePath,
				Index:       s.Index(),
				QueryRunner: query.NewEngine(s.Index(), 0, engineOpts...),
				Clock:       types.RealClock{},
				Logger:      appLogger,
			})
		},
	}

	root.PersistentFlags().StringVar(&storePath, "store", defaultStorePath(), "path to the Wyrd store directory")
	root.PersistentFlags().StringVar(&logLevel, "log-level", "", "log level: debug, info, warn, error (default: info, env: WYRD_LOG_LEVEL)")

	root.AddCommand(initCmd(&storePath))
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

// initCmd implements `wyrd init`.
func initCmd(storePath *string) *cobra.Command {
	return &cobra.Command{
		Use:   "init",
		Short: "Initialise a new Wyrd store",
		Long: `Create the store directory structure, copy starter files,
run git init, and write .gitattributes for the merge driver.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := cli.Init(*storePath); err != nil {
				return err
			}
			fmt.Fprintf(os.Stdout, "Wyrd store initialised at %s\n", *storePath)
			return nil
		},
	}
}

// addCmd implements `wyrd add`.
func addCmd(storePath *string) *cobra.Command {
	var nodeType string
	var linkID string
	var title string

	cmd := &cobra.Command{
		Use:   "add <body>",
		Short: "Quick-add a task node",
		Long: `Create a new node from a body text argument.
Defaults to type 'task' with status 'inbox'.`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if title == "" {
				form := huh.NewForm(
					huh.NewGroup(
						huh.NewInput().
							Title("Title").
							Value(&title).
							Placeholder("Short title for the node (optional — press enter to skip)"),
					),
				).WithTheme(huh.ThemeFunc(huh.ThemeCharm)).WithShowHelp(true)
				if err := form.Run(); err != nil && !errors.Is(err, huh.ErrUserAborted) {
					return err
				}
			}

			s, err := openStore(*storePath)
			if err != nil {
				return err
			}
			id, err := cli.Add(s, cli.AddOptions{
				Body:     args[0],
				Title:    title,
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
	cmd.Flags().StringVar(&title, "title", "", "short title for the node")
	return cmd
}

// journalCmd implements `wyrd journal`.
func journalCmd(storePath *string) *cobra.Command {
	var linkID string

	cmd := &cobra.Command{
		Use:   "journal",
		Short: "Create a journal node",
		RunE: func(cmd *cobra.Command, args []string) error {
			title := time.Now().Format("2006-01-02")
			var body string

			form := huh.NewForm(
				huh.NewGroup(
					huh.NewInput().
						Title("Title").
						Value(&title),
					huh.NewText().
						Title("Body").
						Value(&body).
						Lines(12).
						Placeholder("Write your journal entry...").
						Validate(func(s string) error {
							if s == "" {
								return errors.New("body is required")
							}
							return nil
						}),
				),
			).WithTheme(huh.ThemeFunc(huh.ThemeCharm)).WithShowHelp(true)

			if err := form.Run(); err != nil {
				if errors.Is(err, huh.ErrUserAborted) {
					fmt.Fprintln(os.Stdout, "Cancelled.")
					return nil
				}
				return err
			}

			s, err := openStore(*storePath)
			if err != nil {
				return err
			}
			id, err := cli.Journal(s, cli.JournalOptions{
				Title:  title,
				Body:   body,
				LinkID: linkID,
			})
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
		Short: "Create a note node",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			title := args[0]
			var body string

			form := huh.NewForm(
				huh.NewGroup(
					huh.NewText().
						Title("Body").
						Value(&body).
						Lines(8).
						Placeholder("Write your note...").
						Validate(func(s string) error {
							if s == "" {
								return errors.New("body is required")
							}
							return nil
						}),
				),
			).WithTheme(huh.ThemeFunc(huh.ThemeCharm)).WithShowHelp(true)

			if err := form.Run(); err != nil {
				if errors.Is(err, huh.ErrUserAborted) {
					fmt.Fprintln(os.Stdout, "Cancelled.")
					return nil
				}
				return err
			}

			s, err := openStore(*storePath)
			if err != nil {
				return err
			}
			id, err := cli.Note(s, cli.NoteOptions{
				Title:  title,
				Body:   body,
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
			if err := cli.Spend(s, s.Index(), cli.SpendOptions{
				Category: args[0],
				Amount:   amount,
				Note:     args[2],
			}); err != nil {
				return err
			}
			fmt.Fprintln(os.Stdout, "Spend recorded.")
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
			s, err := openStore(*storePath)
			if err != nil {
				return err
			}
			var engineOpts []query.EngineOption
			if appLogger != nil {
				engineOpts = append(engineOpts, query.WithLogger(appLogger))
			}
			engine := query.NewEngine(s.Index(), 0, engineOpts...)
			return cli.RunQuery(engine, types.RealClock{}, cli.QueryOptions{QueryString: args[0]}, os.Stdout)
		},
	}
}

// viewCmd implements `wyrd view`.
func viewCmd(storePath *string) *cobra.Command {
	return &cobra.Command{
		Use:   "view <name>",
		Short: "Run a saved view and print results",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			s, err := openStore(*storePath)
			if err != nil {
				return err
			}
			var engineOpts []query.EngineOption
			if appLogger != nil {
				engineOpts = append(engineOpts, query.WithLogger(appLogger))
			}
			engine := query.NewEngine(s.Index(), 0, engineOpts...)
			return cli.RunView(s, engine, types.RealClock{}, args[0], os.Stdout)
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

// Ensure Store satisfies both StoreFS and PluginStore at compile time.
var _ types.StoreFS = (*store.Store)(nil)
var _ types.PluginStore = (*store.Store)(nil)
