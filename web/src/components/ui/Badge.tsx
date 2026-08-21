import * as React from 'react';
import clsx from 'clsx';
import type { Role } from '../../lib/api';

const roleClasses: Record<Role, string> = {
  OWNER: 'bg-accent-100 text-accent-800 dark:bg-accent-400/10 dark:text-accent-400',
  ADMIN: 'bg-primary-100 text-primary-800 dark:bg-primary-400/10 dark:text-primary-400',
  MEMBER: 'bg-neutral-100 text-neutral-600 dark:bg-white/5 dark:text-neutral-400'
};

export function RoleBadge({ role }: { role: Role }) {
  return (
    <span className={clsx('inline-flex items-center rounded-md px-2 py-0.5 text-xs font-medium', roleClasses[role])}>
      {role}
    </span>
  );
}

export function Badge({ children, tone = 'neutral' }: { children: React.ReactNode; tone?: 'neutral' | 'ok' | 'alert' }) {
  const toneClasses = {
    neutral: 'bg-neutral-100 text-neutral-600 dark:bg-white/5 dark:text-neutral-400',
    ok: 'bg-emerald-100 text-emerald-800 dark:bg-emerald-400/10 dark:text-emerald-400',
    alert: 'bg-red-100 text-red-700 dark:bg-red-400/10 dark:text-red-400'
  }[tone];
  return <span className={clsx('inline-flex items-center rounded-md px-2 py-0.5 text-xs font-medium', toneClasses)}>{children}</span>;
}
