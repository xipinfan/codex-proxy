import clsx from 'clsx';
import type { ButtonHTMLAttributes, ReactNode } from 'react';

interface ButtonProps extends ButtonHTMLAttributes<HTMLButtonElement> {
  children: ReactNode;
  variant?: 'primary' | 'secondary' | 'ghost';
}

export function Button({
  children,
  className,
  variant = 'primary',
  type = 'button',
  ...props
}: ButtonProps) {
  return (
    <button
      type={type}
      className={clsx(
        'inline-flex h-11 items-center justify-center rounded-2xl px-4 text-sm font-semibold transition disabled:cursor-not-allowed disabled:opacity-60',
        variant === 'primary' &&
          'bg-[linear-gradient(135deg,#3bb8c5_0%,#f39239_100%)] text-white shadow-[0_16px_32px_rgba(91,116,121,0.22)] hover:brightness-105',
        variant === 'secondary' &&
          'border border-[color:var(--border-soft)] bg-white/78 text-[color:var(--text-primary)] hover:bg-white',
        variant === 'ghost' &&
          'bg-transparent text-[color:var(--text-secondary)] hover:bg-white/55 hover:text-[color:var(--text-primary)]',
        className,
      )}
      {...props}
    >
      {children}
    </button>
  );
}
