export interface QuotaWindowView {
  label: string;
  usedPercent: number;
  availablePercent: number;
  resetAt: string | null;
  windowSeconds: number | null;
}

export interface QuotaDetailsView {
  primaryWindow: QuotaWindowView | null;
  secondaryWindow: QuotaWindowView | null;
  windows: QuotaWindowView[];
}

function asRecord(input: unknown): Record<string, unknown> | null {
  return input && typeof input === 'object' && !Array.isArray(input) ? (input as Record<string, unknown>) : null;
}

function asNumber(input: unknown): number | null {
  if (typeof input === 'number' && Number.isFinite(input)) {
    return input;
  }

  if (typeof input === 'string' && input.trim()) {
    const value = Number(input);
    return Number.isFinite(value) ? value : null;
  }

  return null;
}

function clampPercent(input: number): number {
  return Math.max(0, Math.min(100, input));
}

function toResetDate(input: unknown): string | null {
  const value = asNumber(input);
  if (value === null || value <= 0) {
    return null;
  }

  const millis = value > 10_000_000_000 ? value : value * 1000;
  const date = new Date(millis);
  return Number.isNaN(date.getTime()) ? null : date.toISOString();
}

function labelForWindow(seconds: number | null, fallback: string): string {
  if (seconds === 18_000) {
    return '5 小时额度';
  }
  if (seconds === 604_800) {
    return '7 日额度';
  }
  if (seconds && seconds > 0) {
    const hours = seconds / 3600;
    if (hours < 24) {
      return `${Math.round(hours)} 小时额度`;
    }
    return `${Math.round(hours / 24)} 日额度`;
  }
  return fallback;
}

function parseWindow(input: unknown, fallback: string): QuotaWindowView | null {
  const record = asRecord(input);
  if (!record) {
    return null;
  }

  const usedPercent = asNumber(record.used_percent ?? record.usedPercent);
  if (usedPercent === null) {
    return null;
  }

  const windowSeconds = asNumber(record.limit_window_seconds ?? record.limitWindowSeconds);
  const normalizedUsed = clampPercent(usedPercent);

  return {
    label: labelForWindow(windowSeconds, fallback),
    usedPercent: normalizedUsed,
    availablePercent: clampPercent(100 - normalizedUsed),
    resetAt: toResetDate(record.reset_at ?? record.resetAt),
    windowSeconds,
  };
}

export function parseQuotaDetails(rawData: unknown): QuotaDetailsView {
  const root = asRecord(rawData);
  const rateLimit = asRecord(root?.rate_limit ?? root?.rateLimit);
  const primaryWindow = parseWindow(rateLimit?.primary_window ?? rateLimit?.primaryWindow ?? root, '5 小时额度');
  const secondaryWindow = parseWindow(rateLimit?.secondary_window ?? rateLimit?.secondaryWindow, '7 日额度');

  return {
    primaryWindow,
    secondaryWindow,
    windows: [primaryWindow, secondaryWindow].filter((window): window is QuotaWindowView => Boolean(window)),
  };
}
