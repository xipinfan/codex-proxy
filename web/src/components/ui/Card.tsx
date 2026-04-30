import clsx from 'clsx';
import type { HTMLAttributes, ReactNode } from 'react';

interface CardProps extends HTMLAttributes<HTMLDivElement> {
  children?: ReactNode;
}

export function Card({ children, className, ...props }: CardProps) {
  return (
    <div
      className={clsx(
        'rounded-[28px] border border-white/50 bg-[color:var(--bg-surface)] p-5 shadow-[var(--shadow-soft)] backdrop-blur-xl',
        className,
      )}
      {...props}
    >
      {children}
    </div>
  );
}

