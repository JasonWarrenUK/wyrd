package types

// Theme defines the colour palette and glyphs used by the TUI.
// Stored at /store/themes/{name}.jsonc.
type Theme struct {
	// Name is the display name of the theme.
	Name string `json:"name"`

	// Source is an optional attribution URL.
	Source string `json:"source,omitempty"`

	// Tiers holds colour definitions for each terminal capability tier.
	Tiers ThemeTiers `json:"tiers"`

	// Glyphs maps semantic glyph roles to Unicode characters.
	Glyphs ThemeGlyphs `json:"glyphs"`
}

// ThemeTiers holds colour definitions for each terminal capability level.
type ThemeTiers struct {
	// Truecolor holds hex values for terminals that support 24-bit colour.
	Truecolor *ColourTier `json:"truecolor,omitempty"`

	// C256 holds xterm-256 colour indices.
	C256 *ColourTier `json:"256,omitempty"`

	// C16 holds ANSI base colour names for the fallback tier.
	C16 *ColourTier `json:"16,omitempty"`
}

// ColourTier holds the colour values for a single terminal capability tier.
// Hex values are used for truecolor; xterm indices for 256; ANSI names for 16.
type ColourTier struct {
	BG        ColourBG      `json:"bg"`
	FG        ColourFG      `json:"fg"`
	Accent    ColourAccent  `json:"accent"`
	Border    string        `json:"border"`
	Selection string        `json:"selection"`
	StatusBar string        `json:"status_bar"`
	Energy    ColourEnergy  `json:"energy"`
	Overflow  ColourOverflow `json:"overflow"`
	Budget    ColourBudget  `json:"budget"`
}

// ColourBG holds background colour roles.
type ColourBG struct {
	Primary   string `json:"primary"`
	Secondary string `json:"secondary"`
}

// ColourFG holds foreground colour roles.
type ColourFG struct {
	Primary string `json:"primary"`
	Muted   string `json:"muted"`
}

// ColourAccent holds accent colour roles.
type ColourAccent struct {
	Primary   string `json:"primary"`
	Secondary string `json:"secondary"`
}

// ColourEnergy holds energy-level colour roles.
type ColourEnergy struct {
	Deep   string `json:"deep"`
	Medium string `json:"medium"`
	Low    string `json:"low"`
}

// ColourOverflow holds schedule-overrun colour roles.
type ColourOverflow struct {
	Warn     string `json:"warn"`
	Critical string `json:"critical"`
}

// ColourBudget holds budget-status colour roles.
type ColourBudget struct {
	OK      string `json:"ok"`
	Caution string `json:"caution"`
	Over    string `json:"over"`
}

// ThemeGlyphs maps semantic roles to Unicode glyph characters.
type ThemeGlyphs struct {
	EnergyDeep    string `json:"energy_deep"`
	EnergyMedium  string `json:"energy_medium"`
	EnergyLow     string `json:"energy_low"`
	Overflow      string `json:"overflow"`
	Blocked       string `json:"blocked"`
	Waiting       string `json:"waiting"`
	BudgetOK      string `json:"budget_ok"`
	BudgetCaution string `json:"budget_caution"`
	BudgetOver    string `json:"budget_over"`
}
