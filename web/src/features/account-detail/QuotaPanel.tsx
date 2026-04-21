import { formatDateTime, formatPercent } from '../../lib/format';
import type { AccountView } from '../../lib/types';

function asNumber(input: unknown): number | null {
  if (typeof input === 'number' && Number.isFinite(input)) {
    return input;
  }

  if (typeof input === 'string' && input.trim()) {
    const value = Number(input);
    return Number.isFinite(value) ? value : null;
  }

  return null;
}

function deriveQuotaPercent(account: AccountView): number | null {
  const rawData = account.quota?.rawData;

  if (rawData && typeof rawData === 'object' && !Array.isArray(rawData)) {
    const record = rawData as Record<string, unknown>;
    const direct = asNumber(record.used_percent ?? record.usedPercent ?? record.usage_percent ?? record.usagePercent);
    if (direct !== null) {
      return Math.max(0, Math.min(100, direct));
    }

    const used = asNumber(record.used ?? record.current_usage ?? record.currentUsage);
    const limit = asNumber(record.limit ?? record.max ?? record.total ?? record.quota);
    if (used !== null && limit !== null && limit > 0) {
      return Math.max(0, Math.min(100, (used / limit) * 100));
    }
  }

  if (account.quotaExhausted) {
    return 100;
  }

  return null;
}

export function QuotaPanel({ account }: { account: AccountView }) {
  if (!account.quota) {
    return (
      <section className="rounded-[24px] border border-[color:var(--border-soft)] bg-white/70 p-4">
        <p className="text-xs uppercase tracking-[0.24em] text-[color:var(--text-secondary)]">额度窗口</p>
        <p className="mt-3 text-sm text-[color:var(--text-secondary)]">暂无额度数据</p>
      </section>
    );
  }

  const usedPercent = deriveQuotaPercent(account);

  return (
    <section className="rounded-[24px] border border-[color:var(--border-soft)] bg-white/70 p-4">
      <div className="flex items-center justify-between">
        <p className="text-xs uppercase tracking-[0.24em] text-[color:var(--text-secondary)]">额度窗口</p>
        <span className="text-sm font-semibold text-[color:var(--text-primary)]">{formatPercent(usedPercent)}</span>
      </div>
      <div className="mt-3 h-3 overflow-hidden rounded-full bg-[rgba(59,184,197,0.12)]">
        <div
          className="h-full rounded-full bg-gradient-to-r from-[#3bb8c5] to-[#f39239] transition-[width]"
          style={{ width: `${usedPercent ?? 12}%` }}
        />
      </div>
      <div className="mt-4 grid gap-3 text-sm text-[color:var(--text-secondary)] sm:grid-cols-2">
        <div>
          <p className="text-xs uppercase tracking-[0.18em]">重置时间</p>
          <p className="mt-1 font-medium text-[color:var(--text-primary)]">{formatDateTime(account.quotaResetsAt)}</p>
        </div>
        <div>
          <p className="text-xs uppercase tracking-[0.18em]">最近检查</p>
          <p className="mt-1 font-medium text-[color:var(--text-primary)]">{formatDateTime(account.quota.checkedAt)}</p>
        </div>
      </div>
    </section>
  );
}
