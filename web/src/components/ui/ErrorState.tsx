import { Button } from './Button';
import { Card } from './Card';

interface ErrorStateProps {
  message: string;
  onRetry?: () => void;
}

export function ErrorState({ message, onRetry }: ErrorStateProps) {
  return (
    <Card className="flex min-h-[280px] flex-col items-start justify-center gap-5 rounded-[32px] border-[rgba(207,94,72,0.22)] bg-[rgba(255,247,244,0.88)] px-8">
      <span className="rounded-full bg-[rgba(207,94,72,0.14)] px-3 py-1 text-xs font-semibold uppercase tracking-[0.16em] text-[#8f2e1f]">
        同步异常
      </span>
      <div className="space-y-3">
        <h2 className="text-3xl font-semibold tracking-[-0.03em]">控制台暂时没拿到账号统计</h2>
        <p className="max-w-xl text-base leading-7 text-[color:var(--text-secondary)]">{message}</p>
      </div>
      {onRetry ? <Button onClick={onRetry}>重试请求</Button> : null}
    </Card>
  );
}
