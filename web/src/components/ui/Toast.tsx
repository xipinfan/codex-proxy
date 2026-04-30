import { Button } from './Button';

interface ToastProps {
  tone: 'success' | 'error' | 'info';
  text: string;
  onClose: () => void;
}

export function Toast({ tone, text, onClose }: ToastProps) {
  const toneClass =
    tone === 'success'
      ? 'border-[rgba(59,184,197,0.28)] bg-[rgba(255,251,245,0.96)] text-[#14626b]'
      : tone === 'error'
        ? 'border-[rgba(207,94,72,0.24)] bg-[rgba(255,247,244,0.98)] text-[#8f2e1f]'
        : 'border-[rgba(122,91,62,0.18)] bg-[rgba(255,251,245,0.96)] text-[color:var(--text-primary)]';

  return (
    <div className="pointer-events-none fixed inset-x-0 top-5 z-[70] flex justify-center px-4" role="status" aria-live="polite">
      <div
        role="alert"
        className={`pointer-events-auto flex w-full max-w-[560px] items-center justify-between gap-3 rounded-[20px] border px-4 py-3 text-sm shadow-[0_18px_50px_rgba(74,51,29,0.18)] backdrop-blur-xl ${toneClass}`}
      >
        <span>{text}</span>
        <Button variant="ghost" onClick={onClose}>
          关闭提示
        </Button>
      </div>
    </div>
  );
}
