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

  it('keeps explicit zero lifetime values without falling back to legacy totals', () => {
    const adapted = adaptStatsResponse({
      summary: {
        total_input_tokens: 999,
        total_output_tokens: 111,
        token_overview: {
          lifetime: {
            input_tokens: 0,
            output_tokens: 0,
            total_tokens: 0,
            request_count: 0,
          },
        },
      },
      accounts: [
        {
          email: 'zero@example.com',
          usage: {
            input_tokens: 800,
            output_tokens: 200,
            total_tokens: 1000,
            total_completions: 30,
            lifetime_input_tokens: 0,
            lifetime_output_tokens: 0,
            lifetime_total_tokens: 0,
            lifetime_request_count: 0,
          },
        },
      ],
      pagination: null,
    });

    expect(adapted.summary.tokenOverview?.lifetime.inputTokens).toBe(0);
    expect(adapted.summary.tokenOverview?.lifetime.outputTokens).toBe(0);
    expect(adapted.summary.tokenOverview?.lifetime.totalTokens).toBe(0);
    expect(adapted.accounts[0].usage.lifetimeInputTokens).toBe(0);
    expect(adapted.accounts[0].usage.lifetimeOutputTokens).toBe(0);
    expect(adapted.accounts[0].usage.lifetimeTotalTokens).toBe(0);
    expect(adapted.accounts[0].usage.lifetimeRequestCount).toBe(0);
  });

  it('maps account availability fields', () => {
    const adapted = adaptStatsResponse({
      server_time: '2026-04-25T10:07:14+08:00',
      accounts: [
        {
          email: 'cooldown-expired@example.com',
          status: 'active',
          stored_status: 'cooldown',
          pickable: true,
          cooldown_remaining_ms: 0,
          unavailable_reason: '',
        },
      ],
    });

    expect(adapted.serverTime).toBe('2026-04-25T10:07:14+08:00');
    expect(adapted.accounts[0].storedStatus).toBe('cooldown');
    expect(adapted.accounts[0].pickable).toBe(true);
    expect(adapted.accounts[0].cooldownRemainingMs).toBe(0);
    expect(adapted.accounts[0].unavailableReason).toBe('');
  });
});
