import { useCallback, useEffect, useMemo, useState } from 'react';

import { Button } from '../../components/ui/Button';
import { Card } from '../../components/ui/Card';
import { EmptyState } from '../../components/ui/EmptyState';
import { ErrorState } from '../../components/ui/ErrorState';
import { AccountDetailDrawer } from '../account-detail/AccountDetailDrawer';
import { OAuthImportDialog } from '../oauth-import/OAuthImportDialog';
import { SettingsDialog } from '../settings/SettingsDialog';
import { AccountsTable } from './AccountsTable';
import { StatsOverview } from './StatsOverview';
import { fetchStats, ingestAccounts, runProgressAction } from '../../lib/api';
import { formatCompactNumber } from '../../lib/format';
import { ingestAccountFromOAuth } from '../../lib/oauth';
import { loadConsoleSettings, saveConsoleSettings } from '../../lib/storage';
import { adaptStatsResponse } from '../../lib/stats';
import type { AccountView, ConsoleSettings, IngestResult, ProgressEvent, StatsQuery, StatsView, SummaryView, TokenFilePayload } from '../../lib/types';

type DashboardPreviewState = 'live' | 'ready' | 'empty' | 'error';
type RequestState = 'loading' | 'ready' | 'empty' | 'error';

interface DashboardPageProps {
  state?: DashboardPreviewState;
  errorMessage?: string;
  initialStats?: StatsView;
  initialSettings?: ConsoleSettings;
  summary?: SummaryView;
  accounts?: AccountView[];
  onOpenImport?: () => void;
}

const emptyStats: StatsView = {
  summary: {
    total: 0,
    active: 0,
    cooldown: 0,
    disabled: 0,
    rpm: 0,
    totalInputTokens: 0,
    totalOutputTokens: 0,
  },
  accounts: [],
  pagination: {
    page: 1,
    pageSize: 20,
    total: 0,
    filteredTotal: 0,
    totalPages: 1,
    returned: 0,
    hasPrev: false,
    hasNext: false,
    query: '',
  },
};

function toStatus(state: DashboardPreviewState, stats: StatsView): RequestState {
  if (state === 'error') {
    return 'error';
  }
  if (state === 'empty') {
    return 'empty';
  }
  if (state === 'ready') {
    return stats.accounts.length === 0 ? 'empty' : 'ready';
  }

  return 'loading';
}

function buildQuery(page: number, settings: ConsoleSettings, query: string): StatsQuery {
  return {
    page,
    pageSize: settings.pageSize,
    query,
    includeQuota: settings.includeQuota,
  };
}

export function DashboardPage({
  state = 'live',
  errorMessage = '暂时无法加载统计信息。',
  initialStats = emptyStats,
  initialSettings,
  summary,
  accounts,
  onOpenImport,
}: DashboardPageProps) {
  const controlledStats = useMemo<StatsView>(() => {
    if (!summary && !accounts) {
      return initialStats;
    }

    return {
      summary: summary ?? emptyStats.summary,
      accounts: accounts ?? [],
      pagination: initialStats.pagination ?? emptyStats.pagination,
    };
  }, [accounts, initialStats, summary]);
  const controlledMode = Boolean(summary || accounts || onOpenImport);
  const effectiveState = controlledMode ? (errorMessage ? 'error' : (controlledStats.accounts.length === 0 ? 'empty' : 'ready')) : state;
  const isPreview = controlledMode || effectiveState !== 'live';
  const [settings, setSettings] = useState<ConsoleSettings>(() => initialSettings ?? loadConsoleSettings());
  const [stats, setStats] = useState<StatsView>(controlledStats);
  const [requestState, setRequestState] = useState<RequestState>(() => toStatus(effectiveState, controlledStats));
  const [requestError, setRequestError] = useState(errorMessage);
  const [isRefreshing, setIsRefreshing] = useState(false);
  const [query, setQuery] = useState('');
  const [page, setPage] = useState(1);
  const [selectedAccountId, setSelectedAccountId] = useState<string | null>(null);
  const [settingsOpen, setSettingsOpen] = useState(false);
  const [oauthOpen, setOauthOpen] = useState(false);
  const [actionMessage, setActionMessage] = useState<{ tone: 'success' | 'error' | 'info'; text: string } | null>(null);
  const [streamAction, setStreamAction] = useState<{ kind: 'refresh' | 'quota'; text: string } | null>(null);

  const selectedAccount = useMemo(
    () => stats.accounts.find((account) => account.id === selectedAccountId) ?? null,
    [selectedAccountId, stats.accounts],
  );

  const loadStats = useCallback(
    async (nextQuery: StatsQuery, nextSettings = settings) => {
      setIsRefreshing(true);
      setRequestError('');
      if (stats.accounts.length === 0) {
        setRequestState('loading');
      }

      try {
        const payload = await fetchStats(nextSettings, nextQuery);
        const nextStats = adaptStatsResponse(payload);
        setStats(nextStats);
        setRequestState(nextStats.accounts.length === 0 ? 'empty' : 'ready');
      } catch (error) {
        const message = error instanceof Error ? error.message : '加载统计信息失败。';
        setRequestError(message);
        setRequestState('error');
      } finally {
        setIsRefreshing(false);
      }
    },
    [settings, stats.accounts.length],
  );

  useEffect(() => {
    if (isPreview) {
      setStats(controlledStats);
      setRequestError(errorMessage);
      setRequestState(toStatus(effectiveState, controlledStats));
      return;
    }

    void loadStats(buildQuery(page, settings, query));
  }, [controlledStats, effectiveState, errorMessage, isPreview, loadStats, page, query, settings]);

  useEffect(() => {
    if (isPreview || settings.autoRefreshSeconds <= 0) {
      return undefined;
    }

    const interval = window.setInterval(() => {
      void loadStats(buildQuery(page, settings, query));
    }, settings.autoRefreshSeconds * 1000);

    return () => window.clearInterval(interval);
  }, [isPreview, loadStats, page, query, settings]);

  useEffect(() => {
    if (!selectedAccountId) {
      return;
    }

    const stillExists = stats.accounts.some((account) => account.id === selectedAccountId);
    if (!stillExists) {
      setSelectedAccountId(null);
    }
  }, [selectedAccountId, stats.accounts]);

  async function handleRefresh() {
    await loadStats(buildQuery(page, settings, query));
  }

  function describeProgress(kind: 'refresh' | 'quota', event: ProgressEvent): string {
    if (event.type === 'item') {
      const current = event.current ?? 0;
      const total = event.total ?? 0;
      const email = event.email ?? '未知账号';
      return kind === 'refresh'
        ? `正在刷新 ${email}（${current}/${total}）`
        : `正在检查 ${email} 的额度（${current}/${total}）`;
    }

    return kind === 'refresh'
      ? `刷新完成，成功 ${event.successCount ?? 0} 个，失败 ${event.failedCount ?? 0} 个。`
      : `额度检查完成，成功 ${event.successCount ?? 0} 个，失败 ${event.failedCount ?? 0} 个。`;
  }

  async function handleProgressAction(kind: 'refresh' | 'quota') {
    setStreamAction({
      kind,
      text: kind === 'refresh' ? '正在准备刷新任务流...' : '正在准备额度检查流...',
    });
    setActionMessage({
      tone: 'info',
      text: kind === 'refresh' ? '开始刷新账号数据...' : '开始检查额度...',
    });

    try {
      const done = await runProgressAction(
        settings,
        kind === 'refresh' ? '/refresh' : '/check-quota',
        (event) => {
          setStreamAction({
            kind,
            text: describeProgress(kind, event),
          });
        },
      );

      setActionMessage({
        tone: done.failedCount && done.failedCount > 0 ? 'error' : 'success',
        text: describeProgress(kind, done),
      });
      await loadStats(buildQuery(page, settings, query));
    } catch (error) {
      setActionMessage({
        tone: 'error',
        text: error instanceof Error ? error.message : '执行失败。',
      });
    } finally {
      setStreamAction(null);
    }
  }

  function handleSaveSettings(next: ConsoleSettings) {
    saveConsoleSettings(next);
    setSettings(next);
    setPage(1);
    setActionMessage({ tone: 'success', text: '设置已保存，面板将按新配置刷新。' });
  }

  async function handleImport(callbackUrl: string): Promise<IngestResult> {
    const result = await ingestAccountFromOAuth(settings, callbackUrl);
    setActionMessage({
      tone: result.failed > 0 ? 'error' : 'success',
      text:
        result.failed > 0
          ? `已导入 ${result.added + result.updated} 个账号，失败 ${result.failed} 个。`
          : `已成功导入 ${result.added + result.updated} 个账号。`,
    });
    setPage(1);
    await loadStats(buildQuery(1, settings, query));
    return result;
  }

  async function handleManualImport(payload: TokenFilePayload): Promise<IngestResult> {
    const result = await ingestAccounts(settings, [payload]);
    setActionMessage({
      tone: result.failed > 0 ? 'error' : 'success',
      text:
        result.failed > 0
          ? `已导入 ${result.added + result.updated} 个账号，失败 ${result.failed} 个。`
          : `已成功导入 ${result.added + result.updated} 个账号。`,
    });
    setPage(1);
    await loadStats(buildQuery(1, settings, query));
    return result;
  }

  return (
    <main className="mx-auto flex min-h-screen w-full max-w-[1480px] flex-col gap-6 px-4 py-5 sm:px-6 sm:py-8 lg:px-10">
      <header className="rounded-[32px] border border-white/50 bg-[color:var(--bg-surface)] p-6 shadow-[var(--shadow-soft)] backdrop-blur-xl">
        <div className="flex flex-col gap-6 lg:flex-row lg:items-end lg:justify-between">
          <div className="space-y-4">
            <div className="inline-flex items-center gap-3 rounded-full bg-white/66 px-4 py-2 text-xs font-semibold uppercase tracking-[0.18em] text-[color:var(--text-secondary)]">
              <span className="h-2 w-2 rounded-full bg-[color:var(--accent-cyan)]" />
              账号运维控制台
            </div>
            <div className="space-y-2">
              <h1 className="text-4xl font-semibold tracking-[-0.05em] sm:text-5xl">Codex 账号运维面板</h1>
              <p className="max-w-2xl text-base leading-7 text-[color:var(--text-secondary)]">
                主表、抽屉和额度面板共用一套玻璃层级，方便你在一个工作面里看清池子健康度、活跃账号和导入动作。
              </p>
            </div>
          </div>

          <div className="flex flex-col items-stretch gap-3 sm:flex-row sm:flex-wrap sm:justify-end">
            <Button variant="secondary" onClick={() => setSettingsOpen(true)}>
              设置
            </Button>
            <Button variant="secondary" onClick={() => setOauthOpen(true)}>
              导入账号
            </Button>
            <Button
              variant="secondary"
              onClick={() => void handleProgressAction('quota')}
              disabled={isRefreshing || Boolean(streamAction)}
            >
              {streamAction?.kind === 'quota' ? '检查中...' : '检查额度'}
            </Button>
            {onOpenImport ? (
              <Button variant="secondary" onClick={onOpenImport}>
                导入账号
              </Button>
            ) : null}
            <Button
              onClick={() => void handleProgressAction('refresh')}
              disabled={isRefreshing || Boolean(streamAction)}
            >
              {streamAction?.kind === 'refresh' ? '刷新中...' : '刷新数据'}
            </Button>
          </div>
        </div>

        <div className="mt-6 flex flex-wrap items-center gap-3 text-sm text-[color:var(--text-secondary)]">
          <span className="rounded-full border border-[color:var(--border-soft)] bg-white/70 px-3 py-1">接口地址 {settings.baseUrl || '当前来源地址'}</span>
          <span className="rounded-full border border-[color:var(--border-soft)] bg-white/70 px-3 py-1">分页大小 {settings.pageSize}</span>
          <span className="rounded-full border border-[color:var(--border-soft)] bg-white/70 px-3 py-1">额度检查 {settings.includeQuota ? '已开启' : '已关闭'}</span>
        </div>

        {actionMessage ? (
          <div
            className={`mt-6 flex items-center justify-between gap-3 rounded-[22px] px-4 py-3 text-sm ${
              actionMessage.tone === 'success'
                ? 'bg-[rgba(59,184,197,0.14)] text-[#14626b]'
                : actionMessage.tone === 'error'
                  ? 'bg-[rgba(207,94,72,0.12)] text-[#8f2e1f]'
                  : 'bg-white/70 text-[color:var(--text-primary)]'
            }`}
          >
            <span>{actionMessage.text}</span>
            <Button variant="ghost" onClick={() => setActionMessage(null)}>
              关闭提示
            </Button>
          </div>
        ) : null}

        {streamAction ? (
          <div className="mt-4 rounded-[22px] bg-white/70 px-4 py-3 text-sm text-[color:var(--text-primary)]">
            {streamAction.text}
          </div>
        ) : null}
      </header>

      {requestState === 'loading' ? (
        <section className="grid gap-4 xl:grid-cols-[repeat(6,minmax(0,1fr))]">
          {Array.from({ length: 6 }).map((_, index) => (
            <Card key={index} className="h-[138px] animate-pulse rounded-[30px] bg-white/55">
              <span className="sr-only">加载中</span>
            </Card>
          ))}
        </section>
      ) : (
        <StatsOverview summary={stats.summary} />
      )}

      <Card className="overflow-hidden rounded-[32px] p-0">
        <div className="flex flex-col gap-4 border-b border-[rgba(122,91,62,0.08)] px-6 py-5 lg:flex-row lg:items-center lg:justify-between">
          <div>
            <p className="text-xs uppercase tracking-[0.22em] text-[color:var(--text-secondary)]">账号列表</p>
            <h2 className="mt-2 text-2xl font-semibold tracking-[-0.04em]">账号池概览</h2>
          </div>
          <div className="flex flex-col gap-3 sm:flex-row sm:items-center">
            <input
              className="console-field w-full min-w-[260px]"
              value={query}
              onChange={(event) => {
                setPage(1);
                setQuery(event.target.value);
              }}
              placeholder="按邮箱搜索"
            />
            <span className="text-sm text-[color:var(--text-secondary)]">
              当前显示 {formatCompactNumber(stats.accounts.length)} / {formatCompactNumber(stats.pagination?.filteredTotal ?? stats.accounts.length)}
            </span>
          </div>
        </div>

        {requestState === 'error' ? (
          <div className="p-6">
            <ErrorState message={requestError} onRetry={() => void handleRefresh()} />
          </div>
        ) : requestState === 'empty' ? (
          <div className="p-6">
            <EmptyState onImport={() => setOauthOpen(true)} />
          </div>
        ) : (
          <>
            <div className="overflow-x-auto">
              <table className="min-w-full border-collapse">
                <thead>
                  <tr className="bg-[rgba(255,255,255,0.44)] text-left text-xs uppercase tracking-[0.18em] text-[color:var(--text-secondary)]">
                    <th className="px-6 py-4 font-medium">账号</th>
                    <th className="px-4 py-4 font-medium">状态</th>
                    <th className="px-4 py-4 font-medium">请求数</th>
                    <th className="px-4 py-4 font-medium">错误数</th>
                    <th className="px-4 py-4 font-medium">Token 数</th>
                    <th className="px-4 py-4 font-medium">最近使用</th>
                    <th className="px-4 py-4 font-medium">最近刷新</th>
                    <th className="px-4 py-4 font-medium">额度</th>
                  </tr>
                </thead>
                <tbody>
                  <AccountsTable accounts={stats.accounts} selectedAccountId={selectedAccountId} onSelect={(accountId: string) => setSelectedAccountId(accountId)} />
                </tbody>
              </table>
            </div>

            <div className="flex flex-col gap-3 border-t border-[rgba(122,91,62,0.08)] px-6 py-4 text-sm text-[color:var(--text-secondary)] sm:flex-row sm:items-center sm:justify-between">
              <span>
                第 {stats.pagination?.page ?? page} 页 / 共 {stats.pagination?.totalPages ?? 1} 页
              </span>
              <div className="flex items-center gap-3">
                <Button variant="secondary" disabled={page <= 1 || isRefreshing} onClick={() => setPage((current) => Math.max(1, current - 1))}>
                  上一页
                </Button>
                <Button variant="secondary" disabled={!stats.pagination?.hasNext || isRefreshing} onClick={() => setPage((current) => current + 1)}>
                  下一页
                </Button>
              </div>
            </div>
          </>
        )}
      </Card>

      <AccountDetailDrawer account={selectedAccount} open={Boolean(selectedAccount)} onClose={() => setSelectedAccountId(null)} />
      <SettingsDialog open={settingsOpen} initialValue={settings} onSave={handleSaveSettings} onClose={() => setSettingsOpen(false)} />
      <OAuthImportDialog open={oauthOpen} onClose={() => setOauthOpen(false)} onOAuthImport={handleImport} onManualImport={handleManualImport} />
    </main>
  );
}

