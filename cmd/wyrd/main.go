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
	"github.com/jasonwarrenuk/wyrd/internal/stage"
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

			// Build the merged kind registry: baked-in defaults shadowed by the
			// user's kinds.jsonc. A parse failure in the baked-in defaults is a
			// build bug, not a user error — treat it as fatal.
			kindDefaults, err := stage.DefaultKinds()
			if err != nil {
				return fmt.Errorf("loading built-in kind defaults: %w", err)
			}
			userKindReg, err := s.ReadKinds()
			if err != nil {
				// Non-fatal: log and continue without user kinds.
				if appLogger != nil {
					appLogger.Warn("could not load kinds.jsonc; using defaults only", "err", err)
				}
				userKindReg = types.NewKindRegistry(nil)
			}
			kinds := stage.MergeKinds(kindDefaults, userKindReg.All())

			// Build the merged stage-group registry: baked-in defaults (SL.13
			// will add user groups from stages.jsonc as the second argument).
			groupDefaults, err := stage.DefaultStageGroups()
			if err != nil {
				return fmt.Errorf("loading built-in stage-group defaults: %w", err)
			}
			stageGroups := stage.MergeStageGroups(groupDefaults, nil)

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
				Kinds:       kinds,
				StageGroups: stageGroups,
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
	root.AddCommand(budgetCmd(&storePath))
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

// budgetCmd implements `wyrd budget`.
func budgetCmd(storePath *string) *cobra.Command {
	budget := &cobra.Command{
		Use:   "budget",
		Short: "Manage budget envelopes",
	}

	budget.AddCommand(budgetCreateCmd(storePath))
	return budget
}

// budgetCreateCmd implements `wyrd budget create`.
func budgetCreateCmd(storePath *string) *cobra.Command {
	var category string
	var allocated float64
	var period string
	var warnAt float64
	var linkID string

	cmd := &cobra.Command{
		Use:   "create [category]",
		Short: "Create a new budget envelope",
		Long: `Create a new budget node with a category, allocation, period, and warning threshold.
The category may be supplied as a positional argument or via --category.
When required values are omitted, an interactive form prompts for them.`,
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			// Positional argument overrides --category when both are absent as flag.
			if len(args) == 1 && category == "" {
				category = args[0]
			}

			// Interactive fallback: if category or allocated are not provided
			// via flags, prompt for them.
			needsInteractive := category == "" || allocated == 0

			if needsInteractive {
				catValue := category
				allocStr := ""
				if allocated > 0 {
					allocStr = strconv.FormatFloat(allocated, 'f', 2, 64)
				}
				periodValue := period
				if periodValue == "" {
					periodValue = "month"
				}
				warnAtStr := "0.8"
				if warnAt > 0 {
					warnAtStr = strconv.FormatFloat(warnAt, 'f', 2, 64)
				}

				// Build form fields; skip category input when already supplied.
				var fields []huh.Field
				if catValue == "" {
					fields = append(fields, huh.NewInput().
						Title("Category").
						Value(&catValue).
						Placeholder("e.g. groceries, transport, entertainment").
						Validate(func(s string) error {
							if s == "" {
								return errors.New("category is required")
							}
							return nil
						}),
					)
				}
				fields = append(fields,
					huh.NewInput().
						Title("Allocated amount").
						Value(&allocStr).
						Placeholder("0.00").
						Validate(func(s string) error {
							if s == "" {
								return errors.New("allocated amount is required")
							}
							v, err := strconv.ParseFloat(s, 64)
							if err != nil {
								return errors.New("must be a number")
							}
							if v <= 0 {
								return errors.New("must be greater than zero")
							}
							return nil
						}),
					huh.NewSelect[string]().
						Title("Period").
						Options(
							huh.NewOption("Weekly", "week"),
							huh.NewOption("Monthly", "month"),
							huh.NewOption("Quarterly", "quarter"),
							huh.NewOption("Yearly", "year"),
						).
						Value(&periodValue),
					huh.NewInput().
						Title("Warn at (fraction 0–1)").
						Value(&warnAtStr).
						Placeholder("0.8"),
				)

				form := huh.NewForm(
					huh.NewGroup(fields...),
				).WithTheme(huh.ThemeFunc(huh.ThemeCharm)).WithShowHelp(true)

				if err := form.Run(); err != nil {
					if errors.Is(err, huh.ErrUserAborted) {
						fmt.Fprintln(os.Stdout, "Cancelled.")
						return nil
					}
					return err
				}

				category = catValue
				if v, err := strconv.ParseFloat(allocStr, 64); err == nil {
					allocated = v
				}
				period = periodValue
				if v, err := strconv.ParseFloat(warnAtStr, 64); err == nil {
					warnAt = v
				}
			}

			s, err := openStore(*storePath)
			if err != nil {
				return err
			}
			id, err := cli.BudgetCreate(s, cli.BudgetCreateOptions{
				Category:  category,
				Allocated: allocated,
				Period:    period,
				WarnAt:    warnAt,
				LinkID:    linkID,
			})
			if err != nil {
				return err
			}
			fmt.Fprintf(os.Stdout, "Created budget node %s\n", id)
			return nil
		},
	}

	cmd.Flags().StringVar(&category, "category", "", "budget category name")
	cmd.Flags().Float64Var(&allocated, "allocated", 0, "amount allocated for this period")
	cmd.Flags().StringVar(&period, "period", "", "budget period (week, month, quarter, year)")
	cmd.Flags().Float64Var(&warnAt, "warn-at", 0, "fraction of allocation that triggers a warning (0–1)")
	cmd.Flags().StringVar(&linkID, "link", "", "create a 'related' edge to this node ID")
	return cmd
}

// spendCmd implements `wyrd spend`.
func spendCmd(storePath *string) *cobra.Command {
	var dateFlag string
	cmd := &cobra.Command{
		Use:   "spend <category> <amount> <note>",
		Short: "Log a spend entry",
		Long: `Record a spending event under a budget category.
Amount must be a positive decimal number.
Use --date YYYY-MM-DD to back-date or future-date the entry; defaults to today.`,
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
				Date:     dateFlag,
			}); err != nil {
				return err
			}
			fmt.Fprintln(os.Stdout, "Spend recorded.")
			return nil
		},
	}
	cmd.Flags().StringVar(&dateFlag, "date", "", "Entry date (YYYY-MM-DD); defaults to today")
	return cmd
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
			return cli.Sync(s, cli.SyncOptions{Logger: appLogger}, os.Stdout)
		},
	}
}

// queryCmd implements `wyrd query`.
func queryCmd(storePath *string) *cobra.Command {
	return &cobra.Command{
		Use:   "query <cypher>",
		Short: "Run a Cypher query and print results",
		Example: `  wyrd query "MATCH (n) WHERE 'task' IN n.types AND n.status IN ['inbox', 'active'] RETURN n"
  wyrd query "MATCH (n:task) RETURN n.title, n.date.due ORDER BY n.date.due"`,
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
	var dryRun bool

	cmd := &cobra.Command{
		Use:   "compact",
		Short: "Move archived nodes and orphan edges to archive/",
		Long:  "Compact scans for nodes with status \"archived\" and moves them (and any edges that touch them) to archive/nodes/ and archive/edges/. Use --dry-run to preview without making changes.",
		RunE: func(cmd *cobra.Command, args []string) error {
			s, err := openStore(*storePath)
			if err != nil {
				return err
			}
			defer s.Close()

			return cli.Compact(s, s.Index(), dryRun, os.Stdout)
		},
	}

	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "Preview what would be moved without making changes")
	return cmd
}

// Ensure Store satisfies both StoreFS and PluginStore at compile time.
var _ types.StoreFS = (*store.Store)(nil)
var _ types.PluginStore = (*store.Store)(nil)
