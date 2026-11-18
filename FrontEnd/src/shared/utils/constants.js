// Hằng số dùng chung toàn hệ thống.

export const PAGE_SIZE = 8;

/* ---------------- Phòng ---------------- */
export const ROOM_TYPES = ['Standard', 'Superior', 'Deluxe', 'Family', 'Suite'];

export const ROOM_STATUS = {
  Available: {
    label: 'Phòng trống',
    badge: 'bg-emerald-50 text-emerald-700 ring-emerald-600/20',
    card: 'border-emerald-300 bg-emerald-50 hover:bg-emerald-100/70',
    dot: 'bg-emerald-500',
    hex: '#10b981',
  },
  Occupied: {
    label: 'Đang sử dụng',
    badge: 'bg-red-50 text-red-700 ring-red-600/20',
    card: 'border-red-300 bg-red-50 hover:bg-red-100/70',
    dot: 'bg-red-500',
    hex: '#ef4444',
  },
  Reserved: {
    label: 'Đã đặt trước',
    badge: 'bg-amber-50 text-amber-700 ring-amber-600/20',
    card: 'border-amber-300 bg-amber-50 hover:bg-amber-100/70',
    dot: 'bg-amber-500',
    hex: '#f59e0b',
  },
  Cleaning: {
    label: 'Đang dọn dẹp',
    badge: 'bg-cyan-50 text-cyan-700 ring-cyan-600/20',
    card: 'border-cyan-300 bg-cyan-50 hover:bg-cyan-100/70',
    dot: 'bg-cyan-500',
    hex: '#06b6d4',
  },
  Maintenance: {
    label: 'Đang bảo trì',
    badge: 'bg-slate-100 text-slate-600 ring-slate-500/20',
    card: 'border-slate-300 bg-slate-100 hover:bg-slate-200/70',
    dot: 'bg-slate-500',
    hex: '#64748b',
  },
};
export const ROOM_STATUS_LIST = Object.keys(ROOM_STATUS);

/* ---------------- Đặt phòng ---------------- */
export const BOOKING_STATUS = {
  Pending: { label: 'Chờ xác nhận', badge: 'bg-amber-50 text-amber-700 ring-amber-600/20' },
  Confirmed: { label: 'Đã xác nhận', badge: 'bg-sky-100 text-navy-700 ring-sky-500/20' },
  'Checked-in': { label: 'Đang lưu trú', badge: 'bg-emerald-50 text-emerald-700 ring-emerald-600/20' },
  'Checked-out': { label: 'Đã trả phòng', badge: 'bg-slate-100 text-slate-600 ring-slate-500/20' },
  Cancelled: { label: 'Đã hủy', badge: 'bg-red-50 text-red-700 ring-red-600/20' },
};
export const BOOKING_STATUS_LIST = Object.keys(BOOKING_STATUS);

// Các trạng thái còn hiệu lực (đang chiếm phòng / chưa hoàn tất).
export const ACTIVE_BOOKING_STATUSES = ['Pending', 'Confirmed', 'Checked-in'];

/* ---------------- Khách hàng ---------------- */
export const NATIONALITIES = [
  'Việt Nam',
  'Hoa Kỳ',
  'Nhật Bản',
  'Hàn Quốc',
  'Trung Quốc',
  'Anh',
  'Pháp',
  'Đức',
  'Úc',
  'Thái Lan',
  'Khác',
];

/* ---------------- Dịch vụ ---------------- */
export const SERVICE_TYPES = ['Giặt ủi', 'Nhà hàng', 'Spa', 'Minibar', 'Đưa đón sân bay', 'Khác'];

/* ---------------- Hóa đơn ---------------- */
export const PAYMENT_METHODS = ['Tiền mặt', 'Thẻ tín dụng', 'Chuyển khoản', 'Ví điện tử'];

export const PAYMENT_STATUS = {
  'Đã thanh toán': { badge: 'bg-emerald-50 text-emerald-700 ring-emerald-600/20' },
  'Chưa thanh toán': { badge: 'bg-amber-50 text-amber-700 ring-amber-600/20' },
};
export const PAYMENT_STATUS_LIST = Object.keys(PAYMENT_STATUS);

/* ---------------- Nhân viên ---------------- */
export const POSITIONS = ['Quản lý', 'Lễ tân', 'Buồng phòng', 'Kế toán', 'Bảo vệ', 'Bếp'];

export const EMPLOYEE_STATUS = {
  'Đang làm việc': { badge: 'bg-emerald-50 text-emerald-700 ring-emerald-600/20' },
  'Nghỉ phép': { badge: 'bg-amber-50 text-amber-700 ring-amber-600/20' },
  'Đã nghỉ việc': { badge: 'bg-slate-100 text-slate-600 ring-slate-500/20' },
};
export const EMPLOYEE_STATUS_LIST = Object.keys(EMPLOYEE_STATUS);
