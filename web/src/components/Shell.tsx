import * as React from 'react';
import { NavLink, Outlet, useNavigate } from 'react-router-dom';
import clsx from 'clsx';
import { UserCircleIcon, ArrowRightStartOnRectangleIcon } from '@heroicons/react/24/outline';
import { useAuth } from '../lib/auth';
import { avatarUrl } from '../lib/api';
import { Logo } from './Logo';

const NAV = [
  { name: 'Accounts', to: '/accounts' },
  { name: 'Teams', to: '/teams' },
  { name: 'API Keys', to: '/api-keys' },
  { name: 'Playground', to: '/playground' },
  { name: 'Usage', to: '/usage' },
  { name: 'Request Logs', to: '/request-logs' }
];

function maskKey(key: string): string {
  const [prefix] = key.split('_');
  return `${prefix}_••••••••`;
}

/** The signed-in user's picture in the sidebar, falling back to the generic icon if none is set. */
function SidebarAvatar({ accountId, nonce }: { accountId: string; nonce: number }) {
  const [broken, setBroken] = React.useState(false);
  React.useEffect(() => setBroken(false), [nonce]);

  if (broken) {
    return <UserCircleIcon className="h-8 w-8 shrink-0 text-neutral-400" />;
  }
  return (
    <img
      src={`${avatarUrl(accountId)}?v=${nonce}`}
      alt=""
      className="h-8 w-8 shrink-0 rounded-full bg-neutral-200 object-cover dark:bg-white/10"
      onError={() => setBroken(true)}
    />
  );
}

export function Shell() {
  const { apiKey, account, signOut, avatarNonce } = useAuth();
  const navigate = useNavigate();

  return (
    <div className="min-h-full bg-white lg:bg-neutral-100 dark:bg-neutral-950 dark:lg:bg-black">
      <div className="fixed inset-y-0 left-0 hidden w-64 flex-col lg:flex">
        <nav className="flex h-full flex-col p-4">
          <div className="flex items-center px-2 py-2">
            <Logo />
          </div>

          <ul className="mt-6 flex flex-1 flex-col gap-0.5">
            {NAV.map((item) => (
              <li key={item.to}>
                <NavLink
                  to={item.to}
                  className={({ isActive }) =>
                    clsx(
                      'flex items-center rounded-md px-2.5 py-2 text-sm font-medium transition-colors',
                      isActive
                        ? 'bg-neutral-950/5 text-neutral-950 dark:bg-white/10 dark:text-white'
                        : 'text-neutral-600 hover:bg-neutral-950/5 hover:text-neutral-950 dark:text-neutral-400 dark:hover:bg-white/5 dark:hover:text-white'
                    )
                  }
                >
                  {item.name}
                </NavLink>
              </li>
            ))}
          </ul>

          <div className="mt-auto space-y-3 border-t border-neutral-950/5 pt-4 dark:border-white/10">
            {apiKey && (
              <button
                type="button"
                title="Edit your profile"
                onClick={() => navigate('/accounts', { state: { editSelf: true } })}
                className="flex w-full items-center gap-2 rounded-md px-2.5 py-1 text-left transition hover:bg-neutral-950/5 dark:hover:bg-white/5"
              >
                {account ? (
                  <SidebarAvatar accountId={account.id} nonce={avatarNonce} />
                ) : (
                  <UserCircleIcon className="h-8 w-8 shrink-0 text-neutral-400" />
                )}
                <div className="min-w-0">
                  <p className="truncate text-sm font-medium text-neutral-900 dark:text-white">
                    {account?.name || account?.email || maskKey(apiKey)}
                  </p>
                  {account && <p className="truncate text-xs text-neutral-500 dark:text-neutral-400">{account.role}</p>}
                </div>
              </button>
            )}
            <div className="flex items-center px-2.5">
              <button
                onClick={signOut}
                className="flex items-center gap-1.5 text-sm font-medium text-neutral-500 transition hover:text-red-600 dark:text-neutral-400 dark:hover:text-red-400"
              >
                <ArrowRightStartOnRectangleIcon className="h-4 w-4" />
                Sign out
              </button>
            </div>
          </div>
        </nav>
      </div>

      <header className="border-b border-neutral-950/5 px-4 py-3 lg:hidden dark:border-white/10">
        <div className="flex items-center justify-between">
          <Logo />
          <div className="flex items-center gap-2">
            <button onClick={signOut} className="text-sm font-medium text-neutral-500 dark:text-neutral-400">
              Sign out
            </button>
          </div>
        </div>
        <nav className="mt-3 flex gap-1">
          {NAV.map((item) => (
            <NavLink
              key={item.to}
              to={item.to}
              className={({ isActive }) =>
                clsx(
                  'rounded-md px-2.5 py-1.5 text-sm font-medium',
                  isActive
                    ? 'bg-neutral-950/5 text-neutral-950 dark:bg-white/10 dark:text-white'
                    : 'text-neutral-600 dark:text-neutral-400'
                )
              }
            >
              {item.name}
            </NavLink>
          ))}
        </nav>
      </header>

      <main className="lg:pl-64 lg:pr-2 lg:pt-2 lg:pb-2">
        <div className="p-6 lg:min-h-[calc(100vh-1rem)] lg:rounded-xl lg:bg-white lg:p-10 lg:shadow-sm lg:ring-1 lg:ring-neutral-950/5 dark:lg:bg-neutral-900 dark:lg:ring-white/10">
          <div className="mx-auto max-w-5xl">
            <Outlet />
          </div>
        </div>
      </main>
    </div>
  );
}
