import { Card } from '../../components/ui/Card';
import { Drawer } from '../../components/ui/Drawer';
import { formatTokenCompact, formatTokenFull } from '../../lib/format';
import type { AccountView, SummaryView, TokenBucketView } from '../../lib/types';

interface TokenOverviewDrawerProps {
  open: boolean;
  onClose: () => void;
  summary: SummaryView;
  accounts: AccountView[];
}

function TokenMetricRow({ label, value, compact = false }: { label: string; value: number; compact?: boolean }) {
  const fullValue = formatTokenFull(value);
  return (
    <div className="flex min-w-0 items-center justify-between gap-3">
      <span className="shrink-0 whitespace-nowrap">{label}</span>
      <span className="min-w-0 truncate whitespace-nowrap text-right font-medium tabular-nums text-[color:var(--text-primary)]" title={fullValue}>
        {compact ? formatTokenCompact(value) : fullValue}
      </span>
    </div>
  );
}

function BucketPanel({ label, bucket, compact = false }: { label: string; bucket: TokenBucketView; compact?: boolean }) {
  const fullTotal = formatTokenFull(bucket.totalTokens);
  return (
    <Card className={`${compact ? 'p-4' : ''} min-w-0 rounded-[22px] bg-white/72 shadow-none`}>
      <p className="text-xs font-semibold tracking-[0.18em] text-[color:var(--text-secondary)]">{label}</p>
      <p
        className={`${compact ? 'text-[1.45rem]' : 'text-2xl'} mt-2 truncate whitespace-nowrap font-semibold tracking-[-0.04em] tabular-nums`}
        title={fullTotal}
      >
        {compact ? formatTokenCompact(bucket.totalTokens) : fullTotal}
      </p>
      <div className={`${compact ? 'text-[13px]' : 'text-sm'} mt-3 grid gap-2 text-[color:var(--text-secondary)]`}>
        <TokenMetricRow label="输入" value={bucket.inputTokens} compact={compact} />
        <TokenMetricRow label="输出" value={bucket.outputTokens} compact={compact} />
        <TokenMetricRow label="请求数" value={bucket.requestCount} compact={compact} />
      </div>
    </Card>
  );
}

export function TokenOverviewDrawer({ open, onClose, summary, accounts }: TokenOverviewDrawerProps) {
  const overview = summary.tokenOverview ?? {
    today: { inputTokens: 0, outputTokens: 0, totalTokens: 0, requestCount: 0 },
    sevenDays: { inputTokens: 0, outputTokens: 0, totalTokens: 0, requestCount: 0 },
    thirtyDays: { inputTokens: 0, outputTokens: 0, totalTokens: 0, requestCount: 0 },
    lifetime: { inputTokens: 0, outputTokens: 0, totalTokens: 0, requestCount: 0 },
    updatedAt: null,
  };
  const topAccounts = [...accounts]
    .sort((a, b) => (b.usage.thirtyDayTotalTokens ?? 0) - (a.usage.thirtyDayTotalTokens ?? 0))
    .slice(0, 5);

  return (
    <Drawer open={open} onClose={onClose} title="Token 概览">
      <div className="space-y-4">
        <div className="flex items-start justify-between gap-3">
          <div>
            <p className="text-xs uppercase tracking-[0.22em] text-[color:var(--text-secondary)]">Token 概览</p>
            <h2 className="mt-2 text-2xl font-semibold tracking-[-0.04em]">周期消耗细化</h2>
            <p className="mt-1 text-sm text-[color:var(--text-secondary)]">主面板默认展示今日，这里展示今日、7 日、30 日与累计统计。</p>
          </div>
          <button type="button" className="rounded-full bg-white/75 px-3 py-1 text-sm text-[color:var(--text-secondary)]" onClick={onClose}>
            关闭
          </button>
        </div>

        <BucketPanel label="今日" bucket={overview.today} />

        <div className="grid gap-3 sm:grid-cols-3">
          <BucketPanel label="近 7 日" bucket={overview.sevenDays} compact />
          <BucketPanel label="近 30 日" bucket={overview.thirtyDays} compact />
          <BucketPanel label="累计" bucket={overview.lifetime} compact />
        </div>

        <Card className="rounded-[22px] bg-white/72 shadow-none">
          <p className="text-xs font-semibold tracking-[0.18em] text-[color:var(--text-secondary)]">账号贡献 Top（近 30 日）</p>
          <div className="mt-3 space-y-2">
            {topAccounts.length === 0 ? (
              <p className="text-sm text-[color:var(--text-secondary)]">暂无可展示的账号贡献数据。</p>
            ) : (
              topAccounts.map((account) => (
                <div key={account.id} className="flex items-center justify-between rounded-[16px] bg-[rgba(255,255,255,0.74)] px-3 py-2">
                  <div>
                    <p className="text-sm font-medium">{account.email}</p>
                    <p className="text-xs text-[color:var(--text-secondary)]">今日 {formatTokenCompact(account.usage.todayTotalTokens)}</p>
                  </div>
                  <p className="text-sm font-semibold">{formatTokenCompact(account.usage.thirtyDayTotalTokens)}</p>
                </div>
              ))
            )}
          </div>
        </Card>

        <Card className="rounded-[22px] bg-[rgba(255,250,245,0.84)] shadow-none">
          <p className="text-xs font-semibold tracking-[0.18em] text-[color:var(--text-secondary)]">统计说明</p>
          <p className="mt-2 text-sm text-[color:var(--text-secondary)]">
            统计基于系统记录到的 usage 聚合，不等同于 OpenAI 官方账单；今日/7 日/30 日按自然日窗口计算。
          </p>
        </Card>
      </div>
    </Drawer>
  );
}
