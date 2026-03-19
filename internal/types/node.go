// Package types defines the core data structures shared across all Wyrd packages.
package types

import "time"

// Node represents a vertex in the Wyrd property graph.
// Nodes are stored as individual JSONC files under /store/nodes/{uuid}.jsonc.
// They are never deleted from disk — archiving sets status to "archived".
type Node struct {
	// ID is a UUID v4, auto-generated and immutable.
	ID string `json:"id"`

	// Body is the primary markdown content. Required.
	Body string `json:"body"`

	// Types is the list of template types applied to this node.
	// Minimum one type. Determines which conditional fields are active.
	Types []string `json:"types"`

	// Created is the creation timestamp, auto-generated and immutable.
	Created time.Time `json:"created"`

	// Modified is updated on any field change.
	Modified time.Time `json:"modified"`

	// Source is populated on nodes created by sync plugins.
	Source *Source `json:"source,omitempty"`

	// Properties holds all template-contributed and user-defined fields.
	// Stored as a flexible map to support arbitrary template fields.
	Properties map[string]interface{} `json:"-"`
}

// Source describes where a synced node originated.
type Source struct {
	// Type identifies the plugin that created this node (e.g., "github").
	Type string `json:"type"`

	// Repo is the external repository or collection identifier.
	Repo string `json:"repo,omitempty"`

	// ID is the external identifier within the source system.
	ID string `json:"id"`

	// URL is the canonical URL for the source entity.
	URL string `json:"url,omitempty"`

	// LastSynced is the most recent sync timestamp.
	LastSynced time.Time `json:"last_synced"`
}

// SpendEntry records a single spend event in a budget envelope's spend_log.
type SpendEntry struct {
	// Date is the ISO 8601 date of the spend.
	Date string `json:"date"`

	// Amount is the monetary value spent.
	Amount float64 `json:"amount"`

	// Note is a human-readable description of the spend.
	Note string `json:"note,omitempty"`
}
