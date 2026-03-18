# Theme Format

Stored at `/store/themes/{name}.jsonc`.

## Structure

```jsonc
{
  "name": "Ember & Violet",
  "source": "reasonable.work/colors",
  "tiers": {
    "truecolor": {
      "bg": { "primary": "#000000", "secondary": "#222222" },
      "fg": { "primary": "#f6f6f6", "muted": "#8b8b8b" },
      "accent": { "primary": "#9b70ff", "secondary": "#d57300" },
      "border": "#3e3e3e",
      "selection": "#2d0fbf",
      "status_bar": "#222222",
      "energy": { "deep": "#794aff", "medium": "#b98300", "low": "#6f6f6f" },
      "overflow": { "warn": "#d57300", "critical": "#ff4647" },
      "budget": { "ok": "#d57300", "caution": "#d57300", "over": "#ff4647" }
    },
    "256": {
      // xterm-256 colour indices for each role
    },
    "16": {
      // ANSI base colour names for each role
    }
  },
  "glyphs": {
    "energy_deep": "▓",
    "energy_medium": "█",
    "energy_low": "░",
    "overflow": "◆",
    "blocked": "⊘",
    "waiting": "◇",
    "budget_ok": "●",
    "budget_caution": "◐",
    "budget_over": "○"
  }
}
```

## Colour Roles

| Role | Purpose |
|------|---------|
| `bg.primary` | Main background |
| `bg.secondary` | Alternate pane background, status bar |
| `fg.primary` | Body text |
| `fg.muted` | Metadata, timestamps, secondary text |
| `accent.primary` | Active elements, selected items, node titles |
| `accent.secondary` | Secondary highlights, edge labels |
| `border` | Pane dividers, box edges |
| `selection` | Focused element background |
| `energy.deep` | Deep work schedule blocks |
| `energy.medium` | Medium energy schedule blocks |
| `energy.low` | Low energy schedule blocks |
| `overflow.warn` | Schedule overrun, budget approaching limit |
| `overflow.critical` | Hard overrun, budget exceeded |
| `budget.ok` | Envelope with plenty remaining |
| `budget.caution` | Envelope approaching limit |
| `budget.over` | Envelope exceeded |

## Colour Tiers

The TUI detects terminal capabilities and selects the best available tier.

**Truecolor:** full hex values. Detected via `COLORTERM=truecolor`.

**256-colour:** nearest xterm-256 index. Detected via `TERM=xterm-256color` or similar.

**16-colour:** ANSI base colour names (`black`, `red`, `green`, `yellow`, `blue`, `magenta`, `cyan`, `white`, plus bright variants). Fallback tier.

## Glyphs

Energy levels and budget status use paired colour + glyph signalling. The glyph provides redundant information for colour-blind users. Themes can customise glyph choices (e.g., an accessibility theme might use letters instead of geometric shapes).
