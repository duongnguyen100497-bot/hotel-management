import { Hotel } from 'lucide-react';
import { useSettings } from '../../../shared/contexts/SettingsContext.jsx';

// Logo khách sạn: dùng ảnh đã tải lên (Cài đặt) hoặc biểu tượng mặc định.
export default function Logo({ size = 40, showName = true, dark = false, className = '' }) {
  const { hotel } = useSettings();
  return (
    <div className={`flex items-center gap-3 ${className}`}>
      {hotel.logo ? (
        <img
          src={hotel.logo}
          alt={hotel.name}
          style={{ width: size, height: size }}
          className="rounded-xl bg-white object-contain p-0.5"
        />
      ) : (
        <span
          style={{ width: size, height: size }}
          className="flex items-center justify-center rounded-xl bg-gradient-to-br from-sky-300 to-sky-500 text-navy-950"
        >
          <Hotel size={size * 0.55} />
        </span>
      )}
      {showName && (
        <div className="min-w-0 leading-tight">
          <p className={`truncate text-base font-semibold ${dark ? 'text-white' : 'text-navy-900'}`}>{hotel.name}</p>
          <p className={`text-[11px] uppercase tracking-widest ${dark ? 'text-sky-200' : 'text-slate-400'}`}>
            Hotel Management
          </p>
        </div>
      )}
    </div>
  );
}
