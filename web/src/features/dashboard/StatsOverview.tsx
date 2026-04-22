import { Card } from '../../components/ui/Card';
import { formatCompactNumber, formatNumber } from '../../lib/format';
import type { SummaryView } from '../../lib/types';

interface StatsOverviewProps {
  summary: SummaryView;
}

const cards = [
  { key: 'total', label: '账号总数', accent: 'from-[#f39239] to-[#ffd29e]' },
  { key: 'active', label: '正常可用', accent: 'from-[#3bb8c5] to-[#a7edf3]' },
  { key: 'cooldown', label: '冷却保留', accent: 'from-[#ffb15d] to-[#ffe3ba]' },
  { key: 'disabled', label: '已停用', accent: 'from-[#cf5e48] to-[#f6c4ba]' },
  { key: 'rpm', label: '每分钟请求', accent: 'from-[#6bbad1] to-[#cadff0]' },
] as const;

export function StatsOverview({ summary }: StatsOverviewProps) {
  return (
    <section className="grid gap-4 xl:grid-cols-[repeat(6,minmax(0,1fr))]">
      {cards.map((card) => (
        <Card key={card.key} className="group relative overflow-hidden rounded-[30px] p-0">
          <div className={`h-1.5 bg-gradient-to-r ${card.accent}`} />
          <div className="pointer-events-none absolute right-[-30px] top-[-34px] h-24 w-24 rounded-full bg-white/45 blur-2xl transition group-hover:scale-125" />
          <div className="relative space-y-3 p-5">
            <p className="text-xs font-semibold tracking-[0.18em] text-[color:var(--text-secondary)]">{card.label}</p>
            <p className="text-3xl font-semibold tracking-[-0.04em]">{formatCompactNumber(summary[card.key])}</p>
          </div>
        </Card>
      ))}
      <Card className="relative overflow-hidden rounded-[30px] bg-[rgba(255,250,245,0.88)]">
        <div className="absolute right-[-42px] top-[-42px] h-28 w-28 rounded-full bg-[rgba(59,184,197,0.14)] blur-2xl" />
        <p className="relative text-xs font-semibold tracking-[0.18em] text-[color:var(--text-secondary)]">令牌流量</p>
        <div className="mt-3 grid gap-3 text-sm text-[color:var(--text-secondary)]">
          <div className="flex items-center justify-between">
            <span>输入</span>
            <span className="font-semibold text-[color:var(--text-primary)]">{formatNumber(summary.totalInputTokens)}</span>
          </div>
          <div className="flex items-center justify-between">
            <span>输出</span>
            <span className="font-semibold text-[color:var(--text-primary)]">{formatNumber(summary.totalOutputTokens)}</span>
          </div>
        </div>
      </Card>
    </section>
  );
}
