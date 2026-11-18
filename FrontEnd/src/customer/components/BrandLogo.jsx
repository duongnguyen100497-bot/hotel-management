import { Link } from 'react-router-dom';
import { Hotel } from 'lucide-react';
import { useSettings } from '../../shared/contexts/SettingsContext.jsx';
import { CUSTOMER_ROUTES } from '../routes/paths.js';

// Logo khách sạn cho website khách hàng (lấy tên/logo từ Cài đặt của Admin).
export default function BrandLogo({ light = false, size = 40, className = '' }) {
  const { hotel } = useSettings();
  return (
    <Link to={CUSTOMER_ROUTES.HOME} className={`flex items-center gap-3 ${className}`} aria-label={`${hotel.name} – Trang chủ`}>
      {hotel.logo ? (
        <img src={hotel.logo} alt="" style={{ width: size, height: size }} className="rounded-xl bg-white object-contain p-0.5" />
      ) : (
        <span
          style={{ width: size, height: size }}
          className={`flex items-center justify-center rounded-xl ${light ? 'bg-white text-brand' : 'bg-brand text-white'}`}
        >
          <Hotel size={size * 0.52} />
        </span>
      )}
      <span className="leading-tight">
        <span className={`block text-lg font-bold tracking-tight ${light ? 'text-white' : 'text-brand'}`}>{hotel.name}</span>
        <span className={`block text-xs ${light ? 'text-sky-200' : 'text-muted'}`}>Khách sạn {hotel.stars} sao</span>
      </span>
    </Link>
  );
}
