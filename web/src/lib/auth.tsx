import * as React from 'react';
import { getApiKey, setApiKey } from './api';
import { getMe } from './accounts';
import type { Account, Role } from './api';
import { Logo } from '../components/Logo';
import { ThemeToggle } from '../components/ThemeToggle';

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

  const value = React.useMemo(
    () => ({ apiKey, account, isOwnerOrAdmin, status, error, signIn, signOut }),
    [apiKey, account, isOwnerOrAdmin, status, error, signIn, signOut]
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

/** Renders an API key prompt until one is set and confirmed to work against the API. */
export function ApiKeyGate({ children }: { children: React.ReactNode }) {
  const { status, error, signIn } = useAuth();
  const [input, setInput] = React.useState('');

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

  const handleSubmit = (e: React.FormEvent) => {
    e.preventDefault();
    if (!input.trim()) return;
    signIn(input.trim());
  };

  return (
    <div className="relative flex min-h-full items-center justify-center bg-neutral-50 px-4 dark:bg-neutral-950">
      <ThemeToggle className="fixed right-5 top-5" />
      <div className="w-full max-w-sm">
        <div className="mb-8 flex justify-center">
          <Logo />
        </div>
        <div className="rounded-xl bg-white p-6 shadow-sm ring-1 ring-neutral-950/5 dark:bg-white/5 dark:ring-white/10">
          <h1 className="text-base font-semibold text-neutral-900 dark:text-white">Sign in with an API key</h1>
          <p className="mt-1 text-sm text-neutral-500 dark:text-neutral-400">
            Paste a key issued via <code className="rounded bg-neutral-100 px-1 py-0.5 text-xs dark:bg-white/10">createApiKey</code> or{' '}
            <code className="rounded bg-neutral-100 px-1 py-0.5 text-xs dark:bg-white/10">cmd/seed</code>. Stored only in this
            browser.
          </p>
          <form onSubmit={handleSubmit} className="mt-4 space-y-3">
            <input
              type="password"
              autoFocus
              value={input}
              onChange={(e) => setInput(e.target.value)}
              placeholder="llmgw_..."
              className="block w-full rounded-md border-0 bg-white px-3 py-2 text-sm text-neutral-900 shadow-sm ring-1 ring-inset ring-neutral-300 placeholder:text-neutral-400 focus:ring-2 focus:ring-inset focus:ring-primary-600 dark:bg-white/5 dark:text-white dark:ring-white/10"
            />
            {error && <p className="text-sm text-red-600 dark:text-red-400">{error}</p>}
            <button
              type="submit"
              disabled={!input.trim()}
              className="w-full rounded-md bg-primary-600 px-3 py-2 text-sm font-semibold text-white shadow-sm transition hover:bg-primary-500 disabled:cursor-not-allowed disabled:opacity-50"
            >
              Continue
            </button>
          </form>
        </div>
      </div>
    </div>
  );
}
