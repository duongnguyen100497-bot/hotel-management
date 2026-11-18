import { Suspense, useEffect } from 'react';
import { Outlet, useLocation } from 'react-router-dom';
import SiteHeader from '../components/SiteHeader.jsx';
import { CustomerAuthProvider } from '../contexts/CustomerAuthContext.jsx';
import SiteFooter from '../components/SiteFooter.jsx';
import { LoadingBlock } from '../components/States.jsx';
import '../styles/customer.css';

// Cuộn tới phần tử theo #hash. Trang có thể đang lazy-load nên thử lại trong tối đa ~1 giây.
function scrollToHash(hash) {
  const id = decodeURIComponent(hash.slice(1));
  let tries = 0;
  const attempt = () => {
    const el = document.getElementById(id);
    if (el) el.scrollIntoView({ behavior: 'smooth', block: 'start' });
    else if (tries < 20) {
      tries += 1;
      timer = setTimeout(attempt, 50);
    }
  };
  let timer = setTimeout(attempt, 0);
  return () => clearTimeout(timer);
}

// Layout độc lập cho website khách hàng: Header + nội dung + Footer, không có Sidebar của Admin.
export default function CustomerLayout() {
  const { pathname, hash } = useLocation();

  useEffect(() => {
    if (hash) return scrollToHash(hash);
    window.scrollTo({ top: 0 });
    return undefined;
  }, [pathname, hash]);

  return (
    <CustomerAuthProvider>
    <div className="customer-site flex min-h-screen flex-col bg-white font-sans text-ink">
      <a
        href="#main-content"
        className="sr-only z-50 rounded-lg bg-white px-4 py-2 text-sm font-semibold text-brand focus:not-sr-only focus:fixed focus:left-4 focus:top-4"
      >
        Bỏ qua điều hướng
      </a>
      <SiteHeader />
      {/* Header cố định: trang chủ để hero chạy dưới header, các trang khác chừa chỗ cho header */}
      <main id="main-content" className={`flex-1 ${pathname === '/' ? '' : 'pt-[72px]'}`}>
        <Suspense fallback={<LoadingBlock />}>
          <Outlet />
        </Suspense>
      </main>
      <SiteFooter />
    </div>
    </CustomerAuthProvider>
  );
}
