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

## Iteration D22fix — Scope hover zoom to backdrop image only

**Target:** `web-astryx/src/index.css` — `.mizu-card:hover .mizu-card-img-wrap img`

**Problem:** The hover zoom rule targeted `.mizu-card-img-wrap img` which matched ALL images in the card wrap, including the overlay logo. Logos would scale up on hover, which looked wrong.

**What changed:** Scoped the zoom transform to `.mizu-card-img` only (the backdrop image class). Also updated the reduced-motion override to match.

**Outcome:** Kept. tsc clean. Logos stay put on hover, only the backdrop zooms.

## Iteration D23 — SettingsPage section headers: visual hierarchy + spacing

**Target:** `web-astryx/src/routes/SettingsPage.tsx` — Connection, Sync, Notifications, Defaults section headings

**Problem:** Section headings (Connection, Sync, Notifications, Defaults) sat flush against the Divider with no breathing room. All used `Heading level={2}` with no distinguishing style, making sections blend together visually.

**What changed:** Added `.settings-section-header` CSS class to all four section headings. The class adds `padding-bottom: var(--spacing-2)` for breathing room before the divider, `font-size: 1.05em` for slightly more prominence, and `letter-spacing: -0.01em` for tighter, more deliberate typography.

**Outcome:** Kept. tsc clean. Section headers now have clear visual hierarchy and spacing.

## Iteration D24 — AddAnimeDialog tracking rules visual grouping

**Target:** `web-astryx/src/routes/AddAnimeDialog.tsx` — tracking rules section in the match step

**Problem:** The three tracking Switches and the start-mode Selector sat bare in the VStack with just a Divider and Heading. No visual container separated them from the match list above, making the section feel unstructured.

**What changed:** Wrapped the tracking rules (Heading + 3 Switches + Selector) in a `Card variant="muted" padding={4}` with an inner `VStack gap={3}`. This creates a clear visual grouping, consistent with how ShowDetailPage and SettingsPage use muted cards for grouped content.

**Outcome:** Kept. tsc clean. Tracking rules now have a clear visual container that separates them from the match list.

## Iteration D25 — Loading states audit

**Target:** All pages — Spinner and isLoading usage

**Problem:** Audit for consistency across loading states.

**Findings:** Loading states are already consistent:
- Full page load: `Spinner size="lg"` with label, centered in `VStack gap={3} hAlign="center" padding={6}`
- Section load: `Spinner size="md"` with label (ShowDetailPage episodes)
- Button loading: `isLoading` prop on Button (Astryx built-in spinner)
- Inline icon: `Spinner size="sm"` as button icon (test connection buttons)
- No double-spinner issues remaining (test buttons use icon-based spinner, not isLoading)

**Outcome:** No changes needed. Loading states are consistent across the app.

## Iteration D26 — ShowDetailPage hero no-backdrop fallback

**Target:** `web-astryx/src/routes/ShowDetailPage.tsx` + `web-astryx/src/index.css` — hero section when no backdrop image exists

**Problem:** When a show has no backdrop image, the hero showed a flat `--color-background-muted` block — visually empty and inconsistent with the rich hero shown when a backdrop exists.

**What changed:** Added a `.show-hero-placeholder` div that renders when `backdropUrl` is falsy. It uses a 135° diagonal gradient from a subtle accent-tinted muted to surface-mixed muted, giving the hero depth and visual interest without a backdrop. The gradient uses theme tokens so it adapts to all themes.

**Outcome:** Kept. tsc clean. No-backdrop heroes now have a subtle gradient instead of flat emptiness.

## Iteration D27 — Navigation active indicator: accent underline

**Target:** `web-astryx/src/index.css` — TopNav active item styling

**Problem:** The TopNavItem's built-in selected state uses `--color-neutral` background + semibold text. This is functional but subtle — the active page isn't immediately obvious, especially in themes where `--color-neutral` is very close to the hover background.

**What changed:** Added a 2px accent-colored underline indicator below the active nav item, using `::after` on `[aria-current="page"]`. The underline is inset by `--spacing-3` on each side so it sits under the text, not the full pill. It uses `--color-accent` so it adapts to every theme.

**Outcome:** Kept. tsc clean. Active nav item now has a clear accent underline indicator.

## Iteration D28 — Fix double-spinners + section heading consistency

**Target:** `web-astryx/src/routes/ShowDetailPage.tsx`, `web-astryx/src/routes/ShowsPage.tsx`, `web-astryx/src/index.css`

**Problem:** Two buttons had both `isLoading` (Astryx built-in spinner) AND a CSS `.spin` class on the RefreshCw icon — causing a double-spinner effect. Also, ShowDetailPage section headings (Episode list, Tracking rules) lacked the `.settings-section-header` styling that SettingsPage headings got in D23.

**What changed:**
- Removed `isLoading` from both artwork refresh buttons (ShowDetailPage + ShowsPage) — kept the spinning RefreshCw icon as the sole loading indicator, which is more contextual for a refresh action
- Added `.settings-section-header` class to "Episode list" and "Tracking rules" headings for consistent typography hierarchy across all pages
- Added `.spin` to the reduced-motion guard list

**Outcome:** Kept. tsc clean. No more double-spinners, consistent section headings across pages.

## Iteration D29 — Mobile responsiveness audit

**Target:** `web-astryx/src/routes/SettingsPage.tsx`, `web-astryx/src/index.css`

**Problem:** Two mobile responsiveness issues found:
1. SettingsPage Defaults section used `Grid columns={3}` (fixed 3 columns) — tracking rule cards would be cramped on narrow screens
2. Fixed-width TextInputs and Selectors in settings rows (width=320, 280, 260) would overflow on screens narrower than 360px

**What changed:**
- Changed SettingsPage Grid from `columns={3}` to `columns={{ minWidth: 240, max: 3 }}` — responsive auto-fill that collapses to 1 column on narrow screens, caps at 3 on wide screens
- Added mobile CSS rule in the `@media (max-width: 600px)` block: `.settings-row .astryx-text-input` and `.astryx-selector` get `width: 100%; max-width: 100%` so they fill the stacked row instead of overflowing

**Audit findings (no changes needed):**
- TopNav has built-in mobile drawer mode via Astryx
- ShowsPage card grid already uses `columns={{ minWidth: 320 }}` (responsive)
- ShowDetailPage tracking rules already use `columns={{ minWidth: 240 }}` (responsive)
- All page headers use `wrap="wrap"` on HStack
- Show hero and content use `clamp()` for responsive padding/sizing
- ActivityPage HStacks all use `wrap="wrap"`
- AddAnimeDialog wizard steps hide labels on mobile (existing rule)

**Outcome:** Kept. tsc clean. Settings page now fully responsive on mobile.

## Iteration D30 — Empty states styling consistency audit

**Target:** All pages — EmptyState component usage

**Problem:** Audit for consistency across empty states.

**Findings:** Empty states are already consistent across all pages:
- All 9 EmptyState instances use `icon={<Icon icon={...} size="lg" color="secondary" />}`
- All have `title` and `description` props
- Most have `actions` with a contextual Button (Clear search, Show all, etc.)
- Icons are contextually appropriate: Waves (empty library), CircleAlert (no results), Search (search empties), History (no activity)
- The few without actions are in dialog contexts where the next step is obvious from the wizard flow

**Outcome:** No changes needed. Empty states are consistent and well-structured.

## Iteration D31 — Micro-interactions: card overlay slide-up + nav underline animation

**Target:** `web-astryx/src/index.css` — card overlay, nav active indicator

**Problem:** Two micro-interaction refinements:
1. Card overlay content (logo/title + badge) was static on hover — no movement to complement the card lift and image zoom
2. Nav active underline appeared instantly on page change — no entrance animation

**What changed:**
- Added `transition: transform 0.2s ease` to `.mizu-card-overlay` and `transform: translateY(-2px)` on `.mizu-card:hover` — overlay content subtly slides up when hovering a card, complementing the existing card lift and image zoom
- Added `@keyframes nav-underline-in` with `scaleX(0→1)` animation on the nav active underline — the accent underline now scales in from center when navigating between pages
- Both additions have reduced-motion guards

**Outcome:** Kept. tsc clean. Cards feel more alive on hover, nav transitions feel more polished.

## Iteration D32 — Pre-build custom themes + load Figtree font

**Target:** `web-astryx/src/lib/themes.ts`, `web-astryx/index.html`, new built theme files

**Problem:** Two console warnings:
1. `[Astryx] Theme "flexoki" is using runtime style injection` — all 4 custom themes (flexoki, tokyo-night, mizu-wintry, y2k-rounded) were defined at runtime via `defineTheme()`, causing Astryx to inject `<style>` tags at runtime instead of using pre-built CSS
2. `Request for font "Figtree" blocked at visibility level 2` — flexoki and tokyo-night themes reference Figtree font but it was never loaded

**What changed:**
- Ran `bunx astryx theme build` for all 4 custom themes, generating pre-built CSS + JS files:
  - `flexoki.js` + `flexokiTheme.css`
  - `tokyo-night.js` + `tokyoNightTheme.css`
  - `mizu-wintry.js` + `mizuTheme.css`
  - `y2k-rounded.js` + `y2kRoundedTheme.css`
- Extracted `y2kRoundedTheme` from inline definition in `themes.ts` to its own file (`y2kRoundedTheme.ts`) so it could be built
- Updated `themes.ts` to import built theme objects + CSS instead of runtime `defineTheme()` — no more runtime style injection warning
- Added Figtree font loading via Google Fonts `<link>` in `index.html` with `preconnect` for performance

**Outcome:** Kept. tsc clean. Runtime style injection warning eliminated, Figtree font loads properly.

## Iteration D33 — ShowDetailPage actions bar + artwork card polish

**Target:** `web-astryx/src/index.css`, `web-astryx/src/routes/ShowDetailPage.tsx`

**Problem:** Three small visual issues:
1. Actions bar had no visual separation from the hero — both use `--color-background-muted`, so when the bar slides out it blends seamlessly with no delineation
2. Artwork card heading (`ImageIcon`) had no icon color — inherited default text color instead of secondary icon color
3. Artwork heading lacked the `.settings-section-header` class that other section headings got in D23/D28

**What changed:**
- Added `border-top: 1px solid color-mix(in srgb, var(--color-border) 50%, transparent)` to `.show-actions-bar` — subtle semi-transparent top border that creates visual separation from the hero without being heavy-handed
- Added `color="var(--color-icon-secondary)"` to the `ImageIcon` in the artwork card header
- Added `.settings-section-header` class to the Artwork heading for typography consistency

**Outcome:** Kept. tsc clean. Actions bar has clear delineation, artwork card header is visually consistent.

## Iteration D34 — Consistent section headings across all pages

**Target:** `web-astryx/src/routes/AddAnimeDialog.tsx`

**Problem:** The "Tracking rules" heading inside the AddAnimeDialog muted card (added in D24) didn't have the `.settings-section-header` class that all other section headings got in D23/D28/D33.

**What changed:** Added `.settings-section-header` class to the tracking rules heading in AddAnimeDialog for typography consistency across all pages.

**Audit findings (no changes needed):**
- ActivityPage is well-styled: log card hover, error rows with tinted background + left border, expanded details fade-in, chevron rotation
- All bare lucide icons either inherit color from parent (error rows) or are inside Astryx Button/SegmentedControlItem icon props (color handled by component)
- Dialog success screen heading is contextual (anime title) — doesn't need section header class

**Outcome:** Kept. tsc clean. All section headings across the app now use consistent typography.

## Iteration D35 — ShowsPage: Add status badge to no-image cards

**Target:** `web-astryx/src/routes/ShowsPage.tsx` — card content when no backdrop/poster image

**Problem:** Cards without an image showed the title and episode number but no status badge (enabled/disabled). The badge was only rendered inside the image overlay block, so no-image cards were missing the at-a-glance status indicator that image cards have.

**What changed:** Added the status `Badge` to the no-image card content, displayed inline with the title using an `HStack justify="between"`. The badge uses the same variant logic as the overlay badge (`show.enabled ? "success" : "neutral"`).

**Outcome:** Kept. tsc clean. All cards now show status badge regardless of whether they have an image.

## Iteration D36 — Save button success flash + ShowsPage no-image badge

**Target:** `web-astryx/src/routes/ShowDetailPage.tsx`, `web-astryx/src/index.css`

**Problem:** When saving tracking rules, the button label changed to "Saved" with a Check icon but stayed the same accent blue color — no visual confirmation that the save succeeded. The test connection button already had this success color treatment (D27/R56).

**What changed:** Added `.save-flash` CSS class that applies when `savedFlash` is true. The class overrides the primary button background/border/text to use `--color-success` / `--color-on-success` tokens with a 0.2s transition, matching the test connection button's success state. The button flashes green for 1.5s then returns to normal.

**Outcome:** Kept. tsc clean. Save button now provides clear visual feedback on success.

## Iteration D37 — Wizard poster placeholder gradient + audit sweep

**Target:** `web-astryx/src/index.css` — `.wizard-poster-placeholder`

**Problem:** The wizard poster placeholder (shown when a Sonarr series has no poster) was a flat `--color-background-muted` rectangle, inconsistent with the card image placeholder which got a gradient in D22.

**What changed:** Replaced flat background with a 135° gradient from `--color-background-muted` to a mix of surface and muted — matching the card image placeholder style from D22.

**Audit findings (no changes needed):**
- No remaining inline `style={{}}` in any `.tsx` file (only `xstyle` stylex-native and CSS custom property overrides)
- Astryx Toast only supports `'info' | 'error'` types — current mapping is correct
- Wizard step indicator has comprehensive 3-state styling with progress lines and mobile adaptation
- Scrollbars and text selection are themed (D17)
- Page transitions via `.page-enter` animation on all pages
- All section headings use `.settings-section-header` for consistent typography

**Outcome:** Kept. tsc clean. Poster placeholders now have consistent gradient styling across the app.

## Iteration D38 — ShowsPage sort dropdown: selected indicator + sort icon

**Target:** `web-astryx/src/routes/ShowsPage.tsx` — sort DropdownMenu

**Problem:** The sort dropdown showed no indication of which option was currently selected. Users had to open the menu and scan to find the active sort. The button also had no icon, making it less obvious as a sort control.

**What changed:**
- Added `Check` icon to the selected sort option via the `icon` slot on `DropdownMenuItemData` — shows a checkmark next to the active sort option
- Added `ArrowDownUp` icon to the dropdown button to signal it's a sort control
- Imported `Check` and `ArrowDownUp` from lucide-react

**Outcome:** Kept. tsc clean. Sort dropdown now shows selected state and has a clear sort icon.

## Iteration D39 — ActivityPage empty state sync button: add loading feedback

**Target:** `web-astryx/src/routes/ActivityPage.tsx` — empty state "Sync now" button

**Problem:** The "Sync now" button in the empty state (no activity yet) didn't have `isLoading` — clicking it gave no visual feedback while the sync was running. The header "Sync now" button already had `isLoading={syncing}`.

**What changed:** Added `isLoading={syncing}` and dynamic label (`syncing ? "Syncing..." : "Sync now"`) to the empty state sync button, matching the header button's behavior.

**Outcome:** Kept. tsc clean. Both sync buttons now show loading state consistently.

## Iteration D40 — Hero image loading priority + ActivityPage sync feedback

**Target:** `web-astryx/src/routes/ShowDetailPage.tsx` — hero backdrop image

**Problem:** The hero backdrop image used `loading="lazy"` — inappropriate for an above-the-fold image that's likely the LCP element. Lazy loading delays the hero image fetch until after the initial render, making the page feel slower.

**What changed:**
- Changed `loading="lazy"` to `loading="eager"` — fetches the hero image immediately
- Added `fetchPriority="high"` — tells the browser to prioritize this image in the loading queue

**Outcome:** Kept. tsc clean. Hero image loads faster, improving perceived page load speed.

## Iteration D41 — Episode monitor toggle: accent-colored pressed icon

**Target:** `web-astryx/src/routes/ShowDetailPage.tsx` — episode monitor ToggleButton

**Problem:** The episode monitor ToggleButton swapped between Eye (unmonitored) and EyeOff (monitored) icons, but both used the same default color. Per Astryx docs guidance: "Use a filled or colored icon for the pressed state so users can see the current state at a glance."

**What changed:** Added `color="var(--color-accent)"` to the `pressedIcon` (EyeOff) — when an episode is monitored, the icon shows in the accent color, making the monitored state immediately visible without needing to parse the icon shape.

**Outcome:** Kept. tsc clean. Monitored episodes now have a clear accent-colored icon for at-a-glance state recognition.

## Iteration D42 — Collapsible expand/collapse animation (skipped)

**Target:** `web-astryx/node_modules/@astryxdesign/core/src/Collapsible/Collapsible.tsx`

**Problem:** The Collapsible component uses `display: none` for hidden content, causing an instant show/hide with no animation. The `.episode-list-wrap` has a fade-in-up entrance animation, but collapse is instant.

**Investigation:** Read the Astryx Collapsible source. Content toggling is done via `styles.contentHidden: { display: 'none' }` — a StyleX-managed style applied directly on the content div. CSS height transitions don't work from `display: none`. Modifying the component in `node_modules` would be lost on reinstall.

**Outcome:** Skipped. The current behavior (instant toggle + fade-in-up entrance) is acceptable. A proper fix would require extending the Astryx Collapsible component itself (e.g., using a `grid-template-rows: 0fr → 1fr` technique), which is a larger change than warranted for this micro-interaction.

## Iteration D43 — ActivityPage load-more button visual separator

**Target:** `web-astryx/src/routes/ActivityPage.tsx` — load more button at bottom of activity list

**Problem:** The "Load more" button sat directly below the last activity card with no visual separation, making it feel like it was part of the card content rather than a distinct pagination action.

**What changed:** Added `.load-more-bar` CSS class with a subtle top border (`1px solid var(--color-border)`) + padding/margin to create visual separation from the activity cards above. The button is centered in a clear "footer" area.

**Outcome:** Kept. tsc clean. The load-more button now has clear visual separation as a pagination action.

## Iteration D44 — Season collapsible: episode count pill

**Target:** `web-astryx/src/index.css` — `.season-episode-count` in season collapsible trigger

**Problem:** The episode count next to the season label in the collapsible trigger was plain secondary text with no visual distinction. It blended into the trigger and didn't read as a count/metadata.

**What changed:** Styled `.season-episode-count` as a subtle pill: `padding: 2px 8px`, `border-radius: var(--radius-1)`, `background: var(--color-background-muted)`, `font-variant-numeric: tabular-nums` for stable number alignment. The pill sits at the right end of the trigger (flex space-between) and reads clearly as metadata.

**Outcome:** Kept. tsc clean. Episode count now has a clear visual identity as a metadata pill rather than plain text.

## Iterations D45–D48 — Audit pass (no changes needed)

**D45: Dialog entrance animation** — Read the Astryx Dialog source. It already has a built-in directional entrance animation (`enterDirectional` keyframes) that calculates the animation origin from the trigger element's position. AlertDialog wraps Dialog and inherits this. No improvement needed.

**D46: Input focus states consistency** — Audited all Astryx input components (TextInput, Selector, NumberInput, Switch, Button). All use `:focus-visible` with `2px solid var(--color-accent)` and consistent outline offsets. The app's custom focus styles (`.mizu-card`, `.astryx-list-item`, `.astryx-clickable-card`, `.astryx-collapsible > button`) also use `--color-accent`. Already consistent.

**D47: Grid gap consistency** — Audited all Grid and VStack/HStack gap values across all pages. Grid uses gap={4} everywhere. Page-level VStack gaps vary (6 vs 8) intentionally based on content density. Section headers consistently use gap={1}. No inconsistencies found.

**D48: Color contrast audit** — Reviewed all 4 custom themes (Flexoki, Tokyo Night, Mizu Wintry, Y2K Rounded). All use well-known palettes designed with contrast in mind. Only hardcoded colors in CSS are `#fff` on dark gradient overlays (card overlay title with text-shadow) and `var(--color-text-on-accent, #fff)` fallback on wizard step dots. Both are appropriate uses.

**Outcome:** No changes needed. The app is consistent across these dimensions.

## Iteration D49 — Wizard step content fade-in transition

**Target:** `web-astryx/src/routes/AddAnimeDialog.tsx` — step content area between wizard steps

**Problem:** When advancing between wizard steps (Series → Match → Post-Add, etc.), the content swapped instantly with no transition. The step indicator (dots/labels) had transitions, but the main content area didn't, creating a jarring cut.

**What changed:** Wrapped all step content blocks in a `<div key={w.addStep} className="wizard-step-content">`. The `key` prop changes with the step, causing React to unmount/remount the div — re-triggering the `fade-in-up` CSS animation (0.25s ease-out). Added `.wizard-step-content` to the `prefers-reduced-motion` guard to disable the animation for users who prefer reduced motion.

**Outcome:** Kept. tsc clean. Wizard step transitions now have a subtle fade-in-up entrance, matching the app's existing animation language (page-enter, episode-list-wrap, activity-details-expanded).

## Iteration D50 — Scroll-to-top on route change + settings error fade-in

**Target:** `web-astryx/src/App.tsx` — route changes; `web-astryx/src/index.css` — settings error text

**Problem 1:** When navigating between pages (e.g., clicking a show card from the library), the new page opened at the same scroll position as the previous page. This is a common SPA UX issue — the user expects to start at the top of each new page.

**What changed 1:** Added a `ScrollToTop` component that calls `window.scrollTo(0, 0)` on every `pathname` change. Placed it inside `<AppProvider>` but before `<Routes>` so it runs on every navigation.

**Problem 2:** Settings row error text (e.g., "URL must start with http://") appeared instantly with no animation, drawing abrupt attention.

**What changed 2:** Added `animation: fade-in-up 0.2s ease-out` to `.settings-row-error` for a subtle entrance. Added to `prefers-reduced-motion` guard.

**Outcome:** Kept. tsc clean. Page navigation now starts at the top. Settings errors fade in subtly instead of appearing abruptly.

## Iteration D51 — Activity log card: clickable summary to expand details

**Target:** `web-astryx/src/routes/ActivityPage.tsx` — activity log card summary area

**Problem:** The only way to expand an activity log's details was the small "Details" ghost button on the right. The summary area (badge + title + timestamp) wasn't clickable, even though it's the natural click target for expanding a log entry.

**What changed:** Made the `.activity-log-summary` VStack clickable when the log has details — added `onClick`, `role="button"`, `tabIndex={0}`, and `onKeyDown` (Enter/Space) handlers. Added CSS: `cursor: pointer`, subtle hover background (`--color-overlay-hover`), and `:focus-visible` accent outline for keyboard users. The Details button still works independently.

**Outcome:** Kept. tsc clean. The entire summary area is now clickable and keyboard-accessible, providing a larger click target and more natural interaction pattern.

## Iteration D52 — HTML head polish: meta description, theme-color, noscript

**Target:** `web-astryx/index.html`

**Problem:** The HTML head was missing several standard meta tags: no `<meta name="description">` (bad for SEO/sharing), no `<meta name="theme-color">` (mobile browsers show default white/black chrome instead of matching the app theme), and no `<noscript>` fallback.

**What changed:**
- Added `<meta name="description">` with a concise app description
- Added two `<meta name="theme-color">` tags — one for dark mode (`#1a1b26`, matching Tokyo Night dark body) and one for light mode (`#f2f0e5`, matching Flexoki light body), both with `media` attributes so the browser picks the right one
- Added `<noscript>` message inside `#root` for users with JS disabled

**Outcome:** Kept. tsc clean. Mobile browser chrome now matches the app theme. Search engines and link previews have a description. No-JS users get a message instead of a blank page.

## Iteration D53 — SettingsPage save button success flash

**Target:** `web-astryx/src/routes/SettingsPage.tsx` — save settings button

**Problem:** The ShowDetailPage save button flashes green with a Check icon on successful save (D36), but the SettingsPage save button had no visual success feedback — it just reverted from "Saving..." back to "Save settings" with no confirmation that the save succeeded. This was an inconsistency between the two pages with save actions.

**What changed:** Added `savedFlash` state + `savedFlashRef` timeout (1.5s) to SettingsPage, matching the ShowDetailPage pattern. The save button now shows "Saved" with a Check icon and the `.save-flash` green color override for 1.5 seconds after a successful save, then reverts to normal. Added cleanup in the existing unmount effect.

**Outcome:** Kept. tsc clean. Both save actions now have consistent success feedback — green flash + Check icon + "Saved" label.

## Iteration D54 — CSS cleanup: standard line-clamp, token-ize hardcoded values

**Target:** `web-astryx/src/index.css` — `.show-hero-overview`, `.mizu-card-overlay`

**Problem:** Several CSS values were hardcoded pixels instead of design tokens, and the overview line clamp only had the `-webkit-` prefix.

**What changed:**
- Added standard `line-clamp: 3` alongside `-webkit-line-clamp: 3` in `.show-hero-overview` for future-proofing (Firefox supports the unprefixed property)
- Replaced `margin-top: 8px` → `var(--spacing-2)` in `.show-hero-overview`
- Replaced `padding: 12px` → `var(--spacing-3)` and `gap: 8px` → `var(--spacing-2)` in `.mizu-card-overlay`

**Outcome:** Kept. tsc clean. CSS now uses spacing tokens consistently and has both prefixed and standard line-clamp.

## Iteration D55 — Scope wizard step dot transition

**Target:** `web-astryx/src/index.css` — `.wizard-step-dot`

**Problem:** The wizard step dot used `transition: all 200ms ease` — `transition: all` is generally discouraged because it can cause unexpected transitions on properties that change for non-animation reasons (e.g., layout shifts).

**What changed:** Replaced `transition: all 200ms ease` with `transition: background-color 200ms ease, color 200ms ease, box-shadow 200ms ease` — the only three properties that actually change between step states (future/past/current).

**Outcome:** Kept. tsc clean. Transition is now scoped to only the properties that animate, preventing unexpected side effects.

## Iteration D56 — Fix ShowDetailPage hero width misalignment

**Target:** `web-astryx/src/index.css` — `.show-shell`

**Problem:** The hero `show-shell` had `width: calc(100% - clamp(24px, 8vw, 64px))` and `max-width: calc(900px - 64px)`, making it up to 64px narrower per side than the content section below (which uses the full 900px with `paddingInline: clamp(12px, 4vw, 32px)`). This created a visible misalignment where the hero header was smaller than the rest of the page content — it looked like a bug rather than an intentional design choice.

**What changed:** Set `.show-shell` to `width: 100%` and `max-width: 900px` — matching the content section's `Section maxWidth={900}`. The hero content's internal padding (`padding: clamp(20px, 5vw, 32px) clamp(12px, 4vw, 32px)`) already aligns horizontally with the content section's `paddingInline: clamp(12px, 4vw, 32px)`, so text in the hero now aligns with text in the content below.

**Outcome:** Kept. tsc clean. Hero and content are now the same width, with text aligned consistently.

## Iteration D57 — Wizard poster loading skeleton

**Target:** `web-astryx/src/routes/AddAnimeDialog.tsx` — series list poster thumbnails

**Problem:** When a poster URL existed, only the `<img>` was rendered with `opacity: 0` until load. While the image was loading, the list item showed an empty 32×44 blank space — no visual placeholder. The `wizard-poster-placeholder` gradient was only shown when no URL existed at all.

**What changed:** Wrapped the poster image in a `.wizard-poster-wrap` div (position: relative, 32×44) containing both the placeholder gradient div and the image (position: absolute, inset: 0). Now the placeholder gradient is visible behind the image while it loads, creating a skeleton effect. When the image loads (opacity → 1), it covers the placeholder seamlessly.

**Outcome:** Kept. tsc clean. Poster thumbnails now show a gradient placeholder during loading instead of blank space, matching the pattern used for show card images.

## Iteration D58 — Actions bar padding alignment with hero content

**Target:** `web-astryx/src/index.css` — `.show-actions-bar` horizontal padding

**Problem:** The actions bar used `padding: ... var(--spacing-3) ...` (12px) for horizontal padding, while the hero content above it used `padding: ... clamp(12px, 4vw, 32px) ...`. On desktop (900px wide), the hero text had 32px padding while the actions bar controls had only 12px — a 20px misalignment that made the controls look pushed toward the edges relative to the hero text.

**What changed:** Changed the actions bar horizontal padding from `var(--spacing-3)` (12px) to `clamp(12px, 4vw, 32px)` — matching the hero content's horizontal padding exactly. Now the controls in the actions bar align vertically with the hero text above them.

**Outcome:** Kept. tsc clean. Actions bar controls now align with hero text across all viewport sizes.

## Iteration D59 — List item active state for press feedback

**Target:** `web-astryx/src/index.css` — `.astryx-list .astryx-list-item`

**Problem:** List items had a hover background (`--color-overlay-hover`) but no `:active` state. On touch devices and mouse clicks, there was no visual feedback when pressing a list item — it went from hover directly to no state with no intermediate "pressed" cue.

**What changed:** Added `.astryx-list .astryx-list-item:active { background-color: var(--color-overlay-pressed); }` — uses the existing Astryx `--color-overlay-pressed` token (slightly stronger than hover) for press feedback. Not scoped to `@media (hover: hover)` so it works on touch too.

**Outcome:** Kept. tsc clean. List items now have a subtle press feedback using the design system's pressed overlay token.

## Iteration D60 — Activity log card expanded indicator

**Target:** `web-astryx/src/routes/ActivityPage.tsx` + `web-astryx/src/index.css` — `.activity-log-card`

**Problem:** When a log card was expanded, the chevron rotated but the card border didn't change — the only visual indicator of expanded state was the chevron direction and the appearance of details below. On hover the card already got `border-color: var(--color-border-emphasized)`, but expanded cards lost that emphasis when the mouse moved away.

**What changed:** Added `activity-log-card-expanded` class to the Card when expanded, with `border-color: var(--color-border-emphasized)` — persisting the emphasized border while expanded. The card already had `transition: border-color 0.2s ease` so the change animates smoothly.

**Outcome:** Kept. tsc clean. Expanded log cards now have a persistent emphasized border, making the expanded state visually clear without relying solely on the chevron.

## Iteration D61 — Wizard step description info icon

**Target:** `web-astryx/src/routes/AddAnimeDialog.tsx` + `web-astryx/src/index.css` — `.wizard-step-description`

**Problem:** The step description was a plain text block in a muted pill. It lacked visual structure — just a blob of secondary text with no icon to signal "this is guidance/info."

**What changed:** Added an `Info` icon (14px, secondary color) to the left of the description text. Updated the CSS to use `display: flex` with `align-items: flex-start` and a `gap: var(--spacing-2)`. The icon has `flex-shrink: 0` and `margin-top: 2px` to align with the first line of text.

**Outcome:** Kept. tsc clean. The step description now has a clear info icon prefix, giving it more visual structure and making it read as guidance rather than just muted text.

## Iteration D62 — Hero max-height check (no-op)

**Target:** `.show-hero` min-height/max-height

**Problem:** Considered adding a max-height to prevent very long titles from making the hero excessively tall.

**Decision:** No-op. The hero has `overflow: hidden` and content is bottom-aligned with `justify-content: flex-end`. Adding a max-height would clip long titles, which is worse than letting it grow naturally. The `min-height: clamp(160px, 30vw, 220px)` already ensures a consistent minimum.

## Iteration D63 — Content section vertical spacing check (no-op)

**Target:** Gap between hero Section and content Section on ShowDetailPage

**Problem:** Considered whether the gap between the hero and content section was inconsistent with other page spacing.

**Decision:** No-op. The actions bar bridges the hero and content with `margin-top: calc(-1 * var(--radius-container))`, creating a seamless visual connection. The spacing is intentional and consistent.

## Iteration D64 — Mobile episode filter parity with desktop

**Target:** `web-astryx/src/routes/ShowDetailPage.tsx` — `.episode-filters-mobile` SegmentedControl

**Problem:** The desktop episode filter showed 5 tabs with episode counts (`All (67)`, `Manga (4)`, `Anime (N)`, `Mixed (N)`, `Filler (N)`) and icons. The mobile filter only showed 4 tabs with no counts, different labels ("Canon" instead of "Manga"), and was missing the "Anime" tab entirely. Mobile users got less information and fewer filtering options.

**What changed:** Updated the mobile SegmentedControl to match the desktop version exactly — same 5 items, same labels with counts, same icons. The SegmentedControl's `layout="fill"` handles the narrower width by distributing space equally.

**Outcome:** Kept. tsc clean. Mobile users now have the same filtering granularity and information as desktop users.

## Iteration D65 — CSS consolidation + reduced-motion completeness

**Target:** `web-astryx/src/index.css` — `.mizu-card-img` duplicate declarations + `prefers-reduced-motion` block

**Problem 1:** `.mizu-card-img` had two separate CSS blocks — one at line 181 (`transition: transform 0.3s ease` + position styles) and another at line 266 (`opacity: 0; transition: opacity 0.3s ease, transform 0.3s ease`). The first block's transition was dead code, overridden by the second. The hover zoom rule (`.mizu-card:hover .mizu-card-img { transform: scale(1.04) }`) was sandwiched between them, separated from the transition it depended on.

**What changed:** Consolidated into a single `.mizu-card-img` block with all properties (position, opacity, transition) followed immediately by the `.is-loaded` state and the `:hover` zoom rule. Removed the duplicate block.

**Problem 2:** Several elements with `transition: border-color 0.2s ease` or `transition: background-color` were missing from the `prefers-reduced-motion` block: `.activity-log-card`, `.season-card`, `.artwork-card`, `.activity-log-summary[role="button"]`, `.save-flash[data-variant="primary"]`. While these are subtle color transitions (not movement), the WCAG reduced-motion preference asks for all motion to be minimized.

**What changed:** Added all five selectors to the existing `transition: none` rule in the reduced-motion block.

**Outcome:** Kept. tsc clean. CSS is more maintainable with no duplicate declarations, and all transitions respect reduced-motion preference.

## Iteration D66 — Fix non-existent --spacing-80 token in wizard list

**Target:** `web-astryx/src/index.css` — `.wizard-list` max-height

**Problem:** `.wizard-list` used `max-height: var(--spacing-80)` but the Astryx spacing scale only goes up to `--spacing-12` (48px). The token `--spacing-80` doesn't exist in any theme or in the Astryx core token definitions. This means `max-height` resolved to nothing — the wizard series list never scrolled, it just grew until the dialog's own `maxHeight="85vh"` kicked in, pushing the footer buttons off-screen.

**What changed:** Replaced `var(--spacing-80)` with `400px` — a reasonable fixed height that allows ~8-10 list items visible before scrolling, fitting comfortably within the 640px dialog with its header and footer.

**Outcome:** Kept. tsc clean. The wizard list now scrolls properly within the dialog instead of overflowing.

## Iteration D67 — Fix non-existent --radius-1 and --radius-2 tokens

**Target:** `web-astryx/src/index.css` — 6 occurrences of `--radius-1` and 1 of `--radius-2`

**Problem:** The CSS used `--radius-1` (4px) and `--radius-2` (8px) tokens that don't exist in the Astryx token system. The actual named tokens are `--radius-inner` (4px), `--radius-element` (8px), `--radius-container` (12px), etc. The `--radius-2` usage had a fallback (`var(--radius-2, 8px)`) so it worked accidentally, but the four `--radius-1` usages had no fallback — they resolved to nothing, meaning those elements had no border-radius.

**Affected elements:** `.season-episode-count` pill, `.wizard-poster-thumb` border, `.wizard-poster-placeholder` border, `.activity-log-summary[role="button"]` focus ring, `.activity-error-row` left border radius.

**What changed:** Replaced all `--radius-1` with `--radius-inner` and `--radius-2` with `--radius-element` — the correct Astryx token names for the same values.

**Outcome:** Kept. tsc clean. All border-radius values now use valid design system tokens. The season episode count pill, wizard poster thumbnails, and activity log summary now have proper rounded corners that were previously missing.

## Iteration D68 — Fix ShowDetailPage centering + hero/content width alignment

**Target:** `web-astryx/src/index.css` — `.section-centered` and `.show-shell`

**Problem 1 (centering):** The `.section-centered` class sets `marginInline: auto` to center Section components, but Astryx's Section component renders StyleX-generated `marginInlineStart` and `marginInlineEnd` properties (for nested padding escape) that override our plain CSS `marginInline: auto`. On pages with no parent container padding to escape, these resolve to `calc(-1 * 0px)` = `0px`, which prevents centering.

**Fix 1:** Added `!important` to `.section-centered { marginInline: auto !important; }` to override the StyleX escape margins. This ensures all page Sections (ShowsPage, ActivityPage, SettingsPage, ShowDetailPage) are properly centered.

**Problem 2 (hero wider than content):** The hero `.show-shell` had `width: 100%; max-width: 900px` — going edge-to-edge within the 900px container. The content Section below has `paddingInline: clamp(12px, 4vw, 32px)` (via `.show-content-padded`), making the actual content ~836px on desktop. The hero image appeared wider than the content below it.

**Fix 2:** Added `padding-inline: clamp(12px, 4vw, 32px)` and `box-sizing: border-box` to `.show-shell`, matching the content's horizontal padding exactly. The hero image is now inset by the same amount as the content text, so both align vertically.

**Outcome:** Kept. tsc clean. All pages should now be properly centered, and the ShowDetailPage hero aligns with the content below it.
