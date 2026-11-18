import { useCallback, useMemo } from 'react';
import { Link, useParams, useSearchParams } from 'react-router-dom';
import { ArrowLeft, ArrowRight, BedDouble, CalendarClock, ChevronRight, Eye, Ruler, ShieldCheck, Users } from 'lucide-react';
import RoomGallery from '../components/RoomGallery.jsx';
import BookingSummary from '../components/BookingSummary.jsx';
import RoomCard, { PriceBlock, availabilityLabel } from '../components/RoomCard.jsx';
import { EmptyBlock, ErrorBlock } from '../components/States.jsx';
import useFetch from '../../shared/hooks/useFetch.js';
import useDocumentTitle from '../hooks/useDocumentTitle.js';
import { useSettings } from '../../shared/contexts/SettingsContext.jsx';
import customerRoomService from '../services/customerRoomService.js';
import { ROOM_AMENITIES } from '../data/amenities.js';
import { CANCEL_RULES, CHECK_RULES } from '../data/bookingPolicy.js';
import { CUSTOMER_ROUTES } from '../routes/paths.js';
import { queryToSearch, searchToQuery } from '../utils/bookingSearch.js';

function Fact({ icon: Icon, label, value }) {
  return (
    <div className="flex items-center gap-3">
      <span className="flex h-11 w-11 items-center justify-center rounded-2xl bg-accent-soft text-accent"><Icon size={20} /></span>
      <div>
        <p className="text-xs text-muted">{label}</p>
        <p className="font-semibold text-brand">{value}</p>
      </div>
    </div>
  );
}

function Policy({ icon: Icon, title, items }) {
  return (
    <div className="rounded-3xl bg-surface p-6">
      <h3 className="flex items-center gap-2 text-base font-bold text-brand"><Icon size={18} className="text-accent" /> {title}</h3>
      <ul className="mt-3 space-y-2 text-sm leading-relaxed text-slate-600">
        {items.map((t) => (
          <li key={t} className="flex gap-2"><span className="mt-2 h-1.5 w-1.5 shrink-0 rounded-full bg-accent" aria-hidden="true" />{t}</li>
        ))}
      </ul>
    </div>
  );
}

export default function RoomDetail() {
  const { id } = useParams();
  const { hotel } = useSettings();
  const [params, setParams] = useSearchParams();
  const search = useMemo(() => queryToSearch(params), [params]);
  const searchKey = searchToQuery(search);

  const load = useCallback(() => customerRoomService.getRoomType(id, search), [id, searchKey]); // eslint-disable-line react-hooks/exhaustive-deps
  const { data, loading, error, reload } = useFetch(load);
  const room = data?.room;

  useDocumentTitle(room ? `${room.name} – ${hotel.name}` : 'Chi tiết phòng');

  const onChange = (patch) =>
    setParams((prev) => {
      const next = new URLSearchParams(prev);
      new URLSearchParams(searchToQuery({ ...search, ...patch })).forEach((v, k) => next.set(k, v));
      return next;
    }, { replace: true });

  const back = (
    <Link to={`${CUSTOMER_ROUTES.ROOMS}?${searchKey}`} className="inline-flex items-center gap-1.5 text-sm font-semibold text-accent hover:underline">
      <ArrowLeft size={16} /> Tất cả phòng nghỉ
    </Link>
  );

  if (loading && !room) {
    return (
      <div className="c-container py-10" role="status" aria-label="Đang tải chi tiết phòng">
        <div className="h-5 w-40 animate-pulse rounded bg-slate-200" />
        <div className="mt-6 h-[420px] animate-pulse rounded-3xl bg-slate-200" />
        <div className="mt-8 h-10 w-1/2 animate-pulse rounded bg-slate-200" />
      </div>
    );
  }
  if (error) {
    return (
      <div className="c-container py-14">
        {back}
        <div className="mt-6">
          {error.includes('Không tìm thấy') ? (
            <EmptyBlock title="Không tìm thấy hạng phòng" description="Đường dẫn có thể đã cũ hoặc hạng phòng không còn được bán." action={<Link to={CUSTOMER_ROUTES.ROOMS} className="c-btn-primary">Xem tất cả phòng</Link>} />
          ) : (
            <ErrorBlock message={error} onRetry={reload} />
          )}
        </div>
      </div>
    );
  }
  if (!room) return null;

  const amenities = room.amenities.filter((k) => ROOM_AMENITIES[k]);
  const status = availabilityLabel(room);
  const slides = room.images.length
    ? room.images.map((src, i) => ({ src, variant: [room.id, 'bath', 'view'][i] || room.id, alt: `${room.name} – ảnh ${i + 1}` }))
    : [
        { variant: room.id, alt: `${room.name}, phòng ngủ (ảnh minh họa)` },
        { variant: 'bath', alt: `${room.name}, phòng tắm (ảnh minh họa)` },
        { variant: 'view', alt: `${room.name}, tầm nhìn (ảnh minh họa)` },
      ];
  const rules = CHECK_RULES.map((t) => t.replace('{checkIn}', hotel.checkInTime).replace('{checkOut}', hotel.checkOutTime));
  const query = searchKey;

  return (
    <div className="pb-28 lg:pb-20">
      <div className="c-container py-6">
        <nav aria-label="Đường dẫn" className="flex items-center gap-1.5 text-sm text-muted">
          <Link to="/" className="hover:text-accent">Trang chủ</Link>
          <ChevronRight size={14} />
          <Link to={`${CUSTOMER_ROUTES.ROOMS}?${query}`} className="hover:text-accent">Phòng nghỉ</Link>
          <ChevronRight size={14} />
          <span className="font-medium text-brand" aria-current="page">{room.name}</span>
        </nav>
      </div>

      <div className="c-container"><RoomGallery slides={slides} /></div>

      <div className="c-container mt-10 grid gap-12 lg:grid-cols-[1fr_24rem] lg:gap-14">
        <div>
          <div className="flex flex-wrap items-center gap-3">
            <h1 className="text-3xl font-extrabold tracking-tight text-brand sm:text-4xl">{room.name}</h1>
            {status && (
              <span className={`rounded-full px-3 py-1 text-xs font-semibold ${status.ok ? 'bg-green-50 text-success' : 'bg-slate-100 text-slate-600'}`}>{status.text}</span>
            )}
          </div>
          <p className="mt-2 text-lg text-muted">{room.highlight}</p>

          <div className="mt-8 grid gap-5 rounded-3xl border border-slate-100 bg-white p-6 shadow-soft sm:grid-cols-2 xl:grid-cols-4">
            {room.area && <Fact icon={Ruler} label="Diện tích" value={`${room.area} m²`} />}
            <Fact icon={Users} label="Sức chứa" value={`Tối đa ${room.capacity} khách`} />
            <Fact icon={BedDouble} label="Loại giường" value={room.bedType} />
            {room.view && <Fact icon={Eye} label="Tầm nhìn" value={room.view} />}
          </div>

          <section className="mt-12" aria-labelledby="desc-title">
            <h2 id="desc-title" className="text-2xl font-bold text-brand">Về hạng phòng này</h2>
            <p className="mt-3 max-w-2xl text-base leading-relaxed text-slate-600">{room.description}</p>
          </section>

          <section className="mt-12" aria-labelledby="amen-title">
            <h2 id="amen-title" className="text-2xl font-bold text-brand">Tiện nghi trong phòng</h2>
            <ul className="mt-5 grid gap-3 sm:grid-cols-2 lg:grid-cols-3">
              {amenities.map((k) => {
                const { label, icon: Icon } = ROOM_AMENITIES[k];
                return (
                  <li key={k} className="flex items-center gap-3 rounded-2xl border border-slate-100 px-4 py-3 text-sm font-medium text-slate-700">
                    <Icon size={18} className="text-accent" /> {label}
                  </li>
                );
              })}
            </ul>
          </section>

          <section className="mt-12 grid gap-5 md:grid-cols-2" aria-label="Chính sách">
            <Policy icon={CalendarClock} title="Nhận phòng và trả phòng" items={rules} />
            <Policy icon={ShieldCheck} title="Chính sách hủy phòng" items={CANCEL_RULES} />
          </section>
        </div>

        <aside className="hidden lg:block" aria-label="Tóm tắt đặt phòng">
          <div className="sticky top-24"><BookingSummary room={room} search={search} onChange={onChange} /></div>
        </aside>
      </div>

      {/* Mobile: tóm tắt nằm trong trang, thanh giá cố định phía dưới */}
      <div className="c-container mt-10 lg:hidden"><BookingSummary room={room} search={search} onChange={onChange} idPrefix="m-summary" /></div>
      <div className="fixed inset-x-0 bottom-0 z-30 border-t border-slate-200 bg-white/95 px-4 py-3 backdrop-blur lg:hidden">
        <div className="mx-auto flex max-w-lg items-center justify-between gap-3">
          <PriceBlock room={room} />
          <a href="#m-summary-in" className="c-btn-primary" onClick={(e) => { e.preventDefault(); document.getElementById('m-summary-in')?.scrollIntoView({ behavior: 'smooth', block: 'center' }); }}>
            Chọn ngày <ArrowRight size={16} />
          </a>
        </div>
      </div>

      {data.others.length > 0 && (
        <section className="c-container mt-20" aria-labelledby="others-title">
          <h2 id="others-title" className="text-2xl font-bold text-brand sm:text-3xl">Hạng phòng khác</h2>
          <div className="mt-8 grid gap-x-8 gap-y-12 sm:grid-cols-2 lg:grid-cols-3">
            {data.others.slice(0, 3).map((r) => <RoomCard key={r.id} room={r} query={query} />)}
          </div>
        </section>
      )}
    </div>
  );
}
