import clsx from 'clsx';
import type { ReactNode } from 'react';

interface DrawerProps {
  children: ReactNode;
  open: boolean;
  onClose: () => void;
  title: string;
}

export function Drawer({ children, open, onClose, title }: DrawerProps) {
  return (
    <>
      <button
        type="button"
        aria-label="关闭抽屉遮罩"
        className={clsx(
          'fixed inset-0 z-30 bg-[rgba(64,40,14,0.14)] transition',
          open ? 'pointer-events-auto opacity-100' : 'pointer-events-none opacity-0',
        )}
        onClick={onClose}
      />
      <aside
        role="dialog"
        aria-modal="true"
        aria-label={title}
        className={clsx(
          'fixed inset-y-4 right-4 z-40 w-[min(460px,calc(100vw-24px))] overflow-y-auto rounded-[30px] border border-white/55 bg-[rgba(255,251,245,0.92)] p-5 shadow-[var(--shadow-floating)] backdrop-blur-2xl transition duration-300',
          open ? 'translate-x-0 opacity-100' : 'pointer-events-none translate-x-8 opacity-0',
        )}
      >
        {children}
      </aside>
    </>
  );
}
