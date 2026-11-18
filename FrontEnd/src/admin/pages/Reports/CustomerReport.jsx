import { useCallback } from 'react';
import {
  Bar,
  BarChart,
  CartesianGrid,
  Cell,
  Legend,
  Pie,
  PieChart,
  ResponsiveContainer,
  Tooltip,
  XAxis,
  YAxis,
} from 'recharts';
import { Globe2, Repeat, UserPlus, Users } from 'lucide-react';
import ChartCard, { chartTooltipStyle } from '../../components/dashboard/ChartCard.jsx';
import StatCard from '../../components/dashboard/StatCard.jsx';
import DataTable from '../../components/common/DataTable.jsx';
import Avatar from '../../components/common/Avatar.jsx';
import { ErrorState, LoadingState } from '../../components/common/StateViews.jsx';
import useFetch from '../../../shared/hooks/useFetch.js';
import reportService from '../../services/reportService.js';
import { downloadCsv } from '../../utils/exportCsv.js';
import { formatCurrency } from '../../../shared/utils/formatters.js';
import { ExportButtons, periodLabel, periodSlug } from './ReportFilters.jsx';

const PIE_COLORS = ['#243f75', '#38acf6', '#7cc8fb', '#10b981', '#f59e0b', '#94a3b8', '#8aa9da', '#ef4444'];

export default function CustomerReport({ filter }) {
  const fetcher = useCallback(() => reportService.getCustomerReport(filter), [filter]);
  const { data, loading, error, reload } = useFetch(fetcher);

  if (loading) return <LoadingState label="Đang tạo báo cáo khách hàng..." />;
  if (error) return <ErrorState message={error} onRetry={reload} />;

  const { series, nationalities, topCustomers, summary } = data;

  const exportExcel = () =>
    downloadCsv(
      `bao-cao-khach-hang-${periodSlug(filter)}.csv`,
      [
        { header: 'Mã KH', key: 'id' },
        { header: 'Họ và tên', key: 'fullName' },
        { header: 'Quốc tịch', key: 'nationality' },
        { header: 'Số lượt lưu trú', key: 'stays' },
        { header: 'Tổng chi tiêu (VND)', key: 'spent' },
      ],
      topCustomers,
    );

  const columns = [
    {
      key: 'fullName',
      header: 'Khách hàng',
      render: (c) => (
        <div className="flex items-center gap-2.5">
          <Avatar name={c.fullName} size={32} />
          <div>
            <p className="font-medium text-slate-800">{c.fullName}</p>
            <p className="text-xs text-slate-400">{c.id}</p>
          </div>
        </div>
      ),
    },
    { key: 'nationality', header: 'Quốc tịch', hideBelow: 'sm' },
    { key: 'stays', header: 'Lượt lưu trú', className: 'text-right' },
    { key: 'spent', header: 'Tổng chi tiêu', className: 'text-right', render: (c) => <span className="font-semibold text-navy-900">{formatCurrency(c.spent)}</span> },
  ];

  return (
    <div className="print-area space-y-5">
      <div className="flex flex-wrap items-center justify-between gap-3">
        <h2 className="text-lg font-semibold">Báo cáo khách hàng · {periodLabel(filter)}</h2>
        <ExportButtons onExcel={exportExcel} />
      </div>

      <div className="grid grid-cols-1 gap-4 sm:grid-cols-2 xl:grid-cols-4">
        <StatCard title="Tổng khách hàng" value={summary.totalCustomers} icon={Users} tone="navy" />
        <StatCard title="Khách mới trong kỳ" value={summary.newInPeriod} icon={UserPlus} tone="sky" />
        <StatCard title="Khách quay lại" value={summary.returning} subtitle="Từ 2 lượt lưu trú trở lên" icon={Repeat} tone="green" />
        <StatCard title="Khách quốc tế" value={summary.foreign} icon={Globe2} tone="amber" />
      </div>

      <div className="grid grid-cols-1 gap-5 xl:grid-cols-3">
        <ChartCard title="Lượt khách theo kỳ" subtitle="Khách mới và khách quay lại" className="xl:col-span-2">
          <div className="h-72">
            <ResponsiveContainer width="100%" height="100%">
              <BarChart data={series} margin={{ top: 4, right: 4, left: -16, bottom: 0 }}>
                <CartesianGrid strokeDasharray="3 3" vertical={false} stroke="#e2e8f0" />
                <XAxis dataKey="label" tickLine={false} axisLine={false} tick={{ fontSize: 12, fill: '#94a3b8' }} />
                <YAxis tickLine={false} axisLine={false} tick={{ fontSize: 12, fill: '#94a3b8' }} />
                <Tooltip cursor={{ fill: '#f0f8ff' }} contentStyle={chartTooltipStyle} />
                <Legend iconType="circle" wrapperStyle={{ fontSize: 12 }} />
                <Bar dataKey="newGuests" name="Khách mới" fill="#2c4d90" radius={[4, 4, 0, 0]} maxBarSize={24} />
                <Bar dataKey="returning" name="Khách quay lại" fill="#7cc8fb" radius={[4, 4, 0, 0]} maxBarSize={24} />
              </BarChart>
            </ResponsiveContainer>
          </div>
        </ChartCard>

        <ChartCard title="Quốc tịch" subtitle="Cơ cấu khách hàng">
          <div className="h-72">
            <ResponsiveContainer width="100%" height="100%">
              <PieChart>
                <Pie data={nationalities} dataKey="value" nameKey="name" innerRadius={48} outerRadius={82} paddingAngle={2}>
                  {nationalities.map((n, i) => (
                    <Cell key={n.name} fill={PIE_COLORS[i % PIE_COLORS.length]} />
                  ))}
                </Pie>
                <Tooltip contentStyle={chartTooltipStyle} formatter={(v, name) => [`${v} khách`, name]} />
                <Legend iconType="circle" wrapperStyle={{ fontSize: 12 }} />
              </PieChart>
            </ResponsiveContainer>
          </div>
        </ChartCard>
      </div>

      <div className="card overflow-hidden">
        <div className="border-b border-slate-100 px-4 py-3">
          <h3 className="text-base font-semibold">Khách hàng chi tiêu nhiều nhất</h3>
        </div>
        <DataTable columns={columns} data={topCustomers} />
      </div>
    </div>
  );
}
