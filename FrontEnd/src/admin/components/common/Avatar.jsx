import { initials } from '../../../shared/utils/formatters.js';

export default function Avatar({ name, size = 36, className = '' }) {
  return (
    <span
      style={{ width: size, height: size, fontSize: size * 0.36 }}
      className={`inline-flex shrink-0 items-center justify-center rounded-full bg-navy-100 font-semibold text-navy-700 ${className}`}
    >
      {initials(name)}
    </span>
  );
}
