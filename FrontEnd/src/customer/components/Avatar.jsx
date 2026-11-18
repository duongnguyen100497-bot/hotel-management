import { initials } from '../../shared/utils/formatters.js';

// Ảnh đại diện chữ cái đầu của khách (kiểu dáng riêng của website khách hàng).
export default function Avatar({ name, size = 36, className = '' }) {
  return (
    <span
      style={{ width: size, height: size, fontSize: size * 0.36 }}
      className={`inline-flex shrink-0 items-center justify-center rounded-full bg-accent-soft font-semibold text-accent ${className}`}
    >
      {initials(name)}
    </span>
  );
}
