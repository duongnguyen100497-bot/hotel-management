import { Link, Navigate, Outlet, useLocation } from 'react-router-dom';
import { ShieldAlert } from 'lucide-react';
import { useAuth } from '../contexts/AuthContext.jsx';
import { ROUTES } from './paths.js';
import { MODULES } from '../utils/permissions.js';

// Yêu cầu đăng nhập.
export function RequireAuth() {
  const { isAuthenticated } = useAuth();
  const location = useLocation();
  if (!isAuthenticated) return <Navigate to={ROUTES.LOGIN} replace state={{ from: location.pathname }} />;
  return <Outlet />;
}

// Yêu cầu quyền truy cập module theo vai trò (phân quyền giao diện).
export function RequirePermission({ module, children }) {
  const { hasAccess } = useAuth();
  if (hasAccess(module)) return children;

  const fallback = MODULES.find((m) => hasAccess(m.key));
  return (
    <div className="card mx-auto mt-10 max-w-md p-8 text-center">
      <span className="mx-auto flex h-14 w-14 items-center justify-center rounded-full bg-red-50 text-red-500">
        <ShieldAlert size={26} />
      </span>
      <h1 className="mt-4 text-lg font-semibold">Không có quyền truy cập</h1>
      <p className="mt-1 text-sm text-slate-500">Vai trò của bạn chưa được cấp quyền sử dụng chức năng này.</p>
      {fallback && (
        <Link to={fallback.path} className="btn-primary mt-5">
          Về trang {fallback.label}
        </Link>
      )}
    </div>
  );
}
