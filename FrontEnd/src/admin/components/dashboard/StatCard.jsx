const TONES = {
  navy: 'bg-navy-50 text-navy-700',
  green: 'bg-green-50 text-green-600',
  red: 'bg-red-50 text-red-600',
  amber: 'bg-amber-50 text-amber-600',
  sky: 'bg-sky-100 text-sky-500',
  slate: 'bg-slate-100 text-slate-600',
};

export default function StatCard({ title, value, subtitle, icon: Icon, tone = 'navy' }) {
  return (
    <div className="card card-hover flex items-start justify-between gap-3 p-5 transition-transform duration-200 hover:-translate-y-0.5">
      <div className="min-w-0">
        <p className="text-sm text-slate-500">{title}</p>
        <p className="mt-1.5 truncate text-2xl font-semibold text-navy-900">{value}</p>
        {subtitle && <p className="mt-1 text-xs text-slate-400">{subtitle}</p>}
      </div>
      <span className={`flex h-11 w-11 shrink-0 items-center justify-center rounded-xl ${TONES[tone]}`}>
        <Icon size={21} />
      </span>
    </div>
  );
}
