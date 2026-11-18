import bookingService from '../../shared/services/bookingService.js';
import customerRoomService from './customerRoomService.js';
import { persistOnlineData } from '../../shared/services/onlineDataStore.js';
import { delay } from '../../shared/services/api.js';
import { countNights, totalGuests, validateSearch } from '../utils/bookingSearch.js';
import { calcPricing } from '../utils/pricing.js';
import { cancelEligibility } from '../utils/bookingRules.js';
import { firstError, isEmail, isPhone, minLength, required } from '../../shared/utils/validators.js';

export const PAYMENT_METHODS = {
  hotel: { label: 'Thanh toán tại khách sạn', available: true },
  transfer: { label: 'Chuyển khoản ngân hàng', available: true },
  card: { label: 'Thẻ thanh toán trực tuyến', available: false },
};

export const SPECIAL_REQUEST_MAX = 500;

export function validateGuest(v) {
  const errors = {};
  const set = (k, msg) => msg && (errors[k] = msg);
  set('fullName', firstError(required(v.fullName, 'Họ và tên'), minLength(v.fullName, 2, 'Họ và tên')));
  set('email', firstError(required(v.email, 'Email'), isEmail(v.email)));
  set('phone', firstError(required(v.phone, 'Số điện thoại'), isPhone(v.phone)));
  set('nationality', required(v.nationality, 'Quốc tịch'));
  if ((v.request || '').length > SPECIAL_REQUEST_MAX) errors.request = `Tối đa ${SPECIAL_REQUEST_MAX} ký tự`;
  return errors;
}

const customerBookingService = {
  // Tạo đặt phòng trực tuyến. Mỗi phòng là một booking riêng (Pending) để Admin xử lý như booking thường.
  // Chống đặt trùng: kiểm tra lại phòng trống ngay lúc gửi; bookingService.create tiếp tục chặn trùng lịch
  // theo phòng. Nếu không đủ phòng thì hoàn tác các phòng đã giữ trước đó.
  async createBooking({ customer, guest, roomType, search, paymentMethod }) {
    await delay(400);
    const searchErrors = validateSearch(search);
    if (Object.keys(searchErrors).length) throw new Error(Object.values(searchErrors)[0]);
    const guestErrors = validateGuest(guest);
    if (Object.keys(guestErrors).length) throw Object.assign(new Error('Thông tin khách hàng chưa hợp lệ'), { fields: guestErrors });
    const method = PAYMENT_METHODS[paymentMethod];
    if (!method || !method.available) throw new Error('Phương thức thanh toán này hiện chưa khả dụng');

    const wanted = Number(search.rooms);
    if (totalGuests(search) > roomType.capacity * wanted) {
      throw new Error(`${roomType.name} chứa tối đa ${roomType.capacity} khách mỗi phòng. Hãy tăng số phòng hoặc chọn hạng khác.`);
    }

    const nights = countNights(search.checkIn, search.checkOut);
    const pricing = calcPricing({ price: roomType.price, nights, rooms: wanted, promoPercent: roomType.promoPercent });
    const perRoom = {
      serviceFee: Math.round(pricing.serviceFee / wanted),
      vat: Math.round(pricing.vat / wanted),
    };
    const groupId = `GRP${Date.now().toString(36).toUpperCase()}`;
    const created = [];
    const tried = new Set();

    try {
      for (let i = 0; i < wanted; i += 1) {
        let placed = false;
        // Thử lần lượt các phòng còn trống (loại trừ phòng đã thử) đến khi giữ được một phòng.
        // eslint-disable-next-line no-await-in-loop
        while (!placed) {
          // eslint-disable-next-line no-await-in-loop
          const free = (await customerRoomService.pickRooms(roomType.type, search.checkIn, search.checkOut, 1, [...tried]))[0];
          if (!free) throw new Error('Rất tiếc, hạng phòng này vừa hết phòng cho ngày bạn chọn. Vui lòng chọn ngày hoặc hạng phòng khác.');
          tried.add(free.code);
          const guestsHere = Math.ceil(totalGuests(search) / wanted);
          try {
            // eslint-disable-next-line no-await-in-loop
            const booking = await bookingService.create({
              customerId: customer.id,
              customerName: guest.fullName.trim(),
              phone: guest.phone.replace(/[\s.-]/g, ''),
              roomType: roomType.type,
              roomCode: free.code,
              checkIn: search.checkIn,
              checkOut: search.checkOut,
              guests: guestsHere,
              total: pricing.perNight * nights, // tiền phòng, cùng cách tính với Admin
              status: 'Pending',
              note: `[Đặt trực tuyến] ${(guest.request || '').trim()}`.trim(),
              // Trường bổ sung của đặt phòng trực tuyến (Admin bỏ qua nếu không dùng)
              source: 'online',
              groupId,
              contactEmail: guest.email.trim().toLowerCase(),
              nationality: guest.nationality,
              adults: Number(search.adults),
              children: Number(search.children),
              serviceFee: perRoom.serviceFee,
              vat: perRoom.vat,
              grandTotal: pricing.perNight * nights + perRoom.serviceFee + perRoom.vat,
              paymentMethod: method.label,
              paymentStatus: 'Chưa thanh toán',
            });
            created.push(booking);
            placed = true;
          } catch (e) {
            // Phòng vừa bị người khác giữ: thử phòng khác. Lỗi khác thì dừng.
            if (!/đã có đặt phòng/.test(e.message)) throw e;
          }
        }
      }
    } catch (e) {
      // Hoàn tác các phòng đã giữ để không để lại booking dở dang.
      // eslint-disable-next-line no-await-in-loop
      for (const b of created) await bookingService.remove(b.id).catch(() => {});
      throw e;
    }

    persistOnlineData();
    return { groupId, bookings: created, pricing };
  },

  async listMine(customerId) {
    const all = await bookingService.getByCustomer(customerId);
    return all.sort((a, b) => (b.createdAt || '').localeCompare(a.createdAt || '') || b.id.localeCompare(a.id));
  },

  // Khách hủy đặt phòng của chính mình, theo chính sách hủy.
  async cancelMine(customerId, bookingId, checkInTime) {
    const mine = await bookingService.getByCustomer(customerId);
    const booking = mine.find((b) => b.id === bookingId);
    if (!booking) throw new Error('Không tìm thấy đặt phòng của bạn');
    const rule = cancelEligibility(booking, checkInTime);
    if (!rule.ok) throw new Error(rule.reason);
    const updated = await bookingService.cancel(bookingId);
    persistOnlineData();
    return updated;
  },
};

export default customerBookingService;
