import { delay } from '../../shared/services/api.js';
import bookingService from '../../shared/services/bookingService.js';
import roomService from '../../shared/services/roomService.js';
import invoiceService from '../../shared/services/invoiceService.js';
import activityService from './activityService.js';

// Luồng nghiệp vụ nhận / trả phòng: cập nhật đồng thời đặt phòng, phòng, hóa đơn và nhật ký.
const checkInOutService = {
  async checkIn(bookingId) {
    const booking = bookingService.internal.get().find((b) => b.id === bookingId);
    if (!booking) throw new Error('Không tìm thấy đặt phòng');
    if (booking.status !== 'Confirmed') throw new Error('Chỉ có thể check-in đặt phòng đã xác nhận');

    const room = roomService.internal.get().find((r) => r.id === booking.roomCode);
    if (room && ['Occupied', 'Cleaning', 'Maintenance'].includes(room.status)) {
      throw new Error(`Phòng ${booking.roomCode} chưa sẵn sàng (${room.status === 'Occupied' ? 'đang có khách' : room.status === 'Cleaning' ? 'đang dọn dẹp' : 'đang bảo trì'})`);
    }

    const updated = await bookingService.update(bookingId, {
      status: 'Checked-in',
      checkInTime: new Date().toISOString(),
    });
    roomService.setStatusSync(booking.roomCode, 'Occupied');
    activityService.log('checkin', `Check-in phòng ${booking.roomCode} – ${booking.customerName}`);
    return updated;
  },

  // payload: { serviceItems: [{name, qty, price}], surcharge, paymentMethod }
  async checkOut(bookingId, { serviceItems = [], surcharge = 0, paymentMethod = 'Tiền mặt' } = {}) {
    const booking = bookingService.internal.get().find((b) => b.id === bookingId);
    if (!booking) throw new Error('Không tìm thấy đặt phòng');
    if (booking.status !== 'Checked-in') throw new Error('Đặt phòng này chưa check-in');

    const updated = await bookingService.update(bookingId, {
      status: 'Checked-out',
      checkOutTime: new Date().toISOString(),
    });
    roomService.setStatusSync(booking.roomCode, 'Cleaning');

    const serviceCharge = serviceItems.reduce((s, it) => s + it.qty * it.price, 0);
    const invoice = await invoiceService.create({
      bookingId,
      customerName: booking.customerName,
      roomCode: booking.roomCode,
      checkIn: booking.checkIn,
      checkOut: booking.checkOut,
      serviceItems,
      roomCharge: booking.total,
      serviceCharge,
      surcharge: Number(surcharge) || 0,
      paymentMethod,
      paymentStatus: 'Chưa thanh toán',
    });
    activityService.log('checkout', `Check-out phòng ${booking.roomCode} – ${booking.customerName}`);
    return { booking: updated, invoice };
  },

  async markRoomReady(roomCode) {
    await delay(250);
    const room = roomService.internal.get().find((r) => r.id === roomCode);
    if (!room) throw new Error('Không tìm thấy phòng');
    roomService.setStatusSync(roomCode, 'Available');
    activityService.log('room', `Phòng ${roomCode} đã dọn xong, sẵn sàng đón khách`);
    return { ...room, status: 'Available' };
  },
};

export default checkInOutService;
