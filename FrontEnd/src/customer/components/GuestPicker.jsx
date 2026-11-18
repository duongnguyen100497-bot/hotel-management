import { useEffect, useRef, useState } from 'react';
import { ChevronDown, Minus, Plus, Users } from 'lucide-react';
import useClickOutside from '../../shared/hooks/useClickOutside.js';
import { SEARCH_LIMITS } from '../utils/bookingSearch.js';

function CounterRow({ label, hint, value, min, max, onChange }) {
  return (
    <div className="flex items-center justify-between gap-4 py-3">
      <div>
        <p className="text-sm font-semibold text-ink">{label}</p>
        {hint && <p className="text-xs text-muted">{hint}</p>}
      </div>
      <div className="flex items-center gap-3">
        <button
          type="button"
          onClick={() => onChange(value - 1)}
          disabled={value <= min}
          className="flex h-8 w-8 items-center justify-center rounded-full border border-slate-300 text-brand transition hover:border-accent hover:text-accent disabled:cursor-not-allowed disabled:opacity-40"
          aria-label={`Giảm ${label.toLowerCase()}`}
        >
          <Minus size={14} />
        </button>
        <span className="w-5 text-center text-sm font-semibold tabular-nums" aria-live="polite">
          {value}
        </span>
        <button
          type="button"
          onClick={() => onChange(value + 1)}
          disabled={value >= max}
          className="flex h-8 w-8 items-center justify-center rounded-full border border-slate-300 text-brand transition hover:border-accent hover:text-accent disabled:cursor-not-allowed disabled:opacity-40"
          aria-label={`Tăng ${label.toLowerCase()}`}
        >
          <Plus size={14} />
        </button>
      </div>
    </div>
  );
}

// Chọn số người lớn / trẻ em / số phòng. Ràng buộc: số phòng không vượt quá số người lớn.
export default function GuestPicker({ value, onChange, error, id = 'guest-picker' }) {
  const [open, setOpen] = useState(false);
  const ref = useRef(null);
  useClickOutside(ref, () => setOpen(false), open);

  useEffect(() => {
    if (!open) return undefined;
    const onKey = (e) => e.key === 'Escape' && setOpen(false);
    document.addEventListener('keydown', onKey);
    return () => document.removeEventListener('keydown', onKey);
  }, [open]);

  const setAdults = (adults) => onChange({ adults, rooms: Math.min(value.rooms, adults) });
  const summary = `${value.adults} người lớn${value.children ? `, ${value.children} trẻ em` : ''}, ${value.rooms} phòng`;

  return (
    <div ref={ref} className="relative">
      <button
        type="button"
        id={id}
        onClick={() => setOpen((o) => !o)}
        aria-expanded={open}
        aria-haspopup="dialog"
        className={`block w-full rounded-2xl px-5 py-3 text-left transition hover:bg-surface focus-visible:bg-surface focus-visible:outline-none ${
          error ? 'ring-1 ring-red-300' : ''
        }`}
      >
        <span className="flex items-center justify-between gap-2 text-xs font-medium text-muted">
          <span className="flex items-center gap-1.5">
            <Users size={14} className="text-accent" /> Khách và phòng
          </span>
          <ChevronDown size={14} className={`transition-transform ${open ? 'rotate-180' : ''}`} />
        </span>
        <span className="mt-0.5 block truncate text-base font-semibold text-ink">{summary}</span>
      </button>

      {open && (
        <div
          role="dialog"
          aria-label="Chọn số khách và số phòng"
          className="absolute left-0 right-0 top-full z-30 mt-2 min-w-[18rem] c-fade-in divide-y divide-slate-100 rounded-2xl border border-slate-100 bg-white px-5 py-2 shadow-lift sm:left-auto sm:w-80 lg:left-0"
        >
          <CounterRow label="Người lớn" hint="Từ 13 tuổi" value={value.adults} min={1} max={SEARCH_LIMITS.maxAdults} onChange={setAdults} />
          <CounterRow label="Trẻ em" hint="Dưới 13 tuổi" value={value.children} min={0} max={SEARCH_LIMITS.maxChildren} onChange={(children) => onChange({ children })} />
          <CounterRow
            label="Số phòng"
            hint="Tối đa bằng số người lớn"
            value={value.rooms}
            min={1}
            max={Math.min(value.adults, SEARCH_LIMITS.maxRooms)}
            onChange={(rooms) => onChange({ rooms })}
          />
          <div className="py-3">
            <button type="button" className="c-btn-primary w-full" onClick={() => setOpen(false)}>
              Xong
            </button>
          </div>
        </div>
      )}
    </div>
  );
}
