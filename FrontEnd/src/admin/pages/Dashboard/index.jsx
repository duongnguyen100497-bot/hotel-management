import {
  BedDouble,
  CalendarPlus,
  DoorClosed,
  DoorOpen,
  LogIn,
  LogOut,
  Percent,
  TrendingUp,
  Users,
  Wallet,
  Wrench,
} from 'lucide-react';
import PageHeader from '../../components/common/PageHeader.jsx';
import { ErrorState, LoadingState } from '../../components/common/StateViews.jsx';
import StatCard from '../../components/dashboard/StatCard.jsx';
import RevenueChart from '../../components/dashboard/RevenueChart.jsx';
import OccupancyChart from '../../components/dashboard/OccupancyChart.jsx';
import RoomStatusChart from '../../components/dashboard/RoomStatusChart.jsx';
import BookingListCard from '../../components/dashboard/BookingListCard.jsx';
import ActivityList from '../../components/dashboard/ActivityList.jsx';
import useFetch from '../../../shared/hooks/useFetch.js';
import dashboardService from '../../services/dashboardService.js';
import { formatCurrency } from '../../../shared/utils/formatters.js';
import { ROUTES } from '../../routes/paths.js';

export default function Dashboard() {
  const { data, loading, error, reload } = useFetch(dashboardService.getOverview);

  if (loading) return <LoadingState label="Đang tải tổng quan..." />;
  if (error) return <ErrorState message={error} onRetry={reload} />;

  const { stats } = data;

  return (
    <div className="space-y-6">
      <PageHeader title="Tổng quan" description="Tình hình hoạt động của khách sạn trong ngày." />

      <section className="grid grid-cols-1 gap-4 sm:grid-cols-2 xl:grid-cols-4">
        <StatCard title="Tổng số phòng" value={stats.totalRooms} icon={BedDouble} tone="navy" />
        <StatCard title="Phòng đang sử dụng" value={stats.occupied} subtitle="Có khách lưu trú" icon={DoorClosed} tone="red" />
        <StatCard title="Phòng trống" value={stats.available} subtitle="Sẵn sàng đón khách" icon={DoorOpen} tone="green" />
        <StatCard title="Phòng đang bảo trì" value={stats.maintenance} icon={Wrench} tone="slate" />
        <StatCard title="Khách đang lưu trú" value={stats.guestsStaying} subtitle="Tổng số khách" icon={Users} tone="sky" />
        <StatCard title="Doanh thu hôm nay" value={formatCurrency(stats.revenueToday)} icon={Wallet} tone="amber" />
        <StatCard title="Doanh thu tháng" value={formatCurrency(stats.revenueMonth)} icon={TrendingUp} tone="green" />
        <StatCard title="Tỷ lệ lấp đầy" value={`${stats.occupancyRate}%`} subtitle="Phòng đang sử dụng / tổng phòng" icon={Percent} tone="navy" />
      </section>

      <section className="grid grid-cols-1 gap-4 xl:grid-cols-3">
        <RevenueChart byDay={data.revenueByDay} byMonth={data.revenueByMonth} />
        <RoomStatusChart data={data.roomStatus} />
      </section>

      <section className="grid grid-cols-1 gap-4 xl:grid-cols-2">
        <OccupancyChart data={data.occupancySeries} />
        <ActivityList activities={data.activities} />
      </section>

      <section className="grid grid-cols-1 gap-4 xl:grid-cols-3">
        <BookingListCard
          title="Khách vừa đặt phòng"
          icon={CalendarPlus}
          rows={data.recentBookings}
          dateLabel="Nhận phòng"
          dateField="checkIn"
          showStatus
          linkTo={ROUTES.BOOKINGS}
        />
        <BookingListCard
          title="Khách sắp check-in"
          icon={LogIn}
          rows={data.upcomingCheckIn}
          dateLabel="Nhận phòng"
          dateField="checkIn"
          linkTo={ROUTES.CHECK_IN_OUT}
        />
        <BookingListCard
          title="Khách sắp check-out"
          icon={LogOut}
          rows={data.upcomingCheckOut}
          dateLabel="Trả phòng"
          dateField="checkOut"
          linkTo={ROUTES.CHECK_IN_OUT}
        />
      </section>
    </div>
  );
}
