import { Area, AreaChart, CartesianGrid, ResponsiveContainer, Tooltip, XAxis, YAxis } from 'recharts';
import ChartCard, { chartTooltipStyle } from './ChartCard.jsx';

export default function OccupancyChart({ data }) {
  return (
    <ChartCard title="Tỷ lệ sử dụng phòng" subtitle="7 ngày gần nhất (%)">
      <div className="h-64">
        <ResponsiveContainer width="100%" height="100%">
          <AreaChart data={data} margin={{ top: 4, right: 8, left: -20, bottom: 0 }}>
            <defs>
              <linearGradient id="occFill" x1="0" y1="0" x2="0" y2="1">
                <stop offset="0%" stopColor="#38acf6" stopOpacity={0.35} />
                <stop offset="100%" stopColor="#38acf6" stopOpacity={0.02} />
              </linearGradient>
            </defs>
            <CartesianGrid strokeDasharray="3 3" vertical={false} stroke="#e2e8f0" />
            <XAxis dataKey="label" tickLine={false} axisLine={false} tick={{ fontSize: 12, fill: '#94a3b8' }} />
            <YAxis domain={[0, 100]} tickLine={false} axisLine={false} tick={{ fontSize: 12, fill: '#94a3b8' }} />
            <Tooltip contentStyle={chartTooltipStyle} formatter={(v) => [`${v}%`, 'Lấp đầy']} />
            <Area type="monotone" dataKey="rate" stroke="#1d3260" strokeWidth={2.5} fill="url(#occFill)" dot={{ r: 3, fill: '#1d3260' }} />
          </AreaChart>
        </ResponsiveContainer>
      </div>
    </ChartCard>
  );
}
