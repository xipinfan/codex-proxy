import { render, screen, waitFor } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { describe, expect, it, vi } from 'vitest';

import { OAuthImportDialog } from '../features/oauth-import/OAuthImportDialog';

describe('OAuthImportDialog', () => {
  it('passes the callback url to the import handler', async () => {
    const user = userEvent.setup();
    const onOAuthImport = vi.fn().mockResolvedValue(undefined);
    const onManualImport = vi.fn().mockResolvedValue(undefined);

    render(<OAuthImportDialog open onClose={() => {}} onOAuthImport={onOAuthImport} onManualImport={onManualImport} />);

    await user.type(
      screen.getByLabelText(/回调 URL/i),
      'http://127.0.0.1:1455/callback#access_token=at&id_token=it&refresh_token=rt',
    );
    await user.click(screen.getByRole('button', { name: /解析并导入/i }));

    await waitFor(() => {
      expect(onOAuthImport).toHaveBeenCalledWith(
        'http://127.0.0.1:1455/callback#access_token=at&id_token=it&refresh_token=rt',
      );
    });
  });

  it('submits a manual token payload from direct fields', async () => {
    const user = userEvent.setup();
    const onOAuthImport = vi.fn().mockResolvedValue(undefined);
    const onManualImport = vi.fn().mockResolvedValue(undefined);

    render(<OAuthImportDialog open onClose={() => {}} onOAuthImport={onOAuthImport} onManualImport={onManualImport} />);

    await user.click(screen.getByRole('button', { name: /字段直填导入/i }));
    await user.clear(screen.getByLabelText(/type（账号类型）/i));
    await user.type(screen.getByLabelText(/type（账号类型）/i), 'codex');
    await user.type(screen.getByLabelText(/email（邮箱）/i), 'alice@example.com');
    await user.type(screen.getByLabelText(/access_token（访问令牌）/i), 'access-token');
    await user.type(screen.getByLabelText(/refresh_token（刷新令牌）/i), 'refresh-token');
    await user.type(screen.getByLabelText(/account_id（账号 ID）/i), 'acct_123');
    await user.click(screen.getByRole('button', { name: /直接导入账号/i }));

    await waitFor(() => {
      expect(onManualImport).toHaveBeenCalledWith({
        type: 'codex',
        email: 'alice@example.com',
        access_token: 'access-token',
        refresh_token: 'refresh-token',
        rk: 'refresh-token',
        account_id: 'acct_123',
      });
    });
  });

  it('shows a validation hint when the callback url is empty', async () => {
    const user = userEvent.setup();

    render(<OAuthImportDialog open onClose={() => {}} onOAuthImport={vi.fn()} onManualImport={vi.fn()} />);

    await user.click(screen.getByRole('button', { name: /解析并导入/i }));

    expect(screen.getByText(/请粘贴完整回调 URL/i)).toBeInTheDocument();
  });

  it('shows a validation hint when manual token fields are empty', async () => {
    const user = userEvent.setup();

    render(<OAuthImportDialog open onClose={() => {}} onOAuthImport={vi.fn()} onManualImport={vi.fn()} />);

    await user.click(screen.getByRole('button', { name: /字段直填导入/i }));
    await user.click(screen.getByRole('button', { name: /直接导入账号/i }));

    expect(screen.getAllByText(/至少填写 access_token、refresh_token 或 id_token 中的一项/i)).toHaveLength(2);
  });
});
