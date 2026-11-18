import customerService from './customerService.js';
import bookingService from './bookingService.js';

// Lưu dữ liệu phát sinh từ website khách hàng (khách đăng ký mới, đặt phòng trực tuyến) vào localStorage
// để không mất khi tải lại trang. Chỉ lưu các bản ghi có source === 'online'.
// Khi có Backend thật, file này không còn cần thiết vì dữ liệu nằm trên máy chủ.
// Giới hạn: chỉnh sửa của Admin lên bản ghi trực tuyến chỉ được lưu từ lần khách thao tác kế tiếp.
const KEY = 'hms_online_data_v1';

const read = () => {
  try {
    const raw = window.localStorage.getItem(KEY);
    const data = raw ? JSON.parse(raw) : {};
    return { customers: data.customers || [], bookings: data.bookings || [] };
  } catch {
    return { customers: [], bookings: [] };
  }
};

export function persistOnlineData() {
  try {
    const data = {
      customers: customerService.internal.get().filter((c) => c.source === 'online'),
      bookings: bookingService.internal.get().filter((b) => b.source === 'online'),
    };
    window.localStorage.setItem(KEY, JSON.stringify(data));
  } catch {
    /* localStorage bị chặn hoặc đầy: bỏ qua, dữ liệu vẫn còn trong phiên hiện tại */
  }
}

// Nạp lại dữ liệu đã lưu vào các service (gọi một lần khi khởi động ứng dụng).
export function hydrateOnlineData() {
  const saved = read();
  const merge = (service, records) => {
    const current = service.internal.get();
    const known = new Set(current.map((r) => r.id));
    const fresh = records.filter((r) => !known.has(r.id));
    if (fresh.length) service.internal.set([...fresh, ...current]);
  };
  merge(customerService, saved.customers);
  merge(bookingService, saved.bookings);
}

export function clearOnlineData() {
  try {
    window.localStorage.removeItem(KEY);
  } catch {
    /* ignore */
  }
}
