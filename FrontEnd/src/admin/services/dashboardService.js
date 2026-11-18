import { delay } from '../../shared/services/api.js';
import { addDays, todayStr } from '../../shared/utils/dateUtils.js';
import { ROOM_STATUS } from '../../shared/utils/constants.js';
import roomService from '../../shared/services/roomService.js';
import bookingService from '../../shared/services/bookingService.js';
import activityService from './activityService.js';
import { dailyOccupancy, revenueOfDay, revenueOfMonth } from './reportService.js';

const dashboardService = {
  async getOverview() {
    await delay(450);
    const today = todayStr();
    const year = Number(today.slice(0, 4));
    const month = Number(today.slice(5, 7));

    const rooms = roomService.internal.get();
    const bookings = bookingService.internal.get();
    const count = (status) => rooms.filter((r) => r.status === status).length;

    const totalRooms = rooms.length;
    const occupied = count('Occupied');
    const occupancyRate = totalRooms ? Math.round((occupied / totalRooms) * 100) : 0;
    const staying = bookings.filter((b) => b.status === 'Checked-in');

    const stats = {
      totalRooms,
      occupied,
      available: count('Available'),
      maintenance: count('Maintenance'),
      guestsStaying: staying.reduce((s, b) => s + (b.guests || 0), 0),
      revenueToday: revenueOfDay(today).total,
      revenueMonth: revenueOfMonth(year, month).total,
      occupancyRate,
    };

    const limit = addDays(today, 2);
    const upcomingCheckIn = bookings
      .filter((b) => ['Confirmed', 'Pending'].includes(b.status) && b.checkIn >= today && b.checkIn <= limit)
      .sort((a, b) => a.checkIn.localeCompare(b.checkIn))
      .slice(0, 6);
    const upcomingCheckOut = staying
      .filter((b) => b.checkOut <= limit)
      .sort((a, b) => a.checkOut.localeCompare(b.checkOut))
      .slice(0, 6);
    const recentBookings = [...bookings]
      .sort((a, b) => (b.createdAt || '').localeCompare(a.createdAt || ''))
      .slice(0, 6);

    const revenueByDay = Array.from({ length: 14 }, (_, i) => {
      const date = addDays(today, i - 13);
      return { label: date.slice(8, 10) + '/' + date.slice(5, 7), total: revenueOfDay(date).total };
    });
    const revenueByMonth = Array.from({ length: 12 }, (_, i) => ({
      label: `T${i + 1}`,
      total: revenueOfMonth(year, i + 1).total,
    }));
    const occupancySeries = Array.from({ length: 7 }, (_, i) => {
      const date = addDays(today, i - 6);
      return {
        label: date.slice(8, 10) + '/' + date.slice(5, 7),
        rate: i === 6 ? occupancyRate : dailyOccupancy(date),
      };
    });
    const roomStatus = Object.entries(ROOM_STATUS).map(([status, meta]) => ({
      status,
      name: meta.label,
      value: count(status),
      color: meta.hex,
    }));

    const activities = await activityService.getRecent(8);

    return {
      stats,
      upcomingCheckIn,
      upcomingCheckOut,
      recentBookings,
      revenueByDay,
      revenueByMonth,
      occupancySeries,
      roomStatus,
      activities,
    };
  },
};

export default dashboardService;
