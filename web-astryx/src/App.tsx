import { useEffect, lazy, Suspense } from "react";
import { Routes, Route, Navigate, useLocation } from "react-router-dom";
import { AppProvider } from "./lib/store";
import { ToastViewport } from "./components/ToastViewport";
import { ShowsPage } from "./routes/ShowsPage";
import { prefetchDetailChunk } from "./lib/chunkPrefetch";

// Route-based code splitting — ShowsPage (landing) loads eagerly,
// other pages are lazy-loaded on first navigation.
const ShowDetailPage = lazy(() =>
  import("./routes/ShowDetailPage").then((m) => ({ default: m.ShowDetailPage })),
);
const ActivityPage = lazy(() =>
  import("./routes/ActivityPage").then((m) => ({ default: m.ActivityPage })),
);
const SettingsPage = lazy(() =>
  import("./routes/SettingsPage").then((m) => ({ default: m.SettingsPage })),
);

function ScrollToTop() {
  const { pathname } = useLocation();
  useEffect(() => {
    window.scrollTo(0, 0);
  }, [pathname]);
  return null;
}

function PageFallback() {
  return null;
}

export function App() {
  // Prefetch the ShowDetailPage chunk during idle time after first
  // paint. The detail page is the most likely next navigation from
  // the landing page — prefetching it eliminates the chunk load delay
  // for users who click without hovering first.
  useEffect(() => {
    const ric = "requestIdleCallback" in window
      ? (cb: () => void) => (window as Window).requestIdleCallback(cb, { timeout: 3000 })
      : (cb: () => void) => setTimeout(cb, 500);
    ric(() => prefetchDetailChunk());
  }, []);

  return (
    <AppProvider>
      <ScrollToTop />
      <Routes>
        <Route path="/" element={<ShowsPage />} />
        <Route path="/shows" element={<ShowsPage />} />
        <Route
          path="/shows/:id"
          element={
            <Suspense fallback={<PageFallback />}>
              <ShowDetailPage />
            </Suspense>
          }
        />
        <Route
          path="/activity"
          element={
            <Suspense fallback={<PageFallback />}>
              <ActivityPage />
            </Suspense>
          }
        />
        <Route
          path="/settings"
          element={
            <Suspense fallback={<PageFallback />}>
              <SettingsPage />
            </Suspense>
          }
        />
        <Route path="*" element={<Navigate to="/" replace />} />
      </Routes>
      <ToastViewport />
    </AppProvider>
  );
}
