package store

import (
	"sync"

	"github.com/jasonwarrenuk/wyrd/internal/types"
)

// memIndex is an in-memory graph index implementing types.GraphIndex.
// It is safe for concurrent reads and writes.
type memIndex struct {
	mu    sync.RWMutex
	nodes map[string]*types.Node
	edges map[string]*types.Edge
	// from maps nodeID -> list of edge IDs where edge.From == nodeID
	from map[string][]string
	// to maps nodeID -> list of edge IDs where edge.To == nodeID
	to map[string][]string
}

func newMemIndex() *memIndex {
	return &memIndex{
		nodes: make(map[string]*types.Node),
		edges: make(map[string]*types.Edge),
		from:  make(map[string][]string),
		to:    make(map[string][]string),
	}
}

// GetNode returns a node by UUID. Returns NotFoundError if absent.
func (idx *memIndex) GetNode(id string) (*types.Node, error) {
	idx.mu.RLock()
	defer idx.mu.RUnlock()
	n, ok := idx.nodes[id]
	if !ok {
		return nil, &types.NotFoundError{Kind: "node", ID: id}
	}
	return n, nil
}

// GetEdge returns an edge by UUID. Returns NotFoundError if absent.
func (idx *memIndex) GetEdge(id string) (*types.Edge, error) {
	idx.mu.RLock()
	defer idx.mu.RUnlock()
	e, ok := idx.edges[id]
	if !ok {
		return nil, &types.NotFoundError{Kind: "edge", ID: id}
	}
	return e, nil
}

// AllNodes returns every node in the index.
func (idx *memIndex) AllNodes() []*types.Node {
	idx.mu.RLock()
	defer idx.mu.RUnlock()
	out := make([]*types.Node, 0, len(idx.nodes))
	for _, n := range idx.nodes {
		out = append(out, n)
	}
	return out
}

// AllEdges returns every edge in the index.
func (idx *memIndex) AllEdges() []*types.Edge {
	idx.mu.RLock()
	defer idx.mu.RUnlock()
	out := make([]*types.Edge, 0, len(idx.edges))
	for _, e := range idx.edges {
		out = append(out, e)
	}
	return out
}

// EdgesFrom returns all edges whose From field equals nodeID.
func (idx *memIndex) EdgesFrom(nodeID string) []*types.Edge {
	idx.mu.RLock()
	defer idx.mu.RUnlock()
	ids := idx.from[nodeID]
	out := make([]*types.Edge, 0, len(ids))
	for _, id := range ids {
		if e, ok := idx.edges[id]; ok {
			out = append(out, e)
		}
	}
	return out
}

// EdgesTo returns all edges whose To field equals nodeID.
func (idx *memIndex) EdgesTo(nodeID string) []*types.Edge {
	idx.mu.RLock()
	defer idx.mu.RUnlock()
	ids := idx.to[nodeID]
	out := make([]*types.Edge, 0, len(ids))
	for _, id := range ids {
		if e, ok := idx.edges[id]; ok {
			out = append(out, e)
		}
	}
	return out
}

// NodesByType returns all nodes that include typeName in their Types slice.
func (idx *memIndex) NodesByType(typeName string) []*types.Node {
	idx.mu.RLock()
	defer idx.mu.RUnlock()
	var out []*types.Node
	for _, n := range idx.nodes {
		for _, t := range n.Types {
			if t == typeName {
				out = append(out, n)
				break
			}
		}
	}
	return out
}

// upsertNode adds or updates a node in the index.
func (idx *memIndex) upsertNode(n *types.Node) {
	idx.mu.Lock()
	defer idx.mu.Unlock()
	idx.nodes[n.ID] = n
}

// upsertEdge adds or updates an edge in the index.
func (idx *memIndex) upsertEdge(e *types.Edge) {
	idx.mu.Lock()
	defer idx.mu.Unlock()

	// Remove old edge references if updating.
	if old, ok := idx.edges[e.ID]; ok {
		idx.removeEdgeRefs(old)
	}

	idx.edges[e.ID] = e
	idx.from[e.From] = append(idx.from[e.From], e.ID)
	idx.to[e.To] = append(idx.to[e.To], e.ID)
}

// removeEdge removes an edge from the index by ID.
func (idx *memIndex) removeEdge(id string) {
	idx.mu.Lock()
	defer idx.mu.Unlock()
	if e, ok := idx.edges[id]; ok {
		idx.removeEdgeRefs(e)
		delete(idx.edges, id)
	}
}

// removeEdgeRefs removes an edge from the from/to maps. Must be called with lock held.
func (idx *memIndex) removeEdgeRefs(e *types.Edge) {
	idx.from[e.From] = removeString(idx.from[e.From], e.ID)
	idx.to[e.To] = removeString(idx.to[e.To], e.ID)
}

func removeString(slice []string, s string) []string {
	out := slice[:0]
	for _, v := range slice {
		if v != s {
			out = append(out, v)
		}
	}
	return out
}
