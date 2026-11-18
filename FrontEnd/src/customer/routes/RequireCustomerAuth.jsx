import { Navigate, Outlet, useLocation } from 'react-router-dom';
import { useCustomerAuth } from '../contexts/CustomerAuthContext.jsx';
import { CUSTOMER_ROUTES } from './paths.js';
import { isAdminPath } from '../../shared/appModes.js';

// Chặn khách chưa đăng nhập khỏi các trang tài khoản / đặt phòng. Chuyển sang đăng nhập và quay lại sau đó.
export default function RequireCustomerAuth() {
  const { isAuthenticated } = useCustomerAuth();
  const location = useLocation();
  if (!isAuthenticated) {
    return <Navigate to={CUSTOMER_ROUTES.LOGIN} replace state={{ from: `${location.pathname}${location.search}` }} />;
  }
  return <Outlet />;
}

// Chỉ cho phép quay lại đường dẫn nội bộ của website khách hàng (không cho /admin hay URL ngoài).
export function safeReturnPath(from) {
  if (typeof from !== 'string' || !from.startsWith('/') || from.startsWith('//')) return CUSTOMER_ROUTES.ACCOUNT;
  if (isAdminPath(from)) return CUSTOMER_ROUTES.ACCOUNT;
  if (from === CUSTOMER_ROUTES.LOGIN || from === CUSTOMER_ROUTES.REGISTER) return CUSTOMER_ROUTES.ACCOUNT;
  return from;
}
