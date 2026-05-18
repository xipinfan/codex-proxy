import { render, screen, waitFor } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { describe, expect, it, vi } from 'vitest';

import { AccountDetailDrawer } from '../features/account-detail/AccountDetailDrawer';
import { sampleAccount } from './fixtures';

describe('AccountDetailDrawer', () => {
  it('renders quota reset and usage values', () => {
    render(<AccountDetailDrawer account={sampleAccount} open onClose={() => {}} />);

    expect(screen.getByText(/额度窗口/i)).toBeInTheDocument();
    expect(screen.getByText(/5 小时额度/i)).toBeInTheDocument();
    expect(screen.getByText(/7 日额度/i)).toBeInTheDocument();
    expect(screen.getAllByText(/78%/i).length).toBeGreaterThanOrEqual(1);
    expect(screen.getByText(/97%/i)).toBeInTheDocument();
    expect(screen.getByRole('progressbar', { name: /5 小时额度 可用额度/i })).toHaveAttribute('aria-valuenow', '78');
    expect(screen.getByRole('progressbar', { name: /7 日额度 可用额度/i })).toHaveAttribute('aria-valuenow', '97');
    expect(screen.getByText(/累计 Token/i)).toBeInTheDocument();
    expect(screen.getAllByText(/5,000/i).length).toBeGreaterThanOrEqual(2);
  });

  it('renders fallback when quota data is missing', () => {
    render(<AccountDetailDrawer account={{ ...sampleAccount, quota: null }} open onClose={() => {}} />);

    expect(screen.getByText(/暂无额度数据/i)).toBeInTheDocument();
  });

  it('renders subscription window in account details', () => {
    render(
      <AccountDetailDrawer
        account={{
          ...sampleAccount,
          subscriptionActiveStart: '2026-05-01T00:00:00Z',
          subscriptionActiveUntil: '2026-06-01T00:00:00Z',
        }}
        open
        onClose={() => {}}
      />,
    );

    expect(screen.getByText('会员开始时间')).toBeInTheDocument();
    expect(screen.getByText('会员到期时间')).toBeInTheDocument();
    expect(screen.getByText(/05\/01/)).toBeInTheDocument();
    expect(screen.getByText(/06\/01/)).toBeInTheDocument();
  });

  it('supports deleting an account from danger actions', async () => {
    const user = userEvent.setup();
    const onDeleteAccount = vi.fn().mockResolvedValue(undefined);

    render(<AccountDetailDrawer account={sampleAccount} open onClose={() => {}} onDeleteAccount={onDeleteAccount} />);

    await user.click(screen.getByRole('button', { name: /删除此账号/i }));
    await user.click(screen.getByRole('button', { name: /确认删除此账号/i }));

    await waitFor(() => {
      expect(onDeleteAccount).toHaveBeenCalledWith(sampleAccount);
    });
  });
});
