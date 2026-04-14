package store

import (
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
		watcher.Close()
		return err
	}
	if err := watcher.Add(edgesDir); err != nil {
		watcher.Close()
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
		}
		// Node removal is not supported — nodes are never deleted.
	case "edges":
		if event.Op&(fsnotify.Create|fsnotify.Write) != 0 {
			edge, err := s.ReadEdge(id)
			if err != nil {
				s.logWarn("watcher: failed to read edge", "id", id, "err", err)
				return
			}
			s.index.upsertEdge(edge)
		} else if event.Op&fsnotify.Remove != 0 {
			s.index.removeEdge(id)
		}
	}
}
