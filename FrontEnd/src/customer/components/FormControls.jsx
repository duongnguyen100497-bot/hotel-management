import { useId, useState } from 'react';
import { AlertCircle, Eye, EyeOff } from 'lucide-react';

// Ô nhập có nhãn + thông báo lỗi gắn đúng ô (aria-describedby) để trình đọc màn hình đọc được.
export function TextField({ label, error, hint, type = 'text', as = 'input', className = '', children, ...rest }) {
  const uid = useId();
  const id = rest.id || uid;
  const [shown, setShown] = useState(false);
  const isPassword = type === 'password';
  const describedBy = error ? `${id}-err` : hint ? `${id}-hint` : undefined;
  const Tag = as;
  const base = `c-input ${error ? 'c-input-error' : ''} ${isPassword ? 'pr-12' : ''}`;

  return (
    <div className={className}>
      <label htmlFor={id} className="mb-1.5 block text-sm font-semibold text-slate-700">
        {label}
        {rest.required && <span className="text-danger" aria-hidden="true"> *</span>}
      </label>
      <div className="relative">
        <Tag id={id} type={as === 'input' ? (isPassword && shown ? 'text' : type) : undefined} aria-invalid={Boolean(error)} aria-describedby={describedBy} className={base} {...rest}>
          {children}
        </Tag>
        {isPassword && (
          <button
            type="button"
            onClick={() => setShown((s) => !s)}
            className="absolute right-2 top-1/2 flex h-9 w-9 -translate-y-1/2 items-center justify-center rounded-lg text-muted hover:bg-surface hover:text-accent"
            aria-label={shown ? 'Ẩn mật khẩu' : 'Hiện mật khẩu'}
            aria-pressed={shown}
          >
            {shown ? <EyeOff size={18} /> : <Eye size={18} />}
          </button>
        )}
      </div>
      {error ? (
        <p id={`${id}-err`} role="alert" className="mt-1.5 flex items-start gap-1.5 text-sm text-danger">
          <AlertCircle size={14} className="mt-0.5 shrink-0" /> {error}
        </p>
      ) : hint ? (
        <p id={`${id}-hint`} className="mt-1.5 text-xs text-muted">{hint}</p>
      ) : null}
    </div>
  );
}

export function FormAlert({ children, tone = 'danger' }) {
  if (!children) return null;
  const styles = tone === 'danger' ? 'border-red-200 bg-red-50 text-danger' : 'border-green-200 bg-green-50 text-success';
  return (
    <div role={tone === 'danger' ? 'alert' : 'status'} className={`flex items-start gap-2 rounded-xl border px-4 py-3 text-sm font-medium ${styles}`}>
      <AlertCircle size={16} className="mt-0.5 shrink-0" /> <span>{children}</span>
    </div>
  );
}

export function PasswordMeter({ score }) {
  const labels = ['', 'Yếu', 'Trung bình', 'Khá', 'Mạnh'];
  const colors = ['bg-slate-200', 'bg-danger', 'bg-warning', 'bg-accent', 'bg-success'];
  return (
    <div className="mt-2" aria-live="polite">
      <div className="flex gap-1.5" aria-hidden="true">
        {[1, 2, 3, 4].map((n) => (
          <span key={n} className={`h-1.5 flex-1 rounded-full transition-colors ${score >= n ? colors[score] : 'bg-slate-200'}`} />
        ))}
      </div>
      {score > 0 && <p className="mt-1 text-xs text-muted">Độ mạnh mật khẩu: <strong className="text-ink">{labels[score]}</strong></p>}
    </div>
  );
}
