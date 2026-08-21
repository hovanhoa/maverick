import { ExclamationTriangleIcon } from '@heroicons/react/24/outline';

export function ErrorAlert({ message }: { message: string }) {
  return (
    <div className="flex items-start gap-2 rounded-md bg-red-50 px-3 py-2.5 text-sm text-red-700 dark:bg-red-500/10 dark:text-red-400">
      <ExclamationTriangleIcon className="mt-0.5 h-4 w-4 shrink-0" />
      <span>{message}</span>
    </div>
  );
}
