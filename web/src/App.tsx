import { BrowserRouter, Routes, Route, Navigate } from 'react-router-dom';
import { AuthProvider, ApiKeyGate } from './lib/auth';
import { Shell } from './components/Shell';
import { AccountsPage } from './pages/AccountsPage';
import { TeamsPage } from './pages/TeamsPage';
import { ApiKeysPage } from './pages/ApiKeysPage';
import { UsagePage } from './pages/UsagePage';
import { RequestLogsPage } from './pages/RequestLogsPage';
import { PlaygroundPage } from './pages/PlaygroundPage';

export function App() {
  return (
    <AuthProvider>
      <ApiKeyGate>
        <BrowserRouter>
          <Routes>
            <Route element={<Shell />}>
              <Route index element={<Navigate to="/accounts" replace />} />
              <Route path="accounts" element={<AccountsPage />} />
              <Route path="teams" element={<TeamsPage />} />
              <Route path="api-keys" element={<ApiKeysPage />} />
              <Route path="playground" element={<PlaygroundPage />} />
              <Route path="usage" element={<UsagePage />} />
              <Route path="request-logs" element={<RequestLogsPage />} />
              <Route path="*" element={<Navigate to="/accounts" replace />} />
            </Route>
          </Routes>
        </BrowserRouter>
      </ApiKeyGate>
    </AuthProvider>
  );
}
