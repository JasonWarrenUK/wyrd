// Package sync provides git-based synchronisation for the Wyrd store.
// It shells out to the system git binary rather than using a Go git library,
// keeping the dependency surface small and the behaviour predictable.
package sync

import (
	"bytes"
	"fmt"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	clog "github.com/charmbracelet/log"
)

// gitAttributesContent is written to .gitattributes in the store root.
// It instructs git to invoke the wyrd-merge-driver binary for all JSONC files.
const gitAttributesContent = "*.jsonc merge=wyrd-merge\n"

// mergeDriverConfig is the snippet appended to .git/config that registers
// the wyrd-merge custom driver.
const mergeDriverConfig = `[merge "wyrd-merge"]
	name = Wyrd JSONC three-way merge
	driver = wyrd merge-driver %O %A %B
`

// Init initialises storePath as a git repository and configures the custom
// JSONC merge driver. Safe to call on an already-initialised repository.
func Init(storePath string) error {
	// Initialise the repository (idempotent).
	if err := runGit(storePath, "init"); err != nil {
		return fmt.Errorf("git init failed: %w", err)
	}

	// Write .gitattributes so every JSONC file uses our merge driver.
	attrPath := filepath.Join(storePath, ".gitattributes")
	if err := writeFileIfChanged(attrPath, []byte(gitAttributesContent)); err != nil {
		return fmt.Errorf("failed to write .gitattributes: %w", err)
	}

	// Register the merge driver in .git/config if not already present.
	gitConfigPath := filepath.Join(storePath, ".git", "config")
	if err := appendMergeDriverConfig(gitConfigPath); err != nil {
		return fmt.Errorf("failed to configure merge driver: %w", err)
	}

	return nil
}

// Sync performs the full synchronisation cycle for storePath:
//  1. Stage all changes.
//  2. Commit with an auto-generated message (skipped if nothing to commit).
//  3. Pull with rebase to integrate remote changes.
//  4. Push the result to the remote.
//
// Network errors are surfaced with actionable messages. The caller is
// responsible for configuring the remote before calling Sync.
func Sync(storePath string, logger *clog.Logger) error {
	// Stage all changes (new, modified, and deleted files).
	logDebug(logger, "staging all changes", "path", storePath)
	if err := runGit(storePath, "add", "--all"); err != nil {
		return fmt.Errorf("failed to stage changes: %w", err)
	}

	// Build a descriptive commit message from the index.
	message, err := buildCommitMessage(storePath)
	if err != nil {
		return fmt.Errorf("failed to build commit message: %w", err)
	}

	// Only commit if there is something staged.
	if message != "" {
		logDebug(logger, "committing", "message", message)
		if err := runGit(storePath, "commit", "-m", message); err != nil {
			return fmt.Errorf("failed to commit: %w", err)
		}
	}

	// Pull with rebase so our commits stay on top of remote changes.
	// Capture stderr to detect network errors and produce actionable output.
	logDebug(logger, "pulling with rebase")
	if err := runGitWithNetworkCheck(storePath, "pull", "--rebase"); err != nil {
		logError(logger, "pull failed", "err", err)
		return err
	}

	// Push to the configured remote.
	logDebug(logger, "pushing to remote")
	if err := runGitWithNetworkCheck(storePath, "push"); err != nil {
		logError(logger, "push failed", "err", err)
		return err
	}

	logInfo(logger, "sync complete", "path", storePath)
	return nil
}

// Status returns the list of files with uncommitted changes in storePath.
// Used by the TUI status bar to indicate pending local modifications.
func Status(storePath string) ([]string, error) {
	cmd := exec.Command("git", "-C", storePath, "status", "--porcelain")
	out, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("git status failed: %w", err)
	}

	if len(bytes.TrimSpace(out)) == 0 {
		return []string{}, nil
	}

	var changed []string
	for _, line := range strings.Split(string(out), "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		// Porcelain format: XY PATH or XY ORIG -> PATH
		parts := strings.SplitN(line, " ", 2)
		if len(parts) == 2 {
			// Handle renames: "ORIG -> PATH"
			filePart := strings.TrimSpace(parts[1])
			if idx := strings.LastIndex(filePart, " -> "); idx >= 0 {
				filePart = filePart[idx+4:]
			}
			changed = append(changed, filePart)
		}
	}

	return changed, nil
}

// buildCommitMessage inspects the git index and constructs a message of the
// form "wyrd: created N nodes, updated M nodes, deleted K edges" etc.
// Returns an empty string when nothing is staged.
func buildCommitMessage(storePath string) (string, error) {
	cmd := exec.Command("git", "-C", storePath, "diff", "--cached", "--name-status")
	out, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("git diff failed: %w", err)
	}

	if len(bytes.TrimSpace(out)) == 0 {
		return "", nil
	}

	counters := map[string]int{
		"node created": 0,
		"node updated": 0,
		"node deleted": 0,
		"edge created": 0,
		"edge updated": 0,
		"edge deleted": 0,
		"file created": 0,
		"file updated": 0,
		"file deleted": 0,
	}

	nodePattern := regexp.MustCompile(`nodes/[^/]+\.jsonc$`)
	edgePattern := regexp.MustCompile(`edges/[^/]+\.jsonc$`)

	for _, line := range strings.Split(string(out), "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		// Name-status format: STATUS\tPATH (or STATUS\tOLD\tNEW for renames)
		fields := strings.Fields(line)
		if len(fields) < 2 {
			continue
		}

		status := fields[0]
		path := fields[len(fields)-1]

		var kind string
		switch {
		case nodePattern.MatchString(path):
			kind = "node"
		case edgePattern.MatchString(path):
			kind = "edge"
		default:
			kind = "file"
		}

		switch {
		case status == "A":
			counters[kind+" created"]++
		case status == "M":
			counters[kind+" updated"]++
		case status == "D":
			counters[kind+" deleted"]++
		case strings.HasPrefix(status, "R"):
			// Rename counts as an update.
			counters[kind+" updated"]++
		}
	}

	var parts []string
	for _, key := range []string{
		"node created", "node updated", "node deleted",
		"edge created", "edge updated", "edge deleted",
		"file created", "file updated", "file deleted",
	} {
		n := counters[key]
		if n == 0 {
			continue
		}
		keyParts := strings.Split(key, " ")
		noun := keyParts[0]
		verb := keyParts[1]
		parts = append(parts, fmt.Sprintf("%s %s %s", verb, strconv.Itoa(n), pluralise(noun, n)))
	}

	if len(parts) == 0 {
		return "", nil
	}

	return "wyrd: " + strings.Join(parts, ", "), nil
}

// pluralise returns the word with an appended "s" when count != 1.
func pluralise(word string, count int) string {
	if count == 1 {
		return word
	}
	return word + "s"
}

// runGit executes a git command in the given directory, discarding stdout.
// Stderr is captured and returned as part of any error message.
func runGit(dir string, args ...string) error {
	cmd := exec.Command("git", append([]string{"-C", dir}, args...)...)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		msg := strings.TrimSpace(stderr.String())
		if msg != "" {
			return fmt.Errorf("%w: %s", err, msg)
		}
		return err
	}
	return nil
}

// runGitWithNetworkCheck runs a git command that touches the network and
// produces a helpful error message when connectivity issues are detected.
func runGitWithNetworkCheck(dir string, args ...string) error {
	cmd := exec.Command("git", append([]string{"-C", dir}, args...)...)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	err := cmd.Run()
	if err == nil {
		return nil
	}

	stderrStr := stderr.String()

	// Classify common network-related failure patterns.
	switch {
	case containsAny(stderrStr,
		"Could not resolve host",
		"Connection refused",
		"Connection timed out",
		"network is unreachable",
		"Network is unreachable",
	):
		return fmt.Errorf("network unreachable — check your internet connection and try again: %s", strings.TrimSpace(stderrStr))

	case containsAny(stderrStr,
		"Permission denied",
		"Authentication failed",
		"could not read Username",
		"Repository not found",
	):
		return fmt.Errorf("authentication failed — check your git credentials or SSH key: %s", strings.TrimSpace(stderrStr))

	case containsAny(stderrStr,
		"CONFLICT",
		"conflict",
	):
		return fmt.Errorf("merge conflict detected — resolve conflicts in %s and run sync again: %s", dir, strings.TrimSpace(stderrStr))

	case containsAny(stderrStr, "has no upstream"):
		return fmt.Errorf("no upstream branch configured — run: git -C %q push -u origin <branch>", dir)

	default:
		msg := strings.TrimSpace(stderrStr)
		if msg != "" {
			return fmt.Errorf("git %s failed: %s", args[0], msg)
		}
		return fmt.Errorf("git %s failed: %w", args[0], err)
	}
}

// containsAny reports whether s contains any of the given substrings.
func containsAny(s string, substrings ...string) bool {
	for _, sub := range substrings {
		if strings.Contains(s, sub) {
			return true
		}
	}
	return false
}

// writeFileIfChanged writes content to path only when the file does not already
// contain identical content, avoiding unnecessary writes and git dirty state.
func writeFileIfChanged(path string, content []byte) error {
	existing, err := readFileSafe(path)
	if err == nil && bytes.Equal(existing, content) {
		return nil // Already up to date.
	}

	return writeFile(path, content)
}

// appendMergeDriverConfig adds the wyrd-merge stanza to .git/config when it
// is not already present.
func appendMergeDriverConfig(configPath string) error {
	existing, _ := readFileSafe(configPath)
	if strings.Contains(string(existing), `[merge "wyrd-merge"]`) {
		return nil // Already configured.
	}

	f, err := openFileAppend(configPath)
	if err != nil {
		return err
	}
	defer f.Close()

	_, err = f.WriteString(mergeDriverConfig)
	return err
}

// logDebug emits a debug-level log entry when a logger is configured.
func logDebug(logger *clog.Logger, msg string, keyvals ...interface{}) {
	if logger != nil {
		logger.Debug(msg, keyvals...)
	}
}

// logInfo emits an info-level log entry when a logger is configured.
func logInfo(logger *clog.Logger, msg string, keyvals ...interface{}) {
	if logger != nil {
		logger.Info(msg, keyvals...)
	}
}

// logError emits an error-level log entry when a logger is configured.
func logError(logger *clog.Logger, msg string, keyvals ...interface{}) {
	if logger != nil {
		logger.Error(msg, keyvals...)
	}
}
