import { lazy, Suspense } from 'react';
import { useLocation } from 'react-router-dom';
import { isAdminPath } from './shared/appModes.js';
import AppLoading from './shared/AppLoading.jsx';

// Hai chế độ của dự án, tải riêng biệt: chỉ code của chế độ đang dùng được tải về.
const AdminApp = lazy(() => import('./admin/AdminApp.jsx'));
const CustomerApp = lazy(() => import('./customer/CustomerApp.jsx'));

export default function App() {
  const { pathname } = useLocation();
  return <Suspense fallback={<AppLoading />}>{isAdminPath(pathname) ? <AdminApp /> : <CustomerApp />}</Suspense>;
}
