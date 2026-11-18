import { Link } from 'react-router-dom';
import { Clock, Mail, MapPin, Phone } from 'lucide-react';
import BrandLogo from './BrandLogo.jsx';
import { useSettings } from '../../shared/contexts/SettingsContext.jsx';
import { FOOTER_LINKS } from '../data/homeContent.js';

function LinkColumn({ title, links }) {
  return (
    <div>
      <h3 className="text-base font-bold text-white">{title}</h3>
      <ul className="mt-4 space-y-2.5 text-sm">
        {links.map((l) => (
          <li key={l.label}>
            <Link to={l.to} className="text-sky-200/80 transition-colors hover:text-white">
              {l.label}
            </Link>
          </li>
        ))}
      </ul>
    </div>
  );
}

export default function SiteFooter() {
  const { hotel } = useSettings();
  return (
    <footer className="bg-brand-dark text-sky-100">
      <div className="c-container grid gap-10 py-14 sm:grid-cols-2 lg:grid-cols-[1.4fr_1fr_1fr_1.4fr]">
        <div>
          <BrandLogo light />
          <p className="mt-4 max-w-xs text-sm leading-relaxed text-sky-200/80">
            Trải nghiệm lưu trú tinh tế giữa trung tâm thành phố, phục vụ tận tâm 24/7.
          </p>
        </div>

        <LinkColumn title="Khám phá" links={FOOTER_LINKS.explore} />
        <LinkColumn title="Tài khoản" links={FOOTER_LINKS.account} />

        <div>
          <h3 className="text-base font-bold text-white">Liên hệ</h3>
          <ul className="mt-4 space-y-3 text-sm text-sky-200/80">
            <li className="flex gap-2.5">
              <MapPin size={16} className="mt-0.5 shrink-0 text-sky-300" /> {hotel.address}
            </li>
            <li className="flex gap-2.5">
              <Phone size={16} className="mt-0.5 shrink-0 text-sky-300" />
              <a href={`tel:${hotel.phone.replace(/\s/g, '')}`} className="hover:text-white">
                {hotel.phone}
              </a>
            </li>
            <li className="flex gap-2.5">
              <Mail size={16} className="mt-0.5 shrink-0 text-sky-300" />
              <a href={`mailto:${hotel.email}`} className="hover:text-white">
                {hotel.email}
              </a>
            </li>
            <li className="flex gap-2.5">
              <Clock size={16} className="mt-0.5 shrink-0 text-sky-300" />
              Nhận phòng {hotel.checkInTime} · Trả phòng {hotel.checkOutTime}
            </li>
          </ul>
        </div>
      </div>

      <div className="border-t border-white/10">
        <div className="c-container flex flex-col items-center justify-between gap-2 py-5 text-xs text-sky-200/60 sm:flex-row">
          <p>
            © {new Date().getFullYear()} {hotel.name}. Bảo lưu mọi quyền.
          </p>
        </div>
      </div>
    </footer>
  );
}
