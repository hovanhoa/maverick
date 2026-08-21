import { BrowserRouter, Routes, Route, Navigate } from 'react-router-dom';
import { AuthProvider, ApiKeyGate } from './lib/auth';
import { ThemeProvider } from './lib/theme';
import { Shell } from './components/Shell';
import { AccountsPage } from './pages/AccountsPage';
import { TeamsPage } from './pages/TeamsPage';
import { ApiKeysPage } from './pages/ApiKeysPage';

export function App() {
  return (
    <ThemeProvider>
      <AuthProvider>
        <ApiKeyGate>
          <BrowserRouter>
            <Routes>
              <Route element={<Shell />}>
                <Route index element={<Navigate to="/accounts" replace />} />
                <Route path="accounts" element={<AccountsPage />} />
                <Route path="teams" element={<TeamsPage />} />
                <Route path="api-keys" element={<ApiKeysPage />} />
                <Route path="*" element={<Navigate to="/accounts" replace />} />
              </Route>
            </Routes>
          </BrowserRouter>
        </ApiKeyGate>
      </AuthProvider>
    </ThemeProvider>
  );
}
