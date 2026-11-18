import { Link } from 'react-router-dom';
import { ArrowRight, BedDouble, Ruler, Users } from 'lucide-react';
import Photo from './Photo.jsx';
import { ROOM_AMENITIES } from '../data/amenities.js';
import { CARD_AMENITY_LIMIT } from '../data/roomCatalog.js';
import { CUSTOMER_ROUTES } from '../routes/paths.js';
import { formatCurrency } from '../../shared/utils/formatters.js';
import { unitPrice } from '../utils/pricing.js';

// Trạng thái còn phòng hiển thị cho khách (undefined = chưa có thông tin tìm kiếm).
export function availabilityLabel(room) {
  if (room.available === undefined) return null;
  if (room.available) return { ok: true, text: room.availableCount <= 3 ? `Chỉ còn ${room.availableCount} phòng` : 'Còn phòng' };
  if (!room.fitsGuests) return { ok: false, text: `Tối đa ${room.capacity} khách / phòng` };
  return { ok: false, text: 'Hết phòng' };
}

export function PriceBlock({ room, className = '' }) {
  const promo = room.promoPercent > 0;
  return (
    <div className={className}>
      {promo && <p className="text-sm text-muted line-through">{formatCurrency(room.price)}</p>}
      <p className="text-2xl font-extrabold leading-none text-brand">
        {formatCurrency(unitPrice(room.price, room.promoPercent))}
        <span className="ml-1 text-sm font-medium text-muted">/ đêm</span>
      </p>
    </div>
  );
}

// Thẻ hạng phòng với đỉnh cửa vòm. `query` giữ lại tham số tìm kiếm khi sang trang chi tiết / đặt phòng.
export default function RoomCard({ room, query = '', className = '' }) {
  const suffix = query ? `?${query}` : '';
  const status = availabilityLabel(room);
  const canBook = room.available !== false;
  const amenityKeys = room.amenities.filter((k) => ROOM_AMENITIES[k]);
  const shown = amenityKeys.slice(0, CARD_AMENITY_LIMIT);
  const extra = amenityKeys.length - shown.length;

  return (
    <article className={`group flex h-full flex-col ${className}`}>
      <Link to={`${CUSTOMER_ROUTES.ROOM_DETAIL(room.id)}${suffix}`} className="block focus:outline-none focus-visible:ring-2 focus-visible:ring-accent focus-visible:ring-offset-4 c-arch" aria-label={`Xem chi tiết ${room.name}`}>
        <Photo
          src={room.images[0]}
          variant={room.id}
          alt={`${room.name}${room.images[0] ? '' : ' (ảnh minh họa)'}`}
          className="c-arch aspect-[1/1.08] w-full shadow-soft"
        >
          <span className="absolute bottom-4 left-4 rounded-full bg-white px-3 py-1 text-sm font-semibold text-brand shadow-soft">{room.type}</span>
          {status && (
            <span className={`absolute bottom-4 right-4 rounded-full px-3 py-1 text-xs font-semibold shadow-soft ${status.ok ? 'bg-success text-white' : 'bg-slate-800 text-white'}`}>
              {status.text}
            </span>
          )}
        </Photo>
      </Link>

      <div className="flex flex-1 flex-col px-1 pt-5">
        <h3 className="text-xl font-bold text-brand">
          <Link to={`${CUSTOMER_ROUTES.ROOM_DETAIL(room.id)}${suffix}`} className="rounded transition-colors hover:text-accent focus:outline-none focus-visible:ring-2 focus-visible:ring-accent focus-visible:ring-offset-2">
            {room.name}
          </Link>
        </h3>
        <p className="mt-1 text-sm leading-relaxed text-muted">{room.highlight}</p>

        <ul className="mt-4 flex flex-wrap gap-x-5 gap-y-2 text-sm text-slate-700">
          {room.area && (
            <li className="flex items-center gap-1.5"><Ruler size={15} className="text-accent" /> {room.area} m²</li>
          )}
          <li className="flex items-center gap-1.5"><Users size={15} className="text-accent" /> Tối đa {room.capacity} khách</li>
          <li className="flex items-center gap-1.5"><BedDouble size={15} className="text-accent" /> {room.bedType}</li>
        </ul>

        <ul className="mt-4 flex flex-wrap gap-1.5" aria-label="Tiện nghi">
          {shown.map((key) => {
            const { label, icon: Icon } = ROOM_AMENITIES[key];
            return (
              <li key={key} className="inline-flex items-center gap-1 rounded-full bg-surface px-2.5 py-1 text-xs text-slate-600">
                <Icon size={13} className="text-accent" /> {label}
              </li>
            );
          })}
          {extra > 0 && <li className="inline-flex items-center rounded-full bg-surface px-2.5 py-1 text-xs font-medium text-muted">+{extra} tiện nghi</li>}
        </ul>

        <div className="mt-auto flex items-end justify-between gap-3 pt-6">
          <PriceBlock room={room} />
          {canBook ? (
            <Link to={`${CUSTOMER_ROUTES.BOOKING(room.id)}${suffix}`} className="c-btn-primary">
              Đặt phòng <ArrowRight size={16} />
            </Link>
          ) : (
            <button type="button" className="c-btn-primary" disabled>
              Hết phòng
            </button>
          )}
        </div>
      </div>
    </article>
  );
}
