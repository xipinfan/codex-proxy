import { render, screen, waitFor } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { describe, expect, it, vi } from 'vitest';

import { OAuthImportDialog } from '../features/oauth-import/OAuthImportDialog';

describe('OAuthImportDialog', () => {
  it('polls the backend after opening the oauth page and shows the final result', async () => {
    const user = userEvent.setup();
    const openSpy = vi.spyOn(window, 'open').mockImplementation(() => null);
    const onOAuthStart = vi.fn().mockResolvedValue({
      authorize_url: 'https://auth.openai.com/oauth/authorize?state=abc',
      state: 'oauth-state',
      expires_in: 300,
    });
    const onOAuthPoll = vi
      .fn()
      .mockResolvedValueOnce({ status: 'pending' })
      .mockResolvedValueOnce({
        status: 'completed',
        result: {
          added: 1,
          updated: 0,
          failed: 0,
          pool_total: 1,
        },
      });
    const onManualImport = vi.fn().mockResolvedValue(undefined);
    const onOAuthComplete = vi.fn().mockResolvedValue({
      added: 0,
      updated: 0,
      failed: 0,
      pool_total: 0,
    });

    render(
      <OAuthImportDialog
        open
        onClose={() => {}}
        onOAuthStart={onOAuthStart}
        onOAuthPoll={onOAuthPoll}
        onOAuthComplete={onOAuthComplete}
        onManualImport={onManualImport}
      />,
    );
    await user.click(screen.getByRole('button', { name: /打开 OpenAI 授权页/i }));

    await waitFor(() => {
      expect(onOAuthPoll).toHaveBeenCalledWith('oauth-state');
    });
    expect(openSpy).toHaveBeenCalledWith('https://auth.openai.com/oauth/authorize?state=abc', '_blank', 'noopener,noreferrer');
    expect(await screen.findByText(/已成功导入 1 个账号/i)).toBeInTheDocument();
    openSpy.mockRestore();
  });

  it('submits a manual token payload from direct fields', async () => {
    const user = userEvent.setup();
    const onOAuthStart = vi.fn().mockResolvedValue({
      authorize_url: 'https://auth.openai.com/oauth/authorize?state=abc',
      state: 'oauth-state',
      expires_in: 300,
    });
    const onManualImport = vi.fn().mockResolvedValue(undefined);
    const onOAuthComplete = vi.fn().mockResolvedValue({
      added: 0,
      updated: 0,
      failed: 0,
      pool_total: 0,
    });

    render(
      <OAuthImportDialog
        open
        onClose={() => {}}
        onOAuthStart={onOAuthStart}
        onOAuthPoll={vi.fn()}
        onOAuthComplete={onOAuthComplete}
        onManualImport={onManualImport}
      />,
    );

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

  it('shows a validation hint when manual token fields are empty', async () => {
    const user = userEvent.setup();

    render(
      <OAuthImportDialog
        open
        onClose={() => {}}
        onOAuthStart={vi.fn()}
        onOAuthPoll={vi.fn()}
        onOAuthComplete={vi.fn()}
        onManualImport={vi.fn()}
      />,
    );

    await user.click(screen.getByRole('button', { name: /字段直填导入/i }));
    await user.click(screen.getByRole('button', { name: /直接导入账号/i }));

    expect(screen.getByText(/至少填写 access_token、refresh_token 或 id_token 中的一项/i)).toBeInTheDocument();
  });

  it('requests backend pkce url before opening oauth page', async () => {
    const user = userEvent.setup();
    const openSpy = vi.spyOn(window, 'open').mockImplementation(() => null);
    const onOAuthStart = vi.fn().mockResolvedValue({
      authorize_url: 'https://auth.openai.com/oauth/authorize?state=abc',
      state: 'oauth-state',
      expires_in: 300,
    });

    render(
      <OAuthImportDialog
        open
        onClose={() => {}}
        onOAuthStart={onOAuthStart}
        onOAuthPoll={vi.fn().mockResolvedValue({ status: 'pending' })}
        onOAuthComplete={vi.fn()}
        onManualImport={vi.fn()}
      />,
    );
    await user.click(screen.getByRole('button', { name: /打开 OpenAI 授权页/i }));

    await waitFor(() => {
      expect(onOAuthStart).toHaveBeenCalledTimes(1);
      expect(openSpy).toHaveBeenCalledWith('https://auth.openai.com/oauth/authorize?state=abc', '_blank', 'noopener,noreferrer');
    });
    openSpy.mockRestore();
  });

  it('submits callback url to complete oauth manually', async () => {
    const user = userEvent.setup();
    const onOAuthComplete = vi.fn().mockResolvedValue({
      added: 1,
      updated: 0,
      failed: 0,
      pool_total: 1,
    });

    render(
      <OAuthImportDialog
        open
        onClose={() => {}}
        onOAuthStart={vi.fn()}
        onOAuthPoll={vi.fn()}
        onOAuthComplete={onOAuthComplete}
        onManualImport={vi.fn()}
      />,
    );

    await user.type(
      screen.getByLabelText(/回调 URL/i),
      'http://localhost:1455/auth/callback?code=demo&state=oauth-state',
    );
    await user.click(screen.getByRole('button', { name: /提交回调 URL/i }));

    await waitFor(() => {
      expect(onOAuthComplete).toHaveBeenCalledWith('http://localhost:1455/auth/callback?code=demo&state=oauth-state');
    });
    expect(await screen.findByText(/已成功导入 1 个账号/i)).toBeInTheDocument();
  });
});
