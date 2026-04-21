import clsx from 'clsx';
import type { KeyboardEvent } from 'react';

import { Badge } from '../../components/ui/Badge';
import { formatDateTime, formatNumber } from '../../lib/format';
import type { AccountView } from '../../lib/types';

interface AccountsTableProps {
  accounts: AccountView[];
  selectedAccountId: string | null;
  onSelect: (accountId: string) => void;
}

function formatQuotaStatus(account: AccountView): string {
  if (account.quotaExhausted) {
    return '已耗尽';
  }
  if (account.quota?.valid === false) {
    return '检查失败';
  }
  if (account.quota?.valid) {
    return '可用';
  }

  return '未检查';
}

export function AccountsTable({ accounts, selectedAccountId, onSelect }: AccountsTableProps) {
  return (
    <>
      {accounts.map((account) => {
        const isSelected = selectedAccountId === account.id;
        const handleKeyDown = (event: KeyboardEvent<HTMLTableRowElement>) => {
          if (event.key === 'Enter' || event.key === ' ') {
            event.preventDefault();
            onSelect(account.id);
          }
        };

        return (
          <tr
            key={account.id}
            role="button"
            tabIndex={0}
            aria-label={account.email}
            data-selected={isSelected ? 'true' : 'false'}
            onClick={() => onSelect(account.id)}
            onKeyDown={handleKeyDown}
            className={clsx(
              'cursor-pointer border-b border-[rgba(122,91,62,0.08)] text-left transition last:border-b-0 focus:outline-none focus-visible:bg-white/80 focus-visible:ring-2 focus-visible:ring-[rgba(255,154,61,0.42)]',
              isSelected
                ? 'bg-[rgba(255,154,61,0.12)] shadow-[inset_0_0_0_1px_rgba(255,154,61,0.24)]'
                : 'hover:bg-white/70',
            )}
          >
            <td className="px-6 py-4">
              <span className="block font-medium text-[color:var(--text-primary)]">{account.email}</span>
              <span className="mt-1 block text-xs text-[color:var(--text-secondary)]">{account.planType || '暂无套餐信息'}</span>
            </td>
            <td className="px-4 py-4"><Badge status={account.status} /></td>
            <td className="px-4 py-4 text-[color:var(--text-secondary)]">{formatNumber(account.totalRequests ?? 0)}</td>
            <td className="px-4 py-4 text-[color:var(--text-secondary)]">{formatNumber(account.totalErrors ?? 0)}</td>
            <td className="px-4 py-4 text-[color:var(--text-secondary)]">{formatNumber(account.usage.totalTokens)}</td>
            <td className="px-4 py-4 text-[color:var(--text-secondary)]">{formatDateTime(account.lastUsedAt)}</td>
            <td className="px-4 py-4 text-[color:var(--text-secondary)]">{formatDateTime(account.lastRefreshedAt)}</td>
            <td className="px-4 py-4 text-[color:var(--text-secondary)]">{formatQuotaStatus(account)}</td>
          </tr>
        );
      })}
    </>
  );
}

