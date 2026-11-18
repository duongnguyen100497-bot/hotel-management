import { Link } from 'react-router-dom';
import { ArrowRight, BedDouble } from 'lucide-react';
import HeroSection from '../components/HeroSection.jsx';
import SearchBooking from '../components/SearchBooking.jsx';
import RoomCard from '../components/RoomCard.jsx';
import SectionHeading from '../components/SectionHeading.jsx';
import StarRating from '../components/StarRating.jsx';
import Photo from '../components/Photo.jsx';
import Avatar from '../components/Avatar.jsx';
import { DataBlock } from '../components/States.jsx';
import useFetch from '../../shared/hooks/useFetch.js';
import useDocumentTitle from '../hooks/useDocumentTitle.js';
import { useSettings } from '../../shared/contexts/SettingsContext.jsx';
import customerRoomService from '../services/customerRoomService.js';
import { CUSTOMER_ROUTES } from '../routes/paths.js';
import { formatCurrency } from '../../shared/utils/formatters.js';
import { FEATURED_ROOM_TYPES } from '../data/roomCatalog.js';
import { SITE_PHOTOS } from '../data/photos.js';
import { ABOUT, AMENITIES, GALLERY, OFFERS, TESTIMONIALS } from '../data/homeContent.js';

function Fact({ value, label }) {
  return (
    <div className="border-l-2 border-accent/30 pl-4">
      <p className="text-2xl font-extrabold text-brand sm:text-3xl">{value}</p>
      <p className="mt-0.5 text-sm text-muted">{label}</p>
    </div>
  );
}

function AboutSection({ stats }) {
  const { hotel } = useSettings();
  return (
    <section id="about" className="c-section scroll-mt-20" aria-labelledby="about-title">
      <div className="c-container grid items-center gap-14 lg:grid-cols-[0.9fr_1.1fr] lg:gap-20">
        <div className="relative mx-auto w-full max-w-md lg:max-w-none">
          <Photo src={SITE_PHOTOS.lobby} variant="lobby" alt={`Sảnh ${hotel.name}`} className="c-arch aspect-[4/5] w-full shadow-lift" />
          <Photo
            src={SITE_PHOTOS.roomView}
            variant="view"
            alt="Tầm nhìn từ phòng"
            className="absolute -bottom-8 -right-3 aspect-square w-2/5 rounded-3xl border-[6px] border-white shadow-lift sm:-right-8"
          />
        </div>
        <div>
          <SectionHeading id="about-title" title={ABOUT.title} />
          {ABOUT.paragraphs.map((p) => (
            <p key={p} className="mt-5 max-w-xl text-base leading-relaxed text-muted sm:text-lg">
              {p}
            </p>
          ))}
          <div className="mt-10 grid max-w-xl grid-cols-2 gap-x-6 gap-y-8">
            <Fact value={`${hotel.stars} sao`} label="Tiêu chuẩn khách sạn" />
            <Fact value={stats ? `${stats.totalRooms} phòng` : '…'} label={stats ? `${stats.typeCount} hạng phòng` : 'Nhiều hạng phòng'} />
            <Fact value={stats ? formatCurrency(stats.minPrice) : '…'} label="Giá từ mỗi đêm" />
            <Fact value={hotel.checkInTime} label="Giờ nhận phòng" />
          </div>
        </div>
      </div>
    </section>
  );
}

function FeaturedRooms({ data, loading, error, reload }) {
  const featured = FEATURED_ROOM_TYPES.map((t) => (data?.types ?? []).find((x) => x.type === t)).filter(Boolean);
  return (
    <section id="rooms" className="c-section scroll-mt-20 bg-surface" aria-labelledby="rooms-title">
      <div className="c-container">
        <div className="flex flex-wrap items-end justify-between gap-6">
          <SectionHeading
            id="rooms-title"
            title="Chọn không gian dành cho bạn"
            description="Ba hạng phòng được đặt nhiều nhất. Giá và tình trạng phòng luôn khớp với hệ thống quản lý của khách sạn."
          />
          <Link to={CUSTOMER_ROUTES.ROOMS} className="c-btn-outline c-btn-lg">
            Xem tất cả hạng phòng <ArrowRight size={18} />
          </Link>
        </div>

        <div className="mt-14 min-h-[14rem]">
          <DataBlock
            loading={loading}
            error={error}
            onRetry={reload}
            isEmpty={featured.length === 0}
            emptyProps={{ icon: BedDouble, title: 'Chưa có phòng để hiển thị', description: 'Vui lòng quay lại sau.' }}
          >
            <div className="grid gap-x-8 gap-y-14 md:grid-cols-2 lg:grid-cols-3">
              {featured.map((room, i) => (
                <RoomCard key={room.id} room={room} className={i === 1 ? 'lg:mt-14' : ''} />
              ))}
            </div>
          </DataBlock>
        </div>
      </div>
    </section>
  );
}

function AmenitiesSection() {
  return (
    <section id="amenities" className="c-section scroll-mt-20" aria-labelledby="amenities-title">
      <div className="c-container grid gap-14 lg:grid-cols-[5fr_7fr] lg:gap-20">
        <div className="lg:sticky lg:top-28 lg:self-start">
          <SectionHeading id="amenities-title" title="Mọi thứ bạn cần cho một kỳ nghỉ trọn vẹn" description="Từ bữa sáng đến xe đưa đón sân bay, chúng tôi lo phần còn lại." />
          <Photo src={SITE_PHOTOS.pool} variant="pool" alt="Hồ bơi ngoài trời" className="mt-10 aspect-[5/4] w-full rounded-[2rem] shadow-lift" />
        </div>
        <ul className="grid gap-x-10 sm:grid-cols-2">
          {AMENITIES.map(({ icon: Icon, title, description }) => (
            <li key={title} className="group flex gap-4 border-t border-slate-200 py-7">
              <span className="flex h-12 w-12 shrink-0 items-center justify-center rounded-2xl bg-accent-soft text-accent transition-colors duration-300 group-hover:bg-accent group-hover:text-white">
                <Icon size={22} />
              </span>
              <div>
                <h3 className="text-base font-bold text-brand">{title}</h3>
                <p className="mt-1 text-sm leading-relaxed text-muted">{description}</p>
              </div>
            </li>
          ))}
        </ul>
      </div>
    </section>
  );
}

function OffersSection() {
  return (
    <section id="offers" className="c-section scroll-mt-20 bg-brand" aria-labelledby="offers-title">
      <div className="c-container">
        <SectionHeading light id="offers-title" title="Ưu đãi dành riêng cho khách đặt trực tuyến" description="Quyền lợi được khách sạn áp dụng khi xác nhận đặt phòng." />
        <ul className="mt-12 divide-y divide-white/15 border-y border-white/15">
          {OFFERS.map((offer) => (
            <li key={offer.id} className="group grid gap-3 py-8 transition-colors hover:bg-white/5 md:grid-cols-[1.1fr_1.4fr_auto] md:items-center md:gap-10 md:px-4">
              <h3 className="text-2xl font-bold text-white sm:text-3xl">{offer.title}</h3>
              <p className="max-w-lg text-base leading-relaxed text-sky-200/85">{offer.description}</p>
              <span className="w-fit rounded-full bg-warning px-3.5 py-1.5 text-sm font-semibold text-brand-dark">{offer.tag}</span>
            </li>
          ))}
        </ul>
      </div>
    </section>
  );
}

function TestimonialsSection() {
  const [lead, ...rest] = TESTIMONIALS;
  return (
    <section id="reviews" className="c-section scroll-mt-20" aria-labelledby="reviews-title">
      <div className="c-container">
        <SectionHeading id="reviews-title" title="Khách đã ở nói gì" />
        <div className="mt-12 grid gap-10 lg:grid-cols-[1.4fr_1fr] lg:gap-16">
          <figure>
            <StarRating value={lead.rating} />
            <blockquote className="mt-5 text-2xl font-semibold leading-snug tracking-tight text-brand sm:text-3xl">“{lead.quote}”</blockquote>
            <figcaption className="mt-8 flex items-center gap-3">
              <Avatar name={lead.name} size={48} className="!bg-accent-soft !text-accent" />
              <div>
                <p className="font-semibold text-brand">{lead.name}</p>
                <p className="text-sm text-muted">{lead.trip}</p>
              </div>
            </figcaption>
          </figure>
          <div className="space-y-6">
            {rest.map((t) => (
              <figure key={t.id} className="rounded-3xl bg-surface p-6">
                <StarRating value={t.rating} />
                <blockquote className="mt-3 text-sm leading-relaxed text-slate-600">“{t.quote}”</blockquote>
                <figcaption className="mt-4 flex items-center gap-3">
                  <Avatar name={t.name} size={36} className="!bg-white !text-accent" />
                  <div>
                    <p className="text-sm font-semibold text-brand">{t.name}</p>
                    <p className="text-xs text-muted">{t.trip}</p>
                  </div>
                </figcaption>
              </figure>
            ))}
          </div>
        </div>
      </div>
    </section>
  );
}

function GallerySection() {
  return (
    <section id="gallery" className="c-section scroll-mt-20 bg-surface" aria-labelledby="gallery-title">
      <div className="c-container">
        <SectionHeading id="gallery-title" title="Không gian của chúng tôi" />
        <div className="mt-12 grid auto-rows-[150px] grid-cols-2 gap-3 sm:auto-rows-[190px] sm:gap-4 md:grid-cols-4">
          {GALLERY.map((g) => (
            <div key={g.id} className={`group overflow-hidden rounded-3xl ${g.className || ''}`}>
              <Photo src={g.src} variant={g.variant} alt={g.label} label={g.label} className="h-full w-full" />
            </div>
          ))}
        </div>
      </div>
    </section>
  );
}

function CtaBand() {
  return (
    <section className="c-section" aria-label="Đặt phòng ngay">
      <div className="c-container">
        <div className="relative isolate grid items-center gap-8 overflow-hidden rounded-[2rem] bg-gradient-to-br from-brand-dark via-brand to-brand-light px-6 py-12 sm:px-12 sm:py-14 md:grid-cols-[1.2fr_0.8fr]">
          <div>
            <h2 className="text-3xl font-extrabold tracking-tight text-white sm:text-4xl">Chọn ngày, chọn phòng, xong trong vài bước.</h2>
            <p className="mt-4 max-w-md text-sky-100/90">Phòng được giữ ngay khi bạn gửi yêu cầu. Lễ tân sẽ xác nhận với bạn trước ngày nhận phòng.</p>
            <Link to={CUSTOMER_ROUTES.ROOMS} className="c-btn-light c-btn-lg mt-8">
              Đặt phòng ngay <ArrowRight size={18} />
            </Link>
          </div>
          <Photo src={SITE_PHOTOS.suite} variant="suite" alt="Phòng Suite" className="c-arch hidden aspect-[4/3] w-full max-w-xs justify-self-end ring-1 ring-white/20 md:block" />
        </div>
      </div>
    </section>
  );
}

export default function Home() {
  const { hotel } = useSettings();
  useDocumentTitle(`${hotel.name} – Đặt phòng trực tuyến`);
  const state = useFetch(customerRoomService.getOverview);

  return (
    <>
      <HeroSection minPrice={state.data?.stats.minPrice}>
        <SearchBooking />
      </HeroSection>
      <AboutSection stats={state.data?.stats} />
      <FeaturedRooms {...state} />
      <AmenitiesSection />
      <OffersSection />
      <TestimonialsSection />
      <GallerySection />
      <CtaBand />
    </>
  );
}
