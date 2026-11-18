// Đường dẫn của trang quản trị: tất cả nằm dưới /admin.
import { ADMIN_BASE } from '../../shared/appModes.js';

export { ADMIN_BASE };
const admin = (path) => `${ADMIN_BASE}${path}`;

export const ROUTES = {
  LOGIN: admin('/login'),
  DASHBOARD: admin('/dashboard'),
  ROOMS: admin('/rooms'),
  BOOKINGS: admin('/bookings'),
  CUSTOMERS: admin('/customers'),
  CHECK_IN_OUT: admin('/check-in-out'),
  SERVICES: admin('/services'),
  INVOICES: admin('/invoices'),
  REPORTS: admin('/reports'),
  EMPLOYEES: admin('/employees'),
  SETTINGS: admin('/settings'),
};
