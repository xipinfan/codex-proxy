import clsx from 'clsx';
import type { ReactNode } from 'react';

import { formatStatusLabel } from '../../lib/format';

interface BadgeProps {
  children?: ReactNode;
  status?: string;
}

export function Badge({ children, status }: BadgeProps) {
  const text = children ?? formatStatusLabel(status ?? '');

  return (
    <span
      className={clsx(
        'inline-flex items-center rounded-full px-3 py-1 text-xs font-semibold uppercase tracking-[0.12em]',
        status === 'active' && 'bg-[rgba(59,184,197,0.16)] text-[#14626b]',
        status === 'cooldown' && 'bg-[rgba(243,146,57,0.18)] text-[#8f5317]',
        status === 'disabled' && 'bg-[rgba(207,94,72,0.16)] text-[#8f2e1f]',
        !status && 'bg-[rgba(32,25,22,0.08)] text-[color:var(--text-secondary)]',
      )}
    >
      {text}
    </span>
  );
}
