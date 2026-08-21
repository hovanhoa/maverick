import clsx from 'clsx';

/** "M" monogram cut by a diagonal slash. */
function MonogramMark({ className }: { className?: string }) {
  return (
    <svg viewBox="0 0 100 100" className={className} aria-hidden="true">
      <text x="50" y="76" textAnchor="middle" fontFamily="'Archivo Black', sans-serif" fontSize="72" fill="currentColor">
        M
      </text>
      <line x1="18" y1="92" x2="82" y2="8" stroke="currentColor" strokeWidth="4" />
    </svg>
  );
}

export function LogoMark({ className }: { className?: string }) {
  return (
    <span className={clsx('inline-flex h-8 w-8 items-center justify-center rounded-lg bg-neutral-900 dark:bg-neutral-700', className)}>
      <MonogramMark className="h-5 w-5 text-white" />
    </span>
  );
}

export function Logo({ className }: { className?: string }) {
  return (
    <span className={clsx('inline-flex items-center gap-2.5', className)}>
      <LogoMark />
      <span className="font-heading text-base font-semibold tracking-tight text-neutral-950 dark:text-white">Maverick</span>
    </span>
  );
}
