import { useEffect, useState } from 'react';

import { Button } from '../../components/ui/Button';
import { Modal } from '../../components/ui/Modal';
import { buildManualTokenFilePayload } from '../../lib/oauth';
import type { IngestResult, OAuthPollResponse, OAuthStartResponse, TokenFilePayload } from '../../lib/types';

interface OAuthImportDialogProps {
  open: boolean;
  onClose: () => void;
  onOAuthStart: () => Promise<OAuthStartResponse>;
  onOAuthPoll: (state: string) => Promise<OAuthPollResponse>;
  onManualImport: (payload: TokenFilePayload) => Promise<IngestResult | void>;
}

type ImportMode = 'oauth' | 'manual';

const initialManualForm: TokenFilePayload = {
  type: 'codex',
  refresh_token: '',
  access_token: '',
  id_token: '',
  account_id: '',
  email: '',
  expired: '',
};

function buildSuccessMessage(result: IngestResult | void): string {
  if (!result) {
    return '导入请求已提交。';
  }

  const total = result.added + result.updated;
  const main = result.failed > 0 ? `已导入 ${total} 个账号，失败 ${result.failed} 个。` : `已成功导入 ${total} 个账号。`;
  const validationText = result.validation?.message ? `校验结果：${result.validation.message}` : '';
  return validationText ? `${main} ${validationText}` : main;
}

export function OAuthImportDialog({ open, onClose, onOAuthStart, onOAuthPoll, onManualImport }: OAuthImportDialogProps) {
  const [mode, setMode] = useState<ImportMode>('oauth');
  const [manualForm, setManualForm] = useState<TokenFilePayload>(initialManualForm);
  const [errorMessage, setErrorMessage] = useState('');
  const [successMessage, setSuccessMessage] = useState('');
  const [isSubmitting, setIsSubmitting] = useState(false);
  const [isOpeningAuth, setIsOpeningAuth] = useState(false);
  const [oauthState, setOauthState] = useState<string | null>(null);

  useEffect(() => {
    if (open) {
      setMode('oauth');
      setManualForm(initialManualForm);
      setErrorMessage('');
      setSuccessMessage('');
      setOauthState(null);
    }
  }, [open]);

  function updateManualField<K extends keyof TokenFilePayload>(key: K, value: TokenFilePayload[K]) {
    setManualForm((current) => ({ ...current, [key]: value }));
  }

  async function handleOpenAuthPage() {
    setErrorMessage('');
    setSuccessMessage('');
    setIsOpeningAuth(true);
    try {
      const started = await onOAuthStart();
      setOauthState(started.state);
      window.open(started.authorize_url, '_blank', 'noopener,noreferrer');
      setSuccessMessage('授权页已打开。完成登录后会自动回调并导入，当前正在等待授权结果。');
      setIsOpeningAuth(false);
      await waitForOAuthResult(started.state, started.expires_in);
    } catch (error) {
      setErrorMessage(error instanceof Error ? error.message : '拉起授权页失败。');
    } finally {
      setIsOpeningAuth(false);
    }
  }

  async function waitForOAuthResult(state: string, expiresIn: number) {
    const deadline = Date.now() + Math.max(30, expiresIn) * 1000;
    setIsSubmitting(true);
    while (Date.now() < deadline) {
      const result = await onOAuthPoll(state);
      if (result.status === 'completed') {
        setSuccessMessage(buildSuccessMessage(result.result));
        setErrorMessage('');
        setOauthState(null);
        setIsSubmitting(false);
        return;
      }
      if (result.status === 'failed') {
        throw new Error(result.message || '授权失败。');
      }
      await new Promise((resolve) => window.setTimeout(resolve, 600));
    }
    throw new Error('等待授权回调超时，请重新打开授权页。');
  }

  async function handleManualSubmit(event: React.FormEvent<HTMLFormElement>) {
    event.preventDefault();

    const payload = buildManualTokenFilePayload(manualForm);
    if (!payload.access_token && !payload.refresh_token && !payload.id_token) {
      setErrorMessage('至少填写 access_token、refresh_token 或 id_token 中的一项');
      setSuccessMessage('');
      return;
    }

    setErrorMessage('');
    setSuccessMessage('');
    setIsSubmitting(true);

    try {
      const result = await onManualImport(payload);
      setSuccessMessage(buildSuccessMessage(result));
    } catch (error) {
      setErrorMessage(error instanceof Error ? error.message : '导入失败。');
    } finally {
      setIsSubmitting(false);
    }
  }

  return (
    <Modal open={open} onClose={onClose} title="导入 Codex 账号">
      <div className="mb-6 flex items-start justify-between gap-4">
        <div>
          <p className="text-xs font-semibold tracking-[0.22em] text-[color:var(--text-secondary)]">账号导入</p>
          <h2 className="mt-2 text-2xl font-semibold tracking-[-0.04em]">把 Codex 账号加入账号池</h2>
          <p className="mt-2 max-w-xl text-sm leading-6 text-[color:var(--text-secondary)]">
            支持两种方式：打开授权页完成回调导入，或直接填写账号字段导入。含 <code>refresh_token</code> 的账号会在导入后自动校验有效性。
          </p>
        </div>
        <Button variant="ghost" onClick={onClose}>
          关闭
        </Button>
      </div>

      <div className="mb-4 flex flex-wrap gap-2 rounded-[24px] bg-white/60 p-1">
        <Button variant={mode === 'oauth' ? 'primary' : 'secondary'} onClick={() => setMode('oauth')}>
          授权回调导入
        </Button>
        <Button variant={mode === 'manual' ? 'primary' : 'secondary'} onClick={() => setMode('manual')}>
          字段直填导入
        </Button>
      </div>

      {mode === 'oauth' ? (
        <div className="grid gap-4">
          <div className="rounded-[24px] border border-[color:var(--border-soft)] bg-[rgba(255,250,245,0.84)] p-4 shadow-[inset_0_1px_0_rgba(255,255,255,0.72)]">
            <p className="text-xs font-semibold tracking-[0.18em] text-[color:var(--text-secondary)]">授权导入流程</p>
            <ol className="mt-3 grid gap-3 text-sm leading-6 text-[color:var(--text-secondary)]">
              <li>1. 打开 OpenAI 授权页并完成登录。</li>
              <li>2. 浏览器会跳回本机 <code>localhost:1455</code> 回调监听。</li>
              <li>3. 控制台自动拿到授权 code，换取令牌并导入到当前账号池。</li>
            </ol>
          </div>

          <div className="flex flex-wrap items-center gap-3">
            <Button variant="secondary" disabled={isOpeningAuth || isSubmitting} onClick={() => void handleOpenAuthPage()}>
              {isOpeningAuth ? '拉起中...' : isSubmitting ? '等待回调中...' : '打开 OpenAI 授权页'}
            </Button>
            <span className="text-sm text-[color:var(--text-secondary)]">授权地址由后端动态生成，并自动附带 PKCE 与 state 校验。</span>
          </div>

          {oauthState ? (
            <p className="rounded-[20px] bg-white/70 px-4 py-3 text-sm text-[color:var(--text-secondary)]">
              正在等待 state <code>{oauthState.slice(0, 8)}</code> 的回调结果，请在新打开的授权页完成登录。
            </p>
          ) : null}

          {errorMessage ? <p className="rounded-[20px] bg-[rgba(207,94,72,0.12)] px-4 py-3 text-sm text-[#8f2e1f]">{errorMessage}</p> : null}
          {successMessage ? <p className="rounded-[20px] bg-[rgba(59,184,197,0.14)] px-4 py-3 text-sm text-[#14626b]">{successMessage}</p> : null}

          <div className="mt-2 flex items-center justify-end gap-3">
            <Button variant="secondary" onClick={onClose}>
              稍后再说
            </Button>
          </div>
        </div>
      ) : (
        <form className="grid gap-4" onSubmit={handleManualSubmit}>
          <div className="rounded-[24px] border border-[color:var(--border-soft)] bg-[rgba(255,250,245,0.84)] p-4 shadow-[inset_0_1px_0_rgba(255,255,255,0.72)]">
            <p className="text-xs font-semibold tracking-[0.18em] text-[color:var(--text-secondary)]">字段直填导入</p>
            <p className="mt-3 text-sm leading-6 text-[color:var(--text-secondary)]">
              直接填写账号字段并导入。至少填写 <code>access_token</code>、<code>refresh_token</code> 或 <code>id_token</code> 中的一项；如果填写了 <code>refresh_token</code>，系统会在导入后自动尝试刷新校验。
            </p>
          </div>

          <div className="grid gap-4 md:grid-cols-2">
            <label className="grid gap-2 text-sm font-medium text-[color:var(--text-secondary)]">
              <span>type（账号类型）</span>
              <input className="console-field" value={manualForm.type} onChange={(event) => updateManualField('type', event.target.value)} />
            </label>
            <label className="grid gap-2 text-sm font-medium text-[color:var(--text-secondary)]">
              <span>email（邮箱）</span>
              <input className="console-field" value={manualForm.email} onChange={(event) => updateManualField('email', event.target.value)} />
            </label>
          </div>

          <label className="grid gap-2 text-sm font-medium text-[color:var(--text-secondary)]">
            <span>access_token（访问令牌）</span>
            <textarea className="min-h-[92px] rounded-[24px] border border-[color:var(--border-soft)] bg-white/80 px-4 py-3 outline-none" value={manualForm.access_token} onChange={(event) => updateManualField('access_token', event.target.value)} />
          </label>

          <label className="grid gap-2 text-sm font-medium text-[color:var(--text-secondary)]">
            <span>refresh_token（刷新令牌）</span>
            <textarea className="min-h-[92px] rounded-[24px] border border-[color:var(--border-soft)] bg-white/80 px-4 py-3 outline-none" value={manualForm.refresh_token} onChange={(event) => updateManualField('refresh_token', event.target.value)} />
          </label>

          <label className="grid gap-2 text-sm font-medium text-[color:var(--text-secondary)]">
            <span>id_token（身份令牌）</span>
            <textarea className="min-h-[92px] rounded-[24px] border border-[color:var(--border-soft)] bg-white/80 px-4 py-3 outline-none" value={manualForm.id_token} onChange={(event) => updateManualField('id_token', event.target.value)} />
          </label>

          <div className="grid gap-4 md:grid-cols-2">
            <label className="grid gap-2 text-sm font-medium text-[color:var(--text-secondary)]">
              <span>account_id（账号 ID）</span>
              <input className="console-field" value={manualForm.account_id} onChange={(event) => updateManualField('account_id', event.target.value)} />
            </label>
            <label className="grid gap-2 text-sm font-medium text-[color:var(--text-secondary)]">
              <span>expired（过期时间）</span>
              <input className="console-field" value={manualForm.expired} onChange={(event) => updateManualField('expired', event.target.value)} />
            </label>
          </div>

          {errorMessage ? <p className="rounded-[20px] bg-[rgba(207,94,72,0.12)] px-4 py-3 text-sm text-[#8f2e1f]">{errorMessage}</p> : null}
          {successMessage ? <p className="rounded-[20px] bg-[rgba(59,184,197,0.14)] px-4 py-3 text-sm text-[#14626b]">{successMessage}</p> : null}

          <div className="mt-2 flex items-center justify-end gap-3">
            <Button variant="secondary" onClick={onClose}>
              稍后再说
            </Button>
            <Button type="submit" disabled={isSubmitting}>
              {isSubmitting ? '导入中...' : '直接导入账号'}
            </Button>
          </div>
        </form>
      )}
    </Modal>
  );
}
