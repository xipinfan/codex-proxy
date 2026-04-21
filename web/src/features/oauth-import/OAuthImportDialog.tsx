import { useEffect, useState } from 'react';

import { Button } from '../../components/ui/Button';
import { Modal } from '../../components/ui/Modal';
import { buildManualTokenFilePayload, codexOAuthPortalUrl } from '../../lib/oauth';
import type { IngestResult, TokenFilePayload } from '../../lib/types';

interface OAuthImportDialogProps {
  open: boolean;
  onClose: () => void;
  onOAuthImport: (callbackUrl: string) => Promise<IngestResult | void>;
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
  return result.failed > 0 ? `已导入 ${total} 个账号，失败 ${result.failed} 个。` : `已成功导入 ${total} 个账号。`;
}

export function OAuthImportDialog({ open, onClose, onOAuthImport, onManualImport }: OAuthImportDialogProps) {
  const [mode, setMode] = useState<ImportMode>('oauth');
  const [callbackUrl, setCallbackUrl] = useState('');
  const [manualForm, setManualForm] = useState<TokenFilePayload>(initialManualForm);
  const [errorMessage, setErrorMessage] = useState('');
  const [successMessage, setSuccessMessage] = useState('');
  const [isSubmitting, setIsSubmitting] = useState(false);

  useEffect(() => {
    if (open) {
      setMode('oauth');
      setCallbackUrl('');
      setManualForm(initialManualForm);
      setErrorMessage('');
      setSuccessMessage('');
    }
  }, [open]);

  function updateManualField<K extends keyof TokenFilePayload>(key: K, value: TokenFilePayload[K]) {
    setManualForm((current) => ({ ...current, [key]: value }));
  }

  async function handleOAuthSubmit(event: React.FormEvent<HTMLFormElement>) {
    event.preventDefault();

    if (!callbackUrl.trim()) {
      setErrorMessage('请粘贴完整回调 URL');
      setSuccessMessage('');
      return;
    }

    setErrorMessage('');
    setSuccessMessage('');
    setIsSubmitting(true);

    try {
      const result = await onOAuthImport(callbackUrl.trim());
      setSuccessMessage(buildSuccessMessage(result));
    } catch (error) {
      setErrorMessage(error instanceof Error ? error.message : '导入失败。');
    } finally {
      setIsSubmitting(false);
    }
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
          <p className="text-xs uppercase tracking-[0.22em] text-[color:var(--text-secondary)]">账号导入</p>
          <h2 className="mt-2 text-2xl font-semibold tracking-[-0.04em]">把 Codex 账号加入账号池</h2>
        </div>
        <Button variant="ghost" onClick={onClose}>
          关闭
        </Button>
      </div>

      <div className="mb-4 flex flex-wrap gap-2 rounded-[24px] bg-white/60 p-1">
        <Button variant={mode === 'oauth' ? 'primary' : 'secondary'} onClick={() => setMode('oauth')}>
          OAuth 回调导入
        </Button>
        <Button variant={mode === 'manual' ? 'primary' : 'secondary'} onClick={() => setMode('manual')}>
          字段直填导入
        </Button>
      </div>

      {mode === 'oauth' ? (
        <form className="grid gap-4" onSubmit={handleOAuthSubmit}>
          <ol className="grid gap-3 rounded-[24px] border border-[color:var(--border-soft)] bg-white/70 p-4 text-sm leading-6 text-[color:var(--text-secondary)]">
            <li>1. 打开 OpenAI 授权页并完成登录。</li>
            <li>2. 浏览器跳回 localhost 后，复制完整回调 URL。</li>
            <li>3. 粘贴到下方，解析并导入到当前账号池。</li>
          </ol>

          <div className="flex flex-wrap items-center gap-3">
            <Button variant="secondary" onClick={() => window.open(codexOAuthPortalUrl, '_blank', 'noopener,noreferrer')}>
              打开 OpenAI 授权页
            </Button>
            <span className="text-sm text-[color:var(--text-secondary)]">使用回调 URL 中的 token 字段完成导入。</span>
          </div>

          <label className="grid gap-2 text-sm font-medium text-[color:var(--text-secondary)]">
            <span>回调 URL</span>
            <textarea
              className="min-h-[140px] rounded-[24px] border border-[color:var(--border-soft)] bg-white/80 px-4 py-3 outline-none transition focus:border-[rgba(243,146,57,0.4)] focus:shadow-[0_0_0_4px_rgba(243,146,57,0.12)]"
              value={callbackUrl}
              onChange={(event) => setCallbackUrl(event.target.value)}
              placeholder="http://127.0.0.1:1455/callback#access_token=..."
            />
          </label>

          {errorMessage ? <p className="rounded-[20px] bg-[rgba(207,94,72,0.12)] px-4 py-3 text-sm text-[#8f2e1f]">{errorMessage}</p> : null}
          {successMessage ? <p className="rounded-[20px] bg-[rgba(59,184,197,0.14)] px-4 py-3 text-sm text-[#14626b]">{successMessage}</p> : null}

          <div className="mt-2 flex items-center justify-end gap-3">
            <Button variant="secondary" onClick={onClose}>
              稍后再说
            </Button>
            <Button type="submit" disabled={isSubmitting}>
              {isSubmitting ? '导入中...' : '解析并导入'}
            </Button>
          </div>
        </form>
      ) : (
        <form className="grid gap-4" onSubmit={handleManualSubmit}>
          <p className="rounded-[24px] border border-[color:var(--border-soft)] bg-white/70 p-4 text-sm leading-6 text-[color:var(--text-secondary)]">
            直接填写账号 token 字段并导入。至少填写 access_token、refresh_token 或 id_token 中的一项。
          </p>

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
