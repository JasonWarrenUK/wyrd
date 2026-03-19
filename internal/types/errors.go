package types

import "fmt"

// NotFoundError is returned when a requested entity does not exist.
type NotFoundError struct {
	// Kind is the entity type (e.g., "node", "edge", "template").
	Kind string
	// ID is the identifier that was not found.
	ID string
}

func (e *NotFoundError) Error() string {
	return fmt.Sprintf("%s %q not found", e.Kind, e.ID)
}

// ValidationError is returned when input fails a structural check.
type ValidationError struct {
	// Field is the name of the invalid field.
	Field string
	// Message describes the validation failure.
	Message string
}

func (e *ValidationError) Error() string {
	if e.Field != "" {
		return fmt.Sprintf("validation error on %q: %s", e.Field, e.Message)
	}
	return fmt.Sprintf("validation error: %s", e.Message)
}

// ParseError is returned when JSONC or query parsing fails.
type ParseError struct {
	// Source identifies where the parse error originated (file path or query string).
	Source string
	// Message describes the parse failure.
	Message string
}

func (e *ParseError) Error() string {
	if e.Source != "" {
		return fmt.Sprintf("parse error in %s: %s", e.Source, e.Message)
	}
	return fmt.Sprintf("parse error: %s", e.Message)
}

// ConflictError is returned when an operation would violate data integrity.
type ConflictError struct {
	// Message describes the conflict.
	Message string
}

func (e *ConflictError) Error() string {
	return fmt.Sprintf("conflict: %s", e.Message)
}

// PluginError is returned when a plugin operation fails.
type PluginError struct {
	// Plugin is the name of the failing plugin.
	Plugin string
	// Message describes the failure.
	Message string
}

func (e *PluginError) Error() string {
	return fmt.Sprintf("plugin %q error: %s", e.Plugin, e.Message)
}

// QueryError is returned when a query is invalid or fails to execute.
type QueryError struct {
	// Query is the offending query string.
	Query string
	// Message describes the failure.
	Message string
}

func (e *QueryError) Error() string {
	return fmt.Sprintf("query error: %s", e.Message)
}
