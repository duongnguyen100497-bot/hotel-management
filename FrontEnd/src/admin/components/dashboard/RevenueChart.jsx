import { useState } from 'react';
import { Bar, BarChart, CartesianGrid, ResponsiveContainer, Tooltip, XAxis, YAxis } from 'recharts';
import ChartCard, { chartTooltipStyle } from './ChartCard.jsx';
import { formatCompactVnd, formatCurrency } from '../../../shared/utils/formatters.js';

const MODES = [
  { key: 'day', label: 'Theo ngày' },
  { key: 'month', label: 'Theo tháng' },
];

export default function RevenueChart({ byDay, byMonth }) {
  const [mode, setMode] = useState('day');
  const data = mode === 'day' ? byDay : byMonth;

  return (
    <ChartCard
      title="Doanh thu"
      subtitle={mode === 'day' ? '14 ngày gần nhất' : 'Các tháng trong năm'}
      className="xl:col-span-2"
      action={
        <div className="flex rounded-lg bg-slate-100 p-0.5 text-xs font-medium">
          {MODES.map((m) => (
            <button
              key={m.key}
              type="button"
              onClick={() => setMode(m.key)}
              className={`rounded-md px-3 py-1.5 transition-colors ${
                mode === m.key ? 'bg-white text-navy-800 shadow-sm' : 'text-slate-500 hover:text-navy-700'
              }`}
            >
              {m.label}
            </button>
          ))}
        </div>
      }
    >
      <div className="h-72">
        <ResponsiveContainer width="100%" height="100%">
          <BarChart data={data} margin={{ top: 4, right: 4, left: -8, bottom: 0 }}>
            <CartesianGrid strokeDasharray="3 3" vertical={false} stroke="#e2e8f0" />
            <XAxis dataKey="label" tickLine={false} axisLine={false} tick={{ fontSize: 12, fill: '#94a3b8' }} />
            <YAxis
              tickLine={false}
              axisLine={false}
              tick={{ fontSize: 12, fill: '#94a3b8' }}
              tickFormatter={(v) => formatCompactVnd(v)}
              width={56}
            />
            <Tooltip
              cursor={{ fill: '#f0f8ff' }}
              contentStyle={chartTooltipStyle}
              formatter={(v) => [formatCurrency(v), 'Doanh thu']}
            />
            <Bar dataKey="total" fill="#2c4d90" radius={[6, 6, 0, 0]} maxBarSize={36} />
          </BarChart>
        </ResponsiveContainer>
      </div>
    </ChartCard>
  );
}
