import { render, screen } from '@testing-library/react';
import { describe, expect, it, vi } from 'vitest';
import { TokenOverviewDrawer } from '../features/token-overview/TokenOverviewDrawer';

describe('TokenOverviewDrawer', () => {
  it('uses compact Chinese units in narrow period cards while preserving full titles', () => {
    render(
      <TokenOverviewDrawer
        open
        onClose={vi.fn()}
        accounts={[]}
        summary={{
          total: 0,
          active: 0,
          cooldown: 0,
          disabled: 0,
          rpm: 0,
          totalInputTokens: 0,
          totalOutputTokens: 0,
          tokenOverview: {
            today: { inputTokens: 133_456_439, outputTokens: 599_878, totalTokens: 134_056_317, requestCount: 1_526 },
            sevenDays: { inputTokens: 133_456_495, outputTokens: 599_969, totalTokens: 134_056_464, requestCount: 1_530 },
            thirtyDays: { inputTokens: 133_456_495, outputTokens: 599_969, totalTokens: 134_056_464, requestCount: 1_530 },
            lifetime: { inputTokens: 133_456_495, outputTokens: 599_969, totalTokens: 134_056_464, requestCount: 1_530 },
            updatedAt: null,
          },
        }}
      />,
    );

    expect(screen.getByText('134,056,317')).toBeInTheDocument();
    expect(screen.getAllByText('1.34亿').length).toBeGreaterThanOrEqual(3);
    expect(screen.getAllByText('1.33亿').length).toBeGreaterThanOrEqual(3);
    expect(screen.getAllByTitle('134,056,464').length).toBeGreaterThanOrEqual(3);
  });
});
