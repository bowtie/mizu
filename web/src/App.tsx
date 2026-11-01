import { useEffect, lazy, Suspense } from "react";
import { Routes, Route, Navigate, useLocation } from "react-router-dom";
import { ToastViewport } from "./components/ToastViewport";
import { prefetchDetailChunk } from "./lib/chunkPrefetch";
import { useAuthStatus, useBootstrap, useSettings } from "./lib/queries";
import { needsOnboarding } from "./lib/onboarding";
import { applySoundsEnabled } from "./lib/sounds";

// Route-based code splitting — all pages are lazy-loaded. ShowsPage
// (landing) is also lazy but its import is fired immediately in
// main.tsx so the browser starts downloading it in parallel with
// the main bundle parse.
const ShowsPage = lazy(() => import("./routes/ShowsPage").then((m) => ({ default: m.ShowsPage })));
const ShowDetailPage = lazy(() =>
  import("./routes/ShowDetailPage").then((m) => ({ default: m.ShowDetailPage })),
);
const ActivityPage = lazy(() =>
  import("./routes/ActivityPage").then((m) => ({ default: m.ActivityPage })),
);
const SettingsPage = lazy(() =>
  import("./routes/SettingsPage").then((m) => ({ default: m.SettingsPage })),
);
const OnboardingPage = lazy(() =>
  import("./routes/OnboardingPage").then((m) => ({ default: m.OnboardingPage })),
);
const LoginPage = lazy(() => import("./routes/LoginPage").then((m) => ({ default: m.LoginPage })));

function ScrollToTop() {
  const { pathname } = useLocation();
  useEffect(() => {
    window.scrollTo(0, 0);
  }, [pathname]);
  return null;
}

function PageFallback() {
  // Minimal skeleton — just enough to fill the content area so the
  // page doesn't look broken during the brief gap between React mount
  // and the lazy route chunk loading. ShowsPage is modulepreloaded in
  // HTML, so this typically shows for <50ms on first visit.
  return (
    <div style={{ padding: "var(--spacing-5)", maxWidth: 900, margin: "0 auto" }}>
      <div
        className="skeleton"
        style={{
          width: 120,
          height: 28,
          borderRadius: "var(--radius-inner)",
          marginBottom: "var(--spacing-4)",
        }}
      />
      <div
        style={{
          display: "grid",
          gridTemplateColumns: "repeat(auto-fill, minmax(200px, 1fr))",
          gap: "var(--spacing-4)",
        }}
      >
        {Array.from({ length: 6 }).map((_, i) => (
          <div
            key={i}
            className="skeleton"
            style={{ height: 280, borderRadius: "var(--radius-container)" }}
          />
        ))}
      </div>
    </div>
  );
}

// Applies the persisted "interaction sounds" preference whenever the
// bootstrap settings load or change. Without this, cuelume keeps its
// built-in enabled default after every reload while the Settings toggle
// still shows the saved "off" state. Mounted inside the authenticated
// tree only, so it never fires bootstrap on the login page.
function SoundsPreference() {
  const { data } = useBootstrap();
  useEffect(() => {
    const enabled = data?.settings?.ui_sounds_enabled;
    if (typeof enabled === "boolean") applySoundsEnabled(enabled);
  }, [data]);
  return null;
}

// Gate every content route behind auth. While the status request is in
// flight we render a neutral fallback rather than flashing the app; once
// resolved, unauthenticated visitors are bounced to /login with a
// return path. When no password is configured (fresh install) the app
// stays open — matching the *arr first-run behavior.
function RequireAuth({ children }: { children: React.ReactNode }) {
  const { data: status, isLoading } = useAuthStatus();
  const location = useLocation();

  if (isLoading) return <PageFallback />;
  if (status?.configured && !status.authenticated) {
    const from = encodeURIComponent(location.pathname + location.search);
    return <Navigate to={`/login?from=${from}`} replace />;
  }
  return (
    <>
      <SoundsPreference />
      {children}
    </>
  );
}

// A fresh install has no Sonarr key yet; send those visitors to the welcome
// flow instead of an empty library. "Skip setup" opts out for this browser.
function RequireSonarr({ children }: { children: React.ReactNode }) {
  const { data: settings, isPending } = useSettings();
  if (isPending) return <PageFallback />;
  if (needsOnboarding(settings?.sonarr_api_key_set)) return <Navigate to="/welcome" replace />;
  return <>{children}</>;
}

export function App() {
  // Prefetch the ShowDetailPage chunk during idle time after first
  // paint. The detail page is the most likely next navigation from
  // the landing page — prefetching it eliminates the chunk load delay
  // for users who click without hovering first.
  useEffect(() => {
    const ric =
      "requestIdleCallback" in window
        ? (cb: () => void) => (window as Window).requestIdleCallback(cb, { timeout: 3000 })
        : (cb: () => void) => setTimeout(cb, 500);
    ric(() => prefetchDetailChunk());
  }, []);

  return (
    <>
      <ScrollToTop />
      <Routes>
        <Route
          path="/login"
          element={
            <Suspense fallback={<PageFallback />}>
              <LoginPage />
            </Suspense>
          }
        />
        <Route
          path="/"
          element={
            <RequireAuth>
              <RequireSonarr>
                <Suspense fallback={<PageFallback />}>
                  <ShowsPage />
                </Suspense>
              </RequireSonarr>
            </RequireAuth>
          }
        />
        <Route
          path="/shows"
          element={
            <RequireAuth>
              <RequireSonarr>
                <Suspense fallback={<PageFallback />}>
                  <ShowsPage />
                </Suspense>
              </RequireSonarr>
            </RequireAuth>
          }
        />
        <Route
          path="/welcome"
          element={
            <RequireAuth>
              <Suspense fallback={<PageFallback />}>
                <OnboardingPage />
              </Suspense>
            </RequireAuth>
          }
        />
        <Route
          path="/shows/:id"
          element={
            <RequireAuth>
              <Suspense fallback={<PageFallback />}>
                <ShowDetailPage />
              </Suspense>
            </RequireAuth>
          }
        />
        <Route
          path="/activity"
          element={
            <RequireAuth>
              <Suspense fallback={<PageFallback />}>
                <ActivityPage />
              </Suspense>
            </RequireAuth>
          }
        />
        <Route
          path="/settings"
          element={
            <RequireAuth>
              <Suspense fallback={<PageFallback />}>
                <SettingsPage />
              </Suspense>
            </RequireAuth>
          }
        />
        <Route path="*" element={<Navigate to="/" replace />} />
      </Routes>
      <ToastViewport />
    </>
  );
}
