import { Routes, Route } from "react-router-dom";
import { AppProvider } from "./lib/store";
import { ToastViewport } from "./components/ToastViewport";
import { ShowsPage } from "./routes/ShowsPage";
import { ShowDetailPage } from "./routes/ShowDetailPage";
import { ActivityPage } from "./routes/ActivityPage";
import { SettingsPage } from "./routes/SettingsPage";

export function App() {
  return (
    <AppProvider>
      <Routes>
        <Route path="/" element={<ShowsPage />} />
        <Route path="/shows" element={<ShowsPage />} />
        <Route path="/shows/:id" element={<ShowDetailPage />} />
        <Route path="/activity" element={<ActivityPage />} />
        <Route path="/settings" element={<SettingsPage />} />
      </Routes>
      <ToastViewport />
    </AppProvider>
  );
}
