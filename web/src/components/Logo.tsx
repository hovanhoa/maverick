import clsx from 'clsx';

/** Angular hawk/eagle head with a swept, fanning wing trailing behind it. */
function EagleMark({ className }: { className?: string }) {
  return (
    <svg viewBox="0 0 130 110" fill="none" className={className} aria-hidden="true">
      <g fill="currentColor">
        <path d="M44,6 L62,12 L68,24 L58,28 L66,38 L52,50 L8,46 L28,32 L22,20 Z" />
        <path d="M60,40 L74,38 L124,10 L96,26 L68,36 Z" />
        <path d="M58,46 L76,45 L122,20 L92,36 L64,44 Z" />
        <path d="M56,52 L76,53 L116,32 L86,46 L60,50 Z" />
        <path d="M54,58 L74,61 L108,44 L80,56 L56,58 Z" />
        <path d="M50,64 L70,68 L98,56 L74,64 L52,64 Z" />
      </g>
    </svg>
  );
}

export function LogoMark({ className }: { className?: string }) {
  return <EagleMark className={clsx('h-7 w-auto text-neutral-900 dark:text-white', className)} />;
}

export function Logo({ className }: { className?: string }) {
  return (
    <span className={clsx('inline-flex items-center gap-2.5', className)}>
      <LogoMark />
      <span className="font-heading text-base font-semibold tracking-tight text-neutral-950 dark:text-white">Maverick</span>
    </span>
  );
}
