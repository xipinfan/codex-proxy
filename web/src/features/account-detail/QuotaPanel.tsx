import { formatDateTime, formatNumber, formatPercent } from '../../lib/format';
import { parseQuotaDetails } from '../../lib/quota';
import type { AccountView } from '../../lib/types';

function deriveQuotaPercent(account: AccountView): number | null {
  const primaryWindow = parseQuotaDetails(account.quota?.rawData).primaryWindow;
  if (primaryWindow) {
    return primaryWindow.usedPercent;
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
  const details = parseQuotaDetails(account.quota.rawData);

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
      {details.windows.length > 0 ? (
        <div className="relative mt-4 grid gap-3 text-sm text-[color:var(--text-secondary)]">
          {details.windows.map((window) => (
            <div key={window.label} className="rounded-[20px] bg-white/62 px-4 py-3">
              <div className="flex items-center justify-between gap-3">
                <p className="text-xs font-semibold tracking-[0.16em]">{window.label}</p>
                <span className="font-semibold text-[color:var(--text-primary)]">可用 {formatPercent(window.availablePercent)}</span>
              </div>
              <div className="mt-3 h-2 overflow-hidden rounded-full bg-[rgba(59,184,197,0.12)]">
                <div className="h-full rounded-full bg-gradient-to-r from-[#3bb8c5] to-[#f39239]" style={{ width: `${window.usedPercent}%` }} />
              </div>
              <div className="mt-2 flex flex-wrap items-center justify-between gap-2">
                <span>已用 {formatPercent(window.usedPercent)}</span>
                <span>重置 {formatDateTime(window.resetAt)}</span>
              </div>
            </div>
          ))}
        </div>
      ) : null}

      <div className="relative mt-4 grid gap-3 text-sm text-[color:var(--text-secondary)] sm:grid-cols-2">
        <div className="rounded-[20px] bg-white/62 px-4 py-3">
          <p className="text-xs font-semibold tracking-[0.16em]">历史令牌消耗</p>
          <p className="mt-1 font-medium text-[color:var(--text-primary)]">{formatNumber(account.usage.totalTokens)}</p>
        </div>
        <div className="rounded-[20px] bg-white/62 px-4 py-3">
          <p className="text-xs font-semibold tracking-[0.16em]">最近检查</p>
          <p className="mt-1 font-medium text-[color:var(--text-primary)]">{formatDateTime(account.quota.checkedAt)}</p>
        </div>
      </div>
    </section>
  );
}
