import { NavLink } from 'react-router-dom';
import {
  BarChart3,
  BedDouble,
  CalendarCheck,
  ConciergeBell,
  KeyRound,
  LayoutDashboard,
  LogOut,
  Receipt,
  Settings,
  UserCog,
  Users,
  X,
} from 'lucide-react';
import Logo from '../common/Logo.jsx';
import Avatar from '../common/Avatar.jsx';
import { useAuth } from '../../contexts/AuthContext.jsx';
import { MODULES, ROLES } from '../../utils/permissions.js';

const ICONS = {
  dashboard: LayoutDashboard,
  rooms: BedDouble,
  bookings: CalendarCheck,
  customers: Users,
  checkInOut: KeyRound,
  services: ConciergeBell,
  invoices: Receipt,
  reports: BarChart3,
  employees: UserCog,
  settings: Settings,
};

export default function Sidebar({ open, onClose }) {
  const { user, hasAccess, logout } = useAuth();
  const items = MODULES.filter((m) => hasAccess(m.key));

  return (
    <>
      {open && (
        <div
          className="fixed inset-0 z-40 bg-navy-950/50 backdrop-blur-[2px] animate-fade-in lg:hidden print:hidden"
          onClick={onClose}
          aria-hidden="true"
        />
      )}
      <aside
        className={`fixed inset-y-0 left-0 z-50 flex w-64 flex-col bg-gradient-to-b from-navy-900 to-navy-950 text-sky-100 shadow-xl transition-transform duration-300 lg:z-30 lg:translate-x-0 print:hidden ${
          open ? 'translate-x-0' : '-translate-x-full'
        }`}
      >
        <div className="flex h-16 items-center justify-between border-b border-white/10 px-5">
          <Logo size={36} dark />
          <button type="button" onClick={onClose} className="rounded-lg p-1.5 text-sky-200 hover:bg-white/10 lg:hidden" aria-label="Đóng menu">
            <X size={18} />
          </button>
        </div>

        <nav className="flex-1 space-y-1 overflow-y-auto px-3 py-4">
          {items.map((m) => {
            const Icon = ICONS[m.key];
            return (
              <NavLink
                key={m.key}
                to={m.path}
                onClick={onClose}
                className={({ isActive }) =>
                  `group flex items-center gap-3 rounded-lg px-3 py-2.5 text-sm font-medium transition-all duration-200 ${
                    isActive
                      ? 'bg-white/10 text-white shadow-inner ring-1 ring-white/10'
                      : 'text-sky-200/80 hover:bg-white/5 hover:text-white'
                  }`
                }
              >
                {({ isActive }) => (
                  <>
                    <Icon size={18} className={isActive ? 'text-sky-300' : 'text-sky-300/60 group-hover:text-sky-300'} />
                    {m.label}
                  </>
                )}
              </NavLink>
            );
          })}
        </nav>

        <div className="border-t border-white/10 p-3">
          <div className="flex items-center gap-3 rounded-lg bg-white/5 p-2.5">
            <Avatar name={user?.name} size={36} className="!bg-sky-200 !text-navy-900" />
            <div className="min-w-0 flex-1">
              <p className="truncate text-sm font-medium text-white">{user?.name}</p>
              <p className="truncate text-xs text-sky-200/70">{ROLES[user?.role]}</p>
            </div>
            <button
              type="button"
              onClick={logout}
              className="rounded-lg p-1.5 text-sky-200/80 transition-colors hover:bg-white/10 hover:text-white"
              aria-label="Đăng xuất"
              title="Đăng xuất"
            >
              <LogOut size={17} />
            </button>
          </div>
        </div>
      </aside>
    </>
  );
}
