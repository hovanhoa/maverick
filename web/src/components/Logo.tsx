import clsx from 'clsx';

/** Stylized wing/phoenix mark - spread wings fanning into three feather points each side,
 * meeting at a center point above a three-point tail. */
function WingMark({ className }: { className?: string }) {
  return (
    <svg viewBox="0 0 220 110" fill="none" className={className} aria-hidden="true">
      <g fill="currentColor">
        <path d="M110,32 L128,42 L216,6 L166,14 L124,32 Z" />
        <path d="M110,32 L118,46 L172,10 L136,18 L112,40 Z" />
        <path d="M110,32 L110,50 L138,16 L110,32 Z" />
        <path d="M110,32 L92,42 L4,6 L54,14 L96,32 Z" />
        <path d="M110,32 L102,46 L48,10 L84,18 L108,40 Z" />
        <path d="M110,32 L110,50 L82,16 L110,32 Z" />
        <path d="M110,36 L92,52 L82,92 L100,58 Z" />
        <path d="M110,36 L101,58 L110,105 L119,58 Z" />
        <path d="M110,36 L128,52 L138,92 L120,58 Z" />
      </g>
    </svg>
  );
}

export function LogoMark({ className }: { className?: string }) {
  return <WingMark className={clsx('h-6 w-auto text-neutral-900 dark:text-white', className)} />;
}

export function Logo({ className }: { className?: string }) {
  return (
    <span className={clsx('inline-flex items-center gap-2.5', className)}>
      <LogoMark />
      <span className="font-heading text-base font-semibold tracking-tight text-neutral-950 dark:text-white">Maverick</span>
    </span>
  );
}
