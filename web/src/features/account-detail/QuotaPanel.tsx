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
      <section className="relative overflow-hidden rounded-[26px] border border-[color:var(--border-soft)] bg-[rgba(255,250,245,0.78)] p-5">
        <div className="absolute right-[-36px] top-[-36px] h-24 w-24 rounded-full bg-[rgba(243,146,57,0.12)] blur-2xl" />
        <p className="relative text-xs font-semibold tracking-[0.22em] text-[color:var(--text-secondary)]">额度窗口</p>
        <p className="relative mt-3 text-sm text-[color:var(--text-secondary)]">暂无额度数据，下一次检查后会在这里显示使用比例和重置时间。</p>
      </section>
    );
  }

  const usedPercent = deriveQuotaPercent(account);

  return (
    <section className="relative overflow-hidden rounded-[26px] border border-[color:var(--border-soft)] bg-[rgba(255,250,245,0.82)] p-5 shadow-[inset_0_1px_0_rgba(255,255,255,0.72)]">
      <div className="absolute right-[-44px] top-[-44px] h-28 w-28 rounded-full bg-[rgba(59,184,197,0.16)] blur-2xl" />
      <div className="relative flex items-center justify-between">
        <div>
          <p className="text-xs font-semibold tracking-[0.22em] text-[color:var(--text-secondary)]">额度窗口</p>
          <p className="mt-1 text-sm text-[color:var(--text-secondary)]">用于判断账号是否还能继续参与调度</p>
        </div>
        <span className="rounded-full bg-white/70 px-3 py-1 text-sm font-semibold text-[color:var(--text-primary)]">{formatPercent(usedPercent)}</span>
      </div>
      <div className="relative mt-4 h-3 overflow-hidden rounded-full bg-[rgba(59,184,197,0.12)]">
        <div
          className="h-full rounded-full bg-gradient-to-r from-[#3bb8c5] to-[#f39239] transition-[width]"
          style={{ width: `${usedPercent ?? 12}%` }}
        />
      </div>
      <div className="relative mt-4 grid gap-3 text-sm text-[color:var(--text-secondary)] sm:grid-cols-2">
        <div className="rounded-[20px] bg-white/62 px-4 py-3">
          <p className="text-xs font-semibold tracking-[0.16em]">重置时间</p>
          <p className="mt-1 font-medium text-[color:var(--text-primary)]">{formatDateTime(account.quotaResetsAt)}</p>
        </div>
        <div className="rounded-[20px] bg-white/62 px-4 py-3">
          <p className="text-xs font-semibold tracking-[0.16em]">最近检查</p>
          <p className="mt-1 font-medium text-[color:var(--text-primary)]">{formatDateTime(account.quota.checkedAt)}</p>
        </div>
      </div>
    </section>
  );
}
