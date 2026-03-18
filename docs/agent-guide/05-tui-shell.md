# Phase 3: TUI Shell

*Agent brief. Read this, then read the referenced docs.*

## Your Mission

Build the Bubble Tea application skeleton: the split-pane layout, theme engine, keyboard model, command palette, and status bar. Phase 4 agents mount views, rituals, schedule, and capture into this frame. Your output should be a working terminal app that boots, renders themed panes with placeholder content, and responds to keyboard navigation.

## Required Reading

- [Design language](../aesthetics/design-language.md)
- [Theme palettes](../aesthetics/theme-palettes.md)
- [Theme format](../formats/theme.md)
- [ADR-002: Stack selection](../adr/adr-002-stack-selection.md)
- [ADR-007: Theme system](../adr/adr-007-theme-system.md)

## Deliverables

### 1. Bubble Tea Application Root

The top-level `tea.Model` that owns the application state. It should follow the Elm architecture strictly: all state in the Model, all state transitions in Update, all rendering in View.

The app model holds: the active theme, the current left-pane content, the current right-pane content, focus state (which pane is active), the command palette state, and the status bar state.

### 2. Split-Pane Layout

Two main panes (left and right) with a status bar spanning the bottom. Use Lipgloss for box rendering, borders, and spacing. The layout from the design language doc:

```
┌──────────────────────────┬─────────────────────────┐
│   Left pane              │   Right pane             │
│   (schedule/view/ritual) │   (detail/editor)        │
├──────────────────────────┴─────────────────────────┤
│ [status bar]                                        │
└────────────────────────────────────────────────────┘
```

Panes should resize with the terminal window. The left pane takes roughly 50% width by default (configurable later).

### 3. Theme Engine

Load theme JSONC files from `/store/themes/`. Detect terminal colour capability:

- `COLORTERM=truecolor` or `COLORTERM=24bit` → use `tiers.truecolor`
- `TERM` containing `256color` → use `tiers.256`
- Fallback → use `tiers.16`

Expose a `Theme` struct with methods like `BgPrimary()`, `FgMuted()`, `AccentPrimary()` etc. that return `lipgloss.Color` values appropriate for the detected tier.

Load the theme specified in `/store/config.jsonc` (field: `"theme": "ember-violet"`). Fall back to the first available theme if the specified one doesn't exist.

### 4. Keyboard Model

Vim-style navigation as described in the design language:

- `h`/`j`/`k`/`l` for directional movement within panes
- `gg` / `G` for top/bottom of lists
- `/` for in-pane search
- `Ctrl+W` to switch focus between panes
- `q` or `Ctrl+C` to quit
- `:` opens the command palette
- `Ctrl+K` opens fuzzy command search

The keyboard layer must be extensible: Phase 4 agents will register additional keybindings for their features, and plugins declare their own keybindings in manifests.

### 5. Command Palette

A modal overlay triggered by `:` that shows all registered commands. Supports tab completion and displays keybinding hints next to each command. `Ctrl+K` opens the same palette in fuzzy search mode (filter as you type).

Start with a minimal command set: `quit`, `theme {name}`, `sync`, `help`. Phase 4 agents will register additional commands.

### 6. Status Bar

Spans the full width of the terminal. Divided into sections:

- Left: capture bar (text input for quick node creation; implementation deferred to Phase 4, but reserve the space)
- Right: time tracker display (placeholder; Phase 4 fills in the logic)

Section headers throughout the TUI use uppercase with letter-spacing in muted text colour: `TODAY'S SCHEDULE`, `EDGES`, `BUDGETS`.

### 7. Empty States

When a pane has no content, display a message in muted text rather than a blank rectangle. Phase 4 agents will provide contextual empty messages; your job is the rendering infrastructure.

## Constraints

- The app must start and render within 200ms on a store with 100 nodes. Theme loading and index building happen during startup; profile both.
- Lipgloss styles must be derived from the active theme, never hardcoded. If someone changes the theme at runtime (via the command palette), the entire UI re-renders with the new colours.
- The Bubble Tea model must be structured so Phase 4 agents can add new pane content types without modifying the shell. Use an interface for pane content (e.g. `PaneModel` with `Update`, `View`, and `KeyBindings` methods).
- Border characters must come from the theme's glyph set or from Lipgloss defaults. Don't hardcode box-drawing characters that might not render in all terminals.

## Testing Requirements

- The app boots and renders without crashing on an empty store (no nodes, no edges, just the config and one theme).
- Theme switching via command palette changes all colours immediately.
- Terminal resize is handled gracefully (panes redistribute width, no rendering artefacts).
- Keyboard input is routed correctly to the focused pane.
- Test all four shipped themes to verify colour role mapping.

## Output Structure

```
internal/tui/
  app.go            # Bubble Tea application root (Model, Update, View)
  theme.go          # Theme loading, tier detection, colour accessors
  layout.go         # Split-pane layout with Lipgloss
  keyboard.go       # Key routing, vim motions, pane switching
  palette.go        # Command palette (modal overlay)
  statusbar.go      # Status bar rendering
  pane.go           # PaneModel interface for extensibility
  app_test.go
  theme_test.go
  layout_test.go
```
