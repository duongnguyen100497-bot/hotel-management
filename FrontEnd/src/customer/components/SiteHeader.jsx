import { useEffect, useRef, useState } from 'react';
import { Link, NavLink, useLocation, useNavigate } from 'react-router-dom';
import { CalendarCheck, ChevronDown, LogOut, Menu, UserRound, X } from 'lucide-react';
import Avatar from './Avatar.jsx';
import useClickOutside from '../../shared/hooks/useClickOutside.js';
import { useCustomerAuth } from '../contexts/CustomerAuthContext.jsx';
import BrandLogo from './BrandLogo.jsx';
import { CUSTOMER_ROUTES } from '../routes/paths.js';
import { NAV_LINKS } from '../data/homeContent.js';

const isHashLink = (to) => to.includes('#');

// Header website khách hàng. Trong suốt trên hero của trang chủ, chuyển sang nền trắng khi cuộn.
function AccountMenu({ overHero }) {
  const { customer, account, logout } = useCustomerAuth();
  const navigate = useNavigate();
  const [open, setOpen] = useState(false);
  const ref = useRef(null);
  useClickOutside(ref, () => setOpen(false), open);
  const name = customer?.fullName || account?.email || 'Khách';
  const itemClass = 'flex w-full items-center gap-2.5 rounded-lg px-3 py-2.5 text-left text-sm font-medium text-slate-700 hover:bg-accent-soft hover:text-accent';
  return (
    <div ref={ref} className="relative">
      <button type="button" onClick={() => setOpen((o) => !o)} aria-expanded={open} aria-haspopup="menu"
        className={`flex items-center gap-2 rounded-full py-1 pl-1 pr-3 text-sm font-semibold transition ${overHero ? 'text-white hover:bg-white/10' : 'text-brand hover:bg-accent-soft'}`}>
        <Avatar name={name} size={32} className="!bg-accent !text-white" />
        <span className="max-w-[8rem] truncate">{name}</span>
        <ChevronDown size={15} className={`transition-transform ${open ? 'rotate-180' : ''}`} />
      </button>
      {open && (
        <div role="menu" className="absolute right-0 top-full z-50 mt-2 w-56 c-fade-in rounded-2xl border border-slate-100 bg-white p-2 shadow-lift">
          <Link role="menuitem" to={CUSTOMER_ROUTES.ACCOUNT} className={itemClass} onClick={() => setOpen(false)}><UserRound size={16} /> Tài khoản</Link>
          <Link role="menuitem" to={CUSTOMER_ROUTES.MY_BOOKINGS} className={itemClass} onClick={() => setOpen(false)}><CalendarCheck size={16} /> Đặt phòng của tôi</Link>
          <button role="menuitem" type="button" className={`${itemClass} !text-danger hover:!bg-red-50`} onClick={() => { setOpen(false); logout(); navigate(CUSTOMER_ROUTES.HOME); }}><LogOut size={16} /> Đăng xuất</button>
        </div>
      )}
    </div>
  );
}

export default function SiteHeader() {
  const { pathname } = useLocation();
  const { isAuthenticated, logout } = useCustomerAuth();
  const navigate = useNavigate();
  const [scrolled, setScrolled] = useState(false);
  const [menuOpen, setMenuOpen] = useState(false);

  useEffect(() => {
    const onScroll = () => setScrolled(window.scrollY > 24);
    onScroll();
    window.addEventListener('scroll', onScroll, { passive: true });
    return () => window.removeEventListener('scroll', onScroll);
  }, []);

  useEffect(() => setMenuOpen(false), [pathname]);

  const overHero = pathname === CUSTOMER_ROUTES.HOME && !scrolled && !menuOpen;

  const linkClass = ({ isActive }) =>
    `rounded-lg px-3 py-2 text-sm font-medium transition-colors ${
      overHero
        ? isActive ? 'text-white' : 'text-sky-100 hover:text-white'
        : isActive ? 'text-accent' : 'text-slate-600 hover:text-accent'
    }`;
  const hashClass = `rounded-lg px-3 py-2 text-sm font-medium transition-colors ${
    overHero ? 'text-sky-100 hover:text-white' : 'text-slate-600 hover:text-accent'
  }`;

  const renderLink = (item, mobile = false) => {
    if (isHashLink(item.to)) {
      return (
        <Link key={item.label} to={item.to} className={mobile ? 'block rounded-lg px-3 py-3 text-base font-medium text-slate-700 hover:bg-accent-soft' : hashClass}>
          {item.label}
        </Link>
      );
    }
    return (
      <NavLink
        key={item.label}
        to={item.to}
        end={item.to === CUSTOMER_ROUTES.HOME}
        className={mobile ? ({ isActive }) => `block rounded-lg px-3 py-3 text-base font-medium ${isActive ? 'bg-accent-soft text-accent' : 'text-slate-700 hover:bg-accent-soft'}` : linkClass}
      >
        {item.label}
      </NavLink>
    );
  };

  return (
    <header
      className={`fixed inset-x-0 top-0 z-40 transition-all duration-300 ${
        overHero ? 'bg-transparent' : 'bg-white/95 shadow-soft backdrop-blur'
      }`}
    >
      <div className="c-container flex h-[72px] items-center justify-between gap-4">
        <BrandLogo light={overHero} />

        <nav className="hidden items-center gap-1 lg:flex" aria-label="Điều hướng chính">
          {NAV_LINKS.map((item) => renderLink(item))}
        </nav>

        <div className="hidden items-center gap-2 lg:flex">
          {isAuthenticated ? (
            <AccountMenu overHero={overHero} />
          ) : (
            <>
              <Link to={CUSTOMER_ROUTES.LOGIN} className={overHero ? 'c-btn-glass c-btn-sm !border-transparent !bg-transparent' : 'c-btn-ghost c-btn-sm'}>
                Đăng nhập
              </Link>
              <Link to={CUSTOMER_ROUTES.REGISTER} className={overHero ? 'c-btn-glass c-btn-sm' : 'c-btn-outline c-btn-sm'}>
                Đăng ký
              </Link>
            </>
          )}
          <Link to={CUSTOMER_ROUTES.ROOMS} className="c-btn-primary c-btn-sm">
            Đặt phòng ngay
          </Link>
        </div>

        <button
          type="button"
          onClick={() => setMenuOpen((o) => !o)}
          className={`flex h-10 w-10 items-center justify-center rounded-xl lg:hidden ${overHero ? 'text-white hover:bg-white/10' : 'text-brand hover:bg-accent-soft'}`}
          aria-label={menuOpen ? 'Đóng menu' : 'Mở menu'}
          aria-expanded={menuOpen}
          aria-controls="mobile-menu"
        >
          {menuOpen ? <X size={22} /> : <Menu size={22} />}
        </button>
      </div>

      {menuOpen && (
        <div id="mobile-menu" className="c-fade-in border-t border-slate-100 bg-white lg:hidden">
          <nav className="c-container space-y-1 py-3" aria-label="Điều hướng di động">
            {NAV_LINKS.map((item) => renderLink(item, true))}
            <div className="grid grid-cols-2 gap-2 pt-3">
              {isAuthenticated ? (
                <>
                  <Link to={CUSTOMER_ROUTES.ACCOUNT} className="c-btn-outline">Tài khoản</Link>
                  <Link to={CUSTOMER_ROUTES.MY_BOOKINGS} className="c-btn-outline">Đặt phòng của tôi</Link>
                  <button type="button" className="c-btn-outline col-span-2 !text-danger" onClick={() => { logout(); navigate(CUSTOMER_ROUTES.HOME); }}>Đăng xuất</button>
                </>
              ) : (
                <>
                  <Link to={CUSTOMER_ROUTES.LOGIN} className="c-btn-outline">Đăng nhập</Link>
                  <Link to={CUSTOMER_ROUTES.REGISTER} className="c-btn-outline">Đăng ký</Link>
                </>
              )}
              <Link to={CUSTOMER_ROUTES.ROOMS} className="c-btn-primary col-span-2">
                Đặt phòng ngay
              </Link>
            </div>
          </nav>
        </div>
      )}
    </header>
  );
}
