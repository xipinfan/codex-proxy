import { render, screen } from '@testing-library/react';
import { describe, expect, it } from 'vitest';

import { AccountDetailDrawer } from '../features/account-detail/AccountDetailDrawer';
import { sampleAccount } from './fixtures';

describe('AccountDetailDrawer', () => {
  it('renders quota reset and usage values', () => {
    render(<AccountDetailDrawer account={sampleAccount} open onClose={() => {}} />);

    expect(screen.getByText(/额度窗口/i)).toBeInTheDocument();
    expect(screen.getByText(/重置时间/i)).toBeInTheDocument();
    expect(screen.getByText(/5,000/i)).toBeInTheDocument();
  });

  it('renders fallback when quota data is missing', () => {
    render(<AccountDetailDrawer account={{ ...sampleAccount, quota: null }} open onClose={() => {}} />);

    expect(screen.getByText(/暂无额度数据/i)).toBeInTheDocument();
  });
});
