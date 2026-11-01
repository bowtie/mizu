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

## Iteration 11 — AddAnimeDialog: poster fade-in + step description styling

**Target:** `AddAnimeDialog.tsx` — poster thumbnails and step description text

**Problem:** Poster thumbnails in the series list appeared abruptly with no fade-in (inconsistent with card images which now fade in). The step description text sat as plain unstyled text between the wizard steps and the divider — no visual hierarchy separating it from the content below.

**What changed:**
- Added `opacity: 0` + `transition: opacity 0.2s ease` to `.wizard-poster-thumb`, with `.is-loaded` class added via `onLoad` handler — consistent with card/hero image fade-in pattern
- Added `.wizard-step-description` CSS class — muted background, rounded corners, and secondary text color to visually distinguish the contextual help text from the step content below it

**Outcome:** Kept. tsc clean. Poster thumbnails now fade in smoothly. Step description has visual weight as a contextual hint rather than floating plain text.

## Iteration 12 — Page entrance animation + reduced-motion coverage

**Target:** All route pages — page mount animation and reduced-motion audit

**Problem:** Pages appeared instantly with no transition, making navigation feel abrupt. The `prefers-reduced-motion` block didn't cover several newer transitions (wizard steps, poster fade-in, chevron).

**What changed:**
- Added `@keyframes fade-in-up` — subtle 8px upward translate + opacity fade over 300ms
- Added `.page-enter` class — applied to the main content Section of all 4 pages (ShowsPage, ActivityPage, SettingsPage, ShowDetailPage content section)
- Expanded `prefers-reduced-motion: reduce` block to cover all new transitions: `.wizard-poster-thumb`, `.wizard-step-dot`, `.wizard-step-label`, `.wizard-step::after`, `.button-icon-chevron`, and disabled `.page-enter` animation

**Outcome:** Kept. tsc clean. Page transitions now have a subtle entrance animation. All animations respect reduced-motion preferences.

## Iteration 13 — Card grid staggered entrance animation

**Target:** ShowsPage card grid — cards animate in with a cascade effect

**Problem:** Cards in the library grid appeared all at once. A staggered entrance creates a more polished, app-like feel.

**What changed:**
- Added `@keyframes card-enter` — 12px upward translate + slight scale (0.98→1) + opacity fade over 350ms
- Applied to `.mizu-card` with `backwards` fill mode (cards start invisible before their delay)
- Staggered delays via `nth-child` selectors — first 8 cards get 40ms incremental delays, cards 9-11 get diminishing increments, card 12+ all at 360ms (caps the cascade for large libraries)
- Added `.mizu-card { animation: none }` to reduced-motion block

**Outcome:** Kept. tsc clean. Library cards now cascade in on page load — subtle but noticeable polish.

## Iteration 14 — ActivityPage log card hover + expanded details animation

**Target:** `ActivityPage.tsx` — log cards and expandable details section

**Problem:** Activity log cards had no hover feedback. The expanded details section appeared instantly with no transition when clicking "Details".

**What changed:**
- `.activity-log-card` CSS class — border-color transition on hover (muted → emphasized border), gives interactive feedback
- `.activity-details-expanded` CSS class — reuses `fade-in-up` keyframe for a 200ms entrance animation when details expand
- Both added to reduced-motion block

**Outcome:** Kept. tsc clean. Log cards now respond to hover, details section animates in smoothly.

## Iteration 15 — ShowDetailPage season card hover + episode list fade-in

**Target:** ShowDetailPage season cards and Collapsible content

**Problem:** Season cards had no hover feedback. Episode list content inside Collapsible appeared instantly when expanded.

**What changed:**
- `.season-card` CSS class — border-color transition on hover (same pattern as activity log cards)
- `.episode-list-wrap` — reuses `fade-in-up` keyframe for 200ms entrance animation when Collapsible expands
- Both added to reduced-motion block

**Outcome:** Kept. tsc clean. Season cards respond to hover, episode lists fade in smoothly when expanded.

## Iteration 16 — Favicon fix + minor cleanup

**Target:** `web-astryx/index.html` — missing favicon causing 404

**Problem:** No favicon was defined, causing a 404 request for `/favicon.ico` on every page load.

**What changed:**
- Added an inline SVG emoji favicon (🌊 wave, matching the app's theme) via data URI — no extra file needed, no 404

**Outcome:** Kept. tsc clean. No more favicon 404 in console.

## Iteration 17 — Custom scrollbar styling + text selection color

**Target:** Global CSS — scrollbars and text selection

**Problem:** Default browser scrollbars looked out of place against the themed UI. Text selection used the browser default blue.

**What changed:**
- `scrollbar-width: thin` + `scrollbar-color` for Firefox — thin themed scrollbars
- `::-webkit-scrollbar` styling for Chrome/Safari — 8px width, transparent track, themed thumb with rounded corners and hover state
- `::selection` — accent-tinted selection background using `color-mix` at 25% opacity

**Outcome:** Kept. tsc clean. Scrollbars and text selection now match the app's theme.

## Iteration 18 — AddAnimeDialog success screen + logo fade-in regression fix

**Target:** AddAnimeDialog post-add success screen + ShowsPage card logo visibility

**Problem:** The success screen had a plain Check icon with no visual weight. Also, D4's image fade-in rule (`.mizu-card-img-wrap img { opacity: 0 }`) was hiding overlay logos because it targeted all images inside the wrap, not just the backdrop.

**What changed:**
- **Logo fix:** Scoped fade-in to `.mizu-card-img` only (the backdrop image class), not `.mizu-card-img-wrap img` (which also matched the logo)
- **Success screen:** Wrapped Check icon in `.wizard-success-icon` — 64px circular badge with success-tinted background and border
- **Success pulse animation:** `@keyframes success-pulse` — scale from 0.8→1.05→1 with opacity fade, gives a satisfying "pop" on appearance
- **Success screen entrance:** `.wizard-success` reuses `fade-in-up` for the whole section
- All added to reduced-motion block

**Outcome:** Kept. tsc clean. Logos visible again. Success screen now has a polished circular badge with a pulse animation.

## Iteration 19 — ShowDetailPage artwork card visual consistency

**Target:** ShowDetailPage artwork card

**Problem:** The artwork Card used the default variant (white/surface background) while the season cards below used `variant="muted"` — visual inconsistency within the same page. No hover feedback on the artwork card.

**What changed:**
- Changed artwork Card to `variant="muted"` to match season cards
- Added `.artwork-card` class with same hover border transition as `.season-card`
- Combined hover rules for both card classes

**Outcome:** Kept. tsc clean. Artwork card now visually consistent with season cards on the same page.

## Iteration 20 — ShowDetailPage hero text readability

**Target:** ShowDetailPage hero title and overview text

**Problem:** Hero title and overview text sit over a backdrop image with a gradient overlay. In some themes or with bright/busy images, the text could be hard to read where the gradient is weaker (upper portion of the content area).

**What changed:**
- Added `text-shadow: 0 1px 6px color-mix(in srgb, var(--color-background-body) 60%, transparent)` to hero `h1` and `.show-hero-overview`
- Uses the body background color at 60% opacity — dark shadow in dark mode (behind light text), light shadow in light mode (behind dark text) — ensures readability in both themes
- Subtle enough not to look heavy-handed

**Outcome:** Kept. tsc clean. Hero text remains readable over any backdrop image in both light and dark themes.

## Iteration 21 — ActivityPage error rows — tinted background + left border accent

**Target:** ActivityPage error rows (both run-level and item-level)

**Problem:** Error rows were just red text with an alert icon — no visual container to make them scannable at a glance when scanning a list of activity items.

**What changed:**
- Added tinted background: `color-mix(in srgb, var(--color-error) 8%, transparent)` — subtle red tint
- Added left border accent: 2px solid error-colored border on the inline-start side
- Added padding and margin for spacing
- Rounded corners on the right side (matching the left border accent)
- Works in both light and dark themes via `color-mix`

**Outcome:** Kept. tsc clean. Errors now stand out visually with a tinted container and accent border.

## Iteration 22 — ShowsPage card overlay gradient + empty placeholder

**Target:** ShowsPage card image overlay gradient and empty card placeholder

**Problem:** The overlay gradient used hardcoded `rgba(0, 0, 0, ...)` which doesn't adapt to theme. The empty card placeholder had no background — just a centered icon on the muted wrap background.

**What changed:**
- **Overlay gradient:** Replaced hardcoded `rgba(0, 0, 0, 0.45→0)` with `color-mix(in srgb, var(--color-background-body) 50%→20%→0%)` — adapts to light/dark themes. Extended gradient to 55% height for smoother fade. Added opacity transition on hover (fades to 70% on hover, letting the image show through more).
- **Empty placeholder:** Added 135° gradient from `--color-background-muted` to a mix of surface and muted — gives empty cards a subtle visual texture instead of a flat background.

**Outcome:** Kept. tsc clean. Card overlays now theme-aware, empty cards have subtle depth.
