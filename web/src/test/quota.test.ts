import { describe, expect, it } from 'vitest';

import { parseQuotaDetails } from '../lib/quota';

describe('parseQuotaDetails', () => {
  it('extracts 5 hour and 7 day quota windows from wham usage payload', () => {
    const parsed = parseQuotaDetails({
      rate_limit: {
        primary_window: {
          used_percent: 22,
          limit_window_seconds: 18000,
          reset_at: 1776886455,
        },
        secondary_window: {
          used_percent: 3,
          limit_window_seconds: 604800,
          reset_at: 1777473255,
        },
      },
    });

    expect(parsed.primaryWindow?.label).toBe('5 小时额度');
    expect(parsed.primaryWindow?.usedPercent).toBe(22);
    expect(parsed.primaryWindow?.availablePercent).toBe(78);
    expect(parsed.secondaryWindow?.label).toBe('7 日额度');
    expect(parsed.secondaryWindow?.usedPercent).toBe(3);
    expect(parsed.secondaryWindow?.availablePercent).toBe(97);
  });
});
