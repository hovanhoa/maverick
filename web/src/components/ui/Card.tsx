import * as React from 'react';
import clsx from 'clsx';

export function Card({ children, className }: { children: React.ReactNode; className?: string }) {
  return (
    <div className={clsx('rounded-xl bg-white shadow-sm ring-1 ring-neutral-950/5 dark:bg-white/5 dark:ring-white/10', className)}>
      {children}
    </div>
  );
}

export function CardHeader({ title, description, action }: { title: string; description?: string; action?: React.ReactNode }) {
  return (
    <div className="flex items-start justify-between gap-4 border-b border-neutral-950/5 px-5 py-4 dark:border-white/10">
      <div>
        <h2 className="text-sm font-semibold text-neutral-900 dark:text-white">{title}</h2>
        {description && <p className="mt-0.5 text-sm text-neutral-500 dark:text-neutral-400">{description}</p>}
      </div>
      {action}
    </div>
  );
}
