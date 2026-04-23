import { render, screen } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { describe, expect, it, vi } from 'vitest';
import { AccountsTable } from '../features/dashboard/AccountsTable';
import { DashboardPage } from '../features/dashboard/DashboardPage';

const sampleAccount = {
  id: 'a@example.com',
  email: 'a@example.com',
  status: 'active',
  planType: 'plus',
  quota: {
    valid: true,
    statusCode: 200,
    checkedAt: '2026-04-21T12:30:00Z',
    rawData: {
      rate_limit: {
        primary_window: {
          used_percent: 23,
          limit_window_seconds: 18000,
        },
        secondary_window: {
          used_percent: 4,
          limit_window_seconds: 604800,
        },
      },
    },
  },
  usage: {
    totalCompletions: 12,
    inputTokens: 200,
    outputTokens: 50,
    totalTokens: 250,
    todayInputTokens: 40,
    todayOutputTokens: 10,
    todayTotalTokens: 50,
    todayRequestCount: 2,
    sevenDayInputTokens: 120,
    sevenDayOutputTokens: 30,
    sevenDayTotalTokens: 150,
    sevenDayRequestCount: 6,
    thirtyDayInputTokens: 200,
    thirtyDayOutputTokens: 50,
    thirtyDayTotalTokens: 250,
    thirtyDayRequestCount: 12,
    lifetimeInputTokens: 200,
    lifetimeOutputTokens: 50,
    lifetimeTotalTokens: 250,
    lifetimeRequestCount: 12,
  },
};

describe('DashboardPage', () => {
  it('renders empty state when there are no accounts', () => {
    render(
      <DashboardPage
        summary={{ total: 0, active: 0, cooldown: 0, disabled: 0, rpm: 0, totalInputTokens: 0, totalOutputTokens: 0 }}
        accounts={[]}
        errorMessage=""
        onOpenImport={vi.fn()}
      />,
    );

    expect(screen.getByText(/导入你的第一个 Codex 账号/i)).toBeInTheDocument();
    expect(screen.getByRole('button', { name: /^设置$/i })).toBeInTheDocument();
    expect(screen.getAllByRole('button', { name: /导入账号/i }).length).toBeGreaterThan(0);
    expect(screen.getByText(/账号池概览/i)).toBeInTheDocument();
  });

  it('renders error state when stats request fails', () => {
    render(
      <DashboardPage
        summary={{ total: 0, active: 0, cooldown: 0, disabled: 0, rpm: 0, totalInputTokens: 0, totalOutputTokens: 0 }}
        accounts={[]}
        errorMessage="401 Unauthorized"
        onOpenImport={vi.fn()}
      />,
    );

    expect(screen.getByText(/401 Unauthorized/i)).toBeInTheDocument();
  });

  it('opens token overview drawer when token card is clicked', async () => {
    const user = userEvent.setup();

    render(
      <DashboardPage
        summary={{
          total: 1,
          active: 1,
          cooldown: 0,
          disabled: 0,
          rpm: 2,
          totalInputTokens: 200,
          totalOutputTokens: 50,
          tokenOverview: {
            today: { inputTokens: 40, outputTokens: 10, totalTokens: 50, requestCount: 2 },
            sevenDays: { inputTokens: 120, outputTokens: 30, totalTokens: 150, requestCount: 6 },
            thirtyDays: { inputTokens: 200, outputTokens: 50, totalTokens: 250, requestCount: 12 },
            lifetime: { inputTokens: 200, outputTokens: 50, totalTokens: 250, requestCount: 12 },
            updatedAt: '2026-04-23T00:00:00Z',
          },
        }}
        accounts={[sampleAccount]}
        errorMessage=""
      />
    );

    await user.click(screen.getByRole('button', { name: /Token 概览/i }));
    expect(screen.getByRole('dialog', { name: /Token 概览/i })).toBeInTheDocument();
  });
});

describe('AccountsTable', () => {
  it('renders semantic table rows with all dashboard columns', () => {
    render(
      <table>
        <tbody>
          <AccountsTable
            accounts={[sampleAccount]}
            selectedAccountId="a@example.com"
            onSelect={vi.fn()}
          />
        </tbody>
      </table>,
    );

    const row = screen.getByRole('button', { name: /a@example.com/i });
    expect(row.tagName).toBe('TR');
    expect(row).toHaveAttribute('data-selected', 'true');
    expect(screen.getAllByRole('cell')).toHaveLength(8);
  });

  it('renders compact quota progress bars for 5h and 7d windows', () => {
    render(
      <table>
        <tbody>
          <AccountsTable
            accounts={[sampleAccount]}
            selectedAccountId=""
            onSelect={vi.fn()}
          />
        </tbody>
      </table>,
    );

    expect(screen.getByText('5h')).toBeInTheDocument();
    expect(screen.getByText('7d')).toBeInTheDocument();
    expect(screen.getByText('77%')).toBeInTheDocument();
    expect(screen.getByText('96%')).toBeInTheDocument();
    expect(screen.getByRole('progressbar', { name: /5h 可用额度/i })).toHaveAttribute('aria-valuenow', '77');
    expect(screen.getByRole('progressbar', { name: /7d 可用额度/i })).toHaveAttribute('aria-valuenow', '96');
  });

  it('notifies when a row is clicked', async () => {
    const user = userEvent.setup();
    const handleSelect = vi.fn();

    render(
      <table>
        <tbody>
          <AccountsTable
            accounts={[sampleAccount]}
            selectedAccountId=""
            onSelect={handleSelect}
          />
        </tbody>
      </table>,
    );

    await user.click(screen.getByRole('button', { name: /a@example.com/i }));

    expect(handleSelect).toHaveBeenCalledWith('a@example.com');
  });
});
