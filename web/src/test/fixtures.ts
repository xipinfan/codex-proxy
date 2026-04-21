import type { AccountView, QuotaView, StatsView } from '../lib/types';

export const sampleQuota: QuotaView = {
  valid: true,
  statusCode: 200,
  checkedAt: '2026-04-21T12:30:00Z',
  rawData: {
    limit: 100,
    used: 64,
    used_percent: 64,
    resets_at: '2026-04-22T00:00:00Z',
  },
};

export const sampleAccount: AccountView = {
  id: 'a@example.com',
  email: 'a@example.com',
  status: 'active',
  planType: 'pro',
  disableReason: '',
  totalRequests: 42,
  totalErrors: 2,
  consecutiveFailures: 0,
  lastUsedAt: '2026-04-21T11:00:00Z',
  lastRefreshedAt: '2026-04-21T10:00:00Z',
  cooldownUntil: '',
  quotaExhausted: false,
  quotaResetsAt: '2026-04-22T00:00:00Z',
  tokenExpire: '2026-04-30T00:00:00Z',
  usage: {
    totalCompletions: 19,
    inputTokens: 3200,
    outputTokens: 1800,
    totalTokens: 5000,
  },
  quota: sampleQuota,
};

export const sampleStats: StatsView = {
  summary: {
    total: 3,
    active: 2,
    cooldown: 1,
    disabled: 0,
    rpm: 5,
    totalInputTokens: 9200,
    totalOutputTokens: 4100,
  },
  accounts: [
    sampleAccount,
    {
      ...sampleAccount,
      id: 'b@example.com',
      email: 'b@example.com',
      status: 'cooldown',
      planType: 'plus',
      quotaExhausted: true,
      quotaResetsAt: '2026-04-21T20:00:00Z',
    },
  ],
  pagination: {
    page: 1,
    pageSize: 20,
    total: 3,
    filteredTotal: 2,
    totalPages: 1,
    returned: 2,
    hasPrev: false,
    hasNext: false,
    query: '',
  },
};
