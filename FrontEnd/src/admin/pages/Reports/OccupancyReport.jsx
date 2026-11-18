import { useCallback } from 'react';
import { Area, AreaChart, CartesianGrid, ResponsiveContainer, Tooltip, XAxis, YAxis } from 'recharts';
import { ArrowDown, ArrowUp, BedDouble, Percent } from 'lucide-react';
import ChartCard, { chartTooltipStyle } from '../../components/dashboard/ChartCard.jsx';
import StatCard from '../../components/dashboard/StatCard.jsx';
import DataTable from '../../components/common/DataTable.jsx';
import { ErrorState, LoadingState } from '../../components/common/StateViews.jsx';
import useFetch from '../../../shared/hooks/useFetch.js';
import reportService from '../../services/reportService.js';
import { downloadCsv } from '../../utils/exportCsv.js';
import { ExportButtons, periodLabel, periodSlug } from './ReportFilters.jsx';

export default function OccupancyReport({ filter }) {
  const fetcher = useCallback(() => reportService.getOccupancyReport(filter), [filter]);
  const { data, loading, error, reload } = useFetch(fetcher);

  if (loading) return <LoadingState label="Đang tạo báo cáo công suất phòng..." />;
  if (error) return <ErrorState message={error} onRetry={reload} />;

  const { series, summary, totalRooms } = data;

  const exportExcel = () =>
    downloadCsv(
      `bao-cao-cong-suat-phong-${periodSlug(filter)}.csv`,
      [
        { header: 'Kỳ', key: 'label' },
        { header: 'Số phòng bán được (bình quân)', key: 'sold' },
        { header: 'Công suất (%)', key: 'rate' },
      ],
      series,
    );

  const columns = [
    { key: 'label', header: filter.mode === 'day' ? 'Ngày' : filter.mode === 'month' ? 'Tháng' : 'Năm', render: (r) => <span className="font-medium">{r.label}</span> },
    { key: 'sold', header: `Phòng bán được / ${totalRooms}`, className: 'text-right' },
    {
      key: 'rate',
      header: 'Công suất',
      render: (r) => (
        <div className="flex items-center justify-end gap-3">
          <div className="hidden h-2 w-32 overflow-hidden rounded-full bg-slate-100 sm:block">
            <div className="h-full rounded-full bg-navy-600" style={{ width: `${r.rate}%` }} />
          </div>
          <span className="w-10 text-right font-semibold text-navy-900">{r.rate}%</span>
        </div>
      ),
    },
  ];

  return (
    <div className="print-area space-y-5">
      <div className="flex flex-wrap items-center justify-between gap-3">
        <h2 className="text-lg font-semibold">Báo cáo công suất phòng · {periodLabel(filter)}</h2>
        <ExportButtons onExcel={exportExcel} />
      </div>

      <div className="grid grid-cols-1 gap-4 sm:grid-cols-2 xl:grid-cols-4">
        <StatCard title="Công suất trung bình" value={`${summary.average}%`} icon={Percent} tone="navy" />
        <StatCard title="Cao nhất" value={`${summary.max}%`} icon={ArrowUp} tone="green" />
        <StatCard title="Thấp nhất" value={`${summary.min}%`} icon={ArrowDown} tone="amber" />
        <StatCard title="Tổng lượt phòng bán" value={summary.soldTotal} subtitle={`Trên ${totalRooms} phòng`} icon={BedDouble} tone="sky" />
      </div>

      <ChartCard title="Biểu đồ công suất phòng" subtitle="Tỷ lệ lấp đầy theo kỳ (%)">
        <div className="h-80">
          <ResponsiveContainer width="100%" height="100%">
            <AreaChart data={series} margin={{ top: 4, right: 8, left: -16, bottom: 0 }}>
              <defs>
                <linearGradient id="occReportFill" x1="0" y1="0" x2="0" y2="1">
                  <stop offset="5%" stopColor="#38acf6" stopOpacity={0.35} />
                  <stop offset="95%" stopColor="#38acf6" stopOpacity={0} />
                </linearGradient>
              </defs>
              <CartesianGrid strokeDasharray="3 3" vertical={false} stroke="#e2e8f0" />
              <XAxis dataKey="label" tickLine={false} axisLine={false} tick={{ fontSize: 12, fill: '#94a3b8' }} />
              <YAxis domain={[0, 100]} tickLine={false} axisLine={false} tick={{ fontSize: 12, fill: '#94a3b8' }} tickFormatter={(v) => `${v}%`} />
              <Tooltip contentStyle={chartTooltipStyle} formatter={(v) => [`${v}%`, 'Công suất']} />
              <Area type="monotone" dataKey="rate" stroke="#2c4d90" strokeWidth={2.5} fill="url(#occReportFill)" />
            </AreaChart>
          </ResponsiveContainer>
        </div>
      </ChartCard>

      <div className="card overflow-hidden">
        <DataTable columns={columns} data={series} rowKey="label" pageSize={12} />
      </div>
    </div>
  );
}
