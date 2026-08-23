import clsx from 'clsx';

export function LogoMark({ className }: { className?: string }) {
  return <img src="/android-chrome-192x192.png" alt="" className={clsx('h-6 w-6', className)} />;
}

export function Logo({ className }: { className?: string }) {
  return (
    <span className={clsx('inline-flex items-center gap-2.5', className)}>
      <LogoMark />
      <span className="font-heading text-base font-semibold tracking-tight text-neutral-950 dark:text-white">Maverick</span>
    </span>
  );
}
