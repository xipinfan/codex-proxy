import { Badge } from '../../components/ui/Badge';
import { Button } from '../../components/ui/Button';
import { Card } from '../../components/ui/Card';
import { Drawer } from '../../components/ui/Drawer';
import { formatDateTime, formatNumber } from '../../lib/format';
import type { AccountView } from '../../lib/types';
import { QuotaPanel } from './QuotaPanel';

interface AccountDetailDrawerProps {
  account: AccountView | null;
  open: boolean;
  onClose: () => void;
}

export function AccountDetailDrawer({ account, open, onClose }: AccountDetailDrawerProps) {
  return (
    <Drawer open={open} onClose={onClose} title={account ? `${account.email} 详情` : '账号详情'}>
      {account ? (
        <div className="space-y-4">
          <div className="flex items-start justify-between gap-3">
            <div>
              <p className="text-xs uppercase tracking-[0.22em] text-[color:var(--text-secondary)]">账号详情</p>
              <h2 className="mt-2 text-2xl font-semibold tracking-[-0.04em]">{account.email}</h2>
              <p className="mt-1 text-sm text-[color:var(--text-secondary)]">{account.planType || '暂无套餐信息'}</p>
            </div>
            <Button variant="ghost" onClick={onClose}>
              关闭
            </Button>
          </div>

          <div className="flex items-center gap-3">
            <Badge status={account.status} />
            <span className="text-sm text-[color:var(--text-secondary)]">令牌过期时间 {formatDateTime(account.tokenExpire)}</span>
          </div>

          <Card className="rounded-[24px] bg-white/72 shadow-none">
            <p className="text-xs uppercase tracking-[0.22em] text-[color:var(--text-secondary)]">用量概览</p>
            <div className="mt-4 grid gap-3 sm:grid-cols-3">
              <div>
                <p className="text-xs text-[color:var(--text-secondary)]">完成次数</p>
                <p className="mt-1 text-xl font-semibold">{formatNumber(account.usage.totalCompletions)}</p>
              </div>
              <div>
                <p className="text-xs text-[color:var(--text-secondary)]">输入</p>
                <p className="mt-1 text-xl font-semibold">{formatNumber(account.usage.inputTokens)}</p>
              </div>
              <div>
                <p className="text-xs text-[color:var(--text-secondary)]">输出</p>
                <p className="mt-1 text-xl font-semibold">{formatNumber(account.usage.outputTokens)}</p>
              </div>
            </div>
            <div className="mt-4 rounded-[22px] bg-[rgba(32,25,22,0.04)] px-4 py-3">
              <p className="text-xs uppercase tracking-[0.18em] text-[color:var(--text-secondary)]">总 Tokens</p>
              <p className="mt-1 text-2xl font-semibold">{formatNumber(account.usage.totalTokens)}</p>
            </div>
          </Card>

          <QuotaPanel account={account} />

          <Card className="rounded-[24px] bg-white/72 shadow-none">
            <p className="text-xs uppercase tracking-[0.22em] text-[color:var(--text-secondary)]">健康指标</p>
            <div className="mt-4 grid gap-4 text-sm text-[color:var(--text-secondary)]">
              <div className="flex items-center justify-between">
                <span>请求数 / 错误数</span>
                <span className="font-medium text-[color:var(--text-primary)]">{formatNumber(account.totalRequests ?? 0)} / {formatNumber(account.totalErrors ?? 0)}</span>
              </div>
              <div className="flex items-center justify-between">
                <span>连续失败次数</span>
                <span className="font-medium text-[color:var(--text-primary)]">{formatNumber(account.consecutiveFailures ?? 0)}</span>
              </div>
              <div className="flex items-center justify-between">
                <span>最近刷新</span>
                <span className="font-medium text-[color:var(--text-primary)]">{formatDateTime(account.lastRefreshedAt)}</span>
              </div>
              <div className="flex items-center justify-between">
                <span>最近使用</span>
                <span className="font-medium text-[color:var(--text-primary)]">{formatDateTime(account.lastUsedAt)}</span>
              </div>
              <div className="rounded-[20px] border border-[color:var(--border-soft)] bg-[rgba(255,255,255,0.76)] px-4 py-3">
                <p className="text-xs uppercase tracking-[0.18em]">停用原因</p>
                <p className="mt-1 text-sm text-[color:var(--text-primary)]">{account.disableReason || '未报告停用原因'}</p>
              </div>
            </div>
          </Card>
        </div>
      ) : null}
    </Drawer>
  );
}

