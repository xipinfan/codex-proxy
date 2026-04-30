import clsx from 'clsx';
import type { ReactNode } from 'react';

interface ModalProps {
  children: ReactNode;
  open: boolean;
  onClose: () => void;
  title: string;
}

export function Modal({ children, open, onClose, title }: ModalProps) {
  return (
    <div
      className={clsx(
        'fixed inset-0 z-50 flex items-center justify-center p-4 transition',
        open ? 'pointer-events-auto opacity-100' : 'pointer-events-none opacity-0',
      )}
    >
      <button
        type="button"
        aria-label="关闭弹窗遮罩"
        className="absolute inset-0 bg-[rgba(44,28,10,0.18)] backdrop-blur-sm"
        onClick={onClose}
      />
      <div
        role="dialog"
        aria-modal="true"
        aria-label={title}
        className="relative z-10 w-full max-w-[680px] rounded-[32px] border border-white/55 bg-[rgba(255,251,245,0.94)] p-6 shadow-[var(--shadow-floating)]"
      >
        {children}
      </div>
    </div>
  );
}
