import * as React from 'react';
import { getApiKey, setApiKey, login as loginRequest } from './api';
import { getMe } from './accounts';
import type { Account, Role } from './api';
import { Logo } from '../components/Logo';

type Status = 'idle' | 'loading' | 'ready' | 'error';

interface AuthContextValue {
  apiKey: string | null;
  /** The account backing the current API key - null until resolved. Used to gate the UI to what the caller's role actually permits. */
  account: Account | null;
  isOwnerOrAdmin: boolean;
  status: Status;
  error: string | null;
  signIn: (key: string) => void;
  signOut: () => void;
  /** Re-fetches `account` - call after the signed-in user edits their own profile (name, avatar), so the sidebar reflects it without a full reload. */
  refreshAccount: () => void;
  /** Bumped by refreshAccount - append to an avatar URL to bust the browser's image cache after an upload/delete. */
  avatarNonce: number;
}

const AuthContext = React.createContext<AuthContextValue | null>(null);

const PRIVILEGED: Role[] = ['OWNER', 'ADMIN'];

export function AuthProvider({ children }: { children: React.ReactNode }) {
  const [apiKey, setApiKeyState] = React.useState<string | null>(getApiKey());
  const [account, setAccount] = React.useState<Account | null>(null);
  const [status, setStatus] = React.useState<Status>(apiKey ? 'loading' : 'idle');
  const [error, setError] = React.useState<string | null>(null);

  const resolve = React.useCallback(async () => {
    setStatus('loading');
    setError(null);
    try {
      const me = await getMe();
      setAccount(me);
      setStatus('ready');
    } catch (err) {
      setApiKey(null);
      setApiKeyState(null);
      setAccount(null);
      setStatus('error');
      setError(err instanceof Error ? err.message : 'unable to verify this key');
    }
  }, []);

  // Re-verify whenever a key becomes present - covers both a fresh sign-in and
  // a stored key picked up again on page load.
  React.useEffect(() => {
    if (apiKey) {
      resolve();
    }
  }, [apiKey, resolve]);

  const signIn = React.useCallback((key: string) => {
    setApiKey(key);
    setApiKeyState(key);
  }, []);

  const signOut = React.useCallback(() => {
    setApiKey(null);
    setApiKeyState(null);
    setAccount(null);
    setStatus('idle');
    setError(null);
  }, []);

  const isOwnerOrAdmin = account != null && PRIVILEGED.includes(account.role);

  const [avatarNonce, setAvatarNonce] = React.useState(0);

  // Quietly re-fetches `account` without touching `status` - a full
  // resolve() would flip status back to 'loading' and briefly replace the
  // whole app with ApiKeyGate's "Connecting…" screen on every profile save.
  const refreshAccount = React.useCallback(async () => {
    if (!apiKey) return;
    try {
      setAccount(await getMe());
    } catch {
      // Best-effort - the sidebar just keeps showing the stale account.
    }
    setAvatarNonce((n) => n + 1);
  }, [apiKey]);

  const value = React.useMemo(
    () => ({ apiKey, account, isOwnerOrAdmin, status, error, signIn, signOut, refreshAccount, avatarNonce }),
    [apiKey, account, isOwnerOrAdmin, status, error, signIn, signOut, refreshAccount, avatarNonce]
  );

  return <AuthContext.Provider value={value}>{children}</AuthContext.Provider>;
}

export function useAuth(): AuthContextValue {
  const ctx = React.useContext(AuthContext);
  if (!ctx) {
    throw new Error('useAuth must be used within an AuthProvider');
  }
  return ctx;
}

const inputClasses =
  'block w-full rounded-md border-0 bg-white px-3 py-2 text-sm text-neutral-900 shadow-sm ring-1 ring-inset ring-neutral-300 placeholder:text-neutral-400 focus:ring-2 focus:ring-inset focus:ring-primary-600 dark:bg-white/5 dark:text-white dark:ring-white/10';

/** Renders a login prompt until a key is set (via username/password, or a pasted key) and confirmed to work. */
export function ApiKeyGate({ children }: { children: React.ReactNode }) {
  const { status, error, signIn } = useAuth();
  const [mode, setMode] = React.useState<'password' | 'apiKey'>('password');

  const [username, setUsername] = React.useState('');
  const [password, setPassword] = React.useState('');
  const [loggingIn, setLoggingIn] = React.useState(false);
  const [loginError, setLoginError] = React.useState<string | null>(null);

  const [apiKeyInput, setApiKeyInput] = React.useState('');

  if (status === 'ready') {
    return <>{children}</>;
  }

  if (status === 'loading') {
    return (
      <div className="flex min-h-full items-center justify-center bg-neutral-50 dark:bg-neutral-950">
        <p className="text-sm text-neutral-500 dark:text-neutral-400">Connecting…</p>
      </div>
    );
  }

  const handlePasswordSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    if (!username.trim() || !password) return;
    setLoggingIn(true);
    setLoginError(null);
    try {
      const { key } = await loginRequest(username.trim(), password);
      signIn(key);
    } catch (err) {
      setLoginError(err instanceof Error ? err.message : String(err));
    } finally {
      setLoggingIn(false);
    }
  };

  const handleApiKeySubmit = (e: React.FormEvent) => {
    e.preventDefault();
    if (!apiKeyInput.trim()) return;
    signIn(apiKeyInput.trim());
  };

  return (
    <div className="relative flex min-h-full items-center justify-center bg-neutral-50 px-4 dark:bg-neutral-950">
      <div className="w-full max-w-sm">
        <div className="mb-8 flex justify-center">
          <Logo />
        </div>
        <div className="rounded-xl bg-white p-6 shadow-sm ring-1 ring-neutral-950/5 dark:bg-white/5 dark:ring-white/10">
          {mode === 'password' ? (
            <>
              <h1 className="text-base font-semibold text-neutral-900 dark:text-white">Sign in</h1>
              <p className="mt-1 text-sm text-neutral-500 dark:text-neutral-400">
                Use the username and password an OWNER or ADMIN issued you.
              </p>
              <form onSubmit={handlePasswordSubmit} className="mt-4 space-y-3">
                <input
                  type="text"
                  autoFocus
                  autoComplete="username"
                  value={username}
                  onChange={(e) => setUsername(e.target.value)}
                  placeholder="Username"
                  className={inputClasses}
                />
                <input
                  type="password"
                  autoComplete="current-password"
                  value={password}
                  onChange={(e) => setPassword(e.target.value)}
                  placeholder="Password"
                  className={inputClasses}
                />
                {(loginError ?? error) && <p className="text-sm text-red-600 dark:text-red-400">{loginError ?? error}</p>}
                <button
                  type="submit"
                  disabled={!username.trim() || !password || loggingIn}
                  className="w-full rounded-md bg-primary-600 px-3 py-2 text-sm font-semibold text-white shadow-sm transition hover:bg-primary-500 disabled:cursor-not-allowed disabled:opacity-50"
                >
                  {loggingIn ? 'Signing in…' : 'Sign in'}
                </button>
              </form>
              <button
                type="button"
                onClick={() => setMode('apiKey')}
                className="mt-4 w-full text-center text-xs text-neutral-400 hover:text-neutral-600 dark:hover:text-neutral-300"
              >
                Sign in with an API key instead
              </button>
            </>
          ) : (
            <>
              <h1 className="text-base font-semibold text-neutral-900 dark:text-white">Sign in with an API key</h1>
              <p className="mt-1 text-sm text-neutral-500 dark:text-neutral-400">
                Paste a key issued from the API Keys page or <code className="rounded bg-neutral-100 px-1 py-0.5 text-xs dark:bg-white/10">cmd/seed</code>.
                Stored only in this browser.
              </p>
              <form onSubmit={handleApiKeySubmit} className="mt-4 space-y-3">
                <input
                  type="password"
                  autoFocus
                  value={apiKeyInput}
                  onChange={(e) => setApiKeyInput(e.target.value)}
                  placeholder="llmgw_..."
                  className={inputClasses}
                />
                {error && <p className="text-sm text-red-600 dark:text-red-400">{error}</p>}
                <button
                  type="submit"
                  disabled={!apiKeyInput.trim()}
                  className="w-full rounded-md bg-primary-600 px-3 py-2 text-sm font-semibold text-white shadow-sm transition hover:bg-primary-500 disabled:cursor-not-allowed disabled:opacity-50"
                >
                  Continue
                </button>
              </form>
              <button
                type="button"
                onClick={() => setMode('password')}
                className="mt-4 w-full text-center text-xs text-neutral-400 hover:text-neutral-600 dark:hover:text-neutral-300"
              >
                Sign in with username and password instead
              </button>
            </>
          )}
        </div>
      </div>
    </div>
  );
}
