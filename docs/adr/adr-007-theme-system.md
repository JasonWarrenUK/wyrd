# ADR-007: Theme System Built on Reasonable Colors

**Status:** Accepted

## Context

The TUI needs multiple themes supporting three colour tiers (truecolor, 256-colour, 16-colour) and accessibility for common forms of colour blindness. Terminal colour work is fiddly; picking arbitrary hex values produces inconsistent contrast and accessibility failures.

## Decision

All theme palettes are built from Reasonable Colors (reasonable.work/colors), an open-source colour system in the LCH colour space with guaranteed contrast ratios.

The system's key property: any two shades with a difference of 3 produce at least 4.5:1 contrast (WCAG AA body text). A difference of 4 gives at least 7:1 (WCAG AAA). These guarantees hold across all 24 hue families and allow cross-hue mixing.

**Four themes ship with Wyrd:**

1. **Cairn** (dark, warm): near-black bg, violet accents, cinnamon/amber warmth.
2. **Peat** (dark, cool): near-black bg with teal-6 secondary, cinnamon secondary accent.
3. **Kiln** (light, warm): terracotta; orange-1/cinnamon-1 backgrounds, dark earth text, red accents.
4. **Fell** (light, cartographic): cream paper, contour-line greens, footpath pinks.

Energy levels pair colour with glyph differentiation (█ deep, ▓ medium, ░ low) so the signal doesn't depend on colour alone. Budget status uses filled/half/empty circles (●/◐/○).

Theme files are user-editable JSONC. Users can create additional themes.

## Consequences

**Positive:** contrast correctness by construction. Cross-hue pairing is safe. Three colour tiers handled per theme. The glyph system provides redundant signalling for colour-blind users.

**Negative:** palette choices are constrained to Reasonable Colors' 24 hue families with 6 shades each. Custom hex values outside the system lose the contrast guarantees. Some desired colours may not have an exact match in the system.
