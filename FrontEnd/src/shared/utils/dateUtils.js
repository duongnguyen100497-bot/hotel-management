// Tiện ích ngày tháng (dùng chuỗi 'YYYY-MM-DD' theo giờ địa phương để tránh lệch múi giờ)
const pad = (n) => String(n).padStart(2, '0');

export const toISODate = (value) => {
  const d = new Date(value);
  return `${d.getFullYear()}-${pad(d.getMonth() + 1)}-${pad(d.getDate())}`;
};

export const todayStr = () => toISODate(new Date());

export const parseDate = (dateStr) => new Date(`${dateStr}T00:00:00`);

export const addDays = (dateStr, n) => {
  const d = parseDate(dateStr);
  d.setDate(d.getDate() + n);
  return toISODate(d);
};

export const diffDays = (from, to) =>
  Math.round((parseDate(to) - parseDate(from)) / 86400000);

// month: 1-12
export const daysInMonth = (year, month) => new Date(year, month, 0).getDate();

export const makeDate = (year, month, day) => `${year}-${pad(month)}-${pad(day)}`;
