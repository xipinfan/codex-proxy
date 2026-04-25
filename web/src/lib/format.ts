const dateTimeFormatter = new Intl.DateTimeFormat('zh-CN', {
  month: '2-digit',
  day: '2-digit',
  hour: '2-digit',
  minute: '2-digit',
});

export function formatNumber(value: number): string {
  return new Intl.NumberFormat('zh-CN').format(value);
}

function trimFixed(value: number, fractionDigits: number): string {
  return value.toFixed(fractionDigits).replace(/\.0+$/, '').replace(/(\.\d*[1-9])0+$/, '$1');
}

export function formatCompactNumber(value: number): string {
  const abs = Math.abs(value);
  if (abs < 10_000) {
    return formatNumber(value);
  }

  if (abs < 100_000_000) {
    const fractionDigits = abs >= 100_000 ? (abs >= 1_000_000 ? 1 : 0) : 1;
    return `${trimFixed(value / 10_000, fractionDigits)}万`;
  }

  return `${trimFixed(value / 100_000_000, 2)}亿`;
}

export function formatTokenCompact(value: number): string {
  return formatCompactNumber(value);
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

export function formatAvailabilityReason(reason: string): string {
  if (reason === 'cooldown') {
    return '冷却中';
  }
  if (reason === 'disabled') {
    return '已停用';
  }
  if (reason === 'token_expiring') {
    return '令牌即将过期';
  }
  return '可调度';
}

export function formatDurationMs(value: number): string {
  if (!Number.isFinite(value) || value <= 0) {
    return '0 秒';
  }

  const totalSeconds = Math.ceil(value / 1000);
  const minutes = Math.floor(totalSeconds / 60);
  const seconds = totalSeconds % 60;
  if (minutes <= 0) {
    return `${seconds} 秒`;
  }
  return seconds > 0 ? `${minutes} 分 ${seconds} 秒` : `${minutes} 分`;
}

export function formatPercent(value: number | null): string {
  if (value === null || Number.isNaN(value)) {
    return '待检查';
  }

  return `${Math.round(value)}%`;
}
