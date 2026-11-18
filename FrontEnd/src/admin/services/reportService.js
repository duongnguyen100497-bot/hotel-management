import { delay } from '../../shared/services/api.js';
import { hashString, seededRandom } from '../utils/seededRandom.js';
import { daysInMonth, makeDate } from '../../shared/utils/dateUtils.js';
import roomService from '../../shared/services/roomService.js';
import bookingService from '../../shared/services/bookingService.js';
import customerService from '../../shared/services/customerService.js';
import invoiceService from '../../shared/services/invoiceService.js';
import { toISODate } from '../../shared/utils/dateUtils.js';
import serviceCatalogService from './serviceCatalogService.js';

/* ---------- Bộ sinh số liệu mock ổn định theo ngày ---------- */
const isWeekend = (dateStr) => {
  const dow = new Date(`${dateStr}T00:00:00`).getDay();
  return dow === 5 || dow === 6;
};

export function dailyRevenue(dateStr) {
  const rng = seededRandom(hashString(`rev${dateStr}`));
  const million = 30 + rng() * 40 + (isWeekend(dateStr) ? 18 : 0);
  const total = Math.round(million * 10) * 100000;
  const room = Math.round((total * (0.78 + rng() * 0.1)) / 100000) * 100000;
  return { total, room, service: total - room };
}

export function dailyOccupancy(dateStr) {
  const rng = seededRandom(hashString(`occ${dateStr}`));
  return Math.min(98, Math.round(52 + rng() * 30 + (isWeekend(dateStr) ? 10 : 0)));
}

// Hóa đơn đã thanh toán thật được cộng vào số liệu mô phỏng để Dashboard / báo cáo phản ứng khi thu tiền.
const actualRevenue = (prefix) =>
  invoiceService.internal
    .get()
    .filter((i) => i.paymentStatus === 'Đã thanh toán' && toISODate(i.paidAt || i.createdAt).startsWith(prefix))
    .reduce(
      (acc, i) => ({
        total: acc.total + Number(i.roomCharge || 0) + Number(i.serviceCharge || 0) + Number(i.surcharge || 0),
        room: acc.room + Number(i.roomCharge || 0),
        service: acc.service + Number(i.serviceCharge || 0) + Number(i.surcharge || 0),
      }),
      { total: 0, room: 0, service: 0 },
    );

const withActual = (mock, prefix) => {
  const a = actualRevenue(prefix);
  return { total: mock.total + a.total, room: mock.room + a.room, service: mock.service + a.service };
};

const sumRevenue = (items) =>
  items.reduce(
    (acc, v) => ({ total: acc.total + v.total, room: acc.room + v.room, service: acc.service + v.service }),
    { total: 0, room: 0, service: 0 },
  );

export function monthRevenue(year, month) {
  const days = Array.from({ length: daysInMonth(year, month) }, (_, i) =>
    dailyRevenue(makeDate(year, month, i + 1)),
  );
  return sumRevenue(days);
}

export const revenueOfDay = (dateStr) => withActual(dailyRevenue(dateStr), dateStr);
export const revenueOfMonth = (year, month) =>
  withActual(monthRevenue(year, month), `${year}-${String(month).padStart(2, '0')}`);

export function monthOccupancy(year, month) {
  const n = daysInMonth(year, month);
  let sum = 0;
  for (let i = 1; i <= n; i += 1) sum += dailyOccupancy(makeDate(year, month, i));
  return Math.round(sum / n);
}

/* ---------- Trục thời gian theo bộ lọc ---------- */
// mode: 'day' (các ngày trong tháng) | 'month' (12 tháng trong năm) | 'year' (5 năm gần nhất)
const periods = ({ mode, year, month }) => {
  if (mode === 'day') {
    return Array.from({ length: daysInMonth(year, month) }, (_, i) => ({
      label: String(i + 1),
      key: makeDate(year, month, i + 1),
      year,
      month,
      day: i + 1,
    }));
  }
  if (mode === 'month') {
    return Array.from({ length: 12 }, (_, i) => ({ label: `T${i + 1}`, key: `${year}-${i + 1}`, year, month: i + 1 }));
  }
  return Array.from({ length: 5 }, (_, i) => ({ label: String(year - 4 + i), key: `${year - 4 + i}`, year: year - 4 + i }));
};

const revenueOf = (p, mode) => {
  if (mode === 'day') return revenueOfDay(p.key);
  if (mode === 'month') return revenueOfMonth(p.year, p.month);
  return sumRevenue(Array.from({ length: 12 }, (_, i) => revenueOfMonth(p.year, i + 1)));
};

const occupancyOf = (p, mode) => {
  if (mode === 'day') return dailyOccupancy(p.key);
  if (mode === 'month') return monthOccupancy(p.year, p.month);
  return Math.round(
    Array.from({ length: 12 }, (_, i) => monthOccupancy(p.year, i + 1)).reduce((a, b) => a + b, 0) / 12,
  );
};

const scaleFor = (mode) => (mode === 'day' ? 1 : mode === 'month' ? 12 : 150);

/* ---------- API báo cáo ---------- */
const reportService = {
  async getRevenueReport(filter) {
    await delay(350);
    const series = periods(filter).map((p) => ({ label: p.label, ...revenueOf(p, filter.mode) }));
    const total = series.reduce((s, x) => s + x.total, 0);
    const room = series.reduce((s, x) => s + x.room, 0);
    const best = series.reduce((a, b) => (b.total > a.total ? b : a), series[0]);
    return {
      series,
      summary: { total, room, service: total - room, average: Math.round(total / series.length), best },
    };
  },

  async getOccupancyReport(filter) {
    await delay(350);
    const totalRooms = roomService.internal.get().length || 1;
    const series = periods(filter).map((p) => {
      const rate = occupancyOf(p, filter.mode);
      return { label: p.label, rate, sold: Math.round((rate / 100) * totalRooms) };
    });
    const rates = series.map((s) => s.rate);
    return {
      series,
      totalRooms,
      summary: {
        average: Math.round(rates.reduce((a, b) => a + b, 0) / rates.length),
        max: Math.max(...rates),
        min: Math.min(...rates),
        soldTotal: series.reduce((s, x) => s + x.sold, 0),
      },
    };
  },

  async getCustomerReport(filter) {
    await delay(350);
    const customers = customerService.internal.get();
    const bookings = bookingService.internal.get().filter((b) => b.status !== 'Cancelled');

    const nationalityMap = {};
    customers.forEach((c) => {
      nationalityMap[c.nationality] = (nationalityMap[c.nationality] || 0) + 1;
    });
    const nationalities = Object.entries(nationalityMap)
      .map(([name, value]) => ({ name, value }))
      .sort((a, b) => b.value - a.value);

    const spend = {};
    const stays = {};
    bookings.forEach((b) => {
      spend[b.customerId] = (spend[b.customerId] || 0) + b.total;
      stays[b.customerId] = (stays[b.customerId] || 0) + 1;
    });
    const topCustomers = customers
      .map((c) => ({ ...c, spent: spend[c.id] || 0, stays: stays[c.id] || 0 }))
      .sort((a, b) => b.spent - a.spent)
      .slice(0, 6);

    const scale = scaleFor(filter.mode);
    const series = periods(filter).map((p) => {
      const rng = seededRandom(hashString(`cus${p.key}`));
      return {
        label: p.label,
        newGuests: Math.round((2 + rng() * 5) * scale),
        returning: Math.round((1 + rng() * 3) * scale),
      };
    });

    return {
      series,
      nationalities,
      topCustomers,
      summary: {
        totalCustomers: customers.length,
        foreign: customers.filter((c) => c.nationality !== 'Việt Nam').length,
        returning: Object.values(stays).filter((n) => n >= 2).length,
        newInPeriod: series.reduce((s, x) => s + x.newGuests, 0),
      },
    };
  },

  async getServiceReport(filter) {
    await delay(350);
    const services = serviceCatalogService.internal.get();
    const scale = scaleFor(filter.mode);
    const rows = services
      .map((s) => {
        const rng = seededRandom(hashString(`svc${s.id}${filter.mode}${filter.year}${filter.month}`));
        const count = Math.round((3 + rng() * 14) * scale * (s.active ? 1 : 0.3));
        return { id: s.id, name: s.name, type: s.type, price: s.price, count, revenue: count * s.price };
      })
      .sort((a, b) => b.revenue - a.revenue);
    return {
      rows,
      summary: {
        totalRevenue: rows.reduce((s, r) => s + r.revenue, 0),
        totalCount: rows.reduce((s, r) => s + r.count, 0),
        top: rows[0],
      },
    };
  },
};

export default reportService;
