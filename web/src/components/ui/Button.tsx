import * as React from 'react';
import clsx from 'clsx';

type Variant = 'primary' | 'secondary' | 'ghost' | 'danger';

const variantClasses: Record<Variant, string> = {
  primary: 'bg-primary-600 text-white shadow-sm hover:bg-primary-500 focus-visible:outline-primary-600',
  secondary:
    'bg-white text-neutral-700 shadow-sm ring-1 ring-inset ring-neutral-300 hover:bg-neutral-50 focus-visible:outline-primary-600 dark:bg-white/5 dark:text-neutral-200 dark:ring-white/10 dark:hover:bg-white/10',
  ghost:
    'bg-transparent text-neutral-500 hover:bg-neutral-950/5 hover:text-neutral-700 focus-visible:outline-primary-600 dark:text-neutral-400 dark:hover:bg-white/5 dark:hover:text-neutral-200',
  danger:
    'bg-white text-red-600 shadow-sm ring-1 ring-inset ring-red-200 hover:bg-red-50 focus-visible:outline-red-600 dark:bg-white/5 dark:text-red-400 dark:ring-red-500/30 dark:hover:bg-red-500/10'
};

export function Button({
  variant = 'secondary',
  className,
  ...props
}: React.ButtonHTMLAttributes<HTMLButtonElement> & { variant?: Variant }) {
  return (
    <button
      className={clsx(
        'inline-flex items-center justify-center gap-1.5 rounded-md px-3 py-2 text-sm font-semibold transition focus-visible:outline focus-visible:outline-2 focus-visible:outline-offset-2 disabled:cursor-not-allowed disabled:opacity-50',
        variantClasses[variant],
        className
      )}
      {...props}
    />
  );
}
