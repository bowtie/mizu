// Shared vitest setup, loaded for every test file via `setupFiles` in
// vite.config.ts.
//
// React 19's act() requires this flag to be set in the test environment,
// otherwise it warns that the environment isn't act-aware on every act()
// call. It previously had to be duplicated at the top of each DOM test
// file — centralizing it here means new component tests need zero
// per-file setup (the happy-dom environment itself is also configured
// globally in vite.config.ts).
(globalThis as { IS_REACT_ACT_ENVIRONMENT?: boolean }).IS_REACT_ACT_ENVIRONMENT = true;
