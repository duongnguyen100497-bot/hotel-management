import { useCallback } from 'react';
import { Bar, BarChart, CartesianGrid, Legend, ResponsiveContainer, Tooltip, XAxis, YAxis } from 'recharts';
import { BedDouble, ConciergeBell, TrendingUp, Trophy } from 'lucide-react';
import ChartCard, { chartTooltipStyle } from '../../components/dashboard/ChartCard.jsx';
import StatCard from '../../components/dashboard/StatCard.jsx';
import DataTable from '../../components/common/DataTable.jsx';
import { ErrorState, LoadingState } from '../../components/common/StateViews.jsx';
import useFetch from '../../../shared/hooks/useFetch.js';
import reportService from '../../services/reportService.js';
import { downloadCsv } from '../../utils/exportCsv.js';
import { formatCompactVnd, formatCurrency } from '../../../shared/utils/formatters.js';
import { ExportButtons, periodLabel, periodSlug } from './ReportFilters.jsx';

export default function RevenueReport({ filter }) {
  const fetcher = useCallback(() => reportService.getRevenueReport(filter), [filter]);
  const { data, loading, error, reload } = useFetch(fetcher);

  if (loading) return <LoadingState label="Đang tạo báo cáo doanh thu..." />;
  if (error) return <ErrorState message={error} onRetry={reload} />;

  const { series, summary } = data;

  const exportExcel = () =>
    downloadCsv(
      `bao-cao-doanh-thu-${periodSlug(filter)}.csv`,
      [
        { header: 'Kỳ', key: 'label' },
        { header: 'Tiền phòng (VND)', key: 'room' },
        { header: 'Dịch vụ (VND)', key: 'service' },
        { header: 'Tổng doanh thu (VND)', key: 'total' },
      ],
      series,
    );

  const columns = [
    { key: 'label', header: filter.mode === 'day' ? 'Ngày' : filter.mode === 'month' ? 'Tháng' : 'Năm', render: (r) => <span className="font-medium">{r.label}</span> },
    { key: 'room', header: 'Tiền phòng', className: 'text-right', render: (r) => formatCurrency(r.room) },
    { key: 'service', header: 'Dịch vụ', className: 'text-right', render: (r) => formatCurrency(r.service) },
    { key: 'total', header: 'Tổng', className: 'text-right', render: (r) => <span className="font-semibold text-navy-900">{formatCurrency(r.total)}</span> },
  ];

  return (
    <div className="print-area space-y-5">
      <div className="flex flex-wrap items-center justify-between gap-3">
        <h2 className="text-lg font-semibold">Báo cáo doanh thu · {periodLabel(filter)}</h2>
        <ExportButtons onExcel={exportExcel} />
      </div>

      <div className="grid grid-cols-1 gap-4 sm:grid-cols-2 xl:grid-cols-4">
        <StatCard title="Tổng doanh thu" value={formatCurrency(summary.total)} icon={TrendingUp} tone="navy" />
        <StatCard title="Doanh thu phòng" value={formatCurrency(summary.room)} icon={BedDouble} tone="sky" />
        <StatCard title="Doanh thu dịch vụ" value={formatCurrency(summary.service)} icon={ConciergeBell} tone="green" />
        <StatCard
          title="Cao nhất"
          value={formatCurrency(summary.best.total)}
          subtitle={`${summary.best.label} · TB ${formatCompactVnd(summary.average)}/kỳ`}
          icon={Trophy}
          tone="amber"
        />
      </div>

      <ChartCard title="Biểu đồ doanh thu" subtitle="Tiền phòng và dịch vụ theo kỳ">
        <div className="h-80">
          <ResponsiveContainer width="100%" height="100%">
            <BarChart data={series} margin={{ top: 4, right: 4, left: -8, bottom: 0 }}>
              <CartesianGrid strokeDasharray="3 3" vertical={false} stroke="#e2e8f0" />
              <XAxis dataKey="label" tickLine={false} axisLine={false} tick={{ fontSize: 12, fill: '#94a3b8' }} />
              <YAxis tickLine={false} axisLine={false} tick={{ fontSize: 12, fill: '#94a3b8' }} tickFormatter={(v) => formatCompactVnd(v)} width={56} />
              <Tooltip cursor={{ fill: '#f0f8ff' }} contentStyle={chartTooltipStyle} formatter={(v, name) => [formatCurrency(v), name]} />
              <Legend iconType="circle" wrapperStyle={{ fontSize: 12 }} />
              <Bar dataKey="room" name="Tiền phòng" stackId="rev" fill="#2c4d90" maxBarSize={36} />
              <Bar dataKey="service" name="Dịch vụ" stackId="rev" fill="#7cc8fb" radius={[6, 6, 0, 0]} maxBarSize={36} />
            </BarChart>
          </ResponsiveContainer>
        </div>
      </ChartCard>

      <div className="card overflow-hidden">
        <DataTable columns={columns} data={series} rowKey="label" pageSize={12} />
      </div>
    </div>
  );
}
