import { describe, expect, it } from 'vitest';
import { adaptStatsResponse } from '../lib/stats';

describe('adaptStatsResponse', () => {
  it('maps summary and accounts safely', () => {
    const adapted = adaptStatsResponse({
      summary: {
        total: 3,
        active: 2,
        cooldown: 1,
        disabled: 0,
        rpm: 5,
        token_overview: {
          today: { total_tokens: 12, request_count: 2 },
        },
      },
      accounts: [{ email: 'a@example.com', status: 'active', usage: { total_tokens: 12, today_total_tokens: 3 } }],
      pagination: { page: 1, page_size: 20, filtered_total: 1, total_pages: 1 },
    });

    expect(adapted.summary.total).toBe(3);
    expect(adapted.summary.tokenOverview?.today.totalTokens).toBe(12);
    expect(adapted.accounts[0].email).toBe('a@example.com');
    expect(adapted.accounts[0].usage.totalTokens).toBe(12);
    expect(adapted.accounts[0].usage.todayTotalTokens).toBe(3);
  });
});
