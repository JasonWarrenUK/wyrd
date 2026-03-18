# Theme Palettes

All palettes built from [Reasonable Colors](https://www.reasonable.work/colors/) (LCH colour space, MIT licence). Contrast guarantees: shade difference of 3 = 4.5:1 minimum (WCAG AA); difference of 4 = 7:1 minimum (WCAG AAA).

## Theme 1: Cairn

*Dark, warm. Violet structure with cinnamon/amber warmth. Stacked stone at dusk.*

| Role | Hex | Source |
|------|-----|--------|
| bg.primary | `#000000` | black |
| bg.secondary | `#222222` | gray-6 |
| fg.primary | `#f6f6f6` | gray-1 |
| fg.muted | `#8b8b8b` | gray-3 |
| accent.primary | `#9b70ff` | violet-3 |
| accent.secondary | `#d57300` | cinnamon-3 |
| border | `#3e3e3e` | gray-5 |
| selection | `#2d0fbf` | violet-5 |
| energy.deep | `#794aff` | violet-4 |
| energy.medium | `#b98300` | amber-3 |
| energy.low | `#6f6f6f` | gray-4 |
| overflow.warn | `#d57300` | cinnamon-3 |
| overflow.critical | `#ff4647` | red-3 |

## Theme 2: Peat

*Dark, cool. Boggy dark greens and teal accents. Earthy, damp, quiet.*

| Role | Hex | Source |
|------|-----|--------|
| bg.primary | `#000000` | black |
| bg.secondary | `#002722` | teal-6 |
| fg.primary | `#d7fff7` | teal-1 |
| fg.muted | `#6f6f6f` | gray-4 |
| accent.primary | `#009e8c` | teal-3 |
| accent.secondary | `#d57300` | cinnamon-3 |
| border | `#00443c` | teal-5 |
| selection | `#00443c` | teal-5 |
| energy.deep | `#009e8c` | teal-3 |
| energy.medium | `#b98300` | amber-3 |
| energy.low | `#6f6f6f` | gray-4 |
| overflow.warn | `#d57300` | cinnamon-3 |
| overflow.critical | `#ff4647` | red-3 |

## Theme 3: Kiln

*Light, warm. Terracotta. Orange and cinnamon earth tones, red glaze accents.*

| Role | Hex | Source |
|------|-----|--------|
| bg.primary | `#fff8f5` | orange-1 |
| bg.secondary | `#fff8f3` | cinnamon-1 |
| fg.primary | `#401600` | orange-6 |
| fg.muted | `#752100` | orange-5 |
| accent.primary | `#cd3c00` | orange-4 |
| accent.secondary | `#e0002b` | red-4 |
| border | `#ffded1` | orange-2 |
| selection | `#ffdfc6` | cinnamon-2 |
| energy.deep | `#cd3c00` | orange-4 |
| energy.medium | `#ac5c00` | cinnamon-4 |
| energy.low | `#8b8b8b` | gray-3 |
| overflow.warn | `#ac5c00` | cinnamon-4 |
| overflow.critical | `#e0002b` | red-4 |

## Theme 4: Fell

*Light, cartographic. Cream paper, contour-line greens, footpath pinks, grid blue.*

| Role | Hex | Source |
|------|-----|--------|
| bg.primary | `#fff8ef` | amber-1 |
| bg.secondary | `#fff9e5` | yellow-1 |
| fg.primary | `#371d00` | cinnamon-6 |
| fg.muted | `#926700` | amber-4 |
| accent.primary | `#004908` | green-5 |
| accent.secondary | `#82002c` | raspberry-5 |
| border | `#ffe0b2` | amber-2 |
| selection | `#d5f200` | lime-2 |
| energy.deep | `#004908` | green-5 |
| energy.medium | `#82002c` | raspberry-5 |
| energy.low | `#926700` | amber-4 |
| overflow.warn | `#cd3c00` | orange-4 |
| overflow.critical | `#e0002b` | red-4 |

## Shared Across All Themes

**Glyphs** (consistent regardless of theme):

| Signal | Glyph | Meaning |
|--------|-------|---------|
| `▓` | Dense fill | Deep work schedule block |
| `█` | Solid fill | Medium energy schedule block |
| `░` | Light fill | Low energy schedule block |
| `◆` | Diamond | Overflow / time displacement |
| `⊘` | Circle-slash | Blocked |
| `◇` | Open diamond | Waiting |
| `●` | Filled circle | Budget OK |
| `◐` | Half circle | Budget caution |
| `○` | Open circle | Budget over |

## 16-Colour Fallback Mapping

| Role | Dark themes | Light themes |
|------|-------------|--------------|
| bg | black | white |
| fg | white | black |
| accent.primary | magenta (Cairn) / cyan (Peat) | green (Fell) / red (Kiln) |
| accent.secondary | yellow | yellow |
| energy.deep | magenta / cyan | green / red |
| energy.medium | yellow | yellow |
| energy.low | bright black | bright white |
| overflow.warn | yellow | yellow |
| overflow.critical | red | red |

At 16 colours, glyph differentiation is the primary distinguishing mechanism for energy levels.
