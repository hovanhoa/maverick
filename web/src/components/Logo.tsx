import clsx from 'clsx';

/** Minimalist top-down fighter-jet silhouette - nose up, delta wings, twin tail fins. */
function JetMark({ className }: { className?: string }) {
  return (
    <svg viewBox="0 0 24 24" fill="none" className={className} aria-hidden="true">
      <path
        d="M12 1L13.1 5.4 21 14 21 16.3 13.7 13.6 14.3 18.6 17.2 20.6 17.2 21.9 12 20.3 6.8 21.9 6.8 20.6 9.7 18.6 10.3 13.6 3 16.3 3 14 10.9 5.4Z"
        fill="currentColor"
      />
    </svg>
  );
}

export function LogoMark({ className }: { className?: string }) {
  return (
    <span className={clsx('inline-flex h-8 w-8 items-center justify-center rounded-lg bg-neutral-900 shadow-sm dark:bg-neutral-700', className)}>
      <JetMark className="h-4 w-4 text-accent-500" />
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
