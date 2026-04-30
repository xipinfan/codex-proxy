import type {
  AccountStatsResponse,
  AccountStatus,
  AccountView,
  PaginationView,
  QuotaInfoResponse,
  QuotaView,
  StatsResponse,
  StatsView,
  SummaryView,
  UsageStatsResponse,
  UsageView,
  TokenBucketResponse,
  TokenBucketView,
  TokenOverviewView,
} from './types';

function toNumber(input: unknown): number {
  const value = Number(input ?? 0);
  return Number.isFinite(value) ? value : 0;
}

function toDateString(input: unknown): string | null {
  if (typeof input !== 'string') {
    return null;
  }

  const value = input.trim();
  return value ? value : null;
}

function toStatus(input: unknown): AccountStatus {
  if (input === 'active' || input === 'cooldown' || input === 'disabled') {
    return input;
  }

  return 'unknown';
}

function toUsageView(input: UsageStatsResponse | undefined): UsageView {
  return {
    totalCompletions: toNumber(input?.total_completions),
    inputTokens: toNumber(input?.input_tokens),
    outputTokens: toNumber(input?.output_tokens),
    totalTokens: toNumber(input?.total_tokens),
    todayInputTokens: toNumber(input?.today_input_tokens),
    todayOutputTokens: toNumber(input?.today_output_tokens),
    todayTotalTokens: toNumber(input?.today_total_tokens),
    todayRequestCount: toNumber(input?.today_request_count),
    sevenDayInputTokens: toNumber(input?.seven_day_input_tokens),
    sevenDayOutputTokens: toNumber(input?.seven_day_output_tokens),
    sevenDayTotalTokens: toNumber(input?.seven_day_total_tokens),
    sevenDayRequestCount: toNumber(input?.seven_day_request_count),
    thirtyDayInputTokens: toNumber(input?.thirty_day_input_tokens),
    thirtyDayOutputTokens: toNumber(input?.thirty_day_output_tokens),
    thirtyDayTotalTokens: toNumber(input?.thirty_day_total_tokens),
    thirtyDayRequestCount: toNumber(input?.thirty_day_request_count),
    lifetimeInputTokens: toNumber(input?.lifetime_input_tokens) || toNumber(input?.input_tokens),
    lifetimeOutputTokens: toNumber(input?.lifetime_output_tokens) || toNumber(input?.output_tokens),
    lifetimeTotalTokens: toNumber(input?.lifetime_total_tokens) || toNumber(input?.total_tokens),
    lifetimeRequestCount: toNumber(input?.lifetime_request_count) || toNumber(input?.total_completions),
  };
}

function toTokenBucketView(input: TokenBucketResponse | undefined): TokenBucketView {
  return {
    inputTokens: toNumber(input?.input_tokens),
    outputTokens: toNumber(input?.output_tokens),
    totalTokens: toNumber(input?.total_tokens),
    requestCount: toNumber(input?.request_count),
  };
}

function toTokenOverviewView(input: StatsResponse['summary']): TokenOverviewView {
  const totalInput = toNumber(input?.total_input_tokens);
  const totalOutput = toNumber(input?.total_output_tokens);
  const lifetime = toTokenBucketView(input?.token_overview?.lifetime);

  return {
    today: toTokenBucketView(input?.token_overview?.today),
    sevenDays: toTokenBucketView(input?.token_overview?.seven_days),
    thirtyDays: toTokenBucketView(input?.token_overview?.thirty_days),
    lifetime: {
      ...lifetime,
      inputTokens: lifetime.inputTokens || totalInput,
      outputTokens: lifetime.outputTokens || totalOutput,
      totalTokens: lifetime.totalTokens || totalInput + totalOutput,
    },
    updatedAt: toDateString(input?.token_overview?.updated_at),
  };
}

function toQuotaView(input: QuotaInfoResponse | null | undefined): QuotaView | null {
  if (!input) {
    return null;
  }

  return {
    valid: Boolean(input.valid),
    statusCode: toNumber(input.status_code),
    checkedAt: toDateString(input.checked_at),
    rawData: input.raw_data ?? null,
  };
}

function toAccountView(input: AccountStatsResponse): AccountView {
  const email = String(input?.email ?? 'unknown');
  const accountId = typeof input?.account_id === 'string' && input.account_id.trim() ? input.account_id : email;

  return {
    id: accountId,
    email,
    status: toStatus(input?.status),
    planType: String(input?.plan_type ?? ''),
    disableReason: String(input?.disable_reason ?? ''),
    totalRequests: toNumber(input?.total_requests),
    totalErrors: toNumber(input?.total_errors),
    consecutiveFailures: toNumber(input?.consecutive_failures),
    lastUsedAt: toDateString(input?.last_used_at),
    lastRefreshedAt: toDateString(input?.last_refreshed_at),
    cooldownUntil: toDateString(input?.cooldown_until),
    quotaExhausted: Boolean(input?.quota_exhausted),
    quotaResetsAt: toDateString(input?.quota_resets_at),
    tokenExpire: String(input?.token_expire ?? ''),
    usage: toUsageView(input?.usage),
    quota: toQuotaView(input?.quota),
  };
}

function toSummaryView(input: StatsResponse['summary']): SummaryView {
  return {
    total: toNumber(input?.total),
    active: toNumber(input?.active),
    cooldown: toNumber(input?.cooldown),
    disabled: toNumber(input?.disabled),
    rpm: toNumber(input?.rpm),
    totalInputTokens: toNumber(input?.total_input_tokens),
    totalOutputTokens: toNumber(input?.total_output_tokens),
    tokenOverview: toTokenOverviewView(input),
  };
}

function toPaginationView(input: StatsResponse['pagination']): PaginationView | null {
  if (!input) {
    return null;
  }

  return {
    page: toNumber(input?.page) || 1,
    pageSize: toNumber(input?.page_size) || 20,
    total: toNumber(input?.total),
    filteredTotal: toNumber(input?.filtered_total),
    totalPages: toNumber(input?.total_pages) || 1,
    returned: toNumber(input?.returned),
    hasPrev: Boolean(input?.has_prev),
    hasNext: Boolean(input?.has_next),
    query: String(input?.query ?? ''),
  };
}

export function adaptStatsResponse(input: StatsResponse): StatsView {
  return {
    summary: toSummaryView(input?.summary),
    accounts: Array.isArray(input?.accounts) ? input.accounts.map(toAccountView) : [],
    pagination: toPaginationView(input?.pagination),
  };
}
