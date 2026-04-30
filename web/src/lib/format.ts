const dateTimeFormatter = new Intl.DateTimeFormat('zh-CN', {
  month: '2-digit',
  day: '2-digit',
  hour: '2-digit',
  minute: '2-digit',
});

export function formatNumber(value: number): string {
  return new Intl.NumberFormat('zh-CN').format(value);
}

export function formatCompactNumber(value: number): string {
  return new Intl.NumberFormat('zh-CN', {
    notation: 'compact',
    maximumFractionDigits: value >= 100 ? 0 : 1,
  }).format(value);
}

export function formatTokenCompact(value: number): string {
  const abs = Math.abs(value);
  if (abs < 1_000) {
    return formatNumber(value);
  }
  if (abs < 1_000_000) {
    return `${(value / 1_000).toFixed(abs >= 100_000 ? 0 : 1)}K`;
  }
  if (abs < 1_000_000_000) {
    return `${(value / 1_000_000).toFixed(abs >= 100_000_000 ? 0 : 1)}M`;
  }
  return `${(value / 1_000_000_000).toFixed(abs >= 100_000_000_000 ? 0 : 1)}B`;
}

export function formatTokenFull(value: number): string {
  return formatNumber(value);
}

export function formatDateTime(value: string | null | undefined): string {
  if (!value) {
    return '暂无';
  }

  const date = new Date(value);
  if (Number.isNaN(date.getTime())) {
    return '暂无';
  }

  return dateTimeFormatter.format(date);
}

export function formatStatusLabel(status: string): string {
  if (status === 'active') {
    return '正常';
  }
  if (status === 'cooldown') {
    return '冷却中';
  }
  if (status === 'disabled') {
    return '已停用';
  }

  return status ? `未知状态：${status}` : '未知';
}

export function formatPercent(value: number | null): string {
  if (value === null || Number.isNaN(value)) {
    return '待检查';
  }

  return `${Math.round(value)}%`;
}
