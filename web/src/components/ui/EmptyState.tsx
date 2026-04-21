import { Button } from './Button';
import { Card } from './Card';

interface EmptyStateProps {
  onImport?: () => void;
}

export function EmptyState({ onImport }: EmptyStateProps) {
  return (
    <Card className="flex min-h-[320px] flex-col items-start justify-center gap-5 rounded-[32px] border-dashed bg-[rgba(255,250,245,0.78)] px-8">
      <span className="rounded-full bg-[rgba(59,184,197,0.14)] px-3 py-1 text-xs font-semibold uppercase tracking-[0.16em] text-[#14626b]">
        空账号池
      </span>
      <div className="max-w-xl space-y-3">
        <h2 className="text-3xl font-semibold tracking-[-0.03em]">导入你的第一个 Codex 账号</h2>
        <p className="text-base leading-7 text-[color:var(--text-secondary)]">
          当前账号池还是空的。你可以通过 OAuth 回调 URL 或直接填写 token 字段导入账号，随后就能在这里查看健康度、额度与错误状态。
        </p>
      </div>
      {onImport ? <Button onClick={onImport}>打开导入流程</Button> : null}
    </Card>
  );
}
