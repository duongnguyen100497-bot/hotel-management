import { Link } from 'react-router-dom';
import { ArrowRight, Clock, Star } from 'lucide-react';
import Photo from './Photo.jsx';
import { useSettings } from '../../shared/contexts/SettingsContext.jsx';
import { CUSTOMER_ROUTES } from '../routes/paths.js';
import { formatCurrency } from '../../shared/utils/formatters.js';
import { HERO } from '../data/homeContent.js';

// Hero: tiêu đề lớn bên trái, khung cửa vòm với cảnh thành phố lúc hoàng hôn bên phải.
// `children` (thanh tìm phòng) chồng lên mép dưới của hero.
export default function HeroSection({ children, image, minPrice }) {
  const { hotel } = useSettings();
  return (
    <>
      <section className="relative isolate overflow-hidden bg-brand-dark" aria-labelledby="hero-title">
        <div className="absolute inset-0 -z-10 bg-gradient-to-br from-brand-dark via-brand to-brand-light" aria-hidden="true" />
        <div className="absolute -right-40 top-10 -z-10 h-[34rem] w-[34rem] rounded-full bg-accent/25 blur-3xl" aria-hidden="true" />

        <div className="c-container grid items-center gap-10 pb-40 pt-28 sm:pt-32 lg:grid-cols-[1.1fr_0.9fr] lg:gap-6 lg:pb-44 lg:pt-36">
          <div className="relative z-10">
            <h1 id="hero-title" className="text-[2.75rem] font-extrabold leading-[1.02] tracking-tight text-white sm:text-6xl lg:text-7xl">
              {HERO.lines.map((line) => (
                <span key={line} className="hero-line">
                  {line}
                </span>
              ))}
            </h1>
            <div className="hero-fade">
              <p className="mt-6 max-w-lg text-base leading-relaxed text-sky-100/90 sm:text-lg">{HERO.description}</p>

              <div className="mt-8 flex flex-wrap items-center gap-3">
                <Link to={CUSTOMER_ROUTES.ROOMS} className="c-btn-primary c-btn-lg">
                  Khám phá phòng <ArrowRight size={18} />
                </Link>
                <Link to="/#about" className="c-btn-glass c-btn-lg">
                  Về {hotel.name}
                </Link>
              </div>

              <p className="mt-8 flex items-center gap-2 text-sm text-sky-200">
                <Clock size={16} /> Nhận phòng từ {hotel.checkInTime}, trả phòng trước {hotel.checkOutTime}
              </p>
            </div>
          </div>

          <div className="relative mx-auto w-full max-w-sm sm:max-w-md lg:max-w-none">
            <div className="hero-arch c-arch relative aspect-[4/5] w-full overflow-hidden shadow-2xl ring-1 ring-white/20 lg:mx-auto lg:aspect-[3/4] lg:max-h-[600px] lg:max-w-md">
              <Photo src={image} variant="hero" alt={`${hotel.name} lúc hoàng hôn (ảnh minh họa)`} className="absolute inset-0 h-full w-full" />
            </div>
            {minPrice > 0 && (
              <div className="hero-fade absolute -bottom-5 left-2 flex items-center gap-3 rounded-2xl bg-white px-4 py-3 shadow-lift sm:left-0 lg:-left-2">
                <span className="flex h-10 w-10 items-center justify-center rounded-full bg-warning/15 text-warning">
                  <Star size={18} className="fill-warning" />
                </span>
                <span className="leading-tight">
                  <span className="block text-xs text-muted">{hotel.stars} sao · Giá từ</span>
                  <span className="block text-lg font-extrabold text-brand">
                    {formatCurrency(minPrice)}
                    <span className="text-xs font-medium text-muted"> / đêm</span>
                  </span>
                </span>
              </div>
            )}
          </div>
        </div>
      </section>

      <div className="c-container relative z-10 -mt-20 sm:-mt-24">{children}</div>
    </>
  );
}
