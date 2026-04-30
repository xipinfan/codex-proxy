import { useEffect, useState } from 'react';

import { Button } from '../../components/ui/Button';
import { Modal } from '../../components/ui/Modal';
import type { ConsoleSettings } from '../../lib/types';

interface SettingsDialogProps {
  open: boolean;
  initialValue: ConsoleSettings;
  onSave: (value: ConsoleSettings) => void;
  onClose: () => void;
}

export function SettingsDialog({ open, initialValue, onSave, onClose }: SettingsDialogProps) {
  const [formValue, setFormValue] = useState(initialValue);

  useEffect(() => {
    if (open) {
      setFormValue(initialValue);
    }
  }, [initialValue, open]);

  function update<K extends keyof ConsoleSettings>(key: K, value: ConsoleSettings[K]) {
    setFormValue((current) => ({ ...current, [key]: value }));
  }

  function handleSubmit(event: React.FormEvent<HTMLFormElement>) {
    event.preventDefault();
    onSave({
      ...formValue,
      pageSize: Math.max(1, Number(formValue.pageSize) || initialValue.pageSize),
      autoRefreshSeconds: Math.max(0, Number(formValue.autoRefreshSeconds) || 0),
    });
    onClose();
  }

  return (
    <Modal open={open} onClose={onClose} title="控制台设置">
      <div className="mb-6 flex items-start justify-between gap-4">
        <div>
          <p className="text-xs uppercase tracking-[0.22em] text-[color:var(--text-secondary)]">控制台设置</p>
          <h2 className="mt-2 text-2xl font-semibold tracking-[-0.04em]">控制台连接配置</h2>
        </div>
        <Button variant="ghost" onClick={onClose}>
          关闭
        </Button>
      </div>

      <form className="grid gap-4" onSubmit={handleSubmit}>
        <label className="grid gap-2 text-sm font-medium text-[color:var(--text-secondary)]">
          <span>接口基础地址</span>
          <input className="console-field" value={formValue.baseUrl} onChange={(event) => update('baseUrl', event.target.value)} />
        </label>

        <label className="grid gap-2 text-sm font-medium text-[color:var(--text-secondary)]">
          <span>管理密钥</span>
          <input
            className="console-field"
            value={formValue.apiKey}
            onChange={(event) => update('apiKey', event.target.value)}
            type="password"
            placeholder="sk-..."
          />
        </label>

        <div className="grid gap-4 md:grid-cols-2">
          <label className="grid gap-2 text-sm font-medium text-[color:var(--text-secondary)]">
          <span>默认分页数量</span>
            <input
              className="console-field"
              type="number"
              min={1}
              value={formValue.pageSize}
              onChange={(event) => update('pageSize', Number(event.target.value))}
            />
          </label>

          <label className="grid gap-2 text-sm font-medium text-[color:var(--text-secondary)]">
            <span>自动刷新秒数</span>
            <input
              className="console-field"
              type="number"
              min={0}
              step={15}
              value={formValue.autoRefreshSeconds}
              onChange={(event) => update('autoRefreshSeconds', Number(event.target.value))}
            />
          </label>
        </div>

        <label className="flex items-center gap-3 rounded-[24px] border border-[color:var(--border-soft)] bg-white/70 px-4 py-4 text-sm text-[color:var(--text-primary)]">
          <input checked={formValue.includeQuota} onChange={(event) => update('includeQuota', event.target.checked)} type="checkbox" />
          <span>默认包含额度快照</span>
        </label>

        <div className="mt-2 flex items-center justify-end gap-3">
          <Button variant="secondary" onClick={onClose}>
            取消
          </Button>
          <Button type="submit">保存设置</Button>
        </div>
      </form>
    </Modal>
  );
}
