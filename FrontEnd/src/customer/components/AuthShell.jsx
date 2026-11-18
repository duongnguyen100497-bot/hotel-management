import { Link } from 'react-router-dom';
import Photo from './Photo.jsx';
import { useSettings } from '../../shared/contexts/SettingsContext.jsx';

// Khung chung cho Đăng nhập / Đăng ký: cửa vòm minh họa bên trái, biểu mẫu bên phải.
export default function AuthShell({ title, subtitle, children, footer }) {
  const { hotel } = useSettings();
  return (
    <div className="c-container grid min-h-[calc(100vh-72px)] items-center gap-12 py-10 lg:grid-cols-[1fr_1.05fr] lg:py-14">
      <div className="relative hidden lg:block">
        <Photo variant="hero" alt={`${hotel.name} lúc hoàng hôn (ảnh minh họa)`} className="c-arch mx-auto aspect-[3/4] w-full max-w-md shadow-lift" />
        <p className="mx-auto mt-6 max-w-md text-center text-sm text-muted">
          Đăng nhập để đặt phòng nhanh hơn, theo dõi đặt phòng và quản lý thông tin của bạn tại {hotel.name}.
        </p>
      </div>
      <div className="mx-auto w-full max-w-md">
        <h1 className="text-3xl font-extrabold tracking-tight text-brand sm:text-4xl">{title}</h1>
        {subtitle && <p className="mt-3 text-muted">{subtitle}</p>}
        <div className="mt-8">{children}</div>
        {footer && <div className="mt-8 border-t border-slate-100 pt-6 text-center text-sm text-muted">{footer}</div>}
        <p className="mt-6 text-center text-xs text-muted">
          <Link to="/" className="hover:text-accent">Về trang chủ</Link>
        </p>
      </div>
    </div>
  );
}
