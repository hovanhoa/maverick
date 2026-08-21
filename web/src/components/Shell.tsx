import { NavLink, Outlet } from 'react-router-dom';
import clsx from 'clsx';
import { UserCircleIcon, ArrowRightStartOnRectangleIcon } from '@heroicons/react/24/outline';
import { useAuth } from '../lib/auth';
import { Logo } from './Logo';
import { ThemeToggle } from './ThemeToggle';

const NAV = [
  { name: 'Accounts', to: '/accounts' },
  { name: 'Teams', to: '/teams' },
  { name: 'API Keys', to: '/api-keys' }
];

function maskKey(key: string): string {
  const [prefix] = key.split('_');
  return `${prefix}_••••••••`;
}

export function Shell() {
  const { apiKey, account, signOut } = useAuth();

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
              <div className="flex items-center gap-2 px-2.5">
                <UserCircleIcon className="h-8 w-8 shrink-0 text-neutral-400" />
                <div className="min-w-0">
                  <p className="truncate text-sm font-medium text-neutral-900 dark:text-white">
                    {account?.email ?? maskKey(apiKey)}
                  </p>
                  {account && <p className="truncate text-xs text-neutral-500 dark:text-neutral-400">{account.role}</p>}
                </div>
              </div>
            )}
            <div className="flex items-center justify-between px-2.5">
              <button
                onClick={signOut}
                className="flex items-center gap-1.5 text-sm font-medium text-neutral-500 transition hover:text-red-600 dark:text-neutral-400 dark:hover:text-red-400"
              >
                <ArrowRightStartOnRectangleIcon className="h-4 w-4" />
                Sign out
              </button>
              <ThemeToggle />
            </div>
          </div>
        </nav>
      </div>

      <header className="border-b border-neutral-950/5 px-4 py-3 lg:hidden dark:border-white/10">
        <div className="flex items-center justify-between">
          <Logo />
          <div className="flex items-center gap-2">
            <ThemeToggle />
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
