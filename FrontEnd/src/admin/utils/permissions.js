import { ROUTES } from '../routes/paths.js';

export const MODULES = [
  { key: 'dashboard', label: 'Dashboard', path: ROUTES.DASHBOARD },
  { key: 'rooms', label: 'Quản lý phòng', path: ROUTES.ROOMS },
  { key: 'bookings', label: 'Đặt phòng', path: ROUTES.BOOKINGS },
  { key: 'customers', label: 'Khách hàng', path: ROUTES.CUSTOMERS },
  { key: 'checkInOut', label: 'Nhận / Trả phòng', path: ROUTES.CHECK_IN_OUT },
  { key: 'services', label: 'Dịch vụ', path: ROUTES.SERVICES },
  { key: 'invoices', label: 'Hóa đơn', path: ROUTES.INVOICES },
  { key: 'reports', label: 'Báo cáo', path: ROUTES.REPORTS },
  { key: 'employees', label: 'Nhân viên', path: ROUTES.EMPLOYEES },
  { key: 'settings', label: 'Cài đặt', path: ROUTES.SETTINGS },
];

export const ROLES = {
  admin: 'Quản trị viên',
  manager: 'Quản lý',
  receptionist: 'Lễ tân',
  accountant: 'Kế toán',
};

const all = MODULES.map((m) => m.key);

export const DEFAULT_PERMISSIONS = {
  admin: all,
  manager: all.filter((k) => k !== 'employees'),
  receptionist: ['dashboard', 'rooms', 'bookings', 'customers', 'checkInOut', 'services', 'settings'],
  accountant: ['dashboard', 'invoices', 'reports', 'settings'],
};
