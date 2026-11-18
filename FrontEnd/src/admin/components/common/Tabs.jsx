export default function Tabs({ tabs, active, onChange, className = '' }) {
  return (
    <div className={`no-print flex gap-1 overflow-x-auto border-b border-slate-200 ${className}`} role="tablist">
      {tabs.map(({ key, label, icon: Icon }) => (
        <button
          key={key}
          type="button"
          role="tab"
          aria-selected={active === key}
          onClick={() => onChange(key)}
          className={`-mb-px flex items-center gap-2 whitespace-nowrap border-b-2 px-4 py-2.5 text-sm font-medium transition-colors ${
            active === key
              ? 'border-navy-700 text-navy-800'
              : 'border-transparent text-slate-500 hover:text-navy-700'
          }`}
        >
          {Icon && <Icon size={16} />}
          {label}
        </button>
      ))}
    </div>
  );
}
