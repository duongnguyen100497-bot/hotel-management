import { CUSTOMER_STATUS } from '../utils/bookingRules.js';

const TONES = {
  warning: 'bg-amber-50 text-amber-700 ring-amber-200',
  accent: 'bg-accent-soft text-accent ring-accent/20',
  success: 'bg-green-50 text-success ring-green-200',
  muted: 'bg-slate-100 text-slate-600 ring-slate-200',
  danger: 'bg-red-50 text-danger ring-red-200',
};

export default function StatusBadge({ status }) {
  const s = CUSTOMER_STATUS[status] || { label: status, tone: 'muted' };
  return <span className={`inline-flex items-center rounded-full px-3 py-1 text-xs font-semibold ring-1 ring-inset ${TONES[s.tone]}`}>{s.label}</span>;
}
