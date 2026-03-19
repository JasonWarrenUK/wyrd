package cli

import (
	"fmt"
	"io"
	"os"
	"strings"
	"text/tabwriter"

	"github.com/charmbracelet/x/term"
	"github.com/jasonwarrenuk/wyrd/internal/types"
)

// QueryOptions holds the parameters for the query command.
type QueryOptions struct {
	// QueryString is the Cypher subset query to execute.
	QueryString string
}

// RunQuery executes a query via the provided QueryRunner and prints the results.
// When stdout is a TTY the output is rendered as an aligned table; otherwise
// it falls back to tab-separated values for piping.
func RunQuery(runner types.QueryRunner, clock types.Clock, opts QueryOptions, out io.Writer) error {
	if opts.QueryString == "" {
		return &types.ValidationError{Field: "query", Message: "query string must not be empty"}
	}

	result, err := runner.Run(opts.QueryString, clock)
	if err != nil {
		return fmt.Errorf("query failed: %w", err)
	}

	if len(result.Rows) == 0 {
		fmt.Fprintln(out, "(no results)")
		return nil
	}

	isTTY := isTerminal(out)
	printTable(out, result, isTTY)
	return nil
}

// RunView executes a saved view by name and prints the results.
func RunView(store types.StoreFS, runner types.QueryRunner, clock types.Clock, viewName string, out io.Writer) error {
	if viewName == "" {
		return &types.ValidationError{Field: "name", Message: "view name must not be empty"}
	}

	view, err := store.ReadView(viewName)
	if err != nil {
		return fmt.Errorf("view %q not found — run 'wyrd plugin list' to see available views", viewName)
	}

	result, err := runner.Run(view.Query, clock)
	if err != nil {
		return fmt.Errorf("view %q query failed: %w", viewName, err)
	}

	if len(result.Rows) == 0 {
		fmt.Fprintln(out, "(no results)")
		return nil
	}

	isTTY := isTerminal(out)
	printTable(out, result, isTTY)
	return nil
}

// printTable renders query results as an aligned table (TTY) or TSV (non-TTY).
func printTable(out io.Writer, result *types.QueryResult, isTTY bool) {
	if isTTY {
		w := tabwriter.NewWriter(out, 0, 0, 2, ' ', 0)
		fmt.Fprintln(w, strings.Join(result.Columns, "\t"))
		fmt.Fprintln(w, strings.Repeat("-\t", len(result.Columns)))
		for _, row := range result.Rows {
			vals := make([]string, len(result.Columns))
			for i, col := range result.Columns {
				if v, ok := row[col]; ok {
					vals[i] = fmt.Sprintf("%v", v)
				} else {
					vals[i] = ""
				}
			}
			fmt.Fprintln(w, strings.Join(vals, "\t"))
		}
		w.Flush()
	} else {
		// Tab-separated output for non-TTY consumers.
		fmt.Fprintln(out, strings.Join(result.Columns, "\t"))
		for _, row := range result.Rows {
			vals := make([]string, len(result.Columns))
			for i, col := range result.Columns {
				if v, ok := row[col]; ok {
					vals[i] = fmt.Sprintf("%v", v)
				} else {
					vals[i] = ""
				}
			}
			fmt.Fprintln(out, strings.Join(vals, "\t"))
		}
	}
}

// isTerminal reports whether w is a terminal file descriptor.
func isTerminal(w io.Writer) bool {
	if f, ok := w.(*os.File); ok {
		return term.IsTerminal(f.Fd())
	}
	return false
}
