# Look: ganglion's design system (draft 1, 2026-11-14)

Builds on 07/R4 (snazzy, minimalist, one palette), 08/D35 (light and dark, gruvbox as a taste reference), 08/V1–V3. Live reference: http://framework:7171/design. Tokens: ganglion/static/css/base.css.

## Palette

C1. Surfaces and ink are warm neutrals (hue ~70–80 in OKLCH): paper in light mode, warm near-black in dark. Its own palette, not gruvbox.
   - light: bg #f4efe7, surface #fbf8f3, ink #2c251f (14.3:1 on surface)
   - dark: bg #161310, surface #1f1b17, ink #ece4d5 (13.5:1)
C2. One meaning per colour:
   - accent (teal): things you can act on, focus, selection. #007570 / #4ab9b2
   - fail (rust red): non-zero exit, errors. #a83825 / #f47d67
   - ok (olive): exit 0. #40731a / #80b761
   - warn (amber-brown). #8c5700 / #d29922
   - all four pass 5:1 on their surface
C3. Status never uses colour alone: every status tag has an icon and a label (✓ Exit 0, ✕ Exit 1, dashed ○ No Exit).
C4. Series colours for comparing models side by side: blue, amber, violet. Validated with the dataviz skill's checker for *all pairs* (scatter-safe), both modes:
   - light #196ea9, #be7100, #b0a2fe: worst CVD ΔE 21.1, normal-vision 25.9
   - dark #4493d0, #cc7d1b, #764297: worst CVD ΔE 15.7
   - violet is under 3:1 contrast in each mode, so series are always labelled directly (as in the playground's pane headers).
C5. Sequential ramp (magnitude, e.g. surprise bits): one amber hue, 9 steps; near-zero recedes toward the surface in both modes, so in dark mode the high end is the brightest.
C6. Diverging ramp (signed values, e.g. slot activations): blue ↔ amber with a neutral grey midpoint, 4 steps per arm. Avoids red so it never reads as failure.

## Type

Y1. IBM Plex Sans (400, 500, 600, 400 italic) for UI; IBM Plex Mono (400, 500) only for commands, output, tokens, IDs. OFL, self-hosted woff2 (latin subset), licence kept in static/fonts.
Y2. Scale 1.2 from 15px body: 12, 13, 15, 18, 22, 28, 38. Tabular figures for numbers in tables and stats.
Y3. Labels (buttons, tabs, headings, column heads) in Title Case; prose in sentence case.

## Components

K1. Card (14px radius, hairline border, soft shadow), table (hairline rows, hover wash, whole row clickable), tags/pills, segmented controls, tabs with accent underline, buttons (primary teal, default, ghost), inputs with teal focus ring, tooltips (dark ink chip), bars, histogram, token heatmap, slot matrix.
K2. Event card: `$`-prefixed command in mono, meta row, output block with line numbers, lazy "Show More / Show All" (08/N4), Wrap toggle.
K3. Theme switch (Auto / Light / Dark) in the top bar; remembered per browser; applied before first paint.

## Wire (08/D39, measured 2026-11-14)

N1. First visit ~155 KB, of which ~125 KB is fonts; every static file is served under a build hash with a one-year immutable cache, so after that a page costs only its HTML: 2–10 KB gzipped.
N2. No inline scripts or styles (strict CSP); sizes and colours are set through the CSSOM.

## Decided

D42. Design system as above. (user: "The design system looks great")

## Open questions

(none for this note)
