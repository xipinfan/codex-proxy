import { Badge } from '../../components/ui/Badge';
import { Button } from '../../components/ui/Button';
import { Card } from '../../components/ui/Card';
import { Drawer } from '../../components/ui/Drawer';
import { formatDateTime, formatNumber, formatTokenFull } from '../../lib/format';
import type { AccountView } from '../../lib/types';
import { QuotaPanel } from './QuotaPanel';
import { useState } from 'react';

interface AccountDetailDrawerProps {
  account: AccountView | null;
  open: boolean;
  onClose: () => void;
  onDeleteAccount?: (account: AccountView) => Promise<void>;
}

export function AccountDetailDrawer({ account, open, onClose, onDeleteAccount }: AccountDetailDrawerProps) {
  const [confirmDelete, setConfirmDelete] = useState(false);
  const [deleting, setDeleting] = useState(false);

  async function handleDelete() {
    if (!account || !onDeleteAccount) {
      return;
    }
    setDeleting(true);
    try {
      await onDeleteAccount(account);
      setConfirmDelete(false);
    } finally {
      setDeleting(false);
    }
  }

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
            <p className="text-xs font-semibold tracking-[0.22em] text-[color:var(--text-secondary)]">用量概览</p>
            <div className="mt-4 grid gap-3 sm:grid-cols-2">
              <div className="rounded-[18px] bg-[rgba(255,255,255,0.72)] px-3 py-2">
                <p className="text-xs text-[color:var(--text-secondary)]">今日</p>
                <p className="mt-1 text-lg font-semibold">{formatTokenFull(account.usage.todayTotalTokens)}</p>
              </div>
              <div className="rounded-[18px] bg-[rgba(255,255,255,0.72)] px-3 py-2">
                <p className="text-xs text-[color:var(--text-secondary)]">近 7 日</p>
                <p className="mt-1 text-lg font-semibold">{formatTokenFull(account.usage.sevenDayTotalTokens)}</p>
              </div>
              <div className="rounded-[18px] bg-[rgba(255,255,255,0.72)] px-3 py-2">
                <p className="text-xs text-[color:var(--text-secondary)]">近 30 日</p>
                <p className="mt-1 text-lg font-semibold">{formatTokenFull(account.usage.thirtyDayTotalTokens)}</p>
              </div>
              <div className="rounded-[18px] bg-[rgba(255,255,255,0.72)] px-3 py-2">
                <p className="text-xs text-[color:var(--text-secondary)]">累计</p>
                <p className="mt-1 text-lg font-semibold">{formatTokenFull(account.usage.lifetimeTotalTokens)}</p>
              </div>
            </div>
            <div className="mt-4 grid gap-3 sm:grid-cols-3">
              <div>
                <p className="text-xs text-[color:var(--text-secondary)]">完成次数</p>
                <p className="mt-1 text-xl font-semibold">{formatNumber(account.usage.lifetimeRequestCount)}</p>
              </div>
              <div>
                <p className="text-xs text-[color:var(--text-secondary)]">输入</p>
                <p className="mt-1 text-xl font-semibold">{formatNumber(account.usage.lifetimeInputTokens)}</p>
              </div>
              <div>
                <p className="text-xs text-[color:var(--text-secondary)]">输出</p>
                <p className="mt-1 text-xl font-semibold">{formatNumber(account.usage.lifetimeOutputTokens)}</p>
              </div>
            </div>
            <div className="mt-4 rounded-[22px] bg-[rgba(32,25,22,0.04)] px-4 py-3">
              <p className="text-xs font-semibold tracking-[0.18em] text-[color:var(--text-secondary)]">令牌总量</p>
              <p className="mt-1 text-2xl font-semibold">{formatNumber(account.usage.lifetimeTotalTokens)}</p>
              <p className="mt-1 text-xs text-[color:var(--text-secondary)]">统计基于系统记录到的 usage 聚合，不等同于 OpenAI 官方账单。</p>
            </div>
          </Card>

          <QuotaPanel account={account} />

          <Card className="rounded-[24px] bg-white/72 shadow-none">
            <p className="text-xs font-semibold tracking-[0.22em] text-[color:var(--text-secondary)]">健康指标</p>
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
                <p className="text-xs font-semibold tracking-[0.18em]">停用原因</p>
                <p className="mt-1 text-sm text-[color:var(--text-primary)]">{account.disableReason || '未报告停用原因'}</p>
              </div>
            </div>
          </Card>

          <Card className="rounded-[24px] border border-[rgba(207,94,72,0.22)] bg-[rgba(255,247,244,0.88)] shadow-none">
            <p className="text-xs font-semibold tracking-[0.22em] text-[#8f2e1f]">危险操作</p>
            <p className="mt-2 text-sm text-[color:var(--text-secondary)]">删除后会从账号池移除，并清理对应存储记录。</p>
            {confirmDelete ? (
              <div className="mt-3 flex items-center gap-3">
                <Button variant="secondary" onClick={() => setConfirmDelete(false)}>
                  取消
                </Button>
                <Button disabled={deleting} onClick={() => void handleDelete()}>
                  {deleting ? '删除中...' : '确认删除此账号'}
                </Button>
              </div>
            ) : (
              <div className="mt-3">
                <Button onClick={() => setConfirmDelete(true)}>删除此账号</Button>
              </div>
            )}
          </Card>
        </div>
      ) : null}
    </Drawer>
  );
}

