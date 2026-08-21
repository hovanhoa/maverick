import * as React from 'react';
import * as Headless from '@headlessui/react';
import { XMarkIcon } from '@heroicons/react/24/outline';

export function Modal({
  open,
  onClose,
  title,
  children
}: {
  open: boolean;
  onClose: () => void;
  title: string;
  children: React.ReactNode;
}) {
  return (
    <Headless.Transition show={open} as={React.Fragment}>
      <Headless.Dialog onClose={onClose} className="relative z-50">
        <Headless.TransitionChild
          enter="ease-out duration-200"
          enterFrom="opacity-0"
          enterTo="opacity-100"
          leave="ease-in duration-150"
          leaveFrom="opacity-100"
          leaveTo="opacity-0"
        >
          <div className="fixed inset-0 bg-neutral-950/25" />
        </Headless.TransitionChild>

        <div className="fixed inset-0 flex items-center justify-center p-4">
          <Headless.TransitionChild
            enter="ease-out duration-200"
            enterFrom="opacity-0 scale-95"
            enterTo="opacity-100 scale-100"
            leave="ease-in duration-150"
            leaveFrom="opacity-100 scale-100"
            leaveTo="opacity-0 scale-95"
          >
            <Headless.DialogPanel className="w-full max-w-md rounded-xl bg-white p-6 shadow-xl ring-1 ring-neutral-950/5 dark:bg-neutral-900 dark:ring-white/10">
              <div className="mb-4 flex items-center justify-between">
                <Headless.DialogTitle className="text-base font-semibold text-neutral-900 dark:text-white">
                  {title}
                </Headless.DialogTitle>
                <button
                  onClick={onClose}
                  className="rounded-md p-1 text-neutral-400 hover:bg-neutral-100 hover:text-neutral-600 dark:hover:bg-white/10"
                >
                  <XMarkIcon className="h-5 w-5" />
                </button>
              </div>
              {children}
            </Headless.DialogPanel>
          </Headless.TransitionChild>
        </div>
      </Headless.Dialog>
    </Headless.Transition>
  );
}
