package types

// FieldType enumerates the supported template field types.
type FieldType string

const (
	FieldTypeString      FieldType = "string"
	FieldTypeNumber      FieldType = "number"
	FieldTypeDate        FieldType = "date"
	FieldTypeDatetime    FieldType = "datetime"
	FieldTypeBoolean     FieldType = "boolean"
	FieldTypeEnum        FieldType = "enum"
	FieldTypeStringArray FieldType = "string_array"
	FieldTypeObject      FieldType = "object"
)

// FieldDef describes a single field contributed by a template.
type FieldDef struct {
	// Type is the field's data type.
	Type FieldType `json:"type"`

	// Values lists valid options for enum fields.
	Values []string `json:"values,omitempty"`

	// Default is the value applied when a node gains this type.
	// nil means no default (use zero value for the type).
	Default interface{} `json:"default,omitempty"`

	// Optional marks fields that may be absent on a node.
	Optional bool `json:"optional,omitempty"`

	// Required marks fields that must be present on a node.
	Required bool `json:"required,omitempty"`

	// Description is a human-readable explanation of the field's purpose.
	Description string `json:"description,omitempty"`
}

// Template defines the fields contributed when a node carries a given type.
// Stored at /store/templates/{type_name}.jsonc.
type Template struct {
	// TypeName is the identifier used in node.Types (e.g., "task", "journal").
	TypeName string `json:"type_name"`

	// Fields maps field names to their definitions.
	Fields map[string]FieldDef `json:"fields"`
}

// SavedView is a named Cypher query with optional presentation metadata.
// Stored at /store/views/{name}.jsonc.
type SavedView struct {
	// Name is the display name shown in the sidebar and command palette.
	Name string `json:"name"`

	// Pinned controls whether the view appears in the TUI sidebar.
	Pinned bool `json:"pinned,omitempty"`

	// Query is the Cypher query string for single-query views.
	Query string `json:"query,omitempty"`

	// Queries holds named query strings for multi-query dashboard views.
	// Recognised keys: "tasks", "notes", "journals".
	// Any key absent or empty falls back to the DefaultDashboardQuery value.
	Queries map[string]string `json:"queries,omitempty"`

	// Display is the rendering mode.
	Display DisplayMode `json:"display,omitempty"`

	// Columns lists which node fields to show in list mode.
	Columns []string `json:"columns,omitempty"`
}

// DisplayMode enumerates the supported view rendering modes.
type DisplayMode string

const (
	DisplayList     DisplayMode = "list"
	DisplayTimeline DisplayMode = "timeline"
	DisplayProse    DisplayMode = "prose"
	DisplayBudget   DisplayMode = "budget"
)

// Ritual defines an automated check-in workflow.
// Stored at /store/rituals/{name}.jsonc.
type Ritual struct {
	// Name is the ritual's identifier.
	Name string `json:"name"`

	// Schedule defines when the ritual should trigger.
	Schedule RitualSchedule `json:"schedule"`

	// Friction controls how the ritual is presented to the user.
	Friction FrictionLevel `json:"friction"`

	// Steps is the ordered list of ritual steps.
	Steps []RitualStep `json:"steps"`
}

// RitualSchedule defines when a ritual triggers.
type RitualSchedule struct {
	// Days is the list of weekday abbreviations (mon, tue, wed, thu, fri, sat, sun).
	Days []string `json:"days,omitempty"`

	// Day is a single weekday abbreviation for weekly rituals.
	Day string `json:"day,omitempty"`

	// Time is the 24-hour time in "HH:MM" format.
	Time string `json:"time"`
}

// FrictionLevel controls how a ritual is presented.
type FrictionLevel string

const (
	// FrictionGate requires the user to complete or explicitly defer the ritual.
	FrictionGate FrictionLevel = "gate"

	// FrictionNudge appears as a bypassable suggestion.
	FrictionNudge FrictionLevel = "nudge"
)

// StepType enumerates the supported ritual step types.
type StepType string

const (
	StepQuerySummary StepType = "query_summary"
	StepQueryList    StepType = "query_list"
	StepView         StepType = "view"
	StepAction       StepType = "action"
	StepGate         StepType = "gate"
	StepPrompt       StepType = "prompt"
)

// RitualStep is a single step within a ritual.
type RitualStep struct {
	// Type determines how this step is executed.
	Type StepType `json:"type"`

	// Label is the display title for query_summary and query_list steps.
	Label string `json:"label,omitempty"`

	// Query is the Cypher query for query_summary, query_list, and gate steps.
	Query string `json:"query,omitempty"`

	// Template is the interpolation string for query_summary steps.
	Template string `json:"template,omitempty"`

	// View is the saved view name for view steps.
	View string `json:"view,omitempty"`

	// Action is the built-in action name for action steps.
	Action string `json:"action,omitempty"`

	// Condition is the Cypher query for gate steps.
	Condition string `json:"condition,omitempty"`

	// Field is the boolean return field name for gate steps.
	Field string `json:"field,omitempty"`

	// Then is the step to execute when the gate condition is true.
	Then *RitualStep `json:"then,omitempty"`

	// EmptyMessage is shown when query_list returns no results.
	EmptyMessage string `json:"empty_message,omitempty"`

	// Text is the prompt text for prompt steps.
	Text string `json:"text,omitempty"`

	// Creates defines the node created by a prompt step.
	Creates *PromptCreates `json:"creates,omitempty"`
}

// PromptCreates defines the node and optional edge created by a prompt step.
type PromptCreates struct {
	// Types is the list of template types for the new node.
	Types []string `json:"types"`

	// Edge is an optional edge to create from the new node.
	Edge *PromptEdge `json:"edge,omitempty"`
}

// PromptEdge defines the edge to create from a prompt-created node.
type PromptEdge struct {
	// Type is the edge type.
	Type string `json:"type"`

	// To is the target node UUID or a variable reference like "$today_schedule".
	To string `json:"to"`
}
