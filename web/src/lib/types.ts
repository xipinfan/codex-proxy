export type AccountStatus = 'active' | 'cooldown' | 'disabled' | 'unknown';

export interface ConsoleSettings {
  baseUrl: string;
  apiKey: string;
  pageSize: number;
  autoRefreshSeconds: number;
  includeQuota: boolean;
}

export interface SummaryView {
  total: number;
  active: number;
  cooldown: number;
  disabled: number;
  rpm: number;
  totalInputTokens: number;
  totalOutputTokens: number;
}

export interface UsageView {
  totalCompletions: number;
  inputTokens: number;
  outputTokens: number;
  totalTokens: number;
}

export interface QuotaView {
  valid: boolean;
  statusCode: number;
  checkedAt: string | null;
  rawData: Record<string, unknown> | null;
}

export interface AccountView {
  id: string;
  email: string;
  status: AccountStatus | string;
  planType: string;
  disableReason?: string;
  totalRequests?: number;
  totalErrors?: number;
  consecutiveFailures?: number;
  lastUsedAt?: string | null;
  lastRefreshedAt?: string | null;
  cooldownUntil?: string | null;
  quotaExhausted?: boolean;
  quotaResetsAt?: string | null;
  tokenExpire?: string;
  usage: UsageView;
  quota?: QuotaView | null;
}

export interface PaginationView {
  page: number;
  pageSize: number;
  total?: number;
  filteredTotal: number;
  totalPages: number;
  returned?: number;
  hasPrev?: boolean;
  hasNext?: boolean;
  query?: string;
}

export interface StatsView {
  summary: SummaryView;
  accounts: AccountView[];
  pagination: PaginationView | null;
}

export interface OAuthCallbackPayload {
  refreshToken: string;
  accessToken: string;
  idToken: string;
  accountId: string;
  email: string;
  expiresAt: string;
}

export interface StatsQuery {
  page: number;
  pageSize: number;
  query?: string;
  includeQuota: boolean;
}

export interface ApiErrorShape {
  error?: {
    message?: string;
  };
  message?: string;
}

export interface UsageStatsResponse {
  total_completions?: number;
  input_tokens?: number;
  output_tokens?: number;
  total_tokens?: number;
}

export interface QuotaInfoResponse {
  valid?: boolean;
  status_code?: number;
  checked_at?: string;
  raw_data?: Record<string, unknown> | null;
}

export interface AccountStatsResponse {
  account_id?: string;
  email?: string;
  status?: AccountStatus | string;
  plan_type?: string;
  disable_reason?: string;
  total_requests?: number;
  total_errors?: number;
  consecutive_failures?: number;
  last_used_at?: string;
  last_refreshed_at?: string;
  cooldown_until?: string;
  quota_exhausted?: boolean;
  quota_resets_at?: string;
  token_expire?: string;
  usage?: UsageStatsResponse;
  quota?: QuotaInfoResponse | null;
}

export interface StatsResponse {
  summary?: {
    total?: number;
    active?: number;
    cooldown?: number;
    disabled?: number;
    rpm?: number;
    total_input_tokens?: number;
    total_output_tokens?: number;
  };
  accounts?: AccountStatsResponse[];
  pagination?: {
    page?: number;
    page_size?: number;
    total?: number;
    filtered_total?: number;
    total_pages?: number;
    returned?: number;
    has_prev?: boolean;
    has_next?: boolean;
    query?: string;
  } | null;
}

export interface TokenFilePayload {
  type?: string;
  refresh_token?: string;
  rk?: string;
  access_token?: string;
  id_token?: string;
  account_id?: string;
  email?: string;
  expired?: string;
}

export interface IngestResult {
  added: number;
  updated: number;
  failed: number;
  pool_total: number;
  errors?: string[];
}

export interface ProgressEvent {
  type: string;
  email?: string;
  success?: boolean;
  message?: string;
  total?: number;
  successCount?: number;
  failedCount?: number;
  remaining?: number;
  duration?: string;
  current?: number;
}

export const defaultSettings: ConsoleSettings = {
  baseUrl: '',
  apiKey: '',
  pageSize: 20,
  autoRefreshSeconds: 0,
  includeQuota: true,
};
