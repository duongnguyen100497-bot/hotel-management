import { CalendarDays, Moon, Users } from 'lucide-react';
import Photo from './Photo.jsx';
import { BOOKING_POLICY } from '../data/bookingPolicy.js';
import { countNights } from '../utils/bookingSearch.js';
import { calcPricing } from '../utils/pricing.js';
import { formatCurrency, formatDate } from '../../shared/utils/formatters.js';

function Line({ label, value, strong }) {
  return (
    <div className={`flex items-baseline justify-between gap-4 ${strong ? 'text-brand' : 'text-sm text-slate-600'}`}>
      <dt className={strong ? 'font-bold' : ''}>{label}</dt>
      <dd className={strong ? 'text-2xl font-extrabold' : 'font-medium text-ink'}>{value}</dd>
    </div>
  );
}

// Thẻ tóm tắt đặt phòng + chi tiết giá, dùng ở các bước của quy trình đặt phòng.
export default function PriceSummary({ room, search }) {
  const nights = countNights(search.checkIn, search.checkOut);
  const price = calcPricing({ price: room.price, nights, rooms: search.rooms, promoPercent: room.promoPercent });
  return (
    <aside aria-label="Tóm tắt đặt phòng" className="overflow-hidden rounded-3xl border border-slate-100 bg-white shadow-lift">
      <Photo src={room.images[0]} variant={room.id} alt={`${room.name}${room.images[0] ? '' : ' (ảnh minh họa)'}`} className="aspect-[16/8] w-full" />
      <div className="p-6">
        <h2 className="text-xl font-bold text-brand">{room.name}</h2>
        <p className="mt-0.5 text-sm text-muted">{room.bedType}{room.area ? `, ${room.area} m²` : ''}</p>

        <ul className="mt-4 space-y-2 text-sm text-slate-700">
          <li className="flex items-center gap-2.5"><CalendarDays size={16} className="text-accent" /> {formatDate(search.checkIn)} – {formatDate(search.checkOut)}</li>
          <li className="flex items-center gap-2.5"><Moon size={16} className="text-accent" /> {nights} đêm, {search.rooms} phòng</li>
          <li className="flex items-center gap-2.5"><Users size={16} className="text-accent" /> {search.adults} người lớn{search.children ? `, ${search.children} trẻ em` : ''}</li>
        </ul>

        <dl className="mt-5 space-y-2.5 border-t border-slate-100 pt-5">
          <Line label={`${formatCurrency(price.perNight)} × ${nights} đêm × ${price.rooms} phòng`} value={formatCurrency(price.roomCharge)} />
          <Line label={`Phí dịch vụ (${Math.round(BOOKING_POLICY.serviceFeeRate * 100)}%)`} value={formatCurrency(price.serviceFee)} />
          <Line label={`Thuế VAT (${Math.round(BOOKING_POLICY.vatRate * 100)}%)`} value={formatCurrency(price.vat)} />
          <div className="border-t border-slate-100 pt-3">
            <Line strong label="Tổng thanh toán" value={formatCurrency(price.total)} />
          </div>
        </dl>
      </div>
    </aside>
  );
}
