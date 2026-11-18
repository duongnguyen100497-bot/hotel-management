// Dự án có hai chế độ chạy độc lập trong cùng một ứng dụng:
//  - Website khách hàng (src/customer): các URL công khai /, /rooms, /login ...
//  - Trang quản trị (src/admin): mọi URL nằm dưới /admin
// Hai chế độ không import code của nhau; chỉ dùng chung dữ liệu và tiện ích trong src/shared.
export const ADMIN_BASE = '/admin';

export const isAdminPath = (pathname = '') => pathname === ADMIN_BASE || pathname.startsWith(`${ADMIN_BASE}/`);
