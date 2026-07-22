import { HashRouter, Routes, Route, Navigate } from 'react-router-dom';
import { AppShell } from '@astryxdesign/core/AppShell';
import { AppProvider } from './lib/store';
import { Navbar } from './components/Navbar';
import { ToastViewport } from './components/ToastViewport';
import { ShowsPage } from './routes/ShowsPage';
import { ShowDetailPage } from './routes/ShowDetailPage';
import { ActivityPage } from './routes/ActivityPage';
import { SettingsPage } from './routes/SettingsPage';

export function App() {
  return (
    <AppProvider>
      <HashRouter>
        <AppShell
          topNav={<Navbar />}
          contentPadding={0}
          height="fill"
        >
          <Routes>
            <Route path="/" element={<Navigate to="/shows" replace />} />
            <Route path="/shows" element={<ShowsPage />} />
            <Route path="/shows/:id" element={<ShowDetailPage />} />
            <Route path="/activity" element={<ActivityPage />} />
            <Route path="/settings" element={<SettingsPage />} />
          </Routes>
        </AppShell>
      </HashRouter>
      <ToastViewport />
    </AppProvider>
  );
}
