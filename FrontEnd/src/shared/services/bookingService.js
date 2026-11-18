import { createMockService } from './createMockService.js';
import { seedBookings } from './mockData.js';
import { ACTIVE_BOOKING_STATUSES } from '../utils/constants.js';

const base = createMockService({
  prefix: 'BK',
  pad: 4,
  initial: seedBookings,
  sortBy: (a, b) => (b.createdAt || '').localeCompare(a.createdAt || ''),
  validate(b, store, selfId) {
    if (b.checkOut <= b.checkIn) throw new Error('Ngày trả phòng phải sau ngày nhận phòng');
    if (!ACTIVE_BOOKING_STATUSES.includes(b.status)) return;
    const conflict = store.find(
      (o) =>
        o.id !== selfId &&
        o.roomCode === b.roomCode &&
        ACTIVE_BOOKING_STATUSES.includes(o.status) &&
        o.checkIn < b.checkOut &&
        b.checkIn < o.checkOut,
    );
    if (conflict) {
      throw new Error(`Phòng ${b.roomCode} đã có đặt phòng ${conflict.id} trong khoảng thời gian này`);
    }
  },
});

const bookingService = {
  ...base,
  async getByCustomer(customerId) {
    const all = await base.getAll();
    return all.filter((b) => b.customerId === customerId);
  },
  async getByRoom(roomCode) {
    const all = await base.getAll();
    return all.filter((b) => b.roomCode === roomCode);
  },
  cancel(id) {
    return base.update(id, { status: 'Cancelled' });
  },
};

export default bookingService;
