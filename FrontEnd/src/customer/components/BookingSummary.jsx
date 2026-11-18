import { Link } from 'react-router-dom';
import { AlertCircle, CalendarDays, CheckCircle2 } from 'lucide-react';
import GuestPicker from './GuestPicker.jsx';
import { CUSTOMER_ROUTES } from '../routes/paths.js';
import { addDays, todayStr } from '../../shared/utils/dateUtils.js';
import { countNights, searchToQuery, validateSearch } from '../utils/bookingSearch.js';
import { BOOKING_POLICY } from '../data/bookingPolicy.js';
import { calcPricing } from '../utils/pricing.js';
import { formatCurrency } from '../../shared/utils/formatters.js';

function Row({ label, value, strong = false }) {
  return (
    <div className={`flex items-baseline justify-between gap-4 ${strong ? 'text-base font-bold text-brand' : 'text-sm text-slate-600'}`}>
      <dt>{label}</dt>
      <dd className={strong ? 'text-xl font-extrabold' : 'font-medium text-ink'}>{value}</dd>
    </div>
  );
}

// Tóm tắt đặt phòng ở trang chi tiết: chọn ngày/khách, xem tổng tiền và bấm đặt.
export default function BookingSummary({ room, search, onChange, idPrefix = 'summary' }) {
  const errors = validateSearch(search);
  const valid = Object.keys(errors).length === 0;
  const nights = countNights(search.checkIn, search.checkOut);
  const price = calcPricing({ price: room.price, nights, rooms: search.rooms, promoPercent: room.promoPercent });
  const bookable = valid && room.available;
  const messages = Object.values(errors);

  const changeCheckIn = (checkIn) =>
    onChange({ checkIn, checkOut: checkIn && search.checkOut <= checkIn ? addDays(checkIn, 1) : search.checkOut });

  let availability = null;
  if (valid) {
    if (room.available) availability = { ok: true, text: room.availableCount <= 3 ? `Còn ${room.availableCount} phòng cho ngày này` : 'Còn phòng cho ngày bạn chọn' };
    else if (!room.fitsGuests) availability = { ok: false, text: `Hạng phòng này chứa tối đa ${room.capacity} khách / phòng. Hãy tăng số phòng hoặc chọn hạng khác.` };
    else availability = { ok: false, text: room.availableCount === 0 ? 'Hết phòng trong khoảng ngày này' : `Chỉ còn ${room.availableCount} phòng, ít hơn số phòng bạn cần` };
  }

  return (
    <div className="rounded-3xl border border-slate-100 bg-white p-6 shadow-lift">
      <p className="text-sm text-muted">Giá mỗi đêm</p>
      <p className="mt-0.5 text-3xl font-extrabold text-brand">
        {formatCurrency(price.perNight)}
        {room.promoPercent > 0 && <span className="ml-2 text-base font-medium text-muted line-through">{formatCurrency(room.price)}</span>}
      </p>

      <div className="mt-5 divide-y divide-slate-100 rounded-2xl border border-slate-200">
        <div className="grid grid-cols-2 divide-x divide-slate-100">
          {[
            [`${idPrefix}-in`, 'Nhận phòng', search.checkIn, todayStr(), changeCheckIn],
            [`${idPrefix}-out`, 'Trả phòng', search.checkOut, search.checkIn ? addDays(search.checkIn, 1) : todayStr(), (v) => onChange({ checkOut: v })],
          ].map(([id, label, value, min, fn]) => (
            <label key={id} htmlFor={id} className="block cursor-pointer px-4 py-3 hover:bg-surface">
              <span className="flex items-center gap-1.5 text-xs font-medium text-muted"><CalendarDays size={13} className="text-accent" /> {label}</span>
              <input id={id} type="date" value={value} min={min} onChange={(e) => fn(e.target.value)} className="mt-0.5 block w-full bg-transparent text-sm font-semibold text-ink focus:outline-none" />
            </label>
          ))}
        </div>
        <GuestPicker id={`${idPrefix}-guests`} value={search} onChange={onChange} />
      </div>

      {messages.length > 0 && (
        <ul role="alert" className="mt-3 space-y-1">
          {messages.map((m) => (
            <li key={m} className="flex items-start gap-1.5 text-sm text-danger"><AlertCircle size={14} className="mt-0.5 shrink-0" /> {m}</li>
          ))}
        </ul>
      )}

      {availability && (
        <p className={`mt-4 flex items-start gap-2 rounded-xl px-3 py-2.5 text-sm font-medium ${availability.ok ? 'bg-green-50 text-success' : 'bg-red-50 text-danger'}`} role="status">
          {availability.ok ? <CheckCircle2 size={16} className="mt-0.5 shrink-0" /> : <AlertCircle size={16} className="mt-0.5 shrink-0" />} {availability.text}
        </p>
      )}

      {valid && (
        <dl className="mt-5 space-y-2.5 border-t border-slate-100 pt-5">
          <Row label={`${formatCurrency(price.perNight)} × ${nights} đêm × ${price.rooms} phòng`} value={formatCurrency(price.roomCharge)} />
          <Row label={`Phí dịch vụ (${Math.round(BOOKING_POLICY.serviceFeeRate * 100)}%)`} value={formatCurrency(price.serviceFee)} />
          <Row label={`Thuế VAT (${Math.round(BOOKING_POLICY.vatRate * 100)}%)`} value={formatCurrency(price.vat)} />
          <div className="border-t border-slate-100 pt-3">
            <Row strong label="Tổng cộng" value={formatCurrency(price.total)} />
          </div>
        </dl>
      )}

      {bookable ? (
        <Link to={`${CUSTOMER_ROUTES.BOOKING(room.id)}?${searchToQuery(search)}`} className="c-btn-primary c-btn-lg mt-6 w-full">
          Đặt phòng ngay
        </Link>
      ) : (
        <button type="button" className="c-btn-primary c-btn-lg mt-6 w-full" disabled>
          {valid ? 'Không thể đặt phòng này' : 'Chọn ngày hợp lệ để đặt'}
        </button>
      )}
      <p className="mt-3 text-center text-xs text-muted">Bạn có thể xem lại toàn bộ thông tin trước khi xác nhận.</p>
    </div>
  );
}
