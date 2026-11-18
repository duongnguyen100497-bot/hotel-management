import { useCallback } from 'react';
import { Bar, BarChart, CartesianGrid, ResponsiveContainer, Tooltip, XAxis, YAxis } from 'recharts';
import { ConciergeBell, Hash, TrendingUp, Trophy } from 'lucide-react';
import ChartCard, { chartTooltipStyle } from '../../components/dashboard/ChartCard.jsx';
import StatCard from '../../components/dashboard/StatCard.jsx';
import DataTable from '../../components/common/DataTable.jsx';
import { ErrorState, LoadingState } from '../../components/common/StateViews.jsx';
import useFetch from '../../../shared/hooks/useFetch.js';
import reportService from '../../services/reportService.js';
import { downloadCsv } from '../../utils/exportCsv.js';
import { formatCompactVnd, formatCurrency } from '../../../shared/utils/formatters.js';
import { ExportButtons, periodLabel, periodSlug } from './ReportFilters.jsx';

export default function ServiceReport({ filter }) {
  const fetcher = useCallback(() => reportService.getServiceReport(filter), [filter]);
  const { data, loading, error, reload } = useFetch(fetcher);

  if (loading) return <LoadingState label="Đang tạo báo cáo dịch vụ..." />;
  if (error) return <ErrorState message={error} onRetry={reload} />;

  const { rows, summary } = data;

  const exportExcel = () =>
    downloadCsv(
      `bao-cao-dich-vu-${periodSlug(filter)}.csv`,
      [
        { header: 'Mã DV', key: 'id' },
        { header: 'Dịch vụ', key: 'name' },
        { header: 'Loại', key: 'type' },
        { header: 'Đơn giá (VND)', key: 'price' },
        { header: 'Số lượt dùng', key: 'count' },
        { header: 'Doanh thu (VND)', key: 'revenue' },
      ],
      rows,
    );

  const columns = [
    { key: 'name', header: 'Dịch vụ', render: (r) => <span className="font-medium text-slate-800">{r.name}</span> },
    { key: 'type', header: 'Loại', hideBelow: 'sm' },
    { key: 'price', header: 'Đơn giá', hideBelow: 'md', className: 'text-right', render: (r) => formatCurrency(r.price) },
    { key: 'count', header: 'Lượt dùng', className: 'text-right' },
    { key: 'revenue', header: 'Doanh thu', className: 'text-right', render: (r) => <span className="font-semibold text-navy-900">{formatCurrency(r.revenue)}</span> },
  ];

  return (
    <div className="print-area space-y-5">
      <div className="flex flex-wrap items-center justify-between gap-3">
        <h2 className="text-lg font-semibold">Báo cáo dịch vụ · {periodLabel(filter)}</h2>
        <ExportButtons onExcel={exportExcel} />
      </div>

      <div className="grid grid-cols-1 gap-4 sm:grid-cols-3">
        <StatCard title="Doanh thu dịch vụ" value={formatCurrency(summary.totalRevenue)} icon={TrendingUp} tone="navy" />
        <StatCard title="Tổng lượt sử dụng" value={summary.totalCount} icon={Hash} tone="sky" />
        <StatCard title="Dịch vụ bán chạy" value={summary.top?.name || '—'} subtitle={summary.top ? formatCurrency(summary.top.revenue) : ''} icon={Trophy} tone="amber" />
      </div>

      <ChartCard title="Doanh thu theo dịch vụ" subtitle="Sắp xếp từ cao đến thấp">
        <div style={{ height: Math.max(240, rows.length * 44) }}>
          <ResponsiveContainer width="100%" height="100%">
            <BarChart data={rows} layout="vertical" margin={{ top: 4, right: 16, left: 8, bottom: 0 }}>
              <CartesianGrid strokeDasharray="3 3" horizontal={false} stroke="#e2e8f0" />
              <XAxis type="number" tickLine={false} axisLine={false} tick={{ fontSize: 12, fill: '#94a3b8' }} tickFormatter={(v) => formatCompactVnd(v)} />
              <YAxis type="category" dataKey="name" width={130} tickLine={false} axisLine={false} tick={{ fontSize: 12, fill: '#475569' }} />
              <Tooltip cursor={{ fill: '#f0f8ff' }} contentStyle={chartTooltipStyle} formatter={(v) => [formatCurrency(v), 'Doanh thu']} />
              <Bar dataKey="revenue" fill="#2c4d90" radius={[0, 6, 6, 0]} maxBarSize={24} />
            </BarChart>
          </ResponsiveContainer>
        </div>
      </ChartCard>

      <div className="card overflow-hidden">
        {rows.length === 0 ? (
          <div className="flex flex-col items-center gap-2 py-12 text-slate-400">
            <ConciergeBell size={26} />
            <p className="text-sm">Chưa có dịch vụ nào.</p>
          </div>
        ) : (
          <DataTable columns={columns} data={rows} pageSize={10} />
        )}
      </div>
    </div>
  );
}
