import { lazy, Suspense } from 'react';
import { Navigate, Route, Routes } from 'react-router-dom';
import { ADMIN_BASE, ROUTES } from './paths.js';
import { RequireAuth, RequirePermission } from './ProtectedRoute.jsx';
import MainLayout from '../components/layout/MainLayout.jsx';
import { LoadingState } from '../components/common/StateViews.jsx';

const Login = lazy(() => import('../pages/Login/index.jsx'));
const Dashboard = lazy(() => import('../pages/Dashboard/index.jsx'));
const Rooms = lazy(() => import('../pages/Rooms/index.jsx'));
const Bookings = lazy(() => import('../pages/Bookings/index.jsx'));
const Customers = lazy(() => import('../pages/Customers/index.jsx'));
const CheckInOut = lazy(() => import('../pages/CheckInOut/index.jsx'));
const Services = lazy(() => import('../pages/Services/index.jsx'));
const Invoices = lazy(() => import('../pages/Invoices/index.jsx'));
const Reports = lazy(() => import('../pages/Reports/index.jsx'));
const Employees = lazy(() => import('../pages/Employees/index.jsx'));
const Settings = lazy(() => import('../pages/Settings/index.jsx'));

const guarded = (module, element) => <RequirePermission module={module}>{element}</RequirePermission>;

// Toàn bộ route của trang quản trị (/admin/*).
export default function AdminRoutes() {
  return (
    <Suspense fallback={<LoadingState />}>
      <Routes>
        <Route path={ROUTES.LOGIN} element={<Login />} />

        <Route element={<RequireAuth />}>
          <Route element={<MainLayout />}>
            <Route path={ADMIN_BASE} element={<Navigate to={ROUTES.DASHBOARD} replace />} />
            <Route path={ROUTES.DASHBOARD} element={guarded('dashboard', <Dashboard />)} />
            <Route path={ROUTES.ROOMS} element={guarded('rooms', <Rooms />)} />
            <Route path={ROUTES.BOOKINGS} element={guarded('bookings', <Bookings />)} />
            <Route path={ROUTES.CUSTOMERS} element={guarded('customers', <Customers />)} />
            <Route path={ROUTES.CHECK_IN_OUT} element={guarded('checkInOut', <CheckInOut />)} />
            <Route path={ROUTES.SERVICES} element={guarded('services', <Services />)} />
            <Route path={ROUTES.INVOICES} element={guarded('invoices', <Invoices />)} />
            <Route path={ROUTES.REPORTS} element={guarded('reports', <Reports />)} />
            <Route path={ROUTES.EMPLOYEES} element={guarded('employees', <Employees />)} />
            <Route path={ROUTES.SETTINGS} element={guarded('settings', <Settings />)} />
          </Route>
        </Route>

        <Route path="*" element={<Navigate to={ROUTES.DASHBOARD} replace />} />
      </Routes>
    </Suspense>
  );
}
