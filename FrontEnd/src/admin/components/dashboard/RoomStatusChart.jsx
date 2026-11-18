import { Cell, Pie, PieChart, ResponsiveContainer, Tooltip } from 'recharts';
import ChartCard, { chartTooltipStyle } from './ChartCard.jsx';

export default function RoomStatusChart({ data }) {
  const total = data.reduce((s, d) => s + d.value, 0);
  return (
    <ChartCard title="Trạng thái phòng" subtitle={`Tổng ${total} phòng`}>
      <div className="relative h-44">
        <ResponsiveContainer width="100%" height="100%">
          <PieChart>
            <Pie data={data} dataKey="value" nameKey="name" innerRadius={52} outerRadius={78} paddingAngle={2} stroke="none">
              {data.map((d) => (
                <Cell key={d.status} fill={d.color} />
              ))}
            </Pie>
            <Tooltip contentStyle={chartTooltipStyle} formatter={(v, n) => [`${v} phòng`, n]} />
          </PieChart>
        </ResponsiveContainer>
        <div className="pointer-events-none absolute inset-0 flex flex-col items-center justify-center">
          <span className="text-2xl font-semibold text-navy-900">{total}</span>
          <span className="text-xs text-slate-400">phòng</span>
        </div>
      </div>
      <ul className="mt-3 grid grid-cols-1 gap-1.5 text-sm sm:grid-cols-2 xl:grid-cols-1 2xl:grid-cols-2">
        {data.map((d) => (
          <li key={d.status} className="flex items-center justify-between gap-2">
            <span className="flex items-center gap-2 text-slate-600">
              <span className="h-2.5 w-2.5 rounded-full" style={{ backgroundColor: d.color }} />
              {d.name}
            </span>
            <span className="font-medium text-slate-800">{d.value}</span>
          </li>
        ))}
      </ul>
    </ChartCard>
  );
}
