import { createMockService } from './createMockService.js';
import { seedRooms } from './mockData.js';
import bookingService from './bookingService.js';
import { ACTIVE_BOOKING_STATUSES } from '../utils/constants.js';

const base = createMockService({
  initial: seedRooms,
  idFrom: (p) => p.code,
  unique: [{ field: 'code', message: 'Mã phòng đã tồn tại' }],
  sortBy: (a, b) => a.code.localeCompare(b.code),
});

const roomService = {
  ...base,

  create(payload) {
    return base.create({ ...payload, code: payload.code.trim().toUpperCase() });
  },

  async remove(id) {
    const room = base.internal.get().find((r) => r.id === id);
    if (room?.status === 'Occupied') throw new Error('Không thể xóa phòng đang có khách lưu trú');
    const hasActive = bookingService.internal
      .get()
      .some((b) => b.roomCode === id && ACTIVE_BOOKING_STATUSES.includes(b.status));
    if (hasActive) throw new Error('Phòng đang có đặt phòng chưa hoàn tất, không thể xóa');
    return base.remove(id);
  },

  updateStatus(id, status) {
    return base.update(id, { status });
  },

  // Cập nhật đồng bộ cho luồng check-in / check-out.
  setStatusSync(id, status) {
    base.internal.set(base.internal.get().map((r) => (r.id === id ? { ...r, status } : r)));
  },
};

export default roomService;
