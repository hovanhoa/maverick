import clsx from 'clsx';

/** Bold angular "M" wing mark, each stroke split by a thin accent stripe near its leading edge. */
function WingMMark({ className }: { className?: string }) {
  return (
    <svg viewBox="0 0 200 170" fill="none" className={className} aria-hidden="true">
      <g fill="currentColor">
        <path d="M0,4 L64,44 L80,54 L10,18 Z" />
        <path d="M16,24 L72,60 L92,96 L100,160 L58,96 Z" />
        <path d="M200,4 L136,44 L120,54 L190,18 Z" />
        <path d="M184,24 L128,60 L108,96 L100,160 L142,96 Z" />
      </g>
    </svg>
  );
}

export function LogoMark({ className }: { className?: string }) {
  return <WingMMark className={clsx('h-6 w-auto text-neutral-900 dark:text-white', className)} />;
}

export function Logo({ className }: { className?: string }) {
  return (
    <span className={clsx('inline-flex items-center gap-2.5', className)}>
      <LogoMark />
      <span className="font-heading text-base font-semibold tracking-tight text-neutral-950 dark:text-white">Maverick</span>
    </span>
  );
}
