import { useEffect, lazy, Suspense } from "react";
import { Routes, Route, Navigate, useLocation } from "react-router-dom";
import { AppProvider } from "./lib/store";
import { ToastViewport } from "./components/ToastViewport";
import { ShowsPage } from "./routes/ShowsPage";

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
