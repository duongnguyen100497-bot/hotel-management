import { Check } from 'lucide-react';

export const BOOKING_STEPS = ['Chọn phòng', 'Thông tin khách', 'Kiểm tra', 'Thanh toán', 'Xác nhận'];

// Thanh các bước đặt phòng. `current` 1-5; chỉ cho quay lại các bước đã qua.
export default function BookingStepper({ current, onGo }) {
  return (
    <nav aria-label="Các bước đặt phòng">
      <ol className="flex items-center">
        {BOOKING_STEPS.map((label, i) => {
          const n = i + 1;
          const done = n < current;
          const active = n === current;
          return (
            <li key={label} className={`flex items-center ${n < BOOKING_STEPS.length ? 'flex-1' : ''}`} aria-current={active ? 'step' : undefined}>
              <button
                type="button"
                disabled={!done || !onGo}
                onClick={() => onGo?.(n)}
                className="group flex items-center gap-2.5 disabled:cursor-default"
                aria-label={`Bước ${n}: ${label}${done ? ' (đã xong)' : active ? ' (đang thực hiện)' : ''}`}
              >
                <span className={`flex h-9 w-9 shrink-0 items-center justify-center rounded-full text-sm font-bold transition-colors ${done ? 'bg-success text-white' : active ? 'bg-accent text-white ring-4 ring-accent/20' : 'bg-slate-100 text-muted'}`}>
                  {done ? <Check size={17} /> : n}
                </span>
                <span className={`hidden text-sm font-semibold md:block ${active ? 'text-brand' : done ? 'text-slate-700 group-enabled:group-hover:text-accent' : 'text-muted'}`}>{label}</span>
              </button>
              {n < BOOKING_STEPS.length && <span className={`mx-3 h-0.5 flex-1 rounded ${done ? 'bg-success' : 'bg-slate-200'}`} aria-hidden="true" />}
            </li>
          );
        })}
      </ol>
      <p className="mt-3 text-sm font-semibold text-brand md:hidden">Bước {current}/{BOOKING_STEPS.length}: {BOOKING_STEPS[current - 1]}</p>
    </nav>
  );
}
