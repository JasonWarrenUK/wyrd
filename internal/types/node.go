// Package types defines the core data structures shared across all Wyrd packages.
package types

import "time"

// DateFields groups all date-related timestamps for a node into a single
// nested "date" object on disk.
type DateFields struct {
	// Created is the node creation timestamp.
	Created time.Time `json:"created"`

	// Modified is the last-modified timestamp.
	Modified time.Time `json:"modified"`

	// Due is the date by which a task must be completed.
	Due *time.Time `json:"due,omitempty"`

	// About is the date a journal entry or note is about (not when it was
	// written). Use this for daily journals and event-related notes.
	About *time.Time `json:"about,omitempty"`

	// Schedule is the date/time at which a task is scheduled to start.
	Schedule *time.Time `json:"schedule,omitempty"`

	// Start is the actual start date of work on a task.
	Start *time.Time `json:"start,omitempty"`

	// SnoozeUntil suppresses a task from the dashboard until this date.
	SnoozeUntil *time.Time `json:"snooze_until,omitempty"`
}

// Node represents a vertex in the Wyrd property graph.
// Nodes are stored as individual JSONC files under /store/nodes/{uuid}.jsonc.
// They are never deleted from disk — archiving sets status to "archived".
type Node struct {
	// ID is a UUID v4, auto-generated and immutable.
	ID string `json:"id"`

	// Title is an optional short display title. When present it is preferred
	// over the first line of Body for list views and headings.
	Title string `json:"title,omitempty"`

	// Body is the primary markdown content. Required.
	Body string `json:"body"`

	// Types is the list of template types applied to this node.
	// Minimum one type. Determines which conditional fields are active.
	Types []string `json:"types"`

	// Kind names the kind registry entry this node belongs to (SL.1).
	// Empty on nodes created before the status lattice; readers must treat
	// an empty Kind as "no kind assigned" rather than a valid registry key.
	Kind string `json:"kind,omitempty"`

	// Stage is the node's current stage within its kind's stage group (SL.1).
	// Valid values are defined by the stage group model (SL.2); until a kind
	// is assigned, Stage is empty.
	Stage string `json:"stage,omitempty"`

	// Date holds the full set of date fields for this node, including
	// creation and last-modified timestamps. Written on disk as a nested
	// "date" object. The json:"-" tag prevents double-serialisation;
	// WriteNode handles it.
	Date DateFields `json:"-"`

	// Source is populated on nodes created by sync plugins.
	Source *Source `json:"source,omitempty"`

	// Properties holds all template-contributed and user-defined fields.
	// Stored as a flexible map to support arbitrary template fields.
	Properties map[string]interface{} `json:"-"`
}

// Clone returns a copy of the node safe to mutate independently of the
// original: the Types slice, Properties map, Date pointer fields, and Source
// are all freshly allocated. Values held inside Properties are not deep-copied;
// callers that mutate nested property values (rather than replacing keys)
// must copy those themselves.
func (n *Node) Clone() *Node {
	if n == nil {
		return nil
	}
	c := *n

	if n.Types != nil {
		c.Types = make([]string, len(n.Types))
		copy(c.Types, n.Types)
	}

	if n.Properties != nil {
		c.Properties = make(map[string]interface{}, len(n.Properties))
		for k, v := range n.Properties {
			c.Properties[k] = v
		}
	}

	copyTime := func(t *time.Time) *time.Time {
		if t == nil {
			return nil
		}
		v := *t
		return &v
	}
	c.Date.Due = copyTime(n.Date.Due)
	c.Date.About = copyTime(n.Date.About)
	c.Date.Schedule = copyTime(n.Date.Schedule)
	c.Date.Start = copyTime(n.Date.Start)
	c.Date.SnoozeUntil = copyTime(n.Date.SnoozeUntil)

	if n.Source != nil {
		src := *n.Source
		c.Source = &src
	}

	return &c
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
