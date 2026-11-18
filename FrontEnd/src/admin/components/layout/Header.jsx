import { useEffect, useRef, useState } from 'react';
import { Link, useLocation } from 'react-router-dom';
import { Bell, ChevronDown, LogOut, Menu, Settings as SettingsIcon } from 'lucide-react';
import Avatar from '../common/Avatar.jsx';
import { useAuth } from '../../contexts/AuthContext.jsx';
import useClickOutside from '../../../shared/hooks/useClickOutside.js';
import activityService from '../../services/activityService.js';
import { MODULES, ROLES } from '../../utils/permissions.js';
import { ROUTES } from '../../routes/paths.js';
import { timeAgo } from '../../../shared/utils/formatters.js';

export default function Header({ onMenuClick }) {
  const { user, logout, hasAccess } = useAuth();
  const { pathname } = useLocation();
  const [notifOpen, setNotifOpen] = useState(false);
  const [userOpen, setUserOpen] = useState(false);
  const [notifications, setNotifications] = useState([]);
  const notifRef = useRef(null);
  const userRef = useRef(null);

  useClickOutside(notifRef, () => setNotifOpen(false), notifOpen);
  useClickOutside(userRef, () => setUserOpen(false), userOpen);

  useEffect(() => {
    if (notifOpen) activityService.getRecent(5).then(setNotifications);
  }, [notifOpen]);

  const current = MODULES.find((m) => pathname.startsWith(m.path));
  const today = new Intl.DateTimeFormat('vi-VN', {
    weekday: 'long',
    day: '2-digit',
    month: '2-digit',
    year: 'numeric',
  }).format(new Date());

  return (
    <header className="sticky top-0 z-20 flex h-16 items-center gap-3 border-b border-slate-200 bg-white/90 px-4 backdrop-blur sm:px-6 print:hidden">
      <button type="button" onClick={onMenuClick} className="btn-icon lg:hidden" aria-label="Mở menu">
        <Menu size={20} />
      </button>

      <div className="min-w-0 flex-1">
        <h2 className="truncate text-base font-semibold text-navy-900 sm:text-lg">{current?.label || 'Hotel Management'}</h2>
        <p className="hidden text-xs capitalize text-slate-400 sm:block">{today}</p>
      </div>

      {/* Thông báo */}
      <div className="relative" ref={notifRef}>
        <button
          type="button"
          onClick={() => {
            setNotifOpen((v) => !v);
            setUserOpen(false);
          }}
          className="btn-icon relative"
          aria-label="Thông báo"
        >
          <Bell size={19} />
          <span className="absolute right-1.5 top-1.5 h-2 w-2 rounded-full bg-sky-500 ring-2 ring-white" />
        </button>
        {notifOpen && (
          <div className="absolute right-0 mt-2 w-80 max-w-[calc(100vw-2rem)] animate-fade-in overflow-hidden rounded-xl border border-slate-100 bg-white shadow-card-hover">
            <div className="border-b border-slate-100 px-4 py-3 text-sm font-semibold text-navy-900">Thông báo gần đây</div>
            <ul className="max-h-80 divide-y divide-slate-100 overflow-y-auto">
              {notifications.length === 0 && <li className="px-4 py-6 text-center text-sm text-slate-400">Đang tải...</li>}
              {notifications.map((n) => (
                <li key={n.id} className="px-4 py-3 transition-colors hover:bg-sky-50/50">
                  <p className="text-sm text-slate-700">{n.message}</p>
                  <p className="mt-0.5 text-xs text-slate-400">{timeAgo(n.time)}</p>
                </li>
              ))}
            </ul>
          </div>
        )}
      </div>

      {/* Người dùng */}
      <div className="relative" ref={userRef}>
        <button
          type="button"
          onClick={() => {
            setUserOpen((v) => !v);
            setNotifOpen(false);
          }}
          className="flex items-center gap-2 rounded-lg py-1 pl-1 pr-2 transition-colors hover:bg-slate-100"
        >
          <Avatar name={user?.name} size={34} />
          <div className="hidden text-left leading-tight sm:block">
            <p className="text-sm font-medium text-slate-800">{user?.name}</p>
            <p className="text-xs text-slate-400">{ROLES[user?.role]}</p>
          </div>
          <ChevronDown size={15} className="hidden text-slate-400 sm:block" />
        </button>
        {userOpen && (
          <div className="absolute right-0 mt-2 w-56 animate-fade-in overflow-hidden rounded-xl border border-slate-100 bg-white py-1 shadow-card-hover">
            <div className="border-b border-slate-100 px-4 py-3">
              <p className="truncate text-sm font-medium text-slate-800">{user?.name}</p>
              <p className="truncate text-xs text-slate-400">{user?.email}</p>
            </div>
            {hasAccess('settings') && (
              <Link
                to={ROUTES.SETTINGS}
                onClick={() => setUserOpen(false)}
                className="flex items-center gap-2 px-4 py-2.5 text-sm text-slate-600 transition-colors hover:bg-sky-50"
              >
                <SettingsIcon size={16} /> Tài khoản & cài đặt
              </Link>
            )}
            <button
              type="button"
              onClick={logout}
              className="flex w-full items-center gap-2 px-4 py-2.5 text-sm text-red-600 transition-colors hover:bg-red-50"
            >
              <LogOut size={16} /> Đăng xuất
            </button>
          </div>
        )}
      </div>
    </header>
  );
}
