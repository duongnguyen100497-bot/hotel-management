import { useState } from 'react';
import { useNavigate } from 'react-router-dom';
import { AlertCircle, CalendarDays, Search } from 'lucide-react';
import GuestPicker from './GuestPicker.jsx';
import { CUSTOMER_ROUTES } from '../routes/paths.js';
import { addDays, todayStr } from '../../shared/utils/dateUtils.js';
import { countNights, defaultSearch, searchToQuery, validateSearch } from '../utils/bookingSearch.js';

function DateField({ id, label, hint, value, min, onChange, error }) {
  return (
    <label
      htmlFor={id}
      className={`block cursor-pointer rounded-2xl px-5 py-3 transition hover:bg-surface focus-within:bg-surface ${error ? 'ring-1 ring-red-300' : ''}`}
    >
      <span className="flex items-center justify-between gap-2 text-xs font-medium text-muted">
        <span className="flex items-center gap-1.5">
          <CalendarDays size={14} className="text-accent" /> {label}
        </span>
        {hint && <span className="rounded-full bg-accent-soft px-2 py-0.5 font-semibold text-accent">{hint}</span>}
      </span>
      <input
        id={id}
        type="date"
        value={value}
        min={min}
        onChange={(e) => onChange(e.target.value)}
        aria-invalid={Boolean(error)}
        className="mt-0.5 block w-full bg-transparent text-base font-semibold text-ink focus:outline-none"
      />
    </label>
  );
}

// Thanh tìm phòng: dùng ở trang chủ (mặc định chuyển sang /rooms) hoặc trang danh sách (truyền onSearch).
export default function SearchBooking({ initial, onSearch, submitLabel = 'Tìm phòng', className = '' }) {
  const navigate = useNavigate();
  const [values, setValues] = useState(() => initial || defaultSearch());
  const [errors, setErrors] = useState({});

  const patch = (next) => {
    setValues((prev) => ({ ...prev, ...next }));
    setErrors({});
  };

  // Đổi ngày nhận phòng: tự đẩy ngày trả phòng lên nếu không còn hợp lệ.
  const changeCheckIn = (checkIn) => {
    setValues((prev) => ({
      ...prev,
      checkIn,
      checkOut: checkIn && prev.checkOut <= checkIn ? addDays(checkIn, 1) : prev.checkOut,
    }));
    setErrors({});
  };

  const submit = (e) => {
    e.preventDefault();
    const found = validateSearch(values);
    setErrors(found);
    if (Object.keys(found).length > 0) return;
    if (onSearch) onSearch(values);
    else navigate(`${CUSTOMER_ROUTES.ROOMS}?${searchToQuery(values)}`);
  };

  const nights = countNights(values.checkIn, values.checkOut);
  const messages = Object.values(errors);

  return (
    <form onSubmit={submit} noValidate className={`rounded-3xl bg-white p-2 shadow-lift ring-1 ring-slate-100 ${className}`} aria-label="Tìm phòng trống">
      <div className="grid grid-cols-1 gap-1 md:grid-cols-2 lg:grid-cols-[1fr_1fr_1.2fr_auto] lg:items-center">
        <DateField id="search-checkin" label="Nhận phòng" value={values.checkIn} min={todayStr()} onChange={changeCheckIn} error={errors.checkIn} />
        <DateField
          id="search-checkout"
          label="Trả phòng"
          hint={nights > 0 ? `${nights} đêm` : undefined}
          value={values.checkOut}
          min={values.checkIn ? addDays(values.checkIn, 1) : todayStr()}
          onChange={(checkOut) => patch({ checkOut })}
          error={errors.checkOut}
        />
        <GuestPicker value={values} onChange={patch} error={errors.guests} />
        <button type="submit" className="c-btn-primary c-btn-lg m-1 w-auto md:col-span-2 lg:col-span-1 lg:h-full lg:!rounded-2xl lg:!px-8">
          <Search size={18} /> {submitLabel}
        </button>
      </div>

      {messages.length > 0 && (
        <ul className="space-y-1 px-5 pb-3 pt-2" role="alert">
          {messages.map((m) => (
            <li key={m} className="flex items-center gap-1.5 text-sm text-danger">
              <AlertCircle size={14} className="shrink-0" /> {m}
            </li>
          ))}
        </ul>
      )}
    </form>
  );
}
