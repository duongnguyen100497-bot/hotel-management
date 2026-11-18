import { BOOKING_POLICY } from '../data/bookingPolicy.js';

// Tính giá đặt phòng. Mọi số tiền làm tròn đến đồng.
// promoPercent (0-100): giảm giá theo loại phòng, mặc định không có.
export function unitPrice(price, promoPercent = 0) {
  const p = Number(promoPercent) || 0;
  return p > 0 ? Math.round(price * (1 - p / 100)) : price;
}

export function calcPricing({ price, nights, rooms = 1, promoPercent = 0 }) {
  const n = Math.max(0, Number(nights) || 0);
  const r = Math.max(1, Number(rooms) || 1);
  const perNight = unitPrice(price, promoPercent);
  const roomCharge = perNight * n * r;
  const serviceFee = Math.round(roomCharge * BOOKING_POLICY.serviceFeeRate);
  const vat = Math.round(roomCharge * BOOKING_POLICY.vatRate);
  return { perNight, nights: n, rooms: r, roomCharge, serviceFee, vat, total: roomCharge + serviceFee + vat };
}
