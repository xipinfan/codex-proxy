import clsx from 'clsx';
import type { KeyboardEvent } from 'react';

import { Badge } from '../../components/ui/Badge';
import { formatDateTime, formatNumber, formatPercent } from '../../lib/format';
import { parseQuotaDetails } from '../../lib/quota';
import type { AccountView } from '../../lib/types';

interface AccountsTableProps {
  accounts: AccountView[];
  selectedAccountId: string | null;
  onSelect: (accountId: string) => void;
}

function QuotaBars({ account }: { account: AccountView }) {
  const details = parseQuotaDetails(account.quota?.rawData);
  if (details.primaryWindow || details.secondaryWindow) {
    const windows = [
      { key: 'primary', label: '5h', window: details.primaryWindow },
      { key: 'secondary', label: '7d', window: details.secondaryWindow },
    ];

    return (
      <div className="flex min-w-[180px] flex-col gap-2">
        {windows.map(({ key, label, window }) => {
          const percent = window?.availablePercent ?? null;
          return (
            <div key={key} className="grid grid-cols-[28px_minmax(0,1fr)_40px] items-center gap-2 text-xs">
              <span className="font-medium text-[color:var(--text-secondary)]">{label}</span>
              <div className="h-1.5 overflow-hidden rounded-full bg-[rgba(122,91,62,0.12)]">
                <div
                  role="progressbar"
                  aria-label={`${label} 可用额度`}
                  aria-valuemin={0}
                  aria-valuemax={100}
                  aria-valuenow={percent ?? 0}
                  className={clsx(
                    'h-full rounded-full transition-[width] duration-300',
                    percent === null ? 'bg-[rgba(122,91,62,0.18)]' : 'bg-gradient-to-r from-[#3bb8c5] to-[#f39239]',
                  )}
                  style={{ width: `${percent ?? 18}%` }}
                />
              </div>
              <span className="text-right font-medium text-[color:var(--text-secondary)]">{formatPercent(percent)}</span>
            </div>
          );
        })}
      </div>
    );
  }
  if (account.quotaExhausted) {
    return <span className="text-xs font-medium text-[#b35445]">已耗尽</span>;
  }
  if (account.quota?.valid === false) {
    return <span className="text-xs font-medium text-[#b35445]">检查失败</span>;
  }
  if (account.quota?.valid) {
    return <span className="text-xs font-medium text-[#2f7c72]">可用</span>;
  }

  return <span className="text-xs font-medium text-[color:var(--text-secondary)]">未检查</span>;
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
            <td className="px-4 py-4 text-[color:var(--text-secondary)]"><QuotaBars account={account} /></td>
          </tr>
        );
      })}
    </>
  );
}

