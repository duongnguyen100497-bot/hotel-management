import {
  BOOKING_STATUS,
  EMPLOYEE_STATUS,
  PAYMENT_STATUS,
  ROOM_STATUS,
} from '../../../shared/utils/constants.js';

export function Badge({ className = '', children }) {
  return (
    <span
      className={`inline-flex items-center gap-1.5 whitespace-nowrap rounded-full px-2.5 py-0.5 text-xs font-medium ring-1 ring-inset ${className}`}
    >
      {children}
    </span>
  );
}

export function RoomStatusBadge({ status }) {
  const meta = ROOM_STATUS[status];
  if (!meta) return null;
  return (
    <Badge className={meta.badge}>
      <span className={`h-1.5 w-1.5 rounded-full ${meta.dot}`} />
      {meta.label}
    </Badge>
  );
}

export function BookingStatusBadge({ status }) {
  const meta = BOOKING_STATUS[status];
  return meta ? <Badge className={meta.badge}>{meta.label}</Badge> : null;
}

export function PaymentStatusBadge({ status }) {
  const meta = PAYMENT_STATUS[status];
  return meta ? <Badge className={meta.badge}>{status}</Badge> : null;
}

export function EmployeeStatusBadge({ status }) {
  const meta = EMPLOYEE_STATUS[status];
  return meta ? <Badge className={meta.badge}>{status}</Badge> : null;
}

export function ActiveBadge({ active }) {
  return active ? (
    <Badge className="bg-green-50 text-green-700 ring-green-600/20">Đang hoạt động</Badge>
  ) : (
    <Badge className="bg-slate-100 text-slate-600 ring-slate-500/20">Tạm ngưng</Badge>
  );
}
