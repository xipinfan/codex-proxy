import type {
  AccountDeletePayload,
  ApiErrorShape,
  ConsoleSettings,
  IngestResult,
  OAuthPollResponse,
  OAuthStartResponse,
  ProgressEvent,
  StatsQuery,
  StatsResponse,
  TokenFilePayload,
} from './types';

export class ApiError extends Error {
  status: number;

  constructor(message: string, status: number) {
    super(message);
    this.name = 'ApiError';
    this.status = status;
  }
}

export async function fetchStats(settings: ConsoleSettings, params: StatsQuery): Promise<StatsResponse> {
  const search = new URLSearchParams({
    page: String(params.page),
    page_size: String(params.pageSize),
    include_quota: String(params.includeQuota),
  });

  if (params.query) {
    search.set('q', params.query);
  }

  return requestJSON<StatsResponse>(settings, `/stats?${search.toString()}`);
}

export async function ingestAccounts(
  settings: ConsoleSettings,
  payload: TokenFilePayload[],
): Promise<IngestResult> {
  return requestJSON<IngestResult>(settings, '/admin/accounts/ingest', {
    method: 'POST',
    headers: {
      'Content-Type': 'application/json',
    },
    body: JSON.stringify(payload),
  });
}

export async function startCodexOAuth(settings: ConsoleSettings): Promise<OAuthStartResponse> {
  return requestJSON<OAuthStartResponse>(settings, '/oauth/codex/start', {
    method: 'POST',
  });
}

export async function pollCodexOAuth(settings: ConsoleSettings, state: string): Promise<OAuthPollResponse> {
  const search = new URLSearchParams({ state });
  return requestJSON<OAuthPollResponse>(settings, `/oauth/codex/result?${search.toString()}`);
}

export async function completeCodexOAuth(settings: ConsoleSettings, callbackUrl: string): Promise<IngestResult> {
  return requestJSON<IngestResult>(settings, '/oauth/codex/complete', {
    method: 'POST',
    headers: {
      'Content-Type': 'application/json',
    },
    body: JSON.stringify({ callback_url: callbackUrl }),
  });
}

export async function deleteAccount(settings: ConsoleSettings, payload: AccountDeletePayload): Promise<void> {
  await requestJSON(settings, '/admin/accounts/delete', {
    method: 'POST',
    headers: {
      'Content-Type': 'application/json',
    },
    body: JSON.stringify(payload),
  });
}

export async function runProgressAction(
  settings: ConsoleSettings,
  path: '/refresh' | '/check-quota',
  onEvent?: (event: ProgressEvent) => void,
): Promise<ProgressEvent> {
  const url = `${resolveBaseUrl(settings.baseUrl)}${path}`;
  const response = await fetch(url, {
    method: 'POST',
    headers: buildHeaders(settings),
  });

  if (!response.ok) {
    throw await toApiError(response);
  }

  if (!response.body) {
    throw new ApiError('流式响应体为空', response.status);
  }

  const reader = response.body.getReader();
  const decoder = new TextDecoder();
  let buffer = '';
  let lastDone: ProgressEvent | null = null;

  while (true) {
    const { done, value } = await reader.read();
    if (done) {
      break;
    }

    buffer += decoder.decode(value, { stream: true });
    const chunks = buffer.split('\n\n');
    buffer = chunks.pop() ?? '';

    for (const chunk of chunks) {
      const parsed = parseProgressEventChunk(chunk);
      if (!parsed) {
        continue;
      }
      onEvent?.(parsed);
      if (parsed.type === 'done') {
        lastDone = parsed;
      }
    }
  }

  buffer += decoder.decode();
  if (buffer.trim()) {
    const parsed = parseProgressEventChunk(buffer);
    if (parsed) {
      onEvent?.(parsed);
      if (parsed.type === 'done') {
        lastDone = parsed;
      }
    }
  }

  if (!lastDone) {
    throw new ApiError('未从进度流中收到完成事件', response.status);
  }

  return lastDone;
}

export function buildHeaders(
  settings: ConsoleSettings,
  initHeaders: HeadersInit = {},
): Record<string, string> {
  const headers = normalizeHeaders(initHeaders);

  if (!headers.Accept) {
    headers.Accept = 'application/json';
  }

  if (settings.apiKey && !headers.Authorization && !headers['x-api-key']) {
    headers.Authorization = `Bearer ${settings.apiKey}`;
  }

  return headers;
}

export function resolveBaseUrl(baseUrl: string): string {
  const trimmed = baseUrl.trim();
  if (trimmed) {
    return trimmed.replace(/\/+$/, '');
  }

  if (typeof window !== 'undefined' && window.location.origin) {
    return window.location.origin.replace(/\/+$/, '');
  }

  return 'http://127.0.0.1:8080';
}

async function requestJSON<T>(
  settings: ConsoleSettings,
  path: string,
  init: RequestInit = {},
): Promise<T> {
  const url = `${resolveBaseUrl(settings.baseUrl)}${path.startsWith('/') ? path : `/${path}`}`;
  const response = await fetch(url, {
    ...init,
    headers: buildHeaders(settings, init.headers),
  });

  if (!response.ok) {
    throw await toApiError(response);
  }

  return (await response.json()) as T;
}

async function toApiError(response: Response): Promise<ApiError> {
  let message = `${response.status} ${response.statusText}`.trim();

  try {
    const data = (await response.json()) as ApiErrorShape;
    if (data?.error?.message) {
      message = data.error.message;
    }
  } catch {
    try {
      const text = await response.text();
      if (text.trim()) {
        message = text.trim();
      }
    } catch {
      // Ignore secondary parsing failures and fall back to the status line.
    }
  }

  return new ApiError(message, response.status);
}

function normalizeHeaders(headers: HeadersInit): Record<string, string> {
  if (headers instanceof Headers) {
    return Object.fromEntries(headers.entries());
  }

  if (Array.isArray(headers)) {
    return Object.fromEntries(headers);
  }

  return { ...headers };
}

function parseProgressEventChunk(chunk: string): ProgressEvent | null {
  const lines = chunk
    .split(/\r?\n/)
    .map((line) => line.trim())
    .filter(Boolean);

  if (lines.length === 0) {
    return null;
  }

  const dataLine = lines.find((line) => line.startsWith('data:'));
  if (!dataLine) {
    return null;
  }

  const payload = dataLine.slice('data:'.length).trim();
  const parsed = JSON.parse(payload) as Record<string, unknown>;

  return {
    type: String(parsed.type ?? ''),
    email: typeof parsed.email === 'string' ? parsed.email : undefined,
    success: typeof parsed.success === 'boolean' ? parsed.success : undefined,
    message: typeof parsed.message === 'string' ? parsed.message : undefined,
    total: typeof parsed.total === 'number' ? parsed.total : undefined,
    successCount:
      typeof parsed.success_count === 'number'
        ? parsed.success_count
        : typeof parsed.successCount === 'number'
          ? parsed.successCount
          : undefined,
    failedCount:
      typeof parsed.failed_count === 'number'
        ? parsed.failed_count
        : typeof parsed.failedCount === 'number'
          ? parsed.failedCount
          : undefined,
    remaining: typeof parsed.remaining === 'number' ? parsed.remaining : undefined,
    duration: typeof parsed.duration === 'string' ? parsed.duration : undefined,
    current: typeof parsed.current === 'number' ? parsed.current : undefined,
  };
}
