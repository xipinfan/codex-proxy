import { render, screen } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { describe, expect, it, vi } from 'vitest';

import { SettingsDialog } from '../features/settings/SettingsDialog';
import { defaultSettings } from '../lib/types';

describe('SettingsDialog', () => {
  it('saves settings and closes dialog', async () => {
    const user = userEvent.setup();
    const onSave = vi.fn();
    const onClose = vi.fn();

    render(
      <SettingsDialog
        open
        initialValue={defaultSettings}
        onSave={onSave}
        onClose={onClose}
      />,
    );

    expect(screen.getByText(/控制台设置/i)).toBeInTheDocument();
    await user.type(screen.getByLabelText(/接口基础地址/i), 'http://127.0.0.1:8080');
    await user.click(screen.getByRole('button', { name: /保存设置/i }));

    expect(onSave).toHaveBeenCalledWith(
      expect.objectContaining({
        baseUrl: 'http://127.0.0.1:8080',
      }),
    );
    expect(onClose).toHaveBeenCalled();
  });
});
