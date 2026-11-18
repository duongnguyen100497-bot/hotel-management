import { Link, useNavigate } from 'react-router-dom';
import { History, KeyRound, LayoutDashboard, LogOut, CalendarCheck, UserRound } from 'lucide-react';
import Avatar from './Avatar.jsx';
import { useCustomerAuth } from '../contexts/CustomerAuthContext.jsx';
import { CUSTOMER_ROUTES } from '../routes/paths.js';

const ITEMS = [
  { key: 'overview', label: 'Tổng quan tài khoản', icon: LayoutDashboard, to: CUSTOMER_ROUTES.ACCOUNT },
  { key: 'profile', label: 'Thông tin cá nhân', icon: UserRound, to: `${CUSTOMER_ROUTES.ACCOUNT}?tab=profile` },
  { key: 'bookings', label: 'Đặt phòng của tôi', icon: CalendarCheck, to: CUSTOMER_ROUTES.MY_BOOKINGS },
  { key: 'history', label: 'Lịch sử đặt phòng', icon: History, to: `${CUSTOMER_ROUTES.MY_BOOKINGS}?tab=done` },
  { key: 'password', label: 'Đổi mật khẩu', icon: KeyRound, to: `${CUSTOMER_ROUTES.ACCOUNT}?tab=password` },
];

// Thanh bên của khu vực tài khoản. `active` là key của mục đang mở.
export default function AccountNav({ active }) {
  const { customer, account, logout } = useCustomerAuth();
  const navigate = useNavigate();
  const name = customer?.fullName || account?.email || 'Khách';
  return (
    <nav aria-label="Tài khoản" className="rounded-3xl border border-slate-100 bg-white p-3 shadow-soft">
      <div className="flex items-center gap-3 px-3 pb-4 pt-3">
        <Avatar name={name} size={48} className="!bg-accent-soft !text-accent" />
        <div className="min-w-0">
          <p className="truncate font-bold text-brand">{name}</p>
          <p className="truncate text-xs text-muted">{account?.email}</p>
        </div>
      </div>
      <ul className="space-y-0.5 border-t border-slate-100 pt-3">
        {ITEMS.map(({ key, label, icon: Icon, to }) => (
          <li key={key}>
            <Link to={to} aria-current={active === key ? 'page' : undefined} className={`flex items-center gap-3 rounded-xl px-3 py-2.5 text-sm font-medium transition-colors ${active === key ? 'bg-accent-soft text-accent' : 'text-slate-700 hover:bg-surface'}`}>
              <Icon size={18} /> {label}
            </Link>
          </li>
        ))}
        <li>
          <button type="button" onClick={() => { logout(); navigate(CUSTOMER_ROUTES.HOME); }} className="flex w-full items-center gap-3 rounded-xl px-3 py-2.5 text-left text-sm font-medium text-danger hover:bg-red-50">
            <LogOut size={18} /> Đăng xuất
          </button>
        </li>
      </ul>
    </nav>
  );
}
