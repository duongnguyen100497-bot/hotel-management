import { BOOKING_POLICY } from '../data/bookingPolicy.js';

// Quy tắc nghiệp vụ phía khách hàng cho một booking.
export const CUSTOMER_STATUS = {
  Pending: { label: 'Chờ xác nhận', tone: 'warning' },
  Confirmed: { label: 'Đã xác nhận', tone: 'accent' },
  'Checked-in': { label: 'Đang lưu trú', tone: 'success' },
  'Checked-out': { label: 'Hoàn thành', tone: 'muted' },
  Cancelled: { label: 'Đã hủy', tone: 'danger' },
};

export const STATUS_TABS = [
  { key: 'all', label: 'Tất cả', match: () => true },
  { key: 'upcoming', label: 'Sắp tới', match: (b) => b.status === 'Pending' || b.status === 'Confirmed' },
  { key: 'staying', label: 'Đang lưu trú', match: (b) => b.status === 'Checked-in' },
  { key: 'done', label: 'Hoàn thành', match: (b) => b.status === 'Checked-out' },
  { key: 'cancelled', label: 'Đã hủy', match: (b) => b.status === 'Cancelled' },
];

// Mốc nhận phòng: ngày nhận + giờ nhận phòng của khách sạn (vd "14:00").
export function checkInMoment(booking, checkInTime = '14:00') {
  return new Date(`${booking.checkIn}T${/^\d{2}:\d{2}$/.test(checkInTime) ? checkInTime : '14:00'}:00`);
}

// Được hủy trực tuyến khi còn Chờ xác nhận / Đã xác nhận và cách giờ nhận phòng ít nhất freeCancelHours.
export function cancelEligibility(booking, checkInTime, now = new Date()) {
  if (booking.status !== 'Pending' && booking.status !== 'Confirmed') {
    return { ok: false, reason: booking.status === 'Cancelled' ? 'Đặt phòng này đã được hủy.' : 'Chỉ hủy được đặt phòng chưa nhận phòng.' };
  }
  const hoursLeft = (checkInMoment(booking, checkInTime) - now) / 3600000;
  if (hoursLeft < BOOKING_POLICY.freeCancelHours) {
    return { ok: false, reason: `Đã quá thời hạn hủy trực tuyến (${BOOKING_POLICY.freeCancelHours} giờ trước giờ nhận phòng). Vui lòng liên hệ khách sạn.` };
  }
  return { ok: true, reason: '' };
}

// Số tiền khách phải trả cho booking: ưu tiên tổng đã tính lúc đặt (có phí + VAT), nếu không có thì dùng tiền phòng.
export const bookingGrandTotal = (b) => b.grandTotal ?? b.total ?? 0;
