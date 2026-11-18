import { createMockService } from './createMockService.js';
import { seedCustomers } from './mockData.js';
import bookingService from './bookingService.js';
import { ACTIVE_BOOKING_STATUSES } from '../utils/constants.js';

const base = createMockService({
  prefix: 'KH',
  pad: 3,
  initial: seedCustomers,
  unique: [{ field: 'idNumber', message: 'CCCD/Hộ chiếu đã tồn tại trong hệ thống' }],
  sortBy: (a, b) => a.id.localeCompare(b.id),
});

const customerService = {
  ...base,
  async remove(id) {
    const hasActive = bookingService.internal
      .get()
      .some((b) => b.customerId === id && ACTIVE_BOOKING_STATUSES.includes(b.status));
    if (hasActive) throw new Error('Khách hàng đang có đặt phòng chưa hoàn tất, không thể xóa');
    return base.remove(id);
  },
};

export default customerService;
