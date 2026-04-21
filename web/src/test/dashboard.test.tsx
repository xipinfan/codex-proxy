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
  usage: {
    totalCompletions: 12,
    inputTokens: 200,
    outputTokens: 50,
    totalTokens: 250,
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
