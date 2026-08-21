import clsx from 'clsx';
import { SunIcon, MoonIcon } from '@heroicons/react/24/outline';
import { useTheme } from '../lib/theme';

export function ThemeToggle({ className }: { className?: string }) {
  const { theme, toggle } = useTheme();
  const isDark = theme === 'dark';

  return (
    <button
      onClick={toggle}
      title={isDark ? 'Switch to light theme' : 'Switch to dark theme'}
      className={clsx(
        'inline-flex items-center gap-1.5 rounded-md px-2 py-1.5 text-xs font-medium text-neutral-500 transition hover:bg-neutral-950/5 hover:text-neutral-900 dark:text-neutral-400 dark:hover:bg-white/5 dark:hover:text-white',
        className
      )}
    >
      {isDark ? <SunIcon className="h-4 w-4" /> : <MoonIcon className="h-4 w-4" />}
      {isDark ? 'Light' : 'Dark'}
    </button>
  );
}
