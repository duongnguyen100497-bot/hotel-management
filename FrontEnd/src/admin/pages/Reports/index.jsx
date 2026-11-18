import { useMemo, useState } from 'react';
import { BedDouble, ConciergeBell, TrendingUp, Users } from 'lucide-react';
import PageHeader from '../../components/common/PageHeader.jsx';
import Tabs from '../../components/common/Tabs.jsx';
import ReportFilters from './ReportFilters.jsx';
import RevenueReport from './RevenueReport.jsx';
import OccupancyReport from './OccupancyReport.jsx';
import CustomerReport from './CustomerReport.jsx';
import ServiceReport from './ServiceReport.jsx';

const TABS = [
  { key: 'revenue', label: 'Doanh thu', icon: TrendingUp },
  { key: 'occupancy', label: 'Công suất phòng', icon: BedDouble },
  { key: 'customers', label: 'Khách hàng', icon: Users },
  { key: 'services', label: 'Dịch vụ', icon: ConciergeBell },
];

const REPORTS = {
  revenue: RevenueReport,
  occupancy: OccupancyReport,
  customers: CustomerReport,
  services: ServiceReport,
};

export default function Reports() {
  const now = new Date();
  const [tab, setTab] = useState('revenue');
  const [mode, setMode] = useState('month');
  const [year, setYear] = useState(now.getFullYear());
  const [month, setMonth] = useState(now.getMonth() + 1);

  // Giữ tham chiếu ổn định để các báo cáo chỉ tải lại khi bộ lọc thật sự đổi.
  const filter = useMemo(() => ({ mode, year, month }), [mode, year, month]);

  const handleFilterChange = (patch) => {
    if (patch.mode !== undefined) setMode(patch.mode);
    if (patch.year !== undefined) setYear(patch.year);
    if (patch.month !== undefined) setMonth(patch.month);
  };

  const Report = REPORTS[tab];

  return (
    <div>
      <PageHeader title="Báo cáo & Thống kê" description="Dữ liệu mô phỏng, có thể lọc theo ngày, tháng, năm và xuất báo cáo." />
      <Tabs tabs={TABS} active={tab} onChange={setTab} className="mb-5" />
      <ReportFilters filter={filter} onChange={handleFilterChange} />
      <Report filter={filter} />
    </div>
  );
}
