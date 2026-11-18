const DATE_ONLY = /^\d{4}-\d{2}-\d{2}$/;
const toDate = (v) => (typeof v === 'string' && DATE_ONLY.test(v) ? new Date(`${v}T00:00:00`) : new Date(v));

export const formatCurrency = (value) =>
  new Intl.NumberFormat('vi-VN', {
    style: 'currency',
    currency: 'VND',
    maximumFractionDigits: 0,
  }).format(value ?? 0);

export const formatNumber = (value) => new Intl.NumberFormat('vi-VN').format(value ?? 0);

export const formatCompactVnd = (value = 0) => {
  if (value >= 1e9) return `${(value / 1e9).toFixed(1)} tỷ`;
  if (value >= 1e6) return `${Math.round(value / 1e6)} tr`;
  return formatNumber(value);
};

export const formatDate = (value) => {
  if (!value) return '';
  return new Intl.DateTimeFormat('vi-VN').format(toDate(value));
};

export const formatTime = (value) => {
  if (!value) return '';
  return new Intl.DateTimeFormat('vi-VN', { hour: '2-digit', minute: '2-digit' }).format(toDate(value));
};

export const formatDateTime = (value) => {
  if (!value) return '';
  return `${formatTime(value)} ${formatDate(value)}`;
};

export const timeAgo = (iso) => {
  const minutes = Math.max(0, Math.round((Date.now() - new Date(iso).getTime()) / 60000));
  if (minutes < 1) return 'Vừa xong';
  if (minutes < 60) return `${minutes} phút trước`;
  const hours = Math.round(minutes / 60);
  if (hours < 24) return `${hours} giờ trước`;
  return `${Math.round(hours / 24)} ngày trước`;
};

export const initials = (name = '') =>
  name
    .trim()
    .split(/\s+/)
    .slice(-2)
    .map((w) => w[0]?.toUpperCase())
    .join('');
