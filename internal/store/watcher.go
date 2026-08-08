package store

import (
	"os"
	"path/filepath"

	"github.com/fsnotify/fsnotify"
)

// startWatcher starts a filesystem watcher on the nodes and edges directories.
// On file change, the index is updated incrementally.
func (s *Store) startWatcher() error {
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		return err
	}

	nodesDir := filepath.Join(s.path, "nodes")
	edgesDir := filepath.Join(s.path, "edges")

	if err := watcher.Add(nodesDir); err != nil {
		_ = watcher.Close()
		return err
	}
	if err := watcher.Add(edgesDir); err != nil {
		_ = watcher.Close()
		return err
	}

	s.watcher = watcher

	go func() {
		for {
			select {
			case event, ok := <-watcher.Events:
				if !ok {
					return
				}
				s.handleWatchEvent(event)
			case err, ok := <-watcher.Errors:
				if !ok {
					return
				}
				s.logWarn("watcher error", "err", err)
			}
		}
	}()

	return nil
}

// handleWatchEvent processes a single filesystem event.
func (s *Store) handleWatchEvent(event fsnotify.Event) {
	path := event.Name
	ext := filepath.Ext(path)
	if ext != ".jsonc" {
		return
	}

	dir := filepath.Base(filepath.Dir(path))
	base := filepath.Base(path)
	id := base[:len(base)-len(".jsonc")]

	switch dir {
	case "nodes":
		if event.Op&(fsnotify.Create|fsnotify.Write) != 0 {
			node, err := s.ReadNode(id)
			if err != nil {
				s.logWarn("watcher: failed to read node", "id", id, "err", err)
				return
			}
			s.index.upsertNode(node)
		} else if event.Op&(fsnotify.Remove|fsnotify.Rename) != 0 {
			// Nodes are never deleted from the user-facing model, but the
			// file backing one can still disappear from nodes/: cli.Compact
			// renames archived node files out of this directory, and so does
			// a `git pull` that picks up someone else's compaction on
			// another machine. Evicting here keeps a running TUI's index in
			// sync with the filesystem without requiring a restart.
			//
			// Deliberately not matching fsnotify.Chmod: a permission change
			// carries no removal semantics and would evict a live node.
			//
			// A stat guard is required before evicting: on kqueue (macOS,
			// BSD) an atomic rename-over-an-existing-file — exactly what
			// jsonc.WriteFile's temp-file-then-rename does on every ordinary
			// write — is delivered as Remove immediately followed by Create
			// for the *same* destination path, not as a single Write. Acting
			// on the Remove alone would evict a node on every write to it.
			// If the path still exists, this is that overwrite case: treat
			// it as an upsert instead of a removal.
			if _, statErr := os.Stat(path); statErr == nil {
				if node, err := s.ReadNode(id); err == nil {
					s.index.upsertNode(node)
				}
				return
			}
			s.index.removeNode(id)
		}
	case "edges":
		if event.Op&(fsnotify.Create|fsnotify.Write) != 0 {
			edge, err := s.ReadEdge(id)
			if err != nil {
				s.logWarn("watcher: failed to read edge", "id", id, "err", err)
				return
			}
			s.index.upsertEdge(edge)
		} else if event.Op&(fsnotify.Remove|fsnotify.Rename) != 0 {
			// Same rename-over-existing-file ambiguity as the nodes case
			// above: a stat guard tells a real removal apart from an
			// in-place overwrite delivered as Remove+Create.
			if _, statErr := os.Stat(path); statErr == nil {
				if edge, err := s.ReadEdge(id); err == nil {
					s.index.upsertEdge(edge)
				}
				return
			}
			s.index.removeEdge(id)
		}
	}
}
