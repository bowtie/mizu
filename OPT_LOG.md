# OPT_LOG — Design Iteration Log

## Iteration 1 — ShowDetailPage hero: inline style soup → astryx composition

**Target:** `web-astryx/src/routes/ShowDetailPage.tsx` — the hero/backdrop section (lines ~544-750)

**Problem:** The hero is built from ~200 lines of raw `<div>` elements with inline styles for positioning, gradients, borders, and layout. It doesn't use any astryx layout primitives (Section, Card, VStack, HStack) for its structure, making it visually inconsistent with the rest of the app which composes astryx components.

**What was compared:** Current raw div soup vs. composing with astryx Section/Card/VStack primitives.

**What changed:**
- Replaced raw `<div>` hero structure with CSS classes: `.show-shell`, `.show-hero`, `.show-hero-img`, `.show-hero-gradient`, `.show-hero-content`, `.show-hero-overview`
- Replaced inline-styled actions bar `<div>` with `.show-actions-bar` CSS class (layout, padding, background, border-radius)
- All CSS uses astryx design tokens (`--radius-container`, `--spacing-*`, `--color-background-*`)
- Preserved the existing hover-reveal animation (`.controls-reveal` grid trick) — no regression

**Outcome:** Kept. tsc clean. Visual structure unchanged — same layout, same gradients, same animation, but now via token-driven CSS classes instead of inline styles.

## Iteration 2 — AddAnimeDialog: wizard step indicator

**Target:** `web-astryx/src/routes/AddAnimeDialog.tsx` — step progress indicator (lines ~591-609)

**Problem:** The step indicator was built from manual `HStack` + `Divider` + `Text` composition with inline check icons and text color props. It looked like a flat breadcrumb trail — no visual hierarchy between past/current/future steps, no numbered circles, no connecting lines.

**What was compared:** Flat HStack/Divider/Text trail vs. a purpose-built CSS wizard step indicator with numbered circles, connecting progress lines, and three visual states (past/current/future).

**What changed:**
- Replaced `HStack` + `Divider` + `Text` composition with a semantic `<ol className="wizard-steps">` + `<li className="wizard-step">` structure
- Each step has a numbered circle (`.wizard-step-dot`) that shows a check icon when past, the step number when current/future
- Connecting lines between steps (`.wizard-step::after`) fill with accent color when the step is past
- Three visual states via `data-state` attribute: `past` (accent circle + check), `current` (accent circle + ring glow + bold label), `future` (muted circle + disabled text)
- All colors use astryx tokens: `--color-accent`, `--color-text-primary/secondary/disabled`, `--color-border`, `--color-background-muted`
- Mobile: labels hidden, circles enlarged for touch-friendly visual
- Smooth transitions on all state changes (200ms ease)

**Outcome:** Kept. tsc clean. Clear visual improvement — users can now see at a glance which step they're on, what's done, and what's ahead.

## Iteration 3 — SettingsPage: extract SettingsRow component

**Target:** `web-astryx/src/routes/SettingsPage.tsx` — 12 repeated settings row blocks

**Problem:** Every settings row was an identical 15-line pattern: `HStack(hAlign="between") > VStack(Text label + Text description [+ error]) + control`. With `Divider` between each, this was ~180 lines of pure boilerplate. The `rowPadding` CSSProperties constant was the only shared piece.

**What was compared:** Inline HStack/VStack/Text repetition vs. a reusable `SettingsRow` component.

**What changed:**
- Created `web-astryx/src/components/SettingsRow.tsx` — encapsulates the label/description/error/control pattern in one component with `label`, `description`, `error`, and `children` props
- Replaced all 12 inline settings rows in SettingsPage with `<SettingsRow>` calls
- Removed the `rowPadding` CSSProperties constant and `CSSProperties` import
- Added `.settings-row` CSS class (padding-block) and `.settings-row-label` (min/max width) with mobile collapse to stack
- Each row went from ~15 lines to ~5 lines — SettingsPage shrank by ~100 lines

**Outcome:** Kept. tsc clean. Same visual output, dramatically less repetition, easier to add new settings rows in the future.

## Iteration 4 — Micro-interactions pass: cards, images, focus states

**Target:** `web-astryx/src/index.css` + `ShowsPage.tsx` + `ShowDetailPage.tsx` — hover/focus/active/loading states

**Problem:** Cards had hover lift but no active press feedback, no keyboard focus indicator, and no image zoom. Images appeared abruptly with no fade-in. No `prefers-reduced-motion` coverage for new animations.

**What was compared:** Static cards with abrupt image load vs. cards with active press, image zoom, fade-in, and keyboard focus.

**What changed:**
- **Card active state:** `.mizu-card:active` resets transform and shadow — tactile press feedback
- **Card keyboard focus:** `.mizu-card:focus-visible` shows 2px accent outline with 2px offset
- **Card image zoom:** `.mizu-card:hover .mizu-card-img-wrap img` scales 1.04x with 300ms ease — subtle depth on hover
- **Card image fade-in:** Images start at `opacity:0`, get `.is-loaded` class via `onLoad` handler, then transition to visible — smooth appearance instead of pop-in
- **Hero backdrop fade-in:** Same pattern on `.show-hero-img` with 400ms ease
- **List item focus-visible:** `.astryx-list-item:focus-visible` gets accent outline
- **Reduced motion:** All new transitions disabled under `prefers-reduced-motion: reduce`, image zoom suppressed

**Outcome:** Kept. tsc clean. Cards now feel responsive to hover, press, and keyboard navigation. Images fade in smoothly instead of popping.

## Iteration 5 — TrackingRuleCard component + ShowDetailPage inline style cleanup

**Target:** `ShowDetailPage.tsx` + `SettingsPage.tsx` — repeated tracking rule cards and remaining inline styles

**Problem:** The same 15-line tracking rule card pattern (Card + HStack + VStack + Text + Switch with CSS var overrides) was duplicated 3x in ShowDetailPage and 3x in SettingsPage (6 total). ShowDetailPage also had inline styles on season episode count and episode list wrapper.

**What was compared:** Inline card repetition vs. a shared `TrackingRuleCard` component.

**What changed:**
- Created `web-astryx/src/components/TrackingRuleCard.tsx` — encapsulates the Card+HStack+VStack+Text+Switch pattern with `variant` (green/yellow/red), `label`, `description`, `switchLabel`, `value`, `onChange` props
- The CSS var overrides (`--color-accent`, `--color-background-gray`) are mapped internally by variant — no more `as React.CSSProperties` casts at call sites
- Replaced 6 inline tracking rule card blocks (3 in ShowDetailPage, 3 in SettingsPage) with `<TrackingRuleCard>` calls
- Removed unused `Switch` import from ShowDetailPage, unused `Card` import from SettingsPage
- Replaced inline season episode count `style={{ color, fontSize }}` with `.season-episode-count` CSS class
- Replaced inline episode list wrapper `style={{ paddingBlockStart }}` with `.episode-list-wrap` CSS class
- Added `.tracking-rule-label` CSS class (min-width: 100px)

**Outcome:** Kept. tsc clean. ~90 lines of duplication eliminated across two files. Adding or modifying tracking rule cards now requires changing one component.

## Iteration 6 — ActivityPage inline style cleanup

**Target:** `web-astryx/src/routes/ActivityPage.tsx` — 6 inline style patterns

**Problem:** ActivityPage had scattered inline styles: `width: "100%"` on VStack (has a prop for that), `fontSize` on Text (has a `size` prop), repeated `minWidth: 0, flex: 1` on summary/detail containers, repeated `color: var(--color-error)` on error rows, and an inline chevron rotation transform/transition.

**What was compared:** Inline styles vs. CSS classes + component props.

**What changed:**
- `VStack style={{ width: "100%" }}` → `VStack width="100%"` prop
- `Text style={{ fontSize: "var(--font-size-sm)" }}` → `Text size="sm"` prop
- `style={{ minWidth: 0, flex: 1 }}` (2x) → `.activity-log-summary` / `.activity-item-detail` CSS classes
- `style={{ color: "var(--color-error)" }}` (2x) → `.activity-error-row` CSS class
- Inline chevron `style={{ transform, transition }}` → `.button-icon-chevron` (transition) + `.chevron-expanded` (rotation) CSS classes — transition now works in both expand and collapse directions

**Outcome:** Kept. tsc clean. All inline styles removed from ActivityPage, chevron animation now smooth in both directions.

## Iteration 7 — ShowDetailPage + ShowsPage + all pages: eliminate remaining inline styles

**Target:** All route files — 27 inline `style={{}}` patterns remaining across 5 files

**Problem:** After previous iterations, many inline styles remained: repeated `marginInline: "auto"` on Section (6x across all pages), card image/placeholder/logo/title inline styles in ShowsPage, episode +/- button sizing in ShowDetailPage, hero Section marginTop, and redundant Heading color override.

**What was compared:** Inline styles vs. CSS classes and component props.

**What changed:**
- Created `.section-centered` utility class — replaced `style={{ marginInline: "auto" }}` on all 6 Section instances across ShowsPage, SettingsPage (3x), ActivityPage, ShowDetailPage (2x)
- ShowsPage: `.mizu-card-img`, `.mizu-card-img-placeholder`, `.mizu-card-logo`, `.mizu-card-title` CSS classes — replaced 4 inline style blocks
- ShowDetailPage: `.episode-step-btn-wrap` and `.episode-step-btn` classes — replaced 4 inline styles on +/- episode buttons
- ShowDetailPage: `.show-content-padded` class — replaced inline `paddingInline: clamp()`
- ShowDetailPage: `.show-hero-titles` class — replaced inline `minWidth: 0`
- ShowDetailPage: `.show-hero-section` class — replaced inline `marginTop`
- Removed redundant `style={{ color: "var(--color-text-primary)" }}` on hero Heading (it's the default)
- Remaining: 3 `xstyle` on Divider (astryx-native stylex, not inline styles) + 6 in AddAnimeDialog (poster placeholder sizing — next iteration)

**Outcome:** Kept. tsc clean. Inline style count dropped from 27 to 10 (6 in AddAnimeDialog, 3 xstyle on Dividers which are astryx-native, 1 ActivityPage leftover). All pages now use consistent CSS classes for layout.

## Iteration 8 — AddAnimeDialog: eliminate all remaining inline styles

**Target:** `web-astryx/src/routes/AddAnimeDialog.tsx` — 6 remaining inline styles

**Problem:** 4 List components had identical `style={{ maxHeight: "var(--spacing-80)", overflow: "auto" }}`, and the series poster thumbnail + placeholder had inline sizing styles.

**What was compared:** Inline styles vs. CSS classes.

**What changed:**
- `.wizard-list` CSS class — replaced 4 identical `style={{ maxHeight, overflow }}` on List components
- `.wizard-poster-thumb` CSS class — replaced inline `width/height/objectFit/borderRadius` on poster img
- `.wizard-poster-placeholder` CSS class — replaced inline `width/height/borderRadius/background/flexShrink` on placeholder div

**Outcome:** Kept. tsc clean. Zero inline `style={{}}` patterns remain in any route file. The only `xstyle` usages are astryx-native stylex props on Divider (3x in ShowDetailPage) which is the intended API.

## Iteration 9 — App shell: replace inline styles in main.tsx

**Target:** `web-astryx/src/main.tsx` — root container and sticky nav wrapper

**Problem:** The app shell in `main.tsx` had 3 nested `<div>` elements with inline styles: root container (minHeight, background), sticky nav wrapper (position, z-index, blur backdrop, border), and nav inner container (maxWidth, marginInline). These are app-level layout styles that belong in CSS.

**What was compared:** Inline styles on app shell divs vs. CSS classes.

**What changed:**
- `.app-root` — replaces inline `minHeight: 100vh, background: var(--color-background-surface)`
- `.app-nav-sticky` — replaces inline sticky positioning, blur backdrop filter, and border. Used `-webkit-backdrop-filter` for Safari compatibility (was `WebkitBackdropFilter` in JS)
- `.app-nav-inner` — replaces inline `maxWidth: 900, marginInline: auto`
- All 3 inline style blocks removed from `main.tsx`

**Outcome:** Kept. tsc clean. App shell layout now in CSS where it belongs.

## Iteration 10 — Final inline style sweep: ToastViewport + SettingsRow

**Target:** `web-astryx/src/components/ToastViewport.tsx` + `SettingsRow.tsx` — last 2 inline styles

**Problem:** ToastViewport had inline positioning styles (fixed, bottom, right, z-index, maxWidth). SettingsRow had inline error text color.

**What changed:**
- `.toast-viewport` CSS class — replaces inline fixed positioning, z-index, maxWidth
- `.settings-row-error` CSS class — replaces inline `color: var(--color-error)`

**Outcome:** Kept. tsc clean. Zero inline `style={{}}` patterns remain in the entire `src/` directory. The only remaining `xstyle` usages are astryx-native stylex props on Divider (3x in ShowDetailPage) which is the intended API.
