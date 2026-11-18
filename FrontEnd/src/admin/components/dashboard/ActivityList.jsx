import { Activity, BedDouble, CalendarPlus, CreditCard, LogIn, LogOut } from 'lucide-react';
import { timeAgo } from '../../../shared/utils/formatters.js';

const TYPES = {
  checkin: { icon: LogIn, cls: 'bg-green-50 text-green-600' },
  checkout: { icon: LogOut, cls: 'bg-slate-100 text-slate-600' },
  booking: { icon: CalendarPlus, cls: 'bg-blue-50 text-blue-600' },
  payment: { icon: CreditCard, cls: 'bg-amber-50 text-amber-600' },
  room: { icon: BedDouble, cls: 'bg-cyan-50 text-cyan-600' },
};

export default function ActivityList({ activities }) {
  return (
    <div className="card">
      <div className="flex items-center gap-2 border-b border-slate-100 px-5 py-4">
        <Activity size={18} className="text-navy-500" />
        <h3 className="text-base font-semibold">Hoạt động gần đây</h3>
      </div>
      <ul className="divide-y divide-slate-100">
        {activities.map((a) => {
          const { icon: Icon, cls } = TYPES[a.type] || TYPES.room;
          return (
            <li key={a.id} className="flex items-start gap-3 px-5 py-3 transition-colors hover:bg-sky-50/50">
              <span className={`mt-0.5 flex h-8 w-8 shrink-0 items-center justify-center rounded-full ${cls}`}>
                <Icon size={15} />
              </span>
              <div className="min-w-0">
                <p className="text-sm text-slate-700">{a.message}</p>
                <p className="mt-0.5 text-xs text-slate-400">{timeAgo(a.time)}</p>
              </div>
            </li>
          );
        })}
        {activities.length === 0 && <li className="px-5 py-8 text-center text-sm text-slate-400">Chưa có hoạt động</li>}
      </ul>
    </div>
  );
}
