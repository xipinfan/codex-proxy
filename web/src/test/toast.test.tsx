import { render, screen } from '@testing-library/react';
import { describe, expect, it, vi } from 'vitest';

import { Toast } from '../components/ui/Toast';

describe('Toast', () => {
  it('renders as a floating status message without taking page layout space', () => {
    render(<Toast tone="success" text="额度检查完成，成功 0 个，失败 0 个。" onClose={vi.fn()} />);

    const toastRegion = screen.getByRole('status');
    expect(toastRegion).toHaveClass('fixed');
    expect(screen.getByText(/额度检查完成，成功 0 个，失败 0 个。/i)).toBeInTheDocument();
    expect(screen.getByRole('button', { name: /关闭提示/i })).toBeInTheDocument();
  });
});
