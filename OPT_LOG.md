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

## Iteration D69 — Centered TopNav layout with tilde NavIcon logo

**Target:** `web-astryx/src/main.tsx` — Nav component

**Problem:** The TopNav used `startContent` for nav items, placing them immediately after the heading on the left side. This is a standard left-aligned layout. The user wanted a centered navigation pattern (heading left, nav items centered, actions right) matching the Astryx example with `centerContent`. The heading also lacked a logo/icon.

**What changed:**
1. Switched from `startContent` to `centerContent` for the nav items — this triggers Astryx's grid layout mode where heading is left, nav items are centered, and endContent is right.
2. Added a `NavIcon` with a tilde (`~`) as the logo in the `TopNavHeading`. The NavIcon renders as a circular accent-colored badge containing the tilde character, styled with `fontSize: 18, fontWeight: 700`.
3. Imported `NavIcon` from `@astryxdesign/core/NavIcon`.

**Outcome:** Kept. tsc clean. The nav now uses the centered layout pattern — "Mizu" heading with a tilde logo on the left, Library/Activity/Settings nav items centered, and the theme picker on the right.

## Iteration D70 — Harmonize transition timings to design tokens

**Target:** `web-astryx/src/index.css` — 9 transition declarations using hardcoded `0.2s ease` or `200ms ease`

**Problem:** UI element transitions (hover states, border-color, background-color, transform, opacity for non-image elements) used a mix of `0.2s ease` and `200ms ease` — same duration but inconsistent notation, and neither used the Astryx design system tokens. The token `--duration-fast` (175ms) with `--ease-standard` (cubic-bezier(0.24, 1, 0.4, 1)) is the correct pair for quick UI feedback.

**What changed:** Replaced all 9 occurrences of `0.2s ease` / `200ms ease` on UI element transitions with `var(--duration-fast) var(--ease-standard)`. Affected: `.mizu-card-overlay` transform, `.save-flash` color transitions, `.season-card`/`.artwork-card` border-color, `.wizard-poster-thumb` opacity, `.activity-log-card` border-color, `.button-icon-chevron` transform, `.wizard-step-dot` background/color/box-shadow, `.wizard-step-label` color, `.wizard-step::after` background-color.

**Not changed:** Image-related transitions (`0.3s`/`0.4s` on `.mizu-card-img`, `.mizu-card-img-wrap::after`, `.show-hero-img`) — these intentionally use longer durations for image loading/fade-in. Keyframe animations (`0.2s ease-out`) — these use a different easing curve and can't use tokens in the shorthand.

**Outcome:** Kept. tsc clean. All UI feedback transitions now use consistent design system tokens (175ms with standard easing), making the app feel more unified.

## Iteration D71 — Replace manual layout with AppShell

**Target:** `web-astryx/src/main.tsx` + `web-astryx/src/index.css`

**Problem:** The app used a manual layout (`div.app-root > div.app-nav-sticky > div.app-nav-inner > Nav`) instead of Astryx's AppShell component. This meant we were missing AppShell's built-in features: skip-to-content link, `role="main"` on content area, proper sticky header in auto height mode, and responsive nav handling. The manual layout also required custom CSS for sticky positioning and centering that fought with StyleX.

**What changed:**
1. Replaced the manual `div.app-root`/`div.app-nav-sticky`/`div.app-nav-inner` structure with `<AppShell variant="surface" height="auto" topNav={<Nav />}>`. AppShell handles the layout frame, sticky header, and accessibility.
2. Removed `.app-root`, `.app-nav-sticky`, `.app-nav-inner` CSS rules.
3. Added `.astryx-app-shell-header` CSS to apply the backdrop-blur effect (semi-transparent surface + blur + saturate) that the manual nav had, targeting AppShell's header class.
4. Kept `.section-centered { marginInline: auto !important }` for content centering within the AppShell content area.

**Outcome:** Kept. tsc clean. The app now uses AppShell for structural layout with proper accessibility (skip link, role=main), and the nav retains its backdrop-blur effect.

## Iteration D72 — Update favicon to tilde matching NavIcon logo

**Target:** `web-astryx/index.html` — favicon

**Problem:** The favicon was a 🌊 emoji but D69 changed the nav logo to a tilde (`~`) in a NavIcon. The favicon didn't match the new branding.

**What changed:** Updated the SVG favicon from 🌊 to `~` with `font-weight: 700` to match the NavIcon styling.

**Outcome:** Kept. tsc clean. Favicon now matches the tilde logo in the nav.

## Iteration D73-D76 — Settings spacing, token-based colors, section variant consistency, activity spacing fix

**Targets:** `index.css`, `ShowDetailPage.tsx`

**Problems:**
1. Settings section headers (`Connection`, `Sync`, `Notifications`) had `padding-bottom: var(--spacing-2)` (8px) — too tight against the Divider below. Changed to `--spacing-3` (12px) for proper breathing room.
2. `.mizu-card-overlay-title` used hardcoded `color: #fff` and `text-shadow: rgba(0,0,0,0.6)` — not theme-aware. Replaced with `var(--color-text-on-accent, #fff)` and `color-mix(in srgb, var(--color-background-body) 70%, transparent)` for the shadow, so both adapt to active theme.
3. ShowDetailPage content Section used default `variant="section"` (surface background) while the hero used `variant="transparent"`. Since AppShell already provides a surface background, this created a visible seam between hero and content. Changed content Section to `variant="transparent"`.
4. Activity page `.load-more-bar` had `margin-block-start: var(--spacing-2)` on top of being inside a `VStack gap={3}` — double spacing. Removed the redundant margin.

**Outcome:** Kept. tsc clean. Four small consistency fixes — settings headers have more breathing room, card overlay text is theme-aware, show detail page has no surface background seam, activity load-more has correct spacing.

## Iteration D77 — Fix content centering + navbar width + revert favicon

**Targets:** `index.css`, `index.html`

**Problems:**
1. Content hugged the left side — `.section-centered { marginInline: auto !important }` was fighting StyleX's negative escape margins on Section's outer div. The `!important` wasn't reliably winning.
2. Navbar spanned full page width with no max-width constraint — didn't match the 900px content area.
3. Favicon was changed to `~` in D72 but user preferred the original 🌊 wave.

**What changed:**
1. Added `#astryx-app-shell-main { max-width: 900px; margin-inline: auto; }` — constrains the AppShell's main content area to 900px and centers it. This is the proper fix: center the container, not each individual Section.
2. Added `.astryx-top-nav { max-width: 900px; margin-inline: auto; }` — constrains the nav bar to match the content width.
3. Removed `!important` from `.section-centered` — no longer needed since the parent container handles centering.
4. Reverted favicon from `~` back to 🌊.

**Outcome:** Kept. tsc clean. Content and navbar are both centered at 900px max-width. Favicon restored to wave.

## Iteration D78 — Fix show detail z-index + hero/content width mismatch

**Targets:** `index.css`, `ShowDetailPage.tsx`

**Problems:**
1. On the anime details page, content scrolled over the sticky header — `.show-hero` had `z-index: 2` which was higher than AppShell's header `z-index: 1`.
2. The hero header was narrower than the content below it — the content VStack had `padding={4}` (16px from Astryx) **plus** `.show-content-padded`'s `clamp(12px, 4vw, 32px)`, while the hero only had the clamp padding from `.show-shell`. Double padding = content inset by extra 16px on each side.

**What changed:**
1. Removed `z-index: 2` from `.show-hero` — `isolation: isolate` alone is enough for internal stacking (image → gradient → content), and without the z-index it won't overlap the sticky header.
2. Added `z-index: 100` to `.astryx-app-shell-header` to ensure the header stays above all page content.
3. Removed `padding={4}` from the content VStack in ShowDetailPage — `.show-content-padded` already handles horizontal padding with the same `clamp(12px, 4vw, 32px)` as `.show-shell`.
4. Added `paddingBlock: var(--spacing-4)` to `.show-content-padded` to compensate for the removed VStack vertical padding.
5. Removed redundant `max-width: 900px; margin-inline: auto` from `.show-shell` — `#astryx-app-shell-main` already constrains and centers the container.

**Outcome:** Kept. tsc clean. Header stays above content (z-index 100), hero and content have matching horizontal padding.

## Iterations D79-D92 — Show detail page: actions bar, artwork, hover-reveal, overview caching

**Targets:** `index.css`, `ShowDetailPage.tsx`, `models.go`, `database.go`, `sync.go`, `router.go`

**Summary of changes across these iterations:**
- D79: Removed z-index from actions bar/controls-reveal so sticky header stays on top
- D80: Added spacing between hero section and artwork content
- D81: Fixed translucent bar — fully hide actions bar when collapsed (translateY -100%)
- D82: Restored z-index:1 on .show-hero so actions bar hides behind it
- D83: Fixed translucent bar — clip controls-reveal overflow when collapsed (overflow:hidden on 0fr grid)
- D84: Removed padding-inline from .show-shell so hero image goes edge-to-edge
- D85: Replaced activity log pill badges with round icon bubbles on left side (D85b-D85d: fixed alignment, full-width body, bubble-only-on-header)
- D86: Delayed hover-reveal collapse by 300ms on initial load to prevent stutter
- D87: Always prefer local artwork when files exist — removed artwork_source gate
- D88: Removed overflow:hidden from controls-reveal — was clipping border-radius on unhover
- D89: Gated hover-reveal behind pageSettled state — bar stays expanded until episodes load
- D91: Hid episode count subtitle in hero while episodes loading
- D92: Stored overview on ShowMapping model, added DB migration + backfill from sonarr_series_cache. Skip Sonarr series fetch when local artwork exists. D92b: fall back to series.overview on first load before backfill.

**Outcome:** Kept. Show detail page actions bar now smoothly reveals on hover without stutter, artwork always prefers local files, overview is cached in DB eliminating a Sonarr API call on page load.

## Iterations D93-D98 — Show detail page: parallel fetch, episode count badge, air dates, monitor tooltip

**Targets:** `ShowDetailPage.tsx`, `index.css`, `models.go`, `database.go`, `sync.go`, `router.go`, `types.ts`, `api.ts`

**Summary:**
- D93: Removed Sonarr series fetch from detail page — backfill overview from cache instead
- D94: Parallelized show + episodes fetch via Promise.all — eliminated sequential waterfall
- D95: Removed dead settings fetch from detail page (was only used for Sonarr image fallback, now gone)
- D96: Moved episode count to a gray Badge pill next to the title in an HStack (D96b: tightened heading line-height, D96c: forced align-items center for vertical centering)
- D97: Added AirDate field to EpisodeInfo model + DB migration. Populated in processSeries and fetchEpisodesLive. Displayed in episode list. D97b: Used Astryx Timestamp component instead of raw string. D97c: Moved air date into description line. D97d: Tried right side before badge, reverted to description line.
- D98: Wrapped monitor ToggleButton in Tooltip with placement="start" (left) instead of using button's tooltip prop (which hardcoded "above"). D98b: Removed blue accent color from pressedIcon.

**Outcome:** Kept. Detail page now makes 1 API call (episodes only) instead of 3 (series + episodes + settings). Episode count is a clean badge next to title. Air dates shown via Astryx Timestamp. Monitor toggle tooltip appears on the left.

## Iterations D99-D103 — Show detail page: bottom rendering fix, Section padding normalization

**Targets:** `index.css`, `ShowDetailPage.tsx`

**Summary:**
- D99: Changed .controls-reveal default from grid-template-rows:1fr/overflow:visible to 0fr/overflow:hidden — prevented actions bar background from bleeding as translucent bar at bottom
- D100: Fixed bottom rendering issue — overrode LayoutContent's height:100% + overflow:clip on #astryx-app-shell-main (these prevented the AppShell root from growing past 100dvh in auto mode, causing body background to show below clipped content)
- D101-D101d: Added bottom padding to main content area, iterated on values (spacing-6 → spacing-4 → 6px → scoped to show-content-padded only)
- D102-D102d: Increased hero section top margin to spacing-6, iterated on bottom padding values
- D103: Switched both ShowDetailPage Sections to padding={4} like other pages. Removed .show-hero-section and .show-content-padded CSS hacks. Hero escapes Section padding via negative margins on .show-shell.

**Outcome:** Kept. Bottom rendering issue fixed (no more black bar / clipped content). Show detail page now uses same Section padding as all other pages — consistent spacing via Astryx's system.

## Iterations D104-D106 — VStack gap unification, hero header fixes

**Targets:** `ActivityPage.tsx`, `ShowsPage.tsx`, `SettingsPage.tsx`, `ShowDetailPage.tsx`, `index.css`, `main.tsx`

**Summary:**
- D104: Tried removing explicit VStack gap props to use defaults — reverted immediately (VStack default gap is null/0px, not 3 as assumed)
- D105: Unified Settings VStack gap from 8 to 6, matching Activity/Shows/ShowDetail (all now gap={6} = 24px)
- D106 (user changes): Fixed hero header, replaced tilde text logo with Mizu wave SVG, removed nav underline indicator CSS, tweaked actions bar margin for seamless corner transition

**Outcome:** Kept. All pages now use consistent VStack gap={6}. Mizu has a proper wave logo. Nav uses Astryx's built-in selected state instead of custom underline.

## Iterations D108-D110 — Transition token harmonization + micro-interactions

**Targets:** `index.css`

**Summary:**
- D108: Replaced raw `ease`/`0.18s`/`0.3s` timings on library cards with `var(--duration-fast)` / `var(--duration-medium)` / `var(--ease-standard)` tokens
- D109: Replaced all remaining raw `ease-out` values across the CSS with `var(--ease-standard)` — fade-in-up animations, page-enter, success-pulse, controls-reveal transitions, episode-list-wrap, activity-details-expanded, wizard-step-content
- D110: Replaced hero image opacity transition `0.4s ease` with `var(--duration-medium) var(--ease-standard)`

**Outcome:** Kept. All transitions and animations in the app now use Astryx design tokens consistently. No raw timing values remain except intentional `transition-delay: 300ms` for the hover-reveal interaction (which is a specific delay, not a duration).

## Iterations D112-D114 — Logo centering, instant hero image, badge alignment

**Targets:** `main.tsx`, `store.tsx`, `ShowDetailPage.tsx`, `index.css`

**Summary:**
- D112: Centered Mizu wave logo SVG — shifted paths from x=3.75 to x=3, y=9 to y=8 to center the wave within the 24×24 viewBox
- D113: Instant hero image on ShowDetailPage — backdropUrl memo now falls back to store `shows` array for artwork URL before getShow resolves. When navigating from Library, the image appears immediately.
- D114: Fixed two issues:
  - Hero image slow on direct navigation/refresh: moved shows + settings fetch to AppProvider (store.tsx) so the store is populated app-wide on mount, not just when ShowsPage loads. ShowDetailPage's backdropUrl fallback now works even on refresh.
  - Episodes badge not vertically aligned with title: changed h1 line-height from 1.1 to 1 in .show-hero-titles — tightens the line box to exactly match the font size so flex align-items:center centers the visible text, not the inflated line box.

**Outcome:** Kept. Hero image now loads instantly on both navigation and refresh. Badge pill is vertically centered with the title text.

## Iteration D115 — TrackingRuleCard: whole card clickable via SelectableCard

**Target:** `TrackingRuleCard.tsx` (used on both SettingsPage and ShowDetailPage)

**What changed:** Replaced the Card + Switch composition with Astryx's SelectableCard. The entire card surface is now clickable to toggle — no more aiming for the small switch. SelectableCard provides:
- Built-in checkbox semantics (hidden `<input type="checkbox">`)
- Visual selection indicator (inset box-shadow ring in the variant color)
- Hover/pressed/focus states via the design system

Removed the Switch component and the `accentVars` CSS custom property mapping that was needed to theme it. Added a small checkmark icon in the top-right corner as a secondary visual indicator when selected. Removed the now-unused `.tracking-rule-label` CSS class.

**Why:** The Switch was a small click target inside a larger card. Users naturally click the card expecting it to toggle, but only the switch worked. SelectableCard is Astryx's intended component for exactly this pattern (the ClickableCard source even says "For toggle selection, use SelectableCard").

**Compared against:** Card + Switch (previous), ClickableCard (navigation, not toggle).

**Outcome:** Kept. Cleaner, more accessible, whole-card click area. Applies to both SettingsPage defaults and ShowDetailPage tracking rules.

## Iterations D116-D116b — Section header cleanup + TrackingRuleCard color indicator

**Targets:** `SettingsPage.tsx`, `ShowDetailPage.tsx`, `AddAnimeDialog.tsx`, `TrackingRuleCard.tsx`, `index.css`

**Summary:**
- D116: Removed `.settings-section-header` CSS hack (font-size: 1.05em, letter-spacing, padding-bottom). These overrides fought the Heading component's design system sizing. Replaced `VStack gap={0}` + padding-bottom hack with `VStack gap={2}` for natural spacing between heading, divider, and rows. Removed className from all 8 headings across SettingsPage, ShowDetailPage, and AddAnimeDialog.
- D116b: TrackingRuleCard checkmark now always visible — colored with the variant's border token (green/yellow/red) at full opacity when selected, dimmed to 0.3 when unselected. Previously the indicator only appeared when selected, leaving unselected cards with no variant color identity. Added smooth opacity transition.

**Outcome:** Kept. Section headings now use pure Astryx Heading sizing. Tracking rule cards always show their variant color, making the three card types visually distinct even when unselected.

## Iteration D117 — Settings row spacing via VStack gap instead of padding hack

**Target:** `SettingsPage.tsx`, `index.css`

**What changed:** Removed `.settings-row { padding-block: var(--spacing-4) }` CSS hack. Each settings row was adding 16px top + 16px bottom padding on top of the VStack gap, creating inconsistent spacing between dividers and rows. Changed the three settings sections (Connection, Sync, Notifications) from `VStack gap={2}` to `VStack gap={3}` (12px) for uniform spacing between all children — headings, dividers, and rows.

**Why:** The padding hack made spacing depend on which element was adjacent (row→divider had different gap than heading→divider). Using VStack gap alone makes all spacing uniform and controlled by the design system's spacing scale.

**Compared against:** Manual padding-block on each row (previous), gap={0} + padding (original).

**Outcome:** Kept. Uniform spacing throughout settings sections. One less CSS hack.

## Iteration D118 — AddAnimeDialog: TrackingRuleCard replaces plain Switches

**Target:** `AddAnimeDialog.tsx`

**What changed:** Replaced three plain Switch components (Track filler / Track mixed canon / Track anime canon) with TrackingRuleCard in a Grid layout, matching the pattern used on SettingsPage and ShowDetailPage. Removed unused Switch import.

**Why:** The add-anime wizard's tracking rules were the only place using plain Switches instead of the SelectableCard-based TrackingRuleCard. Now all three screens that show tracking rules (Settings defaults, ShowDetailPage per-anime, AddAnimeDialog wizard) use the same component with whole-card click, variant colors, and checkmark indicator.

**Compared against:** Plain Switch components (previous).

**Outcome:** Kept. Consistent tracking rule UI across all three screens.

## Iteration D119 — Revert TrackingRuleCard to Card + Switch with whole-card click

**Target:** `TrackingRuleCard.tsx`, `index.css`

**What changed:** Reverted from SelectableCard (checkmark + selection ring) back to Card + Switch. The Switch is the stronger on/off affordance for "track this episode type" — users immediately understand it's a toggle. The whole card is now clickable: onClick on the Card calls onChange(!value), but clicks on the Switch's input are filtered out to prevent double-toggle. Restored the accentVars CSS custom property mapping so the Switch track takes the variant's color when on (green/yellow/red instead of default accent). Added .tracking-rule-card to the hover border-emphasis CSS group.

**Why:** The SelectableCard approach (D115) removed the Switch in favor of a checkmark + ring. While technically correct, the Switch is a better visual metaphor for "enable/disable tracking." The user's original request was "the whole card should be clickable" — not "replace the switch." This keeps the Switch visual while achieving the whole-card click target.

**Compared against:** SelectableCard with checkmark (D115-D116b, reverted), plain Switch without card click (original).

**Outcome:** Kept. Best of both — familiar Switch toggle + whole-card click area + variant-colored switch track.

## Iteration D120 — Fix Switch off-state variant color + library card fade speed

**Targets:** `TrackingRuleCard.tsx`, `index.css`

**Summary:**
- Moved accentVars CSS custom properties from Card's style to Switch's style. The Switch's off-state track uses `--color-background-gray` and on-state uses `--color-accent` — these are read by StyleX at the component level, so they must be on the Switch's own DOM element, not a parent. Previously they were on the Card and didn't cascade, leaving all switches grey when off. Now each variant's switch track is tinted green/yellow/red even when off.
- Sped up library card image fade-in from `--duration-medium` (410ms) to `--duration-fast` (175ms) for snappier image appearance on scroll.

**Outcome:** Kept. Switch tracks now show variant colors in both on and off states.

## Iteration D121 — Fix Switch track variant colors via CSS targeting

**Target:** `TrackingRuleCard.tsx`, `index.css`

**What changed:** The previous approach (D120) tried to override StyleX theme tokens (`--color-background-gray`, `--color-accent`) via inline `style` on the Switch. This didn't work because StyleX compiles theme tokens to hashed CSS variable names — setting the raw name in a style prop doesn't override the generated one.

New approach: use CSS selectors targeting the Switch's track element directly. The Card outputs `data-variant="green/yellow/red"` and the Switch track has class `astryx-switch` with `data-checked` when on. Six CSS rules override the track background:
- Off state: `var(--color-background-green/yellow/red)` — tinted with the variant color
- On state: `var(--color-border-green/yellow/red)` — full variant color

Removed the dead `accentVars` object and `CSSProperties` import from the component.

**Why:** The user wanted the switch track to show the card's variant color in both on and off states, not the default grey/accent.

**Outcome:** Kept. Switch tracks now show variant colors in both states.

## Iteration D122 — Fix tracking rule toggle lag (double-fire)

**Target:** `TrackingRuleCard.tsx`

**What changed:** Wrapped the Switch in a `div` with `onClick={e => e.stopPropagation()}`. Previously, clicking the Switch fired both the Switch's `onChange` AND the Card's `onClick` (DOM click events bubble from the input up to the card). The `handleCardClick` guard (`target.closest("input")`) was unreliable — the double `onChange` call caused unnecessary re-renders and lag, especially on ShowDetailPage where `rulesDirty` recomputes on every render.

**Why:** User reported "insanely laggy" toggling on tracking rules vs. fine on settings. The settings page uses plain Switch (no card click handler), so no double-fire. ShowDetailPage and AddAnimeDialog use TrackingRuleCard with the card click handler.

**Outcome:** Kept. Switch clicks are now isolated from the card click handler.

## Iteration D123 — Fix wrong token names + replace fake token refs with real values

**Target:** `index.css`

**What changed:**
- Fixed `--color-text-on-accent` → `--color-on-accent` (3 occurrences). The token name was wrong — Astryx defines `--color-on-accent`, not `--color-text-on-accent`. The `#fff` fallback was always being used instead of the actual theme token.
- Replaced `var(--text-body-size, 15px)` → `14px` and `var(--text-supporting-size, 13px)` → `13px` (3 occurrences). These StyleX tokens compile to hashed variable names, so the `var()` references never resolved — the fallback values were always used. Using raw values directly is more honest and avoids implying token integration that doesn't exist.
- Replaced `padding: 2px 8px` → `var(--spacing-0-5) var(--spacing-2)` on `.season-episode-count` (token-based spacing).
- Replaced `margin-top: 2px` → `var(--spacing-0-5)` on `.wizard-step-description-icon`.

**Why:** Wrong token names silently fall back to defaults, defeating the purpose of referencing tokens. Fake `var()` references to StyleX tokens that compile to hashed names are misleading.

**Outcome:** Kept. Color tokens now resolve correctly. Font sizes are explicit instead of pretending to use tokens.

## Iteration D124 — Audit CSS token references (no fixes needed)

**Target:** `web-astryx/src/index.css` — all `var(--*)` references

**What changed:** Nothing — audit confirmed all tokens are valid.

**Investigation:** The built `@astryxdesign/core/astryx.css` (imported via `main.tsx`) defines all tokens at `:root` in a single massive declaration. Verified every `var(--*)` reference in our `index.css` against the built CSS:
- All `--color-*` tokens (accent, neutral, text-primary/secondary/disabled, border, border-emphasized, background-surface/body/muted, overlay-hover/pressed, success/error/warning, on-accent/on-success) ✓
- All categorical tokens (background-blue/cyan/gray/green/orange/pink/purple/red/teal/yellow, border-blue/cyan/gray/green/orange/pink/purple/red/teal/yellow) ✓
- All spacing tokens (spacing-0-5 through spacing-6) ✓
- All duration/ease/radius/shadow tokens ✓

**Why:** Needed to confirm that our CSS token references actually resolve at runtime, since StyleX compiles some tokens to hashed names. The built `astryx.css` is a pre-compiled artifact that exports the raw token names at `:root`, so all our references work correctly.

**Outcome:** No changes needed. All 40+ token references in index.css are valid.

## Iteration D125 — Fix tracking rule toggle lag on ShowDetailPage (memoize episode list)

**Target:** `web-astryx/src/routes/ShowDetailPage.tsx`

**What changed:**
- Extracted the episode list section (filters + season groups + episode items) into a `EpisodeListSection` component wrapped in `React.memo`. This component receives props: `episodesLoading`, `filteredEpisodes`, `groupedBySeason`, `epStats`, `activeTypeFilter`, `isMobile`, `syncingEps`, `onFilterChange`, `onToggleMonitor`.
- Wrapped `toggleEpisodeMonitor` in `useCallback` with deps `[showId, addNotification]` so the callback identity is stable across tracking-rule state changes.
- `setActiveTypeFilter` is a useState setter — already stable.

**Why:** The user reported tracking rule toggles feel "insanely laggy" on ShowDetailPage but fine on SettingsPage. Root cause: ShowDetailPage is a 920-line monolith with ~20 state variables and a potentially 1200+ item episode list. Toggling a tracking rule (e.g. `setTrackFiller`) triggers a full re-render of the entire component, including re-creating every ListItem in the episode list. SettingsPage has no such list, so its toggles are instant. By memoizing the episode list, tracking-rule state changes no longer trigger episode list re-renders — React skips it entirely since none of its props changed.

**Outcome:** Kept. Tracking rule toggles should now be as snappy as SettingsPage. tsc + go build pass.

## Iteration D126 — Clean up test button variant hacks

**Target:** `SettingsPage.tsx` (test connection + test ntfy buttons), `index.css`

**What changed:**
- Test buttons (Sonarr + ntfy) no longer switch to `variant="primary"` for success state. Instead, they stay `variant="secondary"` and convey success via a green Check icon (`color="var(--color-success)"`) + "Connected"/"Sent" label text.
- Removed the `.test-status-btn[data-variant="primary"]` CSS rule that used `!important` to override the blue primary variant to green. This was fighting the StyleX-compiled variant styles.
- Error state still uses `variant="destructive"` (red) — no override needed there.
- The `.test-status-btn` class now only suppresses the pointer cursor (`cursor: default`), which is all it needs.

**Why:** The `!important` CSS hack to recolor primary buttons green was a workaround for using the wrong variant. Success doesn't require a color change — the icon + label communicate it clearly. This is more consistent with the Astryx variant system: secondary for neutral/idle, destructive for errors, and icon color for success indication.

**Outcome:** Kept. tsc passes. Removed 5 lines of `!important` CSS. Test buttons now use the variant system cleanly.

## Iteration D127 — Clean up save-flash !important hack

**Target:** `index.css` (save-flash rule), `SettingsPage.tsx`, `ShowDetailPage.tsx`

**What changed:**
- Replaced the `.save-flash[data-variant="primary"]` CSS that used `!important` to override the button's background/border/color to green. Now uses a subtle green box-shadow ring (`0 0 0 2px` with 40% opacity success color) that doesn't fight the StyleX-compiled variant background.
- Made the Check icon green (`color="var(--color-success)"`) on both save buttons (SettingsPage + ShowDetailPage) when in the saved-flash state, providing an additional color cue.
- The label change ("Saving..." → "Saved") + icon swap (Save → green Check) + subtle green ring together communicate success without hijacking the button's variant.

**Why:** Same principle as D126 — `!important` overrides on StyleX-compiled variant styles are fragile and fight the design system. The box-shadow approach is non-destructive: it adds a visual cue on top of the existing variant styling rather than replacing it.

**Outcome:** Kept. tsc passes. Removed 3 `!important` declarations. Save button now stays primary (blue) with a green ring + green check icon for the 1.5s confirmation flash.

## Iteration D128 — Remove redundant !important from hero titles alignment

**Target:** `index.css` — `.show-hero-titles > div:first-child`

**What changed:**
- Removed `.show-hero-titles > div:first-child { align-items: center !important }` — the HStack inside `.show-hero-titles` already sets `align="center"` via its prop, which StyleX compiles to `align-items: center`. The `!important` CSS was a redundant belt-and-suspenders override.

**Why:** Audited all remaining `!important` declarations in index.css. Most are legitimate (AppShell header background, LayoutContent height/overflow, Switch track colors, mobile responsive overrides). This one was redundant — the component prop already handles it.

**Remaining !important declarations (all legitimate):**
- `.astryx-app-shell-header` background — overrides StyleX AppShell header
- `#astryx-app-shell-main` height/overflow — overrides StyleX LayoutContent
- `.show-actions-bar` mobile responsive — overrides desktop styles at <600px
- `.tracking-rule-card` Switch track colors — overrides StyleX Switch track (6 declarations)
- `.settings-row` mobile input width — overrides fixed-width inputs at <600px

**Outcome:** Kept. tsc passes. Removed 1 redundant `!important` declaration.

## Iteration D129 — Deduplicate episode filter SegmentedControl + remove unused isMobile prop

**Target:** `ShowDetailPage.tsx` — `EpisodeListSection` component

**What changed:**
- Extracted the duplicated `SegmentedControlItem` list into a shared `filterItems` variable. Previously the same 5 items (All, Manga, Anime, Mixed, Filler) were copy-pasted for both `episode-filters-desktop` and `episode-filters-mobile` wrappers — ~30 lines of pure duplication.
- Removed the `isMobile` prop from `EpisodeListSection` — it was destructured but never used inside the component. The desktop/mobile visibility is handled entirely by CSS media queries on the wrapper divs. `isMobile` is still used in the parent `ShowDetailPage` for the `filteredEpisodes` memo and the mobile filter fallback effect.

**Why:** Code duplication is a maintenance burden — if a filter item is added/changed, you'd need to edit two places. The unused `isMobile` prop was misleading (implied the component had responsive logic when it doesn't).

**Outcome:** Kept. tsc passes. Reduced EpisodeListSection by ~20 lines of duplicated JSX.

## Iteration D130 — Fix hero title line-height causing badge misalignment

**Target:** `index.css` — `.show-hero-titles h1` rule

**What changed:**
- Removed `line-height: 1` from `.show-hero-titles h1, .show-hero-titles [data-role="heading"]`. The `line-height: 1` was making the h1's content box tighter than the Badge's box, causing visual misalignment between the title text and the episode count badge even though the HStack had `align="center"`. With the default line-height from Astryx's Heading component, the h1's box height better matches the Badge's, so flexbox centering aligns them properly.

**Why:** The user reported the episodes badge pill was not vertically aligned with the title text. The `line-height: 1` override was shrinking the h1's box height below the Badge's natural height, making `align-items: center` center unequal-height boxes — the Badge appeared shifted relative to the text.

**Outcome:** Kept. tsc passes. The heading now uses its natural line-height, allowing the HStack's `align="center"` to properly vertically center the title and badge.

## Iteration D131 — Add visual distinction for success/warning toasts

**Target:** `ToastViewport.tsx`, `index.css`

**What changed:**
- Wrapped each Toast in a `<div data-notification-type={n.type}>` wrapper in ToastViewport. Previously the notification type (success/warning/error/info) was discarded — everything non-error rendered as a generic info toast.
- Added CSS overrides for success toasts (green background via `--color-success`) and warning toasts (yellow background via `--color-warning` with `--color-on-warning` text color). Error toasts already use Astryx's built-in error styling.
- The `!important` is needed because StyleX compiles the Toast's `background-color` with high specificity — same pattern as the other legitimate `!important` overrides.

**Why:** The app uses `addNotification("Saved", "success")` and `addNotification("...", "warning")` extensively, but all non-error toasts looked identical (dark inverted surface). Success and warning notifications should be visually distinct so users can quickly recognize the outcome without reading the text.

**Outcome:** Kept. tsc passes. Success toasts are now green, warning toasts are yellow, error toasts are red (Astryx built-in), info toasts are dark (Astryx default).

## Iteration D132 — Fix semantic color tokens for text on images

**Target:** `index.css` — `.mizu-card-overlay-title`, `.show-hero-content`

**What changed:**
- `.mizu-card-overlay-title`: Changed `color: var(--color-on-accent)` → `color: var(--color-on-dark)`. The overlay title is white text on a card image, not on an accent-colored background. `--color-on-accent` is semantically "text on accent color" while `--color-on-dark` is "text on dark surface" — the card image with its gradient overlay is always dark, so `--color-on-dark` is correct. Both resolve to white in light mode, but in dark mode `--color-on-accent` could theoretically differ if a theme changes the accent color.
- `.show-hero-content`: Added `color: var(--color-on-dark)` explicitly. Previously the hero text inherited `--color-text-primary` from the body, which is dark in light mode and light in dark mode. But the hero text always sits on a darkened image area (gradient overlay to `--color-background-body`), so it should always be white/light regardless of theme mode. Without this, in light mode the hero title could be dark text on a dark image area.

**Why:** Semantic token correctness. Text on images should use `--color-on-dark` (always light) since the image area is always darkened by the gradient overlay, regardless of theme mode. Using `--color-on-accent` or inheriting `--color-text-primary` is semantically wrong and could cause readability issues with certain themes or images.

**Outcome:** Kept. tsc passes. Card overlay titles and hero content text now use the correct semantic token for text on dark surfaces.

## Iteration D133 — Add skeleton shimmer to image placeholders

**Target:** `index.css` — `.mizu-card-img-placeholder`, `.wizard-poster-placeholder`

**What changed:**
- Added a `skeleton-pulse` keyframe animation (opacity 0.6 ↔ 1.0) to both image placeholder classes. The pulse uses `--duration-medium` (410ms) with `alternate infinite` for a subtle breathing effect while images load.
- Added both placeholders to the `prefers-reduced-motion: reduce` override list to disable the animation for accessibility.
- The animation matches the Astryx Skeleton component's approach (opacity pulse) but is applied via CSS to our existing placeholder elements rather than replacing them with Skeleton components — simpler, no component changes needed.

**Why:** Static gradient placeholders look frozen/lifeless while images load. A subtle pulse animation communicates "loading" to the user and feels more polished. This aligns with the Astryx Skeleton component's pattern (which uses the same opacity pulse technique).

**Outcome:** Kept. tsc passes. Card image and wizard poster placeholders now have a subtle pulse animation while loading, disabled under reduced-motion.

## Iteration D134 — Add press feedback to tracking rule cards and activity log summary

**Target:** `index.css` — `.tracking-rule-card:active`, `.activity-log-summary[role="button"]:active`

**What changed:**
- `.tracking-rule-card:active`: Added `transform: scale(0.99)` with `transition: transform var(--duration-fast) var(--ease-standard)`. When the user presses/taps the tracking rule card (which is clickable as a whole), the card slightly shrinks to give tactile press feedback.
- `.activity-log-summary[role="button"]:active`: Added `background-color: var(--color-overlay-pressed)`. When the user presses the expandable activity log summary row, it gets the Astryx pressed overlay color for consistent press feedback.

**Why:** Micro-interaction polish. Clickable elements should give immediate tactile feedback on press, not just on hover. The tracking rule cards have hover states but no press state — adding a subtle scale-down on press makes them feel more responsive and physical. The activity log summary rows similarly had hover/focus states but no press state. Using `--color-overlay-pressed` matches Astryx's own pressed-state pattern.

**Outcome:** Kept. tsc passes. Tracking rule cards now scale down slightly on press, activity log summary rows get pressed overlay color on press.

## Iteration D135 — Add hover/press feedback to Collapsible triggers and SegmentedControl items

**Target:** `index.css` — `.astryx-collapsible > button`, `.astryx-segmented-control-item:not(.selected):not([disabled]):active`

**What changed:**
- `.astryx-collapsible > button`: Added `transition: background-color var(--duration-fast) var(--ease-standard)` and `border-radius: var(--radius-inner)`. Added `:hover` state with `background-color: var(--color-overlay-hover)` (guarded by `@media (hover: hover)` to avoid sticky hover on touch devices). Added `:active` state with `background-color: var(--color-overlay-pressed)`. The Astryx Collapsible trigger button has `all: unset` which strips all native button styling — it has a focus-visible outline (we already added that in an earlier iteration) but no hover or press feedback at all.
- `.astryx-segmented-control-item:not(.selected):not([disabled]):active`: Added `background-color: var(--color-overlay-pressed)`. The Astryx SegmentedControlItem has a hover state (overlay-hover) but no active/press state. Only applies to unselected, non-disabled items — selected items have their own background from the colored filter overrides.

**Why:** Micro-interaction consistency. The Collapsible trigger is used for season rows in the episode list — clicking them to expand/collapse should feel responsive. Without any hover or press feedback, the trigger feels dead/static. The SegmentedControl items (episode type filter, activity status filter) similarly feel unresponsive on press without an active state. Using `--color-overlay-hover` and `--color-overlay-pressed` matches Astryx's own interaction state tokens.

**Outcome:** Kept. tsc passes. Collapsible triggers now have hover background + press feedback, SegmentedControl items have press feedback. Both use Astryx's standard overlay tokens for visual consistency.

## Iteration D136 — Add reduced-motion overrides for new transitions + improve hero placeholder

**Target:** `index.css` — `@media (prefers-reduced-motion: reduce)`, `.show-hero-placeholder`

**What changed:**
- Added `.astryx-collapsible > button` and `.tracking-rule-card` to the `prefers-reduced-motion: reduce` transition-none list. These elements got new transitions in D134/D135 (background-color transition on Collapsible, transform transition on tracking rule cards) but were missing from the reduced-motion override. Users with reduced motion preference will now see instant state changes instead of animated transitions on these elements.
- Improved `.show-hero-placeholder`: Added `display: flex; align-items: center; justify-content: center` and a `::after` pseudo-element showing a large faint tilde (`~`) — the app's logo character. The placeholder was previously just a flat gradient; now it has a subtle centered brand mark that makes the no-backdrop state feel intentional rather than broken/empty. The tilde uses `--color-icon-secondary` at 40% opacity so it's subtle.

**Why:** Accessibility compliance — all transitions we add must be disabled under reduced motion. The hero placeholder improvement makes the no-backdrop-image fallback state look more polished and on-brand, matching the tilde logo used in the TopNav heading.

**Outcome:** Kept. tsc passes. Reduced-motion users get instant state changes on Collapsible triggers and tracking rule cards. Hero placeholder now shows a subtle centered tilde brand mark.

## Iteration D137 — Simplify hero content nesting + fix overview text color on dark image

**Target:** `ShowDetailPage.tsx` — hero content JSX, `index.css` — `.show-hero-overview`

**What changed:**
- Removed redundant outer `VStack gap={4}` wrapper in `.show-hero-content` that contained only one child (the inner `VStack gap={1} className="show-hero-titles"`). The outer VStack added an unnecessary DOM wrapper with no layout effect since it had only one child.
- Added `color: color-mix(in srgb, var(--color-on-dark) 75%, transparent)` to `.show-hero-overview`. The Text component uses `color="secondary"` which maps to `--color-text-secondary` — in light mode this is a medium-dark gray that's hard to read on a darkened image. The overview text should be a slightly dimmed version of the hero's light text (`--color-on-dark` at 75% opacity) to maintain hierarchy while staying readable on the gradient-overlaid image.

**Why:** Code cleanliness (removing dead wrapper) and readability fix. The overview text sits on the same darkened image area as the title, so it should use a light color derived from `--color-on-dark` rather than `--color-text-secondary` which is designed for regular page backgrounds, not image overlays.

**Outcome:** Kept. tsc passes. Hero content has one less unnecessary wrapper. Overview text is now a readable light color at 75% opacity on the dark image gradient.

## Iteration D138 — Fix hero heading color override for light mode readability

**Target:** `index.css` — `.show-hero-content h1`

**What changed:**
- Added `.show-hero-content h1 { color: var(--color-on-dark) !important; }` to override the Astryx Heading component's StyleX `color="primary"` (which maps to `--color-text-primary`). In light mode, `--color-text-primary` is dark (e.g., `#100F0F` in Flexoki) — dark text on a darkened image gradient is unreadable. The `!important` is needed because StyleX compiles the Heading's color with high specificity via hashed CSS variables.

**Why:** The `.show-hero-content` container sets `color: var(--color-on-dark)` (always light), but the Heading component overrides this with its own `colorStyles['primary']` which resolves to `--color-text-primary` (dark in light mode). Without this override, the hero title would be dark text on a dark image in light mode — a critical readability bug. This is the same pattern as the `.show-hero-overview` color fix from D137, but for the heading element which has a stronger StyleX override.

**Outcome:** Kept. tsc passes. Hero title is now always light text (`--color-on-dark`) on the darkened image, regardless of theme mode.

## Iteration D139 — Fix light-mode readability of text on image overlays

**Target:** `index.css` — `.show-hero-gradient`, `.show-hero-content h1/.show-hero-overview` text-shadow, `.mizu-card-img-wrap::after`, `.mizu-card-overlay-title` text-shadow

**What changed:**
- `.show-hero-gradient`: Changed gradient from `color-mix(in srgb, var(--color-background-body) ...%)` to `color-mix(in srgb, #000 ...%)`. The gradient overlays the hero backdrop image, fading from 75% black at the bottom to 8% black at the top. Previously used `--color-background-body` which is light in light mode (e.g., `#F2F0E5` in Flexoki) — a light gradient at the bottom made white hero text (`--color-on-dark`) unreadable in light mode.
- `.show-hero-content h1, .show-hero-overview` text-shadow: Changed from `color-mix(in srgb, var(--color-background-body) 60%, transparent)` to `color-mix(in srgb, #000 60%, transparent)`. Dark text-shadow ensures readability on the image regardless of theme mode.
- `.mizu-card-img-wrap::after`: Same fix — gradient changed from `--color-background-body` to `#000`. The card overlay title is white (`--color-on-dark`), so the gradient at the bottom of card images must be dark in both theme modes.
- `.mizu-card-overlay-title` text-shadow: Changed from `color-mix(in srgb, var(--color-background-body) 70%, transparent)` to `color-mix(in srgb, #000 70%, transparent)`.

**Why:** Critical light-mode readability bug. All text on image overlays (hero title, hero overview, card overlay titles) uses `--color-on-dark` (always white/light). The gradient overlays and text-shadows were using `--color-background-body` which is light in light mode — making white text on a light gradient unreadable. Using `#000` instead ensures the gradient always darkens the image at the bottom where text sits, and the text-shadow always provides a dark halo around white text, in both light and dark modes.

**Outcome:** Kept. tsc passes. Hero and card text on image overlays is now readable in both light and dark mode. Gradients use black-based overlays instead of theme-dependent background color.

## Iteration D140 — Fix hero placeholder text readability in light mode

**Target:** `index.css` — `.show-hero-placeholder`

**What changed:**
- Added a black-based darkening gradient as the first layer in the `background` shorthand: `linear-gradient(to top, color-mix(in srgb, #000 60%, transparent) 0%, color-mix(in srgb, #000 20%, transparent) 50%, transparent 100%)`. This sits on top of the existing decorative gradient (accent-tinted muted → surface) and darkens the bottom area where the hero title/overview text sits.
- The decorative gradient is preserved as the second background layer — it shows through where the darkening gradient is transparent (upper area).
- The tilde `::after` brand mark remains visible in the upper-center area where the darkening gradient fades to transparent.

**Why:** Same readability bug as D139 but for the no-backdrop-image fallback state. The hero text (`--color-on-dark`, always white) sits at the bottom of the placeholder. In light mode, the placeholder's decorative gradient uses light theme colors (`--color-background-muted`, `--color-background-surface`) — white text on a light gradient is unreadable. Adding a black-based darkening gradient at the bottom ensures the text area is always dark enough for white text, matching the pattern used for the hero image gradient in D139.

**Outcome:** Kept. tsc passes. Hero placeholder now has a dark bottom gradient for text readability in both light and dark mode, with the decorative accent gradient and tilde brand mark visible in the upper area.

## Iteration D141 — Fix missing SegmentedControlItem closing tag + add drop-shadow to card logos

**Target:** `ShowDetailPage.tsx` — `filterItems` JSX, `index.css` — `.mizu-card-logo`

**What changed:**
- Fixed a JSX syntax bug in `filterItems`: the last `<SegmentedControlItem>` was missing its `/>` self-closing tag. The `icon={<SkipForward size={16} />}` prop's `/>` was being misread as closing the parent element. Added the missing `/>` before `</>`. This was caught by the Vite oxc parser (which is stricter than tsc) and caused a parse error that broke the dev server.
- Added `filter: drop-shadow(0 1px 4px color-mix(in srgb, #000 60%, transparent))` to `.mizu-card-logo`. Anime logo images are often light-colored and can blend into light areas of the card image. A subtle dark drop-shadow ensures the logo is visible regardless of the underlying image content, matching the text-shadow pattern used for `.mizu-card-overlay-title`.

**Why:** The JSX bug was a real syntax error that broke the dev server. The drop-shadow improves logo readability on card images — logos sit in the same overlay area as the title text (which already has a text-shadow), so they should have equivalent visual treatment.

**Outcome:** Kept. tsc passes. Vite parse error resolved. Card logos now have a subtle drop-shadow for readability on any image background.

## Iteration D142 — Guard card hover states with @media (hover: hover) for touch devices

**Target:** `index.css` — `.mizu-card:hover`, `.mizu-card:hover .mizu-card-img`, `.mizu-card:hover .mizu-card-img-wrap::after`

**What changed:**
- Wrapped `.mizu-card:hover { transform: translateY(-2px); box-shadow: var(--shadow-med); }` in `@media (hover: hover)` block. Previously unguarded — on touch devices, tapping a card could leave it stuck in the hover state (elevated with medium shadow) after the tap ends.
- Wrapped `.mizu-card:hover .mizu-card-img { transform: scale(1.04); }` in `@media (hover: hover)` block. Same issue — image zoom could get stuck on touch devices.
- Wrapped `.mizu-card:hover .mizu-card-img-wrap::after { opacity: 0.7; }` in `@media (hover: hover)` block. Same issue — gradient overlay opacity could get stuck.
- The `:active` states (press feedback) remain unguarded since they work correctly on touch devices.
- The `prefers-reduced-motion` block's `.mizu-card:hover .mizu-card-img { transform: none }` still works correctly — it appears later in the file so it overrides the hover transform when both media queries match.

**Why:** Touch device compatibility. On touch devices, `:hover` states can get "stuck" after a tap — the browser applies hover on touchstart and doesn't always clear it on touchend. Guarding hover-only visual effects with `@media (hover: hover)` ensures they only apply on devices with a real hover capability (mouse/trackpad). This matches the pattern already used for Collapsible triggers, list items, activity log cards, and tracking rule cards.

**Outcome:** Kept. tsc passes. Card hover effects (lift, image zoom, gradient opacity) are now properly guarded for touch devices.

## Iteration D143 — Fix invalid Badge variant="gray" → use variant="neutral"

**Target:** `ShowDetailPage.tsx` — episode count Badge in hero header

**What changed:**
- Changed `<Badge variant="gray" label={`${episodes.length} episodes`} />` to `<Badge variant="neutral" label={`${episodes.length} episodes`} />`. The Astryx Badge component does not have a `gray` variant — its variant map includes `neutral`, `info`, `success`, `warning`, `error`, `blue`, `cyan`, `green`, `orange`, `pink`, `purple`, `red`, `teal`, `yellow` but no `gray`. When `variant="gray"` was passed, `variants['gray']` resolved to `undefined`, so the badge got no background color and no explicit text color — it rendered as unstyled text with just padding/border-radius from the base styles. Using `variant="neutral"` gives it `--color-neutral` background and `--color-text-primary` text color, which is the intended subtle pill appearance.

**Why:** Bug fix. The `gray` variant doesn't exist in Astryx's Badge component. The badge was silently rendering without any variant styles — no background pill, just floating text. `neutral` is the correct variant for a subtle, non-semantic badge.

**Outcome:** Kept. tsc passes. Episode count badge in hero now has a proper neutral background pill instead of unstyled text.

## Iteration D144 — Style hero episode badge for visibility on dark gradient

**Target:** `index.css` — `.show-hero-content .astryx-badge`

**What changed:**
- Added CSS override for `.show-hero-content .astryx-badge`: `background-color: color-mix(in srgb, #fff 15%, transparent)`, `color: var(--color-on-dark)`, `backdrop-filter: blur(4px)`. The Badge `variant="neutral"` uses `--color-neutral` which is a very subtle overlay (e.g., `#100F0F0F` in Flexoki light mode) — nearly invisible on the dark hero gradient. This override gives the badge a semi-transparent white frosted-glass pill that's visible on any image background, while keeping the text color consistent with the hero text (`--color-on-dark`).

**Why:** The neutral badge variant is designed for regular page backgrounds where its subtle overlay is sufficient. On the dark hero gradient, it disappears. A frosted glass effect (semi-transparent white + backdrop blur) is a common pattern for badges on images — it adapts to the underlying content while remaining visible.

**Outcome:** Kept. tsc passes. Hero episode count badge now has a visible frosted glass pill on the dark gradient overlay.

## Iteration D145 — Style neutral card overlay badge with frosted glass for image visibility

**Target:** `index.css` — `.mizu-card-overlay .astryx-badge[data-variant="neutral"]`

**What changed:**
- Added CSS override for neutral badges in card overlays: `background-color: color-mix(in srgb, #fff 15%, transparent)`, `color: var(--color-on-dark)`, `backdrop-filter: blur(4px)`. When a show is paused, its card badge uses `variant="neutral"` which has a very subtle `--color-neutral` background — nearly invisible on card images. This override gives it the same frosted glass treatment as the hero badge (D144). Only targets `data-variant="neutral"` so success badges (solid green) are unaffected.

**Why:** Consistency with D144. The neutral badge variant is too subtle on image backgrounds. Shows that are "Paused" need a visible badge on their card image, just like the episode count badge on the hero. The frosted glass pattern (semi-transparent white + backdrop blur) works on any image content.

**Outcome:** Kept. tsc passes. Paused-show badges on card images now have a visible frosted glass pill, matching the hero badge style.

## Iteration D146 — Add keyboard accessibility to TrackingRuleCard

**Target:** `TrackingRuleCard.tsx`, `index.css`

**What changed:**
- Added `role="button"`, `tabIndex={0}`, `aria-pressed={value}`, and `aria-label` to the TrackingRuleCard's root Card element so it's keyboard-focusable and announced as a toggle button by screen readers.
- Added `onKeyDown` handler: Enter and Space toggle the switch value. The handler checks if the event originated from the switch's internal input (via `event.target` check) to avoid double-firing when the switch itself handles the keypress.
- Added `.tracking-rule-card:focus-visible` to the shared focus-visible outline CSS rule (alongside `.astryx-list-item`, `.astryx-clickable-card`, `.astryx-collapsible > button`) so the card shows a consistent focus ring when navigated via keyboard.

**Why:** The TrackingRuleCard is fully clickable (clicking anywhere on the card toggles the switch), but it was only accessible via mouse. Keyboard users couldn't tab to it or toggle it. Adding `role="button"` + `tabIndex` + `onKeyDown` brings it to parity with other interactive Astryx elements. The `aria-pressed` attribute communicates the on/off state to assistive technology.

**Outcome:** Kept. tsc passes. TrackingRuleCard is now keyboard-accessible with a visible focus ring.

## Iteration D147 — Replace custom season-episode-count pill with Astryx Badge

**Target:** `ShowDetailPage.tsx`, `index.css`

**What changed:**
- Replaced the hand-styled `<span className="season-episode-count">` in the Collapsible trigger with an Astryx `<Badge variant="neutral" label="N episodes" />`. The custom CSS class `.season-episode-count` (8 lines: color, font-size, font-variant-numeric, padding, border-radius, background) was removed since Badge already provides all of these with consistent design tokens.

**Why:** The season episode count was the only pill/badge in the app using a hand-rolled CSS class instead of the Astryx Badge component. Every other count pill (hero episode count, card status badge, settings "Key set" badge) uses Badge. Using Badge here ensures consistent spacing, typography, border-radius, and color tokens across all pills in the app.

**Outcome:** Kept. tsc passes. Season collapsible triggers now use the same Badge component as the rest of the app.

## Iteration D148 — Replace wizard-step-description with Astryx Card + HStack

**Target:** `AddAnimeDialog.tsx`, `index.css`

**What changed:**
- Replaced the hand-styled `<div className="wizard-step-description">` (custom flex layout with muted background, border-radius, padding) with an Astryx `<Card variant="muted" padding={3}>` containing an `<HStack gap={2} align="start">` for the icon + text layout. This is exactly what Card `variant="muted"` is designed for — a muted background container with consistent padding and radius.
- Removed the `.wizard-step-description` CSS rule (8 lines: display, align-items, gap, padding, background-color, border-radius, color). Kept a minimal `.wizard-step-description-icon` rule for `flex-shrink: 0` and color since the icon still needs those overrides inside the HStack.

**Why:** The wizard step description was a hand-rolled muted container duplicating what Card `variant="muted"` already provides with design system tokens. Using Card ensures consistent background color, border-radius, and padding with other muted containers in the app (e.g., season cards, artwork card, activity log cards).

**Outcome:** Kept. tsc passes. Wizard step description now uses Astryx Card variant="muted" instead of custom CSS.

## Iteration D149 — Add fade-in transition to hero backdrop image

**Target:** `ShowDetailPage.tsx`, `index.css`

**What changed:**
- Added `opacity: 0` initial state + `transition: opacity var(--duration-medium) var(--ease-standard)` to `.show-hero-img`, with `.show-hero-img.is-loaded { opacity: 1 }` for the loaded state.
- Added `onLoad` handler to the hero `<img>` element that adds the `is-loaded` class — same pattern already used for card images (`.mizu-card-img`).

**Why:** The hero backdrop image appeared instantly with no transition, which felt jarring compared to the card images which fade in smoothly. Adding the same fade-in pattern creates a consistent loading experience across all images in the app. The `--duration-medium` (410ms) gives a gentle reveal that matches the hero's cinematic feel.

**Outcome:** Kept. tsc passes. Hero backdrop now fades in smoothly on load, matching the card image pattern.

## Iteration D150 — Fix tracking-rule-card press animation transition

**Target:** `index.css`

**What changed:**
- Moved the `transform` transition from the `:active` pseudo-state to the base `.tracking-rule-card` rule. Previously, `transition: transform` was only declared inside `.tracking-rule-card:active`, meaning the scale-down animated but the scale-up (when released) snapped back instantly with no transition. Now the base rule has `transition: border-color ..., transform ...` so both press and release animate smoothly.

**Why:** The press feedback felt broken — the card scaled down smoothly on press but snapped back instantly on release. A transition on the base element ensures both directions animate. This is the same pattern used by `.mizu-card` (transition on base, transform on `:active`).

**Outcome:** Kept. tsc passes. Tracking rule card press feedback now animates smoothly in both directions.

## Iteration D151 — Add touch fallback for show-actions-bar

**Target:** `index.css`

**What changed:**
- Added `@media (hover: none)` block that sets `grid-template-rows: 1fr` on `.controls-reveal` and `transform: translateY(0)` on `.show-actions-bar`. This makes the actions bar always visible on touch devices where the hover-reveal pattern doesn't work.

**Why:** The show detail page's action bar (Save, Remove, episode marker controls) uses a hover-reveal pattern: the bar is hidden by default and slides down when the user hovers over the hero area. On touch devices (phones, tablets), `:hover` doesn't apply, so the `@media (hover: hover)` block that controls the reveal never activates. This means the actions bar is permanently hidden on touch devices, making it impossible to save tracking rules, remove an anime, or adjust the episode marker. The `@media (hover: none)` fallback ensures the bar is always visible on these devices.

**Outcome:** Kept. tsc passes. Touch device users can now access the action bar on the show detail page.

## Iteration D152 — Eliminate duplicate episode-filters SegmentedControl

**Target:** `ShowDetailPage.tsx`, `index.css`

**What changed:**
- Removed the duplicate `episode-filters-desktop` and `episode-filters-mobile` wrapper divs that rendered two identical SegmentedControl instances (same props, same children). Replaced with a single SegmentedControl.
- Removed the `.episode-filters-desktop` and `.episode-filters-mobile` CSS rules (show/hide via media query) since they're no longer needed.

**Why:** The two SegmentedControl instances were completely identical — same value, onChange, label, layout, size, and children. The CSS just showed one and hid the other based on viewport width. This wasted DOM nodes and made the episode filter logic harder to maintain (any change had to be applied in two places). A single SegmentedControl with `layout="fill"` already adapts to container width, so the duplication was unnecessary.

**Outcome:** Kept. tsc passes. Episode filter is now a single component — cleaner DOM, no visual change.

## Iteration D153 — Fix hero image fade-in on show navigation (key prop)

**Target:** `ShowDetailPage.tsx`

**What changed:**
- Added `key={showId}` to the hero `<img>` element. When navigating between shows, React reuses the same `<img>` DOM node (just updates `src`), so the `is-loaded` class from the previous image persists. This means the new image appears at `opacity: 1` immediately, skipping the fade-in transition added in D149. The `key` prop forces React to re-mount the `<img>` when `showId` changes, resetting the class list so the new image starts at `opacity: 0` and fades in properly.

**Why:** Without the key, the fade-in transition only played on the first page load. Subsequent navigations between shows showed the new backdrop instantly with no transition, which was inconsistent with the card images (which always fade in). The key ensures the fade-in plays every time the show changes.

**Outcome:** Kept. tsc passes. Hero backdrop now fades in on every show navigation, not just the first load.

## Iteration D153b — Fix hero image key to include artwork version

**Target:** `ShowDetailPage.tsx`

**What changed:**
- Updated the hero `<img>` key from `key={showId}` to `key={\`${showId}-${artworkVersion}\`}`. When artwork is refreshed, `artworkVersion` increments and `backdropUrl` gets a new cache-bust query param. Without the version in the key, React wouldn't re-mount the image, so the `is-loaded` class persisted and the new artwork appeared instantly with no fade-in. Including the version in the key ensures the image re-mounts and fades in whenever artwork is refreshed.

**Why:** Follow-up to D153. The key needed both `showId` (for navigation) and `artworkVersion` (for artwork refresh) to cover all cases where the image source changes.

**Outcome:** Kept. tsc passes. Hero backdrop now fades in on both show navigation and artwork refresh.

## Iteration D154 — Align hero content + actions bar horizontal padding with Section padding

**Target:** `index.css`

**What changed:**
- Changed `.show-hero-content` horizontal padding from `clamp(12px, 4vw, 32px)` to `var(--spacing-4)` (16px).
- Changed `.show-actions-bar` horizontal padding from `clamp(12px, 4vw, 32px)` to `var(--spacing-4)` (16px).

**Why:** The hero content and actions bar used a responsive `clamp(12px, 4vw, 32px)` for horizontal padding, which at 900px viewport width resolved to 32px. But the page content below (artwork card, tracking rules, episode list) uses `Section padding={4}` which maps to `--spacing-4` = 16px. This 16px mismatch meant the hero title and action bar content were pushed further inward than the page content below, creating visual misalignment. Using `var(--spacing-4)` ensures the hero text, action bar controls, and page content all share the same horizontal alignment.

**Outcome:** Kept. tsc passes. Hero content, actions bar, and page content now share consistent horizontal padding (16px).

## Iteration D155 — Fix show-shell top margin to match Section padding

**Target:** `index.css`

**What changed:**
- Changed `.show-shell` `margin-top` from `var(--spacing-2)` (8px) to `var(--spacing-4)` (16px).

**Why:** The show detail page's hero shell had 8px top margin, but all other pages use `Section padding={4}` which gives 16px top padding. This 8px difference meant the hero header sat closer to the navbar than content on other pages (Library, Activity, Settings), creating an inconsistent vertical rhythm. Using `var(--spacing-4)` aligns the hero's top spacing with the Section padding used everywhere else.

**Outcome:** Kept. tsc passes. Show detail hero now has consistent top spacing with other pages.

## Iteration D155b — Remove redundant show-shell margin-top (Section padding already provides it)

**Target:** `index.css`

**What changed:**
- Removed `margin-top: var(--spacing-4)` from `.show-shell`. The hero Section already has `padding={4}` (16px) which provides the top spacing. The margin-top was doubling up, giving the hero 32px top spacing instead of 16px like other pages.

**Why:** In D155 I added `margin-top: var(--spacing-4)` to match other pages' top spacing, but I overlooked that the hero is already wrapped in a `<Section padding={4}>` which provides 16px top padding. The margin-top stacked on top of the Section padding, giving 32px — double what other pages get. Removing the margin-top lets the Section padding handle it alone, matching other pages exactly.

**Outcome:** Kept. tsc passes. Hero top spacing now matches other pages (16px from Section padding only).

## Iteration D156 — Fix hero image loading late on navigation/refresh

**Target:** `ShowDetailPage.tsx`

**What changed:**
- Removed the early-return spinner that blocked the entire page (including the hero image) while `getShow()` was loading.
- The hero shell now renders immediately using `showId` from the URL — `backdropUrl` is memoized from `showId` alone (`getArtworkBannerUrl(showId)`), so the `<img>` starts loading the instant the route matches, before any API calls complete.
- The hero title shows a non-breaking space (`\u00a0`) as placeholder while `show` is null, preserving the hero's height so there's no layout shift when the title arrives.
- The actions bar (Save/Remove/episode controls) is now conditionally rendered only when `show` is available, since it needs show data for the watermark and save logic.
- The content section below (artwork card, tracking rules, episode list) shows a spinner while `loadingShow || !show`, replacing the old full-page spinner.
- The AlertDialog is also conditionally rendered only when `show` is available.

**Why:** The hero image was the last thing to load because the entire page was blocked behind a spinner while `getShow()` completed (typically 200-500ms). But the hero image URL only needs `showId` from the URL — it doesn't depend on show data at all. The backend artwork endpoint serves the file if it exists. By rendering the hero shell immediately, the image starts loading in parallel with the API calls, cutting perceived load time significantly.

**Outcome:** Kept. tsc passes. Hero image now starts loading immediately on navigation/refresh instead of waiting for getShow().

## Iteration D157 — Fix episode badge vertical alignment with hero title

**Target:** `ShowDetailPage.tsx`, `index.css`

**What changed:**
- Changed the HStack wrapping the hero title + badge from `align="center"` to `align="baseline"`. Center alignment aligns based on box height, but the Heading h1 has a large line-height that makes the box taller than the visible text. Baseline alignment aligns the badge to the text baseline, which is visually closer to the text center.
- Added `.show-hero-title` class with `line-height: 1.2` to tighten the heading's line-height, reducing the gap between the text and the badge.
- Added `.show-hero-badge` class with `vertical-align: middle` and `transform: translateY(-1px)` to fine-tune the badge position so it sits visually centered with the title text.

**Why:** The episode count badge appeared slightly higher than the title text because `align="center"` centered based on the heading's full box height (including line-height padding above and below the text), not the visible text. Baseline alignment + tightened line-height + a 1px nudge brings the badge into visual alignment with the title.

**Outcome:** Kept. tsc passes. Episode badge now visually aligns with the hero title text.

## Iteration D158 — Backend artwork caching + actions bar padding/divider fixes

**Target:** `internal/artwork/manager.go`, `web-astryx/src/index.css`

**What changed:**
- **Backend caching:** Changed `Cache-Control: no-cache` to `max-age=300, must-revalidate` in `ServeImage()` (line 375). The ETag (from file modtime + size) and 304 Not Modified handling were already in place, but `no-cache` forced the browser to revalidate on every single request — even for cached images, a network round-trip was required. With `max-age=300`, the browser serves cached images instantly for 5 minutes without any network request, then revalidates via ETag after. The `?v=` cache-bust param on `backdropUrl` (incremented on artwork refresh) still bypasses cache when artwork is explicitly refreshed.
- **Actions bar bottom padding:** Set `.show-actions-bar` bottom padding from `var(--spacing-2)` (8px) to `0`. When the actions bar slid down on hover, its 8px bottom padding added to the hero Section's 16px bottom padding + content Section's 16px top padding, creating a visible gap increase. With 0 bottom padding, the gap stays consistent whether the bar is hidden or revealed.
- **Actions bar divider:** Added `border-bottom` to `.show-actions-bar` matching the existing `border-top` style (`1px solid color-mix(in srgb, var(--color-border) 50%, transparent)`). This provides a visual separator between the actions bar and the content section below when the bar is revealed.
- **Restored `.show-shell` margin-top:** D155b had removed `margin-top: var(--spacing-4)` from `.show-shell`. User confirmed the top padding makes the hero look better, so it was restored.

**Why:** Three issues reported by user: (1) hero image loads late on navigation/refresh — the `no-cache` header was forcing a network round-trip even for cached images; (2) bottom padding increases when the actions bar slides down — the bar's own bottom padding was adding to the Section gaps; (3) no divider between the actions bar and content below.

**Outcome:** Kept. tsc + go build pass. Backend now caches artwork for 5 min, actions bar no longer adds extra bottom padding, divider separates bar from content.

## Iteration D159 — CSS cleanup: duplicate tracking-rule-card rules + hero image opacity removal

**Target:** `web-astryx/src/index.css`, `web-astryx/src/routes/ShowDetailPage.tsx`

**What changed:**
- **Removed duplicate `.tracking-rule-card` CSS:** Two separate blocks defined the same transition property — lines 786-800 set `transition: border-color` + `:active { transform: scale(0.99) }`, then lines 832-839 redefined the same selector with `transition: border-color, transform` + the same `:active` rule. The second block silently overrode the first. Consolidated into one block. Also moved `.tracking-rule-card` out of the shared `season-card, artwork-card` transition group since it has its own (broader) transition.
- **Removed hero image opacity fade-in:** The `.show-hero-img` had `opacity: 0` with a transition to `opacity: 1` on `.is-loaded`. This made the image invisible until the `onLoad` callback fired — even for cached images, creating a perceived delay. Removed the opacity transition entirely. The image is now visible immediately when the browser renders it. Also removed the `onLoad` handler from the `<img>` element and the `.show-hero-img` entry from the `prefers-reduced-motion` block (no transition to disable).

**Why:** (1) Duplicate CSS rules are confusing and error-prone — someone editing the first block would think their change is live but the second block silently overrides it. (2) The user repeatedly complained about the hero image loading slowly and asked "why are we even rendering the image at opacity" — the `opacity: 0` start state was the cause. Cached images should appear instantly, not fade in.

**Outcome:** Kept. tsc + go build pass. CSS is cleaner, hero image appears immediately.

## Iteration D160 — Activity page: staggered card entrance + load-more separator

**Target:** `web-astryx/src/index.css`

**What changed:**
- **Staggered card entrance:** Added `card-enter` animation to `.activity-log-card` with nth-child delays (0-220ms, capped at 8+). Matches the library card pattern — cards fade and slide up slightly on page load, creating a polished entrance. The animation uses `backwards` fill mode so cards are invisible before their delay fires, preventing a flash of unstyled content.
- **Reduced motion:** Added `.activity-log-card { animation: none }` to the `prefers-reduced-motion` block so the entrance is disabled for users who prefer reduced motion.
- **Load-more separator:** Added `margin-block-start: var(--spacing-2)` to `.load-more-bar` for slightly more breathing room above the separator border, making it feel less cramped against the last card.

**Why:** The activity page cards appeared all at once with no entrance animation, while the library page had a polished staggered entrance. Adding the same pattern to activity cards creates visual consistency across pages. The load-more separator was flush against the last card with only padding — a small margin improves the visual rhythm.

**Outcome:** Kept. tsc passes. Activity cards now animate in with a staggered entrance matching the library page.

## Iteration D161 — Settings page: wrap sections in muted cards for visual grouping

**Target:** `web-astryx/src/routes/SettingsPage.tsx`

**What changed:**
- Wrapped the Connection, Sync, and Notifications sections in `<Card variant="muted" padding={4}>` to create visual grouping. Previously these sections were bare VStacks with Heading + Divider + rows — no visual boundary between sections except the VStack gap. The muted Card provides a clear visual container for each settings group, matching the pattern used in ShowDetailPage's artwork card.
- The Defaults section was left unwrapped because it contains TrackingRuleCards in a Grid — wrapping it would create excessive nesting (Card > Grid > Card).
- Internal Dividers between rows are kept — they still work inside the Card.
- Added `Card` import from `@astryxdesign/core/Card`.

**Why:** The settings page was visually flat — three sections (Connection, Sync, Notifications) flowed as undifferentiated VStacks with only gap spacing between them. There was no visual boundary to help the user understand where one section ends and another begins. Wrapping each section in a muted Card creates clear visual grouping, making the page easier to scan and more consistent with the ShowDetailPage which uses Card variant="muted" for its artwork section.

**Outcome:** Kept. tsc passes. Settings page sections now have clear visual grouping via muted cards.

## Iteration D162 — Season card expanded state border emphasis

**Target:** `web-astryx/src/index.css`

**What changed:**
- Added `.season-card:has([aria-expanded="true"]) { border-color: var(--color-border-emphasized) }` to visually distinguish expanded season cards from collapsed ones. The Collapsible component sets `aria-expanded="true"` on its trigger button when open, so `:has()` targets the expanded state without needing JS or extra className props.
- The border emphasis matches the existing hover state (`var(--color-border-emphasized)`), so hovering an expanded card keeps the same border — no visual conflict.

**Why:** When multiple season cards are visible, it's hard to tell which ones are expanded vs collapsed at a glance. The muted Card variant has a subtle border that doesn't change between states. Adding a slightly emphasized border when expanded gives users a clear visual signal of which seasons they've opened.

**Outcome:** Kept. tsc passes. Expanded season cards now have a emphasized border for clear visual distinction.

## Iteration D163 — Actions bar: center alignment + divider fix

**Target:** `web-astryx/src/index.css`, `web-astryx/src/routes/ShowDetailPage.tsx`

**What changed:**
- Changed `.show-actions-bar` from `align-items: flex-end` to `align-items: center` — buttons were aligned to the bottom of the bar, making them look like they were "at the bottom." Center alignment puts them vertically centered in the bar.
- Added `.show-hero-section { padding-bottom: 0 }` to remove the hero Section's 16px bottom padding. Previously the Section padding created a gap between the actions bar's `border-bottom` divider and the content Section below, making the divider appear disconnected. With 0 bottom padding, the bar's border-bottom sits flush at the end of the hero Section, creating a clean divider between hero and content.

**Why:** User reported "the bar buttons at the bottom and still no divider." The buttons were using `flex-end` alignment (bottom of bar) instead of centered. The divider (border-bottom on the actions bar) was visually disconnected from the content because the hero Section's 16px bottom padding created a gap.

**Outcome:** Kept. tsc passes. Buttons now centered in bar, divider visible flush against content.
